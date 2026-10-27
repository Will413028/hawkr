package worker

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

const (
	finalSellerStoreID = int64(7001)
	finalSellerToken   = "TOKEN-FOR-STORE-7001"
)

func finalSellerTemplate() *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:               900,
		Exchange:         "hawkr.order",
		RoutingKey:       "order.test_push_identity",
		EventType:        "order.test_push_identity",
		TargetUserType:   "seller",
		TargetUserField:  "store_id",
		Category:         "order_update",
		TitleTemplate:    "測試通知",
		BodyTemplate:     "測試內容",
		DataType:         "TEST",
		Enabled:          true,
		PushMode:         "visible",
		IdentityMigrated: true,
	}
}

func TestHandleTemplate_SellerTargetUsesStoreIDForInboxAndPush(t *testing.T) {
	cache := buildTestCache([]*domain.NotificationTemplate{finalSellerTemplate()})
	tokens := &keyedTokenRepo{tokensByUser: map[int64][]*domain.DeviceToken{
		finalSellerStoreID: {{Token: finalSellerToken}},
	}}
	prefs := &keyedPrefRepo{prefs: map[int64]map[string]*domain.NotificationPreference{}}
	sendPush, pushRepo, notifier := newKeyedSendPush(tokens, prefs)
	inboxUC, inboxRepo := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC,
		"hawkr.order", "https://test-media.example.com", zap.NewNop())

	body, _ := json.Marshal(makePayload("store_id", finalSellerStoreID))
	if err := handler(context.Background(), "order.test_push_identity", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if inboxRepo.createdItem == nil || inboxRepo.createdItem.UserID != finalSellerStoreID {
		t.Fatalf("inbox user_id=%+v, want %d", inboxRepo.createdItem, finalSellerStoreID)
	}
	if pushRepo.record == nil || pushRepo.record.UserID != finalSellerStoreID {
		t.Fatalf("push record=%+v, want user_id=%d", pushRepo.record, finalSellerStoreID)
	}
	if len(notifier.calls) != 1 || len(notifier.calls[0].tokens) != 1 || notifier.calls[0].tokens[0] != finalSellerToken {
		t.Fatalf("notifier calls=%+v, want token %q", notifier.calls, finalSellerToken)
	}
}

func TestHandleTemplate_SellerPreferenceUsesStoreID(t *testing.T) {
	cache := buildTestCache([]*domain.NotificationTemplate{finalSellerTemplate()})
	tokens := &keyedTokenRepo{tokensByUser: map[int64][]*domain.DeviceToken{
		finalSellerStoreID: {{Token: finalSellerToken}},
	}}
	prefs := &keyedPrefRepo{prefs: map[int64]map[string]*domain.NotificationPreference{
		finalSellerStoreID: {
			"order_update": {
				UserID:   finalSellerStoreID,
				UserType: "seller",
				Category: "order_update",
				Enabled:  false,
			},
		},
	}}
	sendPush, pushRepo, notifier := newKeyedSendPush(tokens, prefs)
	inboxUC, _ := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC,
		"hawkr.order", "https://test-media.example.com", zap.NewNop())

	body, _ := json.Marshal(makePayload("store_id", finalSellerStoreID))
	if err := handler(context.Background(), "order.test_push_identity", body); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("notifier calls=%d, want 0 for disabled store preference", len(notifier.calls))
	}
	if pushRepo.record != nil {
		t.Fatalf("push record=%+v, want nil when category is disabled", pushRepo.record)
	}
}
