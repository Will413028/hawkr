-- 移除 order.refund.requested → buyer / seller 兩個 template
--
-- 理由：該 routing key 從未被任何 service publish（OrderEventType.ORDER_REFUND_REQUESTED
-- 註解明寫「保留供未來使用」），templates 一直是死碼。買家申請售後現由
-- aftersales.case.applied 系列 routing key 處理（migration 20260426180002）。
DELETE FROM "notification_templates" WHERE "routing_key" = 'order.refund.requested';
