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
	"github.com/gin-gonic/gin"
	"github.com/xinyuan-js/myblog/apps/api/internal/auth"
	"github.com/xinyuan-js/myblog/apps/api/internal/config"
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
		wantStatus int
	}{
		{name: "missing session", wantStatus: http.StatusUnauthorized},
		{name: "missing csrf", cookie: true, origin: "https://blog.example.com", wantStatus: http.StatusForbidden},
		{name: "cross site origin", cookie: true, origin: "https://evil.example", csrf: csrfToken, wantStatus: http.StatusForbidden},
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
				rows := sqlmock.NewRows([]string{"github_id", "github_login", "display_name", "avatar_url", "csrf_token_hash"}).
					AddRow(uint64(12345678), "admin", "Admin", "https://example.com/avatar.png", csrfHash[:])
				mock.ExpectQuery("SELECT github_id").WithArgs(tokenHash[:]).WillReturnRows(rows)
				mock.ExpectExec("UPDATE admin_sessions SET last_seen_at").WithArgs(tokenHash[:]).WillReturnResult(sqlmock.NewResult(0, 1))
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
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
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
