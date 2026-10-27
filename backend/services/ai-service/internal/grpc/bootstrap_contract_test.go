package grpc

import (
	"sort"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/config"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

func TestNewGRPCServerConfig_FinalRuntimePreservesAIContract(t *testing.T) {
	got := newGRPCServerConfig(&config.Config{
		Env: "staging",
		Server: config.ServerConfig{
			GRPCPort: "19939",
		},
	})

	if got.GRPCPort != "19939" {
		t.Fatalf("GRPCPort = %q, want 19939", got.GRPCPort)
	}
	if got.HTTPPort != "" {
		t.Fatalf("HTTPPort = %q, want empty in final gRPC-only runtime", got.HTTPPort)
	}
	if got.Env != "staging" {
		t.Fatalf("Env = %q, want staging", got.Env)
	}
	if got.MaxRecvMsgSize != grpcMaxMsgSize || got.MaxSendMsgSize != grpcMaxMsgSize {
		t.Fatalf("message limits = recv:%d send:%d, want recv/send:%d", got.MaxRecvMsgSize, got.MaxSendMsgSize, grpcMaxMsgSize)
	}
	if !got.EnableStreamInterceptors {
		t.Fatal("EnableStreamInterceptors = false, want true for AI streaming RPCs")
	}
	if len(got.ExtraUnaryInterceptors) != 1 || len(got.ExtraStreamInterceptors) != 1 {
		t.Fatalf("method-policy interceptors = unary:%d stream:%d, want one each", len(got.ExtraUnaryInterceptors), len(got.ExtraStreamInterceptors))
	}

	wantHealth := []string{
		pb.AIImageService_ServiceDesc.ServiceName,
		pb.AIAdminService_ServiceDesc.ServiceName,
		pb.RecommendationService_ServiceDesc.ServiceName,
		pb.AIAssistantService_ServiceDesc.ServiceName,
		pb.AIProcurementService_ServiceDesc.ServiceName,
	}
	if len(got.HealthServiceNames) != len(wantHealth) {
		t.Fatalf("health service names = %v, want %v", got.HealthServiceNames, wantHealth)
	}
	for i, want := range wantHealth {
		if got.HealthServiceNames[i] != want {
			t.Fatalf("health service name %d = %q, want %q", i, got.HealthServiceNames[i], want)
		}
	}
}

func TestNewHealthController_UsesAllGeneratedAIServiceNames(t *testing.T) {
	controller := newHealthController()
	if controller == nil {
		t.Fatal("newHealthController returned nil")
	}

	want := []string{
		pb.AIImageService_ServiceDesc.ServiceName,
		pb.AIAdminService_ServiceDesc.ServiceName,
		pb.RecommendationService_ServiceDesc.ServiceName,
		pb.AIAssistantService_ServiceDesc.ServiceName,
		pb.AIProcurementService_ServiceDesc.ServiceName,
	}
	sort.Strings(want)
	got := controller.ServiceNames()
	if len(got) != len(want) {
		t.Fatalf("health service names = %v, want %v", got, want)
	}
	for i, wantName := range want {
		if got[i] != wantName {
			t.Fatalf("health service name %d = %q, want %q", i, got[i], wantName)
		}
	}
}
