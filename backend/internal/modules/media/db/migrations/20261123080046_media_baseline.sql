-- Add new schema named "media"
CREATE SCHEMA "media";
-- Create "assets" table
CREATE TABLE "media"."assets" (
  "id" uuid NOT NULL,
  "visibility" text NOT NULL,
  "object_key" text NOT NULL,
  "content_type" text NOT NULL,
  "size_bytes" bigint NOT NULL,
  "state" text NOT NULL,
  "kek_id" text NULL,
  "wrapped_dek" bytea NULL,
  "chunk_count" integer NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "assets_object_key_key" UNIQUE ("object_key"),
  CONSTRAINT "assets_chunk_count_check" CHECK (((visibility = 'SECURE'::text) = (chunk_count IS NOT NULL)) AND ((chunk_count IS NULL) OR (chunk_count > 0))),
  CONSTRAINT "assets_content_type_check" CHECK (content_type <> ''::text),
  CONSTRAINT "assets_kek_id_check" CHECK ((visibility = 'SECURE'::text) = (kek_id IS NOT NULL)),
  CONSTRAINT "assets_object_key_prefix_check" CHECK (((visibility = 'PUBLIC'::text) AND (object_key ~~ 'public/%'::text)) OR ((visibility = 'SECURE'::text) AND (object_key ~~ 'secure/%'::text))),
  CONSTRAINT "assets_size_check" CHECK (size_bytes >= 0),
  CONSTRAINT "assets_state_check" CHECK (state = ANY (ARRAY['STAGED'::text, 'DELETING'::text, 'DELETED'::text])),
  CONSTRAINT "assets_visibility_check" CHECK (visibility = ANY (ARRAY['PUBLIC'::text, 'SECURE'::text])),
  CONSTRAINT "assets_wrapped_dek_check" CHECK ((visibility = 'SECURE'::text) = (wrapped_dek IS NOT NULL))
);
-- Create index "assets_deleting" to table: "assets"
CREATE INDEX "assets_deleting" ON "media"."assets" ("id") WHERE (state = 'DELETING'::text);
-- Create "secure_chunks" table
CREATE TABLE "media"."secure_chunks" (
  "asset_id" uuid NOT NULL,
  "idx" integer NOT NULL,
  "object_key" text NOT NULL,
  PRIMARY KEY ("asset_id", "idx"),
  CONSTRAINT "secure_chunks_object_key_key" UNIQUE ("object_key"),
  CONSTRAINT "secure_chunks_idx_check" CHECK (idx >= 0),
  CONSTRAINT "secure_chunks_object_key_check" CHECK (object_key ~~ 'secure/%'::text)
);
-- Create "asset_deletion_requests" table
CREATE TABLE "media"."asset_deletion_requests" (
  "asset_id" uuid NOT NULL,
  "requested_at" timestamptz NOT NULL,
  "applied_at" timestamptz NULL,
  PRIMARY KEY ("asset_id"),
  CONSTRAINT "asset_deletion_requests_asset_fkey" FOREIGN KEY ("asset_id") REFERENCES "media"."assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
-- Create index "asset_deletion_requests_pending" to table: "asset_deletion_requests"
CREATE INDEX "asset_deletion_requests_pending" ON "media"."asset_deletion_requests" ("requested_at") WHERE (applied_at IS NULL);
-- Create "staged_uploads" table
CREATE TABLE "media"."staged_uploads" (
  "id" uuid NOT NULL,
  "token_hash" bytea NOT NULL,
  "uploader_id" bigint NOT NULL,
  "uploader_admin" boolean NOT NULL,
  "delegated_store_id" bigint NULL,
  "visibility" text NOT NULL,
  "declared_size" bigint NOT NULL,
  "declared_content_type" text NULL,
  "declared_md5" text NULL,
  "state" text NOT NULL,
  "reject_reason" text NULL,
  "tmp_key" text NULL,
  "final_key" text NULL,
  "asset_id" uuid NULL,
  "planned_asset_id" uuid NULL,
  "chunk_count" integer NULL,
  "next_index" integer NULL,
  "kek_id" text NULL,
  "wrapped_dek" bytea NULL,
  "expires_at" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "staged_uploads_final_key_key" UNIQUE ("final_key"),
  CONSTRAINT "staged_uploads_planned_asset_id_key" UNIQUE ("planned_asset_id"),
  CONSTRAINT "staged_uploads_tmp_key_key" UNIQUE ("tmp_key"),
  CONSTRAINT "staged_uploads_token_hash_key" UNIQUE ("token_hash"),
  CONSTRAINT "staged_uploads_asset_fkey" FOREIGN KEY ("asset_id") REFERENCES "media"."assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "staged_uploads_asset_state_check" CHECK (((state <> ALL (ARRAY['READY'::text, 'CONFIRMED'::text])) OR (asset_id IS NOT NULL)) AND ((asset_id IS NULL) OR (state = ANY (ARRAY['READY'::text, 'CONFIRMED'::text, 'DELETING'::text, 'DELETED'::text])))),
  CONSTRAINT "staged_uploads_chunks_check" CHECK (((visibility = 'SECURE'::text) = (planned_asset_id IS NOT NULL)) AND ((visibility = 'SECURE'::text) = (chunk_count IS NOT NULL)) AND ((visibility = 'SECURE'::text) = (next_index IS NOT NULL)) AND ((visibility = 'SECURE'::text) = (kek_id IS NOT NULL)) AND ((visibility = 'SECURE'::text) = (wrapped_dek IS NOT NULL)) AND ((chunk_count IS NULL) OR (chunk_count > 0)) AND ((next_index IS NULL) OR ((next_index >= 0) AND (next_index <= chunk_count)))),
  CONSTRAINT "staged_uploads_declared_content_type_check" CHECK (((declared_content_type IS NULL) OR (declared_content_type <> ''::text)) AND ((visibility = 'SECURE'::text) OR (declared_content_type IS NOT NULL)) AND ((state <> ALL (ARRAY['READY'::text, 'CONFIRMED'::text])) OR (declared_content_type IS NOT NULL))),
  CONSTRAINT "staged_uploads_declared_md5_check" CHECK ((declared_md5 IS NULL) OR (declared_md5 <> ''::text)),
  CONSTRAINT "staged_uploads_declared_size_check" CHECK (declared_size > 0),
  CONSTRAINT "staged_uploads_delegated_store_check" CHECK ((delegated_store_id IS NULL) OR ((delegated_store_id > 0) AND uploader_admin)),
  CONSTRAINT "staged_uploads_final_key_state_check" CHECK ((state <> ALL (ARRAY['UPLOADED'::text, 'READY'::text, 'CONFIRMED'::text])) OR (final_key IS NOT NULL)),
  CONSTRAINT "staged_uploads_md5_visibility_check" CHECK ((visibility = 'PUBLIC'::text) = (declared_md5 IS NOT NULL)),
  CONSTRAINT "staged_uploads_ready_chunks_check" CHECK ((visibility <> 'SECURE'::text) OR (state <> ALL (ARRAY['READY'::text, 'CONFIRMED'::text])) OR (next_index = chunk_count)),
  CONSTRAINT "staged_uploads_reject_reason_check" CHECK ((reject_reason IS NULL) OR (reject_reason = ANY (ARRAY['UNRECOGNIZED_FORMAT'::text, 'SIZE_MISMATCH'::text, 'CHECKSUM_MISMATCH'::text, 'PROCESSING_FAILED'::text, 'EXPIRED'::text]))),
  CONSTRAINT "staged_uploads_reject_state_check" CHECK (((state <> 'REJECTED'::text) OR (reject_reason IS NOT NULL)) AND ((reject_reason IS NULL) OR (state = ANY (ARRAY['REJECTED'::text, 'DELETING'::text, 'DELETED'::text])))),
  CONSTRAINT "staged_uploads_state_check" CHECK (state = ANY (ARRAY['PENDING'::text, 'UPLOADING'::text, 'UPLOADED'::text, 'READY'::text, 'REJECTED'::text, 'CONFIRMED'::text, 'DELETING'::text, 'DELETED'::text])),
  CONSTRAINT "staged_uploads_tmp_key_check" CHECK ((visibility = 'PUBLIC'::text) = (tmp_key IS NOT NULL)),
  CONSTRAINT "staged_uploads_token_hash_check" CHECK (octet_length(token_hash) = 32),
  CONSTRAINT "staged_uploads_uploader_check" CHECK (uploader_id > 0),
  CONSTRAINT "staged_uploads_visibility_check" CHECK (visibility = ANY (ARRAY['PUBLIC'::text, 'SECURE'::text]))
);
-- Create index "staged_uploads_deleting" to table: "staged_uploads"
CREATE INDEX "staged_uploads_deleting" ON "media"."staged_uploads" ("id") WHERE (state = 'DELETING'::text);
-- Create index "staged_uploads_gc" to table: "staged_uploads"
CREATE INDEX "staged_uploads_gc" ON "media"."staged_uploads" ("expires_at") WHERE (state = ANY (ARRAY['PENDING'::text, 'UPLOADING'::text, 'READY'::text, 'REJECTED'::text]));
-- Create index "staged_uploads_uploading_rewrap" to table: "staged_uploads"
CREATE INDEX "staged_uploads_uploading_rewrap" ON "media"."staged_uploads" ("id") WHERE (state = 'UPLOADING'::text);
-- Create "upload_claims" table
CREATE TABLE "media"."upload_claims" (
  "upload_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "asset_id" uuid NULL,
  "claimed_xid" xid8 NOT NULL DEFAULT pg_current_xact_id(),
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("upload_id"),
  CONSTRAINT "upload_claims_asset_key" UNIQUE ("asset_id"),
  CONSTRAINT "upload_claims_asset_fkey" FOREIGN KEY ("asset_id") REFERENCES "media"."assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "upload_claims_upload_fkey" FOREIGN KEY ("upload_id") REFERENCES "media"."staged_uploads" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "upload_claims_asset_check" CHECK ((kind <> 'CONFIRMED'::text) OR (asset_id IS NOT NULL)),
  CONSTRAINT "upload_claims_kind_check" CHECK (kind = ANY (ARRAY['CONFIRMED'::text, 'RECLAIMED'::text]))
);
