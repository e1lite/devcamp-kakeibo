package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/handler"
)

func TestHealth_ServeHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		ping           handler.PingFunc
		wantStatus     int
		wantErrorCode  string
		wantDatabaseOK bool
	}{
		{
			name:           "DB に疎通できれば 200 を返す",
			ping:           func(context.Context) error { return nil },
			wantStatus:     http.StatusOK,
			wantDatabaseOK: true,
		},
		{
			name:          "DB に疎通できなければ 503 DATABASE_UNAVAILABLE を返す",
			ping:          func(context.Context) error { return errors.New("connection refused") },
			wantStatus:    http.StatusServiceUnavailable,
			wantErrorCode: "DATABASE_UNAVAILABLE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)

			handler.NewHealth("1.0.0", tt.ping).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("ステータスコード: got %d, want %d", rec.Code, tt.wantStatus)
			}

			if tt.wantErrorCode != "" {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				decodeJSON(t, rec, &body)

				if body.Error.Code != tt.wantErrorCode {
					t.Errorf("エラーコード: got %q, want %q", body.Error.Code, tt.wantErrorCode)
				}
				return
			}

			var body struct {
				Data struct {
					Status    string `json:"status"`
					Version   string `json:"version"`
					Database  string `json:"database"`
					CheckedAt string `json:"checked_at"`
				} `json:"data"`
			}
			decodeJSON(t, rec, &body)

			if body.Data.Status != "ok" {
				t.Errorf("status: got %q, want %q", body.Data.Status, "ok")
			}
			if body.Data.Version != "1.0.0" {
				t.Errorf("version: got %q, want %q", body.Data.Version, "1.0.0")
			}
			if tt.wantDatabaseOK && body.Data.Database != "ok" {
				t.Errorf("database: got %q, want %q", body.Data.Database, "ok")
			}

			// API 仕様書 2 章: 日時はタイムゾーンオフセットを必ず含む
			if _, err := time.Parse(time.RFC3339, body.Data.CheckedAt); err != nil {
				t.Errorf("checked_at が RFC3339 ではありません: %q (%v)", body.Data.CheckedAt, err)
			}
		})
	}
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()

	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Errorf("Content-Type: got %q", got)
	}
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("レスポンスの JSON パースに失敗しました: %v", err)
	}
}
