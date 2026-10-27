-- Create "conversations" table
CREATE TABLE "conversations" (
  "id" bigserial NOT NULL,
  "room_id" character varying(64) NOT NULL,
  "participant_ids" jsonb NOT NULL,
  "last_message_id" bigint NULL DEFAULT 0,
  "last_message_content" text NULL DEFAULT '',
  "updated_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id")
);
-- Create index "idx_conversations_room_id" to table: "conversations"
CREATE UNIQUE INDEX "idx_conversations_room_id" ON "conversations" ("room_id");
-- Create index "idx_conversations_updated_at" to table: "conversations"
CREATE INDEX "idx_conversations_updated_at" ON "conversations" ("updated_at" DESC);
-- Create "messages" table
CREATE TABLE "messages" (
  "id" bigserial NOT NULL,
  "room_id" character varying(64) NOT NULL,
  "sender_id" bigint NOT NULL,
  "receiver_id" bigint NULL DEFAULT 0,
  "type" integer NOT NULL DEFAULT 1,
  "content" text NOT NULL,
  "created_at" timestamptz NULL DEFAULT CURRENT_TIMESTAMP,
  "is_read" boolean NULL DEFAULT false,
  PRIMARY KEY ("id")
);
-- Create index "idx_messages_receiver_id" to table: "messages"
CREATE INDEX "idx_messages_receiver_id" ON "messages" ("receiver_id");
-- Create index "idx_messages_room_id" to table: "messages"
CREATE INDEX "idx_messages_room_id" ON "messages" ("room_id");
-- Create index "idx_messages_sender_id" to table: "messages"
CREATE INDEX "idx_messages_sender_id" ON "messages" ("sender_id");
-- Create index "idx_messages_site_room_created" to table: "messages"
CREATE INDEX "idx_messages_site_room_created" ON "messages" ("room_id", "created_at" DESC);
