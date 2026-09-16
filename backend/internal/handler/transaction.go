package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/service"
)

// 取引まわりのエラーコード。
const (
	codeTransactionNotFound   = "TRANSACTION_NOT_FOUND"
	codePaymentMethodNotFound = "PAYMENT_METHOD_NOT_FOUND"
	codeMerchantNotFound      = "MERCHANT_NOT_FOUND"
	codeInvalidAmount         = "INVALID_AMOUNT"
	codeInvalidDateRange      = "INVALID_DATE_RANGE"
	codeUnsupportedCurrency   = "UNSUPPORTED_CURRENCY"
	codeAlreadyMerged         = "ALREADY_MERGED"
)

// Transaction は API-010〜014 を扱う。
type Transaction struct {
	svc *service.Transaction
}

// NewTransaction は Transaction ハンドラを生成する。
func NewTransaction(svc *service.Transaction) *Transaction {
	return &Transaction{svc: svc}
}

type refResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type merchantResponse struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Address *string `json:"address"`
}

type paymentMethodResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type transactionResponse struct {
	ID                  int64                  `json:"id"`
	OccurredAt          string                 `json:"occurred_at"`
	AmountMinor         int64                  `json:"amount_minor"`
	Currency            string                 `json:"currency"`
	Merchant            *merchantResponse      `json:"merchant"`
	Category            *refResponse           `json:"category"`
	PaymentMethod       *paymentMethodResponse `json:"payment_method"`
	Source              string                 `json:"source"`
	Status              string                 `json:"status"`
	IsPossibleDuplicate bool                   `json:"is_possible_duplicate"`
	EventCount          int                    `json:"event_count"`
}

// transactionDetailResponse は API-012 が返す詳細。一覧より項目が多い。
type transactionDetailResponse struct {
	transactionResponse
	AmountJPYMinor int64   `json:"amount_jpy_minor"`
	CategorySource string  `json:"category_source"`
	IsUserEdited   bool    `json:"is_user_edited"`
	Note           *string `json:"note"`
	// PaymentEvents は Step 8 以降に実装する。手動登録では常に空配列
	PaymentEvents []struct{} `json:"payment_events"`
}

func toTransactionResponse(row *repository.TransactionRow) transactionResponse {
	res := transactionResponse{
		ID:                  row.ID,
		OccurredAt:          row.OccurredAt.Format(time.RFC3339),
		AmountMinor:         row.AmountMinor,
		Currency:            row.Currency,
		Source:              row.Source,
		Status:              row.Status,
		IsPossibleDuplicate: row.IsPossibleDuplicate,
		// 決済イベントは Step 8 以降。手動登録の取引は常に 0 件
		EventCount: 0,
	}

	if row.MerchantID != nil && row.MerchantName != nil {
		res.Merchant = &merchantResponse{
			ID:      *row.MerchantID,
			Name:    *row.MerchantName,
			Address: row.MerchantAddress,
		}
	}
	if row.CategoryID != nil && row.CategoryName != nil {
		res.Category = &refResponse{ID: *row.CategoryID, Name: *row.CategoryName}
	}
	if row.PaymentMethodID != nil && row.PaymentMethodName != nil {
		kind := ""
		if row.PaymentMethodKind != nil {
			kind = *row.PaymentMethodKind
		}
		res.PaymentMethod = &paymentMethodResponse{
			ID:   *row.PaymentMethodID,
			Name: *row.PaymentMethodName,
			Kind: kind,
		}
	}
	return res
}

// List は API-010 取引一覧取得。
func (h *Transaction) List(w http.ResponseWriter, r *http.Request, userID int64) {
	filter, apiErr := parseListFilter(r)
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	result, err := h.svc.List(r.Context(), userID, *filter)
	if err != nil {
		h.writeError(w, err)
		return
	}

	items := make([]transactionResponse, 0, len(result.Rows))
	for i := range result.Rows {
		items = append(items, toTransactionResponse(&result.Rows[i]))
	}

	httpx.WriteData(w, http.StatusOK, items, map[string]any{
		"total":              result.Total,
		"total_amount_minor": result.TotalAmountMinor,
	})
}

// Get は API-012 取引詳細取得。
func (h *Transaction) Get(w http.ResponseWriter, r *http.Request, userID int64) {
	id, apiErr := httpx.PathID(r, "id", codeTransactionNotFound, "取引が存在しません")
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	row, err := h.svc.Find(r.Context(), userID, id)
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.WriteData(w, http.StatusOK, toDetailResponse(row), nil)
}

func toDetailResponse(row *repository.TransactionRow) transactionDetailResponse {
	return transactionDetailResponse{
		transactionResponse: toTransactionResponse(row),
		AmountJPYMinor:      row.AmountJPYMinor,
		CategorySource:      row.CategorySource,
		IsUserEdited:        row.IsUserEdited,
		Note:                row.Note,
		PaymentEvents:       []struct{}{},
	}
}

type createTransactionRequest struct {
	OccurredAt      string  `json:"occurred_at"`
	AmountMinor     int64   `json:"amount_minor"`
	Currency        string  `json:"currency"`
	MerchantName    *string `json:"merchant_name"`
	CategoryID      *int64  `json:"category_id"`
	PaymentMethodID *int64  `json:"payment_method_id"`
	Note            *string `json:"note"`
}

// Create は API-011 取引の手動登録。
func (h *Transaction) Create(w http.ResponseWriter, r *http.Request, userID int64) {
	var req createTransactionRequest
	if apiErr := httpx.DecodeJSON(w, r, &req); apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	occurredAt, apiErr := parseTime(req.OccurredAt, "occurred_at")
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	row, err := h.svc.Create(r.Context(), userID, service.CreateTransactionInput{
		OccurredAt:      occurredAt,
		AmountMinor:     req.AmountMinor,
		Currency:        req.Currency,
		MerchantName:    req.MerchantName,
		CategoryID:      req.CategoryID,
		PaymentMethodID: req.PaymentMethodID,
		Note:            req.Note,
	})
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.WriteData(w, http.StatusCreated, toDetailResponse(row), nil)
}

type updateTransactionRequest struct {
	OccurredAt      httpx.Optional[*string] `json:"occurred_at"`
	AmountMinor     httpx.Optional[*int64]  `json:"amount_minor"`
	MerchantID      httpx.Optional[*int64]  `json:"merchant_id"`
	CategoryID      httpx.Optional[*int64]  `json:"category_id"`
	PaymentMethodID httpx.Optional[*int64]  `json:"payment_method_id"`
	Note            httpx.Optional[*string] `json:"note"`
	// LearnCategory は Step 8 以降で使う。受け付けるが現時点では無視する
	LearnCategory httpx.Optional[*bool] `json:"learn_category"`
}

// Update は API-013 取引の修正。
func (h *Transaction) Update(w http.ResponseWriter, r *http.Request, userID int64) {
	id, apiErr := httpx.PathID(r, "id", codeTransactionNotFound, "取引が存在しません")
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	var req updateTransactionRequest
	if derr := httpx.DecodeJSON(w, r, &req); derr != nil {
		httpx.WriteError(w, derr)
		return
	}

	in := service.UpdateTransactionInput{
		AmountMinor:     req.AmountMinor,
		MerchantID:      req.MerchantID,
		CategoryID:      req.CategoryID,
		PaymentMethodID: req.PaymentMethodID,
		Note:            req.Note,
	}

	if req.OccurredAt.Set {
		in.OccurredAt.Set = true
		if req.OccurredAt.Value != nil {
			occurredAt, perr := parseTime(*req.OccurredAt.Value, "occurred_at")
			if perr != nil {
				httpx.WriteError(w, perr)
				return
			}
			in.OccurredAt.Value = &occurredAt
		}
	}

	row, err := h.svc.Update(r.Context(), userID, id, in)
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.WriteData(w, http.StatusOK, toDetailResponse(row), map[string]any{
		// カテゴリ規則の学習は Step 8 以降（API 仕様書 5.1 の ※1）。
		// レスポンスの形は変えず、常に未学習として返す
		"learned_rule":    nil,
		"affected_future": false,
	})
}

// Delete は API-014 取引の削除（論理削除）。
func (h *Transaction) Delete(w http.ResponseWriter, r *http.Request, userID int64) {
	id, apiErr := httpx.PathID(r, "id", codeTransactionNotFound, "取引が存在しません")
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

// parseListFilter はクエリパラメータを絞り込み条件に変換する。
func parseListFilter(r *http.Request) (*repository.ListFilter, *httpx.APIError) {
	q := r.URL.Query()
	filter := &repository.ListFilter{}

	// from / to は日付（YYYY-MM-DD）で受け取り、JST の 0 時を境界にする
	if v := q.Get("from"); v != "" {
		t, err := parseDateJST(v)
		if err != nil {
			return nil, httpx.ValidationError("from は YYYY-MM-DD 形式で指定してください")
		}
		filter.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := parseDateJST(v)
		if err != nil {
			return nil, httpx.ValidationError("to は YYYY-MM-DD 形式で指定してください")
		}
		// to はその日を含めるため、翌日の 0 時未満とする
		end := t.AddDate(0, 0, 1)
		filter.To = &end
	}

	for _, p := range []struct {
		name string
		dest **int64
	}{
		{name: "category_id", dest: &filter.CategoryID},
		{name: "payment_method_id", dest: &filter.PaymentMethodID},
		{name: "merchant_id", dest: &filter.MerchantID},
	} {
		if v := q.Get(p.name); v != "" {
			id, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, httpx.ValidationError(p.name + " は整数で指定してください")
			}
			*p.dest = &id
		}
	}

	if v := q.Get("needs_review"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, httpx.ValidationError("needs_review は true / false で指定してください")
		}
		filter.NeedsReview = &b
	}

	if v := q.Get("q"); v != "" {
		filter.Query = &v
	}

	for _, p := range []struct {
		name string
		dest *int
	}{
		{name: "limit", dest: &filter.Limit},
		{name: "offset", dest: &filter.Offset},
	} {
		if v := q.Get(p.name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				return nil, httpx.ValidationError(p.name + " は 0 以上の整数で指定してください")
			}
			*p.dest = n
		}
	}

	return filter, nil
}

// jst は集計の基準タイムゾーン（DB 仕様書 1 章）。
var jst = time.FixedZone("Asia/Tokyo", 9*60*60)

func parseDateJST(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, jst)
}

// parseTime は ISO 8601 の日時を解釈する。
// タイムゾーンオフセットを必ず含むこと（API 仕様書 2 章）。
func parseTime(value, field string) (time.Time, *httpx.APIError) {
	if value == "" {
		return time.Time{}, httpx.ValidationError(field + " は必須です")
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, httpx.ValidationError(
			field + " は ISO 8601（タイムゾーンオフセットを含む）で指定してください")
	}
	return t, nil
}

// writeError はサービス層のエラーを API 仕様書のステータスとエラーコードに対応づける。
func (h *Transaction) writeError(w http.ResponseWriter, err error) {
	var validationErr *service.ValidationError

	switch {
	case errors.As(err, &validationErr):
		httpx.WriteError(w, httpx.ValidationError(validationErr.Message))

	case errors.Is(err, service.ErrInvalidAmount):
		httpx.WriteError(w, httpx.BadRequest(codeInvalidAmount, "金額は 1 以上で指定してください"))

	case errors.Is(err, service.ErrInvalidDateRange):
		httpx.WriteError(w, httpx.BadRequest(codeInvalidDateRange, "from が to より後です"))

	case errors.Is(err, service.ErrLimitTooLarge):
		httpx.WriteError(w, httpx.BadRequest(
			httpx.CodeLimitTooLarge, "limit は 100 以下で指定してください"))

	case errors.Is(err, service.ErrUnsupportedCurrency):
		httpx.WriteError(w, httpx.BadRequest(
			codeUnsupportedCurrency, "現時点では JPY のみ対応しています"))

	case errors.Is(err, service.ErrForbidden):
		httpx.WriteError(w, httpx.Forbidden("他ユーザの取引です"))

	case errors.Is(err, service.ErrNotFound), errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, httpx.NotFound(codeTransactionNotFound, "取引が存在しません"))

	case errors.Is(err, service.ErrCategoryNotFound):
		httpx.WriteError(w, httpx.NotFound(
			codeCategoryNotFound, "指定したカテゴリが存在しません"))

	case errors.Is(err, service.ErrPaymentMethodNotFound):
		httpx.WriteError(w, httpx.NotFound(
			codePaymentMethodNotFound, "指定した決済手段が存在しません"))

	case errors.Is(err, service.ErrMerchantNotFound):
		httpx.WriteError(w, httpx.NotFound(codeMerchantNotFound, "指定した店舗が存在しません"))

	case errors.Is(err, service.ErrAlreadyMerged):
		httpx.WriteError(w, httpx.Conflict(
			codeAlreadyMerged, "マージの統合元は単独で削除できません"))

	default:
		slog.Error("取引の処理に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
	}
}
