package grpc

import (
	"testing"

	reviewpb "github.com/Will413028/hawkr/go-pkg/proto/review"
)

func TestNewHealthController_UsesGeneratedReviewServiceName(t *testing.T) {
	controller := newHealthController()
	if controller == nil {
		t.Fatal("newHealthController returned nil")
	}
	if got, want := controller.ServiceNames(), []string{reviewpb.ReviewService_ServiceDesc.ServiceName}; len(got) != 1 || got[0] != want[0] {
		t.Fatalf("health service names = %v, want %v", got, want)
	}
}
