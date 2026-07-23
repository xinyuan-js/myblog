package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/example/myblog/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	router, err := NewRouter(config.Config{
		Environment:    config.Test,
		TrustedProxies: nil,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	return router
}

func TestAdminMiddlewareRequiresSessionAndCSRF(t *testing.T) {
	const token = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const secret = "ssssssssssssssssssssssssssssssss"
	csrfMAC := hmac.New(sha256.New, []byte(secret))
	_, _ = csrfMAC.Write([]byte("csrf:" + token))
	csrfToken := base64.RawURLEncoding.EncodeToString(csrfMAC.Sum(nil))
	tokenHash := sha256.Sum256([]byte(token))
	csrfHash := sha256.Sum256([]byte(csrfToken))

	tests := []struct {
		name       string
		cookie     bool
		origin     string
		csrf       string
		storedID   uint64
		wantStatus int
	}{
		{name: "missing session", wantStatus: http.StatusUnauthorized},
		{name: "missing csrf", cookie: true, origin: "https://blog.example.com", wantStatus: http.StatusForbidden},
		{name: "cross site origin", cookie: true, origin: "https://evil.example", csrf: csrfToken, wantStatus: http.StatusForbidden},
		{name: "ordinary user", cookie: true, storedID: 87654321, origin: "https://blog.example.com", csrf: csrfToken, wantStatus: http.StatusForbidden},
		{name: "valid request", cookie: true, origin: "https://blog.example.com", csrf: csrfToken, wantStatus: http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			service := auth.NewService(db, config.Config{
				Environment: config.Production, AppOrigin: "https://blog.example.com", SessionCookieName: "blog_session",
				GitHubAdminID: 12345678, OAuthStateSecret: secret,
			})
			if test.cookie {
				storedID := test.storedID
				if storedID == 0 {
					storedID = 12345678
				}
				rows := sqlmock.NewRows([]string{
					"github_id", "github_login", "display_name", "email", "avatar_url", "csrf_token_hash", "refresh_last_seen",
				}).AddRow(storedID, "user", "User", "user@example.com", "https://example.com/avatar.png", csrfHash[:], false)
				mock.ExpectQuery("SELECT session.github_id").WithArgs(tokenHash[:]).WillReturnRows(rows)
				if storedID != 12345678 {
					mock.ExpectQuery("SELECT EXISTS").WithArgs(storedID).
						WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				}
			}

			router := gin.New()
			router.POST("/write", requireAdmin(service, slog.New(slog.NewTextHandler(io.Discard, nil))), requireCSRF(service), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodPost, "/write", nil)
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: "blog_session", Value: token})
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.csrf != "" {
				request.Header.Set("X-CSRF-Token", test.csrf)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOwnerMiddlewareRejectsDelegatedAdministrator(t *testing.T) {
	for _, test := range []struct {
		name       string
		isOwner    bool
		wantStatus int
	}{
		{name: "delegated administrator", wantStatus: http.StatusForbidden},
		{name: "configured owner", isOwner: true, wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET("/permissions", func(c *gin.Context) {
				c.Set(adminSessionKey, auth.Session{User: auth.User{IsAdmin: true, IsOwner: test.isOwner}})
				c.Next()
			}, requireOwner(), func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/permissions", nil))
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
		})
	}
}

func TestRateLimiterBoundsClientState(t *testing.T) {
	limiter := &fixedWindowLimiter{limit: 1, window: time.Minute, clients: make(map[string]windowCounter)}
	now := time.Now()
	for index := 0; index < maxRateLimitClients+32; index++ {
		limiter.record(now.Add(time.Duration(index)*time.Nanosecond), fmt.Sprintf("client-%d", index))
	}
	if len(limiter.clients) > maxRateLimitClients {
		t.Fatalf("rate limiter retained %d clients, want at most %d", len(limiter.clients), maxRateLimitClients)
	}
	if blocked, _ := limiter.record(now, "repeated-client"); blocked {
		t.Fatal("first request was unexpectedly blocked")
	}
	if blocked, retry := limiter.record(now, "repeated-client"); !blocked || retry < 1 {
		t.Fatalf("second request blocked=%v retry=%d", blocked, retry)
	}
}

func TestOAuthCallbackCannotBeStarvedBySharedIPRateLimit(t *testing.T) {
	cfg := config.Config{Environment: config.Test, TrustedProxies: nil}
	service := auth.NewService(nil, config.Config{
		AppOrigin: "https://blog.example.com", SessionCookieName: "blog_session",
	})
	router, err := NewRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{Auth: service})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 64; index++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodGet, "/api/auth/github/callback?error=access_denied", nil,
		)
		request.RemoteAddr = "192.0.2.10:12345"
		router.ServeHTTP(response, request)
		if response.Code != http.StatusFound {
			t.Fatalf("callback %d status = %d, want %d", index+1, response.Code, http.StatusFound)
		}
	}
}

func TestHealthEndpoints(t *testing.T) {
	for _, path := range []string{"/api/healthz", "/api/readyz"} {
		t.Run(path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			newTestRouter(t).ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if response.Header().Get("X-Request-ID") == "" {
				t.Fatal("X-Request-ID is empty")
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" || response.Header().Get("X-Frame-Options") != "DENY" {
				t.Fatal("security response headers are missing")
			}
		})
	}
}

func TestUnknownRouteUsesStableErrorContract(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	newTestRouter(t).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	var payload errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "ROUTE_NOT_FOUND" || payload.RequestID == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestArtalkProxyRejectsUnsupportedMethods(t *testing.T) {
	cfg := config.Config{
		Environment:       config.Test,
		SessionCookieName: "blog_session",
		ArtalkInternalURL: "http://127.0.0.1:23366",
	}
	router, err := NewRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		Auth: auth.NewService(nil, cfg),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodConnect, http.MethodPatch, http.MethodTrace} {
		t.Run(method, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(method, "/internal/artalk/api/v2/conf", nil)
			router.ServeHTTP(response, request)
			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusMethodNotAllowed, response.Body.String())
			}
		})
	}
}

func TestArtalkProxyRequiresBlogSessionForEveryMutation(t *testing.T) {
	cfg := config.Config{
		Environment:       config.Test,
		SessionCookieName: "blog_session",
		ArtalkInternalURL: "http://127.0.0.1:23366",
	}
	router, err := NewRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		Auth: auth.NewService(nil, cfg),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/internal/artalk/api/v2/comments"},
		{method: http.MethodPost, path: "/internal/artalk/api/v2/votes/comment/1/up"},
		{method: http.MethodPut, path: "/internal/artalk/api/v2/comments/1"},
		{method: http.MethodDelete, path: "/internal/artalk/api/v2/comments/1"},
	} {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, nil)
			router.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusUnauthorized, response.Body.String())
			}
		})
	}
}

func TestArtalkProxyRequiresBlogSessionForAuthenticatedReads(t *testing.T) {
	cfg := config.Config{
		Environment:       config.Test,
		SessionCookieName: "blog_session",
		ArtalkInternalURL: "http://127.0.0.1:23366",
	}
	router, err := NewRouter(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), Dependencies{
		Auth: auth.NewService(nil, cfg),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/internal/artalk/api/v2/notifies", nil)
	request.Header.Set("Authorization", "Bearer stale-artalk-token")
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, http.StatusUnauthorized, response.Body.String())
	}
}

func TestRecoveryUsesStableErrorContract(t *testing.T) {
	router := newTestRouter(t)
	router.GET("/api/test-panic", func(_ *gin.Context) {
		panic("test panic")
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/test-panic", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", response.Code)
	}
	var payload errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Code != "INTERNAL_ERROR" || payload.RequestID == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
