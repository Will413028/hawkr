# accountdeletion 模組

帳號刪除的流程管理者；App、後台與訂單完成 consumer 共用公開 API。決策見 `backend/docs/adr/2026-09-30-account-deletion-module.md`。

## 職責

- 解析帳號是否刪除、有沒有 SELLER、帳號唯一的買家與目前的店家，協調等待訂單、關閉買家（管理員申請時連同店家）與刪除帳號。
- 在 transaction 前透過自有 Orders port 確認未結訂單：買家以 `buyer_id`，帳號目前的店家（管理員的申請與完成等待中的申請）以 `store_id`；任一查詢失敗都不寫入。
- 保存流程狀態與每次受理的申請紀錄；同一 pending 流程的重送也追加 attempt。

## 擁有的資料

schema `accountdeletion` 的 `deletion_requests`（流程狀態）與 `request_attempts`（append-only 申請紀錄）；沒有跨 schema FK。

## 模組內部結構

根套件宣告公開 API 與 Orders port（買家側 `HasOpenOrders`、店家側 `HasOpenStoreOrders`，同一個 `legacy/orderservice` client 實作），`internal/request` 執行申請的流程，`internal/model` 定義輸入及狀態，`internal/store` 為 sqlc 生成查詢。依賴 identity、buyer、seller；依賴圖以 `internal/arch/modules.go` 為準。

## 不變量

- 帳號本人的申請在 SELLER 存在時拒絕且不寫入（要先關店）；訂單 port 失敗時回 unavailable 且不寫入。`TestSellerPreconditionWritesNothing`、`TestOrdersErrorWritesNothing`。
- 管理員的申請把店家當參與者：交易內讀帳號目前的店家，有未結訂單時買家與店家一起進入等待，否則在同一個 transaction 關閉買家、關閉店家、刪帳號。停權的店家也一樣。`TestAdminRequestClosesStoreBuyerAndAccountTogether`、`TestAdminRequestDeletesSuspendedSeller`。
- 店家的訂單與買家的訂單一樣阻擋刪帳：管理員的申請與 `CompletePending` 在 transaction 前先讀帳號目前的店家（`CompletePending` 先確認有待辦才發任何訂單 RPC，沒有待辦的帳號直接回 FailedPrecondition：`TestCompletePendingWithoutARequestDoesNotQueryOrders`），買家沒有未結訂單時再查店家（買家已有就不必問）；有未結訂單時帳號、買家與店家一起進入等待，兩側都結清才完成；查詢失敗回 unavailable、不寫入也不完成（fail-closed）。交易內重讀店家，與交易前查訂單的不是同一個 stint（查完之後店家關閉又重新入駐）就回 unavailable 請重試。沒有覆寫旗標，管理員也不能略過店家的訂單。`TestAdminRequestIsPendingWhileTheStoreHasOpenOrders`、`TestCompletePendingChecksTheStoreAfterTheBuyerSideClears`、`TestStoreOrdersErrorFailsClosed`、`TestStoreReplacedAfterTheOrdersCheckWritesNothing`；帳號本人有店家時仍先拒絕、不問店家的訂單（`TestOwnerRequestWithAStoreDoesNotAskStoreOrders`）。
- 不論申請者與角色，交易內都讀帳號目前的店家：有店家卻沒有 SELLER 是不變量被破壞（開店與授予同一個 transaction），回 internal、不寫入（`TestStoreWithoutSellerRoleIsInternalError`）。有 SELLER 卻沒有店家也是不變量被破壞（關店與移除 SELLER 同一個 transaction），同樣回 internal、記 error log、不寫入（`TestSellerRoleWithoutStoreIsInternalError`）。
- 申請進入等待時，同一個 transaction 讓帳號進入 PENDING_DELETION（`identity.BeginDeletion`）：之後 identity 拒絕授予角色，所以等待期間不會開出新店；店家隨管理員的申請進入等待後，賣家也不能自己關店（等待刪除的店家沒有 `CAP_ACCOUNT_CLOSE`）。完成時只關閉已隨申請進入等待的店家。`TestStoreCannotOpenWhileAccountPendingDeletion`、`TestPendingStoreCannotBeClosedOrReplacedByOwner`、`TestAdminPendingIncludesStoreAndCompletesIt`。
- 同帳號只有一筆 PENDING_ORDERS：DB partial UNIQUE，申請在 transaction 內先以 identity 的帳號列鎖（`identity.LockAccount`）序列化。每次受理都追加 attempt，不覆寫先前的申請者與原因。`TestPendingRequestUniqueAndRepeatedRequest`、`TestConcurrentRequestsReuseOneProcessRow`、`TestAdminAttemptAfterOwnerPendingIsRecorded`。
- requester/operator/reason 在 attempts、status/completed_at 在 requests 的配對由 DB CHECK 與輸入驗證保護。`TestRequestConstraints`。
- 管理員的理由由本模組驗證與正規化（`platform/auditreason`：去掉前後空白，1 到 1000 字，與 seller 的後台關店理由同一條規則），錯誤帶欄位 `Reason`，傳輸層只對應到請求的路徑；理由是管理員的稽核文字，個資清理不動它（重新評估：隱私政策定稿時，或 A8）。`TestDeleteAccountValidatesItsInput`（httpapi/seller 的整合測試）。
- 有未結訂單時買家保留並進入 PENDING_DELETION；完成時在同一 transaction 關閉 buyer、刪帳號並標記申請完成。`TestPendingRequestUniqueAndRepeatedRequest`、`TestCompletePendingChecksOrdersAgain`、`TestRequestClosedDeletesAccountAndCompletes`。
- 遠端訂單只在 transaction 外查；交易內先以 `identity.LockAccount` 鎖住帳號（與授予、移除角色與刪帳號同一把鎖），重讀帳號與買家。一個帳號一生只有一個買家，交易外查訂單用的 buyer_id 在交易內不會換成另一個，所以不比對、不重試。SELLER 即使在遠端查詢後才加入也不寫入；完成狀態只在帳號同交易刪除或已刪時提交。`TestSellerGrantedBetweenOrdersReadAndTransactionWritesNothing`、`TestSellerGrantedBeforeTransactionLockIsNotDeleted`、`TestConcurrentRequestsReuseOneProcessRow`。
- 關閉買家只有本模組這一條路徑：交易內先 `identity.LockAccount`，再呼叫 buyer 的 `BeginDeletion`／`CloseForDeletion`（它們不自己鎖帳號），鎖順序帳號列先於買家列；完成時同交易刪帳號。買家的刪除事件有同 transaction 的訂閱者（social 鎖作者的內容與 stats，在 seller 的上層），所以派發事件之前先 `Buyer.LockForDeletion`、`Seller.LockForDeletion` 鎖完買家與店家列（`lockParticipants`），規則 4 的順序才成立：`TestDeletionLocksBuyerAndSellerBeforeSocialRows`。`TestRequestClosedDeletesAccountAndCompletes`、`TestCompletePendingChecksOrdersAgain` 同時檢查帳號已刪與買家已關。
- 買家刪帳時兩個方向的封鎖都刪（A7-1b）：`closeParticipants` 在鎖完買家與店家之後、買家的刪除事件派發之前呼叫 `Seller.ClearBlocksAgainstBuyer`（seller 擁有的 `store_blocks` 中以他為對象的列），再呼叫 `Buyer.CloseForDeletion`（buyer 擁有的：他發起的兩種封鎖與以他為對象的買家封鎖買家，在 buyer 自己的刪帳 transaction 內刪）；同一個 transaction，所以不留半套。`TestRequestClosedDeletesBlocksInBothDirections`。
- 帳號只由本模組刪除，不訂閱 identity 的 AccountDeleted；每小時 RunOnStart 的 River reconciliation 依 account_id 有界掃描、逐筆失敗仍繼續，防止錯過訂單事件。`TestReconcileBatchesAndContinuesAfterError`。一小時符合訂單結案非即時刪帳的需求，並限制過渡期 Java 訂單服務負載；步驟 4 事件只加速收斂。
- 帳號活著 ⇔ 買家活著（ADR `backend/docs/adr/2026-09-30-account-created-provisions-buyer.md`）：未刪除的帳號沒有現任買家是不變量被破壞，回 internal 並記 error log，不查訂單、不寫入、不刪帳。`TestAccountWithoutBuyerIsInternalError`。

## 已知限制

- Java 訂單在單體外，查詢只能在 transaction 前。ordering 搬入單體時由其公開 API 實作 port 並移入 transaction。
- 帳號本人有店家時仍要先關店（關店本身會檢查店家的未結訂單，在 seller）；是否讓本人的刪帳申請也把店家當參與者，等步驟 8 的 App 流程決定。
- 店家側訂單的 order.completed 加速在 legacy consumer（`buyerconsumer` 帶 `store_id`，以 seller 的公開 API 解析店家的帳號）；事件遺失或晚到由每小時的對帳補上，對帳會查兩側。

## 讀取模型

無；公開 API 回傳流程 State。

## 不負責

- buyer stint、PII 與收藏等清理：buyer 模組。
- 帳號的 SELLER 角色與墓碑、Clerk 使用者刪除：identity 模組。
- 訂單狀態的來源：ordering；過渡期由 `internal/legacy/orderservice` 投影 CodeMeta。
- Connect、legacy consumer 與後台路由：各入口及 `cmd` 組裝。

## 決策紀錄

舊服務機制的逐項重新判斷見同目錄 `CARRYOVER.md`。
