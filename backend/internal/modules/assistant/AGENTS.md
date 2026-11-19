# assistant 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/assistant.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

買家助理的對話：買家與助理的跨輪對話、串流回應與續傳。服務 buyer-app 的買家。來源是 ai-service，搬遷計畫 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）；依賴：ordering、catalog、buyer。目前是骨架，還沒有實作。行為規格的唯一來源是 `backend/docs/modules/assistant.md`。

## 擁有的資料

目前無 schema，沒有 `db/` 與 migration，登記在 `cmd/hawkr/modules.go` 的 `withoutData`；第一次要存自己的資料時建 schema 與 migration，再移到 `modules()`。讀取模型：無。

## 發布的讀取介面

無。<!-- 每個 `pub_*` 一行：形狀與用途（契約在 `db/pub.sql`）。 -->

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與錯誤。
- `internal/model` 放對外型別；`internal/feature` 是第一個子領域，實作時改成業務名稱；`internal/store` 是 sqlc 的查詢套件。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

模組只 import 依賴表允許的模組根套件：`internal/arch` 的依賴方向測試守門。其餘不變量隨實作補上。

## 不負責

採購（`procurement`）、修圖（`imageai`）；賣家助理與刊登助理已依 A9-D2 刪除。

## 決策紀錄

從 ai-service 沿用的每個機制、當時的決定與理由，以及與 ai-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。<!-- 相關 ADR 列在這一行後面。 -->
