-- 通報佐證的位置加上界（A3b 步驟 10e 第 3c-a 刀之三的審查修正）：位置從 0 連號，一筆通報全部的佐證最多 55 張（原通報加每則補充各 5 張，model.MaxReportEvidence =
-- 5 × (1 + 10)），其他佐證表（measure_request_evidence、kyc_documents）也都有上界。20261018092959_seller_report_cases 為了讓補充的佐證接在後面編號，把原本的 < 5
-- 放寬成只有 >= 0，沒有補回上界；讀取端（governance/evidence.go）以 MaxReportEvidence 為位置的界限，DB 也要擋住。TestChecksMatchGo 綁住這個值。
-- 既有的列都在範圍內：每次送出最多 5 張、每筆通報最多 10 則補充（模組檢查），所以不需要轉換資料。
-- Modify "report_evidence" table
ALTER TABLE "seller"."report_evidence" DROP CONSTRAINT "report_evidence_position_check", ADD CONSTRAINT "report_evidence_position_check" CHECK (("position" >= 0) AND ("position" < 55));
