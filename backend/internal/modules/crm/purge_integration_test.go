//go:build integration

package crm_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

func (f *fixture) seedBuyerEverywhere(buyerID int64) {
	f.t.Helper()
	for i, store := range []int64{storeA, storeB} {
		o := "ORD-" + sn(int(buyerID)) + "-" + sn(i)
		f.mustOrder("evt-"+o, o, store, buyerID, 1000)
		if _, err := f.partial("REF-"+o, o, store, buyerID, 100); err != nil {
			f.t.Fatal(err)
		}
		if err := f.m.AddCustomerTags(context.Background(), store, buyerID, []string{"常客"}); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *fixture) deleteBuyer(buyerID int64) {
	f.t.Helper()
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		return f.m.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: buyerID, OccurredAt: epoch})
	})
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) retireStore(storeID int64) {
	f.t.Helper()
	if err := f.retireStoreErr(storeID); err != nil {
		f.t.Fatal(err)
	}
}

// retireStoreErr 讓店家退場並在同 transaction 通知訂閱者；回傳錯誤，讓測試可以在 goroutine 內呼叫。
func (f *fixture) retireStoreErr(storeID int64) error {
	return f.db.InTx(context.Background(), func(ctx context.Context) error {
		return f.m.OnStoreRetired(ctx, seller.StoreRetired{StoreID: storeID, OccurredAt: epoch})
	})
}

func (f *fixture) rowsOfBuyer(buyerID int64) int64 {
	return f.scalar(`SELECT (SELECT count(*) FROM crm.customers WHERE buyer_id = $1)
	  + (SELECT count(*) FROM crm.order_nets WHERE buyer_id = $1)
	  + (SELECT count(*) FROM crm.ledger l JOIN crm.order_nets o USING (order_no) WHERE o.buyer_id = $1)
	  + (SELECT count(*) FROM crm.customer_tags WHERE buyer_id = $1)`, buyerID)
}

// W2 (b)：買家刪帳 → 訂閱者只記 inactive_parties 並排 job（不動客戶列）；job 清掉該買家在各店的列，別人的不動。
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
		t.Fatal("訂閱者在事件 transaction 內就改了客戶列（單體規則 4：只記複本與排 job）")
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
	// 冪等：重跑不報錯。
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
	if err := f.m.ErasePartyData(context.Background(), "BUYER", buyer1); err != nil {
		t.Fatal(err)
	}
	o := "ORD-" + sn(int(buyer1)) + "-0"
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

// 保底掃描：已在 inactive_parties 又還有資料的身分補排 job；清完之後不再補排。
func TestSweepRequeuesPartiesStillHoldingData(t *testing.T) {
	t.Parallel()
	f := setup(t)
	f.seedBuyerEverywhere(buyer1)
	f.seedBuyerEverywhere(buyer2)
	f.exec("INSERT INTO crm.inactive_parties (kind, party_id, since) VALUES ('BUYER', $1, $2)", buyer1, epoch)
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

// 投影與 Deactivate 交錯（兩條連線控制交錯）：Deactivate 的 transaction 先持身分鎖並寫了 inactive_parties 但還沒 commit，
// 這時進來的投影必須等它 commit 再查 inactive_parties，看到「已停用」而不寫任何東西。沒有身分鎖時，投影讀到「未停用」就寫入並 commit，
// 之後 Deactivate commit、清除 job 跑完也不會再回頭刪它。
func TestProjectionWaitsForConcurrentDeactivation(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	deactivated := make(chan struct{})
	release := make(chan struct{})
	txDone := make(chan error, 1)
	go func() {
		txDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if err := f.m.OnBuyerDeleted(ctx, buyer.BuyerDeleted{BuyerID: buyer1, OccurredAt: epoch}); err != nil {
				return err
			}
			close(deactivated)
			<-release
			return nil
		})
	}()
	<-deactivated
	type result struct {
		ok  bool
		err error
	}
	res := make(chan result, 1)
	go func() {
		ok, err := f.order("evt-1", "O1", storeA, buyer1, 1000)
		res <- result{ok, err}
	}()
	// 投影卡在身分鎖上（advisory lock 未被授予）；以輪詢等到它出現，不靠固定的睡眠長度。
	deadline := time.Now().Add(10 * time.Second)
	for f.scalar("SELECT count(*) FROM pg_locks WHERE locktype = 'advisory' AND NOT granted") == 0 {
		select {
		case r := <-res:
			// 沒有身分鎖：投影在 Deactivate commit 之前就跑完了。
			close(release)
			if err := <-txDone; err != nil {
				t.Error(err)
			}
			t.Fatalf("投影沒有等 Deactivate 的身分鎖就結束了：recorded=%v err=%v", r.ok, r.err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("等不到投影卡在身分鎖上")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if err := <-txDone; err != nil {
		t.Fatal(err)
	}
	r := <-res
	if r.err != nil || r.ok {
		t.Fatalf("recorded=%v err=%v，預期 false、nil（買家已停用）", r.ok, r.err)
	}
	if f.count("crm.customers")+f.count("crm.order_nets")+f.count("crm.ledger") != 0 {
		t.Fatal("投影在買家停用之後仍寫入了資料")
	}
}
