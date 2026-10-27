-- name: InsertReport :one
-- 新增通報；違反任一個去重的 UNIQUE（只比對還沒有結果的通報）時不寫入、回 0 列（不讓 transaction 失效），由呼叫方讀出既有的那一筆。
INSERT INTO seller.reports (
    id, store_id, product_id, order_no, category, description, reporter_buyer_id, reporter_phone, reporter_email,
    reporter_name, created_at, updated_at
)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.narg(product_id), sqlc.narg(order_no), sqlc.arg(category), sqlc.arg(description),
    sqlc.narg(reporter_buyer_id), sqlc.narg(reporter_phone), sqlc.narg(reporter_email), sqlc.narg(reporter_name),
    sqlc.arg(now), sqlc.arg(now)
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: DuplicateBuyerReport :one
-- 登入的檢舉人在同一個台北日對同一個店家、同一個類別已有、還沒有結果的通報（與 reports_buyer_daily 同一個條件）。
SELECT *
FROM seller.reports
WHERE reporter_buyer_id = sqlc.arg(buyer_id)::bigint AND store_id = sqlc.arg(store_id) AND category = sqlc.arg(category) AND outcome IS NULL
  AND (created_at AT TIME ZONE 'Asia/Taipei')::date = (sqlc.arg(now)::timestamptz AT TIME ZONE 'Asia/Taipei')::date;

-- name: DuplicateContactReport :one
-- 匿名的同一支電話或同一個 email 在同一個台北 ISO 週對同一個店家已有、還沒有結果的通報（與 reports_phone_weekly、reports_email_weekly 同一個條件）。
-- matched_by 是這次送出和它比對到的方式：電話與 email 都相同是 BOTH，否則是相同的那一個。
SELECT sqlc.embed(r),
       (CASE
          WHEN r.reporter_phone = sqlc.narg(phone)::text AND r.reporter_email = sqlc.narg(email)::text THEN 'BOTH'
          WHEN r.reporter_phone = sqlc.narg(phone)::text THEN 'PHONE'
          ELSE 'EMAIL'
        END)::text AS matched_by
FROM seller.reports r
WHERE r.store_id = sqlc.arg(store_id) AND r.reporter_buyer_id IS NULL AND r.outcome IS NULL
  AND (r.reporter_phone = sqlc.narg(phone)::text OR r.reporter_email = sqlc.narg(email)::text)
  AND date_trunc('week', r.created_at AT TIME ZONE 'Asia/Taipei') = date_trunc('week', sqlc.arg(now)::timestamptz AT TIME ZONE 'Asia/Taipei')
ORDER BY r.created_at, r.id
LIMIT 1;

-- name: InsertReportSupplement :one
-- 補充：去重期間內同一位檢舉人的重複送出併入原通報（呼叫方已鎖住通報並確認沒有結果、沒有超過上限）。matched_by 只有匿名的補充有。
INSERT INTO seller.report_supplements (id, report_id, seq, description, matched_by, created_at)
VALUES (sqlc.arg(id), sqlc.arg(report_id), sqlc.arg(seq), sqlc.arg(description), sqlc.narg(matched_by), sqlc.arg(now))
RETURNING *;

-- name: SupplementCount :one
SELECT count(*)::int
FROM seller.report_supplements
WHERE report_id = $1;

-- name: ReportSupplements :many
-- 一批通報的補充，依通報、舊到新。
SELECT *
FROM seller.report_supplements
WHERE report_id = ANY(sqlc.arg(report_ids)::uuid[])
ORDER BY report_id, created_at, id;

-- name: InsertReportEvidence :exec
-- 一次送出的第 position 張佐證（submission_seq 0 是原通報、1..10 是補充；position 0..4，DB 保證每次送出最多 5 張）。
INSERT INTO seller.report_evidence (report_id, submission_seq, position, supplement_id, asset_id, object_key)
VALUES (sqlc.arg(report_id), sqlc.arg(submission_seq), sqlc.arg(position), sqlc.narg(supplement_id), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: ReportEvidence :many
-- 一筆通報的佐證，依送出的順序、次內位置；API 的「第 n 張」就是這個順序。
SELECT *
FROM seller.report_evidence
WHERE report_id = $1
ORDER BY submission_seq, position;

-- name: ReportEvidenceOf :many
-- 一批通報的佐證，依通報、送出的順序、次內位置。
SELECT *
FROM seller.report_evidence
WHERE report_id = ANY(sqlc.arg(report_ids)::uuid[])
ORDER BY report_id, submission_seq, position;

-- name: IncrementDiscardedSubmissions :exec
-- 補充已達上限、內容沒有存下的匿名送出：只加計數，不動版本（沒有存下任何會影響後台判斷的內容）。
UPDATE seller.reports
SET discarded_submissions = discarded_submissions + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: BumpReportVersion :exec
-- 補充存下之後讓通報的版本前進：之前讀到這筆通報的分案、設定結果與移動會撞版本，後台重讀才看得到補充。
UPDATE seller.reports
SET version = version + 1,
    updated_at = sqlc.arg(now)
WHERE id = sqlc.arg(id);

-- name: ReportByID :one
SELECT *
FROM seller.reports
WHERE id = $1;

-- name: LockReport :one
-- 鎖定通報，序列化 admin 對它的處理與補充。
SELECT *
FROM seller.reports
WHERE id = $1
FOR UPDATE;

-- name: LockReports :many
-- 依 id 升序鎖定一批通報（鎖順序：店家列、通報列、案件列、措施列；多筆通報依 id 升序）。
SELECT *
FROM seller.reports
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR UPDATE;

-- name: TriageReport :one
-- 分案：掛到案件並記下平台判定類別與分案的人、時間；只在還沒有案件也沒有結果、版本相符時寫入，否則 0 列。
UPDATE seller.reports r
SET case_id = sqlc.arg(case_id)::uuid,
    assessed_category = sqlc.arg(category)::text,
    triaged_at = sqlc.arg(now)::timestamptz,
    triaged_by_admin_id = sqlc.arg(admin_id)::bigint,
    supplements_seen_seq = coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0),
    version = r.version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE r.id = sqlc.arg(id) AND r.version = sqlc.arg(expected_version)::bigint AND r.case_id IS NULL AND r.outcome IS NULL
RETURNING *;

-- name: MoveReport :one
-- 移到其他案件（改平台判定類別）：分案的時間與人不變；只在進行中的案件裡、還沒有結果、版本相符時寫入，否則 0 列。
UPDATE seller.reports r
SET case_id = sqlc.arg(case_id)::uuid,
    assessed_category = sqlc.arg(category)::text,
    supplements_seen_seq = coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0),
    version = r.version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE r.id = sqlc.arg(id) AND r.version = sqlc.arg(expected_version)::bigint AND r.case_id IS NOT NULL AND r.outcome IS NULL
RETURNING *;

-- name: MarkReportSupplementsSeen :one
-- 記下管理員看過這筆通報此刻的全部補充（把 supplements_seen_seq 設成最新一則的編號）；呼叫方已鎖住通報並比對過版本。**不改 version 與 updated_at**
-- （已讀標記不讓 etag 前進，不會讓別處讀到的 etag 過時）。只在還沒有結果、而且存在編號大於已看過的補充時寫入，否則 0 列（冪等）；
-- 條件與 model.Report.HasUnseenSupplements 同一個定義。
UPDATE seller.reports r
SET supplements_seen_seq = (SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id)
WHERE r.id = sqlc.arg(id) AND r.outcome IS NULL
  AND EXISTS (SELECT 1 FROM seller.report_supplements p WHERE p.report_id = r.id AND p.seq > r.supplements_seen_seq)
RETURNING *;

-- name: ResolveReport :one
-- 設定結果；只在還沒有結果、版本相符時寫入，否則 0 列。有了結果的通報不再提醒補充（Report.HasUnseenSupplements），所以不記 supplements_seen_seq。
UPDATE seller.reports
SET outcome = sqlc.arg(outcome)::text,
    resolved_at = sqlc.arg(now)::timestamptz,
    resolved_by_admin_id = sqlc.arg(admin_id)::bigint,
    resolution_note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)::bigint AND outcome IS NULL
RETURNING *;

-- name: ResolveCaseReports :many
-- 案件結案時，還沒有結果的成員全部帶入預設結果與說明；回傳帶入的通報（發 ReportResolved 用）。呼叫方已鎖住案件與全部成員。
UPDATE seller.reports
SET outcome = sqlc.arg(outcome)::text,
    resolved_at = sqlc.arg(now)::timestamptz,
    resolved_by_admin_id = sqlc.arg(admin_id)::bigint,
    resolution_note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE case_id = sqlc.arg(case_id) AND outcome IS NULL
RETURNING id, store_id, reporter_buyer_id;

-- name: ReportDetails :many
-- 一批通報加上讀取時要的店家與補充摘要（單筆讀取、命令的回應與匯出用）：店名（個資清理之後是 NULL）、店家是否已關閉、補充數、最新補充的編號（latest_supplement_seq，「有新的補充」比它與 supplements_seen_seq）與最新補充的時間。
SELECT sqlc.embed(r),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (SELECT count(*) FROM seller.report_supplements p WHERE p.report_id = r.id)::int AS supplement_count,
       coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0)::int AS latest_supplement_seq,
       (SELECT max(p.created_at) FROM seller.report_supplements p WHERE p.report_id = r.id) AS latest_supplement_at
FROM seller.reports r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE r.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY r.created_at, r.id;

-- name: AdminReports :many
-- admin 的通報清單：新到舊（keyset），篩選條件為 NULL 時不篩。state 是處理狀態（與 model.Report.State 同一個定義，由資料集測試鎖住）；
-- category 是生效的類別（案件裡的是平台判定類別，其他是檢舉人選的）；citable_as_source 只留此刻可以當認定來源的通報（店家沒有關閉、結果沒有否定它；
-- 與 model.ReportFacts.Availability 的 CiteAsSource 同一個定義，由資料集測試鎖住）。
SELECT sqlc.embed(r),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (SELECT count(*) FROM seller.report_supplements p WHERE p.report_id = r.id)::int AS supplement_count,
       coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0)::int AS latest_supplement_seq,
       (SELECT max(p.created_at) FROM seller.report_supplements p WHERE p.report_id = r.id) AS latest_supplement_at
FROM seller.reports r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'AWAITING_TRIAGE' AND r.case_id IS NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'IN_CASE' AND r.case_id IS NOT NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'RESOLVED' AND r.outcome IS NOT NULL))
  AND (sqlc.narg(category)::text IS NULL OR coalesce(r.assessed_category, r.category) = sqlc.narg(category)::text)
  AND (sqlc.narg(store_id)::bigint IS NULL OR r.store_id = sqlc.narg(store_id)::bigint)
  AND (sqlc.narg(case_id)::uuid IS NULL OR r.case_id = sqlc.narg(case_id)::uuid)
  AND (NOT sqlc.arg(citable_as_source)::bool
       OR (s.status <> 'CLOSED' AND (r.outcome IS NULL OR r.outcome NOT IN ('NO_VIOLATION', 'INSUFFICIENT', 'ABUSIVE'))))
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (r.created_at, r.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminReportsQueue :many
-- 同 AdminReports，但舊到新（keyset）：待分案與案件成員的處理順序（等最久的在最前面）。
SELECT sqlc.embed(r),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (SELECT count(*) FROM seller.report_supplements p WHERE p.report_id = r.id)::int AS supplement_count,
       coalesce((SELECT max(p.seq) FROM seller.report_supplements p WHERE p.report_id = r.id), 0)::int AS latest_supplement_seq,
       (SELECT max(p.created_at) FROM seller.report_supplements p WHERE p.report_id = r.id) AS latest_supplement_at
FROM seller.reports r
JOIN seller.stores s ON s.store_id = r.store_id
WHERE (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'AWAITING_TRIAGE' AND r.case_id IS NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'IN_CASE' AND r.case_id IS NOT NULL AND r.outcome IS NULL)
    OR (sqlc.narg(state)::text = 'RESOLVED' AND r.outcome IS NOT NULL))
  AND (sqlc.narg(category)::text IS NULL OR coalesce(r.assessed_category, r.category) = sqlc.narg(category)::text)
  AND (sqlc.narg(store_id)::bigint IS NULL OR r.store_id = sqlc.narg(store_id)::bigint)
  AND (sqlc.narg(case_id)::uuid IS NULL OR r.case_id = sqlc.narg(case_id)::uuid)
  AND (NOT sqlc.arg(citable_as_source)::bool
       OR (s.status <> 'CLOSED' AND (r.outcome IS NULL OR r.outcome NOT IN ('NO_VIOLATION', 'INSUFFICIENT', 'ABUSIVE'))))
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (r.created_at, r.id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY r.created_at, r.id
LIMIT sqlc.arg(page_size);

-- name: ReportFindingSources :many
-- 這些通報裡，哪些是某個認定的來源（finding_reports）：認定的來源通報不能移到其他案件。
SELECT DISTINCT report_id
FROM seller.finding_reports
WHERE report_id = ANY(sqlc.arg(report_ids)::uuid[]);

-- name: BuyerReports :many
-- 登入的檢舉人自己的通報，新到舊（keyset）。
SELECT *
FROM seller.reports
WHERE reporter_buyer_id = sqlc.arg(buyer_id)::bigint
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (created_at, id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: EraseExpiredContacts :many
-- 把清除時間已到的匿名通報的聯絡欄位清成 NULL，一次最多 batch_size 筆；回傳每筆清掉的通報是否只靠絕對上限（capped）清的。清除時間有兩條：
-- 一是有結果的，greatest(建立 + 90 天, 結果 + 30 天)；二是絕對上限，建立滿 365 天（created_at < cap_cutoff），不論有沒有結果。依 created_at 掃描。
UPDATE seller.reports
SET reporter_phone = NULL,
    reporter_email = NULL,
    reporter_name = NULL,
    contact_erased_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id IN (
    SELECT r.id
    FROM seller.reports r
    WHERE r.reporter_buyer_id IS NULL AND r.contact_erased_at IS NULL
      AND (r.created_at < sqlc.arg(cap_cutoff)::timestamptz
           OR (r.resolved_at IS NOT NULL AND r.created_at < sqlc.arg(created_cutoff)::timestamptz AND r.resolved_at < sqlc.arg(resolved_cutoff)::timestamptz))
    ORDER BY r.created_at
    LIMIT sqlc.arg(batch_size)
    FOR UPDATE SKIP LOCKED
)
RETURNING (NOT (resolved_at IS NOT NULL AND created_at < sqlc.arg(created_cutoff)::timestamptz AND resolved_at < sqlc.arg(resolved_cutoff)::timestamptz))::bool AS capped;

-- name: EvidenceAssets :many
-- 以 ref_id（通報、請求或詢問的 ID）登記的佐證中，業務列目前指著的 asset（同步 job 讀的）。
SELECT asset_id FROM seller.report_evidence WHERE report_id = sqlc.arg(ref_id)
UNION ALL
SELECT asset_id FROM seller.measure_request_evidence WHERE request_id = sqlc.arg(ref_id)
UNION ALL
SELECT asset_id FROM seller.case_inquiry_reply_evidence WHERE inquiry_id = sqlc.arg(ref_id);

-- name: InsertEvidenceAccess :exec
-- 佐證的讀取紀錄：在讀取之前寫入；report_id 與 request_id 剛好一個，連同 position（通報的另加 submission_seq）指向那一張佐證。
INSERT INTO seller.evidence_accesses (access_id, report_id, submission_seq, request_id, position, admin_id, accessed_at)
VALUES (sqlc.arg(access_id), sqlc.narg(report_id), sqlc.narg(submission_seq), sqlc.narg(request_id), sqlc.arg(position), sqlc.arg(admin_id), sqlc.arg(now));

-- name: ReviewReportTriageByIDs :many
-- 這些通報的「待分案」開始時間與期限（資源的 triage_deadline）。規則只有 seller.review_report_triage(at)（objects.sql），這裡只選它的結果；
-- 不是待分案的通報沒有列。函式內聯後 `report_id = ANY` 成為 reports 主鍵的 Index Cond（TestReviewDeadlinePushdown 鎖住）。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.report_id::uuid AS report_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_report_triage(sqlc.arg(at)::timestamptz) r
WHERE r.report_id = ANY(sqlc.arg(ids)::uuid[]);
