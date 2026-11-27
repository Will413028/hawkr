//go:build integration

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

type testSocialSellers struct {
	*seller.Reader
}

func (testSocialSellers) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) {
	return false, nil
}

type fakeReviewOrders struct {
	byNo map[string]review.OrderForReview
}

func (o *fakeReviewOrders) OrderForReview(_ context.Context, orderNo string) (review.OrderForReview, error) {
	v, ok := o.byNo[orderNo]
	if !ok {
		return review.OrderForReview{}, review.ErrOrderNotFound
	}
	return v, nil
}

type catalogSocialReviewFixture struct {
	t       *testing.T
	d       *db.DB
	bus     *events.Bus
	accts   *identity.Module
	buyers  *buyer.Module
	sellers *seller.Reader
	catalog *catalog.Module
	social  *social.Module
	review  *review.Module
	orders  *fakeReviewOrders
	now     time.Time
	seq     int
}

func newCatalogSocialReviewFixture(t *testing.T) *catalogSocialReviewFixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalog.Migrations()},
		migrate.Module{Name: "social", Dir: social.Migrations()},
		migrate.Module{Name: "review", Dir: review.Migrations()},
	))
	bus := events.NewBus()
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	f := &catalogSocialReviewFixture{
		t: t, d: d, bus: bus, now: now,
		orders: &fakeReviewOrders{byNo: map[string]review.OrderForReview{}},
	}
	f.accts = identity.New(identity.Deps{DB: d, Events: bus, Now: clock, Logger: logger})
	f.buyers = buyer.New(buyer.Deps{DB: d, Events: bus, Now: clock, Logger: logger})
	subscribeBuyerLifecycle(bus, f.buyers)
	f.sellers = seller.NewReader(d, clock)
	med := media.NewFake()
	f.catalog = newCatalog(d, med, f.buyers)
	f.social = newSocial(
		d, bus, jc, f.buyers, testSocialSellers{f.sellers}, med, socialProducts{catalog: f.catalog},
		social.ScheduleAuthorizerFunc(func(context.Context, social.ScheduledPublish) error { return social.ErrScheduleDenied }),
		logger,
	)
	f.review = review.New(review.Deps{
		DB: d, Events: bus, Media: med, Jobs: jc, Buyers: f.buyers, Stores: f.sellers, Orders: f.orders, Catalog: f.catalog,
		NewID: uuid.NewV7, Now: clock, Logger: logger,
	})

	if _, err := d.Pool().Exec(context.Background(), `DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *catalogSocialReviewFixture) seedStore(ctx context.Context, accountID int64) int64 {
	f.t.Helper()
	if _, err := f.d.Pool().Exec(ctx, `INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $1, $2, 'logo/k', $3, '63000010', $4, $4)`, accountID, fmt.Sprintf("店%d", accountID), uuid.NewV7(), f.now); err != nil {
		f.t.Fatal(err)
	}
	var storeID int64
	if err := f.d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.d.Pool().Exec(ctx, `INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, f.now, uuid.NewV7()); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.d.Pool().Exec(ctx, `INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, f.now); err != nil {
		f.t.Fatal(err)
	}
	return storeID
}

func (f *catalogSocialReviewFixture) newBuyer(ctx context.Context, subject string) int64 {
	f.t.Helper()
	login, err := f.accts.Login(ctx, identity.LoginInput{
		Identity: identity.ProviderIdentity{Provider: identity.ProviderGoogle, Subject: subject, Name: subject},
		Client:   identity.ClientApp,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	auth, err := f.accts.Authenticate(ctx, login.Token)
	if err != nil {
		f.t.Fatal(err)
	}
	p, err := f.buyers.Profile(ctx, auth.Principal.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return p.BuyerID
}

func (f *catalogSocialReviewFixture) createProduct(ctx context.Context, storeID, accountID int64, status catalog.ProductStatus) catalog.Product {
	f.t.Helper()
	f.seq++
	cats, err := f.catalog.Categories(ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	var leaf uuid.UUID
	for _, c := range cats {
		if c.IsLeaf() {
			leaf = c.ID
			break
		}
	}
	p, err := f.catalog.CreateProduct(ctx, storeID, media.Uploader{ID: accountID}, uuid.NewV7(), catalog.ProductInput{
		Title:      fmt.Sprintf("商品 %d", f.seq),
		CategoryID: leaf,
		Gallery:    []catalog.Image{{Token: fmt.Sprintf("img-%d", f.seq)}},
		Skus:       []catalog.SkuInput{{Price: 120, StockQty: 5}},
	})
	if err != nil {
		f.t.Fatal(err)
	}
	if status != catalog.StatusDraft {
		if err := f.catalog.SetProductStatus(ctx, storeID, p.ID, p.Version, status); err != nil {
			f.t.Fatal(err)
		}
		p, err = f.catalog.Product(ctx, storeID, p.ID)
		if err != nil {
			f.t.Fatal(err)
		}
	}
	return p
}

func TestSocialProductsVerifiesCatalogProductAndStoreOwnership(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newCatalogSocialReviewFixture(t)

	store1 := f.seedStore(ctx, 3001)
	store2 := f.seedStore(ctx, 3002)
	active := f.createProduct(ctx, store1, 3001, catalog.StatusActive)
	draft := f.createProduct(ctx, store1, 3001, catalog.StatusDraft)
	archived := f.createProduct(ctx, store1, 3001, catalog.StatusArchived)
	otherActive := f.createProduct(ctx, store2, 3002, catalog.StatusActive)

	st1, err := f.sellers.Store(ctx, store1)
	if err != nil {
		t.Fatal(err)
	}

	missing := uuid.NewV7()
	for name, badID := range map[string]uuid.UUID{
		"不存在":  missing,
		"草稿":   draft.ID,
		"已下架":  archived.ID,
		"他店商品": otherActive.ID,
	} {
		pid := badID
		if _, err := f.social.CreatePost(ctx, st1, social.CreatePostInput{Body: "貼文", ProductID: &pid}, nil); apperr.KindOf(err) != apperr.KindInvalidInput {
			t.Fatalf("%s 應回 InvalidInput：%v", name, err)
		}
	}

	pid := active.ID
	post, err := f.social.CreatePost(ctx, st1, social.CreatePostInput{Body: "貼文", ProductID: &pid}, nil)
	if err != nil {
		t.Fatalf("同店上架商品應通過：%v", err)
	}
	if post.ProductID == nil || *post.ProductID != active.ID {
		t.Fatalf("貼文商品 ID 不符：%+v", post.ProductID)
	}

	cards := socialCards{catalog: f.catalog}
	pubCards, err := cards.Cards(ctx, []uuid.UUID{active.ID, draft.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(pubCards) != 1 || pubCards[active.ID].Name != active.Title || !pubCards[active.ID].Available {
		t.Fatalf("公開卡片應只回上架商品：%+v", pubCards)
	}
	ownCards, err := cards.Cards(ctx, []uuid.UUID{active.ID, draft.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownCards) != 2 || ownCards[draft.ID].Available {
		t.Fatalf("含下架/草稿卡片應回兩筆且草稿 available=false：%+v", ownCards)
	}
}

func TestReviewSyncsRatingToCatalogAndRollsBackOnMissingProduct(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newCatalogSocialReviewFixture(t)

	store1 := f.seedStore(ctx, 4001)
	prod := f.createProduct(ctx, store1, 4001, catalog.StatusActive)
	b1 := f.newBuyer(ctx, "g-b1")
	b2 := f.newBuyer(ctx, "g-b2")

	f.orders.byNo["ORD-1"] = review.OrderForReview{
		OrderNo: "ORD-1", BuyerID: b1, StoreID: store1, Status: review.OrderStatusCompleted, CompletedAt: f.now,
		Items: []review.OrderItem{{OrderItemID: 1, ProductID: prod.ID, SkuName: "預設", Price: 120}},
	}
	f.orders.byNo["ORD-2"] = review.OrderForReview{
		OrderNo: "ORD-2", BuyerID: b2, StoreID: store1, Status: review.OrderStatusCompleted, CompletedAt: f.now,
		Items: []review.OrderItem{{OrderItemID: 2, ProductID: prod.ID, SkuName: "預設", Price: 120}},
	}
	f.orders.byNo["ORD-MISSING"] = review.OrderForReview{
		OrderNo: "ORD-MISSING", BuyerID: b2, StoreID: store1, Status: review.OrderStatusCompleted, CompletedAt: f.now,
		Items: []review.OrderItem{{OrderItemID: 3, ProductID: uuid.NewV7(), SkuName: "預設", Price: 120}},
	}

	r1, err := f.review.Submit(ctx, review.SubmitInput{BuyerID: b1, OrderNo: "ORD-1", OrderItemID: 1, Rating: 5, Content: "很棒"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.review.Submit(ctx, review.SubmitInput{BuyerID: b2, OrderNo: "ORD-2", OrderItemID: 2, Rating: 4, Content: "不錯"}); err != nil {
		t.Fatal(err)
	}

	got, err := f.catalog.Product(ctx, store1, prod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RatingAverage != 4.5 || got.ReviewCount != 2 {
		t.Fatalf("Submit 後評分應為 (4.5, 2)：avg=%v count=%d", got.RatingAverage, got.ReviewCount)
	}

	if _, err := f.review.Hide(ctx, review.HideInput{ReviewID: r1.ID, AdminID: 9, Category: "SPAM", Text: "垃圾訊息"}); err != nil {
		t.Fatal(err)
	}
	got, err = f.catalog.Product(ctx, store1, prod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RatingAverage != 4.0 || got.ReviewCount != 1 {
		t.Fatalf("Hide 後評分應為 (4.0, 1)：avg=%v count=%d", got.RatingAverage, got.ReviewCount)
	}

	if _, err := f.review.Restore(ctx, review.RestoreInput{ReviewID: r1.ID, AdminID: 9, Reason: "誤判"}); err != nil {
		t.Fatal(err)
	}
	got, err = f.catalog.Product(ctx, store1, prod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RatingAverage != 4.5 || got.ReviewCount != 2 {
		t.Fatalf("Restore 後評分應為 (4.5, 2)：avg=%v count=%d", got.RatingAverage, got.ReviewCount)
	}

	// 不存在的商品：SetRating 回 ErrProductNotFound，評價寫入必須一起回滾。
	_, err = f.review.Submit(ctx, review.SubmitInput{BuyerID: b2, OrderNo: "ORD-MISSING", OrderItemID: 3, Rating: 5, Content: "不存在商品"})
	if !errors.Is(err, catalog.ErrProductNotFound) {
		t.Fatalf("不存在商品應回 catalog.ErrProductNotFound：%v", err)
	}
	var cnt int64
	if err := f.d.Pool().QueryRow(ctx, `SELECT count(*) FROM review.reviews WHERE order_no = 'ORD-MISSING'`).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("SetRating 失敗時評價必須回滾：count=%d", cnt)
	}

	// 買家停用並 PurgeAuthor 後，catalog 評分同步為剩餘公開評價 (4.0, 1)。
	if err := f.buyers.BeginDeletion(ctx, b1); err != nil {
		t.Fatal(err)
	}
	if err := f.review.PurgeAuthor(ctx, b1); err != nil {
		t.Fatal(err)
	}
	got, err = f.catalog.Product(ctx, store1, prod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RatingAverage != 4.0 || got.ReviewCount != 1 {
		t.Fatalf("PurgeAuthor 後評分應為 (4.0, 1)：avg=%v count=%d", got.RatingAverage, got.ReviewCount)
	}
}
