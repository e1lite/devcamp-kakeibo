#!/bin/bash
# 本番の DB にマイグレーションを適用する。API の成果物（dist/api.tar.gz）に同梱して配る。
#
# Session Manager で API の EC2 に入り、次のように実行する:
#   sudo /opt/kakeibo/api/migrate.sh up         最新まで適用する
#   sudo /opt/kakeibo/api/migrate.sh version    現在のバージョンを表示する
#
# 接続先（DATABASE_URL）は、起動時にユーザーデータが Parameter Store から書き出した
# /etc/kakeibo/api.env から読む。誰がいつ実行したかは Session Manager の記録に残る。
set -euo pipefail

ENV_FILE=/etc/kakeibo/api.env

if [ ! -r "$ENV_FILE" ]; then
  echo "$ENV_FILE を読めません。sudo を付けて実行してください" >&2
  exit 1
fi

# シェルとして読み込む（source）と値の中の記号が解釈されるため、1 行ずつ KEY=VALUE として取り込む。
# 最初の = で分ける。read の IFS='=' で分けると、base64 の値の末尾の = が欠けることがある
while IFS= read -r line; do
  case "$line" in
    '' | '#'*) continue ;;
  esac
  export "${line%%=*}=${line#*=}"
done <"$ENV_FILE"

# cmd/migrate は作業ディレクトリの migrations/ を読む
cd "$(dirname "$0")"
exec ./migrate "$@"
