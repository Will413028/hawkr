package outbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRow struct {
	id      int64
	attempt int
}

func (r *fakeRow) OutboxID() int64         { return r.id }
func (r *fakeRow) OutboxAttemptCount() int { return r.attempt }

type claimCall struct {
	id              int64
	leaseUntil      time.Time
	claimableBefore time.Time
}

type markFailedCall struct {
	id      int64
	lastErr string
}

type updateCall struct {
	id            int64
	attemptCount  int
	nextAttemptAt time.Time
	lastError     string
}

type fakeRepo struct {
	pending []*fakeRow

	claimClaimed int64 // 0 = default to "claimed" (1) unless claimErr is set
	claimErr     error
	claimCalls   []claimCall

	markDoneCalls   []int64
	markDoneErr     error
	markFailedCalls []markFailedCall
	markFailedErr   error
	updateCalls     []updateCall
	updateErr       error
}

func (f *fakeRepo) FetchPendingDue(_ context.Context, _ time.Time, _ int) ([]*fakeRow, error) {
	return f.pending, nil
}

func (f *fakeRepo) Claim(_ context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	f.claimCalls = append(f.claimCalls, claimCall{id: id, leaseUntil: leaseUntil, claimableBefore: claimableBefore})
	if f.claimErr != nil {
		return 0, f.claimErr
	}
	if f.claimClaimed != 0 {
		return f.claimClaimed, nil
	}
	return 1, nil
}

func (f *fakeRepo) MarkDone(_ context.Context, id int64, _ time.Time) error {
	f.markDoneCalls = append(f.markDoneCalls, id)
	return f.markDoneErr
}

func (f *fakeRepo) MarkFailed(_ context.Context, id int64, lastErr string, _ time.Time) error {
	f.markFailedCalls = append(f.markFailedCalls, markFailedCall{id: id, lastErr: lastErr})
	return f.markFailedErr
}

func (f *fakeRepo) UpdateAttempt(_ context.Context, id int64, attemptCount int, nextAttemptAt time.Time, lastError string) error {
	f.updateCalls = append(f.updateCalls, updateCall{id: id, attemptCount: attemptCount, nextAttemptAt: nextAttemptAt, lastError: lastError})
	return f.updateErr
}

func newTestWorker(repo *fakeRepo, process ProcessFunc[*fakeRow], hooks Hooks[*fakeRow]) *Worker[*fakeRow] {
	return New[*fakeRow](repo, process, Config{
		MaxAttempts: 5,
		BaseBackoff: time.Minute,
		MaxBackoff:  time.Hour,
		FetchBatch:  50,
		ClaimLease:  3 * time.Minute,
	}, hooks)
}

func TestWorker_ProcessSuccess_MarksDoneAndFiresOnDone(t *testing.T) {
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}}}
	var gotResult any
	var gotRow *fakeRow
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		return "ok-note", nil
	}, Hooks[*fakeRow]{
		OnDone: func(_ context.Context, row *fakeRow, result any) {
			gotRow = row
			gotResult = result
		},
	})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.markDoneCalls) != 1 || repo.markDoneCalls[0] != 1 {
		t.Fatalf("markDoneCalls = %+v, want [1]", repo.markDoneCalls)
	}
	if gotRow == nil || gotRow.id != 1 {
		t.Fatalf("OnDone row = %+v, want id=1", gotRow)
	}
	if gotResult != "ok-note" {
		t.Fatalf("OnDone result = %v, want ok-note", gotResult)
	}
	if len(repo.updateCalls) != 0 || len(repo.markFailedCalls) != 0 {
		t.Fatalf("did not expect UpdateAttempt/MarkFailed; got update=%+v failed=%+v", repo.updateCalls, repo.markFailedCalls)
	}
}

func TestWorker_ClaimLost_SkipsProcessAndHooks(t *testing.T) {
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}}, claimClaimed: -1} // sentinel handled below
	repo.claimClaimed = 0
	// Force "lost" by making Claim return 0 explicitly via a wrapper repo.
	lostRepo := &lostClaimRepo{fakeRepo: repo}
	processCalled := false
	attemptFailedCalled := false
	w := New[*fakeRow](lostRepo, func(_ context.Context, _ *fakeRow) (any, error) {
		processCalled = true
		return nil, nil
	}, Config{MaxAttempts: 5, BaseBackoff: time.Minute, MaxBackoff: time.Hour, FetchBatch: 50, ClaimLease: 3 * time.Minute},
		Hooks[*fakeRow]{OnAttemptFailed: func(_ context.Context, _ *fakeRow, _ string, _ error) { attemptFailedCalled = true }})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if processCalled {
		t.Fatal("claim lost (rows affected = 0) must not call process")
	}
	if attemptFailedCalled {
		t.Fatal("claim lost must not fire OnAttemptFailed — this replica never attempted the row")
	}
	if len(repo.claimCalls) != 1 {
		t.Fatalf("claimCalls = %+v, want exactly one claim attempt", repo.claimCalls)
	}
}

// lostClaimRepo wraps fakeRepo so Claim always reports "another replica won"
// (rows affected = 0) while still recording the call for assertions.
type lostClaimRepo struct{ *fakeRepo }

func (l *lostClaimRepo) Claim(ctx context.Context, id int64, leaseUntil, claimableBefore time.Time) (int64, error) {
	l.claimCalls = append(l.claimCalls, claimCall{id: id, leaseUntil: leaseUntil, claimableBefore: claimableBefore})
	return 0, nil
}

func TestWorker_ClaimError_SkipsProcessButFiresOnAttemptFailed(t *testing.T) {
	claimErr := errors.New("connection refused")
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}}, claimErr: claimErr}
	processCalled := false
	failedCount := 0
	var gotCause string
	var gotErr error
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		processCalled = true
		return nil, nil
	}, Hooks[*fakeRow]{OnAttemptFailed: func(_ context.Context, row *fakeRow, cause string, err error) {
		failedCount++
		gotCause = cause
		gotErr = err
		if row.id != 1 {
			t.Errorf("OnAttemptFailed row.id = %d, want 1", row.id)
		}
	}})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if processCalled {
		t.Fatal("claim error must not call process")
	}
	if failedCount != 1 {
		t.Fatalf("OnAttemptFailed fired %d times, want 1", failedCount)
	}
	if gotCause != "claim" {
		t.Errorf("cause = %q, want %q", gotCause, "claim")
	}
	if !errors.Is(gotErr, claimErr) {
		t.Errorf("err = %v, want %v", gotErr, claimErr)
	}
}

// TestWorker_ProcessFails_FiresOnAttemptFailedWithCause verifies the
// non-terminal Process-failure path reports cause="process" with the
// underlying error (not just the stringified lastErr MarkFailed/UpdateAttempt
// receive).
func TestWorker_ProcessFails_FiresOnAttemptFailedWithCause(t *testing.T) {
	processErr := errors.New("upstream 503")
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}}}
	var gotCause string
	var gotErr error
	failedCount := 0
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		return nil, processErr
	}, Hooks[*fakeRow]{OnAttemptFailed: func(_ context.Context, _ *fakeRow, cause string, err error) {
		failedCount++
		gotCause = cause
		gotErr = err
	}})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if failedCount != 1 {
		t.Fatalf("OnAttemptFailed fired %d times, want 1", failedCount)
	}
	if gotCause != "process" {
		t.Errorf("cause = %q, want %q", gotCause, "process")
	}
	if !errors.Is(gotErr, processErr) {
		t.Errorf("err = %v, want %v", gotErr, processErr)
	}
}

// TestWorker_MarkDoneFails_FiresOnAttemptFailedWithCause verifies a
// successful Process whose MarkDone write then fails is reported as
// cause="mark_done" (and still bumps the attempt, same as before this hook
// carried cause information).
func TestWorker_MarkDoneFails_FiresOnAttemptFailedWithCause(t *testing.T) {
	markDoneErr := errors.New("db unavailable")
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}}, markDoneErr: markDoneErr}
	var gotCause string
	var gotErr error
	failedCount := 0
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		return nil, nil
	}, Hooks[*fakeRow]{OnAttemptFailed: func(_ context.Context, _ *fakeRow, cause string, err error) {
		failedCount++
		gotCause = cause
		gotErr = err
	}})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if failedCount != 1 {
		t.Fatalf("OnAttemptFailed fired %d times, want 1", failedCount)
	}
	if gotCause != "mark_done" {
		t.Errorf("cause = %q, want %q", gotCause, "mark_done")
	}
	if !errors.Is(gotErr, markDoneErr) {
		t.Errorf("err = %v, want %v", gotErr, markDoneErr)
	}
	if len(repo.updateCalls) != 1 || repo.updateCalls[0].lastError != "mark_done: db unavailable" {
		t.Errorf("updateCalls = %+v, want one call with lastError prefixed mark_done:", repo.updateCalls)
	}
}

// TestWorker_UpdateAttemptFails_FiresSecondOnAttemptFailedForUpdateAttempt
// verifies the previously-silently-discarded UpdateAttempt error (the row's
// own retry-bookkeeping write) now gets its own OnAttemptFailed fire, in
// ADDITION to the original cause's fire — both failures are real and
// independent, so both must be observable.
func TestWorker_UpdateAttemptFails_FiresSecondOnAttemptFailedForUpdateAttempt(t *testing.T) {
	processErr := errors.New("upstream 503")
	updateErr := errors.New("db write timeout")
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}}, updateErr: updateErr}
	var causes []string
	var errs []error
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		return nil, processErr
	}, Hooks[*fakeRow]{OnAttemptFailed: func(_ context.Context, _ *fakeRow, cause string, err error) {
		causes = append(causes, cause)
		errs = append(errs, err)
	}})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(causes) != 2 {
		t.Fatalf("OnAttemptFailed fired %d times, want 2 (process, then update_attempt); causes=%v", len(causes), causes)
	}
	if causes[0] != "process" || !errors.Is(errs[0], processErr) {
		t.Errorf("first fire = (%q, %v), want (process, %v)", causes[0], errs[0], processErr)
	}
	if causes[1] != "update_attempt" || !errors.Is(errs[1], updateErr) {
		t.Errorf("second fire = (%q, %v), want (update_attempt, %v)", causes[1], errs[1], updateErr)
	}
}

func TestWorker_ProcessFails_BumpsAttemptWithBackoff(t *testing.T) {
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}}}
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		return nil, errors.New("upstream 503")
	}, Hooks[*fakeRow]{})

	before := time.Now()
	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	after := time.Now()

	if len(repo.updateCalls) != 1 {
		t.Fatalf("updateCalls = %+v, want 1", repo.updateCalls)
	}
	call := repo.updateCalls[0]
	if call.attemptCount != 1 {
		t.Errorf("attemptCount = %d, want 1", call.attemptCount)
	}
	if call.lastError != "upstream 503" {
		t.Errorf("lastError = %q, want %q", call.lastError, "upstream 503")
	}
	expectedMin := before.Add(2 * time.Minute)
	expectedMax := after.Add(2 * time.Minute).Add(time.Second)
	if call.nextAttemptAt.Before(expectedMin) || call.nextAttemptAt.After(expectedMax) {
		t.Errorf("nextAttemptAt = %v, want between %v and %v", call.nextAttemptAt, expectedMin, expectedMax)
	}
	if len(repo.markFailedCalls) != 0 || len(repo.markDoneCalls) != 0 {
		t.Errorf("did not expect MarkFailed/MarkDone; got failed=%+v done=%+v", repo.markFailedCalls, repo.markDoneCalls)
	}
}

func TestWorker_AttemptCapReached_MarksFailedAndFiresOnTerminalFailed(t *testing.T) {
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 4}}} // one more failure hits MaxAttempts=5
	var gotAttempt int
	var gotLastErr string
	var gotMarkErr error
	attemptFailedCount := 0
	var gotCause string
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		return nil, errors.New("still failing")
	}, Hooks[*fakeRow]{
		OnTerminalFailed: func(_ context.Context, _ *fakeRow, newAttempt int, lastErr string, markFailedErr error) {
			gotAttempt = newAttempt
			gotLastErr = lastErr
			gotMarkErr = markFailedErr
		},
		OnAttemptFailed: func(_ context.Context, _ *fakeRow, cause string, _ error) {
			attemptFailedCount++
			gotCause = cause
		},
	})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	// A terminal failure is still an attempt failure: both hooks fire.
	if attemptFailedCount != 1 {
		t.Fatalf("OnAttemptFailed fired %d times, want 1", attemptFailedCount)
	}
	if gotCause != "process" {
		t.Errorf("OnAttemptFailed cause = %q, want %q", gotCause, "process")
	}
	if len(repo.markFailedCalls) != 1 || repo.markFailedCalls[0].id != 1 {
		t.Fatalf("markFailedCalls = %+v, want one call for id 1", repo.markFailedCalls)
	}
	if repo.markFailedCalls[0].lastErr != "still failing" {
		t.Errorf("MarkFailed lastErr = %q, want %q", repo.markFailedCalls[0].lastErr, "still failing")
	}
	if gotAttempt != 5 {
		t.Errorf("OnTerminalFailed newAttempt = %d, want 5", gotAttempt)
	}
	if gotLastErr != "still failing" {
		t.Errorf("OnTerminalFailed lastErr = %q, want %q", gotLastErr, "still failing")
	}
	if gotMarkErr != nil {
		t.Errorf("OnTerminalFailed markFailedErr = %v, want nil", gotMarkErr)
	}
	if len(repo.updateCalls) != 0 {
		t.Errorf("did not expect UpdateAttempt at terminal; got %+v", repo.updateCalls)
	}
}

func TestWorker_TerminalFailed_MarkFailedWriteErrors_StillFiresHookDegraded(t *testing.T) {
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 4}}, markFailedErr: errors.New("db unavailable")}
	var gotMarkErr error
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		return nil, errors.New("still failing")
	}, Hooks[*fakeRow]{
		OnTerminalFailed: func(_ context.Context, _ *fakeRow, _ int, _ string, markFailedErr error) {
			gotMarkErr = markFailedErr
		},
	})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if gotMarkErr == nil || gotMarkErr.Error() != "db unavailable" {
		t.Fatalf("OnTerminalFailed markFailedErr = %v, want db unavailable", gotMarkErr)
	}
}

func TestWorker_LeaseBaselineIsPerRowNotPerTick(t *testing.T) {
	repo := &fakeRepo{pending: []*fakeRow{{id: 1, attempt: 0}, {id: 2, attempt: 0}}}
	calls := 0
	w := newTestWorker(repo, func(_ context.Context, _ *fakeRow) (any, error) {
		if calls == 0 {
			time.Sleep(40 * time.Millisecond)
		}
		calls++
		return nil, nil
	}, Hooks[*fakeRow]{})

	if err := w.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(repo.claimCalls) != 2 {
		t.Fatalf("claimCalls = %+v, want two", repo.claimCalls)
	}
	gap := repo.claimCalls[1].leaseUntil.Sub(repo.claimCalls[0].leaseUntil)
	if gap < 20*time.Millisecond {
		t.Fatalf("second row's lease only %v later than first — lease baseline is shared per-tick, not taken per-row", gap)
	}
	if !repo.claimCalls[0].claimableBefore.Equal(repo.claimCalls[1].claimableBefore) {
		t.Fatalf("claimableBefore should share the tick start: %v vs %v", repo.claimCalls[0].claimableBefore, repo.claimCalls[1].claimableBefore)
	}
}

func TestBackoffFor_ExponentialCappedAtMax(t *testing.T) {
	base := time.Minute
	maxD := time.Hour
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{3, 8 * time.Minute},
		{4, 16 * time.Minute},
		{10, time.Hour}, // capped
	}
	for _, c := range cases {
		if got := backoffFor(c.attempt, base, maxD); got != c.want {
			t.Errorf("backoffFor(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}
