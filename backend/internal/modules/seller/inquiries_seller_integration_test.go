//go:build integration

package seller_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	. "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func (f *fixture) sellerReply(t *testing.T, accountID int64, inquiryID uuid.UUID, body string, names ...string) (SellerInquiry, error) {
	t.Helper()
	return f.module.ReplyToInquiry(context.Background(), ReplyToInquiryInput{
		AccountID: accountID, InquiryID: inquiryID, Body: body, EvidenceTokens: f.evidence(t, media.Uploader{ID: accountID}, names...),
	})
}

func (f *fixture) sellerList(t *testing.T, accountID int64, state *InquiryState) SellerInquiryPage {
	t.Helper()
	var out SellerInquiryPage
	var after *InquiryCursor
	for {
		page, err := f.module.AccountInquiries(context.Background(), AccountInquiryListInput{AccountID: accountID, PageSize: 2, State: state, After: after})
		must(t, err)
		out.Items = append(out.Items, page.Items...)
		out.PendingReplyCount = page.PendingReplyCount
		if page.Next == nil {
			return out
		}
		after = page.Next
	}
}

// 別家店的帳號讀不到、回覆不了（與不存在同一個 not_found），也不轉正附件、不寫入；列表只含自己店家的詢問。
func TestSellerCannotReadOrReplyToAnotherStoresInquiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, s := f.open(t)
	stranger, other := f.open(t)
	i := f.mustInquire(t, f.foodCase(t, s.StoreID).ID).Inquiry
	f.mustInquire(t, f.foodCase(t, other.StoreID).ID)

	_, err := f.module.AccountInquiry(ctx, stranger, i.ID)
	wantKind(t, err, apperr.KindNotFound)
	_, err = f.module.AccountInquiry(ctx, owner, uuid.NewV7())
	wantKind(t, err, apperr.KindNotFound)
	strangerToken, strangerAsset := f.stageSecure(t, media.Uploader{ID: stranger}, evidencePDF("doc"))
	_, err = f.module.ReplyToInquiry(ctx, ReplyToInquiryInput{AccountID: stranger, InquiryID: i.ID, Body: "不是我的店", EvidenceTokens: []string{strangerToken}})
	wantKind(t, err, apperr.KindNotFound)
	f.wantAsset(t, strangerAsset, "STAGED")
	if f.count(t, "SELECT count(*) FROM seller.case_inquiries WHERE reply_received_at IS NOT NULL") != 0 {
		t.Fatal("別人的回覆寫入了")
	}
	mine := f.sellerList(t, owner, nil)
	if len(mine.Items) != 1 || mine.Items[0].ID != i.ID || mine.PendingReplyCount != 1 {
		t.Fatalf("list=%+v", mine)
	}
	got, err := f.module.AccountInquiry(ctx, owner, i.ID)
	must(t, err)
	if got.ID != i.ID || !got.CanReply || got.Message != i.Message {
		t.Fatalf("get=%+v", got)
	}
	for _, id := range []int64{0, -1} {
		if _, err := f.module.AccountInquiry(ctx, id, i.ID); err == nil {
			t.Fatalf("帳號 %d 不該讀得到", id)
		}
	}
}

// 賣家回覆：詢問結束、管道是賣家中心、回覆的帳號是賣家、附件以賣家帳號（非管理員）轉正並只留數量、案件的版本前進；
// 管理員從後台看到管道與回覆的帳號；之後再回覆（賣家或管理員）都不寫入。
func TestSellerReplyEndsTheInquiryAndIsVisibleToAdmin(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, s := f.open(t)
	c := f.foodCase(t, s.StoreID)
	i := f.mustInquire(t, c.ID).Inquiry
	before := f.caseOf(t, c.ID)
	f.now = f.now.Add(2 * time.Hour)

	for name, tt := range map[string]struct {
		body   string
		tokens []string
		field  string
	}{
		"內容是空白":  {"  ", nil, "Body"},
		"內容太長":   {strings.Repeat("字", 4001), nil, "Body"},
		"附件超過 5": {"x", []string{"a", "b", "c", "d", "e", "f"}, "EvidenceTokens"},
	} {
		_, err := f.sellerReply(t, owner, i.ID, tt.body, tt.tokens...)
		if fields := invalidFields(t, err); !slices.Equal(fields, []string{tt.field}) {
			t.Errorf("%s：欄位=%v err=%v", name, fields, err)
		}
	}
	if f.count(t, "SELECT count(*) FROM seller.case_inquiries WHERE reply_received_at IS NOT NULL") != 0 {
		t.Fatal("不合法的輸入寫入了")
	}

	got, err := f.sellerReply(t, owner, i.ID, "  已附上進貨單  ", "doc1", "doc2")
	must(t, err)
	r := got.Reply
	if got.State() != InquiryStateReplied || got.CanReply || got.AttachmentCount != 2 || r == nil || r.Body != "已附上進貨單" || r.Channel != InquiryChannelSellerCenter ||
		r.RepliedByAccountID != owner || r.RecordedByAdminID != 0 || !r.ReceivedAt.Equal(f.now) || !r.RecordedAt.Equal(f.now) || len(r.Evidence) != 0 {
		t.Fatalf("reply=%+v", got)
	}
	if n := f.activeEvidenceAssets(t, i.ID); n != 2 {
		t.Fatalf("有效的附件資產=%d", n)
	}
	after := f.caseOf(t, c.ID)
	if after.Version != before.Version+1 || !after.UpdatedAt.Equal(f.now) || after.AwaitingSeller() {
		t.Fatalf("case=%+v", after.Case)
	}
	if n := f.count(t, "SELECT count(*) FROM seller.evidence_accesses"); n != 0 {
		t.Fatalf("賣家自己上傳的附件不寫 evidence_accesses：%d", n)
	}

	view := f.inquiriesOf(t, c.ID)[0]
	if view.Reply == nil || view.Reply.Channel != InquiryChannelSellerCenter || view.Reply.RepliedByAccountID != owner || view.Reply.RecordedByAdminID != 0 || len(view.Reply.Evidence) != 2 {
		t.Fatalf("admin view=%+v", view.Reply)
	}

	f.now = f.now.Add(time.Hour)
	_, err = f.sellerReply(t, owner, i.ID, "再回一次")
	wantBlocked(t, err, "這筆詢問已經回覆過")
	if f.count(t, "SELECT count(*) FROM seller.case_inquiries WHERE reply_body = '再回一次'") != 0 {
		t.Fatal("第二次回覆寫入了")
	}
	rec, err := f.reply(t, i.ID, f.now)
	must(t, err)
	if !rec.AlreadyRecorded || rec.Inquiry.Reply.RepliedByAccountID != owner {
		t.Fatalf("rec=%+v", rec.Inquiry.Reply)
	}
	again, err := f.module.AccountInquiry(ctx, owner, i.ID)
	must(t, err)
	if again.Reply.Body != "已附上進貨單" || again.AttachmentCount != 2 {
		t.Fatalf("again=%+v", again)
	}
}

// 管理員記下的回覆賣家看得到（唯讀，管道與內容），也不能再回覆；管理員不能選賣家中心管道。
func TestSellerSeesTheAdminRecordedReplyAndAdminCannotPickTheSellerCenterChannel(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, s := f.open(t)
	i := f.mustInquire(t, f.foodCase(t, s.StoreID).ID).Inquiry
	_, err := f.module.RecordCaseInquiryReply(ctx, RecordCaseInquiryReplyInput{InquiryID: i.ID, AdminID: inquiryRecorder, Channel: InquiryChannelSellerCenter, ReceivedAt: f.now, Body: "x"})
	if fields := invalidFields(t, err); !slices.Equal(fields, []string{"Channel"}) {
		t.Fatalf("fields=%v err=%v", fields, err)
	}
	_, err = f.reply(t, i.ID, f.now, "e0")
	must(t, err)
	got, err := f.module.AccountInquiry(ctx, owner, i.ID)
	must(t, err)
	if got.State() != InquiryStateReplied || got.CanReply || got.Reply.Channel != InquiryChannelPhone || got.AttachmentCount != 1 || len(got.Reply.Evidence) != 0 {
		t.Fatalf("get=%+v", got)
	}
	_, err = f.sellerReply(t, owner, i.ID, "補充")
	wantBlocked(t, err, "這筆詢問已經回覆過")
}

// 撤回之後（管理員撤回或案件結案）賣家仍讀得到（唯讀）但不能回覆，也不算待回覆。
func TestSellerCannotReplyToAWithdrawnInquiry(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, s := f.open(t)
	i := f.mustInquire(t, f.foodCase(t, s.StoreID).ID).Inquiry
	_, err := f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: i.ID, AdminID: inquiryWithdraw})
	must(t, err)
	lateToken, lateAsset := f.stageSecure(t, media.Uploader{ID: owner}, evidencePDF("doc"))
	_, err = f.module.ReplyToInquiry(ctx, ReplyToInquiryInput{AccountID: owner, InquiryID: i.ID, Body: "太晚了", EvidenceTokens: []string{lateToken}})
	wantBlocked(t, err, "這筆詢問已撤回")
	f.wantAsset(t, lateAsset, "STAGED")
	got, err := f.module.AccountInquiry(ctx, owner, i.ID)
	must(t, err)
	if got.State() != InquiryStateWithdrawn || got.CanReply || got.Withdrawn == nil {
		t.Fatalf("get=%+v", got)
	}
	if page := f.sellerList(t, owner, nil); page.PendingReplyCount != 0 || len(page.Items) != 1 {
		t.Fatalf("page=%+v", page)
	}
}

// 逾期後仍可回覆；已關閉的店家，詢問開放中也能讀與回覆。
func TestSellerCanReplyAfterTheDeadlineAndAfterTheStoreClosed(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, s := f.open(t)
	i := f.mustInquire(t, f.foodCase(t, s.StoreID).ID).Inquiry
	f.now = i.DueAt.Add(48 * time.Hour)
	if page := f.sellerList(t, owner, nil); page.PendingReplyCount != 1 || !page.Items[0].CanReply {
		t.Fatalf("逾期仍算待回覆：%+v", page)
	}
	_, err := f.module.CloseCurrent(ctx, owner)
	must(t, err)
	if page := f.sellerList(t, owner, nil); len(page.Items) != 1 || !page.Items[0].CanReply || page.PendingReplyCount != 1 {
		t.Fatalf("關店後仍讀得到：%+v", page)
	}
	got, err := f.sellerReply(t, owner, i.ID, "逾期且關店後回覆")
	must(t, err)
	if got.State() != InquiryStateReplied || !got.Reply.ReceivedAt.Equal(f.now) {
		t.Fatalf("got=%+v", got)
	}
}

// 列表新到舊、可依狀態篩選、分頁；待回覆數不受篩選與分頁影響，只算這個帳號的。
func TestSellerInquiryListIsNewestFirstFilteredAndCounted(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	owner, s := f.open(t)
	_, other := f.open(t)
	f.mustInquire(t, f.foodCase(t, other.StoreID).ID)
	var ids []uuid.UUID
	for _, category := range []policy.Category{policy.FoodSafety, policy.Counterfeit, policy.UnlicensedSale, policy.ProhibitedGoods} {
		f.now = f.now.Add(time.Hour)
		c, _ := f.newCase(t, s.StoreID, category, SeverityHigh, 1)
		ids = append(ids, f.mustInquire(t, c.ID).Inquiry.ID)
	}
	_, err := f.sellerReply(t, owner, ids[0], "回覆")
	must(t, err)
	_, err = f.module.WithdrawCaseInquiry(ctx, WithdrawCaseInquiryInput{InquiryID: ids[1], AdminID: inquiryWithdraw})
	must(t, err)

	all := f.sellerList(t, owner, nil)
	got := make([]uuid.UUID, len(all.Items))
	for n, it := range all.Items {
		got[n] = it.ID
	}
	if want := []uuid.UUID{ids[3], ids[2], ids[1], ids[0]}; !slices.Equal(got, want) || all.PendingReplyCount != 2 {
		t.Fatalf("got=%v pending=%d", got, all.PendingReplyCount)
	}
	for state, want := range map[InquiryState][]uuid.UUID{
		InquiryStateOpen: {ids[3], ids[2]}, InquiryStateReplied: {ids[0]}, InquiryStateWithdrawn: {ids[1]},
	} {
		page := f.sellerList(t, owner, &state)
		var have []uuid.UUID
		for _, it := range page.Items {
			have = append(have, it.ID)
		}
		if !slices.Equal(have, want) || page.PendingReplyCount != 2 {
			t.Errorf("%s：have=%v want=%v pending=%d", state, have, want, page.PendingReplyCount)
		}
	}
	bad := InquiryState("DONE")
	_, err = f.module.AccountInquiries(ctx, AccountInquiryListInput{AccountID: owner, State: &bad})
	wantKind(t, err, apperr.KindInvalidInput)
}
