package otel

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// remoteParent 建一個「遠端、可選 sampled」的 parent context，模擬前端傳進來的 traceparent。
func remoteParent(sampled bool) context.Context {
	var flags trace.TraceFlags
	if sampled {
		flags = trace.FlagsSampled
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		SpanID:     trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		TraceFlags: flags,
		Remote:     true,
	})
	return trace.ContextWithSpanContext(context.Background(), sc)
}

func sample(t *testing.T, parent context.Context) sdktrace.SamplingResult {
	t.Helper()
	return buildSampler().ShouldSample(sdktrace.SamplingParameters{
		ParentContext: parent,
		TraceID:       trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		Name:          "GET /api/v1/foo",
	})
}

// 核心：前端未取樣（-00）傳進來時，gateway 仍須錄（否則 tag 指向 Tempo 不存在的 trace）。
func TestBuildSampler_RemoteParentNotSampled_StillRecords(t *testing.T) {
	t.Setenv("TRACING_SAMPLING", "1.0")
	if got := sample(t, remoteParent(false)).Decision; got != sdktrace.RecordAndSample {
		t.Fatalf("remote-not-sampled parent: got %v, want RecordAndSample", got)
	}
}

// 回歸守門：前端已取樣（-01）照常錄、續同一條 trace。
func TestBuildSampler_RemoteParentSampled_Records(t *testing.T) {
	t.Setenv("TRACING_SAMPLING", "1.0")
	if got := sample(t, remoteParent(true)).Decision; got != sdktrace.RecordAndSample {
		t.Fatalf("remote-sampled parent: got %v, want RecordAndSample", got)
	}
}

// 證明 override 走自家 ratio：ratio=0 時，remote-not-sampled parent 應被 Drop（而非無條件錄）。
func TestBuildSampler_RemoteParentNotSampled_RespectsRatio(t *testing.T) {
	t.Setenv("TRACING_SAMPLING", "0")
	if got := sample(t, remoteParent(false)).Decision; got != sdktrace.Drop {
		t.Fatalf("ratio=0 remote-not-sampled parent: got %v, want Drop", got)
	}
}
