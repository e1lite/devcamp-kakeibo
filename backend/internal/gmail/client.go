// Package gmail は Gmail API のうち、メールの取込に使う 3 つだけを呼ぶ。
//
//   - users.getProfile: 連携したメールアドレスを知る（API-030）
//   - users.messages.list: 検索条件に合うメールの ID を取る
//   - users.messages.get（format=raw）: メールの原文を取る
//
// Google 公式のクライアントライブラリは使わず、golang.org/x/oauth2 が作る HTTP クライアントで
// REST を直接呼ぶ。使う API が 3 つだけで、通信を差し替えてテストしやすいため
// （docs/mail-import-mvp.md 判断 9）。
package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// DefaultBaseURL は Gmail API の、認証したユーザ自身を表すエンドポイント。
const DefaultBaseURL = "https://gmail.googleapis.com/gmail/v1/users/me"

// ReadonlyScope はメールの読み取りだけを許すスコープ。
const ReadonlyScope = "https://www.googleapis.com/auth/gmail.readonly"

// ErrReauthRequired は認証情報が失効・取り消された（連携し直す必要がある）ことを表す。
//
// OAuth アプリが「テスト」の状態だと、リフレッシュトークンは 7 日で失効する
// （docs/mail-import-mvp.md 判断 1）。ユーザが Google の設定から連携を取り消した場合も同じ。
var ErrReauthRequired = errors.New("認証情報が失効しています（Gmail）。連携し直してください")

// ErrRateLimited は Gmail API の利用上限に一時的に達したことを表す。時間をおけば回復するため、
// 連携し直す必要は無い（ErrReauthRequired と区別する）。
var ErrRateLimited = errors.New("メールの取得 API の利用上限に一時的に達しました。時間をおいて同期してください")

// 利用上限（429、または理由が上限の 403）のときに、待ってからやり直す回数
const maxRetries = 3

// Client は Gmail API を呼ぶ。
type Client struct {
	http    *http.Client
	baseURL string
	// retryWait は利用上限に達したときに最初に待つ時間。やり直すたびに倍にする
	retryWait time.Duration
}

// NewClient は Client を生成する。httpClient は oauth2.Config.Client などで、
// リクエストにアクセストークンを付けるものを渡す。
func NewClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{http: httpClient, baseURL: strings.TrimRight(baseURL, "/"), retryWait: time.Second}
}

// WithRetryWait は利用上限に達したときに待つ時間を変える（テストで待たないために使う）。
func (c *Client) WithRetryWait(d time.Duration) *Client {
	c.retryWait = d
	return c
}

// Profile は連携したアカウントの情報。
type Profile struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    string `json:"historyId"`
}

// Profile は連携したメールアドレスを取る（users.getProfile）。
func (c *Client) Profile(ctx context.Context) (*Profile, error) {
	var profile Profile
	if err := c.get(ctx, "/profile", nil, &profile); err != nil {
		return nil, err
	}
	if profile.EmailAddress == "" {
		return nil, errors.New("連携したアカウントのプロフィールにメールアドレスがありません")
	}
	return &profile, nil
}

// MessageRef は検索結果の 1 件。本文は含まない。
type MessageRef struct {
	ID       string `json:"id"`
	ThreadID string `json:"threadId"`
}

// ListMessages は検索条件（Gmail の検索窓と同じ書き方）に合うメールを、新しい順に最大 100 件返す。
// 続きがあれば次のページの pageToken を返し、無ければ空文字を返す（users.messages.list）。
func (c *Client) ListMessages(ctx context.Context, query, pageToken string) ([]MessageRef, string, error) {
	params := url.Values{"q": {query}, "maxResults": {"100"}}
	if pageToken != "" {
		params.Set("pageToken", pageToken)
	}

	var res struct {
		Messages      []MessageRef `json:"messages"`
		NextPageToken string       `json:"nextPageToken"`
	}
	if err := c.get(ctx, "/messages", params, &res); err != nil {
		return nil, "", err
	}
	return res.Messages, res.NextPageToken, nil
}

// RawMessage はメール 1 通の原文。
type RawMessage struct {
	ID       string
	ThreadID string
	// ReceivedAt は Gmail が受信した日時（internalDate）。Date ヘッダは送信元が自由に書けるため、こちらを使う
	ReceivedAt time.Time
	Raw        []byte
}

// RawMessage はメールの原文を取る（users.messages.get?format=raw）。
func (c *Client) RawMessage(ctx context.Context, id string) (*RawMessage, error) {
	var res struct {
		ID           string `json:"id"`
		ThreadID     string `json:"threadId"`
		InternalDate string `json:"internalDate"`
		Raw          string `json:"raw"`
	}
	if err := c.get(ctx, "/messages/"+url.PathEscape(id), url.Values{"format": {"raw"}}, &res); err != nil {
		return nil, err
	}

	// raw は base64url。末尾の = は付く場合と付かない場合があるため、外してから読む
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(res.Raw, "="))
	if err != nil {
		return nil, fmt.Errorf("メール %s の原文を読めません: %w", id, err)
	}

	millis, err := strconv.ParseInt(res.InternalDate, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("メール %s の受信日時を読めません: %w", id, err)
	}

	return &RawMessage{
		ID:         res.ID,
		ThreadID:   res.ThreadID,
		ReceivedAt: time.UnixMilli(millis),
		Raw:        raw,
	}, nil
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	endpoint := c.baseURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	wait := c.retryWait
	for attempt := 0; ; attempt++ {
		err := c.getOnce(ctx, endpoint, out)
		if !errors.Is(err, ErrRateLimited) || attempt >= maxRetries {
			return err
		}
		// 利用上限は短い時間の呼び出しの多さで決まるため、少し待てば回復することが多い
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		wait *= 2
	}
}

func (c *Client) getOnce(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("リクエストの生成に失敗しました: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// アクセストークンの更新（リフレッシュトークンの利用）で失敗すると、ここに来る
		if isRevoked(err) {
			return fmt.Errorf("%w（invalid_grant）", ErrReauthRequired)
		}
		return fmt.Errorf("メールの取得 API の呼び出しに失敗しました: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return statusError(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("メールの取得 API の応答を読めません: %w", err)
	}
	return nil
}

// apiError は Gmail API のエラー応答の本文。
type apiError struct {
	Error struct {
		Message string `json:"message"`
		Status  string `json:"status"`
		Errors  []struct {
			Reason string `json:"reason"`
		} `json:"errors"`
	} `json:"error"`
}

// 403 のうち、利用上限に達したことを表す理由。これらは連携し直す必要が無い
var rateLimitReasons = map[string]bool{
	"rateLimitExceeded":     true,
	"userRateLimitExceeded": true,
	"quotaExceeded":         true,
	"dailyLimitExceeded":    true,
	"RESOURCE_EXHAUSTED":    true,
}

// statusError は 200 以外の応答を、呼び出し側が扱えるエラーにする。
//
// Gmail API は「権限が無い」ときも「利用上限に達した」ときも 403 を返す。
// 本文の理由で見分け、利用上限で連携を止めてしまわないようにする。
// 理由はエラーの文言に残し、同期履歴（error_message）から原因を追えるようにする。
func statusError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var parsed apiError
	_ = json.Unmarshal(body, &parsed)

	reason := parsed.Error.Status
	if len(parsed.Error.Errors) > 0 && parsed.Error.Errors[0].Reason != "" {
		reason = parsed.Error.Errors[0].Reason
	}
	detail := fmt.Sprintf("HTTP %d", resp.StatusCode)
	if reason != "" {
		detail += ": " + reason
	}
	if parsed.Error.Message != "" {
		detail += ": " + parsed.Error.Message
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && rateLimitReasons[reason]:
		return fmt.Errorf("%w（%s）", ErrRateLimited, detail)
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		// アクセストークンが無効、またはスコープが外された（gmail.readonly の同意を取り消した）など。
		// 連携し直してもらう
		return fmt.Errorf("%w（%s）", ErrReauthRequired, detail)
	default:
		return fmt.Errorf("メールの取得 API でエラーが起きました（%s）", detail)
	}
}

// isRevoked はリフレッシュトークンが失効・取り消されたことによる失敗かを返す。
// Google はこのとき invalid_grant を返す。
func isRevoked(err error) bool {
	var retrieveErr *oauth2.RetrieveError
	return errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant"
}
