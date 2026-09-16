// Package config はアプリケーションの設定を環境変数から読み込む。
package config

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

// Config はアプリケーション全体の設定を保持する。
type Config struct {
	// Env は実行環境。local / staging / production を想定する。
	Env string
	// Port は API サーバが待ち受けるポート。
	Port int
	// DatabaseURL は PostgreSQL への接続文字列。
	DatabaseURL string
	// JWTSecret は Bearer トークン（JWT / HS256）の署名鍵。
	JWTSecret string

	// GoogleClientID / GoogleClientSecret は Google OAuth のクライアント認証情報。
	GoogleClientID     string
	GoogleClientSecret string
	// GoogleRedirectURL は Google に登録したコールバック URL（API-003）。
	GoogleRedirectURL string

	// AllowedRedirectURIs はログイン後に戻してよいフロントの URL。
	// オープンリダイレクタにしないための許可リスト。
	AllowedRedirectURIs []string
	// DefaultRedirectURI は redirect_uri が指定されなかった場合の戻り先。
	DefaultRedirectURI string
}

// IsLocal はローカル開発環境かどうかを返す。
func (c *Config) IsLocal() bool {
	return c.Env == "local"
}

// Load は環境変数から設定を読み込む。
// 必須の環境変数が欠けている場合はエラーを返す。
func Load() (*Config, error) {
	cfg := &Config{
		Env: envOrDefault("APP_ENV", "local"),
	}

	port, err := strconv.Atoi(envOrDefault("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("PORT は整数で指定してください: %w", err)
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("PORT が範囲外です: %d", port)
	}
	cfg.Port = port

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, errors.New("DATABASE_URL が設定されていません")
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET が設定されていません")
	}

	cfg.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	if cfg.GoogleClientID == "" {
		return nil, errors.New("GOOGLE_CLIENT_ID が設定されていません")
	}

	cfg.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	if cfg.GoogleClientSecret == "" {
		return nil, errors.New("GOOGLE_CLIENT_SECRET が設定されていません")
	}

	cfg.GoogleRedirectURL = envOrDefault("GOOGLE_REDIRECT_URL",
		fmt.Sprintf("http://localhost:%d/api/v1/auth/google/callback", cfg.Port))

	cfg.DefaultRedirectURI = envOrDefault("DEFAULT_REDIRECT_URI", "http://localhost:5173/auth/callback")

	// カンマ区切り。既定値は開発用フロント（Vite）のコールバック
	allowed := envOrDefault("ALLOWED_REDIRECT_URIS", cfg.DefaultRedirectURI)
	for _, uri := range strings.Split(allowed, ",") {
		if trimmed := strings.TrimSpace(uri); trimmed != "" {
			cfg.AllowedRedirectURIs = append(cfg.AllowedRedirectURIs, trimmed)
		}
	}
	if !slices.Contains(cfg.AllowedRedirectURIs, cfg.DefaultRedirectURI) {
		return nil, errors.New("DEFAULT_REDIRECT_URI が ALLOWED_REDIRECT_URIS に含まれていません")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
