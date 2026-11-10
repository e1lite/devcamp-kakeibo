package repository_test

import (
	"errors"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
)

func TestOAuthState_発行と消費(t *testing.T) {
	tx := beginTx(t)
	user := newUser(t, repository.NewUser(tx))
	repo := repository.NewOAuthState(tx)
	now := time.Now()

	data := auth.StateData{
		Purpose:     auth.PurposeMailLink,
		RedirectURI: "http://localhost:5173/settings/mail-accounts",
		UserID:      user.ID,
	}
	state, err := repo.Issue(t.Context(), data, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	got, err := repo.Consume(t.Context(), state, auth.PurposeMailLink, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("消費に失敗しました: %v", err)
	}
	if got != data {
		t.Errorf("紐づけた値: got %+v, want %+v", got, data)
	}

	// 1 度使った state は、別の台に届いても使えない
	if _, cerr := repo.Consume(t.Context(), state, auth.PurposeMailLink, now.Add(time.Minute)); !errors.Is(cerr, auth.ErrStateNotFound) {
		t.Errorf("使用済みの state: got %v, want ErrStateNotFound", cerr)
	}
}

func TestOAuthState_ログインはユーザなしで保存できる(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewOAuthState(tx)
	now := time.Now()

	data := auth.StateData{Purpose: auth.PurposeLogin, RedirectURI: "http://localhost:5173/auth/callback"}
	state, err := repo.Issue(t.Context(), data, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	got, err := repo.Consume(t.Context(), state, auth.PurposeLogin, now)
	if err != nil {
		t.Fatalf("消費に失敗しました: %v", err)
	}
	if got != data {
		t.Errorf("紐づけた値: got %+v, want %+v", got, data)
	}
}

func TestOAuthState_用途違いと期限切れは使えず削除される(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewOAuthState(tx)
	now := time.Now()
	login := auth.StateData{Purpose: auth.PurposeLogin, RedirectURI: "http://localhost:5173/auth/callback"}

	// ログイン用の state をメール連携のコールバックに使い回させない（API-029）
	wrongPurpose, err := repo.Issue(t.Context(), login, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	if _, cerr := repo.Consume(t.Context(), wrongPurpose, auth.PurposeMailLink, now); !errors.Is(cerr, auth.ErrStateNotFound) {
		t.Errorf("用途違い: got %v, want ErrStateNotFound", cerr)
	}
	if _, cerr := repo.Consume(t.Context(), wrongPurpose, auth.PurposeLogin, now); !errors.Is(cerr, auth.ErrStateNotFound) {
		t.Errorf("用途違いで 1 度失敗した state が残っています: %v", cerr)
	}

	expired, err := repo.Issue(t.Context(), login, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	if _, cerr := repo.Consume(t.Context(), expired, auth.PurposeLogin, now.Add(auth.StateTTL+time.Second)); !errors.Is(cerr, auth.ErrStateNotFound) {
		t.Errorf("期限切れ: got %v, want ErrStateNotFound", cerr)
	}
}

func TestOAuthState_発行のたびに期限切れの行を消す(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewOAuthState(tx)
	now := time.Now()
	login := auth.StateData{Purpose: auth.PurposeLogin, RedirectURI: "http://localhost:5173/auth/callback"}

	old, err := repo.Issue(t.Context(), login, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	if _, err := repo.Issue(t.Context(), login, now); err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	var count int64
	if err := tx.Table("oauth_states").Where("state = ?", old).Count(&count).Error; err != nil {
		t.Fatalf("件数の取得に失敗しました: %v", err)
	}
	if count != 0 {
		t.Error("期限切れの state が残っています（ログインされない state が溜まり続ける）")
	}
}

func TestOAuthState_未発行のstate(t *testing.T) {
	tx := beginTx(t)
	repo := repository.NewOAuthState(tx)

	if _, cerr := repo.Consume(t.Context(), "never-issued", auth.PurposeLogin, time.Now()); !errors.Is(cerr, auth.ErrStateNotFound) {
		t.Errorf("got %v, want ErrStateNotFound", cerr)
	}
}
