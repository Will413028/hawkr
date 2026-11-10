# payment 模組行為規格

payment 是收款與付款的模組（ADR `docs/adr/2026-09-24-go-modular-monolith.md` 的依賴表葉節點）。現階段只擁有收款帳戶（Stripe 計畫 D28，步驟 7d′）；付款、退款與撥款是 B2、B3、B8–B10 的範圍，進來時擴充這份規格。

## 1. 範圍與公開面

- 資料：schema `payment`，表 `accounts`（收款帳戶，`db/schema.sql`）；`db/objects.sql` 有發布的 `pub_review_item`／`pub_review_items(at)`（後台待辦）。
- 子領域套件：`internal/account`（port、Stripe 與 fake adapter、帳戶快取、建立／同步／輪詢／刪除的 job、收款事件的 inbox handler）、`internal/model`（對外型別與 `PayoutAccountChanged`）、`internal/store`（sqlc）。
- 公開 API（根套件 `payment`）：
  - 擁有者用（seller 呼叫）：`RequestAccount`（擁有者核准的 transaction 內）、`Account`、`CreateOnboardingSession`、`SyncAccountUnlessFresh`（賣家觸發的 transaction 外同步重讀，§2）、`Capabilities`、`Configured`、`ForgetOwners`（擁有者個資清理的 transaction 內）。呼叫方先確認呼叫者是擁有者（Stripe 計畫 D30：賣家端的 `GetMyPaymentAccount`、`CreatePaymentOnboardingSession` 留在 `SellerAppService`）。
  - 後台：`RefreshAccount`、`RetryAccountCreate`，對外是 `hawkr.admin.payment.v1alpha1.PaymentAdminService`（`GetPaymentAccount`、`RefreshPaymentAccount`、`RetryPaymentAccountCreate`；權限 `payment.accounts.get`、`.refresh`、`.retry_create`）。
  - webhook：`ReceiveEvent`（HTTP 入口 `POST /webhooks/stripe`，`internal/httpapi/payment`）。
  - 事件：`PayoutAccountChanged`（§5）。
- 擁有者鍵是 `store_id`（seller 的店家；不建跨 schema FK）。出現第二種擁有者時再通用化（D28）。payment 是葉節點，不讀 seller：擁有者是否營運中經組裝點注入的 `Owners`（cmd/hawkr 以 `seller.Module.Store` 實作）。
- 不在本模組：開通閘門（seller 的 `store_requirement_flags`，讀 seller 自存的副本）、防詐 hash（seller 的 `account_identity_hashes`）、可曝光公告（seller）、webhook inbox 的表與派送（platform 的 `internal/platform/inbox`，D29）。

## 2. 收款帳戶

- 一個擁有者一列，關店再開沿用（D19）；列只存狀態與建立外部帳戶要用的聯絡信箱（D23）。外部呼叫（建立帳戶、重讀帳戶、開通 session）不在 transaction 內（單體規則 5）。
- 建立：擁有者核准的 transaction 內 `RequestAccount` 只 INSERT「建立中」的列（`external_account_id` NULL、`NOT_CREATED`；`ON CONFLICT DO NOTHING`）並排入以 `store_id` 為唯一鍵的建立 job（`payment.account_ensure`）；已有列時不改它（聯絡信箱維持第一次的快照），還沒有外部 ID、也沒有進入建立失敗時重新排入建立 job。沒有聯絡信箱回 `FAILED_PRECONDITION`，核准整筆不寫入。沒有設定 provider 時什麼都不做。
- 建立 job：讀本地列，`Owners` 回答擁有者不在營運中就略過；在 transaction 外以列上保存的 UUID（`create_idempotency_key`，寫列時產生的 v4）為冪等鍵呼叫 provider，收件方式取列上的 `requirement_collection`；再於 transaction 內填上外部 ID（狀態先記 `ACTION_REQUIRED`）並排入同步 job。暫時性失敗由 River 退避重試（最多 8 次）；最後一次或永久失敗記 `create_failed_at`，進後台待辦 `PAYMENT_ACCOUNT_CREATE_FAILED`（§6），管理員「重試建立」清空並重排。
- 同步 job（`payment.account_sync`，唯一鍵 `store_id`）：在 transaction 外重讀外部帳戶，在 transaction 內鎖帳戶列、整列覆寫快照（沒有觀測歷史，重讀即最新）、發布 `PayoutAccountChanged`（§5），並經 platform inbox 把這個外部帳戶在讀取開始前收到的事件整批標記已處理（`MarkRouteProcessed`，處理者 `payment.account_sync`）；之後仍有未處理事件（讀取期間到的）時回 `again`，worker 以 snooze 再跑一輪。外部帳戶不存在是永久失敗（job 取消）。
- 同步與同步並行（D32）：`SyncAccountUnlessFresh` 給 seller 的 `RefreshMyPaymentAccount` 在 job 之外同步呼叫，所以「一個帳戶同一時間一個同步」只剩 River 唯一鍵保證。寫快照時只有這次讀取的開始時間不早於已存的 `fetched_at`（NULL 一律寫）才寫；被擋下的同步（較晚開始的讀取已經先寫入）不寫快取、不標記 inbox 事件、不發布 `PayoutAccountChanged`，`again` 為 false（較晚開始的那一次涵蓋了在這次開始之前收到的事件）。必要條件是存在 job 以外的同步呼叫，那個呼叫拿掉時條件一起拿掉。同步呼叫方（RPC）忽略 `again`。守門：`TestAnOlderReadNeverOverwritesANewerCommittedRead`、`TestARejectedSyncDoesNotMarkEventsOrPublish`。
- 賣家觸發的重讀間隔（D33）：`SyncAccountUnlessFresh` 在同一帳戶上次讀取（`fetched_at`，讀取開始時間）距現在不到 `ReadDebounce`（10 秒，具名常數，時鐘由組裝點注入）時不呼叫 provider、直接成功（快取就是結果）；滿 10 秒才走 `Sync`。必要條件：外部讀取由賣家觸發，頻率不受 job 唯一鍵約束；那個呼叫拿掉時間隔一起拿掉。沒有 provider、沒有帳戶列或還沒有外部 ID 時什麼都不做；重讀失敗是明確動作的失敗：暫時性回 `Unavailable`，外部帳戶不存在（`ErrPermanent`）回 `FailedPrecondition`。守門：`TestReadIsFreshOnlyInsideTheDebounceWindow`、`TestSellerTriggeredRereadIsDebouncedAndFailsLoudly`。
- 狀態推導（`account.StatusOf`，D13）：沒有外部 ID＝`NOT_CREATED`；`active`＝`ENABLED`；`rejected`、`unsupported`＝`DISABLED`；任何一筆細節的行動方是賣家（含 `pending`）＝`ACTION_REQUIRED`；`pending`＝`UNDER_REVIEW`；`restricted` 加平台處理或無解法的細節＝`DISABLED`；不認得的狀態或行動方、`restricted` 但沒有細節＝`UNDER_REVIEW` 並記警示。細節多筆時賣家能行動的優先，其次 `DISABLED`。`status_reason` 是導出狀態的那一筆細節的 provider 原因碼（只給後台）。`transfers_active` 等於 `ENABLED`；`payouts_active` 只顯示。守門：`TestStatusOfFollowsTheDecisionTable`。
- 開通 session：外部帳戶建立之前、被停用（`DISABLED`）時回 `FAILED_PRECONDITION`；介面由列上的收件方式決定（託管連結或內嵌元件）。
- 刪除：擁有者個資清理的 transaction 內 `ForgetOwners` 只排入刪除 job（`payment.account_purge`），job 在自己的 transaction 刪除那些擁有者的列（含聯絡信箱）。外部帳戶不處理。

## 3. 收款事件與 inbox（D29）

- 入口驗簽（`Provider.ParseEvent`）後在一個 transaction 內以 `inbox.Record` 寫進 platform 的 `inbox.events`（`(provider, event_id)` 唯一；路由鍵是外部帳戶 ID；原始 payload 原樣保存，保存期 `EventRetention` 30 天）並排入派送 job，commit 之後才回 2xx。重送的事件不寫入、不派送；驗過簽章但不處理的事件回 2xx、不寫 inbox；簽章或格式不合回 400；沒有設定 provider 回 503。
- 派送（platform 的 `inbox.dispatch` job）呼叫收款的 handler：外部帳戶對得到本地帳戶時只排入同步 job，**不標記已處理**——事件只是「重讀帳戶」的訊號，由涵蓋它的同步標記；同步被執行中的 job 合併掉時，事件留在 inbox 讓那一輪回 `again`。對不到本地帳戶的事件只留紀錄、直接標記已處理（處理者 `payment.account_event`）。
- 補掃分工：platform 的補掃只重派派送 job 被丟棄或用完重試、仍未派送成功也未處理的列；payment 的 15 分鐘輪詢（`payment.account_sweep`）排 UNDER_REVIEW、ACTION_REQUIRED、原因碼不是 `rejected*`／`unsupported*` 的 DISABLED 帳戶的同步（Stripe 可以解除 restricted，掉了 webhook 時只有輪詢補得到）、inbox 有超過 1 分鐘未處理事件的帳戶的同步（`inbox.StaleRoutes`），以及閒置 10 分鐘的建立列（擁有者營運中才排）。
- payload 與 `PayoutAccountChanged` 的明文 fingerprint 不寫 log。

## 4. 後台

- `GetPaymentAccount` 讀擁有者的帳戶（沒有回 `NOT_FOUND`）；`RefreshPaymentAccount` 只排同步 job（沒有 provider 或還沒有外部 ID 回 `FAILED_PRECONDITION`）；`RetryPaymentAccountCreate` 只有建立失敗的列可以重試（其他狀態 `FAILED_PRECONDITION`）。

## 5. 事件

- `PayoutAccountChanged{OccurredAt, StoreID, Provider, TransfersActive, BankFingerprints}`：同步 job 寫快照的 transaction 內每次都發布（fingerprint 可能在轉帳能力不變時改變）。seller 的同 transaction 訂閱者把 `TransfersActive` 複製到開通閘門的副本、把 fingerprint 寫成防詐 hash（D12），轉帳能力有變時重新公告。訂閱者失敗時整筆同步 rollback，River 重試。

## 6. 發布的讀取介面

- `payment.pub_review_items(at)`：後台待辦的 payment 部分（reviewqueue 讀，形狀在 `db/pub.sql`）。現在只有 `PAYMENT_ACCOUNT_CREATE_FAILED`：`create_failed_at` 有值的帳戶，item_id 是店家 ID 的文字，開始是失敗的時刻，期限 24 小時。只有店家 ID，店名由後台入口（`internal/httpapi/reviewqueue`）經 seller 的店家卡片補上。

## 7. 鎖順序

- 全域順序見單體規則 4 與 `platform/lockorder` 的 `Modules`（payment 在 seller 之前）。payment 自己的寫入只鎖帳戶列（`LockPaymentAccount`）；同步在帳戶列之後發事件，seller 的訂閱者再鎖 seller 的列。擁有者（seller）的 transaction 只對 payment 做 INSERT ON CONFLICT DO NOTHING 與排 job，不等 payment 的既有列；`inbox` schema 是 platform 的，不在模組順序內。

## 8. 已知限制

- 同一家店再次核准時不更新還沒建立外部帳戶的列的聯絡信箱（核准不能寫 payment 的既有列，§2）；建立後改信箱本來就延後（D23）。
- `PAYMENT_ACCOUNT_CREATE_FAILED` 不再排除已關閉的店家（payment 不讀 seller 的店家狀態）：關閉店家的待辦由管理員「重試建立」清掉，建立 job 會因擁有者不在營運中而略過。
- 本機種子（`dev-seed`）以 fake provider 走正式的建立與同步；店家已有別的 provider 的帳戶時種子不改它，只印出提醒。
