package interceptor

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Reproduces issue #1627: a client disconnects mid-call (tab closed, app
// backgrounded, navigated away). The incoming request context is canceled,
// and the handler returns context.Canceled almost immediately. This must be
// mapped to codes.Canceled, not codes.Internal — otherwise it's logged and
// alerted on as a genuine server error even though no service is unhealthy.
func TestErrorInterceptor_ClientCanceledContext_ReturnsCanceledNotInternal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, ctx.Err()
	}

	interceptor := ErrorInterceptor()
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}, handler)

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Canceled {
		t.Fatalf("expected codes.Canceled, got %v (message: %q)", st.Code(), st.Message())
	}
}

func TestStreamErrorInterceptor_ClientCanceledContext_ReturnsCanceledNotInternal(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stream := &fakeServerStream{ctx: ctx}
	handler := func(srv interface{}, ss grpc.ServerStream) error {
		return ss.Context().Err()
	}

	interceptor := StreamErrorInterceptor()
	err := interceptor(nil, stream, &grpc.StreamServerInfo{FullMethod: "/test.Service/Method"}, handler)

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected a gRPC status error, got %v", err)
	}
	if st.Code() != codes.Canceled {
		t.Fatalf("expected codes.Canceled, got %v (message: %q)", st.Code(), st.Message())
	}
}

type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context {
	return f.ctx
}
