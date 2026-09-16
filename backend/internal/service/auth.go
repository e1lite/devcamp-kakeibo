// Package service は業務ロジックを担う。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"golang.org/x/oauth2"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/model"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

// 認証処理のエラー。ハンドラがこれを見てエラーコードを出し分ける。
var (
	// ErrInvalidRedirectURI は redirect_uri が許可リストにない。
	ErrInvalidRedirectURI = errors.New("redirect_uri が許可されていません")
	// ErrInvalidState は state が未発行・期限切れ・不一致（CSRF の疑い）。
	ErrInvalidState = errors.New("state が不正です")
	// ErrOAuthExchange は認可コードとトークンの交換に失敗した。
	ErrOAuthExchange = errors.New("認可コードの交換に失敗しました")
)

// googleUserInfo は Google の userinfo エンドポイントのレスポンス。
type googleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

const googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

// Auth は Google OAuth によるログインを担う（API-002 / API-003）。
type Auth struct {
	oauth               *oauth2.Config
	states              *auth.StateStore
	tokens              *auth.TokenIssuer
	users               *repository.User
	allowedRedirectURIs []string
	defaultRedirectURI  string
	httpClient          *http.Client
}

// AuthConfig は Auth サービスの依存と設定。
type AuthConfig struct {
	OAuth               *oauth2.Config
	States              *auth.StateStore
	Tokens              *auth.TokenIssuer
	Users               *repository.User
	AllowedRedirectURIs []string
	DefaultRedirectURI  string
}

// NewAuth は Auth サービスを生成する。
func NewAuth(cfg AuthConfig) *Auth {
	return &Auth{
		oauth:               cfg.OAuth,
		states:              cfg.States,
		tokens:              cfg.Tokens,
		users:               cfg.Users,
		allowedRedirectURIs: cfg.AllowedRedirectURIs,
		defaultRedirectURI:  cfg.DefaultRedirectURI,
		httpClient:          &http.Client{Timeout: 10 * time.Second},
	}
}

// AuthorizeURL は Google の認可画面の URL を組み立てる（API-002）。
//
// redirectURI が空なら既定値を使う。許可リストにない値は拒否する
// （オープンリダイレクタにして、トークンを任意のサイトへ送らせないため）。
func (s *Auth) AuthorizeURL(redirectURI string, now time.Time) (string, error) {
	if redirectURI == "" {
		redirectURI = s.defaultRedirectURI
	}
	if !slices.Contains(s.allowedRedirectURIs, redirectURI) {
		return "", ErrInvalidRedirectURI
	}

	state, err := s.states.Issue(redirectURI, now)
	if err != nil {
		return "", err
	}

	return s.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

// IsAllowedRedirectURI は戻り先が許可リストにあるかを返す。
//
// AuthorizeURL でも検証しているが、実際にリダイレクトする直前に
// もう一度確認するためにハンドラから呼ぶ。state ストアの内容を信頼せず、
// 「リダイレクト先は必ず許可リストのいずれか」を出口側で保証する。
func (s *Auth) IsAllowedRedirectURI(uri string) bool {
	return slices.Contains(s.allowedRedirectURIs, uri)
}

// CallbackResult はログイン完了後にハンドラへ返す情報。
type CallbackResult struct {
	AccessToken string
	ExpiresIn   int
	User        *model.User
	IsNewUser   bool
	RedirectURI string
}

// HandleCallback は認可コードを検証してログインを完了させる（API-003）。
//
// google_sub で既存ユーザを探し、いなければ作成する。
// メールアドレスは Google 側で変わりうるため、ログインのたびに同期する。
func (s *Auth) HandleCallback(ctx context.Context, code, state string, now time.Time) (*CallbackResult, error) {
	redirectURI, err := s.states.Consume(state, now)
	if err != nil {
		return nil, ErrInvalidState
	}

	token, err := s.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrOAuthExchange, err)
	}

	info, err := s.fetchUserInfo(ctx, token)
	if err != nil {
		return nil, err
	}

	user, isNew, err := s.upsertUser(ctx, info)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.tokens.Issue(user.ID, now)
	if err != nil {
		return nil, err
	}

	return &CallbackResult{
		AccessToken: accessToken,
		ExpiresIn:   int(auth.TokenTTL.Seconds()),
		User:        user,
		IsNewUser:   isNew,
		RedirectURI: redirectURI,
	}, nil
}

func (s *Auth) fetchUserInfo(ctx context.Context, token *oauth2.Token) (*googleUserInfo, error) {
	client := s.oauth.Client(ctx, token)
	client.Timeout = s.httpClient.Timeout

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("ユーザ情報リクエストの生成に失敗しました: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: ユーザ情報の取得に失敗しました: %w", ErrOAuthExchange, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: ユーザ情報の取得が %d を返しました", ErrOAuthExchange, resp.StatusCode)
	}

	var info googleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("%w: ユーザ情報の解析に失敗しました: %w", ErrOAuthExchange, err)
	}
	if info.Sub == "" || info.Email == "" {
		return nil, fmt.Errorf("%w: ユーザ情報に sub または email が含まれていません", ErrOAuthExchange)
	}

	return &info, nil
}

func (s *Auth) upsertUser(ctx context.Context, info *googleUserInfo) (*model.User, bool, error) {
	existing, err := s.users.FindByGoogleSub(ctx, info.Sub)
	switch {
	case err == nil:
		// メールアドレス・表示名は Google 側で変わりうるため毎回同期する
		if existing.Email != info.Email || displayNameChanged(existing.DisplayName, info.Name) {
			existing.Email = info.Email
			existing.DisplayName = optionalString(info.Name)
			if uerr := s.users.UpdateProfile(ctx, existing); uerr != nil {
				return nil, false, uerr
			}
		}
		return existing, false, nil

	case errors.Is(err, repository.ErrNotFound):
		user := &model.User{
			GoogleSub:   info.Sub,
			Email:       info.Email,
			DisplayName: optionalString(info.Name),
			Timezone:    "Asia/Tokyo",
		}
		if cerr := s.users.CreateWithInitialData(ctx, user); cerr != nil {
			return nil, false, cerr
		}
		return user, true, nil

	default:
		return nil, false, err
	}
}

// Me はユーザ情報と連携メールアカウントの集計を返す（API-004）。
type Me struct {
	User              *model.User
	MailAccountsCount int
	NeedsReauth       bool
}

// FindMe は認証済みユーザの情報を取得する。
func (s *Auth) FindMe(ctx context.Context, userID int64) (*Me, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	summary, err := s.users.SummarizeMailAccounts(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &Me{
		User:              user,
		MailAccountsCount: summary.Count,
		NeedsReauth:       summary.NeedsReauth,
	}, nil
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func displayNameChanged(current *string, incoming string) bool {
	if current == nil {
		return incoming != ""
	}
	return *current != incoming
}
