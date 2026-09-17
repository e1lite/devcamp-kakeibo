package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/server"
)

// ユーザ存在確認のスタブ。ミドルウェア単体を見たいので DB は使わない。
var (
	userFound        = func(context.Context, int64) (bool, error) { return true, nil }
	userMissing      = func(context.Context, int64) (bool, error) { return false, nil }
	userLookupFailed = func(context.Context, int64) (bool, error) {
		return false, errors.New("データベースに接続できません")
	}
)

// TestRequireAuth は認証が必要なエンドポイント（API-004）で
// Authorization ヘッダの扱いを確認する。
func TestRequireAuth(t *testing.T) {
	t.Parallel()

	issuer := auth.NewTokenIssuer("test-secret")

	validToken, err := issuer.Issue(7, time.Now())
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	expiredToken, err := issuer.Issue(7, time.Now().Add(-auth.TokenTTL-time.Minute))
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	tests := []struct {
		name   string
		header string
		// userExists が nil ならユーザは存在するものとして扱う
		userExists server.UserExistsFunc
		wantStatus int
		wantCode   string
	}{
		{
			name:       "有効なトークンなら通る",
			header:     "Bearer " + validToken,
			wantStatus: http.StatusOK,
		},
		{
			name:       "小文字の bearer でも通る",
			header:     "bearer " + validToken,
			wantStatus: http.StatusOK,
		},
		{
			name:       "ヘッダなしは INVALID_TOKEN",
			header:     "",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			name:       "Bearer 以外のスキームは INVALID_TOKEN",
			header:     "Basic dXNlcjpwYXNz",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			name:       "トークンが空なら INVALID_TOKEN",
			header:     "Bearer ",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			name:       "壊れたトークンは INVALID_TOKEN",
			header:     "Bearer not-a-jwt",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			// 期限切れは INVALID_TOKEN と区別する。
			// クライアントは TOKEN_EXPIRED なら再ログインへ誘導できる
			name:       "期限切れは TOKEN_EXPIRED",
			header:     "Bearer " + expiredToken,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "TOKEN_EXPIRED",
		},
		{
			// 署名が正しくてもユーザが消えていれば、そのトークンはもう使えない。
			// ここで弾かないと外部キー違反などで 500 になる
			name:       "ユーザが存在しなければ INVALID_TOKEN",
			header:     "Bearer " + validToken,
			userExists: userMissing,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			// 存在確認そのものが失敗したのはサーバ側の問題。
			// 401 を返すと利用者を不要な再ログインに誘導してしまう
			name:       "存在確認が失敗したら INTERNAL_ERROR",
			header:     "Bearer " + validToken,
			userExists: userLookupFailed,
			wantStatus: http.StatusInternalServerError,
			wantCode:   "INTERNAL_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			userExists := tt.userExists
			if userExists == nil {
				userExists = userFound
			}

			// 認証を通ったら 200 を返すだけのハンドラを保護する
			protected := server.RequireAuthForTest(issuer, userExists,
				func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("ステータスコード: got %d, want %d (body=%s)",
					rec.Code, tt.wantStatus, rec.Body.String())
			}

			if tt.wantCode == "" {
				return
			}

			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("JSON パースに失敗しました: %v", err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("error.code: got %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}

// TestUserIDFrom は認証を通ったハンドラがユーザ ID を受け取れることを確認する。
func TestUserIDFrom(t *testing.T) {
	t.Parallel()

	issuer := auth.NewTokenIssuer("test-secret")
	token, err := issuer.Issue(123, time.Now())
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	var gotUserID int64
	handler := server.RequireAuthForTest(issuer, userFound, func(_ http.ResponseWriter, r *http.Request) {
		id, ok := server.UserIDFrom(r.Context())
		if !ok {
			t.Error("コンテキストにユーザ ID がありません")
			return
		}
		gotUserID = id
	})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if gotUserID != 123 {
		t.Errorf("ユーザ ID: got %d, want 123", gotUserID)
	}
}
