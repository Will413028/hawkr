-- Create "seller_ranking_facts" table
CREATE TABLE "seller_ranking_facts" (
  "store_id" bigint NOT NULL,
  "tier_rank" integer NULL,
  "monthly_order_cap" integer NULL,
  "monthly_completed_orders" integer NOT NULL DEFAULT 0,
  "count_period" character varying(7) NOT NULL DEFAULT '',
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("store_id")
);
-- Create "seller_ranking_processed_events" table
CREATE TABLE "seller_ranking_processed_events" (
  "event_id" text NOT NULL,
  "processed_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("event_id")
);
