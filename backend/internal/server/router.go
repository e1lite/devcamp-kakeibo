// Package server はルーティングと HTTP サーバの組み立てを行う。
package server

import (
	"net/http"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/handler"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
)

// Deps はルータが必要とする依存をまとめる。
type Deps struct {
	Version string
	Ping    handler.PingFunc
	Tokens  *auth.TokenIssuer
	Auth    *handler.Auth
}

// authedHandler は認証済みユーザ ID を受け取るハンドラ。
type authedHandler func(w http.ResponseWriter, r *http.Request, userID int64)

// NewRouter はルーティングを組み立てる。
//
// Go 1.22 以降の http.ServeMux は "GET /api/v1/transactions/{id}" 形式の
// メソッド + パスパターンをサポートするため、標準ライブラリだけで構成できる。
func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	// --- 認証不要 ---
	mux.Handle("GET /health", handler.NewHealth(deps.Version, deps.Ping))        // API-001
	mux.HandleFunc("GET /api/v1/auth/google", deps.Auth.GoogleStart)             // API-002
	mux.HandleFunc("GET /api/v1/auth/google/callback", deps.Auth.GoogleCallback) // API-003

	// --- 認証必要 ---
	authed := func(h authedHandler) http.Handler {
		return requireAuth(deps.Tokens)(http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				userID, ok := UserIDFrom(r.Context())
				if !ok {
					// requireAuth を通っていれば必ず入っている。
					// ここに来るのはルーティングの組み立てミス
					httpx.WriteError(w, httpx.Unauthorized(
						httpx.CodeInvalidToken, "認証情報を取得できません"))
					return
				}
				h(w, r, userID)
			}))
	}

	mux.Handle("GET /api/v1/auth/me", authed(deps.Auth.Me))          // API-004
	mux.Handle("POST /api/v1/auth/logout", authed(deps.Auth.Logout)) // API-005

	// TODO(Step 3): 以下を実装する（API 仕様書 5.1 の「Step 3（コア）」）
	//   API-010 GET    /api/v1/transactions
	//   API-011 POST   /api/v1/transactions
	//   API-012 GET    /api/v1/transactions/{id}
	//   API-013 PUT    /api/v1/transactions/{id}
	//   API-014 DELETE /api/v1/transactions/{id}
	//   API-019 GET    /api/v1/categories
	//   API-020 POST   /api/v1/categories
	//   API-021 PUT / DELETE /api/v1/categories/{id}

	// ServeMux は未登録パスに素の 404 を返すため、
	// 共通のエラー形式に合わせたハンドラを置く。
	mux.HandleFunc("/", notFound)

	return chain(mux, recoverPanic, requestLogger)
}

func notFound(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteError(w, httpx.NotFound(httpx.CodeNotFound, "エンドポイントが存在しません"))
}
