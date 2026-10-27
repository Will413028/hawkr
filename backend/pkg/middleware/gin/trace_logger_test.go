package ginmw

import (
	"net/http"
	"net/http/httptest"
	"testing"

	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestTraceLogger_InjectsSpanContextFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	core, logs := observer.New(zapcore.InfoLevel)
	r := gin.New()
	r.Use(TraceLogger(zap.New(core)))
	r.GET("/", func(c *gin.Context) {
		pkglogger.FromContext(c.Request.Context()).Info("handled")
	})

	traceID, err := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	if err != nil {
		t.Fatalf("parse trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatalf("parse span id: %v", err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(trace.ContextWithSpanContext(req.Context(), spanContext))
	r.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(entries))
	}
	fields := entries[0].ContextMap()
	if got := fields["trace_id"]; got != traceID.String() {
		t.Errorf("trace_id = %v, want %s", got, traceID.String())
	}
	if got := fields["span_id"]; got != spanID.String() {
		t.Errorf("span_id = %v, want %s", got, spanID.String())
	}
}
