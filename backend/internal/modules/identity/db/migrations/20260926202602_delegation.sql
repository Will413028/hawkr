-- Modify "admins" table
ALTER TABLE "identity"."admins" ADD COLUMN "delegation_enabled" boolean NOT NULL DEFAULT false;
-- Create "delegated_operation_sessions" table
CREATE TABLE "identity"."delegated_operation_sessions" (
  "id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY,
  "display_code" text NOT NULL,
  "credential_hash" bytea NOT NULL,
  "handoff_code_hash" bytea NULL,
  "handoff_expires_at" timestamptz NULL,
  "actor_admin_id" bigint NOT NULL,
  "actor_display_name" text NOT NULL,
  "target_store_id" bigint NOT NULL,
  "target_seller_account_id" bigint NOT NULL,
  "target_store_name" text NOT NULL,
  "reason_code" text NOT NULL,
  "reason_label" text NOT NULL,
  "case_reference" text NULL,
  "capabilities" text[] NOT NULL,
  "issued_at" timestamptz NOT NULL,
  "expires_at" timestamptz NOT NULL,
  "last_active_at" timestamptz NOT NULL,
  "ended_at" timestamptz NULL,
  "revoked_at" timestamptz NULL,
  "revoked_by_admin_id" bigint NULL,
  "revoke_reason" text NULL,
  "status" text NOT NULL,
  "revision" integer NOT NULL DEFAULT 0,
  "idempotency_key" text NOT NULL,
  "create_request_id" text NOT NULL,
  "create_correlation_id" text NOT NULL,
  "create_client_ip" text NULL,
  "create_user_agent" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "delegated_operation_sessions_actor_admin_id_fkey" FOREIGN KEY ("actor_admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "delegated_operation_sessions_revoked_by_admin_id_fkey" FOREIGN KEY ("revoked_by_admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "delegated_operation_sessions_capabilities_check" CHECK ((cardinality(capabilities) >= 1) AND (cardinality(capabilities) <= 9) AND (capabilities <@ ARRAY['storefront.read'::text, 'storefront.write'::text, 'products.read'::text, 'products.write'::text, 'products.publish'::text, 'posts.read'::text, 'posts.write'::text, 'posts.publish'::text, 'orders.read'::text]) AND (cardinality(capabilities) = ((((((((
CASE
    WHEN ('storefront.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END +
CASE
    WHEN ('storefront.write'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.write'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.publish'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.write'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.publish'::text = ANY (capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('orders.read'::text = ANY (capabilities)) THEN 1
    ELSE 0
END))),
  CONSTRAINT "delegated_operation_sessions_case_reference_check" CHECK ((case_reference IS NULL) OR ((char_length(case_reference) >= 4) AND (char_length(case_reference) <= 200) AND (case_reference ~ '[0-9]'::text))),
  CONSTRAINT "delegated_operation_sessions_display_code_check" CHECK (display_code ~ '^OPS-[0-9A-Z]{13}$'::text),
  CONSTRAINT "delegated_operation_sessions_handoff_check" CHECK ((handoff_code_hash IS NULL) OR (handoff_expires_at IS NOT NULL)),
  CONSTRAINT "delegated_operation_sessions_last_active_check" CHECK ((last_active_at >= issued_at) AND (last_active_at <= expires_at)),
  CONSTRAINT "delegated_operation_sessions_lifetime_check" CHECK ((expires_at > issued_at) AND (expires_at <= (issued_at + '00:30:00'::interval))),
  CONSTRAINT "delegated_operation_sessions_reason_code_check" CHECK (reason_code = ANY (ARRAY['seller_request'::text, 'platform_maintenance'::text, 'content_governance'::text])),
  CONSTRAINT "delegated_operation_sessions_revision_check" CHECK (revision >= 0),
  CONSTRAINT "delegated_operation_sessions_revoke_reason_check" CHECK ((revoke_reason IS NULL) OR ((char_length(revoke_reason) >= 1) AND (char_length(revoke_reason) <= 200))),
  CONSTRAINT "delegated_operation_sessions_revoked_check" CHECK ((status = 'revoked'::text) = (revoked_at IS NOT NULL)),
  CONSTRAINT "delegated_operation_sessions_second_precision_check" CHECK ((issued_at = date_trunc('second'::text, issued_at)) AND (expires_at = date_trunc('second'::text, expires_at)) AND (last_active_at = date_trunc('second'::text, last_active_at))),
  CONSTRAINT "delegated_operation_sessions_status_check" CHECK (status = ANY (ARRAY['active'::text, 'revoked'::text]))
);
-- Create index "idx_delegated_operation_sessions_actor_created_at" to table: "delegated_operation_sessions"
CREATE INDEX "idx_delegated_operation_sessions_actor_created_at" ON "identity"."delegated_operation_sessions" ("actor_admin_id", "created_at" DESC);
-- Create index "idx_delegated_operation_sessions_store_created_at" to table: "delegated_operation_sessions"
CREATE INDEX "idx_delegated_operation_sessions_store_created_at" ON "identity"."delegated_operation_sessions" ("target_store_id", "created_at" DESC);
-- Create index "uidx_delegated_operation_sessions_active_actor" to table: "delegated_operation_sessions"
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_active_actor" ON "identity"."delegated_operation_sessions" ("actor_admin_id") WHERE ((status = 'active'::text) AND (ended_at IS NULL));
-- Create index "uidx_delegated_operation_sessions_active_store" to table: "delegated_operation_sessions"
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_active_store" ON "identity"."delegated_operation_sessions" ("target_store_id") WHERE ((status = 'active'::text) AND (ended_at IS NULL));
-- Create index "uidx_delegated_operation_sessions_actor_idempotency" to table: "delegated_operation_sessions"
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_actor_idempotency" ON "identity"."delegated_operation_sessions" ("actor_admin_id", "idempotency_key");
-- Create index "uidx_delegated_operation_sessions_credential_hash" to table: "delegated_operation_sessions"
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_credential_hash" ON "identity"."delegated_operation_sessions" ("credential_hash");
-- Create index "uidx_delegated_operation_sessions_display_code" to table: "delegated_operation_sessions"
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_display_code" ON "identity"."delegated_operation_sessions" ("display_code");
-- Create index "uidx_delegated_operation_sessions_handoff_code_hash" to table: "delegated_operation_sessions"
CREATE UNIQUE INDEX "uidx_delegated_operation_sessions_handoff_code_hash" ON "identity"."delegated_operation_sessions" ("handoff_code_hash") WHERE (handoff_code_hash IS NOT NULL);
-- Create "admin_operation_audit_logs" table
CREATE TABLE "identity"."admin_operation_audit_logs" (
  "id" uuid NOT NULL,
  "session_id" bigint NOT NULL,
  "actor_admin_id" bigint NOT NULL,
  "actor_display_name" text NOT NULL,
  "target_store_id" bigint NOT NULL,
  "target_seller_account_id" bigint NOT NULL,
  "target_store_name" text NOT NULL,
  "origin" text NOT NULL,
  "capability" text NOT NULL,
  "action" text NOT NULL,
  "resource_type" text NOT NULL,
  "resource_id" text NULL,
  "resource_name" text NULL,
  "result" text NOT NULL,
  "biz_code" integer NULL,
  "operation_reason" text NULL,
  "case_reference" text NULL,
  "request_id" text NOT NULL,
  "correlation_id" text NOT NULL,
  "idempotency_key" text NULL,
  "payload_hash" text NULL,
  "before_snapshot" jsonb NULL,
  "after_snapshot" jsonb NULL,
  "changed_fields" jsonb NULL,
  "resource_version_before" bigint NULL,
  "resource_version_after" bigint NULL,
  "client_ip" text NULL,
  "user_agent" text NULL,
  "risk_level" text NOT NULL,
  "notify_seller" boolean NOT NULL,
  "notified_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "confirmed_at" timestamptz NULL,
  PRIMARY KEY ("id"),
  CONSTRAINT "admin_operation_audit_logs_actor_admin_id_fkey" FOREIGN KEY ("actor_admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "admin_operation_audit_logs_session_id_fkey" FOREIGN KEY ("session_id") REFERENCES "identity"."delegated_operation_sessions" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "admin_operation_audit_logs_action_check" CHECK (action = ANY (ARRAY['STOREFRONT_UPDATE'::text, 'STOREFRONT_LOGO_UPDATE'::text, 'STOREFRONT_COVER_UPDATE'::text, 'PRODUCT_CREATE'::text, 'PRODUCT_UPDATE'::text, 'PRODUCT_PUBLISH'::text, 'PRODUCT_UNPUBLISH'::text, 'PRODUCT_BATCH_UNPUBLISH'::text, 'POST_CREATE'::text, 'POST_UPDATE'::text, 'POST_PUBLISH'::text, 'POST_UNPUBLISH'::text, 'POST_SCHEDULE'::text, 'ORDER_PII_REVEAL'::text, 'STOREFRONT_READ'::text, 'PRODUCT_READ'::text, 'POST_READ'::text, 'ORDER_READ'::text, 'AUDIT_CORRECTION'::text])),
  CONSTRAINT "admin_operation_audit_logs_biz_code_check" CHECK ((result = 'FAILED'::text) OR (biz_code IS NULL)),
  CONSTRAINT "admin_operation_audit_logs_capability_check" CHECK (capability = ANY (ARRAY['storefront.read'::text, 'storefront.write'::text, 'products.read'::text, 'products.write'::text, 'products.publish'::text, 'posts.read'::text, 'posts.write'::text, 'posts.publish'::text, 'orders.read'::text])),
  CONSTRAINT "admin_operation_audit_logs_confirmed_check" CHECK ((result = 'PENDING'::text) = (confirmed_at IS NULL)),
  CONSTRAINT "admin_operation_audit_logs_notified_check" CHECK ((notified_at IS NULL) OR (notify_seller AND (result = 'SUCCEEDED'::text))),
  CONSTRAINT "admin_operation_audit_logs_origin_check" CHECK (origin = ANY (ARRAY['interactive'::text, 'scheduled'::text])),
  CONSTRAINT "admin_operation_audit_logs_reason_check" CHECK ((operation_reason IS NULL) OR ((char_length(operation_reason) >= 1) AND (char_length(operation_reason) <= 100))),
  CONSTRAINT "admin_operation_audit_logs_resource_type_check" CHECK (resource_type = ANY (ARRAY['storefront'::text, 'product'::text, 'post'::text, 'order'::text])),
  CONSTRAINT "admin_operation_audit_logs_result_check" CHECK (result = ANY (ARRAY['PENDING'::text, 'SUCCEEDED'::text, 'FAILED'::text])),
  CONSTRAINT "admin_operation_audit_logs_risk_level_check" CHECK (risk_level = ANY (ARRAY['low'::text, 'medium'::text, 'high'::text]))
);
-- Create index "idx_admin_operation_audit_logs_actor_created_at" to table: "admin_operation_audit_logs"
CREATE INDEX "idx_admin_operation_audit_logs_actor_created_at" ON "identity"."admin_operation_audit_logs" ("actor_admin_id", "created_at" DESC);
-- Create index "idx_admin_operation_audit_logs_pending" to table: "admin_operation_audit_logs"
CREATE INDEX "idx_admin_operation_audit_logs_pending" ON "identity"."admin_operation_audit_logs" ("created_at") WHERE (result = 'PENDING'::text);
-- Create index "idx_admin_operation_audit_logs_session_created_at" to table: "admin_operation_audit_logs"
CREATE INDEX "idx_admin_operation_audit_logs_session_created_at" ON "identity"."admin_operation_audit_logs" ("session_id", "created_at" DESC);
-- Create index "idx_admin_operation_audit_logs_store_created_at" to table: "admin_operation_audit_logs"
CREATE INDEX "idx_admin_operation_audit_logs_store_created_at" ON "identity"."admin_operation_audit_logs" ("target_store_id", "created_at" DESC);
-- Create index "uidx_admin_operation_audit_logs_idempotency" to table: "admin_operation_audit_logs"
CREATE UNIQUE INDEX "uidx_admin_operation_audit_logs_idempotency" ON "identity"."admin_operation_audit_logs" ("session_id", "idempotency_key") WHERE (idempotency_key IS NOT NULL);
-- Create "seller_delegation_authorizations" table
CREATE TABLE "identity"."seller_delegation_authorizations" (
  "id" uuid NOT NULL,
  "target_store_id" bigint NULL,
  "source" text NOT NULL,
  "source_reference" text NOT NULL,
  "granted_capabilities" text[] NOT NULL,
  "effective_from" timestamptz NOT NULL,
  "effective_until" timestamptz NULL,
  "approved_by_admin_id" bigint NOT NULL,
  "approved_at" timestamptz NOT NULL,
  "revoked_at" timestamptz NULL,
  "revoked_by_admin_id" bigint NULL,
  "revoke_reason" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "seller_delegation_authorizations_approved_by_admin_id_fkey" FOREIGN KEY ("approved_by_admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "seller_delegation_authorizations_revoked_by_admin_id_fkey" FOREIGN KEY ("revoked_by_admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "seller_delegation_authorizations_capabilities_check" CHECK ((cardinality(granted_capabilities) >= 1) AND (cardinality(granted_capabilities) <= 9) AND (granted_capabilities <@ ARRAY['storefront.read'::text, 'storefront.write'::text, 'products.read'::text, 'products.write'::text, 'products.publish'::text, 'posts.read'::text, 'posts.write'::text, 'posts.publish'::text, 'orders.read'::text]) AND (cardinality(granted_capabilities) = ((((((((
CASE
    WHEN ('storefront.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END +
CASE
    WHEN ('storefront.write'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.write'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('products.publish'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.write'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('posts.publish'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END) +
CASE
    WHEN ('orders.read'::text = ANY (granted_capabilities)) THEN 1
    ELSE 0
END))),
  CONSTRAINT "seller_delegation_authorizations_global_check" CHECK ((target_store_id IS NOT NULL) OR (source = 'platform_policy'::text)),
  CONSTRAINT "seller_delegation_authorizations_period_check" CHECK ((effective_until IS NULL) OR (effective_until > effective_from)),
  CONSTRAINT "seller_delegation_authorizations_revoke_reason_check" CHECK ((revoke_reason IS NULL) OR ((char_length(revoke_reason) >= 1) AND (char_length(revoke_reason) <= 200))),
  CONSTRAINT "seller_delegation_authorizations_revoked_check" CHECK (((revoked_at IS NULL) = (revoked_by_admin_id IS NULL)) AND ((revoked_at IS NULL) = (revoke_reason IS NULL))),
  CONSTRAINT "seller_delegation_authorizations_source_check" CHECK (source = ANY (ARRAY['contract'::text, 'ticket'::text, 'platform_policy'::text])),
  CONSTRAINT "seller_delegation_authorizations_source_reference_check" CHECK ((char_length(source_reference) >= 4) AND (char_length(source_reference) <= 200) AND (source_reference ~ '[0-9]'::text))
);
-- Create index "uidx_seller_delegation_authorizations_global" to table: "seller_delegation_authorizations"
CREATE UNIQUE INDEX "uidx_seller_delegation_authorizations_global" ON "identity"."seller_delegation_authorizations" (((target_store_id IS NULL))) WHERE (target_store_id IS NULL);
-- Create index "uidx_seller_delegation_authorizations_store" to table: "seller_delegation_authorizations"
CREATE UNIQUE INDEX "uidx_seller_delegation_authorizations_store" ON "identity"."seller_delegation_authorizations" ("target_store_id") WHERE (target_store_id IS NOT NULL);
-- Create "delegation_authorization_audit_logs" table
CREATE TABLE "identity"."delegation_authorization_audit_logs" (
  "id" uuid NOT NULL,
  "authorization_id" uuid NOT NULL,
  "target_store_id" bigint NOT NULL,
  "action" text NOT NULL,
  "actor_admin_id" bigint NOT NULL,
  "actor_display_name" text NOT NULL,
  "operation_reason" text NOT NULL,
  "source" text NULL,
  "source_reference" text NULL,
  "capabilities_before" text[] NULL,
  "capabilities_after" text[] NULL,
  "effective_until_before" timestamptz NULL,
  "effective_until_after" timestamptz NULL,
  "idempotency_key" text NULL,
  "request_id" text NULL,
  "correlation_id" text NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "delegation_authorization_audit_logs_actor_admin_id_fkey" FOREIGN KEY ("actor_admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "delegation_authorization_audit_logs_authorization_id_fkey" FOREIGN KEY ("authorization_id") REFERENCES "identity"."seller_delegation_authorizations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "delegation_authorization_audit_logs_action_check" CHECK (action = ANY (ARRAY['GRANT'::text, 'REINSTATE'::text, 'UPDATE'::text, 'REVOKE'::text])),
  CONSTRAINT "delegation_authorization_audit_logs_reason_check" CHECK ((char_length(operation_reason) >= 1) AND (char_length(operation_reason) <= 200))
);
-- Create index "idx_delegation_authorization_audit_logs_store_created_at" to table: "delegation_authorization_audit_logs"
CREATE INDEX "idx_delegation_authorization_audit_logs_store_created_at" ON "identity"."delegation_authorization_audit_logs" ("target_store_id", "created_at" DESC);
-- Create index "uidx_delegation_authorization_audit_logs_idempotency" to table: "delegation_authorization_audit_logs"
CREATE UNIQUE INDEX "uidx_delegation_authorization_audit_logs_idempotency" ON "identity"."delegation_authorization_audit_logs" ("authorization_id", "idempotency_key") WHERE (idempotency_key IS NOT NULL);
