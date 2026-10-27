//go:build integration

// services/ai-service/internal/procurement/usecase/graph_dispatch_integration_test.go
package usecase

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	sessiondatabase "google.golang.org/adk/v2/session/database"
	"google.golang.org/adk/v2/workflow"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

// itBuyerID moved here from the deleted turn_idempotency_integration_test.go
// (Task 6a of the procurement-graph-cutover plan, 2026-08-31) — still used by
// quote_confirm_integration_test.go.
const itBuyerID = identity.BuyerID(1234)

// setupGraphDispatchSessionService opens ADK's GORM-backed session.Service
// against dsn — the caller passes the same DSN it built the ledger's pgx
// pool from, so both the ledger (*repository.Repo, via pgx) and the ADK
// session (via GORM) read/write the same real Postgres instance for this
// test. This file adds no second dbtest.New/TestMain; it reuses this
// package's existing dbtest_test.go's package-level pg. The database is
// already fully migrated, including
// db/migrations/20260406093514_init_session_tables.sql (ADK's own session
// tables live in this same ai-service database) — no separate AutoMigrate
// call, mirroring internal/agent/di.go's provideSessionService exactly.
func setupGraphDispatchSessionService(t *testing.T, dsn string) session.Service {
	t.Helper()
	svc, err := sessiondatabase.NewSessionService(postgres.Open(dsn))
	require.NoError(t, err)
	return svc
}

// TestClaimAndDispatchTurnViaGraph_Integration_RealPostgres_CropDisambiguation
// is the permanent regression guard for the RunID:0 orphaned-interrupt-row
// defect this rewrite fixes, run against a REAL Postgres database (via
// testcontainers, same mechanism internal/procurement/repository's own
// ClaimRun/TerminalizeRun tests use) rather than fakeRunStore — this is the
// strongest evidence available short of live dev traffic, and unlike a dev
// flag flip it needs no per-run authorization, touches no shared
// environment, and is repeatable in CI. It exercises three things a fake
// store cannot: (1) a real *repository.Repo backed by real SQL (ClaimRun's
// actual transaction, CreateInterrupt's actual FK-less insert,
// TerminalizeRun's actual CAS UPDATE), (2) a real ADK session.Service
// backed by the same database's GORM-managed tables — including
// graph_nodes.go's stateValue JSON-round-trip fallback path, which only
// activates when state comes back from a real JSON-column round-trip, not
// session.InMemoryService's in-process map, and (3) that the interrupt row
// this rewrite creates is genuinely findable by RunID afterward — the exact
// query the old RunID:0 placeholder made permanently impossible.
func TestClaimAndDispatchTurnViaGraph_Integration_RealPostgres_CropDisambiguation(t *testing.T) {
	ctx := context.Background()

	// pg is this package's existing dbtest_test.go package-level instance
	// (own template DB "procurement_usecase_test") — cloned once here so
	// the ledger pool and the ADK session.Service below share the exact
	// same real database, the same way production shares one DSN across
	// module.go's newProcurementPool and internal/agent/di.go's
	// provideSessionService.
	db, drop := pg.Clone(ctx, t)
	defer drop()

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	sessionSvc := setupGraphDispatchSessionService(t, db.DSN)

	extractor := fakeExtractor{patch: domain.SlotPatch{Item: item("蘋果")}}
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates: []domain.CropDisambiguationOption{
			{Label: "富士蘋果"}, {Label: "五爪蘋果"},
		},
	}}

	g, err := NewProcurementGraph(extractor, resolver, seqIDs(1000), nil, &fakeStepNarrator{text: "narrated"}, nil)
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: g,
		SessionService: sessionSvc, AutoCreateSession: true,
	})
	require.NoError(t, err)

	in := TurnInput{BuyerID: identity.BuyerID(90100), ClientRequestID: "integration-crop-1", Message: "我要買蘋果"}

	_, err = ClaimAndDispatchTurnViaGraph(ctx, in, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: repo, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	// Recover the real run ID exactly the way ClaimRun's own idempotency
	// short-circuit would (same key, same hash) — proves the ID through the
	// production code path rather than a schema-dependent raw query, and
	// simultaneously proves ClaimRun's post-terminalize read-back reflects
	// the real, current status.
	claimedRun, isNew, err := repo.ClaimRun(ctx, repository.ClaimRunInput{
		BuyerID: in.BuyerID, ClientRequestID: in.ClientRequestID,
		CommandKind: "SUBMIT_TURN", PayloadHash: canonicalTurnHash(in), RunType: domain.RunTypeTurn,
	})
	require.NoError(t, err)
	require.False(t, isNew, "sanity check: this must hit ClaimAndDispatchTurnViaGraph's existing claim, not create a second run")
	require.NotZero(t, claimedRun.ID, "sanity check: ClaimRun must have persisted a real run row")
	require.Equal(t, domain.RunStatusWaiting(), claimedRun.Status,
		"the claimed run must be terminalized as WAITING_FOR_INPUT, not left non-terminal")

	var interruptCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_agent_run_interrupts WHERE run_id = $1 AND status = 'PENDING'`,
		claimedRun.ID,
	).Scan(&interruptCount))
	require.Equal(t, 1, interruptCount,
		"expected exactly one findable, non-orphaned PENDING interrupt row for the real claimed RunID — "+
			"this is the exact query the old RunID:0 placeholder made permanently impossible to satisfy")
}

// newTerminalCompletionGraph builds a minimal graph reaching a real
// terminal node, over the given real *runner.Runner's session.Service —
// same isolation precedent as this package's unit-test-level
// newTerminalCompletionRunner (graph_dispatch_test.go), here driven
// against a real Postgres session.Service instead of an in-memory one, so
// stateValue's JSON-round-trip fallback path (graph_nodes.go) is actually
// exercised for graph_turn_progress/history, not just current_slots/now/
// request_snapshot as the existing crop-disambiguation integration test
// above exercises.
func newTerminalCompletionGraph(t *testing.T, sessionSvc session.Service, progress graphTurnProgress) *runner.Runner {
	t.Helper()

	seedNode := workflow.NewEmittingFunctionNode(
		"integration_seed_progress",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return nil, emitGraphTurnProgress(ctx, emit, progress)
		},
		workflow.NodeConfig{},
	)
	terminalNode := workflow.NewEmittingFunctionNode(
		"integration_ready_to_finalize_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, nil, nil, nil)
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "graph_dispatch_integration_test_completion_chain",
		Edges: workflow.Chain(workflow.Start, seedNode, terminalNode),
	})
	require.NoError(t, err)

	r, err := runner.New(runner.Config{
		AppName: string(ProcurementAppName), Agent: wfAgent,
		SessionService: sessionSvc, AutoCreateSession: true,
	})
	require.NoError(t, err)
	return r
}

// TestClaimAndDispatchTurnViaGraph_Integration_MultiTurnContinuation_And_Replay
// is the strongest available evidence for this whole sub-project's two
// headline claims (spec §0): (1) a second turn against the same request
// resumes the first turn's real, Postgres-persisted slots instead of
// starting over, and (2) a repeated claim against a graph-produced run now
// replays real content via a real ResultRevisionID — both against a real
// *repository.Repo and a real Postgres-backed ADK session.Service, not
// fakes.
func TestClaimAndDispatchTurnViaGraph_Integration_MultiTurnContinuation_And_Replay(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	sessionSvc := setupGraphDispatchSessionService(t, db.DSN)
	buyerID := identity.BuyerID(90200)

	// Turn 1: new conversation, only Item known.
	r1 := newTerminalCompletionGraph(t, sessionSvc, graphTurnProgress{NextSlots: domain.Slots{Item: item("蘋果")}})
	out1, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		BuyerID: buyerID, ClientRequestID: "integration-continue-1", Message: "我要買蘋果",
	}, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r1, ResumeRunner: ResumeRunner{Runner: r1},
		RfqConfirmationReader: repo, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)
	require.NotZero(t, out1.Result.RequestID)
	require.NotEmpty(t, out1.Result.MissingRequired, "sanity check: still missing required slots after turn 1")

	// Turn 2: same RequestID, a merged Slots representing "item carried
	// forward, city newly added" — resolveGraphTurnState (Task 1) must
	// have read turn 1's real persisted slots back for this to be a
	// meaningful continuation proof rather than an isolated write.
	merged := domain.Slots{Item: item("蘋果"), DeliveryCity: strPtr("台北市")}
	r2 := newTerminalCompletionGraph(t, sessionSvc, graphTurnProgress{NextSlots: merged})
	out2, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		RequestID: out1.Result.RequestID, BuyerID: buyerID, ClientRequestID: "integration-continue-2", Message: "台北市",
	}, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r2, ResumeRunner: ResumeRunner{Runner: r2},
		RfqConfirmationReader: repo, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err)
	require.Equal(t, out1.Result.RequestID, out2.Result.RequestID)
	require.Equal(t, "蘋果", *out2.Result.Slots.Item)
	require.Equal(t, "台北市", *out2.Result.Slots.DeliveryCity)
	require.Equal(t, strconv.FormatInt(out1.Result.RequestID, 10)+"-rev"+strconv.FormatInt(out1.Result.RevisionID, 10),
		out2.Result.DialoguePlan.StateVersion,
		"StateVersion must reflect the tip revision resolveGraphTurnState actually read back from history — proves real read-back, not just a shared RequestID")

	var requestCount int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_requests WHERE id = $1`, out1.Result.RequestID,
	).Scan(&requestCount))
	require.Equal(t, 1, requestCount, "must be exactly one request row across both turns, not two")

	// Replay: repeating turn 2's exact (buyer, clientRequestID) must
	// return the same real content, not ErrRunResultUnavailable.
	replay, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		RequestID: out1.Result.RequestID, BuyerID: buyerID, ClientRequestID: "integration-continue-2", Message: "台北市",
	}, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r2, ResumeRunner: ResumeRunner{Runner: r2},
		RfqConfirmationReader: repo, Entitlement: nil, Quota: nil,
	}, TurnStreamHooks{})
	require.NoError(t, err, "a repeat claim on a graph-produced, now-persisted run must replay, not error")
	require.Equal(t, out2.Result.Reply, replay.Result.Reply)
	require.Equal(t, out2.Result.RevisionID, replay.Result.RevisionID)
}

// TestClaimAndDispatchTurnViaGraph_Integration_RealPostgres_QuotaReservedAndConfirmed
// proves quota reservation survives a real Postgres round-trip: the
// reservation shows up as a real row, and after a successful turn it is
// confirmed (not left pending, not released) — the exact guarantee
// composeGraphTurnResult/graphTurnExecutor's unit tests (Task 3) already
// pin with fakes, now proven against the real repository.QuotaReserver
// implementation (repository/quota.go) instead of a fake standing in for it.
//
// 沿用 newTerminalCompletionGraph 而不是完整 graph：與上方 multi-turn 測試共用同一
// 份隔離前提，只證明 confirm 路徑的持久化，不重跑 guard 鏈。
func TestClaimAndDispatchTurnViaGraph_Integration_RealPostgres_QuotaReservedAndConfirmed(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	sessionSvc := setupGraphDispatchSessionService(t, db.DSN)

	r := newTerminalCompletionGraph(t, sessionSvc, graphTurnProgress{NextSlots: domain.Slots{Item: item("蘋果")}})

	buyerID := identity.BuyerID(90200)
	// Captured once and reused below — testEntitlement()'s PeriodStart is
	// derived from time.Now() (truncated to the second), so calling it
	// again later in this test could straddle a wall-clock second boundary
	// and silently land in a different period bucket than the turn actually
	// reserved against, defeating the confirmedBefore assertion below.
	ent := testEntitlement()
	client := &fakeEntitlementClient{entitlement: ent}
	in := TurnInput{BuyerID: buyerID, ClientRequestID: "integration-quota-1", Message: "我要買蘋果"}

	out, err := ClaimAndDispatchTurnViaGraph(ctx, in, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: repo, Entitlement: client, Quota: repo,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.Equal(t, ent.PlanCode, out.Result.Quota.PlanCode)
	require.Equal(t, ent.MonthlyAnswerLimit-1, out.Result.Quota.Remaining, "a fresh buyer's first confirmed turn this period")

	// Prove the reservation is durably CONFIRMED (not left PENDING, not
	// released) by reserving again for the same buyer/period through the
	// real QuotaReserver — ReserveQuota's own confirmedBefore return value
	// (repository/quota.go:28-30: "已確認（durable）的用量") is exactly
	// this proof, and reusing the production interface avoids depending on
	// this plan's own guess at internal table/column names for a raw SQL
	// assertion.
	_, _, confirmedBefore, err := repo.ReserveQuota(ctx, buyerID, ent.PeriodStart, ent.MonthlyAnswerLimit)
	require.NoError(t, err)
	require.Equal(t, 1, confirmedBefore, "the first turn's reservation must show up as durably confirmed usage, not pending or released")
}

// TestClaimAndDispatchTurnViaGraph_Integration_RealPostgres_QuotaExhausted_RejectsBeforeGraphRun
// proves the fail-closed rejection happens against a real, already-exhausted
// quota — reserving MonthlyAnswerLimit turns' worth of quota for this buyer
// first via the same real QuotaReserver, then proving the next
// ClaimAndDispatchTurnViaGraph call is rejected with QuotaExceededError and
// creates no request row.
func TestClaimAndDispatchTurnViaGraph_Integration_RealPostgres_QuotaExhausted_RejectsBeforeGraphRun(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()

	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())

	buyerID := identity.BuyerID(90201)
	ent := testEntitlement()
	ent.MonthlyAnswerLimit = 1 // small limit, cheap to exhaust deterministically
	client := &fakeEntitlementClient{entitlement: ent}

	// Exhaust the limit directly against the real QuotaReserver — same
	// dependency ClaimAndDispatchTurnViaGraph itself will call, so this
	// setup step is not a parallel/simulated path.
	reservationID, reserved, _, err := repo.ReserveQuota(ctx, buyerID, ent.PeriodStart, ent.MonthlyAnswerLimit)
	require.NoError(t, err)
	require.True(t, reserved, "sanity check: the first reservation for a fresh buyer must succeed")
	_ = reservationID // confirmed implicitly by never releasing it — a real AppendRevision-style confirm isn't needed to prove exhaustion; an unreleased PENDING reservation already counts against the limit (repository/quota.go's ReserveQuota counts pending+confirmed, same invariant HandleStream's own quota tests rely on)

	in := TurnInput{BuyerID: buyerID, ClientRequestID: "integration-quota-exhausted-1", Message: "我要買蘋果"}

	_, err = ClaimAndDispatchTurnViaGraph(ctx, in, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: nil, ResumeRunner: ResumeRunner{Runner: nil},
		RfqConfirmationReader: repo, Entitlement: client, Quota: repo,
	}, TurnStreamHooks{})

	var quotaErr *QuotaExceededError
	require.ErrorAs(t, err, &quotaErr)
	require.Equal(t, ent.MonthlyAnswerLimit, quotaErr.Limit)

	// r (the *runner.Runner) was passed nil — had the rejection not
	// short-circuited before resolveGraphTurnState/r.Run(), this test would
	// have panicked on a nil-pointer Run() call instead of returning
	// cleanly, the same proof technique Task 3's unit test
	// (TestClaimAndDispatchTurnViaGraph_QuotaExceeded_BlocksBeforeAnyGraphExecution)
	// uses with a fake store; this integration test additionally proves the
	// exhaustion itself is real (a genuine second ReserveQuota call against
	// the real table returned reserved=false), which the unit test's fake
	// QuotaReserver cannot prove on its own.
}

// The four tests below are ports of the deleted turn_idempotency_integration_test.go
// (Task 6a of the procurement-graph-cutover plan, 2026-08-31): per that
// task's own brief, before deleting a legacy-Handle-only real-Postgres
// idempotency test file, this file (this package's only other
// //go:build integration test file) must be checked for an equivalent
// graph-path test — none existed, so this is a genuine coverage gap, closed
// here by porting each test to drive ClaimAndDispatchTurnViaGraph instead of
// TurnUseCase.Handle, keeping every assertion (row counts, snapshot
// content, DialoguePlan byte-equality, cross-buyer isolation) unchanged.
// These prove idempotency requires the ordering between usecase and
// repository calls to be correct — an in-memory fake would trivially stay
// green even if that ordering regressed, per newTurnUseCaseOnDB's own doc
// comment in the now-deleted file — so, like the legacy tests they replace,
// they must run against real Postgres, not fakeRunStore/fakeRepo.

// TestClaimAndDispatchTurnViaGraph_Integration_RetryWithoutRequestIDIsFullyIdempotent
// is D1 path A: the client posts a turn, never receives the response, and
// retries with the same clientRequestId and no requestId. Before Task 1-6 of
// this plan's predecessor fix, this created a second, orphaned, empty
// request on retry — this proves the graph path is exactly as idempotent as
// the legacy Handle path it replaces.
func TestClaimAndDispatchTurnViaGraph_Integration_RetryWithoutRequestIDIsFullyIdempotent(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	sessionSvc := setupGraphDispatchSessionService(t, db.DSN)

	next := domain.Slots{Item: item("寶島梨禮盒"), Quantity: ptr(50)}
	r := newTerminalCompletionGraph(t, sessionSvc, graphTurnProgress{
		Patch: domain.SlotPatch{Item: item("寶島梨禮盒"), Quantity: ptr(50)}, NextSlots: next,
	})
	in := TurnInput{
		BuyerID: itBuyerID, RequestID: 0,
		Message: "我要買寶島梨禮盒 50 盒", ClientRequestID: "cr-retry",
	}
	deps := GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: repo,
	}

	first, err := ClaimAndDispatchTurnViaGraph(ctx, in, deps, TurnStreamHooks{})
	require.NoError(t, err)
	second, err := ClaimAndDispatchTurnViaGraph(ctx, in, deps, TurnStreamHooks{})
	require.NoError(t, err)

	require.Equal(t, first.Result.RequestID, second.Result.RequestID,
		"重送必須回同一個 requestId；不同代表又建了一個孤兒 request")
	require.Equal(t, first.Result.RevisionID, second.Result.RevisionID)
	require.Equal(t, first.Result, second.Result, "重播回應必須與首次逐欄位一致")

	var requests int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_requests WHERE buyer_id = $1`, int64(itBuyerID),
	).Scan(&requests))
	require.Equal(t, int64(1), requests, "procurement_requests 只能有一列")

	var revisions int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM procurement_request_revisions WHERE request_id = $1`, first.Result.RequestID,
	).Scan(&revisions))
	require.Equal(t, int64(2), revisions, "初始空版本 + 一次 append，重送不得再追加")

	req, rev, err := repo.GetSnapshot(ctx, second.Result.RequestID, itBuyerID)
	require.NoError(t, err)
	require.Equal(t, second.Result.RevisionID, rev.ID)
	require.Equal(t, req.ActiveRevisionID, rev.ID)

	slots, err := repo.GetSlots(ctx, second.Result.RequestID, itBuyerID)
	require.NoError(t, err)
	require.NotNil(t, slots.Item, "客戶端存下的 requestId 不得指向空需求")
	require.Equal(t, "寶島梨禮盒", *slots.Item)
}

// TestClaimAndDispatchTurnViaGraph_Integration_KeyReusedOnDifferentRequestConflicts
// is D1 path B: the same buyer reuses the same clientRequestId against
// ANOTHER request. Deliberately reuses the FIRST call's exact message (not a
// third, different message, unlike the legacy test this replaces) — the
// graph path's idempotency key is enforced two layers deep from Handle's
// single RevisionStore-level check: dispatchTurnViaExecutor's ClaimRun call
// gates on a (buyerID, clientRequestID)-keyed *payload hash* first (which
// includes Message, canonicalTurnHash), and only once that hash matches does
// control reach replayTurnRun's explicit
// `in.RequestID != 0 && in.RequestID != prev.RequestID` check — the direct
// port of HandleStream's own check (turn.go, "TurnUseCase.HandleStream 同一條
// 防線") that actually raises domain.ErrIdempotencyKeyConflict. A changed
// message on the reused key would instead trip ClaimRun's own hash-mismatch
// branch first and surface repository.ErrIdempotencyConflict — a real,
// different, already-covered-elsewhere sentinel (see the legacy
// TestClaimAndDispatchTurn_DifferentPayloadSameKey_ReturnsIdempotencyConflict,
// deleted alongside ClaimAndDispatchTurn by this same task) — which would
// silently test a different scenario than this test's name promises. Kept
// identical message on the reused key is what actually reaches (and proves)
// the ErrIdempotencyKeyConflict branch the original test targets.
func TestClaimAndDispatchTurnViaGraph_Integration_KeyReusedOnDifferentRequestConflicts(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	sessionSvc := setupGraphDispatchSessionService(t, db.DSN)
	deps := func(r *runner.Runner) GraphControllerDeps {
		return GraphControllerDeps{
			RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
			RfqConfirmationReader: repo,
		}
	}

	firstProgress := graphTurnProgress{Patch: domain.SlotPatch{Item: item("甲需求")}, NextSlots: domain.Slots{Item: item("甲需求")}}
	firstIn := TurnInput{BuyerID: itBuyerID, Message: "甲需求", ClientRequestID: "cr-reused"}
	first, err := ClaimAndDispatchTurnViaGraph(ctx, firstIn, deps(newTerminalCompletionGraph(t, sessionSvc, firstProgress)), TurnStreamHooks{})
	require.NoError(t, err)

	otherProgress := graphTurnProgress{Patch: domain.SlotPatch{Item: item("乙需求")}, NextSlots: domain.Slots{Item: item("乙需求")}}
	other, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		BuyerID: itBuyerID, Message: "乙需求", ClientRequestID: "cr-other",
	}, deps(newTerminalCompletionGraph(t, sessionSvc, otherProgress)), TurnStreamHooks{})
	require.NoError(t, err)
	require.NotEqual(t, first.Result.RequestID, other.Result.RequestID)

	beforeReq, beforeRev, err := repo.GetSnapshot(ctx, other.Result.RequestID, itBuyerID)
	require.NoError(t, err)

	// Same message as firstIn (see doc comment above) — reaches ClaimRun's
	// hash-match branch, falling through to replayTurnRun's RequestID check.
	_, err = ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		BuyerID: itBuyerID, RequestID: other.Result.RequestID,
		Message: "甲需求", ClientRequestID: "cr-reused",
	}, deps(newTerminalCompletionGraph(t, sessionSvc, firstProgress)), TurnStreamHooks{})
	require.ErrorIs(t, err, domain.ErrIdempotencyKeyConflict,
		"冪等鍵被重用在不同命令上必須明確報衝突，不得靜默丟棄寫入")

	afterReq, afterRev, err := repo.GetSnapshot(ctx, other.Result.RequestID, itBuyerID)
	require.NoError(t, err)
	require.Equal(t, beforeReq.ActiveRevisionID, afterReq.ActiveRevisionID,
		"衝突的那一次不得改動目標 request 的 active revision")
	require.Equal(t, beforeRev.ID, afterRev.ID)
	require.Equal(t, beforeRev.SlotsHash, afterRev.SlotsHash)

	firstReq, _, err := repo.GetSnapshot(ctx, first.Result.RequestID, itBuyerID)
	require.NoError(t, err)
	require.Equal(t, first.Result.RevisionID, firstReq.ActiveRevisionID)
}

// TestClaimAndDispatchTurnViaGraph_Integration_IdempotentRetry_DialoguePlanIsByteEquivalent
// pins DialoguePlan specifically (byte-equal, not just deep-equal via the
// broader "whole result" assertion above), matching what the legacy test
// this replaces called out separately.
func TestClaimAndDispatchTurnViaGraph_Integration_IdempotentRetry_DialoguePlanIsByteEquivalent(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	sessionSvc := setupGraphDispatchSessionService(t, db.DSN)

	progress := graphTurnProgress{Patch: domain.SlotPatch{Item: item("蘋果")}, NextSlots: domain.Slots{Item: item("蘋果")}}
	r := newTerminalCompletionGraph(t, sessionSvc, progress)
	deps := GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(), Runner: r, ResumeRunner: ResumeRunner{Runner: r},
		RfqConfirmationReader: repo,
	}
	in := TurnInput{BuyerID: itBuyerID, Message: "蘋果", ClientRequestID: "cr-retry-plan"}

	first, err := ClaimAndDispatchTurnViaGraph(ctx, in, deps, TurnStreamHooks{})
	require.NoError(t, err)
	require.NotEqual(t, domain.DialoguePlanV1{}, first.Result.DialoguePlan,
		"測試前提不成立：這一輪必須先算出非零值的 plan，否則後面的相等比對沒有意義")

	second, err := ClaimAndDispatchTurnViaGraph(ctx, in, deps, TurnStreamHooks{})
	require.NoError(t, err)

	require.Equal(t, first.Result.DialoguePlan, second.Result.DialoguePlan,
		"重播必須回第一次已保存的 plan，不得重新計算出另一個（即使兩次算出來語意相同，PlanHash／StateVersion 等欄位也必須完全一致）")
}

// TestClaimAndDispatchTurnViaGraph_Integration_IdempotencyScopedPerBuyer
// proves the idempotency key is scoped to (buyer_id, client_request_id), not
// client_request_id alone — same DB unique index the legacy test this
// replaces pinned.
func TestClaimAndDispatchTurnViaGraph_Integration_IdempotencyScopedPerBuyer(t *testing.T) {
	ctx := context.Background()

	db, drop := pg.Clone(ctx, t)
	defer drop()
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()

	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	sessionSvc := setupGraphDispatchSessionService(t, db.DSN)

	progress := graphTurnProgress{Patch: domain.SlotPatch{Item: item("梨")}, NextSlots: domain.Slots{Item: item("梨")}}

	a, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		BuyerID: itBuyerID, Message: "梨", ClientRequestID: "shared-key",
	}, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
		Runner: newTerminalCompletionGraph(t, sessionSvc, progress), RfqConfirmationReader: repo,
	}, TurnStreamHooks{})
	require.NoError(t, err)
	b, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		BuyerID: identity.BuyerID(5678), Message: "梨", ClientRequestID: "shared-key",
	}, GraphControllerDeps{
		RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
		Runner: newTerminalCompletionGraph(t, sessionSvc, progress), RfqConfirmationReader: repo,
	}, TurnStreamHooks{})
	require.NoError(t, err)

	require.NotEqual(t, a.Result.RequestID, b.Result.RequestID)
	require.NotEqual(t, a.Result.RevisionID, b.Result.RevisionID)
}
