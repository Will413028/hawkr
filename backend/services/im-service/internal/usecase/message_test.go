package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	bizcode "github.com/Will413028/hawkr/go-pkg/errors/bizcode"
	mediaRef "github.com/Will413028/hawkr/go-pkg/media"
	"github.com/Will413028/hawkr/go-pkg/ratelimit"
	"github.com/Will413028/hawkr/im-service/internal/domain"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

// ==================== Mock Clients ====================

type mockBuyerClient struct {
	GetProfileBatchFn  func(ctx context.Context, buyerIDs []int64) (map[int64]BuyerProfileInfo, error)
	ResolvePrincipalFn func(ctx context.Context, buyerID int64) int64
	// ResolvePrincipalStrictFn overrides the default three-way answer. Leave it
	// nil to model the grandfathered baseline (see ResolvePrincipalStrict
	// below); set it to exercise "not mine" (Found=false) or "cannot answer"
	// (error), the two states #2789's fail-closed guard keys on.
	ResolvePrincipalStrictFn func(ctx context.Context, buyerID int64) (PrincipalResolution, error)
	ListIDsByAccountFn       func(ctx context.Context, accountID int64) ([]int64, error)
}

func (m *mockBuyerClient) GetProfileBatch(ctx context.Context, buyerIDs []int64) (map[int64]BuyerProfileInfo, error) {
	if m.GetProfileBatchFn != nil {
		return m.GetProfileBatchFn(ctx, buyerIDs)
	}
	return nil, nil
}

func (m *mockBuyerClient) ResolvePrincipal(ctx context.Context, buyerID int64) int64 {
	if m.ResolvePrincipalFn != nil {
		return m.ResolvePrincipalFn(ctx, buyerID)
	}
	return buyerID
}

// ResolvePrincipalStrict defaults to the grandfathered production baseline:
// every buyer_id is live and resolves (Found=true) to whatever
// ResolvePrincipalFn says — identity when that is nil. It deliberately does NOT
// derive Found from "the value changed": under grandfather buyer_id ==
// account_id, so a real resolution and a non-resolution are numerically
// identical, and inferring Found from equality would model every undiverged id
// as unresolved.
func (m *mockBuyerClient) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (PrincipalResolution, error) {
	if m.ResolvePrincipalStrictFn != nil {
		return m.ResolvePrincipalStrictFn(ctx, buyerID)
	}
	return PrincipalResolution{AccountID: m.ResolvePrincipal(ctx, buyerID), Found: true}, nil
}

func (m *mockBuyerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if m.ListIDsByAccountFn != nil {
		return m.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

type mockSellerClient struct {
	GetProfileBatchFn  func(ctx context.Context, storeIDs []int64) (map[int64]SellerProfileInfo, error)
	ResolvePrincipalFn func(ctx context.Context, storeID int64) int64
	// ResolvePrincipalStrictFn: see mockBuyerClient's identically named field.
	ResolvePrincipalStrictFn func(ctx context.Context, storeID int64) (PrincipalResolution, error)
	ListIDsByAccountFn       func(ctx context.Context, accountID int64) ([]int64, error)
}

func (m *mockSellerClient) GetProfileBatch(ctx context.Context, storeIDs []int64) (map[int64]SellerProfileInfo, error) {
	if m.GetProfileBatchFn != nil {
		return m.GetProfileBatchFn(ctx, storeIDs)
	}
	return nil, nil
}

func (m *mockSellerClient) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	if m.ResolvePrincipalFn != nil {
		return m.ResolvePrincipalFn(ctx, storeID)
	}
	return storeID
}

// ResolvePrincipalStrict mirrors mockBuyerClient's; see that comment.
func (m *mockSellerClient) ResolvePrincipalStrict(ctx context.Context, storeID int64) (PrincipalResolution, error) {
	if m.ResolvePrincipalStrictFn != nil {
		return m.ResolvePrincipalStrictFn(ctx, storeID)
	}
	return PrincipalResolution{AccountID: m.ResolvePrincipal(ctx, storeID), Found: true}, nil
}

func (m *mockSellerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if m.ListIDsByAccountFn != nil {
		return m.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

type mockMediaClient struct {
	FinalizeAndLinkFn       func(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error)
	ReleaseChatReferencesFn func(ctx context.Context, refIDs []string) error
}

func (m *mockMediaClient) FinalizeAndLink(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error) {
	if m.FinalizeAndLinkFn != nil {
		return m.FinalizeAndLinkFn(ctx, stagedToken, refType, refID)
	}
	return AssetInfo{}, nil
}

func (m *mockMediaClient) ReleaseChatReferences(ctx context.Context, refIDs []string) error {
	if m.ReleaseChatReferencesFn != nil {
		return m.ReleaseChatReferencesFn(ctx, refIDs)
	}
	return nil
}

type mockHub struct{}

func (m *mockHub) Run()                                                  {}
func (m *mockHub) Register(client *domain.Client)                        {}
func (m *mockHub) Unregister(client *domain.Client)                      {}
func (m *mockHub) SendToUser(userID int64, message *domain.Message) bool { return true }
func (m *mockHub) SendToUserRole(userID int64, loginRole int32, message *domain.Message) bool {
	return true
}
func (m *mockHub) Broadcast(message []byte)                                            {}
func (m *mockHub) BroadcastExcept(message *domain.Message, exclude map[int64]bool) int { return 0 }
func (m *mockHub) IsOnline(userID int64) bool                                          { return false }
func (m *mockHub) ConnectionCount() int                                                { return 0 }
func (m *mockHub) DisconnectUser(userID int64) bool                                    { return false }
func (m *mockHub) SubscribePresence(client *domain.Client, peer domain.PresenceKey)    {}
func (m *mockHub) UnsubscribePresence(client *domain.Client, peer domain.PresenceKey)  {}

type capturingHub struct {
	captured   *domain.Message   // last message sent (backward compat)
	messages   []*domain.Message // all messages, in order
	lastUserID int64
	lastRole   int32
}

func (h *capturingHub) Run()                                                  {}
func (h *capturingHub) Register(client *domain.Client)                        {}
func (h *capturingHub) Unregister(client *domain.Client)                      {}
func (h *capturingHub) SendToUser(userID int64, message *domain.Message) bool { return true }
func (h *capturingHub) SendToUserRole(userID int64, loginRole int32, message *domain.Message) bool {
	h.captured = message
	h.messages = append(h.messages, message)
	h.lastUserID = userID
	h.lastRole = loginRole
	return true
}
func (h *capturingHub) Broadcast(message []byte)                                            {}
func (h *capturingHub) BroadcastExcept(message *domain.Message, exclude map[int64]bool) int { return 0 }
func (h *capturingHub) IsOnline(userID int64) bool                                          { return false }
func (h *capturingHub) ConnectionCount() int                                                { return 0 }
func (h *capturingHub) DisconnectUser(userID int64) bool                                    { return false }
func (h *capturingHub) SubscribePresence(client *domain.Client, peer domain.PresenceKey)    {}
func (h *capturingHub) UnsubscribePresence(client *domain.Client, peer domain.PresenceKey)  {}

// lastOfType returns the most recent captured message with the given type, or nil.
func (h *capturingHub) lastOfType(t string) *domain.Message {
	for i := len(h.messages) - 1; i >= 0; i-- {
		if h.messages[i].Type == t {
			return h.messages[i]
		}
	}
	return nil
}

type capturingChatPublisher struct {
	mu   sync.Mutex
	last *ChatMessageSentEvent
}

func (c *capturingChatPublisher) PublishChatMessageSent(_ context.Context, e ChatMessageSentEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last = &e
}

func (c *capturingChatPublisher) get() *ChatMessageSentEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

type mockMsgRepo struct {
	CreateFn                      func(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error)
	MarkAllAsReadByBuyerFn        func(ctx context.Context, receiverID int64) error
	MarkAllAsReadBySellerFn       func(ctx context.Context, receiverID int64) error
	ListAfterSeqFn                func(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error)
	RedactBySenderFn              func(ctx context.Context, senderIDs []int64) (int64, error)
	ListImageMessageIDsBySenderFn func(ctx context.Context, senderIDs []int64) ([]int64, error)
}

func (m *mockMsgRepo) Create(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, arg)
	}
	return gen.Message{ID: 1, RoomID: arg.RoomID, SenderID: arg.SenderID, Type: arg.Type, Content: arg.Content, SeqID: 1}, nil
}
func (m *mockMsgRepo) CreateWithLock(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	return m.Create(ctx, arg)
}
func (m *mockMsgRepo) GetByID(ctx context.Context, id int64) (gen.Message, error) {
	return gen.Message{}, nil
}
func (m *mockMsgRepo) List(ctx context.Context, arg gen.ListMessagesParams) ([]gen.Message, error) {
	return nil, nil
}
func (m *mockMsgRepo) ListBefore(ctx context.Context, arg gen.ListMessagesBeforeParams) ([]gen.Message, error) {
	return nil, nil
}
func (m *mockMsgRepo) MarkAsRead(ctx context.Context, arg gen.MarkMessagesAsReadParams) error {
	return nil
}
func (m *mockMsgRepo) MarkByIDsAsRead(ctx context.Context, receiverID int64, ids []int64) error {
	return nil
}
func (m *mockMsgRepo) MarkAllAsReadByBuyer(ctx context.Context, receiverID int64) error {
	if m.MarkAllAsReadByBuyerFn != nil {
		return m.MarkAllAsReadByBuyerFn(ctx, receiverID)
	}
	return nil
}
func (m *mockMsgRepo) MarkAllAsReadBySeller(ctx context.Context, receiverID int64) error {
	if m.MarkAllAsReadBySellerFn != nil {
		return m.MarkAllAsReadBySellerFn(ctx, receiverID)
	}
	return nil
}
func (m *mockMsgRepo) RecalcUnreadForMessages(ctx context.Context, messageIDs []int64) error {
	return nil
}
func (m *mockMsgRepo) ListAfterSeq(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
	if m.ListAfterSeqFn != nil {
		return m.ListAfterSeqFn(ctx, arg)
	}
	return nil, nil
}
func (m *mockMsgRepo) HideMessage(_ context.Context, _ int64, _ string) error { return nil }
func (m *mockMsgRepo) RedactBySender(ctx context.Context, senderIDs []int64) (int64, error) {
	if m.RedactBySenderFn != nil {
		return m.RedactBySenderFn(ctx, senderIDs)
	}
	return 0, nil
}
func (m *mockMsgRepo) ListImageMessageIDsBySender(ctx context.Context, senderIDs []int64) ([]int64, error) {
	if m.ListImageMessageIDsBySenderFn != nil {
		return m.ListImageMessageIDsBySenderFn(ctx, senderIDs)
	}
	return nil, nil
}

type mockConvRepo struct {
	GetByRoomIDFn               func(ctx context.Context, roomID string) (gen.Conversation, error)
	ListByBuyerFn               func(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error)
	ListBySellerFn              func(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error)
	UpsertFn                    func(ctx context.Context, arg gen.UpsertConversationParams) (gen.Conversation, error)
	IncrementBuyerUnreadFn      func(ctx context.Context, roomID string) error
	IncrementSellerUnreadFn     func(ctx context.Context, roomID string) error
	ResetBuyerUnreadFn          func(ctx context.Context, roomID string) error
	ResetSellerUnreadFn         func(ctx context.Context, roomID string) error
	ResetAllBuyerUnreadFn       func(ctx context.Context, buyerID int64) error
	ResetAllSellerUnreadFn      func(ctx context.Context, storeID int64) error
	SumBuyerUnreadCountFn       func(ctx context.Context, buyerID int64) (int64, error)
	SumSellerUnreadCountFn      func(ctx context.Context, storeID int64) (int64, error)
	RedactLastMessageBySenderFn func(ctx context.Context, senderIDs []int64, placeholder string) error
	BindOrderFn                 func(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error
}

func (m *mockConvRepo) GetByRoomID(ctx context.Context, roomID string) (gen.Conversation, error) {
	if m.GetByRoomIDFn != nil {
		return m.GetByRoomIDFn(ctx, roomID)
	}
	return gen.Conversation{}, pgx.ErrNoRows
}
func (m *mockConvRepo) ListByBuyer(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
	if m.ListByBuyerFn != nil {
		return m.ListByBuyerFn(ctx, arg)
	}
	return nil, nil
}
func (m *mockConvRepo) ListBySeller(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
	if m.ListBySellerFn != nil {
		return m.ListBySellerFn(ctx, arg)
	}
	return nil, nil
}
func (m *mockConvRepo) Upsert(ctx context.Context, arg gen.UpsertConversationParams) (gen.Conversation, error) {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, arg)
	}
	return gen.Conversation{RoomID: arg.RoomID}, nil
}
func (m *mockConvRepo) IncrementBuyerUnread(ctx context.Context, roomID string) error {
	if m.IncrementBuyerUnreadFn != nil {
		return m.IncrementBuyerUnreadFn(ctx, roomID)
	}
	return nil
}
func (m *mockConvRepo) IncrementSellerUnread(ctx context.Context, roomID string) error {
	if m.IncrementSellerUnreadFn != nil {
		return m.IncrementSellerUnreadFn(ctx, roomID)
	}
	return nil
}
func (m *mockConvRepo) ResetBuyerUnread(ctx context.Context, roomID string) error {
	if m.ResetBuyerUnreadFn != nil {
		return m.ResetBuyerUnreadFn(ctx, roomID)
	}
	return nil
}
func (m *mockConvRepo) ResetSellerUnread(ctx context.Context, roomID string) error {
	if m.ResetSellerUnreadFn != nil {
		return m.ResetSellerUnreadFn(ctx, roomID)
	}
	return nil
}
func (m *mockConvRepo) ResetAllBuyerUnread(ctx context.Context, buyerID int64) error {
	if m.ResetAllBuyerUnreadFn != nil {
		return m.ResetAllBuyerUnreadFn(ctx, buyerID)
	}
	return nil
}
func (m *mockConvRepo) ResetAllSellerUnread(ctx context.Context, storeID int64) error {
	if m.ResetAllSellerUnreadFn != nil {
		return m.ResetAllSellerUnreadFn(ctx, storeID)
	}
	return nil
}
func (m *mockConvRepo) SumBuyerUnreadCount(ctx context.Context, buyerID int64) (int64, error) {
	if m.SumBuyerUnreadCountFn != nil {
		return m.SumBuyerUnreadCountFn(ctx, buyerID)
	}
	return 0, nil
}
func (m *mockConvRepo) SumSellerUnreadCount(ctx context.Context, storeID int64) (int64, error) {
	if m.SumSellerUnreadCountFn != nil {
		return m.SumSellerUnreadCountFn(ctx, storeID)
	}
	return 0, nil
}
func (m *mockConvRepo) RedactLastMessageBySender(ctx context.Context, senderIDs []int64, placeholder string) error {
	if m.RedactLastMessageBySenderFn != nil {
		return m.RedactLastMessageBySenderFn(ctx, senderIDs, placeholder)
	}
	return nil
}
func (m *mockConvRepo) BindOrder(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error {
	if m.BindOrderFn != nil {
		return m.BindOrderFn(ctx, roomID, orderNo, verifiedAt)
	}
	return nil
}

// ==================== GetParticipants ====================

func TestGetParticipants_ReturnsSellerStoreInfo(t *testing.T) {
	buyerClient := &mockBuyerClient{
		GetProfileBatchFn: func(ctx context.Context, buyerIDs []int64) (map[int64]BuyerProfileInfo, error) {
			return map[int64]BuyerProfileInfo{
				42: {Nickname: "Alice", Avatar: "https://cdn/alice.jpg"},
			}, nil
		},
	}
	sellerClient := &mockSellerClient{
		GetProfileBatchFn: func(ctx context.Context, storeIDs []int64) (map[int64]SellerProfileInfo, error) {
			return map[int64]SellerProfileInfo{
				100: {
					Nickname:  "Cora",
					Avatar:    "https://cdn/avatar.jpg",
					StoreName: "Hawkr 有機農場",
					LogoKey:   "https://cdn/logo.jpg",
				},
			}, nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, buyerClient: buyerClient, sellerClient: sellerClient}
	participants := uc.GetParticipants(context.Background(), "b42_s100")

	if len(participants) != 2 {
		t.Fatalf("len(participants) = %d, want 2", len(participants))
	}

	// Buyer participant
	if participants[0].UserID != 42 {
		t.Errorf("participants[0].UserID = %d, want 42", participants[0].UserID)
	}
	if participants[0].Role != "buyer" {
		t.Errorf("participants[0].Role = %q, want buyer", participants[0].Role)
	}
	if participants[0].DisplayName != "Alice" {
		t.Errorf("participants[0].DisplayName = %q, want Alice", participants[0].DisplayName)
	}

	// Seller participant — should use StoreName and LogoKey
	if participants[1].UserID != 100 {
		t.Errorf("participants[1].UserID = %d, want 100", participants[1].UserID)
	}
	if participants[1].Role != "seller" {
		t.Errorf("participants[1].Role = %q, want seller", participants[1].Role)
	}
	if participants[1].DisplayName != "Hawkr 有機農場" {
		t.Errorf("participants[1].DisplayName = %q, want Hawkr 有機農場", participants[1].DisplayName)
	}
	if participants[1].AvatarKey != "https://cdn/logo.jpg" {
		t.Errorf("participants[1].AvatarKey = %q, want https://cdn/logo.jpg", participants[1].AvatarKey)
	}
}

func TestGetParticipants_EmptyStoreName_DisplayNameIsEmpty(t *testing.T) {
	sellerClient := &mockSellerClient{
		GetProfileBatchFn: func(ctx context.Context, storeIDs []int64) (map[int64]SellerProfileInfo, error) {
			return map[int64]SellerProfileInfo{
				100: {StoreName: "", LogoKey: "", Avatar: "https://cdn/avatar.jpg"},
			}, nil
		},
	}
	buyerClient := &mockBuyerClient{
		GetProfileBatchFn: func(ctx context.Context, buyerIDs []int64) (map[int64]BuyerProfileInfo, error) {
			return map[int64]BuyerProfileInfo{42: {Nickname: "Alice"}}, nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, buyerClient: buyerClient, sellerClient: sellerClient}
	participants := uc.GetParticipants(context.Background(), "b42_s100")

	seller := participants[1]
	if seller.DisplayName != "" {
		t.Errorf("DisplayName = %q, want empty (store_name should never be empty, no fallback)", seller.DisplayName)
	}
	if seller.AvatarKey != "https://cdn/avatar.jpg" {
		t.Errorf("AvatarKey = %q, want https://cdn/avatar.jpg", seller.AvatarKey)
	}
}

func TestGetParticipants_InvalidRoomID(t *testing.T) {
	uc := &MessageUseCase{genID: func() int64 { return 99999 }}
	participants := uc.GetParticipants(context.Background(), "invalid")

	if participants != nil {
		t.Errorf("participants = %v, want nil for invalid roomID", participants)
	}
}

func TestGetParticipants_ProfileNotFound(t *testing.T) {
	buyerClient := &mockBuyerClient{
		GetProfileBatchFn: func(ctx context.Context, buyerIDs []int64) (map[int64]BuyerProfileInfo, error) {
			return map[int64]BuyerProfileInfo{}, nil // empty — not found（明確區分於查詢失敗）
		},
	}
	sellerClient := &mockSellerClient{
		GetProfileBatchFn: func(ctx context.Context, storeIDs []int64) (map[int64]SellerProfileInfo, error) {
			return map[int64]SellerProfileInfo{}, nil // empty — not found（明確區分於查詢失敗）
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, buyerClient: buyerClient, sellerClient: sellerClient}
	participants := uc.GetParticipants(context.Background(), "b42_s100")

	if len(participants) != 2 {
		t.Fatalf("len(participants) = %d, want 2", len(participants))
	}
	// Should still have userId and role, but empty display info
	if participants[0].UserID != 42 || participants[0].Role != "buyer" {
		t.Errorf("buyer participant = %+v, want UserID=42 Role=buyer", participants[0])
	}
	if participants[0].DisplayName != "" {
		t.Errorf("buyer DisplayName = %q, want empty", participants[0].DisplayName)
	}
	if participants[1].UserID != 100 || participants[1].Role != "seller" {
		t.Errorf("seller participant = %+v, want UserID=100 Role=seller", participants[1])
	}
	if participants[1].DisplayName != "" {
		t.Errorf("seller DisplayName = %q, want empty", participants[1].DisplayName)
	}
}

// ==================== RedactDeletedAccount ====================

func TestRedactDeletedAccount_PurgesMessagesAndPreviews(t *testing.T) {
	var gotMsgSenders, gotConvSenders []int64
	var gotPlaceholder string
	msgRepo := &mockMsgRepo{
		RedactBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			gotMsgSenders = senderIDs
			return 3, nil
		},
	}
	convRepo := &mockConvRepo{
		RedactLastMessageBySenderFn: func(_ context.Context, senderIDs []int64, placeholder string) error {
			gotConvSenders = senderIDs
			gotPlaceholder = placeholder
			return nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		return []int64{777}, nil
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		return nil, nil
	}}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: convRepo, buyerClient: buyerClient, sellerClient: sellerClient, genID: func() int64 { return 1 }}

	if err := uc.RedactDeletedAccount(context.Background(), 777); err != nil {
		t.Fatalf("RedactDeletedAccount returned error: %v", err)
	}
	if len(gotMsgSenders) != 1 || gotMsgSenders[0] != 777 {
		t.Errorf("RedactBySender senderIDs = %v, want [777]", gotMsgSenders)
	}
	if len(gotConvSenders) != 1 || gotConvSenders[0] != 777 {
		t.Errorf("RedactLastMessageBySender senderIDs = %v, want [777]", gotConvSenders)
	}
	if gotPlaceholder != deletedMessagePlaceholder {
		t.Errorf("conversation placeholder = %q, want %q", gotPlaceholder, deletedMessagePlaceholder)
	}
}

func TestRedactDeletedAccount_EnumeratesAllHistoricalBuyerAndStoreIDs(t *testing.T) {
	var gotSenders []int64
	msgRepo := &mockMsgRepo{
		RedactBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			gotSenders = senderIDs
			return int64(len(senderIDs)), nil
		},
		ListImageMessageIDsBySenderFn: func(_ context.Context, _ []int64) ([]int64, error) { return nil, nil },
	}
	convRepo := &mockConvRepo{
		RedactLastMessageBySenderFn: func(_ context.Context, _ []int64, _ string) error { return nil },
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		if accountID != 777 {
			t.Fatalf("buyerClient.ListIDsByAccount accountID = %d, want 777", accountID)
		}
		return []int64{777, 888}, nil // closed stint + current stint
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, accountID int64) ([]int64, error) {
		if accountID != 777 {
			t.Fatalf("sellerClient.ListIDsByAccount accountID = %d, want 777", accountID)
		}
		return []int64{999}, nil // this account also has a diverged store_id
	}}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: convRepo, buyerClient: buyerClient, sellerClient: sellerClient, genID: func() int64 { return 1 }}

	if err := uc.RedactDeletedAccount(context.Background(), 777); err != nil {
		t.Fatalf("RedactDeletedAccount returned error: %v", err)
	}
	want := map[int64]bool{777: true, 888: true, 999: true}
	if len(gotSenders) != len(want) {
		t.Fatalf("RedactBySender senderIDs = %v, want exactly %v", gotSenders, want)
	}
	for _, id := range gotSenders {
		if !want[id] {
			t.Errorf("unexpected senderID %d in purge set", id)
		}
	}
}

// A deleted account's send-guard marker must be written for EVERY historical
// identity, not just the login account_id. SendMessage looks the marker up by
// receiverID — a buyer_id/store_id surrogate decoded from the roomID — so an
// accountID-only marker is a key that the read path never constructs once the
// two have diverged (identity separation, 2026-09-02). The bug was invisible on
// grandfathered accounts, where account_id == buyer_id makes the single marker
// happen to match.
func TestRedactDeletedAccount_MarksEveryHistoricalIdentity(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	msgRepo := &mockMsgRepo{
		RedactBySenderFn:              func(_ context.Context, s []int64) (int64, error) { return int64(len(s)), nil },
		ListImageMessageIDsBySenderFn: func(_ context.Context, _ []int64) ([]int64, error) { return nil, nil },
	}
	convRepo := &mockConvRepo{
		RedactLastMessageBySenderFn: func(_ context.Context, _ []int64, _ string) error { return nil },
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{888}, nil // diverged buyer surrogate
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{999}, nil // diverged store surrogate
	}}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: convRepo, buyerClient: buyerClient,
		sellerClient: sellerClient, rdb: rdb, genID: func() int64 { return 1 }}

	if err := uc.RedactDeletedAccount(context.Background(), 2600000012); err != nil {
		t.Fatalf("RedactDeletedAccount returned error: %v", err)
	}

	// The surrogates are the discriminating half: keying on accountID alone still
	// satisfies the 2600000012 assertion, so that one alone cannot fail this test.
	for _, id := range []int64{2600000012, 888, 999} {
		if n, err := rdb.Exists(context.Background(), deletedAccountKey(id)).Result(); err != nil || n != 1 {
			t.Errorf("deleted-account marker for identity %d: exists=%d err=%v, want exists=1", id, n, err)
		}
	}
	// Control: an identity never bound to this account must NOT be marked, otherwise
	// a blanket write would pass the assertions above without discriminating anything.
	if n, _ := rdb.Exists(context.Background(), deletedAccountKey(1234)).Result(); n != 0 {
		t.Errorf("unrelated identity 1234 was marked deleted (exists=%d), want 0", n)
	}
}

func TestRedactDeletedAccount_PropagatesBuyerClientEnumerationError(t *testing.T) {
	msgRepo := &mockMsgRepo{
		RedactBySenderFn: func(_ context.Context, _ []int64) (int64, error) {
			t.Fatal("RedactBySender must not be called when enumeration fails")
			return 0, nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return nil, fmt.Errorf("buyer-service unavailable")
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) { return nil, nil }}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: &mockConvRepo{}, buyerClient: buyerClient, sellerClient: sellerClient, genID: func() int64 { return 1 }}

	if err := uc.RedactDeletedAccount(context.Background(), 777); err == nil {
		t.Fatal("expected error when buyer-service enumeration fails, got nil")
	}
}

func TestRedactDeletedAccount_PropagatesSellerClientEnumerationError(t *testing.T) {
	msgRepo := &mockMsgRepo{
		RedactBySenderFn: func(_ context.Context, _ []int64) (int64, error) {
			t.Fatal("RedactBySender must not be called when enumeration fails")
			return 0, nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return []int64{777}, nil
	}}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) {
		return nil, fmt.Errorf("seller-service unavailable")
	}}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: &mockConvRepo{}, buyerClient: buyerClient, sellerClient: sellerClient, genID: func() int64 { return 1 }}

	if err := uc.RedactDeletedAccount(context.Background(), 777); err == nil {
		t.Fatal("expected error when seller-service enumeration fails, got nil")
	}
}

func TestRedactDeletedAccount_PropagatesMessageRepoError(t *testing.T) {
	convCalled := false
	msgRepo := &mockMsgRepo{
		RedactBySenderFn: func(_ context.Context, _ []int64) (int64, error) {
			return 0, fmt.Errorf("db down")
		},
	}
	convRepo := &mockConvRepo{
		RedactLastMessageBySenderFn: func(_ context.Context, _ []int64, _ string) error {
			convCalled = true
			return nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) { return []int64{777}, nil }}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) { return nil, nil }}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: convRepo, buyerClient: buyerClient, sellerClient: sellerClient, genID: func() int64 { return 1 }}

	// Must surface the error so the consumer requeues → DLQ (erasure never silently dropped).
	if err := uc.RedactDeletedAccount(context.Background(), 777); err == nil {
		t.Fatal("expected error when message redaction fails, got nil")
	}
	// Conversation preview purge must be skipped once the message purge failed.
	if convCalled {
		t.Error("RedactLastMessageBySender called despite message redaction failure")
	}
}

func TestRedactDeletedAccount_ReleasesChatImageReferences(t *testing.T) {
	var gotRefIDs []string
	msgRepo := &mockMsgRepo{
		RedactBySenderFn: func(_ context.Context, _ []int64) (int64, error) { return 2, nil },
		ListImageMessageIDsBySenderFn: func(_ context.Context, senderIDs []int64) ([]int64, error) {
			if len(senderIDs) != 1 || senderIDs[0] != 777 {
				t.Errorf("ListImageMessageIDsBySender senderIDs = %v, want [777]", senderIDs)
			}
			return []int64{101, 102}, nil
		},
	}
	media := &mockMediaClient{
		ReleaseChatReferencesFn: func(_ context.Context, refIDs []string) error {
			gotRefIDs = refIDs
			return nil
		},
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) { return []int64{777}, nil }}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) { return nil, nil }}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: &mockConvRepo{}, buyerClient: buyerClient, sellerClient: sellerClient, mediaClient: media, genID: func() int64 { return 1 }}

	if err := uc.RedactDeletedAccount(context.Background(), 777); err != nil {
		t.Fatalf("RedactDeletedAccount returned error: %v", err)
	}
	want := []string{"101", "102"}
	if len(gotRefIDs) != len(want) || gotRefIDs[0] != want[0] || gotRefIDs[1] != want[1] {
		t.Errorf("ReleaseChatReferences refIDs = %v, want %v", gotRefIDs, want)
	}
}

func TestRedactDeletedAccount_SkipsReleaseWhenNoImages(t *testing.T) {
	called := false
	msgRepo := &mockMsgRepo{
		RedactBySenderFn:              func(_ context.Context, _ []int64) (int64, error) { return 0, nil },
		ListImageMessageIDsBySenderFn: func(_ context.Context, _ []int64) ([]int64, error) { return nil, nil },
	}
	media := &mockMediaClient{
		ReleaseChatReferencesFn: func(_ context.Context, _ []string) error { called = true; return nil },
	}
	buyerClient := &mockBuyerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) { return []int64{777}, nil }}
	sellerClient := &mockSellerClient{ListIDsByAccountFn: func(_ context.Context, _ int64) ([]int64, error) { return nil, nil }}
	uc := &MessageUseCase{msgRepo: msgRepo, convRepo: &mockConvRepo{}, buyerClient: buyerClient, sellerClient: sellerClient, mediaClient: media, genID: func() int64 { return 1 }}

	if err := uc.RedactDeletedAccount(context.Background(), 777); err != nil {
		t.Fatalf("RedactDeletedAccount returned error: %v", err)
	}
	if called {
		t.Error("ReleaseChatReferences should not be called when there are no image messages")
	}
}

// ==================== SendMessage ====================

func newTestUC(mediaClient MediaClient, msgRepo domain.MessageRepository) *MessageUseCase {
	if msgRepo == nil {
		msgRepo = &mockMsgRepo{}
	}
	return &MessageUseCase{
		msgRepo:     msgRepo,
		convRepo:    &mockConvRepo{},
		hub:         &mockHub{},
		mediaClient: mediaClient,
		genID:       func() int64 { return 99999 },
	}
}

func TestSendMessage_RejectsOverLengthContent(t *testing.T) {
	created := false
	repo := &mockMsgRepo{
		CreateFn: func(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			created = true
			return gen.Message{
				ID: 1, RoomID: arg.RoomID, SenderID: arg.SenderID,
				Type: arg.Type, Content: arg.Content, SeqID: 1,
				CreatedAt: timePointer(time.Time{}),
			}, nil
		},
	}
	uc := newTestUC(&mockMediaClient{}, repo)

	// 4097 multi-byte runes: proves the guard counts characters (char_length
	// semantics), not bytes, and rejects before any persist.
	overLong := strings.Repeat("好", MaxMessageContentLength+1)
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, overLong)
	if err == nil {
		t.Fatal("expected error for over-length content, got nil")
	}
	if code := pkgerrors.GetCode(err); code != pkgerrors.CodeInvalidInput {
		t.Fatalf("want CodeInvalidInput (400), got %v (err=%v)", code, err)
	}
	if created {
		t.Fatal("over-length content must be rejected before persisting")
	}

	// Boundary: exactly the cap is accepted and persisted.
	atCap := strings.Repeat("好", MaxMessageContentLength)
	if _, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, atCap); err != nil {
		t.Fatalf("content at the cap should be accepted, got %v", err)
	}
	if !created {
		t.Fatal("at-cap content should have been persisted")
	}
}

func TestTombstoneContent(t *testing.T) {
	const original = "原文機密內容"
	hiddenAt := timePointer(time.Now())

	if got := TombstoneContent(gen.Message{Content: original}); got != original {
		t.Errorf("visible message: content = %q, want original %q", got, original)
	}
	if got := TombstoneContent(gen.Message{Content: original, HiddenAt: hiddenAt}); got == original || got != hiddenMessagePlaceholder {
		t.Errorf("hidden message: content = %q, want placeholder %q (must not leak original)", got, hiddenMessagePlaceholder)
	}
	if got := TombstoneContent(gen.Message{Content: original, DeletedAt: hiddenAt}); got == original || got != deletedMessagePlaceholder {
		t.Errorf("deleted message: content = %q, want placeholder %q (must not leak original)", got, deletedMessagePlaceholder)
	}
	// hidden takes precedence over deleted when both are set.
	if got := TombstoneContent(gen.Message{Content: original, HiddenAt: hiddenAt, DeletedAt: hiddenAt}); got != hiddenMessagePlaceholder {
		t.Errorf("hidden+deleted: content = %q, want hidden placeholder %q", got, hiddenMessagePlaceholder)
	}
}

func TestSendMessage_Text_NoMediaCall(t *testing.T) {
	called := false
	media := &mockMediaClient{
		FinalizeAndLinkFn: func(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error) {
			called = true
			return AssetInfo{}, nil
		},
	}

	var savedContent string
	repo := &mockMsgRepo{
		CreateFn: func(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			savedContent = arg.Content
			return gen.Message{
				ID:        1,
				RoomID:    arg.RoomID,
				SenderID:  arg.SenderID,
				Type:      arg.Type,
				Content:   arg.Content,
				SeqID:     1,
				CreatedAt: timePointer(time.Time{}),
			}, nil
		},
	}

	uc := newTestUC(media, repo)
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, "你好")
	if err != nil {
		t.Fatalf("SendMessage TEXT error: %v", err)
	}
	if called {
		t.Error("MediaClient.FinalizeAndLink should not be called for TEXT messages")
	}
	if savedContent != "你好" {
		t.Errorf("content = %q, want 你好", savedContent)
	}
}

func TestSendMessage_Image_FinalizeSuccess(t *testing.T) {
	media := &mockMediaClient{
		FinalizeAndLinkFn: func(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error) {
			if stagedToken != "stk_abc123" {
				t.Errorf("stagedToken = %q, want stk_abc123", stagedToken)
			}
			if refType != mediaRef.RefTypeChatMessage {
				t.Errorf("refType = %q, want %s", refType, mediaRef.RefTypeChatMessage)
			}
			return AssetInfo{AssetId: "99001", ObjectKey: "chat/2026/03/abc.jpg"}, nil
		},
	}

	var savedContent string
	var savedType int32
	repo := &mockMsgRepo{
		CreateFn: func(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			savedContent = arg.Content
			savedType = arg.Type
			return gen.Message{
				ID:        2,
				RoomID:    arg.RoomID,
				SenderID:  arg.SenderID,
				Type:      arg.Type,
				Content:   arg.Content,
				SeqID:     2,
				CreatedAt: timePointer(time.Time{}),
			}, nil
		},
	}

	uc := newTestUC(media, repo)
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 2, "stk_abc123")
	if err != nil {
		t.Fatalf("SendMessage IMAGE error: %v", err)
	}
	// 49 §5.3 #2：圖片訊息 DB 存 objectKey（顯示 URL 於讀取路徑現組），不再存完整 URL
	wantKey := "chat/2026/03/abc.jpg"
	if savedContent != wantKey {
		t.Errorf("content = %q, want objectKey %q", savedContent, wantKey)
	}
	if savedType != 2 {
		t.Errorf("type = %d, want 2", savedType)
	}
}

func TestSendMessage_Image_PublishesFriendlyBody(t *testing.T) {
	// IMAGE message content is a CDN URL; the chat event's contentPreview (used as
	// the notification body) must be a friendly string, never the URL.
	publisher := &capturingChatPublisher{}
	media := &mockMediaClient{
		FinalizeAndLinkFn: func(_ context.Context, _, _, _ string) (AssetInfo, error) {
			return AssetInfo{AssetId: "99001", ObjectKey: "chat/2026/03/abc.jpg"}, nil
		},
	}
	repo := &mockMsgRepo{
		CreateFn: func(_ context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			return gen.Message{ID: 7, RoomID: arg.RoomID, SenderID: arg.SenderID, Type: arg.Type, Content: arg.Content, SeqID: 7, CreatedAt: timePointer(time.Time{})}, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:       repo,
		convRepo:      &mockConvRepo{},
		hub:           &mockHub{},
		mediaClient:   media,
		chatPublisher: publisher,
		genID:         func() int64 { return 99999 },
	}

	if _, err := uc.SendMessage(context.Background(), "b42_s100", 42, 2, "stk_abc123"); err != nil {
		t.Fatalf("SendMessage IMAGE error: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // publish runs in a goroutine

	event := publisher.get()
	if event == nil {
		t.Fatal("PublishChatMessageSent was not called for image message")
	}
	if event.ContentPreview != "傳送了一張圖片" {
		t.Errorf("ContentPreview = %q, want 傳送了一張圖片 (never the CDN URL)", event.ContentPreview)
	}
	if event.MessageID != "7" {
		t.Errorf("MessageID = %q, want 7", event.MessageID)
	}
}

func TestSendMessage_Image_FinalizeFail(t *testing.T) {
	mc := &mockMediaClient{
		FinalizeAndLinkFn: func(ctx context.Context, stagedToken, refType, refID string) (AssetInfo, error) {
			return AssetInfo{}, fmt.Errorf("invalid token")
		},
	}

	uc := newTestUC(mc, nil)
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 2, "bad_token")
	if err == nil {
		t.Fatal("SendMessage IMAGE should fail when FinalizeAndLink fails")
	}
	if got := err.Error(); got != "圖片處理失敗: invalid token" {
		t.Errorf("error = %q, want 圖片處理失敗: invalid token", got)
	}
}

// ==================== Role Isolation ====================

// Account 100 has conversations as both buyer and seller.
// buyerConv: b100_s200 (account 100 is the buyer)
// sellerConv: b300_s100 (account 100 is the seller)
var (
	buyerConv = gen.Conversation{
		ID: 1, RoomID: "b100_s200",
		BuyerID: 100, StoreID: 200,
		LastMessageContent: stringPointer("buyer msg"),
		UpdatedAt:          timePointer(time.Time{}),
	}
	sellerConv = gen.Conversation{
		ID: 2, RoomID: "b300_s100",
		BuyerID: 300, StoreID: 100,
		LastMessageContent: stringPointer("seller msg"),
		UpdatedAt:          timePointer(time.Time{}),
	}
)

func newRoleTestUC(convRepo *mockConvRepo) *MessageUseCase {
	return &MessageUseCase{genID: func() int64 { return 99999 },
		msgRepo:  &mockMsgRepo{},
		convRepo: convRepo,
		hub:      &mockHub{},
	}
}

func TestGetConversations_BuyerRole_OnlyBuyerConversations(t *testing.T) {
	convRepo := &mockConvRepo{
		ListByBuyerFn: func(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
			if arg.BuyerID != 100 {
				t.Errorf("BuyerID = %d, want 100", arg.BuyerID)
			}
			return []gen.Conversation{buyerConv}, nil
		},
		ListBySellerFn: func(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
			t.Error("ListBySeller should not be called for buyer role")
			return nil, nil
		},
	}

	uc := newRoleTestUC(convRepo)
	convs, err := uc.GetConversations(context.Background(), 100, 1, 1, 20) // accountType=1 (BUYER)
	if err != nil {
		t.Fatalf("GetConversations error: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("len(convs) = %d, want 1", len(convs))
	}
	if convs[0].RoomID != "b100_s200" {
		t.Errorf("convs[0].RoomID = %q, want b100_s200", convs[0].RoomID)
	}
	if convs[0].TargetUserID != 200 {
		t.Errorf("TargetUserID = %d, want 200 (the seller)", convs[0].TargetUserID)
	}
}

func TestGetConversations_SellerRole_OnlySellerConversations(t *testing.T) {
	convRepo := &mockConvRepo{
		ListBySellerFn: func(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
			if arg.StoreID != 100 {
				t.Errorf("StoreID = %d, want 100", arg.StoreID)
			}
			return []gen.Conversation{sellerConv}, nil
		},
		ListByBuyerFn: func(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
			t.Error("ListByBuyer should not be called for seller role")
			return nil, nil
		},
	}

	uc := newRoleTestUC(convRepo)
	convs, err := uc.GetConversations(context.Background(), 100, 2, 1, 20) // accountType=2 (SELLER)
	if err != nil {
		t.Fatalf("GetConversations error: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("len(convs) = %d, want 1", len(convs))
	}
	if convs[0].RoomID != "b300_s100" {
		t.Errorf("convs[0].RoomID = %q, want b300_s100", convs[0].RoomID)
	}
	if convs[0].TargetUserID != 300 {
		t.Errorf("TargetUserID = %d, want 300 (the buyer)", convs[0].TargetUserID)
	}
}

func TestGetConversations_NormalizesPagination(t *testing.T) {
	tests := []struct {
		name                  string
		accountType           int32
		page, pageSize        int32
		wantLimit, wantOffset int32
	}{
		{name: "buyer defaults", accountType: 1, wantLimit: 20},
		{name: "seller clamps and offsets", accountType: 2, page: 3, pageSize: 999, wantLimit: 100, wantOffset: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotLimit, gotOffset int32
			var called bool
			convRepo := &mockConvRepo{
				ListByBuyerFn: func(_ context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
					called = true
					gotLimit, gotOffset = arg.Limit, arg.Offset
					return nil, nil
				},
				ListBySellerFn: func(_ context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
					called = true
					gotLimit, gotOffset = arg.Limit, arg.Offset
					return nil, nil
				},
			}

			uc := newRoleTestUC(convRepo)
			if _, err := uc.GetConversations(context.Background(), 100, tt.accountType, tt.page, tt.pageSize); err != nil {
				t.Fatalf("GetConversations error: %v", err)
			}
			if !called {
				t.Fatal("conversation repository was not called")
			}
			if gotLimit != tt.wantLimit || gotOffset != tt.wantOffset {
				t.Errorf("pagination = (limit %d, offset %d), want (limit %d, offset %d)", gotLimit, gotOffset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}

// #2137：買家的對話列表每一列就是一個賣家，卡片連的是賣場。target_user_id 在
// 賣家側是 store_id（內部代理鍵），賣家頁 canonical 網址走 store_code——之前
// 只帶前者，前端每次點擊都要多吃一次 middleware 308。值本來就在同一次
// GetSellerBatch 回應裡（店名與 logo 就是從那裡抄的），只是組 DTO 時被丟掉。
//
// store_code 刻意設成與 store_id 不同：兩者相等的賣家驗不出有沒有讀對欄位。
func TestGetConversations_CarriesSellerStoreCode(t *testing.T) {
	convRepo := &mockConvRepo{
		ListByBuyerFn: func(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
			return []gen.Conversation{buyerConv}, nil
		},
	}
	uc := newRoleTestUC(convRepo)
	uc.sellerClient = &mockSellerClient{
		GetProfileBatchFn: func(ctx context.Context, storeIDs []int64) (map[int64]SellerProfileInfo, error) {
			return map[int64]SellerProfileInfo{
				200: {StoreName: "Seller X", LogoKey: "logo.jpg", StoreCode: "2649605557"},
			}, nil
		},
	}

	convs, err := uc.GetConversations(context.Background(), 100, 1, 1, 20)
	if err != nil {
		t.Fatalf("GetConversations error: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("len(convs) = %d, want 1", len(convs))
	}
	if convs[0].StoreName != "Seller X" {
		t.Errorf("StoreName = %q, want Seller X", convs[0].StoreName)
	}
	if convs[0].StoreCode != "2649605557" {
		t.Errorf("StoreCode = %q, want 2649605557", convs[0].StoreCode)
	}
}

func TestSendMessageByUsers_BuyerCreatesCorrectRoomID(t *testing.T) {
	var savedRoomID string
	convRepo := &mockConvRepo{} // GetByRoomID returns ErrNoRows → new conversation
	msgRepo := &mockMsgRepo{
		CreateFn: func(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			savedRoomID = arg.RoomID
			return gen.Message{
				ID: 1, RoomID: arg.RoomID, SenderID: arg.SenderID,
				Type: arg.Type, Content: arg.Content, SeqID: 1,
				CreatedAt: timePointer(time.Time{}),
			}, nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 },
		msgRepo: msgRepo, convRepo: convRepo, hub: &mockHub{},
	}
	_, err := uc.SendMessageByUsers(context.Background(), 42, 100, 1, 1, "hi") // accountType=1 (BUYER)
	if err != nil {
		t.Fatalf("SendMessageByUsers error: %v", err)
	}
	// Buyer (42) should be in buyer position
	if savedRoomID != "b42_s100" {
		t.Errorf("roomID = %q, want b42_s100 (sender=buyer)", savedRoomID)
	}
}

func TestSendMessageByUsers_SellerCreatesCorrectRoomID(t *testing.T) {
	var savedRoomID string
	convRepo := &mockConvRepo{} // GetByRoomID returns ErrNoRows → new conversation
	msgRepo := &mockMsgRepo{
		CreateFn: func(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			savedRoomID = arg.RoomID
			return gen.Message{
				ID: 1, RoomID: arg.RoomID, SenderID: arg.SenderID,
				Type: arg.Type, Content: arg.Content, SeqID: 1,
				CreatedAt: timePointer(time.Time{}),
			}, nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 },
		msgRepo: msgRepo, convRepo: convRepo, hub: &mockHub{},
	}
	_, err := uc.SendMessageByUsers(context.Background(), 100, 42, 2, 1, "hi") // accountType=2 (SELLER)
	if err != nil {
		t.Fatalf("SendMessageByUsers error: %v", err)
	}
	// Seller (100) should be in seller position → b{receiver}_s{sender}
	if savedRoomID != "b42_s100" {
		t.Errorf("roomID = %q, want b42_s100 (sender=seller)", savedRoomID)
	}
}

func TestRegression_SameAccountDifferentRole_ConversationsIsolated(t *testing.T) {
	// DB 中 account 100 有兩筆對話：
	// - b100_s200：account 100 作為買家，和賣家 200 聊天
	// - b300_s100：account 100 作為賣家，收到買家 300 的訊息
	allConvs := []gen.Conversation{buyerConv, sellerConv}

	convRepo := &mockConvRepo{
		ListByBuyerFn: func(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
			// 模擬 DB: WHERE buyer_id = 100 只回傳 buyerConv
			var result []gen.Conversation
			for _, c := range allConvs {
				if c.BuyerID == arg.BuyerID {
					result = append(result, c)
				}
			}
			return result, nil
		},
		ListBySellerFn: func(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
			// 模擬 DB: WHERE store_id = 100 只回傳 sellerConv
			var result []gen.Conversation
			for _, c := range allConvs {
				if c.StoreID == arg.StoreID {
					result = append(result, c)
				}
			}
			return result, nil
		},
	}
	uc := newRoleTestUC(convRepo)

	// ---- 用買家 App 登入 (loginRole=1) ----
	buyerResult, err := uc.GetConversations(context.Background(), 100, 1, 1, 20)
	if err != nil {
		t.Fatalf("buyer GetConversations error: %v", err)
	}
	if len(buyerResult) != 1 {
		t.Fatalf("buyer should see 1 conversation, got %d", len(buyerResult))
	}
	if buyerResult[0].RoomID != "b100_s200" {
		t.Errorf("buyer should see b100_s200, got %q", buyerResult[0].RoomID)
	}
	// 確認買家看不到賣家的對話
	for _, c := range buyerResult {
		if c.RoomID == "b300_s100" {
			t.Error("BUG: buyer login sees seller conversation b300_s100")
		}
	}

	// ---- 用賣家 App 登入 (loginRole=2) ----
	sellerResult, err := uc.GetConversations(context.Background(), 100, 2, 1, 20)
	if err != nil {
		t.Fatalf("seller GetConversations error: %v", err)
	}
	if len(sellerResult) != 1 {
		t.Fatalf("seller should see 1 conversation, got %d", len(sellerResult))
	}
	if sellerResult[0].RoomID != "b300_s100" {
		t.Errorf("seller should see b300_s100, got %q", sellerResult[0].RoomID)
	}
	// 確認賣家看不到買家的對話
	for _, c := range sellerResult {
		if c.RoomID == "b100_s200" {
			t.Error("BUG: seller login sees buyer conversation b100_s200")
		}
	}
}

// Regression: 買家登入按「全部已讀」不該影響賣家角色的未讀訊息
func TestRegression_GetUnreadCount_IsolatedByRole(t *testing.T) {
	var calledBuyer, calledSeller bool
	convRepo := &mockConvRepo{
		SumBuyerUnreadCountFn: func(ctx context.Context, buyerID int64) (int64, error) {
			calledBuyer = true
			return 3, nil
		},
		SumSellerUnreadCountFn: func(ctx context.Context, storeID int64) (int64, error) {
			calledSeller = true
			return 5, nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: convRepo, hub: &mockHub{}}

	// 以 buyer 登入 → 只算 buyer 未讀
	count, err := uc.GetUnreadCount(context.Background(), 100, 1)
	if err != nil {
		t.Fatalf("GetUnreadCount buyer error: %v", err)
	}
	if count != 3 {
		t.Errorf("buyer unread = %d, want 3", count)
	}
	if !calledBuyer {
		t.Error("SumBuyerUnreadCount should be called for buyer role")
	}
	if calledSeller {
		t.Error("SumSellerUnreadCount should NOT be called for buyer role")
	}

	// 以 seller 登入 → 只算 seller 未讀
	calledBuyer, calledSeller = false, false
	count, err = uc.GetUnreadCount(context.Background(), 100, 2)
	if err != nil {
		t.Fatalf("GetUnreadCount seller error: %v", err)
	}
	if count != 5 {
		t.Errorf("seller unread = %d, want 5", count)
	}
	if calledBuyer {
		t.Error("SumBuyerUnreadCount should NOT be called for seller role")
	}
	if !calledSeller {
		t.Error("SumSellerUnreadCount should be called for seller role")
	}
}

func TestRegression_MarkAllRead_IsolatedByRole(t *testing.T) {
	msgRepo := &mockMsgRepo{}
	var calledBuyer, calledSeller bool
	msgRepo.MarkAllAsReadByBuyerFn = func(ctx context.Context, receiverID int64) error {
		calledBuyer = true
		return nil
	}
	msgRepo.MarkAllAsReadBySellerFn = func(ctx context.Context, receiverID int64) error {
		calledSeller = true
		return nil
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, msgRepo: msgRepo, convRepo: &mockConvRepo{}, hub: &mockHub{}}

	// 以 buyer 登入按全部已讀 → 只標記 buyer 的
	if err := uc.MarkAllRead(context.Background(), 100, 1); err != nil {
		t.Fatalf("MarkAllRead buyer error: %v", err)
	}
	if !calledBuyer {
		t.Error("MarkAllAsReadByBuyer should be called for buyer role")
	}
	if calledSeller {
		t.Error("BUG: MarkAllAsReadBySeller should NOT be called for buyer role")
	}

	// 以 seller 登入按全部已讀 → 只標記 seller 的
	calledBuyer, calledSeller = false, false
	if err := uc.MarkAllRead(context.Background(), 100, 2); err != nil {
		t.Fatalf("MarkAllRead seller error: %v", err)
	}
	if calledBuyer {
		t.Error("BUG: MarkAllAsReadByBuyer should NOT be called for seller role")
	}
	if !calledSeller {
		t.Error("MarkAllAsReadBySeller should be called for seller role")
	}
}

// ==================== Unread count 欄位化 ====================

func TestSendMessage_IncrementsSellerUnread_WhenBuyerSends(t *testing.T) {
	var incrementedSeller bool
	convRepo := &mockConvRepo{
		IncrementSellerUnreadFn: func(ctx context.Context, roomID string) error {
			incrementedSeller = true
			if roomID != "b42_s100" {
				t.Errorf("roomID = %q, want b42_s100", roomID)
			}
			return nil
		},
		IncrementBuyerUnreadFn: func(ctx context.Context, roomID string) error {
			t.Error("IncrementBuyerUnread should NOT be called when buyer sends")
			return nil
		},
	}
	msgRepo := &mockMsgRepo{
		CreateFn: func(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			return gen.Message{ID: 1, RoomID: arg.RoomID, SenderID: arg.SenderID, Type: arg.Type, Content: arg.Content, SeqID: 1, CreatedAt: timePointer(time.Time{})}, nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, msgRepo: msgRepo, convRepo: convRepo, hub: &mockHub{}}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, "hi")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if !incrementedSeller {
		t.Error("seller unread count should be incremented when buyer sends")
	}
}

func TestMarkConversationRead_ResetsBuyerUnread(t *testing.T) {
	var resetCalled bool
	convRepo := &mockConvRepo{
		ResetBuyerUnreadFn: func(ctx context.Context, roomID string) error {
			resetCalled = true
			if roomID != "b42_s100" {
				t.Errorf("roomID = %q, want b42_s100", roomID)
			}
			return nil
		},
		ResetSellerUnreadFn: func(ctx context.Context, roomID string) error {
			t.Error("ResetSellerUnread should NOT be called when buyer reads")
			return nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: convRepo, hub: &mockHub{}}
	if err := uc.MarkConversationRead(context.Background(), "b42_s100", 42); err != nil {
		t.Fatalf("MarkConversationRead error: %v", err)
	}
	if !resetCalled {
		t.Error("buyer unread count should be reset when buyer marks conversation as read")
	}
}

func TestMarkAllRead_ResetsAllBuyerUnread(t *testing.T) {
	var resetCalled bool
	convRepo := &mockConvRepo{
		ResetAllBuyerUnreadFn: func(ctx context.Context, buyerID int64) error {
			resetCalled = true
			if buyerID != 100 {
				t.Errorf("buyerID = %d, want 100", buyerID)
			}
			return nil
		},
		ResetAllSellerUnreadFn: func(ctx context.Context, storeID int64) error {
			t.Error("ResetAllSellerUnread should NOT be called for buyer role")
			return nil
		},
	}
	msgRepo := &mockMsgRepo{}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, msgRepo: msgRepo, convRepo: convRepo, hub: &mockHub{}}
	if err := uc.MarkAllRead(context.Background(), 100, 1); err != nil {
		t.Fatalf("MarkAllRead error: %v", err)
	}
	if !resetCalled {
		t.Error("ResetAllBuyerUnread should be called for buyer role")
	}
}

func TestGetConversations_ReadsUnreadFromColumn(t *testing.T) {
	convRepo := &mockConvRepo{
		ListByBuyerFn: func(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
			return []gen.Conversation{{
				ID: 1, RoomID: "b100_s200",
				BuyerID: 100, StoreID: 200,
				BuyerUnreadCount:   7,
				SellerUnreadCount:  3,
				LastMessageContent: stringPointer("hi"),
				UpdatedAt:          timePointer(time.Time{}),
			}}, nil
		},
	}

	uc := &MessageUseCase{genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: convRepo, hub: &mockHub{}}
	convs, err := uc.GetConversations(context.Background(), 100, 1, 1, 20)
	if err != nil {
		t.Fatalf("GetConversations error: %v", err)
	}
	if len(convs) != 1 {
		t.Fatalf("len(convs) = %d, want 1", len(convs))
	}
	if convs[0].UnreadCount != 7 {
		t.Errorf("UnreadCount = %d, want 7 (from buyer_unread_count column)", convs[0].UnreadCount)
	}
}

// ==================== 業務碼 ====================

func TestSendMessageByUsers_SelfMessage_ReturnsBizCode9201(t *testing.T) {
	uc := &MessageUseCase{
		genID:    func() int64 { return 99999 },
		msgRepo:  &mockMsgRepo{},
		convRepo: &mockConvRepo{},
		hub:      &mockHub{},
	}
	_, err := uc.SendMessageByUsers(context.Background(), 42, 42, 1, 1, "hi")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	code := pkgerrors.GetCode(err)
	if int32(code) != bizcode.SelfMessageNotAllowed {
		t.Errorf("error code = %d, want %d (SelfMessageNotAllowed)", int32(code), bizcode.SelfMessageNotAllowed)
	}
	if err.Error() != "不能傳訊息給自己" {
		t.Errorf("error message = %q, want 不能傳訊息給自己", err.Error())
	}
}

// TestSendMessage_DivergedSellerID_SamePrincipalAsBuyer_Blocked is the
// guard-regression case (ADR §13): the room's sellerID (999) is a diverged
// store_id whose principal is 42 — the same human as the room's
// buyerID (42). The literal ids differ, but the guard must still fire.
func TestSendMessage_DivergedSellerID_SamePrincipalAsBuyer_Blocked(t *testing.T) {
	sellerClient := &mockSellerClient{
		ResolvePrincipalFn: func(ctx context.Context, storeID int64) int64 {
			if storeID == 999 {
				return 42
			}
			return storeID
		},
	}
	uc := &MessageUseCase{
		genID:        func() int64 { return 99999 },
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          &mockHub{},
		sellerClient: sellerClient,
	}
	roomID := BuildRoomID(42, 999)
	_, err := uc.SendMessage(context.Background(), roomID, 42, 1, "hi")
	if err == nil {
		t.Fatal("expected error for same-principal self-message via diverged seller id, got nil")
	}
}

// TestSendMessage_DifferentPrincipals_Allowed proves the grandfathered/normal
// case is unaffected: distinct principals can message each other.
func TestSendMessage_DifferentPrincipals_Allowed(t *testing.T) {
	sellerClient := &mockSellerClient{}
	uc := &MessageUseCase{
		genID:        func() int64 { return 99999 },
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          &mockHub{},
		sellerClient: sellerClient,
	}
	roomID := BuildRoomID(42, 200)
	_, err := uc.SendMessage(context.Background(), roomID, 42, 1, "hi")
	if err != nil {
		t.Fatalf("unexpected error for distinct-principal message: %v", err)
	}
}

// TestSendMessage_DivergedBuyerID_SamePrincipalAsSeller_Blocked is the ②
// W2/W3 regression case (mirror of TestSendMessage_DivergedSellerID_SamePrincipalAsBuyer_Blocked
// above, but on the buyer side): the room's buyerID (500) is a diverged
// buyer_id — the buyer closed and later reopened, minting a new store-stint
// surrogate — whose principal is 42, the same human as the room's sellerID
// (42, undiverged). Before this fix the guard only resolved the seller side
// and compared the raw buyerID straight against sellerPrincipal, so a
// diverged buyer_id (500) never matched sellerPrincipal (42) and the
// self-message slipped through undetected. It must still fire once the
// buyer side is also resolved to its principal.
func TestSendMessage_DivergedBuyerID_SamePrincipalAsSeller_Blocked(t *testing.T) {
	buyerClient := &mockBuyerClient{
		ResolvePrincipalFn: func(_ context.Context, buyerID int64) int64 {
			if buyerID == 500 {
				return 42
			}
			return buyerID
		},
	}
	uc := &MessageUseCase{
		genID:       func() int64 { return 99999 },
		msgRepo:     &mockMsgRepo{},
		convRepo:    &mockConvRepo{},
		hub:         &mockHub{},
		buyerClient: buyerClient,
	}
	roomID := BuildRoomID(500, 42)
	_, err := uc.SendMessage(context.Background(), roomID, 500, 1, "hi")
	if err == nil {
		t.Fatal("expected error for same-principal self-message via diverged buyer id, got nil")
	}
}

// ==================== #2789：身分未解析時的守門行為 ====================
//
// 這三支釘的是「room 兩端是不是同一個人」這道守門在**解析失敗**時的行為。
// 守門本身（不同 principal 可以互傳、相同 principal 擋下）由上面四支覆蓋。

// TestSendMessage_UnresolvedSeller_FailsClosed is the discriminating test for
// stage B2a's im half. sellerID is not a live store_id (soft-deleted stint or
// unknown), buyerID resolves normally to a different account.
//
// It fails against BOTH rejected designs:
//   - today's echo-back (`return storeID`): 999 != 42, guard passes, message sent
//   - the "return 0" proposal:              0   != 42, guard passes, message sent
//
// Only reading unresolved_ids and refusing produces an error here. The guard's
// whole job is "these two ids might be the same human"; an id nobody can
// resolve is exactly the case where it cannot answer that, so it must not
// pretend it did.
func TestSendMessage_UnresolvedSeller_FailsClosed(t *testing.T) {
	sellerClient := &mockSellerClient{
		ResolvePrincipalStrictFn: func(_ context.Context, _ int64) (PrincipalResolution, error) {
			return PrincipalResolution{}, nil // not a live store_id
		},
	}
	buyerClient := &mockBuyerClient{
		ResolvePrincipalStrictFn: func(_ context.Context, bid int64) (PrincipalResolution, error) {
			return PrincipalResolution{AccountID: bid, Found: true}, nil
		},
	}
	uc := &MessageUseCase{
		genID:        func() int64 { return 99999 },
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          &mockHub{},
		sellerClient: sellerClient,
		buyerClient:  buyerClient,
	}
	_, err := uc.SendMessage(context.Background(), BuildRoomID(42, 999), 42, 1, "hi")
	if err == nil {
		t.Fatal("expected the send to be refused when the seller side is unresolved, got nil")
	}
	if code := pkgerrors.GetCode(err); code != pkgerrors.CodeInvalidInput {
		t.Errorf("error code = %d, want %d (CodeInvalidInput)", int32(code), int32(pkgerrors.CodeInvalidInput))
	}
	if err.Error() == "無效的 room_id：buyer 和 seller 不能是同一人" {
		t.Error("unresolved must not be reported as 'same person' — the two ids were never compared")
	}
}

// TestSendMessage_BothSidesUnresolved_NotReportedAsSamePerson is the pin
// against the "unresolved -> return 0" proposal specifically.
//
// Under that proposal both sides collapse to 0, `0 == 0` holds, and two
// genuinely different people are refused with "buyer 和 seller 不能是同一人" —
// a false positive dressed up as a guard hit. Fail-closed refuses too, but for
// the true reason, so the assertion is on WHICH refusal fires, not on whether
// one does.
func TestSendMessage_BothSidesUnresolved_NotReportedAsSamePerson(t *testing.T) {
	unresolved := func(_ context.Context, _ int64) (PrincipalResolution, error) {
		return PrincipalResolution{}, nil
	}
	uc := &MessageUseCase{
		genID:        func() int64 { return 99999 },
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          &mockHub{},
		sellerClient: &mockSellerClient{ResolvePrincipalStrictFn: unresolved},
		buyerClient:  &mockBuyerClient{ResolvePrincipalStrictFn: unresolved},
	}
	_, err := uc.SendMessage(context.Background(), BuildRoomID(42, 999), 42, 1, "hi")
	if err == nil {
		t.Fatal("expected the send to be refused when neither side resolves, got nil")
	}
	if err.Error() != "無效的 room_id：買家或賣家已不存在" {
		t.Errorf("error = %q, want the unresolved-identity refusal; two unresolved ids must never be collapsed into 'same person'", err.Error())
	}
}

// TestSendMessage_PrincipalLookupError_ReturnsUnavailable keeps "the role
// service is down" separate from "this id is not a live surrogate". Both refuse
// the send, but only one of them is the caller's fault, and a 400 on an
// infrastructure outage is a lie the client cannot act on.
func TestSendMessage_PrincipalLookupError_ReturnsUnavailable(t *testing.T) {
	uc := &MessageUseCase{
		genID:    func() int64 { return 99999 },
		msgRepo:  &mockMsgRepo{},
		convRepo: &mockConvRepo{},
		hub:      &mockHub{},
		sellerClient: &mockSellerClient{
			ResolvePrincipalStrictFn: func(_ context.Context, _ int64) (PrincipalResolution, error) {
				return PrincipalResolution{}, errors.New("seller-service unavailable")
			},
		},
	}
	_, err := uc.SendMessage(context.Background(), BuildRoomID(42, 999), 42, 1, "hi")
	if err == nil {
		t.Fatal("expected the send to be refused when the principal lookup errors, got nil")
	}
	if code := pkgerrors.GetCode(err); code != pkgerrors.CodeUnavailable {
		t.Errorf("error code = %d, want %d (CodeUnavailable)", int32(code), int32(pkgerrors.CodeUnavailable))
	}
}

func TestChatRoleFromAccountType(t *testing.T) {
	tests := []struct {
		want        string
		accountType int32
	}{
		{"buyer", 1},
		{"seller", 2},
		{"", 3},
		{"", 0},
	}
	for _, tt := range tests {
		got := chatRoleFromAccountType(tt.accountType)
		if got != tt.want {
			t.Errorf("chatRoleFromAccountType(%d) = %q, want %q", tt.accountType, got, tt.want)
		}
	}
}

// ==================== SendMessage: sender profile ====================

func TestSendMessage_BuyerSender_WSCarriesSenderName(t *testing.T) {
	hub := &capturingHub{}
	buyerClient := &mockBuyerClient{
		GetProfileBatchFn: func(_ context.Context, ids []int64) (map[int64]BuyerProfileInfo, error) {
			return map[int64]BuyerProfileInfo{
				42: {Nickname: "Alice", Avatar: "https://cdn/alice.jpg"},
			}, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:     &mockMsgRepo{},
		convRepo:    &mockConvRepo{},
		hub:         hub,
		buyerClient: buyerClient,
		genID:       func() int64 { return 1 },
	}

	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, "你好")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	chatMsg := hub.lastOfType("chat")
	if chatMsg == nil {
		t.Fatal("hub.SendToUserRole was not called with a chat message")
	}
	if chatMsg.SenderName != "Alice" {
		t.Errorf("SenderName = %q, want Alice", chatMsg.SenderName)
	}
	if chatMsg.SenderAvatar != "https://cdn/alice.jpg" {
		t.Errorf("SenderAvatar = %q, want https://cdn/alice.jpg", chatMsg.SenderAvatar)
	}
}

func TestSendMessage_SellerSender_WSCarriesStoreName(t *testing.T) {
	hub := &capturingHub{}
	sellerClient := &mockSellerClient{
		GetProfileBatchFn: func(_ context.Context, ids []int64) (map[int64]SellerProfileInfo, error) {
			return map[int64]SellerProfileInfo{
				100: {
					StoreName: "鮮果小舖",
					LogoKey:   "https://cdn/logo.jpg",
					Avatar:    "https://cdn/avatar.jpg",
				},
			}, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          hub,
		sellerClient: sellerClient,
		genID:        func() int64 { return 1 },
	}

	_, err := uc.SendMessage(context.Background(), "b42_s100", 100, 1, "您好，已出貨")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	chatMsg := hub.lastOfType("chat")
	if chatMsg == nil {
		t.Fatal("hub.SendToUserRole was not called with a chat message")
	}
	if chatMsg.SenderName != "鮮果小舖" {
		t.Errorf("SenderName = %q, want 鮮果小舖", chatMsg.SenderName)
	}
	if chatMsg.SenderAvatar != "https://cdn/logo.jpg" {
		t.Errorf("SenderAvatar = %q, want https://cdn/logo.jpg (LogoKey)", chatMsg.SenderAvatar)
	}
}

func TestSendMessage_SellerSender_AvatarFallsBackToPersonal(t *testing.T) {
	hub := &capturingHub{}
	sellerClient := &mockSellerClient{
		GetProfileBatchFn: func(_ context.Context, ids []int64) (map[int64]SellerProfileInfo, error) {
			return map[int64]SellerProfileInfo{
				100: {
					StoreName: "小農直送",
					LogoKey:   "",
					Avatar:    "https://cdn/personal.jpg",
				},
			}, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          hub,
		sellerClient: sellerClient,
		genID:        func() int64 { return 1 },
	}

	_, err := uc.SendMessage(context.Background(), "b42_s100", 100, 1, "您好")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	chatMsg := hub.lastOfType("chat")
	if chatMsg == nil {
		t.Fatal("hub.SendToUserRole was not called with a chat message")
	}
	if chatMsg.SenderAvatar != "https://cdn/personal.jpg" {
		t.Errorf("SenderAvatar = %q, want https://cdn/personal.jpg (Avatar fallback)", chatMsg.SenderAvatar)
	}
}

func TestSendMessage_NilClients_SenderNameEmpty(t *testing.T) {
	hub := &capturingHub{}
	uc := &MessageUseCase{
		msgRepo:  &mockMsgRepo{},
		convRepo: &mockConvRepo{},
		hub:      hub,
		genID:    func() int64 { return 1 },
		// buyerClient and sellerClient intentionally nil
	}

	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, "你好")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	chatMsg := hub.lastOfType("chat")
	if chatMsg == nil {
		t.Fatal("hub.SendToUserRole was not called with a chat message")
	}
	if chatMsg.SenderName != "" {
		t.Errorf("SenderName = %q, want empty when clients are nil", chatMsg.SenderName)
	}
	if chatMsg.SenderAvatar != "" {
		t.Errorf("SenderAvatar = %q, want empty when clients are nil", chatMsg.SenderAvatar)
	}
}

func TestSendMessage_PublishesChatEventWithSenderName(t *testing.T) {
	publisher := &capturingChatPublisher{}
	buyerClient := &mockBuyerClient{
		GetProfileBatchFn: func(_ context.Context, ids []int64) (map[int64]BuyerProfileInfo, error) {
			return map[int64]BuyerProfileInfo{
				42: {Nickname: "Alice", Avatar: "https://cdn/alice.jpg"},
			}, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:       &mockMsgRepo{},
		convRepo:      &mockConvRepo{},
		hub:           &mockHub{},
		buyerClient:   buyerClient,
		chatPublisher: publisher,
		genID:         func() int64 { return 1 },
	}

	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, "你好")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	// Publish runs in a goroutine — give it time to run
	// (tests are fast; 50ms is enough for a no-op mock)
	time.Sleep(50 * time.Millisecond)

	event := publisher.get()
	if event == nil {
		t.Fatal("PublishChatMessageSent was not called")
	}
	if event.SenderName != "Alice" {
		t.Errorf("event.SenderName = %q, want Alice", event.SenderName)
	}
	if event.ConversationID != "b42_s100" {
		t.Errorf("event.ConversationID = %q, want b42_s100", event.ConversationID)
	}
	// Sender 42 is the buyer, so the receiver (seller 100) is pushed as "seller".
	if event.ReceiverType != "seller" {
		t.Errorf("event.ReceiverType = %q, want seller", event.ReceiverType)
	}
	if event.ReceiverID != "100" {
		t.Errorf("event.ReceiverID = %q, want 100", event.ReceiverID)
	}
}

func TestSendMessage_SellerReceiver_PublishesPrincipalAccountID(t *testing.T) {
	publisher := &capturingChatPublisher{}
	sellerClient := &mockSellerClient{
		ResolvePrincipalFn: func(_ context.Context, sid int64) int64 {
			if sid == 777 {
				return 42 // diverged incarnation: store 777 owned by account 42
			}
			return sid
		},
	}
	uc := &MessageUseCase{
		msgRepo:       &mockMsgRepo{},
		convRepo:      &mockConvRepo{},
		hub:           &mockHub{},
		sellerClient:  sellerClient,
		chatPublisher: publisher,
		genID:         func() int64 { return 1 },
	}

	// buyer 5 sends to seller-side 777 → FCM recipient must be principal 42
	if _, err := uc.SendMessage(context.Background(), "b5_s777", 5, 1, "hi"); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	event := publisher.get()
	if event == nil {
		t.Fatal("PublishChatMessageSent was not called")
	}
	if event.ReceiverType != "seller" {
		t.Fatalf("ReceiverType = %q, want seller", event.ReceiverType)
	}
	if event.ReceiverID != "42" {
		t.Errorf("ReceiverID = %q, want principal \"42\" (device tokens key on account_id, not the store surrogate)", event.ReceiverID)
	}
	if event.StoreID != "777" {
		t.Errorf("StoreID = %q, want raw store_id \"777\" (unresolved, distinct from the principal 42 in ReceiverID)", event.StoreID)
	}
}

// TestSendMessage_BuyerReceiver_PublishesBuyerIDNotPrincipal is the #1626 fix.
//
// This test previously asserted the OPPOSITE (ReceiverID == principal account_id,
// justified as "device tokens key on account_id"). That justification was wrong:
// gateway's upsertDeviceToken resolves loginRole==1 to CurrentBuyerID before
// calling RegisterDeviceToken (gateway/internal/handler/notification.go:75-89),
// and the inbox read does the same (resolveInboxUserID). Both halves of a buyer
// notification — inbox row and FCM token lookup — key on buyer_id, so publishing
// account_id made the buyer template's row and push silently undeliverable.
//
// The seller half stays on account_id (see the sibling test above): only the
// buyer identity space diverges from the login principal.
func TestSendMessage_BuyerReceiver_PublishesBuyerIDNotPrincipal(t *testing.T) {
	publisher := &capturingChatPublisher{}
	buyerClient := &mockBuyerClient{
		ResolvePrincipalFn: func(_ context.Context, bid int64) int64 {
			if bid == 555 {
				return 7 // diverged incarnation: buyer 555 owned by account 7
			}
			return bid
		},
	}
	uc := &MessageUseCase{
		msgRepo:       &mockMsgRepo{},
		convRepo:      &mockConvRepo{},
		hub:           &mockHub{},
		buyerClient:   buyerClient,
		chatPublisher: publisher,
		genID:         func() int64 { return 1 },
	}

	// seller 200 sends to buyer-side 555 → recipient must stay the buyer_id 555,
	// NOT its login principal 7.
	if _, err := uc.SendMessage(context.Background(), "b555_s200", 200, 1, "hi"); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	event := publisher.get()
	if event == nil {
		t.Fatal("PublishChatMessageSent was not called")
	}
	if event.ReceiverType != "buyer" {
		t.Fatalf("ReceiverType = %q, want buyer", event.ReceiverType)
	}
	if event.ReceiverID != "555" {
		t.Errorf("ReceiverID = %q, want buyer_id \"555\" (inbox row + device token both key on buyer_id; account_id 7 is undeliverable)", event.ReceiverID)
	}
	if event.StoreID != "200" {
		t.Errorf("StoreID = %q, want raw store_id \"200\" even on a buyer-received message", event.StoreID)
	}
}

// TestSendMessage_ClosedStore_ReadOnlyTombstone is the ② read-only tombstone
// regression: a closed store's conversation must reject new sends (buyer
// still sees history + the tombstoned store name via GetProfileBatch, but
// can't send) with bizcode 9207.
func TestSendMessage_ClosedStore_ReadOnlyTombstone(t *testing.T) {
	sellerClient := &mockSellerClient{
		GetProfileBatchFn: func(_ context.Context, ids []int64) (map[int64]SellerProfileInfo, error) {
			return map[int64]SellerProfileInfo{
				777: {StoreName: "老王鮮魚（已停業）", StoreClosed: true},
			}, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          &mockHub{},
		sellerClient: sellerClient,
		genID:        func() int64 { return 1 },
	}

	_, err := uc.SendMessage(context.Background(), "b5_s777", 5, 1, "還在嗎")
	if err == nil {
		t.Fatal("expected closed-store send to be rejected")
	}
	if code := pkgerrors.GetCode(err); int32(code) != bizcode.StoreClosedMessageNotAllowed {
		t.Errorf("err code = %d, want %d (StoreClosedMessageNotAllowed)", int32(code), bizcode.StoreClosedMessageNotAllowed)
	}
}

// TestSendMessage_ClosedStoreGuard_LookupFailureFailsOpen 釘住一個**刻意的**取捨：
// 店家狀態查不到（RPC 失敗，不是查無此店）時，守門放行而非擋下。
//
// 為什麼要有這支測試：#2830 群組 5 之前，adapter 把 error 吞掉只回空 map，
// 於是「seller-service 掛了」與「這家店沒關」走同一條 code path —— fail-open
// 是意外不是決定。現在 error 傳回呼叫點、由呼叫點明示放行並記 log。
// 行為沒變，但決定變得看得見，因此需要一支測試守住它不被人「順手修正」
// 成 fail-closed：那會讓 seller-service 一抖動就全站 IM 送不出去，
// 代價遠大於少數訊息落進唯讀墓碑對話（對方本來就回不了）。
//
// 鑑別力：把守門改成「err != nil 時 return error」→ 本測試轉紅。
func TestSendMessage_ClosedStoreGuard_LookupFailureFailsOpen(t *testing.T) {
	called := 0
	sellerClient := &mockSellerClient{
		GetProfileBatchFn: func(_ context.Context, ids []int64) (map[int64]SellerProfileInfo, error) {
			called++
			return nil, errors.New("seller-service unavailable")
		},
	}
	uc := &MessageUseCase{
		msgRepo:      &mockMsgRepo{},
		convRepo:     &mockConvRepo{},
		hub:          &mockHub{},
		sellerClient: sellerClient,
		genID:        func() int64 { return 1 },
	}

	_, err := uc.SendMessage(context.Background(), "b5_s777", 5, 1, "還在嗎")
	if err != nil {
		t.Fatalf("店家狀態查詢失敗時應放行送出（明示 fail-open），got err = %v", err)
	}
	if called != 1 {
		t.Errorf("守門應實際查詢一次店家狀態，called = %d", called)
	}
}

// ==================== unread_changed WS push ====================

// TestPushUnreadChanged_Buyer verifies that pushUnreadChanged queries
// SumBuyerUnreadCount for role=1 and sends an unread_changed WS event.
func TestPushUnreadChanged_Buyer(t *testing.T) {
	hub := &capturingHub{}
	convRepo := &mockConvRepo{
		SumBuyerUnreadCountFn: func(_ context.Context, buyerID int64) (int64, error) {
			return 5, nil
		},
		SumSellerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
			t.Error("SumSellerUnreadCount must NOT be called for buyer role")
			return 0, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:  &mockMsgRepo{},
		convRepo: convRepo,
		hub:      hub,
		genID:    func() int64 { return 99999 },
	}

	uc.pushUnreadChanged(context.Background(), 100, accountTypeBuyer)

	ev := hub.lastOfType("unread_changed")
	if ev == nil {
		t.Fatal("no unread_changed event sent")
	}
	if ev.Role != int32(accountTypeBuyer) {
		t.Errorf("Role = %d, want %d (buyer)", ev.Role, int32(accountTypeBuyer))
	}
	if ev.UnreadCount == nil {
		t.Fatal("UnreadCount is nil")
	}
	if *ev.UnreadCount != 5 {
		t.Errorf("UnreadCount = %d, want 5", *ev.UnreadCount)
	}
	if hub.lastUserID != 100 {
		t.Errorf("pushed to userID %d, want 100", hub.lastUserID)
	}
	if hub.lastRole != int32(accountTypeBuyer) {
		t.Errorf("pushed with loginRole %d, want %d (buyer)", hub.lastRole, int32(accountTypeBuyer))
	}
}

// TestSendMessage_PushesUnreadChangedToReceiver verifies that SendMessage also
// pushes an unread_changed event to the receiver after delivering the chat message.
// Sender=buyer(100), receiver=seller(200).
func TestSendMessage_PushesUnreadChangedToReceiver(t *testing.T) {
	hub := &capturingHub{}
	convRepo := &mockConvRepo{
		SumBuyerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil
		},
		SumSellerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
			return 7, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:  &mockMsgRepo{},
		convRepo: convRepo,
		hub:      hub,
		genID:    func() int64 { return 99999 },
	}

	// buyer(100) sends to seller(200) → receiver role=seller(2)
	_, err := uc.SendMessage(context.Background(), "b100_s200", 100, 1, "你好")
	if err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}

	ev := hub.lastOfType("unread_changed")
	if ev == nil {
		t.Fatal("no unread_changed event sent after SendMessage")
	}
	if ev.Role != int32(accountTypeSeller) {
		t.Errorf("Role = %d, want %d (seller)", ev.Role, int32(accountTypeSeller))
	}
	if ev.UnreadCount == nil {
		t.Fatal("UnreadCount is nil")
	}
	if *ev.UnreadCount != 7 {
		t.Errorf("UnreadCount = %d, want 7", *ev.UnreadCount)
	}
}

// TestMarkConversationRead_PushesUnreadChangedToReader verifies that
// MarkConversationRead pushes an unread_changed event to the reader after
// resetting unread counts.
func TestMarkConversationRead_PushesUnreadChangedToReader(t *testing.T) {
	hub := &capturingHub{}
	convRepo := &mockConvRepo{
		SumBuyerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
			return 0, nil // all cleared
		},
	}
	uc := &MessageUseCase{
		msgRepo:  &mockMsgRepo{},
		convRepo: convRepo,
		hub:      hub,
		genID:    func() int64 { return 99999 },
	}

	// readerID=100 is the buyer in b100_s200
	if err := uc.MarkConversationRead(context.Background(), "b100_s200", 100); err != nil {
		t.Fatalf("MarkConversationRead error: %v", err)
	}

	ev := hub.lastOfType("unread_changed")
	if ev == nil {
		t.Fatal("no unread_changed event sent after MarkConversationRead")
	}
	if ev.Role != int32(accountTypeBuyer) {
		t.Errorf("Role = %d, want %d (buyer)", ev.Role, int32(accountTypeBuyer))
	}
	if ev.UnreadCount == nil {
		t.Fatal("UnreadCount is nil")
	}
	if *ev.UnreadCount != 0 {
		t.Errorf("UnreadCount = %d, want 0 (all cleared)", *ev.UnreadCount)
	}
}

// TestMarkConversationRead_PushesUnreadChangedToReader_SellerReader verifies that
// when the reader is the seller (readerID != buyerID), MarkConversationRead pushes
// an unread_changed event with role=seller and uses SumSellerUnreadCount.
func TestMarkConversationRead_PushesUnreadChangedToReader_SellerReader(t *testing.T) {
	hub := &capturingHub{}
	convRepo := &mockConvRepo{
		SumBuyerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
			t.Error("SumBuyerUnreadCount must NOT be called for seller reader")
			return 0, nil
		},
		SumSellerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
			return 3, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo:  &mockMsgRepo{},
		convRepo: convRepo,
		hub:      hub,
		genID:    func() int64 { return 99999 },
	}

	// readerID=200 is the seller in b100_s200 (buyerID=100, sellerID=200)
	if err := uc.MarkConversationRead(context.Background(), "b100_s200", 200); err != nil {
		t.Fatalf("MarkConversationRead error: %v", err)
	}

	ev := hub.lastOfType("unread_changed")
	if ev == nil {
		t.Fatal("no unread_changed event sent after MarkConversationRead (seller reader)")
	}
	if ev.Role != int32(accountTypeSeller) {
		t.Errorf("Role = %d, want %d (seller)", ev.Role, int32(accountTypeSeller))
	}
	if ev.UnreadCount == nil {
		t.Fatal("UnreadCount is nil")
	}
	if *ev.UnreadCount != 3 {
		t.Errorf("UnreadCount = %d, want 3", *ev.UnreadCount)
	}
	if hub.lastUserID != 200 {
		t.Errorf("pushed to userID %d, want 200 (seller)", hub.lastUserID)
	}
	if hub.lastRole != int32(accountTypeSeller) {
		t.Errorf("pushed with loginRole %d, want %d (seller)", hub.lastRole, int32(accountTypeSeller))
	}
}

// TestMarkAllRead_PushesUnreadChanged verifies that MarkAllRead pushes an
// unread_changed event for the correct role and uses the right Sum function.
func TestMarkAllRead_PushesUnreadChanged(t *testing.T) {
	t.Run("buyer", func(t *testing.T) {
		hub := &capturingHub{}
		convRepo := &mockConvRepo{
			SumBuyerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
				return 4, nil
			},
			SumSellerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
				t.Error("SumSellerUnreadCount must NOT be called for buyer accountType")
				return 0, nil
			},
		}
		uc := &MessageUseCase{
			msgRepo:  &mockMsgRepo{},
			convRepo: convRepo,
			hub:      hub,
			genID:    func() int64 { return 99999 },
		}

		if err := uc.MarkAllRead(context.Background(), 100, accountTypeBuyer); err != nil {
			t.Fatalf("MarkAllRead buyer error: %v", err)
		}

		ev := hub.lastOfType("unread_changed")
		if ev == nil {
			t.Fatal("no unread_changed event sent after MarkAllRead (buyer)")
		}
		if ev.Role != int32(accountTypeBuyer) {
			t.Errorf("Role = %d, want %d (buyer)", ev.Role, int32(accountTypeBuyer))
		}
		if ev.UnreadCount == nil {
			t.Fatal("UnreadCount is nil")
		}
		if *ev.UnreadCount != 4 {
			t.Errorf("UnreadCount = %d, want 4", *ev.UnreadCount)
		}
		if hub.lastUserID != 100 {
			t.Errorf("pushed to userID %d, want 100", hub.lastUserID)
		}
		if hub.lastRole != int32(accountTypeBuyer) {
			t.Errorf("pushed with loginRole %d, want %d (buyer)", hub.lastRole, int32(accountTypeBuyer))
		}
	})

	t.Run("seller", func(t *testing.T) {
		hub := &capturingHub{}
		convRepo := &mockConvRepo{
			SumBuyerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
				t.Error("SumBuyerUnreadCount must NOT be called for seller accountType")
				return 0, nil
			},
			SumSellerUnreadCountFn: func(_ context.Context, _ int64) (int64, error) {
				return 9, nil
			},
		}
		uc := &MessageUseCase{
			msgRepo:  &mockMsgRepo{},
			convRepo: convRepo,
			hub:      hub,
			genID:    func() int64 { return 99999 },
		}

		if err := uc.MarkAllRead(context.Background(), 200, accountTypeSeller); err != nil {
			t.Fatalf("MarkAllRead seller error: %v", err)
		}

		ev := hub.lastOfType("unread_changed")
		if ev == nil {
			t.Fatal("no unread_changed event sent after MarkAllRead (seller)")
		}
		if ev.Role != int32(accountTypeSeller) {
			t.Errorf("Role = %d, want %d (seller)", ev.Role, int32(accountTypeSeller))
		}
		if ev.UnreadCount == nil {
			t.Fatal("UnreadCount is nil")
		}
		if *ev.UnreadCount != 9 {
			t.Errorf("UnreadCount = %d, want 9", *ev.UnreadCount)
		}
		if hub.lastUserID != 200 {
			t.Errorf("pushed to userID %d, want 200", hub.lastUserID)
		}
		if hub.lastRole != int32(accountTypeSeller) {
			t.Errorf("pushed with loginRole %d, want %d (seller)", hub.lastRole, int32(accountTypeSeller))
		}
	})
}

// ==================== 賣家停權 IM 訂單關聯（14 號 §6.5） ====================

type mockOrderClient struct {
	GetOrderPartiesFn func(ctx context.Context, orderNo string) (OrderParties, error)
	HasOrderBetweenFn func(ctx context.Context, buyerID, storeID int64) (bool, string, error)
	hasOrderBetweenN  int
	getOrderPartiesN  int
}

func (m *mockOrderClient) GetOrderParties(ctx context.Context, orderNo string) (OrderParties, error) {
	m.getOrderPartiesN++
	if m.GetOrderPartiesFn != nil {
		return m.GetOrderPartiesFn(ctx, orderNo)
	}
	return OrderParties{}, fmt.Errorf("not found")
}

func (m *mockOrderClient) HasOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, string, error) {
	m.hasOrderBetweenN++
	if m.HasOrderBetweenFn != nil {
		return m.HasOrderBetweenFn(ctx, buyerID, storeID)
	}
	return false, "", nil
}

// TestSendMessage_SuspendedSeller_BoundConversation_Allowed: a conversation
// already bound (order_binding_verified_at set) lets a suspended seller send —
// no order-service calls needed, the bind itself already encodes the three-way
// match (14 號 §6.5.3).
func TestSendMessage_SuspendedSeller_BoundConversation_Allowed(t *testing.T) {
	convRepo := &mockConvRepo{
		GetByRoomIDFn: func(ctx context.Context, roomID string) (gen.Conversation, error) {
			return gen.Conversation{RoomID: roomID, OrderBindingVerifiedAt: timePointer(time.Now())}, nil
		},
	}
	orderClient := &mockOrderClient{}
	uc := &MessageUseCase{
		genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: convRepo, hub: &mockHub{}, orderClient: orderClient,
	}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 100, 1, "hi", WithSellerSuspended(true))
	if err != nil {
		t.Fatalf("unexpected error for suspended seller with pre-bound conversation: %v", err)
	}
	if orderClient.hasOrderBetweenN != 0 || orderClient.getOrderPartiesN != 0 {
		t.Errorf("order-service should not be called when already bound; hasOrderBetweenN=%d getOrderPartiesN=%d",
			orderClient.hasOrderBetweenN, orderClient.getOrderPartiesN)
	}
}

// TestSendMessage_SuspendedSeller_UnboundNoRelatedOrder_Denied: no binding and
// no related order between the pair → deny with SellerSuspendedOperationBlocked
// (§6.5.3 "未綁定 → 走回填路徑；...失敗 → 擋").
func TestSendMessage_SuspendedSeller_UnboundNoRelatedOrder_Denied(t *testing.T) {
	orderClient := &mockOrderClient{
		HasOrderBetweenFn: func(ctx context.Context, buyerID, sellerID int64) (bool, string, error) {
			return false, "", nil
		},
	}
	uc := &MessageUseCase{
		genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: &mockConvRepo{}, hub: &mockHub{}, orderClient: orderClient,
	}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 100, 1, "hi", WithSellerSuspended(true))
	if err == nil {
		t.Fatal("expected deny for suspended seller with no related order, got nil")
	}
	if code := pkgerrors.GetCode(err); int32(code) != bizcode.SellerSuspendedOperationBlocked {
		t.Errorf("error code = %d, want %d (SellerSuspendedOperationBlocked)", int32(code), bizcode.SellerSuspendedOperationBlocked)
	}
}

// TestSendMessage_SuspendedSeller_OrderServiceError_Denied: order-service call
// fails (cannot verify) → deny, fail-closed per §6.5.3 (not fail-open).
func TestSendMessage_SuspendedSeller_OrderServiceError_Denied(t *testing.T) {
	orderClient := &mockOrderClient{
		HasOrderBetweenFn: func(ctx context.Context, buyerID, sellerID int64) (bool, string, error) {
			return false, "", fmt.Errorf("order-service unavailable")
		},
	}
	uc := &MessageUseCase{
		genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: &mockConvRepo{}, hub: &mockHub{}, orderClient: orderClient,
	}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 100, 1, "hi", WithSellerSuspended(true))
	if err == nil {
		t.Fatal("expected deny when order-service verification fails, got nil")
	}
	if code := pkgerrors.GetCode(err); int32(code) != bizcode.SellerSuspendedOperationBlocked {
		t.Errorf("error code = %d, want %d (SellerSuspendedOperationBlocked)", int32(code), bizcode.SellerSuspendedOperationBlocked)
	}
}

// TestSendMessage_SuspendedSeller_BackfillSucceeds_AllowedAndBinds: existing
// unbound conversation, HasOrderBetween finds a related order → allow AND
// persist the backfilled binding (14 號 §6.5.1 路徑②).
func TestSendMessage_SuspendedSeller_BackfillSucceeds_AllowedAndBinds(t *testing.T) {
	var boundRoomID, boundOrderNo string
	convRepo := &mockConvRepo{
		BindOrderFn: func(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error {
			boundRoomID, boundOrderNo = roomID, orderNo
			return nil
		},
	}
	orderClient := &mockOrderClient{
		HasOrderBetweenFn: func(ctx context.Context, buyerID, sellerID int64) (bool, string, error) {
			if buyerID == 42 && sellerID == 100 {
				return true, "O20260713001", nil
			}
			return false, "", nil
		},
	}
	uc := &MessageUseCase{
		genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: convRepo, hub: &mockHub{}, orderClient: orderClient,
	}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 100, 1, "hi", WithSellerSuspended(true))
	if err != nil {
		t.Fatalf("unexpected error for suspended seller with backfillable order: %v", err)
	}
	if boundRoomID != "b42_s100" || boundOrderNo != "O20260713001" {
		t.Errorf("BindOrder called with (%q,%q), want (b42_s100, O20260713001)", boundRoomID, boundOrderNo)
	}
}

// TestSendMessage_OrderNoHint_ThreeWayMatch_BindsProactively: non-suspended
// sender supplies an order_no intent hint that matches this room's buyer/seller
// → binding is established proactively (best-effort, never blocks the send),
// even though the seller isn't currently suspended (14 號 §6.5.1 路徑①).
func TestSendMessage_OrderNoHint_ThreeWayMatch_BindsProactively(t *testing.T) {
	var boundOrderNo string
	convRepo := &mockConvRepo{
		BindOrderFn: func(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error {
			boundOrderNo = orderNo
			return nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderPartiesFn: func(ctx context.Context, orderNo string) (OrderParties, error) {
			return OrderParties{BuyerID: 42, StoreID: 100}, nil
		},
		HasOrderBetweenFn: func(ctx context.Context, buyerID, sellerID int64) (bool, string, error) {
			return true, "O_HINT", nil
		},
	}
	uc := &MessageUseCase{
		genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: convRepo, hub: &mockHub{}, orderClient: orderClient,
	}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, "聯絡賣家", WithOrderNoHint("O_HINT"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if boundOrderNo != "O_HINT" {
		t.Errorf("BindOrder orderNo = %q, want O_HINT", boundOrderNo)
	}
}

// TestSendMessage_OrderNoHint_MismatchedParty_NotBound is the anti-fraud
// regression: a suspended seller cannot bind by hinting someone else's real
// order_no (order.buyer != this room's buyer) — 14 號 §6.5.3「賣家無法藉掛他人
// 單號開新對話」. The mismatched hint must not establish a binding, and with no
// independently-related order between THIS pair, the send is still denied.
func TestSendMessage_OrderNoHint_MismatchedParty_NotBound(t *testing.T) {
	var bindCalled bool
	convRepo := &mockConvRepo{
		BindOrderFn: func(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error {
			bindCalled = true
			return nil
		},
	}
	orderClient := &mockOrderClient{
		GetOrderPartiesFn: func(ctx context.Context, orderNo string) (OrderParties, error) {
			// order_no belongs to a different buyer (999), not this room's buyer (42).
			return OrderParties{BuyerID: 999, StoreID: 100}, nil
		},
		HasOrderBetweenFn: func(ctx context.Context, buyerID, sellerID int64) (bool, string, error) {
			return false, "", nil // no legitimate relation between 42 and 100 either
		},
	}
	uc := &MessageUseCase{
		genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: convRepo, hub: &mockHub{}, orderClient: orderClient,
	}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 100, 1, "hi", WithOrderNoHint("O_OTHER_BUYER"), WithSellerSuspended(true))
	if err == nil {
		t.Fatal("expected deny for mismatched order_no hint with no independent relation, got nil")
	}
	if bindCalled {
		t.Error("BindOrder must not be called for a mismatched (掛他人單號) hint")
	}
}

// TestSendMessage_BuyerSender_SuspendedFlagIgnored: WithSellerSuspended only
// gates the SELLER side of the room (§6.5.3); a buyer sending is never gated by
// it, regardless of binding state.
func TestSendMessage_BuyerSender_SuspendedFlagIgnored(t *testing.T) {
	orderClient := &mockOrderClient{
		HasOrderBetweenFn: func(ctx context.Context, buyerID, sellerID int64) (bool, string, error) {
			t.Error("order-service must not be consulted for a buyer sender")
			return false, "", nil
		},
	}
	uc := &MessageUseCase{
		genID: func() int64 { return 99999 }, msgRepo: &mockMsgRepo{}, convRepo: &mockConvRepo{}, hub: &mockHub{}, orderClient: orderClient,
	}
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, "hi", WithSellerSuspended(true))
	if err != nil {
		t.Fatalf("unexpected error for buyer sender: %v", err)
	}
}

func TestSendMessage_ProductCard_HintBecomesEventPreview(t *testing.T) {
	publisher := &capturingChatPublisher{}
	repo := &mockMsgRepo{
		CreateFn: func(_ context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
			return gen.Message{ID: 7, RoomID: arg.RoomID, SenderID: arg.SenderID, Type: arg.Type, Content: arg.Content, PreviewText: arg.PreviewText, SeqID: 7, CreatedAt: timePointer(time.Time{})}, nil
		},
	}
	var upsertArg gen.UpsertConversationParams
	convRepo := &mockConvRepo{
		UpsertFn: func(_ context.Context, arg gen.UpsertConversationParams) (gen.Conversation, error) {
			upsertArg = arg
			return gen.Conversation{}, nil
		},
	}
	uc := &MessageUseCase{
		msgRepo: repo, convRepo: convRepo, hub: &mockHub{}, chatPublisher: publisher,
		genID: func() int64 { return 99999 },
	}

	cardJSON := `{"messageType":"product","productId":"859783856743286973","productName":"池上米"}`
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, cardJSON, WithPreviewHint("傳送了一張商品卡片"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // publish runs in a goroutine

	event := publisher.get()
	if event == nil {
		t.Fatal("PublishChatMessageSent was not called")
	}
	if event.ContentPreview != "傳送了一張商品卡片" {
		t.Errorf("ContentPreview = %q, want 傳送了一張商品卡片 (never the raw JSON)", event.ContentPreview)
	}
	if upsertArg.LastMessagePreview == nil || *upsertArg.LastMessagePreview != "傳送了一張商品卡片" {
		t.Errorf("conversations.last_message_preview = %+v, want valid 傳送了一張商品卡片", upsertArg.LastMessagePreview)
	}
}

func TestSendMessage_CardWithoutHint_FallsBackToGenericLabel(t *testing.T) {
	// Regression guard for the original bug: a card message sent without a
	// preview hint (old/forgetful caller) must never leak raw JSON into the
	// push notification body.
	publisher := &capturingChatPublisher{}
	uc := &MessageUseCase{
		msgRepo: &mockMsgRepo{}, convRepo: &mockConvRepo{}, hub: &mockHub{}, chatPublisher: publisher,
		genID: func() int64 { return 99999 },
	}

	cardJSON := `{"messageType":"product","productId":"859783856743286973","productName":"池上米"}`
	_, err := uc.SendMessage(context.Background(), "b42_s100", 42, 1, cardJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	event := publisher.get()
	if event == nil {
		t.Fatal("PublishChatMessageSent was not called")
	}
	if event.ContentPreview != "傳送了一張卡片" {
		t.Errorf("ContentPreview = %q, want generic 傳送了一張卡片 fallback", event.ContentPreview)
	}
	if strings.Contains(event.ContentPreview, "{") {
		t.Errorf("ContentPreview leaked raw JSON: %q", event.ContentPreview)
	}
}

func TestComputePreview(t *testing.T) {
	long := strings.Repeat("字", PreviewMaxLength+10)
	wantLongTruncated := strings.Repeat("字", PreviewMaxLength) + "..."
	cases := []struct {
		name    string
		msgType int32
		content string
		hint    string
		want    string
	}{
		{"image 一律固定字樣，忽略 content/hint", 2, "chat/2026/01/x.jpg", "不該被用到", "傳送了一張圖片"},
		{"有 hint 直接用 hint（卡片訊息的路徑）", 1, `{"messageType":"product","productName":"池上米"}`, "傳送了一張商品卡片", "傳送了一張商品卡片"},
		{"無 hint 且 content 長得像 JSON → 通用卡片字樣，絕不外洩原文", 1, `{"messageType":"product","productName":"池上米"}`, "", "傳送了一張卡片"},
		{"無 hint 且 content 是陣列 JSON → 同樣視為卡片", 1, `[{"a":1}]`, "", "傳送了一張卡片"},
		{"純文字：沒有 hint、也不像 JSON → 原文", 1, "請問還有現貨嗎？", "", "請問還有現貨嗎？"},
		{"純文字過長（多位元組字元）→ 依字元數截斷，不可切在字元中間", 1, long, "", wantLongTruncated},
		{"hint 過長（多位元組字元）→ 依字元數截斷，不可切在字元中間", 1, "{}", long, wantLongTruncated},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := computePreview(c.msgType, c.content, c.hint)
			if got != c.want {
				t.Errorf("computePreview(%d, %q, %q) = %q, want %q", c.msgType, c.content, c.hint, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("computePreview(%d, %q, %q) = %q is not valid UTF-8", c.msgType, c.content, c.hint, got)
			}
		})
	}
}

func TestAuthorizeRoomAccess(t *testing.T) {
	roomID := BuildRoomID(1001, 2002) // buyer=1001, seller=2002

	if err := authorizeRoomAccess(roomID, 1001, accountTypeBuyer); err != nil {
		t.Fatalf("buyer participant should be authorized, got err: %v", err)
	}
	if err := authorizeRoomAccess(roomID, 2002, accountTypeSeller); err != nil {
		t.Fatalf("seller participant should be authorized, got err: %v", err)
	}
	if err := authorizeRoomAccess(roomID, 9999, accountTypeBuyer); err == nil {
		t.Fatal("non-participant buyer must be rejected")
	}
	// #2499 acceptance criteria: a numeric collision between the buyer/store
	// surrogate namespaces must not grant cross-role access.
	if err := authorizeRoomAccess(roomID, 1001, accountTypeSeller); err == nil {
		t.Fatal("same numeric id under the wrong role must be rejected")
	}
	if err := authorizeRoomAccess("not-a-room-id", 1001, accountTypeBuyer); err == nil {
		t.Fatal("malformed roomID must be rejected")
	}
	if err := authorizeRoomAccess(roomID, 1001, 0); err == nil {
		t.Fatal("unknown loginRole (0) must be rejected")
	}
}

func TestAllowSyncFrame_NilLimiterAlwaysAllows(t *testing.T) {
	uc := &MessageUseCase{}
	if !uc.allowSyncFrame(context.Background(), 1001) {
		t.Fatal("a MessageUseCase built without NewMessageUseCase (nil syncLimiter, as in most existing tests) must not rate-limit")
	}
}

func TestAllowSyncFrame_EnforcesLimit(t *testing.T) {
	uc := &MessageUseCase{syncLimiter: ratelimit.New(nil, 0, nil)} // nil rdb -> in-memory fallback
	for i := 0; i < syncRateLimit; i++ {
		if !uc.allowSyncFrame(context.Background(), 1001) {
			t.Fatalf("call %d/%d should be within the limit", i+1, syncRateLimit)
		}
	}
	if uc.allowSyncFrame(context.Background(), 1001) {
		t.Fatal("one call past the limit should be rejected")
	}
}

// Regression: SendMessage's participant check (message.go, "sender %d is not
// a participant of room %s") had zero test coverage before #2499's audit.
// Pin it down so it can't silently regress — this is the consistency
// baseline #2499's authorizeRoomAccess is modeled on.
func TestSendMessage_RejectsNonParticipantSender(t *testing.T) {
	uc := &MessageUseCase{genID: func() int64 { return 1 }}
	room := BuildRoomID(1001, 2002)

	_, err := uc.SendMessage(context.Background(), room, 9999, 1, "hello")
	if err == nil {
		t.Fatal("expected an error when sender is neither the buyer nor seller side of the room")
	}
}
