//go:build integration

package notification

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// fakeResolver 取代 seller 那一半：買家的帳號、封鎖與追蹤者讀真的 buyer 模組，店家只有「存活的店與它的帳號」和「店封鎖買家」。
type fakeResolver struct {
	buyers        *buyer.Module
	mu            sync.Mutex
	storeAccounts map[int64]int64
	storeBlocks   map[[2]int64]bool
}

func (f *fakeResolver) addStore(storeID, account int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.storeAccounts[storeID] = account
}

func (f *fakeResolver) storeBlocksBuyer(storeID, buyerID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.storeBlocks[[2]int64{storeID, buyerID}] = true
}

func (f *fakeResolver) BuyerAccounts(ctx context.Context, ids []int64) (map[int64]int64, error) {
	return f.buyers.ResolvePrincipals(ctx, ids)
}

func (f *fakeResolver) StoreAccounts(_ context.Context, ids []int64) (map[int64]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int64]int64{}
	for _, id := range ids {
		if a, ok := f.storeAccounts[id]; ok {
			out[id] = a
		}
	}
	return out, nil
}

func (f *fakeResolver) BuyersBlockedBetween(ctx context.Context, a, b int64) (bool, error) {
	return f.buyers.IsBlockedBetween(ctx, a, b)
}

func (f *fakeResolver) BuyerBlocksStore(ctx context.Context, buyerID, storeID int64) (bool, error) {
	return f.buyers.IsBlockingStore(ctx, buyerID, storeID)
}

func (f *fakeResolver) StoreBlocksBuyer(_ context.Context, storeID, buyerID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.storeBlocks[[2]int64{storeID, buyerID}], nil
}

func (f *fakeResolver) StoreFollowers(ctx context.Context, storeID, after int64, limit int) ([]registry.FollowerRef, error) {
	rows, err := f.buyers.ListLiveFollowerAccounts(ctx, storeID, after, limit)
	if err != nil {
		return nil, err
	}
	out := make([]registry.FollowerRef, len(rows))
	for i, r := range rows {
		out[i] = registry.FollowerRef{BuyerID: r.BuyerID, AccountID: r.AccountID}
	}
	return out, nil
}

// 測試專用型別：真正的型別由 A8-3b、A8-3c 加入。四個型別各練一種解析：店家收件（STORE，actor 是買家）、買家收件（BUYER，actor 可以是買家或店）、
// 母意圖（對追蹤者 fan-out）與它的子型別（Spawned，BUYER，actor 是店）。
type storeCommentParams struct {
	EventID      string `json:"event_id"`
	StoreID      int64  `json:"store_id"`
	ActorBuyerID int64  `json:"actor_buyer_id"`
}

type buyerReplyParams struct {
	EventID          string         `json:"event_id"`
	RecipientBuyerID int64          `json:"recipient_buyer_id"`
	ActorSide        model.Audience `json:"actor_side"`
	ActorID          int64          `json:"actor_id"`
}

type postParams struct {
	EventID string `json:"event_id"`
	StoreID int64  `json:"store_id"`
}

type followerItemParams struct {
	StoreID int64 `json:"store_id"`
	BuyerID int64 `json:"buyer_id"`
}

func plainText[P any](l uitext.Locale, _ P) (registry.Content, error) {
	return registry.Content{Title: uitext.Message{Text: "t", Locale: l}, Body: uitext.Message{Text: "b", Locale: l}}, nil
}

var (
	testStoreComment = registry.Define(registry.Definition[storeCommentParams]{
		Name: "test_store_comment", Audiences: []model.Audience{model.AudienceStore}, Category: model.CategoryOrder, Presentation: model.PresentationOrder,
		Channels:          []model.Channel{model.ChannelInbox},
		IdempotencySource: "test.StoreComment.EventID",
		IdempotencyKey:    func(p storeCommentParams) string { return p.EventID },
		OccurredAt:        func(storeCommentParams) time.Time { return epoch },
		Expand: func(ctx context.Context, r registry.Resolver, p storeCommentParams) (*registry.Expansion, error) {
			return registry.StoreRecipient(ctx, r, p.StoreID, registry.Principal{Side: model.AudienceBuyer, ID: p.ActorBuyerID})
		},
		Text:    plainText[storeCommentParams],
		Example: storeCommentParams{EventID: "e", StoreID: 1, ActorBuyerID: 2},
	})

	testBuyerReply = registry.Define(registry.Definition[buyerReplyParams]{
		Name: "test_buyer_reply", Audiences: []model.Audience{model.AudienceBuyer}, Category: model.CategoryLike, Presentation: model.PresentationSocial,
		Channels:          []model.Channel{model.ChannelInbox},
		IdempotencySource: "test.BuyerReply.EventID",
		IdempotencyKey:    func(p buyerReplyParams) string { return p.EventID },
		OccurredAt:        func(buyerReplyParams) time.Time { return epoch },
		Expand: func(ctx context.Context, r registry.Resolver, p buyerReplyParams) (*registry.Expansion, error) {
			return registry.BuyerRecipient(ctx, r, p.RecipientBuyerID, registry.Principal{Side: p.ActorSide, ID: p.ActorID})
		},
		Text:    plainText[buyerReplyParams],
		Example: buyerReplyParams{EventID: "e", RecipientBuyerID: 1, ActorSide: model.AudienceStore, ActorID: 2},
	})

	testFollowPost = registry.Define(registry.Definition[postParams]{
		Name:              "test_follow_post",
		IdempotencySource: "test.FollowPost.EventID",
		IdempotencyKey:    func(p postParams) string { return p.EventID },
		OccurredAt:        func(postParams) time.Time { return epoch },
		Broadcast: &registry.Broadcast[postParams]{
			Child: "test_follow_post_item",
			Targets: func(ctx context.Context, r registry.Resolver, p postParams, after int64, limit int) ([]registry.Target, error) {
				page, err := r.StoreFollowers(ctx, p.StoreID, after, limit)
				if err != nil {
					return nil, err
				}
				out := make([]registry.Target, len(page))
				for i, f := range page {
					out[i] = registry.Target{ID: f.BuyerID, Params: followerItemParams{StoreID: p.StoreID, BuyerID: f.BuyerID}}
				}
				return out, nil
			},
		},
		Example: postParams{EventID: "e", StoreID: 1},
	})

	testFollowPostItem = registry.Define(registry.Definition[followerItemParams]{
		Name: "test_follow_post_item", Audiences: []model.Audience{model.AudienceBuyer}, Category: model.CategoryLike, Presentation: model.PresentationSocial,
		Channels: []model.Channel{model.ChannelInbox}, Spawned: true,
		Expand: func(ctx context.Context, r registry.Resolver, p followerItemParams) (*registry.Expansion, error) {
			return registry.BuyerRecipient(ctx, r, p.BuyerID, registry.Principal{Side: model.AudienceStore, ID: p.StoreID})
		},
		Text:    plainText[followerItemParams],
		Example: followerItemParams{StoreID: 1, BuyerID: 2},
	})
)

var resolutionTestTypes = func() map[string]registry.Entry {
	out := map[string]registry.Entry{}
	for _, e := range resolutionTestEntries() {
		out[e.Spec().Name] = e
	}
	return out
}()

func resolutionTestEntries() []registry.Entry {
	return []registry.Entry{testStoreComment, testBuyerReply, testFollowPost, testFollowPostItem}
}

func (f *fixture) buyerID(t *testing.T, account int64) int64 {
	t.Helper()
	p, err := f.buyer.Profile(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	return p.BuyerID
}

func (f *fixture) newBuyer(t *testing.T) (account, buyerID int64) {
	t.Helper()
	account = f.newAccount(t)
	return account, f.buyerID(t, account)
}

// ok 吃下一個 (bool, error) 的結果並在錯誤時中止：ok(t)(f.buyer.BlockBuyer(...))。
func ok(t *testing.T) func(bool, error) {
	t.Helper()
	return func(_ bool, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
}

// runAll 展開排著的 process 與 fan-out job，直到沒有為止；回傳執行的 fan-out 頁數與 process 數。
func (f *fixture) runAll(t *testing.T, svc *intake.Service) (pages []intake.FanoutArgs, processed int) {
	t.Helper()
	for {
		fan, _ := f.jobs.takeFanouts()
		proc := f.jobs.take()
		if len(fan) == 0 && len(proc) == 0 {
			return pages, processed
		}
		for _, a := range fan {
			if err := svc.Fanout(context.Background(), a); err != nil {
				t.Fatal(err)
			}
			pages = append(pages, a)
		}
		for _, a := range proc {
			if err := svc.Process(context.Background(), a.IntentID); err != nil {
				t.Fatal(err)
			}
			processed++
		}
	}
}

func (f *fixture) intakeWithChunk(chunk int) *intake.Service {
	return intake.New(intake.Deps{
		DB: f.db, Accounts: f.identity, Buyers: f.buyer, Stores: f.stores, Registry: testRegistry{}, Resolver: f.resolver, Jobs: f.jobs,
		FanoutChunk: chunk, Now: time.Now, NewID: uuid.NewV7, Logger: slog.New(slog.DiscardHandler),
	})
}

// 事件只帶 store_id 與買家 ID：worker 把店解析成擁有者帳號，寫進該店（STORE 受眾）的收件匣；查不到存活店家的意圖不寫、參數清成 {}；
// 店主自己對自己的店留言是自己觸發的事，被略過。
func TestStoreEventResolvesToTheOwnerAccountAndStore(t *testing.T) {
	f := setupIntake(t)
	owner, ownerBuyer := f.newBuyer(t)
	_, actorBuyer := f.newBuyer(t)
	f.resolver.addStore(7, owner)

	f.accept(t, "test_store_comment", storeCommentParams{EventID: "c1", StoreID: 7, ActorBuyerID: actorBuyer})
	f.run(t)
	var audience string
	var storeID *int64
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT audience, store_id FROM notification.inbox_items WHERE account_id = $1`, owner).Scan(&audience, &storeID); err != nil {
		t.Fatalf("店主沒有收件匣列：%v", err)
	}
	if audience != "STORE" || storeID == nil || *storeID != 7 {
		t.Fatalf("收件匣列的受眾與店 = %s %v，預期 STORE 與店 7", audience, storeID)
	}

	f.accept(t, "test_store_comment", storeCommentParams{EventID: "gone", StoreID: 8, ActorBuyerID: actorBuyer})
	f.run(t)
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items`); n != 1 {
		t.Fatalf("查不到存活店家的意圖寫了列，共 %d 列", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.intents WHERE idempotency_key = 'test_store_comment:gone' AND params = '{}'::jsonb`); n != 1 {
		t.Fatal("沒有收件人的意圖參數要清成 {}")
	}

	f.accept(t, "test_store_comment", storeCommentParams{EventID: "self", StoreID: 7, ActorBuyerID: ownerBuyer})
	f.run(t)
	if n := f.inboxCount(t, owner); n != 1 {
		t.Fatalf("店主對自己的店留言又寫了列，共 %d 列", n)
	}
}

// 封鎖雙方不送：買家對買家（兩個方向）、買家與店（買家封鎖店、店封鎖買家，收件人是店或是買家各測）。每組先有一個不被擋的對照。
func TestBlockedPairsAreSkippedInBothDirections(t *testing.T) {
	f := setupIntake(t)
	a1, b1 := f.newBuyer(t)
	a2, b2 := f.newBuyer(t)
	owner7, _ := f.newBuyer(t)
	owner8, _ := f.newBuyer(t)
	owner9, _ := f.newBuyer(t)
	f.resolver.addStore(7, owner7)
	f.resolver.addStore(8, owner8)
	f.resolver.addStore(9, owner9)
	ctx := context.Background()

	reply := func(id string, recipient int64, side model.Audience, actor int64) {
		f.accept(t, "test_buyer_reply", buyerReplyParams{EventID: id, RecipientBuyerID: recipient, ActorSide: side, ActorID: actor})
		f.run(t)
	}
	comment := func(id string, store, actorBuyer int64) {
		f.accept(t, "test_store_comment", storeCommentParams{EventID: id, StoreID: store, ActorBuyerID: actorBuyer})
		f.run(t)
	}
	want := func(name string, account int64, n int) {
		t.Helper()
		if got := f.inboxCount(t, account); got != n {
			t.Fatalf("%s：帳號 %d 有 %d 列，預期 %d", name, account, got, n)
		}
	}

	reply("bb-control", b1, model.AudienceBuyer, b2)
	want("買家對買家沒有封鎖", a1, 1)
	ok(t)(f.buyer.BlockBuyer(ctx, b1, b2))
	reply("bb-recipient-blocks", b1, model.AudienceBuyer, b2)
	want("收件買家封鎖 actor", a1, 1)
	reply("bb-actor-blocks", b2, model.AudienceBuyer, b1)
	want("actor 封鎖收件買家", a2, 0)

	comment("sb-control", 9, b1)
	want("店與買家沒有封鎖", owner9, 1)
	ok(t)(f.buyer.BlockStore(ctx, b1, 7))
	comment("sb-buyer-blocks-store", 7, b1)
	want("收件店被 actor 買家封鎖", owner7, 0)
	f.resolver.storeBlocksBuyer(8, b1)
	comment("sb-store-blocks-buyer", 8, b1)
	want("收件店封鎖 actor 買家", owner8, 0)

	reply("bs-control", b2, model.AudienceStore, 9)
	want("買家收件、actor 店沒有封鎖", a2, 1)
	ok(t)(f.buyer.BlockStore(ctx, b2, 7))
	reply("bs-recipient-blocks-store", b2, model.AudienceStore, 7)
	want("收件買家封鎖 actor 店", a2, 1)
	f.resolver.storeBlocksBuyer(8, b2)
	reply("bs-store-blocks-recipient", b2, model.AudienceStore, 8)
	want("actor 店封鎖收件買家", a2, 1)
}

// 追蹤者 fan-out：母意圖分頁展開，每位追蹤者一個子意圖與收件匣列；封鎖店的追蹤者被略過；重跑（包含從頭整條鏈）不重複，
// 重跑之前新增的追蹤者只多一筆。排頁 job 的唯一鍵不含 Completed、含 Running（River 的規定，wiki/tech/river-unique-job-dedup.md R1）。
func TestFanoutPagesFollowersAndRerunningCreatesNoDuplicates(t *testing.T) {
	f := setupIntake(t)
	svc := f.intakeWithChunk(2)
	ctx := context.Background()
	const storeID = 7
	var accounts, buyerIDs []int64
	for range 5 {
		a, b := f.newBuyer(t)
		accounts, buyerIDs = append(accounts, a), append(buyerIDs, b)
		if err := f.buyer.Follow(ctx, b, storeID); err != nil {
			t.Fatal(err)
		}
	}
	ok(t)(f.buyer.BlockStore(ctx, buyerIDs[3], storeID))
	slices.Sort(buyerIDs)

	f.accept(t, "test_follow_post", postParams{EventID: "post-1", StoreID: storeID})
	if n, _ := f.runAll(t, svc); len(n) != 3 {
		t.Fatalf("5 位追蹤者、一頁 2 位，預期 3 頁，實際 %d 頁", len(n))
	}
	var parent uuid.UUID
	if err := f.db.Pool().QueryRow(ctx, `SELECT id FROM notification.intents WHERE type = 'test_follow_post'`).Scan(&parent); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.intents`); n != 6 {
		t.Fatalf("意圖 %d 筆，預期母意圖 1 加子意圖 5", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE type = 'test_follow_post_item'`); n != 4 {
		t.Fatalf("收件匣 %d 列，預期 4（封鎖店的追蹤者略過）", n)
	}
	if n := f.inboxCount(t, accounts[3]); n != 0 {
		t.Fatalf("封鎖店的追蹤者仍有 %d 列", n)
	}

	a6, b6 := f.newBuyer(t)
	if err := f.buyer.Follow(ctx, b6, storeID); err != nil {
		t.Fatal(err)
	}
	if err := f.jobs.Enqueue(ctx, intake.FanoutArgs{IntentID: parent}, nil); err != nil {
		t.Fatal(err)
	}
	pages, processed := f.runAll(t, svc)
	if len(pages) != 4 || processed != 1 {
		t.Fatalf("重跑：%d 頁、%d 個 process；預期 4 頁（6 位追蹤者）、只有新追蹤者的 1 個 process", len(pages), processed)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.intents`); n != 7 {
		t.Fatalf("重跑後意圖 %d 筆，預期 7", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE type = 'test_follow_post_item'`); n != 5 || f.inboxCount(t, a6) != 1 {
		t.Fatalf("重跑後收件匣 %d 列，預期 5（新追蹤者 1 列）", n)
	}
	for i, p := range pages {
		want := int64(0)
		if i > 0 {
			want = pages[i-1].AfterID
		}
		if i > 0 && p.AfterID <= want {
			t.Fatalf("第 %d 頁的游標 %d 沒有前進", i, p.AfterID)
		}
	}
}
