package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	reviewpb "github.com/Will413028/hawkr/go-pkg/proto/review"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/review-service/internal/config"
	"github.com/Will413028/hawkr/review-service/internal/infrastructure"
	"github.com/Will413028/hawkr/review-service/internal/usecase"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestReviewBootstrap_UsesGRPCHealthAndReleasesPort(t *testing.T) {
	grpcPort := freeReviewTCPPort(t)
	httpPort := freeReviewTCPPort(t)
	t.Setenv("HTTP_PORT", httpPort)

	app, serverRef, healthRef := newReviewBootstrapApp(t, &config.Config{
		Env:    "development",
		Server: config.ServerConfig{GRPCPort: grpcPort},
	})

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("start review-service: %v", err)
	}

	server := *serverRef
	if _, ok := server.GetServiceInfo()[reviewpb.ReviewService_ServiceDesc.ServiceName]; !ok {
		t.Fatalf("registered gRPC services = %v, want %s", server.GetServiceInfo(), reviewpb.ReviewService_ServiceDesc.ServiceName)
	}

	conn := dialReviewHealth(t, grpcPort)
	defer conn.Close()
	healthClient := healthpb.NewHealthClient(conn)
	assertReviewHealth(t, healthClient, "", healthpb.HealthCheckResponse_SERVING)
	assertReviewHealth(t, healthClient, reviewpb.ReviewService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)

	unknownCtx, unknownCancel := context.WithTimeout(context.Background(), time.Second)
	defer unknownCancel()
	if _, err := healthClient.Check(unknownCtx, &healthpb.HealthCheckRequest{Service: "hawkr.unknown.Service"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown health service code = %s, want %s", status.Code(err), codes.NotFound)
	}

	if err := assertReviewPortClosed(httpPort); err != nil {
		t.Fatal(err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("stop review-service: %v", err)
	}
	if err := waitReviewPortClosed(grpcPort); err != nil {
		t.Fatal(err)
	}
	if got := (*healthRef).ServiceNames(); len(got) != 1 || got[0] != reviewpb.ReviewService_ServiceDesc.ServiceName {
		t.Fatalf("health controller service names after shutdown = %v", got)
	}
}

func TestReviewBootstrap_RejectsOccupiedGRPCPort(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied gRPC port: %v", err)
	}
	defer occupied.Close()

	grpcPort := fmt.Sprintf("%d", occupied.Addr().(*net.TCPAddr).Port)
	app, _, _ := newReviewBootstrapApp(t, &config.Config{
		Env:    "development",
		Server: config.ServerConfig{GRPCPort: grpcPort},
	})
	startCtx, startCancel := context.WithTimeout(context.Background(), time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err == nil {
		t.Fatal("review-service start returned nil for an occupied gRPC port")
	}
}

func newReviewBootstrapApp(t *testing.T, cfg *config.Config) (*fx.App, **grpclib.Server, **pkgserver.HealthController) {
	t.Helper()
	var server *grpclib.Server
	var health *pkgserver.HealthController
	app := fx.New(
		fx.NopLogger,
		fx.Supply(cfg),
		fx.Supply(&usecase.ReviewUseCase{}),
		fx.Supply(zap.NewNop()),
		fx.Replace(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})),
		infrastructure.Module,
		Module,
		fx.Populate(&server, &health),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("review bootstrap FX graph: %v", err)
	}
	return app, &server, &health
}

func freeReviewTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free TCP port: %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func dialReviewHealth(t *testing.T, port string) *grpclib.ClientConn {
	t.Helper()
	conn, err := grpclib.NewClient("passthrough:///127.0.0.1:"+port,
		grpclib.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial review gRPC health: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := healthpb.NewHealthClient(conn)
	for ctx.Err() == nil {
		if _, err := client.Check(ctx, &healthpb.HealthCheckRequest{}); err == nil {
			return conn
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = conn.Close()
	t.Fatalf("review gRPC health did not become reachable: %v", ctx.Err())
	return nil
}

func assertReviewHealth(t *testing.T, client healthpb.HealthClient, service string, want healthpb.HealthCheckResponse_ServingStatus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
	if err != nil {
		t.Fatalf("health Check(%q): %v", service, err)
	}
	if got := response.GetStatus(); got != want {
		t.Fatalf("health status for %q = %s, want %s", service, got, want)
	}
}

func assertReviewPortClosed(port string) error {
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return fmt.Errorf("legacy HTTP port %s is listening in gRPC-only mode", port)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func waitReviewPortClosed(port string) error {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 50*time.Millisecond)
		if err != nil {
			return nil
		}
		_ = conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("gRPC port %s remained open after shutdown", port)
}
