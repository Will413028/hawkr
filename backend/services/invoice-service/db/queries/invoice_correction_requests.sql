-- invoice_correction_requests｜買家付款後修改發票資料
-- SSOT：設計書 58 號 §5.1.11＋§7 correction 狀態機
-- 本波只放骨架：修改請求的建立／婉拒／作廢重開在本波範圍外。
-- ⚠️ 補 query 時：uq_invcorr_open_per_document（REQUESTED／VOID_REISSUE_IN_PROGRESS）是
--    「同一張發票同時只能有一個進行中的修改請求」那把序列閘門（I5），寫入路徑不得繞過它。

-- name: GetInvoiceCorrectionRequestByID :one
SELECT * FROM invoice_correction_requests WHERE id = sqlc.arg(id);

-- name: GetOpenCorrectionRequestByDocument :one
-- 述詞與 uq_invcorr_open_per_document 逐字相同（兩個狀態正面列出）。
SELECT * FROM invoice_correction_requests
WHERE document_id = sqlc.arg(document_id)
  AND status IN ('REQUESTED', 'VOID_REISSUE_IN_PROGRESS');

-- ===========================================================================
-- M6 付款後修改請求狀態機（§7.3.6，8 條 live 轉移 R01–R05、R08–R10）的寫入面。
-- 值域 6 值（chk_invcorr_status），**無 VOID_FAILED**：作廢被拒是一個可自動判定的終局，
-- R05 直接落 REJECTED 並寫 reject_reason_code。
-- 終態：REJECTED、UPDATED_BEFORE_ISSUE、COMPLETED。
-- ===========================================================================

-- name: InsertInvoiceCorrectionRequest :one
-- R01 `[*]` → REQUESTED（買家於 B06 送出）。
-- 序列閘門（I5）：本 document 不得有任何非終態的 VOID／REISSUE／ALLOWANCE operation。
--   那道閘門的 DB 落點是 uq_invop_open_adjustment_per_document ＋ 本表的
--   uq_invcorr_open_per_document（REQUESTED／VOID_REISSUE_IN_PROGRESS 兩值）；
--   application 面在同一交易內先 `SELECT ... FOR UPDATE` 鎖 invoice_documents 該列再判定。
-- ⚠️ ON CONFLICT DO NOTHING 不指名 target：rows=0 ⇒ 這張發票已有進行中的修改請求
--    （B06 顯示「此訂單有處理中的調整」），呼叫端改讀 GetOpenCorrectionRequestByDocument。
-- expected_snapshot_version 是買家送出當下看到的內容版本；它與 document 的 snapshot_version
--   比對由 usecase 做（本表只存快照，不做跨表比較）。
-- before_payload／after_payload **已遮罩**：密文欄位存 hash 與遮罩值，不存明文。
INSERT INTO invoice_correction_requests (
  id, document_id, requested_by_buyer_id, expected_snapshot_version,
  before_payload, after_payload, status, created_at, updated_at)
VALUES (
  sqlc.arg(id), sqlc.arg(document_id), sqlc.arg(requested_by_buyer_id),
  sqlc.arg(expected_snapshot_version), sqlc.arg(before_payload), sqlc.arg(after_payload),
  'REQUESTED', sqlc.arg(now), sqlc.arg(now))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: RejectInvoiceCorrectionRequest :execrows
-- R02 REQUESTED → REJECTED（送出當下的預檢不過）
-- R05 VOID_REISSUE_IN_PROGRESS → REJECTED（f0501 回 3050123／3050126／3050141，作廢被 provider 拒絕）
-- 兩條邊共用一句：差別只在**發現的時點**（預檢時 vs 送出後 AMEGO 才說），買家看到的答案是
--   同一句，reject_reason_code 取自同一組 7 值（chk_invcorr_reject_reason_code）；兩條邊各自
--   只用其中一部分：ALREADY_REPORTED 只有 R02 寫、PROVIDER_STATE_CONFLICT 只有 R05 寫。
--   AMEGO 三個碼一一對映：3050123 ⇒ ALREADY_REPORTED、3050126 ⇒ PAST_VOID_DEADLINE、
--   3050141 ⇒ ALLOWANCE_EXISTS。
-- chk_invcorr_reject_reason：REJECTED ⇒ reject_reason_code 不得為空 ⇒ 兩欄同一句寫。
-- ⚠️ R05 的同一事務內，M1 由 VOIDING 回到 ISSUED（RestoreInvoiceDocumentToIssued）——
--    **買家手上那張憑證仍然有效**，只是內容改不了。少做那一半，那張發票會永遠停在 VOIDING。
-- REJECTED 是終態且不在 uq_invcorr_open_per_document 的述詞內 ⇒ 名額釋放，買家可另提一次。
UPDATE invoice_correction_requests
SET status = 'REJECTED',
    reject_reason_code = sqlc.arg(reject_reason_code),
    reject_reason_detail = sqlc.narg(reject_reason_detail),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('REQUESTED', 'VOID_REISSUE_IN_PROGRESS');

-- name: MarkInvoiceCorrectionUpdatedBeforeIssue :execrows
-- R03 REQUESTED → UPDATED_BEFORE_ISSUE（尚未送出 AMEGO，直接改快照）。
-- 守衛是 operation 的 `HELD` CAS rows=1（HoldInvoiceOperation），不是本句能表達的：
--   搶不到 HELD ⇒ 這條邊不成立，改走 R04 作廢重開。
-- 同一事務內另外三件事：snapshot_version += 1（**由 order-service 回傳值複製**，I-50，
--   本服務沒有遞增它的 query）、重建 lines（DeleteInvoiceDocumentLines ＋ 重新插入）、
--   operation 指向新版（AdminRetryInvoiceOperationAfterFix 的 request_snapshot_hash 那一欄）。
-- 終態。
UPDATE invoice_correction_requests
SET status = 'UPDATED_BEFORE_ISSUE',
    handling_mode = 'UPDATE_SNAPSHOT',
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'REQUESTED';

-- name: StartInvoiceCorrectionVoidReissue :execrows
-- R04 REQUESTED → VOID_REISSUE_IN_PROGRESS（預檢通過且 document 已 ISSUED）。
-- 🔴 順序必須是「**先確保可開再作廢**」：作廢不可逆（且明令不可用 DB restore 把 ISSUED
--    回復為未開立）。預檢把字軌不足／載具失效／已過修改期限／已存在折讓四類**可預先判定**
--    的失敗擋在作廢之前 ⇒ REISSUE_FAILED 就只剩「預檢通過後 AMEGO 才壞掉」這一種殘餘機率。
--    那四個判定在 usecase，本句只負責落地與 CAS。
-- void_adjustment_id 指向同一事務內建立的 VOID adjustment（fk_invcorr_void）。
UPDATE invoice_correction_requests
SET status = 'VOID_REISSUE_IN_PROGRESS',
    handling_mode = 'VOID_AND_REISSUE',
    void_adjustment_id = sqlc.arg(void_adjustment_id),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'REQUESTED';

-- name: MarkInvoiceCorrectionReissueFailed :execrows
-- R08 VOID_REISSUE_IN_PROGRESS → REISSUE_FAILED（作廢成功但重開 operation 進入任一 FAILED_*）。
-- 🔴 **這是本設計書中最嚴重的單一狀態**：買家原本持有的合法發票已經作廢、新發票不存在，
--    而這是平台的操作造成的——買家的公司報帳因此少一張進項憑證。
--    處置四件（都在本句之外、但必須同一批做完）：① A06 獨立的最高優先「無憑證」佇列
--    ② 獨立 SLA：進入本狀態後 **15 分鐘**告警（一般 review_required 是 1 小時）＋ 直接推
--    Discord／PagerDuty 等級 ③ 立即通知買家，文案必須明說「原發票已作廢、新發票開立中」，
--    不得沿用「處理中」的通用文案 ④ INV.dl 每 5 分鐘掃 I8。
-- ⚠️ **不設自動重試排程**：R09 只有 ADM 端點 52 這一個觸發者。自動重送可能開出第二張。
UPDATE invoice_correction_requests
SET status = 'REISSUE_FAILED',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'VOID_REISSUE_IN_PROGRESS';

-- name: RetryInvoiceCorrectionReissue :execrows
-- R09 REISSUE_FAILED → VOID_REISSUE_IN_PROGRESS（ADM 於 A06 對該 correction 的 REISSUE
--     operation 按端點 52 安全重試，M2 J18／J19，operation 回 READY 的**同一事務內**連動本列）。
-- 守衛：該 operation 通過端點 52 的「可安全重試」判定（status ∈ {FAILED_PERMANENT,
--   RETRY_SCHEDULED, READY} 且冪等鍵在 provider 端已確認未產生任何文件）——跨表，在 usecase。
-- ⚠️ **沒有第二個觸發者**。
UPDATE invoice_correction_requests
SET status = 'VOID_REISSUE_IN_PROGRESS',
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status = 'REISSUE_FAILED'
  AND version = sqlc.arg(expected_version);

-- name: GetReissueFailedCorrectionByDocument :one
-- R09／R10 的入口：找這張原始 document 上停在 REISSUE_FAILED 的修改請求。
--
-- 🔴 **為什麼不廣化 GetOpenCorrectionRequestByDocument**：那一支的述詞與
--    uq_invcorr_open_per_document 逐字相同（REQUESTED／VOID_REISSUE_IN_PROGRESS），
--    而 REISSUE_FAILED **不在**那個 partial unique index 裡 ⇒ 同一張 document 上
--    「一筆 REISSUE_FAILED ＋ 一筆新的 REQUESTED」在 DB 層完全合法。把 REISSUE_FAILED
--    加進那一支，它就從「最多一列」變成「可能多列」，而 sqlc 的 :one 對多列是
--    **取任意一列、零錯誤零 log**（#3010 同形）。兩個語意不同的查詢要分成兩支。
--
-- ⚠️ 本支自己也可能命中多列（同一張發票連續兩次作廢重開都失敗），所以明寫
--    ORDER BY id DESC LIMIT 1 取最新那一筆——id 是 TSID，遞增即時序。
--    這是刻意的取捨而不是疏忽：ADM 按下的那一次重試對應的是最近一次失敗。
SELECT * FROM invoice_correction_requests
WHERE document_id = sqlc.arg(document_id)
  AND status = 'REISSUE_FAILED'
ORDER BY id DESC
LIMIT 1;

-- name: CompleteInvoiceCorrectionRequest :execrows
-- R10 VOID_REISSUE_IN_PROGRESS／REISSUE_FAILED → COMPLETED（新 document 轉 ISSUED）。
-- 守衛「原 document 已轉 SUPERSEDED」跨表，在 usecase（MarkInvoiceDocumentSuperseded）。
-- `completed_at`：**本列是它唯一的寫入者**（本表只有 R10 以 COMPLETED 為目標）。
--   它不進任何 DTO、也不是查詢入口，是結案時間的稽核欄；買家面的進度一律讀 status。
-- outbox invoice.reissued（→ 通知買家）由呼叫端在同一交易內寫。終態。
UPDATE invoice_correction_requests
SET status = 'COMPLETED',
    reissued_document_id = sqlc.arg(reissued_document_id),
    completed_at = sqlc.arg(now),
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id)
  AND status IN ('VOID_REISSUE_IN_PROGRESS', 'REISSUE_FAILED');

-- name: GetInvoiceCorrectionRequestForBuyer :one
-- 端點 12（B06 查詢付款後修改申請）。**三段擁有者述詞缺一不可**（§8.1.4(b)）：
--   ① correction 自己的申請人 ＝ 當前買家
--   ② 它掛的 document 的買受人 ＝ 當前買家（ADM 事後改過買受人的情況下 ① 仍成立）
--   ③ 該 document 屬於 request 帶進來的那張訂單（proto GetInvoiceCorrectionRequest
--      同時帶 order_no 與 correction_id ⇒ 不匹配一律走 not-found 出口，不得只驗其一）
-- 三段都在 SQL 裡 ⇒ 越權與不存在同樣是 pgx.ErrNoRows，沒有第二個分支可以回不同的東西。
SELECT r.* FROM invoice_correction_requests r
JOIN invoice_documents d ON d.id = r.document_id
WHERE r.id = sqlc.arg(id)
  AND r.requested_by_buyer_id = sqlc.arg(buyer_id)::bigint
  AND d.source_type = 'ORDER'
  AND d.source_ref = sqlc.arg(source_ref)::text
  AND d.recipient_type = 'BUYER'
  AND d.recipient_ref_id = sqlc.arg(buyer_id)::bigint;

-- name: GetLatestCorrectionRequestByOrder :one
-- B04（端點 8）用：這張**訂單**上最近一次修改請求，不分狀態。
--
-- 🔴 **範圍是訂單不是單一 document**（2026-09-05 修）：更正請求掛在**被作廢的那張**
--    document 上，而 VOID_AND_REISSUE 完成之後訂單的最新 document 是**重開的新那張**
--    ——它身上沒有任何 correction 列。先前這一支以 `latest.ID` 查 ⇒ 每一次成功的
--    作廢重開都會讓 latest_correction_id 立刻變回空字串，而註解寫的是「不分狀態」。
--    dev 實測 O323007025：correction 7501682805546418176 是 COMPLETED、
--    reissued_document_id 指向現行文件，端點 8 仍回空字串。
--    症狀是買家一重開頁面就看不到「你的更正已完成」，而那正是本欄存在的理由。
--
-- 🔴 為什麼不能沿用 GetOpenCorrectionRequestByDocument：那一支只認 REQUESTED／
--    VOID_REISSUE_IN_PROGRESS ⇒ **被婉拒（REJECTED）的那一筆查不到**，而那正是
--    買家最需要看到的一筆（`reject_reason_code` 決定他該改走折讓還是申訴）。
--    2026-09-05 第 6 輪稽核實測：網頁有 `fetchInvoiceCorrection`（端點 12 的 client）
--    卻**零呼叫端**，因為端點 8 的回應裡沒有任何管道拿得到 correction_id
--    ⇒ 買家送出修改申請、關掉頁面再回來，申請進度與婉拒理由三處都看不到。
--
-- ⚠️ 同一張 document 上可以有多列（一筆 REJECTED ＋ 一筆新的 REQUESTED 完全合法，
--    見 GetReissueFailedCorrectionByDocument 的說明）⇒ 必須明寫 ORDER BY ... LIMIT 1。
--    少了它，sqlc 的 `:one` 對多列是**取任意一列、零錯誤零 log**（#3010 同形）。
--    id 是 TSID，遞增即時序。
SELECT r.* FROM invoice_correction_requests r
JOIN invoice_documents d ON d.id = r.document_id
WHERE d.source_type = 'ORDER'
  AND d.source_ref = sqlc.arg(source_ref)::text
  AND r.requested_by_buyer_id = sqlc.arg(buyer_id)::bigint
ORDER BY r.id DESC
LIMIT 1;
