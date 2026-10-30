# seller/internal/views

商品瀏覽數的記錄與彙總。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 瀏覽數以 `store_id` 與台北日計，請求路徑不呼叫商品服務。守門：`TestRollupBucketsByStoreAndTaipeiDayOfOccurrence`。規格：§16。
