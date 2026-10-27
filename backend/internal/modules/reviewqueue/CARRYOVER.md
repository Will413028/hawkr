# reviewqueue 模組的沿用盤點

## 沿用盤點

這個模組沒有舊服務，不是搬遷；下表是設計時曾經可以照搬、被重新判斷後沒有沿用的機制（後台待辦在 3cd 的設計、同類型的其他做法）。沒有 ID 方案、extension、CodeMeta 值域要沿用：模組沒有自己的資料，也就沒有表、schema 與 migration（Will 2026-10-20 決定：沒有自己資料的模組不建 schema）。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 3cd D′1 (c) 與 A7-D8 (c) 的 httpapi 合併層（各來源各自分頁，handler 做 N 路合併、rank、`SeekFor`） | 各來源不能共用一個 SQL 語句 | 不成立：所有來源在同一個資料庫，擁有者可以發布 SQL 函式 | 單一資料庫內由一個查詢 `UNION ALL` 後排序分頁（keyset），不在應用層合併（出處：`docs/plans/2026-09-29-a3-buyer-seller/step10e-3d-industry-research.md` §B1、§B5；ADR `2026-10-20-admin-review-queue.md`「考慮過的方案」的業界基準） | 不沿用；設計 v3 改成擁有者發布 `pub_*_review_items`，這裡一支 SQL 排序（E4、E5–E7 刪除） |
| 待辦另存一張表（SLA 紀錄）並在每個指令寫入 | 要凍結排序鍵、或讀取推導太慢 | 不成立：推導即時，成員與期限不會與來源狀態分歧；量測見設計 §6.5 | 先推導，慢了或要稽核時再實體化（出處：同上研究文件 §B1、§B2；ADR `2026-10-20-admin-review-queue.md`「考慮過的方案」的業界基準與「終局與觸發」） | 不另存；觸發條件寫在 AGENTS.md「已知限制」與設計 E12 |
| 各來源的 Go 判定式（`Report.AwaitingTriage` 等）當待辦成員 | 規則只有一個家 | 不成立：成員與期限同時要給 SQL 排序，Go 與 SQL 兩份會分歧，而且這五個判定式在正式程式沒有呼叫者（ADR 2026-09-24 D3 修訂第 4 點） | 規則只放在一處，另一端讀它 | 規則在 seller 的 SQL 函式；五個沒有正式呼叫者的 Go 判定式（`Report.AwaitingTriage` 等）已刪除，指令用的 `State()`、`Open()`、`Availability()` 照舊，seller 的資料集測試拿 SQL 的成員比對它們 |
| 整頁的 `InReadSnapshot`（唯讀 REPEATABLE READ） | 一次請求要跨多個語句讀到同一個快照 | 不成立：一支語句 | 單一語句的快照就夠 | 不用；`List` 不開 transaction（E8） |

## 傳輸層的沿用盤點

舊 gateway 沒有這個 API；下表是分頁與錯誤這兩項該照什麼做的判斷，其他（認證、限流、trace）由 `internal/httpapi` 的共用 interceptor 提供。新的對外 API 照〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 舊 gateway 的 offset 分頁 | 舊 REST 契約 | 不成立：新的 Connect 契約，從未上線 | keyset（Google AIP-158），token 綁篩選 | token 是 `rpc.TokenFor` 的 keyset，綁請求的 kinds（API 規範六） |
| 舊 gateway 的錯誤碼 | 舊 wire 契約 | 不成立 | `apperr` 轉 Connect code | page_size 負數、token 與 kinds 不符、kinds 含 UNSPECIFIED 或未知值是 INVALID_ARGUMENT；沒有任何種類的讀取權時回空清單 |

## 與舊服務的差異

無：沒有舊服務；這個模組是新的，也沒有資料搬移。

