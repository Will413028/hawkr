-- 1. 建立 push_notifications 表（推播追蹤），先不建 unique index
CREATE TABLE "push_notifications" (
  "id" bigint NOT NULL,
  "user_id" bigint NOT NULL,
  "user_type" varchar(10) NOT NULL,
  "event_type" varchar(50) NOT NULL,
  "reference_id" varchar(100),
  "status" varchar(20) NOT NULL,
  "error_log" text,
  "sent_at" timestamptz,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);

CREATE INDEX "push_notifications_user_idx" ON "push_notifications" ("user_id", "user_type");

-- 2. 將現有 notifications 資料複製到 push_notifications 作為歷史推播紀錄
INSERT INTO "push_notifications" (id, user_id, user_type, event_type, reference_id, status, error_log, sent_at, created_at)
SELECT id, user_id, user_type, event_type, reference_id, status, error_log, sent_at, created_at
FROM "notifications";

-- 2a. 去重 push_notifications：同 (event_type, reference_id, status='sent') 保留最舊一筆
--     舊資料可能有多用戶各自成功送出同一 event+reference，unique index 建立前必須清理
DELETE FROM "push_notifications" pn
USING (
  SELECT MIN(id) AS min_id, event_type, reference_id
  FROM "push_notifications"
  WHERE status = 'sent' AND reference_id IS NOT NULL
  GROUP BY event_type, reference_id
  HAVING COUNT(*) > 1
) dup
WHERE pn.event_type = dup.event_type
  AND pn.reference_id = dup.reference_id
  AND pn.status = 'sent'
  AND pn.id != dup.min_id;

-- 2b. 建立 push_notifications dedup unique index（資料已去重，建立安全）
CREATE UNIQUE INDEX "push_notifications_dedup_idx" ON "push_notifications" ("event_type", "reference_id")
  WHERE "status" = 'sent' AND "reference_id" IS NOT NULL;

-- 3. 從 notifications 移除非 inbox 紀錄（status != 'sent' 從未顯示於 inbox）
DELETE FROM "notifications" WHERE status != 'sent';

-- 4. 去重 notifications（同 user+event+reference 保留最舊一筆，防止 unique index 建立失敗）
DELETE FROM "notifications" n
USING (
  SELECT MIN(id) AS min_id, user_id, user_type, event_type, reference_id
  FROM "notifications"
  WHERE reference_id IS NOT NULL
  GROUP BY user_id, user_type, event_type, reference_id
  HAVING COUNT(*) > 1
) dup
WHERE n.user_id = dup.user_id
  AND n.user_type = dup.user_type
  AND n.event_type = dup.event_type
  AND n.reference_id = dup.reference_id
  AND n.id != dup.min_id;

-- 5. 移除 notifications 舊 index（部分含 status 過濾）
DROP INDEX IF EXISTS "idx_notifications_dedup";
DROP INDEX IF EXISTS "idx_notifications_inbox";
DROP INDEX IF EXISTS "idx_notifications_unread";
DROP INDEX IF EXISTS "idx_notifications_cleanup_failed";
DROP INDEX IF EXISTS "idx_notifications_cleanup_deleted";

-- 6. 建立新 inbox index（不含 status 條件）
CREATE INDEX "idx_notifications_inbox" ON "notifications" ("user_id", "user_type", "id" DESC) WHERE "deleted_at" IS NULL;
CREATE INDEX "idx_notifications_unread" ON "notifications" ("user_id", "user_type") WHERE "deleted_at" IS NULL AND "is_read" = false;
CREATE INDEX "idx_notifications_cleanup_deleted" ON "notifications" ("deleted_at") WHERE "deleted_at" IS NOT NULL;

-- 7. 建立 inbox 去重 unique index
CREATE UNIQUE INDEX "notifications_inbox_dedup_idx" ON "notifications" ("user_id", "user_type", "event_type", "reference_id")
  WHERE "reference_id" IS NOT NULL;

-- 8. 移除 notifications 推播相關欄位（最後執行，確保資料已複製）
ALTER TABLE "notifications" DROP COLUMN IF EXISTS "status";
ALTER TABLE "notifications" DROP COLUMN IF EXISTS "sent_at";
ALTER TABLE "notifications" DROP COLUMN IF EXISTS "error_log";
