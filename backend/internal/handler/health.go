// Package handler は HTTP ハンドラを提供する。
package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
)

// PingFunc はデータベースへの疎通確認を行う。
// テストから差し替えられるよう関数型にしている。
type PingFunc func(ctx context.Context) error

// Health は API-001 ヘルスチェックを扱う。認証不要。
type Health struct {
	version string
	ping    PingFunc
}

// NewHealth は Health ハンドラを生成する。
func NewHealth(version string, ping PingFunc) *Health {
	return &Health{version: version, ping: ping}
}

type healthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Database  string `json:"database"`
	CheckedAt string `json:"checked_at"`
}

// ServeHTTP は GET /health を処理する。
//
// プロセスが生きていても DB に繋がらなければリクエストは処理できない。
// 200 を返し続けると壊れたインスタンスにトラフィックが流れ続けるため、
// 疎通確認の結果を 200 の条件に含める（API 仕様書 API-001）。
func (h *Health) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// ヘルスチェックが詰まると監視側のタイムアウトを巻き込むため、短く打ち切る。
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.ping(ctx); err != nil {
		slog.Error("ヘルスチェックに失敗しました", "error", err)
		httpx.WriteError(w, httpx.NewAPIError(
			http.StatusServiceUnavailable,
			httpx.CodeDatabaseUnavailable,
			"データベースへ疎通できません",
		))
		return
	}

	httpx.WriteData(w, http.StatusOK, healthResponse{
		Status:    "ok",
		Version:   h.version,
		Database:  "ok",
		CheckedAt: time.Now().Format(time.RFC3339),
	}, nil)
}
