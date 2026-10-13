package handler_test

import (
	"context"
	"encoding/json"
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
	defaultRedirectURI = "http://localhost:5173/auth/callback"
	// savedRedirectURI は API-002 で保存される戻り先。既定値と区別できるよう別の URL にする
	savedRedirectURI = "http://localhost:5173/other/callback"
)

// roundTripFunc は関数を http.RoundTripper として使う。
// Google のトークンエンドポイントを、ネットワークに出ずに差し替えるために使う。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newAuthHandler(t *testing.T) (*handler.Auth, *auth.StateStore) {
	t.Helper()

	states := auth.NewStateStore()
	svc := service.NewAuth(service.AuthConfig{
		OAuth: &oauth2.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://accounts.example.com/auth",
				TokenURL: "https://accounts.example.com/token",
			},
		},
		States:              states,
		AllowedRedirectURIs: []string{defaultRedirectURI, savedRedirectURI},
		DefaultRedirectURI:  defaultRedirectURI,
	})
	return handler.NewAuth(svc), states
}

func issueState(t *testing.T, states *auth.StateStore) string {
	t.Helper()

	state, err := states.Issue(auth.StateData{Purpose: auth.PurposeLogin, RedirectURI: savedRedirectURI}, time.Now())
	if err != nil {
		t.Fatalf("state の発行に失敗しました: %v", err)
	}
	return state
}

// callback は API-003 を呼ぶ。accept が空ならブラウザからの遷移として扱われる。
func callback(ctx context.Context, h *handler.Auth, query url.Values, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(ctx, http.MethodGet,
		"/api/v1/auth/google/callback?"+query.Encode(), nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}

	rec := httptest.NewRecorder()
	h.GoogleCallback(rec, req)
	return rec
}

// assertErrorRedirect はフロントへ 302 で戻され、フラグメントにエラーコードだけが
// 載っていることを確認する。
func assertErrorRedirect(t *testing.T, rec *httptest.ResponseRecorder, wantBase, wantCode string) {
	t.Helper()

	if rec.Code != http.StatusFound {
		t.Fatalf("ステータスコード: got %d, want 302（body: %s）", rec.Code, rec.Body.String())
	}

	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("Location を解析できません: %v", err)
	}

	fragment := location.Fragment
	location.Fragment = ""
	if location.String() != wantBase {
		t.Errorf("戻り先: got %q, want %q", location.String(), wantBase)
	}

	values, err := url.ParseQuery(fragment)
	if err != nil {
		t.Fatalf("フラグメントを解析できません: %v", err)
	}
	if got := values.Get("error"); got != wantCode {
		t.Errorf("error: got %q, want %q", got, wantCode)
	}
	// エラー時にトークンなど余計な値を渡していないこと
	if len(values) != 1 {
		t.Errorf("フラグメントに error 以外が含まれています: %q", fragment)
	}
}

// TestAuth_GoogleCallback_ブラウザではエラーをフロントへ戻す は API-003 の
// ブラウザ遷移時のエラーの返し方（Step 4 で変更）を確認する。
//
// JSON を返すとユーザが API サーバの画面に取り残されるため、フロントへリダイレクトする。
// 戻り先は、state を検証できたら API-002 で保存したもの、できなければ既定値。
func TestAuth_GoogleCallback_ブラウザではエラーをフロントへ戻す(t *testing.T) {
	t.Parallel()

	t.Run("同意画面で拒否し state を検証できれば、保存した戻り先へ ACCESS_DENIED を渡す", func(t *testing.T) {
		t.Parallel()
		h, states := newAuthHandler(t)

		rec := callback(t.Context(), h, url.Values{
			"error": {"access_denied"},
			"state": {issueState(t, states)},
		}, "")

		assertErrorRedirect(t, rec, savedRedirectURI, "ACCESS_DENIED")
	})

	t.Run("同意画面で拒否し state を検証できなければ、既定の戻り先へ ACCESS_DENIED を渡す", func(t *testing.T) {
		t.Parallel()
		h, _ := newAuthHandler(t)

		rec := callback(t.Context(), h, url.Values{
			"error": {"access_denied"},
			"state": {"never-issued"},
		}, "")

		assertErrorRedirect(t, rec, defaultRedirectURI, "ACCESS_DENIED")
	})

	t.Run("state が無ければ、既定の戻り先へ VALIDATION_ERROR を渡す", func(t *testing.T) {
		t.Parallel()
		h, _ := newAuthHandler(t)

		rec := callback(t.Context(), h, url.Values{"code": {"dummy"}}, "")

		assertErrorRedirect(t, rec, defaultRedirectURI, "VALIDATION_ERROR")
	})

	t.Run("未発行の state は、既定の戻り先へ INVALID_STATE を渡す", func(t *testing.T) {
		t.Parallel()
		h, _ := newAuthHandler(t)

		rec := callback(t.Context(), h, url.Values{
			"code":  {"dummy"},
			"state": {"never-issued"},
		}, "")

		assertErrorRedirect(t, rec, defaultRedirectURI, "INVALID_STATE")
	})

	t.Run("認可コードの交換に失敗したら、保存した戻り先へ OAUTH_EXCHANGE_FAILED を渡す", func(t *testing.T) {
		t.Parallel()
		h, states := newAuthHandler(t)

		// Google のトークンエンドポイントが認可コードを拒否した状況を作る
		google := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant"}`)),
				Request:    r,
			}, nil
		})}
		ctx := context.WithValue(t.Context(), oauth2.HTTPClient, google)

		rec := callback(ctx, h, url.Values{
			"code":  {"expired-code"},
			"state": {issueState(t, states)},
		}, "")

		assertErrorRedirect(t, rec, savedRedirectURI, "OAUTH_EXCHANGE_FAILED")
	})

	t.Run("拒否で消費した state は、もう一度使えない", func(t *testing.T) {
		t.Parallel()
		h, states := newAuthHandler(t)
		state := issueState(t, states)

		callback(t.Context(), h, url.Values{"error": {"access_denied"}, "state": {state}}, "")
		rec := callback(t.Context(), h, url.Values{"code": {"dummy"}, "state": {state}}, "")

		assertErrorRedirect(t, rec, defaultRedirectURI, "INVALID_STATE")
	})
}

// TestAuth_GoogleCallback_JSONを求められたらJSONで返す は curl での動作確認の経路が
// 変わっていないことを確認する。
func TestAuth_GoogleCallback_JSONを求められたらJSONで返す(t *testing.T) {
	t.Parallel()
	h, _ := newAuthHandler(t)

	rec := callback(t.Context(), h, url.Values{
		"code":  {"dummy"},
		"state": {"never-issued"},
	}, "application/json")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ステータスコード: got %d, want 400", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "" {
		t.Errorf("JSON の経路でリダイレクトしています: %q", location)
	}

	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("レスポンスを解析できません: %v", err)
	}
	if body.Error.Code != "INVALID_STATE" {
		t.Errorf("error.code: got %q, want INVALID_STATE", body.Error.Code)
	}
}
