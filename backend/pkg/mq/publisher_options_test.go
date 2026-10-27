package mq

import (
	"testing"
	"time"
)

// applyOptions mirrors what NewPublisher does before dialling AMQP, so we
// can test Option composition without needing a live broker.
func applyOptions(opts ...Option) publisherOpts {
	cfg := publisherOpts{confirmTimeout: defaultConfirmTimeout}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

func TestPublisherOptions_DefaultConfirmTimeout(t *testing.T) {
	cfg := applyOptions()
	if cfg.confirmTimeout != 30*time.Second {
		t.Fatalf("default confirmTimeout: got %v, want 30s", cfg.confirmTimeout)
	}
}

func TestPublisherOptions_WithConfirmTimeout(t *testing.T) {
	cfg := applyOptions(WithConfirmTimeout(5 * time.Second))
	if cfg.confirmTimeout != 5*time.Second {
		t.Fatalf("WithConfirmTimeout(5s): got %v, want 5s", cfg.confirmTimeout)
	}

	cfg = applyOptions(WithConfirmTimeout(60 * time.Second))
	if cfg.confirmTimeout != 60*time.Second {
		t.Fatalf("WithConfirmTimeout(60s): got %v, want 60s", cfg.confirmTimeout)
	}
}

func TestPublisherOptions_LastWriteWins(t *testing.T) {
	cfg := applyOptions(
		WithConfirmTimeout(5*time.Second),
		WithConfirmTimeout(60*time.Second),
	)
	if cfg.confirmTimeout != 60*time.Second {
		t.Fatalf("last-write-wins: got %v, want 60s", cfg.confirmTimeout)
	}
}
