package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	"github.com/Will413028/hawkr/ai-service/internal/infrastructure"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	pkgserver "github.com/Will413028/hawkr/go-pkg/server"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

func reserveAITCPPort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free TCP port: %v", err)
	}
	defer listener.Close()
	return fmt.Sprintf("%d", listener.Addr().(*net.TCPAddr).Port)
}

func aiBootstrapApp(t *testing.T, grpcPort string) (*fx.App, **grpc.Server) {
	t.Helper()

	var server *grpc.Server
	app := fx.New(
		fx.Supply(&config.Config{
			Env:    "development",
			Server: config.ServerConfig{GRPCPort: grpcPort},
		}),
		fx.Supply(zap.NewNop()),
		fx.Supply(&AIGrpcServer{}, &AIAssistantGrpcServer{}, &AIProcurementGrpcServer{}),
		infrastructure.Module,
		fx.Provide(newGRPCServerConfig),
		fx.Provide(newHealthController),
		fx.Provide(pkgserver.NewGRPCServer),
		fx.Invoke(registerAIImageService),
		fx.Invoke(registerAIAdminService),
		fx.Invoke(registerRecommendationService),
		fx.Invoke(registerAIAssistantService),
		fx.Invoke(registerAIProcurementService),
		fx.Invoke(markAIReady),
		fx.Populate(&server),
	)
	return app, &server
}

func waitForAIHealth(t *testing.T, client healthpb.HealthClient, service string) *healthpb.HealthCheckResponse {
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

func TestAIBootstrap_HealthRegistrationAndLifecycle(t *testing.T) {
	grpcPort := reserveAITCPPort(t)
	app, serverRef := aiBootstrapApp(t, grpcPort)

	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err != nil {
		t.Fatalf("start ai-service: %v", err)
	}

	server := *serverRef
	for _, serviceName := range []string{
		pb.AIImageService_ServiceDesc.ServiceName,
		pb.AIAdminService_ServiceDesc.ServiceName,
		pb.RecommendationService_ServiceDesc.ServiceName,
		pb.AIAssistantService_ServiceDesc.ServiceName,
		pb.AIProcurementService_ServiceDesc.ServiceName,
	} {
		if _, ok := server.GetServiceInfo()[serviceName]; !ok {
			t.Fatalf("registered gRPC services = %v, want %s", server.GetServiceInfo(), serviceName)
		}
	}

	conn, err := grpc.NewClient("passthrough:///127.0.0.1:"+grpcPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	defer conn.Close()
	healthClient := healthpb.NewHealthClient(conn)

	if got := waitForAIHealth(t, healthClient, "").GetStatus(); got != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("aggregate health = %s, want SERVING", got)
	}
	for _, serviceName := range []string{
		pb.AIImageService_ServiceDesc.ServiceName,
		pb.AIAdminService_ServiceDesc.ServiceName,
		pb.RecommendationService_ServiceDesc.ServiceName,
		pb.AIAssistantService_ServiceDesc.ServiceName,
		pb.AIProcurementService_ServiceDesc.ServiceName,
	} {
		if got := waitForAIHealth(t, healthClient, serviceName).GetStatus(); got != healthpb.HealthCheckResponse_SERVING {
			t.Fatalf("named health %q = %s, want SERVING", serviceName, got)
		}
	}

	unknownCtx, unknownCancel := context.WithTimeout(context.Background(), time.Second)
	defer unknownCancel()
	_, err = healthClient.Check(unknownCtx, &healthpb.HealthCheckRequest{Service: "hawkr.ai.UnknownService"})
	if code := status.Code(err); code != codes.NotFound {
		t.Fatalf("unknown service health code = %s, want %s (err=%v)", code, codes.NotFound, err)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer stopCancel()
	if err := app.Stop(stopCtx); err != nil {
		t.Fatalf("stop ai-service: %v", err)
	}
	if conn, err := net.DialTimeout("tcp", "127.0.0.1:"+grpcPort, 50*time.Millisecond); err == nil {
		conn.Close()
		t.Fatalf("shutdown did not release gRPC port %s", grpcPort)
	}
}

func TestAIBootstrap_GRPCBindFailureIsReturnedByFX(t *testing.T) {
	occupied, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("reserve occupied gRPC port: %v", err)
	}
	defer occupied.Close()

	app, _ := aiBootstrapApp(t, fmt.Sprintf("%d", occupied.Addr().(*net.TCPAddr).Port))
	startCtx, startCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer startCancel()
	if err := app.Start(startCtx); err == nil {
		t.Fatal("ai-service start returned nil for an occupied gRPC port")
	}
}
