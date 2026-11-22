-- Modify "messages" table
ALTER TABLE "im"."messages" ADD COLUMN "request_id" uuid NOT NULL, ADD CONSTRAINT "messages_request_key" UNIQUE ("conversation_id", "sender_kind", "request_id");
