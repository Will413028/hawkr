package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"google.golang.org/adk/v2/session"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/testkit"
	"github.com/Will413028/hawkr/go-pkg/identity"
)

func TestGraphConversation_TurnThenAnswerThenAnswer_StaysGraphRouted(t *testing.T) {
	ctx := context.Background()
	buyerID := identity.BuyerID(90300)
	runStore := newFakeRunStore()
	repo := newFakeRepo()
	dispatcher := NewRunDispatcher()

	extractor := &fakeExtractor{patch: domain.SlotPatch{Item: item("蘋果")}}
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{}
	narrator := &fakeStepNarrator{text: "narrated"}

	r, _, err := NewProcurementGraphRunner(extractor, resolver, seqIDs(9100), session.InMemoryService(), nil, narrator, nil)
	require.NoError(t, err)

	deps := GraphControllerDeps{
		RunStore: runStore, RevisionStore: repo, Dispatcher: dispatcher, Runner: r, ResumeRunner: ResumeRunner{Runner: r},
	}

	turnIn := TurnInput{BuyerID: buyerID, ClientRequestID: "conv-1-turn", Message: "我要買蘋果"}
	turnOut, err := ClaimAndDispatchTurnViaGraph(ctx, turnIn, deps, TurnStreamHooks{})
	require.NoError(t, err)
	require.NotNil(t, turnOut.Result.Question, "missing required slots must produce a clarifying question")
	firstRun, ok := runStore.runs[fakePatchKey{buyerID, "conv-1-turn"}]
	require.True(t, ok, "ClaimAndDispatchTurnViaGraph must have claimed a run under this ClientRequestID")
	require.Equal(t, domain.ControllerKindGraph, firstRun.ControllerKind)
	askedSlot := turnOut.Result.Question.SlotKey

	answerUC := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)

	answerIn1 := AnswerInput{
		BuyerID: buyerID, RequestID: turnOut.Result.RequestID, SlotKey: askedSlot,
		FreeText: "臺北市", ExpectedRevisionID: turnOut.Result.RevisionID,
		ClientRequestID: "conv-1-answer-1",
	}
	answerOut1, err := answerUC.ClaimAndDispatchAnswer(ctx, answerIn1, runStore, dispatcher)
	require.NoError(t, err)
	answerRun1, ok := runStore.runs[fakePatchKey{buyerID, "conv-1-answer-1"}]
	require.True(t, ok, "ClaimAndDispatchAnswer must have claimed a run under this ClientRequestID")
	require.Equal(t, domain.ControllerKindGraph, answerRun1.ControllerKind,
		"SubmitAnswer on a graph-dispatched conversation must claim its own run as Graph, not Durable")

	lastAnswerOut := answerOut1
	for i := 0; i < 5 && lastAnswerOut.Result.Question != nil; i++ {
		nextSlot := lastAnswerOut.Result.Question.SlotKey
		answerIn := AnswerInput{
			BuyerID: buyerID, RequestID: lastAnswerOut.Result.RequestID, SlotKey: nextSlot,
			ExpectedRevisionID: lastAnswerOut.Result.RevisionID,
			ClientRequestID:    "conv-1-answer-loop-" + string(nextSlot),
		}
		switch nextSlot {
		case domain.SlotQuantity:
			answerIn.FreeText = "5"
			answerIn.UnitOptionID = "unit-kg"
		case domain.SlotBudgetPerUnit:
			answerIn.FreeText = "100"
		case domain.SlotDeliveryDate:
			answerIn.FreeText = time.Now().UTC().AddDate(0, 0, 7).Format("2006-01-02")
		default:
			answerIn.FreeText = "測試值"
		}
		lastAnswerOut, err = answerUC.ClaimAndDispatchAnswer(ctx, answerIn, runStore, dispatcher)
		require.NoError(t, err)
	}
	require.Nil(t, lastAnswerOut.Result.Question, "conversation must reach ready-to-finalize within the bounded loop")
	require.Empty(t, lastAnswerOut.Result.MissingRequired)
}

func TestGraphConversation_RewindThenContinue_ReAsksRewoundSlot(t *testing.T) {
	ctx := context.Background()
	buyerID := identity.BuyerID(90301)
	runStore := newFakeRunStore()
	repo := newFakeRepo()
	dispatcher := NewRunDispatcher()

	extractor := &fakeExtractor{patch: domain.SlotPatch{Item: item("蘋果")}}
	resolver := &testkit.FakeResolver[cropidentity.DisambiguationResult]{}
	narrator := &fakeStepNarrator{text: "narrated"}

	r, _, err := NewProcurementGraphRunner(extractor, resolver, seqIDs(9200), session.InMemoryService(), nil, narrator, nil)
	require.NoError(t, err)

	deps := GraphControllerDeps{
		RunStore: runStore, RevisionStore: repo, Dispatcher: dispatcher, Runner: r, ResumeRunner: ResumeRunner{Runner: r},
	}

	turnIn := TurnInput{BuyerID: buyerID, ClientRequestID: "conv-2-turn", Message: "我要買蘋果"}
	turnOut, err := ClaimAndDispatchTurnViaGraph(ctx, turnIn, deps, TurnStreamHooks{})
	require.NoError(t, err)
	require.NotNil(t, turnOut.Result.Question)
	require.Equal(t, domain.SlotDeliveryCity, turnOut.Result.Question.SlotKey)

	answerUC := NewAnswerUseCase(repo, nil, nil, nil, nil, nil)

	answerIn := AnswerInput{
		BuyerID: buyerID, RequestID: turnOut.Result.RequestID, SlotKey: domain.SlotQuantity,
		FreeText: "5", UnitOptionID: "unit-kg", ExpectedRevisionID: turnOut.Result.RevisionID,
		ClientRequestID: "conv-2-answer-quantity-1",
	}
	answerOut, err := answerUC.ClaimAndDispatchAnswer(ctx, answerIn, runStore, dispatcher)
	require.NoError(t, err)
	answerRun, ok := runStore.runs[fakePatchKey{buyerID, "conv-2-answer-quantity-1"}]
	require.True(t, ok, "ClaimAndDispatchAnswer must have claimed a run under this ClientRequestID")
	require.Equal(t, domain.ControllerKindGraph, answerRun.ControllerKind,
		"SubmitAnswer on a graph-dispatched conversation must claim its own run as Graph, not Durable")
	require.NotNil(t, answerOut.Result.Question)
	require.Equal(t, domain.SlotDeliveryCity, answerOut.Result.Question.SlotKey,
		"delivery_city must still be the only missing required slot after answering quantity out of order")

	rewindUC := NewRewindUseCase(repo, nil, nil, nil, nil, nil)
	rewindIn := RewindInput{
		BuyerID: buyerID, RequestID: answerOut.Result.RequestID, SlotKey: domain.SlotQuantity,
		ClientRequestID: "conv-2-rewind-1",
	}
	rewindOut, err := rewindUC.ClaimAndDispatchRewind(ctx, rewindIn, runStore, dispatcher)
	require.NoError(t, err)
	rewindRun, ok := runStore.runs[fakePatchKey{buyerID, "conv-2-rewind-1"}]
	require.True(t, ok, "ClaimAndDispatchRewind must have claimed a run under this ClientRequestID")
	require.Equal(t, domain.ControllerKindGraph, rewindRun.ControllerKind,
		"Rewind on a graph-dispatched conversation must claim its own run as Graph, not Durable")
	require.NotNil(t, rewindOut.Result.Question, "rewinding a just-answered slot must re-ask it")
	require.Equal(t, domain.SlotQuantity, rewindOut.Result.Question.SlotKey,
		"must re-ask the REWOUND slot (quantity), not delivery_city — the slot PickNextQuestion's priority engine would pick instead")
}
