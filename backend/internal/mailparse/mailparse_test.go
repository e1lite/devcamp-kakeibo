package mailparse_test

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/mailparse"
)

// テストのメールはここで組み立てる。実際のカード会社のメールは個人情報を含むため使わない。

func encode(t *testing.T, enc encoding.Encoding, s string) []byte {
	t.Helper()
	b, err := enc.NewEncoder().Bytes([]byte(s))
	if err != nil {
		t.Fatalf("テストデータの文字コード変換に失敗しました: %v", err)
	}
	return b
}

// encodedWord は件名などに使う =?charset?B?...?= を作る。
func encodedWord(t *testing.T, charset string, enc encoding.Encoding, s string) string {
	t.Helper()
	return "=?" + charset + "?B?" + base64.StdEncoding.EncodeToString(encode(t, enc, s)) + "?="
}

func TestParse_ISO2022JPの件名と本文(t *testing.T) {
	t.Parallel()

	body := encode(t, japanese.ISO2022JP, "ご利用日時：2026/10/06 12:31\r\nご利用金額：580円\r\n")
	raw := "From: " + encodedWord(t, "ISO-2022-JP", japanese.ISO2022JP, "テストカード") + " <Info@Card.Example.JP>\r\n" +
		"Subject: " + encodedWord(t, "ISO-2022-JP", japanese.ISO2022JP, "カードご利用のお知らせ") + "\r\n" +
		"Date: Tue, 6 Oct 2026 12:35:00 +0900\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=ISO-2022-JP\r\n" +
		"Content-Transfer-Encoding: 7bit\r\n" +
		"\r\n" + string(body)

	msg, err := mailparse.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("解析に失敗しました: %v", err)
	}

	if msg.Subject != "カードご利用のお知らせ" {
		t.Errorf("件名: got %q", msg.Subject)
	}
	if msg.FromName != "テストカード" {
		t.Errorf("差出人名: got %q", msg.FromName)
	}
	// 送信元は小文字にそろえる（テンプレートの送信元と照合しやすくするため）
	if msg.FromAddress != "info@card.example.jp" {
		t.Errorf("送信元: got %q", msg.FromAddress)
	}
	if want := time.Date(2026, 10, 6, 3, 35, 0, 0, time.UTC); !msg.Date.Equal(want) {
		t.Errorf("日時: got %v, want %v", msg.Date, want)
	}
	// 改行は \n にそろえる
	if msg.Text != "ご利用日時：2026/10/06 12:31\nご利用金額：580円\n" {
		t.Errorf("本文: got %q", msg.Text)
	}
}

func TestParse_multipartのテキストとHTML(t *testing.T) {
	t.Parallel()

	html := base64.StdEncoding.EncodeToString(encode(t, japanese.ShiftJIS, "<p>ご利用金額：<b>1,200円</b></p>"))
	raw := strings.Join([]string{
		"From: notice@shop.example.com",
		"Subject: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte("ご注文の確認")) + "?=",
		"MIME-Version: 1.0",
		`Content-Type: multipart/mixed; boundary="outer"`,
		"",
		"--outer",
		`Content-Type: multipart/alternative; boundary="inner"`,
		"",
		"--inner",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable",
		"",
		// 「金額」を quoted-printable で書き、行末の = で行を継ぐ
		"=E9=87=91=E9=A1=8D=EF=BC=9A1,200=",
		"=E5=86=86",
		"--inner",
		"Content-Type: text/html; charset=Shift_JIS",
		"Content-Transfer-Encoding: base64",
		"",
		html[:20],
		html[20:],
		"--inner--",
		"--outer",
		"Content-Type: text/plain; charset=UTF-8",
		`Content-Disposition: attachment; filename="receipt.txt"`,
		"",
		"添付ファイルの本文（拾わない）",
		"--outer--",
		"",
	}, "\r\n")

	msg, err := mailparse.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("解析に失敗しました: %v", err)
	}

	if msg.Subject != "ご注文の確認" {
		t.Errorf("件名: got %q", msg.Subject)
	}
	if msg.Text != "金額：1,200円" {
		t.Errorf("テキスト: got %q", msg.Text)
	}
	if msg.HTML != "<p>ご利用金額：<b>1,200円</b></p>" {
		t.Errorf("HTML: got %q", msg.HTML)
	}
}

func TestParse_ヘッダが読めなければエラー(t *testing.T) {
	t.Parallel()

	if _, err := mailparse.Parse([]byte("これはメールではありません")); err == nil {
		t.Error("ヘッダの無いデータでエラーになりませんでした")
	}
}

func TestParse_知らない文字コードはそのまま読む(t *testing.T) {
	t.Parallel()

	raw := "From: a@example.com\r\nContent-Type: text/plain; charset=x-unknown\r\n\r\nplain text"
	msg, err := mailparse.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("解析に失敗しました: %v", err)
	}
	if msg.Text != "plain text" {
		t.Errorf("本文: got %q", msg.Text)
	}
}
