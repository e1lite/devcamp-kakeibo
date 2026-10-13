// Package extract は、送信元ごとのテンプレート（parser_templates）でメールを判定し、
// 本文から決済の項目を取り出す（パイプライン P2 判定・P3 抽出）。
//
// 判定と抽出はどちらも純粋な関数にし、ルールを変えたら保存済みのメールを全件やり直せるようにする
// （email_messages に生データを残す設計の前提。パイプライン P1）。
package extract

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// jst は利用日時を読むタイムゾーン。カード会社の通知は日本時間で書かれている。
var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

// Rules は parser_templates.extraction_rules の中身。
//
// DB 仕様書 3.5 の例（occurred_at・amount・merchant）に、最小版で次の 3 つを足した。
//   - skip: 取り込まないメールの条件（返品など）
//   - payment_method: このテンプレートで作る決済手段。全ユーザ共通のテンプレート（user_id = NULL）は
//     ユーザごとの payment_methods を参照できないため、名前と種別で持ち、ユーザごとに作る
//   - gmail_query: このテンプレートの送信元を Gmail で探す条件（docs/mail-import-mvp.md 判断 2）
type Rules struct {
	OccurredAt struct {
		Regex   string   `json:"regex"`
		Layouts []string `json:"layouts"`
	} `json:"occurred_at"`
	// Amount の 1 つ目のグループは数値、2 つ目は単位（「円」か通貨コード）
	Amount struct {
		Regex string `json:"regex"`
	} `json:"amount"`
	Merchant struct {
		Regex string `json:"regex"`
	} `json:"merchant"`
	Skip []struct {
		Regex  string `json:"regex"`
		Reason string `json:"reason"`
	} `json:"skip"`
	PaymentMethod PaymentMethod `json:"payment_method"`
	GmailQuery    string        `json:"gmail_query"`
}

// PaymentMethod はテンプレートで作る決済手段。
type PaymentMethod struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Issuer string `json:"issuer"`
}

// Template は判定と抽出の準備ができたテンプレート。
type Template struct {
	ID            int64
	Name          string
	EventType     string
	PaymentMethod PaymentMethod
	GmailQuery    string

	from       *regexp.Regexp
	subject    *regexp.Regexp
	occurredAt *regexp.Regexp
	layouts    []string
	amount     *regexp.Regexp
	merchant   *regexp.Regexp
	skips      []skipRule
}

type skipRule struct {
	pattern *regexp.Regexp
	reason  string
}

// TemplateSource は parser_templates の 1 行のうち、テンプレートの組み立てに使う列。
type TemplateSource struct {
	ID              int64
	Name            string
	EventType       string
	FromPattern     string
	SubjectPattern  *string
	ExtractionRules []byte
}

// Compile は parser_templates の 1 行から Template を作る。正規表現が壊れていればエラー。
func Compile(src TemplateSource) (*Template, error) {
	var rules Rules
	if err := json.Unmarshal(src.ExtractionRules, &rules); err != nil {
		return nil, fmt.Errorf("テンプレート %q の抽出ルールを読めません: %w", src.Name, err)
	}

	t := &Template{
		ID:            src.ID,
		Name:          src.Name,
		EventType:     src.EventType,
		PaymentMethod: rules.PaymentMethod,
		GmailQuery:    rules.GmailQuery,
		layouts:       rules.OccurredAt.Layouts,
	}

	compile := func(field, pattern string) (*regexp.Regexp, error) {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("テンプレート %q の %s の正規表現が不正です: %w", src.Name, field, err)
		}
		return re, nil
	}

	var err error
	if t.from, err = compile("from_pattern", src.FromPattern); err != nil {
		return nil, err
	}
	if src.SubjectPattern != nil && *src.SubjectPattern != "" {
		if t.subject, err = compile("subject_pattern", *src.SubjectPattern); err != nil {
			return nil, err
		}
	}
	if t.occurredAt, err = compile("occurred_at", rules.OccurredAt.Regex); err != nil {
		return nil, err
	}
	if t.amount, err = compile("amount", rules.Amount.Regex); err != nil {
		return nil, err
	}
	if t.merchant, err = compile("merchant", rules.Merchant.Regex); err != nil {
		return nil, err
	}
	for _, s := range rules.Skip {
		re, cerr := compile("skip", s.Regex)
		if cerr != nil {
			return nil, cerr
		}
		t.skips = append(t.skips, skipRule{pattern: re, reason: s.Reason})
	}
	if len(t.layouts) == 0 {
		return nil, fmt.Errorf("テンプレート %q に日時の書式（layouts）がありません", src.Name)
	}
	return t, nil
}

// Matches は送信元と件名がこのテンプレートの対象かを返す（パイプライン P2 判定）。
//
// 件名の条件は、同じ送信元の販促メールを除くためのもの。条件が無ければ送信元だけで判定する。
func (t *Template) Matches(fromAddress, subject string) bool {
	if !t.from.MatchString(strings.ToLower(fromAddress)) {
		return false
	}
	return t.subject == nil || t.subject.MatchString(subject)
}

// Event はメール 1 通から取り出した決済。
type Event struct {
	OccurredAt     time.Time
	AmountMinor    int64
	Currency       string
	AmountJPYMinor int64
	// RawMerchant はメールに書かれたままの店舗名。正規化した名前は MerchantName
	RawMerchant  string
	MerchantName string
}

// SkipError は取り込まないと決めたメール（返品・外貨など）。失敗ではない。
type SkipError struct {
	Reason string
}

func (e *SkipError) Error() string { return e.Reason }

// IsSkip は err が SkipError かを返す。
func IsSkip(err error) (*SkipError, bool) {
	var skip *SkipError
	ok := errors.As(err, &skip)
	return skip, ok
}

// Extract は本文から決済の項目を取り出す（パイプライン P3 抽出、P4 正規化の一部）。
//
// 取り込まないメールは *SkipError、項目が見つからない・読めない場合はそれ以外のエラーを返す。
func (t *Template) Extract(body string) (*Event, error) {
	for _, s := range t.skips {
		if s.pattern.MatchString(body) {
			return nil, &SkipError{Reason: s.reason}
		}
	}

	dateText, err := capture(t.occurredAt, body, "利用日時")
	if err != nil {
		return nil, err
	}
	occurredAt, err := parseTime(dateText, t.layouts)
	if err != nil {
		return nil, err
	}

	m := t.amount.FindStringSubmatch(body)
	if len(m) < 3 {
		return nil, errors.New("利用金額が見つかりません")
	}
	amount, currency, err := parseAmount(m[1], m[2])
	if err != nil {
		return nil, err
	}

	rawMerchant, err := capture(t.merchant, body, "利用先")
	if err != nil {
		return nil, err
	}

	return &Event{
		OccurredAt:     occurredAt,
		AmountMinor:    amount,
		Currency:       currency,
		AmountJPYMinor: amount,
		RawMerchant:    rawMerchant,
		MerchantName:   NormalizeMerchant(rawMerchant),
	}, nil
}

func capture(re *regexp.Regexp, body, label string) (string, error) {
	m := re.FindStringSubmatch(body)
	if len(m) < 2 {
		return "", fmt.Errorf("%sが見つかりません", label)
	}
	value := strings.TrimFunc(m[1], unicode.IsSpace)
	if value == "" {
		return "", fmt.Errorf("%sが空です", label)
	}
	return value, nil
}

func parseTime(value string, layouts []string) (time.Time, error) {
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, jst); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("利用日時 %q を読めません", value)
}

// parseAmount は金額を円の整数にする。
//
// 「1,156円」のほか、海外の加盟店では「1,234.00JPY」のように通貨コードで書かれる。
// 円以外の通貨は、円に換算する為替レートの取得元がまだ無いため取り込まない
// （手入力の取引と同じ制限。API-011 の UNSUPPORTED_CURRENCY）。
func parseAmount(number, unit string) (int64, string, error) {
	currency := strings.ToUpper(strings.TrimSpace(unit))
	if currency == "円" {
		currency = "JPY"
	}
	if currency != "JPY" {
		return 0, "", &SkipError{Reason: fmt.Sprintf("外貨（%s）の取引は、まだ取り込めません", currency)}
	}

	value, ok := new(big.Rat).SetString(strings.ReplaceAll(number, ",", ""))
	if !ok {
		return 0, "", fmt.Errorf("利用金額 %q を読めません", number)
	}
	// 円には小数の単位が無い。「1,234.00JPY」の .00 以外の端数は想定外として扱う
	if !value.IsInt() {
		return 0, "", fmt.Errorf("利用金額 %q に円未満の端数があります", number)
	}
	amount := value.Num()
	if !amount.IsInt64() || amount.Int64() < 1 {
		return 0, "", fmt.Errorf("利用金額 %q が範囲外です", number)
	}
	return amount.Int64(), "JPY", nil
}

// NormalizeMerchant は店舗名の表記をそろえる（パイプライン P4 正規化の最小限）。
//
// NFKC で全角英数字を半角に、半角カナを全角にそろえ、連続する空白を 1 つにする。
// 「ＩＣ　Ｐａｙ」と「IC Pay」が別の店舗にならないようにするため。
// 法人格の除去や表記ゆれの対応表（merchant_aliases）は拡張で行う。
func NormalizeMerchant(raw string) string {
	normalized := norm.NFKC.String(raw)
	return strings.Join(strings.Fields(normalized), " ")
}
