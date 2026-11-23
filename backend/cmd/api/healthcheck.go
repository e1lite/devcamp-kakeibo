package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// healthcheckTimeout は 1 回のヘルスチェックの上限。Docker の healthcheck の timeout より短くする。
const healthcheckTimeout = 3 * time.Second

// runHealthcheck は同じコンテナで動いている API サーバの GET /health（API-001）を呼び、
// 正常なら 0、異常なら 1 を返す（Docker の healthcheck は終了コードで判定する）。
//
// /health は DB への疎通も確かめるため、DB に繋がらないコンテナも異常（unhealthy）になる。
// 起動の設定（config.Load）は読まない。ヘルスチェックに必要なのは待ち受けポートだけのため。
func runHealthcheck() int {
	port, err := strconv.Atoi(envOrDefault("PORT", "8080"))
	if err != nil || port < 1 || port > 65535 {
		fmt.Fprintln(os.Stderr, "PORT は 1〜65535 の整数で指定してください")
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()

	if err := checkHealth(ctx, http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d/health", port)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// checkHealth は url に GET し、200 以外ならエラーを返す。テストからクライアントを差し替えられるよう分けている。
func checkHealth(ctx context.Context, client *http.Client, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("ヘルスチェックのリクエストを作れません: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ヘルスチェックに失敗しました: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ヘルスチェックの応答が %d でした", resp.StatusCode)
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
