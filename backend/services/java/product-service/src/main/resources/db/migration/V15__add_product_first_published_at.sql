-- 儲存不可變的「至少曾公開一次」事實；PUBLIC / TEMPORARILY_UNAVAILABLE 等狀態由讀取端推導。
ALTER TABLE product
    ADD COLUMN first_published_at TIMESTAMPTZ;

-- 只能證明目前 ACTIVE 的存量商品曾公開；既有 INACTIVE 不猜測歷史，避免洩漏草稿／審核未通過商品。
UPDATE product
SET first_published_at = COALESCE(updated_at, created_at, NOW())
WHERE status = '4004';

-- product 有 create、import、bulk suspension restore 等多個 SQL 寫入路徑；DB invariant 確保任何路徑
-- 第一次進入 ACTIVE 都只寫一次，後續更新不得覆寫首次公開時間。
CREATE OR REPLACE FUNCTION preserve_product_first_published_at()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' AND OLD.first_published_at IS NOT NULL THEN
        NEW.first_published_at = OLD.first_published_at;
    ELSIF NEW.status = '4004' THEN
        NEW.first_published_at = COALESCE(NEW.first_published_at, NOW());
    ELSE
        NEW.first_published_at = NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_product_first_published_at
    BEFORE INSERT OR UPDATE ON product
    FOR EACH ROW
    EXECUTE FUNCTION preserve_product_first_published_at();

COMMENT ON COLUMN product.first_published_at IS
    '首次公開時間（不可變）；NULL 表示無法證明曾公開，publication state 由此欄位、status、deleted_at、seller_deleted_at 推導';
