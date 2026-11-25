-- Drop index "events_pending" from table: "events"
DROP INDEX "inbox"."events_pending";
-- Drop index "events_purge" from table: "events"
DROP INDEX "inbox"."events_purge";
-- Drop index "events_undispatched" from table: "events"
DROP INDEX "inbox"."events_undispatched";
-- Modify "events" table
ALTER TABLE "inbox"."events" ADD CONSTRAINT "events_dead_check" CHECK (((dead_at IS NULL) = (dead_reason IS NULL)) AND ((dead_at IS NULL) OR (processed_at IS NULL))), ADD CONSTRAINT "events_dead_reason_check" CHECK ((dead_reason IS NULL) OR (dead_reason = ANY (ARRAY['undispatched'::text, 'unprocessed'::text]))), ADD COLUMN "dead_at" timestamptz NULL, ADD COLUMN "dead_reason" text NULL;
-- Create index "events_pending" to table: "events"
CREATE INDEX "events_pending" ON "inbox"."events" ("provider", "route_key", "received_at") WHERE ((processed_at IS NULL) AND (dead_at IS NULL));
-- Create index "events_purge" to table: "events"
CREATE INDEX "events_purge" ON "inbox"."events" ("purge_after") WHERE ((processed_at IS NOT NULL) OR (dead_at IS NOT NULL));
-- Create index "events_undispatched" to table: "events"
CREATE INDEX "events_undispatched" ON "inbox"."events" ("received_at") WHERE ((processed_at IS NULL) AND (dead_at IS NULL) AND (dispatched_at IS NULL));
-- Create index "events_dead" to table: "events"
CREATE INDEX "events_dead" ON "inbox"."events" ("dead_at") WHERE (dead_at IS NOT NULL);
