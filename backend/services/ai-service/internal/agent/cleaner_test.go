package agent

import (
	"bytes"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncBuffer is a mutex-guarded io.Writer wrapping bytes.Buffer. The
// scheduler's background goroutine writes log lines to it while the test
// goroutine polls its contents concurrently — a raw bytes.Buffer would race
// under `go test -race`, so both Write and String take the same lock.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// TestSessionCleaner_StartRecoversPanicAndLogs drives cleanup() on a
// SessionCleaner whose pool field is nil. pool.Exec calls (*pgxpool.Pool).
// Acquire, which dereferences the nil receiver's acquireTracer field and
// panics with a nil pointer dereference before ever reaching a network call
// — confirmed against this module's pinned github.com/jackc/pgx/v5 v5.8.0 —
// so this is deterministic and DB-free. WithInitialDelay(0) inside Start
// makes the first (panicking) pass fire immediately, so this test never
// waits on the real 1-hour cadence.
func TestSessionCleaner_StartRecoversPanicAndLogs(t *testing.T) {
	buf := &syncBuffer{}
	prevDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	defer slog.SetDefault(prevDefault)

	c := &SessionCleaner{}
	c.Start()
	defer c.Stop()

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), "session cleaner 發生 panic") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected panic log line within 200ms, got: %q", buf.String())
}
