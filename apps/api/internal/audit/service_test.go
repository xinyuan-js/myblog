package audit

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRecordAndListAdministratorAuditEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := NewService(db)
	record := Record{
		ActorGitHubID: 42, ActorLogin: "owner", Method: "DELETE",
		RequestPath: "/api/admin/posts/9", ResponseStatus: 204,
		RequestID: "request-id", ClientIP: "127.0.0.1",
	}
	mock.ExpectExec("INSERT INTO admin_audit_events").
		WithArgs(
			record.ActorGitHubID, record.ActorLogin, record.Method, record.RequestPath,
			record.ResponseStatus, record.RequestID, record.ClientIP, record.ResourceLocation,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	if err := service.Record(context.Background(), record); err != nil {
		t.Fatal(err)
	}

	query := Query{Page: 1, PageSize: 20, Outcome: "success", Search: `owner%`}
	pattern := `%owner\%%`
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(pattern, pattern, pattern).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	occurredAt := time.Now().UTC()
	mock.ExpectQuery("SELECT id, actor_github_id").
		WithArgs(pattern, pattern, pattern, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "actor_github_id", "actor_login", "method", "request_path",
			"response_status", "request_id", "client_ip", "resource_location", "occurred_at",
		}).AddRow(
			1, 42, "owner", "DELETE", "/api/admin/posts/9", 204,
			"request-id", "127.0.0.1", "", occurredAt,
		))
	page, err := service.List(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if page.Pagination.Total != 1 || len(page.Items) != 1 || page.Items[0].ActorGitHubID != 42 {
		t.Fatalf("page = %+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
