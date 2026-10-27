# buyer 模組

買家業務身分與個人資料的擁有者；入口切換依遷移計畫 A3a 步驟 4。

## 職責

- 買家 stint、profile、頭像、收貨地址、收藏、追蹤、個人封鎖、瀏覽、補貨訂閱與刪除（設計書 39）。
- 會員方案、指派與額度週期（設計書 59、ADR 2026-09-18）。
- 同 transaction 發布業務事件（含進入刪除中時的 `BuyerDeletionRequested`、關閉時的 `BuyerDeleted`）；River 釋放頭像與清理 PII。買家不是 identity 的角色，建立與關閉都不授予、移除角色。

## 擁有的資料

schema `buyer`：`buyers`、`shipping_addresses`、`favorites`、`seller_subscriptions`、`blocked_buyers`、`blocked_stores`、`visit_history`、`restock_subscriptions`、`membership_plans`、`memberships`，以及 media 引用的同步狀態 `media_ref_sync` 與退場資產 `retired_assets`。

## 模組內部結構

根套件只組裝、別名與轉呼叫。`internal/profile`、`address`、`favorite`、`follow`、`visit`、`restock`、`membership`、`deletion` 各管自己的 use case；`mediaref` 是頭像引用的同步（退場資產、版本與 `buyer.media_sync` job）；`stint` 共用買家鎖定與狀態檢查，`model` 管型別與輸入共通規則，`store` 是 sqlc 生成物。依賴圖唯一來源是 `internal/arch/modules.go`。

## 不變量

- 帳號活著 ⇔ 買家活著（ADR `2026-09-30-account-created-provisions-buyer`）：買家只在 identity 建帳的 transaction 內由 `OnAccountCreated` 建立，一個帳號一生只有一個買家：DB `UNIQUE (account_id)`，重送或關閉後再建都回錯誤讓建帳回滾，不重讀、不補建。買家只由 `accountdeletion` 隨帳號刪除關閉（本模組不訂閱 `AccountDeleted`，沒有第二條關閉路徑），所以存活帳號查不到現任買家是不變量被破壞，呼叫方回 internal。`TestAccountCreatedCreatesTheOnlyBuyer` 驗證。
- CLOSED 必有 deleted_at，PII 只能清 CLOSED；未清 PII 的暱稱必須 1–100 字元，清理後為 NULL：DB CHECK、關閉 use case 與清理條件共同保證。`TestPIICleanupStintsAndBacklog`、`TestSchemaConstraints` 驗證。
- 地址最多一個 default：DB partial UNIQUE；寫入先鎖買家，新增首筆自動預設、修改保留原預設、刪除後按 updated_at/address_id 遞補。`TestAddressDefaultsAndOwnership` 同時驗證寫入與 DB 約束。
- profile、地址與會員方案各自持有版本；更新以 SQL 條件式版本比對，預設地址切換會遞增受影響地址的版本。`TestProfileVersionCAS`、`TestAddressVersionCASAndDefaultBumps`、`TestPlanVersionCAS`、`TestAvatarVersionCAS`、`TestAddressKeysetPaging` 驗證。
- 收藏、追蹤是複合鍵；事件只在真正插入／刪除時發布。pending 不可新增；`TestIdempotentEvents`、`TestDeletionLifecycle` 驗證。
- 個人封鎖（遷移計畫 A7-1b）分兩張表：`blocked_buyers`（買家封鎖買家，主鍵 (blocker_buyer_id, blocked_buyer_id)，兩邊都 `REFERENCES buyers`，CHECK 不能封鎖自己，反向索引 (blocked_buyer_id, blocker_buyer_id)）與 `blocked_stores`（買家封鎖店，主鍵 (blocker_buyer_id, store_id)，store_id 在 seller、不建跨模組外鍵，反向索引 (store_id, blocker_buyer_id)）。兩種對象是兩個號碼空間，分表後不可能把買家 7 當成店 7。API 兩組：`BlockBuyer`／`UnblockBuyer`、`BlockStore`／`UnblockStore`（冪等，回「有沒有改變」，不發布事件；封鎖要發起者營運中，解除不用；封鎖買家檢查對象存在且沒關閉，封鎖自己回 InvalidInput；店是否存在 buyer 不檢查，不依賴 seller）、`ListBlockedBuyers`／`ListBlockedStores`（keyset，新的在前）、`IsBlockingStore`（買家是否封鎖店）、`IsBlockedBetween`（兩位買家任一方向），後兩者給 A6 的 im、review 與 A7-3 的通知過濾；social 的查詢 join `pub_blocked_stores()`、`pub_blocked_buyers()`。買家刪帳時（`CloseForDeletion`，`internal/deletion`）同 transaction 刪他發起的兩種封鎖與以他為對象的 `blocked_buyers` 列；各店對他的封鎖在 seller（由 accountdeletion 刪，見該模組）；以店為對象的列在店退場後保留。封鎖買家在同一個 transaction 依 buyer_id 升冪鎖兩列（發起者 `LockBuyer`、對象 `LockBuyerShared`），鎖到後才檢查狀態：與對象刪帳並發不會留下指向已關閉買家的列，兩人同時互相封鎖不會互等；封鎖店只鎖發起者。`TestBlockAndUnblockAreIdempotent`、`TestBlockRejectsSelfAndInvalidTargets`、`TestBuyerAndStoreBlocksWithTheSameIDAreIndependent`、`TestIsBlockedBetweenCoversBothDirections`、`TestListBlockedBuyersAndStoresPaginateNewestFirst`、`TestClosingABuyerDeletesBlocksInBothDirections`、`TestMutualBlocksDoNotDeadlock`、`TestBlockingAClosedBuyerFails` 驗證。
- `BeginDeletion` 重送不重複寫入，`CloseForDeletion` 可關閉 ACTIVE／PENDING 且重送安全。`TestDeletionParticipantIdempotency` 驗證。
- 補貨僅存待通知關聯，PK (buyer_id, product_id)；取消與通知後刪除。FanOut 每次獨立 transaction 最多 limit 筆（1..100），回 more 供步驟 4 job 分批循環；刪除與事件原子提交。`TestRestockBatchesAndRollback` 驗證。
- 會員每 stint 最多一個 ACTIVE（partial UNIQUE），request_id 必填且全域 UNIQUE（1–100 字元）：DB UNIQUE；指派鎖買家，request_id 的 transaction advisory lock 序列化跨買家重試，並比對 buyer、plan、期間、操作者與 note。`TestMembershipIdempotency` 驗證。
- gender、狀態與會員 source 有 CHECK（ADMIN 為唯一儲存來源；SYSTEM_DEFAULT 僅在記憶體合成權益）；寫入驗證與受限轉移，`TestSchemaConstraints`、`TestEnumChecksMatchGo` 與純函式驗證測試覆蓋。
- transaction 外才呼叫 media，而且只有 `Finalize`（轉正，不建立引用）。頭像引用只由 `buyer.media_sync` 同步 job 寫入（`internal/mediaref`，演算法是 `media.Sync`）：換頭像在版本條件的 transaction 內寫 object key 與 asset ID（`buyers_avatar_asset_check` 成對）、把被換掉的頭像記成退場（`retired_assets`）、遞增 `media_ref_sync.seq` 並排入同步；關閉時把目前的頭像記成退場（關閉的買家沒有目前的資產，PII 清理不再另外處理引用）。已退場的頭像不能寫回：同一個 staged token 重新轉正會回到同一個資產，回 InvalidInput。job 以 (ref_type, ref_id) 去重（含執行中），Link 買家列目前的頭像、精確 Unlink 退場的資產，結算時 seq 變了就再跑一輪。`TestAvatarOrder`、`TestSyncBetweenFinalizeAndCommitKeepsThePointedAtAsset`、`TestRetiredAvatarCannotBeWrittenBack`、`TestLateLinkAfterUnlinkIsHealedByTheNextSync`、`TestClosingRetiresTheAvatarAndSyncUnlinksIt` 驗證；決策見 `docs/adr/2026-10-03-media-references-owner-side.md`。
- pending 僅禁止新增收藏與追蹤；profile、頭像、地址、瀏覽、補貨與會員仍可寫。`TestProfilePartialUpdateAndPendingWrites` 驗證。
- profile 在 buyer 列鎖內合併；nil 保留，DisplayName 空字串拒絕，Phone/Gender/City/Job 空字串與 BirthDate 零值清為 NULL。`TestConcurrentProfilePatchesPreserveBothFields` 驗證不遺失並行更新。
- `BeginDeletion`／`CloseForDeletion` 的前置條件：只由 `accountdeletion` 在已以 `identity.LockAccount` 鎖住帳號列的 transaction 內呼叫，鎖順序是帳號列先於買家列；本模組只鎖買家列，不自己鎖帳號。刪帳時買家一定關閉由 `accountdeletion` 的 `TestRequestClosedDeletesAccountAndCompletes`、`TestCompletePendingChecksOrdersAgain` 驗證。
- `LockActive(ctx, buyerID)`：給其他模組的寫入路徑在自己的 transaction 內以共享鎖確認買家是 ACTIVE，鎖持有到 transaction 結束；`BeginDeletion` 的排他鎖要等它，所以申請刪除之前成立的寫入一定先於清除 job 提交，之後才開始的回 FailedPrecondition，已關閉回 NotFound。沒有 transaction 時回錯誤。鎖順序：帳號列、買家列、呼叫方自己的表（social 的買家寫入與清除 job 都照這個順序）。`TestLockActiveSerializesWithBeginDeletion` 驗證。
- PII 啟動即排程、每小時補跑，以 ByPeriod=time.Hour 去重；completed job 留存不阻擋下一小時。只清嚴格超過 30 天的 CLOSED stint；排程設定、River 去重與 30 天邊界皆有測試。
- 公開 ID 必須為正數；批次 ID 上限 `MaxBatchSize`，只在模組檢查（handler 只解析 ID），空批次允許；清單上限 100。`TestInvalidIDsRejectedBeforeDatabaseAccess`、`TestBatchBoundariesRejectedBeforeDatabaseAccess` 驗證邊界。
- 每個買家的地址數以 `model.MaxAddresses` 為上限：新增時在買家列鎖內計數，超過回 FailedPrecondition（要先刪掉一筆，〈API 設計規範（Connect）〉§八）；地址清單的預設頁大小等於上限，一頁就是完整清單。`TestAddressCap` 驗證。
- 補貨 `FanOut` 的輸入是必填的 `FanOutInput`（商品、顯示快照、事件發生時間，全部來自上游事件）；發生時間為零回 InvalidInput，不以處理當下的時間代替。`TestFanOutRejectsZeroOccurrenceTime` 驗證。

## 已知限制

- Java 仍存 BIGINT，buyer_id/address_id 暫用 IDENTITY；最後數字 ID 消費端改寫時依 row-identity ADR 換 UUID（階段 C）。
- HTTP 路由屬於 httpapi；legacy gRPC（`internal/legacy/buyerserver`）、RabbitMQ relay 與 consumer 在 `internal/legacy`，隨對應的舊服務搬進單體刪除。
- social-service（A7）的 GetSubscribersList 由 legacy adapter 以有界 offset 轉接（見 `CARRYOVER.md`）；模組提供 keyset Subscribers 與 SubscriberCounts。gateway buyer REST 不由 legacy server 承接，不新增 offset 或 GetSavedItems 形狀。
- 刪帳由 `accountdeletion` 編排（seller 也是參與者）；buyer 只提供 `BeginDeletion`／`CloseForDeletion`，不刪帳號。
- media 在 B1 前是遠端；引用的登記與解除只由同步 job 做（上一條），寫入與同步的先後靠 seq 與退場名單、不靠時間。唯一依賴時間的是新資產要在 media GC 的 7 天寬限（從資產建立起算）內被 Link，重試用盡或 job 遺失由 `buyer.media_sweep` 補排，監控訊號見 ADR。換圖禁止在呼叫者 transaction 內開始（轉正是外部呼叫）。B1 之後引用與業務列同一個 transaction，同步 job 與退場表一起消失。
- 會員方案的種子資料是一支資料 migration（`*_buyer_membership_plan_seeds.sql`）。額度與價格沿用設計書 59 的 placeholder，正式開放方案前由營運定案（後台的 `UpdateMembershipPlan` 可改）。

## 發布的讀取介面（pub_*）

形狀宣告在 `db/pub.sql`（依賴表允許的上層模組的 sqlc 讀它，`internal/arch` 的 `TestSQLCReadsOnlyItsOwnSchemaAndAllowedPubContracts` 擋住讀內部表），定義在 `db/objects.sql` 與手寫的 migration，`cmd/hawkr` 的 `TestPubContractsMatchMigrations` 比對兩者。`pub_inactive_buyer` 只是函式回傳列的形狀，消費端的 query 不得直接參照它，只能呼叫函式。改 `pub_*` 的形狀等於改契約：同一次變更內更新 `pub.sql` 與所有消費端。

- `buyer.pub_inactive_buyers() RETURNS SETOF buyer.pub_inactive_buyer (buyer_id bigint)`：此刻不是營運中的買家（`status <> 'ACTIVE'`，刪除中與已關閉）。申請刪除起就算（刪帳申請不能撤回，只有完成），買家寫的內容從那一刻起不公開；買家的狀態不隨時間變化，所以沒有時間參數。用法（social）：social 在 `BuyerDeletionRequested`／`BuyerDeleted` 的同 transaction 訂閱者把這個事實複製進自己的 `inactive_authors` 並讓內容重新求值（可見性與計數立即改好），讀取與 `reevaluate_*` 只讀複本；`pub_inactive_buyers` 只給 social 每日對帳的 Go 查詢核對複本是否漏了，不給 trigger 或旗標的求值呼叫。`TestInactiveBuyersAndDeletionRequestedEvent` 驗證。
- `buyer.pub_blocked_buyers() RETURNS SETOF buyer.pub_blocked_buyer (blocker_buyer_id bigint, blocked_buyer_id bigint)` 與 `buyer.pub_blocked_stores() RETURNS SETOF buyer.pub_blocked_store (blocker_buyer_id bigint, store_id bigint)`：買家封鎖買家、買家封鎖店的現況，一列一個封鎖，強一致、沒有時間參數、不建投影；兩種對象各一個函式，不共用 kind 欄位。用法（social，A7-1b 第二部分）：公開可讀述詞內以 `EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() b WHERE b.blocker_buyer_id = $1 AND b.store_id = $2)` 檢查觀看者封鎖了店，以兩個 `pub_blocked_buyers()` 的 EXISTS（兩個方向）檢查兩位買家之間的封鎖；函式會被內聯，走各表的主鍵或反向索引。`TestPubBlocksIsInlinedAndUsesTheIndexes` 驗證。
- `BuyerDeletionRequested` 在 `BeginDeletion` 把買家從 ACTIVE 轉成 PENDING_DELETION 的同一個 transaction 發布一次（重送、之後的關閉都不再發）；訂閱者只該排入自己的清除 job（單體規則 4）。

## 讀取模型

清單只回 ID、時間；BatchDisplay 對 CLOSED 回 Deleted=true、名稱與頭像為 nil，BatchMeta 與單筆 profile 不曝露 CLOSED PII。ResolvePrincipals 僅回存活 stint，CLOSED／未知 ID 不在 map，legacy adapter 由差集產生 unresolved_ids。BuyerIDsByAccount 最多回一個（含已關閉）。墓碑文字由 client（Connect 卡片的 `deleted`）與 legacy adapter（步驟 4）呈現，模組不存或回傳墓碑字串。粉絲統計 `SubscriberStats(storeID, windows)` 一句查詢回總數與每個時間窗 [start, end) 的新增追蹤數（1 到 4 個窗，同一個 snapshot）；期間的界線由呼叫者決定（儀表板是台北的完整日，legacy 的舊儀表板自己算台北週日起的週），百分比由呈現層計算。

## 不負責

- 店家存在、停權、自我追蹤檢查：`internal/httpapi/buyer` 經店家的 port 檢查。
- 未結訂單查詢、帳號刪除編排：`accountdeletion` 先查訂單，再呼叫本模組的 `BeginDeletion`／`CloseForDeletion`。
- 商品瀏覽數的原始事件、彙總與儀表板統計：seller（`RecordProductView`、`IngestProductView`、`ProductViewsBetween`）；匿名打點不經 buyer。`RecordVisit(buyerID, productID)` 不帶店家：buyer 對商品沒有依賴，請求路徑上不解析商品；`ProductViewed` 只帶 `EventID`、`OccurredAt`、`BuyerID`、`ProductID`，seller 在背景才解析店家（見 seller 的 CARRYOVER）。`internal/httpapi/buyer` 的 `TestRecordVisitDoesNotTouchTheProductPort` 與 `cmd/hawkr` 的 `TestLoggedInProductViewSucceedsWithoutProductServiceAndIsRolledUpLater` 驗證。
- 收藏／追蹤／瀏覽／會員卡／消費紀錄的顯示補資料：`internal/httpapi/buyer`。事件 legacy payload 補資料：步驟 4。
- TGOS、Redis、通知 inbox、MemberLevel、會員計費、email 副本不搬（計畫決定 9a、10a、D8）。

## 決策紀錄

從 buyer-service 沿用的每個機制、當時的決定與理由，以及與 buyer-service 的差異在同目錄的 `CARRYOVER.md`。改動或移除沿用的機制前，先讀那一列。

## 特性測試

不採用 D4 的雙跑特性測試（計畫決定 11）：入口、資料模型與刻意修正的缺陷不同，兩側不應得到相同結果。由 `buyer_integration_test.go`、`invariants_integration_test.go`、`lifecycle_integration_test.go`、`version_integration_test.go` 覆蓋建帳時建立買家、關閉的副作用、PII 邊界、部分更新、版本衝突、地址分頁與預設切換、補貨及跨買家會員冪等；方案測試讀 migration 套用的種子資料。pure validation、月週期與排程設定用單元測試；River integration 驗證跨去重時間窗。Connect handler 的測試（授權、etag、越權、分頁、必填欄位、刪帳）在 `internal/httpapi/buyer` 與 `internal/httpapi/identity`；步驟 4 的驗收包含 compose legacy 測試。
