-- RFQ 發布直投線的收件人通知模板（#2450 Slice 2b、64 號設計書 §4.2–§4.4、36 號 §3.6l.1）。
--
-- Producer : ai-service `procurement_rfq_publication_outbox` worker
--            （internal/procurement/publicationoutbox）→ exchange `procurement.events`
--            routing key `procurement.rfq_publication_invite.created`
--            （`pkg/mq.RoutingKeyProcurementRfqPublicationInviteCreated`）
-- Consumer : notification-service 模板引擎，queue `notification.procurement`（`'#'` 綁定）
--            ⇒ 已登記於 infra/mq/topology/topology.yaml，consumer.go 的
--              notificationQueueByExchange 也已有 "procurement.events" 一列
--            ⇒ **本檔不需要動 topology 與 consumer map**，缺的只有模板列本身。
--
-- ⚠️ 這條線與 20260815120000 的 procurement quote-invite 是**兩條不同的線**，只共用
--    exchange。payload 形狀不同：本線恰好三欄且全為 camelCase 字串化 TSID
--    （`inviteId` / `publicationId` / `storeId`，producer 端
--    services/ai-service/internal/procurement/usecase/publication_execute.go 的
--    `inviteCreatedPayload`），quote-invite 那條是 snake_case `store_id`。
--    **兩者的欄名不可互抄**——抄錯的後果是 target_user_field 解析不到收件人，
--    通知靜默 0 投遞（template_handler.go 對取不到 user_id 只印 Warn 後 return）。
--    因此本列的 target_user_field 是 `storeId`，不是 `store_id`。
--
-- ── 為什麼 reference_id_template 必須是 `#{inviteId}` ─────────────────────────
-- worker 的順序刻意是「先發 MQ、後把 invite 轉 SENT」（publicationoutbox/worker.go
-- 的 process；對調會留下「說已送出但沒送」）。MarkDone 失敗時那一列會重發，而**吸收
-- 重發的機械保證**就是本服務的兩道 DB 唯一索引：
--     notifications_inbox_dedup_idx / push_notifications_dedup_idx
--     UNIQUE (user_id, user_type, event_type, reference_id)
-- 這個保證只有在 reference_id 逐 invite 唯一時才成立：
--   * 綁 `#{publicationId}`（同一場發布的所有收件人共用一個值）⇒ 同一場發布只有第一位
--     賣家收得到通知，其餘全被去重吞掉；
--   * **留空 ⇒ 去重整個關掉**（不是「退化成上一種」）：handleTemplate 算出空字串，
--     repository 的 nullableText（internal/repository/postgres/nullable.go:5-10）把 ""
--     映成 SQL NULL，而兩道索引都是 partial index（`WHERE reference_id IS NOT NULL`），
--     於是該列**根本不在索引裡**——每一次重發都會寫成一則全新的通知打擾賣家。
-- 兩種失效都沒有任何訊號（不進 DLQ、不重試、沒有 metric），所以綁定關係另有
-- internal/worker/procurement_rfq_publication_invite_test.go 逐條斷言（含直接讀本檔的
-- 錨點測試），不是只靠這段註解。
--
-- ── 本列刻意不給 refType／refId，schema_url 維持 NULL ────────────────────────
-- 13 號設計書 §5.2 的 refType 是**封閉值域**，既有成員沒有一個描述得了「RFQ 發布 invite」
-- 這個資源；而賣家回覆面（64 §4.4 的 sellerResponseVersion）本波尚未實作，沒有落地頁可指。
-- 硬挑 `PROCUREMENT_QUOTE_INVITE`（那是 §3.6l 的報價單邀請，另一個資源、另一張表）正是
-- 該值域要防的錯誤：前端會拿 publication invite id 去開報價單頁。
-- gateway 只在 refType 與 refId 皆非空時才回傳整組（notification_inbox.go），所以兩者
-- 一起不給是完整且一致的狀態，本列已登記在 template_contract_test.go 的
-- deliberatelyNoRefContract **並寫進 13 號 §5.2 的對照表**（兩者缺一即靜默漂移，
-- 沒有任何腳本會拿 §5.2 去對 DB）。
--
-- ⚠️ 「本期無落地頁」**不是阻擋條件**（13 號 §5.2 逐字寫明，DELEGATION 與
--    PROCUREMENT_QUOTE_INVITE 現況即是）。擴充值域這條路是通的，而且只需改
--    13 號 §5.2 與 refTypeDomain，兩者都在本 repo；PROCUREMENT_QUOTE_INVITE 當初就是在
--    三個前端零分支的情況下這樣加進來的。本次選擇不擴充，是因為「賣家收到這則通知後應該
--    去哪裡」是尚未定案的**產品決定**，不是技術阻礙。
--    代價要講明：延後的後果是**即使賣家回覆面上線，通知仍然無處可跳**。
-- 🔓 解除條件：賣家回覆面落地時，先改 13 號 §5.2 的值域清單與對照表（寫明它指向什麼資源、
--    四個消費者各自的處置），再改 refTypeDomain，最後才用新的 migration 補 refType/refId
--    與 schema_url——順序不可顛倒。
--
-- ── enabled = true（與 20260815120000 那兩列當時的處境不同）────────────────────
-- 那兩列建立時 exchange 不存在、publisher 是 stub，所以刻意停用。本列不是：
--   1. producer 已實作並已 commit（publicationoutbox worker ＋ execute 交易內寫 outbox）
--   2. exchange `procurement.events` 與 queue `notification.procurement` 已在 topology.yaml，
--      且已由 20260904100000 啟用的 quote-invite 模板實際在消費
--   3. tests/contract/golden/event_payload_keys.json 已補上本事件的 emitted_keys
-- 停用等於讓已經發得出去的事件繼續被 logger.Debug + ACK 靜默丟棄，那正是本檔要消滅的狀態。
--
-- identity_migrated = true 為 seller 列的硬性條件（chk_notification_templates_seller_identity_migrated）；
-- 本列自始就寫 store_id 身分空間，沒有 account_id 相容分流可回退。

INSERT INTO "notification_templates" (
  "exchange", "routing_key",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "enabled", "push_mode", "identity_migrated"
) VALUES
(
  'procurement.events', 'procurement.rfq_publication_invite.created',
  'procurement.rfq_publication_invite.created.seller', 'seller', 'storeId',
  'order_update',
  '買家指名向你採購',
  '有買家把採購需求直接發給你，請開啟網頁查看',
  NULL,
  'PROCUREMENT_RFQ_PUBLICATION_INVITE',
  '{
     "inviteId":      {"field": "inviteId"},
     "publicationId": {"field": "publicationId"}
   }'::jsonb,
  NULL, '#{inviteId}',
  true, 'visible', true
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
