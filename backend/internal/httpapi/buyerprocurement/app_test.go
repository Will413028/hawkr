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
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
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
	events    []procurement.RunEvent
	snapshot  procurement.RuntimeSnapshot
	outcome   procurement.CancelRunOutcome
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

func (f *fakeModule) GetRuntimeSnapshot(_ context.Context, buyer int64, conv uuid.UUID) (procurement.RuntimeSnapshot, error) {
	if buyer != buyerID || conv != ownedRun {
		return procurement.RuntimeSnapshot{}, apperr.NotFound("找不到這個對話")
	}
	return f.snapshot, nil
}

func setup(t *testing.T, m Module) procurementv1alpha1connect.BuyerProcurementAppServiceClient {
	t.Helper()
	app := NewApp(m, buyerLookup{}, slog.New(slog.DiscardHandler))
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
		PublishedRfq: &procurement.PublishedRfq{PublicationID: ownedRun, Version: 2, PublishTime: now, RecipientCount: 3, QueuedCount: 2, ConfirmedRfqSnapshotID: ownedRun},
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
	} {
		if len(tc.miss) != 0 {
			t.Errorf("%s：請求訊息沒有這些路徑：%v", name, tc.miss)
		}
	}
}
