//go:build integration

package notification

import (
	"context"
	"log/slog"
	"strconv"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/uitext"
)

// contentBase 是內容測試的固定時間；買家模組的時鐘也固定在它，追蹤時間因此在之後發布的事件之前。
var contentBase = time.Date(2026, 11, 23, 4, 30, 0, 0, time.UTC)

func contentClock() time.Time { return contentBase }

// codeOfStore 是測試用的 store_code 對照：store_code 與 store_id 刻意不同，用 store_id 組連結就對不上。
func codeOfStore(storeID int64) int64 { return 9_000_000 + storeID }

type fakeStoreCodes struct{ missing map[int64]bool }

func (f fakeStoreCodes) StoreCodes(_ context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	for _, id := range ids {
		if !f.missing[id] {
			out[id] = codeOfStore(id)
		}
	}
	return out, nil
}

// setupContent 組真的 buyer 與 seller、正式的註冊表與正式的內容訂閱者；store_code 的來源是替身（NewStoreCodes 只在這個測試以外驗證）。
func setupContent(t *testing.T, missing map[int64]bool) *fixture {
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
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: contentClock, Logger: log})
	events.Subscribe(bus, buyers.OnAccountCreated)
	q := nopReportingJobs{}
	sl := seller.New(seller.Deps{
		KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: id, Media: media.NewFake(), Orders: dirNoOrders{}, ProductStores: dirNoProducts{}, Registry: dirNoRegistry{}, Now: time.Now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: q, Now: time.Now, Logger: log}),
	})
	resolver := NewResolver(buyers, sl, &fakeReporters{})
	rec := &recorder{}
	m := newModule(Deps{
		DB: d, Identity: id, Buyer: buyers, Seller: sl, Resolver: resolver, StoreCodes: fakeStoreCodes{missing: missing}, Jobs: rec, Now: contentClock, NewID: uuid.NewV7, Logger: log,
	}, testRegistry{})
	events.Subscribe[social.CommentCreated](bus, m.OnCommentCreated)
	events.Subscribe[social.CommentReplied](bus, m.OnCommentRepliedToBuyer)
	events.Subscribe[social.CommentReplied](bus, m.OnCommentRepliedToStore)
	events.Subscribe[social.CommentLiked](bus, m.OnCommentLikedBuyerAuthor)
	events.Subscribe[social.CommentLiked](bus, m.OnCommentLikedStoreAuthor)
	events.Subscribe[social.PostLiked](bus, m.OnPostLiked)
	events.Subscribe[social.PostPublished](bus, m.OnPostPublished)
	events.Subscribe[buyer.SellerFollowed](bus, m.OnSellerFollowed)
	events.Subscribe[buyer.RestockTargeted](bus, m.OnRestockTargeted)
	events.Subscribe[review.ReviewCreated](bus, m.OnReviewCreated)
	events.Subscribe[im.MessageSent](bus, m.OnMessageSentToBuyer)
	events.Subscribe[im.MessageSent](bus, m.OnMessageSentToStore)
	return &fixture{db: d, identity: id, buyer: buyers, sellerMod: sl, bus: bus, jobs: rec, module: m, logs: logs, resolver: resolver}
}

func (f *fixture) publishEvent(t *testing.T, publish func(ctx context.Context) error) {
	t.Helper()
	if err := f.db.InTx(context.Background(), publish); err != nil {
		t.Fatal(err)
	}
	f.runAll(t, f.module.intake)
}

func (f *fixture) typeCount(t *testing.T, account int64, typ string) int {
	return f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND type = $2`, account, typ)
}

type contentWorld struct {
	ownerAccount, storeID int64
	buyerAAccount, buyerA int64
	buyerB                int64
}

func newContentWorld(t *testing.T, f *fixture) contentWorld {
	t.Helper()
	owner, _ := f.newBuyer(t)
	aAccount, a := f.newBuyer(t)
	_, b := f.newBuyer(t)
	return contentWorld{ownerAccount: owner, storeID: f.openStore(t, owner), buyerAAccount: aAccount, buyerA: a, buyerB: b}
}

var (
	storeSide = func(id int64) social.Actor { return social.Actor{Kind: social.ActorStore, ID: id} }
	buyerSide = func(id int64) social.Actor { return social.Actor{Kind: social.ActorBuyer, ID: id} }
)

// 每個內容型別：一個來源事件 → 收件匣一列（im 只有推播投遞、沒有收件匣列）；追蹤的店發文由 fan-out 送到追蹤者。
func TestEachContentEventReachesItsRecipient(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	ctx := context.Background()
	now := contentBase
	followerAccount, follower := f.newBuyer(t)
	if err := f.buyer.Follow(ctx, follower, w.storeID); err != nil {
		t.Fatal(err)
	}
	post, comment := uuid.NewV7(), uuid.NewV7()
	pub := func(e any) func(context.Context) error {
		return func(ctx context.Context) error {
			switch e := e.(type) {
			case social.CommentCreated:
				return events.Publish(ctx, f.bus, e)
			case social.CommentReplied:
				return events.Publish(ctx, f.bus, e)
			case social.CommentLiked:
				return events.Publish(ctx, f.bus, e)
			case social.PostLiked:
				return events.Publish(ctx, f.bus, e)
			case social.PostPublished:
				return events.Publish(ctx, f.bus, e)
			case buyer.SellerFollowed:
				return events.Publish(ctx, f.bus, e)
			case buyer.RestockTargeted:
				return events.Publish(ctx, f.bus, e)
			case review.ReviewCreated:
				return events.Publish(ctx, f.bus, e)
			case im.MessageSent:
				return events.Publish(ctx, f.bus, e)
			}
			panic("unknown event")
		}
	}
	cases := []struct {
		typ     string
		account int64
		event   any
	}{
		{"comment_created", w.ownerAccount, social.CommentCreated{EventID: uuid.NewV7(), OccurredAt: now, CommentID: comment, PostID: post, StoreID: w.storeID, Actor: buyerSide(w.buyerB)}},
		{"comment_replied_buyer", w.buyerAAccount, social.CommentReplied{EventID: uuid.NewV7(), OccurredAt: now, CommentID: comment, PostID: post, StoreID: w.storeID, ReplyToActor: buyerSide(w.buyerA), Actor: storeSide(w.storeID)}},
		{"comment_replied_store", w.ownerAccount, social.CommentReplied{EventID: uuid.NewV7(), OccurredAt: now, CommentID: comment, PostID: post, StoreID: w.storeID, ReplyToActor: storeSide(w.storeID), Actor: buyerSide(w.buyerB)}},
		{"comment_liked_buyer", w.buyerAAccount, social.CommentLiked{EventID: uuid.NewV7(), OccurredAt: now, CommentID: comment, PostID: post, StoreID: w.storeID, Author: buyerSide(w.buyerA), Actor: buyerSide(w.buyerB)}},
		{"comment_liked_store", w.ownerAccount, social.CommentLiked{EventID: uuid.NewV7(), OccurredAt: now, CommentID: comment, PostID: post, StoreID: w.storeID, Author: storeSide(w.storeID), Actor: buyerSide(w.buyerB)}},
		{"post_liked", w.ownerAccount, social.PostLiked{EventID: uuid.NewV7(), OccurredAt: now, PostID: post, StoreID: w.storeID, BuyerID: w.buyerB}},
		{"store_followed", w.ownerAccount, buyer.SellerFollowed{EventID: uuid.NewV7(), OccurredAt: now, StoreID: w.storeID, BuyerID: w.buyerB}},
		{"review_created", w.ownerAccount, review.ReviewCreated{EventID: uuid.NewV7(), OccurredAt: now, ReviewID: uuid.NewV7(), BuyerID: w.buyerB, StoreID: w.storeID, ProductID: 11}},
		{"product_restocked", w.buyerAAccount, buyer.RestockTargeted{EventID: uuid.NewV7(), OccurredAt: now, BuyerID: w.buyerA, ProductID: 11, ProductName: "Apple", ImageKey: "public/apple"}},
		{"post_published_item", followerAccount, social.PostPublished{EventID: uuid.NewV7(), OccurredAt: now.Add(time.Hour), PostID: post, StoreID: w.storeID}},
	}
	for _, c := range cases {
		f.publishEvent(t, pub(c.event))
		if n := f.typeCount(t, c.account, c.typ); n != 1 {
			t.Errorf("%s：收件人有 %d 列，預期 1", c.typ, n)
		}
	}

	chatCases := []struct {
		typ     string
		account int64
		sender  im.PartyKind
	}{
		{"chat_message_buyer", w.buyerAAccount, im.PartyStore},
		{"chat_message_store", w.ownerAccount, im.PartyBuyer},
	}
	for _, c := range chatCases {
		f.publishEvent(t, pub(im.MessageSent{EventID: uuid.NewV7(), OccurredAt: now, ConversationID: uuid.NewV7(), MessageID: uuid.NewV7(), Seq: 1, SenderKind: c.sender, BuyerID: w.buyerA, StoreID: w.storeID}))
		if n := f.typeCount(t, c.account, c.typ); n != 0 {
			t.Errorf("%s：聊天只推播，收件匣不該有列（%d）", c.typ, n)
		}
		if n := f.count(t, `SELECT count(*) FROM notification.deliveries d JOIN notification.intents i ON i.id = d.intent_id WHERE d.account_id = $1 AND d.channel = 'PUSH' AND i.type = $2`, c.account, c.typ); n != 1 {
			t.Errorf("%s：推播投遞 %d 筆，預期 1", c.typ, n)
		}
	}
}

// 封鎖雙方都不送（真實內容型別）：買家封鎖店、店封鎖買家、買家封鎖買家，各方向。
func TestContentTypesSkipBlockedPairsInBothDirections(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	ctx := context.Background()
	now := contentBase
	_, buyerC := f.newBuyer(t)
	_, buyerD := f.newBuyer(t)
	comment := func(actor int64) {
		f.publishEvent(t, func(ctx context.Context) error {
			return events.Publish(ctx, f.bus, social.CommentCreated{EventID: uuid.NewV7(), OccurredAt: now, CommentID: uuid.NewV7(), PostID: uuid.NewV7(), StoreID: w.storeID, Actor: buyerSide(actor)})
		})
	}
	reply := func(recipient, actor int64) {
		f.publishEvent(t, func(ctx context.Context) error {
			return events.Publish(ctx, f.bus, social.CommentReplied{EventID: uuid.NewV7(), OccurredAt: now, CommentID: uuid.NewV7(), PostID: uuid.NewV7(), StoreID: w.storeID, ReplyToActor: buyerSide(recipient), Actor: buyerSide(actor)})
		})
	}
	comment(w.buyerB)
	if n := f.typeCount(t, w.ownerAccount, "comment_created"); n != 1 {
		t.Fatalf("沒有封鎖時店家有 %d 列，預期 1", n)
	}
	ok(t)(f.buyer.BlockStore(ctx, w.buyerB, w.storeID))
	comment(w.buyerB)
	ok(t)(f.sellerMod.BlockBuyer(ctx, w.storeID, buyerC))
	comment(buyerC)
	if n := f.typeCount(t, w.ownerAccount, "comment_created"); n != 1 {
		t.Fatalf("買家封鎖店、店封鎖買家之後店家有 %d 列，預期仍是 1", n)
	}

	reply(w.buyerA, buyerD)
	if n := f.typeCount(t, w.buyerAAccount, "comment_replied_buyer"); n != 1 {
		t.Fatalf("沒有封鎖時收件買家有 %d 列，預期 1", n)
	}
	ok(t)(f.buyer.BlockBuyer(ctx, w.buyerA, buyerD))
	reply(w.buyerA, buyerD)
	if n := f.typeCount(t, w.buyerAAccount, "comment_replied_buyer"); n != 1 {
		t.Fatalf("買家封鎖買家之後有 %d 列，預期仍是 1", n)
	}
}

// 追蹤的店發文只送給發文當下已追蹤、沒有封鎖店的買家。
func TestFollowedStorePostFansOutToFollowersOnly(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	ctx := context.Background()
	var accounts []int64
	for range 3 {
		a, b := f.newBuyer(t)
		accounts = append(accounts, a)
		if err := f.buyer.Follow(ctx, b, w.storeID); err != nil {
			t.Fatal(err)
		}
		if len(accounts) == 3 {
			ok(t)(f.buyer.BlockStore(ctx, b, w.storeID))
		}
	}
	f.publishEvent(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, social.PostPublished{EventID: uuid.NewV7(), OccurredAt: contentBase.Add(time.Hour), PostID: uuid.NewV7(), StoreID: w.storeID})
	})
	for i, a := range accounts {
		want := 1
		if i == 2 {
			want = 0
		}
		if n := f.typeCount(t, a, "post_published_item"); n != want {
			t.Errorf("追蹤者 %d 有 %d 列，預期 %d", i, n, want)
		}
	}
	if n := f.typeCount(t, w.buyerAAccount, "post_published_item"); n != 0 {
		t.Errorf("沒有追蹤的買家收到 %d 列", n)
	}
}

// 深連結讀取時用 store_code（不是 store_id）組成，en 收件人加 /en 前綴；解析不到 store_code 的店連結是空的，通知本身仍在。
func TestContentDeepLinkUsesTheStoreCodeAndLocalePrefix(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	post := uuid.NewV7()
	f.publishEvent(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, social.CommentCreated{EventID: uuid.NewV7(), OccurredAt: contentBase, CommentID: uuid.NewV7(), PostID: post, StoreID: w.storeID, Actor: buyerSide(w.buyerB)})
	})
	r := Recipient{AccountID: w.ownerAccount, Audience: AudienceStore, StoreID: w.storeID}
	for l, prefix := range map[uitext.Locale]string{uitext.ZhTW: "", uitext.EN: "/en"} {
		page, err := f.module.ListInbox(context.Background(), r, nil, l, nil, 10)
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("%s：%+v（%v）", l, page, err)
		}
		want := prefix + "/sellers/" + strconv.FormatInt(codeOfStore(w.storeID), 10) + "/posts"
		if page.Items[0].DeepLink != want {
			t.Errorf("%s 的深連結 = %q，預期 %q", l, page.Items[0].DeepLink, want)
		}
	}
}

func TestContentDeepLinkIsEmptyWhenTheStoreCodeDoesNotResolve(t *testing.T) {
	missing := map[int64]bool{}
	f := setupContent(t, missing)
	w := newContentWorld(t, f)
	missing[w.storeID] = true
	f.publishEvent(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, social.PostLiked{EventID: uuid.NewV7(), OccurredAt: contentBase, PostID: uuid.NewV7(), StoreID: w.storeID, BuyerID: w.buyerB})
	})
	page, err := f.module.ListInbox(context.Background(), Recipient{AccountID: w.ownerAccount, Audience: AudienceStore, StoreID: w.storeID}, nil, uitext.EN, nil, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].DeepLink != "" {
		t.Fatalf("%+v（%v）", page, err)
	}
}

// 同一則貼文的第二次發布事件（事件 ID 不同）不再通知追蹤者：母意圖的冪等鍵是貼文。
func TestRepublishingAPostDoesNotNotifyFollowersAgain(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	account, b := f.newBuyer(t)
	if err := f.buyer.Follow(context.Background(), b, w.storeID); err != nil {
		t.Fatal(err)
	}
	post := uuid.NewV7()
	for i := range 2 {
		f.publishEvent(t, func(ctx context.Context) error {
			return events.Publish(ctx, f.bus, social.PostPublished{EventID: uuid.NewV7(), OccurredAt: contentBase.Add(time.Hour * time.Duration(i+1)), PostID: post, StoreID: w.storeID})
		})
	}
	if n := f.typeCount(t, account, "post_published_item"); n != 1 {
		t.Errorf("追蹤者有 %d 列，預期 1", n)
	}
	if n := f.count(t, `SELECT count(*) FROM notification.intents WHERE type = 'post_published'`); n != 1 {
		t.Errorf("母意圖 %d 個，預期 1", n)
	}
}

// 評價通知也套封鎖略過：店家封鎖評價的買家就收不到；沒有封鎖時收得到。
func TestReviewCreatedSkipsABuyerTheStoreBlocked(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	publishReview := func(buyerID int64) {
		f.publishEvent(t, func(ctx context.Context) error {
			return events.Publish(ctx, f.bus, review.ReviewCreated{EventID: uuid.NewV7(), OccurredAt: contentBase, ReviewID: uuid.NewV7(), BuyerID: buyerID, StoreID: w.storeID, ProductID: 11})
		})
	}
	publishReview(w.buyerB)
	if n := f.typeCount(t, w.ownerAccount, "review_created"); n != 1 {
		t.Fatalf("沒有封鎖時店家有 %d 列，預期 1", n)
	}
	ok(t)(f.sellerMod.BlockBuyer(context.Background(), w.storeID, w.buyerA))
	publishReview(w.buyerA)
	if n := f.typeCount(t, w.ownerAccount, "review_created"); n != 1 {
		t.Errorf("店家封鎖買家之後有 %d 列，預期仍是 1", n)
	}
}

// 補貨通知存商品名稱快照與圖片 key，深連結是商品頁；收件人是有意願的買家本人。
func TestProductRestockedStoresTheNameSnapshotAndLinksToTheProduct(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	f.publishEvent(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, buyer.RestockTargeted{EventID: uuid.NewV7(), OccurredAt: contentBase, BuyerID: w.buyerA, ProductID: 11, ProductName: "Apple", ImageKey: "public/apple"})
	})
	if n := f.count(t, `SELECT count(*) FROM notification.inbox_items WHERE account_id = $1 AND type = 'product_restocked' AND params->>'product_name' = 'Apple' AND params->>'image_key' = 'public/apple'`, w.buyerAAccount); n != 1 {
		t.Fatalf("補貨收件匣 %d 列，預期 1", n)
	}
	page, err := f.module.ListInbox(context.Background(), Recipient{AccountID: w.buyerAAccount, Audience: AudienceBuyer}, nil, uitext.EN, nil, 10)
	if err != nil || len(page.Items) != 1 || page.Items[0].DeepLink != "/en/products/11" {
		t.Fatalf("%+v（%v）", page, err)
	}
}

// 一列的參數裡店家號碼壞掉（<= 0）不讓整頁收件匣失敗：那列沒有連結，其他列照常。
func TestAnInboxRowWithAnInvalidStoreIDDoesNotFailThePage(t *testing.T) {
	f := setupContent(t, nil)
	w := newContentWorld(t, f)
	post := uuid.NewV7()
	f.publishEvent(t, func(ctx context.Context) error {
		return events.Publish(ctx, f.bus, social.CommentCreated{EventID: uuid.NewV7(), OccurredAt: contentBase, CommentID: uuid.NewV7(), PostID: post, StoreID: w.storeID, Actor: buyerSide(w.buyerB)})
	})
	f.mustExec(t, `INSERT INTO notification.inbox_items (id, account_id, audience, store_id, type, params, occurred_at, created_at, updated_at)
		VALUES ($1, $2, 'STORE', $3, 'comment_created', $4::jsonb, $5, $5, $5)`,
		uuid.NewV7(), w.ownerAccount, w.storeID,
		`{"event_id":"`+uuid.NewV7().String()+`","occurred_at":"2026-11-23T04:30:00Z","comment_id":"`+uuid.NewV7().String()+`","post_id":"`+post.String()+`","store_id":0,"actor":{"side":"BUYER","id":1}}`,
		contentBase.Add(-time.Hour))
	page, err := f.module.ListInbox(context.Background(), Recipient{AccountID: w.ownerAccount, Audience: AudienceStore, StoreID: w.storeID}, nil, uitext.ZhTW, nil, 10)
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("%+v（%v）", page, err)
	}
	links := 0
	for _, it := range page.Items {
		if it.DeepLink != "" {
			links++
		}
	}
	if links != 1 {
		t.Errorf("有連結的列 %d，預期 1（壞店號的那列留空）", links)
	}
}

// 正式的 NewStoreCodes 接真的 seller：曾經公開過的店回 store_code，從沒公開過的店不在結果裡。
func TestNewStoreCodesReadsStoreCodesFromTheRealSeller(t *testing.T) {
	f := setupContent(t, nil)
	owner, _ := f.newBuyer(t)
	exposed := f.openStore(t, owner)
	other, _ := f.newBuyer(t)
	hidden := f.openStore(t, other)
	f.mustExec(t, `UPDATE seller.stores SET first_exposed_at = $2 WHERE store_id = $1`, exposed, contentBase)
	var code int64
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT store_code FROM seller.stores WHERE store_id = $1`, exposed).Scan(&code); err != nil {
		t.Fatal(err)
	}
	got, err := NewStoreCodes(f.sellerMod).StoreCodes(context.Background(), []int64{exposed, hidden, exposed, 987654321})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[exposed] != code || code <= 0 {
		t.Errorf("StoreCodes = %v，預期只有 {%d: %d}", got, exposed, code)
	}
}
