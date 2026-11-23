# A8 notification 遷移盤點（唯讀）

- Snapshot commit: `894625b5449687c628736b5e0956cafa5f243d83`（branch dev；工作區既有改動 `backend/go.work.sum`、`scratch_old.yaml` 與本盤點無關，未動）
- 路徑基準：repo 根 `/Users/will/second-brain/.projects/personal/hawkr`；`backend/...` 省略前綴時，notification-service 內路徑以 `NS/` 代表 `backend/services/notification-service/`，gateway 以 `GW/` 代表 `backend/gateway/internal/`。
- 「去向」欄由 A8-0 依計畫步驟填（2026-11-23）。
- 證據等級：標 (測) 者是實際重放 migration 後查出的結果；標 (讀) 者是讀程式碼推得；標 (猜) 者未驗證。
- 方法備註：把 `NS/db/migrations/` 107 個檔依檔名排序重放進一個臨時本機 Postgres（unix socket、無網路、用完已刪），得到最終 schema 與 `notification_templates` 94 列（92 enabled）。與 `NS/db/schema/schema.sql` 的 6 張表逐欄一致（無 drift）。與 `backend/docs/plans/2026-09-24-go-modular-monolith/event-anomalies.md:3` 記載的「94 rows, 92 enabled」相符。
- 既有設計文件（本盤點不重複決策，只補現況）：ADR `docs/adr/2026-10-05-notification-module.md`（accepted，9 項決策）、主計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md:298-305`（A8 段）、`:276-278`（A7 延後到 A8 的項目）、`:263`（A6 延後到 A8 的項目）。

## 0. 服務概觀

| 項目 | 內容 | 來源 |
|---|---|---|
| 形態 | Go + fx，gRPC 服務（`GRPC_PORT` 預設 10037），無 HTTP；DB `hawkr_notification` | `NS/internal/app/app.go:14-41`、`NS/internal/config/config.go:96` |
| 模組 | config / infrastructure(FCM+otel) / repository / mq(publisher) / client(buyer、seller gRPC) / usecase / grpc / worker | `NS/internal/app/app.go:18-40` |
| 消費 | 15 個 exchange 各一個 `#` queue（動態模板）＋ 3 個靜態 queue（`buyer.deleted`、`account.deleted`、`seller.deleted`） | `NS/internal/worker/consumer.go:41-57`、`NS/internal/worker/account_deleted.go:137-143` |
| 發布 | `hawkr.notification` 的 `notification.inbox.new`、`notification.unread_changed`（給 gateway SSE） | `NS/internal/mq/publisher.go:39-57` |
| 外部 | FCM（唯一推播通道）；gRPC 呼叫 buyer-service、seller-service 各 1 支（列舉歷史 id） | `NS/internal/infrastructure/firebase.go`、`NS/internal/client/{buyer,seller}/client.go:35` |
| 呼叫方 | 只有 gateway（16 支 RPC 全部；`backend/docs/plans/2026-09-24-go-modular-monolith/rpc-pairs.csv`：gateway→notification 16）。Java 服務 0 處呼叫（rg 0 hit） | 見第 1 節 |
| 單體現況 | `backend/internal/modules/` 沒有 notification 模組；`backend/api/` 只有舊 gRPC proto（`backend/api/proto/notification/notification.proto`），`api/`（Connect 契約）沒有任何 notification service（只有 `SELLER_CAPABILITY_NOTIFICATION = 10`，`api/hawkr/seller/v1alpha1/seller.proto:307`） | rg，見附錄 A |
| 舊服務是否還部署 | **不確定**：`backend/cmd/hawkr/seller_events_test.go:13` 與 `backend/internal/legacy/sellerrelay/relay.go:10` 寫「舊通知服務沒有部署」，但 `backend/docker-compose-local.yml:1139` 仍有 notification-service，relay 的 doc comment 仍寫「notification-service 還在消費」 | 見不確定清單 U1 |

## 1. RPC / REST / SSE 端點與各 client 呼叫方

gRPC 服務定義：`backend/api/proto/notification/notification.proto:10-41`（16 RPC；同檔另有 `backend/services/java/shared/java-proto/src/main/proto/notification/notification.proto` 副本）。REST 前綴 `/api/v1`（`GW/router/constants.go:7`；`/app` 要 JWT，`/admin` 要 admin）。gateway 全部經 `GW/grpc/notification*_client.go`（16 個 RPC 包裝＋`Close`，各自 5–30 秒 timeout；`BroadcastAnnouncement` 30 秒，`notification_inbox_client.go:46-47`）。

說明欄「忽略 Code」＝gateway handler 丟掉 response 的 `Code` 欄位（service 以 `Code != 0` 回業務錯誤、gRPC error 為 nil），見第 7 節。

| # | RPC | proto | service 實作 | REST 路由（gateway 路由檔:行） | gateway handler | gateway 忽略 resp.Code？ | website | admin | buyer-app | seller-app | 去向 |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | GetNotificationPreferences | proto:10 | `NS/internal/grpc/server.go:65` | GET `/app/buyers/me/notification-preferences`（`routes_buyer.go:15`）；GET `/app/sellers/me/notification-preferences`（`routes_seller.go:33`，`CapNotification`） | `notification_preference.go:124`（買）、`:37`（賣） | 是（直接讀 resp.Preferences） | `website/lib/notifications/repo.ts:164` | - | `buyer-app/lib/features/notification/data/notification_repository.dart:153` | `seller-app/lib/features/notification/data/notification_repository.dart:166` | 改寫成 Connect 偏好讀取（受眾與分類只來自註冊表；A8-2） |
| 2 | UpdateNotificationPreference | proto:13 | `server.go:99` | PUT 同上兩條（`routes_buyer.go:16`、`routes_seller.go:34`） | `notification_preference.go:170`（買）、`:72`（賣） | **是（`_, err :=` :99、:205）** | `repo.ts:184` | - | `notification_repository.dart:176` | `notification_repository.dart:192` | 改寫成 Connect 偏好寫入（分類與受眾驗證只在註冊表，缺陷 #8、#9、#11 消失；A8-2） |
| 3 | RegisterDeviceToken | proto:16 | `server.go:120`（只收 `platform=="ios"`） | POST `/app/notifications/device-token`（`routes_notification.go:14`）；PUT 同路徑（`:15`，`RefreshDeviceToken` 走同一 RPC，`notification.go:46`） | `notification.go:29`、`:53` | 是（`_, err =` :95；service 對錯誤 return gRPC error，影響小） | 0 | - | `device_registrar.dart:45`（POST） | `device_registrar.dart:75`（POST） | 改寫成 Connect 裝置 token 註冊（不帶 `platform`，每次啟動呼叫並更新 `last_registered_at`；D7；A8-2） |
| 4 | RemoveDeviceToken | proto:19 | `server.go:144` | DELETE `/app/notifications/device-token`（`routes_notification.go:16`） | `notification.go:123` | 是（:159） | 0 | - | `device_registrar.dart:74` | `device_registrar.dart:119` | 改寫成 Connect 裝置 token 刪除（A8-2） |
| 5 | ListNotifications | proto:37 | `server_inbox.go:14` | GET `/app/notifications?cursorId&pageSize&category`（`routes_notification.go:19`） | `notification_inbox.go:100` | 否（:127 檢查） | `repo.ts:68` | - | `notification_repository.dart:43` | `notification_repository.dart:42` | 改寫成 Connect 收件匣列表（cursor 分頁、分類篩選，`total` 跟著篩選；A8-2） |
| 6 | GetUnreadCount | proto:38 | `server_inbox.go:61` | GET `/app/notifications/unread-count`（`:20`） | `notification_inbox.go:155` | 否 | `repo.ts:108` | - | `:65`（另 `notification_controller.dart:323` 的訪客短路） | `:66` | 改寫成 Connect 未讀數（A8-2） |
| 7 | MarkAsRead | proto:39 | `server_inbox.go:76` | PATCH `/app/notifications/:id/read`（`:21`） | `notification_inbox.go:195` | **是（`_, grpcErr :=` :212）** | `repo.ts:123` | - | `:89` | `:93` | 改寫成 Connect 已讀（不存在或非本人回 NotFound；A8-2） |
| 8 | MarkAllAsRead | proto:40 | `server_inbox.go:94` | PATCH `/app/notifications/read-all`（`:22`） | `notification_inbox.go:235` | 否 | `repo.ts:129` | - | `:105` | `:112` | 改寫成 Connect 全部已讀（A8-2） |
| 9 | DeleteNotifications | proto:41 | `server_inbox.go:109` | DELETE `/app/notifications` body `{ids}`（`:23`） | `notification_inbox.go:280` | 否 | `repo.ts:144` | - | `:128` | `:138` | 改寫成 Connect 刪除（A8-2） |
| 10 | (SSE，非 RPC) `notifications/stream` | 無（gateway 自己的 SSE） | 無；資料來自 MQ `hawkr.notification` | GET `/app/notifications/stream`（`routes_notification.go:26`；限流豁免 `GW/middleware/ratelimit.go:103`） | `GW/handler/sse_stream.go:29` → `GW/realtime/notification_channel.go` → `GW/mq/consumer.go` | n/a | `website/lib/sse/useNotificationStream.ts:170` | - | 0（設計上「僅 web」，`NS/internal/mq/publisher.go:8`） | 0 | 改寫成 Connect server streaming，只送未讀數與最新收件匣 ID 的失效訊號（D11），同一串流承載 `seller_workspace` 事件（U10；A8-2） |
| 11 | ReloadTemplates | proto:22 | `server_admin.go:17` | POST `/admin/notification-templates/reload`（`routes_admin_notification.go:10`） | `admin_notification_template.go:99` | 否 | - | `admin/src/app/admin/notification-templates/page.tsx:271` | - | - | 刪除（模板退場，後台不做模板編輯、預覽、試發；A8-5 刪頁、A8-7 刪 gateway 路由與 proto） |
| 12 | ListTemplates | proto:25 | `server_admin.go:30` | GET `/admin/notification-templates`（`:11`） | `admin_notification_template.go:114` | 否 | - | `page.tsx:224` | - | - | 刪除（模板退場，後台不做模板編輯、預覽、試發；A8-5 刪頁、A8-7 刪 gateway 路由與 proto） |
| 13 | GetTemplate | proto:26 | `server_admin.go:63` | GET `/admin/notification-templates/:id`（`:12`） | `admin_notification_template.go:151` | 否 | - | `page.tsx:337` | - | - | 刪除（模板退場，後台不做模板編輯、預覽、試發；A8-5 刪頁、A8-7 刪 gateway 路由與 proto） |
| 14 | UpdateTemplateEnabled | proto:27 | `server_admin.go:83` | PATCH `/admin/notification-templates/:id/enabled`（`:14`） | `admin_notification_template.go:189` | **是（:204）** | - | `page.tsx:246` | - | - | 刪除（模板退場，缺陷 #8 隨之消失；A8-5 刪頁、A8-7 刪 gateway 路由與 proto） |
| 15 | PreviewTemplate | proto:30 | `server_admin.go:97` | POST `/admin/notification-templates/:id/preview`（`:15`） | `admin_notification_template.go:231` | 否 | - | `page.tsx:438` | - | - | 刪除（模板退場，後台不做模板編輯、預覽、試發；A8-5 刪頁、A8-7 刪 gateway 路由與 proto） |
| 16 | TestSendTemplate | proto:31 | `server_admin.go:148` | POST `/admin/notification-templates/:id/test-send`（`:16`） | `admin_notification_template.go:285` | **是（:302）** | - | `page.tsx:458` | - | - | 刪除（試發路徑，缺陷 #21；A8-5 刪頁、A8-7 刪 gateway 路由與 proto） |
| 17 | BroadcastAnnouncement | proto:34 | `server_broadcast.go:36` | POST `/admin/notifications/broadcast`（`:19`） | `admin_notification_template.go:329` | 否 | 0（`website/.../DelegatedOrderDetailPanel.tsx:44` 只是註解提到） | `page.tsx:308` | - | - | 改寫成 Connect 後台公告（建立、列表；收件匣與推播分開 job，沒有 token 的人也收得到；A8-2、A8-3；admin 端 A8-5） |

（表列 17 行＝16 RPC + 1 SSE。）

補充：
- 註冊的 REST 路由共 20 條（`routes_notification.go` 9 條含 stream；`routes_admin_notification.go` 7 條；`routes_buyer.go` 2 條；`routes_seller.go` 2 條；其中 POST/PUT device-token 共用 1 支 RPC）。`GW/router/routes_admin_notification.go:13` 註明 POST/PUT 模板編輯路由已移除（SSOT 政策）。
- `backend/api/openapi/openapi.yaml` 有 notification 路徑 17 筆（rg 行 4973–5365、8388、11567–11996、17062），這是 gateway REST 的契約來源（`AGENTS.md`「舊 gateway 的 REST」列）。
- 單體入口對這些路徑的處理：`backend/internal/httpapi/session.go:92-101` 明列放行停權帳號的 `DELETE /api/v1/app/notifications/device-token`（其餘寫入對停權帳號回 ACCOUNT_SUSPENDED）；SSE 經 `backend/internal/legacy/gatewayproxy` 反代（`backend/internal/httpapi/entry_test.go:406`、`backend/internal/legacy/gatewayproxy/proxy_test.go:135`）。
- buyer-app 用 `suspension_signal.dart:39` 以 regex 辨識 `PATCH /app/notifications/{id}/read`（停權時的前端行為依賴此路徑）。
- 授權：賣家偏好路由掛 `authz.CapNotification`（`GW/authz/capability.go:47`、`backend/api/.../seller.proto:307`）；admin 模板路由只有 `requireAdmin()`（`GW/router/routes.go:31`），沒有更細的 admin 權限（讀）。
- 身分空間：gateway 在呼叫前把 account 解析成 `buyer_id`（買）或 `store_id`（賣）再放進 `user_id`（`GW/handler/notification_inbox.go` 的 `resolveNotificationUserID`）；service 完全不認得 account_id。

### 1.1 REST 呼叫方計數（非測試檔；指令見附錄 A）

| client | `app/notifications` | `notification-preferences` | `admin/notification` |
|---|---|---|---|
| website | 16 | 7 | 1（註解） |
| admin | 0 | 0 | 9 |
| buyer-app/lib | 12 | 3 | 0 |
| seller-app/lib | 7 | 3 | 0 |
| packages | 0 | 0 | 0 |
| backend/internal（非測試） | 1（`session.go:92`） | 0 | 0 |

SSE 事件契約（website 消費端）：`notification.new`（`{item, unreadCount}`）、`notification.unread_changed`（`{unreadCount}`），見 `GW/realtime/notification_channel.go:84-110`；website 用 `website/lib/notifications/events.ts`、`unread-context.tsx`、`stream-item.ts`。SSE topic 為 `notification:user:{id}`（`notification_channel.go` 的 `TopicNotificationUser`）。

## 2. DB 表（`NS/db/schema/schema.sql`，與重放 107 個 migration 的結果一致）

| # | 表 | 欄位（值得注意者） | 索引／限制 | 寫入者 | 讀取者 | 來源 | 去向 |
|---|---|---|---|---|---|---|---|
| 1 | `notifications`（收件匣，存渲染後快照） | `id` TSID、`user_id`（buyer→buyer_id，seller→store_id）、`user_type`、`event_type`、`reference_id`（可 NULL）、`title`/`body`（渲染後文字）、`data` jsonb（含 `category`、`type`、`url`、`imageUrl`…）、`is_read`、`read_at`、`deleted_at` | PK id；`notifications_inbox_dedup_idx` UNIQUE (user_id,user_type,event_type,reference_id) WHERE reference_id IS NOT NULL；`idx_notifications_inbox` (user_id,user_type,id DESC) WHERE deleted_at IS NULL；`idx_notifications_unread`；`idx_notifications_cleanup_deleted` | template handler、BroadcastAnnouncement、cleanup | gateway REST/SSE 經 RPC 5–9 | `schema.sql:15-34`、`db/queries/notifications.sql` | 改寫成 新模組收件匣表（型別＋參數快照＋聚合鍵＋actor 清單＋已讀，讀取時依請求語系組字；A8-1）；舊資料不搬（系統未上線） |
| 2 | `push_notifications`（投遞紀錄＋冪等） | `id`、`user_id`、`user_type`、`event_type`、`reference_id`、`status`(`sent`/`partial`/`failed`/`skipped`)、`error_log`（失敗 token 前綴+訊息）、`sent_at` | PK；`push_notifications_dedup_idx` UNIQUE WHERE status='sent' AND reference_id IS NOT NULL；`push_notifications_user_idx`；冪等實際用 `ExistsPushProcessed`（不分 status） | `SendPushUseCase` | `ExistsPushProcessed`、cleanup | `schema.sql:79-92`、`queries/push_notifications.sql:1-18` | 改寫成 每渠道投遞紀錄（待送／已送／永久失敗、實際送出的文字與 locale，UNIQUE（意圖、收件人、渠道）；D10；A8-1、A8-3） |
| 3 | `device_tokens` | `id`、`user_id`、`user_type`、`token`、`platform`（註解 android\|ios，但 RPC 只收 ios）、`is_active`、`created_at`、`updated_at`（被當作「最後使用 App」） | UNIQUE (user_id,user_type,token)；`idx_device_tokens_user`；無 token 單獨唯一鍵 | Register/Remove RPC、FCM 失敗分類、seller.deleted、cleanup(90 天) | `SendPushUseCase`、Broadcast（分頁列舉） | `schema.sql:1-13`、`queries/device_tokens.sql` | 改寫成 以 token 為主鍵、綁一個帳號與一支 App、記 `last_registered_at`、無 `platform` 的裝置 token 表（D7；A8-1、A8-3） |
| 4 | `notification_preferences` | `user_id`、`user_type`、`category`、`enabled` | UNIQUE (user_id,user_type,category)；沒有列＝預設開 | Update RPC | `SendPushUseCase`（只擋推播）、Get RPC | `schema.sql:36-47`、`queries/notification_preferences.sql` | 改寫成 偏好覆寫列 `(account_id, 受眾, 分類, 渠道, enabled)`，只存覆寫（D4；A8-1） |
| 5 | `notification_templates` | `exchange`、`routing_key`、`event_type`、`target_user_type`、`target_user_field`、`category`、`title_template`/`body_template`（`#{field\|default}`）、`body_fallback`、`data_type`、`data_fields` jsonb（FieldSpec：`field`/`const`）、`schema_url`（`{key}` 深連結）、`reference_id_template`、`image_key_template`、`payload_filter` jsonb（等值或 `$ne`）、`enabled`、`push_mode`（visible/silent；**無 CHECK**）、`identity_migrated`（賣家恆 true，CHECK 強制）、`updated_by` | UNIQUE (event_type,target_user_type)；CHECK `chk_notification_templates_seller_identity_migrated`；`id` IDENTITY | 只有 Atlas migration（`UpdateTemplateEnabled` RPC 只改 enabled） | `TemplateCache`（只載 enabled，每 5 分鐘＋啟動＋`ReloadTemplates`） | `schema.sql:49-77`、`db/migrations/20260703130000_template_ssot_baseline.sql`（78 列基準）起累積；現 94 列/92 enabled；enabled 中 14 silent、44 帶 image_key_template、10 帶 payload_filter（測） | 刪除（通知型別定義在程式碼的型別註冊表，94 列模板退場；A8-1、A8-7） |
| 6 | `system_notification_logs` | `notification_type`、`title`、`body`、`image_key`、`deep_link_url`、`content_hash`、`triggered_by_account_id`、`status`… | idx on triggered_at、content_hash | **無**：repository 與 domain 型別存在，但沒有接進 DI、沒有 usecase/RPC 使用（`rg SystemNotificationLog NS/internal/repository/postgres/di.go` 0 hit） | 無 | `schema.sql:94-113`、`NS/internal/repository/postgres/system_notification_log.go` | 刪除（死碼，缺陷 #22；A8-7） |

migration 統計：`NS/db/migrations/` 107 檔（含 `atlas.sum`）；最後一個 `20260917030000_delete_procurement_quote_invite_templates.sql`。已退場欄位／表（不在最終 schema）：`queue_name`（20260726042448）、`fcm_topic`（20260822130001）、舊 `image_url`→`image_key`（20260717150000）。

## 3. 事件：消費與發布

### 3.1 消費（動態：每個 exchange 一個 `#` queue；`NS/internal/worker/consumer.go:41-57`）

| # | exchange | queue（代碼） | topology 綁定 | handler | 重試 | 目前模板數（enabled/total） | 去向 |
|---|---|---|---|---|---|---|---|
| 1 | `aftersales.events` | `notification.aftersales` | `#`（`infra/mq/topology/topology.yaml:181-186`，`retry.max_attempts: 3`） | `newTemplateHandler(...)`（`NS/internal/worker/template_handler.go:22`） | MaxRetries 3（`consumer.go:101`） | 37/37 | 隨發送端搬遷（D13） |
| 2 | `hawkr.buyer` | `notification.buyer` | `#`（`:187`） | 同上 | 3 | 1/1 | 刪除（D13 不接 RabbitMQ 消費者；唯一模板 `product.favorited` 依 D3 不做；A8-7） |
| 3 | `hawkr.content` | `notification.content` | `#`（`:199`） | 同上 | 3 | 10/10 | 改寫成 social 單體事件的同步訂閱者（`comment_created`、`comment_liked`、`comment_replied`、`store_followed`、`post_published`、`store_content_taken_down`、`post_liked`；`product.viewed` 不做，D3；A8-3） |
| 4 | `hawkr.delegation` | `notification.delegation` | `#`（`:205`） | 同上 | 3 | 1/1 | 改寫成 delegation 事件的同步訂閱者（`delegation_operation_completed`；A8-3） |
| 5 | `hawkr.im` | `notification.im` | `#`（`:211`） | 同上 | 3 | 2/2 | 改寫成 `im.MessageSent` 的同步訂閱者（`chat_message`，只推播不寫收件匣；A8-3） |
| 6 | `hawkr.invoice` | `notification.invoice` | `#`（`:217`） | 同上 | 3 | 13/13 | 隨發送端搬遷（D13） |
| 7 | `hawkr.marketing` | `notification.marketing` | `#`（`:223`） | 同上 | 3 | 2/2 | 隨發送端搬遷（D13） |
| 8 | `hawkr.merchant` | `notification.merchant` | `#`（`:229`） | 同上 | 3 | 5/5（**沒有任何發送端**，見 7.14） | 改寫成 seller KYC 事件訂閱（`kyc_approved`、`kyc_rejected`；A8-3）；`forwarded`、`pending_info`、`under_review` 三個 key 隨發送端搬遷（D13） |
| 9 | `hawkr.order` | `notification.order` | `#`（`:235`） | 同上 | 3 | 7/7 | 隨發送端搬遷（D13） |
| 10 | `hawkr.product` | `notification.product` | `#`（`:253`） | 同上 | 3 | 4/4 | 改寫成 `buyer.RestockTargeted` 訂閱（`product_restocked`；A8-3）；`product.certification.*` 三個 key 隨發送端搬遷（D13） |
| 11 | `hawkr.review` | `notification.review` | `#`（`:259`） | 同上 | 3 | 1/1 | 改寫成 `review.ReviewCreated` 的同步訂閱者（`review_created`；A8-3） |
| 12 | `hawkr.seller` | `notification.seller` | `#`（`:265`） | 同上 | 3 | 2/2 | 改寫成 `seller.StoreSuspended`、`StoreUnsuspended` 的同步訂閱者（`store_suspended`、`store_unsuspended`；A8-3） |
| 13 | `hawkr.shipping` | `notification.shipping` | `#`（`:277`） | 同上 | 3 | 5/5 | 隨發送端搬遷（D13） |
| 14 | `payment.events` | `notification.payment` | `#`（`:247`） | 同上 | 3 | 1/1 | 隨發送端搬遷（D13） |
| 15 | `procurement.events`（`mq.ExchangeProcurement`） | `notification.procurement` | `#`（`:241`） | 同上 | 3 | 1/2（外部業者那列是 inert 標記，disabled） | 改寫成 `procurement.RfqPublished` 的同步訂閱者（`rfq_invited`；A8-3）；外部業者 key 隨發送端搬遷（D13） |

萬用字元訂閱：上面 15 條全是 `#`（`consumer.go:99`），收到後先以 `(exchange, routing_key)` 查 `TemplateCache`，查不到就 debug log 丟棄（`template_handler.go:31-41`）。事件進來但沒有模板的 routing key 數：39（`event-anomalies.md:17` 的清單，設計時盤點；本次未重算）。
**不在 map 的 exchange**：`hawkr.notification`（自己的發布目標，`consumer.go:34-40` 註明刻意不消費）。DB 有一列模板指向它（`notification.announcement`，disabled，標記列），所以即使被 enabled 也永遠不會被消費；啟動時 `consumer.go:78-85` 會 log error 提示這種情況（但它 disabled 所以不會進 cache）。

### 3.2 消費（靜態；個資清除）

| # | exchange / routing key | queue | handler | 動作 | 來源 | 去向 |
|---|---|---|---|---|---|---|
| 1 | `hawkr.buyer` / `buyer.deleted` | `notification.buyer.deleted`（`pkg/mq/exchanges.go:105`） | `NS/internal/worker/account_deleted.go:44-95` | 以 `accountId` 列舉歷史 buyer_id/store_id（呼叫 buyer-service、seller-service 各一次）→ 紅線化 `chat.message.sent` 的收件匣列（`title=''`、`body=''`、移除 `senderName`、`contentPreview`、軟刪）→ 軟刪退休 buyer stint 的整個收件匣 | `worker/account_deleted.go:34-95`、`notifications.sql:61-91` | 改寫成 個資清除訂閱者：訂閱 `buyer.BuyerDeleted`，排 job 清收件匣參數與偏好，未送出的意圖執行時先確認收件帳號未刪除（A8-1） |
| 2 | `hawkr.account` / `account.deleted` | `notification.account.deleted`（`:103`） | `account_deleted.go:107-127` | 只做第一步（chat 紅線化）；**不**停用 device token、不刪偏好、不軟刪其他收件匣列 | 同上 | 改寫成 個資清除訂閱者：訂閱 `identity.AccountDeleted` 清收件匣與偏好（A8-1），同 transaction 刪 token（A8-3） |
| 3 | `hawkr.seller` / `seller.deleted` | `notification.seller.deleted`（`:107`） | `NS/internal/worker/seller_deleted.go:30-102` | 紅線化 chat → 軟刪退休 store 的收件匣 → 停用 `device_tokens`（`DeactivateByUser(store_id,'seller')`） | `seller_deleted.go`、`device_tokens.sql:31-34` | 改寫成 個資清除訂閱者：訂閱 `seller.StoreRetired` 清收件匣參數與偏好（A8-1） |

事件 payload 型別：`pkg/mq/events.go:225-240`（`AccountDeletedEvent{accountId}`、`SellerDeletedEvent{accountId, store_id}`、`BuyerDeletedEvent{accountId, buyerId}`；欄名大小寫風格不一致）。

### 3.3 發布（notification-service 自己）

| # | exchange / routing key | payload | 觸發點 | 消費者 | 可靠度 | 來源 | 去向 |
|---|---|---|---|---|---|---|---|
| 1 | `hawkr.notification` / `notification.inbox.new` | `NotificationInboxNewEvent{userId,userType,item{id,userId,userType,eventType,type,referenceId,title,body,isRead,createdAt,readAt,data},unreadCount}` | `CreateInboxItem` 且 `affected>0` | gateway SSE（`GW/realtime/notification_channel.go:36-38`；queue 為 exclusive、auto-delete、auto-ack） | fire-and-forget；錯誤 `_ =` 丟棄（`usecase/notification_inbox.go` `publishInboxNew`）；publisher 建不起來就永久 no-op（`NS/internal/mq/di.go:23-30`） | `NS/internal/mq/publisher.go:39-47`、`pkg/mq/events.go:166-182` | 改寫成 單體行程內 hub 的串流失效訊號（未讀數與最新收件匣 ID，不經 MQ、不帶 item；D11；A8-2） |
| 2 | `hawkr.notification` / `notification.unread_changed` | `{userId,userType,unreadCount}` | MarkAsRead/MarkAllAsRead/Delete 且 changed>0 | 同上 | 同上 | `publisher.go:49-57`、`pkg/mq/events.go:184-195` | 改寫成 單體行程內 hub 的串流失效訊號（未讀數與最新收件匣 ID，不經 MQ；D11；A8-2） |

### 3.4 消費端對應：73 組 (exchange, routing key) 與其發送端（設計時盤點＋本次交叉）

下表是第 5 節「模板」依 `(exchange, routing_key)` 去重後的發送端對照（73 組 pair，含 `hawkr.notification`/`notification.announcement` 標記列）：

- 發送端依 `backend/docs/plans/2026-09-24-go-modular-monolith/event-pairs.csv`（設計時盤點，非本次實測）：aftersales-service 25、invoice-service 12、seller-service 8（含 `merchant.verification.*` 5 + `seller.deleted/suspended/unsuspended`）、social-service 6、buyer-service 5、order-service 4、shipping-service 4、product-service 3、account-service 2、ai-service 2、coupon-service 1、im-service 1、payment-service 1、review-service 1。
- 現況（本次讀碼）已改由單體 relay 發送的 22 個 route 在第 8.1 表；其餘仍由 Java（aftersales、order、shipping、product、coupon、payment）與 Go 的 invoice-service、ai-service 發送。
- 無發送端的 route：`merchant.verification.*`（5 enabled 模板，`sellerrelay/relay.go:10` 明寫不再轉送）、`coupon.issued`（發布端是死碼，`event-anomalies.md:14`）。
- 送不到的事件（`hawkr.governance` 5 key 等）：見 `event-anomalies.md:7-14`。

## 4. 通道與外部 provider

| # | 通道／provider | 實作 | 設定／環境變數（只列名稱） | 狀態 | 來源 | 去向 |
|---|---|---|---|---|---|---|
| 1 | FCM 可見推播（iOS，APNs 經 FCM） | `SendEachForMulticast`；`Notification{Title,Body,ImageURL}`；APNS `Sound: default`、`FCMOptions.ImageURL` | `FIREBASE_CREDENTIALS_JSON`（空值 → `disabledNotifier`，所有推播記為 `skipped`，不報錯） | 使用中 | `NS/internal/infrastructure/firebase.go:44-110` | 改寫成 FCM adapter 可見通知（APNs alert，FCM `notification` payload；A8-3、A8-6） |
| 2 | FCM 無聲推播（data-only，`silent` 模板） | `SendEachForMulticast`；APNS `apns-priority: 10`、`content-available` | 同上 | 使用中（14 個 silent 模板） | `firebase.go:142-181` | 刪除（App 改用可見通知，刪 data-only 推播路徑；狀態重讀改由推播 payload 的重讀提示 enum 承載，D6；A8-2、A8-6） |
| 3 | FCM topic 廣播 | `SendToTopic` | - | **死碼**（`fcm_topic` 欄位已於 20260822130001 刪除；`rg SendToTopic` 非測試 4 hit，只剩介面＋實作＋disabled stub） | `firebase.go:114-138`、`NS/internal/domain/push_notification.go` | 刪除（死碼，缺陷 #22；A8-7） |
| 4 | FCM 錯誤分類 | 只在 `IsUnregistered`、`IsSenderIDMismatch` 時停用 token（用 `DeactivateTokenByValue`，**依 token 值停用所有使用者列**） | - | 使用中；ThirdPartyAuth 錯誤沒有告警（ADR 已記） | `firebase.go:202-207`、`NS/internal/usecase/send_push.go:134-139`、`device_tokens.sql:15-18` | 保留→FCM adapter（只在 Unregistered、SenderIDMismatch 停用 token，ThirdPartyAuthError 告警，每批最多 500 個 token；A8-3） |
| 5 | Email（Resend，`pkg/mail`） | 設定被載入並做 prod 驗證，但**沒有任何程式讀取 `cfg.Mail.*`**（`rg 'Mail\.(Provider|From|...)'` 排除 config.go 後 0 hit）；vendor 邀請信 consumer 已在 commit `5c0292db2`（2026-11-19）刪除 | `MAIL_PROVIDER`、`MAIL_FROM`、`RESEND_API_KEY`、`MAIL_RECIPIENT_POLICY`、`MAIL_ALLOWED_RECIPIENTS`、`MAIL_REDIRECT_TO` | **死設定**（但 `config.go:145-165` 仍在 prod 強制 `MAIL_PROVIDER=resend` 與 `MAIL_RECIPIENT_POLICY=unrestricted`） | `NS/internal/config/config.go:25-45,111-118,145-165` | 改寫成 Resend adapter 加本機替身，寄件對象只限 `identity.ContactEmail` 已驗證者（D8；A8-3）；舊 `MAIL_*`、`RESEND_API_KEY` 死設定刪除（A8-7） |
| 6 | SMS | 無 | - | 不存在（ADR 決策 9：不做，取貨簡訊 A8 開工時查 7-ELEVEN） | - | 不做（範圍：不做 SMS，通知 ADR 決策 9；取貨簡訊依 D5 刪除） |
| 7 | LINE | 無（LINE 只用於登入身分，不在本服務） | - | 不存在 | - | 不做（範圍：不做 LINE，通知 ADR 決策 9） |
| 8 | Web Push | 無；web 用 SSE（上面第 3.3 節） | - | 不存在 | - | 不做（範圍：不做 Web Push，通知 ADR 決策 9） |
| 9 | 站內收件匣 | `notifications` 表 | - | 使用中 | 第 2 節 | 保留→新模組收件匣（A8-1） |
| 10 | 即時（SSE）到 web | MQ → gateway SSE | `RABBITMQ_URL`（service）；gateway 自己的連線設定 | 使用中 | 第 3.3 節 | 改寫成 Connect server streaming 失效訊號（D11；A8-2） |
| 11 | 媒體 URL 組裝（push 圖片） | `mediaurl.BuildURL(MediaBaseURL, objectKey)` | `MEDIA_BASE_URL`（無預設，缺值啟動失敗） | 使用中（44 個模板帶 `image_key_template`） | `template_handler.go:175-189`、`config.go:106` | 待決：新型別的推播是否仍帶圖片（舊 44 個模板帶 `image_key_template`），範圍、D6 與 A8-1 型別註冊表屬性清單都沒寫 |
| 12 | 其他服務設定 | buyer-service / seller-service gRPC | `BUYER_SERVICE_GRPC_ADDR`、`LEGACY_SELLER_GRPC_ADDR`（兩者預設都是 `127.0.0.1:9941`） | 使用中（僅個資清除流程） | `config.go:108-109` | 刪除（個資清除改為單體事件訂閱，不再跨服務 gRPC 列舉歷史 id；A8-1、A8-7） |
| 13 | 其他 env（service 自己） | - | `ENV`、`DB_HOST`、`DB_PORT`、`DB_POOL_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`、`DB_SSLMODE`、`GRPC_PORT`；otel：`OTEL_EXPORTER_OTLP_ENDPOINT`、`OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`、`TRACING_SAMPLING`、`POD_NAME` | - | `config.go:86-117`、`backend/pkg/database/dsn.go:67-79`、`backend/pkg/otel/*.go` | 刪除（隨 notification-service 刪除；A8-7） |

偏好分類（`NS/internal/domain/notification_preference.go:5-59`）：`AllCategories` 10 類（order_update、refund_update、system、chat、follower、post、coupon、message、shipment_anomaly、restock）；`SellerCategories` 6、`BuyerCategories` 8；賣家 `shipment_anomaly` 強制開（`forcedCategoriesByUserType`）；`categoryMetaByUserType` 的中文標題在 `NS/internal/usecase/notification_preference.go:24-45`。**偏好只擋推播，不擋收件匣**（`send_push.go:63-77`；handler 先寫收件匣 `template_handler.go:196-213`）。

## 5. 通知型別（模板）與自我略過

### 5.1 模板引擎規則（現行行為；`NS/internal/worker/template_handler.go`）

| # | 規則 | 說明 | 來源 | 去向 |
|---|---|---|---|---|
| 1 | 查模板 | `(exchange, routing_key)` → 0..N 個模板（同一 routing key 可對買、賣各一張，例 `chat.message.sent`、`comment.created`） | `template_cache.go:49-57`、`template_handler.go:31` | 改寫成 型別註冊表以事件型別查型別（程式碼，不經 DB 模板；D13、A8-1） |
| 2 | payload 解碼 | `json.Decoder.UseNumber`，保留 64 位 TSID（issue #1316）；解碼失敗回 error（才會被 MQ 重試） | `template_handler.go:398-406,43-50` | 刪除（單體事件是 typed 結構，不再解碼 JSON payload；A8-1） |
| 3 | payload_filter | 每個 key：等值（缺 key＝不通過）或 `{"$ne": v}`（缺 key＝通過）；值以字串比較 | `template_handler.go:432-465`、`domain/notification_template.go` | 改寫成 型別註冊表依事件欄位分流型別（如 `MeasureCreated` 依 `Kind`、解除請求只送 `MeasureRequestDecided`，D3；A8-3） |
| 4 | 收件人 | 讀 `target_user_field`（number/json.Number/string，必須 >0），錯誤就 warn 並略過這則 | `template_handler.go:353-391,134-141` | 改寫成 worker 解析收件人 `(account_id, 受眾 BUYER 或 STORE, store_id)`（A8-1） |
| 5 | 自我略過 | 見 5.3 | `template_handler.go:109-115,145-153` | 改寫成 帳號比對的自我略過（缺陷 #12；A8-1） |
| 6 | 渲染 | `#{field}`、`#{field\|default}`（缺值/nil/空字串用 default，第一個 `\|` 切分）；title 缺欄位不處理，body 缺欄位且有 `body_fallback` 則用 fallback；缺欄位的 `#{x}` 原樣留在輸出 | `template_handler.go:238-296` | 改寫成 模組 `internal/text` 伺服器組字 zh-TW／en，每個型別固定輸出比對測試（D1；A8-1） |
| 7 | data map | `data_fields` 的 FieldSpec（`const` 或 `field`）＋固定加 `type=data_type`、`category`、`url`（`schema_url` 內 `{key}` 以 data map 替換）、`imageUrl`（`image_key_template` 渲染＋`mediaurl.BuildURL`，有殘留佔位符就省略） | `template_handler.go:168-189,298-348`、`domain/notification_template.go` | 改寫成 伺服器給的深連結＋呈現分類 enum＋狀態重讀提示 enum（D6；A8-1、A8-2） |
| 8 | reference_id | `reference_id_template` 渲染（**忽略 missing 清單**，`:193`）；空字串＝無冪等 | `template_handler.go:191-194` | 改寫成 型別必須宣告冪等鍵來源（缺陷 #3、#4 消失；A8-1） |
| 9 | push_mode | `silent`：不寫收件匣、只送 data-only 推播；其他任何值（含 `normal`）＝visible：先寫收件匣（失敗只 log）再推播 | `template_handler.go:155-235` | 改寫成 型別註冊表宣告渠道（收件匣、推播、email；`chat_message` 只推播，D3；silent 模式不再存在；A8-1） |
| 10 | 推播 | `SendPushUseCase.Execute`：冪等檢查 → 偏好（分類）→ 取 active tokens → FCM → 依錯誤停用 token → 寫 `push_notifications` | `NS/internal/usecase/send_push.go:48-164` | 改寫成 意圖 job 展開為每渠道投遞 job，偏好在 worker 套用（D10；A8-1、A8-3） |
| 11 | 收件匣寫入 | `INSERT … ON CONFLICT (user_id,user_type,event_type,reference_id) WHERE reference_id IS NOT NULL DO NOTHING`；新增成功才發 SSE | `queries/notifications.sql:1-8`、`usecase/notification_inbox.go` `CreateInboxItem` | 改寫成 worker 寫收件匣，有聚合鍵時未讀列 upsert，冪等 UNIQUE（D12；A8-1） |

### 5.2 啟用中的模板統計（測：重放 migration）

- 總計 94 列、92 enabled（2 列 disabled 標記：`notification.announcement`、`procurement.rfq_publication_invite.external_created.vendor`）。
- 依 `target_user_type`：buyer 37、seller 55（enabled）。依 `push_mode`（enabled）：visible 77、silent 14、`normal` 1（`payment.rescue.refund_completed`，id 836178646375858196）。
- 依 `category`（enabled，買/賣）：order_update 27/29、refund_update 3/0、system 1/15、chat 1/1、content 1/4、coupon 1/0、follower 0/3、message 2/0、restock 1/0、seller_account 0/2、shipment_anomaly 0/1。**`content`（5 列）與 `seller_account`（2 列）不在 `AllCategories`**（見 7.10）。
- 依 exchange（enabled）：aftersales.events 37、hawkr.invoice 13、hawkr.content 10、hawkr.order 7、hawkr.merchant 5、hawkr.shipping 5、hawkr.product 4、hawkr.im 2、hawkr.marketing 2、hawkr.seller 2、hawkr.buyer 1、hawkr.delegation 1、hawkr.review 1、payment.events 1、procurement.events 1。
- 無 `reference_id_template`（無冪等）：`comment.created`、`comment.created.buyer`（2 列）。
- 14 個 silent 模板（只推 data、不寫收件匣）：`aftersales.appeal.submitted.buyer`、`aftersales.case.applied.buyer`、`aftersales.case.cancelled_by_buyer.buyer`、`aftersales.case.coupon_compensated.seller`、`aftersales.case.refund_approved.seller`、`aftersales.case.rejected.seller`、`aftersales.case.return_approved.seller`、`aftersales.case.return_received.seller`、`aftersales.refund.completed.seller`、`invoice.issued.seller`、`order.cancelled.merchant`、`order.completed.buyer`、`order.completed.merchant`、`shipment.delivered.seller`。
- 賣家模板的 `target_user_field` 現在全是 store_id 空間（`store_id`、`target_store_id`、`storeId`、`postAuthorStoreId`…）；買家是 `buyer_id`／`follower_id`／`receiver_id`／`postAuthorId`／`replyToUserId`。（測）

### 5.3 自我略過（`actorFieldByEventType`，`template_handler.go:109-115`）

只有 5 個 event_type 有登記；其餘不做自我略過。比對條件：`actorID == userID`（兩個都以 int64 解出）；actor 欄位缺失或解析失敗時**靜默不略過**（`:146` 的 `aerr == nil &&`）。

| # | event_type（模板的，不是 routing key） | actor 欄位 | 模板 target_user_field | 兩邊的身分空間（取自 golden `backend/tests/contract/golden/event_payload_keys.json` 的 `identity_spaces`） | 空間一致？ | 去向 |
|---|---|---|---|---|---|---|
| 1 | `comment.created`（賣家） | `commenterId` | `postAuthorStoreId` | 兩邊 `store_id` | 是（但 commenterId 在買家留言時是什麼？見 U7） | 改寫成 `comment_created` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |
| 2 | `comment.created.buyer` | `commenterPrincipalId` | `postAuthorId`（buyer_id） | actor：`{buyer: buyer_id, seller: account_id}` | **否**（留言者是賣家時 account_id 對 buyer_id） | 刪除（`comment.created.buyer` 買家版不做：貼文作者恆為店家，單體無發送端，D3；A8-3） |
| 3 | `comment.replied.seller` | `replierId` | `replyToUserStoreId` | 兩邊 `store_id` | 是 | 改寫成 `comment_replied` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |
| 4 | `comment.replied.buyer` | `replierPrincipalId` | `replyToUserId`（buyer_id） | actor：`{buyer: buyer_id, seller: account_id}` | **否**（同 #2） | 改寫成 `comment_replied` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |
| 5 | `comment.like_changed`（賣家） | `userId` | `commentAuthorStoreId` | actor：`{buyer: buyer_id, seller: store_id}`；target `store_id` | **否**（按讚者是買家時 buyer_id 對 store_id） | 改寫成 `comment_liked` 型別的帳號比對自我略過（缺陷 #12；A8-1、A8-3） |

主計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md:278` 已記：自我略過把買家 ID 拿去比店家 ID，「notification 搬進單體時改以帳號比對（Will 2026-11-17）」；A7-3 的 `socialrelay` 已先以帳號擋下真正的自我通知（`socialrelay/relay.go:14-16`）。

### 5.4 全部 94 列模板（來源：重放 `NS/db/migrations/*.sql` 後的 `notification_templates`；「目前發送端」欄取自 event-pairs.csv 與 `internal/legacy/rabbitmq/testdata/routes.json`）

| # | tpl id | exchange | routing key | event_type | 對象 | target_user_field | category | mode | on | payload_filter | reference_id_template | data.type | 目前發送端 (event-pairs.csv 設計時盤點 / 單體 relay) | 去向 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | 836178646375858192 | aftersales.events | aftersales.appeal.submitted | aftersales.appeal.submitted.buyer | buyer | buyer_id | order_update | silent | Y |  | `case_submitted_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 2 | 836178646375858193 | aftersales.events | aftersales.appeal.submitted | aftersales.appeal.submitted.seller | seller | store_id | order_update | visible | Y |  | `case_submitted_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 3 | 836178646375858176 | aftersales.events | aftersales.case.applied | aftersales.case.applied.buyer | buyer | buyer_id | order_update | silent | Y |  | `case_applied_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 4 | 836178646375858177 | aftersales.events | aftersales.case.applied | aftersales.case.applied.seller | seller | store_id | order_update | visible | Y |  | `case_applied_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 5 | 845336352869318761 | aftersales.events | aftersales.case.auto_completed_confirm_receipt_timeout | aftersales.case.auto_completed_confirm_receipt_timeout.seller | seller | store_id | order_update | visible | Y |  | `case_auto_completed_confirm_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 6 | 851819712504397826 | aftersales.events | aftersales.case.auto_completed_return_pending_timeout | aftersales.case.auto_completed_return_pending_timeout.buyer | buyer | buyer_id | refund_update | visible | Y |  | `case_auto_completed_return_pending_buyer_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 7 | 851819712504397827 | aftersales.events | aftersales.case.auto_completed_return_pending_timeout | aftersales.case.auto_completed_return_pending_timeout.seller | seller | store_id | order_update | visible | Y |  | `case_auto_completed_return_pending_seller_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 8 | 845336352869318760 | aftersales.events | aftersales.case.auto_completed_store_pickup_timeout | aftersales.case.auto_completed_store_pickup_timeout.seller | seller | store_id | order_update | visible | Y |  | `case_auto_completed_store_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 9 | 836166767511339008 | aftersales.events | aftersales.case.auto_escalated | aftersales.case.auto_escalated.buyer | buyer | buyer_id | order_update | visible | Y |  | `auto_escalated_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 10 | 836166767511339009 | aftersales.events | aftersales.case.auto_escalated | aftersales.case.auto_escalated.seller | seller | store_id | order_update | visible | Y |  | `auto_escalated_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 11 | 845348489209577675 | aftersales.events | aftersales.case.auto_rejected | aftersales.case.auto_rejected.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_auto_rejected_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 12 | 836178646375858186 | aftersales.events | aftersales.case.cancelled_by_buyer | aftersales.case.cancelled_by_buyer.buyer | buyer | buyer_id | order_update | silent | Y |  | `case_cancelled_by_buyer_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 13 | 836178646375858187 | aftersales.events | aftersales.case.cancelled_by_buyer | aftersales.case.cancelled_by_buyer.seller | seller | store_id | order_update | visible | Y |  | `case_cancelled_by_buyer_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 14 | 859506920694743152 | aftersales.events | aftersales.case.cancelled_by_confirm_receipt | aftersales.case.cancelled_by_confirm_receipt.seller | seller | store_id | order_update | visible | Y |  | `case_cancelled_by_confirm_receipt_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 15 | 836178646375858182 | aftersales.events | aftersales.case.coupon_compensated | aftersales.case.coupon_compensated.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_coupon_compensated_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 16 | 836178646375858183 | aftersales.events | aftersales.case.coupon_compensated | aftersales.case.coupon_compensated.seller | seller | store_id | order_update | silent | Y |  | `case_coupon_compensated_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 17 | 836178646375858180 | aftersales.events | aftersales.case.refund_approved | aftersales.case.refund_approved.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_refund_approved_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 18 | 836178646375858181 | aftersales.events | aftersales.case.refund_approved | aftersales.case.refund_approved.seller | seller | store_id | order_update | silent | Y |  | `case_refund_approved_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 19 | 836178646375858184 | aftersales.events | aftersales.case.rejected | aftersales.case.rejected.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_rejected_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 20 | 836178646375858185 | aftersales.events | aftersales.case.rejected | aftersales.case.rejected.seller | seller | store_id | order_update | silent | Y |  | `case_rejected_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 21 | 836178646375858178 | aftersales.events | aftersales.case.return_approved | aftersales.case.return_approved.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_return_approved_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 22 | 836178646375858179 | aftersales.events | aftersales.case.return_approved | aftersales.case.return_approved.seller | seller | store_id | order_update | silent | Y |  | `case_return_approved_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 23 | 845336352869318756 | aftersales.events | aftersales.case.return_at_store | aftersales.case.return_at_store.seller | seller | store_id | order_update | visible | Y |  | `case_return_at_store_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 24 | 845336352869318757 | aftersales.events | aftersales.case.return_at_store_24h | aftersales.case.return_at_store_24h.seller | seller | store_id | order_update | visible | Y |  | `case_return_at_store_24h_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 25 | 845336352869318758 | aftersales.events | aftersales.case.return_delivered | aftersales.case.return_delivered.seller | seller | store_id | order_update | visible | Y |  | `case_return_delivered_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 26 | 845336352869318759 | aftersales.events | aftersales.case.return_delivered_24h | aftersales.case.return_delivered_24h.seller | seller | store_id | order_update | visible | Y |  | `case_return_delivered_24h_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 27 | 845348489209577672 | aftersales.events | aftersales.case.return_failed | aftersales.case.return_failed.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_return_failed_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 28 | 845348489209577673 | aftersales.events | aftersales.case.return_failed | aftersales.case.return_failed.seller | seller | store_id | order_update | visible | Y |  | `case_return_failed_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 29 | 859435123710361710 | aftersales.events | aftersales.case.return_in_transit | aftersales.case.return_in_transit.seller | seller | store_id | order_update | visible | Y |  | `case_return_in_transit_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 30 | 851819712504397825 | aftersales.events | aftersales.case.return_pending_24h | aftersales.case.return_pending_24h.buyer | buyer | buyer_id | refund_update | visible | Y |  | `case_return_pending_24h_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 31 | 836178646375858188 | aftersales.events | aftersales.case.return_received | aftersales.case.return_received.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_return_received_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 32 | 836178646375858189 | aftersales.events | aftersales.case.return_received | aftersales.case.return_received.seller | seller | store_id | order_update | silent | Y |  | `case_return_received_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 33 | 836178646375858194 | aftersales.events | aftersales.platform_decision.made | aftersales.platform_decision.made.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_made_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 34 | 836178646375858195 | aftersales.events | aftersales.platform_decision.made | aftersales.platform_decision.made.seller | seller | store_id | order_update | visible | Y |  | `case_made_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 35 | 845348489209577674 | aftersales.events | aftersales.refund.awaiting_buyer_page | aftersales.refund.awaiting_buyer_page.buyer | buyer | buyer_id | order_update | visible | Y |  | `awaiting_buyer_page_#{refund_task_id}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 36 | 836178646375858190 | aftersales.events | aftersales.refund.completed | aftersales.refund.completed.buyer | buyer | buyer_id | order_update | visible | Y |  | `case_completed_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 37 | 836178646375858191 | aftersales.events | aftersales.refund.completed | aftersales.refund.completed.seller | seller | store_id | order_update | silent | Y |  | `case_completed_#{case_no}` | AFTER_SALE_STATUS | aftersales-service | 隨發送端搬遷（D13） |
| 38 | 21 | hawkr.buyer | buyer.favorite.added | product.favorited | seller | store_id | follower | visible | Y |  | `fav_#{seller_account_id}_#{product_id}` | PRODUCT_FAVORITED | buyer-service -> now emitted by monolith relay `buyerrelay` | 不做（D3：收藏不做；A8-3） |
| 39 | 100 | hawkr.content | comment.created | comment.created.buyer | buyer | postAuthorId | message | visible | Y | `{"postAuthorType": "buyer"}` | (none) | POST_NEW_COMMENT | social-service -> now emitted by monolith relay `socialrelay` | 不做（D3：貼文作者恆為店家，單體無買家版發送端；A8-3） |
| 40 | 20 | hawkr.content | comment.created | comment.created | seller | postAuthorStoreId | content | visible | Y | `{"postAuthorType": "seller"}` | (none) | COMMENT_CREATED | social-service -> now emitted by monolith relay `socialrelay` | 改寫成 型別 `comment_created`（SELLER、互動；A8-3） |
| 41 | 851819712504397824 | hawkr.content | comment.like_changed | comment.like_changed | seller | commentAuthorStoreId | content | visible | Y | `{"action": "LIKE", "commentAuthorType": {"$ne": "buyer"}}` | `comment_like_changed_#{commentId}` | COMMENT_LIKED | social-service -> now emitted by monolith relay `socialrelay` | 改寫成 型別 `comment_liked`（BUYER 或 SELLER、聚合鍵 `comment_like:{comment_id}`；D12；A8-3） |
| 42 | 25 | hawkr.content | comment.replied | comment.replied.buyer | buyer | replyToUserId | message | visible | Y | `{"replyToUserType": "buyer"}` | `reply_#{postId}_#{commentId}` | POST_COMMENT_REPLY | social-service -> now emitted by monolith relay `socialrelay` | 改寫成 型別 `comment_replied`（BUYER 受眾；A8-3） |
| 43 | 24 | hawkr.content | comment.replied | comment.replied.seller | seller | replyToUserStoreId | content | visible | Y | `{"replyToUserType": "seller"}` | `reply_#{postId}_#{commentId}` | COMMENT_REPLIED | social-service -> now emitted by monolith relay `socialrelay` | 改寫成 型別 `comment_replied`（SELLER 受眾；A8-3） |
| 44 | 13 | hawkr.content | follow.added | follow.added | seller | store_id | follower | visible | Y |  | `follow_#{merchant_id}_#{follower_id}` | FOLLOWER_ADDED | buyer-service -> now emitted by monolith relay `buyerrelay` | 改寫成 型別 `store_followed`（聚合鍵 `follow:{store_id}`；D12；A8-3） |
| 45 | 19 | hawkr.content | post.created | post.created | buyer | follower_id | content | visible | Y |  | `post_#{post_id}_#{follower_id}` | POST_FOLLOWED_AUTHOR | social-service -> now emitted by monolith relay `socialrelay` | 改寫成 型別 `post_published`（BUYER 追蹤者；A8-3） |
| 46 | 912663901278044361 | hawkr.content | post.hidden_by_platform | post.hidden_by_platform.seller | seller | store_id | system | visible | Y |  | `content_hidden_#{post_id}` | POST_HIDDEN_BY_PLATFORM | social-service -> now emitted by monolith relay `socialrelay` | 改寫成 型別 `store_content_taken_down`（SELLER、交易必要、`ACCOUNT`；A8-3） |
| 47 | 23 | hawkr.content | post.liked | post.liked | seller | store_id | content | visible | Y |  | `post_liked_#{seller_id}_#{post_id}` | POST_LIKED | social-service -> now emitted by monolith relay `socialrelay` | 改寫成 型別 `post_liked`（聚合鍵 `post_like:{post_id}`；D12；A8-3） |
| 48 | 22 | hawkr.content | product.viewed | product.viewed | seller | store_id | follower | visible | Y |  | `viewed_#{seller_id}_#{product_id}` | PRODUCT_VIEWED | buyer-service -> now emitted by monolith relay `buyerrelay` | 不做（D3：`product.viewed` 不做；A8-3） |
| 49 | 860417358747926642 | hawkr.delegation | delegation.operation.completed | delegation.operation.completed.seller | seller | target_store_id | system | visible | Y |  | `#{audit_log_id}` | DELEGATION_OPERATION_COMPLETED | account-service -> now emitted by monolith relay `delegationservice` | 改寫成 型別 `delegation_operation_completed`（SELLER、交易必要、`ACCOUNT`；A8-3） |
| 50 | 846737671714766848 | hawkr.im | chat.message.sent | chat.message.sent | buyer | receiver_id | chat | visible | Y | `{"receiver_type": "buyer"}` | `#{message_id}` | chat_message | im-service -> now emitted by monolith relay `imrelay` | 改寫成 型別 `chat_message`（BUYER 受眾，只推播不寫收件匣；D3；A8-3） |
| 51 | 846737671714766849 | hawkr.im | chat.message.sent | chat.message.sent | seller | store_id | chat | visible | Y | `{"receiver_type": "seller"}` | `#{message_id}` | chat_message | im-service -> now emitted by monolith relay `imrelay` | 改寫成 型別 `chat_message`（SELLER 受眾，只推播不寫收件匣；D3；A8-3） |
| 52 | 860417358747926652 | hawkr.invoice | invoice.allowance.issued | invoice.allowance.issued.buyer | buyer | buyer_id | order_update | visible | Y |  | `#{notification_ref_id}` | INVOICE_ALLOWANCE_ISSUED | invoice-service | 隨發送端搬遷（D13） |
| 53 | 860417358747926656 | hawkr.invoice | invoice.complaint.created | invoice.complaint.created.seller | seller | store_id | system | visible | Y |  | `#{notification_ref_id}` | INVOICE_COMPLAINT_CREATED | invoice-service | 隨發送端搬遷（D13） |
| 54 | 860417358747926658 | hawkr.invoice | invoice.complaint.overdue | invoice.complaint.overdue.seller | seller | store_id | system | visible | Y |  | `#{notification_ref_id}` | INVOICE_COMPLAINT_OVERDUE | invoice-service | 隨發送端搬遷（D13） |
| 55 | 860417358747926657 | hawkr.invoice | invoice.complaint.responded | invoice.complaint.responded.buyer | buyer | buyer_id | order_update | visible | Y |  | `#{notification_ref_id}` | INVOICE_COMPLAINT_RESPONDED | invoice-service | 隨發送端搬遷（D13） |
| 56 | 860417358747926650 | hawkr.invoice | invoice.issue_failed | invoice.issue_failed.seller | seller | store_id | system | visible | Y |  | `#{notification_ref_id}` | INVOICE_ISSUE_FAILED | invoice-service | 隨發送端搬遷（D13） |
| 57 | 860417358747926648 | hawkr.invoice | invoice.issued | invoice.issued.buyer | buyer | buyer_id | order_update | visible | Y |  | `#{notification_ref_id}` | INVOICE_ISSUED | invoice-service | 隨發送端搬遷（D13） |
| 58 | 860417358747926649 | hawkr.invoice | invoice.issued | invoice.issued.seller | seller | store_id | order_update | silent | Y |  | `#{notification_ref_id}` | INVOICE_ISSUED | invoice-service | 隨發送端搬遷（D13） |
| 59 | 860417358747926655 | hawkr.invoice | invoice.operation.review_required | invoice.operation.review_required.seller | seller | store_id | system | visible | Y |  | `#{notification_ref_id}` | INVOICE_OPERATION_REVIEW_REQUIRED | invoice-service | 隨發送端搬遷（D13） |
| 60 | 860417358747926654 | hawkr.invoice | invoice.receipt.issued | invoice.receipt.issued.buyer | buyer | buyer_id | order_update | visible | Y |  | `#{notification_ref_id}` | INVOICE_RECEIPT_ISSUED | invoice-service | 隨發送端搬遷（D13） |
| 61 | 860417358747926653 | hawkr.invoice | invoice.reissued | invoice.reissued.buyer | buyer | buyer_id | order_update | visible | Y |  | `#{notification_ref_id}` | INVOICE_REISSUED | invoice-service | 隨發送端搬遷（D13） |
| 62 | 860417358747926660 | hawkr.invoice | invoice.resolved_externally | invoice.resolved_externally.buyer | buyer | buyer_id | order_update | visible | Y |  | `#{notification_ref_id}` | INVOICE_RESOLVED_EXTERNALLY | invoice-service | 隨發送端搬遷（D13） |
| 63 | 860417358747926659 | hawkr.invoice | invoice.seller_tax_profile.status_changed | invoice.seller_tax_profile.status_changed.seller | seller | store_id | system | visible | Y |  | `#{notification_ref_id}` | INVOICE_SELLER_TAX_PROFILE_STATUS_CHANGED | invoice-service | 隨發送端搬遷（D13） |
| 64 | 860417358747926651 | hawkr.invoice | invoice.voided | invoice.voided.buyer | buyer | buyer_id | order_update | visible | Y |  | `#{notification_ref_id}` | INVOICE_VOIDED | invoice-service | 隨發送端搬遷（D13） |
| 65 | 103 | hawkr.marketing | coupon.expiring | coupon.expiring | buyer | buyer_id | coupon | visible | Y |  | `expiring_#{coupon_instance_id}_#{date_key}` | COUPON_EXPIRING | coupon-service | 隨發送端搬遷（D13） |
| 66 | 14 | hawkr.marketing | coupon.issued | coupon.issued | buyer | buyer_id | system | visible | Y |  | `coupon_#{buyer_id}_#{coupon_id}` | COUPON_RECEIVED | (none found) | 隨發送端搬遷（D13） |
| 67 | 27 | hawkr.merchant | merchant.verification.approved | merchant.verification.approved | seller | store_id | system | visible | Y |  | `merchant_verification_#{merchant_id}_#{event_id}` | MERCHANT.VERIFICATION.APPROVED | seller-service | 改寫成 型別 `kyc_approved`（SELLER、交易必要、`ACCOUNT`；A8-3） |
| 68 | 860417358747926641 | hawkr.merchant | merchant.verification.forwarded | merchant.verification.forwarded | seller | store_id | system | visible | Y |  | `merchant_verification_forwarded_#{merchant_id}_#{event_id}` | MERCHANT.VERIFICATION.FORWARDED | seller-service | 隨發送端搬遷（D13） |
| 69 | 29 | hawkr.merchant | merchant.verification.pending_info | merchant.verification.pending_info | seller | store_id | system | visible | Y |  | `merchant_verification_#{merchant_id}_#{event_id}` | MERCHANT.VERIFICATION.PENDING_INFO | seller-service | 隨發送端搬遷（D13） |
| 70 | 28 | hawkr.merchant | merchant.verification.rejected | merchant.verification.rejected | seller | store_id | system | visible | Y |  | `merchant_verification_#{merchant_id}_#{event_id}` | MERCHANT.VERIFICATION.REJECTED | seller-service | 改寫成 型別 `kyc_rejected`（SELLER、交易必要、`ACCOUNT`，帶退回理由；A8-3） |
| 71 | 860417358747926646 | hawkr.merchant | merchant.verification.under_review | merchant.verification.under_review | seller | store_id | system | visible | Y |  | `merchant_verification_under_review_#{merchant_id}_#{event_id}` | MERCHANT.VERIFICATION.UNDER_REVIEW | seller-service | 隨發送端搬遷（D13） |
| 72 | 860417358747926647 | hawkr.notification | notification.announcement | notification.announcement | seller | (empty) | system | visible | N |  | `announcement_#{announcementId}_#{merchantId}` | SYSTEM_ANNOUNCEMENT | (none found) | 改寫成 後台公告資料（非事件型別，收件匣與推播分開 job；A8-2、A8-3） |
| 73 | 7 | hawkr.order | order.cancelled | order.cancelled.buyer | buyer | buyer_id | order_update | visible | Y | `{"cancel_reason": {"$ne": "9104"}}` | `#{order_no}` | ORDER_CANCELLED | order-service | 隨發送端搬遷（D13） |
| 74 | 912663901340959123 | hawkr.order | order.cancelled | order.cancelled.seller_suspended | buyer | buyer_id | order_update | visible | Y | `{"cancel_reason": "9104"}` | `#{order_no}` | ORDER_CANCELLED | order-service | 隨發送端搬遷（D13） |
| 75 | 8 | hawkr.order | order.cancelled | order.cancelled.merchant | seller | store_id | order_update | silent | Y |  | `#{order_no}` | ORDER_CANCELLED | order-service | 隨發送端搬遷（D13） |
| 76 | 5 | hawkr.order | order.completed | order.completed.buyer | buyer | buyer_id | order_update | silent | Y |  | `#{order_no}` | ORDER_COMPLETED | order-service | 隨發送端搬遷（D13） |
| 77 | 6 | hawkr.order | order.completed | order.completed.merchant | seller | store_id | order_update | silent | Y |  | `#{order_no}` | ORDER_COMPLETED | order-service | 隨發送端搬遷（D13） |
| 78 | 1 | hawkr.order | order.paid | order.paid | buyer | buyer_id | order_update | visible | Y |  | `#{order_no}` | ORDER_PAID | order-service | 隨發送端搬遷（D13） |
| 79 | 17 | hawkr.order | payout.completed | payout.completed | seller | store_id | order_update | visible | Y |  | `payout_#{order_no}` | PAYOUT_COMPLETED | order-service | 隨發送端搬遷（D13） |
| 80 | 860417358747926643 | hawkr.product | product.certification.approved | product.certification.approved.seller | seller | store_id | system | visible | Y |  | `product_cert_#{application_id}` | PRODUCT_CERTIFICATION_STATUS | product-service | 隨發送端搬遷（D13） |
| 81 | 860417358747926644 | hawkr.product | product.certification.rejected | product.certification.rejected.seller | seller | store_id | system | visible | Y |  | `product_cert_#{application_id}` | PRODUCT_CERTIFICATION_STATUS | product-service | 隨發送端搬遷（D13） |
| 82 | 860417358747926645 | hawkr.product | product.certification.revoked | product.certification.revoked.seller | seller | store_id | system | visible | Y |  | `product_cert_#{application_id}` | PRODUCT_CERTIFICATION_STATUS | product-service | 隨發送端搬遷（D13） |
| 83 | 854394970889519104 | hawkr.product | product.restock.target | product.restock.target | buyer | buyer_id | restock | visible | Y |  | `restock_#{subscription_id}` | RESTOCK_AVAILABLE | buyer-service -> now emitted by monolith relay `buyerrelay` | 改寫成 型別 `product_restocked`（BUYER、行銷、`PROMOTION`；D4；A8-3） |
| 84 | 30 | hawkr.review | review.created | review.created | seller | store_id | order_update | visible | Y |  | `review_#{order_no}_#{store_id}` | REVIEW_CREATED | review-service -> now emitted by monolith relay `reviewrelay` | 改寫成 型別 `review_created`（SELLER、互動、`REVIEW`；A8-3） |
| 85 | 912663901340959121 | hawkr.seller | seller.suspended | seller.suspended.notice | seller | store_id | seller_account | visible | Y |  | `#{suspensionId}` | SELLER_SUSPENDED | seller-service -> now emitted by monolith relay `sellerrelay` | 改寫成 型別 `store_suspended`（SELLER、交易必要、`ACCOUNT`；A8-3） |
| 86 | 912663901340959122 | hawkr.seller | seller.unsuspended | seller.unsuspended.notice | seller | store_id | seller_account | visible | Y |  | `#{suspensionId}` | SELLER_UNSUSPENDED | seller-service -> now emitted by monolith relay `sellerrelay` | 改寫成 型別 `store_unsuspended`（SELLER、交易必要、`ACCOUNT`；A8-3） |
| 87 | 850227930553384960 | hawkr.shipping | shipment.anomaly.detected | shipment.anomaly.detected.seller | seller | store_id | shipment_anomaly | visible | Y |  | `#{order_no}|#{anomaly_type}` | SHIPMENT_ANOMALY | shipping-service | 隨發送端搬遷（D13） |
| 88 | 2 | hawkr.shipping | shipment.created | shipment.created | seller | store_id | order_update | visible | Y |  | `#{order_no}` | ORDER_CREATED | shipping-service | 隨發送端搬遷（D13） |
| 89 | 15 | hawkr.shipping | shipment.delivered | shipment.delivered | buyer | buyer_id | order_update | visible | Y |  | `#{order_no}` | ORDER_DELIVERED | shipping-service | 隨發送端搬遷（D13） |
| 90 | 833733364656635904 | hawkr.shipping | shipment.delivered | shipment.delivered.seller | seller | store_id | order_update | silent | Y |  | `#{order_no}` | ORDER_DELIVERED | shipping-service | 隨發送端搬遷（D13） |
| 91 | 3 | hawkr.shipping | shipment.status.changed | shipment.status.changed | buyer | buyer_id | order_update | visible | Y | `{"status": "31"}` | `#{order_no}` | ORDER_SHIPPED | shipping-service | 隨發送端搬遷（D13） |
| 92 | 836178646375858196 | payment.events | payment.rescue.refund_completed | payment.rescue.refund_completed | buyer | buyer_id | refund_update | normal | Y |  | `rescue_refund_#{trade_no}` | ORDER_REFUNDED | payment-service | 隨發送端搬遷（D13） |
| 93 | 860417358747926661 | procurement.events | procurement.rfq_publication_invite.created | procurement.rfq_publication_invite.created.seller | seller | storeId | order_update | visible | Y |  | `#{inviteId}` | PROCUREMENT_RFQ_PUBLICATION_INVITE | ai-service | 改寫成 型別 `rfq_invited`（SELLER、互動、`PROCUREMENT`；D3；A8-3） |
| 94 | 860417358747926662 | procurement.events | procurement.rfq_publication_invite.external_created | procurement.rfq_publication_invite.external_created.vendor | vendor | (empty) | order_update | visible | N |  | (none) | PROCUREMENT_RFQ_PUBLICATION_INVITE_EXTERNAL | ai-service | 隨發送端搬遷（D13） |

## 6. 背景工作、重試、outbox、排程

| # | 工作 | 機制 | 參數 | 失敗處理 | 來源 | 去向 |
|---|---|---|---|---|---|---|
| 1 | MQ 消費（18 個 subscription） | `mq.NewResilientConsumer`（`pkg/mq`），帶 OTel metrics；啟動時先同步 `LoadFromDB` 再建訂閱 | `MaxRetries: 3`；queue 由 infra/mq topology 預先建立（passive declare），每 queue 有 `{queue}.retry` exchange、`.dlq` | handler 回 error → republish 到 `{queue}.retry`、`x-mq-retry-count`+1、原訊息 ack；超過 3 次進 DLQ（`pkg/mq/failure.go`、`resilient_republish_test.go`）。**模板 handler 只在 payload 解碼失敗時回 error**（見 7.1），所以推播／收件匣失敗不會走這條重試；靜態 handler（個資清除）會回 error 重試 | `NS/internal/worker/di.go:26-58`、`consumer.go:93-104`、`infra/mq/topology/topology.yaml:175-283` | 改寫成 單體事件同步訂閱者只排意圖 job，River 重試（D0b、D10；缺陷 #1；A8-1） |
| 2 | 模板快取刷新 | `outbox.Scheduler`（`pkg/outbox/scheduler.go`）固定間隔，有 panic guard；不重建訂閱，只換 in-memory map | 5 分鐘，無 initial delay | 失敗 log，保留舊快取 | `worker/di.go:60-98` | 刪除（模板退場，沒有快取，缺陷 #25；A8-7） |
| 3 | 清理 job | 同上 scheduler | 每 6 小時，initial delay 1 分鐘，每批 1000 筆循環到清完 | 單批失敗 log 後該輪結束 | `NS/internal/worker/cleanup.go:15-124` | 改寫成 保留期清除 job（A8-1）；token 清除見 3c（A8-3） |
| 3a | | 刪 `push_notifications` status in (failed, skipped) 且 >30 天 | | | `cleanup.go:54-62` | 待決：投遞紀錄（含永久失敗、跳過）的保留期，計畫 D2 只定收件匣 180 天 |
| 3b | | 刪 `notifications` 已軟刪且 `deleted_at` >90 天 | | | `cleanup.go:67-75` | 改寫成 收件匣 180 天硬刪，不再軟刪（D2；A8-1） |
| 3c | | 停用 `device_tokens` is_active 且 `updated_at` >90 天（「殭屍 token」） | | | `cleanup.go:80-98` | 改寫成 每日 job 刪 60 天未更新的 token（`last_registered_at`；D7；A8-3） |
| 4 | 重送／補送 | **無**（沒有 outbox、沒有 re-drive、沒有手動補列；計畫 `:300` 已要求 A8 設計） | - | - | 主計畫 `:300` | 改寫成 依投遞紀錄重排永久失敗的手段與 runbook，加未觸達查詢（A8-3） |
| 5 | 排程發送／聚合／節流 | **無**（提醒類事件由發送端排程：coupon.expiring、aftersales `*_24h` 在各自服務；賣家瀏覽通知節流在 `backend/internal/modules/buyer/CARRYOVER.md:65` 標為 A8） | - | - | rg | 改寫成 聚合鍵未讀合併（D12；A8-1）；時間窗摘要不做（範圍）；提醒類事件的排程隨發送端搬遷（D13） |
| 6 | 冪等 | 推播：`ExistsPushProcessed`（不分 status）；收件匣：partial unique index；兩者都以 `reference_id` 為鍵（空＝無冪等） | - | - | `send_push.go:48-61`、`schema.sql:34,92` | 改寫成 意圖冪等鍵＝事件來源身分，投遞冪等鍵＝（意圖、收件人、渠道）UNIQUE（A8-1、A8-3） |
| 7 | 廣播 | 同步迴圈，每批 100 個 token（offset 分頁）、每個使用者經 `SendPushUseCase` 逐一送 | batch 100 | 單一使用者失敗只 log | `server_broadcast.go:36-157` | 改寫成 公告是資料，收件匣與推播分開 job（缺陷 #5–#7；A8-2、A8-3） |

## 7. 程式碼可見的既有缺陷（編號；證據為檔案:行）

等級：(讀) 讀碼推得、(測) 實際重放 migration 查得、(猜) 未驗證。已有測試或型別守住的標「已被保護」。

| # | 缺陷 | 證據 | 等級 | 去向 |
|---|---|---|---|---|
| 1 | 模板 handler 吞掉所有送出與寫入失敗並回 nil（ACK）：收件匣寫入失敗只 log 後繼續（`:206-212`）、推播失敗只 log（`:229-235`）、整個 handler 解碼成功後一律 `return nil`（`:66`），所以 `MaxRetries: 3` 與 DLQ 對這些失敗不生效；DB 暫時故障＝通知永久遺失。ADR 脈絡也列（「推播失敗只記 log 後 ACK」） | `NS/internal/worker/template_handler.go:53-66,198-212,218-235`、`consumer.go:101` | (讀) | 修：意圖 job＋投遞紀錄 UNIQUE＋River 重試；型別必須宣告冪等鍵來源（A8-1、A8-3） |
| 2 | 推播冪等是「先查、再送、後寫」非原子：`ExistsProcessed`（`send_push.go:50`）→ FCM 送出 → `pushRepo.Create`（`:163`）。兩個併發投遞可同時通過檢查而雙推；FCM 已送出但 `Create` 失敗時（回 err、handler 只 log）重投遞會再推。唯一索引只涵蓋 `status='sent'`（`schema.sql:92`），與檢查用的「任何 status」不一致 | `send_push.go:48-61,163`、`queries/push_notifications.sql:8-18`、`schema.sql:92` | (讀)；發生頻率 (猜) | 修：意圖 job＋投遞紀錄 UNIQUE＋River 重試；型別必須宣告冪等鍵來源（A8-1、A8-3） |
| 3 | 2 個 enabled 模板沒有 `reference_id_template`（`comment.created`、`comment.created.buyer`）→ 收件匣唯一索引不適用（`reference_id IS NULL`）、推播冪等閘跳過（`send_push.go:49`）→ 任何重複投遞都重複通知 | (測) `select … where enabled and reference_id_template is null or ''` = 2 列；`schema.sql:34`、`send_push.go:49` | (測) | 修：意圖 job＋投遞紀錄 UNIQUE＋River 重試；型別必須宣告冪等鍵來源（A8-1、A8-3） |
| 4 | `reference_id` 渲染時缺欄位會留下字面 `#{field}`：`referenceID, _ = RenderTemplate(...)` 丟棄 missing 清單（`:193`）。結果該使用者之後同模板所有通知共用同一個 reference_id → 收件匣 `ON CONFLICT DO NOTHING` 與推播冪等閘會靜默丟棄後續通知。**部分被保護**：`template_contract_test.go`（integration，需 CI）用 golden `emitted_keys` 逐列檢查每個模板讀的 key 都有發送端發出 | `template_handler.go:191-194,238-296`、`NS/internal/worker/template_contract_test.go:1-12` | (讀)；契約測試未在本機跑 | 修：意圖 job＋投遞紀錄 UNIQUE＋River 重試；型別必須宣告冪等鍵來源（A8-1、A8-3） |
| 5 | 廣播用 `OFFSET` 分頁走 `is_active=true` 的 token，而迴圈內 `SendPushUseCase` 會停用死 token（`DeactivateByToken`）：被停用的列離開結果集，後面的列前移，下一頁 offset 會跳過使用者 | `server_broadcast.go:77-108`、`send_push.go:134-139`、`queries/device_tokens.sql:41-46` | (讀) | 修：公告是資料，收件匣與推播分開 job（A8-2、A8-3） |
| 6 | 廣播是同步長迴圈，gateway client timeout 30 秒（`notification_inbox_client.go:46-47`）；逾時後 request ctx 被取消，迴圈仍用該 ctx 呼叫 repo／FCM（`server_broadcast.go:100`）→ 後半收件人可能全部失敗、呼叫端收不到結果。使用者數成長後必發生 | `GW/grpc/notification_inbox_client.go:46-47`、`server_broadcast.go:83-108` | (讀)；實際人數／耗時 (猜，未量測) | 修：公告是資料，收件匣與推播分開 job（A8-2、A8-3） |
| 7 | 廣播的 `recipients++` 不看 `deliverAnnouncement` 是否成功（兩個寫入都吞錯誤）→ `recipientCount` 與實際送達無關；`TargetUserType` 不驗證（未知值＝0 人、Code 0）；只送給「有 active device token 的人」（網頁與沒裝 App 的人收不到，ADR 已記） | `server_broadcast.go:40-47,95-102,124-157` | (讀) | 修：公告是資料，收件匣與推播分開 job（A8-2、A8-3） |
| 8 | gateway 丟掉 response `Code`（service 以 `Code!=0` 回業務錯誤、gRPC error=nil）：`MarkAsRead`（`notification_inbox.go:212`）、`UpdateNotificationPreference`（`notification_preference.go:99,205`）、`AdminUpdateTemplateEnabled`（`:204`）、`AdminTestSendTemplate`（`:302`）。後果：對強制分類（賣家 `shipment_anomaly`）關閉偏好，service 拒絕（`usecase/notification_preference.go:99-101`、`server.go:109-113`），gateway 仍回 200（gateway 自己的註解 `notification_preference.go:90-91` 以為會回 400）；app 看到 `code==0` 以為成功（`notification_repository.dart:176-186`）；模板 id 無效／不存在也回 200 | 見左 | (讀) | 消失：Connect 錯誤；分類與受眾只在註冊表（A8-1、A8-2） |
| 9 | gateway 與 service 的分類白名單不一致：gateway 買家 `validCategories`（`notification_preference.go:196-199`）沒有 `restock`，但 service `BuyerCategories` 有（`domain/notification_preference.go:40-49`）→ 買家無法經 REST 切換補貨通知（400）；賣家白名單（`:87-91`）含 `post` 但 `SellerCategories` 沒有（`domain:31-38`），`categoryMetaByUserType` 有 seller/post 的死文案（`usecase/notification_preference.go:31`）；swagger 註解寫「7 個」類別（`:28,:115`）與實際 6／8 不符 | 見左 | (讀) | 消失：Connect 錯誤；分類與受眾只在註冊表（A8-1、A8-2） |
| 10 | 模板使用 `AllCategories` 以外的分類：`content`（5 個 enabled 模板）、`seller_account`（2 個）。這些通知無法在偏好畫面列出或關閉（`UpdatePreference` 先 `IsValidCategory`，`server.go:100`；`GetPreferences` 只列 `CategoriesForUserType`），`SendPushUseCase` 查偏好時永遠查不到列＝一律推播 | (測) 分類統計；`domain/notification_preference.go:18-29`、`usecase/notification_preference.go:71-90` | (測) | 消失：Connect 錯誤；分類與受眾只在註冊表（A8-1、A8-2） |
| 11 | `UpdatePreference` 不驗證分類是否屬於該 `user_type`，也不驗證 `user_type`：買家可寫入 `follower`、賣家可寫入 `coupon`；未知 user_type 被 `CategoriesForUserType`／`ForcedCategoriesFor` 當成 seller 處理 | `server.go:99-117`、`usecase/notification_preference.go:98-103`、`domain/notification_preference.go:61-75` | (讀) | 消失：Connect 錯誤；分類與受眾只在註冊表（A8-1、A8-2） |
| 12 | 自我略過比對不同身分空間（5.3 表 #2、#4、#5），且 actor 欄位缺失時靜默不略過（`:146`）；只有 5 個 event_type 有登記，且沒有任何測試證明「登記集合＝該略過的集合」之外的覆蓋（主計畫 `:278` 已列） | `template_handler.go:109-115,145-153`、golden `identity_spaces` | (讀) | 修：帳號比對（A8-1） |
| 13 | 裝置 token 綁定：唯一鍵 `(user_id,user_type,token)`，`UpsertDeviceToken` 不會停用同一 token 在其他使用者下的列 → 同一支手機換人登入（未呼叫 DELETE 登出時，例如被動登出）會收到前一人的通知（ADR 已記）；`buyer.deleted`／`account.deleted` 不停用 token（只有 `seller.deleted` 做，`seller_deleted.go:88-100`）；`DeactivateTokenByValue` 依 token 值停用**所有**使用者的列（`device_tokens.sql:15-18`，用在 FCM 死 token 時合理） | `schema.sql:11`、`queries/device_tokens.sql:1-18`、`worker/account_deleted.go:44-127` | (讀) | 修：token 主鍵與綁定、個資清除（A8-1、A8-3） |
| 14 | 帳號刪除只清 chat 收件匣：`account.deleted` handler 只呼叫 `RedactChatNotificationsByAccount`（`account_deleted.go:107-127`），其他事件的渲染文字（買家暱稱、訂單、留言內容等存在 `title`/`body`/`data`）不被紅線化，也不軟刪該帳號的其他收件匣列、不刪 `notification_preferences`；只有 `buyer.deleted`／`seller.deleted` 會軟刪對應 stint 的整個收件匣（再 90 天後實刪） | `account_deleted.go:107-127`、`queries/notifications.sql:61-106`、`cleanup.go:67-75` | (讀) | 修：token 主鍵與綁定、個資清除（A8-1、A8-3） |
| 15 | 只收 iOS：service `server.go:121` 與 gateway `notification.go:68` 都拒絕非 `ios`；schema 註解寫 `android | ios`（`schema.sql:6`）；兩支 App 都硬送 `ios`（`buyer-app/.../device_registrar.dart:32`、`seller-app/.../device_registrar.dart:65`）；FCM payload 沒有任何 Android 設定（`firebase.go:74-89`） | 見左 | (讀) | 不是缺陷：Android 已移除；刪 `platform` 欄位與兩處驗證（A8-2、A8-7） |
| 16 | 保留期：`push_notifications` 的 `sent`／`partial` 永不清（`cleanup.go:56-61` 只清 failed/skipped）；`notifications` 沒被軟刪的永不到期（ADR 脈絡已記） | `cleanup.go:54-75` | (讀) | 修：180 天（A8-1） |
| 17 | 列表 `total` 不受 `category` 篩選影響（`CountByUser` 不帶分類，`usecase/notification_inbox.go:60`、`queries/notifications.sql:26-30`）；`cursorId` 解析失敗被忽略＝0＝回第一頁（`server_inbox.go:19`）；gateway `pageSize` 解析失敗被忽略（`notification_inbox.go:113`） | 見左 | (讀) | 修（A8-2） |
| 18 | `MarkAsRead` 的 SQL 沒有 `deleted_at IS NULL`（`notifications.sql:38-44`），且 `changed==0`（不存在、別人的 id、已讀）都回成功（`usecase/notification_inbox.go:73-82`），沒有 404 | 見左 | (讀) | 修（A8-2） |
| 19 | SSE topic 只以 `userId` 為鍵（`TopicNotificationUser(id)`；`ResolveTopic` 只讀 `userId`，`user_type` 被忽略），而 buyer_id 與 store_id 在 grandfather 帳號上數值相同（`queries/notifications.sql:84-86` 的註解：「buyer_id == store_id == account_id」）→ 同帳號的買家串流可能收到賣家收件匣的 `notification.new`／未讀數，反之亦然 | `GW/realtime/notification_channel.go:48-74`（`TopicNotificationUser`、`ResolveTopic`）、`GW/handler/sse_stream.go:29-45` | (讀)；實際是否已有跨身分事件 (猜) | 消失：`(account_id, 受眾)`；串流只是訊號，資料由列表保證（A8-2） |
| 20 | SSE 發布是 best-effort：`_ =` 丟棄錯誤；啟動時 RabbitMQ 連不上則 `Publisher` 為 nil 且**永不重建**（`mq/di.go:23-30`）直到重啟；gateway 消費端 auto-ack、exclusive 非持久 queue（`GW/mq/consumer.go:~155-180`） | `NS/internal/mq/di.go:23-47`、`NS/internal/usecase/notification_inbox.go` `publishInboxNew`/`publishUnreadChanged` | (讀) | 消失：`(account_id, 受眾)`；串流只是訊號，資料由列表保證（A8-2） |
| 21 | `TestSendTemplate`／`PreviewTemplate` 與正式路徑不同：測試送出不帶 `Silent`、不帶 `ImageURL`、不套 `payload_filter`、不寫收件匣，且寫入 `push_notifications`（真的 `event_type`、`reference_id=test_<ms>`，`sent` 永不清）；gateway 的 `targetAccountId` 欄位被原樣塞進 `TargetUserId`（`admin_notification_template.go:299-306`），但 service 把它當 buyer_id／store_id 解讀（`server_admin.go:154`），要填的是代理 id 而非帳號 id | `server_admin.go:97-211`、`template_handler.go:117-236`、`admin_notification_template.go:285-327` | (讀) | 刪除（A8-7） |
| 22 | 死碼與死設定：`system_notification_logs` 表＋repository＋domain 型別沒接 DI、沒有 usecase／RPC（`rg SystemNotificationLog NS/internal/repository/postgres/di.go` 0 hit）；`FCM SendToTopic`（`fcm_topic` 欄位已刪）；`CheckCategoryEnabled`（`usecase/notification_preference.go:105-116`，僅定義無呼叫）；`MailConfig` 與 6 個 `MAIL_*`／`RESEND_API_KEY` 無任何讀取者卻仍在 prod 強制 `MAIL_PROVIDER=resend` 與 `MAIL_RECIPIENT_POLICY=unrestricted`（`config.go:145-165`）；`identity_migrated` 欄位對賣家恆 true（CHECK）、對買家無意義；兩列 disabled 標記模板；`20260911100000_vendor_portal_invite_external_golden_marker.sql:1-12` 的註解指向已不存在的 `internal/worker/vendor_invite.go` | 見左；`rg` 計數見附錄 A | (讀)(測) | 刪除（A8-7） |
| 23 | 孤兒模板：5 個 enabled `merchant.verification.*` 模板沒有任何發送端（`sellerrelay/relay.go:10` 寫「不再轉成舊的 merchant.verification.*」；`routes.json` 無 `hawkr.merchant`）；`coupon.issued` 模板 enabled 但發布端是死碼（`event-anomalies.md:14`）；`procurement.rfq_publication_invite.external_created` 的 consumer 已在 `5c0292db2` 刪除，事件仍由 ai-service 發出（`services/ai-service/internal/procurement/publicationoutbox/worker.go`） | 見左 | (讀)(測) | 刪除（A8-7） |
| 24 | `push_mode` 沒有 CHECK，且有一列 enabled 值是 `normal`（`payment.rescue.refund_completed`，buyer，id 836178646375858196）；程式只比對 `== "silent"`，所以 `normal` 被當 visible | (測)；`template_handler.go:155` | (測) | 消失：Connect 錯誤；分類與受眾只在註冊表，silent 模式不再存在（A8-1、A8-2） |
| 25 | 模板快取是行程內：`ReloadTemplates` 只刷新收到 RPC 的那個實例（`server_admin.go:17-28`），其他實例最多等 5 分鐘（`worker/di.go:60`）；訂閱集合開機時固定，新增 exchange 要改程式＋topology（`consumer.go:11-57,73-85`）。副本數未知 | `template_cache.go:69-83` | (讀)；副本數 (猜) | 刪除（A8-7） |
| 26 | FCM `SendEachForMulticast` 單次上限 500 個 token（FCM API 限制，來自文件記憶，未在本機驗證）；`GetActiveTokensByUser` 無上限（`device_tokens.sql:36-39`）、`Send` 一次傳全部（`firebase.go:84-91`）；90 天殭屍回收是唯一上限 | `firebase.go:69-110` | (猜) | 修：分批（A8-3） |
| 27 | 所有推播文字只有 zh-TW（模板是中文、`imrelay/relay.go:40` 註明舊 notification 只認 zh-TW）；收件匣存渲染快照＋gateway 以 `strings.ToUpper(ReplaceAll(event_type,".","_"))` 衍生 `type`（`notification_inbox.go:32-34`），三個 client 依賴這些字串。ADR 已決定改 typed 參數＋雙語（非缺陷，列為搬遷時要保住的 client 契約） | `GW/handler/notification_inbox.go:32-95` | (讀) | 修（A8-1） |

## 8. 單體側橋接（目前餵給 notification-service 的東西）

### 8.1 單體現在送出的 22 個 (exchange, routing key)（`backend/internal/legacy/rabbitmq/testdata/routes.json`，由 `backend/internal/legacy/rabbitmq/publish_routes_test.go` 掃描產生）與 notification 端的消費

| # | exchange | routing key | monolith publisher pkg (routes.json) | notification consumer | 去向 |
|---|---|---|---|---|---|
| 1 | hawkr.account | account.deleted | internal/legacy/accountservice | static subscription queue notification.account.deleted | 改寫成 個資清除訂閱者訂閱 `identity.AccountDeleted`（A8-1），同 transaction 刪 token（A8-3）；不做通知型別（D3） |
| 2 | hawkr.buyer | buyer.deleted | internal/legacy/buyerrelay | static subscription queue notification.buyer.deleted | 改寫成 個資清除訂閱者訂閱 `buyer.BuyerDeleted`（A8-1）；不做通知型別（D3） |
| 3 | hawkr.buyer | buyer.favorite.added | internal/legacy/buyerrelay | enabled template(s): product.favorited/seller | 不做（D3：收藏不做；A8-3 刪 `buyerrelay` 對應 wire） |
| 4 | hawkr.buyer | buyer.favorite.removed | internal/legacy/buyerrelay | deliberately not notified (notifyAllowlist, relay_routes_test.go:61-66) | 不做（D3：取消收藏不需要告知賣家；A8-3 刪 `buyerrelay` 對應 wire） |
| 5 | hawkr.content | comment.created | internal/legacy/socialrelay | enabled template(s): comment.created.buyer/buyer, comment.created/seller | 改寫成 型別 `comment_created`（SELLER、互動、`SOCIAL`；A8-3） |
| 6 | hawkr.content | comment.like_changed | internal/legacy/socialrelay | enabled template(s): comment.like_changed/seller | 改寫成 型別 `comment_liked`（受眾依 `Author` 種類、互動、`SOCIAL`、聚合鍵 `comment_like:{comment_id}`；D12；A8-3） |
| 7 | hawkr.content | comment.replied | internal/legacy/socialrelay | enabled template(s): comment.replied.buyer/buyer, comment.replied.seller/seller | 改寫成 型別 `comment_replied`（受眾依 `ReplyToActor` 種類、互動、`SOCIAL`；A8-3） |
| 8 | hawkr.content | follow.added | internal/legacy/buyerrelay | enabled template(s): follow.added/seller | 改寫成 型別 `store_followed`（SELLER、互動、`SOCIAL`、聚合鍵 `follow:{store_id}`；D12；A8-3） |
| 9 | hawkr.content | post.created | internal/legacy/socialrelay | enabled template(s): post.created/buyer | 改寫成 型別 `post_published`（BUYER 追蹤者、互動、`SOCIAL`；A8-3） |
| 10 | hawkr.content | post.hidden_by_platform | internal/legacy/socialrelay | enabled template(s): post.hidden_by_platform.seller/seller | 改寫成 型別 `store_content_taken_down`（SELLER、交易必要、`ACCOUNT`；A8-3） |
| 11 | hawkr.content | post.liked | internal/legacy/socialrelay | enabled template(s): post.liked/seller | 改寫成 型別 `post_liked`（SELLER、互動、`SOCIAL`、聚合鍵 `post_like:{post_id}`；D12；A8-3） |
| 12 | hawkr.content | product.viewed | internal/legacy/buyerrelay | enabled template(s): product.viewed/seller | 不做（D3：`product.viewed` 不做；A8-3 刪 `buyerrelay` 的 `product.viewed` wire） |
| 13 | hawkr.delegation | delegation.operation.completed | internal/legacy/delegationservice | enabled template(s): delegation.operation.completed.seller/seller | 改寫成 型別 `delegation_operation_completed`（SELLER、交易必要、`ACCOUNT`，代行者不能讀 D9；A8-3） |
| 14 | hawkr.im | chat.message.sent | internal/legacy/imrelay | enabled template(s): chat.message.sent/buyer, chat.message.sent/seller | 改寫成 型別 `chat_message`（對方受眾、互動、`MESSAGE`，只推播不寫收件匣；D3；A8-3） |
| 15 | hawkr.product | product.restock.target | internal/legacy/buyerrelay | enabled template(s): product.restock.target/buyer | 改寫成 型別 `product_restocked`（BUYER、行銷、`PROMOTION`；D4；A8-3） |
| 16 | hawkr.review | review.created | internal/legacy/reviewrelay | enabled template(s): review.created/seller | 改寫成 型別 `review_created`（SELLER、互動、`REVIEW`；A8-3） |
| 17 | hawkr.review | review.hidden | internal/legacy/reviewrelay | deliberately not notified (notifyAllowlist, relay_routes_test.go:61-66) | 不做（D3：事件只表示聚合要重送、沒有人可通知；A8-3 保留 `reviewrelay` 評分聚合那一半） |
| 18 | hawkr.seller | seller.deleted | internal/legacy/sellerrelay | static subscription queue notification.seller.deleted | 改寫成 個資清除訂閱者訂閱 `seller.StoreRetired`（A8-1）；不做通知型別（D3） |
| 19 | hawkr.seller | seller.exposure.changed | internal/legacy/sellerrelay | deliberately not notified (notifyAllowlist, relay_routes_test.go:61-66) | 不做（D3：曝光是內部可見度，原因已由措施與停權通知涵蓋） |
| 20 | hawkr.seller | seller.profile.changed | internal/legacy/sellerrelay | deliberately not notified (notifyAllowlist, relay_routes_test.go:61-66) | 不做（D3：賣家自己的寫入，自我通知無意義） |
| 21 | hawkr.seller | seller.suspended | internal/legacy/sellerrelay | enabled template(s): seller.suspended.notice/seller | 改寫成 型別 `store_suspended`（SELLER、交易必要、`ACCOUNT`；`MeasureCreated(SUSPENSION)` 不另發，D3；A8-3） |
| 22 | hawkr.seller | seller.unsuspended | internal/legacy/sellerrelay | enabled template(s): seller.unsuspended.notice/seller | 改寫成 型別 `store_unsuspended`（SELLER、交易必要、`ACCOUNT`；A8-3） |

### 8.2 relay／橋接套件（隨 A8 刪除或轉成模組訂閱）

| # | 套件 | 角色 | 訂閱的模組事件 → 舊 wire | 註明的退場條件 | 註冊點 | 來源 | 去向 |
|---|---|---|---|---|---|---|---|
| 1 | `backend/internal/legacy/socialrelay` | social／seller 事件 → `hawkr.content`（source `social-service`）；relay 端以帳號比對自我通知、查封鎖雙方、內容不可見不送；`post.created` 對每個追蹤者一個 job | `PostPublished`→`post.created`、`CommentCreated`→`comment.created`、`CommentReplied`→`comment.replied`、`PostLiked`→`post.liked`、`CommentLiked`→`comment.like_changed`（只送 LIKE）、`seller.MeasureCreated`→`post.hidden_by_platform` | 「隨 notification 搬進單體而刪除」（`relay.go:19`）；主計畫 `:276-277,295` | `cmd/hawkr/social_events.go` | `socialrelay/relay.go:1-19,35-45` | 刪除（事件改由 notification 模組直接訂閱；A8-3） |
| 2 | `backend/internal/legacy/reviewrelay` | review 事件 → `hawkr.review`（Java product 評分聚合＋notification 賣家通知）；wire `notify_seller_account_id` 在 relay 時解析（店家不存在＝0＝不推） | `ReviewCreated`→`review.created`、`ReviewAggregateChanged`→`review.hidden` | 「隨 notification、catalog、ordering 搬進單體而刪除」（`relay.go:17`） | `cmd/hawkr/review.go` | `reviewrelay/relay.go:1-17,155,239-277` | 刪除 notification 一半（`review_created` 改由模組訂閱；A8-3）；給 Java product 評分聚合的那一半保留（A8-3 不能動：A6 延後項） |
| 3 | `backend/internal/legacy/imrelay` | im 事件 → `hawkr.im` `chat.message.sent`（離線推播）；預覽句取共用文案目錄 zh-TW | `im.MessageSent` | 「notification 搬進單體後刪除」（`relay.go:11`） | `cmd/hawkr/im.go` | `imrelay/relay.go:1-40,118-167` | 刪除（`chat_message` 改由模組訂閱；A8-3） |
| 4 | `backend/internal/legacy/buyerrelay` | buyer 事件 → `hawkr.buyer`／`hawkr.content`／`hawkr.product` | `buyer.deleted`、`buyer.favorite.added/removed`、`follow.added`、`product.viewed`、`product.restock.target` | 隨下游消費者搬遷退場 | `cmd/hawkr/buyer_events.go` | `buyerrelay/relay.go:1-25`；routes.json | 刪除追蹤、`product.viewed`、`product.restock.target` 的 wire（後者只有 notification 消費，改為 `product_restocked` 訂閱；A8-3）；保留 `buyer.deleted`、`favorite.added/removed`（Java `coupon-service` `BuyerDeletedConsumer`、`product-service` `FavoriteCountConsumer` 仍消費，隨該服務搬遷） |
| 5 | `backend/internal/legacy/sellerrelay` | seller 事件 → `hawkr.seller`；**不轉送**平台審核（KYC）事件與治理事件（「A8 的 notification 訂閱」）、`StoreOpened` 等 | `seller.deleted/suspended/unsuspended/profile.changed/exposure.changed` | 「隨 seller 的消費者…notification…搬進單體而退場」（`relay.go:14`） | `cmd/hawkr/seller_events.go`、測試 `cmd/hawkr/seller_events_test.go:13` | `sellerrelay/relay.go:1-14` | 刪除 notification 的部分（A8-3）；舊通知服務 A8-7 一併刪除 |
| 6 | `backend/internal/legacy/delegationservice`（`events.go`） | 代行完成事件 → `hawkr.delegation` `delegation.operation.completed`（每個 ID 都是字串）；`notify_seller` 旗標由代行 gateway／`httpapi/social/delegated.go` 帶入 | `delegation` 模組事件 | notification 仍訂閱該 key | `cmd/hawkr/main.go:40` | `delegationservice/events.go:15,52`、`GW/handler/delegation/orchestrator.go:81-85`、`backend/api/.../social.proto`（`notify_seller` 欄位 `api/hawkr/social/v1alpha1/social.proto:868`） | 刪除（`delegationservice/events.go` 的通知 wire；`delegation_operation_completed` 改由模組訂閱；A8-3） |
| 7 | `backend/internal/legacy/accountservice`（`events.go`） | 發 `hawkr.account` `account.deleted`（im 與 notification 還在訂閱） | 帳號刪除 | - | `cmd/hawkr/main.go:33` | `accountservice/events.go:14` | 保留→不動（不在 A8-3、A8-7 刪除清單；notification 改訂 `identity.AccountDeleted`，A8-1） |
| 8 | `backend/internal/legacy/bizcalendarrelay` | **與 notification 無關**（行事曆推給 invoice-service）；列出避免誤判 | - | A5 | - | `bizcalendarrelay/relay.go:1-12` | 保留→不動（與 notification 無關，規模表「`bizcalendarrelay` 無關」） |

### 8.3 已預留給 A8 的單體事件／消費者（尚未有訂閱者）

| # | 事件／工作 | 位置 | 備註 | 去向 |
|---|---|---|---|---|
| 1 | `accountsuspension` 的 `Suspended`、`Unsuspended`（帶理由類別；同 transaction 發） | `backend/internal/modules/accountsuspension/events.go:10`、`CARRYOVER.md:14` | 「通知（A8）消費它」；主計畫 `:276` 帳號停權與解除通知 | 改寫成 型別 `account_suspended`、`account_unsuspended`（BUYER、交易必要、`ACCOUNT`；A8-1 端到端範例） |
| 2 | `seller` 的 `ReportReceived`、`ReportResolved` | `backend/internal/modules/seller/events.go:33`、`.../model/report_case.go:744` | 檢舉人一律是登入買家（站內通知）；確認信地址不得留在 job 參數（主計畫 `:304`） | 改寫成 型別 `report_received`（確認信，意圖參數只放 report 參照，worker 向 seller 讀地址）、`report_resolved`（BUYER、交易必要、`ACCOUNT`；A8-3） |
| 3 | `seller` 的 `MeasureRequestSubmitted`（提醒 admin） | `.../seller/internal/model/measure.go:1515` | | 不做（D3：`MeasureRequestSubmitted` 不進通知，管理員不是收件人） |
| 4 | `seller` 的措施／治理事件（警告、停權、申訴結果）；`CaseInquiryCreated` 已刪（無消費者） | `.../seller/internal/measure/measure.go:5`、`.../governance/governance.go:4`、`.../governance/inquiry.go:134`、`seller/CARRYOVER.md:51,88-89` | 舊 `seller.governance.*` 5 個 key 送往沒建立的 `hawkr.governance` | 改寫成 型別 `store_measure_issued`、`store_measure_revised`、`store_measure_ended`、`store_measure_revoked`、`store_measure_request_decided`、`store_case_inquiry`（`CaseInquiryCreated` 由 seller 新增並同 tx 發布）；解除請求只送 Decided、`MeasureRequestClosed` 不做（D3；A8-3） |
| 5 | seller 的 KYC 審核事件＋payment 的 `PayoutAccountChanged` | 主計畫 `:305` | 賣家目前收不到核准／退回／收款啟用通知；含 Stripe `minimum_deadline` 提醒 | 改寫成 型別 `kyc_approved`、`kyc_rejected`、`payout_account_activated`、`payout_account_deadline`（payment 先加首次啟用時間與補件期限欄位；D3；A8-3）；`KYCSubmitted`、`KYCCancelled` 不做（D3） |
| 6 | procurement 的 `RfqPublished` | `backend/internal/modules/procurement/internal/model/rfq.go:79` | 「通知（A9-3c 的 relay 與 A8）消費它」；目前 `procurement.rfq_publication_invite.created` 仍由 ai-service outbox 發（`services/ai-service/internal/procurement/publicationoutbox/worker.go`） | 改寫成 型別 `rfq_invited`（SELLER、互動、`PROCUREMENT`；D3；A8-3） |
| 7 | social：作者被隱藏內容／收到警告的推播（Q6）、依案件決定通知每位檢舉人（A7-D20）、封鎖雙方不送規則（A7-3 的同一測試） | 主計畫 `:276` | | 改寫成 moderation 新增事件與型別 `content_hidden`（只給買家作者，D3）、`author_warned`、`moderation_case_closed`（帶每位檢舉人帳號），在處置的同 transaction 發布（A8-3） |
| 8 | review／im 事件改由 notification 直接消費並刪 relay | 主計畫 `:263` | | 改寫成 notification 直接消費 review、im 事件並刪 relay 的 notification 一半（A8-3） |
| 9 | 發票寄送的買家 email 來源（identity 的 `accounts.email`）、identity 接 Clerk `user.updated` webhook | 主計畫 `:299,:301`；`backend/internal/modules/identity/AGENTS.md:36`（device token 擁有者暫為 notification-service） | | 改寫成 email adapter 只寄 `identity.ContactEmail` 的 `ContactEmailVerified`（D8；A8-3）；發票寄送隨發送端搬遷（D13）；Clerk 已移除，不做 |
| 10 | `notifications/stream` 改 server streaming（續傳游標取代 `Last-Event-ID`，保留每帳號串流上限） | 主計畫 `:302`（D7） | gateway 現有上限：`sse.ErrUserCapReached`、`ErrPodCapReached`（`GW/handler/sse_stream.go`） | 改寫成 Connect server streaming 失效訊號；不做續傳游標（D11；A8-2） |
| 11 | 賣家瀏覽／追蹤通知節流／聚合 | `backend/internal/modules/buyer/CARRYOVER.md:65` | | 改寫成 追蹤用聚合鍵 `follow:{store_id}`（D12；A8-3）；瀏覽通知不做（D3） |
| 12 | 食安事件通知近 6 個月購買者 | 主計畫 `:303`（前提 B8） | | 保留→延後至 B8（範圍「延後」：食安事件通知近 6 個月購買者，觸發 B8） |

### 8.4 消費端守門測試（搬遷時要一起處理）

| # | 檔案 | 守什麼 | 備註 | 去向 |
|---|---|---|---|---|
| 1 | `NS/internal/worker/relay_routes_test.go:15-154` | 單體每個送出的 route 要被 topology 綁到、要有 enabled template 或靜態訂閱或列入 `notifyAllowlist`（4 項：`seller.profile.changed`、`seller.exposure.changed`、`buyer.favorite.removed`、`review.hidden`）；讀 `backend/internal/legacy/rabbitmq/testdata/routes.json`；註明「最後一個舊 Publisher 使用者刪除時刪掉」 | | 刪除（只為舊通知存在的守門測試；A8-7） |
| 2 | `NS/internal/worker/template_routes_integration_test.go`、`template_contract_test.go`（integration，需 DB 容器） | route→template、模板讀的 key↔ golden（`backend/tests/contract/golden/event_payload_keys.json`，92 筆；`known_pending.json`） | 依全域規則不在本機跑 | 刪除（只為舊通知存在的守門測試；A8-7） |
| 3 | `backend/internal/legacy/{socialrelay,reviewrelay,imrelay,sellerrelay}/{wire,golden}_test.go` | relay 的 wire 逐欄位釘住、模板讀的 key 對照 | relay 刪除時一併退場 | 刪除（relay 刪除時一併退場；A8-3） |
| 4 | `backend/internal/httpapi/suspension_test.go:227-270`、`entry_test.go:406-437` | 停權放行 `DELETE device-token`、SSE 經 proxy | 路徑改變時要跟著改 | 改寫成 新 Connect 路徑的停權放行（device token 刪除）與串流限流豁免、BFF 代理測試（A8-2） |

## 不確定清單

- U1：舊 notification-service 現在是否還在跑？`sellerrelay/relay.go:10`、`cmd/hawkr/seller_events_test.go:13` 說「舊通知服務沒有部署」，但 `docker-compose-local.yml:1139-1176` 仍有它、其餘 relay 的 doc comment 仍說它「還在消費」。本次未查任何部署資料（無網路、不讀 secrets）。若實際沒部署，則 `merchant.verification.*` 之外也沒有人在送推播。
- U2：副本數與 gRPC 認證。`ReloadTemplates` 的多副本問題（7.25）取決於副本數；`NS` 的 gRPC server 走 `pkg/server` 的共用 interceptor 鏈，本次未讀它是否驗證呼叫者，所以不知道管理 RPC（`BroadcastAnnouncement`、`ReloadTemplates`）是否只靠網路隔離。
- U3：`mq.NewResilientConsumer` 的 prefetch／並行度與 handler 是否序列處理，本次未讀 `pkg/mq/resilient*.go` 全文；7.2 的併發雙推是否真能發生取決於此。
- U4：事件 payload 的實際線上形狀以 golden 與 `wire_test` 為準；本次只讀 golden 的 `identity_spaces` 與模板欄位，沒有逐一比對 92 個 golden 項與 94 個模板（那是 `template_contract_test.go` 的工作，需 DB 容器，依規則不在本機跑，狀態：待 CI）。
- U5：`event-pairs.csv` 的發送端是 2026-09 設計時的盤點，部分 Java 發送端可能已改；第 5.4 表的「目前發送端」只對 22 個單體 route 以 `routes.json` 校正，其餘（Java aftersales/order/shipping/product/coupon/payment、Go invoice-service、ai-service）未逐一重驗。`coupon.issued` 無發送端的結論來自 `event-anomalies.md:14`，本次只確認模板 enabled，未重查 Java 呼叫端。
- U6：website 是否真的不註冊 device token（rg `device-token` 在 `website` 0 hit，故推論只有兩支 App 註冊）；website 的通知收件匣在 SSE 之外是否另有輪詢（只讀 `useNotificationStream.ts:170`、`repo.ts`，未讀 `useUnreadCount.ts` 的重試邏輯）。
- U7：7.12／5.3 的「身分空間是否一致」只依 golden 的 `identity_spaces` 宣告，未檢查 social 發送端實際填什麼（例如買家留言時 `commenterId` 是否為空或 buyer_id）；`socialrelay` 已改成帳號比對，實際線上是否仍可觸發取決於 relay 先擋的比例，未量測。
- U8：website／app 是否對 `type`（由 `event_type` 衍生的大寫字串）、`refType`、`data.*` 做分支（7.27）未逐一盤點；搬遷時 inbox 契約的最小保留集合需要另外盤。`GW/handler/notification_inbox.go:32-95` 是衍生規則的唯一來源（讀）。
- U9：SSE（7.19）跨身分串流是否真的發生，取決於 gateway 解析 `streamUserID` 與 service 的 `userId`（inbox 身分）是否永遠同空間；SSE 的 `AuthorizeSubscribe` 只比對 topic 字串，沒有 user_type。未做動態驗證（backend-e2e-verify 需本機棧）。
- U10：Seller workspace 的即時通道（`hawkr.seller_workspace`、`GW/realtime/seller_workspace_channel.go`、`website/lib/hooks/useSellerWorkspaceRealtime.ts`）與通知 SSE 共用 `/app/notifications/stream` 路徑或 hub（`website/lib/sse/useNotificationStream.ts:44`），不屬於 notification-service，但搬遷 `notifications/stream` 時會碰到；本次未盤它的消費者。
- U11：`backend/services/java/shared/java-proto/.../notification.proto` 是副本（與 `backend/api/proto/notification/notification.proto` 同名），本次沒有做 diff；Java 端 0 處使用，但 `java-proto` 是 shared module。
- U12：`GetNotificationPreferences`／Update 在 gateway 以 buyer、seller 各兩支 handler 實作；seller 路由掛 `CapNotification`，buyer 沒有 capability 層；管理員代行（delegation）是否能呼叫這些 `/app` 通知路由未查（`seller.proto:307` 註解寫「通知收件箱、偏好與裝置」屬白名單）。
- U13：收件匣 `notifications` 與 `device_tokens` 目前實際資料量／歷史 `account_id` 空間殘留列（`20260830100003…marker.sql` 註解提到 dev 有 23 列）本次無法查（不碰任何資料庫實例）；遷移資料策略（搬或不搬）需要 planner 與 Will 決定。

## 附錄 A：本次執行的 rg／檢查指令與命中數

`cd /Users/will/second-brain/.projects/personal/hawkr`（唯讀；除本檔與暫存檔外沒有寫入 repo；`git status` 與開始時相同）。

| 指令 | 命中行數 |
|---|---|
| `rg -n '^\s*rpc ' backend/api/proto/notification/notification.proto` | 16 |
| `rg -l 'go-pkg/proto/notification' backend --glob '*.go' --glob '!**/*.pb.go' --glob '!services/notification-service/**'` | 24 |
| `rg -l 'go-pkg/proto/notification' backend/gateway --glob '*.go' --glob '!**/*_test.go'` | 8 |
| `rg -n 'h\.Notification\.' backend/gateway/internal/handler --glob '!**/*_test.go'` | 18 |
| `rg -n 'registerNotificationRoutes\|registerAdminNotificationTemplateRoutes' backend/gateway/internal/router` | 4 |
| `rg -n -i 'notification' backend/api/openapi/openapi.yaml` | 62 |
| `rg -n '/api/v1/(app\|admin)/[a-z/{}_-]*notification' backend/api/openapi/openapi.yaml` | 17 |
| `rg -l 'NotificationServiceGrpc\|NotificationServiceBlockingStub' backend/services/java --glob '*.java' --glob '!**/target/**'` | 0 |
| `rg -n 'app/notifications' website admin buyer-app/lib seller-app/lib packages --glob '!**/node_modules/**' --glob '!**/*test*' --glob '!**/tests/**' --glob '!**/*.md' --glob '!**/*.json'` | 35 |
| `rg -n 'admin/notification' admin/src website --glob '!**/node_modules/**' --glob '!**/*test*' --glob '!**/tests/**' --glob '!**/*.md' --glob '!**/*.json'` | 10 |
| `rg -n 'notification-preferences' website/lib website/features buyer-app/lib seller-app/lib --glob '!**/*test*'` | 13 |
| `rg -l 'vendor_invite\|withVendorInvite' backend --glob '*.go'` | 0 |
| `rg -n 'SystemNotificationLogRepository\|system_notification_logs' backend/services/notification-service/internal --glob '!**/gen/**' --glob '!**/*_test.go'` | 11 |
| `rg -n 'CheckCategoryEnabled' backend/services/notification-service --glob '!**/*_test.go'` | 2 |
| `rg -n 'SendToTopic' backend/services/notification-service --glob '!**/*_test.go'` | 4 |
| `rg -n 'Mail\.(Provider\|From\|ResendAPIKey\|RecipientPolicy\|AllowedRecipients\|RedirectTo)' backend/services/notification-service --glob '!**/*_test.go' --glob '!**/config/config.go'` | 0 |
| `rg -n 'actorFieldByEventType' backend/services/notification-service --glob '!**/*_test.go'` | 9 |
| `rg -n 'platform.*android\|Platform(Android\|IOS)' backend/services/notification-service/internal --glob '!**/*_test.go'` | 2 |
| `rg -n "notification" backend/gateway/internal/router/routes*.go`（路由登記） | 見第 1 節（20 條 REST） |
| `find backend/services/notification-service -type f \| wc -l` | 215（其中 migrations 107 含 atlas.sum） |
| 重放 `NS/db/migrations/*.sql`（臨時本機 Postgres 18，unix socket，無網路，已刪）後 `select count(*) from notification_templates` | 94（92 enabled） |
| 重放後 `select category,target_user_type,count(*) … where enabled group by 1,2` | 見 5.2（含 `content`、`seller_account`） |
| 重放後 `select … where enabled and (reference_id_template is null or reference_id_template='')` | 2 |
| 重放後 `select … where push_mode not in ('visible','silent')` | 1（`normal`） |
| `python3` 讀 `backend/internal/legacy/rabbitmq/testdata/routes.json` | 22 筆 |
| `rg -n "A8" internal cmd --glob '*.go' --glob '!**/*_test.go'`（cwd=backend；含 1 筆與本題無關的 codemeta） | 11（見 8.3） |
| `rg -n -i "notif" backend/docs/plans/2026-09-24-go-modular-monolith/{event-pairs,events,rpc-calls,rpc-pairs,rpc-servers,db-tables,external-systems,idrefs,replicas}.csv`（計數） | 16 / 100 / 23 / 3 / 20 / 3 / 2 / 6 / 1 |
| `rg -n "app/notifications" website admin buyer-app/lib seller-app/lib packages …`（排除 test/md/json，各 client） | website 16、admin 0、buyer-app 12、seller-app 7、packages 0 |
| `rg -n "notification-preferences" …`（各 client） | website 7、admin 0、buyer-app 3、seller-app 3 |
| `rg -n "admin/notification" …`（各 client） | website 1（註解）、admin 9 |
| `rg -n "_dio\.put" buyer-app/.../data/push seller-app/.../data/push`（有沒有人呼叫 PUT device-token） | 0 |
| `rg -n "resp.Code != 0" backend/gateway/internal/handler/notification_inbox.go` | 4（list、unread、read-all、delete；`MarkAsRead` 沒有） |
