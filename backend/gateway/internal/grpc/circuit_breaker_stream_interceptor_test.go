package grpc

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingClientStream struct {
	grpc.ClientStream
	recvErrs []error // 依序回傳給每次 RecvMsg 呼叫
	recvIdx  int
}

func (s *recordingClientStream) RecvMsg(m any) error {
	if s.recvIdx >= len(s.recvErrs) {
		return io.EOF
	}
	err := s.recvErrs[s.recvIdx]
	s.recvIdx++
	return err
}

func TestCircuitBreakerStreamInterceptor_OpenState_RejectsWithoutCallingStreamer(t *testing.T) {
	resetStreamBreakers()
	cfg := breakerConfig{minRequests: 1, failureRatio: 0.1, timeout: time.Minute, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerStreamInterceptorWith(cfg)
	cc := dialTarget(t, "dns:///svc-b:50051")

	failingStreamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return nil, status.Error(codes.Unavailable, "down")
	}
	// 打到超過 minRequests、failureRatio 門檻，逼 breaker open
	for i := 0; i < 3; i++ {
		_, _ = intercept(context.Background(), &grpc.StreamDesc{}, cc, "/m", failingStreamer)
	}

	called := false
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		called = true
		return &recordingClientStream{}, nil
	}
	_, err := intercept(context.Background(), &grpc.StreamDesc{}, cc, "/m", streamer)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected Unavailable when breaker open, got %v", err)
	}
	if called {
		t.Fatal("streamer should not be called while breaker is open")
	}
}

// TestCircuitBreakerStreamInterceptor_LogsStateChange proves the stream
// breaker is not silent when it trips. Without this the unary breaker's
// warning is the only one an incident responder sees, which reads as "the
// stream side is fine" — the opposite of the truth during an ai-service
// outage. kind=stream is what separates the two log lines: both breakers use
// the same target as their gobreaker Name.
func TestCircuitBreakerStreamInterceptor_LogsStateChange(t *testing.T) {
	resetStreamBreakers()
	core, logs := observer.New(zapcore.WarnLevel)
	defer zap.ReplaceGlobals(zap.New(core))()

	cfg := breakerConfig{minRequests: 1, failureRatio: 0.1, timeout: time.Minute, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerStreamInterceptorWith(cfg)
	cc := dialTarget(t, "dns:///svc-log:50051")

	failingStreamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return nil, status.Error(codes.Unavailable, "down")
	}
	for i := 0; i < 3; i++ {
		_, _ = intercept(context.Background(), &grpc.StreamDesc{}, cc, "/m", failingStreamer)
	}

	entries := logs.FilterMessage("circuit breaker state change").All()
	if len(entries) == 0 {
		t.Fatal("stream breaker tripped without logging a state change")
	}
	fields := entries[0].ContextMap()
	if fields["kind"] != "stream" {
		t.Fatalf(`kind field = %v, want "stream" (otherwise indistinguishable from the unary breaker)`, fields["kind"])
	}
	if fields["target"] != cc.Target() {
		t.Fatalf("target field = %v, want %q", fields["target"], cc.Target())
	}
	if fields["to"] != "open" {
		t.Fatalf(`to field = %v, want "open"`, fields["to"])
	}
}

func TestCircuitBreakerStreamInterceptor_RecordsSuccessOnceAtStreamEOF(t *testing.T) {
	resetStreamBreakers()
	cfg := breakerConfig{minRequests: 10, failureRatio: 0.5, timeout: time.Minute, halfOpenMax: 1, interval: time.Minute}
	intercept := circuitBreakerStreamInterceptorWith(cfg)
	cc := dialTarget(t, "dns:///svc-c:50051")

	inner := &recordingClientStream{} // 無 recvErrs 預設 → 第一次 RecvMsg 就回 io.EOF
	streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		return inner, nil
	}
	wrapped, err := intercept(context.Background(), &grpc.StreamDesc{}, cc, "/m", streamer)
	if err != nil {
		t.Fatalf("intercept: %v", err)
	}
	if err := wrapped.RecvMsg(nil); !errors.Is(err, io.EOF) {
		t.Fatalf("expected io.EOF passthrough, got %v", err)
	}
	// 再呼叫一次 RecvMsg 不應該讓 done 被呼叫第二次（wrapped stream 只在第一次遇到終止
	// 錯誤時記一次帳，第二次呼叫 RecvMsg 本身在真實 gRPC 使用情境下不會發生，但這裡驗證
	// wrapper 不會重複計數，避免未來誤用造成 breaker 數字失真）。
	_ = wrapped.RecvMsg(nil)

	cb := getStreamBreaker(cc.Target(), cfg)
	counts := cb.Counts()
	if counts.TotalSuccesses != 1 {
		t.Fatalf("want 1 recorded success, got %d (counts=%+v)", counts.TotalSuccesses, counts)
	}
}
