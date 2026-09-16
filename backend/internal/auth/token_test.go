package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
)

const testSecret = "test-secret-for-unit-tests"

func TestTokenIssuer_IssueAndVerify(t *testing.T) {
	t.Parallel()

	issuer := auth.NewTokenIssuer(testSecret)
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	token, err := issuer.Issue(42, now)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	userID, err := issuer.Verify(token)
	if err != nil {
		t.Fatalf("検証に失敗しました: %v", err)
	}
	if userID != 42 {
		t.Errorf("ユーザ ID: got %d, want 42", userID)
	}
}

func TestTokenIssuer_Verify_Expired(t *testing.T) {
	t.Parallel()

	issuer := auth.NewTokenIssuer(testSecret)

	// 有効期限（24 時間）より前に発行されたトークン
	issuedAt := time.Now().Add(-auth.TokenTTL - time.Minute)
	token, err := issuer.Issue(1, issuedAt)
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	_, err = issuer.Verify(token)
	if !errors.Is(err, auth.ErrTokenExpired) {
		t.Errorf("期限切れを区別できていません: got %v, want ErrTokenExpired", err)
	}
}

func TestTokenIssuer_Verify_Invalid(t *testing.T) {
	t.Parallel()

	issuer := auth.NewTokenIssuer(testSecret)
	valid, err := issuer.Issue(1, time.Now())
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}

	// alg=none への差し替え。ヘッダとペイロードだけで署名が空のトークン
	const algNone = "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0." +
		"eyJzdWIiOiIxIiwiZXhwIjo0MTAyNDQ0ODAwfQ."

	tests := []struct {
		name  string
		token string
	}{
		{name: "空文字", token: ""},
		{name: "JWT の形をしていない", token: "not-a-jwt"},
		{name: "別の鍵で署名されている", token: signedWithOtherSecret(t)},
		{name: "署名部分を改ざん", token: valid[:len(valid)-4] + "AAAA"},
		{name: "alg=none への差し替え", token: algNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := issuer.Verify(tt.token); !errors.Is(err, auth.ErrTokenInvalid) {
				t.Errorf("got %v, want ErrTokenInvalid", err)
			}
		})
	}
}

func signedWithOtherSecret(t *testing.T) string {
	t.Helper()

	token, err := auth.NewTokenIssuer("another-secret").Issue(1, time.Now())
	if err != nil {
		t.Fatalf("発行に失敗しました: %v", err)
	}
	return token
}
