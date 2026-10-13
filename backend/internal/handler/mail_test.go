package handler_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/handler"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/service"
)

const (
	defaultMailRedirectURI = "http://localhost:5173/settings/mail-accounts"
	// savedMailRedirectURI は API-029 で保存される戻り先。既定値と区別できるよう別の URL にする
	savedMailRedirectURI = "http://localhost:5173/settings/mail-accounts?from=menu"
)

func newMailHandler(t *testing.T) (*handler.Mail, *auth.StateStore) {
	t.Helper()

	cipher, err := auth.NewCredentialCipher(bytes.Repeat([]byte{1}, auth.CredentialKeySize))
	if err != nil {
		t.Fatalf("暗号の初期化に失敗しました: %v", err)
	}

	states := auth.NewStateStore()
	svc := service.NewMail(service.MailConfig{
		OAuth: &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.example.com/auth",
				TokenURL: "https://accounts.example.com/token",
			},
		},
		States:              states,
		Cipher:              cipher,
		AllowedRedirectURIs: []string{defaultMailRedirectURI, savedMailRedirectURI},
		DefaultRedirectURI:  defaultMailRedirectURI,
		GmailBaseURL:        "https://gmail.example.com/v1/users/me",
	})
	return handler.NewMail(svc), states
}

func issueMailState(t *testing.T, states *auth.StateStore) string {
	t.Helper()

	state, err := states.Issue(auth.StateData{
		Purpose:     auth.PurposeMailLink,
		RedirectURI: savedMailRedirectURI,
		UserID:      7,
	}, time.Now())
	if err != nil {
		t.Fatalf("state の発行に失敗しました: %v", err)
	}
	return state
}

func mailCallback(ctx context.Context, h *handler.Mail, query url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/api/v1/mail-accounts/google/callback?"+query.Encode(), nil)
	rec := httptest.NewRecorder()
	h.GoogleCallback(rec, req)
	return rec
}

// assertMailRedirect はメール連携画面へ 302 で戻され、フラグメントに結果が載っていることを確認する。
func assertMailRedirect(t *testing.T, rec *httptest.ResponseRecorder, wantBase string, want url.Values) {
	t.Helper()

	if rec.Code != http.StatusFound {
		t.Fatalf("ステータスコード: got %d, want 302（body: %s）", rec.Code, rec.Body.String())
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location を解析できません: %v", err)
	}

	fragment, err := url.ParseQuery(location.Fragment)
	if err != nil {
		t.Fatalf("フラグメントを解析できません: %v", err)
	}
	location.Fragment = ""
	if location.String() != wantBase {
		t.Errorf("戻り先: got %q, want %q", location.String(), wantBase)
	}
	// 結果だけを渡し、Google の認証情報などは一切載せない
	if fragment.Encode() != want.Encode() {
		t.Errorf("フラグメント: got %q, want %q", fragment.Encode(), want.Encode())
	}
}

func mailError(code string) url.Values {
	return url.Values{"mail_link": {"error"}, "code": {code}}
}

// googleToken は Google のトークンエンドポイントを差し替え、決まった応答を返す。
func googleToken(status int, body string) context.Context {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: status,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})}
	return context.WithValue(context.Background(), oauth2.HTTPClient, client)
}

func TestMail_GoogleCallback_エラーはメール連携画面へ戻す(t *testing.T) {
	t.Parallel()

	t.Run("ログイン用の state は使えず、既定の戻り先へ INVALID_STATE を渡す", func(t *testing.T) {
		t.Parallel()
		h, states := newMailHandler(t)

		// ログインの途中の state で、他人の Gmail を自分の連携として登録させない（API-029）
		loginState, err := states.Issue(auth.StateData{
			Purpose:     auth.PurposeLogin,
			RedirectURI: savedMailRedirectURI,
		}, time.Now())
		if err != nil {
			t.Fatalf("state の発行に失敗しました: %v", err)
		}

		rec := mailCallback(t.Context(), h, url.Values{"code": {"dummy"}, "state": {loginState}})
		assertMailRedirect(t, rec, defaultMailRedirectURI, mailError("INVALID_STATE"))
	})

	t.Run("同意画面で拒否したら、保存した戻り先へ ACCESS_DENIED を渡す", func(t *testing.T) {
		t.Parallel()
		h, states := newMailHandler(t)

		rec := mailCallback(t.Context(), h, url.Values{"error": {"access_denied"}, "state": {issueMailState(t, states)}})
		assertMailRedirect(t, rec, savedMailRedirectURI, mailError("ACCESS_DENIED"))
	})

	t.Run("state が無ければ、既定の戻り先へ VALIDATION_ERROR を渡す", func(t *testing.T) {
		t.Parallel()
		h, _ := newMailHandler(t)

		rec := mailCallback(t.Context(), h, url.Values{"code": {"dummy"}})
		assertMailRedirect(t, rec, defaultMailRedirectURI, mailError("VALIDATION_ERROR"))
	})

	t.Run("認可コードの交換に失敗したら OAUTH_EXCHANGE_FAILED", func(t *testing.T) {
		t.Parallel()
		h, states := newMailHandler(t)

		ctx := googleToken(http.StatusBadRequest, `{"error":"invalid_grant"}`)
		rec := mailCallback(ctx, h, url.Values{"code": {"expired"}, "state": {issueMailState(t, states)}})
		assertMailRedirect(t, rec, savedMailRedirectURI, mailError("OAUTH_EXCHANGE_FAILED"))
	})

	t.Run("同意画面で Gmail の読み取りのチェックを外したら INSUFFICIENT_SCOPE", func(t *testing.T) {
		t.Parallel()
		h, states := newMailHandler(t)

		// Google はスコープ単位で拒否でき、許可されたスコープだけを scope に返す
		ctx := googleToken(http.StatusOK,
			`{"access_token":"at","token_type":"Bearer","expires_in":3600,"refresh_token":"rt","scope":"openid email"}`)
		rec := mailCallback(ctx, h, url.Values{"code": {"ok"}, "state": {issueMailState(t, states)}})
		assertMailRedirect(t, rec, savedMailRedirectURI, mailError("INSUFFICIENT_SCOPE"))
	})
}
