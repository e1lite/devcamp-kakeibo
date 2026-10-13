package repository_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// TestParserTemplate_SyncSystem は、全ユーザ共通のテンプレートをコードの定義に合わせる処理を確認する。
// 起動のたびに呼ぶため、2 回目は作らずに更新すること。
func TestParserTemplate_SyncSystem(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewParserTemplate(tx)
	user := newUser(t, repository.NewUser(tx))

	subject := "ご利用のお知らせ"
	template := model.ParserTemplate{
		// 開発用のデータベースに本物のテンプレートがあっても衝突しない名前にする
		Name:            "テスト用テンプレート " + uniqueSub(t),
		FromPattern:     `^notice@card\.example\.jp$`,
		SubjectPattern:  &subject,
		EventType:       "usage_notice",
		BodyFormat:      "text",
		ExtractionRules: []byte(`{"amount":{"regex":"old"}}`),
		Priority:        100,
		IsActive:        true,
		CreatedBy:       "system",
	}
	if err := repo.SyncSystem(t.Context(), []model.ParserTemplate{template}); err != nil {
		t.Fatalf("1 回目の反映に失敗しました: %v", err)
	}

	template.ExtractionRules = []byte(`{"amount":{"regex":"new"}}`)
	if err := repo.SyncSystem(t.Context(), []model.ParserTemplate{template}); err != nil {
		t.Fatalf("2 回目の反映に失敗しました: %v", err)
	}

	templates, err := repo.ListActive(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("取得に失敗しました: %v", err)
	}
	var found []model.ParserTemplate
	for _, tmpl := range templates {
		if tmpl.Name == template.Name {
			found = append(found, tmpl)
		}
	}
	if len(found) != 1 {
		t.Fatalf("同じテンプレートが %d 件あります（2 回目で作り直しています）", len(found))
	}

	// jsonb に保存した抽出ルールを、そのまま読み戻せること
	var rules struct {
		Amount struct {
			Regex string `json:"regex"`
		} `json:"amount"`
	}
	if err := json.Unmarshal(found[0].ExtractionRules, &rules); err != nil {
		t.Fatalf("抽出ルールを読めません: %v", err)
	}
	if rules.Amount.Regex != "new" {
		t.Errorf("抽出ルールが更新されていません: %q", rules.Amount.Regex)
	}
}

// TestEmailMessage_SaveExtracted は、利用通知 1 通から決済イベントと取引を作る処理を確認する。
func TestEmailMessage_SaveExtracted(t *testing.T) {
	tx := beginTx(t)
	user := newUser(t, repository.NewUser(tx))
	account := newMailAccount(t, repository.NewMailAccount(tx), user.ID)
	repo := repository.NewEmailMessage(tx)

	subject := "ご利用のお知らせ"
	template := model.ParserTemplate{
		Name: "テスト用テンプレート " + uniqueSub(t), FromPattern: "x", SubjectPattern: &subject,
		EventType: "usage_notice", BodyFormat: "text", ExtractionRules: []byte(`{}`),
		Priority: 100, IsActive: true, CreatedBy: "system",
	}
	if err := tx.Create(&template).Error; err != nil {
		t.Fatalf("テンプレートの作成に失敗しました: %v", err)
	}

	save := func(providerID string, amount int64) (int64, *model.EmailMessage, error) {
		email := newEmail(user.ID, account.ID, providerID)
		if _, err := repo.Insert(t.Context(), email); err != nil {
			t.Fatalf("メールの保存に失敗しました: %v", err)
		}
		id, err := repo.SaveExtracted(t.Context(), repository.ExtractedPayment{
			Email:               email,
			TemplateID:          template.ID,
			EventType:           "usage_notice",
			OccurredAt:          time.Date(2026, 10, 11, 16, 58, 0, 0, jst),
			AmountMinor:         amount,
			Currency:            "JPY",
			AmountJPYMinor:      amount,
			RawMerchant:         "テストマート　渋谷店",
			MerchantName:        "テストマート 渋谷店",
			PaymentMethodName:   "テストカード（クレジット）",
			PaymentMethodKind:   "credit_card",
			PaymentMethodIssuer: "テストカード株式会社",
			Now:                 time.Now(),
		})
		return id, email, err
	}

	txnID, email, err := save("gmail-1", 1156)
	if err != nil {
		t.Fatalf("取り込みに失敗しました: %v", err)
	}

	txn, err := repository.NewTransaction(tx).FindByID(t.Context(), txnID)
	if err != nil {
		t.Fatalf("作った取引を取得できません: %v", err)
	}
	if txn.Source != model.TransactionSourceEmail || txn.Status != model.TransactionStatusPending {
		t.Errorf("取引: source=%q status=%q, want email / pending（利用通知は確定前の速報）", txn.Source, txn.Status)
	}
	if txn.CategoryID != nil {
		t.Errorf("カテゴリ: got %d, want 未分類（自動分類は拡張）", *txn.CategoryID)
	}
	if txn.AmountMinor != 1156 || txn.MerchantName == nil || *txn.MerchantName != "テストマート 渋谷店" {
		t.Errorf("取引の中身: amount=%d merchant=%v", txn.AmountMinor, txn.MerchantName)
	}
	if txn.PaymentMethodName == nil || *txn.PaymentMethodName != "テストカード（クレジット）" {
		t.Errorf("決済手段: got %v", txn.PaymentMethodName)
	}

	var event model.PaymentEvent
	if eerr := tx.Where("email_message_id = ?", email.ID).First(&event).Error; eerr != nil {
		t.Fatalf("決済イベントが作られていません: %v", eerr)
	}
	if event.TransactionID == nil || *event.TransactionID != txnID {
		t.Errorf("決済イベントが取引に紐づいていません: %v", event.TransactionID)
	}

	var saved model.EmailMessage
	if serr := tx.First(&saved, email.ID).Error; serr != nil {
		t.Fatalf("メールを取得できません: %v", serr)
	}
	if saved.ParseStatus != model.ParseStatusSuccess || saved.Classification != model.ClassificationPayment {
		t.Errorf("メールの状態: parse_status=%q classification=%q", saved.ParseStatus, saved.Classification)
	}

	// 同じメールをもう一度取り込もうとしても、取引を 2 つ作らない
	again, err := repo.SaveExtracted(t.Context(), repository.ExtractedPayment{
		Email: email, TemplateID: template.ID, EventType: "usage_notice", OccurredAt: time.Now(),
		AmountMinor: 1, Currency: "JPY", AmountJPYMinor: 1, PaymentMethodName: "テストカード（クレジット）",
		PaymentMethodKind: "credit_card", Now: time.Now(),
	})
	if !errors.Is(err, repository.ErrAlreadyProcessed) || again != 0 {
		t.Errorf("2 回目: id=%d err=%v, want ErrAlreadyProcessed", again, err)
	}

	// 2 通目は、同じ店舗・同じ決済手段を使い回す（同じ名前で行を増やさない）
	secondID, _, err := save("gmail-2", 580)
	if err != nil {
		t.Fatalf("2 通目の取り込みに失敗しました: %v", err)
	}
	second, err := repository.NewTransaction(tx).FindByID(t.Context(), secondID)
	if err != nil {
		t.Fatalf("2 通目の取引を取得できません: %v", err)
	}
	if *second.MerchantID != *txn.MerchantID || *second.PaymentMethodID != *txn.PaymentMethodID {
		t.Error("同じ名前の店舗・決済手段が 2 つ作られています")
	}
}
