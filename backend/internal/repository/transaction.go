package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
)

// Transaction は transactions テーブルへのアクセスを提供する。
//
// 参照系のクエリは必ず deleted_at IS NULL で絞る。
// 付け忘れると削除済みの取引が画面に出る（DB 仕様書 3.7）。
type Transaction struct {
	db *gorm.DB
}

// NewTransaction は Transaction リポジトリを生成する。
func NewTransaction(db *gorm.DB) *Transaction {
	return &Transaction{db: db}
}

// TransactionRow は取引と、結合したマスタの表示名。
type TransactionRow struct {
	model.Transaction
	MerchantName      *string `gorm:"column:merchant_name"`
	MerchantAddress   *string `gorm:"column:merchant_address"`
	CategoryName      *string `gorm:"column:category_name"`
	PaymentMethodName *string `gorm:"column:payment_method_name"`
	PaymentMethodKind *string `gorm:"column:payment_method_kind"`
}

// ListFilter は API-010 の絞り込み条件。
type ListFilter struct {
	From            *time.Time
	To              *time.Time
	CategoryID      *int64
	PaymentMethodID *int64
	MerchantID      *int64
	NeedsReview     *bool
	Query           *string
	Limit           int
	Offset          int
}

// ListResult は一覧と、条件に合致する全体の集計。
type ListResult struct {
	Rows             []TransactionRow
	Total            int64
	TotalAmountMinor int64
}

// 結合したマスタの表示名を含む SELECT 句。
//
// GORM は読み込み先の構造体から列名を推測するため、結合を伴う場合は
// 必ず明示する。省略すると transactions.merchant_name のような
// 存在しない列を SELECT に含めてしまう。
const transactionSelect = `transactions.*,
	m.name AS merchant_name,
	m.address AS merchant_address,
	c.name AS category_name,
	pm.name AS payment_method_name,
	pm.kind AS payment_method_kind`

const transactionJoins = `
	LEFT JOIN merchants m ON m.id = transactions.merchant_id
	LEFT JOIN categories c ON c.id = transactions.category_id
	LEFT JOIN payment_methods pm ON pm.id = transactions.payment_method_id`

// List は条件に合致する取引を新着順に取得する（API-010）。
//
// 一覧と同時に総件数と合計金額も返す。絞り込み条件を変えるたびに
// 合計を別途取得する必要がなくなり、リクエスト数を削減できる。
func (r *Transaction) List(ctx context.Context, userID int64, f ListFilter) (*ListResult, error) {
	base := r.scoped(ctx, userID)
	base = applyFilter(base, f)

	// 集計は件数と合計を 1 クエリで取る
	var summary struct {
		Total            int64 `gorm:"column:total"`
		TotalAmountMinor int64 `gorm:"column:total_amount_minor"`
	}
	err := base.Session(&gorm.Session{}).
		Select("COUNT(*) AS total, COALESCE(SUM(amount_jpy_minor), 0) AS total_amount_minor").
		Scan(&summary).Error
	if err != nil {
		return nil, fmt.Errorf("取引の集計に失敗しました: %w", err)
	}

	var rows []TransactionRow
	err = base.Session(&gorm.Session{}).
		Select(transactionSelect).
		Joins(transactionJoins).
		Order("transactions.occurred_at DESC, transactions.id DESC").
		Limit(f.Limit).
		Offset(f.Offset).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("取引一覧の取得に失敗しました: %w", err)
	}

	return &ListResult{
		Rows:             rows,
		Total:            summary.Total,
		TotalAmountMinor: summary.TotalAmountMinor,
	}, nil
}

// FindByID は取引を 1 件取得する（API-012）。所有者の確認は呼び出し側が行う。
//
// 論理削除済みの行は返さない。クライアントから見れば削除されているため、
// 存在しないものとして扱う。
func (r *Transaction) FindByID(ctx context.Context, id int64) (*TransactionRow, error) {
	var row TransactionRow

	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Select(transactionSelect).
		Joins(transactionJoins).
		Where("transactions.id = ? AND transactions.deleted_at IS NULL", id).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("取引の取得に失敗しました: %w", err)
	}
	return &row, nil
}

// Create は取引を作成する（API-011）。
func (r *Transaction) Create(ctx context.Context, txn *model.Transaction) error {
	if err := r.db.WithContext(ctx).Create(txn).Error; err != nil {
		return fmt.Errorf("取引の作成に失敗しました: %w", err)
	}
	return nil
}

// Update は指定した列だけを更新する（API-013）。
func (r *Transaction) Update(ctx context.Context, id int64, fields map[string]any) error {
	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(fields).Error
	if err != nil {
		return fmt.Errorf("取引の更新に失敗しました: %w", err)
	}
	return nil
}

// SoftDelete は取引を論理削除する（API-014）。
//
// 物理削除すると payment_events.transaction_id が SET NULL となり、
// イベントが未名寄せに戻って次のバッチで同じ取引が再生成される（DB 仕様書 3.7）。
func (r *Transaction) SoftDelete(ctx context.Context, id int64, now time.Time) error {
	err := r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", now).Error
	if err != nil {
		return fmt.Errorf("取引の削除に失敗しました: %w", err)
	}
	return nil
}

// scoped はユーザと論理削除で絞った状態のクエリを返す。
func (r *Transaction) scoped(ctx context.Context, userID int64) *gorm.DB {
	return r.db.WithContext(ctx).
		Model(&model.Transaction{}).
		Where("transactions.user_id = ? AND transactions.deleted_at IS NULL", userID)
}

func applyFilter(query *gorm.DB, f ListFilter) *gorm.DB {
	if f.From != nil {
		query = query.Where("transactions.occurred_at >= ?", *f.From)
	}
	if f.To != nil {
		query = query.Where("transactions.occurred_at < ?", *f.To)
	}
	if f.CategoryID != nil {
		query = query.Where("transactions.category_id = ?", *f.CategoryID)
	}
	if f.PaymentMethodID != nil {
		query = query.Where("transactions.payment_method_id = ?", *f.PaymentMethodID)
	}
	if f.MerchantID != nil {
		query = query.Where("transactions.merchant_id = ?", *f.MerchantID)
	}
	if f.NeedsReview != nil {
		query = query.Where("transactions.is_possible_duplicate = ?", *f.NeedsReview)
	}
	if f.Query != nil {
		// 店舗名とメモの部分一致。ILIKE で大文字小文字を区別しない
		pattern := "%" + escapeLike(*f.Query) + "%"
		query = query.Where(`(
			transactions.note ILIKE ?
			OR transactions.merchant_id IN (
				SELECT id FROM merchants WHERE user_id = transactions.user_id AND name ILIKE ?
			)
		)`, pattern, pattern)
	}
	return query
}

// escapeLike は LIKE のワイルドカードを打ち消す。
// 検索語に含まれる % や _ がパターンとして解釈されないようにする。
func escapeLike(s string) string {
	var escaped []rune
	for _, r := range s {
		if r == '%' || r == '_' || r == '\\' {
			escaped = append(escaped, '\\')
		}
		escaped = append(escaped, r)
	}
	return string(escaped)
}
