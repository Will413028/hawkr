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
-- Modify "asset_holds" table
DELETE FROM "media"."asset_holds";
ALTER TABLE "media"."asset_holds" DROP CONSTRAINT "asset_holds_pkey", DROP CONSTRAINT "asset_holds_asset_fkey", ADD CONSTRAINT "asset_holds_gate_kind_check" CHECK (gate_kind = 'HOLD'::text), ADD COLUMN "hold_id" uuid NOT NULL, ADD COLUMN "gate_kind" text NOT NULL DEFAULT 'HOLD', ADD PRIMARY KEY ("hold_id"), ADD CONSTRAINT "asset_holds_gate_fkey" FOREIGN KEY ("asset_id", "gate_kind") REFERENCES "media"."asset_gates" ("asset_id", "kind") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Create index "asset_holds_asset" to table: "asset_holds"
CREATE INDEX "asset_holds_asset" ON "media"."asset_holds" ("asset_id");
-- Create "asset_hold_releases" table
CREATE TABLE "media"."asset_hold_releases" (
  "hold_id" uuid NOT NULL,
  "released_at" timestamptz NOT NULL,
  PRIMARY KEY ("hold_id"),
  CONSTRAINT "asset_hold_releases_hold_fkey" FOREIGN KEY ("hold_id") REFERENCES "media"."asset_holds" ("hold_id") ON UPDATE NO ACTION ON DELETE NO ACTION
);
