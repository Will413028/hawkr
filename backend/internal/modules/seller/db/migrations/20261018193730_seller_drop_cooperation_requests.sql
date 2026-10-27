-- 配合要求的退場（A3b 步驟 10e 第 3c-b 刀，B22）：舊的配合要求沒有給賣家的說明與期限，不能誠實地轉成詢問（3c-a 的 W2 先例：沒有誠實的轉換就不寫）；
-- 有列就失敗並列出通報 ID，本機照 3c-a 設計書 §2.3 重建後重跑。3c-a 起沒有任何寫入者（RPC 與事件都已刪除），CI 的資料庫是新建的。
DO $$ DECLARE ids text; BEGIN
  SELECT string_agg(DISTINCT report_id::text, ', ' ORDER BY report_id::text) INTO ids FROM seller.report_cooperation_requests;
  IF ids IS NOT NULL THEN RAISE EXCEPTION '舊的要求賣家配合沒有對應的詢問內容，請重建本機資料庫或人工刪除後重跑：%', ids; END IF;
END $$;
-- Drop "report_cooperation_requests" table
DROP TABLE "seller"."report_cooperation_requests";
