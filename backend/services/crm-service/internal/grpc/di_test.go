package grpc

import (
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/crm"
)

func TestNewHealthController_UsesGeneratedCrmServiceName(t *testing.T) {
	controller := newHealthController()
	if controller == nil {
		t.Fatal("newHealthController returned nil")
	}

	names := controller.ServiceNames()
	if len(names) != 1 || names[0] != pb.CrmService_ServiceDesc.ServiceName {
		t.Fatalf("health service names = %v, want [%s]", names, pb.CrmService_ServiceDesc.ServiceName)
	}
}
