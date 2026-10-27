-- ============================================================================
-- Hawkr 測試資料：優惠券 (hawkr_coupon)
-- ============================================================================
-- 測試優惠券模板 ID 範圍：50001~50099
-- 測試優惠券實例 ID 範圍：60001~60099
--
-- Enum 合法值（DB 儲存 value）：
--   CouponType: 1=CASH / 2=PERCENT
--   CouponTemplateStatus: 0=DRAFT / 1=ACTIVE / 2=INACTIVE / 3=EXPIRED / 4=REVOKED
--   CouponInstanceStatus: 0=UNUSED / 1=RESERVED / 2=USED / 3=EXPIRED / 4=REVOKED / 5=ANNUL
--   IssuerType: 1=ADMIN / 2=SELLER
--
-- 賣家 store_id：1009 (store_code 9, HAWK9), 1004 (store_code 4, HAWK4)；店家在單體的 seller schema，由 backend 的 make dev-seed 寫入
-- ============================================================================

BEGIN;

-- 清除舊測試資料
DELETE FROM coupon_instances
WHERE
    instance_id BETWEEN 60001 AND 60099;

DELETE FROM coupon_claims
WHERE
    template_id BETWEEN 50001 AND 50099;

DELETE FROM coupon_templates
WHERE
    template_id BETWEEN 50001 AND 50099;

DELETE FROM seller_coupon_quotas WHERE store_id IN (1009, 1004);

-- 優惠券模板
INSERT INTO
    coupon_templates (
        template_id,
        title,
        type,
        issuer_type,
        issuer_account_id,
        issuer_store_id,
        applicable_store_id,
        target_type,
        threshold_amount,
        discount_amount,
        discount_percent,
        max_discount_amount,
        start_at,
        end_at,
        usage_limit_per_account,
        total_issue_limit,
        issued_count,
        status
    )
VALUES
    -- 全站現金折扣 100 元（滿 500 折 100）
    (
        50001,
        '新會員首購優惠',
        '1',
        '1',
        1,
        NULL,
        NULL,
        '1',
        500,
        100,
        NULL,
        NULL,
        '2026-01-01 00:00:00',
        '2026-12-31 23:59:59',
        1,
        1000,
        2,
        '1'
    ),

-- 店家 1009 專屬折扣 50 元（滿 300 折 50）
(
    50002,
    '有機農場回饋券',
    '1',
    '2',
    NULL,
    1009,
    1009,
    '1',
    300,
    50,
    NULL,
    NULL,
    '2026-01-01 00:00:00',
    '2026-12-31 23:59:59',
    3,
    500,
    1,
    '1'
),

-- 百分比折扣（全站 9 折，上限 200 元）
(
    50003,
    '春季特惠 9 折券',
    '2',
    '1',
    1,
    NULL,
    NULL,
    '1',
    800,
    NULL,
    10,
    200,
    '2026-03-01 00:00:00',
    '2026-06-30 23:59:59',
    1,
    200,
    0,
    '1'
);

-- 優惠券實例（已發放給買家）
INSERT INTO
    coupon_instances (
        instance_id,
        template_id,
        owner_buyer_id,
        applicable_store_id,
        status
    )
VALUES
    -- owner_buyer_id 是 buyer_id，不是 account_id：本資料庫查不到單體的 buyer schema，照 01-account.sql 的規則
    -- （buyer_id = account_id + 50000）直接寫。
    -- 帳號 2001（buyer 52001）持有全站 100 元折扣券
    (60001, 50001, 52001, NULL, '0'),
    -- 帳號 2002（buyer 52002）持有全站 100 元折扣券
    (60002, 50001, 52002, NULL, '0'),
    -- 帳號 2001（buyer 52001）持有店家 1009 專屬 50 元券
    (60003, 50002, 52001, 1009, '0');

-- 領取記錄（claim_required=true 的 template 必須有對應 claim row，
-- 否則 validateCoupon 會 throw COUPON_NOT_CLAIMED）
-- 真實業務流程：claim API 在同 transaction 內 INSERT claims + instances；
-- seed 直造 instance 須補 claim 維持 fixture 一致
INSERT INTO
    coupon_claims (template_id, buyer_id)
VALUES
    (50001, 52001),
    (50001, 52002),
    (50002, 52001);

-- 賣家優惠券配額
INSERT INTO
    seller_coupon_quotas (
        store_id,
        max_template_count
    )
VALUES (1009, 5),
    (1004, 5);

COMMIT;