# 通知微服務 (Notification Service) 詳細設計書

## 1. 概述 (Overview)

本文件描述 **Notification Service** 的詳細設計，涵蓋買家端與賣家端的完整推播通知方案。

服務透過 **RabbitMQ 事件驅動** 接收各微服務發布的領域事件，經冪等檢查與偏好設定過濾後，透過 **Firebase Cloud Messaging (FCM)** 推播至 Android / iOS 裝置。

---

## 2. 系統架構 (System Architecture)

### 2.1 環境與通訊

| 項目 | 值 |
|---|---|
| Language | Go 1.25 |
| DI Framework | Uber Fx |
| Database | PostgreSQL (`hawkr_notification`) |
| Message Broker | RabbitMQ (Topic Exchange) |
| Push Service | Firebase Cloud Messaging (FCM) |
| gRPC Port | 10037 |
| Management / health transport | gRPC `grpc.health.v1.Health` only（aggregate `""` + named `hawkr.notification.NotificationService`） |
| Metrics | OTLP push to Collector（不暴露 `/metrics`） |

### 2.2 事件驅動架構

```
┌──────────────┐   order.paid / order.*         ┌─────────────────────┐
│ order-       │───────────────────────────────▶│                     │
│ service      │                                │                     │
├──────────────┤   merchant.verification.*      │                     │
│ seller-      │───────────────────────────────▶│                     │
│ service      │                                │                     │
├──────────────┤   post.created / comment.*     │                     │
│ social-      │───────────────────────────────▶│                     │
│ service      │                                │  notification-      │    ┌─────────┐
├──────────────┤   follow.added                 │  service             │───▶│  FCM    │
│ buyer-       │───────────────────────────────▶│  (Go)               │    └────┬────┘
│ service      │                                │                     │         │
├──────────────┤   coupon.issued                │                     │    ┌────▼────┐
│ coupon-      │───────────────────────────────▶│                     │    │ Buyer / │
│ service      │                                │                     │    │ Seller  │
├──────────────┤                                │                     │    │  App    │
│ admin /      │   BroadcastAnnouncement (gRPC) │                     │    └─────────┘
│ gateway      │───────────────────────────────▶│                     │
├──────────────┤                                │                     │
│ im-service   │   chat.message.sent (MQ)       │                     │
│              │───────────────────────────────▶│                     │
├──────────────┤   RegisterDeviceToken /        │                     │
│ gateway      │   RemoveDeviceToken (gRPC)     │                     │
│              │───────────────────────────────▶│                     │
└──────────────┘                                └─────────────────────┘
```

### 2.3 目錄結構

```
services/notification-service/
├── cmd/notification-service/main.go
├── cmd/grpc-healthcheck/main.go
├── db/
│   ├── migrations/
│   ├── queries/
│   └── schema/
└── internal/
    ├── app/app.go                        # Fx application 組裝
    ├── config/config.go                  # Config loading + Validate
    ├── domain/
    │   ├── device_token.go               # DeviceToken entity + repo interface
    │   ├── push_notification.go          # PushNotificationRecord entity
    │   ├── notification_preference.go    # 偏好設定 entity + category 定義
    │   ├── notification_template.go      # NotificationTemplate entity + repo interface
    │   └── system_notification_log.go    # 系統通知（廣播）留痕
    ├── infrastructure/firebase.go        # FCM implementation (PushNotifier)
    ├── client/{buyer,seller}/            # 下游 gRPC client（粉絲列表、賣家資料）
    ├── grpc/
    │   ├── server.go                     # 偏好設定 + RegisterDeviceToken/RemoveDeviceToken
    │   ├── server_admin.go               # Admin 面 RPC（template 管理）
    │   ├── server_inbox.go               # 站內信 inbox 讀取／已讀
    │   └── server_broadcast.go           # BroadcastAnnouncement
    ├── mq/publisher.go                   # 本服務對外發布事件
    ├── repository/postgres/              # Repository implementations (sqlc)
    ├── usecase/
    │   ├── dto/device_token.go           # SendPushRequest DTO
    │   ├── register_token.go
    │   ├── remove_token.go
    │   ├── send_push.go                  # 核心推播邏輯 (冪等 + 偏好 + FCM)
    │   ├── notification_inbox.go         # 站內信讀寫
    │   └── notification_preference.go    # 偏好設定 CRUD
    └── worker/
        ├── di.go                         # Fx module（啟動 consumer）
        ├── consumer.go                   # RabbitMQ subscription 定義（queue ↔ exchange SSOT）
        ├── template_handler.go           # 🔴 唯一的事件處理路徑：DB-driven template dispatch
        ├── template_cache.go             # notification_templates 記憶體快取
        ├── cleanup.go                    # 過期推播紀錄／站內信清理
        ├── account_deleted.go            # 帳號刪除：清除 device token 與站內信
        ├── seller_deleted.go             # 賣家刪除：同上（賣家軸）
        └── vendor_invite.go              # 供應商邀請（非 template 路徑，需外部 client 組資料）
```

🔴 **事件處理沒有 per-event handler 檔。** 除 `account_deleted.go`／`seller_deleted.go`／
`vendor_invite.go` 三支需要額外資料查詢的特例外，**所有通知事件一律由
`template_handler.go` 以 `(exchange, routing_key)` 查 `notification_templates` 資料列驅動**。
新增一種通知＝新增一列 template（migration），**不新增 Go 檔**。

---

## 2.4 Device Token 註冊與管理

### 註冊方式

Device token 在 **使用者登入時由 Gateway 自動註冊**，前端不需要額外呼叫 API。

```
前端登入（帶 fcmToken + platform）
  → Gateway GoogleOAuth handler
    → 登入成功後 fire-and-forget goroutine
      → notification-service gRPC RegisterDeviceToken
        → RegisterTokenUseCase.Execute()
          → device_tokens 表 upsert
```

- 登入 request 的 `fcmToken` 為空時，Gateway 跳過註冊
- 註冊失敗不影響登入回應（僅 log warning）
- 同一 `(user_id, user_type, token)` 組合 upsert，不產生重複記錄

### Token Refresh

FCM token 可能被 Firebase 輪換，App 的 `onTokenRefresh` 回呼需呼叫：

```
PUT /api/v1/app/notifications/device-token
Body: { "token": "<new_fcm_token>", "platform": "android|ios" }
```

此 endpoint 由 Gateway handler 處理，內部呼叫 notification-service gRPC `RegisterDeviceToken`。

### Token 刪除（登出）

```
DELETE /api/v1/app/notifications/device-token
Body: { "token": "<fcm_token>" }
```

此 endpoint 由 Gateway handler 處理，內部呼叫 notification-service gRPC `RemoveDeviceToken`。

### gRPC RPC 定義

```protobuf
rpc RegisterDeviceToken(RegisterDeviceTokenRequest) returns (RegisterDeviceTokenResponse);
rpc RemoveDeviceToken(RemoveDeviceTokenRequest) returns (RemoveDeviceTokenResponse);

message RegisterDeviceTokenRequest {
    int64  user_id   = 1;
    string user_type = 2;  // "buyer" | "merchant"
    string token     = 3;  // FCM device token
    string platform  = 4;  // "android" | "ios"
}

message RegisterDeviceTokenResponse {
    int32  code    = 1;
    string message = 2;
}

message RemoveDeviceTokenRequest {
    int64  user_id   = 1;
    string user_type = 2;  // "buyer" | "merchant"
    string token     = 3;  // FCM device token
}

message RemoveDeviceTokenResponse {
    int32  code    = 1;
    string message = 2;
}
```

---

## 3. 通知偏好設定 (Notification Preferences)

### 3.1 Category 定義

使用者可透過 App 的「通知設定」頁面開關各類別。未設定的 category 預設為 `enabled = true`。

| Category | 買家 | 賣家 | 說明 |
|---|:---:|:---:|---|
| `order_update` | ✓ | ✓ | 訂單相關（下單、付款、出貨、完成、取消） |
| `refund_update` | ✓ | ✓ | 退款相關（退款申請） |
| `system` | ✓ | ✓ | 系統公告 |
| `chat` | ✓ | ✓ | 聊天訊息 |
| `shipment_anomaly` | — | 🔒 | 配送異常（**賣家專屬**，強制 on 不可關；買家不開放此類 — template `target_user_type='seller'`） |
| `follower` | — | ✓ | 新追蹤者（賣家專屬） |
| `coupon` | ✓ | — | 優惠券發放（買家專屬） |
| `post` | ✓ | — | 追蹤賣家發文（買家專屬） |
| `message` | ✓ | — | 貼文留言（買家專屬） |
| `restock` | ✓ | — | 補貨通知（買家專屬；買家對售完商品訂閱補貨提醒） |

> `🔒` 表示後端強制 `enabled=true` 且 PUT API 拒絕修改該 category（gateway 回 400 INVALID_ARGUMENT）。

### 3.2 API

| 端點 | 方法 | 說明 |
|---|---|---|
| `/api/v1/app/buyers/me/notification-preferences` | GET | 取得買家 7 個 category 開關（不含 `shipment_anomaly` — 賣家專屬） |
| `/api/v1/app/buyers/me/notification-preferences` | PUT | 更新買家單一 category |
| `/api/v1/app/sellers/me/notification-preferences` | GET | 取得賣家 7 個 category 開關（含 `shipment_anomaly`，proto `NotificationPreferenceItem.forced` 由 backend `domain.IsForcedCategory(userType, category)` 計算填入；對賣家的 `shipment_anomaly` 永遠 `forced=true`） |
| `/api/v1/app/sellers/me/notification-preferences` | PUT | 更新賣家單一 category（嘗試關閉 forced category 回 400） |

### 3.3 資料表

```sql
CREATE TABLE notification_preferences (
    id         bigserial PRIMARY KEY,
    user_id    bigint NOT NULL,
    user_type  varchar(10) NOT NULL,  -- 'buyer' | 'merchant'
    category   varchar(30) NOT NULL,
    enabled    boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, user_type, category)
);
CREATE INDEX idx_notification_preferences_user ON notification_preferences (user_id, user_type);
```

### 3.4 eventTypeToCategory Mapping

推播發送前透過此 mapping 找到對應的偏好 category，若使用者已關閉則跳過推播。未列出的 event_type 一律發送（fail-open）。

```go
var eventTypeToCategory = map[string]string{
    "order.placed":                 "order_update",
    "order.paid":                   "order_update",
    "order.ORDER_PAID":             "order_update",
    "order.ORDER_COMPLETED":        "order_update",
    "order.ORDER_SHIPPED":          "order_update",
    "order.ORDER_CANCELLED":        "order_update",
    "order.ORDER_REFUND_REQUESTED": "refund_update",
    "chat.message.sent":            "chat",
    "follow.added":                 "follower",
    "coupon.issued":                "coupon",
    "post.created":                 "post",
    "comment.created":              "message",
    "shipment.anomaly.detected":    "shipment_anomaly",  // 僅 seller target_user_type
    "RESTOCK_AVAILABLE":            "restock",            // 補貨通知（買家專屬，event_type 採 UPPER_SNAKE 對齊 NotificationType.RESTOCK_AVAILABLE，由單體 buyer 模組 fan-out 發 product.restock.target；mapping 不影響推播流程，由 template 端 category=restock 直接決定）
}
```

---

## 4. 推播通知總覽

### 4.1 買家通知

| # | 通知名稱 | Exchange | Routing Key | Queue | Event Type | Category | Title | Body | 觸發時機 | 來源 Service |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 訂單付款成功 | `hawkr.order` | `order.paid` | `notification.order` | `order.paid` | `order_update` | 訂單付款成功 | 您的訂單 #{orderNo} 已付款成功，賣家將盡快出貨！ | 買家完成付款 | order-service |
| 2 | 訂單已出貨 | `hawkr.order` | `order.shipped` | `notification.order` | `order.ORDER_SHIPPED` | `order_update` | 訂單已出貨 | 訂單 #{orderSn} 已出貨，請留意物流狀態。 | 賣家出貨 | order-service |
| 3 | 訂單已完成 | `hawkr.order` | `order.completed` | `notification.order` | `order.ORDER_COMPLETED` | `order_update` | 訂單已完成 | 訂單 #{orderSn} 已完成，感謝您的購買！ | 訂單確認完成 | order-service |
| 4 | 訂單已取消 | `hawkr.order` | `order.cancelled` | `notification.order` | `order.ORDER_CANCELLED` | `order_update` | 訂單已取消 | 訂單 #{orderSn} 已取消。 | 訂單取消 | order-service |
| 5 | 優惠券發放 | `hawkr.marketing` | `coupon.issued` | `notification.marketing` | `coupon.issued` | `coupon` | 你有新的優惠券！ | 你收到了「{couponName}」優惠券 | Admin/賣家發放或買家領取 | coupon-service (Java) |
| 6 | 賣家發文 | `hawkr.content` | `post.created` | `notification.content` | `post.created` | `post` | {sellerName} 發布了新貼文 | {contentPreview} | 追蹤的賣家發布貼文 | social-service |
| 7 | 貼文留言 | `hawkr.content` | `comment.created` | `notification.content` | `comment.created` | `message` | 你的貼文有新留言 | {commenterName}：{contentPreview} | 有人在貼文留言（排除自己） | social-service |
| 8 | 聊天訊息 | `hawkr.im` | `chat.message.sent` | `notification.im` | `chat.message.sent` | `chat` | {senderName} | {contentPreview} | 收到聊天訊息 | im-service |

### 4.2 賣家通知

| # | 通知名稱 | Exchange | Routing Key | Queue | Event Type | Category | Title | Body | 觸發時機 | 來源 Service |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 新訂單 | `hawkr.shipping` | `shipment.created` | `notification.shipping` | `shipment.created` | `order_update` | 您有新訂單！ | 買家已支付，請儘速準備商品並在 2 天內出貨。 | 物流單建立成功（含 Backstop 救活） | shipping-service |
| 2 | 退款申請 | `hawkr.order` | `order.refund.requested` | `notification.order` | `order.ORDER_REFUND_REQUESTED` | `refund_update` | 收到退款申請 | 訂單 #{orderSn} 買家已申請退款，請儘速處理。 | 買家申請退款 | order-service |
| 3 | 訂單出貨 | `hawkr.order` | `order.shipped` | `notification.order` | `order.ORDER_SHIPPED` | `order_update` | 訂單已出貨 | 訂單 #{orderSn} 已出貨，請留意物流狀態。 | 訂單出貨 | order-service |
| 4 | 訂單完成 | `hawkr.order` | `order.completed` | `notification.order` | `order.ORDER_COMPLETED` | `order_update` | 訂單已完成 | 訂單 #{orderSn} 已完成，感謝您的購買！ | 訂單完成 | order-service |
| 5 | 訂單取消 | `hawkr.order` | `order.cancelled` | `notification.order` | `order.ORDER_CANCELLED` | `order_update` | 訂單已取消 | 訂單 #{orderSn} 已取消。 | 訂單取消 | order-service |
| 6 | 系統公告 | — （不經 MQ） | — | — | `notification.announcement` | `system` | {title} | {body} | 平台管理員發布系統公告，走 gRPC `BroadcastAnnouncement` 直接派送（見 13 號 §7） | gateway/admin |
| 7 | 新追蹤者 | `hawkr.content` | `follow.added` | `notification.content` | `merchant.new_follower` | `follower` | 你有新粉絲！ | {followerName} 開始追蹤你了！ | 買家追蹤賣家 | 單體 buyer 模組 |
| 8 | 實名認證結果 | `hawkr.merchant` | `merchant.verification.{approved,rejected,pending_info,forwarded}` | `notification.merchant` | 同 routing key | — (always send) | 依審核結果 | （依審核結果） | 管理員審核認證 | 單體 seller 模組 |
| 9 | 貼文留言 | `hawkr.content` | `comment.created` | `notification.content` | `comment.created` | `message` | 你的貼文有新留言 | {commenterName}：{contentPreview} | 買家在賣家貼文留言 | social-service |
| 10 | 聊天訊息 | `hawkr.im` | `chat.message.sent` | `notification.im` | `chat.message.sent` | `chat` | {senderName} | {contentPreview} | 收到聊天訊息 | im-service |
| 11 | 配送異常 | `hawkr.shipping` | `shipment.anomaly.detected` | `notification.shipping` | `shipment.anomaly.detected` | `shipment_anomaly` | 配送異常通知 | 訂單 #{orderNo} {anomalyTypeLabel}，請儘速處理。 | shipping-service 偵測到配送異常 | shipping-service |

> 配送異常推送節奏（**HIGH** 立即推、**MEDIUM** 早晚摘要、**LOW** 僅寫 inbox 不推 push）由 notification-service consumer 端依 `severity` 欄位處理；買家若同訂單有對應配送，視場景另發既有 `shipment.delivered` 等通知（不在本表）。詳見 [06 §5.21](./06-物流微服務（Shipping%20Service）詳細設計書.md#521-配送異常賣家通知)。
>
> **RETURN_STUCK 特殊說明**：此 anomaly_type 在 `aftersale_cases` 而非 `shipments`，shipping-service Dashboard SQL（06 §5.21.7）不查 aftersale_cases 因此**不會列出 RETURN_STUCK row**；但 FCM 推送仍會送達。前端 dashboard 頁底部加說明文字引導賣家至「售後管理」查看退貨卡關案件（避免「收到推送但 dashboard 找不到」的 UX 困惑）。

---

## 5. RabbitMQ Consumer 配置

### 5.1 Subscription 總覽

| # | 來源 Exchange | Queue | 綁定 | 涵蓋事件（由 `notification_templates` 決定，非 topology）|
|---|---|---|---|---|
| 1 | `aftersales.events` | `notification.aftersales` | `#` | 售後案件全生命週期（申請 / 核准 / 退貨 / 退款 / 平台裁決…）|
| 2 | `hawkr.buyer` | `notification.buyer` | `#` | 買家帳號相關 |
| 3 | `hawkr.content` | `notification.content` | `#` | 貼文建立 / 留言 / 按讚 / 追蹤 / 平台隱藏 |
| 4 | `hawkr.delegation` | `notification.delegation` | `#` | 代行操作完成通知賣家（56 號 §6.3 / 36 號 §3.6j）|
| 5 | `hawkr.im` | `notification.im` | `#` | 聊天訊息（依 `receiverType` 分買賣家 template）|
| 6 | `hawkr.invoice` | `notification.invoice` | `#` | 電子發票開立 / 作廢 / 折讓 |
| 7 | `hawkr.marketing` | `notification.marketing` | `#` | 優惠券發放 / 到期提醒 |
| 8 | `hawkr.merchant` | `notification.merchant` | `#` | 賣家認證結果 |
| 9 | `hawkr.order` | `notification.order` | `#` | 訂單生命週期（付款 / 出貨 / 完成 / 取消 / 退款申請）|
| 10 | `hawkr.product` | `notification.product` | `#` | 商品收藏 / 到貨通知 / 瀏覽 |
| 11 | `hawkr.review` | `notification.review` | `#` | 評價建立 |
| 12 | `hawkr.seller` | `notification.seller` | `#` | 賣家停權 / 解除停權 |
| 13 | `hawkr.shipping` | `notification.shipping` | `#` | 物流單建立 / 狀態變更 / 配送完成 / 異常 |
| 14 | `payment.events` | `notification.payment` | `#` | 付款救援退款完成 / 撥款完成 |
| 15 | `procurement.events` | `notification.procurement` | `#` | 採購報價單邀請（建立 / 已被他人承接）|

> `hawkr.notification` **不在本表**：它是 notification-service 自己的 publish 目標
> （`notification.inbox.new` / `notification.unread_changed`，由 gateway SSE 消費），
> 訂閱它等於自產自銷。

> **SSOT 是 code 不是本表**：queue 對照表在 `internal/worker/consumer.go` 的
> `notificationQueueByExchange`，並由建置期不變式 `TestNotificationQueuesExistInTerraformTopology`
> 驗證與 `infra/mq/topology/topology.yaml` 一致。

> **綁定粒度**：每個來源 exchange 一條 queue ＋ `#` 萬用綁定，dispatch 依
> `(exchange, routing_key)` 查 `notification_templates`——**新增／修改／刪除 template
> 不需要任何 topology 變更**。設計理由見 ADR `2026-07-25-notification-queue-granularity`。

> **聊天訊息**：im-service publish `hawkr.im` / `chat.message.sent`，由模板引擎透過 2 筆 `notification_templates`（buyer / seller，`payload_filter` on `receiverType`）消費，`push_mode='visible'`。詳見 §8。

---

## 6. FCM Payload 標準格式與 Type 定義

所有推播（皆走 MQ 事件 → 模板引擎）統一使用以下 FCM payload 格式：

```json
{
  "message": {
    "token": "<FCM_TOKEN>",
    "notification": {
      "title": "新訂單通知",
      "body": "收到新訂單，請儘速確認處理"
    },
    "data": {
      "type": "order_created",
      "order_id": "12345678911"
    },
    "android": {
      "priority": "high"
    }
  }
}
```

| 區塊 | 說明 |
|---|---|
| `notification` | 系統通知標題與內容，APP 在背景或關閉時也會顯示通知（強推） |
| `data` | 前端用 `type` 欄位路由通知到對應頁面，附帶業務欄位供前端使用 |
| `android.priority` | 設為 `"high"` 確保 Android 端即時顯示推播 |

> **注意**：使用 `notification` 區塊（強推場景）時，**不需要** `apns.payload.aps.content-available`，否則 iOS 會收到兩次通知。

> **`data.type` vs `EventType` — 容易搞混，注意區分**
>
> | 名稱 | 用途 | 範例 | 誰用 |
> |---|---|---|---|
> | `data.type`（FCM flat type） | 前端路由，決定點擊通知後跳到哪個頁面 | `"order_shipped"` | 前端 |
> | `EventType`（MQ event type） | 後端內部：查通知偏好（`eventTypeToCategory`）+ 冪等檢查 | `"order.ORDER_SHIPPED"` | 後端 |
>
> 兩者在 worker handler 中同時設定，互不影響：
> - `SendPushRequest.EventType` = MQ event type（後端用）
> - `SendPushRequest.Data["type"]` = FCM flat type（前端用）

### 6.1 `data.type` 完整定義

每個 `type` 代表一個獨立事件，前端依據 `type` 決定點擊通知後的路由。新增 type 時前後端需同步：**後端加 handler、前端加路由處理**。

`notification.title` / `body` 由後端 notification-service 設定，前端直接顯示，不需要自行組裝文案。

#### 訂單相關（共用 `order_id` 欄位）

| `type` | 說明 | 通知對象 | `notification.title` | `notification.body` |
|---|---|---|---|---|
| `order_paid` | 付款成功 | 買家 | 訂單付款成功 | 您的訂單 #{orderNo} 已付款成功，賣家將盡快出貨！ |
| `order_created` | 新訂單 | 賣家 | 您有新訂單！ | 買家已支付，請儘速準備商品並在 2 天內出貨。 |
| `order_shipped` | 已出貨 | 買家 + 賣家 | 訂單已出貨 | 訂單 #{orderSn} 已出貨，請留意物流狀態。 |
| `order_completed` | 已完成 | 買家 + 賣家 | 訂單已完成 | 訂單 #{orderSn} 已完成，感謝您的購買！ |
| `order_cancelled` | 已取消 | 買家 + 賣家 | 訂單已取消 | 訂單 #{orderSn} 已取消。 |
| `order_refund_requested` | 退款申請 | 賣家 | 收到退款申請 | 訂單 #{orderSn} 買家已申請退款，請儘速處理。 |

```json
"data": {
    "type": "order_created",
    "order_id": "12345678911"
}
```

#### 聊天

| `type` | 說明 | 通知對象 | `notification.title` | `notification.body` |
|---|---|---|---|---|
| `chat_message` | 聊天訊息 | 買家 / 賣家 | {senderName} | {contentPreview} |

```json
"data": {
    "type": "chat_message",
    "senderId": "2663888155",
    "senderName": "小明",
    "senderAvatar": "https://cdn.../avatar.jpg",
    "conversationId": "b2663888155_s2683479278",
    "messageId": "123456",
    "sentAt": "2026-02-17T10:00:00Z"
}
```

> 由 im-service publish `hawkr.im` / `chat.message.sent`，模板引擎以 `push_mode='visible'` 推送 **notification + data**。前端以 `conversationId` 導航至聊天室，WebSocket / FCM 雙通道去重用 `(conversationId, seqId)`（seqId 屬 WS / 前端去重概念，不在 FCM event payload 中）。

#### 行銷 / 社群 / 追蹤

| `type` | 說明 | 通知對象 | `notification.title` | `notification.body` | `data` 欄位 |
|---|---|---|---|---|---|
| `coupon_issued` | 優惠券發放 | 買家 | 你有新的優惠券！ | 你收到了「{couponName}」優惠券 | `coupon_id`, `coupon_name` |
| `post_created` | 賣家發文 | 買家（追蹤者） | {sellerName} 發布了新貼文 | {contentPreview} | `post_id`, `seller_name` |
| `comment_created` | 貼文留言 | 貼文作者 | 你的貼文有新留言 | {commenterName}：{contentPreview} | `post_id`, `comment_id`, `commenter_name` |
| `follower_added` | 新追蹤者 | 賣家 | 你有新粉絲！ | {followerName} 開始追蹤你了！ | `follower_id`, `follower_name` |

```json
// 範例：coupon_issued
"data": {
    "type": "coupon_issued",
    "coupon_id": "789456",
    "coupon_name": "新會員 9 折券"
}
```

#### 系統 / 認證

| `type` | 說明 | 通知對象 | `notification.title` | `notification.body` | `data` 欄位 |
|---|---|---|---|---|---|
| `SYSTEM_ANNOUNCEMENT` | 系統公告 | 買家 / 賣家（依 admin 選擇的 target_user_type） | {title}（admin 自訂） | {body}（admin 自訂） | `announcementId`、`category=system` |
| `verification_approved` | 認證通過 | 賣家 | 實名認證審核通過 | 恭喜！您的實名認證已通過審核，即可開始在 Hawkr 銷售商品。 | — |
| `verification_rejected` | 認證未通過 | 賣家 | 實名認證審核未通過 | 您的實名認證申請未通過，請重新提交資料。 | — |

#### 補貨通知（共用 `product_id` 欄位）

| `type` | 說明 | 通知對象 | `notification.title` | `notification.body` |
|---|---|---|---|---|
| `RESTOCK_AVAILABLE` | 訂閱商品補貨 | 買家 | 🛎️ 您追蹤的商品補貨了 | 「{productName}」已經補貨上架，快來搶購吧！ |

```json
"data": {
    "type": "RESTOCK_AVAILABLE",
    "category": "restock",
    "refType": "PRODUCT",
    "refId": "843856384063471760",
    "url": "hawkr://products/843856384063471760",
    "imageUrl": "https://media.example.com/i/original/public/product/1001/20260615/cover-uuid.jpg"
}
```

> 圖片欄位：payload 來源（`product.restock.target`）帶 `imageKey`（objectKey），模板 `image_key_template` 填 objectKey 佔位，由 notification-service 渲染端組出上例 `imageUrl` 完整 URL（49 號設計書 §5.3／§6.3）。

> 由 product-service 偵測庫存 0→可見 transition（02 §5.13b）→ 發 `product.restock` event → 單體 buyer 模組（`internal/legacy/buyerconsumer`）fan-out 為每個 ACTIVE 訂閱者發 `product.restock.target` event → notification-service template-driven consumer 自動寫 inbox + FCM。
>
> **冪等鍵**：notification `reference_id_template = restock_#{subscriptionId}`，搭配 schema 既有 UNIQUE `(user_id, user_type, event_type, reference_id)` 保 inbox 端 fan-out retry 時不重複插入。

```json
// 範例：系統公告（gRPC BroadcastAnnouncement 產出，見 §7.5）
"data": {
    "type": "SYSTEM_ANNOUNCEMENT",
    "category": "system",
    "announcementId": "7495464541774413824"
}
```

---

## 7. 各事件 Payload 與推播設計

> 本章每一節描述**一筆 `notification_templates` 資料列**的現行欄位值。
> 「Payload 欄位」表是 producer 必須發出的 wire 契約（JSON key 逐字）；
> 「推播」表是該 template 的 `target_user_field` / `target_user_type` /
> `title_template` / `body_template` / `reference_id_template` / `data_type` 值。
> 佔位符語法 `#{field}` 直接對應 payload 的 JSON key。

### 7.1 訂單付款成功（買家）

**事件來源**：order-service 發布 `hawkr.order` / `order.paid`

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `orderNo` | string | 訂單編號，進 title/body/referenceId |
| `shippingMethod` | string | 配送方式 |
| `occurredAt` | string (RFC3339) | 事件發生時間 |
| `buyerAccountId` | int64 | 推播對象（`target_user_field`） |

| 欄位 | 推播 |
|---|---|
| UserID | `event.BuyerAccountID` |
| UserType | `buyer` |
| Title | `訂單付款成功` |
| Body | `您的訂單 #{OrderNo} 已付款成功，賣家將盡快出貨！` |
| ReferenceID | `{OrderNo}`（冪等） |
| Data | `{"type": "order_paid", "order_id": "{OrderNo}"}` |

---

### 7.2 買家已下單（賣家）

**事件來源**：shipping-service 發布 `hawkr.shipping` / `shipment.created`

> **綁定在 `shipment.created` 而非 `order.paid` 是規格**：「賣家收到推播」必須等於
> 「賣家打開 App 有物流單可動作」。綁 `order.paid` 時若 `createShipmentFromOrder`
> 失敗，賣家會收到推播卻打開 App 沒物流單，且 Backstop 救活時沒有第二次通知。
> 現行語意：Backstop 救活時賣家在救活當下被推播；建單永久失敗則
> `shipment.created` 永不發出、賣家不會收到假象訂單通知。

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `orderNo` | string | 訂單編號，進 body 與 referenceId |
| `shipmentNo` | string | 物流單號 |
| `shipTradeNo` | string | PayUni 物流交易序號 |
| `shippingMethod` | string | 配送方式 |
| `status` | string | 物流單狀態 |
| `labelUrl` | string | 標籤列印網址 |
| `processingDeadline` | string | 賣家出貨期限 |
| `sellerAccountId` | string | 推播對象（`target_user_field`） |
| `buyerAccountId` | string | 買家識別（取自 `OrderShippingInfo`） |
| `totalActualAmount` | string | 實付金額（元，整數字串） |
| `buyerNickname` | string \| null | best-effort enrich，失敗為 null |
| `buyerAvatar` | string \| null | 絕對 URL，best-effort enrich，失敗為 null |

| 欄位 | 推播 |
|---|---|
| Queue | `notification.shipping` |
| target_user_field | `sellerAccountId` |
| UserType | `seller` |
| Title | `買家已下單` |
| Body | `🧧 您有新訂單！買家已支付，請儘速準備商品並在 2 天內出貨。`（靜態文字，無 `#{}` 佔位符）|
| ReferenceID | `#{orderNo}`（冪等） |
| Data | `{"type": "ORDER_CREATED", "refType": "ORDER", "refId": "{orderNo}", "order_id": "{orderNo}", "buyerNickname": "{buyerNickname}", "buyerAvatar": "{buyerAvatar}"}` |

> **個人化字樣為 best-effort**：shipping-service 在 `publishShipmentCreatedEvent` 內透過 `BuyerGrpcClient.batchGetBuyerMeta` enrich 取得 buyerNickname / buyerAvatar 後寫入 payload；enrich 失敗時兩欄為 null，`FieldSpec.Resolve` 略過該欄位，前端 `_personalize` fallback 至通用「買家」字樣，title/body 主文不受影響。

---

### 7.3 訂單生命週期（買家／賣家）

**事件來源**：order-service 發布 `hawkr.order` / `order.shipped`、`order.completed`、`order.cancelled`、`order.refund.requested`

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `eventType` | string | `ORDER_SHIPPED` / `ORDER_COMPLETED` / `ORDER_CANCELLED` / `ORDER_REFUND_REQUESTED` |
| `orderSn` | string | 訂單編號，進 body 與 referenceId |
| `sellerId` | int64 | 賣家推播對象 |
| `buyerId` | int64 | 買家推播對象 |

**推播內容依 eventType 決定**：

| eventType | Title | Body |
|---|---|---|
| `ORDER_SHIPPED` | 訂單已出貨 | 訂單 #{orderSn} 已出貨，請留意物流狀態。 |
| `ORDER_COMPLETED` | 訂單已完成 | 訂單 #{orderSn} 已完成，感謝您的購買！ |
| `ORDER_CANCELLED` | 訂單已取消 | 訂單 #{orderSn} 已取消。 |
| `ORDER_REFUND_REQUESTED` | 收到退款申請 | 訂單 #{orderSn} 買家已申請退款，請儘速處理。 |

**推播對象**：每個事件同時推播給**買家**（`event.BuyerAccountID`）和**賣家**（`event.SellerAccountID`），`ORDER_REFUND_REQUESTED` 僅推播給賣家。

**FCM Data Payload**：

```json
{ "type": "<依下表>", "order_id": "#{orderSn}" }
```

`eventType` → `data.type` 對照：

| EventType | `data.type` |
|---|---|
| `ORDER_SHIPPED` | `order_shipped` |
| `ORDER_COMPLETED` | `order_completed` |
| `ORDER_CANCELLED` | `order_cancelled` |
| `ORDER_REFUND_REQUESTED` | `order_refund_requested` |

---

### 7.4 實名認證結果（賣家）

**事件來源**：單體 seller 模組（經 `internal/legacy/sellerrelay`）發布 `hawkr.merchant` / `merchant.verification.completed`

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `eventId` | string | 事件識別 |
| `merchantId` | int64 | 推播對象（`target_user_field`） |
| `status` | string | `approved` \| `rejected`，決定 title/body 與 `data.type` |
| `reason` | string（可省略） | 未通過原因 |

| Status | Title | Body |
|---|---|---|
| `approved` | 實名認證審核通過 | 恭喜！您的實名認證已通過審核，即可開始在 Hawkr 銷售商品。 |
| `rejected` | 實名認證審核未通過 | 您的實名認證申請未通過，請重新提交資料。 |

**FCM Data Payload**：

```json
{ "type": "verification_#{status}" }
```

`status=approved` → `verification_approved`；`status=rejected` → `verification_rejected`。

> **注意**：此通知無對應 category（不在 `eventTypeToCategory` 中的 mapped event），always send。

---

### 7.5 系統公告 — 走 gRPC，不經 MQ

公告**沒有 MQ 事件**，也沒有 template：唯一寫入路徑是 notification-service 的
gRPC `BroadcastAnnouncement`（`internal/grpc/server_broadcast.go`），由 admin →
Gateway 呼叫。它逐一收件人走與模板引擎相同的 `SendPushUseCase`（冪等、通知偏好、
狀態/error_log、死 token 停用）＋ `CreateInboxItem`。

**公告不得改走 MQ**：`pkg/mq/exchanges.go` 沒有 announcement routing key，
`notification_templates` 也沒有對應資料列；新增其一都會造出第二條寫入路徑。

| 欄位 | 值 |
|---|---|
| 收件人 | 該 `target_user_type` 底下所有持有 active device token 的使用者 |
| ReferenceID | `announcement_{broadcastId}_{userId}`（`broadcastId` 為每則公告一個 TSID） |
| Category | `system`（受使用者「系統通知」開關約束，13 號 §9） |
| Data | `{"type": "SYSTEM_ANNOUNCEMENT", "category": "system", "announcementId": "{broadcastId}"}` |
| 回應 | `broadcast_id`（＝上列 `broadcastId`）＋ `recipient_count`，皆為字串。Gateway 原樣放進 `data.broadcastId` / `data.recipientCount` |

> 語意一律進 `data`、`message` 恆為 `"success"`，見 API 設計規範「message 來源策略」。
> **收件人數不得只寫在 `message` 的中文句子裡**，那要剖析字串才讀得出來。

> `broadcastId` 進 `reference_id` 是必要的：只用發布者 id 會讓同一管理員的第二則公告
> 與第一則撞鍵，inbox 被 `ON CONFLICT DO NOTHING` 吞掉、push 撞 dedup index，而 FCM
> banner 已經送出去了——公告閃一次之後哪裡都找不到。

---

### 7.6 新追蹤者（賣家）

**事件來源**：單體 buyer 模組（經 `internal/legacy/buyerrelay`）發布 `hawkr.content` / `follow.added`

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `followerId` | int64 | 追蹤者，進 referenceId 與 data |
| `followerName` | string | 追蹤者暱稱，為空時 body 走 fallback 句 |
| `merchantId` | int64 | 推播對象（`target_user_field`） |
| `occurredAt` | string (RFC3339) | 事件發生時間 |

| 欄位 | 推播 |
|---|---|
| UserID | `event.MerchantID` |
| Title | `你有新粉絲！` |
| Body | `{followerName} 開始追蹤你了！`（若無名字：`有人開始追蹤你了！`） |
| ReferenceID | `follow_{merchantId}_{followerId}`（冪等） |
| Data | `{"type": "follower_added", "follower_id": "{FollowerID}", "follower_name": "{FollowerName}"}` |

---

### 7.7 優惠券發放（買家）

**事件來源**：coupon-service (Java) 發布 `hawkr.marketing` / `coupon.issued`

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `buyerId` | int64 | 推播對象（`target_user_field`） |
| `couponId` | int64 | 優惠券識別，進 referenceId 與 data |
| `couponName` | string | 優惠券名稱，為空時 body 走 fallback 句 |
| `expireAt` | string (RFC3339) | 到期時間 |

| 欄位 | 推播 |
|---|---|
| UserID | `event.BuyerID` |
| UserType | `buyer` |
| Title | `你有新的優惠券！` |
| Body | `你收到了「{couponName}」優惠券`（若無名字：`你收到了一張新的優惠券`） |
| ReferenceID | `coupon_{buyerId}_{couponId}`（冪等） |
| Data | `{"type": "coupon_issued", "coupon_id": "{CouponID}", "coupon_name": "{CouponName}"}` |

**觸發時機**：
- Admin 批量發放優惠券（`issueInstances`）
- 買家主動領取優惠券（`receiveCoupon`）

---

### 7.8 賣家發文（買家）

**事件來源**：social-service 發布 `hawkr.content` / `post.created`

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `postId` | int64 | 貼文識別，進 referenceId 與 data |
| `sellerAccountId` | int64 | 發文賣家，用於查粉絲列表 |
| `sellerName` | string | 賣家名稱，為空時 title 走 fallback 句 |
| `contentPreview` | string | 內文摘要，為空時 body 走 fallback 句 |

**扇出規則**：本 template 的推播對象不是單一 payload 欄位——須透過單體 buyer 模組的 gRPC
`GetSubscribersList` 取得追蹤該賣家的所有買家，逐一推播。

| 欄位 | 推播 |
|---|---|
| UserID | 各 subscriberID（逐一推播） |
| UserType | `buyer` |
| Title | `{sellerName} 發布了新貼文`（若無名字：`你追蹤的賣家發布了新貼文`） |
| Body | `{contentPreview}`（若無：`查看最新動態`） |
| ReferenceID | `post_{postId}_{buyerId}`（每個買家獨立冪等） |
| Data | `{"type": "post_created", "post_id": "{PostID}", "seller_name": "{SellerName}"}` |

> **無追蹤者**：正常結束（ACK），不發送推播。
> **單體 buyer 模組不可用**：記錄 error log，不推播，不回傳錯誤（不進 DLQ）。

---

### 7.9 貼文留言（買家／賣家）

**事件來源**：social-service 發布 `hawkr.content` / `comment.created`

| Payload 欄位 | 型別 | 用途 |
|---|---|---|
| `commentId` | int64 | 留言識別，進 referenceId 與 data |
| `postId` | int64 | 貼文識別，進 referenceId 與 data |
| `commenterId` | int64 | 留言者，用於「自己留言不通知」判定 |
| `commenterName` | string | 留言者暱稱，進 body |
| `postAuthorId` | int64 | 推播對象（`target_user_field`） |
| `postAuthorType` | string | `buyer` \| `merchant`，缺省視為 `merchant` |
| `contentPreview` | string | 留言摘要，進 body |

| 欄位 | 推播 |
|---|---|
| UserID | `event.PostAuthorID` |
| UserType | `event.PostAuthorType`（預設 `merchant`） |
| Title | `你的貼文有新留言` |
| Body | `{commenterName}：{contentPreview}` |
| ReferenceID | `comment_{postId}_{commentId}`（冪等） |
| Data | `{"type": "comment_created", "post_id": "{PostID}", "comment_id": "{CommentID}", "commenter_name": "{CommenterName}"}` |

**特殊邏輯**：
- **自己留言不通知**：`commenterID == postAuthorID` 時跳過
- **僅頂層留言觸發**：賣家回覆（`parentID > 0`）不發布事件

---

### 7.10 賣家停權相關通知（DB-driven template，無需改 Go code）

停權連動 saga 的 notification 落地。三個通知全部走既有模板引擎（`notification_templates` rows + `payload_filter`），**無需新增 Go handler**。整體 saga 概述見 seller 模組規格 §10.2（`docs/modules/seller.md`）、46 號 §5；事件以 36 號 §3.6f 為 SSOT。

| # | 觸發事件 | 對象 | template（`event_type` / `payload_filter`） | 內容摘要 |
|---|---|---|---|---|
| 1 | `hawkr.seller` / `seller.suspended` | 賣家 | `seller.suspended.notice`（`target_user_field=accountId`, `UserType=seller`） | 「您的賣家帳號已被停權」+ 原因碼 `reasonCode`（SellerSuspensionReason）+ 引導至停權公告頁 / 申訴入口 |
| 2 | `hawkr.seller` / `seller.unsuspended` | 賣家 | `seller.unsuspended.notice`（同上） | 「您的賣家帳號停權已解除，可恢復營運」 |
| 3 | `hawkr.order` / `order.cancelled` + `payload_filter: {"cancel_reason": "9104"}` | 買家 | `order.cancelled.seller_suspended`（`target_user_field=buyer_id`, `UserType=buyer`, `reference_id_template=#{order_no}` 冪等） | 「訂單因賣家停權已取消」+ 未付款款項無需處理 |

- 通知 1/2 由 `notification.seller` queue（§5.1 第 12 列）消費 `seller.suspended` / `seller.unsuspended`。
- 通知 3 與既有泛用取消通知（`order.cancelled.buyer`，id=7）掛在同一個 `order.cancelled` routing key 上，兩者互斥靠 `payload_filter` 逐 template 比對達成：id=7 為 `{"cancel_reason": {"$ne": "9104"}}`（排除語法，`matchesFilter` 支援 `$ne` 運算子，見 `internal/domain/notification_template.go` `FilterCondition` / `internal/worker/template_handler.go` `matchesFilter`），本 template 為 `{"cancel_reason": "9104"}`（等值）。兩者為互斥條件，確保賣家停權取消訂單時買家剛好收到一則通知（本模板）。需 order-service `order.cancelled` payload 帶 `cancel_reason`（既有欄位，Java `cancelReason`，wire snake_case）。
  - ⚠ 這個互斥保證來自 `payload_filter` 排他匹配，**不是** `notification_inbox` 的冪等去重：冪等鍵為 `(user_id, user_type, event_type, reference_id)` 四元組（見 `internal/usecase/notification_inbox.go`），`order.cancelled.buyer` 與 `order.cancelled.seller_suspended` 的 `event_type` 不同，即使 `reference_id` 皆為 `order_no` 也不會互相去重；「單一則」完全依賴兩個 template 的 `payload_filter` 互斥，新增/修改任一方時務必確認彼此條件仍互斥，否則會雙發或漏發。
- fail 策略：通知非必達，publisher fail-and-forget（失敗只 log，不 abort 主流程），與 §7.2 賣家通知一致。
- **DB-driven 落地**：三筆 template 為 `notification_templates` 資料列（含 `title_template` / `body_template` / `data_type` / `category`），Go code 零改動；`payload_filter` 的等值與 `$ne` 排除比對皆為模板引擎既有能力。

---

## 8. 聊天訊息推播（MQ 事件 + 模板引擎，push_mode=visible）

im-service publish RabbitMQ 事件（`hawkr.im` / `chat.message.sent`），由 notification-service 既有模板引擎消費，**與其他通知一致**走 MQ + 模板管線，不再透過 gRPC `SendDataPush`（該 RPC 已從 proto / Java mirror 移除）。

聊天採 **WebSocket + FCM 雙通道** 送達，同一條訊息可能兩邊都收到，前端靠 `(conversationId, seqId)` 去重。FCM 為 **visible notification + data**（非 data-only），背景 / 關閉時 OS 自動顯示，前景時前端在使用者正在檢視該對話時抑制 banner。

```go
// im-service publish（goroutine，失敗僅 log；訊息已持久化 + WS 投遞）
event := ChatMessageSentEvent{
    MessageId:      messageID,        // string，避免 TSID 在 float64 下精度遺失
    ConversationId: roomID,
    SenderId:       senderID,
    SenderName:     senderName,
    SenderAvatar:   senderAvatar,
    ReceiverId:     receiverID,
    ReceiverType:   receiverType,     // "buyer" | "seller"
    ContentPreview: preview,          // 圖片訊息固定為「傳送了一張圖片」
    SentAt:         sentAt,           // RFC3339
}
go publisher.Publish(ctx, "hawkr.im", "chat.message.sent", event)
```

notification-service 端以 **2 筆 `notification_templates`**（buyer / seller，用 `payload_filter` on `receiverType` 區分）消費：
- `push_mode='visible'`、`category='chat'`、`event_type='chat.message.sent'`、`queue_name='notification.im'`
- `target_user_field='receiverId'`、`title_template='#{senderName}'`、`body_template='#{contentPreview}'`、`body_fallback='您有一則新訊息'`
- `data_type='chat_message'`、`reference_id_template='#{messageId}'`（冪等 key = `event_type` + `messageId`）
- `data_fields` 映射 senderId / senderName / senderAvatar / conversationId / messageId / sentAt

因此聊天自動沿用模板管線的 inbox 寫入、失效 token 停用、冪等與通知偏好過濾（category `chat`，無偏好 row 時 fail-open）。

> 前端以 `type=chat_message` + `conversationId` 直接跳轉聊天室。

---

## 9. 推播發送核心流程

```
事件進入 worker handler
  ├── JSON unmarshal（失敗 → log error, ACK 丟棄）
  ├── 驗證必填欄位（缺失 → log warn, ACK 丟棄）
  └── 組裝 SendPushRequest → SendPushUseCase.Execute()
        ├── 冪等檢查：event_type + reference_id 是否已成功發過
        │     └── 已發過 → skip, return nil
        ├── 偏好設定檢查：eventTypeToCategory → GetByCategory()
        │     └── enabled=false → skip, log "category disabled"
        ├── 取得使用者所有 active device tokens
        │     └── 無 token → 記錄 status=skipped, return
        ├── FCM 發送（Send title/body/data to all tokens）
        │     ├── 成功 → status=sent
        │     ├── 部分失敗 → status=partial, deactivate invalid tokens
        │     └── 全部失敗 → status=failed
        └── 寫入 push_notifications 表（記錄推播結果）
```

### 9.1 冪等機制

冪等鍵為 `(user_id, user_type, event_type, reference_id)` 四元組。
`reference_id` 由 template 的 `reference_id_template` 產出：

| event_type（routing key） | `reference_id_template` | 說明 |
|---|---|---|
| `order.paid` | `#{orderNo}` | 同一訂單不重複推播 |
| `shipment.created` | `#{orderNo}` | 同一訂單不重複推播 |
| `order.shipped` / `order.completed` / `order.cancelled` / `order.refund.requested` | `#{orderSn}` | 同一訂單事件不重複 |
| `merchant.verification.completed` | `merchant_verification_#{merchantId}` | 同一賣家不重複 |
| `BroadcastAnnouncement`（gRPC，不經 template） | `announcement_#{broadcastId}_#{userId}` | 同一公告同一收件人 |
| `follow.added` | `follow_#{merchantId}_#{followerId}` | 同一粉絲不重複 |
| `coupon.issued` | `coupon_#{buyerId}_#{couponId}` | 同一優惠券同一買家 |
| `post.created` | `post_#{postId}_#{buyerId}` | 同一貼文同一買家（逐粉絲獨立冪等） |
| `comment.created` | `comment_#{postId}_#{commentId}` | 同一留言不重複 |

### 9.2 錯誤處理策略

| 錯誤類型 | 處理方式 |
|---|---|
| JSON unmarshal 失敗 | ACK（不 requeue），記錄 error log |
| 必要欄位缺失 | ACK（丟棄），記錄 warn log |
| SendPush 失敗 | ACK（已在 push_notifications 表記錄），記錄 error log |
| FCM token 無效 | 自動 deactivate token，不影響其他推播 |
| 單體 buyer 模組 gRPC 失敗 | ACK，記錄 error log，不推播 |

> 所有 handler 回傳 `nil`（always ACK），推播是 best-effort。

---

## 10. 事件發布端

### 10.1 各 Service 發布的事件

| Service | 語言 | 發布的事件 | Exchange | 發布時機 |
|---|---|---|---|---|
| order-service | Java | `order.paid`, `order.shipped`, `order.completed`, `order.cancelled`, `order.refund.requested` | `hawkr.order` | 訂單生命週期變更 |
| 單體 seller 模組 | Go | `merchant.verification.completed` | `hawkr.merchant` | 管理員審核賣家認證 |
| social-service | Go | `post.created` / `post.liked` / `post.hidden.by_platform` / `comment.created`（fat） / `comment.replied`（fat） / `comment.updated` / `comment.deleted` / `comment.like_changed` | `hawkr.content` | 貼文 / 留言 lifecycle。`comment.updated` / `comment.deleted` / `comment.like_changed` 供 [41 SSE 推播](41-Realtime%20SSE%20推播平台（Realtime%20SSE%20Push%20Platform）詳細設計書.md) realtime 同步；**沒有 `comment.liked` 這個 routing key**，按讚通知一律訂閱 `comment.like_changed` 並過濾 LIKE only |
| coupon-service | Java | `coupon.issued` | `hawkr.marketing` | 批量發放或買家領取優惠券 |
| im-service | Go | `chat.message.sent` | `hawkr.im` | 收到聊天訊息 |
| gateway | Go | — (gRPC `RegisterDeviceToken`) | — | 使用者登入成功 / FCM token refresh |
| **notification-service** | Go | `notification.inbox.new` / `notification.unread_changed` | `hawkr.notification` | inbox 寫入 / mark-read 後 publish，供 [41 SSE 推播](41-Realtime%20SSE%20推播平台（Realtime%20SSE%20Push%20Platform）詳細設計書.md) **web 端** 即時更新 badge 與 inbox（mobile 維持 FCM，不訂閱本 channel） |

### 10.2 發布模式

所有事件發布採 **best-effort**：
- 發布失敗只記 log，不影響主流程（貼文/留言/優惠券仍成功建立）
- Go services 使用 `pkg/mq.Publisher`
- Java coupon-service 使用 Spring AMQP `RabbitTemplate.convertAndSend()`

---

## 11. 前端 `data.type` 路由對照表

前端收到推播後，依 `data.type` 做 switch 導航。完整 type 定義與 data 欄位見 [Section 6.1](#61-datatype-完整定義)。

| `data.type` | 導航參數 | 前端導航 | Buyer App | Seller App |
|---|---|---|:---:|:---:|
| `order_paid` | `order_id` | 訂單詳情頁 | ✅ | — |
| `order_created` | `order_id` | 訂單列表（待出貨） | — | ✅ |
| `order_shipped` | `order_id` | 訂單詳情頁 | ✅ | ✅ |
| `order_completed` | `order_id` | 訂單詳情頁 | ✅ | ✅ |
| `order_cancelled` | `order_id` | 訂單詳情頁 | ✅ | ✅ |
| `order_refund_requested` | `order_id` | 訂單詳情頁 | — | ✅ |
| `chat_message` | `conversationId`, `senderId`, `messageId` | 聊天室 | ✅ | ✅ |
| `coupon_issued` | `coupon_id`, `coupon_name` | 優惠券詳情頁 | ✅ | — |
| `post_created` | `post_id`, `seller_name` | 貼文詳情頁 | ✅ | — |
| `comment_created` | `post_id`, `comment_id`, `commenter_name` | 貼文詳情頁 | ✅ | ✅ |
| `follower_added` | `follower_id`, `follower_name` | 顧客個人頁 | — | ✅ |
| `SYSTEM_ANNOUNCEMENT` | `announcementId` | 系統公告頁 | ✅ | ✅ |
| `verification_approved` | — | 實名認證頁 | — | ✅ |
| `verification_rejected` | — | 實名認證頁 | — | ✅ |

---

---

## 12. 測試覆蓋

### 12.1 Worker 層必測場景

模板引擎是唯一事件處理路徑 ⇒ 測試綁 **template 行為**，不綁 per-event 檔案。

| 層級 | 必測場景 |
|---|---|
| template dispatch | `(exchange, routing_key)` 命中／未命中；`payload_filter` 等值與 `$ne` 排他比對；多 template 綁同一 routing key 時互斥 |
| 佔位符解析 | `#{field}` 命中；欄位缺失時該佔位符略過而非整則失敗；巢狀 payload 取值 |
| 推播對象解析 | `target_user_field` 取值；欄位缺失時 ACK 丟棄並記 warn |
| 失敗語意 | JSON unmarshal 失敗 ACK；下游 gRPC 失敗 ACK 不進 DLQ（§9.2 表為準） |
| 廣播（gRPC 路徑） | 寫收件匣；兩則不同公告各自進收件匣（`reference_id` 帶公告識別）；同公告重投遞去重；關閉系統通知則不推播；FCM 部分失敗照實記錄並停用死 token；`data` 帶 `category` / `type` |
| 特例 handler | `account_deleted` / `seller_deleted` 的 token 與 inbox 清除；`vendor_invite` 的外部資料查詢失敗降級 |

### 12.2 Usecase 層必測場景

| 對象 | 必測場景 |
|---|---|
| `SendPushUseCase` | 冪等跳過、無 token 跳過、`GetActiveTokens` 錯誤、全部成功、部分失敗、全部失敗、invalid token deactivate |
| `NotificationPreference` | buyer/merchant 預設值、category 區隔、DB 記錄覆蓋、title/description 差異化 |
| `NotificationInbox` | 冪等四元組 `(user_id, user_type, event_type, reference_id)` 衝突去重；mark-read 後 publish `notification.unread_changed` |

### 12.3 Domain 層必測場景

| 對象 | 必測場景 |
|---|---|
| `notification_preference` | `CategoriesForUserType`（buyer/merchant/unknown）、`IsValidCategory`、`AllCategories` 聯集完整性 |
| `notification_template` | `FilterCondition` 的等值與 `$ne` 語意；`FieldSpec.Resolve` 對 null 欄位的略過行為 |
