package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("TRUSTED_PROXIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Environment != Development || cfg.HTTPAddr != ":8080" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.ReadHeaderTimeout != 5*time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected timeout defaults: %+v", cfg)
	}
	if cfg.DatabaseMaxOpen != 10 || cfg.DatabaseMaxIdle != 5 || cfg.DatabaseDSN == "" {
		t.Fatalf("unexpected database defaults: %+v", cfg)
	}
}

func TestLoadParsesOverrides(t *testing.T) {
	t.Setenv("APP_ENV", Production)
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.0/8")
	t.Setenv("DATABASE_MAX_OPEN", "20")
	t.Setenv("DATABASE_MAX_IDLE", "8")
	t.Setenv("APP_ORIGIN", "https://blog.example.com")
	t.Setenv("GITHUB_CALLBACK_URL", "https://blog.example.com/api/auth/github/callback")
	t.Setenv("GITHUB_CLIENT_ID", "client-id")
	t.Setenv("GITHUB_CLIENT_SECRET", "client-secret")
	t.Setenv("ADMIN_GITHUB_ID", "12345678")
	t.Setenv("OAUTH_STATE_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Setenv("MINIO_ACCESS_KEY", "production-access")
	t.Setenv("MINIO_SECRET_KEY", "0123456789abcdef0123456789abcdef")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
	if len(cfg.TrustedProxies) != 2 {
		t.Fatalf("TrustedProxies = %#v", cfg.TrustedProxies)
	}
	if cfg.DatabaseMaxOpen != 20 || cfg.DatabaseMaxIdle != 8 {
		t.Fatalf("unexpected database pool settings: %+v", cfg)
	}
}

func TestValidateRejectsUnsafeURLAndCredentialConfiguration(t *testing.T) {
	valid := Config{
		Environment: Production, HTTPAddr: ":8080", ReadHeaderTimeout: time.Second, ReadTimeout: time.Second,
		WriteTimeout: time.Second, IdleTimeout: time.Second, ShutdownTimeout: time.Second,
		DatabaseDSN: "dsn", DatabaseMaxOpen: 1, DatabaseMaxIdle: 1, DatabaseMaxLife: time.Minute,
		AppOrigin: "https://blog.example.com", GitHubClientID: "id", GitHubClientSecret: "secret", GitHubAdminID: 1,
		GitHubCallbackURL: "https://blog.example.com/api/auth/github/callback", OAuthStateSecret: "0123456789abcdef0123456789abcdef",
		SessionCookieName: "blog_session", SessionSecure: true, SessionTTL: time.Hour,
		MinIOEndpoint: "minio:9000", MinIOAccessKey: "access", MinIOSecretKey: "0123456789abcdef0123456789abcdef",
		MinIOBucket: "blog-media", MediaPublicURL: "/uploads",
	}
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "origin query", mutate: func(c *Config) { c.AppOrigin = "https://blog.example.com?next=evil" }},
		{name: "callback other origin", mutate: func(c *Config) { c.GitHubCallbackURL = "https://evil.example/api/auth/github/callback" }},
		{name: "callback wrong path", mutate: func(c *Config) { c.GitHubCallbackURL = "https://blog.example.com/callback" }},
		{name: "cookie injection", mutate: func(c *Config) { c.SessionCookieName = "session; evil" }},
		{name: "invalid MinIO endpoint", mutate: func(c *Config) { c.MinIOEndpoint = "minio:not-a-port" }},
		{name: "short MinIO secret", mutate: func(c *Config) { c.MinIOSecretKey = "short" }},
		{name: "media traversal", mutate: func(c *Config) { c.MediaPublicURL = "/uploads/../private" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid configuration rejected: %v", err)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "environment", key: "APP_ENV", value: "staging"},
		{name: "address", key: "HTTP_ADDR", value: "8080"},
		{name: "duration", key: "HTTP_READ_TIMEOUT", value: "soon"},
		{name: "database max open", key: "DATABASE_MAX_OPEN", value: "many"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			if _, err := Load(); err == nil {
				t.Fatal("Load() error = nil, want an error")
			}
		})
	}
}
