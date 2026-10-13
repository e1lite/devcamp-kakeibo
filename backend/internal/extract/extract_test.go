package extract_test

import (
	"strings"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/extract"
)

// テストの本文は、実際の利用通知の「◇項目：値」の並びだけを真似て作った架空のもの。
// 利用者のメール（個人情報を含む）はリポジトリに置かない。

var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

func builtin(t *testing.T, name string) *extract.Template {
	t.Helper()
	for _, b := range extract.BuiltinTemplates {
		if strings.Contains(b.Name, name) {
			tmpl, err := extract.Compile(b.Source())
			if err != nil {
				t.Fatalf("テンプレートを組み立てられません: %v", err)
			}
			return tmpl
		}
	}
	t.Fatalf("テンプレート %q がありません", name)
	return nil
}

func creditBody(amount, kind string) string {
	return strings.Join([]string{
		"テスト　様",
		"",
		"ご利用カード：Ｏｌｉｖｅ／クレジット",
		"",
		"◇利用日：2026/10/11 16:58",
		"◇利用先：テストマート　渋谷店",
		"◇利用取引：" + kind,
		"◇利用金額：" + amount,
		"",
		"※（以下、案内文）",
	}, "\n")
}

func debitBody(amount string) string {
	return strings.Join([]string{
		"テスト 様",
		"",
		"◇利用日  ：2026/10/03 01:02:59",
		"◇利用先　：ＩＣ　Ｐａｙ",
		"◇利用金額：" + amount,
		"◇承認番号：000000",
		"  ※利用日は日本標準時刻となります。",
	}, "\n")
}

func TestTemplate_Matches_送信元と件名で判定する(t *testing.T) {
	t.Parallel()

	credit := builtin(t, "クレジット")
	debit := builtin(t, "デビット")

	tests := []struct {
		name    string
		tmpl    *extract.Template
		from    string
		subject string
		want    bool
	}{
		{"クレジットの利用通知", credit, "statement@vpass.ne.jp", "ご利用のお知らせ【三井住友カード】", true},
		{"送信元の大文字小文字は区別しない", credit, "Statement@Vpass.ne.jp", "ご利用のお知らせ【三井住友カード】", true},
		{"同じ送信元でも件名が違えば対象外（販促メールなど）", credit, "statement@vpass.ne.jp", "キャンペーンのご案内", false},
		{"デビットの利用通知", debit, "smbc-debit@smbc-card.com", "ご利用のお知らせ【三井住友カード】", true},
		{"別の送信元の「ご利用のお知らせ」は対象外", credit, "no-reply@point.example.jp", "ポイントご利用のお知らせ", false},
		{"似た送信元（前後に文字が付く）は対象外", credit, "statement@vpass.ne.jp.example.com", "ご利用のお知らせ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.tmpl.Matches(tt.from, tt.subject); got != tt.want {
				t.Errorf("Matches(%q, %q) = %v, want %v", tt.from, tt.subject, got, tt.want)
			}
		})
	}
}

func TestTemplate_Extract_クレジットの利用通知(t *testing.T) {
	t.Parallel()

	event, err := builtin(t, "クレジット").Extract(creditBody("1,156円", "買物"))
	if err != nil {
		t.Fatalf("抽出に失敗しました: %v", err)
	}

	if want := time.Date(2026, 10, 11, 16, 58, 0, 0, jst); !event.OccurredAt.Equal(want) {
		t.Errorf("利用日時: got %v, want %v", event.OccurredAt, want)
	}
	if event.AmountMinor != 1156 || event.AmountJPYMinor != 1156 || event.Currency != "JPY" {
		t.Errorf("金額: got %d %s（円換算 %d）", event.AmountMinor, event.Currency, event.AmountJPYMinor)
	}
	if event.RawMerchant != "テストマート　渋谷店" {
		t.Errorf("店舗（生）: got %q", event.RawMerchant)
	}
	// 全角の空白は半角にそろえる
	if event.MerchantName != "テストマート 渋谷店" {
		t.Errorf("店舗（正規化）: got %q", event.MerchantName)
	}
}

func TestTemplate_Extract_デビットは秒と項目名の後の空白も読む(t *testing.T) {
	t.Parallel()

	event, err := builtin(t, "デビット").Extract(debitBody("16,422円"))
	if err != nil {
		t.Fatalf("抽出に失敗しました: %v", err)
	}

	if want := time.Date(2026, 10, 3, 1, 2, 59, 0, jst); !event.OccurredAt.Equal(want) {
		t.Errorf("利用日時: got %v, want %v", event.OccurredAt, want)
	}
	if event.AmountMinor != 16422 {
		t.Errorf("金額: got %d", event.AmountMinor)
	}
	// 全角英数字は半角に（「ＩＣ　Ｐａｙ」と「IC Pay」を同じ店舗にする）
	if event.MerchantName != "IC Pay" {
		t.Errorf("店舗（正規化）: got %q", event.MerchantName)
	}
}

func TestTemplate_Extract_金額の書き方(t *testing.T) {
	t.Parallel()

	credit := builtin(t, "クレジット")
	tests := []struct {
		amount string
		want   int64
	}{
		{"580円", 580},
		{"12,345円", 12345},
		// 海外の加盟店は通貨コードで書く。円なら .00 の付いた書き方も受け付ける
		{"1,234.00JPY", 1234},
		{"98JPY", 98},
	}
	for _, tt := range tests {
		t.Run(tt.amount, func(t *testing.T) {
			t.Parallel()
			event, err := credit.Extract(creditBody(tt.amount, "買物"))
			if err != nil {
				t.Fatalf("抽出に失敗しました: %v", err)
			}
			if event.AmountMinor != tt.want {
				t.Errorf("金額: got %d, want %d", event.AmountMinor, tt.want)
			}
		})
	}
}

func TestTemplate_Extract_取り込まないメール(t *testing.T) {
	t.Parallel()

	credit := builtin(t, "クレジット")
	tests := []struct {
		name string
		body string
		want string
	}{
		{"返品は二重に数えないよう取り込まない", creditBody("1,156円", "返品"), "返品"},
		{"外貨は円に換算できないため取り込まない", creditBody("12.34USD", "買物"), "外貨（USD）"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := credit.Extract(tt.body)
			skip, ok := extract.IsSkip(err)
			if !ok {
				t.Fatalf("取り込まない扱いになりませんでした: %v", err)
			}
			if !strings.Contains(skip.Reason, tt.want) {
				t.Errorf("理由: got %q, want %q を含む", skip.Reason, tt.want)
			}
		})
	}
}

func TestTemplate_Extract_項目が欠けていれば失敗(t *testing.T) {
	t.Parallel()

	credit := builtin(t, "クレジット")
	tests := map[string]string{
		"金額が無い":       strings.Replace(creditBody("1,156円", "買物"), "◇利用金額：1,156円", "", 1),
		"利用先が無い":      strings.Replace(creditBody("1,156円", "買物"), "◇利用先：テストマート　渋谷店", "", 1),
		"円未満の端数がある":   creditBody("1,234.50JPY", "買物"),
		"テンプレートと違う形式": "ご利用ありがとうございました。",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := credit.Extract(body)
			if err == nil {
				t.Fatal("エラーになりませんでした")
			}
			if _, skip := extract.IsSkip(err); skip {
				t.Errorf("失敗ではなく「取り込まない」扱いになっています: %v", err)
			}
		})
	}
}

func TestCompile_壊れたルールはエラー(t *testing.T) {
	t.Parallel()

	src := extract.BuiltinTemplates[0].Source()
	src.FromPattern = `([`
	if _, err := extract.Compile(src); err == nil {
		t.Error("不正な正規表現でエラーになりませんでした")
	}
}
