-- 刪除 reports_contact_retention（resolved_at 的部分索引）：EraseExpiredContacts 改成依 created_at 掃描（reports_contact_cap），
-- 本機 PostgreSQL 18、30 萬列、含與不含此索引的 EXPLAIN 都選 reports_contact_cap，此索引沒有使用者（只增加寫入成本）。
-- Drop index "reports_contact_retention" from table: "reports"
DROP INDEX "seller"."reports_contact_retention";
