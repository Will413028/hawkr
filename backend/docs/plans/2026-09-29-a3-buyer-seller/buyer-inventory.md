# A3 buyer 盤點：`backend/services/buyer-service` → `internal/modules/buyer`

唯讀盤點，2026-09-29。以下路徑相對於 `backend/`，另有標註的除外。
參考：`AGENTS.md`、計畫「模組完成的定義」（`docs/plans/2026-09-24-go-modular-monolith.md` L24-46）、`internal/modules/identity/AGENTS.md`、ADR D8/D9、arch 依賴表 `internal/arch/modules.go`。

---

## 0. 摘要（先看這段）

- 12 張表（`rg -c 'CREATE TABLE' services/buyer-service/db/schema/schema.sql` = 12），48 支 RPC（`awk -F, '$3=="buyer-service"' docs/plans/.../rpc-servers.csv | wc -l` = 48，proto 裡也是 48）。沒有 extension、沒有樂觀鎖。
- **依賴方向衝突是最大的設計題**：arch 表只准 `buyer → identity, media`，但 buyer-service 同步呼叫 product（catalog）、seller、order（ordering）、crm 來補顯示欄位、檢查刪除條件、組事件 payload。這就是計畫寫的「buyer 往上的補資料移到 http」。反過來，seller-service 也呼叫 buyer（粉絲數、`HasActiveBuyer`），可是 arch 表不准 `seller → buyer`。
- **A3 之後還要有 legacy gRPC server 的呼叫方**不只有 Java：Go 的 ai（A9）、crm/im/review（A6）、invoice（A5）、notification（A8）、social（A7）都比 A3 晚搬。另外 gateway 的 `buyerid.Resolver`（Redis `buyer:current`）替大約 20 個 handler 檔（cart、order、checkout、coupon、aftersales、shipping、payment、notification…）解析 buyer_id，也還在呼叫 `GetBuyerProfile`。
- 事件：發出 6 種 routing key，消費 3 種（`account.deleted` 現在由單體的 identity 經 legacy RabbitMQ 發出）。外部 consumer 包含 Java（coupon、product），所以 buyer 的事件 A3 之後仍然要經 `internal/legacy/rabbitmq` 送出。
- 實際確認的缺陷：**PII 清理 cron 依 `account_id` 清掉帳號「所有」stint，包含重新註冊之後仍在使用的 stint**；「取消收藏」即使沒刪到列也照樣發出 removed 事件，會重複扣 `favorite_count`；重複追蹤會重發 `follow.added`；刪除流程在持有 `FOR UPDATE` 鎖的 transaction 裡呼叫 order-service gRPC；軟刪的地址永遠保留 PII。細節見 §8。
- 沒接上的功能（D8：不搬）：`notifications` 表（只剩 DELETE）、`MemberLevel` 升等、`GetBuyerProfileBatch`（0 個呼叫方）、`payment.membership.buyer.*` 的 consumer、會員計費、買家自助刪除（`DELETE /app/buyers/me` 沒有 client 呼叫，也不在 openapi 裡）、`POST /app/buyers/me`（沒有 client 呼叫）、後台 agent_entitlements 與 `/buyers/me/assistant/quota`（沒有 client 呼叫）。

---

## 1. 領域與資料

來源：`services/buyer-service/db/schema/schema.sql`（231 行，Atlas desired state）、22 支 migration（`ls db/migrations/*.sql | wc -l` = 22），`db/atlas.hcl`。DB 是 `hawkr_buyer`，`public` schema。

### 1.1 表

| 表 | 主鍵／ID | 值得注意的欄位 | 約束 | 刪除方式 |
|---|---|---|---|---|
| `buyers` | `buyer_id bigint`，TSID（`go-pkg/tsid`，`usecase/di.go:31`） | `account_id`、`is_current`（stint）、`display_name`、`email`、`phone`（從 account 複製）、`gender`（無 CHECK；client 送 `male/female/other/""`）、`avatar`（object key，預設 `public/defaults/buyer_avatar.png`）、`level`（預設 `bronze`，無 CHECK）、`birth_date`、`city`、`job`、`status`、`deleted_at`、`pii_cleared_at` | `chk_buyers_status IN (active,pending_deletion,closed)`、`chk_buyers_active_is_current`（非 closed 必須是 current）、`chk_buyers_deleted_requires_closed`、partial UNIQUE `uq_buyers_account_current (account_id) WHERE is_current` | 軟刪（`status=closed` 並填 `deleted_at`），30 天後 cron 清掉 PII；列永遠保留（墓碑） |
| `shipping_addresses` | `bigserial` | `label` varchar(20) 預設 `home`（無 CHECK）、`receiver_name/phone`、`city`、`district`、`detail_address`、`is_default` | 只有 `buyer_id` 的 index；**沒有「每個買家一個 default」的 partial UNIQUE** | 軟刪（`deleted_at`）；買家刪除時整批軟刪，**之後沒有清 PII** |
| `seller_subscriptions`（追蹤賣家） | `bigserial` | `buyer_id`、`store_id` | `UNIQUE(buyer_id, store_id)` | 硬刪 |
| `favorites` | `bigserial` | `buyer_id`、`product_id` | `UNIQUE(buyer_id, product_id)` | 硬刪 |
| `visit_history` | `bigserial` | `buyer_id`、`product_id`、`visited_at`（upsert） | `UNIQUE(buyer_id, product_id)`；index `(buyer_id, visited_at DESC)` | 硬刪 |
| `notifications` | `bigserial` | `type`、`title`、`ref_type/ref_id`、`is_read` | — | **已死**：queries 只剩 `DeleteNotificationsByBuyerID`（`db/queries/notifications.sql` 2 行）；設計書 39 §2.5/§5.4 也寫「沒有寫入入口」，通知已改由 notification-service 處理 |
| `restock_subscription` | `bigserial` | `status` varchar(20)，存 CodeMeta 值 `9501/9502/9503`（**無 CHECK**）、`notified_at` | partial UNIQUE `(buyer_id, product_id) WHERE status='9501'`；兩個 partial index | 狀態轉移（CANCELLED/NOTIFIED 是終態，留作稽核） |
| `restock_event_dedup` | `event_id uuid` | `product_id`、`consumed_at` | PK | 沒有 TTL 或清理 job（只有 `consumed_at` index） |
| `buyer_deletion_audit` | `bigint`，TSID | `account_id`、`operator_id`（admin）、`reason` | append-only | — |
| `buyer_deleted_outbox` | `bigint`，TSID | `account_id`、`buyer_id`（入列當下的快照）、`status`（無 CHECK）、`attempt_count`、`next_attempt_at`、`last_error`、`completed_at` | partial index `WHERE status='pending'` | — |
| `membership_plans` | `bigint`，種子資料的 ID 寫死 1001-1003 | `plan_code` UNIQUE、`monthly_answer_limit`、`monthly_price_twd`、`is_purchasable` | CHECK price >= 0、limit > 0；migration 內含種子資料（free/basic/premium，值是 PLACEHOLDER） | — |
| `buyer_memberships` | `bigint`（repo 的 `newID`，TSID） | `buyer_id`（stint 層級）、`plan_code` FK → `membership_plans.plan_code`、`period_start/end`、`source`、`granted_by`（admin id）、`status`、`request_id`（冪等鍵） | CHECK `source IN (admin, system_default)`、`status IN (active, superseded)`；partial UNIQUE：每個買家只有一筆 active；partial UNIQUE `request_id WHERE != ''` | 以 supersede 取代舊列 |

`db-tables.csv` 記 12 張表、94 欄。

### 1.2 擁有的資料與不變量

- **買家 stint**（ADR `2026-07-11-identity-store-stint-symmetry.md`，accepted，並註記「stint 在 buyer aggregate，A3 搬遷時核對」）：每個 `account_id` 最多一個 `is_current`；「刪除後再取得」會插入新的 stint（新的 `buyer_id`），舊 stint 的資料在結構上與新 stint 隔離。持久化的歷史 `buyer_id`（訂單、評論、訊息）一律解析回當時那個 stint。
- 狀態機：`active →（有未結訂單）pending_deletion → closed`，或 `active →（0 張訂單）closed`。pending 期間不能新增收藏（9203）；再次刪除 pending 的帳號回 9204。
- 顯示名字的墓碑 `'已刪除帳號'` 是跨服務共用的字面值（`buyers.sql` 註解列出 5 個可執行的位置：buyer 讀與寫、crm 兩處、seller 一處）。
- 地址歸屬：每一次讀寫都檢查 `addr.BuyerID == 目前 stint`（否則 Forbidden）。
- 會員：每個 stint 同時最多一筆 active（由 DB 強制）；額度週期以 stint 的 `created_at` 為錨點、逐月滾動（ADR `2026-09-18-buyer-membership-entitlement-model.md` D2/D2b）；過期或沒有會員就退回 `free`。
- 補貨通知：每個（買家、商品）最多一筆 ACTIVE；fan-out 一次性（ACTIVE→NOTIFIED）。

### 1.3 不擁有（但現在有碰到）

- `buyers.email`、`phone`：從 account-service 複製（`EnsureProfile` 呼叫 `GetAccountInfo`）。identity 的帳號已經沒有手機；email 是建帳時的快照。
- 頭像的 asset reference 在 media（`BUYER_AVATAR`，**ref_id = account_id，不是 buyer_id**）。

---

## 2. 沿用的機制（沿用盤點的原料）

| 機制 | 位置 | 備註、從零設計會怎麼做 |
|---|---|---|
| ID：`buyer_id`、audit、outbox、membership 用 TSID bigint；其餘 bigserial | `usecase/di.go:31` `tsid.NewGenerator` | 計畫 L201：A3 過渡期主鍵用 `bigint GENERATED ALWAYS AS IDENTITY`（Java 存 BIGINT），最後一個 Java 消費者改寫後換成 uuid。`membership_plans` 的 ID 寫死，但 `plan_code` 已經是自然鍵，id 欄位可以拿掉 |
| stint 模型（`is_current` 加 partial UNIQUE、`RetireCurrentStint` 與 insert 在同一個 tx、`ErrBuyerCreateConflict` 之後重讀） | `usecase/buyer_profile.go:81-173` | ADR 07-11 仍然有效；在單體裡要重新判斷，因為每個 Java idref 都指向 buyer_id |
| 以 `account_id` 為鍵的寫入（`UpdateBuyer`、`UpdateBuyerAvatar`、`UpdateBuyerStatus`、`SetBuyerPIICleared`） | `db/queries/buyers.sql` | legacy backlog #2345「仍需使用 account_id 的位置」；見 §8 D1 |
| Redis `fav:set:{buyerId}`（無 TTL，write-through，冷啟動從 DB 載入） | `repository/redis/favorite_cache.go` | 單體的 platform 沒有 Redis，identity 也拿掉了 Redis。PK 加 UNIQUE index 查 membership 本來就快，建議刪除 |
| Redis 地址查詢快取（`addr:sug:v3:*`、`addr:val:v3:*`，TTL 24h，fail-open） | `repository/redis/address_geocode_cache.go` | 只在 TGOS 有開通時才有意義 |
| 外部 provider TGOS（MOI 地址查詢），經 GCP Cloud Run 的 `tgos-proxy`／egress-proxy relay，GCP SA 金鑰檔 | `client/geocode/*`、`pkg/geocode` | **仰賴公司時期的 GCP infra（D9）**；沒有設定時回 9700（fail-closed）。seller-service 也用同一套（9700 共用）。從零設計：要嘛 D5 的同步查詢 client（地址自動完成是同步 UX，不適合排 River job），要嘛整個拿掉。要決定 |
| 地址查詢在 gateway 依角色分派到 buyer 或 seller | `gateway/internal/handler/address_geocode.go` | 重複實作；兩邊都搬進單體後應該只有一個擁有者 |
| Outbox `buyer_deleted_outbox` 加 `go-pkg/outbox.Worker`（claim lease 3 分鐘、退避 2^n 分鐘、最多 5 次）與 fx scheduler（每分鐘）；Prometheus 指標與監控 scheduler | `worker/*` | 用 `events.Publish`（同一個 tx）取代；Java、未搬的 Go consumer 經 legacy RabbitMQ adapter（見 §4） |
| 「盡力而為」的 goroutine 發事件（`syncutil.SafeGo`，沒有 outbox）：`buyer.favorite.added/removed`、`product.viewed`；`follow.added` 同步發但失敗只記 log | `usecase/favorite.go:378,417`、`visit.go:145`、`follow.go:59` | 事件遺失會讓 `favorite_count` 飄移。從零設計：同一個 tx 的事件 |
| 發出事件的 `publisher confirm` 5 秒上限；restock fan-out 在 tx 裡呼叫 AMQP publish（每批 200） | `mq/publisher.go`、`mq/restock_consumer.go` | 同一個 tx 的事件或 River job 取代 |
| 兩層冪等：`restock_event_dedup` 加狀態 claim | `mq/restock_consumer.go` | 單體版可以直接用 River 的唯一 job（以 event_id 當冪等鍵），不需要 dedup 表 |
| PII 清理 cron（每天 19:00 UTC，一次 100 筆，`outbox.DailyScheduler`） | `app/cron/pii_cleanup.go` | River periodic job；有缺陷，見 §8 |
| 刪除流程的 `rolestate` 三態（`HasActiveSeller` → `RemoveBuyerRole(other)`）與 `source_id` 柵欄 | `usecase/delete_buyer_account.go`、`complete_pending_deletion.go` | identity 的 AGENTS 已經寫明：A3 時改成 buyer 在同一個 tx 內授予或移除角色，拿掉 RoleState 與 `source_id` |
| `EnsureProfile` 內的 `AddBuyerRole`（補 account_type 的 buyer bit，fail-open） | `buyer_profile.go:93-99` | bitmask 已經不存在（identity 只有 `account_roles`）；A3 改成同一個 tx 內授予 |
| `GetAccountInfo` 在確保 profile 時回填 email／phone | `buyer_profile.go:85-101` | identity 沒有手機；是否保留 email 副本要決定（replicas.csv 那一列） |
| 預設暱稱：token 名稱，否則 `Hawkr 用戶%04d`（`math/rand`）；只補空的暱稱 | `buyer_profile.go:24-30,117-126` | commit 210179ec1 讓 identity 每次都送驗證過的 Clerk 名稱 |
| 分頁：offset（`go-pkg/pagination`、`httputil.ParsePageParam`）；restock 用 id cursor | `grpc/handler.go:635,668` | 單體用 AIP-158（`platform/pagination`） |
| 驗證：收件人姓名的 PayUni 長度規則（ASCII 算 1、其他算 2，範圍 [4,30]） | `validation/payuni_field.go` | 規則源自物流 provider；其餘欄位（gender、email、phone、label、city/district）**在 buyer 端沒有驗證**，只做 `phone.NormalizeToLocal` |
| 電話正規化 `go-pkg/phone.NormalizeToLocal` | `domain/shipping_address.go`、`buyer_profile.go:195` | 共用 helper，要找單體裡的擁有者 |
| 錯誤：envelope `Code` 加 `pkgerrors`（DomainError），另外回 gRPC status | `grpc/handler.go` | Connect 錯誤碼加 `ErrorInfo`（ADR api-error-model） |
| 頭像 URL 由 `cfg.MediaBaseURL` 組（公司網域出現在 `config/config.go:148`，D9 baseline） | `grpc/mapper.go`、`config/config.go` | ADR `2026-09-25-api-image-uri-template.md` |
| 服務間 HMAC 簽章 interceptor（`go-pkg/middleware/grpc/interceptor`） | `infrastructure/di.go` | 單體的 legacy server 不驗簽（identity 傳輸層盤點那一列） |
| fx 組裝、OTel、`go-pkg/server`、`grpc-healthcheck` | `app/app.go`、`cmd/` | 刪除 |
| sqlc（有一支查詢用手寫 SQL：`GetPrincipalsByBuyerIDs` 的 `unnest ... LEFT JOIN`） | `repository/postgres/buyer.go:99` | 改寫成 sqlc |
| Atlas `atlas.hcl` 裡寫死公司 infra IP `203.0.113.10`（D9 baseline：`company-infra-ip`） | `db/atlas.hcl` | 隨目錄刪除 |

`go-pkg` 的 import 次數（`rg -o --no-filename 'github.com/Will413028/hawkr/go-pkg/[a-zA-Z0-9_/]+' services/buyer-service --glob '!**/*_test.go' | sort | uniq -c`）：database 18、logger 16、errors 14、outbox 8、mq 7、timeutil 6、interceptor 6、grpcclient 6、pagination 5、bizcode 5、rolestate 4+1、proto/buyer 3、media 3、syncutil 2、server 2、redis 2、phone 2、otel 2、geocode 2、config 2、tsid 1、proto/{seller,product,order,media,crm,account} 各 1、pgxutil 1、httputil 1。

樂觀鎖或 version 欄位：沒有（schema 裡沒有 `version`，也沒有 `expectedVersion`）。

### 2.1 CodeMeta 值域與錯誤碼（逐一列出）

| 項目 | 值 | 定義位置 | 用在 |
|---|---|---|---|
| `RestockSubscriptionStatus` | ACTIVE 9501／NOTIFIED 9502／CANCELLED 9503 | `業務狀態碼.md:661`，使用服務只有 buyer | `restock_subscription.status`（字串、無 CHECK）與 partial index 的 WHERE |
| `BuyerEventType` | `BUYER_FAVORITE_ADDED`／`BUYER_FAVORITE_REMOVED` | `內部技術碼.md:542`（buyer、product） | 事件的 `eventType`；product 的 dedup 表 |
| `ProductEventType`（消費端） | `PRODUCT_RESTOCK`、`PRODUCT_RESTOCK_TARGET` 等 | `內部技術碼.md:705` | restock consumer 與 fan-out |
| `MembershipEventType` | `PAYMENT_MEMBERSHIP_BUYER_SUCCESS/FAILED` 等 | `內部技術碼.md:235`（使用服務含 buyer） | **buyer 端沒有實作**（D8 不搬） |
| `MembershipBillingErrorCode` | — | `內部技術碼.md:260`（使用服務含 buyer） | buyer 端沒有使用；計費沒有接上 |
| `AssetReferenceType` | `BUYER_AVATAR` 7112 | `業務類型碼.md:352` | media 的 ref_type |
| `OrderStatus`（消費端） | 0,10,20,30,32,35,40 視為「未結」 | `業務狀態碼.md:7` | `client/order/order_client.go:64` 寫死字串 `"0,10,20,30,32,35,40"` |
| 內部 Go enum（不在 CodeMeta）：`BuyerStatus`、`MemberLevel`（bronze/silver/gold/platinum，永遠是 bronze）、membership `source`/`status`、outbox `status`、地址 `label`、`gender` | — | `domain/*`、`valueobject/member_level.go` | 前兩個有 CHECK，其餘沒有 |

錯誤碼（`bizcode`，`pkg/errors/bizcode/bizcode.go`）：

| 名稱 | 碼 | 丟出的位置 |
|---|---|---|
| `BUYER_ACCOUNT_DELETING` | 9203 | `favorite.go:352`（刪除中不能收藏） |
| `ACCOUNT_ALREADY_IN_DELETION` | 9204 | `delete_buyer_account.go:115` |
| `INVALID_PLAN_CODE` | 9400 | `membership.go:130,197` |
| `FEATURE_ADDRESS_LOOKUP_DISABLED` | 9700（與 seller 共用） | `address_geocode.go:66` |
| `SELLER_NOT_FOUND` | 9233 | seller client 的轉譯（`client/seller`） |
| 通用：NotFound、Forbidden（地址不屬於你）、InvalidInput（buyer_id/product_id ≤ 0、PayUni 姓名、會員欄位）、Conflict（`ErrBuyerCreateConflict`，內部使用） | — | 見 `rg -n 'pkgerrors\.New' services/buyer-service/internal` |

### 2.2 排程與背景工作

1. PII 清理：每天 19:00 UTC（`app/cron`）。
2. `buyer_deleted_outbox` 的 worker：每分鐘，加上監控 scheduler（`worker/buyer_deleted_outbox_monitoring.go`）。
3. RabbitMQ consumer：3 個（見 §4）。

### 2.3 外部 provider

TGOS 地址查詢（`external-systems.csv`：「via tgos-proxy/egress-proxy Cloud Run relay」）。其他外部呼叫只有內部服務。

---

## 3. RPC（`rpc-servers.csv` 48 列，`disposition` 全部空白）

呼叫方以實際程式碼為準（`scratchpad/callers.sh`：逐一比對每個 client 檔裡的 `.Rpc(` 或 Java 的 `.rpc(`）。它與 `rpc-calls.csv` 一致，只差 csv 把 buyer→order 的呼叫記成 `GetMyOrders`，實際用途是 `GetActiveBuyerOrderCount`。標記：**J** = Java（A3 之後仍在，需要 legacy gRPC server）；**G(Ax)** = 比 A3 晚搬的 Go 服務（同樣需要 legacy server，直到 Ax）；gw = gateway REST；mono = 單體的 `internal/legacy/buyerservice`；S = seller-service（和 A3 同一步搬）。

| RPC | 呼叫方 | 備註 |
|---|---|---|
| GetBuyerProfile | gw（`buyerid/resolver.go:76`，Redis `buyer:current`，**替大約 20 個 handler 檔解析 buyer_id**；還有 `buyer.go` GetMemberProfile）、S（`HasActiveBuyer` → RoleState） | 由 account_id 查 |
| GetBuyerProfileByBuyerID | gw（seller_crm LookupBuyer）、G(A6) crm、G(A7) social | |
| GetBuyerProfileBatch | **無** | 只有 invoice client 的註解提到它（警告不要用）。刪除 |
| UpdateBuyerProfile | gw | |
| EnsureBuyerProfile | mono（`internal/legacy/buyerservice`）、gw（`POST /app/buyers/me`，沒有 client 呼叫） | |
| GetAddressList | gw、G(A6) crm | |
| GetAddress | gw、**J** order | order 下單時讀地址（replicas：寫一次的快照） |
| AddAddress／UpdateAddress／DeleteAddress | gw | |
| SuggestAddress | gw | TGOS |
| ValidateAddress | gw、**J** shipping | TGOS |
| GetFavoriteList／CheckFavorite／AddFavorite／RemoveFavorite／GetFavoriteCount | gw | CheckFavorite 也用在商品詳情頁 |
| BatchCheckFavorites | gw（商品列表） | |
| ListFavoritedProductIds | **J** product | |
| ListSubscribedSellerIds | **J** product | |
| GetSubscriptionList／SubscribeSeller／UnsubscribeSeller／CheckSubscription | gw | GET list 不在 openapi 裡 |
| GetSubscribersList | gw（`/buyers/me/followers`）、G(A7) social | |
| GetSubscriberDashboardStats | gw（seller dashboard）、S（seller profile 的粉絲數，`seller-service/internal/grpc/handler.go:346`） | |
| BatchGetSubscriberCounts | S（`recommended_sellers.go:203`，A11 移到 discovery） | |
| GetSavedItems | gw | |
| GetVisitList／RecordVisit／ClearVisitHistory | gw | |
| UpdateBuyerAvatar | gw（buyer-app、website、**seller-app** 都呼叫） | |
| DeleteBuyerAccount | gw（`DELETE /admin/accounts/:id` 的編排；`DELETE /app/buyers/me` 沒有 client 呼叫） | |
| GetMemberCards／GetSpendingRecords | gw | |
| BatchGetBuyerMetaByBuyerIDs | gw（seller_crm）、G(A6) crm、G(A7) social | |
| BatchGetBuyerDisplayByBuyerIDs | **J** aftersales、coupon、order、shipping；G(A6) im、review；G(A5) invoice | 墓碑名字 |
| ResolvePrincipals（buyer_id → account_id） | **J** aftersales、cart、coupon、order、shipping；G(A6) im；G(A7) social | |
| SubscribeRestock／UnsubscribeRestock／ListRestockSubscriptions | gw | |
| BatchCheckRestockSubscriptions | gw（`applyRestockSubscribed`：商品列表、詳情、搜尋、marketplace、推薦、賣家頁、收藏、瀏覽紀錄） | |
| ListBuyerIDsByAccount | G(A6) im、G(A8) notification | 僅供內部 PII 清除 |
| GetBuyerStatsForAdmin | gw（admin members 頁） | |
| GetBuyerEntitlement | gw（admin agent_entitlements、buyer quota，**兩者都沒有 client 呼叫**）、G(A9) ai（`client/buyer/adapter.go:61`，真的在用：額度） | |
| AssignMembership | gw（沒有 client 呼叫） | |
| ListMembershipPlans／UpdateMembershipPlan | gw（admin `membership-plans` 頁） | |

**A3 之後 legacy gRPC server 必須提供的 RPC**（Java 加上比 A3 晚搬的 Go）：GetAddress、ValidateAddress、ListFavoritedProductIds、ListSubscribedSellerIds、BatchGetBuyerDisplayByBuyerIDs、ResolvePrincipals（Java 6 支）；GetBuyerProfileByBuyerID、GetAddressList、BatchGetBuyerMetaByBuyerIDs、GetSubscribersList、ListBuyerIDsByAccount、GetBuyerEntitlement（Go 6 支）；外加 gateway 的 `GetBuyerProfile`（buyer_id 解析，除非改由單體注入 `X-Buyer-Id`）。

**buyer-service 往外呼叫**（`rpc-calls.csv` 的 caller=buyer-service，14 列）：

| 被呼叫的服務.RPC | 用途 | A3 之後 |
|---|---|---|
| account.GetAccountInfo／BindBuyer／RemoveRole | 回填 email/phone、補 buyer bit、刪除時移除角色 | 在單體內改成 identity 的公開 API，同一個 tx 內授予或移除角色 |
| media.FinalizeImages／LinkImages／ReleaseReferences | 頭像 | 單體的 media 模組（`apiOnly`，經 legacy 呼叫 Java） |
| product.GetProductsByIDs | 收藏、瀏覽、補貨清單的顯示；favorite.added 與 product.viewed 事件的 payload | **catalog 不在 buyer 的依賴表**：移到 http 組裝 |
| seller.GetSellerBatch／ResolvePrincipals／GetSellerByUserId | 追蹤、收藏、瀏覽、會員卡、收藏中心的賣家顯示；事件 payload 的 seller account；刪除時的 `HasActiveSeller` | **seller 不在依賴表**：移到 http；RoleState 由 identity 在 tx 內處理而消失 |
| order.GetMyOrders（計算未結訂單數）／GetBuyerMerchantSummary／GetBuyerOrdersForSeller | 刪除的前提條件；會員卡；消費紀錄 | **ordering 不在依賴表**：會員卡與消費紀錄移到 http；「未結訂單不能刪」需要另一種做法（見 §9 Q3） |
| crm.GetBuyerVipStatuses | 會員卡的 VIP 標記 | crm 依賴 buyer，buyer 不能呼叫 crm：移到 http |

---

## 4. 事件、副本、ID 參照

### 4.1 events.csv（`rg 'buyer-service' events.csv`，16 列）

| routing key | 方向 | 對方 | 對方語言與搬遷時點 | 用途／機制 |
|---|---|---|---|---|
| `account.deleted` | 消費 | 生產者現在是單體的 identity（經 `internal/legacy/rabbitmq`） | — | 釋放頭像的 media ref（ref_id = account_id）→ A3 改成模組內直接處理 identity 的 `AccountDeleted` 事件 |
| `order.completed` | 消費 | order-service | **J** | 完成 pending 的刪除 → legacy RabbitMQ adapter 收下再轉呼叫 |
| `product.restock` | 消費 | product-service | **J** | fan-out 發出 `product.restock.target` → legacy adapter |
| `buyer.deleted` | 生產（outbox） | coupon **J**；crm、im G(A6)；notification G(A8)；social G(A7) | 混合 | 仍要經 legacy RabbitMQ 送出 |
| `buyer.favorite.added` | 生產（goroutine） | product **J**（計數）、notification G(A8)（通知賣家） | | legacy RabbitMQ |
| `buyer.favorite.removed` | 生產（goroutine） | product **J** | | legacy RabbitMQ |
| `follow.added`（exchange `hawkr.content`） | 生產（同步，失敗只記 log） | notification G(A8) | | legacy RabbitMQ |
| `product.viewed`（exchange `hawkr.content`） | 生產（goroutine） | notification G(A8)（推播給賣家）、seller-service（瀏覽統計，同在 A3） | | seller 那邊變成單體內的事件；notification 那邊走 legacy |
| `product.restock.target` | 生產（tx 內 AMQP） | notification G(A8) | | legacy RabbitMQ |
| `payment.membership.buyer.success/failed` | 「消費」 | payment **J** | `producer-only`：buyer 端沒有實作 | D8：不搬，去向填「刪除（沒有接上）」 |

`event-pairs.csv`：buyer→notification 5 種、buyer→product 2、buyer→coupon/crm/im/social 各 1、buyer→seller 1；account→buyer 1、order→buyer 1、product→buyer 1。

### 4.2 replicas.csv（涉及 buyer 的 7 列）

| holder | 內容 | 來源 | 是否刻意的快照 | 盤點意見 |
|---|---|---|---|---|
| buyer-service | `buyers.email, phone` | account-service | no | identity 已沒有手機；決定刪除或保留快照 |
| order-service | `orders.recipient_*`、`shipping_address` | buyer（`shipping_addresses`） | yes | Java 端保留 |
| product-service | `product.favorite_count` | buyer 的事件 | no | 隨 goroutine 事件飄移；B 階段 catalog 可以改成讀取模型 |
| crm-service | `crm_customers.nickname, phone` | buyer／account | no | A6 |
| social-service | `cms_comment.user_name/avatar` 等 | buyer／seller | no | A7 |
| media-service | `asset_reference(BUYER_AVATAR, account_id)` | buyer | no | |
| gateway | Redis `buyer:current:{accountId}`，60 秒 | buyer | no | 要決定：單體注入 header，或 gateway 繼續呼叫 legacy |
| payment-service | membership 的 subject／quote 快照 | buyer／seller | yes（流程處於休眠） | D8 |

### 4.3 idrefs.csv（涉及 buyer 的列）

- buyer 持有：`buyers.account_id`、`buyer_deleted_outbox.account_id`、`buyer_deletion_audit.account_id` → identity；`buyer_deletion_audit.operator_id` → identity 的 admin；`favorites/restock*/visit_history.product_id` → catalog（Java）；`seller_subscriptions.store_id` → seller；`notifications.ref_id` → 任意。
- 參照 `buyer_id` 的其他服務：aftersales、ai（大量 procurement 表）、cart、coupon、crm、im（包含 `messages.sender_id` 多型）、invoice、order、payment（包含 `subject_id`、`owner_ref_id` 多型）、review、seller（`seller_store_visitors.visitor_buyer_id`）、shipping、social（多型的 `user_id`）、identity（`account_roles.source_id`，A3 後刪除）。→ `buyer_id` 暫時必須維持 bigint。

---

## 5. 對外 REST（gateway）

路由：`gateway/internal/router/routes_buyer.go`（`/api/v1/app/buyers/me/*` 與 `/app/products/:id/restock_subscriptions`），另外還有 `routes_address.go`、`routes_crm.go`、`routes_seller.go`（dashboard）、`routes_admin.go`、`routes_ai.go`、`routes_product.go`。handler 與 RPC 的對應用 `scratchpad/enclosing.sh` 產生。

`openapi.yaml` 裡直接或部分由 buyer 提供的 operation：**40 個**（29 個 path；統計腳本在上方 transcript，以 path 找出底下的 method）。另外有 **2 條路由不在 spec 裡**（`gateway/internal/router/openapi_coverage_test.go:74,77`）：`DELETE /api/v1/app/buyers/me`（notAnnotated）、`GET /api/v1/app/buyers/me/subscriptions`（swagSkippedAnnotation）。

| path | method（operationId） | 經過的 RPC |
|---|---|---|
| `/app/buyers/me` | GET getMemberProfile、PUT updateMemberProfile、POST ensureMyBuyerProfile；DELETE（不在 spec） | GetBuyerProfile、UpdateBuyerProfile、EnsureBuyerProfile（並失效 gateway 的 buyer:current）、DeleteBuyerAccount |
| `/app/buyers/me/avatar` | PUT | UpdateBuyerAvatar |
| `/app/buyers/me/addresses[/{id}]` | GET、POST、GET、PUT、DELETE | 地址 5 支 |
| `/app/address/suggestions`、`/validate` | GET、GET | 依角色分派：Suggest/ValidateAddress 到 buyer 或 seller |
| `/app/buyers/me/favorites`、`/count`、`/{id}` | GET、POST、GET、GET、DELETE | 收藏 5 支（清單另外補上 restock 標記） |
| `/app/buyers/me/subscriptions[/{store_id}]` | POST、DELETE、GET；GET 清單（不在 spec） | 追蹤 4 支 |
| `/app/buyers/me/followers` | GET | GetSubscribersList |
| `/app/buyers/me/saves` | GET | GetSavedItems |
| `/app/buyers/me/member-cards`、`/{seller_id}/records` | GET、GET | GetMemberCards、GetSpendingRecords |
| `/app/buyers/me/browse-history` | GET、POST、DELETE | 瀏覽 3 支（清單另外補上 restock 標記） |
| `/app/buyers/me/restock_subscriptions`、`/app/products/{id}/restock_subscriptions` | GET；POST、DELETE | 補貨 3 支 |
| `/app/buyers/me/assistant/quota` | GET | GetBuyerEntitlement |
| `/app/sellers/me/customers/lookup`、`/{buyerId}/meta` | GET、GET | 賣家 CRM：GetBuyerMetaByBuyerID、GetProfileByBuyerID（加上 account 的 GetAccountByPhone，現在一律回找不到；加上 order、coupon） |
| `/app/sellers/me/dashboard` | GET | GetSubscriberDashboardStats，加上 crm、order、seller |
| `/admin/buyers/stats` | GET | GetBuyerStatsForAdmin |
| `/admin/accounts/{id}` | DELETE | 編排：先刪 seller → 再刪 buyer（admin）→ 兩邊都沒有時直接軟刪帳號（`handler/admin_account.go`；router 的註解：「留在 gateway，到 buyer／seller 搬進單體時一起搬」） |
| `/admin/agent_entitlements/{buyer_id}[/assign]` | GET、POST | GetBuyerEntitlement、AssignMembership |
| `/admin/membership_plans[/{plan_code}]` | GET、PATCH | List/UpdateMembershipPlan |
| `/app/buyers/me/seller_binding` | PUT | **不經過 buyer**（`Account.BindSeller`）；只是掛在 `/buyers/me` 群組下 |

### 5.1 「buyer 往上的補資料」

兩層，A3 都要處理：

1. **buyer-service 內部**呼叫上層模組來補顯示欄位（違反 arch 表，必須移到 `internal/httpapi` 做讀取組裝）：
   - 收藏清單：product.GetProductsByIDs（含下架商品）加上 seller.GetSellerBatch（`favorite.go:135,163`）
   - 瀏覽清單：product（只取 ACTIVE）加上 seller（`visit.go:66,87`）
   - 追蹤清單：seller.GetSellerBatch（`follow.go:130`）
   - 收藏中心：商品側 `enrichFavoritesWithProductInfo`（product 加 seller，`saved_items.go:141`），賣家側 seller.GetSellerBatch（`saved_items.go:186`）
   - 補貨清單：product（`restock_subscription.go:163`）
   - 會員卡：order.GetBuyerMerchantSummary、seller.GetSellerBatch、crm.GetBuyerVipStatuses（`member_card.go:38,64,96`）
   - 消費紀錄：order.GetBuyerOrdersForSeller（`spending_record.go:50,67`）
   - 事件 payload 的補資料：favorite.added 需要商品名、store、賣家 account；product.viewed 同樣；follow.added 需要賣家 account（`ResolvePrincipal`）
   - 刪除的前提條件：order 的未結訂單數；seller 的 `HasActiveSeller`
2. **gateway 端**把 buyer 的資料疊到別的模組的讀取結果上：`applyRestockSubscribed`（product 列表與詳情、search、marketplace、recommendation、seller 商品頁、favorites、visits；設計書 39 §5.5b「isRestockSubscribed Gateway Aggregation 矩陣」）與 `BatchCheckFavorites`／`CheckFavorite`（商品列表與詳情）。那些路由屬於 Java 的 catalog，A3 之後還在 gateway；它們需要一個「目前買家對這些商品是否已收藏或已訂閱補貨」的來源（legacy gRPC 或單體的內部 API）。

---

## 6. Client（呼叫上述 REST 的檔案）

用 `scratchpad/clients.sh`（`rg -l -F <path 片段>`，排除 test、gen、node_modules）。

- **website**（大約 13 個 repo 檔）：`lib/addresses/repo.ts`、`lib/favorites/repo.ts`、`lib/follow/repo.ts`（subscriptions、followers）、`lib/hooks/useFollowedSellers.ts`、`lib/member-cards/repo.ts`、`lib/visits/repo.ts`、`lib/hooks/useRecordHistory.ts`、`lib/restock-subscriptions/repo.ts`、`features/product/components/ProductPurchaseBox.tsx`、`lib/saved-items/repo.ts`、`lib/profile/repo.ts`（GET/PUT `/app/buyers/me`、avatar）、`lib/seller/shipping.ts`（地址查詢）、`lib/seller/repo.ts`（customers lookup/meta、dashboard、seller_binding）、`lib/hooks/useSellerDashboard.ts`、`features/seller/onboarding/SellerOnboardingClient.tsx`（validate）、`features/seller-center/orders/SellerOrdersPage.tsx`（customer meta）。另外 `lib/hooks/useAccount.ts`、`lib/auth/teardown.ts`、`lib/chat/chatIdentity.ts`、`components/shared/RoleZoneGuard.tsx`、`features/feed/components/*` 引用 `buyers/me`（多數是註解或角色判斷，逐一確認後才算）。
- **admin**（3 個檔）：`src/app/admin/members/page.tsx`（buyers/stats）、`src/app/admin/membership-plans/page.tsx`、`src/app/admin/merchants/[id]/SellerAccountDeleteDialog.tsx` 與 `page.tsx`（DELETE `admin/accounts`）。**沒有** agent_entitlements 的頁面。
- **buyer-app**（大約 14 個檔）：`features/address/data/address_repository.dart`、`features/cart/data/cart_repository.dart`（addresses）、`features/address/data/tgos_address_service.dart`、`features/product/data/product_repository.dart`（favorites、browse-history、restock）、`features/product/presentation/merchant_item_detail_screen.dart`、`features/visit/data/visit_repository.dart`、`features/seller/data/seller_repository.dart`、`features/seller/presentation/merchant_screen.dart`、`features/follow/data/follow_repository.dart`、`features/settings/presentation/my_subscription_screen.dart`、`features/member_card/data/member_card_repository.dart`、`features/auth/data/auth_repository.dart`（GET/PUT `/app/buyers/me`）、`features/settings/presentation/profile_edit_screen.dart`（avatar）、`features/settings/data/buyer_profile_fields.dart`（gender、level）。**沒有呼叫** `/buyers/me/restock_subscriptions`、`/saves`、`/followers`、DELETE `/buyers/me`。
- **seller-app**（大約 7 個檔）：`features/customer/data/subscriber_remote_service.dart`（followers、meta）、`features/customer/data/customer_repository.dart`（meta）、`features/coupon/data/coupon_repository.dart`（customers/lookup）、`features/dashboard/*`、`features/seller/data/seller_repository.dart`（**GET/PUT `/app/buyers/me` 與 `/buyers/me/avatar`：seller-app 把買家 profile 當成使用者 profile 用**）、`features/settings/data/tgos_address_service.dart`。

---

## 7. 單體目前的耦合

- `internal/legacy/buyerservice/buyerservice.go`（109 行）：`Client.Ensure`（Connect gRPC，3 秒 timeout，帶 `x-account-id` header，HMAC 簽章）；`Profiles.Ensure` 先同步呼叫，失敗時排 River job `legacy.ensure_buyer_profile`（MaxAttempts 10，大約 3 小時）；`Worker` 負責重試。組裝點在 `cmd/hawkr/main.go:147,189`（serve）與 `:256,272`（worker）。
- 呼叫點：`internal/httpapi/session.go:58-59`（新帳號而且 `ActsAsBuyer`）、`internal/httpapi/identity/app.go:71-72`（每次 `GetSession`）。介面是 `httpidentity.BuyerProfiles`（`internal/httpapi/identity/app.go:21`）。`ActsAsBuyer`：`X-Active-Role` 是 BUYER，或沒有提示而帳號有買家角色。
- commit 210179ec1：identity 每次都把驗證過的 Clerk 名稱放進 `rpc.Principal.Name`，兩個呼叫點都傳它當 `suggestedNickname`；buyer-service 只補空的暱稱（`buyer_profile.go:117-126`）。測試在 `internal/httpapi/session_nickname_test.go`（同一帳號的並行首次請求）。
- 已知限制（寫在套件註解）：單體碰不到 gateway 的 Redis，stint 換新之後 `buyer:current` 最多 60 秒才過期。
- EnsureProfile 會回呼單體：`GetAccountInfo`、`BindBuyer`（AddBuyerRole）經 `internal/legacy/accountservice/server.go:70,113`；刪除時 `RemoveRole` 經 `:135`（實作是 identity，含 `source_id` 柵欄與 RoleState）。
- identity 的 AGENTS 裡**延到 A3 的項目**：
  1. `account_roles.source_id` 柵欄（#3189）：A3 後拿掉欄位與柵欄（L44、L82）。
  2. RoleState 三態：A3 時由 buyer、seller 在建立或刪除 profile 的同一個 tx 內授予或移除角色；刪帳號改成上層明確的 use case（L143）。
  3. 「建帳時授予 BUYER」改由 buyer 模組授予（L112、L143）。
  4. 買家 profile 的建立由 buyer 模組接手（`ActsAsBuyer` 的過渡邏輯，L59、L141）。
  5. 賣家 CRM 以手機找顧客現在一律回找不到；改用什麼資料隨 A3 決定（L49、L113）。
  6. `admin DELETE /accounts/:id` 的編排留在 gateway，A3 時搬（`routes_admin.go` 的註解）。
- arch 依賴表：`buyer: {identity, media}`；依賴 buyer 的有 fulfillment、catalog、invoice、cart、coupon、ordering、aftersales、review、im、crm、social、discovery、agent、notification。**seller 不能依賴 buyer**。

---

## 8. 可疑的缺陷與出人意料的行為

**D1 PII 清理會清掉仍在使用的 stint（真缺陷）**：`ListBuyersPendingPIICleanup` 選出 `deleted_at < now-30d AND pii_cleared_at IS NULL` 的 `account_id`，接著 `SetBuyerPIICleared` 以 `WHERE account_id=$1 AND pii_cleared_at IS NULL` 更新「該帳號的全部 stint」（`db/queries/buyers.sql` 的 SetBuyerPIICleared 與 ListBuyersPendingPIICleanup；cron 在 `app/cron/pii_cleanup.go`）。一個帳號刪除買家身分之後 30 天內重新註冊，新的 active stint 的暱稱會被改成 `'已刪除帳號'`，email、phone、avatar 也被清掉。加上查詢沒有 ORDER、每天只處理 100 筆、沒有迴圈，積壓會一直拖下去。修法：以 `buyer_id` 為鍵，只清 closed 的 stint。

**D2 取消收藏重複扣計數**：`Remove` 不管有沒有刪到列都發 `buyer.favorite.removed`，而且每次 `EventID` 都是新的 UUID（`favorite.go:411-420`、`mq/publisher.go`）。註解說「consumer dedup 加 favorite_count > 0 的保護」，但 dedup 以 event_id 為鍵，擋不住。對同一個商品連點兩次取消，會把別人的收藏數也扣掉。

**D3 事件可能遺失**：favorite 與 product.viewed 用 goroutine 發、`follow.added` 失敗只記 log，都沒有 outbox，所以 `favorite_count` 與通知都可能漏。

**D4 重複追蹤會重發 `follow.added`**：`CreateSubscription ... ON CONFLICT DO NOTHING` 之後 repo 視為成功（`repository/postgres/follow.go:32-35`），use case 照樣發事件，賣家會重複收到「被追蹤」的推播。追蹤不檢查 store 是否存在、不擋 pending_deletion（收藏會擋，行為不一致），也不擋追蹤自己的店。

**D5 每次瀏覽都發 `product.viewed`**，notification-service 有 `product.viewed` 的模板（`notification-service/db/migrations/20260703160000_1176_flipped_events_template_snake.sql:65`），可能每次瀏覽都推播給賣家。要對照設計書確認這是不是本意。

**D6 刪除流程在 tx 內呼叫外部服務**：`delete_buyer_account.go` 與 `complete_pending_deletion.go` 在持有 `GetByAccountIDForUpdate`／`GetByBuyerIDForUpdate` 鎖的 tx 裡呼叫 order-service 的 `GetActiveBuyerOrderCount`（5 秒 timeout）。

**D7 pending_deletion 也會馬上移除角色**：tx 之後的 `RemoveBuyerRole` 在 pending 的路徑上也會執行（註解有寫是刻意的，原本用意是撤銷 token）。在 Clerk／identity 之下，如果這是最後一個角色，identity 會立刻刪帳號與 Clerk 使用者，並發出 `AccountDeleted` → buyer 釋放頭像的 ref（買家還在 pending，訂單畫面仍會顯示頭像）。要重新決定 pending 的語意。

**D8 軟刪的地址永遠保留 PII**：買家刪除時 `SoftDeleteShippingAddressesByBuyerID` 保留收件人姓名、電話、地址，PII cron 不處理這張表。order 在下單時已經把地址複製成快照（replicas.csv），所以地址不需要軟刪。

**D9 刪除時沒有清 Redis 的收藏集合，也沒有發 favorite.removed**：`ClearAssociatedData` 硬刪 favorites，但 product 的 `favorite_count` 不會減。

**D10 沒有「每個買家一個 default 地址」的約束**；刪除 default 地址之後沒有遞補；`Update` 可以把唯一的 default 設成 false。

**D11 缺 CHECK**：`restock_subscription.status`、`buyer_deleted_outbox.status`、`buyers.gender`、`buyers.level`、`shipping_addresses.label` 都沒有 CHECK。

**D12 會員冪等鍵**：`AssignMembership` 發現相同的 `request_id` 時，直接回既有的列，不比對 buyer／plan 是否相同。

**D13 註解過時**：`address_geocode.go` 與 cache 的註解還寫 Google Maps（已經換成 TGOS，#1533）。`delete_buyer_account.go` 的註解還寫「revoke token + Redis blacklist」（Clerk 之後已不適用）。

**D14 seller-app 仰賴買家 profile**：seller-app 以 `GET/PUT /app/buyers/me` 當使用者 profile（`seller-app/lib/features/seller/data/seller_repository.dart:132,175`），可是單體只有在 `ActsAsBuyer` 時才確保 profile。只開過 seller-app 的帳號會拿到找不到（要實際驗證）。

**D15 admin 刪除時的部分失敗**：`DELETE /admin/accounts/:id` 先強制刪 seller；買家若是 pending_deletion，buyer 端回 9204，整個請求失敗，但 seller 已經刪了。

**D16 `restock_event_dedup` 沒有清理**，會無限成長。

**D17 賣家 CRM 的 lookup 依手機**，現在一律回找不到（identity 的限制），但 `buyers.phone` 其實還有（從 profile 編輯來的）。資料擁有者要在 A3 決定。

**D18 `atlas.hcl` 寫死公司的 DB IP**，`config.go` 有公司的媒體網域（D9 baseline 5 個檔）；刪除舊服務目錄時一起消失。

### 8.1 設計書有、但程式沒接上（D8：不搬，去向寫「刪除」）

| 項目 | 證據 |
|---|---|
| `notifications` 表與通知 RPC | 只剩 DELETE 查詢；設計書 39 §2.5、§5.4 |
| `MemberLevel` 升等（silver/gold/platinum） | `UpgradeLevel` 沒有呼叫方（`rg -n 'UpgradeLevel' services/buyer-service --glob '!*_test.go'` 只找到定義）；設計書附錄寫「預留」；buyer-app 仍然顯示 `level` |
| `GetBuyerProfileBatch` | 0 個呼叫方 |
| 會員計費（`payment.membership.buyer.*`、設計書 60、ADR `2026-08-29-membership-billing-payuni-period.md`） | events.csv 標 `producer-only`；payment 的 provider adapter 丟出 UNSUPPORTED；D8 明列 |
| 買家自助刪除 `DELETE /app/buyers/me` | 不在 openapi 裡，四個 client 都沒有呼叫；只有 admin 的編排用到刪除的 use case。注意 App Store 5.1.1(v) 要求 App 內可以刪帳號（identity 的 `RevokeAppleSignIn` 是為此而設），所以這是「刪除」還是「A3 補做」要決定 |
| `POST /app/buyers/me`（EnsureMyBuyerProfile） | 沒有 client 呼叫；單體在 GetSession 時確保 |
| 後台 `agent_entitlements`、`/buyers/me/assistant/quota` | 沒有 client 呼叫；`GetBuyerEntitlement` 本身仍被 ai-service 使用 |
| `restock_event_dedup.event_type` | CodeMeta 說「可選，未必落表」，schema 也沒有這個欄位 |
| legacy backlog：buyer #91 個人資料的「行政區」「個人簡介」沒有後端欄位；#2345 account_id 的位置登記；#3524 `buyer_memberships.source` 的付費續約值域（隨計費，不搬） | 私人封存的 legacy-backlog §A3、L129 |

---

## 9. 需要決定的設計題（給 A3 的計畫）

1. **追蹤（`seller_subscriptions`）歸誰？** seller 需要粉絲數（profile、dashboard、推薦），但 arch 表不准 seller → buyer；buyer 需要 store 的顯示。選項：留在 buyer，粉絲數由 http 組裝；或搬到 seller（「追蹤」是 store 的聚合）；或 seller 維護一個讀取模型（buyer 發事件）。
2. **往上的補資料移到 http**：會員卡、消費紀錄、收藏清單、瀏覽、追蹤、收藏中心都改成 `internal/httpapi` 的讀取組裝（buyer 只回 ID 與時間）。不過 product 與 order 在 Java，http 層需要 legacy client（catalog／ordering 的公開 API 先以 legacy 實作，D8 的做法）。
3. **「有未結訂單不能刪買家」**：buyer 不能依賴 ordering。選項：由上層（http 或 ordering）協調；或 buyer 維護「未結訂單數」的讀取模型（依 order 事件，Java 期間經 legacy consumer）；或刪除的 use case 放在比 buyer 更上層的組裝點。另外還要決定 pending_deletion 的語意與自助刪除要不要做（§8 D7、§8.1）。
4. **gateway 的 buyer_id 解析**：A3 之後 20 多個 gateway handler 仍然需要 `account → buyer_id`。選項：(a) legacy gRPC server 繼續提供 `GetBuyerProfile`（加上 60 秒快取與失效問題）；(b) 單體在 `gatewayproxy` 轉發時注入已驗證的 `X-Buyer-Id`（同 `X-Account-Id` 的做法），gateway 的 resolver 與 Redis 快取可以刪除。
5. **gateway 的 restock／favorite 疊加**：catalog 路由還在 gateway 期間的資料來源（legacy gRPC 保留 `BatchCheckFavorites`／`BatchCheckRestockSubscriptions`／`CheckFavorite`，或 gateway 改打單體的內部端點）。
6. **legacy gRPC server 的範圍**：Java 6 支，加上晚搬的 Go 6 支，加上 gateway 需要的（第 4、5 點）。
7. **事件**：buyer.deleted、favorite.*、follow.added、product.viewed、restock.target 仍經 `internal/legacy/rabbitmq` 送出（同一個 tx 內寫入、由 River job 轉送）；`order.completed`、`product.restock` 由 legacy consumer 接收；`account.deleted` 改成模組內的 identity 事件。
8. **TGOS 地址查詢**：依賴公司的 GCP relay（D9）。要重新開通（新的 proxy 與憑證、放哪個模組、同步 client 的 timeout），還是刪掉這項功能（client 已經能處理 9700）？
9. **Redis**：收藏集合與地址快取，建議都刪除（單體沒有 Redis）。
10. **ID**：`buyer_id` 用 IDENTITY bigint（計畫 L201）；其他表是否全部改成 IDENTITY；`membership_plans` 拿掉 id、以 `plan_code` 當主鍵。
11. **`buyers.email`/`phone` 副本**：刪除（email 在 identity）或保留為買家可編輯的聯絡資料；會影響賣家 CRM 依手機查詢（identity 延到 A3 的第 5 項）。
12. **角色授予**：「建帳時授予 BUYER」（identity）改成 buyer 建 profile 時在同一個 tx 內授予；`ActsAsBuyer` 與 legacy 重試 job 移除；`source_id` 與 RoleState 刪除。buyer 呼叫 identity 的公開 API（依賴表允許）。
13. **stint 模型與 ADR 07-11**：在 IDENTITY 主鍵下是否保留（稽核與 PII 清理仍需要），或簡化。依 ADR 狀態欄「A3 搬遷時核對」。
14. **CodeMeta 的去向**：RestockSubscriptionStatus → 模組的 Go 型別加 CHECK（值改成 `ACTIVE/NOTIFIED/CANCELLED`？）；BuyerEventType、ProductEventType 只剩 legacy 事件用；9203/9204/9400/9700 → `ErrorReason`；9700 與 seller 共用。
15. **對外 Connect API**：以 `api/` 重新設計（不照搬路徑與 envelope）；四個 client 大約 37 個檔要換（§6）。
16. **特性測試**：比照 identity 與 content，舊的一側是否還能啟動（buyer-service 需要 account-service 的 legacy server，目前由單體提供），決定寫或退役。
