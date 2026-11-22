package seller

import (
	"context"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/seller/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	model "github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type blockModuleFake struct {
	fakeModule
	blocked   [][2]int64
	unblocked [][2]int64
	changed   bool
	page      model.BlockedBuyerPage
	input     model.BlockedBuyerListInput
}

func (f *blockModuleFake) BlockBuyer(_ context.Context, store, b int64) (bool, error) {
	f.blocked = append(f.blocked, [2]int64{store, b})
	return f.changed, nil
}
func (f *blockModuleFake) UnblockBuyer(_ context.Context, store, b int64) (bool, error) {
	f.unblocked = append(f.unblocked, [2]int64{store, b})
	return f.changed, nil
}
func (f *blockModuleFake) ListBlockedBuyers(_ context.Context, in model.BlockedBuyerListInput) (model.BlockedBuyerPage, error) {
	f.input = in
	return f.page, nil
}

type authorFake struct {
	author social.ContentAuthor
	got    social.Caller
}

func (f *authorFake) ContentAuthorFor(_ context.Context, c social.Caller, _ social.Target) (social.ContentAuthor, error) {
	f.got = c
	return f.author, nil
}

type reviewFake struct {
	r         review.Review
	err       error
	gotViewer int64
}

func (f *reviewFake) GetReview(_ context.Context, _ uuid.UUID, viewer int64) (review.Review, error) {
	f.gotViewer = viewer
	return f.r, f.err
}

type accountsOf map[int64]int64

func (a accountsOf) AccountIDByBuyerID(_ context.Context, id int64) (int64, error) {
	if v, ok := a[id]; ok {
		return v, nil
	}
	return 0, apperr.NotFound("no buyer")
}

type storesOf map[int64]int64

func (s storesOf) Store(_ context.Context, id int64) (model.Store, error) {
	if v, ok := s[id]; ok {
		return model.Store{StoreID: id, AccountID: v}, nil
	}
	return model.Store{}, apperr.NotFound("no store")
}

// blockEnv：呼叫者店是 11（帳號 7）；買家 70 是帳號 7 自己的買家身分，買家 80 是別人，買家 81 已關閉。
type blockEnv struct {
	h       *App
	m       *blockModuleFake
	authors *authorFake
	f       *fakeFollowers
	reviews *reviewFake
	ctx     context.Context
}

func newBlockEnv(t *testing.T) *blockEnv {
	t.Helper()
	name := "阿美"
	m := &blockModuleFake{fakeModule: fakeModule{}, changed: true}
	authors := &authorFake{}
	reviews := &reviewFake{err: apperr.NotFound("no review")}
	f := &fakeFollowers{displays: []buyer.Display{{BuyerID: 70, DisplayName: &name}, {BuyerID: 80, DisplayName: &name}, {BuyerID: 81, DisplayName: &name, Deleted: true}}}
	images, err := rpc.NewImages("http://localhost:8787")
	if err != nil {
		t.Fatal(err)
	}
	src := blocks.Sources{Authors: authors, Reviews: reviews, Identities: blocks.Identities{Buyers: accountsOf{70: 7, 80: 8, 81: 9}, Stores: storesOf{11: 7}}}
	h, err := NewApp(m, &paymentModule{}, f, images, Dashboard{}, testStats(fakeFollowerCounts{}, fakeRatings{}), fakeLinks{}, mustWebOrigin(t), src, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &blockEnv{h: h, m: m, authors: authors, f: f, reviews: reviews, ctx: ctxWithStore(currentStore())}
}

func commentRef() *pb.BlockBuyerRequest {
	return &pb.BlockBuyerRequest{Source: &pb.BlockBuyerRequest_CommentId{CommentId: uuid.UUID{9}.String()}}
}

func buyerAuthor(id, storeID int64) social.ContentAuthor {
	return social.ContentAuthor{Found: true, Author: social.Actor{Kind: social.ActorBuyer, ID: id}, StoreID: storeID}
}

func TestBlockBuyerBlocksTheAuthorOfAReferenceInTheCallersStore(t *testing.T) {
	for _, changed := range []bool{true, false} {
		e := newBlockEnv(t)
		e.m.changed = changed
		e.authors.author = buyerAuthor(80, 11)
		resp, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(commentRef()))
		if err != nil || resp.Msg.GetChanged() != changed {
			t.Fatalf("changed=%v resp=%v err=%v", changed, resp, err)
		}
		if len(e.m.blocked) != 1 || e.m.blocked[0] != [2]int64{11, 80} {
			t.Fatalf("要以店 11 封鎖買家 80：%+v", e.m.blocked)
		}
		if e.authors.got.Store == nil || e.authors.got.Store.StoreID != 11 || e.authors.got.BuyerID != 0 {
			t.Fatalf("解析要以店主身分讀：%+v", e.authors.got)
		}
	}
}

func TestBlockBuyerRejectsTheOwnersOwnBuyerIdentity(t *testing.T) {
	e := newBlockEnv(t)
	e.authors.author = buyerAuthor(70, 11) // 帳號 7 的買家身分，帳號 7 也擁有店 11
	_, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(commentRef()))
	if apperr.KindOf(err) != apperr.KindInvalidInput || len(e.m.blocked) != 0 {
		t.Fatalf("店家封鎖店主本人的買家身分要 invalid_argument、不寫入：%v %+v", err, e.m.blocked)
	}
}

func TestBlockBuyerRejectsAStoreAuthorAndUnreadableOrForeignReferences(t *testing.T) {
	e := newBlockEnv(t)
	e.authors.author = social.ContentAuthor{Found: true, Author: social.Actor{Kind: social.ActorStore, ID: 11}, StoreID: 11}
	if _, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(commentRef())); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("作者不是買家：%v", err)
	}
	e.authors.author = social.ContentAuthor{}
	if _, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(commentRef())); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("讀不到：%v", err)
	}
	e.authors.author = buyerAuthor(80, 12) // 別家店的內容
	if _, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(commentRef())); apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("別家店的內容與讀不到同一個答案：%v", err)
	}
	if len(e.m.blocked) != 0 {
		t.Fatalf("被拒絕的請求不能寫入：%+v", e.m.blocked)
	}
}

func TestBlockBuyerChecksThatTheTargetBuyerExists(t *testing.T) {
	e := newBlockEnv(t)
	e.authors.author = buyerAuthor(81, 11) // 已關閉
	if _, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(commentRef())); apperr.KindOf(err) != apperr.KindNotFound || len(e.m.blocked) != 0 {
		t.Fatalf("已關閉的買家不能封鎖：%v %+v", err, e.m.blocked)
	}
}

func TestUnblockBuyerIsIdempotent(t *testing.T) {
	e := newBlockEnv(t)
	e.m.changed = false
	resp, err := e.h.UnblockBuyer(e.ctx, connect.NewRequest(&pb.UnblockBuyerRequest{BuyerId: "80"}))
	if err != nil || resp.Msg.GetChanged() || e.m.unblocked[0] != [2]int64{11, 80} {
		t.Fatalf("%v %v %+v", resp, err, e.m.unblocked)
	}
}

func TestListBlockedBuyersShowsClosedBuyersAsDeletedWithOneBatchLookup(t *testing.T) {
	e := newBlockEnv(t)
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	e.m.page = model.BlockedBuyerPage{
		Items: []model.BlockedBuyer{{BuyerID: 80, BlockedAt: at}, {BuyerID: 81, BlockedAt: at.Add(-time.Hour)}, {BuyerID: 82, BlockedAt: at.Add(-2 * time.Hour)}},
		Next:  &model.BlockedBuyerCursor{Time: at.Add(-2 * time.Hour), BuyerID: 82},
	}
	resp, err := e.h.ListBlockedBuyers(e.ctx, connect.NewRequest(&pb.ListBlockedBuyersRequest{PageSize: 3}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetBlockedBuyers()
	if len(got) != 3 || e.m.input.StoreID != 11 || e.m.input.PageSize != 3 {
		t.Fatalf("rows=%d in=%+v", len(got), e.m.input)
	}
	if b := got[0].GetBuyer(); b.GetDeleted() || b.GetDisplayName() != "阿美" {
		t.Fatalf("存活的買家：%v", b)
	}
	for _, i := range []int{1, 2} {
		if b := got[i].GetBuyer(); !b.GetDeleted() || b.GetDisplayName() != "" || b.GetAvatar() != nil || b.GetId() == "" {
			t.Fatalf("已關閉或查不到的買家要顯示成已刪除：%v", b)
		}
	}
	if _, err := e.h.ListBlockedBuyers(e.ctx, connect.NewRequest(&pb.ListBlockedBuyersRequest{PageToken: resp.Msg.GetNextPageToken()})); err != nil || e.m.input.After == nil || e.m.input.After.BuyerID != 82 {
		t.Fatalf("游標：%v %+v", err, e.m.input.After)
	}
	// page token 只對同一家店有效。
	other := currentStore()
	other.StoreID = 12
	if _, err := e.h.ListBlockedBuyers(ctxWithStore(other), connect.NewRequest(&pb.ListBlockedBuyersRequest{PageToken: resp.Msg.GetNextPageToken()})); err == nil {
		t.Fatal("別家店的 token 要被拒絕")
	}
}

func reviewRef() *pb.BlockBuyerRequest {
	return &pb.BlockBuyerRequest{Source: &pb.BlockBuyerRequest_ReviewId{ReviewId: uuid.UUID{9}.String()}}
}

func TestBlockBuyerByReviewBlocksTheReviewerInTheCallersStore(t *testing.T) {
	e := newBlockEnv(t)
	e.reviews.err = nil
	e.reviews.r = review.Review{ID: uuid.UUID{9}, BuyerID: 80, StoreID: 11}
	resp, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(reviewRef()))
	if err != nil || !resp.Msg.GetChanged() {
		t.Fatalf("%v %v", resp, err)
	}
	if len(e.m.blocked) != 1 || e.m.blocked[0] != [2]int64{11, 80} {
		t.Fatalf("要以店 11 封鎖寫評價的買家 80：%+v", e.m.blocked)
	}
	if e.reviews.gotViewer != 0 || e.authors.got != (social.Caller{}) {
		t.Fatalf("店家以訪客的公開規則讀評價、不問 social：觀看者 %d social %+v", e.reviews.gotViewer, e.authors.got)
	}
}

func TestBlockBuyerByAnUnreadableReviewIsNotFound(t *testing.T) {
	e := newBlockEnv(t)
	if _, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(reviewRef())); apperr.KindOf(err) != apperr.KindNotFound || len(e.m.blocked) != 0 {
		t.Fatalf("看不到的評價要 not_found、不寫入：%v %+v", err, e.m.blocked)
	}
}

func TestBlockBuyerByAnotherStoresReviewIsNotFound(t *testing.T) {
	e := newBlockEnv(t)
	e.reviews.err = nil
	e.reviews.r = review.Review{ID: uuid.UUID{9}, BuyerID: 80, StoreID: 12}
	if _, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(reviewRef())); apperr.KindOf(err) != apperr.KindNotFound || len(e.m.blocked) != 0 {
		t.Fatalf("別家店的評價與讀不到同一個答案：%v %+v", err, e.m.blocked)
	}
}

func TestBlockBuyerByAReviewWrittenByTheOwnersBuyerIdentityIsRejected(t *testing.T) {
	e := newBlockEnv(t)
	e.reviews.err = nil
	e.reviews.r = review.Review{ID: uuid.UUID{9}, BuyerID: 70, StoreID: 11} // 帳號 7 的買家身分，帳號 7 也擁有店 11
	if _, err := e.h.BlockBuyer(e.ctx, connect.NewRequest(reviewRef())); apperr.KindOf(err) != apperr.KindInvalidInput || len(e.m.blocked) != 0 {
		t.Fatalf("店家封鎖店主本人的買家身分要 invalid_argument、不寫入：%v %+v", err, e.m.blocked)
	}
}
