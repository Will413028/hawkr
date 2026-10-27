-- issue #1467 根治：matchesFilter 新增 $ne 排除語法（見 internal/domain/notification_template.go
-- FilterCondition / internal/worker/template_handler.go matchesFilter），本 migration 是第一個使用者。
--
-- id=7（order.cancelled.buyer，泛用取消通知）原本 payload_filter=NULL 全收，與 id=403
-- （order.cancelled.seller_suspended，payload_filter cancel_reason=9104）同時命中賣家停權取消，
-- 買家因此收到兩則通知（短期方案接受於 20260713000000_seller_suspension_notification_templates.sql
-- 內記錄，本 migration 為該記錄承諾的根治）。
-- 加上 payload_filter {"cancel_reason": {"$ne": "9104"}} 後，id=7 與 id=403 互斥，
-- 賣家停權取消恢復「買家只收一則」。
--
-- body_template 原文「因庫存不足」對 cancel_reason=9104 以外的其餘原因（買家自願取消未付款單
-- ／cancel_reason=NULL、付款逾時 9101、出貨逾時 9102、建單逾時 9103、賣家因缺貨/品質/天災/
-- 建議拆單取消等 92xx）也是錯誤歸因文案。改用不臆測原因的中性文案；退款用「如已付款」hedge
-- （而非斷言一定退款）——因為部分原因（買家自願取消未付款單、付款逾時）發生於訂單從未付款，
-- 沒有退款這回事；其餘原因（出貨逾時／建單逾時／賣家因故取消）發生於已付款訂單，才有退款。
-- title_template 原文「賣家主動取消」在買家自願取消（OrderService.cancelOrder → cancel_reason=NULL）
-- 場景下歸因錯誤（是買家自己取消，不是賣家），一併改為中性標題。
UPDATE "notification_templates"
SET
  "title_template" = '訂單已取消',
  "body_template" = '您的訂單 ##{order_no} 已取消，如已付款將原路退回，如有疑問請洽客服。',
  "payload_filter" = '{"cancel_reason": {"$ne": "9104"}}'::jsonb,
  "updated_at" = NOW()
WHERE "id" = 7;
