# seller/internal/closing

關店與恢復營業。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 關店只走 `closing.close()`，紀錄只在 `store_closures`，自己關店先查未結訂單。守門：`TestEveryClosePathRecordsWhoClosedTheStore`。規格：§11。
