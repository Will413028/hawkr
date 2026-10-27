-- name: BatchGetSellerRankingFacts :many
SELECT * FROM seller_ranking_facts WHERE store_id = ANY(sqlc.arg(store_ids)::bigint[]);

-- name: ClaimSellerRankingProcessedEvent :execrows
INSERT INTO seller_ranking_processed_events (event_id) VALUES ($1) ON CONFLICT DO NOTHING;

-- name: UpsertSellerRankingOrderCount :exec
INSERT INTO seller_ranking_facts (store_id, monthly_completed_orders, count_period, updated_at)
VALUES ($1, 1, $2, now())
ON CONFLICT (store_id) DO UPDATE SET
  monthly_completed_orders = CASE
    WHEN seller_ranking_facts.count_period = $2 THEN seller_ranking_facts.monthly_completed_orders + 1
    ELSE 1
  END,
  count_period = $2,
  updated_at = now();

-- name: UpsertSellerRankingTier :exec
-- WHERE 子句擋亂序：只在新事件的 effective_at 不比目前存的舊（或目前還沒有任何值）時
-- 才覆寫，否則整筆 UPDATE 不生效（PostgreSQL 對 ON CONFLICT ... DO UPDATE ... WHERE
-- 條件不成立時就是 no-op，不會報錯也不會插入第二列）。
INSERT INTO seller_ranking_facts (store_id, tier_rank, monthly_order_cap, tier_effective_at, updated_at)
VALUES ($1, $2, $3, $4, now())
ON CONFLICT (store_id) DO UPDATE SET
  tier_rank = $2,
  monthly_order_cap = $3,
  tier_effective_at = $4,
  updated_at = now()
WHERE seller_ranking_facts.tier_effective_at IS NULL OR $4 >= seller_ranking_facts.tier_effective_at;
