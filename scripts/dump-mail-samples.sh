#!/usr/bin/env bash
#
# 取り込んだメール（email_messages）を mail-samples/messages.jsonl に書き出す。
# カード会社の利用通知の形（どの行に金額・店舗があるか）を見て、抽出のテンプレートを作るために使う。
#
# mail-samples/ は .gitignore に入っている。個人のメールを含むため、書き出したファイルはコミットしない。
#
# 使い方:
#   ./scripts/dump-mail-samples.sh          新しい順に 30 通
#   ./scripts/dump-mail-samples.sh 100      件数を指定する
#
set -euo pipefail

cd "$(dirname "$0")/.."

limit="${1:-30}"
if ! [[ "$limit" =~ ^[0-9]+$ ]]; then
    echo "件数は整数で指定してください" >&2
    exit 1
fi

mkdir -p mail-samples
out=mail-samples/messages.jsonl

# 1 通を 1 行の JSON にする（本文の改行は JSON の \n になる）
docker compose exec -T db psql -U kakeibo -d kakeibo -At -c "
    SELECT json_build_object(
        'id', id,
        'from_address', from_address,
        'from_name', from_name,
        'subject', subject,
        'received_at', received_at,
        'body_text', body_text,
        'body_html', body_html
    )
    FROM email_messages
    ORDER BY received_at DESC
    LIMIT ${limit}
" > "$out"

echo "$(wc -l < "$out" | tr -d ' ') 通を ${out} に書き出しました"
