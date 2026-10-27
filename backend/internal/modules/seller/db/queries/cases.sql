-- name: InsertCase :one
-- 開案（分案或移動時，這家店的這個類別沒有進行中的案件）；cases_one_open 是保底：呼叫方已鎖住店家列，同一家店的分案、移動、結案彼此串行。
INSERT INTO seller.cases (id, store_id, category, severity, opened_at, opened_by_admin_id, updated_at)
VALUES (sqlc.arg(id), sqlc.arg(store_id), sqlc.arg(category), sqlc.arg(severity), sqlc.arg(now), sqlc.arg(admin_id), sqlc.arg(now))
RETURNING *;

-- name: CaseByID :one
SELECT *
FROM seller.cases
WHERE id = $1;

-- name: LockCase :one
SELECT *
FROM seller.cases
WHERE id = $1
FOR UPDATE;

-- name: LockCases :many
-- 依 id 升序鎖定一批案件（鎖順序：店家列、通報列、案件列（id 升序）、措施列）。
SELECT *
FROM seller.cases
WHERE id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY id
FOR UPDATE;

-- name: LockOpenCase :one
-- 鎖定這家店這個類別進行中的案件（沒有時 0 列）。
SELECT *
FROM seller.cases
WHERE store_id = sqlc.arg(store_id) AND category = sqlc.arg(category) AND closed_at IS NULL
FOR UPDATE;

-- name: OpenCaseOf :one
-- 這家店這個類別進行中的案件（不鎖；呼叫方已鎖住店家列，所有改案件的指令都先鎖店家，所以讀到之後不會被別人關閉或另開）。
SELECT *
FROM seller.cases
WHERE store_id = sqlc.arg(store_id) AND category = sqlc.arg(category) AND closed_at IS NULL;

-- name: OpenCasesOfStore :many
-- 這家店全部進行中的案件（每個類別最多一個；不鎖，同 OpenCaseOf）：分案與移動的去處。
SELECT *
FROM seller.cases
WHERE store_id = sqlc.arg(store_id) AND closed_at IS NULL;

-- name: CaseReportIDs :many
-- 案件的成員，舊到新。
SELECT id
FROM seller.reports
WHERE case_id = $1
ORDER BY created_at, id;

-- name: BumpCase :exec
-- 案件層級的處理（成員進出、成員有了結果、記錄轉報）：版本加一，updated_at 是上一次處理的時間。
UPDATE seller.cases
SET version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id);

-- name: DecideCase :one
-- 決定即結案；只在進行中、版本相符時寫入，否則 0 列。
UPDATE seller.cases
SET closed_at = sqlc.arg(now)::timestamptz,
    closed_by_admin_id = sqlc.arg(admin_id)::bigint,
    close_reason = 'DECIDED',
    decision = sqlc.arg(decision)::text,
    decision_note = sqlc.narg(note),
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)::bigint AND closed_at IS NULL
RETURNING *;

-- name: CloseEmptyCase :one
-- 通報都移走的案件以 NO_REPORTS 自動關閉（不算決定）；案件還有成員或已結案時 0 列。呼叫方已鎖住案件列。
UPDATE seller.cases c
SET closed_at = sqlc.arg(now)::timestamptz,
    closed_by_admin_id = sqlc.arg(admin_id)::bigint,
    close_reason = 'NO_REPORTS',
    version = c.version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE c.id = sqlc.arg(id) AND c.closed_at IS NULL AND NOT EXISTS (SELECT 1 FROM seller.reports r WHERE r.case_id = c.id)
RETURNING *;

-- name: ChangeCaseSeverity :one
-- 改嚴重度；只在進行中、版本相符時寫入，否則 0 列。
UPDATE seller.cases
SET severity = sqlc.arg(severity)::text,
    version = version + 1,
    updated_at = sqlc.arg(now)::timestamptz
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version)::bigint AND closed_at IS NULL
RETURNING *;

-- name: CasesByIDs :many
-- 一批案件加上店名（個資清理之後是 NULL）、店家是否已關閉與個資是否已清理。
SELECT sqlc.embed(c),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (s.pii_purged_at IS NOT NULL)::bool AS store_purged
FROM seller.cases c
JOIN seller.stores s ON s.store_id = c.store_id
WHERE c.id = ANY(sqlc.arg(ids)::uuid[])
ORDER BY c.opened_at DESC, c.id DESC;

-- name: AdminCases :many
-- admin 的案件清單：開案時間新到舊（keyset），篩選條件為 NULL 時不篩；state 是 OPEN 或 CLOSED（與 model.Case.Open 同一個定義）；
-- awaiting_seller 為 true 只列進行中而且有還沒結束的詢問的案件（與 model.Case.AwaitingSeller、OpenCaseInquiries 同一個定義）。
SELECT sqlc.embed(c),
       s.store_name AS store_name,
       (s.status = 'CLOSED')::bool AS store_closed,
       (s.pii_purged_at IS NOT NULL)::bool AS store_purged
FROM seller.cases c
JOIN seller.stores s ON s.store_id = c.store_id
WHERE (sqlc.narg(state)::text IS NULL
    OR (sqlc.narg(state)::text = 'OPEN' AND c.closed_at IS NULL)
    OR (sqlc.narg(state)::text = 'CLOSED' AND c.closed_at IS NOT NULL))
  AND (sqlc.narg(category)::text IS NULL OR c.category = sqlc.narg(category)::text)
  AND (sqlc.narg(store_id)::bigint IS NULL OR c.store_id = sqlc.narg(store_id)::bigint)
  AND (NOT sqlc.arg(awaiting_seller)::bool OR (c.closed_at IS NULL AND EXISTS (
        SELECT 1 FROM seller.case_inquiries i WHERE i.case_id = c.id AND i.reply_received_at IS NULL AND i.withdrawn_at IS NULL)))
  AND (
    sqlc.narg(after_opened_at)::timestamptz IS NULL OR
    (c.opened_at, c.id) < (sqlc.narg(after_opened_at)::timestamptz, sqlc.narg(after_id)::uuid)
  )
ORDER BY c.opened_at DESC, c.id DESC
LIMIT sqlc.arg(page_size);

-- name: CaseReportCounts :many
-- 每個案件的成員數、還沒有結果的成員數、結果是已採取措施的成員數，與有補充還沒被管理員看過的成員數（還沒有結果，而且存在編號大於通報的 supplements_seen_seq 的補充；與 model.Report.HasUnseenSupplements 同一個定義）。
SELECT c.id AS case_id,
       agg.report_count,
       agg.unresolved_count,
       agg.action_taken_count,
       (SELECT count(*) FROM seller.reports r WHERE r.case_id = c.id AND r.outcome IS NULL AND EXISTS (
            SELECT 1 FROM seller.report_supplements p
            WHERE p.report_id = r.id AND p.seq > r.supplements_seen_seq))::int AS unseen_supplement_reports
FROM seller.cases c
CROSS JOIN LATERAL (
    SELECT count(*)::int AS report_count,
           (count(*) FILTER (WHERE r.outcome IS NULL))::int AS unresolved_count,
           (count(*) FILTER (WHERE r.outcome = 'ACTION_TAKEN'))::int AS action_taken_count
    FROM seller.reports r
    WHERE r.case_id = c.id
) agg
WHERE c.id = ANY(sqlc.arg(case_ids)::uuid[]);

-- name: CaseMeasureCounts :many
-- 每個案件導出的措施數（不論狀態）與其中未撤銷的數目。措施 m 是案件 C 導出的 ⇔ m.source_case_id = C，或 m 依據的認定（含引用的既有認定）以 C 的通報為來源
-- （measure_findings → finding_reports → reports.case_id）。這個定義有兩處 SQL：這裡與 ListMeasures 的 source_case_id 篩選（measures.sql），由資料集測試鎖住一致。
-- 沒有導出措施的案件不出現在結果裡（呼叫方當成 0）。
SELECT d.case_id,
       count(*)::int AS resulting_measures,
       (count(*) FILTER (WHERE d.status = 'ENFORCED'))::int AS unrevoked_measures
FROM (
    SELECT m.source_case_id AS case_id, m.id AS measure_id, m.status
    FROM seller.measures m
    WHERE m.source_case_id = ANY(sqlc.arg(case_ids)::uuid[])
    UNION
    SELECT r.case_id, m.id AS measure_id, m.status
    FROM seller.reports r
    JOIN seller.finding_reports fr ON fr.report_id = r.id
    JOIN seller.measure_findings mf ON mf.violation_id = fr.violation_id
    JOIN seller.measures m ON m.id = mf.measure_id
    WHERE r.case_id = ANY(sqlc.arg(case_ids)::uuid[])
) d
GROUP BY d.case_id;

-- name: InsertReportCaseMove :exec
INSERT INTO seller.report_case_moves (id, report_id, store_id, from_case_id, to_case_id, admin_id, moved_at)
VALUES (sqlc.arg(id), sqlc.arg(report_id), sqlc.arg(store_id), sqlc.arg(from_case_id), sqlc.arg(to_case_id), sqlc.arg(admin_id), sqlc.arg(now));

-- name: CaseMoves :many
-- 移入或移出這個案件的移動，舊到新。
SELECT *
FROM seller.report_case_moves
WHERE from_case_id = $1 OR to_case_id = $1
ORDER BY moved_at, id;

-- name: InsertCaseReferral :one
-- 記錄轉報（outcome 為 REFERRED）：每個案件一列；已經有時（已轉報或不轉報）不寫入、回 0 列（不讓 transaction 失效）。
INSERT INTO seller.case_referrals (case_id, category, outcome, channel, reference_no, follow_up_on, follow_up_set_at, recorded_at, recorded_by_admin_id)
VALUES (sqlc.arg(case_id), sqlc.arg(category), 'REFERRED', sqlc.arg(channel), sqlc.narg(reference_no), sqlc.narg(follow_up_on),
        CASE WHEN sqlc.narg(follow_up_on)::date IS NULL THEN NULL ELSE sqlc.arg(now)::timestamptz END, sqlc.arg(now), sqlc.arg(admin_id))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: InsertCaseReferralWaiver :one
-- 標記不轉報（outcome 為 WAIVED）：和轉報同一列，所以互斥；已經有時不寫入、回 0 列。
INSERT INTO seller.case_referrals (case_id, category, outcome, waiver_reason, recorded_at, recorded_by_admin_id)
VALUES (sqlc.arg(case_id), sqlc.arg(category), 'WAIVED', sqlc.arg(reason), sqlc.arg(now), sqlc.arg(admin_id))
ON CONFLICT DO NOTHING
RETURNING *;

-- name: CaseReferrals :many
SELECT *
FROM seller.case_referrals
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[]);

-- name: InsertCaseReferralReply :execrows
-- 記下主管機關的回覆；outcome 取預設值 REFERRED，複合外鍵讓它只能掛在已轉報的列。同一位管理員的同一個 request_id 已經有時不寫入、回 0 列
-- （不讓 transaction 失效，呼叫方接著讀原本那一則比對內容）。
INSERT INTO seller.case_referral_replies (id, case_id, body, received_on, recorded_by_admin_id, recorded_at, request_id, request_hash)
VALUES (sqlc.arg(id), sqlc.arg(case_id), sqlc.arg(body), sqlc.arg(received_on), sqlc.arg(admin_id), sqlc.arg(now), sqlc.narg(request_id), sqlc.narg(request_hash))
ON CONFLICT (recorded_by_admin_id, request_id) DO NOTHING;

-- name: CaseReferralReplyByRequest :one
-- 冪等重送的查詢：範圍是 (管理員, request_id)。
SELECT *
FROM seller.case_referral_replies
WHERE recorded_by_admin_id = sqlc.arg(admin_id) AND request_id = sqlc.arg(request_id);

-- name: CaseReferralReplies :many
-- 一批案件的主管機關回覆，舊到新（依記下的時間）。
SELECT *
FROM seller.case_referral_replies
WHERE case_id = ANY(sqlc.arg(case_ids)::uuid[])
ORDER BY recorded_at, id;

-- name: SetReferralFollowUp :exec
-- 更新或清掉追蹤日期（只對已轉報的列；不轉報的列 CHECK 要求它是 NULL），同時記下設定的時間（清掉時一起清）：追蹤逾期只看這個時間之後的回覆。
UPDATE seller.case_referrals
SET follow_up_on = sqlc.narg(follow_up_on),
    follow_up_set_at = CASE WHEN sqlc.narg(follow_up_on)::date IS NULL THEN NULL ELSE sqlc.arg(now)::timestamptz END
WHERE case_id = sqlc.arg(case_id) AND outcome = 'REFERRED';

-- name: ReviewCaseDecisionByIDs :many
-- 這些案件的「待決定」開始時間與期限（資源的 decide_deadline）。規則只有 seller.review_case_decision(at)（objects.sql）；不是待決定的案件
-- （已結案、等賣家回覆）沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.case_id::uuid AS case_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_case_decision(sqlc.arg(at)::timestamptz) r
WHERE r.case_id = ANY(sqlc.arg(ids)::uuid[]);

-- name: ReviewCaseFDAReferralByIDs :many
-- 這些案件的「待轉報」開始時間與期限（資源的 fda_referral_deadline）。規則只有 seller.review_case_fda_referral(at)（objects.sql）；不是待轉報的案件沒有列。
-- 輸出欄位的轉型是給 sqlc 的（它不型別化複合型別函式的輸出欄位，沒有轉型會在生成時報 column does not exist）。
SELECT r.case_id::uuid AS case_id, r.start_at::timestamptz AS start_at, r.due_at::timestamptz AS due_at
FROM seller.review_case_fda_referral(sqlc.arg(at)::timestamptz) r
WHERE r.case_id = ANY(sqlc.arg(ids)::uuid[]);
