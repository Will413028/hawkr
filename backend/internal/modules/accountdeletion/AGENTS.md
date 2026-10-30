# accountdeletion 模組

## 職責

帳號刪除的流程管理者：解析帳號、買家與店家，協調等待未結訂單、關閉參與者與刪除帳號；App、後台與訂單完成 consumer 共用公開 API。行為規格的唯一來源是 `docs/modules/accountdeletion.md`。

## 擁有的資料

schema `accountdeletion`（`db/schema.sql`）：`deletion_requests`（流程狀態）、`request_attempts`（append-only 申請紀錄）；沒有跨 schema FK。讀取模型：無，公開 API 回傳流程 `State`。

## 發布的讀取介面

無。沒有 `db/pub.sql`，所以沒有 `pub_*`。唯一的讀取是 Go 的 `Reader.AccountsWithOpenRequest`（只需 `*db.DB`），形狀見 `docs/modules/accountdeletion.md` §1。

## 模組內部結構

- 根套件宣告公開 API 與 `Orders` port（買家側 `HasOpenOrders`、店家側 `HasOpenStoreOrders`，由 `internal/legacy/orderservice` 的同一個 client 實作）。
- `internal/request` 執行申請，`internal/model` 定義輸入與狀態，`internal/store` 是 sqlc 生成的查詢。
- 依賴 identity、buyer、seller；依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

- 訂單 port 失敗回 unavailable 且不寫入。守門：`TestOrdersErrorWritesNothing`。規格：§3。
- 不論申請者，店家都是參與者，同一個 transaction 關閉買家、店家與帳號（停權店家也一樣）。守門：`TestAdminRequestClosesStoreBuyerAndAccountTogether`、`TestOwnerRequestClosesStoreBuyerAndAccountTogether`、`TestAdminRequestDeletesSuspendedSeller`。規格：§4、§5。
- 店家的未結訂單與買家的一樣阻擋刪帳，查詢失敗 fail-closed，交易內店家換過就不寫入，沒有覆寫旗標。守門：`TestAdminRequestIsPendingWhileTheStoreHasOpenOrders`、`TestStoreOrdersErrorFailsClosed`、`TestStoreReplacedAfterTheOrdersCheckWritesNothing`、`TestStoreOpenedBetweenOrdersReadAndTransactionWritesNothing`、`TestOwnerRequestIsPendingWhileTheStoreHasOpenOrders`。規格：§5。
- `CompletePending` 沒有待辦申請時不發任何訂單 RPC；買家側結清後才查店家側。守門：`TestCompletePendingWithoutARequestDoesNotQueryOrders`、`TestCompletePendingChecksTheStoreAfterTheBuyerSideClears`。規格：§6。
- 有店家沒有 SELLER、有 SELLER 沒有店家、帳號沒有現任買家，都回 internal 且不寫入。守門：`TestStoreWithoutSellerRoleIsInternalError`、`TestSellerRoleWithoutStoreIsInternalError`、`TestAccountWithoutBuyerIsInternalError`。規格：§3、§5。
- 申請進入等待時帳號同交易進入 PENDING_DELETION，等待期間不能開店，等待中的店家不能自己關店。守門：`TestStoreCannotOpenWhileAccountPendingDeletion`、`TestPendingStoreCannotBeClosedOrReplacedByOwner`、`TestAdminPendingIncludesStoreAndCompletesIt`。規格：§3、§5。
- 同帳號只有一筆 PENDING_ORDERS（DB partial UNIQUE，交易內先 `identity.LockAccount` 序列化），每次受理追加 attempt。守門：`TestPendingRequestUniqueAndRepeatedRequest`、`TestConcurrentRequestsReuseOneProcessRow`、`TestAdminAttemptAfterOwnerPendingIsRecorded`、`TestRequestConstraints`（DB CHECK）。規格：§2、§3、§7。
- 遠端訂單只在 transaction 外查；店家或 SELLER 在遠端查詢後才出現也不寫入。守門：`TestStoreOpenedBetweenOrdersReadAndTransactionWritesNothing`、`TestSellerGrantedBeforeTransactionLockIsNotDeleted`。規格：§7。
- 關閉買家只有本模組一條路徑，鎖順序帳號列、買家列、店家列、上層模組；刪帳同時檢查帳號已刪與買家已關。守門：`TestDeletionLocksBuyerAndSellerBeforeSocialRows`、`TestRequestClosedDeletesAccountAndCompletes`、`TestCompletePendingChecksOrdersAgain`。規格：§7。
- 買家刪帳時兩個方向的封鎖都刪。守門：`TestRequestClosedDeletesBlocksInBothDirections`。規格：§7。
- 帳號只由本模組刪除；每小時 River 對帳有界掃描、逐筆失敗仍繼續。守門：`TestReconcileBatchesAndContinuesAfterError`。規格：§6。
- 管理員的理由由本模組驗證與正規化。守門：`TestDeleteAccountValidatesItsInput`（httpapi/seller）、DB CHECK `request_attempts_reason_check`。規格：§8。

## 不負責

- buyer stint、PII 與收藏等清理：buyer 模組。
- 帳號的 SELLER 角色、墓碑與登入 session 的刪除：identity 模組。
- 訂單狀態的來源：ordering；過渡期由 `internal/legacy/orderservice` 投影 CodeMeta。
- Connect、legacy consumer 與後台路由：各入口及 `cmd` 組裝。

## 決策紀錄

從舊服務沿用的每個機制、當時的決定與理由，以及與舊服務的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-09-30-account-deletion-module.md`（流程管理者放置、關閉買家只由本模組編排、等待刪除是 identity 保存的帳號狀態）、`docs/adr/2026-09-30-account-created-provisions-buyer.md`（帳號活著 ⇔ 買家活著）。
