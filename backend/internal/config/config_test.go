package config_test

import (
	"testing"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/config"
)

func TestLoad(t *testing.T) {
	const (
		validDSN    = "postgres://kakeibo:kakeibo@localhost:5433/kakeibo?sslmode=disable"
		validSecret = "test-secret"
	)

	tests := []struct {
		name     string
		env      map[string]string
		wantErr  bool
		wantPort int
		wantEnv  string
	}{
		{
			name:     "必須の環境変数が揃っていれば既定値で読み込める",
			env:      map[string]string{"DATABASE_URL": validDSN, "JWT_SECRET": validSecret},
			wantPort: 8080,
			wantEnv:  "local",
		},
		{
			name: "PORT と APP_ENV は上書きできる",
			env: map[string]string{
				"DATABASE_URL": validDSN, "JWT_SECRET": validSecret,
				"PORT": "9000", "APP_ENV": "production",
			},
			wantPort: 9000,
			wantEnv:  "production",
		},
		{
			name:    "DATABASE_URL がなければエラー",
			env:     map[string]string{"JWT_SECRET": validSecret},
			wantErr: true,
		},
		{
			name:    "JWT_SECRET がなければエラー",
			env:     map[string]string{"DATABASE_URL": validDSN},
			wantErr: true,
		},
		{
			name: "PORT が整数でなければエラー",
			env: map[string]string{
				"DATABASE_URL": validDSN, "JWT_SECRET": validSecret, "PORT": "abc",
			},
			wantErr: true,
		},
		{
			name: "PORT が範囲外ならエラー",
			env: map[string]string{
				"DATABASE_URL": validDSN, "JWT_SECRET": validSecret, "PORT": "70000",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv は環境変数を使うため t.Parallel と併用できない
			for _, key := range []string{"APP_ENV", "PORT", "DATABASE_URL", "JWT_SECRET"} {
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
