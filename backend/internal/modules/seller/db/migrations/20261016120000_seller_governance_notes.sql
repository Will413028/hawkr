-- 內部註記改名 governance_notes（之後也掛通報）：資料原地保留，只改資料表、主鍵、外鍵、檢查與 NOT NULL 約束與索引的名稱（PostgreSQL 18 的 NOT NULL 有自己的約束名稱，
-- 不改的話與 schema.sql 建出來的約束名稱不同）。
-- Rename "measure_notes" table
ALTER TABLE "seller"."measure_notes" RENAME TO "governance_notes";
-- Rename constraints of "governance_notes" table
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_pkey" TO "governance_notes_pkey";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_measure_id_fkey" TO "governance_notes_measure_id_fkey";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_admin_check" TO "governance_notes_admin_check";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_body_check" TO "governance_notes_body_check";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_note_id_not_null" TO "governance_notes_note_id_not_null";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_measure_id_not_null" TO "governance_notes_measure_id_not_null";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_admin_id_not_null" TO "governance_notes_admin_id_not_null";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_body_not_null" TO "governance_notes_body_not_null";
ALTER TABLE "seller"."governance_notes" RENAME CONSTRAINT "measure_notes_created_at_not_null" TO "governance_notes_created_at_not_null";
-- Rename index "measure_notes_measure" to "governance_notes_measure"
ALTER INDEX "seller"."measure_notes_measure" RENAME TO "governance_notes_measure";
