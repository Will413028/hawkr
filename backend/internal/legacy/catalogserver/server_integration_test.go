//go:build integration

package catalogserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/legacy/gen/product"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
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

type fixture struct {
	t         *testing.T
	m         *catalog.Module
	srv       *Server
	seq       int
	storeID   int64
	accountID int64
}

func setup(t *testing.T) (*fixture, context.Context) {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "identity", Dir: identity.Migrations()},
		migrate.Module{Name: "media", Dir: media.Migrations()},
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "catalog", Dir: catalog.Migrations()},
	))
	now := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	m := catalog.New(catalog.Deps{DB: d, NewID: uuid.NewV7, Now: func() time.Time { return now }, Media: media.NewFake(), Events: events.NewBus(), Restock: noopNotifier{}})
	ctx := context.Background()
	accountID, err := sellerfixture.NewAccount(ctx, d, now)
	if err != nil {
		t.Fatal(err)
	}
	storeID, err := sellerfixture.ExposableStore(ctx, d, accountID, now)
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, m: m, srv: New(m, slog.New(slog.NewTextHandler(io.Discard, nil))), storeID: storeID, accountID: accountID}, ctx
}

func (f *fixture) product(ctx context.Context, title string, stock int32, compare *int64) catalog.Product {
	f.t.Helper()
	f.seq++
	cats, err := f.m.Categories(ctx)
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
	p, err := f.m.CreateProduct(ctx, f.storeID, media.Uploader{ID: f.accountID}, uuid.NewV7(), catalog.ProductInput{
		Title: title, CategoryID: leaf, Gallery: []catalog.Image{{Token: fmt.Sprintf("img-%d", f.seq)}},
		Skus: []catalog.SkuInput{{Price: 120, CompareAtPrice: compare, StockQty: stock}},
	}, nil)
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.m.SetProductStatus(ctx, f.storeID, p.ID, p.Version, catalog.StatusActive, nil); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *fixture) stock(ctx context.Context, sku uuid.UUID) int32 {
	f.t.Helper()
	s, err := f.m.OwnSku(ctx, f.storeID, sku)
	if err != nil {
		f.t.Fatal(err)
	}
	return s.Sku.StockQty
}

func TestDeductAndRestoreAreIdempotentOnStockTxIDAgainstTheRealCatalog(t *testing.T) {
	t.Parallel()
	f, ctx := setup(t)
	p := f.product(ctx, "手工皂", 10, nil)
	sku := p.Skus[0].ID
	in := []*pb.StockItem{{SkuId: sku.String(), Quantity: "4"}}

	for i := range 2 {
		r, err := f.srv.DeductStock(ctx, connect.NewRequest(&pb.DeductStockRequest{StockTxId: "tx-1", Items: in}))
		if err != nil || r.Msg.GetStatus() != statusSuccess {
			t.Fatalf("deduct #%d = %v / %v", i+1, r, err)
		}
	}
	if got := f.stock(ctx, sku); got != 6 {
		t.Fatalf("stock = %d, want 6 after two identical deducts", got)
	}
	for i := range 2 {
		r, err := f.srv.RestoreStock(ctx, connect.NewRequest(&pb.RestoreStockRequest{StockTxId: "tx-1", Items: in}))
		if err != nil || r.Msg.GetStatus() != statusSuccess {
			t.Fatalf("restore #%d = %v / %v", i+1, r, err)
		}
	}
	if got := f.stock(ctx, sku); got != 10 {
		t.Fatalf("stock = %d, want 10 after two identical restores", got)
	}
	r, err := f.srv.RestoreStock(ctx, connect.NewRequest(&pb.RestoreStockRequest{StockTxId: "unknown-tx", Items: in}))
	if err != nil || r.Msg.GetStatus() != statusNoopNotFound || f.stock(ctx, sku) != 10 {
		t.Fatalf("restore without a deduction = %v / %v, want NOOP_NOT_FOUND and no stock change", r, err)
	}
}

func TestInsufficientAndMissingSkusAgainstTheRealCatalog(t *testing.T) {
	t.Parallel()
	f, ctx := setup(t)
	p := f.product(ctx, "手工皂", 2, nil)
	sku := p.Skus[0].ID
	r, err := f.srv.DeductStock(ctx, connect.NewRequest(&pb.DeductStockRequest{StockTxId: "tx-1", Items: []*pb.StockItem{{SkuId: sku.String(), Quantity: "3"}}}))
	if err != nil || r.Msg.GetStatus() != statusInsufficientStock || len(r.Msg.GetInsufficientItems()) != 1 {
		t.Fatalf("deduct = %v / %v", r, err)
	}
	if it := r.Msg.GetInsufficientItems()[0]; it.GetSkuId() != sku.String() || it.GetRequestedQty() != "3" || it.GetAvailableQty() != "2" {
		t.Fatalf("item = %v, want requested 3 / available 2", it)
	}
	missing := uuid.NewV7()
	r, err = f.srv.DeductStock(ctx, connect.NewRequest(&pb.DeductStockRequest{StockTxId: "tx-2", Items: []*pb.StockItem{{SkuId: missing.String(), Quantity: "1"}}}))
	if err != nil || r.Msg.GetStatus() != statusInsufficientStock || len(r.Msg.GetInsufficientItems()) != 1 || r.Msg.GetInsufficientItems()[0].GetAvailableQty() != "0" {
		t.Fatalf("unknown sku = %v / %v, want INSUFFICIENT_STOCK with available 0", r, err)
	}
	if f.stock(ctx, sku) != 2 {
		t.Fatal("a failed deduct must not change stock")
	}
}

func TestSnapshotRpcsAgainstTheRealCatalog(t *testing.T) {
	t.Parallel()
	f, ctx := setup(t)
	compare := int64(150)
	p := f.product(ctx, "手工皂", 5, &compare)
	sku := p.Skus[0].ID
	missing := uuid.NewV7().String()

	one, err := f.srv.GetProductSimple(ctx, connect.NewRequest(&pb.GetProductSimpleRequest{SkuId: sku.String()}))
	if err != nil {
		t.Fatal(err)
	}
	d := one.Msg.GetData()
	if one.Msg.GetCode() != codeOK || d.GetProductId() != p.ID.String() || d.GetStoreId() != fmt.Sprint(f.storeID) || d.GetName() != "手工皂" || !d.GetIsActive() ||
		d.GetPrice() != "120" || d.GetListPrice() != "150" || d.GetMaxLimit() != "5" || d.GetImageObjectKey() == "" {
		t.Fatalf("GetProductSimple = %v", one.Msg)
	}
	cart, err := f.srv.BatchGetSkuCartInfo(ctx, connect.NewRequest(&pb.BatchGetSkuCartInfoRequest{SkuIds: []string{sku.String(), missing}}))
	if err != nil || len(cart.Msg.GetSkus()) != 1 || cart.Msg.GetSkus()[sku.String()].GetStockQty() != "5" {
		t.Fatalf("BatchGetSkuCartInfo = %v / %v", cart, err)
	}
	checkout, err := f.srv.BatchGetProductForCheckout(ctx, connect.NewRequest(&pb.BatchGetProductForCheckoutRequest{SkuIds: []string{sku.String(), missing}}))
	if err != nil || len(checkout.Msg.GetData()) != 1 {
		t.Fatalf("BatchGetProductForCheckout = %v / %v", checkout, err)
	}
	if x := checkout.Msg.GetData()[sku.String()]; x.GetSalePrice() != "120" || x.GetListPrice() != "150" || x.GetProductId() != p.ID.String() || !x.GetIsActive() {
		t.Fatalf("checkout info = %v", x)
	}

	if _, err := f.m.SetProductStatus(ctx, f.storeID, p.ID, mustVersion(t, f, ctx, p.ID), catalog.StatusArchived, nil); err != nil {
		t.Fatal(err)
	}
	checkout, _ = f.srv.BatchGetProductForCheckout(ctx, connect.NewRequest(&pb.BatchGetProductForCheckoutRequest{SkuIds: []string{sku.String()}}))
	if x := checkout.Msg.GetData()[sku.String()]; x == nil || x.GetIsActive() {
		t.Fatalf("an archived product's SKU still comes back, inactive: %v", x)
	}
}

func mustVersion(t *testing.T, f *fixture, ctx context.Context, id uuid.UUID) int64 {
	t.Helper()
	p, err := f.m.Product(ctx, f.storeID, id)
	if err != nil {
		t.Fatal(err)
	}
	return p.Version
}
