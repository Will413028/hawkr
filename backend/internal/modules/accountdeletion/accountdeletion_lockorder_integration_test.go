//go:build integration

package accountdeletion

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

type noStoreBlocks struct{}

func (noStoreBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }

type noProductChecks struct{}

func (noProductChecks) VerifyProduct(context.Context, int64, int64) error {
	return social.ErrProductNotAvailable
}

// lockedNow 回報一列現在是不是被別的 transaction 鎖著（NOWAIT 拿不到，SQLSTATE 55P03）。
func (f *fixture) lockedNow(t *testing.T, sql string, args ...any) bool {
	t.Helper()
	var n int64
	err := f.db.Conn(context.Background()).QueryRow(context.Background(), sql, args...).Scan(&n)
	var pe *pgconn.PgError
	switch {
	case err == nil:
		return false
	case errors.As(err, &pe) && pe.Code == "55P03":
		return true
	default:
		t.Fatalf("%v\n%s", err, sql)
		return false
	}
}

// 鎖順序（單體規則 4：identity 帳號列 → buyer → seller → 上層模組）：帳號刪除的 transaction 在店家列上被擋住的時候，買家列已鎖、
// social 的列還沒鎖。買家事件的同 transaction 訂閱者（social）會鎖作者的留言與 stats，所以 accountdeletion 要先鎖完買家與店家，
// 才派發買家事件（拿掉 Request 裡的先鎖：social 的列在店家列之前被鎖，這個測試失敗）。兩條路徑都量：有未結訂單（BeginDeletion 派發事件）
// 與沒有（CloseForDeletion 派發事件）。
func TestDeletionLocksBuyerAndSellerBeforeSocialRows(t *testing.T) {
	for name, open := range map[string]bool{"申請進入等待": true, "直接關閉": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setupWith(t, migrate.Module{Name: "social", Dir: social.Migrations()})
			sm := social.New(social.Deps{
				DB: f.db, Events: f.bus, Media: media.NewFake(), Jobs: f.queue, Buyers: f.buyer, Stores: seller.NewReader(f.db, time.Now), StoreBlocks: noStoreBlocks{},
				Products: noProductChecks{}, NewID: uuid.NewV7, Now: time.Now,
			})
			events.Subscribe(f.bus, sm.OnBuyerDeletionRequested)
			events.Subscribe(f.bus, sm.OnBuyerDeleted)
			id, buyerID := f.account(t)
			s := f.store(t, id)
			postID, commentID := uuid.NewV7(), uuid.NewV7()
			if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO social.posts (id, store_id, body, status, created_at, updated_at) VALUES ($1, $2, 'x', 'DRAFT', now(), now())`, postID, s.StoreID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO social.comments (id, post_id, actor_kind, actor_id, body, created_at, updated_at) VALUES ($1, $2, 'BUYER', $3, 'c', now(), now())`, commentID, postID, buyerID); err != nil {
				t.Fatal(err)
			}
			f.orders.mu.Lock()
			f.orders.open = open
			f.orders.mu.Unlock()

			// 另一個 transaction 先鎖住店家列，讓帳號刪除停在店家列上。
			held, release, holdDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			go func() {
				holdDone <- f.db.InTx(ctx, func(ctx context.Context) error {
					var n int64
					if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 FOR UPDATE", s.StoreID).Scan(&n); err != nil {
						return err
					}
					close(held)
					<-release
					return nil
				})
			}()
			<-held
			reqDone := make(chan error, 1)
			go func() { _, err := f.module.Request(ctx, adminRequest(id)); reqDone <- err }()
			deadline := time.Now().Add(15 * time.Second)
			for {
				var waiting int64
				if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting > 0 {
					break
				}
				select {
				case err := <-reqDone:
					t.Fatalf("帳號刪除沒有停在店家列上就結束了：%v", err)
				default:
				}
				if time.Now().After(deadline) {
					t.Fatal("帳號刪除沒有卡在鎖上")
				}
				time.Sleep(5 * time.Millisecond)
			}
			socialLocked := f.lockedNow(t, "SELECT 1 FROM social.comments WHERE id = $1 FOR NO KEY UPDATE NOWAIT", commentID)
			buyerLocked := f.lockedNow(t, "SELECT 1 FROM buyer.buyers WHERE buyer_id = $1 FOR UPDATE NOWAIT", buyerID)
			close(release)
			if err := <-holdDone; err != nil {
				t.Fatal(err)
			}
			if err := <-reqDone; err != nil {
				t.Fatalf("放開店家列之後帳號刪除要完成：%v", err)
			}
			var problems []string
			if !buyerLocked {
				problems = append(problems, "停在店家列上的時候買家列沒有被鎖（規則 4：buyer 在 seller 之前）")
			}
			if socialLocked {
				problems = append(problems, "停在店家列上的時候 social 的留言列已經被鎖（規則 4：上層模組在 seller 之後）")
			}
			if len(problems) > 0 {
				t.Fatal(strings.Join(problems, "；"))
			}
			// 事件確實派發了：複本在，否則上面的順序什麼也沒證明。
			var n int64
			if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM social.inactive_authors WHERE buyer_id = $1", buyerID).Scan(&n); err != nil || n != 1 {
				t.Fatalf("social 的複本 %d %v", n, err)
			}
		})
	}
}
