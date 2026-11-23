package moderation

import (
	"context"
	"strings"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/moderation/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	sellermod "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

const (
	callerBuyer = 7  // fakeBuyers 把登入的帳號對到這個買家
	callerStore = 11 // 測試裡呼叫者此刻的店
	otherBuyer  = 99
	otherStore  = 12
)

func chatReport(messageID uuid.UUID) *pb.SubmitReportRequest {
	return &pb.SubmitReportRequest{
		Target:   &pb.ReportTarget{Kind: pb.ReportTargetKind_REPORT_TARGET_KIND_CHAT_MESSAGE, Id: messageID.String()},
		Category: commonpb.PolicyCategory_POLICY_CATEGORY_HARASSMENT,
	}
}

func chatEnv(t *testing.T, ids identities, store *sellermod.Store) (*env, *fakeReports) {
	t.Helper()
	r := &fakeReports{}
	return newEnvIdentities(t, r, &fakeStores{pagedStores{current: store}}, ids), r
}

func messageIn(parties im.ConversationParties) (uuid.UUID, fakeMessages) {
	id := uuid.New()
	return id, fakeMessages{parties: map[uuid.UUID]im.ConversationParties{id: parties}}
}

func TestChatReportFromTheBuyerSideUsesTheBuyerIdentity(t *testing.T) {
	id, msgs := messageIn(im.ConversationParties{BuyerID: callerBuyer, StoreID: otherStore})
	e, r := chatEnv(t, identities{messages: msgs}, &sellermod.Store{StoreID: callerStore})
	if _, err := e.client.SubmitReport(context.Background(), signedIn(chatReport(id))); err != nil {
		t.Fatal(err)
	}
	if len(r.in) != 1 || r.in[0].ReporterBuyerID != callerBuyer || r.in[0].ReporterStoreID != 0 {
		t.Fatalf("買家方的檢舉人：%+v", r.in)
	}
}

func TestChatReportFromTheStoreSideUsesTheCurrentStoreNotTheBuyer(t *testing.T) {
	id, msgs := messageIn(im.ConversationParties{BuyerID: otherBuyer, StoreID: callerStore})
	e, r := chatEnv(t, identities{messages: msgs}, &sellermod.Store{StoreID: callerStore})
	if _, err := e.client.SubmitReport(context.Background(), signedIn(chatReport(id))); err != nil {
		t.Fatal(err)
	}
	if len(r.in) != 1 || r.in[0].ReporterStoreID != callerStore || r.in[0].ReporterBuyerID != 0 {
		t.Fatalf("店家方的檢舉人：%+v", r.in)
	}
	if r.in[0].Target != (moderation.Target{Kind: moderation.TargetChatMessage, ContentID: id}) {
		t.Fatalf("對象：%+v", r.in[0].Target)
	}
}

func TestAStoreOnlyAccountCanReportAChatMessage(t *testing.T) {
	id, msgs := messageIn(im.ConversationParties{BuyerID: otherBuyer, StoreID: callerStore})
	e, r := chatEnv(t, identities{buyers: fakeBuyers{missing: true}, messages: msgs}, &sellermod.Store{StoreID: callerStore})
	if _, err := e.client.SubmitReport(context.Background(), signedIn(chatReport(id))); err != nil {
		t.Fatal(err)
	}
	if len(r.in) != 1 || r.in[0].ReporterStoreID != callerStore {
		t.Fatalf("沒有買家身分的店家帳號：%+v", r.in)
	}
}

func TestChatReportByANonPartyIsNotFoundLikeAMissingMessageAndNeverReachesTheModule(t *testing.T) {
	id, msgs := messageIn(im.ConversationParties{BuyerID: otherBuyer, StoreID: otherStore})
	e, r := chatEnv(t, identities{messages: msgs}, &sellermod.Store{StoreID: callerStore})
	_, nonParty := e.client.SubmitReport(context.Background(), signedIn(chatReport(id)))
	_, missing := e.client.SubmitReport(context.Background(), signedIn(chatReport(uuid.New())))
	if connect.CodeOf(nonParty) != connect.CodeNotFound || connect.CodeOf(missing) != connect.CodeNotFound {
		t.Fatalf("非對話方 %v；不存在 %v", nonParty, missing)
	}
	if nonParty.Error() != missing.Error() {
		t.Fatalf("兩種 not_found 的訊息不同會洩漏訊息存在：%q vs %q", nonParty, missing)
	}
	if len(r.in) != 0 {
		t.Fatalf("不是對話方不該到模組：%+v", r.in)
	}
}

func TestChatReportWithoutAnyIdentityOnTheConversationIsNotFound(t *testing.T) {
	id, msgs := messageIn(im.ConversationParties{BuyerID: otherBuyer, StoreID: callerStore})
	e, r := chatEnv(t, identities{buyers: fakeBuyers{missing: true}, messages: msgs}, nil)
	if _, err := e.client.SubmitReport(context.Background(), signedIn(chatReport(id))); connect.CodeOf(err) != connect.CodeNotFound || len(r.in) != 0 {
		t.Fatalf("沒有買家也沒有店：%v %+v", err, r.in)
	}
}

func TestChatReportWhenTheCallerIsBothSidesIsAnInvariantFailure(t *testing.T) {
	id, msgs := messageIn(im.ConversationParties{BuyerID: callerBuyer, StoreID: callerStore})
	e, r := chatEnv(t, identities{messages: msgs}, &sellermod.Store{StoreID: callerStore})
	_, err := e.client.SubmitReport(context.Background(), signedIn(chatReport(id)))
	if connect.CodeOf(err) != connect.CodeInternal || len(r.in) != 0 {
		t.Fatalf("兩邊都是：%v %+v", err, r.in)
	}
	if !strings.Contains(e.logs.String(), "不變量被破壞") {
		t.Fatalf("要記 error log：%s", e.logs.String())
	}
}

func TestNonChatReportsStillRequireTheBuyerIdentity(t *testing.T) {
	e, r := chatEnv(t, identities{buyers: fakeBuyers{missing: true}}, &sellermod.Store{StoreID: callerStore})
	_, err := e.client.SubmitReport(context.Background(), signedIn(&pb.SubmitReportRequest{
		Target:   &pb.ReportTarget{Kind: pb.ReportTargetKind_REPORT_TARGET_KIND_COMMENT, Id: uuid.New().String()},
		Category: commonpb.PolicyCategory_POLICY_CATEGORY_SPAM,
	}))
	if connect.CodeOf(err) != connect.CodeInternal || len(r.in) != 0 {
		t.Fatalf("沒有買家身分的其他種類：%v %+v", err, r.in)
	}
}

func TestListMyStoreReportsReadsTheCurrentStoresReportsOnly(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	id1, id2, msg := uuid.UUID{1}, uuid.UUID{2}, uuid.UUID{3}
	r := &fakeReports{page: moderation.ReportPage{
		Items: []moderation.ReportItem{{
			Report: moderation.Report{ID: id1, Category: policy.Harassment, Description: "辱罵", CreatedAt: at, Outcome: moderation.OutcomeNone},
			Target: moderation.Target{Kind: moderation.TargetChatMessage, ContentID: msg},
		}},
		Next: &moderation.ReportCursor{CreatedAt: at, ID: id2},
	}}
	e := newEnvIdentities(t, r, &fakeStores{pagedStores{current: &sellermod.Store{StoreID: callerStore}}}, identities{})
	resp, err := e.client.ListMyStoreReports(context.Background(), signedIn(&pb.ListMyStoreReportsRequest{PageSize: 5}))
	if err != nil {
		t.Fatal(err)
	}
	if r.gotStore != callerStore || r.gotSize != 5 || r.gotBuyer != 0 {
		t.Fatalf("要以呼叫者此刻的店讀：store=%d size=%d buyer=%d", r.gotStore, r.gotSize, r.gotBuyer)
	}
	got := resp.Msg.GetReports()
	if len(got) != 1 || got[0].GetId() != id1.String() || got[0].GetCommunity().GetTarget().GetKind() != pb.ReportTargetKind_REPORT_TARGET_KIND_CHAT_MESSAGE ||
		got[0].GetCommunity().GetTarget().GetId() != msg.String() || got[0].GetDescription() != "辱罵" {
		t.Fatalf("回應：%v", resp.Msg)
	}
	token := resp.Msg.GetNextPageToken()
	if token == "" {
		t.Fatal("還有下一頁要給 token")
	}
	if _, err := e.client.ListMyStoreReports(context.Background(), signedIn(&pb.ListMyStoreReportsRequest{PageToken: token})); err != nil || r.gotAfter == nil || r.gotAfter.ID != id2 {
		t.Fatalf("游標：%v %+v", err, r.gotAfter)
	}
	other := newEnvIdentities(t, &fakeReports{}, &fakeStores{pagedStores{current: &sellermod.Store{StoreID: otherStore}}}, identities{})
	if _, err := other.client.ListMyStoreReports(context.Background(), signedIn(&pb.ListMyStoreReportsRequest{PageToken: token})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("別家店的 token 不能沿用：%v", err)
	}
	if _, err := e.client.ListMyReports(context.Background(), signedIn(&pb.ListMyReportsRequest{PageToken: token})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("別的方法的 token 不能沿用：%v", err)
	}
}

func TestListMyStoreReportsWithoutAStoreIsNotFound(t *testing.T) {
	r := &fakeReports{}
	e := newEnvIdentities(t, r, &fakeStores{}, identities{})
	if _, err := e.client.ListMyStoreReports(context.Background(), signedIn(&pb.ListMyStoreReportsRequest{})); connect.CodeOf(err) != connect.CodeNotFound || r.gotStore != 0 {
		t.Fatalf("沒有店：%v store=%d", err, r.gotStore)
	}
}
