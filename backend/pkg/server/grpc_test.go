package server

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/config"
	"github.com/Will413028/hawkr/go-pkg/middleware/grpc/interceptor"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestBuildServerOptions_StreamInterceptorsToggle(t *testing.T) {
	logger := zap.NewNop()

	off := BuildServerOptions(ServerConfig{EnableStreamInterceptors: false}, logger)
	if len(off) != 4 {
		t.Fatalf("EnableStreamInterceptors=false: got %d grpc.ServerOption, want 4 (StatsHandler + ChainUnaryInterceptor + KeepaliveEnforcementPolicy + KeepaliveParams)", len(off))
	}

	on := BuildServerOptions(ServerConfig{EnableStreamInterceptors: true}, logger)
	if len(on) != 5 {
		t.Fatalf("EnableStreamInterceptors=true: got %d grpc.ServerOption, want 5 (+ ChainStreamInterceptor)", len(on))
	}
}

// TestBuildServerOptions_MaxMsgSize verifies MaxRecvMsgSize/MaxSendMsgSize only
// append a grpc.ServerOption when non-zero — the zero value must not add a
// spurious 0-byte limit (that would break every RPC).
func TestBuildServerOptions_MaxMsgSize(t *testing.T) {
	logger := zap.NewNop()

	base := BuildServerOptions(ServerConfig{}, logger)
	if len(base) != 4 {
		t.Fatalf("zero MaxRecvMsgSize/MaxSendMsgSize: got %d grpc.ServerOption, want 4 (no size options appended)", len(base))
	}

	withSizes := BuildServerOptions(ServerConfig{MaxRecvMsgSize: 10 << 20, MaxSendMsgSize: 10 << 20}, logger)
	if len(withSizes) != 6 {
		t.Fatalf("non-zero MaxRecvMsgSize/MaxSendMsgSize: got %d grpc.ServerOption, want 6 (+2 size options)", len(withSizes))
	}
}

// TestBuildServerOptions_ExtraInterceptorsRunAfterStandardChain verifies
// ExtraUnaryInterceptors/ExtraStreamInterceptors are appended after the
// standard chain (they run last, wrapping the innermost handler), matching
// the position ai-service's method-policy interceptors held before migrating
// to pkg/server.
func TestBuildServerOptions_ExtraInterceptorsRunAfterStandardChain(t *testing.T) {
	logger := zap.NewNop()
	var ran bool
	extra := func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ran = true
		return handler(ctx, req)
	}

	opts := BuildServerOptions(ServerConfig{ExtraUnaryInterceptors: []grpc.UnaryServerInterceptor{extra}}, logger)
	if len(opts) != 4 {
		t.Fatalf("got %d grpc.ServerOption, want 4 (extra interceptors join the existing ChainUnaryInterceptor, not a new option)", len(opts))
	}

	server := grpc.NewServer(opts...)
	defer server.Stop()
	if ran {
		t.Fatalf("extra interceptor ran before any RPC — BuildServerOptions must not invoke it eagerly")
	}
}

// TestBuildServerOptions_LoggingOptionsThreadThrough verifies LoggingOptions
// (e.g. WithExpectedCodes) reach interceptor.LoggingInterceptor — this is what
// lets the identity module migrate to pkg/server.NewGRPCServer without losing its
// issue #2072 expected-code handling.
func TestBuildServerOptions_LoggingOptionsThreadThrough(t *testing.T) {
	logger := zap.NewNop()

	withoutOpt := BuildServerOptions(ServerConfig{}, logger)
	withOpt := BuildServerOptions(ServerConfig{
		LoggingOptions: []interceptor.LoggingOption{
			interceptor.WithExpectedCodes(map[string][]codes.Code{
				"/hawkr.account.AccountService/GetWebSession": {codes.NotFound},
			}),
		},
	}, logger)

	if len(withoutOpt) != len(withOpt) {
		t.Fatalf("LoggingOptions must not change the grpc.ServerOption count: got %d without, %d with", len(withoutOpt), len(withOpt))
	}
}

func freePort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to reserve a free port: %v", err)
	}
	port := fmt.Sprintf("%d", lis.Addr().(*net.TCPAddr).Port)
	lis.Close()
	return port
}

func TestSetHealthStatus_TracksAggregateAndConfiguredServices(t *testing.T) {
	const serviceName = "hawkr.review.ReviewService"
	healthServer := health.NewServer()

	setHealthStatus(healthServer, []string{serviceName, ""}, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	for _, service := range []string{"", serviceName} {
		resp, err := healthServer.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: service})
		if err != nil {
			t.Fatalf("health Check for %q: %v", service, err)
		}
		if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_NOT_SERVING {
			t.Fatalf("health status for %q = %s, want NOT_SERVING", service, resp.GetStatus())
		}
	}

	setHealthStatus(healthServer, []string{serviceName, ""}, grpc_health_v1.HealthCheckResponse_SERVING)
	for _, service := range []string{"", serviceName} {
		resp, err := healthServer.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: service})
		if err != nil {
			t.Fatalf("health Check for %q after transition: %v", service, err)
		}
		if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
			t.Fatalf("health status for %q after transition = %s, want SERVING", service, resp.GetStatus())
		}
	}
}

func TestNewGRPCServer_LifecycleHooksBindAndReleasePort(t *testing.T) {
	port := freePort(t)
	lc := fxtest.NewLifecycle(t)

	NewGRPCServer(GRPCServerParams{
		Lifecycle: lc,
		Config:    ServerConfig{ServerConfig: config.ServerConfig{GRPCPort: port}},
		Logger:    zap.NewNop(),
	})

	lc.RequireStart()

	dialed := false
	for i := 0; i < 10; i++ {
		conn, err := net.DialTimeout("tcp", ":"+port, 50*time.Millisecond)
		if err == nil {
			dialed = true
			conn.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !dialed {
		t.Fatalf("OnStart did not bind :%s within the retry window", port)
	}

	lc.RequireStop()

	if _, err := net.DialTimeout("tcp", ":"+port, 50*time.Millisecond); err == nil {
		t.Fatalf("OnStop (GracefulStop) did not release :%s", port)
	}
}

func TestNewGRPCServer_ReportsHealthAfterBind(t *testing.T) {
	port := freePort(t)
	lc := fxtest.NewLifecycle(t)

	NewGRPCServer(GRPCServerParams{
		Lifecycle: lc,
		Config:    ServerConfig{ServerConfig: config.ServerConfig{GRPCPort: port}},
		Logger:    zap.NewNop(),
	})
	lc.RequireStart()

	conn, err := grpc.NewClient("passthrough:///127.0.0.1:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var resp *grpc_health_v1.HealthCheckResponse
	for ctx.Err() == nil {
		resp, err = client.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health Check: %v", err)
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("health status = %s, want SERVING", resp.GetStatus())
	}

	lc.RequireStop()
}

func TestNewGRPCServer_ReportsConfiguredServiceHealthAfterBind(t *testing.T) {
	const serviceName = "hawkr.review.ReviewService"
	port := freePort(t)
	lc := fxtest.NewLifecycle(t)

	NewGRPCServer(GRPCServerParams{
		Lifecycle: lc,
		Config: ServerConfig{
			ServerConfig:       config.ServerConfig{GRPCPort: port},
			HealthServiceNames: []string{serviceName},
		},
		Logger: zap.NewNop(),
	})
	lc.RequireStart()

	conn, err := grpc.NewClient("passthrough:///127.0.0.1:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()

	client := grpc_health_v1.NewHealthClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := client.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: serviceName})
	if err != nil {
		t.Fatalf("service-specific health Check: %v", err)
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		t.Fatalf("service-specific health status = %s, want SERVING", resp.GetStatus())
	}
	_, err = client.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: "hawkr.unconfigured.Service"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unconfigured service-specific health error code = %s, want %s", status.Code(err), codes.NotFound)
	}

	lc.RequireStop()
}

func TestNewGRPCServer_PropagatesBindError(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied TCP port: %v", err)
	}
	defer occupied.Close()

	port := strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port)
	lc := fxtest.NewLifecycle(t)
	NewGRPCServer(GRPCServerParams{
		Lifecycle: lc,
		Config: ServerConfig{
			ServerConfig:       config.ServerConfig{GRPCPort: port},
			HealthServiceNames: []string{"hawkr.review.ReviewService"},
		},
		Logger: zap.NewNop(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := lc.Start(ctx); err == nil {
		t.Fatal("gRPC lifecycle start succeeded on an occupied port")
	}
}
