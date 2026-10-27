//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/database"
)

// TestNotifyRunWake_ObservableByListener verifies the low-level mechanism
// this task's own scope delivers: notifyRunWake's NOTIFY is observable by a
// listener on the SAME connection that issued LISTEN. It deliberately drives
// its own manual LISTEN rather than going through WatchRun/wake — this keeps
// the test anchored to the NOTIFY/LISTEN primitive itself, independent of
// WatchRun's own connection-acquisition and goroutine wiring, which
// TestWatchRun_ReceivesWakeOnNotify below covers end-to-end (Task 11 wired
// listenAll up to a real per-run LISTEN; see run_watch.go).
//
// LISTEN must be confirmed established (require.NoError) before NOTIFY is
// sent — this is the query-to-subscribe race doc57 §2.5-4 warns about;
// Task 11's real WatchRun/listenAll wiring needs the same ordering (see that
// function's doc comment on why listenAll must run before the notification
// read-loop goroutine starts).
func TestNotifyRunWake_ObservableByListener(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(ctx, "LISTEN "+runWakeChannel(42))
	require.NoError(t, err)

	repo.notifyRunWake(42)

	notifyCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	notification, err := conn.Conn().WaitForNotification(notifyCtx)
	require.NoError(t, err)
	require.Equal(t, runWakeChannel(42), notification.Channel)
}

// TestWatchRun_ConnectionLifecycle covers WatchRun's connection lifecycle:
// it acquires exactly one pool connection and starts a background goroutine
// on it, and cancel() releases that connection cleanly (no leak) and closes
// wake. runID is arbitrary here — this test only cares about the connection
// bookkeeping, not which channel got LISTENed to (that's
// TestWatchRun_ReceivesWakeOnNotify below).
func TestWatchRun_ConnectionLifecycle(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	wake, cancel, err := repo.WatchRun(ctx, 999)
	require.NoError(t, err)
	require.NotNil(t, wake)

	require.Eventually(t, func() bool {
		return pool.Stat().AcquiredConns() == 1
	}, time.Second, 10*time.Millisecond, "expected WatchRun to hold exactly one acquired connection")

	cancel()

	require.Eventually(t, func() bool {
		return pool.Stat().AcquiredConns() == 0
	}, time.Second, 10*time.Millisecond, "expected cancel() to release WatchRun's connection")

	select {
	case _, ok := <-wake:
		require.False(t, ok, "expected wake channel to be closed after cancel()")
	case <-time.After(2 * time.Second):
		t.Fatal("wake channel was not closed after cancel()")
	}
}

// TestWatchRun_ReceivesWakeOnNotify is the direct regression test for the
// race Task 5's implementer found and Task 11 had to avoid re-introducing:
// listenAll now issues a real per-run LISTEN, and it must complete BEFORE
// the notification read-loop goroutine starts reading off the same
// *pgx.Conn — otherwise pgx tears the connection down ("conn closed") the
// moment a real LISTEN and a concurrent WaitForNotification race on it. This
// test drives the whole WatchRun→listenAll→notifyRunWake→wake path together,
// so it would fail (or flake/hang) if that ordering ever regressed.
func TestWatchRun_ReceivesWakeOnNotify(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupProcurementTestDB(ctx, t)
	defer cleanup()
	repo := New(pool, database.NewTxManager(pool), testIDGen())

	const runID = int64(4242)
	wake, cancel, err := repo.WatchRun(ctx, runID)
	require.NoError(t, err)
	defer cancel()

	// LISTEN is confirmed established by WatchRun returning without error
	// (listenAll's conn.Exec already completed synchronously — see
	// run_watch.go's ordering guarantee) before notifyRunWake fires.
	repo.notifyRunWake(runID)

	select {
	case gotRunID, ok := <-wake:
		require.True(t, ok, "wake channel closed unexpectedly")
		require.Equal(t, runID, gotRunID)
	case <-time.After(3 * time.Second):
		t.Fatal("did not receive wake within timeout")
	}
}
