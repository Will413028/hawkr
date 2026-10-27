package grpc

import (
	"context"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/grpc/codes"
)

type stubRunStore struct {
	restoreCalls []repository.RestoreInterruptAfterFailedResumeInput

	mu sync.Mutex

	runID int64

	runIDByClientRequestID map[string]int64

	claims     int
	events     []repository.AppendRunEventInput
	terminals  []repository.TerminalizeRunInput
	interrupts []repository.CreateInterruptInput

	claimedControllerKinds []domain.ControllerKind

	onClaim  func()
	claimCtx context.Context

	hasOpenInterrupt       bool
	openInterrupt          repository.Interrupt
	findOpenInterruptCalls int

	resumeOut   repository.ResumeRunOutput
	resumeErr   error
	resumeCalls []repository.ResumeRunInput
}

func (s *stubRunStore) ClaimRun(ctx context.Context, in repository.ClaimRunInput) (repository.Run, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCtx = ctx
	s.claims++
	s.claimedControllerKinds = append(s.claimedControllerKinds, in.ControllerKind)
	if s.onClaim != nil {
		s.onClaim()
	}
	runID := s.runID
	if mapped, ok := s.runIDByClientRequestID[in.ClientRequestID]; ok {
		runID = mapped
	}
	return repository.Run{ID: runID, RunType: domain.RunTypeTurn, Status: domain.RunStatusQueued(), Version: 1}, true, nil
}

func (s *stubRunStore) AppendRunEvent(_ context.Context, in repository.AppendRunEventInput) (repository.RunEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, in)
	return repository.RunEvent{RunID: in.RunID, EventType: in.EventType}, nil
}

func (s *stubRunStore) TerminalizeRun(_ context.Context, in repository.TerminalizeRunInput) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.terminals = append(s.terminals, in)
	return true, nil
}

// GetRun 是取消觀察器（usecase.runCancelObserver）的讀路徑。這些測試驗的都不
// 是取消，所以一律回 RUNNING＝「沒有人按停止」，讓 pipeline 照原本的路徑跑完。
func (s *stubRunStore) GetRun(_ context.Context, runID int64, buyerID identity.BuyerID) (repository.Run, error) {
	return repository.Run{ID: runID, BuyerID: buyerID, Status: domain.RunStatusRunning()}, nil
}

func (s *stubRunStore) CreateInterrupt(_ context.Context, in repository.CreateInterruptInput) (repository.Interrupt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interrupts = append(s.interrupts, in)
	return repository.Interrupt{ID: "interrupt-77", RunID: in.RunID, InterruptKind: in.InterruptKind, Status: "PENDING"}, nil
}

func (s *stubRunStore) HasIdempotencyKey(context.Context, identity.BuyerID, string) (bool, error) {
	return false, nil
}

func (s *stubRunStore) FindOpenInterruptForRequest(context.Context, int64, identity.BuyerID) (repository.Interrupt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.findOpenInterruptCalls++
	if s.hasOpenInterrupt {
		return s.openInterrupt, nil
	}
	return repository.Interrupt{}, repository.ErrNotFound
}

func (s *stubRunStore) SetRunConversationID(context.Context, int64, int64) error {
	return nil
}

func (s *stubRunStore) FindByIdempotencyKey(context.Context, identity.BuyerID, string) (repository.IdempotentResult, error) {
	return repository.IdempotentResult{}, repository.ErrNotFound
}

func (s *stubRunStore) ResumeRun(_ context.Context, in repository.ResumeRunInput) (repository.ResumeRunOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resumeCalls = append(s.resumeCalls, in)
	if s.resumeErr != nil {
		return repository.ResumeRunOutput{}, s.resumeErr
	}
	return s.resumeOut, nil
}

func (s *stubRunStore) RestoreInterruptAfterFailedResume(
	_ context.Context, in repository.RestoreInterruptAfterFailedResumeInput,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restoreCalls = append(s.restoreCalls, in)
	return true, nil
}

func durableRunDeps(runStore usecase.ResumeRunStore, enabled bool) DurableRunDeps {
	_ = enabled
	return DurableRunDeps{
		RunStore:   runStore,
		Dispatcher: usecase.NewRunDispatcher(),
	}
}

func TestSubmitTurn_OpenInterrupt_ReturnsAbortedApprovalPending(t *testing.T) {
	runStore := &stubRunStore{runID: 5150, hasOpenInterrupt: true, openInterrupt: repository.Interrupt{ID: "interrupt-1", Status: "PENDING"}}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil, usecase.GraphControllerDeps{RunStore: runStore}, nil, SellerCommandDeps{})

	_, err := s.SubmitTurn(buyerActorContext("8001"),
		&aipb.TurnRequest{RequestId: "1", ClientRequestId: "cr-new-1", Message: "我要買寶島梨"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	code, reason := statusCodeAndReason(t, err)
	if code != codes.Aborted {
		t.Fatalf("status code = %v, want Aborted", code)
	}
	if reason != "APPROVAL_PENDING" {
		t.Fatalf("reason = %q, want APPROVAL_PENDING", reason)
	}
	if runStore.findOpenInterruptCalls != 1 {
		t.Fatalf("FindOpenInterruptForRequest calls = %d, want 1", runStore.findOpenInterruptCalls)
	}
	if runStore.claims != 0 {
		t.Fatalf("claims = %d, want 0 (guard must block before ClaimRun, zero mutation)", runStore.claims)
	}
}

func TestSubmitTurnStream_OpenInterrupt_ReturnsAbortedApprovalPending(t *testing.T) {
	runStore := &stubRunStore{runID: 5150, hasOpenInterrupt: true, openInterrupt: repository.Interrupt{ID: "interrupt-1", Status: "PENDING"}}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil, usecase.GraphControllerDeps{RunStore: runStore}, nil, SellerCommandDeps{})
	stream := &fakeAssistantStream{ctx: buyerActorContext("8001")}

	err := s.SubmitTurnStream(&aipb.TurnRequest{RequestId: "1", ClientRequestId: "cr-new-1", Message: "我要買寶島梨"}, stream)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	code, reason := statusCodeAndReason(t, err)
	if code != codes.Aborted {
		t.Fatalf("status code = %v, want Aborted", code)
	}
	if reason != "APPROVAL_PENDING" {
		t.Fatalf("reason = %q, want APPROVAL_PENDING", reason)
	}
	if len(stream.sent) != 0 {
		t.Fatalf("guard 擋下時不得送出任何 frame，got %d", len(stream.sent))
	}
	if runStore.findOpenInterruptCalls != 1 {
		t.Fatalf("FindOpenInterruptForRequest calls = %d, want 1", runStore.findOpenInterruptCalls)
	}
	if runStore.claims != 0 {
		t.Fatalf("claims = %d, want 0 (guard must block before ClaimRun, zero mutation)", runStore.claims)
	}
}

type successGraphRevisionStore struct{}

func (successGraphRevisionStore) CreateRequest(context.Context, identity.BuyerID) (repository.Request, repository.Revision, error) {
	return repository.Request{ID: 1}, repository.Revision{}, nil
}

func (successGraphRevisionStore) AppendRevision(context.Context, repository.AppendInput) (repository.Revision, error) {
	return repository.Revision{}, nil
}

func (successGraphRevisionStore) GetSlots(context.Context, int64, identity.BuyerID) (domain.Slots, error) {
	return domain.Slots{}, nil
}

func (successGraphRevisionStore) GetSnapshot(context.Context, int64, identity.BuyerID) (repository.Request, repository.Revision, error) {
	return repository.Request{}, repository.Revision{}, nil
}

func (successGraphRevisionStore) FindByIdempotencyKey(context.Context, identity.BuyerID, string) (repository.IdempotentResult, error) {
	return repository.IdempotentResult{}, repository.ErrNotFound
}

func (successGraphRevisionStore) ListBranchRevisions(context.Context, int64, identity.BuyerID) ([]repository.RevisionWithPatch, error) {
	return nil, nil
}

func TestSubmitTurn_DurableRunDepsWired_GraphDispatchOnly(t *testing.T) {
	graphStore := &stubRunStore{runID: 7001}
	durableStore := &stubRunStore{runID: 5150}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		durableRunDeps(durableStore, true), nil,
		usecase.GraphControllerDeps{
			RunStore:      graphStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        &runner.Runner{},
		}, nil, SellerCommandDeps{})

	_, err := s.SubmitTurn(buyerActorContext("8001"),
		&aipb.TurnRequest{ClientRequestId: "cr-priority-1", Message: "我要買寶島梨"})

	if err == nil {
		t.Fatal("expected an error from the graph branch's zero-value Runner panic; got nil — the durableRun branch may have run instead")
	}
	if graphStore.claims != 1 {
		t.Fatalf("graph branch must have claimed exactly one run, got claims=%d", graphStore.claims)
	}
	if durableStore.claims != 0 {
		t.Fatalf("durableRun branch must not run at all when the graph branch is enabled, got claims=%d", durableStore.claims)
	}
	if len(graphStore.terminals) != 1 || graphStore.terminals[0].Status != domain.RunStatusFailed() {
		t.Fatalf("expected the graph branch to terminalize its run as FAILED after the panic: %+v", graphStore.terminals)
	}
}

func newTask4PendingCropGateRunner(t *testing.T) *runner.Runner {
	t.Helper()

	seedPendingCropGateOutput := workflow.NewFunctionNode(
		"task4_seed_crop_gate_output_pending",
		func(_ agent.Context, _ string) (usecase.CropGateOutput, error) {
			return usecase.CropGateOutput{
				Pending:    true,
				Candidates: []domain.CropDisambiguationOption{{Label: "紅蘿蔔"}, {Label: "白蘿蔔"}},
			}, nil
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "task4_submit_turn_graph_success_chain",
		Edges: workflow.Chain(workflow.Start, seedPendingCropGateOutput, usecase.NewCropDisambiguationInterruptNode()),
	})
	if err != nil {
		t.Fatalf("workflowagent.New failed: %v", err)
	}

	r, err := runner.New(runner.Config{
		AppName: string(usecase.ProcurementAppName), Agent: wfAgent,
		SessionService: session.InMemoryService(), AutoCreateSession: true,
	})
	if err != nil {
		t.Fatalf("runner.New failed: %v", err)
	}
	return r
}

func TestSubmitTurn_AlwaysDispatchesViaGraph_NoFlagsRequired(t *testing.T) {
	runStore := &stubRunStore{runID: 9001}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:      runStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        newTask4PendingCropGateRunner(t),
		}, nil, SellerCommandDeps{})

	result, err := s.SubmitTurn(buyerActorContext("9001"),
		&aipb.TurnRequest{ClientRequestId: "cr-graph-always-1", Message: "我要買紅蘿蔔"})

	if err != nil {
		t.Fatalf("SubmitTurn must succeed unconditionally via the graph path, got error: %v", err)
	}
	if runStore.claims != 1 {
		t.Fatalf("expected exactly one ClaimRun call, got claims=%d", runStore.claims)
	}
	if len(runStore.claimedControllerKinds) != 1 || runStore.claimedControllerKinds[0] != domain.ControllerKindGraph {
		t.Fatalf("ClaimAndDispatchTurnViaGraph must claim ControllerKindGraph, got %+v", runStore.claimedControllerKinds)
	}
	if len(runStore.terminals) != 1 {
		t.Fatalf("expected exactly one TerminalizeRun call (the graph run must reach completion, not hang or short-circuit), got %+v", runStore.terminals)
	}
	if got := runStore.terminals[0].Status; got != domain.RunStatusWaiting() {
		t.Fatalf("expected the crop-disambiguation pause to terminalize as WAITING_FOR_INPUT, got %v", got)
	}
	if result.GetRunId() != "9001" {
		t.Fatalf("expected the response to carry the real ledger run_id, got %q", result.GetRunId())
	}
	if result.GetInterruptId() != "interrupt-77" || result.GetInterruptKind() != "CLARIFICATION" {
		t.Fatalf("expected the response to carry interrupt fields, got id=%q kind=%q",
			result.GetInterruptId(), result.GetInterruptKind())
	}
}

func TestSubmitTurnStream_AlwaysDispatchesViaGraph_NoFlagsRequired(t *testing.T) {
	runStore := &stubRunStore{runID: 9002}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:      runStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        newTask4PendingCropGateRunner(t),
		}, nil, SellerCommandDeps{})
	stream := &fakeAssistantStream{ctx: buyerActorContext("9002")}

	if err := s.SubmitTurnStream(&aipb.TurnRequest{ClientRequestId: "cr-graph-always-stream-1", Message: "我要買紅蘿蔔"}, stream); err != nil {
		t.Fatalf("SubmitTurnStream must succeed unconditionally via the graph path, got error: %v", err)
	}
	if runStore.claims != 1 {
		t.Fatalf("expected exactly one ClaimRun call, got claims=%d", runStore.claims)
	}
	if len(runStore.claimedControllerKinds) != 1 || runStore.claimedControllerKinds[0] != domain.ControllerKindGraph {
		t.Fatalf("ClaimAndDispatchTurnViaGraph must claim ControllerKindGraph, got %+v", runStore.claimedControllerKinds)
	}
	if len(stream.sent) == 0 {
		t.Fatal("expected at least one frame to be sent")
	}
	first := stream.sent[0].GetRunAccepted()
	if first == nil {
		t.Fatalf("first frame must be RunAccepted, got %T", stream.sent[0].GetPayload())
	}
	if first.GetRunId() != "9002" {
		t.Fatalf("RunAccepted.run_id = %q, want 9002", first.GetRunId())
	}
	var terminal *aipb.TurnResult
	for _, ev := range stream.sent {
		if tr := ev.GetTurnResult(); tr != nil {
			terminal = tr
		}
	}
	if terminal == nil {
		t.Fatal("expected a terminal TurnResult frame")
	}
	if terminal.GetRunId() != "9002" {
		t.Fatalf("terminal frame run_id = %q, want 9002", terminal.GetRunId())
	}
	if terminal.GetInterruptId() != "interrupt-77" || terminal.GetInterruptKind() != "CLARIFICATION" {
		t.Fatalf("terminal frame must carry interrupt fields, got id=%q kind=%q",
			terminal.GetInterruptId(), terminal.GetInterruptKind())
	}
	for _, ev := range stream.sent {
		if ev.GetDone() != nil {
			t.Fatal("SubmitTurnStream must not send the empty DoneEvent")
		}
	}
}

func TestRewind_OpenInterrupt_ReturnsAbortedApprovalPending(t *testing.T) {
	runStore := &stubRunStore{runID: 5150, hasOpenInterrupt: true, openInterrupt: repository.Interrupt{ID: "interrupt-1", Status: "PENDING"}}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, &usecase.RewindUseCase{}, nil, nil, nil, nil, nil, nil,
		durableRunDeps(runStore, false), nil, usecase.GraphControllerDeps{}, nil, SellerCommandDeps{})

	_, err := s.Rewind(buyerActorContext("8001"),
		&aipb.RewindRequest{RequestId: "1", ClientRequestId: "cr-rewind-new-1", SlotKey: "quantity"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	code, reason := statusCodeAndReason(t, err)
	if code != codes.Aborted {
		t.Fatalf("status code = %v, want Aborted", code)
	}
	if reason != "APPROVAL_PENDING" {
		t.Fatalf("reason = %q, want APPROVAL_PENDING", reason)
	}
	if runStore.findOpenInterruptCalls != 1 {
		t.Fatalf("FindOpenInterruptForRequest calls = %d, want 1", runStore.findOpenInterruptCalls)
	}
	if runStore.claims != 0 {
		t.Fatalf("claims = %d, want 0 (guard must block before ClaimRun, zero mutation)", runStore.claims)
	}
}
