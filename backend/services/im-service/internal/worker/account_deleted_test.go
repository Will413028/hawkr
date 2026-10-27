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

// mustMarshalAccountDeleted builds an account.deleted wire body matching
// mq.AccountDeletedEvent's json tags (event_type/accountId). Built as a map,
// not the struct, so this test doesn't take on a hidden compile-time
// assumption about the payload shape beyond what the handler itself decodes.
func mustMarshalAccountDeleted(t *testing.T, accountID int64) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"event_type": "ACCOUNT_DELETED",
		"accountId":  accountID,
	})
	if err != nil {
		t.Fatalf("marshal test event: %v", err)
	}
	return body
}

func TestAccountDeletedHandler_PurgesBySenderAccountID(t *testing.T) {
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
	handler := newAccountDeletedHandler(msgUC, zap.NewNop())

	body := mustMarshalAccountDeleted(t, 777)
	if err := handler(context.Background(), "account.deleted", body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(gotSenders) != 1 || gotSenders[0] != 777 {
		t.Errorf("RedactBySender senderIDs = %v, want [777] (payload.AccountID)", gotSenders)
	}
}

func TestAccountDeletedHandler_MissingAccountID_ReturnsError(t *testing.T) {
	msgUC := usecase.NewMessageUseCase(&fakeMsgRepo{}, &fakeConvRepo{}, nil, nil, nil, nil, nil, nil, nil, &config.Config{})
	handler := newAccountDeletedHandler(msgUC, zap.NewNop())

	body, _ := json.Marshal(map[string]interface{}{"event_type": "ACCOUNT_DELETED"})
	if err := handler(context.Background(), "account.deleted", body); err == nil {
		t.Fatal("expected error for missing accountId, got nil")
	}
}

func TestAccountDeletedHandler_PropagatesRedactError(t *testing.T) {
	msgRepo := &fakeMsgRepo{
		RedactBySenderFn: func(_ context.Context, _ []int64) (int64, error) {
			return 0, context.DeadlineExceeded
		},
	}
	buyerClient := &fakeBuyerClient{}
	sellerClient := &fakeSellerClient{}
	msgUC := usecase.NewMessageUseCase(msgRepo, &fakeConvRepo{}, nil, buyerClient, sellerClient, nil, nil, nil, nil, &config.Config{})
	handler := newAccountDeletedHandler(msgUC, zap.NewNop())

	body := mustMarshalAccountDeleted(t, 777)
	if err := handler(context.Background(), "account.deleted", body); err == nil {
		t.Fatal("expected error propagated from RedactDeletedAccount, got nil")
	}
}

// TestAccountDeletedPayload_EventTypeTagMatchesRealWire guards against the
// wire-tag bug this task fixes: accountDeletedPayload tagged EventType as
// json:"eventType", but the real publisher (account-service) emits
// event_type on the wire (pkg/mq/events.go's BaseEvent, line 84).
func TestAccountDeletedPayload_EventTypeTagMatchesRealWire(t *testing.T) {
	wire, err := json.Marshal(mq.AccountDeletedEvent{
		BaseEvent: mq.NewBaseEvent(mq.EventTypeAccountDeleted, "account-service"),
		AccountID: 777,
	})
	if err != nil {
		t.Fatalf("marshal real wire event: %v", err)
	}

	var payload mq.AccountDeletedEvent
	if err := json.Unmarshal(wire, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload.EventType != mq.EventTypeAccountDeleted {
		t.Errorf("EventType = %q, want %q (payload's tag must match the real wire key event_type, not eventType)", payload.EventType, mq.EventTypeAccountDeleted)
	}
	if payload.AccountID != 777 {
		t.Errorf("AccountID = %d, want 777", payload.AccountID)
	}
}

func TestAccountDeletedHandler_MalformedJSON_UsesBindTypedHandlerErrorFormat(t *testing.T) {
	msgUC := usecase.NewMessageUseCase(&fakeMsgRepo{}, &fakeConvRepo{}, nil, nil, nil, nil, nil, nil, nil, &config.Config{})
	handler := newAccountDeletedHandler(msgUC, zap.NewNop())

	err := handler(context.Background(), "account.deleted", []byte("not json"))
	if err == nil {
		t.Fatal("malformed JSON should return err (framework retries → DLQ)")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("error = %q, want it to come from mq.BindTypedHandler's wrapped message (\"unmarshal event...\")", err.Error())
	}
}
