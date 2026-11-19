-- Add new schema named "assistant"
CREATE SCHEMA "assistant";
-- Create "app_states" table
CREATE TABLE "assistant"."app_states" (
  "app_name" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("app_name"),
  CONSTRAINT "app_states_state_check" CHECK (jsonb_typeof(state) = 'object'::text)
);
-- Create "sessions" table
CREATE TABLE "assistant"."sessions" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "id" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "created_at" timestamptz NOT NULL,
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("app_name", "user_id", "id"),
  CONSTRAINT "sessions_key_check" CHECK ((app_name <> ''::text) AND (user_id <> ''::text) AND (id <> ''::text)),
  CONSTRAINT "sessions_state_check" CHECK (jsonb_typeof(state) = 'object'::text)
);
-- Create "user_states" table
CREATE TABLE "assistant"."user_states" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "state" jsonb NOT NULL DEFAULT '{}',
  "updated_at" timestamptz NOT NULL,
  PRIMARY KEY ("app_name", "user_id"),
  CONSTRAINT "user_states_state_check" CHECK (jsonb_typeof(state) = 'object'::text)
);
-- Create "events" table
CREATE TABLE "assistant"."events" (
  "app_name" text NOT NULL,
  "user_id" text NOT NULL,
  "session_id" text NOT NULL,
  "id" text NOT NULL,
  "timestamp" timestamptz NOT NULL,
  "event" jsonb NOT NULL,
  PRIMARY KEY ("app_name", "user_id", "session_id", "id"),
  CONSTRAINT "events_app_name_user_id_session_id_fkey" FOREIGN KEY ("app_name", "user_id", "session_id") REFERENCES "assistant"."sessions" ("app_name", "user_id", "id") ON UPDATE NO ACTION ON DELETE CASCADE,
  CONSTRAINT "events_event_check" CHECK (jsonb_typeof(event) = 'object'::text),
  CONSTRAINT "events_id_check" CHECK (id <> ''::text)
);
-- Create index "events_session_time" to table: "events"
CREATE INDEX "events_session_time" ON "assistant"."events" ("app_name", "user_id", "session_id", "timestamp", "id");
