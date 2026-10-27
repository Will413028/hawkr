-- 49 號 §5.3 #10：notification 圖片欄位誠實命名（名帶 url 實存 objectKey → 改為 *_key）。
-- RENAME COLUMN 保值（0 存量）；渲染端於 push 送出前以 mediaurl.BuildURL 現組顯示 URL，
-- BuildURL 對存量完整 URL 走 passthrough，producer 遷移期不破圖。
ALTER TABLE "notification_templates" RENAME COLUMN "image_url_template" TO "image_key_template";
ALTER TABLE "system_notification_logs" RENAME COLUMN "image_url" TO "image_key";
