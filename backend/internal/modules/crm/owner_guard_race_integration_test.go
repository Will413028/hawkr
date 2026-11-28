//go:build integration

package crm_test

import (
	"context"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// 買家關閉與客戶層寫入交錯：關閉的 transaction 持買家列的 FOR UPDATE 時，寫入要等在守門鎖（LockNotClosed 的 FOR KEY SHARE）上；
// 關閉提交之後寫入回 CUSTOMER_NOT_FOUND，什麼都沒寫。拿掉 lockCustomerParties 的 LockNotClosed，寫入不會等（它的事後可見性檢查只在
// 單執行緒下也回 NotFound），這個測試在「應該等在守門鎖上」失敗。
func TestCustomerWriteWaitsForBuyerClosureAndWritesNothing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		write func(f *fixture) error
		clean func(f *fixture) bool
	}{
		{
			"SetCustomerType",
			func(f *fixture) error {
				return errOnly(f.m.SetCustomerType(context.Background(), f.storeA, buyer1, crm.TypeVIP))
			},
			func(f *fixture) bool {
				return f.scalar("SELECT count(*) FROM crm.customers WHERE customer_type <> 'NORMAL'") == 0
			},
		},
		{
			"AddCustomerTags",
			func(f *fixture) error {
				return errOnly(f.m.AddCustomerTags(context.Background(), f.storeA, buyer1, []string{"晚到"}))
			},
			func(f *fixture) bool {
				return f.count("crm.tags") == 0 && f.count("crm.customer_tags") == 0
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := setup(t)
			ctx := context.Background()
			f.mustOrder("evt-1", "O1", f.storeA, buyer1, 100)

			closer, err := f.db.Pool().Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer closer.Rollback(ctx) //nolint:errcheck
			if _, err := closer.Exec(ctx, "SELECT 1 FROM buyer.buyers WHERE buyer_id = $1 FOR UPDATE", buyer1); err != nil {
				t.Fatal(err)
			}

			done := make(chan error, 1)
			go func() { done <- c.write(f) }()
			deadline := time.Now().Add(10 * time.Second)
			for f.lockWaiters() < 1 {
				select {
				case err := <-done:
					t.Fatalf("應該等在守門鎖上，卻已回傳：%v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("寫入沒有等在守門鎖上")
				}
				time.Sleep(5 * time.Millisecond)
			}
			select {
			case err := <-done:
				t.Fatalf("關閉提交之前寫入不能完成：%v", err)
			default:
			}

			if _, err := closer.Exec(ctx, "UPDATE buyer.buyers SET status = 'CLOSED', deleted_at = now(), updated_at = now(), version = version + 1 WHERE buyer_id = $1", buyer1); err != nil {
				t.Fatal(err)
			}
			if err := closer.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			wantKind(t, <-done, apperr.KindNotFound, crm.ReasonCustomerNotFound)
			if !c.clean(f) {
				t.Fatal("被擋的寫入留下了資料")
			}
		})
	}
}
