package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc は HTTP の通信をせずに応答を返すための差し替え用の Transport。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCheckHealth(t *testing.T) {
	const url = "http://127.0.0.1:8080/health"

	respond := func(status int) roundTripFunc {
		return func(r *http.Request) (*http.Response, error) {
			if r.Method != http.MethodGet || r.URL.String() != url {
				t.Errorf("リクエスト = %s %s、期待値 = GET %s", r.Method, r.URL, url)
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}"))}, nil
		}
	}

	tests := []struct {
		name      string
		transport roundTripFunc
		wantErr   bool
	}{
		{name: "200 なら正常", transport: respond(http.StatusOK)},
		{name: "503（DB に繋がらない）なら異常", transport: respond(http.StatusServiceUnavailable), wantErr: true},
		{
			name: "接続できない（サーバが起動していない）なら異常",
			transport: func(*http.Request) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &http.Client{Transport: tt.transport}
			err := checkHealth(context.Background(), client, url)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkHealth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRunHealthcheckInvalidPort(t *testing.T) {
	t.Setenv("PORT", "abc")
	if got := runHealthcheck(); got != 1 {
		t.Errorf("runHealthcheck() = %d、期待値 = 1", got)
	}
}
