package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Will413028/hawkr/crm-service/internal/config"
	"github.com/Will413028/hawkr/crm-service/internal/infrastructure"
	"github.com/Will413028/hawkr/crm-service/internal/usecase"
	pb "github.com/Will413028/hawkr/go-pkg/proto/crm"
	"go.uber.org/fx"
	"go.uber.org/zap"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func reserveTCPPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free TCP port: %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func crmBootstrapApp(t *testing.T, grpcPort string) *fx.App {
	t.Helper()

	return fx.New(
		fx.Supply(&config.Config{
			Env:    "development",
			Server: config.ServerConfig{GRPCPort: grpcPort},
		}),
		fx.Supply(&usecase.CrmUseCase{}),
		fx.Supply(zap.NewNop()),
		infrastructure.Module,
		Module,
	)
}

func waitForHealth(t *testing.T, client healthpb.HealthClient, service string) *healthpb.HealthCheckResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var response *healthpb.HealthCheckResponse
	var err error
	for ctx.Err() == nil {
		response, err = client.Check(ctx, &healthpb.HealthCheckRequest{Service: service})
		if err == nil {
			return response
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("health check %q: %v", service, err)
	return nil
}

func TestCrmBootstrap_HealthRegistrationAndLifecycle(t *testing.T) {
	grpcPort := reserveTCPPort(t)
	app := crmBootstrapApp(t, grpcPort)

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("start CRM service: %v", err)
	}

	conn, err := grpclib.NewClient("passthrough:///127.0.0.1:"+grpcPort,
		grpclib.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()
	healthClient := healthpb.NewHealthClient(conn)

	serviceName := pb.CrmService_ServiceDesc.ServiceName
	if got := waitForHealth(t, healthClient, "").GetStatus(); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("aggregate health = %s, want SERVING", got)
	}
	if got := waitForHealth(t, healthClient, serviceName).GetStatus(); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("CRM named health = %s, want SERVING", got)
	}

	unknownCtx, unknownCancel := context.WithTimeout(context.Background(), time.Second)
	defer unknownCancel()
	_, err = healthClient.Check(unknownCtx, &healthpb.HealthCheckRequest{Service: "hawkr.crm.UnknownService"})
	if code := status.Code(err); code != codes.NotFound {
		t.Fatalf("unknown service health code = %s, want %s (err=%v)", code, codes.NotFound, err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("stop CRM service: %v", err)
	}
	if conn, err := net.DialTimeout("tcp", "127.0.0.1:"+grpcPort, 50*time.Millisecond); err == nil {
		conn.Close()
		t.Fatalf("shutdown did not release gRPC port %s", grpcPort)
	}
}

func TestCrmBootstrap_GRPCBindFailureIsReturnedByFX(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied gRPC port: %v", err)
	}
	defer occupied.Close()

	app := crmBootstrapApp(t, fmt.Sprintf("%d", occupied.Addr().(*net.TCPAddr).Port))
	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err == nil {
		t.Fatal("CRM service start returned nil for an occupied gRPC port")
	}
}
