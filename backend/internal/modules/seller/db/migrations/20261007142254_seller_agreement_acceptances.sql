-- Create "agreement_acceptances" table
CREATE TABLE "seller"."agreement_acceptances" (
  "acceptance_id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "store_id" bigint NOT NULL,
  "kind" text NOT NULL,
  "version" text NOT NULL,
  "account_id" bigint NOT NULL,
  "accepted_at" timestamptz NOT NULL,
  "submission_review_id" uuid NULL,
  "client_ip" inet NULL,
  "user_agent" text NULL,
  "pii_purged_at" timestamptz NULL,
  PRIMARY KEY ("acceptance_id"),
  CONSTRAINT "agreement_acceptances_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "agreement_acceptances_submission_review_id_fkey" FOREIGN KEY ("submission_review_id") REFERENCES "seller"."kyc_reviews" ("review_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "agreement_acceptances_account_check" CHECK (account_id > 0),
  CONSTRAINT "agreement_acceptances_kind_check" CHECK (kind = ANY (ARRAY['SELLER_TERMS'::text, 'KYC_PRIVACY_NOTICE'::text, 'PAYUNI_TERMS'::text])),
  CONSTRAINT "agreement_acceptances_purged_check" CHECK ((pii_purged_at IS NULL) OR ((client_ip IS NULL) AND (user_agent IS NULL))),
  CONSTRAINT "agreement_acceptances_submission_check" CHECK ((kind = 'SELLER_TERMS'::text) = (submission_review_id IS NULL)),
  CONSTRAINT "agreement_acceptances_user_agent_check" CHECK ((user_agent IS NULL) OR ((char_length(user_agent) >= 1) AND (char_length(user_agent) <= 512))),
  CONSTRAINT "agreement_acceptances_version_check" CHECK (char_length(version) >= 1)
);
-- Create index "agreement_acceptances_seller_terms" to table: "agreement_acceptances"
CREATE UNIQUE INDEX "agreement_acceptances_seller_terms" ON "seller"."agreement_acceptances" ("store_id") WHERE (kind = 'SELLER_TERMS'::text);
-- Create index "agreement_acceptances_store" to table: "agreement_acceptances"
CREATE INDEX "agreement_acceptances_store" ON "seller"."agreement_acceptances" ("store_id", "accepted_at", "acceptance_id");
-- Create index "agreement_acceptances_submission" to table: "agreement_acceptances"
CREATE INDEX "agreement_acceptances_submission" ON "seller"."agreement_acceptances" ("submission_review_id") WHERE (submission_review_id IS NOT NULL);
