// Package auth は Bearer トークン（JWT）の発行・検証と、
// OAuth の state 管理を扱う。
package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenTTL は Bearer トークンの有効期限（API 仕様書 3 章）。
const TokenTTL = 24 * time.Hour

// トークン検証のエラー。API 仕様書のエラーコードに対応する。
var (
	// ErrTokenInvalid は署名不正・形式不正。401 INVALID_TOKEN に対応する。
	ErrTokenInvalid = errors.New("トークンが不正です")
	// ErrTokenExpired は有効期限切れ。401 TOKEN_EXPIRED に対応する。
	ErrTokenExpired = errors.New("トークンの有効期限が切れています")
)

// TokenIssuer は JWT（HS256）の発行と検証を行う。
type TokenIssuer struct {
	secret []byte
}

// NewTokenIssuer は署名鍵を指定して TokenIssuer を生成する。
func NewTokenIssuer(secret string) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret)}
}

// Issue はユーザ ID を subject に持つトークンを発行する。
func (t *TokenIssuer) Issue(userID int64, now time.Time) (string, error) {
	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", fmt.Errorf("トークンの署名に失敗しました: %w", err)
	}
	return signed, nil
}

// Verify はトークンを検証し、ユーザ ID を返す。
//
// 期限切れは ErrTokenExpired、それ以外の不正は ErrTokenInvalid を返す。
// 呼び出し側はこの 2 つを区別して 401 のエラーコードを出し分ける。
func (t *TokenIssuer) Verify(tokenString string) (int64, error) {
	var claims jwt.RegisteredClaims

	_, err := jwt.ParseWithClaims(tokenString, &claims,
		func(token *jwt.Token) (any, error) {
			// alg の差し替え攻撃を防ぐため、署名方式を HS256 に固定する
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("想定外の署名方式です: %v", token.Header["alg"])
			}
			return t.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return 0, ErrTokenExpired
		}
		return 0, ErrTokenInvalid
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID < 1 {
		return 0, ErrTokenInvalid
	}
	return userID, nil
}
