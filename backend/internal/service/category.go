package service

import (
	"context"
	"errors"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// カテゴリ固有のエラー。
var (
	// ErrDuplicateCategoryName は同名のカテゴリが既に存在する。409 に対応する。
	ErrDuplicateCategoryName = errors.New("同名のカテゴリが存在します")
	// ErrParentNotFound は指定した親カテゴリが存在しない。404 に対応する。
	ErrParentNotFound = errors.New("指定した親カテゴリが存在しません")
	// ErrInvalidParent は自分自身または子孫を親に指定した（循環する）。400 に対応する。
	ErrInvalidParent = errors.New("自分自身または子孫を親に指定できません")
	// ErrSystemCategoryNotDeletable は初期カテゴリを削除しようとした。400 に対応する。
	ErrSystemCategoryNotDeletable = errors.New("初期カテゴリは削除できません")
)

// CategoryNameMaxLen はカテゴリ名の最大長（DB 仕様書 3.11 の varchar(50)）。
const CategoryNameMaxLen = 50

// Category はカテゴリの参照と更新を担う（API-019 / 020 / 021）。
type Category struct {
	repo *repository.Category
}

// NewCategory は Category サービスを生成する。
func NewCategory(repo *repository.Category) *Category {
	return &Category{repo: repo}
}

// List はユーザのカテゴリを表示順に返す（API-019）。
func (s *Category) List(ctx context.Context, userID int64, withCounts bool) ([]repository.CategoryWithCount, error) {
	return s.repo.List(ctx, userID, withCounts)
}

// CreateCategoryInput は API-020 のリクエスト。
type CreateCategoryInput struct {
	Name      string
	ParentID  *int64
	SortOrder int
}

// Create はカテゴリを作成する（API-020）。
func (s *Category) Create(ctx context.Context, userID int64, in CreateCategoryInput) (*model.Category, error) {
	if err := validateCategoryName(in.Name); err != nil {
		return nil, err
	}

	if in.ParentID != nil {
		if err := s.requireOwnedParent(ctx, *in.ParentID, userID); err != nil {
			return nil, err
		}
	}

	category := &model.Category{
		UserID:    userID,
		Name:      in.Name,
		ParentID:  in.ParentID,
		SortOrder: in.SortOrder,
		// ユーザが作るカテゴリは常に削除可能。is_system はログイン時の
		// 初期カテゴリにのみ立てる（repository.defaultCategories）
		IsSystem: false,
	}

	if err := s.repo.Create(ctx, category); err != nil {
		if errors.Is(err, repository.ErrDuplicateName) {
			return nil, ErrDuplicateCategoryName
		}
		return nil, err
	}
	return category, nil
}

// UpdateCategoryInput は API-021 PUT のリクエスト。
// 指定されたフィールドのみを更新する。
type UpdateCategoryInput struct {
	Name      httpx.Optional[*string]
	ParentID  httpx.Optional[*int64]
	SortOrder httpx.Optional[*int]
}

// Update はカテゴリを更新する（API-021 PUT）。
func (s *Category) Update(ctx context.Context, userID, id int64, in UpdateCategoryInput) (*model.Category, error) {
	current, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	fields := make(map[string]any, 3)

	if in.Name.Set {
		if in.Name.Value == nil {
			return nil, ErrValidation("name に null は指定できません")
		}
		if verr := validateCategoryName(*in.Name.Value); verr != nil {
			return nil, verr
		}
		fields["name"] = *in.Name.Value
	}

	if in.ParentID.Set {
		if perr := s.validateNewParent(ctx, userID, id, in.ParentID.Value); perr != nil {
			return nil, perr
		}
		// null が指定された場合は親を外す
		fields["parent_id"] = in.ParentID.Value
	}

	if in.SortOrder.Set {
		if in.SortOrder.Value == nil {
			return nil, ErrValidation("sort_order に null は指定できません")
		}
		fields["sort_order"] = *in.SortOrder.Value
	}

	if len(fields) == 0 {
		// 更新対象が無いので DB に触らず現在の状態を返す
		return current, nil
	}

	if uerr := s.repo.Update(ctx, id, fields); uerr != nil {
		if errors.Is(uerr, repository.ErrDuplicateName) {
			return nil, ErrDuplicateCategoryName
		}
		return nil, uerr
	}

	return s.repo.FindByID(ctx, id)
}

// Delete はカテゴリを物理削除する（API-021 DELETE）。
func (s *Category) Delete(ctx context.Context, userID, id int64) error {
	category, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return err
	}

	// 「未分類」は自動分類が失敗したときの受け皿になるため消せると困る
	if category.IsSystem {
		return ErrSystemCategoryNotDeletable
	}

	return s.repo.Delete(ctx, id)
}

// findOwned はカテゴリを取得し、所有者であることを確認する。
func (s *Category) findOwned(ctx context.Context, userID, id int64) (*model.Category, error) {
	category, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if category.UserID != userID {
		return nil, ErrForbidden
	}
	return category, nil
}

// validateNewParent は親カテゴリの付け替えが妥当かを確認する。
func (s *Category) validateNewParent(ctx context.Context, userID, id int64, newParentID *int64) error {
	if newParentID == nil {
		return nil // 親を外すだけなので循環しない
	}

	if err := s.requireOwnedParent(ctx, *newParentID, userID); err != nil {
		return err
	}

	// 自分自身や子孫を親にすると循環する。
	// 新しい親から親を辿って自分に到達するなら、その親は自分の子孫である
	cyclic, err := s.repo.IsDescendantOrSelf(ctx, *newParentID, id, userID)
	if err != nil {
		return err
	}
	if cyclic {
		return ErrInvalidParent
	}
	return nil
}

// requireOwnedParent は親カテゴリが自分のものとして存在することを確認する。
//
// FK 制約だけでは他ユーザのカテゴリを親にできてしまうため、
// user_id まで含めて確認する。
func (s *Category) requireOwnedParent(ctx context.Context, parentID, userID int64) error {
	exists, err := s.repo.ExistsForUser(ctx, parentID, userID)
	if err != nil {
		return err
	}
	if !exists {
		return ErrParentNotFound
	}
	return nil
}

func validateCategoryName(name string) error {
	if name == "" {
		return ErrValidation("name は必須です")
	}
	if len([]rune(name)) > CategoryNameMaxLen {
		return ErrValidation("name は 50 文字以内で指定してください")
	}
	return nil
}
