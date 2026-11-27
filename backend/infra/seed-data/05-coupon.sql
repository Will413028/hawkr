
BEGIN;

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

INSERT INTO
    coupon_instances (
        instance_id,
        template_id,
        owner_buyer_id,
        applicable_store_id,
        status
    )
VALUES
    (60001, 50001, 52001, NULL, '0'),
    (60002, 50001, 52002, NULL, '0'),
    (60003, 50002, 52001, 1009, '0');

INSERT INTO
    coupon_claims (template_id, buyer_id)
VALUES
    (50001, 52001),
    (50001, 52002),
    (50002, 52001);

INSERT INTO
    seller_coupon_quotas (
        store_id,
        max_template_count
    )
VALUES (1009, 5),
    (1004, 5);

COMMIT;