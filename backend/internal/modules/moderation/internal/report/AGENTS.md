# moderation/internal/report

檢舉的送出與檢舉人自己的清單。

## 不變量

- 送出時依序鎖 `reporter_daily_counts`、`cases`、`case_snapshots`、`media_ref_sync`；案件處理（`internal/handle`）與刪帳訂閱者（`internal/retention`）的鎖順序要與它相容（`internal/platform/lockorder`）。
- 每日額度、去重與冪等的規則見模組 `AGENTS.md`。
- 評價與聊天訊息的檢舉經 review 的 `ReviewForReport`、im 的 `MessageForReport`（唯讀、不鎖列、加入 `SubmitReport` 的 transaction）；聊天快照是被檢舉訊息加前後各 `ChatContextWindow` 則，前後文已隱藏或已抹除的是墓碑（`TestChatReportFreezesFiveBeforeAndAfterWithTombstones`）；非對話方與已隱藏的訊息是 NotFound（`TestReportAuthorizationForChatAndReview`）。
