package grpc

import (
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/pricebook"
)

func TestNewHealthController_UsesGeneratedPricebookServiceName(t *testing.T) {
	controller := newHealthController()
	if controller == nil {
		t.Fatal("newHealthController returned nil")
	}

	got := controller.ServiceNames()
	want := pb.PricebookService_ServiceDesc.ServiceName
	if len(got) != 1 || got[0] != want {
		t.Fatalf("health service names = %v, want [%s]", got, want)
	}
}
