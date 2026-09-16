package service

import "errors"

// リソース共通のエラー。ハンドラがこれを見てステータスとエラーコードを決める。
var (
	// ErrForbidden は他ユーザのリソースを指定された。403 に対応する。
	//
	// 404 ではなく 403 を返すのは、存在の有無を推測されないようにするより、
	// 自分のデータでないことを明示する方が UI 上扱いやすいため（API 仕様書 6 章冒頭）。
	ErrForbidden = errors.New("他ユーザのリソースです")
	// ErrNotFound はリソースが存在しない。404 に対応する。
	ErrNotFound = errors.New("リソースが存在しません")
)

// ValidationError は入力値の不備。400 VALIDATION_ERROR に対応する。
//
// どの項目がなぜ不正なのかをメッセージで伝えるため、
// センチネル値ではなく型にしている。
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// ErrValidation は ValidationError を生成する。
func ErrValidation(message string) error {
	return &ValidationError{Message: message}
}
