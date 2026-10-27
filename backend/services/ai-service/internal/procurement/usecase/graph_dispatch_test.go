// services/ai-service/internal/procurement/usecase/graph_dispatch_test.go
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// fakeRunStore is the RunClaimStore in-memory test double, letting the
// claim+dispatch orchestration be tested without touching a DB — same
// approach and same trade-offs as fakerepo_test.go's fakeRepo: it models
// the behavior that actually drives orchestration decisions (idempotency-key
// hits, hash conflicts, a terminalized run's result_revision_id going
// non-zero), not transactions/seq allocation/CAS, which only a repository
// integration test (repository/run_test.go) verifies.
//
// Moved here from the deleted turn_claim_dispatch_test.go (Task 6a of the
// procurement-graph-cutover plan, 2026-08-31): this fake is the RunClaimStore
// double every graph-dispatch test in this file already depended on, not
// just TestCanonicalTurnHash_IgnoresRequestID's narrow "fixtures it needs" —
// resume_test.go/answer_test.go/rewind_test.go/turn_run_lifecycle_test.go
// also construct it via newFakeRunStore(), so it has to live somewhere all
// of them can reach; this file (graph_dispatch_test.go) is both the file the
// brief already named as TestCanonicalTurnHash_IgnoresRequestID's new home
// and the single heaviest consumer of this fake, so it becomes the new
// owner rather than inventing a fifth fixture file.
type fakeRunStore struct {
	mu sync.Mutex

	nextID int64

	// runs 以冪等鍵 (buyerID, clientRequestID) 索引，對齊真正的
	// uq_procurement_agent_run_commands 唯一索引欄位。
	runs   map[fakePatchKey]*repository.Run
	hashes map[fakePatchKey]string

	events     []repository.AppendRunEventInput
	terminals  []repository.TerminalizeRunInput
	interrupts []repository.CreateInterruptInput

	claimErr     error
	appendErr    error
	terminalErr  error
	interruptErr error

	// getRunStatus／getRunErr 控制 GetRun（取消觀察器唯一的讀路徑）。預設空
	// 字串代表「沒有人按停止」——測試要驗取消時把它設成
	// domain.RunStatusCancelReq()。getRunCalls 讓節流行為可被斷言。
	getRunStatus string
	getRunErr    error
	getRunCalls  int

	// findByIdempotencyKeyResult／findByIdempotencyKeyErr control
	// FindByIdempotencyKey — added for replayTurnRun (turn_run_lifecycle.go,
	// 2026-08-30). Default findByIdempotencyKeyErr is repository.ErrNotFound
	// (set in newFakeRunStore below), matching what a graph-executor run
	// actually has today: no answer_patch row was ever written for it.
	findByIdempotencyKeyResult repository.IdempotentResult
	findByIdempotencyKeyErr    error

	// revisionStore, when set, makes FindByIdempotencyKey delegate to it
	// instead of findByIdempotencyKeyResult／findByIdempotencyKeyErr above.
	// Added 2026-08-30 alongside ClaimAndDispatchTurn's rewiring onto
	// dispatchTurnViaExecutor/replayTurnRun (turn_run_lifecycle.go): that
	// shared replay path now reads a repeated claim's result via
	// runStore.FindByIdempotencyKey instead of TurnUseCase's own
	// uc.store.FindByIdempotencyKey. In production those are the exact same
	// *repository.Repo instance (internal/procurement/module.go wires both
	// usecase.RevisionStore and usecase.ResumeRunStore off the same
	// fx-provided `r *repository.Repo` singleton), so this call sees
	// whatever executeTurnPipeline's HandleStream just wrote via AppendRevision.
	// fakeRepo and fakeRunStore are intentionally separate fakes for
	// different concerns (RevisionStore vs RunClaimStore); tests that drive
	// ClaimAndDispatchTurn through a real replay (not just
	// TestDispatchTurnViaExecutor_*'s direct, executor-level fixtures) need
	// this field set to the same *fakeRepo passed to newTurnUseCaseForClaim
	// to reproduce that "same instance" topology — otherwise the fake
	// FindByIdempotencyKey never observes what the fake RevisionStore just
	// recorded, which is a divergence real production wiring does not have.
	revisionStore *fakeRepo

	// openInterrupt／hasOpenInterrupt 讓 FindOpenInterruptForRequest 可控——
	// 預設沒有 open interrupt（guard 永遠放行），guard 測試自己的 fixture
	// 才需要設它（見 answer_test.go 的 open-interrupt guard 案例）。
	openInterrupt    repository.Interrupt
	hasOpenInterrupt bool
	// findOpenInterruptCalls 記錄 FindOpenInterruptForRequest 被呼叫的次數——
	// 跟 procurement_durable_run_test.go 的 stubRunStore 同一個理由：
	// TestClaimAndDispatchTurn_RequestIDZero_SkipsGuardAndClaimsNormally 需要
	// 斷言「guard 根本沒被呼叫」，不只是「呼叫了但放行」。
	findOpenInterruptCalls int

	// setConversationIDCalls 記錄 SetRunConversationID 每次被呼叫的
	// (runID, conversationID) 參數，供 Task 15 write-path 回填測試斷言
	// executeTurnPipeline 到底有沒有呼叫、呼叫時帶的值對不對。這個 fake 刻意
	// 不模擬真正的 `conversation_id IS NULL` guard（那是 repository dbtest
	// 才驗證得到的持久層細節，同本檔檔頭註解的既有取捨）——只記錄呼叫本身。
	setConversationIDCalls []setConversationIDCall
}

type setConversationIDCall struct {
	runID          int64
	conversationID int64
}

func newFakeRunStore() *fakeRunStore {
	return &fakeRunStore{
		nextID: 900,
		runs:   map[fakePatchKey]*repository.Run{},
		hashes: map[fakePatchKey]string{},
		// Default: no stored answer_patch. Tests that need a replayable
		// result set findByIdempotencyKeyResult and clear this err to nil.
		findByIdempotencyKeyErr: repository.ErrNotFound,
	}
}

func (f *fakeRunStore) id() int64 { f.nextID++; return f.nextID }

func (f *fakeRunStore) ClaimRun(_ context.Context, in repository.ClaimRunInput) (repository.Run, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.claimErr != nil {
		return repository.Run{}, false, f.claimErr
	}
	key := fakePatchKey{in.BuyerID, in.ClientRequestID}
	if existing, ok := f.runs[key]; ok {
		if f.hashes[key] != in.PayloadHash {
			return repository.Run{}, false, repository.ErrIdempotencyConflict
		}
		return *existing, false, nil
	}
	run := &repository.Run{
		ID: f.id(), BuyerID: in.BuyerID, RunType: in.RunType,
		Status: domain.RunStatusQueued(), Version: 1, LastEventSeq: 1,
		ControllerKind: in.ControllerKind,
	}
	f.runs[key] = run
	f.hashes[key] = in.PayloadHash
	return *run, true, nil
}

func (f *fakeRunStore) AppendRunEvent(_ context.Context, in repository.AppendRunEventInput) (repository.RunEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.appendErr != nil {
		return repository.RunEvent{}, f.appendErr
	}
	f.events = append(f.events, in)
	return repository.RunEvent{ID: f.id(), RunID: in.RunID, Seq: int64(len(f.events)) + 1, EventType: in.EventType}, nil
}

func (f *fakeRunStore) TerminalizeRun(_ context.Context, in repository.TerminalizeRunInput) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.terminalErr != nil {
		return false, f.terminalErr
	}
	f.terminals = append(f.terminals, in)
	// 把終態反映回 run 上：replayExistingRun 用 ResultRevisionID 當「這次執行
	// 已經留下可重播結果」的就緒判準，不同步回去的話，重送測試會拿到
	// ErrRunResultUnavailable 而不是重播結果。
	for _, run := range f.runs {
		if run.ID == in.RunID {
			run.Status = in.Status
			run.FailureReason = in.FailureReason
			run.ResultRevisionID = in.ResultRevisionID
			run.Version++
		}
	}
	return true, nil
}

// GetRun 是取消觀察器的讀路徑。回傳的 Status 由 getRunStatus 控制，空字串時
// 給 RUNNING——那是「run 還在跑、沒人按停止」的預設情境，絕大多數既有測試要的
// 就是它，不必逐一改。
func (f *fakeRunStore) GetRun(_ context.Context, runID int64, buyerID identity.BuyerID) (repository.Run, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getRunCalls++
	if f.getRunErr != nil {
		return repository.Run{}, f.getRunErr
	}
	status := f.getRunStatus
	if status == "" {
		status = domain.RunStatusRunning()
	}
	return repository.Run{ID: runID, BuyerID: buyerID, Status: status}, nil
}

func (f *fakeRunStore) CreateInterrupt(_ context.Context, in repository.CreateInterruptInput) (repository.Interrupt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.interruptErr != nil {
		return repository.Interrupt{}, f.interruptErr
	}
	f.interrupts = append(f.interrupts, in)
	return repository.Interrupt{
		ID: strconv.FormatInt(f.id(), 10), RunID: in.RunID,
		InterruptKind: in.InterruptKind, ResponseSchemaVersion: in.ResponseSchemaVersion, Status: "PENDING",
	}, nil
}

// HasIdempotencyKey 只看這把鍵有沒有 command row，不比對 hash——同 repository
// 版本的既有慣例（hash 是否相符是 ClaimRun 自己的職責）。
func (f *fakeRunStore) HasIdempotencyKey(_ context.Context, buyerID identity.BuyerID, clientRequestID string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.runs[fakePatchKey{buyerID, clientRequestID}]
	return ok, nil
}

// FindOpenInterruptForRequest 預設回 repository.ErrNotFound（沒有 open
// interrupt，guard 放行）——測試需要模擬「這個 request 被 park 住」時設
// hasOpenInterrupt/openInterrupt。
func (f *fakeRunStore) FindOpenInterruptForRequest(_ context.Context, _ int64, _ identity.BuyerID) (repository.Interrupt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.findOpenInterruptCalls++
	if f.hasOpenInterrupt {
		return f.openInterrupt, nil
	}
	return repository.Interrupt{}, repository.ErrNotFound
}

// SetRunConversationID records the call and, mirroring what the real
// repository method does, updates the in-memory run's ConversationID —
// TestClaimAndDispatchTurn_NewConversation_BackfillsRunConversationID relies
// on being able to observe this on the run this fake already tracks.
func (f *fakeRunStore) SetRunConversationID(_ context.Context, runID, conversationID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setConversationIDCalls = append(f.setConversationIDCalls, setConversationIDCall{runID: runID, conversationID: conversationID})
	for _, run := range f.runs {
		if run.ID == runID {
			run.ConversationID = conversationID
		}
	}
	return nil
}

func (f *fakeRunStore) FindByIdempotencyKey(ctx context.Context, buyerID identity.BuyerID, clientRequestID string) (repository.IdempotentResult, error) {
	f.mu.Lock()
	delegate := f.revisionStore
	result, err := f.findByIdempotencyKeyResult, f.findByIdempotencyKeyErr
	f.mu.Unlock()
	if delegate != nil {
		return delegate.FindByIdempotencyKey(ctx, buyerID, clientRequestID)
	}
	if err != nil {
		return repository.IdempotentResult{}, err
	}
	return result, nil
}

func (f *fakeRunStore) eventTypes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.events))
	for _, e := range f.events {
		out = append(out, e.EventType)
	}
	return out
}

// TestCanonicalTurnHash_IgnoresRequestID 釘住 canonicalTurnHash 的兩個刻意選擇：
// RequestID 不進 hash（否則「先不帶 requestId、重送時帶上」這種完全合法的重送
// 會變成 409），Message 進 hash（同鍵不同訊息是真正的鍵重用）。
//
// Moved here from the deleted turn_claim_dispatch_test.go — canonicalTurnHash
// is still used by dispatchTurnViaExecutor, shared with
// ClaimAndDispatchTurnViaGraph (Task 6a of the procurement-graph-cutover plan).
func TestCanonicalTurnHash_IgnoresRequestID(t *testing.T) {
	base := TurnInput{BuyerID: 1, Message: "我要買梨", ClientRequestID: "cr-1"}
	withRequest := base
	withRequest.RequestID = 12345

	require.Equal(t, canonicalTurnHash(base), canonicalTurnHash(withRequest))

	other := base
	other.Message = "我要買蘋果"
	require.NotEqual(t, canonicalTurnHash(base), canonicalTurnHash(other))

	otherBuyer := base
	otherBuyer.BuyerID = identity.BuyerID(2)
	require.NotEqual(t, canonicalTurnHash(base), canonicalTurnHash(otherBuyer))
}

// TestResolveGraphTurnState_NewRequest_CreatesRequestAndSeedsEmptyState
// proves the RequestID==0 path: a brand-new conversation creates a real
// request row (via RevisionStore.CreateRequest) and seeds an empty-slots
// state — the same values stateFor used to hard-code, now sourced from a
// real (if freshly created) request instead of never touching the store.
func TestResolveGraphTurnState_NewRequest_CreatesRequestAndSeedsEmptyState(t *testing.T) {
	repo := newFakeRepo()

	requestID, state, err := resolveGraphTurnState(t.Context(), repo, nil, TurnInput{BuyerID: identity.BuyerID(42)})
	require.NoError(t, err)

	require.Equal(t, 1, repo.createRequestCalls, "a new conversation must create exactly one request row")
	require.NotZero(t, requestID)
	require.Equal(t, domain.Slots{}, state["current_slots"])
	require.Equal(t, requestID, state["request_id"])
	require.NotNil(t, state["history"], "history must be present even for a brand-new request (a one-entry root chain)")
}

// TestResolveGraphTurnState_ExistingRequest_ReadsStoredSlotsWithoutCreating
// proves the RequestID!=0 path: an existing request's already-persisted
// slots are read back, not re-synthesized — the actual fix for the
// multi-turn-memory-loss reversal (spec §0).
func TestResolveGraphTurnState_ExistingRequest_ReadsStoredSlotsWithoutCreating(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)
	_, err = repo.AppendRevision(t.Context(), repository.AppendInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(42), Slots: domain.Slots{Item: item("蘋果")},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)

	requestID, state, err := resolveGraphTurnState(t.Context(), repo, nil, TurnInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(42),
	})
	require.NoError(t, err)

	require.Equal(t, 1, repo.createRequestCalls, "an existing request must not be created a second time")
	require.Equal(t, req.ID, requestID)
	gotSlots, ok := state["current_slots"].(domain.Slots)
	require.True(t, ok)
	require.Equal(t, "蘋果", *gotSlots.Item, "must read back the previously persisted slots, not an empty one")
	history, ok := state["history"].([]repository.RevisionWithPatch)
	require.True(t, ok)
	require.Len(t, history, 2, "root revision + the one AppendRevision call above")
}

// TestResolveGraphTurnState_StoreError_Propagates proves a store failure
// (here: buyer/request ownership mismatch, the same repository.ErrNotFound
// path GetSlots already returns) surfaces as an error rather than a silent
// empty state.
func TestResolveGraphTurnState_StoreError_Propagates(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)

	_, _, err = resolveGraphTurnState(t.Context(), repo, nil, TurnInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(999), // wrong buyer for this request
	})
	require.ErrorIs(t, err, repository.ErrNotFound)
}

// fakeRfqConfirmationReader is the usecase.RfqConfirmationReader test
// double — no existing fake implements this interface (CLAUDE.md §15
// search performed: turn_test.go/rfq_confirmation_test.go have no
// FindLatestConfirmedRfqSnapshotByRequestID double), so this is the one
// genuinely new test double this plan introduces.
type fakeRfqConfirmationReader struct {
	snapshot repository.ConfirmedRfqSnapshot
	err      error
}

func (f fakeRfqConfirmationReader) FindLatestConfirmedRfqSnapshotByRequestID(_ context.Context, _ int64, _ identity.BuyerID) (repository.ConfirmedRfqSnapshot, error) {
	return f.snapshot, f.err
}

// TestResolveGraphTurnState_RfqConfirmedAndFresh_SeedsRfqConfirmed proves
// the "confirmed and matches the tip revision" case — mirrors HandleStream's
// own turn.go:581-582 branch.
func TestResolveGraphTurnState_RfqConfirmedAndFresh_SeedsRfqConfirmed(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)
	rev, err := repo.AppendRevision(t.Context(), repository.AppendInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(42), Slots: domain.Slots{Item: item("蘋果")},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	rfq := fakeRfqConfirmationReader{snapshot: repository.ConfirmedRfqSnapshot{RequestRevisionID: rev.ID}}

	_, state, err := resolveGraphTurnState(t.Context(), repo, rfq, TurnInput{RequestID: req.ID, BuyerID: identity.BuyerID(42)})
	require.NoError(t, err)
	require.Equal(t, domain.RfqConfirmed, state["rfq_state"])
}

// TestResolveGraphTurnState_RfqConfirmedButStale_SeedsRfqStale proves the
// "confirmed but the request moved on since" case (turn.go:583-584) — the
// snapshot's RequestRevisionID no longer matches the current tip.
func TestResolveGraphTurnState_RfqConfirmedButStale_SeedsRfqStale(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)
	_, err = repo.AppendRevision(t.Context(), repository.AppendInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(42), Slots: domain.Slots{Item: item("蘋果")},
		Source: "LLM_EXTRACT", ClientRequestID: "seed-1", PayloadJSON: []byte(`{}`), RequestStatus: "CLARIFYING",
	})
	require.NoError(t, err)
	// Snapshot points at a revision ID that is not the current tip (999
	// never exists in this fixture) — the exact shape turn.go's "confirmed,
	// but not the tip" branch checks for.
	rfq := fakeRfqConfirmationReader{snapshot: repository.ConfirmedRfqSnapshot{RequestRevisionID: 999}}

	_, state, err := resolveGraphTurnState(t.Context(), repo, rfq, TurnInput{RequestID: req.ID, BuyerID: identity.BuyerID(42)})
	require.NoError(t, err)
	require.Equal(t, domain.RfqStale, state["rfq_state"])
}

// TestResolveGraphTurnState_NoConfirmedRfqSnapshot_SeedsRfqNone proves the
// "never confirmed" case (turn.go:585-586: repository.ErrConfirmedRfqSnapshotNotFound).
func TestResolveGraphTurnState_NoConfirmedRfqSnapshot_SeedsRfqNone(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)
	rfq := fakeRfqConfirmationReader{err: repository.ErrConfirmedRfqSnapshotNotFound}

	_, state, err := resolveGraphTurnState(t.Context(), repo, rfq, TurnInput{RequestID: req.ID, BuyerID: identity.BuyerID(42)})
	require.NoError(t, err)
	require.Equal(t, domain.RfqNone, state["rfq_state"])
}

// TestResolveGraphTurnState_RfqQueryError_SeedsRfqNoneConservatively proves
// the fail-safe default: an unexpected query error (not the "not found"
// sentinel) must not fail the whole turn, and must not be mistaken for
// RfqConfirmed/RfqStale — turn.go:587-588's `default:` branch.
func TestResolveGraphTurnState_RfqQueryError_SeedsRfqNoneConservatively(t *testing.T) {
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)
	rfq := fakeRfqConfirmationReader{err: errors.New("boom: rfq snapshot query unavailable")}

	_, state, err := resolveGraphTurnState(t.Context(), repo, rfq, TurnInput{RequestID: req.ID, BuyerID: identity.BuyerID(42)})
	require.NoError(t, err, "a query error must degrade to RfqNone, not fail the turn")
	require.Equal(t, domain.RfqNone, state["rfq_state"])
}

// TestResolveGraphTurnState_NilRfqConfirmationReader_SeedsRfqNone proves the
// nil-checker fail-safe (production wiring gap or a test that doesn't care
// about RFQ) degrades the same way a query error does, never panics.
func TestResolveGraphTurnState_NilRfqConfirmationReader_SeedsRfqNone(t *testing.T) {
	repo := newFakeRepo()

	_, state, err := resolveGraphTurnState(t.Context(), repo, nil, TurnInput{BuyerID: identity.BuyerID(42)})
	require.NoError(t, err)
	require.Equal(t, domain.RfqNone, state["rfq_state"])
}

// TestClaimAndDispatchTurnViaGraph_OpenInterruptExists_BlockedByGuard proves
// GuardNoOpenInterrupt's *blocking* behavior for this entry point, not just
// that it is called — the flag-enabled happy-path test below always runs
// with hasOpenInterrupt=false, so on its own it would never catch a refactor
// that moved the guard call below the r.Run loop (or dropped it). Mirrors
// the existing production pattern this same guard is pinned by:
// TestClaimAndDispatchTurn_OpenInterruptExists_NewCommand_BlockedByGuard
// (turn_claim_dispatch_test.go:371-382) and
// TestClaimAndDispatchAnswer_OpenInterruptExists_NewCommand_BlockedByGuard
// (answer_test.go:1414-1429).
//
// r is nil here too: if the guard did NOT block and control reached the
// r.Run loop, this test would fail with a nil-pointer panic rather than a
// silently-wrong pass — at least as strong a proof as asserting r.Run was
// never called.
func TestClaimAndDispatchTurnViaGraph_OpenInterruptExists_BlockedByGuard(t *testing.T) {
	store := newFakeRunStore()
	store.hasOpenInterrupt = true
	store.openInterrupt = repository.Interrupt{ID: "int-open-graph-dispatch-1", RunID: 9400, Status: "PENDING"}

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		RequestID: 1, BuyerID: identity.BuyerID(42), Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: nil,
		Dispatcher: NewRunDispatcher(), Runner: nil, ResumeRunner: ResumeRunner{Runner: nil}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})

	require.ErrorIs(t, err, ErrInterruptPending)
	require.Zero(t, len(store.interrupts), "guard 擋下時不得呼叫 CreateInterrupt")
	require.Zero(t, len(store.runs), "guard 擋下時不得呼叫 ClaimRun")
}

// TestClaimAndDispatchTurnViaGraph_Retry_SkipsGuardEvenWithOpenInterrupt
// mirrors TestClaimAndDispatchTurn_Retry_SkipsGuardEvenWithOpenInterrupt
// (turn_claim_dispatch_test.go) at this entry point: once multi-turn
// continuation makes in.RequestID != 0 the normal case for every message
// after a conversation's first (Task 1), the guard must not block a
// genuine retry (isRetry==true) of the SAME command just because some
// OTHER request happens to have an open interrupt — only a genuinely new
// command against a request with an open interrupt should be blocked.
//
// fakeRunStore.HasIdempotencyKey (turn_claim_dispatch_test.go) has no
// injectable boolean — it genuinely checks whether f.runs already has an
// entry for (buyerID, clientRequestID), populated by a real prior
// ClaimRun call. So "isRetry==true" is produced the honest way: dispatch
// the same TurnInput twice and observe the guard is skipped the second
// time, exactly like TestClaimAndDispatchTurn_Retry_SkipsGuardEvenWithOpenInterrupt
// already does for the legacy path.
func TestClaimAndDispatchTurnViaGraph_Retry_SkipsGuardEvenWithOpenInterrupt(t *testing.T) {
	store := newFakeRunStore()
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)
	// store.revisionStore = repo: same "same instance" topology precedent as
	// TestClaimAndDispatchTurnViaGraph_ClaimRunReplay_ReplaysRealContent above
	// — without this, fakeRunStore.FindByIdempotencyKey never observes what
	// the graph executor's first call just persisted via repo.AppendRevision,
	// so the second (retry) call would fail with ErrRunResultUnavailable
	// regardless of whether the guard itself was correctly skipped. Not
	// itself part of Task 7's guard-skip fix — needed to actually exercise
	// dispatchTurnViaExecutor's real replay path in this test, exactly as
	// the pre-existing real-replay test above already does.
	store.revisionStore = repo

	r := newNotPendingRunner(t)
	in := TurnInput{RequestID: req.ID, BuyerID: identity.BuyerID(42), ClientRequestID: "req-retry-1", Message: "我要買蘋果"}

	_, err = ClaimAndDispatchTurnViaGraph(t.Context(), in, GraphControllerDeps{
		RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err, "sanity check: the first (genuinely new) call must succeed and populate store.runs")
	guardCallsAfterFirst := store.findOpenInterruptCalls

	// An interrupt appears on this request before the retry arrives.
	store.hasOpenInterrupt = true
	store.openInterrupt = repository.Interrupt{ID: "int-open-graph-retry-1", RunID: 9401, Status: "PENDING"}

	_, err = ClaimAndDispatchTurnViaGraph(t.Context(), in, GraphControllerDeps{
		RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err, "a retry must not be blocked by GuardNoOpenInterrupt even though store.hasOpenInterrupt is true")
	require.Equal(t, guardCallsAfterFirst, store.findOpenInterruptCalls, "the guard itself must not be called again on a genuine retry")
}

// pendingCropGateCandidates is shared by the two runner-driven tests below.
func pendingCropGateCandidates() []domain.CropDisambiguationOption {
	return []domain.CropDisambiguationOption{
		{Label: "紅蘿蔔", CategoryID: "21", CategoryLevel: 3},
		{Label: "白蘿蔔", CategoryID: "22", CategoryLevel: 3},
	}
}

// newPendingInterruptRunner builds a *runner.Runner over a MINIMAL graph —
// Start -> a local passthrough seed node -> the REAL, unmodified Task 6
// NewCropDisambiguationInterruptNode() — rather than the full T8
// NewProcurementGraph. This is a deliberate test-isolation choice, not a
// defect workaround: NewProcurementGraph's extract_slots/validate_slots/
// crop_gate nodes previously could not complete a real runner.Run() at all
// (a jsonschema-go v0.4.3 bug, https://github.com/google/jsonschema-go/
// issues/23, triggered by workflow.NewFunctionNodeFromState's automatic
// struct output schema) — that has since been fixed (graph_nodes.go now
// builds these nodes with workflow.NewFunctionNode instead; see
// TestNewProcurementGraph_RealRunner_PausesAtCropDisambiguation in
// graph_test.go for the permanent regression guard proving the full real
// graph now runs end to end through a real scheduler). This file keeps the
// minimal substitute graph anyway because its own job is testing
// ClaimAndDispatchTurnViaGraph's ledger-translation logic in isolation, not
// re-proving the graph's own correctness — graph_test.go already owns that.
//
// This substitution reuses the exact pattern graph_interrupt_test.go's own
// TestCropDisambiguationInterruptNode_PauseResume_ViaRunner already
// established and documented (a local string->CropGateOutput passthrough
// standing in for the upstream extract/validate/crop_gate chain) — it is
// not a new precedent invented for this task.
func newPendingInterruptRunner(t *testing.T) *runner.Runner {
	t.Helper()

	candidates := pendingCropGateCandidates()
	// Emitting (not workflow.NewFunctionNode) so this seed can also write
	// graph_turn_progress via emitGraphTurnProgress — mirroring the real
	// NewCropGateNode's own CropPending/CropCandidates write (graph_nodes.go)
	// so graphTurnExecutor's interrupt branch (Sub-project 3b, Task 6) has
	// real progress to compose a genuine crop-disambiguation question from,
	// not just the CropGateOutput the interrupt node itself forwards.
	seedPendingCropGateOutput := workflow.NewEmittingFunctionNode(
		"test_seed_crop_gate_output_pending",
		func(ctx agent.Context, _ string, emit func(*session.Event) error) (CropGateOutput, error) {
			if err := emitGraphTurnProgress(ctx, emit, graphTurnProgress{CropPending: true, CropCandidates: candidates}); err != nil {
				return CropGateOutput{}, err
			}
			return CropGateOutput{Pending: true, Candidates: candidates}, nil
		},
		workflow.NodeConfig{},
	)
	interruptNode := NewCropDisambiguationInterruptNode()

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_dispatch_test_interrupt_chain",
		Edges: workflow.Chain(workflow.Start, seedPendingCropGateOutput, interruptNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// newNotPendingRunner mirrors newPendingInterruptRunner but seeds a
// Pending=false CropGateOutput, so the real (unmodified) interrupt node
// passes straight through without pausing (graph_interrupt.go: `if
// !in.Pending { return in, nil }`) — the normal-completion counterpart used
// by TestClaimAndDispatchTurnViaGraph_NormalCompletion_TerminalizesAsSucceeded
// below.
func newNotPendingRunner(t *testing.T) *runner.Runner {
	t.Helper()

	seedNotPendingCropGateOutput := workflow.NewFunctionNode(
		"test_seed_crop_gate_output_not_pending",
		func(_ agent.Context, _ string) (CropGateOutput, error) {
			return CropGateOutput{Pending: false}, nil
		},
		workflow.NodeConfig{},
	)
	interruptNode := NewCropDisambiguationInterruptNode()
	terminalNode := workflow.NewEmittingFunctionNode(
		"test_not_pending_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, nil, nil, nil)
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_dispatch_test_normal_completion_chain",
		Edges: workflow.Chain(workflow.Start, seedNotPendingCropGateOutput, interruptNode, terminalNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// newActivityEmittingRunner builds a *runner.Runner over a MINIMAL graph —
// Start -> a local node that emits activityEvents Activity events via the
// same emitActivityEvent helper extract_slots itself calls (Task 3) ->
// interruptNode — following newNotPendingRunner's exact construction
// pattern above. This file's existing newNotPendingRunner/
// newPendingInterruptRunner helpers never activate extract_slots (they
// build a hand-rolled chain that never calls emitActivityEvent), so
// neither can exercise the new translation branch under test here.
//
// activityEvents was hardcoded to 1 until Sub-project 1 Task 3 needed a
// producer that overruns a small activity channel
// (TestRunGraphOutcome_ActivitySendNeverBlocks). Parameterised rather than
// forked into a near-identical sibling helper, per CLAUDE.md §15.
func newActivityEmittingRunner(t *testing.T, activityEvents int) *runner.Runner {
	t.Helper()

	emitNode := workflow.NewEmittingFunctionNode(
		"test_emit_activity_event",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (CropGateOutput, error) {
			for i := 1; i <= activityEvents; i++ {
				// Status／Label 逐字照 twoPhaseActivity 的 completion 分支填——
				// 這支 fixture 自建 payload、不經過那支 helper，兩邊漂掉時
				// 本檔的形狀斷言會變成在測一個 production 不存在的形狀
				// （twoPhaseActivity 本身的行為由 graph_nodes_activity_test.go
				// 直接釘住）。
				if err := emitActivityEvent(ctx, emit, ActivityEventPayload{
					RowID: int64(i), Attempt: 1, MaxAttempts: 2, RetryGroupKey: int64(i),
					Code: string(domain.StepExtractSlots), Text: "正在讀你的需求，抓品項、數量和交期",
					Label:  domain.StepLabels[domain.StepExtractSlots],
					Status: codemeta.ProcurementStepStatusSucceeded.Value,
				}); err != nil {
					return CropGateOutput{}, err
				}
			}
			return CropGateOutput{Pending: false}, nil
		},
		workflow.NodeConfig{},
	)
	interruptNode := NewCropDisambiguationInterruptNode()
	terminalNode := workflow.NewEmittingFunctionNode(
		"test_activity_emitting_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, nil, nil, nil)
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_dispatch_test_activity_event_chain",
		Edges: workflow.Chain(workflow.Start, emitNode, interruptNode, terminalNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// panickingGraphRunnerNode builds a *runner.Runner whose single node
// panics as soon as it runs — the graph-path counterpart of
// turn_claim_dispatch_test.go's panickingExtractor, proving
// ClaimAndDispatchTurnViaGraph now gets the same panic-recovery guarantee
// the legacy path already had, via the shared dispatchTurnViaExecutor
// (Task 2) instead of a duplicate implementation this task deliberately
// does not write.
func panickingGraphRunner(t *testing.T) *runner.Runner {
	t.Helper()

	panicNode := workflow.NewFunctionNode(
		"test_panic_node",
		func(_ agent.Context, _ any) (CropGateOutput, error) {
			panic("boom inside graph executor")
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_dispatch_test_panic_chain",
		Edges: workflow.Chain(workflow.Start, panicNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// TestClaimAndDispatchTurnViaGraph_BackgroundPanic_DoesNotHangCaller is new
// coverage this task adds — before this task, ClaimAndDispatchTurnViaGraph
// had no dispatch-decoupling or panic-recovery at all (its own doc comment
// said so explicitly), so a panicking node would have crashed the whole
// process. Mirrors TestClaimAndDispatchTurn_BackgroundPanic_DoesNotHangCaller
// (turn_claim_dispatch_test.go) at this entry point.
func TestClaimAndDispatchTurnViaGraph_BackgroundPanic_DoesNotHangCaller(t *testing.T) {
	r := panickingGraphRunner(t)
	store := newFakeRunStore()

	type outcome struct {
		res SubmitTurnResult
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		res, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{BuyerID: 42, ClientRequestID: "req-panic-1", Message: "我要買蘋果"}, GraphControllerDeps{
			RunStore: store, RevisionStore: newFakeRepo(),
			Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
		}, TurnStreamHooks{})
		ch <- outcome{res, err}
	}()

	select {
	case got := <-ch:
		require.Error(t, got.err, "panic 之後呼叫端必須拿到錯誤，不是永遠卡住")
	case <-time.After(5 * time.Second):
		t.Fatal("ClaimAndDispatchTurnViaGraph 沒有在背景 panic 之後返回——呼叫端被卡住了")
	}

	require.Len(t, store.terminals, 1)
	require.Equal(t, domain.RunStatusFailed(), store.terminals[0].Status)
	// RUN_FAILED 的 public payload 必須自描述地帶著 failureReason：gateway 靠它把
	// 這筆 ledger event 投影成原生 AG-UI RUN_ERROR 的 reason（doc57 §4.2）。舊版
	// 這裡是字面 `{}`，消費端因此只知道「失敗了」而不知道為什麼。
	require.JSONEq(t,
		`{"failureReason":"`+codemeta.ProcurementRunFailureReasonTurnPipelineError.Value+`"}`,
		string(store.terminals[0].PublicPayloadJSON))
}

// TestClaimAndDispatchTurnViaGraph_NormalCompletion_TerminalizesAsSucceeded
// proves the non-interrupt path also correctly closes out the run it
// claimed — TerminalizeRun must still be called (as SUCCEEDED), not skipped,
// when the run never pauses. Without this, a normal-completion run would sit
// forever in the non-terminal state ClaimRun leaves it in — the same
// orphaned-row failure mode this rewrite exists to fix, just for the
// completion path instead of the interrupt path.
func TestClaimAndDispatchTurnViaGraph_NormalCompletion_TerminalizesAsSucceeded(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-normal-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Empty(t, store.interrupts, "no CreateInterrupt call expected on a non-pausing run")
	require.Len(t, store.terminals, 1, "expected exactly one store.TerminalizeRun call")
	got := store.terminals[0]
	require.Equal(t, domain.RunStatusSucceeded(), got.Status)
}

// TestClaimAndDispatchTurnViaGraph_WritesControllerKindGraph is the graph-path
// counterpart of TestClaimAndDispatchTurn_WritesControllerKindDurable.
func TestClaimAndDispatchTurnViaGraph_WritesControllerKindGraph(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-controllerkind-graph-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Len(t, store.runs, 1)
	for _, run := range store.runs {
		require.Equal(t, domain.ControllerKindGraph, run.ControllerKind)
	}
}

// TestClaimAndDispatchTurnViaGraph_PayloadRuntimeType_IsStructNotMap
// empirically resolves the task brief's second flagged unknown: what Go
// type actually reaches fc.Args["payload"] on the event
// ClaimAndDispatchTurnViaGraph reads out of r.Run's event stream.
//
// Static evidence (recorded here, not trusted on its own): workflow.
// NewRequestInputEvent (request_input.go:85, ADK v2.2.0) stores
// req.Payload — the caller's raw Go value — directly into
// args["payload"], with no json.Marshal. That event object is the same
// *session.Event the runner later yields: runner.go:346's
// r.sessionService.AppendEvent(ctx, storedSession, event) runs BEFORE
// runner.go:350's yield(event, nil), passing the identical pointer: for
// session.InMemoryService (used here and by
// TestClaimAndDispatchTurnViaGraph_FlagEnabled_CropDisambiguation_
// CreatesInterrupt below), AppendEvent's eventCopy
// (inmemory.go:224-243) only shallow-copies LLMResponse for its own
// stored history — it never mutates the original event, so no
// JSON round-trip happens anywhere on this path.
//
// This test does not merely trust that reasoning — it drives a real
// *runner.Runner (via newPendingInterruptRunner, which chains the REAL,
// unmodified Task 6 NewCropDisambiguationInterruptNode) and asserts the
// concrete runtime type of fc.Args["payload"], failing loudly with the
// actual %T if the assumption above is ever wrong.
func TestClaimAndDispatchTurnViaGraph_PayloadRuntimeType_IsStructNotMap(t *testing.T) {
	r := newPendingInterruptRunner(t)

	var sawPayload bool
	for ev, err := range r.Run(t.Context(), "u1", "s1",
		&genai.Content{Role: genai.RoleUser, Parts: []*genai.Part{{Text: "go"}}},
		agent.RunConfig{},
	) {
		require.NoError(t, err)
		if ev == nil || len(ev.LongRunningToolIDs) == 0 || ev.Content == nil {
			continue
		}
		for _, p := range ev.Content.Parts {
			if p.FunctionCall == nil {
				continue
			}
			raw, ok := p.FunctionCall.Args["payload"]
			require.True(t, ok, "expected a \"payload\" key in FunctionCall.Args")
			t.Logf("fc.Args[%q] runtime type = %T", "payload", raw)

			_, isMap := raw.(map[string]any)
			require.False(t, isMap, "fc.Args[\"payload\"] must NOT be map[string]any on this in-process path")

			payload, isStruct := raw.(CropDisambiguationInterruptPayload)
			require.True(t, isStruct, "expected fc.Args[\"payload\"] to be the literal CropDisambiguationInterruptPayload value, got %T", raw)
			require.Equal(t, cropDisambiguationInterruptKind, payload.Kind)
			sawPayload = true
		}
	}
	require.True(t, sawPayload, "expected the pause event to carry a FunctionCall payload")
}

// TestClaimAndDispatchTurnViaGraph_FlagEnabled_CropDisambiguation_CreatesInterrupt
// is the end-to-end proof: with the flag on, ClaimAndDispatchTurnViaGraph
// must (1) claim a real run via store.ClaimRun, (2) translate a
// crop_disambiguation pause into exactly one store.CreateInterrupt call
// carrying that run's real, non-zero RunID (the regression guard for the
// RunID:0 orphaned-interrupt-row defect this rewrite fixes — previously
// hardcoded to 0, unresolvable by any later query), and (3) terminalize the
// run as WAITING_FOR_INPUT via store.TerminalizeRun.
//
// It drives newPendingInterruptRunner's minimal graph (Start -> pending
// seed -> the real NewCropDisambiguationInterruptNode), not
// NewProcurementGraph itself — see newPendingInterruptRunner's doc comment
// for why this file keeps that isolation even though the full graph can now
// run for real.
func TestClaimAndDispatchTurnViaGraph_FlagEnabled_CropDisambiguation_CreatesInterrupt(t *testing.T) {
	r := newPendingInterruptRunner(t)
	store := newFakeRunStore()

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-crop-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Len(t, store.runs, 1, "expected exactly one store.ClaimRun call")
	var claimedRun repository.Run
	for _, run := range store.runs {
		claimedRun = *run
	}
	require.NotZero(t, claimedRun.ID, "sanity check: fakeRunStore must have assigned a non-zero run ID")

	require.Len(t, store.interrupts, 1, "expected exactly one store.CreateInterrupt call on crop disambiguation pause")
	got := store.interrupts[0]
	require.Equal(t, claimedRun.ID, got.RunID,
		"CreateInterrupt must carry the real RunID from ClaimRun, not the old hardcoded 0 placeholder")
	require.Equal(t, "CLARIFICATION", got.InterruptKind,
		"ledger InterruptKind must be the existing interruptOutcomeFor sentinel value, not the graph payload's Kind field")
	require.Equal(t, "v1", got.ResponseSchemaVersion)

	require.Len(t, store.terminals, 1, "expected exactly one store.TerminalizeRun call")
	gotTerminal := store.terminals[0]
	require.Equal(t, claimedRun.ID, gotTerminal.RunID)
	require.Equal(t, domain.RunStatusWaiting(), gotTerminal.Status,
		"a crop-disambiguation pause must terminalize the run as WAITING_FOR_INPUT, not SUCCEEDED")
}

// newTerminalCompletionRunner builds a minimal graph reaching a real
// terminal node — Start -> a seed node writing a fully-complete
// graph_turn_progress -> the real ready_to_finalize_terminal body
// (runGraphTerminalNode) — proving graphTurnExecutor's persistence step
// end to end without needing the real controller/a fake LLM (same
// isolation precedent as newTerminalMarkerRunner in graph_terminal_test.go
// and this file's own newNotPendingRunner/newPendingInterruptRunner).
//
// pricePreviewChecker is threaded straight into runGraphTerminalNode — in
// production it is bound at graph-construction time (NewProcurementGraph,
// graph.go), not passed through ClaimAndDispatchTurnViaGraph, so this
// helper has to accept it directly rather than relying on any dispatch-time
// argument. 2026-08-31 (Sub-project 3a Task 4): every pre-existing caller
// only needed the persistence path, not the price-preview nudge, so they
// keep passing nil (identical behavior to before this parameter existed);
// the parity test in turn_test.go is the first caller that needs a real,
// non-nil checker to prove hasPreview=false produces the same NextAction
// nudge on both the legacy and graph paths.
func newTerminalCompletionRunner(t *testing.T, progress graphTurnProgress, pricePreviewChecker PricePreviewChecker) *runner.Runner {
	t.Helper()

	seedNode := workflow.NewEmittingFunctionNode(
		"test_seed_progress_for_completion",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return nil, emitGraphTurnProgress(ctx, emit, progress)
		},
		workflow.NodeConfig{},
	)
	terminalNode := workflow.NewEmittingFunctionNode(
		"test_ready_to_finalize_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, pricePreviewChecker, nil, nil)
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_dispatch_test_completion_chain",
		Edges: workflow.Chain(workflow.Start, seedNode, terminalNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// TestClaimAndDispatchTurnViaGraph_NormalCompletion_PersistsRealResult
// proves this task's core deliverable: a normal completion now produces a
// real, persisted TurnResult (non-empty Reply, a real RevisionID from a
// real AppendRevision call) instead of the hard-coded TurnResult{} this
// task removes.
func TestClaimAndDispatchTurnViaGraph_NormalCompletion_PersistsRealResult(t *testing.T) {
	// domain.Slots' required set is exactly Item/DeliveryCity/Unit
	// (domain/slots.go's requiredSlots).
	full := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")}
	require.Empty(t, full.MissingRequired(), "sanity check: this fixture must be genuinely complete")

	r := newTerminalCompletionRunner(t, graphTurnProgress{NextSlots: full}, nil)
	store := newFakeRunStore()
	repo := newFakeRepo()

	out, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-persist-1", Message: "都給你了",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.NotEmpty(t, out.Result.Reply, "expected a real composed reply, not the old hard-coded empty TurnResult")
	require.NotZero(t, out.Result.RevisionID, "expected a real AppendRevision-produced RevisionID")
	require.Empty(t, out.Result.MissingRequired)
	require.Equal(t, 1, repo.createRequestCalls, "a new conversation must create exactly one request row")
}

// TestClaimAndDispatchTurnViaGraph_ClaimRunReplay_ReplaysRealContent
// REPLACES TestClaimAndDispatchTurnViaGraph_ClaimRunReplay_ReturnsRunResultUnavailable
// (Sub-project 1): before this task, a graph-produced run's
// ResultRevisionID was always 0, so any repeat claim short-circuited into
// ErrRunResultUnavailable. Now that AppendRevision runs for real, a repeat
// claim against the same (BuyerID, ClientRequestID) must replay the exact
// same content instead — closing Sub-project 1 spec §5's flagged gap.
func TestClaimAndDispatchTurnViaGraph_ClaimRunReplay_ReplaysRealContent(t *testing.T) {
	full := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市"), Unit: strPtr("箱")}
	r := newTerminalCompletionRunner(t, graphTurnProgress{NextSlots: full}, nil)
	store := newFakeRunStore()
	repo := newFakeRepo()
	// runStore.FindByIdempotencyKey must observe what revStore.AppendRevision
	// just recorded — in production both usecase.RevisionStore and
	// usecase.RunClaimStore are the same *repository.Repo singleton
	// (internal/procurement/module.go), but fakeRunStore and fakeRepo are
	// intentionally separate fakes for different concerns, so this wiring
	// reproduces that "same instance" topology, same as every other
	// real-replay test in this file (turn_claim_dispatch_test.go's
	// `runStore.revisionStore = repo` precedent).
	store.revisionStore = repo
	in := TurnInput{BuyerID: 42, ClientRequestID: "req-replay-real-1", Message: "都給你了"}

	first, err := ClaimAndDispatchTurnViaGraph(t.Context(), in, GraphControllerDeps{
		RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)
	require.NotEmpty(t, first.Result.Reply)

	second, err := ClaimAndDispatchTurnViaGraph(t.Context(), in, GraphControllerDeps{
		RunStore: store, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err, "a repeat claim on a graph-produced run must now replay successfully, not error")
	require.Equal(t, first.Result.Reply, second.Result.Reply)
	require.Equal(t, first.Result.RevisionID, second.Result.RevisionID)
	require.Len(t, store.runs, 1, "a replayed claim must not create a second ledger run")
}

// TestClaimAndDispatchTurnViaGraph_ExistingRequest_ContinuesConversation
// is the multi-turn continuation proof at this layer (the testcontainers
// integration test in Task 8 additionally proves it against a real
// Postgres RevisionStore): a second turn against the same RequestID sees
// the first turn's recorded slots reflected in its own composed result.
func TestClaimAndDispatchTurnViaGraph_ExistingRequest_ContinuesConversation(t *testing.T) {
	store := newFakeRunStore()
	repo := newFakeRepo()

	r1 := newTerminalCompletionRunner(t, graphTurnProgress{NextSlots: domain.Slots{Item: item("蘋果")}}, nil)
	out1, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-continue-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r1, ResumeRunner: ResumeRunner{Runner: r1}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)
	require.NotZero(t, out1.Result.RequestID)
	require.NotEmpty(t, out1.Result.MissingRequired, "sanity check: this fixture must still be missing required slots")

	// Second turn: same RequestID, a seed node that carries the item
	// forward (progress.NextSlots would, in the real graph, come from
	// validate_slots' Apply over whatever resolveGraphTurnState read back
	// as current_slots — this test seeds the equivalent merged result
	// directly, since it isolates graphTurnExecutor's persistence/
	// continuation wiring, not validate_slots' own Apply logic, which
	// Task 3's tests already cover).
	merged := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市")}
	r2 := newTerminalCompletionRunner(t, graphTurnProgress{NextSlots: merged}, nil)
	out2, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		RequestID: out1.Result.RequestID, BuyerID: 42, ClientRequestID: "req-continue-2", Message: "台北市",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r2, ResumeRunner: ResumeRunner{Runner: r2}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Equal(t, out1.Result.RequestID, out2.Result.RequestID, "must be the same conversation, not a new one")
	require.Equal(t, "蘋果", *out2.Result.Slots.Item, "the second turn's result must still carry the first turn's recorded item")
	require.Equal(t, "台北市", *out2.Result.Slots.DeliveryCity)
	require.Equal(t, strconv.FormatInt(out1.Result.RequestID, 10)+"-rev"+strconv.FormatInt(out1.Result.RevisionID, 10),
		out2.Result.DialoguePlan.StateVersion,
		"StateVersion must reflect the tip revision resolveGraphTurnState actually read back from history — proves real read-back, not just a shared RequestID")
	require.Equal(t, 1, repo.createRequestCalls, "the second turn must not create a second request row")
}

// TestClaimAndDispatchTurnViaGraph_NormalCompletion_EmitsTerminalResultEvent
// pins Finding 1 of the 2026-08-30 final whole-branch review: before this
// fix, graphTurnExecutor's returned turnPipelineOutcome.events was always
// nil, so SubmitTurnStream's `for _, event := range out.Events { emit(event) }`
// loop (procurement_handler.go) never emitted a terminal AG-UI frame
// (RUN_FINISHED/RUN_ERROR) for a graph-path run — the buyer would get
// RunAccepted and then the stream would just close, a protocol violation on
// the one new production-reachable path this branch adds. This test proves
// at the usecase layer that a normal completion's returned
// SubmitTurnResult.Events now contains exactly one TurnEventResult-kind
// event, mirroring HandleStream's own terminal-event guarantee (turn.go's
// `yield(TurnEvent{Kind: TurnEventResult, Result: result}, nil)`).
func TestClaimAndDispatchTurnViaGraph_NormalCompletion_EmitsTerminalResultEvent(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()

	out, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-events-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Len(t, out.Events, 1, "expected exactly one terminal event so SubmitTurnStream has something to emit")
	require.Equal(t, TurnEventResult, out.Events[0].Kind)
}

// TestClaimAndDispatchTurnViaGraph_ActivityEvent_AppendsToLedger proves the
// new translation branch: an Activity session.Event emitted by a graph node
// (tagged via emitActivityEventFuncName, same shape extract_slots itself
// produces) must be translated into a store.AppendRunEvent call carrying
// AGENT_ACTIVITY_SNAPSHOT and the payload's code/text fields — not silently
// dropped by the existing crop-disambiguation-only event loop.
func TestClaimAndDispatchTurnViaGraph_ActivityEvent_AppendsToLedger(t *testing.T) {
	r := newActivityEmittingRunner(t, 1)
	store := newFakeRunStore()

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-activity-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	var activityEvents []repository.AppendRunEventInput
	for _, e := range store.events {
		if e.EventType == codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value {
			activityEvents = append(activityEvents, e)
		}
	}
	require.Len(t, activityEvents, 1, "expected exactly one AGENT_ACTIVITY_SNAPSHOT ledger event")

	var payload map[string]any
	require.NoError(t, json.Unmarshal(activityEvents[0].PublicPayloadJSON, &payload))
	require.Equal(t, string(domain.StepExtractSlots), payload["code"])
	require.NotEmpty(t, payload["text"])

	// CLAUDE.md §4: any ID crossing a JSON boundary must be a string, not
	// a number, to avoid JS precision loss. ledgerEventTypeForActivity
	// formats both via strconv.FormatInt — assert that here so a
	// regression back to a bare int64/float64 in the payload is caught.
	_, ok := payload["rowId"].(string)
	require.True(t, ok, "rowId must be a JSON string per CLAUDE.md §4")
	_, ok = payload["retryGroupKey"].(string)
	require.True(t, ok, "retryGroupKey must be a JSON string per CLAUDE.md §4")

	// T23：graph 路徑的列也要帶 status 與 server 端固定 label。這條 rail 有兩個
	// producer（graph 節點與候選搜尋 run），只要其中一邊漏填，前端就會拿到一
	// 半有狀態、一半沒有的混合清單。
	require.Equal(t, codemeta.ProcurementStepStatusSucceeded.Value, payload["status"],
		"completion 事件的 status 必須是 SUCCEEDED")
	require.Equal(t, domain.StepLabels[domain.StepExtractSlots], payload["label"],
		"label 必須來自 domain.StepLabels，不得由 consumer 自備第二份對照表")
	// graph 那三個節點都是 top-level、沒有計數概念——缺席比補零值誠實。
	require.NotContains(t, payload, "parentRowId")
	require.NotContains(t, payload, "count")
}

// TestLedgerEventTypeForActivity_NestedRowWithCount 補上另一半形狀：巢狀子列
// 與計數只在有值時出現，且都是 JSON 字串（id 與計數依 API/Proto 規範 §6）。
func TestLedgerEventTypeForActivity_NestedRowWithCount(t *testing.T) {
	count := 12
	eventType, raw := ledgerEventTypeForActivity(ActivityEventPayload{
		RowID: 9002, Attempt: 1, MaxAttempts: 1, RetryGroupKey: 9002,
		Code: string(domain.StepSearchPlatform), Label: domain.StepLabels[domain.StepSearchPlatform],
		Status:      codemeta.ProcurementStepStatusSucceeded.Value,
		ParentRowID: 9001, Count: &count,
	})
	require.Equal(t, codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value, eventType)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "9001", payload["parentRowId"], "parentRowId 必須是字串 id")
	require.Equal(t, "12", payload["count"], "count 必須是字串（計數依規範 §6）")
	require.Equal(t, "翻平台上的賣家", payload["label"])
	require.Equal(t, "", payload["text"], "候選搜尋沒有 narrator，text 恆空、由 label 承接顯示")
}

// TestLedgerEventTypeForActivity_ZeroCountIsNotAbsent 釘住指標型別的意義：
// 「查到 0 家」是有效結果，不能跟「這一列沒有計數概念」在 wire 上長得一樣。
func TestLedgerEventTypeForActivity_ZeroCountIsNotAbsent(t *testing.T) {
	zero := 0
	_, raw := ledgerEventTypeForActivity(ActivityEventPayload{
		RowID: 9003, RetryGroupKey: 9003, Code: string(domain.StepSearchExternal),
		Status: codemeta.ProcurementStepStatusSucceeded.Value, Count: &zero,
	})
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "0", payload["count"])
}

// TestClaimAndDispatchTurnViaGraph_ActivityEvent_AppendFailure_DoesNotAbortDispatch
// pins the non-blocking discipline spec'd for this branch — mirroring
// turn.go:952-960's existing precedent for the non-graph dispatch path and
// Task 4's crop_gate fix for the emit side: an AppendRunEvent failure must
// be logged and continued past, never propagated as this function's error.
func TestClaimAndDispatchTurnViaGraph_ActivityEvent_AppendFailure_DoesNotAbortDispatch(t *testing.T) {
	r := newActivityEmittingRunner(t, 1)
	store := newFakeRunStore()
	store.appendErr = errors.New("db down")

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-activity-2", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err, "AppendRunEvent failure must not abort graph dispatch (spec §2.5(b), same discipline as turn.go:952-960)")
}

// TestRunGraphOutcome_ActivitySendNeverBlocks pins the producer half of the
// live-activity path: runGraphOutcome's send onto the per-request activity
// channel is non-blocking, so a consumer that never reads cannot stall graph
// execution.
//
// The subject is runGraphOutcome's `default:` arm, NOT dispatchTurnViaExecutor
// — it is driven through graphTurnExecutor (its production caller) only so
// resolveGraphTurnState builds a real state map, and deliberately not through
// ClaimAndDispatchTurnViaGraph, whose select loop would drain the channel and
// make it impossible to fill.
//
// The event count MUST exceed the channel's capacity, otherwise the
// `default:` arm never executes and this test would pass even with the arm
// deleted — a scheduler-luck detector rather than a test. With 5 events
// against a capacity of 2, the send for events 3..5 has nowhere to go:
// without `default:` those sends block forever and this test times out.
func TestRunGraphOutcome_ActivitySendNeverBlocks(t *testing.T) {
	const emittedActivityEvents = 5
	// Capacity deliberately far below emittedActivityEvents — see the doc
	// comment: this is what makes the non-blocking arm load-bearing.
	activityCh := make(chan []byte, 2)

	r := newActivityEmittingRunner(t, emittedActivityEvents)
	store := newFakeRunStore()
	exec := graphTurnExecutor(r, newFakeRepo(), nil, nil, nil, activityCh, nil, nil)

	done := make(chan error, 1)
	go func() {
		_, err := exec(t.Context(), repository.Run{ID: 1, BuyerID: 42, Version: 1}, TurnInput{
			BuyerID: 42, ClientRequestID: "req-activity-nonblocking-1", Message: "我要買蘋果",
		}, store)
		done <- err
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("runGraphOutcome blocked sending onto a full activity channel — the non-blocking default arm is gone")
	}

	require.Len(t, activityCh, cap(activityCh),
		"the channel must be full — otherwise this test never overran it and proves nothing")

	var ledgered int
	for _, e := range store.events {
		if e.EventType == codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value {
			ledgered++
		}
	}
	require.Equal(t, emittedActivityEvents, ledgered,
		"dropping a live payload must not drop the ledger row — the ledger is the complete record, the channel is best-effort")
}

// TestClaimAndDispatchTurnViaGraph_NewConversation_SessionIDDerivesFromResolvedRequestID
// is the regression guard for the ProcurementSessionID(in.RequestID) latent
// bug (spec §4.2): every brand-new conversation has in.RequestID==0, so
// deriving the ADK session ID from the raw input (not the resolved,
// just-created request ID) would collide every new conversation from the
// same buyer onto the identical session. Proven by having the graph itself
// report its own session ID as an Activity event's text (reusing the
// existing, already-tested Activity-event-to-ledger translation — no new
// capture mechanism needed) and asserting it is NOT "0" (ProcurementSessionID's
// value for the unresolved in.RequestID==0).
func newSessionIDReportingRunner(t *testing.T) *runner.Runner {
	t.Helper()

	reportNode := workflow.NewEmittingFunctionNode(
		"test_report_session_id",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (CropGateOutput, error) {
			sessionID := ctx.Session().ID()
			if err := emitActivityEvent(ctx, emit, ActivityEventPayload{
				RowID: 1, Attempt: 1, MaxAttempts: 1, RetryGroupKey: 1,
				Code: string(domain.StepExtractSlots), Text: sessionID,
			}); err != nil {
				return CropGateOutput{}, err
			}
			return CropGateOutput{Pending: false}, nil
		},
		workflow.NodeConfig{},
	)
	interruptNode := NewCropDisambiguationInterruptNode()
	terminalNode := workflow.NewEmittingFunctionNode(
		"test_session_id_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, nil, nil, nil)
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_dispatch_test_session_id_chain",
		Edges: workflow.Chain(workflow.Start, reportNode, interruptNode, terminalNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

func TestClaimAndDispatchTurnViaGraph_NewConversation_SessionIDDerivesFromResolvedRequestID(t *testing.T) {
	r := newSessionIDReportingRunner(t)
	store := newFakeRunStore()

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-sessionid-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	var sawSessionID string
	for _, e := range store.events {
		if e.EventType == codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value {
			var payload map[string]any
			require.NoError(t, json.Unmarshal(e.PublicPayloadJSON, &payload))
			sawSessionID, _ = payload["text"].(string)
		}
	}
	require.NotEmpty(t, sawSessionID)
	require.NotEqual(t, ProcurementSessionID(0), sawSessionID,
		"a brand-new conversation must not reuse ProcurementSessionID(0) — every new conversation from the same buyer would otherwise collide on one ADK session")
}

// TestClaimAndDispatchTurnViaGraph_QuotaExceeded_BlocksBeforeAnyGraphExecution
// pins the fail-closed short-circuit: ReserveQuota returning reserved=false
// must stop the turn before ClaimRun's own run row does any graph work —
// r is passed nil, so if quota-checking ever ran AFTER resolveGraphTurnState/
// r.Run() started, this test would panic on a nil-pointer Run() call instead
// of cleanly returning QuotaExceededError (same nil-runner proof technique
// TestClaimAndDispatchTurnViaGraph_OpenInterruptExists_BlockedByGuard already
// uses in this file for an analogous "never reached the graph" claim).
func TestClaimAndDispatchTurnViaGraph_QuotaExceeded_BlocksBeforeAnyGraphExecution(t *testing.T) {
	store := newFakeRunStore()
	repo := newFakeRepo()
	client := &fakeEntitlementClient{entitlement: testEntitlement()}
	quota := &fakeQuotaReserver{reserved: false}

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-quota-exceeded-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: nil, ResumeRunner: ResumeRunner{Runner: nil}, RfqConfirmationReader: nil, Entitlement: client, Quota: quota,
	}, TurnStreamHooks{})

	var quotaErr *QuotaExceededError
	require.ErrorAs(t, err, &quotaErr)
	require.Equal(t, testEntitlement().MonthlyAnswerLimit, quotaErr.Limit)
	require.Zero(t, repo.createRequestCalls, "quota must be checked before resolveGraphTurnState ever creates a request row")
	require.Equal(t, 1, quota.reserveCalls)
	require.Zero(t, quota.releaseCalls, "nothing was ever reserved successfully — there is nothing to release")

	require.Len(t, store.terminals, 1,
		"a quota-blocked run must still be terminalized — otherwise it sits in QUEUED forever with no lazy reaper (2026-08-31 final review finding 1)")
	require.Equal(t, domain.RunStatusFailed(), store.terminals[0].Status)
}

// TestClaimAndDispatchTurnViaGraph_EntitlementLookupFails_AbortsBeforeGraph
// mirrors the exceeded-quota short-circuit for the other fail-closed branch
// (turn.go:365-369's "查不到方案就不知道該用哪個上限，放行等於無限額度").
func TestClaimAndDispatchTurnViaGraph_EntitlementLookupFails_AbortsBeforeGraph(t *testing.T) {
	store := newFakeRunStore()
	repo := newFakeRepo()
	client := &fakeEntitlementClient{err: errors.New("buyer-service unavailable")}
	quota := &fakeQuotaReserver{reserved: true}

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-entitlement-fail-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: nil, ResumeRunner: ResumeRunner{Runner: nil}, RfqConfirmationReader: nil, Entitlement: client, Quota: quota,
	}, TurnStreamHooks{})

	require.Error(t, err)
	require.Zero(t, repo.createRequestCalls)
	require.Zero(t, quota.reserveCalls, "entitlement must be resolved before quota is ever reserved")

	require.Len(t, store.terminals, 1,
		"an entitlement-lookup failure must still terminalize the claimed run (2026-08-31 final review finding 1)")
	require.Equal(t, domain.RunStatusFailed(), store.terminals[0].Status)
}

// TestClaimAndDispatchTurnViaGraph_ResolveGraphTurnStateFails_TerminalizesRun
// pins the one early-return in graphTurnExecutor that, before the
// 2026-08-31 final review fix, did NOT terminalize the claimed run on
// error: unlike the three quota-related early returns immediately above
// resolveGraphTurnState's call site (both tests above this one), a
// resolveGraphTurnState failure just returned the bare error, leaving the
// run stuck in QUEUED forever — no v1 sweeper reclaims it — which makes any
// retry of the same clientRequestId return ErrRunResultUnavailable forever
// (replayTurnRun, turn_run_lifecycle.go).
//
// Failure fixture mirrors TestResolveGraphTurnState_StoreError_Propagates's
// own style: an existing request owned by buyer 42, addressed with the
// wrong buyer ID (999) so GetSlots inside resolveGraphTurnState returns
// repository.ErrNotFound — Entitlement/Quota stay nil so quotaEnabled is
// false and resolveGraphTurnState's own error path is what's under test,
// not the quota branches above it.
func TestClaimAndDispatchTurnViaGraph_ResolveGraphTurnStateFails_TerminalizesRun(t *testing.T) {
	store := newFakeRunStore()
	repo := newFakeRepo()
	req, _, err := repo.CreateRequest(t.Context(), identity.BuyerID(42))
	require.NoError(t, err)

	_, err = ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		RequestID: req.ID, BuyerID: identity.BuyerID(999), // wrong buyer for req.ID
		ClientRequestID: "req-resolve-state-fail-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: nil, ResumeRunner: ResumeRunner{Runner: nil}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.Error(t, err)
	require.ErrorIs(t, err, repository.ErrNotFound)

	require.Len(t, store.terminals, 1,
		"resolveGraphTurnState's error path must terminalize the claimed run — otherwise it sits in QUEUED forever with no lazy reaper (2026-08-31 final review finding)")
	require.Equal(t, domain.RunStatusFailed(), store.terminals[0].Status)
}

// TestClaimAndDispatchTurnViaGraph_NormalCompletion_ConfirmsQuota_DoesNotRelease
// proves the success path: a real reservation gets threaded into
// AppendRevision's Quota* fields and is NOT released — releasing an already-
// confirmed reservation would double-refund the buyer's used quota.
func TestClaimAndDispatchTurnViaGraph_NormalCompletion_ConfirmsQuota_DoesNotRelease(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()
	repo := &recordingAppendRepo{fakeRepo: newFakeRepo()}
	client := &fakeEntitlementClient{entitlement: testEntitlement()}
	quota := &fakeQuotaReserver{reservationID: 4242, reserved: true, confirmedBefore: 3}

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-quota-confirm-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: client, Quota: quota,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Equal(t, int64(4242), repo.lastAppend.QuotaReservationID, "AppendRevision must receive the real reservation ID, not the hard-coded 0")
	require.Equal(t, testEntitlement().PeriodStart, repo.lastAppend.QuotaPeriodStart)
	require.Zero(t, quota.releaseCalls, "a successfully confirmed reservation must not also be released")
}

// TestClaimAndDispatchTurnViaGraph_AppendRevisionFails_ReleasesQuota proves
// the defer's actual job: if persistence fails after a real reservation, the
// reservation must be released, not silently kept (which would permanently
// cost the buyer quota for a turn that produced nothing).
func TestClaimAndDispatchTurnViaGraph_AppendRevisionFails_ReleasesQuota(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()
	repo := &failingAppendRepo{fakeRepo: newFakeRepo(), err: errors.New("db write failed")}
	client := &fakeEntitlementClient{entitlement: testEntitlement()}
	quota := &fakeQuotaReserver{reservationID: 4343, reserved: true}

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-quota-release-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: client, Quota: quota,
	}, TurnStreamHooks{})
	require.Error(t, err)

	require.Equal(t, 1, quota.releaseCalls)
	require.Equal(t, int64(4343), quota.releasedReservationID)
}

// TestClaimAndDispatchTurnViaGraph_NilEntitlementOrQuota_SkipsQuotaEntirely
// proves the paired nil-safe fail-safe (quotaEnabled := entitlementClient !=
// nil && quotaReserver != nil, mirroring HandleStream's own quotaEnabled) —
// every other test in this file that doesn't pass client/quota must keep
// working exactly as before this task, with zero quota interaction.
func TestClaimAndDispatchTurnViaGraph_NilEntitlementOrQuota_SkipsQuotaEntirely(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()
	repo := &recordingAppendRepo{fakeRepo: newFakeRepo()}

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-quota-nil-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)
	require.Zero(t, repo.lastAppend.QuotaReservationID)
	require.True(t, repo.lastAppend.QuotaPeriodStart.IsZero())
}

// TestClaimAndDispatchTurnViaGraph_NormalCompletion_SetsQuotaSnapshot proves
// TurnResult.Quota is actually populated on success — the live-only field
// (turn.go:153-164) HandleStream's own quota tests assert the same way.
func TestClaimAndDispatchTurnViaGraph_NormalCompletion_SetsQuotaSnapshot(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()
	repo := newFakeRepo()
	ent := testEntitlement()
	client := &fakeEntitlementClient{entitlement: ent}
	quota := &fakeQuotaReserver{reservationID: 4444, reserved: true, confirmedBefore: 3}

	out, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-quota-snapshot-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, RfqConfirmationReader: nil, Entitlement: client, Quota: quota,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Equal(t, ent.PlanCode, out.Result.Quota.PlanCode)
	require.Equal(t, ent.MonthlyAnswerLimit, out.Result.Quota.Limit)
	require.Equal(t, ent.MonthlyAnswerLimit-3-1, out.Result.Quota.Remaining, "Remaining = limit - confirmedBefore - this turn's own confirm")
	require.Equal(t, ent.PeriodEnd, out.Result.Quota.ResetAt)
}

// TestClaimAndDispatchTurnViaGraph_CropDisambiguationPause_ConfirmsQuotaAndProducesContent
// REPLACES TestClaimAndDispatchTurnViaGraph_CropDisambiguationPause_ReleasesQuota
// (Sub-project 3a): that test pinned the KNOWN, disclosed divergence from
// HandleStream this task fixes (spec §0.5/§4.4) — a graph-path crop-
// disambiguation pause now composes real buyer-visible content via
// composeGraphTurnResult (the same compose logic a normal completion uses)
// and calls AppendRevision for it, so the reservation gets confirmed like
// HandleStream's own disambiguation-pause branch already does, not released.
func TestClaimAndDispatchTurnViaGraph_CropDisambiguationPause_ConfirmsQuotaAndProducesContent(t *testing.T) {
	r := newPendingInterruptRunner(t)
	store := newFakeRunStore()
	repo := newFakeRepo()
	client := &fakeEntitlementClient{entitlement: testEntitlement()}
	quota := &fakeQuotaReserver{reservationID: 4545, reserved: true}

	out, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-quota-interrupt-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, Entitlement: client, Quota: quota,
	}, TurnStreamHooks{})
	require.NoError(t, err)
	require.NotEmpty(t, out.InterruptID, "sanity check: this fixture must actually pause")

	require.Zero(t, quota.releaseCalls, "a disambiguation pause that produces real content must confirm quota, not release it — matching HandleStream")
	require.NotEmpty(t, out.Result.Reply, "buyer must see the disambiguation question, not an empty reply")
	require.NotNil(t, out.Result.Question)
	require.Equal(t, domain.SlotCropDisambiguation, out.Result.Question.SlotKey)
	require.NotZero(t, out.Result.RevisionID, "the disambiguation question itself must be persisted as a revision")
}

// TestClaimAndDispatchTurnViaGraph_CropDisambiguationPause_AppendRevisionFails_ReleasesQuota
// is the failure-path counterpart: if persisting the disambiguation content
// fails, the reservation must still be released (never silently kept).
func TestClaimAndDispatchTurnViaGraph_CropDisambiguationPause_AppendRevisionFails_ReleasesQuota(t *testing.T) {
	r := newPendingInterruptRunner(t)
	store := newFakeRunStore()
	repo := &failingAppendRepo{fakeRepo: newFakeRepo(), err: errors.New("db write failed")}
	client := &fakeEntitlementClient{entitlement: testEntitlement()}
	quota := &fakeQuotaReserver{reservationID: 4646, reserved: true}

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: identity.BuyerID(42), ClientRequestID: "req-quota-interrupt-fail-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: repo,
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r}, Entitlement: client, Quota: quota,
	}, TurnStreamHooks{})
	require.Error(t, err)

	require.Equal(t, 1, quota.releaseCalls)
	require.Equal(t, int64(4646), quota.releasedReservationID)
}

// TestBuildResumedCropGateOutput 守住 resume 要轉交給圖的欄位。這幾行原本內嵌在
// resumeGraphTurn 裡，而那支函式的既有測試用的 fake runner 忽略傳進去的訊息——
// 實測把 Resolution 拿掉，整個 procurement 套件仍然全綠。抽出來才守得到。
func TestBuildResumedCropGateOutput(t *testing.T) {
	current := domain.Slots{Item: item("高麗菜")}

	t.Run("Item 沿用現況，解析結果轉交", func(t *testing.T) {
		res := domain.NewSelfPricedItemResolution(75, "甘藍-初秋", domain.CategoryAbsent("測試"))

		got := buildResumedCropGateOutput(current, domain.SlotPatch{ItemResolution: res})

		require.Equal(t, item("高麗菜"), got.Item, "挑候選不改寫 item 原話")
		require.Equal(t, res, got.Resolution,
			"不轉交的話買家挑完候選畫面零回饋——crop_resume_reconcile 是唯一能寫回 progress 的地方")
		require.False(t, got.Pending)
	})

	t.Run("逃生艙：Item 改寫成買家描述，自填價一併轉交", func(t *testing.T) {
		res := domain.NewSelfPricedItemResolution(75, "", domain.CategoryAbsent("測試"))

		got := buildResumedCropGateOutput(current, domain.SlotPatch{Item: item("紫蘇"), ItemResolution: res})

		require.Equal(t, item("紫蘇"), got.Item)
		require.Equal(t, res, got.Resolution)
	})

	t.Run("空 patch：兩者都不動", func(t *testing.T) {
		got := buildResumedCropGateOutput(current, domain.SlotPatch{})

		require.Equal(t, item("高麗菜"), got.Item)
		require.Nil(t, got.Resolution)
	})
}

// TestResolveRfqState_FourWayDerivation is the shared-helper-level pin for
// the four cases turn.go/graph_dispatch.go both already had their own
// duplicate copy of (Sub-project 3b, Task 4 dedup) — see
// TestResolveGraphTurnState_Rfq*_Seeds* in graph_dispatch_test.go for the
// pre-existing graph-path-level coverage of the same four cases, now
// exercised through this one shared function instead of two copies.
func TestResolveRfqState_FourWayDerivation(t *testing.T) {
	history := []repository.RevisionWithPatch{
		{Revision: repository.Revision{ID: 501}},
		{Revision: repository.Revision{ID: 502}}, // tip
	}

	t.Run("confirmed and matches tip", func(t *testing.T) {
		rfq := fakeRfqConfirmationReader{snapshot: repository.ConfirmedRfqSnapshot{RequestRevisionID: 502}}
		got := resolveRfqState(context.Background(), rfq, 1, identity.BuyerID(42), history)
		require.Equal(t, domain.RfqConfirmed, got)
	})

	t.Run("confirmed but stale", func(t *testing.T) {
		rfq := fakeRfqConfirmationReader{snapshot: repository.ConfirmedRfqSnapshot{RequestRevisionID: 999}}
		got := resolveRfqState(context.Background(), rfq, 1, identity.BuyerID(42), history)
		require.Equal(t, domain.RfqStale, got)
	})

	t.Run("never confirmed", func(t *testing.T) {
		rfq := fakeRfqConfirmationReader{err: repository.ErrConfirmedRfqSnapshotNotFound}
		got := resolveRfqState(context.Background(), rfq, 1, identity.BuyerID(42), history)
		require.Equal(t, domain.RfqNone, got)
	})

	t.Run("query error degrades to none", func(t *testing.T) {
		rfq := fakeRfqConfirmationReader{err: errors.New("boom")}
		got := resolveRfqState(context.Background(), rfq, 1, identity.BuyerID(42), history)
		require.Equal(t, domain.RfqNone, got)
	})

	t.Run("nil reader degrades to none", func(t *testing.T) {
		got := resolveRfqState(context.Background(), nil, 1, identity.BuyerID(42), history)
		require.Equal(t, domain.RfqNone, got)
	})

	t.Run("empty history degrades to none", func(t *testing.T) {
		rfq := fakeRfqConfirmationReader{snapshot: repository.ConfirmedRfqSnapshot{RequestRevisionID: 502}}
		got := resolveRfqState(context.Background(), rfq, 1, identity.BuyerID(42), nil)
		require.Equal(t, domain.RfqNone, got)
	})
}

// TestClaimAndDispatchTurnViaGraph_CancelRequested_TerminalizesAsCancelled 是
// 「停止鈕真的會停」這件事的第一個機械證據。
//
// 在它之前 CANCEL_REQUESTED 是唯寫的：CancelRun 寫進去，沒有任何執行路徑讀
// 它 ⇒ 買家按停止之後 turn 照跑完、終態照樣寫 SUCCEEDED、CANCELLED 這個終態
// 在全 repo 沒有任何寫入者。
//
// 四條斷言各自擋掉一種「看起來停了其實沒停」的實作：
//   - Status＝CANCELLED：寫 SUCCEEDED／FAILED 都會紅。
//   - TerminalEventType＝RUN_CANCELLED：ledger 的消費端（SSE、ERROR_RECOVERY）
//     靠它分辨這是買家自己要的，不是故障。
//   - FailureReason 為空：取消不是失敗。填了會讓 ERROR_RECOVERY 長出一個假的
//     「可重試錯誤」。
//   - ResultRevisionID＝0：**最有鑑別力的一條**。它證明這一輪真的沒有產出結果
//     ——一個「先跑完再把終態蓋成 CANCELLED」的實作會照樣寫 revision，前三條
//     斷言全過而這條會紅。
func TestClaimAndDispatchTurnViaGraph_CancelRequested_TerminalizesAsCancelled(t *testing.T) {
	r := newNotPendingRunner(t)
	store := newFakeRunStore()
	store.getRunStatus = domain.RunStatusCancelReq()

	_, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
		BuyerID: 42, ClientRequestID: "req-cancel-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: store, RevisionStore: newFakeRepo(),
		Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err, "取消是正常的終止路徑，不是呼叫端要處理的錯誤")

	require.Len(t, store.terminals, 1, "取消仍必須走同一條 TerminalizeRun 收尾，不能讓 run 懸著")
	got := store.terminals[0]
	require.Equal(t, domain.RunStatusCancelled(), got.Status)
	require.Equal(t, codemeta.ProcurementLedgerEventTypeRunCancelled.Value, got.TerminalEventType)
	require.Empty(t, got.FailureReason, "取消不是失敗，不得帶 failureReason")
	require.Zero(t, got.ResultRevisionID,
		"取消的這一輪不得產出 revision——買家明說不要的結果不能變成需求的最新狀態")
}

// TestClaimAndDispatchTurnViaGraph_PipelineErrorWinsOverCancel 守的是回報誠實
// 性：已經發生的錯誤是事實。一個後到的取消不得把 FAILED 蓋成 CANCELLED，否則
// 真正的故障會被記成「使用者自己停的」，而那條線上沒有人會再去看它。
func TestClaimAndDispatchTurnViaGraph_PipelineErrorWinsOverCancel(t *testing.T) {
	r := panickingGraphRunner(t)
	store := newFakeRunStore()
	store.getRunStatus = domain.RunStatusCancelReq()

	type outcome struct {
		res SubmitTurnResult
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		res, err := ClaimAndDispatchTurnViaGraph(t.Context(), TurnInput{
			BuyerID: 42, ClientRequestID: "req-cancel-vs-error-1", Message: "我要買蘋果",
		}, GraphControllerDeps{
			RunStore: store, RevisionStore: newFakeRepo(),
			Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
			RfqConfirmationReader: nil, Entitlement: nil, Quota: nil,
		}, TurnStreamHooks{})
		ch <- outcome{res, err}
	}()

	select {
	case got := <-ch:
		require.Error(t, got.err)
	case <-time.After(5 * time.Second):
		t.Fatal("呼叫端被卡住了")
	}

	require.Len(t, store.terminals, 1)
	require.Equal(t, domain.RunStatusFailed(), store.terminals[0].Status,
		"真實故障不得被一個後到的取消蓋成 CANCELLED")
}
