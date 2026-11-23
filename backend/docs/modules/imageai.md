# imageai 模組行為規格

本文是單體 `imageai` 模組行為的唯一來源：賣家的 AI 修圖、修圖配額與後台風格管理。A9-4a 完成資料表、圖片模型、配額與併發上限；use case 與 handler 隨 A9-4b、A9-4c。

## 1. 範圍與公開介面

- 來源是 ai-service；搬遷計畫與決定見 `backend/docs/plans/2026-11-19-a9-agent.md`（邊界 A9-D3）。
- 依賴：seller、media。
- 目前的公開方法：`ConsumeQuota`、`RefundQuota`、`AcquireTransformSlot`、`OnStoreRetired`；`ReadConfig` 讀設定。

## 2. 資料

schema `imageai`，三張表，沒有預約表（A9-D9）：

| 表 | 內容 |
|---|---|
| `styles` | 風格。主鍵 UUIDv7；`(category, name)` 唯一；每個用途最多一個預設（partial unique index）；`prompt` 1 到 500 字元（以 Unicode 字元計）；`version` 是 etag；縮圖用 `thumbnail_key` 與 `thumbnail_asset_id`，空字串表示沒有 |
| `quotas` | 個別設定的每週額度，`store_id` 為主鍵，`weekly_limit >= 0`，`version` 是 etag |
| `usage` | 每店一列（主鍵 `store_id`）：`period_start` 是計數所屬的週，`used` 計數器 `>= 0`；新的一週由扣額度的 UPSERT 重設 |

- migration seed 每個用途（`PRODUCT`、`POST`）一筆中性預設風格（A9-D39）；伺服器沒有寫死的後備提示詞。預設風格不可刪（`failed_precondition`，不新增 ErrorReason），`default_style` 只能設成 true、設成 false 回 `invalid_argument`，換預設只能把另一筆設成 true。
- `store_id` 是 bigint 外部鍵，沒有跨 schema FK。店家退場（`seller.StoreRetired`）時，同 transaction 刪該店的 `quotas` 與 `usage` 列，冪等。

## 3. 額度

- 週期是週一 00:00 Asia/Taipei 起算的一週，`period_start` 存該時刻；重置時間是下週一 00:00。
- 預設每週 10 次（`DefaultWeeklyLimit`），後台能設定的上限是 1000（`MaxWeeklyLimit`）。兩個數字是模組常數，不是 config；`quotas.weekly_limit` 的上限由寫入端檢查，資料庫只擋負數。
- 上限的取法：該店有 `quotas` 列就用它，否則用預設。
- 扣額度是單一條件 UPSERT：`INSERT ... ON CONFLICT (store_id) DO UPDATE ... WHERE period_start < 本週 OR (period_start = 本週 AND used < limit)`，列屬於較舊的一週時 `used` 重設為 1，回 0 列就是用完。上限為 0 時不建用量列。
- 退額度是 `used = used - 1 WHERE period_start = 該週 AND used > 0`，不會變負，回 0 列回 `ErrNothingToRefund`；走連線池（不加入呼叫端 transaction），使用 `context.WithoutCancel` 加 5 秒 deadline，請求被中斷或呼叫端回滾後仍會退。
- 已知限制（A9-D38）：扣額度之後、退額度之前伺服器程序終止，該次額度不退回。不建預約表或 job 收尾。

## 4. 圖片模型

- 修圖的模型由 `internal/platform/llm` 的 `ImageAI` 角色建構，預設 `gemini-2.5-flash-image`，可用 `LLM_IMAGEAI_MODEL` 改成登記表內的圖片模型；金鑰共用 `GEMINI_API_KEY`。
- 回傳的是窄介面 `llm.ImageGenerator`：一張原圖加提示詞進、圖片加 `FinishReason`、`BlockReason` 出。ADK 的 model 不用：prompt 被擋時它丟掉 `BlockReason`（spike 結果見 `backend/docs/plans/2026-11-19-a9-agent/4-inventory.md` §3.1）。
- 每次呼叫有 60 秒 deadline。模組內不得出現 Gemini 的端點字串（`internal/arch` 的守門測試）。

## 5. 同時修圖上限

- 每個 pod 一個計數 semaphore，`AcquireTransformSlot` 不等待，滿了回 `ok=false`；跨 pod 不協調。
- 上限 N 由環境變數 `IMAGEAI_MAX_CONCURRENT` 讀（`ReadConfig`），預設 21，至少 1。21 沿用舊版 byte 預算推得的數字（256 MiB 除以 2 MiB 乘 6），沒有量測過；以記憶體量測校正前不當成容量保證。
