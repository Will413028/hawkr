package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

const (
	notificationChannelName    = "notification"
	notificationBatchEventName = "notification.batch"
)

// NotificationChannel implements Channel for per-user notification stream。
// 41 號設計書 §四。僅 web 訂閱、JWT auth、topic = notification:user:{inboxUserId}。
//
// ⚠️ `inboxUserId` **不是** JWT 帶的登入 principal：它是 `notifications.user_id`，
// 身分空間由 user_type 決定——買家＝當前 buyer_id；賣家＝當前 store_id。
// ResolveTopic 直接取事件 payload 的 userId（就是那一欄），所以訂閱端也必須用同一個
// canonical 空間解析（handler.StreamNotifications 走 resolveInboxUserIDInt）。
// 歷史 retired stint 不透過 account_id fallback 重新曝光，交由 lifecycle cleanup 處理。
type NotificationChannel struct{}

// NewNotificationChannel constructs the notification channel。
func NewNotificationChannel() *NotificationChannel {
	return &NotificationChannel{}
}

func (n *NotificationChannel) Name() string { return notificationChannelName }

func (n *NotificationChannel) Exchange() string { return pkgmq.ExchangeNotification }

func (n *NotificationChannel) RoutingKeys() []string {
	return []string{
		pkgmq.RoutingKeyNotificationInboxNew,
		pkgmq.RoutingKeyNotificationUnreadChanged,
	}
}

func (n *NotificationChannel) BatchEventName() string { return notificationBatchEventName }

// AuthorizeSubscribe 驗兩件事：訂閱主體必須是已登入的身分（accountID > 0），且 topic
// 必須是**該主體自己的**那一條。
//
// ⚠️ 第二項在今天唯一的呼叫點上是由建構方式保證的，不是它擋下來的：
// handler.StreamNotifications 的 topic 就是拿同一個 streamUserID 呼叫
// TopicNotificationUser 產生的，兩個參數同源 ⇒ 比對恆等，那條路徑上它不可能失敗。
// **今天真正在授權的是「streamUserID 由 JWT 推導」這件事本身。**
//
// 那為什麼留著：它守的是**下一個**呼叫點。一旦有人讓 topic 變成 caller 可控
// （?topic=、Last-Event-ID 夾帶 topic、新 endpoint 共用 serveSSE）而主體仍由 JWT 推導，
// 它就是唯一擋得住「訂閱別人的通知流」的地方。這條失敗分支沒有任何 handler 級路徑碰得到，
// 所以由 handler.TestNotificationChannel_AuthorizeSubscribeRejectsForeignTopic 直接守。
//
// 不要把這段註解縮回「驗呼叫者的身分」：它驗的是 serveSSE 兩個參數之間的一致性，
// 推不出身分是對的——兩個參數同源時，錯的身分會一起錯。
func (n *NotificationChannel) AuthorizeSubscribe(_ context.Context, topic string, accountID int64) error {
	if accountID <= 0 {
		return ErrForbidden
	}
	expected := TopicNotificationUser(accountID)
	if topic != expected {
		return ErrForbidden
	}
	return nil
}

type notificationTopicResolver struct {
	UserID int64 `json:"userId"`
}

func (n *NotificationChannel) ResolveTopic(_ string, payload []byte) (string, bool) {
	var p notificationTopicResolver
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", false
	}
	if p.UserID <= 0 {
		return "", false
	}
	return TopicNotificationUser(p.UserID), true
}

func (n *NotificationChannel) BuildSSEEvent(routingKey string, payload []byte) (string, []byte, error) {
	switch routingKey {
	case pkgmq.RoutingKeyNotificationInboxNew:
		var ev pkgmq.NotificationInboxNewEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("notification.inbox.new unmarshal: %w", err)
		}
		out := notificationNewSSE{
			Item:        ev.Item,
			UnreadCount: ev.UnreadCount,
		}
		data, err := json.Marshal(out)
		return "notification.new", data, err

	case pkgmq.RoutingKeyNotificationUnreadChanged:
		var ev pkgmq.NotificationUnreadChangedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("notification.unread_changed unmarshal: %w", err)
		}
		out := notificationUnreadChangedSSE{
			UnreadCount: ev.UnreadCount,
		}
		data, err := json.Marshal(out)
		return "notification.unread_changed", data, err
	}
	return "", nil, nil
}

// TopicNotificationUser is the canonical topic for a single user's notification stream。
// inboxUserID 是 notifications.user_id（買家＝當前 buyer_id；賣家＝當前 store_id），
// 不是 JWT 帶的登入 principal。
func TopicNotificationUser(inboxUserID int64) string {
	return fmt.Sprintf("notification:user:%d", inboxUserID)
}

// ---------------- SSE payload shapes ---------------- //

type notificationNewSSE struct {
	Item        map[string]interface{} `json:"item"`
	UnreadCount int64                  `json:"unreadCount"`
}

type notificationUnreadChangedSSE struct {
	UnreadCount int64 `json:"unreadCount"`
}
