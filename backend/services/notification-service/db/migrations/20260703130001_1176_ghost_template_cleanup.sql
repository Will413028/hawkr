-- issue #1176 PR-1: 幽靈 template 處置 + 孤兒 data_fields 清理（36 號設計書 §7.6.4 處置決議表）

-- return.shipped：語意已被 aftersales.case.return_in_transit（#1226）取代 → 刪除
DELETE FROM notification_templates
WHERE event_type = 'return.shipped' AND target_user_type = 'seller';

-- order.shipped.merchant：silent 刷新語意已被 seller.order.updated（#1181）取代 → 刪除
DELETE FROM notification_templates
WHERE event_type = 'order.shipped.merchant' AND target_user_type = 'seller';

-- payout.completed：撥款業務未實作 → disable，待撥款功能落地（獨立 feature issue）再 enable
UPDATE notification_templates
SET enabled = false, updated_at = CURRENT_TIMESTAMP
WHERE event_type = 'payout.completed' AND target_user_type = 'seller';

-- 孤兒 data_fields：publisher 設計上不發 auto_complete_reason(_label)（原因由 event_type 區分，09 號設計書 §238）
UPDATE notification_templates
SET data_fields = (data_fields - 'autoCompleteReason') - 'autoCompleteReasonLabel',
    updated_at = CURRENT_TIMESTAMP
WHERE event_type IN (
  'aftersales.case.auto_completed_store_pickup_timeout.seller',
  'aftersales.case.auto_completed_confirm_receipt_timeout.seller'
) AND data_fields ?| ARRAY['autoCompleteReason', 'autoCompleteReasonLabel'];
