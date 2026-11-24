-- Create "announcements" table
CREATE TABLE "notification"."announcements" (
  "id" uuid NOT NULL,
  "audiences" text[] NOT NULL,
  "title" text NOT NULL,
  "body" text NOT NULL,
  "admin_id" bigint NOT NULL,
  "request_id" uuid NULL,
  "inbox_count" integer NOT NULL DEFAULT 0,
  "inbox_done_at" timestamptz NULL,
  "push_done_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "announcements_request_key" UNIQUE ("admin_id", "request_id"),
  CONSTRAINT "announcements_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "announcements_audiences_check" CHECK (((audiences = ARRAY['BUYER'::text]) OR (audiences = ARRAY['STORE'::text])) OR (audiences = ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "announcements_body_check" CHECK (((char_length(body) >= 1) AND (char_length(body) <= 2000)) AND (body !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "announcements_inbox_count_check" CHECK (inbox_count >= 0),
  CONSTRAINT "announcements_title_check" CHECK (((char_length(title) >= 1) AND (char_length(title) <= 100)) AND (title !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))
);
-- Create index "announcements_listing" to table: "announcements"
CREATE INDEX "announcements_listing" ON "notification"."announcements" ("created_at" DESC, "id" DESC);
-- Modify "inbox_items" table
ALTER TABLE "notification"."inbox_items" ADD CONSTRAINT "inbox_items_announcement_check" CHECK ((type = 'announcement'::text) = (announcement_id IS NOT NULL)), ADD COLUMN "announcement_id" uuid NULL, ADD CONSTRAINT "inbox_items_announcement_id_fkey" FOREIGN KEY ("announcement_id") REFERENCES "notification"."announcements" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "inbox_items_announcement_recipient" to table: "inbox_items"
CREATE UNIQUE INDEX "inbox_items_announcement_recipient" ON "notification"."inbox_items" ("announcement_id", "account_id", "audience", "store_id") NULLS NOT DISTINCT WHERE (announcement_id IS NOT NULL);
