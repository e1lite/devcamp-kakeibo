package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
)

// contextKey はコンテキストのキー。他パッケージのキーと衝突しないよう独自型にする。
type contextKey struct{ name string }

var userIDKey = &contextKey{name: "userID"}

// UserIDFrom はコンテキストから認証済みユーザの ID を取り出す。
// requireAuth を通っていないハンドラでは ok が false になる。
func UserIDFrom(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDKey).(int64)
	return userID, ok
}

// requireAuth は Authorization ヘッダの Bearer トークンを検証する。
//
// ヘルスチェックと OAuth の開始・コールバックを除く、
// すべてのリクエストに Bearer トークンが必要（API 仕様書 3 章）。
func requireAuth(tokens *auth.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				httpx.WriteError(w, httpx.Unauthorized(
					httpx.CodeInvalidToken, "Authorization ヘッダが不正です"))
				return
			}

			userID, err := tokens.Verify(token)
			if err != nil {
				// 期限切れとそれ以外を区別する。
				// クライアントは TOKEN_EXPIRED なら再ログインへ誘導できる
				if errors.Is(err, auth.ErrTokenExpired) {
					httpx.WriteError(w, httpx.Unauthorized(
						httpx.CodeTokenExpired, "トークンの有効期限が切れています"))
					return
				}
				httpx.WriteError(w, httpx.Unauthorized(
					httpx.CodeInvalidToken, "トークンが不正です"))
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken は Authorization ヘッダから Bearer トークンを取り出す。
func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "

	header := r.Header.Get("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
