#!/usr/bin/env bash
#
# API-001〜005（ヘルスチェックと認証）の動作確認。
#
# API-002 / API-003（Google OAuth の開始とコールバック）はブラウザでの
# 同意操作が必要なため、ここでは自動化しない。
# ログインが成功していること自体は、取得できたトークンで API-004 が
# 200 を返すことによって確認する。
#
# 使い方:
#   make run                             別のターミナルで API サーバを起動
#   ./scripts/verify-auth.sh             .token に保存されたトークンを使う
#   ./scripts/verify-auth.sh '<URL>'     初回はログイン後のリダイレクト URL を渡す
#
set -uo pipefail

# shellcheck source=./lib.sh
source "$(dirname "$0")/lib.sh"

load_token "${1:-}"

banner "ヘルスチェックと認証 動作確認"

# --- 正常系 -------------------------------------------------------------

check_noauth "API-001 ヘルスチェック（認証不要。DB への疎通も確認する）" \
    200 /health

check "API-002 Google OAuth 開始（302 で Google の認可画面へリダイレクト）" \
    302 GET /api/v1/auth/google

check "API-004 現在のユーザ取得（ログインが成功していることの確認）" \
    200 GET /api/v1/auth/me

check "API-005 ログアウト" \
    204 POST /api/v1/auth/logout

# --- 異常系 -------------------------------------------------------------

check_noauth "Authorization ヘッダが無ければ 401 INVALID_TOKEN" \
    401 /api/v1/auth/me

check_with_token "壊れたトークンは 401 INVALID_TOKEN" \
    401 GET /api/v1/auth/me "not-a-jwt"

check "許可リストにない redirect_uri は 400 INVALID_REDIRECT_URI" \
    400 GET '/api/v1/auth/google?redirect_uri=https://example.com/steal'

check "state が無いコールバックは 400 VALIDATION_ERROR" \
    400 GET '/api/v1/auth/google/callback?code=dummy'

check "未発行の state は 400 INVALID_STATE（CSRF 対策）" \
    400 GET '/api/v1/auth/google/callback?code=dummy&state=never-issued'

# ブラウザからの遷移では JSON を返さず、フロントのログイン処理画面へエラーコードを渡す
# （Step 4 で変更、Step 6 で実装）。state を検証できないため、既定の戻り先へ戻る
DEFAULT_REDIRECT_URI="${DEFAULT_REDIRECT_URI:-http://localhost:5173/auth/callback}"

check_redirect "ブラウザで未発行の state なら、フロントへ #error=INVALID_STATE を渡す" \
    '/api/v1/auth/google/callback?code=dummy&state=never-issued' \
    "${DEFAULT_REDIRECT_URI}#error=INVALID_STATE"

check_redirect "ブラウザで同意画面を拒否したら、フロントへ #error=ACCESS_DENIED を渡す" \
    '/api/v1/auth/google/callback?error=access_denied&state=never-issued' \
    "${DEFAULT_REDIRECT_URI}#error=ACCESS_DENIED"

check_noauth "存在しないエンドポイントは共通のエラー形式で 404" \
    404 /api/v1/no-such-endpoint

summary
