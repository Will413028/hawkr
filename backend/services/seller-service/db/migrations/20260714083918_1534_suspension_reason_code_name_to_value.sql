-- issue #1534: seller_suspensions.reason_code 存值由 CodeMeta Name 切換為 Value 數字碼
-- （業務狀態碼.md SellerSuspensionReason：POLICY_VIOLATION→9601 / REPEATED_WARNING_IGNORED→9602 / OTHER→9603），
-- 對齊全 repo「CodeMeta enum DB 存 Value」慣例，消除 #1514 的 DB(Name)↔wire(Value) 轉換層。
-- Data-only（欄位型別 varchar(30) 不變）；idempotent：已是 Value 形態的 row 不匹配 WHERE，重跑零副作用。
UPDATE "seller_suspensions" SET "reason_code" = '9601' WHERE "reason_code" = 'POLICY_VIOLATION';
UPDATE "seller_suspensions" SET "reason_code" = '9602' WHERE "reason_code" = 'REPEATED_WARNING_IGNORED';
UPDATE "seller_suspensions" SET "reason_code" = '9603' WHERE "reason_code" = 'OTHER';
