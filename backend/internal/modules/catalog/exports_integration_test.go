//go:build integration

package catalog

import (
	"context"
	"errors"
	"slices"
	"testing"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
)

func sortedIDs(ids ...uuid.UUID) []uuid.UUID {
	slices.SortFunc(ids, uuid.UUID.Compare)
	return ids
}

func mapKeys[V any](m map[uuid.UUID]V) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return sortedIDs(out...)
}

func TestProductCardsPublicAndIncludeInactive(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeA, acctA := f.store()
	storeB, acctB := f.store()
	active := f.active(storeA, acctA, stocked(f.input("上架"), 3))
	soldOut := f.active(storeA, acctA, stocked(f.input("售完"), 0))
	archived := f.active(storeA, acctA, f.input("下架"))
	if err := setStatusP(f.m, f.ct, storeA, archived.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	draft := f.create(storeA, acctA, f.input("草稿"))
	deleted := f.create(storeA, acctA, f.input("已刪除"))
	if err := f.m.DeleteProduct(f.ct, storeA, deleted.ID, 0); err != nil {
		t.Fatal(err)
	}
	hidden := f.active(storeB, acctB, f.input("店家不可曝光"))
	f.setExposable(storeB, false)
	all := []uuid.UUID{active.ID, soldOut.ID, archived.ID, draft.ID, deleted.ID, hidden.ID, uuid.NewV7()}

	pub, err := f.m.ProductCards(f.ct, f.clock(), all, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := mapKeys(pub), sortedIDs(active.ID, soldOut.ID); !slices.Equal(got, want) {
		t.Fatalf("public cards = %v, want only ACTIVE products of exposable stores %v", got, want)
	}
	c := pub[active.ID]
	if c.StoreID != storeA || c.Title != "上架" || c.CoverKey == "" || c.MinPrice != 100 || c.MaxPrice != 100 || c.TotalStock != 3 || c.Status != StatusActive {
		t.Fatalf("card content = %+v", c)
	}
	if pub[soldOut.ID].TotalStock != 0 {
		t.Fatalf("a sold-out product still has a card with zero stock: %+v", pub[soldOut.ID])
	}

	inc, err := f.m.ProductCards(f.ct, f.clock(), all, true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := mapKeys(inc), sortedIDs(active.ID, soldOut.ID, archived.ID, draft.ID, hidden.ID); !slices.Equal(got, want) {
		t.Fatalf("include-inactive cards = %v, want every status of every store except deleted/unknown %v", got, want)
	}
	if inc[archived.ID].Status != StatusArchived || inc[draft.ID].Status != StatusDraft {
		t.Fatalf("statuses: %v %v", inc[archived.ID].Status, inc[draft.ID].Status)
	}
}

func TestBatchReadsRejectBadInputAndTolerateEmpty(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	for name, call := range map[string]func([]uuid.UUID) error{
		"cards":    func(ids []uuid.UUID) error { _, err := f.m.ProductCards(f.ct, f.clock(), ids, true); return err },
		"stores":   func(ids []uuid.UUID) error { _, err := f.m.ProductStoreIDs(f.ct, ids); return err },
		"checkout": func(ids []uuid.UUID) error { _, err := f.m.CheckoutSnapshot(f.ct, ids); return err },
		"cart":     func(ids []uuid.UUID) error { _, err := f.m.SkuCartInfo(f.ct, ids); return err },
	} {
		if err := call(nil); err != nil {
			t.Errorf("%s: empty input = %v, want no error", name, err)
		}
		if err := call([]uuid.UUID{uuid.Nil()}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: nil id = %v, want ErrInvalidInput", name, err)
		}
		over := make([]uuid.UUID, MaxBatchIDs+1)
		for i := range over {
			over[i] = uuid.NewV7()
		}
		if err := call(over); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: %d ids = %v, want ErrInvalidInput", name, len(over), err)
		}
	}
}

func TestProductStoreIDsIncludeInactiveButNotDeleted(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeA, acctA := f.store()
	storeB, acctB := f.store()
	active := f.active(storeA, acctA, f.input("上架"))
	draft := f.create(storeA, acctA, f.input("草稿"))
	archived := f.active(storeB, acctB, f.input("下架"))
	if err := setStatusP(f.m, f.ct, storeB, archived.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	f.setExposable(storeB, false)
	deleted := f.create(storeA, acctA, f.input("已刪除"))
	if err := f.m.DeleteProduct(f.ct, storeA, deleted.ID, 0); err != nil {
		t.Fatal(err)
	}
	got, err := f.m.ProductStoreIDs(f.ct, []uuid.UUID{active.ID, draft.ID, archived.ID, deleted.ID, uuid.NewV7()})
	if err != nil {
		t.Fatal(err)
	}
	want := map[uuid.UUID]int64{active.ID: storeA, draft.ID: storeA, archived.ID: storeB}
	if len(got) != len(want) {
		t.Fatalf("store ids = %v, want %v", got, want)
	}
	for id, s := range want {
		if got[id] != s {
			t.Fatalf("store of %s = %d, want %d (all: %v)", id, got[id], s, got)
		}
	}
}

func TestStoresWithPublicProductsOnlyCountsActiveProductsOfExposableStores(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	withActive, acct1 := f.store()
	onlyDraft, acct2 := f.store()
	onlyArchived, acct3 := f.store()
	hidden, acct4 := f.store()
	onlyDeleted, acct5 := f.store()
	f.active(withActive, acct1, f.input("a"))
	f.active(withActive, acct1, f.input("b"))
	f.create(onlyDraft, acct2, f.input("草稿"))
	ar := f.active(onlyArchived, acct3, f.input("下架"))
	if err := setStatusP(f.m, f.ct, onlyArchived, ar.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	f.active(hidden, acct4, f.input("不可曝光"))
	f.setExposable(hidden, false)
	del := f.active(onlyDeleted, acct5, f.input("已刪除"))
	if err := f.m.DeleteProduct(f.ct, onlyDeleted, del.ID, 0); err != nil {
		t.Fatal(err)
	}
	got, err := f.m.StoresWithPublicProducts(f.ct, f.clock())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []int64{withActive}) {
		t.Fatalf("stores = %v, want only [%d] (once, even with two products)", got, withActive)
	}
	empty := newProductFixture(t)
	if got, err := empty.m.StoresWithPublicProducts(empty.ct, empty.clock()); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("no products = %v, %v, want an empty non-nil slice", got, err)
	}
}

func TestCheckoutSnapshotReportsArchivedAndDeletedSkuAsInactive(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	compare := int64(150)
	in := f.input("手工皂")
	in.Axes = []Axis{{Name: "香味", Values: []string{"薰衣草"}}, {Name: "尺寸", Values: []string{"大"}}}
	in.Skus = []SkuInput{{Option1: "薰衣草", Option2: "大", Price: 120, CompareAtPrice: &compare, StockQty: 5}}
	live := f.active(storeID, acct, in)
	plain := f.active(storeID, acct, stocked(f.input("無選項"), 2))
	arch := f.active(storeID, acct, stocked(f.input("下架"), 7))
	if err := setStatusP(f.m, f.ct, storeID, arch.ID, StatusArchived); err != nil {
		t.Fatal(err)
	}
	draft := f.create(storeID, acct, stocked(f.input("草稿"), 1))
	gone := f.active(storeID, acct, stocked(f.input("已刪除"), 4))
	if err := f.m.DeleteProduct(f.ct, storeID, gone.ID, 0); err != nil {
		t.Fatal(err)
	}
	// 只刪 SKU：多 SKU 商品改成只剩一個組合，另一個 SKU 被標刪除。
	two := f.active(storeID, acct, func() ProductInput {
		x := f.input("兩個 SKU")
		x.Axes = []Axis{{Name: "尺寸", Values: []string{"S", "M"}}}
		x.Skus = []SkuInput{{Option1: "S", Price: 100, StockQty: 3}, {Option1: "M", Price: 100, StockQty: 3}}
		return x
	}())
	var dropped uuid.UUID
	for _, s := range two.Skus {
		if s.Option1 == "M" {
			dropped = s.ID
		}
	}
	shrunk := f.input("兩個 SKU")
	shrunk.Axes = []Axis{{Name: "尺寸", Values: []string{"S"}}}
	shrunk.Skus = []SkuInput{{Option1: "S", Price: 100}}
	shrunk.Gallery = nil
	for _, g := range two.Gallery {
		shrunk.Gallery = append(shrunk.Gallery, Image{AssetID: g.AssetID, Alt: g.Alt})
	}
	if _, err := updateP(f.m, f.ct, storeID, media.Uploader{ID: acct}, two.ID, shrunk); err != nil {
		t.Fatal(err)
	}

	ids := []uuid.UUID{live.Skus[0].ID, plain.Skus[0].ID, arch.Skus[0].ID, draft.Skus[0].ID, gone.Skus[0].ID, dropped, uuid.NewV7()}
	for name, call := range map[string]func(context.Context, []uuid.UUID) (map[uuid.UUID]SkuSnapshot, error){
		"CheckoutSnapshot": f.m.CheckoutSnapshot, "SkuCartInfo": f.m.SkuCartInfo,
	} {
		got, err := call(f.ct, ids)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 6 {
			t.Fatalf("%s: %d snapshots, want 6 (every existing SKU, deleted and archived included; the unknown id absent): %v", name, len(got), got)
		}
		s := got[live.Skus[0].ID]
		if !s.Active || s.Deleted || s.Status != StatusActive || s.ProductID != live.ID || s.StoreID != storeID || s.ProductTitle != "手工皂" || s.SkuName != "薰衣草 / 大" ||
			s.Price != 120 || s.CompareAtPrice == nil || *s.CompareAtPrice != 150 || s.StockQty != 5 || s.CoverKey == "" {
			t.Fatalf("%s: live snapshot = %+v", name, s)
		}
		if p := got[plain.Skus[0].ID]; !p.Active || p.SkuName != "" || p.CompareAtPrice != nil || p.StockQty != 2 {
			t.Fatalf("%s: option-less snapshot = %+v, want active, empty sku name, no compare-at price", name, p)
		}
		if a := got[arch.Skus[0].ID]; a.Active || a.Deleted || a.Status != StatusArchived || a.StockQty != 7 {
			t.Fatalf("%s: archived product's SKU = %+v, want inactive but not deleted, stock kept", name, a)
		}
		if d := got[draft.Skus[0].ID]; d.Active || d.Status != StatusDraft {
			t.Fatalf("%s: draft SKU = %+v, want inactive", name, d)
		}
		if g := got[gone.Skus[0].ID]; g.Active || !g.Deleted {
			t.Fatalf("%s: deleted product's SKU = %+v, want inactive and deleted", name, g)
		}
		if d := got[dropped]; d.Active || !d.Deleted || d.Status != StatusActive {
			t.Fatalf("%s: deleted SKU of an active product = %+v, want inactive and deleted", name, d)
		}
	}
}

func TestFavoriteCountFollowsTheCallersTransactionAndFloorsAtZero(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.active(storeID, acct, f.input("收藏"))
	count := func() int32 { return f.counters(p.ID).FavoriteCount }
	versionBefore := f.counters(p.ID).Version

	for range 2 {
		if err := f.m.AddFavorite(f.ct, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if count() != 2 {
		t.Fatalf("favorite count = %d, want 2", count())
	}
	if got := f.counters(p.ID); got.Version != versionBefore {
		t.Fatalf("version = %d, want %d: favorites must not move the etag", got.Version, versionBefore)
	}

	rolledBack := errors.New("roll back")
	err := f.d.InTx(f.ct, func(ctx context.Context) error {
		if err := f.m.AddFavorite(ctx, p.ID); err != nil {
			return err
		}
		return rolledBack
	})
	if !errors.Is(err, rolledBack) || count() != 2 {
		t.Fatalf("err = %v, count = %d, an Add inside a rolled-back transaction must not stick", err, count())
	}
	if err := f.d.InTx(f.ct, func(ctx context.Context) error { return f.m.RemoveFavorite(ctx, p.ID) }); err != nil || count() != 1 {
		t.Fatalf("err = %v, count = %d, want 1 after a committed Remove", err, count())
	}
	for range 3 {
		if err := f.m.RemoveFavorite(f.ct, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if count() != 0 {
		t.Fatalf("count = %d, Remove must floor at 0", count())
	}
}

func TestFavoriteCountOnMissingAndDeletedProducts(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.active(storeID, acct, f.input("收藏後刪除"))
	if err := f.m.AddFavorite(f.ct, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.m.DeleteProduct(f.ct, storeID, p.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := f.m.AddFavorite(f.ct, p.ID); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("Add on a deleted product = %v, want ErrProductNotFound", err)
	}
	if err := f.m.RemoveFavorite(f.ct, p.ID); err != nil {
		t.Fatalf("Remove on a deleted product = %v, the count follows the favorite row", err)
	}
	// AdminProduct 不讀已刪除的商品，所以直接讀列上的收藏數。
	var favorites int32
	if err := f.d.Pool().QueryRow(f.ct, `SELECT favorite_count FROM catalog.products WHERE id = $1`, p.ID).Scan(&favorites); err != nil {
		t.Fatal(err)
	}
	if favorites != 0 {
		t.Fatalf("count = %d, want 0 after the Remove", favorites)
	}
	for name, err := range map[string]error{
		"add unknown":    f.m.AddFavorite(f.ct, uuid.NewV7()),
		"remove unknown": f.m.RemoveFavorite(f.ct, uuid.NewV7()),
	} {
		if !errors.Is(err, ErrProductNotFound) {
			t.Errorf("%s = %v, want ErrProductNotFound", name, err)
		}
	}
	for name, err := range map[string]error{"add nil": f.m.AddFavorite(f.ct, uuid.Nil()), "remove nil": f.m.RemoveFavorite(f.ct, uuid.Nil())} {
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s = %v, want ErrInvalidInput", name, err)
		}
	}
}
