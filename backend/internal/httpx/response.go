// Package httpx は API 仕様書 2 章の共通レスポンス形式を組み立てる。
//
//	成功時: { "data": { }, "meta": { } }
//	エラー時: { "error": { "code": "ERROR_CODE", "message": "エラーメッセージ" } }
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type successEnvelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteData は成功レスポンスを書き出す。meta が不要な場合は nil を渡す。
func WriteData(w http.ResponseWriter, status int, data, meta any) {
	writeJSON(w, status, successEnvelope{Data: data, Meta: meta})
}

// WriteNoContent は 204 を返す。ボディは書き出さない。
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteError は APIError をエラーレスポンスとして書き出す。
func WriteError(w http.ResponseWriter, err *APIError) {
	writeJSON(w, err.Status, errorEnvelope{
		Error: errorBody{Code: err.Code, Message: err.Message},
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		// ヘッダを書き出した後なのでステータスは変えられない。
		// クライアントには壊れた JSON が届くため、サーバ側には必ず記録する。
		slog.Error("レスポンスの書き出しに失敗しました", "error", err)
	}
}
