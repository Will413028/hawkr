-- name: DeleteSellerScores :exec
DELETE FROM discovery.seller_scores;

-- name: InsertSellerScores :exec
INSERT INTO discovery.seller_scores (
    store_id, score, demand, quality, reliability_penalty, newness, distinct_buyers, qualifying_orders,
    seller_cancelled_orders, rating_count, rating_average, published_at, formula_version, computed_at
)
SELECT r.store_id, r.score, r.demand, r.quality, r.reliability_penalty, r.newness, r.distinct_buyers, r.qualifying_orders,
       r.seller_cancelled_orders, r.rating_count, r.rating_average, r.published_at, r.formula_version, r.computed_at
FROM jsonb_to_recordset(sqlc.arg(rows)::jsonb) AS r(
    store_id bigint, score double precision, demand double precision, quality double precision,
    reliability_penalty double precision, newness double precision, distinct_buyers bigint, qualifying_orders bigint,
    seller_cancelled_orders bigint, rating_count bigint, rating_average double precision, published_at timestamptz,
    formula_version integer, computed_at timestamptz
);

-- name: FirstSellerScores :many
SELECT store_id, qualifying_orders, rating_count, rating_average, score, published_at
FROM discovery.seller_scores
ORDER BY score DESC, published_at DESC, store_id DESC
LIMIT sqlc.arg(page_size);

-- name: NextSellerScores :many
SELECT store_id, qualifying_orders, rating_count, rating_average, score, published_at
FROM discovery.seller_scores
WHERE (score, published_at, store_id) < (sqlc.arg(after_score)::double precision, sqlc.arg(after_published_at)::timestamptz, sqlc.arg(after_store_id)::bigint)
ORDER BY score DESC, published_at DESC, store_id DESC
LIMIT sqlc.arg(page_size);
