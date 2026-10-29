-- Create "account_suspensions" table
CREATE TABLE "identity"."account_suspensions" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "account_id" bigint NOT NULL,
  "kind" text NOT NULL DEFAULT 'PERMANENT',
  "reason_category" text NOT NULL,
  "reason_text" text NOT NULL DEFAULT '',
  "source_ref" text NULL,
  "suspended_at" timestamptz NOT NULL,
  "released_at" timestamptz NULL,
  "release_reason" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "account_suspensions_account_id_fkey" FOREIGN KEY ("account_id") REFERENCES "identity"."accounts" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "account_suspensions_kind_check" CHECK (kind = 'PERMANENT'::text),
  CONSTRAINT "account_suspensions_reason_category_check" CHECK (reason_category = ANY (ARRAY['FOOD_SAFETY'::text, 'PROHIBITED_GOODS'::text, 'UNLICENSED_SALE'::text, 'MISLEADING_INFO'::text, 'FAKE_CERTIFICATION'::text, 'COUNTERFEIT'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text, 'OTHER'::text, 'SPAM'::text, 'HARASSMENT'::text, 'HATE'::text])),
  CONSTRAINT "account_suspensions_reason_text_check" CHECK ((char_length(reason_text) <= 1000) AND (reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "account_suspensions_release_check" CHECK (((released_at IS NULL) AND (release_reason IS NULL)) OR ((released_at IS NOT NULL) AND (release_reason IS NOT NULL) AND (released_at >= suspended_at))),
  CONSTRAINT "account_suspensions_release_reason_check" CHECK ((release_reason IS NULL) OR ((char_length(release_reason) >= 1) AND (char_length(release_reason) <= 1000) AND (release_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "account_suspensions_source_ref_check" CHECK ((source_ref IS NULL) OR ((char_length(source_ref) >= 1) AND (char_length(source_ref) <= 200) AND (source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)))
);
-- Create index "idx_account_suspensions_account_id_suspended_at" to table: "account_suspensions"
CREATE INDEX "idx_account_suspensions_account_id_suspended_at" ON "identity"."account_suspensions" ("account_id", "suspended_at" DESC);
-- Create index "uidx_account_suspensions_active" to table: "account_suspensions"
CREATE UNIQUE INDEX "uidx_account_suspensions_active" ON "identity"."account_suspensions" ("account_id") WHERE (released_at IS NULL);
-- Backfill suspension records for accounts suspended before this table existed
INSERT INTO "identity"."account_suspensions" ("account_id", "reason_category", "suspended_at")
SELECT "id", 'OTHER', "suspended_at" FROM "identity"."accounts" WHERE "suspended_at" IS NOT NULL;
