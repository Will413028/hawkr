-- 會員方案初始資料；由獨立資料 migration 套用，測試共用此來源。
INSERT INTO buyer.membership_plans (
    plan_code, display_name, monthly_answer_limit, monthly_price_twd,
    is_purchasable, created_at, updated_at
) VALUES
    ('free', '免費方案', 100, 0, true, now(), now()),
    ('basic', '基本方案', 300, 199, true, now(), now()),
    ('premium', '進階方案', 1000, 499, true, now(), now());
