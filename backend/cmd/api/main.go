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

	"github.com/e1lite/devcamp-kakeibo/backend/internal/config"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/database"
	"github.com/e1lite/devcamp-kakeibo/backend/internal/server"
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

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
		Handler: server.NewRouter(server.Deps{
			Version: version,
			Ping:    func(ctx context.Context) error { return database.Ping(ctx, db) },
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
