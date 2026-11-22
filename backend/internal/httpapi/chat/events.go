package chat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
	"uuid"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/identity"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// Subscription 是一條即時事件流（*im.Subscription 滿足它；測試換成假的）。
type Subscription interface {
	Next(ctx context.Context) (im.Event, error)
	Revalidate(ctx context.Context, p im.Party) error
	Close()
}

// Streams 是即時事件的來源（用 ModuleStreams 包 *im.Module）。
type Streams interface {
	Subscribe(ctx context.Context, p im.Party, cursors map[uuid.UUID]int64) (Subscription, error)
}

type moduleStreams struct{ m *im.Module }

// ModuleStreams 把 *im.Module 接成 Streams。
func ModuleStreams(m *im.Module) Streams { return moduleStreams{m} }

func (s moduleStreams) Subscribe(ctx context.Context, p im.Party, cursors map[uuid.UUID]int64) (Subscription, error) {
	sub, err := s.m.Subscribe(ctx, p, cursors)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// Reauthenticator 重新驗證 session token（identity.Module 滿足它；與入口的 withSession 用同一個驗證入口）。
type Reauthenticator interface {
	Authenticate(ctx context.Context, token string) (identity.Authenticated, error)
}

type writeDeadlineKey struct{}

// WithStreamWriter 讓串流 handler 能設定底層連線的寫入期限：Connect 的 ServerStream 拿不到 http.ResponseWriter，所以由這一層
// 中介層以 http.ResponseController 包住它，把設定函式放進 request context。ResponseController 靠每一層 wrapper 的 Unwrap
// 找到底層連線，入口的中介層都實作了 Unwrap（測試 TestStreamWriteDeadlineReachesTheConnectionThroughTheMiddlewareChain 守著）。
func WithStreamWriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		set := func(t time.Time) error { return rc.SetWriteDeadline(t) }
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), writeDeadlineKey{}, set)))
	})
}

// StreamWriteDeadline 回傳 WithStreamWriter 放進 ctx 的寫入期限設定函式；請求沒有經過 WithStreamWriter 時是 nil。
func StreamWriteDeadline(ctx context.Context) func(time.Time) error {
	set, _ := ctx.Value(writeDeadlineKey{}).(func(time.Time) error)
	return set
}

// classifyEnd 把模組結束串流的原因對到對外的錯誤：關店是 failed_precondition 與 STORE_CLOSED，伺服器關閉是 unavailable（client 依 backoff 重連）。
func classifyEnd(err error) (error, bool) {
	var ended *im.Ended
	if !errors.As(err, &ended) {
		return nil, false
	}
	switch ended.Reason {
	case im.EndLostEligibility:
		return apperr.FailedPrecondition("店家已停業，無法再收到聊天事件").WithReason(apperr.ReasonStoreClosed), true
	case im.EndShutdown:
	}
	return apperr.WithCause(apperr.Unavailable("服務重新啟動中，請稍後重連"), err), true
}

// parseCursors 解析 cursors：key 必須是對話 ID。數量上限由模組守（MaxCursors），超過回 invalid_argument 指向 cursors。
func parseCursors(raw map[string]int64) (map[uuid.UUID]int64, error) {
	out := make(map[uuid.UUID]int64, len(raw))
	for k, v := range raw {
		id, err := uuid.Parse(k)
		if err != nil {
			return nil, rpc.RequiredField("cursors", "cursors 的 key 必須是有效的對話 ID")
		}
		out[id] = v
	}
	return out, nil
}

// toEvent 把模組事件轉成 frame。訊息經 toMessage（墓碑不帶內容、不回 object key）；模組事件沒有的欄位不發明。
func (h *App) toEvent(ev im.Event, info streamInfo) (*pb.ChatEvent, error) {
	switch ev.Kind {
	case im.EventReady:
		return &pb.ChatEvent{Event: &pb.ChatEvent_Ready{Ready: &pb.Ready{
			UnreadCount: ev.Unread, HeartbeatInterval: durationpb.New(info.HeartbeatEvery), MaxLifetime: durationpb.New(info.MaxLifetime),
		}}}, nil
	case im.EventMessage:
		m, err := h.toMessage(ev.Message)
		if err != nil {
			return nil, err
		}
		return &pb.ChatEvent{Event: &pb.ChatEvent_Message{Message: &pb.MessageEvent{Message: m, Updated: ev.Updated}}}, nil
	case im.EventUnread:
		return &pb.ChatEvent{Event: &pb.ChatEvent_Unread{Unread: &pb.UnreadChanged{UnreadCount: ev.Unread}}}, nil
	case im.EventPeerRead:
		return &pb.ChatEvent{Event: &pb.ChatEvent_PeerRead{PeerRead: &pb.PeerRead{ConversationId: ev.ConversationID.String(), PeerReadSeq: ev.PeerReadSeq}}}, nil
	case im.EventResync:
		r := &pb.Resync{}
		if ev.All {
			r.Scope = &pb.Resync_All{All: true}
		} else {
			r.Scope = &pb.Resync_ConversationId{ConversationId: ev.ConversationID.String()}
		}
		return &pb.ChatEvent{Event: &pb.ChatEvent_Resync{Resync: r}}, nil
	}
	return nil, fmt.Errorf("httpapi/chat：不認得的事件種類 %d", ev.Kind)
}

// guardTimeout 是一次重新驗證（驗 session、讀身分與店家）的時間上限。
const guardTimeout = 10 * time.Second

// guard 回傳重新驗證的函式：以請求帶的 token 重新驗證 session（與入口同一個 Authenticate；帳號必須還是原來那個），再用 reparty 重讀身分，
// 成功後交給 Subscription.Revalidate。session 失效、帳號刪除、買家身分對不上是 unauthenticated，店家的店沒了或換了是 Ended（EndLostEligibility）；身分讀取的暫時錯誤原樣回（算暫時失敗）。
func (h *App) guard(header http.Header, accountID int64, kind im.PartyKind, sub Subscription, reparty func(context.Context, int64) (im.Party, error), same func(im.Party) bool) func(context.Context) error {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, guardTimeout)
		defer cancel()
		token, _ := rpc.CredentialFromHeader(header)
		if token == "" {
			return apperr.Unauthenticated("需要登入")
		}
		a, err := h.auth.Authenticate(ctx, token)
		if err != nil {
			return err
		}
		if a.Principal.Kind != identity.PrincipalAccount || a.Principal.ID != accountID {
			return apperr.Unauthenticated("需要登入")
		}
		p, err := reparty(ctx, accountID)
		switch {
		case err == nil && same(p):
			return sub.Revalidate(ctx, p)
		case err == nil, apperr.KindOf(err) == apperr.KindNotFound:
			if kind == im.PartyStore {
				// 店沒了或換成另一家：這條串流的店已不再有效，與關店同一個結果。
				return &im.Ended{Reason: im.EndLostEligibility}
			}
			// 買家身分對不上屬於 session 或帳號層的失效。
			return apperr.Unauthenticated("需要登入")
		}
		return err
	}
}

// serveStream 是兩支串流方法共用的流程：解析 cursors、訂閱、交給 runStream。身分由 party 在入口推導一次，之後由 guard 重驗。
func (h *App) serveStream(ctx context.Context, header http.Header, rawCursors map[string]int64, out func(*pb.ChatEvent) error,
	party func(context.Context) (im.Party, error), reparty func(context.Context, int64) (im.Party, error)) error {
	cursors, err := parseCursors(rawCursors)
	if err != nil {
		return err
	}
	setDeadline := StreamWriteDeadline(ctx)
	if setDeadline == nil {
		return errors.New("httpapi/chat：請求沒有經過 WithStreamWriter，串流拿不到寫入期限的設定")
	}
	acct, err := rpc.RequireAccount(ctx)
	if err != nil {
		return err
	}
	p, err := party(ctx)
	if err != nil {
		return err
	}
	sub, err := h.streams.Subscribe(ctx, p, cursors)
	if err != nil {
		if end, ok := classifyEnd(err); ok {
			return end
		}
		return h.failure(err, rpc.FieldPaths{"Cursors": "cursors"})
	}
	same := func(q im.Party) bool { return q.Kind == p.Kind && q.ID() == p.ID() }
	return runStream(ctx, streamSpec[im.Event]{
		Config: h.stream, Clock: h.clock, Jitter: h.jitter,
		Next: sub.Next, Close: sub.Close, Classify: classifyEnd,
		Guard:            h.guard(header, acct.ID, p.Kind, sub, reparty, same),
		SetWriteDeadline: setDeadline,
		Write: func(ev im.Event, info streamInfo) error {
			f, err := h.toEvent(ev, info)
			if err != nil {
				return err
			}
			return out(f)
		},
		Heartbeat: func() error {
			return out(&pb.ChatEvent{Event: &pb.ChatEvent_Heartbeat{Heartbeat: &pb.Heartbeat{}}})
		},
	})
}

func (h *App) StreamEvents(ctx context.Context, req *connect.Request[pb.StreamEventsRequest], st *connect.ServerStream[pb.StreamEventsResponse]) error {
	return h.serveStream(ctx, req.Header(), req.Msg.GetCursors(),
		func(f *pb.ChatEvent) error { return st.Send(&pb.StreamEventsResponse{Event: f}) },
		h.buyerParty, h.buyerPartyOf)
}

func (h *App) StreamStoreEvents(ctx context.Context, req *connect.Request[pb.StreamStoreEventsRequest], st *connect.ServerStream[pb.StreamStoreEventsResponse]) error {
	return h.serveStream(ctx, req.Header(), req.Msg.GetCursors(),
		func(f *pb.ChatEvent) error { return st.Send(&pb.StreamStoreEventsResponse{Event: f}) },
		h.storeParty, h.storePartyOf)
}
