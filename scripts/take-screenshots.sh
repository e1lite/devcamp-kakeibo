#!/usr/bin/env bash
# Step 8 の動作確認のスクリーンショットを撮る（公開 URL の画面を、ヘッドレスの Chrome で操作する）。
#
#   ./scripts/take-screenshots.sh
#
# 事前に、ログインしたときのトークンをリポジトリ直下の .token に置く（.gitignore 済み）:
#   1. Chrome で https://app.autokakeibo.com を開いてログインする
#   2. 開発者ツールのコンソールで copy(sessionStorage.getItem("kakeibo.access_token")) を実行する
#   3. ターミナルで pbpaste > .token
#
# 出力: docs/step8-screenshots/ の PNG（01〜11）と api-log.txt（画面の操作で呼ばれた API の記録）
# 画面上のメールアドレスは伏せ字にしてから撮る。テスト用に作った取引・カテゴリは最後に消す。
# puppeteer-core はプロジェクトの依存に入れず、~/.cache/kakeibo-screenshots に入れる（初回だけ npm install）。
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
TOKEN_FILE="${ROOT}/.token"
OUT_DIR="${ROOT}/docs/step8-screenshots"
PUPPETEER_DIR="${HOME}/.cache/kakeibo-screenshots"
PUPPETEER_VERSION="25.13.0"
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

[ -x "$CHROME" ] || { echo "Google Chrome が見つかりません: ${CHROME}" >&2; exit 1; }
[ -s "$TOKEN_FILE" ] || { echo ".token がありません。このスクリプトの冒頭の手順でトークンを置いてください" >&2; exit 1; }
# トークンは JWT（英数字・-・_ を . で 3 つに区切ったもの）。クリップボードの別の文字が入っていたら、ここで止める
if ! LC_ALL=C grep -Eq '^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$' "$TOKEN_FILE"; then
  echo ".token の中身がトークンの形になっていません。このスクリプトの冒頭の手順で、コピーし直してください" >&2
  exit 1
fi

if [ ! -d "${PUPPETEER_DIR}/node_modules/puppeteer-core" ]; then
  echo "==> puppeteer-core@${PUPPETEER_VERSION} を ${PUPPETEER_DIR} に入れます（初回だけ）"
  mkdir -p "$PUPPETEER_DIR"
  npm install --prefix "$PUPPETEER_DIR" --no-audit --no-fund --silent "puppeteer-core@${PUPPETEER_VERSION}"
fi

BASE_URL="${BASE_URL:-https://app.autokakeibo.com}" \
  OUT_DIR="$OUT_DIR" TOKEN_FILE="$TOKEN_FILE" PUPPETEER_DIR="$PUPPETEER_DIR" \
  node scripts/take-screenshots.mjs
echo "==> ${OUT_DIR}"
