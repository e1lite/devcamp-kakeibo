package server

import (
	"net/http"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
)

// RequireAuthForTest は非公開の requireAuth をテストから使えるようにする。
//
// 認証ミドルウェアだけを検証したいが、NewRouter は service.Auth 経由で
// データベースを要求する。ミドルウェアの責務（Authorization ヘッダの解釈）は
// DB と無関係なので、ここだけ切り出してテストする。
func RequireAuthForTest(tokens *auth.TokenIssuer, next http.HandlerFunc) http.Handler {
	return requireAuth(tokens)(next)
}
