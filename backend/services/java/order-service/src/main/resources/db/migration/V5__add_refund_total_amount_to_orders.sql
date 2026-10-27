-- 03 §3.1：累計已完成退款金額（元）。
-- 由 aftersales.refund.completed（PARTIAL outcome）消費時同事務累加（03 §5.7.1）；
-- =0 表示無已完成退款。完成訂單時撥款任務分流依據（03 §3.11.1 / §3.11.2）。
ALTER TABLE orders ADD COLUMN refund_total_amount BIGINT NOT NULL DEFAULT 0;
