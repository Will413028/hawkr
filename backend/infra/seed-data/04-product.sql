
BEGIN;

DELETE FROM store_exposure WHERE store_id IN (1009, 1004);

DELETE FROM product_certification_review_event WHERE application_id IN (
  SELECT a.id FROM product_certification_application a
  WHERE a.product_id IN (
    SELECT id FROM product
    WHERE id BETWEEN 10001 AND 10099
       OR category BETWEEN 4 AND 99
       OR category_lv2 BETWEEN 4 AND 99
       OR category_lv1 BETWEEN 4 AND 99
  )
);
DELETE FROM product_certification_application WHERE product_id IN (
  SELECT id FROM product
  WHERE id BETWEEN 10001 AND 10099
     OR category BETWEEN 4 AND 99
     OR category_lv2 BETWEEN 4 AND 99
     OR category_lv1 BETWEEN 4 AND 99
);
DELETE FROM product_hashtag WHERE product_id IN (
  SELECT id FROM product
  WHERE id BETWEEN 10001 AND 10099
     OR category BETWEEN 4 AND 99
     OR category_lv2 BETWEEN 4 AND 99
     OR category_lv1 BETWEEN 4 AND 99
);
DELETE FROM product_sku WHERE product_id IN (
  SELECT id FROM product
  WHERE id BETWEEN 10001 AND 10099
     OR category BETWEEN 4 AND 99
     OR category_lv2 BETWEEN 4 AND 99
     OR category_lv1 BETWEEN 4 AND 99
);
DELETE FROM product
WHERE id BETWEEN 10001 AND 10099
   OR category BETWEEN 4 AND 99
   OR category_lv2 BETWEEN 4 AND 99
   OR category_lv1 BETWEEN 4 AND 99;
DELETE FROM category WHERE id BETWEEN 4 AND 99;

INSERT INTO category (id, name, parent_id, status, created_by) VALUES
  (4,  '葉菜類',   1,    '9004', 'system'),
  (5,  '根莖類',   1,    '9004', 'system'),
  (6,  '熱帶水果', 2,    '9004', 'system'),
  (7,  '溫帶水果', 2,    '9004', 'system'),
  (8,  '豬肉',     3,    '9004', 'system'),
  (9,  '雞肉',     3,    '9004', 'system'),
  (10, '海鮮',     NULL, '9004', 'system'),
  (11, '蝦蟹',     10,   '9004', 'system'),
  (12, '魚類',     10,   '9004', 'system'),
  (13, '茶葉',     NULL, '9004', 'system'),
  (14, '烏龍茶',   13,   '9004', 'system'),
  (15, '加工食品', NULL, '9004', 'system'),
  (16, '果乾蜜餞', 15,   '9004', 'system'),
  (33, '貝類',     10,   '9004', 'system');

INSERT INTO category (id, name, parent_id, status, created_by) VALUES
  (17, '高麗菜',       4,  '9004', 'system'),
  (18, '小白菜',       4,  '9004', 'system'),
  (19, '沙拉葉菜',     4,  '9004', 'system'),
  (20, '地瓜',         5,  '9004', 'system'),
  (21, '紅蘿蔔',       5,  '9004', 'system'),
  (22, '芒果',         6,  '9004', 'system'),
  (23, '水蜜桃',       7,  '9004', 'system'),
  (24, '黑毛豬五花',   8,  '9004', 'system'),
  (25, '土雞',         9,  '9004', 'system'),
  (26, '明蝦',         11, '9004', 'system'),
  (27, '海膽醬',       11, '9004', 'system'),
  (28, '透抽',         12, '9004', 'system'),
  (29, '黑鮪魚',       12, '9004', 'system'),
  (30, '高山烏龍',     14, '9004', 'system'),
  (31, '鮮蚵',         33, '9004', 'system'),
  (32, '芒果乾',       16, '9004', 'system');

SELECT setval(pg_get_serial_sequence('category', 'id'), GREATEST((SELECT MAX(id) FROM category), 33));


INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10001, '有機高麗菜', '4004', 17, 4, 1,
  '蔬菜', '葉菜類', '高麗菜', '台南新化',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"嚴選台南新化有機農場種植，不使用農藥與化學肥料，口感清甜爽脆。適合炒食、煮湯、做沙拉。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"1001","objectKey":"public/product/2676772453/20260806/7498aaf8-2e7a-4b38-9160-0fcb2e24dc9b.jpg"},{"assetId":"1002","objectKey":"public/product/2627714723/20260806/1851096f-95d5-43c6-b453-c8ac893ab562.jpg"}]',
  '[{"name":"3701","certNo":"1-008-131005"}]',
  120, 120, 9, 1009
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10002, '有機地瓜', '4004', 20, 5, 1,
  '蔬菜', '根莖類', '地瓜', '台南新化',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"台農57號黃金地瓜，有機栽種，鬆軟綿密，自然甜味。蒸烤皆宜，是健康的主食替代選擇。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"2001","objectKey":"public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg"}]',
  '[{"name":"3701","certNo":"1-008-131006"}]',
  180, 180, 9, 1009
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10003, '愛文芒果', '4004', 22, 6, 2,
  '水果', '熱帶水果', '芒果', '台南玉井',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"台南玉井產地直送愛文芒果，果肉細緻多汁，甜度可達15度以上。每年6-8月限量供應。","style":{"fontSize":16,"fontWeight":"normal"}},{"type":"text","id":"b2","content":"冷藏配送，收到後請儘速食用，冷藏可保存約5天。","style":{"fontSize":14,"fontWeight":"normal","color":"#666"}}]}',
  '[{"assetId":"3001","objectKey":"public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg"},{"assetId":"3002","objectKey":"public/product/2685914682/20260719/33a61c98-d1a5-46bb-851a-f9ec416d593c.jpg"},{"assetId":"3003","objectKey":"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png"}]',
  '[{"name":"3702","certNo":"TAP-2025-0032"}]',
  580, 1080, 9, 1009
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10004, '有機小白菜', '4004', 18, 4, 1,
  '蔬菜', '葉菜類', '小白菜', '台南善化',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"每日清晨現採，有機種植小白菜，葉片鮮嫩翠綠。清炒或煮湯都是家常好味道。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"4001","objectKey":"public/product/2676772453/20260806/7498aaf8-2e7a-4b38-9160-0fcb2e24dc9b.jpg"}]',
  '[{"name":"3701","certNo":"1-008-131007"}]',
  45, 45, 9, 1009
);


INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10005, '澎湖野生明蝦', '4004', 26, 11, 10,
  '海鮮', '蝦蟹', '明蝦', '澎湖',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"澎湖海域野生捕撈明蝦，肉質鮮甜Q彈，急速冷凍保鮮。適合清蒸、鹽烤、火鍋。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"5001","objectKey":"public/product/2627714723/20260806/1851096f-95d5-43c6-b453-c8ac893ab562.jpg"},{"assetId":"5002","objectKey":"public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg"}]',
  '[]',
  450, 450, 4, 1004
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10006, '鮮凍透抽', '4004', 28, 12, 10,
  '海鮮', '魚類', '透抽', '澎湖',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"澎湖現撈透抽，船上急速冷凍，鎖住海味鮮甜。口感脆嫩，川燙或快炒皆宜。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"6001","objectKey":"public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg"}]',
  '[]',
  380, 380, 4, 1004
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10007, '台灣黑毛豬五花肉', '4004', 24, 8, 3,
  '肉類', '豬肉', '黑毛豬五花', '屏東',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"屏東在地黑毛豬，飼養超過12個月，肉質鮮嫩油花分佈均勻。適合滷肉、烤肉、煎炒。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"7001","objectKey":"public/product/2685914682/20260719/33a61c98-d1a5-46bb-851a-f9ec416d593c.jpg"},{"assetId":"7002","objectKey":"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png"}]',
  '[{"name":"3703","certNo":"CAS-2025-1234"}]',
  250, 450, 4, 1004
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10008, '放山土雞', '4004', 25, 9, 3,
  '肉類', '雞肉', '土雞', '屏東',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"屏東放山飼養土雞，自然環境成長，肉質結實有嚼勁。燉湯尤其鮮甜，補身首選。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"8001","objectKey":"public/product/2676772453/20260806/7498aaf8-2e7a-4b38-9160-0fcb2e24dc9b.jpg"}]',
  '[]',
  650, 650, 4, 1004
);


INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10001, '1kg', 1.0, '3301', 120, 50,
  25, 20, 15,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10002, '3斤裝', 3.0, '3302', 180, 30,
  30, 20, 15,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10003, '5斤禮盒', 5.0, '3302', 580, 20,
  35, 25, 15,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10003, '10斤禮盒', 10.0, '3302', 1080, 10,
  40, 30, 20,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10004, '250g', 250, '3303', 45, 100,
  20, 15, 10,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3003","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10005, '600g', 600, '3303', 450, 25,
  30, 20, 10,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10006, '1kg', 1.0, '3301', 380, 40,
  35, 20, 10,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10007, '300g', 300, '3303', 250, 35,
  20, 15, 8,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"},{"type":"3002","temperature":"3202","freeThreshold":"0","feePayer":"3102"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10007, '600g', 600, '3303', 450, 20,
  25, 18, 10,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"},{"type":"3002","temperature":"3202","freeThreshold":"0","feePayer":"3102"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10008, '全雞約1.8kg', 1.8, '3301', 650, 15,
  30, 25, 20,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);


INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10009, '拉拉山水蜜桃', '4004', 23, 7, 2,
  '水果', '溫帶水果', '水蜜桃', '桃園復興',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"桃園拉拉山高海拔栽種水蜜桃，日夜溫差大，果肉細緻多汁。每年7-8月限量採收，手工套袋無農藥殘留。","style":{"fontSize":16,"fontWeight":"normal"}},{"type":"text","id":"b2","content":"冷藏宅配，收到請立即冷藏，建議3天內食用風味最佳。","style":{"fontSize":14,"fontWeight":"normal","color":"#666"}}]}',
  '[{"assetId":"9001","objectKey":"public/product/2627714723/20260806/1851096f-95d5-43c6-b453-c8ac893ab562.jpg"},{"assetId":"9002","objectKey":"public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg"},{"assetId":"9003","objectKey":"public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg"}]',
  '[{"name":"3702","certNo":"TAP-2026-0088"}]',
  780, 1380, 9, 1009
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10010, '有機紅蘿蔔', '4004', 21, 5, 1,
  '蔬菜', '根莖類', '紅蘿蔔', '雲林西螺',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"雲林西螺有機農場種植，土壤肥沃日照充足，紅蘿蔔甜度高、口感脆。富含β-胡蘿蔔素，生食熟食皆適合。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"10001","objectKey":"public/product/2685914682/20260719/33a61c98-d1a5-46bb-851a-f9ec416d593c.jpg"},{"assetId":"10002","objectKey":"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png"}]',
  '[{"name":"3701","certNo":"1-008-131010"}]',
  65, 120, 9, 1009
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10011, '阿里山高山烏龍茶', '4004', 30, 14, 13,
  '茶葉', '烏龍茶', '高山烏龍', '嘉義阿里山',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"海拔1200公尺以上茶園手採，春茶一心二葉精製。茶湯金黃透亮，花香馥郁回甘悠長。真空包裝保鮮。","style":{"fontSize":16,"fontWeight":"normal"}},{"type":"text","id":"b2","content":"每盒含4兩裝（150g），附精美茶罐。送禮自用兩相宜。","style":{"fontSize":14,"fontWeight":"normal","color":"#666"}}]}',
  '[{"assetId":"11001","objectKey":"public/product/2676772453/20260806/7498aaf8-2e7a-4b38-9160-0fcb2e24dc9b.jpg"},{"assetId":"11002","objectKey":"public/product/2627714723/20260806/1851096f-95d5-43c6-b453-c8ac893ab562.jpg"},{"assetId":"11003","objectKey":"public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg"}]',
  '[{"name":"3702","certNo":"ALI-TEA-2026-015"}]',
  450, 1600, 9, 1009
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10012, '有機綜合沙拉菜', '4004', 19, 4, 1,
  '蔬菜', '葉菜類', '沙拉葉菜', '台南善化',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"每日清晨現採，有機種植綜合沙拉菜，含羅美生菜、芝麻葉、紅橡葉。免洗即食，打開即可享用。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"12001","objectKey":"public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg"}]',
  '[{"name":"3701","certNo":"1-008-131012"}]',
  89, 89, 9, 1009
);


INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10013, '澎湖現剖海膽醬', '4004', 27, 11, 10,
  '海鮮', '蝦蟹', '海膽醬', '澎湖白沙',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"澎湖白沙海域手工採集馬糞海膽，現剖現製，無添加防腐劑。每瓶約100g純海膽肉，濃郁海味適合拌飯拌麵。","style":{"fontSize":16,"fontWeight":"normal"}},{"type":"text","id":"b2","content":"季節限量商品，每年5-9月供應。冷凍保存可達3個月。","style":{"fontSize":14,"fontWeight":"normal","color":"#666"}}]}',
  '[{"assetId":"13001","objectKey":"public/product/2685914682/20260719/33a61c98-d1a5-46bb-851a-f9ec416d593c.jpg"},{"assetId":"13002","objectKey":"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png"}]',
  '[]',
  680, 1280, 4, 1004
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10014, '東港黑鮪魚生魚片', '4004', 29, 12, 10,
  '海鮮', '魚類', '黑鮪魚', '屏東東港',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"屏東東港黑鮪魚季直送，嚴選大腹部位，油脂豐富入口即化。-60°C超低溫急凍，完整保留鮮度。","style":{"fontSize":16,"fontWeight":"normal"}},{"type":"text","id":"b2","content":"解凍後請於24小時內食用。每份約200g，附山葵及醬油包。","style":{"fontSize":14,"fontWeight":"normal","color":"#666"}}]}',
  '[{"assetId":"14001","objectKey":"public/product/2676772453/20260806/7498aaf8-2e7a-4b38-9160-0fcb2e24dc9b.jpg"},{"assetId":"14002","objectKey":"public/product/2627714723/20260806/1851096f-95d5-43c6-b453-c8ac893ab562.jpg"},{"assetId":"14003","objectKey":"public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg"}]',
  '[{"name":"3704","certNo":"HACCP-2026-DP-0047"}]',
  890, 890, 4, 1004
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10015, '嘉義布袋鮮蚵', '4004', 31, 33, 10,
  '海鮮', '貝類', '鮮蚵', '嘉義布袋',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"嘉義布袋外海養殖鮮蚵，每日產地現剖，肥美飽滿乳白色澤。蚵仔煎、蚵仔麵線、炸蚵嗲皆適合。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"15001","objectKey":"public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg"},{"assetId":"15002","objectKey":"public/product/2685914682/20260719/33a61c98-d1a5-46bb-851a-f9ec416d593c.jpg"}]',
  '[]',
  220, 420, 4, 1004
);

INSERT INTO product (
  id, name, status, category, category_lv2, category_lv1,
  cat_l1_name, cat_l2_name, cat_l3_name, origin,
  description, gallery, certifications,
  price_min, price_max, created_by, store_id
) VALUES (
  10016, '枋山愛文芒果乾', '4001', 32, 16, 15,
  '加工食品', '果乾蜜餞', '芒果乾', '屏東枋山',
  '{"version":"1.0","blocks":[{"type":"text","id":"b1","content":"嚴選屏東枋山在欉紅愛文芒果，低溫烘乾48小時，無添加糖及防腐劑，保留天然芒果香甜。","style":{"fontSize":16,"fontWeight":"normal"}}]}',
  '[{"assetId":"16001","objectKey":"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png"}]',
  '[]',
  180, 320, 4, 1004
);


INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10009, '6入禮盒', 2.4, '3301', 780, 15,
  35, 25, 15,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10009, '12入禮盒', 4.8, '3301', 1380, 8,
  40, 30, 18,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10010, '500g', 500, '3303', 65, 80,
  25, 15, 10,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10010, '1kg', 1.0, '3301', 120, 50,
  30, 18, 12,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10011, '4兩裝', 150, '3303', 450, 40,
  15, 10, 10,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"},{"type":"3003","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10011, '1斤禮盒', 1.0, '3302', 1600, 20,
  30, 20, 15,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10012, '150g', 150, '3303', 89, 60,
  20, 15, 8,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3101"},{"type":"3003","temperature":"3202","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10013, '1瓶', 100, '3303', 680, 30,
  10, 10, 8,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10013, '2瓶組合', 200, '3303', 1280, 15,
  18, 10, 8,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10014, '200g', 200, '3303', 890, 20,
  25, 15, 5,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10015, '300g', 300, '3303', 220, 45,
  20, 15, 8,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10015, '600g', 600, '3303', 420, 25,
  25, 18, 10,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10016, '100g', 100, '3303', 180, 100,
  18, 12, 5,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10016, '200g', 200, '3303', 320, 60,
  22, 15, 5,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

INSERT INTO product_certification_application
    (id, product_id, store_id, submitted_by_account_id, certification_type, certificate_no,
     status, source, requires_manual_review, gov_verify_state,
     submitted_at, reviewed_at, reviewed_by_account_id, created_at)
SELECT ((EXTRACT(EPOCH FROM NOW()) * 1000)::BIGINT << 22)
           + ROW_NUMBER() OVER (ORDER BY p.id, c->>'name'),
       p.id,
       p.store_id,
       NULL,
       (c->>'name')::INT,
       trim(c->>'certNo'),
       'APPROVED',
       'SEED',
       FALSE,
       'PENDING',
       NOW(), NOW(), NULL, NOW()
FROM product p, jsonb_array_elements(p.certifications) c
WHERE p.id BETWEEN 10001 AND 10099
  AND p.certifications IS NOT NULL
  AND jsonb_array_length(p.certifications) > 0;

WITH sku_expanded AS (
    SELECT s.id AS sku_id, s.product_id,
           elem->>'type' AS type,
           elem->>'temperature' AS temperature,
           NULLIF(elem->>'freeThreshold', '') AS free_threshold,
           elem->>'feePayer' AS fee_payer
    FROM product_sku s, jsonb_array_elements(s.shipping_method) elem
    WHERE s.product_id BETWEEN 10001 AND 10099
),
grouped AS (
    SELECT product_id, type, temperature,
           COALESCE(MIN(NULLIF(free_threshold, '0')::numeric), 0) AS threshold,
           (array_agg(fee_payer ORDER BY sku_id))[1] AS fee_payer
    FROM sku_expanded
    GROUP BY product_id, type, temperature
),
aggregated AS (
    SELECT product_id,
           jsonb_agg(
               jsonb_build_object(
                   'type', type, 'temperature', temperature,
                   'freeThreshold', threshold::text, 'feePayer', fee_payer
               )
               ORDER BY type, temperature
           ) AS methods
    FROM grouped
    GROUP BY product_id
)
UPDATE product p
SET stock_qty = COALESCE((SELECT SUM(s.stock_qty) FROM product_sku s WHERE s.product_id = p.id), 0),
    shipping_methods = COALESCE((SELECT methods FROM aggregated a WHERE a.product_id = p.id), '[]'::jsonb),
    list_price_min = (SELECT MIN(s.price) FROM product_sku s WHERE s.product_id = p.id),
    list_price_max = (SELECT MAX(s.price) FROM product_sku s WHERE s.product_id = p.id),
    sale_price_min = (SELECT MIN(s.sale_price) FROM product_sku s WHERE s.product_id = p.id),
    sale_price_max = (SELECT MAX(s.sale_price) FROM product_sku s WHERE s.product_id = p.id)
WHERE p.id BETWEEN 10001 AND 10099;

UPDATE product p SET searchable_text = NULLIF(trim(BOTH ' ' FROM concat_ws(' ',
    p.name,
    (SELECT string_agg(h.name, ' ' ORDER BY ph.position)
       FROM product_hashtag ph JOIN hashtag h ON h.id = ph.hashtag_id
       WHERE ph.product_id = p.id),
    (SELECT string_agg(s.sku_name, ' ' ORDER BY s.id)
       FROM product_sku s WHERE s.product_id = p.id),
    (SELECT c.name FROM category c WHERE c.id = p.category_lv1),
    (SELECT c.name FROM category c WHERE c.id = p.category_lv2),
    (SELECT c.name FROM category c WHERE c.id = p.category),
    (SELECT string_agg(b->>'content', ' ')
       FROM jsonb_array_elements(COALESCE(p.description->'blocks', '[]'::jsonb)) b
       WHERE b->>'type' = 'text')
)), '')
WHERE p.deleted_at IS NULL
  AND (p.searchable_text IS NULL OR p.searchable_text = '');

COMMIT;
