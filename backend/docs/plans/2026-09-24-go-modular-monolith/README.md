# 遷移盤點資料

**快照**：monorepo `6c8bc0c15`，2026-09-24。所有路徑都相對於 `backend/`。

**用途**：Go modular monolith 遷移時逐項打勾的清單（見計畫檔的「模組完成的定義」）。每個模組搬遷完成時，屬於它的 RPC、事件、副本都必須有去向：`rpc-servers.csv`、`events.csv`、`replicas.csv` 的 `disposition` 欄由 `backend/scripts/check-migration-inventory.py` 在 CI 檢查（格式與規則見該腳本的說明）。

**優先序**：分析當時的建議和 ADR `docs/adr/2026-09-24-go-modular-monolith.md` 衝突時，以 ADR 為準。例如 `flows.md` D4 建議「改 join `sellers.status`」，但 ADR D3 禁止跨 schema SQL，所以改為 catalog 的 `store_state` 讀取模型。

**生命週期**：這些是遷移前的快照，不會隨程式碼更新。遷移完成後，連同計畫一起封存。

| 檔案 | 內容 |
|---|---|
| `rpc-servers.csv` | 30 個 proto service、730 個 RPC，以及實作它們的 deployable |
| `rpc-calls.csv` | 跨 deployable 的 gRPC 呼叫：呼叫方、被呼叫方、RPC、呼叫點數、範例路徑 |
| `rpc-pairs.csv` | 依呼叫方與被呼叫方彙總 |
| `rpc-method.md` | 同步呼叫圖的做法與已知盲點（例如沒算 gateway 的 HTTP reverse proxy） |
| `external-systems.csv` | 各 deployable 直接往來的第三方系統 |
| `events.csv` | 211 筆事件（151 個 routing key）：producer、consumer、queue、用途，以及 STATE／PROJECTION／SIDE_EFFECT 分類 |
| `event-pairs.csv` | 依 producer 與 consumer 彙總 |
| `event-anomalies.md` | catalog、程式碼、topology 三者不一致之處；沒有 consumer 的事件與 DLQ |
| `flows.md` | 跨服務一致性流程（A1–E2）：現行機制、部分失敗時的行為、單一 transaction 能否取代 |
| `replicas.csv` | 50 份資料副本：20 份刻意的時點快照，30 份讀取副本 |
| `idrefs.csv` | 各服務的資料庫存了哪些別的服務擁有的 ID |
| `db-tables.csv` | 各 deployable 的資料表數量與表名，共 257 張 |
| `cochange.py` | 以 PR 為單位計算 co-change 耦合度，可以重跑；單體完成後改成以模組為單位 |

## co-change 摘要（455 個 PR）

77% 的 PR 只改一個服務。共同 PR 數達 5 個以上的組合：

| 組合 | 共同 PR | 耦合度 |
|---|---|---|
| buyer + seller | 18 | 0.25 |
| invoice + order | 6 | 0.24 |
| aftersales + order | 5 | 0.22 |
| order + payment | 5 | 0.20 |
| order + product | 6 | 0.20 |
| buyer + im | 6 | 0.18 |
| product + seller | 10 | 0.16 |
| im + notification、account + buyer | 5、6 | 0.16 |

cart + order、coupon + order、product-import + product 的共同 PR 都不到 5 個。
