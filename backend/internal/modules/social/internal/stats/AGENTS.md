# social/internal/stats

計數的維護與每日對帳。

## 不變量

- 計數只由 trigger 與 `reevaluate_*` 維護，「是否計入」只有那一份定義；stats 有 `CHECK (>= 0)`。守門：`TestStatsMatchTheDatasetAfterRandomWrites`、`TestConcurrentReevaluatesDoNotCountTwice`、`TestReevaluateHasNoDeadlocksAndEndsConsistent`、`TestStatsRejectNegativeCounts`。規格：§6。
- 每日對帳先修旗標再覆寫計數，且不蓋掉並行的加減。守門：`TestReconcileRepairsFlagsBeforeCounts`、`TestRecountDoesNotOverwriteConcurrentLikes`。規格：§6。
