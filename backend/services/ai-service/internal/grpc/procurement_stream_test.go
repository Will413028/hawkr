package grpc

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/usecase"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
	"google.golang.org/genai"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const RetiredDoneFrameJSON = `{"requestId":"7001","conversationId":"7002","revisionId":"7003","slots":{"item":"寶島梨","quantity":50,"unit":"箱","packaging":"紙箱","budget_per_unit":1200,"delivery_city":"臺中市","delivery_date":"2026-09-01","exact_match_required":true},"rejected":[{"key":"delivery_date","reason":"到貨日不可早於今天，已忽略"}],"missingRequired":["unit"],"recordedThisTurn":[{"key":"item","label":"品項","value":"寶島梨"}],"degraded":true,"degradedReason":"需求解析暫時無法使用，已保留你目前的需求內容","reply":"已為您記下品項與數量","steps":[{"code":"EXTRACT_SLOTS","status":"DONE","label":"抓出品項、數量和交期","durationMs":340}],"question":{"slotKey":"quantity","prompt":"請問需要多少數量？","options":[{"optionId":"opt-50","label":"50","value":"50"},{"optionId":"opt-100","label":"100","value":"100"}],"unitOptions":[{"optionId":"unit-box","label":"箱","value":"箱"}],"allowFreeText":true,"source":"FIXED","canSkip":false}}`

func strptr(s string) *string { return &s }
func intptr(v int) *int       { return &v }
func boolptr(v bool) *bool    { return &v }

func fullyPopulatedTurnResult() usecase.TurnResult {
	return usecase.TurnResult{
		RequestID: 7001, ConversationID: 7002, RevisionID: 7003,
		Slots: domain.Slots{
			Item: strptr("寶島梨"), Quantity: intptr(50), Unit: strptr("箱"),
			Packaging: strptr("紙箱"), BudgetPerUnit: intptr(1200), DeliveryCity: strptr("臺中市"),
			DeliveryDate: strptr("2026-09-01"), ExactMatchRequired: boolptr(true),
		},
		Rejected:         []domain.RejectedField{{Key: "delivery_date", Reason: "到貨日不可早於今天，已忽略"}},
		MissingRequired:  []domain.SlotKey{domain.SlotUnit},
		RecordedThisTurn: []domain.RecordedField{{Key: "item", Label: "品項", Value: "寶島梨"}},
		Degraded:         true,
		DegradedReason:   "需求解析暫時無法使用，已保留你目前的需求內容",
		Reply:            "已為您記下品項與數量",
		Steps: []domain.Step{
			{Code: domain.StepExtractSlots, Status: "DONE", Label: domain.StepLabels[domain.StepExtractSlots], DurationMs: 340},
		},
		Question: &domain.ClarifyQuestion{
			SlotKey: domain.SlotQuantity, Prompt: "請問需要多少數量？",
			Options:       []domain.QuestionOption{{OptionID: "opt-50", Label: "50", Value: "50"}, {OptionID: "opt-100", Label: "100", Value: "100"}},
			UnitOptions:   []domain.QuestionOption{{OptionID: "unit-box", Label: "箱", Value: "箱"}},
			AllowFreeText: true, Source: domain.ClarifyQuestionSourceFixed, CanSkip: false,
		},
	}
}

func TestRetiredDoneFrameJSON_MatchesTurnResultMarshal(t *testing.T) {
	got, err := json.Marshal(fullyPopulatedTurnResult())
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(got) != RetiredDoneFrameJSON {
		t.Fatalf("retired done-frame payload drifted:\n got %s\nwant %s", got, RetiredDoneFrameJSON)
	}
}

func TestToTurnResultProto_CarriesEveryField(t *testing.T) {
	in := fullyPopulatedTurnResult()
	got := toTurnResultProto(in)

	if got.GetRequestId() != "7001" || got.GetConversationId() != "7002" || got.GetRevisionId() != "7003" {
		t.Fatalf("ids not carried: %+v", got)
	}
	if got.GetSlots().GetItem() != "寶島梨" || got.GetSlots().GetQuantity() != "50" || !got.GetSlots().GetExactMatchRequired() {
		t.Fatalf("slots not carried: %+v", got.GetSlots())
	}
	if len(got.GetRejected()) != 1 || got.GetRejected()[0].GetKey() != "delivery_date" {
		t.Fatalf("rejected not carried: %+v", got.GetRejected())
	}
	if len(got.GetMissingRequired()) != 1 || got.GetMissingRequired()[0] != "unit" {
		t.Fatalf("missingRequired not carried: %+v", got.GetMissingRequired())
	}
	if len(got.GetRecordedThisTurn()) != 1 || got.GetRecordedThisTurn()[0].GetLabel() != "品項" {
		t.Fatalf("recordedThisTurn not carried: %+v", got.GetRecordedThisTurn())
	}
	if !got.GetDegraded() || got.GetDegradedReason() != in.DegradedReason || got.GetReply() != in.Reply {
		t.Fatalf("scalar fields not carried: %+v", got)
	}
	if len(got.GetSteps()) != 1 || got.GetSteps()[0].GetDurationMs() != 340 {
		t.Fatalf("steps not carried: %+v", got.GetSteps())
	}
	q := got.GetQuestion()
	if q == nil || q.GetSlotKey() != "quantity" || len(q.GetOptions()) != 2 || len(q.GetUnitOptions()) != 1 || !q.GetAllowFreeText() {
		t.Fatalf("question not carried: %+v", q)
	}
}

func TestToTurnResultProto_CarriesNextAction(t *testing.T) {
	in := usecase.TurnResult{RequestID: 1, NextAction: "REVIEW_PRICE_PREVIEW"}
	got := toTurnResultProto(in)
	if got.GetNextAction() != "REVIEW_PRICE_PREVIEW" {
		t.Fatalf("next_action not carried: %+v", got)
	}
}

type failingSendStream struct {
	fakeAssistantStream
	sendErr error
	calls   int
}

func (f *failingSendStream) Send(*aipb.StreamEvent) error {
	f.calls++
	return f.sendErr
}

func TestSubmitTurnStream_ClientDisconnectDoesNotCancelUsecaseContext(t *testing.T) {
	streamCtx, cancel := context.WithCancel(buyerActorContext("9005"))
	defer cancel()

	runStore := &stubRunStore{runID: 9005}
	runStore.onClaim = cancel
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:      runStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        newTask4PendingCropGateRunner(t),
		}, nil, SellerCommandDeps{})
	stream := &fakeAssistantStream{ctx: streamCtx}

	_ = s.SubmitTurnStream(&aipb.TurnRequest{ClientRequestId: "cr-1", Message: "我要買寶島梨"}, stream)

	if streamCtx.Err() == nil {
		t.Fatal("test bug: the stream context was never actually cancelled, so this proves nothing")
	}
	if runStore.claimCtx == nil {
		t.Fatal("usecase never ran (ClaimRun was never called)")
	}
	if err := runStore.claimCtx.Err(); err != nil {
		t.Fatalf("usecase context died with the client connection: %v", err)
	}
}

func TestSubmitTurnStream_SendFailureIsReportedWhenTheTurnItselfSucceeded(t *testing.T) {
	runStore := &stubRunStore{runID: 9003}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:      runStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        newTask4PendingCropGateRunner(t),
		}, nil, SellerCommandDeps{})
	sendErr := status.Error(codes.Unavailable, "transport is closing")
	stream := &failingSendStream{fakeAssistantStream: fakeAssistantStream{ctx: buyerActorContext("9003")}, sendErr: sendErr}

	err := s.SubmitTurnStream(&aipb.TurnRequest{ClientRequestId: "cr-1", Message: "我要買寶島梨"}, stream)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("Send failure not surfaced: %v", err)
	}
	if runStore.claims != 1 {
		t.Fatalf("expected the turn itself to have actually run and claimed, got claims=%d", runStore.claims)
	}
	if stream.calls != 1 {
		t.Fatalf("expected exactly one Send attempt after the first failure (later frames skipped), got %d", stream.calls)
	}
}

const activityEmitFuncName = "emit_activity_event"

func newActivityEmittingRunner(t *testing.T, activityEvents int) *runner.Runner {
	t.Helper()

	emitNode := workflow.NewEmittingFunctionNode(
		"task13_emit_activity_then_pending_crop_gate",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (usecase.CropGateOutput, error) {
			message := ""
			if content := ctx.UserContent(); content != nil {
				for _, p := range content.Parts {
					if p.Text != "" {
						message = p.Text
						break
					}
				}
			}
			for i := 1; i <= activityEvents; i++ {
				ev := session.NewEvent(ctx, ctx.InvocationID())
				ev.Content = &genai.Content{
					Parts: []*genai.Part{{
						FunctionCall: &genai.FunctionCall{
							Name: activityEmitFuncName,
							Args: map[string]any{"payload": usecase.ActivityEventPayload{
								RowID: int64(i), Attempt: 1, MaxAttempts: 1, RetryGroupKey: int64(i),
								Code: string(domain.StepExtractSlots), Text: message,
							}},
						},
					}},
				}
				if err := emit(ev); err != nil {
					return usecase.CropGateOutput{}, err
				}
			}
			return usecase.CropGateOutput{
				Pending:    true,
				Candidates: []domain.CropDisambiguationOption{{Label: "紅蘿蔔"}, {Label: "白蘿蔔"}},
			}, nil
		},
		workflow.NodeConfig{},
	)

	wfAgent, err := workflowagent.New(workflowagent.Config{
		Name:  "task13_activity_emitting_chain",
		Edges: workflow.Chain(workflow.Start, emitNode, usecase.NewCropDisambiguationInterruptNode()),
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

type activityDataJSON struct {
	RowID         string `json:"rowId"`
	Attempt       int    `json:"attempt"`
	MaxAttempts   int    `json:"maxAttempts"`
	RetryGroupKey string `json:"retryGroupKey"`
	Code          string `json:"code"`
	Text          string `json:"text"`
}

func TestSubmitTurnStream_RunAcceptedIsFirstFrame_EvenWhenActivityEmitsEarly(t *testing.T) {
	runStore := &stubRunStore{runID: 9101}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:      runStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        newActivityEmittingRunner(t, 1),
		}, nil, SellerCommandDeps{})
	stream := &fakeAssistantStream{ctx: buyerActorContext("9101")}

	if err := s.SubmitTurnStream(&aipb.TurnRequest{ClientRequestId: "cr-activity-order-1", Message: "我要買蘿蔔"}, stream); err != nil {
		t.Fatalf("SubmitTurnStream failed: %v", err)
	}

	firstActivityIdx := -1
	activityFrames := 0
	for i, ev := range stream.sent {
		if ev.GetActivity() != nil {
			activityFrames++
			if firstActivityIdx == -1 {
				firstActivityIdx = i
			}
		}
	}
	if activityFrames == 0 {
		t.Fatalf("expected at least one Activity frame on the stream, got frames: %s", frameKinds(stream.sent))
	}
	if len(stream.sent) == 0 || stream.sent[0].GetRunAccepted() == nil {
		t.Fatalf("first frame must be RunAccepted, got frames: %s", frameKinds(stream.sent))
	}
	if got := stream.sent[0].GetRunAccepted().GetRunId(); got != "9101" {
		t.Fatalf("RunAccepted.run_id = %q, want 9101", got)
	}
	if firstActivityIdx <= 0 {
		t.Fatalf("Activity frame must come strictly after RunAccepted, got frames: %s", frameKinds(stream.sent))
	}

	var got activityDataJSON
	if err := json.Unmarshal(stream.sent[firstActivityIdx].GetActivity().GetDataJson(), &got); err != nil {
		t.Fatalf("Activity.data_json is not valid JSON: %v (%q)", err, stream.sent[firstActivityIdx].GetActivity().GetDataJson())
	}
	want := activityDataJSON{
		RowID: "1", Attempt: 1, MaxAttempts: 1, RetryGroupKey: "1",
		Code: string(domain.StepExtractSlots), Text: "我要買蘿蔔",
	}
	if got != want {
		t.Fatalf("Activity.data_json = %+v, want %+v", got, want)
	}
}

func TestSubmitTurnStream_PostClaimFailure_SendsRunAcceptedThenError(t *testing.T) {
	runStore := &stubRunStore{runID: 9150}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:      runStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        &runner.Runner{},
		}, nil, SellerCommandDeps{})
	stream := &fakeAssistantStream{ctx: buyerActorContext("9150")}

	err := s.SubmitTurnStream(&aipb.TurnRequest{ClientRequestId: "cr-post-claim-fail-1", Message: "我要買蘿蔔"}, stream)

	if err == nil {
		t.Fatalf("expected a non-nil gRPC error from the post-claim dispatch failure, got nil (frames: %s)", frameKinds(stream.sent))
	}
	if runStore.claims != 1 {
		t.Fatalf("expected the run to have been claimed before failing, got claims=%d", runStore.claims)
	}
	if len(runStore.terminals) != 1 || runStore.terminals[0].Status != domain.RunStatusFailed() {
		t.Fatalf("expected exactly one FAILED TerminalizeRun (proof the failure happened after ClaimRun), got %+v", runStore.terminals)
	}

	if len(stream.sent) != 1 {
		t.Fatalf("expected exactly one frame (RunAccepted) before the error, got frames: %s", frameKinds(stream.sent))
	}
	accepted := stream.sent[0].GetRunAccepted()
	if accepted == nil {
		t.Fatalf("the only frame must be RunAccepted, got frames: %s", frameKinds(stream.sent))
	}
	if accepted.GetRunId() != "9150" {
		t.Fatalf("RunAccepted.run_id = %q, want 9150", accepted.GetRunId())
	}
}

func TestSubmitTurn_UnaryWireShapeUnchangedByActivityEvents(t *testing.T) {
	newServer := func(r *runner.Runner, store *stubRunStore) *AIProcurementGrpcServer {
		return NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
			DurableRunDeps{}, nil,
			usecase.GraphControllerDeps{
				RunStore:      store,
				RevisionStore: successGraphRevisionStore{},
				Dispatcher:    usecase.NewRunDispatcher(),
				Runner:        r,
			}, nil, SellerCommandDeps{})
	}

	quietStore := &stubRunStore{runID: 9111}
	quiet, err := newServer(newTask4PendingCropGateRunner(t), quietStore).SubmitTurn(
		buyerActorContext("9111"),
		&aipb.TurnRequest{ClientRequestId: "cr-unary-quiet-1", Message: "我要買蘿蔔"})
	if err != nil {
		t.Fatalf("SubmitTurn (no activity) failed: %v", err)
	}

	noisyStore := &stubRunStore{runID: 9111}
	noisy, err := newServer(newActivityEmittingRunner(t, 1), noisyStore).SubmitTurn(
		buyerActorContext("9111"),
		&aipb.TurnRequest{ClientRequestId: "cr-unary-noisy-1", Message: "我要買蘿蔔"})
	if err != nil {
		t.Fatalf("SubmitTurn (activity-emitting) failed: %v", err)
	}

	activityLedgerEvents := 0
	for _, ev := range noisyStore.events {
		if ev.EventType == codemeta.ProcurementLedgerEventTypeAgentActivitySnapshot.Value {
			activityLedgerEvents++
		}
	}
	if activityLedgerEvents == 0 {
		t.Fatalf("test bug: the activity-emitting runner produced no Activity ledger event, so comparing the two responses proves nothing (events=%+v)", noisyStore.events)
	}
	if quietActivity := len(quietStore.events); quietActivity != 0 {
		t.Fatalf("test bug: the baseline runner was supposed to emit no run events at all, got %+v", quietStore.events)
	}

	if !proto.Equal(quiet, noisy) {
		t.Fatalf("Activity events changed SubmitTurn's unary response shape:\n without activity: %+v\n with activity:    %+v", quiet, noisy)
	}
}

func TestSubmitTurnStream_ConcurrentRequests_DoNotCrossDeliverActivity(t *testing.T) {
	const isolationActivityEvents = 4

	runStore := &stubRunStore{
		runID: 9200,
		runIDByClientRequestID: map[string]int64{
			"cr-isolation-a": 9201,
			"cr-isolation-b": 9202,
		},
	}
	s := NewAIProcurementGrpcServer(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		DurableRunDeps{}, nil,
		usecase.GraphControllerDeps{
			RunStore:      runStore,
			RevisionStore: successGraphRevisionStore{},
			Dispatcher:    usecase.NewRunDispatcher(),
			Runner:        newActivityEmittingRunner(t, isolationActivityEvents),
		}, nil, SellerCommandDeps{})

	type concurrentCall struct {
		buyerID   string
		requestID string
		clientReq string
		message   string
		wantRunID string
		stream    *fakeAssistantStream
		err       error
	}
	calls := []*concurrentCall{
		{buyerID: "9201", requestID: "7301", clientReq: "cr-isolation-a", message: "買家甲要買紅蘿蔔", wantRunID: "9201"},
		{buyerID: "9202", requestID: "7302", clientReq: "cr-isolation-b", message: "買家乙要買白蘿蔔", wantRunID: "9202"},
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, c := range calls {
		c.stream = &fakeAssistantStream{ctx: buyerActorContext(c.buyerID)}
		wg.Add(1)
		go func(c *concurrentCall) {
			defer wg.Done()
			<-start
			c.err = s.SubmitTurnStream(&aipb.TurnRequest{
				RequestId: c.requestID, ClientRequestId: c.clientReq, Message: c.message,
			}, c.stream)
		}(c)
	}
	close(start)
	wg.Wait()

	for _, c := range calls {
		if c.err != nil {
			t.Fatalf("SubmitTurnStream(%s) failed: %v", c.clientReq, c.err)
		}
		if len(c.stream.sent) == 0 || c.stream.sent[0].GetRunAccepted() == nil {
			t.Fatalf("%s: first frame must be RunAccepted, got frames: %s", c.clientReq, frameKinds(c.stream.sent))
		}
		if got := c.stream.sent[0].GetRunAccepted().GetRunId(); got != c.wantRunID {
			t.Fatalf("%s: RunAccepted.run_id = %q, want %q — 這個 stream 拿到的是別條 run 的 id",
				c.clientReq, got, c.wantRunID)
		}
		var texts []string
		for _, ev := range c.stream.sent {
			activity := ev.GetActivity()
			if activity == nil {
				continue
			}
			var payload activityDataJSON
			if err := json.Unmarshal(activity.GetDataJson(), &payload); err != nil {
				t.Fatalf("%s: Activity.data_json is not valid JSON: %v", c.clientReq, err)
			}
			texts = append(texts, payload.Text)
		}
		if len(texts) != isolationActivityEvents {
			t.Fatalf("%s: expected exactly %d Activity frames (this request's own), got %d: %q — frames: %s",
				c.clientReq, isolationActivityEvents, len(texts), texts, frameKinds(c.stream.sent))
		}
		for i, got := range texts {
			if got != c.message {
				t.Fatalf("%s: Activity frame %d carries another request's payload: text=%q, want %q",
					c.clientReq, i, got, c.message)
			}
		}
	}
}

func frameKinds(events []*aipb.StreamEvent) string {
	kinds := make([]string, 0, len(events))
	for _, ev := range events {
		switch {
		case ev.GetRunAccepted() != nil:
			kinds = append(kinds, "RunAccepted")
		case ev.GetActivity() != nil:
			kinds = append(kinds, "Activity")
		case ev.GetStep() != nil:
			kinds = append(kinds, "Step")
		case ev.GetMessage() != nil:
			kinds = append(kinds, "Message")
		case ev.GetTurnResult() != nil:
			kinds = append(kinds, "TurnResult")
		case ev.GetDone() != nil:
			kinds = append(kinds, "Done")
		default:
			kinds = append(kinds, "Unknown")
		}
	}
	if len(kinds) == 0 {
		return "(none)"
	}
	out := kinds[0]
	for _, k := range kinds[1:] {
		out += ", " + k
	}
	return out
}
