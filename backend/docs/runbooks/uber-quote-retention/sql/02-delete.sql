-- 02：實際刪除。**包在交易裡，預設 ROLLBACK**。
-- DB：hawkr_shipping
-- 用法：psql -v ON_ERROR_STOP=1 -f 02-delete.sql   → 看完數字後，確認無誤再把結尾改成 COMMIT 重跑
--
-- 🔴 前置：01-preview 的 B 段必須是 0 列。B 段有列代表 TTL 掃描或建單鏈壞了，
--    那些列是**證據**不是垃圾——刪掉它們等於把 bug 的唯一痕跡銷毀。
--    本支不靠人記得這件事：下面的閘門會自己檢查，不通過就整筆拒絕。
--
-- 為什麼刪除範圍只含終態：見 runbook §4。
-- 為什麼可以重跑：述詞是「終態 ＋ 逾期」，刪過的列不會再命中；孤兒掃描同理。
--
-- ⚠️ retention_days 在本檔只出現在 \set 這一行。閘門與刪除都走同一張暫存表，
--    不得在別處再寫一次天數——兩處天數不同會讓閘門檢查的區間與實際刪的區間錯開，
--    而那是完全無聲的。

\set retention_days 90

BEGIN;

-- ---------------------------------------------------------------------------
-- 閘門（fail-closed，跑在任何 DELETE 之前）
-- ---------------------------------------------------------------------------
CREATE TEMP TABLE overdue_non_terminal ON COMMIT DROP AS
SELECT quote_id, status, created_at
FROM uber_quotes
WHERE status IN ('CREATED', 'CHECKOUT_ACCEPTED')
  AND created_at < NOW() - (:retention_days || ' days')::interval;

\echo '--- 閘門：逾期且非終態的 quote（必須 0）---'
SELECT status, count(*) AS rows FROM overdue_non_terminal GROUP BY status ORDER BY status;

DO $$
DECLARE n bigint;
BEGIN
  SELECT count(*) INTO n FROM overdue_non_terminal;
  IF n > 0 THEN
    RAISE EXCEPTION
      '有 % 列逾保留期卻仍非終態（CREATED／CHECKOUT_ACCEPTED）。'
      '這是缺陷訊號不是清理對象：CREATED 逾期代表 TTL 掃描沒在跑，'
      'CHECKOUT_ACCEPTED 逾期代表收了錢沒建單。本輪拒絕執行，先查 runbook 第 4 節。', n;
  END IF;
END $$;

-- ---------------------------------------------------------------------------
-- 刪除
-- ---------------------------------------------------------------------------
-- 鎖定本輪要刪的 quote_id，讓 package_plan 與 uber_quotes 刪的是同一批。
-- （不靠「先刪 quote 再掃孤兒」：那在交易中途失敗時會留下真孤兒。）
CREATE TEMP TABLE doomed_quotes ON COMMIT DROP AS
SELECT quote_id
FROM uber_quotes
WHERE status IN ('CONSUMED', 'EXPIRED', 'SUPERSEDED')
  AND created_at < NOW() - (:retention_days || ' days')::interval;

\echo '--- 本輪鎖定的 quote 數 ---'
SELECT count(*) AS doomed_quotes FROM doomed_quotes;

-- 🔴 順序：子表先於母表。V13 無 FK ⇒ DB 不會替你擋順序錯誤，錯了就是留下孤兒。
\echo '--- 刪 package_plan（掛在本輪 quote 上）---'
DELETE FROM package_plan p USING doomed_quotes d WHERE p.quote_id = d.quote_id;

\echo '--- 刪 package_plan（前一輪中斷留下的孤兒）---'
DELETE FROM package_plan p
WHERE NOT EXISTS (SELECT 1 FROM uber_quotes q WHERE q.quote_id = p.quote_id);

\echo '--- 刪 uber_quotes ---'
DELETE FROM uber_quotes q USING doomed_quotes d WHERE q.quote_id = d.quote_id;

-- 🔴 預設 ROLLBACK。確認上面每一行的 DELETE 筆數與 01-preview 相符之後，
--    把下一行改成 COMMIT; 再跑一次。
ROLLBACK;
