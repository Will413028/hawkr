-- Add seq_id column for per-conversation sequence numbers (gap-fill sync)
ALTER TABLE "messages" ADD COLUMN "seq_id" bigint NOT NULL DEFAULT 0;

-- Backfill seq_id for existing messages: assign sequential numbers per room_id ordered by id
WITH numbered AS (
    SELECT id, ROW_NUMBER() OVER (PARTITION BY room_id ORDER BY id) AS rn
    FROM messages
)
UPDATE messages SET seq_id = numbered.rn
FROM numbered WHERE messages.id = numbered.id;

-- Unique index to prevent duplicate seq_id within same room
CREATE UNIQUE INDEX "idx_messages_room_seq" ON "messages" ("room_id", "seq_id");
