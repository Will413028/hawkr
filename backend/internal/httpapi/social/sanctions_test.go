package social

import (
	"context"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/social/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
)

func TestListMySanctionsCarriesKindCategoryAndReasonAndPages(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	id := uuid.UUID{1}
	w := &fakeWrites{sanction: social.SanctionPage{
		Sanctions: []social.Sanction{{ID: id, BuyerID: 7, Kind: social.SanctionWarning, Category: "HARASSMENT", Text: "請友善發言", CreatedAt: at}},
		Next:      &social.SanctionCursor{CreatedAt: at, ID: id},
	}}
	e := newAppEnv(t, w, &fakeStores{})
	resp, err := e.client.ListMySanctions(context.Background(), signedIn(&pb.ListMySanctionsRequest{PageSize: 5}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetSanctions()
	if len(got) != 1 || got[0].GetKind() != pb.SanctionKind_SANCTION_KIND_WARNING || got[0].GetCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_HARASSMENT || got[0].GetReason() != "請友善發言" || !got[0].GetCreateTime().AsTime().Equal(at) {
		t.Fatalf("處分：%v", got)
	}
	if w.gotBuyer != 7 || w.gotSize != 5 {
		t.Fatalf("要以呼叫者的買家身分讀、帶頁大小：%d %d", w.gotBuyer, w.gotSize)
	}
	if resp.Msg.GetNextPageToken() == "" {
		t.Fatal("有下一頁要帶 token")
	}
	if _, err := e.client.ListMySanctions(context.Background(), signedIn(&pb.ListMySanctionsRequest{PageToken: resp.Msg.GetNextPageToken()})); err != nil || w.gotSAft == nil || w.gotSAft.ID != id {
		t.Fatalf("游標：%v %+v", err, w.gotSAft)
	}
	if _, err := e.client.ListMySanctions(context.Background(), signedIn(&pb.ListMySanctionsRequest{PageToken: "garbage"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("壞 token：%v", err)
	}
}

func TestListMyHiddenContentCarriesTheBodyAndReason(t *testing.T) {
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	postID := uuid.UUID{2}
	store := int64(11)
	w := &fakeWrites{hidden: social.HiddenPage{
		Items: []social.HiddenContent{
			{HideID: uuid.UUID{3}, Target: social.Target{Kind: social.TargetComment, ID: uuid.UUID{4}}, PostID: &postID, StoreID: &store, Body: "原文", CreatedAt: at, Category: "SPAM", Text: "廣告", HiddenAt: at.Add(time.Hour)},
			{HideID: uuid.UUID{5}, Target: social.Target{Kind: social.TargetForumEntry, ID: uuid.UUID{6}}, Body: "討論", CreatedAt: at, Category: "HATE", HiddenAt: at},
		},
		Next: &social.HiddenCursor{HiddenAt: at, HideID: uuid.UUID{5}},
	}}
	e := newAppEnv(t, w, &fakeStores{})
	resp, err := e.client.ListMyHiddenContent(context.Background(), signedIn(&pb.ListMyHiddenContentRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetHiddenContents()
	if len(got) != 2 {
		t.Fatalf("rows=%d", len(got))
	}
	c := got[0]
	if c.GetKind() != pb.HiddenContentKind_HIDDEN_CONTENT_KIND_COMMENT || c.GetBody() != "原文" || c.GetReason() != "廣告" || c.GetCategory() != commonpb.PolicyCategory_POLICY_CATEGORY_SPAM || c.GetPostId() != postID.String() || c.GetStoreId() != rpc.FormatID(11) || !c.GetHideTime().AsTime().Equal(at.Add(time.Hour)) {
		t.Fatalf("被隱藏的留言：%v", c)
	}
	f := got[1]
	if f.GetKind() != pb.HiddenContentKind_HIDDEN_CONTENT_KIND_FORUM_ENTRY || f.PostId != nil || f.StoreId != nil || f.GetReason() != "" {
		t.Fatalf("被隱藏的討論區留言：%v", f)
	}
	if _, err := e.client.ListMyHiddenContent(context.Background(), signedIn(&pb.ListMyHiddenContentRequest{PageToken: resp.Msg.GetNextPageToken()})); err != nil || w.gotHAft == nil || w.gotHAft.HideID != (uuid.UUID{5}) {
		t.Fatalf("游標：%v %+v", err, w.gotHAft)
	}
}

func TestSanctionReadsRequireTheBuyerIdentity(t *testing.T) {
	e := newAppEnv(t, &fakeWrites{}, &fakeStores{})
	if _, err := e.client.ListMySanctions(context.Background(), connect.NewRequest(&pb.ListMySanctionsRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("未登入：%v", err)
	}
	if _, err := e.client.ListMyHiddenContent(context.Background(), connect.NewRequest(&pb.ListMyHiddenContentRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("未登入：%v", err)
	}
}

// 作者讀自己的處分與被隱藏的內容只以買家身分，不查店家身分。
func TestAuthoredListsDoNotLookUpTheStore(t *testing.T) {
	s := &fakeStores{own: &seller.Store{StoreID: 5}}
	e := newAppEnv(t, &fakeWrites{}, s)
	if _, err := e.client.ListMySanctions(context.Background(), signedIn(&pb.ListMySanctionsRequest{})); err != nil {
		t.Fatal(err)
	}
	if _, err := e.client.ListMyHiddenContent(context.Background(), signedIn(&pb.ListMyHiddenContentRequest{})); err != nil {
		t.Fatal(err)
	}
	if s.storeLookups != 0 {
		t.Fatalf("查了店家身分 %d 次", s.storeLookups)
	}
}

func TestSanctionAndHiddenContentEnumsMatchTheContract(t *testing.T) {
	if got := rpc.EnumMismatches[pb.SanctionKind](social.SanctionKindValues()); len(got) != 0 {
		t.Fatalf("SanctionKind：%v", got)
	}
	if got := rpc.EnumMismatches[pb.HiddenContentKind]([]social.TargetKind{social.TargetComment, social.TargetForumEntry}); len(got) != 0 {
		t.Fatalf("HiddenContentKind：%v", got)
	}
}
