-- Modify "payuni_registrations" table
ALTER TABLE "payuni_registrations" ADD COLUMN "last_claimed_at" timestamptz NULL;

UPDATE "payuni_registrations"
SET "last_claimed_at" = "last_synced_at"
WHERE "last_synced_at" IS NOT NULL;
