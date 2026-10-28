# buyer 模組

## 職責

買家業務身分與個人資料的擁有者：stint、profile、頭像、收貨地址、收藏、追蹤、個人封鎖、瀏覽、補貨訂閱、刪除，以及會員方案與額度週期。行為規格的唯一來源是 `docs/modules/buyer.md`。

## 擁有的資料

schema `buyer`（`db/schema.sql`）：`buyers`、`shipping_addresses`、`favorites`、`seller_subscriptions`、`blocked_buyers`、`blocked_stores`、`visit_history`、`restock_subscriptions`、`membership_plans`、`memberships`、媒體同步狀態 `media_ref_sync` 與退場資產 `retired_assets`。讀取模型：清單只回 ID 與時間，CLOSED 不曝露 PII（規格 §9）。

## 發布的讀取介面

形狀在 `db/pub.sql`，定義在 `db/objects.sql` 與手寫 migration；消費端只能呼叫函式。改形狀等於改契約：同一次變更內更新 `pub.sql` 與所有消費端。守門：`TestPubContractsMatchMigrations`（cmd/hawkr）。語意見規格 §10。

- `buyer.pub_inactive_buyers() RETURNS SETOF buyer.pub_inactive_buyer (buyer_id bigint)`：此刻不是營運中的買家（刪除中與已關閉），沒有時間參數。給 social 每日對帳核對複本，不給求值呼叫。守門：`TestInactiveBuyersAndDeletionRequestedEvent`。
- `buyer.pub_blocked_buyers() RETURNS SETOF buyer.pub_blocked_buyer (blocker_buyer_id bigint, blocked_buyer_id bigint)` 與 `buyer.pub_blocked_stores() RETURNS SETOF buyer.pub_blocked_store (blocker_buyer_id bigint, store_id bigint)`：封鎖現況，一列一個封鎖，強一致、沒有時間參數，兩種對象不共用 kind 欄位。給 social 的述詞內聯使用。守門：`TestPubBlocksIsInlinedAndUsesTheIndexes`。

## 模組內部結構

根套件只組裝、別名與轉呼叫。`internal/profile`、`address`、`favorite`、`follow`、`block`、`visit`、`restock`、`membership`、`deletion` 各管自己的 use case；`mediaref` 同步頭像引用；`stint` 共用買家鎖定與狀態檢查；`model` 放型別與輸入共通規則。依賴圖的唯一來源是 `internal/arch/modules.go` 的 `InternalDependencies`。

## 不變量

守門測試在本模組，其他模組的另標。

- 帳號活著 ⇔ 買家活著；一帳號一生一個買家（`UNIQUE (account_id)`），只在建帳 transaction 內建立，只由 accountdeletion 關閉。守門：`TestAccountCreatedCreatesTheOnlyBuyer`、`TestRequestClosedDeletesAccountAndCompletes`（accountdeletion）。規格：§2。
- CLOSED 必有 deleted_at，PII 只清 CLOSED，未清的暱稱 1–100 字元。守門：DB CHECK、`TestPIICleanupStintsAndBacklog`。規格：§2。
- 刪帳前置條件：帳號列鎖先於買家列；`BeginDeletion`／`CloseForDeletion` 重送安全。守門：`TestDeletionParticipantIdempotency`、`TestCompletePendingChecksOrdersAgain`（accountdeletion）。規格：§2。
- `LockActive` 與 `BeginDeletion` 序列化。守門：`TestLockActiveSerializesWithBeginDeletion`。規格：§2。
- 事件只在真正插入或刪除時發布。守門：`TestIdempotentEvents`。規格：§5。
- 更新以版本條件比對，profile 在買家列鎖內合併。守門：`TestProfileVersionCAS`、`TestAddressVersionCASAndDefaultBumps`、`TestConcurrentProfilePatchesPreserveBothFields`。規格：§3。
- 頭像引用只由 `buyer.media_sync` 寫入，已退場的頭像不能寫回，transaction 內不呼叫 media。守門：`TestRetiredAvatarCannotBeWrittenBack`、`TestSyncBetweenFinalizeAndCommitKeepsThePointedAtAsset`、DB `buyers_avatar_asset_check`。規格：§3。
- 地址最多一個 default，上限 `model.MaxAddresses`。守門：DB partial UNIQUE、`TestAddressDefaultsAndOwnership`、`TestAddressCap`。規格：§4。
- 買家封鎖與店封鎖分兩張表；封鎖買家依 buyer_id 升冪鎖兩列。守門：`TestBuyerAndStoreBlocksWithTheSameIDAreIndependent`、`TestMutualBlocksDoNotDeadlock`、`TestBlockingAClosedBuyerFails`。規格：§6。
- 補貨 `FanOut` 批次有界，發生時間不得為零。守門：`TestRestockBatchesAndRollback`、`TestFanOutRejectsZeroOccurrenceTime`。規格：§7。
- 會員每買家最多一個 ACTIVE，`request_id` 全域 UNIQUE；來源與狀態有 CHECK。守門：DB UNIQUE、`TestMembershipIdempotency`、`TestEnumChecksMatchGo`。規格：§8。
- 公開 ID 為正數、批次不超過 `MaxBatchSize`，在存取資料庫前檢查。守門：`TestInvalidIDsRejectedBeforeDatabaseAccess`、`TestBatchBoundariesRejectedBeforeDatabaseAccess`。規格：§9。

## 不負責

- 店家存在、停權、自我追蹤檢查：`internal/httpapi/buyer` 經店家的 port。
- 未結訂單查詢、帳號刪除編排：`accountdeletion`；各店對買家的封鎖由它刪（seller 擁有）。
- 商品瀏覽的原始事件、彙總與儀表板統計：seller。守門：`TestRecordVisitDoesNotTouchTheProductPort`（httpapi/buyer）。
- 收藏、追蹤、瀏覽、會員卡的顯示補資料：`internal/httpapi/buyer`；事件的 legacy payload 補資料：`internal/legacy`。
- TGOS、Redis、通知 inbox、MemberLevel、會員計費、email 副本：不搬。

## 決策紀錄

從 buyer-service 沿用的每個機制、當時的決定與理由，以及與 buyer-service 的差異在同目錄的 `CARRYOVER.md`；改動或移除沿用的機制前，先讀那一列。相關 ADR：`docs/adr/2026-09-30-account-created-provisions-buyer.md`、`docs/adr/2026-09-30-account-deletion-module.md`、`docs/adr/2026-10-03-media-references-owner-side.md`、`docs/adr/2026-09-18-buyer-membership-entitlement-model.md`。
