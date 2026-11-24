# im/internal/write

寫入路徑：送出、已讀、隱藏與還原、內容下架的訂閱者。

## 不變量

- 店家寄出的訊息可被 seller 的內容下架措施（`CHAT_MESSAGE`，A6-D21）下架：im 不存複本，隱藏的讀取述詞（`messages.sql`、`conversations.sql` 的 `hidden`）同時看未還原的 `hides` 與 `seller.pub_content_takedowns()`，下架與隱藏一樣是墓碑；`OnMeasureCreated`／`OnMeasureRevoked` 同 transaction 依 id 升序鎖訊息列、建立時核對寄件方是店家且對話屬措施的店家（不符回 `TakedownContentError`，不存在的訊息成立）、對每則發可見度改變的 NOTIFY（`TestStoreChatTakedownTombstonesAndRevokeRestores`、`TestTakedownRejectsAMessageThatIsNotTheMeasureStores`）。
- 送出冪等：`SendInput.RequestID`（必填、UUIDv4）；鍵是 `UNIQUE (conversation_id, sender_kind, request_id)`（`messages_request_key`）。重送判定（`MessageByRequest`）在對話解析與授權之後、守門 G1–G5 之前：回第一次寫入的那則（同 ID、seq），不配 seq、不 NOTIFY、不發事件、不排 job、不碰 staged 圖片；Type 不同或文字正規化後不同回 InvalidInput 指向 `RequestID`（圖片只比 Type）。
- 送出冪等的並行：以入口的 advisory lock 序列化（B1-D26）：Send 的 transaction 第一句 `LockSendRequest`（`pg_advisory_xact_lock`，鍵是 (買家, 店家, 寄件方, request_id) 的雜湊，不在鎖序裡），取得後在 transaction 內重做重送判定，之後才取守門鎖、`Confirm` 圖片、`AllocateSeq`、寫入；輸家等贏家提交後走重送判定回第一則，不會先撞到 `Confirm` 的 token 認領（B1-D23）。
- 送出冪等的快速路徑與最後防線：transaction 外另有一次重送判定當快速路徑，只因守門在 transaction 外而重送不能被之後才發生的封鎖擋下。`messages_request_key` 唯一鍵留作最後防線，撞到回 Internal，沒有重試迴圈。守門：`TestConcurrentSameRequestIDWritesOnce`、`TestConcurrentSameRequestIDImageSendsConfirmOneAsset`。
