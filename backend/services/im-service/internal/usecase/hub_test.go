package usecase

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Will413028/hawkr/im-service/internal/domain"
)

// mockConn implements domain.Connection for testing (no real connection).
type mockConn struct{}

func (m *mockConn) Close() error { return nil }

func newTestClient(userID int64, loginRole int32) *domain.Client {
	return &domain.Client{
		UserID:    userID,
		LoginRole: loginRole,
		Conn:      &mockConn{},
		Send:      make(chan []byte, 16),
	}
}

// drainSend reads all pending messages from Send channel without blocking.
func drainSend(c *domain.Client) []domain.Message {
	var msgs []domain.Message
	for {
		select {
		case data := <-c.Send:
			var msg domain.Message
			if err := json.Unmarshal(data, &msg); err == nil {
				msgs = append(msgs, msg)
			}
		default:
			return msgs
		}
	}
}

// Regression: 同帳號同時開 buyer app 和 seller app，
// buyer 收到的聊天訊息不該推到 seller app 的 WS 連線。
func TestRegression_SendToUserRole_OnlyMatchingRoleReceives(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	// Account 100 同時開了 buyer app (loginRole=1) 和 seller app (loginRole=2)
	buyerClient := newTestClient(100, 1)
	sellerClient := newTestClient(100, 2)
	hub.Register(buyerClient)
	hub.Register(sellerClient)

	// 等 register 完成
	time.Sleep(10 * time.Millisecond)

	msg := &domain.Message{Type: "chat", RoomID: "b100_s200", Content: "hello"}

	// 推送給 account 100 的 buyer 角色 (loginRole=1)
	hub.SendToUserRole(100, 1, msg)

	buyerMsgs := drainSend(buyerClient)
	sellerMsgs := drainSend(sellerClient)

	if len(buyerMsgs) != 1 {
		t.Errorf("buyer should receive 1 message, got %d", len(buyerMsgs))
	}
	if len(sellerMsgs) != 0 {
		t.Errorf("BUG: seller should NOT receive buyer's message, got %d", len(sellerMsgs))
	}

	// 推送給 account 100 的 seller 角色 (loginRole=2)
	hub.SendToUserRole(100, 2, msg)

	buyerMsgs = drainSend(buyerClient)
	sellerMsgs = drainSend(sellerClient)

	if len(buyerMsgs) != 0 {
		t.Errorf("BUG: buyer should NOT receive seller's message, got %d", len(buyerMsgs))
	}
	if len(sellerMsgs) != 1 {
		t.Errorf("seller should receive 1 message, got %d", len(sellerMsgs))
	}
}

func TestSendToUser_BroadcastsToAllRoles(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	buyerClient := newTestClient(100, 1)
	sellerClient := newTestClient(100, 2)
	hub.Register(buyerClient)
	hub.Register(sellerClient)
	time.Sleep(10 * time.Millisecond)

	msg := &domain.Message{Type: "system", Content: "broadcast"}

	// SendToUser (loginRole=0) 應推給所有連線
	hub.SendToUser(100, msg)

	buyerMsgs := drainSend(buyerClient)
	sellerMsgs := drainSend(sellerClient)

	if len(buyerMsgs) != 1 {
		t.Errorf("buyer should receive broadcast, got %d", len(buyerMsgs))
	}
	if len(sellerMsgs) != 1 {
		t.Errorf("seller should receive broadcast, got %d", len(sellerMsgs))
	}
}

// type=3 帳號的 WS 連線 (LoginRole=0) 應該收到所有角色的訊息
func TestRegression_Type3Client_ReceivesBothRoles(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	// Account 100 從 type=3 帳號連線，LoginRole=0（gateway 對 type=3 不設角色）
	type3Client := newTestClient(100, 0)
	hub.Register(type3Client)
	time.Sleep(10 * time.Millisecond)

	// 收到 buyer 角色的訊息
	hub.SendToUserRole(100, 1, &domain.Message{Type: "chat", Content: "buyer msg"})
	msgs := drainSend(type3Client)
	if len(msgs) != 1 {
		t.Errorf("type=3 client should receive buyer msg, got %d", len(msgs))
	}

	// 收到 seller 角色的訊息
	hub.SendToUserRole(100, 2, &domain.Message{Type: "chat", Content: "seller msg"})
	msgs = drainSend(type3Client)
	if len(msgs) != 1 {
		t.Errorf("type=3 client should receive seller msg, got %d", len(msgs))
	}
}

// broadcast 使用 RLock，不應阻塞 SendToUser
func TestBroadcast_DoesNotBlockSendToUser(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	c1 := newTestClient(1, 0)
	c2 := newTestClient(2, 0)
	hub.Register(c1)
	hub.Register(c2)
	time.Sleep(10 * time.Millisecond)

	// 同時 broadcast 和 SendToUser，不應 deadlock
	done := make(chan struct{})
	go func() {
		hub.Broadcast([]byte(`{"type":"system"}`))
		close(done)
	}()

	// SendToUser 應在 broadcast 期間也能正常執行
	sent := hub.SendToUser(1, &domain.Message{Type: "chat", Content: "direct"})
	if !sent {
		t.Error("SendToUser should succeed concurrently with broadcast")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("broadcast deadlocked — RLock optimization may be broken")
	}
}

// presence 訂閱：訂閱當下該 peer 尚未上線 → 收到 online=false snapshot；
// peer 的對應角色連線時 → 收到 online=true；最後一條該角色連線斷開 → 收到 online=false + lastSeenAt。
func TestPresence_SubscribeSnapshotAndTransitions(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	subscriber := newTestClient(10, 1) // buyer 訂閱賣家 (userID=20, role=2)
	hub.Register(subscriber)
	time.Sleep(10 * time.Millisecond)
	drainSend(subscriber) // 清掉自己上線產生的無關事件（無訂閱者不會有）

	peer := domain.PresenceKey{UserID: 20, LoginRole: 2}
	hub.SubscribePresence(subscriber, peer)

	msgs := drainSend(subscriber)
	if len(msgs) != 1 || msgs[0].Type != "presence" || msgs[0].Online == nil || *msgs[0].Online {
		t.Fatalf("expected snapshot offline event, got %+v", msgs)
	}

	// 賣家上線
	sellerClient := newTestClient(20, 2)
	hub.Register(sellerClient)
	time.Sleep(10 * time.Millisecond)

	msgs = drainSend(subscriber)
	if len(msgs) != 1 || msgs[0].Online == nil || !*msgs[0].Online || msgs[0].To != 20 || msgs[0].PeerRole != 2 {
		t.Fatalf("expected online=true event for (20,2), got %+v", msgs)
	}

	// 同帳號 buyer 角色再連線，不該觸發 (20,2) 的 presence
	buyerOfSamePeer := newTestClient(20, 1)
	hub.Register(buyerOfSamePeer)
	time.Sleep(10 * time.Millisecond)

	if msgs := drainSend(subscriber); len(msgs) != 0 {
		t.Fatalf("buyer-role register should not trigger seller-role presence, got %+v", msgs)
	}

	// 第二條賣家連線，也不該重複推 online
	seller2 := newTestClient(20, 2)
	hub.Register(seller2)
	time.Sleep(10 * time.Millisecond)
	if msgs := drainSend(subscriber); len(msgs) != 0 {
		t.Fatalf("second seller-role conn should not re-emit online, got %+v", msgs)
	}

	// 斷開第一條賣家，還有一條 → 不該推 offline
	hub.Unregister(sellerClient)
	time.Sleep(10 * time.Millisecond)
	if msgs := drainSend(subscriber); len(msgs) != 0 {
		t.Fatalf("one seller-role conn remains, should not emit offline, got %+v", msgs)
	}

	// 斷開最後一條賣家 → offline + lastSeenAt
	hub.Unregister(seller2)
	time.Sleep(10 * time.Millisecond)
	msgs = drainSend(subscriber)
	if len(msgs) != 1 || msgs[0].Online == nil || *msgs[0].Online {
		t.Fatalf("expected offline event, got %+v", msgs)
	}
	if msgs[0].LastSeenAt == 0 {
		t.Fatalf("expected lastSeenAt set on offline event, got %+v", msgs[0])
	}
}

// unsub 後不再收到 presence 更新。
func TestPresence_UnsubscribeStopsEvents(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	sub := newTestClient(30, 1)
	hub.Register(sub)
	time.Sleep(10 * time.Millisecond)

	peer := domain.PresenceKey{UserID: 40, LoginRole: 2}
	hub.SubscribePresence(sub, peer)
	drainSend(sub) // 清 snapshot

	hub.UnsubscribePresence(sub, peer)

	seller := newTestClient(40, 2)
	hub.Register(seller)
	time.Sleep(10 * time.Millisecond)

	if msgs := drainSend(sub); len(msgs) != 0 {
		t.Fatalf("after unsubscribe, no events expected, got %+v", msgs)
	}
}

// 訂閱者自己斷線時，presenceSubs 中的引用要被清掉，避免後續 send 到已 close 的 channel。
func TestPresence_SubscriberDisconnectCleansSubs(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	sub := newTestClient(50, 1)
	hub.Register(sub)
	time.Sleep(10 * time.Millisecond)

	peer := domain.PresenceKey{UserID: 60, LoginRole: 2}
	hub.SubscribePresence(sub, peer)
	hub.Unregister(sub)
	time.Sleep(10 * time.Millisecond)

	// peer 上線，若 sub 未清會 panic（send on closed channel）
	seller := newTestClient(60, 2)
	hub.Register(seller)
	time.Sleep(10 * time.Millisecond)

	hub.mu.RLock()
	_, still := hub.presenceSubs[peer]
	hub.mu.RUnlock()
	if still {
		t.Fatal("presenceSubs entry should be removed after subscriber disconnects")
	}
}

// ==================== Typing relay ====================

// Regression: typing 事件應被轉發給目標 user，不能靜默丟掉或廣播給所有人。
func TestTypingRelay_DeliveredToTargetOnly(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	buyer := newTestClient(100, 1)
	seller := newTestClient(200, 2)
	other := newTestClient(300, 1)
	hub.Register(buyer)
	hub.Register(seller)
	hub.Register(other)
	time.Sleep(10 * time.Millisecond)

	// buyer 送 typing → seller
	typingMsg := &domain.Message{Type: "typing", From: 100, To: 200}
	hub.SendToUser(200, typingMsg)

	sellerMsgs := drainSend(seller)
	buyerMsgs := drainSend(buyer)
	otherMsgs := drainSend(other)

	if len(sellerMsgs) != 1 || sellerMsgs[0].Type != "typing" {
		t.Errorf("seller should receive 1 typing event, got %+v", sellerMsgs)
	}
	if len(buyerMsgs) != 0 {
		t.Errorf("BUG: buyer (sender) should NOT receive own typing, got %d", len(buyerMsgs))
	}
	if len(otherMsgs) != 0 {
		t.Errorf("BUG: unrelated client should NOT receive typing, got %d", len(otherMsgs))
	}
}

// stop_typing 與 typing 相同轉發行為
func TestStopTypingRelay_DeliveredToTarget(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	seller := newTestClient(200, 2)
	hub.Register(seller)
	time.Sleep(10 * time.Millisecond)

	hub.SendToUser(200, &domain.Message{Type: "stop_typing", From: 100, To: 200})

	msgs := drainSend(seller)
	if len(msgs) != 1 || msgs[0].Type != "stop_typing" {
		t.Errorf("seller should receive stop_typing, got %+v", msgs)
	}
}

// ==================== Typing rate limiter ====================

func TestAllowTypingRate_FirstEventAlwaysAllowed(t *testing.T) {
	lastSent := make(map[int64]time.Time)
	if !allowTypingRate(lastSent, 200) {
		t.Error("first typing event should be allowed")
	}
}

func TestAllowTypingRate_BlocksWithinWindow(t *testing.T) {
	lastSent := make(map[int64]time.Time)
	allowTypingRate(lastSent, 200) // first — allowed
	if allowTypingRate(lastSent, 200) {
		t.Error("second event within window should be blocked")
	}
}

func TestAllowTypingRate_AllowsAfterWindow(t *testing.T) {
	lastSent := make(map[int64]time.Time)
	lastSent[200] = time.Now().Add(-typingRateWindow - time.Millisecond) // 剛好過期
	if !allowTypingRate(lastSent, 200) {
		t.Error("event after window should be allowed")
	}
}

func TestAllowTypingRate_DifferentTargetsAreIndependent(t *testing.T) {
	lastSent := make(map[int64]time.Time)
	allowTypingRate(lastSent, 200) // to=200 佔用
	if !allowTypingRate(lastSent, 201) {
		t.Error("different target should have independent rate limit")
	}
}

// Send channel 滿時不應 close 連線（只 log 丟訊息）
func TestBroadcast_FullChannel_DoesNotCloseConnection(t *testing.T) {
	hub := NewMessageHub()
	go hub.Run()

	// 建立 buffer=1 的 client，容易塞滿
	c := &domain.Client{
		UserID:    1,
		LoginRole: 0,
		Conn:      &mockConn{},
		Send:      make(chan []byte, 1),
	}
	hub.Register(c)
	time.Sleep(10 * time.Millisecond)

	// 塞滿 Send channel
	c.Send <- []byte(`{"type":"fill"}`)

	// Broadcast 時 channel 已滿，訊息應被丟棄但連線不應被關閉
	hub.Broadcast([]byte(`{"type":"overflow"}`))
	time.Sleep(10 * time.Millisecond)

	// 驗證連線仍然活著：可以 SendToUser
	// 先清空 channel
	<-c.Send
	sent := hub.SendToUser(1, &domain.Message{Type: "ping"})
	if !sent {
		t.Error("client should still be connected after broadcast overflow")
	}
}
