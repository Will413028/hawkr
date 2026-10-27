-- 違規的內部說明改成只新增的註記時間軸（violation_notes），違規的修改紀錄改成每一版的完整內容（violation_revisions）。
-- 回填不比對時鐘：violation_changes 的 version 是修改之後的違規版本，建立時是 1，只有修改（生效中才能改）、申訴與裁定
-- 會加一，而申訴之前只有修改，所以修改的版本序列裡第一個缺號就是提出申訴；申訴之前版本等於第幾版，申訴那一版是
-- 缺號減一。只改內部說明的修改不產生版本（內部說明改成註記）；第 n 筆改了賣家可見內容的修改產生第 n + 1 版，每一版的
-- 內容由目前的內容往回推（每筆修改記的是被改掉的值）。
-- 回填的前提是每一次 version 加一都有解釋：version 從 20261002120004 起就有，violation_changes 要到 20261004021301 才出現，
-- 兩者之間的修改沒有紀錄，會被當成申訴，回填出錯誤的歷史。寫入路徑只有：建立（1）、每筆修改紀錄（1）、提出申訴（1）、
-- 裁定申訴（1；接受申訴的撤銷包在這一次）、admin 直接撤銷（1；沒申訴或申訴被駁回之後）。對不上的違規整個 migration 失敗，列出 ID。
DO $$
DECLARE
  unexplained text;
BEGIN
  SELECT string_agg(v.id::text, ', ' ORDER BY v.id) INTO unexplained
  FROM "seller"."violations" v
  WHERE v.version <> 1
    + (SELECT count(*) FROM "seller"."violation_changes" c WHERE c.violation_id = v.id)
    + CASE WHEN v.appeal_status IS NOT NULL THEN 1 ELSE 0 END
    + CASE WHEN v.appeal_status IN ('ACCEPTED', 'REJECTED') THEN 1 ELSE 0 END
    + CASE WHEN v.status = 'REVOKED' AND v.appeal_status IS DISTINCT FROM 'ACCEPTED' THEN 1 ELSE 0 END;
  IF unexplained IS NOT NULL THEN
    RAISE EXCEPTION 'violations.version 有無法由 violation_changes 解釋的加一，回填會寫出錯誤的歷史，請先人工補齊：%', unexplained;
  END IF;
END
$$;
-- Create "violation_revisions" table
CREATE TABLE "seller"."violation_revisions" (
  "violation_id" uuid NOT NULL,
  "revision" integer NOT NULL,
  "statement_to_seller" text NOT NULL,
  "sanction_ends_at" timestamptz NULL,
  "admin_id" bigint NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("violation_id", "revision"),
  CONSTRAINT "violation_revisions_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "violation_revisions_revision_check" CHECK (revision >= 1),
  CONSTRAINT "violation_revisions_statement_check" CHECK ((char_length(statement_to_seller) >= 1) AND (char_length(statement_to_seller) <= 2000))
);
-- Create "violation_notes" table
CREATE TABLE "seller"."violation_notes" (
  "note_id" uuid NOT NULL,
  "violation_id" uuid NOT NULL,
  "admin_id" bigint NOT NULL,
  "body" text NOT NULL,
  "created_at" timestamptz NOT NULL,
  PRIMARY KEY ("note_id"),
  CONSTRAINT "violation_notes_admin_check" CHECK (admin_id > 0),
  CONSTRAINT "violation_notes_body_check" CHECK ((char_length(body) >= 1) AND (char_length(body) <= 2000))
);
-- Create index "violation_notes_violation" to table: "violation_notes"
CREATE INDEX "violation_notes_violation" ON "seller"."violation_notes" ("violation_id", "created_at", "note_id");
-- Modify "violations" table
ALTER TABLE "seller"."violations" ADD COLUMN "current_revision" integer NULL, ADD COLUMN "appeal_revision" integer NULL;
-- Backfill "violation_revisions": 第 1 版是建立時的內容，第 n + 1 版是第 n 筆修改之後的內容
INSERT INTO "seller"."violation_revisions" ("violation_id", "revision", "statement_to_seller", "sanction_ends_at", "admin_id", "created_at")
WITH ranked AS (
  SELECT c.*, row_number() OVER (PARTITION BY c.violation_id ORDER BY c.version) AS n
  FROM "seller"."violation_changes" c
  WHERE c.statement_before IS NOT NULL OR c.ends_at_after IS NOT NULL
),
revisions AS (
  SELECT v.id AS violation_id, 1 AS revision, v.created_by_admin_id AS admin_id, v.created_at
  FROM "seller"."violations" v
  UNION ALL
  SELECT r.violation_id, (r.n + 1)::integer, r.admin_id, r.changed_at
  FROM ranked r
)
SELECT r.violation_id, r.revision,
  COALESCE((SELECT k.statement_before FROM ranked k
            WHERE k.violation_id = r.violation_id AND k.n >= r.revision AND k.statement_before IS NOT NULL
            ORDER BY k.n LIMIT 1), v.statement_to_seller),
  CASE WHEN v.penalty = 'SUSPENSION' THEN COALESCE((SELECT k.ends_at_before FROM ranked k
            WHERE k.violation_id = r.violation_id AND k.n >= r.revision AND k.ends_at_before IS NOT NULL
            ORDER BY k.n LIMIT 1), (SELECT s.ends_at FROM "seller"."suspensions" s WHERE s.violation_id = v.id)) END,
  r.admin_id, r.created_at
FROM revisions r
JOIN "seller"."violations" v ON v.id = r.violation_id;
-- Backfill "violation_notes": 第一則是建立時的內部說明，之後每次改內部說明一則（內容是改完之後的值）
INSERT INTO "seller"."violation_notes" ("note_id", "violation_id", "admin_id", "body", "created_at")
WITH described AS (
  SELECT c.violation_id, c.admin_id, c.changed_at, c.description_before,
         row_number() OVER (PARTITION BY c.violation_id ORDER BY c.version) AS k
  FROM "seller"."violation_changes" c
  WHERE c.description_before IS NOT NULL
)
SELECT uuidv7(), v.id, v.created_by_admin_id,
  COALESCE((SELECT d.description_before FROM described d WHERE d.violation_id = v.id AND d.k = 1), v.description), v.created_at
FROM "seller"."violations" v
UNION ALL
SELECT uuidv7(), d.violation_id, d.admin_id,
  COALESCE((SELECT nx.description_before FROM described nx WHERE nx.violation_id = d.violation_id AND nx.k = d.k + 1), v.description),
  d.changed_at
FROM described d
JOIN "seller"."violations" v ON v.id = d.violation_id;
-- Backfill "violations": 目前的版本是 1 加上改了賣家可見內容的修改筆數；申訴提出時的版本是 1 加上第一個缺號（申訴提出的
-- 版本）之前改了賣家可見內容的修改筆數
UPDATE "seller"."violations" v
SET current_revision = 1 + (SELECT count(*) FROM "seller"."violation_changes" c
                            WHERE c.violation_id = v.id AND (c.statement_before IS NOT NULL OR c.ends_at_after IS NOT NULL)),
    appeal_revision = CASE WHEN v.appeal_status IS NULL THEN NULL ELSE 1 + (
      SELECT count(*) FROM "seller"."violation_changes" c
      WHERE c.violation_id = v.id AND (c.statement_before IS NOT NULL OR c.ends_at_after IS NOT NULL) AND c.version < (
        SELECT min(g) FROM generate_series(2, 1 + (SELECT COALESCE(max(x.version), 1) FROM "seller"."violation_changes" x WHERE x.violation_id = v.id)) g
        WHERE NOT EXISTS (SELECT 1 FROM "seller"."violation_changes" x WHERE x.violation_id = v.id AND x.version = g))
    )::integer END;
-- Modify "violations" table
ALTER TABLE "seller"."violations" DROP CONSTRAINT "violations_description_check", DROP COLUMN "description", DROP CONSTRAINT "violations_statement_check", DROP COLUMN "statement_to_seller", ALTER COLUMN "current_revision" SET NOT NULL, ADD CONSTRAINT "violations_revision_check" CHECK ((current_revision >= 1) AND ((appeal_status IS NULL) = (appeal_revision IS NULL)) AND ((appeal_revision IS NULL) OR ((appeal_revision >= 1) AND (appeal_revision <= current_revision))));
-- Modify "violation_revisions" table
ALTER TABLE "seller"."violation_revisions" ADD CONSTRAINT "violation_revisions_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "violation_notes" table
ALTER TABLE "seller"."violation_notes" ADD CONSTRAINT "violation_notes_violation_id_fkey" FOREIGN KEY ("violation_id") REFERENCES "seller"."violations" ("id") ON UPDATE NO ACTION ON DELETE NO ACTION;
-- Modify "violations" table
ALTER TABLE "seller"."violations" ADD CONSTRAINT "violations_appeal_revision_fkey" FOREIGN KEY ("id", "appeal_revision") REFERENCES "seller"."violation_revisions" ("violation_id", "revision") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED, ADD CONSTRAINT "violations_current_revision_fkey" FOREIGN KEY ("id", "current_revision") REFERENCES "seller"."violation_revisions" ("violation_id", "revision") ON UPDATE NO ACTION ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED;
-- Drop "violation_changes" table
DROP TABLE "seller"."violation_changes";
