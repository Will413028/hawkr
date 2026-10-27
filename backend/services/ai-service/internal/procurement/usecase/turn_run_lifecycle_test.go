package usecase

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/stretchr/testify/require"
)

// TestReplayTurnRun_NoResultRevisionID_ReturnsRunResultUnavailable mirrors
// replayExistingRun's existing not-ready guard (turn.go:1161-1163) — a run
// whose pipeline hasn't finished (or produced no revision) has nothing to
// replay yet.
func TestReplayTurnRun_NoResultRevisionID_ReturnsRunResultUnavailable(t *testing.T) {
	runStore := newFakeRunStore()
	run := repository.Run{ID: 1, BuyerID: 42, ResultRevisionID: 0}

	_, err := replayTurnRun(context.Background(), run, TurnInput{BuyerID: 42, ClientRequestID: "cr-1"}, runStore)
	require.ErrorIs(t, err, ErrRunResultUnavailable)
}

// TestReplayTurnRun_NoStoredAnswerPatch_ReturnsRunResultUnavailable covers
// the case this task's widened interface makes newly reachable for the
// graph path: ResultRevisionID is non-zero (a real terminal write
// happened) but FindByIdempotencyKey finds nothing (the graph path writes
// no answer_patch row today — that is Sub-project 2's job, not this
// plan's). Must fail the same honest way replayExistingRun already does
// for this case (turn.go:1166-1168), not panic or return a zero TurnResult.
func TestReplayTurnRun_NoStoredAnswerPatch_ReturnsRunResultUnavailable(t *testing.T) {
	runStore := newFakeRunStore()
	runStore.findByIdempotencyKeyErr = repository.ErrNotFound
	run := repository.Run{ID: 1, BuyerID: 42, ResultRevisionID: 777}

	_, err := replayTurnRun(context.Background(), run, TurnInput{BuyerID: 42, ClientRequestID: "cr-1"}, runStore)
	require.ErrorIs(t, err, ErrRunResultUnavailable)
}

// TestReplayTurnRun_StoredPatch_ReturnsReplayedResult proves the happy
// path: given a stored answer_patch, replayTurnRun returns the same
// TurnResult replayResult would build from it directly — this is a thin
// wrapper around existing, untouched logic, not a new algorithm.
func TestReplayTurnRun_StoredPatch_ReturnsReplayedResult(t *testing.T) {
	runStore := newFakeRunStore()
	patchJSON, err := json.Marshal(turnPayload{Message: "我要買蘋果", Reply: "已記下你的需求"})
	require.NoError(t, err)
	runStore.findByIdempotencyKeyResult = repository.IdempotentResult{
		RequestID: 55, RevisionID: 999, PayloadJSON: patchJSON,
	}
	runStore.findByIdempotencyKeyErr = nil
	run := repository.Run{ID: 1, BuyerID: 42, ResultRevisionID: 999}

	got, err := replayTurnRun(context.Background(), run, TurnInput{BuyerID: 42, ClientRequestID: "cr-1"}, runStore)
	require.NoError(t, err)
	require.Equal(t, int64(55), got.RequestID)
	require.Equal(t, "已記下你的需求", got.Reply)
}

// TestReplayTurnRun_SameKeyDifferentRequest_ReturnsKeyConflict mirrors
// replayExistingRun's existing conflict check (turn.go:1174-1176) verbatim.
func TestReplayTurnRun_SameKeyDifferentRequest_ReturnsKeyConflict(t *testing.T) {
	runStore := newFakeRunStore()
	runStore.findByIdempotencyKeyResult = repository.IdempotentResult{RequestID: 55, RevisionID: 999}
	runStore.findByIdempotencyKeyErr = nil
	run := repository.Run{ID: 1, BuyerID: 42, ResultRevisionID: 999}

	_, err := replayTurnRun(context.Background(), run,
		TurnInput{BuyerID: 42, RequestID: 999999, ClientRequestID: "cr-1"}, runStore)
	require.ErrorIs(t, err, domain.ErrIdempotencyKeyConflict)
}

// TestTerminalizeFailedRun_WritesFailedStatus proves the free function
// (moved from *TurnUseCase.terminalizeFailedRun, turn.go:1058-1079) still
// does exactly what its method form did: mark the run FAILED with the
// existing TURN_PIPELINE_ERROR reason.
func TestTerminalizeFailedRun_WritesFailedStatus(t *testing.T) {
	runStore := newFakeRunStore()
	run := repository.Run{ID: 1, BuyerID: 42, Version: 3}

	terminalizeFailedRun(context.Background(), run, 42, runStore)

	require.Len(t, runStore.terminals, 1)
	got := runStore.terminals[0]
	require.Equal(t, domain.RunStatusFailed(), got.Status)
	require.Equal(t, codemeta.ProcurementRunFailureReasonTurnPipelineError.Value, got.FailureReason)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunFailed.Value, got.TerminalEventType)
}

// TestTerminalizeFailedRun_TerminalizeRunErrors_DoesNotPanic proves the
// log-only-on-failure discipline (turn.go:1069-1071) survived the move —
// this must not panic or return an error (it has no return value).
func TestTerminalizeFailedRun_TerminalizeRunErrors_DoesNotPanic(t *testing.T) {
	runStore := newFakeRunStore()
	runStore.terminalErr = errBoom
	run := repository.Run{ID: 1, BuyerID: 42, Version: 3}

	require.NotPanics(t, func() {
		terminalizeFailedRun(context.Background(), run, 42, runStore)
	})
}

// fakeTurnExecutor is a minimal, controllable TurnExecutor — lets
// dispatchTurnViaExecutor's own tests exercise the shared claim/replay/
// dispatch/panic-recovery logic without depending on executeTurnPipeline
// or a real graph runner, matching CLAUDE.md §15 (search before adding a
// parallel test double — this is the first TurnExecutor test double, not
// a duplicate of one).
type fakeTurnExecutor struct {
	outcome turnPipelineOutcome
	err     error
	panics  bool
	calls   int
	// onCall, when set, runs inside the executor body — i.e. on the
	// dispatcher's background goroutine, while dispatchTurnViaExecutor's
	// caller is still blocked waiting for `done`. That window is the only
	// place a test can observe "during dispatch" behaviour (activity
	// delivery, claim/dispatch ordering); extending this existing double
	// with one optional hook keeps `calls` accounting and the `panics`
	// branch intact, per CLAUDE.md §15 (extend the owner, don't fork a
	// parallel fixture).
	onCall func()
}

func (f *fakeTurnExecutor) call(ctx context.Context, run repository.Run, in TurnInput, runStore RunClaimStore) (turnPipelineOutcome, error) {
	f.calls++
	if f.panics {
		panic("boom inside fake executor")
	}
	if f.onCall != nil {
		f.onCall()
	}
	return f.outcome, f.err
}

// TestDispatchTurnViaExecutor_NewClaim_CallsExecutorOnce proves the basic
// wiring: a genuinely new claim invokes the executor exactly once and
// returns its outcome wrapped into SubmitTurnResult.
func TestDispatchTurnViaExecutor_NewClaim_CallsExecutorOnce(t *testing.T) {
	fe := &fakeTurnExecutor{outcome: turnPipelineOutcome{result: TurnResult{RequestID: 5, RevisionID: 6}}}
	runStore := newFakeRunStore()
	in := TurnInput{BuyerID: 1, Message: "test", ClientRequestID: "cr-exec-1"}

	got, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(), domain.ControllerKindDurable, fe.call, TurnStreamHooks{})
	require.NoError(t, err)
	require.Equal(t, 1, fe.calls)
	require.NotZero(t, got.RunID)
	require.Equal(t, int64(5), got.Result.RequestID)
}

// TestDispatchTurnViaExecutor_RepeatedClaim_DoesNotCallExecutorTwice
// mirrors TestClaimAndDispatchTurn_DuplicateClientRequestID_DoesNotRerunModel
// (turn_claim_dispatch_test.go) at the shared-layer level: a repeated
// (BuyerID, ClientRequestID) must not invoke the executor a second time —
// it must replay via replayTurnRun (Task 1) instead.
func TestDispatchTurnViaExecutor_RepeatedClaim_DoesNotCallExecutorTwice(t *testing.T) {
	fe := &fakeTurnExecutor{outcome: turnPipelineOutcome{result: TurnResult{RequestID: 5, RevisionID: 6}}}
	runStore := newFakeRunStore()
	in := TurnInput{BuyerID: 1, Message: "test", ClientRequestID: "cr-exec-2"}

	first, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(), domain.ControllerKindDurable, fe.call, TurnStreamHooks{})
	require.NoError(t, err)
	require.Equal(t, 1, fe.calls)

	// The fake executor doesn't actually call TerminalizeRun/set
	// ResultRevisionID on the run (it just returns an outcome — Task
	// 1's fakeRunStore.TerminalizeRun is never invoked by this fake
	// executor), so a genuine replay isn't reachable through THIS
	// particular fake without also faking a terminalize call. Assert
	// what dispatchTurnViaExecutor itself is responsible for instead:
	// the run was claimed only once.
	second, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(), domain.ControllerKindDurable, fe.call, TurnStreamHooks{})
	require.Error(t, err, "second call hits the not-yet-terminalized run and must not silently re-execute")
	require.Equal(t, 1, fe.calls, "executor must not be called a second time for a repeated claim, regardless of replay outcome")
	require.Equal(t, first.RunID, second.RunID)
}

// TestDispatchTurnViaExecutor_ExecutorPanics_TerminalizesFailedAndReturnsError
// proves the shared panic-recovery guarantee this task exists to give the
// graph path — mirrors TestClaimAndDispatchTurn_BackgroundPanic_DoesNotHangCaller
// at the shared-layer level, using a fake executor instead of a real
// pipeline so this is a fast unit test, not a background-panic race test.
func TestDispatchTurnViaExecutor_ExecutorPanics_TerminalizesFailedAndReturnsError(t *testing.T) {
	fe := &fakeTurnExecutor{panics: true}
	runStore := newFakeRunStore()
	in := TurnInput{BuyerID: 1, Message: "test", ClientRequestID: "cr-exec-panic-1"}

	type outcome struct {
		res SubmitTurnResult
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		res, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(), domain.ControllerKindDurable, fe.call, TurnStreamHooks{})
		ch <- outcome{res, err}
	}()

	select {
	case got := <-ch:
		require.Error(t, got.err)
	case <-time.After(5 * time.Second):
		t.Fatal("dispatchTurnViaExecutor did not return after the executor panicked")
	}
	require.Len(t, runStore.terminals, 1)
	require.Equal(t, domain.RunStatusFailed(), runStore.terminals[0].Status)
}

// TestDispatchTurnViaExecutor_ClaimError_ReturnsErrorWithoutCallingExecutor
// proves the executor is never invoked when ClaimRun itself fails.
func TestDispatchTurnViaExecutor_ClaimError_ReturnsErrorWithoutCallingExecutor(t *testing.T) {
	fe := &fakeTurnExecutor{}
	runStore := newFakeRunStore()
	runStore.claimErr = errBoom
	in := TurnInput{BuyerID: 1, Message: "test", ClientRequestID: "cr-exec-claimerr-1"}

	_, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(), domain.ControllerKindDurable, fe.call, TurnStreamHooks{})
	require.ErrorIs(t, err, errBoom)
	require.Zero(t, fe.calls)
}

// activityDeliveryProbeTimeout is the shared "this should have happened
// essentially instantly" bound for the activity-hook tests below. It serves
// two related purposes:
//
//   - in TestDispatchTurnViaExecutor_DeliversActivityDuringDispatch, how long
//     the fake executor waits for the caller-side select loop to consume its
//     payloads before giving up and returning anyway;
//   - in TestDispatchTurnViaExecutor_NilHooks_UnchangedBehaviour, how long the
//     test waits for dispatch itself to return before calling it hung.
//
// Generous on purpose: both are "did it happen at all" probes, not latency
// assertions, and a correct implementation satisfies each within microseconds.
// On a correct implementation neither deadline is ever reached — the cost of
// the generosity is only that a genuine regression takes this long to report.
const activityDeliveryProbeTimeout = 2 * time.Second

// TestDispatchTurnViaExecutor_DeliversActivityDuringDispatch is the core
// behavioural test for this task: activity payloads written to
// hooks.ActivityCh while the executor is still running must reach
// hooks.OnActivity *before* the executor returns — not be swept up
// afterwards.
//
// The mechanism that makes that distinction observable: the executor sends
// two payloads and then blocks until OnActivity has seen both. Because
// `done` is only written after the executor returns, the executor being
// unblocked at all proves delivery happened while dispatch was still in
// flight. A post-hoc drain (the only other delivery route) cannot satisfy
// it — the drain runs after `<-done`, which cannot fire until the executor
// has already given up on its timeout, leaving deliveredDuringDispatch
// false. That flag, not the payload contents, is this test's load-bearing
// assertion; the mutation check recorded in this task's report reverts the
// select loop to a bare `<-done` (drain kept) and confirms this test fails.
func TestDispatchTurnViaExecutor_DeliversActivityDuringDispatch(t *testing.T) {
	activityCh := make(chan []byte, 64)
	seen := make(chan struct{})

	var mu sync.Mutex
	var got []string
	var deliveredDuringDispatch bool

	fe := &fakeTurnExecutor{
		outcome: turnPipelineOutcome{result: TurnResult{RequestID: 5, RevisionID: 6}},
		onCall: func() {
			activityCh <- []byte(`{"rowId":"1","code":"EXTRACT_SLOTS","text":""}`)
			activityCh <- []byte(`{"rowId":"1","code":"EXTRACT_SLOTS","text":"整理你的採購條件"}`)
			select {
			case <-seen:
				mu.Lock()
				deliveredDuringDispatch = true
				mu.Unlock()
			case <-time.After(activityDeliveryProbeTimeout):
			}
		},
	}
	runStore := newFakeRunStore()
	in := TurnInput{BuyerID: 1, Message: "我要買蘋果", ClientRequestID: "cr-activity-1"}

	hooks := TurnStreamHooks{
		ActivityCh: activityCh,
		OnActivity: func(payload []byte) {
			mu.Lock()
			got = append(got, string(payload))
			n := len(got)
			mu.Unlock()
			if n == 2 {
				close(seen)
			}
		},
	}

	res, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(),
		domain.ControllerKindGraph, fe.call, hooks)
	require.NoError(t, err)
	require.Equal(t, int64(5), res.Result.RequestID, "the ordinary dispatch result must still come back unchanged")

	mu.Lock()
	defer mu.Unlock()
	require.True(t, deliveredDuringDispatch,
		"activity payloads must reach OnActivity while the executor is still running, not only after it returns")
	require.Equal(t, []string{
		`{"rowId":"1","code":"EXTRACT_SLOTS","text":""}`,
		`{"rowId":"1","code":"EXTRACT_SLOTS","text":"整理你的採購條件"}`,
	}, got, "both payloads must arrive, in order, byte-for-byte")
}

// TestDispatchTurnViaExecutor_OnClaimedFiresBeforeDispatch pins the ordering
// the handler depends on: RunAccepted must be emittable before any activity
// frame, so OnClaimed has to run before the executor does.
//
// How it fails if the behaviour regresses: moving the OnClaimed call below
// the select loop makes order ["executor","claimed"]; dropping the call
// entirely makes it ["executor"]. The ordering asserted here is deterministic,
// not a race — dispatchTurnViaExecutor blocks until the executor has
// finished, so both appends have happened by the time the assertion runs.
//
// One mutation this test does NOT catch: moving the OnClaimed call to just
// below dispatcher.Dispatch. The caller appending "claimed" almost always
// wins the footrace against goroutine start-up, so this test would likely
// still pass. TestDispatchTurnViaExecutor_OnClaimedFiresOnReplayedRun below
// catches that one deterministically (a replayed claim never reaches
// dispatcher.Dispatch at all, so claimedIDs would have length 1, not 2).
func TestDispatchTurnViaExecutor_OnClaimedFiresBeforeDispatch(t *testing.T) {
	var mu sync.Mutex
	var order []string
	var claimedRunID int64

	fe := &fakeTurnExecutor{onCall: func() {
		mu.Lock()
		order = append(order, "executor")
		mu.Unlock()
	}}
	runStore := newFakeRunStore()
	in := TurnInput{BuyerID: 1, Message: "我要買蘋果", ClientRequestID: "cr-onclaimed-1"}

	res, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(),
		domain.ControllerKindGraph, fe.call, TurnStreamHooks{
			OnClaimed: func(runID int64) {
				mu.Lock()
				order = append(order, "claimed")
				claimedRunID = runID
				mu.Unlock()
			},
		})
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"claimed", "executor"}, order)
	require.Equal(t, res.RunID, claimedRunID, "OnClaimed must carry the id of the run that was actually claimed")
}

// TestDispatchTurnViaExecutor_OnClaimedFiresOnReplayedRun pins the other half
// of the same Global Constraint: OnClaimed sits above the `if !isNew` branch
// on purpose. A repeated (BuyerID, ClientRequestID) is still a real, claimed
// run, so the handler must still be able to announce it — otherwise a retried
// request streams events that never identify their run.
//
// How it fails if the behaviour regresses: sinking the OnClaimed call below
// the `if !isNew` block leaves calls==1 (only the first, genuinely-new claim
// fires it) instead of 2.
func TestDispatchTurnViaExecutor_OnClaimedFiresOnReplayedRun(t *testing.T) {
	fe := &fakeTurnExecutor{outcome: turnPipelineOutcome{result: TurnResult{RequestID: 5, RevisionID: 6}}}
	runStore := newFakeRunStore()
	in := TurnInput{BuyerID: 1, Message: "我要買蘋果", ClientRequestID: "cr-onclaimed-replay-1"}

	var claimedIDs []int64
	hooks := TurnStreamHooks{OnClaimed: func(runID int64) { claimedIDs = append(claimedIDs, runID) }}

	first, err := dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(),
		domain.ControllerKindGraph, fe.call, hooks)
	require.NoError(t, err)

	// Second claim replays (this fake executor never terminalizes, so the
	// replay itself has nothing to return — see
	// TestDispatchTurnViaExecutor_RepeatedClaim_DoesNotCallExecutorTwice
	// for why that error is expected and not what this test is about).
	_, err = dispatchTurnViaExecutor(context.Background(), in, runStore, NewRunDispatcher(),
		domain.ControllerKindGraph, fe.call, hooks)
	require.ErrorIs(t, err, ErrRunResultUnavailable,
		"must be the replay branch's not-ready sentinel — a bare Error check would also swallow an unrelated replay regression")
	require.Equal(t, 1, fe.calls, "the replayed claim must still not re-run the executor")

	require.Equal(t, []int64{first.RunID, first.RunID}, claimedIDs,
		"OnClaimed must fire for the replayed claim too, carrying the same run id")
}

// TestDispatchTurnViaExecutor_NilHooks_UnchangedBehaviour covers the two
// caller shapes that leave part of TurnStreamHooks nil.
//
// Deliberately NOT wrapped in require.NotPanics: dispatch runs on a spawned
// goroutine here (so a hang is reported as a bounded, legible failure instead
// of a package-wide test timeout), and require.NotPanics cannot observe a
// panic raised on a different goroutine — it would be a hollow assertion. A
// nil-func dereference in the select loop still fails this test, loudly and
// with a better stack trace, by taking the test binary down.
//
// The "fully zero hooks" subtest is a named regression guard for the contract
// SubmitTurn (unary) relies on. Honest about its overlap: dropping the
// `hooks.OnClaimed != nil` guard would also fail
// TestDispatchTurnViaExecutor_NewClaim_CallsExecutorOnce, which already
// dispatches with TurnStreamHooks{}. Its unique contribution is the bounded
// return — an implementation that ranged over the nil channel would hang, and
// this names that failure.
//
// The "live channel, nil OnActivity" subtest is the load-bearing one and is
// covered by nothing else. It is deterministic by construction: the channel
// has capacity 1 and the executor sends two payloads, so the second send can
// only complete after the select loop has received the first — which forces
// the loop's `case raw := <-hooks.ActivityCh` arm to execute with
// hooks.OnActivity == nil. Dropping the loop's `hooks.OnActivity != nil`
// guard therefore panics on every run, not on a lucky schedule; dropping
// drainActivity's `onActivity == nil` guard panics whenever the drain picks
// up the leftover second payload. Deleting the ActivityCh arm outright
// deadlocks the executor on its second send, which the timeout reports.
func TestDispatchTurnViaExecutor_NilHooks_UnchangedBehaviour(t *testing.T) {
	type dispatchOutcome struct {
		res SubmitTurnResult
		err error
	}
	run := func(t *testing.T, fe *fakeTurnExecutor, in TurnInput, hooks TurnStreamHooks, hangMsg string) {
		t.Helper()
		ch := make(chan dispatchOutcome, 1)
		go func() {
			res, err := dispatchTurnViaExecutor(context.Background(), in, newFakeRunStore(), NewRunDispatcher(),
				domain.ControllerKindGraph, fe.call, hooks)
			ch <- dispatchOutcome{res, err}
		}()
		select {
		case got := <-ch:
			require.NoError(t, got.err)
			require.Equal(t, int64(5), got.res.Result.RequestID, "the dispatch result must be unchanged by the hooks machinery")
			require.NotZero(t, got.res.RunID)
			require.Equal(t, 1, fe.calls)
		case <-time.After(activityDeliveryProbeTimeout):
			t.Fatal(hangMsg)
		}
	}

	t.Run("fully zero hooks", func(t *testing.T) {
		fe := &fakeTurnExecutor{outcome: turnPipelineOutcome{result: TurnResult{RequestID: 5, RevisionID: 6}}}
		run(t, fe, TurnInput{BuyerID: 1, Message: "我要買蘋果", ClientRequestID: "cr-nilhooks-1"},
			TurnStreamHooks{}, "dispatchTurnViaExecutor did not return with zero-value hooks")
	})

	t.Run("live channel, nil OnActivity", func(t *testing.T) {
		activityCh := make(chan []byte, 1)
		fe := &fakeTurnExecutor{
			outcome: turnPipelineOutcome{result: TurnResult{RequestID: 5, RevisionID: 6}},
			onCall: func() {
				activityCh <- []byte(`{"rowId":"1"}`)
				// Blocks until the select loop has taken the first
				// payload out of the capacity-1 buffer.
				activityCh <- []byte(`{"rowId":"2"}`)
			},
		}
		run(t, fe, TurnInput{BuyerID: 1, Message: "我要買蘋果", ClientRequestID: "cr-nilhooks-2"},
			TurnStreamHooks{ActivityCh: activityCh},
			"dispatchTurnViaExecutor did not return with an ActivityCh but no OnActivity — the select loop never consumed the channel")
	})
}

// TestDrainActivity_ForwardsBufferedPayloadsAndReturns tests drainActivity
// directly, which is fully deterministic and involves no scheduler at all:
// same package, package-level function, pre-filled buffered channel, no
// concurrent producer.
//
// This is a different claim from the one drainActivity's doc comment declines
// to test. That one — "drain is the SOLE delivery path" — really is
// unreproducible, because the select loop in dispatchTurnViaExecutor normally
// consumes first. What IS testable, and is pinned here, is drainActivity's own
// contract: given payloads sitting in the channel, forward every one of them
// in order, then return.
//
// How it fails if the behaviour regresses:
//   - a body that returns without forwarding (e.g. the loop deleted, or the
//     early-return guard widened) leaves got empty or short;
//   - forwarding out of order, or dropping one, fails the ordered compare;
//   - `for raw := range ch` instead of the select/default form never reaches
//     the `default:` exit and blocks forever on the empty channel, deadlocking
//     this test — which pins the return, not just the forwarding.
func TestDrainActivity_ForwardsBufferedPayloadsAndReturns(t *testing.T) {
	t.Run("forwards every buffered payload in order, then returns", func(t *testing.T) {
		ch := make(chan []byte, 3)
		ch <- []byte(`{"rowId":"1"}`)
		ch <- []byte(`{"rowId":"2"}`)
		ch <- []byte(`{"rowId":"3"}`)

		var got []string
		drainActivity(ch, func(payload []byte) { got = append(got, string(payload)) })

		require.Equal(t, []string{`{"rowId":"1"}`, `{"rowId":"2"}`, `{"rowId":"3"}`}, got)
		require.Empty(t, ch, "the channel must be left empty")
	})

	t.Run("empty channel is a no-op", func(t *testing.T) {
		called := false
		drainActivity(make(chan []byte, 3), func([]byte) { called = true })
		require.False(t, called)
	})

	t.Run("nil channel returns without calling onActivity", func(t *testing.T) {
		called := false
		require.NotPanics(t, func() {
			drainActivity(nil, func([]byte) { called = true })
		})
		require.False(t, called)
	})

	t.Run("nil onActivity returns without dereferencing it", func(t *testing.T) {
		ch := make(chan []byte, 1)
		ch <- []byte(`{"rowId":"1"}`)
		require.NotPanics(t, func() { drainActivity(ch, nil) })
		require.Len(t, ch, 1, "with nowhere to forward to, the payload is left where it was")
	})
}
