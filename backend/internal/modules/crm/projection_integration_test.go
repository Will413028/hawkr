//go:build integration

package crm_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/crm"
)

const (
	storeA = int64(10)
	storeB = int64(11)
	buyer1 = int64(100)
	buyer2 = int64(101)
)

// 重放冪等：同一個 order.completed 送兩次、同一筆退款（同冪等鍵）送兩次 → total_spent、order_count、ledger 列數不變。
func TestReplayIsIdempotent(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 1000)
	if ok, err := f.order("evt-1", "O1", storeA, buyer1, 1000); err != nil || ok {
		t.Fatalf("重放的 order.completed：recorded=%v err=%v，預期 false、nil", ok, err)
	}
	if ok, err := f.partial("REF-1", "O1", storeA, buyer1, 300); err != nil || !ok {
		t.Fatalf("第一次部分退款：recorded=%v err=%v", ok, err)
	}
	for i := 0; i < 3; i++ {
		if ok, err := f.partial("REF-1", "O1", storeA, buyer1, 300); err != nil || ok {
			t.Fatalf("重放的部分退款：recorded=%v err=%v，預期 false、nil", ok, err)
		}
	}
	if spent, orders, _ := f.customer(storeA, buyer1); spent != 700 || orders != 1 {
		t.Fatalf("total_spent=%d order_count=%d，預期 700、1", spent, orders)
	}
	if n := f.count("crm.ledger"); n != 2 {
		t.Fatalf("ledger %d 列，預期 2（一筆訂單、一筆退款）", n)
	}
	f.assertInvariant(storeA, buyer1)
}

// 退款的冪等鍵與 order.completed 的 event_id 在不同命名空間：同一個字串不會互相擋掉。
func TestOrderAndRefundKeysDoNotCollide(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("same-key", "O1", storeA, buyer1, 500)
	if ok, err := f.partial("same-key", "O1", storeA, buyer1, 100); err != nil || !ok {
		t.Fatalf("與 order.completed 同字串的退款鍵被擋：recorded=%v err=%v", ok, err)
	}
	if spent, _, _ := f.customer(storeA, buyer1); spent != 400 {
		t.Fatalf("total_spent=%d，預期 400", spent)
	}
}

// 同一單號的第二個 order.completed（換了 event_id）不重複累加。
func TestSecondOrderCompletedWithNewEventIDDoesNotDoubleCount(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 1000)
	if _, err := f.order("evt-2", "O1", storeA, buyer1, 1000); err != nil {
		t.Fatal(err)
	}
	if spent, orders, _ := f.customer(storeA, buyer1); spent != 1000 || orders != 1 {
		t.Fatalf("total_spent=%d order_count=%d，預期 1000、1", spent, orders)
	}
}

// 事件順序表格（design §11.1）：每個順序的 total_spent 與 order_count，且過程中從不 < 0（CHECK 也會讓負值直接失敗）。
func TestEventOrderingTable(t *testing.T) {
	t.Parallel()
	type event struct {
		kind   string
		amount int64
	}
	tests := []struct {
		name       string
		events     []event
		wantSpent  int64
		wantOrders int64
		wantRow    bool
	}{
		{"訂單→部分→全額", []event{{"order", 1000}, {"partial", 300}, {"full", 0}}, 0, 1, true},
		{"全額→訂單", []event{{"full", 0}, {"order", 1000}}, 0, 1, true},
		{"部分→訂單", []event{{"partial", 300}, {"order", 1000}}, 700, 1, true},
		{"訂單→全額→部分（重試排後）", []event{{"order", 1000}, {"full", 0}, {"partial", 300}}, 0, 1, true},
		{"訂單→部分", []event{{"order", 1000}, {"partial", 300}}, 700, 1, true},
		{"只有退款", []event{{"partial", 300}, {"full", 0}}, 0, 0, false},
	}
	for i, tc := range tests {
		buyer := buyer1 + int64(i)
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := setup(t)
			for j, e := range tc.events {
				var err error
				switch e.kind {
				case "order":
					_, err = f.order(fmt.Sprintf("evt-%d", j), "O1", storeA, buyer, e.amount)
				case "partial":
					_, err = f.partial(fmt.Sprintf("REF-%d", j), "O1", storeA, buyer, e.amount)
				case "full":
					_, err = f.full(fmt.Sprintf("REF-%d", j), "O1", storeA, buyer)
				}
				if err != nil {
					t.Fatalf("第 %d 個事件 %s：%v", j, e.kind, err)
				}
			}
			spent, orders, ok := f.customer(storeA, buyer)
			if ok != tc.wantRow || spent != tc.wantSpent || orders != tc.wantOrders {
				t.Fatalf("row=%v total_spent=%d order_count=%d，預期 %v、%d、%d", ok, spent, orders, tc.wantRow, tc.wantSpent, tc.wantOrders)
			}
			f.assertInvariant(storeA, buyer)
		})
	}
}

// Q11：退款先到不建客戶列（舊碼會以 order_count=0 建客戶）。
func TestRefundBeforeOrderDoesNotCreateCustomer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	if _, err := f.partial("REF-1", "O1", storeA, buyer1, 300); err != nil {
		t.Fatal(err)
	}
	if _, err := f.full("REF-2", "O2", storeA, buyer1); err != nil {
		t.Fatal(err)
	}
	if n := f.count("crm.customers"); n != 0 {
		t.Fatalf("退款建立了 %d 個客戶列", n)
	}
	if n := f.count("crm.order_nets"); n != 2 {
		t.Fatalf("order_nets %d 列，預期 2（退款只記在單上）", n)
	}
}

// 同單的訂單與退款並行 N 次：結果與任何序列化順序一致。
func TestConcurrentOrderAndRefundOnSameOrder(t *testing.T) {
	t.Parallel()
	f := setup(t)
	const n = 12
	// 訂單與全額退款：兩種順序的結果都是 0。
	errs := parallel(2*n, func(i int) error {
		o := fmt.Sprintf("OF%d", i/2)
		buyer := buyer1 + int64(i/2)
		if i%2 == 0 {
			_, err := f.order("evt-"+o, o, storeA, buyer, 1000)
			return err
		}
		_, err := f.full("REF-"+o, o, storeA, buyer)
		return err
	})
	noErrors(t, errs)
	for i := 0; i < n; i++ {
		spent, orders, ok := f.customer(storeA, buyer1+int64(i))
		if !ok || spent != 0 || orders != 1 {
			t.Fatalf("OF%d：row=%v total_spent=%d order_count=%d，預期 0、1", i, ok, spent, orders)
		}
		f.assertInvariant(storeA, buyer1+int64(i))
	}
	// 訂單與部分退款：兩種順序的結果都是 700。
	errs = parallel(2*n, func(i int) error {
		o := fmt.Sprintf("OP%d", i/2)
		buyer := buyer2 + 1000 + int64(i/2)
		if i%2 == 0 {
			_, err := f.order("evt-"+o, o, storeB, buyer, 1000)
			return err
		}
		_, err := f.partial("REF-"+o, o, storeB, buyer, 300)
		return err
	})
	noErrors(t, errs)
	for i := 0; i < n; i++ {
		if spent, _, _ := f.customer(storeB, buyer2+1000+int64(i)); spent != 700 {
			t.Fatalf("OP%d：total_spent=%d，預期 700", i, spent)
		}
	}
}

// 同一客戶、不同訂單的事件並行：total_spent 等於事後 SUM(GREATEST(net_amount, 0)) 與期望值（READ COMMITTED 下的 lost update 守門）。
func TestConcurrentOrdersOfOneCustomer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	const orders = 24
	var want int64
	for i := 0; i < orders; i++ {
		amount := int64(100 + i*7)
		refund := int64(0)
		if i%3 == 0 {
			refund = amount / 4
		}
		want += amount - refund
	}
	// 客戶列被更新後多持有 20ms 才放開（只拉寬「後到的語句在前一個 transaction 未提交時開始」的窗口，不是時間斷言）：
	// 以 SUM 子查詢重算 total_spent 時，後到者用語句開始時的快照，會漏掉前一個剛提交的訂單。
	f.exec(`CREATE FUNCTION crm.hold_customer_row() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_sleep(0.02); RETURN NULL; END $$`)
	f.exec(`CREATE TRIGGER hold_customer_row AFTER INSERT OR UPDATE ON crm.customers FOR EACH ROW EXECUTE FUNCTION crm.hold_customer_row()`)
	errs := parallel(orders*2, func(i int) error {
		k := i / 2
		o := fmt.Sprintf("C%d", k)
		amount := int64(100 + k*7)
		if i%2 == 0 {
			_, err := f.order("evt-"+o, o, storeA, buyer1, amount)
			return err
		}
		if k%3 != 0 {
			return nil
		}
		_, err := f.partial("REF-"+o, o, storeA, buyer1, amount/4)
		return err
	})
	noErrors(t, errs)
	spent, count, ok := f.customer(storeA, buyer1)
	if !ok || spent != want || count != orders {
		t.Fatalf("row=%v total_spent=%d（期望 %d）order_count=%d（期望 %d）", ok, spent, want, count, orders)
	}
	f.assertInvariant(storeA, buyer1)
}

// 驗證層：不合法的事件回 ErrInvalidEvent，什麼都不寫。
func TestInvalidEventsAreRejectedWithoutWrites(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	long := make([]byte, crm.MaxEventKeyLen+1)
	for i := range long {
		long[i] = 'k'
	}
	bad := []func() error{
		func() error { _, err := f.order("", "O1", storeA, buyer1, 10); return err },
		func() error { _, err := f.order(string(long), "O1", storeA, buyer1, 10); return err },
		func() error { _, err := f.order("e", "", storeA, buyer1, 10); return err },
		func() error { _, err := f.order("e", "O1", 0, buyer1, 10); return err },
		func() error { _, err := f.order("e", "O1", storeA, -1, 10); return err },
		func() error { _, err := f.order("e", "O1", storeA, buyer1, -10); return err },
		func() error { _, err := f.partial("k", "O1", storeA, buyer1, 0); return err },
		func() error {
			_, err := f.m.RecordRefund(ctx, crm.Refund{Key: "k", OrderNo: "O1", StoreID: storeA, BuyerID: buyer1, Outcome: "WHATEVER"})
			return err
		},
	}
	for i, call := range bad {
		if err := call(); !errors.Is(err, crm.ErrInvalidEvent) {
			t.Errorf("第 %d 個壞輸入：err=%v，預期 ErrInvalidEvent", i, err)
		}
	}
	if f.count("crm.order_nets")+f.count("crm.ledger")+f.count("crm.customers") != 0 {
		t.Fatal("壞輸入寫入了資料")
	}
}

// 同一單號帶不同的買家：上游自相矛盾，回 ErrPartiesMismatch，不改任何東西。
func TestPartiesMismatchIsRejected(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.mustOrder("evt-1", "O1", storeA, buyer1, 1000)
	if _, err := f.partial("REF-1", "O1", storeA, buyer2, 100); !errors.Is(err, crm.ErrPartiesMismatch) {
		t.Fatalf("err=%v，預期 ErrPartiesMismatch", err)
	}
	if spent, _, _ := f.customer(storeA, buyer1); spent != 1000 {
		t.Fatalf("total_spent=%d", spent)
	}
	if n := f.count("crm.ledger"); n != 1 {
		t.Fatalf("ledger %d 列，預期 1（錯誤的退款沒有留下紀錄）", n)
	}
}
