package buyer

import (
	"context"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	"github.com/Will413028/hawkr/backend/internal/httpapi/blocks"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/buyer/v1alpha1"
	commonpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/common/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	model "github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// blockFake 記下封鎖寫入與批次查詢次數；名單與顯示資料由欄位給。
type blockFake struct {
	fakeModule
	blockedBuyers  [][2]int64
	blockedStores  [][2]int64
	changed        bool
	buyerPage      model.BlockedBuyerPage
	storePage      model.BlockedStorePage
	display        []model.Display
	displayCalls   int
	gotBuyerInput  model.BlockedBuyerListInput
	unblockedStore [][2]int64
}

func (f *blockFake) BlockBuyer(_ context.Context, b, t int64) (bool, error) {
	f.blockedBuyers = append(f.blockedBuyers, [2]int64{b, t})
	return f.changed, nil
}
func (f *blockFake) BlockStore(_ context.Context, b, s int64) (bool, error) {
	f.blockedStores = append(f.blockedStores, [2]int64{b, s})
	return f.changed, nil
}
func (f *blockFake) UnblockBuyer(context.Context, int64, int64) (bool, error) { return f.changed, nil }
func (f *blockFake) UnblockStore(_ context.Context, b, s int64) (bool, error) {
	f.unblockedStore = append(f.unblockedStore, [2]int64{b, s})
	return f.changed, nil
}
func (f *blockFake) ListBlockedBuyers(_ context.Context, in model.BlockedBuyerListInput) (model.BlockedBuyerPage, error) {
	f.gotBuyerInput = in
	return f.buyerPage, nil
}
func (f *blockFake) ListBlockedStores(context.Context, model.BlockedStoreListInput) (model.BlockedStorePage, error) {
	return f.storePage, nil
}
func (f *blockFake) BatchDisplay(context.Context, []int64) ([]model.Display, error) {
	f.displayCalls++
	return f.display, nil
}

type fakeAuthors struct {
	author social.ContentAuthor
	got    social.Caller
}

func (f *fakeAuthors) ContentAuthorFor(_ context.Context, c social.Caller, _ social.Target) (social.ContentAuthor, error) {
	f.got = c
	return f.author, nil
}

type accountsOf map[int64]int64

func (a accountsOf) AccountIDByBuyerID(_ context.Context, id int64) (int64, error) {
	if v, ok := a[id]; ok {
		return v, nil
	}
	return 0, apperr.NotFound("no buyer")
}

type storesOf map[int64]int64

func (s storesOf) Store(_ context.Context, id int64) (seller.Store, error) {
	if v, ok := s[id]; ok {
		return seller.Store{StoreID: id, AccountID: v}, nil
	}
	return seller.Store{}, apperr.NotFound("no store")
}

const callerBuyerID = 5007

// blockEnv 是封鎖測試的環境：呼叫者是帳號 1、買家 5007；帳號 1 擁有店 10，帳號 2 擁有店 20 與買家 6.
type blockEnv struct {
	h       *App
	m       *blockFake
	authors *fakeAuthors
	ctx     context.Context
}

func newBlockEnv(t *testing.T, cards ...seller.StoreCard) *blockEnv {
	t.Helper()
	m := &blockFake{fakeModule: fakeModule{profile: model.Profile{BuyerID: callerBuyerID, AccountID: 1}}}
	authors := &fakeAuthors{}
	src := blocks.Sources{Authors: authors, Identities: blocks.Identities{
		Buyers: accountsOf{callerBuyerID: 1, 6: 2},
		Stores: storesOf{10: 1, 20: 2},
	}}
	images, err := rpc.NewImages("http://localhost:8787")
	if err != nil {
		t.Fatal(err)
	}
	h := mustAppWith(t, m, images, Ports{Stores: fakeStores{cards: cards}}, src)
	ctx := rpc.WithPrincipal(context.Background(), identity.Principal{Kind: identity.PrincipalAccount, ID: 1})
	return &blockEnv{h: h, m: m, authors: authors, ctx: ctx}
}

func card(id int64, status seller.CardStatus) seller.StoreCard {
	name := "店"
	return seller.StoreCard{StoreID: id, StoreCode: id, Name: &name, Status: status}
}

func commentRef() *pb.BlockAuthorRequest {
	return &pb.BlockAuthorRequest{Source: &pb.BlockAuthorRequest_CommentId{CommentId: uuid.UUID{9}.String()}}
}

func TestBlockAuthorBlocksABuyerAuthorAndReportsWhetherStateChanged(t *testing.T) {
	for _, changed := range []bool{true, false} {
		e := newBlockEnv(t)
		e.m.changed = changed
		e.authors.author = social.ContentAuthor{Found: true, Author: social.Actor{Kind: social.ActorBuyer, ID: 6}, StoreID: 20}
		resp, err := e.h.BlockAuthor(e.ctx, connect.NewRequest(commentRef()))
		if err != nil || resp.Msg.GetChanged() != changed {
			t.Fatalf("changed=%v resp=%v err=%v", changed, resp, err)
		}
		if len(e.m.blockedBuyers) != 1 || e.m.blockedBuyers[0] != [2]int64{callerBuyerID, 6} || len(e.m.blockedStores) != 0 {
			t.Fatalf("要以呼叫者的買家身分封鎖買家 6：%+v %+v", e.m.blockedBuyers, e.m.blockedStores)
		}
		if e.authors.got.BuyerID != callerBuyerID || e.authors.got.Store != nil {
			t.Fatalf("解析要以呼叫者的買家身分讀：%+v", e.authors.got)
		}
	}
}

func TestBlockAuthorBlocksAStoreAuthorAsAStore(t *testing.T) {
	e := newBlockEnv(t, card(20, seller.CardOpen))
	e.m.changed = true
	e.authors.author = social.ContentAuthor{Found: true, Author: social.Actor{Kind: social.ActorStore, ID: 20}, StoreID: 20}
	if _, err := e.h.BlockAuthor(e.ctx, connect.NewRequest(commentRef())); err != nil {
		t.Fatal(err)
	}
	if len(e.m.blockedStores) != 1 || e.m.blockedStores[0] != [2]int64{callerBuyerID, 20} || len(e.m.blockedBuyers) != 0 {
		t.Fatalf("要封鎖店 20：%+v %+v", e.m.blockedBuyers, e.m.blockedStores)
	}
}

func TestBlockAuthorOfAnUnreadableReferenceIsNotFoundAndWritesNothing(t *testing.T) {
	e := newBlockEnv(t)
	_, err := e.h.BlockAuthor(e.ctx, connect.NewRequest(commentRef()))
	if apperr.KindOf(err) != apperr.KindNotFound {
		t.Fatalf("讀不到的參照要 not_found：%v", err)
	}
	if len(e.m.blockedBuyers)+len(e.m.blockedStores) != 0 {
		t.Fatal("讀不到的參照不能寫入")
	}
}

func TestBlockingTheCallersOwnStoreIsRejectedOnEveryEntryPoint(t *testing.T) {
	e := newBlockEnv(t, card(10, seller.CardOpen))
	// 從店面頁封鎖。
	_, err := e.h.BlockStore(e.ctx, connect.NewRequest(&pb.BlockStoreRequest{StoreId: "10"}))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("封鎖自己的店要 invalid_argument：%v", err)
	}
	// 從自己店的留言封鎖（作者是店家 10）。
	e.authors.author = social.ContentAuthor{Found: true, Author: social.Actor{Kind: social.ActorStore, ID: 10}, StoreID: 10}
	_, err = e.h.BlockAuthor(e.ctx, connect.NewRequest(commentRef()))
	if apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("從留言封鎖自己的店要 invalid_argument：%v", err)
	}
	if len(e.m.blockedStores) != 0 {
		t.Fatalf("被拒絕的請求不能寫入：%+v", e.m.blockedStores)
	}
}

func TestBlockStoreRejectsAStoreWithoutACardWithoutRevealingItExists(t *testing.T) {
	e := newBlockEnv(t) // 沒有任何店家卡片：不存在或從沒公開過
	_, err := e.h.BlockStore(e.ctx, connect.NewRequest(&pb.BlockStoreRequest{StoreId: "20"}))
	if apperr.KindOf(err) != apperr.KindNotFound || len(e.m.blockedStores) != 0 {
		t.Fatalf("err=%v writes=%+v", err, e.m.blockedStores)
	}
}

func TestBlockStoreAndUnblocksReportChange(t *testing.T) {
	e := newBlockEnv(t, card(20, seller.CardOpen))
	e.m.changed = true
	bs, err := e.h.BlockStore(e.ctx, connect.NewRequest(&pb.BlockStoreRequest{StoreId: "20"}))
	if err != nil || !bs.Msg.GetChanged() {
		t.Fatalf("%v %v", bs, err)
	}
	us, err := e.h.UnblockStore(e.ctx, connect.NewRequest(&pb.UnblockStoreRequest{StoreId: "20"}))
	if err != nil || !us.Msg.GetChanged() || e.m.unblockedStore[0] != [2]int64{callerBuyerID, 20} {
		t.Fatalf("%v %v %+v", us, err, e.m.unblockedStore)
	}
	e.m.changed = false
	ub, err := e.h.UnblockBuyer(e.ctx, connect.NewRequest(&pb.UnblockBuyerRequest{BuyerId: "6"}))
	if err != nil || ub.Msg.GetChanged() {
		t.Fatalf("冪等：本來就沒封鎖 changed 要是 false：%v %v", ub, err)
	}
	if _, err := e.h.UnblockBuyer(e.ctx, connect.NewRequest(&pb.UnblockBuyerRequest{BuyerId: "x"})); apperr.KindOf(err) != apperr.KindInvalidInput {
		t.Fatalf("ID 格式錯誤：%v", err)
	}
}

func TestListBlockedBuyersShowsClosedBuyersAsDeletedWithOneBatchLookup(t *testing.T) {
	e := newBlockEnv(t)
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	name := "阿美"
	e.m.buyerPage = model.BlockedBuyerPage{
		Items: []model.BlockedBuyerItem{{BuyerID: 6, Time: at}, {BuyerID: 7, Time: at.Add(-time.Hour)}, {BuyerID: 8, Time: at.Add(-2 * time.Hour)}},
		Next:  &model.BlockedBuyerCursor{Time: at.Add(-2 * time.Hour), BuyerID: 8},
	}
	// 6 存活、7 已關閉（有個資也不能出現）、8 查不到。
	e.m.display = []model.Display{{BuyerID: 6, DisplayName: &name}, {BuyerID: 7, DisplayName: &name, Deleted: true}}
	resp, err := e.h.ListBlockedBuyers(e.ctx, connect.NewRequest(&pb.ListBlockedBuyersRequest{PageSize: 3}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetBlockedBuyers()
	if e.m.displayCalls != 1 || len(got) != 3 || e.m.gotBuyerInput.BuyerID != callerBuyerID || e.m.gotBuyerInput.PageSize != 3 {
		t.Fatalf("批次一次、帶呼叫者與頁大小：calls=%d rows=%d in=%+v", e.m.displayCalls, len(got), e.m.gotBuyerInput)
	}
	if got[0].GetDeleted() || got[0].GetDisplayName() != "阿美" {
		t.Fatalf("存活的買家：%v", got[0])
	}
	for _, i := range []int{1, 2} {
		if !got[i].GetDeleted() || got[i].GetDisplayName() != "" || got[i].GetAvatar() != nil || got[i].GetBuyerId() == "" {
			t.Fatalf("已關閉的買家要顯示成已刪除（沒有名稱與頭像）、仍有 id 可解除：%v", got[i])
		}
	}
	if resp.Msg.GetNextPageToken() == "" {
		t.Fatal("有下一頁要帶 token")
	}
	// token 內容與 scope 往返，下一頁的游標傳進模組。
	if _, err := e.h.ListBlockedBuyers(e.ctx, connect.NewRequest(&pb.ListBlockedBuyersRequest{PageToken: resp.Msg.GetNextPageToken()})); err != nil || e.m.gotBuyerInput.After == nil || e.m.gotBuyerInput.After.BuyerID != 8 {
		t.Fatalf("游標：%v %+v", err, e.m.gotBuyerInput.After)
	}
	if _, err := e.h.ListBlockedBuyers(e.ctx, connect.NewRequest(&pb.ListBlockedBuyersRequest{PageToken: "garbage"})); err == nil || connect.CodeOf(rpc.ConnectError(e.ctx, err)) != connect.CodeInvalidArgument {
		t.Fatalf("壞 token：%v", err)
	}
}

func TestListBlockedStoresShowsATerminatedStoreAsClosed(t *testing.T) {
	e := newBlockEnv(t, card(20, seller.CardOpen), card(21, seller.CardClosed))
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	e.m.storePage = model.BlockedStorePage{Items: []model.BlockedStoreItem{{StoreID: 20, Time: at}, {StoreID: 21, Time: at.Add(-time.Hour)}, {StoreID: 22, Time: at.Add(-2 * time.Hour)}}}
	resp, err := e.h.ListBlockedStores(e.ctx, connect.NewRequest(&pb.ListBlockedStoresRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	got := resp.Msg.GetBlockedStores()
	if len(got) != 3 {
		t.Fatalf("所有封鎖的列都在名單裡：%d", len(got))
	}
	if got[0].GetStore().GetStatus() != commonpb.StoreCardStatus_STORE_CARD_STATUS_OPEN || got[1].GetStore().GetStatus() != commonpb.StoreCardStatus_STORE_CARD_STATUS_CLOSED {
		t.Fatalf("營業中與已關閉的店：%v %v", got[0].GetStore(), got[1].GetStore())
	}
	if got[2].GetStore() != nil || got[2].GetStoreId() != "22" {
		t.Fatalf("沒有卡片的店不顯示卡片但仍可解除：%v", got[2])
	}
}

func TestEveryBlockEntryPointRequiresTheBuyerIdentity(t *testing.T) {
	e := newBlockEnv(t)
	anon := context.Background()
	_, err1 := e.h.BlockStore(anon, connect.NewRequest(&pb.BlockStoreRequest{StoreId: "20"}))
	_, err2 := e.h.BlockAuthor(anon, connect.NewRequest(commentRef()))
	_, err3 := e.h.ListBlockedBuyers(anon, connect.NewRequest(&pb.ListBlockedBuyersRequest{}))
	for i, err := range []error{err1, err2, err3} {
		if err == nil || connect.CodeOf(rpc.ConnectError(anon, err)) != connect.CodePermissionDenied {
			t.Fatalf("入口 %d 沒有買家身分要 permission_denied：%v", i, err)
		}
	}
}

// 重複封鎖同一位作者（重試）第二次成功且 changed 為假：解析不因為自己的封鎖而失敗。
func TestBlockAuthorTwiceSucceedsAndTheSecondCallChangesNothing(t *testing.T) {
	e := newBlockEnv(t)
	e.authors.author = social.ContentAuthor{Found: true, Author: social.Actor{Kind: social.ActorBuyer, ID: 6}, StoreID: 20}
	for i, want := range []bool{true, false} {
		e.m.changed = want
		resp, err := e.h.BlockAuthor(e.ctx, connect.NewRequest(commentRef()))
		if err != nil || resp.Msg.GetChanged() != want {
			t.Fatalf("第 %d 次：changed=%v resp=%v err=%v", i+1, want, resp, err)
		}
	}
	if len(e.m.blockedBuyers) != 2 {
		t.Fatalf("兩次都要走封鎖入口：%+v", e.m.blockedBuyers)
	}
}
