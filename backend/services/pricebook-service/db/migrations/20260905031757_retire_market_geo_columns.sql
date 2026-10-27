-- market 的三個地理欄位退場：建表時就在，但從來沒有任何寫入路徑
-- （唯一的 INSERT 是 UpsertMarket，只寫 id／market_code／name），也沒有任何
-- query 讀它們。dev 上 21 筆市場三欄全 NULL——不是資料沒補齊，是結構上沒有東西
-- 會去填。市場→縣市的解析走 geography 套件的 facility registry（嵌入式資料），
-- 不是這張表。詳見 db/schema/schema.sql 該表的註解。
--
-- 純 DROP、無資料要保留：三欄在全部環境都是 100% NULL。
-- Modify "market" table
ALTER TABLE "market" DROP COLUMN "county_code", DROP COLUMN "latitude", DROP COLUMN "longitude";
