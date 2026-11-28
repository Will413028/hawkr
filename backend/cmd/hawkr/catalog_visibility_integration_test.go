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
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/contentkind"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

type visibilityFixture struct {
	t       *testing.T
	d       *db.DB
	catalog *catalog.Module
	seller  *seller.Module
	now     time.Time
	seq     int
}

func newVisibilityFixture(t *testing.T) *visibilityFixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalog.Migrations()},
	))
	bus := events.NewBus()
	queue, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	f := &visibilityFixture{t: t, d: d, now: time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)}
	clock := func() time.Time { return f.now }
	accounts := identity.New(identity.Deps{DB: d, Events: bus, Now: clock, Logger: logger})
	buyers := buyer.New(buyer.Deps{DB: d, Events: bus, Now: clock, Logger: logger})
	f.seller = seller.New(seller.Deps{
		DB: d, Events: bus, Jobs: queue, Identity: accounts, Media: noMedia{}, Orders: noOrders{}, ProductStores: &productStoresStub{}, Registry: noRegistry{},
		KYCMAC: sealtest.MAC(t), Now: clock, Logger: logger,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, clock), Events: bus, Jobs: queue, Now: clock, Logger: logger}),
	})
	f.catalog = catalog.New(catalog.Deps{DB: d, Events: bus, NewID: uuid.NewV7, Now: clock, Media: media.NewFake(), Restock: catalogRestock{buyer: buyers}})
	f.exec(`DO $$ DECLARE r record; BEGIN
		FOR r IN SELECT conrelid::regclass AS t, conname FROM pg_constraint
		         WHERE conrelid = 'seller.kyc_applications'::regclass AND contype = 'c'
		LOOP EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', r.t, r.conname); END LOOP; END $$`)
	return f
}

func (f *visibilityFixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.d.Pool().Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatalf("%v\n%s", err, sql)
	}
}

func (f *visibilityFixture) store() (storeID, accountID int64) {
	f.t.Helper()
	ctx := context.Background()
	if err := f.d.Pool().QueryRow(ctx, "INSERT INTO identity.accounts DEFAULT VALUES RETURNING id").Scan(&accountID); err != nil {
		f.t.Fatal(err)
	}
	f.exec(`INSERT INTO seller.stores (store_code, account_id, store_name, logo_key, logo_asset_id, town_code, created_at, updated_at)
		VALUES ($1, $1, '測試店', 'logo/k', $3, '63000010', $2, $2)`, accountID, f.now, uuid.NewV7())
	if err := f.d.Pool().QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE account_id = $1", accountID).Scan(&storeID); err != nil {
		f.t.Fatal(err)
	}
	f.exec(`INSERT INTO seller.store_exposures (store_id) VALUES ($1)`, storeID)
	f.exec(`INSERT INTO seller.kyc_applications (application_id, store_id, status, seller_type, created_at, updated_at, decided_at)
		VALUES ($3, $1, 'APPROVED', 'INDIVIDUAL', $2, $2, $2)`, storeID, f.now, uuid.NewV7())
	f.exec(`INSERT INTO seller.payout_capabilities (store_id, provider, transfers_active, updated_at) VALUES ($1, 'fake', true, $2)`, storeID, f.now)
	return storeID, accountID
}

func (f *visibilityFixture) product(storeID, accountID int64, status catalog.ProductStatus, stock int32) catalog.Product {
	f.t.Helper()
	ctx := context.Background()
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
		Title: fmt.Sprintf("商品 %d", f.seq), CategoryID: leaf, Gallery: []catalog.Image{{Token: fmt.Sprintf("img-%d", f.seq)}},
		Skus: []catalog.SkuInput{{Price: 120, StockQty: stock}},
	}, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	steps := map[catalog.ProductStatus][]catalog.ProductStatus{
		catalog.StatusDraft:    nil,
		catalog.StatusActive:   {catalog.StatusActive},
		catalog.StatusArchived: {catalog.StatusActive, catalog.StatusArchived},
	}
	for _, to := range steps[status] {
		if p, err = f.catalog.SetProductStatus(ctx, storeID, p.ID, p.Version, to, nil); err != nil {
			f.t.Fatal(err)
		}
	}
	return p
}

func (f *visibilityFixture) suspend(storeID int64) seller.Measure {
	f.t.Helper()
	review := f.now.Add(7 * 24 * time.Hour)
	r, err := f.seller.CreateMeasure(context.Background(), seller.CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: seller.MeasureSuspension, StatementToSeller: "風險", Note: "內部紀錄", ReviewBy: &review,
		Risk: &seller.RiskBasis{Ground: seller.RiskFraud},
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return r.Measure
}

func (f *visibilityFixture) takedown(storeID int64, productID uuid.UUID) seller.Measure {
	f.t.Helper()
	r, err := f.seller.CreateMeasure(context.Background(), seller.CreateMeasureInput{
		AdminID: 7, StoreID: storeID, Kind: seller.MeasureContentTakedown, StatementToSeller: "商品下架", Note: "內部紀錄",
		Violation: &seller.ViolationBasis{Finding: &seller.NewFinding{Category: policy.Spam, Severity: seller.SeverityLow}},
		Takedown: &seller.ContentTakedown{Contents: []seller.TakedownContent{{
			ContentRef: seller.ContentRef{Kind: contentkind.Product, ID: productID},
		}}},
	})
	if err != nil {
		f.t.Fatal(err)
	}
	return r.Measure
}

func (f *visibilityFixture) revoke(m seller.Measure) {
	f.t.Helper()
	if _, err := f.seller.RevokeMeasure(context.Background(), seller.RevokeMeasureInput{
		MeasureID: m.ID, ExpectedVersion: m.Version, AdminID: 7, Reason: seller.RevokeAdminCorrection,
	}); err != nil {
		f.t.Fatal(err)
	}
}

func (f *visibilityFixture) deduct(p catalog.Product) error {
	f.seq++
	return f.catalog.DeductStock(context.Background(), fmt.Sprintf("deduct-%d", f.seq), []catalog.StockItem{{SkuID: p.Skus[0].ID, Quantity: 1}})
}

func (f *visibilityFixture) stockOf(p catalog.Product) int32 {
	f.t.Helper()
	snaps, err := f.catalog.CheckoutSnapshot(context.Background(), []uuid.UUID{p.Skus[0].ID})
	if err != nil {
		f.t.Fatal(err)
	}
	return snaps[p.Skus[0].ID].StockQty
}

func (f *visibilityFixture) expectBuyerVisibility(p catalog.Product, visible bool, label string) {
	f.t.Helper()
	ctx := context.Background()
	t := f.t
	sku := p.Skus[0].ID
	_, err := f.catalog.PublicProduct(ctx, p.ID, f.now)
	if visible != (err == nil) || (!visible && !errors.Is(err, catalog.ErrProductNotFound)) {
		t.Errorf("%s：公開頁 err=%v，預期可見=%v", label, err, visible)
	}
	page, err := f.catalog.ListPublicProducts(ctx, f.now, catalog.PublicFilter{StoreIDs: []int64{p.StoreID}}, catalog.SortLatest, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, it := range page.Items {
		listed = listed || it.ID == p.ID
	}
	if listed != visible {
		t.Errorf("%s：公開列表含有=%v，預期 %v", label, listed, visible)
	}
	of, err := f.catalog.ListPublicProductsOfStores(ctx, f.now, []int64{p.StoreID}, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	followed := false
	for _, it := range of.Items {
		followed = followed || it.ID == p.ID
	}
	if followed != visible {
		t.Errorf("%s：追蹤動態含有=%v，預期 %v", label, followed, visible)
	}
	stores, err := f.catalog.StoresWithPublicProducts(ctx, f.now)
	if err != nil {
		t.Fatal(err)
	}
	hasStore := false
	for _, id := range stores {
		hasStore = hasStore || id == p.StoreID
	}
	if hasStore != visible {
		t.Errorf("%s：有公開商品的店家含有=%v，預期 %v", label, hasStore, visible)
	}
	public, err := f.catalog.ProductCards(ctx, f.now, []uuid.UUID{p.ID}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := public[p.ID]; ok != visible {
		t.Errorf("%s：公開卡片存在=%v，預期 %v", label, ok, visible)
	}
	all, err := f.catalog.ProductCards(ctx, f.now, []uuid.UUID{p.ID}, true)
	if err != nil {
		t.Fatal(err)
	}
	if card, ok := all[p.ID]; !ok || card.Visible != visible {
		t.Errorf("%s：include_inactive 卡片 ok=%v visible=%v，預期仍回卡片且 visible=%v", label, ok, card.Visible, visible)
	}
	for name, read := range map[string]func(context.Context, []uuid.UUID) (map[uuid.UUID]catalog.SkuSnapshot, error){"CheckoutSnapshot": f.catalog.CheckoutSnapshot, "SkuCartInfo": f.catalog.SkuCartInfo} {
		snaps, err := read(ctx, []uuid.UUID{sku})
		if err != nil {
			t.Fatal(err)
		}
		if snap, ok := snaps[sku]; !ok || snap.Active != visible {
			t.Errorf("%s：%s ok=%v active=%v，預期 active=%v", label, name, ok, snap.Active, visible)
		}
	}
	purchasable, err := f.catalog.SkuPurchasable(ctx, sku)
	if err != nil {
		t.Fatal(err)
	}
	if purchasable != visible {
		t.Errorf("%s：SkuPurchasable=%v，預期 %v", label, purchasable, visible)
	}
	before := f.stockOf(p)
	err = f.deduct(p)
	switch {
	case visible && err != nil:
		t.Errorf("%s：可見商品扣庫存失敗：%v", label, err)
	case !visible && !errors.Is(err, catalog.ErrProductNotPurchasable):
		t.Errorf("%s：不可見商品扣庫存回 %v，預期 ErrProductNotPurchasable", label, err)
	}
	want := before
	if visible {
		want = before - 1
	}
	if after := f.stockOf(p); after != want {
		t.Errorf("%s：扣庫存後庫存 %d，預期 %d", label, after, want)
	}
}

func TestSuspendedStoreProductIsNotVisible(t *testing.T) {
	t.Parallel()
	f := newVisibilityFixture(t)
	storeID, accountID := f.store()
	p := f.product(storeID, accountID, catalog.StatusActive, 5)
	f.expectBuyerVisibility(p, true, "停權前")
	f.suspend(storeID)
	f.expectBuyerVisibility(p, false, "停權後")
}

func TestSuspendedStoreSkuIsNotPurchasableAtCheckout(t *testing.T) {
	t.Parallel()
	f := newVisibilityFixture(t)
	ctx := context.Background()
	storeID, accountID := f.store()
	p := f.product(storeID, accountID, catalog.StatusActive, 5)
	sku := p.Skus[0].ID
	f.suspend(storeID)
	snaps, err := f.catalog.CheckoutSnapshot(ctx, []uuid.UUID{sku})
	if err != nil {
		t.Fatal(err)
	}
	if snap := snaps[sku]; snap.Active || snap.StockQty != 5 {
		t.Fatalf("停權店家的 SKU 購物車快照必須 active=false 且仍讀得到庫存：%+v", snap)
	}
	if err := f.deduct(p); !errors.Is(err, catalog.ErrProductNotPurchasable) {
		t.Fatalf("停權店家的商品不能扣庫存：%v", err)
	}
	if got := f.stockOf(p); got != 5 {
		t.Fatalf("庫存不得被扣：%d", got)
	}
}

func TestProductUnderMeasureIsNotVisible(t *testing.T) {
	t.Parallel()
	f := newVisibilityFixture(t)
	storeID, accountID := f.store()
	p := f.product(storeID, accountID, catalog.StatusActive, 5)
	other := f.product(storeID, accountID, catalog.StatusActive, 5)
	f.expectBuyerVisibility(p, true, "措施前")
	m := f.takedown(storeID, p.ID)
	f.expectBuyerVisibility(p, false, "措施中")
	f.expectBuyerVisibility(other, true, "同店其他商品不受影響")
	f.revoke(m)
	f.expectBuyerVisibility(p, true, "撤銷後")
}

func TestArchivedProductPageStaysViewableUnderViewableFunction(t *testing.T) {
	t.Parallel()
	f := newVisibilityFixture(t)
	ctx := context.Background()
	storeID, accountID := f.store()
	p := f.product(storeID, accountID, catalog.StatusArchived, 5)
	pub, err := f.catalog.PublicProduct(ctx, p.ID, f.now)
	if err != nil || pub.Status != catalog.StatusArchived {
		t.Fatalf("已下架商品的單筆頁仍可讀：%+v err=%v", pub, err)
	}
	page, err := f.catalog.ListPublicProducts(ctx, f.now, catalog.PublicFilter{StoreIDs: []int64{storeID}}, catalog.SortLatest, nil, 50)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("已下架商品不在公開列表：%+v err=%v", page.Items, err)
	}
	cards, err := f.catalog.ProductCards(ctx, f.now, []uuid.UUID{p.ID}, true)
	if err != nil || !cards[p.ID].Visible {
		t.Fatalf("已下架商品的卡片仍可瀏覽（visible）：%+v err=%v", cards[p.ID], err)
	}
	if snaps, err := f.catalog.CheckoutSnapshot(ctx, []uuid.UUID{p.Skus[0].ID}); err != nil || snaps[p.Skus[0].ID].Active {
		t.Fatalf("已下架商品不可購買：%+v err=%v", snaps[p.Skus[0].ID], err)
	}
	if err := f.deduct(p); !errors.Is(err, catalog.ErrProductNotPurchasable) {
		t.Fatalf("已下架商品不能扣庫存：%v", err)
	}
	f.takedown(storeID, p.ID)
	if _, err := f.catalog.PublicProduct(ctx, p.ID, f.now); !errors.Is(err, catalog.ErrProductNotFound) {
		t.Fatalf("被措施下架的商品連單筆頁也看不到：%v", err)
	}
	if cards, err := f.catalog.ProductCards(ctx, f.now, []uuid.UUID{p.ID}, true); err != nil || cards[p.ID].Visible {
		t.Fatalf("被措施下架的卡片 visible 必須是 false：%+v err=%v", cards[p.ID], err)
	}
}
