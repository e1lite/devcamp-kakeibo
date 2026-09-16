package config_test

import (
	"testing"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/config"
)

func TestLoad(t *testing.T) {
	const (
		validDSN      = "postgres://kakeibo:kakeibo@localhost:5433/kakeibo?sslmode=disable"
		validSecret   = "test-secret"
		validClientID = "test-client-id.apps.googleusercontent.com"
		validClientSc = "test-client-secret"
	)

	// required は必須の環境変数をすべて埋めたものを返す。
	// 個別のケースでは、この上に上書き・削除して条件を作る。
	required := func(overrides map[string]string) map[string]string {
		env := map[string]string{
			"DATABASE_URL":         validDSN,
			"JWT_SECRET":           validSecret,
			"GOOGLE_CLIENT_ID":     validClientID,
			"GOOGLE_CLIENT_SECRET": validClientSc,
		}
		for key, value := range overrides {
			if value == "" {
				delete(env, key)
				continue
			}
			env[key] = value
		}
		return env
	}

	tests := []struct {
		name     string
		env      map[string]string
		wantErr  bool
		wantPort int
		wantEnv  string
	}{
		{
			name:     "必須の環境変数が揃っていれば既定値で読み込める",
			env:      required(nil),
			wantPort: 8080,
			wantEnv:  "local",
		},
		{
			name:     "PORT と APP_ENV は上書きできる",
			env:      required(map[string]string{"PORT": "9000", "APP_ENV": "production"}),
			wantPort: 9000,
			wantEnv:  "production",
		},
		{
			name:    "DATABASE_URL がなければエラー",
			env:     required(map[string]string{"DATABASE_URL": ""}),
			wantErr: true,
		},
		{
			name:    "JWT_SECRET がなければエラー",
			env:     required(map[string]string{"JWT_SECRET": ""}),
			wantErr: true,
		},
		{
			name:    "GOOGLE_CLIENT_ID がなければエラー",
			env:     required(map[string]string{"GOOGLE_CLIENT_ID": ""}),
			wantErr: true,
		},
		{
			name:    "GOOGLE_CLIENT_SECRET がなければエラー",
			env:     required(map[string]string{"GOOGLE_CLIENT_SECRET": ""}),
			wantErr: true,
		},
		{
			name:    "PORT が整数でなければエラー",
			env:     required(map[string]string{"PORT": "abc"}),
			wantErr: true,
		},
		{
			name:    "PORT が範囲外ならエラー",
			env:     required(map[string]string{"PORT": "70000"}),
			wantErr: true,
		},
		{
			// オープンリダイレクタを防ぐ許可リストが、既定の戻り先を
			// 含んでいない設定ミスを起動時に検出する
			name: "DEFAULT_REDIRECT_URI が許可リストにないとエラー",
			env: required(map[string]string{
				"DEFAULT_REDIRECT_URI":  "http://localhost:5173/auth/callback",
				"ALLOWED_REDIRECT_URIS": "http://example.com/callback",
			}),
			wantErr: true,
		},
	}

	envKeys := []string{
		"APP_ENV", "PORT", "DATABASE_URL", "JWT_SECRET",
		"GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL",
		"DEFAULT_REDIRECT_URI", "ALLOWED_REDIRECT_URIS",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv は環境変数を使うため t.Parallel と併用できない
			for _, key := range envKeys {
				t.Setenv(key, "")
			}
			for key, value := range tt.env {
				t.Setenv(key, value)
			}

			cfg, err := config.Load()

			if tt.wantErr {
				if err == nil {
					t.Fatal("エラーを期待したが nil でした")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}

			if cfg.Port != tt.wantPort {
				t.Errorf("Port: got %d, want %d", cfg.Port, tt.wantPort)
			}
			if cfg.Env != tt.wantEnv {
				t.Errorf("Env: got %q, want %q", cfg.Env, tt.wantEnv)
			}
			if got, want := cfg.IsLocal(), tt.wantEnv == "local"; got != want {
				t.Errorf("IsLocal(): got %v, want %v", got, want)
			}
		})
	}
}
