# imageai 模組

<!-- 只放改這個模組前必須知道的規則，上限 6,000 bytes（根目錄 `.agentfile-lint.toml`）。行為規格與已知限制寫在 `backend/docs/modules/imageai.md`，理由寫在 ADR，這裡只指過去。 -->

## 職責

賣家的 AI 修圖、修圖配額與後台風格管理。服務 website 與 seller-app 的賣家與 admin 的風格管理。來源是 ai-service，搬遷計畫 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）；依賴：seller、media。目前有資料表、圖片模型、配額與同時修圖上限（A9-4a），use case 與 handler 隨 A9-4b、A9-4c。行為規格的唯一來源是 `backend/docs/modules/imageai.md`。

## 擁有的資料

schema `imageai`：`styles`（風格，UUIDv7）、`quotas`（個別額度）、`usage`（每店每週計數器）；沒有預約表（A9-D9）。登記在 `cmd/hawkr/modules.go` 的 `modules()`。讀取模型：無。

## 發布的讀取介面

無。<!-- 每個 `pub_*` 一行：形狀與用途（契約在 `db/pub.sql`）。 -->

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名、事件與錯誤。
- `internal/model` 放型別與常數；`internal/quota` 是額度的扣、退與店家退場清除；`internal/limit` 是同時修圖的計數 semaphore；`internal/store` 是 sqlc 的查詢套件。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`；增加 use case 或子領域時同步修改圖。各子領域在自己套件宣告最小的 `Deps`，由根套件逐欄組裝。

## 不變量

模組只 import 依賴表允許的模組根套件：`internal/arch` 的依賴方向測試守門。

- 扣額度只有一個條件 UPDATE（`WHERE used < limit`），外部呼叫在額度的 transaction 之外；失敗退回，扣後程序終止不退是已知限制（A9-D38）。
- 每個用途恆有一筆預設風格（migration seed）；`prompt` 1 到 500 字元。
- 模型只經 `internal/platform/llm` 建構；模組內不得出現 Gemini 端點字串（`internal/arch` 守門）。
- 預設額度 10、後台上限 1000 是模組常數，同時修圖上限讀 `IMAGEAI_MAX_CONCURRENT`；數字與理由在 `backend/docs/modules/imageai.md`。

## 不負責

採購（`procurement`）、買家助理（`assistant`）；圖檔的儲存與引用屬 media。

## 決策紀錄

從 ai-service 沿用的每個機制、當時的決定與理由，以及與 ai-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。<!-- 相關 ADR 列在這一行後面。 -->
