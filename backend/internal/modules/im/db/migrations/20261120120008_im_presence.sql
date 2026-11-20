-- Create "presence_streams" table
CREATE UNLOGGED TABLE "im"."presence_streams" (
  "stream_id" uuid NOT NULL,
  "kind" text NOT NULL,
  "party_id" bigint NOT NULL,
  "expires_at" timestamptz NOT NULL,
  PRIMARY KEY ("stream_id"),
  CONSTRAINT "presence_streams_kind_check" CHECK (kind = ANY (ARRAY['BUYER'::text, 'STORE'::text])),
  CONSTRAINT "presence_streams_party_check" CHECK (party_id > 0)
);
-- Create index "presence_streams_party" to table: "presence_streams"
CREATE INDEX "presence_streams_party" ON "im"."presence_streams" ("kind", "party_id", "expires_at");
