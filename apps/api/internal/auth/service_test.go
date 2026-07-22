package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/xinyuan-js/myblog/apps/api/internal/config"
)

func testService() *Service {
	return &Service{cfg: config.Config{OAuthStateSecret: strings.Repeat("s", 32)}, now: func() time.Time { return time.Unix(1_700_000_000, 0) }}
}

func TestSignedOAuthStateRejectsTampering(t *testing.T) {
	service := testService()
	value, err := service.signState(oauthState{Nonce: "nonce", ReturnTo: "/admin", ExpiresAt: 1_700_000_600})
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.verifyState(value)
	if err != nil || state.ReturnTo != "/admin" {
		t.Fatalf("verify state = %+v, %v", state, err)
	}
	value = "x" + value[1:]
	if _, err := service.verifyState(value); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("tampered state error = %v", err)
	}
}

func TestSanitizeReturnToPreventsOpenRedirect(t *testing.T) {
	tests := map[string]string{
		"/admin/posts/1/edit":                      "/admin/posts/1/edit",
		"https://evil.example/admin":               "/admin",
		"//evil.example/admin":                     "/admin",
		"/admin/login":                             "/admin",
		"/admin\r\nLocation: https://evil.example": "/admin",
	}
	for input, expected := range tests {
		if actual := sanitizeReturnTo(input); actual != expected {
			t.Errorf("sanitizeReturnTo(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestCSRFTokenIsStableAndBoundToSession(t *testing.T) {
	service := testService()
	first := service.csrfToken("session-one")
	if first == "" || first != service.csrfToken("session-one") {
		t.Fatal("csrf token is not stable")
	}
	if first == service.csrfToken("session-two") {
		t.Fatal("csrf token is not session-bound")
	}
}

func TestParseSessionCookieRejectsSeparators(t *testing.T) {
	if _, ok := parseSessionCookie(strings.Repeat("a", 43)); !ok {
		t.Fatal("valid cookie rejected")
	}
	for _, value := range []string{"short", strings.Repeat("a", 40) + ".x", strings.Repeat("a", 40) + "/x"} {
		if _, ok := parseSessionCookie(value); ok {
			t.Errorf("invalid cookie accepted: %q", value)
		}
	}
}

func TestAuthenticateRechecksConfiguredAdministratorID(t *testing.T) {
	for _, test := range []struct {
		name           string
		storedID       uint64
		wantErr        error
		expectLastSeen bool
		expectDelete   bool
	}{
		{name: "configured administrator", storedID: 12345678, expectLastSeen: true},
		{name: "revoked administrator", storedID: 87654321, wantErr: ErrUnauthenticated, expectDelete: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			service := &Service{db: db, cfg: config.Config{
				GitHubAdminID: 12345678, OAuthStateSecret: strings.Repeat("s", 32),
			}}
			token := strings.Repeat("a", 43)
			tokenHash := sha256.Sum256([]byte(token))
			csrfHash := sha256.Sum256([]byte(service.csrfToken(token)))
			rows := sqlmock.NewRows([]string{"github_id", "github_login", "display_name", "avatar_url", "csrf_token_hash"}).
				AddRow(test.storedID, "admin", "Admin", "https://example.com/avatar.png", csrfHash[:])
			mock.ExpectQuery("SELECT github_id").WithArgs(tokenHash[:]).WillReturnRows(rows)
			if test.expectDelete {
				mock.ExpectExec("DELETE FROM admin_sessions").WithArgs(tokenHash[:]).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			if test.expectLastSeen {
				mock.ExpectExec("UPDATE admin_sessions SET last_seen_at").WithArgs(tokenHash[:]).WillReturnResult(sqlmock.NewResult(0, 1))
			}

			session, err := service.Authenticate(context.Background(), token)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Authenticate() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && session.User.GitHubID != 12345678 {
				t.Fatalf("authenticated GitHub ID = %d", session.User.GitHubID)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
