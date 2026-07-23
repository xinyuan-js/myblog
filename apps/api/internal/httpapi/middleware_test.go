package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryLogDoesNotExposePanicValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	router := gin.New()
	router.Use(requestIDMiddleware(), recoveryMiddleware(logger))
	router.GET("/panic", func(*gin.Context) { panic("secret-value-that-must-not-be-logged") })

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if strings.Contains(output.String(), "secret-value-that-must-not-be-logged") {
		t.Fatal("panic value was written to the log")
	}
	if !strings.Contains(output.String(), `"panicType":"string"`) {
		t.Fatalf("panic type missing from log: %s", output.String())
	}
}
