#!/usr/bin/env bash
#
# API-010〜014（取引 CRUD）の動作確認。
#
# 判定基準:
#   - 正常系のリクエストで期待するレスポンスが返ること
#   - 異常系のリクエストで想定するエラーレスポンスが返ること
#   - 作成・更新・削除の後に取得系を呼び、データが永続化されていること
#
# 件数・合計の確認方針:
#   実行開始時点の件数と合計を「基準値」として取っておき、以降はそこからの
#   差分で検証する。これにより、データベースに以前のデータが残っていても
#   確認が成り立つ。また作成した取引は最後にすべて削除するため、
#   このスクリプトは何度実行しても同じ状態から始められる。
#
# 使い方:
#   make run                                    別のターミナルで API サーバを起動
#   ./scripts/verify-transactions.sh            .token に保存されたトークンを使う
#   ./scripts/verify-transactions.sh '<URL>'    初回はログイン後のリダイレクト URL を渡す
#
set -uo pipefail

# shellcheck source=./lib.sh
source "$(dirname "$0")/lib.sh"

load_token "${1:-}"

banner "取引 CRUD 動作確認"

# 既存のカテゴリ ID を 1 つ取る（取引に紐づける）
CATEGORY_ID=$(req GET /api/v1/categories | sed '$d' \
    | tr '}' '\n' | grep '"name":"食費"' | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
if [ -z "$CATEGORY_ID" ]; then
    echo "カテゴリ ID を取得できませんでした" >&2
    exit 1
fi
echo "紐づけるカテゴリ ID: ${CATEGORY_ID}"

# --- 正常系 -------------------------------------------------------------

check "API-010 取引一覧（実行開始時点。以降はこの件数・合計との差分で確認する）" \
    200 GET /api/v1/transactions
BASE_ALL_N=$(meta_value total)
BASE_ALL_A=$(meta_value total_amount_minor)
if [ -z "$BASE_ALL_N" ] || [ -z "$BASE_ALL_A" ]; then
    echo "一覧の meta を読み取れませんでした。サーバの応答を確認してください" >&2
    exit 1
fi

# 絞り込みの確認も差分で行うため、各条件の基準値も先に取っておく
read -r BASE_DAY_N BASE_DAY_A <<<"$(snapshot '/api/v1/transactions?from=2026-09-10&to=2026-09-10')"
read -r BASE_CAT_N BASE_CAT_A <<<"$(snapshot "/api/v1/transactions?category_id=${CATEGORY_ID}")"
read -r BASE_SHOP_N BASE_SHOP_A <<<"$(snapshot '/api/v1/transactions?q=定食')"
read -r BASE_NOTE_N BASE_NOTE_A <<<"$(snapshot '/api/v1/transactions?q=現金')"
echo
echo "基準値: 全件 ${BASE_ALL_N} 件 / 合計 ${BASE_ALL_A} 円"

check "API-011 取引を手動登録する（店舗名は新規作成される）" \
    201 POST /api/v1/transactions \
    "{\"occurred_at\":\"2026-09-10T19:20:00+09:00\",\"amount_minor\":1200,\"merchant_name\":\"近所の定食屋\",\"category_id\":${CATEGORY_ID},\"note\":\"現金\"}"
TXN_ID=$(json_value id <<<"$LAST_PAYLOAD")
if [ -z "$TXN_ID" ]; then
    echo "作成した取引の ID を取得できませんでした" >&2
    exit 1
fi
echo
echo "作成された取引 ID: ${TXN_ID}"

check "API-012 作成後に詳細を取得し、永続化されていることを確認する" \
    200 GET "/api/v1/transactions/${TXN_ID}"
expect "amount_minor" 1200 "$(json_value amount_minor <<<"$LAST_PAYLOAD")"
expect "note"         "現金" "$(json_value note <<<"$LAST_PAYLOAD")"

check "API-011 2 件目を別の日付で登録する" \
    201 POST /api/v1/transactions \
    '{"occurred_at":"2026-09-01T12:00:00+09:00","amount_minor":580,"merchant_name":"セブン-イレブン渋谷店"}'
TXN_ID_2=$(json_value id <<<"$LAST_PAYLOAD")
if [ -z "$TXN_ID_2" ]; then
    echo "2 件目の取引の ID を取得できませんでした" >&2
    exit 1
fi

check "API-010 一覧（登録した 2 件・1780 円ぶんが増えていること）" \
    200 GET /api/v1/transactions
expect_meta "$((BASE_ALL_N + 2))" "$((BASE_ALL_A + 1780))"

check "API-010 期間で絞り込む（9/10 のみ）" \
    200 GET '/api/v1/transactions?from=2026-09-10&to=2026-09-10'
expect_meta "$((BASE_DAY_N + 1))" "$((BASE_DAY_A + 1200))"

check "API-010 カテゴリで絞り込む" \
    200 GET "/api/v1/transactions?category_id=${CATEGORY_ID}"
expect_meta "$((BASE_CAT_N + 1))" "$((BASE_CAT_A + 1200))"

check "API-010 店舗名の部分一致で検索する" \
    200 GET '/api/v1/transactions?q=定食'
expect_meta "$((BASE_SHOP_N + 1))" "$((BASE_SHOP_A + 1200))"

check "API-010 メモの部分一致で検索する" \
    200 GET '/api/v1/transactions?q=現金'
expect_meta "$((BASE_NOTE_N + 1))" "$((BASE_NOTE_A + 1200))"

# meta は「条件に合致する総件数・総額」であり、そのページの件数ではない
check "API-010 ページネーション（limit=1。data は 1 件、meta は全体の件数・合計）" \
    200 GET '/api/v1/transactions?limit=1&offset=0'
expect "data の件数" 1 "$(data_count occurred_at)"
expect_meta "$((BASE_ALL_N + 2))" "$((BASE_ALL_A + 1780))"

check "API-013 金額とメモを修正する" \
    200 PUT "/api/v1/transactions/${TXN_ID}" \
    '{"amount_minor":1500,"note":"修正後のメモ"}'
expect "amount_minor" 1500 "$(json_value amount_minor <<<"$LAST_PAYLOAD")"

check "API-012 修正後に詳細を取得し、金額とメモが反映されていること" \
    200 GET "/api/v1/transactions/${TXN_ID}"
expect "amount_minor" 1500       "$(json_value amount_minor <<<"$LAST_PAYLOAD")"
expect "note"         "修正後のメモ" "$(json_value note <<<"$LAST_PAYLOAD")"

check "API-013 部分更新（note だけ null にし、金額は維持される）" \
    200 PUT "/api/v1/transactions/${TXN_ID}" '{"note":null}'
expect "amount_minor" 1500   "$(json_value amount_minor <<<"$LAST_PAYLOAD")"
expect "note"         "null" "$(json_value note <<<"$LAST_PAYLOAD")"

# --- 異常系 -------------------------------------------------------------

check "金額が 0 なら 400 INVALID_AMOUNT" \
    400 POST /api/v1/transactions \
    '{"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":0}'

check "occurred_at が無ければ 400 VALIDATION_ERROR" \
    400 POST /api/v1/transactions '{"amount_minor":100}'

check "occurred_at にタイムゾーンが無ければ 400 VALIDATION_ERROR" \
    400 POST /api/v1/transactions \
    '{"occurred_at":"2026-09-10 19:20:00","amount_minor":100}'

check "JPY 以外の通貨は 400 UNSUPPORTED_CURRENCY（為替レートの取得元が無いため）" \
    400 POST /api/v1/transactions \
    '{"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":100,"currency":"USD"}'

check "存在しないカテゴリを指定したら 404 CATEGORY_NOT_FOUND" \
    404 POST /api/v1/transactions \
    '{"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":100,"category_id":999999}'

check "存在しない決済手段を指定したら 404 PAYMENT_METHOD_NOT_FOUND" \
    404 POST /api/v1/transactions \
    '{"occurred_at":"2026-09-10T19:20:00+09:00","amount_minor":100,"payment_method_id":999999}'

check "from が to より後なら 400 INVALID_DATE_RANGE" \
    400 GET '/api/v1/transactions?from=2026-09-10&to=2026-09-01'

check "limit が 100 を超えたら 400 LIMIT_TOO_LARGE" \
    400 GET '/api/v1/transactions?limit=101'

check "存在しない取引の取得は 404 TRANSACTION_NOT_FOUND" \
    404 GET /api/v1/transactions/999999

check "存在しない取引の更新は 404 TRANSACTION_NOT_FOUND" \
    404 PUT /api/v1/transactions/999999 '{"amount_minor":100}'

check_noauth "認証なしのリクエストは 401 INVALID_TOKEN" \
    401 /api/v1/transactions

# --- 削除と永続化の確認 -------------------------------------------------

# 削除の前後を比べられるよう、修正後（1500 円）の合計をここで記録する
check "API-010 削除前の一覧（修正後の 1500 円が合計に入っている）" \
    200 GET /api/v1/transactions
expect_meta "$((BASE_ALL_N + 2))" "$((BASE_ALL_A + 2080))"

check "API-014 取引を削除する（論理削除）" \
    204 DELETE "/api/v1/transactions/${TXN_ID}"

check "削除後に詳細を取得すると 404（クライアントからは存在しない）" \
    404 GET "/api/v1/transactions/${TXN_ID}"

check "API-010 削除後の一覧（1500 円が件数からも合計からも除かれている）" \
    200 GET /api/v1/transactions
expect_meta "$((BASE_ALL_N + 1))" "$((BASE_ALL_A + 580))"

check "削除済みの取引を再度削除すると 404" \
    404 DELETE "/api/v1/transactions/${TXN_ID}"

# --- 後始末 -------------------------------------------------------------

check "2 件目も削除する（繰り返し実行できるようにするための後始末）" \
    204 DELETE "/api/v1/transactions/${TXN_ID_2}"

# 異常系のリクエストが取引を作っていないことも、ここで同時に確認できる
check "API-010 実行開始時点の件数・合計に戻っていること" \
    200 GET /api/v1/transactions
expect_meta "$BASE_ALL_N" "$BASE_ALL_A"

summary
