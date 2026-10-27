-- Seller notification write-side identity-space fix, batch 1 of N (issue #2707).
--
-- 13 號設計書 §12.1 的賣家收件人契約已從 account_id 改為當前 store_id（見
-- notification-service 賣家身份鍵補齊 store_id 對齊 ADR，2026-08-30；讀端已在同一批
-- 工作中改為 gateway resolveInboxUserID 對 user_type='seller' 解析 storeid.Resolver.
-- CurrentStoreID，見 gateway 該次改動）。這一批處理的是 target_user_type='seller' 的
-- template 裡，publisher payload **已經**帶著未經解析、當下有效的 store_id 欄位、可以
-- 直接改 target_user_field 的 9 支——不需要動任何服務的程式碼，純資料 migration。
--
-- 逐支對過 publisher 原始碼（不是只看 golden／欄位名，§12.1 判斷步驟第 2 步）；
-- 沒有 store_id 欄位可用的其餘 seller 側 template（aftersales 全系列、chat.message.sent、
-- comment.*、follow.added、merchant.verification.*、payout.completed、post.*、
-- product.favorited、product.viewed、shipment.*）維持原樣，登記在同一 PR 的
-- tests/contract/golden/known_pending_identity.json——它們的 publisher 目前只發
-- 已解析成 account_id 的欄位，補一個未解析的 store_id 欄位是各自服務的程式碼改動，
-- 超出本 migration 範圍。
--
-- 本 PR 同時更新 tests/contract/golden/event_payload_keys.json：9 支的新目標欄位原本
-- 就在各自 golden 條目的 emitted_keys 內（seller.suspended.notice／
-- seller.unsuspended.notice 的 store_id 例外——golden 遺漏了這個真實存在的欄位，本次
-- 一併補上，見 pkg/mq/events.go SellerSuspendedEvent／SellerUnsuspendedEvent），但
-- identity_spaces 都還沒有宣告過 store_id 這個空間，本次一併補上 "<field>": "store_id"。
-- template_contract_test.go 的 expectedIdentitySpace["seller"] 目前仍是 "account_id"
-- （這支 map 由本 issue 另一個 task 反轉），所以這 9 支現在會讓契約測試對
-- assertKeyIdentitySpace 那一行紅——這是預期內、且必要的紅：它證明 want/got 真的在
-- 比對正確的兩個值，等 expectedIdentitySpace 反轉後就會轉綠。
--
-- ══════════════════════════════════════════════════════════════════════
-- Part A：notify_seller_account_id → store_id（3 支，payload 兩欄皆在）
-- ══════════════════════════════════════════════════════════════════════
--
-- order.cancelled.merchant / order.completed.merchant：
--   services/java/order-service/.../mq/dto/OrderCancelledPayload.java（storeId 欄位，
--   `orders.store_id` 原始值，2026-08-05 為拆掉「outbox 階段=store_id / wire 階段=
--   account_id」雙義而新增，與 notifySellerAccountId 並存，見該檔 class doc）、
--   OrderCompletedPayload.java（同構，class doc 明寫 storeId 是「店家的 store_id
--   surrogate（原始值，非登入 account_id）」，crm-service 的統計 RPC 也以此欄位為鍵）。
-- review.created：
--   pkg/mq/events.go ReviewCreatedEvent.StoreID `json:"store_id"`（review-service 產生，
--   與 NotifySellerAccountID 並存；20260802120000 migration 已用同一個 store_id 欄位
--   重鍵 reference_id_template，本次是同一份資料的第二個消費者）。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'notify_seller_account_id'
  AND event_type IN (
    'order.cancelled.merchant',
    'order.completed.merchant',
    'review.created'
  );

-- ══════════════════════════════════════════════════════════════════════
-- Part B：seller_account_id → store_id（3 支，商品實名認證裁決）
-- ══════════════════════════════════════════════════════════════════════
--
-- product.certification.approved.seller / rejected.seller / revoked.seller：
--   services/java/product-service/.../mq/dto/ProductCertificationDecidedPayload.java
--   的 storeId 與 sellerAccountId 兩個 component 並存（storeId＝「賣家 store 代理鍵」，
--   sellerAccountId＝「賣家登入 principal，fail-closed 解析而來」）。
--   ⚠️ 該檔 class doc 目前仍寫著舊契約下的告誡（「拿 storeId 頂替在測試環境全綠，只有
--   re-key 過的店家收不到且無告警」）——那是 account_id 契約時期的正確提醒，本次
--   ADR 把賣家收件人契約整個換成 store_id 之後已經反過來：現在該告誡對調成
--   「拿 sellerAccountId 才會在 re-key 過的店家收不到」。Javadoc 文字本身不影響
--   wire 或本 migration 的正確性，留給該服務的下一次改動一併更正，不在本次範圍內。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'seller_account_id'
  AND event_type IN (
    'product.certification.approved.seller',
    'product.certification.rejected.seller',
    'product.certification.revoked.seller'
  );

-- ══════════════════════════════════════════════════════════════════════
-- Part C：accountId → store_id（2 支，賣家停權／解除停權）
-- ══════════════════════════════════════════════════════════════════════
--
-- seller.suspended.notice / seller.unsuspended.notice：
--   pkg/mq/events.go SellerSuspendedEvent.StoreID `json:"store_id,string"` 與
--   SellerUnsuspendedEvent.StoreID（同構）跟 AccountID 並存；發布端
--   services/seller-service/internal/worker/seller_deleted_outbox_worker.go
--   publishSellerLifecycleRow 的 case Suspended/Unsuspended 分支直接把
--   SellerSuspensionOutboxPayload.StoreID（停權當下寫入 outbox 的 store_id）帶進兩個
--   事件，未經任何解析——停權操作本身就是對某個當下生效中的 store 執行，這個值就是
--   當時有效的 store_id。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'accountId'
  AND event_type IN (
    'seller.suspended.notice',
    'seller.unsuspended.notice'
  );

-- ══════════════════════════════════════════════════════════════════════
-- Part D：target_seller_account_id → target_store_id（1 支，平台代行操作完成通知）
-- ══════════════════════════════════════════════════════════════════════
--
-- delegation.operation.completed.seller：
--   services/account-service/internal/delegation/usecase/audit.go 的 outboxPayload
--   同時寫入 "target_store_id"（l.TargetStoreID）與 "target_seller_account_id"
--   （l.TargetSellerAccountID）兩個 key。20260815130000 migration 當時選
--   target_seller_account_id，理由完全成立於彼時的帳契約（seller=account_id）；
--   ADR 反轉契約後同一組並存欄位的正解對調成 target_store_id——這支 template
--   沒有任何 actor 比對邏輯（template_handler.go actorFieldByEventType 未登記
--   delegation.operation.completed.seller），不受 self-notification skip 影響。
UPDATE "notification_templates"
SET target_user_field = 'target_store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'target_seller_account_id'
  AND event_type = 'delegation.operation.completed.seller';
