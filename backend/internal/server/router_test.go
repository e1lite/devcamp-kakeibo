package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/handler"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/server"
)

// newTestRouter はルーティングの検証用にルータを組み立てる。
//
// 認証まわりのエンドポイントはここでは叩かないため、
// service.Auth（DB を要求する）は nil のままでよい。
// ルート登録時にメソッド値を作るだけで、呼ばれない限り参照されない。
func newTestRouter() http.Handler {
	return server.NewRouter(server.Deps{
		Version:    "test",
		Ping:       func(context.Context) error { return nil },
		Tokens:     auth.NewTokenIssuer("test-secret"),
		UserExists: func(context.Context, int64) (bool, error) { return true, nil },
		Auth:       handler.NewAuth(nil),
	})
}

func TestRouter_Health(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouter_UnknownPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{name: "未登録のパス", method: http.MethodGet, path: "/api/v1/unknown"},
		// ServeMux はパスが一致してもメソッドが違えば 405 を返すが、
		// ここでは共通のエラー形式で返ることだけを確認する
		{name: "ルート", method: http.MethodGet, path: "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			newTestRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s: got %d, want %d", tt.method, tt.path, rec.Code, http.StatusNotFound)
			}

			// 素の 404 ではなく、API 仕様書 2 章のエラー形式で返ること
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("JSON パースに失敗しました: %v", err)
			}
			if body.Error.Code != "NOT_FOUND" {
				t.Errorf("error.code: got %q, want %q", body.Error.Code, "NOT_FOUND")
			}
		})
	}
}

func TestRouter_RecoverPanic(t *testing.T) {
	t.Parallel()

	// panic するハンドラでもプロセスを落とさず 500 を返すこと
	router := server.NewRouter(server.Deps{
		Version: "test",
		Ping:    func(context.Context) error { panic("ping が panic した") },
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic 時: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
