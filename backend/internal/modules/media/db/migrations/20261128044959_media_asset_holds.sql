-- Create "orphan_objects" table
CREATE TABLE "media"."orphan_objects" (
  "object_key" text NOT NULL,
  "recorded_at" timestamptz NOT NULL,
  PRIMARY KEY ("object_key"),
  CONSTRAINT "orphan_objects_object_key_check" CHECK (object_key ~~ 'secure/%'::text)
);
-- Create "asset_copy_requests" table
CREATE TABLE "media"."asset_copy_requests" (
  "asset_id" uuid NOT NULL,
  "source_asset_id" uuid NOT NULL,
  "requested_at" timestamptz NOT NULL,
  "completed_at" timestamptz NULL,
  "failed_at" timestamptz NULL,
  PRIMARY KEY ("asset_id"),
  CONSTRAINT "asset_copy_requests_asset_fkey" FOREIGN KEY ("asset_id") REFERENCES "media"."assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "asset_copy_requests_source_fkey" FOREIGN KEY ("source_asset_id") REFERENCES "media"."assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "asset_copy_requests_outcome_check" CHECK ((completed_at IS NULL) OR (failed_at IS NULL)),
  CONSTRAINT "asset_copy_requests_self_check" CHECK (asset_id <> source_asset_id)
);
-- Create index "asset_copy_requests_pending" to table: "asset_copy_requests"
CREATE INDEX "asset_copy_requests_pending" ON "media"."asset_copy_requests" ("requested_at") WHERE ((completed_at IS NULL) AND (failed_at IS NULL));
-- Create "asset_gates" table
CREATE TABLE "media"."asset_gates" (
  "asset_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("asset_id"),
  CONSTRAINT "asset_gates_kind_key" UNIQUE ("asset_id", "kind"),
  CONSTRAINT "asset_gates_asset_fkey" FOREIGN KEY ("asset_id") REFERENCES "media"."assets" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "asset_gates_kind_check" CHECK (kind = ANY (ARRAY['HOLD'::text, 'DELETE'::text]))
);
-- Create "asset_holds" table
CREATE TABLE "media"."asset_holds" (
  "hold_id" uuid NOT NULL,
  "asset_id" uuid NOT NULL,
  "gate_kind" text NOT NULL DEFAULT 'HOLD',
  "source" text NOT NULL,
  "source_id" uuid NOT NULL,
  "until" timestamptz NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("hold_id"),
  CONSTRAINT "asset_holds_gate_fkey" FOREIGN KEY ("asset_id", "gate_kind") REFERENCES "media"."asset_gates" ("asset_id", "kind") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "asset_holds_gate_kind_check" CHECK (gate_kind = 'HOLD'::text),
  CONSTRAINT "asset_holds_source_check" CHECK (source <> ''::text)
);
-- Create index "asset_holds_asset" to table: "asset_holds"
CREATE INDEX "asset_holds_asset" ON "media"."asset_holds" ("asset_id");
-- Create index "asset_holds_source" to table: "asset_holds"
CREATE INDEX "asset_holds_source" ON "media"."asset_holds" ("source", "source_id");
-- Create index "asset_holds_until" to table: "asset_holds"
CREATE INDEX "asset_holds_until" ON "media"."asset_holds" ("until");
-- Create "asset_hold_releases" table
CREATE TABLE "media"."asset_hold_releases" (
  "hold_id" uuid NOT NULL,
  "released_at" timestamptz NOT NULL,
  PRIMARY KEY ("hold_id"),
  CONSTRAINT "asset_hold_releases_hold_fkey" FOREIGN KEY ("hold_id") REFERENCES "media"."asset_holds" ("hold_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
