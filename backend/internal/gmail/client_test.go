package gmail_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/gmail"
)

// Gmail API への通信は、ネットワークに出ずに関数で差し替える。

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func respond(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func newClient(fn roundTripFunc) *gmail.Client {
	return gmail.NewClient(&http.Client{Transport: fn}, "https://gmail.example/v1/users/me")
}

func TestClient_ListMessages_検索条件とページ送り(t *testing.T) {
	t.Parallel()

	var gotQuery, gotPageToken string
	client := newClient(func(r *http.Request) (*http.Response, error) {
		gotQuery = r.URL.Query().Get("q")
		gotPageToken = r.URL.Query().Get("pageToken")
		return respond(http.StatusOK, `{"messages":[{"id":"m1","threadId":"t1"},{"id":"m2","threadId":"t2"}],"nextPageToken":"next"}`), nil
	})

	refs, next, err := client.ListMessages(t.Context(), "subject:ご利用 after:1780000000", "page2")
	if err != nil {
		t.Fatalf("取得に失敗しました: %v", err)
	}
	if gotQuery != "subject:ご利用 after:1780000000" || gotPageToken != "page2" {
		t.Errorf("送った条件: q=%q pageToken=%q", gotQuery, gotPageToken)
	}
	if len(refs) != 2 || refs[0].ID != "m1" || next != "next" {
		t.Errorf("結果: refs=%+v next=%q", refs, next)
	}
}

func TestClient_ListMessages_該当なし(t *testing.T) {
	t.Parallel()

	// 該当が無いとき、Gmail は messages を省いた応答を返す
	client := newClient(func(*http.Request) (*http.Response, error) {
		return respond(http.StatusOK, `{"resultSizeEstimate":0}`), nil
	})

	refs, next, err := client.ListMessages(t.Context(), "subject:none", "")
	if err != nil || len(refs) != 0 || next != "" {
		t.Errorf("got refs=%v next=%q err=%v", refs, next, err)
	}
}

func TestClient_RawMessage_原文と受信日時を読む(t *testing.T) {
	t.Parallel()

	original := "From: a@example.com\r\nSubject: test\r\n\r\nbody?"
	// Gmail は base64url で返す。末尾の = が付かない場合もある
	encoded := strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(original)), "=")

	client := newClient(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("format") != "raw" {
			t.Errorf("format=raw で取得していません: %s", r.URL)
		}
		return respond(http.StatusOK, `{"id":"m1","threadId":"t1","internalDate":"1791865800000","raw":"`+encoded+`"}`), nil
	})

	msg, err := client.RawMessage(t.Context(), "m1")
	if err != nil {
		t.Fatalf("取得に失敗しました: %v", err)
	}
	if string(msg.Raw) != original {
		t.Errorf("原文: got %q", msg.Raw)
	}
	if want := time.UnixMilli(1791865800000); !msg.ReceivedAt.Equal(want) {
		t.Errorf("受信日時: got %v, want %v", msg.ReceivedAt, want)
	}
}

func TestClient_失効した認証情報は再認証が必要(t *testing.T) {
	t.Parallel()

	cases := map[string]roundTripFunc{
		// リフレッシュトークンが失効（テスト状態の 7 日）・取り消された
		"invalid_grant": func(*http.Request) (*http.Response, error) {
			return nil, &oauth2.RetrieveError{ErrorCode: "invalid_grant"}
		},
		"401": func(*http.Request) (*http.Response, error) {
			return respond(http.StatusUnauthorized, `{}`), nil
		},
		// gmail.readonly の同意を取り消した
		"403": func(*http.Request) (*http.Response, error) {
			return respond(http.StatusForbidden, `{}`), nil
		},
	}

	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := newClient(fn).Profile(t.Context()); !errors.Is(err, gmail.ErrReauthRequired) {
				t.Errorf("got %v, want ErrReauthRequired", err)
			}
		})
	}
}

// TestClient_利用上限は再認証にしない は、403 を一律に「連携し直す」にしないことを確かめる。
//
// Gmail API は利用上限に達したときも 403 を返す。これを失効と扱うと、連携した直後の
// 過去メールの取込（短い時間に多く呼ぶ）で、連携が「再認証が必要」になってしまう。
func TestClient_利用上限は再認証にしない(t *testing.T) {
	t.Parallel()

	rateLimited := func(status int, reason string) roundTripFunc {
		return func(*http.Request) (*http.Response, error) {
			return respond(status, fmt.Sprintf(
				`{"error":{"code":%d,"message":"slow down","errors":[{"reason":%q}]}}`, status, reason)), nil
		}
	}

	for name, fn := range map[string]roundTripFunc{
		"403 rateLimitExceeded":     rateLimited(http.StatusForbidden, "rateLimitExceeded"),
		"403 userRateLimitExceeded": rateLimited(http.StatusForbidden, "userRateLimitExceeded"),
		"429":                       rateLimited(http.StatusTooManyRequests, "rateLimitExceeded"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			counted := roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return fn(r)
			})
			_, err := newClient(counted).WithRetryWait(0).Profile(t.Context())
			if !errors.Is(err, gmail.ErrRateLimited) || errors.Is(err, gmail.ErrReauthRequired) {
				t.Errorf("got %v, want ErrRateLimited（再認証ではない）", err)
			}
			// 最初の 1 回と、やり直し 3 回
			if calls != 4 {
				t.Errorf("呼び出し回数: got %d, want 4", calls)
			}
		})
	}
}

func TestClient_利用上限はやり直して回復する(t *testing.T) {
	t.Parallel()

	calls := 0
	client := newClient(func(*http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return respond(http.StatusTooManyRequests, `{}`), nil
		}
		return respond(http.StatusOK, `{"emailAddress":"user@gmail.example.com"}`), nil
	}).WithRetryWait(0)

	profile, err := client.Profile(t.Context())
	if err != nil || profile.EmailAddress != "user@gmail.example.com" {
		t.Errorf("got profile=%+v err=%v", profile, err)
	}
}

func TestClient_権限が無い403は再認証が必要で理由を残す(t *testing.T) {
	t.Parallel()

	client := newClient(func(*http.Request) (*http.Response, error) {
		return respond(http.StatusForbidden,
			`{"error":{"code":403,"message":"Request had insufficient authentication scopes.","errors":[{"reason":"insufficientPermissions"}]}}`), nil
	})

	_, err := client.Profile(t.Context())
	if !errors.Is(err, gmail.ErrReauthRequired) {
		t.Fatalf("got %v, want ErrReauthRequired", err)
	}
	// 同期履歴の error_message から原因を追えるよう、Google の理由を文言に残す
	if !strings.Contains(err.Error(), "insufficientPermissions") {
		t.Errorf("理由が残っていません: %v", err)
	}
}

func TestClient_それ以外のエラーは再認証にしない(t *testing.T) {
	t.Parallel()

	client := newClient(func(*http.Request) (*http.Response, error) {
		return respond(http.StatusInternalServerError, `{"error":"backend"}`), nil
	})

	_, err := client.Profile(t.Context())
	if err == nil || errors.Is(err, gmail.ErrReauthRequired) {
		t.Errorf("一時的な障害で連携を止めてしまいます: %v", err)
	}
}
