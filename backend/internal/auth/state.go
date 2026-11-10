package auth

import (
	"context"
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

// StateStore は OAuth の state の保管場所。
//
// CSRF 対策として、認可開始時に発行した state がコールバックで
// 戻ってきたことを確認する。1 度使った state は削除し、再利用を防ぐ。
//
// 本番では API サーバを複数台で動かし、認可の開始とコールバックが別の台に振り分けられうるため、
// 台をまたいで共有できる DB に置く（repository.OAuthState、docs/step7-infra.md 6 章 #1）。
// MemoryStateStore はプロセス内に置く実装で、単体テストに使う。
type StateStore interface {
	// Issue は新しい state を発行し、data と紐づけて保持する。
	Issue(ctx context.Context, data StateData, now time.Time) (string, error)
	// Consume は state を検証し、紐づく値を返して削除する。
	// 未発行・期限切れ・使用済み、または発行した目的が purpose と違う場合は ErrStateNotFound を返す。
	Consume(ctx context.Context, state string, purpose StatePurpose, now time.Time) (StateData, error)
}

// GenerateState は推測できない state の文字列を作る。
func GenerateState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("state の生成に失敗しました: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// CheckConsumed は、保管場所から取り出した（＝すでに削除した）state が使えるかを確かめる。
// 期限切れ、または発行した目的が purpose と違う場合は ErrStateNotFound を返す。
//
// 期限切れ・用途違いでも、取り出した時点で削除済みのため再利用はできない。
func CheckConsumed(data StateData, expiresAt time.Time, purpose StatePurpose, now time.Time) (StateData, error) {
	if now.After(expiresAt) || data.Purpose != purpose {
		return StateData{}, ErrStateNotFound
	}
	return data, nil
}

type stateEntry struct {
	data      StateData
	expiresAt time.Time
}

// MemoryStateStore は state をプロセス内のメモリに保持する StateStore。
//
// 単一プロセスの中でしか共有できないため、本番（複数台）では使わない。単体テスト用。
type MemoryStateStore struct {
	mu      sync.Mutex
	entries map[string]stateEntry
}

// NewMemoryStateStore は MemoryStateStore を生成する。
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{entries: make(map[string]stateEntry)}
}

// Issue は新しい state を発行し、data と紐づけて保持する。
func (s *MemoryStateStore) Issue(_ context.Context, data StateData, now time.Time) (string, error) {
	state, err := GenerateState()
	if err != nil {
		return "", err
	}

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
func (s *MemoryStateStore) Consume(_ context.Context, state string, purpose StatePurpose, now time.Time) (StateData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[state]
	if !ok {
		return StateData{}, ErrStateNotFound
	}

	// 見つかった時点で削除する。期限切れ・用途違いでも再利用させない
	delete(s.entries, state)

	return CheckConsumed(entry.data, entry.expiresAt, purpose, now)
}

// pruneLocked は期限切れの state を削除する。呼び出し側がロックを保持していること。
//
// 発行のたびに掃除することで、ログインされないまま溜まった state が
// メモリを占有し続けるのを防ぐ。
func (s *MemoryStateStore) pruneLocked(now time.Time) {
	for state, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, state)
		}
	}
}
