//go:build integration

package catalogconsumer

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/legacy/catalogserver"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
	"github.com/Will413028/hawkr/backend/internal/testkit/sellerfixture"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

type noopNotifier struct{}

func (noopNotifier) NotifyRestocked(context.Context, catalog.RestockedSKU) error { return nil }
func (noopNotifier) PurgeSKUSubscriptions(context.Context, []uuid.UUID) error    { return nil }

const accountID = int64(1001)

type fixture struct {
	m       *catalog.Module
	storeID int64
}

func setup(t *testing.T) (*fixture, context.Context) {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalog.Migrations()},
	))
	now := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	ctx := context.Background()
	storeID, err := sellerfixture.ExposableStore(ctx, d, accountID, now)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{m: catalog.New(catalog.Deps{DB: d, NewID: uuid.NewV7, Now: func() time.Time { return now }, Media: media.NewFake(), Events: events.NewBus(), Restock: noopNotifier{}}), storeID: storeID}, ctx
}

func product(t *testing.T, f *fixture, ctx context.Context, stock int32) catalog.Product {
	t.Helper()
	cats, err := f.m.Categories(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var leaf uuid.UUID
	for _, c := range cats {
		if c.IsLeaf() {
			leaf = c.ID
			break
		}
	}
	p, err := f.m.CreateProduct(ctx, f.storeID, media.Uploader{ID: accountID}, uuid.NewV7(), catalog.ProductInput{
		Title: "手工皂", CategoryID: leaf, Gallery: []catalog.Image{{Token: "img-" + uuid.NewV7().String()}},
		Skus: []catalog.SkuInput{{Price: 120, StockQty: stock}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.SetProductStatus(ctx, f.storeID, p.ID, p.Version, catalog.StatusActive, nil); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCancelledOrderRestoresStockOnceAgainstTheRealCatalog(t *testing.T) {
	t.Parallel()
	f, ctx := setup(t)
	p := product(t, f, ctx, 10)
	sku := p.Skus[0].ID
	if err := f.m.DeductStock(ctx, "tx-1", []catalog.StockItem{{SkuID: sku, Quantity: 4}}); err != nil {
		t.Fatal(err)
	}
	body := fmt.Appendf(nil, `{"order_no":"O1","stock_tx_id":"tx-1","items":[{"sku_id":%q,"quantity":"4"}]}`, sku.String())
	c := Consumer{Catalog: f.m}
	for i := range 3 {
		if err := c.OrderCancelled(ctx, body); err != nil {
			t.Fatalf("delivery #%d: %v", i+1, err)
		}
	}
	s, err := f.m.OwnSku(ctx, f.storeID, sku)
	if err != nil || s.Sku.StockQty != 10 {
		t.Fatalf("stock = %d (%v), want 10: a resent order.cancelled restores once", s.Sku.StockQty, err)
	}
	if err := f.m.RestoreStock(ctx, catalogserver.RestoreKey("tx-1"), "tx-1", []catalog.StockItem{{SkuID: sku, Quantity: 4}}); err != nil {
		t.Fatal(err)
	}
	if s, _ := f.m.OwnSku(ctx, f.storeID, sku); s.Sku.StockQty != 10 {
		t.Fatalf("stock = %d, want 10: the gRPC path shares the restore key", s.Sku.StockQty)
	}
	none := fmt.Appendf(nil, `{"order_no":"O2","stock_tx_id":"never","items":[{"sku_id":%q,"quantity":"1"}]}`, sku.String())
	if err := c.OrderCancelled(ctx, none); err != nil {
		t.Fatalf("cancelled order without a deduction: %v", err)
	}
}

func TestSalesAreRecordedOncePerOrderNoAgainstTheRealCatalog(t *testing.T) {
	t.Parallel()
	f, ctx := setup(t)
	p := product(t, f, ctx, 1)
	c := Consumer{Catalog: f.m}
	body := func(order string, qty int) []byte {
		return fmt.Appendf(nil, `{"event_id":"e","order_no":%q,"items":[{"product_id":%q,"quantity":%d}]}`, order, p.ID.String(), qty)
	}
	for i := range 3 {
		if err := c.OrderSalesAccumulated(ctx, body("O1", 2)); err != nil {
			t.Fatalf("delivery #%d: %v", i+1, err)
		}
	}
	got, err := f.m.AdminProduct(ctx, p.ID)
	if err != nil || got.SalesCount != 2 {
		t.Fatalf("sales = %d (%v), want 2: the same order_no counts once", got.SalesCount, err)
	}
	if err := c.OrderSalesAccumulated(ctx, body("O2", 3)); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.m.AdminProduct(ctx, p.ID); got.SalesCount != 5 {
		t.Fatalf("sales = %d, want 5 after a second order", got.SalesCount)
	}
	unknown := fmt.Appendf(nil, `{"order_no":"O3","items":[{"product_id":%q,"quantity":1}]}`, uuid.NewV7().String())
	if err := c.OrderSalesAccumulated(ctx, unknown); err == nil || !isPermanent(err) {
		t.Fatalf("unknown product = %v, want a permanent error", err)
	}
	if err := c.OrderSalesAccumulated(ctx, body("O3", 1)); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.m.AdminProduct(ctx, p.ID); got.SalesCount != 6 {
		t.Fatalf("sales = %d, want 6: the failed order_no was not registered", got.SalesCount)
	}
}
