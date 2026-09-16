package httpx

import (
	"fmt"
	"net/http"
)

// エラーコード。API 仕様書 6 章のエラーレスポンス表と対応する。
//
// 粒度の方針（API 仕様書 8 章の確認結果 4）:
// 形式不正のような「どの項目でも起きうる」ものは VALIDATION_ERROR にまとめ、
// 画面上で別のメッセージを出したいものは個別のコードにする。
const (
	// 認証・認可（全エンドポイント共通）
	CodeInvalidToken = "INVALID_TOKEN"
	CodeTokenExpired = "TOKEN_EXPIRED"
	CodeForbidden    = "FORBIDDEN"

	// 汎用
	CodeValidationError = "VALIDATION_ERROR"
	CodeInternalError   = "INTERNAL_ERROR"
	CodeNotFound        = "NOT_FOUND"

	// 一覧系
	CodeLimitTooLarge    = "LIMIT_TOO_LARGE"
	CodeInvalidDateRange = "INVALID_DATE_RANGE"

	// ヘルスチェック（API-001）
	CodeDatabaseUnavailable = "DATABASE_UNAVAILABLE"
)

// APIError はクライアントへ返すエラー。ステータスコードとエラーコードを対で持つ。
type APIError struct {
	Status  int
	Code    string
	Message string
	// Err は原因となった内部エラー。ログにのみ出し、レスポンスには含めない。
	Err error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap は errors.Is / errors.As で原因エラーを辿れるようにする。
func (e *APIError) Unwrap() error { return e.Err }

// NewAPIError は任意のステータス・コードの APIError を作る。
func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// Unauthorized は 401 を返す。トークンが不正な場合に使う。
func Unauthorized(code, message string) *APIError {
	return NewAPIError(http.StatusUnauthorized, code, message)
}

// Forbidden は 403 を返す。
//
// 他ユーザのリソースを指定された場合は 404 ではなく 403 を返す（API 仕様書 6 章冒頭）。
// 存在の有無を推測されないようにするより、自分のデータでないことを明示する方が
// UI 上扱いやすいため。
func Forbidden(message string) *APIError {
	return NewAPIError(http.StatusForbidden, CodeForbidden, message)
}

// NotFound は 404 を返す。code にはリソースごとのコードを渡す
// （例: TRANSACTION_NOT_FOUND）。
func NotFound(code, message string) *APIError {
	return NewAPIError(http.StatusNotFound, code, message)
}

// ValidationError は 400 VALIDATION_ERROR を返す。
func ValidationError(message string) *APIError {
	return NewAPIError(http.StatusBadRequest, CodeValidationError, message)
}

// BadRequest は 400 を任意のエラーコードで返す。
func BadRequest(code, message string) *APIError {
	return NewAPIError(http.StatusBadRequest, code, message)
}

// Conflict は 409 を返す。
func Conflict(code, message string) *APIError {
	return NewAPIError(http.StatusConflict, code, message)
}

// Internal は 500 を返す。原因エラーはログにのみ出し、クライアントには見せない。
func Internal(err error) *APIError {
	return &APIError{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternalError,
		Message: "サーバ内部エラーが発生しました",
		Err:     err,
	}
}
