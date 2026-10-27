-- #1228 淘汰買家重啟（reopen）流程：被拒後統一改走平台申訴（appeal）。
-- 移除 reopen_count / max_reopen_count 欄位。
--
-- ⚠️ 破壞性 DDL（DROP COLUMN）：部署前已 pg_dump 備份 aftersale_cases。
-- dev 盤點（2026-06-30）：status=65 共 0 筆、reopen_count>0 共 0 筆。
-- 下方 UPDATE 為防禦性遷移，保證任何環境（含 prod 若有殘留 65）皆安全：
--   被拒後重啟中（REOPENED=65）的 case → 回退至 REJECTED(60) 並清空 process_deadline，
--   買家可於申訴期內走 §10 平台申訴救濟（符合產品方向「被拒後唯一救濟＝申訴」）。

-- 1. 防禦性遷移：任何殘留 REOPENED(65) → REJECTED(60)，清空賣家處理期限（60 無 process_deadline）
UPDATE aftersale_cases
   SET status = '60',
       process_deadline = NULL
 WHERE status = '65';

-- 2. 移除 reopen 相關欄位
ALTER TABLE aftersale_cases DROP COLUMN reopen_count;
ALTER TABLE aftersale_cases DROP COLUMN max_reopen_count;
