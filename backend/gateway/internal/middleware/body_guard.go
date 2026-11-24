package middleware

import (
	"net/http"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

const (
	defaultMaxBodyBytes = 2 << 20 // 2MiB — general JSON API routes

	defaultReadDeadline = 30 * time.Second // general routes: small JSON bodies, no reason to need longer
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
		// Known, accepted limitation: for chunked/unknown Content-Length
		// requests, an oversized body is only caught here at read time, and
		// proxy.CreateProxy (the reverse-proxy handler)
		// will likely surface that as a copy error rather than a clean 413.
		// Bounded memory still holds either way; only status-code cleanliness
		// differs, and only for this rare chunked-transfer case.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, defaultMaxBodyBytes)

		// Bound slow-body attacks uniformly — but only
		// for requests that actually HAVE a body. Calling
		// ResponseController.SetReadDeadline on a bodyless request (GET, or
		// any ContentLength==0 request) trips a still-open Go stdlib bug
		// (golang/go#70834): the server misreads "no body to read" as a read
		// timeout, cancels that request's context, and — on a reused
		// keep-alive connection — that canceled state then contaminates
		// every subsequent request on the SAME connection. This was issue
		// #1624's real root cause: Traefik keeps long-lived keep-alive
		// connections to Gateway, so once one request on a connection trips
		// this, every unrelated request that happens to land on that same
		// connection afterward sees an already-canceled context, forever,
		// until the connection is torn down. Skipping the deadline here is
		// safe: a request with no body has nothing left to read that could
		// hang. ContentLength==-1 (chunked) still gets the deadline — there
		// IS a body there, just of unknown size.
		// http.NewResponseController never returns nil — no nil check needed.
		if c.Request.ContentLength != 0 {
			_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Now().Add(defaultReadDeadline))
		}
		c.Next()
	}
}
