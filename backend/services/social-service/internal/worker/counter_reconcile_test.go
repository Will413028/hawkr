package worker

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/Will413028/hawkr/social-service/internal/domain"
)

// panicOnReconcileLikeCounts is a minimal domain.PostRepository stub whose
// only implemented method panics. Every other method is left on the nil
// embedded interface and must never be invoked by this test, because
// reconcile() calls postRepo.ReconcileLikeCounts first and nothing else runs
// once it panics.
type panicOnReconcileLikeCounts struct {
	domain.PostRepository
}

func (panicOnReconcileLikeCounts) ReconcileLikeCounts(ctx context.Context) error {
	panic("boom: simulated reconcile panic")
}

func TestCounterReconcileScheduler_RecoversPanicAndLogs(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	// commentRepo is intentionally nil: postRepo.ReconcileLikeCounts panics
	// before reconcile() ever reaches a commentRepo call. interval is short
	// (not the real 10-minute cadence) purely so this test doesn't block.
	s := newCounterReconcileScheduler(5*time.Millisecond, panicOnReconcileLikeCounts{}, nil, logger)
	s.Start(context.Background())
	defer s.Stop()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) && logs.Len() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	s.Stop()
	if logs.Len() < 1 {
		t.Fatalf("expected at least 1 error log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Message != "counter reconciler panicked" {
		t.Errorf("log message = %q, want %q", entry.Message, "counter reconciler panicked")
	}
}
