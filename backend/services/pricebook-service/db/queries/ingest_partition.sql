-- 宣告：為指定市場×日期建立分區。已存在的不動（重跑安全，也不會把
-- complete 打回 pending）。休市日在呼叫端就決定好 status，不在 SQL 判斷。
-- name: DeclarePartitions :exec
INSERT INTO ingest_partition (market_id, trade_date, status, declared_at)
SELECT unnest(@market_ids::bigint[]), unnest(@trade_dates::date[]),
       unnest(@statuses::smallint[]), @declared_at
ON CONFLICT (market_id, trade_date) DO NOTHING;

-- 工作清單：還沒拿到、且還沒放棄的格子，最舊的先做。
-- name: ListPendingPartitions :many
SELECT market_id, trade_date, attempt_count
FROM ingest_partition
WHERE status = @pending_status
ORDER BY trade_date, market_id;

-- 抓到資料：終態。
-- name: MarkPartitionsComplete :exec
UPDATE ingest_partition
SET status = @complete_status, completed_at = @completed_at,
    attempt_count = attempt_count + 1, last_attempt_at = @completed_at,
    last_error = NULL
WHERE (market_id, trade_date) IN (
  SELECT unnest(@market_ids::bigint[]), unnest(@trade_dates::date[])
);

-- 這次沒拿到：留在 pending，attempt_count 進位；超過上限的轉 abandoned。
-- 兩件事寫在同一句，避免「加完次數才發現該放棄」需要第二趟往返。
-- name: MarkPartitionsAttempted :exec
UPDATE ingest_partition
SET attempt_count = attempt_count + 1,
    last_attempt_at = @attempted_at,
    last_error = @last_error,
    status = CASE WHEN attempt_count + 1 >= @max_attempts::int
                  THEN @abandoned_status::smallint ELSE status END
WHERE (market_id, trade_date) IN (
  SELECT unnest(@market_ids::bigint[]), unnest(@trade_dates::date[])
);

-- 休市：終態，不消耗 attempt。宣告之後才同步到的休市日曆會走這裡。
-- name: MarkPartitionsRestDay :exec
UPDATE ingest_partition
SET status = @rest_day_status
WHERE status = @pending_status
  AND (market_id, trade_date) IN (
    SELECT unnest(@market_ids::bigint[]), unnest(@trade_dates::date[])
  );

-- 監控用：各狀態的格數，abandoned 另外帶最舊的一格供告警訊息使用。
-- name: CountPartitionsByStatus :many
SELECT status, count(*)::bigint AS cnt, min(trade_date)::date AS oldest
FROM ingest_partition GROUP BY status;

-- 宣告需要的兩份輸入：目前 active 的市場、以及該區間內的休市日。
-- 兩者都只在每日宣告時各查一次，量都很小（21 個市場、一次窗口的休市日）。
-- name: ListActiveMarkets :many
SELECT id, market_code FROM market WHERE active ORDER BY id;

-- name: ListRestDaysInRange :many
SELECT DISTINCT market_id, rest_date
FROM market_rest_day
WHERE rest_date BETWEEN @from_date AND @to_date;

-- 人工接受這一格為「上游確認沒有資料」。終態，且是唯一會寫 accepted_* 三欄的
-- 路徑（見 internal/domain/constants.go 的 PartitionStatusAcceptedGap）。
--
-- WHERE 帶 status = @abandoned_status：只有已經放棄的格子能被接受。還在 pending
-- 的格子代表重試還沒用完，這時候接受等於跳過系統自己的判定；complete 的格子
-- 更不該被標成缺口。條件不符時 affected rows = 0，呼叫端據此回錯而不是靜默成功。
-- name: AcceptPartitionGap :execrows
UPDATE ingest_partition
SET status = @accepted_status,
    accepted_at = @accepted_at,
    accepted_by = @accepted_by,
    accepted_reason = @accepted_reason
WHERE market_id = @market_id
  AND trade_date = @trade_date
  AND status = @abandoned_status;

-- 接受前的現況查核：這一格存在嗎、現在是什麼狀態。
-- name: GetPartition :one
SELECT market_id, trade_date, status, attempt_count, last_attempt_at,
       completed_at, last_error, accepted_at, accepted_by, accepted_reason
FROM ingest_partition
WHERE market_id = @market_id AND trade_date = @trade_date;

-- 市場代碼 → id。刻意不過濾 active：接受缺口是維運動作，對一個剛停用的市場
-- 的歷史缺口做這件事是合理的。
-- name: GetMarketIDByCode :one
SELECT id FROM market WHERE market_code = @market_code;

-- 接受缺口時發現上游其實回了休市哨兵：那一格的正確終態是 rest_day 而不是
-- accepted_gap。MarkPartitionsRestDay 只認 pending，這裡專門處理 abandoned。
-- name: ResolveAbandonedPartitionAsRestDay :execrows
UPDATE ingest_partition
SET status = @rest_day_status, last_error = NULL
WHERE market_id = @market_id
  AND trade_date = @trade_date
  AND status = @abandoned_status;
