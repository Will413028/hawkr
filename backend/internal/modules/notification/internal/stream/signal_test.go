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
