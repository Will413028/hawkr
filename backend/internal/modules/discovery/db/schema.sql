
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA discovery;

CREATE TABLE discovery.seller_scores (
    store_id                BIGINT PRIMARY KEY,
    score                   DOUBLE PRECISION NOT NULL,
    demand                  DOUBLE PRECISION NOT NULL,
    quality                 DOUBLE PRECISION NOT NULL,
    reliability_penalty     DOUBLE PRECISION NOT NULL,
    newness                 DOUBLE PRECISION NOT NULL,
    distinct_buyers         BIGINT NOT NULL,
    qualifying_orders       BIGINT NOT NULL,
    seller_cancelled_orders BIGINT NOT NULL,
    rating_count            BIGINT NOT NULL,
    rating_average          DOUBLE PRECISION,
    published_at            TIMESTAMPTZ NOT NULL,
    formula_version         INTEGER NOT NULL,
    computed_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT seller_scores_store_check CHECK (store_id > 0),
    CONSTRAINT seller_scores_score_check CHECK (score > '-Infinity'::double precision AND score < 'Infinity'::double precision),
    CONSTRAINT seller_scores_demand_check CHECK (demand BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_quality_check CHECK (quality BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_penalty_check CHECK (reliability_penalty BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_newness_check CHECK (newness BETWEEN 0 AND 1),
    CONSTRAINT seller_scores_counts_check CHECK (
        distinct_buyers >= 0 AND qualifying_orders >= 0 AND seller_cancelled_orders >= 0 AND rating_count >= 0
    ),
    CONSTRAINT seller_scores_rating_check CHECK (
        (rating_count = 0 AND rating_average IS NULL) OR (rating_count > 0 AND rating_average BETWEEN 0 AND 5)
    ),
    CONSTRAINT seller_scores_version_check CHECK (formula_version >= 1)
);

CREATE INDEX seller_scores_rank_idx ON discovery.seller_scores (score DESC, published_at DESC, store_id DESC);

CREATE TABLE discovery.recompute_state (
    job                       TEXT PRIMARY KEY,
    last_attempt_started_at   TIMESTAMPTZ NOT NULL,
    last_succeeded_started_at TIMESTAMPTZ,
    last_succeeded_at         TIMESTAMPTZ,
    last_failed_at            TIMESTAMPTZ,
    last_error                TEXT,
    CONSTRAINT recompute_state_job_check CHECK (job ~ '^[a-z][a-z_]{0,39}$'),
    CONSTRAINT recompute_state_success_check CHECK (
        (last_succeeded_started_at IS NULL AND last_succeeded_at IS NULL) OR
        (last_succeeded_started_at IS NOT NULL AND last_succeeded_at >= last_succeeded_started_at AND
         last_succeeded_started_at <= last_attempt_started_at)
    ),
    CONSTRAINT recompute_state_failure_check CHECK (
        (last_failed_at IS NULL AND last_error IS NULL) OR
        (last_failed_at IS NOT NULL AND last_error IS NOT NULL AND char_length(last_error) BETWEEN 1 AND 500)
    )
);
