-- name: InsertViolation :one
-- 認定只由建立措施的 transaction 寫入（origin_measure_id 是那個措施；與 measure_findings 的複合外鍵延遲到 commit 才檢查）。
INSERT INTO seller.violations (
    id, store_id, account_id, category, severity, escalated_from, origin_measure_id,
    created_by_admin_id, created_at, updated_at
)
VALUES (
    sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(account_id), sqlc.arg(category), sqlc.arg(severity),
    sqlc.narg(escalated_from), sqlc.arg(origin_measure_id), sqlc.arg(admin_id), sqlc.arg(now), sqlc.arg(now)
)
RETURNING *;

-- name: InsertFindingReports :exec
-- 認定的來源通報（與 InsertViolation 同一個 transaction；呼叫方已依 id 升序鎖住這些通報並驗證過）。
INSERT INTO seller.finding_reports (violation_id, report_id)
SELECT sqlc.arg(violation_id), unnest(sqlc.arg(report_ids)::uuid[]);

-- name: FindingReportIDs :many
-- 一批認定的來源通報，依認定、通報 id 排序。
SELECT violation_id, report_id
FROM seller.finding_reports
WHERE violation_id = ANY(sqlc.arg(violation_ids)::uuid[])
ORDER BY violation_id, report_id;

-- name: CountRecentMediumViolations :one
-- 升級規則的計數：帳號自 since 起未撤銷、原本是 MEDIUM 的認定（帳號所有 stint 一起算）。呼叫方已鎖住帳號的全部 stint，
-- 計數與新增之間不會插入別筆。
SELECT count(*)
FROM seller.violations
WHERE account_id = sqlc.arg(account_id) AND status = 'ENFORCED' AND COALESCE(escalated_from, severity) = 'MEDIUM'
  AND created_at >= sqlc.arg(since);

-- name: ViolationByID :one
SELECT *
FROM seller.violations
WHERE id = $1;

-- name: ViolationsByIDs :many
SELECT *
FROM seller.violations
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id;

-- name: OriginFindings :many
-- 措施自己建立、仍生效的認定（撤銷措施並要求撤銷依據時，撤銷的就是它們）。
SELECT *
FROM seller.violations
WHERE origin_measure_id = $1 AND status = 'ENFORCED'
ORDER BY created_at, id;

-- name: RevokeViolations :many
-- 撤銷一批認定（連同它們的措施撤銷時）；已撤銷的不動。
UPDATE seller.violations
SET status = 'REVOKED',
    revoked_reason = sqlc.arg(reason)::text,
    revoked_by_admin_id = sqlc.arg(admin_id)::bigint,
    revoked_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND status = 'ENFORCED'
RETURNING *;

-- name: AdminViolations :many
-- admin 的認定列表，新到舊（keyset），篩選條件為 NULL 時不篩。
SELECT *
FROM seller.violations v
WHERE (sqlc.narg(store_id)::bigint IS NULL OR v.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR v.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(status)::text IS NULL OR v.status = sqlc.narg(status))
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (v.created_at, v.id) < (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY v.created_at DESC, v.id DESC
LIMIT sqlc.arg(page_size);

-- name: InsertMeasure :one
-- 新增措施：目前值寫進 measures，同一個語句追加第 1 版的歷史快照（SQL:2011 系統版本表：目前值與歷史由同一個寫入路徑維持一致）。
-- 依據與 current_revision 的外鍵延遲到 commit 才檢查（依據在同一個 transaction 稍後寫入）。starts_at 由呼叫方算（停權與立即生效的終止是 now，
-- 預告終止是 now 加 720 小時；CHECK 保證）。
WITH created AS (
    INSERT INTO seller.measures (
        id, account_id, store_id, kind, basis, risk_ground, lead_finding_id, review_by, statement_to_seller, starts_at, ends_at,
        termination_ground, source_measure_id, source_case_id, source_authority_order_id, current_revision, request_id, request_hash, created_by_admin_id, created_at, updated_at
    )
    VALUES (
        sqlc.arg(id), sqlc.arg(account_id), sqlc.arg(store_id), sqlc.arg(kind), sqlc.arg(basis), sqlc.narg(risk_ground),
        sqlc.narg(lead_finding_id), sqlc.narg(review_by), sqlc.arg(statement_to_seller), sqlc.arg(starts_at), sqlc.narg(ends_at),
        sqlc.narg(termination_ground), sqlc.narg(source_measure_id), sqlc.narg(source_case_id), sqlc.narg(source_authority_order_id),
        1, sqlc.narg(request_id), sqlc.narg(request_hash), sqlc.arg(admin_id), sqlc.arg(now), sqlc.arg(now)
    )
    RETURNING *
), history AS (
    INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, created_at)
    SELECT id, current_revision, statement_to_seller, ends_at, created_by_admin_id, created_at FROM created
)
SELECT * FROM created;

-- name: InsertMeasureFinding :exec
INSERT INTO seller.measure_findings (measure_id, violation_id)
VALUES (sqlc.arg(measure_id), sqlc.arg(violation_id));

-- name: MeasureFindings :many
-- 一批措施的依據認定；每個措施的主要依據（lead_finding_id，建立時由規則決定：有新的認定是新的認定，否則是最嚴重、同嚴重度最新的引用認定）排第一，
-- 其餘依建立時間。
SELECT mf.measure_id, sqlc.embed(v)
FROM seller.measure_findings mf
JOIN seller.violations v ON v.id = mf.violation_id
JOIN seller.measures m ON m.id = mf.measure_id
WHERE mf.measure_id = ANY(sqlc.arg(measure_ids)::uuid[])
ORDER BY mf.measure_id, (v.id = m.lead_finding_id) DESC, v.created_at, v.id;

-- name: MeasuresCitingFindings :many
-- 引用這些認定、仍未撤銷的措施（撤銷認定時連帶撤銷它們）；排除發動的措施。
SELECT DISTINCT m.*
FROM seller.measure_findings mf
JOIN seller.measures m ON m.id = mf.measure_id
WHERE mf.violation_id = ANY(sqlc.arg(violation_ids)::uuid[]) AND m.status = 'ENFORCED' AND m.id <> sqlc.arg(except_id)
ORDER BY m.id;

-- name: MeasureByID :one
SELECT *
FROM seller.measures
WHERE id = $1;

-- name: LockMeasure :one
SELECT *
FROM seller.measures
WHERE id = $1
FOR UPDATE;

-- name: MeasuresByIDs :many
SELECT *
FROM seller.measures
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id;

-- name: MeasureStoreNames :many
-- 一批措施的店名（store_id 去重後的批次讀取）；個資清理之後 store_name 是 NULL。
SELECT store_id, store_name
FROM seller.stores
WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: MeasureByRequest :one
-- 冪等重送的查詢：範圍是 (管理員, request_id)。
SELECT *
FROM seller.measures
WHERE created_by_admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: MeasureRevisions :many
-- 版本歷史（含目前那一版），舊到新。
SELECT *
FROM seller.measure_revisions
WHERE measure_id = $1
ORDER BY revision;

-- name: AdvanceMeasureRevision :one
-- 前進到下一版（版本不符或已撤銷時 0 列）：目前值（給賣家的理由、結束時間）寫進 measures，同一個語句把新一版的快照追加到
-- measure_revisions（current_revision 的外鍵延遲到 commit 才檢查）。呼叫方傳入完整的下一版內容，沒改的欄位也要帶原值。作者是管理員（admin_id），
-- 或系統（system_cause，沒有管理員：店家關閉時結束預告終止）；兩者剛好一個（measure_revisions_author_check）。
WITH advanced AS (
    UPDATE seller.measures
    SET current_revision = current_revision + 1,
        statement_to_seller = sqlc.arg(statement_to_seller)::text,
        ends_at = sqlc.narg(ends_at)::timestamptz,
        updated_at = sqlc.arg(now),
        version = version + 1
    WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED'
    RETURNING *
), history AS (
    INSERT INTO seller.measure_revisions (measure_id, revision, statement_to_seller, ends_at, admin_id, system_cause, created_at)
    SELECT id, current_revision, statement_to_seller, ends_at, sqlc.narg(admin_id)::bigint, sqlc.narg(system_cause)::text, updated_at FROM advanced
)
SELECT * FROM advanced;

-- name: TouchMeasure :one
-- 不改賣家看得到的版本、但後台看到的狀態改變時（請求提出或駁回）：後台版本加一。版本不符或已撤銷時 0 列。
UPDATE seller.measures
SET updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED'
RETURNING *;

-- name: SetMeasureReview :one
-- 重排複審期限（RISK）：後台版本加一，賣家的版本（current_revision）與目前值不變。
UPDATE seller.measures
SET review_by = sqlc.arg(review_by)::timestamptz,
    updated_at = sqlc.arg(now),
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED' AND basis = 'RISK'
RETURNING *;

-- name: RevokeMeasure :one
-- ENFORCED → REVOKED（admin 撤銷或接受申覆）；版本不符或已撤銷時 0 列。
UPDATE seller.measures
SET status = 'REVOKED',
    revoked_reason = sqlc.arg(reason)::text,
    revoked_by_admin_id = sqlc.arg(admin_id)::bigint,
    revoked_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz,
    version = version + 1
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND status = 'ENFORCED'
RETURNING *;

-- name: CascadeRevokeMeasures :many
-- 連帶撤銷的措施：依據的認定被撤銷（FINDING_REVOKED）或來源的終止被撤銷（SOURCE_REVOKED），原因由呼叫方給；不帶版本（呼叫方已鎖住帳號與這些措施）。
UPDATE seller.measures
SET status = 'REVOKED',
    revoked_reason = sqlc.arg(reason)::text,
    revoked_by_admin_id = sqlc.arg(admin_id)::bigint,
    revoked_at = sqlc.arg(now)::timestamptz,
    updated_at = sqlc.arg(now)::timestamptz,
    version = version + 1
WHERE id = ANY(sqlc.arg(ids)::uuid[]) AND status = 'ENFORCED'
RETURNING *;

-- name: InsertGovernanceNote :one
-- 措施或案件的內部註記（剛好一個，CHECK 保證）：只新增，沒有更新與刪除；不動 measures 與 cases，所以不改 version。措施或案件不存在時外鍵擋下（23503）。
-- 改案件嚴重度時寫的那則註記帶改前與改後（severity_from、severity_to）。RETURNING：回應用資料庫存下的值（timestamptz 只到微秒），與之後讀到的完全相同。
INSERT INTO seller.governance_notes (note_id, measure_id, case_id, admin_id, body, severity_from, severity_to, created_at)
VALUES (sqlc.arg(note_id), sqlc.narg(measure_id), sqlc.narg(case_id), sqlc.arg(admin_id), sqlc.arg(body), sqlc.narg(severity_from), sqlc.narg(severity_to), sqlc.arg(now))
RETURNING *;

-- name: MeasureNotes :many
-- 內部註記，舊到新（keyset，AIP-158）；after_* 為 NULL 時從頭開始。
SELECT *
FROM seller.governance_notes
WHERE measure_id = sqlc.arg(measure_id)
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (created_at, note_id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at, note_id
LIMIT sqlc.arg(page_size);

-- name: CaseNotes :many
-- 案件的內部註記，舊到新（keyset，AIP-158）；after_* 為 NULL 時從頭開始。
SELECT *
FROM seller.governance_notes
WHERE case_id = sqlc.arg(case_id)
  AND (
    sqlc.narg(after_created_at)::timestamptz IS NULL OR
    (created_at, note_id) > (sqlc.narg(after_created_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY created_at, note_id
LIMIT sqlc.arg(page_size);

-- name: InsertMeasureRequest :one
-- basis 由措施帶入（複合外鍵 measure_requests_measure_basis_fkey 與 CHECK 保證解除請求只掛在風險措施上）。
INSERT INTO seller.measure_requests (id, measure_id, kind, state, reason, revision, submitted_at, basis)
SELECT sqlc.arg(id)::uuid, m.id, sqlc.arg(kind)::text, 'PENDING', sqlc.arg(reason)::text, sqlc.arg(revision)::integer, sqlc.arg(now)::timestamptz, m.basis
FROM seller.measures m
WHERE m.id = sqlc.arg(measure_id)
RETURNING *;

-- name: InsertRequestEvidence :exec
INSERT INTO seller.measure_request_evidence (request_id, position, asset_id, object_key)
VALUES (sqlc.arg(request_id), sqlc.arg(position), sqlc.arg(asset_id), sqlc.arg(object_key));

-- name: RequestEvidence :many
SELECT *
FROM seller.measure_request_evidence
WHERE request_id = $1
ORDER BY position;

-- name: RequestsEvidence :many
-- 一批請求的佐證（讀取單筆措施時一次載入全部請求的佐證）。
SELECT *
FROM seller.measure_request_evidence
WHERE request_id = ANY(sqlc.arg(request_ids)::uuid[])
ORDER BY request_id, position;

-- name: RequestsByMeasures :many
-- 一批措施上的請求，新到舊。
SELECT *
FROM seller.measure_requests
WHERE measure_id = ANY(sqlc.arg(measure_ids)::uuid[])
ORDER BY measure_id, submitted_at DESC, id DESC;

-- name: DecideRequest :one
-- 裁定待處理的請求（接受或駁回，理由必填）；不是 PENDING 時 0 列。
UPDATE seller.measure_requests
SET state = sqlc.arg(state)::text,
    decision_reason = sqlc.arg(reason)::text,
    decided_by_admin_id = sqlc.arg(admin_id)::bigint,
    decided_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND state = 'PENDING'
RETURNING *;

-- name: CloseRequests :many
-- 措施撤銷時，把這些措施上待處理的請求結案（不帶理由，原因 REVOKED，帶執行撤銷的 admin）。
UPDATE seller.measure_requests
SET state = 'CLOSED',
    close_cause = 'REVOKED',
    decided_by_admin_id = sqlc.arg(admin_id)::bigint,
    decided_at = sqlc.arg(now)::timestamptz
WHERE measure_id = ANY(sqlc.arg(measure_ids)::uuid[]) AND state = 'PENDING'
RETURNING *;

-- name: CloseEndedRemediations :many
-- 措施已結束而失去標的的待處理解除請求，以 CLOSED（ENDED）結案：系統結案沒有 admin，結案時間是措施的結束時間（不是現在），讓結案前後
-- model.Measure.EffectiveRequest 的輸出相同。
UPDATE seller.measure_requests r
SET state = 'CLOSED',
    close_cause = 'ENDED',
    decided_by_admin_id = NULL,
    decided_at = COALESCE(m.ends_at, r.submitted_at)
FROM seller.measures m
WHERE m.id = r.measure_id AND r.measure_id = ANY(sqlc.arg(measure_ids)::uuid[]) AND r.kind = 'REMEDIATION' AND r.state = 'PENDING'
RETURNING r.*;

-- name: MeasuresWithEndedRemediation :many
-- 已結束（停權措施、不在 active_ids）而仍有待處理解除請求的措施；active_ids 是同一個 transaction 內 EffectiveMeasures 的結果（「生效中」的
-- 定義只有那一處）；account_id 為 NULL 時不限帳號。解除請求只在措施生效中才有標的，措施的結案掃描與 SubmitAppeal 以外的路徑用它找到
-- 要結案的措施（時效，不是正確性：對外的狀態由 model.Measure.EffectiveRequest 推導，寫入路徑各自強制不變量）。
SELECT m.*
FROM seller.measures m
WHERE m.status = 'ENFORCED' AND m.kind = 'SUSPENSION' AND m.id <> ALL(sqlc.arg(active_ids)::uuid[])
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND EXISTS (SELECT 1 FROM seller.measure_requests r WHERE r.measure_id = m.id AND r.kind = 'REMEDIATION' AND r.state = 'PENDING')
ORDER BY m.id;

-- 措施列表的 state 篩選：ACTIVE 與 ENDED 用同一個交易取得的 EffectiveMeasures 結果（active_ids）；「生效中」的 SQL 定義是 seller.effective_measures(at)（EffectiveMeasures
-- 指定帳號、EffectiveMeasuresAll 全部帳號都讀它），與 model.StateOf 的一致由 TestEffectiveMeasuresPerAccountAllAccountsAndStateOfAgreeOnADataset 鎖住；ISSUED、REVOKED 只看儲存的欄位。

-- name: AccountMeasures :many
-- 賣家自己的措施：帳號所有 stint 上的（效果以帳號為單位），新到舊（keyset）；
SELECT m.*
FROM seller.measures m
WHERE m.account_id = sqlc.arg(account_id)
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind = 'WARNING')
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (m.created_at, m.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminMeasures :many
-- admin 的措施列表，新到舊（keyset），篩選條件為 NULL 時不篩；request_state 篩選的是請求對外的狀態（model.Measure.EffectiveRequest：措施已不在
-- active_ids 而儲存為 PENDING 的解除請求算 CLOSED），所以 request_state 不為 NULL 時 active_ids 一定是 EffectiveMeasures 的結果；
-- 與推導的一致由 TestListMeasuresByRequestStateAgreesWithEffectiveRequest 鎖住。
SELECT m.*
FROM seller.measures m
WHERE (sqlc.narg(store_id)::bigint IS NULL OR m.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(source_case_id)::uuid IS NULL OR m.source_case_id = sqlc.narg(source_case_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    JOIN seller.reports r ON r.id = fr.report_id
    WHERE mf.measure_id = m.id AND r.case_id = sqlc.narg(source_case_id)::uuid))
  AND (sqlc.narg(source_report_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fr.report_id = sqlc.narg(source_report_id)::uuid))
  AND (sqlc.narg(source_authority_order_id)::uuid IS NULL OR m.source_authority_order_id = sqlc.narg(source_authority_order_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.narg(source_authority_order_id)::uuid))
  AND (sqlc.narg(basis)::text IS NULL OR m.basis = sqlc.narg(basis))
  AND (sqlc.narg(request_state)::text IS NULL AND sqlc.narg(request_kind)::text IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_requests r
    WHERE r.measure_id = m.id
      AND (sqlc.narg(request_kind)::text IS NULL OR r.kind = sqlc.narg(request_kind)::text)
      AND (sqlc.narg(request_state)::text IS NULL OR sqlc.narg(request_state)::text =
        CASE WHEN r.state = 'PENDING' AND r.kind = 'REMEDIATION' AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) THEN 'CLOSED' ELSE r.state END)))
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind = 'WARNING')
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (m.created_at, m.id) < (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT sqlc.arg(page_size);

-- name: AdminMeasuresPendingRequests :many
-- 待處理請求的收件匣（申覆與解除請求合在一條查詢）：依請求提出時間舊到新（等最久的在最前面）；sort_at 是 keyset 的位置。解除請求只在措施生效中
-- （active_ids，EffectiveMeasures 的結果）才算待處理，與 model.Measure.EffectiveRequest 同義。
SELECT sqlc.embed(m), q.submitted_at AS sort_at
FROM seller.measures m
JOIN seller.measure_requests q ON q.measure_id = m.id AND q.state = 'PENDING'
  AND (q.kind = 'APPEAL' OR m.id = ANY(sqlc.arg(active_ids)::uuid[]))
WHERE (sqlc.narg(store_id)::bigint IS NULL OR m.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(source_case_id)::uuid IS NULL OR m.source_case_id = sqlc.narg(source_case_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    JOIN seller.reports r ON r.id = fr.report_id
    WHERE mf.measure_id = m.id AND r.case_id = sqlc.narg(source_case_id)::uuid))
  AND (sqlc.narg(source_report_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fr.report_id = sqlc.narg(source_report_id)::uuid))
  AND (sqlc.narg(source_authority_order_id)::uuid IS NULL OR m.source_authority_order_id = sqlc.narg(source_authority_order_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.narg(source_authority_order_id)::uuid))
  AND (sqlc.narg(basis)::text IS NULL OR m.basis = sqlc.narg(basis))
  AND (sqlc.narg(request_kind)::text IS NULL OR q.kind = sqlc.narg(request_kind)::text)
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind = 'WARNING')
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (q.submitted_at, m.id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY q.submitted_at, m.id
LIMIT sqlc.arg(page_size);

-- name: AdminMeasuresReviewDue :many
-- 複審到期的收件匣：成員只讀 seller.review_measure_review(now)（objects.sql，後台待辦的 MEASURE_REVIEW 也讀它）：RISK、生效中、
-- now 不早於 review_by 前 24 小時（3cd D′11）；依 review_by 舊到新。其餘篩選（店家、帳號、依據、請求、狀態）照列表的語意一併套用，
-- active_ids（EffectiveMeasures 的結果）只給請求與狀態篩選。
SELECT sqlc.embed(m), m.review_by AS sort_at
FROM seller.measures m
WHERE m.id IN (SELECT r.measure_id FROM seller.review_measure_review(sqlc.arg(now)::timestamptz) r)
  AND (sqlc.narg(store_id)::bigint IS NULL OR m.store_id = sqlc.narg(store_id))
  AND (sqlc.narg(account_id)::bigint IS NULL OR m.account_id = sqlc.narg(account_id))
  AND (sqlc.narg(source_case_id)::uuid IS NULL OR m.source_case_id = sqlc.narg(source_case_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    JOIN seller.reports r ON r.id = fr.report_id
    WHERE mf.measure_id = m.id AND r.case_id = sqlc.narg(source_case_id)::uuid))
  AND (sqlc.narg(source_report_id)::uuid IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_reports fr ON fr.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fr.report_id = sqlc.narg(source_report_id)::uuid))
  AND (sqlc.narg(source_authority_order_id)::uuid IS NULL OR m.source_authority_order_id = sqlc.narg(source_authority_order_id)::uuid OR EXISTS (
    SELECT 1 FROM seller.measure_findings mf
    JOIN seller.finding_authority_orders fa ON fa.violation_id = mf.violation_id
    WHERE mf.measure_id = m.id AND fa.order_id = sqlc.narg(source_authority_order_id)::uuid))
  AND (sqlc.narg(basis)::text IS NULL OR m.basis = sqlc.narg(basis))
  AND (sqlc.narg(request_state)::text IS NULL AND sqlc.narg(request_kind)::text IS NULL OR EXISTS (
    SELECT 1 FROM seller.measure_requests r
    WHERE r.measure_id = m.id
      AND (sqlc.narg(request_kind)::text IS NULL OR r.kind = sqlc.narg(request_kind)::text)
      AND (sqlc.narg(request_state)::text IS NULL OR sqlc.narg(request_state)::text =
        CASE WHEN r.state = 'PENDING' AND r.kind = 'REMEDIATION' AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) THEN 'CLOSED' ELSE r.state END)))
  AND (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'ISSUED' AND m.status = 'ENFORCED' AND m.kind = 'WARNING')
    OR (sqlc.narg(state)::text = 'REVOKED' AND m.status = 'REVOKED')
    OR (sqlc.narg(state)::text = 'ACTIVE' AND m.id = ANY(sqlc.arg(active_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'SCHEDULED' AND m.id = ANY(sqlc.arg(scheduled_ids)::uuid[]))
    OR (sqlc.narg(state)::text = 'ENDED' AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.id <> ALL(sqlc.arg(active_ids)::uuid[]) AND m.id <> ALL(sqlc.arg(scheduled_ids)::uuid[])))
  AND (
    sqlc.narg(after_at)::timestamptz IS NULL OR
    (m.review_by, m.id) > (sqlc.narg(after_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY m.review_by, m.id
LIMIT sqlc.arg(page_size);

-- name: LinkedMeasureAccounts :many
-- 以這些措施為來源、仍未撤銷的關聯終止所在的帳號（撤銷來源之前先依 account_id 升序鎖住它們，不加鎖讀取；鎖住之後 LockLinkedMeasures 再讀一次）。
SELECT DISTINCT account_id
FROM seller.measures
WHERE source_measure_id = ANY(sqlc.arg(source_ids)::uuid[]) AND status = 'ENFORCED'
ORDER BY account_id;

-- name: LockLinkedMeasures :many
-- 以這些措施為來源、仍未撤銷的關聯終止，鎖住（呼叫方已鎖住這些帳號全部的 stint）；來源撤銷時連帶撤銷它們。
SELECT *
FROM seller.measures
WHERE source_measure_id = ANY(sqlc.arg(source_ids)::uuid[]) AND status = 'ENFORCED'
ORDER BY id
FOR UPDATE;

-- name: LinkedMeasuresOf :many
-- 以這筆措施為來源、仍未撤銷的關聯終止（admin 的關聯帳號區塊顯示 linked_measure_id）。
SELECT id, account_id
FROM seller.measures
WHERE source_measure_id = $1 AND status = 'ENFORCED';

-- name: QualifyingCitedFindings :many
-- MATERIAL_BREACH（條款 21.3 第 1 款）引用的認定中，未撤銷、屬於這個帳號、而且建立它的措施早於 older_than（至少已存在 7 日）的。
SELECT v.id
FROM seller.violations v
JOIN seller.measures om ON om.id = v.origin_measure_id
WHERE v.id = ANY(sqlc.arg(ids)::uuid[]) AND v.account_id = sqlc.arg(account_id) AND v.status = 'ENFORCED'
  AND om.created_at <= sqlc.arg(older_than)::timestamptz
ORDER BY v.id;

-- name: OpenNoticeTerminations :many
-- 帳號上未結束的預告終止（termination_ground = NOTICE，ENFORCED，尚未結束：SCHEDULED 或 ACTIVE）；店家關閉時結束它們（條款 21.2）。
SELECT *
FROM seller.measures
WHERE account_id = $1 AND status = 'ENFORCED' AND kind = 'TERMINATION' AND termination_ground = 'NOTICE' AND ends_at IS NULL
ORDER BY id
FOR UPDATE;

-- name: ReviewMeasureAppealByIDs :many
-- 這些措施請求（申覆）的裁定開始時間與期限（資源的 decide_deadline）。規則只有 seller.review_measure_appeal(at)（objects.sql）；不是待處理的請求沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.request_id::uuid AS request_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_measure_appeal(sqlc.arg(at)::timestamptz) r
WHERE r.request_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ReviewMeasureLiftRequestByIDs :many
-- 這些措施請求（解除請求）的裁定開始時間與期限（資源的 decide_deadline）。規則只有 seller.review_measure_lift_request(at)（objects.sql）；
-- 不是待處理、或措施已不在生效中的請求沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.request_id::uuid AS request_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_measure_lift_request(sqlc.arg(at)::timestamptz) r
WHERE r.request_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ReviewMeasureReviewByIDs :many
-- 這些措施的複審開始時間與期限（資源的 review_deadline）。規則只有 seller.review_measure_review(at)（objects.sql）；不是風險類型、不在生效中、
-- 或還沒到 review_by 前 24 小時的措施沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.measure_id::uuid AS measure_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_measure_review(sqlc.arg(at)::timestamptz) r
WHERE r.measure_id = ANY(sqlc.arg(ids)::uuid[]);
