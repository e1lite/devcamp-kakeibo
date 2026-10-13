package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// StateTTL は OAuth の state の有効期限（API 仕様書 API-002 / API-029）。
const StateTTL = 10 * time.Minute

// ErrStateNotFound は state が未発行・期限切れ・使用済み・用途違いであることを表す。
// 400 INVALID_STATE に対応する。
var ErrStateNotFound = errors.New("state が不正です")

// StatePurpose は state を発行した目的。ログインとメール連携で同じ Google の OAuth を使うため、
// ログイン用の state をメール連携のコールバックに使い回させない（API 仕様書 API-029）。
type StatePurpose string

// StatePurpose の値。
const (
	// PurposeLogin はログイン（API-002 → API-003）。
	PurposeLogin StatePurpose = "login"
	// PurposeMailLink はメール連携（API-029 → API-030）。
	PurposeMailLink StatePurpose = "mail_link"
)

// StateData は state に紐づけて保持する値。
type StateData struct {
	Purpose StatePurpose
	// RedirectURI は OAuth の完了後に戻すフロントの URL。
	RedirectURI string
	// UserID はメール連携を始めたユーザ。コールバックには Authorization ヘッダが付かないため、
	// state からユーザを特定する（API-030）。ログインでは 0。
	UserID int64
}

type stateEntry struct {
	data      StateData
	expiresAt time.Time
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

// Issue は新しい state を発行し、data と紐づけて保持する。
func (s *StateStore) Issue(data StateData, now time.Time) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("state の生成に失敗しました: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(buf)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(now)
	s.entries[state] = stateEntry{
		data:      data,
		expiresAt: now.Add(StateTTL),
	}

	return state, nil
}

// Consume は state を検証し、紐づく値を返して削除する。
// 未発行・期限切れ・使用済み、または発行した目的が purpose と違う場合は ErrStateNotFound を返す。
func (s *StateStore) Consume(state string, purpose StatePurpose, now time.Time) (StateData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[state]
	if !ok {
		return StateData{}, ErrStateNotFound
	}

	// 見つかった時点で削除する。期限切れ・用途違いでも再利用させない
	delete(s.entries, state)

	if now.After(entry.expiresAt) || entry.data.Purpose != purpose {
		return StateData{}, ErrStateNotFound
	}
	return entry.data, nil
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
