package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	procurementwire "github.com/Will413028/hawkr/gateway/internal/procurement"
	"github.com/Will413028/hawkr/gateway/internal/response"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// streamWriteTimeout bounds a SINGLE write to the client — independent of
// streamTurnDeadline below (which bounds the UPSTREAM/Recv side): a client
// that keeps the TCP connection open but stops reading would otherwise pin
// this goroutine and its connection forever regardless of how the upstream
// side is bounded. Same 150s value the retired HTTP proxy path used, applied
// through the deadlineResponseWriter wrapper below. Out of scope for the
// gRPC-keepalive/per-turn-deadline rework this file went through — this is a
// distinct problem (client-side write stalls) and is left as-is.
const streamWriteTimeout = 150 * time.Second

// sseKeepaliveInterval is how often serveAIAssistantStreamWith writes a bare
// SSE comment (": ping\n\n") to the client while the Recv loop is otherwise
// silent (an LLM "thinking" gap can legitimately last many seconds with no
// StreamEvent to relay).
//
// ⚠️ Why this exists at all: without it, an idle-but-open SSE connection has
// no origin-side timer forcing it closed — that's the whole point of moving
// liveness to gRPC keepalive (pkg/grpcclient.Dial / pkg/server.BuildServerOptions)
// instead of guessing at the application layer. But Cloudflare (sitting in
// front of this gateway) has its OWN idle timeout, roughly 100s, that we
// don't control and can't see from here. Drop below that and CF's cutoff
// becomes the real, invisible ceiling on every AI stream. A comment line
// every 20s keeps the connection visibly alive to any such intermediary
// without perturbing the AG-UI event stream itself (SSE comments are
// invisible to EventSource/fetch-stream parsers).
//
// ⚠️ Why this has a cost, and why it is only safe together with the per-turn
// deadline (see streamTurnDeadline): SSE is one-way — the server never reads
// from the client — so once nothing here forces the connection closed, a
// browser tab that goes away no longer ends this goroutine on its own.
// gateway/internal/sse/capgroup.go:14-22 documents a real incident from
// exactly this shape: as long as an intermediary keeps accepting the
// heartbeat, ctx.Done() never fires and w.Write never errors, so a
// same-family "keep it alive with heartbeats" fix can turn a closed browser
// tab into a goroutine that idles for however long nothing else bounds it.
// streamTurnDeadline is that bound here: this stream is request-scoped, so
// the deadline is a hard, known ceiling regardless of what the client does.
// This is why the two must land together — this heartbeat without that
// deadline reintroduces the capgroup.go failure mode on every AI stream.
const sseKeepaliveInterval = 20 * time.Second

// streamTurnDeadline bounds one whole AI assistant/procurement turn, end to
// end — a single hard ceiling on the stream's total lifetime. This replaced
// an earlier per-frame idle timer that re-armed around every Recv and
// guessed at "gap between frames" as a proxy for liveness; that guess is
// gone now that gRPC keepalive (pkg/grpcclient.Dial / pkg/server.
// BuildServerOptions) handles actual connection liveness and this deadline
// handles "how long is one turn allowed to run."
//
// This number is a PRODUCT decision (how long a user will tolerate waiting
// for one reply), not a measured p99 — do not treat it as a latency SLO to
// tune from timing data alone. Reference data point: live per-turn timing
// verification on 2026-08-21 measured 16-46s total turn duration for
// successful turns, well under this ceiling.
//
// gRPC propagates this deadline automatically to ai-service — the server's
// LLM call inherits it via ctx, so a turn that would blow past this budget
// gets cut server-side too, not just at the gateway.
const streamTurnDeadline = 120 * time.Second

// ServeAIAssistantStream drives a single AIAssistantService server-streaming
// RPC to completion, translating each received StreamEvent into an SSE
// frame written to c. Since Phase C this is the only transport for the 12
// AIAssistantService SSE endpoints — the httputil.ReverseProxy path it
// replaced is gone.
//
// open is called with a context this function owns (derived from
// c.Request.Context(), so client disconnect cancels the upstream gRPC call —
// see design spec §3.7) — the caller supplies open as a closure over the
// specific RPC to invoke, e.g. func(ctx) { return h.AIAssistant.SellerChat(ctx, req) },
// so this function has no per-RPC knowledge.
//
// If open itself fails (including a circuit-breaker-open rejection), this
// fails fast with a plain HTTP 502 envelope — no automatic retry, because
// these RPCs have side effects (a real LLM call), and a transparent retry
// could double-charge/double-call without the caller realizing (design
// spec §3.8).
func ServeAIAssistantStream(c *gin.Context, threadID string, open func(ctx context.Context) (grpcclient.AssistantStream, error)) {
	serveAIAssistantStreamWith(c, threadID, open, streamTurnDeadline, rawGRPCErrorFrameCode, false)
}

// ServeAIProcurementStream is ServeAIAssistantStream for the one
// AIProcurementService streaming RPC (SubmitTurnStream). Identical wiring, one
// deliberate difference: the terminal AG-UI RUN_ERROR frame's `code` carries an
// HTTP status, not the raw gRPC code.
//
// That difference is not cosmetic. ai-service's retired HTTP handler
// (procurement/handler/stream.go PostTurnStream) emitted `{"code":404,...}` /
// `{"code":409,...}` / `{"code":500,...}` in-band, and the web client branches
// on exactly those numbers (hawkr-website
// features/procurement-assistant/useProcurementSession.ts: err.code === 404 →
// CONVERSATION_GONE, err.code === 409 → IDEMPOTENCY_CONFLICT). Relaying
// st.Code() verbatim would put 5/10/13 on the wire instead, and both branches
// would silently fall through to a generic failure.
//
// The AG-UI migration kept the value but changed its JSON type: agUIRunError.Code
// is a Go string, so the frame now carries `"code":"409"` rather than
// `"code":409`. That is load-bearing on the consumer side — hawkr-website
// lib/ai-chat/stream-client.ts accepts the field only when it matches
// /^-?\d+$/ and drops it otherwise, so a non-numeric code silently becomes
// undefined and the 409 branch degrades to a generic 500. Any future machine
// code (a reason/UPPER_SNAKE identifier) must therefore go in a NEW field, not
// be squeezed into this one.
//
// Assistant deliberately keeps the raw-code behavior — see
// rawGRPCErrorFrameCode.
//
// The final `true` argument is deferRunStarted (see serveAIAssistantStreamWith's
// doc comment): this is the ONE caller that passes true, because this is the
// only AIAssistantService-family RPC whose real ledger run_id (Task 7's
// RunAccepted StreamEvent) isn't known until after Recv() returns the first
// event. The other 12 endpoints (ServeAIAssistantStream) keep passing false —
// trace_id remains their runId, unchanged.
func ServeAIProcurementStream(c *gin.Context, threadID string, open func(ctx context.Context) (grpcclient.AssistantStream, error)) {
	serveAIAssistantStreamWith(c, threadID, open, streamTurnDeadline, procurementErrorFrameCode, true)
}

// errorFrameCode maps a mid-stream gRPC status code to the numeric `code` the
// SSE `error` frame carries. Supplied by the caller so the two stream flavors
// can differ without either one silently changing the other's wire contract.
type errorFrameCode func(codes.Code) int32

// rawGRPCErrorFrameCode preserves the behavior the 12 AIAssistantService SSE
// endpoints have been live with since Phase B: the raw gRPC code number.
//
// It is left as-is deliberately. All three Flutter SSE consumers of those
// endpoints (hawkr_buyer + hawkr_seller ai_assistant_controller.dart,
// hawkr_seller product_listing_chat_controller.dart) read only the frame's
// `message` and never look at `code`, so the number is inert for them; changing
// it would be an unrequested behavior change on a live path, with a bigger
// blast radius than this fix's scope.
func rawGRPCErrorFrameCode(code codes.Code) int32 { return int32(code) }

// procurementErrorFrameCode reuses the same fixed bucket table the 18 unary
// procurement handlers map their errors through (spec §6), so an `error` frame's
// code and the equivalent unary response's code are the same number for the
// same underlying failure.
func procurementErrorFrameCode(code codes.Code) int32 {
	return int32(procurementwire.GRPCCodeToHTTPStatus(code))
}

// serveAIAssistantStreamWith is the internal implementation; exported for
// testing with an injected turn deadline (production always calls
// ServeAIAssistantStream or ServeAIProcurementStream, both of which pass
// streamTurnDeadline — this parameter exists so a test can inject a tiny
// value instead of waiting out the real 120s, the same shape the old
// idleTimeout parameter had before this rework).
//
// deferRunStarted controls when/how the RUN_STARTED frame's runId is decided
// (Task 13, doc57 §6 落差 2). false (the 12 AIAssistantService endpoints, via
// ServeAIAssistantStream): unchanged pre-Task-13 behavior — RUN_STARTED is
// written immediately, before the Recv loop starts, carrying
// c.GetString("trace_id").
//
// true (ServeAIProcurementStream only): RUN_STARTED is NOT written up front.
// The Recv loop instead waits for its first result and inspects it:
//   - If it's a StreamEvent_RunAccepted (Task 7's new oneof case — ai-service
//     always sends this as the guaranteed-first frame, right after the claim
//     transaction commits and before dispatch starts; see
//     procurement_handler.go SubmitTurnStream), runID is replaced with its
//     real ledger run_id, and RUN_STARTED is written using that value. The
//     RunAccepted event itself carries no further payload to relay, so it is
//     consumed here and not passed to writeStreamEvent.
//   - Otherwise — the first event was something other than RunAccepted,
//     which the guarantee above means should not happen, so this signals a
//     protocol break rather than an expected configuration — this is the
//     fallback the plan calls for: log a warn and fall back to trace_id,
//     same as the deferRunStarted=false behavior, so the stream still
//     completes instead of hanging or crashing. The first event is then
//     relayed normally (it's real content, not a transport-only signal like
//     RunAccepted).
//   - If the very first Recv() itself fails (before any event arrived),
//     RUN_STARTED is still written (with the trace_id fallback) before the
//     resulting RUN_ERROR frame, so a client never sees a terminal frame
//     without a preceding RUN_STARTED.
//
// Either way, once decided, runID is reused for every subsequent frame in
// this stream that carries one (writeStreamEvent's runID parameter), exactly
// like the non-deferred path already does.
func serveAIAssistantStreamWith(c *gin.Context, threadID string, open func(ctx context.Context) (grpcclient.AssistantStream, error), turnDeadline time.Duration, errorCode errorFrameCode, deferRunStarted bool) {
	// WithTimeout (not WithCancel): still derives from c.Request.Context(), so
	// client disconnect still cancels it exactly as before, but it now also
	// carries a hard upper bound — see streamTurnDeadline's doc comment.
	ctx, cancel := context.WithTimeout(c.Request.Context(), turnDeadline)
	defer cancel()

	stream, err := open(ctx)
	if err != nil {
		// Nothing has been written to c.Writer yet — no SSE headers, no body —
		// so a normal HTTP error response is still possible. Both
		// Flutter apps branch on `response.statusCode != 200` to surface an
		// "AI service error"; an HTTP 200 carrying an SSE error frame would
		// show them nothing at all. Reuses the package's existing envelope
		// writer so the shape (code/message/traceId) stays identical to every
		// other upstream-unreachable response the gateway emits.
		//
		// FailClientFacing（424，原本是 502）：這條的消費者是兩支 App 與 web 的 AI
		// 助手，沒有第三方送信端。Cloudflare 會把 origin 的 5xx body 整個換成自己的
		// text/plain 錯誤頁 ⇒ 上面那句「Flutter apps branch on statusCode != 200」
		// 在 502 下仍成立，但 envelope 本身（message／traceId）永遠送不到；424 是
		// 4xx，CF 不碰，兩個前提同時滿足。判準與兩類消費者的差異見
		// reverse_proxy.go 的 UpstreamFailureStatus。
		writeUpstreamFailureEnvelope(c.Writer, c.Request, FailClientFacing, err)
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", response.CacheControlPrivateNoStore)
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Per-write deadline for the client side of the stream — see
	// streamWriteTimeout. Frames go through w; flushes still go through
	// c.Writer (deadlineResponseWriter only intercepts Write, and
	// http.ResponseController walks its Unwrap chain anyway).
	w := &deadlineResponseWriter{
		ResponseWriter: c.Writer,
		rc:             http.NewResponseController(c.Writer),
		timeout:        streamWriteTimeout,
	}

	// runID identifies this one stream/run for every AG-UI frame that carries
	// a runId (RUN_STARTED, RUN_FINISHED, tool-call ids, ...). trace_id (set
	// earlier in the gin middleware chain) is the default source — it means
	// the run can be correlated with server-side logs without inventing a
	// second id, and it's ALWAYS what the 12 non-procurement endpoints use
	// (deferRunStarted false below never overrides it). For the procurement
	// path (deferRunStarted true), this is only the starting value: it gets
	// replaced with the real ledger run_id once the Recv loop below sees
	// Task 7's RunAccepted event — see serveAIAssistantStreamWith's doc
	// comment.
	runID := c.GetString("trace_id")

	// runStartedWritten tracks whether RUN_STARTED has gone out yet. When
	// deferRunStarted is false this is immediately true — RUN_STARTED is the
	// first frame of the stream, written before the Recv loop starts, exactly
	// as before Task 13, so the client learns threadId/runId before anything
	// else. When true, writing is postponed to the Recv loop below (see the
	// `case res := <-recvCh` branch), which decides the real runID first.
	runStartedWritten := !deferRunStarted
	if runStartedWritten {
		writeAGUIFrame(w, marshalAGUI(agUIRunStarted{Type: "RUN_STARTED", ThreadID: threadID, RunID: runID}))
		c.Writer.Flush()
	}

	state := &agUIStreamState{}

	// recvCh carries every stream.Recv() outcome from the forwarding
	// goroutine below into the single select loop further down. This — not a
	// mutex — is why the sseKeepaliveInterval ping below cannot race with the
	// Recv-driven writes: w is written from exactly one goroutine (this one,
	// running serveAIAssistantStreamWith). The forwarding goroutine never
	// touches w; it only ever sends on recvCh. By construction there is
	// exactly one writer, so no lock is needed.
	//
	// The send below is a plain, unconditional (blocking) channel send — an
	// earlier version raced an alternative `case <-ctx.Done(): return` into
	// the same select, reasoning that it prevented a leak if nobody was
	// listening any more. That reasoning was wrong and the alternative case
	// introduced a real bug: when ctx's own deadline firing and Recv()
	// returning happen at the same moment, Go's select can pick the
	// ctx.Done() branch instead of the send, silently discarding the one
	// result the loop below was waiting on and hanging it forever. The plain
	// send is both simpler and actually leak-free: recvCh's buffer of 1 means
	// this send only ever has to wait for the loop below to finish whatever
	// it's currently doing (at most one ping write), and that loop's only
	// return path is receiving a non-nil err from this exact channel — so
	// this goroutine's final send (the one with err != nil, right before it
	// returns) is provably always the one thing the loop below is still
	// listening for. There is no case where this goroutine sends into a void.
	recvCh := make(chan streamRecvResult, 1)
	go func() {
		for {
			event, err := stream.Recv()
			recvCh <- streamRecvResult{event: event, err: err}
			if err != nil {
				return
			}
		}
	}()

	pingTicker := time.NewTicker(sseKeepaliveInterval)
	defer pingTicker.Stop()

	for {
		select {
		case res := <-recvCh:
			if res.err != nil {
				if !runStartedWritten {
					// deferRunStarted was true and the very first Recv() call
					// failed before any event (RunAccepted or otherwise) ever
					// arrived. Still write RUN_STARTED (trace_id fallback) so
					// the terminal RUN_ERROR frame below is never the very
					// first thing on the wire — AG-UI's lifecycle requires
					// RUN_STARTED to precede any other run-scoped event. No
					// need to set runStartedWritten here: this branch always
					// returns below, so nothing downstream would read it again.
					writeAGUIFrame(w, marshalAGUI(agUIRunStarted{Type: "RUN_STARTED", ThreadID: threadID, RunID: runID}))
					c.Writer.Flush()
				}
				// The stream already started (SSE headers are out), so the
				// HTTP status is committed — a mid-stream failure can only be
				// reported in-band, as an SSE error frame.
				if !errors.Is(res.err, io.EOF) {
					st := status.Convert(res.err)
					message := st.Message()
					// procurement 的錯誤帶 BusinessError detail（reason／
					// message／remaining／reset_at）。同一個 helper 也是 unary
					// 那條路徑（handler/procurement_errors.go 的
					// writeProcurementError）唯一的取出點，所以同一個失敗在
					// SSE 與 JSON 兩種 transport 上帶的是同一組欄位——doc 57
					// §4.7 的 AGENT_MONTHLY_QUOTA_EXCEEDED 要求 remaining=0 ＋
					// resetAt 上 wire，只有 unary 帶到不算數（買家的
					// procurement turn 走的正是這條 SSE）。
					// 12 支 AIAssistantService streaming 端點的 status 沒有這
					// 個 detail，這裡拿到 nil，frame 形狀一個位元組都不變。
					bizErr := procurementwire.BusinessErrorFromStatus(st)
					if bizErr.GetMessage() != "" {
						message = bizErr.GetMessage()
					}
					if st.Code() == codes.DeadlineExceeded {
						// streamTurnDeadline expiring reaches here as ctx's
						// DeadlineExceeded, wrapped by grpc-go's
						// status.FromContextError — st.Message() is then just
						// err.Error(), the literal English string "context
						// deadline exceeded". That must never reach the user
						// verbatim (same rule ServeAIProcurementStream's
						// GRPCCodeToHTTPStatus mapping already enforces for
						// every other code): swap in a real message. A plain
						// client disconnect still reaches this branch as
						// codes.Canceled/"context canceled" instead — left
						// unmapped, because by then the connection is
						// actually gone and nothing downstream ever reads the
						// message. A deadline is different: the turn is
						// alive and simply timed out, so this message is the
						// first time that fact is user-visible.
						message = "本次回覆花費時間過長，請重新發送這句話再試一次"
					}
					writeErrorFrame(w, state, errorCode(st.Code()), message, bizErr)
					c.Writer.Flush()
				}
				return
			}
			if !runStartedWritten {
				ra, isRunAccepted := res.event.GetPayload().(*aipb.StreamEvent_RunAccepted)
				if isRunAccepted {
					// The guaranteed-first event on the claim+dispatch-routed
					// path (procurement_handler.go SubmitTurnStream): supplies
					// the real ledger run_id. It carries no further payload of
					// its own to relay, so it's consumed here — never passed
					// to writeStreamEvent — and the loop moves on to the next
					// Recv() result (the `continue` below).
					runID = ra.RunAccepted.GetRunId()
				} else {
					// Fallback (Step 3(c) of the plan): ai-service always
					// sends RunAccepted as the guaranteed-first frame (see
					// procurement_handler.go SubmitTurnStream), so reaching
					// this branch means the first event was something else —
					// a protocol break, not an expected configuration. This
					// must not hang or crash the stream: fall back to
					// trace_id, exactly like the non-deferred
					// (deferRunStarted=false) behavior, and still relay this
					// real event below instead of dropping it.
					zap.L().Warn("ai_assistant_stream: procurement stream's first event was not RunAccepted — falling back to trace_id as runId",
						zap.String("thread_id", threadID),
						zap.String("payload_type", fmt.Sprintf("%T", res.event.GetPayload())))
				}
				writeAGUIFrame(w, marshalAGUI(agUIRunStarted{Type: "RUN_STARTED", ThreadID: threadID, RunID: runID}))
				c.Writer.Flush()
				runStartedWritten = true
				if isRunAccepted {
					continue
				}
			}
			writeStreamEvent(w, res.event, state, threadID, runID)
			c.Writer.Flush()
		case <-pingTicker.C:
			// A leading ':' makes this an SSE comment line — invisible to
			// EventSource/fetch-stream parsers, present only to keep
			// intermediaries from treating the connection as idle. See
			// sseKeepaliveInterval's doc comment for why this exists and why
			// it depends on streamTurnDeadline.
			_, _ = io.WriteString(w, ": ping\n\n")
			c.Writer.Flush()
		}
	}
}

// streamRecvResult carries one stream.Recv() outcome from the forwarding
// goroutine in serveAIAssistantStreamWith into its select loop.
type streamRecvResult struct {
	event *aipb.StreamEvent
	err   error
}

// agUIStreamState carries the per-stream state needed to correctly wrap a
// gRPC StreamEvent sequence as AG-UI events. The source protobuf payloads
// don't self-describe "is this the first text chunk" or "how many tool
// calls has this stream had" — a single event, taken alone, can't decide
// whether to emit TEXT_MESSAGE_START. One instance is created per stream by
// serveAIAssistantStreamWith and threaded through every writeStreamEvent
// call for that stream (Task 4).
type agUIStreamState struct {
	// textMessageID is empty until the first StreamEvent_Text of the current
	// message has been seen; non-empty afterwards, holding the messageId
	// TEXT_MESSAGE_START used. procurement's one-shot MessageEvent (Task 3)
	// sets and clears this within the same call since it has no delta phase.
	textMessageID string
	// toolCallCounter synthesizes distinct toolCallId values — ToolResultEvent
	// carries no id of its own, and AG-UI's tool-call lifecycle needs one.
	toolCallCounter int
	// runEnded is set once a terminal AG-UI event (RUN_ERROR or RUN_FINISHED)
	// has been written for this stream. Both are terminal per the AG-UI spec —
	// nothing may follow either one. Once true, writeStreamEvent must not write
	// any further AG-UI events for this stream, even if the upstream gRPC
	// stream keeps delivering events (e.g. ai-service's real mid-run-failure
	// emission is ErrorEvent followed by DoneEvent — see writeStreamEvent's
	// StreamEvent_Error case).
	runEnded bool
}

// isStepFinished 判斷一個步驟是否已經跑完，決定要送 STEP_FINISHED 還是 STEP_STARTED。
//
// 判偽的代價是靜默的：那個步驟會被譯成 STEP_STARTED，前端跑條永遠不收尾，而且沒有任何
// 錯誤訊號。改名過渡期（#2446）曾在這裡同時接受舊值 "DONE"；ai-service 已全面部署改名版本
// 且 DB 舊值已收斂，故本波移除。值域 SSOT 是 CodeMeta 的 ProcurementStepStatus。
func isStepFinished(status string) bool {
	return status == codemeta.ProcurementStepStatusSucceeded.Value
}

func writeStreamEvent(w io.Writer, event *aipb.StreamEvent, state *agUIStreamState, threadID, runID string) {
	if state.runEnded {
		return
	}
	switch payload := event.GetPayload().(type) {
	case *aipb.StreamEvent_Text:
		if state.textMessageID == "" {
			state.textMessageID = runID
			writeAGUIFrame(w, marshalAGUI(agUITextMessageStart{
				Type: "TEXT_MESSAGE_START", MessageID: state.textMessageID, Role: "assistant",
			}))
		}
		writeAGUIFrame(w, marshalAGUI(agUITextMessageContent{
			Type: "TEXT_MESSAGE_CONTENT", MessageID: state.textMessageID, Delta: payload.Text.GetText(),
		}))
	case *aipb.StreamEvent_Done:
		if state.textMessageID != "" {
			writeAGUIFrame(w, marshalAGUI(agUITextMessageEnd{Type: "TEXT_MESSAGE_END", MessageID: state.textMessageID}))
			state.textMessageID = ""
		}
		writeAGUIFrame(w, marshalAGUI(agUIRunFinished{
			Type: "RUN_FINISHED", ThreadID: threadID, RunID: runID,
			Result: map[string]string{"sessionId": payload.Done.GetSessionId()},
		}))
		state.runEnded = true
	case *aipb.StreamEvent_Error:
		zap.L().Warn("ai_assistant_stream: ai-service returned ErrorEvent",
			zap.Int32("code", payload.Error.GetCode()),
			zap.String("message", payload.Error.GetMessage()))
		if state.textMessageID != "" {
			writeAGUIFrame(w, marshalAGUI(agUITextMessageEnd{Type: "TEXT_MESSAGE_END", MessageID: state.textMessageID}))
			state.textMessageID = ""
		}
		writeAGUIFrame(w, marshalAGUI(agUIRunError{
			Type: "RUN_ERROR", Message: payload.Error.GetMessage(),
			Code: strconv.FormatInt(int64(payload.Error.GetCode()), 10),
		}))
		state.runEnded = true
	case *aipb.StreamEvent_DraftPatch:
		patchJSON := payload.DraftPatch.GetPatchJson()
		var value any = json.RawMessage(patchJSON)
		if !json.Valid([]byte(patchJSON)) {
			// Empty string or non-JSON text: json.RawMessage must itself be
			// valid JSON to embed, otherwise marshalAGUI's containing
			// json.Marshal call fails and (per its documented "ignore marshal
			// error" convention) silently returns "" — writeAGUIFrame would
			// then emit an empty `data: \n\n` line, dropping this event
			// entirely with no error signal. Falling back to the plain string
			// preserves the payload (as a JSON string, not nested JSON).
			value = patchJSON
		}
		writeAGUIFrame(w, marshalAGUI(agUICustom{
			Type: "CUSTOM", Name: "draft_patch", Value: value,
		}))
	case *aipb.StreamEvent_ToolResult:
		state.toolCallCounter++
		toolCallID := fmt.Sprintf("%s-tool-%d", runID, state.toolCallCounter)
		writeAGUIFrame(w, marshalAGUI(agUIToolCallStart{
			Type: "TOOL_CALL_START", ToolCallID: toolCallID, ToolCallName: "tool_result",
		}))
		writeAGUIFrame(w, marshalAGUI(agUIToolCallEnd{
			Type: "TOOL_CALL_END", ToolCallID: toolCallID,
		}))
		writeAGUIFrame(w, marshalAGUI(agUIToolCallResult{
			Type: "TOOL_CALL_RESULT", MessageID: runID, ToolCallID: toolCallID,
			Content: payload.ToolResult.GetResultJson(),
		}))
	case *aipb.StreamEvent_Step:
		step := payload.Step
		if isStepFinished(step.GetStatus()) {
			writeAGUIFrame(w, marshalAGUI(agUIStepFinished{Type: "STEP_FINISHED", StepName: step.GetCode()}))
		} else {
			writeAGUIFrame(w, marshalAGUI(agUIStepStarted{Type: "STEP_STARTED", StepName: step.GetCode()}))
		}
		// AG-UI 的 STEP_STARTED/STEP_FINISHED 只帶 stepName，容不下步驟跑條
		// UI 需要顯示的 label/durationMs/turnId/seq——用 CUSTOM 事件補帶。
		writeAGUIFrame(w, marshalAGUI(agUICustom{Type: "CUSTOM", Name: "step_detail", Value: stepDetailPayload{
			TurnID: step.GetTurnId(), Seq: step.GetSeq(), Code: step.GetCode(), Status: step.GetStatus(),
			Label: step.GetLabel(), DurationMs: step.GetDurationMs(), SafeArgs: step.GetSafeArgs(),
		}}))
	case *aipb.StreamEvent_Activity:
		// 與 GET replay 路徑（ai_run_events_stream.go 的 writeRunEvent 對
		// AGENT_ACTIVITY_SNAPSHOT／_DELTA 走的 customSlugForRunEventType 分支）
		// 逐字相同的 frame——doc57 §2.1：POST 串流與 replay 送同一種 frame，
		// 前端因此只需要一套 activity 消費邏輯。差別只有一處：POST 路徑不寫
		// `id:` 行（那是 replay 專屬的 ledger seq cursor，見 writeAGUIFrameWithID
		// 的註解；在沒有 replay 語意的 POST 串流上推進 Last-Event-ID 沒有意義）。
		//
		// data_json 逐字透傳、不重新 marshal：前端 normalizer 是 fail-closed 的，
		// 改形狀等於讓那一列活動被丟掉。rawJSONOrString 在 payload 非合法 JSON
		// （含空值）時降級成字串，輸出仍是 schema-valid 的 CUSTOM（@ag-ui/core
		// 的 value 是 z.any()）——這很重要：前端 stream client 遇到一個解不開的
		// frame 會 onError 中止整條串流，不是只跳過這一列。
		writeAGUIFrame(w, marshalAGUI(agUICustom{
			Type: "CUSTOM", Name: "activity", Value: rawJSONOrString(payload.Activity.GetDataJson()),
		}))
	case *aipb.StreamEvent_Message:
		msg := payload.Message
		messageID := msg.GetTurnId()
		writeAGUIFrame(w, marshalAGUI(agUITextMessageStart{Type: "TEXT_MESSAGE_START", MessageID: messageID, Role: "assistant"}))
		writeAGUIFrame(w, marshalAGUI(agUITextMessageContent{Type: "TEXT_MESSAGE_CONTENT", MessageID: messageID, Delta: msg.GetReply()}))
		writeAGUIFrame(w, marshalAGUI(agUITextMessageEnd{Type: "TEXT_MESSAGE_END", MessageID: messageID}))
		// MessageEvent.reasoning 依 #2447 BE-01A 下架：上游停 populate 後，proto
		// 欄位號與名字也已 reserved 化，這裡沒有欄位可轉發。輸出面仍由
		// done_frame_test 的 DropsReasoning sentinel 守著。
	case *aipb.StreamEvent_TurnResult:
		result := procurementwire.TurnResultFromProto(payload.TurnResult)
		writeAGUIFrame(w, marshalAGUI(agUIStateSnapshot{Type: "STATE_SNAPSHOT", Snapshot: result}))
		// outcome 讀自 TurnResult 本身的 additive 欄位（Task 7 選的方案 (a)：
		// interrupt_id=22／interrupt_kind=23 直接掛在 TurnResult 上，unary 的
		// SubmitTurn 與這裡的串流終端事件因此交出同一份形狀）。interrupt_id 非空
		// 才是 WAITING_FOR_INPUT——空字串（含 Task 7 review 留下的已知落差：
		// replayExistingRun 對 WAITING_FOR_INPUT 的冪等重播目前不回填這兩個欄位）
		// 一律照現況渲染成 success，不在這裡臆測或回填 interrupt 狀態。
		outcome := agUIRunOutcome{Type: "success"}
		if interruptID := payload.TurnResult.GetInterruptId(); interruptID != "" {
			outcome = agUIRunOutcome{
				Type:       "interrupt",
				Interrupts: []agUIInterruptEntry{{ID: interruptID, Reason: payload.TurnResult.GetInterruptKind()}},
			}
		}
		writeAGUIFrame(w, marshalAGUI(agUIRunFinished{
			Type: "RUN_FINISHED", ThreadID: threadID, RunID: runID, Result: result, Outcome: &outcome,
		}))
		state.runEnded = true
	default:
		// StreamEvent 的 oneof 再新增 payload 時，這裡是唯一會出聲的地方。
		// （現況：oneof 有 10 個欄位，本 switch 明確處理其中 9 個；剩下的
		// run_accepted 是傳輸層信號，在 serveAIAssistantStreamWith 的 Recv
		// loop 就被消費掉，不會被交給這個函式。）
		// 沒有 default 的話新 payload 會被靜默丟棄：Recv 正常、連線正常、client
		// 只是永遠等不到那個事件——這種形狀的失敗在 dev 幾乎不可能被發現。
		// 刻意不寫任何 frame：翻譯層不知道該把未知 payload 譯成什麼 AG-UI 事件，
		// 硬造一個 CUSTOM 只會讓消費端收到形狀不明的資料。
		zap.L().Warn("ai_assistant_stream: unhandled StreamEvent payload type — 上游新增了 oneof 分支但翻譯層未跟上",
			zap.String("payload_type", fmt.Sprintf("%T", payload)),
			zap.String("thread_id", threadID),
			zap.String("run_id", runID))
	}
}

// stepDetailPayload mirrors services/ai-service/internal/procurement/handler/
// stream.go's stepEventPayload struct field-for-field (including the
// omitempty tag) — that package lives in ai-service's own module and cannot
// be imported here. Named differently (not stepEventPayload) to avoid
// colliding with that retired struct's name now that this file's own
// StreamEvent_Step case is implemented. A map[string]any marshal cannot
// reproduce omitempty (it would always emit "durationMs":0 instead of
// omitting the key), so this is a real struct, not a generic map — same
// reasoning as procurement_response_mapping.go's DTOs.
type stepDetailPayload struct {
	TurnID     string            `json:"turnId"`
	Seq        int32             `json:"seq"`
	Code       string            `json:"code"`
	Status     string            `json:"status"`
	Label      string            `json:"label"`
	DurationMs int64             `json:"durationMs,omitempty"`
	SafeArgs   map[string]string `json:"safeArgs,omitempty"`
}

// writeErrorFrame writes a terminal RUN_ERROR frame for the transport-level
// (Recv-failure) error path — as opposed to writeStreamEvent's
// StreamEvent_Error case, which handles the in-band error the upstream
// gRPC stream itself can send. Closes an open text message first (same
// requirement as the in-band case: RUN_ERROR is terminal, nothing may
// follow it, and a dangling TEXT_MESSAGE_START would make that violation
// worse) and marks state as ended for consistency, even though this is
// already the terminal exit point of its one caller.
//
// bizErr 是這個失敗附帶的 BusinessError detail，可以是 nil（傳輸層失敗與 12 支
// AIAssistantService streaming 端點都沒有）。非 nil 時它的 reason／remaining／
// reset_at 一起上 frame——protoc-gen-go 的 GetXxx() 對 nil receiver 回零值，所以
// 這裡不必再做一次 nil 檢查，空字串會被 agUIRunError 的 omitempty 吃掉。
func writeErrorFrame(w io.Writer, state *agUIStreamState, code int32, message string, bizErr *aipb.BusinessError) {
	if state.textMessageID != "" {
		writeAGUIFrame(w, marshalAGUI(agUITextMessageEnd{Type: "TEXT_MESSAGE_END", MessageID: state.textMessageID}))
		state.textMessageID = ""
	}
	writeAGUIFrame(w, marshalAGUI(agUIRunError{
		Type: "RUN_ERROR", Message: message, Code: strconv.FormatInt(int64(code), 10),
		Reason: bizErr.GetReason(), Remaining: bizErr.GetRemaining(), ResetAt: bizErr.GetResetAt(),
	}))
	state.runEnded = true
}

// deadlineResponseWriter refreshes a write deadline before delegating each
// Write, so a stalled-but-connected client (TCP window full, Write()
// blocking forever) can't hold the connection/goroutine open indefinitely.
// Unwrap (not a direct Flusher impl) is enough on Go 1.21+: both
// http.ResponseController.Flush and .SetWriteDeadline walk the Unwrap chain
// to reach whatever concrete Flusher/deadline-setter is underneath — which
// might itself be another wrapper (gin's own ResponseWriter, or a
// middleware's wrapper), not necessarily the raw net/http one.
type deadlineResponseWriter struct {
	http.ResponseWriter
	rc      *http.ResponseController
	timeout time.Duration
}

func (d *deadlineResponseWriter) Write(p []byte) (int, error) {
	_ = d.rc.SetWriteDeadline(time.Now().Add(d.timeout))
	return d.ResponseWriter.Write(p)
}

func (d *deadlineResponseWriter) Unwrap() http.ResponseWriter { return d.ResponseWriter }
