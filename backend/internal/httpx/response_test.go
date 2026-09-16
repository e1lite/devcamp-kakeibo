package httpx_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
)

func TestWriteData(t *testing.T) {
	t.Parallel()

	t.Run("meta が nil なら meta キーを含めない", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		httpx.WriteData(rec, http.StatusOK, map[string]int{"id": 1}, nil)

		var raw map[string]json.RawMessage
		if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
			t.Fatalf("JSON パースに失敗しました: %v", err)
		}

		if _, ok := raw["data"]; !ok {
			t.Error("data キーがありません")
		}
		if _, ok := raw["meta"]; ok {
			t.Error("meta を渡していないのに meta キーが含まれています")
		}
	})

	t.Run("meta を渡せば meta キーを含める", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		httpx.WriteData(rec, http.StatusOK,
			[]int{1, 2},
			map[string]int{"total": 2},
		)

		var body struct {
			Data []int `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("JSON パースに失敗しました: %v", err)
		}

		if body.Meta.Total != 2 {
			t.Errorf("meta.total: got %d, want 2", body.Meta.Total)
		}
	})
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        *httpx.APIError
		wantStatus int
		wantCode   string
	}{
		{
			name:       "401",
			err:        httpx.Unauthorized(httpx.CodeInvalidToken, "トークンが不正です"),
			wantStatus: http.StatusUnauthorized,
			wantCode:   "INVALID_TOKEN",
		},
		{
			name:       "403 は他ユーザのリソースで返す",
			err:        httpx.Forbidden("他ユーザのリソースです"),
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "400 VALIDATION_ERROR",
			err:        httpx.ValidationError("必須項目が不足しています"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "VALIDATION_ERROR",
		},
		{
			name:       "409",
			err:        httpx.Conflict("ALREADY_MERGED", "すでにマージ済みです"),
			wantStatus: http.StatusConflict,
			wantCode:   "ALREADY_MERGED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			httpx.WriteError(rec, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("ステータスコード: got %d, want %d", rec.Code, tt.wantStatus)
			}

			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("JSON パースに失敗しました: %v", err)
			}

			if body.Error.Code != tt.wantCode {
				t.Errorf("error.code: got %q, want %q", body.Error.Code, tt.wantCode)
			}
			if body.Error.Message == "" {
				t.Error("error.message が空です")
			}
		})
	}
}

// errDBConnection は内部エラーの扱いを確認するためのダミー。
var errDBConnection = errors.New("dial tcp 127.0.0.1:5433: connection refused")

func TestInternal(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	apiErr := httpx.Internal(errDBConnection)
	httpx.WriteError(rec, apiErr)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ステータスコード: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	// 内部エラーの詳細（接続先など）はクライアントに見せない
	if got := rec.Body.String(); strings.Contains(got, "127.0.0.1") {
		t.Errorf("内部エラーの詳細がレスポンスに含まれています: %s", got)
	}

	// 一方で、ログに出せるよう errors.Is で原因を辿れる必要がある
	if !errors.Is(apiErr, errDBConnection) {
		t.Error("Unwrap で原因エラーを辿れません")
	}
}
