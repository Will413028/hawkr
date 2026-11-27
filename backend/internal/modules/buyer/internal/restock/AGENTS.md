# buyer/internal/restock

補貨訂閱與通知目標（D26）：SKU 粒度的一次性訂閱，由 catalog 的補貨 worker 經 `cmd/hawkr` 的 adapter 呼叫 `FanOut`、`PurgeSKUs`。

## 不變量

- 訂閱以 (買家, SKU) 為鍵、不設到期，同組重複訂閱冪等；另存 SKU 所屬的 `product_id` 供清單顯示。SKU 存在與否、有沒有貨由 `internal/httpapi/buyer` 經 catalog 的 port 確認（有貨拒絕，`RESTOCK_SKU_IN_STOCK`），本子領域不看 catalog。守門：`TestRestockFanOutIsPerSKU`、`TestConnectRestockSubscriptionIsPerSKUAndRejectedWhenInStock`（httpapi/buyer）。
- `FanOut` 只通知並刪除輸入 SKU 的訂閱，同商品其他 SKU 的訂閱不被通知也不被刪；事件與刪除同 transaction、批次有界（1..100）、發生時間不得為零、不得在 transaction 內開始。守門：`TestRestockFanOutIsPerSKU`、`TestRestockBatchesAndRollback`、`TestFanOutRejectsZeroOccurrenceTime`。
- `PurgeSKUs` 只刪指定 SKU 的訂閱，沒有訂閱或空清單是 no-op，可重複呼叫。守門：`TestPurgeRestockSKUs`。
- `BatchCheck` 以商品回報任一 SKU 已訂閱。守門：`TestRestockFanOutIsPerSKU`。
