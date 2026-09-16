package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// StateTTL は OAuth の state の有効期限（API 仕様書 API-002）。
const StateTTL = 10 * time.Minute

// ErrStateNotFound は state が未発行・期限切れ・使用済みであることを表す。
// 400 INVALID_STATE に対応する。
var ErrStateNotFound = errors.New("state が不正です")

type stateEntry struct {
	redirectURI string
	expiresAt   time.Time
}

// StateStore は OAuth の state を保持する。
//
// CSRF 対策として、認可開始時に発行した state がコールバックで
// 戻ってきたことを確認する。1 度使った state は削除し、再利用を防ぐ。
//
// Step 3 ではプロセス内のメモリに保持する。単一インスタンス前提のため、
// Step 7 以降で複数インスタンスに分散させる場合は Redis 等の
// 共有ストアに置き換える必要がある（再起動でも失われるが、
// 失効しても最大 10 分でユーザがログインし直すだけで済む）。
type StateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
}

// NewStateStore は StateStore を生成する。
func NewStateStore() *StateStore {
	return &StateStore{entries: make(map[string]stateEntry)}
}

// Issue は新しい state を発行し、redirect_uri と紐づけて保持する。
func (s *StateStore) Issue(redirectURI string, now time.Time) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("state の生成に失敗しました: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	s.entries[state] = stateEntry{
		redirectURI: redirectURI,
		expiresAt:   now.Add(StateTTL),
	}

	return state, nil
}

// Consume は state を検証し、紐づく redirect_uri を返して削除する。
// 未発行・期限切れ・使用済みの場合は ErrStateNotFound を返す。
func (s *StateStore) Consume(state string, now time.Time) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[state]
	if !ok {
		return "", ErrStateNotFound
	}

	// 見つかった時点で削除する。期限切れでも再利用させない
	delete(s.entries, state)

	if now.After(entry.expiresAt) {
		return "", ErrStateNotFound
	}
	return entry.redirectURI, nil
}

// pruneLocked は期限切れの state を削除する。呼び出し側がロックを保持していること。
//
// 発行のたびに掃除することで、ログインされないまま溜まった state が
// メモリを占有し続けるのを防ぐ。
func (s *StateStore) pruneLocked(now time.Time) {
	for state, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, state)
		}
	}
}
