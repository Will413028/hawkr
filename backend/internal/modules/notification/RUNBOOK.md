# notification 維運手冊：重送與未觸達查詢

推播與 email 的投遞紀錄（`notification.deliveries`）是觸達與否的唯一來源。River 只保留完成的 job 24 小時，收件匣不是渠道（收件匣列本身就是「已進收件匣」的紀錄），所以判斷「有沒有送到」只看投遞紀錄。投遞紀錄與收件匣、意圖同為 180 天保留期，超過之後無法重送也查不到。

## 狀態與失敗原因

投遞狀態只有 `PENDING`（待送）、`SENT`（已送出）、`FAILED_PERMANENT`（永久失敗，帶 `failure_reason`）。渠道 job 暫時性失敗會由 River 退避重試，最後一次仍失敗才記永久失敗（`retries_exhausted`）。

| 失敗原因 | 意思 | 重送 |
|---|---|---|
| `no_device_token` | 收件人沒有可推播的裝置 | 可重送：收件人之後可能已登入並註冊裝置 |
| `token_unregistered` | 推播服務回報 token 已失效 | 可重送：收件人可能已有新 token |
| `push_rejected` | 推播服務拒絕這則推播 | 可重送：先確認不是請求本身有問題 |
| `no_email_address` | 沒有已驗證的信箱 | 可重送：收件人之後可能已驗證信箱 |
| `email_rejected` | 寄信服務拒絕這封信 | 可重送：先確認不是寄件設定的問題 |
| `retries_exhausted` | 暫時性失敗用盡重試次數 | 可重送：先確認外部服務已恢復 |
| `no_longer_valid` | 送出前重查發現來源事實已不成立，刻意不送 | 永不重送 |
| `email_suppressed` | 位址在停寄名單（硬退信或投訴） | 永不重送 |
| `recipient_gone` | 收件人已刪除或退場 | 永不重送 |
| `intent_missing`、`type_unregistered`、`render_failed`、`channel_unsupported` | 意圖已清除、型別不在、組字失敗、渠道不支援 | 永不重送：重送不會有不同結果 |

可重送的白名單寫在 `internal/reach` 的 `resendable`；新增的失敗原因預設不可重送。

## 什麼時候重送

只在「原因已排除」之後：外部服務故障已恢復（`retries_exhausted`、`push_rejected`、`email_rejected`）、或收件人端的條件已改變（`no_device_token`、`no_email_address`）。原因沒排除就重送只會再失敗一次。行銷類通知過了時效再送沒有意義，重送也不會繞過靜音時段（見下）。

## 怎麼重送

後台 Connect RPC `hawkr.admin.notification.v1alpha1.NotificationAdminService/ResendDeliveries`，需要權限 `notification.deliveries.resend`，每次呼叫（`dry_run` 除外）寫一筆後台稽核紀錄。

請求欄位：

- `fail_time_start`、`fail_time_end`：必填，失敗時間窗 `[start, end)`，以投遞最後一次失敗的時間（`updated_at`）計；`end` 不可在未來，窗長上限 90 天。
- `category`、`channel`（只能 PUSH 或 EMAIL）、`reasons`（只能是可重送的原因）、`account_id`：選填，不給就不限。
- `max_count`：一次最多重排幾筆，預設 100、上限 500，依失敗時間由舊到新取。
- `dry_run`：只回會重排的筆數，不改資料、不寫稽核紀錄。

建議流程：先 `dry_run` 看筆數，再不帶 `dry_run` 呼叫；回應 `has_more` 為 true 時用「同一個」窗與條件再呼叫，直到 `resend_count` 為 0。窗的終點固定不動：重送後又失敗的投遞失敗時間會晚於終點，不會在同一輪被選到第二次。

## 重送做了什麼、沒做什麼

重送只做一件事：在同一個 transaction 把符合的 `FAILED_PERMANENT` 列改回 `PENDING`、清掉 `failure_reason`，並為每一列排一個渠道 job。之後的送出由原本的渠道 job（`notification.deliver`）執行，所以：

- 送出前仍會重新檢查：收件人是否已刪除或退場、email 是否在停寄名單、來源事實是否仍成立（例如券是否仍可用）、行銷類推播的靜音時段（台北 22:00 至 09:00 延到靜音結束）。重送的投遞若在這些檢查失敗，會再次記為永久失敗（原因同上表），不會送出。
- 冪等：只會選中 `FAILED_PERMANENT` 的列。已送出（`SENT`）或已重排（`PENDING`）的投遞不會再被選中，重複呼叫不會讓同一筆送出兩次；投遞冪等鍵（意圖、收件人、渠道）不變，不會建立新的投遞紀錄。
- 重送排的 job 帶 `requeued_at`（重排時間）：River 的唯一鍵連已完成的 job 也算，沒有它，24 小時內完成過的 job 會讓重送的 job 被略過。
- 並行呼叫以 `FOR UPDATE SKIP LOCKED` 各拿不重疊的列。

## 怎麼看結果

- `resend_count`：這次重排的筆數；`has_more`：還有符合條件但因 `max_count` 沒處理的。
- 重排後的結果看投遞紀錄：`SENT` 成功；再度 `FAILED_PERMANENT` 看新的 `failure_reason`；仍是 `PENDING` 表示 job 還沒執行（靜音時段延後、佇列忙碌或在重試中）。
- 稽核紀錄的對象種類是 `delivery_resend`，細節有筆數、條件與時間窗。

## 未觸達查詢

後台 RPC `NotificationAdminService/ListUnreachedNotifications`（權限 `notification.unreached_notifications.list`，唯讀）；後台頁面「未觸達通知」顯示近 7 天的結果。

定義：近 `days` 天（預設 7、上限 90，依意圖建立時間）內、交易必要分類（使用者不能關閉的分類，由型別註冊表的分類推導，不寫死型別名單）的通知，某個收件人在所有渠道都沒有觸達，即該收件人這則通知的投遞紀錄全部是 `FAILED_PERMANENT`。

- 推播或 email 任一筆 `SENT` 就算觸達。
- 收件匣不算觸達：只有收件匣列、推播與 email 都失敗的，仍列為未觸達。
- 還在 `PENDING` 的不算（還沒失敗）；失敗原因是 `no_longer_valid`（刻意不送）或 `recipient_gone`（收件人已刪除）的不算。
- 沒有投遞紀錄的意圖（沒有收件人、或收件人被略過）不算。
- 回應有總筆數與分頁的明細，明細只有參考 ID（投遞 id、意圖 id、帳號與店家 ID、型別、分類、各渠道的失敗原因），沒有文字、位址與 token。

查到未觸達之後，依失敗原因判斷：可重送的原因排除後用上面的流程重送；`no_email_address`、`no_device_token` 這類收件人端問題，重送前先確認收件人已能收到。
