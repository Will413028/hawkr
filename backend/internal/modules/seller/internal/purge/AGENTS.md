# seller/internal/purge

關店後的兩段清理。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 兩段清理只在期滿且無保存條件時做，候選鎖住後重判。守門：`TestPurgeRechecksTheAppealHoldAfterLockingTheCandidates`。規格：§11。
