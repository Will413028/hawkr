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
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
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

// guards 是 crm 的守門 port 的實作：買家走真正的 buyer 模組（LockNotClosed、關閉買家的 CloseForDeletion），店家走同形的替身（seller 的完整模組太重，
// 真正的 seller 實作由 seller 與 cmd/hawkr 的整合測試驗證）。測試用固定的 ID，買家與店家列在第一次被守門時自動建立（營運中），已存在不動。
type guards struct {
	db    *db.DB
	owner *buyer.Module
}

func ensureBuyer(ctx context.Context, d *db.DB, id int64) error {
	_, err := d.Pool().Exec(ctx, `INSERT INTO buyer.buyers (buyer_id, account_id, display_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE VALUES ($1::bigint, 1000000 + $1::bigint, 'b', now(), now()) ON CONFLICT (buyer_id) DO NOTHING`, id)
	return err
}

func ensureStore(ctx context.Context, d *db.DB, id int64) error {
	_, err := d.Pool().Exec(ctx, `INSERT INTO seller.stores (store_id, store_code, account_id, store_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE VALUES ($1::bigint, $1::bigint, 2000000 + $1::bigint, 's', now(), now()) ON CONFLICT (store_id) DO NOTHING`, id)
	return err
}

func (g *guards) LockNotClosed(ctx context.Context, id int64) error {
	if err := ensureBuyer(ctx, g.db, id); err != nil {
		return err
	}
	return g.owner.LockNotClosed(ctx, id)
}

func (g *guards) LockNotRetired(ctx context.Context, id int64) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("LockNotRetired 必須在 transaction 內呼叫")
	}
	if err := ensureStore(ctx, g.db, id); err != nil {
		return err
	}
	var retired bool
	err := g.db.Conn(ctx).QueryRow(ctx, "SELECT retired_at IS NOT NULL FROM seller.stores WHERE store_id = $1 FOR KEY SHARE", id).Scan(&retired)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && retired) {
		return apperr.NotFound("店家已退場")
	}
	return err
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
	g := &guards{db: d, owner: owner}
	m := crm.New(crm.Deps{DB: d, Jobs: q, Buyers: g, Stores: g, NewID: newIDs(), Now: now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
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
