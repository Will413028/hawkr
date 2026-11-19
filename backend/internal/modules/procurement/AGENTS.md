# procurement 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/procurement.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

買家採購（RFQ）與賣家報價；採購的另一半（賣家回報價）也在這裡。服務買家與賣家。事件 `procurement.*` 由 notification 訂閱（A9-D1 保留的那條）。來源是 ai-service，搬遷計畫 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）；依賴：ordering、catalog、buyer、seller；不依賴 notification（notification 依賴 procurement，方向見 `internal/arch/modules.go`）。目前是骨架，還沒有實作。行為規格的唯一來源是 `backend/docs/modules/procurement.md`。

## 擁有的資料

schema `procurement`（`db/schema.sql`）：尚未建立表（A9-3 以後隨實作建立）。讀取模型：無。

## 發布的讀取介面

無。<!-- 每個 `pub_*` 一行：形狀與用途（契約在 `db/pub.sql`）。 -->

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與錯誤。
- `internal/model` 放對外型別；`internal/feature` 是第一個子領域，實作時改成業務名稱；`internal/store` 是 sqlc 的查詢套件。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

模組只 import 依賴表允許的模組根套件：`internal/arch` 的依賴方向測試守門。其餘不變量隨實作補上。

## 不負責

買家助理（`assistant`）、AI 修圖（`imageai`）、站外業者入口（A9-D1 已刪除，不重做）。

## 決策紀錄

從 ai-service 沿用的每個機制、當時的決定與理由，以及與 ai-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。<!-- 相關 ADR 列在這一行後面。 -->
