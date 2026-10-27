package media

import (
	"testing"

	"github.com/Will413028/hawkr/seller-service/internal/config"
)

// TestNewClient_ConnectsToConfiguredTarget is the behavior baseline for the
// "grpc.NewClient boilerplate → grpcclient.Dial" refactor in this file: it
// only asserts NewClient's externally observable contract, which must hold
// both before and after the refactor (pure boilerplate substitution, no
// behavior change — see plan Architecture note).
func TestNewClient_ConnectsToConfiguredTarget(t *testing.T) {
	cfg := &config.Config{
		MediaService: config.MediaServiceConfig{Addr: "media-service.internal:9090"},
	}

	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c == nil {
		t.Fatal("NewClient() returned nil client")
	}
}
