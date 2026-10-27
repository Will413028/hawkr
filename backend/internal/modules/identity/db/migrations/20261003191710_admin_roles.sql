-- Create "admin_role_events" table
CREATE TABLE "identity"."admin_role_events" (
  "id" uuid NOT NULL DEFAULT uuidv7(),
  "admin_id" bigint NOT NULL,
  "role" text NOT NULL,
  "action" text NOT NULL,
  "reason" text NOT NULL,
  "source" text NOT NULL,
  "db_user" text NOT NULL,
  "os_user" text NULL,
  "occurred_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "admin_role_events_admin_id_fkey" FOREIGN KEY ("admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "admin_role_events_action_check" CHECK (action = ANY (ARRAY['GRANT'::text, 'REVOKE'::text])),
  CONSTRAINT "admin_role_events_reason_check" CHECK ((reason = btrim(reason)) AND ((char_length(reason) >= 1) AND (char_length(reason) <= 1000))),
  CONSTRAINT "admin_role_events_role_check" CHECK (role = 'OWNER'::text),
  CONSTRAINT "admin_role_events_source_check" CHECK (source = 'CLI'::text)
);
-- Create index "idx_admin_role_events_admin_id_occurred_at" to table: "admin_role_events"
CREATE INDEX "idx_admin_role_events_admin_id_occurred_at" ON "identity"."admin_role_events" ("admin_id", "occurred_at" DESC);
-- Create "admin_roles" table
CREATE TABLE "identity"."admin_roles" (
  "admin_id" bigint NOT NULL,
  "role" text NOT NULL,
  "granted_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("admin_id", "role"),
  CONSTRAINT "admin_roles_admin_id_fkey" FOREIGN KEY ("admin_id") REFERENCES "identity"."admins" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "admin_roles_role_check" CHECK (role = 'OWNER'::text)
);
