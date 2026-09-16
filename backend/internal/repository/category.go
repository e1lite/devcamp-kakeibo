package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
)

// ErrDuplicateName は UNIQUE(user_id, name) に違反したことを表す。
var ErrDuplicateName = errors.New("同じ名前がすでに存在します")

// PostgreSQL のエラーコード。
const (
	pgUniqueViolation = "23505"
)

// isUniqueViolation は UNIQUE 制約違反かどうかを判定する。
//
// 事前に SELECT で存在確認をすると、確認から INSERT までの間に
// 他のリクエストが同じ名前を作る余地が残る。制約違反を捕まえる方が競合に強い。
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

// Category は categories テーブルへのアクセスを提供する。
type Category struct {
	db *gorm.DB
}

// NewCategory は Category リポジトリを生成する。
func NewCategory(db *gorm.DB) *Category {
	return &Category{db: db}
}

// CategoryWithCount はカテゴリと、そのカテゴリの取引件数。
type CategoryWithCount struct {
	model.Category
	TransactionCount int `gorm:"column:transaction_count"`
}

// List はユーザのカテゴリを表示順に取得する（API-019）。
//
// マスタであり 1 ユーザあたり数十件を想定するため、ページネーションは設けない。
// withCounts が true の場合のみ取引件数を集計する。
func (r *Category) List(ctx context.Context, userID int64, withCounts bool) ([]CategoryWithCount, error) {
	var categories []CategoryWithCount

	query := r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("categories.user_id = ?", userID).
		Order("categories.sort_order, categories.id")

	// Select は withCounts に関わらず明示する。
	// 省略すると GORM が読み込み先の構造体から列名を推測し、
	// テーブルに存在しない categories.transaction_count を SELECT に含めてしまう。
	if withCounts {
		// 論理削除済みの取引は数えない（DB 仕様書 3.7）。
		// サブクエリで集計してから結合することで、カテゴリ側の
		// GROUP BY を避けつつ 0 件のカテゴリも残す
		query = query.
			Select("categories.*, COALESCE(t.cnt, 0) AS transaction_count").
			Joins(`LEFT JOIN (
				SELECT category_id, COUNT(*) AS cnt
				FROM transactions
				WHERE user_id = ? AND deleted_at IS NULL
				GROUP BY category_id
			) t ON t.category_id = categories.id`, userID)
	} else {
		// 集計しない場合、TransactionCount は走査されずゼロ値のままになる
		query = query.Select("categories.*")
	}

	if err := query.Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("カテゴリ一覧の取得に失敗しました: %w", err)
	}
	return categories, nil
}

// FindByID はカテゴリを取得する。所有者の確認は呼び出し側が行う。
//
// 他ユーザのカテゴリでも行を返すのは、サービス層で 403 と 404 を
// 区別できるようにするため（API 仕様書 6 章冒頭）。
func (r *Category) FindByID(ctx context.Context, id int64) (*model.Category, error) {
	var category model.Category
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("カテゴリの取得に失敗しました: %w", err)
	}
	return &category, nil
}

// Create はカテゴリを作成する。同名が存在する場合は ErrDuplicateName を返す。
func (r *Category) Create(ctx context.Context, category *model.Category) error {
	if err := r.db.WithContext(ctx).Create(category).Error; err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("カテゴリの作成に失敗しました: %w", err)
	}
	return nil
}

// Update は指定した列だけを更新する。同名が存在する場合は ErrDuplicateName を返す。
func (r *Category) Update(ctx context.Context, id int64, fields map[string]any) error {
	err := r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("id = ?", id).
		Updates(fields).Error
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicateName
		}
		return fmt.Errorf("カテゴリの更新に失敗しました: %w", err)
	}
	return nil
}

// Delete はカテゴリを物理削除する（API-021）。
//
// 論理削除にすると、削除済みの行が UNIQUE(user_id, name) を占有し、
// 同名のカテゴリを作り直せなくなる（DB 仕様書 3.7）。
// 参照している取引の category_id は FK の SET NULL により NULL になる。
func (r *Category) Delete(ctx context.Context, id int64) error {
	err := r.db.WithContext(ctx).Delete(&model.Category{}, id).Error
	if err != nil {
		return fmt.Errorf("カテゴリの削除に失敗しました: %w", err)
	}
	return nil
}

// ExistsForUser は指定したカテゴリがそのユーザのものとして存在するかを返す。
// 親カテゴリの検証に使う。FK 制約だけでは他ユーザのカテゴリを親にできてしまう。
func (r *Category) ExistsForUser(ctx context.Context, id, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Category{}).
		Where("id = ? AND user_id = ?", id, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("親カテゴリの確認に失敗しました: %w", err)
	}
	return count > 0, nil
}

// IsDescendantOrSelf は candidate が target 自身、または target の子孫かを返す。
//
// 親の付け替えで循環（A の親が B、B の親が A）が生まれるのを防ぐために使う。
// candidate から親を辿って target に到達すれば、candidate は target の子孫である。
func (r *Category) IsDescendantOrSelf(ctx context.Context, candidate, target, userID int64) (bool, error) {
	const query = `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id FROM categories WHERE id = ? AND user_id = ?
			UNION ALL
			SELECT c.id, c.parent_id
			FROM categories c
			JOIN ancestors a ON c.id = a.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = ?)`

	var found bool
	err := r.db.WithContext(ctx).Raw(query, candidate, userID, target).Scan(&found).Error
	if err != nil {
		return false, fmt.Errorf("カテゴリの循環確認に失敗しました: %w", err)
	}
	return found, nil
}
