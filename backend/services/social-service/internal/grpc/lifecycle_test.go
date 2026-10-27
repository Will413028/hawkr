package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/social"
	"github.com/Will413028/hawkr/social-service/internal/config"
	"github.com/Will413028/hawkr/social-service/internal/infrastructure"
	"go.uber.org/fx"
	"go.uber.org/zap"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func TestSocialBootstrap_UsesGRPCHealthAndReleasesPort(t *testing.T) {
	grpcPort := freeSocialTCPPort(t)
	app, serverRef := newSocialBootstrapApp(t, &config.Config{
		Env:    "development",
		Server: config.ServerConfig{GRPCPort: grpcPort},
	})

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("start social-service: %v", err)
	}

	server := *serverRef
	if _, ok := server.GetServiceInfo()[pb.SocialService_ServiceDesc.ServiceName]; !ok {
		t.Fatalf("registered gRPC services = %v, want %s", server.GetServiceInfo(), pb.SocialService_ServiceDesc.ServiceName)
	}

	conn := dialSocialHealth(t, grpcPort)
	defer conn.Close()
	healthClient := healthpb.NewHealthClient(conn)
	assertSocialHealth(t, healthClient, "", healthpb.HealthCheckResponse_SERVING)
	assertSocialHealth(t, healthClient, pb.SocialService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)

	unknownCtx, unknownCancel := context.WithTimeout(context.Background(), time.Second)
	defer unknownCancel()
	if _, err := healthClient.Check(unknownCtx, &healthpb.HealthCheckRequest{Service: "hawkr.unknown.Service"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown health service code = %s, want %s", status.Code(err), codes.NotFound)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("stop social-service: %v", err)
	}
	if err := waitSocialPortClosed(grpcPort); err != nil {
		t.Fatal(err)
	}
}

func TestSocialBootstrap_RejectsOccupiedGRPCPort(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied gRPC port: %v", err)
	}
	defer occupied.Close()

	grpcPort := fmt.Sprintf("%d", occupied.Addr().(*net.TCPAddr).Port)
	app, _ := newSocialBootstrapApp(t, &config.Config{
		Env:    "development",
		Server: config.ServerConfig{GRPCPort: grpcPort},
	})
	startCtx, startCancel := context.WithTimeout(context.Background(), time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err == nil {
		t.Fatal("social-service start returned nil for an occupied gRPC port")
	}
}

func TestSocialBootstrap_DoesNotOpenLegacyHTTPPort(t *testing.T) {
	grpcPort := freeSocialTCPPort(t)
	httpPort := freeSocialTCPPort(t)
	loaded := loadSocialConfigForLifecycleTest(t, grpcPort, httpPort)
	app, _ := newSocialBootstrapApp(t, loaded)

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("start gRPC-only social-service: %v", err)
	}
	if err := assertSocialPortClosed(httpPort); err != nil {
		t.Fatal(err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("stop gRPC-only social-service: %v", err)
	}
}

func newSocialBootstrapApp(t *testing.T, cfg *config.Config) (*fx.App, **grpclib.Server) {
	t.Helper()
	var server *grpclib.Server
	app := fx.New(
		fx.NopLogger,
		fx.Supply(cfg),
		fx.Supply(&ContentServer{}),
		fx.Supply(zap.NewNop()),
		infrastructure.Module,
		fx.Provide(newHealthController),
		fx.Invoke(registerSocialService),
		fx.Invoke(markSocialReady),
		fx.Populate(&server),
	)
	if err := app.Err(); err != nil {
		t.Fatalf("social bootstrap FX graph: %v", err)
	}
	return app, &server
}

func loadSocialConfigForLifecycleTest(t *testing.T, grpcPort, httpPort string) *config.Config {
	t.Helper()
	t.Setenv("APP_ENV", "development")
	t.Setenv("GRPC_PORT", grpcPort)
	t.Setenv("HTTP_PORT", httpPort)
	t.Setenv("MEDIA_BASE_URL", "https://media.example.test")

	loaded, err := config.Load()
	if err != nil {
		t.Fatalf("load social config: %v", err)
	}
	return loaded
}

func freeSocialTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free TCP port: %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func dialSocialHealth(t *testing.T, port string) *grpclib.ClientConn {
	t.Helper()
	conn, err := grpclib.NewClient("passthrough:///127.0.0.1:"+port,
		grpclib.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial social gRPC health: %v", err)
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
	t.Fatalf("social gRPC health did not become reachable: %v", ctx.Err())
	return nil
}

func assertSocialHealth(t *testing.T, client healthpb.HealthClient, service string, want healthpb.HealthCheckResponse_ServingStatus) {
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

func assertSocialPortClosed(port string) error {
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

func waitSocialPortClosed(port string) error {
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
