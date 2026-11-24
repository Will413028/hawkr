# announce（後台公告）

建立與列表（`Create`、`List`）與派送 job 鏈（`fanout.go`）。規格：計畫 `backend/docs/plans/2026-11-23-a8-notification.md` 的 A8-2c、D17（只寫英文）、D18（分類 `ANNOUNCEMENT`：收件匣不可關、推播可關、不寄 email）。

## 不變量

- 公告是資料：`announcements` 一列、`intents` 一列（id 與公告相同，只為讓推播投遞紀錄的 `intent_id` 有對象；型別 `announcement` 是註冊表的 Fanout 型別，不排 `notification.process_intent`）、兩條 job 鏈的第一個 job，同一個 transaction，並寫 `adminaudit`。守門：`TestAnnouncementAdminRPCs`。
- 冪等：`(admin_id, request_id)` UNIQUE；重送回第一次的公告、不再排 job 與寫稽核，內容不同回 InvalidInput（`RequestID`）。守門：`TestCreateWithTheSameRequestIDMakesOneAnnouncement`。
- 收件匣與推播是兩條分開的鏈（`InboxArgs`、`PushArgs`）：逐受眾、逐頁（`ChunkSize` 個收件人，keyset 游標在 job 參數），一頁處理完才排下一頁，沒有收件人了換受眾，最後設 `inbox_done_at`／`push_done_at`；兩者都有才是 `dispatched`。每個收件人自己一個 transaction，先 `intake.LockRecipient`（帳號、買家或店家）；已刪除、關閉、退場的略過。守門：`TestFanOutIsChunkedAndRerunningAPageIsIdempotent`、`TestBothAudiencesAreDispatchedInTurn`。
- 沒有 device token 的帳號也有收件匣列（不看 token）；推播只看 `ANNOUNCEMENT` 的推播偏好（`model.Enabled`），每個沒關掉的收件人一筆 `PENDING` 投遞紀錄並排 `notification.deliver`（A8-3 的 adapter 處理沒有 token 的情形）。守門：`TestAccountWithoutDeviceTokenStillGetsTheInboxItem`、`TestPushRespectsTheAnnouncementPreferenceAndInboxDoesNot`。
- 重試冪等：收件匣列以 `inbox_items_announcement_recipient` UNIQUE、投遞紀錄以 `deliveries_recipient_channel_key`；`inbox_count` 只在真的新寫列時同 transaction 遞增。
- 公告收件匣列的 `type = 'announcement'`、`announcement_id` 非空、`params = {"announcement_id": <ID>}`（只存 ID）；註冊表的 `announcement` 型別（`registry/announcement.go`：Fanout、兩個受眾、收件匣加推播、`FixedLocale` 英文）在讀取時以 `Load` 向公告表解析標題與內文，所以收件匣的型別篩選、分類篩選、未讀數與全部已讀都涵蓋公告，任何請求語系都是同一份英文。`registry.Delivered` 由這個型別推導。守門：`TestAnnouncementShowsInTheInboxAndIsRenderedInEnglish`、`TestGoldenAnnouncementRendering`、`TestEveryRegisteredTypePassesTheSelfCheck`。
