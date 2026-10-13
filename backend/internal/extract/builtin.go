package extract

import "encoding/json"

// Builtin は全ユーザ共通のテンプレート（parser_templates.user_id = NULL）の定義。
// サーバの起動時にデータベースへ反映する（repository.ParserTemplate.SyncBuiltin）。
//
// SQL のマイグレーションではなくコードに置くのは、単体テストと実際の判定・抽出で
// 同じ定義を使い、ルールとテストがずれないようにするため。
type Builtin struct {
	Name           string
	FromPattern    string
	SubjectPattern string
	EventType      string
	Rules          Rules
}

// RulesJSON は extraction_rules に保存する JSON。
func (b Builtin) RulesJSON() []byte {
	data, err := json.Marshal(b.Rules)
	if err != nil {
		// Rules は固定の型だけで組み立てるため、ここで失敗するのはコードの誤り
		panic(err)
	}
	return data
}

// Source は Compile に渡す形にする（単体テストで使う）。
func (b Builtin) Source() TemplateSource {
	subject := b.SubjectPattern
	return TemplateSource{
		Name:            b.Name,
		EventType:       b.EventType,
		FromPattern:     b.FromPattern,
		SubjectPattern:  &subject,
		ExtractionRules: b.RulesJSON(),
	}
}

// 三井住友カードの利用通知に共通する項目の書き方。「◇項目：値」の形で、
// 項目名と「：」の間に半角・全角の空白が入ることがある（デビットの通知）。
const (
	smbcOccurredAt = `◇利用日[\s　]*[：:][\s　]*(\d{4}/\d{2}/\d{2} \d{2}:\d{2}(?::\d{2})?)`
	smbcAmount     = `◇利用金額[\s　]*[：:][\s　]*([\d,]+(?:\.\d+)?)[\s　]*(円|[A-Z]{3})`
	smbcMerchant   = `◇利用先[\s　]*[：:][\s　]*(.+)`
	smbcSubject    = `ご利用のお知らせ`
	smbcIssuer     = "三井住友カード株式会社"
)

// smbcRules は三井住友カードの利用通知の抽出ルール。
func smbcRules(paymentMethod PaymentMethod, gmailQuery string, skips ...skipSpec) Rules {
	var r Rules
	r.OccurredAt.Regex = smbcOccurredAt
	// デビットは秒まで、クレジットは分まで書かれる
	r.OccurredAt.Layouts = []string{"2006/01/02 15:04:05", "2006/01/02 15:04"}
	r.Amount.Regex = smbcAmount
	r.Merchant.Regex = smbcMerchant
	for _, s := range skips {
		r.Skip = append(r.Skip, struct {
			Regex  string `json:"regex"`
			Reason string `json:"reason"`
		}{Regex: s.regex, Reason: s.reason})
	}
	r.PaymentMethod = paymentMethod
	r.GmailQuery = gmailQuery
	return r
}

type skipSpec struct {
	regex  string
	reason string
}

// BuiltinTemplates は最小版で対応する送信元。利用者が実際に受け取っている
// 三井住友カード（Olive）のクレジットとデビットの利用通知（docs/mail-import-mvp.md 判断 3）。
var BuiltinTemplates = []Builtin{
	{
		Name:           "三井住友カード（クレジット）利用通知",
		FromPattern:    `^statement@vpass\.ne\.jp$`,
		SubjectPattern: smbcSubject,
		EventType:      "usage_notice",
		Rules: smbcRules(
			PaymentMethod{Name: "三井住友カード（クレジット）", Kind: "credit_card", Issuer: smbcIssuer},
			`from:statement@vpass.ne.jp subject:"ご利用のお知らせ"`,
			// 返品は元の取引を打ち消す操作で、支出として足すと二重に数えてしまう。
			// 元の取引との対応づけ（名寄せ）は拡張で行うため、最小版では取り込まない
			skipSpec{regex: `◇利用取引[\s　]*[：:][\s　]*返品`, reason: "返品（取引の取り消し）は、まだ取り込めません"},
		),
	},
	{
		Name:           "三井住友カード（デビット）利用通知",
		FromPattern:    `^smbc-debit@smbc-card\.com$`,
		SubjectPattern: smbcSubject,
		EventType:      "usage_notice",
		Rules: smbcRules(
			// デビットは利用と同時に口座から引き落とされるため、種別は bank とする
			PaymentMethod{Name: "三井住友カード（デビット）", Kind: "bank", Issuer: smbcIssuer},
			`from:smbc-debit@smbc-card.com subject:"ご利用のお知らせ"`,
		),
	},
}
