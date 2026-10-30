# seller/internal/payuni

PayUni 的外部呼叫與同步 job。

## 不變量

- PayUni 外部呼叫不在 transaction 內，一店一個 sync job。守門：`TestSyncJobIsUniquePerStoreInRiver`。規格：§14。
