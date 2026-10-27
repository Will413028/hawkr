-- 曝光規則唯一的家搬進 seller 的 SQL（A′(i)，Will 2026-10-18；ADR 2026-09-24-go-modular-monolith 文末修訂的曝光規則段）：
-- 與時間無關的條件 store_requirement_flags（view）、「措施生效中」唯一的定義 effective_measures(at)、發布的讀取介面
-- pub_exposable_stores(at)。Atlas 社群版略過 view、函式與複合型別，這支 migration 是手寫的（`atlas migrate new`，單體規則 6）；
-- 定義與 schema.sql 相同（TestSchemaFileMatchesMigrations），註解見 schema.sql。
CREATE VIEW seller.store_requirement_flags AS
SELECT s.store_id,
       s.account_id,
       s.status <> 'CLOSED' AS active,
       s.status <> 'PENDING_DELETION' AS not_pending_deletion,
       EXISTS (
           SELECT 1
           FROM seller.kyc_applications a
           JOIN seller.payuni_registrations r ON r.store_id = a.store_id
           WHERE a.store_id = s.store_id AND a.status = 'APPROVED' AND r.bind_status = 'BOUND' AND r.outcome = 'APPROVED'
       ) AS kyc_approved,
       (s.store_name IS NOT NULL)::boolean AS has_store_name,
       (s.logo_key IS NOT NULL)::boolean AS has_logo,
       (s.town_code IS NOT NULL)::boolean AS has_town_code
FROM seller.stores s;

CREATE TYPE seller.effective_measure AS (id UUID, account_id BIGINT);

CREATE FUNCTION seller.effective_measures(at TIMESTAMPTZ) RETURNS SETOF seller.effective_measure
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT m.id, m.account_id
    FROM seller.measures m
    WHERE m.ends_at > at AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at <= at
    UNION ALL
    SELECT m.id, m.account_id
    FROM seller.measures m
    WHERE m.ends_at IS NULL AND m.status = 'ENFORCED' AND m.kind IN ('SUSPENSION', 'TERMINATION') AND m.starts_at <= at;
END;

CREATE TYPE seller.pub_exposable_store AS (store_id BIGINT);

CREATE FUNCTION seller.pub_exposable_stores(at TIMESTAMPTZ) RETURNS SETOF seller.pub_exposable_store
LANGUAGE sql STABLE
BEGIN ATOMIC
    SELECT f.store_id
    FROM seller.store_requirement_flags f
    WHERE f.active AND f.not_pending_deletion AND f.kyc_approved AND f.has_store_name AND f.has_logo AND f.has_town_code
      AND NOT EXISTS (SELECT 1 FROM seller.effective_measures(at) e WHERE e.account_id = f.account_id);
END;
