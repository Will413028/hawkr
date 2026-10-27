package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newTestGinContext builds a *gin.Context complete with a Request. The request
// is not optional decoration: every gateway error writer resolves a traceId via
// envelope.ExtractTraceID, which reads request headers and panics on a nil
// c.Request — a bare gin.CreateTestContext would only pass by accident, on a
// writer that happens not to emit traceId.
func newTestGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", nil)
	return c, rec
}

func TestWriteProcurementError_WithBusinessErrorDetail_UsesReasonAndMessage(t *testing.T) {
	st := status.New(codes.FailedPrecondition, "品項、配送城市、數量、到貨日皆備齊才能寄送報價單")
	stWithDetails, err := st.WithDetails(&aipb.BusinessError{Reason: "QUOTE_NOT_READY", Message: "品項、配送城市、數量、到貨日皆備齊才能寄送報價單"})
	if err != nil {
		t.Fatalf("WithDetails failed: %v", err)
	}

	c, rec := newTestGinContext()
	writeProcurementError(c, stWithDetails.Err())

	if rec.Code != 422 {
		t.Fatalf("HTTP status = %d, want 422", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"reason":"QUOTE_NOT_READY"`) {
		t.Fatalf("body missing reason field: %s", body)
	}
	if !strings.Contains(body, "品項、配送城市、數量、到貨日皆備齊才能寄送報價單") {
		t.Fatalf("body missing message text: %s", body)
	}
}

func TestWriteProcurementError_WithoutDetail_FallsBackToPlainMessage(t *testing.T) {
	c, rec := newTestGinContext()
	writeProcurementError(c, status.Error(codes.Internal, "internal error"))

	if rec.Code != 500 {
		t.Fatalf("HTTP status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"reason":""`) {
		t.Fatalf("expected empty reason on fallback path: %s", rec.Body.String())
	}
}

// procurementErrorKeys is the exact key set every procurement error response —
// RPC-originated or Gateway pre-flight — must carry (traceId only when a trace
// is actually present, matching envelope.Envelope's own omitempty).
func procurementErrorKeys(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("response body is not JSON: %v (%s)", err, body)
	}
	for _, k := range []string{"code", "reason", "message"} {
		if _, ok := got[k]; !ok {
			t.Fatalf("error envelope missing %q key: %s", k, body)
		}
	}
	return got
}

// TestProcurementErrorEnvelope_RPCAndPreflightShareOneShape is the regression
// guard for the defect this pairing closes: before it, RPC-originated errors
// went through writeProcurementError and emitted {code,reason,message} (no
// traceId), while every local pre-flight rejection went through
// pkg/envelope's response.BadRequest/response.Err and emitted
// {code,message,traceId} (no reason) — so `.reason` was undefined for a
// meaningful fraction of the 18 endpoints' 400/422 responses. Both writers must
// now be indistinguishable from the outside.
func TestProcurementErrorEnvelope_RPCAndPreflightShareOneShape(t *testing.T) {
	rpcCtx, rpcRec := newTestGinContext()
	rpcCtx.Request.Header.Set("x-trace-id", "trace-abc")
	writeProcurementError(rpcCtx, status.Error(codes.InvalidArgument, "clientRequestId is required"))

	preflightCtx, preflightRec := newTestGinContext()
	preflightCtx.Request.Header.Set("x-trace-id", "trace-abc")
	badProcurementRequest(preflightCtx, "clientRequestId is required")

	if rpcRec.Code != preflightRec.Code {
		t.Fatalf("HTTP status differs: rpc=%d preflight=%d", rpcRec.Code, preflightRec.Code)
	}
	if got, want := rpcRec.Body.String(), preflightRec.Body.String(); got != want {
		t.Fatalf("envelope bytes differ for the same rejection:\n rpc      %s\n preflight %s", got, want)
	}

	got := procurementErrorKeys(t, rpcRec.Body.Bytes())
	if got["traceId"] != "trace-abc" {
		t.Fatalf("traceId not preserved on the procurement error envelope: %v", got)
	}
	if got["code"] != float64(400) || got["reason"] != "" || got["message"] != "clientRequestId is required" {
		t.Fatalf("unexpected envelope contents: %v", got)
	}
}

// TestWriteProcurementErrorBody_WithReason_KeepsReasonAndMessageSplit pins the
// one pre-flight rejection that carries a machine code: Gateway's INPUT_TOO_LONG
// 422 must be byte-identical to what ai-service's own SubmitTurn RPC produces
// for the same input (procurementStatus(codes.FailedPrecondition,
// "INPUT_TOO_LONG", "單則訊息上限 2000 字") → 422 through the same table), so a
// client cannot tell which layer rejected it.
func TestWriteProcurementErrorBody_WithReason_KeepsReasonAndMessageSplit(t *testing.T) {
	preflightCtx, preflightRec := newTestGinContext()
	writeProcurementErrorBody(preflightCtx, http.StatusUnprocessableEntity, "INPUT_TOO_LONG", "單則訊息上限 2000 字")

	st := status.New(codes.FailedPrecondition, "單則訊息上限 2000 字")
	stWithDetails, err := st.WithDetails(&aipb.BusinessError{Reason: "INPUT_TOO_LONG", Message: "單則訊息上限 2000 字"})
	if err != nil {
		t.Fatalf("WithDetails failed: %v", err)
	}
	rpcCtx, rpcRec := newTestGinContext()
	writeProcurementError(rpcCtx, stWithDetails.Err())

	if preflightRec.Code != http.StatusUnprocessableEntity || rpcRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("INPUT_TOO_LONG must be 422 on both layers: preflight=%d rpc=%d", preflightRec.Code, rpcRec.Code)
	}
	if got, want := preflightRec.Body.String(), rpcRec.Body.String(); got != want {
		t.Fatalf("INPUT_TOO_LONG envelope differs between layers:\n preflight %s\n rpc       %s", got, want)
	}
}

// TestWriteProcurementError_QuotaExceeded_CarriesRemainingAndResetAt is the
// unary half of doc 57 §4.7. ai-service's procurementQuotaExceededStatus
// populates BusinessError.remaining/reset_at — the two fields Task 10 added to
// the proto for this one reason — and this writer must carry both onto the wire
// rather than reading only reason/message off the same detail.
func TestWriteProcurementError_QuotaExceeded_CarriesRemainingAndResetAt(t *testing.T) {
	const message = "本月 Agent 回答次數已用完（30/30），2026-09-01T00:00:00Z 後重置"
	st := status.New(codes.ResourceExhausted, message)
	stWithDetails, err := st.WithDetails(&aipb.BusinessError{
		Reason: "AGENT_MONTHLY_QUOTA_EXCEEDED", Message: message,
		Remaining: "0", ResetAt: "2026-09-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("WithDetails failed: %v", err)
	}

	c, rec := newTestGinContext()
	writeProcurementError(c, stWithDetails.Err())

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("HTTP status = %d, want 429", rec.Code)
	}
	got := procurementErrorKeys(t, rec.Body.Bytes())
	if got["reason"] != "AGENT_MONTHLY_QUOTA_EXCEEDED" {
		t.Fatalf("reason = %v, want AGENT_MONTHLY_QUOTA_EXCEEDED (body %s)", got["reason"], rec.Body.String())
	}
	if got["remaining"] != "0" {
		t.Fatalf("remaining = %v, want \"0\" (body %s)", got["remaining"], rec.Body.String())
	}
	if got["resetAt"] != "2026-09-01T00:00:00Z" {
		t.Fatalf("resetAt = %v, want the ISO 8601 reset instant (body %s)", got["resetAt"], rec.Body.String())
	}
}

// TestWriteProcurementError_NonQuotaReason_OmitsQuotaKeys pins the additive
// half: remaining/resetAt are `omitempty` because every other
// ProcurementErrorReason leaves them empty on the BusinessError, and
// ai_procurement_error_envelope_test.go asserts the EXACT key set on all 18
// endpoints — two permanently-empty keys would turn that gate red platform-wide.
func TestWriteProcurementError_NonQuotaReason_OmitsQuotaKeys(t *testing.T) {
	st := status.New(codes.FailedPrecondition, "品項、配送城市、數量、到貨日皆備齊才能寄送報價單")
	stWithDetails, err := st.WithDetails(&aipb.BusinessError{
		Reason: "QUOTE_NOT_READY", Message: "品項、配送城市、數量、到貨日皆備齊才能寄送報價單",
	})
	if err != nil {
		t.Fatalf("WithDetails failed: %v", err)
	}

	c, rec := newTestGinContext()
	writeProcurementError(c, stWithDetails.Err())

	got := procurementErrorKeys(t, rec.Body.Bytes())
	for _, k := range []string{"remaining", "resetAt"} {
		if _, present := got[k]; present {
			t.Fatalf("non-quota error envelope gained %q: %s", k, rec.Body.String())
		}
	}
}
