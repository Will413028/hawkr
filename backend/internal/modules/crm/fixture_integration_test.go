//go:build integration

package crm_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/payment"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/gcis"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/seal/sealtest"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// queue 記錄排入的 job（必須在 transaction 內，與真正的 jobs.Client 相同）。
type queue struct {
	mu   sync.Mutex
	args []river.JobArgs
}

func (q *queue) Enqueue(ctx context.Context, args river.JobArgs, _ *river.InsertOpts) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("enqueue outside transaction")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.args = append(q.args, args)
	return nil
}

func (*queue) EnqueueReport(context.Context, river.JobArgs, *river.InsertOpts) (bool, error) {
	return true, nil
}

// noRegistry、noProducts、noOrders 是 seller 的外部 port 的替身：這裡的測試只用 seller 的守門與表，不會呼叫它們。
type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []int64) (map[int64]int64, error) { return nil, nil }

type noOrders struct{}

func (noOrders) HasOpenOrders(context.Context, int64) (bool, error)      { return false, nil }
func (noOrders) HasOpenStoreOrders(context.Context, int64) (bool, error) { return false, nil }

func (q *queue) count(kind string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := 0
	for _, a := range q.args {
		if a.Kind() == kind {
			n++
		}
	}
	return n
}

type fixture struct {
	t     *testing.T
	db    *db.DB
	m     *crm.Module
	q     *queue
	owner *buyer.Module
	bus   *events.Bus
}

// 預設的角色：店 storeA..storeC（10..12）與買家 100..199 在 setup 明確建立；其他 ID 的測試自己 seedBuyers／seedStores。

// seedBuyers 以明確的 ID 建立營運中的買家列（最小的合法列，不經 OnAccountCreated）；守門走真正的 buyer 模組，沒有列的 ID 回 NotFound。
func seedBuyers(t *testing.T, d *db.DB, lo, hi int64) {
	t.Helper()
	if _, err := d.Pool().Exec(context.Background(), `INSERT INTO buyer.buyers (buyer_id, account_id, display_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE SELECT g, 1000000 + g, 'b', now(), now() FROM generate_series($1::bigint, $2::bigint) AS g`, lo, hi); err != nil {
		t.Fatal(err)
	}
}

// seedStores 以明確的 ID 建立營運中的店家列（最小的合法列）；守門走真正的 seller 模組。
func seedStores(t *testing.T, d *db.DB, lo, hi int64) {
	t.Helper()
	if _, err := d.Pool().Exec(context.Background(), `INSERT INTO seller.stores (store_id, store_code, account_id, store_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE SELECT g, g, 2000000 + g, 's', now(), now() FROM generate_series($1::bigint, $2::bigint) AS g`, lo, hi); err != nil {
		t.Fatal(err)
	}
}

var epoch = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

// newID 產生遞增的 UUIDv7 形狀的 ID（測試要 deterministic，所以不用時鐘）。
func newIDs() func() uuid.UUID {
	var n atomic.Uint64
	return func() uuid.UUID {
		var u uuid.UUID
		v := n.Add(1)
		for i := 0; i < 8; i++ {
			u[15-i] = byte(v >> (8 * i))
		}
		u[6] = 0x70 | (u[6] & 0x0f)
		u[8] = 0x80 | (u[8] & 0x3f)
		return u
	}
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pg.NewDB(t, testdb.Migrations(
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "crm", Dir: crm.Migrations()},
	))
	var tick atomic.Int64
	q := &queue{}
	bus := events.NewBus()
	now := func() time.Time { return epoch.Add(time.Duration(tick.Add(1)) * time.Second) }
	owner := buyer.New(buyer.Deps{DB: d, Events: bus, Jobs: q, Media: media.NewFake(), Now: now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: now, Logger: log})
	sl := seller.New(seller.Deps{KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: id, Media: media.NewFake(), Orders: noOrders{}, ProductStores: noProducts{}, Registry: noRegistry{}, Now: now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, now), Events: bus, Jobs: q, Now: now, Logger: log})})
	seedStores(t, d, storeA, storeC)
	seedBuyers(t, d, buyer1, buyer1+99)
	m := crm.New(crm.Deps{DB: d, Jobs: q, Buyers: owner, Stores: sl, NewID: newIDs(), Now: now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	return &fixture{t: t, db: d, m: m, q: q, owner: owner, bus: bus}
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	if _, err := f.db.Pool().Exec(context.Background(), sql, args...); err != nil {
		f.t.Fatalf("%s：%v", sql, err)
	}
}

func (f *fixture) scalar(sql string, args ...any) int64 {
	f.t.Helper()
	var n int64
	if err := f.db.Pool().QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		f.t.Fatalf("%s：%v", sql, err)
	}
	return n
}

func (f *fixture) count(table string) int64 { return f.scalar("SELECT count(*) FROM " + table) }

// customer 回傳客戶的 total_spent、order_count 與是否存在。
func (f *fixture) customer(storeID, buyerID int64) (spent int64, orders int64, exists bool) {
	f.t.Helper()
	err := f.db.Pool().QueryRow(context.Background(),
		"SELECT total_spent, order_count FROM crm.customers WHERE store_id = $1 AND buyer_id = $2", storeID, buyerID).Scan(&spent, &orders)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, false
		}
		f.t.Fatal(err)
	}
	return spent, orders, true
}

// netSum 是投影的不變量：total_spent 恆等於該客戶 completed 訂單的 SUM(GREATEST(net_amount, 0))。
func (f *fixture) netSum(storeID, buyerID int64) int64 {
	f.t.Helper()
	return f.scalar("SELECT COALESCE(SUM(GREATEST(net_amount, 0)), 0)::bigint FROM crm.order_nets WHERE store_id = $1 AND buyer_id = $2 AND completed", storeID, buyerID)
}

func (f *fixture) assertInvariant(storeID, buyerID int64) {
	f.t.Helper()
	spent, _, ok := f.customer(storeID, buyerID)
	if !ok {
		return
	}
	if sum := f.netSum(storeID, buyerID); spent != sum {
		f.t.Fatalf("店 %d 買家 %d：total_spent %d 不等於 SUM(GREATEST(net, 0)) %d", storeID, buyerID, spent, sum)
	}
}

func (f *fixture) order(eventID, orderNo string, storeID, buyerID, amount int64) (bool, error) {
	return f.m.RecordOrderCompleted(context.Background(), crm.OrderCompleted{EventID: eventID, OrderNo: orderNo, StoreID: storeID, BuyerID: buyerID, Amount: amount})
}

func (f *fixture) partial(key, orderNo string, storeID, buyerID, amount int64) (bool, error) {
	return f.m.RecordRefund(context.Background(), crm.Refund{Key: key, OrderNo: orderNo, StoreID: storeID, BuyerID: buyerID, Outcome: crm.RefundPartial, Amount: amount})
}

func (f *fixture) full(key, orderNo string, storeID, buyerID int64) (bool, error) {
	return f.m.RecordRefund(context.Background(), crm.Refund{Key: key, OrderNo: orderNo, StoreID: storeID, BuyerID: buyerID, Outcome: crm.RefundFull})
}

func (f *fixture) mustOrder(eventID, orderNo string, storeID, buyerID, amount int64) {
	f.t.Helper()
	if ok, err := f.order(eventID, orderNo, storeID, buyerID, amount); err != nil || !ok {
		f.t.Fatalf("order %s：recorded=%v err=%v", eventID, ok, err)
	}
}

// parallel 讓 n 個 goroutine 同時開跑（先在柵欄前等齊），回傳每個的錯誤。
func parallel(n int, fn func(i int) error) []error {
	errs := make([]error, n)
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	ready.Add(n)
	done.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			errs[i] = fn(i)
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	return errs
}

func noErrors(t *testing.T, errs []error) {
	t.Helper()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("第 %d 個並行呼叫失敗：%v", i, err)
		}
	}
}

func sn(n int) string { return fmt.Sprintf("%d", n) }
