-- Add new schema named "im"
CREATE SCHEMA "im";
-- Create "conversations" table
CREATE TABLE "im"."conversations" (
  "id" uuid NOT NULL,
  "buyer_id" bigint NOT NULL,
  "store_id" bigint NOT NULL,
  "last_seq" bigint NOT NULL DEFAULT 0,
  "buyer_last_read_seq" bigint NOT NULL DEFAULT 0,
  "store_last_read_seq" bigint NOT NULL DEFAULT 0,
  "last_message_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "conversations_pair_key" UNIQUE ("buyer_id", "store_id"),
  CONSTRAINT "conversations_buyer_check" CHECK (buyer_id > 0),
  CONSTRAINT "conversations_buyer_read_check" CHECK ((buyer_last_read_seq >= 0) AND (buyer_last_read_seq <= last_seq)),
  CONSTRAINT "conversations_last_seq_check" CHECK (last_seq >= 0),
  CONSTRAINT "conversations_store_check" CHECK (store_id > 0),
  CONSTRAINT "conversations_store_read_check" CHECK ((store_last_read_seq >= 0) AND (store_last_read_seq <= last_seq))
);
-- Create index "conversations_buyer" to table: "conversations"
CREATE INDEX "conversations_buyer" ON "im"."conversations" ("buyer_id", "last_message_at" DESC, "id" DESC);
-- Create index "conversations_store" to table: "conversations"
CREATE INDEX "conversations_store" ON "im"."conversations" ("store_id", "last_message_at" DESC, "id" DESC);
-- Create "inactive_parties" table
CREATE TABLE "im"."inactive_parties" (
  "kind" text NOT NULL,
  "party_id" bigint NOT NULL,
  "since" timestamptz NOT NULL,
  PRIMARY KEY ("kind", "party_id"),
  CONSTRAINT "inactive_parties_kind_check" CHECK (kind = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "inactive_parties_party_check" CHECK (party_id > 0)
);
-- Create "media_ref_sync" table
CREATE TABLE "im"."media_ref_sync" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "seq" bigint NOT NULL DEFAULT 1,
  "synced_seq" bigint NOT NULL DEFAULT 0,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("ref_type", "ref_id")
);
-- Create index "media_ref_sync_unsynced" to table: "media_ref_sync"
CREATE INDEX "media_ref_sync_unsynced" ON "im"."media_ref_sync" ("updated_at") WHERE (synced_seq < seq);
-- Create "retired_assets" table
CREATE TABLE "im"."retired_assets" (
  "ref_type" text NOT NULL,
  "ref_id" text NOT NULL,
  "asset_id" text NOT NULL,
  "retired_at" timestamptz NOT NULL,
  "unlinked_at" timestamptz NULL,
  PRIMARY KEY ("ref_type", "ref_id", "asset_id")
);
-- Create index "retired_assets_retired_at" to table: "retired_assets"
CREATE INDEX "retired_assets_retired_at" ON "im"."retired_assets" ("retired_at");
-- Create "messages" table
CREATE TABLE "im"."messages" (
  "id" uuid NOT NULL,
  "conversation_id" uuid NOT NULL,
  "seq" bigint NOT NULL,
  "sender_kind" text NOT NULL,
  "message_type" text NOT NULL,
  "content" text NULL,
  "image_asset_id" text NULL,
  "image_object_key" text NULL,
  "created_at" timestamptz NOT NULL,
  "erased_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "messages_seq_key" UNIQUE ("conversation_id", "seq"),
  CONSTRAINT "messages_conversation_id_fkey" FOREIGN KEY ("conversation_id") REFERENCES "im"."conversations" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "messages_content_check" CHECK ((content IS NULL) OR ((char_length(content) >= 1) AND (char_length(content) <= 4096) AND (content !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "messages_image_check" CHECK (((image_asset_id IS NULL) OR (image_asset_id <> ''::text)) AND ((image_object_key IS NULL) OR (image_object_key <> ''::text))),
  CONSTRAINT "messages_sender_kind_check" CHECK (sender_kind = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "messages_seq_check" CHECK (seq > 0),
  CONSTRAINT "messages_shape_check" CHECK (((erased_at IS NULL) AND (((message_type = 'TEXT'::text) AND (content IS NOT NULL) AND (image_asset_id IS NULL) AND (image_object_key IS NULL)) OR ((message_type = 'IMAGE'::text) AND (content IS NULL) AND (image_asset_id IS NOT NULL) AND (image_object_key IS NOT NULL)))) OR ((erased_at IS NOT NULL) AND (content IS NULL) AND (image_asset_id IS NULL) AND (image_object_key IS NULL))),
  CONSTRAINT "messages_type_check" CHECK (message_type = ANY (ARRAY['TEXT'::text, 'IMAGE'::text]))
);
-- Create index "messages_conversation_seq" to table: "messages"
CREATE INDEX "messages_conversation_seq" ON "im"."messages" ("conversation_id", "seq" DESC);
-- Create "hides" table
CREATE TABLE "im"."hides" (
  "id" uuid NOT NULL,
  "message_id" uuid NOT NULL,
  "reason_category" text NOT NULL,
  "reason_text" text NOT NULL DEFAULT '',
  "source_ref" text NULL,
  "admin_id" bigint NOT NULL,
  "hidden_at" timestamptz NOT NULL,
  "restored_at" timestamptz NULL,
  "restored_by" bigint NULL,
  "restore_reason" text NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "hides_message_id_fkey" FOREIGN KEY ("message_id") REFERENCES "im"."messages" ("id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "hides_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "hides_reason_category_check" CHECK (reason_category = ANY (ARRAY['MISLEADING_INFO'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text, 'SPAM'::text, 'HARASSMENT'::text, 'HATE'::text, 'SEXUAL_CONTENT'::text])),
  CONSTRAINT "hides_reason_text_check" CHECK ((char_length(reason_text) <= 500) AND (reason_text !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)),
  CONSTRAINT "hides_restore_check" CHECK (((restored_at IS NULL) AND (restored_by IS NULL) AND (restore_reason IS NULL)) OR ((restored_at IS NOT NULL) AND (restored_by > 0) AND (restore_reason IS NOT NULL) AND (restored_at >= hidden_at))),
  CONSTRAINT "hides_restore_reason_check" CHECK ((restore_reason IS NULL) OR ((char_length(restore_reason) >= 1) AND (char_length(restore_reason) <= 500) AND (restore_reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text))),
  CONSTRAINT "hides_source_ref_check" CHECK ((source_ref IS NULL) OR ((char_length(source_ref) >= 1) AND (char_length(source_ref) <= 200) AND (source_ref !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$'::text)))
);
-- Create index "hides_active_message" to table: "hides"
CREATE UNIQUE INDEX "hides_active_message" ON "im"."hides" ("message_id") WHERE (restored_at IS NULL);
-- Create index "hides_message" to table: "hides"
CREATE INDEX "hides_message" ON "im"."hides" ("message_id", "hidden_at");
