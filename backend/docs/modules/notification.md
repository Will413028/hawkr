# notification 模組行為規格

本文是單體 `notification` 模組行為的唯一來源。範圍：通知型別、收件匣、偏好、裝置 token、推播與 email 的投遞、後台公告、串流訊號、保留期與個資清除。決策與演進見 ADR `docs/adr/2026-10-05-notification-module.md`、遷移計畫 `backend/docs/plans/2026-11-23-a8-notification.md`（D0–D30）；機制的沿用盤點在 `internal/modules/notification/CARRYOVER.md`，維運（重送、未觸達查詢）在同目錄 `RUNBOOK.md`，不變量在各 `AGENTS.md`。

## 1. 範圍與公開介面

- 資料在 schema `notification`（`db/schema.sql`）：`intents`、`inbox_items`（與連結表 `inbox_item_intents`）、`deliveries`、`device_tokens`、`preference_overrides`、`announcements`、`email_suppressions`。
- 對外 API 是 Connect：`hawkr.notification.v1alpha1.NotificationAppService`（收件匣列表、未讀數、已讀、全部已讀、批次刪除、偏好讀寫、裝置 token 註冊與刪除、`WatchNotifications` 串流）與後台 `hawkr.admin.notification.v1alpha1.NotificationAdminService`（公告建立與列表、重送投遞、未觸達查詢）。代行者不能呼叫（D9）。
- 模組本體只回報來源事件，不被來源模組 import：依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。單體內的發布介面只有 `PublishSellerWorkspace(ctx, storeID, change)`（D19，給之後搬進單體的 order、aftersales 在自己的 transaction 內呼叫）。
- 只接單體模組的事件。還沒搬進單體的服務（Java、invoice、ai）的通知型別隨發送端搬遷時定義，不經 `internal/legacy` 轉接（D13）；到那時為止，那些事件沒有通知。

## 2. 通知型別

- 型別定義在程式碼（`internal/registry`）：每個型別宣告 typed 參數、受眾（BUYER 或 STORE，一個型別一個受眾）、偏好分類、呈現分類、渠道、冪等鍵來源、可選的聚合鍵、actor 欄位、zh-TW／en 文案、深連結與可選的推播圖片。型別沒有資料表，文案是編譯期檢查的訊息目錄（`internal/text`，`make uitext` 生成）。
- 註冊表自檢（`internal/registry` 測試）強制：每個型別有冪等鍵來源、分類在偏好分類清單、呈現分類在 D6 清單、聚合鍵只在宣告可聚合的型別、參數不含第三方名稱欄位。每個型別有 zh-TW、en 固定輸出的比對測試。
- 偏好分類（D4、D18、D29）：交易必要（`ORDER`、`SHIPMENT`、`AFTERSALES`、`PAYOUT`、`ACCOUNT`，店家另有 `SHIPMENT_ANOMALY`）收件匣與 email 不可關；互動（`CHAT`、`REVIEW`、`COMMENT`、`LIKE`、`FOLLOW`、`FOLLOWED_STORE_POST`、`PROCUREMENT`）可關；行銷（`COUPON`、`RESTOCK`）預設開、可關；`ANNOUNCEMENT` 收件匣不可關、推播可關、不寄 email。規則在 `model.Delivers`、`model.Disableable`，與 proto 同步。
- 呈現分類是 10 個 enum（D6），不是每型別一個值；推播與串流訊號另帶狀態重讀提示（`ORDER`、`AFTER_SALE`、`SELLER_STATUS`，可為空）。
- 深連結是 website 相對路徑加語系前綴（D20），沒有路由的目的地不產生連結。路徑表在 `backend/docs/plans/2026-11-23-a8-notification/deep-links.md`。

## 3. 意圖管線

- 來源模組的 transaction 內，同步訂閱者只呼叫 `intake.Accept`：插一列 `intents`（冪等鍵 = 型別名稱加事件來源身分，UNIQUE 加 `ON CONFLICT`）並排 `notification.process_intent` job。唯一例外是登記在 `tokenPurgeSubscribers` 的 token 清除訂閱者（停權與刪帳號時同 transaction 刪 token）。
- worker（`Process`）解析收件人 `(account_id, 受眾, store_id)`、鎖收件人（順序：identity 帳號列、買家或店家列、notification 的列，`platform/lockorder`）、略過已刪除、關閉或退場的收件人、略過自己（以帳號比對）與封鎖雙方（買家對買家、買家對店雙向）、套偏好，然後寫收件匣（有聚合鍵時未讀列 upsert 合併）與每渠道投遞紀錄，並為每筆新投遞紀錄在同 transaction 排 `notification.deliver` job。收件人需要展開的型別（追蹤者、補貨）走 `notification.fanout_intent` 分頁，每位收件人各自的意圖與冪等鍵。
- 收件匣不是渠道：收件匣列本身就是「已進收件匣」的紀錄。投遞紀錄只記推播與 email，是觸達與否的唯一來源。
- 收件匣只存型別、參數快照與聚合鍵，讀取時依請求語系組字；參數只存 ID 與事件事實，不存第三方的名稱、頭像或聯絡方式（例外：`product_restocked` 的商品名稱快照，公開目錄資料，D28）。

## 4. 渠道投遞

- `notification.deliver` 把投遞從 `PENDING` 定案為 `SENT` 或 `FAILED_PERMANENT`（帶 `failure_reason`）。送出前在同 transaction 以同一組鎖確認收件人未刪除未退場，否則記永久失敗、不寫送出文字；送出時依帳號 `preferred_locale` 組字。
- 推播（FCM）：BUYER 受眾只推 buyer App、STORE 只推 seller App；只在 Unregistered、SenderIDMismatch 停用 token，授權錯誤告警，每批最多 500 個 token；可見通知（APNs alert）。行銷類推播在台北 22:00–09:00 延到 09:00 後送出，券在靜音時段結束前就到期時例外（D26）。
- email（Resend）：只寄 `identity.ContactEmail` 的已驗證信箱，不寄停寄名單（`email_suppressions` 只存位址 HMAC-SHA256，永久保留；hard bounce 可由後台解除、complaint 永不解除，D22）；退信與投訴 webhook 先寫 `platform/inbox` 再寫名單。確認信的地址不放在任何 job 參數，投遞時依帳號經 identity 讀。
- adapter 對外只回暫時性或永久性錯誤；暫時性由 River 退避重試，用盡記 `retries_exhausted`。FCM 與 Resend 各有本機替身。
- 裝置 token：以 token 為主鍵、綁一個帳號與一支 App，記 `last_registered_at`，App 每次啟動註冊；登入改綁、登出刪、帳號停權與刪帳號同 transaction 刪（店家停權不動），每日 job 刪 60 天未更新的 token。
- 重送與未觸達查詢見 `RUNBOOK.md`。

## 5. 後台公告

公告是資料，收件匣與推播分開 job，沒有裝置 token 的帳號也收得到收件匣（D17、D18）。單一 `title`／`body`，一律英文，不分語系。公告以註冊的扇出型別 `announcement` 進收件匣，推播與收件匣各自分頁派送；派送中重試可能重建使用者剛刪的公告列（記於 CARRYOVER）。

## 6. 串流

- `WatchNotifications` 是 server streaming，只傳失效訊號（未讀數、新建或聚合更新的收件匣 ID 最多 20 個，超過標 truncated），不傳通知內容；client 收到後重拉列表，重連後先拉一次（D11、D21）。賣家工作區事件與通知共用同一條串流（D19）。
- 訂閱鍵 `(account_id, 受眾)`，每個 serve 行程每帳號最多 8 條（`stream.MaxPerAccount`，不跨副本），超過回 resource_exhausted。
- serve 與 worker 是不同行程：寫入端在自己的 transaction 內 `pg_notify`（payload 只有業務鍵），serve 以一條專用 LISTEN 連線（`Run`，只有 serve 跑）扇出給本行程訂閱者；LISTEN 重連後對所有本地訂閱者發完整重讀訊號。rollback 不送。

## 7. 保留期與個資清除

- 收件匣、投遞紀錄與意圖都是 180 天硬刪（D2、D15）：清除 job 先刪過期投遞，再刪過期且已無投遞的意圖（`deliveries.intent_id` 是 `ON DELETE RESTRICT`）。
- 訂閱 `identity.AccountDeleted`、`buyer.BuyerDeleted`、`seller.StoreRetired`，各排 job 清除該收件人範圍內的收件匣、偏好、投遞紀錄的送出文字與意圖參數。刪除之後才到的意圖不建收件匣。
- 投遞紀錄的「實際送出文字」含送出時的事實，受 180 天上限約束，只有收件人自己刪帳號時提前清（D16）。

## 8. 不負責

- 事件的發布與來源流程：各擁有模組（accountsuspension、seller、moderation、social、review、im、payment、delegation、procurement、coupon、buyer）。
- 行銷 email、SMS、LINE、Web Push；後台模板編輯、預覽與試發；時間窗摘要；`product.viewed`；收藏通知。
- 延後項與觸發見遷移計畫 `2026-11-23-a8-notification.md` 的「延後」。
