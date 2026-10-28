package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	grpcclient "github.com/Will413028/hawkr/gateway/internal/grpc"
	aipb "github.com/Will413028/hawkr/go-pkg/proto/ai"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type mockAIProcurementServiceClient struct {
	aipb.AIProcurementServiceClient

	submitTurnResp   *aipb.TurnResult
	submitTurnErr    error
	gotSubmitTurnReq *aipb.TurnRequest

	getSnapshotResp *aipb.SnapshotResponse
	getSnapshotErr  error

	startCandidateSearchResp *aipb.StartCandidateSearchResponse
	startCandidateSearchErr  error

	getCandidateSearchResp *aipb.CandidateSetResponse
	getCandidateSearchErr  error

	toggleSelectionResp   *aipb.ToggleSelectionResponse
	toggleSelectionErr    error
	gotToggleSelectionReq *aipb.ToggleSelectionRequest

	submitAnswerResp   *aipb.TurnResult
	submitAnswerErr    error
	gotSubmitAnswerReq *aipb.SubmitAnswerRequest

	rewindResp   *aipb.TurnResult
	rewindErr    error
	gotRewindReq *aipb.RewindRequest

	previewRfqPriceResp   *aipb.RfqPricePreviewResponse
	previewRfqPriceErr    error
	gotPreviewRfqPriceReq *aipb.RfqPricePreviewRequest

	confirmRfqSnapshotResp   *aipb.RfqConfirmationResponse
	confirmRfqSnapshotErr    error
	gotConfirmRfqSnapshotReq *aipb.RfqConfirmationRequest

	previewRfqPublicationResp   *aipb.RfqPublicationPreviewResponse
	previewRfqPublicationErr    error
	gotPreviewRfqPublicationReq *aipb.RfqPublicationPreviewRequest

	executeRfqPublicationResp   *aipb.RfqPublicationExecuteResponse
	executeRfqPublicationErr    error
	gotExecuteRfqPublicationReq *aipb.RfqPublicationExecuteRequest

	listConversationsResp   *aipb.ListConversationsResponse
	listConversationsErr    error
	gotListConversationsReq *aipb.ListConversationsRequest

	getConversationMessagesResp *aipb.ConversationMessagesResponse
	getConversationMessagesErr  error

	renameConversationErr    error
	gotRenameConversationReq *aipb.RenameConversationRequest

	deleteConversationErr    error
	gotDeleteConversationReq *aipb.DeleteConversationRequest

	cancelRunResp   *aipb.CancelRunResponse
	cancelRunErr    error
	gotCancelRunReq *aipb.CancelRunRequest

	resumeRunResp   *aipb.ResumeRunResponse
	resumeRunErr    error
	gotResumeRunReq *aipb.ResumeRunRequest
}

func (m *mockAIProcurementServiceClient) SubmitTurn(ctx context.Context, in *aipb.TurnRequest, opts ...grpc.CallOption) (*aipb.TurnResult, error) {
	m.gotSubmitTurnReq = in
	return m.submitTurnResp, m.submitTurnErr
}

func (m *mockAIProcurementServiceClient) GetSnapshot(ctx context.Context, in *aipb.GetSnapshotRequest, opts ...grpc.CallOption) (*aipb.SnapshotResponse, error) {
	return m.getSnapshotResp, m.getSnapshotErr
}

func (m *mockAIProcurementServiceClient) StartCandidateSearch(ctx context.Context, in *aipb.StartCandidateSearchRequest, opts ...grpc.CallOption) (*aipb.StartCandidateSearchResponse, error) {
	return m.startCandidateSearchResp, m.startCandidateSearchErr
}

func (m *mockAIProcurementServiceClient) GetCandidateSearch(ctx context.Context, in *aipb.GetCandidateSearchRequest, opts ...grpc.CallOption) (*aipb.CandidateSetResponse, error) {
	return m.getCandidateSearchResp, m.getCandidateSearchErr
}

func (m *mockAIProcurementServiceClient) ToggleSelection(ctx context.Context, in *aipb.ToggleSelectionRequest, opts ...grpc.CallOption) (*aipb.ToggleSelectionResponse, error) {
	m.gotToggleSelectionReq = in
	return m.toggleSelectionResp, m.toggleSelectionErr
}

func (m *mockAIProcurementServiceClient) SubmitAnswer(ctx context.Context, in *aipb.SubmitAnswerRequest, opts ...grpc.CallOption) (*aipb.TurnResult, error) {
	m.gotSubmitAnswerReq = in
	return m.submitAnswerResp, m.submitAnswerErr
}

func (m *mockAIProcurementServiceClient) Rewind(ctx context.Context, in *aipb.RewindRequest, opts ...grpc.CallOption) (*aipb.TurnResult, error) {
	m.gotRewindReq = in
	return m.rewindResp, m.rewindErr
}

func (m *mockAIProcurementServiceClient) PreviewRfqPrice(ctx context.Context, in *aipb.RfqPricePreviewRequest, opts ...grpc.CallOption) (*aipb.RfqPricePreviewResponse, error) {
	m.gotPreviewRfqPriceReq = in
	return m.previewRfqPriceResp, m.previewRfqPriceErr
}

func (m *mockAIProcurementServiceClient) ConfirmRfqSnapshot(ctx context.Context, in *aipb.RfqConfirmationRequest, opts ...grpc.CallOption) (*aipb.RfqConfirmationResponse, error) {
	m.gotConfirmRfqSnapshotReq = in
	return m.confirmRfqSnapshotResp, m.confirmRfqSnapshotErr
}

func (m *mockAIProcurementServiceClient) PreviewRfqPublication(ctx context.Context, in *aipb.RfqPublicationPreviewRequest, opts ...grpc.CallOption) (*aipb.RfqPublicationPreviewResponse, error) {
	m.gotPreviewRfqPublicationReq = in
	return m.previewRfqPublicationResp, m.previewRfqPublicationErr
}

func (m *mockAIProcurementServiceClient) ExecuteRfqPublication(ctx context.Context, in *aipb.RfqPublicationExecuteRequest, opts ...grpc.CallOption) (*aipb.RfqPublicationExecuteResponse, error) {
	m.gotExecuteRfqPublicationReq = in
	return m.executeRfqPublicationResp, m.executeRfqPublicationErr
}

func (m *mockAIProcurementServiceClient) ListConversations(ctx context.Context, in *aipb.ListConversationsRequest, opts ...grpc.CallOption) (*aipb.ListConversationsResponse, error) {
	m.gotListConversationsReq = in
	return m.listConversationsResp, m.listConversationsErr
}

func (m *mockAIProcurementServiceClient) GetConversationMessages(ctx context.Context, in *aipb.GetConversationMessagesRequest, opts ...grpc.CallOption) (*aipb.ConversationMessagesResponse, error) {
	return m.getConversationMessagesResp, m.getConversationMessagesErr
}

func (m *mockAIProcurementServiceClient) RenameConversation(ctx context.Context, in *aipb.RenameConversationRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	m.gotRenameConversationReq = in
	if m.renameConversationErr != nil {
		return nil, m.renameConversationErr
	}
	return &emptypb.Empty{}, nil
}

func (m *mockAIProcurementServiceClient) DeleteConversation(ctx context.Context, in *aipb.DeleteConversationRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	m.gotDeleteConversationReq = in
	if m.deleteConversationErr != nil {
		return nil, m.deleteConversationErr
	}
	return &emptypb.Empty{}, nil
}

func (m *mockAIProcurementServiceClient) CancelRun(ctx context.Context, in *aipb.CancelRunRequest, opts ...grpc.CallOption) (*aipb.CancelRunResponse, error) {
	m.gotCancelRunReq = in
	return m.cancelRunResp, m.cancelRunErr
}

func (m *mockAIProcurementServiceClient) ResumeRun(ctx context.Context, in *aipb.ResumeRunRequest, opts ...grpc.CallOption) (*aipb.ResumeRunResponse, error) {
	m.gotResumeRunReq = in
	return m.resumeRunResp, m.resumeRunErr
}

func setupProcurementUnaryRouter(h *Holder) *gin.Engine {
	r := gin.New()
	app := r.Group("/api/v1/app")
	app.Use(fakeJWTAuth())
	buyers := app.Group("/buyers/me/procurement")
	buyers.POST("/turns", h.ProxyProcurementTurn)
	buyers.GET("/requests/:id/snapshot", h.ProxyProcurementSnapshot)
	buyers.POST("/requests/:id/candidate-searches", h.ProxyProcurementCandidateSearch)
	buyers.GET("/requests/:id/candidate-searches/:runId", h.ProxyProcurementCandidateSearchGet)
	buyers.POST("/requests/:id/selections", h.ProxyProcurementSelection)
	buyers.POST("/requests/:id/answers", h.ProxyProcurementAnswer)
	buyers.POST("/requests/:id/rewinds", h.ProxyProcurementRewind)
	buyers.POST("/requests/:id/rfq-price-previews", h.ProxyProcurementRfqPricePreview)
	buyers.POST("/requests/:id/rfq-confirmations", h.ProxyProcurementRfqConfirmation)
	buyers.POST("/requests/:id/rfq-publications/preview", h.ProxyProcurementRfqPublicationPreview)
	buyers.POST("/requests/:id/rfq-publications", h.ProxyProcurementRfqPublicationExecute)
	buyers.GET("/conversations", h.ProxyProcurementConversations)
	buyers.GET("/conversations/:id/messages", h.ProxyProcurementConversationMessages)
	buyers.PATCH("/conversations/:id", h.ProxyProcurementConversationRename)
	buyers.DELETE("/conversations/:id", h.ProxyProcurementConversationDelete)
	buyers.POST("/runs/:runId/cancel", h.ProxyProcurementRunCancel)
	buyers.POST("/runs/:runId/resume", h.ProxyProcurementRunResume)
	return r
}

func newProcurementTestHolder(mock *mockAIProcurementServiceClient) *Holder {
	return &Holder{
		AIProcurement: grpcclient.NewAIProcurementClientForTest(mock),
		BuyerID:       buyerResolverForTest(8001),
		Store:         storeResolverForTest(7001),
		MediaBaseURL:  "https://media.test",
	}
}

func mustWithDetails(t *testing.T, code codes.Code, msg string, detail *aipb.BusinessError) error {
	t.Helper()
	st, err := status.New(code, msg).WithDetails(detail)
	if err != nil {
		t.Fatalf("WithDetails failed: %v", err)
	}
	return st.Err()
}

func TestProxyProcurementTurn_Success_ReturnsTurnResultJSON(t *testing.T) {
	quantity := "50"
	item := "寶島梨"
	mock := &mockAIProcurementServiceClient{
		submitTurnResp: &aipb.TurnResult{
			RequestId: "77", ConversationId: "5", RevisionId: "3",
			Slots:            &aipb.Slots{Item: &item, Quantity: &quantity},
			Rejected:         []*aipb.RejectedField{},
			MissingRequired:  []string{"unit"},
			RecordedThisTurn: []*aipb.RecordedField{{Key: "item", Label: "品項", Value: "寶島梨"}},
			Reply:            "已記錄品項與數量",
			Degraded:         false,
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","message":"我要買50箱寶島梨"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotSubmitTurnReq.GetClientRequestId() != "c-1" || mock.gotSubmitTurnReq.GetMessage() != "我要買50箱寶島梨" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotSubmitTurnReq)
	}

	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["requestId"] != "77" {
		t.Fatalf("requestId = %v, want quoted string \"77\"", data["requestId"])
	}
	if data["reply"] != "已記錄品項與數量" {
		t.Fatalf("reply = %v", data["reply"])
	}
	slots, ok := data["slots"].(map[string]interface{})
	if !ok {
		t.Fatalf("slots is not an object: %v", data["slots"])
	}
	if q, ok := slots["quantity"].(float64); !ok || q != 50 {
		t.Fatalf("slots.quantity = %v (%T), want JSON number 50", slots["quantity"], slots["quantity"])
	}
	if _, present := slots["delivery_city"]; present {
		t.Fatalf("slots.delivery_city should be omitted when unset (omitempty): %v", slots)
	}
	if _, present := data["degradedReason"]; present {
		t.Fatalf("degradedReason should be omitted when empty (omitempty): %v", data)
	}
	if _, present := data["steps"]; present {
		t.Fatalf("steps should be omitted when empty (omitempty): %v", data)
	}
	if q, present := data["question"]; !present || q != nil {
		t.Fatalf("question should be present and explicitly null when unset: %v", data)
	}
}

func TestProxyProcurementTurn_UsecaseError_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		submitTurnErr: mustWithDetails(t, codes.Aborted, "clientRequestId 已用於其他需求，請改用新的 clientRequestId",
			&aipb.BusinessError{Reason: "IDEMPOTENCY_KEY_CONFLICT", Message: "clientRequestId 已用於其他需求，請改用新的 clientRequestId"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","message":"hi"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "IDEMPOTENCY_KEY_CONFLICT" {
		t.Fatalf("reason = %v, want IDEMPOTENCY_KEY_CONFLICT", respBody["reason"])
	}
}

func TestProxyProcurementTurn_MissingClientRequestID_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	body := `{"message":"hi"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotSubmitTurnReq != nil {
		t.Fatalf("SubmitTurn should not have been called: %+v", mock.gotSubmitTurnReq)
	}
}

func TestProxyProcurementTurn_MessageTooLong_ReturnsUnprocessableEntity(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	longMessage := strings.Repeat("字", 2001)
	body := `{"clientRequestId":"c-1","message":"` + longMessage + `"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnprocessableEntity)
	if mock.gotSubmitTurnReq != nil {
		t.Fatalf("SubmitTurn should not have been called for an over-length message: %+v", mock.gotSubmitTurnReq)
	}
}

func TestProxyProcurementTurn_CarriesBlocksInResponse(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		submitTurnResp: &aipb.TurnResult{
			RequestId: "77", ConversationId: "5", RevisionId: "3",
			Reply:        "已收到你的訊息。",
			Blocks:       []*aipb.Block{{Kind: "UNDERSTOOD", Text: "已收到你的訊息。"}},
			DialoguePlan: &aipb.DialoguePlanSummary{ActCode: "ACK_AND_ASK_REQUIRED"},
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","message":"我要買50箱寶島梨"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/turns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")

	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})

	blocks, ok := data["blocks"].([]interface{})
	if !ok || len(blocks) == 0 {
		t.Fatalf("data.blocks should be a non-empty array, got %v (%T)", data["blocks"], data["blocks"])
	}
	block := blocks[0].(map[string]interface{})
	if block["kind"] != "UNDERSTOOD" {
		t.Fatalf("blocks[0].kind = %v, want UNDERSTOOD", block["kind"])
	}

	dialoguePlan, ok := data["dialoguePlan"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.dialoguePlan is not an object: %v", data["dialoguePlan"])
	}
	if dialoguePlan["dialogueActCode"] != "ACK_AND_ASK_REQUIRED" {
		t.Fatalf("data.dialoguePlan.dialogueActCode = %v, want ACK_AND_ASK_REQUIRED", dialoguePlan["dialogueActCode"])
	}
}

func TestProxyProcurementSnapshot_Success_ReturnsSnapshotJSON(t *testing.T) {
	item := "紅龍果"
	mock := &mockAIProcurementServiceClient{
		getSnapshotResp: &aipb.SnapshotResponse{
			RequestId: "77", RevisionId: "3", Status: "REQUIREMENT_READY",
			Slots:           &aipb.Slots{Item: &item},
			MissingRequired: []string{},
			CropResolution:  &aipb.CropResolution{Candidates: []*aipb.CropDisambiguationOption{}},
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/77/snapshot", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["status"] != "REQUIREMENT_READY" {
		t.Fatalf("status = %v", data["status"])
	}
	cr, ok := data["cropResolution"].(map[string]interface{})
	if !ok {
		t.Fatalf("cropResolution is not an object: %v", data["cropResolution"])
	}
	candidates, ok := cr["candidates"].([]interface{})
	if !ok || len(candidates) != 0 {
		t.Fatalf("candidates should be an empty array, not null: %v", cr["candidates"])
	}
}

func TestProxyProcurementSnapshot_RunsAndActiveConfirmation_MapsToJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		getSnapshotResp: &aipb.SnapshotResponse{
			RequestId: "77", RevisionId: "3", Status: "WAITING_FOR_INPUT",
			Slots:           &aipb.Slots{},
			MissingRequired: []string{},
			CropResolution:  &aipb.CropResolution{Candidates: []*aipb.CropDisambiguationOption{}},
			Runs: []*aipb.AgentRunSummary{
				{RunId: "901", RunType: "TURN", Status: "WAITING_FOR_INPUT", LastSeqId: "5", MinAvailableSeqId: "1"},
			},
			ActiveRunId: "901",
			OpenConfirmation: &aipb.AgentRunOpenConfirmation{
				InterruptId: "int-1", ConfirmationKind: "CLARIFICATION", ExpiresAt: "",
			},
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/77/snapshot", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})

	runs, ok := data["runs"].([]interface{})
	if !ok || len(runs) != 1 {
		t.Fatalf("runs = %v, want a 1-element array", data["runs"])
	}
	run := runs[0].(map[string]interface{})
	if run["runId"] != "901" || run["runType"] != "TURN" || run["status"] != "WAITING_FOR_INPUT" ||
		run["lastSeqId"] != "5" || run["minAvailableSeqId"] != "1" {
		t.Fatalf("run summary = %v, want run 901's fields to round-trip", run)
	}
	if data["activeRunId"] != "901" {
		t.Fatalf("activeRunId = %v, want \"901\"", data["activeRunId"])
	}
	oc, ok := data["openConfirmation"].(map[string]interface{})
	if !ok {
		t.Fatalf("openConfirmation is not an object: %v", data["openConfirmation"])
	}
	if oc["interruptId"] != "int-1" || oc["confirmationKind"] != "CLARIFICATION" || oc["expiresAt"] != "" {
		t.Fatalf("openConfirmation = %v, want {interruptId:int-1 confirmationKind:CLARIFICATION expiresAt:\"\"}", oc)
	}
}

func TestProxyProcurementSnapshot_NoActiveRun_OmitsActiveRunIdAndOpenConfirmation(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		getSnapshotResp: &aipb.SnapshotResponse{
			RequestId: "77", RevisionId: "3", Status: "CLARIFYING",
			Slots:           &aipb.Slots{},
			MissingRequired: []string{},
			CropResolution:  &aipb.CropResolution{Candidates: []*aipb.CropDisambiguationOption{}},
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/77/snapshot", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})

	if runs, ok := data["runs"].([]interface{}); !ok || len(runs) != 0 {
		t.Fatalf("runs = %v, want an empty array, not omitted or null", data["runs"])
	}
	if _, present := data["activeRunId"]; present {
		t.Fatalf("activeRunId should be omitted when there is no active run, got: %v", data["activeRunId"])
	}
	if _, present := data["openConfirmation"]; present {
		t.Fatalf("openConfirmation should be omitted when there is no active run, got: %v", data["openConfirmation"])
	}
}

func TestProxyProcurementSnapshot_NotFound_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{getSnapshotErr: status.Error(codes.NotFound, "request not found")}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/999/snapshot", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 404)
	if respBody["reason"] != "" {
		t.Fatalf("reason should fall back to empty string when the gRPC error carries no BusinessError detail, got %v", respBody["reason"])
	}
}

func TestProxyProcurementCandidateSearch_Success_ReturnsStartResponseJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		startCandidateSearchResp: &aipb.StartCandidateSearchResponse{
			RequestId: "77", RunId: "601", CandidateSetId: "501", Status: "RUNNING",
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/candidate-searches", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})

	for key, want := range map[string]any{
		"requestId":      "77",
		"runId":          "601",
		"candidateSetId": "501",
		"status":         "RUNNING",
	} {
		got, present := data[key]
		if !present {
			t.Fatalf("對外 JSON 缺少 key %q；實際 keys=%v", key, data)
		}
		if got != want {
			t.Fatalf("data[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestProxyProcurementCandidateSearch_RequirementNotReady_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		startCandidateSearchErr: mustWithDetails(t, codes.Aborted, "需求尚未就緒，仍有必填欄位缺漏",
			&aipb.BusinessError{Reason: "REQUIREMENT_NOT_READY", Message: "需求尚未就緒，仍有必填欄位缺漏"}),
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/candidate-searches", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "REQUIREMENT_NOT_READY" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementCandidateSearchGet_Success_ReturnsCandidateSetJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		getCandidateSearchResp: &aipb.CandidateSetResponse{
			RequestId: "77", RunId: "601", CandidateSetId: "501", Status: "SUCCEEDED",
			PlatformLaneStatus: "SUCCEEDED", ExternalLaneStatus: "SUCCEEDED",
			FailureReason: "",
			Candidates: &aipb.CandidateLanes{
				Platform: []*aipb.CandidateItem{{
					CandidateId: "1", SourceId: "9001", Lane: "PLATFORM", DisplayName: "紅龍果",
					Score: 0.9, MatchedFactsJson: `[{"key":"price"}]`, UnverifiableFactsJson: `[]`,
					FactsAsOf: "2026-08-16T00:00:00Z", Selected: true,
					StoreId: "1001", SellerName: "青果行 A", StoreCode: "25ABCDEFGH",
					LogoKey: "seller/1001/logo.jpg",
				}},
				External: []*aipb.CandidateItem{{
					CandidateId: "2", SourceId: "lead-7", Lane: "EXTERNAL", DisplayName: "站外果園",
					Score: 0.5, MatchedFactsJson: `[]`, UnverifiableFactsJson: `[]`,
					FactsAsOf: "2026-08-16T00:00:00Z",
				}},
			},
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/77/candidate-searches/501", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	candidates, ok := data["candidates"].(map[string]interface{})
	if !ok {
		t.Fatalf("candidates is not an object: %v", data["candidates"])
	}
	platform, ok := candidates["platform"].([]interface{})
	if !ok || len(platform) != 1 {
		t.Fatalf("platform candidates = %v", candidates["platform"])
	}
	item := platform[0].(map[string]interface{})
	if item["candidateId"] != "1" || item["displayName"] != "紅龍果" {
		t.Fatalf("item = %v", item)
	}
	matchedFacts, ok := item["matchedFacts"].([]interface{})
	if !ok || len(matchedFacts) != 1 {
		t.Fatalf("matchedFacts should be a raw JSON array, got %v (%T)", item["matchedFacts"], item["matchedFacts"])
	}
	external, ok := candidates["external"].([]interface{})
	if !ok || len(external) != 1 {
		t.Fatalf("external should be a 1-element array, not null: %v", candidates["external"])
	}

	for key, want := range map[string]any{
		"storeId":    "1001",
		"sellerName": "青果行 A",
		"storeCode":  "25ABCDEFGH",
		"logoKey":    "seller/1001/logo.jpg",
		"logoUrl":    "https://media.test/i/original/seller/1001/logo.jpg",
	} {
		got, present := item[key]
		if !present {
			t.Fatalf("PLATFORM 候選缺少 #2377 的 key %q；實際 keys=%v", key, item)
		}
		if got != want {
			t.Fatalf("item[%q] = %v, want %v", key, got, want)
		}
	}

	extItem := external[0].(map[string]interface{})
	for _, key := range []string{"storeId", "sellerName", "storeCode", "logoKey", "logoUrl"} {
		if _, present := extItem[key]; present {
			t.Fatalf("EXTERNAL 候選不得帶 %q（不偽造平台 seller identity）；實際 keys=%v", key, extItem)
		}
	}

	for key, want := range map[string]any{
		"requestId":          "77",
		"runId":              "601",
		"candidateSetId":     "501",
		"status":             "SUCCEEDED",
		"platformLaneStatus": "SUCCEEDED",
		"externalLaneStatus": "SUCCEEDED",
		"failureReason":      "",
	} {
		got, present := data[key]
		if !present {
			t.Fatalf("對外 JSON 缺少 key %q；實際 keys=%v", key, data)
		}
		if got != want {
			t.Fatalf("data[%q] = %v, want %v", key, got, want)
		}
	}

	if _, present := data["errorCode"]; present {
		t.Fatalf("errorCode 已於 #2446 契約凍結移除，不得再出現在對外 JSON；實際 keys=%v", data)
	}
}

func TestCandidateSetResponse_FailureReasonComesFromProto(t *testing.T) {
	dto := candidateSetResponseFromProto(&aipb.CandidateSetResponse{FailureReason: "WORKER_LOST"}, "")

	if dto.FailureReason != "WORKER_LOST" {
		t.Fatalf("FailureReason = %q, want WORKER_LOST——這是買家看得到的失敗原因，"+
			"接線斷掉的話前端只會顯示空白，沒有任何錯誤訊號", dto.FailureReason)
	}
}

func TestProxyProcurementCandidateSearchGet_NotFound_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{getCandidateSearchErr: status.Error(codes.NotFound, "candidate search run not found")}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/requests/77/candidate-searches/999", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 404)
}

func TestProxyProcurementSelection_Success_ReturnsToggleResponseJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		toggleSelectionResp: &aipb.ToggleSelectionResponse{
			RequestId: "77", CandidateSetId: "501", Lane: "PLATFORM", CandidateId: "1", Selected: true,
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"candidateSetId":"501","lane":"PLATFORM","candidateId":"1","selected":true}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/selections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotToggleSelectionReq.GetRequestId() != "77" || mock.gotToggleSelectionReq.GetCandidateId() != "1" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotToggleSelectionReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["selected"] != true || data["candidateSetId"] != "501" {
		t.Fatalf("data = %v", data)
	}
}

func TestProxyProcurementSelection_CandidateFactsStale_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		toggleSelectionErr: mustWithDetails(t, codes.Aborted, "候選資料已更新，請重新抓取候選並確認",
			&aipb.BusinessError{Reason: "CANDIDATE_FACTS_STALE", Message: "候選資料已更新，請重新抓取候選並確認"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"candidateSetId":"501","lane":"PLATFORM","candidateId":"1","selected":true}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/selections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "CANDIDATE_FACTS_STALE" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementAnswer_Success_ReturnsTurnResultJSON(t *testing.T) {
	item := "紅龍果"
	mock := &mockAIProcurementServiceClient{
		submitAnswerResp: &aipb.TurnResult{
			RequestId: "77", ConversationId: "5", RevisionId: "4",
			Slots:            &aipb.Slots{Item: &item},
			Rejected:         []*aipb.RejectedField{},
			MissingRequired:  []string{"delivery_city"},
			RecordedThisTurn: []*aipb.RecordedField{{Key: "item", Label: "品項", Value: "紅龍果"}},
			Reply:            "已記錄品項",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"item","freeText":"紅龍果","expectedRevisionId":"3","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/answers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotSubmitAnswerReq.GetRequestId() != "77" || mock.gotSubmitAnswerReq.GetSlotKey() != "item" ||
		mock.gotSubmitAnswerReq.GetFreeText() != "紅龍果" || mock.gotSubmitAnswerReq.GetExpectedRevisionId() != "3" ||
		mock.gotSubmitAnswerReq.GetClientRequestId() != "c-1" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotSubmitAnswerReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["requestId"] != "77" || data["reply"] != "已記錄品項" {
		t.Fatalf("data = %v", data)
	}
}

func TestProxyProcurementAnswer_CropDisambiguation_OtherWithFreeText_Accepted(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		submitAnswerResp: &aipb.TurnResult{RequestId: "77", ConversationId: "5", RevisionId: "4", Reply: "已記錄"},
	}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"crop_disambiguation","optionId":"crop-other","freeText":"野生黑木耳","expectedRevisionId":"3","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/answers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotSubmitAnswerReq == nil || mock.gotSubmitAnswerReq.GetOptionId() != "crop-other" ||
		mock.gotSubmitAnswerReq.GetFreeText() != "野生黑木耳" {
		t.Fatalf("request not forwarded to RPC as expected: %+v", mock.gotSubmitAnswerReq)
	}
}

func TestProxyProcurementAnswer_CropDisambiguation_OtherMissingFreeText_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"crop_disambiguation","optionId":"crop-other","expectedRevisionId":"3","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/answers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotSubmitAnswerReq != nil {
		t.Fatalf("SubmitAnswer should not have been called: %+v", mock.gotSubmitAnswerReq)
	}
}

func TestProxyProcurementAnswer_CropDisambiguation_RealCandidateWithFreeText_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"crop_disambiguation","optionId":"crop-CROP-A","freeText":"野生黑木耳","expectedRevisionId":"3","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/answers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotSubmitAnswerReq != nil {
		t.Fatalf("SubmitAnswer should not have been called: %+v", mock.gotSubmitAnswerReq)
	}
}

func TestProxyProcurementAnswer_CropDisambiguation_MissingOptionId_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"crop_disambiguation","freeText":"野生黑木耳","expectedRevisionId":"3","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/answers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotSubmitAnswerReq != nil {
		t.Fatalf("SubmitAnswer should not have been called: %+v", mock.gotSubmitAnswerReq)
	}
}

func TestProxyProcurementAnswer_StaleRevision_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		submitAnswerErr: mustWithDetails(t, codes.Aborted, "需求已被其他裝置改動",
			&aipb.BusinessError{Reason: "STALE_REVISION", Message: "需求已被其他裝置改動"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"item","freeText":"紅龍果","expectedRevisionId":"3","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/answers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "STALE_REVISION" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementAnswer_ConflictingFields_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"item","optionId":"o1","freeText":"紅龍果","expectedRevisionId":"3","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/answers", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotSubmitAnswerReq != nil {
		t.Fatalf("SubmitAnswer should not have been called: %+v", mock.gotSubmitAnswerReq)
	}
}

func TestProxyProcurementRewind_Success_ReturnsTurnResultJSON(t *testing.T) {
	item := "紅龍果"
	mock := &mockAIProcurementServiceClient{
		rewindResp: &aipb.TurnResult{
			RequestId: "77", ConversationId: "5", RevisionId: "6",
			Slots:            &aipb.Slots{Item: &item},
			Rejected:         []*aipb.RejectedField{},
			MissingRequired:  []string{},
			RecordedThisTurn: []*aipb.RecordedField{},
			Reply:            "重新回答：品項",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"item","clientRequestId":"c-2"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rewinds", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotRewindReq.GetRequestId() != "77" || mock.gotRewindReq.GetSlotKey() != "item" || mock.gotRewindReq.GetClientRequestId() != "c-2" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotRewindReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["reply"] != "重新回答：品項" {
		t.Fatalf("data = %v", data)
	}
}

func TestProxyProcurementRewind_ApprovalPending_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		rewindErr: mustWithDetails(t, codes.Aborted, "這個需求有一個尚未回覆的中斷，請改用 resume 回覆",
			&aipb.BusinessError{Reason: "APPROVAL_PENDING", Message: "這個需求有一個尚未回覆的中斷，請改用 resume 回覆"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"slotKey":"item","clientRequestId":"c-2"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rewinds", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "APPROVAL_PENDING" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqPricePreview_ReturnsPreviewJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPriceResp: &aipb.RfqPricePreviewResponse{
			PricePreviewId: "501", Version: "1",
			BuyerRequestedRange: &aipb.BuyerRequestedRange{Currency: "TWD", PriceComponent: "GOODS_ONLY", LowerBoundAmount: "30", UpperBoundAmount: "42", BasisQuantity: "1000", BasisUnit: "GRAM"},
			QuoteTotalRange:     &aipb.QuoteTotalRange{Currency: "TWD", LowerBoundAmount: "1500", UpperBoundAmount: "2100", Quantity: "50", QuantityUnit: "KG"},
			PayloadHash:         "hash-abc",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"expectedRevisionId":"3","clientRequestId":"c-1","deliveryCity":"台北市","buyerRequestedRange":{"currency":"TWD","lowerBoundAmount":"30","upperBoundAmount":"42"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-price-previews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotPreviewRfqPriceReq.GetRequestId() != "77" || mock.gotPreviewRfqPriceReq.GetExpectedRevisionId() != "3" ||
		mock.gotPreviewRfqPriceReq.GetClientRequestId() != "c-1" ||
		mock.gotPreviewRfqPriceReq.GetDeliveryCity() != "台北市" ||
		mock.gotPreviewRfqPriceReq.GetBuyerRequestedRange().GetLowerBoundAmount() != "30" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotPreviewRfqPriceReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["pricePreviewId"] != "501" || data["version"] != "1" || data["payloadHash"] != "hash-abc" {
		t.Fatalf("data = %v", data)
	}
	for _, gone := range []string{"pricingMode", "referenceAvailability", "referencePriceRange", "dealTemperature", "marketPriceBasis"} {
		if _, present := data[gone]; present {
			t.Errorf("%s 已不在回應契約內：%v", gone, data)
		}
	}
	buyerRange, ok := data["buyerRequestedRange"].(map[string]interface{})
	if !ok {
		t.Fatalf("buyerRequestedRange is not an object: %v", data["buyerRequestedRange"])
	}
	if buyerRange["currency"] != "TWD" || buyerRange["lowerBoundAmount"] != "30" || buyerRange["upperBoundAmount"] != "42" ||
		buyerRange["basisQuantity"] != "1000" || buyerRange["basisUnit"] != "GRAM" {
		t.Fatalf("buyerRequestedRange = %v", buyerRange)
	}
	total, ok := data["quoteTotalRange"].(map[string]interface{})
	if !ok || total["lowerBoundAmount"] != "1500" || total["quantityUnit"] != "KG" {
		t.Fatalf("quoteTotalRange = %v", data["quoteTotalRange"])
	}
}

func TestProxyProcurementRfqPricePreview_NilQuoteTotalRange_IsExplicitNull(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPriceResp: &aipb.RfqPricePreviewResponse{
			PricePreviewId: "504", Version: "1",
			BuyerRequestedRange: &aipb.BuyerRequestedRange{Currency: "TWD", LowerBoundAmount: "650", UpperBoundAmount: "850"},
			PayloadHash:         "hash-null",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"expectedRevisionId":"3","clientRequestId":"c-null","deliveryCity":"桃園市","buyerRequestedRange":{"currency":"TWD","lowerBoundAmount":"650","upperBoundAmount":"850"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-price-previews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	data := parseJSON(t, w.Body)["data"].(map[string]interface{})
	if v, present := data["quoteTotalRange"]; !present || v != nil {
		t.Fatalf("quoteTotalRange should be present and explicitly null: %v", data["quoteTotalRange"])
	}
}

func TestProxyProcurementRfqPricePreview_PriceBasisUnavailable_ReturnsUnprocessableEntity(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPriceErr: mustWithDetails(t, codes.FailedPrecondition, "currency/quantity/unit/conversion basis cannot be canonicalized",
			&aipb.BusinessError{Reason: "PRICE_BASIS_UNAVAILABLE", Message: "currency/quantity/unit/conversion basis cannot be canonicalized"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"expectedRevisionId":"3","clientRequestId":"c-3","deliveryCity":"台北市"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-price-previews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnprocessableEntity)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 422)
	if respBody["reason"] != "PRICE_BASIS_UNAVAILABLE" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqPricePreview_StaleRevision_ReturnsConflict(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPriceErr: mustWithDetails(t, codes.Aborted, "需求已被其他裝置改動",
			&aipb.BusinessError{Reason: "STALE_REVISION", Message: "需求已被其他裝置改動"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"expectedRevisionId":"2","clientRequestId":"c-4","deliveryCity":"台北市"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-price-previews", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "STALE_REVISION" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqPricePreview_InvalidBody_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-price-previews", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotPreviewRfqPriceReq != nil {
		t.Fatalf("PreviewRfqPrice should not have been called: %+v", mock.gotPreviewRfqPriceReq)
	}
}

func TestProxyProcurementRfqConfirmation_Success_ReturnsConfirmationJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		confirmRfqSnapshotResp: &aipb.RfqConfirmationResponse{
			ConfirmedRfqSnapshotId: "901", PayloadHash: "hash-confirm-1",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","expectedRevisionId":"3","pricePreviewId":"501","pricePreviewVersion":"1","payloadHash":"hash-abc"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-confirmations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotConfirmRfqSnapshotReq.GetRequestId() != "77" || mock.gotConfirmRfqSnapshotReq.GetClientRequestId() != "c-1" ||
		mock.gotConfirmRfqSnapshotReq.GetExpectedRevisionId() != "3" || mock.gotConfirmRfqSnapshotReq.GetPricePreviewId() != "501" ||
		mock.gotConfirmRfqSnapshotReq.GetPricePreviewVersion() != "1" || mock.gotConfirmRfqSnapshotReq.GetPayloadHash() != "hash-abc" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotConfirmRfqSnapshotReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["confirmedRfqSnapshotId"] != "901" || data["payloadHash"] != "hash-confirm-1" {
		t.Fatalf("data = %v", data)
	}
}

func TestProxyProcurementRfqConfirmation_StaleRevision_ReturnsConflict(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		confirmRfqSnapshotErr: mustWithDetails(t, codes.Aborted, "需求已被其他裝置改動",
			&aipb.BusinessError{Reason: "STALE_REVISION", Message: "需求已被其他裝置改動"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-2","expectedRevisionId":"2","pricePreviewId":"501","pricePreviewVersion":"1","payloadHash":"hash-abc"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-confirmations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "STALE_REVISION" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqConfirmation_StalePricePreview_ReturnsConflict(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		confirmRfqSnapshotErr: mustWithDetails(t, codes.Aborted, "報價預覽已過期，請重新試算",
			&aipb.BusinessError{Reason: "STALE_REVISION", Message: "報價預覽已過期，請重新試算"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-3","expectedRevisionId":"3","pricePreviewId":"501","pricePreviewVersion":"1","payloadHash":"stale-hash"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-confirmations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "STALE_REVISION" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqConfirmation_PricePreviewNotFound_ReturnsNotFound(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		confirmRfqSnapshotErr: mustWithDetails(t, codes.NotFound, "報價預覽不存在",
			&aipb.BusinessError{Reason: "RESOURCE_NOT_FOUND", Message: "報價預覽不存在"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-4","expectedRevisionId":"3","pricePreviewId":"999999","pricePreviewVersion":"1","payloadHash":"hash-abc"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-confirmations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 404)
	if respBody["reason"] != "RESOURCE_NOT_FOUND" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqConfirmation_InvalidBody_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-confirmations", strings.NewReader(`not-json`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotConfirmRfqSnapshotReq != nil {
		t.Fatalf("ConfirmRfqSnapshot should not have been called: %+v", mock.gotConfirmRfqSnapshotReq)
	}
}

func TestProxyProcurementConversations_Success_ReturnsListJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		listConversationsResp: &aipb.ListConversationsResponse{
			Records: []*aipb.ConversationSummary{{
				ConversationId: "5", Title: "買雞蛋", LastMessagePreview: "已記錄品項", UpdatedAt: "2026-08-16T00:00:00Z",
			}},
			HasMore: true, NextCursorId: "abc",
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations?cursor_id=start&page_size=10", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotListConversationsReq.GetCursorId() != "start" || mock.gotListConversationsReq.GetPageSize() != "10" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotListConversationsReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["hasMore"] != true || data["nextCursorId"] != "abc" {
		t.Fatalf("data = %v", data)
	}
	records, ok := data["records"].([]interface{})
	if !ok || len(records) != 1 {
		t.Fatalf("records = %v", data["records"])
	}
	record := records[0].(map[string]interface{})
	if record["conversationId"] != "5" || record["title"] != "買雞蛋" {
		t.Fatalf("record = %v", record)
	}
}

func TestProxyProcurementConversations_InternalError_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{listConversationsErr: status.Error(codes.Internal, "internal error")}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusInternalServerError)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 500)
}

func TestProxyProcurementConversationMessages_Success_ReturnsMessagesJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		getConversationMessagesResp: &aipb.ConversationMessagesResponse{
			ConversationId: "5", RequestId: "77", Status: "REQUIREMENT_READY", Title: "買雞蛋",
			Records: []*aipb.ConversationMessage{{
				Id: "1-buyer", Role: "buyer", Text: "我要買雞蛋",
				RecordedThisTurn: []*aipb.RecordedField{{Key: "item", Label: "品項", Value: "雞蛋"}},
				CreatedAt:        "2026-08-16T00:00:00Z",
			}},
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations/5/messages", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["conversationId"] != "5" || data["title"] != "買雞蛋" {
		t.Fatalf("data = %v", data)
	}
	records, ok := data["records"].([]interface{})
	if !ok || len(records) != 1 {
		t.Fatalf("records = %v", data["records"])
	}
	record := records[0].(map[string]interface{})
	recordedThisTurn, ok := record["recordedThisTurn"].([]interface{})
	if !ok || len(recordedThisTurn) != 1 {
		t.Fatalf("recordedThisTurn = %v", record["recordedThisTurn"])
	}
	rf := recordedThisTurn[0].(map[string]interface{})
	if rf["key"] != "item" || rf["value"] != "雞蛋" {
		t.Fatalf("recordedThisTurn[0] = %v", rf)
	}
}

func TestProxyProcurementConversationMessages_DropsReasoning(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		getConversationMessagesResp: &aipb.ConversationMessagesResponse{
			ConversationId: "5", RequestId: "77", Status: "REQUIREMENT_READY", Title: "買雞蛋",
			Records: []*aipb.ConversationMessage{{
				Id: "1-assistant", Role: "assistant", Text: "已記下",
				CreatedAt: "2026-08-16T00:00:00Z",
			}},
		},
	}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations/5/messages", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	body := w.Body.String()
	if strings.Contains(body, "reasoning") {
		t.Fatalf("conversation messages 回應不得含 reasoning, got %s", body)
	}
}

func TestProxyProcurementConversationMessages_NotFound_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{getConversationMessagesErr: status.Error(codes.NotFound, "conversation not found")}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/app/buyers/me/procurement/conversations/999/messages", nil)
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 404)
}

func TestProxyProcurementConversationRename_Success_ReturnsNoDataKey(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	body := `{"title":"新標題","clientRequestId":"c-3"}`
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/app/buyers/me/procurement/conversations/5", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotRenameConversationReq.GetConversationId() != "5" || mock.gotRenameConversationReq.GetTitle() != "新標題" ||
		mock.gotRenameConversationReq.GetClientRequestId() != "c-3" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotRenameConversationReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	if _, present := respBody["data"]; present {
		t.Fatalf("data key should be absent (RPC returns google.protobuf.Empty): %v", respBody)
	}
}

func TestProxyProcurementConversationRename_InvalidTitle_Returns422(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		renameConversationErr: mustWithDetails(t, codes.FailedPrecondition, "標題需為 1-40 字且不含換行",
			&aipb.BusinessError{Reason: "CONVERSATION_TITLE_INVALID", Message: "標題需為 1-40 字且不含換行"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"title":"","clientRequestId":"c-3"}`
	req, _ := http.NewRequest(http.MethodPatch, "/api/v1/app/buyers/me/procurement/conversations/5", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusUnprocessableEntity)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 422)
	if respBody["reason"] != "CONVERSATION_TITLE_INVALID" {
		t.Fatalf("reason = %v, want CONVERSATION_TITLE_INVALID", respBody["reason"])
	}
}

func TestProxyProcurementConversationDelete_Success_ReturnsNoDataKey(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-4"}`
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/app/buyers/me/procurement/conversations/5", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotDeleteConversationReq.GetConversationId() != "5" || mock.gotDeleteConversationReq.GetClientRequestId() != "c-4" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotDeleteConversationReq)
	}
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	if _, present := respBody["data"]; present {
		t.Fatalf("data key should be absent (RPC returns google.protobuf.Empty): %v", respBody)
	}
}

func TestProxyProcurementConversationDelete_NotFound_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{deleteConversationErr: status.Error(codes.NotFound, "conversation not found")}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-4"}`
	req, _ := http.NewRequest(http.MethodDelete, "/api/v1/app/buyers/me/procurement/conversations/999", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 404)
}

func TestProxyProcurementRunCancel_Success_ReturnsCancelResponseJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		cancelRunResp: &aipb.CancelRunResponse{
			RunId: "5150", RunStatus: "CANCEL_REQUESTED", CancelDisposition: "REQUESTED", RunVersion: "6",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","expectedRunVersion":"5","reason":"buyer changed mind"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/cancel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotCancelRunReq.GetRunId() != "5150" || mock.gotCancelRunReq.GetClientRequestId() != "c-1" ||
		mock.gotCancelRunReq.GetExpectedRunVersion() != "5" || mock.gotCancelRunReq.GetReason() != "buyer changed mind" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotCancelRunReq)
	}

	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	for key, want := range map[string]any{
		"schemaVersion":     "hawkr.agent.cancel.v1",
		"runId":             "5150",
		"runStatus":         "CANCEL_REQUESTED",
		"cancelDisposition": "REQUESTED",
		"runVersion":        "6",
	} {
		got, present := data[key]
		if !present {
			t.Fatalf("對外 JSON 缺少 key %q；實際 keys=%v", key, data)
		}
		if got != want {
			t.Fatalf("data[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestProxyProcurementRunCancel_InvalidBody_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/cancel", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotCancelRunReq != nil {
		t.Fatalf("CancelRun should not have been called: %+v", mock.gotCancelRunReq)
	}
}

func TestProxyProcurementRunCancel_VersionConflict_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		cancelRunErr: mustWithDetails(t, codes.Aborted, "取消命令所依據的版本已過期，run 目前仍在執行中，請重新查詢最新狀態後再試",
			&aipb.BusinessError{Reason: "APPROVAL_STALE", Message: "取消命令所依據的版本已過期，run 目前仍在執行中，請重新查詢最新狀態後再試"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","expectedRunVersion":"3"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/cancel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "APPROVAL_STALE" {
		t.Fatalf("reason = %v, want APPROVAL_STALE", respBody["reason"])
	}
}

func TestProxyProcurementRunCancel_NotFound_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{cancelRunErr: status.Error(codes.NotFound, "request not found")}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","expectedRunVersion":"3"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/999999/cancel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 404)
}

func TestProxyProcurementRunCancel_AlreadyTerminalDisposition_Returns200(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		cancelRunResp: &aipb.CancelRunResponse{
			RunId: "5150", RunStatus: "SUCCEEDED", CancelDisposition: "TOO_LATE", RunVersion: "9",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","expectedRunVersion":"9"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/cancel", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["cancelDisposition"] != "TOO_LATE" || data["runStatus"] != "SUCCEEDED" {
		t.Fatalf("data = %v", data)
	}
}

func TestProxyProcurementRunResume_Success_ReturnsResumeResponseJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		resumeRunResp: &aipb.ResumeRunResponse{RunId: "6001", RunStatus: "QUEUED"},
	}
	h := newProcurementTestHolder(mock)

	body := `{"interruptId":"int-1","clientRequestId":"c-1","payload":{"optionId":"crop-a"}}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotResumeRunReq.GetInterruptId() != "int-1" || mock.gotResumeRunReq.GetClientRequestId() != "c-1" {
		t.Fatalf("request not mapped correctly: %+v", mock.gotResumeRunReq)
	}
	if !strings.Contains(string(mock.gotResumeRunReq.GetResponsePayload()), `"optionId":"crop-a"`) {
		t.Fatalf("responsePayload not forwarded correctly: %s", mock.gotResumeRunReq.GetResponsePayload())
	}

	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	data := respBody["data"].(map[string]interface{})
	if data["runId"] != "6001" || data["runStatus"] != "QUEUED" {
		t.Fatalf("data = %v", data)
	}
	if _, present := data["schemaVersion"]; present {
		t.Fatalf("resume response should not carry a fabricated schemaVersion (only cancel's response shape has one): %v", data)
	}
}

func TestProxyProcurementRunResume_InvalidBody_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/resume", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotResumeRunReq != nil {
		t.Fatalf("ResumeRun should not have been called: %+v", mock.gotResumeRunReq)
	}
}

func TestProxyProcurementRunResume_AlreadyResolved_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		resumeRunErr: mustWithDetails(t, codes.Aborted, "這個核准請求已被處理，請重新整理取得最新狀態",
			&aipb.BusinessError{Reason: "APPROVAL_ALREADY_RESOLVED", Message: "這個核准請求已被處理，請重新整理取得最新狀態"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"interruptId":"int-1","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "APPROVAL_ALREADY_RESOLVED" {
		t.Fatalf("reason = %v, want APPROVAL_ALREADY_RESOLVED", respBody["reason"])
	}
}

func TestProxyProcurementRunResume_InterruptNotFound_ReturnsMappedJSON(t *testing.T) {
	mock := &mockAIProcurementServiceClient{resumeRunErr: status.Error(codes.NotFound, "request not found")}
	h := newProcurementTestHolder(mock)

	body := `{"interruptId":"int-999","clientRequestId":"c-1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/runs/5150/resume", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusNotFound)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 404)
}

func TestProxyProcurementRfqPublicationPreview_Success_ReturnsExactJSONContract(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPublicationResp: &aipb.RfqPublicationPreviewResponse{
			ApprovalId: "7001", ApprovalVersion: "1",
			AudienceHash:            "audience-hash-1",
			SellerFacingPayloadHash: "sfp-hash-1",
			SellerFacingPayloadJson: "{}",
			ExpiresAt:               "2026-09-08T00:00:00Z",
			AudiencePolicyId:        "WISH_POOL_ALL_ACTIVE_SELLERS_V1",
			AudiencePolicyVersion:   "1",
			RecipientCount:          "0",
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"previewClientRequestId":"p-1","expectedRevisionId":"3","confirmedRfqSnapshotId":"901","audienceMode":"WISH_POOL"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	got := mock.gotPreviewRfqPublicationReq
	if got.GetRequestId() != "77" || got.GetPreviewClientRequestId() != "p-1" ||
		got.GetExpectedRevisionId() != "3" || got.GetConfirmedRfqSnapshotId() != "901" ||
		got.GetAudienceMode() != "WISH_POOL" {
		t.Fatalf("request not mapped correctly: %+v", got)
	}

	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	want := map[string]interface{}{
		"approvalId": "7001", "approvalVersion": "1",
		"audienceHash": "audience-hash-1", "sellerFacingPayloadHash": "sfp-hash-1",
		"sellerFacingPayloadJson": "{}", "expiresAt": "2026-09-08T00:00:00Z",
		"audiencePolicyId": "WISH_POOL_ALL_ACTIVE_SELLERS_V1", "audiencePolicyVersion": "1",
		"recipientCount": "0",
	}
	if data := respBody["data"]; !reflect.DeepEqual(data, want) {
		t.Fatalf("data = %#v, want %#v", data, want)
	}
}

func TestProxyProcurementRfqPublicationPreview_DirectRecipients_PassesOrderedSelection(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPublicationResp: &aipb.RfqPublicationPreviewResponse{ApprovalId: "7002", ApprovalVersion: "1"},
	}
	h := newProcurementTestHolder(mock)

	body := `{"previewClientRequestId":"p-2","expectedRevisionId":"3","confirmedRfqSnapshotId":"901",` +
		`"audienceMode":"DIRECT_RECIPIENTS","candidateSetId":"900","selectedCandidateIds":["22","11"],` +
		`"responseDeadline":"2026-09-15"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	got := mock.gotPreviewRfqPublicationReq
	if got.GetCandidateSetId() != "900" {
		t.Fatalf("candidateSetId = %q", got.GetCandidateSetId())
	}
	if ids := got.GetSelectedCandidateIds(); len(ids) != 2 || ids[0] != "22" || ids[1] != "11" {
		t.Fatalf("selectedCandidateIds = %v, want [22 11] in order", ids)
	}
	if got.ResponseDeadline == nil || got.GetResponseDeadline() != "2026-09-15" {
		t.Fatalf("responseDeadline = %v", got.ResponseDeadline)
	}
}

func TestProxyProcurementRfqPublicationPreview_OmittedResponseDeadline_StaysNil(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPublicationResp: &aipb.RfqPublicationPreviewResponse{ApprovalId: "7003", ApprovalVersion: "1"},
	}
	h := newProcurementTestHolder(mock)

	body := `{"previewClientRequestId":"p-3","expectedRevisionId":"3","confirmedRfqSnapshotId":"901","audienceMode":"WISH_POOL"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if mock.gotPreviewRfqPublicationReq.ResponseDeadline != nil {
		t.Fatalf("responseDeadline = %v, want nil", *mock.gotPreviewRfqPublicationReq.ResponseDeadline)
	}
}

func TestProxyProcurementRfqPublicationPreview_ApprovalStale_ReturnsConflict(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		previewRfqPublicationErr: mustWithDetails(t, codes.Aborted, "這份定案內容已不對應目前的需求，請重新確認",
			&aipb.BusinessError{Reason: "APPROVAL_STALE", Message: "這份定案內容已不對應目前的需求，請重新確認"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"previewClientRequestId":"p-4","expectedRevisionId":"3","confirmedRfqSnapshotId":"901","audienceMode":"WISH_POOL"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications/preview", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "APPROVAL_STALE" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqPublicationPreview_InvalidBody_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications/preview", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotPreviewRfqPublicationReq != nil {
		t.Fatalf("PreviewRfqPublication should not have been called: %+v", mock.gotPreviewRfqPublicationReq)
	}
}

func TestProxyProcurementRfqPublicationExecute_Success_ReturnsExactJSONContract(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		executeRfqPublicationResp: &aipb.RfqPublicationExecuteResponse{
			PublicationId:           "8001",
			PublicationVersion:      "1",
			State:                   "PUBLISHED",
			AudienceMode:            "WISH_POOL",
			AudienceHash:            "aud-hash",
			SellerFacingPayloadHash: "sfp-hash",
			PublishedAt:             "2026-09-09T00:00:00Z",
			Replayed:                false,
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","approvalId":"7001","approvalVersion":"1","expectedRevisionId":"3","payloadHash":"sfp-hash"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)

	got := mock.gotExecuteRfqPublicationReq
	if got.GetRequestId() != "77" || got.GetClientRequestId() != "c-1" ||
		got.GetApprovalId() != "7001" || got.GetApprovalVersion() != "1" ||
		got.GetExpectedRevisionId() != "3" || got.GetPayloadHash() != "sfp-hash" {
		t.Fatalf("request not mapped correctly: %+v", got)
	}

	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 0)
	want := map[string]interface{}{
		"publicationId": "8001", "publicationVersion": "1", "state": "PUBLISHED",
		"audienceMode": "WISH_POOL", "audienceHash": "aud-hash",
		"sellerFacingPayloadHash": "sfp-hash",
		"publishedAt":             "2026-09-09T00:00:00Z", "replayed": false,
	}
	if data := respBody["data"]; !reflect.DeepEqual(data, want) {
		t.Fatalf("data = %#v, want %#v", data, want)
	}
}

func TestProxyProcurementRfqPublicationExecute_Replayed_PassesThroughAsBool(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		executeRfqPublicationResp: &aipb.RfqPublicationExecuteResponse{
			PublicationId: "8001", PublicationVersion: "1", State: "PUBLISHED",
			AudienceMode: "WISH_POOL", AudienceHash: "aud-hash",
			SellerFacingPayloadHash: "sfp-hash", PublishedAt: "2026-09-09T00:00:00Z",
			Replayed: true,
		},
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-1","approvalId":"7001","approvalVersion":"1","expectedRevisionId":"3","payloadHash":"sfp-hash"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	respBody := parseJSON(t, w.Body)
	data, ok := respBody["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object: %#v", respBody["data"])
	}
	if replayed, ok := data["replayed"].(bool); !ok || replayed != true {
		t.Fatalf("replayed = %#v, want bool true", data["replayed"])
	}
}

func TestProxyProcurementRfqPublicationExecute_BodyRequestIdIgnored_PathWins(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		executeRfqPublicationResp: &aipb.RfqPublicationExecuteResponse{PublicationId: "8002", PublicationVersion: "1"},
	}
	h := newProcurementTestHolder(mock)

	body := `{"requestId":"999","clientRequestId":"c-2","approvalId":"7002","approvalVersion":"1"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusOK)
	if got := mock.gotExecuteRfqPublicationReq.GetRequestId(); got != "77" {
		t.Fatalf("requestId = %q, want %q (from URL path, body's requestId must be ignored)", got, "77")
	}
}

func TestProxyProcurementRfqPublicationExecute_ApprovalStale_ReturnsConflict(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		executeRfqPublicationErr: mustWithDetails(t, codes.Aborted, "這筆核准已不是最新版本，請重新預覽發布",
			&aipb.BusinessError{Reason: "APPROVAL_STALE", Message: "這筆核准已不是最新版本，請重新預覽發布"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-3","approvalId":"7003","approvalVersion":"1","expectedRevisionId":"3","payloadHash":"sfp-hash"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "APPROVAL_STALE" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqPublicationExecute_IdempotencyKeyConflict_ReturnsConflict(t *testing.T) {
	mock := &mockAIProcurementServiceClient{
		executeRfqPublicationErr: mustWithDetails(t, codes.Aborted, "同一個 clientRequestId 已用過不同的 approvalId",
			&aipb.BusinessError{Reason: "IDEMPOTENCY_KEY_CONFLICT", Message: "同一個 clientRequestId 已用過不同的 approvalId"}),
	}
	h := newProcurementTestHolder(mock)

	body := `{"clientRequestId":"c-4","approvalId":"7004","approvalVersion":"1","expectedRevisionId":"3","payloadHash":"sfp-hash"}`
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusConflict)
	respBody := parseJSON(t, w.Body)
	assertCode(t, respBody, 409)
	if respBody["reason"] != "IDEMPOTENCY_KEY_CONFLICT" {
		t.Fatalf("reason = %v", respBody["reason"])
	}
}

func TestProxyProcurementRfqPublicationExecute_InvalidBody_ReturnsBadRequestWithoutCallingRPC(t *testing.T) {
	mock := &mockAIProcurementServiceClient{}
	h := newProcurementTestHolder(mock)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/app/buyers/me/procurement/requests/77/rfq-publications", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", "99")
	w := httptest.NewRecorder()
	setupProcurementUnaryRouter(h).ServeHTTP(w, req)

	assertHTTPStatus(t, w, http.StatusBadRequest)
	if mock.gotExecuteRfqPublicationReq != nil {
		t.Fatalf("ExecuteRfqPublication should not have been called: %+v", mock.gotExecuteRfqPublicationReq)
	}
}
