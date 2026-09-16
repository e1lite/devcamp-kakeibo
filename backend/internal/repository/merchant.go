package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
)

// Merchant は merchants テーブルへのアクセスを提供する。
type Merchant struct {
	db *gorm.DB
}

// NewMerchant は Merchant リポジトリを生成する。
func NewMerchant(db *gorm.DB) *Merchant {
	return &Merchant{db: db}
}

// FindOrCreateByName は名前で店舗を探し、無ければ作成する（API-011）。
//
// 手動登録では店舗名が文字列で渡されるため、同じ名前で何度登録しても
// 店舗が増えないようにする。UNIQUE(user_id, name) があるため、
// 競合した場合は作成に失敗した側が既存行を読み直す。
func (r *Merchant) FindOrCreateByName(ctx context.Context, userID int64, name string) (*model.Merchant, error) {
	found, err := r.findByName(ctx, userID, name)
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	merchant := &model.Merchant{
		UserID: userID,
		Name:   name,
		// 手動登録の店舗は住所が無いため、ジオコーディングの対象にしない
		GeocodeStatus: "skipped",
	}
	if cerr := r.db.WithContext(ctx).Create(merchant).Error; cerr != nil {
		if isUniqueViolation(cerr) {
			// 同時に同じ名前が作られた。作られた行を読み直す
			return r.findByName(ctx, userID, name)
		}
		return nil, fmt.Errorf("店舗の作成に失敗しました: %w", cerr)
	}
	return merchant, nil
}

func (r *Merchant) findByName(ctx context.Context, userID int64, name string) (*model.Merchant, error) {
	var merchant model.Merchant
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND name = ?", userID, name).
		First(&merchant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("店舗の取得に失敗しました: %w", err)
	}
	return &merchant, nil
}

// ExistsForUser は指定した店舗がそのユーザのものとして存在するかを返す。
func (r *Merchant) ExistsForUser(ctx context.Context, id, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Merchant{}).
		Where("id = ? AND user_id = ?", id, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("店舗の確認に失敗しました: %w", err)
	}
	return count > 0, nil
}

// PaymentMethod は payment_methods テーブルへのアクセスを提供する。
type PaymentMethod struct {
	db *gorm.DB
}

// NewPaymentMethod は PaymentMethod リポジトリを生成する。
func NewPaymentMethod(db *gorm.DB) *PaymentMethod {
	return &PaymentMethod{db: db}
}

// ExistsForUser は指定した決済手段がそのユーザのものとして存在するかを返す。
func (r *PaymentMethod) ExistsForUser(ctx context.Context, id, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.PaymentMethod{}).
		Where("id = ? AND user_id = ?", id, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("決済手段の確認に失敗しました: %w", err)
	}
	return count > 0, nil
}
