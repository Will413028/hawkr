package chat

import (
	"context"
	"fmt"
	"log/slog"
	"uuid"

	"connectrpc.com/connect"

	pb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/chat/v1alpha1/chatv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/modules/im"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

// App 實作 ChatAppService。
type App struct {
	reads  Reads
	writes Writes
	buyers Buyers
	stores Stores
	images rpc.Images
	log    *slog.Logger
	// streams 與 auth 給串流用：訂閱即時事件，與每 RecheckEvery 重新驗證 session。
	streams Streams
	auth    Reauthenticator
	// stream 是串流傳輸的常數、時鐘與抖動來源；正式組裝用預設值，測試注入假的。
	stream streamConfig
	clock  streamClock
	jitter func() float64
	// reasonOf 取出模組錯誤帶的業務原因；正式組裝是 im.ReasonOf，測試換成固定的原因（模組的原因型別不能在模組外建構）。
	reasonOf func(error) im.Reason
}

var _ chatv1alpha1connect.ChatAppServiceHandler = (*App)(nil)

// NewApp 建立登入者的聊天 handler。能力宣告表（declarations）與 ChatAppService 的方法對不上時回錯誤，不能啟動。
func NewApp(reads Reads, writes Writes, streams Streams, auth Reauthenticator, buyers Buyers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	return newApp(declarations, reads, writes, streams, auth, buyers, stores, images, log)
}

// newApp 的宣告表由參數給（測試用缺一列的表驗證啟動檢查）。
func newApp(table map[string]declaration, reads Reads, writes Writes, streams Streams, auth Reauthenticator, buyers Buyers, stores Stores, images rpc.Images, log *slog.Logger) (*App, error) {
	switch {
	case log == nil:
		return nil, fmt.Errorf("httpapi/chat：需要 logger")
	case reads == nil:
		return nil, fmt.Errorf("httpapi/chat：需要 Reads")
	case writes == nil:
		return nil, fmt.Errorf("httpapi/chat：需要 Writes")
	case streams == nil:
		return nil, fmt.Errorf("httpapi/chat：需要 Streams")
	case auth == nil:
		return nil, fmt.Errorf("httpapi/chat：需要 Reauthenticator")
	case buyers == nil:
		return nil, fmt.Errorf("httpapi/chat：需要 Buyers")
	case stores == nil:
		return nil, fmt.Errorf("httpapi/chat：需要 Stores")
	}
	if err := checkDeclarations(table, serviceDescriptor()); err != nil {
		return nil, err
	}
	return &App{
		reads: reads, writes: writes, streams: streams, auth: auth, buyers: buyers, stores: stores, images: images, log: log, reasonOf: im.ReasonOf,
		stream: defaultStreamConfig(), clock: realClock(), jitter: randFloat,
	}, nil
}

func (h *App) ListConversations(ctx context.Context, req *connect.Request[pb.ListConversationsRequest]) (*connect.Response[pb.ListConversationsResponse], error) {
	size, err := pageSize(req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	p, err := h.buyerParty(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(chatv1alpha1connect.ChatAppServiceListConversationsProcedure, p.BuyerID)
	after, err := decodeCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.reads.ListConversations(ctx, p, after, size)
	if err != nil {
		return nil, h.failure(err, nil)
	}
	items, err := h.buyerConversations(ctx, page.Items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListConversationsResponse{Conversations: items, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *App) ListStoreConversations(ctx context.Context, req *connect.Request[pb.ListStoreConversationsRequest]) (*connect.Response[pb.ListStoreConversationsResponse], error) {
	size, err := pageSize(req.Msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	p, err := h.storeParty(ctx)
	if err != nil {
		return nil, err
	}
	scope := rpc.PageScope(chatv1alpha1connect.ChatAppServiceListStoreConversationsProcedure, p.ID())
	after, err := decodeCursor(scope, req.Msg.GetPageToken())
	if err != nil {
		return nil, err
	}
	page, err := h.reads.ListConversations(ctx, p, after, size)
	if err != nil {
		return nil, h.failure(err, nil)
	}
	items, err := h.storeConversations(ctx, page.Items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListStoreConversationsResponse{Conversations: items, NextPageToken: nextToken(scope, page.Next)}), nil
}

func (h *App) GetStoreConversationByBuyer(ctx context.Context, req *connect.Request[pb.GetStoreConversationByBuyerRequest]) (*connect.Response[pb.GetStoreConversationByBuyerResponse], error) {
	buyerID, err := rpc.ParseID("buyer_id", req.Msg.GetBuyerId())
	if err != nil {
		return nil, err
	}
	// 契約承諾不區分原因：沒有店、店家關閉、買家不存在、沒有對話都是同一個 not_found（同 code、訊息與 details）。
	p, err := h.storeParty(ctx)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return nil, conversationNotFound()
		}
		return nil, err
	}
	c, err := h.reads.StoreConversationByBuyer(ctx, p, buyerID)
	if err != nil {
		if apperr.KindOf(err) == apperr.KindNotFound {
			return nil, conversationNotFound()
		}
		return nil, h.failure(err, nil)
	}
	items, err := h.storeConversations(ctx, []im.Conversation{c})
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetStoreConversationByBuyerResponse{Conversation: items[0]}), nil
}

func (h *App) ListMessages(ctx context.Context, req *connect.Request[pb.ListMessagesRequest]) (*connect.Response[pb.ListMessagesResponse], error) {
	msg := req.Msg
	id, err := rpc.ParseUUID("conversation_id", msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	size, err := pageSize(msg.GetPageSize())
	if err != nil {
		return nil, err
	}
	if msg.BeforeSeq != nil && msg.AfterSeq != nil {
		return nil, rpc.RequiredField("after_seq", "before_seq 與 after_seq 不能同時給")
	}
	if msg.BeforeSeq != nil && *msg.BeforeSeq < 0 {
		return nil, rpc.RequiredField("before_seq", "before_seq 不能是負數")
	}
	if msg.AfterSeq != nil && *msg.AfterSeq < 0 {
		return nil, rpc.RequiredField("after_seq", "after_seq 不能是負數")
	}
	p, err := h.partyOf(ctx, id)
	if err != nil {
		return nil, err
	}
	var page im.MessagePage
	if msg.AfterSeq != nil {
		page, err = h.reads.MessagesAfter(ctx, id, p, *msg.AfterSeq, size)
	} else {
		page, err = h.reads.ListMessages(ctx, id, p, beforeSeq(msg), size)
	}
	if err != nil {
		return nil, h.failure(err, nil)
	}
	items, err := h.toMessages(page.Items)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.ListMessagesResponse{Messages: items, HasMore: page.HasMore, PeerReadSeq: page.PeerReadSeq}), nil
}

// beforeSeq 是交給模組的 before_seq（模組以 0 表示從最新開始）：沒設是 0；明確給 0 是「seq 小於 0」，序號從 1 起所以沒有訊息，
// 以 1 表示（同樣沒有 seq 小於 1 的訊息），仍回真正的 peer_read_seq。
func beforeSeq(msg *pb.ListMessagesRequest) int64 {
	if msg.BeforeSeq == nil {
		return 0
	}
	return max(*msg.BeforeSeq, 1)
}

// sendInput 解析送出的共同欄位：request_id 必填 UUIDv4（模組另驗），內容恰好是文字或圖片其中一個。
func sendInput(requestID string, content *pb.MessageContent) (im.SendInput, error) {
	rid, err := rpc.ParseRequestID(requestID)
	if err != nil {
		return im.SendInput{}, err
	}
	if rid == nil {
		return im.SendInput{}, rpc.RequiredField("request_id", "request_id 為必填")
	}
	in := im.SendInput{RequestID: *rid}
	switch c := content.GetContent().(type) {
	case *pb.MessageContent_Text:
		in.Type, in.Text = im.TypeText, c.Text
	case *pb.MessageContent_ImageStagedToken:
		in.Type, in.StagedToken = im.TypeImage, c.ImageStagedToken
	default:
		return im.SendInput{}, rpc.RequiredField("content", "content 必須是文字或圖片其中一個")
	}
	return in, nil
}

func (h *App) send(ctx context.Context, in im.SendInput, paths rpc.FieldPaths) (*pb.Message, error) {
	m, err := h.writes.Send(ctx, in)
	if err != nil {
		return nil, h.failure(err, paths)
	}
	return h.toMessage(m)
}

func (h *App) SendMessage(ctx context.Context, req *connect.Request[pb.SendMessageRequest]) (*connect.Response[pb.SendMessageResponse], error) {
	msg := req.Msg
	id, err := rpc.ParseUUID("conversation_id", msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	in, err := sendInput(msg.GetRequestId(), msg.GetContent())
	if err != nil {
		return nil, err
	}
	if in.Sender, err = h.partyOf(ctx, id); err != nil {
		return nil, err
	}
	in.ConversationID = id
	m, err := h.send(ctx, in, contentPaths)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SendMessageResponse{Message: m}), nil
}

func (h *App) SendMessageToStore(ctx context.Context, req *connect.Request[pb.SendMessageToStoreRequest]) (*connect.Response[pb.SendMessageToStoreResponse], error) {
	msg := req.Msg
	storeID, err := rpc.ParseID("store_id", msg.GetStoreId())
	if err != nil {
		return nil, err
	}
	in, err := sendInput(msg.GetRequestId(), msg.GetContent())
	if err != nil {
		return nil, err
	}
	if in.Sender, err = h.buyerParty(ctx); err != nil {
		return nil, err
	}
	in.CounterpartID = storeID
	m, err := h.send(ctx, in, toStorePaths)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SendMessageToStoreResponse{Message: m}), nil
}

func (h *App) SendMessageToBuyer(ctx context.Context, req *connect.Request[pb.SendMessageToBuyerRequest]) (*connect.Response[pb.SendMessageToBuyerResponse], error) {
	msg := req.Msg
	buyerID, err := rpc.ParseID("buyer_id", msg.GetBuyerId())
	if err != nil {
		return nil, err
	}
	in, err := sendInput(msg.GetRequestId(), msg.GetContent())
	if err != nil {
		return nil, err
	}
	if in.Sender, err = h.storeParty(ctx); err != nil {
		return nil, err
	}
	in.CounterpartID = buyerID
	m, err := h.send(ctx, in, toBuyerPaths)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.SendMessageToBuyerResponse{Message: m}), nil
}

func (h *App) MarkConversationRead(ctx context.Context, req *connect.Request[pb.MarkConversationReadRequest]) (*connect.Response[pb.MarkConversationReadResponse], error) {
	msg := req.Msg
	id, err := rpc.ParseUUID("conversation_id", msg.GetConversationId())
	if err != nil {
		return nil, err
	}
	if msg.GetUptoSeq() < 0 {
		return nil, rpc.RequiredField("upto_seq", "upto_seq 不能是負數")
	}
	p, err := h.partyOf(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := h.writes.MarkRead(ctx, id, p, msg.GetUptoSeq()); err != nil {
		return nil, h.failure(err, nil)
	}
	return connect.NewResponse(&pb.MarkConversationReadResponse{}), nil
}

func (h *App) MarkAllRead(ctx context.Context, _ *connect.Request[pb.MarkAllReadRequest]) (*connect.Response[pb.MarkAllReadResponse], error) {
	p, err := h.buyerParty(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.writes.MarkAllRead(ctx, p); err != nil {
		return nil, h.failure(err, nil)
	}
	return connect.NewResponse(&pb.MarkAllReadResponse{}), nil
}

func (h *App) MarkAllStoreRead(ctx context.Context, _ *connect.Request[pb.MarkAllStoreReadRequest]) (*connect.Response[pb.MarkAllStoreReadResponse], error) {
	p, err := h.storeParty(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.writes.MarkAllRead(ctx, p); err != nil {
		return nil, h.failure(err, nil)
	}
	return connect.NewResponse(&pb.MarkAllStoreReadResponse{}), nil
}

func (h *App) GetUnreadCount(ctx context.Context, _ *connect.Request[pb.GetUnreadCountRequest]) (*connect.Response[pb.GetUnreadCountResponse], error) {
	p, err := h.buyerParty(ctx)
	if err != nil {
		return nil, err
	}
	n, err := h.reads.UnreadTotal(ctx, p)
	if err != nil {
		return nil, h.failure(err, nil)
	}
	return connect.NewResponse(&pb.GetUnreadCountResponse{UnreadCount: n}), nil
}

func (h *App) GetStoreUnreadCount(ctx context.Context, _ *connect.Request[pb.GetStoreUnreadCountRequest]) (*connect.Response[pb.GetStoreUnreadCountResponse], error) {
	p, err := h.storeParty(ctx)
	if err != nil {
		return nil, err
	}
	n, err := h.reads.UnreadTotal(ctx, p)
	if err != nil {
		return nil, h.failure(err, nil)
	}
	return connect.NewResponse(&pb.GetStoreUnreadCountResponse{UnreadCount: n}), nil
}

// presence 查呼叫者這一方的對話的對方是否在線：ID 先檢查數量與格式，不是自己的對話由模組回 not_found（整個請求失敗）。
func (h *App) presence(ctx context.Context, rawIDs []string, party func(context.Context) (im.Party, error)) ([]*pb.ConversationPresence, error) {
	if len(rawIDs) == 0 {
		return nil, rpc.RequiredField("conversation_ids", "conversation_ids 至少要有一個")
	}
	if len(rawIDs) > im.MaxPresenceConversations {
		return nil, rpc.RequiredField("conversation_ids", fmt.Sprintf("conversation_ids 一次最多 %d 個", im.MaxPresenceConversations))
	}
	ids := make([]uuid.UUID, len(rawIDs))
	for i, raw := range rawIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			bad := apperr.InvalidInputOn([]string{apperr.IndexedField("conversation_ids", i)}, "%s", "conversation_ids 必須是有效的 ID")
			return nil, rpc.WithFieldPaths(bad, rpc.FieldPaths{"conversation_ids[]": "conversation_ids[]"})
		}
		ids[i] = id
	}
	p, err := party(ctx)
	if err != nil {
		return nil, err
	}
	states, err := h.reads.Presence(ctx, p, ids)
	if err != nil {
		return nil, h.failure(err, contentPaths)
	}
	out := make([]*pb.ConversationPresence, len(states))
	for i, s := range states {
		out[i] = &pb.ConversationPresence{ConversationId: s.ConversationID.String(), Online: s.Online}
	}
	return out, nil
}

func (h *App) GetPresence(ctx context.Context, req *connect.Request[pb.GetPresenceRequest]) (*connect.Response[pb.GetPresenceResponse], error) {
	out, err := h.presence(ctx, req.Msg.GetConversationIds(), h.buyerParty)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetPresenceResponse{Presences: out}), nil
}

func (h *App) GetStorePresence(ctx context.Context, req *connect.Request[pb.GetStorePresenceRequest]) (*connect.Response[pb.GetStorePresenceResponse], error) {
	out, err := h.presence(ctx, req.Msg.GetConversationIds(), h.storeParty)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetStorePresenceResponse{Presences: out}), nil
}
