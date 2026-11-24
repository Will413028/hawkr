package stream

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Will413028/hawkr/backend/internal/modules/notification/internal/model"
)

func TestDecodeRejectsMalformedPayloads(t *testing.T) {
	good := payload{V: payloadVersion, Kind: kindInbox, Account: 1, Audience: "BUYER"}
	if _, err := decode(good.encode()); err != nil {
		t.Fatalf("合法的通知被拒絕：%v", err)
	}
	for name, p := range map[string]payload{
		"版本不認得":     {V: 9, Kind: kindInbox, Account: 1, Audience: "BUYER"},
		"種類不認得":     {V: payloadVersion, Kind: "x"},
		"沒有帳號":      {V: payloadVersion, Kind: kindInbox, Audience: "BUYER"},
		"BUYER 帶了店": {V: payloadVersion, Kind: kindInbox, Account: 1, Audience: "BUYER", Store: 3},
		"STORE 沒有店": {V: payloadVersion, Kind: kindInbox, Account: 1, Audience: "STORE"},
		"受眾不認得":     {V: payloadVersion, Kind: kindInbox, Account: 1, Audience: "X"},
		"提示不認得":     {V: payloadVersion, Kind: kindInbox, Account: 1, Audience: "BUYER", Hint: "NOPE"},
		"工作區沒有店":    {V: payloadVersion, Kind: kindWorkspace, Change: int(SellerOrder)},
		"工作區變更不認得":  {V: payloadVersion, Kind: kindWorkspace, Store: 7, Change: 99},
	} {
		if _, err := decode(p.encode()); !errors.Is(err, errPayload) {
			t.Errorf("%s：應回 errPayload：%v", name, err)
		}
	}
	if _, err := decode("not json"); !errors.Is(err, errPayload) {
		t.Errorf("非 JSON 應回 errPayload：%v", err)
	}
}

func TestNotifyRejectsInvalidInputBeforeTouchingTheDatabase(t *testing.T) {
	ctx := context.Background()
	if err := NotifyWorkspace(ctx, nil, 0, SellerOrder); err == nil {
		t.Fatal("店 0 應被拒絕")
	}
	if err := NotifyWorkspace(ctx, nil, 7, 0); err == nil {
		t.Fatal("未指定的變更應被拒絕")
	}
	if err := NotifyInbox(ctx, nil, storeKey(1), 0, model.StateHintNone); err == nil {
		t.Fatal("STORE 沒有店應被拒絕")
	}
}

func TestDispatchRoutesByAccountAudienceAndStore(t *testing.T) {
	h := newHub(fakeRead, 8)
	b := subscribe(t, h, buyer(1), 0)
	st := subscribe(t, h, storeKey(1), 7)
	other := subscribe(t, h, storeKey(2), 8)
	for _, s := range []*Subscription{b, st, other} {
		next(t, s)
	}
	h.dispatch(payload{V: payloadVersion, Kind: kindInbox, Account: 1, Audience: "BUYER", Hint: "ORDER"}.encode())
	if ev := next(t, b); ev.Inbox == nil || !slices.Equal(ev.Inbox.Hints, []model.StateHint{model.StateHintOrder}) {
		t.Fatalf("買家訂閱者應收到帶提示的訊號：%+v", ev)
	}
	wantNone(t, st)
	wantNone(t, other)
	h.dispatch(payload{V: payloadVersion, Kind: kindWorkspace, Store: 7, Change: int(SellerAfterSale)}.encode())
	if ev := next(t, st); ev.Workspace != SellerAfterSale {
		t.Fatalf("店 7 應收到售後變更：%+v", ev)
	}
	wantNone(t, other)
	wantNone(t, b)
	h.dispatch("garbage")
	wantNone(t, b)
}

type fakeConn struct {
	msgs    chan string
	failNow chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (c *fakeConn) Listen(context.Context) error { return nil }
func (c *fakeConn) Wait(ctx context.Context) (string, error) {
	select {
	case m := <-c.msgs:
		return m, nil
	case <-c.failNow:
		return "", errors.New("connection reset")
	case <-ctx.Done():
		return "", ctx.Err()
	}
}
func (c *fakeConn) Close(context.Context) { c.once.Do(func() { close(c.closed) }) }

func TestRunDeliversNotificationsAndResyncsOnReconnect(t *testing.T) {
	h := newHub(fakeRead, 8)
	h.now = time.Now
	h.backoffBase, h.backoffMax = time.Millisecond, 2*time.Millisecond
	conns := make(chan *fakeConn, 2)
	dial := func(context.Context) (Conn, error) {
		c := &fakeConn{msgs: make(chan string, 4), failNow: make(chan struct{}), closed: make(chan struct{})}
		conns <- c
		return c, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- h.Run(ctx, dial) }()
	first := <-conns
	select {
	case <-h.Listening():
	case <-time.After(2 * time.Second):
		t.Fatal("LISTEN 沒有建立")
	}
	st := subscribe(t, h, storeKey(1), 7)
	next(t, st)

	first.msgs <- payload{V: payloadVersion, Kind: kindInbox, Account: 1, Audience: "STORE", Store: 7}.encode()
	if ev := next(t, st); ev.Inbox == nil {
		t.Fatalf("LISTEN 收到的通知應扇出：%+v", ev)
	}
	wantNone(t, st)

	close(first.failNow)
	<-conns
	if ev := next(t, st); ev.Inbox == nil {
		t.Fatalf("重連後應先收到重讀收件匣的訊號：%+v", ev)
	}
	got := []SellerChange{next(t, st).Workspace, next(t, st).Workspace}
	if !slices.Equal(got, []SellerChange{SellerOrder, SellerAfterSale}) {
		t.Fatalf("重連後店家訂閱者應收到全部賣家工作區變更：%v", got)
	}
	wantNone(t, st)

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("ctx 結束 Run 應回 nil：%v", err)
	}
	if _, err := st.Next(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Run 結束後開著的訂閱應以 ErrClosed 結束：%v", err)
	}
	if _, err := h.Subscribe(buyer(1), 0); !errors.Is(err, ErrClosed) {
		t.Fatalf("Run 結束後不再接受訂閱：%v", err)
	}
}

// runFlapping 讓每條連線連上後經過 uptime（假時鐘）就斷線，dials 次後取消 ctx，回傳每次重連前的等待。
func runFlapping(t *testing.T, uptime time.Duration, dials int) []time.Duration {
	t.Helper()
	h := newHub(fakeRead, 8)
	h.backoffBase, h.backoffMax = time.Second, 8*time.Second
	h.jitter = func() float64 { return 1 }
	var clock time.Time
	h.now = func() time.Time { return clock }
	var sleeps []time.Duration
	h.sleep = func(_ context.Context, d time.Duration) { sleeps = append(sleeps, d) }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := 0
	dial := func(context.Context) (Conn, error) {
		n++
		if n > dials {
			cancel()
			return nil, ctx.Err()
		}
		return &flapConn{advance: func() { clock = clock.Add(uptime) }}, nil
	}
	if err := h.Run(ctx, dial); err != nil {
		t.Fatal(err)
	}
	return sleeps
}

type flapConn struct{ advance func() }

func (c *flapConn) Listen(context.Context) error { return nil }
func (c *flapConn) Wait(context.Context) (string, error) {
	c.advance()
	return "", errors.New("connection reset")
}
func (c *flapConn) Close(context.Context) {}

// 連上就斷的連線退避遞增到上限，不會每次都從頭等（否則每次重連的重讀訊號約每秒數次）。
// mutation：Run 連線成功就重設退避 → 第二次起的等待不遞增，失敗。
func TestRunBackoffKeepsGrowingWhileTheConnectionFlaps(t *testing.T) {
	got := runFlapping(t, time.Millisecond, 5)
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 8 * time.Second}
	if !slices.Equal(got, want) {
		t.Fatalf("等待 = %v，預期 %v", got, want)
	}
}

// 連線穩定維持夠久之後斷線，退避回到起點。
// mutation：stableAfter 檢查拿掉 → 穩定的連線也不重設，等待遞增，失敗。
func TestRunBackoffResetsAfterAStableConnection(t *testing.T) {
	got := runFlapping(t, stableAfter, 4)
	want := []time.Duration{time.Second, time.Second, time.Second, time.Second}
	if !slices.Equal(got, want) {
		t.Fatalf("等待 = %v，預期 %v", got, want)
	}
}

// 時鐘與等待都不注入（nil）時 Run 不會 panic（連線一律不算穩定）。
// mutation：Run 不在 now 為 nil 時給預設時鐘 → 第一次連線後呼叫 nil 函式而 panic。
func TestRunWithoutInjectedClockDoesNotPanic(t *testing.T) {
	h := NewHub(fakeRead, nil, nil)
	h.backoffBase, h.backoffMax = time.Millisecond, time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := 0
	dial := func(context.Context) (Conn, error) {
		n++
		if n > 2 {
			cancel()
			return nil, ctx.Err()
		}
		return &flapConn{advance: func() {}}, nil
	}
	if err := h.Run(ctx, dial); err != nil {
		t.Fatal(err)
	}
	if n < 3 {
		t.Fatalf("應重連到第 3 次撥號：%d", n)
	}
}
