package service

import "errors"

// リソース共通のエラー。ハンドラがこれを見てステータスとエラーコードを決める。
var (
	// ErrNotFound はリソースが存在しない。404 に対応する。
	//
	// 他ユーザのリソースを指定された場合もこれを返す（API 仕様書 6 章冒頭）。
	// 403 で区別すると「その ID のリソースが存在する」ことが伝わり、
	// 連番の ID を推測で叩けば他ユーザのデータの有無が分かってしまうため。
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
