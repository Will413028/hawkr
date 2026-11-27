package chat

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/rpc/errdetails"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1/chatv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeSub struct {
	events      chan im.Event
	closed      chan struct{}
	closeOnce   sync.Once
	revalidated chan im.Party
	reval       error
}

func newFakeSub() *fakeSub {
	return &fakeSub{events: make(chan im.Event, 16), closed: make(chan struct{}), revalidated: make(chan im.Party, 8)}
}

func (s *fakeSub) Next(ctx context.Context) (im.Event, error) {
	select {
	case ev := <-s.events:
		return ev, nil
	case <-ctx.Done():
		return im.Event{}, ctx.Err()
	}
}

func (s *fakeSub) Revalidate(_ context.Context, p im.Party) error {
	s.revalidated <- p
	return s.reval
}

func (s *fakeSub) Close() { s.closeOnce.Do(func() { close(s.closed) }) }

type fakeStreams struct {
	mu      sync.Mutex
	sub     *fakeSub
	err     error
	calls   int
	party   im.Party
	cursors map[uuid.UUID]int64
}

func (f *fakeStreams) Subscribe(_ context.Context, p im.Party, c map[uuid.UUID]int64) (Subscription, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.party, f.cursors = p, c
	if f.err != nil {
		return nil, f.err
	}
	return f.sub, nil
}

func (f *fakeStreams) snapshot() (int, im.Party, map[uuid.UUID]int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.party, f.cursors
}

type fakeAuth struct {
	mu     sync.Mutex
	err    error
	tokens []string
}

func (f *fakeAuth) Authenticate(_ context.Context, token string) (identity.Authenticated, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens = append(f.tokens, token)
	if f.err != nil {
		return identity.Authenticated{}, f.err
	}
	return identity.Authenticated{Principal: identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount}}, nil
}

type streamEnv struct {
	*env
	streams *fakeStreams
	auth    *fakeAuth
	clk     *fakeClock
	client  chatv1alpha1connect.ChatAppServiceClient
}

func newStreamEnv(t *testing.T, store *seller.Store) *streamEnv {
	t.Helper()
	e := newEnv(t, store)
	se := &streamEnv{env: e, streams: &fakeStreams{sub: newFakeSub()}, auth: &fakeAuth{}, clk: newFakeClock(t)}
	var err error
	if e.app, err = NewApp(e.reads, e.writes, se.streams, se.auth, e.buyers, e.stores, images(t), slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	e.app.reasonOf = testReasonOf
	e.app.stream, e.app.clock, e.app.jitter = testStreamConfig(), se.clk.clock(), func() float64 { return 0.5 }
	path, h := chatv1alpha1connect.NewChatAppServiceHandler(e.app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	mux := http.NewServeMux()
	mux.Handle(path, WithStreamWriter(h))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: callerAccount})))
	}))
	t.Cleanup(srv.Close)
	hc := *srv.Client()
	hc.Timeout = 5 * time.Second
	se.client = chatv1alpha1connect.NewChatAppServiceClient(&hc, srv.URL)
	return se
}

func fieldOf(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		for _, d := range ce.Details() {
			if v, e := d.Value(); e == nil {
				if br, ok := v.(*errdetails.BadRequest); ok && len(br.GetFieldViolations()) == 1 {
					return br.GetFieldViolations()[0].GetField()
				}
			}
		}
	}
	return ""
}

func reasonFromClient(err error) string {
	var ce *connect.Error
	if errors.As(err, &ce) {
		for _, d := range ce.Details() {
			if v, e := d.Value(); e == nil {
				if info, ok := v.(*errdetails.ErrorInfo); ok {
					return info.GetReason()
				}
			}
		}
	}
	return ""
}

func (se *streamEnv) openBuyer(t *testing.T, cursors map[string]int64) *connect.ServerStreamForClient[pb.StreamEventsResponse] {
	t.Helper()
	req := connect.NewRequest(&pb.StreamEventsRequest{Cursors: cursors})
	req.Header().Set("Authorization", "Bearer tok-1")
	st, err := se.client.StreamEvents(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func (se *streamEnv) push(t *testing.T, ev im.Event) {
	t.Helper()
	select {
	case se.streams.sub.events <- ev:
	case <-time.After(5 * time.Second):
		t.Fatal("串流沒有取走事件")
	}
}

func recv(t *testing.T, st *connect.ServerStreamForClient[pb.StreamEventsResponse]) *pb.ChatEvent {
	t.Helper()
	if !st.Receive() {
		t.Fatalf("串流提前結束：%v", st.Err())
	}
	return st.Msg().GetEvent()
}

func waitClosed(t *testing.T, s *fakeSub) {
	t.Helper()
	select {
	case <-s.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("handler 返回前沒有呼叫 Subscription.Close")
	}
}

func TestStreamEventsSubscribesAsTheDerivedParty(t *testing.T) {
	t.Run("買家版用買家身分", func(t *testing.T) {
		se := newStreamEnv(t, ownStore())
		se.push(t, im.Event{Kind: im.EventReady})
		st := se.openBuyer(t, nil)
		recv(t, st)
		_, p, _ := se.streams.snapshot()
		assertBuyerParty(t, p)
	})
	t.Run("店家版用此刻的店", func(t *testing.T) {
		se := newStreamEnv(t, ownStore())
		se.push(t, im.Event{Kind: im.EventReady})
		st, err := se.client.StreamStoreEvents(t.Context(), connect.NewRequest(&pb.StreamStoreEventsRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		if !st.Receive() {
			t.Fatal(st.Err())
		}
		_, p, _ := se.streams.snapshot()
		assertStoreParty(t, p)
	})
	t.Run("沒有店的店家版是 not_found 且不訂閱", func(t *testing.T) {
		se := newStreamEnv(t, nil)
		st, err := se.client.StreamStoreEvents(t.Context(), connect.NewRequest(&pb.StreamStoreEventsRequest{}))
		if err == nil {
			st.Receive()
			err = st.Err()
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("= %v", err)
		}
		if n, _, _ := se.streams.snapshot(); n != 0 {
			t.Errorf("Subscribe 被呼叫了 %d 次", n)
		}
	})
}

func TestStreamEventsCursors(t *testing.T) {
	t.Run("key 不是 UUID 是 invalid_argument 指向 cursors", func(t *testing.T) {
		se := newStreamEnv(t, ownStore())
		st := se.openBuyer(t, map[string]int64{"not-a-uuid": 3})
		st.Receive()
		if err := st.Err(); connect.CodeOf(err) != connect.CodeInvalidArgument || fieldOf(err) != "cursors" {
			t.Fatalf("= %v（欄位 %q）", err, fieldOf(err))
		}
		if n, _, _ := se.streams.snapshot(); n != 0 {
			t.Error("不合法的游標不該到模組")
		}
	})
	t.Run("合法游標原樣傳給模組", func(t *testing.T) {
		se := newStreamEnv(t, ownStore())
		se.push(t, im.Event{Kind: im.EventReady})
		st := se.openBuyer(t, map[string]int64{convID.String(): 9})
		recv(t, st)
		if _, _, c := se.streams.snapshot(); len(c) != 1 || c[convID] != 9 {
			t.Errorf("cursors = %v", c)
		}
	})
	t.Run("模組回超過上限的錯誤傳遞成 invalid_argument 指向 cursors", func(t *testing.T) {
		se := newStreamEnv(t, ownStore())
		se.streams.err = apperr.InvalidInputOn([]string{"Cursors"}, "%s", "游標太多")
		st := se.openBuyer(t, nil)
		st.Receive()
		if err := st.Err(); connect.CodeOf(err) != connect.CodeInvalidArgument || fieldOf(err) != "cursors" {
			t.Fatalf("= %v（欄位 %q）", err, fieldOf(err))
		}
	})
}

func TestStreamSubscribeFailuresMapToTheirCodes(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		code connect.Code
	}{
		"並行上限":      {apperr.ResourceExhausted(0, "太多串流"), connect.CodeResourceExhausted},
		"模組關閉":      {&im.Ended{Reason: im.EndShutdown}, connect.CodeUnavailable},
		"店家已失去讀取資格": {apperr.FailedPrecondition("關店"), connect.CodeFailedPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			se := newStreamEnv(t, ownStore())
			se.streams.err = tc.err
			st := se.openBuyer(t, nil)
			st.Receive()
			if got := connect.CodeOf(st.Err()); got != tc.code {
				t.Fatalf("code = %v，預期 %v（%v）", got, tc.code, st.Err())
			}
		})
	}
}

func TestStreamReadyCarriesUnreadAndTheStreamsActualHeartbeatAndLifetime(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	se.push(t, im.Event{Kind: im.EventReady, Unread: 4})
	st := se.openBuyer(t, nil)
	r := recv(t, st).GetReady()
	if r.GetUnreadCount() != 4 || r.GetHeartbeatInterval().AsDuration() != 15*time.Second || r.GetMaxLifetime().AsDuration() != 900*time.Second {
		t.Fatalf("Ready = %v", r)
	}
}

func TestStreamMapsEveryEventKindToItsFrame(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	text := "你好"
	key := "private/object-key.jpg"
	events := []im.Event{
		{Kind: im.EventReady, Unread: 1},
		{Kind: im.EventMessage, Message: sampleMessage(3), Updated: false},
		{Kind: im.EventMessage, Message: im.Message{ID: uuid.UUID{9}, ConversationID: convID, Seq: 4, SenderKind: im.PartyStore, Type: im.TypeText, Text: &text, Hidden: true, CreatedAt: at}, Updated: true},
		{Kind: im.EventMessage, Message: im.Message{ID: uuid.UUID{10}, ConversationID: convID, Seq: 5, SenderKind: im.PartyStore, Type: im.TypeImage, Image: &im.Image{ObjectKey: key}, CreatedAt: at}},
		{Kind: im.EventUnread, Unread: 7},
		{Kind: im.EventPeerRead, ConversationID: convID, PeerReadSeq: 5},
		{Kind: im.EventResync, ConversationID: convID},
		{Kind: im.EventResync, All: true},
	}
	for _, ev := range events {
		se.push(t, ev)
	}
	st := se.openBuyer(t, nil)
	var got []*pb.ChatEvent
	for range events {
		got = append(got, recv(t, st))
	}
	if got[0].GetReady().GetUnreadCount() != 1 {
		t.Errorf("ready = %v", got[0])
	}
	if m := got[1].GetMessage(); m.GetUpdated() || m.GetMessage().GetSeq() != 3 || m.GetMessage().GetText() != "哈囉" {
		t.Errorf("message = %v", got[1])
	}
	if m := got[2].GetMessage(); !m.GetUpdated() || !m.GetMessage().GetHidden() || m.GetMessage().Text != nil {
		t.Errorf("墓碑不帶內容、updated 為真：%v", got[2])
	}
	if img := got[3].GetMessage().GetMessage().GetImage(); img.GetUriTemplate() == "" || strings.Contains(got[3].String(), "object_key") {
		t.Errorf("圖片訊息只回組好的網址（與 unary 同一個 presenter）：%v", got[3])
	}
	if got[4].GetUnread().GetUnreadCount() != 7 {
		t.Errorf("unread = %v", got[4])
	}
	if p := got[5].GetPeerRead(); p.GetConversationId() != convID.String() || p.GetPeerReadSeq() != 5 {
		t.Errorf("peer_read = %v", got[5])
	}
	if r := got[6].GetResync(); r.GetConversationId() != convID.String() || r.GetAll() {
		t.Errorf("resync(conversation) = %v", got[6])
	}
	if r := got[7].GetResync(); !r.GetAll() || r.GetConversationId() != "" {
		t.Errorf("resync(all) = %v", got[7])
	}
}

func TestStreamHandlerClosesTheSubscriptionWhenTheClientGoesAway(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	ctx, cancel := context.WithCancel(t.Context())
	req := connect.NewRequest(&pb.StreamEventsRequest{})
	se.push(t, im.Event{Kind: im.EventReady})
	st, err := se.client.StreamEvents(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Receive() {
		t.Fatal(st.Err())
	}
	cancel()
	waitClosed(t, se.streams.sub)
}

func TestStreamHandlerClosesTheSubscriptionWhenTheStreamEndsOnItsOwn(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	se.push(t, im.Event{Kind: im.EventReady})
	st := se.openBuyer(t, nil)
	recv(t, st)
	se.clk.fire(lifeTimer, 1)
	for st.Receive() {
	}
	if err := st.Err(); err != nil {
		t.Fatalf("壽命到期要正常結束：%v", err)
	}
	waitClosed(t, se.streams.sub)
}

func TestStreamGuardReauthenticatesWithTheRequestTokenAndRevalidatesTheParty(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	se.push(t, im.Event{Kind: im.EventReady})
	st := se.openBuyer(t, nil)
	recv(t, st)
	se.clk.fire(chkTimer, 1)
	select {
	case p := <-se.streams.sub.revalidated:
		assertBuyerParty(t, p)
	case <-time.After(5 * time.Second):
		t.Fatal("重驗成功後沒有呼叫 Revalidate")
	}
	se.auth.mu.Lock()
	defer se.auth.mu.Unlock()
	if len(se.auth.tokens) != 1 || se.auth.tokens[0] != "tok-1" {
		t.Errorf("重新驗證要用請求帶的 token：%v", se.auth.tokens)
	}
}

func TestStreamGuardSessionGoneEndsUnauthenticated(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	se.push(t, im.Event{Kind: im.EventReady})
	st := se.openBuyer(t, nil)
	recv(t, st)
	se.auth.mu.Lock()
	se.auth.err = apperr.Unauthenticated("session 失效")
	se.auth.mu.Unlock()
	se.clk.fire(chkTimer, 1)
	for st.Receive() {
	}
	if connect.CodeOf(st.Err()) != connect.CodeUnauthenticated {
		t.Fatalf("= %v", st.Err())
	}
	waitClosed(t, se.streams.sub)
}

func TestStreamGuardStoreGoneEndsWithStoreClosed(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	se.push(t, im.Event{Kind: im.EventReady})
	st, err := se.client.StreamStoreEvents(t.Context(), func() *connect.Request[pb.StreamStoreEventsRequest] {
		r := connect.NewRequest(&pb.StreamStoreEventsRequest{})
		r.Header().Set("Authorization", "Bearer tok-1")
		return r
	}())
	if err != nil {
		t.Fatal(err)
	}
	st.Receive()
	se.stores.own = nil
	se.clk.fire(chkTimer, 1)
	for st.Receive() {
	}
	if err := st.Err(); connect.CodeOf(err) != connect.CodeFailedPrecondition || reasonFromClient(err) != "STORE_CLOSED" {
		t.Fatalf("= %v（reason %q）", err, reasonFromClient(err))
	}
}

func TestStreamGuardBuyerIdentityChangedEndsUnauthenticated(t *testing.T) {
	se := newStreamEnv(t, ownStore())
	se.push(t, im.Event{Kind: im.EventReady})
	st := se.openBuyer(t, nil)
	recv(t, st)
	se.buyers.swapped.Store(71)
	se.clk.fire(chkTimer, 1)
	for st.Receive() {
	}
	if err := st.Err(); connect.CodeOf(err) != connect.CodeUnauthenticated || reasonFromClient(err) == "STORE_CLOSED" {
		t.Fatalf("買家身分對不上要 unauthenticated：%v（reason %q）", err, reasonFromClient(err))
	}
	waitClosed(t, se.streams.sub)
}
