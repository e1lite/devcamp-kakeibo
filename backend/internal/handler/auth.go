package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/httpx"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/service"
)

// 認証まわりのエラーコード。
const (
	codeInvalidRedirectURI = "INVALID_REDIRECT_URI"
	codeInvalidState       = "INVALID_STATE"
	codeOAuthExchange      = "OAUTH_EXCHANGE_FAILED"
	codeAccessDenied       = "ACCESS_DENIED"
	codeUserNotFound       = "USER_NOT_FOUND"
)

// Auth は API-002〜005 を扱う。
type Auth struct {
	svc *service.Auth
	now func() time.Time
}

// NewAuth は Auth ハンドラを生成する。
func NewAuth(svc *service.Auth) *Auth {
	return &Auth{svc: svc, now: time.Now}
}

// GoogleStart は API-002 Google OAuth 開始。認証不要。
func (h *Auth) GoogleStart(w http.ResponseWriter, r *http.Request) {
	authURL, err := h.svc.AuthorizeURL(r.URL.Query().Get("redirect_uri"), h.now())
	if err != nil {
		if errors.Is(err, service.ErrInvalidRedirectURI) {
			httpx.WriteError(w, httpx.BadRequest(
				codeInvalidRedirectURI, "redirect_uri が許可リストにありません"))
			return
		}
		slog.Error("認可 URL の生成に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}

	// authURL は oauth2.Config.AuthCodeURL が組み立てた Google の認可エンドポイント。
	// クエリの redirect_uri は許可リストとの照合と state への保存にしか使われず、
	// この URL には入らないため、リダイレクト先は常に accounts.google.com になる。
	//nolint:gosec // G710: リダイレクト先はライブラリが組み立てる固定のエンドポイント
	http.Redirect(w, r, authURL, http.StatusFound)
}

type callbackResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"`
	User        callbackUser `json:"user"`
}

type callbackUser struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

// GoogleCallback は API-003 Google OAuth コールバック。認証不要。
//
// 既定はフロントへ 302 でリダイレクトし、トークンを URL フラグメントに載せる。
// Accept: application/json の場合は 200 で JSON を返す（curl での動作確認用）。
// エラーも同じく、ブラウザからの遷移ではフロントへ戻す（writeCallbackError）。
func (h *Auth) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	state := query.Get("state")

	// ユーザが同意画面で拒否した場合、Google は error=access_denied を state とともに返す
	if query.Get("error") != "" {
		// state は使い捨てのため、拒否された場合も消費しておく。
		// 検証できれば保存した戻り先へ、できなければ既定の戻り先へ戻す
		redirectURI, _ := h.svc.ConsumeState(state, h.now())
		h.writeCallbackError(w, r, redirectURI, httpx.NewAPIError(
			http.StatusForbidden, codeAccessDenied, "Google の同意画面で拒否されました"))
		return
	}

	code := query.Get("code")
	if code == "" || state == "" {
		h.writeCallbackError(w, r, "", httpx.ValidationError("code と state は必須です"))
		return
	}

	redirectURI, err := h.svc.ConsumeState(state, h.now())
	if err != nil {
		// CSRF の可能性があるため、詳細は返さずログに残す
		slog.Warn("state の検証に失敗しました", "error", err)
		h.writeCallbackError(w, r, "", httpx.BadRequest(
			codeInvalidState, "state が不正です。ログインをやり直してください"))
		return
	}

	result, err := h.svc.Login(r.Context(), code, h.now())
	if err != nil {
		h.writeCallbackError(w, r, redirectURI, loginError(err))
		return
	}

	if !wantsJSON(r) {
		h.redirectWithToken(w, r, redirectURI, result)
		return
	}

	httpx.WriteData(w, http.StatusOK, callbackResponse{
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   result.ExpiresIn,
		User:        callbackUser{ID: result.User.ID, Email: result.User.Email},
	}, map[string]any{"is_new_user": result.IsNewUser})
}

// loginError は Login の失敗を API-003 のエラーに対応づける。
func loginError(err error) *httpx.APIError {
	if errors.Is(err, service.ErrOAuthExchange) {
		slog.Error("認可コードの交換に失敗しました", "error", err)
		return httpx.BadRequest(codeOAuthExchange, "認可コードの交換に失敗しました")
	}
	slog.Error("ログイン処理に失敗しました", "error", err)
	return httpx.Internal(err)
}

// writeCallbackError は API-003 のエラーを返す。
//
// curl での確認（Accept: application/json）には JSON で返す。
// ブラウザからの遷移で JSON を返すと、ユーザが API サーバの画面に取り残されて
// アプリへ戻れないため、フロントへ 302 でリダイレクトしてエラーコードを
// フラグメントで渡す（API 仕様書 API-003、Step 4 で変更）。
//
// redirectURI は API-002 で保存した戻り先。state を検証できず取り出せなかった
// 場合は空で渡し、既定の戻り先へ戻す。
func (h *Auth) writeCallbackError(w http.ResponseWriter, r *http.Request, redirectURI string, apiErr *httpx.APIError) {
	if wantsJSON(r) {
		httpx.WriteError(w, apiErr)
		return
	}

	// 渡すのはエラーコードだけでトークンは含まないが、戻り先は許可リストのものに限る
	if redirectURI == "" || !h.svc.IsAllowedRedirectURI(redirectURI) {
		redirectURI = h.svc.DefaultRedirectURI()
	}

	fragment := url.Values{}
	fragment.Set("error", apiErr.Code)
	h.redirectWithFragment(w, r, redirectURI, fragment)
}

// redirectWithToken はフロントの redirect_uri へトークンを渡してリダイレクトする。
//
// クエリ文字列ではなくフラグメント（#）に載せるのは、
// フラグメントがサーバへ送信されず、Referer ヘッダにも乗らないため。
func (h *Auth) redirectWithToken(
	w http.ResponseWriter, r *http.Request, redirectURI string, result *service.CallbackResult,
) {
	// リダイレクトする直前にもう一度許可リストを確認する。
	// AuthorizeURL でも検証しているが、state ストアの内容を信頼せず、
	// 「トークンを渡す先は必ず許可リストのいずれか」を出口側で保証する。
	if !h.svc.IsAllowedRedirectURI(redirectURI) {
		slog.Error("許可されていない redirect_uri へのリダイレクトを阻止しました",
			"redirect_uri", redirectURI)
		httpx.WriteError(w, httpx.BadRequest(
			codeInvalidRedirectURI, "redirect_uri が許可リストにありません"))
		return
	}

	fragment := url.Values{}
	fragment.Set("access_token", result.AccessToken)
	fragment.Set("token_type", "Bearer")
	fragment.Set("expires_in", strconv.Itoa(result.ExpiresIn))
	h.redirectWithFragment(w, r, redirectURI, fragment)
}

// redirectWithFragment は戻り先へフラグメントを付けて 302 でリダイレクトする。
// 戻り先が許可リストにあることは呼び出し側で確認しておくこと。
func (h *Auth) redirectWithFragment(w http.ResponseWriter, r *http.Request, redirectURI string, fragment url.Values) {
	target, err := url.Parse(redirectURI)
	if err != nil {
		slog.Error("redirect_uri の解析に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}
	target.Fragment = fragment.Encode()

	// 呼び出し側で許可リストとの一致を確認済み。
	// target はその文字列をパースしただけで、ホストもパスも変えていない。
	//nolint:gosec // G710: リダイレクト先は許可リストと完全一致することを確認済み
	http.Redirect(w, r, target.String(), http.StatusFound)
}

type meResponse struct {
	ID                int64   `json:"id"`
	Email             string  `json:"email"`
	DisplayName       *string `json:"display_name"`
	Timezone          string  `json:"timezone"`
	MailAccountsCount int     `json:"mail_accounts_count"`
	NeedsReauth       bool    `json:"needs_reauth"`
}

// Me は API-004 現在のユーザ取得。認証必要。
func (h *Auth) Me(w http.ResponseWriter, r *http.Request, userID int64) {
	me, err := h.svc.FindMe(r.Context(), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// トークンは有効だがユーザが削除されている
			httpx.WriteError(w, httpx.NotFound(codeUserNotFound, "ユーザが存在しません"))
			return
		}
		slog.Error("ユーザ情報の取得に失敗しました", "error", err)
		httpx.WriteError(w, httpx.Internal(err))
		return
	}

	httpx.WriteData(w, http.StatusOK, meResponse{
		ID:                me.User.ID,
		Email:             me.User.Email,
		DisplayName:       me.User.DisplayName,
		Timezone:          me.User.Timezone,
		MailAccountsCount: me.MailAccountsCount,
		NeedsReauth:       me.NeedsReauth,
	}, nil)
}

// Logout は API-005 ログアウト。認証必要。
//
// ステートレスな JWT のため、サーバ側に失効リストを持たない。
// ログアウトの実体はクライアントがトークンを破棄することであり、
// 本エンドポイントはその完了を受け付けて 204 を返すだけ（API 仕様書 API-005）。
func (h *Auth) Logout(w http.ResponseWriter, _ *http.Request, _ int64) {
	httpx.WriteNoContent(w)
}

// wantsJSON は Accept ヘッダが JSON を求めているかを判定する。
func wantsJSON(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json")
}
