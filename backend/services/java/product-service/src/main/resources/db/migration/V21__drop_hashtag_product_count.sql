-- hashtag 的公開商品數改為讀取時計算（只計 ACTIVE、未刪、店家可曝光的商品），不再儲存預先計算的計數。
-- 欄位上的 CHECK（V5）與 idx_hashtag_popular（V1）隨欄位一併移除。
ALTER TABLE hashtag DROP CONSTRAINT IF EXISTS chk_hashtag_product_count_nonneg;
DROP INDEX IF EXISTS idx_hashtag_popular;
ALTER TABLE hashtag DROP COLUMN product_count;
