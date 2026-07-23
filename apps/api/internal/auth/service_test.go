package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/myblog/apps/api/internal/config"
)

func testService() *Service {
	return &Service{cfg: config.Config{OAuthStateSecret: strings.Repeat("s", 32)}, now: func() time.Time { return time.Unix(1_700_000_000, 0) }}
}

func TestConfiguredRejectsPlaceholderOAuthCredentials(t *testing.T) {
	base := config.Config{
		GitHubClientID: "real-client-id", GitHubClientSecret: "real-client-secret",
		GitHubAdminID: 12345678, OAuthStateSecret: strings.Repeat("s", 32),
	}
	if !(&Service{cfg: base}).Configured() {
		t.Fatal("complete OAuth configuration was rejected")
	}
	for _, placeholder := range []string{"local-placeholder", "replace-with-client-id", "change-me"} {
		cfg := base
		cfg.GitHubClientID = placeholder
		if (&Service{cfg: cfg}).Configured() {
			t.Errorf("placeholder credential %q was accepted", placeholder)
		}
	}
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
		"/posts/hello?from=login":                  "/posts/hello?from=login",
		"https://evil.example/admin":               "/",
		"//evil.example/admin":                     "/",
		"/admin/login":                             "/",
		"/admin\r\nLocation: https://evil.example": "/",
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

func TestAuthenticateDerivesAdministratorRoleFromConfiguredID(t *testing.T) {
	for _, test := range []struct {
		name      string
		storedID  uint64
		wantAdmin bool
	}{
		{name: "configured administrator", storedID: 12345678, wantAdmin: true},
		{name: "ordinary authenticated user", storedID: 87654321, wantAdmin: false},
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
			rows := sqlmock.NewRows([]string{
				"github_id", "github_login", "display_name", "email", "avatar_url", "csrf_token_hash", "refresh_last_seen",
			}).AddRow(test.storedID, "user", "User", "user@example.com", "https://example.com/avatar.png", csrfHash[:], false)
			mock.ExpectQuery("SELECT session.github_id").WithArgs(tokenHash[:]).WillReturnRows(rows)
			if test.storedID != 12345678 {
				mock.ExpectQuery("SELECT EXISTS").WithArgs(test.storedID).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			}
			session, err := service.Authenticate(context.Background(), token)
			if err != nil {
				t.Fatalf("Authenticate() error = %v", err)
			}
			if session.User.GitHubID != test.storedID || session.User.IsAdmin != test.wantAdmin ||
				session.User.IsOwner != (test.storedID == 12345678) {
				t.Fatalf("authenticated user = %+v, want ID %d admin=%v", session.User, test.storedID, test.wantAdmin)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAuthenticateRefreshesOnlyStaleLastSeenTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := &Service{db: db, cfg: config.Config{
		GitHubAdminID: 42, OAuthStateSecret: strings.Repeat("s", 32),
	}}
	token := strings.Repeat("a", 43)
	tokenHash := sha256.Sum256([]byte(token))
	csrfHash := sha256.Sum256([]byte(service.csrfToken(token)))
	rows := sqlmock.NewRows([]string{
		"github_id", "github_login", "display_name", "email", "avatar_url", "csrf_token_hash", "refresh_last_seen",
	}).AddRow(42, "user", "User", "user@example.com", "https://example.com/avatar.png", csrfHash[:], true)
	mock.ExpectQuery("SELECT session.github_id").WithArgs(tokenHash[:]).WillReturnRows(rows)
	mock.ExpectExec(`UPDATE user_sessions\s+SET last_seen_at`).
		WithArgs(tokenHash[:]).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if _, err := service.Authenticate(context.Background(), token); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateSessionStoresIdentityAndSessionAtomically(t *testing.T) {
	user := User{
		GitHubID: 42, Login: "octocat", Name: "Octo Cat",
		Email: "octocat@example.com", AvatarURL: "https://example.com/avatar.png",
	}
	for _, test := range []struct {
		name        string
		sessionErr  error
		wantFailure bool
	}{
		{name: "commit both records"},
		{name: "roll back identity when session insert fails", sessionErr: errors.New("insert failed"), wantFailure: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			now := time.Unix(1_700_000_000, 0).UTC()
			service := &Service{
				db: db,
				cfg: config.Config{
					OAuthStateSecret: strings.Repeat("s", 32),
					SessionTTL:       7 * 24 * time.Hour,
				},
				now: func() time.Time { return now },
			}
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO github_users").
				WithArgs(user.GitHubID, user.Login, user.Name, user.Email, user.AvatarURL).
				WillReturnResult(sqlmock.NewResult(0, 1))
			sessionInsert := mock.ExpectExec("INSERT INTO user_sessions").
				WithArgs(
					sqlmock.AnyArg(), sqlmock.AnyArg(), user.GitHubID, user.Login, user.Name,
					user.Email, user.AvatarURL, now.Add(service.cfg.SessionTTL), now,
				)
			if test.sessionErr != nil {
				sessionInsert.WillReturnError(test.sessionErr)
				mock.ExpectRollback()
			} else {
				sessionInsert.WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			}

			session, err := service.createSession(context.Background(), user)
			if (err != nil) != test.wantFailure {
				t.Fatalf("createSession() error = %v", err)
			}
			if !test.wantFailure && (session.User.GitHubID != user.GitHubID || session.CSRFToken == "" || session.rawToken == "") {
				t.Fatalf("session = %+v", session)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
