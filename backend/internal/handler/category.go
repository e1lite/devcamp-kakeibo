package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/service"
)

// カテゴリまわりのエラーコード。
const (
	codeCategoryNotFound      = "CATEGORY_NOT_FOUND"
	codeParentNotFound        = "PARENT_NOT_FOUND"
	codeDuplicateCategoryName = "DUPLICATE_CATEGORY_NAME"
	codeInvalidParent         = "INVALID_PARENT"
	codeSystemCategory        = "SYSTEM_CATEGORY_NOT_DELETABLE"
)

// Category は API-019 / 020 / 021 を扱う。
type Category struct {
	svc *service.Category
}

// NewCategory は Category ハンドラを生成する。
func NewCategory(svc *service.Category) *Category {
	return &Category{svc: svc}
}

type categoryResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder int    `json:"sort_order"`
	IsSystem  bool   `json:"is_system"`
	// TransactionCount は include_counts=true のときのみ含める
	TransactionCount *int `json:"transaction_count,omitempty"`
}

// List は API-019 カテゴリ一覧。
func (h *Category) List(w http.ResponseWriter, r *http.Request, userID int64) {
	withCounts := r.URL.Query().Get("include_counts") == "true"

	categories, err := h.svc.List(r.Context(), userID, withCounts)
	if err != nil {
		slog.Error("カテゴリ一覧の取得に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}

	items := make([]categoryResponse, 0, len(categories))
	for _, c := range categories {
		item := categoryResponse{
			ID:        c.ID,
			Name:      c.Name,
			ParentID:  c.ParentID,
			SortOrder: c.SortOrder,
			IsSystem:  c.IsSystem,
		}
		if withCounts {
			count := c.TransactionCount
			item.TransactionCount = &count
		}
		items = append(items, item)
	}

	httpx.WriteData(w, http.StatusOK, items, map[string]any{"total": len(items)})
}

type createCategoryRequest struct {
	Name      string `json:"name"`
	ParentID  *int64 `json:"parent_id"`
	SortOrder *int   `json:"sort_order"`
}

// Create は API-020 カテゴリ作成。
func (h *Category) Create(w http.ResponseWriter, r *http.Request, userID int64) {
	var req createCategoryRequest
	if apiErr := httpx.DecodeJSON(w, r, &req); apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	in := service.CreateCategoryInput{Name: req.Name, ParentID: req.ParentID}
	if req.SortOrder != nil {
		in.SortOrder = *req.SortOrder
	}

	category, err := h.svc.Create(r.Context(), userID, in)
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.WriteData(w, http.StatusCreated, categoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		ParentID:  category.ParentID,
		SortOrder: category.SortOrder,
		IsSystem:  category.IsSystem,
	}, nil)
}

type updateCategoryRequest struct {
	Name      httpx.Optional[*string] `json:"name"`
	ParentID  httpx.Optional[*int64]  `json:"parent_id"`
	SortOrder httpx.Optional[*int]    `json:"sort_order"`
}

// Update は API-021 PUT カテゴリ更新。
func (h *Category) Update(w http.ResponseWriter, r *http.Request, userID int64) {
	id, apiErr := httpx.PathID(r, "id", codeCategoryNotFound, "カテゴリが存在しません")
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	var req updateCategoryRequest
	if derr := httpx.DecodeJSON(w, r, &req); derr != nil {
		httpx.WriteError(w, derr)
		return
	}

	category, err := h.svc.Update(r.Context(), userID, id, service.UpdateCategoryInput{
		Name:      req.Name,
		ParentID:  req.ParentID,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.WriteData(w, http.StatusOK, categoryResponse{
		ID:        category.ID,
		Name:      category.Name,
		ParentID:  category.ParentID,
		SortOrder: category.SortOrder,
		IsSystem:  category.IsSystem,
	}, nil)
}

// Delete は API-021 DELETE カテゴリ削除。
func (h *Category) Delete(w http.ResponseWriter, r *http.Request, userID int64) {
	id, apiErr := httpx.PathID(r, "id", codeCategoryNotFound, "カテゴリが存在しません")
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	if err := h.svc.Delete(r.Context(), userID, id); err != nil {
		h.writeError(w, err)
		return
	}

	httpx.WriteNoContent(w)
}

// writeError はサービス層のエラーを API 仕様書のステータスとエラーコードに対応づける。
func (h *Category) writeError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError

	switch {
	case errors.As(err, &validationErr):
		httpx.WriteError(w, httpx.ValidationError(validationErr.Message))

	case errors.Is(err, service.ErrInvalidParent):
		httpx.WriteError(w, httpx.BadRequest(
			codeInvalidParent, "自分自身または子孫を親に指定できません"))

	case errors.Is(err, service.ErrSystemCategoryNotDeletable):
		httpx.WriteError(w, httpx.BadRequest(
			codeSystemCategory, "初期カテゴリは削除できません"))

	case errors.Is(err, service.ErrForbidden):
		httpx.WriteError(w, httpx.Forbidden("他ユーザのカテゴリです"))

	case errors.Is(err, service.ErrNotFound), errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, httpx.NotFound(codeCategoryNotFound, "カテゴリが存在しません"))

	case errors.Is(err, service.ErrParentNotFound):
		httpx.WriteError(w, httpx.NotFound(
			codeParentNotFound, "指定した親カテゴリが存在しません"))

	case errors.Is(err, service.ErrDuplicateCategoryName):
		httpx.WriteError(w, httpx.Conflict(
			codeDuplicateCategoryName, "同名のカテゴリが存在します"))

	default:
		slog.Error("カテゴリの処理に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
	}
}
