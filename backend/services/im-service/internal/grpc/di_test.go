package grpc

import (
	"context"
	"reflect"
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/go-pkg/serviceauth"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const diTestStreamKey = "01234567890123456789012345678901"

type diTestServerStream struct {
	ctx context.Context
}

func (s *diTestServerStream) SetHeader(metadata.MD) error  { return nil }
func (s *diTestServerStream) SendHeader(metadata.MD) error { return nil }
func (s *diTestServerStream) SetTrailer(metadata.MD)       {}
func (s *diTestServerStream) Context() context.Context     { return s.ctx }
func (s *diTestServerStream) SendMsg(interface{}) error    { return nil }
func (s *diTestServerStream) RecvMsg(interface{}) error    { return nil }

func TestNewGRPCServerConfig_EnablesTypedStreamAuth(t *testing.T) {
	cfg := newGRPCServerConfig(&config.Config{GRPCPort: "9933", ChatStreamJWTSecret: diTestStreamKey})
	if !cfg.EnableStreamInterceptors {
		t.Fatal("IM ChatStream must enable shared stream interceptors")
	}
	if len(cfg.ExtraStreamInterceptors) != 1 {
		t.Fatalf("extra stream interceptors = %d, want one method-specific auth interceptor", len(cfg.ExtraStreamInterceptors))
	}

	signer, err := serviceauth.NewSigner(serviceauth.Config{
		CurrentKey: diTestStreamKey,
		Issuer:     "gateway",
		Audience:   "im-service",
		Purpose:    "im-chat-stream",
	})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	raw, err := signer.Sign()
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+raw))
	called := false
	handler := func(_ interface{}, _ grpc.ServerStream) error {
		called = true
		return nil
	}
	ss := &diTestServerStream{ctx: ctx}
	if err := cfg.ExtraStreamInterceptors[0](nil, ss, &grpc.StreamServerInfo{FullMethod: serviceauth.ChatStreamMethod}, handler); err != nil {
		t.Fatalf("valid stream token rejected: %v", err)
	}
	if !called {
		t.Fatal("valid stream token did not reach handler")
	}
}

func TestNewGRPCServerConfig_UsesFinalHealthContract(t *testing.T) {
	cfg := newGRPCServerConfig(&config.Config{GRPCPort: "9933", ChatStreamJWTSecret: diTestStreamKey})

	if cfg.GRPCPort != "9933" {
		t.Fatalf("GRPCPort = %q, want 9933", cfg.GRPCPort)
	}
	if cfg.HTTPPort != "" {
		t.Fatalf("HTTPPort = %q, want empty in final IM server config", cfg.HTTPPort)
	}
	if want := []string{pb.IMService_ServiceDesc.ServiceName}; !reflect.DeepEqual(cfg.HealthServiceNames, want) {
		t.Fatalf("HealthServiceNames = %v, want %v", cfg.HealthServiceNames, want)
	}
}

func TestNewHealthController_UsesGeneratedIMServiceName(t *testing.T) {
	health := newHealthController()
	want := []string{pb.IMService_ServiceDesc.ServiceName}
	if got := health.ServiceNames(); !reflect.DeepEqual(got, want) {
		t.Fatalf("health service names = %v, want %v", got, want)
	}
}

func TestNewGRPCServerConfig_MissingKeyFailsClosedOnlyForChatStream(t *testing.T) {
	cfg := newGRPCServerConfig(&config.Config{GRPCPort: "9933"})
	if len(cfg.ExtraStreamInterceptors) != 1 {
		t.Fatalf("extra stream interceptors = %d, want one fail-closed interceptor", len(cfg.ExtraStreamInterceptors))
	}
	handler := func(_ interface{}, _ grpc.ServerStream) error { return nil }
	err := cfg.ExtraStreamInterceptors[0](nil, &diTestServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{FullMethod: serviceauth.ChatStreamMethod}, handler)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing key ChatStream code = %s, want %s", status.Code(err), codes.Unauthenticated)
	}

	called := false
	err = cfg.ExtraStreamInterceptors[0](nil, &diTestServerStream{ctx: context.Background()}, &grpc.StreamServerInfo{FullMethod: "/hawkr.im.IMService/Other"}, func(_ interface{}, _ grpc.ServerStream) error {
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("non-chat stream should pass through, err=%v called=%v", err, called)
	}
}
