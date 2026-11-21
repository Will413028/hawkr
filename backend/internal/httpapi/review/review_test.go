package review

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/review/v1alpha1/reviewv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	callerAccount = 1
	callerBuyer   = 70
	callerStoreID = 11
)

var at = time.Date(2026, 11, 2, 3, 4, 5, 0, time.UTC)

// fakeReads 記下每個讀取的參數並回固定的結果。
type fakeReads struct {
	page   review.Page
	one    review.Review
	stats  review.StoreStats
	err    error
	calls  []string
	viewer int64
	id     int64
	filter review.Filter
	item   int64
	after  *review.Cursor
	size   int32
	gotID  uuid.UUID
}

func (f *fakeReads) rec(op string, id, viewer int64, filter review.Filter, after *review.Cursor, size int32) (review.Page, error) {
	f.calls = append(f.calls, op)
	f.id, f.viewer, f.filter, f.after, f.size = id, viewer, filter, after, size
	return f.page, f.err
}

func (f *fakeReads) ListProductReviews(_ context.Context, productID, viewer int64, flt review.Filter, after *review.Cursor, size int32) (review.Page, error) {
	return f.rec("ListProductReviews", productID, viewer, flt, after, size)
}

func (f *fakeReads) ListStoreReviews(_ context.Context, storeID, viewer int64, flt review.Filter, after *review.Cursor, size int32) (review.Page, error) {
	return f.rec("ListStoreReviews", storeID, viewer, flt, after, size)
}

func (f *fakeReads) ListOwnStoreReviews(_ context.Context, storeID int64, flt review.Filter, after *review.Cursor, size int32) (review.Page, error) {
	return f.rec("ListOwnStoreReviews", storeID, 0, flt, after, size)
}

func (f *fakeReads) ListMyReviews(_ context.Context, buyerID, orderItemID int64, after *review.Cursor, size int32) (review.Page, error) {
	f.item = orderItemID
	return f.rec("ListMyReviews", buyerID, 0, review.Filter{}, after, size)
}

func (f *fakeReads) GetReview(_ context.Context, id uuid.UUID, viewer int64) (review.Review, error) {
	f.calls = append(f.calls, "GetReview")
	f.gotID, f.viewer = id, viewer
	return f.one, f.err
}

func (f *fakeReads) StoreStats(_ context.Context, storeID int64) (review.StoreStats, error) {
	f.calls = append(f.calls, "StoreStats")
	f.id = storeID
	return f.stats, f.err
}

// fakeWrites 記下寫入的輸入。
type fakeWrites struct {
	review review.Review
	err    error
	submit review.SubmitInput
	reply  review.ReplyInput
	calls  []string
}

func (f *fakeWrites) Submit(_ context.Context, in review.SubmitInput) (review.Review, error) {
	f.calls = append(f.calls, "Submit")
	f.submit = in
	return f.review, f.err
}

func (f *fakeWrites) Reply(_ context.Context, in review.ReplyInput) (review.Review, error) {
	f.calls = append(f.calls, "Reply")
	f.reply = in
	return f.review, f.err
}

type fakeBuyers struct {
	displays []buyer.Display
	calls    int
	batches  [][]int64
}

func (f *fakeBuyers) CurrentBuyerID(_ context.Context, accountID int64) (int64, error) {
	if accountID != callerAccount {
		return 0, apperr.NotFound("no buyer")
	}
	return callerBuyer, nil
}

func (f *fakeBuyers) BatchDisplay(_ context.Context, ids []int64) ([]buyer.Display, error) {
	f.calls++
	f.batches = append(f.batches, slices.Clone(ids))
	return f.displays, nil
}

type fakeStores struct{ own *seller.Store }

func (f fakeStores) CurrentStore(context.Context, int64) (seller.Store, error) {
	if f.own == nil {
		return seller.Store{}, apperr.NotFound("no store")
	}
	return *f.own, nil
}

func images(t *testing.T) rpc.Images {
	t.Helper()
	im, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return im
}

func signedIn() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})
}

func asAdmin() context.Context {
	return rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 9})
}

func codeOf(err error) connect.Code {
	return connect.CodeOf(rpc.ConnectError(context.Background(), err))
}

// reasonOf 回傳錯誤帶的 ErrorInfo.reason；沒有時是空字串。
func reasonOf(err error) string {
	for _, d := range rpc.ConnectError(context.Background(), err).Details() {
		v, e := d.Value()
		if e != nil {
			continue
		}
		if info, ok := v.(*errdetails.ErrorInfo); ok {
			return info.GetReason()
		}
	}
	return ""
}

func ownStore() *seller.Store { return &seller.Store{StoreID: callerStoreID, AccountID: callerAccount} }

type env struct {
	reads  *fakeReads
	writes *fakeWrites
	buyers *fakeBuyers
	public *Public
	app    *App
}

// withReason 是帶業務原因的模組錯誤的替身：reasonOf 的測試版本只認這個型別。
type reasonedError struct {
	error
	reason review.Reason
}

func withReason(err error, r review.Reason) error { return reasonedError{err, r} }

func (e reasonedError) Unwrap() error { return e.error }

func testReasonOf(err error) review.Reason {
	var r reasonedError
	if errors.As(err, &r) {
		return r.reason
	}
	return ""
}

func newEnv(t *testing.T, store *seller.Store) *env {
	t.Helper()
	e := &env{reads: &fakeReads{}, writes: &fakeWrites{}, buyers: &fakeBuyers{}}
	var err error
	if e.public, err = NewPublic(e.reads, e.buyers, images(t), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	if e.app, err = NewApp(e.reads, e.writes, e.buyers, fakeStores{own: store}, images(t), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	e.app.reasonOf = testReasonOf
	return e
}

func sampleReview(id byte) review.Review {
	key := "items/sku.jpg"
	return review.Review{
		ID: uuid.UUID{id}, OrderNo: "ORD-1", OrderItemID: 501, BuyerID: 80, StoreID: 11, ProductID: 31, Rating: 4, Content: "好吃",
		Images:  []review.Image{{AssetID: "a1", ObjectKey: "reviews/a1.jpg"}, {AssetID: "a2", ObjectKey: "reviews/a2.jpg"}},
		SkuName: "大份", ItemImageKey: &key, Price: 250, CreatedAt: at,
		Reply: &review.Reply{Content: "謝謝", RepliedAt: at.Add(time.Hour)},
	}
}

func TestListProductReviewsMapsEveryFieldAndHidesOrderDetailsInThePublicView(t *testing.T) {
	e := newEnv(t, nil)
	name, avatar := "阿美", "avatars/80.jpg"
	e.buyers.displays = []buyer.Display{{BuyerID: 80, DisplayName: &name, Avatar: &avatar}}
	e.reads.page = review.Page{Reviews: []review.Review{sampleReview(1)}}
	resp, err := e.public.ListProductReviews(context.Background(), connect.NewRequest(&pb.ListProductReviewsRequest{ProductId: "31"}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetReviews()
	if len(got) != 1 {
		t.Fatalf("reviews = %d", len(got))
	}
	r := got[0]
	if r.GetId() != (uuid.UUID{1}).String() || r.GetProductId() != "31" || r.GetStoreId() != "11" || r.GetRating() != 4 || r.GetContent() != "好吃" ||
		r.GetSkuName() != "大份" || r.GetPrice() != 250 || !r.GetCreateTime().AsTime().Equal(at) {
		t.Errorf("基本欄位：%v", r)
	}
	if len(r.GetImages()) != 2 || r.GetImages()[0].GetUriTemplate() != "https://media.example.test/i/{preset}/reviews/a1.jpg" ||
		r.GetItemImage().GetUriTemplate() != "https://media.example.test/i/{preset}/items/sku.jpg" {
		t.Errorf("圖片要組成 uri_template、不回 object key：%v %v", r.GetImages(), r.GetItemImage())
	}
	if r.GetReply().GetContent() != "謝謝" || !r.GetReply().GetReplyTime().AsTime().Equal(at.Add(time.Hour)) {
		t.Errorf("回覆：%v", r.GetReply())
	}
	if r.GetAuthor().GetName() != "阿美" || r.GetAuthor().GetAvatar().GetUriTemplate() != "https://media.example.test/i/{preset}/avatars/80.jpg" {
		t.Errorf("作者：%v", r.GetAuthor())
	}
	if r.OrderNo != nil || r.OrderItemId != nil {
		t.Errorf("公開讀取不帶訂單編號與訂單項目：%v %v", r.OrderNo, r.OrderItemId)
	}
	if resp.Msg.GetNextPageToken() != "" {
		t.Error("沒有下一頁不帶 token")
	}
}

func TestReviewWithoutReplyAnItemImageOrAuthorDisplayLeavesThoseFieldsUnset(t *testing.T) {
	e := newEnv(t, nil)
	r := sampleReview(1)
	r.Reply, r.ItemImageKey = nil, nil
	e.reads.page = review.Page{Reviews: []review.Review{r}}
	e.buyers.displays = []buyer.Display{{BuyerID: 80, Deleted: true}}
	resp, err := e.public.ListStoreReviews(context.Background(), connect.NewRequest(&pb.ListStoreReviewsRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetReviews()[0]
	if got.Reply != nil || got.ItemImage != nil || got.GetAuthor().Name != nil || got.GetAuthor().GetAvatar() != nil {
		t.Errorf("沒有的欄位不設：%v", got)
	}
}

func TestListProductReviewsReadsAsTheViewerAndPassesFilterAndPaging(t *testing.T) {
	e := newEnv(t, nil)
	rating := int32(5)
	req := &pb.ListProductReviewsRequest{ProductId: "31", PageSize: 7, Rating: &rating, WithImages: true}
	if _, err := e.public.ListProductReviews(context.Background(), connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	if e.reads.viewer != 0 || e.reads.id != 31 || e.reads.size != 7 || e.reads.filter != (review.Filter{Rating: 5, WithImages: true}) || e.reads.after != nil {
		t.Errorf("訪客：viewer=%d id=%d size=%d filter=%+v", e.reads.viewer, e.reads.id, e.reads.size, e.reads.filter)
	}
	if _, err := e.public.ListProductReviews(signedIn(), connect.NewRequest(req)); err != nil {
		t.Fatal(err)
	}
	if e.reads.viewer != callerBuyer {
		t.Errorf("登入的觀看者要以買家 ID 傳給模組（套用封鎖）：%d", e.reads.viewer)
	}
	if _, err := e.public.ListProductReviews(asAdmin(), connect.NewRequest(req)); err != nil || e.reads.viewer != 0 {
		t.Errorf("管理員以訪客讀：%v viewer=%d", err, e.reads.viewer)
	}
}

func TestListStoreReviewsPassesTheViewerBuyerSoTheModuleAppliesBlocks(t *testing.T) {
	e := newEnv(t, nil)
	if _, err := e.public.ListStoreReviews(signedIn(), connect.NewRequest(&pb.ListStoreReviewsRequest{StoreId: "11"})); err != nil {
		t.Fatal(err)
	}
	if e.reads.calls[0] != "ListStoreReviews" || e.reads.id != 11 || e.reads.viewer != callerBuyer {
		t.Fatalf("店家公開評價要帶觀看者的買家 ID：%v id=%d viewer=%d", e.reads.calls, e.reads.id, e.reads.viewer)
	}
	if _, err := e.public.ListStoreReviews(context.Background(), connect.NewRequest(&pb.ListStoreReviewsRequest{StoreId: "11"})); err != nil || e.reads.viewer != 0 {
		t.Fatalf("訪客不套封鎖：%v viewer=%d", err, e.reads.viewer)
	}
}

func TestPublicListsRejectBadIDsAndAreNotCalledWithThem(t *testing.T) {
	e := newEnv(t, nil)
	for _, bad := range []string{"", "x", "0", "-3"} {
		if _, err := e.public.ListProductReviews(context.Background(), connect.NewRequest(&pb.ListProductReviewsRequest{ProductId: bad})); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("商品 %q：%v", bad, err)
		}
		if _, err := e.public.ListStoreReviews(context.Background(), connect.NewRequest(&pb.ListStoreReviewsRequest{StoreId: bad})); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("店家 %q：%v", bad, err)
		}
		if _, err := e.public.GetStoreReviewStats(context.Background(), connect.NewRequest(&pb.GetStoreReviewStatsRequest{StoreId: bad})); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("統計 %q：%v", bad, err)
		}
	}
	if len(e.reads.calls) != 0 {
		t.Errorf("格式錯誤的 ID 不能到模組：%v", e.reads.calls)
	}
}

func TestPageTokenRoundTripsAndIsBoundToTheSameQuery(t *testing.T) {
	e := newEnv(t, nil)
	next := review.Cursor{CreatedAt: at, ID: uuid.UUID{9}}
	e.reads.page = review.Page{Reviews: []review.Review{sampleReview(1)}, Next: &next}
	resp, err := e.public.ListProductReviews(context.Background(), connect.NewRequest(&pb.ListProductReviewsRequest{ProductId: "31"}))
	if err != nil || resp.Msg.GetNextPageToken() == "" {
		t.Fatalf("有下一頁要帶 token：%v %v", resp, err)
	}
	token := resp.Msg.GetNextPageToken()
	if _, err := e.public.ListProductReviews(context.Background(), connect.NewRequest(&pb.ListProductReviewsRequest{ProductId: "31", PageToken: token})); err != nil {
		t.Fatal(err)
	}
	if e.reads.after == nil || !e.reads.after.CreatedAt.Equal(at) || e.reads.after.ID != next.ID {
		t.Errorf("游標要傳進模組：%+v", e.reads.after)
	}
	rating := int32(3)
	for name, req := range map[string]*pb.ListProductReviewsRequest{
		"換商品":     {ProductId: "32", PageToken: token},
		"換篩選":     {ProductId: "31", PageToken: token, Rating: &rating},
		"換附圖":     {ProductId: "31", PageToken: token, WithImages: true},
		"壞 token": {ProductId: "31", PageToken: "garbage"},
	} {
		if _, err := e.public.ListProductReviews(context.Background(), connect.NewRequest(req)); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s 不能沿用 token：%v", name, err)
		}
	}
}

func TestListReviewsMapsModuleRatingErrorToTheRequestField(t *testing.T) {
	e := newEnv(t, nil)
	e.reads.err = apperr.InvalidInputOn([]string{"Filter.Rating"}, "星等不合法")
	_, err := e.public.ListStoreReviews(context.Background(), connect.NewRequest(&pb.ListStoreReviewsRequest{StoreId: "11"}))
	ce := rpc.ConnectError(context.Background(), err)
	if ce.Code() != connect.CodeInvalidArgument {
		t.Fatalf("%v", err)
	}
	found := false
	for _, d := range ce.Details() {
		if v, e := d.Value(); e == nil {
			if br, ok := v.(*errdetails.BadRequest); ok && len(br.GetFieldViolations()) == 1 && br.GetFieldViolations()[0].GetField() == "rating" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("星等錯誤要指向 rating 欄位：%v", ce.Details())
	}
}

func TestGetStoreReviewStatsReturnsAllFiveStarBucketsAndDoesNotReadTheViewer(t *testing.T) {
	e := newEnv(t, nil)
	e.reads.stats = review.StoreStats{Count: 9, Average: 4.33, Distribution: [5]int64{1, 0, 2, 3, 3}}
	resp, err := e.public.GetStoreReviewStats(signedIn(), connect.NewRequest(&pb.GetStoreReviewStatsRequest{StoreId: "11"}))
	if err != nil {
		t.Fatal(err)
	}
	m := resp.Msg
	if m.GetCount() != 9 || m.GetAverage() != 4.33 || len(m.GetDistribution()) != 5 || e.reads.id != 11 {
		t.Fatalf("統計：%v id=%d", m, e.reads.id)
	}
	for i, b := range m.GetDistribution() {
		if b.GetRating() != int32(i+1) || b.GetCount() != e.reads.stats.Distribution[i] {
			t.Errorf("第 %d 項：%v", i, b)
		}
	}
	if e.buyers.calls != 0 {
		t.Error("統計不依觀看者，不查作者顯示資料")
	}
}

func TestAuthorDisplayIsLookedUpOncePerUniqueBuyer(t *testing.T) {
	e := newEnv(t, nil)
	a, b, c := sampleReview(1), sampleReview(2), sampleReview(3)
	b.BuyerID, c.BuyerID = 81, 80
	e.reads.page = review.Page{Reviews: []review.Review{a, b, c}}
	if _, err := e.public.ListProductReviews(context.Background(), connect.NewRequest(&pb.ListProductReviewsRequest{ProductId: "31"})); err != nil {
		t.Fatal(err)
	}
	if e.buyers.calls != 1 || !slices.Equal(e.buyers.batches[0], []int64{80, 81}) {
		t.Errorf("一次批次、去重：%d %v", e.buyers.calls, e.buyers.batches)
	}
}

func TestGetReviewReadsAsTheCallersBuyerAndMapsNotFound(t *testing.T) {
	e := newEnv(t, nil)
	e.reads.one = sampleReview(1)
	resp, err := e.app.GetReview(signedIn(), connect.NewRequest(&pb.GetReviewRequest{ReviewId: (uuid.UUID{1}).String()}))
	if err != nil || resp.Msg.GetReview().GetId() != (uuid.UUID{1}).String() {
		t.Fatalf("%v %v", resp, err)
	}
	if e.reads.viewer != callerBuyer || e.reads.gotID != (uuid.UUID{1}) {
		t.Errorf("觀看者是呼叫者的買家：%d %v", e.reads.viewer, e.reads.gotID)
	}
	if resp.Msg.GetReview().OrderNo != nil {
		t.Error("GetReview 是公開可讀的視圖，不帶訂單編號")
	}
	e.reads.err = apperr.NotFound("看不到")
	if _, err := e.app.GetReview(signedIn(), connect.NewRequest(&pb.GetReviewRequest{ReviewId: (uuid.UUID{1}).String()})); codeOf(err) != connect.CodeNotFound || reasonOf(err) != "" {
		t.Errorf("看不到的評價 not_found 且不帶 reason：%v", err)
	}
	if _, err := e.app.GetReview(signedIn(), connect.NewRequest(&pb.GetReviewRequest{ReviewId: "x"})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("ID 格式：%v", err)
	}
	if _, err := e.app.GetReview(asAdmin(), connect.NewRequest(&pb.GetReviewRequest{ReviewId: (uuid.UUID{1}).String()})); codeOf(err) != connect.CodePermissionDenied {
		t.Errorf("管理員：%v", err)
	}
}

func TestReplyReviewActsAsTheCallersCurrentStoreAndReturnsTheStoreView(t *testing.T) {
	e := newEnv(t, ownStore())
	e.writes.review = sampleReview(1)
	resp, err := e.app.ReplyReview(signedIn(), connect.NewRequest(&pb.ReplyReviewRequest{ReviewId: (uuid.UUID{1}).String(), Content: "謝謝"}))
	if err != nil {
		t.Fatal(err)
	}
	if e.writes.reply.Store.StoreID != callerStoreID || e.writes.reply.ReviewID != (uuid.UUID{1}) || e.writes.reply.Content != "謝謝" {
		t.Errorf("店家身分取自呼叫者此刻的店：%+v", e.writes.reply)
	}
	if resp.Msg.GetReview().GetOrderNo() != "ORD-1" || resp.Msg.GetReview().GetOrderItemId() != "501" {
		t.Errorf("店家視圖帶訂單資訊：%v", resp.Msg.GetReview())
	}
}

func TestReplyReviewWithoutAStoreIsNotFoundAndNeverReachesTheModule(t *testing.T) {
	e := newEnv(t, nil)
	_, err := e.app.ReplyReview(signedIn(), connect.NewRequest(&pb.ReplyReviewRequest{ReviewId: (uuid.UUID{1}).String(), Content: "x"}))
	if codeOf(err) != connect.CodeNotFound || len(e.writes.calls) != 0 {
		t.Fatalf("沒有店：%v calls=%v", err, e.writes.calls)
	}
}

func TestReplyReviewOfAnotherStoresReviewKeepsTheModulePermissionDenied(t *testing.T) {
	e := newEnv(t, ownStore())
	e.writes.err = apperr.PermissionDenied("不是你的評價")
	_, err := e.app.ReplyReview(signedIn(), connect.NewRequest(&pb.ReplyReviewRequest{ReviewId: (uuid.UUID{1}).String(), Content: "x"}))
	if codeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("評價不屬於呼叫者的店：%v", err)
	}
}

func TestReplyReviewIsCheckedAgainstTheModulesMarketingCapability(t *testing.T) {
	declared := declarations[reviewv1alpha1connect.ReviewAppServiceReplyReviewProcedure]
	if declared.capability != seller.CapMarketing {
		t.Fatalf("宣告的能力 = %q", declared.capability)
	}
	suspended := ownStore()
	suspended.Status = seller.Active
	suspended.Sanctions = seller.SanctionedBy(seller.EffectiveMeasure{MeasureID: uuid.NewV7()})
	suspended.Conditions = seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true, KYCApproved: true, TransfersActive: true, StoreName: true, Logo: true, TownCode: true}
	wantBlocked := seller.Authorize(*suspended, declared.capability) != nil
	// 沒有資料庫的模組：能力檢查在碰資料庫之前，被擋的回模組的錯誤，通過的會走到資料庫而 panic。
	h, err := NewApp(&fakeReads{}, moduleWrites{&review.Module{}}, &fakeBuyers{}, fakeStores{own: suspended}, images(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.ReplyReview(signedIn(), connect.NewRequest(&pb.ReplyReviewRequest{ReviewId: (uuid.UUID{1}).String(), Content: "x"}))
	if !wantBlocked || codeOf(err) != connect.CodePermissionDenied || reasonOf(err) != "SELLER_SUSPENDED" {
		t.Fatalf("停權的店家要被 CapMarketing 擋下（permission_denied＋SELLER_SUSPENDED）：wantBlocked=%v %v", wantBlocked, err)
	}
}

// moduleWrites 把真的模組當寫入來源；通過能力檢查的呼叫會在沒有資料庫的模組裡 panic，這裡接住並回 errReached。
type moduleWrites struct{ m *review.Module }

var errReached = errors.New("通過了能力檢查")

func (w moduleWrites) Reply(ctx context.Context, in review.ReplyInput) (r review.Review, err error) {
	defer func() {
		if recover() != nil {
			err = errReached
		}
	}()
	return w.m.Reply(ctx, in)
}

func (w moduleWrites) Submit(ctx context.Context, in review.SubmitInput) (review.Review, error) {
	return w.m.Submit(ctx, in)
}

func TestReplyReviewOfAnActiveStorePassesTheModulesCapabilityCheck(t *testing.T) {
	st := ownStore()
	st.Status = seller.Active
	st.Sanctions = seller.NoSanctions()
	st.Conditions = seller.Conditions{Derived: true, Active: true, NotPendingDeletion: true, KYCApproved: true, TransfersActive: true, StoreName: true, Logo: true, TownCode: true}
	h, err := NewApp(&fakeReads{}, moduleWrites{&review.Module{}}, &fakeBuyers{}, fakeStores{own: st}, images(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.ReplyReview(signedIn(), connect.NewRequest(&pb.ReplyReviewRequest{ReviewId: (uuid.UUID{1}).String(), Content: "x"})); !errors.Is(err, errReached) {
		t.Fatalf("正常的店家要通過能力檢查：%v", err)
	}
}

func TestCreateReviewActsAsTheCallersBuyerAndReturnsTheAuthorView(t *testing.T) {
	e := newEnv(t, nil)
	e.writes.review = sampleReview(1)
	req := &pb.CreateReviewRequest{OrderNo: "ORD-1", OrderItemId: "501", Rating: 4, Content: "好吃", ImageStagedTokens: []string{"t1", "t2"}}
	resp, err := e.app.CreateReview(signedIn(), connect.NewRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	want := review.SubmitInput{BuyerID: callerBuyer, OrderNo: "ORD-1", OrderItemID: 501, Rating: 4, Content: "好吃", StagedTokens: []string{"t1", "t2"}}
	if !slices.Equal(e.writes.submit.StagedTokens, want.StagedTokens) {
		t.Errorf("staged tokens：%v", e.writes.submit.StagedTokens)
	}
	e.writes.submit.StagedTokens = nil
	want.StagedTokens = nil
	if e.writes.submit.BuyerID != want.BuyerID || e.writes.submit.OrderNo != want.OrderNo || e.writes.submit.OrderItemID != want.OrderItemID || e.writes.submit.Rating != want.Rating || e.writes.submit.Content != want.Content {
		t.Errorf("輸入：%+v", e.writes.submit)
	}
	if resp.Msg.GetReview().GetOrderNo() != "ORD-1" || resp.Msg.GetReview().GetOrderItemId() != "501" {
		t.Errorf("作者的視圖帶訂單資訊：%v", resp.Msg.GetReview())
	}
	if _, err := e.app.CreateReview(signedIn(), connect.NewRequest(&pb.CreateReviewRequest{OrderNo: "ORD-1", OrderItemId: "x", Rating: 4})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("訂單項目 ID 格式：%v", err)
	}
}

func TestCreateReviewMapsModuleReasonsToTheContractReasons(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		code   connect.Code
		reason string
	}{
		{"已評價", withReason(apperr.AlreadyExists("已評價"), review.ReasonAlreadyExists), connect.CodeAlreadyExists, "REVIEW_ALREADY_EXISTS"},
		{"訂單不能評價", withReason(apperr.FailedPrecondition("未完成"), review.ReasonOrderNotReviewable), connect.CodeFailedPrecondition, "ORDER_NOT_REVIEWABLE"},
		{"超過期限", withReason(apperr.FailedPrecondition("過期"), review.ReasonWindowExpired), connect.CodeFailedPrecondition, "REVIEW_WINDOW_EXPIRED"},
		{"訂單不存在不帶 reason", withReason(apperr.NotFound("找不到"), review.ReasonOrderNotFound), connect.CodeNotFound, ""},
		{"內容被擋不帶 reason", withReason(apperr.InvalidInputOn([]string{"Content"}, "不合規"), review.ReasonModerationRejected), connect.CodeInvalidArgument, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t, nil)
			e.writes.err = c.err
			_, err := e.app.CreateReview(signedIn(), connect.NewRequest(&pb.CreateReviewRequest{OrderNo: "ORD-1", OrderItemId: "501", Rating: 4}))
			if codeOf(err) != c.code || reasonOf(err) != c.reason {
				t.Errorf("code=%v reason=%q，預期 %v %q", codeOf(err), reasonOf(err), c.code, c.reason)
			}
		})
	}
}

func TestReplyReviewMapsNotReplyableToTheContractReason(t *testing.T) {
	e := newEnv(t, ownStore())
	e.writes.err = withReason(apperr.FailedPrecondition("不能回覆"), review.ReasonNotReplyable)
	_, err := e.app.ReplyReview(signedIn(), connect.NewRequest(&pb.ReplyReviewRequest{ReviewId: (uuid.UUID{1}).String(), Content: "x"}))
	if codeOf(err) != connect.CodeFailedPrecondition || reasonOf(err) != "REVIEW_NOT_REPLYABLE" {
		t.Fatalf("%v %q", codeOf(err), reasonOf(err))
	}
}

func TestEveryModuleReasonTheClientBranchesOnIsMappedToAContractReason(t *testing.T) {
	want := map[review.Reason]string{
		review.ReasonAlreadyExists: "REVIEW_ALREADY_EXISTS", review.ReasonOrderNotReviewable: "ORDER_NOT_REVIEWABLE",
		review.ReasonWindowExpired: "REVIEW_WINDOW_EXPIRED", review.ReasonNotReplyable: "REVIEW_NOT_REPLYABLE",
	}
	if len(reasons) != len(want) {
		t.Fatalf("對照表 %d 項，預期 %d", len(reasons), len(want))
	}
	for r, name := range want {
		if got := reasons[r].String(); got != name {
			t.Errorf("%s 對到 %q，預期 %q", r, got, name)
		}
	}
}

func TestListMyReviewsFiltersByOrderItemAndScopesTheToken(t *testing.T) {
	e := newEnv(t, nil)
	next := review.Cursor{CreatedAt: at, ID: uuid.UUID{9}}
	e.reads.page = review.Page{Reviews: []review.Review{sampleReview(1)}, Next: &next}
	item := "501"
	resp, err := e.app.ListMyReviews(signedIn(), connect.NewRequest(&pb.ListMyReviewsRequest{OrderItemId: &item, PageSize: 5}))
	if err != nil {
		t.Fatal(err)
	}
	if e.reads.id != callerBuyer || e.reads.item != 501 || e.reads.size != 5 {
		t.Errorf("買家 %d 訂單項目 %d size %d", e.reads.id, e.reads.item, e.reads.size)
	}
	if resp.Msg.GetReviews()[0].GetOrderNo() != "ORD-1" {
		t.Error("作者的視圖帶訂單資訊")
	}
	token := resp.Msg.GetNextPageToken()
	if token == "" {
		t.Fatal("有下一頁要帶 token")
	}
	other := "502"
	if _, err := e.app.ListMyReviews(signedIn(), connect.NewRequest(&pb.ListMyReviewsRequest{OrderItemId: &other, PageToken: token})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("換了 order_item_id 不能沿用 token：%v", err)
	}
	if _, err := e.app.ListMyReviews(signedIn(), connect.NewRequest(&pb.ListMyReviewsRequest{})); err != nil || e.reads.item != 0 {
		t.Errorf("不帶篩選是全部：%v item=%d", err, e.reads.item)
	}
	bad := "x"
	if _, err := e.app.ListMyReviews(signedIn(), connect.NewRequest(&pb.ListMyReviewsRequest{OrderItemId: &bad})); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("order_item_id 格式：%v", err)
	}
}

func TestListOwnStoreReviewsDerivesTheStoreFromTheCallerAndDoesNotApplyAViewer(t *testing.T) {
	e := newEnv(t, ownStore())
	e.reads.page = review.Page{Reviews: []review.Review{sampleReview(1)}}
	rating := int32(2)
	resp, err := e.app.ListOwnStoreReviews(signedIn(), connect.NewRequest(&pb.ListOwnStoreReviewsRequest{Rating: &rating, WithImages: true, PageSize: 3}))
	if err != nil {
		t.Fatal(err)
	}
	if e.reads.calls[0] != "ListOwnStoreReviews" || e.reads.id != callerStoreID || e.reads.filter != (review.Filter{Rating: 2, WithImages: true}) || e.reads.size != 3 {
		t.Errorf("店家取自呼叫者此刻的店：%v id=%d filter=%+v", e.reads.calls, e.reads.id, e.reads.filter)
	}
	if resp.Msg.GetReviews()[0].GetOrderNo() != "ORD-1" {
		t.Error("店家自己的視圖帶訂單資訊")
	}
	m := pb.File_hawkr_review_v1alpha1_review_proto.Messages().ByName("ListOwnStoreReviewsRequest")
	if m.Fields().ByName("store_id") != nil {
		t.Error("ListOwnStoreReviews 不帶 store_id")
	}
}

func TestListOwnStoreReviewsWithoutAStoreIsNotFound(t *testing.T) {
	e := newEnv(t, nil)
	if _, err := e.app.ListOwnStoreReviews(signedIn(), connect.NewRequest(&pb.ListOwnStoreReviewsRequest{})); codeOf(err) != connect.CodeNotFound || len(e.reads.calls) != 0 {
		t.Fatalf("沒有店：%v calls=%v", err, e.reads.calls)
	}
	if _, err := e.app.ListOwnStoreReviews(asAdmin(), connect.NewRequest(&pb.ListOwnStoreReviewsRequest{})); codeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("管理員：%v", err)
	}
}

// 能力宣告表與 ReviewAppService 的方法一一對應：缺一個或多一個都不能啟動（NewApp 檢查的就是這個）。
func TestDeclarationsMatchTheServiceMethods(t *testing.T) {
	svc := serviceDescriptor()
	if err := checkDeclarations(declarations, svc); err != nil {
		t.Fatal(err)
	}
	for procedure := range declarations {
		short := make(map[string]declaration, len(declarations))
		for k, v := range declarations {
			if k != procedure {
				short[k] = v
			}
		}
		if err := checkDeclarations(short, svc); err == nil {
			t.Errorf("缺 %s 的宣告卻通過了檢查", procedure)
		}
		if _, err := newApp(short, &fakeReads{}, &fakeWrites{}, &fakeBuyers{}, fakeStores{}, images(t), slog.New(slog.DiscardHandler)); err == nil {
			t.Errorf("缺 %s 的宣告表要不能啟動", procedure)
		}
	}
	extra := map[string]declaration{"/hawkr.review.v1alpha1.ReviewAppService/Nope": exempt("x")}
	for k, v := range declarations {
		extra[k] = v
	}
	if err := checkDeclarations(extra, svc); err == nil {
		t.Error("多一個指向不存在方法的宣告卻通過了檢查")
	}
	for name, d := range map[string]declaration{
		"兩者皆無":   {},
		"兩者皆有":   {capability: seller.CapMarketing, exemptReason: "x"},
		"不存在的能力": requires("CAP_NOPE"),
	} {
		bad := map[string]declaration{}
		for k, v := range declarations {
			bad[k] = v
		}
		bad[reviewv1alpha1connect.ReviewAppServiceReplyReviewProcedure] = d
		if err := checkDeclarations(bad, svc); err == nil {
			t.Errorf("%s 的宣告通過了檢查", name)
		}
	}
}

func TestConstructorsRequireTheirDependencies(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	if _, err := NewPublic(&fakeReads{}, &fakeBuyers{}, images(t), nil); err == nil {
		t.Error("Public 需要 logger")
	}
	if _, err := NewPublic(&fakeReads{}, nil, images(t), log); err == nil {
		t.Error("Public 需要 Buyers")
	}
	if _, err := NewApp(&fakeReads{}, &fakeWrites{}, &fakeBuyers{}, nil, images(t), log); err == nil {
		t.Error("App 需要 Stores")
	}
}
