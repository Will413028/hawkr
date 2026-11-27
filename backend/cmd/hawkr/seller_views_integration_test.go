//go:build integration

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/platform/twtime"
)

type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noMedia struct{}

func (noMedia) Confirm(context.Context, media.Uploader, string, media.Visibility) (media.ConfirmedAsset, error) {
	return media.ConfirmedAsset{}, errors.New("not used")
}
func (noMedia) MarkDeleted(context.Context, []uuid.UUID) error { return nil }
func (noMedia) ReadSecure(context.Context, uuid.UUID) (io.ReadCloser, string, error) {
	return nil, "", errors.New("not used")
}

type noOrders struct{}

func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

type productStoresStub struct {
	stores map[uuid.UUID]int64
	err    error
	calls  int
}

func (p *productStoresStub) StoreIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	out := map[uuid.UUID]int64{}
	for _, id := range ids {
		if storeID, ok := p.stores[id]; ok {
			out[id] = storeID
		}
	}
	return out, nil
}

// 登入買家的商品瀏覽經 buyer 的 ProductViewed 追加成 seller 的原始事件（只有 insert，不在請求內解析商品或碰彙總）：商品服務
// 不通時瀏覽照樣成功（舊的耦合：RecordVisit 在請求內解析商品，服務掛了就失敗）；之後由彙總 job 解析出 store_id（不是帳號 ID：
// 舊實作的 relay 把帳號 ID 當 seller_id 寫入，儀表板以 store_id 讀，App 來源的瀏覽從不進儀表板）再進每日彙總。
func TestLoggedInProductViewSucceedsWithoutProductServiceAndIsRolledUpLater(t *testing.T) {
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()}, migrate.Module{Name: "buyer", Dir: buyer.Migrations()}, migrate.Module{Name: "seller", Dir: seller.Migrations()},
	))
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, twtime.Taipei)
	clock := func() time.Time { return now }
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	accounts := identity.New(identity.Deps{DB: d, Events: bus, Now: clock, Logger: logger})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Now: clock, Logger: logger})
	kycMAC := sealtest.MAC(t)
	products := &productStoresStub{err: errors.New("product service down")}
	sellers := seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: accounts, Media: noMedia{}, Orders: noOrders{}, ProductStores: products, Registry: noRegistry{},
		KYCMAC: kycMAC, Now: clock, Logger: logger,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, time.Now), Events: bus, Jobs: queue, Now: clock, Logger: logger}),
	})
	subscribeSellerViews(bus, sellers)

	var ids []int64
	for range 5 {
		var id int64
		if err := d.Conn(ctx).QueryRow(ctx, "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	buyerAccount, sellerAccount := ids[4], ids[2]
	for i, id := range []int64{ids[0], ids[1], ids[3]} {
		if _, err := sellers.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: id, StoreName: "別家" + string(rune('甲'+i)), ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路一號"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := buyers.OnAccountCreated(ctx, identity.AccountCreated{AccountID: buyerAccount, Name: "買家"}); err != nil {
		t.Fatal(err)
	}
	profile, err := buyers.Profile(ctx, buyerAccount)
	if err != nil {
		t.Fatal(err)
	}
	store, err := sellers.Apply(ctx, seller.ApplyInput{AcceptedAgreements: seller.CurrentApplyAcceptances(), AccountID: sellerAccount, StoreName: "範例小農", ContactPhone: "0912345678", TownCode: "63000050", AddressDetail: "測試路一號"})
	if err != nil {
		t.Fatal(err)
	}
	if store.StoreID == sellerAccount || store.StoreID == profile.BuyerID || store.StoreID == buyerAccount || profile.BuyerID == buyerAccount {
		t.Fatalf("fixture must keep store_id %d, seller account %d, buyer_id %d and buyer account %d apart", store.StoreID, sellerAccount, profile.BuyerID, buyerAccount)
	}

	productA, productB := uuid.NewV7(), uuid.NewV7()
	for _, productID := range []uuid.UUID{productA, productA, productB} {
		if err := buyers.RecordVisit(ctx, profile.BuyerID, productID); err != nil {
			t.Fatalf("visit failed while the product service is down: %v", err)
		}
	}
	if products.calls != 0 {
		t.Fatalf("visit path called the product port %d times", products.calls)
	}
	var events int
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM seller.product_view_events WHERE processed_at IS NULL").Scan(&events); err != nil || events != 3 {
		t.Fatalf("pending events=%d err=%v, want 3", events, err)
	}
	if _, err := sellers.RollupProductViews(ctx); err == nil {
		t.Fatal("rollup swallowed the product service error")
	}
	first, last := time.Date(2026, 10, 1, 0, 0, 0, 0, twtime.Taipei), time.Date(2026, 10, 8, 0, 0, 0, 0, twtime.Taipei)
	if views, err := sellers.ProductViewsBetween(ctx, store.StoreID, first, last); err != nil || views != 0 {
		t.Fatalf("views=%d err=%v before the rollup succeeds", views, err)
	}
	products.err, products.stores = nil, map[uuid.UUID]int64{productA: store.StoreID, productB: store.StoreID}
	if n, err := sellers.RollupProductViews(ctx); err != nil || n != 3 {
		t.Fatalf("rollup n=%d err=%v", n, err)
	}
	views, err := sellers.ProductViewsBetween(ctx, store.StoreID, first, last)
	if err != nil {
		t.Fatal(err)
	}
	if views != 3 {
		t.Fatalf("store views=%d, want 3", views)
	}
	if views, err = sellers.ProductViewsBetween(ctx, sellerAccount, first, last); err != nil || views != 0 {
		t.Fatalf("account-keyed views=%d err=%v: views must not be keyed by account_id", views, err)
	}
	var day time.Time
	if err := d.Conn(ctx).QueryRow(ctx, "SELECT day FROM seller.product_views_daily WHERE store_id = $1", store.StoreID).Scan(&day); err != nil || day.Format(time.DateOnly) != "2026-10-07" {
		t.Fatalf("day=%v err=%v", day, err)
	}
}
