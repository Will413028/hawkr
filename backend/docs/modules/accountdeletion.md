# accountdeletion 模組行為規格

本文是單體 `accountdeletion` 模組行為的唯一來源。帳號刪除的流程管理者：解析帳號、買家與店家，協調等待未結訂單、關閉參與者與刪除帳號；App、後台與訂單完成 consumer 共用同一組公開 API。

## 1. 範圍與公開介面

- 根套件公開 `Module`（申請、完成等待中的申請、對帳）、型別別名 `RequestInput`、`Requester`、`State`，以及唯讀的 `Reader`。`Reader.AccountsWithOpenRequest(ctx, accountIDs)` 只需要 `*db.DB`，回傳其中有 `PENDING_ORDERS` 申請的帳號（依 ID 排序），給不備齊整個模組的組裝使用（本機種子的前提檢查）。
- 根套件宣告 `Orders` port：買家側 `HasOpenOrders(buyerID)`、店家側 `HasOpenStoreOrders(storeID)`。實作是 `internal/legacy/orderservice` 的同一個 client。
- 依賴 identity、buyer、seller，沒有模組依賴它；依賴圖的唯一來源是 `internal/arch/modules.go`。`internal/request` 執行申請，`internal/model` 定義輸入與狀態，`internal/store` 是 sqlc 生成的查詢。
- 無讀取模型；公開 API 回傳流程 `State`。

## 2. 資料表與約束

- schema `accountdeletion`，沒有跨 schema FK；`account_id` 是 bigint。
- `deletion_requests`（流程狀態）：`status` 為 `PENDING_ORDERS` 或 `COMPLETED`，`COMPLETED` 若且唯若有 `completed_at`；同帳號只有一筆 `PENDING_ORDERS`（partial UNIQUE `deletion_requests_one_pending`），歷史申請可有多筆。
- `request_attempts`（append-only 申請紀錄）：每次受理追加一筆，記 `requester`（`ACCOUNT_OWNER` 或 `ADMIN`）、`operator_admin_id`、`reason`、時間；`requester = 'ADMIN'` 若且唯若有 `operator_admin_id`；理由長度 ≤ 1000，管理員的理由非空且前後無空白。
- 申請 id 與 attempt id 是應用層產生的 UUIDv7，無 DEFAULT。

## 3. 申請的受理

- 買家是帳號唯一的買家（一個帳號一生一個買家）。未刪除的帳號沒有現任買家是不變量被破壞：回 internal、記 error log，不查訂單、不寫入、不刪帳。
- 未結訂單在 transaction 前由 `Orders` port 查：買家以 `buyer_id`；管理員的申請與 `CompletePending` 另以帳號目前的店家 `store_id` 查店家側。任一查詢失敗回 unavailable、不寫入、不完成（fail-closed）。
- 沒有未結訂單：同一個 transaction 關閉買家（管理員的申請連同店家）、刪帳號並把申請標為完成。
- 有未結訂單：買家（管理員的申請連同店家）保留並進入 PENDING_DELETION，申請為 `PENDING_ORDERS`；同一個 transaction 讓帳號進入 PENDING_DELETION（`identity.BeginDeletion`），之後 identity 拒絕授予角色，所以等待期間不會開出新店。
- 同一個 pending 流程的重送沿用同一筆申請，追加 attempt，不覆寫先前的申請者與原因；管理員接續本人的 pending 申請也追加。

## 4. 帳號本人的申請

- 帳號有 SELLER 時拒絕且不寫入（要先關店，關店本身在 seller 檢查店家的未結訂單）；此時不問店家的訂單。

## 5. 管理員的申請與店家

- 管理員的申請把店家當參與者：交易內讀帳號目前的店家，停權的店家也一樣。沒有未結訂單時，同一個 transaction 關閉買家、關閉店家、刪帳號。
- 店家的訂單與買家的訂單一樣阻擋刪帳。買家側先查，已有未結訂單就不問店家；有未結訂單時帳號、買家與店家一起進入等待，兩側都結清才完成。沒有覆寫旗標，管理員也不能略過店家的訂單。
- 交易內重讀店家；與交易前查訂單的不是同一個店家（查完之後店家關閉又重新入駐）時回 unavailable 請重試，不寫入。
- 店家隨管理員的申請進入等待後，賣家不能自己關店或換店（等待刪除的店家沒有 `CAP_ACCOUNT_CLOSE`）。完成時只關閉已隨申請進入等待的店家。
- 不論申請者與角色，交易內都讀帳號目前的店家：有店家卻沒有 SELLER、或有 SELLER 卻沒有店家，都是不變量被破壞，回 internal、記 error log、不寫入。

## 6. 完成等待中的申請（CompletePending）與對帳

- 先確認有 `PENDING_ORDERS` 申請才發任何訂單 RPC；沒有待辦的帳號直接回 FailedPrecondition。
- 買家側結清後再查店家側；兩側都沒有未結訂單才在同一個 transaction 關閉 buyer（與店家）、刪帳號並標記申請完成。仍有未結訂單則維持等待。
- order.completed 的 legacy consumer（`buyerconsumer`，帶 `store_id`，以 seller 的公開 API 解析店家的帳號）呼叫 `CompletePending` 加速收斂，一側失敗不擋另一側；有 `store_id` 卻沒有設定解析器是錯誤。
- 每小時的 River reconciliation 在啟動時補跑一次，依 `account_id` 有界 keyset 掃描 `PENDING_ORDERS` 的申請並呼叫 `CompletePending`，逐筆失敗仍繼續；事件遺失或晚到由它補上，對帳查兩側。
- 帳號只由本模組刪除，不訂閱 identity 的 `AccountDeleted`。

## 7. 交易、鎖與序列化

- 遠端訂單只在 transaction 外查。交易內先 `identity.LockAccount`（與授予角色、移除角色、刪帳號同一把帳號列鎖）序列化，再重讀帳號與買家；一個帳號只有一個買家，所以交易外查訂單用的 `buyer_id` 在交易內不比對、不重試。
- SELLER 即使在遠端查詢後才加入也不寫入（本人的申請拒絕，管理員的申請連同店家處理）；已被刪除的帳號記完成；完成狀態只在帳號同交易刪除或已刪時提交。
- 關閉買家只有本模組這一條路徑：鎖完帳號列後呼叫 buyer 的 `BeginDeletion`／`CloseForDeletion`（它們不自己鎖帳號），完成時同交易刪帳號。鎖順序：帳號列、買家列、店家列、上層模組（單體規則 4）。
- 買家的刪除事件有同 transaction 的訂閱者（social 鎖作者的內容與 stats，在 seller 的上層），所以派發事件之前先 `Buyer.LockForDeletion`、`Seller.LockForDeletion` 鎖完買家與店家列（`lockParticipants`）。
- 買家刪帳時兩個方向的封鎖都刪：`closeParticipants` 在鎖完買家與店家之後、買家的刪除事件派發之前，先呼叫 `Seller.ClearBlocksAgainstBuyer`（seller 擁有的 `store_blocks` 中以他為對象的列），再呼叫 `Buyer.CloseForDeletion`（他發起的兩種封鎖與以他為對象的買家封鎖買家）；同一個 transaction，不留半套。

## 8. 管理員的理由

- 管理員的理由由本模組驗證與正規化（`platform/auditreason`：去掉前後空白，1 到 1000 字，與 seller 的後台關店理由同一條規則），錯誤帶欄位 `Reason`，傳輸層只對應到請求的路徑。
- 理由是管理員的稽核文字，個資清理不動它。

## 9. 已知限制

- Java 訂單服務在單體外，訂單查詢只能在 transaction 前，查完到交易之間的變化靠交易內的重讀與 `PENDING_ORDERS` 對帳補上。
- 帳號本人有店家時仍要先關店，本人的申請不把店家當參與者。
- 對帳每小時一次；訂單結案不即時刪帳，事件只加速收斂。
