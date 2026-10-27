-- 通報佐證改以「第幾次送出」編號、補充已看過的時間（A3b 步驟 10e 第 3c-a 刀的設計審查修正，Will 決定 2026-10-18）：
--  1. report_supplements.seq（1..10，每筆通報內唯一）：每筆通報最多 10 則補充改由 DB 保證，不只是模組檢查。
--  2. report_evidence 的鍵從 (report_id, position)（position 0..54 連號，上界 55 = 5 × 11，「每次送出最多 5 張」退成模組檢查）改成
--     (report_id, submission_seq, position)：submission_seq 0 是原通報、1..10 是補充，position 0..4，所以每次送出最多 5 張由 DB 保證；
--     supplement_id 與 submission_seq 一致由 CHECK 與複合外鍵保證。evidence_accesses 跟著用同一個鍵指向佐證列。
--  3. reports.supplements_seen_at：管理員最後一次分案或移動的時間，「有新的補充」不再拿案件的 updated_at 當已看過的點。
--  4. reports_contact_cap：匿名聯絡方式的絕對保留上限（建立滿 365 天，不論有沒有結果）的清理索引。
-- 既有資料：補充依建立時間編號；佐證依舊的 position 在各自那次送出內重新編號，讀取紀錄跟著換成新的鍵。
-- Modify "report_supplements" table
ALTER TABLE "seller"."report_supplements" ADD COLUMN "seq" smallint NULL;
UPDATE "seller"."report_supplements" p SET "seq" = n.seq
FROM (SELECT id, (row_number() OVER (PARTITION BY report_id ORDER BY created_at, id))::smallint AS seq FROM "seller"."report_supplements") n
WHERE n.id = p.id;
ALTER TABLE "seller"."report_supplements" ALTER COLUMN "seq" SET NOT NULL, ADD CONSTRAINT "report_supplements_seq_check" CHECK ((seq >= 1) AND (seq <= 10)), ADD CONSTRAINT "report_supplements_report_seq_key" UNIQUE ("report_id", "seq"), ADD CONSTRAINT "report_supplements_seq_key" UNIQUE ("id", "report_id", "seq");
-- Modify "reports" table
ALTER TABLE "seller"."reports" ADD COLUMN "supplements_seen_at" timestamptz NULL, ADD CONSTRAINT "reports_supplements_seen_check" CHECK ((supplements_seen_at IS NULL) OR (supplements_seen_at >= created_at));
-- Create index "reports_contact_cap" to table: "reports"
CREATE INDEX "reports_contact_cap" ON "seller"."reports" ("created_at") WHERE ((reporter_buyer_id IS NULL) AND (contact_erased_at IS NULL));
-- Modify "evidence_accesses" table
ALTER TABLE "seller"."evidence_accesses" DROP CONSTRAINT "evidence_accesses_report_fkey", ADD COLUMN "submission_seq" smallint NULL;
-- Modify "report_evidence" table
ALTER TABLE "seller"."report_evidence" DROP CONSTRAINT "report_evidence_pkey", DROP CONSTRAINT "report_evidence_supplement_fkey", DROP CONSTRAINT "report_evidence_position_check", ADD COLUMN "submission_seq" smallint NULL, ADD COLUMN "new_position" integer NULL;
WITH numbered AS (
  SELECT e.report_id, e.position AS old_position, coalesce(p.seq, 0) AS seq,
         (row_number() OVER (PARTITION BY e.report_id, coalesce(p.seq, 0) ORDER BY e.position) - 1)::integer AS new_position
  FROM "seller"."report_evidence" e
  LEFT JOIN "seller"."report_supplements" p ON p.id = e.supplement_id
)
UPDATE "seller"."report_evidence" e SET "submission_seq" = n.seq, "new_position" = n.new_position
FROM numbered n WHERE n.report_id = e.report_id AND n.old_position = e.position;
UPDATE "seller"."evidence_accesses" a SET "submission_seq" = e.submission_seq, "position" = e.new_position
FROM "seller"."report_evidence" e WHERE a.report_id = e.report_id AND a.position = e.position;
UPDATE "seller"."report_evidence" SET "position" = "new_position";
ALTER TABLE "seller"."report_evidence" DROP COLUMN "new_position", ALTER COLUMN "submission_seq" SET NOT NULL, ADD CONSTRAINT "report_evidence_position_check" CHECK (("position" >= 0) AND ("position" <= 4)), ADD CONSTRAINT "report_evidence_submission_check" CHECK ((submission_seq >= 0) AND (submission_seq <= 10)), ADD CONSTRAINT "report_evidence_supplement_check" CHECK ((submission_seq = 0) = (supplement_id IS NULL)), ADD PRIMARY KEY ("report_id", "submission_seq", "position"), ADD CONSTRAINT "report_evidence_supplement_fkey" FOREIGN KEY ("supplement_id", "report_id", "submission_seq") REFERENCES "seller"."report_supplements" ("id", "report_id", "seq") ON UPDATE NO ACTION ON DELETE NO ACTION;
ALTER TABLE "seller"."evidence_accesses" ADD CONSTRAINT "evidence_accesses_seq_check" CHECK ((report_id IS NULL) = (submission_seq IS NULL)), ADD CONSTRAINT "evidence_accesses_report_fkey" FOREIGN KEY ("report_id", "submission_seq", "position") REFERENCES "seller"."report_evidence" ("report_id", "submission_seq", "position") ON UPDATE NO ACTION ON DELETE NO ACTION;
