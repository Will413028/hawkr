package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeProcurementTurnStream implements grpc.ServerStreamingClient[aipb.StreamEvent]
// (Recv() (*aipb.StreamEvent, error) + grpc.ClientStream) — the exact return type
// of AIProcurementServiceClient.SubmitTurnStream — mirroring
// gateway/internal/proxy/ai_assistant_stream_test.go's fakeAssistantStream, which
// cannot be reused directly here since it is unexported in a different package.
type fakeProcurementTurnStream struct {
	grpc.ClientStream
	events []*aipb.StreamEvent
	err    error
	idx    int
}

func (s *fakeProcurementTurnStream) Recv() (*aipb.StreamEvent, error) {
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

// mockProcurementStreamServiceClient is a narrow mock scoped to just
// SubmitTurnStream, kept separate from mockAIProcurementServiceClient
// (ai_procurement_unary_test.go) rather than adding stream fields to that
// unary-only mock — this task's own git rules only allow adding a dedicated
// stream test file, not touching the existing unary test file.
type mockProcurementStreamServiceClient struct {
	aipb.AIProcurementServiceClient

	resp *fakeProcurementTurnStream
	err  error

	gotReq *aipb.TurnRequest
	gotCtx context.Context
}

func (m *mockProcurementStreamServiceClient) SubmitTurnStream(ctx context.Context, in *aipb.TurnRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[aipb.StreamEvent], error) {
	m.gotReq = in
	m.gotCtx = ctx
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

// setupProcurementStreamRouter mounts only the one handler this task cuts
// over, under its @Router annotation's exact path (ai.go
// ProxyProcurementTurnStream).
func setupProcurementStreamRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	app.POST("/buyers/me/procurement/turns/stream", h.ProxyProcurementTurnStream)
	return r
}

func newProcurementStreamTestHolder(mock *mockProcurementStreamServiceClient) *Holder {
	return &Holder{
		AIProcurement: grpcclient.NewAIProcurementClientForTest(mock),
		BuyerID:       buyerResolverForTest(8001),
	}
}

func TestProxyProcurementTurnStream_Success_WritesAGUIFrames(t *testing.T) {
	mock := &mockProcurementStreamServiceClient{
		resp: &fakeProcurementTurnStream{events: []*aipb.StreamEvent{
			{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{
				TurnId: "c-1", Seq: 0, Code: "EXTRACT", Status: "RUNNING", Label: "解析中",
			}}},
			{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{
				TurnId: "c-1", Seq: 1, Code: "EXTRACT", Status: "SUCCEEDED", Label: "解析中", DurationMs: 340,
			}}},
			{Payload: &aipb.StreamEvent_Message{Message: &aipb.MessageEvent{
				TurnId: "c-1", Reply: "已為您找到 3 個候選",
			}}},
			{Payload: &aipb.StreamEvent_TurnResult{TurnResult: &aipb.TurnResult{
				RequestId: "9001", ConversationId: "9002", RevisionId: "9003",
				Reply: "已為您找到 3 個候選", MissingRequired: []string{"unit"},
			}}},
		}},
	}
	h := newProcurementStreamTestHolder(mock)

	body := `{"clientRequestId":"c-1","message":"我要買50箱寶島梨"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementStreamRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	// SSE header 由 proxy 自己寫（不是 router 的 PrivateNoStore 中介層——這支測試的
	// setupProcurementStreamRouter 根本沒掛它）。gin 的 c.Header 是 Set 語意，handler
	// 後寫的值會覆蓋中介層，所以那行字面值必須自己是 private, no-store（#2447 BE-01A）。
	if cc := w.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", cc)
	}

	got := w.Body.String()
	if strings.Contains(got, "event:") {
		t.Fatalf("AG-UI frames must not have an event: line, got %q", got)
	}
	wantInOrder := []string{
		`"type":"RUN_STARTED"`,
		`"type":"STEP_STARTED","stepName":"EXTRACT"`,
		`"type":"CUSTOM","name":"step_detail","value":{"turnId":"c-1","seq":0,"code":"EXTRACT","status":"RUNNING","label":"解析中"}`,
		`"type":"STEP_FINISHED","stepName":"EXTRACT"`,
		`"type":"CUSTOM","name":"step_detail","value":{"turnId":"c-1","seq":1,"code":"EXTRACT","status":"SUCCEEDED","label":"解析中","durationMs":340}`,
		`"type":"TEXT_MESSAGE_START","messageId":"c-1","role":"assistant"`,
		`"type":"TEXT_MESSAGE_CONTENT","messageId":"c-1","delta":"已為您找到 3 個候選"`,
		`"type":"TEXT_MESSAGE_END","messageId":"c-1"`,
		`"type":"STATE_SNAPSHOT","snapshot":{"requestId":"9001"`,
		`"type":"RUN_FINISHED"`,
	}
	lastIdx := -1
	for _, want := range wantInOrder {
		idx := strings.Index(got, want)
		if idx == -1 {
			t.Fatalf("missing frame containing %q in %q", want, got)
		}
		if idx <= lastIdx {
			t.Fatalf("frame %q out of order (expected after index %d, found at %d) in %q", want, lastIdx, idx, got)
		}
		lastIdx = idx
	}
	if !strings.Contains(got, `"missingRequired":["unit"]`) {
		t.Fatalf("STATE_SNAPSHOT/RUN_FINISHED should carry the TurnResult fields, got %q", got)
	}

	if mock.gotReq.GetClientRequestId() != "c-1" || mock.gotReq.GetMessage() != "我要買50箱寶島梨" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotReq)
	}
	if got := mock.gotCtx.Value(grpcclient.CtxKeyBuyerID); got != identity.BuyerID(8001) {
		t.Fatalf("buyerID not injected into RPC context: got %v, want 8001", got)
	}
}

func TestProxyProcurementTurnStream_MissingClientRequestID_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockProcurementStreamServiceClient{}
	h := newProcurementStreamTestHolder(mock)

	body := `{"message":"hi"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementStreamRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotReq != nil {
		t.Fatalf("SubmitTurnStream should not have been called: %+v", mock.gotReq)
	}
}

func TestProxyProcurementTurnStream_MissingMessage_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockProcurementStreamServiceClient{}
	h := newProcurementStreamTestHolder(mock)

	body := `{"clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementStreamRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotReq != nil {
		t.Fatalf("SubmitTurnStream should not have been called: %+v", mock.gotReq)
	}
}

// TestProxyProcurementTurnStream_MessageTooLong_ReturnsUnprocessableEntityWithoutCallingRPC
// mirrors ProxyProcurementTurn's own regression guard (ai_procurement_unary_test.go
// TestProxyProcurementTurn_MessageTooLong_ReturnsUnprocessableEntity): ai-service's
// retired HTTP handler (procurement/handler/stream.go PostTurnStream) rejected an
// over-length message with a plain 422 BEFORE ever switching the connection into
// SSE mode. The new SubmitTurnStream RPC enforces the same limit server-side, but
// only after the stream is already open — surfacing as a mid-stream SSE error frame,
// not a clean HTTP 422 — so Gateway must keep this pre-flight check to avoid a wire
// behavior regression on this path.
func TestProxyProcurementTurnStream_MessageTooLong_ReturnsUnprocessableEntityWithoutCallingRPC(t *testing.T) {
	mock := &mockProcurementStreamServiceClient{}
	h := newProcurementStreamTestHolder(mock)

	longMessage := strings.Repeat("字", 2001)
	body := `{"clientRequestId":"c-1","message":"` + longMessage + `"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementStreamRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnprocessableEntity)
	if mock.gotReq != nil {
		t.Fatalf("SubmitTurnStream should not have been called for an over-length message: %+v", mock.gotReq)
	}
}

// TestProxyProcurementTurnStream_MidStreamError_WritesSSEErrorFrame proves the
// cut-over handler goes through ServeAIProcurementStream's error-mapping path,
// and that the frame's numeric code is an HTTP status (409 for Aborted), not
// the raw gRPC code (10) — hawkr-website's useProcurementSession.ts
// branches on err.code === 409 for IDEMPOTENCY_CONFLICT and on 404 for
// CONVERSATION_GONE, exactly the numbers ai-service's retired HTTP handler
// wrote into this frame.
func TestProxyProcurementTurnStream_MidStreamError_WritesSSEErrorFrame(t *testing.T) {
	mock := &mockProcurementStreamServiceClient{
		resp: &fakeProcurementTurnStream{
			events: []*aipb.StreamEvent{{Payload: &aipb.StreamEvent_Step{Step: &aipb.StepEvent{
				TurnId: "c-1", Code: "EXTRACT", Status: "RUNNING",
			}}}},
			err: status.Error(codes.Aborted, "clientRequestId 已用於其他需求，請改用新的 clientRequestId"),
		},
	}
	h := newProcurementStreamTestHolder(mock)

	body := `{"clientRequestId":"c-1","message":"hi"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementStreamRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	got := w.Body.String()
	if !strings.Contains(got, `"type":"RUN_ERROR"`) {
		t.Fatalf("expected a RUN_ERROR frame after the upstream stream's Recv() returned a non-EOF error, got %q", got)
	}
	if !strings.Contains(got, `"code":"409"`) {
		t.Fatalf("error frame must carry HTTP 409 (as a string) for codes.Aborted, got %q", got)
	}
}
