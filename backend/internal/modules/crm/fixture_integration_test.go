//go:build integration

package crm_test

import (
	"bytes"
	"context"
	"encoding/json"
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

type noRegistry struct{}

func (noRegistry) Lookup(context.Context, string) (gcis.Registration, bool, error) {
	return gcis.Registration{}, false, nil
}

type noProducts struct{}

func (noProducts) StoreIDs(context.Context, []uuid.UUID) (map[uuid.UUID]int64, error) {
	return nil, nil
}

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

type logSink struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logSink) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logSink) levels() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []string
	for _, line := range bytes.Split(l.buf.Bytes(), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		var r struct {
			Level string `json:"level"`
		}
		if err := json.Unmarshal(line, &r); err != nil {
			panic(err)
		}
		out = append(out, r.Level)
	}
	return out
}

type fixture struct {
	t     *testing.T
	db    *db.DB
	m     *crm.Module
	q     *queue
	owner *buyer.Module
	bus   *events.Bus
	logs  *logSink
	gate  *suspendable
}

type suspendable struct {
	*seller.Module
	suspended atomic.Bool
}

func (g *suspendable) Store(ctx context.Context, storeID int64) (seller.Store, error) {
	st, err := g.Module.Store(ctx, storeID)
	if err == nil && g.suspended.Load() {
		st.Sanctions = nil
	}
	return st, err
}

func seedBuyers(t *testing.T, d *db.DB, lo, hi int64) {
	t.Helper()
	if _, err := d.Pool().Exec(context.Background(), `INSERT INTO buyer.buyers (buyer_id, account_id, display_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE SELECT g, 1000000 + g, 'b', now(), now() FROM generate_series($1::bigint, $2::bigint) AS g`, lo, hi); err != nil {
		t.Fatal(err)
	}
}

func seedStores(t *testing.T, d *db.DB, lo, hi int64) {
	t.Helper()
	if _, err := d.Pool().Exec(context.Background(), `INSERT INTO seller.stores (store_id, store_code, account_id, store_name, created_at, updated_at)
		OVERRIDING SYSTEM VALUE SELECT g, g, 2000000 + g, 's', now(), now() FROM generate_series($1::bigint, $2::bigint) AS g`, lo, hi); err != nil {
		t.Fatal(err)
	}
}

var epoch = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

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
	owner := buyer.New(buyer.Deps{DB: d, Events: bus, Media: media.NewFake(), Now: now, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	id := identity.New(identity.Deps{DB: d, Events: bus, Now: now, Logger: log})
	sl := seller.New(seller.Deps{KYCMAC: sealtest.MAC(t), DB: d, Events: bus, Jobs: q, Identity: id, Media: media.NewFake(), Orders: noOrders{}, ProductStores: noProducts{}, Registry: noRegistry{}, Now: now, Logger: log,
		Payment: payment.New(payment.Deps{DB: d, Audit: adminaudit.New(d, now), Events: bus, Jobs: q, Now: now, Logger: log})})
	gate := &suspendable{Module: sl}
	seedStores(t, d, storeA, storeC)
	seedBuyers(t, d, buyer1, buyer1+99)
	logs := &logSink{}
	m := crm.New(crm.Deps{DB: d, Jobs: q, Buyers: owner, Stores: gate, NewID: newIDs(), Now: now, Logger: slog.New(slog.NewJSONHandler(logs, nil))})
	events.Subscribe(bus, m.OnBuyerDeleted)
	events.Subscribe(bus, m.OnStoreRetired)
	return &fixture{t: t, db: d, m: m, q: q, owner: owner, bus: bus, logs: logs, gate: gate}
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

func errOnly[T any](_ T, err error) error { return err }
