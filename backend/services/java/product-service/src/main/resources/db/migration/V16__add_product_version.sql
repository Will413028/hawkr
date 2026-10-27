-- 樂觀鎖版本欄（56 §5.4）。version 不由任何既有欄位推導，樂觀鎖從 0 起算，
-- DEFAULT 0 對既有列即正確初值，因此刻意不追加回填 UPDATE。
ALTER TABLE product
    ADD COLUMN version BIGINT NOT NULL DEFAULT 0;

-- 恆真的不變式交給 DB 守（expand-contract 規範 §三）。
ALTER TABLE product
    ADD CONSTRAINT chk_product_version_nonneg CHECK (version >= 0);

COMMENT ON COLUMN product.version IS
    '樂觀鎖版本（設計書 56 §5.4）。每次成功寫入 +1；代行寫入以 expectedVersion 比對，不符即 409。';
