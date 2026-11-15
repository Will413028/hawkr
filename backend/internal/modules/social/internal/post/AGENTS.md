# social/internal/post

貼文生命週期、排程發布與公開讀取。取消排程的低層（事件、快取失效、依發起者取消）在 `internal/schedule`。

## 不變量

- 排程的真相是 `post_schedules`（一次排程一列、不重用，取消原因凍結在列上）；River job `social.publish_scheduled` 只帶 `schedule_id`，唯一鍵涵蓋排隊、執行、重試中的狀態。到點時排程不是 `PENDING` 就什麼都不做，重複 job 無害；沒有保底掃描（job 與排程同 transaction 寫入）。守門：`TestScheduledPublishIsIdempotent`。
- 最後一次嘗試失敗，另開 transaction（貼文列 → 排程列）取消為 `RETRY_EXHAUSTED` 並記 error log。守門：`TestRetryExhaustionCancelsTheSchedule`。
- 鎖順序：貼文列 → 排程列。手動發布、重排、刪除、內容下架、店家清除、店主取消與到點發布都先鎖貼文列（多篇依 ID 由小到大），所以彼此序列化。守門：`TestScheduledPublishAndOwnerCancelSerialize`。
- 取消原因：手動發布與重排 `REPLACED`、刪除與店家清除 `DELETED`、內容下架 `TAKEN_DOWN`、店主 `OWNER_CANCELED`、授權失效 `AUTHORITY_LOST`、不再滿足公開要件 `NOT_PUBLISHABLE`、重試用盡 `RETRY_EXHAUSTED`、到點超過 `model.ScheduleExpiry` `EXPIRED`；每個被取消的排程發 `PostScheduleCanceled`。守門：`TestManualPublishCancelsTheSchedule`、`TestDeleteCancelsTheSchedule`、`TestTakedownCancelsTheSchedule`、`TestPurgeOfAStoreCancelsItsSchedules`。
- 到點先做 social 自己的檢查（未刪、未下架；已 `PUBLISHED` 取消為 `REPLACED`；超過 `ScheduleExpiry` 取消為 `EXPIRED`，皆不重試），再問 `Deps.Authorizers` 該發起者種類的 `ScheduleAuthorizer`：`ErrScheduleDenied` 取消為 `AUTHORITY_LOST`，其他錯誤回傳重試。授權實作只能用不鎖的讀取。守門：`TestAuthorizerDenialCancelsWithoutRetry`、`TestAuthorizerIsConsultedPerKind`、`TestDueScheduleBeyondTheExpiryWindowIsCanceledAsExpired`。
- 授權通過後、任何寫入之前檢查公開要件（不滿足 `NOT_PUBLISHABLE`）；`publishLocked` 與訂閱者的任何錯誤都回滾重試，不當成 `NOT_PUBLISHABLE`。守門：`TestSubscriberFailureAfterPublishRollsBackInsteadOfCancelingAsNotPublishable`。
- `OWNER` 的授權與手動發布同一個店家授權（`CapMarketing`）：只有 `PermissionDenied`、`NotFound` 是 `AUTHORITY_LOST`，`FailedPrecondition` 與其他錯誤重試。守門：`TestOwnerAuthorizerDeniesOnlyPermanentLossOfTheRight`、`TestOwnerScheduleIsRetriedWhileThePreconditionIsMissing`。
- 排程與店主取消都遞增貼文 `version` 並回傳新版本（取消帶讀取時的版本，沒有待發布排程時冪等、不檢查版本）；被內容下架的貼文不能排程。守門：`TestScheduleBumpsThePostVersion`、`TestScheduleRejectsAPostUnderTakedown`。
- 所有系統取消由 `schedule.Canceler` 在取消處遞增一次（呼叫方自己遞增的傳 `CallerBumps`）；取消不排快取失效（有待發布排程的貼文不是公開的）。守門：`TestEverySystemCancelBumpsThePostVersion`。
- 排程中的貼文可以編輯，編輯後仍要滿足公開要件。守門：`TestEditingAScheduledPostStillMeetsPublishRequirements`。
- 排程時點距現在 5 分鐘到 90 天（`model.MinScheduleLead`、`model.MaxScheduleLead`），可排程的狀態是 `DRAFT`、`UNPUBLISHED`。守門：`TestScheduleBounds`、`TestScheduleStateAndVersionRules`。
- 建立貼文的 `request_id` 去重（AIP-155）：交易外先 replay，再 `media.Finalize`，交易內 `INSERT ... ON CONFLICT (store_id, request_id) DO NOTHING`，0 列就在同一交易內 replay 讀回先到的那一篇；內容 hash 不同一律 InvalidInput。依賴 Finalize 冪等（同 token 回同一資產，規格 media §2）加唯一鍵裁決，不加鎖（session 層 advisory lock 持一條連線再取第二條，會在連線池耗盡時死結）。回應在交易內組好。守門：`TestPostRequestIDConcurrentRetriesWithImageCreateOnePost`、`TestPostRequestIDWithDifferentContentIsInvalidInputEvenConcurrently`、`TestFakeFinalizeIsIdempotentForTheSameToken`。
