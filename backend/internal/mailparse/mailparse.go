// Package mailparse はメールの原文（RFC 5322 / MIME）を、送信元・件名・本文に分解する。
//
// 送信元ごとの判定・抽出（どの行が金額か、など）は扱わない。ここではどの送信元の
// メールにも共通する、文字コードと転送符号化の解読だけを行う（パイプライン P3
// 「日本語メール特有の処理」）。
//
//   - 文字コード: ISO-2022-JP（日本のメールでいまも使われる）・Shift_JIS・EUC-JP・UTF-8
//   - 転送符号化: quoted-printable / base64
//   - 件名・差出人名の符号化（=?ISO-2022-JP?B?...?=）
//   - multipart（テキストと HTML の両方を持つメールが多い）
package mailparse

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/text/encoding/htmlindex"
)

// Message はメール 1 通を分解した結果。
type Message struct {
	FromAddress string
	FromName    string
	Subject     string
	// Date は Date ヘッダの日時。無い・読めない場合はゼロ値（受信日時は Gmail 側の値を使う）
	Date time.Time
	// Text は最初の text/plain の本文。改行は \n にそろえる
	Text string
	// HTML は最初の text/html の本文
	HTML string
}

// 入れ子の multipart を無限にたどらないための上限
const maxDepth = 10

var wordDecoder = &mime.WordDecoder{CharsetReader: charsetReader}

// Parse はメールの原文を分解する。
//
// 本文の一部が読めなくても、読めたところまでを返す（1 通の一部の失敗で、そのメール全体を捨てない）。
// ヘッダそのものが読めない場合だけエラーを返す。
func Parse(raw []byte) (*Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("メールのヘッダを読めません: %w", err)
	}

	result := &Message{Subject: decodeHeader(msg.Header.Get("Subject"))}

	parser := mail.AddressParser{WordDecoder: wordDecoder}
	if from, ferr := parser.Parse(msg.Header.Get("From")); ferr == nil {
		result.FromAddress = strings.ToLower(from.Address)
		result.FromName = from.Name
	} else {
		// 規格どおりでない From もある。読めなければ、そのままを送信元として残す
		result.FromAddress = strings.TrimSpace(decodeHeader(msg.Header.Get("From")))
	}

	if date, derr := msg.Header.Date(); derr == nil {
		result.Date = date
	}

	walk(result, msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body, 0)
	return result, nil
}

// walk は本文の部品をたどり、最初の text/plain と text/html を拾う。
func walk(result *Message, contentType, transferEncoding string, body io.Reader, depth int) {
	if depth > maxDepth {
		return
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || contentType == "" {
		// Content-Type が無いメールは text/plain（RFC 2045）
		mediaType, params = "text/plain", map[string]string{"charset": "us-ascii"}
	}

	switch {
	case strings.HasPrefix(mediaType, "multipart/"):
		reader := multipart.NewReader(body, params["boundary"])
		for {
			// NextPart は quoted-printable を自動で解読してヘッダを消すため、
			// 転送符号化をこちらで一律に扱えるよう NextRawPart を使う
			part, perr := reader.NextRawPart()
			if perr != nil {
				return
			}
			if isAttachment(part.Header.Get("Content-Disposition")) {
				continue
			}
			walk(result, part.Header.Get("Content-Type"), part.Header.Get("Content-Transfer-Encoding"), part, depth+1)
		}

	case mediaType == "text/plain" && result.Text == "":
		result.Text = normalizeNewlines(readText(body, transferEncoding, params["charset"]))

	case mediaType == "text/html" && result.HTML == "":
		result.HTML = readText(body, transferEncoding, params["charset"])
	}
}

func isAttachment(disposition string) bool {
	kind, _, err := mime.ParseMediaType(disposition)
	return err == nil && kind == "attachment"
}

// readText は転送符号化を解いてから、文字コードを UTF-8 にする。
// 解けなかった部分は捨て、読めたところまでを返す。
func readText(body io.Reader, transferEncoding, charset string) string {
	var decoded io.Reader
	switch strings.ToLower(strings.TrimSpace(transferEncoding)) {
	case "base64":
		// 76 文字ごとの改行は base64 のデコーダが読み飛ばす
		decoded = base64.NewDecoder(base64.StdEncoding, body)
	case "quoted-printable":
		decoded = quotedprintable.NewReader(body)
	default:
		// 7bit / 8bit / binary
		decoded = body
	}

	raw, _ := io.ReadAll(decoded)
	reader, err := charsetReader(charset, bytes.NewReader(raw))
	if err != nil {
		// 知らない文字コードは UTF-8 とみなす（大半の送信元は UTF-8 か ISO-2022-JP）
		return string(raw)
	}
	text, err := io.ReadAll(reader)
	if err != nil {
		return string(raw)
	}
	return string(text)
}

// charsetReader は文字コード名から UTF-8 への変換を返す。
// 名前の表記ゆれ（Shift_JIS / shift-jis / x-sjis など）は WHATWG の一覧で吸収する。
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	name := strings.ToLower(strings.TrimSpace(charset))
	if name == "" || name == "utf-8" || name == "us-ascii" {
		return input, nil
	}
	encoding, err := htmlindex.Get(name)
	if err != nil {
		return nil, fmt.Errorf("対応していない文字コードです: %s", charset)
	}
	return encoding.NewDecoder().Reader(input), nil
}

// decodeHeader は =?ISO-2022-JP?B?...?= のように符号化されたヘッダを読む。
// 読めなければ、そのままの文字列を返す。
func decodeHeader(value string) string {
	decoded, err := wordDecoder.DecodeHeader(value)
	if err != nil {
		return value
	}
	return decoded
}

func normalizeNewlines(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
}
