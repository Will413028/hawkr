//go:build integration

package catalog

import (
	"context"
	"errors"
	"math"
	"testing"
	"uuid"
)

func (f *productFixture) counters(id uuid.UUID) Product {
	f.t.Helper()
	p, err := f.m.AdminProduct(f.ct, id)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func TestSetRatingWritesAbsoluteValuesNotASum(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.create(storeID, acct, f.input("評分"))

	if err := f.m.SetRating(f.ct, p.ID, 4.5, 10); err != nil {
		t.Fatal(err)
	}
	if err := f.m.SetRating(f.ct, p.ID, 3.25, 4); err != nil {
		t.Fatal(err)
	}
	got := f.counters(p.ID)
	if got.RatingAverage != 3.25 || got.ReviewCount != 4 {
		t.Fatalf("the second absolute value replaces the first: average %v count %d, want 3.25 / 4", got.RatingAverage, got.ReviewCount)
	}
	if got.Version != p.Version {
		t.Fatalf("counters do not change the etag: %d vs %d", got.Version, p.Version)
	}

	// 評價全部移除：回到 0 / 0。
	if err := f.m.SetRating(f.ct, p.ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	if got := f.counters(p.ID); got.RatingAverage != 0 || got.ReviewCount != 0 {
		t.Fatalf("reset: %v / %d", got.RatingAverage, got.ReviewCount)
	}
}

func TestSetRatingRejectsInvalidInputAndMissingProducts(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.create(storeID, acct, f.input("評分驗證"))
	if err := f.m.SetRating(f.ct, p.ID, 4, 2); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		id      uuid.UUID
		average float64
		count   int32
	}{
		{"nil product", uuid.Nil(), 4, 1},
		{"above 5", p.ID, 5.01, 1},
		{"negative average", p.ID, -0.1, 1},
		{"NaN", p.ID, math.NaN(), 1},
		{"infinite", p.ID, math.Inf(1), 1},
		{"negative count", p.ID, 4, -1},
		{"average without reviews", p.ID, 3, 0},
	} {
		if err := f.m.SetRating(f.ct, c.id, c.average, c.count); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: err = %v, want ErrInvalidInput", c.name, err)
		}
	}
	if got := f.counters(p.ID); got.RatingAverage != 4 || got.ReviewCount != 2 {
		t.Fatalf("rejected input must not write: %v / %d", got.RatingAverage, got.ReviewCount)
	}
	if err := f.m.SetRating(f.ct, uuid.NewV7(), 4, 1); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("unknown product = %v, want ErrProductNotFound", err)
	}
}

func TestRecordSalesCountsAnOrderOnce(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	a := f.create(storeID, acct, f.input("銷量甲"))
	b := f.create(storeID, acct, f.input("銷量乙"))

	items := []SalesItem{{ProductID: a.ID, Quantity: 2}, {ProductID: b.ID, Quantity: 1}, {ProductID: a.ID, Quantity: 3}}
	if err := f.m.RecordSales(f.ct, "ORD-1", items); err != nil {
		t.Fatal(err)
	}
	if got := f.counters(a.ID).SalesCount; got != 5 {
		t.Fatalf("two lines of the same product are summed: %d, want 5", got)
	}
	// MQ 重送同一筆訂單：不再累計（連內容不同的重送也以訂單編號為準）。
	if err := f.m.RecordSales(f.ct, "ORD-1", items); err != nil {
		t.Fatal(err)
	}
	if err := f.m.RecordSales(f.ct, "ORD-1", []SalesItem{{ProductID: a.ID, Quantity: 9}}); err != nil {
		t.Fatal(err)
	}
	if a2, b2 := f.counters(a.ID).SalesCount, f.counters(b.ID).SalesCount; a2 != 5 || b2 != 1 {
		t.Fatalf("a resend of the same order_no counts once: %d / %d, want 5 / 1", a2, b2)
	}
	// 另一筆訂單照常累計。
	if err := f.m.RecordSales(f.ct, "ORD-2", []SalesItem{{ProductID: a.ID, Quantity: 4}}); err != nil {
		t.Fatal(err)
	}
	if got := f.counters(a.ID).SalesCount; got != 9 {
		t.Fatalf("a different order counts: %d, want 9", got)
	}
	if f.counters(a.ID).Version != a.Version {
		t.Fatal("sales do not change the etag")
	}
}

func TestRecordSalesRejectsBadInputAndRollsBackOnAMissingProduct(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	a := f.create(storeID, acct, f.input("銷量回滾"))
	ok := []SalesItem{{ProductID: a.ID, Quantity: 1}}

	for _, c := range []struct {
		name    string
		orderNo string
		items   []SalesItem
	}{
		{"empty order_no", "", ok},
		{"padded order_no", " ORD ", ok},
		{"no items", "ORD-BAD", nil},
		{"zero quantity", "ORD-BAD", []SalesItem{{ProductID: a.ID}}},
		{"nil product", "ORD-BAD", []SalesItem{{Quantity: 1}}},
	} {
		if err := f.m.RecordSales(f.ct, c.orderNo, c.items); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s: err = %v, want ErrInvalidInput", c.name, err)
		}
	}

	// 有商品不存在：整筆回滾，包括訂單的登記，之後同一個 order_no 修正後還能累計。
	if err := f.m.RecordSales(f.ct, "ORD-MISSING", append([]SalesItem{{ProductID: uuid.NewV7(), Quantity: 1}}, ok...)); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("missing product = %v, want ErrProductNotFound", err)
	}
	if got := f.counters(a.ID).SalesCount; got != 0 {
		t.Fatalf("nothing counted after the rollback: %d", got)
	}
	if err := f.m.RecordSales(f.ct, "ORD-MISSING", ok); err != nil {
		t.Fatal(err)
	}
	if got := f.counters(a.ID).SalesCount; got != 1 {
		t.Fatalf("the order_no is not claimed by the failed attempt: %d, want 1", got)
	}
}

func TestLockProductReturnsTheCurrentRatingInsideATransaction(t *testing.T) {
	t.Parallel()
	f := newProductFixture(t)
	storeID, acct := f.store()
	p := f.create(storeID, acct, f.input("鎖商品"))
	if err := f.m.SetRating(f.ct, p.ID, 4.5, 10); err != nil {
		t.Fatal(err)
	}

	if _, err := f.m.LockProduct(f.ct, p.ID); err == nil || errors.Is(err, ErrProductNotFound) {
		t.Fatalf("outside a transaction the row lock would be released at once: %v", err)
	}
	err := f.d.InTx(f.ct, func(ctx context.Context) error {
		got, err := f.m.LockProduct(ctx, p.ID)
		if err != nil {
			return err
		}
		if got.Average != 4.5 || got.Count != 10 || got.StoreID != storeID {
			t.Errorf("current rating: %+v", got)
		}
		if _, err := f.m.LockProduct(ctx, uuid.NewV7()); !errors.Is(err, ErrProductNotFound) {
			t.Errorf("a missing product: %v", err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
