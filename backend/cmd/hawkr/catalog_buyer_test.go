package main

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
)

type fakeRestockBuyers struct {
	inputs []buyer.FanOutInput
	more   []bool
	err    error
	purged [][]uuid.UUID
}

func (f *fakeRestockBuyers) FanOut(_ context.Context, input buyer.FanOutInput) (bool, error) {
	f.inputs = append(f.inputs, input)
	if f.err != nil {
		return false, f.err
	}
	return f.more[len(f.inputs)-1], nil
}

func (f *fakeRestockBuyers) PurgeRestockSKUs(_ context.Context, skuIDs []uuid.UUID) error {
	f.purged = append(f.purged, skuIDs)
	return nil
}

// NotifyRestocked 一批批呼叫 FanOut 直到 SKU 沒有待通知的訂閱，每一批帶同一份 SKU、商品與快照。
func TestNotifyRestockedFansOutUntilTheSKUHasNoMoreSubscriptions(t *testing.T) {
	buyers := &fakeRestockBuyers{more: []bool{true, true, false}}
	occurred := time.Date(2026, 12, 1, 9, 5, 0, 0, time.UTC)
	sku := catalog.RestockedSKU{ProductID: uuid.NewV7(), SKUID: uuid.NewV7(), StoreID: 3, Name: "蘋果", ImageKey: "public/apple", OccurredAt: occurred}
	if err := (catalogRestock{buyer: buyers}).NotifyRestocked(t.Context(), sku); err != nil {
		t.Fatal(err)
	}
	want := buyer.FanOutInput{ProductID: sku.ProductID, SKUID: sku.SKUID, Limit: buyer.MaxBatchSize, ProductName: "蘋果", ImageKey: "public/apple", OccurredAt: occurred}
	if len(buyers.inputs) != 3 {
		t.Fatalf("FanOut calls=%d, want 3", len(buyers.inputs))
	}
	for i, input := range buyers.inputs {
		if input != want {
			t.Fatalf("call %d input=%+v, want %+v", i, input, want)
		}
	}
}

func TestNotifyRestockedStopsOnAnError(t *testing.T) {
	failure := errors.New("fan-out failed")
	buyers := &fakeRestockBuyers{err: failure}
	err := (catalogRestock{buyer: buyers}).NotifyRestocked(t.Context(), catalog.RestockedSKU{ProductID: uuid.NewV7(), SKUID: uuid.NewV7(), OccurredAt: time.Now()})
	if !errors.Is(err, failure) || len(buyers.inputs) != 1 {
		t.Fatalf("err=%v calls=%d, want the error after one call", err, len(buyers.inputs))
	}
}

func TestPurgeSKUSubscriptionsPassesTheSKUs(t *testing.T) {
	buyers := &fakeRestockBuyers{}
	skus := []uuid.UUID{uuid.NewV7(), uuid.NewV7()}
	if err := (catalogRestock{buyer: buyers}).PurgeSKUSubscriptions(t.Context(), skus); err != nil {
		t.Fatal(err)
	}
	if len(buyers.purged) != 1 || len(buyers.purged[0]) != 2 || buyers.purged[0][0] != skus[0] || buyers.purged[0][1] != skus[1] {
		t.Fatalf("purged=%v", buyers.purged)
	}
}

func TestProductInfoMapsTheCatalogCard(t *testing.T) {
	card := catalog.ProductSummary{ID: uuid.NewV7(), StoreID: 9, Title: "蘋果", CoverKey: "public/apple", MinPrice: 10, MaxPrice: 30, Status: catalog.StatusActive, TotalStock: 4}
	got := productInfo(card)
	if got.ID != card.ID || got.StoreID != 9 || got.Name != "蘋果" || got.ImageKey != "public/apple" || got.MinPrice != 10 || got.MaxPrice != 30 || !got.Available || got.Rating != nil {
		t.Fatalf("info=%+v", got)
	}
	card.ReviewCount, card.RatingAverage = 3, 4.5
	if got = productInfo(card); got.Rating == nil || *got.Rating != 4.5 {
		t.Fatalf("rating=%v", got.Rating)
	}
	for name, mutate := range map[string]func(*catalog.ProductSummary){
		"sold out": func(c *catalog.ProductSummary) { c.TotalStock = 0 },
		"archived": func(c *catalog.ProductSummary) { c.Status = catalog.StatusArchived },
		"draft":    func(c *catalog.ProductSummary) { c.Status = catalog.StatusDraft },
	} {
		c := card
		mutate(&c)
		if productInfo(c).Available {
			t.Fatalf("%s must not be available", name)
		}
	}
}
