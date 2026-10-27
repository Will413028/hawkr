package grpc

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	pb "github.com/Will413028/hawkr/go-pkg/proto/im"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

// ==================== Mocks ====================

type stubHub struct{}

func (s *stubHub) Run()                                                  {}
func (s *stubHub) Register(client *domain.Client)                        {}
func (s *stubHub) Unregister(client *domain.Client)                      {}
func (s *stubHub) SendToUser(userID int64, message *domain.Message) bool { return true }
func (s *stubHub) SendToUserRole(userID int64, loginRole int32, message *domain.Message) bool {
	return true
}
func (s *stubHub) Broadcast(message []byte)                                            {}
func (s *stubHub) BroadcastExcept(message *domain.Message, exclude map[int64]bool) int { return 0 }
func (s *stubHub) IsOnline(userID int64) bool                                          { return false }
func (s *stubHub) ConnectionCount() int                                                { return 0 }
func (s *stubHub) DisconnectUser(userID int64) bool                                    { return false }
func (s *stubHub) SubscribePresence(client *domain.Client, peer domain.PresenceKey)    {}
func (s *stubHub) UnsubscribePresence(client *domain.Client, peer domain.PresenceKey)  {}

type stubMsgRepo struct {
	lastContent    string
	lastType       int32
	listAfterSeqFn func(context.Context, gen.ListMessagesAfterSeqParams) ([]gen.Message, error)
}

func (s *stubMsgRepo) Create(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	s.lastContent = arg.Content
	s.lastType = arg.Type
	createdAt := time.Now()
	return gen.Message{
		ID: 1, RoomID: arg.RoomID, SenderID: arg.SenderID,
		Type: arg.Type, Content: arg.Content, SeqID: 1,
		CreatedAt: &createdAt,
	}, nil
}
func (s *stubMsgRepo) GetByID(ctx context.Context, id int64) (gen.Message, error) {
	return gen.Message{}, nil
}
func (s *stubMsgRepo) List(ctx context.Context, arg gen.ListMessagesParams) ([]gen.Message, error) {
	return nil, nil
}
func (s *stubMsgRepo) ListBefore(ctx context.Context, arg gen.ListMessagesBeforeParams) ([]gen.Message, error) {
	return nil, nil
}
func (s *stubMsgRepo) MarkAsRead(ctx context.Context, arg gen.MarkMessagesAsReadParams) error {
	return nil
}
func (s *stubMsgRepo) MarkByIDsAsRead(ctx context.Context, receiverID int64, ids []int64) error {
	return nil
}
func (s *stubMsgRepo) RecalcUnreadForMessages(ctx context.Context, messageIDs []int64) error {
	return nil
}
func (s *stubMsgRepo) MarkAllAsReadByBuyer(ctx context.Context, receiverID int64) error  { return nil }
func (s *stubMsgRepo) MarkAllAsReadBySeller(ctx context.Context, receiverID int64) error { return nil }
func (s *stubMsgRepo) ListAfterSeq(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
	if s.listAfterSeqFn != nil {
		return s.listAfterSeqFn(ctx, arg)
	}
	return nil, nil
}
func (s *stubMsgRepo) CreateWithLock(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	return s.Create(ctx, arg)
}
func (s *stubMsgRepo) HideMessage(_ context.Context, _ int64, _ string) error     { return nil }
func (s *stubMsgRepo) RedactBySender(_ context.Context, _ []int64) (int64, error) { return 0, nil }
func (s *stubMsgRepo) ListImageMessageIDsBySender(_ context.Context, _ []int64) ([]int64, error) {
	return nil, nil
}

type stubConvRepo struct{}

func (s *stubConvRepo) GetByRoomID(ctx context.Context, roomID string) (gen.Conversation, error) {
	return gen.Conversation{}, pgx.ErrNoRows
}
func (s *stubConvRepo) ListByBuyer(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
	return nil, nil
}
func (s *stubConvRepo) ListBySeller(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
	return nil, nil
}
func (s *stubConvRepo) Upsert(ctx context.Context, arg gen.UpsertConversationParams) (gen.Conversation, error) {
	return gen.Conversation{RoomID: arg.RoomID}, nil
}
func (s *stubConvRepo) IncrementBuyerUnread(ctx context.Context, roomID string) error  { return nil }
func (s *stubConvRepo) IncrementSellerUnread(ctx context.Context, roomID string) error { return nil }
func (s *stubConvRepo) ResetBuyerUnread(ctx context.Context, roomID string) error      { return nil }
func (s *stubConvRepo) ResetSellerUnread(ctx context.Context, roomID string) error     { return nil }
func (s *stubConvRepo) ResetAllBuyerUnread(ctx context.Context, id int64) error        { return nil }
func (s *stubConvRepo) ResetAllSellerUnread(ctx context.Context, id int64) error       { return nil }
func (s *stubConvRepo) SumBuyerUnreadCount(ctx context.Context, id int64) (int64, error) {
	return 0, nil
}
func (s *stubConvRepo) SumSellerUnreadCount(ctx context.Context, id int64) (int64, error) {
	return 0, nil
}
func (s *stubConvRepo) RedactLastMessageBySender(_ context.Context, _ []int64, _ string) error {
	return nil
}
func (s *stubConvRepo) BindOrder(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

type stubMediaClient struct {
	failWith error
	called   bool
}

func (s *stubMediaClient) FinalizeAndLink(_ context.Context, _, _, _ string) (usecase.AssetInfo, error) {
	s.called = true
	if s.failWith != nil {
		return usecase.AssetInfo{}, s.failWith
	}
	return usecase.AssetInfo{
		AssetId:   "99001",
		ObjectKey: "chat/2026/03/test.jpg",
	}, nil
}

func (s *stubMediaClient) ReleaseChatReferences(_ context.Context, _ []string) error {
	return nil
}

// ==================== Helper ====================

func newTestServer(t *testing.T, msgRepo *stubMsgRepo, media *stubMediaClient) (pb.IMServiceClient, func()) {
	t.Helper()

	cfg := &config.Config{MediaBaseURL: "https://test-media.example.com"}
	uc := usecase.NewMessageUseCase(msgRepo, &stubConvRepo{}, &stubHub{}, nil, nil, nil, nil, media, nil, cfg)

	srv := NewIMGrpcServer(&stubHub{}, uc)

	lis := bufconn.Listen(1024 * 1024)
	grpcSrv := grpc.NewServer()
	pb.RegisterIMServiceServer(grpcSrv, srv)
	go func() { _ = grpcSrv.Serve(lis) }()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	return pb.NewIMServiceClient(conn), func() {
		_ = conn.Close()
		grpcSrv.Stop()
	}
}

// ==================== Tests ====================

func TestGRPC_SendMessage_Image(t *testing.T) {
	repo := &stubMsgRepo{}
	media := &stubMediaClient{}
	client, cleanup := newTestServer(t, repo, media)
	defer cleanup()

	resp, err := client.SendMessage(context.Background(), &pb.SendMessageRequest{
		FromUserId: "42",
		ToUserId:   "100",
		Type:       pb.MessageType_MESSAGE_TYPE_IMAGE,
		Content:    "stk_abc123",
	})
	if err != nil {
		t.Fatalf("SendMessage IMAGE error: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("resp.Code = %d, want 0", resp.Code)
	}
	if !media.called {
		t.Error("MediaClient.FinalizeAndLink should be called for IMAGE messages")
	}
	// 49 §6.6 im/S4：DB 存 objectKey，回應直送裸 key（顯示 URL 由 Gateway 組）。
	wantKey := "chat/2026/03/test.jpg"
	if repo.lastContent != wantKey {
		t.Errorf("saved content = %q, want persisted objectKey %q", repo.lastContent, wantKey)
	}
	if resp.Content != wantKey {
		t.Errorf("resp.Content = %q, want bare objectKey %q", resp.Content, wantKey)
	}
}

// TestGRPC_SendMessage_SelfMessage_EnvelopeBizCode9201 確認自傳訊息時 gRPC handler
// 用 envelope 回傳業務碼 9201，而非 gRPC error（EnvelopeGuardInterceptor 轉換前的原始行為）。
func TestGRPC_SendMessage_SelfMessage_EnvelopeBizCode9201(t *testing.T) {
	repo := &stubMsgRepo{}
	media := &stubMediaClient{}
	client, cleanup := newTestServer(t, repo, media)
	defer cleanup()

	resp, err := client.SendMessage(context.Background(), &pb.SendMessageRequest{
		FromUserId:  "42",
		ToUserId:    "42", // same as FromUserId → 不能傳訊息給自己
		AccountType: "1",
		Type:        pb.MessageType_MESSAGE_TYPE_TEXT,
		Content:     "hi",
	})
	if err != nil {
		t.Fatalf("expected envelope response, got gRPC error: %v", err)
	}
	if resp.Code != 9201 {
		t.Errorf("resp.Code = %d, want 9201 (SelfMessageNotAllowed)", resp.Code)
	}
	if resp.Message != "不能傳訊息給自己" {
		t.Errorf("resp.Message = %q, want 不能傳訊息給自己", resp.Message)
	}
}

// TestGRPC_SendMessage_SellerSuspendedMetadata_WiredToUsecase covers
// server.go:33-36 / :72-73 — the transport boundary that reads the forwarded
// `x-seller-suspended` gRPC metadata (interceptor.SellerSuspendedFromContext,
// key "x-seller-suspended") and threads it into
// usecase.WithSellerSuspended(...). Drives the real wire (bufconn) with the
// metadata set/unset on the outgoing client context — a fake/mocked usecase
// would only prove the handler *calls* an option constructor, not that the
// value observed by SendMessage genuinely originates from incoming gRPC
// metadata (14 號 §6.5.4). orderClient is nil in newTestServer, so an unbound
// conversation with no order_no hint can never resolve a binding
// (resolveOrderBinding's E14 nil-guard) — the only variable between the two
// subtests is whether the metadata was set, isolating this exact wiring.
func TestGRPC_SendMessage_SellerSuspendedMetadata_WiredToUsecase(t *testing.T) {
	const sellerID = int64(500)
	const buyerID = int64(300)

	t.Run("metadata set -> usecase sees sellerSuspended=true -> blocked 9240", func(t *testing.T) {
		repo := &stubMsgRepo{}
		media := &stubMediaClient{}
		client, cleanup := newTestServer(t, repo, media)
		defer cleanup()

		ctx := metadata.AppendToOutgoingContext(context.Background(), "x-seller-suspended", "ADMIN_SUSPENSION")
		resp, err := client.SendMessage(ctx, &pb.SendMessageRequest{
			FromUserId:  strconv.FormatInt(sellerID, 10),
			ToUserId:    strconv.FormatInt(buyerID, 10),
			AccountType: "2", // SELLER — sender is the suspended seller side of the room
			Type:        pb.MessageType_MESSAGE_TYPE_TEXT,
			Content:     "hi",
		})
		if err != nil {
			t.Fatalf("expected envelope response, got gRPC error: %v", err)
		}
		if resp.Code != 9240 {
			t.Errorf("resp.Code = %d, want 9240 (SellerSuspendedOperationBlocked) — metadata not wired to usecase", resp.Code)
		}
	})

	t.Run("metadata absent -> usecase sees sellerSuspended=false -> allowed", func(t *testing.T) {
		repo := &stubMsgRepo{}
		media := &stubMediaClient{}
		client, cleanup := newTestServer(t, repo, media)
		defer cleanup()

		resp, err := client.SendMessage(context.Background(), &pb.SendMessageRequest{
			FromUserId:  strconv.FormatInt(sellerID, 10),
			ToUserId:    strconv.FormatInt(buyerID, 10),
			AccountType: "2",
			Type:        pb.MessageType_MESSAGE_TYPE_TEXT,
			Content:     "hi",
		})
		if err != nil {
			t.Fatalf("SendMessage error: %v", err)
		}
		if resp.Code != 0 {
			t.Errorf("resp.Code = %d, want 0 — absent metadata must fail-open (not default to suspended)", resp.Code)
		}
	})
}

func TestGRPC_SendMessage_Image_FinalizeFail(t *testing.T) {
	repo := &stubMsgRepo{}
	media := &stubMediaClient{failWith: fmt.Errorf("bad token")}
	client, cleanup := newTestServer(t, repo, media)
	defer cleanup()

	_, err := client.SendMessage(context.Background(), &pb.SendMessageRequest{
		FromUserId: "42",
		ToUserId:   "100",
		Type:       pb.MessageType_MESSAGE_TYPE_IMAGE,
		Content:    "bad_token",
	})
	if err == nil {
		t.Fatal("SendMessage IMAGE should fail when FinalizeAndLink fails")
	}
}
