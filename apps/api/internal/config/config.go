package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	Development = "development"
	Test        = "test"
	Production  = "production"
)

type Config struct {
	Environment        string
	HTTPAddr           string
	ReadHeaderTimeout  time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	ShutdownTimeout    time.Duration
	TrustedProxies     []string
	DatabaseDSN        string
	DatabaseMaxOpen    int
	DatabaseMaxIdle    int
	DatabaseMaxLife    time.Duration
	AppOrigin          string
	GitHubClientID     string
	GitHubClientSecret string
	GitHubAdminID      uint64
	GitHubCallbackURL  string
	OAuthStateSecret   string
	SessionCookieName  string
	SessionSecure      bool
	SessionTTL         time.Duration
	MinIOEndpoint      string
	MinIOAccessKey     string
	MinIOSecretKey     string
	MinIOBucket        string
	MinIOUseSSL        bool
	MediaPublicURL     string
}

func Load() (Config, error) {
	var err error
	cfg := Config{
		Environment:        envOrDefault("APP_ENV", Development),
		HTTPAddr:           envOrDefault("HTTP_ADDR", ":8080"),
		ReadHeaderTimeout:  5 * time.Second,
		ReadTimeout:        15 * time.Second,
		WriteTimeout:       30 * time.Second,
		IdleTimeout:        60 * time.Second,
		ShutdownTimeout:    10 * time.Second,
		TrustedProxies:     splitCSV(os.Getenv("TRUSTED_PROXIES")),
		DatabaseDSN:        envOrDefault("DATABASE_DSN", "blog:blog@tcp(127.0.0.1:3306)/blog?charset=utf8mb4&parseTime=true&loc=UTC&multiStatements=true"),
		DatabaseMaxOpen:    10,
		DatabaseMaxIdle:    5,
		DatabaseMaxLife:    30 * time.Minute,
		AppOrigin:          envOrDefault("APP_ORIGIN", "http://localhost:5173"),
		GitHubClientID:     strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID")),
		GitHubClientSecret: strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET")),
		GitHubCallbackURL:  envOrDefault("GITHUB_CALLBACK_URL", "http://localhost:8080/api/auth/github/callback"),
		OAuthStateSecret:   strings.TrimSpace(os.Getenv("OAUTH_STATE_SECRET")),
		SessionCookieName:  envOrDefault("SESSION_COOKIE_NAME", "blog_session"),
		SessionSecure:      envOrDefault("SESSION_COOKIE_SECURE", "false") == "true",
		SessionTTL:         7 * 24 * time.Hour,
		MinIOEndpoint:      envOrDefault("MINIO_ENDPOINT", "127.0.0.1:9000"),
		MinIOAccessKey:     envOrDefault("MINIO_ACCESS_KEY", "minioadmin"),
		MinIOSecretKey:     envOrDefault("MINIO_SECRET_KEY", "minioadmin"),
		MinIOBucket:        envOrDefault("MINIO_BUCKET", "blog-media"),
		MinIOUseSSL:        envOrDefault("MINIO_USE_SSL", "false") == "true",
		MediaPublicURL:     strings.TrimSuffix(envOrDefault("MEDIA_PUBLIC_URL", "/uploads"), "/"),
	}
	if value := strings.TrimSpace(os.Getenv("ADMIN_GITHUB_ID")); value != "" {
		cfg.GitHubAdminID, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("parse ADMIN_GITHUB_ID: %w", err)
		}
	}

	if cfg.ReadHeaderTimeout, err = durationFromEnv("HTTP_READ_HEADER_TIMEOUT", cfg.ReadHeaderTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ReadTimeout, err = durationFromEnv("HTTP_READ_TIMEOUT", cfg.ReadTimeout); err != nil {
		return Config{}, err
	}
	if cfg.WriteTimeout, err = durationFromEnv("HTTP_WRITE_TIMEOUT", cfg.WriteTimeout); err != nil {
		return Config{}, err
	}
	if cfg.IdleTimeout, err = durationFromEnv("HTTP_IDLE_TIMEOUT", cfg.IdleTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationFromEnv("HTTP_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseMaxOpen, err = intFromEnv("DATABASE_MAX_OPEN", cfg.DatabaseMaxOpen); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseMaxIdle, err = intFromEnv("DATABASE_MAX_IDLE", cfg.DatabaseMaxIdle); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseMaxLife, err = durationFromEnv("DATABASE_MAX_LIFETIME", cfg.DatabaseMaxLife); err != nil {
		return Config{}, err
	}
	if cfg.SessionTTL, err = durationFromEnv("SESSION_TTL", cfg.SessionTTL); err != nil {
		return Config{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate configuration: %w", err)
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch c.Environment {
	case Development, Test, Production:
	default:
		return fmt.Errorf("APP_ENV must be one of %q, %q or %q", Development, Test, Production)
	}

	_, port, err := net.SplitHostPort(c.HTTPAddr)
	if err != nil {
		return fmt.Errorf("HTTP_ADDR must use host:port format: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("HTTP_ADDR port must be between 1 and 65535")
	}

	for name, value := range map[string]time.Duration{
		"HTTP_READ_HEADER_TIMEOUT": c.ReadHeaderTimeout,
		"HTTP_READ_TIMEOUT":        c.ReadTimeout,
		"HTTP_WRITE_TIMEOUT":       c.WriteTimeout,
		"HTTP_IDLE_TIMEOUT":        c.IdleTimeout,
		"HTTP_SHUTDOWN_TIMEOUT":    c.ShutdownTimeout,
		"DATABASE_MAX_LIFETIME":    c.DatabaseMaxLife,
		"SESSION_TTL":              c.SessionTTL,
	} {
		if value <= 0 {
			return fmt.Errorf("%s must be greater than zero", name)
		}
	}
	if strings.TrimSpace(c.DatabaseDSN) == "" {
		return errors.New("DATABASE_DSN must not be empty")
	}
	if c.DatabaseMaxOpen < 1 {
		return errors.New("DATABASE_MAX_OPEN must be at least 1")
	}
	if c.DatabaseMaxIdle < 0 || c.DatabaseMaxIdle > c.DatabaseMaxOpen {
		return errors.New("DATABASE_MAX_IDLE must be between 0 and DATABASE_MAX_OPEN")
	}
	appOrigin, err := url.Parse(c.AppOrigin)
	if err != nil || (appOrigin.Scheme != "http" && appOrigin.Scheme != "https") || appOrigin.Host == "" ||
		appOrigin.User != nil || appOrigin.Path != "" || appOrigin.RawQuery != "" || appOrigin.Fragment != "" {
		return errors.New("APP_ORIGIN must be an origin such as https://blog.example.com")
	}
	callback, err := url.Parse(c.GitHubCallbackURL)
	if err != nil || (callback.Scheme != "http" && callback.Scheme != "https") || callback.Host == "" || callback.User != nil ||
		callback.Path != "/api/auth/github/callback" || callback.RawQuery != "" || callback.Fragment != "" {
		return errors.New("GITHUB_CALLBACK_URL must be an absolute /api/auth/github/callback URL")
	}
	if !regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`).MatchString(c.SessionCookieName) {
		return errors.New("SESSION_COOKIE_NAME must be a valid cookie name")
	}
	minioHost, minioPort, err := net.SplitHostPort(c.MinIOEndpoint)
	portNumber, portErr := strconv.Atoi(minioPort)
	if err != nil || minioHost == "" || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("MINIO_ENDPOINT must use host:port format without a URL scheme")
	}
	if c.MinIOAccessKey == "" || c.MinIOSecretKey == "" {
		return errors.New("MinIO access credentials must not be empty")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`).MatchString(c.MinIOBucket) {
		return errors.New("MINIO_BUCKET is not a valid S3 bucket name")
	}
	if !validPublicMediaURL(c.MediaPublicURL) {
		return errors.New("MEDIA_PUBLIC_URL must be an absolute URL or an absolute path")
	}
	if c.Environment == Production {
		if c.GitHubClientID == "" || c.GitHubClientSecret == "" || c.GitHubAdminID == 0 {
			return errors.New("GitHub OAuth settings and ADMIN_GITHUB_ID are required in production")
		}
		if len(c.OAuthStateSecret) < 32 {
			return errors.New("OAUTH_STATE_SECRET must contain at least 32 characters in production")
		}
		if len(c.MinIOAccessKey) < 3 || len(c.MinIOSecretKey) < 32 {
			return errors.New("production MinIO access key and secret must be at least 3 and 32 characters")
		}
		if !c.SessionSecure || appOrigin.Scheme != "https" || callback.Scheme != "https" {
			return errors.New("production cookies, APP_ORIGIN and GITHUB_CALLBACK_URL must use HTTPS")
		}
		if !strings.EqualFold(appOrigin.Scheme, callback.Scheme) || !strings.EqualFold(appOrigin.Host, callback.Host) {
			return errors.New("production GITHUB_CALLBACK_URL must use the APP_ORIGIN origin")
		}
		if c.MinIOAccessKey == "minioadmin" || c.MinIOSecretKey == "minioadmin" {
			return errors.New("default MinIO credentials are forbidden in production")
		}
	}
	return nil
}

func validPublicMediaURL(value string) bool {
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.IsAbs() {
		return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && strings.HasPrefix(parsed.Path, "/")
	}
	decoded, err := url.PathUnescape(parsed.Path)
	return err == nil && parsed.Host == "" && strings.HasPrefix(decoded, "/") && decoded != "/" && path.Clean(decoded) == decoded && !strings.HasSuffix(decoded, "/")
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return duration, nil
}

func intFromEnv(name string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			result = append(result, value)
		}
	}
	return result
}
