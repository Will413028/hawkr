-- Modify "seller_deleted_outbox" table
ALTER TABLE "seller_deleted_outbox" ADD COLUMN "event_published_at" timestamptz NULL, ADD COLUMN "role_removed_at" timestamptz NULL;
