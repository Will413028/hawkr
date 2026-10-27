//go:build integration

package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

type stageDispatchResolver struct {
	initialCalls   atomic.Int32
	selectionCalls atomic.Int32
}

func (r *stageDispatchResolver) Resolve(context.Context, string) (cropidentity.DisambiguationResult, error) {
	r.initialCalls.Add(1)
	return cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Selection:           &domain.CropSelection{Stage: domain.CropSelectionGroup},
		Candidates:          []domain.CropDisambiguationOption{{CategoryID: "8", CategoryLevel: 2, Label: "豬肉"}},
	}, nil
}

func (r *stageDispatchResolver) ResolveSelection(_ context.Context, text string, selection *domain.CropSelection, _ []domain.CropDisambiguationOption, optionID, _ string) (cropidentity.DisambiguationResult, error) {
	r.selectionCalls.Add(1)
	if text != "臺灣豬五花" {
		return cropidentity.DisambiguationResult{}, fmt.Errorf("original item was replaced: %q", text)
	}
	if selection.Stage == domain.CropSelectionGroup && optionID == "category-8" {
		return cropidentity.DisambiguationResult{
			NeedsDisambiguation: true,
			Selection: &domain.CropSelection{Stage: domain.CropSelectionItem,
				ParentCategoryID: "8", ParentLabel: "豬肉"},
			Candidates: []domain.CropDisambiguationOption{{CategoryID: "24", CategoryLevel: 3, Label: "五花肉"}},
		}, nil
	}
	if selection.Stage == domain.CropSelectionItem && optionID == "category-24" {
		return cropidentity.DisambiguationResult{
			Unavailable: true,
			Selection: &domain.CropSelection{Stage: domain.CropSelectionPrice,
				ParentCategoryID: "8", ParentLabel: "豬肉", CategoryID: "24", Label: "豬肉／五花肉"},
		}, nil
	}
	return cropidentity.DisambiguationResult{}, ErrInvalidOptionID
}

// This follows the production dispatch entry points with real Postgres
// revisions, run/interrupt transactions, and ADK session storage. Only the
// extractor and category resolver are deterministic local test doubles.
func TestGraphStageDispatch_Integration_Postgres_ReparksAndPreservesSnapshot(t *testing.T) {
	ctx := t.Context()
	db, drop := pg.Clone(ctx, t)
	defer drop()
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()
	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	svc := setupGraphDispatchSessionService(t, db.DSN)
	buyerID := identity.BuyerID(91308)
	quantity, budget := 50, 220
	extractor := &resumeProtocolCountingExtractor{inner: fakeExtractor{patch: domain.SlotPatch{
		Item: item("臺灣豬五花"), Quantity: &quantity, Unit: item("KG"),
		BudgetPerUnit: &budget, DeliveryCity: item("台北市"),
	}}}
	resolver := &stageDispatchResolver{}
	narrator := &fakeStepNarrator{text: "已處理本輪輸入"}
	var graphID atomic.Int64
	graphID.Store(1000000)
	newRunner := func() *runner.Runner {
		graph, err := NewProcurementGraph(extractor, resolver,
			func() int64 { return graphID.Add(1) }, nil, narrator, nil)
		require.NoError(t, err)
		r, err := runner.New(runner.Config{AppName: string(ProcurementAppName), Agent: graph, SessionService: svc, AutoCreateSession: true})
		require.NoError(t, err)
		return r
	}
	deps := GraphControllerDeps{RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
		Runner: newRunner(), RfqConfirmationReader: repo, CropResolver: resolver, Narrator: narrator}
	first, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{
		BuyerID: buyerID, Message: "我要臺灣豬五花 50 公斤，預算 220，送台北市", ClientRequestID: "stage-dispatch-first",
	}, deps, TurnStreamHooks{})
	require.NoError(t, err)
	requestID := first.Result.RequestID
	require.NotZero(t, requestID)

	assertSlots := func(got domain.Slots) {
		t.Helper()
		require.Equal(t, item("臺灣豬五花"), got.Item)
		require.Equal(t, &quantity, got.Quantity)
		require.Equal(t, item("KG"), got.Unit)
		require.Equal(t, &budget, got.BudgetPerUnit)
		require.Equal(t, item("台北市"), got.DeliveryCity)
	}
	assertPersisted := func(result TurnResult, previousRevision int64, stage string) repository.Interrupt {
		t.Helper()
		assertSlots(result.Slots)
		slots, err := repo.GetSlots(ctx, requestID, buyerID)
		require.NoError(t, err)
		assertSlots(slots)
		_, currentRev, err := repo.GetSnapshot(ctx, requestID, buyerID)
		require.NoError(t, err)
		require.Equal(t, result.RevisionID, currentRev.ID)
		history, err := repo.ListBranchRevisions(ctx, requestID, buyerID)
		require.NoError(t, err)
		require.NotEmpty(t, history)
		var payload turnPayload
		require.NoError(t, json.Unmarshal(history[len(history)-1].PatchPayloadJSON, &payload))
		require.Equal(t, result.Question, payload.Question, "snapshot must render the current persisted question")
		if previousRevision != 0 {
			require.Equal(t, strconv.FormatInt(requestID, 10)+"-rev"+strconv.FormatInt(previousRevision, 10), result.DialoguePlan.StateVersion,
				"child composition must use the latest revision history, not the original parked turn")
			sess, err := svc.Get(ctx, &session.GetRequest{AppName: string(ProcurementAppName), UserID: strconv.FormatInt(int64(buyerID), 10), SessionID: ProcurementSessionID(requestID)})
			require.NoError(t, err)
			raw, err := sess.Session.State().Get("current_slots")
			require.NoError(t, err)
			encoded, err := json.Marshal(raw)
			require.NoError(t, err)
			var baseline domain.Slots
			require.NoError(t, json.Unmarshal(encoded, &baseline))
			assertSlots(baseline)
		}
		open, err := repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
		if stage == "" {
			require.ErrorIs(t, err, repository.ErrNotFound, "completed final price must leave no open card")
			return repository.Interrupt{}
		}
		require.NoError(t, err)
		var asked CropDisambiguationInterruptPayload
		require.NoError(t, json.Unmarshal(open.PayloadJSON, &asked))
		require.NotNil(t, asked.Selection)
		require.Equal(t, stage, asked.Selection.Stage)
		require.NotEmpty(t, asked.WorkflowInterruptID)
		require.Equal(t, domain.BuildCropDisambiguationQuestion(asked.Candidates, asked.Selection), result.Question)
		return open
	}
	firstOpen := assertPersisted(first.Result, 0, domain.CropSelectionGroup)

	resume := func(open repository.Interrupt, key, optionID, text, price string) ResumeResult {
		t.Helper()
		body, err := json.Marshal(cropDisambiguationResponseV1{OptionID: optionID, FreeText: text, SelfEnteredPriceAmount: price})
		require.NoError(t, err)
		deps.Runner = newRunner() // no in-process workflow state survives a stage
		deps.ResumeRunner = ResumeRunner{Runner: deps.Runner}
		out, err := ClaimAndDispatchResume(ctx, ResumeInput{BuyerID: buyerID, InterruptID: open.ID,
			ClientRequestID: key, ResponsePayload: body}, repo, deps.Dispatcher, deps)
		require.NoError(t, err)
		require.NotNil(t, out.Result)
		return out
	}
	rejectResume := func(open repository.Interrupt, revisionID int64, key string, body []byte) {
		t.Helper()
		_, err := ClaimAndDispatchResume(ctx, ResumeInput{BuyerID: buyerID, InterruptID: open.ID,
			ClientRequestID: key, ResponsePayload: body}, repo, deps.Dispatcher, deps)
		require.ErrorIs(t, err, ErrInvalidOptionID)
		stillOpen, err := repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
		require.NoError(t, err)
		require.Equal(t, open.ID, stillOpen.ID, "invalid input must not consume the stored question")
		_, revision, err := repo.GetSnapshot(ctx, requestID, buyerID)
		require.NoError(t, err)
		require.Equal(t, revisionID, revision.ID, "invalid input must not append a revision")
	}
	rejectResume(firstOpen, first.Result.RevisionID, "stage-invalid-group", []byte(`{"optionId":"category-forged"}`))
	require.Zero(t, resolver.selectionCalls.Load())
	second := resume(firstOpen, "stage-dispatch-group", "category-8", "", "")
	require.Equal(t, domain.RunStatusWaiting(), second.RunStatus)
	secondOpen := assertPersisted(*second.Result, first.Result.RevisionID, domain.CropSelectionItem)
	require.NotEqual(t, firstOpen.ID, secondOpen.ID)

	third := resume(secondOpen, "stage-dispatch-item", "category-24", "", "")
	require.Equal(t, domain.RunStatusWaiting(), third.RunStatus)
	thirdOpen := assertPersisted(*third.Result, second.Result.RevisionID, domain.CropSelectionPrice)
	require.NotEqual(t, secondOpen.ID, thirdOpen.ID)

	rejectResume(thirdOpen, third.Result.RevisionID, "stage-invalid-price", []byte(`{"optionId":"crop-other","freeText":"臺灣豬五花"}`))
	final := resume(thirdOpen, "stage-dispatch-price", domain.CropOtherOptionID, "臺灣豬五花", "220")
	require.Equal(t, domain.RunStatusSucceeded(), final.RunStatus)
	assertPersisted(*final.Result, third.Result.RevisionID, "")
	require.NotNil(t, final.Result.Slots.ItemResolution)
	require.Equal(t, "24", final.Result.Slots.ItemResolution.CategoryID)
	require.Equal(t, 220, *final.Result.Slots.ItemResolution.PricePerKg)
	require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls))
	require.EqualValues(t, 1, resolver.initialCalls.Load())
	require.EqualValues(t, 2, resolver.selectionCalls.Load())

	// A client retry after completion must return the same child without
	// rerunning the resolver or creating another revision/interrupt.
	body := []byte(`{"optionId":"crop-other","freeText":"臺灣豬五花","selfEnteredPriceAmount":"220"}`)
	replay, err := ClaimAndDispatchResume(ctx, ResumeInput{BuyerID: buyerID, InterruptID: thirdOpen.ID,
		ClientRequestID: "stage-dispatch-price", ResponsePayload: body}, repo, deps.Dispatcher, deps)
	require.NoError(t, err)
	require.Equal(t, final.RunID, replay.RunID)
	require.Equal(t, final.RunStatus, replay.RunStatus)
	_, finalRev, err := repo.GetSnapshot(ctx, requestID, buyerID)
	require.NoError(t, err)
	require.Equal(t, final.Result.RevisionID, finalRev.ID)
}

// Emulate the previous binary's actual handoff node and event shape, then
// replace its graph with today's re-entry implementation. The legacy payload
// has neither Selection/WorkflowInterruptID nor the new session state key.
func TestGraphStageDispatch_Integration_Postgres_UpgradesLegacyHandoffPause(t *testing.T) {
	ctx := t.Context()
	db, drop := pg.Clone(ctx, t)
	defer drop()
	pool, err := database.NewPool(ctx, db.DSN)
	require.NoError(t, err)
	defer pool.Close()
	repo := repository.New(pool, database.NewTxManager(pool), testIDGen())
	svc := setupGraphDispatchSessionService(t, db.DSN)
	buyerID := identity.BuyerID(91309)
	quantity := 50
	extractor := &resumeProtocolCountingExtractor{inner: fakeExtractor{patch: domain.SlotPatch{
		Item: item("高麗菜"), Quantity: &quantity, Unit: item("KG"),
	}}}
	resolver := &countingCropResolver{inner: &testkit.FakeResolver[cropidentity.DisambiguationResult]{ResolveResult: cropidentity.DisambiguationResult{
		NeedsDisambiguation: true,
		Candidates:          []domain.CropDisambiguationOption{{CategoryID: "17", CategoryLevel: 3, Label: "高麗菜"}},
	}}}
	narrator := &fakeStepNarrator{text: "需要確認品項"}
	ids := seqIDs(2000000)
	legacyInterrupt := workflow.NewEmittingFunctionNode("crop_disambiguation_interrupt",
		func(ctx agent.Context, in CropGateOutput, emit func(*session.Event) error) (CropGateOutput, error) {
			if !in.Pending {
				return in, nil
			}
			if err := emit(workflow.NewRequestInputEvent(ctx, session.RequestInput{
				InterruptID: cropDisambiguationInterruptKind,
				Payload: CropDisambiguationInterruptPayload{Kind: cropDisambiguationInterruptKind,
					Candidates: in.Candidates, PendingReason: in.PendingReason, FactsAsOf: in.FactsAsOf},
			})); err != nil {
				return CropGateOutput{}, err
			}
			return CropGateOutput{}, workflow.ErrNodeInterrupted
		}, workflow.NodeConfig{})
	legacyGraph, err := workflowagent.New(workflowagent.Config{Name: "procurement_graph",
		Edges: workflow.Chain(workflow.Start, NewExtractSlotsNode(extractor, ids),
			NewValidateSlotsNode(ids), NewCropGateNode(resolver, ids), legacyInterrupt),
	})
	require.NoError(t, err)
	legacyRunner, err := runner.New(runner.Config{AppName: string(ProcurementAppName), Agent: legacyGraph,
		SessionService: svc, AutoCreateSession: true})
	require.NoError(t, err)
	deps := GraphControllerDeps{RunStore: repo, RevisionStore: repo, Dispatcher: NewRunDispatcher(),
		Runner: legacyRunner, ResumeRunner: ResumeRunner{Runner: legacyRunner}, RfqConfirmationReader: repo, CropResolver: resolver, Narrator: narrator}
	first, err := ClaimAndDispatchTurnViaGraph(ctx, TurnInput{BuyerID: buyerID,
		Message: "我要高麗菜 50 公斤", ClientRequestID: "legacy-handoff-first"}, deps, TurnStreamHooks{})
	require.NoError(t, err)
	requestID := first.Result.RequestID
	open, err := repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	require.NoError(t, err)
	var asked CropDisambiguationInterruptPayload
	require.NoError(t, json.Unmarshal(open.PayloadJSON, &asked))
	require.Empty(t, asked.WorkflowInterruptID)
	require.Nil(t, asked.Selection)
	stored, err := svc.Get(ctx, &session.GetRequest{AppName: string(ProcurementAppName),
		UserID: strconv.FormatInt(int64(buyerID), 10), SessionID: ProcurementSessionID(requestID)})
	require.NoError(t, err)
	_, err = stored.Session.State().Get(cropWorkflowInterruptStateKey)
	require.ErrorIs(t, err, session.ErrStateKeyNotExist)

	upgradedGraph, err := NewProcurementGraph(extractor, resolver, ids, nil, narrator, nil)
	require.NoError(t, err)
	deps.Runner, err = runner.New(runner.Config{AppName: string(ProcurementAppName), Agent: upgradedGraph,
		SessionService: svc, AutoCreateSession: true})
	require.NoError(t, err)
	// resume 走 ResumeRunner，升級後的 graph 也要換過去，否則驗到的是舊 graph。
	deps.ResumeRunner = ResumeRunner{Runner: deps.Runner}
	resumed, err := ClaimAndDispatchResume(ctx, ResumeInput{BuyerID: buyerID, InterruptID: open.ID,
		ClientRequestID: "legacy-handoff-upgraded-answer", ResponsePayload: []byte(`{"optionId":"crop-other","freeText":"高麗菜","selfEnteredPriceAmount":"80"}`)},
		repo, deps.Dispatcher, deps)
	require.NoError(t, err)
	require.Equal(t, domain.RunStatusSucceeded(), resumed.RunStatus)
	require.NotNil(t, resumed.Result)
	require.Equal(t, item("高麗菜"), resumed.Result.Slots.Item)
	require.Equal(t, &quantity, resumed.Result.Slots.Quantity)
	require.Equal(t, item("KG"), resumed.Result.Slots.Unit)
	require.NotNil(t, resumed.Result.Slots.ItemResolution)
	require.Equal(t, domain.ItemResolvedSelfPriced, resumed.Result.Slots.ItemResolution.Kind)
	require.Equal(t, 80, *resumed.Result.Slots.ItemResolution.PricePerKg)
	require.Equal(t, strconv.FormatInt(requestID, 10)+"-rev"+strconv.FormatInt(first.Result.RevisionID, 10), resumed.Result.DialoguePlan.StateVersion)
	_, err = repo.FindOpenInterruptForRequest(ctx, requestID, buyerID)
	require.ErrorIs(t, err, repository.ErrNotFound)
	require.EqualValues(t, 1, atomic.LoadInt32(&extractor.calls))
	require.EqualValues(t, 1, atomic.LoadInt32(&resolver.calls))
}
