package server

import (
	"net/http"
	"testing"
	"time"

	"github.com/example/myblog/apps/api/internal/config"
)

func TestNewAppliesHTTPConfiguration(t *testing.T) {
	handler := http.NewServeMux()
	cfg := config.Config{
		HTTPAddr:          "127.0.0.1:8080",
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      3 * time.Second,
		IdleTimeout:       4 * time.Second,
	}

	httpServer := New(cfg, handler)
	if httpServer.Addr != cfg.HTTPAddr || httpServer.Handler != handler {
		t.Fatalf("server address or handler was not applied")
	}
	if httpServer.ReadHeaderTimeout != cfg.ReadHeaderTimeout ||
		httpServer.ReadTimeout != cfg.ReadTimeout ||
		httpServer.WriteTimeout != cfg.WriteTimeout ||
		httpServer.IdleTimeout != cfg.IdleTimeout {
		t.Fatalf("server timeouts do not match config: %+v", httpServer)
	}
}
