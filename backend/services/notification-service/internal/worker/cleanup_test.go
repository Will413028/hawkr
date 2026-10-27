package worker

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestCleanupScheduler_RecoversPanicAndLogs drives runCleanup with a nil
// *pgxpool.Pool. pool.Exec dereferences the pool's internal fields and
// panics with a nil pointer dereference before ever reaching the network —
// confirmed against this module's pinned github.com/jackc/pgx/v5 v5.9.2 —
// so this is deterministic and DB-free. interval/initialDelay are short
// test values, not the real 6-hour/1-minute production cadence.
func TestCleanupScheduler_RecoversPanicAndLogs(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	logger := zap.New(core)

	s := newCleanupScheduler(10*time.Second, 5*time.Millisecond, nil, logger)
	s.Start(context.Background())
	defer s.Stop()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) && logs.Len() == 0 {
		time.Sleep(5 * time.Millisecond)
	}
	if logs.Len() != 1 {
		t.Fatalf("expected 1 error log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]
	if entry.Message != "[cleanup] job panicked" {
		t.Errorf("log message = %q, want %q", entry.Message, "[cleanup] job panicked")
	}
}
