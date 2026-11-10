#!/usr/bin/env bash
# docs/ の構成図（SVG）を PNG にする。提出フォームに添付する画像を作るため。
#
#   ./scripts/export-diagram.sh docs/step8-infra-diagram.svg docs/zh/step8-infra-diagram.svg
#
# SVG と同じ場所に、同じ名前の .png を作る（2 倍の解像度）。
# Google Chrome のヘッドレスモードで描画するため、日本語・中国語のフォントは Mac に入っているものが使われる。
set -euo pipefail

CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
[ -x "$CHROME" ] || { echo "Google Chrome が見つかりません: $CHROME" >&2; exit 1; }
[ "$#" -gt 0 ] || { echo "使い方: $0 <svg> [<svg> ...]" >&2; exit 1; }

for svg in "$@"; do
  [ -f "$svg" ] || { echo "見つかりません: $svg" >&2; exit 1; }
  abs="$(cd "$(dirname "$svg")" && pwd)/$(basename "$svg")"
  png="${abs%.svg}.png"

  # ルート要素の width・height を読む（ウィンドウの大きさを図に合わせる）
  size="$(sed -n 's/.*<svg[^>]* width="\([0-9]*\)" height="\([0-9]*\)".*/\1,\2/p' "$abs" | head -n 1)"
  [ -n "$size" ] || { echo "width・height を読めません: $svg" >&2; exit 1; }

  profile="$(mktemp -d)"
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --no-first-run \
    --user-data-dir="$profile" --force-device-scale-factor=2 \
    --window-size="$size" --screenshot="$png" "file://$abs" >/dev/null 2>&1
  rm -rf "$profile"

  echo "==> $png"
done
