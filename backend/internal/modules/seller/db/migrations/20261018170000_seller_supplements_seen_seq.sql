-- 「有新的補充」的已看過標記改成補充的編號（A3b 步驟 10e 第 3c-a 刀的最後一輪審查修正，Will 決定 2026-10-18）：
-- reports.supplements_seen_at（應用程式時鐘）拿來比補充的 created_at（可能是另一個實例的時鐘），時鐘不同步時標記成功卻仍顯示未看過。
-- 改成 reports.supplements_seen_seq（SMALLINT，0..10）：管理員看過的最新補充的編號（report_supplements.seq），「有新的補充」= 存在編號更大的補充，不依賴時鐘。
-- 既有資料：標記取代為建立時間不晚於舊標記的補充的最大編號；從沒記過的是 0。
-- Modify "reports" table
ALTER TABLE "seller"."reports" ADD COLUMN "supplements_seen_seq" smallint NOT NULL DEFAULT 0;
UPDATE "seller"."reports" r SET "supplements_seen_seq" = n.seq
FROM (SELECT p.report_id, max(p.seq) AS seq FROM "seller"."report_supplements" p
      JOIN "seller"."reports" r2 ON r2.id = p.report_id
      WHERE r2.supplements_seen_at IS NOT NULL AND p.created_at <= r2.supplements_seen_at
      GROUP BY p.report_id) n
WHERE n.report_id = r.id;
ALTER TABLE "seller"."reports" DROP CONSTRAINT "reports_supplements_seen_check", DROP COLUMN "supplements_seen_at", ADD CONSTRAINT "reports_supplements_seen_check" CHECK ((supplements_seen_seq >= 0) AND (supplements_seen_seq <= 10));
