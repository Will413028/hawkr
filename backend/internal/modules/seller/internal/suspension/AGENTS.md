# seller/internal/suspension

停權期間的重算。

## 不變量

- 停權期間只由 `suspension.Recompute` 寫，先鎖再讀。守門：`TestRecomputeReadsMeasuresAfterLockingTheStores`。規格：§5。
