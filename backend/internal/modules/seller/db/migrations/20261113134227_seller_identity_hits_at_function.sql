-- 命中的定義涵蓋受處分帳號目前的雜湊（Stripe 計畫 D46、D47、步驟 8e）：view measure_identity_hits 換成帶時間的函式，來源是在 at 生效而且帶身分效果的措施的
-- 快照加上被處分帳號目前的 account_identity_hashes；「帶身分效果」的規則寫成 view identity_effect_measures，快照的寫入也讀它。
-- 手寫的 migration（單體規則 6）；定義與 objects.sql 相同（TestSchemaFileMatchesMigrations），註解見該檔。依賴它的函式先 drop 再建。
DROP FUNCTION seller.pub_review_items(TIMESTAMPTZ);
DROP FUNCTION seller.review_shared_bank_account(TIMESTAMPTZ);
DROP VIEW seller.measure_identity_hits;

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
