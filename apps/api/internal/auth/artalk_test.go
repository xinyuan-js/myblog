package auth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/myblog/apps/api/internal/config"
	"github.com/go-sql-driver/mysql"
)

func TestArtalkIdentityTokenIsShortLivedAndTamperProof(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	service := &Service{
		cfg: config.Config{OAuthStateSecret: strings.Repeat("s", 32)},
		now: func() time.Time { return now },
	}
	session := Session{User: User{
		GitHubID: 100001, Login: "octocat", Name: "Example User",
		Email: "OWNER@EXAMPLE.COM", IsAdmin: true,
	}}
	token, err := service.IssueArtalkToken(session)
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.VerifyArtalkToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if user.Subject != "100001" || user.Email != "owner@example.com" || !user.EmailVerified {
		t.Fatalf("userinfo = %+v", user)
	}

	tampered := "x" + token[1:]
	if _, err := service.VerifyArtalkToken(tampered); !errors.Is(err, ErrInvalidArtalkToken) {
		t.Fatalf("tampered token error = %v", err)
	}
	service.now = func() time.Time { return now.Add(artalkTokenTTL + time.Second) }
	if _, err := service.VerifyArtalkToken(token); !errors.Is(err, ErrInvalidArtalkToken) {
		t.Fatalf("expired token error = %v", err)
	}
}

func TestValidateArtalkSessionBindsTokenToBlogUser(t *testing.T) {
	for _, test := range []struct {
		name      string
		response  string
		wantError error
	}{
		{
			name:     "matching verified identity",
			response: `{"user":{"email":"OWNER@example.com"},"is_login":true}`,
		},
		{
			name:      "different Artalk user",
			response:  `{"user":{"email":"other@example.com"},"is_login":true}`,
			wantError: ErrArtalkUserMismatch,
		},
		{
			name:      "invalid Artalk session",
			response:  `{"user":null,"is_login":false}`,
			wantError: ErrArtalkSessionInvalid,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(nil, config.Config{ArtalkInternalURL: "http://artalk.test"})
			service.client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/api/v2/user" {
					t.Fatalf("path = %q", request.URL.Path)
				}
				if request.Header.Get("Authorization") != "Bearer artalk-token" {
					t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(test.response)),
					Request:    request,
				}, nil
			})}
			err := service.ValidateArtalkSession(context.Background(), "artalk-token", Session{
				User: User{Email: "owner@example.com"},
			})
			if !errors.Is(err, test.wantError) {
				t.Fatalf("error = %v, want %v", err, test.wantError)
			}
		})
	}
}

func TestPrepareArtalkIdentityKeepsMappedUserAcrossEmailChanges(t *testing.T) {
	blogDB, blogMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer blogDB.Close()
	artalkDB, artalkMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer artalkDB.Close()
	service := NewService(blogDB, config.Config{})
	service.SetArtalkDatabase(artalkDB)

	blogMock.ExpectQuery("SELECT artalk_user_id").
		WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"artalk_user_id"}).AddRow(17))
	artalkMock.ExpectQuery("SELECT EXISTS").
		WithArgs("new@example.com", uint64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	artalkMock.ExpectExec("UPDATE users").
		WithArgs("new@example.com", uint64(17)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	id, err := service.prepareArtalkIdentity(context.Background(), 42, "NEW@example.com")
	if err != nil || id != 17 {
		t.Fatalf("prepareArtalkIdentity() = %d, %v", id, err)
	}
	if err := blogMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := artalkMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateArtalkSessionRejectsIncompleteIdentityBeforeMutation(t *testing.T) {
	blogDB, blogMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer blogDB.Close()
	artalkDB, artalkMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer artalkDB.Close()
	service := NewService(blogDB, config.Config{GitHubAdminID: 42})
	service.SetArtalkDatabase(artalkDB)

	_, err = service.CreateArtalkSession(context.Background(), Session{User: User{
		GitHubID: 42, Login: "user", Name: "User",
	}})
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("CreateArtalkSession() error = %v", err)
	}
	if err := blogMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := artalkMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareArtalkIdentityRejectsEmailOwnedByAnotherArtalkUser(t *testing.T) {
	blogDB, blogMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer blogDB.Close()
	artalkDB, artalkMock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer artalkDB.Close()
	service := NewService(blogDB, config.Config{})
	service.SetArtalkDatabase(artalkDB)

	blogMock.ExpectQuery("SELECT artalk_user_id").
		WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"artalk_user_id"}).AddRow(17))
	artalkMock.ExpectQuery("SELECT EXISTS").
		WithArgs("taken@example.com", uint64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

	_, err = service.prepareArtalkIdentity(context.Background(), 42, "taken@example.com")
	if !errors.Is(err, ErrArtalkIdentityConflict) {
		t.Fatalf("prepareArtalkIdentity() error = %v", err)
	}
}

func TestBindArtalkIdentityRejectsReplacingExistingMapping(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, config.Config{})
	mock.ExpectExec("UPDATE github_users").
		WithArgs(uint64(17), uint64(42), uint64(17)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT artalk_user_id").
		WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"artalk_user_id"}).AddRow(19))

	if err := service.bindArtalkIdentity(context.Background(), 42, 17); !errors.Is(err, ErrArtalkIdentityConflict) {
		t.Fatalf("bindArtalkIdentity() error = %v", err)
	}
}

func TestBindArtalkIdentityReportsUniqueMappingConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, config.Config{})
	mock.ExpectExec("UPDATE github_users").
		WithArgs(uint64(17), uint64(42), uint64(17)).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate mapping"})

	if err := service.bindArtalkIdentity(context.Background(), 42, 17); !errors.Is(err, ErrArtalkIdentityConflict) {
		t.Fatalf("bindArtalkIdentity() error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
