#!/usr/bin/env bash
#
# Step 3 のコアエンドポイント全体の動作確認。
# 提出物「API の動作確認」に貼る出力はこのスクリプトで一括して取得する。
#
# 使い方:
#   make run                            別のターミナルで API サーバを起動
#   ./scripts/verify-all.sh             .token に保存されたトークンを使う
#   ./scripts/verify-all.sh '<URL>'     初回はログイン後のリダイレクト URL を渡す
#
#   # ファイルに保存する場合
#   ./scripts/verify-all.sh > verify-output.txt 2>&1
#
set -uo pipefail

cd "$(dirname "$0")/.."

# shellcheck source=./lib.sh
source ./scripts/lib.sh

load_token "${1:-}"

echo "===================================================================="
echo " Step 3 動作確認  $(date '+%Y-%m-%d %H:%M:%S')"
echo " ベース URL: ${BASE_URL}"
echo "===================================================================="
echo

failed=0

echo "### 1. ヘルスチェックと認証（API-001〜005）"
echo
./scripts/verify-auth.sh || failed=1
echo

echo "### 2. カテゴリ CRUD（API-019 / 020 / 021）"
echo
./scripts/verify-categories.sh || failed=1
echo

echo "### 3. 取引 CRUD（API-010〜014）"
echo
./scripts/verify-transactions.sh || failed=1
echo

echo "===================================================================="
if [ "$failed" -eq 0 ]; then
    echo " すべての動作確認に成功しました"
else
    echo " 失敗した確認があります。上のログを確認してください"
fi
echo "===================================================================="

exit "$failed"
