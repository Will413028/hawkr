-- Create "notification_templates" table
CREATE TABLE "notification_templates" (
  "id" bigserial NOT NULL,
  "exchange" character varying(100) NOT NULL,
  "routing_key" character varying(200) NOT NULL,
  "queue_name" character varying(200) NOT NULL,
  "event_type" character varying(100) NOT NULL,
  "target_user_type" character varying(10) NOT NULL,
  "target_user_field" character varying(100) NOT NULL,
  "category" character varying(30) NOT NULL,
  "title_template" text NOT NULL,
  "body_template" text NOT NULL,
  "body_fallback" text NULL,
  "data_type" character varying(100) NOT NULL,
  "data_fields" jsonb NULL,
  "schema_url" character varying(500) NULL,
  "reference_id_template" character varying(200) NULL,
  "fcm_topic" character varying(200) NULL,
  "enabled" boolean NOT NULL DEFAULT true,
  "updated_by" bigint NULL,
  "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY ("id"),
  CONSTRAINT "uni_notification_templates_event_user_type" UNIQUE ("event_type", "target_user_type")
);
-- Create index "idx_notification_templates_enabled" to table: "notification_templates"
CREATE INDEX "idx_notification_templates_enabled" ON "notification_templates" ("enabled");
-- Create index "idx_notification_templates_queue_routing" to table: "notification_templates"
CREATE INDEX "idx_notification_templates_queue_routing" ON "notification_templates" ("queue_name", "routing_key");
