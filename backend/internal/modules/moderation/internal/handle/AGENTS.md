# moderation/internal/handle

案件的處理：管理員對進行中的案件下決定並結案，處置與結案同一個 transaction。

## 不變量

- 案件結案是終態，不重開、不改判；結案 UPDATE 以 `(id, version, closed_at IS NULL)` 轉移，0 列回 `VersionConflict` 並回滾處置（`TestDecideRollsBackTheActionWhenCloseLosesTheRace`）；新檢舉掛進既有案件讓 `version` 加一，管理員讀到之後才掛進來的檢舉讓結案衝突（`TestAReportAttachedAfterTheAdminReadMakesTheCloseConflict`）。決定只存在案件，檢舉的結果由案件推導，不存逐筆 outcome。
- 內容已被刪除時，警告與停權用快照裡凍結的作者，隱藏與內容下架回 FailedPrecondition（`TestWarnAndBanUseTheFrozenAuthorWhenTheContentIsGone`）。
- 後台決定留稽核：`DecideCase` 在結案後同 transaction 呼叫 `adminaudit.Record`（`Actor.Action` 必填，是後台 RPC 的 procedure 名稱，缺漏是接線錯誤、回未分類的 internal error），版本衝突回滾時紀錄跟著消失（`TestDecideWritesTheAdminAuditRecordInTheSameTransaction`）。
- 處置與結案在同一個 transaction；案件列最後才寫：擁有處分的模組（social、identity、seller）的列都排在 moderation 之前，先呼叫它們、後寫自己的列（`internal/platform/lockorder`）；每次處理只走一條處置路徑。
- 店家寫的內容只能走內容下架，內容必須經 social 驗證屬於管理員指定的店家；買家不能隱藏或警告店家，店家不能被停權。
