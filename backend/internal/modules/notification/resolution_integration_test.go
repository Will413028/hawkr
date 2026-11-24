//go:build integration

package notification

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river/rivertype"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/intake"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/registry"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// setupResolution 組真的 buyer 與 seller 模組，Resolver 是正式的 NewResolver：店家解析、店封鎖買家與追蹤者都走真的實作。
func setupResolution(t *testing.T) *fixture {
	t.Helper()
	t.Parallel()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "notification", Dir: Migrations()},
	))
	logs := &syncBuffer{}
	log := slog.New(slog.NewTextHandler(logs, nil))
	bus := events.NewBus()
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: time.Now, Logger: log})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: time.Now, Logger: log})
	events.Subscribe(bus, buyers.OnAccountCreated)
	q := nopReportingJobs{}
	sl := seller.New(seller.Deps{
		KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: id, Media: media.NewFake(), Orders: dirNoOrders{}, ProductStores: dirNoProducts{}, Registry: dirNoRegistry{}, Now: time.Now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: q, Now: time.Now, Logger: log}),
	})
	resolver := NewResolver(buyers, sl, &fakeReporters{})
	rec := &recorder{}
	m := newModule(Deps{
		DB: d, Identity: id, Buyer: buyers, Seller: sl, Resolver: resolver, Jobs: rec, Now: time.Now, NewID: uuid.NewV7, Logger: log,
	}, testRegistry{})
	return &fixture{db: d, identity: id, buyer: buyers, sellerMod: sl, bus: bus, jobs: rec, module: m, logs: logs, resolver: resolver}
}

// openStore 讓帳號開一間店並回傳 store_id。
func (f *fixture) openStore(t *testing.T, account int64) int64 {
	t.Helper()
	s, err := f.sellerMod.Apply(context.Background(), seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: account, StoreName: "測試小店"})
	if err != nil {
		t.Fatal(err)
	}
	return s.StoreID
}

// 測試專用型別（還沒有正式的 Expand 或 fan-out 型別）。四個型別各練一種解析：店家收件（STORE，actor 是買家）、買家收件（BUYER，actor 可以是買家或店）、
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
	EventID string    `json:"event_id"`
	StoreID int64     `json:"store_id"`
	At      time.Time `json:"at"`
	// Skip 是測試用的過濾：Targets 濾掉這些買家（模擬型別自己的濾除，底層的分頁仍照原樣前進）。
	Skip []int64 `json:"skip"`
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
		OccurredAt:        func(p postParams) time.Time { return p.At },
		Broadcast: &registry.Broadcast[postParams]{
			Child: "test_follow_post_item",
			Targets: func(ctx context.Context, r registry.Resolver, p postParams, req registry.PageRequest) (registry.TargetPage, error) {
				page, err := r.StoreFollowers(ctx, p.StoreID, req.After, req.Limit, req.OccurredAt)
				if err != nil {
					return registry.TargetPage{}, err
				}
				var out []registry.Target
				for _, f := range page {
					if !slices.Contains(p.Skip, f.BuyerID) {
						out = append(out, registry.Target{ID: f.BuyerID, Params: followerItemParams{StoreID: p.StoreID, BuyerID: f.BuyerID}})
					}
				}
				next, done := registry.FollowersCursor(page, req.After, req.Limit)
				return registry.TargetPage{Targets: out, Next: next, Done: done}, nil
			},
		},
		Example: postParams{EventID: "e", StoreID: 1, At: epoch},
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
		DB: f.db, Accounts: f.identity, Buyers: f.buyer, Stores: f.sellerMod, Registry: testRegistry{}, Resolver: f.resolver, Jobs: f.jobs,
		FanoutChunk: chunk, Now: time.Now, NewID: uuid.NewV7, Logger: slog.New(slog.DiscardHandler),
	})
}

// 事件只帶 store_id 與買家 ID：worker 把店解析成擁有者帳號（真的 seller），寫進該店（STORE 受眾）的收件匣；查不到店、店已關閉的意圖不寫、參數清成 {}；
// 店主自己對自己的店留言是自己觸發的事，被略過。
func TestStoreEventResolvesToTheOwnerAccountAndStore(t *testing.T) {
	f := setupResolution(t)
	ctx := context.Background()
	owner, ownerBuyer := f.newBuyer(t)
	_, actorBuyer := f.newBuyer(t)
	storeID := f.openStore(t, owner)

	f.accept(t, "test_store_comment", storeCommentParams{EventID: "c1", StoreID: storeID, ActorBuyerID: actorBuyer})
	f.run(t)
	var audience string
	var rowStore *int64
	if err := f.db.Pool().QueryRow(ctx, `SELECT audience, store_id FROM notification.inbox_items WHERE account_id = $1`, owner).Scan(&audience, &rowStore); err != nil {
		t.Fatalf("店主沒有收件匣列：%v", err)
	}
	if audience != "STORE" || rowStore == nil || *rowStore != storeID {
		t.Fatalf("收件匣列的受眾與店 = %s %v，預期 STORE 與店 %d", audience, rowStore, storeID)
	}

	f.accept(t, "test_store_comment", storeCommentParams{EventID: "gone", StoreID: storeID + 1000, ActorBuyerID: actorBuyer})
	f.run(t)
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items`); n != 1 {
		t.Fatalf("查不到店家的意圖寫了列，共 %d 列", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.intents WHERE idempotency_key = 'test_store_comment:gone' AND params = '{}'::jsonb`); n != 1 {
		t.Fatal("沒有收件人的意圖參數要清成 {}")
	}

	f.accept(t, "test_store_comment", storeCommentParams{EventID: "self", StoreID: storeID, ActorBuyerID: ownerBuyer})
	f.run(t)
	if n := f.inboxCount(t, owner); n != 1 {
		t.Fatalf("店主對自己的店留言又寫了列，共 %d 列", n)
	}

	closedOwner, _ := f.newBuyer(t)
	closedStore := f.openStore(t, closedOwner)
	if _, err := f.sellerMod.CloseCurrent(ctx, closedOwner); err != nil {
		t.Fatal(err)
	}
	f.accept(t, "test_store_comment", storeCommentParams{EventID: "closed", StoreID: closedStore, ActorBuyerID: actorBuyer})
	f.run(t)
	if n := f.inboxCount(t, closedOwner); n != 0 {
		t.Fatalf("已關閉的店仍收到 %d 列", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.intents WHERE idempotency_key = 'test_store_comment:closed' AND params = '{}'::jsonb`); n != 1 {
		t.Fatal("已關閉的店的意圖參數要清成 {}")
	}
}

// 正式的 NewResolver 接真的 seller：存活的店解析成擁有者帳號、不存在與已關閉的店不在結果裡，店封鎖買家讀 seller 的封鎖表。
// mutation：StoreAccounts 不略過已關閉的店，或 StoreBlocksBuyer 回 false → 這個測試紅。
func TestNewResolverReadsStoresAndStoreBlocksFromTheRealSeller(t *testing.T) {
	f := setupResolution(t)
	ctx := context.Background()
	openOwner, _ := f.newBuyer(t)
	closedOwner, _ := f.newBuyer(t)
	_, blockedBuyer := f.newBuyer(t)
	_, otherBuyer := f.newBuyer(t)
	open, closed := f.openStore(t, openOwner), f.openStore(t, closedOwner)
	if _, err := f.sellerMod.CloseCurrent(ctx, closedOwner); err != nil {
		t.Fatal(err)
	}

	got, err := f.resolver.StoreAccounts(ctx, []int64{open, closed, closed + 1000, open})
	if err != nil || len(got) != 1 || got[open] != openOwner {
		t.Fatalf("StoreAccounts = %v（%v），預期只有店 %d → 帳號 %d", got, err, open, openOwner)
	}
	if _, err := f.resolver.StoreAccounts(ctx, []int64{0}); err == nil {
		t.Error("店號 0 要回錯誤")
	}

	ok(t)(f.sellerMod.BlockBuyer(ctx, open, blockedBuyer))
	for name, c := range map[string]struct {
		buyer int64
		want  bool
	}{"店封鎖的買家": {blockedBuyer, true}, "沒被封鎖的買家": {otherBuyer, false}} {
		if blocked, err := f.resolver.StoreBlocksBuyer(ctx, open, c.buyer); err != nil || blocked != c.want {
			t.Errorf("%s：StoreBlocksBuyer = %v（%v），預期 %v", name, blocked, err, c.want)
		}
	}
	if blocked, err := f.resolver.StoreBlocksBuyer(ctx, closed, blockedBuyer); err != nil || blocked {
		t.Errorf("別間店沒有封鎖：%v（%v）", blocked, err)
	}
}

// 封鎖雙方不送：買家對買家（兩個方向）、買家與店（買家封鎖店、店封鎖買家，收件人是店或是買家各測）。每組先有一個不被擋的對照。
func TestBlockedPairsAreSkippedInBothDirections(t *testing.T) {
	f := setupResolution(t)
	a1, b1 := f.newBuyer(t)
	a2, b2 := f.newBuyer(t)
	owner7, _ := f.newBuyer(t)
	owner8, _ := f.newBuyer(t)
	owner9, _ := f.newBuyer(t)
	s7, s8, s9 := f.openStore(t, owner7), f.openStore(t, owner8), f.openStore(t, owner9)
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

	comment("sb-control", s9, b1)
	want("店與買家沒有封鎖", owner9, 1)
	ok(t)(f.buyer.BlockStore(ctx, b1, s7))
	comment("sb-buyer-blocks-store", s7, b1)
	want("收件店被 actor 買家封鎖", owner7, 0)
	ok(t)(f.sellerMod.BlockBuyer(ctx, s8, b1))
	comment("sb-store-blocks-buyer", s8, b1)
	want("收件店封鎖 actor 買家", owner8, 0)

	reply("bs-control", b2, model.AudienceStore, s9)
	want("買家收件、actor 店沒有封鎖", a2, 1)
	ok(t)(f.buyer.BlockStore(ctx, b2, s7))
	reply("bs-recipient-blocks-store", b2, model.AudienceStore, s7)
	want("收件買家封鎖 actor 店", a2, 1)
	ok(t)(f.sellerMod.BlockBuyer(ctx, s8, b2))
	reply("bs-store-blocks-recipient", b2, model.AudienceStore, s8)
	want("actor 店封鎖收件買家", a2, 1)
}

// 重試時意圖已經寫過收件匣（連結存在），之後才出現封鎖：略過但不清參數，已寫的列與渠道 job 還要讀這份參數。
// mutation：scrubSkipped 的 IntentHasRecipientRows 守門拿掉 → 參數被清成 {}，這個測試紅。
func TestRetryAfterABlockDoesNotScrubAnAlreadyDeliveredIntent(t *testing.T) {
	f := setupResolution(t)
	ctx := context.Background()
	a1, b1 := f.newBuyer(t)
	_, b2 := f.newBuyer(t)

	f.accept(t, "test_buyer_reply", buyerReplyParams{EventID: "retry", RecipientBuyerID: b1, ActorSide: model.AudienceBuyer, ActorID: b2})
	f.run(t)
	if n := f.inboxCount(t, a1); n != 1 {
		t.Fatalf("第一次處理寫了 %d 列，預期 1", n)
	}
	ok(t)(f.buyer.BlockBuyer(ctx, b2, b1))
	var intentID uuid.UUID
	if err := f.db.Pool().QueryRow(ctx, `SELECT id FROM notification.intents WHERE idempotency_key = 'test_buyer_reply:retry'`).Scan(&intentID); err != nil {
		t.Fatal(err)
	}
	if err := f.module.intake.Process(ctx, intentID); err != nil {
		t.Fatal(err)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.intents WHERE id = $1 AND params <> '{}'::jsonb`, intentID); n != 1 {
		t.Fatal("已經寫過收件匣的意圖在重試時被清掉參數")
	}
	if n := f.inboxCount(t, a1); n != 1 {
		t.Fatalf("重試後收件匣 %d 列，預期仍是 1", n)
	}
}

// 參數裡的買家號碼不合法（0）是永久錯誤：Process 取消 job（JobCancel），不是重試 25 次的暫時性錯誤。
// mutation：registry 的 ErrBadParams 對應（BuyerRecipient 的 ID 檢查與 permanent 轉換）拿掉 → 回一般錯誤，這個測試紅。
func TestProcessCancelsAnIntentWithAnInvalidBuyerID(t *testing.T) {
	f := setupResolution(t)
	f.accept(t, "test_buyer_reply", buyerReplyParams{EventID: "bad-id", RecipientBuyerID: 0, ActorSide: model.AudienceStore, ActorID: 1})
	jobs := f.jobs.take()
	if len(jobs) != 1 {
		t.Fatalf("排了 %d 個 job", len(jobs))
	}
	svc := intake.New(intake.Deps{
		DB: f.db, Accounts: f.identity, Buyers: f.buyer, Stores: f.sellerMod, Registry: testRegistry{}, Resolver: noBuyersResolver{Resolver: f.resolver}, Jobs: f.jobs,
		Now: time.Now, NewID: uuid.NewV7, Logger: slog.New(slog.DiscardHandler),
	})
	err := svc.Process(context.Background(), jobs[0].IntentID)
	var cancel *rivertype.JobCancelError
	if !errors.As(err, &cancel) {
		t.Fatalf("Process = %v，預期 JobCancel（買家號碼的檢查要在 Expand 自己擋，不能靠買家模組拒絕）", err)
	}
	if !errors.Is(err, registry.ErrBadParams) {
		t.Fatalf("Process = %v，預期原因是 ErrBadParams", err)
	}
}

// noBuyersResolver 對任何買家號碼都回「查不到」而不報錯，讓 Expand 自己的號碼檢查成為唯一擋下不合法買家號碼的地方（正式的買家模組自己也會拒絕）。
type noBuyersResolver struct{ registry.Resolver }

func (noBuyersResolver) BuyerAccounts(context.Context, []int64) (map[int64]int64, error) {
	return map[int64]int64{}, nil
}

// 追蹤者 fan-out：母意圖分頁展開，每位追蹤者一個子意圖與收件匣列；封鎖店的追蹤者被略過；重跑（包含從頭整條鏈）不重複，
// 重跑之前新增的追蹤者只多一筆。排頁 job 的唯一鍵不含 Completed、含 Running（River 的規定，見 intake 的 fanoutOpts）。
func TestFanoutPagesFollowersAndRerunningCreatesNoDuplicates(t *testing.T) {
	f := setupResolution(t)
	svc := f.intakeWithChunk(2)
	ctx := context.Background()
	storeOwner, _ := f.newBuyer(t)
	storeID := f.openStore(t, storeOwner)
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

	f.accept(t, "test_follow_post", postParams{EventID: "post-1", StoreID: storeID, At: time.Now().Add(time.Hour)})
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

// 型別自己濾掉整頁的追蹤者時，分頁仍沿底層前進：後面的追蹤者照樣收到，不會在短頁停下。
// mutation：Fanout 改回以 len(Items) < 一頁筆數 判斷結束 → 第一頁被濾空就停，後面的追蹤者沒有收件匣列，這個測試紅。
func TestFanoutContinuesPastAPageThatTheTypeFilteredEmpty(t *testing.T) {
	f := setupResolution(t)
	svc := f.intakeWithChunk(2)
	ctx := context.Background()
	storeOwner, _ := f.newBuyer(t)
	storeID := f.openStore(t, storeOwner)
	var accounts, buyerIDs []int64
	for range 5 {
		a, b := f.newBuyer(t)
		accounts, buyerIDs = append(accounts, a), append(buyerIDs, b)
		if err := f.buyer.Follow(ctx, b, storeID); err != nil {
			t.Fatal(err)
		}
	}
	slices.Sort(buyerIDs)
	skip := buyerIDs[:2]
	f.accept(t, "test_follow_post", postParams{EventID: "skip-first-page", StoreID: storeID, At: time.Now().Add(time.Hour), Skip: skip})
	pages, _ := f.runAll(t, svc)
	if len(pages) != 3 {
		t.Fatalf("5 位追蹤者、一頁 2 位，預期 3 頁，實際 %d 頁", len(pages))
	}
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE type = 'test_follow_post_item'`); n != 3 {
		t.Fatalf("收件匣 %d 列，預期 3（前兩位被型別濾掉）", n)
	}
	for i, a := range accounts {
		b, err := f.buyer.CurrentBuyerID(ctx, a)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if slices.Contains(skip, b) {
			want = 0
		}
		if n := f.inboxCount(t, a); n != want {
			t.Errorf("追蹤者 %d（買家 %d）有 %d 列，預期 %d", i, b, n, want)
		}
	}
}

// 事件發生之後才追蹤的買家不通知；FanoutChunk 設得比 buyer 的分頁上限大時夾到上限，不會讓 buyer 回 InvalidInput 而重試。
// mutation：查詢的 created_at <= followed_by 拿掉，或 fanoutChunk 不夾上限 → 這個測試紅。
func TestFanoutSkipsLateFollowersAndClampsAnOversizedChunk(t *testing.T) {
	f := setupResolution(t)
	svc := f.intakeWithChunk(registry.MaxFollowerPage * 5)
	ctx := context.Background()
	storeOwner, _ := f.newBuyer(t)
	storeID := f.openStore(t, storeOwner)
	earlyAccount, earlyBuyer := f.newBuyer(t)
	if err := f.buyer.Follow(ctx, earlyBuyer, storeID); err != nil {
		t.Fatal(err)
	}
	eventAt := time.Now()
	lateAccount, lateBuyer := f.newBuyer(t)
	if err := f.buyer.Follow(ctx, lateBuyer, storeID); err != nil {
		t.Fatal(err)
	}

	f.accept(t, "test_follow_post", postParams{EventID: "late", StoreID: storeID, At: eventAt})
	f.runAll(t, svc)
	if n := f.inboxCount(t, earlyAccount); n != 1 {
		t.Fatalf("事件前就追蹤的買家有 %d 列，預期 1", n)
	}
	if n := f.inboxCount(t, lateAccount); n != 0 {
		t.Fatalf("事件後才追蹤的買家有 %d 列，預期 0", n)
	}
}
