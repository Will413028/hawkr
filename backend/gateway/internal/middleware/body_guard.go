package middleware

import (
	"net/http"
	"time"

	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

const (
	defaultMaxBodyBytes   = 2 << 20  // 2MiB — general JSON API routes
	uploadMaxBodyBytes    = 10 << 20 // 10MiB — image/CSV upload routes with client-side compression
	kycUploadMaxBodyBytes = 20 << 20 // 20MiB — media/secure_upload: the one upload route with no client-side
	// dimension resize (only JPEG quality re-encode), so a high-MP camera
	// photo (ID card, bank passbook) can plausibly exceed the standard
	// 10MiB tier. See hawkr_seller MediaRepository.uploadPrivateDocument
	// vs. uploadWithSession (the latter compresses via ImageProcessingService,
	// the former doesn't) — follow-up tracked to add client-side compression
	// there; this is the immediate backend-side safety margin.

	defaultReadDeadline   = 30 * time.Second  // general routes: small JSON bodies, no reason to need longer
	uploadReadDeadline    = 120 * time.Second // upload routes: 10MiB must survive slow mobile uplinks
	kycUploadReadDeadline = 240 * time.Second // 20MiB tier: deadline doubled alongside the byte cap to hold
	// the same ~0.67 Mbps minimum-bandwidth floor as uploadReadDeadline —
	// raising only the byte cap without raising the deadline would tighten
	// the effective bandwidth requirement and reintroduce the exact
	// slow-connection failure mode this tiering was built to avoid.
)

// bodyLimits pairs a route's accept-size cap with how long it's allowed to
// take to arrive — the two must be tiered together, not set independently.
// A fixed 30s read deadline applied uniformly across every route, including
// the 10MiB upload tier, would silently abort a legitimate upload before it
// finishes on a slow mobile connection: SetReadDeadline is an ABSOLUTE
// deadline for reading the entire remaining body, not a per-chunk/idle
// timeout, so 10MiB in 30s requires ~2.7 Mbps sustained — routine to miss on
// congested/rural mobile uplinks. This was caught in whole-branch review,
// not the per-task review, because judging BodyGuard's deadline in isolation
// never weighs it against the size tier it simultaneously allows.
type bodyLimits struct {
	maxBytes     int64
	readDeadline time.Duration
}

// uploadRoutes lists routes that legitimately need a larger body — and
// correspondingly longer read deadline — than the default tier, keyed by
// "METHOD fullpath" (c.FullPath()'s route template, not the resolved URL).
// New routes are covered by the default tier automatically — this map only
// needs entries for the exceptions.
var uploadRoutes = map[string]bodyLimits{
	"POST /api/v1/app/image_transformations":                 {uploadMaxBodyBytes, uploadReadDeadline},
	"PUT /api/v1/app/product_import_jobs/:import_job_id/csv": {uploadMaxBodyBytes, uploadReadDeadline},
	"PUT /api/v1/app/media/secure_upload/:id":                {kycUploadMaxBodyBytes, kycUploadReadDeadline},
}

// BodyGuard bounds request body size and read duration before any handler
// or downstream middleware (Logger in particular) reads the body. Must run
// before Logger in the middleware chain (see infrastructure.newEngine).
func BodyGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		limits := bodyLimits{defaultMaxBodyBytes, defaultReadDeadline}
		if v, ok := uploadRoutes[c.Request.Method+" "+c.FullPath()]; ok {
			limits = v
		}
		if c.Request.ContentLength > limits.maxBytes {
			response.Err(c, http.StatusRequestEntityTooLarge, http.StatusRequestEntityTooLarge, "request body too large")
			c.Abort()
			return
		}
		// Known, accepted limitation: for chunked/unknown Content-Length
		// requests, an oversized body is only caught here at read time, and
		// proxy.CreateProxy (the reverse-proxy handler for upload routes)
		// will likely surface that as a copy error rather than a clean 413.
		// Bounded memory still holds either way; only status-code cleanliness
		// differs, and only for this rare chunked-transfer case.
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limits.maxBytes)

		// Bound slow-body attacks uniformly, tiered per route above — but only
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
			_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Now().Add(limits.readDeadline))
		}
		c.Next()
	}
}
