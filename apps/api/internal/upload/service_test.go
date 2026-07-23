package upload

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

type cleanupObjectStore struct {
	deleted      []string
	deleteErrors map[string]error
}

func (*cleanupObjectStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *cleanupObjectStore) Delete(_ context.Context, key string) error {
	s.deleted = append(s.deleted, key)
	return s.deleteErrors[key]
}

func TestAcceptedImageRequiresMatchingExtension(t *testing.T) {
	tests := []struct {
		mime, extension, format string
		ok                      bool
	}{
		{"image/jpeg", ".jpg", "jpeg", true}, {"image/jpeg", ".jpeg", "jpeg", true},
		{"image/png", ".png", "png", true}, {"image/webp", ".webp", "webp", true},
		{"image/gif", ".gif", "gif", true}, {"image/png", ".jpg", "", false}, {"image/svg+xml", ".svg", "", false},
	}
	for _, tt := range tests {
		format, _, _, ok := acceptedImage(tt.mime, tt.extension)
		if ok != tt.ok || format != tt.format {
			t.Errorf("acceptedImage(%q,%q) = %q,%v", tt.mime, tt.extension, format, ok)
		}
	}
}

func TestEscapeLikeEscapesWildcards(t *testing.T) {
	if actual := escapeLike(`50%_off\today`); actual != `50\%\_off\\today` {
		t.Fatalf("escapeLike = %q", actual)
	}
}

func TestValidImageDimensionsLimitsDecodedMemory(t *testing.T) {
	for _, test := range []struct {
		width, height int
		want          bool
	}{
		{width: 7680, height: 4320, want: true},
		{width: 10000, height: 4000, want: true},
		{width: 12000, height: 12000, want: false},
		{width: 20000, height: 1, want: false},
		{width: 0, height: 100, want: false},
	} {
		if got := validImageDimensions(test.width, test.height); got != test.want {
			t.Fatalf("validImageDimensions(%d, %d) = %v, want %v", test.width, test.height, got, test.want)
		}
	}
}

func TestTrashListIncludesInterruptedPermanentDeletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := &Service{db: db}
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM uploads u WHERE u.status IN`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	trashedAt := time.Now().UTC().Add(-time.Hour)
	mock.ExpectQuery(`SELECT u.id,u.public_url`).
		WithArgs(20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_url", "original_filename", "content_type", "size", "width", "height",
			"status", "created_at", "trashed_at", "storage_key", "usage_count",
		}).AddRow(
			7, "/uploads/a.webp", "a.webp", "image/webp", 10, 1, 1,
			"deleting", trashedAt.Add(-time.Hour), trashedAt, "a.webp", 0,
		))
	mock.ExpectQuery(`SELECT ur.upload_id`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"upload_id", "resource_type", "resource_id", "field_name", "label"}))
	page, err := service.List(context.Background(), ListQuery{Page: 1, PageSize: 20, Status: "trashed", Usage: "all"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Status != "deleting" {
		t.Fatalf("trash page = %+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupPendingClaimsStateBeforeDeletingObject(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	objects := &cleanupObjectStore{}
	service := &Service{db: db, objects: objects}
	olderThan := time.Now().UTC().Add(-time.Hour)
	mock.ExpectQuery(`SELECT id,storage_key FROM uploads`).
		WithArgs(olderThan, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "storage_key"}).AddRow(9, "pending.webp"))
	mock.ExpectExec(`UPDATE uploads SET status='deleting'`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM uploads WHERE id=\? AND status='deleting'`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := service.CleanupPending(context.Background(), olderThan, 10); err != nil {
		t.Fatal(err)
	}
	if len(objects.deleted) != 1 || objects.deleted[0] != "pending.webp" {
		t.Fatalf("deleted objects = %v", objects.deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupPendingContinuesAfterOneObjectFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	deleteFailure := errors.New("object unavailable")
	objects := &cleanupObjectStore{deleteErrors: map[string]error{"broken.webp": deleteFailure}}
	service := &Service{db: db, objects: objects}
	olderThan := time.Now().UTC().Add(-time.Hour)
	mock.ExpectQuery(`SELECT id,storage_key FROM uploads`).
		WithArgs(olderThan, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "storage_key"}).
			AddRow(9, "broken.webp").
			AddRow(10, "healthy.webp"))
	mock.ExpectExec(`UPDATE uploads SET status='deleting'`).
		WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE uploads SET status='deleting'`).
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM uploads WHERE id=\? AND status='deleting'`).
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = service.CleanupPending(context.Background(), olderThan, 10)
	if !errors.Is(err, deleteFailure) {
		t.Fatalf("CleanupPending() error = %v", err)
	}
	if len(objects.deleted) != 2 || objects.deleted[0] != "broken.webp" || objects.deleted[1] != "healthy.webp" {
		t.Fatalf("deleted objects = %v", objects.deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupTrashedContinuesAfterOneObjectFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	deleteFailure := errors.New("object unavailable")
	objects := &cleanupObjectStore{deleteErrors: map[string]error{"broken.webp": deleteFailure}}
	service := &Service{db: db, objects: objects}
	olderThan := time.Now().UTC().Add(-30 * 24 * time.Hour)
	mock.ExpectQuery(`SELECT id FROM uploads WHERE status IN`).
		WithArgs(olderThan, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT storage_key,status`).
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"storage_key", "status", "refs"}).
			AddRow("broken.webp", "trashed", 0))
	mock.ExpectExec(`UPDATE uploads SET status='deleting'`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`UPDATE uploads SET status='trashed'`).
		WithArgs(int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT storage_key,status`).
		WithArgs(int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"storage_key", "status", "refs"}).
			AddRow("healthy.webp", "trashed", 0))
	mock.ExpectExec(`UPDATE uploads SET status='deleting'`).
		WithArgs(int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`DELETE FROM uploads WHERE id=\? AND status='deleting'`).
		WithArgs(int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = service.CleanupTrashed(context.Background(), olderThan, 10)
	if !errors.Is(err, deleteFailure) {
		t.Fatalf("CleanupTrashed() error = %v", err)
	}
	if len(objects.deleted) != 2 || objects.deleted[0] != "broken.webp" || objects.deleted[1] != "healthy.webp" {
		t.Fatalf("deleted objects = %v", objects.deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
