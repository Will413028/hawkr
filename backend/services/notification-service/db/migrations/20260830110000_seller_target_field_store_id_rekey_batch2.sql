-- Seller notification write-side identity-space fix, batch 2 of N (issue #2707).
--
-- 延續 20260830100000_seller_target_field_store_id_rekey_batch1.sql（那批處理了
-- 9 支 publisher payload 本來就帶著未解析 store_id 欄位的 template）。issue #2707
-- followup（PR #2918/#2921/#2925/#2928/#2933/#2935/#2937，2026-08-30）已經幫剩下
-- 41 支未遷移 seller template 對應的 7 個服務全部補上了原始、未解析的 store_id
-- 欄位（純 additive，Phase A）。本批（batch2）處理其中可以直接切換、零額外程式碼
-- 風險的 36 支。
--
-- 排除的 5 支，理由各自不同：
--   - comment.created／comment.like_changed／comment.replied.seller（3 支）：
--     notification-service worker/template_handler.go 的 actorFieldByEventType
--     為這 3 支登記了 self-notification skip，actor 欄位（commenterPrincipalId／
--     commentAuthorStoreId... 現況是 commenterPrincipalId／replierPrincipalId／
--     likerPrincipalId）目前跟 target_user_field 同樣落在已解析的 account_id
--     空間。若本批把 target_user_field 切到 Phase A 新加的 raw store_id 欄位、
--     卻不同步處理 actor 欄位，兩者會落在不同身分空間、self-skip 的比對永遠
--     不成立——不會報錯，是安靜地不再 skip，賣家會開始收到自己對自己動作的推播。
--     payload 目前沒有 actor 側對應的原始 store_id 欄位可用，需要 social-service
--     先補一次小型 Phase-A 式程式碼改動，留給獨立的 batch3 PR 處理，不在本批範圍。
--     （worker/template_contract_test.go 的 assertIdentitySpace 對 actorFieldByEventType
--     也有機械化檢查，這 3 支若誤入本批，contract test 會直接 FAIL，不是靜默通過。）
--   - procurement.quote_invite.created.seller／procurement.quote_invite.lost.seller
--     （2 支）：全 repo 查無任何 producer 發布這兩個 event_type，只是
--     20260815120000_procurement_quote_invite_templates.sql 為未來功能預先建立
--     的模板 row（該 migration header 已註明 enabled=false、publisher 是 stub）。
--     沒有 producer 就沒有 Phase A 可補的欄位，也沒有現行洩漏風險，不屬於本次
--     issue #2707 的範圍，維持 target_user_field='seller_account_id'、
--     identity_migrated=false 原樣不動，等該功能真正實作時直接以 store_id 為
--     契約起手，不留第三次技術債。
--
-- 逐支對過 Phase A 各 PR 實際加的欄位名稱（不是只看 event_type 猜測）：
--
-- ══════════════════════════════════════════════════════════════════════
-- Part A：seller_account_id → store_id（25 支）
-- ══════════════════════════════════════════════════════════════════════
--
-- aftersales-service 21 支（PR #2918）：CaseTransitionPayload（17/21 最大宗，共用
-- 一支 buildCaseTransitionPayload）／CaseAutoEscalatedPayload／RefundCompletedPayload／
-- AppealSubmittedPayload／PlatformDecisionMadePayload 皆新增 Java storeId component，
-- Jackson 預設 snake_case 序列化為 JSON "store_id"。
-- shipping-service 3 支（PR #2937）：ShipmentCreatedPayload／ShippingEventPayload／
-- 共享 ShipmentAnomalyDetectedPayload（common-core，3 個 producer 已同步）皆新增
-- Java storeId → JSON "store_id"。
-- buyer-service product.favorited（PR #2928）：BuyerFavoriteEvent.StoreID string
-- `json:"store_id"`。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'seller_account_id'
  AND event_type IN (
    'aftersales.appeal.submitted.seller',
    'aftersales.case.applied.seller',
    'aftersales.case.auto_completed_confirm_receipt_timeout.seller',
    'aftersales.case.auto_completed_return_pending_timeout.seller',
    'aftersales.case.auto_completed_store_pickup_timeout.seller',
    'aftersales.case.auto_escalated.seller',
    'aftersales.case.cancelled_by_buyer.seller',
    'aftersales.case.cancelled_by_confirm_receipt.seller',
    'aftersales.case.coupon_compensated.seller',
    'aftersales.case.refund_approved.seller',
    'aftersales.case.rejected.seller',
    'aftersales.case.return_approved.seller',
    'aftersales.case.return_at_store.seller',
    'aftersales.case.return_at_store_24h.seller',
    'aftersales.case.return_delivered.seller',
    'aftersales.case.return_delivered_24h.seller',
    'aftersales.case.return_failed.seller',
    'aftersales.case.return_in_transit.seller',
    'aftersales.case.return_received.seller',
    'aftersales.platform_decision.made.seller',
    'aftersales.refund.completed.seller',
    'product.favorited',
    'shipment.anomaly.detected.seller',
    'shipment.created',
    'shipment.delivered.seller'
  );

-- ══════════════════════════════════════════════════════════════════════
-- Part B：receiver_id → store_id（1 支，聊天訊息）
-- ══════════════════════════════════════════════════════════════════════
--
-- chat.message.sent（PR #2925）：im-service internal/usecase/message.go
-- ChatMessageSentEvent.StoreID string `json:"store_id"`——無條件帶當下房間賣家側
-- 的原始 store_id，不受收件人角色（買家／賣家）影響（見欄位註解）。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'receiver_id'
  AND event_type = 'chat.message.sent';

-- ══════════════════════════════════════════════════════════════════════
-- Part C：merchant_id → store_id（6 支）
-- ══════════════════════════════════════════════════════════════════════
--
-- follow.added（PR #2928）：buyer-service pkg/mq/events.go FollowAddedEvent.StoreID
-- int64 `json:"store_id"`。
-- merchant.verification.*（5 支，PR #2933）：seller-service
-- internal/domain/merchant_verification_outbox.go
-- MerchantVerificationCompletedEvent.StoreID int64 `json:"store_id"`，approved／
-- forwarded／pending_info／rejected／under_review 五個狀態共用同一個 outbox row
-- 建構子，全部帶同一個欄位（含全系列唯一 MODERATE 修復：admin 拒絕路徑改用
-- GetByAccountIDForUpdate，已 live 活觸發驗證過無視關店狀態仍正確解析 store_id）。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'merchant_id'
  AND event_type IN (
    'follow.added',
    'merchant.verification.approved',
    'merchant.verification.forwarded',
    'merchant.verification.pending_info',
    'merchant.verification.rejected',
    'merchant.verification.under_review'
  );

-- ══════════════════════════════════════════════════════════════════════
-- Part D：notify_seller_account_id → store_id（1 支，撥款完成）
-- ══════════════════════════════════════════════════════════════════════
--
-- payout.completed（PR #2935）：order-service GrantPaymentWorker.java 既有的
-- Long storeId 區域變數（原本就給 resolvePrincipals 用），PayoutCompletedPayload
-- 新增 Java storeId → JSON "store_id"，零新查詢。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'notify_seller_account_id'
  AND event_type = 'payout.completed';

-- ══════════════════════════════════════════════════════════════════════
-- Part E：seller_id → store_id（3 支）
-- ══════════════════════════════════════════════════════════════════════
--
-- product.viewed（PR #2928）：buyer-service ProductViewedEvent.StoreID int64
-- `json:"store_id"`。
-- post.hidden_by_platform.seller／post.liked（PR #2921）：social-service
-- pkg/mq/events.go PostHiddenByPlatformEvent.StoreID／PostLikedEvent.StoreID
-- int64 `json:"store_id"`——皆無條件是真正的 store_id，不受任何 actor 比對邏輯
-- 影響（template_handler.go 的 actorFieldByEventType 未登記這兩支，跟被排除的
-- 3 支 comment.* 不同類）。
UPDATE "notification_templates"
SET target_user_field = 'store_id',
    updated_at         = CURRENT_TIMESTAMP
WHERE target_user_type = 'seller'
  AND target_user_field = 'seller_id'
  AND event_type IN (
    'product.viewed',
    'post.hidden_by_platform.seller',
    'post.liked'
  );
