// Command api は決済メール解析による自動家計簿サービスの API サーバ。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/e1lite/devcamp-kakeibo/backend/internal/auth"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/config"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/database"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/gmail"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/handler"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/repository"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/server"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/service"
)

// version はビルド時に -ldflags で埋め込む。
var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("起動に失敗しました", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	db, err := database.Open(cfg.DatabaseURL, cfg.IsLocal())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := database.Close(db); cerr != nil {
			slog.Error("データベース接続のクローズに失敗しました", "error", cerr)
		}
	}()

	tokens := auth.NewTokenIssuer(cfg.JWTSecret)
	userRepo := repository.NewUser(db)
	// ログインとメール連携で同じ state の保管場所を使う。用途（StatePurpose）で区別し、
	// ログイン用の state をメール連携に使い回させない
	states := auth.NewStateStore()
	authService := service.NewAuth(service.AuthConfig{
		OAuth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleRedirectURL,
			Endpoint:     google.Endpoint,
			// ログインに必要な最小のスコープ。メール読み取りの
			// gmail.readonly はここでは要求しない（API 仕様書 3 章の認証 2 層）
			Scopes: []string{"openid", "email", "profile"},
		},
		States:              states,
		Tokens:              tokens,
		Users:               userRepo,
		AllowedRedirectURIs: cfg.AllowedRedirectURIs,
		DefaultRedirectURI:  cfg.DefaultRedirectURI,
	})

	credentialCipher, err := auth.NewCredentialCipher(cfg.MailCredentialKey)
	if err != nil {
		return err
	}
	mailService := service.NewMail(service.MailConfig{
		OAuth: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.GoogleMailRedirectURL,
			Endpoint:     google.Endpoint,
			// メールの読み取りだけ。ログインとは別に、メール連携画面から求める（API 仕様書 3 章の認証 2 層）
			Scopes: []string{gmail.ReadonlyScope},
		},
		States:              states,
		Cipher:              credentialCipher,
		Accounts:            repository.NewMailAccount(db),
		Jobs:                repository.NewSyncJob(db),
		Messages:            repository.NewEmailMessage(db),
		Templates:           repository.NewParserTemplate(db),
		AllowedRedirectURIs: cfg.AllowedRedirectURIs,
		DefaultRedirectURI:  cfg.DefaultMailRedirectURI,
		SearchQuery:         cfg.MailSearchQuery,
		BackfillDays:        cfg.MailBackfillDays,
		GmailBaseURL:        gmail.DefaultBaseURL,
	})
	// 前回サーバを止めたときに実行中だった同期は中断されている。失敗として記録し、次の同期を始められるようにする
	if err := mailService.RecoverInterrupted(context.Background()); err != nil {
		return err
	}
	// 全ユーザ共通のテンプレート（三井住友カードの利用通知など）を、コードの定義に合わせる
	if err := mailService.SyncBuiltinTemplates(context.Background()); err != nil {
		return err
	}

	categoryRepo := repository.NewCategory(db)
	categoryService := service.NewCategory(categoryRepo)
	paymentMethodRepo := repository.NewPaymentMethod(db)
	transactionService := service.NewTransaction(
		repository.NewTransaction(db),
		repository.NewMerchant(db),
		categoryRepo,
		paymentMethodRepo,
	)

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: server.NewRouter(server.Deps{
			Version:       version,
			Ping:          func(ctx context.Context) error { return database.Ping(ctx, db) },
			Tokens:        tokens,
			UserExists:    userRepo.Exists,
			Auth:          handler.NewAuth(authService),
			Category:      handler.NewCategory(categoryService),
			Transaction:   handler.NewTransaction(transactionService),
			PaymentMethod: handler.NewPaymentMethod(service.NewPaymentMethod(paymentMethodRepo)),
			Mail:          handler.NewMail(mailService),
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return serveUntilSignal(srv)
}

// serveUntilSignal はサーバを起動し、SIGINT / SIGTERM でグレースフルに停止する。
//
// 停止シグナルを受けたら新規接続の受付をやめ、処理中のリクエストが
// 終わるのを最大 10 秒待つ。途中で切ると書き込み中のトランザクションが
// 中途半端な状態で終わる可能性があるため。
func serveUntilSignal(srv *http.Server) error {
	shutdownErr := make(chan error, 1)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		sig := <-quit

		slog.Info("停止シグナルを受信しました", "signal", sig.String())

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		shutdownErr <- srv.Shutdown(ctx)
	}()

	slog.Info("API サーバを起動しました", "addr", srv.Addr, "version", version)

	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("サーバの起動に失敗しました: %w", err)
	}

	if err := <-shutdownErr; err != nil {
		return fmt.Errorf("グレースフル停止に失敗しました: %w", err)
	}

	slog.Info("API サーバを停止しました")
	return nil
}
