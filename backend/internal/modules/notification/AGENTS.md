# notification 模組

## 職責

站內通知的收件匣、偏好、裝置 token、投遞紀錄與通知型別註冊表，取代 `services/notification-service`（遷移計畫 A8）。來源模組在自己的 transaction 內只排一個通知意圖 job；worker 解析收件人、套偏好、寫收件匣與投遞紀錄，每筆新投遞紀錄同 transaction 排 `notification.deliver` job（不送出）。已建：資料模型、型別註冊表、意圖管線、保留期與個資清除、後台公告；其餘對外方法、渠道 adapter 隨之後的步驟加入。行為規格的唯一來源是 `backend/docs/plans/2026-11-23-a8-notification.md`（D0–D18）與 ADR `docs/adr/2026-10-05-notification-module.md`。

## 擁有的資料

schema `notification`（`db/schema.sql`）：`intents`（意圖，冪等鍵 UNIQUE）、`inbox_items`（收件匣列）、`deliveries`（推播與 email 的投遞紀錄）、`device_tokens`、`preference_overrides`、`announcements`（後台公告）；suppression 表隨 A8-3 建。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、`Migrations`、五個訂閱者入口（`On*`）、`AddWorkers` 與 `PeriodicJobs`；組裝在 `cmd/hawkr/notification.go`（serve 與 worker 都建並訂閱）。
- `internal/intake`：意圖管線。`subscribers.go` 的同步訂閱者只呼叫 `Accept`（插 `intents` 列並排 `notification.process_intent` job）；`Process`（worker）在 transaction 外讀意圖、解析收件人、略過 actor 是收件人自己的意圖，再開 transaction：先 `identity.ShareLockAccount` 鎖收件帳號列、確認未刪除（已刪除或不存在就不寫），再依受眾以 `FOR KEY SHARE` 鎖買家列或店家列、已關閉或已退場就略過；略過時同 transaction 把該意圖參數換成 `'{}'`（D16），最後套偏好、寫收件匣與投遞紀錄。
- `internal/purge`：保留期清除與個資清除；不變量見 `internal/purge/AGENTS.md`。
- `internal/announce`：後台公告與派送 job 鏈；見其 `AGENTS.md`。
- `internal/model`：受眾、渠道、偏好分類（D4）、呈現分類（D6）與「不可關」規則。
- `internal/registry`：型別註冊表。每個型別一個 `Definition[P]`（typed 參數、受眾、分類、渠道、冪等鍵、聚合鍵、actor、文案、深連結、圖片），`Define` 變成 `Entry`。
- `internal/text`：zh-TW、en 文案（`make uitext` 生成 `text_gen.go`）；`internal/store` 是 sqlc 套件（`db/queries/intake.sql`、`retention.sql`）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- 偏好分類是 D4 的 14 個加 D18 的 `ANNOUNCEMENT`，與 proto 同步；交易必要的收件匣、email 不可關，公告無 email、收件匣不可關、推播可關（`model.Delivers`），店家 `SHIPMENT_ANOMALY` 推播亦然，規則只在 `model.Disableable`；受眾分類見 `model.CategoriesFor`。守門：`TestDisableableFollowsD4`、`TestCategoryGoldenMatchesRegistry`。
- 每個型別有冪等鍵來源（意圖冪等鍵＝型別名稱＋事件來源身分）、分類在 D4 清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別。守門：`registry.Validate`、`TestEveryRegisteredTypePassesTheSelfCheck`、`TestSelfCheckRejectsBrokenDefinitions`。
- 參數快照只存 ID 與事件事實，不存第三方的名稱、頭像或聯絡方式。守門：`TestParamsDeclareNoThirdPartyNameFields`。
- 每個型別在 zh-TW、en 有固定輸出，en 沒有執行期退回 zh-TW，文字的語系取自 `Rendered.Locale`、各部分不一致即錯誤；冪等鍵與聚合鍵過長時改存型別名稱加雜湊。守門：`TestGoldenRendering`、`TestGoldenIdempotencyKeys`、`TestEveryRenderableValueHasEnglishText`、`TestRenderRejectsMixedLocales`、`TestLongKeysAreHashedToFit`。
- 收件人是 `(account_id, 受眾, store_id)`，BUYER 沒有 store_id、STORE 一定有；投遞冪等鍵是 (意圖, 帳號, 受眾, store_id, 渠道)（NULLS NOT DISTINCT），未讀聚合鍵也含 store_id，同帳號先後的兩間店是兩個收件人。意圖與收件匣列的連結在 `inbox_item_intents`（意圖被清除只刪連結，列不跟著刪）；`deliveries.intent_id` 是 `ON DELETE RESTRICT`。守門：`inbox_items_store_check`、`deliveries_recipient_channel_key`、`inbox_items_unread_aggregation`。
- 依賴方向：notification 依賴來源模組的公開事件型別與讀取 API，來源模組不 import notification。守門：`internal/arch` 的依賴表。

- 意圖冪等鍵＝型別名稱＋事件來源身分（`intents` UNIQUE 加 `ON CONFLICT`）；同步訂閱者只准呼叫 `intake.Accept`，`Accept` 只准插意圖列與排 job（A8-3 的 token 刪除訂閱者登記在 `tokenPurgeSubscribers`）。守門：`TestSameEventTwiceMakesOneIntentAndOneInboxRow`、`TestSynchronousSubscribersOnlyAccept`。
- worker 的鎖順序是 identity 帳號列、買家列或店家列、notification 的列（`platform/lockorder`）；已刪除、關閉、退場的收件人不寫。守門：`TestIntentProcessedAfterBuyerDeletedOrStoreRetiredCreatesNothing`。
- 保留期與個資清除（180 天硬刪、三個刪除事件各清自己範圍內的收件人）的規則與守門見 `internal/purge/AGENTS.md`。
- 投遞紀錄是推播與 email 的唯一帳本，這一步只建 `PENDING` 並排 `notification.deliver` job（job 不送出，A8-3 接 adapter）。見 `CARRYOVER.md`。

## 不負責

- 事件的發布與停權、解除停權的流程：`accountsuspension`（事件 `Suspended`、`Unsuspended`）。
- 還沒搬進單體的服務（Java、invoice、ai）的通知型別：隨各發送端搬遷時定義，不經 `internal/legacy`（D13）。

## 決策紀錄

沿用自 notification-service 的機制、決定與差異在同目錄的 `CARRYOVER.md`；改動或移除前先讀那一列。ADR：`docs/adr/2026-10-05-notification-module.md`。
