package grpc

import (
	"testing"

	pb "github.com/Will413028/hawkr/go-pkg/proto/seller"
)

func TestHealthController_UsesGeneratedSellerServiceName(t *testing.T) {
	health := newHealthController()
	want := []string{pb.SellerService_ServiceDesc.ServiceName}
	if got := health.ServiceNames(); len(got) != 1 || got[0] != want[0] {
		t.Fatalf("health service names = %v, want %v", got, want)
	}
}
