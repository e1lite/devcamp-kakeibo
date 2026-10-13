package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
)

var loginState = auth.StateData{Purpose: auth.PurposeLogin, RedirectURI: "http://localhost:5173/auth/callback"}

func TestStateStore_IssueAndConsume(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	data := auth.StateData{
		Purpose:     auth.PurposeMailLink,
		RedirectURI: "http://localhost:5173/settings/mail-accounts",
		UserID:      42,
	}

	state, err := store.Issue(data, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	if state == "" {
		t.Fatal("state が空です")
	}

	got, err := store.Consume(state, auth.PurposeMailLink, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("消費に失敗しました: %v", err)
	}
	if got != data {
		t.Errorf("紐づけた値: got %+v, want %+v", got, data)
	}
}

func TestStateStore_Consume_一度使ったstateは再利用できない(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Now()

	state, err := store.Issue(loginState, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	if _, err := store.Consume(state, auth.PurposeLogin, now); err != nil {
		t.Fatalf("1 回目の消費に失敗しました: %v", err)
	}

	// 2 回目はリプレイ攻撃とみなして拒否する
	if _, err := store.Consume(state, auth.PurposeLogin, now); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("2 回目: got %v, want ErrStateNotFound", err)
	}
}

// TestStateStore_Consume_用途の違うstateは使えない は、ログイン用の state を
// メール連携のコールバックに使い回せないことを確認する（API 仕様書 API-029）。
// 使い回せると、ログインの途中の state で、他人の Gmail を自分の連携として登録させる余地が生まれる。
func TestStateStore_Consume_用途の違うstateは使えない(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Now()

	state, err := store.Issue(loginState, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	if _, err := store.Consume(state, auth.PurposeMailLink, now); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("用途違い: got %v, want ErrStateNotFound", err)
	}
	// 用途違いで失敗した state も消える。正しい用途で使い直すこともできない
	if _, err := store.Consume(state, auth.PurposeLogin, now); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("失敗後の再利用: got %v, want ErrStateNotFound", err)
	}
}

func TestStateStore_Consume_期限切れ(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Now()

	state, err := store.Issue(loginState, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	expired := now.Add(auth.StateTTL + time.Second)
	if _, err := store.Consume(state, auth.PurposeLogin, expired); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("got %v, want ErrStateNotFound", err)
	}
}

func TestStateStore_Consume_未発行のstate(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()

	if _, err := store.Consume("never-issued", auth.PurposeLogin, time.Now()); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("got %v, want ErrStateNotFound", err)
	}
}

func TestStateStore_Issue_毎回異なるstateを返す(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Now()
	seen := make(map[string]bool, 100)

	for range 100 {
		state, err := store.Issue(loginState, now)
		if err != nil {
			t.Fatalf("発行に失敗しました: %v", err)
		}
		if seen[state] {
			t.Fatalf("state が重複しました: %s", state)
		}
		seen[state] = true
	}
}
