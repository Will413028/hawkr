-- seller 模組的 view、函式與複合型別（單體規則 6）。表、約束與索引在 schema.sql（Atlas 的 desired state，`atlas migrate diff`
-- 只讀它）；這個檔的物件 Atlas 社群版不處理（v1.3.3 在 desired state 裡遇到 view 直接報錯 "views are available to logged-in users only"），
-- 所以改這裡時以 `atlas migrate new <名稱>` 手寫 migration，再 `atlas migrate hash`。sqlc 與 TestSchemaFileMatchesMigrations 依序讀
-- schema.sql 與這個檔，兩者合起來才是 migration 套用後的完整結果。這裡只能有 view、函式與複合型別（internal/arch 的測試）。

-- 曝光規則唯一的家（ADR 2026-09-24-go-modular-monolith 文末修訂「跨模組讀取改用擁有者發布的 pub_*」的曝光規則段；簡報
-- docs/plans/2026-10-18-a7-social/d3-read-model-brief.md §4.1 (i)）。下面的物件一律以 `atlas migrate new` 手寫 migration
-- （單體規則 6，見檔頭），漏建由 TestSchemaFileMatchesMigrations 擋；函式一律 BEGIN ATOMIC，
-- 底層欄位改型別或刪除時 Postgres 拒絕（依賴追蹤），要在同一份 migration 內 drop 再建。時間一律由呼叫方以參數傳入（請求的 clock），不用 now()。

-- 店家與時間無關的條件，一欄一個布林（Go 的 model.Requirement 一項對一欄，Go 只讀這裡、不重算；停權與時間有關，在 effective_measures）：
--   - active：沒有關閉；not_pending_deletion：沒有正在隨帳號刪除；
--   - kyc_approved：平台審核已核准（有一筆 KYC 申請是 APPROVED）；transfers_active：收款帳戶的轉帳能力啟用（payout_capabilities.transfers_active：payment 模組
--     經 PayoutAccountChanged 複製過來的副本，Stripe 計畫 D10、D20、D28）；onboarding_complete 是這兩欄的 AND（D31 Q3：開通任務清單與能力推導都讀這兩欄，
--     pub_exposable_stores 讀 onboarding_complete，所以任務、閘門與曝光同源）；
--   - has_store_name、has_logo、has_town_code：有值。文字欄位不存空字串（stores_store_name_check、stores_images_check、stores_town_code_check），
--     店名也不存頭尾是 Unicode 空白的值（stores_store_name_check，platform/trimtext.Check，與寫入端的 trimtext.Trim 同一個集合），所以有值就是 NOT NULL；店名只有個資清理過的列是 NULL。
CREATE VIEW seller.store_requirement_flags AS
SELECT s.store_id,
       s.account_id,
       s.status <> 'CLOSED' AS active,
       s.status <> 'PENDING_DELETION' AS not_pending_deletion,
       (c.kyc_approved AND c.transfers_active) AS onboarding_complete,
       (s.store_name IS NOT NULL)::boolean AS has_store_name,
       (s.logo_key IS NOT NULL)::boolean AS has_logo,
       (s.town_code IS NOT NULL)::boolean AS has_town_code,
       c.kyc_approved,
       c.transfers_active
FROM seller.stores s
CROSS JOIN LATERAL (
    SELECT EXISTS (
               SELECT 1 FROM seller.kyc_applications a WHERE a.store_id = s.store_id AND a.status = 'APPROVED'
           ) AS kyc_approved,
           EXISTS (
               SELECT 1 FROM seller.payout_capabilities p WHERE p.store_id = s.store_id AND p.transfers_active
           ) AS transfers_active
) c;

-- 生效中的措施的列形狀：只放判斷與 join 需要的鍵。不回傳 SETOF seller.measures：measures 加任何欄位後，回傳整列的函式在內聯時就失敗
-- （return type mismatch），每次改 measures 都得重建函式。
CREATE TYPE seller.effective_measure AS (id UUID, account_id BIGINT);

-- 「措施生效中」唯一的定義：未撤銷的停權或終止合作措施，已經開始（starts_at 不晚於 at），而且沒有固定期間（ends_at 為空）或 ends_at 晚於 at。
-- 帳號是措施自己的 account_id，帳號任何一個 stint 上的措施都算（含已關閉的）。EffectiveMeasures、EffectiveMeasuresAll 與 pub_exposable_stores
-- 都讀它；預定生效（ScheduledMeasureIDs）與 model.StateOf 是另外的寫法，由資料集測試鎖住。
-- 寫成單一 SELECT（不用 UNION ALL）：內聯之後是 measures 上的一個掃描，外層的帳號條件（`e.account_id = f.account_id` 這種關聯條件，
-- 或 `= ANY($ids)`）成為 measures_enforced_suspension 的 Index Cond；UNION ALL 的兩段是子查詢，關聯條件推不進去，每個探測都重掃全平台
-- 生效中的措施。全部帳號的讀取由 planner 把 OR 的兩邊各配一個部分索引（BitmapOr：measures_active_timed 的 ends_at > at、
-- measures_active_open）。計畫由 TestExposableStoresJoinProbesMeasuresByAccount 與 TestEffectiveMeasuresReadsScaleWithTheEffectiveSetNotTheHistory 鎖住。
CREATE FUNCTION seller.effective_measures(at TIMESTAMPTZ) RETURNS SETOF seller.effective_measure
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT m.id, m.account_id
    FROM seller.measures m
    WHERE m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at <= at AND (m.ends_at IS NULL OR m.ends_at > at);
END;

-- 「帶身分效果的措施」唯一的定義：停權，以及非預告的終止合作（含關聯帳號的終止）；預告終止（NOTICE）不禁止日後重新申請，不帶身分效果。
-- 與時間無關（是否生效另由 effective_measures 決定）。快照的寫入（SnapshotMeasureIdentities）與命中的讀取（measure_identity_sources）都讀它，Go 不判斷。
CREATE VIEW seller.identity_effect_measures AS
SELECT m.id, m.account_id
FROM seller.measures m
WHERE m.kind = 'SUSPENSION' OR (m.kind = 'TERMINATION' AND m.termination_ground <> 'NOTICE');

-- 措施在 at 帶身分效果的 hash 來源，唯一的定義：措施在 effective_measures(at) 而且帶身分效果（identity_effect_measures），來源是
-- (a) 措施的身分快照（措施建立時從被處分帳號的 account_identity_hashes 複製；保存期過後帳號的 hash 被刪，快照仍在）與
-- (b) 被處分帳號此刻還在 account_identity_hashes 的 hash（措施建立之後才同步進來的 fingerprint 與統編）。
-- kind 是快照的種類（統編 TAX_ID、provider 回報的銀行帳號 fingerprint 是 BANK_ACCOUNT）；created_at 只有 (b) 有值（hash 的寫入時間），(a) 是 NULL。
-- 同一個 hash 同時在兩個來源時有兩列，讀取方以 DISTINCT 或 min 聚合。
-- 寫成頂層的 UNION ALL、兩段都是單純的連接（沒有 LATERAL）：內聯之後呼叫方的 hash = ANY(...) 與 measure_id = ... 推進兩段，各自走
-- account_identity_hashes_hash、measure_identities 的索引，不必探測每一筆生效中的措施。計畫由 TestMeasureIdentityMatchesUsesTheHashIndexes 鎖住。
CREATE TYPE seller.measure_identity_source AS (measure_id UUID, account_id BIGINT, kind TEXT, key_id TEXT, hash BYTEA, created_at TIMESTAMPTZ);

CREATE FUNCTION seller.measure_identity_sources(at TIMESTAMPTZ) RETURNS SETOF seller.measure_identity_source
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT i.id, i.account_id, mi.kind, mi.key_id, mi.hash, NULL::timestamptz
    FROM seller.measure_identities mi
    JOIN seller.identity_effect_measures i ON i.id = mi.measure_id
    JOIN seller.effective_measures(at) e ON e.id = i.id
    UNION ALL
    SELECT i.id, i.account_id, CASE o.field WHEN 'TAX_ID' THEN 'TAX_ID' ELSE 'BANK_ACCOUNT' END, o.key_id, o.hash, o.created_at
    FROM seller.account_identity_hashes o
    JOIN seller.identity_effect_measures i ON i.account_id = o.account_id
    JOIN seller.effective_measures(at) e ON e.id = i.id;
END;

-- 命中，唯一的定義：measure_identity_sources(at) 的來源與 account_identity_hashes 裡 (hash, key_id) 相同的列，種類要對得上（統編 TAX_ID 對 TAX_ID，
-- BANK_FINGERPRINT 對 BANK_ACCOUNT）。一列是一把金鑰的一筆命中（同一個帳號、同一個欄位可有多列），created_at 是兩邊 hash 都存在的時刻：
-- 快照來源是對方 hash 的寫入時間，目前 hash 來源是兩筆寫入時間較晚的。措施不在 at 生效或不帶身分效果時沒有列。
-- 不排除被處分的帳號本身（它自己的 hash 也會命中自己）：呼叫方自己加條件。LinkedAccountHits（處分頁的關聯帳號）、BankFingerprintHitExists 與
-- review_shared_bank_account（待辦）都讀它，命中的條件只寫這一份；以 (hash, key_id) 的索引等值連接，不解密。
CREATE TYPE seller.measure_identity_hit AS (measure_id UUID, account_id BIGINT, field TEXT, created_at TIMESTAMPTZ);

CREATE FUNCTION seller.measure_identity_hits(at TIMESTAMPTZ) RETURNS SETOF seller.measure_identity_hit
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT s.measure_id, f.account_id, f.field, GREATEST(f.created_at, s.created_at)
    FROM seller.measure_identity_sources(at) s
    JOIN seller.account_identity_hashes f ON f.hash = s.hash AND f.key_id = s.key_id
    WHERE s.kind = CASE f.field WHEN 'TAX_ID' THEN 'TAX_ID' ELSE 'BANK_ACCOUNT' END;
END;

-- 發布的讀取介面（pub_*，形狀宣告在 db/pub.sql，契約測試比對兩者）：at 這一刻可以公開曝光的店家。條件是 store_requirement_flags 全部為真，
-- 而且帳號在 at 沒有生效中的措施（effective_measures）。「至少一個上架商品」不在這裡，由曝光端檢查。依賴表允許的上層模組在自己的 SQL 裡
-- join 它（例如 `EXISTS (SELECT 1 FROM seller.pub_exposable_stores($now) e WHERE e.store_id = p.store_id)`）；seller 自己的 exposure.Classify
-- 也讀它。函式會被內聯，沒有 Function Scan。
CREATE TYPE seller.pub_exposable_store AS (store_id BIGINT);

CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT f.store_id
    FROM seller.store_requirement_flags f
    WHERE f.active AND f.not_pending_deletion AND f.onboarding_complete AND f.has_store_name AND f.has_logo AND f.has_town_code
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = f.account_id);
END;

-- 審閱政策唯一的家（step10e-3d-design.md E10、§3.1）：營運要處理的事項，每種一支 seller.review_<種類>(at)。每一列是一件待辦：種類、對象的鍵、店家、
-- 開始時間與期限；成員的條件、開始時間與期限的時長只寫在這裡。後台待辦（pub_review_items 是它們的 UNION ALL）、資源輸出的 *_deadline 與複審到期的收件匣
-- 都讀這些函式，Go 不寫時長。Go 沒有第二份成員規則；成員與指令真正用的 Go 定義
-- （Report.State()、Case.Open()、CaseInquiry.Open() 加 due_at 等）一致由資料集測試鎖住。時長是上線前的起始值（rb R3、3b D8），一律日曆時間（開始時間由各種類定義：KYC 從可以裁定時起算，店家不是營運中或帳號停權時暫停，恢復後重新起算），上線後依 rb R3 的觸發校準；
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
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

-- REPORT_TRIAGE 待分案：沒有案件也沒有結果（Report.State() 是 AWAITING_TRIAGE）。開始是建立時間；期限是通報人選了食安的 4 小時，其他類別 72 小時。
CREATE FUNCTION seller.review_report_triage(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'REPORT_TRIAGE'::text, r.id::text, r.id, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, r.store_id, COALESCE(s.store_name, ''),
           r.created_at,
           r.created_at + CASE WHEN r.category = 'FOOD_SAFETY' THEN INTERVAL '4 hours' ELSE INTERVAL '72 hours' END
    FROM seller.reports r
    JOIN seller.stores s ON s.store_id = r.store_id
    WHERE r.case_id IS NULL AND r.outcome IS NULL;
END;

-- CASE_DECISION 待決定：進行中而且沒有還沒結束的詢問（Case.Open() 而且不在等賣家）。開始是 max(開案時間, 最後一筆詢問的回覆收到時間, 最後一筆管理員撤回的時間)，
-- 系統撤回（案件結案）不算（規則只在這裡）；期限依嚴重度：嚴重 24 小時、高 72 小時、中 168 小時、低 336 小時。
-- 新增嚴重度而這裡沒有時長時，期限是 NULL（測試要失敗）。
CREATE FUNCTION seller.review_case_decision(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'CASE_DECISION'::text, c.id::text, NULL::uuid, c.id, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, c.store_id, COALESCE(s.store_name, ''),
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

-- CASE_INQUIRY_OVERDUE 詢問逾期：還沒結束的詢問，詢問的期限（due_at）已到（CaseInquiry.Open() 而且 due_at <= at）。營運要跟進的期限：
-- 開始是詢問的 due_at，期限再 24 小時。這是營運的義務；詢問自己的期限（賣家回覆的 create_time 到 due_at）是存的，不在這裡。
CREATE FUNCTION seller.review_case_inquiry_overdue(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'CASE_INQUIRY_OVERDUE'::text, i.id::text, NULL::uuid, i.case_id, i.id, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, i.store_id, COALESCE(s.store_name, ''),
           i.due_at, i.due_at + INTERVAL '24 hours'
    FROM seller.case_inquiries i
    JOIN seller.stores s ON s.store_id = i.store_id
    WHERE i.reply_received_at IS NULL AND i.withdrawn_at IS NULL AND i.due_at <= at;
END;

-- AUTHORITY_ORDER 主管機關來函：還沒完成也沒撤銷（AuthorityOrder.Open）。開始是收文時間，期限是登記時存下的 due_at。來函沒有單一店家。
CREATE FUNCTION seller.review_authority_order(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'AUTHORITY_ORDER'::text, o.id::text, NULL::uuid, NULL::uuid, NULL::uuid, o.id, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, NULL::uuid, NULL::bigint, NULL::text,
           o.received_at, o.due_at
    FROM seller.authority_orders o
    WHERE o.completed_at IS NULL AND o.revoked_at IS NULL;
END;

-- AUTHORITY_TARGET_NOTICE 告知被指名者：來函的對象，告知是暫不告知，而且確認日期（recheck_on）不晚於 at 的台北日期
-- （告知是 WITHHELD）。開始是確認日期的台北 00:00，期限再 72 小時。店家是對象指名的店家；對象指名整個帳號時沒有店家。
CREATE FUNCTION seller.review_authority_target_notice(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'AUTHORITY_TARGET_NOTICE'::text, t.order_id::text || '/' || lpad(t.position::text, 5, '0'),
           NULL::uuid, NULL::uuid, NULL::uuid, t.order_id, t.position, t.account_id,
           NULL::uuid, NULL::uuid, NULL::uuid, t.store_id, CASE WHEN s.store_id IS NULL THEN NULL ELSE COALESCE(s.store_name, '') END,
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
           q.measure_id, q.id, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
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
           q.measure_id, q.id, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
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
           m.id, NULL::uuid, NULL::uuid, m.store_id, COALESCE(s.store_name, ''),
           m.review_by - INTERVAL '24 hours', m.review_by
    FROM seller.measures m
    JOIN seller.stores s ON s.store_id = m.store_id
    WHERE m.basis = 'RISK' AND m.review_by IS NOT NULL AND m.review_by - INTERVAL '24 hours' <= at
      AND EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.id = m.id);
END;

-- 帳號最近一次停權結束的時間：帳號在 at 以前已經結束的停權與終止合作措施的結束時間的最大值（到期是 ends_at，撤銷是 revoked_at，兩者都有時取較早的）；
-- 只算真的生效過的（結束時間晚於 starts_at：還沒開始就被撤銷的不算）；沒有時是 NULL。「措施生效中」（effective_measures）的第五種讀法
-- （前四種是 EffectiveMeasures、EffectiveMeasuresAll、ScheduledMeasureIDs 與 Go 的 StateOf，見 AGENTS.md），只給 review_kyc_application 算停權期間不計的審核期限起點，
-- 由資料集測試在邊界（ends_at、revoked_at 等於 at 的瞬間）與 effective_measures 鎖住。
CREATE FUNCTION seller.account_sanction_lifted_at(lifted_account_id BIGINT, at TIMESTAMPTZ) RETURNS TIMESTAMPTZ
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT max(least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END))
    FROM seller.measures m
    WHERE m.account_id = lifted_account_id AND m.kind IN ('SUSPENSION', 'TERMINATION')
      AND least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END) <= at
      AND least(m.ends_at, CASE WHEN m.status = 'REVOKED' THEN m.revoked_at END) > m.starts_at;
END;

-- KYC_APPLICATION 審核商家認證：審核中（SUBMITTED）、店家營運中（已關閉與等待刪除時審核暫停）、帳號沒有生效中的停權（停權時不能核准，審核暫停；退回雖然可用，件不在待辦）。
-- 成員對應的 Go 定義是 APPROVE_KYC 的原因清單為空（model.KYCActionUnavailableReasons）：身分命中別的帳號生效中的處分不在原因裡，照舊留在待辦讓員工退回；
-- 商家清單的「待審核」分頁與統計讀這個函式，所以待審核只有這一個定義。開始是 max(送出時間, 這個 stint 最近一次恢復營業的時間, 帳號最近一次停權結束的時間)，
-- 暫停的期間不算審核期限，期限 168 小時。
CREATE FUNCTION seller.review_kyc_application(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'KYC_APPLICATION'::text, k.application_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, NULL::bigint,
           NULL::uuid, NULL::uuid, k.application_id, k.store_id, COALESCE(s.store_name, ''),
           greatest(k.submitted_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = k.store_id), seller.account_sanction_lifted_at(s.account_id, at)),
           greatest(k.submitted_at, (SELECT max(c.reopened_at) FROM seller.store_closures c WHERE c.store_id = k.store_id), seller.account_sanction_lifted_at(s.account_id, at)) + INTERVAL '168 hours'
    FROM seller.kyc_applications k
    JOIN seller.stores s ON s.store_id = k.store_id
    WHERE k.status = 'SUBMITTED' AND s.status = 'ACTIVE'
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = s.account_id);
END;

-- SHARED_BANK_ACCOUNT 共用銀行帳號：處分（在 effective_measures(at)）建立之後，別的帳號才同步了與被處分帳號相同的銀行帳號 fingerprint
-- （measure_identity_hits(at) 的 BANK_FINGERPRINT 命中：這一對全部命中列裡最早的 fingerprint 寫入時間晚於處分的建立時間；最早的命中早於或等於處分建立的，處分頁的關聯帳號已看得到，
-- 之後關店重開的新 stint 或補金鑰多出較晚的列也不進待辦）。
-- 一對（處分、命中的帳號）一列，多把金鑰不重複；命中的帳號自己有生效中的處分時不列；這一對已有 measure_linked_reviews（已檢視）時不列。
-- 開始是這一對全部銀行 fingerprint 命中列的最早寫入時間，期限 168 小時。店家是命中帳號最近的 stint（store_id 最大的，含已關閉；與處分頁關聯帳號的店家相同），
-- 對象的鍵是 measure_id/account_id（item_id 同種類內唯一）。只列出、不自動處分。
CREATE FUNCTION seller.review_shared_bank_account(at TIMESTAMPTZ) RETURNS SETOF seller.review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT 'SHARED_BANK_ACCOUNT'::text, p.measure_id::text || '/' || p.account_id::text, NULL::uuid, NULL::uuid, NULL::uuid, NULL::uuid, NULL::smallint, p.account_id,
           p.measure_id, NULL::uuid, NULL::uuid, st.store_id, COALESCE(st.store_name, ''),
           p.start_at, p.start_at + INTERVAL '168 hours'
    FROM (
        SELECT m.id AS measure_id, h.account_id, min(h.created_at) AS start_at
        FROM seller.effective_measures(at) e
        JOIN seller.measures m ON m.id = e.id
        JOIN seller.measure_identity_hits(at) h ON h.measure_id = m.id
        WHERE h.field = 'BANK_FINGERPRINT' AND h.account_id <> m.account_id
          AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) x WHERE x.account_id = h.account_id)
          AND NOT EXISTS (SELECT 1 FROM seller.measure_linked_reviews r WHERE r.measure_id = m.id AND r.account_id = h.account_id)
        GROUP BY m.id, m.created_at, h.account_id
        HAVING min(h.created_at) > m.created_at
    ) p
    CROSS JOIN LATERAL (
        SELECT s.store_id, s.store_name
        FROM seller.stores s
        WHERE s.account_id = p.account_id
        ORDER BY s.store_id DESC
        LIMIT 1
    ) st;
END;

-- 發布的讀取介面（step10e-3d-design.md §2.1）：at 這一刻營運要處理的事項，後台待辦（reviewqueue 模組）的 seller 部分。每一列是一件待辦：種類、對象的鍵、
-- 店家、開始時間與期限，欄位的意義同 review_item。10 種都有期限，due_at 一律有值（E3）。是 10 支 review_<種類>(at) 的 UNION ALL：成員的條件與時長只寫在那些
-- 函式（規則只有一個家，E10），這裡不重寫；成員與指令用的 Go 定義一致由資料集測試鎖住。時間由呼叫方以請求的 clock 傳入，不用 now()。
-- 消費端排序與分頁（reviewqueue 的 keyset：due_at、start_at、kind COLLATE "C"、item_id COLLATE "C"）；kind 是 UPPER_SNAKE，item_id 的位元組順序穩定。
CREATE TYPE seller.pub_review_item AS (
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
    kyc_application_id UUID,
    store_id BIGINT,
    store_name TEXT,
    start_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ
);

CREATE FUNCTION seller.pub_review_items(at TIMESTAMPTZ) RETURNS SETOF seller.pub_review_item
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT * FROM seller.review_report_triage(at)
    UNION ALL
    SELECT * FROM seller.review_case_decision(at)
    UNION ALL
    SELECT * FROM seller.review_case_inquiry_overdue(at)
    UNION ALL
    SELECT * FROM seller.review_authority_order(at)
    UNION ALL
    SELECT * FROM seller.review_authority_target_notice(at)
    UNION ALL
    SELECT * FROM seller.review_measure_appeal(at)
    UNION ALL
    SELECT * FROM seller.review_measure_lift_request(at)
    UNION ALL
    SELECT * FROM seller.review_measure_review(at)
    UNION ALL
    SELECT * FROM seller.review_kyc_application(at)
    UNION ALL
    SELECT * FROM seller.review_shared_bank_account(at);
END;

-- 發布的讀取介面（pub_*，形狀宣告在 db/pub.sql，契約測試比對兩者）：此刻生效中的內容下架措施的對象，一列是一個（措施，內容）。內容下架建立即生效、
-- 沒有結束時間，生效與否只看 status，所以不收 at。給 social 的每日對帳核對它自己的複本 social.content_takedowns 有沒有漏或多（Go 查詢；
-- social 的讀取不 join 它，D-sync）。
CREATE TYPE seller.pub_content_takedown AS (measure_id UUID, content_kind TEXT, content_id UUID);

CREATE FUNCTION seller.pub_content_takedowns() RETURNS SETOF seller.pub_content_takedown
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT c.measure_id, c.content_kind, c.content_id
    FROM seller.measures m
    JOIN seller.measure_contents c ON c.measure_id = m.id
    WHERE m.status = 'ENFORCED' AND m.kind = 'CONTENT_TAKEDOWN';
END;
