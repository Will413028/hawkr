-- 手寫 migration（不是 atlas migrate diff 產生的）。
--
-- 為什麼手寫：Atlas 社群版的 postgres driver **只追蹤被 serial／identity 欄位擁有的
-- sequence**，看不到 standalone `CREATE SEQUENCE`。實測 `atlas schema inspect` 對
-- schema.sql 裡的這兩支回 0 命中 ⇒ 放在 declarative schema 裡永遠不會被 diff 出來。
--
-- 這兩支是 §5.3.3 定版的：AMEGO 開立折讓成功後**不回傳任何識別碼**（只有 code / msg），
-- 且 AllowanceNumber 上限 16 字 ⇒ 單號必須我方自產、**送出前先落庫**，否則「送出成功但
-- 回應遺失」就永遠查不回來，唯一救回路徑是拿我方自己產的單號打 allowance_query。
-- TSID 十進位 19 碼加前綴就爆 16 字，所以用 SEQUENCE。
--
-- 多副本天然安全：nextval 不受交易回滾影響。跳號可接受（法規只要求不可重複、無連號
-- 義務，U-5）。全域共用跨 issuer：AMEGO 只要求帳戶內唯一，全域唯一是它的安全超集。
--
-- 缺了它們的症狀不是編譯錯：invoice_adjustments.allowance_number（chk_invadj_allowance_shape
-- 要求 ALLOWANCE 必填）與 invoice_documents.receipt_no（chk_invdoc_receipt 雙向等式）
-- 沒有任何產生器 ⇒ 折讓與收據永遠開不出來。
CREATE SEQUENCE IF NOT EXISTS "invoice_allowance_number_seq" AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;
CREATE SEQUENCE IF NOT EXISTS "invoice_receipt_number_seq"   AS bigint START WITH 1 INCREMENT BY 1 NO CYCLE;
