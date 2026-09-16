#!/usr/bin/env bash
#
# 動作確認スクリプトの共通処理。各スクリプトから source して使う。

BASE_URL="${BASE_URL:-http://localhost:8080}"
TOKEN_FILE="${TOKEN_FILE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.token}"

pass=0
fail=0

# load_token [ARG] — 引数・環境変数・保存済みファイルの順にトークンを解決する
load_token() {
    if [ $# -ge 1 ] && [ -n "$1" ]; then
        case "$1" in
            *access_token=*) TOKEN=$(sed -n 's/.*access_token=\([^&]*\).*/\1/p' <<<"$1") ;;
            *)               TOKEN=$1 ;;
        esac
        printf '%s' "$TOKEN" > "$TOKEN_FILE"
        echo "トークンを ${TOKEN_FILE} に保存しました（次回から引数なしで実行できます）"
    fi

    if [ -z "${TOKEN:-}" ] && [ -f "$TOKEN_FILE" ]; then
        TOKEN=$(cat "$TOKEN_FILE")
    fi

    if [ -z "${TOKEN:-}" ]; then
        cat >&2 <<EOF
トークンがありません。

1. ブラウザで ${BASE_URL}/api/v1/auth/google を開いてログインする
2. リダイレクト先（読み込めないページ）のアドレスバーを ⌘L → ⌘C でコピーする
3. その URL を引数に渡して実行する

   $0 'コピーした URL'

トークンは .token に保存され、次回から引数なしで実行できます。
EOF
        exit 1
    fi
}

# req METHOD PATH [BODY] — レスポンスボディと HTTP ステータスを出力する
req() {
    local method=$1 path=$2 body=${3:-}
    local args=(-sS -X "$method" -H "Authorization: Bearer ${TOKEN}" -w '\n%{http_code}')
    if [ -n "$body" ]; then
        args+=(-H 'Content-Type: application/json' -d "$body")
    fi
    curl "${args[@]}" "${BASE_URL}${path}"
}

# check DESCRIPTION EXPECTED_STATUS METHOD PATH [BODY]
check() {
    local description=$1 expected=$2 method=$3 path=$4 body=${5:-}

    local response status payload
    response=$(req "$method" "$path" "$body")
    status=$(tail -n1 <<<"$response")
    payload=$(sed '$d' <<<"$response")

    echo "--------------------------------------------------------------------"
    echo "# ${description}"
    echo "\$ curl -X ${method} ${BASE_URL}${path}${body:+ -d '${body}'}"
    echo "${payload}"

    if [ "$status" = "$expected" ]; then
        echo "=> ${status} OK"
        pass=$((pass + 1))
    else
        echo "=> ${status} NG（期待値: ${expected}）"
        fail=$((fail + 1))
    fi
}

# check_noauth DESCRIPTION EXPECTED_STATUS PATH — Authorization ヘッダを付けずに呼ぶ
check_noauth() {
    local description=$1 expected=$2 path=$3

    local response status payload
    response=$(curl -sS -w '\n%{http_code}' "${BASE_URL}${path}")
    status=$(tail -n1 <<<"$response")
    payload=$(sed '$d' <<<"$response")

    echo "--------------------------------------------------------------------"
    echo "# ${description}"
    echo "\$ curl ${BASE_URL}${path}    # Authorization ヘッダなし"
    echo "${payload}"

    if [ "$status" = "$expected" ]; then
        echo "=> ${status} OK"
        pass=$((pass + 1))
    else
        echo "=> ${status} NG（期待値: ${expected}）"
        fail=$((fail + 1))
    fi
}

# check_with_token DESCRIPTION EXPECTED_STATUS METHOD PATH TOKEN_VALUE
# 保存済みのトークンではなく、指定したトークンで呼ぶ（壊れたトークンの確認に使う）
check_with_token() {
    local description=$1 expected=$2 method=$3 path=$4 token=$5

    local response status payload
    response=$(curl -sS -X "$method" -H "Authorization: Bearer ${token}" \
        -w '\n%{http_code}' "${BASE_URL}${path}")
    status=$(tail -n1 <<<"$response")
    payload=$(sed '$d' <<<"$response")

    echo "--------------------------------------------------------------------"
    echo "# ${description}"
    echo "\$ curl -X ${method} -H 'Authorization: Bearer ${token}' ${BASE_URL}${path}"
    echo "${payload}"

    if [ "$status" = "$expected" ]; then
        echo "=> ${status} OK"
        pass=$((pass + 1))
    else
        echo "=> ${status} NG（期待値: ${expected}）"
        fail=$((fail + 1))
    fi
}

# json_value KEY — 標準入力の JSON から "KEY":値 を 1 つ取り出す（数値・文字列）
json_value() {
    local key=$1
    tr '}' '\n' | grep -o "\"${key}\":[^,]*" | head -1 | cut -d: -f2- | tr -d '"'
}

# summary — 集計結果を出力し、失敗があれば終了コード 1 を返す
summary() {
    echo "===================================================================="
    echo " 成功: ${pass} 件 / 失敗: ${fail} 件"
    echo "===================================================================="
    [ "$fail" -eq 0 ]
}

# banner TITLE
banner() {
    echo "===================================================================="
    echo " $1  $(date '+%Y-%m-%d %H:%M:%S')"
    echo "===================================================================="
}
