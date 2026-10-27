package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"google.golang.org/grpc/codes"
)

// This file is the regression guard for the "two incompatible error envelope
// shapes across the same 18 endpoints" defect: RPC-originated failures came out
// as {code,reason,message} while every LOCAL pre-flight validation rejection
// went through pkg/envelope and came out as {code,message,traceId}. A client
// reading `.reason` got undefined for a meaningful fraction of 400/422
// responses, which defeats the point of this migration's reason/message split
// (spec §6/§7).
//
// Both paths must now produce the identical key set on ALL 18 cut-over
// endpoints — which is why these tests enumerate every endpoint by name rather
// than spot-checking a few: the defect was that some endpoints differed from
// others, so a sample would not have caught it.

// wantProcurementErrorKeys is the shape every procurement error response must
// have. traceId is present here because these tests always send a trace header;
// it is `omitempty` on the struct, matching envelope.Envelope.
var wantProcurementErrorKeys = []string{"code", "message", "reason", "traceId"}

func assertProcurementErrorEnvelope(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantReason string) {
	t.Helper()
	if w.Code != wantStatus {
		t.Fatalf("HTTP status = %d, want %d (body %s)", w.Code, wantStatus, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v (%s)", err, w.Body.String())
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != strings.Join(wantProcurementErrorKeys, ",") {
		t.Fatalf("envelope keys = %v, want %v (body %s)", keys, wantProcurementErrorKeys, w.Body.String())
	}
	if got["code"] != float64(wantStatus) {
		t.Fatalf("envelope code = %v, want %d", got["code"], wantStatus)
	}
	if got["reason"] != wantReason {
		t.Fatalf("envelope reason = %v, want %q", got["reason"], wantReason)
	}
	if got["traceId"] != "trace-envelope" {
		t.Fatalf("envelope lost traceId: %v", got)
	}
}

func newProcurementErrorRequest(method, path, body string) *http.Request {
	var req *http.Request
	if body == "" {
		req, _ = http.NewRequest(method, path, nil)
	} else {
		req, _ = http.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Account-Id", "99")
	req.Header.Set("x-trace-id", "trace-envelope")
	return req
}

// procurementRPCErrorCase drives one endpoint's RPC-originated failure by
// arming the corresponding field on the shared mock.
type procurementRPCErrorCase struct {
	name   string
	method string
	path   string
	body   string
	arm    func(m *mockAIProcurementServiceClient, err error)
}

// allProcurementUnaryEndpoints lists all 16 unary/JSON endpoints of the 17
// cut-over set (the 17th, ProxyProcurementTurnStream, is SSE and is covered by
// TestProcurementErrorEnvelope_StreamPreflightMatchesUnary below — its
// pre-flight rejections are plain JSON like the rest, its mid-stream failures
// are SSE frames by definition and cannot share this shape).
func allProcurementUnaryEndpoints() []procurementRPCErrorCase {
	return []procurementRPCErrorCase{
		{"Turn", http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", `{"clientRequestId":"c-1","message":"hi"}`,
			func(m *mockAIProcurementServiceClient, err error) { m.submitTurnErr = err }},
		{"Snapshot", http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/1/snapshot", "",
			func(m *mockAIProcurementServiceClient, err error) { m.getSnapshotErr = err }},
		{"CandidateSearch", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/candidate-searches", "",
			func(m *mockAIProcurementServiceClient, err error) { m.startCandidateSearchErr = err }},
		{"CandidateSearchGet", http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/1/candidate-searches/2", "",
			func(m *mockAIProcurementServiceClient, err error) { m.getCandidateSearchErr = err }},
		{"Selection", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/selections", `{"candidateSetId":"2","lane":"PLATFORM","candidateId":"3","selected":true}`,
			func(m *mockAIProcurementServiceClient, err error) { m.toggleSelectionErr = err }},
		{"Answer", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/answers", `{"clientRequestId":"c-1","slotKey":"unit","optionId":"o-1","expectedRevisionId":"5"}`,
			func(m *mockAIProcurementServiceClient, err error) { m.submitAnswerErr = err }},
		{"Rewind", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/rewinds", `{"clientRequestId":"c-1","slotKey":"unit"}`,
			func(m *mockAIProcurementServiceClient, err error) { m.rewindErr = err }},
		{"Conversations", http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations", "",
			func(m *mockAIProcurementServiceClient, err error) { m.listConversationsErr = err }},
		{"ConversationMessages", http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations/1/messages", "",
			func(m *mockAIProcurementServiceClient, err error) { m.getConversationMessagesErr = err }},
		{"ConversationRename", http.MethodPatch, "/api/v1/app/buyers/me/procurement/conversations/1", `{"title":"新標題","clientRequestId":"c-1"}`,
			func(m *mockAIProcurementServiceClient, err error) { m.renameConversationErr = err }},
		{"ConversationDelete", http.MethodDelete, "/api/v1/app/buyers/me/procurement/conversations/1", `{"clientRequestId":"c-1"}`,
			func(m *mockAIProcurementServiceClient, err error) { m.deleteConversationErr = err }},
	}
}

// TestProcurementErrorEnvelope_AllRPCErrorPathsShareOneShape covers the
// RPC-originated half on every unary endpoint.
func TestProcurementErrorEnvelope_AllRPCErrorPathsShareOneShape(t *testing.T) {
	cases := allProcurementUnaryEndpoints()
	if len(cases) != 11 {
		t.Fatalf("expected all 11 unary procurement endpoints to be listed, got %d", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockAIProcurementServiceClient{}
			tc.arm(mock, mustWithDetails(t, codes.FailedPrecondition, "品項、配送城市、數量、到貨日皆備齊才能寄送報價單",
				&aipb.BusinessError{Reason: "QUOTE_NOT_READY", Message: "品項、配送城市、數量、到貨日皆備齊才能寄送報價單"}))

			w := httptest.NewRecorder()
			setupProcurementUnaryRouter(newProcurementTestHolder(mock)).ServeHTTP(w, newProcurementErrorRequest(tc.method, tc.path, tc.body))

			assertProcurementErrorEnvelope(t, w, http.StatusUnprocessableEntity, "QUOTE_NOT_READY")
		})
	}
}

// TestProcurementErrorEnvelope_AllPreflightRejectionsShareOneShape covers the
// local-validation half: every endpoint that parses a JSON body must reject a
// malformed one with the SAME envelope the RPC path produces, not
// pkg/envelope's reason-less one.
func TestProcurementErrorEnvelope_AllPreflightRejectionsShareOneShape(t *testing.T) {
	cases := []struct{ name, method, path string }{
		{"Turn", http.MethodPost, "/api/v1/app/buyers/me/procurement/turns"},
		{"Selection", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/selections"},
		{"Answer", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/answers"},
		{"Rewind", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/rewinds"},
		{"ConversationRename", http.MethodPatch, "/api/v1/app/buyers/me/procurement/conversations/1"},
		{"ConversationDelete", http.MethodDelete, "/api/v1/app/buyers/me/procurement/conversations/1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockAIProcurementServiceClient{}
			w := httptest.NewRecorder()
			setupProcurementUnaryRouter(newProcurementTestHolder(mock)).ServeHTTP(w, newProcurementErrorRequest(tc.method, tc.path, `{"broken":`))

			assertProcurementErrorEnvelope(t, w, http.StatusBadRequest, "")
		})
	}
}

// TestProcurementErrorEnvelope_FieldValidationRejections covers the per-field
// pre-flight rejections that are not "malformed body" — the ones that used to
// go through response.Err and therefore had no `reason` key at all. The
// INPUT_TOO_LONG case additionally pins the reason/message split (422 +
// machine code), which is what the RPC side now emits too.
func TestProcurementErrorEnvelope_FieldValidationRejections(t *testing.T) {
	longMessage := strings.Repeat("字", 2001)
	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantReason string
	}{
		{"Turn/missing clientRequestId", http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", `{"message":"hi"}`, http.StatusBadRequest, ""},
		{"Turn/missing message", http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", `{"clientRequestId":"c-1"}`, http.StatusBadRequest, ""},
		{"Turn/message too long", http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", `{"clientRequestId":"c-1","message":"` + longMessage + `"}`, http.StatusUnprocessableEntity, "INPUT_TOO_LONG"},
		{"Answer/missing clientRequestId", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/answers", `{"slotKey":"unit","optionId":"o-1"}`, http.StatusBadRequest, ""},
		{"Answer/missing slotKey", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/answers", `{"clientRequestId":"c-1","optionId":"o-1"}`, http.StatusBadRequest, ""},
		{"Answer/over-specified", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/answers", `{"clientRequestId":"c-1","slotKey":"unit","optionId":"o-1","freeText":"x"}`, http.StatusBadRequest, ""},
		{"Answer/quantity rejects optionId", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/answers", `{"clientRequestId":"c-1","slotKey":"quantity","optionId":"o-1"}`, http.StatusBadRequest, ""},
		{"Rewind/missing clientRequestId", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/rewinds", `{"slotKey":"unit"}`, http.StatusBadRequest, ""},
		{"Rewind/missing slotKey", http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/1/rewinds", `{"clientRequestId":"c-1"}`, http.StatusBadRequest, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockAIProcurementServiceClient{}
			w := httptest.NewRecorder()
			setupProcurementUnaryRouter(newProcurementTestHolder(mock)).ServeHTTP(w, newProcurementErrorRequest(tc.method, tc.path, tc.body))

			assertProcurementErrorEnvelope(t, w, tc.wantStatus, tc.wantReason)
		})
	}
}

// TestProcurementErrorEnvelope_StreamPreflightMatchesUnary is the 17th
// endpoint. Its mid-stream failures are SSE frames (a different contract by
// construction — see ServeAIProcurementStream), but everything it rejects
// BEFORE switching the connection into SSE mode is a plain JSON response and
// must match the other 16.
func TestProcurementErrorEnvelope_StreamPreflightMatchesUnary(t *testing.T) {
	longMessage := strings.Repeat("字", 2001)
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantReason string
	}{
		{"malformed body", `{"broken":`, http.StatusBadRequest, ""},
		{"missing clientRequestId", `{"message":"hi"}`, http.StatusBadRequest, ""},
		{"missing message", `{"clientRequestId":"c-1"}`, http.StatusBadRequest, ""},
		{"message too long", `{"clientRequestId":"c-1","message":"` + longMessage + `"}`, http.StatusUnprocessableEntity, "INPUT_TOO_LONG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockProcurementStreamServiceClient{}
			w := httptest.NewRecorder()
			setupProcurementStreamRouter(newProcurementStreamTestHolder(mock)).ServeHTTP(w,
				newProcurementErrorRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns/stream", tc.body))

			assertProcurementErrorEnvelope(t, w, tc.wantStatus, tc.wantReason)
			if mock.gotReq != nil {
				t.Fatalf("pre-flight rejection must not reach the RPC: %+v", mock.gotReq)
			}
		})
	}
}
