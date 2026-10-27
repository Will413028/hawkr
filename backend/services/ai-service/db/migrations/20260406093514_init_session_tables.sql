-- Create "app_states" table
CREATE TABLE "app_states" (
  "app_name" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "update_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("app_name")
);
-- Create "sessions" table
CREATE TABLE "sessions" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "id" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "create_time" timestamptz NOT NULL DEFAULT now(),
  "update_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("app_name", "user_id", "id")
);
-- Create index "idx_sessions_update_time" to table: "sessions"
CREATE INDEX "idx_sessions_update_time" ON "sessions" ("update_time");
-- Create "user_states" table
CREATE TABLE "user_states" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "update_time" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("app_name", "user_id")
);
-- Create "events" table
CREATE TABLE "events" (
  "id" text NOT NULL,
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "session_id" text NOT NULL,
  "invocation_id" text NOT NULL DEFAULT '',
  "author" text NOT NULL DEFAULT '',
  "actions" bytea NULL,
  "long_running_tool_ids_json" jsonb NULL,
  "branch" text NULL,
  "timestamp" timestamptz NULL,
  "content" jsonb NULL,
  "grounding_metadata" jsonb NULL,
  "custom_metadata" jsonb NULL,
  "usage_metadata" jsonb NULL,
  "citation_metadata" jsonb NULL,
  "partial" boolean NULL,
  "turn_complete" boolean NULL,
  "error_code" text NULL,
  "error_message" text NULL,
  "interrupted" boolean NULL,
  PRIMARY KEY ("id", "app_name", "user_id", "session_id"),
  CONSTRAINT "fk_sessions_events" FOREIGN KEY ("app_name", "user_id", "session_id") REFERENCES "sessions" ("app_name", "user_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE
);
-- Create index "idx_events_timestamp" to table: "events"
CREATE INDEX "idx_events_timestamp" ON "events" ("timestamp");
