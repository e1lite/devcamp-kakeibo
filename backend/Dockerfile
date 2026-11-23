# syntax=docker/dockerfile:1
#
# API サーバ（Go）のイメージ。マルチステージビルドで、ビルドに使う Go の環境（約 300 MB）を
# 実行用のイメージに持ち込まない。実行用のイメージには、ビルドした 2 つのコマンドと migrations/ だけを入れる。
#
#   /app/api               API サーバ（既定のコマンド）。`/app/api healthcheck` でヘルスチェック
#   /app/migrate up        マイグレーション（Docker Compose では migrate サービスとして先に 1 回だけ動かす）
#
# ビルド: docker compose build api   （単体なら docker build -t kakeibo-api ./backend）

# ---- 1. ビルド用のステージ -------------------------------------------------------
# ローカル開発と同じ Go 1.27.1。$BUILDPLATFORM（ビルドするマシン）で動かし、
# $TARGETOS / $TARGETARCH 向けにクロスコンパイルする。Arm の Mac で amd64 のイメージを作るときも、
# Go のコンパイラをエミュレーションで動かさずに済む（Step 10・11 で AWS 向けに作るときのため）
FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine3.24 AS build
WORKDIR /src

# 依存の取得を先に行う。go.mod / go.sum が変わらなければ、この層はキャッシュが使われ、
# ソースを変えただけのビルドでは依存を取り直さない
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
# /health の version に出す値。docker compose build では APP_VERSION（未指定なら dev）を渡す
ARG VERSION=dev
# CGO_ENABLED=0 で C のライブラリに依存しない（静的リンクの）実行ファイルにする。
# そのため、実行用のイメージに libc が無くても動く
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
      go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
      go build -trimpath -ldflags "-s -w" -o /out/migrate ./cmd/migrate

# ---- 2. 実行用のステージ ---------------------------------------------------------
# distroless の static：シェル・パッケージマネージャを含まず、CA 証明書（Google の API を HTTPS で呼ぶ）と
# タイムゾーンのデータだけを持つ。攻撃に使える道具が無く、脆弱性の対象になるパッケージも少ない。
# :nonroot はユーザ nonroot（UID 65532）で動かす。root で動かさない
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app
# cmd/migrate は作業ディレクトリの migrations/ を読む
COPY migrations ./migrations
COPY --from=build /out/api /out/migrate ./

# TZ：コンテナの既定のタイムゾーンは UTC のため、API が返す日時が「...Z」になる
# （ホストで動かすと「+09:00」。どちらも同じ時刻だが、動かす場所で出力が変わってしまう）。
# API 仕様書の例と同じ +09:00 になるよう、イメージで日本時間に固定する。どこで動かしても同じになる
ENV APP_ENV=production \
    PORT=8080 \
    TZ=Asia/Tokyo
EXPOSE 8080
USER nonroot:nonroot

# exec 形式（JSON の配列）で書く。シェルを挟まないため API サーバが PID 1 になり、
# docker compose stop の SIGTERM を直接受け取ってグレースフルに停止できる
CMD ["/app/api"]
