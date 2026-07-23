package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/example/myblog/apps/api/internal/audit"
	"github.com/example/myblog/apps/api/internal/auth"
	"github.com/gin-gonic/gin"
)

func TestAuditMiddlewareRecordsAuthenticatedAdministratorMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	service := audit.NewService(db)
	mock.ExpectExec("INSERT INTO admin_audit_events").
		WithArgs(
			uint64(42), "owner", http.MethodDelete, "/api/admin/posts/9",
			http.StatusNoContent, sqlmock.AnyArg(), "192.0.2.1", "",
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	router := gin.New()
	router.Use(requestIDMiddleware())
	router.DELETE(
		"/api/admin/posts/:id",
		func(c *gin.Context) {
			c.Set(adminSessionKey, auth.Session{User: auth.User{GitHubID: 42, Login: "owner", IsAdmin: true}})
			c.Next()
		},
		auditAdminMutations(service, slog.New(slog.NewTextHandler(io.Discard, nil))),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/admin/posts/9", nil)
	router.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
