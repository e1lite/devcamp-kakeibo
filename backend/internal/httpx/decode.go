package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// maxBodyBytes はリクエストボディの上限。
// 巨大なボディでメモリを食い潰されないように制限する。
const maxBodyBytes = 1 << 20 // 1 MiB

// Optional は「指定されなかった」と「null が指定された」を区別できる値。
//
// 部分更新（PUT）では 3 つの状態を区別する必要がある。
//
//	{}                   → Set=false            変更しない
//	{"parent_id": null}  → Set=true, Value=nil  親を外す
//	{"parent_id": 3}     → Set=true, Value=&3   親を 3 にする
//
// ポインタ 1 つでは「キーが無い」と「null」がどちらも nil になり区別できない。
type Optional[T any] struct {
	// Set はキーがリクエストに含まれていたかを表す。
	Set bool
	// Value はキーの値。null の場合はゼロ値（ポインタなら nil）。
	Value T
}

// UnmarshalJSON はキーが存在する場合にのみ呼ばれる。これを利用して Set を立てる。
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

// DecodeJSON はリクエストボディを dst に読み込む。
//
// 仕様にないフィールドが送られた場合はエラーにする。
// 黙って無視すると、フィールド名の打ち間違いが「なぜか反映されない」という
// 分かりにくい不具合になるため。
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) *APIError {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return decodeError(err)
	}

	// ボディに 2 つ目の JSON が続いていないことを確認する
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ValidationError("リクエストボディが不正です")
	}

	return nil
}

func decodeError(err error) *APIError {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxBytesErr *http.MaxBytesError

	switch {
	case errors.As(err, &syntaxErr):
		return ValidationError(fmt.Sprintf("JSON の構文が不正です（%d 文字目）", syntaxErr.Offset))

	case errors.As(err, &typeErr):
		return ValidationError(fmt.Sprintf("%s の型が不正です", typeErr.Field))

	case errors.As(err, &maxBytesErr):
		return ValidationError("リクエストボディが大きすぎます")

	case errors.Is(err, io.EOF):
		return ValidationError("リクエストボディが空です")

	default:
		// DisallowUnknownFields のエラーは型が公開されていないため、
		// メッセージで判定するしかない
		return ValidationError("リクエストボディが不正です: " + err.Error())
	}
}

// PathID はパスパラメータを ID として取り出す。
//
// 1 以上の整数でない場合は 404 を返す。存在しえない ID を指定されたのだから、
// 「不正な形式」よりも「見つからない」として扱う方がクライアントには分かりやすい。
func PathID(r *http.Request, name, notFoundCode, notFoundMessage string) (int64, *APIError) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id < 1 {
		return 0, NotFound(notFoundCode, notFoundMessage)
	}
	return id, nil
}
