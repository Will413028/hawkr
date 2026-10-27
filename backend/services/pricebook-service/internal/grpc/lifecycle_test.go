package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
	"github.com/Will413028/hawkr/pricebook-service/internal/config"
	"github.com/Will413028/hawkr/pricebook-service/internal/infrastructure"
	"go.uber.org/fx"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestPricebookBootstrap_RejectsOccupiedGRPCPort(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied gRPC port: %v", err)
	}
	defer occupied.Close()

	port := fmt.Sprintf("%d", occupied.Addr().(*net.TCPAddr).Port)
	app := newTestBootstrapApp(t, port)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.Start(ctx); err == nil {
		t.Fatal("pricebook gRPC bootstrap succeeded on an occupied port")
	}
}

func TestPricebookBootstrap_UsesGRPCHealthAndReleasesPort(t *testing.T) {
	port := freePricebookTCPPort(t)
	app := newTestBootstrapApp(t, port)

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("pricebook gRPC bootstrap start: %v", err)
	}

	conn := dialPricebookHealth(t, port)
	defer conn.Close()
	client := healthpb.NewHealthClient(conn)

	assertPricebookHealth(t, client, "", healthpb.HealthCheckResponse_SERVING)
	assertPricebookHealth(t, client, pb.PricebookService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	if _, err := client.Check(context.Background(), &healthpb.HealthCheckRequest{Service: "hawkr.unknown.Service"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown health service code = %s, want %s", status.Code(err), codes.NotFound)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("pricebook gRPC bootstrap stop: %v", err)
	}
	if err := waitPricebookPortClosed(port); err != nil {
		t.Fatal(err)
	}
}

func TestPricebookBootstrap_IgnoresOccupiedLegacyHTTPPort(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve occupied legacy HTTP port: %v", err)
	}
	defer occupied.Close()
	t.Setenv("HTTP_PORT", fmt.Sprintf("%d", occupied.Addr().(*net.TCPAddr).Port))

	port := freePricebookTCPPort(t)
	app := newTestBootstrapApp(t, port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("gRPC-only bootstrap must ignore occupied legacy HTTP port: %v", err)
	}
	if err := app.Stop(ctx); err != nil {
		t.Fatalf("gRPC-only bootstrap stop: %v", err)
	}
}

func newTestBootstrapApp(t *testing.T, port string) *fx.App {
	t.Helper()
	app := fx.New(
		fx.NopLogger,
		fx.Supply(&config.Config{
			Env:    "development",
			Server: config.ServerConfig{GRPCPort: port},
		}),
		fx.Supply(&PricebookServer{}),
		fx.Supply(zap.NewNop()),
		infrastructure.Module,
		fx.Provide(newHealthController),
		fx.Invoke(registerPricebookService),
		fx.Invoke(markPricebookReady),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("test bootstrap FX graph: %v", err)
	}
	return app
}

func freePricebookTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free TCP port: %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func dialPricebookHealth(t *testing.T, port string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///127.0.0.1:"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial pricebook gRPC health: %v", err)
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
	conn.Close()
	t.Fatalf("pricebook gRPC health did not become reachable: %v", ctx.Err())
	return nil
}

func assertPricebookHealth(t *testing.T, client healthpb.HealthClient, service string, want healthpb.HealthCheckResponse_ServingStatus) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
	if err != nil {
		t.Fatalf("health Check(%q): %v", service, err)
	}
	if got := resp.GetStatus(); got != want {
		t.Fatalf("health status for %q = %s, want %s", service, got, want)
	}
}

func waitPricebookPortClosed(port string) error {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 50*time.Millisecond)
		if err != nil {
			return nil
		}
		conn.Close()
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("gRPC port %s remained open after shutdown", port)
}
