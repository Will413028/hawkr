INSERT INTO priceable_surface_form (entity_kind, entity_code, form_raw, form_kind)
SELECT 'COMMODITY', m.id::text, '糯米', 'ALIAS'
FROM commodity m
WHERE m.active
  AND m.source = 'MOA_RICE'
  AND m.commodity_code = 'pt_1japt'
ON CONFLICT DO NOTHING;
