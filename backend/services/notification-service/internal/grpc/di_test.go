package grpc

import (
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/Will413028/hawkr/notification-service/internal/config"
)

func TestNewGRPCServerConfig_FinalRuntimeHasNoHTTPPort(t *testing.T) {
	got := newGRPCServerConfig(&config.Config{
		Env: "dev",
		Server: config.ServerConfig{
			GRPCPort: "10037",
		},
	})

	if got.GRPCPort != "10037" {
		t.Fatalf("GRPCPort = %q, want 10037", got.GRPCPort)
	}
	if got.HTTPPort != "" {
		t.Fatalf("HTTPPort = %q, want empty in final gRPC-only runtime", got.HTTPPort)
	}
	if got.Env != "dev" {
		t.Fatalf("Env = %q, want dev", got.Env)
	}
}

func TestNewHealthController_UsesGeneratedNotificationServiceName(t *testing.T) {
	controller := newHealthController()
	if controller == nil {
		t.Fatal("newHealthController returned nil")
	}
	if len(controller.ServiceNames()) != 1 || controller.ServiceNames()[0] != pb.NotificationService_ServiceDesc.ServiceName {
		t.Fatalf("health service names = %v, want [%s]", controller.ServiceNames(), pb.NotificationService_ServiceDesc.ServiceName)
	}
}
