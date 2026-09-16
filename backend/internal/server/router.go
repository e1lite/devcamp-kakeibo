// Package server はルーティングと HTTP サーバの組み立てを行う。
package server

import (
	"net/http"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/handler"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
)

// Deps はルータが必要とする依存をまとめる。
type Deps struct {
	Version string
	Ping    handler.PingFunc
}

// NewRouter はルーティングを組み立てる。
//
// Go 1.22 以降の http.ServeMux は "GET /api/v1/transactions/{id}" 形式の
// メソッド + パスパターンをサポートするため、標準ライブラリだけで構成できる。
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	// --- 認証不要 ---
	// API-001 ヘルスチェック
	mux.Handle("GET /health", handler.NewHealth(deps.Version, deps.Ping))

	// TODO(Step 3): 以下を実装する（API 仕様書 5.1 の「Step 3（コア）」）
	//   API-002 GET  /api/v1/auth/google
	//   API-003 GET  /api/v1/auth/google/callback
	//   API-004 GET  /api/v1/auth/me
	//   API-005 POST /api/v1/auth/logout
	//   API-010 GET  /api/v1/transactions
	//   API-011 POST /api/v1/transactions
	//   API-012 GET  /api/v1/transactions/{id}
	//   API-013 PUT  /api/v1/transactions/{id}
	//   API-014 DELETE /api/v1/transactions/{id}
	//   API-019 GET  /api/v1/categories
	//   API-020 POST /api/v1/categories
	//   API-021 PUT / DELETE /api/v1/categories/{id}

	// ServeMux は未登録パスに素の 404 を返すため、
	// 共通のエラー形式に合わせたハンドラを置く。
	mux.HandleFunc("/", notFound)

	return chain(mux, recoverPanic, requestLogger)
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteError(w, httpx.NotFound(httpx.CodeNotFound, "エンドポイントが存在しません"))
}
