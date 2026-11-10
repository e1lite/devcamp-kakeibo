#!/bin/bash
# フロント（Next.js）の EC2 の起動スクリプト。起動テンプレート lt-web の「ユーザーデータ」に全文を貼る。
#
# EC2 が初めて起動したときに 1 回だけ root で動き、次のことをする:
#   1. CloudWatch Agent を入れて、起動ログとアプリのログを CloudWatch Logs（/kakeibo/web）へ送る
#   2. S3 から成果物（releases/web.tar.gz）を取って /opt/kakeibo/web に展開する
#   3. systemd のサービス kakeibo-web として常駐させる（落ちたら 5 秒後に自動で再起動）
#
# フロントは秘密情報を使わないため、Parameter Store は読まない（IAM ロールにも読む権限を付けていない）。
# 成果物に Node.js 本体を同梱しているため、EC2 に Node.js を入れる必要は無い（scripts/build-release.sh）。
set -euo pipefail

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
          { "file_path": "/var/log/kakeibo/web.log", "log_group_name": "/kakeibo/web", "log_stream_name": "{instance_id}" },
          { "file_path": "/var/log/cloud-init-output.log", "log_group_name": "/kakeibo/web", "log_stream_name": "{instance_id}-boot" }
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
mkdir -p /opt/kakeibo/web
aws s3 cp "s3://${BUCKET}/releases/web.tar.gz" /tmp/web.tar.gz --region "$REGION" --no-progress
tar -xzf /tmp/web.tar.gz -C /opt/kakeibo/web
rm -f /tmp/web.tar.gz
# Next.js が実行中に .next/cache へ書き込むことがあるため、実行ユーザーの持ち物にする
chown -R kakeibo:kakeibo /opt/kakeibo/web /var/log/kakeibo
log "version=$(cat /opt/kakeibo/web/VERSION)"

# --- 3. サービスとして常駐させる
cat >/etc/systemd/system/kakeibo-web.service <<'EOF'
[Unit]
Description=kakeibo web front (Next.js standalone)
After=network-online.target
Wants=network-online.target

[Service]
User=kakeibo
Group=kakeibo
WorkingDirectory=/opt/kakeibo/web
# ALB のヘルスチェックとリクエストを受けるため、全インタフェースの 5173 番で待ち受ける
Environment=NODE_ENV=production PORT=5173 HOSTNAME=0.0.0.0 NEXT_TELEMETRY_DISABLED=1
ExecStart=/opt/kakeibo/web/node server.js
Restart=always
RestartSec=5
StandardOutput=append:/var/log/kakeibo/web.log
StandardError=append:/var/log/kakeibo/web.log

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now kakeibo-web
log "kakeibo-web を起動しました"
