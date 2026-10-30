# seller/internal/governance

通報、案件與詢問。

## 不變量

- 通報只指向公開過的店，重複送出併成補充；佐證與證件先寫讀取紀錄才讀。守門：`TestEvidenceReadIsRecordedBeforeTheBytesAreRead`。規格：§7、§9。
