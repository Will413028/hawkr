UPDATE "notification_templates"
SET target_user_field = 'notify_buyer_account_id'
WHERE event_type IN (
    'aftersales.refund.completed.buyer',
    'aftersales.refund.awaiting_buyer_page.buyer'
);
UPDATE "notification_templates"
SET body_template = '賣家已同意退款，請至訂單管理填寫收款銀行帳號以收取退款（訂單 ##{order_no}）',
    reference_id_template = 'awaiting_buyer_page_#{refund_task_id}'
WHERE event_type = 'aftersales.refund.awaiting_buyer_page.buyer';
