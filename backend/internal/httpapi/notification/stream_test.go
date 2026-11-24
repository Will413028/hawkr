package notification

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/notification/v1alpha1/notificationv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/notification"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

const (
	account      = int64(7)
	noStoreAcct  = int64(8)
	storeIDOfAcc = int64(77)
)

type stores struct{}

func (stores) ActingStore(_ context.Context, a int64, _ seller.Capability) (seller.Store, error) {
	if a == account {
		return seller.Store{StoreID: storeIDOfAcc}, nil
	}
	return seller.Store{}, apperr.NotFound("沒有店家")
}

// watcher 把測試推進來的事件交給 handler。
type watcher struct {
	events chan notification.StreamEvent
	once   sync.Once
	done   chan struct{}
	fail   chan struct{}
}

func (w *watcher) Next(ctx context.Context) (notification.StreamEvent, error) {
	select {
	case ev := <-w.events:
		return ev, nil
	case <-ctx.Done():
		return notification.StreamEvent{}, ctx.Err()
	case <-w.done:
		return notification.StreamEvent{}, errors.New("closed")
	case <-w.fail:
		return notification.StreamEvent{}, notification.ErrClosed
	}
}

func (w *watcher) Close() { w.once.Do(func() { close(w.done) }) }

// fakeStreams 只實作串流需要的兩個方法，其餘（unary）由內嵌的 nil 介面擋住：串流 handler 不該碰它們。
type fakeStreams struct {
	Notifications
	mu      sync.Mutex
	params  []notification.WatchParams
	err     error
	watcher *watcher
}

func (f *fakeStreams) ValidateAudience(_ context.Context, a notification.Audience) error {
	if a != notification.AudienceBuyer && a != notification.AudienceStore {
		return apperr.InvalidInputOn([]string{"Audience"}, "%s", "audience 必須是 BUYER 或 STORE")
	}
	return nil
}

func (f *fakeStreams) Watch(p notification.WatchParams) (notification.Watcher, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.params = append(f.params, p)
	if f.err != nil {
		return nil, f.err
	}
	return f.watcher, nil
}

func setupStream(t *testing.T, fs *fakeStreams, lifetime time.Duration) notificationv1alpha1connect.NotificationAppServiceClient {
	t.Helper()
	app, err := NewApp(fs, stores{}, rpc.Images{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	app.lifetime = func() time.Duration { return lifetime }
	path, h := notificationv1alpha1connect.NewNotificationAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.Header.Get("X-Test-Account"); v != "" {
			a, _ := strconv.ParseInt(v, 10, 64)
			r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: a}))
		}
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return notificationv1alpha1connect.NewNotificationAppServiceClient(srv.Client(), srv.URL)
}

func watch(t *testing.T, c notificationv1alpha1connect.NotificationAppServiceClient, a int64, aud pb.NotificationAudience) (*connect.ServerStreamForClient[pb.WatchNotificationsResponse], error) {
	t.Helper()
	req := connect.NewRequest(&pb.WatchNotificationsRequest{Audience: aud})
	req.Header().Set("X-Test-Account", strconv.FormatInt(a, 10))
	return c.WatchNotifications(t.Context(), req)
}

func newFake() *fakeStreams {
	return &fakeStreams{watcher: &watcher{events: make(chan notification.StreamEvent, 8), done: make(chan struct{}), fail: make(chan struct{})}}
}

func TestWatchMapsInboxAndSellerWorkspaceSignals(t *testing.T) {
	fs := newFake()
	c := setupStream(t, fs, time.Minute)
	latest := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	fs.watcher.events <- notification.StreamEvent{Inbox: &notification.InboxSignal{UnreadCount: 3, LatestItemID: &latest, Hints: []notification.StateHint{notification.StateHintOrder, notification.StateHintAfterSale}}}
	fs.watcher.events <- notification.StreamEvent{Inbox: &notification.InboxSignal{}}
	fs.watcher.events <- notification.StreamEvent{Workspace: notification.SellerOrderChanged}
	fs.watcher.events <- notification.StreamEvent{Workspace: notification.SellerAfterSaleChanged}
	st, err := watch(t, c, account, pb.NotificationAudience_NOTIFICATION_AUDIENCE_STORE)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	recv := func() *pb.WatchNotificationsResponse {
		t.Helper()
		if !st.Receive() {
			t.Fatalf("串流提早結束：%v", st.Err())
		}
		return st.Msg()
	}
	in := recv().GetInbox()
	if in.GetUnreadCount() != 3 || in.GetLatestInboxItemId() != latest.String() || len(in.GetStateRefreshHints()) != 2 ||
		in.GetStateRefreshHints()[0] != pb.StateRefreshHint_STATE_REFRESH_HINT_ORDER || in.GetStateRefreshHints()[1] != pb.StateRefreshHint_STATE_REFRESH_HINT_AFTER_SALE {
		t.Errorf("inbox 訊號 = %v", in)
	}
	empty := recv().GetInbox()
	if empty == nil || empty.LatestInboxItemId != nil || len(empty.GetStateRefreshHints()) != 0 {
		t.Errorf("空收件匣的訊號不該帶最新 ID 與提示：%v", empty)
	}
	if got := recv().GetSellerWorkspace().GetChange(); got != pb.SellerWorkspaceChange_SELLER_WORKSPACE_CHANGE_ORDER {
		t.Errorf("賣家工作區 = %v", got)
	}
	if got := recv().GetSellerWorkspace().GetChange(); got != pb.SellerWorkspaceChange_SELLER_WORKSPACE_CHANGE_AFTER_SALE {
		t.Errorf("賣家工作區 = %v", got)
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if len(fs.params) != 1 || fs.params[0] != (notification.WatchParams{AccountID: account, Audience: notification.AudienceStore, StoreID: storeIDOfAcc}) {
		t.Errorf("訂閱條件 = %+v", fs.params)
	}
}

func TestWatchMapsNewItemIDsAndTruncation(t *testing.T) {
	fs := newFake()
	c := setupStream(t, fs, time.Minute)
	a := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	b := uuid.MustParse("55555555-5555-4555-8555-555555555555")
	fs.watcher.events <- notification.StreamEvent{Inbox: &notification.InboxSignal{UnreadCount: 2, NewItemIDs: []uuid.UUID{a, b}}}
	fs.watcher.events <- notification.StreamEvent{Inbox: &notification.InboxSignal{UnreadCount: 30, NewItemsTruncated: true}}
	st, err := watch(t, c, noStoreAcct, pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if !st.Receive() {
		t.Fatalf("串流提早結束：%v", st.Err())
	}
	if in := st.Msg().GetInbox(); !slices.Equal(in.GetNewInboxItemIds(), []string{a.String(), b.String()}) || in.GetNewItemsTruncated() {
		t.Errorf("新通知 ID 應依序對映：%v", in)
	}
	if !st.Receive() {
		t.Fatalf("串流提早結束：%v", st.Err())
	}
	if in := st.Msg().GetInbox(); len(in.GetNewInboxItemIds()) != 0 || !in.GetNewItemsTruncated() {
		t.Errorf("truncated 應對映且不帶 ID：%v", in)
	}
}

func TestWatchBuyerAudienceSubscribesWithoutAStore(t *testing.T) {
	fs := newFake()
	c := setupStream(t, fs, time.Minute)
	fs.watcher.events <- notification.StreamEvent{Inbox: &notification.InboxSignal{UnreadCount: 1}}
	st, err := watch(t, c, noStoreAcct, pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if !st.Receive() || st.Msg().GetInbox().GetUnreadCount() != 1 {
		t.Fatalf("買家受眾不需要店家也應收到訊號：%v", st.Err())
	}
	fs.mu.Lock()
	defer fs.mu.Unlock()
	if fs.params[0].StoreID != 0 || fs.params[0].Audience != notification.AudienceBuyer {
		t.Errorf("訂閱條件 = %+v", fs.params[0])
	}
}

func failure(t *testing.T, c notificationv1alpha1connect.NotificationAppServiceClient, a int64, aud pb.NotificationAudience) connect.Code {
	t.Helper()
	st, err := watch(t, c, a, aud)
	if err != nil {
		return connect.CodeOf(err)
	}
	defer func() { _ = st.Close() }()
	if st.Receive() {
		t.Fatalf("不該收到訊號：%v", st.Msg())
	}
	return connect.CodeOf(st.Err())
}

func TestWatchRejectsBadAudienceMissingStoreAndOverCap(t *testing.T) {
	fs := newFake()
	c := setupStream(t, fs, time.Minute)
	if got := failure(t, c, account, pb.NotificationAudience_NOTIFICATION_AUDIENCE_UNSPECIFIED); got != connect.CodeInvalidArgument {
		t.Errorf("未指定受眾 = %v", got)
	}
	if got := failure(t, c, noStoreAcct, pb.NotificationAudience_NOTIFICATION_AUDIENCE_STORE); got != connect.CodeNotFound {
		t.Errorf("STORE 受眾沒有店家 = %v，要 NOT_FOUND", got)
	}
	fs.mu.Lock()
	if len(fs.params) != 0 {
		t.Errorf("沒有店家不該訂閱：%+v", fs.params)
	}
	fs.err = apperr.ResourceExhausted(time.Second, "太多串流")
	fs.mu.Unlock()
	if got := failure(t, c, account, pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER); got != connect.CodeResourceExhausted {
		t.Errorf("超過上限 = %v，要 RESOURCE_EXHAUSTED", got)
	}
}

func TestWatchRejectsNonAccountPrincipals(t *testing.T) {
	fs := newFake()
	app, err := NewApp(fs, stores{}, rpc.Images{}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	path, h := notificationv1alpha1connect.NewNotificationAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAdmin, ID: 1}))
		h.ServeHTTP(w, r)
	}))
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := notificationv1alpha1connect.NewNotificationAppServiceClient(srv.Client(), srv.URL)
	st, err := c.WatchNotifications(t.Context(), connect.NewRequest(&pb.WatchNotificationsRequest{Audience: pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER}))
	if err == nil {
		defer func() { _ = st.Close() }()
		st.Receive()
		err = st.Err()
	}
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("非顧客帳號 = %v，要 PERMISSION_DENIED", err)
	}
}

func TestWatchEndsCleanlyAtTheLifetimeAndReleasesTheSubscription(t *testing.T) {
	fs := newFake()
	c := setupStream(t, fs, 50*time.Millisecond)
	st, err := watch(t, c, account, pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	if st.Receive() {
		t.Fatalf("壽命到期前沒有事件：%v", st.Msg())
	}
	if st.Err() != nil {
		t.Fatalf("壽命到期應正常結束：%v", st.Err())
	}
	select {
	case <-fs.watcher.done:
	case <-time.After(2 * time.Second):
		t.Fatal("串流結束後應關閉訂閱")
	}
}

func TestWatchMapsHubShutdownToUnavailable(t *testing.T) {
	t.Run("on Next", func(t *testing.T) {
		fs := newFake()
		close(fs.watcher.fail)
		c := setupStream(t, fs, time.Minute)
		st, err := watch(t, c, account, pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER)
		if err == nil {
			defer func() { _ = st.Close() }()
			st.Receive()
			err = st.Err()
		}
		if got := connect.CodeOf(err); got != connect.CodeUnavailable {
			t.Fatalf("code = %v，要 unavailable：%v", got, err)
		}
	})
	t.Run("on Watch", func(t *testing.T) {
		fs := newFake()
		fs.err = notification.ErrClosed
		c := setupStream(t, fs, time.Minute)
		st, err := watch(t, c, account, pb.NotificationAudience_NOTIFICATION_AUDIENCE_BUYER)
		if err == nil {
			defer func() { _ = st.Close() }()
			st.Receive()
			err = st.Err()
		}
		if got := connect.CodeOf(err); got != connect.CodeUnavailable {
			t.Fatalf("code = %v，要 unavailable：%v", got, err)
		}
	})
}
