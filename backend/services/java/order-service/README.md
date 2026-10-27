# Order Service

訂單微服務，統一處理買家、賣家、後台管理端的訂單生命週期。

## 技術棧

- Java 21 + Spring Boot 3.2.1
- gRPC（對外通訊，統一由 Gateway 呼叫）
- MyBatis-Plus + PostgreSQL 15
- RabbitMQ（非同步事件消費）
- Redis（流水號生成）
- Flyway（DB Migration）
- TSID（分散式主鍵，node-id-base: 300）

## 服務端口

| 用途     | 端口 |
|----------|------|
| HTTP     | 9914 |
| gRPC     | 9915 |

## 專案結構

```
src/main/java/io/github/will413028/hawkr/order/
├── config/          # Spring 配置（RabbitMQ、MyBatis TypeHandler）
├── constant/        # 服務專用常數（OrderConstants、MQ 常數）
├── converter/       # 型別轉換器
├── entity/          # MyBatis-Plus Entity（8 張表）
├── exception/       # 業務例外（OrderNotFoundException、InvalidStatusException 等）
├── grpc/            # gRPC Service 實作與 Proto 轉換器
├── mq/              # MQ 消費者（PaymentEventConsumer）
├── repository/      # MyBatis-Plus Mapper
├── service/         # 核心業務邏輯（OrderService）
└── worker/          # 定時任務 Worker（退款執行、自動退款、庫存補償）
```

## 資料庫表

| 表名                   | 用途               |
|------------------------|--------------------|
| orders                 | 訂單主表           |
| order_items            | 訂單明細表         |
| refund_requests        | 退款申請表         |
| financial_logs         | 資金流水表（冪等） |
| stock_tx_records       | 庫存交易協調表     |
| payment_event_dedup    | 支付事件冪等表     |
| auto_refund_tasks      | 自動退款任務表     |

## gRPC API

定義於 `api/proto/order/order.proto`

### 買家端
- `CreateOrder` - 建立訂單（含庫存扣減）
- `CancelOrder` - 取消訂單（僅 PENDING_PAYMENT）
- `GetMyOrders` - 查詢我的訂單列表
- `GetOrder` - 查詢訂單詳情
- `ApplyRefund` - 申請退款
- `SelectRefundMethod` - 選擇退款方式（原路/優惠券）

### 賣家端
- `GetSellerOrders` - 查詢賣家訂單列表
- `GetSellerOrderDetail` - 查詢賣家訂單詳情
- `ShipOrder` - 賣家發貨

### 管理端
- `SearchOrders` - 後台搜尋訂單
- `GetAdminOrderDetail` - 後台訂單詳情（含資金流水）
- `AuditRefund` - 審核退款申請

## MQ 消費

監聽 `payment.events` Exchange：
- `payment.success` → 更新訂單為已付款 / 建立自動退款任務
- `payment.expired` → 取消訂單並釋放庫存

## Worker 定時任務

| Worker                    | 週期  | 用途                           |
|---------------------------|-------|--------------------------------|
| RefundExecutionWorker     | 15 秒 | 執行退款（呼叫 Payment/Coupon） |
| AutoRefundWorker          | 30 秒 | 處理已取消但收到付款的自動退款   |
| StockCompensationWorker   | 60 秒 | 庫存扣減/歸還失敗補償           |

## 狀態碼

所有狀態碼使用字串存儲（VARCHAR），定義於 `services/java/shared/common-core`：

- `OrderStatus` - 訂單狀態（0:待付款, 10:已付款, 15:已發貨, 20:已完成, 30:已取消, 40:部分退款, 90:全額退款）
- `OrderItemStatus` - 明細退款狀態（0:正常, 10:退款中, 20:已退原路, 21:已退優惠券）
- `RefundRequestStatus` - 退款申請狀態（0:待審核, 10:待用戶選擇, 15:處理中, 20:完成, 90:駁回）
- `FinancialLogType` - 資金流水類型（1:收款, 2:原路退款, 3:優惠券退款）

## 依賴服務

| 服務             | 用途              | gRPC 端口 |
|------------------|-------------------|-----------|
| product-service  | 庫存扣減/歸還     | 9913      |
| payment-service  | 原路退款（TODO）  | 9931      |
| coupon-service   | 優惠券退款        | 9941      |

## 本地開發

```bash
# 前提：PostgreSQL、Redis、RabbitMQ 已啟動

# 編譯共用模組
cd services/java/shared/common-core && mvn clean install
cd services/java/shared/java-proto && mvn clean install

# 編譯並啟動
cd services/java/order-service
mvn compile
mvn spring-boot:run
```
