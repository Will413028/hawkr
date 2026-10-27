-- Issue #843：PayUni MemberStatus=5（會員層待補）在 search_merchant_status 沒有
-- 會員層 reason 欄位，賣家補件通知因此開天窗（只顯示通用「請儘速補件」、不知如何處理）。
--
-- 本檔讓 merchant.verification.pending_info 模板顯示 seller-service 帶入的 #{reason}：
--   - 商店層 (MerStatus=91)：PayUni 回傳的原因文字（例「請補上負責人身分證背面」）。
--   - 會員層 (MemberStatus=5)：seller-service 套用固定引導（查收 PayUni 通知信 + 至
--     PayUni 會員後台填寫會員資料）。
--
-- reason 為空時 PublishVerificationCompleted 因 omitempty 不帶該欄位，RenderTemplate
-- 偵測到缺值 → 套用下方 body_fallback，故不會顯示殘留的 "#{reason}"。

UPDATE "notification_templates"
SET
  "body_template" = '您的實名認證需補件：#{reason}',
  "body_fallback" = '您的實名認證需要補充資料，請查收 PayUni 通知信並完成驗證。'
WHERE "event_type" = 'merchant.verification.pending_info'
  AND "target_user_type" = 'seller';
