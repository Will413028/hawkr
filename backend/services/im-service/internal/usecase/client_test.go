package usecase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
)

func TestProcessClientFrame_PreservesReceiveOnlyWhitelist(t *testing.T) {
	client := &domain.Client{UserID: 1001, LoginRole: accountTypeBuyer, Send: make(chan []byte, 4)}
	hub := &mockHub{}

	ProcessClientFrame(context.Background(), client, hub, nil, ClientFrame{
		Message: domain.Message{Type: "ping"},
	})
	if got := drainSend(client); len(got) != 1 || got[0].Type != "pong" {
		t.Fatalf("ping result = %+v, want one connection-local pong", got)
	}

	ProcessClientFrame(context.Background(), client, hub, nil, ClientFrame{
		Message: domain.Message{Type: "chat", Content: "must never persist"},
	})
	if got := drainSend(client); len(got) != 0 {
		t.Fatalf("inbound chat produced frames = %+v, want none", got)
	}
}

func TestProcessClientFrame_DropsPongWhenQueueIsFull(t *testing.T) {
	client := &domain.Client{UserID: 1001, LoginRole: accountTypeBuyer, Send: make(chan []byte, 1)}
	client.Send <- []byte(`{"type":"already_queued"}`)

	done := make(chan struct{})
	go func() {
		ProcessClientFrame(context.Background(), client, &mockHub{}, nil, ClientFrame{
			Message: domain.Message{Type: "ping"},
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("full connection queue blocked frame processing")
	}
	if got := len(client.Send); got != 1 {
		t.Fatalf("queue length = %d, want existing item only after dropping pong", got)
	}
}

// #2499: unauthorized rooms must never reach the repository, and the response
// must be indistinguishable from "authorized but no new messages".
func TestHandleSync_UnauthorizedRoom_NoQueryAndIndistinguishableFromEmpty(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	client := newTestClient(1001, accountTypeBuyer)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	queried := false
	msgRepo := &mockMsgRepo{
		ListAfterSeqFn: func(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
			queried = true
			return nil, nil
		},
	}
	uc := &MessageUseCase{msgRepo: msgRepo}

	victimRoom := BuildRoomID(9999, 8888) // neither ID is this client's

	handleSyncCursors(context.Background(), client, hub, uc, map[string]int64{victimRoom: 0})

	if queried {
		t.Fatal("#2499: unauthorized room must never reach the repository")
	}

	msgs := drainSend(client)
	if len(msgs) != 1 || msgs[0].Type != "sync_done" || msgs[0].HasMore {
		t.Fatalf("expected exactly one indistinguishable sync_done (empty, hasMore=false), got %+v", msgs)
	}
}

func TestHandleSync_AuthorizedRoom_QueriesAndForwardsMessages(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	client := newTestClient(1001, accountTypeBuyer)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	room := BuildRoomID(1001, 2002)
	msgRepo := &mockMsgRepo{
		ListAfterSeqFn: func(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
			if arg.RoomID != room {
				t.Fatalf("unexpected roomID queried: %q", arg.RoomID)
			}
			return []gen.Message{{ID: 1, RoomID: room, SeqID: 1, SenderID: 2002, ReceiverID: 1001}}, nil
		},
	}
	uc := &MessageUseCase{msgRepo: msgRepo}

	handleSyncCursors(context.Background(), client, hub, uc, map[string]int64{room: 0})

	msgs := drainSend(client)
	if len(msgs) != 2 {
		t.Fatalf("expected 1 chat message + 1 sync_done, got %d: %+v", len(msgs), msgs)
	}
	if msgs[0].Type != "chat" || msgs[1].Type != "sync_done" {
		t.Fatalf("unexpected message sequence: %+v", msgs)
	}
}

func TestHandleSync_ExceedsRoomCap_DropsWholeFrame(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()
	client := newTestClient(1001, accountTypeBuyer)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	queried := false
	msgRepo := &mockMsgRepo{
		ListAfterSeqFn: func(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
			queried = true
			return nil, nil
		},
	}
	uc := &MessageUseCase{msgRepo: msgRepo}

	convs := make(map[string]int64, maxSyncRoomsPerFrame+1)
	for i := 0; i <= maxSyncRoomsPerFrame; i++ {
		convs[fmt.Sprintf("b1001_s%d", i)] = 0
	}
	handleSyncCursors(context.Background(), client, hub, uc, convs)

	if queried {
		t.Fatal("#2499: a frame exceeding the room cap must be dropped before any repository call")
	}
	if msgs := drainSend(client); len(msgs) != 0 {
		t.Fatalf("expected no response for a dropped over-cap frame, got %+v", msgs)
	}
}

func TestHandleSync_RateLimited_DropsFrame(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()
	client := newTestClient(1001, accountTypeBuyer)
	hub.Register(client)
	time.Sleep(10 * time.Millisecond)

	queried := 0
	msgRepo := &mockMsgRepo{
		ListAfterSeqFn: func(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
			queried++
			return nil, nil
		},
	}
	room := BuildRoomID(1001, 2002)
	uc := &MessageUseCase{msgRepo: msgRepo, syncLimiter: ratelimit.New(nil, 0, nil)}
	for i := 0; i < syncRateLimit; i++ {
		handleSyncCursors(context.Background(), client, hub, uc, map[string]int64{room: 0})
	}
	if queried != syncRateLimit {
		t.Fatalf("expected %d queries within the rate limit, got %d", syncRateLimit, queried)
	}

	handleSyncCursors(context.Background(), client, hub, uc, map[string]int64{room: 0}) // one over the limit
	if queried != syncRateLimit {
		t.Fatalf("#2499: one frame over the rate limit must not reach the repository, queried=%d", queried)
	}
}
