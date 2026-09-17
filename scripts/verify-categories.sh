#!/usr/bin/env bash
#
# API-019 / 020 / 021（カテゴリ CRUD）の動作確認。
#
# 判定基準:
#   - 正常系のリクエストで期待するレスポンスが返ること
#   - 異常系のリクエストで想定するエラーレスポンスが返ること
#   - 作成・更新・削除の後に取得系を呼び、データが永続化されていること
#
# 使い方:
#   make run                                  別のターミナルで API サーバを起動
#   ./scripts/verify-categories.sh            .token に保存されたトークンを使う
#   ./scripts/verify-categories.sh '<URL>'    初回はログイン後のリダイレクト URL を渡す
#
set -uo pipefail

# shellcheck source=./lib.sh
source "$(dirname "$0")/lib.sh"

load_token "${1:-}"

# category_id NAME — 一覧から指定した名前のカテゴリの ID を取り出す
category_id() {
    local name=$1
    req GET /api/v1/categories | sed '$d' \
        | tr '}' '\n' \
        | grep "\"name\":\"${name}\"" \
        | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2
}

banner "カテゴリ CRUD 動作確認"

# --- 正常系 -------------------------------------------------------------

check "API-019 カテゴリ一覧（実行開始時点。ログイン時に初期カテゴリが作られている）" \
    200 GET /api/v1/categories
BASE_COUNT=$(meta_value total)
if [ -z "$BASE_COUNT" ]; then
    echo "一覧の meta を読み取れませんでした。サーバの応答を確認してください" >&2
    exit 1
fi
echo
echo "基準値: ${BASE_COUNT} 件"

check "API-019 取引件数つきで取得する" \
    200 GET '/api/v1/categories?include_counts=true'

check "API-020 カテゴリを作成する" \
    201 POST /api/v1/categories '{"name":"交際費","sort_order":10}'

CREATED_ID=$(json_value id <<<"$LAST_PAYLOAD")
if [ -z "$CREATED_ID" ]; then
    echo "作成したカテゴリの ID を取得できませんでした" >&2
    exit 1
fi
echo
echo "作成されたカテゴリ ID: ${CREATED_ID}"

# 初期カテゴリ（is_system = true）の ID。削除できないことの確認に使う
SYSTEM_ID=$(category_id "食費")

check "API-019 作成後に一覧を取得し、永続化されていることを確認する" \
    200 GET /api/v1/categories
expect "meta.total" "$((BASE_COUNT + 1))" "$(meta_value total)"

check "API-021 PUT カテゴリ名と表示順を更新する" \
    200 PUT "/api/v1/categories/${CREATED_ID}" '{"name":"交際費・接待","sort_order":11}'
expect "name"       "交際費・接待" "$(json_value name <<<"$LAST_PAYLOAD")"
expect "sort_order" 11         "$(json_value sort_order <<<"$LAST_PAYLOAD")"

check "API-021 PUT 部分更新（sort_order だけ変更し、name は維持される）" \
    200 PUT "/api/v1/categories/${CREATED_ID}" '{"sort_order":12}'
expect "name"       "交際費・接待" "$(json_value name <<<"$LAST_PAYLOAD")"
expect "sort_order" 12         "$(json_value sort_order <<<"$LAST_PAYLOAD")"

# --- 異常系 -------------------------------------------------------------

check "name が空なら 400 VALIDATION_ERROR" \
    400 POST /api/v1/categories '{"name":""}'

check "name が 50 文字を超えたら 400 VALIDATION_ERROR" \
    400 POST /api/v1/categories \
    '{"name":"あいうえおかきくけこあいうえおかきくけこあいうえおかきくけこあいうえおかきくけこあいうえおかきくけこあ"}'

check "仕様にないフィールドを送ったら 400 VALIDATION_ERROR" \
    400 POST /api/v1/categories '{"name":"タイポ確認","sort_oder":1}'

check "同名のカテゴリは 409 DUPLICATE_CATEGORY_NAME" \
    409 POST /api/v1/categories '{"name":"食費"}'

check "存在しない親カテゴリを指定したら 404 PARENT_NOT_FOUND" \
    404 POST /api/v1/categories '{"name":"親なし","parent_id":999999}'

check "自分自身を親にしたら 400 INVALID_PARENT（循環するため）" \
    400 PUT "/api/v1/categories/${CREATED_ID}" "{\"parent_id\":${CREATED_ID}}"

check "存在しないカテゴリの更新は 404 CATEGORY_NOT_FOUND" \
    404 PUT /api/v1/categories/999999 '{"name":"存在しない"}'

check "初期カテゴリ（is_system）の削除は 400 SYSTEM_CATEGORY_NOT_DELETABLE" \
    400 DELETE "/api/v1/categories/${SYSTEM_ID}"

check_noauth "認証なしのリクエストは 401 INVALID_TOKEN" \
    401 /api/v1/categories

# --- 削除と永続化の確認 -------------------------------------------------

check "API-021 DELETE 作成したカテゴリを削除する" \
    204 DELETE "/api/v1/categories/${CREATED_ID}"

check "削除したカテゴリを更新しようとすると 404 CATEGORY_NOT_FOUND" \
    404 PUT "/api/v1/categories/${CREATED_ID}" '{"name":"消えたはず"}'

check "API-019 削除後の一覧（実行開始時点の件数に戻り、作成したカテゴリが消えている）" \
    200 GET /api/v1/categories
expect "meta.total"           "$BASE_COUNT" "$(meta_value total)"
expect "交際費・接待 を含む行数" 0            "$(grep -c '交際費・接待' <<<"$LAST_PAYLOAD")"

summary
