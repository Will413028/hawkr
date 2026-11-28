-- 禁運品／品類守門的 SKU 標記欄（66 號詳細設計書 §5.1.9 ②）。
--
-- 🔴 **為什麼「只做碼表與檢查、不做這個欄位」是最糟的組合**（66 §5.1.9 逐字）：
--    結帳的第 6 道檢查（禁運品）會照跑，但每一筆讀到的都是空陣列 ⇒ **恆為通過**，
--    而且看起來完全正常——沒有錯誤、沒有告警、測試也會綠。本檔把 ② 補上，
--    讓 ①（碼表）與 ③（賣家上架 UI ＋ 結帳檢查）日後接上來時不是空轉。
--
-- ⚠️ 本波**零程式碼消費者**：沒有任何 mapper／service 會讀寫這一欄
--    （寫入者是 66 §5.1.9 ③ 的賣家上架 UI，讀取者是 §7.4 道 6 的結帳檢查，兩者都在後續波次）。
--    ⇒ 這份 DDL 打錯不會讓任何測試轉紅，唯一的守門是
--    ProductSkuRestrictedCategorySchemaContractTest 與 review。
--
-- ---------------------------------------------------------------------
-- 🔴 值域（RestrictedGoodsCategory）尚未定案 —— 本檔如何做到「沒有碼表 ⇒ 寫不進去」
-- ---------------------------------------------------------------------
-- 66 附錄 C.2 ⑤ 把 RestrictedGoodsCategory 的值域掛在 UP-4（法遵 ＋ Uber 台灣條款）之下，
-- 至今未定案。值域未定時**不得自己發明碼值**（CodeMeta 是 SSOT，碼值由 .md 定），
-- 於是本波只建欄位、不建碼表。
--
-- 但「沒有碼表」的預設後果是**什麼都寫得進去**（JSONB 對內容零約束），那正好是相反的失敗方向：
-- 在碼表定案前，任何服務都能往這一欄塞自創字串，而且不會有任何東西擋下來——
-- 等 UP-4 定案時，DB 裡已經躺著一批沒有人認得的值。
--
-- ⇒ 本檔用 CHECK 把它反過來鎖成 fail-closed：**值域目前是空集合，所以合法內容只有空陣列**。
--    這不是「暫時懶得驗證」，而是把「空值域」這件事逐字翻譯成約束——
--    任何寫入嘗試都會當場 23514（check_violation），訊息直接指向本檔。
--
-- 🔴 **UP-4 定案時要做的事（兩件，同一個 V{N+1} 內，缺一即回到「什麼都寫得進去」）**：
--    1. `docs/代碼管理（CodeMeta）/業務類型碼.md` 新增 RestrictedGoodsCategory 碼表
--       ＋ `generate.sh --all` ＋ 四份前端拷貝。
--    2. 在新的 migration 內**置換**（不是刪除）本檔的 CHECK：
--       改成逐值列舉，例如
--         CHECK (jsonb_typeof(restricted_category_codes) = 'array'
--                AND NOT EXISTS (SELECT 1 FROM jsonb_array_elements_text(restricted_category_codes) e
--                                WHERE e.value NOT IN ('<碼值1>', '<碼值2>', ...)))
--       ⚠️ **只 DROP 不 ADD ＝ 把守門拆掉**。寫入路徑同時要走
--       `CodeMetaValidator.validate(RestrictedGoodsCategory.class, code, "restrictedCategoryCodes")`
--       （應用層擋得早、訊息好讀；DB CHECK 是繞過應用層時的最後一道，兩道都要）。
--
-- 回填判定（《DB Migration 相容性規範（expand-contract）》§三 逐條走過）：
-- 該規範針對「新欄位可由既有欄位推導」——那時 ADD COLUMN ... DEFAULT 會把每一列填成同一個錯值。
-- 本欄不由任何既有欄位推導：既有 SKU 從未被標記過禁運品類，空陣列 `[]` **即為正確初值**
-- （66 §5.4 逐字「不需要回填」）。因此不追加 UPDATE。
ALTER TABLE product_sku
    ADD COLUMN restricted_category_codes JSONB NOT NULL DEFAULT '[]'::jsonb;

-- 型別不變式：它是陣列，不是物件也不是純量。
-- 值域不變式：RestrictedGoodsCategory 目前是空集合 ⇒ 唯一合法內容是空陣列（見上方說明）。
ALTER TABLE product_sku
    ADD CONSTRAINT chk_product_sku_restricted_category_codes CHECK (
        jsonb_typeof(restricted_category_codes) = 'array'
        AND jsonb_array_length(restricted_category_codes) = 0
    );

COMMENT ON COLUMN product_sku.restricted_category_codes IS
    '禁運品／品類標記，元素為 RestrictedGoodsCategory 的 Value（66 §5.1.9 ②）。'
    '空陣列 = 未標記任何受限品類。'
    '🔴 值域待 UP-4 定案，在那之前 chk_product_sku_restricted_category_codes 只允許空陣列 '
    '——「沒有碼表」必須等於「寫不進去」，不是「什麼都寫得進去」。'
    'UP-4 定案時：同一個 V{N+1} 內建碼表並把該 CHECK 置換成逐值列舉（只 DROP 不 ADD = 拆掉守門），'
    '寫入路徑同時走 CodeMetaValidator。';
