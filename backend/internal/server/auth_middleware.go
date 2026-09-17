package server

import (
	"context"
	"errors"
	"log/slog"
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

// UserExistsFunc は userID のユーザが存在するかを返す。
//
// ミドルウェアがリポジトリに直接依存しないよう関数として受け取る
// （handler.PingFunc と同じ考え方）。
type UserExistsFunc func(ctx context.Context, userID int64) (bool, error)

// requireAuth は Authorization ヘッダの Bearer トークンを検証する。
//
// ヘルスチェックと OAuth の開始・コールバックを除く、
// すべてのリクエストに Bearer トークンが必要（API 仕様書 3 章）。
func requireAuth(tokens *auth.TokenIssuer, userExists UserExistsFunc) func(http.Handler) http.Handler {
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

			// 署名と有効期限が正しくても、ユーザが消えていればその
			// トークンはもう使えない。ここで弾かないと userID だけが
			// 下流に渡り、同じトークンでもエンドポイントごとに
			// 404 / 空配列 / 外部キー違反の 500 とばらばらの応答になる
			exists, err := userExists(r.Context(), userID)
			if err != nil {
				slog.Error("ユーザの存在確認に失敗しました", "error", err)
				httpx.WriteError(w, httpx.Internal(err))
				return
			}
			if !exists {
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
