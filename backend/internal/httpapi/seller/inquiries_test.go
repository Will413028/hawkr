package seller

import (
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/reflect/protoreflect"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/seller/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

var inquiryID = uuid.MustParse("0192f1c0-0000-7000-8000-0000000000a1")

func sellerInquiryFixture() model.SellerInquiry {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return model.SellerInquiry{
		CaseInquiry: model.CaseInquiry{
			ID: inquiryID, CaseID: uuid.MustParse("0192f1c0-0000-7000-8000-0000000000c1"), StoreID: 4401, Kind: model.InquiryKindRecords,
			Message: "請提供進貨紀錄", CreatedAt: at, CreatedByAdminID: 7771, DueAt: at.Add(72 * time.Hour),
		},
		CanReply: true,
	}
}

func TestEnumsOfTheSellerInquiryMatchTheModule(t *testing.T) {
	for name, got := range map[string][]string{
		"InquiryKind":         rpc.EnumMismatches[pb.InquiryKind](model.InquiryKindValues()),
		"InquiryState":        rpc.EnumMismatches[pb.InquiryState](model.InquiryStateValues()),
		"InquiryReplyChannel": rpc.EnumMismatches[pb.InquiryReplyChannel](model.InquiryReplyChannelValues()),
	} {
		if len(got) != 0 {
			t.Errorf("%s：%v", name, got)
		}
	}
}

// 賣家只看到說明、期限、狀態與回覆；案件、店家與管理員的識別不輸出，撤回不帶原因與管理員，回覆附件只帶數量。
func TestSellerInquiryResponseOmitsInternalFields(t *testing.T) {
	open := toSellerInquiry(sellerInquiryFixture())
	if open.GetState() != pb.InquiryState_INQUIRY_STATE_OPEN || !open.GetCanReply() || open.GetMessage() != "請提供進貨紀錄" ||
		open.GetReplyDeadline() == nil || !open.GetReplyDeadline().GetStartTime().AsTime().Equal(sellerInquiryFixture().CreatedAt) ||
		open.GetReplyAttachmentMax() != model.MaxEvidence {
		t.Fatalf("open=%v", open)
	}

	replied := sellerInquiryFixture()
	replied.CanReply = false
	replied.AttachmentCount = 2
	replied.Reply = &model.InquiryReply{
		Body: "已附上", Channel: model.InquiryChannelSellerCenter, ReceivedAt: replied.CreatedAt.Add(time.Hour), RecordedAt: replied.CreatedAt.Add(time.Hour),
		RepliedByAccountID: 9, Evidence: []model.Evidence{{AssetID: "a", ObjectKey: "secret-key", Position: 0}},
	}
	out := toSellerInquiry(replied)
	if out.GetState() != pb.InquiryState_INQUIRY_STATE_REPLIED || out.GetCanReply() || out.GetReplyDeadline() != nil ||
		out.GetReply().GetChannel() != pb.InquiryReplyChannel_INQUIRY_REPLY_CHANNEL_SELLER_CENTER || out.GetReply().GetAttachmentCount() != 2 {
		t.Fatalf("out=%v", out)
	}
	if text := out.String(); strings.Contains(text, "secret-key") || strings.Contains(text, "7771") || strings.Contains(text, "4401") || strings.Contains(text, "0192f1c0-0000-7000-8000-0000000000c1") {
		t.Fatalf("內部欄位出現在回應：%s", text)
	}

	withdrawn := sellerInquiryFixture()
	withdrawn.CanReply = false
	withdrawn.Withdrawn = &model.InquiryWithdrawal{At: withdrawn.CreatedAt.Add(time.Hour), AdminID: 7772, Cause: model.InquiryWithdrawCaseClosed}
	w := toSellerInquiry(withdrawn)
	if w.GetState() != pb.InquiryState_INQUIRY_STATE_WITHDRAWN || w.GetWithdrawTime() == nil || w.GetReplyDeadline() != nil || strings.Contains(w.String(), "7772") {
		t.Fatalf("withdrawn=%v", w)
	}

	adminRecorded := sellerInquiryFixture()
	adminRecorded.Reply = &model.InquiryReply{Body: "電話中說明", Channel: model.InquiryChannelPhone, ReceivedAt: adminRecorded.CreatedAt, RecordedAt: adminRecorded.CreatedAt, RecordedByAdminID: 7773}
	if r := toSellerInquiry(adminRecorded).GetReply(); r.GetChannel() != pb.InquiryReplyChannel_INQUIRY_REPLY_CHANNEL_PHONE || strings.Contains(r.String(), "7773") {
		t.Fatalf("reply=%v", r)
	}
}

// 帳號是登入的呼叫者；待回覆數與分頁位置照轉；page token 綁帳號與篩選。
func TestInquiryMethodsScopeByTheCallingAccount(t *testing.T) {
	v := sellerInquiryFixture()
	m := &kycModule{inquiry: v, inquiryPage: model.SellerInquiryPage{
		Items: []model.SellerInquiry{v}, Next: &model.InquiryCursor{CreatedAt: v.CreatedAt, ID: v.ID}, PendingReplyCount: 3,
	}}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	ctx := accountContext(7)
	list, err := h.ListMyInquiries(ctx, connect.NewRequest(&pb.ListMyInquiriesRequest{PageSize: 5, State: pb.InquiryState_INQUIRY_STATE_OPEN}))
	if err != nil || len(list.Msg.GetInquiries()) != 1 || list.Msg.GetPendingReplyCount() != 3 || m.inquiryListIn.AccountID != 7 || m.inquiryListIn.PageSize != 5 ||
		m.inquiryListIn.State == nil || *m.inquiryListIn.State != model.InquiryStateOpen || list.Msg.GetNextPageToken() == "" {
		t.Fatalf("resp=%v err=%v input=%+v", list, err, m.inquiryListIn)
	}
	next := &pb.ListMyInquiriesRequest{PageToken: list.Msg.GetNextPageToken(), State: pb.InquiryState_INQUIRY_STATE_OPEN}
	if _, err := h.ListMyInquiries(ctx, connect.NewRequest(next)); err != nil || m.inquiryListIn.After == nil || m.inquiryListIn.After.ID != v.ID {
		t.Fatalf("err=%v input=%+v", err, m.inquiryListIn)
	}
	if _, err := h.ListMyInquiries(accountContext(8), connect.NewRequest(next)); err == nil {
		t.Fatal("別的帳號的 page token 被接受")
	}
	if _, err := h.ListMyInquiries(ctx, connect.NewRequest(&pb.ListMyInquiriesRequest{PageToken: list.Msg.GetNextPageToken()})); err == nil {
		t.Fatal("換了篩選仍接受上一組的 page token")
	}
	if _, err := h.GetMyInquiry(ctx, connect.NewRequest(&pb.GetMyInquiryRequest{InquiryId: v.ID.String()})); err != nil || m.getAccount != 7 || m.getID != v.ID {
		t.Fatalf("err=%v account=%d id=%v", err, m.getAccount, m.getID)
	}
	resp, err := h.ReplyToInquiry(ctx, connect.NewRequest(&pb.ReplyToInquiryRequest{InquiryId: v.ID.String(), Body: "回覆", EvidenceStagedTokens: []string{"t1"}}))
	if err != nil || resp.Msg.GetInquiry().GetId() != v.ID.String() {
		t.Fatalf("resp=%v err=%v", resp, err)
	}
	if want := (model.ReplyToInquiryInput{AccountID: 7, InquiryID: v.ID, Body: "回覆", EvidenceTokens: []string{"t1"}}); m.replyIn.AccountID != want.AccountID ||
		m.replyIn.InquiryID != want.InquiryID || m.replyIn.Body != want.Body || len(m.replyIn.EvidenceTokens) != 1 {
		t.Fatalf("input=%+v", m.replyIn)
	}
}

func TestInquiryMethodsRejectMalformedIDsAndMissingPrincipals(t *testing.T) {
	m := &kycModule{inquiry: sellerInquiryFixture()}
	h := mustApp(t, m, nil, rpc.Images{}, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}))
	ctx := accountContext(7)
	if _, err := h.GetMyInquiry(ctx, connect.NewRequest(&pb.GetMyInquiryRequest{InquiryId: "x"})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
	if _, err := h.ReplyToInquiry(ctx, connect.NewRequest(&pb.ReplyToInquiryRequest{InquiryId: "x", Body: "a"})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("err=%v", err)
	}
	bg := t.Context()
	if _, err := h.ListMyInquiries(bg, connect.NewRequest(&pb.ListMyInquiriesRequest{})); err == nil {
		t.Fatal("沒有登入的呼叫者被接受")
	}
	if _, err := h.GetMyInquiry(bg, connect.NewRequest(&pb.GetMyInquiryRequest{InquiryId: inquiryID.String()})); err == nil {
		t.Fatal("沒有登入的呼叫者被接受")
	}
	if _, err := h.ReplyToInquiry(bg, connect.NewRequest(&pb.ReplyToInquiryRequest{InquiryId: inquiryID.String(), Body: "a"})); err == nil {
		t.Fatal("沒有登入的呼叫者被接受")
	}
}

func TestSellerInquiryFieldPathsExistInTheRequests(t *testing.T) {
	for name, c := range map[string]struct {
		paths rpc.FieldPaths
		req   interface{ ProtoReflect() protoreflect.Message }
	}{
		"list":  {inquiryListPaths, &pb.ListMyInquiriesRequest{}},
		"reply": {inquiryReplyPaths, &pb.ReplyToInquiryRequest{}},
	} {
		if missing := rpc.CheckFieldPaths(c.req.ProtoReflect().Descriptor(), c.paths); len(missing) > 0 {
			t.Errorf("%s：不存在的路徑 %v", name, missing)
		}
	}
}

// 後台看到賣家中心的回覆：管道是 SELLER_CENTER，回覆的帳號有值，管理員記下的欄位不設；管理員記下的回覆相反。
func TestAdminInquiryShowsWhoReplied(t *testing.T) {
	at := sellerInquiryFixture().CreatedAt
	seller := model.AdminInquiry{CaseInquiry: sellerInquiryFixture().CaseInquiry}
	seller.Reply = &model.InquiryReply{Body: "好", Channel: model.InquiryChannelSellerCenter, ReceivedAt: at, RecordedAt: at, RepliedByAccountID: 9}
	got := toCaseInquiry(seller).GetReply()
	if got.GetChannel() != adminpb.CaseInquiryReplyChannel_CASE_INQUIRY_REPLY_CHANNEL_SELLER_CENTER || got.GetRepliedByAccountId() != "9" || got.GetRecordedByAdminId() != "" {
		t.Fatalf("reply=%v", got)
	}
	admin := model.AdminInquiry{CaseInquiry: sellerInquiryFixture().CaseInquiry}
	admin.Reply = &model.InquiryReply{Body: "好", Channel: model.InquiryChannelPhone, ReceivedAt: at, RecordedAt: at, RecordedByAdminID: 8}
	got = toCaseInquiry(admin).GetReply()
	if got.GetRecordedByAdminId() != "8" || got.GetRepliedByAccountId() != "" {
		t.Fatalf("reply=%v", got)
	}
}
