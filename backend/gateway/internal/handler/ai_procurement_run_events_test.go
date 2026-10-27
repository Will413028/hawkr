package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	"github.com/Will413028/hawkr/go-pkg/identity"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

// fakeRunEventsWatchStream implements grpc.ServerStreamingClient[aipb.RunEvent]
// (Recv() (*aipb.RunEvent, error) + grpc.ClientStream) — the exact return type
// of AIProcurementServiceClient.WatchRunEvents — mirroring
// ai_procurement_stream_test.go's fakeProcurementTurnStream for the sibling
// SubmitTurnStream RPC.
type fakeRunEventsWatchStream struct {
	grpc.ClientStream
	events []*aipb.RunEvent
	idx    int
}

func (s *fakeRunEventsWatchStream) Recv() (*aipb.RunEvent, error) {
	if s.idx < len(s.events) {
		e := s.events[s.idx]
		s.idx++
		return e, nil
	}
	return nil, io.EOF
}

// mockRunEventsServiceClient is a narrow mock scoped to just WatchRunEvents,
// same reasoning as mockProcurementStreamServiceClient (a dedicated stream
// mock kept separate from the unary-only mockAIProcurementServiceClient in
// ai_procurement_unary_test.go).
type mockRunEventsServiceClient struct {
	aipb.AIProcurementServiceClient

	resp *fakeRunEventsWatchStream
	err  error

	gotReq *aipb.WatchRunEventsRequest
	gotCtx context.Context
}

func (m *mockRunEventsServiceClient) WatchRunEvents(ctx context.Context, in *aipb.WatchRunEventsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[aipb.RunEvent], error) {
	m.gotReq = in
	m.gotCtx = ctx
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

// setupRunEventsRouter mounts only ProxyProcurementRunEvents under its exact
// @Router annotation path.
func setupRunEventsRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	app.GET("/buyers/me/procurement/runs/:runId/events", h.ProxyProcurementRunEvents)
	return r
}

func newRunEventsTestHolder(mock *mockRunEventsServiceClient, flagEnabled bool) *Holder {
	return &Holder{
		AIProcurement:               grpcclient.NewAIProcurementClientForTest(mock),
		BuyerID:                     buyerResolverForTest(8001),
		ProcurementReplaySseEnabled: flagEnabled,
	}
}

func TestProxyProcurementRunEvents_CapabilityFlagOff_Returns503WithoutCallingRPC(t *testing.T) {
	mock := &mockRunEventsServiceClient{}
	h := newRunEventsTestHolder(mock, false)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/runs/5150/events", nil)
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupRunEventsRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusServiceUnavailable)
	if mock.gotReq != nil {
		t.Fatalf("WatchRunEvents must not be called while the capability flag is off: %+v", mock.gotReq)
	}
	body := parseJSON(t, w.Body)
	if body["reason"] != "CAPABILITY_UNAVAILABLE" {
		t.Fatalf("expected reason=CAPABILITY_UNAVAILABLE, got %v", body)
	}
}

func TestProxyProcurementRunEvents_Success_ForwardsRunIdBuyerIdAndLastEventID(t *testing.T) {
	mock := &mockRunEventsServiceClient{
		resp: &fakeRunEventsWatchStream{events: []*aipb.RunEvent{
			{RunId: "5150", Seq: "3", Type: codemeta.ProcurementLedgerEventTypeStepStarted.Value, DataJson: []byte(`{"code":"EXTRACT"}`)},
		}},
	}
	h := newRunEventsTestHolder(mock, true)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/runs/5150/events", nil)
	req.Header.Set("X-Account-Id", "99")
	req.Header.Set("Last-Event-ID", "2")

	w := httptest.NewRecorder()
	setupRunEventsRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", cc)
	}

	if mock.gotReq == nil {
		t.Fatal("expected WatchRunEvents to be called")
	}
	if mock.gotReq.GetRunId() != "5150" {
		t.Fatalf("RunId = %q, want 5150 (from :runId path param)", mock.gotReq.GetRunId())
	}
	if mock.gotReq.GetAfterSeq() != "2" {
		t.Fatalf("AfterSeq = %q, want 2 (from Last-Event-ID header)", mock.gotReq.GetAfterSeq())
	}
	if got := mock.gotCtx.Value(grpcclient.CtxKeyBuyerID); got != identity.BuyerID(8001) {
		t.Fatalf("buyerID not injected into RPC context: got %v, want 8001", got)
	}

	body := w.Body.String()
	if !strings.Contains(body, "id: 3\n") {
		t.Fatalf("expected the ledger event's seq as an id: line, got %q", body)
	}
	if !strings.Contains(body, `"type":"CUSTOM","name":"step_detail"`) {
		t.Fatalf("expected the projected AG-UI frame, got %q", body)
	}
}

func TestProxyProcurementRunEvents_RequiresAuthentication(t *testing.T) {
	mock := &mockRunEventsServiceClient{}
	h := newRunEventsTestHolder(mock, true)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/runs/5150/events", nil)
	// 沒有 X-Account-Id：fakeJWTAuth 直接 401，handler 不會被呼叫到。

	w := httptest.NewRecorder()
	setupRunEventsRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnauthorized)
	if mock.gotReq != nil {
		t.Fatalf("WatchRunEvents must not be called for an unauthenticated request: %+v", mock.gotReq)
	}
}
