-- ============================================================================
-- Hawkr 測試資料：商品 (hawkr_product)
-- ============================================================================
-- 測試商品 ID 範圍：10001~10099
-- 測試分類 ID 範圍：4~99（1~3 由 V1 migration 建立）
--
-- 分類三層結構（product.category/category_lv1/category_lv2 為後端強制要求，
-- 見 ProductService.java §建立商品：category 必須是「第三層 leaf」，
-- category_lv2 = leaf.parent_id，category_lv1 = category_lv2.parent_id，
-- 缺任一層祖先鏈直接建立失敗，無 fallback）：
--   lv1（頂層，parent_id NULL）：1=蔬菜, 2=水果, 3=肉類（V1 建立）+ 本檔新增 10=海鮮, 13=茶葉, 15=加工食品
--   lv2（id 4~16, 33，本檔新增）：lv1 的子分類
--   lv3/leaf（id 17~32，本檔新增）：lv2 的子分類，商品實際掛載的 category 必須是這層
--
-- JSONB 欄位注意事項：
--   1. 所有 key 一律 camelCase
--   2. shipping_method.freeThreshold 為字串 "1500"（非數字 1500）
--   3. shipping_method.type/temperature/feePayer 使用 Enum value（數字短碼）
--   4. gallery 的 objectKey 為物件路徑（非完整 URL）
--   5. certifications 的 name 是 **CertificationType 的 value（3701~3706）**，不是中文標籤。
--      #2755 起 product.certifications 由「賣家宣稱的公開欄位」降級為
--      **只由 APPROVED 申請重算的公開投影**（19 號設計書），權威來源是
--      product_certification_application。本檔因此必須兩邊一起寫（見下方「農產品認證申請」節），
--      且 name 一律用碼值 —— 中文值會被 (c->>'name')::INT 直接炸掉（這是刻意的 fail-loud）
--      ⚠️ gallery 的 key 刻意用 dev 媒體儲存上**真實存在**的物件。先前這裡是
--      public/product/1000x/main.jpg 這種推導出來的路徑，而那些物件從來沒有被上傳過，
--      於是本機每一張商品圖都是破的——破圖又很容易被當成「本機沒有媒體服務」而不再追，
--      使本機所有涉及圖片的 UI 驗證實際上是盲的。若這些物件日後被清理，換一組
--      仍存在的 key 即可（GET https://test-media.example.com/i/original/<key> 回 200 就是還在）。
--
-- 賣家：account_id 9、store_id 1009（翠綠有機農場，store_code 9、HAWK9）；account_id 4、store_id 1004（澎湖鮮味水產，store_code 4、HAWK4）。
-- 店家本身在單體的 seller schema，由 backend 的 make dev-seed 寫入（cmd/hawkr/dev_seed.go），本檔的 store_id 要與它一致；
-- created_by 是帳號（account_id），store_id 是店家，兩者刻意不同值。
-- ============================================================================

BEGIN;

-- 清除舊測試資料
-- store_exposure（店家可曝光的讀取模型）只清種子店家 1009、1004 的列：單體的重建資料庫（Java 的資料庫還在）會讓 seller 的公告版本從頭算，
-- Java 依版本丟棄小於或等於現有的事件，不清就讓種子店家停在舊的狀態。seller 的公告版本只增不減，所以清掉之後的任何在途舊事件
-- 要嘛被最後的全量重新公告（seed-test-data.sh 的最後一步，版本更大）覆蓋，要嘛在它之後被丟棄。其他店家的列不碰。
DELETE FROM store_exposure WHERE store_id IN (1009, 1004);

-- 範圍刻意擴大到「任何引用測試分類 4~99 的商品」（非僅 10001~10099 ID 區間），
-- 避免其他流程（如 site-onboard 手動測試）留下的商品在下方 DELETE category 時
-- 因 FK 而 abort（#1509 已知失敗模式之一）。刪除順序需先 product_hashtag / product_sku
-- 再 product，最後才是 category（FK 依賴方向）。
-- 認證申請與審核事件先刪（review_event.application_id 指向 application，無 FK 約束但語意上有依賴）。
-- 範圍與下方 product 的刪除條件一致，否則重跑 seed 會累積上一輪的申請列，
-- 而下一次 rebuildProjection 會把它們一起放回公開投影。
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

-- =====================
-- 子分類（lv2，第二層）
-- =====================
-- 父分類由 V1 migration 建立：1=蔬菜, 2=水果, 3=肉類
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

-- =====================
-- 葉節點分類（lv3，第三層，商品實際掛載處）
-- =====================
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

-- 重設 category 序列，避免自動生成 ID 衝突
SELECT setval(pg_get_serial_sequence('category', 'id'), GREATEST((SELECT MAX(id) FROM category), 33));

-- =====================
-- 商品：賣家 9（翠綠有機農場）
-- =====================

-- 1. 有機高麗菜
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

-- 2. 有機地瓜
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

-- 3. 愛文芒果
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

-- 4. 有機小白菜
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

-- =====================
-- 商品：賣家 4（澎湖鮮味水產）
-- =====================

-- 5. 澎湖野生明蝦
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

-- 6. 鮮凍透抽
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

-- 7. 台灣黑毛豬五花肉
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

-- 8. 放山土雞
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

-- =====================
-- 商品 SKU
-- =====================
-- product_sku 使用 BIGSERIAL，不指定 id

-- 有機高麗菜 - 1kg
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10001, '1kg', 1.0, '3301', 120, 50,
  25, 20, 15,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"}]'
);

-- 有機地瓜 - 3斤裝
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10002, '3斤裝', 3.0, '3302', 180, 30,
  30, 20, 15,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"}]'
);

-- 愛文芒果 - 5斤禮盒
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10003, '5斤禮盒', 5.0, '3302', 580, 20,
  35, 25, 15,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

-- 愛文芒果 - 10斤禮盒
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10003, '10斤禮盒', 10.0, '3302', 1080, 10,
  40, 30, 20,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

-- 有機小白菜 - 250g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10004, '250g', 250, '3303', 45, 100,
  20, 15, 10,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3003","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

-- 澎湖野生明蝦 - 600g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10005, '600g', 600, '3303', 450, 25,
  30, 20, 10,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3101"}]'
);

-- 鮮凍透抽 - 1kg
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10006, '1kg', 1.0, '3301', 380, 40,
  35, 20, 10,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3101"}]'
);

-- 台灣黑毛豬五花肉 - 300g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10007, '300g', 300, '3303', 250, 35,
  20, 15, 8,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"},{"type":"3002","temperature":"3202","freeThreshold":"0","feePayer":"3102"}]'
);

-- 台灣黑毛豬五花肉 - 600g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10007, '600g', 600, '3303', 450, 20,
  25, 18, 10,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"},{"type":"3002","temperature":"3202","freeThreshold":"0","feePayer":"3102"}]'
);

-- 放山土雞 - 全雞約1.8kg
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10008, '全雞約1.8kg', 1.8, '3301', 650, 15,
  30, 25, 20,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);

-- =====================
-- 新增商品：賣家 9（翠綠有機農場）
-- =====================

-- 9. 日本品種水蜜桃（季節限定，冷藏配送）
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
  '[{"name":"3702","certNo":"TAP-2026-0088"}]',   -- 3702 TAP 產銷履歷（原為中文標籤「產銷履歷」，非合法碼值）
  780, 1380, 9, 1009
);

-- 10. 有機紅蘿蔔（常溫配送，支援超取）
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
  '[{"name":"3701","certNo":"1-008-131010"}]',   -- 3701 ORGANIC 有機農產品（原為中文標籤「有機農產品標章」）
  65, 120, 9, 1009
);

-- 11. 阿里山高山茶（常溫配送，支援超取＋即時快遞）
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
  '[{"name":"3702","certNo":"ALI-TEA-2026-015"}]',   -- 3702 TAP（原為「產地證明」——六個 CertificationType 裡根本沒有這個東西，見檔頭說明）
  450, 1600, 9, 1009
);

-- 12. 有機綜合沙拉菜（冷藏配送，即時快遞）
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
  '[{"name":"3701","certNo":"1-008-131012"}]',   -- 3701 ORGANIC 有機農產品（原為中文標籤「有機農產品標章」）
  89, 89, 9, 1009
);

-- =====================
-- 新增商品：賣家 4（澎湖鮮味水產）
-- =====================

-- 13. 澎湖海膽醬（冷凍宅配，高單價精品）
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

-- 14. 東港黑鮪魚生魚片（冷凍宅配）
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
  '[{"name":"3704","certNo":"HACCP-2026-DP-0047"}]',   -- 3704 HACCP 食品安全（原為中文標籤「HACCP食品安全」）
  890, 890, 4, 1004
);

-- 15. 嘉義布袋鮮蚵（冷藏宅配，賣家免運）
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

-- 16. 屏東枋山愛文芒果乾（常溫，超取+宅配，草稿中未上架）
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

-- =====================
-- 新增商品 SKU
-- =====================

-- 拉拉山水蜜桃 - 6入禮盒
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10009, '6入禮盒', 2.4, '3301', 780, 15,
  35, 25, 15,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

-- 拉拉山水蜜桃 - 12入禮盒
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10009, '12入禮盒', 4.8, '3301', 1380, 8,
  40, 30, 18,
  '[{"type":"3001","temperature":"3202","freeThreshold":"2000","feePayer":"3101"}]'
);

-- 有機紅蘿蔔 - 500g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10010, '500g', 500, '3303', 65, 80,
  25, 15, 10,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

-- 有機紅蘿蔔 - 1kg
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10010, '1kg', 1.0, '3301', 120, 50,
  30, 18, 12,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

-- 阿里山高山烏龍茶 - 4兩裝（150g）
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10011, '4兩裝', 150, '3303', 450, 40,
  15, 10, 10,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"},{"type":"3003","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

-- 阿里山高山烏龍茶 - 1斤禮盒（600g）
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10011, '1斤禮盒', 1.0, '3302', 1600, 20,
  30, 20, 15,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

-- 有機綜合沙拉菜 - 150g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10012, '150g', 150, '3303', 89, 60,
  20, 15, 8,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3101"},{"type":"3003","temperature":"3202","freeThreshold":"0","feePayer":"3101"}]'
);

-- 澎湖現剖海膽醬 - 1瓶（100g）
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10013, '1瓶', 100, '3303', 680, 30,
  10, 10, 8,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);

-- 澎湖現剖海膽醬 - 2瓶組合
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10013, '2瓶組合', 200, '3303', 1280, 15,
  18, 10, 8,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);

-- 東港黑鮪魚生魚片 - 200g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10014, '200g', 200, '3303', 890, 20,
  25, 15, 5,
  '[{"type":"3001","temperature":"3203","freeThreshold":"2000","feePayer":"3102"}]'
);

-- 嘉義布袋鮮蚵 - 300g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10015, '300g', 300, '3303', 220, 45,
  20, 15, 8,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"}]'
);

-- 嘉義布袋鮮蚵 - 600g
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10015, '600g', 600, '3303', 420, 25,
  25, 18, 10,
  '[{"type":"3001","temperature":"3202","freeThreshold":"1500","feePayer":"3102"}]'
);

-- 枋山愛文芒果乾 - 100g（草稿中）
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10016, '100g', 100, '3303', 180, 100,
  18, 12, 5,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

-- 枋山愛文芒果乾 - 200g（草稿中）
INSERT INTO product_sku (
  product_id, sku_name, weight, unit, price, stock_qty,
  length_cm, width_cm, height_cm, shipping_method
) VALUES (
  10016, '200g', 200, '3303', 320, 60,
  22, 15, 5,
  '[{"type":"3001","temperature":"3201","freeThreshold":"1500","feePayer":"3101"},{"type":"3002","temperature":"3201","freeThreshold":"0","feePayer":"3101"}]'
);

-- =====================
-- 農產品認證申請（19 號設計書 §4.1 / §15.4）
-- =====================
-- product.certifications 從 #2755 起是「只由 APPROVED 申請重算的公開投影」，不是權威來源。
-- 只灌投影不灌申請表 ⇒ 這些商品的認證在權威表裡不存在 ⇒ 任何一次核准／撤銷／撤回觸發的
-- rebuildProjection 都會把它們整組抹掉（那支函式是拿 APPROVED 集合**覆寫**投影，不是 merge），
-- 而症狀是「seed 完看起來正常，直到某次審核動作才整組消失」。
--
-- 本節的列**由 product.certifications 反推**而不是另外手寫一份：手寫兩份必然漂移，
-- 而這裡要保證的正是「兩邊一致」這件事本身。
--
-- 欄位判斷：
--   status = 'APPROVED'            投影只收 APPROVED；填 SUBMITTED 會讓投影與申請表對不上。
--   source = 'SEED'                ProductCertificationSource.SEED：「測試資料，不代表任何真實賣家宣稱」。
--   submitted_by_account_id = NULL seed 沒有真實送出者。🔴 絕不可填 store_id 或 created_by 頂替：
--                                  store_id 是營運鍵、account_id 是登入 principal（CLAUDE.md §6），
--                                  而 seed 裡兩者恰好相等（首任 grandfather）⇒ 借用了照樣全綠，
--                                  這正是這個 class 測不出來的原因。V18 對 LEGACY_MIGRATION 同此處理。
--   reviewed_by_account_id = NULL  同上，沒有真實裁決者。
--   gov_verify_state = 'PENDING'   seed 沒有跑過任何政府查驗，不得偽造成 VERIFIED。
--                                  APPROVED + PENDING 是合法組合：3704/3705/3706 本來就無線上
--                                  查驗介面（UNSUPPORTED），永遠人工審。
-- 刻意不寫 product_certification_review_event：seed 不偽造審核事件，與 V18 對 LEGACY_MIGRATION
-- 列的處理一致（那支 migration 同樣只建申請列、不建事件）。
--
-- 刻意**不加** ON CONFLICT DO NOTHING：V18 需要它是因為存量 JSONB 不受控（同商品同 type 可能兩筆）；
-- seed 是受控資料，撞上 uk_pca_active 代表這個檔案自己寫錯了，要當場炸掉而不是靜默少一列。
-- 同理，中文標籤會讓 (c->>'name')::INT 直接報 invalid input syntax —— 這兩個都是刻意的 fail-loud。
--
-- id 的產法沿用 V18__add_product_certification_application.sql：
--   ((unix_epoch_ms) << 22) + row_number()
-- 對齊 Java 側 TsidUtil（common-core）的 42-bit timestamp + 10-bit node + 12-bit sequence 佈局
-- ⇒ 低 22 位是 node ＋ sequence，毫秒左移 22 位正好落在 timestamp 段。
-- 非碰撞論證：應用層 TSID 的 timestamp 基準是 TSID epoch 2020-01-01
-- （值 = (unix_ms − 1577836800000) << 22），本式用的是 raw unix epoch，兩者差約 6.6e18
-- ⇒ 本式產出的 id 永遠落在應用層值域之上約 50 年。不可改用 bigserial 或小整數，那才會撞。
-- =====================
INSERT INTO product_certification_application
    (id, product_id, store_id, submitted_by_account_id, certification_type, certificate_no,
     status, source, requires_manual_review, gov_verify_state,
     submitted_at, reviewed_at, reviewed_by_account_id, created_at)
SELECT ((EXTRACT(EPOCH FROM NOW()) * 1000)::BIGINT << 22)
           + ROW_NUMBER() OVER (ORDER BY p.id, c->>'name'),
       p.id,
       p.store_id,
       NULL,                                  -- 🔴 見上：不得以 store_id / created_by 頂替 account_id
       (c->>'name')::INT,                     -- 中文標籤在這裡會直接炸（刻意）
       trim(c->>'certNo'),
       'APPROVED',
       'SEED',
       FALSE,
       'PENDING',                             -- seed 未查驗，不偽造 VERIFIED
       NOW(), NOW(), NULL, NOW()
FROM product p, jsonb_array_elements(p.certifications) c
WHERE p.id BETWEEN 10001 AND 10099
  AND p.certifications IS NOT NULL
  AND jsonb_array_length(p.certifications) > 0;

-- =====================
-- 重算 product.stock_qty / product.shipping_methods / 價格區間（02 §5.13a 反正規化聚合欄位）
-- seed 直接 INSERT product_sku 繞過應用層的 ProductAggregateRules / updatePriceRange，
-- product 表這些欄位吃 DEFAULT 0 / '[]' / NULL，導致上架商品在列表 API 顯示 stock=0、
-- smoke-checkout 選不到可購買商品。
-- 演算法對齊 ProductAggregateRules.aggregateStock / mergeShippingMethods
-- 與 ProductService.updatePriceRange：
--   stock_qty = 該商品所有 SKU 的 stock_qty 加總
--   shipping_methods = 依 (type, temperature) 去重聯集，free_threshold 取「非 0/非空」中最小值，
--                       feePayer 取 sku.id 最小者
--   list_price_min/max = SKU.price（原價）的 min/max
--   sale_price_min/max = SKU.sale_price（特價）的 min/max；全部 SKU 皆無特價則為 NULL
--   （price_min/price_max 本檔商品 INSERT 已直接帶入正確值，此處不重算）
-- =====================
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

-- =====================
-- 重算 searchable_text（02 §3.9 / §7.5）
-- seed 直接 INSERT 未帶 searchable_text，補算以確保新環境商品可被關鍵字搜尋。
-- 拼接順序與 SearchableTextBuilder 一致：name → hashtag canonical → sku_name → cat_l1/l2/l3_name → pure_description。
-- =====================
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
