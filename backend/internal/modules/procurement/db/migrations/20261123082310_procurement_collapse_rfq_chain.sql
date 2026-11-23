-- Modify "buyer_quote_acceptances" table
ALTER TABLE "procurement"."buyer_quote_acceptances" DROP COLUMN "confirmed_rfq_payload_hash", ADD COLUMN "seller_facing_hash" text NOT NULL;
-- Modify "rfq_publications" table
ALTER TABLE "procurement"."rfq_publications" ADD CONSTRAINT "rfq_publications_approval_version_check" CHECK (approval_version >= 1), ADD COLUMN "approval_version" integer NOT NULL;
-- Modify "rfq_publication_approvals" table
ALTER TABLE "procurement"."rfq_publication_approvals" ADD CONSTRAINT "rfq_publication_approvals_candidate_set_check" CHECK ((audience_mode = 'DIRECT_RECIPIENTS'::text) OR (candidate_set_id IS NULL)), ADD CONSTRAINT "rfq_publication_approvals_note_check" CHECK ((char_length(buyer_note) <= 500) AND (buyer_note !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)), ADD CONSTRAINT "rfq_publication_approvals_range_check" CHECK ((range_lower_amount > 0) AND (range_upper_amount >= range_lower_amount)), DROP COLUMN "confirmed_rfq_snapshot_id", ADD COLUMN "range_lower_amount" bigint NOT NULL, ADD COLUMN "range_upper_amount" bigint NOT NULL, ADD COLUMN "buyer_note" text NOT NULL DEFAULT '', ADD COLUMN "candidate_set_id" uuid NULL, ADD CONSTRAINT "rfq_publication_approvals_candidate_set_id_fkey" FOREIGN KEY ("candidate_set_id") REFERENCES "procurement"."candidate_sets" ("id") ON UPDATE NO ACTION ON DELETE SET NULL;
-- Drop "confirmed_rfq_snapshots" table
DROP TABLE "procurement"."confirmed_rfq_snapshots";
-- Drop "price_previews" table
DROP TABLE "procurement"."price_previews";
