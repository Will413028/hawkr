-- invoice_documents｜發票／收據文件
-- SSOT：設計書 58 號 §5.1.5（DDL／欄名）＋ §5.2.1（候選查詢 body 的 SSOT）＋ §8.1.4(b)（越權述詞）
--
-- 本表承載兩組**獨立**的租約（§5.2.3 第 3、4 組），共用同一張表但欄位不同、worker 不同：
--   upload_recheck_next_at ── INV.rc 附加組，租約 120s，打 AMEGO /json/invoice_status（唯讀）
--   pdf_fetch_next_at      ── INV.pdf，租約 120s，invoice_file ＋ 下載 ＋ MED 上傳
-- PDF 刻意不與開票共用租約：50.2s 併進去會讓 INV.op 的下界逼近 110s，而開票是法定期限最緊的路徑。
--
-- 兩組的候選述詞都**正面列出**要處理的值。上傳輪詢用 (NULL, 1, 2, 3, 31, 32) 而不是
-- `IS DISTINCT FROM 99`：91 與 -1 是終態、99 是完成，否定述詞會讓它們永久滯留在候選集合，
-- 單調累積直到把 LIMIT 額度吃光，全程無錯誤無告警（§5.2.3 第 3 組）。
-- PDF 用 `pdf_object_key IS NULL` 而不是狀態否定式：已回填的列必須離開候選集合。

-- name: FetchDueUploadRechecks :many
-- 上傳狀態輪詢的候選（AMEGO 無 webhook，只能自己輪詢 /json/invoice_status）。
-- 述詞與 ClaimUploadRecheck 逐字鏡射；吃 idx_invdoc_upload_recheck。
SELECT * FROM invoice_documents
WHERE status = 'ISSUED'
  AND upload_recheck_next_at IS NOT NULL
  AND upload_recheck_next_at <= sqlc.arg(now)
  AND (provider_upload_status IS NULL OR provider_upload_status IN (1, 2, 3, 31, 32))
ORDER BY upload_recheck_next_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimUploadRecheck :execrows
-- 租約 120s（下界 10.3s ＝ 一次 invoice_status 10s ＋ DB 0.3s），基準逐筆在 claim 當下取。
-- 本句就地遞增 upload_recheck_attempts（§5.2.1 逐字）：次數用盡（>= 48）的處置是
--    upload_recheck_next_at = NULL ＋ 進 ADM 佇列，辨識鍵＝次數 ≥ 48 且租約欄為 NULL。
UPDATE invoice_documents
SET upload_recheck_next_at = sqlc.arg(lease_until),
    upload_recheck_attempts = upload_recheck_attempts + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND status = 'ISSUED'
  AND upload_recheck_next_at IS NOT NULL
  AND upload_recheck_next_at <= sqlc.arg(claimable_before)
  AND (provider_upload_status IS NULL OR provider_upload_status IN (1, 2, 3, 31, 32));

-- name: RecordUploadRecheckResult :execrows
-- 上傳輪詢查回來之後的處置。**每一種結果都要有出邊**，否則候選集合出不去（I-48）——
-- 五種結果共用這一句，差別只在呼叫端傳什麼（§5.2.1 的處置表）：
--   99（財政部已收訖）  ⇒ status=99、next_at=NULL。**不發任何事件**（§6.3.3.1 沒有這條 routing key，
--                          發到沒有 binding 的 key 會靜默消失、不進 DLQ、不報錯）
--   1/2/3/31/32（在途）  ⇒ 更新 status、next_at = now + 指數退避（上限 6 小時）
--   91（錯誤）           ⇒ status=91、next_at=NULL、寫 invoice_status_history；告警走 DB 不走 MQ
--   -1（NOT_FOUND）      ⇒ status=-1、next_at=NULL；**同時**建一列 operation_type='QUERY' 走對帳。
--                          絕不可把 -1 當成「這張票沒開成功」（實測是 false negative，照信會重複開立）
--   次數用盡（>= 48）    ⇒ next_at=NULL，進 ADM 佇列
-- ⚠️ chk_invdoc_upload_recheck：還掛著租約 ⇒ provider_upload_status 必須仍在在途集合內。
--    寫 99／91／-1 時**必須同時**把 upload_recheck_next_at 設 NULL，否則當場 23514。
UPDATE invoice_documents
SET provider_upload_status = sqlc.narg(provider_upload_status),
    provider_upload_checked_at = sqlc.arg(now),
    upload_recheck_next_at = sqlc.narg(upload_recheck_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND status = 'ISSUED';

-- name: FetchDuePdfFetches :many
-- INV.pdf 的候選（走 pkg/outbox.Repository[T] 五方法契約的 FetchPendingDue）。
-- 述詞與 ClaimPdfFetch 逐字鏡射；吃 idx_invdoc_pdf_fetch。
SELECT * FROM invoice_documents
WHERE pdf_fetch_next_at IS NOT NULL
  AND pdf_object_key IS NULL
  AND pdf_fetch_next_at <= sqlc.arg(now)
ORDER BY pdf_fetch_next_at
LIMIT sqlc.arg(row_limit);

-- name: ClaimPdfFetch :execrows
-- 租約 120s（下界 50.2s ＝ invoice_file 10s ＋ 下載 20s ＋ MED 上傳 20s ＋ DB 0.2s），2.39× 餘裕。
UPDATE invoice_documents
SET pdf_fetch_next_at = sqlc.arg(lease_until),
    pdf_fetch_attempts = pdf_fetch_attempts + 1, updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND pdf_fetch_next_at IS NOT NULL
  AND pdf_object_key IS NULL
  AND pdf_fetch_next_at <= sqlc.arg(claimable_before);

-- name: MarkPdfFetchDone :execrows
-- Repository[T].MarkDone。回填 objectKey 是離開候選集合的第一種方式（另兩種見 MarkPdfFetchStopped）。
-- 只存 objectKey，禁 URL／presigned／JWT（§25 多媒體規範）。
UPDATE invoice_documents
SET pdf_object_key = sqlc.arg(pdf_object_key),
    pdf_fetch_next_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND pdf_object_key IS NULL;

-- name: MarkPdfFetchStopped :execrows
-- Repository[T].MarkFailed。離開候選集合的另兩種方式：載具發票無 PDF 可下載／次數用盡。
-- 一律 pdf_fetch_next_at = NULL。本表沒有 pdf 專用的 last_error 欄 ⇒ 失敗原因寫
--    invoice_status_history（§5.1.13），不塞進本句。
UPDATE invoice_documents
SET pdf_fetch_next_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND pdf_object_key IS NULL;

-- name: UpdatePdfFetchAttempt :exec
-- Repository[T].UpdateAttempt —— 退避重排，把「租約」換成「下次可執行時間」。
-- ⚠️ 只寫 pdf_fetch_next_at：attempts 已在 ClaimPdfFetch 就地遞增（§5.2.1 逐字），
--    這裡再寫一次會雙重計數 ⇒ repository adapter 收到框架給的 attemptCount 後丟棄不用。
UPDATE invoice_documents
SET pdf_fetch_next_at = sqlc.arg(next_attempt_at),
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: FetchApproachingStatutoryDeadlines :many
-- 法定存證期限逼近告警的資料面（《硬上限依賴接入治理規範》要求的「逼近上限」告警）。
-- 吃 idx_invdoc_deadline；四個狀態正面列出（與該索引述詞逐字相同）。
-- 消費者：Issuance.SweepOverdueStatutoryDeadlines（INV.dl 的 tick，5 分鐘一次）。
--    設計書 §5.2.1 的「過期者標 AWAITING_MANUAL ＋ manual_reason_code='DEADLINE_MISSED'」
--    已於 2026-09-08 接上（在那之前本方法零呼叫端、DEADLINE_MISSED 零生產者，
--    而這段註解逐字寫著「那支掃描不存在」——它在掃描接上的那一刻變成假的）。
-- ⚠️ 本查詢回四個狀態，而掃描**只碰 PENDING 與 ISSUE_FAILED**：
--    ISSUING 代表正在送 AMEGO（逾期了也該等它跑完，MarkAwaitingManual 的 allowed-from
--    天然擋住）、AWAITING_MANUAL 已經在目的地。改這裡的狀態清單前先讀那支掃描——
--    它的過濾是**額外的一層**，不是本查詢的鏡像。
-- ⚠️ 這個索引述詞同時餵 M7 的逼近告警（threshold = now+6h）與逾期掃描
--    （threshold = now）。兩個呼叫端差別只在 threshold，別把過濾寫進 SQL。
SELECT * FROM invoice_documents
WHERE status IN ('PENDING', 'ISSUING', 'ISSUE_FAILED', 'AWAITING_MANUAL')
  AND statutory_deadline_at IS NOT NULL
  AND statutory_deadline_at <= sqlc.arg(threshold)
ORDER BY statutory_deadline_at
LIMIT sqlc.arg(row_limit);

-- name: GetInvoiceDocumentByID :one
SELECT * FROM invoice_documents WHERE id = sqlc.arg(id);

-- name: GetInvoiceDocumentForBuyer :one
-- Tier A 越權：buyer_id 是查詢述詞的一部分，不是查回來之後才比對的欄位（§8.1.4(b)）。
-- 「先 SELECT 再 if doc.BuyerID != caller { return Forbidden }」有兩個問題：
--   ① 很容易在中途某個錯誤分支回出不同的 code／message，越權與不存在就可分辨了
--   ② 兩者的回應時間不同（查得到 vs 查不到），構成時間側信道
-- 三個欄名全部取自 §5.1.5：
--   ① 訂單編號在 source_ref（且必須同時限定 source_type='ORDER'，MANUAL 文件的 source_ref 是 NULL）
--   ② 買家在 (recipient_type, recipient_ref_id) 這一對上；BUYER ⇒ recipient_ref_id = buyers.buyer_id
--   ③ invoice_documents 沒有 deleted_at（全書只有 buyer_invoice_preferences 有 soft delete）
SELECT * FROM invoice_documents
WHERE source_type = 'ORDER' AND source_ref = $1
  AND recipient_type = 'BUYER' AND recipient_ref_id = $2;

-- name: GetInvoiceDocumentBySourceRef :one
-- 反向認領與退款事件用 order_no 找文件（吃 idx_invdoc_source_ref）。
-- ⚠️ 連結欄名不是 order_no：invoice_documents.source_ref（source_type='ORDER' 時 ＝ orders.order_no）
--    才是那條連結；invoice_operations 完全沒有訂單欄，operation 的擁有者判定一律經本表 join。
-- ⚠️ **id 是必要的 tie-breaker 不是裝飾**（#3087）：created_at 由**應用層時鐘**寫入，
--    同一次呼叫內建立的兩列（作廢重開就是這個形狀：原票轉 SUPERSEDED ＋ 建替代票）
--    會拿到**完全相同**的值 ⇒ 只有 created_at 的排序不穩定，`:one` 取到哪一列由
--    planner 決定、零錯誤零 log。id 是 TSID（遞增），它讓排序全序。
SELECT * FROM invoice_documents
WHERE source_type = 'ORDER' AND source_ref = sqlc.arg(source_ref)
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- ===========================================================================
-- M1 發票文件狀態機（§7.3.1，17 條轉移）的寫入面。
--
-- 三件貫穿本區塊的事：
--   ① §7.3.1 轉換表的「來源狀態」欄就是 allowed-from 的 SSOT。漏一個起點 ＝ rows=0
--      靜默失敗：不拋例外、不進 DLQ、不告警，該筆永遠停在原狀態（§7.3.0 規則 4）。
--   ② 本表有四條**雙向等式** CHECK，它們決定「哪些欄位必須跟狀態同一句寫」：
--        chk_invdoc_voided_pair    (status='VOIDED')            = (voided_at IS NOT NULL)
--        chk_invdoc_na_reason      (status='NOT_APPLICABLE')    = (not_applicable_reason IS NOT NULL)
--        chk_invdoc_external_no    (status='RESOLVED_EXTERNALLY') = 外部三欄各自非空
--        chk_invdoc_carrier_pair   (carrier_type IS NULL)       = (carrier_id1_ciphertext IS NULL)
--      雙向 ⇒ **離開**那個狀態時也要把欄位清掉，不是只有進入時要寫。少做一半當場 23514。
--   ③ 狀態轉移一律 version = version + 1（§5.3.6）；只有排程 claim 例外，而 M1 的兩組 claim
--      （ClaimPdfFetch／ClaimUploadRecheck）已在本檔上半部，都不碰 version。
-- ===========================================================================

-- name: InsertInvoiceDocument :one
-- M1 的兩條建立邊（`[*]` → PENDING／`[*]` → NOT_APPLICABLE）共用一句：差別只在 status 與
-- not_applicable_reason，而 chk_invdoc_na_reason 是雙向等式 ⇒ 傳錯組合當場 23514，
-- 不可能靜默寫進一列「NOT_APPLICABLE 但說不出原因」或「PENDING 卻帶著原因」。
-- ⚠️ **不發任何事件**（§6.3 沒有 invoice.document.created）——建立不是對外可見的業務事實，
--    買家端由 B04 讀 gRPC 呈現。
-- ⚠️ uq_invdoc_order_identity（issuer_id, source_ref, document_kind, snapshot_version,
--    issue_attempt_seq）WHERE source_type='ORDER' 是 I-13。ON CONFLICT DO NOTHING **不指名
--    target**：本表有四個 partial unique，指名其一 ⇒ 另三個衝突時 23505 並讓整個交易
--    aborted（25P02）。rows=0 ⇒ 這份文件已建過（重複消費 order.paid），呼叫端改讀既有列。
-- ⚠️ snapshot_version 唯一的合法來源是 order-service 的回傳值（I-50）：本檔沒有、也不得新增
--    任何遞增它的 UPDATE。
INSERT INTO invoice_documents (
  id, issuer_id, issuer_name_snapshot, issuer_tax_id_snapshot,
  source_type, source_ref, trigger_type,
  recipient_type, recipient_ref_id, recipient_name, recipient_tax_id,
  recipient_email_ciphertext, recipient_email_hash, recipient_phone_ciphertext,
  carrier_type, carrier_id1_ciphertext, carrier_id2_ciphertext, carrier_value_hash, donation_code,
  document_kind, fulfillment_mode, status, not_applicable_reason,
  snapshot_version, issue_attempt_seq, provider_account_version, original_document_id,
  sales_amount, free_tax_sales_amount, zero_tax_sales_amount, tax_amount, total_amount,
  tax_type, tax_rate_bps,
  provider, provider_request_no, receipt_no, receipt_authorization_version,
  invoice_date_taipei, tax_period_code, issue_not_before, paid_at, statutory_deadline_at,
  source_snapshot, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(issuer_id), sqlc.arg(issuer_name_snapshot), sqlc.narg(issuer_tax_id_snapshot),
  sqlc.arg(source_type), sqlc.narg(source_ref), sqlc.arg(trigger_type),
  sqlc.arg(recipient_type), sqlc.narg(recipient_ref_id), sqlc.arg(recipient_name), sqlc.arg(recipient_tax_id),
  sqlc.narg(recipient_email_ciphertext), sqlc.narg(recipient_email_hash), sqlc.narg(recipient_phone_ciphertext),
  sqlc.narg(carrier_type), sqlc.narg(carrier_id1_ciphertext), sqlc.narg(carrier_id2_ciphertext),
  sqlc.narg(carrier_value_hash), sqlc.narg(donation_code),
  sqlc.arg(document_kind), sqlc.arg(fulfillment_mode), sqlc.arg(status), sqlc.narg(not_applicable_reason),
  sqlc.arg(snapshot_version), sqlc.arg(issue_attempt_seq), sqlc.narg(provider_account_version),
  sqlc.narg(original_document_id),
  sqlc.arg(sales_amount), sqlc.arg(free_tax_sales_amount), sqlc.arg(zero_tax_sales_amount),
  sqlc.arg(tax_amount), sqlc.arg(total_amount),
  sqlc.arg(tax_type), sqlc.arg(tax_rate_bps),
  sqlc.narg(provider), sqlc.narg(provider_request_no), sqlc.narg(receipt_no),
  sqlc.narg(receipt_authorization_version),
  sqlc.narg(invoice_date_taipei), sqlc.narg(tax_period_code),
  sqlc.arg(issue_not_before), sqlc.narg(paid_at), sqlc.narg(statutory_deadline_at),
  sqlc.narg(source_snapshot), sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: MarkInvoiceDocumentIssuing :execrows
-- 兩條邊：PENDING → ISSUING（ISSUE operation READY→SENDING 的同一事務內連動）與
--        ISSUE_FAILED → ISSUING（ADM／worker 重試，operation 已回 READY）。
-- 兩者 SET 形狀相同、都由 operation 側的轉移驅動 ⇒ 共用一句，allowed-from 逐字等於那兩列。
UPDATE invoice_documents
SET status = 'ISSUING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'ISSUE_FAILED');

-- name: MarkInvoiceDocumentIssued :execrows
-- ISSUING → ISSUED（f0401 code=0，或 invoice_query 回填）。
-- chk_invdoc_issued_has_no（I-16）：AMEGO_AUTO ＋ ISSUED ⇒ provider_invoice_no 不得為空 ⇒
--   本句必須在同一句寫進號碼，分兩句寫第一句就 23514。
-- 副作用面：`pdf_fetch_next_at = now` 是 PDF 抓取候選集合的**入口**（I-56 的正向），
--   `upload_recheck_next_at = now` 是上傳輪詢候選集合的入口（I-48 的正向）。
--   兩者由呼叫端傳入而非寫死 now()：載具發票沒有 PDF 可下載 ⇒ 傳 NULL，該列一開始就不進候選。
-- ⚠️ 此時 provider_upload_status 仍為 NULL，chk_invdoc_upload_recheck 的
--   `provider_upload_status IS NULL` 分支放行；之後寫 99／91／-1 時**必須**同時把
--   upload_recheck_next_at 清成 NULL（RecordUploadRecheckResult 的責任）。
UPDATE invoice_documents
SET status = 'ISSUED',
    -- 🔴 開出來了就要把「為什麼卡住」清掉（2026-09-05 補）。
    -- 在 MarkInvoiceDocumentIssueFailed 開始寫 manual_reason_code 之前，本欄只有
    -- AWAITING_MANUAL 會設，而離開它的唯一路徑 ResumeInvoiceDocumentFromManual
    -- 已經清了 ⇒ 到得了 ISSUED 時它必然是 NULL，不清也沒事。
    -- 現在 ISSUE_FAILED 也會寫它，而 J18／J19（ADM 重試）與 J33（字軌恢復自動回 READY）
    -- **都不經過那三個清除者** ⇒ 不清的話，一張重試後成功的發票會永遠掛著上一次的
    -- 失敗理由，賣家中心與 ADM 都會在一個 SUCCEEDED 的作業旁邊顯示「原因：賣家字軌不足」。
    manual_reason_code = NULL,
    provider = sqlc.arg(provider),
    provider_invoice_no = sqlc.arg(provider_invoice_no),
    provider_random_code = sqlc.narg(provider_random_code),
    provider_issued_at = sqlc.narg(provider_issued_at),
    invoice_date_taipei = sqlc.narg(invoice_date_taipei),
    tax_period_code = sqlc.narg(tax_period_code),
    pdf_fetch_next_at = sqlc.narg(pdf_fetch_next_at),
    upload_recheck_next_at = sqlc.narg(upload_recheck_next_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING';

-- name: MarkInvoiceDocumentIssueFailed :execrows
-- ISSUING → ISSUE_FAILED（operation 進入 FAILED_PERMANENT／FAILED_RETRY_EXHAUSTED／
-- FAILED_UNKNOWN／**SELLER_ACTION_REQUIRED**）。
-- ⚠️ 第四個來源不叫 FAILED_*，所以呼叫端若把觸發條件寫成「任一 FAILED_*」會漏掉它——
--    漏掉的後果是 document 永遠停在 ISSUING，賣家 S08 看到「開立中」而系統早已停手在等他
--    去財政部配字軌（§7.3.1 該列的紅字）。那個判定在 usecase，本句只負責落地。
-- **document 不自己發事件**：事件由對應的 operation 轉移發（J11／J14／J32 ⇒ P2；J17 ⇒ P7），
-- 同一件事只有一個 producer。
--
-- 🔴 `manual_reason_code` **必須在這一句寫**（2026-09-05 補）：三個 settle* 都算好了
-- CodeMeta 的理由碼傳進來，而在此之前它只落 `invoice_status_history.reason_code`
-- ——那張表不在任何賣家可見的 DTO 裡 ⇒ `InvoiceJobInfo.manual_reason_code` 對
-- 這條路**恆為空**，而 `invoice.issue_failed.seller` 的通知逐字叫賣家
-- 「至賣家中心的『發票異常作業』**查看原因**並處理」。
-- 最嚴重的是 `SELLER_TRACK_INSUFFICIENT`（3040111）：它的處置與其他失敗相反
-- （重試無效、要去財政部配號），看不到它的賣家只會反覆按重試燒掉 attempt。
-- ⚠️ 本欄先前的唯一寫入者是 MarkInvoiceDocumentAwaitingManual ⇒ 語意是
-- 「為什麼需要人工」；ISSUE_FAILED 同樣是「卡住了、要有人做事」，兩者同義。
-- `chk_invdoc_manual_reason` 只約束 `AWAITING_MANUAL ⇒ 非空`，反向不管，本句合法。
UPDATE invoice_documents
SET status = 'ISSUE_FAILED',
    manual_reason_code = sqlc.arg(manual_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING';

-- name: MarkInvoiceDocumentPendingRetry :execrows
-- ISSUING → PENDING（operation 退回 RETRY_SCHEDULED）。
-- 與 ResumeInvoiceDocumentFromManual 分開：那一條的來源是 AWAITING_MANUAL 且要清
-- manual_reason_code，合併會讓 worker 的退避路徑意外清掉 ADM 還沒處理完的人工理由碼。
UPDATE invoice_documents
SET status = 'PENDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING';

-- name: MarkInvoiceDocumentAwaitingManual :execrows
-- PENDING／ISSUE_FAILED → AWAITING_MANUAL（fail-closed 偵測命中，或 ADM 判定需人工判斷）。
-- chk_invdoc_manual_reason（I-19）：AWAITING_MANUAL ⇒ manual_reason_code 不得為空 ⇒
-- 理由碼與狀態必須同一句寫。它同時是 ADM NEEDS_MANUAL 工作台直接顯示的那個值。
UPDATE invoice_documents
SET status = 'AWAITING_MANUAL',
    manual_reason_code = sqlc.arg(manual_reason_code),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'ISSUE_FAILED');

-- name: ResumeInvoiceDocumentFromManual :execrows
-- AWAITING_MANUAL → PENDING（ADM 給出判定，回自動線）。expectedVersion CAS ＋ 必填理由。
-- 清空 manual_reason_code 是狀態語意的一部分：留著會讓 ADM 工作台在下一次進 AWAITING_MANUAL
-- 時顯示上一輪的舊理由。
UPDATE invoice_documents
SET status = 'PENDING',
    manual_reason_code = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'AWAITING_MANUAL'
  AND version = sqlc.arg(expected_version);

-- name: MarkInvoiceDocumentResolvedExternally :execrows
-- ISSUE_FAILED／AWAITING_MANUAL → RESOLVED_EXTERNALLY（賣家回報自行開立號碼）。
-- chk_invdoc_external_no（I-53）是**三條**雙向等式：external_invoice_no／external_invoice_date／
--   external_reported_at 必須與狀態同進同出 ⇒ 只寫號碼當場 23514。
-- chk_invdoc_external_excl：provider_invoice_no 必須保持 NULL——**平台不冒充開立人**，
--   而且兩欄永不互相 fallback。
-- chk_invdoc_external_no_shape：2 英文 + 8 數字（格式在 DB 層守，不只在端點）。
-- chk_invdoc_external_reported_after：external_reported_at >= issue_not_before。
--
-- 🔴 **allowed-from 含 NOT_APPLICABLE，但只限兩個 mode**（2026-09-05，DEC-20260905-221900 選 A）。
--    稅務資格尚未核准的賣家（SELF_ISSUE_PENDING 與更早階段）的每一筆訂單都落
--    fulfillment_mode ∈ {SELLER_SELF_ISSUE, LEGACY_MANUAL} ＋ status='NOT_APPLICABLE'，
--    而 chk_invdoc_self_issue 逐字允許這兩個 mode 走到 RESOLVED_EXTERNALLY、
--    CodeMeta 與設計書 §7.3.1 也都這樣寫 —— 在此之前 allowed-from 到不了，
--    賣家自行開了票卻回報不了，端點 62 回 9349「請等待平台通知」而沒有人會通知（#3166）。
--
-- 🔴 **mode 條件不可省**：NOT_APPLICABLE 也涵蓋 AMEGO_AUTO 底下的情境（零元訂單、
--    切點前訂單）。少了它，那些訂單也會開放賣家回報一個發票號碼，而它們本來就不該有。
--
-- 🔴 **not_applicable_reason 必須同一句清成 NULL**：chk_invdoc_na_reason 是雙向等式
--    (status='NOT_APPLICABLE') = (not_applicable_reason IS NOT NULL) ⇒ 只改 status
--    不清 reason 當場 23514。原本這一句不需要清它，因為它從不從 NOT_APPLICABLE 出發。
UPDATE invoice_documents
SET status = 'RESOLVED_EXTERNALLY',
    external_invoice_no = sqlc.arg(external_invoice_no),
    external_invoice_date = sqlc.arg(external_invoice_date),
    external_reported_at = sqlc.arg(external_reported_at),
    manual_reason_code = NULL,
    not_applicable_reason = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND (
    status IN ('ISSUE_FAILED', 'AWAITING_MANUAL')
    OR (status = 'NOT_APPLICABLE'
        AND fulfillment_mode IN ('SELLER_SELF_ISSUE', 'LEGACY_MANUAL'))
  );

-- name: MarkInvoiceDocumentCancelledBeforeIssue :execrows
-- PENDING → CANCELLED_BEFORE_ISSUE（累計退款 = 實付總額）與
-- AWAITING_MANUAL → CANCELLED_BEFORE_ISSUE（ADM 確認本單不需開立，雙人確認 ＋ 必填理由）。
-- 兩條邊 SET 形狀相同 ⇒ 共用；守衛差異（前者由退款金額判定、後者由 ADM 二次確認）在 usecase。
-- ⚠️ **不發事件**（§6.3 沒有 invoice.document.cancelled）；買家端由 B04 讀 gRPC 呈現。
-- 清 manual_reason_code：離開 AWAITING_MANUAL 之後那個值就不再有讀者。
UPDATE invoice_documents
SET status = 'CANCELLED_BEFORE_ISSUE',
    cancel_reason_code = sqlc.arg(cancel_reason_code),
    cancel_reason = sqlc.narg(cancel_reason),
    manual_reason_code = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('PENDING', 'AWAITING_MANUAL');

-- name: MarkInvoiceDocumentVoiding :execrows
-- ISSUED → VOIDING（VOID operation READY→SENDING）。
-- 守衛「本 document 無生效 ALLOWANCE；invoice_status = 99」在 usecase（前者要看
-- invoice_adjustments、後者是 provider 側事實），本句只守狀態。
UPDATE invoice_documents
SET status = 'VOIDING',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUED';

-- name: MarkInvoiceDocumentVoided :execrows
-- VOIDING → VOIDED（invoice_query 確認 invoice_type = C0501）。
-- chk_invdoc_voided_pair 是雙向等式 ⇒ voided_at 必須與狀態同一句寫。
-- ⚠️ VOIDED **不是終態**：轉換表最後一列有 VOIDED → SUPERSEDED（作廢後重開）。
UPDATE invoice_documents
SET status = 'VOIDED',
    voided_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'VOIDING';

-- name: RestoreInvoiceDocumentToIssued :execrows
-- VOIDING → ISSUED（f0501 回 3050121/3050123/3050126/3050141，作廢被 provider 拒絕）。
-- **買家手上那張憑證仍然有效**，只是內容改不了；同一事務內 correction_request 走 R05 → REJECTED。
-- 與 MarkInvoiceDocumentIssued 分開是必須的：那一句會覆寫 provider_invoice_no 等回填欄，
-- 而這條邊沒有任何 provider 回填，共用會把既有號碼蓋成呼叫端隨手傳的值。
UPDATE invoice_documents
SET status = 'ISSUED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'VOIDING';

-- name: MarkInvoiceDocumentSuperseded :execrows
-- VOIDED／ISSUED → SUPERSEDED（重開文件已 ISSUED；後者是作廢確認遲到的補正路徑）。
-- 守衛 superseded_by 非空且該列 status='ISSUED' 在 usecase（跨列，SQL 這一側只能守自己）。
-- 🔴 `voided_at = NULL` 不是筆誤：chk_invdoc_voided_pair 是**雙向**等式
--    (status='VOIDED') = (voided_at IS NOT NULL) ⇒ 從 VOIDED 轉出時若不清空，這一句每次都
--    以 23514 失敗。作廢時點的稽核由 invoice_status_history（subject_type='DOCUMENT'）承載
--    ——那正是 §7.3.0 規則 2「status 是當前狀態，歷程由 history 承載」的實例。
UPDATE invoice_documents
SET status = 'SUPERSEDED',
    superseded_by_id = sqlc.arg(superseded_by_id),
    voided_at = NULL,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('VOIDED', 'ISSUED');

-- name: AddInvoiceDocumentAllowanceTotal :execrows
-- I4／I-07：累計折讓金額。**單句 CAS，不可用「先查再寫」**——先查再寫在兩個副本同時處理
-- 兩筆折讓時會雙雙讀到舊值、雙雙通過上限檢查。
-- 上限由 chk_invdoc_allowance_cap（0 ≤ allowance_total_amount ≤ total_amount）守，
-- 述詞裡的 `+ delta <= total_amount` 是為了讓超額回 **rows=0**（呼叫端可翻成業務碼）而不是
-- 23514（整個交易 aborted，25P02 之後每一句都被拒）。兩層都要，不是二選一。
-- ⚠️ 刻意**不用 trigger**：本 repo 所有 Go 服務的 Atlas schema 零 trigger、零自訂函式，
--    為一條約束開先例會讓 atlas migrate diff 的行為變成新的未知（§5.3.6）。
-- 必須與 invoice_adjustments 的 → COMPLETED 轉換在**同一交易內**（I-08 的單一寫入路徑）。
UPDATE invoice_documents
SET allowance_total_amount = allowance_total_amount + sqlc.arg(delta_amount),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND allowance_total_amount + sqlc.arg(delta_amount) <= total_amount
  AND allowance_total_amount + sqlc.arg(delta_amount) >= 0;

-- name: NextReceiptNo :one
-- §5.3.3：平台代製收據的號碼。與折讓單號同一組理由（見 NextAllowanceNumber），
-- 由 chk_invdoc_receipt 的雙向等式要求 RECEIPT 文件必填。
-- ⚠️ 外層的 ::text 不可省，理由同 NextAllowanceNumber。
SELECT ('R' || lpad(nextval('invoice_receipt_number_seq')::text, 12, '0'))::text AS receipt_no;

-- ===========================================================================
-- 讀取視圖（分頁列表 ＋ owner-scoped 明細）
--
-- 三條貫穿本區塊的規則：
--
--   ① **擁有者一律寫進 WHERE**（§8.1.4(b) Tier A）。買家用
--      `recipient_type='BUYER' AND recipient_ref_id`、賣家銷項用
--      `invoice_issuers.issuer_ref_id`（`issuer_type='SELLER'` 時 ＝ store_id）、
--      賣家進項用 `recipient_type='SELLER' AND recipient_ref_id`。
--      查無與越權在這一層本來就長得一樣 ⇒ 清單回 0 列、單筆回 pgx.ErrNoRows，
--      **沒有第二個分支可以回不同的東西**。
--      ⚠️ 兩套述詞不得互相回退（§8.1.4(a)）：document-scoped 查不到就是查不到，
--         不得改用 order 述詞再查一次。
--
--   ② **每一支 List 都有一支述詞逐字相同的 Count**。兩份述詞漂移的症狀是
--      「總數說有 30 筆、翻到第二頁是空的」，而沒有任何東西會報錯——
--      `total` 與 `records` 分屬兩個母體，前端算出的頁數會多一頁或少一頁。
--      改任一支必須同批改另一支。
--
--   ③ **狀態值一律走參數**（禁硬編碼，CLAUDE.md §8）：`status_filter`／
--      `*_statuses::text[]` 由呼叫端從 CodeMeta 生成常數組出。
--      **結構鑑別欄例外**：`source_type='ORDER'`／`recipient_type='BUYER'`／
--      `issuer_type='SELLER'` 是資料模型的鑑別值不是業務狀態碼，沿用
--      GetInvoiceDocumentForBuyer 的既有寫法（§8.1.4(b) 逐字印出的那一段）。
--
-- 🔴 **開立日期軸 ＝ `COALESCE(provider_issued_at, created_at)`**（本檔唯一定義處）。
--    §6.1.10 只說「依 `issued_at` 的 UTC+8 日界」，而 `provider_issued_at` 對
--    RECEIPT／尚未開立／RESOLVED_EXTERNALLY 的文件都是 NULL ⇒ 單用它會讓那些
--    文件在**任何**帶日期的查詢下整批消失（不是報錯，是消失）。
--    `YYYY-MM-DD` → UTC 區間的換算在呼叫端做（它才知道 UTC+8 日界），本層只收
--    已換算好的 timestamptz；格式驗證同樣在上游一次做對（§6.1.10 的 AMEGO 實測坑）。
--    List 與 Count 必須用**同一個運算式**，排序鍵也是它。
-- ===========================================================================

-- name: ListBuyerInvoiceDocuments :many
-- 端點 68（B09 買家發票中心）。授權述詞 ＝ recipient_type='BUYER' AND recipient_ref_id，
-- **與訂單無關**——§2.3 方向三（平台→買家）的發票不掛在任何訂單底下，order-scoped
-- 的入口取不到它。吃 idx_invdoc_recipient。
SELECT * FROM invoice_documents
WHERE recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(provider_issued_at, created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(provider_issued_at, created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = ''        OR status        = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(document_kind_filter)::text = '' OR document_kind = sqlc.arg(document_kind_filter)::text)
ORDER BY COALESCE(provider_issued_at, created_at) DESC, id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountBuyerInvoiceDocuments :one
-- 端點 68 的 total。述詞與 ListBuyerInvoiceDocuments **逐字相同**（規則 ②）。
SELECT COUNT(*)::bigint FROM invoice_documents
WHERE recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(provider_issued_at, created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(provider_issued_at, created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = ''        OR status        = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(document_kind_filter)::text = '' OR document_kind = sqlc.arg(document_kind_filter)::text);

-- name: GetBuyerInvoiceDocumentByID :one
-- 端點 69／70／71（document-scoped）。**不帶 order_no**：方向三的發票沒有訂單，
-- 帶了就填不出來。授權述詞與端點 68 同一條。
SELECT * FROM invoice_documents
WHERE id = sqlc.arg(id)
  AND recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint;

-- name: ListOrderInvoiceDocumentsForBuyer :many
-- 端點 8（GetOrderInvoice）的 `documents[]`。
--
-- 🔴 **必須是 :many 不是 :one。** 一張訂單在 VOID_AND_REISSUE 之後會有多份文件
--    （舊的 status=SUPERSEDED 並帶 superseded_by_id，新的帶 original_document_id
--    指回鏈頭）；`issue_attempt_seq` 就是為此存在的實體軸。既有的
--    GetInvoiceDocumentForBuyer 是 `:one` ⇒ 同一組述詞命中兩列時 **pgx 取任意一列
--    而不報錯**（LIMIT 都沒有，順序未定義），症狀是買家看到作廢前的舊發票而畫面
--    完全正常。那一支保留給「只需要一份就夠」的既有呼叫端，本支給列表。
--
-- 排序：內容軸 → 實體軸 → id，讓最新一次開立排最前。
SELECT * FROM invoice_documents
WHERE source_type = 'ORDER'
  AND source_ref = sqlc.arg(source_ref)
  AND recipient_type = 'BUYER'
  AND recipient_ref_id = sqlc.arg(buyer_id)::bigint
ORDER BY snapshot_version DESC, issue_attempt_seq DESC, id DESC;

-- name: ListSellerInvoiceDocuments :many
-- 端點 30（S06 賣家銷項紀錄）。issuer-scoped：歸屬鍵是 invoice_issuers.issuer_ref_id
-- （`issuer_type='SELLER'` 時 ＝ store_id），**不是** invoice_documents 上的任何欄位
-- ——本表沒有 store_id 欄，一定要 join（§8.1.4(a)）。吃 idx_invdoc_issuer_status。
--
-- `latest_operation_id` 走 lateral 子查詢一次帶出（S06 第 8 欄「查看異常 → S08」
-- 的唯一 href 來源）：逐列再查一次就是 N+1，而 S06 預設 20 列。
-- 🔴 外層 `COALESCE(..., 0)::bigint` 不可省：沒有任何 operation 時子查詢回 NULL，
--    而 sqlc 對純量子查詢一律推成 non-null（`int64`）⇒ 掃描時當場
--    `cannot scan NULL into *int64`，且**只在「這份文件還沒有任何 operation」時發生**
--    ——happy path 全綠，第一份剛建立的文件進列表就炸。
--    哨兵值 0 ＝ 無 operation（TSID 恆 > 0），呼叫端據此翻成空字串（前端靠
--    「有 key 但值是空字串」與「後端還沒部署這一欄」區分，58A T06）。
--
-- `abnormal_only` 的值域由呼叫端從 CodeMeta 組出（58A S06 ⑥：ISSUE_FAILED／
-- AWAITING_MANUAL）。寫死在 SQL 會讓「異常」的定義出現第二份且無人維護。
SELECT sqlc.embed(d),
       COALESCE((SELECT o.id FROM invoice_operations o
                  WHERE o.document_id = d.id
                  ORDER BY o.created_at DESC, o.id DESC
                  LIMIT 1), 0)::bigint AS latest_operation_id
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(abnormal_only)::boolean
       OR d.status = ANY(sqlc.arg(abnormal_statuses)::text[]))
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountSellerInvoiceDocuments :one
-- 端點 30 的 total。述詞與 ListSellerInvoiceDocuments 逐字相同（規則 ②）；
-- 只少了 lateral 子查詢——它不在 WHERE 裡，不影響母體。
SELECT COUNT(*)::bigint
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (NOT sqlc.arg(abnormal_only)::boolean
       OR d.status = ANY(sqlc.arg(abnormal_statuses)::text[]));

-- name: GetSellerInvoiceDocumentByID :one
-- 端點 31／32（S07 明細、重寄）。issuer-scoped，形狀同上：越權與不存在皆 pgx.ErrNoRows
-- ⇒ 呼叫端一律翻 9301。
SELECT sqlc.embed(d),
       COALESCE((SELECT o.id FROM invoice_operations o
                  WHERE o.document_id = d.id
                  ORDER BY o.created_at DESC, o.id DESC
                  LIMIT 1), 0)::bigint AS latest_operation_id
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE d.id = sqlc.arg(id)
  AND i.issuer_type = 'SELLER'
  AND i.issuer_ref_id = sqlc.arg(store_id)::bigint;

-- name: ListRecentSellerInvoiceDocuments :many
-- 端點 16（S01 總覽）⑤「最近開立的發票與收據」。固定筆數、不接受任何 query
-- （要更多去端點 30）⇒ 刻意不與 ListSellerInvoiceDocuments 共用：那一支帶 9 個
-- 篩選參數，總覽為了呼叫它得傳一整組「不篩選」的空值，而任何一個傳錯都會靜默少列。
SELECT * FROM invoice_documents d
WHERE d.issuer_id = sqlc.arg(issuer_id)
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit);

-- name: ListSellerReceivedInvoiceDocuments :many
-- 端點 72（S12 賣家進項）。**與端點 30 是兩個資料集，不是同一支加 flag**：
-- 銷項可作廢可折讓、進項是唯讀報稅憑證，法律角色不同（proto 該 RPC 的註解）。
-- 授權述詞 ＝ recipient_type='SELLER' AND recipient_ref_id ＝ 當前 store_id
-- （§8.1.4(a) 「賣家 recipient 側」那一列）。吃 idx_invdoc_recipient。
--
-- `recipient_note`（用途說明）依 proto 定案「不加 DB 欄，由 invoice_document_lines
-- 推導」：本支回**兩個輸入**——line_seq 最小那一行的 description，與該文件的行數。
-- 「等 N 項」的字串組裝在呼叫端（那是呈現層的措辭，不是資料）。
-- ⚠️ `first_line_description` 同樣要 `COALESCE(..., '')::text`（理由同 latest_operation_id：
--    無明細行時子查詢回 NULL 而 sqlc 推成 `string`）。
-- ⚠️ 推不出值（line_count = 0 或描述為空字串）代表資料有問題，呼叫端**不得靜默略過**：
--    Phase 1 方向二／三／四只由端點 57 產生，而該路徑走 MANUAL_LINES ⇒ 明細行
--    必有 description ⇒ 推導必有值。
SELECT sqlc.embed(d),
       COALESCE((SELECT l.description FROM invoice_document_lines l
                  WHERE l.document_id = d.id
                  ORDER BY l.line_seq
                  LIMIT 1), '')::text AS first_line_description,
       (SELECT COUNT(*)::bigint FROM invoice_document_lines l
         WHERE l.document_id = d.id) AS line_count
FROM invoice_documents d
WHERE d.recipient_type = 'SELLER'
  AND d.recipient_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = '' OR d.status = sqlc.arg(status_filter)::text)
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountSellerReceivedInvoiceDocuments :one
-- 端點 72 的 total。述詞與 ListSellerReceivedInvoiceDocuments 逐字相同（規則 ②）。
SELECT COUNT(*)::bigint FROM invoice_documents d
WHERE d.recipient_type = 'SELLER'
  AND d.recipient_ref_id = sqlc.arg(store_id)::bigint
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
  AND (sqlc.arg(status_filter)::text = '' OR d.status = sqlc.arg(status_filter)::text);

-- name: ListInvoiceDocumentsForAdmin :many
-- 端點 47（A12）。**沒有擁有者述詞**——ADM 看全平台是設計如此（§8.1.4 的 Tier A
-- 只約束買賣家面）；`store_id` 在這裡是**篩選**不是授權，因此可以不給（＝全平台）。
-- 這也是為什麼它不能與端點 30 共用一支：那一支的 store_id 是必填的授權述詞，
-- 讓它變成可選會把賣家面的越權防線一起拆掉。
SELECT sqlc.embed(d),
       i.issuer_ref_id AS issuer_ref_id,
       i.issuer_type   AS issuer_type,
       COALESCE((SELECT o.id FROM invoice_operations o
                  WHERE o.document_id = d.id
                  ORDER BY o.created_at DESC, o.id DESC
                  LIMIT 1), 0)::bigint AS latest_operation_id
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz)
ORDER BY COALESCE(d.provider_issued_at, d.created_at) DESC, d.id DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountInvoiceDocumentsForAdmin :one
-- 端點 47 的 total。述詞與 ListInvoiceDocumentsForAdmin 逐字相同（規則 ②）。
SELECT COUNT(*)::bigint
FROM invoice_documents d
JOIN invoice_issuers i ON i.id = d.issuer_id
WHERE (NOT sqlc.arg(has_store_id)::boolean
       OR (i.issuer_type = 'SELLER' AND i.issuer_ref_id = sqlc.arg(store_id)::bigint))
  AND (sqlc.arg(order_no)::text = ''
       OR (d.source_type = 'ORDER' AND d.source_ref = sqlc.arg(order_no)::text))
  AND (sqlc.arg(status_filter)::text = '' OR d.status           = sqlc.arg(status_filter)::text)
  AND (sqlc.arg(mode_filter)::text = ''   OR d.fulfillment_mode = sqlc.arg(mode_filter)::text)
  AND (NOT sqlc.arg(has_date_from)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) >= sqlc.arg(date_from)::timestamptz)
  AND (NOT sqlc.arg(has_date_to)::boolean
       OR COALESCE(d.provider_issued_at, d.created_at) <= sqlc.arg(date_to)::timestamptz);

-- name: GetInvoiceDocumentByProviderRequestNo :one
-- 手動開票（端點 57）的業務冪等鍵查詢。
-- uq_invdoc_provider_request_no（provider_request_no）WHERE provider_request_no IS NOT NULL
--   是這條路徑上唯一擋得住重複開票的索引：invoice_documents 沒有 idempotency_key 欄，
--   而 uq_invdoc_order_identity 帶著 WHERE source_type='ORDER' ⇒ 對 MANUAL 的列完全不適用。
-- 🔴 呼叫端必須把「查不到」當成正常結果（可以建新的），不是錯誤：把它當錯誤往上拋，
--   第一次手動開票就會 500。
SELECT * FROM invoice_documents
WHERE provider_request_no = sqlc.arg(provider_request_no);

-- name: ClaimInvoiceNotificationResend :one
-- 端點 32／49／71 的重寄窗口（§6.1.10）：同一 document_id 每 10 分鐘 1 次。
--
-- 🔴 **窗口守衛就是這句的 WHERE**，沒有第二處。三個入口（賣家／ADM／買家）共用同一個
--    計數器（key ＝ document_id，不是 document_id × actor）——否則 ADM 幫忙點一次、
--    賣家再點一次，買家會在十分鐘內收到兩封。
--    無列命中 ＝ 還在窗口內 ⇒ 呼叫端回 **9352**（429），絕不當成功往下走。
-- ⚠️ **不動 version**：重寄不是狀態轉移，沒有人拿它做樂觀鎖。跟著 +1 的話，ADM 開著
--    A06 頁面時只要有人按了重寄，他的重試／取消就會被回 9343 而畫面上什麼都沒變。
-- ⚠️ window_start 由呼叫端算（now − 10 分鐘）而不是寫成 now() - interval：時間來源
--    必須是可注入的 clock，否則「窗口內」那一格沒有任何測試造得出來。
-- RETURNING 回**遞增後**的次數：它同時是 notification_ref_id 的去重種子
--    （notification-service 的冪等鍵含 reference_id ⇒ 沿用同一個 ref id 的第二封信
--    會被靜默吞掉，而按下去的人看到的是成功）。
UPDATE invoice_documents
SET notification_resent_at = sqlc.arg(now),
    notification_resend_count = notification_resend_count + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id)
  AND (notification_resent_at IS NULL OR notification_resent_at <= sqlc.arg(window_start))
RETURNING notification_resend_count;

-- name: ReviseInvoiceDocumentSnapshot :execrows
-- §7.3.5 R03（開立前直接改）：把買家更正後的收件人資料與新的 snapshot_version 寫回
-- **同一列**（不是建新列）。
--
-- 為什麼是就地更新而不是新列（五條依據，2026-08-31 判定）：
--   ① uq_invdoc_order_identity 的定位是「**自動線冪等鍵**」（§5.1.1 索引說明），
--      防的是 order.paid consumer 重投時建出第二份，不是宣告身分不可變
--   ② 該 consumer 讀 ORD.GetOrderInvoiceSnapshotInternal，而它 ORDER BY snapshot_version
--      DESC ⇒ 永遠拿最新版 ⇒ 更新後的重投會算出新版本、撞到本列、正確冪等
--   ③ 時序圖逐字「**重建** lines」——重建既有文件的明細
--   ④ invoice_correction_requests 有 reissued_document_id（R04 用），R03 無對應欄位
--      ⇒ 這條邊不產生新文件
--   ⑤ DeleteInvoiceDocumentLines 的述詞正面列出 PENDING／AWAITING_MANUAL，
--      「重建未送出文件的明細」是明文支援的動作
--
-- 🔴 述詞限定未送出的兩個狀態：一份已 ISSUED 的發票被就地換掉內容而號碼不變，
--    稅務上那是偽造（同 DeleteInvoiceDocumentLines 的理由，兩句必須同時成立）。
-- 🔴 snapshot_version **只能來自 order-service 的回傳值**（I-50）：本句是複製，
--    不是遞增——呼叫端不得自己 +1。
-- 🔴 本句的密文與雜湊參數由 **repository 層**算好再傳（同 InsertInvoiceDocument）：
--    金鑰環住在那一層，usecase 送進來的是明文。本檔任何 query 都只收密文與雜湊，
--    那是 §5.1.1 的 PII 邊界——邊界在 SQL 這一側，不在 usecase 那一側。
UPDATE invoice_documents
SET snapshot_version           = sqlc.arg(snapshot_version),
    -- 🔴 **document_kind 與金額四欄必須跟著新的買受人重算**（#3085／#3089 B 組 A-3）。
    --    在 2026-09-04 之前本句不 SET 它們，於是買家在一張 B2C 文件上補打統編時：
    --      recipient_tax_id 變成 '12345678'、document_kind 仍是 'B2C'
    --      ⇒ chk_invdoc_b2c（B2C ⇒ tax_amount=0 AND recipient_tax_id='0000000000'）
    --      當場 23514 ⇒ 買家看到 500。**而「補打統編」正是端點 11 的主要使用情境。**
    --    R04（作廢重開）2026-09-01 已因同一個症狀修好，R03 沒有回灌——同一個形狀
    --    在對稱的另一條路上原封不動地留著。
    document_kind              = sqlc.arg(document_kind),
    sales_amount               = sqlc.arg(sales_amount),
    free_tax_sales_amount      = sqlc.arg(free_tax_sales_amount),
    zero_tax_sales_amount      = sqlc.arg(zero_tax_sales_amount),
    tax_amount                 = sqlc.arg(tax_amount),
    total_amount               = sqlc.arg(total_amount),
    -- statutory_deadline_at 也跟著 kind 走：B2C 是 2 個自然日、B2B 是 7 個
    -- （chk_invdoc_deadline_presence 只要求「B2C／B2B ⇒ 非空」，所以不重算**不會**
    --  有任何東西轉紅——只會讓這批發票在錯的順序被處理、在錯的時點被告警）。
    statutory_deadline_at      = sqlc.narg(statutory_deadline_at),
    recipient_type             = sqlc.arg(recipient_type),
    recipient_ref_id           = sqlc.narg(recipient_ref_id),
    recipient_name             = sqlc.arg(recipient_name),
    recipient_tax_id           = sqlc.arg(recipient_tax_id),
    recipient_email_ciphertext = sqlc.narg(recipient_email_ciphertext),
    recipient_email_hash       = sqlc.narg(recipient_email_hash),
    carrier_type               = sqlc.narg(carrier_type),
    carrier_id1_ciphertext     = sqlc.narg(carrier_id1_ciphertext),
    carrier_id2_ciphertext     = sqlc.narg(carrier_id2_ciphertext),
    carrier_value_hash         = sqlc.narg(carrier_value_hash),
    donation_code              = sqlc.narg(donation_code),
    updated_at                 = sqlc.arg(now),
    version                    = version + 1
WHERE id = sqlc.arg(id)
  AND version = sqlc.arg(expected_version)
  AND status IN ('PENDING', 'AWAITING_MANUAL');

-- name: LockInvoiceDocumentForCorrection :one
-- R01（買家送出修改請求）的第一步：同一交易內鎖住該列再判定。
--
-- 為什麼需要顯式的 FOR UPDATE，而不是只靠兩個 partial unique：
--   那兩個索引擋的是「同時存在兩筆進行中的修改／調整」，擋不住
--   「讀到 snapshot_version=1、判定 expectedSnapshotVersion 相符、
--     期間 R03 把它推成 2、然後我用過期的判定結果建了 correction」。
--   expected_snapshot_version 一旦寫進 invoice_correction_requests 就是既成事實，
--   而它是後續 R03 送去 order-service 的樂觀鎖基準 ⇒ 錯了會在跨服務那一層才炸，
--   那時已經隔了一個 usecase 的距離。
--
-- ⚠️ 只鎖不改：本句不得帶任何 SET。它的作用是讓「讀 → 判定 → 建立」三步原子化。
SELECT * FROM invoice_documents
WHERE id = sqlc.arg(id)
FOR UPDATE;

-- name: MarkInvoiceDocumentReceiptIssued :execrows
-- ISSUING → ISSUED（平台代製收據產製完成，#3026）。**不經 provider**。
--
-- 🔴 與 MarkInvoiceDocumentIssued 分成兩句，不是重複：
--   chk_invdoc_receipt_no_provider（document_kind='RECEIPT' ⇒ provider 與
--   provider_invoice_no 都必須為 NULL）讓那一句在收據上**必定 23514**——它把
--   provider 寫成 'AMEGO'。共用一句的唯一寫法是讓 provider 變成可空參數，
--   而那會讓「AMEGO_AUTO ＋ ISSUED ⇒ provider_invoice_no 非空」（chk_invdoc_issued_has_no）
--   失去它在型別層的保證。
--
-- 🔴 pdf_object_key 在**同一句**寫入，不另開一句：收據的 PDF 是我方即時產出的，
--   沒有 INV.pdf 那種「先 ISSUED、之後再抓檔」的階段。分兩句寫的話，兩句之間
--   document 會處在「已開立但沒有檔案」的狀態，而 FileAvailability 對那個狀態
--   回的是 NOT_APPLICABLE ⇒ 買家在那個窗口按下下載會看到「無法下載」。
--
-- receipt_authorization_version 是賣家同意條款版本的快照（schema 註解逐字：
--   「開立時快照進 invoice_documents.receipt_authorization_version」）。
--   seller_tax_profiles 是就地更新、取消授權會把來源清成 NULL ⇒ 不在這裡快照就再也查不到。
--
-- document_kind = 'RECEIPT' 寫進 WHERE 而不只靠呼叫端：它是這句與上一句的分界，
--   放在述詞裡才擋得住「拿收據那句去寫一張 B2C」——那會產出一張沒有發票號碼的 ISSUED 發票。
UPDATE invoice_documents
SET status = 'ISSUED',
    receipt_authorization_version = sqlc.arg(receipt_authorization_version),
    pdf_object_key = sqlc.arg(pdf_object_key),
    provider_issued_at = sqlc.arg(provider_issued_at),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'ISSUING'
  AND document_kind = 'RECEIPT';
