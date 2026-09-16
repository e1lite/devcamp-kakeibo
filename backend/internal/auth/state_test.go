package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
)

func TestStateStore_IssueAndConsume(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	const redirectURI = "http://localhost:5173/auth/callback"

	state, err := store.Issue(redirectURI, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	if state == "" {
		t.Fatal("state が空です")
	}

	got, err := store.Consume(state, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("消費に失敗しました: %v", err)
	}
	if got != redirectURI {
		t.Errorf("redirect_uri: got %q, want %q", got, redirectURI)
	}
}

func TestStateStore_Consume_一度使ったstateは再利用できない(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Now()

	state, err := store.Issue("http://localhost:5173/auth/callback", now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	if _, err := store.Consume(state, now); err != nil {
		t.Fatalf("1 回目の消費に失敗しました: %v", err)
	}

	// 2 回目はリプレイ攻撃とみなして拒否する
	if _, err := store.Consume(state, now); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("2 回目: got %v, want ErrStateNotFound", err)
	}
}

func TestStateStore_Consume_期限切れ(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Now()

	state, err := store.Issue("http://localhost:5173/auth/callback", now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	expired := now.Add(auth.StateTTL + time.Second)
	if _, err := store.Consume(state, expired); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("got %v, want ErrStateNotFound", err)
	}
}

func TestStateStore_Consume_未発行のstate(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()

	if _, err := store.Consume("never-issued", time.Now()); !errors.Is(err, auth.ErrStateNotFound) {
		t.Errorf("got %v, want ErrStateNotFound", err)
	}
}

func TestStateStore_Issue_毎回異なるstateを返す(t *testing.T) {
	t.Parallel()

	store := auth.NewStateStore()
	now := time.Now()
	seen := make(map[string]bool, 100)

	for range 100 {
		state, err := store.Issue("http://localhost:5173/auth/callback", now)
		if err != nil {
			t.Fatalf("発行に失敗しました: %v", err)
		}
		if seen[state] {
			t.Fatalf("state が重複しました: %s", state)
		}
		seen[state] = true
	}
}
