-- 政策分類（platform/policy，step10e-reports-brief R10）取代違規類別：通報的類別只接受檢舉人可選的 12 個值（沒有 PLATFORM_ABUSE），認定的類別只接受
-- 平台判定與認定可用的 12 個值（沒有 OTHER）；UNAUTHORIZED_SALE 改名 UNLICENSED_SALE，其餘舊值保留。
-- 轉換前先核對：認定的類別是 OTHER 的列沒有誠實的對應（OTHER 只給檢舉人選，認定的理由要指到條款），改寫成別的類別會寫出錯誤的歷史，所以整個 migration
-- 失敗並列出認定的 ID（先例：20261015074826 的 closed_by = ADMIN）。認定被措施與版本引用，不能單獨刪除：本機資料過不了時，重建本機的 hawkr database
-- （DROP DATABASE 後 CREATE DATABASE，再 make migrate、make dev-seed；步驟見 seller 的 CARRYOVER「政策分類」列）後重跑；CI 的資料庫是新建的。
-- measures.request_hash 不重算：它只影響本機資料以舊類別的同一個 request_id 重送（會回「內容不同」）。
DO $$
DECLARE
  other_findings text;
BEGIN
  SELECT string_agg(v.id::text, ', ' ORDER BY v.id) INTO other_findings
  FROM "seller"."violations" v WHERE v.category = 'OTHER';
  IF other_findings IS NOT NULL THEN
    RAISE EXCEPTION '認定的類別 OTHER 沒有對應的政策分類（OTHER 只給檢舉人選），請先人工處理這些認定或重建本機資料庫後重跑：%', other_findings;
  END IF;
END
$$;
-- Drop the old category checks first: they reject the renamed value.
ALTER TABLE "seller"."reports" DROP CONSTRAINT "reports_category_check";
ALTER TABLE "seller"."violations" DROP CONSTRAINT "violations_category_check";
-- Rename the value (no reports_buyer_daily conflict: no row has UNLICENSED_SALE before the rename).
UPDATE "seller"."reports" SET "category" = 'UNLICENSED_SALE' WHERE "category" = 'UNAUTHORIZED_SALE';
UPDATE "seller"."violations" SET "category" = 'UNLICENSED_SALE' WHERE "category" = 'UNAUTHORIZED_SALE';
-- Modify "reports" table
ALTER TABLE "seller"."reports" ADD CONSTRAINT "reports_category_check" CHECK (category = ANY (ARRAY['FOOD_SAFETY'::text, 'PROHIBITED_GOODS'::text, 'UNLICENSED_SALE'::text, 'MISLEADING_INFO'::text, 'FAKE_CERTIFICATION'::text, 'COUNTERFEIT'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'OTHER'::text]));
-- Modify "violations" table
ALTER TABLE "seller"."violations" ADD CONSTRAINT "violations_category_check" CHECK (category = ANY (ARRAY['FOOD_SAFETY'::text, 'PROHIBITED_GOODS'::text, 'UNLICENSED_SALE'::text, 'MISLEADING_INFO'::text, 'FAKE_CERTIFICATION'::text, 'COUNTERFEIT'::text, 'IP_INFRINGEMENT'::text, 'FRAUD'::text, 'OFF_PLATFORM_TRADE'::text, 'PRIVACY'::text, 'IMPERSONATION'::text, 'PLATFORM_ABUSE'::text]));
