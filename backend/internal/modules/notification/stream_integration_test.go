//go:build integration

package notification

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/app"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/stream"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func listen(t *testing.T, m *Module) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- m.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Run：%v", err)
		}
	})
	select {
	case <-m.hub.Listening():
	case <-time.After(5 * time.Second):
		t.Fatal("LISTEN 連線沒有建立")
	}
}

func watch(t *testing.T, f *fixture, p WatchParams) Watcher {
	t.Helper()
	s, err := f.module.Watch(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func nextEvent(t *testing.T, s Watcher) StreamEvent {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ev, err := s.Next(ctx)
	if err != nil {
		t.Fatalf("Next：%v", err)
	}
	return ev
}

func noEvent(t *testing.T, s Watcher) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if ev, err := s.Next(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("預期沒有訊號，實際：%+v %v", ev, err)
	}
}

func TestInboxWriteSignalsTheRecipientsStreamOnly(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	buyerSub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	storeSub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceStore, StoreID: 7})
	otherSub := watch(t, f, WatchParams{AccountID: other, Audience: AudienceBuyer})
	for _, s := range []Watcher{buyerSub, storeSub, otherSub} {
		if ev := nextEvent(t, s); ev.Inbox == nil || ev.Inbox.UnreadCount != 0 || ev.Inbox.LatestItemID != nil || len(ev.Inbox.Hints) != 0 {
			t.Fatalf("連線建立的第一則應是空收件匣、無提示的 inbox 訊號：%+v", ev)
		}
	}

	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "e1", PostID: "p", ActorID: other})
	f.run(t)
	ev := nextEvent(t, buyerSub)
	var latest uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.inbox_items WHERE account_id = $1`, acct).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	if ev.Inbox == nil || ev.Inbox.UnreadCount != 1 || ev.Inbox.LatestItemID == nil || *ev.Inbox.LatestItemID != latest {
		t.Fatalf("買家串流應收到未讀 1 與最新的收件匣 ID %s：%+v", latest, ev)
	}
	if !slices.Equal(ev.Inbox.NewItemIDs, []uuid.UUID{latest}) || ev.Inbox.NewItemsTruncated {
		t.Fatalf("新建的通知 ID 應在訊號裡：%+v", ev.Inbox)
	}
	noEvent(t, storeSub)
	noEvent(t, otherSub)

	f.accept(t, "test_store_note", storeNoteParams{AccountID: acct, StoreID: 7, EventID: "s1"})
	f.run(t)
	if ev := nextEvent(t, storeSub); ev.Inbox == nil || ev.Inbox.UnreadCount != 1 {
		t.Fatalf("店家串流應收到店的未讀 1：%+v", ev)
	}
	noEvent(t, buyerSub)
}

func TestPushOnlyIntentDoesNotSignalTheStream(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	storeSub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceStore, StoreID: 7})
	nextEvent(t, storeSub)
	f.accept(t, "test_store_push_only", storeNoteParams{AccountID: acct, StoreID: 7, EventID: "p1"})
	f.run(t)
	noEvent(t, storeSub)
}

func TestStreamCapPerAccountIsEnforcedByWatch(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	for range MaxStreamsPerAccount {
		watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	}
	if _, err := f.module.Watch(WatchParams{AccountID: acct, Audience: AudienceStore, StoreID: 7}); apperr.KindOf(err) != apperr.KindResourceExhausted {
		t.Fatalf("超過上限應回 resource_exhausted：%v", err)
	}
}

func TestSellerWorkspaceChangeReachesTheStoresStreamOnly(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	mine := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceStore, StoreID: 7})
	otherStore := watch(t, f, WatchParams{AccountID: other, Audience: AudienceStore, StoreID: 8})
	buyerSide := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	for _, s := range []Watcher{mine, otherStore, buyerSide} {
		nextEvent(t, s)
	}
	if err := f.module.PublishSellerWorkspace(context.Background(), 7, SellerOrderChanged); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, mine); ev.Workspace != SellerOrderChanged || ev.Inbox != nil {
		t.Fatalf("店 7 的串流應收到訂單變更：%+v", ev)
	}
	noEvent(t, otherStore)
	noEvent(t, buyerSide)
}

// worker 與 serve 是不同行程：寫入收件匣的 module（worker）自己沒有 LISTEN，訂閱者在另一個 module（另一個 serve 行程，自己的 hub）上，訊號只能經資料庫到達。
func TestInboxSignalCrossesTheProcessBoundaryThroughTheDatabase(t *testing.T) {
	f := setupIntake(t)
	serve := newModule(Deps{DB: f.db, Identity: f.identity, Buyer: f.buyer, Seller: f.stores, Validity: noVouchers{}, Jobs: f.jobs, Now: time.Now, NewID: uuid.NewV7}, testRegistry{})
	listen(t, serve)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	sub, err := serve.Watch(WatchParams{AccountID: acct, Audience: AudienceBuyer})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.Close)
	nextEvent(t, sub)

	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "x1", PostID: "p", ActorID: other})
	f.run(t)
	if ev := nextEvent(t, sub); ev.Inbox == nil || ev.Inbox.UnreadCount != 1 || ev.Inbox.LatestItemID == nil {
		t.Fatalf("另一個行程的訂閱者應收到 worker 寫入的未讀 1：%+v", ev)
	}
}

func TestSellerWorkspaceNotifyFollowsTheCallersTransaction(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	sub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceStore, StoreID: 7})
	nextEvent(t, sub)

	rollback := errors.New("rollback")
	err := f.db.InTx(context.Background(), func(ctx context.Context) error {
		if err := f.module.PublishSellerWorkspace(ctx, 7, SellerOrderChanged); err != nil {
			return err
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
	noEvent(t, sub)

	err = f.db.InTx(context.Background(), func(ctx context.Context) error {
		if err := f.module.PublishSellerWorkspace(ctx, 7, SellerAfterSaleChanged); err != nil {
			return err
		}
		noEvent(t, sub)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, sub); ev.Workspace != SellerAfterSaleChanged {
		t.Fatalf("commit 之後應收到售後變更：%+v", ev)
	}
}

func TestListenReconnectEmitsAFullRereadSignal(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	sub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceStore, StoreID: 7})
	nextEvent(t, sub)

	var n int
	if err := f.db.Pool().QueryRow(context.Background(),
		`SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity WHERE query = 'LISTEN '||$1 AND datname = current_database() AND pid <> pg_backend_pid()`, stream.Channel).Scan(&n); err != nil || n != 1 {
		t.Fatalf("應剛好終止一條 LISTEN 連線：%d %v", n, err)
	}
	if ev := nextEvent(t, sub); ev.Inbox == nil {
		t.Fatalf("重連後應先收到重讀收件匣的訊號：%+v", ev)
	}
	got := []SellerWorkspaceChange{nextEvent(t, sub).Workspace, nextEvent(t, sub).Workspace}
	if !slices.Equal(got, []SellerWorkspaceChange{SellerOrderChanged, SellerAfterSaleChanged}) {
		t.Fatalf("重連後店家訂閱者應收到全部賣家工作區變更：%v", got)
	}
	if err := f.module.PublishSellerWorkspace(context.Background(), 7, SellerOrderChanged); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, sub); ev.Workspace != SellerOrderChanged {
		t.Fatalf("重連後的通知仍應送達：%+v", ev)
	}
}

// 串流的未讀數與最新一則和 UnreadCount 同一條規則：沒有註冊的型別（已退場）不計數、不當最新一則。
// mutation：inboxReader 改回不帶型別過濾 → 未讀數多算退場型別的列，失敗。
func TestStreamSignalCountsOnlyRegisteredTypesLikeUnreadCount(t *testing.T) {
	f := setupIntake(t)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "e1", PostID: "p", ActorID: other})
	f.run(t)
	var registered uuid.UUID
	if err := f.db.Pool().QueryRow(context.Background(), `SELECT id FROM notification.inbox_items WHERE account_id = $1`, acct).Scan(&registered); err != nil {
		t.Fatal(err)
	}
	mustExec(t, f.db, `INSERT INTO notification.inbox_items (id, account_id, audience, type, params, occurred_at, created_at, updated_at) VALUES ($1, $2, 'BUYER', 'retired_type', '{}', now(), now(), now() + interval '1 minute')`, uuid.New(), acct)

	sub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	ev := nextEvent(t, sub)
	unread, err := f.module.UnreadCount(context.Background(), Recipient{AccountID: acct, Audience: AudienceBuyer})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Inbox == nil || int64(ev.Inbox.UnreadCount) != unread || unread != 1 || ev.Inbox.LatestItemID == nil || *ev.Inbox.LatestItemID != registered {
		t.Fatalf("訊號應與 UnreadCount（%d）一致，只算註冊的型別，最新一則是 %s：%+v", unread, registered, ev.Inbox)
	}
}

// 已讀、全部已讀、刪除改變未讀數，同一個 transaction 內 pg_notify：同收件人的其他訂閱者收到帶新未讀數的訊號，別的受眾與別的帳號不受影響。
// mutation：app.MarkRead、MarkAllRead、BatchDelete 各自拿掉 inboxChanged 呼叫 → 對應的 subtest 收不到訊號，失敗。
func TestInboxReadAndDeleteSignalTheRecipientsOtherSubscribers(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	for _, id := range []string{"e1", "e2", "e3"} {
		f.accept(t, "test_like", likeParams{AccountID: acct, EventID: id, PostID: "p" + id, ActorID: other})
	}
	f.accept(t, "test_store_note", storeNoteParams{AccountID: acct, StoreID: 7, EventID: "s1"})
	f.run(t)
	var ids []uuid.UUID
	rows, err := f.db.Pool().Query(context.Background(), `SELECT id FROM notification.inbox_items WHERE account_id = $1 AND audience = 'BUYER' ORDER BY id`, acct)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) != 3 {
		t.Fatalf("買家收件匣應有 3 則：%d", len(ids))
	}
	buyerSub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	storeSub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceStore, StoreID: 7})
	otherSub := watch(t, f, WatchParams{AccountID: other, Audience: AudienceBuyer})
	if ev := nextEvent(t, buyerSub); ev.Inbox == nil || ev.Inbox.UnreadCount != 3 {
		t.Fatalf("連線建立時未讀應是 3：%+v", ev)
	}
	nextEvent(t, storeSub)
	nextEvent(t, otherSub)
	ctx := context.Background()
	wantUnread := func(t *testing.T, want int32) {
		t.Helper()
		if ev := nextEvent(t, buyerSub); ev.Inbox == nil || ev.Inbox.UnreadCount != want {
			t.Fatalf("買家串流的未讀數應是 %d：%+v", want, ev)
		}
		noEvent(t, storeSub)
		noEvent(t, otherSub)
	}

	t.Run("mark one read", func(t *testing.T) {
		if err := f.module.MarkInboxItemRead(ctx, acct, ids[0]); err != nil {
			t.Fatal(err)
		}
		wantUnread(t, 2)
	})
	t.Run("batch delete", func(t *testing.T) {
		if err := f.module.BatchDeleteInboxItems(ctx, acct, []uuid.UUID{ids[1]}); err != nil {
			t.Fatal(err)
		}
		wantUnread(t, 1)
	})
	t.Run("mark all read", func(t *testing.T) {
		if _, err := f.module.MarkAllInboxItemsRead(ctx, Recipient{AccountID: acct, Audience: AudienceBuyer}); err != nil {
			t.Fatal(err)
		}
		wantUnread(t, 0)
	})
	t.Run("a rollback after the signal call delivers nothing", func(t *testing.T) {
		failing := app.New(app.Deps{
			DB: f.db, Accounts: f.identity, Registry: testRegistry{}, Now: time.Now,
			Signals: failAfterNotify{dbSignals{f.db}},
		})
		if err := failing.BatchDelete(ctx, acct, []uuid.UUID{ids[2]}); !errors.Is(err, errAfterNotify) {
			t.Fatalf("通知之後的失敗應讓整個 transaction 回滾並回傳：%v", err)
		}
		noEvent(t, buyerSub)
		var n int
		if err := f.db.Pool().QueryRow(ctx, `SELECT count(*) FROM notification.inbox_items WHERE id = $1`, ids[2]).Scan(&n); err != nil || n != 1 {
			t.Fatalf("回滾後通知應還在：%d %v", n, err)
		}
	})
}

var errAfterNotify = errors.New("notify issued, then the transaction fails")

type failAfterNotify struct{ dbSignals }

func (s failAfterNotify) InboxChanged(ctx context.Context, accountID int64, audience model.Audience, storeID int64, hint model.StateHint, itemID uuid.UUID) error {
	if err := s.dbSignals.InboxChanged(ctx, accountID, audience, storeID, hint, itemID); err != nil {
		return err
	}
	return errAfterNotify
}

// 聚合更新（合併進同一則未讀通知）也送出那一列的 ID，且 ID 不變：client 以 ID 去重並更新 toast。
// mutation：intake 的 deliver 不回傳 itemID（或 hub 合併時丟掉 ID）→ 沒有 ID，失敗。
func TestAggregatedBumpRepeatsTheSameItemID(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	a, b := f.newAccount(t), f.newAccount(t)
	sub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	nextEvent(t, sub)

	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "g1", PostID: "p", ActorID: a})
	f.run(t)
	first := nextEvent(t, sub)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "g2", PostID: "p", ActorID: b})
	f.run(t)
	second := nextEvent(t, sub)
	if len(first.Inbox.NewItemIDs) != 1 || !slices.Equal(first.Inbox.NewItemIDs, second.Inbox.NewItemIDs) {
		t.Fatalf("聚合更新應再送同一個 ID：%v 之後 %v", first.Inbox.NewItemIDs, second.Inbox.NewItemIDs)
	}
	if got := f.inboxCount(t, acct); got != 1 {
		t.Fatalf("兩個讚應合併成一列：%d", got)
	}
}

// 已讀、全部已讀、刪除的訊號不帶新通知 ID。
// mutation：app.inboxChanged 傳入非 Nil 的 ID → 已讀訊號帶了 ID，失敗。
func TestReadAndDeleteSignalsCarryNoItemIDs(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "r1", PostID: "p1", ActorID: other})
	f.accept(t, "test_like", likeParams{AccountID: acct, EventID: "r2", PostID: "p2", ActorID: other})
	f.run(t)
	var ids []uuid.UUID
	rows, err := f.db.Pool().Query(context.Background(), `SELECT id FROM notification.inbox_items WHERE account_id = $1 ORDER BY id`, acct)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	sub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	nextEvent(t, sub)
	ctx := context.Background()
	if err := f.module.MarkInboxItemRead(ctx, acct, ids[0]); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, sub); ev.Inbox == nil || len(ev.Inbox.NewItemIDs) != 0 || ev.Inbox.NewItemsTruncated {
		t.Fatalf("已讀的訊號不帶 ID：%+v", ev)
	}
	if err := f.module.BatchDeleteInboxItems(ctx, acct, []uuid.UUID{ids[1]}); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, sub); ev.Inbox == nil || len(ev.Inbox.NewItemIDs) != 0 {
		t.Fatalf("刪除的訊號不帶 ID：%+v", ev)
	}
	if _, err := f.module.MarkAllInboxItemsRead(ctx, Recipient{AccountID: acct, Audience: AudienceBuyer}); err != nil {
		t.Fatal(err)
	}
	if ev := nextEvent(t, sub); ev.Inbox == nil || len(ev.Inbox.NewItemIDs) != 0 {
		t.Fatalf("全部已讀的訊號不帶 ID：%+v", ev)
	}
}

// 訂閱者來不及讀時訊號會合併：不論合併與否，每則新通知的 ID 都送達（超過上限的 truncated 由 hub 單元測試鎖，合併時序在整合測試裡不確定）。
func TestEveryNewItemIDIsDeliveredWhetherOrNotSignalsCoalesce(t *testing.T) {
	f := setupIntake(t)
	listen(t, f.module)
	acct := f.newAccount(t)
	other := f.newAccount(t)
	sub := watch(t, f, WatchParams{AccountID: acct, Audience: AudienceBuyer})
	nextEvent(t, sub)

	for i := range 3 {
		f.accept(t, "test_like", likeParams{AccountID: acct, EventID: fmt.Sprintf("c%d", i), PostID: fmt.Sprintf("p%d", i), ActorID: other})
	}
	f.run(t)
	var got []uuid.UUID
	for range 3 {
		ev := nextEvent(t, sub)
		got = append(got, ev.Inbox.NewItemIDs...)
		if len(got) >= 3 {
			break
		}
	}
	slices.SortFunc(got, uuid.UUID.Compare)
	var want []uuid.UUID
	rows, err := f.db.Pool().Query(context.Background(), `SELECT id FROM notification.inbox_items WHERE account_id = $1 ORDER BY id`, acct)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		want = append(want, id)
	}
	rows.Close()
	if !slices.Equal(got, want) {
		t.Fatalf("三則新通知的 ID 都應送達（合併與否都一樣）：%v，預期 %v", got, want)
	}
}
