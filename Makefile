BACKEND_DIR := backend

# ローカル開発用の接続先。docker-compose.yml のホスト側ポート 5433 に合わせる
export DATABASE_URL ?= postgres://kakeibo:kakeibo@localhost:5433/kakeibo?sslmode=disable
export APP_ENV      ?= local
export PORT         ?= 8080
export JWT_SECRET   ?= local-development-secret-do-not-use-in-production

# Google OAuth の認証情報。.env.local に書いて読み込ませる（コミットしないこと）
-include .env.local
export GOOGLE_CLIENT_ID
export GOOGLE_CLIENT_SECRET
export GOOGLE_REDIRECT_URL    ?= http://localhost:8080/api/v1/auth/google/callback
export DEFAULT_REDIRECT_URI   ?= http://localhost:5173/auth/callback
export ALLOWED_REDIRECT_URIS  ?= http://localhost:5173/auth/callback

.DEFAULT_GOAL := help

.PHONY: help
help: ## コマンド一覧を表示する
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

# --- データベース ---------------------------------------------------------

.PHONY: db-up
db-up: ## PostgreSQL を起動し、接続できるまで待つ
	docker compose up -d --wait db

.PHONY: db-down
db-down: ## PostgreSQL を停止する（データは残る）
	docker compose down

.PHONY: db-reset
db-reset: ## PostgreSQL を停止し、データも消す
	docker compose down -v

.PHONY: db-shell
db-shell: ## psql に入る
	docker compose exec db psql -U kakeibo -d kakeibo

# --- マイグレーション -----------------------------------------------------

.PHONY: migrate-up
migrate-up: ## マイグレーションを最新まで適用する
	cd $(BACKEND_DIR) && go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: ## マイグレーションを 1 つ戻す
	cd $(BACKEND_DIR) && go run ./cmd/migrate down 1

.PHONY: migrate-version
migrate-version: ## 現在のマイグレーションバージョンを表示する
	cd $(BACKEND_DIR) && go run ./cmd/migrate version

# --- アプリケーション -----------------------------------------------------

.PHONY: run
run: ## API サーバを起動する
	cd $(BACKEND_DIR) && go run ./cmd/api

.PHONY: build
build: ## API サーバをビルドする
	cd $(BACKEND_DIR) && go build -o bin/api ./cmd/api

# --- 品質担保（提出物 ④ で出力を貼る対象） --------------------------------

.PHONY: lint
lint: ## 静的解析（go vet + golangci-lint）
	cd $(BACKEND_DIR) && go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 \
		|| { echo "golangci-lint が見つかりません。'make tools' を実行してください"; exit 1; }
	cd $(BACKEND_DIR) && golangci-lint run ./...

.PHONY: test
test: ## 自動テスト（統合テストを含む。DB が起動していること）
	cd $(BACKEND_DIR) && go test ./... -count=1 -cover

.PHONY: test-unit
test-unit: ## 単体テストのみ（DB 不要）
	cd $(BACKEND_DIR) && go test -short ./... -count=1 -cover

.PHONY: check
check: lint test ## 静的解析と自動テストをまとめて実行する

.PHONY: fmt
fmt: ## コードを整形する
	cd $(BACKEND_DIR) && go fmt ./...

# --- セットアップ ---------------------------------------------------------

.PHONY: tools
tools: ## 開発に必要なツールを入れる（golangci-lint v2）
	brew install golangci-lint

.PHONY: setup
setup: db-up migrate-up ## DB を起動してマイグレーションまで済ませる
	@echo "セットアップ完了。'make run' で API サーバを起動できます。"
