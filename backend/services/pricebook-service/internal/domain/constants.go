package domain

// 匯入稽核（ingest_run）的 kind / status 代碼。
//
// 這兩組是本服務的內部簿記代碼：不進 proto、不進 MQ、不進 HTTP 回應，
// 依 `docs/代碼管理（CodeMeta）/README.md:54`「單一服務的純內部 local enum 可不納管」
// 留在服務內，不走 CodeMeta。
//
// 型別為 int16，對齊 sqlc 對 `SMALLINT NOT NULL` 產出的
// `gen.CreateIngestRunParams.Kind` / `.Status`，呼叫端不需轉型。
const (
	// IngestKindDaily 每日排程觸發的匯入。
	IngestKindDaily int16 = 1
	// IngestKindBackfill 手動 backfill CLI 觸發的匯入。
	IngestKindBackfill int16 = 2
)

const (
	// IngestStatusRunning 匯入進行中（建立 ingest_run 時的初始狀態）。
	IngestStatusRunning int16 = 1
	// IngestStatusSucceeded 匯入全數完成。
	IngestStatusSucceeded int16 = 2
	// IngestStatusFailed 匯入中斷；error_message 記錄原因，已成功的段落不回滾。
	IngestStatusFailed int16 = 3
	// IngestStatusNeedsAttention 流程本身沒失敗，但結果不合理到需要人工確認：
	// 有列被跳過，且沒有任何一列寫入或修訂（inserted+revised == 0）。
	//
	// 典型成因是上游改了回應格式（例如 TransDate 的寫法變了），使當日每一列都在
	// 解析階段被跳過——每列各有一筆 log，但 ingest_run 這張稽核表沒有
	// skipped_count 欄位，所以整輪看起來就只是「succeeded 且 inserted=0」，
	// 與「今天本來就沒有行情」無法區分。用一個獨立狀態把兩者分開，
	// error_message 補上 fetched / skipped 的實際數字。
	//
	// status 是無 CHECK 約束的 SMALLINT（見 db/schema/schema.sql 的 ingest_run），
	// 且全服務沒有任何一處依 status 值做查詢或分支，因此新增這個值不需要 migration，
	// 也不會改變既有兩個狀態的語意。
	IngestStatusNeedsAttention int16 = 4
)

// ingest_partition.status 的值域。一格 = 一個 (市場, 交易日)。
//
// 型別為 int16，對齊 sqlc 對 `SMALLINT NOT NULL` 的產出，呼叫端不需轉型
// （比照上面 IngestKind/IngestStatus 的既有慣例）。
const (
	// PartitionStatusPending 已宣告「這一格應該要有資料」，但還沒拿到。
	// 這是唯一會被工作清單撿走的狀態。
	PartitionStatusPending int16 = 1
	// PartitionStatusComplete 已寫入至少一列 price_observation。終態。
	PartitionStatusComplete int16 = 2
	// PartitionStatusRestDay 該市場當天休市，本來就不該有資料。終態。
	// 依 market_rest_day 判定，不消耗 attempt。
	PartitionStatusRestDay int16 = 3
	// PartitionStatusAbandoned 重試次數用盡仍拿不到。終態，但**需要人看**：
	// 它代表一格永久的資料空洞，不是正常結束。
	PartitionStatusAbandoned int16 = 4
	// PartitionStatusAcceptedGap 人工確認「上游確實沒有這一天的資料」後接受
	// 的缺口。終態，且**不告警**——它跟 Abandoned 的差別不在資料（兩者都是
	// 空的），而在**還有沒有人能做點什麼**：Abandoned 可以 backfill，這個不行。
	//
	// 🔴 只有 cmd/pricebook-backfill --accept-gap 會寫這個值，排程與 backfill
	// 的任何路徑都不會。系統不准自己把自己的告警關掉——那正是 ingest_run 的
	// NeedsAttention 之所以不可信的原因（自報結果、自己決定要不要在意）。
	// 接受的當下會重新向上游查一次：查得到資料就改走正常寫入並標 Complete，
	// 拒絕接受。理由與否決的替代方案見 ADR
	// 2026-09-03-pricebook-accepted-gap-terminal-state。
	//
	// ⚠️ 值改動時要同步改 db/schema/schema.sql 的
	// ingest_partition_accepted_evidence CHECK（那裡有一個裸的 5）。
	PartitionStatusAcceptedGap int16 = 5
)

// MaxIngestAttempts 是一格在被判定 Abandoned 之前，允許的抓取次數。
//
// 這個數字就是「多晚算太晚」這個決定本身——在分區模型之前，它是
// cron 的 windowDays 大小的意外副作用（滑出窗口＝再也不會被抓，而且沒有任何
// 東西會發現）。
//
// 取 14 的依據是 2026-09-02 對 dev 實測的 MOA 上架延遲分佈（近 40 天、646 個
// 市場-交易日第一次落進我方 DB 的延遲）：
//
//	0 天 272（42%）｜1 天 178（28%）｜2 天 108（17%）｜3 天 68（11%）
//	4 天 11｜5 天 5｜6 天 3｜11 天 1
//
// 排程每天跑一次，所以 attempt 數約等於天數。14 覆蓋到觀測到的最長延遲
// （11 天）再加約 3 天餘裕。⚠️ 這是可調參數，不是常數真理——分佈會隨上游
// 行為變，重新校準時請重跑那份查詢而不是憑印象加大。
const MaxIngestAttempts = 14
