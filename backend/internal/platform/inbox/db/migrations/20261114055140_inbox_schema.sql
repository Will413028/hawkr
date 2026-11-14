-- Add new schema named "inbox"
CREATE SCHEMA "inbox";
-- Create "events" table
CREATE TABLE "inbox"."events" (
  "provider" text NOT NULL,
  "event_id" text NOT NULL,
  "route_key" text NOT NULL,
  "payload" bytea NOT NULL,
  "received_at" timestamptz NOT NULL,
  "purge_after" timestamptz NOT NULL,
  "dispatched_at" timestamptz NULL,
  "processed_at" timestamptz NULL,
  "processed_by" text NULL,
  PRIMARY KEY ("provider", "event_id"),
  CONSTRAINT "events_event_id_check" CHECK ((char_length(event_id) >= 1) AND (char_length(event_id) <= 255)),
  CONSTRAINT "events_processed_by_check" CHECK ((processed_by IS NULL) OR ((char_length(processed_by) >= 1) AND (char_length(processed_by) <= 100))),
  CONSTRAINT "events_processed_check" CHECK ((processed_at IS NULL) = (processed_by IS NULL)),
  CONSTRAINT "events_provider_check" CHECK (provider ~ '^[a-z][a-z0-9_]{0,39}$'::text),
  CONSTRAINT "events_purge_after_check" CHECK (purge_after > received_at),
  CONSTRAINT "events_route_key_check" CHECK ((char_length(route_key) >= 1) AND (char_length(route_key) <= 255))
);
-- Create index "events_pending" to table: "events"
CREATE INDEX "events_pending" ON "inbox"."events" ("provider", "route_key", "received_at") WHERE (processed_at IS NULL);
-- Create index "events_purge" to table: "events"
CREATE INDEX "events_purge" ON "inbox"."events" ("purge_after") WHERE (processed_at IS NOT NULL);
-- Create index "events_undispatched" to table: "events"
CREATE INDEX "events_undispatched" ON "inbox"."events" ("received_at") WHERE ((processed_at IS NULL) AND (dispatched_at IS NULL));
