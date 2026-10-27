-- 模板引擎擴充：push_mode 欄位
-- visible（預設）= 走 Send（notification + data block，銀幕跳 banner、進 App inbox）
-- silent          = 走 SendDataOnly（只 data，不跳 banner、不進 inbox；App 用於背景刷新）
ALTER TABLE "notification_templates" ADD COLUMN "push_mode" varchar(10) NOT NULL DEFAULT 'visible';
