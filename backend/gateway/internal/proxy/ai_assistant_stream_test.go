package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeAssistantStream struct {
	grpc.ClientStream
	events []*aipb.StreamEvent
	err    error // 若非 nil，events 送完後回傳這個 error 而不是 io.EOF
	idx    int
}

func (s *fakeAssistantStream) Recv() (*aipb.StreamEvent, error) {
	if s.idx < len(s.events) {
		e := s.events[s.idx]
		s.idx++
		return e, nil
	}
	if s.err != nil {
		return nil, s.err
	}
	return nil, io.EOF
}

// deadlineRecorder is an httptest.ResponseRecorder that also records every
// SetWriteDeadline call, so a test can prove the SSE path really arms a
// per-write deadline. It keeps the recorder's Flush so gin's
// responseWriter.Flush (an unchecked http.Flusher type assertion) still works.
type deadlineRecorder struct {
	*httptest.ResponseRecorder
	deadlines []time.Time
}

func (w *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	w.deadlines = append(w.deadlines, t)
	return nil
}

// TestServeAIAssistantStream_ArmsPerWriteDeadline proves the stream path
// wraps c.Writer in deadlineResponseWriter: without it a client that keeps
// the TCP connection open but stops reading would block Write forever —
// neither gRPC keepalive nor streamTurnDeadline bound this goroutine, since
// both only cover the UPSTREAM/Recv side, not a stalled client-side Write.
func TestServeAIAssistantStream_ArmsPerWriteDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := &deadlineRecorder{ResponseRecorder: httptest.NewRecorder()}
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	stream := &fakeAssistantStream{events: []*aipb.StreamEvent{
		{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "a"}}},
		{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "b"}}},
		{Payload: &aipb.StreamEvent_Done{Done: &aipb.DoneEvent{SessionId: "s1"}}},
	}}

	ServeAIAssistantStream(c, "", func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	if len(rec.deadlines) != 6 {
		t.Fatalf("expected one SetWriteDeadline per AG-UI frame (6: RUN_STARTED, TEXT_MESSAGE_START, "+
			"2x TEXT_MESSAGE_CONTENT, TEXT_MESSAGE_END, RUN_FINISHED), got %d", len(rec.deadlines))
	}
	for i, dl := range rec.deadlines {
		if got := time.Until(dl); got <= 0 || got > streamWriteTimeout {
			t.Fatalf("deadline #%d not within (0, %v] of now: got %v ahead", i, streamWriteTimeout, got)
		}
	}
}

// TestServeAIAssistantStream_EmitsRunStartedBeforeFirstEvent proves the
// stream's very first frame is a RUN_STARTED event carrying threadId, before
// any event derived from the upstream Recv loop.
func TestServeAIAssistantStream_EmitsRunStartedBeforeFirstEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	stream := &fakeAssistantStream{events: []*aipb.StreamEvent{
		{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "hi"}}},
		{Payload: &aipb.StreamEvent_Done{Done: &aipb.DoneEvent{SessionId: "s1"}}},
	}}

	ServeAIAssistantStream(c, "session-42", func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if !strings.HasPrefix(body, `data: {"type":"RUN_STARTED","threadId":"session-42","runId":"`) {
		t.Fatalf("expected RUN_STARTED as the first frame carrying threadId, got %q", body)
	}
	if strings.Count(body, "RUN_STARTED") != 1 {
		t.Fatalf("RUN_STARTED must be emitted exactly once per stream, got %q", body)
	}
}

// TestServeAIAssistantStream_RunIdIsStableAcrossOneStream proves runId is
// generated once per stream (from c.GetString("trace_id")) and reused for
// every frame that carries one, not regenerated per event.
func TestServeAIAssistantStream_RunIdIsStableAcrossOneStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)
	// In production, middleware.Tracing sets "trace_id" in c before any
	// handler runs; this bare gin.CreateTestContext skips that middleware
	// chain, so seed it directly to exercise the "stable across a stream"
	// property meaningfully (an empty runId would make every frame trivially
	// "equal", which isn't what this test is proving).
	c.Set("trace_id", "trace-abc")

	stream := &fakeAssistantStream{events: []*aipb.StreamEvent{
		{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "a"}}},
		{Payload: &aipb.StreamEvent_Done{Done: &aipb.DoneEvent{SessionId: "s1"}}},
	}}
	ServeAIAssistantStream(c, "t1", func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	re := regexp.MustCompile(`"runId":"([^"]+)"`)
	matches := re.FindAllStringSubmatch(body, -1)
	if len(matches) < 2 {
		t.Fatalf("expected at least 2 frames carrying runId (RUN_STARTED + RUN_FINISHED), got %q", body)
	}
	first := matches[0][1]
	for _, m := range matches {
		if m[1] != first {
			t.Fatalf("runId changed within one stream: %q vs %q in %q", first, m[1], body)
		}
	}
}

func TestServeAIAssistantStream_OpenFailureFailsFastNoRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	openCalls := 0
	ServeAIAssistantStream(c, "", func(ctx context.Context) (grpcclient.AssistantStream, error) {
		openCalls++
		return nil, status.Error(codes.Unavailable, "ai-service down")
	})

	if openCalls != 1 {
		t.Fatalf("expected exactly one open attempt (no retry), got %d", openCalls)
	}
	// open failed before a single byte went out, so the client must get a real
	// HTTP error (the shared upstream-failure envelope) — NOT a 200 carrying
	// an SSE error frame, which both Flutter apps would render as silence.
	//
	// 424 not 502: this endpoint's consumers are our own frontends, and
	// Cloudflare replaces the body of any 5xx the origin emits, so the
	// envelope would never reach them. The "statusCode != 200" premise this
	// test's comment relies on holds either way; 424 additionally keeps the
	// body. See UpstreamFailureStatus in reverse_proxy.go.
	if w.Code != http.StatusFailedDependency {
		t.Fatalf("expected HTTP 424 on open failure, got %d (body %q)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected a JSON envelope, got Content-Type %q", ct)
	}
	if strings.Contains(w.Body.String(), "event:") {
		t.Fatalf("open failure must not emit an SSE frame, got %q", w.Body.String())
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("body is not a JSON envelope: %v (%q)", err, w.Body.String())
	}
	if env.Code != 424 || env.Message != upstreamFailureMessage {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	// The upstream gRPC status message must NOT reach the user: this envelope
	// is now actually delivered (4xx), and the Flutter apps render `message`
	// verbatim. Details belong in the log, correlated via traceId.
	if strings.Contains(w.Body.String(), "ai-service down") {
		t.Fatalf("upstream error text leaked into the response body: %q", w.Body.String())
	}
}

// TestServeAIAssistantStream_MidStreamRecvErrorAfterDataDoesNotLeakGRPCError
// pins a real 2026-08-16 production incident: a Recv() error arriving AFTER
// some data was already streamed (not just as the very first event) must
// still produce a well-formed terminal RUN_ERROR frame, and the raw Go error
// string (e.g. "rpc error: code = ...") must never leak into the
// client-facing `message` field — only st.Message() may reach the client.
func TestServeAIAssistantStream_MidStreamRecvErrorAfterDataDoesNotLeakGRPCError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	stream := &fakeAssistantStream{
		events: []*aipb.StreamEvent{
			{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "hi"}}},
		},
		err: status.Error(codes.InvalidArgument, "message is required"),
	}

	ServeAIAssistantStream(c, "", func(ctx context.Context) (grpcclient.AssistantStream, error) {
		return stream, nil
	})

	body := w.Body.String()
	if !strings.Contains(body, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected a RUN_ERROR frame, got %q", body)
	}
	if !strings.Contains(body, "message is required") {
		t.Fatalf("expected the gRPC status message to reach the client, got %q", body)
	}
	if strings.Contains(body, "rpc error") {
		t.Fatalf("the raw Go gRPC error string must never leak into the client-facing message, got %q", body)
	}
	if !strings.Contains(body, `"delta":"hi"`) {
		t.Fatalf("expected the earlier text content to still be present before the error, got %q", body)
	}
	if idx := strings.Index(body, `"delta":"hi"`); idx >= strings.Index(body, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected text content before RUN_ERROR, got %q", body)
	}
}

// blockingAssistantStream is a fake stream that blocks forever until its
// context ends (cancelled or, as used below, its deadline expiring),
// simulating a real gRPC stream whose Recv() only unblocks when the RPC's
// context ends.
//
// Recv() returns status.FromContextError(s.ctx.Err()).Err() — NOT bare
// s.ctx.Err() — because that is what grpc-go's real ClientStream.RecvMsg
// actually returns when a stream's context ends (see clientStream's
// RecvMsg/newAttemptLocked using status.FromContextError; verified against
// this repo's vendored google.golang.org/grpc). A bare context.Error()
// (context.DeadlineExceeded/context.Canceled) does not implement
// GRPCStatus(), so status.Convert falls through to codes.Unknown instead of
// codes.DeadlineExceeded/codes.Canceled — a fake that returned the bare
// error would silently fail to exercise serveAIAssistantStreamWith's
// codes.DeadlineExceeded branch at all.
type blockingAssistantStream struct {
	grpc.ClientStream
	ctx context.Context
}

func (s *blockingAssistantStream) Recv() (*aipb.StreamEvent, error) {
	<-s.ctx.Done()
	return nil, status.FromContextError(s.ctx.Err()).Err()
}

// TestServeAIAssistantStream_TurnDeadlineMapsToFriendlyMessage is the
// regression guard for the codes.DeadlineExceeded → fixed-message mapping in
// serveAIAssistantStreamWith: when the per-turn deadline fires while Recv()
// is still blocked, the resulting RUN_ERROR frame's `message` must be the
// fixed Traditional Chinese string, never grpc-go's literal
// status.FromContextError text ("context deadline exceeded"). That mapping
// is a guard against a real production incident class (an unreadable raw
// English error string reaching end users) — a future edit that silently
// breaks it must turn this test red, so it uses turnDeadline (the parameter
// serveAIAssistantStreamWith exists for) to inject a 50ms deadline instead
// of waiting out the real 120s streamTurnDeadline.
func TestServeAIAssistantStream_TurnDeadlineMapsToFriendlyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/x", nil)

	const tinyDeadline = 50 * time.Millisecond

	done := make(chan struct{})
	go func() {
		serveAIAssistantStreamWith(c, "", func(ctx context.Context) (grpcclient.AssistantStream, error) {
			return &blockingAssistantStream{ctx: ctx}, nil
		}, tinyDeadline, rawGRPCErrorFrameCode, false)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("serveAIAssistantStreamWith did not return after its turn deadline fired — the stream goroutine hung instead of converging")
	}

	body := w.Body.String()
	if !strings.Contains(body, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected a RUN_ERROR frame after the turn deadline fired, got %q", body)
	}
	if strings.Contains(body, "context deadline exceeded") {
		t.Fatalf("grpc-go's literal DeadlineExceeded error text leaked into the client-facing message, got %q", body)
	}
	const wantMessage = "本次回覆花費時間過長，請重新發送這句話再試一次"
	if !strings.Contains(body, wantMessage) {
		t.Fatalf("expected the fixed Traditional Chinese timeout message %q, got %q", wantMessage, body)
	}
}

// TestWriteStreamEvent_Step_OmitsDurationMsWhenZero pins the omitempty
// semantics of stepEventPayload.DurationMs (`json:"durationMs,omitempty"`,
// services/ai-service/internal/procurement/handler/stream.go) — a
// map[string]any marshal would always emit "durationMs":0, which is a
// different wire shape than what the HTTP path this replaces produces for a
// step with no measured duration yet.
func TestWriteStreamEvent_Step_OmitsDurationMsWhenZero(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{
		TurnId: "turn-1", Seq: 0, Code: "EXTRACT", Status: "RUNNING", Label: "解析中",
	}}}, state, "thread-1", "run-1")
	out := buf.String()
	if strings.Contains(out, "durationMs") {
		t.Fatalf("durationMs should be omitted when zero (omitempty), got %q", out)
	}
}

func TestWriteStreamEvent_Text_EmitsMessageStartOnceThenContent(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "您"}}}, state, "thread-1", "run-1")
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "好"}}}, state, "thread-1", "run-1")

	out := buf.String()
	wantStart := `data: {"type":"TEXT_MESSAGE_START","messageId":"run-1","role":"assistant"}` + "\n\n"
	wantFirst := `data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"run-1","delta":"您"}` + "\n\n"
	wantSecond := `data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"run-1","delta":"好"}` + "\n\n"
	if out != wantStart+wantFirst+wantSecond {
		t.Fatalf("got %q", out)
	}
	if strings.Count(out, "TEXT_MESSAGE_START") != 1 {
		t.Fatalf("TEXT_MESSAGE_START must be emitted exactly once, got: %q", out)
	}
}

func TestWriteStreamEvent_Done_ClosesMessageThenRunFinished(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{textMessageID: "run-1"}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Done{Done: &aipb.DoneEvent{SessionId: "s1"}}}, state, "thread-1", "run-1")

	out := buf.String()
	wantEnd := `data: {"type":"TEXT_MESSAGE_END","messageId":"run-1"}` + "\n\n"
	wantFinished := `data: {"type":"RUN_FINISHED","threadId":"thread-1","runId":"run-1","result":{"sessionId":"s1"}}` + "\n\n"
	if out != wantEnd+wantFinished {
		t.Fatalf("got %q want %q", out, wantEnd+wantFinished)
	}
}

func TestWriteStreamEvent_Done_NoTextMessageEndWhenNoTextWasSent(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{} // textMessageID never set — a stream that only produced a tool_result, no text
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Done{Done: &aipb.DoneEvent{SessionId: ""}}}, state, "thread-1", "run-1")

	out := buf.String()
	if strings.Contains(out, "TEXT_MESSAGE_END") {
		t.Fatalf("must not close a message that was never started, got %q", out)
	}
	if !strings.Contains(out, `"type":"RUN_FINISHED"`) {
		t.Fatalf("expected RUN_FINISHED, got %q", out)
	}
}

func TestWriteStreamEvent_Error_EmitsRunErrorWithStringCode(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Error{Error: &aipb.ErrorEvent{Code: 500, Message: "boom"}}}, state, "thread-1", "run-1")

	const want = `data: {"type":"RUN_ERROR","message":"boom","code":"500"}` + "\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestWriteStreamEvent_Error_ClosesOpenTextMessageBeforeRunError proves the
// StreamEvent_Error case closes an open text message (TEXT_MESSAGE_END)
// before writing RUN_ERROR — the spec's event-mapping table requires
// TEXT_MESSAGE_END on error the same way it's required on done/turn_result,
// and RUN_ERROR is terminal so nothing may follow it.
func TestWriteStreamEvent_Error_ClosesOpenTextMessageBeforeRunError(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{textMessageID: "run-1"}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Error{Error: &aipb.ErrorEvent{Code: 500, Message: "boom"}}}, state, "thread-1", "run-1")

	const want = `data: {"type":"TEXT_MESSAGE_END","messageId":"run-1"}` + "\n\n" +
		`data: {"type":"RUN_ERROR","message":"boom","code":"500"}` + "\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if state.textMessageID != "" {
		t.Fatalf("expected textMessageID cleared after closing it, got %q", state.textMessageID)
	}
	if !state.runEnded {
		t.Fatalf("expected runEnded to be true after RUN_ERROR")
	}
}

// TestWriteStreamEvent_ErrorThenDone_RunErrorIsTerminal mirrors ai-service's
// real mid-run-failure emission (seller_assistant.go RunAssistantStream: on a
// runner error it calls sink.Error(...) then falls through to
// sink.Done(sessionID)) and proves the resulting AG-UI stream is well-formed:
// RUN_ERROR is terminal, so the subsequent DoneEvent's effects (closing the
// text message a second time, RUN_FINISHED) must never be written.
func TestWriteStreamEvent_ErrorThenDone_RunErrorIsTerminal(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}

	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Text{Text: &aipb.TextEvent{Text: "hi"}}}, state, "thread-1", "run-1")
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Error{Error: &aipb.ErrorEvent{Code: 500, Message: "boom"}}}, state, "thread-1", "run-1")
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Done{Done: &aipb.DoneEvent{SessionId: "s1"}}}, state, "thread-1", "run-1")

	out := buf.String()

	if n := strings.Count(out, `"type":"TEXT_MESSAGE_END"`); n != 1 {
		t.Fatalf("expected TEXT_MESSAGE_END exactly once, got %d in %q", n, out)
	}
	if n := strings.Count(out, `"type":"RUN_ERROR"`); n != 1 {
		t.Fatalf("expected RUN_ERROR exactly once, got %d in %q", n, out)
	}
	if strings.Contains(out, "RUN_FINISHED") {
		t.Fatalf("RUN_FINISHED must not appear — the runEnded guard should have stopped the Done event's effects, got %q", out)
	}
	endIdx := strings.Index(out, "TEXT_MESSAGE_END")
	errIdx := strings.Index(out, `"type":"RUN_ERROR"`)
	if endIdx >= errIdx {
		t.Fatalf("expected TEXT_MESSAGE_END before RUN_ERROR, got %q", out)
	}

	const want = `data: {"type":"TEXT_MESSAGE_START","messageId":"run-1","role":"assistant"}` + "\n\n" +
		`data: {"type":"TEXT_MESSAGE_CONTENT","messageId":"run-1","delta":"hi"}` + "\n\n" +
		`data: {"type":"TEXT_MESSAGE_END","messageId":"run-1"}` + "\n\n" +
		`data: {"type":"RUN_ERROR","message":"boom","code":"500"}` + "\n\n"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestWriteStreamEvent_ToolResult_EmitsStartThenEndThenResult(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_ToolResult{ToolResult: &aipb.ToolResultEvent{ResultJson: `{"orders":[]}`}}}, state, "thread-1", "run-1")

	out := buf.String()
	wantStart := `data: {"type":"TOOL_CALL_START","toolCallId":"run-1-tool-1","toolCallName":"tool_result"}` + "\n\n"
	wantEnd := `data: {"type":"TOOL_CALL_END","toolCallId":"run-1-tool-1"}` + "\n\n"
	wantResult := `data: {"type":"TOOL_CALL_RESULT","messageId":"run-1","toolCallId":"run-1-tool-1","content":"{\"orders\":[]}"}` + "\n\n"
	if out != wantStart+wantEnd+wantResult {
		t.Fatalf("got %q want %q", out, wantStart+wantEnd+wantResult)
	}
	// AG-UI's tool-call lifecycle requires TOOL_CALL_END to close the call
	// (opened by TOOL_CALL_START) before TOOL_CALL_RESULT — a conformant
	// client verifier rejects other events arriving while a tool call is
	// still "open".
	startIdx := strings.Index(out, "TOOL_CALL_START")
	endIdx := strings.Index(out, "TOOL_CALL_END")
	resultIdx := strings.Index(out, "TOOL_CALL_RESULT")
	if startIdx >= endIdx || endIdx >= resultIdx {
		t.Fatalf("expected TOOL_CALL_START < TOOL_CALL_END < TOOL_CALL_RESULT, got %q", out)
	}
}

func TestWriteStreamEvent_ToolResult_IncrementsToolCallIdOnSecondCall(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_ToolResult{ToolResult: &aipb.ToolResultEvent{ResultJson: `{}`}}}, state, "thread-1", "run-1")
	buf.Reset()
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_ToolResult{ToolResult: &aipb.ToolResultEvent{ResultJson: `{}`}}}, state, "thread-1", "run-1")

	if !strings.Contains(buf.String(), `"toolCallId":"run-1-tool-2"`) {
		t.Fatalf("second tool_result must get a distinct toolCallId, got %q", buf.String())
	}
}

// 樣本值刻意用**真實 producer 的形狀**：`patch_json` 的內容是 ai-service
// `seller_assistant.go` 把 `update_draft` 的 `Response["patch"]`（型別
// `agent.ProductDraftPatch`）直接 marshal 出來的，是一個扁平欄位物件。
//
// ⚠️ 這裡曾經用 `{"op":"replace"}`。這支測試驗的是「任意合法 JSON 原樣透傳成
// CUSTOM 事件」，樣本值本身不影響覆蓋——但它會被當成「patch_json 是什麼格式」
// 的證據讀，而那個值看起來就是 RFC 6902 JSON Patch。2026-09-08 查證「能不能
// 把這個事件升級成 AG-UI 的 STATE_DELTA（需要 RFC 6902）」時，差一步就據此
// 得出「是」的相反結論；追到 producer 才確認**不是**（既非 RFC 6902 的
// operation 陣列，也非 RFC 7386 的 merge patch）。
func TestWriteStreamEvent_DraftPatch_EmitsCustomEvent(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_DraftPatch{DraftPatch: &aipb.DraftPatchEvent{PatchJson: `{"name":"高麗菜","origin":"台灣"}`}}}, state, "thread-1", "run-1")

	const want = `data: {"type":"CUSTOM","name":"draft_patch","value":{"name":"高麗菜","origin":"台灣"}}` + "\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// TestWriteStreamEvent_DraftPatch_InvalidJSONDoesNotDropEvent pins the
// fallback for a PatchJson that isn't itself valid JSON (including the
// empty-string case): using it directly as json.RawMessage would make the
// containing json.Marshal call fail, and marshalAGUI's "ignore marshal
// error" convention would then make writeAGUIFrame emit an empty `data: \n\n`
// line — silently dropping the entire event with no error signal. The fix
// falls back to encoding PatchJson as a plain JSON string instead.
func TestWriteStreamEvent_DraftPatch_InvalidJSONDoesNotDropEvent(t *testing.T) {
	cases := []struct {
		name      string
		patchJSON string
	}{
		{name: "empty string", patchJSON: ""},
		{name: "non-JSON text", patchJSON: "not json at all"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			state := &agUIStreamState{}
			writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_DraftPatch{DraftPatch: &aipb.DraftPatchEvent{PatchJson: tc.patchJSON}}}, state, "thread-1", "run-1")

			out := buf.String()
			if out == "" {
				t.Fatalf("draft_patch event must not be silently dropped, got empty output")
			}
			if !strings.Contains(out, `"name":"draft_patch"`) {
				t.Fatalf("expected a CUSTOM draft_patch frame, got %q", out)
			}
			var frame struct {
				Type  string `json:"type"`
				Name  string `json:"name"`
				Value string `json:"value"`
			}
			data := strings.TrimSuffix(strings.TrimPrefix(out, "data: "), "\n\n")
			if err := json.Unmarshal([]byte(data), &frame); err != nil {
				t.Fatalf("frame is not valid JSON: %v (%q)", err, out)
			}
			if frame.Value != tc.patchJSON {
				t.Fatalf("expected value to preserve the original payload %q, got %q", tc.patchJSON, frame.Value)
			}
		})
	}
}

// TestWriteStreamEvent_Activity_MapsToCustomActivity 釘住 StreamEvent_Activity
// → CUSTOM name="activity" 的逐位元組形狀。
//
// 為什麼要在這裡直接測 writeStreamEvent，而不是靠 agui_golden_test.go：那支
// golden 測試的 case 表吃的是**已渲染好的 frame struct**（見該檔 frames []any），
// 它驗的是 encoder 的位元組輸出，完全不經過 writeStreamEvent 的 type switch。
// 換句話說，把這個 case 從 switch 整個刪掉，golden 測試仍然全綠——mapping 這一層
// 只有這支測試守著。
//
// data_json 是 proto 的 bytes 欄位。任何 protojson 路徑都會把它 base64 掉；
// 這裡走的是手寫的 rawJSONOrString，所以 value 必須是**內嵌的 JSON object**
// 而不是一串 base64 字串——下面的字面 want 就是在釘這件事。
//
// payload 的欄位形狀（rowId／attempt／maxAttempts／retryGroupKey／code／text，
// 其中 rowId／retryGroupKey 是十進位字串）來自 api/proto/ai/ai.proto 的
// ActivityEvent 註解，並且必須逐字透傳：前端 normalizer 是 fail-closed 的
// （rowId／code 需為非空字串、text 需存在），重新 marshal 或改形狀會讓整列
// 活動被丟掉。
func TestWriteStreamEvent_Activity_MapsToCustomActivity(t *testing.T) {
	var buf bytes.Buffer
	state := &agUIStreamState{}
	writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Activity{
		Activity: &aipb.ActivityEvent{DataJson: []byte(
			`{"rowId":"7489","attempt":1,"maxAttempts":3,"retryGroupKey":"7489","code":"EXTRACT_SLOTS","text":"整理你的採購條件"}`)},
	}}, state, "thread-1", "run-1")

	// want 是整個 buffer 的完整內容，不是子字串比對——所以它同時釘住「沒有
	// `id:` 行」：replay 路徑（writeAGUIFrameWithID）會多寫一行 `id: <seq>`，
	// 那是 ledger cursor，在沒有 replay 語意的 POST 串流上推進瀏覽器的
	// Last-Event-ID 沒有意義。多寫那一行會讓這個等式立刻不成立。
	const want = `data: {"type":"CUSTOM","name":"activity","value":` +
		`{"rowId":"7489","attempt":1,"maxAttempts":3,"retryGroupKey":"7489","code":"EXTRACT_SLOTS","text":"整理你的採購條件"}}` + "\n\n"
	if got := buf.String(); got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
	if state.runEnded {
		t.Fatal("activity 是中途事件，不得把 run 標成已結束")
	}
}

// TestWriteStreamEvent_Activity_MalformedJSON_StillSchemaValid 釘住壞 payload
// 的降級行為。
//
// 事故形狀：前端 stream-client 對一個解不開的 frame 會呼叫 onError 並中止**整條**
// 串流，不是跳過這一列。所以 data_json 不是合法 JSON 時，這裡必須仍然送出一個
// schema-valid 的 CUSTOM frame（@ag-ui/core 的 value 是 z.any()，字串一樣合法），
// 而不是送出空的 `data: \n\n`（json.RawMessage 自身不合法時，marshalAGUI 會依
// 「忽略 marshal error」的慣例靜默吐空字串）或直接 panic。
func TestWriteStreamEvent_Activity_MalformedJSON_StillSchemaValid(t *testing.T) {
	// wantValue 是 `value` 欄位**序列化後的原始位元組**，不是解碼後的 Go 字串。
	// 刻意不宣告成 string 去比對解碼結果：那樣 `"value":""`、`"value":null` 與
	// 「整個 value key 不存在」三種輸出都會解成 ""，empty／nil 兩個 case 會退化
	// 成 `"" != ""` 這種永遠不會失敗的斷言。用 json.RawMessage 逐位元組比對才
	// 分得出這三者。附帶好處：value 若哪天變成 object（例如誤走 protojson 之外
	// 的重新編碼），錯誤會直接指出 value 不符，而不是誤報成「frame 不是合法 JSON」。
	cases := []struct {
		name      string
		dataJSON  []byte
		wantValue string
	}{
		{name: "non-JSON text", dataJSON: []byte("not json"), wantValue: `"not json"`},
		{name: "truncated JSON", dataJSON: []byte(`{"rowId":"7489"`), wantValue: `"{\"rowId\":\"7489\""`},
		{name: "empty", dataJSON: []byte(""), wantValue: `""`},
		{name: "nil", dataJSON: nil, wantValue: `""`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			state := &agUIStreamState{}
			writeStreamEvent(&buf, &aipb.StreamEvent{Payload: &aipb.StreamEvent_Activity{
				Activity: &aipb.ActivityEvent{DataJson: tc.dataJSON},
			}}, state, "thread-1", "run-1")

			out := buf.String()
			if out == "" {
				t.Fatal("壞掉的 activity payload 不得讓整個 frame 消失（空的 data: 行會被前端當成 parse 失敗）")
			}
			data := strings.TrimSuffix(strings.TrimPrefix(out, "data: "), "\n\n")
			var frame struct {
				Type  string          `json:"type"`
				Name  string          `json:"name"`
				Value json.RawMessage `json:"value"`
			}
			if err := json.Unmarshal([]byte(data), &frame); err != nil {
				t.Fatalf("frame 不是合法 JSON：%v（%q）", err, out)
			}
			if frame.Type != "CUSTOM" || frame.Name != "activity" {
				t.Fatalf("降級後仍必須是 CUSTOM name=activity，得到 type=%q name=%q", frame.Type, frame.Name)
			}
			if string(frame.Value) != tc.wantValue {
				t.Fatalf("降級成 JSON 字串時必須原樣保留 payload：value 應為 %s，實得 %s（完整 frame %q）",
					tc.wantValue, frame.Value, out)
			}
		})
	}
}

// deadlineTrackingResponseWriter records every SetWriteDeadline call instead
// of touching a real connection — mirrors the pattern the old
// handler/ai_stream_test.go used for streamWithDeadline before
// deadlineResponseWriter replaced it.
type deadlineTrackingResponseWriter struct {
	http.ResponseWriter
	deadlines []time.Time
}

func (w *deadlineTrackingResponseWriter) SetWriteDeadline(t time.Time) error {
	w.deadlines = append(w.deadlines, t)
	return nil
}

func TestDeadlineResponseWriter_RefreshesDeadlineBeforeEachWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	dw := &deadlineTrackingResponseWriter{ResponseWriter: rec}
	d := &deadlineResponseWriter{
		ResponseWriter: dw,
		rc:             http.NewResponseController(dw),
		timeout:        30 * time.Second,
	}

	if _, err := d.Write([]byte("chunk one ")); err != nil {
		t.Fatalf("unexpected error on first write: %v", err)
	}
	if _, err := d.Write([]byte("chunk two")); err != nil {
		t.Fatalf("unexpected error on second write: %v", err)
	}

	if len(dw.deadlines) != 2 {
		t.Fatalf("expected SetWriteDeadline called once per Write, got %d calls", len(dw.deadlines))
	}
	for i, dl := range dw.deadlines {
		if got := time.Until(dl); got <= 0 || got > 30*time.Second {
			t.Fatalf("deadline #%d not within (0, 30s] of now: got %v ahead", i, got)
		}
	}
	if got := rec.Body.String(); got != "chunk one chunk two" {
		t.Fatalf("expected both writes to reach the underlying writer, got %q", got)
	}
}

func TestDeadlineResponseWriter_Unwrap(t *testing.T) {
	rec := httptest.NewRecorder()
	d := &deadlineResponseWriter{
		ResponseWriter: rec,
		rc:             http.NewResponseController(rec),
		timeout:        time.Second,
	}

	if got := d.Unwrap(); got != http.ResponseWriter(rec) {
		t.Fatalf("Unwrap() = %v, want the original ResponseWriter %v", got, rec)
	}
}
