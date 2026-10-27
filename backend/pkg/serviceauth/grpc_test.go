package serviceauth

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testStreamMethod = "/hawkr.im.IMService/ChatStream"

type testServerStream struct {
	ctx context.Context
}

func (s *testServerStream) SetHeader(metadata.MD) error  { return nil }
func (s *testServerStream) SendHeader(metadata.MD) error { return nil }
func (s *testServerStream) SetTrailer(metadata.MD)       {}
func (s *testServerStream) Context() context.Context     { return s.ctx }
func (s *testServerStream) SendMsg(interface{}) error    { return nil }
func (s *testServerStream) RecvMsg(interface{}) error    { return nil }

func TestStreamServerInterceptor_RequiresAndVerifiesBearerToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cfg := testAuthConfig(now)
	signer, err := NewSigner(cfg)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	verifier, err := NewVerifier(cfg)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	interceptor := verifier.StreamServerInterceptor(testStreamMethod)

	called := false
	handler := func(_ interface{}, _ grpc.ServerStream) error {
		called = true
		return nil
	}

	for _, tc := range []struct {
		name string
		ctx  context.Context
		code codes.Code
	}{
		{name: "missing metadata", ctx: context.Background(), code: codes.Unauthenticated},
		{name: "wrong bearer", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer invalid")), code: codes.Unauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			err := interceptor(nil, &testServerStream{ctx: tc.ctx}, &grpc.StreamServerInfo{FullMethod: testStreamMethod}, handler)
			if status.Code(err) != tc.code {
				t.Fatalf("code = %s, want %s (err=%v)", status.Code(err), tc.code, err)
			}
			if called {
				t.Fatal("handler ran for unauthenticated stream")
			}
		})
	}

	raw, err := signer.Sign()
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+raw))
	if err := interceptor(nil, &testServerStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: testStreamMethod}, handler); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	if !called {
		t.Fatal("handler did not run for valid token")
	}
}

func TestStreamClientInterceptor_AttachesOnlyToTargetMethod(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	signer, err := NewSigner(testAuthConfig(now))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	interceptor := signer.StreamClientInterceptor(testStreamMethod)

	streamer := func(ctx context.Context, _ *grpc.StreamDesc, _ *grpc.ClientConn, _ string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Internal, "missing outgoing metadata")
		}
		values := md.Get("authorization")
		if len(values) != 1 || len(values[0]) <= len("Bearer ") {
			return nil, status.Error(codes.Internal, "missing bearer token")
		}
		return nil, nil
	}

	if _, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, testStreamMethod, streamer); err != nil {
		t.Fatalf("target method: %v", err)
	}
	if _, err := interceptor(context.Background(), &grpc.StreamDesc{}, nil, "/hawkr.im.IMService/Other", streamer); status.Code(err) != codes.Internal {
		t.Fatalf("non-target method should preserve streamer behavior, got %v", err)
	}
}
