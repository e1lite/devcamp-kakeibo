// Package config はアプリケーションの設定を環境変数から読み込む。
package config

import (
	"encoding/base64"
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

	// AllowedRedirectURIs はログイン・メール連携の後に戻してよいフロントの URL。
	// オープンリダイレクタにしないための許可リスト。
	AllowedRedirectURIs []string
	// DefaultRedirectURI はログインで redirect_uri が指定されなかった場合の戻り先。
	DefaultRedirectURI string

	// GoogleMailRedirectURL は Google に登録したメール連携のコールバック URL（API-030）。
	GoogleMailRedirectURL string
	// DefaultMailRedirectURI はメール連携で redirect_uri が指定されなかった場合の戻り先（メール連携画面）。
	DefaultMailRedirectURI string
	// MailCredentialKey はメール連携の認証情報（リフレッシュトークン）を暗号化する鍵。32 バイト。
	MailCredentialKey []byte
	// MailSearchQuery はメールの取込で Gmail を検索する条件。期間（after:）は同期のたびに付け足す。
	MailSearchQuery string
	// MailBackfillDays は初回の取込で遡る日数。
	MailBackfillDays int
}

// DefaultMailSearchQuery はカード会社の利用通知を探す既定の検索条件。
//
// 対象のカード会社が決まるまでの仮の条件で、件名だけで絞る。決済と関係の無いメールを
// サーバに保存しないよう、受信箱全体は取らない（docs/mail-import-mvp.md 判断 2）。
const DefaultMailSearchQuery = `subject:("利用のお知らせ" OR "利用速報" OR "ご利用通知")`

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
	cfg.DefaultMailRedirectURI = envOrDefault("DEFAULT_MAIL_REDIRECT_URI", "http://localhost:5173/settings/mail-accounts")

	// カンマ区切り。既定値は開発用フロント（Next.js を next dev -p 5173 で起動）の
	// ログインのコールバックとメール連携画面
	allowed := envOrDefault("ALLOWED_REDIRECT_URIS", cfg.DefaultRedirectURI+","+cfg.DefaultMailRedirectURI)
	for _, uri := range strings.Split(allowed, ",") {
		if trimmed := strings.TrimSpace(uri); trimmed != "" {
			cfg.AllowedRedirectURIs = append(cfg.AllowedRedirectURIs, trimmed)
		}
	}
	if !slices.Contains(cfg.AllowedRedirectURIs, cfg.DefaultRedirectURI) {
		return nil, errors.New("DEFAULT_REDIRECT_URI が ALLOWED_REDIRECT_URIS に含まれていません")
	}
	if !slices.Contains(cfg.AllowedRedirectURIs, cfg.DefaultMailRedirectURI) {
		return nil, errors.New("DEFAULT_MAIL_REDIRECT_URI が ALLOWED_REDIRECT_URIS に含まれていません")
	}

	cfg.GoogleMailRedirectURL = envOrDefault("GOOGLE_MAIL_REDIRECT_URL",
		fmt.Sprintf("http://localhost:%d/api/v1/mail-accounts/google/callback", cfg.Port))

	key, err := base64.StdEncoding.DecodeString(os.Getenv("MAIL_CREDENTIAL_KEY"))
	if err != nil || len(key) != 32 {
		return nil, errors.New("MAIL_CREDENTIAL_KEY には 32 バイトの鍵を base64 で指定してください" +
			"（例: openssl rand -base64 32 の出力）")
	}
	cfg.MailCredentialKey = key

	cfg.MailSearchQuery = envOrDefault("MAIL_SEARCH_QUERY", DefaultMailSearchQuery)

	days, err := strconv.Atoi(envOrDefault("MAIL_BACKFILL_DAYS", "90"))
	if err != nil || days < 1 {
		return nil, errors.New("MAIL_BACKFILL_DAYS は 1 以上の整数で指定してください")
	}
	cfg.MailBackfillDays = days

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
