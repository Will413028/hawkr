# seller/internal/purge

關店後的兩段清理。

## 不變量

- 兩段清理只在期滿且無保存條件時做，候選鎖住後重判。守門：`TestPurgeRechecksTheAppealHoldAfterLockingTheCandidates`。規格：§11。
