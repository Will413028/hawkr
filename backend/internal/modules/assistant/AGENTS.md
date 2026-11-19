# assistant 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/assistant.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

買家助理的對話：買家與助理的跨輪對話、串流回應與續傳。服務 buyer-app 的買家。來源是 ai-service，搬遷計畫 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）；依賴：ordering、catalog、buyer。目前只有 session 儲存，agent 與輪次執行在 A9-2b。行為規格的唯一來源是 `backend/docs/modules/assistant.md`。

## 擁有的資料

schema `assistant`（`db/schema.sql`），登記在 `cmd/hawkr/modules.go` 的 `modules()`。四張表是 ADK v2 `session.Service` 的儲存（A9-D5）：`sessions`（session 層 state）、`events`（整個 `session.Event` 的 JSON，partial 不寫、`temp:` key 寫前剝除）、`app_states`、`user_states`；鍵是 ADK 的字串，`user_id` 是買家 ID 的十進位字串（外部鍵、不建 FK）。助理輪次事件不建表，放記憶體環形緩衝（A9-D12）。讀取模型：無。

## 發布的讀取介面

無。<!-- 每個 `pub_*` 一行：形狀與用途（契約在 `db/pub.sql`）。 -->

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與錯誤。
- `internal/model` 放對外型別；`internal/feature` 是第一個子領域，實作時改成業務名稱；`internal/store` 是 sqlc 的查詢套件；`internal/adksession` 以 pgx 實作 ADK 的 `session.Service`。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

模組只 import 依賴表允許的模組根套件：`internal/arch` 的依賴方向測試守門。

- session 的寫入都在 `db.InTx` 內，transaction 內不呼叫 LLM（規則 5）。state delta 以 SQL 的 jsonb `||` 合併，不讀出再寫回；鎖順序 session 列、app 列、user 列（`Create`、`AppendEvent` 同序）。
- `Create` 重複不靠撞唯一鍵（`ON CONFLICT DO NOTHING` 看影響列數），外層 transaction 不會因此 aborted。
- `adksession` 的驗收是 ADK 的 `RunServiceTests`（`session_integration_test.go`，CI 的 Postgres）；`AppendEvent` 寫入內容由純函式 `encodeAppend` 算出，`temp:` 剝除有本機單元測試守。
- 其餘不變量隨實作補上。

## 不負責

採購（`procurement`）、修圖（`imageai`）；賣家助理與刊登助理已依 A9-D2 刪除。

## 決策紀錄

從 ai-service 沿用的每個機制、當時的決定與理由，以及與 ai-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。<!-- 相關 ADR 列在這一行後面。 -->
