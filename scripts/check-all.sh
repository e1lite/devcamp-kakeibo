#!/usr/bin/env bash
#
# バックエンドとフロントエンドの静的解析・自動テストをまとめて実行する。
# 提出物「静的解析・自動テスト」に貼る出力はこのスクリプトで一括して取得する。
#
# 前提:
#   - PostgreSQL が起動していること（make setup。バックエンドの統合テストが接続する）
#   - frontend で npm ci を済ませていること
#
# 使い方:
#   ./scripts/check-all.sh
#   ./scripts/check-all.sh > docs/step6-check-output.txt 2>&1    # ファイルに保存する場合
#
set -uo pipefail

cd "$(dirname "$0")/.."

failed=0

echo "===================================================================="
echo " 静的解析・自動テスト  $(date '+%Y-%m-%d %H:%M:%S')"
echo "===================================================================="
echo

echo "### 1. バックエンド（go vet + golangci-lint + go test）"
echo
echo "\$ make check"
make check || failed=1
echo

echo "### 2. フロントエンド（tsc + ESLint + Vitest）"
echo
echo "\$ cd frontend && npm run check"
# ファイルに保存したときに色の制御文字が混ざらないよう、色を付けない
(cd frontend && NO_COLOR=1 npm run check) || failed=1
echo

echo "===================================================================="
if [ "$failed" -eq 0 ]; then
    echo " すべて成功しました"
else
    echo " 失敗したものがあります。上のログを確認してください"
fi
echo "===================================================================="

exit "$failed"
