package worker

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"go.uber.org/zap"

	"github.com/Will413028/hawkr/im-service/internal/domain"
)

// stubHub is a minimal domain.Hub fake recording DisconnectUser calls and
// simulating "first call disconnects, replay is a no-op" (mirrors the real
// MessageHub.DisconnectUser semantics: delete-from-map, second delete is a miss).
type stubHub struct {
	connected map[int64]bool
	calls     []int64
}

func newStubHub(connectedUserID int64) *stubHub {
	return &stubHub{connected: map[int64]bool{connectedUserID: true}}
}

func (h *stubHub) Run()                                                  {}
func (h *stubHub) Register(client *domain.Client)                        {}
func (h *stubHub) Unregister(client *domain.Client)                      {}
func (h *stubHub) SendToUser(userID int64, message *domain.Message) bool { return false }
func (h *stubHub) SendToUserRole(userID int64, role int32, message *domain.Message) bool {
	return false
}
func (h *stubHub) Broadcast(message []byte)                                            {}
func (h *stubHub) BroadcastExcept(message *domain.Message, exclude map[int64]bool) int { return 0 }
func (h *stubHub) IsOnline(userID int64) bool                                          { return h.connected[userID] }
func (h *stubHub) ConnectionCount() int                                                { return len(h.connected) }
func (h *stubHub) DisconnectUser(userID int64) bool {
	h.calls = append(h.calls, userID)
	if !h.connected[userID] {
		return false
	}
	delete(h.connected, userID)
	return true
}
func (h *stubHub) SubscribePresence(client *domain.Client, peer domain.PresenceKey)   {}
func (h *stubHub) UnsubscribePresence(client *domain.Client, peer domain.PresenceKey) {}

type stubResolver struct {
	resolved int64
}

func (r *stubResolver) CurrentStoreID(ctx context.Context, accountID int64) int64 {
	if r.resolved != 0 {
		return r.resolved
	}
	return accountID
}

func mustMarshalSellerSuspended(t *testing.T, accountID, suspensionID int64, reasonCode string) []byte {
	t.Helper()
	// Mirrors pkg/mq.SellerSuspendedEvent's wire shape (accountId/suspensionId as
	// JSON strings — TSID precision; see docs/開發規範/Java 微服務開發規範.md §2).
	payload := map[string]interface{}{
		"event_id":     "evt1",
		"event_type":   "SELLER_SUSPENDED",
		"suspensionId": "123",
		"accountId":    "",
		"reasonCode":   reasonCode,
	}
	payload["accountId"] = strconv.FormatInt(accountID, 10)
	payload["suspensionId"] = strconv.FormatInt(suspensionID, 10)
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal test event: %v", err)
	}
	return b
}

func TestSellerSuspendedHandler_DisconnectsWSAtHubKey(t *testing.T) {
	hub := newStubHub(100)
	resolver := &stubResolver{resolved: 100}
	handler := newSellerSuspendedHandler(hub, resolver, zap.NewNop())

	body := mustMarshalSellerSuspended(t, 7, 55, "9601")
	if err := handler(context.Background(), "seller.suspended", body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hub.IsOnline(100) {
		t.Error("expected hub key 100 (resolved surrogate) to be disconnected")
	}
	if len(hub.calls) != 1 || hub.calls[0] != 100 {
		t.Errorf("DisconnectUser calls = %v, want [100]", hub.calls)
	}
}

// TestSellerSuspendedHandler_Idempotent_ReplayIsNoOp: a replayed / re-driven
// event (or duplicate MQ delivery) must not error and must be a harmless no-op
// on the second call (36 §3.6f: 重送 0 副作用).
func TestSellerSuspendedHandler_Idempotent_ReplayIsNoOp(t *testing.T) {
	hub := newStubHub(100)
	resolver := &stubResolver{resolved: 100}
	handler := newSellerSuspendedHandler(hub, resolver, zap.NewNop())

	body := mustMarshalSellerSuspended(t, 7, 55, "9601")
	if err := handler(context.Background(), "seller.suspended", body); err != nil {
		t.Fatalf("first call: unexpected error: %v", err)
	}
	if err := handler(context.Background(), "seller.suspended", body); err != nil {
		t.Fatalf("replayed call: unexpected error: %v", err)
	}
	if len(hub.calls) != 2 {
		t.Fatalf("expected 2 DisconnectUser calls (first real, second no-op), got %d", len(hub.calls))
	}
}

func TestSellerSuspendedHandler_NoConnection_IsNoOp(t *testing.T) {
	hub := &stubHub{connected: map[int64]bool{}} // nobody connected
	resolver := &stubResolver{resolved: 100}
	handler := newSellerSuspendedHandler(hub, resolver, zap.NewNop())

	body := mustMarshalSellerSuspended(t, 7, 55, "9601")
	if err := handler(context.Background(), "seller.suspended", body); err != nil {
		t.Fatalf("unexpected error disconnecting a non-connected user: %v", err)
	}
}

func TestSellerSuspendedHandler_MissingAccountID_ReturnsError(t *testing.T) {
	hub := newStubHub(100)
	resolver := &stubResolver{}
	handler := newSellerSuspendedHandler(hub, resolver, zap.NewNop())

	body, _ := json.Marshal(map[string]interface{}{"event_type": "SELLER_SUSPENDED"})
	if err := handler(context.Background(), "seller.suspended", body); err == nil {
		t.Fatal("expected error for missing accountId, got nil")
	}
}
