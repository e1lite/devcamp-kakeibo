#!/usr/bin/env bash
#
# Step 9 の動作確認。Docker Compose で起動した一式（DB・マイグレーション・API・フロント）に対して、
# Step 9 の判定基準（3.1）の 3 種類の確認を curl で行う。提出物の「動作確認」に貼る出力はこのスクリプトで取る。
#
#   0. docker compose up -d で一式が起動すること（コンテナを作り直して起動から確かめる）
#   1. 疎通確認     GET /health（API-001）に API が応答し、DB にも繋がっていること
#   2. 書き込み確認 POST /api/v1/transactions（API-011）で取引を登録し、DB の transactions テーブルに行が入ること
#   3. 読み取り確認 GET /api/v1/transactions/{id}（API-012）で、書き込んだ値を DB から読めること
#   4. （追加）コンテナを作り直しても、DB のデータ（ボリューム）が残ること
#
# このアプリのユーザ登録は Google ログイン（ブラウザでの操作）で行うため、curl での書き込みの確認には
# 取引の登録を使う。curl に付けるトークンは、事前にブラウザでログインして取っておく。
#
# 使い方:
#   1. docker compose up -d                   一式を起動する（初回はイメージのビルドで数分かかる）
#   2. ブラウザで http://localhost:5173 を開き、Google でログインする
#   3. 開発者ツールのコンソールで copy(sessionStorage.getItem("kakeibo.access_token")) を実行し、
#      ターミナルで pbpaste > .token
#   4. ./scripts/verify-compose.sh 2>&1 | tee docs/step9-verify-output.txt
#
# 途中で docker compose down → up -d を 2 回行う（コンテナは作り直すが、DB のボリュームは消さない）。
# 登録した取引は最後に削除する（API-014。取引は論理削除のため、行は deleted_at 付きで残る）。
# トークンと、ログインしたユーザの情報（GET /api/v1/auth/me の中身）は出力しない。
set -uo pipefail

cd "$(dirname "$0")/.."

# shellcheck source=./lib.sh
source ./scripts/lib.sh

BASE_URL="http://localhost:8080"
WEB_URL="http://localhost:5173"
# 出力をファイルに残すため、色の制御文字を出さない
export COMPOSE_ANSI=never

# step TITLE — 確認の区切りの見出し
step() {
    echo
    echo "####################################################################"
    echo "# $1"
    echo "####################################################################"
}

# run COMMAND... — 実行するコマンドを表示してから実行する
run() {
    echo "\$ $*"
    "$@"
}

# wait_healthy SERVICE — コンテナのヘルスチェックが healthy になるまで待つ（最大 60 秒）
wait_healthy() {
    local service=$1 i
    for i in $(seq 1 60); do
        if [ "$(docker compose ps --format '{{.Health}}' "$service" 2>/dev/null)" = "healthy" ]; then
            return 0
        fi
        sleep 1
    done
    echo "${service} が 60 秒たっても healthy になりませんでした（docker compose logs ${service} を確認）" >&2
    return 1
}

# compose_up — docker compose up -d で起動し、状態を表示する
compose_up() {
    run docker compose up -d || exit 1
    # api までは up -d が depends_on で待つ。web は誰も待たないため、ここで待ってから状態を見る
    wait_healthy web || exit 1
    echo
    run docker compose ps -a --format 'table {{.Service}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}'
}

token_help() {
    cat >&2 <<EOF

$1

1. docker compose up -d で一式を起動する
2. ブラウザで ${WEB_URL} を開き、Google でログインする
3. 開発者ツールのコンソールで copy(sessionStorage.getItem("kakeibo.access_token")) を実行する
4. ターミナルで pbpaste > .token

トークンの有効期限は 24 時間。本番（Step 8）でログインしたトークンは、署名の鍵が違うためここでは使えない。
EOF
    exit 1
}

# --- トークン（中身は表示しない） -----------------------------------------------------------
[ -s "$TOKEN_FILE" ] || token_help ".token がありません。"
TOKEN=$(tr -d '[:space:]' <"$TOKEN_FILE")
# JWT（英数字・-・_ を . で 3 つに区切ったもの）の形か。クリップボードの別の文字が入っていたら止める
if ! LC_ALL=C grep -Eq '^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$' <<<"$TOKEN"; then
    token_help ".token の中身がトークンの形になっていません。"
fi

banner "Step 9 動作確認（Docker Compose）"
run docker compose version

# ==============================================================================================
step "0. docker compose up -d で一式が起動すること"
# ==============================================================================================
echo "# 起動から確かめるため、いったんコンテナを消す（DB のボリュームは消さない）"
run docker compose down
echo
compose_up

echo
echo "# migrate は、マイグレーションを流して終了したコンテナ（Exited (0) なら成功）"
run docker compose logs --no-log-prefix migrate

echo
echo "# イメージのサイズ。比較のため、ビルド用のステージだけを取り出したイメージも作る（ビルドのキャッシュが効く）"
docker build -q --target build -t kakeibo-api:build-stage ./backend >/dev/null &&
    docker build -q --target build -t kakeibo-web:build-stage ./frontend >/dev/null
run docker image ls --filter 'reference=kakeibo-*' --format 'table {{.Repository}}:{{.Tag}}\t{{.Size}}'
docker image rm kakeibo-api:build-stage kakeibo-web:build-stage >/dev/null 2>&1

# ログインしたユーザがこの DB にいるか（トークンが使えるか）を、中身を出さずに確かめる
status=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer ${TOKEN}" "${BASE_URL}/api/v1/auth/me")
[ "$status" = "200" ] || token_help "トークンが使えません（GET /api/v1/auth/me が ${status}）。"

# ==============================================================================================
step "1. 疎通確認：GET /health（API-001）"
# ==============================================================================================
check_noauth "API-001 ヘルスチェック（ホスト → api:8080）。DB への疎通も確かめて 200 を返す" 200 /health
expect "data.status"   ok "$(json_value status <<<"$LAST_PAYLOAD")"
expect "data.database" ok "$(json_value database <<<"$LAST_PAYLOAD")"

echo
echo "# フロントのコンテナから、サービス名 api で API に届くこと（Compose のネットワークの名前解決）"
run docker compose exec -T web wget -qO- http://api:8080/health
echo

# ==============================================================================================
step "2. 書き込み確認：POST /api/v1/transactions（API-011）"
# ==============================================================================================
OCCURRED_AT=$(TZ=Asia/Tokyo date '+%Y-%m-%dT%H:%M:%S+09:00')
NOTE="Step 9 docker compose の書き込み確認"
check "API-011 取引を登録する" 201 POST /api/v1/transactions \
    "{\"occurred_at\":\"${OCCURRED_AT}\",\"amount_minor\":980,\"note\":\"${NOTE}\"}"
TXN_ID=$(json_value id <<<"$LAST_PAYLOAD")
if [ -z "$TXN_ID" ]; then
    echo "登録した取引の ID を取得できませんでした" >&2
    summary
    exit 1
fi
echo
echo "登録した取引の ID: ${TXN_ID}"

echo
echo "# DB のコンテナの psql で、transactions テーブルに行が入ったことを確かめる"
run docker compose exec -T db psql -U kakeibo -d kakeibo -c \
    "SELECT id, occurred_at, amount_minor, source, note, deleted_at FROM transactions WHERE id = ${TXN_ID};"
expect "DB の行数（id = ${TXN_ID}、削除されていない）" 1 "$(docker compose exec -T db psql -U kakeibo -d kakeibo -tA -c \
    "SELECT count(*) FROM transactions WHERE id = ${TXN_ID} AND deleted_at IS NULL;" | tr -d '[:space:]')"

# ==============================================================================================
step "3. 読み取り確認：GET /api/v1/transactions/{id}（API-012）"
# ==============================================================================================
check "API-012 登録した取引を取得する（ホスト → api:8080 → db:5432）" 200 GET "/api/v1/transactions/${TXN_ID}"
expect "amount_minor" 980 "$(json_value amount_minor <<<"$LAST_PAYLOAD")"
expect "note"         "$NOTE" "$(json_value note <<<"$LAST_PAYLOAD")"
expect "source"       manual "$(json_value source <<<"$LAST_PAYLOAD")"

# ブラウザと同じ経路。Next.js（web）が /api/* を api:8080 へ転送する
BASE_URL="$WEB_URL"
check "フロントのコンテナ経由で同じ取引を取得する（ホスト → web:3000 → api:8080 → db:5432）" \
    200 GET "/api/v1/transactions/${TXN_ID}"
expect "amount_minor" 980 "$(json_value amount_minor <<<"$LAST_PAYLOAD")"
BASE_URL="http://localhost:8080"

# ==============================================================================================
step "4. コンテナを作り直しても、DB のデータが残ること（名前付きボリューム）"
# ==============================================================================================
run docker volume ls --filter name=db-data
echo
run docker compose down
echo
compose_up
echo
check "作り直したコンテナで、登録した取引を取得する" 200 GET "/api/v1/transactions/${TXN_ID}"
expect "amount_minor" 980 "$(json_value amount_minor <<<"$LAST_PAYLOAD")"
expect "note"         "$NOTE" "$(json_value note <<<"$LAST_PAYLOAD")"

# ==============================================================================================
step "5. 後始末：登録した取引を削除する（API-014）"
# ==============================================================================================
check "API-014 取引を削除する（論理削除）" 204 DELETE "/api/v1/transactions/${TXN_ID}"
check "削除したあとは 404" 404 GET "/api/v1/transactions/${TXN_ID}"

echo
echo "# API のコンテナのログ（直近のリクエスト。/health は Docker のヘルスチェックによるもの）"
docker compose logs --no-log-prefix api | grep '"msg":"request"' | tail -n 8

echo
summary
