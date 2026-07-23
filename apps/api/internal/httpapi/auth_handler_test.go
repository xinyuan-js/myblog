package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/example/myblog/apps/api/internal/config"
	"github.com/gin-gonic/gin"
)

func TestFailedOAuthClearsStateAndSessionCookies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := auth.NewService(nil, config.Config{
		AppOrigin: "https://blog.example.com", SessionCookieName: "blog_session", SessionSecure: true,
	})
	router := gin.New()
	router.GET("/api/auth/github/callback", (authHandler{service: service, logger: slog.New(slog.NewTextHandler(io.Discard, nil))}).githubCallback)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/auth/github/callback?error=access_denied", nil)
	router.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusFound)
	}
	cleared := map[string]bool{}
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge < 0 {
			cleared[cookie.Name] = true
		}
	}
	if !cleared["blog_oauth_state"] || !cleared["blog_session"] {
		t.Fatalf("cleared cookies = %#v, want OAuth state and session", cleared)
	}
}

func TestLogoutOnlyClearsCookieAfterCSRFValidation(t *testing.T) {
	const token = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const secret = "ssssssssssssssssssssssssssssssss"
	tokenHash := sha256.Sum256([]byte(token))
	csrfMAC := hmac.New(sha256.New, []byte(secret))
	_, _ = csrfMAC.Write([]byte("csrf:" + token))
	csrfToken := base64.RawURLEncoding.EncodeToString(csrfMAC.Sum(nil))
	csrfHash := sha256.Sum256([]byte(csrfToken))

	for _, test := range []struct {
		name        string
		origin      string
		csrf        string
		wantStatus  int
		wantCleared bool
	}{
		{name: "cross origin cannot clear cookie", origin: "https://evil.example", csrf: csrfToken, wantStatus: http.StatusForbidden},
		{name: "missing csrf cannot clear cookie", origin: "https://blog.example.com", wantStatus: http.StatusForbidden},
		{name: "valid logout clears cookie", origin: "https://blog.example.com", csrf: csrfToken, wantStatus: http.StatusNoContent, wantCleared: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			service := auth.NewService(db, config.Config{
				Environment: config.Production, AppOrigin: "https://blog.example.com",
				SessionCookieName: "blog_session", SessionSecure: true,
				GitHubAdminID: 123, OAuthStateSecret: secret,
			})
			rows := sqlmock.NewRows([]string{
				"github_id", "github_login", "display_name", "email", "avatar_url", "csrf_token_hash", "refresh_last_seen",
			}).AddRow(123, "user", "User", "user@example.com", "https://example.com/avatar.png", csrfHash[:], false)
			mock.ExpectQuery("SELECT session.github_id").WithArgs(tokenHash[:]).WillReturnRows(rows)
			if test.wantCleared {
				mock.ExpectExec("DELETE FROM user_sessions").WithArgs(tokenHash[:]).WillReturnResult(sqlmock.NewResult(0, 1))
			}

			router := gin.New()
			router.POST("/api/auth/logout", (authHandler{
				service: service, logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
			}).logout)
			request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			request.AddCookie(&http.Cookie{Name: "blog_session", Value: token})
			request.Header.Set("Origin", test.origin)
			if test.csrf != "" {
				request.Header.Set("X-CSRF-Token", test.csrf)
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			cleared := false
			for _, cookie := range response.Result().Cookies() {
				cleared = cleared || (cookie.Name == "blog_session" && cookie.MaxAge < 0)
			}
			if cleared != test.wantCleared {
				t.Fatalf("session cookie cleared = %v, want %v", cleared, test.wantCleared)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidSocialURL(t *testing.T) {
	for _, value := range []string{"https://github.com/example", "http://example.com/feed", "mailto:hello@example.com"} {
		if !validSocialURL(value) {
			t.Errorf("validSocialURL(%q) = false", value)
		}
	}
	for _, value := range []string{"javascript:alert(1)", "data:text/html,x", "/relative", "mailto:not-an-email", "https://user:pass@example.com"} {
		if validSocialURL(value) {
			t.Errorf("validSocialURL(%q) = true", value)
		}
	}
}
