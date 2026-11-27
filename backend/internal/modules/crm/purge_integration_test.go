//go:build integration

package crm_test

import (
	"context"
	"errors"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
)

func (f *fixture) seedBuyerEverywhere(buyerID int64) {
	f.t.Helper()
	for i, store := range []int64{storeA, storeB} {
		o := "ORD-" + sn(int(buyerID)) + "-" + sn(i)
		f.mustOrder("evt-"+o, o, store, buyerID, 1000)
		if _, err := f.partial("REF-"+o, o, store, buyerID, 100); err != nil {
			f.t.Fatal(err)
		}
		if _, err := f.m.AddCustomerTags(context.Background(), store, buyerID, []string{"常客"}); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *fixture) deleteBuyer(buyerID int64) {
	f.t.Helper()
	ctx := context.Background()
	if err := f.owner.CloseForDeletion(ctx, buyerID); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) retireStore(storeID int64) {
	f.t.Helper()
	if err := f.retireStoreErr(storeID); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) retireStoreErr(storeID int64) error {
	ctx := context.Background()
	return f.db.InTx(ctx, func(ctx context.Context) error {
		var id int64
		if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 FOR UPDATE", storeID).Scan(&id); err != nil {
			return err
		}
		return f.retireLocked(ctx, storeID)
	})
}

func (f *fixture) retireLocked(ctx context.Context, storeID int64) error {
	if err := closeStoreRows(ctx, f.db, storeID); err != nil {
		return err
	}
	return events.Publish(ctx, f.bus, seller.StoreRetired{EventID: uuid.New(), StoreID: storeID, OccurredAt: epoch})
}

func closeStoreRows(ctx context.Context, d *db.DB, storeID int64) error {
	if _, ok := db.TxFrom(ctx); !ok {
		return errors.New("closeStoreRows 必須在 transaction 內呼叫")
	}
	if _, err := d.Conn(ctx).Exec(ctx, "INSERT INTO seller.store_closures (store_id, closed_at, closed_by) VALUES ($1, now(), 'ACCOUNT_OWNER')", storeID); err != nil {
		return err
	}
	_, err := d.Conn(ctx).Exec(ctx, "UPDATE seller.stores SET status = 'CLOSED', current_closed_at = now(), retired_at = now() WHERE store_id = $1", storeID)
	return err
}

func (f *fixture) rowsOfBuyer(buyerID int64) int64 {
	return f.scalar(`SELECT (SELECT count(*) FROM crm.customers WHERE buyer_id = $1)
	  + (SELECT count(*) FROM crm.order_nets WHERE buyer_id = $1)
	  + (SELECT count(*) FROM crm.ledger l JOIN crm.order_nets o USING (order_no) WHERE o.buyer_id = $1)
	  + (SELECT count(*) FROM crm.customer_tags WHERE buyer_id = $1)`, buyerID)
}

// W2 (b)：買家關閉 → 訂閱者只排 job（不動客戶列、不存複本）；job 清掉該買家在各店的列，別人的不動。
func TestBuyerDeletionErasesOnlyThatBuyer(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.seedBuyerEverywhere(buyer1)
	f.seedBuyerEverywhere(buyer2)
	f.deleteBuyer(buyer1)
	if n := f.q.count("crm.erase_party"); n != 1 {
		t.Fatalf("排入 %d 個清除 job，預期 1", n)
	}
	if f.rowsOfBuyer(buyer1) == 0 {
		t.Fatal("訂閱者在事件 transaction 內就改了客戶列（單體規則 4：只排 job）")
	}
	if err := f.m.ErasePartyData(context.Background(), "BUYER", buyer1); err != nil {
		t.Fatal(err)
	}
	if n := f.rowsOfBuyer(buyer1); n != 0 {
		t.Fatalf("買家 1 還剩 %d 列", n)
	}
	if n := f.rowsOfBuyer(buyer2); n != 2+2+4+2 {
		t.Fatalf("買家 2 的列數 %d 被動到（預期 客戶 2、單 2、ledger 4、貼標 2）", n)
	}
	if n := f.count("crm.tags"); n != 2 {
		t.Fatalf("標籤 %d 個：買家刪帳不刪店家的標籤定義，預期 2", n)
	}
	if err := f.m.ErasePartyData(context.Background(), "BUYER", buyer1); err != nil {
		t.Fatal(err)
	}
}

// W2 (b)：刪帳（且清除完成）後重放該買家的 order.completed 與退款，不重建 customers、order_nets、ledger。
func TestReplayAfterBuyerDeletionDoesNotRecreateRows(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.seedBuyerEverywhere(buyer1)
	f.deleteBuyer(buyer1)
	o := "ORD-" + sn(int(buyer1)) + "-0"
	before := f.rowsOfBuyer(buyer1)
	if ok, err := f.order("evt-early", o, storeA, buyer1, 1000); err != nil || ok {
		t.Fatalf("抹除前重放 order.completed：recorded=%v err=%v，預期 false、nil", ok, err)
	}
	if after := f.rowsOfBuyer(buyer1); after != before {
		t.Fatalf("抹除前的重放改了資料：%d -> %d", before, after)
	}
	if err := f.m.ErasePartyData(context.Background(), "BUYER", buyer1); err != nil {
		t.Fatal(err)
	}
	if ok, err := f.order("evt-"+o, o, storeA, buyer1, 1000); err != nil || ok {
		t.Fatalf("重放 order.completed：recorded=%v err=%v，預期 false、nil", ok, err)
	}
	if ok, err := f.partial("REF-"+o, o, storeA, buyer1, 100); err != nil || ok {
		t.Fatalf("重放部分退款：recorded=%v err=%v", ok, err)
	}
	if ok, err := f.full("REF-FULL", o, storeA, buyer1); err != nil || ok {
		t.Fatalf("重放全額退款：recorded=%v err=%v", ok, err)
	}
	if f.count("crm.customers")+f.count("crm.order_nets")+f.count("crm.ledger") != 0 {
		t.Fatal("刪帳後的重放重建了資料")
	}
}

// 店家退場：清掉該店的客戶、貼標、標籤定義、單與 ledger，別家店的不動；之後重放不重建。
func TestStoreRetirementErasesTheStore(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.seedBuyerEverywhere(buyer1)
	f.seedBuyerEverywhere(buyer2)
	f.retireStore(storeA)
	if err := f.m.ErasePartyData(context.Background(), "STORE", storeA); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"crm.customers", "crm.order_nets", "crm.tags", "crm.customer_tags"} {
		if n := f.scalar("SELECT count(*) FROM "+table+" WHERE store_id = $1", storeA); n != 0 {
			t.Errorf("%s 還有店 A 的 %d 列", table, n)
		}
	}
	if n := f.scalar("SELECT count(*) FROM crm.customers WHERE store_id = $1", storeB); n != 2 {
		t.Fatalf("店 B 的客戶 %d，預期 2", n)
	}
	if n := f.scalar("SELECT count(*) FROM crm.ledger l JOIN crm.order_nets o USING (order_no) WHERE o.store_id = $1", storeB); n != 4 {
		t.Fatalf("店 B 的 ledger %d，預期 4", n)
	}
	if ok, err := f.order("evt-again", "ORD-new", storeA, buyer1, 500); err != nil || ok {
		t.Fatalf("退場店家的重放：recorded=%v err=%v", ok, err)
	}
	if n := f.scalar("SELECT count(*) FROM crm.customers WHERE store_id = $1", storeA); n != 0 {
		t.Fatalf("退場後重建了 %d 個客戶", n)
	}
}

// 保底掃描：已關閉的買家（buyer.pub_closed_buyers()）又還有資料的身分補排 job；清完之後不再補排。
func TestSweepRequeuesPartiesStillHoldingData(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.seedBuyerEverywhere(buyer1)
	f.seedBuyerEverywhere(buyer2)
	f.markInactive("BUYER", buyer1)
	n, err := f.m.SweepErase(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("掃到 %d 個身分 err=%v，預期 1（買家 2 沒有被停用）", n, err)
	}
	if got := f.q.count("crm.erase_party"); got != 1 {
		t.Fatalf("補排 %d 個 job，預期 1", got)
	}
	if err := f.m.ErasePartyData(context.Background(), "BUYER", buyer1); err != nil {
		t.Fatal(err)
	}
	if n, err := f.m.SweepErase(context.Background()); err != nil || n != 0 {
		t.Fatalf("清完後仍掃到 %d 個身分 err=%v", n, err)
	}
}

// 投影與買家關閉交錯（兩條連線控制交錯）：關閉的 transaction 先持買家列的 FOR UPDATE 還沒 commit，這時進來的投影必須在守門鎖上等它 commit，
// 再看到「已關閉」而不寫任何東西。沒有守門鎖時，投影讀到「未關閉」就寫入並 commit，之後關閉 commit、清除 job 跑完也不會再回頭刪它。
func TestProjectionWaitsForConcurrentClosing(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	closing := make(chan struct{})
	release := make(chan struct{})
	txDone := make(chan error, 1)
	go func() {
		txDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if err := f.owner.CloseForDeletion(ctx, buyer1); err != nil {
				return err
			}
			close(closing)
			<-release
			return nil
		})
	}()
	<-closing
	type result struct {
		ok  bool
		err error
	}
	res := make(chan result, 1)
	go func() {
		ok, err := f.order("evt-1", "O1", storeA, buyer1, 1000)
		res <- result{ok, err}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for f.lockWaiters() == 0 {
		select {
		case r := <-res:
			close(release)
			if err := <-txDone; err != nil {
				t.Error(err)
			}
			t.Fatalf("投影沒有等買家關閉的列鎖就結束了：recorded=%v err=%v", r.ok, r.err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("等不到投影卡在守門鎖上")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if err := <-txDone; err != nil {
		t.Fatal(err)
	}
	r := <-res
	if r.err != nil || r.ok {
		t.Fatalf("recorded=%v err=%v，預期 false、nil（買家已關閉）", r.ok, r.err)
	}
	if f.count("crm.customers")+f.count("crm.order_nets")+f.count("crm.ledger") != 0 {
		t.Fatal("投影在買家關閉之後仍寫入了資料")
	}
}
