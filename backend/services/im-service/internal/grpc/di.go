package grpc

import (
	"context"

	pkgconfig "github.com/Will413028/hawkr/go-pkg/config"
	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/go-pkg/serviceauth"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newGRPCServerConfig adapts im-service's own *config.Config (flat, no Server
// sub-struct) into the package-agnostic pkgserver.ServerConfig that
// pkgserver.NewGRPCServer needs — see pkg/server/grpc.go for the shared
// interceptor chain this replaces. ChatStream is the first IM streaming RPC,
// so the stream chain is enabled and the method-specific service credential
// interceptor is appended after the shared recovery/error chain.
func newGRPCServerConfig(cfg *config.Config) pkgserver.ServerConfig {
	return pkgserver.ServerConfig{
		ServerConfig: pkgconfig.ServerConfig{
			GRPCPort: cfg.GRPCPort,
		},
		Env:                      cfg.Env,
		HealthServiceNames:       []string{pb.IMService_ServiceDesc.ServiceName},
		EnableStreamInterceptors: true,
		ExtraStreamInterceptors:  []grpc.StreamServerInterceptor{newChatStreamAuthInterceptor(cfg)},
	}
}

func newHealthController() *pkgserver.HealthController {
	return pkgserver.NewHealthController(pb.IMService_ServiceDesc.ServiceName)
}

func markIMReady(lc fx.Lifecycle, health *pkgserver.HealthController, _ *grpc.Server, _ *IMGrpcServer) {
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			health.MarkReady()
			return nil
		},
		OnStop: func(context.Context) error {
			health.MarkNotReady()
			return nil
		},
	})
}

func newChatStreamAuthInterceptor(cfg *config.Config) grpc.StreamServerInterceptor {
	verifier, err := serviceauth.NewVerifier(serviceauth.Config{
		CurrentKey:  cfg.ChatStreamJWTSecret,
		PreviousKey: cfg.ChatStreamJWTSecretPrev,
		Issuer:      "gateway",
		Audience:    "im-service",
		Purpose:     "im-chat-stream",
	})
	if err == nil {
		return verifier.StreamServerInterceptor(serviceauth.ChatStreamMethod)
	}

	// A missing/invalid key must not make the migration-only legacy WS listener
	// unusable, but it must fail closed for the new gRPC stream. Keep the error
	// out of the wire and avoid an unauthenticated fallback.
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if info.FullMethod != serviceauth.ChatStreamMethod {
			return handler(srv, ss)
		}
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}
}

// registerIMService runs as an fx.Invoke during fx.New()'s graph
// construction — strictly before app.Start() fires pkgserver.NewGRPCServer's
// OnStart hook (which calls server.Serve), so registration always completes
// before the listener accepts a connection.
func registerIMService(s *grpc.Server, handler *IMGrpcServer) {
	pb.RegisterIMServiceServer(s, handler)
}

// Module provides the gRPC layer dependencies.
var Module = fx.Module("grpc",
	fx.Provide(NewIMGrpcServer),
	fx.Provide(newGRPCServerConfig),
	fx.Provide(newHealthController),
	fx.Provide(pkgserver.NewGRPCServer),
	fx.Invoke(registerIMService),
	fx.Invoke(markIMReady),
)
