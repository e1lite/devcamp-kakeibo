package auth_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
)

func newCipher(t *testing.T, fill byte) *auth.CredentialCipher {
	t.Helper()
	c, err := auth.NewCredentialCipher(bytes.Repeat([]byte{fill}, auth.CredentialKeySize))
	if err != nil {
		t.Fatalf("初期化に失敗しました: %v", err)
	}
	return c
}

func TestCredentialCipher_暗号化して復号できる(t *testing.T) {
	t.Parallel()

	c := newCipher(t, 1)
	const token = "1//0refresh-token-example"

	encrypted, err := c.Encrypt(token)
	if err != nil {
		t.Fatalf("暗号化に失敗しました: %v", err)
	}
	if strings.Contains(encrypted, token) {
		t.Fatal("暗号文に平文がそのまま含まれています")
	}

	got, err := c.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("復号に失敗しました: %v", err)
	}
	if got != token {
		t.Errorf("復号結果: got %q, want %q", got, token)
	}
}

func TestCredentialCipher_同じ値でも毎回違う暗号文になる(t *testing.T) {
	t.Parallel()

	c := newCipher(t, 1)
	first, _ := c.Encrypt("same")
	second, _ := c.Encrypt("same")
	if first == second {
		t.Error("nonce が使い回されています（同じ暗号文になりました）")
	}
}

func TestCredentialCipher_鍵が違えば復号できない(t *testing.T) {
	t.Parallel()

	encrypted, _ := newCipher(t, 1).Encrypt("secret")
	if _, err := newCipher(t, 2).Decrypt(encrypted); !errors.Is(err, auth.ErrCredentialDecrypt) {
		t.Errorf("got %v, want ErrCredentialDecrypt", err)
	}
}

func TestCredentialCipher_改ざんされた値は復号できない(t *testing.T) {
	t.Parallel()

	c := newCipher(t, 1)
	encrypted, _ := c.Encrypt("secret")
	// 暗号文の先頭の 1 文字を変える（base64 として正しいまま中身だけを変える）
	replacement := "A"
	if encrypted[:1] == "A" {
		replacement = "B"
	}
	tampered := replacement + encrypted[1:]

	for _, value := range []string{tampered, "not-base64!!", ""} {
		if _, err := c.Decrypt(value); !errors.Is(err, auth.ErrCredentialDecrypt) {
			t.Errorf("Decrypt(%q): got %v, want ErrCredentialDecrypt", value, err)
		}
	}
}

func TestNewCredentialCipher_鍵の長さを確かめる(t *testing.T) {
	t.Parallel()

	if _, err := auth.NewCredentialCipher(make([]byte, 16)); err == nil {
		t.Error("16 バイトの鍵を受け付けてしまいます（AES-256 には 32 バイト必要）")
	}
}
