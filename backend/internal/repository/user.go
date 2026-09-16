// Package repository はデータベースアクセスを担う。
package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
)

// ErrNotFound はレコードが存在しないことを表す。
var ErrNotFound = errors.New("レコードが見つかりません")

// User は users テーブルへのアクセスを提供する。
type User struct {
	db *gorm.DB
}

// NewUser は User リポジトリを生成する。
func NewUser(db *gorm.DB) *User {
	return &User{db: db}
}

// FindByID はユーザ ID でユーザを取得する。
func (r *User) FindByID(ctx context.Context, id int64) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ユーザの取得に失敗しました: %w", err)
	}
	return &user, nil
}

// FindByGoogleSub は Google アカウントの一意識別子でユーザを取得する。
func (r *User) FindByGoogleSub(ctx context.Context, sub string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("google_sub = ?", sub).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("ユーザの取得に失敗しました: %w", err)
	}
	return &user, nil
}

// CreateWithInitialData はユーザを作成し、初期データも同時に作る。
//
// カテゴリが 1 件もないと取引を登録できないため、
// 初期カテゴリと「現金」の決済手段をここで作る（API 仕様書 API-003）。
// ユーザだけ作られて初期データが無い状態を避けるため、1 つのトランザクションで行う。
func (r *User) CreateWithInitialData(ctx context.Context, user *model.User) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return fmt.Errorf("ユーザの作成に失敗しました: %w", err)
		}

		categories := defaultCategories(user.ID)
		if err := tx.Create(&categories).Error; err != nil {
			return fmt.Errorf("初期カテゴリの作成に失敗しました: %w", err)
		}

		cash := &model.PaymentMethod{
			UserID:   user.ID,
			Name:     "現金",
			Kind:     "cash",
			IsActive: true,
		}
		if err := tx.Create(cash).Error; err != nil {
			return fmt.Errorf("初期決済手段の作成に失敗しました: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// UpdateProfile は Google 側で変わりうるプロフィール情報を反映する。
func (r *User) UpdateProfile(ctx context.Context, user *model.User) error {
	err := r.db.WithContext(ctx).Model(user).
		Select("email", "display_name").
		Updates(map[string]any{
			"email":        user.Email,
			"display_name": user.DisplayName,
		}).Error
	if err != nil {
		return fmt.Errorf("ユーザの更新に失敗しました: %w", err)
	}
	return nil
}

// MailAccountSummary は API-004 が返す連携メールアカウントの集計。
type MailAccountSummary struct {
	Count       int  `gorm:"column:count"`
	NeedsReauth bool `gorm:"column:needs_reauth"`
}

// SummarizeMailAccounts は連携数と、再認証が必要な連携があるかを 1 クエリで取得する。
//
// 解除済み（disabled）は「連携している」とは言えないため件数から除く。
func (r *User) SummarizeMailAccounts(ctx context.Context, userID int64) (*MailAccountSummary, error) {
	var summary MailAccountSummary

	// Select の第 1 引数は 1 つの文字列にまとめる。
	// カンマで区切って複数の引数に分けると、GORM はそれぞれを別の列名として扱い、
	// プレースホルダが置換されないまま SQL が組み立てられる。
	err := r.db.WithContext(ctx).
		Model(&model.MailAccount{}).
		Select(
			"COUNT(*) AS count, COALESCE(BOOL_OR(status = ?), false) AS needs_reauth",
			model.MailAccountStatusReauthRequired,
		).
		Where("user_id = ? AND status <> ?", userID, model.MailAccountStatusDisabled).
		Scan(&summary).Error
	if err != nil {
		return nil, fmt.Errorf("連携メールアカウントの集計に失敗しました: %w", err)
	}

	return &summary, nil
}

// defaultCategories は新規ユーザに作る初期カテゴリを返す。
//
// is_system = true のカテゴリは削除できない（API 仕様書 API-021）。
// 「未分類」は自動分類が失敗したときの受け皿になるため、消せると困る。
func defaultCategories(userID int64) []model.Category {
	names := []string{"食費", "日用品", "交通費", "住居費", "光熱費", "通信費", "娯楽費", "医療費", "未分類"}

	categories := make([]model.Category, 0, len(names))
	for i, name := range names {
		categories = append(categories, model.Category{
			UserID:    userID,
			Name:      name,
			SortOrder: i + 1,
			IsSystem:  true,
		})
	}
	return categories
}
