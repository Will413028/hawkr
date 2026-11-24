-- =============================================
-- V10: coupon_consumption_tasks 加任務型別 RELEASE（B7-4a，D3）
--
-- 取消路徑（買家取消、售後取消、付款逾時）與結帳鎖券失敗補償的釋券，改寫成這張表的任務，由
-- CouponConsumptionWorker 以指數退避重試；釋券以 order_no 為鍵，不需要 coupon_instance_id，
-- 也不需要 buyer_id，所以兩欄改為可空。唯一鍵從 (order_no, coupon_instance_id) 改為
-- (order_no, task_type)：同一筆訂單每種任務只有一筆，重複寫入由 ON CONFLICT DO NOTHING 吸收。
-- 既有的列全是核銷任務，DEFAULT 'CONSUME' 讓它們原樣成立。
-- =============================================

ALTER TABLE coupon_consumption_tasks
    ADD COLUMN task_type VARCHAR(16) NOT NULL DEFAULT 'CONSUME';

ALTER TABLE coupon_consumption_tasks
    ADD CONSTRAINT chk_coupon_consumption_task_type CHECK (task_type IN ('CONSUME', 'RELEASE'));

ALTER TABLE coupon_consumption_tasks ALTER COLUMN coupon_instance_id DROP NOT NULL;
ALTER TABLE coupon_consumption_tasks ALTER COLUMN buyer_id DROP NOT NULL;

ALTER TABLE coupon_consumption_tasks DROP CONSTRAINT uk_coupon_consumption_order_coupon;
ALTER TABLE coupon_consumption_tasks
    ADD CONSTRAINT uk_coupon_consumption_order_type UNIQUE (order_no, task_type);
