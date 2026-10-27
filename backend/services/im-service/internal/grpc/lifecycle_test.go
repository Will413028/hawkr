package grpc

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func ephemeralTCPPort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve ephemeral port: %v", err)
	}
	port := lis.Addr().(*net.TCPAddr).Port
	if err := lis.Close(); err != nil {
		t.Fatalf("release ephemeral port: %v", err)
	}
	return strconv.Itoa(port)
}

func reserveTCPPort(t *testing.T) (net.Listener, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve TCP port: %v", err)
	}
	return lis, strconv.Itoa(lis.Addr().(*net.TCPAddr).Port)
}

func newLifecycleApp(t *testing.T, port string) (*fx.App, **grpc.Server, **pkgserver.HealthController) {
	t.Helper()
	var server *grpc.Server
	var health *pkgserver.HealthController
	app := fx.New(
		fx.NopLogger,
		fx.Supply(&config.Config{
			Env:                 "development",
			GRPCPort:            port,
			ChatStreamJWTSecret: diTestStreamKey,
		}),
		fx.Supply(zap.NewNop()),
		fx.Provide(
			func() domain.Hub { return &stubHub{} },
			func() *usecase.MessageUseCase { return &usecase.MessageUseCase{} },
		),
		Module,
		fx.Populate(&server, &health),
	)
	return app, &server, &health
}

func checkHealth(t *testing.T, client healthpb.HealthClient, service string) healthpb.HealthCheckResponse_ServingStatus {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
	if err != nil {
		t.Fatalf("health Check(%q): %v", service, err)
	}
	return resp.GetStatus()
}

func TestIMBootstrap_ReportsHealthAndReleasesPort(t *testing.T) {
	port := ephemeralTCPPort(t)
	legacyHTTP, legacyHTTPPort := reserveTCPPort(t)
	defer legacyHTTP.Close()
	legacyWS, legacyWSPort := reserveTCPPort(t)
	defer legacyWS.Close()
	app, serverRef, healthRef := newLifecycleApp(t, port)

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("FX start: %v", err)
	}
	server := *serverRef
	if server == nil {
		t.Fatal("FX did not populate gRPC server")
	}
	if _, ok := server.GetServiceInfo()[pb.IMService_ServiceDesc.ServiceName]; !ok {
		t.Fatalf("GetServiceInfo() missing %q", pb.IMService_ServiceDesc.ServiceName)
	}
	names := (*healthRef).ServiceNames()
	if len(names) != 1 || names[0] != pb.IMService_ServiceDesc.ServiceName {
		t.Fatalf("health service names = %v, want generated IM service name", names)
	}

	conn, err := grpc.NewClient(
		"127.0.0.1:"+port,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()
	healthClient := healthpb.NewHealthClient(conn)
	if got := checkHealth(t, healthClient, ""); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("aggregate health = %s, want SERVING", got)
	}
	if got := checkHealth(t, healthClient, pb.IMService_ServiceDesc.ServiceName); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("named health = %s, want SERVING", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	_, err = healthClient.Check(ctx, &healthpb.HealthCheckRequest{Service: "hawkr.im.UnknownService"})
	cancel()
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unknown health code = %s, want %s", status.Code(err), codes.NotFound)
	}

	// Keep the historical listener ports occupied while the final app starts;
	// if a retired HTTP/WS bootstrap is still wired, FX startup must fail here.
	if err := legacyHTTP.Close(); err != nil {
		t.Fatalf("release reserved legacy HTTP port: %v", err)
	}
	probe, err := net.Listen("tcp", ":"+legacyHTTPPort)
	if err != nil {
		t.Fatalf("legacy HTTP port was rebound by IM: %v", err)
	}
	_ = probe.Close()
	if err := legacyWS.Close(); err != nil {
		t.Fatalf("release reserved legacy WS port: %v", err)
	}
	probe, err = net.Listen("tcp", ":"+legacyWSPort)
	if err != nil {
		t.Fatalf("legacy WS port was rebound by IM: %v", err)
	}
	_ = probe.Close()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("FX stop: %v", err)
	}
	probe, err = net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("gRPC port remained bound after FX stop: %v", err)
	}
	_ = probe.Close()
}

func TestIMBootstrap_ReturnsOccupiedPortError(t *testing.T) {
	port := ephemeralTCPPort(t)
	occupied, err := net.Listen("tcp", ":"+port)
	if err != nil {
		t.Fatalf("occupy port: %v", err)
	}
	defer occupied.Close()

	app, _, _ := newLifecycleApp(t, port)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := app.Start(ctx); err == nil {
		t.Fatal("FX start returned nil for an occupied gRPC port")
	}
}
