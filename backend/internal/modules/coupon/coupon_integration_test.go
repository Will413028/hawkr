//go:build integration

package coupon_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/coupon"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

var base = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)

const (
	adminID   = int64(7)
	storeA    = int64(501)
	storeB    = int64(502)
	buyer1    = int64(1001)
	buyer2    = int64(1002)
	buyer3    = int64(1003)
	longAgo   = 72 * time.Hour
	windowLen = 24 * time.Hour
)

type fakeStores struct {
	mu        sync.Mutex
	suspended map[int64]bool
	calls     atomic.Int64
}

func (f *fakeStores) StoreSuspended(_ context.Context, id int64) (bool, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.suspended[id], nil
}

func (f *fakeStores) set(id int64, v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.suspended[id] = v
}

type fakeBuyers struct {
	mu      sync.Mutex
	missing map[int64]bool
	calls   [][]int64
}

func (f *fakeBuyers) Existing(_ context.Context, ids []int64) ([]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, slices.Clone(ids))
	var out []int64
	for _, id := range ids {
		if !f.missing[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

type fakeDirectory struct {
	mu      sync.Mutex
	missing map[int64]bool
	closed  map[int64]bool
}

func (f *fakeDirectory) StoreState(_ context.Context, id int64) (coupon.StoreState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return coupon.StoreState{Exists: !f.missing[id], Closed: f.closed[id]}, nil
}

type fixture struct {
	t      *testing.T
	db     *db.DB
	m      *coupon.Module
	stores *fakeStores
	buyers *fakeBuyers
	dir    *fakeDirectory
	clock  atomic.Int64
}

func (f *fixture) now() time.Time { return time.Unix(0, f.clock.Load()).UTC() }

func (f *fixture) member(buyerID, storeID int64) {
	f.t.Helper()
	if _, err := f.m.RecordOrderCompleted(context.Background(), storeID, buyerID); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) advance(d time.Duration) { f.clock.Add(int64(d)) }

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, stores: &fakeStores{suspended: map[int64]bool{}},
		buyers: &fakeBuyers{missing: map[int64]bool{}}, dir: &fakeDirectory{missing: map[int64]bool{}, closed: map[int64]bool{}}}
	f.clock.Store(base.UnixNano())
	f.db = pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "coupon", Dir: coupon.Migrations()},
		migrate.Module{Name: adminaudit.Schema, Dir: adminaudit.Migrations()},
	))
	f.m = coupon.New(coupon.Deps{DB: f.db, Stores: f.stores, Buyers: f.buyers, StoreDirectory: f.dir, NewID: uuid.NewV7, Now: f.now})
	return f
}

func (f *fixture) campaign(mod ...func(*coupon.CreateCampaignInput)) coupon.Campaign {
	f.t.Helper()
	in := coupon.CreateCampaignInput{
		CampaignSpec: coupon.CampaignSpec{Title: "活動", DiscountType: coupon.DiscountCash, DiscountAmount: 100, StartAt: base.Add(-time.Hour), EndAt: base.Add(windowLen)},
		TargetType:   coupon.TargetAll,
		Issuer:       coupon.Issuer{AdminID: adminID},
	}
	for _, m := range mod {
		m(&in)
	}
	c, err := f.m.CreateCampaign(context.Background(), in)
	if err != nil {
		f.t.Fatalf("建立活動：%v", err)
	}
	if c.Status == coupon.StatusDraft {
		if err := f.m.PublishCampaign(context.Background(), c.ID); err != nil {
			f.t.Fatalf("發布活動：%v", err)
		}
	}
	return c
}

func (f *fixture) claim(c coupon.Campaign, buyer int64) coupon.Voucher {
	f.t.Helper()
	r, err := f.m.Claim(context.Background(), c.ID, buyer)
	if err != nil {
		f.t.Fatalf("買家 %d 領券：%v", buyer, err)
	}
	return r.Voucher
}

func (f *fixture) lockInput(v coupon.Voucher, order string) coupon.LockInput {
	return coupon.LockInput{VoucherID: v.ID, BuyerID: v.BuyerID, OrderNo: order, StoreID: storeA, OrderAmount: 1000}
}

func (f *fixture) lock(v coupon.Voucher, order string) (coupon.LockResult, error) {
	return f.m.Lock(context.Background(), f.lockInput(v, order))
}

func (f *fixture) mustLock(v coupon.Voucher, order string) coupon.LockResult {
	f.t.Helper()
	r, err := f.lock(v, order)
	if err != nil {
		f.t.Fatalf("鎖券（訂單 %s）：%v", order, err)
	}
	return r
}

func (f *fixture) scalar(sql string, args ...any) int64 {
	f.t.Helper()
	var n int64
	if err := f.db.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatalf("%s：%v", sql, err)
	}
	return n
}

func (f *fixture) exec(sql string, args ...any) error {
	_, err := f.db.Pool().Exec(context.Background(), sql, args...)
	return err
}

func (f *fixture) voucherStatus(id uuid.UUID) string {
	f.t.Helper()
	var s string
	if err := f.db.Pool().QueryRow(context.Background(), "SELECT status FROM coupon.coupon_vouchers WHERE id = $1", id).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) issued(id uuid.UUID) int64 {
	return f.scalar("SELECT issued_count FROM coupon.coupon_campaigns WHERE id = $1", id)
}

func wantReason(t *testing.T, err error, want coupon.Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("要失敗並帶原因 %s，卻成功了", want)
	}
	if got := coupon.ReasonOf(err); got != want {
		t.Fatalf("原因 = %q，要 %q（%v）", got, want, err)
	}
}

func wantConstraint(t *testing.T, err error, constraint string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != constraint {
		t.Fatalf("要被 %s 擋下，得到 %v", constraint, err)
	}
}

func parallel(n int, fn func(i int) error) []error {
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = fn(i)
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func (f *fixture) voucherOf(c coupon.Campaign, buyer int64) coupon.Voucher {
	f.t.Helper()
	return f.claim(c, buyer)
}
