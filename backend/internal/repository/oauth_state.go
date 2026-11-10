package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
)

// oauthStateRow は oauth_states テーブルの 1 行。
type oauthStateRow struct {
	State       string `gorm:"primaryKey"`
	Purpose     string
	RedirectURI string
	UserID      *int64
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// TableName は GORM に対応するテーブル名を伝える。
func (oauthStateRow) TableName() string { return "oauth_states" }

// OAuthState は oauth_states テーブルに OAuth の state を保持する（auth.StateStore の本番用の実装）。
//
// API サーバを複数台で動かしても、認可の開始とコールバックが別の台に振り分けられて
// 失敗しないよう、台をまたいで共有できる DB に置く（docs/step7-infra.md 6 章 #1）。
type OAuthState struct {
	db *gorm.DB
}

// NewOAuthState は OAuthState リポジトリを生成する。
func NewOAuthState(db *gorm.DB) *OAuthState {
	return &OAuthState{db: db}
}

var _ auth.StateStore = (*OAuthState)(nil)

// Issue は新しい state を発行し、data と紐づけて保存する。
//
// 発行のたびに期限切れの行を消し、ログインされないまま残った state が溜まり続けないようにする。
func (r *OAuthState) Issue(ctx context.Context, data auth.StateData, now time.Time) (string, error) {
	state, err := auth.GenerateState()
	if err != nil {
		return "", err
	}

	if err := r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&oauthStateRow{}).Error; err != nil {
		return "", fmt.Errorf("期限切れの state の削除に失敗しました: %w", err)
	}

	row := oauthStateRow{
		State:       state,
		Purpose:     string(data.Purpose),
		RedirectURI: data.RedirectURI,
		ExpiresAt:   now.Add(auth.StateTTL),
	}
	if data.UserID != 0 {
		row.UserID = &data.UserID
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return "", fmt.Errorf("state の保存に失敗しました: %w", err)
	}
	return state, nil
}

// Consume は state を検証し、紐づく値を返して削除する。
//
// 取り出しと削除を 1 つの DELETE ... RETURNING で行う。同じ state のコールバックが
// 2 台に同時に届いても、行を受け取れるのは片方だけになり、使い回しを防げる。
// 期限切れ・用途違いでも、見つかった時点で削除する（再利用させない）。
func (r *OAuthState) Consume(ctx context.Context, state string, purpose auth.StatePurpose, now time.Time) (auth.StateData, error) {
	var rows []oauthStateRow
	err := r.db.WithContext(ctx).
		Raw("DELETE FROM oauth_states WHERE state = ? RETURNING state, purpose, redirect_uri, user_id, expires_at, created_at", state).
		Scan(&rows).Error
	if err != nil {
		return auth.StateData{}, fmt.Errorf("state の取り出しに失敗しました: %w", err)
	}
	if len(rows) == 0 {
		return auth.StateData{}, auth.ErrStateNotFound
	}

	row := rows[0]
	data := auth.StateData{
		Purpose:     auth.StatePurpose(row.Purpose),
		RedirectURI: row.RedirectURI,
	}
	if row.UserID != nil {
		data.UserID = *row.UserID
	}
	return auth.CheckConsumed(data, row.ExpiresAt, purpose, now)
}
