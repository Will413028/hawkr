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
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

const (
	testStoreID   = int64(7001)
	testAccountID = int64(1001)
)

// catalogBuyerFixture 是 buyer 與 catalog 的真實接線（subscribeCatalogFavorites、catalogRestock、buyerSkus），資料庫是真的。
type catalogBuyerFixture struct {
	t       *testing.T
	d       *db.DB
	bus     *events.Bus
	buyers  *buyer.Module
	catalog *catalog.Module
	accts   *identity.Module
	seq     int
	logins  int
}

func newCatalogBuyerFixture(t *testing.T) *catalogBuyerFixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalog.Migrations()},
	))
	bus := events.NewBus()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := func() time.Time { return time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC) }
	f := &catalogBuyerFixture{t: t, d: d, bus: bus}
	f.accts = identity.New(identity.Deps{DB: d, Events: bus, Now: now, Logger: logger})
	f.buyers = buyer.New(buyer.Deps{DB: d, Events: bus, Now: now, Logger: logger})
	subscribeBuyerLifecycle(bus, f.buyers)
	f.catalog = newCatalog(d, media.NewFake(), f.buyers)
	subscribeCatalogFavorites(bus, f.catalog)
	return f
}

// newBuyer 建一個買家（建帳時同 transaction 建立）。
func (f *catalogBuyerFixture) newBuyer(ctx context.Context) int64 {
	f.t.Helper()
	f.logins++
	login, err := f.accts.Login(ctx, identity.LoginInput{
		Identity: identity.ProviderIdentity{Provider: identity.ProviderGoogle, Subject: fmt.Sprintf("google-%d", f.logins), Name: "買家"},
		Client:   identity.ClientApp,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	authenticated, err := f.accts.Authenticate(ctx, login.Token)
	if err != nil {
		f.t.Fatal(err)
	}
	profile, err := f.buyers.Profile(ctx, authenticated.Principal.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return profile.BuyerID
}

// product 建立並上架一個商品：每個 stock 一個 SKU（庫存依序）。
func (f *catalogBuyerFixture) product(ctx context.Context, stocks ...int32) catalog.Product {
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
	skus := make([]catalog.SkuInput, len(stocks))
	for i, stock := range stocks {
		skus[i] = catalog.SkuInput{Price: 120, StockQty: stock}
	}
	input := catalog.ProductInput{Title: fmt.Sprintf("商品 %d", f.seq), CategoryID: leaf, Gallery: []catalog.Image{{Token: fmt.Sprintf("img-%d", f.seq)}}, Skus: skus}
	if len(stocks) > 1 {
		values := axisValues(len(stocks))
		input.Axes = []catalog.Axis{{Name: "規格", Values: values}}
		for i := range skus {
			skus[i].Option1 = values[i]
		}
	}
	p, err := f.catalog.CreateProduct(ctx, testStoreID, media.Uploader{ID: testAccountID}, uuid.NewV7(), input)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.catalog.SetProductStatus(ctx, testStoreID, p.ID, p.Version, catalog.StatusActive); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func axisValues(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("款式%d", i+1)
	}
	return out
}

func (f *catalogBuyerFixture) favoriteCount(ctx context.Context, product uuid.UUID) int32 {
	f.t.Helper()
	var n int32
	if err := f.d.Pool().QueryRow(ctx, `SELECT favorite_count FROM catalog.products WHERE id = $1`, product).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func (f *catalogBuyerFixture) subscriptions(ctx context.Context, sku uuid.UUID) int64 {
	f.t.Helper()
	var n int64
	if err := f.d.Pool().QueryRow(ctx, `SELECT count(*) FROM buyer.restock_subscriptions WHERE sku_id = $1`, sku).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

// 收藏數與收藏列在同一個 transaction 內進退（D3）：收藏 +1、取消 -1；重複收藏不重複計數；收藏之後的訂閱者失敗讓整個收藏回滾，收藏數不變。
func TestFavoriteCountFollowsTheFavoriteRowInTheSameTransaction(t *testing.T) {
	ctx := context.Background()
	f := newCatalogBuyerFixture(t)
	buyerID := f.newBuyer(ctx)
	product := f.product(ctx, 5)

	for range 2 {
		if err := f.buyers.AddFavorite(ctx, buyerID, product.ID); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.favoriteCount(ctx, product.ID); got != 1 {
		t.Fatalf("favorite_count = %d, want 1 (重複收藏不重複計數)", got)
	}

	second := f.newBuyer(ctx)
	failure := errors.New("later subscriber failed")
	events.Subscribe(f.bus, func(context.Context, buyer.FavoriteAdded) error { return failure })
	if err := f.buyers.AddFavorite(ctx, second, product.ID); !errors.Is(err, failure) {
		t.Fatalf("AddFavorite = %v, want the later subscriber's error", err)
	}
	if got := f.favoriteCount(ctx, product.ID); got != 1 {
		t.Fatalf("favorite_count = %d after a rolled-back favorite, want 1", got)
	}
	if ok, err := f.buyers.CheckFavorite(ctx, second, product.ID); err != nil || ok {
		t.Fatalf("rolled-back favorite row exists=%v err=%v", ok, err)
	}

	for range 2 {
		if err := f.buyers.RemoveFavorite(ctx, buyerID, product.ID); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.favoriteCount(ctx, product.ID); got != 0 {
		t.Fatalf("favorite_count = %d after the unfavorite, want 0", got)
	}
}

// 收藏不存在的商品被拒絕（catalog 回找不到），收藏列沒有留下。
func TestFavoritingAnUnknownProductIsRejected(t *testing.T) {
	ctx := context.Background()
	f := newCatalogBuyerFixture(t)
	buyerID := f.newBuyer(ctx)
	unknown := uuid.NewV7()
	if err := f.buyers.AddFavorite(ctx, buyerID, unknown); err == nil {
		t.Fatal("favoriting a product that does not exist succeeded")
	}
	if ok, err := f.buyers.CheckFavorite(ctx, buyerID, unknown); err != nil || ok {
		t.Fatalf("favorite row exists=%v err=%v", ok, err)
	}
}

// NotifyRestocked 只通知並刪除這個 SKU 的訂閱：同商品其他 SKU 的訂閱不被通知也不被刪；PurgeSKUSubscriptions 刪對應 SKU 的訂閱。
func TestCatalogRestockNotifiesOnlyTheSKUAndPurgesSubscriptions(t *testing.T) {
	ctx := context.Background()
	f := newCatalogBuyerFixture(t)
	first, second := f.newBuyer(ctx), f.newBuyer(ctx)
	product := f.product(ctx, 0, 0)
	skuA, skuB := product.Skus[0].ID, product.Skus[1].ID
	for _, sub := range []struct {
		buyer int64
		sku   uuid.UUID
	}{{first, skuA}, {first, skuB}, {second, skuB}} {
		if err := f.buyers.SubscribeRestock(ctx, sub.buyer, product.ID, sub.sku); err != nil {
			t.Fatal(err)
		}
	}
	var targeted []buyer.RestockTargeted
	events.Subscribe(f.bus, func(_ context.Context, e buyer.RestockTargeted) error {
		targeted = append(targeted, e)
		return nil
	})
	notifier := catalogRestock{buyer: f.buyers}
	occurred := time.Date(2026, 12, 1, 9, 5, 0, 0, time.UTC)
	for range 2 { // 重送是 no-op：一次性訂閱通知過就沒有了
		if err := notifier.NotifyRestocked(ctx, catalog.RestockedSKU{ProductID: product.ID, SKUID: skuA, StoreID: testStoreID, Name: "蘋果", ImageKey: "public/apple", OccurredAt: occurred}); err != nil {
			t.Fatal(err)
		}
	}
	if len(targeted) != 1 || targeted[0].BuyerID != first || targeted[0].SKUID != skuA || targeted[0].ProductID != product.ID ||
		targeted[0].ProductName != "蘋果" || targeted[0].ImageKey != "public/apple" || !targeted[0].OccurredAt.Equal(occurred) {
		t.Fatalf("targeted=%+v, want only the first buyer for SKU A", targeted)
	}
	if f.subscriptions(ctx, skuA) != 0 || f.subscriptions(ctx, skuB) != 2 {
		t.Fatalf("subscriptions A=%d B=%d, want 0 and 2 (同商品其他 SKU 不被通知也不被刪)", f.subscriptions(ctx, skuA), f.subscriptions(ctx, skuB))
	}

	if err := notifier.PurgeSKUSubscriptions(ctx, []uuid.UUID{skuB}); err != nil {
		t.Fatal(err)
	}
	if f.subscriptions(ctx, skuB) != 0 || len(targeted) != 1 {
		t.Fatalf("after purge: B=%d targeted=%d, want 0 subscriptions and no new notification", f.subscriptions(ctx, skuB), len(targeted))
	}
}

// buyerSkus 以 catalog 的快照回報規格現況：有貨（庫存大於零且上架）、缺貨、已刪除（視為不存在）與不存在。
func TestBuyerSkusReportsStockFromTheCatalogSnapshot(t *testing.T) {
	ctx := context.Background()
	f := newCatalogBuyerFixture(t)
	product := f.product(ctx, 3, 0)
	inStock, soldOut := product.Skus[0].ID, product.Skus[1].ID
	skus := buyerSkus{catalog: f.catalog}
	for name, c := range map[string]struct {
		sku     uuid.UUID
		found   bool
		inStock bool
	}{"in stock": {inStock, true, true}, "sold out": {soldOut, true, false}, "unknown": {uuid.NewV7(), false, false}} {
		got, found, err := skus.RestockSku(ctx, c.sku)
		if err != nil || found != c.found || got.InStock != c.inStock || (found && got.ProductID != product.ID) {
			t.Fatalf("%s: sku=%+v found=%v err=%v", name, got, found, err)
		}
	}
	if err := f.catalog.DeleteProduct(ctx, testStoreID, product.ID, 0); err != nil {
		t.Fatal(err)
	}
	if _, found, err := skus.RestockSku(ctx, soldOut); err != nil || found {
		t.Fatalf("deleted product's SKU found=%v err=%v, want not found", found, err)
	}
}
