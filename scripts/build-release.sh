#!/usr/bin/env bash
# 本番（AWS）に配る成果物を作る。
#
#   ./scripts/build-release.sh        API とフロントの両方
#   ./scripts/build-release.sh api    API だけ（Docker 不要）
#   ./scripts/build-release.sh web    フロントだけ（Docker が必要）
#
# 出力（dist/ はコミットしない）:
#   dist/api.tar.gz  API サーバ・マイグレーションのコマンド・migrations/・migrate.sh
#   dist/web.tar.gz  Next.js の standalone 一式と Node.js 本体
#
# 作った 2 つを S3 の kakeibo-artifacts-<アカウント ID>/releases/ にアップロードすると、
# 次に起動する EC2 がユーザーデータ（deploy/userdata-*.sh）で取得して動かす。
#
# EC2 は t4g（Arm / Graviton）のため、どちらも linux/arm64 向けに作る。
# フロントを Docker の中で作るのは、Mac で作った node_modules（darwin 用のネイティブモジュール）を
# Linux に持ち込まないため。Apple Silicon の Mac なら linux/arm64 のコンテナはエミュレーションなしで動く。
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
DIST="$ROOT/dist"
# フロントのビルドと本番の実行に使う Node.js。Next.js 16 は Node.js 20.9 以上が必要
NODE_IMAGE="node:22.12.0-bookworm-slim"

VERSION="$(git rev-parse --short HEAD)"
if ! git diff --quiet HEAD -- backend frontend deploy; then
  # コミットしていない変更を含む成果物だと分かるようにする
  VERSION="${VERSION}-dirty"
fi

# macOS の tar が拡張属性（._ ファイルなど）を入れないようにする。Linux の tar で展開するときの警告を防ぐ
export COPYFILE_DISABLE=1
MAC_TAR_FLAGS=()
if tar --version 2>/dev/null | grep -q bsdtar; then
  MAC_TAR_FLAGS=(--no-mac-metadata --no-xattrs)
fi

build_api() {
  local work="$DIST/api"
  echo "==> API をビルドします（linux/arm64, version=$VERSION）"
  rm -rf "$work"
  mkdir -p "$work"
  (
    cd backend
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
      go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$work/api" ./cmd/api
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
      go build -trimpath -ldflags "-s -w" -o "$work/migrate" ./cmd/migrate
  )
  # cmd/migrate は作業ディレクトリの migrations/ を読む
  cp -R backend/migrations "$work/migrations"
  cp deploy/migrate.sh "$work/migrate.sh"
  chmod +x "$work/migrate.sh"
  echo "$VERSION" >"$work/VERSION"
  tar "${MAC_TAR_FLAGS[@]}" -C "$work" -czf "$DIST/api.tar.gz" .
  echo "==> $DIST/api.tar.gz（$(du -h "$DIST/api.tar.gz" | cut -f1)）"
}

build_web() {
  echo "==> フロントをビルドします（Docker: $NODE_IMAGE, linux/arm64, version=$VERSION）"
  command -v docker >/dev/null 2>&1 || { echo "docker が見つかりません。Docker Desktop を起動してください" >&2; exit 1; }
  mkdir -p "$DIST"
  rm -f "$DIST/web.tar.gz"
  # ソースは読み取り専用でマウントし、コンテナの中にコピーしてからビルドする。
  # 手元の frontend/node_modules と .next を Linux 用で上書きしないため
  docker run --rm --platform linux/arm64 \
    -v "$ROOT/frontend:/src:ro" \
    -v "$DIST:/out" \
    -e NEXT_TELEMETRY_DISABLED=1 \
    -e VERSION="$VERSION" \
    "$NODE_IMAGE" bash -euo pipefail -c '
      mkdir -p /app /pkg
      tar -C /src \
        --exclude=./node_modules --exclude=./.next --exclude=./.claude \
        --exclude="./.env*" --exclude=./tsconfig.tsbuildinfo \
        -cf - . | tar -C /app -xf -
      cd /app
      npm ci --no-audit --no-fund
      npm run build
      # standalone の server.js は public と .next/static を含まないため、横に置く（Next.js の output の説明どおり）
      cp -a .next/standalone/. /pkg/
      mkdir -p /pkg/.next
      cp -a .next/static /pkg/.next/static
      if [ -d public ]; then cp -a public /pkg/public; fi
      # EC2 に Node.js を入れなくて済むよう、ビルドに使った Node.js 本体を同梱する（版をそろえる）
      cp "$(command -v node)" /pkg/node
      echo "$VERSION" > /pkg/VERSION
      tar -C /pkg -czf /out/web.tar.gz .
    '
  echo "==> $DIST/web.tar.gz（$(du -h "$DIST/web.tar.gz" | cut -f1)）"
}

case "${1:-all}" in
  api) build_api ;;
  web) build_web ;;
  all)
    build_api
    build_web
    ;;
  *)
    echo "使い方: $0 [api|web|all]" >&2
    exit 1
    ;;
esac

echo
echo "次は S3 の kakeibo-artifacts-<アカウント ID>/releases/ に api.tar.gz と web.tar.gz をアップロードしてください"
