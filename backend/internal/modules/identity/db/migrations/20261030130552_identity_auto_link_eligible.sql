-- Modify "account_identities" table
ALTER TABLE "identity"."account_identities" ADD CONSTRAINT "account_identities_auto_link_check" CHECK ((NOT auto_link_eligible) OR ((provider = 'GOOGLE'::text) AND email_verified)), ADD COLUMN "auto_link_eligible" boolean NOT NULL DEFAULT false;
