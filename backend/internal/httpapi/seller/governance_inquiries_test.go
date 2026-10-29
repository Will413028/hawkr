package seller

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1/sellerv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// inquiryModule 記下詢問指令收到的輸入，回傳設定好的結果。
type inquiryModule struct {
	GovernanceModule
	create    model.CreateCaseInquiryInput
	list      model.CaseInquiryListInput
	reply     model.RecordCaseInquiryReplyInput
	withdraw  model.WithdrawCaseInquiryInput
	createRes model.CreateCaseInquiryResult
	page      model.AdminInquiryPage
	replyRes  model.RecordCaseInquiryReplyResult
	withRes   model.WithdrawCaseInquiryResult
	err       error
	calls     int
}

func (m *inquiryModule) CreateCaseInquiry(_ context.Context, in model.CreateCaseInquiryInput) (model.CreateCaseInquiryResult, error) {
	m.create, m.calls = in, m.calls+1
	return m.createRes, m.err
}

func (m *inquiryModule) CaseInquiries(_ context.Context, in model.CaseInquiryListInput) (model.AdminInquiryPage, error) {
	m.list = in
	return m.page, m.err
}

func (m *inquiryModule) RecordCaseInquiryReply(_ context.Context, in model.RecordCaseInquiryReplyInput) (model.RecordCaseInquiryReplyResult, error) {
	m.reply, m.calls = in, m.calls+1
	return m.replyRes, m.err
}

func (m *inquiryModule) WithdrawCaseInquiry(_ context.Context, in model.WithdrawCaseInquiryInput) (model.WithdrawCaseInquiryResult, error) {
	m.withdraw, m.calls = in, m.calls+1
	return m.withRes, m.err
}

func inquiryFixture(state model.InquiryState) model.AdminInquiry {
	i := model.CaseInquiry{
		ID: uuid.NewV7(), CaseID: uuid.NewV7(), StoreID: 12, Kind: model.InquiryKindRecords, Message: "請提供批次", CreatedAt: governanceNow, CreatedByAdminID: 7,
		DueAt: governanceNow.Add(model.InquiryDefaultDue),
	}
	switch state {
	case model.InquiryStateOpen:
	case model.InquiryStateReplied:
		i.Reply = &model.InquiryReply{
			Body: "已附上", Channel: model.InquiryChannelMessaging, ReceivedAt: governanceNow.Add(time.Hour), RecordedAt: governanceNow.Add(2 * time.Hour), RecordedByAdminID: 8,
			Evidence: []model.Evidence{{AssetID: "asset-secret-1", ObjectKey: "key-secret-1", Position: 0}, {AssetID: "asset-secret-2", ObjectKey: "key-secret-2", Position: 1}},
		}
	case model.InquiryStateWithdrawn:
		i.Withdrawn = &model.InquiryWithdrawal{At: governanceNow.Add(time.Hour), AdminID: 9, Cause: model.InquiryWithdrawCaseClosed}
	}
	return model.AdminInquiry{CaseInquiry: i, Availability: i.Availability()}
}

// 詢問的列舉與模組的值一一對應。
func TestInquiryEnumsMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"CaseInquiryKind":          rpc.EnumMismatches[adminpb.CaseInquiryKind](model.InquiryKindValues()),
		"CaseInquiryState":         rpc.EnumMismatches[adminpb.CaseInquiryState](model.InquiryStateValues()),
		"CaseInquiryReplyChannel":  rpc.EnumMismatches[adminpb.CaseInquiryReplyChannel](model.InquiryReplyChannelValues()),
		"CaseInquiryWithdrawCause": rpc.EnumMismatches[adminpb.CaseInquiryWithdrawCause](model.InquiryWithdrawCauseValues()),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

// 欄位對應表指向請求真的有的欄位。
func TestInquiryFieldPathsExistInTheRequests(t *testing.T) {
	for name, c := range map[string]struct {
		paths rpc.FieldPaths
		req   interface{ ProtoReflect() protoreflect.Message }
	}{
		"create": {createInquiryPaths, &adminpb.CreateCaseInquiryRequest{}},
		"reply":  {recordInquiryReplyPaths, &adminpb.RecordCaseInquiryReplyRequest{}},
	} {
		if missing := rpc.CheckFieldPaths(c.req.ProtoReflect().Descriptor(), c.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
}

// 詢問只輸出契約的欄位：期限只在還沒結束時有；回覆與附件的檢視網址（不帶 asset ID 與 object key）；撤回的原因；can_* 取自 Availability，不輸出原因。
func TestCaseInquiryCarriesStateDueAndAvailabilityWithoutReasons(t *testing.T) {
	open := toCaseInquiry(inquiryFixture(model.InquiryStateOpen))
	if open.GetState() != adminpb.CaseInquiryState_CASE_INQUIRY_STATE_OPEN ||
		// 賣家的回覆期限：開始是建立時間，期限是存下的 due_at（不是營運的審閱政策）。
		!open.GetReplyDeadline().GetStartTime().AsTime().Equal(governanceNow) || !open.GetReplyDeadline().GetDueTime().AsTime().Equal(governanceNow.Add(model.InquiryDefaultDue)) || !open.GetCanRecordReply() || !open.GetCanWithdraw() || open.GetReply() != nil ||
		open.GetKind() != adminpb.CaseInquiryKind_CASE_INQUIRY_KIND_RECORDS || open.GetStoreId() != "12" || open.GetCreatedByAdminId() != "7" || open.GetMessage() != "請提供批次" {
		t.Fatalf("open=%v", open)
	}
	repliedFixture := inquiryFixture(model.InquiryStateReplied)
	replied := toCaseInquiry(repliedFixture)
	if replied.GetState() != adminpb.CaseInquiryState_CASE_INQUIRY_STATE_REPLIED || replied.GetReplyDeadline() != nil || replied.GetCanRecordReply() || replied.GetCanWithdraw() ||
		replied.GetReply().GetChannel() != adminpb.CaseInquiryReplyChannel_CASE_INQUIRY_REPLY_CHANNEL_MESSAGING || replied.GetReply().GetBody() != "已附上" ||
		replied.GetReply().GetRecordedByAdminId() != "8" || len(replied.GetReply().GetEvidence()) != 2 {
		t.Fatalf("replied=%v", replied)
	}
	for n, e := range replied.GetReply().GetEvidence() {
		if want := "/api/files/admin/seller/case_inquiries/" + repliedFixture.ID.String() + "/evidence/" + string(rune('0'+n)); e.GetViewUri() != want || e.GetPosition() != int32(n) {
			t.Errorf("evidence[%d]=%v，預期 %s", n, e, want)
		}
	}
	if raw := protojson.Format(replied); strings.Contains(raw, "asset-secret") || strings.Contains(raw, "key-secret") {
		t.Fatalf("附件的 asset ID 與 object key 不得出現在回應：%s", raw)
	}
	withdrawn := toCaseInquiry(inquiryFixture(model.InquiryStateWithdrawn))
	if withdrawn.GetState() != adminpb.CaseInquiryState_CASE_INQUIRY_STATE_WITHDRAWN || withdrawn.GetReplyDeadline() != nil || withdrawn.GetCanRecordReply() || withdrawn.GetCanWithdraw() ||
		withdrawn.GetWithdrawCause() != adminpb.CaseInquiryWithdrawCause_CASE_INQUIRY_WITHDRAW_CAUSE_CASE_CLOSED || withdrawn.GetWithdrawnByAdminId() != "9" || withdrawn.GetWithdrawTime() == nil {
		t.Fatalf("withdrawn=%v", withdrawn)
	}
	// 原因的句子不出現在任何回應裡：can_* 之外沒有欄位說「為什麼不能」。
	raw := protojson.Format(withdrawn) + protojson.Format(replied)
	for _, b := range model.InquiryBlockValues() {
		if strings.Contains(raw, b.Text().Text) {
			t.Errorf("回應不得帶原因的句子 %q：%s", b.Text().Text, raw)
		}
	}
}

// 案件帶「等賣家」、進行中詢問的 ID 與 can_create_inquiry；沒有詢問的案件不帶 ID。
func TestCaseCarriesTheAwaitingSellerFacts(t *testing.T) {
	id := uuid.NewV7()
	waiting := caseFixture(model.CaseFacts{Case: model.Case{OpenInquiryID: &id}})
	got := toCase(waiting)
	if !got.GetAwaitingSeller() || got.GetOpenInquiryId() != id.String() || got.GetCanCreateInquiry() || !got.GetCanDecide() {
		t.Fatalf("waiting=%v", got)
	}
	idle := toCase(caseFixture(model.CaseFacts{}))
	if idle.GetAwaitingSeller() || idle.GetOpenInquiryId() != "" || !idle.GetCanCreateInquiry() {
		t.Fatalf("idle=%v", idle)
	}
	closedAt := governanceNow.Add(time.Hour)
	reason := model.CaseClosedDecided
	closed := toCase(caseFixture(model.CaseFacts{Case: model.Case{ClosedAt: &closedAt, CloseReason: &reason}}))
	if closed.GetAwaitingSeller() || closed.GetCanCreateInquiry() {
		t.Fatalf("closed=%v", closed)
	}
	purged := toCase(caseFixture(model.CaseFacts{StorePurged: true}))
	if purged.GetCanCreateInquiry() {
		t.Fatalf("店家個資已清除：%v", purged)
	}
}

// 發詢問：以呼叫的管理員、解析 request_id（UUIDv4）與選填的期限交給模組；case_id 不是 UUID、request_id 不是 UUIDv4 不呼叫模組；模組回的欄位錯誤對應到請求的路徑。
func TestCreateCaseInquiryUsesTheCallingAdminAndMapsTheInput(t *testing.T) {
	inquiry := inquiryFixture(model.InquiryStateOpen)
	m := &inquiryModule{createRes: model.CreateCaseInquiryResult{Inquiry: inquiry, Case: caseFixture(model.CaseFacts{})}}
	h := NewGovernance(m, rpc.Images{})
	caseID := uuid.NewV7()
	req := uuid.NewV4()
	due := governanceNow.Add(5 * 24 * time.Hour)
	got, err := h.CreateCaseInquiry(adminCtx(42), connect.NewRequest(&adminpb.CreateCaseInquiryRequest{
		CaseId: caseID.String(), Kind: adminpb.CaseInquiryKind_CASE_INQUIRY_KIND_EXPLANATION, Message: "請說明", DueTime: timestamppb.New(due), RequestId: req.String(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	in := m.create
	if in.CaseID != caseID || in.AdminID != 42 || in.Kind != model.InquiryKindExplanation || in.Message != "請說明" || in.DueAt == nil || !in.DueAt.Equal(due) || in.RequestID == nil || *in.RequestID != req ||
		got.Msg.GetInquiry().GetId() != inquiry.ID.String() || got.Msg.GetCase() == nil {
		t.Fatalf("in=%+v", in)
	}
	// 選填都不給。
	if _, err := h.CreateCaseInquiry(adminCtx(42), connect.NewRequest(&adminpb.CreateCaseInquiryRequest{CaseId: caseID.String(), Kind: adminpb.CaseInquiryKind_CASE_INQUIRY_KIND_RECORDS, Message: "x"})); err != nil {
		t.Fatal(err)
	}
	if m.create.DueAt != nil || m.create.RequestID != nil {
		t.Fatalf("in=%+v", m.create)
	}
	calls := m.calls
	for name, r := range map[string]*adminpb.CreateCaseInquiryRequest{
		"case_id 不是 UUID":     {CaseId: "x", Kind: adminpb.CaseInquiryKind_CASE_INQUIRY_KIND_RECORDS, Message: "x"},
		"request_id 不是 UUID":  {CaseId: caseID.String(), Message: "x", RequestId: "x"},
		"request_id 是 UUIDv7": {CaseId: caseID.String(), Message: "x", RequestId: uuid.NewV7().String()},
	} {
		_, err := h.CreateCaseInquiry(adminCtx(42), connect.NewRequest(r))
		t.Run(name, func(t *testing.T) { wantInvalidArgument(t, err) })
	}
	if m.calls != calls {
		t.Fatal("格式不對的請求不得呼叫模組")
	}
	if _, err := h.CreateCaseInquiry(context.Background(), connect.NewRequest(&adminpb.CreateCaseInquiryRequest{CaseId: caseID.String()})); err == nil || m.calls != calls {
		t.Fatal("不是管理員不得呼叫模組")
	}
	// 模組的欄位錯誤指到請求的路徑（經 interceptor 轉成 BadRequest；欄位沒有對應就是 handler 的 bug，interceptor 會記 error）。
	for module, path := range map[string]string{"DueAt": "due_time", "Kind": "kind", "Message": "message", "RequestID": "request_id"} {
		m.err = apperr.InvalidInputOn([]string{module}, "x")
		var logs strings.Builder
		call := rpc.NewInterceptor(slog.New(slog.NewTextHandler(&logs, nil))).WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			return h.CreateCaseInquiry(ctx, connect.NewRequest(&adminpb.CreateCaseInquiryRequest{CaseId: caseID.String()}))
		})
		_, err := call(adminCtx(42), fakeRequest{procedure: sellerv1alpha1connect.SellerGovernanceAdminServiceCreateCaseInquiryProcedure})
		if got := reportErrFields(t, err); !slices.Equal(got, []string{path}) || strings.Contains(logs.String(), "欄位沒有附給 client") {
			t.Errorf("%s：BadRequest 的欄位 %v，要 [%s]；log=%s", module, got, path, logs.String())
		}
	}
}

// 詢問清單的 page token 綁案件：換一個案件從第一頁開始；每頁筆數交給模組。
func TestListCaseInquiriesBindsThePageTokenToTheCase(t *testing.T) {
	cursor := &model.InquiryCursor{CreatedAt: governanceNow, ID: uuid.NewV7()}
	m := &inquiryModule{page: model.AdminInquiryPage{Items: []model.AdminInquiry{inquiryFixture(model.InquiryStateOpen)}, Next: cursor}}
	h := NewGovernance(m, rpc.Images{})
	caseID := uuid.NewV7()
	req := &adminpb.ListCaseInquiriesRequest{CaseId: caseID.String(), PageSize: 2}
	got, err := h.ListCaseInquiries(adminCtx(1), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	if m.list.CaseID != caseID || m.list.PageSize != 2 || m.list.After != nil || len(got.Msg.GetInquiries()) != 1 || got.Msg.GetNextPageToken() == "" {
		t.Fatalf("list=%+v", m.list)
	}
	req.PageToken = got.Msg.GetNextPageToken()
	if _, err := h.ListCaseInquiries(adminCtx(1), connect.NewRequest(req)); err != nil || m.list.After == nil || *m.list.After != *cursor {
		t.Fatalf("after=%+v err=%v", m.list.After, err)
	}
	changed := proto.Clone(req).(*adminpb.ListCaseInquiriesRequest)
	changed.CaseId = uuid.NewV7().String()
	_, err = h.ListCaseInquiries(adminCtx(1), connect.NewRequest(changed))
	wantInvalidArgument(t, err)
	_, err = h.ListCaseInquiries(adminCtx(1), connect.NewRequest(&adminpb.ListCaseInquiriesRequest{CaseId: "x"}))
	wantInvalidArgument(t, err)
}

// 記下回覆與撤回：以呼叫的管理員、解析 id；沒帶收到的時間交給模組判斷（零值）；詢問不存在回 Connect code 的預設文字（不帶伺服器句子），案件不存在仍是自己的句子。
func TestRecordAndWithdrawInquiryUseTheCallingAdmin(t *testing.T) {
	inquiry := inquiryFixture(model.InquiryStateReplied)
	m := &inquiryModule{replyRes: model.RecordCaseInquiryReplyResult{Inquiry: inquiry, Case: caseFixture(model.CaseFacts{}), AlreadyRecorded: true}, withRes: model.WithdrawCaseInquiryResult{Inquiry: inquiry, Case: caseFixture(model.CaseFacts{}), AlreadyWithdrawn: true}}
	h := NewGovernance(m, rpc.Images{})
	id := uuid.NewV7()
	received := governanceNow.Add(-time.Hour)
	got, err := h.RecordCaseInquiryReply(adminCtx(43), connect.NewRequest(&adminpb.RecordCaseInquiryReplyRequest{
		Id: id.String(), Channel: adminpb.CaseInquiryReplyChannel_CASE_INQUIRY_REPLY_CHANNEL_EMAIL, ReceiveTime: timestamppb.New(received), Body: "回覆", EvidenceTokens: []string{"a", "b"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if in := m.reply; in.InquiryID != id || in.AdminID != 43 || in.Channel != model.InquiryChannelEmail || !in.ReceivedAt.Equal(received) || in.Body != "回覆" || !slices.Equal(in.EvidenceTokens, []string{"a", "b"}) || !got.Msg.GetAlreadyRecorded() {
		t.Fatalf("in=%+v", in)
	}
	if _, err := h.RecordCaseInquiryReply(adminCtx(43), connect.NewRequest(&adminpb.RecordCaseInquiryReplyRequest{Id: id.String()})); err != nil || !m.reply.ReceivedAt.IsZero() {
		t.Fatalf("沒帶收到的時間：in=%+v err=%v", m.reply, err)
	}
	w, err := h.WithdrawCaseInquiry(adminCtx(44), connect.NewRequest(&adminpb.WithdrawCaseInquiryRequest{Id: id.String()}))
	if err != nil || m.withdraw != (model.WithdrawCaseInquiryInput{InquiryID: id, AdminID: 44}) || !w.Msg.GetAlreadyWithdrawn() {
		t.Fatalf("withdraw=%+v err=%v", m.withdraw, err)
	}
	for _, r := range []func() error{
		func() error {
			_, err := h.RecordCaseInquiryReply(adminCtx(43), connect.NewRequest(&adminpb.RecordCaseInquiryReplyRequest{Id: "x"}))
			return err
		},
		func() error {
			_, err := h.WithdrawCaseInquiry(adminCtx(43), connect.NewRequest(&adminpb.WithdrawCaseInquiryRequest{Id: "x"}))
			return err
		},
	} {
		wantInvalidArgument(t, r())
	}
	// handler 不改寫模組的 NotFound（詢問不存在的預設文字在模組邊界給）：模組給的句子原樣成為 LocalizedMessage。
	localized := func(err error) []string {
		var ce *connect.Error
		if !errors.As(err, &ce) {
			t.Fatalf("不是 connect 錯誤：%v", err)
		}
		var out []string
		for _, d := range ce.Details() {
			v, e := d.Value()
			if e != nil {
				t.Fatal(e)
			}
			if lm, ok := v.(*errdetails.LocalizedMessage); ok {
				out = append(out, lm.GetMessage())
			}
		}
		return out
	}
	withdraw := func(err error) error {
		m.err = err
		call := rpc.NewInterceptor(slog.New(slog.DiscardHandler)).WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
			return h.WithdrawCaseInquiry(ctx, connect.NewRequest(&adminpb.WithdrawCaseInquiryRequest{Id: id.String()}))
		})
		_, got := call(adminCtx(43), fakeRequest{procedure: sellerv1alpha1connect.SellerGovernanceAdminServiceWithdrawCaseInquiryProcedure})
		return got
	}
	if err := withdraw(apperr.NotFound("%s", "找不到這個案件。")); connect.CodeOf(err) != connect.CodeNotFound || !slices.Equal(localized(err), []string{"找不到這個案件。"}) {
		t.Fatalf("有句子的 NotFound：localized=%v", localized(err))
	}
}

// ListCases 的 awaiting_seller 交給模組，並綁定 page token；和已結案的組合由模組拒絕。
func TestListCasesAwaitingSellerReachesTheModuleAndBindsThePageToken(t *testing.T) {
	cursor := &model.CaseCursor{OpenedAt: governanceNow, ID: uuid.NewV7()}
	c := caseFixture(model.CaseFacts{})
	m := &governanceModule{casePage: model.AdminCasePage{Items: []model.AdminCase{c}, Next: cursor}}
	h := NewGovernance(m, rpc.Images{})
	req := &adminpb.ListCasesRequest{AwaitingSeller: true, PageSize: 2}
	got, err := h.ListCases(adminCtx(1), connect.NewRequest(req))
	if err != nil || !m.caseList.AwaitingSeller {
		t.Fatalf("in=%+v err=%v", m.caseList, err)
	}
	req.PageToken = got.Msg.GetNextPageToken()
	if _, err := h.ListCases(adminCtx(1), connect.NewRequest(req)); err != nil || m.caseList.After == nil {
		t.Fatalf("after=%+v err=%v", m.caseList.After, err)
	}
	changed := proto.Clone(req).(*adminpb.ListCasesRequest)
	changed.AwaitingSeller = false
	_, err = h.ListCases(adminCtx(1), connect.NewRequest(changed))
	wantInvalidArgument(t, err)
}

// 詢問附件的端點：以詢問 ID 與位置定址，交給模組的是 EvidenceInquiry。
func TestInquiryEvidenceEndpointReadsByInquiryID(t *testing.T) {
	inquiry := uuid.NewV7()
	m := &evidenceModule{content: model.FileContent{Data: []byte("PDF"), ContentType: "application/pdf"}}
	path := "/api/files/admin/seller/case_inquiries/" + inquiry.String() + "/evidence/1"
	rec := serveEvidence(m, nil, adminCtx(42), path)
	if rec.Code != 200 || rec.Body.String() != "PDF" || m.in != (model.ReadEvidenceInput{AdminID: 42, Kind: model.EvidenceInquiry, RefID: inquiry, Position: 1}) {
		t.Fatalf("code=%d in=%+v", rec.Code, m.in)
	}
	if rec := serveEvidence(&evidenceModule{}, nil, context.Background(), path); rec.Code != 401 {
		t.Fatalf("visitor code=%d", rec.Code)
	}
	for _, bad := range []string{"/api/files/admin/seller/case_inquiries/x/evidence/0", "/api/files/admin/seller/case_inquiries/" + inquiry.String() + "/evidence/-1"} {
		n := &evidenceModule{}
		if rec := serveEvidence(n, nil, adminCtx(42), bad); rec.Code != 404 || n.calls != 0 {
			t.Fatalf("%s：code=%d", bad, rec.Code)
		}
	}
}
