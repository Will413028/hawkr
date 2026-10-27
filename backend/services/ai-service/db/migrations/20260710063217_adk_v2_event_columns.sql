-- Modify "events" table
ALTER TABLE "events" ADD COLUMN "routes_json" jsonb NULL, ADD COLUMN "output_json" jsonb NULL, ADD COLUMN "node_info_json" jsonb NULL, ADD COLUMN "requested_input_json" jsonb NULL, ADD COLUMN "isolation_scope" text NULL;
