package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimitSeparatesUserAndProjectScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limit := RateLimit(0, 1, func(c *gin.Context) string {
		return c.GetString("rate-key")
	})

	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for _, key := range []string{"u:shared", "p:shared"} {
		key := key
		wg.Add(1)
		go func() {
			defer wg.Done()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Set("rate-key", key)
			<-start
			limit(c)
			statuses <- recorder.Code
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)
	for status := range statuses {
		if status != http.StatusOK {
			t.Fatalf("independent scope was rate limited: status=%d", status)
		}
	}
}
