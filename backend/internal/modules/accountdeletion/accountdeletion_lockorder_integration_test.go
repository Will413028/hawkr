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

	"github.com/Will413028/hawkr/backend/internal/modules/catalog"
	"github.com/Will413028/hawkr/backend/internal/modules/crm"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/review"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/modules/social"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
)

type noStoreBlocks struct{}

func (noStoreBlocks) StoreBlocksBuyer(context.Context, int64, int64) (bool, error) { return false, nil }
func (noStoreBlocks) StoresBlockingBuyer(context.Context, int64, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}
func (noStoreBlocks) BuyersBlockedByStore(context.Context, int64, []int64) (map[int64]bool, error) {
	return map[int64]bool{}, nil
}

type noReviewOrders struct{}

func (noReviewOrders) OrderForReview(context.Context, string) (review.OrderForReview, error) {
	return review.OrderForReview{}, review.ErrOrderNotFound
}

type noProductChecks struct{}

func (noProductChecks) VerifyProduct(context.Context, uuid.UUID, int64) error {
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
				ScheduleAuthorizers: map[social.ScheduleKind]social.ScheduleAuthorizer{social.ScheduleByDelegation: social.ScheduleAuthorizerFunc(func(context.Context, social.ScheduledPublish) error { return social.ErrScheduleDenied })},
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

			socialTouched := func() bool {
				return f.lockedNow(t, "SELECT 1 FROM social.comments WHERE id = $1 FOR NO KEY UPDATE NOWAIT", commentID)
			}
			f.requestHeldOnStoreRow(t, id, buyerID, s.StoreID, "social 的留言列", socialTouched)
			// 事件確實派發了：複本在，否則上面的順序什麼也沒證明。
			var n int64
			if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM social.inactive_authors WHERE buyer_id = $1", buyerID).Scan(&n); err != nil || n != 1 {
				t.Fatalf("social 的複本 %d %v", n, err)
			}
		})
	}
}

type noopReviewCatalog struct{}

func (noopReviewCatalog) LockProduct(context.Context, uuid.UUID) (catalog.Rating, error) {
	return catalog.Rating{}, nil
}

func (noopReviewCatalog) SetRating(context.Context, uuid.UUID, float64, int32) error { return nil }

// review 的訂閱者在買家事件的 transaction 內只排清除 job（不寫複本），所以同樣要排在 buyer、seller 之後：帳號刪除停在店家列上時
// review 的表還沒被碰（pg_locks 沒有 review 的 RowExclusiveLock），放開後 job 才排入。買家的評價列只在 purge job 的 transaction
// 內鎖，不在刪帳的 transaction 內（訂閱者不碰評價列）。兩條路徑都量。
func TestDeletionLocksBuyerAndSellerBeforeReviewRows(t *testing.T) {
	for name, open := range map[string]bool{"申請進入等待": true, "直接關閉": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setupWith(t, migrate.Module{Name: "review", Dir: review.Migrations()})
			rm := review.New(review.Deps{
				DB: f.db, Events: f.bus, Media: media.NewFake(), Jobs: f.queue, Buyers: f.buyer, Stores: seller.NewReader(f.db, time.Now),
				Orders: noReviewOrders{}, Catalog: noopReviewCatalog{}, NewID: uuid.NewV7, Now: time.Now,
			})
			events.Subscribe(f.bus, rm.OnBuyerDeletionRequested)
			events.Subscribe(f.bus, rm.OnBuyerDeleted)
			id, buyerID := f.account(t)
			s := f.store(t, id)
			if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO review.reviews (id, order_no, order_item_id, buyer_id, store_id, product_id, rating, sku_name, price, created_at)
				VALUES ($1, 'N1', 1, $2, $3, $4, 5, 'sku', 1, now())`, uuid.NewV7(), buyerID, s.StoreID, uuid.NewV7()); err != nil {
				t.Fatal(err)
			}
			f.orders.mu.Lock()
			f.orders.open = open
			f.orders.mu.Unlock()

			reviewTouched := func() bool {
				var n int64
				err := f.db.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM pg_locks l JOIN pg_class c ON c.oid = l.relation
					WHERE c.relnamespace = 'review'::regnamespace AND l.mode IN ('RowExclusiveLock', 'RowShareLock') AND l.granted
					AND l.database = (SELECT oid FROM pg_database WHERE datname = current_database())`).Scan(&n)
				if err != nil {
					t.Fatal(err)
				}
				return n > 0
			}
			f.requestHeldOnStoreRow(t, id, buyerID, s.StoreID, "review 的表", reviewTouched)
			if n := f.queue.count("review.purge_author"); n != 1 {
				t.Fatalf("review 的清除 job %d 個（每條刪帳路徑只發一個買家事件，排一次）", n)
			}
			var n int64
			if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM review.reviews WHERE buyer_id = $1", buyerID).Scan(&n); err != nil || n != 1 {
				t.Fatalf("訂閱者不刪評價列：%d %v", n, err)
			}
		})
	}
}

type imStores struct {
	*seller.Reader
	noStoreBlocks
	owner *seller.Module
}

type noImOrders struct{}

func (noImOrders) OpenOrderBetween(context.Context, int64, int64) (bool, error) { return false, nil }

// LockNotRetired 走真正的 seller 模組：退場的事實與守門鎖只在 seller。
func (s imStores) LockNotRetired(ctx context.Context, id int64) error {
	return s.owner.LockNotRetired(ctx, id)
}

func newIM(f *fixture) *im.Module {
	return im.New(im.Deps{
		DB: f.db, Events: f.bus, Media: media.NewFake(), Jobs: f.queue, Buyers: f.buyer, Stores: imStores{Reader: seller.NewReader(f.db, time.Now), owner: f.seller},
		Orders: noImOrders{}, NewID: uuid.NewV7, Now: time.Now,
	})
}

// im 的訂閱者在買家事件的 transaction 內只排抹除 job（不寫複本），所以同樣要排在 buyer、seller 之後：帳號刪除停在店家列上時
// im 的表還沒被碰（pg_locks 沒有 im 的 RowExclusiveLock），放開後 job 才排入。買家寄出的訊息列只在抹除 job 的 transaction 內鎖，不在刪帳的
// transaction 內（訂閱者不碰訊息列）。兩條路徑都量；im 只訂 BuyerDeleted，所以申請進入等待時 im 什麼都不寫。
func TestDeletionLocksBuyerAndSellerBeforeImRows(t *testing.T) {
	for name, open := range map[string]bool{"申請進入等待": true, "直接關閉": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setupWith(t, migrate.Module{Name: "im", Dir: im.Migrations()})
			m := newIM(f)
			events.Subscribe(f.bus, m.OnBuyerDeleted)
			id, buyerID := f.account(t)
			s := f.store(t, id)
			convID := uuid.NewV7()
			if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO im.conversations (id, buyer_id, store_id, last_seq, buyer_last_read_seq, last_message_at, created_at)
				VALUES ($1, $2, $3, 1, 1, now(), now())`, convID, buyerID, s.StoreID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO im.messages (id, conversation_id, seq, sender_kind, message_type, content, created_at, request_id)
				VALUES ($1, $2, 1, 'BUYER', 'TEXT', 'hi', now(), gen_random_uuid())`, uuid.NewV7(), convID); err != nil {
				t.Fatal(err)
			}
			f.orders.mu.Lock()
			f.orders.open = open
			f.orders.mu.Unlock()

			imTouched := func() bool {
				var n int64
				err := f.db.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM pg_locks l JOIN pg_class c ON c.oid = l.relation
					WHERE c.relnamespace = 'im'::regnamespace AND l.mode IN ('RowExclusiveLock', 'RowShareLock') AND l.granted
					AND l.database = (SELECT oid FROM pg_database WHERE datname = current_database())`).Scan(&n)
				if err != nil {
					t.Fatal(err)
				}
				return n > 0
			}
			f.requestHeldOnStoreRow(t, id, buyerID, s.StoreID, "im 的表", imTouched)
			var n int64
			if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM im.messages WHERE erased_at IS NULL").Scan(&n); err != nil || n != 1 {
				t.Fatalf("訂閱者不抹訊息列：%d %v", n, err)
			}
			if want := map[bool]int{true: 0, false: 1}[open]; f.queue.count("im.erase_party") != want {
				t.Fatalf("im 的抹除 job %d 個，預期 %d（只訂 BuyerDeleted：申請進入等待時不排，關閉時才排）", f.queue.count("im.erase_party"), want)
			}
		})
	}
}

// 店家退場：StoreRetired 在 seller purge 的 transaction 內發布，那個 transaction 先鎖店家列；im 的訂閱者之後才排抹除 job，
// 所以順序是 seller → im。鎖順序檢查器（pg.NewDB）會在倒退時讓測試失敗。
func TestStoreRetiredLocksSellerBeforeImRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupWith(t, migrate.Module{Name: "im", Dir: im.Migrations()})
	m := newIM(f)
	events.Subscribe(f.bus, m.OnStoreRetired)
	id, _ := f.account(t)
	s := f.store(t, id)
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		var n int64
		if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 FOR UPDATE", s.StoreID).Scan(&n); err != nil {
			return err
		}
		return events.Publish(ctx, f.bus, seller.StoreRetired{EventID: uuid.NewV7(), OccurredAt: time.Now(), StoreID: s.StoreID, StoreCode: s.StoreCode, AccountID: id})
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.queue.count("im.erase_party"); n != 1 {
		t.Fatalf("im 的抹除 job %d 個", n)
	}
}

func newCRM(f *fixture) *crm.Module {
	return crm.New(crm.Deps{DB: f.db, Jobs: f.queue, Buyers: f.buyer, Stores: f.seller, NewID: uuid.NewV7, Now: time.Now})
}

// crm 的訂閱者在買家事件的 transaction 內只排清除 job（不寫複本），所以同樣要排在 buyer、seller 之後：帳號刪除停在店家列上時
// crm 的表還沒被碰（pg_locks 沒有 crm 的 RowExclusiveLock），放開後 job 才排入。客戶、單與 ledger 只在清除 job 的 transaction 內刪，不在刪帳的
// transaction 內（訂閱者不碰那些列）。只訂 BuyerDeleted，所以申請進入等待時 crm 什麼都不寫。兩條路徑都量。
func TestDeletionLocksBuyerAndSellerBeforeCrmRows(t *testing.T) {
	for name, open := range map[string]bool{"申請進入等待": true, "直接關閉": false} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			f := setupWith(t, migrate.Module{Name: "crm", Dir: crm.Migrations()})
			m := newCRM(f)
			events.Subscribe(f.bus, m.OnBuyerDeleted)
			id, buyerID := f.account(t)
			s := f.store(t, id)
			if _, err := f.db.Conn(ctx).Exec(ctx, `INSERT INTO crm.customers (store_id, buyer_id, total_spent, order_count, first_order_at, updated_at)
				VALUES ($1, $2, 100, 1, now(), now())`, s.StoreID, buyerID); err != nil {
				t.Fatal(err)
			}
			f.orders.mu.Lock()
			f.orders.open = open
			f.orders.mu.Unlock()

			crmTouched := func() bool {
				var n int64
				err := f.db.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM pg_locks l JOIN pg_class c ON c.oid = l.relation
					WHERE c.relnamespace = 'crm'::regnamespace AND l.mode IN ('RowExclusiveLock', 'RowShareLock') AND l.granted
					AND l.database = (SELECT oid FROM pg_database WHERE datname = current_database())`).Scan(&n)
				if err != nil {
					t.Fatal(err)
				}
				return n > 0
			}
			f.requestHeldOnStoreRow(t, id, buyerID, s.StoreID, "crm 的表", crmTouched)
			var n int64
			if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM crm.customers WHERE buyer_id = $1", buyerID).Scan(&n); err != nil || n != 1 {
				t.Fatalf("訂閱者不刪客戶列：%d %v", n, err)
			}
			if want := map[bool]int{true: 0, false: 1}[open]; f.queue.count("crm.erase_party") != want {
				t.Fatalf("crm 的清除 job %d 個，預期 %d（只訂 BuyerDeleted：申請進入等待時不排，關閉時才排）", f.queue.count("crm.erase_party"), want)
			}
		})
	}
}

// 店家退場：StoreRetired 在 seller purge 的 transaction 內發布，那個 transaction 先鎖店家列；crm 的訂閱者之後才排清除 job，
// 所以順序是 seller → crm。鎖順序檢查器（pg.NewDB）會在倒退時讓測試失敗。
func TestStoreRetiredLocksSellerBeforeCrmRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setupWith(t, migrate.Module{Name: "crm", Dir: crm.Migrations()})
	m := newCRM(f)
	events.Subscribe(f.bus, m.OnStoreRetired)
	id, _ := f.account(t)
	s := f.store(t, id)
	err := f.db.InTx(ctx, func(ctx context.Context) error {
		var n int64
		if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 FOR UPDATE", s.StoreID).Scan(&n); err != nil {
			return err
		}
		return events.Publish(ctx, f.bus, seller.StoreRetired{EventID: uuid.NewV7(), OccurredAt: time.Now(), StoreID: s.StoreID, StoreCode: s.StoreCode, AccountID: id})
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.queue.count("crm.erase_party"); n != 1 {
		t.Fatalf("crm 的清除 job %d 個", n)
	}
}

// requestHeldOnStoreRow 讓帳號刪除停在店家列上（另一個 transaction 先鎖住它），此時量兩件事：買家列已鎖、上層模組的列還沒碰
// （touched 回報上層模組是否已經動到自己的資料），再放開店家列並要求帳號刪除完成。
func (f *fixture) requestHeldOnStoreRow(t *testing.T, accountID, buyerID, storeID int64, upper string, touched func() bool) {
	t.Helper()
	ctx := context.Background()
	// 另一個 transaction 先鎖住店家列，讓帳號刪除停在店家列上。
	held, release, holdDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holdDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			var n int64
			if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT store_id FROM seller.stores WHERE store_id = $1 FOR UPDATE", storeID).Scan(&n); err != nil {
				return err
			}
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	reqDone := make(chan error, 1)
	go func() { _, err := f.module.Request(ctx, adminRequest(accountID)); reqDone <- err }()
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
	upperTouched := touched()
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
	if upperTouched {
		problems = append(problems, "停在店家列上的時候 "+upper+"已經被動到（規則 4：上層模組在 seller 之後）")
	}
	if len(problems) > 0 {
		t.Fatal(strings.Join(problems, "；"))
	}
}

// 已刪除帳號沒有帳號列可鎖（LockLiveAccount 對已刪除帳號不鎖），並行完成同一筆待辦申請時，CompleteRequest 的 UPDATE 會等到對方 commit，
// 重新判斷後 0 列。0 列要當成「已完成」回傳該筆申請（不是帳號最新的一筆；後者見下一個測試），不能把 pgx.ErrNoRows 傳出去。拿掉 completeRequest 的 ErrNoRows 處理，這個測試失敗。
func TestCompletePendingOnDeletedAccountToleratesConcurrentCompletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	id, _ := f.account(t)
	f.orders.mu.Lock()
	f.orders.open = true
	f.orders.mu.Unlock()
	if state, err := f.module.Request(ctx, adminRequest(id)); err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	// 帳號被另一條路徑刪除了，待辦申請還在 PENDING_ORDERS。
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE identity.accounts SET deleted_at = now(), deletion_pending_at = NULL, email = NULL, suspended_at = NULL WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}

	// 另一個 transaction 先鎖住待辦申請列；CompletePending 的一般讀取仍讀得到它，之後的 UPDATE 停在這一列上。
	held, release, holdDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holdDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			var rid uuid.UUID
			if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT id FROM accountdeletion.deletion_requests WHERE account_id = $1 AND status = 'PENDING_ORDERS' FOR UPDATE", id).Scan(&rid); err != nil {
				return err
			}
			close(held)
			<-release
			_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE accountdeletion.deletion_requests SET status = 'COMPLETED', completed_at = now() WHERE id = $1", rid)
			return err
		})
	}()
	<-held
	type result struct {
		state State
		err   error
	}
	done := make(chan result, 1)
	go func() { s, err := f.module.CompletePending(ctx, id); done <- result{s, err} }()
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
		case r := <-done:
			t.Fatalf("CompletePending 沒有停在申請列上就結束了：%s %v", r.state, r.err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("CompletePending 沒有卡在鎖上")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if err := <-holdDone; err != nil {
		t.Fatal(err)
	}
	r := <-done
	if r.err != nil || r.state != Completed {
		t.Fatalf("對方已完成同一筆申請：state=%s err=%v，預期 Completed 且沒有錯誤", r.state, r.err)
	}
	if f.count(t, id) != 1 {
		t.Fatal("申請列數量不是 1")
	}
}

// 並行完成同一筆待辦申請之後，Request 要把這次的 attempt 記在「它要完成的那一筆」上，不是帳號最新的一筆：
// 帳號另有一筆更晚的已完成申請時，completeRequest 的 0 列路徑若回傳 LatestRequest，attempt 會掛到別的申請上。
func TestRequestOnDeletedAccountRecordsAttemptOnTheConcurrentlyCompletedRequest(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	id, _ := f.account(t)
	f.orders.mu.Lock()
	f.orders.open = true
	f.orders.mu.Unlock()
	if state, err := f.module.Request(ctx, adminRequest(id)); err != nil || state != PendingOrders {
		t.Fatalf("state=%s err=%v", state, err)
	}
	var pendingID uuid.UUID
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT id FROM accountdeletion.deletion_requests WHERE account_id = $1 AND status = 'PENDING_ORDERS'", id).Scan(&pendingID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Conn(ctx).Exec(ctx, "UPDATE identity.accounts SET deleted_at = now(), deletion_pending_at = NULL, email = NULL, suspended_at = NULL WHERE id = $1", id); err != nil {
		t.Fatal(err)
	}
	// 一筆更晚的已完成申請：LatestRequest 會回它，不是待辦的那一筆。
	if _, err := f.db.Conn(ctx).Exec(ctx, "INSERT INTO accountdeletion.deletion_requests (id, account_id, status, requested_at, completed_at) VALUES ($1, $2, 'COMPLETED', now() + interval '1 hour', now() + interval '1 hour')", uuid.NewV7(), id); err != nil {
		t.Fatal(err)
	}

	held, release, holdDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		holdDone <- f.db.InTx(ctx, func(ctx context.Context) error {
			if _, err := f.db.Conn(ctx).Exec(ctx, "SELECT 1 FROM accountdeletion.deletion_requests WHERE id = $1 FOR UPDATE", pendingID); err != nil {
				return err
			}
			close(held)
			<-release
			_, err := f.db.Conn(ctx).Exec(ctx, "UPDATE accountdeletion.deletion_requests SET status = 'COMPLETED', completed_at = now() WHERE id = $1", pendingID)
			return err
		})
	}()
	<-held
	done := make(chan error, 1)
	go func() { _, err := f.module.Request(ctx, adminRequest(id)); done <- err }()
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
		case err := <-done:
			t.Fatalf("Request 沒有停在申請列上就結束了：%v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Request 沒有卡在鎖上")
		}
		time.Sleep(5 * time.Millisecond)
	}
	close(release)
	if err := <-holdDone; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Request：%v", err)
	}
	var n int64
	if err := f.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM accountdeletion.request_attempts WHERE request_id = $1", pendingID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	// 第一次 Request 的 attempt 也在這一筆上，所以是 2；掛到最新那一筆的話只有 1。
	if n != 2 {
		t.Fatalf("待辦申請上的 attempt = %d，預期 2（第一次與並行完成後的這一次）", n)
	}
}
