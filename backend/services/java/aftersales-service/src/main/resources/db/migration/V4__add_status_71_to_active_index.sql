-- #1225 確認收貨自動取消售後：新增終態 CANCELLED_BY_CONFIRM_RECEIPT(71)
-- uk_aftersale_cases_active_order 排除集合加入 71（比照 CANCELLED(70)「售後未成立」終態，不佔同訂單活躍名額）。
-- 語意一致性（SSOT 與 09 §2.1 helper 對齊）；排除集合擴大屬約束放寬，不會使既有資料違反。
DROP INDEX IF EXISTS uk_aftersale_cases_active_order;
CREATE UNIQUE INDEX uk_aftersale_cases_active_order ON aftersale_cases (order_no)
    WHERE status NOT IN ('50', '70', '71');
