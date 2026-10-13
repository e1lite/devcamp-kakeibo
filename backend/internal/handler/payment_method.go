package handler

import (
	"log/slog"
	"net/http"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/service"
)

// PaymentMethod は API-031 を扱う。
type PaymentMethod struct {
	svc *service.PaymentMethod
}

// NewPaymentMethod は PaymentMethod ハンドラを生成する。
func NewPaymentMethod(svc *service.PaymentMethod) *PaymentMethod {
	return &PaymentMethod{svc: svc}
}

// paymentMethodItem は API-031 の一覧の 1 件。
// 取引に埋め込む paymentMethodResponse より項目が多い。
type paymentMethodItem struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	Issuer    *string `json:"issuer"`
	CardLast4 *string `json:"card_last4"`
	IsActive  bool    `json:"is_active"`
}

// List は API-031 決済手段一覧。
//
// 取引の登録・編集画面の選択肢と、取引一覧の絞り込みに使う。
// 利用停止中も含めたいとき（過去の取引の絞り込み・編集中の現在値の表示）は
// include_inactive=true を付ける（イベント設計 0 章 #4）。
func (h *PaymentMethod) List(w http.ResponseWriter, r *http.Request, userID int64) {
	// API-019 の include_counts と同じく、true 以外は指定なしとして扱う
	includeInactive := r.URL.Query().Get("include_inactive") == "true"

	methods, err := h.svc.List(r.Context(), userID, includeInactive)
	if err != nil {
		slog.Error("決済手段一覧の取得に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}

	items := make([]paymentMethodItem, 0, len(methods))
	for _, m := range methods {
		items = append(items, paymentMethodItem{
			ID:        m.ID,
			Name:      m.Name,
			Kind:      m.Kind,
			Issuer:    m.Issuer,
			CardLast4: m.CardLast4,
			IsActive:  m.IsActive,
		})
	}

	httpx.WriteData(w, http.StatusOK, items, map[string]any{"total": len(items)})
}
