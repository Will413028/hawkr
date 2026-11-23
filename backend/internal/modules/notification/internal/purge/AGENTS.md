# notification/internal/purge

保留期清除與個資清除（D2、D15、D16）；兩者都由 job 分批做、每批一個 transaction、可重跑。

## 不變量

- 同步訂閱者（`OnAccountDeleted`、`OnBuyerDeleted`、`OnStoreRetired`）在來源 transaction 內只排 `notification.purge_recipient` job，不碰 notification 的表。守門：`TestSynchronousSubscribersOnlyEnqueue`、`TestEnqueueOnlyEnqueuesTheJob`。
- 保留期 180 天：收件匣以 `updated_at`、投遞紀錄與意圖以 `created_at` 起算。`Sweep`（週期 job `notification.retention_sweep`，每 24 小時、啟動即跑）依序刪過期投遞紀錄、已沒有投遞紀錄的過期意圖、過期收件匣列；投遞紀錄還在保留期內的過期意圖留著。`deliveries.intent_id` 是 `ON DELETE RESTRICT`：投遞紀錄的 `created_at` 晚於意圖，刪過期意圖不能連帶刪掉保留期內的投遞紀錄，漏掉「沒有投遞紀錄」的條件時 job 會因外鍵失敗。守門：`TestRetentionSweepDeletesExpiredRowsOnly`。
- 收件人範圍：`AccountDeleted` 是帳號的全部列；`BuyerDeleted` 是該帳號的 BUYER 受眾；`StoreRetired` 是該帳號該店（`store_id`）的 STORE 受眾，同帳號的另一間店與另一個受眾不動。偏好是帳號與受眾的、沒有店別，所以 `StoreRetired` 刪帳號的整組 STORE 偏好。守門：`TestPurgeAfterEachDeletionEvent`。
- `Recipient` 的順序：先把該收件人的意圖參數換成 `'{}'`（意圖經 `inbox_item_intents` 與 `deliveries` 找到），再清投遞紀錄的送出文字與語系（列留著，D16），再刪收件匣列（連結隨列刪除，所以意圖要先處理），最後刪偏好。每一步已處理的列不再符合條件，重跑安全。
- 清除只在身分已刪除或退場之後做；之後才處理的意圖由 intake worker 的共享鎖檢查擋掉（`TestIntentProcessedAfterBuyerDeletedOrStoreRetiredCreatesNothing`）。
