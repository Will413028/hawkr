# social/internal/sanction

買家的社群處分與隱藏紀錄。

## 不變量

- 隱藏與還原先鎖目標內容列。守門：`TestDirectHideRowRacingADeleteCountsOnce`。規格：§8。
- 處分事件（D24）：新增隱藏發 `ContentHidden`、還原發 `ContentRestored`、警告發 `BuyerWarned`，都在處分的同一個 transaction 內，只帶參照與列舉（處分 ID、內容參照、作者、`policy.Category`、`SourceRef`、`policy.AppealPath`），不帶管理員說明文字；冪等的重複隱藏不發。`Warn` 自己開（或加入）transaction。守門：`TestHideEventFailureRollsBackTheHide`、`TestRestoreEventFailureRollsBackTheRestore`、`TestWarnPublishesBuyerWarnedAndRollsBackWithTheWarning`。
