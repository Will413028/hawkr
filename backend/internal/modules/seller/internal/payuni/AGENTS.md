# seller/internal/payuni

PayUni 的外部呼叫與同步 job。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- PayUni 外部呼叫不在 transaction 內，一店一個 sync job。守門：`TestSyncJobIsUniquePerStoreInRiver`。規格：§14。
