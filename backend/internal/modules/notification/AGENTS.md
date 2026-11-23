# notification 模組

## 職責

站內通知的收件匣、偏好、裝置 token、投遞紀錄與通知型別註冊表，取代 `services/notification-service`（遷移計畫 A8）。來源模組在自己的 transaction 內只排一個通知意圖 job；worker 解析收件人、套偏好、寫收件匣與投遞紀錄。範圍是資料模型與型別註冊表；對外方法、訂閱者與 job 隨 A8-1b 之後的步驟加入。行為規格的唯一來源是 `backend/docs/plans/2026-11-23-a8-notification.md`（D0–D16）與 ADR `docs/adr/2026-10-05-notification-module.md`。

## 擁有的資料

schema `notification`（`db/schema.sql`）：`intents`（意圖，冪等鍵 UNIQUE）、`inbox_items`（收件匣列）、`deliveries`（推播與 email 的投遞紀錄）、`device_tokens`、`preference_overrides`。公告表隨 A8-2、suppression 表隨 A8-3 建。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、`Migrations`；A8-1b 起放訂閱者與 job 的組裝。
- `internal/model`：受眾、渠道、偏好分類（D4）、呈現分類（D6）、狀態重讀提示與「不可關」規則。
- `internal/registry`：型別註冊表。每個型別一個 `Definition[P]`（typed 參數、受眾、分類、渠道、冪等鍵、聚合鍵、actor、文案、深連結、圖片），`Define` 變成 `Entry`；沒有資料表、沒有執行期反射，反射只出現在自檢測試。
- `internal/text`：zh-TW、en 文案（`make uitext` 生成 `text_gen.go`）；`internal/store` 是 sqlc 套件（第一支 query 隨 A8-1b）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- 偏好分類只有 D4 的 14 個；交易必要分類的收件匣與 email 不可關、`SHIPMENT_ANOMALY` 的推播也不可關，規則只在 `model.Disableable`。守門：`TestDisableableFollowsD4`。
- 每個型別有冪等鍵來源（意圖冪等鍵＝型別名稱＋事件來源身分）、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別。守門：`registry.Validate`、`TestEveryRegisteredTypePassesTheSelfCheck`、`TestSelfCheckRejectsBrokenDefinitions`。
- 參數快照只存 ID 與事件事實，不存第三方的名稱、頭像或聯絡方式；actor 只存帳號 ID。守門：`TestParamsDeclareNoThirdPartyNameFields`。
- 每個型別在 zh-TW、en 有固定輸出。守門：`TestGoldenRendering`、`TestGoldenIdempotencyKeys`。
- 收件人是 `(account_id, 受眾, store_id)`，BUYER 沒有 store_id、STORE 一定有；投遞冪等鍵是 (意圖, 帳號, 受眾, 渠道)。守門：`inbox_items_store_check`、`deliveries_recipient_channel_key`。
- 依賴方向：notification 依賴來源模組的公開事件型別與讀取 API，來源模組不 import notification。守門：`internal/arch` 的依賴表（notification 列只含已在單體、會訂閱或讀取的模組）。

## 不負責

- 事件的發布與停權、解除停權的流程：`accountsuspension`（事件 `Suspended`、`Unsuspended`）。
- 寄件地址：`identity.ContactEmail`。封鎖關係：buyer、seller 的公開 API。
- 還沒搬進單體的服務（Java、invoice、ai）的通知型別：隨各發送端搬遷時定義，不經 `internal/legacy`（D13）。

## 決策紀錄

從 notification-service 沿用的每個機制、當時的決定與理由，以及與舊服務的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。ADR：`docs/adr/2026-10-05-notification-module.md`（含多語系與 A8 兩次修訂）。
