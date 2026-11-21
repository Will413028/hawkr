# im/internal/write

寫入路徑：送出、已讀、隱藏與還原、內容下架的訂閱者。

## 不變量

- 店家寄出的訊息可被 seller 的內容下架措施（`CHAT_MESSAGE`，A6-D21）下架：im 不存複本，隱藏的讀取述詞（`messages.sql`、`conversations.sql` 的 `hidden`）同時看未還原的 `hides` 與 `seller.pub_content_takedowns()`，下架與隱藏一樣是墓碑；`OnMeasureCreated`／`OnMeasureRevoked` 同 transaction 依 id 升序鎖訊息列、建立時核對寄件方是店家且對話屬措施的店家（不符回 `TakedownContentError`，不存在的訊息成立）、對每則發可見度改變的 NOTIFY（`TestStoreChatTakedownTombstonesAndRevokeRestores`、`TestTakedownRejectsAMessageThatIsNotTheMeasureStores`）。
