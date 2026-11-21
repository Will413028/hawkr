//go:build integration

package crm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// markInactive 讓擁有者的狀態變成已停用（買家 CLOSED、店家已退場）：crm 只讀擁有者的 pub 函式，不存複本。
func (f *fixture) markInactive(kind string, id int64) {
	f.t.Helper()
	ctx := context.Background()
	if kind == "BUYER" {
		f.exec("UPDATE buyer.buyers SET status = 'CLOSED', deleted_at = now() WHERE buyer_id = $1", id)
		return
	}
	if err := f.db.InTx(ctx, func(ctx context.Context) error { return closeStoreRows(ctx, f.db, id) }); err != nil {
		f.t.Fatal(err)
	}
}

// 可見性在清除 job 跑之前就已經改好：已停用（買家關閉、店家退場）的買家或店家，讀取看不到、賣家寫入被拒（客戶不存在、標籤不存在），資料還在表裡（job 還沒跑）。
func TestInactiveBuyerIsInvisibleAndReadOnlyToSellers(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeA, buyer1, 100)
	f.mustOrder("evt-2", "O2", storeA, buyer2, 100)
	f.mustAddTags(storeA, buyer1, "舊")
	f.mustAddTags(storeA, buyer2, "舊")
	if err := f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeVIP); err != nil {
		t.Fatal(err)
	}
	f.markInactive("BUYER", buyer1)

	t.Run("ListCustomers", func(t *testing.T) {
		page, err := f.m.ListCustomers(ctx, crm.CustomerListInput{StoreID: storeA, Sort: crm.SortTotalSpent})
		if err != nil || len(page.Customers) != 1 || page.Customers[0].BuyerID != buyer2 {
			t.Fatalf("page=%+v err=%v，預期只有買家 2", page.Customers, err)
		}
	})
	t.Run("GetCustomer", func(t *testing.T) {
		_, err := f.m.GetCustomer(ctx, storeA, buyer1)
		wantKind(t, err, apperr.KindNotFound, crm.ReasonCustomerNotFound)
	})
	t.Run("VIPStatuses", func(t *testing.T) {
		got, err := f.m.VIPStatuses(ctx, buyer1, []int64{storeA})
		if err != nil || got[storeA] {
			t.Fatalf("got=%v err=%v，預期 false", got, err)
		}
	})
	t.Run("SetCustomerType", func(t *testing.T) {
		wantKind(t, f.m.SetCustomerType(ctx, storeA, buyer1, crm.TypeNormal), apperr.KindNotFound, crm.ReasonCustomerNotFound)
		if n := f.scalar("SELECT count(*) FROM crm.customers WHERE buyer_id = $1 AND customer_type = 'VIP'", buyer1); n != 1 {
			t.Fatal("被拒的寫入改了資料")
		}
	})
	t.Run("AddCustomerTags", func(t *testing.T) {
		wantKind(t, f.m.AddCustomerTags(ctx, storeA, buyer1, []string{"新"}), apperr.KindNotFound, crm.ReasonCustomerNotFound)
		if got := f.tagNames(storeA); len(got) != 1 {
			t.Fatalf("標籤 %v，被拒的寫入不該建立新標籤", got)
		}
	})
}

func TestInactiveStoreIsInvisibleAndReadOnlyToSellers(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	f.mustOrder("evt-1", "O1", storeB, buyer1, 100)
	f.mustAddTags(storeB, buyer1, "舊")
	if err := f.m.SetCustomerType(ctx, storeB, buyer1, crm.TypeVIP); err != nil {
		t.Fatal(err)
	}
	tags, _ := f.m.ListTags(ctx, storeB)
	tagID := tags[0].ID
	f.markInactive("STORE", storeB)

	t.Run("ListCustomers", func(t *testing.T) {
		page, err := f.m.ListCustomers(ctx, crm.CustomerListInput{StoreID: storeB, Sort: crm.SortTotalSpent})
		if err != nil || len(page.Customers) != 0 {
			t.Fatalf("page=%+v err=%v，預期空", page.Customers, err)
		}
	})
	t.Run("GetCustomer", func(t *testing.T) {
		_, err := f.m.GetCustomer(ctx, storeB, buyer1)
		wantKind(t, err, apperr.KindNotFound, crm.ReasonCustomerNotFound)
	})
	t.Run("ListTags", func(t *testing.T) {
		if got := f.tagNames(storeB); len(got) != 0 {
			t.Fatalf("標籤 %v，預期空", got)
		}
	})
	t.Run("VIPStatuses", func(t *testing.T) {
		got, err := f.m.VIPStatuses(ctx, buyer1, []int64{storeB})
		if err != nil || got[storeB] {
			t.Fatalf("got=%v err=%v，預期 false", got, err)
		}
	})
	t.Run("SetCustomerType", func(t *testing.T) {
		wantKind(t, f.m.SetCustomerType(ctx, storeB, buyer1, crm.TypeNormal), apperr.KindNotFound, crm.ReasonCustomerNotFound)
	})
	t.Run("RemoveCustomerTag", func(t *testing.T) {
		wantKind(t, f.m.RemoveCustomerTag(ctx, storeB, buyer1, tagID), apperr.KindNotFound, crm.ReasonCustomerNotFound)
	})
	t.Run("AddCustomerTags", func(t *testing.T) {
		wantKind(t, f.m.AddCustomerTags(ctx, storeB, buyer1, []string{"新"}), apperr.KindNotFound, crm.ReasonCustomerNotFound)
		if n := f.scalar("SELECT count(*) FROM crm.tags WHERE store_id = $1", storeB); n != 1 {
			t.Fatalf("店 B 有 %d 個標籤，預期維持 1", n)
		}
	})
	t.Run("CreateTag", func(t *testing.T) {
		_, err := f.m.CreateTag(ctx, storeB, "新")
		wantKind(t, err, apperr.KindNotFound, crm.ReasonTagNotFound)
	})
	t.Run("RenameTag", func(t *testing.T) {
		wantKind(t, f.m.RenameTag(ctx, storeB, tagID, "改"), apperr.KindNotFound, crm.ReasonTagNotFound)
	})
	t.Run("DeleteTag", func(t *testing.T) {
		wantKind(t, f.m.DeleteTag(ctx, storeB, tagID), apperr.KindNotFound, crm.ReasonTagNotFound)
		if n := f.scalar("SELECT count(*) FROM crm.tags WHERE store_id = $1 AND name = '舊'", storeB); n != 1 {
			t.Fatal("被拒的刪除仍刪了標籤")
		}
	})
}

// CR3：退場批次（一個 transaction 對多家店 FOR UPDATE SKIP LOCKED，被守門鎖住的店這一輪跳過、下一輪再退）同時跑這些店與多位買家的投影；
// 不得死結，最終全部退場、狀態正確。
func TestRetiringManyStoresWhileProjectingDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	const stores, buyers = 6, 6
	storeID := func(i int) int64 { return 200 + int64(i%stores) }
	seedStores(t, f.db, storeID(0), storeID(stores-1))
	seedBuyers(t, f.db, 300, 300+buyers-1)
	started := make(chan struct{})
	retired := make(chan error, 1)
	var startOnce sync.Once
	go func() {
		pending := make([]int64, 0, stores)
		for i := 0; i < stores; i++ {
			pending = append(pending, storeID(i))
		}
		for len(pending) > 0 {
			var skipped []int64
			err := f.db.InTx(ctx, func(ctx context.Context) error {
				skipped = skipped[:0]
				for _, id := range pending {
					rows, err := f.db.Conn(ctx).Query(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 AND retired_at IS NULL FOR UPDATE SKIP LOCKED", id)
					if err != nil {
						return err
					}
					got := rows.Next()
					rows.Close()
					if !got {
						skipped = append(skipped, id)
						continue
					}
					// 第一列已鎖住才放行投影：重疊從這裡開始是確定的。
					startOnce.Do(func() { close(started) })
					if err := f.retireLocked(ctx, id); err != nil {
						return err
					}
					time.Sleep(20 * time.Millisecond)
				}
				return nil
			})
			if err != nil {
				startOnce.Do(func() { close(started) })
				retired <- err
				return
			}
			pending = append(pending[:0], skipped...)
		}
		retired <- nil
	}()
	<-started
	errs := parallel(60, func(i int) error {
		_, err := f.order(fmt.Sprintf("evt-%d", i), fmt.Sprintf("O%d", i), storeID(i), 300+int64(i%buyers), 100)
		return err
	})
	noErrors(t, errs)
	if err := <-retired; err != nil {
		t.Fatalf("退場批次：%v", err)
	}
	for i := 0; i < stores; i++ {
		if err := f.m.ErasePartyData(ctx, "STORE", storeID(i)); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.count("crm.customers") + f.count("crm.order_nets") + f.count("crm.ledger"); n != 0 {
		t.Fatalf("退場並清除後還剩 %d 列", n)
	}
	if n := f.count("seller.pub_retired_stores()"); n != stores {
		t.Fatalf("退場的店 %d 家，預期 %d", n, stores)
	}
	if n := f.q.count("crm.erase_party"); n != stores {
		t.Fatalf("清除 job %d 個，預期 %d", n, stores)
	}
}

// CR4：保底掃描分頁走完全部，超過一頁時最後面的身分也會被補排（前面的身分一直失敗不會讓它餓死）。
func TestSweepReachesPartiesBeyondOneBatch(t *testing.T) {
	t.Parallel()
	f := setup(t)
	ctx := context.Background()
	const parties = 230
	f.exec(`INSERT INTO buyer.buyers (buyer_id, account_id, status, display_name, created_at, updated_at, deleted_at)
		OVERRIDING SYSTEM VALUE SELECT g, 1000000 + g, 'CLOSED', 'b', $1, $1, $1 FROM generate_series(1000, $2::int) AS g`, epoch, 1000+parties-1)
	f.exec("INSERT INTO crm.customers (store_id, buyer_id, first_order_at, updated_at) SELECT 5, g, $1, $1 FROM generate_series(1000, $2::int) AS g", epoch, 1000+parties-1)
	seen := func() map[int64]int {
		f.q.mu.Lock()
		defer f.q.mu.Unlock()
		out := map[int64]int{}
		for _, a := range f.q.args {
			var v struct {
				PartyID int64 `json:"party_id"`
			}
			b, _ := json.Marshal(a)
			if err := json.Unmarshal(b, &v); err != nil {
				t.Fatal(err)
			}
			out[v.PartyID]++
		}
		return out
	}
	for round := 1; round <= 2; round++ {
		n, err := f.m.SweepErase(ctx)
		if err != nil || n != parties {
			t.Fatalf("第 %d 輪掃到 %d 個身分 err=%v，預期 %d", round, n, err, parties)
		}
		got := seen()
		if len(got) != parties || got[1000+parties-1] != round || got[1000] != round {
			t.Fatalf("第 %d 輪後補排了 %d 個不同的身分，最後一個 %d 次、第一個 %d 次", round, len(got), got[1000+parties-1], got[1000])
		}
	}
}
