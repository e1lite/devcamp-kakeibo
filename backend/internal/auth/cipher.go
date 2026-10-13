package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// CredentialKeySize は認証情報の暗号鍵の長さ（AES-256）。
const CredentialKeySize = 32

// ErrCredentialDecrypt は保存した認証情報を復号できないことを表す。
// 鍵を変えた・値が壊れているなどで起き、連携し直す必要がある。
var ErrCredentialDecrypt = errors.New("認証情報を復号できません")

// CredentialCipher はメール連携の認証情報（Google のリフレッシュトークン）を暗号化する。
//
// リフレッシュトークンがあれば、ユーザがいなくてもメールを読み続けられる。
// データベースの中身が漏れてもそのまま使えないよう、AES-256-GCM で暗号化して
// mail_accounts.credential_encrypted に保存する（DB 仕様書 3.2、docs/mail-import-mvp.md 判断 8）。
// 鍵はデータベースとは別に、環境変数で渡す。
type CredentialCipher struct {
	aead cipher.AEAD
}

// NewCredentialCipher は 32 バイトの鍵から CredentialCipher を作る。
func NewCredentialCipher(key []byte) (*CredentialCipher, error) {
	if len(key) != CredentialKeySize {
		return nil, fmt.Errorf("暗号鍵は %d バイトで指定してください（%d バイトでした）", CredentialKeySize, len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("暗号鍵の初期化に失敗しました: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("暗号鍵の初期化に失敗しました: %w", err)
	}
	return &CredentialCipher{aead: aead}, nil
}

// Encrypt は平文を暗号化し、nonce と暗号文をつないだものを base64 で返す。
// nonce は毎回乱数で作るため、同じ値を暗号化しても結果は毎回変わる。
func (c *CredentialCipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce の生成に失敗しました: %w", err)
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt は Encrypt した値を復号する。改ざんされた値は GCM の認証で弾く。
func (c *CredentialCipher) Decrypt(encoded string) (string, error) {
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrCredentialDecrypt
	}
	size := c.aead.NonceSize()
	if len(sealed) < size {
		return "", ErrCredentialDecrypt
	}
	plaintext, err := c.aead.Open(nil, sealed[:size], sealed[size:], nil)
	if err != nil {
		return "", ErrCredentialDecrypt
	}
	return string(plaintext), nil
}
