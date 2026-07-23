package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDecodeJSONBodyBoundaries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		body       string
		maximum    int64
		wantStatus int
		wantCode   string
	}{
		{
			name:       "oversized request",
			body:       `{"name":"` + strings.Repeat("a", 32) + `"}`,
			maximum:    16,
			wantStatus: http.StatusRequestEntityTooLarge,
			wantCode:   `"code":"REQUEST_TOO_LARGE"`,
		},
		{
			name:       "unknown field",
			body:       `{"name":"ok","extra":true}`,
			maximum:    64,
			wantStatus: http.StatusBadRequest,
			wantCode:   `"code":"INVALID_ARGUMENT"`,
		},
		{
			name:       "second object",
			body:       `{"name":"ok"} {"name":"again"}`,
			maximum:    64,
			wantStatus: http.StatusBadRequest,
			wantCode:   `"code":"INVALID_ARGUMENT"`,
		},
		{
			name:       "exact boundary",
			body:       `{"name":"ok"}`,
			maximum:    int64(len(`{"name":"ok"}`)),
			wantStatus: http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.POST("/", func(c *gin.Context) {
				var value struct {
					Name string `json:"name"`
				}
				if !decodeJSONBody(c, &value, test.maximum) {
					return
				}
				c.Status(http.StatusNoContent)
			})
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(test.body))
			router.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantCode != "" && !strings.Contains(response.Body.String(), test.wantCode) {
				t.Fatalf("body = %s, want %s", response.Body.String(), test.wantCode)
			}
		})
	}
}
