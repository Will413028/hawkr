# discovery 模組

## 職責

推薦賣家排行（`GetRecommendedSellers` 的後端）：排程重算每家合格店家的分數並存成快照，讀取時依分數取出、再即時檢查曝光資格。目前只有排行，沒有其他探索功能。行為規格的唯一來源是 `docs/modules/discovery.md`。

## 擁有的資料

schema `discovery`（`db/schema.sql`）：`seller_scores`（一家合格店家一列）、`recompute_state`（每個排程工作一列）。`store_id` 是 Java 期間的店家 ID，不建 FK。讀取模型：`seller_scores` 本身，由排程整批重建，不由事件維護。

## 發布的讀取介面

無。沒有 `pub_*`。對外是 Go 公開 API `RecommendedSellers(ctx, limit)`，回傳 `Recommendation`（每列店家 ID 與近 30 天成立訂單數，加快照的 `ComputedAt`），形狀見 `docs/modules/discovery.md` §5。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、port 與型別別名；port（`Orders`、`Products`、`Reviews`、`Stores`）由 discovery 定義，在 `internal/ranking/ports.go`，根套件 `ports.go` 轉出。
- `internal/model` 是公式與型別（純函式，沒有 I/O）；`internal/ranking` 是重算、讀取與 River job；`internal/store` 是 sqlc 的查詢套件。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。依賴的模組只有 seller；order、product、review 的 legacy client 由 `cmd/hawkr/discovery.go` 注入，模組不 import `internal/legacy`。

## 不變量

- 快照整批取代，且只有開始時間晚於目前快照那次的重算能寫入。守門：`TestAnOlderRunFinishingLaterDoesNotOverwriteANewerSnapshot`。規格：`docs/modules/discovery.md` §3。
- 任何上游失敗都不進入取代，上一份快照不變。守門：`TestUpstreamFailureKeepsThePreviousSnapshot`。規格：§6。
- 沒有靜默預設值：失敗的評價不會變成 0；上游給不合法的數值或沒被問的店家，整次重算失敗。守門：`TestFailedRatingIsNeverTurnedIntoZero`、`TestReviewReturningAnUnaskedStoreFailsTheRun`、`TestComputeRejectsBadUpstreamData`。規格：§6。
- 讀取是同一份快照，重算在分頁之間提交也不重複、不漏。守門：`TestReadSeesOneSnapshotEvenWhenARecomputeCommitsBetweenPages`。規格：§5。
- 曝光資格讀取時即時問 seller，停權立即生效；seller 失敗時讀取失敗。守門：`TestStoreSuspendedAfterTheRunIsFilteredAtReadTime`、`TestEligibilityErrorAtReadTimeFailsTheRead`。規格：§5。
- `limit` 必須是 1 到 50。守門：`TestLimitMustBeBetweenOneAndFifty`。規格：§5。
- 每個成分在 0 到 1，總分是有限值，DB 不寫死權重。守門：`TestSchemaChecksComponentsAndFiniteScoreButNotTheFormulaWeights`。規格：§4。
- 新店加分從第一次可公開曝光起算。守門：`TestNewnessCountsFromTheFirstExposableDay`。規格：§4。
- 沒有訂單與評價時順序由新店加分、`published_at DESC`、`store_id DESC` 決定，沒有補位路徑。守門：`TestZeroDataRanksByNewnessThenPublishedAt`、`TestEqualScoresOrderByPublishedAtThenStoreIDDescending`。規格：§4。
- 評價與資格各自分批呼叫（評價每批至多 100 家，資格每批至多 `seller.MaxBatchSize`）。守門：`TestRatingsAreFetchedInBatchesOfAtMostOneHundredAndEveryCandidateIsAskedOnce`、`TestEligibilityIsAskedInBatchesOfAtMostMaxBatchSize`。規格：§3。
- 重算排程每 15 分鐘、啟動即跑，逾時短於間隔。守門：`TestPeriodicConfiguration`。規格：§3。

## 不負責

- 店名、logo、粉絲數等顯示欄位：`internal/httpapi/discovery` 用 seller 的 `StoreCards`、buyer 的 `SubscriberCounts` 組裝；卡片的評分取本模組回傳的快照值，不另查 review。
- Connect API 與 `max_results` 預設值（10，0 的語意）：傳輸層（`httpapi/discovery.DefaultMaxResults`）。
- 「可公開曝光」的規則：seller 的 `ExposableStores`；「有公開商品」：product（或 catalog）；訂單統計的口徑：order 端。
- 營運的置頂與排除、其他探索功能：目前沒有實作，不在本模組。
- 新鮮度告警：單體對 River job 狀態的監控。

## 決策紀錄

從 seller-service 沿用的每個機制、當時的決定與理由，以及與 seller-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-10-03-recommended-sellers-scoring.md`（資格、分數公式、定期重算與快照；文末修訂：評分由店家卡片提供）。
