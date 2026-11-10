#!/bin/bash
# API サーバ（Go）の EC2 の起動スクリプト。起動テンプレート lt-api の「ユーザーデータ」に全文を貼る。
#
# EC2 が初めて起動したときに 1 回だけ root で動き、次のことをする:
#   1. CloudWatch Agent を入れて、起動ログとアプリのログを CloudWatch Logs（/kakeibo/api）へ送る
#   2. S3 から成果物（releases/api.tar.gz）を取って /opt/kakeibo/api に展開する
#   3. 設定を /etc/kakeibo/api.env に書き出す（秘密情報は Parameter Store の /kakeibo/prod/api/* から）
#   4. systemd のサービス kakeibo-api として常駐させる（落ちたら 5 秒後に自動で再起動）
#
# 新しい版を配るときは、S3 の api.tar.gz を差し替えて、Auto Scaling グループの「インスタンスの更新」で台を入れ替える。
# 秘密情報をログに出さないよう、set -x（実行したコマンドの表示）は使わない。
set -euo pipefail

# --- この環境の設定（秘密情報ではないもの） --------------------------------
# 公開する URL。Google に登録したコールバック URL・戻り先の許可リストと一致させる
APP_ORIGIN="https://app.autokakeibo.com"
# 1 台が持つ DB 接続の上限。最大 4 台 × 15 = 60 本で、db.t4g.micro の上限に余裕を残す（Step 7 の 6 章 #3）
DB_MAX_OPEN_CONNS=15
# 秘密情報の置き場所（Parameter Store）
PARAM_PATH="/kakeibo/prod/api"
# ---------------------------------------------------------------------------

log() { echo "[kakeibo-userdata] $*"; }

# インスタンスメタデータ（IMDSv2）から、リージョンとアカウント ID を取る。成果物のバケット名に使う
IMDS_TOKEN="$(curl -sS -X PUT http://169.254.169.254/latest/api/token -H 'X-aws-ec2-metadata-token-ttl-seconds: 300')"
IDENTITY="$(curl -sS -H "X-aws-ec2-metadata-token: $IMDS_TOKEN" http://169.254.169.254/latest/dynamic/instance-identity/document)"
REGION="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["region"])' <<<"$IDENTITY")"
ACCOUNT_ID="$(python3 -c 'import json,sys; print(json.load(sys.stdin)["accountId"])' <<<"$IDENTITY")"
BUCKET="kakeibo-artifacts-${ACCOUNT_ID}"
log "region=$REGION bucket=$BUCKET"

# --- 1. ログを CloudWatch Logs へ送る（後の手順で失敗しても、原因を CloudWatch で見られるよう最初に入れる）
dnf install -y amazon-cloudwatch-agent
mkdir -p /var/log/kakeibo
cat >/opt/aws/amazon-cloudwatch-agent/etc/kakeibo.json <<'EOF'
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          { "file_path": "/var/log/kakeibo/api.log", "log_group_name": "/kakeibo/api", "log_stream_name": "{instance_id}" },
          { "file_path": "/var/log/cloud-init-output.log", "log_group_name": "/kakeibo/api", "log_stream_name": "{instance_id}-boot" }
        ]
      }
    }
  }
}
EOF
/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a fetch-config -m ec2 -s -c file:/opt/aws/amazon-cloudwatch-agent/etc/kakeibo.json

# --- 2. 成果物を取って展開する
id kakeibo >/dev/null 2>&1 || useradd --system --home-dir /opt/kakeibo --shell /sbin/nologin kakeibo
mkdir -p /opt/kakeibo/api
aws s3 cp "s3://${BUCKET}/releases/api.tar.gz" /tmp/api.tar.gz --region "$REGION" --no-progress
tar -xzf /tmp/api.tar.gz -C /opt/kakeibo/api
rm -f /tmp/api.tar.gz
chown -R root:root /opt/kakeibo/api
chown kakeibo:kakeibo /var/log/kakeibo
log "version=$(cat /opt/kakeibo/api/VERSION)"

# --- 3. 設定を書き出す。root だけが読める（中身に DB のパスワードなどを含むため）
ENV_FILE=/etc/kakeibo/api.env
mkdir -p /etc/kakeibo
install -m 600 -o root -g root /dev/null "$ENV_FILE"
cat >>"$ENV_FILE" <<EOF
APP_ENV=production
PORT=8080
GOOGLE_REDIRECT_URL=${APP_ORIGIN}/api/v1/auth/google/callback
GOOGLE_MAIL_REDIRECT_URL=${APP_ORIGIN}/api/v1/mail-accounts/google/callback
DEFAULT_REDIRECT_URI=${APP_ORIGIN}/auth/callback
DEFAULT_MAIL_REDIRECT_URI=${APP_ORIGIN}/settings/mail-accounts
ALLOWED_REDIRECT_URIS=${APP_ORIGIN}/auth/callback,${APP_ORIGIN}/settings/mail-accounts
DB_MAX_OPEN_CONNS=${DB_MAX_OPEN_CONNS}
EOF

# Parameter Store の /kakeibo/prod/api/<名前> を、<名前>=<値> の形で追記する（値はログに出さない）
aws ssm get-parameters-by-path --region "$REGION" --path "$PARAM_PATH" --with-decryption \
  --query 'Parameters[].[Name,Value]' --output text |
  while IFS=$'\t' read -r name value; do
    printf '%s=%s\n' "${name##*/}" "$value" >>"$ENV_FILE"
  done

# 必須の秘密情報がそろっているかを、名前だけで確かめる
for key in DATABASE_URL JWT_SECRET MAIL_CREDENTIAL_KEY GOOGLE_CLIENT_ID GOOGLE_CLIENT_SECRET; do
  if ! grep -q "^${key}=" "$ENV_FILE"; then
    log "ERROR: Parameter Store に ${PARAM_PATH}/${key} がありません（IAM ロールの権限も確認）"
    exit 1
  fi
done
log "設定を ${ENV_FILE} に書き出しました"

# --- 4. サービスとして常駐させる
cat >/etc/systemd/system/kakeibo-api.service <<'EOF'
[Unit]
Description=kakeibo API server (Go)
After=network-online.target
Wants=network-online.target

[Service]
User=kakeibo
Group=kakeibo
WorkingDirectory=/opt/kakeibo/api
EnvironmentFile=/etc/kakeibo/api.env
ExecStart=/opt/kakeibo/api/api
# DB にテーブルが無い（マイグレーション前）・DB が切り替え中などで落ちても、5 秒後に起動し直す
Restart=always
RestartSec=5
StandardOutput=append:/var/log/kakeibo/api.log
StandardError=append:/var/log/kakeibo/api.log

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now kakeibo-api
log "kakeibo-api を起動しました"
