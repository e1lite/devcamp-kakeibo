# syntax=docker/dockerfile:1
#
# フロントエンド（Next.js）のイメージ。マルチステージビルドで、ビルドにだけ使う node_modules（約 500 MB）と
# ソースを実行用のイメージに持ち込まない。実行用のイメージには Next.js の standalone 一式だけを入れる
# （next.config.ts の output: "standalone"。Step 8 で EC2 に配ったものと同じ中身）。
#
# ビルド: docker compose build web

# Step 8 と同じ Node.js 24 LTS（24.22.2）。Alpine 版で、実行用のイメージを小さくする。
# 3 つのステージで同じイメージを使い、ネイティブモジュール（sharp など）の libc（musl）をそろえる
ARG NODE_IMAGE=node:24.22.2-alpine3.24

# ---- 1. 依存の取得 ---------------------------------------------------------------
# package.json / package-lock.json が変わらなければ、この層のキャッシュが使われ npm ci を省ける
FROM ${NODE_IMAGE} AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --no-audit --no-fund

# ---- 2. ビルド -------------------------------------------------------------------
FROM ${NODE_IMAGE} AS build
WORKDIR /app
ENV NEXT_TELEMETRY_DISABLED=1
# /api/* の転送先（next.config.ts の rewrites）。rewrites はビルド時に決まり、
# 実行時に環境変数で変えられないため、ビルド引数で渡す。
# 既定値は Docker Compose のネットワーク内での API のサービス名
ARG API_ORIGIN=http://api:8080
ENV API_ORIGIN=${API_ORIGIN}
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# public/ は今は空でリポジトリに無いため、作っておく（次のステージの COPY が失敗しないように）
RUN mkdir -p public && npm run build

# ---- 3. 実行用 -------------------------------------------------------------------
FROM ${NODE_IMAGE} AS runtime
WORKDIR /app
# HOSTNAME：Docker はコンテナ ID を HOSTNAME に入れる。Next.js の standalone はこの値のアドレスで
# 待ち受けるため、0.0.0.0（すべてのアドレス）にしておく
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
# standalone の server.js は public と .next/static を含まないため、横に置く（Next.js の output の説明どおり）
COPY --from=build --chown=node:node /app/.next/standalone ./
COPY --from=build --chown=node:node /app/.next/static ./.next/static
COPY --from=build --chown=node:node /app/public ./public

# イメージに用意されているユーザ node（UID 1000）で動かす。root で動かさない
USER node
EXPOSE 3000
CMD ["node", "server.js"]
