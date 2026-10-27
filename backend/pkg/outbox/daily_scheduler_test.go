package outbox

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNextOccurrence_TargetLaterToday_ReturnsToday(t *testing.T) {
	now := time.Date(2026, 8, 15, 10, 0, 0, 0, time.UTC)
	got := NextOccurrence(now, 19, 5)
	want := time.Date(2026, 8, 15, 19, 5, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("NextOccurrence(10:00, target 19:05) = %v, want %v", got, want)
	}
}

func TestNextOccurrence_TargetAlreadyPassedToday_ReturnsTomorrow(t *testing.T) {
	now := time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC)
	got := NextOccurrence(now, 19, 5)
	want := time.Date(2026, 8, 16, 19, 5, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("NextOccurrence(20:00, target 19:05) = %v, want %v", got, want)
	}
}

func TestNextOccurrence_ExactlyAtTarget_ReturnsTomorrow(t *testing.T) {
	// now == target instant: !next.After(now) is true (they're equal, not
	// after), so this must roll to tomorrow — matches all three pre-existing
	// hand-rolled implementations this type replaces (buyer-service,
	// seller-service, pricebook-service all use the identical !next.After(now)
	// check), not a behavior this consolidation is free to redefine.
	now := time.Date(2026, 8, 15, 19, 5, 0, 0, time.UTC)
	got := NextOccurrence(now, 19, 5)
	want := time.Date(2026, 8, 16, 19, 5, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("NextOccurrence(exactly at target) = %v, want tomorrow %v", got, want)
	}
}

func TestNextOccurrence_MonthBoundary_RollsCorrectly(t *testing.T) {
	now := time.Date(2026, 1, 31, 23, 0, 0, 0, time.UTC)
	got := NextOccurrence(now, 19, 0)
	want := time.Date(2026, 2, 1, 19, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("NextOccurrence across month boundary = %v, want %v", got, want)
	}
}

func TestDailyScheduler_TickCallsRun(t *testing.T) {
	var calls int32
	s := NewDailyScheduler(19, 0, "test_job", func(_ context.Context) {
		atomic.AddInt32(&calls, 1)
	}, zap.NewNop())

	s.tick(context.Background())

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("calls = %d after one tick, want 1", got)
	}
}

func TestDailyScheduler_PanicInRunIsRecoveredNotPropagated(t *testing.T) {
	s := NewDailyScheduler(19, 0, "test_job", func(_ context.Context) {
		panic("boom")
	}, zap.NewNop())

	// tick() must not let the panic escape — a bare call (no recover here in
	// the test itself) proves DailyScheduler's own recover boundary works,
	// same proof style as pkg/outbox's TestScheduler_PanicInRunIsRecoveredAndReported.
	s.tick(context.Background())
}

func TestDailyScheduler_PanicInOneTickDoesNotPreventNextTick(t *testing.T) {
	var calls int32
	s := NewDailyScheduler(19, 0, "test_job", func(_ context.Context) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			panic("boom on first tick")
		}
	}, zap.NewNop())

	s.tick(context.Background())
	s.tick(context.Background())

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("calls = %d after two ticks (first panics), want 2 — a panic in one tick must not prevent the next", got)
	}
}

func TestDailyScheduler_StopBeforeTargetPreventsRun(t *testing.T) {
	// hourUTC/minuteUTC set far enough in the future (tomorrow relative to any
	// reasonable test-run wall-clock instant is not guaranteed by a fixed
	// clock value alone without control of "now" — instead this derives the
	// target from the real current time plus a safety margin, so the target is
	// always well outside this test's short real-time window regardless of
	// when it runs. This does not test genuine cancellation-race behavior
	// (that would require waiting until the target actually arrives, which is
	// the same up-to-24h limitation the three pre-existing hand-rolled
	// implementations already have and never had a unit test for either) — it
	// tests that Start() does not call run before Stop() and does not call it
	// spuriously either.
	future := time.Now().UTC().Add(2 * time.Hour)
	var calls int32
	s := NewDailyScheduler(future.Hour(), future.Minute(), "test_job", func(_ context.Context) {
		atomic.AddInt32(&calls, 1)
	}, zap.NewNop())

	s.Start(context.Background())
	time.Sleep(20 * time.Millisecond)
	s.Stop()
	time.Sleep(20 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("calls = %d, want 0 — target is 2 hours away, run must not fire before Stop", got)
	}
}

// TestDailyScheduler_ConcurrentAsyncStartAndStop_NoDataRace reproduces
// pricebook-service's forecast job call pattern: a slow startup step runs in
// its own goroutine before Start is ever called, while Stop can fire
// concurrently from a different goroutine (fx's OnStop) — e.g.
// `go func() { catchUp(ctx); scheduler.Start(ctx) }()` racing an OnStop that
// calls `scheduler.Stop()` before the goroutine reaches Start. Before the
// mutex added to Start/Stop, `go test -race` reliably caught a write/read
// race on the unexported cancel field under this exact pattern. This test
// exists to catch a regression if that locking is ever removed, not to
// assert any particular outcome ordering (either "Stop before Start" or
// "Start before Stop" are valid orderings here — the local repro just proves
// neither ordering, run many times under -race, ever produces a race
// detector failure).
func TestDailyScheduler_ConcurrentAsyncStartAndStop_NoDataRace(t *testing.T) {
	for i := 0; i < 50; i++ {
		s := NewDailyScheduler(23, 59, "race_repro", func(_ context.Context) {}, zap.NewNop())

		startCalled := make(chan struct{})
		go func() {
			// Simulate a slow startup step (pricebook's catchUpIfNeeded) that
			// delays the real Start() call, exactly like the production call
			// site — no synchronization with Stop below is intentional, that
			// absence is the scenario under test.
			time.Sleep(time.Millisecond)
			s.Start(context.Background())
			close(startCalled)
		}()

		s.Stop() // races the goroutine above by design
		<-startCalled
		s.Stop() // idempotent cleanup so the loop goroutine above doesn't leak past this test
	}
}
