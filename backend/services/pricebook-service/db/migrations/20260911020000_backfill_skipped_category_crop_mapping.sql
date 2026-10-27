-- category_crop_mapping：補上 20260910141500 因 EXISTS 守衛跳過的 92 筆。
--
-- 背景：
--   20260910141500_promote_curated_category_crop_mapping.sql 帶 EXISTS 守衛，
--   group_code 不在該環境 crop_group 時整列跳過（fail closed）。DEV 的
--   crop_group 有 558 個大類、全部命中；PROD 只有 380 個，其中 30 個大類
--   不存在，於是 1152 筆只落地 1060 筆，92 筆被跳過。
--
--   那 30 個大類幾乎都是花卉與季節性／冷門農產（葉牡丹、鬱金香、陸蓮花、
--   OT 系列百合、枇杷、香櫞、巴西蘑菇、猴頭菇等）。根因不是編碼不一致——
--   已核對 PROD 連「同名不同 code」都沒有，是真的沒有這些大類：
--     DEV  price_observation  2012-01-01 ~ 2026-09-09   3,028,012 筆
--     PROD price_observation  2026-07-27 ~ 2026-09-09     114,938 筆
--   PROD 只有約 6 週資料，這些品項在該窗口內沒有交易紀錄。兩邊每日攝取
--   都正常（max(trade_date) 同為 2026-09-09），缺的是歷史。
--
-- 為什麼這支不帶 EXISTS 守衛：
--   守衛的用意是避免「猜」出一個不存在的大類。這裡不是猜——group_code 來自
--   同一份已驗證的人工策展，且已確認跨環境穩定（188 個 code 中 158 個在
--   PROD 命中、30 個完全不存在、**0 個是同名不同 code**）。加守衛只會讓這
--   92 筆永遠補不進去，因為 migration 套用過就不再執行，等 PROD 攝取到
--   這些品項時沒有任何機制會回頭補。
--
--   懸空映射本身是系統已容忍的狀態：category_crop_mapping.group_code 沒有
--   FK，且 PROD 現有 1339 筆中已有 111 筆指向不存在的 group（來自沒有守衛
--   的 20260830034310 bootstrap）。解析這類分類會回合法的 unavailable
--   business state，不是錯誤。
--
--   替代方案「寫 reconciliation job 自動補」已排除：後端服務責任邊界規範
--   §22 明文禁止在 application code 建立 reconciliation／backfill／drift
--   scanner。
--
-- 套用後行為：這 92 個分類在 PROD 仍會顯示無資料，因為沒有價格可彙整。
-- 本 migration 的價值是等這些品項開始交易時映射已就位，不需要有人記得回補。
--
-- ON CONFLICT DO NOTHING：不覆蓋任何環境已策展的列；在 DEV 為完全 no-op
-- （那 92 筆已存在）。可重複套用。

INSERT INTO category_crop_mapping
    (category_id, status, group_code, mapping_version, source_note, updated_by)
SELECT v.category_id, 'resolved', v.group_code, 'pricebook-category-v1', v.source_note, 0
FROM (VALUES
    ('630', 'moa-group:2bab08af4e1f798d', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('649', 'moa-group:4ab1a8ec34889567', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('650', 'moa-group:4ab1a8ec34889567', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('652', 'moa-group:7f9782ee037ab987', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('719', 'moa-group:9497e1cedf9632c5', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('722', 'moa-group:10c8e397aad9fe17', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('754', 'moa-group:1b41bf00e8e7f4c9', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('770', 'moa-group:a00f326e33a5aabc', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('805', 'moa-group:a5df183a763ec6ff', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1157', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1158', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1159', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1160', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1161', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1162', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1163', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1164', 'moa-group:43004a352e50822a', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1268', 'moa-group:209620ef6eedd7a6', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1269', 'moa-group:209620ef6eedd7a6', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1270', 'moa-group:209620ef6eedd7a6', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1340', 'moa-group:82281e5d58a252b9', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1341', 'moa-group:82281e5d58a252b9', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1412', 'moa-group:64cb5a30a45b97f6', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1413', 'moa-group:64cb5a30a45b97f6', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1414', 'moa-group:64cb5a30a45b97f6', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1415', 'moa-group:64cb5a30a45b97f6', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1416', 'moa-group:64cb5a30a45b97f6', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1417', 'moa-group:64cb5a30a45b97f6', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1418', 'moa-group:64cb5a30a45b97f6', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1427', 'moa-group:561a5983937df243', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1428', 'moa-group:561a5983937df243', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1429', 'moa-group:561a5983937df243', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1430', 'moa-group:561a5983937df243', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1445', 'moa-group:8eef7c8cbfe9814d', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1446', 'moa-group:8eef7c8cbfe9814d', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1447', 'moa-group:8eef7c8cbfe9814d', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1469', 'moa-group:022151ae431a7892', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1486', 'moa-group:58af396edb3f9471', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1487', 'moa-group:404c6248517ddb1f', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1488', 'moa-group:404c6248517ddb1f', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1511', 'moa-group:76296617add00a99', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1512', 'moa-group:76296617add00a99', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1513', 'moa-group:76296617add00a99', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1522', 'moa-group:b2b7adf5cc94a41e', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1523', 'moa-group:b2b7adf5cc94a41e', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1524', 'moa-group:b2b7adf5cc94a41e', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1538', 'moa-group:f2cd62f2617ab700', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1539', 'moa-group:f2cd62f2617ab700', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1540', 'moa-group:5541c67071347097', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1541', 'moa-group:5541c67071347097', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1542', 'moa-group:5541c67071347097', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1543', 'moa-group:5541c67071347097', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1546', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1547', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1548', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1549', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1550', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1551', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1552', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1553', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1554', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1555', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1556', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1557', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1558', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1559', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1560', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1561', 'moa-group:6bfa4ea86e7ec394', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1570', 'moa-group:894ad6167f699dda', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1571', 'moa-group:894ad6167f699dda', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1572', 'moa-group:894ad6167f699dda', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1573', 'moa-group:894ad6167f699dda', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1574', 'moa-group:894ad6167f699dda', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1579', 'moa-group:677333486864bc7c', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1580', 'moa-group:677333486864bc7c', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1724', 'moa-group:7172381936f2f1e3', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1725', 'moa-group:7172381936f2f1e3', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1726', 'moa-group:7172381936f2f1e3', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1727', 'moa-group:7172381936f2f1e3', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1792', 'moa-group:64b99cb776ad0c45', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1793', 'moa-group:64b99cb776ad0c45', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1794', 'moa-group:64b99cb776ad0c45', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1795', 'moa-group:64b99cb776ad0c45', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1874', 'moa-group:5eb3deed059f3a41', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1875', 'moa-group:5eb3deed059f3a41', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1876', 'moa-group:5eb3deed059f3a41', 'L2 分類名稱補上 OT 百合雜交群前綴後精確且唯一命中 crop_group.name（如「多娜托」→「OT多娜托」），非語意猜測'),
    ('1877', 'moa-group:d66078a49e021f7f', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1878', 'moa-group:d66078a49e021f7f', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1879', 'moa-group:d66078a49e021f7f', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1880', 'moa-group:d66078a49e021f7f', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('1888', 'moa-group:fc5aa2c1196c6276', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）'),
    ('2251', 'moa-group:80f3875939c4f13c', 'L2 分類名稱精確且唯一命中 pricebook crop_group.name，沿用 2026-08-30 categoryId bootstrap 的 tier-precedence 邏輯（往上一層比對，非猜測）')
) AS v(category_id, group_code, source_note)
ON CONFLICT (category_id) DO NOTHING;
