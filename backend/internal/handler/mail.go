package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/service"
)

// メール連携まわりのエラーコード。
const (
	codeMailAccountNotFound = "MAIL_ACCOUNT_NOT_FOUND"
	codeAlreadyDisabled     = "ALREADY_DISABLED"
	codeSyncAlreadyRunning  = "SYNC_ALREADY_RUNNING"
	codeAccountDisabled     = "ACCOUNT_DISABLED"
	codeReauthRequired      = "REAUTH_REQUIRED"
	codeInsufficientScope   = "INSUFFICIENT_SCOPE"
)

// Mail は API-006〜009（連携の一覧・解除・同期・同期履歴）と API-029 / 030（連携）を扱う。
type Mail struct {
	svc *service.Mail
	now func() time.Time
}

// NewMail は Mail ハンドラを生成する。
func NewMail(svc *service.Mail) *Mail {
	return &Mail{svc: svc, now: time.Now}
}

type mailAccountResponse struct {
	ID                  int64   `json:"id"`
	EmailAddress        string  `json:"email_address"`
	Provider            string  `json:"provider"`
	Status              string  `json:"status"`
	LastSyncedAt        *string `json:"last_synced_at"`
	CredentialExpiresAt *string `json:"credential_expires_at"`
	BackfilledUntil     *string `json:"backfilled_until"`
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

// ListAccounts は API-006 連携メールアカウント一覧。
//
// credential_encrypted はいかなる場合もレスポンスに含めない（API-006 の注記）。
func (h *Mail) ListAccounts(w http.ResponseWriter, r *http.Request, userID int64) {
	accounts, err := h.svc.ListAccounts(r.Context(), userID)
	if err != nil {
		slog.Error("連携の一覧の取得に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}

	items := make([]mailAccountResponse, 0, len(accounts))
	for _, a := range accounts {
		items = append(items, mailAccountResponse{
			ID:                  a.ID,
			EmailAddress:        a.EmailAddress,
			Provider:            a.Provider,
			Status:              a.Status,
			LastSyncedAt:        formatOptionalTime(a.LastSyncedAt),
			CredentialExpiresAt: formatOptionalTime(a.CredentialExpiresAt),
			BackfilledUntil:     formatOptionalTime(a.BackfilledUntil),
		})
	}
	httpx.WriteData(w, http.StatusOK, items, map[string]any{"total": len(items)})
}

// Disconnect は API-007 連携解除。行は削除せず、認証情報だけを破棄する。
func (h *Mail) Disconnect(w http.ResponseWriter, r *http.Request, userID int64) {
	id, apiErr := httpx.PathID(r, "id", codeMailAccountNotFound, "連携が存在しません")
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}
	if err := h.svc.Disconnect(r.Context(), userID, id); err != nil {
		h.writeError(w, err)
		return
	}
	httpx.WriteNoContent(w)
}

type syncRequest struct {
	Mode string `json:"mode"`
}

type syncJobResponse struct {
	ID            int64   `json:"id"`
	MailAccountID int64   `json:"mail_account_id"`
	Trigger       string  `json:"trigger"`
	StartedAt     string  `json:"started_at"`
	FinishedAt    *string `json:"finished_at"`
	Status        string  `json:"status"`
	FetchedCount  int     `json:"fetched_count"`
	ParsedCount   int     `json:"parsed_count"`
	FailedCount   int     `json:"failed_count"`
	ErrorMessage  *string `json:"error_message"`
}

// Sync は API-008 手動同期の実行。完了を待たずに 202 を返す。
func (h *Mail) Sync(w http.ResponseWriter, r *http.Request, userID int64) {
	id, apiErr := httpx.PathID(r, "id", codeMailAccountNotFound, "連携が存在しません")
	if apiErr != nil {
		httpx.WriteError(w, apiErr)
		return
	}

	// ボディは省略できる（mode の既定は incremental）
	var req syncRequest
	if hasBody(r) {
		if derr := httpx.DecodeJSON(w, r, &req); derr != nil {
			httpx.WriteError(w, derr)
			return
		}
	}
	if req.Mode != "" && req.Mode != "incremental" && req.Mode != "backfill" {
		httpx.WriteError(w, httpx.ValidationError("mode は incremental か backfill で指定してください"))
		return
	}

	job, err := h.svc.RequestSync(r.Context(), userID, id, req.Mode == "backfill")
	if err != nil {
		h.writeError(w, err)
		return
	}

	httpx.WriteData(w, http.StatusAccepted, map[string]any{
		"sync_job_id": job.ID,
		"status":      job.Status,
		"trigger":     job.Trigger,
		"started_at":  job.StartedAt.Format(time.RFC3339),
	}, nil)
}

// ListJobs は API-009 同期履歴取得。
func (h *Mail) ListJobs(w http.ResponseWriter, r *http.Request, userID int64) {
	q := r.URL.Query()
	filter := repository.SyncJobFilter{}

	if v := q.Get("mail_account_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			httpx.WriteError(w, httpx.ValidationError("mail_account_id は整数で指定してください"))
			return
		}
		filter.MailAccountID = &id
	}
	if v := q.Get("status"); v != "" {
		switch v {
		case model.SyncStatusRunning, model.SyncStatusSuccess, model.SyncStatusPartial, model.SyncStatusFailed:
			filter.Status = &v
		default:
			httpx.WriteError(w, httpx.ValidationError("status は running / success / partial / failed で指定してください"))
			return
		}
	}
	for _, p := range []struct {
		name string
		dest *int
	}{{"limit", &filter.Limit}, {"offset", &filter.Offset}} {
		if v := q.Get(p.name); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				httpx.WriteError(w, httpx.ValidationError(p.name+" は 0 以上の整数で指定してください"))
				return
			}
			*p.dest = n
		}
	}

	jobs, total, err := h.svc.ListJobs(r.Context(), userID, filter)
	if err != nil {
		h.writeError(w, err)
		return
	}

	items := make([]syncJobResponse, 0, len(jobs))
	for _, j := range jobs {
		items = append(items, syncJobResponse{
			ID:            j.ID,
			MailAccountID: j.MailAccountID,
			Trigger:       j.Trigger,
			StartedAt:     j.StartedAt.Format(time.RFC3339),
			FinishedAt:    formatOptionalTime(j.FinishedAt),
			Status:        j.Status,
			FetchedCount:  j.FetchedCount,
			ParsedCount:   j.ParsedCount,
			FailedCount:   j.FailedCount,
			ErrorMessage:  j.ErrorMessage,
		})
	}
	httpx.WriteData(w, http.StatusOK, items, map[string]any{"total": total})
}

type linkStartRequest struct {
	RedirectURI string `json:"redirect_uri"`
}

// GoogleStart は API-029 メール連携開始。認可 URL を返し、フロントがブラウザごと遷移する。
//
// API-002 と違い GET のリダイレクトにしないのは、どのユーザの連携かを特定するため
// Bearer トークン付きで呼ぶ必要があるから（ブラウザの遷移では Authorization ヘッダを付けられない）。
func (h *Mail) GoogleStart(w http.ResponseWriter, r *http.Request, userID int64) {
	var req linkStartRequest
	if hasBody(r) {
		if derr := httpx.DecodeJSON(w, r, &req); derr != nil {
			httpx.WriteError(w, derr)
			return
		}
	}

	authURL, err := h.svc.AuthorizeURL(r.Context(), userID, req.RedirectURI, h.now())
	if err != nil {
		if errors.Is(err, service.ErrInvalidRedirectURI) {
			httpx.WriteError(w, httpx.BadRequest(codeInvalidRedirectURI, "redirect_uri が許可リストにありません"))
			return
		}
		slog.Error("メール連携の認可 URL の生成に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}

	httpx.WriteData(w, http.StatusOK, map[string]any{
		"authorization_url": authURL,
		"expires_in":        600,
	}, nil)
}

// GoogleCallback は API-030 メール連携コールバック。認証不要（ユーザは state から特定する）。
//
// JSON は返さず、API-029 で保存した戻り先（メール連携画面）へ 302 でリダイレクトし、
// 結果をフラグメントで渡す。取得した Google の認証情報はフロントに一切渡さない。
func (h *Mail) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	state := query.Get("state")

	// 同意画面で拒否された。state を検証できれば保存した戻り先へ、できなければ既定の戻り先へ
	if query.Get("error") != "" {
		data, _ := h.svc.ConsumeState(state, h.now())
		h.redirectResult(w, r, data.RedirectURI, url.Values{"mail_link": {"error"}, "code": {codeAccessDenied}})
		return
	}

	code := query.Get("code")
	if code == "" || state == "" {
		h.redirectResult(w, r, "", url.Values{"mail_link": {"error"}, "code": {httpx.CodeValidationError}})
		return
	}

	data, err := h.svc.ConsumeState(state, h.now())
	if err != nil {
		// CSRF の可能性、またはログイン用の state を使い回された。詳細は返さずログに残す
		slog.Warn("メール連携の state の検証に失敗しました", "error", err)
		h.redirectResult(w, r, "", url.Values{"mail_link": {"error"}, "code": {codeInvalidState}})
		return
	}

	result, err := h.svc.CompleteLink(r.Context(), data.UserID, code)
	if err != nil {
		errorCode := httpx.CodeInternalError
		switch {
		case errors.Is(err, service.ErrInsufficientScope):
			errorCode = codeInsufficientScope
		case errors.Is(err, service.ErrOAuthExchange):
			errorCode = codeOAuthExchange
		}
		slog.Error("メール連携に失敗しました", "error", err, "code", errorCode)
		h.redirectResult(w, r, data.RedirectURI, url.Values{"mail_link": {"error"}, "code": {errorCode}})
		return
	}

	h.redirectResult(w, r, data.RedirectURI, url.Values{
		"mail_link":       {"success"},
		"mail_account_id": {strconv.FormatInt(result.Account.ID, 10)},
	})
}

// redirectResult はメール連携画面へ、結果をフラグメントに載せてリダイレクトする。
// 戻り先は許可リストのものに限り、それ以外（空を含む）は既定の戻り先にする。
func (h *Mail) redirectResult(w http.ResponseWriter, r *http.Request, redirectURI string, fragment url.Values) {
	if redirectURI == "" || !h.svc.IsAllowedRedirectURI(redirectURI) {
		redirectURI = h.svc.DefaultRedirectURI()
	}

	target, err := url.Parse(redirectURI)
	if err != nil {
		slog.Error("redirect_uri の解析に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}
	target.Fragment = fragment.Encode()

	// 直前で許可リストとの一致を確認済み。target はその文字列をパースしただけ。
	//nolint:gosec // G710: リダイレクト先は許可リストと完全一致することを確認済み
	http.Redirect(w, r, target.String(), http.StatusFound)
}

// writeError はサービス層のエラーを API 仕様書のステータスとエラーコードに対応づける。
func (h *Mail) writeError(w http.ResponseWriter, err error) {
	switch {
	// 他ユーザの連携もここに来る（API 仕様書 6 章冒頭）
	case errors.Is(err, service.ErrNotFound):
		httpx.WriteError(w, httpx.NotFound(codeMailAccountNotFound, "連携が存在しません"))
	case errors.Is(err, service.ErrAlreadyDisabled):
		httpx.WriteError(w, httpx.Conflict(codeAlreadyDisabled, "すでに解除されています"))
	case errors.Is(err, service.ErrSyncAlreadyRunning):
		httpx.WriteError(w, httpx.Conflict(codeSyncAlreadyRunning, "同期が実行中です"))
	case errors.Is(err, service.ErrAccountDisabled):
		httpx.WriteError(w, httpx.Conflict(codeAccountDisabled, "連携が解除されています"))
	case errors.Is(err, service.ErrMailReauthRequired):
		httpx.WriteError(w, httpx.Conflict(codeReauthRequired, "認証の有効期限が切れています。連携し直してください"))
	case errors.Is(err, service.ErrLimitTooLarge):
		httpx.WriteError(w, httpx.BadRequest(httpx.CodeLimitTooLarge, "limit は 100 以下で指定してください"))
	default:
		slog.Error("メール連携の処理に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
	}
}

// hasBody はリクエストにボディがあるかを返す。ボディを省略できる API で使う。
func hasBody(r *http.Request) bool {
	return r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0
}
