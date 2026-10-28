# buyer 模組行為規格

本文是單體 `buyer` 模組行為的唯一來源。買家業務身分與個人資料的擁有者。

## 1. 範圍與公開面

- 買家 stint、profile、頭像、收貨地址、收藏、追蹤（賣家訂閱）、個人封鎖、瀏覽、補貨訂閱與刪除（設計書 39 已退場，只留指引）。
- 會員方案、指派與額度週期（設計書 59、ADR `2026-09-18-buyer-membership-entitlement-model`）。
- 業務事件在同一個 transaction 內發布（含進入刪除中時的 `BuyerDeletionRequested`、關閉時的 `BuyerDeleted`）；River 釋放頭像與清理 PII。
- 買家不是 identity 的角色：建立與關閉買家都不授予、移除角色，也不自己鎖帳號。
- buyer_id、address_id 是資料庫發號的 IDENTITY 整數；membership 是 UUIDv7；收藏、追蹤、補貨等關聯用複合自然鍵（ADR `2026-09-25-row-identity-by-data-kind`）。
- HTTP 路由屬於 httpapi。legacy gRPC（`internal/legacy/buyerserver`）、RabbitMQ relay 與 consumer 在 `internal/legacy`，服務還沒搬進單體的舊服務經它們呼叫 buyer。

## 2. 買家的建立與生命週期

- 帳號活著 ⇔ 買家活著（ADR `2026-09-30-account-created-provisions-buyer`）。買家只在 identity 建帳的 transaction 內由 `OnAccountCreated` 建立；一個帳號一生只有一個買家（DB `UNIQUE (account_id)`）。重送，或買家關閉後再建，都回錯誤讓建帳回滾，不重讀、不補建。
- 買家只由 `accountdeletion` 隨帳號刪除關閉；本模組不訂閱 `AccountDeleted`，沒有第二條關閉路徑。存活帳號查不到現任買家是不變量被破壞，呼叫方回 internal。
- 狀態有 ACTIVE、PENDING_DELETION、CLOSED。CLOSED 必有 `deleted_at`；PII 只能清 CLOSED。
- `BeginDeletion` 把買家從 ACTIVE 轉成 PENDING_DELETION，並在同一個 transaction 發布 `BuyerDeletionRequested` 一次；重送不重複寫入，之後的關閉不再發。訂閱者只該排入自己的清除 job。刪帳申請不能撤回，只有完成。
- `CloseForDeletion` 可關閉 ACTIVE 或 PENDING_DELETION，重送安全。關閉時刪除收貨地址並逐筆發 `FavoriteRemoved`，刪除買家發起的兩種封鎖與以他為對象的 `blocked_buyers` 列，把目前的頭像記成退場，發布 `BuyerDeleted`。
- 前置條件：`BeginDeletion` 與 `CloseForDeletion` 只由 `accountdeletion` 在已以 `identity.LockAccount` 鎖住帳號列的 transaction 內呼叫。鎖順序是帳號列先於買家列；本模組只鎖買家列。刪帳時買家一定關閉。
- 刪帳由 `accountdeletion` 編排（seller 也是參與者）；buyer 只提供 `BeginDeletion` 與 `CloseForDeletion`，不刪帳號。
- `LockActive(ctx, buyerID)` 給其他模組的寫入路徑，在自己的 transaction 內以共享鎖確認買家是 ACTIVE，鎖持有到 transaction 結束。`BeginDeletion` 的排他鎖要等它：申請刪除之前成立的寫入一定先於清除 job 提交；之後才開始的回 FailedPrecondition；已關閉回 NotFound；沒有 transaction 時回錯誤。鎖順序是帳號列、買家列、呼叫方自己的表。
- pending 期間只禁止新增收藏與追蹤；profile、頭像、地址、瀏覽、補貨與會員仍可寫。
- PII 清理：River 在啟動時排程並每小時補跑，以 `ByPeriod = time.Hour` 去重，completed job 留存但不阻擋下一小時。只清嚴格超過 30 天的 CLOSED stint，分批處理到積壓清完。未清 PII 的暱稱必須 1–100 字元，清理後為 NULL。

## 3. 個人資料與頭像

- 更新以 SQL 條件式版本比對；profile、地址與會員方案各自持有版本。
- profile 在買家列鎖內合併：nil 保留，`DisplayName` 空字串拒絕，Phone、Gender、City、Job 空字串與 BirthDate 零值清為 NULL。並行的 profile 更新不遺失任何欄位。
- gender、買家與會員狀態、會員 source 有 CHECK；寫入驗證與受限轉移由純函式驗證涵蓋。會員 source 的唯一儲存值是 ADMIN，SYSTEM_DEFAULT 只在記憶體合成權益。
- 頭像引用只由 `buyer.media_sync` 同步 job 寫入（`internal/mediaref`，演算法是 `media.Sync`，規格見 `docs/modules/media.md` §5）。
- transaction 外才呼叫 media，而且只有 `Finalize`（轉正，不建立引用）。換圖禁止在呼叫者 transaction 內開始。
- 換頭像在版本條件的 transaction 內寫 object key 與 asset ID（`buyers_avatar_asset_check` 要求成對），把被換掉的頭像記成退場（`retired_assets`），遞增 `media_ref_sync.seq` 並排入同步。
- 已退場的頭像不能寫回：同一個 staged token 重新轉正會回到同一個資產，回 InvalidInput。
- 關閉買家時把目前的頭像記成退場；關閉的買家沒有目前的資產，PII 清理不再另外處理引用。
- 同步 job 以 `(ref_type, ref_id)` 去重（含執行中）：Link 買家列目前的頭像，精確 Unlink 退場的資產，結算時 seq 變了就再跑一輪。寫入與同步的先後靠 seq 與退場名單，不靠時間。
- 唯一依賴時間的是新資產要在 media GC 的 7 天寬限（從資產建立起算）內被 Link。重試用盡或 job 遺失由 `buyer.media_sweep` 補排，監控訊號在 ADR `2026-10-03-media-references-owner-side`。

## 4. 收貨地址

- 每個買家最多一個 default 地址（DB partial UNIQUE）。寫入先鎖買家：新增首筆自動成為預設，修改保留原預設，刪除預設後按 `updated_at`、`address_id` 遞補。
- 預設地址切換會遞增受影響地址的版本。清單用 keyset 分頁。
- 每個買家的地址數以 `model.MaxAddresses`（20）為上限：新增時在買家列鎖內計數，超過回 FailedPrecondition（要先刪掉一筆，〈API 設計規範（Connect）〉§八）。地址清單的預設頁大小等於上限，一頁就是完整清單。
- 收貨地址以內政部 8 碼鄉鎮市區代碼儲存（ADR `2026-10-04-address-town-code`）。

## 5. 收藏與追蹤

- 收藏與追蹤是複合鍵。事件只在真正插入或刪除時發布，重送不重複發。pending 的買家不可新增。
- 追蹤的粉絲清單由模組提供 keyset `Subscribers` 與 `SubscriberCounts`。
- 粉絲統計 `SubscriberStats(storeID, windows)` 一句查詢回總數與每個時間窗 [start, end) 的新增追蹤數：1 到 4 個窗，同一個 snapshot。期間的界線由呼叫者決定，百分比由呈現層計算。

## 6. 個人封鎖

- 兩張表。`blocked_buyers`：買家封鎖買家，主鍵 (blocker_buyer_id, blocked_buyer_id)，兩邊都 `REFERENCES buyers`，CHECK 不能封鎖自己，反向索引 (blocked_buyer_id, blocker_buyer_id)。`blocked_stores`：買家封鎖店，主鍵 (blocker_buyer_id, store_id)，`store_id` 在 seller、不建跨模組外鍵，反向索引 (store_id, blocker_buyer_id)。兩種對象是兩個號碼空間。
- API 兩組：`BlockBuyer`／`UnblockBuyer`、`BlockStore`／`UnblockStore`，冪等，回「有沒有改變」，不發布事件。封鎖要發起者營運中，解除不用。封鎖買家檢查對象存在且沒關閉，封鎖自己回 InvalidInput。店是否存在 buyer 不檢查，不依賴 seller。
- `ListBlockedBuyers`／`ListBlockedStores` 用 keyset，新的在前。`IsBlockingStore` 回買家是否封鎖店；`IsBlockedBetween` 回兩位買家任一方向是否封鎖。social 的查詢 join `pub_blocked_stores()`、`pub_blocked_buyers()`。
- 買家刪帳時（`CloseForDeletion`，`internal/deletion`）同 transaction 刪他發起的兩種封鎖與以他為對象的 `blocked_buyers` 列。各店對他的封鎖在 seller（由 accountdeletion 刪）。以店為對象的列在店退場後保留。
- 封鎖買家在同一個 transaction 依 buyer_id 升冪鎖兩列（發起者 `LockBuyer`、對象 `LockBuyerShared`），鎖到後才檢查狀態：與對象刪帳並發不會留下指向已關閉買家的列，兩人同時互相封鎖不會互等。封鎖店只鎖發起者。

## 7. 瀏覽與補貨

- `RecordVisit(buyerID, productID)` 不帶店家，請求路徑上不解析商品，每次瀏覽都發 `ProductViewed`。事件只帶 `EventID`、`OccurredAt`、`BuyerID`、`ProductID`；店家由 seller 在背景解析。商品瀏覽的原始事件、彙總與儀表板統計屬 seller（`RecordProductView`、`IngestProductView`、`ProductViewsBetween`）；匿名打點不經 buyer。
- `visit_history` 每個買家與商品一列，存最後瀏覽時間，不設筆數或期限。
- 補貨只存待通知關聯，PK (buyer_id, product_id)；取消與通知後刪除，沒有終態歷史。
- `FanOut` 每次獨立 transaction 最多處理 limit 筆（1..100），回 `more` 供呼叫的 job 分批循環；刪除與事件原子提交。輸入是必填的 `FanOutInput`（商品、顯示快照、事件發生時間，全部來自上游事件）；發生時間為零回 InvalidInput，不以處理當下的時間代替。

## 8. 會員

- 方案表 `membership_plans`，指派表 `memberships`。每個買家最多一個 ACTIVE 會員（partial UNIQUE）。
- `request_id` 必填且全域 UNIQUE（1–100 字元）。指派鎖買家；以 `request_id` 的 transaction advisory lock 序列化跨買家重試，並比對 buyer、plan、期間、操作者與 note。
- 會員月週期以錨點月 clamp，不回補；人工指派用明確天數；付費過期回 free。時間由注入的 clock 提供。
- 會員方案的種子資料是一支資料 migration（`*_buyer_membership_plan_seeds.sql`）。額度與價格是設計書 59 的 placeholder 值，後台的 `UpdateMembershipPlan` 可修改。

## 9. 讀取模型與輸入邊界

- 清單只回 ID 與時間。`BatchDisplay` 對 CLOSED 回 `Deleted=true`、名稱與頭像為 nil；`BatchMeta` 與單筆 profile 不曝露 CLOSED 的 PII。
- `ResolvePrincipals` 只回存活 stint；CLOSED 與未知 ID 不在 map，legacy adapter 由差集產生 `unresolved_ids`。`BuyerIDsByAccount` 最多回一個（含已關閉）。
- 墓碑文字由 client（Connect 卡片的 `deleted`）與 legacy adapter 呈現，模組不存或回傳墓碑字串。
- 公開 ID 必須為正數；批次 ID 上限 `MaxBatchSize`（100），空批次允許，只在模組檢查（handler 只解析 ID）；清單上限 100。檢查在存取資料庫之前。
- 事件原子發布，帶 UUIDv7 與注入 clock 的 `OccurredAt`。

## 10. 公開讀取介面（pub_*）的語意

形狀宣告在 `db/pub.sql`，定義在 `db/objects.sql` 與手寫的 migration；形狀的清單在 `AGENTS.md`。

- `pub_inactive_buyers()`：此刻不是營運中的買家，申請刪除起就算，買家寫的內容從那一刻起不公開。social 在 `BuyerDeletionRequested`、`BuyerDeleted` 的同 transaction 訂閱者，把這個事實複製進自己的 `inactive_authors` 並讓內容重新求值，讀取與 `reevaluate_*` 只讀複本。函式只給 social 每日對帳的 Go 查詢核對複本是否漏了，不給 trigger 或旗標的求值呼叫。
- `pub_blocked_buyers()`、`pub_blocked_stores()`：封鎖的現況，一列一個封鎖，強一致，沒有時間參數，不建投影；兩種對象各一個函式，不共用 kind 欄位。social 在公開可讀述詞內以 `EXISTS (SELECT 1 FROM buyer.pub_blocked_stores() b WHERE b.blocker_buyer_id = $1 AND b.store_id = $2)` 檢查觀看者封鎖了店，以兩個 `pub_blocked_buyers()` 的 EXISTS（兩個方向）檢查兩位買家之間的封鎖。函式會被內聯，走各表的主鍵或反向索引。

## 11. 已知限制

- buyer_id 與 address_id 是數字 ID，不是 UUID。
- social-service 的 `GetSubscribersList` 由 legacy adapter 以有界 offset 轉接（`SubscribersOffset`）。gateway buyer REST 不由 legacy server 承接，不提供 offset 或 `GetSavedItems` 形狀。
- 頭像引用的登記與解除只由同步 job 做，media 是遠端服務，所以換圖不能在呼叫者 transaction 內開始。
- 會員方案的額度與價格是 placeholder 值。
