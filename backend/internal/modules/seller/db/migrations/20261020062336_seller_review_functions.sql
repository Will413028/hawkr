-- 審閱政策唯一的家（step10e-3d-design.md E10）：複合型別 seller.review_item 與每種待辦一支 seller.review_<種類>(at)。Atlas 社群版略過函式與複合型別，
-- 這支 migration 是手寫的（`atlas migrate new`，單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見 objects.sql。
-- 審閱政策唯一的家（step10e-3d-design.md E10、§3.1）：營運要處理的事項，每種一支 seller.review_<種類>(at)。每一列是一件待辦：種類、對象的鍵、店家、
-- 開始時間與期限；成員的條件、開始時間與期限的時長只寫在這裡。後台待辦（pub_review_items 是它們的 UNION ALL）、資源輸出的 *_deadline 與兩個收件匣
-- （複審到期、食品業者登錄待審核）都讀這些函式，Go 不寫時長。指令用的 Go 判定式（Report.AwaitingTriage、Case.AwaitingDecision、CaseInquiry.Overdue 等）
-- 照舊，成員與函式一致由資料集測試鎖住。時長是上線前的起始值（rb R3、3b D8），一律日曆時間、不暫停，上線後依 rb R3 的觸發校準；
-- 以小時寫（不寫 days）：interval 的 days 在有夏令時間的 session 時區會依日曆加，起始值要的是固定長度。
-- 欄位（所有種類同一個形狀，沒有的欄位 NULL）：kind 是 UPPER_SNAKE 的種類；item_id 是同一種類內唯一、位元組順序穩定的鍵（UUID 的 ::text；
-- 來函的對象是 order_id::text || '/' || lpad(position::text, 5, '0')）；store_id 與 store_name 在沒有單一店家時 NULL，店名已清理時 store_name 是空字串。
-- 每支都寫成單一 SELECT（不在函式內 UNION ALL）、BEGIN ATOMIC：呼叫方的 `WHERE <對象鍵> = ANY($ids)` 內聯後推進底層表（同 effective_measures）；
-- 批次讀只能以 UUID 的對象鍵過濾（item_id 是 ::text 的運算式，推進去也不是索引條件）。時間一律由呼叫方傳入，不用 now()。
CREATE TYPE seller.review_item AS (
    kind TEXT,
    item_id TEXT,
    report_id UUID,
    case_id UUID,
    inquiry_id UUID,
    authority_order_id UUID,
    target_position SMALLINT,
    account_id BIGINT,
    measure_id UUID,
    request_id UUID,
    food_registration_id UUID,
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

-- REPORT_TRIAGE 待分案：沒有案件也沒有結果（Report.AwaitingTriage）。開始是建立時間；期限是通報人選了食安的 4 小時，其他類別 72 小時。
CREATE FUNCTION seller.review_report_triage(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'REPORT_TRIAGE'::text, r.id::text, r.id, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, r.store_id, COALESCE(s.store_name, ''),
           r.created_at,
           r.created_at + CASE WHEN r.category = 'FOOD_SAFETY' THEN INTERVAL '4 hours' ELSE INTERVAL '72 hours' END
    FROM seller.reports r
    JOIN seller.stores s ON s.store_id = r.store_id
    WHERE r.case_id IS NULL AND r.outcome IS NULL;
END;

-- CASE_DECISION 待決定：進行中而且沒有還沒結束的詢問（Case.AwaitingDecision）。開始是 max(開案時間, 最後一筆詢問的回覆收到時間, 最後一筆管理員撤回的時間)，
-- 系統撤回（案件結案）不算（規則只在這裡）；期限依嚴重度：嚴重 24 小時、高 72 小時、中 168 小時、低 336 小時。
-- 新增嚴重度而這裡沒有時長時，期限是 NULL（測試要失敗）。
CREATE FUNCTION seller.review_case_decision(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'CASE_DECISION'::text, c.id::text, NULL::uuid, c.id, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, c.store_id, COALESCE(s.store_name, ''),
           k.start_at,
           k.start_at + CASE c.severity
               WHEN 'CRITICAL' THEN INTERVAL '24 hours'
               WHEN 'HIGH' THEN INTERVAL '72 hours'
               WHEN 'MEDIUM' THEN INTERVAL '168 hours'
               WHEN 'LOW' THEN INTERVAL '336 hours'
           END
    FROM seller.cases c
    JOIN seller.stores s ON s.store_id = c.store_id
    CROSS JOIN LATERAL (
        SELECT greatest(c.opened_at, max(i.reply_received_at), max(i.withdrawn_at) FILTER (WHERE i.withdraw_cause = 'ADMIN')) AS start_at
        FROM seller.case_inquiries i
        WHERE i.case_id = c.id
    ) k
    WHERE c.closed_at IS NULL
      AND NOT EXISTS (SELECT 1 FROM seller.case_inquiries o WHERE o.case_id = c.id AND o.reply_received_at IS NULL AND o.withdrawn_at IS NULL);
END;

-- CASE_INQUIRY_OVERDUE 詢問逾期：還沒結束的詢問，詢問的期限（due_at）已到（CaseInquiry.Overdue：due_at <= at）。營運要跟進的期限：
-- 開始是詢問的 due_at，期限再 24 小時。這是營運的義務；詢問自己的期限（賣家回覆的 create_time 到 due_at）是存的，不在這裡。
CREATE FUNCTION seller.review_case_inquiry_overdue(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'CASE_INQUIRY_OVERDUE'::text, i.id::text, NULL::uuid, i.case_id, i.id, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, i.store_id, COALESCE(s.store_name, ''),
           i.due_at, i.due_at + INTERVAL '24 hours'
    FROM seller.case_inquiries i
    JOIN seller.stores s ON s.store_id = i.store_id
    WHERE i.reply_received_at IS NULL AND i.withdrawn_at IS NULL AND i.due_at <= at;
END;

-- CASE_FDA_REFERRAL 待轉報：食安案件、決定是採取措施、還沒有轉報紀錄（CaseFacts.AwaitingReferral；標記不轉報也是一筆紀錄）。
-- 開始是結案（決定）時間，期限 24 小時。
CREATE FUNCTION seller.review_case_fda_referral(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'CASE_FDA_REFERRAL'::text, c.id::text, NULL::uuid, c.id, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, c.store_id, COALESCE(s.store_name, ''),
           c.closed_at, c.closed_at + INTERVAL '24 hours'
    FROM seller.cases c
    JOIN seller.stores s ON s.store_id = c.store_id
    WHERE c.category = 'FOOD_SAFETY' AND c.decision = 'ACTION_TAKEN'
      AND NOT EXISTS (SELECT 1 FROM seller.case_referrals f WHERE f.case_id = c.id);
END;

-- AUTHORITY_ORDER 主管機關來函：還沒完成也沒撤銷（AuthorityOrder.Open）。開始是收文時間，期限是登記時存下的 due_at。來函沒有單一店家。
CREATE FUNCTION seller.review_authority_order(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'AUTHORITY_ORDER'::text, o.id::text, NULL::uuid, NULL::uuid, NULL::uuid, o.id, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::bigint, NULL::text,
           o.received_at, o.due_at
    FROM seller.authority_orders o
    WHERE o.completed_at IS NULL AND o.revoked_at IS NULL;
END;

-- AUTHORITY_TARGET_NOTICE 告知被指名者：來函的對象，告知是暫不告知，而且確認日期（recheck_on）不晚於 at 的台北日期
-- （AuthorityOrderTarget.NoticeFollowUpDue）。開始是確認日期的台北 00:00，期限再 72 小時。店家是對象指名的店家；對象指名整個帳號時沒有店家。
CREATE FUNCTION seller.review_authority_target_notice(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'AUTHORITY_TARGET_NOTICE'::text, t.order_id::text || '/' || lpad(t.position::text, 5, '0'),
           NULL::uuid, NULL::uuid, NULL::uuid, t.order_id, t.position, t.account_id,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, t.store_id, CASE WHEN s.store_id IS NULL THEN NULL ELSE COALESCE(s.store_name, '') END,
           t.recheck_on::timestamp AT TIME ZONE 'Asia/Taipei',
           (t.recheck_on::timestamp AT TIME ZONE 'Asia/Taipei') + INTERVAL '72 hours'
    FROM seller.authority_order_targets t
    LEFT JOIN seller.stores s ON s.store_id = t.store_id
    WHERE t.notice = 'WITHHELD' AND t.recheck_on <= (at AT TIME ZONE 'Asia/Taipei')::date;
END;

-- MEASURE_APPEAL 裁定申覆：待處理的申覆（申覆不受措施是否生效中影響）。開始是提出時間，期限 168 小時。item_id 是請求的 ID。
CREATE FUNCTION seller.review_measure_appeal(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MEASURE_APPEAL'::text, q.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           q.measure_id, q.id, NULL::uuid, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
           q.submitted_at, q.submitted_at + INTERVAL '168 hours'
    FROM seller.measure_requests q
    JOIN seller.measures m ON m.id = q.measure_id
    JOIN seller.stores s ON s.store_id = m.store_id
    WHERE q.state = 'PENDING' AND q.kind = 'APPEAL';
END;

-- MEASURE_LIFT_REQUEST 裁定解除請求：待處理的解除請求，而且措施在 at 生效中（措施已不在生效中的解除請求推導為 CLOSED：Measure.EffectiveRequest、
-- Measure.PendingRemediation）。「生效中」只讀 effective_measures。開始是提出時間，期限 168 小時。
CREATE FUNCTION seller.review_measure_lift_request(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MEASURE_LIFT_REQUEST'::text, q.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           q.measure_id, q.id, NULL::uuid, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
           q.submitted_at, q.submitted_at + INTERVAL '168 hours'
    FROM seller.measure_requests q
    JOIN seller.measures m ON m.id = q.measure_id
    JOIN seller.stores s ON s.store_id = m.store_id
    WHERE q.state = 'PENDING' AND q.kind = 'REMEDIATION'
      AND EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.id = m.id);
END;

-- MEASURE_REVIEW 複審停權：風險類型（RISK）、生效中、有複審期限，而且 at 不早於 review_by 前 24 小時（3cd D′11）。開始是 review_by 前 24 小時，
-- 期限是 review_by；review_by 過了之後仍在，直到措施結束或撤銷。沒有 Go 判定式：複審沒有指令要用它。
CREATE FUNCTION seller.review_measure_review(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'MEASURE_REVIEW'::text, m.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           m.id, NULL::uuid, NULL::uuid, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
           m.review_by - INTERVAL '24 hours', m.review_by
    FROM seller.measures m
    JOIN seller.stores s ON s.store_id = m.store_id
    WHERE m.basis = 'RISK' AND m.review_by IS NOT NULL AND m.review_by - INTERVAL '24 hours' <= at
      AND EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.id = m.id);
END;

-- FOOD_REGISTRATION 審核食品業者登錄：現在可以裁定的（FoodRegistrationReview.CanApprove）：待審、店家營運中、店家目前的商家認證已核准；
-- 等商家認證的不在這裡。開始是 max(送出時間, 目前核准的商家認證的核准時間)（3b D8「從可以裁定時起算」），期限 168 小時。
CREATE FUNCTION seller.review_food_registration(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'FOOD_REGISTRATION'::text, f.id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, f.id, NULL::uuid, f.store_id, COALESCE(s.store_name, ''),
           greatest(f.submitted_at, k.decided_at), greatest(f.submitted_at, k.decided_at) + INTERVAL '168 hours'
    FROM seller.food_registrations f
    JOIN seller.stores s ON s.store_id = f.store_id
    JOIN seller.kyc_applications k ON k.store_id = f.store_id AND k.status = 'APPROVED'
    WHERE f.state = 'PENDING' AND s.status = 'ACTIVE';
END;

-- KYC_APPLICATION 審核商家認證：審核中（SUBMITTED）。開始是送出時間，期限 168 小時。
CREATE FUNCTION seller.review_kyc_application(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'KYC_APPLICATION'::text, k.application_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, k.application_id, k.store_id, COALESCE(s.store_name, ''),
           k.submitted_at, k.submitted_at + INTERVAL '168 hours'
    FROM seller.kyc_applications k
    JOIN seller.stores s ON s.store_id = k.store_id
    WHERE k.status = 'SUBMITTED';
END;
