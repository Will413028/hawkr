# im 模組

## 職責

買家與店家之間的一對一聊天：對話、訊息（文字與圖片）、送出與守門、已讀與未讀、訊息隱藏的資料、刪帳與退場的抹除。以及即時通道（LISTEN/NOTIFY、訂閱、在線狀態）。給 httpapi、moderation（呼叫 `Hide`／`Restore`）使用。行為規格：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-2a-design.md`、`step-2b-design.md`；對外入口是 `httpapi/chat`（unary 與串流）；`cmd/hawkr` 接刪帳與退場的訂閱者與 worker，serve 以 `serveWithRun` 跑 `Run`（worker 不跑）。

## 擁有的資料

schema `im`（`db/schema.sql`）：`conversations`（`UNIQUE (buyer_id, store_id)`、`last_seq`、兩個已讀游標）、`messages`（`seq`）、`hides`、`presence_streams`（UNLOGGED：每條串流一列，是在線狀態的來源與每個 Party 並行串流數的全域計數）。圖片訊息的資產 ID（`image_asset_id`）是 UUID，資產由 media 擁有。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與 `Orders`、`Buyers`、`Stores` port。
- `internal/write`（送出、已讀、隱藏、還原）、`internal/read`（列表、訊息、未讀、relay 讀取）、`internal/rows`（列轉對外型別、墓碑與預覽推導）、`internal/live`（即時通道：通知、LISTEN 連線、Hub、訂閱者、存活標記與其清理 job）、`internal/purge`（刪帳與退場的訂閱者邏輯、抹除 job 與保底掃描）、`internal/text`（伺服器句子目錄）、`internal/model`、`internal/store`（sqlc）。
- 寫入的鎖順序與送出冪等見 `internal/write/AGENTS.md`。
- 依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。
- 訂單由 `Orders` port 取（`cmd/hawkr/im.go` 以 `legacy/orderservice` 實作，ordering 搬進單體後換）；`Buyers`、`Stores` 傳 `*buyer.Module`、`*seller.Module`。

## 不變量

- 送出的寄件人由呼叫者的 session 決定（`Party`），模組不收 client 帶的寄件人；給 `ConversationID` 時以對話授權，不是該對話的一方回 NotFound。守門順序固定 G1 停權 → G2 停業 → G3 封鎖 → G4 收件人已刪除 → G5 詞典，任何查詢錯誤回 Unavailable（不放行）。守門：`write` 的守門表格測試。
- 停權（G1）與封鎖（G3）共用「進行中」判定（`Orders.OpenOrderBetween`，只在 G1 或 G3 命中時查，一次送出最多一次）：有進行中的訂單或售後就放行；查詢失敗回 Unavailable。停業只有 `CLOSED`，等待刪除雙向都可聊。
- seq 由 `AllocateSeq` 一句 SQL 配發（建立對話、`last_seq + 1`、推進寄件人自己那一方的游標）；`UNIQUE (conversation_id, seq)` 是最終保證；seq 沒有空洞。未讀 = `last_seq - 自己的游標`；兩個游標以 CHECK 限在 `0..last_seq`；`MarkRead` 只前進不超過 `last_seq`；讀取方法不寫游標。
- `Send` 的 `inTx` 第一組語句：`Buyers.LockNotClosed`、`Stores.LockNotRetired`（買家先於店家，`FOR KEY SHARE`；刪除中的買家放行），在 `guard`（含 `Orders` RPC，tx 外）之後、`AllocateSeq` 之前；NotFound 翻成既有訊息。`guard` 與 legacy 呼叫不得進 tx（`TestSendCallsOrdersOutsideTheWriteTransaction`）。讀取讀 `pub_closed_buyers()`、`pub_retired_stores()`。見 `CARRYOVER.md`。
- 預覽不存，讀取時由最後一則推導（隱藏與抹除不給任何內容）；隱藏、抹除的訊息在所有讀取路徑都回墓碑；模組不寫中文句子。
- 刪帳與退場：`OnBuyerDeleted`、`OnStoreRetired` 同 transaction 只排 `im.erase_party`（`EnqueueErase`，以身分為唯一鍵，ByState 含 Running）；只抹該身分寄出的訊息（不依帳號列舉）；不訂 `BuyerDeletionRequested`、不訂 identity 事件。job 依 id 升序每批 200，保留列、清空內容與圖片並把圖標記刪除，可重跑；`im.erase_sweep` 每 15 分鐘補排。
- 隱藏與還原在同 transaction 發 `MessageHidden`、`MessageRestored`（D24，只帶參照與列舉，作者是寄件方 `(AuthorKind, AuthorID)`）；重複隱藏不發。守門：`TestMessageHideEventFailureRollsBackTheHide`。
- 儲存的代碼是 UPPER_SNAKE（`BUYER`、`STORE`、`TEXT`、`IMAGE`）；`message_type` 只有 `TEXT`、`IMAGE`，卡片隨對外契約加。
- 圖片訊息在送出的 transaction 內（守門鎖與配發 seq 之後）`Confirm`，抹除訊息時同 transaction 把圖 `MarkDeleted`；送出回滾時圖片仍是暫存。守門：`TestImageMessageConfirmsTheAssetAndEraseMarksItDeleted`、`TestRolledBackImageSendLeavesTheAssetStaged`、`TestImageSendRefusesATokenStagedByAnotherAccount`。
- 即時通知（頻道 `im_events`）由 `write` 在寫入的同一個 transaction 內 `pg_notify`，不經 `events.Publish`；payload 只有業務鍵與 seq。訂閱者的內容一律經 `MessagesAfter` 讀取（授權在讀取），通知只是提示，漏掉的靠游標補洞。
- 即時通道（LISTEN 連線、Hub 扇出、在線狀態、串流生命週期）的不變量見 `internal/live/AGENTS.md`。

## 不負責

停權時強制斷線；串流傳輸 helper（heartbeat、最長壽命、寫入 deadline、重驗節奏）；Connect handler 與對外契約；隱藏的決定與審核（moderation）；買家名稱與頭像（httpapi 以 `buyer.BatchDisplay` 補）；舊 im-service 已刪除，`legacy/imrelay` 已接進 `cmd/hawkr`（A6-6e）。

## 決策紀錄

從 im-service 沿用的每個機制、當時的決定與理由，以及與 im-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。
