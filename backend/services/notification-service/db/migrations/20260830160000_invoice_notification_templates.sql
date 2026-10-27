-- 電子發票通知模板 —— 58 號設計書 §6.3.9「定版 13 列」＋ §D.4（deep link 落點）。
--
-- Producer : invoice-service 的 outbox dispatcher（internal/worker/outbox_worker.go）
--            exchange=hawkr.invoice，12 個 routing key（pkg/mq/exchanges.go:71-82）
-- Consumer : notification-service 模板引擎，queue notification.invoice（'#' 綁定）
--            ⇒ 已登記於 infra/mq/topology/topology.yaml，consumer.go 的
--              notificationQueueByExchange 也已有 "hawkr.invoice" 一列
--            ⇒ **本檔不需要動 topology 與 consumer map**，缺的只有模板列本身。
--
-- ── 為什麼這一檔是「靜默失效」等級 ──────────────────────────────────────────
-- template_handler.go 對查不到模板的 (exchange, routing_key) 走 logger.Debug + return nil
-- （ack）⇒ 不重試、不進 DLQ、沒有 metric。本檔上線前，invoice-service 每一則已經發得出去
-- 的發票事件都被靜默丟棄，買家收不到任何發票通知，而且沒有任何東西會報警。
--
-- ── 13 列裡 8 列 enabled，其餘 5 列 enabled = false ────────────────────────
-- 依 58 §6.3.9「若 publisher 與模板落在不同 PR：走既有的 enabled=false ＋
-- known_pending.json 模式，並在 migration 檔頭寫死解除條件」。三類原因：
--
--   (A) publisher 根本還沒實作（5 列）
--       invoice.voided / invoice.allowance.issued / invoice.reissued /
--       invoice.operation.review_required / invoice.complaint.overdue
--       —— 2026-08-30 實查 services/invoice-service/internal/ 全域，這 5 個 routing key
--       常數零使用點（只有 pkg/mq/exchanges.go 的宣告與註解提及）。沒有 publisher 就
--       沒有 emitted_keys 可對，硬填 golden 就是 #1529 那個 class（golden 手寫錯不會被
--       任何 gate 抓到，賣家送達通知因此靜默 0 投遞）。
--       🔓 解除條件：該 routing key 在 invoice-service 出現 outbox Insert 呼叫點，
--          且 tests/contract/golden/event_payload_keys.json 補上「從 publisher struct
--          json tag 逐欄抄來的」emitted_keys，涵蓋本檔該列用到的每一個 #{...} 與
--          data_fields[*].field；然後從 known_pending.json 移除該列並 SET enabled = true。
--
--   (B)〔已解除，2026-08-30〕publisher 曾把 store_id 填成 invoice_issuers 的 **PK**
--       invoice.issued.seller / invoice.issue_failed.seller / invoice.complaint.created.seller
--       —— outbox_worker.go 的 withNotifyRouting 讀 payload["store_id"] 去
--       ResolveSellerAccountID，而 invoice_issuers.id 是 TSID、issuer_ref_id 才是
--       sellers.store_id（invoice schema.sql §5.1.1）⇒ 解析必失敗 ⇒
--       notify_seller_account_id 恆為 "0" ⇒ 該則通知被下游 skip，全程零錯誤零 DLQ。
--       ✅ 修復：三支 publisher（submit.go 的 issuedEvent／issueFailedEvent、
--          complaint.go 的 complaintCreatedEvent）改走 usecase/helpers.go 的
--          storeIDWire(issuer)，值取自 invoice_issuers.issuer_ref_id。
--          同批把 seller_complaint.go 的 complaintRespondedEvent 一併修掉（同一個 class）。
--       ✅ 解除依據（原文寫的是「在 dev 實際發一則、確認非 0」，這裡換成**每次 CI 都跑**
--          的等價證據，因為 invoice-service 尚未部署 dev、peer client 也還沒接上）：
--            ① internal/worker/integration_test.go 的
--               TestIntegration_EndToEnd_SellerNotifyRoutingResolves —— 真 PostgreSQL，
--               issuer PK 與 issuer_ref_id 是不同的兩個值，resolver **只認 store_id**，
--               斷言派送後 payload 的 store_id ＝ issuer_ref_id 且
--               notify_seller_account_id ≠ "0"。實測把 publisher 改回 doc.IssuerID 即轉紅。
--            ② internal/usecase/event_store_id_contract_test.go 的 R2 —— AST 規則，
--               禁止任何事件 payload 的 store_id 由 storeIDWire(issuer) 以外的運算式產生。
--
--   (C)〔已解除，2026-08-30〕publisher 的 wire 欄名曾是 camelCase（1 列）
--       invoice.seller_tax_profile.status_changed.seller
--       —— tax_profile.go 的 taxStatusChangedEvent 用
--          notificationRefId / storeId / taxProfileId / fromStatus / toStatus / occurredAt，
--          與 58 §6.3.4「業務欄位一律 snake_case」及其餘 11 個事件全部相反。
--          連帶後果與 (B) 相同：withNotifyRouting 找的是 payload["store_id"]。
--       ✅ 修復：六個 json tag 全改 snake_case，store_id 同時改走 storeIDWire(issuer)。
--          tests/contract/golden/event_payload_keys.json 的 emitted_keys 已同步。
--       ✅ 解除依據：event_store_id_contract_test.go 的 R1（AST 規則，事件 payload 的
--          json tag 一律 snake_case）＋ 模板契約測試對 #{notification_ref_id} 的既有檢查。
--
-- ⚠️ 剩下的 5 列（全部是 (A) 類）**不是**「以後再說」：它們登記在 tests/contract/golden/known_pending.json，
--    每一筆都寫了上面對應的解除條件。契約測試對停用列仍然執行 A 組四條檢查
--    （refType 值域 / refType-refId 成對 / schema_url 渲染 / deliberatelyNoRefContract 反向），
--    所以「被 enable 的那一天」不會是它們第一次受檢查的日子。
--
-- ── refType = INVOICE 是 refTypeDomain 的新增成員（58 §D.4）───────────────────
-- 文件類八列（本表 #1、#2、#4–#8、#10）的 refType 為 INVOICE、refId 為
-- invoice_document_id。舊版 58 曾主張沿用 ORDER，該論證只涵蓋「賣家→買家（有訂單）」
-- 這一個方向；§2.3 矩陣的另外三個方向（平台→賣家服務費／平台→買家訂閱／平台→非會員）
-- source_ref 恆為 NULL ⇒ 沿用 ORDER 的後果不是多跳一次，是 refId **沒有值可填**。
-- 同一波已把 INVOICE 加進 template_contract_test.go 的 refTypeDomain。
-- ⚠️ 13 號設計書 §5.2 的值域清單與對照表為該常數的 SSOT，本波同步新增（順序：文件先、常數後）。
--
-- ── schema_url 的 brace 鍵名是 data_fields 的鍵，不是 payload 欄名 ───────────
-- 58 §D.4 的表把 deep link 寫成 `/account/invoices/#{document_id}`，那個語法在本引擎
-- **渲染不出來**：schema_url 只經過 RenderBraces，而 RenderBraces 會**跳過** `#{` 序列
-- （template_handler.go:347-352）⇒ 送到裝置上的 data.url 會是一段模板字串。
-- 正確寫法是 `{documentId}`，其中 documentId 必須是本列 data_fields 的鍵。
-- （同一個坑的前例：20260816120000 一次修掉 23 支，含 `#{deepLink}` 那支。）
-- 另外 payload 的 wire 欄名是 `invoice_document_id` 不是 `document_id`
--（invoice-service 的 usecase/submit.go，型別 issuedEvent 的 InvoiceDocumentID 欄位；
--  行號會漂，錨用符號名）。
--
-- ── category：買家向 order_update、賣家向壞消息 system ────────────────────────
-- 58 §6.3.9 逐列指定。order_update 是可關閉的偏好類別，賣家關掉之後「開票失敗」
-- 「申訴逾期」就靜默消失，而那兩件事有法定期限 ⇒ 一律 system。
-- **刻意不新增 invoice category**：那要同步改 notification_preference.go 的三份清單
-- ＋ usecase 的 categoryMetaByUserType，再乘上三個前端。
--
-- ── payload 邊界（36 號 / 58 §6.3.3.2）──────────────────────────────────────
-- 🔴 本檔任何一列都**不得**引用 random_code：invoice_no + random_code 就能在財政部平台
--    查到整張發票的全部內容 ⇒ 它是**憑證**不是資料。判準是「拿到它能做什麼」，不是
--    「是不是 PII」。載具識別碼／買受人統編／email／電話同理（payload 本來就只給
--    has_carrier / has_buyer_tax_id 兩個布林）。
-- 發票號碼（invoice_no）則**完整顯示不遮罩**（58 §8.1.3(b) O 類、裁決 1）。
--
-- ── 文案：第 13 列（賣家站外自行開立）是硬約束 ──────────────────────────────
-- 標題與內文都不得寫成「平台已開立」或任何讀起來像平台開的話（58 §6.3.9）；措辭與
-- 58A B04 的 SELLER_SELF_ISSUED banner 同源（「賣家已自行開立發票／由賣家開立與寄送」）。
-- 同一件事在推播與頁面上講成兩種說法，買家會以為是兩件事。
-- 對應地，wire 欄名是 external_invoice_no 不是 invoice_no —— 下游若照 P1 的習慣讀
-- invoice_no，就會把賣家自行開立的號碼當成平台開的（**平台冒充開立人**）。
--
-- ── reference_id_template 一律 #{notification_ref_id}（58 §6.3.3.4）──────────
-- notifications 的冪等鍵是 (user_id, user_type, event_type, reference_id)。只填 order_no
-- 的話，「開立 → 作廢 → 重開」的第二、三次通知會被靜默吞掉。每則 payload 自帶
-- notification_ref_id（INV: / INVOP: / INVCPL: / INVEXT: / INVRS: 前綴，publisher 側組）。
-- 它是**冪等鍵不是 URL**（有前例把它填成完整網址）；URL 走 schema_url。
INSERT INTO "notification_templates" (
  "exchange", "routing_key",
  "event_type", "target_user_type", "target_user_field",
  "category", "title_template", "body_template", "body_fallback",
  "data_type", "data_fields", "schema_url", "reference_id_template",
  "enabled", "push_mode"
) VALUES
-- ───────────────────────────────────────────────────────────────────────────
-- #1  invoice.issued → buyer（P1 買家向）  ✅ enabled
--     publisher: submit.go issuedEventPayload（開立）＋ notification_resend.go emitResend（重寄）
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.issued',
  'invoice.issued.buyer', 'buyer', 'buyer_id',
  'order_update',
  '電子發票已開立',
  '您的電子發票已開立，發票號碼 #{invoice_no}，金額 NT$#{total_amount}。可至發票中心查看或下載。',
  '您有一張電子發票已開立，請至發票中心查看。',
  'INVOICE_ISSUED',
  '{
     "refType":     {"const": "INVOICE"},
     "refId":       {"field": "invoice_document_id"},
     "documentId":  {"field": "invoice_document_id"},
     "invoiceNo":   {"field": "invoice_no"},
     "invoiceDate": {"field": "invoice_date"},
     "totalAmount": {"field": "total_amount"},
     "orderNo":     {"field": "order_no"}
   }'::jsonb,
  'https://example.com/account/invoices/{documentId}', '#{notification_ref_id}',
  true, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #2  invoice.issued → seller（P1 賣家向，silent）  ✅ enabled（原因 (B) 已解除）
--     silent = data-only push 不寫 inbox，只是讓賣家 App 刷新發票列表。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.issued',
  'invoice.issued.seller', 'seller', 'notify_seller_account_id',
  'order_update',
  '電子發票已開立',
  '已為您的訂單開立電子發票，發票號碼 #{invoice_no}，金額 NT$#{total_amount}。',
  '您有一張電子發票已開立。',
  'INVOICE_ISSUED',
  '{
     "refType":     {"const": "INVOICE"},
     "refId":       {"field": "invoice_document_id"},
     "documentId":  {"field": "invoice_document_id"},
     "invoiceNo":   {"field": "invoice_no"},
     "invoiceDate": {"field": "invoice_date"},
     "totalAmount": {"field": "total_amount"},
     "orderNo":     {"field": "order_no"}
   }'::jsonb,
  'https://example.com/seller-center/invoices/{documentId}', '#{notification_ref_id}',
  true, 'silent'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #3  invoice.issue_failed → seller（P2）  ✅ enabled（原因 (B) 已解除）
--     刻意無 refType/refId：開票**失敗**時 invoice_documents 還沒有那一列，
--     沒有 document_id 可指。schema_url 因此是靜態的 jobs 列表（58 §D.4 第 3 列）。
--     已登記 template_contract_test.go 的 deliberatelyNoRefContract。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.issue_failed',
  'invoice.issue_failed.seller', 'seller', 'notify_seller_account_id',
  'system',
  '電子發票開立失敗',
  '有一張電子發票開立失敗（訂單 #{order_no|—}），請至賣家中心的「發票異常作業」查看原因並處理。',
  '有一張電子發票開立失敗，請至賣家中心的「發票異常作業」查看原因並處理。',
  'INVOICE_ISSUE_FAILED',
  '{
     "invoiceOperationId": {"field": "invoice_operation_id"},
     "failureClass":       {"field": "failure_class"},
     "failureReasonCode":  {"field": "failure_reason_code"},
     "orderNo":            {"field": "order_no"}
   }'::jsonb,
  'https://example.com/seller-center/invoices/jobs', '#{notification_ref_id}',
  true, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #4  invoice.voided → buyer（P3）  ⛔ disabled：原因 (A)
--     解除時 golden 需含：notification_ref_id / invoice_document_id / voided_invoice_no
--     （58 §6.3.3.3 P3 差異欄：voided_invoice_no、void_reason_code、voided_at）
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.voided',
  'invoice.voided.buyer', 'buyer', 'buyer_id',
  'order_update',
  '電子發票已作廢',
  '發票號碼 #{voided_invoice_no} 已作廢。若涉及退款將另行通知。',
  '您有一張電子發票已作廢，請至發票中心查看。',
  'INVOICE_VOIDED',
  '{
     "refType":         {"const": "INVOICE"},
     "refId":           {"field": "invoice_document_id"},
     "documentId":      {"field": "invoice_document_id"},
     "voidedInvoiceNo": {"field": "voided_invoice_no"}
   }'::jsonb,
  'https://example.com/account/invoices/{documentId}', '#{notification_ref_id}',
  false, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #5  invoice.allowance.issued → buyer（P4）  ⛔ disabled：原因 (A)
--     解除時 golden 需含：notification_ref_id / invoice_document_id /
--     cumulative_allowance_amount（58 §6.3.3.3 P4：折讓欄 ＋ 累計 ≤ 已開立金額的 clamp）
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.allowance.issued',
  'invoice.allowance.issued.buyer', 'buyer', 'buyer_id',
  'order_update',
  '折讓單已開立',
  '您的訂單已開立折讓單，累計折讓金額 NT$#{cumulative_allowance_amount}。可至發票中心查看。',
  '您有一張折讓單已開立，請至發票中心查看。',
  'INVOICE_ALLOWANCE_ISSUED',
  '{
     "refType":                  {"const": "INVOICE"},
     "refId":                    {"field": "invoice_document_id"},
     "documentId":               {"field": "invoice_document_id"},
     "cumulativeAllowanceAmount": {"field": "cumulative_allowance_amount"}
   }'::jsonb,
  'https://example.com/account/invoices/{documentId}', '#{notification_ref_id}',
  false, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #6  invoice.reissued → buyer（P5）  ⛔ disabled：原因 (A)
--     解除時 golden 需含：notification_ref_id / invoice_document_id /
--     voided_invoice_no / new_invoice_no（58 §6.3.3.3 P5）
--     refId 取共同欄 invoice_document_id：重開會產生新 document，冪等鍵
--     INV:{invoice_document_id} 天然不同（§6.3.3.4）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.reissued',
  'invoice.reissued.buyer', 'buyer', 'buyer_id',
  'order_update',
  '電子發票已重新開立',
  '原發票 #{voided_invoice_no} 已作廢並重新開立，新發票號碼 #{new_invoice_no}。',
  '您的電子發票已重新開立，請至發票中心查看。',
  'INVOICE_REISSUED',
  '{
     "refType":         {"const": "INVOICE"},
     "refId":           {"field": "invoice_document_id"},
     "documentId":      {"field": "invoice_document_id"},
     "voidedInvoiceNo": {"field": "voided_invoice_no"},
     "newInvoiceNo":    {"field": "new_invoice_no"}
   }'::jsonb,
  'https://example.com/account/invoices/{documentId}', '#{notification_ref_id}',
  false, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #7  invoice.receipt.issued → buyer（P6）  ✅ enabled
--     免用統一發票賣家的平台代製**收據**。文案不得讓買家以為拿到的是統一發票
--     （publisher 走同一個 issuedEvent struct，wire 欄名仍是 invoice_no，
--       但對收據而言它裝的是收據號碼 —— notification_resend.go:255-263 的
--       resendInvoiceNo 對 RECEIPT 取 doc.ReceiptNo 即為明證）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.receipt.issued',
  'invoice.receipt.issued.buyer', 'buyer', 'buyer_id',
  'order_update',
  '收據已開立',
  '您的收據已開立，收據號碼 #{invoice_no}，金額 NT$#{total_amount}。可至發票中心查看或下載。',
  '您有一張收據已開立，請至發票中心查看。',
  'INVOICE_RECEIPT_ISSUED',
  '{
     "refType":     {"const": "INVOICE"},
     "refId":       {"field": "invoice_document_id"},
     "documentId":  {"field": "invoice_document_id"},
     "receiptNo":   {"field": "invoice_no"},
     "issuedDate":  {"field": "invoice_date"},
     "totalAmount": {"field": "total_amount"},
     "orderNo":     {"field": "order_no"}
   }'::jsonb,
  'https://example.com/account/invoices/{documentId}', '#{notification_ref_id}',
  true, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #8  invoice.operation.review_required → seller（P7）  ⛔ disabled：原因 (A)
--     解除時 golden 需含：notification_ref_id / invoice_document_id /
--     notify_seller_account_id（58 §6.3.3.3 P7 差異欄 review_reason_code 本列不進文案：
--     它是 CodeMeta InvoiceManualReason 的**碼**，直接顯示不是人話；要顯示人話由前端
--     用 codeLabel 查表，最小材料放在 data 內）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.operation.review_required',
  'invoice.operation.review_required.seller', 'seller', 'notify_seller_account_id',
  'system',
  '開票作業需人工處理',
  '有一筆開票作業需要人工處理，請至賣家中心的發票管理查看處理方式。',
  '有一筆開票作業需要人工處理，請至賣家中心的發票管理查看。',
  'INVOICE_OPERATION_REVIEW_REQUIRED',
  '{
     "refType":          {"const": "INVOICE"},
     "refId":            {"field": "invoice_document_id"},
     "documentId":       {"field": "invoice_document_id"},
     "reviewReasonCode": {"field": "review_reason_code"}
   }'::jsonb,
  'https://example.com/seller-center/invoices/{documentId}', '#{notification_ref_id}',
  false, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #9  invoice.complaint.created → seller（P8）  ✅ enabled（原因 (B) 已解除）
--     刻意無 refType/refId：申訴的主體是**申訴**不是文件，而 refTypeDomain 裡沒有
--     描述得了申訴的值 ⇒ 靜態列表頁（58 §D.4 第 9 列）。已登記 deliberatelyNoRefContract。
--     文案不放買家填寫的自由文字（可能含個資與情緒性內容，36 號 payload 邊界）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.complaint.created',
  'invoice.complaint.created.seller', 'seller', 'notify_seller_account_id',
  'system',
  '買家提出發票申訴',
  '您收到一件新的發票申訴，請於 3 個工作天內回覆；逾期未回覆將列入違規計數。',
  '您收到一件新的發票申訴，請至賣家中心的「發票申訴」處理。',
  'INVOICE_COMPLAINT_CREATED',
  '{
     "complaintId": {"field": "complaint_id"},
     "orderNo":     {"field": "order_no"},
     "slaDueAt":    {"field": "sla_due_at"}
   }'::jsonb,
  'https://example.com/seller-center/invoices/appeals', '#{notification_ref_id}',
  true, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #10 invoice.complaint.responded → buyer（P9）  ✅ enabled
--     文案不放 action 的碼值（InvoiceComplaintAction 是 CodeMeta 碼，直接顯示不是人話）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.complaint.responded',
  'invoice.complaint.responded.buyer', 'buyer', 'buyer_id',
  'order_update',
  '賣家已回覆您的發票申訴',
  '您提出的發票申訴，賣家已回覆，請至發票中心查看處理結果。',
  '您提出的發票申訴已有回覆，請至發票中心查看。',
  'INVOICE_COMPLAINT_RESPONDED',
  '{
     "refType":     {"const": "INVOICE"},
     "refId":       {"field": "invoice_document_id"},
     "documentId":  {"field": "invoice_document_id"},
     "complaintId": {"field": "complaint_id"},
     "action":      {"field": "action"},
     "status":      {"field": "status"},
     "orderNo":     {"field": "order_no"}
   }'::jsonb,
  'https://example.com/account/invoices/{documentId}', '#{notification_ref_id}',
  true, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #11 invoice.complaint.overdue → seller（P10）  ⛔ disabled：原因 (A)
--     刻意無 refType/refId（同 #9，申訴列表）。已登記 deliberatelyNoRefContract。
--     解除時 golden 需含：notification_ref_id / notify_seller_account_id
--     （58 §6.3.3.3 P10 另帶 overdue_hours，本列文案刻意不用它 —— 少一個必填 key，
--       解除條件就少一個可能對不上的地方）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.complaint.overdue',
  'invoice.complaint.overdue.seller', 'seller', 'notify_seller_account_id',
  'system',
  '發票申訴已逾期未回覆',
  '有一件發票申訴已超過 3 個工作天未回覆，請盡快至賣家中心的「發票申訴」處理。',
  '有一件發票申訴已逾期未回覆，請至賣家中心的「發票申訴」處理。',
  'INVOICE_COMPLAINT_OVERDUE',
  NULL,
  'https://example.com/seller-center/invoices/appeals', '#{notification_ref_id}',
  false, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #12 invoice.seller_tax_profile.status_changed → seller（P11）  ✅ enabled（原因 (C) 已解除）
--     data_fields = NULL（58 §6.3.9）：賣家稅務身分不是 refTypeDomain 裡任何一個值
--     描述得了的資源；先例 delegation.operation.completed.seller 同樣是 NULL。
--     schema_url 因此必須是**靜態**網址（NULL 的 data_fields 產不出任何 brace 鍵）。
--     已登記 deliberatelyNoRefContract。
--     P11 是唯一沒有 order_no 與買家身分欄的事件（58 §6.3.3.3）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.seller_tax_profile.status_changed',
  'invoice.seller_tax_profile.status_changed.seller', 'seller', 'notify_seller_account_id',
  'system',
  '稅務身分審核狀態更新',
  '您的發票稅務身分審核狀態已更新，請至賣家中心的「稅務資格」頁查看。',
  '您的發票稅務身分審核狀態已更新，請至賣家中心查看。',
  'INVOICE_SELLER_TAX_PROFILE_STATUS_CHANGED',
  NULL,
  'https://example.com/seller-center/invoices/tax-status', '#{notification_ref_id}',
  true, 'visible'
),
-- ───────────────────────────────────────────────────────────────────────────
-- #13 invoice.resolved_externally → buyer（P12）  ✅ enabled
--     🔴 refType = ORDER 而不是 INVOICE：賣家在站外自行開立 ⇒ 我方**沒有**
--        invoice_documents 列可指（58 §D.4 第 13 列）。
--     🔴 wire 欄名是 external_invoice_no 不是 invoice_no（58 §6.3.3.4）。
--     🔴 文案不得寫成「平台已開立」（58 §6.3.9 硬約束）。
--     賣家**不發**這一則：他是動作發起人（58 §6.3.3.1 P12）。
-- ───────────────────────────────────────────────────────────────────────────
(
  'hawkr.invoice', 'invoice.resolved_externally',
  'invoice.resolved_externally.buyer', 'buyer', 'buyer_id',
  'order_update',
  '賣家已自行開立發票',
  '賣家已為訂單 #{order_no} 自行開立發票，發票號碼 #{external_invoice_no}，由賣家開立與寄送。',
  '賣家已自行開立您這筆訂單的發票，由賣家開立與寄送。',
  'INVOICE_RESOLVED_EXTERNALLY',
  '{
     "refType":             {"const": "ORDER"},
     "refId":               {"field": "order_no"},
     "orderNo":             {"field": "order_no"},
     "externalInvoiceNo":   {"field": "external_invoice_no"},
     "externalInvoiceDate": {"field": "external_invoice_date"}
   }'::jsonb,
  'https://example.com/account/orders/{orderNo}', '#{notification_ref_id}',
  true, 'visible'
)
ON CONFLICT ON CONSTRAINT uni_notification_templates_event_user_type DO NOTHING;
