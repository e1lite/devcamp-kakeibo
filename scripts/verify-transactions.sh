#!/usr/bin/env bash
#
# API-010〜014（取引 CRUD）の動作確認。
#
# 判定基準:
#   - 正常系のリクエストで期待するレスポンスが返ること
#   - 異常系のリクエストで想定するエラーレスポンスが返ること
#   - 作成・更新・削除の後に取得系を呼び、データが永続化されていること
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

check "API-010 取引一覧（初期状態）" \
    200 GET /api/v1/transactions

check "API-011 取引を手動登録する（店舗名は新規作成される）" \
    201 POST /api/v1/transactions \
    "{\"occurred_at\":\"2026-09-10T19:20:00+09:00\",\"amount_minor\":1200,\"merchant_name\":\"近所の定食屋\",\"category_id\":${CATEGORY_ID},\"note\":\"現金\"}"

TXN_ID=$(req GET /api/v1/transactions | sed '$d' \
    | tr '}' '\n' | grep '"amount_minor":1200' | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
if [ -z "$TXN_ID" ]; then
    echo "作成した取引の ID を取得できませんでした" >&2
    exit 1
fi
echo
echo "作成された取引 ID: ${TXN_ID}"

check "API-012 作成後に詳細を取得し、永続化されていることを確認する" \
    200 GET "/api/v1/transactions/${TXN_ID}"

check "API-011 2 件目を別の日付で登録する" \
    201 POST /api/v1/transactions \
    '{"occurred_at":"2026-09-01T12:00:00+09:00","amount_minor":580,"merchant_name":"セブン-イレブン渋谷店"}'

check "API-010 一覧（2 件・合計 1780 円が meta に入ること）" \
    200 GET /api/v1/transactions

check "API-010 期間で絞り込む（9/10 のみ）" \
    200 GET '/api/v1/transactions?from=2026-09-10&to=2026-09-10'

check "API-010 カテゴリで絞り込む" \
    200 GET "/api/v1/transactions?category_id=${CATEGORY_ID}"

check "API-010 店舗名の部分一致で検索する" \
    200 GET '/api/v1/transactions?q=定食'

check "API-010 メモの部分一致で検索する" \
    200 GET '/api/v1/transactions?q=現金'

check "API-010 ページネーション（limit=1）" \
    200 GET '/api/v1/transactions?limit=1&offset=0'

check "API-013 金額とメモを修正する" \
    200 PUT "/api/v1/transactions/${TXN_ID}" \
    '{"amount_minor":1500,"note":"修正後のメモ"}'

check "API-012 修正後に詳細を取得し、is_user_edited が true になっていること" \
    200 GET "/api/v1/transactions/${TXN_ID}"

check "API-013 部分更新（note だけ null にし、金額は維持される）" \
    200 PUT "/api/v1/transactions/${TXN_ID}" '{"note":null}'

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

check "API-014 取引を削除する（論理削除）" \
    204 DELETE "/api/v1/transactions/${TXN_ID}"

check "削除後に詳細を取得すると 404（クライアントからは存在しない）" \
    404 GET "/api/v1/transactions/${TXN_ID}"

check "API-010 削除後の一覧（1 件・合計 580 円。削除分は合計にも含まれない）" \
    200 GET /api/v1/transactions

check "削除済みの取引を再度削除すると 404" \
    404 DELETE "/api/v1/transactions/${TXN_ID}"

summary
