package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
	"uuid"

	"connectrpc.com/connect"

	httpchat "github.com/Will413028/hawkr/backend/internal/httpapi/chat"
	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1/chatv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/modules/seller"
)

// gatedSub 逐筆交出事件：Next 阻塞到測試給下一筆，所以測試能確認「第 N 個 frame 在第 N+1 個事件產生之前就到達 client」。
type gatedSub struct{ gate chan im.Event }

func (s gatedSub) Next(ctx context.Context) (im.Event, error) {
	select {
	case ev := <-s.gate:
		return ev, nil
	case <-ctx.Done():
		return im.Event{}, ctx.Err()
	}
}
func (gatedSub) Revalidate(context.Context, im.Party) error { return nil }
func (gatedSub) Close()                                     {}

type gatedStreams struct{ sub gatedSub }

func (g gatedStreams) Subscribe(context.Context, im.Party, map[uuid.UUID]int64) (httpchat.Subscription, error) {
	return g.sub, nil
}

type chatBuyers struct{}

func (chatBuyers) CurrentBuyerID(context.Context, int64) (int64, error) { return 70, nil }
func (chatBuyers) BatchDisplay(context.Context, []int64) ([]buyer.Display, error) {
	return nil, nil
}

type chatStores struct{}

func (chatStores) CurrentStore(context.Context, int64) (seller.Store, error) {
	return seller.Store{}, errors.New("unused")
}
func (chatStores) StoreCards(context.Context, []int64) ([]seller.StoreCard, error) { return nil, nil }

type chatAuth struct{}

func (chatAuth) Authenticate(context.Context, string) (identity.Authenticated, error) {
	return identity.Authenticated{}, errors.New("unused")
}

// asAccount 以帳號 7 的身分執行 next（入口的 withSession 在正式環境做同一件事；assemble 的 Identity 是具體型別，測試從 service 這一層放身分）。
func asAccount(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(rpc.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalAccount, ID: 7})))
	})
}

// 串流經真正的 ChatAppService handler 與入口的整條中介鏈（tracing、CSRF、session、限流、快取規則、錯誤 log）時逐筆送出：
// 假的來源在測試確認收到上一個 frame 之前不產生下一個事件。任何一層把回應緩衝起來，client 收不到 header 或 frame（ADR 2026-11-22 的義務）。
func TestChatStreamIsNotBufferedThroughTheFullMiddlewareChain(t *testing.T) {
	gate := make(chan im.Event)
	app, err := httpchat.NewApp(&im.Module{}, &im.Module{}, gatedStreams{gatedSub{gate}}, chatAuth{}, chatBuyers{}, chatStores{}, mustImages(t), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	path, h := chatv1alpha1connect.NewChatAppServiceHandler(app, connect.WithInterceptors(rpc.NewInterceptor(slog.New(slog.DiscardHandler))))
	handler, err := assemble(Deps{Logger: slog.New(slog.DiscardHandler)}, []service{{path, asAccount(httpchat.WithStreamWriter(h))}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	// header 要在第一個 frame 送出時就到：緩衝的中介層連 header 都等到回應結束才送。
	transport := srv.Client().Transport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 3 * time.Second
	client := chatv1alpha1connect.NewChatAppServiceClient(&http.Client{Transport: transport}, srv.URL)

	go func() { gate <- im.Event{Kind: im.EventReady, Unread: 1} }()
	st, err := client.StreamEvents(t.Context(), connect.NewRequest(&pb.StreamEventsRequest{}))
	if err != nil {
		t.Fatalf("沒有收到 header：中介層把串流緩衝了（%v）", err)
	}
	if !st.Receive() || st.Msg().GetEvent().GetReady().GetUnreadCount() != 1 {
		t.Fatalf("第一個 frame 沒到：%v", st.Err())
	}
	for i := int64(1); i <= 3; i++ {
		// 這個事件在上一個 frame 已被 client 收下之後才產生。
		select {
		case gate <- im.Event{Kind: im.EventUnread, Unread: 10 + i}:
		case <-time.After(5 * time.Second): // 只防測試卡死
			t.Fatal("來源的事件沒有被取走")
		}
		recvDone := make(chan bool, 1)
		go func() { recvDone <- st.Receive() }()
		select {
		case ok := <-recvDone:
			if !ok || st.Msg().GetEvent().GetUnread().GetUnreadCount() != 10+i {
				t.Fatalf("第 %d 個事件的 frame = %v（%v）", i, st.Msg(), st.Err())
			}
		case <-time.After(3 * time.Second): // 緩衝的中介層會讓 frame 一直不到
			t.Fatalf("第 %d 個 frame 在下一個事件產生前沒有到達 client：中介層把串流緩衝了", i)
		}
	}
}

func mustImages(t *testing.T) rpc.Images {
	t.Helper()
	im, err := rpc.NewImages("https://media.example.test")
	if err != nil {
		t.Fatal(err)
	}
	return im
}

// SetWriteDeadline 經整條中介鏈真的作用在底層連線：handler 設一個已過去的期限，隨後的寫入在連線上以 deadline 錯誤失敗
// （不是 http.ErrNotSupported，也不是靜默成功）。期限在過去，所以結果與機器速度、時鐘無關。
func TestStreamWriteDeadlineReachesTheConnectionThroughTheMiddlewareChain(t *testing.T) {
	type result struct{ set, flush error }
	got := make(chan result, 1)
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		set := httpchat.StreamWriteDeadline(r.Context())
		if set == nil {
			got <- result{set: errors.New("ctx 沒有寫入期限的設定函式")}
			return
		}
		var res result
		res.set = set(time.Now().Add(-time.Second))
		w.Header().Set("Content-Type", "application/connect+json")
		// 寫入超過 server 的緩衝，一定會碰到連線（Flush 經只有 Flush() 的 wrapper 會吞掉錯誤，所以看 Write 的結果）。
		_, res.flush = w.Write(make([]byte, 256<<10))
		got <- res
	})
	const path = "/hawkr.test.v1alpha1.StreamPublicService/"
	handler, err := assemble(Deps{Logger: slog.New(slog.DiscardHandler)}, []service{{path, httpchat.WithStreamWriter(probe)}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+path+"Watch", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/connect+json")
	req.Header.Set("Connect-Protocol-Version", "1")
	if resp, err := srv.Client().Do(req); err == nil {
		_ = resp.Body.Close()
	}
	r := <-got
	if r.set != nil {
		t.Fatalf("SetWriteDeadline 經中介鏈失敗（有 wrapper 沒有 Unwrap？）：%v", r.set)
	}
	if !errors.Is(r.flush, os.ErrDeadlineExceeded) {
		t.Fatalf("過去的寫入期限要讓寫入失敗，得到 %v", r.flush)
	}
}
