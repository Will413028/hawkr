-- Modify "conversations" table
ALTER TABLE "conversations" ADD COLUMN "last_message_preview" text NULL;
-- Modify "messages" table
ALTER TABLE "messages" ADD COLUMN "preview_text" text NULL;
