//go:build integration

package notification

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/stream"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// listen 啟動 m 的 LISTEN 連線（serve 角色的 Run）並等它建立；測試結束時停止。
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
	serve := newModule(Deps{DB: f.db, Identity: f.identity, Buyer: f.buyer, Seller: f.stores, Jobs: f.jobs, Now: time.Now, NewID: uuid.NewV7}, testRegistry{})
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
		`SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity WHERE query = 'LISTEN '||$1 AND pid <> pg_backend_pid()`, stream.Channel).Scan(&n); err != nil || n != 1 {
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
