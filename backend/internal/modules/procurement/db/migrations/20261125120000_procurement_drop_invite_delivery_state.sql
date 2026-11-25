-- Drop index "rfq_publication_invites_queued" from table: "rfq_publication_invites"
DROP INDEX "procurement"."rfq_publication_invites_queued";
-- Modify "rfq_publication_invites" table
ALTER TABLE "procurement"."rfq_publication_invites" DROP CONSTRAINT "rfq_publication_invites_state_check", DROP COLUMN "delivery_state", DROP COLUMN "delivery_state_at";
