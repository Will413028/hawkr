package middleware

import (
	"net/http"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

const (
	defaultMaxBodyBytes = 2 << 20

	defaultReadDeadline = 30 * time.Second
)

// BodyGuard bounds request body size and read duration before any handler
// or downstream middleware (Logger in particular) reads the body. Must run
// before Logger in the middleware chain (see infrastructure.newEngine).
func BodyGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > defaultMaxBodyBytes {
			response.Err(c, http.StatusRequestEntityTooLarge, http.StatusRequestEntityTooLarge, "request body too large")
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, defaultMaxBodyBytes)

		if c.Request.ContentLength != 0 {
			_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Now().Add(defaultReadDeadline))
		}
		c.Next()
	}
}
