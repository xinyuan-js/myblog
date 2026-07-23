package httpapi

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConcurrencyLimiterRejectsOnlyWhileSlotIsOccupied(t *testing.T) {
	gin.SetMode(gin.TestMode)
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce sync.Once
	router := gin.New()
	router.GET("/", newConcurrencyLimiter(1), func(c *gin.Context) {
		enteredOnce.Do(func() { close(entered) })
		<-release
		c.Status(http.StatusNoContent)
	})

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		firstDone <- response
	}()
	<-entered

	blocked := httptest.NewRecorder()
	router.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/", nil))
	if blocked.Code != http.StatusTooManyRequests || blocked.Header().Get("Retry-After") != "1" {
		t.Fatalf("blocked response = %d, headers=%v, body=%s", blocked.Code, blocked.Header(), blocked.Body.String())
	}

	close(release)
	if first := <-firstDone; first.Code != http.StatusNoContent {
		t.Fatalf("first status = %d", first.Code)
	}
	available := httptest.NewRecorder()
	router.ServeHTTP(available, httptest.NewRequest(http.MethodGet, "/", nil))
	if available.Code != http.StatusNoContent {
		t.Fatalf("available status = %d", available.Code)
	}
}
