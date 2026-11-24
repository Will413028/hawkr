-- Modify "accounts" table
ALTER TABLE "identity"."accounts" ADD CONSTRAINT "accounts_preferred_locale_check" CHECK (preferred_locale = ANY (ARRAY['zh-TW'::text, 'en'::text])), ADD COLUMN "preferred_locale" text NOT NULL DEFAULT 'zh-TW';
