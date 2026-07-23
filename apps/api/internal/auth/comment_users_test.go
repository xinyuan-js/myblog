package auth

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/myblog/apps/api/internal/config"
)

func newCommentPolicyService(t *testing.T) (*Service, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Service{
		db:  db,
		cfg: config.Config{CommentDailyLimit: 20, CommentDayOffset: 8},
		now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	}, mock
}

func TestReserveCommentRejectsBlockedAccount(t *testing.T) {
	service, mock := newCommentPolicyService(t)
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT u.comments_blocked, u.comment_daily_limit, COALESCE(du.comment_count, 0)
		FROM github_users u
		LEFT JOIN comment_daily_usage du ON du.github_id = u.github_id AND du.usage_date = ?
		WHERE u.github_id = ?
	`)).WithArgs("2023-11-15", uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"comments_blocked", "comment_daily_limit", "comment_count"}).
			AddRow(true, nil, 0))

	_, err := service.ReserveComment(context.Background(), Session{User: User{GitHubID: 42}})
	if !errors.Is(err, ErrCommentsBlocked) {
		t.Fatalf("ReserveComment() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReserveCommentEnforcesAtomicDailyLimit(t *testing.T) {
	service, mock := newCommentPolicyService(t)
	mock.ExpectQuery("SELECT u.comments_blocked").
		WithArgs("2023-11-15", uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"comments_blocked", "comment_daily_limit", "comment_count"}).
			AddRow(false, 3, 2))
	mock.ExpectExec("INSERT INTO comment_daily_usage").
		WithArgs(uint64(42), "2023-11-15", 3).
		WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := service.ReserveComment(context.Background(), Session{User: User{GitHubID: 42}})
	if !errors.Is(err, ErrCommentDailyLimit) {
		t.Fatalf("ReserveComment() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReserveCommentExemptsAdministrators(t *testing.T) {
	service, mock := newCommentPolicyService(t)
	reservation, err := service.ReserveComment(context.Background(), Session{User: User{GitHubID: 42, IsAdmin: true}})
	if err != nil || reservation.Reserved {
		t.Fatalf("ReserveComment() = %+v, %v", reservation, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
