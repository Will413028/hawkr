package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
	"github.com/Will413028/hawkr/seller-service/internal/config"
	"github.com/Will413028/hawkr/seller-service/internal/infrastructure"
	"go.uber.org/fx"
	"go.uber.org/zap"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestSellerBootstrap_UsesGRPCHealthAndReleasesPort(t *testing.T) {
	port := freeSellerTCPPort(t)
	app, serverRef := newSellerBootstrapApp(t, &config.Config{
		Env:    "development",
		Server: config.ServerConfig{GRPCPort: port},
	})

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("seller gRPC bootstrap start: %v", err)
	}

	server := *serverRef
	if _, ok := server.GetServiceInfo()[pb.SellerService_ServiceDesc.ServiceName]; !ok {
		t.Fatalf("registered gRPC services = %v, want %s", server.GetServiceInfo(), pb.SellerService_ServiceDesc.ServiceName)
	}

	conn := dialSellerHealth(t, port)
	defer conn.Close()
	client := healthpb.NewHealthClient(conn)

	assertSellerHealth(t, client, "", healthpb.HealthCheckResponse_SERVING)
	assertSellerHealth(t, client, pb.SellerService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	unknownCtx, unknownCancel := context.WithTimeout(context.Background(), time.Second)
	defer unknownCancel()
	if _, err := client.Check(unknownCtx, &healthpb.HealthCheckRequest{Service: "hawkr.unknown.Service"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown health service code = %s, want %s", status.Code(err), codes.NotFound)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("seller gRPC bootstrap stop: %v", err)
	}
	if err := waitSellerPortClosed(port); err != nil {
		t.Fatal(err)
	}
}

func TestSellerBootstrap_RejectsOccupiedGRPCPort(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied gRPC port: %v", err)
	}
	defer occupied.Close()

	port := fmt.Sprintf("%d", occupied.Addr().(*net.TCPAddr).Port)
	app, _ := newSellerBootstrapApp(t, &config.Config{
		Env:    "development",
		Server: config.ServerConfig{GRPCPort: port},
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := app.Start(ctx); err == nil {
		t.Fatal("seller gRPC bootstrap succeeded on an occupied port")
	}
}

func TestSellerBootstrap_DoesNotOpenLegacyHTTPPort(t *testing.T) {
	grpcPort := freeSellerTCPPort(t)
	httpPort := freeSellerTCPPort(t)
	loaded := loadSellerConfigForLifecycleTest(t, grpcPort, httpPort)
	app, _ := newSellerBootstrapApp(t, loaded)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := app.Start(ctx); err != nil {
		t.Fatalf("seller gRPC-only bootstrap start: %v", err)
	}
	if err := assertSellerPortClosed(httpPort); err != nil {
		t.Fatal(err)
	}
	if err := app.Stop(ctx); err != nil {
		t.Fatalf("seller gRPC-only bootstrap stop: %v", err)
	}
}

func loadSellerConfigForLifecycleTest(t *testing.T, grpcPort, httpPort string) *config.Config {
	t.Helper()
	t.Setenv("ENV", "development")
	t.Setenv("GRPC_PORT", grpcPort)
	t.Setenv("HTTP_PORT", httpPort)
	t.Setenv("PAYUNI_AGENT_HASH_KEY", "12345678901234567890123456789012")
	t.Setenv("PAYUNI_AGENT_HASH_IV", "1234567890123456")
	t.Setenv("ENCRYPTION_MASTER_KEY", "seller-test-master-key")
	t.Setenv("TGOS_BASE_URL", "https://tgos.example.test")
	t.Setenv("TGOS_QUERYADDR_APPID", "test-appid")
	t.Setenv("TGOS_QUERYADDR_APIKEY", "test-apikey")
	t.Setenv("TGOS_PROXY_CALLER_KEY_FILE", "/tmp/tgos-caller-key.json")
	t.Setenv("MEDIA_BASE_URL", "https://media.example.test")

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load seller config: %v", err)
	}
	return loaded
}

func newSellerBootstrapApp(t *testing.T, cfg *config.Config) (*fx.App, **grpc.Server) {
	t.Helper()
	var server *grpc.Server
	app := fx.New(
		fx.NopLogger,
		fx.Supply(cfg),
		fx.Supply(&SellerServer{}),
		fx.Supply(zap.NewNop()),
		infrastructure.Module,
		fx.Provide(newHealthController),
		fx.Invoke(registerSellerService),
		fx.Invoke(markSellerReady),
		fx.Populate(&server),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("test bootstrap FX graph: %v", err)
	}
	return app, &server
}

func freeSellerTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free TCP port: %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func dialSellerHealth(t *testing.T, port string) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///127.0.0.1:"+port, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial seller gRPC health: %v", err)
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
	t.Fatalf("seller gRPC health did not become reachable: %v", ctx.Err())
	return nil
}

func assertSellerHealth(t *testing.T, client healthpb.HealthClient, service string, want healthpb.HealthCheckResponse_ServingStatus) {
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

func assertSellerPortClosed(port string) error {
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+port, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			return fmt.Errorf("legacy HTTP port %s is listening in gRPC-only mode", port)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func waitSellerPortClosed(port string) error {
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
