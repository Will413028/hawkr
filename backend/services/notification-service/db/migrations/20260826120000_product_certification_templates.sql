-- 商品農產品認證審核結果通知 — 三支 DB-driven template（19 號設計書 §12 / 13 號設計書「商品認證」節 / 36 §3.6h.1）。
--
-- Producer : product-service ProductEventPublisher（核准／駁回／撤銷交易 commit 後的 afterCommit hook）
--            exchange=hawkr.product, routing_key=product.certification.{approved,rejected,revoked}
-- Consumer : notification-service 模板引擎，既有 queue notification.product（'#' 綁定）
--            ⇒ **本次不需要新增 queue、不需要動 infra/mq/topology/topology.yaml**
--            （19 §12.2 要新增的 product.certification.verify 是 product-service 自己消費的背景查驗
--              queue，與本檔無關）。
--
-- ── 為什麼 target_user_field 是 seller_account_id 而不是旁邊的 store_id ──────────
-- product-service 的資料是 store_id-keyed（product_certification_application.store_id），
-- 而賣家收件匣的讀取端（gateway/internal/handler/notification_inbox.go）對 user_type != 'buyer'
-- 直接拿 X-Account-Id 當 user_id，**不解析 store surrogate**。payload 兩個欄位都在，填錯
-- 不報錯、不進 DLQ、沒有任何 log —— 而首任店家（store_id == account_id）會讓 dev 與所有
-- fixture 全綠，只有 re-key 過的店家永遠收不到。CLAUDE.md §6 明令禁止假設 store_id == account_id。
-- publisher 側對應的紅線：seller_account_id 必須 fail-closed 反解（getSellerMetaOrThrow），
-- 不得用失敗回空 Map 的批次版本 —— 那會發出缺收件人的 payload，事件已「成功」發出、重送救不回來。
--
-- ── payload wire 是 snake_case ─────────────────────────────────────────────────
-- 本檔所有 #{...} 與 data_fields.field 都用 snake_case，因為同服務的既有 payload record
-- （ProductRestockPayload）帶 @JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy)，
-- 且本 repo 全部 Java publisher 的 golden emitted_keys 都是 snake_case。
-- 對應的 golden 條目在 tests/contract/golden/event_payload_keys.json。
--
-- ── category = 'system'（刻意不新增 category）──────────────────────────────────
-- §9 的 category 是兩支 App 通知偏好頁的開關集合，多一個值就是前端契約變更，本期不做。
-- system 是既有八個值裡唯一容納得下「平台對賣家的裁決結果」的那一個。
-- ⚠️ 與 merchant.verification.*（商家 KYC）共用同一個 category 是取捨的結果，不是語意相同；
--    文案不得互借（那組 approved 文案寫的是「可開始正式收款、提領與配送設定」）。
--
-- ── data_type = 'PRODUCT_CERTIFICATION_STATUS'（三支共用）──────────────────────
-- 三個裁決共用同一個前端路由，以 event_type 區分子情境 —— 與售後 22 支共用 AFTER_SALE_STATUS
-- 同一種作法。`.seller` 後綴不進 data.type。
--
-- ── refType / refId = PRODUCT / product_id ────────────────────────────────────
-- refType 值域是封閉的（13 §5.2），裡面沒有描述得了「認證申請」的值，而賣家點開通知要去的
-- 地方是**那個商品**。PRODUCT 已在值域內，本次不需要新增 refType。
-- ⚠️ refId（跳轉目標）與 reference_id（去重鍵）刻意不同值：全表慣例是 refId ≡ reference_id，
--    本節是明列的例外 —— refId 要指名跳轉目標（商品），reference_id 要指名這一次裁決（申請）。
--
-- ── reference_id_template = 'product_cert_#{application_id}' ──────────────────
-- 冪等由 notifications 的 UNIQUE (user_id, user_type, event_type, reference_id) 承擔。
-- 取 application_id 而非 product_id：否則同一商品第二次審核會被前一次的列吃掉（唯一鍵撞上、
-- 靜默不寫），而那正是賣家最需要知道的那一則。
--
-- ── schema_url 留 NULL ────────────────────────────────────────────────────────
-- 本期兩個賣家端都沒有落地頁：seller App 不做（19 號「不做」清單），賣家 web 的
-- lib/notifications/resolve.ts 只認 order／chat／post 三組 refType。而 seller App 的 FCM router
-- 對未知 data.type 走 default 分支、只要 data['url'] 非空就 launchUrl(externalApplication)
-- ⇒ 填一個沒有頁面的 deep link 比不填更糟。這是已知缺口不是漏接，前端要接時的最小材料
-- （product_id / product_name / certification_type）已在 data 內。
--
-- ── 三支都 enabled = true、push_mode = 'visible' ──────────────────────────────
-- 「同步快取命中的即刻核准不發通知」這個分流在 **publisher 側**決定（那條路徑不發
-- product.certification.approved），不是在模板側 —— enabled 與 push_mode 都表達不了
-- 「同一個 routing key 有時要發有時不要」。
--
-- ⚠️ 模板列缺席是本鏈路最靜默的失敗：template_handler.go 對查不到模板的 (exchange, routing_key)
--    走 logger.Debug + return nil（ack）⇒ 不重試、不進 DLQ、直接消失。驗收不得只看
--    consumers >= 1，必須端到端斷言 notifications 出現對應列。
--
-- ⚠️ public_reason 用 #{public_reason|預設值} 的可選語法：該欄在 DB 可為 NULL，
--    publisher 沒帶時走預設文案，而不是整則 body 渲染失敗掉進 body_fallback。
INSERT INTO "notification_templates" (
  "exchange", "routing_key",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "enabled", "push_mode"
) VALUES
-- product.certification.approved → seller
(
  'hawkr.product', 'product.certification.approved',
  'product.certification.approved.seller', 'seller', 'seller_account_id',
  'system',
  '商品認證已通過',
  '您的商品「#{product_name}」的農產品認證已通過審核，標章已顯示在商品頁。',
  '您有一項商品農產品認證已通過審核，標章已顯示在商品頁。',
  'PRODUCT_CERTIFICATION_STATUS',
  '{
     "refType":           {"const": "PRODUCT"},
     "refId":             {"field": "product_id"},
     "productId":         {"field": "product_id"},
     "productName":       {"field": "product_name"},
     "certificationType": {"field": "certification_type"}
   }'::jsonb,
  NULL, 'product_cert_#{application_id}',
  true, 'visible'
),
-- product.certification.rejected → seller
(
  'hawkr.product', 'product.certification.rejected',
  'product.certification.rejected.seller', 'seller', 'seller_account_id',
  'system',
  '商品認證未通過',
  '您的商品「#{product_name}」的農產品認證未通過審核。原因：#{public_reason|請至商品管理查看說明}',
  '您有一項商品農產品認證未通過審核，請至商品管理查看說明。',
  'PRODUCT_CERTIFICATION_STATUS',
  '{
     "refType":           {"const": "PRODUCT"},
     "refId":             {"field": "product_id"},
     "productId":         {"field": "product_id"},
     "productName":       {"field": "product_name"},
     "certificationType": {"field": "certification_type"}
   }'::jsonb,
  NULL, 'product_cert_#{application_id}',
  true, 'visible'
),
-- product.certification.revoked → seller
(
  'hawkr.product', 'product.certification.revoked',
  'product.certification.revoked.seller', 'seller', 'seller_account_id',
  'system',
  '商品認證已撤銷',
  '您的商品「#{product_name}」的農產品認證已被撤銷，標章已從商品頁移除。原因：#{public_reason|請至商品管理查看說明}',
  '您有一項商品農產品認證已被撤銷，標章已從商品頁移除，請至商品管理查看說明。',
  'PRODUCT_CERTIFICATION_STATUS',
  '{
     "refType":           {"const": "PRODUCT"},
     "refId":             {"field": "product_id"},
     "productId":         {"field": "product_id"},
     "productName":       {"field": "product_name"},
     "certificationType": {"field": "certification_type"}
   }'::jsonb,
  NULL, 'product_cert_#{application_id}',
  true, 'visible'
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
