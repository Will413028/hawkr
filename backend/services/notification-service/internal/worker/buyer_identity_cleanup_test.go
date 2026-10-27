package worker

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/notification-service/internal/domain"
)

func restockBuyerTemplate() *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:                  991,
		Exchange:            "hawkr.product",
		RoutingKey:          "product.restock.target",
		EventType:           "product.restock.target",
		TargetUserType:      "buyer",
		TargetUserField:     "buyer_id",
		Category:            "restock",
		TitleTemplate:       "補貨通知",
		BodyTemplate:        "商品已補貨",
		DataType:            "RESTOCK_AVAILABLE",
		ReferenceIDTemplate: strPtr("restock_#{subscription_id}"),
		DataFields: map[string]domain.FieldSpec{
			"subscription_id": fieldSpec("subscription_id"),
		},
		Enabled: true,
	}
}

func TestTemplateHandler_RestockUsesCanonicalBuyerID(t *testing.T) {
	assertRestockTargetID(t, map[string]interface{}{
		"buyer_id":        float64(7001),
		"subscription_id": "sub-1",
	}, 7001)
}

func TestTemplateHandler_RestockRejectsRetiredBuyerAccountAlias(t *testing.T) {
	cache := buildTestCache([]*domain.NotificationTemplate{restockBuyerTemplate()})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC,
		"hawkr.product", "https://test-media.example.com", zap.NewNop())

	body, err := json.Marshal(map[string]interface{}{
		"buyer_account_id": float64(7002),
		"subscription_id":  "sub-2",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := handler(context.Background(), "product.restock.target", body); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if inboxRepo.createdItem != nil {
		t.Fatalf("inbox item=%+v, want nil for retired buyer_account_id alias", inboxRepo.createdItem)
	}
	if pushRepo.record != nil {
		t.Fatalf("push record=%+v, want nil for retired buyer_account_id alias", pushRepo.record)
	}
}

func refundCompletedBuyerTemplate() *domain.NotificationTemplate {
	return &domain.NotificationTemplate{
		ID:                  992,
		Exchange:            "payment.events",
		RoutingKey:          "payment.rescue.refund_completed",
		EventType:           "payment.rescue.refund_completed",
		TargetUserType:      "buyer",
		TargetUserField:     "buyer_id",
		Category:            "refund_update",
		TitleTemplate:       "退款完成",
		BodyTemplate:        "退款已完成",
		DataType:            "ORDER_REFUNDED",
		ReferenceIDTemplate: strPtr("rescue_refund_#{trade_no}"),
		DataFields: map[string]domain.FieldSpec{
			"refId": fieldSpec("orderNo"),
		},
		Enabled: true,
	}
}

func TestTemplateHandler_RefundCompletedUsesCanonicalBuyerID(t *testing.T) {
	tokens := &keyedTokenRepo{tokensByUser: map[int64][]*domain.DeviceToken{
		7101: {{Token: "TOKEN-FOR-BUYER-7101"}},
	}}
	prefs := &keyedPrefRepo{prefs: map[int64]map[string]*domain.NotificationPreference{}}
	sendPush, pushRepo, notifier := newKeyedSendPush(tokens, prefs)
	inboxUC, inboxRepo := newTestInboxUC()
	handler := newTemplateHandler(
		buildTestCache([]*domain.NotificationTemplate{refundCompletedBuyerTemplate()}),
		sendPush,
		inboxUC,
		"payment.events",
		"https://test-media.example.com",
		zap.NewNop(),
	)

	// Deliberately make the retired key differ. This fixture proves the template
	// reads the canonical key and never falls back to buyerAccountId.
	body, err := json.Marshal(map[string]interface{}{
		"buyer_id":       float64(7101),
		"buyerAccountId": float64(7102),
		"orderNo":        "ORD-REFUND-001",
		"trade_no":       "TRADE-REFUND-001",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := handler(context.Background(), "payment.rescue.refund_completed", body); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if inboxRepo.createdItem == nil || inboxRepo.createdItem.UserID != 7101 {
		t.Fatalf("inbox user_id=%+v, want 7101", inboxRepo.createdItem)
	}
	if pushRepo.record == nil || pushRepo.record.UserID != 7101 {
		t.Fatalf("push record=%+v, want user_id=7101", pushRepo.record)
	}
	if len(notifier.calls) != 1 || len(notifier.calls[0].tokens) != 1 || notifier.calls[0].tokens[0] != "TOKEN-FOR-BUYER-7101" {
		t.Fatalf("notifier calls=%+v, want canonical buyer token", notifier.calls)
	}
}

func TestTemplateHandler_RefundCompletedRequiresCanonicalBuyerID(t *testing.T) {
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()
	handler := newTemplateHandler(
		buildTestCache([]*domain.NotificationTemplate{refundCompletedBuyerTemplate()}),
		sendPush,
		inboxUC,
		"payment.events",
		"https://test-media.example.com",
		zap.NewNop(),
	)

	body, err := json.Marshal(map[string]interface{}{
		"buyerAccountId": float64(7102),
		"orderNo":        "ORD-REFUND-002",
		"trade_no":       "TRADE-REFUND-002",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := handler(context.Background(), "payment.rescue.refund_completed", body); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if inboxRepo.createdItem != nil {
		t.Fatalf("inbox item=%+v, want nil when canonical buyer_id is missing", inboxRepo.createdItem)
	}
	if pushRepo.record != nil {
		t.Fatalf("push record=%+v, want nil when canonical buyer_id is missing", pushRepo.record)
	}
}

func assertRestockTargetID(t *testing.T, payload map[string]interface{}, want int64) {
	t.Helper()

	cache := buildTestCache([]*domain.NotificationTemplate{restockBuyerTemplate()})
	sendPush, pushRepo := newTestSendPushWithRepo()
	inboxUC, inboxRepo := newTestInboxUC()
	handler := newTemplateHandler(cache, sendPush, inboxUC,
		"hawkr.product", "https://test-media.example.com", zap.NewNop())

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if err := handler(context.Background(), "product.restock.target", body); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if inboxRepo.createdItem == nil {
		t.Fatal("expected inbox item")
	}
	if inboxRepo.createdItem.UserID != want {
		t.Fatalf("inbox user_id=%d, want %d", inboxRepo.createdItem.UserID, want)
	}
	if pushRepo.record == nil {
		t.Fatal("expected push record")
	}
	if pushRepo.record.UserID != want {
		t.Fatalf("push user_id=%d, want %d", pushRepo.record.UserID, want)
	}
}
