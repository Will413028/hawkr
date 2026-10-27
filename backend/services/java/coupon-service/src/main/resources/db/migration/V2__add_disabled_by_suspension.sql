-- 44 號 §5.20 / 32 號 §13.5：賣家停權連動模板停用／還原。
-- SellerSuspendedConsumer 停用賣家 ACTIVE 模板為 INACTIVE 時標記 TRUE；
-- SellerUnsuspendedConsumer 只還原此旗標為 TRUE 的模板，不誤還原賣家本來就自行停用/被 admin
-- 停用的模板（賣家流程本無自助停用，但 admin 可停用）。
ALTER TABLE coupon_templates
  ADD COLUMN IF NOT EXISTS disabled_by_suspension BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN coupon_templates.disabled_by_suspension IS
  '因賣家停權連動停用時標記 TRUE；解除停權時精準還原依據 - 設計書 07 號 §5.20';
