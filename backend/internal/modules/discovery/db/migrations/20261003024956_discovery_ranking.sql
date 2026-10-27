-- Create "recompute_state" table
CREATE TABLE "discovery"."recompute_state" (
  "job" text NOT NULL,
  "last_attempt_started_at" timestamptz NOT NULL,
  "last_succeeded_started_at" timestamptz NULL,
  "last_succeeded_at" timestamptz NULL,
  "last_failed_at" timestamptz NULL,
  "last_error" text NULL,
  PRIMARY KEY ("job"),
  CONSTRAINT "recompute_state_failure_check" CHECK (((last_failed_at IS NULL) AND (last_error IS NULL)) OR ((last_failed_at IS NOT NULL) AND (last_error IS NOT NULL) AND ((char_length(last_error) >= 1) AND (char_length(last_error) <= 500)))),
  CONSTRAINT "recompute_state_job_check" CHECK (job ~ '^[a-z][a-z_]{0,39}$'::text),
  CONSTRAINT "recompute_state_success_check" CHECK (((last_succeeded_started_at IS NULL) AND (last_succeeded_at IS NULL)) OR ((last_succeeded_started_at IS NOT NULL) AND (last_succeeded_at >= last_succeeded_started_at) AND (last_succeeded_started_at <= last_attempt_started_at)))
);
-- Create "seller_scores" table
CREATE TABLE "discovery"."seller_scores" (
  "store_id" bigint NOT NULL,
  "score" double precision NOT NULL,
  "demand" double precision NOT NULL,
  "quality" double precision NOT NULL,
  "reliability_penalty" double precision NOT NULL,
  "newness" double precision NOT NULL,
  "distinct_buyers" bigint NOT NULL,
  "qualifying_orders" bigint NOT NULL,
  "seller_cancelled_orders" bigint NOT NULL,
  "rating_count" bigint NOT NULL,
  "rating_average" double precision NULL,
  "exposable_since" timestamptz NOT NULL,
  "formula_version" integer NOT NULL,
  "computed_at" timestamptz NOT NULL,
  PRIMARY KEY ("store_id"),
  CONSTRAINT "seller_scores_counts_check" CHECK ((distinct_buyers >= 0) AND (qualifying_orders >= 0) AND (seller_cancelled_orders >= 0) AND (rating_count >= 0)),
  CONSTRAINT "seller_scores_demand_check" CHECK ((demand >= (0)::double precision) AND (demand <= (1)::double precision)),
  CONSTRAINT "seller_scores_newness_check" CHECK ((newness >= (0)::double precision) AND (newness <= (1)::double precision)),
  CONSTRAINT "seller_scores_penalty_check" CHECK ((reliability_penalty >= (0)::double precision) AND (reliability_penalty <= (1)::double precision)),
  CONSTRAINT "seller_scores_quality_check" CHECK ((quality >= (0)::double precision) AND (quality <= (1)::double precision)),
  CONSTRAINT "seller_scores_rating_check" CHECK (((rating_count = 0) AND (rating_average IS NULL)) OR ((rating_count > 0) AND ((rating_average >= (0)::double precision) AND (rating_average <= (5)::double precision)))),
  CONSTRAINT "seller_scores_score_check" CHECK ((score > '-Infinity'::double precision) AND (score < 'Infinity'::double precision)),
  CONSTRAINT "seller_scores_store_check" CHECK (store_id > 0),
  CONSTRAINT "seller_scores_version_check" CHECK (formula_version >= 1)
);
-- Create index "seller_scores_rank_idx" to table: "seller_scores"
CREATE INDEX "seller_scores_rank_idx" ON "discovery"."seller_scores" ("score" DESC, "exposable_since" DESC, "store_id" DESC);
