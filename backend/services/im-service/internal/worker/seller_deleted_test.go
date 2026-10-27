package worker

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/go-pkg/mq"
	"github.com/Will413028/hawkr/im-service/internal/config"
	"github.com/Will413028/hawkr/im-service/internal/usecase"
)

// mustMarshalSellerDeleted builds a seller.deleted wire body matching
// mq.SellerDeletedEvent's json tags (event_type/accountId/store_id). Built as a map,
// not the struct, so this test doesn't take on a hidden compile-time
// assumption about the payload shape beyond what the handler itself decodes.
func mustMarshalSellerDeleted(t *testing.T, accountID, storeID int64) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"event_type": "SELLER_DELETED",
		"accountId":  accountID,
		"store_id":   storeID,
	})
	if err != nil {
		t.Fatalf("marshal test event: %v", err)
	}
	return body
}

func TestSellerDeletedHandler_PurgesBySenderAccountID(t *testing.T) {
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
	handler := newSellerDeletedHandler(msgUC, zap.NewNop())

	body := mustMarshalSellerDeleted(t, 777, 100)
	if err := handler(context.Background(), "seller.deleted", body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotSenders) != 1 || gotSenders[0] != 777 {
		t.Errorf("RedactBySender senderIDs = %v, want [777] (payload.AccountID)", gotSenders)
	}
}

func TestSellerDeletedHandler_MissingAccountID_ReturnsError(t *testing.T) {
	msgUC := usecase.NewMessageUseCase(&fakeMsgRepo{}, &fakeConvRepo{}, nil, nil, nil, nil, nil, nil, nil, &config.Config{})
	handler := newSellerDeletedHandler(msgUC, zap.NewNop())

	body, _ := json.Marshal(map[string]interface{}{"event_type": "SELLER_DELETED", "store_id": 100})
	if err := handler(context.Background(), "seller.deleted", body); err == nil {
		t.Fatal("expected error for missing accountId, got nil")
	}
}

// TestSellerDeletedHandler_ZeroStoreID_Succeeds mirrors what
// backfill-seller-deleted actually publishes (services/seller-service/cmd/backfill-seller-deleted/main.go):
// AccountID set, StoreID always zero. The guard must key on AccountID, not
// StoreID, or every backfilled event dead-letters.
func TestSellerDeletedHandler_ZeroStoreID_Succeeds(t *testing.T) {
	msgRepo := &fakeMsgRepo{
		RedactBySenderFn: func(_ context.Context, senderIDs []int64) (int64, error) {
			return 1, nil
		},
	}
	buyerClient := &fakeBuyerClient{}
	sellerClient := &fakeSellerClient{}
	msgUC := usecase.NewMessageUseCase(msgRepo, &fakeConvRepo{}, nil, buyerClient, sellerClient, nil, nil, nil, nil, &config.Config{})
	handler := newSellerDeletedHandler(msgUC, zap.NewNop())

	body := mustMarshalSellerDeleted(t, 777, 0)
	if err := handler(context.Background(), "seller.deleted", body); err != nil {
		t.Fatalf("unexpected error for zero store_id with non-zero accountId: %v", err)
	}
}

func TestSellerDeletedHandler_PropagatesRedactError(t *testing.T) {
	msgRepo := &fakeMsgRepo{
		RedactBySenderFn: func(_ context.Context, _ []int64) (int64, error) {
			return 0, context.DeadlineExceeded
		},
	}
	buyerClient := &fakeBuyerClient{}
	sellerClient := &fakeSellerClient{}
	msgUC := usecase.NewMessageUseCase(msgRepo, &fakeConvRepo{}, nil, buyerClient, sellerClient, nil, nil, nil, nil, &config.Config{})
	handler := newSellerDeletedHandler(msgUC, zap.NewNop())

	body := mustMarshalSellerDeleted(t, 777, 100)
	if err := handler(context.Background(), "seller.deleted", body); err == nil {
		t.Fatal("expected error propagated from RedactDeletedAccount, got nil")
	}
}

// TestSellerDeletedPayload_EventTypeTagMatchesRealWire guards against wire-tag
// bugs like the one fixed for AccountDeletedEvent.
func TestSellerDeletedPayload_EventTypeTagMatchesRealWire(t *testing.T) {
	wire, err := json.Marshal(mq.SellerDeletedEvent{
		BaseEvent: mq.NewBaseEvent(mq.EventTypeSellerDeleted, "seller-service"),
		AccountID: 777,
		StoreID:   100,
	})
	if err != nil {
		t.Fatalf("marshal real wire event: %v", err)
	}

	var payload mq.SellerDeletedEvent
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.EventType != mq.EventTypeSellerDeleted {
		t.Errorf("EventType = %q, want %q (payload's tag must match the real wire key event_type, not eventType)", payload.EventType, mq.EventTypeSellerDeleted)
	}
	if payload.AccountID != 777 {
		t.Errorf("AccountID = %d, want 777", payload.AccountID)
	}
	if payload.StoreID != 100 {
		t.Errorf("StoreID = %d, want 100", payload.StoreID)
	}
}

func TestSellerDeletedHandler_MalformedJSON_UsesBindTypedHandlerErrorFormat(t *testing.T) {
	msgUC := usecase.NewMessageUseCase(&fakeMsgRepo{}, &fakeConvRepo{}, nil, nil, nil, nil, nil, nil, nil, &config.Config{})
	handler := newSellerDeletedHandler(msgUC, zap.NewNop())

	err := handler(context.Background(), "seller.deleted", []byte("not json"))
	if err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ)")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("error = %q, want it to come from mq.BindTypedHandler's wrapped message (\"unmarshal event...\")", err.Error())
	}
}
