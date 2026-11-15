# social/internal/post

貼文生命週期、排程發布與公開讀取。取消排程的低層（事件、快取失效、依發起者取消）在 `internal/schedule`。

## 不變量

- 排程的真相是 `post_schedules`（一次排程一列、不重用，取消原因凍結在列上）；River job `social.publish_scheduled` 只帶 `schedule_id`，唯一鍵涵蓋排隊、執行、重試中的狀態（不含已完成）。到點時排程不是 `PENDING` 就什麼都不做，所以重試與保底掃描補排的重複 job 無害。守門：`TestScheduledPublishIsIdempotent`、`TestSweepRequeuesOverdueSchedules`。
- 鎖順序：貼文列 → 排程列。手動發布、重排、刪除、內容下架、店家清除、店主取消與到點發布都先鎖貼文列（多篇依 ID 由小到大），所以彼此序列化。守門：`TestScheduledPublishAndOwnerCancelSerialize`。
- 取消原因：手動發布與重排 `REPLACED`、刪除與店家清除 `DELETED`、內容下架 `TAKEN_DOWN`、店主 `OWNER_CANCELED`、授權失效 `AUTHORITY_LOST`、到點時不再滿足公開要件 `NOT_PUBLISHABLE`（不重試）；每個被取消的排程發 `PostScheduleCanceled` 並在同 transaction 排快取失效。守門：`TestManualPublishCancelsTheSchedule`、`TestDeleteCancelsTheSchedule`、`TestTakedownCancelsTheSchedule`、`TestPurgeOfAStoreCancelsItsSchedules`。
- 到點先做 social 自己的檢查（貼文未刪、未下架），再問 `Deps.Authorizers` 裡該發起者種類的 `ScheduleAuthorizer`：`ErrScheduleDenied` 取消為 `AUTHORITY_LOST` 且不重試，其他錯誤回傳讓 job 重試；通過才經手動發布同一條 `publishLocked` 發布。授權實作只能用不鎖的讀取。守門：`TestAuthorizerDenialCancelsWithoutRetry`、`TestAuthorizerIsConsultedPerKind`。
- 排程中的貼文可以編輯，編輯後仍要滿足公開要件。守門：`TestEditingAScheduledPostStillMeetsPublishRequirements`。
- 排程時點距現在 5 分鐘到 90 天（`model.MinScheduleLead`、`model.MaxScheduleLead`），可排程的狀態是 `DRAFT`、`UNPUBLISHED`。守門：`TestScheduleBounds`、`TestScheduleStateAndVersionRules`。
