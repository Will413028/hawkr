-- 總覽計數（端點 16 S01 賣家總覽／端點 37 A01 ADM 營運總覽）
--
-- 本檔是既有「一表一檔、按 FROM 的表歸檔」規則的**明示例外**：兩支各自橫跨
-- 3–4 張表，歸到任何一張都是錯的。歸檔鍵改成「服務哪一個端點」。
--
-- 🔴 每個 KPI 的口徑必須寫死（§6.1.11a）：沒有口徑的儀表板數字會讓兩個人看同一個
--    畫面得到不同結論，而 A01 是排班決定「今天先處理什麼」的依據。狀態集合一律
--    由呼叫端以 text[] 傳入（禁硬編碼），對應的 `definition` 文案由同一份常數產生
--    ——**述詞與文案必須同源**，否則文案會描述一個查詢早就不做的事。
--
-- 🔴 每支都用單一 SELECT 一次取回全部計數，不拆成 N 次往返：拆開之後每個計數
--    落在不同的時間點，總覽頁會出現「待處理 3 筆、但清單只有 2 筆」這種自相矛盾，
--    而它是間歇性的、沒有任何測試抓得到。

-- name: GetSellerInvoiceOverviewCounts :one
-- 端點 16（S01）的三個計數。issuer_id 由呼叫端先以 store_id 解析（GetInvoiceIssuerByRef）。
--
-- `overdue_complaint_count` 的述詞是 **OR** 不是單純 `status='OVERDUE'`（§6.1.11a
-- 同名 KPI 的口徑）：OVERDUE 由 INV.dl 掃描寫入，掃描到期之間有一段窗口，此時
-- sla_due_at 已過但狀態還是 OPEN。只取前者會讓賣家在最該行動的那幾小時看到 0。
SELECT
  (SELECT COUNT(*)::bigint FROM invoice_documents d
    WHERE d.issuer_id = sqlc.arg(issuer_id)
      AND d.status = ANY(sqlc.arg(pending_document_statuses)::text[])) AS pending_document_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.issuer_id = sqlc.arg(issuer_id)
      AND c.status = ANY(sqlc.arg(open_complaint_statuses)::text[]))   AS open_complaint_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.issuer_id = sqlc.arg(issuer_id)
      AND (c.status = ANY(sqlc.arg(overdue_complaint_statuses)::text[])
           OR (c.status = ANY(sqlc.arg(open_complaint_statuses)::text[])
               AND c.sla_due_at <= sqlc.arg(now)::timestamptz)))       AS overdue_complaint_count;

-- name: GetInvoiceOverviewCountsForAdmin :one
-- 端點 37（A01）的 7 個 KPI。
--
-- ⚠️ 第 8 個 KPI `suspended_sellers`（因發票違規而停權中的賣家數）**不在本查詢裡**：
--    停權狀態的權威是 sellers.status（seller-service），發票模組只提建議、不執行、
--    也不持有那個狀態（§6.1.16「發票模組只提建議、不執行」）。在這裡湊一個本地
--    計數等於造第二個真相。呼叫端從 seller-service 取，取不到就明說取不到。
--
-- `needs_manual` 是 job 與 document 兩個集合的**聯集**（AWAITING_MANUAL 屬於文件不屬於
--    job，§6.1.14）⇒ 從 invoice_operations 起手 join document，用 OR 而不是 AND。
-- `near_deadline` 是 `pending_tax_reviews` 的**子集**（前四狀態 AND 期限逼近），
--    兩者相加沒有意義——A01 不得顯示合計（§6.1.12 同一條理由）。
SELECT
  (SELECT COUNT(*)::bigint FROM seller_tax_profiles p
    WHERE p.review_status = ANY(sqlc.arg(tax_review_queue_statuses)::text[]))
      AS pending_tax_review_count,
  (SELECT COUNT(*)::bigint FROM seller_tax_profiles p
    WHERE p.review_status = ANY(sqlc.arg(tax_review_queue_statuses)::text[])
      AND p.pending_valid_until IS NOT NULL
      AND p.pending_valid_until <= sqlc.arg(near_deadline_before)::timestamptz)
      AS near_deadline_count,
  (SELECT COUNT(*)::bigint FROM invoice_provider_accounts a
    WHERE a.status = ANY(sqlc.arg(provider_error_statuses)::text[]))
      AS provider_error_count,
  (SELECT COUNT(*)::bigint FROM invoice_operations o
    WHERE o.status = ANY(sqlc.arg(pending_job_statuses)::text[]))
      AS pending_job_count,
  (SELECT COUNT(*)::bigint FROM invoice_operations o
     JOIN invoice_documents d ON d.id = o.document_id
    WHERE (o.status = ANY(sqlc.arg(needs_manual_job_statuses)::text[])
        OR d.status = ANY(sqlc.arg(manual_document_statuses)::text[]))
      -- 🔴 已收束的 document 一律不算（#3014）：一筆 FAILED_PERMANENT 的 ISSUE
      -- operation，在它的 document 被作廢／被重開票取代之後**已經沒有人要處理**了
      -- ——重開那一步就是處理。dev 實測 needs_manual=4 全部是這種票，
      -- 而 A05 的每一筆都在要求管理員去看一件已經結束的事。
      -- 用 NOT (...) 而不是把它併進上面的 OR：上面兩個條件是「需要人」的來源，
      -- 這一個是「不需要人」的否決，兩者不同層，混在同一個布林式裡下次一定被改錯。
      -- ⚠️ COALESCE：`x = ANY(NULL)` 是 NULL ⇒ 漏傳時整個計數變 0 而不是「不排除」。
      AND NOT (d.status = ANY(COALESCE(sqlc.arg(resolved_document_statuses)::text[], ARRAY[]::text[]))))
      AS needs_manual_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.status = ANY(sqlc.arg(open_complaint_statuses)::text[]))
      AS open_complaint_count,
  (SELECT COUNT(*)::bigint FROM invoice_complaints c
    WHERE c.status = ANY(sqlc.arg(overdue_complaint_statuses)::text[])
       OR (c.status = ANY(sqlc.arg(open_complaint_statuses)::text[])
           AND c.sla_due_at <= sqlc.arg(now)::timestamptz))
      AS overdue_complaint_count;
