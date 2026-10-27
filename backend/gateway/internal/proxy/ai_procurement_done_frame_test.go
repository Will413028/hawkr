package proxy

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }

// fullyPopulatedTurnResultProto is the wire form of the same fixture:
// ai-service's toTurnResultProto applied to the usecase.TurnResult the
// ai-service-side test marshals (its
// TestToTurnResultProto_CarriesEveryField pins that conversion field by field,
// including the ID/quantity int→string transposition this side undoes).
func fullyPopulatedTurnResultProto() *aipb.TurnResult {
	return &aipb.TurnResult{
		RequestId: "7001", ConversationId: "7002", RevisionId: "7003",
		Slots: &aipb.Slots{
			Item: sp("寶島梨"), Quantity: sp("50"), Unit: sp("箱"),
			Packaging: sp("紙箱"), BudgetPerUnit: sp("1200"), DeliveryCity: sp("臺中市"),
			DeliveryDate: sp("2026-09-01"), ExactMatchRequired: bp(true),
		},
		Rejected:         []*aipb.RejectedField{{Key: "delivery_date", Reason: "到貨日不可早於今天，已忽略"}},
		MissingRequired:  []string{"unit"},
		RecordedThisTurn: []*aipb.RecordedField{{Key: "item", Label: "品項", Value: "寶島梨"}},
		Degraded:         true,
		DegradedReason:   "需求解析暫時無法使用，已保留你目前的需求內容",
		Reply:            "已為您記下品項與數量",
		Steps:            []*aipb.Step{{Code: "EXTRACT_SLOTS", Status: "DONE", Label: "抓出品項、數量和交期", DurationMs: 340}},
		Question: &aipb.ClarifyQuestion{
			SlotKey: "quantity", Prompt: "請問需要多少數量？",
			Options:       []*aipb.QuestionOption{{OptionId: "opt-50", Label: "50", Value: "50"}, {OptionId: "opt-100", Label: "100", Value: "100"}},
			UnitOptions:   []*aipb.QuestionOption{{OptionId: "unit-box", Label: "箱", Value: "箱"}},
			AllowFreeText: true, Source: "FIXED", CanSkip: false,
		},
	}
}

func TestWriteStreamEvent_Step_EmitsStepStartedOnRunning(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{
		TurnId: "turn-1", Seq: 1, Code: "EXTRACT_SLOTS", Status: "RUNNING", Label: "抓出品項",
	}}}, state, "thread-1", "run-1")

	out := buf.String()
	if !strings.Contains(out, `"type":"STEP_STARTED","stepName":"EXTRACT_SLOTS"`) {
		t.Fatalf("expected STEP_STARTED, got %q", out)
	}
	if !strings.Contains(out, `"type":"CUSTOM","name":"step_detail"`) || !strings.Contains(out, `"turnId":"turn-1"`) {
		t.Fatalf("expected a step_detail CUSTOM event carrying turnId/label, got %q", out)
	}
}

// 三個 case 缺一不可：
//   - 正向 SUCCEEDED：契約凍結後唯一的成功終態值。
//   - 負向 RUNNING：沒有它的話，把 isStepFinished 寫成 `return true` 也會全綠。
//   - 負向 DONE：改名前的舊值，本波起**必須不再被當成完成**。留著這個 case 是為了讓
//     「悄悄把舊值加回去」這件事一定會踩紅——那正是這波要收斂掉的相容層。
func TestWriteStreamEvent_Step_StepFinishedOnTerminalStatus(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		wantFinished bool
	}{
		{"契約凍結後的成功終態", codemeta.ProcurementStepStatusSucceeded.Value, true},
		{"非終態不得判成完成", codemeta.ProcurementStepStatusRunning.Value, false},
		{"改名前的舊值不得再被接受", "DONE", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			state := &agUIStreamState{}
			writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{
				TurnId: "turn-1", Seq: 1, Code: "EXTRACT_SLOTS", Status: tt.status, Label: "抓出品項", DurationMs: 340,
			}}}, state, "thread-1", "run-1")

			out := buf.String()
			gotFinished := strings.Contains(out, `"type":"STEP_FINISHED","stepName":"EXTRACT_SLOTS"`)
			gotStarted := strings.Contains(out, `"type":"STEP_STARTED","stepName":"EXTRACT_SLOTS"`)
			if gotFinished != tt.wantFinished || gotStarted == tt.wantFinished {
				t.Fatalf("status=%q → finished=%v started=%v, want finished=%v\nout=%q",
					tt.status, gotFinished, gotStarted, tt.wantFinished, out)
			}
			if !strings.Contains(out, `"durationMs":340`) {
				t.Fatalf("expected durationMs in step_detail, got %q", out)
			}
		})
	}
}

func TestWriteStreamEvent_Message_EmitsFullTextMessageCycle(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Message{Message: &aipb.MessageEvent{
		TurnId: "turn-1", Reply: "已為您記下品項與數量",
	}}}, state, "thread-1", "run-1")

	out := buf.String()
	wantStart := `data: {"type":"TEXT_MESSAGE_START","messageId":"turn-1","role":"assistant"}` + "\n\n"
	wantContent := `data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"turn-1","delta":"已為您記下品項與數量"}` + "\n\n"
	wantEnd := `data: {"type":"TEXT_MESSAGE_END","messageId":"turn-1"}` + "\n\n"
	if out != wantStart+wantContent+wantEnd {
		t.Fatalf("got %q want %q", out, wantStart+wantContent+wantEnd)
	}
}

// TestWriteStreamEvent_Message_DropsReasoning：AG-UI frame 不得出現 reasoning
// key（#2447 BE-01A——這裡曾是 CUSTOM name="reasoning" 的曝露點）。
//
// proto 的 MessageEvent.reasoning 已於 #2447 reserved 化，輸入端不再可能帶值，
// 主要護欄因此上移到 protoc（重新加回同名欄位會編譯失敗）。這支測試留著守的是
// 另一個方向：gateway 自己從別處合成一個 reasoning key。
func TestWriteStreamEvent_Message_DropsReasoning(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Message{Message: &aipb.MessageEvent{
		TurnId: "turn-1", Reply: "好的",
	}}}, state, "thread-1", "run-1")

	if strings.Contains(buf.String(), "reasoning") {
		t.Fatalf("reasoning 不得出現在任何 AG-UI frame, got %q", buf.String())
	}
	if !strings.Contains(buf.String(), `"delta":"好的"`) {
		t.Fatalf("正常 reply 應照常送出, got %q", buf.String())
	}
}

func TestWriteStreamEvent_TurnResult_EmitsStateSnapshotThenRunFinished(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{
		Payload: &aipb.StreamEvent_TurnResult{TurnResult: fullyPopulatedTurnResultProto()},
	}, state, "thread-1", "run-1")

	out := buf.String()
	parts := strings.Split(strings.TrimSuffix(out, "\n\n"), "\n\n")
	if len(parts) != 2 {
		t.Fatalf("expected exactly 2 frames (STATE_SNAPSHOT + RUN_FINISHED), got %d: %q", len(parts), out)
	}
	if !strings.HasPrefix(parts[0], `data: {"type":"STATE_SNAPSHOT"`) {
		t.Fatalf("frame 1 should be STATE_SNAPSHOT, got %q", parts[0])
	}
	if !strings.Contains(parts[0], `"requestId":"7001"`) {
		t.Fatalf("STATE_SNAPSHOT should carry the TurnResult fields, got %q", parts[0])
	}
	if !strings.HasPrefix(parts[1], `data: {"type":"RUN_FINISHED","threadId":"thread-1","runId":"run-1"`) {
		t.Fatalf("frame 2 should be RUN_FINISHED, got %q", parts[1])
	}
	if !strings.Contains(parts[1], `"requestId":"7001"`) {
		t.Fatalf("RUN_FINISHED.result should also carry the TurnResult, got %q", parts[1])
	}
	// Sentinel（#2447 BE-01A）：STATE_SNAPSHOT／RUN_FINISHED 兩個 frame 都不得
	// 出現 reasoning key。proto 欄位已 reserved 化、fixture 不再 populate，這裡守
	// 的是 TurnResultDTO 從別處長出這個 key。
	if strings.Contains(out, "reasoning") {
		t.Fatalf("reasoning 不得出現在 STATE_SNAPSHOT/RUN_FINISHED, got %q", out)
	}
}

func TestWriteStreamEvent_TurnResult_NullQuestionStaysExplicitNull(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_TurnResult{TurnResult: &aipb.TurnResult{
		RequestId: "1", ConversationId: "2", RevisionId: "3", Reply: "好的",
	}}}, state, "thread-1", "run-1")
	out := buf.String()
	if !strings.Contains(out, `"question":null`) {
		t.Fatalf("question must serialize as explicit null, got %q", out)
	}
}

// TestWriteStreamEvent_TurnResult_CarriesBlocksInBothFrames is Task 8 (#2549
// Phase 2): confirms the PublicAgentMessageV1 fields added to TurnResultDTO in
// Task 7 (Blocks/DialoguePlan) reach the SSE stream path automatically,
// because writeStreamEvent's StreamEvent_TurnResult case has no per-field
// logic of its own — it marshals TurnResultFromProto's DTO into both the
// STATE_SNAPSHOT and RUN_FINISHED frames (see
// TestWriteStreamEvent_TurnResult_EmitsStateSnapshotThenRunFinished above for
// that two-frame shape). This is pure regression coverage; no production code
// change is expected to make it pass.
func TestWriteStreamEvent_TurnResult_CarriesBlocksInBothFrames(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_TurnResult{TurnResult: &aipb.TurnResult{
		RequestId: "1", ConversationId: "2", RevisionId: "3",
		Reply:        "已收到你的訊息。",
		Blocks:       []*aipb.Block{{Kind: "UNDERSTOOD", Text: "已收到你的訊息。"}},
		DialoguePlan: &aipb.DialoguePlanSummary{ActCode: "ACK_AND_ASK_REQUIRED"},
	}}}, state, "thread-1", "run-1")

	out := buf.String()
	if strings.Count(out, `"kind":"UNDERSTOOD"`) != 2 {
		t.Fatalf("expected blocks to appear in both STATE_SNAPSHOT and RUN_FINISHED frames, got: %s", out)
	}
	if strings.Count(out, `"dialogueActCode":"ACK_AND_ASK_REQUIRED"`) != 2 {
		t.Fatalf("expected dialoguePlan to appear in both STATE_SNAPSHOT and RUN_FINISHED frames, got: %s", out)
	}
}

// TestWriteStreamEvent_TurnResult_IncludesSuccessOutcome is Task 13 Step 4:
// the normal (no interrupt) case — TurnResult.interrupt_id is empty because
// this turn's terminal status genuinely isn't WAITING_FOR_INPUT — must
// render as {"type":"success"} on the
// RUN_FINISHED frame's outcome field. fullyPopulatedTurnResultProto() never
// sets InterruptId, so it exercises exactly this path.
func TestWriteStreamEvent_TurnResult_IncludesSuccessOutcome(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{
		Payload: &aipb.StreamEvent_TurnResult{TurnResult: fullyPopulatedTurnResultProto()},
	}, state, "thread-1", "run-1")

	out := buf.String()
	if !strings.Contains(out, `"outcome":{"type":"success"}`) {
		t.Fatalf(`expected RUN_FINISHED frame to carry "outcome":{"type":"success"}, got %q`, out)
	}
	// Outcome only belongs on RUN_FINISHED, not on the preceding STATE_SNAPSHOT
	// frame — result/snapshot payload is the domain object, outcome is a
	// separate AG-UI-level signal about how the run ended.
	parts := strings.Split(strings.TrimSuffix(out, "\n\n"), "\n\n")
	if len(parts) != 2 {
		t.Fatalf("expected exactly 2 frames (STATE_SNAPSHOT + RUN_FINISHED), got %d: %q", len(parts), out)
	}
	if strings.Contains(parts[0], "outcome") {
		t.Fatalf("STATE_SNAPSHOT must not carry an outcome field, got %q", parts[0])
	}
	if !strings.Contains(parts[1], "outcome") {
		t.Fatalf("RUN_FINISHED must carry an outcome field, got %q", parts[1])
	}
}

// TestWriteStreamEvent_TurnResult_IncludesInterruptOutcome pins the shape
// doc57 §9.3 requires when a turn's terminal status is genuinely
// WAITING_FOR_INPUT (v1's only trigger: crop 消歧待選, cropidentity.ComputeGate
// returning Pending: true) — TurnResult.interrupt_id/interrupt_kind
// (additive fields 22/23 on TurnResult, Task 7's chosen design: option (a),
// not a new StreamEvent oneof case) must surface as
// {"type":"interrupt","interrupts":[{"id":...,"reason":...}]} on
// RUN_FINISHED — matching the official @ag-ui/core RunFinishedOutcomeSchema's
// array-wrapped shape, not a flat interruptId/reason pair — so the client
// does not mistake this for an ordinary completed turn and silently
// retry/downgrade it.
func TestWriteStreamEvent_TurnResult_IncludesInterruptOutcome(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	tr := fullyPopulatedTurnResultProto()
	tr.InterruptId = "interrupt-9001"
	tr.InterruptKind = "CLARIFICATION"
	writeStreamEvent(&buf, &aipb.StreamEvent{
		Payload: &aipb.StreamEvent_TurnResult{TurnResult: tr},
	}, state, "thread-1", "run-1")

	out := buf.String()
	const want = `"outcome":{"type":"interrupt","interrupts":[{"id":"interrupt-9001","reason":"CLARIFICATION"}]}`
	if !strings.Contains(out, want) {
		t.Fatalf("expected RUN_FINISHED frame to carry %s, got %q", want, out)
	}
	if strings.Contains(out, `"type":"success"`) {
		t.Fatalf("must not also render as success outcome, got %q", out)
	}
}

// TestWriteStreamEvent_TurnResult_EmptyInterruptIDDoesNotFabricateInterrupt
// pins the documented, deliberate limitation Task 7's review left behind
// (replayExistingRun's idempotent replay of a WAITING_FOR_INPUT run does NOT
// currently backfill InterruptID/InterruptKind — see that function's in-code
// comment): when interrupt_id is empty, this task must render success even
// if interrupt_kind alone happens to be non-empty, because the discriminated
// union is keyed on interrupt_id (per Task 13's brief), and fabricating or
// guessing an interrupt from a partial signal is explicitly out of scope —
// carrying the gap forward honestly, not papering over it.
func TestWriteStreamEvent_TurnResult_EmptyInterruptIDDoesNotFabricateInterrupt(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	tr := fullyPopulatedTurnResultProto()
	tr.InterruptKind = "CLARIFICATION" // interrupt_id left empty — must not synthesize an interrupt outcome
	writeStreamEvent(&buf, &aipb.StreamEvent{
		Payload: &aipb.StreamEvent_TurnResult{TurnResult: tr},
	}, state, "thread-1", "run-1")

	out := buf.String()
	if !strings.Contains(out, `"outcome":{"type":"success"}`) {
		t.Fatalf(`expected success outcome when interrupt_id is empty (even with interrupt_kind set), got %q`, out)
	}
}

// TestServeAIProcurementStream_ErrorFrameCarriesHTTPStatus is I3: the SSE error
// frame's numeric `code` must be an HTTP status, not the raw gRPC code.
// hawkr-website's useProcurementSession.ts branches on exactly
// err.code === 404 (CONVERSATION_GONE) and err.code === 409
// (IDEMPOTENCY_CONFLICT); relaying st.Code() verbatim puts 5 and 10 on the
// wire and both branches fall through to a generic failure.
func TestServeAIProcurementStream_ErrorFrameCarriesHTTPStatus(t *testing.T) {
	cases := []struct {
		name     string
		grpcCode codes.Code
		message  string
		wantCode string
	}{
		{"NotFound → 404", codes.NotFound, "request not found", `"code":"404"`},
		{"Aborted → 409", codes.Aborted, "clientRequestId 已用於其他需求，請改用新的 clientRequestId", `"code":"409"`},
		{"Internal → 500", codes.Internal, "internal error", `"code":"500"`},
		{"FailedPrecondition → 422", codes.FailedPrecondition, "單則訊息上限 2000 字", `"code":"422"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/x", nil)

			stream := &fakeAssistantStream{err: status.Error(tc.grpcCode, tc.message)}
			ServeAIProcurementStream(c, "thread-1", func(context.Context) (grpcclient.AssistantStream, error) {
				return stream, nil
			})

			body := w.Body.String()
			if !strings.Contains(body, tc.wantCode) {
				t.Fatalf("error frame should carry %s, got %q", tc.wantCode, body)
			}
			if !strings.Contains(body, tc.message) {
				t.Fatalf("error frame lost the message: %q", body)
			}
		})
	}
}

// TestServeAIAssistantStream_ErrorFrameKeepsRawGRPCCode is the other side of
// I3's scope decision: Assistant's 12 live SSE endpoints share this file, and
// their behavior is deliberately NOT changed here (their Flutter consumers read
// only `message`, never `code`; changing it would be an unrequested behavior
// change on a live path). This test exists so the split is explicit rather than
// accidental — if someone later "unifies" the two, they have to delete this
// test on purpose.
func TestServeAIAssistantStream_ErrorFrameKeepsRawGRPCCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	stream := &fakeAssistantStream{err: status.Error(codes.NotFound, "session not found")}
	ServeAIAssistantStream(c, "", func(context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	if !strings.Contains(w.Body.String(), `"code":"5"`) {
		t.Fatalf("Assistant's raw-gRPC-code error frame changed unexpectedly: %q", w.Body.String())
	}
}

// TestServeAIProcurementStream_QuotaExceededFrameCarriesReasonRemainingResetAt
// is the SSE half of doc 57 §4.7 (`AGENT_MONTHLY_QUOTA_EXCEEDED` must reach the
// client with `remaining=0` ＋ `resetAt`).
//
// ai-service populates all four BusinessError fields
// (services/ai-service/internal/grpc/procurement_errors.go
// procurementQuotaExceededStatus), but before this test the stream path read
// nothing off st.Details() — it emitted only `code":"429"` plus the human
// message, so the two fields Task 10 added to the proto for this exact purpose
// died at the gateway boundary. Nothing else on the branch would have gone red:
// the fields were correctly populated right up to the gRPC boundary.
//
// The buyer-facing procurement turn is an SSE stream, so this — not the unary
// path — is where a real buyer hits the quota wall.
func TestServeAIProcurementStream_QuotaExceededFrameCarriesReasonRemainingResetAt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	st := status.New(codes.ResourceExhausted, "本月 Agent 回答次數已用完（30/30），2026-09-01T00:00:00Z 後重置")
	stWithDetails, err := st.WithDetails(&aipb.BusinessError{
		Reason:    codemeta.ProcurementErrorReasonAgentMonthlyQuotaExceeded.Value,
		Message:   "本月 Agent 回答次數已用完（30/30），2026-09-01T00:00:00Z 後重置",
		Remaining: "0",
		ResetAt:   "2026-09-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("WithDetails failed: %v", err)
	}

	stream := &fakeAssistantStream{err: stWithDetails.Err()}
	ServeAIProcurementStream(c, "thread-1", func(context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	for _, want := range []string{
		`"code":"429"`,
		`"reason":"` + codemeta.ProcurementErrorReasonAgentMonthlyQuotaExceeded.Value + `"`,
		`"remaining":"0"`,
		`"resetAt":"2026-09-01T00:00:00Z"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("RUN_ERROR frame missing %s (doc 57 §4.7): %q", want, body)
		}
	}
}

// TestServeAIProcurementStream_NonBusinessErrorFrameKeepsItsShape is the
// companion guard: the three new RUN_ERROR keys are additive and omitempty, so
// a failure WITHOUT a BusinessError detail (transport failures, and all 12
// AIAssistantService streaming endpoints) must still emit byte-identical frames
// — otherwise every error frame on the platform grows three permanently-empty
// keys, and testdata/agui-frames/run-error.sse (copied to hawkr-website
// for @ag-ui/core zod validation) would have to be re-synced.
func TestServeAIProcurementStream_NonBusinessErrorFrameKeepsItsShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	stream := &fakeAssistantStream{err: status.Error(codes.Unavailable, "connection refused")}
	ServeAIProcurementStream(c, "thread-1", func(context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	for _, unwanted := range []string{`"reason"`, `"remaining"`, `"resetAt"`} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("non-BusinessError RUN_ERROR frame gained %s: %q", unwanted, body)
		}
	}
}

// TestServeAIProcurementStream_UsesRunAcceptedRunID_NotTraceID is Task 13
// Step 3/4: the procurement stream's RUN_STARTED/RUN_FINISHED frames must
// carry the real ledger run_id from Task 7's RunAccepted StreamEvent (the
// guaranteed-first frame, sent right after the claim transaction commits),
// not c.GetString("trace_id") — trace_id and the ledger run_id are
// different identities (doc57 §6 落差 2); using trace_id was the bug this
// task fixes.
func TestServeAIProcurementStream_UsesRunAcceptedRunID_NotTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)
	c.Set("trace_id", "trace-should-not-appear")

	stream := &fakeAssistantStream{events: []*aipb.StreamEvent{
		{Payload: &aipb.StreamEvent_RunAccepted{RunAccepted: &aipb.RunAccepted{RunId: "12345"}}},
		{Payload: &aipb.StreamEvent_TurnResult{TurnResult: fullyPopulatedTurnResultProto()}},
	}}
	ServeAIProcurementStream(c, "thread-1", func(context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if !strings.Contains(body, `"type":"RUN_STARTED","threadId":"thread-1","runId":"12345"`) {
		t.Fatalf("expected RUN_STARTED to carry runId %q from RunAccepted, got %q", "12345", body)
	}
	if !strings.Contains(body, `"type":"RUN_FINISHED","threadId":"thread-1","runId":"12345"`) {
		t.Fatalf("expected RUN_FINISHED to carry runId %q from RunAccepted, got %q", "12345", body)
	}
	if strings.Contains(body, "trace-should-not-appear") {
		t.Fatalf("trace_id must not leak into any frame once RunAccepted supplied the real ledger run_id, got %q", body)
	}
	// RunAccepted itself must not produce its own AG-UI frame — it's a
	// transport-only signal consumed by serveAIAssistantStreamWith, not
	// something writeStreamEvent translates. Expect exactly 3 frames:
	// RUN_STARTED, STATE_SNAPSHOT, RUN_FINISHED.
	if got := strings.Count(body, "data: "); got != 3 {
		t.Fatalf("expected exactly 3 frames (RUN_STARTED, STATE_SNAPSHOT, RUN_FINISHED), got %d in %q", got, body)
	}
	if strings.Count(body, "RUN_STARTED") != 1 {
		t.Fatalf("expected exactly one RUN_STARTED frame, got %q", body)
	}
}

// TestServeAIProcurementStream_FallsBackToTraceIDWhenNoRunAccepted is Task 13
// Step 3(c)'s fallback: if the first event is not RunAccepted, the very
// first event is real content instead. SubmitTurnStream always sends
// RunAccepted as the guaranteed-first frame, so this signals a protocol
// break rather than an expected configuration (see serveAIAssistantStreamWith's
// doc comment). It must still not hang or drop that event: fall back to trace_id
// (same value the non-deferred/12-endpoint path would use) and still relay
// the event that triggered the fallback.
func TestServeAIProcurementStream_FallsBackToTraceIDWhenNoRunAccepted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)
	c.Set("trace_id", "trace-abc")

	stream := &fakeAssistantStream{events: []*aipb.StreamEvent{
		{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{TurnId: "t1", Code: "EXTRACT_SLOTS", Status: "RUNNING", Label: "解析中"}}},
		{Payload: &aipb.StreamEvent_TurnResult{TurnResult: fullyPopulatedTurnResultProto()}},
	}}
	ServeAIProcurementStream(c, "thread-1", func(context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if !strings.Contains(body, `"type":"RUN_STARTED","threadId":"thread-1","runId":"trace-abc"`) {
		t.Fatalf("expected RUN_STARTED to fall back to trace_id when no RunAccepted arrived, got %q", body)
	}
	if !strings.Contains(body, `"type":"RUN_FINISHED","threadId":"thread-1","runId":"trace-abc"`) {
		t.Fatalf("expected RUN_FINISHED to also use the trace_id fallback, got %q", body)
	}
	// The Step event that triggered the fallback must still be relayed, not
	// silently dropped.
	if !strings.Contains(body, `"type":"STEP_STARTED","stepName":"EXTRACT_SLOTS"`) {
		t.Fatalf("expected the first (non-RunAccepted) event to still be relayed, got %q", body)
	}
}

// TestServeAIProcurementStream_RunStartedPrecedesRunErrorWhenFirstRecvFails
// covers the edge case serveAIAssistantStreamWith's doc comment calls out:
// deferRunStarted is true and the very first Recv() itself fails, before any
// event (RunAccepted or otherwise) ever arrived. RUN_STARTED must still be
// written (trace_id fallback) before the terminal RUN_ERROR frame — a client
// must never see a terminal frame without a preceding RUN_STARTED.
func TestServeAIProcurementStream_RunStartedPrecedesRunErrorWhenFirstRecvFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)
	c.Set("trace_id", "trace-xyz")

	stream := &fakeAssistantStream{err: status.Error(codes.Internal, "boom")}
	ServeAIProcurementStream(c, "thread-1", func(context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if !strings.Contains(body, `"type":"RUN_STARTED","threadId":"thread-1","runId":"trace-xyz"`) {
		t.Fatalf("expected a RUN_STARTED frame with the trace_id fallback before the error, got %q", body)
	}
	startIdx := strings.Index(body, "RUN_STARTED")
	errIdx := strings.Index(body, "RUN_ERROR")
	if startIdx < 0 || errIdx < 0 || startIdx >= errIdx {
		t.Fatalf("expected RUN_STARTED before RUN_ERROR, got %q", body)
	}
}
