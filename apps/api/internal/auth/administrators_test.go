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
