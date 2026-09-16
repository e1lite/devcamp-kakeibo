package server

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder は書き出されたステータスコードを記録する ResponseWriter。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// requestLogger はリクエストの結果を 1 行で記録する。
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// recoverPanic はハンドラ内の panic を 500 に変換する。
// panic でプロセスごと落とすと、他のリクエストまで巻き添えになるため。
func recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic が発生しました", "panic", rec, "path", r.URL.Path)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusInternalServerError)

				const body = `{"error":{"code":"INTERNAL_ERROR","message":"サーバ内部エラーが発生しました"}}`
				if _, err := w.Write([]byte(body)); err != nil {
					// ヘッダを書き出した後なので他に手段がない。記録だけ残す
					slog.Error("panic レスポンスの書き出しに失敗しました", "error", err)
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// chain はミドルウェアを適用する。先に渡したものが外側になる。
func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}
