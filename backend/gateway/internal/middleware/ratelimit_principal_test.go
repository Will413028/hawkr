package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimitPrincipalFromMonolith(t *testing.T) {
	for _, tc := range []struct{ id, kind, want string }{{"7", "4", "m:7"}, {"8", "4", "m:8"}, {"7", "1", "a:7"}, {"7", "0", "a:7"}} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/admin/orders", nil)
		c.Request.RemoteAddr = "192.0.2.1:1234"
		c.Request.Header.Set(headerAccountID, tc.id)
		c.Request.Header.Set(headerAccountType, tc.kind)
		subject, kind := RateLimitSubject(c)
		if subject != tc.want || kind != SubjectVisitor {
			t.Fatalf("subject=%s kind=%s;want %s visitor", subject, kind, tc.want)
		}
	}
}
