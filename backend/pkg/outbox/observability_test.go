package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeMonitorRepo struct {
	pendingCount int
	pendingErr   error

	oldestAge time.Duration
	oldestOk  bool
	oldestErr error

	deleteCutoff time.Time
	deleteLimit  int
	deleteCount  int
	deleteErr    error
}

func (f *fakeMonitorRepo) CountPending(_ context.Context) (int, error) {
	return f.pendingCount, f.pendingErr
}

func (f *fakeMonitorRepo) OldestPendingAge(_ context.Context, _ time.Time) (time.Duration, bool, error) {
	return f.oldestAge, f.oldestOk, f.oldestErr
}

func (f *fakeMonitorRepo) DeleteDoneBefore(_ context.Context, cutoff time.Time, limit int) (int, error) {
	f.deleteCutoff = cutoff
	f.deleteLimit = limit
	return f.deleteCount, f.deleteErr
}

func TestMetrics_Refresh_SetsPendingAndOldestAge(t *testing.T) {
	m := NewMetrics("test_refresh_pending")
	repo := &fakeMonitorRepo{pendingCount: 7, oldestAge: 90 * time.Second, oldestOk: true}

	if err := m.Refresh(context.Background(), repo, time.Now()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := testutil.ToFloat64(m.pending); got != 7 {
		t.Errorf("pending gauge = %v, want 7", got)
	}
	if got := testutil.ToFloat64(m.oldestAge); got != 90 {
		t.Errorf("oldestAge gauge = %v, want 90", got)
	}
}

func TestMetrics_Refresh_NoPendingRows_OldestAgeIsZero(t *testing.T) {
	m := NewMetrics("test_refresh_empty")
	repo := &fakeMonitorRepo{pendingCount: 0, oldestOk: false}

	if err := m.Refresh(context.Background(), repo, time.Now()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := testutil.ToFloat64(m.oldestAge); got != 0 {
		t.Errorf("oldestAge gauge = %v, want 0 when no pending rows", got)
	}
}

func TestMetrics_Refresh_CountPendingError_Propagates(t *testing.T) {
	m := NewMetrics("test_refresh_err")
	repo := &fakeMonitorRepo{pendingErr: errors.New("db down")}

	if err := m.Refresh(context.Background(), repo, time.Now()); err == nil {
		t.Fatal("expected error from Refresh when CountPending fails")
	}
}

func TestCleanupDone_ComputesCutoffAndForwardsLimit(t *testing.T) {
	repo := &fakeMonitorRepo{deleteCount: 12}
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	retention := 30 * 24 * time.Hour

	deleted, err := CleanupDone(context.Background(), repo, now, retention, 500)
	if err != nil {
		t.Fatalf("CleanupDone: %v", err)
	}
	if deleted != 12 {
		t.Errorf("deleted = %d, want 12", deleted)
	}
	wantCutoff := now.Add(-retention)
	if !repo.deleteCutoff.Equal(wantCutoff) {
		t.Errorf("cutoff = %v, want %v", repo.deleteCutoff, wantCutoff)
	}
	if repo.deleteLimit != 500 {
		t.Errorf("limit = %d, want 500", repo.deleteLimit)
	}
}
