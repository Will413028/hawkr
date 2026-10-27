-- Create "store_exposures" table
CREATE TABLE "seller"."store_exposures" (
  "store_id" bigint NOT NULL,
  "exposable" boolean NOT NULL DEFAULT false,
  "version" bigint NOT NULL DEFAULT 0,
  "announced_at" timestamptz NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "store_exposures_store_id_fkey" FOREIGN KEY ("store_id") REFERENCES "seller"."stores" ("store_id") ON UPDATE NO ACTION ON DELETE NO ACTION,
  CONSTRAINT "store_exposures_announced_check" CHECK ((version = 0) = (announced_at IS NULL)),
  CONSTRAINT "store_exposures_placeholder_check" CHECK ((version > 0) OR (NOT exposable)),
  CONSTRAINT "store_exposures_version_check" CHECK (version >= 0)
);
-- Create index "store_exposures_exposable" to table: "store_exposures"
CREATE INDEX "store_exposures_exposable" ON "seller"."store_exposures" ("store_id") WHERE exposable;
-- Backfill: every existing store gets its never-announced row (the first announcement creates version 1)
INSERT INTO "seller"."store_exposures" ("store_id") SELECT "store_id" FROM "seller"."stores";
