-- Modify "conversations" table
ALTER TABLE "conversations" ADD COLUMN "last_message_type" integer NOT NULL DEFAULT 1, ADD COLUMN "buyer_unread_count" integer NOT NULL DEFAULT 0, ADD COLUMN "seller_unread_count" integer NOT NULL DEFAULT 0;
-- Backfill: receiver_id = 0 的舊資料，根據 room_id 和 sender_id 推算正確的 receiver
UPDATE messages SET receiver_id = (
  CASE
    WHEN sender_id = CAST(SUBSTRING(room_id FROM 'b(\d+)_s') AS BIGINT)
    THEN CAST(SUBSTRING(room_id FROM '_s(\d+)$') AS BIGINT)
    ELSE CAST(SUBSTRING(room_id FROM 'b(\d+)_s') AS BIGINT)
  END
) WHERE receiver_id = 0;
-- Modify "messages" table
ALTER TABLE "messages" ALTER COLUMN "receiver_id" SET NOT NULL, ALTER COLUMN "receiver_id" DROP DEFAULT;
-- Backfill: unread counts
UPDATE conversations SET buyer_unread_count = (
  SELECT COUNT(*) FROM messages
  WHERE messages.room_id = conversations.room_id
    AND messages.receiver_id = conversations.buyer_account_id
    AND messages.is_read = FALSE
);
UPDATE conversations SET seller_unread_count = (
  SELECT COUNT(*) FROM messages
  WHERE messages.room_id = conversations.room_id
    AND messages.receiver_id = conversations.seller_account_id
    AND messages.is_read = FALSE
);
