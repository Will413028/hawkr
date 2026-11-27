//go:build integration

package catalog

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/delegatedactor"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func testActor() *delegatedactor.Actor {
	return &delegatedactor.Actor{SessionID: 9, AdminID: 3, RequestID: uuid.NewV7().String(), Reason: "測試", NotifySeller: true, PayloadHash: "h"}
}

// listen 收集商品寫入事件的種類名稱；named 為真時以稽核 handler 的名稱訂閱（帶代行者的事件才發得出去）。
func (f *productFixture) listen(named bool) *[]string {
	var got []string
	record := func(name string) { got = append(got, name) }
	subscribeEvent(f.bus, named, func(context.Context, ProductCreated) error { record("created"); return nil })
	subscribeEvent(f.bus, named, func(context.Context, ProductUpdated) error { record("updated"); return nil })
	subscribeEvent(f.bus, named, func(context.Context, ProductPublished) error { record("published"); return nil })
	subscribeEvent(f.bus, named, func(context.Context, ProductUnpublished) error { record("unpublished"); return nil })
	subscribeEvent(f.bus, named, func(context.Context, ProductPublishRepeated) error { record("publish-repeated"); return nil })
	subscribeEvent(f.bus, named, func(context.Context, ProductUnpublishRepeated) error { record("unpublish-repeated"); return nil })
	return &got
}

func subscribeEvent[E any](b *events.Bus, named bool, h func(context.Context, E) error) {
	if named {
		events.SubscribeNamed(b, delegatedactor.AuditHandler, h)
		return
	}
	events.Subscribe(b, h)
}

func TestProductWritesPublishOneGeneralEventEachAndFailClosedWithoutASubscriber(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	got := f.listen(false)
	up := media.Uploader{ID: acct}
	p := f.create(storeID, acct, f.input("山藥"))
	if _, err := f.m.UpdateProduct(f.ct, storeID, up, p.ID, p.Version, f.input("山藥二號"), nil); err != nil {
		t.Fatal(err)
	}
	for _, to := range []ProductStatus{StatusActive, StatusActive, StatusArchived, StatusArchived} {
		cur, err := f.m.Product(f.ct, storeID, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.m.SetProductStatus(f.ct, storeID, p.ID, cur.Version, to, nil); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"created", "updated", "published", "publish-repeated", "unpublished", "unpublish-repeated"}
	if !slices.Equal(*got, want) {
		t.Fatalf("events = %v, want %v", *got, want)
	}

	// 帶代行者的寫入，沒有名為稽核 handler 的訂閱者：失敗關閉，商品沒有被建立。
	before := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID)
	if _, err := f.m.CreateProduct(f.ct, storeID, up, uuid.NewV7(), f.input("代行"), testActor()); !errors.Is(err, events.ErrNoSubscriber) {
		t.Fatalf("delegated create without an audit handler = %v, want ErrNoSubscriber", err)
	}
	if after := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID); after != before {
		t.Fatalf("a failed-closed delegated create left %d products, want %d", after, before)
	}
	// 代行者只能上架與下架：切成草稿被拒絕。
	cur, _ := f.m.Product(f.ct, storeID, p.ID)
	if err := f.m.SetProductStatus(f.ct, storeID, p.ID, cur.Version, StatusDraft, testActor()); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("delegated set to draft = %v, want ErrInvalidInput", err)
	}
}

func TestDelegatedWritesCarryTheActorAndShareTheWriteTransaction(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	actor := testActor()
	var seen []*delegatedactor.Actor
	var failNext bool
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, func(_ context.Context, e ProductCreated) error {
		seen = append(seen, e.Actor)
		if failNext {
			return errors.New("audit refused")
		}
		return nil
	})
	if _, err := f.m.CreateProduct(f.ct, storeID, media.Uploader{ID: acct}, uuid.NewV7(), f.input("代行建立"), actor); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || seen[0] != actor {
		t.Fatalf("the created event carried %v, want the actor passed to CreateProduct", seen)
	}
	// 稽核 handler 失敗：商品寫入跟著 rollback（稽核與寫入同 transaction）。
	failNext = true
	before := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID)
	if _, err := f.m.CreateProduct(f.ct, storeID, media.Uploader{ID: acct}, uuid.NewV7(), f.input("會被回滾"), testActor()); err == nil {
		t.Fatal("a refusing audit handler must fail the write")
	}
	if after := f.count(`SELECT count(*) FROM catalog.products WHERE store_id = $1`, storeID); after != before {
		t.Fatalf("products = %d after a refused audit, want %d", after, before)
	}
}

func (f *productFixture) activeProducts(storeID, acct int64, n int) []Product {
	f.t.Helper()
	var out []Product
	for i := range n {
		p := f.create(storeID, acct, f.input("批次"+string(rune('甲'+i))))
		f.activate(storeID, p.ID)
		cur, err := f.m.Product(f.ct, storeID, p.ID)
		if err != nil {
			f.t.Fatal(err)
		}
		out = append(out, cur)
	}
	return out
}

func TestUnpublishProductsIsAllOrNothing(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	var batches []ProductsUnpublished
	events.SubscribeNamed(f.bus, delegatedactor.AuditHandler, func(_ context.Context, e ProductsUnpublished) error {
		batches = append(batches, e)
		return nil
	})
	ps := f.activeProducts(storeID, acct, 3)
	archived := f.activeProducts(storeID, acct, 1)[0]
	if err := setStatusP(f.m, f.ct, storeID, archived.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	archived, _ = f.m.Product(f.ct, storeID, archived.ID)
	items := func(stale ...int) []VersionedProduct {
		out := make([]VersionedProduct, 0, len(ps)+1)
		for i, p := range ps {
			v := p.Version
			if slices.Contains(stale, i) {
				v++
			}
			out = append(out, VersionedProduct{ProductID: p.ID, Version: v})
		}
		return append(out, VersionedProduct{ProductID: archived.ID, Version: archived.Version})
	}

	// 兩項版本過時：整批回衝突、列出兩個商品，沒有任何商品被下架、沒有事件。
	err := f.m.UnpublishProducts(f.ct, storeID, items(0, 2), testActor())
	var conflict *BatchVersionConflictError
	if !errors.As(err, &conflict) || !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale batch = %v, want *BatchVersionConflictError", err)
	}
	if want := []uuid.UUID{ps[0].ID, ps[2].ID}; !slices.Equal(conflict.SortedIDs(), sortedIDs(want...)) {
		t.Fatalf("conflicting ids = %v, want %v", conflict.SortedIDs(), sortedIDs(want...))
	}
	for _, p := range ps {
		if got, _ := f.m.Product(f.ct, storeID, p.ID); got.Status != StatusActive || got.Version != p.Version {
			t.Fatalf("product %s changed by a rejected batch: %s v%d", p.ID, got.Status, got.Version)
		}
	}
	if len(batches) != 0 {
		t.Fatalf("a rejected batch published %d events", len(batches))
	}

	// 別家的商品：整批 ErrProductNotFound，同樣不動。
	other, otherAcct := f.store()
	foreign := f.create(other, otherAcct, f.input("別家"))
	if err := f.m.UnpublishProducts(f.ct, storeID, append(items(), VersionedProduct{ProductID: foreign.ID, Version: foreign.Version}), testActor()); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("batch with a foreign product = %v, want ErrProductNotFound", err)
	}

	// 全部版本正確（含已下架的一項）：全部下架、一個事件，已下架的項目是成功且不改版本。
	if err := f.m.UnpublishProducts(f.ct, storeID, items(), testActor()); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 1 || len(batches[0].Items) != 4 {
		t.Fatalf("events = %d (items %d), want one event with 4 items", len(batches), len(batches[0].Items))
	}
	for _, it := range batches[0].Items {
		if it.ProductID == archived.ID {
			if it.Changed || it.VersionAfter != archived.Version {
				t.Fatalf("already-archived item = %+v, want unchanged at v%d", it, archived.Version)
			}
			continue
		}
		if !it.Changed || it.VersionAfter != it.VersionBefore+1 {
			t.Fatalf("item = %+v, want changed with the version bumped by one", it)
		}
	}
	if !slices.IsSortedFunc(batches[0].Items, func(a, b UnpublishedItem) int { return a.ProductID.Compare(b.ProductID) }) {
		t.Fatal("event items are not ordered by product id")
	}
	for _, p := range ps {
		if got, _ := f.m.Product(f.ct, storeID, p.ID); got.Status != StatusArchived {
			t.Fatalf("product %s = %s, want ARCHIVED", p.ID, got.Status)
		}
	}

	// 輸入檢查：空批、重複商品、沒有版本。
	for name, in := range map[string][]VersionedProduct{
		"empty":     nil,
		"duplicate": {{ProductID: ps[0].ID, Version: 1}, {ProductID: ps[0].ID, Version: 1}},
		"no etag":   {{ProductID: ps[0].ID}},
	} {
		if err := f.m.UnpublishProducts(f.ct, storeID, in, nil); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s batch = %v, want ErrInvalidInput", name, err)
		}
	}
}
