# moderation/internal/report

檢舉的送出與檢舉人自己的清單。

## 不變量

- 送出時依序鎖 `reporter_daily_counts`、`cases`、`case_snapshots`、`media_ref_sync`；案件處理（`internal/handle`）與刪帳訂閱者（`internal/retention`）的鎖順序要與它相容（`internal/platform/lockorder`）。
- 檢舉人的擁有者守門鎖（`lockReporter`：店家 `Seller.LockNotRetired`、買家 `Buyer.LockNotClosed`，`FOR KEY SHARE`）先於 moderation 的任何列（seller、buyer 先於 moderation），已退場或關閉回檢舉人專屬的 NotFound，與清除訂閱者序列化（見 `internal/retention/AGENTS.md`）。必要條件：新檢舉在凍結前，店家已被 `CapChatRead` 擋、買家沒有；鎖要擋的是凍結前就返回的重送與併入進行中案件（`recordRequest`）。
- 每日額度、去重與冪等的規則見模組 `AGENTS.md`。
- 檢舉人是買家或店家（`SubmitInput.ReporterBuyerID`、`ReporterStoreID` 恰好一個為正）：店家只能檢舉 CHAT_MESSAGE，`MessageForReport` 以 `im.Party` 的店家核對對話方；本人核對（`notSelf`）以帳號比，店家取 `seller.Store` 的 `AccountID`；額度走 `BumpStoreDailyCount`。REVIEW_REPLY 由買家檢舉，經 `ReviewForReport` 讀回覆，沒有回覆或已被下架是 NotFound（`TestReviewReplyTakedownHidesOnlyTheReply`）。
- 評價與聊天訊息的檢舉經 review 的 `ReviewForReport`、im 的 `MessageForReport`（唯讀、不鎖列、加入 `SubmitReport` 的 transaction）；聊天快照是被檢舉訊息加前後各 `ChatContextWindow` 則，前後文已隱藏或已抹除的是墓碑（`TestChatReportFreezesFiveBeforeAndAfterWithTombstones`）；非對話方與已隱藏的訊息是 NotFound（`TestReportAuthorizationForChatAndReview`）。
