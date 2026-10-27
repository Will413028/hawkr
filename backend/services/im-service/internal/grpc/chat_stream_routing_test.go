package grpc

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

type streamHubMessage struct {
	userID  int64
	message domain.Message
}

type streamHubPresence struct {
	client *domain.Client
	peer   domain.PresenceKey
}

// recordingStreamHub is intentionally small but preserves the real Hub
// contract: registration is observable, targeted sends are copied into the
// connection queue, and presence subscriptions are tracked separately. This
// lets the stream tests exercise the adapter + usecase processor over bufconn
// without requiring Redis/Postgres or a second transport.
type recordingStreamHub struct {
	mu           sync.RWMutex
	clients      map[int64]map[*domain.Client]struct{}
	registered   chan *domain.Client
	unregistered chan *domain.Client
	sent         chan streamHubMessage
	subscribed   chan streamHubPresence
	unsubscribed chan streamHubPresence
}

func newRecordingStreamHub() *recordingStreamHub {
	return &recordingStreamHub{
		clients:      make(map[int64]map[*domain.Client]struct{}),
		registered:   make(chan *domain.Client, 4),
		unregistered: make(chan *domain.Client, 4),
		sent:         make(chan streamHubMessage, 16),
		subscribed:   make(chan streamHubPresence, 4),
		unsubscribed: make(chan streamHubPresence, 4),
	}
}

func (h *recordingStreamHub) Run() {}

func (h *recordingStreamHub) Register(client *domain.Client) {
	h.mu.Lock()
	conns := h.clients[client.UserID]
	if conns == nil {
		conns = make(map[*domain.Client]struct{})
		h.clients[client.UserID] = conns
	}
	conns[client] = struct{}{}
	h.mu.Unlock()
	h.registered <- client
}

func (h *recordingStreamHub) Unregister(client *domain.Client) {
	h.mu.Lock()
	if conns := h.clients[client.UserID]; conns != nil {
		delete(conns, client)
		if len(conns) == 0 {
			delete(h.clients, client.UserID)
		}
	}
	h.mu.Unlock()
	h.unregistered <- client
}

func (h *recordingStreamHub) SendToUser(userID int64, message *domain.Message) bool {
	return h.sendToClients(userID, 0, message)
}

func (h *recordingStreamHub) SendToUserRole(userID int64, loginRole int32, message *domain.Message) bool {
	return h.sendToClients(userID, loginRole, message)
}

func (h *recordingStreamHub) sendToClients(userID int64, loginRole int32, message *domain.Message) bool {
	if message == nil {
		return false
	}
	copyMessage := *message
	h.sent <- streamHubMessage{userID: userID, message: copyMessage}
	data, err := json.Marshal(message)
	if err != nil {
		return false
	}
	sent := false
	h.mu.RLock()
	for client := range h.clients[userID] {
		if loginRole != 0 && client.LoginRole != 0 && client.LoginRole != loginRole {
			continue
		}
		select {
		case client.Send <- data:
			sent = true
		default:
		}
	}
	h.mu.RUnlock()
	return sent
}

func (h *recordingStreamHub) Broadcast(message []byte) {}

func (h *recordingStreamHub) BroadcastExcept(_ *domain.Message, _ map[int64]bool) int { return 0 }

func (h *recordingStreamHub) IsOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}

func (h *recordingStreamHub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	count := 0
	for _, clients := range h.clients {
		count += len(clients)
	}
	return count
}

func (h *recordingStreamHub) DisconnectUser(userID int64) bool {
	h.mu.RLock()
	clients := h.clients[userID]
	if len(clients) == 0 {
		h.mu.RUnlock()
		return false
	}
	clientList := make([]*domain.Client, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	h.mu.RUnlock()
	for _, client := range clientList {
		_ = client.Conn.Close()
	}
	return true
}

func (h *recordingStreamHub) SubscribePresence(client *domain.Client, peer domain.PresenceKey) {
	h.subscribed <- streamHubPresence{client: client, peer: peer}
}

func (h *recordingStreamHub) UnsubscribePresence(client *domain.Client, peer domain.PresenceKey) {
	h.unsubscribed <- streamHubPresence{client: client, peer: peer}
}

func waitStreamHubMessage(t *testing.T, hub *recordingStreamHub) streamHubMessage {
	t.Helper()
	select {
	case got := <-hub.sent:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Hub.SendToUser")
		return streamHubMessage{}
	}
}

func TestGRPC_ChatStream_RoutesReceiveOnlyFrames(t *testing.T) {
	hub := newRecordingStreamHub()
	client, cleanup := newStreamTestClient(t, hub, nil, "", "")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := client.ChatStream(ctx)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if err := stream.Send(openRequest(42, 1)); err != nil {
		t.Fatalf("send open: %v", err)
	}
	select {
	case registered := <-hub.registered:
		if registered.UserID != 42 || registered.LoginRole != 1 {
			t.Fatalf("registered identity = %d/%d, want 42/1", registered.UserID, registered.LoginRole)
		}
	case <-time.After(time.Second):
		t.Fatal("stream client was not registered")
	}

	sendFrame := func(frame *pb.ChatStreamClientFrame) {
		t.Helper()
		if err := stream.Send(&pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Frame{Frame: frame}}); err != nil {
			t.Fatalf("send %s: %v", frame.GetType(), err)
		}
	}

	sendFrame(&pb.ChatStreamClientFrame{Type: "typing", ToUserId: "200"})
	got := waitStreamHubMessage(t, hub)
	if got.userID != 200 || got.message.Type != "typing" || got.message.From != 42 || got.message.To != 200 {
		t.Fatalf("typing relay = %+v, want from=42 to=200", got)
	}

	// The processor owns a per-connection rate window; a second typing frame
	// must not fan out immediately.
	sendFrame(&pb.ChatStreamClientFrame{Type: "typing", ToUserId: "200"})
	select {
	case got := <-hub.sent:
		t.Fatalf("rate-limited typing was relayed: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	sendFrame(&pb.ChatStreamClientFrame{Type: "read_receipt", ToUserId: "200", RoomId: "b42_s200"})
	got = waitStreamHubMessage(t, hub)
	if got.message.Type != "read_receipt" || got.message.From != 42 || got.message.To != 200 {
		t.Fatalf("read receipt relay = %+v, want from=42 to=200", got)
	}

	sendFrame(&pb.ChatStreamClientFrame{Type: "presence_sub", ToUserId: "200", PeerRole: 2})
	select {
	case got := <-hub.subscribed:
		if got.client.UserID != 42 || got.peer != (domain.PresenceKey{UserID: 200, LoginRole: 2}) {
			t.Fatalf("presence subscription = %+v, want client=42 peer=200/2", got)
		}
	case <-time.After(time.Second):
		t.Fatal("presence_sub was not routed")
	}

	sendFrame(&pb.ChatStreamClientFrame{Type: "presence_unsub", ToUserId: "200", PeerRole: 2})
	select {
	case got := <-hub.unsubscribed:
		if got.client.UserID != 42 || got.peer != (domain.PresenceKey{UserID: 200, LoginRole: 2}) {
			t.Fatalf("presence unsubscription = %+v, want client=42 peer=200/2", got)
		}
	case <-time.After(time.Second):
		t.Fatal("presence_unsub was not routed")
	}

	// Both inbound chat and unknown types remain receive-only drops.
	sendFrame(&pb.ChatStreamClientFrame{Type: "chat", ToUserId: "200", Content: "must not persist"})
	sendFrame(&pb.ChatStreamClientFrame{Type: "unknown", ToUserId: "200"})
	select {
	case got := <-hub.sent:
		t.Fatalf("disallowed frame was relayed: %+v", got)
	case <-time.After(50 * time.Millisecond):
	}

	_ = stream.CloseSend()
	select {
	case <-hub.unregistered:
	case <-time.After(time.Second):
		t.Fatal("stream client was not unregistered after CloseSend")
	}
}

func TestGRPC_ChatStream_SyncPreservesCursorAndOutboundIDs(t *testing.T) {
	hub := newRecordingStreamHub()
	const roomID = "b42_s200"
	repo := &stubMsgRepo{
		listAfterSeqFn: func(_ context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
			if arg.RoomID != roomID || arg.SeqID != 7459123456789012345 {
				t.Fatalf("sync query = %+v, want room/cursor exact", arg)
			}
			createdAt := time.UnixMilli(1800000000000)
			return []gen.Message{{
				ID: 7459123456789012346, RoomID: roomID, SeqID: 7459123456789012346,
				SenderID: 200, ReceiverID: 42, CreatedAt: &createdAt,
			}}, nil
		},
	}
	msgUC := usecase.NewMessageUseCase(repo, &stubConvRepo{}, hub, nil, nil, nil, nil, nil, nil, nil)
	client, cleanup := newStreamTestClient(t, hub, msgUC, "", "")
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	stream, err := client.ChatStream(ctx)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if err := stream.Send(openRequest(42, 1)); err != nil {
		t.Fatalf("send open: %v", err)
	}
	select {
	case <-hub.registered:
	case <-time.After(time.Second):
		t.Fatal("stream client was not registered")
	}
	if err := stream.Send(&pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Frame{Frame: &pb.ChatStreamClientFrame{
		Type: "sync", Conversations: map[string]string{roomID: "7459123456789012345"},
	}}}); err != nil {
		t.Fatalf("send sync: %v", err)
	}

	first, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv synced chat: %v", err)
	}
	if first.GetFrame().GetType() != "chat" || first.GetFrame().GetMessageId() != "7459123456789012346" || first.GetFrame().GetSeqId() != "7459123456789012346" {
		t.Fatalf("synced frame = %+v, want exact string IDs", first.GetFrame())
	}
	second, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv sync_done: %v", err)
	}
	if second.GetFrame().GetType() != "sync_done" || second.GetFrame().GetRoomId() != roomID {
		t.Fatalf("sync_done = %+v, want room %q", second.GetFrame(), roomID)
	}

	_ = stream.CloseSend()
}

func TestGRPC_ChatStream_CancellationUnregistersClient(t *testing.T) {
	hub := newRecordingStreamHub()
	client, cleanup := newStreamTestClient(t, hub, nil, "", "")
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.ChatStream(ctx)
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if err := stream.Send(openRequest(42, 1)); err != nil {
		t.Fatalf("send open: %v", err)
	}
	select {
	case <-hub.registered:
	case <-time.After(time.Second):
		t.Fatal("stream client was not registered")
	}

	cancel()
	select {
	case <-hub.unregistered:
	case <-time.After(time.Second):
		t.Fatal("stream client was not unregistered after context cancellation")
	}
}

func openRequest(userID int64, loginRole int32) *pb.ChatStreamRequest {
	return &pb.ChatStreamRequest{Payload: &pb.ChatStreamRequest_Open{
		Open: &pb.ChatStreamOpen{UserId: strconv.FormatInt(userID, 10), LoginRole: loginRole},
	}}
}

var _ domain.Hub = (*recordingStreamHub)(nil)
