package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
)

// ParserTemplate は parser_templates テーブルへのアクセスを提供する。
type ParserTemplate struct {
	db *gorm.DB
}

// NewParserTemplate は ParserTemplate リポジトリを生成する。
func NewParserTemplate(db *gorm.DB) *ParserTemplate {
	return &ParserTemplate{db: db}
}

// SyncSystem は全ユーザ共通のテンプレート（user_id = NULL）を、コードの定義に合わせる。
// 同じ名前があれば判定条件と抽出ルールを更新し、無ければ作る。サーバの起動時に呼ぶ。
//
// UNIQUE(user_id, name) は user_id が NULL の行どうしを重複とみなさないため、
// ON CONFLICT は使えない。名前で探してから更新・作成する。
func (r *ParserTemplate) SyncSystem(ctx context.Context, templates []model.ParserTemplate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range templates {
			t := templates[i]
			var existing model.ParserTemplate
			err := tx.Where("user_id IS NULL AND name = ?", t.Name).First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				if cerr := tx.Create(&t).Error; cerr != nil {
					return fmt.Errorf("テンプレート %q の作成に失敗しました: %w", t.Name, cerr)
				}
			case err != nil:
				return fmt.Errorf("テンプレート %q の取得に失敗しました: %w", t.Name, err)
			default:
				uerr := tx.Model(&existing).Updates(map[string]any{
					"from_pattern":     t.FromPattern,
					"subject_pattern":  t.SubjectPattern,
					"event_type":       t.EventType,
					"extraction_rules": t.ExtractionRules,
				}).Error
				if uerr != nil {
					return fmt.Errorf("テンプレート %q の更新に失敗しました: %w", t.Name, uerr)
				}
			}
		}
		return nil
	})
}

// ListActive はユーザに使うテンプレートを評価順に返す。全ユーザ共通とユーザ自身のもの。
func (r *ParserTemplate) ListActive(ctx context.Context, userID int64) ([]model.ParserTemplate, error) {
	var templates []model.ParserTemplate
	err := r.db.WithContext(ctx).
		Where("is_active AND (user_id IS NULL OR user_id = ?)", userID).
		Order("priority, id").
		Find(&templates).Error
	if err != nil {
		return nil, fmt.Errorf("テンプレートの取得に失敗しました: %w", err)
	}
	return templates, nil
}

// ListPending は判定・抽出がまだのメールを古い順に返す。
func (r *EmailMessage) ListPending(ctx context.Context, mailAccountID int64, limit int) ([]model.EmailMessage, error) {
	var messages []model.EmailMessage
	err := r.db.WithContext(ctx).
		Where("mail_account_id = ? AND parse_status = ?", mailAccountID, model.ParseStatusPending).
		Order("received_at, id").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, fmt.Errorf("未処理のメールの取得に失敗しました: %w", err)
	}
	return messages, nil
}

// MarkResult は取引を作らなかったメール（対象外・取り込まない・失敗）の判定結果を記録する。
func (r *EmailMessage) MarkResult(ctx context.Context, id int64, fields map[string]any) error {
	err := r.db.WithContext(ctx).Model(&model.EmailMessage{}).Where("id = ?", id).Updates(fields).Error
	if err != nil {
		return fmt.Errorf("メールの判定結果の記録に失敗しました: %w", err)
	}
	return nil
}

// ExtractedPayment はメール 1 通から取り出した決済と、それを保存するのに要る値。
type ExtractedPayment struct {
	Email      *model.EmailMessage
	TemplateID int64
	EventType  string
	OccurredAt time.Time
	// AmountMinor は原通貨の金額、AmountJPYMinor は円換算額（最小版は円のみのため同じ値）
	AmountMinor    int64
	Currency       string
	AmountJPYMinor int64
	RawMerchant    string
	// MerchantName は正規化した店舗名。空なら店舗なし
	MerchantName string
	// PaymentMethod はテンプレートで決まる決済手段。名前でユーザごとに探し、無ければ作る
	PaymentMethodName   string
	PaymentMethodKind   string
	PaymentMethodIssuer string
	Now                 time.Time
}

// ErrAlreadyProcessed は、ほかの処理がすでにこのメールを取り込んでいたことを表す。
var ErrAlreadyProcessed = errors.New("このメールはすでに処理されています")

// SaveExtracted は決済イベントと取引を作り、メールを処理済みにする。1 つのトランザクションで行う。
//
// 最小版は名寄せ（P5）をしないため、決済イベント 1 件 = 取引 1 件とする（docs/mail-import-mvp.md 判断 3）。
// 途中で失敗したら全部を戻し、メールは未処理のまま残す（次の同期でやり直せる）。
func (r *EmailMessage) SaveExtracted(ctx context.Context, p ExtractedPayment) (int64, error) {
	var transactionID int64

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 先にメールを未処理から処理済みに変え、同じメールから 2 回取引を作らないようにする
		claim := tx.Model(&model.EmailMessage{}).
			Where("id = ? AND parse_status = ?", p.Email.ID, model.ParseStatusPending).
			Updates(map[string]any{
				"classification":      model.ClassificationPayment,
				"classified_at":       p.Now,
				"matched_template_id": p.TemplateID,
				"parse_status":        model.ParseStatusSuccess,
				"parse_error":         nil,
			})
		if claim.Error != nil {
			return fmt.Errorf("メールの状態の更新に失敗しました: %w", claim.Error)
		}
		if claim.RowsAffected == 0 {
			return ErrAlreadyProcessed
		}

		merchantID, err := findOrCreateEmailMerchant(tx, p.Email.UserID, p.MerchantName)
		if err != nil {
			return err
		}
		paymentMethodID, err := findOrCreatePaymentMethod(tx, p)
		if err != nil {
			return err
		}

		txn := &model.Transaction{
			UserID:          p.Email.UserID,
			OccurredAt:      p.OccurredAt,
			AmountMinor:     p.AmountMinor,
			Currency:        p.Currency,
			AmountJPYMinor:  p.AmountJPYMinor,
			MerchantID:      merchantID,
			PaymentMethodID: &paymentMethodID,
			// カテゴリの自動分類（P6）は拡張。未分類（NULL）で取り込み、利用者が編集で付ける
			CategorySource: model.CategorySourceDefault,
			Source:         model.TransactionSourceEmail,
			// 利用通知は確定前の速報（DB 仕様書 3.7）
			Status: model.TransactionStatusPending,
		}
		if cerr := tx.Create(txn).Error; cerr != nil {
			return fmt.Errorf("取引の作成に失敗しました: %w", cerr)
		}
		transactionID = txn.ID

		linkType := "auto"
		rawMerchant := p.RawMerchant
		event := &model.PaymentEvent{
			UserID:           p.Email.UserID,
			EmailMessageID:   p.Email.ID,
			ParserTemplateID: &p.TemplateID,
			TransactionID:    &txn.ID,
			LinkType:         &linkType,
			LinkedAt:         &p.Now,
			EventType:        p.EventType,
			OccurredAt:       p.OccurredAt,
			AmountMinor:      p.AmountMinor,
			Currency:         p.Currency,
			AmountJPYMinor:   p.AmountJPYMinor,
			RawMerchantText:  &rawMerchant,
			MerchantID:       merchantID,
			PaymentMethodID:  &paymentMethodID,
			NormalizedAt:     &p.Now,
		}
		if cerr := tx.Create(event).Error; cerr != nil {
			return fmt.Errorf("決済イベントの作成に失敗しました: %w", cerr)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return transactionID, nil
}

// findOrCreateEmailMerchant は正規化した名前で店舗を探し、無ければ作る。
//
// 手入力の店舗（Merchant.FindOrCreateByName）と違い、メールの店舗は位置を調べる対象
// （ジオコーディング P4.6）になりうるため、geocode_status は pending で作る。
func findOrCreateEmailMerchant(tx *gorm.DB, userID int64, name string) (*int64, error) {
	if name == "" {
		return nil, nil
	}
	var merchant model.Merchant
	err := tx.Where("user_id = ? AND name = ?", userID, name).First(&merchant).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		merchant = model.Merchant{UserID: userID, Name: name, GeocodeStatus: "pending"}
		if cerr := tx.Create(&merchant).Error; cerr != nil {
			return nil, fmt.Errorf("店舗の作成に失敗しました: %w", cerr)
		}
	} else if err != nil {
		return nil, fmt.Errorf("店舗の取得に失敗しました: %w", err)
	}
	return &merchant.ID, nil
}

// findOrCreatePaymentMethod はテンプレートの決済手段を、名前でユーザごとに探し、無ければ作る。
// 決済手段はメールの解析で自動で作られる前提（API 仕様書 API-031）。
func findOrCreatePaymentMethod(tx *gorm.DB, p ExtractedPayment) (int64, error) {
	var method model.PaymentMethod
	err := tx.Where("user_id = ? AND name = ?", p.Email.UserID, p.PaymentMethodName).First(&method).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		issuer := p.PaymentMethodIssuer
		method = model.PaymentMethod{
			UserID:   p.Email.UserID,
			Name:     p.PaymentMethodName,
			Kind:     p.PaymentMethodKind,
			Issuer:   &issuer,
			IsActive: true,
		}
		if cerr := tx.Create(&method).Error; cerr != nil {
			return 0, fmt.Errorf("決済手段の作成に失敗しました: %w", cerr)
		}
	} else if err != nil {
		return 0, fmt.Errorf("決済手段の取得に失敗しました: %w", err)
	}
	return method.ID, nil
}
