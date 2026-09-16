package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// 取引固有のエラー。
var (
	// ErrCategoryNotFound は指定したカテゴリが存在しない。404 に対応する。
	ErrCategoryNotFound = errors.New("指定したカテゴリが存在しません")
	// ErrPaymentMethodNotFound は指定した決済手段が存在しない。404 に対応する。
	ErrPaymentMethodNotFound = errors.New("指定した決済手段が存在しません")
	// ErrMerchantNotFound は指定した店舗が存在しない。404 に対応する。
	ErrMerchantNotFound = errors.New("指定した店舗が存在しません")
	// ErrInvalidAmount は金額が 0 以下。400 に対応する。
	ErrInvalidAmount = errors.New("金額は 1 以上で指定してください")
	// ErrInvalidDateRange は from が to より後。400 に対応する。
	ErrInvalidDateRange = errors.New("from が to より後です")
	// ErrLimitTooLarge は limit が上限を超えた。400 に対応する。
	ErrLimitTooLarge = errors.New("limit が上限を超えています")
	// ErrUnsupportedCurrency は円換算できない通貨を指定された。400 に対応する。
	ErrUnsupportedCurrency = errors.New("この通貨には対応していません")
	// ErrAlreadyMerged はマージの統合元を単独で削除しようとした。409 に対応する。
	ErrAlreadyMerged = errors.New("マージの統合元は単独で削除できません")
)

// ページネーションの既定値と上限（API 仕様書 2 章）。
const (
	DefaultLimit = 20
	MaxLimit     = 100
	NoteMaxLen   = 500
)

// Transaction は取引の参照と更新を担う（API-010〜014）。
type Transaction struct {
	repo           *repository.Transaction
	merchants      *repository.Merchant
	categories     *repository.Category
	paymentMethods *repository.PaymentMethod
	now            func() time.Time
}

// NewTransaction は Transaction サービスを生成する。
func NewTransaction(
	repo *repository.Transaction,
	merchants *repository.Merchant,
	categories *repository.Category,
	paymentMethods *repository.PaymentMethod,
) *Transaction {
	return &Transaction{
		repo:           repo,
		merchants:      merchants,
		categories:     categories,
		paymentMethods: paymentMethods,
		now:            time.Now,
	}
}

// List は条件に合致する取引を返す（API-010）。
func (s *Transaction) List(ctx context.Context, userID int64, f repository.ListFilter) (*repository.ListResult, error) {
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return nil, ErrInvalidDateRange
	}
	if f.Limit > MaxLimit {
		return nil, ErrLimitTooLarge
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	return s.repo.List(ctx, userID, f)
}

// Find は取引を 1 件返す（API-012）。
func (s *Transaction) Find(ctx context.Context, userID, id int64) (*repository.TransactionRow, error) {
	return s.findOwned(ctx, userID, id)
}

// CreateTransactionInput は API-011 のリクエスト。
type CreateTransactionInput struct {
	OccurredAt      time.Time
	AmountMinor     int64
	Currency        string
	MerchantName    *string
	CategoryID      *int64
	PaymentMethodID *int64
	Note            *string
}

// Create は取引を手動で登録する（API-011）。
//
// 現金など通知メールが発生しない決済を登録する。決済イベントを経由せず
// transactions に直接作成し、source = manual / is_user_edited = true とする。
func (s *Transaction) Create(ctx context.Context, userID int64, in CreateTransactionInput) (*repository.TransactionRow, error) {
	if in.AmountMinor < 1 {
		return nil, ErrInvalidAmount
	}
	if in.OccurredAt.IsZero() {
		return nil, ErrValidation("occurred_at は必須です")
	}
	if err := validateNote(in.Note); err != nil {
		return nil, err
	}

	currency := in.Currency
	if currency == "" {
		currency = model.CurrencyJPY
	}
	// 為替レートの取得元がまだ無いため、円換算できるのは JPY のみ。
	// 多通貨は Step 8 以降で対応する
	if currency != model.CurrencyJPY {
		return nil, ErrUnsupportedCurrency
	}

	if err := s.validateReferences(ctx, userID, in.CategoryID, in.PaymentMethodID, nil); err != nil {
		return nil, err
	}

	txn := &model.Transaction{
		UserID:          userID,
		OccurredAt:      in.OccurredAt,
		AmountMinor:     in.AmountMinor,
		Currency:        currency,
		AmountJPYMinor:  in.AmountMinor,
		CategoryID:      in.CategoryID,
		PaymentMethodID: in.PaymentMethodID,
		CategorySource:  model.CategorySourceDefault,
		Source:          model.TransactionSourceManual,
		Status:          model.TransactionStatusConfirmed,
		// 手動登録はユーザ自身の入力なので、再計算の対象外にする
		IsUserEdited: true,
		Note:         in.Note,
	}

	if in.CategoryID != nil {
		txn.CategorySource = model.CategorySourceUser
	}

	if in.MerchantName != nil && strings.TrimSpace(*in.MerchantName) != "" {
		merchant, err := s.merchants.FindOrCreateByName(ctx, userID, strings.TrimSpace(*in.MerchantName))
		if err != nil {
			return nil, err
		}
		txn.MerchantID = &merchant.ID
	}

	if err := s.repo.Create(ctx, txn); err != nil {
		return nil, err
	}

	return s.repo.FindByID(ctx, txn.ID)
}

// UpdateTransactionInput は API-013 のリクエスト。指定された項目のみ更新する。
type UpdateTransactionInput struct {
	OccurredAt      httpx.Optional[*time.Time]
	AmountMinor     httpx.Optional[*int64]
	MerchantID      httpx.Optional[*int64]
	CategoryID      httpx.Optional[*int64]
	PaymentMethodID httpx.Optional[*int64]
	Note            httpx.Optional[*string]
}

// Update は取引を修正する（API-013）。
//
// 修正した取引は is_user_edited = true となり、名寄せの再計算対象から外れる。
//
// Step 3 ではカテゴリ規則の学習（category_rules への upsert）は行わない。
// 学習には category_rules テーブルが必要だが、コア 6 テーブルに含めないため
// （API 仕様書 5.1 の ※1）。
func (s *Transaction) Update(ctx context.Context, userID, id int64, in UpdateTransactionInput) (*repository.TransactionRow, error) {
	if _, err := s.findOwned(ctx, userID, id); err != nil {
		return nil, err
	}

	fields := map[string]any{}

	if in.OccurredAt.Set {
		if in.OccurredAt.Value == nil {
			return nil, ErrValidation("occurred_at に null は指定できません")
		}
		fields["occurred_at"] = *in.OccurredAt.Value
	}

	if in.AmountMinor.Set {
		if in.AmountMinor.Value == nil {
			return nil, ErrValidation("amount_minor に null は指定できません")
		}
		if *in.AmountMinor.Value < 1 {
			return nil, ErrInvalidAmount
		}
		fields["amount_minor"] = *in.AmountMinor.Value
		// 現状は JPY のみ扱うため、円換算額も同じ値で揃える
		fields["amount_jpy_minor"] = *in.AmountMinor.Value
	}

	if in.Note.Set {
		if err := validateNote(in.Note.Value); err != nil {
			return nil, err
		}
		fields["note"] = in.Note.Value
	}

	if err := s.applyReferences(ctx, userID, in, fields); err != nil {
		return nil, err
	}

	if len(fields) > 0 {
		// ユーザが手修正した取引は再計算の対象から外す
		fields["is_user_edited"] = true

		if err := s.repo.Update(ctx, id, fields); err != nil {
			return nil, err
		}
	}

	return s.repo.FindByID(ctx, id)
}

// Delete は取引を論理削除する（API-014）。
func (s *Transaction) Delete(ctx context.Context, userID, id int64) error {
	txn, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return err
	}

	// マージの統合元は unmerge で復元できる必要があるため、単独では削除させない
	if txn.MergedIntoID != nil {
		return ErrAlreadyMerged
	}

	return s.repo.SoftDelete(ctx, id, s.now())
}

// findOwned は取引を取得し、所有者であることを確認する。
func (s *Transaction) findOwned(ctx context.Context, userID, id int64) (*repository.TransactionRow, error) {
	row, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if row.UserID != userID {
		return nil, ErrForbidden
	}
	return row, nil
}

// applyReferences は外部キー項目の更新を検証して fields に積む。
func (s *Transaction) applyReferences(
	ctx context.Context, userID int64, in UpdateTransactionInput, fields map[string]any,
) error {
	if in.MerchantID.Set {
		if in.MerchantID.Value != nil {
			exists, err := s.merchants.ExistsForUser(ctx, *in.MerchantID.Value, userID)
			if err != nil {
				return err
			}
			if !exists {
				return ErrMerchantNotFound
			}
		}
		fields["merchant_id"] = in.MerchantID.Value
	}

	if in.CategoryID.Set {
		if in.CategoryID.Value != nil {
			exists, err := s.categories.ExistsForUser(ctx, *in.CategoryID.Value, userID)
			if err != nil {
				return err
			}
			if !exists {
				return ErrCategoryNotFound
			}
			// ユーザが指定したカテゴリはルールで上書きさせない
			fields["category_source"] = model.CategorySourceUser
		}
		fields["category_id"] = in.CategoryID.Value
	}

	if in.PaymentMethodID.Set {
		if in.PaymentMethodID.Value != nil {
			exists, err := s.paymentMethods.ExistsForUser(ctx, *in.PaymentMethodID.Value, userID)
			if err != nil {
				return err
			}
			if !exists {
				return ErrPaymentMethodNotFound
			}
		}
		fields["payment_method_id"] = in.PaymentMethodID.Value
	}

	return nil
}

// validateReferences は作成時に指定されたマスタが自分のものかを確認する。
func (s *Transaction) validateReferences(
	ctx context.Context, userID int64, categoryID, paymentMethodID, merchantID *int64,
) error {
	if categoryID != nil {
		exists, err := s.categories.ExistsForUser(ctx, *categoryID, userID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrCategoryNotFound
		}
	}
	if paymentMethodID != nil {
		exists, err := s.paymentMethods.ExistsForUser(ctx, *paymentMethodID, userID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrPaymentMethodNotFound
		}
	}
	if merchantID != nil {
		exists, err := s.merchants.ExistsForUser(ctx, *merchantID, userID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrMerchantNotFound
		}
	}
	return nil
}

func validateNote(note *string) error {
	if note == nil {
		return nil
	}
	if len([]rune(*note)) > NoteMaxLen {
		return ErrValidation("note は 500 文字以内で指定してください")
	}
	return nil
}
