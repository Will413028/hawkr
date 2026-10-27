-- Modify "procurement_conversations" table
ALTER TABLE "procurement_conversations" ADD COLUMN "title" text NOT NULL DEFAULT '', ADD COLUMN "title_source" text NOT NULL DEFAULT 'AUTO', ADD COLUMN "last_message_preview" text NOT NULL DEFAULT '', ADD COLUMN "deleted_at" timestamptz NULL;
-- Create index "idx_procurement_conversations_buyer_active" to table: "procurement_conversations"
CREATE INDEX "idx_procurement_conversations_buyer_active" ON "procurement_conversations" ("buyer_id", "id" DESC) WHERE (deleted_at IS NULL);
-- Create "procurement_conversation_commands" table
CREATE TABLE "procurement_conversation_commands" (
  "id" bigint NOT NULL,
  "buyer_id" bigint NOT NULL,
  "client_request_id" text NOT NULL,
  "conversation_id" bigint NOT NULL,
  "result_json" jsonb NOT NULL DEFAULT '{}',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "uq_procurement_conversation_commands" to table: "procurement_conversation_commands"
CREATE UNIQUE INDEX "uq_procurement_conversation_commands" ON "procurement_conversation_commands" ("buyer_id", "client_request_id");
