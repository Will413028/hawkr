package media

import (
	"testing"

	"github.com/Will413028/hawkr/social-service/internal/config"
)

// TestNewClient_ConnectsToConfiguredTarget is the behavior baseline for the
// "grpc.NewClient boilerplate → grpcclient.Dial" refactor in this file — see
// services/seller-service/internal/client/media/client_test.go (Task 3) for
// the full rationale; this file's NewClient already uses
// interceptor.InternalAuthClientInterceptor, which Step 3 must not change.
func TestNewClient_ConnectsToConfiguredTarget(t *testing.T) {
	cfg := &config.Config{
		Media: config.ExternalServiceConfig{Host: "media-service", Port: "9090"},
	}

	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c == nil {
		t.Fatal("NewClient() returned nil client")
	}
}
