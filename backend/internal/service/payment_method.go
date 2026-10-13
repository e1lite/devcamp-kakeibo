package service

import (
	"context"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// PaymentMethod は決済手段の参照を担う（API-031）。
//
// 作成・編集・削除は提供しない。決済手段はメールの解析（P4）でカード下 4 桁などから
// 自動で作られる前提のため（API 仕様書 API-031）。コアの段階ではユーザ作成時の
// 「現金」のみが存在する。
type PaymentMethod struct {
	repo *repository.PaymentMethod
}

// NewPaymentMethod は PaymentMethod サービスを生成する。
func NewPaymentMethod(repo *repository.PaymentMethod) *PaymentMethod {
	return &PaymentMethod{repo: repo}
}

// List はユーザの決済手段を返す（API-031）。
func (s *PaymentMethod) List(ctx context.Context, userID int64, includeInactive bool) ([]model.PaymentMethod, error) {
	return s.repo.List(ctx, userID, includeInactive)
}
