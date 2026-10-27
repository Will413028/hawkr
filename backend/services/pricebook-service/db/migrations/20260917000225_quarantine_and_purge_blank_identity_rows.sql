INSERT INTO ingest_rejected_identity (
    source, crop_code, market_code, reason,
    occurrence_count, first_seen_at, last_seen_at, first_trade_date, last_trade_date
)
SELECT
    'moa_agri_products_trans_type',
    c.crop_code,
    m.market_code,
    CASE WHEN length(btrim(c.crop_code)) = 0 THEN 'missing_crop_code'
         ELSE 'missing_crop_name' END,
    count(*),
    now(),
    now(),
    (extract(year FROM min(po.trade_date))::int - 1911)::text
        || to_char(min(po.trade_date), '.MM.DD'),
    (extract(year FROM max(po.trade_date))::int - 1911)::text
        || to_char(max(po.trade_date), '.MM.DD')
FROM crop c
JOIN price_observation po ON po.crop_id = c.id
JOIN market m ON m.id = po.market_id
WHERE length(btrim(c.name)) = 0 OR length(btrim(c.crop_code)) = 0
GROUP BY c.crop_code, m.market_code,
         CASE WHEN length(btrim(c.crop_code)) = 0 THEN 'missing_crop_code'
              ELSE 'missing_crop_name' END
ON CONFLICT (source, crop_code, market_code, reason) DO NOTHING;

DELETE FROM price_observation po
USING crop c
WHERE po.crop_id = c.id
  AND (length(btrim(c.name)) = 0 OR length(btrim(c.crop_code)) = 0);

DELETE FROM market_watch_target w
USING crop c
WHERE w.crop_id = c.id
  AND (length(btrim(c.name)) = 0 OR length(btrim(c.crop_code)) = 0);

DELETE FROM category_crop_mapping_crop ccmc
USING crop c
WHERE ccmc.crop_id = c.id
  AND (length(btrim(c.name)) = 0 OR length(btrim(c.crop_code)) = 0);

DELETE FROM crop c
WHERE length(btrim(c.name)) = 0 OR length(btrim(c.crop_code)) = 0;

DELETE FROM crop_group_forecast_result r
USING crop_group g
WHERE r.group_code = g.group_code
  AND (length(btrim(g.name)) = 0 OR length(btrim(g.group_code)) = 0)
  AND NOT EXISTS (SELECT 1 FROM crop c WHERE c.group_id = g.id);

DELETE FROM crop_group g
WHERE (length(btrim(g.name)) = 0 OR length(btrim(g.group_code)) = 0)
  AND NOT EXISTS (SELECT 1 FROM crop c WHERE c.group_id = g.id);

DELETE FROM price_observation po
USING market m
WHERE po.market_id = m.id
  AND (length(btrim(m.name)) = 0 OR length(btrim(m.market_code)) = 0);

DELETE FROM market_watch_target w
USING market m
WHERE w.market_id = m.id
  AND (length(btrim(m.name)) = 0 OR length(btrim(m.market_code)) = 0);

DELETE FROM market_rest_day d
USING market m
WHERE d.market_id = m.id
  AND (length(btrim(m.name)) = 0 OR length(btrim(m.market_code)) = 0);

DELETE FROM ingest_partition p
USING market m
WHERE p.market_id = m.id
  AND (length(btrim(m.name)) = 0 OR length(btrim(m.market_code)) = 0);

DELETE FROM market m
WHERE length(btrim(m.name)) = 0 OR length(btrim(m.market_code)) = 0;
