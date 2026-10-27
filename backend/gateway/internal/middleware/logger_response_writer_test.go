package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponseBodyWriter_CapsCaptureAtScanLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	rbw := &responseBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
	rbw.Header().Set("Content-Type", "application/json")

	big := bytes.Repeat([]byte("y"), scanLimit+5000)
	n, err := rbw.Write(big)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if n != len(big) {
		t.Fatalf("Write must report the full length forwarded to the real ResponseWriter, got %d want %d", n, len(big))
	}
	if rbw.body.Len() != scanLimit+1 {
		t.Fatalf("captured buffer must cap at scanLimit+1 (%d), got %d", scanLimit+1, rbw.body.Len())
	}
	if rec.Body.Len() != len(big) {
		t.Fatalf("the real response body must not be truncated, got %d want %d", rec.Body.Len(), len(big))
	}
}

func TestResponseBodyWriter_SkipsCaptureForStreams(t *testing.T) {
	for _, ct := range []string{"text/event-stream", "application/connect+json", "application/connect+proto"} {
		t.Run(ct, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			rbw := &responseBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
			rbw.Header().Set("Content-Type", ct)

			if _, err := rbw.Write([]byte("event: ping\ndata: {}\n\n")); err != nil {
				t.Fatalf("write failed: %v", err)
			}
			if rbw.body.Len() != 0 {
				t.Fatalf("streaming response must never be captured, got %d bytes buffered", rbw.body.Len())
			}
		})
	}
}

func TestResponseBodyWriter_Unwrap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	rbw := &responseBodyWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}

	var uw interface{ Unwrap() http.ResponseWriter } = rbw
	if uw.Unwrap() != c.Writer {
		t.Fatal("Unwrap must return the embedded gin.ResponseWriter so http.ResponseController can reach the real connection")
	}
}
