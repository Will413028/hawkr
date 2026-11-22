//go:build integration

package crmconsumer

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/testdb"
)

var pg *testdb.Server

func TestMain(m *testing.M) { testdb.Main(m, &pg) }

// parties 是舊訂單服務的替身：單號 → (買家, 店家)。
type parties map[string][2]int64

func (p parties) OrderParties(_ context.Context, orderNo string) (orderservice.OrderParties, error) {
	v, ok := p[orderNo]
	if !ok {
		return orderservice.OrderParties{}, orderservice.ErrOrderNotFound
	}
	return orderservice.OrderParties{OrderNo: orderNo, BuyerID: v[0], StoreID: v[1], Status: "50"}, nil
}

// openGuards 是 crm 的守門 port 的替身：買家與店家都營運中（這組測試驗證消費端的冪等，守門本身由 crm 的整合測試驗證）。
type openGuards struct{}

func (openGuards) LockNotClosed(context.Context, int64) error  { return nil }
func (openGuards) LockNotRetired(context.Context, int64) error { return nil }
func (openGuards) Store(context.Context, int64) (seller.Store, error) {
	return seller.Store{}, nil
}

type env struct {
	t   *testing.T
	d   *db.DB
	crm *crm.Module
	c   Consumer
	q   *recordedJobs
}

func setup(t *testing.T, p parties) *env {
	t.Helper()
	d := pg.NewDB(t, testdb.Jobs(), testdb.Migrations(
		migrate.Module{Name: "buyer", Dir: buyer.Migrations()},
		migrate.Module{Name: "seller", Dir: seller.Migrations()},
		migrate.Module{Name: "crm", Dir: crm.Migrations()},
	))
	client, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	m := crm.New(crm.Deps{DB: d, Jobs: client, Buyers: openGuards{}, Stores: openGuards{}, NewID: uuid.NewV7, Now: func() time.Time { return time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC) }})
	q := &recordedJobs{}
	return &env{t: t, d: d, crm: m, q: q, c: Consumer{CRM: m, Orders: p, Jobs: q}}
}

// run 把 handler 排出的 job 全部交給 worker 各執行一次（模擬 River），再清空佇列。
func (e *env) run() {
	e.t.Helper()
	for _, a := range e.q.args {
		var err error
		switch v := a.(type) {
		case OrderCompletedArgs:
			err = (&orderCompletedWorker{crm: e.crm}).Work(context.Background(), &river.Job[OrderCompletedArgs]{Args: v})
		case RefundCompletedArgs:
			err = (&refundCompletedWorker{crm: e.crm, orders: e.c.Orders}).Work(context.Background(), &river.Job[RefundCompletedArgs]{Args: v})
		}
		if err != nil {
			e.t.Fatalf("worker：%v", err)
		}
	}
	e.q.args = nil
}

func (e *env) spent(store, buyer int64) (spent, orders int64) {
	e.t.Helper()
	err := e.d.Pool().QueryRow(context.Background(), "SELECT total_spent, order_count FROM crm.customers WHERE store_id = $1 AND buyer_id = $2", store, buyer).Scan(&spent, &orders)
	if err != nil {
		e.t.Fatal(err)
	}
	return spent, orders
}

func (e *env) count(table string) int64 {
	e.t.Helper()
	var n int64
	if err := e.d.Pool().QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

func orderBody(eventID, orderNo string, buyer, amount int) string {
	return fmt.Sprintf(`{"event_id":%q,"order_no":%q,"buyer_id":"%d","store_id":"5","total_actual_amount":"%d"}`, eventID, orderNo, buyer, amount)
}

func refundBody(eventID, caseNo, orderNo, outcome string, amount int, ext string) string {
	return fmt.Sprintf(`{"event_id":%q,"case_no":%q,"order_no":%q,"refund_outcome":%q,"refund_amount":"%d","external_ref_id":%q}`, eventID, caseNo, orderNo, outcome, amount, ext)
}

// 重放冪等（端到端）：同一 order.completed 送兩次、同一退款以不同 event_id 重發（Java 每小時重發都換 UUID）、同 external_ref_id 送兩次，
// 每個 job 都真的執行過（不靠 River 的唯一鍵）→ total_spent、order_count、ledger 列數不變。
func TestReplaysThroughConsumerAreIdempotent(t *testing.T) {
	t.Parallel()
	e := setup(t, parties{"O1": {71, 5}})
	ctx := context.Background()
	for _, body := range []string{orderBody("evt-1", "O1", 71, 1000), orderBody("evt-1", "O1", 71, 1000)} {
		if err := e.c.OrderCompleted(ctx, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{
		refundBody("uuid-a", "C1", "O1", "PARTIAL_REFUND", 300, "REF-1"),
		refundBody("uuid-b", "C1", "O1", "PARTIAL_REFUND", 300, "REF-1"), // 每小時重發：新的 event_id
		refundBody("uuid-c", "C1", "O1", "PARTIAL_REFUND", 300, "REF-1"),
		refundBody("uuid-a", "C1", "O1", "PARTIAL_REFUND", 300, "REF-1"), // 同一個 event_id 再送
	} {
		if err := e.c.RefundCompleted(ctx, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	e.run()
	e.run() // 同樣的 job 再跑一輪（River 重試或兩份 queue）。
	// 全部處理完之後又來一次重放。
	if err := e.c.OrderCompleted(ctx, []byte(orderBody("evt-1", "O1", 71, 1000))); err != nil {
		t.Fatal(err)
	}
	if err := e.c.RefundCompleted(ctx, []byte(refundBody("uuid-z", "C1", "O1", "PARTIAL_REFUND", 300, "REF-1"))); err != nil {
		t.Fatal(err)
	}
	e.run()
	if spent, orders := e.spent(5, 71); spent != 700 || orders != 1 {
		t.Fatalf("total_spent=%d order_count=%d，預期 700、1", spent, orders)
	}
	if n := e.count("crm.ledger"); n != 2 {
		t.Fatalf("ledger %d 列，預期 2", n)
	}
}

// 缺 external_ref_id 時用 case_no 導出鍵：同一個 case 的重發同樣只扣一次。
func TestRefundWithoutExternalRefIDDedupsOnCaseNo(t *testing.T) {
	t.Parallel()
	e := setup(t, parties{"O1": {71, 5}})
	ctx := context.Background()
	if err := e.c.OrderCompleted(ctx, []byte(orderBody("evt-1", "O1", 71, 1000))); err != nil {
		t.Fatal(err)
	}
	e.run()
	for _, id := range []string{"u1", "u2"} {
		if err := e.c.RefundCompleted(ctx, []byte(refundBody(id, "CASE-9", "O1", "PARTIAL_REFUND", 400, ""))); err != nil {
			t.Fatal(err)
		}
	}
	e.run()
	if spent, _ := e.spent(5, 71); spent != 600 {
		t.Fatalf("total_spent=%d，預期 600", spent)
	}
}

// 退款事件經真的 River 排程：同冪等鍵、不同 event_id 的重發只排一個 job（唯一鍵是退款鍵，不是 event_id）。
func TestRefundRedeliveryEnqueuesOneRiverJob(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d := pg.NewDB(t, testdb.Jobs())
	client, err := jobs.NewInsertOnly(d)
	if err != nil {
		t.Fatal(err)
	}
	c := Consumer{Jobs: client}
	for _, id := range []string{"u1", "u2", "u3"} {
		if err := c.RefundCompleted(ctx, []byte(refundBody(id, "C1", "O1", "FULL_REFUND", 0, "REF-1"))); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.OrderCompleted(ctx, []byte(orderBody("evt-1", "O1", 71, 1))); err != nil {
		t.Fatal(err)
	}
	if err := c.OrderCompleted(ctx, []byte(orderBody("evt-1", "O1", 71, 1))); err != nil {
		t.Fatal(err)
	}
	var refunds, orders int
	if err := d.Pool().QueryRow(ctx, `SELECT count(*) FILTER (WHERE kind = 'legacy.crm_refund_completed'), count(*) FILTER (WHERE kind = 'legacy.crm_order_completed') FROM river.river_job`).Scan(&refunds, &orders); err != nil {
		t.Fatal(err)
	}
	if refunds != 1 || orders != 1 {
		t.Fatalf("退款 job %d、訂單 job %d，預期各 1", refunds, orders)
	}
}

// 事件順序（端到端）：訂單→全額→部分（重試排後）total_spent 為 0；全額退款先到、訂單後到也是 0；退款先到不建客戶列。
func TestOutOfOrderEventsThroughConsumer(t *testing.T) {
	t.Parallel()
	e := setup(t, parties{"O1": {71, 5}, "O2": {71, 5}, "O3": {72, 5}})
	ctx := context.Background()
	steps := []func() error{
		func() error { return e.c.OrderCompleted(ctx, []byte(orderBody("e1", "O1", 71, 1000))) },
		func() error {
			return e.c.RefundCompleted(ctx, []byte(refundBody("r1", "C1", "O1", "FULL_REFUND", 999, "REF-1")))
		},
		func() error {
			return e.c.RefundCompleted(ctx, []byte(refundBody("r2", "C2", "O1", "PARTIAL_REFUND", 300, "REF-2")))
		},
	}
	for _, s := range steps {
		if err := s(); err != nil {
			t.Fatal(err)
		}
		e.run()
	}
	if spent, orders := e.spent(5, 71); spent != 0 || orders != 1 {
		t.Fatalf("訂單→全額→部分：total_spent=%d order_count=%d，預期 0、1", spent, orders)
	}
	if err := e.c.RefundCompleted(ctx, []byte(refundBody("r3", "C3", "O3", "FULL_REFUND", 0, "REF-3"))); err != nil {
		t.Fatal(err)
	}
	e.run()
	if n := e.count("crm.customers"); n != 1 {
		t.Fatalf("退款先到建立了客戶列：customers=%d，預期 1（只有買家 71）", n)
	}
	if err := e.c.OrderCompleted(ctx, []byte(orderBody("e3", "O3", 72, 500))); err != nil {
		t.Fatal(err)
	}
	e.run()
	if spent, orders := e.spent(5, 72); spent != 0 || orders != 1 {
		t.Fatalf("全額→訂單：total_spent=%d order_count=%d，預期 0、1", spent, orders)
	}
}
