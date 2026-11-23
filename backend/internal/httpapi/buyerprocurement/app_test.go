package buyerprocurement

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"uuid"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/procurement/v1alpha1/procurementv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/procurement"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

const (
	accountID = int64(7)
	buyerID   = int64(4242)
	goodID    = "11111111-1111-4111-8111-111111111111"
)

var ownedRun = uuid.MustParse("22222222-2222-4222-8222-222222222222")

type buyerLookup struct{}

func (buyerLookup) CurrentBuyerID(_ context.Context, account int64) (int64, error) {
	switch account {
	case accountID:
		return buyerID, nil
	case accountID + 1:
		return buyerID + 1, nil
	}
	return 0, errors.New("沒有買家")
}

// fakeModule 的擁有者檢查與真實模組相同：不是自己的 run、中斷、對話回 NOT_FOUND。
type fakeModule struct {
	submitted []procurement.SubmitTurnInput
	submitErr error
	resumeErr error
	searchErr error
	toggleErr error
	toggles   []procurement.ToggleSelectionInput
	answers   []procurement.SubmitAnswerInput
	rewinds   []procurement.RewindInput
	directErr error
	events    []procurement.RunEvent
	snapshot  procurement.RuntimeSnapshot
	outcome   procurement.CancelRunOutcome

	requestSnapshot procurement.RequestSnapshot
	messages        procurement.ConversationMessages
	page            procurement.ConversationPage
	search          procurement.CandidateSearch
	pageSizes       []int32
	pageTokens      []string
	renamed         []string
	deleted         int
	listErr         error
	renameErr       error

	rfq rfqFake
}

func (f *fakeModule) SubmitTurn(_ context.Context, in procurement.SubmitTurnInput) (procurement.TurnSubmitted, error) {
	f.submitted = append(f.submitted, in)
	if f.submitErr != nil {
		return procurement.TurnSubmitted{}, f.submitErr
	}
	return procurement.TurnSubmitted{RequestID: ownedRun, ConversationID: ownedRun, RunID: ownedRun}, nil
}

func (f *fakeModule) WatchRun(_ context.Context, buyer int64, runID uuid.UUID, _ string, send func(procurement.RunEvent) error) error {
	if buyer != buyerID || runID != ownedRun {
		return apperr.NotFound("找不到這個 run")
	}
	for _, e := range f.events {
		if err := send(e); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeModule) CancelRun(_ context.Context, buyer int64, runID uuid.UUID) (procurement.CancelRunOutcome, error) {
	if buyer != buyerID || runID != ownedRun {
		return procurement.CancelRunOutcome{}, apperr.NotFound("找不到這個 run")
	}
	return f.outcome, nil
}

func (f *fakeModule) ResumeRun(context.Context, procurement.ResumeRunInput) (procurement.ResumeRunResult, error) {
	if f.resumeErr != nil {
		return procurement.ResumeRunResult{}, f.resumeErr
	}
	return procurement.ResumeRunResult{RunID: ownedRun, Status: "QUEUED"}, nil
}

func (f *fakeModule) StartCandidateSearch(context.Context, procurement.StartCandidateSearchInput) (procurement.CandidateSearchStarted, error) {
	if f.searchErr != nil {
		return procurement.CandidateSearchStarted{}, f.searchErr
	}
	return procurement.CandidateSearchStarted{RunID: ownedRun, CandidateSetID: ownedRun, Status: "QUEUED"}, nil
}

func (f *fakeModule) ToggleSelection(_ context.Context, in procurement.ToggleSelectionInput) error {
	f.toggles = append(f.toggles, in)
	return f.toggleErr
}

const storedTurnResult = `{"requestId":"r","conversationId":"c","revisionId":"v","slots":{"item":"蘋果"},"reply":"好的","question":{"slotKey":"SLOT_KEY_DELIVERY_CITY","prompt":"送到哪裡","canSkip":false},"textFallback":"好的","messageId":"m","createTime":"2026-11-23T00:00:00Z"}`

func (f *fakeModule) SubmitAnswer(_ context.Context, in procurement.SubmitAnswerInput) ([]byte, error) {
	f.answers = append(f.answers, in)
	if f.directErr != nil {
		return nil, f.directErr
	}
	return []byte(storedTurnResult), nil
}

func (f *fakeModule) Rewind(_ context.Context, in procurement.RewindInput) ([]byte, error) {
	f.rewinds = append(f.rewinds, in)
	if f.directErr != nil {
		return nil, f.directErr
	}
	return []byte(storedTurnResult), nil
}

func (f *fakeModule) GetRuntimeSnapshot(_ context.Context, buyer int64, conv uuid.UUID) (procurement.RuntimeSnapshot, error) {
	if buyer != buyerID || conv != ownedRun {
		return procurement.RuntimeSnapshot{}, apperr.NotFound("找不到這個對話")
	}
	return f.snapshot, nil
}

// cardStub 給每家店一張有名字的卡片，但店 404 沒有卡片（從沒公開過）。
type cardStub struct{}

func (cardStub) StoreCards(_ context.Context, ids []int64) ([]seller.StoreCard, error) {
	var out []seller.StoreCard
	for _, id := range ids {
		if id == 404 {
			continue
		}
		name := fmt.Sprintf("店%d", id)
		out = append(out, seller.StoreCard{StoreID: id, StoreCode: id + 1000, Name: &name, Status: seller.CardOpen})
	}
	return out, nil
}

func setup(t *testing.T, m Module) procurementv1alpha1connect.BuyerProcurementAppServiceClient {
	t.Helper()
	images, err := rpc.NewImages("https://media.example.com")
	if err != nil {
		t.Fatal(err)
	}
	app := NewApp(m, buyerLookup{}, cardStub{}, images, slog.New(slog.DiscardHandler))
	path, h := procurementv1alpha1connect.NewBuyerProcurementAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Test-Account"); v != "" {
			acct, _ := strconv.ParseInt(v, 10, 64)
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: acct}))
		}
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return procurementv1alpha1connect.NewBuyerProcurementAppServiceClient(srv.Client(), srv.URL)
}

func as[T any](req *connect.Request[T], account int64) *connect.Request[T] {
	req.Header().Set("X-Test-Account", strconv.FormatInt(account, 10))
	return req
}

func violationFields(t *testing.T, err error) []string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	var fields []string
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatal(derr)
		}
		if br, ok := v.(*errdetails.BadRequest); ok {
			for _, fv := range br.GetFieldViolations() {
				fields = append(fields, fv.GetField())
			}
		}
	}
	return fields
}

func reasonOf(t *testing.T, err error) string {
	t.Helper()
	var ce *connect.Error
	if !errors.As(err, &ce) {
		t.Fatalf("不是 connect 錯誤：%v", err)
	}
	for _, d := range ce.Details() {
		v, derr := d.Value()
		if derr != nil {
			t.Fatal(derr)
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}

func TestRequestsWithoutAnAccountAreUnauthenticated(t *testing.T) {
	c := setup(t, &fakeModule{})
	_, err := c.SubmitTurn(t.Context(), connect.NewRequest(&pb.SubmitTurnRequest{Message: "x", ClientRequestId: goodID}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("SubmitTurn 未登入 = %v", err)
	}
	_, err = c.CancelRun(t.Context(), connect.NewRequest(&pb.CancelRunRequest{RunId: ownedRun.String()}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("CancelRun 未登入 = %v", err)
	}
}

func TestSubmitTurnValidatesAtTheEntryAndNamesTheField(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	for name, tc := range map[string]struct {
		req   *pb.SubmitTurnRequest
		field string
	}{
		"空訊息":                         {&pb.SubmitTurnRequest{ClientRequestId: goodID}, "message"},
		"沒有 client_request_id":        {&pb.SubmitTurnRequest{Message: "x"}, "client_request_id"},
		"client_request_id 不是 UUIDv4": {&pb.SubmitTurnRequest{Message: "x", ClientRequestId: "abc"}, "client_request_id"},
		"request_id 不是 UUIDv4":        {&pb.SubmitTurnRequest{Message: "x", ClientRequestId: goodID, RequestId: "abc"}, "request_id"},
	} {
		_, err := c.SubmitTurn(t.Context(), as(connect.NewRequest(tc.req), accountID))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，要 INVALID_ARGUMENT", name, err)
			continue
		}
		if got := violationFields(t, err); len(got) != 1 || got[0] != tc.field {
			t.Errorf("%s：欄位 = %v，要 [%s]", name, got, tc.field)
		}
	}
	if len(m.submitted) != 0 {
		t.Fatalf("被拒絕的請求不得到達模組：%+v", m.submitted)
	}
}

func TestSubmitTurnPassesTheBuyerAndMapsModuleErrors(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	res, err := c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{Message: "要蘋果", ClientRequestId: goodID}), accountID))
	if err != nil || res.Msg.GetRunId() != ownedRun.String() || res.Msg.GetConversationId() == "" || res.Msg.GetRequestId() == "" {
		t.Fatalf("回應 = %v, %v", res, err)
	}
	if len(m.submitted) != 1 || m.submitted[0].BuyerID != buyerID || m.submitted[0].RequestID != nil || m.submitted[0].Message != "要蘋果" {
		t.Fatalf("模組收到 %+v，要買家 %d 而不是帳號 %d", m.submitted, buyerID, accountID)
	}

	m.submitErr = apperr.InvalidInputOn([]string{"Message"}, "訊息不能超過 2000 個字元")
	_, err = c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{Message: "x", ClientRequestId: goodID}), accountID))
	if got := violationFields(t, err); len(got) != 1 || got[0] != "message" {
		t.Errorf("模組的 Message 欄位違規 = %v，要 [message]", got)
	}
	m.submitErr = apperr.InvalidInputOn([]string{"client_request_id"}, "同一個 client_request_id 的內容不同")
	_, err = c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{Message: "x", ClientRequestId: goodID}), accountID))
	if got := violationFields(t, err); len(got) != 1 || got[0] != "client_request_id" {
		t.Errorf("重送內容不同 = %v，要 [client_request_id]", got)
	}
	m.submitErr = apperr.FailedPrecondition("進行中").WithReason(apperr.ReasonProcurementRunInProgress)
	_, err = c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{Message: "x", ClientRequestId: goodID}), accountID))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || reasonOf(t, err) != apperr.ReasonProcurementRunInProgress.String() {
		t.Errorf("同需求進行中 = %v", err)
	}
	m.submitErr = apperr.NotFound("找不到這張需求")
	_, err = c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{Message: "x", ClientRequestId: goodID, RequestId: goodID}), accountID))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("別人的需求 = %v，要 NOT_FOUND", err)
	}
}

func TestResumeRunMapsNestedAnswerFieldPaths(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	req := func() *pb.ResumeRunRequest {
		return &pb.ResumeRunRequest{InterruptId: ownedRun.String(), ClientRequestId: goodID, Response: &pb.ResumeRunRequest_Clarification{Clarification: &pb.ClarificationResponse{OptionId: "a"}}}
	}
	for name, tc := range map[string]struct {
		fields []string
		want   []string
	}{
		"兩個都要":  {[]string{"OptionID", "FreeText"}, []string{"clarification.option_id", "clarification.free_text"}},
		"只有文字":  {[]string{"FreeText"}, []string{"clarification.free_text"}},
		"選項不在卡": {[]string{"OptionID"}, []string{"clarification.option_id"}},
	} {
		m.resumeErr = apperr.InvalidInputOn(tc.fields, "回答不合")
		_, err := c.ResumeRun(t.Context(), as(connect.NewRequest(req()), accountID))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%s：%v", name, err)
		}
		got := violationFields(t, err)
		if fmt.Sprint(got) != fmt.Sprint(tc.want) {
			t.Errorf("%s：路徑 = %v，要 %v", name, got, tc.want)
		}
	}
	m.resumeErr = nil
	res, err := c.ResumeRun(t.Context(), as(connect.NewRequest(req()), accountID))
	if err != nil || res.Msg.GetRunId() != ownedRun.String() || res.Msg.GetRunStatus() != pb.RunStatus_RUN_STATUS_QUEUED {
		t.Fatalf("ResumeRun = %v, %v", res, err)
	}
	_, err = c.ResumeRun(t.Context(), as(connect.NewRequest(&pb.ResumeRunRequest{InterruptId: ownedRun.String(), ClientRequestId: goodID}), accountID))
	if got := violationFields(t, err); len(got) != 1 || got[0] != "clarification" {
		t.Errorf("沒有回答 = %v，要 [clarification]", got)
	}
}

func TestOtherBuyersGetNotFound(t *testing.T) {
	c := setup(t, &fakeModule{})
	other := accountID + 1
	_, err := c.CancelRun(t.Context(), as(connect.NewRequest(&pb.CancelRunRequest{RunId: ownedRun.String()}), other))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("CancelRun = %v", err)
	}
	_, err = c.GetRuntimeSnapshot(t.Context(), as(connect.NewRequest(&pb.GetRuntimeSnapshotRequest{ConversationId: ownedRun.String()}), other))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("GetRuntimeSnapshot = %v", err)
	}
	st, err := c.WatchRun(t.Context(), as(connect.NewRequest(&pb.WatchRunRequest{RunId: ownedRun.String()}), other))
	if err == nil {
		for st.Receive() {
		}
		err = st.Err()
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("WatchRun = %v", err)
	}
}

func TestWatchRunConvertsPayloadsAndEndsWithFinished(t *testing.T) {
	at := time.Date(2026, 11, 23, 1, 2, 3, 0, time.UTC)
	m := &fakeModule{events: []procurement.RunEvent{
		{Cursor: "1", OccurredAt: at, Payload: []byte(`{"runAccepted":{"runId":"r","conversationId":"c"}}`)},
		{Cursor: "2", OccurredAt: at, Payload: []byte(`{"finished":{"status":"RUN_STATUS_FAILED","failureReason":"RUN_FAILURE_REASON_MODEL_UNAVAILABLE"}}`)},
	}}
	c := setup(t, m)
	st, err := c.WatchRun(t.Context(), as(connect.NewRequest(&pb.WatchRunRequest{RunId: ownedRun.String()}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	var got []*pb.WatchRunResponse
	for st.Receive() {
		got = append(got, st.Msg())
	}
	if st.Err() != nil || len(got) != 2 {
		t.Fatalf("frames = %v, %v", got, st.Err())
	}
	if got[0].GetRunAccepted().GetConversationId() != "c" || got[0].GetCursor() != "1" || !got[0].GetEventTime().AsTime().Equal(at) {
		t.Errorf("第一筆 = %v", got[0])
	}
	f := got[1].GetFinished()
	if f.GetStatus() != pb.RunStatus_RUN_STATUS_FAILED || f.GetFailureReason() != pb.RunFailureReason_RUN_FAILURE_REASON_MODEL_UNAVAILABLE || got[1].GetCursor() != "2" {
		t.Errorf("finished = %v", got[1])
	}
}

func TestEventToProtoRejectsUnknownOrEmptyPayloads(t *testing.T) {
	for name, payload := range map[string]string{
		"不認得的欄位":  `{"finished":{"status":"RUN_STATUS_FAILED","localizedMessage":"x"}}`,
		"沒有事件":    `{}`,
		"不是 JSON": `nope`,
	} {
		if _, err := eventToProto(procurement.RunEvent{Cursor: "1", Payload: []byte(payload)}); err == nil {
			t.Errorf("%s：要回錯誤", name)
		}
	}
}

func TestCancelRunMapsOutcome(t *testing.T) {
	m := &fakeModule{outcome: procurement.CancelRunOutcome{Status: "CANCEL_REQUESTED", Disposition: procurement.CancelDispositionRequested}}
	c := setup(t, m)
	res, err := c.CancelRun(t.Context(), as(connect.NewRequest(&pb.CancelRunRequest{RunId: ownedRun.String()}), accountID))
	if err != nil || res.Msg.GetRunStatus() != pb.RunStatus_RUN_STATUS_CANCEL_REQUESTED || res.Msg.GetCancelDisposition() != pb.CancelDisposition_CANCEL_DISPOSITION_REQUESTED {
		t.Fatalf("CancelRun = %v, %v", res, err)
	}
	m.outcome = procurement.CancelRunOutcome{}
	_, err = c.CancelRun(t.Context(), as(connect.NewRequest(&pb.CancelRunRequest{RunId: ownedRun.String()}), accountID))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("空的狀態不能當有效值傳出去：%v", err)
	}
	_, err = c.CancelRun(t.Context(), as(connect.NewRequest(&pb.CancelRunRequest{RunId: "x"}), accountID))
	if got := violationFields(t, err); len(got) != 1 || got[0] != "run_id" {
		t.Errorf("run_id 格式 = %v", got)
	}
}

func TestStartCandidateSearchMapsErrorsAndResult(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	res, err := c.StartCandidateSearch(t.Context(), as(connect.NewRequest(&pb.StartCandidateSearchRequest{RequestId: ownedRun.String(), ClientRequestId: goodID}), accountID))
	if err != nil || res.Msg.GetRunId() != ownedRun.String() || res.Msg.GetCandidateSetId() == "" || res.Msg.GetStatus() != pb.RunStatus_RUN_STATUS_QUEUED {
		t.Fatalf("結果 = %v, %v", res, err)
	}
	m.searchErr = apperr.InvalidInputOn([]string{"client_request_id"}, "內容不同")
	_, err = c.StartCandidateSearch(t.Context(), as(connect.NewRequest(&pb.StartCandidateSearchRequest{RequestId: ownedRun.String(), ClientRequestId: goodID}), accountID))
	if got := violationFields(t, err); len(got) != 1 || got[0] != "client_request_id" {
		t.Errorf("欄位 = %v", got)
	}
	m.searchErr = apperr.FailedPrecondition("還沒齊備").WithReason(apperr.ReasonRequirementNotReady)
	_, err = c.StartCandidateSearch(t.Context(), as(connect.NewRequest(&pb.StartCandidateSearchRequest{RequestId: ownedRun.String(), ClientRequestId: goodID}), accountID))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition || reasonOf(t, err) != apperr.ReasonRequirementNotReady.String() {
		t.Errorf("需求未齊備 = %v", err)
	}
}

func TestSnapshotMapsEveryFieldAndRejectsUnspecified(t *testing.T) {
	active := ownedRun
	now := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	m := &fakeModule{snapshot: procurement.RuntimeSnapshot{
		ConversationID: ownedRun, RequestID: ownedRun, Status: "CLARIFYING", Title: "蘋果",
		Messages: []procurement.ConversationMessage{{ID: ownedRun, Role: "USER", Text: "要蘋果", CreateTime: now,
			RecordedThisTurn: []procurement.RecordedField{{Key: "ITEM", Label: "品項", Value: "蘋果"}}}},
		Runs:        []procurement.RunSummary{{RunID: ownedRun, RunType: "TURN", Status: "WAITING_FOR_INPUT", LastCursor: "3"}},
		ActiveRunID: &active,
		OpenConfirmation: &procurement.OpenConfirmation{InterruptID: ownedRun, Kind: "CLARIFICATION", Processing: true,
			Question: procurement.ClarifyQuestion{SlotKey: "ITEM_DISAMBIGUATION", Prompt: "哪一種？", AllowFreeText: true,
				Options: []procurement.QuestionOption{{OptionID: "o1", Label: "A"}}, UnitOptions: []procurement.QuestionOption{{OptionID: "u1", Label: "kg"}}}},
		PublishedRfq: &procurement.PublishedRfq{PublicationID: ownedRun, Version: 2, PublishTime: now, RecipientCount: 3, QueuedCount: 2},
	}}
	c := setup(t, m)
	res, err := c.GetRuntimeSnapshot(t.Context(), as(connect.NewRequest(&pb.GetRuntimeSnapshotRequest{ConversationId: ownedRun.String()}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	s := res.Msg
	q := s.GetOpenConfirmation().GetQuestion()
	if s.GetStatus() != pb.RequestStatus_REQUEST_STATUS_CLARIFYING || s.GetTitle() != "蘋果" || s.GetActiveRunId() != active.String() ||
		s.GetMessages()[0].GetRole() != pb.ConversationRole_CONVERSATION_ROLE_USER || s.GetMessages()[0].GetRecordedThisTurn()[0].GetKey() != pb.SlotKey_SLOT_KEY_ITEM ||
		s.GetRuns()[0].GetRunType() != pb.RunType_RUN_TYPE_TURN || s.GetRuns()[0].GetLastCursor() != "3" || s.GetRuns()[0].GetStatus() != pb.RunStatus_RUN_STATUS_WAITING_FOR_INPUT ||
		!s.GetOpenConfirmation().GetProcessing() || s.GetOpenConfirmation().GetKind() != pb.InterruptKind_INTERRUPT_KIND_CLARIFICATION ||
		q.GetSlotKey() != pb.SlotKey_SLOT_KEY_ITEM_DISAMBIGUATION || !q.GetAllowFreeText() || q.GetOptions()[0].GetOptionId() != "o1" || q.GetUnitOptions()[0].GetLabel() != "kg" ||
		s.GetPublishedRfq().GetVersion() != "2" || s.GetPublishedRfq().GetRecipientCount() != 3 || s.GetPublishedRfq().GetQueuedCount() != 2 {
		t.Fatalf("快照 = %v", s)
	}
	m.snapshot.Status = ""
	_, err = c.GetRuntimeSnapshot(t.Context(), as(connect.NewRequest(&pb.GetRuntimeSnapshotRequest{ConversationId: ownedRun.String()}), accountID))
	if connect.CodeOf(err) != connect.CodeInternal {
		t.Errorf("空的需求狀態 = %v，要 INTERNAL", err)
	}
}

func TestFieldPathsExistInTheRequestMessages(t *testing.T) {
	for name, tc := range map[string]struct {
		paths rpc.FieldPaths
		miss  []string
	}{
		"submit": {submitPaths, rpc.CheckFieldPaths((&pb.SubmitTurnRequest{}).ProtoReflect().Descriptor(), submitPaths)},
		"search": {searchPaths, rpc.CheckFieldPaths((&pb.StartCandidateSearchRequest{}).ProtoReflect().Descriptor(), searchPaths)},
		"resume": {resumePaths, rpc.CheckFieldPaths((&pb.ResumeRunRequest{}).ProtoReflect().Descriptor(), resumePaths)},
		"watch":  {watchPaths, rpc.CheckFieldPaths((&pb.WatchRunRequest{}).ProtoReflect().Descriptor(), watchPaths)},
		"list":   {listPaths, rpc.CheckFieldPaths((&pb.ListConversationsRequest{}).ProtoReflect().Descriptor(), listPaths)},
		"rename": {renamePaths, rpc.CheckFieldPaths((&pb.RenameConversationRequest{}).ProtoReflect().Descriptor(), renamePaths)},
	} {
		if len(tc.miss) != 0 {
			t.Errorf("%s：請求訊息沒有這些路徑：%v", name, tc.miss)
		}
	}
}

func TestSubmitTurnAcceptsAServerIssuedUUIDv7RequestID(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	id := uuid.NewV7()
	if _, err := c.SubmitTurn(t.Context(), as(connect.NewRequest(&pb.SubmitTurnRequest{Message: "x", ClientRequestId: goodID, RequestId: id.String()}), accountID)); err != nil {
		t.Fatalf("UUIDv7 的 request_id = %v，要通過", err)
	}
	if len(m.submitted) != 1 || m.submitted[0].RequestID == nil || *m.submitted[0].RequestID != id {
		t.Fatalf("模組收到 %+v，要帶 request_id %s", m.submitted, id)
	}
}

func owned(buyer int64, id uuid.UUID, what string) error {
	if buyer != buyerID || id != ownedRun {
		return apperr.NotFound("找不到這個%s", what)
	}
	return nil
}

func (f *fakeModule) GetSnapshot(_ context.Context, buyer int64, id uuid.UUID) (procurement.RequestSnapshot, error) {
	if err := owned(buyer, id, "需求"); err != nil {
		return procurement.RequestSnapshot{}, err
	}
	return f.requestSnapshot, nil
}

func (f *fakeModule) GetConversationMessages(_ context.Context, buyer int64, id uuid.UUID) (procurement.ConversationMessages, error) {
	if err := owned(buyer, id, "對話"); err != nil {
		return procurement.ConversationMessages{}, err
	}
	return f.messages, nil
}

func (f *fakeModule) ListConversations(_ context.Context, _ int64, size int32, token string) (procurement.ConversationPage, error) {
	f.pageSizes, f.pageTokens = append(f.pageSizes, size), append(f.pageTokens, token)
	return f.page, f.listErr
}

func (f *fakeModule) RenameConversation(_ context.Context, buyer int64, id uuid.UUID, title string) error {
	if err := owned(buyer, id, "對話"); err != nil {
		return err
	}
	f.renamed = append(f.renamed, title)
	return f.renameErr
}

func (f *fakeModule) DeleteConversation(_ context.Context, buyer int64, id uuid.UUID) error {
	if err := owned(buyer, id, "對話"); err != nil {
		return err
	}
	f.deleted++
	return nil
}

func (f *fakeModule) GetCandidateSearch(_ context.Context, buyer int64, request, run uuid.UUID, _ uitext.Locale) (procurement.CandidateSearch, error) {
	if buyer != buyerID || request != ownedRun || run != ownedRun {
		return procurement.CandidateSearch{}, apperr.NotFound("找不到這次候選搜尋")
	}
	return f.search, nil
}

func TestToggleSelectionValidatesIDsAndNamesTheField(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	for name, tc := range map[string]struct {
		req   *pb.ToggleSelectionRequest
		field string
	}{
		"沒有 request_id":        {&pb.ToggleSelectionRequest{CandidateSetId: goodID, CandidateId: goodID}, "request_id"},
		"candidate_set_id 不合法": {&pb.ToggleSelectionRequest{RequestId: goodID, CandidateSetId: "abc", CandidateId: goodID}, "candidate_set_id"},
		"沒有 candidate_id":      {&pb.ToggleSelectionRequest{RequestId: goodID, CandidateSetId: goodID}, "candidate_id"},
	} {
		_, err := c.ToggleSelection(t.Context(), as(connect.NewRequest(tc.req), accountID))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s：%v，要 INVALID_ARGUMENT", name, err)
			continue
		}
		if got := violationFields(t, err); len(got) != 1 || got[0] != tc.field {
			t.Errorf("%s：欄位 = %v，要 [%s]", name, got, tc.field)
		}
	}
	if len(m.toggles) != 0 {
		t.Fatalf("被拒絕的請求不得到達模組：%+v", m.toggles)
	}
}

func TestToggleSelectionPassesTheBuyerAndTargetStateAndMapsErrors(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	req := &pb.ToggleSelectionRequest{RequestId: goodID, CandidateSetId: goodID, CandidateId: goodID, Selected: true}
	res, err := c.ToggleSelection(t.Context(), as(connect.NewRequest(req), accountID))
	if err != nil || res.Msg.GetRequestId() != goodID || res.Msg.GetCandidateSetId() != goodID || res.Msg.GetCandidateId() != goodID || !res.Msg.GetSelected() {
		t.Fatalf("回應 = %v, %v", res, err)
	}
	if len(m.toggles) != 1 || m.toggles[0].BuyerID != buyerID || !m.toggles[0].Selected || m.toggles[0].RequestID.String() != goodID {
		t.Fatalf("模組收到 %+v", m.toggles)
	}
	m.toggleErr = apperr.VersionConflict("過時").WithMetadata("conflictTarget", "candidate_set")
	_, err = c.ToggleSelection(t.Context(), as(connect.NewRequest(req), accountID))
	if connect.CodeOf(err) != connect.CodeAborted || reasonOf(t, err) != apperr.ReasonVersionConflict.String() {
		t.Errorf("過時的候選集 = %v，要 ABORTED／VERSION_CONFLICT", err)
	}
	m.toggleErr = apperr.NotFound("找不到")
	_, err = c.ToggleSelection(t.Context(), as(connect.NewRequest(req), accountID))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("別人的需求 = %v，要 NOT_FOUND", err)
	}
}

func TestSubmitAnswerAndRewindValidateAtTheEntryAndNameTheField(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	slot := pb.SlotKey_SLOT_KEY_DELIVERY_CITY
	for name, tc := range map[string]struct {
		req   *pb.SubmitAnswerRequest
		field string
	}{
		"沒有 request_id":            {&pb.SubmitAnswerRequest{SlotKey: slot, ExpectedRevisionId: goodID, ClientRequestId: goodID}, "request_id"},
		"沒有 slot_key":              {&pb.SubmitAnswerRequest{RequestId: goodID, ExpectedRevisionId: goodID, ClientRequestId: goodID}, "slot_key"},
		"expected_revision_id 不合法": {&pb.SubmitAnswerRequest{RequestId: goodID, SlotKey: slot, ExpectedRevisionId: "abc", ClientRequestId: goodID}, "expected_revision_id"},
		"沒有 expected_revision_id":  {&pb.SubmitAnswerRequest{RequestId: goodID, SlotKey: slot, ClientRequestId: goodID}, "expected_revision_id"},
		"client_request_id 不是 v4":  {&pb.SubmitAnswerRequest{RequestId: goodID, SlotKey: slot, ExpectedRevisionId: goodID, ClientRequestId: "abc"}, "client_request_id"},
	} {
		_, err := c.SubmitAnswer(t.Context(), as(connect.NewRequest(tc.req), accountID))
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("SubmitAnswer %s：%v，要 INVALID_ARGUMENT", name, err)
			continue
		}
		if got := violationFields(t, err); len(got) != 1 || got[0] != tc.field {
			t.Errorf("SubmitAnswer %s：欄位 = %v，要 [%s]", name, got, tc.field)
		}
		rw := &pb.RewindRequest{RequestId: tc.req.GetRequestId(), SlotKey: tc.req.GetSlotKey(), ExpectedRevisionId: tc.req.GetExpectedRevisionId(), ClientRequestId: tc.req.GetClientRequestId()}
		_, err = c.Rewind(t.Context(), as(connect.NewRequest(rw), accountID))
		if got := violationFields(t, err); len(got) != 1 || got[0] != tc.field {
			t.Errorf("Rewind %s：欄位 = %v，要 [%s]", name, got, tc.field)
		}
	}
	if len(m.answers)+len(m.rewinds) != 0 {
		t.Fatalf("被拒絕的請求不得到達模組：%+v %+v", m.answers, m.rewinds)
	}
}

func TestSubmitAnswerAndRewindPassTheCallerAndReturnTheStoredTurnResult(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	res, err := c.SubmitAnswer(t.Context(), as(connect.NewRequest(&pb.SubmitAnswerRequest{
		RequestId: goodID, SlotKey: pb.SlotKey_SLOT_KEY_QUANTITY, FreeText: "20", UnitOptionId: "unit-kg", ExpectedRevisionId: goodID, ClientRequestId: goodID,
	}), accountID))
	if err != nil {
		t.Fatal(err)
	}
	r := res.Msg.GetResult()
	if r.GetReply() != "好的" || r.GetSlots().GetItem() != "蘋果" || r.GetQuestion().GetSlotKey() != pb.SlotKey_SLOT_KEY_DELIVERY_CITY {
		t.Fatalf("結果 = %v", r)
	}
	if len(m.answers) != 1 || m.answers[0].BuyerID != buyerID || m.answers[0].Slot != "QUANTITY" || m.answers[0].FreeText != "20" || m.answers[0].UnitOptionID != "unit-kg" || m.answers[0].ExpectedRevisionID.String() != goodID {
		t.Fatalf("模組收到 %+v", m.answers)
	}
	rw, err := c.Rewind(t.Context(), as(connect.NewRequest(&pb.RewindRequest{RequestId: goodID, SlotKey: pb.SlotKey_SLOT_KEY_UNIT, ExpectedRevisionId: goodID, ClientRequestId: goodID}), accountID))
	if err != nil || rw.Msg.GetResult().GetReply() != "好的" {
		t.Fatalf("Rewind = %v %v", rw, err)
	}
	if len(m.rewinds) != 1 || m.rewinds[0].BuyerID != buyerID || m.rewinds[0].Slot != "UNIT" {
		t.Fatalf("模組收到 %+v", m.rewinds)
	}
}

func TestSubmitAnswerAndRewindMapModuleErrors(t *testing.T) {
	m := &fakeModule{}
	c := setup(t, m)
	answer := &pb.SubmitAnswerRequest{RequestId: goodID, SlotKey: pb.SlotKey_SLOT_KEY_ITEM, FreeText: "蘋果", ExpectedRevisionId: goodID, ClientRequestId: goodID}
	rewind := &pb.RewindRequest{RequestId: goodID, SlotKey: pb.SlotKey_SLOT_KEY_ITEM, ExpectedRevisionId: goodID, ClientRequestId: goodID}
	for name, tc := range map[string]struct {
		err    error
		code   connect.Code
		reason string
		fields []string
	}{
		"品項文字對不到唯一分類": {apperr.InvalidInputOn([]string{"FreeText"}, "請改用一般訊息"), connect.CodeInvalidArgument, "", []string{"free_text"}},
		"選項不在問題上":     {apperr.InvalidInputOn([]string{"OptionID"}, "x"), connect.CodeInvalidArgument, "", []string{"option_id"}},
		"單位選項":        {apperr.InvalidInputOn([]string{"UnitOptionID"}, "x"), connect.CodeInvalidArgument, "", []string{"unit_option_id"}},
		"略過":          {apperr.InvalidInputOn([]string{"Skip"}, "x"), connect.CodeInvalidArgument, "", []string{"skip"}},
		"欄位不能用":       {apperr.InvalidInputOn([]string{"SlotKey"}, "x"), connect.CodeInvalidArgument, "", []string{"slot_key"}},
		"重送內容不同":      {apperr.InvalidInputOn([]string{"client_request_id"}, "x"), connect.CodeInvalidArgument, "", []string{"client_request_id"}},
		"版本過時":        {apperr.VersionConflict("過時").WithMetadata("conflictTarget", "request_revision"), connect.CodeAborted, apperr.ReasonVersionConflict.String(), nil},
		"待回覆的中斷":      {apperr.FailedPrecondition("有中斷").WithReason(apperr.ReasonApprovalPending), connect.CodeFailedPrecondition, apperr.ReasonApprovalPending.String(), nil},
		"進行中的 run":    {apperr.FailedPrecondition("進行中").WithReason(apperr.ReasonProcurementRunInProgress), connect.CodeFailedPrecondition, apperr.ReasonProcurementRunInProgress.String(), nil},
		"沒有可倒回的內容":    {apperr.FailedPrecondition("沒有可倒回的內容"), connect.CodeFailedPrecondition, "", nil},
		"別人的需求":       {apperr.NotFound("找不到"), connect.CodeNotFound, "", nil},
	} {
		m.directErr = tc.err
		_, errA := c.SubmitAnswer(t.Context(), as(connect.NewRequest(answer), accountID))
		_, errR := c.Rewind(t.Context(), as(connect.NewRequest(rewind), accountID))
		for rpcName, err := range map[string]error{"SubmitAnswer": errA, "Rewind": errR} {
			if connect.CodeOf(err) != tc.code || reasonOf(t, err) != tc.reason {
				t.Errorf("%s %s：%v，要 %v／%q", rpcName, name, err, tc.code, tc.reason)
			}
			if len(tc.fields) > 0 && (rpcName == "SubmitAnswer" || tc.fields[0] == "slot_key" || tc.fields[0] == "client_request_id") { // 倒回只有這兩個欄位違規
				if got := violationFields(t, err); fmt.Sprint(got) != fmt.Sprint(tc.fields) {
					t.Errorf("%s %s：欄位 = %v，要 %v", rpcName, name, got, tc.fields)
				}
			}
		}
	}
}
