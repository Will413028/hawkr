package worker

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/repository/postgres/gen"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

// fakeMsgRepo is a minimal domain.MessageRepository fake for exercising
// newBuyerDeletedHandler's purge call without a real DB. Only RedactBySenderFn
// is instrumented; every other method is a zero-value no-op —
// RedactDeletedAccount (the usecase method the handler calls) reaches
// RedactBySender and ListImageMessageIDsBySender only; the rest are dead for
// this test's purposes but still required to satisfy the interface.
type fakeMsgRepo struct {
	RedactBySenderFn              func(ctx context.Context, senderIDs []int64) (int64, error)
	ListImageMessageIDsBySenderFn func(ctx context.Context, senderIDs []int64) ([]int64, error)
}

func (f *fakeMsgRepo) Create(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	return gen.Message{}, nil
}
func (f *fakeMsgRepo) CreateWithLock(ctx context.Context, arg gen.CreateMessageParams) (gen.Message, error) {
	return gen.Message{}, nil
}
func (f *fakeMsgRepo) GetByID(ctx context.Context, id int64) (gen.Message, error) {
	return gen.Message{}, nil
}
func (f *fakeMsgRepo) List(ctx context.Context, arg gen.ListMessagesParams) ([]gen.Message, error) {
	return nil, nil
}
func (f *fakeMsgRepo) ListBefore(ctx context.Context, arg gen.ListMessagesBeforeParams) ([]gen.Message, error) {
	return nil, nil
}
func (f *fakeMsgRepo) MarkAsRead(ctx context.Context, arg gen.MarkMessagesAsReadParams) error {
	return nil
}
func (f *fakeMsgRepo) MarkByIDsAsRead(ctx context.Context, receiverID int64, ids []int64) error {
	return nil
}
func (f *fakeMsgRepo) RecalcUnreadForMessages(ctx context.Context, messageIDs []int64) error {
	return nil
}
func (f *fakeMsgRepo) MarkAllAsReadByBuyer(ctx context.Context, receiverID int64) error {
	return nil
}
func (f *fakeMsgRepo) MarkAllAsReadBySeller(ctx context.Context, receiverID int64) error {
	return nil
}
func (f *fakeMsgRepo) ListAfterSeq(ctx context.Context, arg gen.ListMessagesAfterSeqParams) ([]gen.Message, error) {
	return nil, nil
}
func (f *fakeMsgRepo) HideMessage(ctx context.Context, id int64, reason string) error {
	return nil
}
func (f *fakeMsgRepo) RedactBySender(ctx context.Context, senderIDs []int64) (int64, error) {
	if f.RedactBySenderFn != nil {
		return f.RedactBySenderFn(ctx, senderIDs)
	}
	return 0, nil
}
func (f *fakeMsgRepo) ListImageMessageIDsBySender(ctx context.Context, senderIDs []int64) ([]int64, error) {
	if f.ListImageMessageIDsBySenderFn != nil {
		return f.ListImageMessageIDsBySenderFn(ctx, senderIDs)
	}
	return nil, nil
}

// fakeConvRepo is a minimal domain.ConversationRepository fake, mirroring
// fakeMsgRepo above. Only RedactLastMessageBySenderFn is instrumented.
type fakeConvRepo struct {
	RedactLastMessageBySenderFn func(ctx context.Context, senderIDs []int64, placeholder string) error
}

func (f *fakeConvRepo) Upsert(ctx context.Context, arg gen.UpsertConversationParams) (gen.Conversation, error) {
	return gen.Conversation{}, nil
}
func (f *fakeConvRepo) GetByRoomID(ctx context.Context, roomID string) (gen.Conversation, error) {
	return gen.Conversation{}, nil
}
func (f *fakeConvRepo) ListByBuyer(ctx context.Context, arg gen.ListConversationsByBuyerParams) ([]gen.Conversation, error) {
	return nil, nil
}
func (f *fakeConvRepo) ListBySeller(ctx context.Context, arg gen.ListConversationsBySellerParams) ([]gen.Conversation, error) {
	return nil, nil
}
func (f *fakeConvRepo) IncrementBuyerUnread(ctx context.Context, roomID string) error  { return nil }
func (f *fakeConvRepo) IncrementSellerUnread(ctx context.Context, roomID string) error { return nil }
func (f *fakeConvRepo) ResetBuyerUnread(ctx context.Context, roomID string) error      { return nil }
func (f *fakeConvRepo) ResetSellerUnread(ctx context.Context, roomID string) error     { return nil }
func (f *fakeConvRepo) ResetAllBuyerUnread(ctx context.Context, buyerID int64) error {
	return nil
}
func (f *fakeConvRepo) ResetAllSellerUnread(ctx context.Context, storeID int64) error { return nil }
func (f *fakeConvRepo) SumBuyerUnreadCount(ctx context.Context, buyerID int64) (int64, error) {
	return 0, nil
}
func (f *fakeConvRepo) SumSellerUnreadCount(ctx context.Context, storeID int64) (int64, error) {
	return 0, nil
}
func (f *fakeConvRepo) RedactLastMessageBySender(ctx context.Context, senderIDs []int64, placeholder string) error {
	if f.RedactLastMessageBySenderFn != nil {
		return f.RedactLastMessageBySenderFn(ctx, senderIDs, placeholder)
	}
	return nil
}
func (f *fakeConvRepo) BindOrder(ctx context.Context, roomID, orderNo string, verifiedAt time.Time) error {
	return nil
}

// fakeBuyerClient is a minimal usecase.BuyerClient fake for the
// account.deleted/buyer.deleted handler tests below. ListIDsByAccountFn
// defaults to returning nil (no historical stints), so RedactDeletedAccount's
// merge collapses to just accountID — these worker-level tests only need
// enumeration to not error, not to exercise multi-stint enumeration itself
// (that's covered at the usecase layer by
// internal/usecase/message_test.go's TestRedactDeletedAccount_* tests).
type fakeBuyerClient struct {
	ListIDsByAccountFn func(ctx context.Context, accountID int64) ([]int64, error)
}

func (f *fakeBuyerClient) GetProfileBatch(ctx context.Context, buyerIDs []int64) (map[int64]usecase.BuyerProfileInfo, error) {
	return nil, nil
}
func (f *fakeBuyerClient) ResolvePrincipal(ctx context.Context, buyerID int64) int64 {
	return buyerID
}

// ResolvePrincipalStrict is not on this worker's path; it exists only to
// satisfy usecase.BuyerClient. Models the grandfathered baseline (every id is
// a live buyer_id whose principal is itself), matching ResolvePrincipal above.
func (f *fakeBuyerClient) ResolvePrincipalStrict(ctx context.Context, buyerID int64) (usecase.PrincipalResolution, error) {
	return usecase.PrincipalResolution{AccountID: buyerID, Found: true}, nil
}
func (f *fakeBuyerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if f.ListIDsByAccountFn != nil {
		return f.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

// fakeSellerClient mirrors fakeBuyerClient above.
type fakeSellerClient struct {
	ListIDsByAccountFn func(ctx context.Context, accountID int64) ([]int64, error)
}

func (f *fakeSellerClient) GetProfileBatch(ctx context.Context, storeIDs []int64) (map[int64]usecase.SellerProfileInfo, error) {
	return nil, nil
}
func (f *fakeSellerClient) ResolvePrincipal(ctx context.Context, storeID int64) int64 {
	return storeID
}

// ResolvePrincipalStrict mirrors fakeBuyerClient's; see that comment.
func (f *fakeSellerClient) ResolvePrincipalStrict(ctx context.Context, storeID int64) (usecase.PrincipalResolution, error) {
	return usecase.PrincipalResolution{AccountID: storeID, Found: true}, nil
}
func (f *fakeSellerClient) ListIDsByAccount(ctx context.Context, accountID int64) ([]int64, error) {
	if f.ListIDsByAccountFn != nil {
		return f.ListIDsByAccountFn(ctx, accountID)
	}
	return nil, nil
}

// mustMarshalBuyerDeleted builds a buyer.deleted wire body: BaseEvent's
// snake_case fields plus BuyerDeletedEvent's accountId/buyerId (mirrors
// pkg/mq.BuyerDeletedEvent's json tags). Built as a map, not the real struct,
// so this test file takes no compile-time dependency on the pkg/mq
// event-definition (foundation) task beyond what production code already
// requires.
func mustMarshalBuyerDeleted(t *testing.T, accountID, buyerID int64) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"event_id":   "evt1",
		"event_type": "BUYER_DELETED",
		"accountId":  accountID,
		"buyerId":    buyerID,
	})
	if err != nil {
		t.Fatalf("marshal test event: %v", err)
	}
	return body
}

func TestBuyerDeletedHandler_PurgesBySenderAccountID(t *testing.T) {
	var gotSenders []int64
	msgRepo := &fakeMsgRepo{
		RedactBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			gotSenders = senderIDs
			return 3, nil
		},
	}
	convRepo := &fakeConvRepo{}
	buyerClient := &fakeBuyerClient{}
	sellerClient := &fakeSellerClient{}
	msgUC := usecase.NewMessageUseCase(msgRepo, convRepo, nil, buyerClient, sellerClient, nil, nil, nil, nil, &config.Config{})
	handler := newBuyerDeletedHandler(msgUC, zap.NewNop())

	body := mustMarshalBuyerDeleted(t, 777, 42)
	if err := handler(context.Background(), "buyer.deleted", body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// This worker-level test only exercises the routing/unmarshal path; the
	// fake buyer/seller clients return no extra historical IDs, so the merged
	// set collapses to just payload.AccountID. Multi-stint enumeration itself
	// is covered at the usecase layer (message_test.go).
	if len(gotSenders) != 1 || gotSenders[0] != 777 {
		t.Errorf("RedactBySender senderIDs = %v, want [777] (payload.AccountID)", gotSenders)
	}
}

func TestBuyerDeletedHandler_MissingAccountID_ReturnsError(t *testing.T) {
	msgUC := usecase.NewMessageUseCase(&fakeMsgRepo{}, &fakeConvRepo{}, nil, nil, nil, nil, nil, nil, nil, &config.Config{})
	handler := newBuyerDeletedHandler(msgUC, zap.NewNop())

	body, _ := json.Marshal(map[string]interface{}{"event_type": "BUYER_DELETED", "buyerId": 42})
	if err := handler(context.Background(), "buyer.deleted", body); err == nil {
		t.Fatal("expected error for missing accountId, got nil")
	}
}

func TestBuyerDeletedHandler_PropagatesRedactError(t *testing.T) {
	msgRepo := &fakeMsgRepo{
		RedactBySenderFn: func(_ context.Context, _ []int64) (int64, error) {
			return 0, context.DeadlineExceeded
		},
	}
	buyerClient := &fakeBuyerClient{}
	sellerClient := &fakeSellerClient{}
	msgUC := usecase.NewMessageUseCase(msgRepo, &fakeConvRepo{}, nil, buyerClient, sellerClient, nil, nil, nil, nil, &config.Config{})
	handler := newBuyerDeletedHandler(msgUC, zap.NewNop())

	body := mustMarshalBuyerDeleted(t, 777, 42)
	if err := handler(context.Background(), "buyer.deleted", body); err == nil {
		t.Fatal("expected error propagated from RedactDeletedAccount, got nil")
	}
}
