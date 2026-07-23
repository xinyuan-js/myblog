package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWritePublicDataSupportsStrongAndWeakConditionalRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	serve := func(validator string) *httptest.ResponseRecorder {
		t.Helper()
		router := gin.New()
		router.GET("/api/site", func(c *gin.Context) {
			if err := writePublicData(c, gin.H{"title": "xinyuan"}, "public, max-age=60"); err != nil {
				t.Error(err)
			}
		})
		request := httptest.NewRequest(http.MethodGet, "/api/site", nil)
		if validator != "" {
			request.Header.Set("If-None-Match", validator)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}
	first := serve("")
	if first.Code != http.StatusOK || first.Header().Get("ETag") == "" {
		t.Fatalf("first response = %d headers=%v body=%s", first.Code, first.Header(), first.Body.String())
	}
	var payload struct {
		Data struct {
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &payload); err != nil || payload.Data.Title != "xinyuan" {
		t.Fatalf("payload=%+v err=%v", payload, err)
	}

	for _, validator := range []string{
		first.Header().Get("ETag"),
		`"unrelated", ` + first.Header().Get("ETag"),
		`W/` + first.Header().Get("ETag"),
		"*",
	} {
		response := serve(validator)
		if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
			t.Fatalf("validator %q response=%d body=%q", validator, response.Code, response.Body.String())
		}
		if response.Header().Get("ETag") != first.Header().Get("ETag") ||
			response.Header().Get("Cache-Control") != "public, max-age=60" {
			t.Fatalf("304 headers=%v", response.Header())
		}
	}
}

func TestErrorResponsesAreNeverCached(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/posts/future", nil)
	writeError(context, http.StatusNotFound, "POST_NOT_FOUND", "文章不存在")

	if response.Code != http.StatusNotFound || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("response=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}
