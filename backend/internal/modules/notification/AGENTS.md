# notification 模組

## 職責

站內通知的收件匣、偏好、裝置 token、投遞紀錄與通知型別註冊表，取代 `services/notification-service`（遷移計畫 A8）。來源模組在自己的 transaction 內只排一個通知意圖 job；worker 解析收件人、套偏好、寫收件匣與投遞紀錄，每筆新投遞紀錄同 transaction 排 `notification.deliver` job（不送出）。已建：資料模型、註冊表、意圖管線、保留期與個資清除、公告、渠道 job（`internal/deliver`）、suppression webhook（`internal/suppress`）、token 清除與內容側型別（A8-3b）。行為規格的唯一來源是 `backend/docs/plans/2026-11-23-a8-notification.md`（D0–D18）與 ADR `docs/adr/2026-10-05-notification-module.md`。

## 擁有的資料

schema `notification`（`db/schema.sql`）：`intents`（意圖，冪等鍵 UNIQUE）、`inbox_items`（收件匣列）、`deliveries`（推播與 email 的投遞紀錄）、`device_tokens`、`preference_overrides`、`announcements`（後台公告）；`email_suppressions`（只存位址 HMAC）。

## 發布的讀取介面

- `Watch`：訂閱 `(帳號, 受眾)` 的訊號串流（STORE 帶現任店）。
- `Run(ctx)`：LISTEN 連線，只有 serve 跑。
- `PublishSellerWorkspace(ctx, storeID, change)`：在呼叫端的 transaction 內呼叫（D19）。

## 模組內部結構

- 根套件只保留 `Module`、`New`、`Migrations`、各來源事件的訂閱者入口（`On*`）、治理與交易側的訂閱登記 `SubscribeGovernance`、`AddWorkers` 與 `PeriodicJobs`；組裝在 `cmd/hawkr/notification.go`（serve 與 worker 都建並訂閱）。
- `internal/intake`：意圖管線與 fan-out；`subscribers.go`、`subscribers_content.go` 的同步訂閱者只呼叫 `Accept`（插 `intents` 列並排 `notification.process_intent` job），`Process`（worker）解析收件人、鎖收件人、略過封鎖與自己、套偏好、寫收件匣與投遞紀錄；不變量見 `internal/intake/AGENTS.md`。
- `internal/stream`：串流 hub；不變量見 `internal/stream/AGENTS.md`。
- `internal/purge`：保留期清除與個資清除；不變量見 `internal/purge/AGENTS.md`。
- `internal/announce`：後台公告與派送 job 鏈；見其 `AGENTS.md`。
- `internal/model`：受眾、渠道、偏好分類（D4）、呈現分類（D6）與「不可關」規則。
- `internal/registry`：型別註冊表。每個型別一個 `Definition[P]`（typed 參數、受眾、分類、渠道、冪等鍵、聚合鍵、actor、文案、深連結、圖片），`Define` 變成 `Entry`。
- `internal/text`：zh-TW、en 文案（`make uitext` 生成 `text_gen.go`）；`internal/store` 是 sqlc 套件（`db/queries/intake.sql`、`retention.sql`）。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- 偏好分類是 D4 的 14 個加 D18 的 `ANNOUNCEMENT`，與 proto 同步；交易必要的收件匣、email 不可關，公告無 email、收件匣不可關、推播可關（`model.Delivers`），店家 `SHIPMENT_ANOMALY` 推播亦然，規則只在 `model.Disableable`；受眾分類見 `model.CategoriesFor`。守門：`TestDisableableFollowsD4`、`TestCategoryGoldenMatchesRegistry`。
- 參數快照只存 ID 與事件事實，不存第三方的名稱、頭像或聯絡方式；唯一例外是 `product_restocked` 的商品名稱快照（公開目錄資料，D28，守門測試明列白名單）。守門：`TestParamsDeclareNoThirdPartyNameFields`。
- 收件人是 `(account_id, 受眾, store_id)`，BUYER 沒有 store_id、STORE 一定有；投遞冪等鍵是 (意圖, 帳號, 受眾, store_id, 渠道)（NULLS NOT DISTINCT），未讀聚合鍵也含 store_id，同帳號先後的兩間店是兩個收件人。意圖與收件匣列的連結在 `inbox_item_intents`（意圖被清除只刪連結，列不跟著刪）；`deliveries.intent_id` 是 `ON DELETE RESTRICT`。守門：`inbox_items_store_check`、`deliveries_recipient_channel_key`、`inbox_items_unread_aggregation`。
- 型別註冊表與文案的不變量（冪等鍵來源、分類、聚合鍵、zh-TW 與 en 的固定輸出）見 `internal/registry/AGENTS.md`。
- 已讀、全部已讀、刪除同 transaction `pg_notify`，同收件人的串流訂閱者收到新未讀數。守門：`TestInboxReadAndDeleteSignalTheRecipientsOtherSubscribers`。
- 依賴方向：notification 依賴來源模組的公開事件型別與讀取 API，來源模組不 import notification。守門：`internal/arch` 的依賴表。

- 意圖冪等鍵＝型別名稱＋事件來源身分（`intents` UNIQUE 加 `ON CONFLICT`）；同步訂閱者只准呼叫 `intake.Accept`，`Accept` 只准插意圖列與排 job（token 刪除訂閱者登記在 `tokenPurgeSubscribers`）。守門：`TestSameEventTwiceMakesOneIntentAndOneInboxRow`、`TestSynchronousSubscribersOnlyAccept`。
- worker 的鎖順序是 identity 帳號列、買家列或店家列、notification 的列（`platform/lockorder`）；已刪除、關閉、退場的收件人不寫。守門：`TestIntentProcessedAfterBuyerDeletedOrStoreRetiredCreatesNothing`。
- 保留期與個資清除（180 天硬刪、三個刪除事件各清自己範圍內的收件人）的規則與守門見 `internal/purge/AGENTS.md`。
- 投遞紀錄是推播與 email 的唯一帳本：渠道 job 把 `PENDING` 定案為 `SENT` 或 `FAILED_PERMANENT`；規則見 `CARRYOVER.md`「渠道 job 與 token」。

## 不負責

- 事件的發布與停權、解除停權的流程：`accountsuspension`（事件 `Suspended`、`Unsuspended`）。
- 治理與交易側事件的發布與來源模組的流程（seller 措施與 KYC、moderation、social、review、im、payment 補件期限掃描、delegation、procurement）：各擁有模組；notification 只訂閱、登記意圖。
- 還沒搬進單體的服務（Java、invoice、ai）的通知型別：隨各發送端搬遷時定義，不經 `internal/legacy`（D13）。

## 決策紀錄

沿用自 notification-service 的機制、決定與差異在同目錄的 `CARRYOVER.md`；改動或移除前先讀那一列。ADR：`docs/adr/2026-10-05-notification-module.md`。
