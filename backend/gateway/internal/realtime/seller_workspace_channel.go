package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	pkgmq "github.com/Will413028/hawkr/go-pkg/mq"
)

const sellerWorkspaceChannelName = "seller_workspace"

// SellerWorkspaceChannel implements Channel for the seller web workspace
// (賣家中心訂單管理頁) realtime invalidation events. 36 §3.6i / 41 §四之二，[B-207] #1181.
//
// 設計重點（與 comment / notification channel 不同）：
//   - **Publish-only channel**：本 channel 不對應任何 SSE endpoint。事件解析到的 topic
//     與 NotificationChannel 同一個 notification:user:{storeId}，因此 seller workspace
//     事件落在賣家「已開著」的 /app/notifications/stream 連線上（issue 要求不開第二條 stream）。
//   - 🔴 **topic 的身分空間是 store_id，不是 login account_id**。訂閱側由
//     notification_inbox.go#resolveNotificationUserID 建（2026-08-30、commit 57cefff9f 起
//     對賣家回傳 current store_id）；發布側由本檔 ResolveTopic 直接讀 payload 的 sellerId。
//     兩個 producer（order-service / aftersales-service）因此必須把 store_id 原樣放進
//     payload.sellerId —— 它們原本會先反解析成 account_id（理由是「SSE topic 是 account_id」，
//     該前提在上述 commit 失效），造成對 store_id != account_id 的賣家 100% 不投遞，
//     而 Hub 以精確鍵 fan-out ⇒ 查無訂閱者不產生任何錯誤或 log。2026-09-03 已修。
//   - **BatchEventName 復用 notification.batch**：與 notification 事件在同 (topic, batch)
//     窗口合併，前端既有 notification.batch 解包邏輯直接處理混合內層事件。
//   - **AuthorizeSubscribe 永不被呼叫**：訂閱授權由 NotificationChannel 在
//     /app/notifications/stream 端處理（topic 的 id 由 JWT accountId 經
//     resolveNotificationUserID 解析而來，賣家為 current store_id）；本 channel
//     只走 Registry.Dispatch（publish 側）。防禦性回 ErrForbidden 拒絕任何直接訂閱。
type SellerWorkspaceChannel struct{}

// NewSellerWorkspaceChannel constructs the seller workspace channel.
func NewSellerWorkspaceChannel() *SellerWorkspaceChannel {
	return &SellerWorkspaceChannel{}
}

func (s *SellerWorkspaceChannel) Name() string { return sellerWorkspaceChannelName }

func (s *SellerWorkspaceChannel) Exchange() string { return pkgmq.ExchangeSellerWorkspace }

func (s *SellerWorkspaceChannel) RoutingKeys() []string {
	return []string{
		pkgmq.RoutingKeySellerOrderUpdated,
		pkgmq.RoutingKeySellerAfterSaleUpdated,
	}
}

// BatchEventName reuses notification.batch so seller events merge with notification
// events under one wrapper on the same topic (41 §4b.2)。
func (s *SellerWorkspaceChannel) BatchEventName() string { return notificationBatchEventName }

// AuthorizeSubscribe — this channel has no endpoint; direct subscribe is never allowed.
func (s *SellerWorkspaceChannel) AuthorizeSubscribe(_ context.Context, _ string, _ int64) error {
	return ErrForbidden
}

// sellerWorkspaceTopicResolver extracts sellerId from either event payload。
type sellerWorkspaceTopicResolver struct {
	SellerID int64 `json:"sellerId"`
}

// ResolveTopic maps both routing keys to the seller's notification topic so events
// land on the seller's already-open /app/notifications/stream connection。
func (s *SellerWorkspaceChannel) ResolveTopic(_ string, payload []byte) (string, bool) {
	var p sellerWorkspaceTopicResolver
	if err := json.Unmarshal(payload, &p); err != nil {
		return "", false
	}
	if p.SellerID <= 0 {
		return "", false
	}
	return TopicNotificationUser(p.SellerID), true
}

func (s *SellerWorkspaceChannel) BuildSSEEvent(routingKey string, payload []byte) (string, []byte, error) {
	switch routingKey {
	case pkgmq.RoutingKeySellerOrderUpdated:
		var ev pkgmq.SellerOrderUpdatedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("seller.order.updated unmarshal: %w", err)
		}
		out := sellerOrderUpdatedSSE{
			EventID:        ev.EventID,
			SellerID:       strconv.FormatInt(ev.SellerID, 10),
			OrderNo:        ev.OrderNo,
			Source:         ev.Source,
			PreviousStatus: ev.PreviousStatus,
			Status:         ev.Status,
			ShipmentStatus: ev.ShipmentStatus,
			ChangedAt:      ev.ChangedAt,
		}
		data, err := json.Marshal(out)
		return "seller.order.updated", data, err

	case pkgmq.RoutingKeySellerAfterSaleUpdated:
		var ev pkgmq.SellerAfterSaleUpdatedEvent
		if err := json.Unmarshal(payload, &ev); err != nil {
			return "", nil, fmt.Errorf("seller.after_sale.updated unmarshal: %w", err)
		}
		out := sellerAfterSaleUpdatedSSE{
			EventID:         ev.EventID,
			SellerID:        strconv.FormatInt(ev.SellerID, 10),
			OrderNo:         ev.OrderNo,
			AfterSaleCaseNo: ev.AfterSaleCaseNo,
			Source:          ev.Source,
			Status:          ev.Status,
			ChangedAt:       ev.ChangedAt,
		}
		data, err := json.Marshal(out)
		return "seller.after_sale.updated", data, err
	}
	return "", nil, nil
}

// ---- SSE payload shapes（sellerId 序列化為 string；對外數字一律 string，見
// docs/開發規範/Java 微服務開發規範.md §2） ---- //

type sellerOrderUpdatedSSE struct {
	EventID        string `json:"eventId"`
	SellerID       string `json:"sellerId"`
	OrderNo        string `json:"orderNo"`
	Source         string `json:"source"`
	PreviousStatus string `json:"previousStatus,omitempty"`
	Status         string `json:"status,omitempty"`
	ShipmentStatus string `json:"shipmentStatus,omitempty"`
	ChangedAt      string `json:"changedAt"`
}

type sellerAfterSaleUpdatedSSE struct {
	EventID         string `json:"eventId"`
	SellerID        string `json:"sellerId"`
	OrderNo         string `json:"orderNo"`
	AfterSaleCaseNo string `json:"afterSaleCaseNo"`
	Source          string `json:"source"`
	Status          string `json:"status,omitempty"`
	ChangedAt       string `json:"changedAt"`
}
