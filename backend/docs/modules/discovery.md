# discovery 模組行為規格

本文是單體 `discovery` 模組行為的唯一來源。目前模組只有推薦賣家排行：排程重算每家合格店家的分數並存成快照，讀取時依分數取出、再即時檢查曝光資格。

## 1. 範圍與公開介面

- 模組只提供 `RecommendedSellers(ctx, limit)`（`GetRecommendedSellers` 的後端）；根套件只保留 `Module`、`New`、轉呼叫方法、port 與型別別名。
- 上游由 discovery 定義 port：`Orders`、`Products`、`Reviews`、`Stores`（`internal/ranking/ports.go`，根套件 `ports.go` 轉出）。order、product、review 的 legacy client 由 `cmd/hawkr/discovery.go` 注入，模組不 import `internal/legacy`；依賴的模組只有 seller（`ranking` 用它的公開型別 `seller.ExposableStore`）。
- 對外的 Connect API 是 `DiscoveryPublicService.SuggestRecommendedSellers`（`internal/httpapi/discovery`）；marketplace 的聚合已刪除，由 client 組合。
- 模組內依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 2. 資料

schema `discovery`（`db/schema.sql`）：

- `seller_scores`：一家合格店家一列。欄位有 `store_id`（業務鍵）、分數與四個成分、各成分的原始輸入、評價數與平均、`published_at`、公式版本、`computed_at`。索引 `seller_scores_rank_idx` 與讀取的排序一致。
- `recompute_state`：每個排程工作一列，目前只有 `seller_scores`。欄位有最近一次嘗試的開始時間、目前快照那次重算的開始與結束時間、最近一次失敗的時間與有界的錯誤文字。
- `store_id` 是 Java 期間的店家 ID，不建 FK。
- `seller_scores` 本身是讀取模型：由排程從 order、review、product、seller 整批重建，不由事件維護（時間窗統計用事件增量會漏收與漂移）。

## 3. 重算

River 排程，每 15 分鐘一次、啟動即跑、單次嘗試上限 10 分鐘（短於間隔）。流程：

1. 先記下這次的開始時間（`recompute_state`）。
2. 候選店家 =「有公開商品」（product port）∩「可公開曝光」（seller 批次判斷，每批 `seller.MaxBatchSize`，附 `PublishedAt`）。
3. 近 30 天訂單統計一次取回（order port）。
4. 評價以批次取（review port，每批最多 `MaxRatingBatch` = 100 家，依序呼叫，每家候選恰問一次；沒有評價的店不在回傳裡）。
5. 以公式 v1 算分，在一個 transaction 內整批取代 `discovery.seller_scores`、更新 `recompute_state`，並先鎖住狀態列（序列化重疊的重算；讀取不受影響）。
6. 只有開始時間晚於目前快照那次的重算才能寫入；較早開始的結果（即使較晚結束）被丟棄並 log。River 的 unique 只去除重複的排入，不保證不重疊。

成立訂單的口徑由 order 端定義（`ListStoreOrderStats`）：已付款、未取消、未全額退款；時間窗以付款時間計，賣家取消以取消時間計。部分退款的訂單仍是成立的購買，只計買家數與筆數，不計金額。

## 4. 公式 v1

實作在 `internal/model/score.go`（純函式，沒有 I/O；常數都在檔內）。

分數 = 0.5·需求 + 0.3·品質 + 0.2·新店 − 0.5·可靠度扣分，每個成分都在 0 到 1。

- 需求：log1p(不重複買家數) 除以候選中的最大值；沒有任何買家時為 0。
- 品質：評分的貝氏平均（先驗權重 10；全站平均取候選的評價數加權平均，沒有任何評價時 4.0），正規化為 (r−1)/4。
- 可靠度扣分：賣家取消 /（成立 + 取消 + 10）。
- 新店：自第一次可公開曝光起 30 天內線性遞減。起點是 seller 的 `PublishedAt`（公開時間 `stores.first_exposed_at`），不是開店、申請或核准時間。
- 權重或形狀有任何改動，`FormulaVersion` 加一。
- 沒有訂單與評價時，所有店的訊號相同，順序由新店加分、`published_at DESC`、`store_id DESC` 決定，沒有另一條補位路徑。

## 5. 讀取

`RecommendedSellers(ctx, limit)`：

- `limit` 必須是 1 到 50；預設值屬於傳輸層（`httpapi/discovery.DefaultMaxResults`，10）。
- 依 `(score, published_at, store_id)` 由大到小分頁讀快照（每頁 100 列），每頁即時問 seller 曝光資格，不可曝光的略過，補到 `limit` 或快照盡頭為止。
- 整個讀取（狀態、每一頁、seller 的資格查詢）在同一個唯讀 `REPEATABLE READ` transaction 內，seller 的查詢加入同一個 transaction，不另佔連線；重算在中途提交也不會讓讀取重複或漏掉店家。呼叫端的 ctx 不可已經在 transaction 內。
- 曝光資格不在快照裡：停權立即生效；seller 失敗時讀取失敗（fail-closed）。
- 回傳 `Recommendation`：每列有店家 ID 與近 30 天成立訂單數（與排名同一份快照），以及快照的 `ComputedAt`（那次重算的開始時間）。`ComputedAt` 為 `nil` 代表從沒成功算過；有值而清單為空代表算過了、沒有合格店家。
- 排名用的評價只留在快照內，不對外輸出；顯示用的評價由 `internal/httpapi/discovery` 與店家卡片一起批次讀取。
- 店名、logo、粉絲數等顯示欄位由 `internal/httpapi/discovery` 用 seller 的 `StoreCards`、buyer 的 `SubscriberCounts` 組裝。

## 6. 失敗與新鮮度

- 評價、統計、公開商品、資格任何一個失敗，整次重算失敗並以 error 記錄上游名稱，上一份快照不變；失敗的評價不會變成 0。
- 上游給的數值不合法（負數、買家數大於訂單數、評分超出刻度）或回了沒被問的店家，同樣讓整次重算失敗，不排除單一店家（`model.Input.Validate`）。
- `recompute_state` 記錄最近一次成功與失敗，失敗的錯誤文字有長度上限。
- 告警不在模組內：由單體對 River job 狀態的監控負責，本模組只提供狀態與 log。

## 7. 已知限制

- 店家下架最後一個商品後，最多一個重算週期（15 分鐘）仍會出現在榜上。
- 訂單統計的口徑由 order 端定義，不在本模組；口徑變動由 order 的契約帶入。
- 讀取的 transaction 在 seller 的資格查詢期間保持開啟（同一條連線）；`limit` 最多 50、快照每頁 100 列，讀取的時間以毫秒計。
- 營運的置頂與排除、首頁區塊排序等其他探索功能，目前沒有實作。
