package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/myblog/apps/api/internal/config"
)

func TestAuthorizationSupportsOwnerAndDelegatedAdministrator(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, config.Config{GitHubAdminID: 100001})

	admin, owner, err := service.authorization(context.Background(), 100001)
	if err != nil || !admin || !owner {
		t.Fatalf("owner authorization = admin:%v owner:%v err:%v", admin, owner, err)
	}

	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	admin, owner, err = service.authorization(context.Background(), 42)
	if err != nil || !admin || owner {
		t.Fatalf("delegated authorization = admin:%v owner:%v err:%v", admin, owner, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdministratorManagementProtectsConfiguredOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, config.Config{GitHubAdminID: 100001})

	if _, err := service.AddAdministrator(context.Background(), 100001, 100001); !errors.Is(err, ErrOwnerManagedByConfig) {
		t.Fatalf("AddAdministrator(owner) error = %v", err)
	}
	if err := service.RemoveAdministrator(context.Background(), 100001); !errors.Is(err, ErrOwnerManagedByConfig) {
		t.Fatalf("RemoveAdministrator(owner) error = %v", err)
	}

	grantedAt := time.Now().UTC()
	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec("INSERT INTO delegated_admins").WithArgs(uint64(42), uint64(100001)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT granted_at").WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"granted_at"}).AddRow(grantedAt))
	added, err := service.AddAdministrator(context.Background(), 42, 100001)
	if err != nil || added.GitHubID != 42 || added.IsOwner || added.GrantedAt == nil {
		t.Fatalf("AddAdministrator() = %+v, %v", added, err)
	}
	mock.ExpectQuery("SELECT EXISTS").WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("DELETE FROM delegated_admins").WithArgs(uint64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := service.RemoveAdministrator(context.Background(), 42); err != nil {
		t.Fatalf("RemoveAdministrator() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListAdministratorsShowsKnownAndPendingIdentities(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db, config.Config{GitHubAdminID: 100001})
	grantedAt := time.Now().UTC()

	mock.ExpectQuery("SELECT github_login, display_name, avatar_url").
		WithArgs(uint64(100001)).
		WillReturnRows(sqlmock.NewRows([]string{"github_login", "display_name", "avatar_url"}).
			AddRow("owner", "Owner", "https://example.com/owner.png"))
	mock.ExpectQuery("SELECT d.github_id, d.granted_at").
		WithArgs(uint64(100001)).
		WillReturnRows(sqlmock.NewRows([]string{
			"github_id", "granted_at", "github_login", "display_name", "avatar_url", "has_signed_in",
		}).
			AddRow(uint64(42), grantedAt, "known", "Known User", "https://example.com/known.png", true).
			AddRow(uint64(43), grantedAt, "", "", "", false))

	items, err := service.ListAdministrators(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || !items[0].IsOwner || !items[0].HasSignedIn ||
		items[1].Login != "known" || !items[1].HasSignedIn ||
		items[2].GitHubID != 43 || items[2].HasSignedIn {
		t.Fatalf("administrators = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileArtalkAdministratorsClearsStaleRoles(t *testing.T) {
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

	service := NewService(blogDB, config.Config{GitHubAdminID: 100001})
	service.SetArtalkDatabase(artalkDB)
	blogMock.ExpectQuery("SELECT email").
		WithArgs(uint64(100001)).
		WillReturnRows(sqlmock.NewRows([]string{"email", "artalk_user_id"}).
			AddRow("owner@example.com", 7).
			AddRow("admin@example.com", nil))
	artalkMock.ExpectBegin()
	artalkMock.ExpectExec("UPDATE users SET is_admin = FALSE").
		WillReturnResult(sqlmock.NewResult(0, 1))
	artalkMock.ExpectExec("UPDATE users SET is_admin = TRUE").
		WithArgs(int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	artalkMock.ExpectExec("UPDATE users SET is_admin = TRUE").
		WithArgs("admin@example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))
	artalkMock.ExpectCommit()

	if err := service.ReconcileArtalkAdministrators(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := blogMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if err := artalkMock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
