-- 新增通知模板：procurement.quote_invite.created.seller / procurement.quote_invite.lost.seller
-- 對應功能：procurement 助手報價單邀請 Phase 1a（Task 11，PLATFORM lane 站內通知）
-- ai-service 的 procurement outbox worker（Task 10，internal/procurement/outbox）依 event_type
-- 找到這兩筆模板；payload 欄位命名（inviteId／batchId）對齊該 worker 實際發出的
-- inviteSentPayload／inviteLostPayload（services/ai-service/internal/procurement/outbox/writer.go）。
--
-- reference_id_template 刻意使用 https:// 網址而非 hawkr:// app scheme——本輪
-- 「網頁端、不做 App」的決定在通知層的落地；schema_url 維持 NULL（無 app deep link）。
-- reference_id_template 語法為 #{fieldName}（見 schema.sql 該欄位註解，由
-- internal/worker/template_handler.go 的 RenderTemplate 渲染，非 RenderBraces 的
-- {key} 語法——後者只用於 schema_url，此處未設）。
--
-- lost.seller 的 reference_id_template 必須帶 #{batchId}（不能是靜態字串）：
-- notification_templates 的 push_notifications dedup unique index 是
-- (user_id, user_type, event_type, reference_id)，若同一 event_type 的
-- reference_id 恆為常數，同一賣家第二次以後的「報價單已被其他賣家承接」通知會被
-- dedup 靜默吞掉（只收得到第一次）。data_fields.refId 已經對 batchId 參數化，
-- reference_id_template 比照 created.seller 用 #{inviteId} 的做法同步帶
-- #{batchId}，兩者維持一致。
--
-- enabled = false（Phase 1a 刻意）：exchange 'procurement.events' 目前不存在於
-- infra/mq/topology/topology.yaml，也不在 notification-service 的
-- notificationQueueByExchange（internal/worker/consumer.go）裡，因此這兩筆模板永遠
-- 不會被消費；enabled = true 的話 buildDynamicSubscriptions 會在**每一次開機**對每筆
-- 模板各記一行 ERROR（那段檢查刻意「大聲」，見該函式註解），把一個尚未接線的功能的
-- 雜訊灌進一個與本功能無關的服務。真正的發送端（ai-service procurement outbox 的
-- PublishFunc，internal/procurement/outbox/worker.go）本輪也還是 stub，沒有任何事件
-- 會被發出來，所以現在就開啟沒有任何好處。
-- 解除條件（Phase 1b／後續 PR，三者依 docs/開發規範/MQ 與事件管理規範.md 同一次變更完成）：
--   1. PublishFunc 真的實作並發出 procurement.quote_invite.created/lost
--   2. exchange 與 queue 進 infra/mq/topology/topology.yaml 並登記進
--      notificationQueueByExchange，MQ catalog（docs/詳細設計書/36）同步更新
--   3. 這兩列改 enabled = true（用新的 migration，不是改這一支）
-- 同一次也要補 tests/contract/golden/event_payload_keys.json 並移除
-- known_pending.json 的兩筆（TestTemplateContract 只掃 enabled = true 的列，
-- 停用期間那兩筆 known_pending 讀不到，但重新啟用時就需要）。

INSERT INTO "notification_templates" (
  "id",
  "exchange", "routing_key",
  "event_type", "target_user_type", "target_user_field",
  "category",
  "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "fcm_topic", "enabled"
) VALUES
-- procurement.quote_invite.created → seller（買家寄送報價單邀請）
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22),
  'procurement.events', 'procurement.quote_invite.created',
  'procurement.quote_invite.created.seller', 'seller', 'seller_account_id',
  'order_update',
  '買家寄送報價單',
  '有買家想跟你採購，請儘速確認報價（#{inviteId}）',
  '您有新的報價邀請，請開啟網頁查看',
  'PROCUREMENT_QUOTE_INVITE',
  '{"refId": {"field": "inviteId"}, "refType": {"const": "PROCUREMENT_QUOTE_INVITE"}}',
  NULL,
  'https://example.com/seller-center/procurement-quotes/#{inviteId}',
  NULL, false -- 見檔頭 enabled = false 的說明
),
-- procurement.quote_invite.lost → seller（報價單已被其他賣家承接）
(
  ((EXTRACT(EPOCH FROM NOW()) * 1000 - 1577836800000)::bigint << 22) + 1,
  'procurement.events', 'procurement.quote_invite.lost',
  'procurement.quote_invite.lost.seller', 'seller', 'seller_account_id',
  'order_update',
  '報價單已被其他賣家承接',
  '這筆採購已經由其他賣家確認，感謝您的回應',
  '您有報價邀請更新，請開啟網頁查看',
  'PROCUREMENT_QUOTE_INVITE',
  '{"refId": {"field": "batchId"}, "refType": {"const": "PROCUREMENT_QUOTE_INVITE"}}',
  NULL,
  'https://example.com/seller-center/procurement-quotes/#{batchId}',
  NULL, false -- 見檔頭 enabled = false 的說明
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
