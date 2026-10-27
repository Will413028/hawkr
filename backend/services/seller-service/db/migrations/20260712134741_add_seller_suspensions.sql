-- Create "seller_suspensions" table
CREATE TABLE "seller_suspensions" (
  "id" bigint NOT NULL,
  "seller_id" bigint NOT NULL,
  "reason_code" character varying(30) NOT NULL,
  "reason_detail" text NULL,
  "status" character varying(20) NOT NULL DEFAULT 'active',
  "suspended_by_admin_id" bigint NOT NULL,
  "suspended_at" timestamptz NOT NULL DEFAULT now(),
  "lifted_by_admin_id" bigint NULL,
  "lifted_at" timestamptz NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id")
);
-- Create index "uq_seller_suspensions_active" to table: "seller_suspensions"
CREATE UNIQUE INDEX "uq_seller_suspensions_active" ON "seller_suspensions" ("seller_id") WHERE ((status)::text = 'active'::text);
