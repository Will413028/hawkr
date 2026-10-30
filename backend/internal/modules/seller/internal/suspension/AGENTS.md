# seller/internal/suspension

停權期間的重算。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 停權期間只由 `suspension.Recompute` 寫，先鎖再讀。守門：`TestRecomputeReadsMeasuresAfterLockingTheStores`。規格：§5。
