# seller/internal/governance

通報、案件與詢問。上層規則見模組根 `AGENTS.md`；行為規格唯一來源是 `docs/modules/seller.md`。

## 不變量

- 通報只指向公開過的店，重複送出併成補充；佐證與證件先寫讀取紀錄才讀。守門：`TestEvidenceReadIsRecordedBeforeTheBytesAreRead`。規格：§7、§9。
