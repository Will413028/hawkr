# coupon 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

## 沿用盤點

coupon-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| ID 方案：TSID（`BIGINT`，應用層產生） | 跨進程產生不撞號 | 不成立：沒有 node 位元，多 replica 會撞（主計畫 0.2 ID 策略） | 實體用應用層 UUIDv7，關聯表用複合鍵（ADR row-identity-by-data-kind） | 活動、券主鍵 UUIDv7（`Deps.NewID`）；資格名單用 `(campaign_id, buyer_id)`；稽核紀錄的 id 由 DB 的 `uuidv7()` 產生（只追加） |
| 兩張表：`coupon_templates`＋`coupon_instances`，instance 在結帳 reserve 時才建，另有 `coupon_claims` | 領取、發放、鎖定各自要有記錄 | 不成立：BD3、BD4、BD6、BD18 都來自增量長出的結構 | 兩層模型：活動規則＋買家持有的券；領取時就產生券（D2） | `coupon_campaigns`＋`coupon_vouchers`；領券、指定發放、補償都只是建立 voucher；`coupon_claims` 併入 voucher（`UNIQUE (campaign_id, buyer_id)`） |
| `claim_required` 旗標（賣家與補償強制 FALSE，不用領就能在結帳用） | 區分要不要先領 | 不成立：D1 規定 client 先領券，轉接層對不到券回「未領取」 | 券都在錢包裡，client 只傳券 id | 刪除旗標；賣家 DESIGNATED 活動由買家（在名單內）領取或 admin 指定發放 |
| 數字字串狀態碼（`'0'`–`'5'`）與 CodeMeta 的 `CouponInstanceStatus`、`CouponTemplateStatus` | 舊契約的代碼管理 | 不成立：CodeMeta 凍結（ADR codemeta-retirement）；沒有 CHECK | text＋CHECK，Go 常數與 proto 值名同形（UPPER_SNAKE） | 券 `AVAILABLE/LOCKED/REDEEMED/VOIDED`；活動 `DRAFT/ACTIVE/ENDED`；EXPIRED 不是狀態（下一列）；各表有 CHECK 與狀態對欄位的互斥檢查 |
| EXPIRED 狀態與 `AutoExpireJob`（每 N 分鐘翻狀態） | 讓查詢能依狀態過濾過期 | 不成立：BD18、BD25；過期可從 `end_at` 推導 | 到期是推導出來的，排程只做到期前提醒 | 刪除狀態與 job；`Campaign.Expired(now)`；時間窗在 `qualify.Check` |
| 模板狀態 `INACTIVE`＋`disabled_by_suspension`（停權停用、解除時精準還原） | 賣家停權時可逆地停用其活動 | 成立（停權期間不得領、鎖）；狀態欄位與事件訂閱不成立：停權狀態由 seller 擁有，可即時查詢 | 資格判定當下查店家狀態，不複製狀態 | 不沿用狀態與欄位（Will 2026-11-23，計畫 D8 修訂）：schema 沒有停權狀態；`qualify.Check` 對賣家發行的活動問 `Stores` port，停權期間 Claim、Lock（含 Quote）回 `COUPON_STORE_SUSPENDED`，解除後自動恢復，平台（admin）活動不受影響。Redeem、Release 不查停權：已鎖的券照常完成（與 D5 一致）。組裝點接 seller 的 `Store(ctx, id).Suspended()`（arch 允許 coupon→seller），B7-3 接線 |
| `usage_limit_per_account`（存了沒有執行） | 每人領取上限 | 不成立：沒有執行邏輯、沒有 client 設定（BD16）；實際限制是 `(template, owner)` 唯一索引＝每人 1 張 | 每人上限用 UNIQUE 或條件式檢查 | 刪除欄位；每位買家每個活動至多一張，`vouchers_campaign_buyer_key`（已核銷與已作廢的也占名額） |
| `total_issue_limit = -1` 表示無限，`issued_count` 在領取後遞增、回傳值被忽略 | 總量上限 | 成立；寫法不成立：BD4、BD5、BD14（`-1` 魔術值） | `UPDATE … SET issued = issued + 1 WHERE issued < limit`；無上限用 NULL | `total_limit` NULL＝無上限（上限至少 1，與「上限 0」分開）；`IncrementIssued` 條件式 UPDATE，回 0 列＝`ReasonIssueLimitReached`；`campaigns_issued_count_check` 當後備 |
| 資格判定 `isEligible`（只在領券呼叫）與 `selectAvailableForBuyer`（列表的另一份等價 SQL） | 領取與使用時判斷買家是否符合 | 判斷成立；兩份實作不成立：BD1、BD2、BD11 | 資格只有一份判定，領取與套用都呼叫 | `qualify.Check`（狀態、時間窗、資格）＋`qualify.Fit`（店家範圍、門檻、折抵）；`Claim`、`Lock`、`Quote` 共用；列表（B7-3）不另寫第二份規則 |
| ALL 資格：買家註冊時間 ≤ 模板建立時間 | 防止新註冊帳號領取既有的全站券（推測的原意，Java 沒有文件） | 不成立：沒有文件說明原意；Java 在查不到註冊時間時兩條路徑行為相反（BD11） | 業界的全站券通常不看註冊時間，新客券另做一種 | 不沿用（Will 2026-11-23：ALL means all buyers）：`TargetAll` 一律符合，刪除 `Buyers` port 與註冊時間判定 |
| MEMBER 資格：查 Java order 的 `GetBuyerMerchantSummary`，失敗回空集合 | 在該店有消費紀錄才符合 | 資格成立；降級不成立：BD28（order 掛掉時 MEMBER 券整批消失） | 讀取方持有投影，由訂單完成事件餵入（D3） | `Members` port（`ports.go`）；B7-2 只定義 port、測試用假的；投影與事件消費在 B7-4；port 失敗整個請求失敗 |
| DESIGNATED、COMPENSATION：`coupon_target_accounts`；`insertIgnore` 逐筆寫 | 資格名單 | 成立 | 名單表，批次寫入、冪等 | 保留表 `coupon_target_accounts`，`InsertTargetAccounts` 以 `unnest` 一次寫入；刪帳時清名單（BD19）在 B7-4 的訂閱者 |
| `validateCoupon`（不鎖、不建 instance；只在驗券檢查狀態、時間窗、scope、門檻） | 結帳前預覽折抵 | 成立；與 reserve 檢查項目不同不成立：BD1、BD2 | 預覽與套用用同一組檢查 | `Quote` 與 `Lock` 同一組檢查（資格、時間窗、店家範圍、門檻），回傳折抵金額；已由同一筆訂單持有的重送略過資格（D5） |
| `reserveCoupon`：先查再插入、UNIQUE 撞了回系統錯誤、不重驗、`writeLog` 吞例外、`applicableStoreId` 恆 NULL | 一張券同時只被一筆訂單鎖；重送冪等 | 鎖的語意成立；寫法不成立：BD2、BD3、BD6、BD7 | 在呼叫方 transaction 內以條件式 UPDATE 鎖定，冪等 | `Lock`：條件式 `UPDATE … WHERE status = 'AVAILABLE'`；同一筆訂單重送冪等成功，別單 `ReasonVoucherLockedByOtherOrder`；一筆訂單只持有一張券（`vouchers_order_no_key`）；鎖沒有到期時間（D3）；稽核同 transaction |
| `consumeCoupon(instanceId, orderNo)`、`releaseCoupon(instanceId, orderNo)`（Release 也處理 USED→UNUSED，賣家取消已付款訂單） | 付款後核銷；取消、逾時、賣家取消後還券 | 成立；以 instance id 為鍵不成立：D3 A 決定以 `order_no` 為鍵（回應遺失時 Java 不知道 instance id） | 狀態轉換跟著訂單，冪等 | `Redeem(orderNo)`、`Release(orderNo)`：條件式 UPDATE、重送冪等；Release 處理 LOCKED 與 REDEEMED（保留賣家取消已付款訂單還券）；活動已結束時 Release 把券作廢而不是回到可用（D5 的 AVAILABLE 都作廢；共享鎖活動列與結束排隊）；Redeem 不檢查活動狀態與時間（BD8、BD10 到 B8） |
| `AutoReleaseJob` 對帳（每分鐘查 order 狀態補 consume 或 release） | Release 漏送時補救 | 不成立：D3 A 決定不做反查 order 的對帳與推導式釋放；BD25（批次被 orphan 卡住、fail-open） | 釋放跟著訂單狀態轉換在同一個 transaction | 刪除；Java 取消路徑補 Release 與重試在 B7-4；漏送由 admin `Void` 人工處理（可作廢 LOCKED） |
| `revokeByTemplate`（賣家收回：作廢 UNUSED 與 RESERVED） | 賣家停止活動 | 語意不成立：BD8（進行中訂單核銷失敗且無補償） | 只擋新的領取與套用；已鎖定的照常完成（D5） | `EndCampaign`：ACTIVE→ENDED，作廢 AVAILABLE，LOCKED 保留；先鎖活動列，與領券、發券排隊；已結束重送成功；賣家路徑以 `StoreID` 限定自己的活動（別人的回 NotFound） |
| `voidInstance`（admin，只作廢 UNUSED） | 營運作廢單張券 | 不完全成立：BD17、D3（卡在 RESERVED 的券要能人工解除） | 作廢單張券，稽核 | `Void`：AVAILABLE 或 LOCKED 可作廢，已核銷的不行（`ReasonInvalidStatus`）；稽核記 admin 與理由 |
| `BuyerDeletedConsumer`、`SellerSuspended/UnsuspendedConsumer`（MQ） | 刪帳、停權時處理券與活動 | 刪帳處理成立，MQ 不成立：buyer 已在單體（D8）；停權處理不成立：改為資格判定時查 seller（見停權列） | 刪帳訂閱 in-process 事件，同 transaction；停權不訂閱 | 不在 B7-2：B7-4 只訂閱 `BuyerDeleted`；`StoreSuspended`、`StoreUnsuspended` 不訂閱（D8 修訂） |
| 賣家配額：`checkQuota`（count 後比較）與 `updateQuota`（先查後寫） | 限制每家店同時進行的活動數（預設 5） | 成立；寫法不成立：BD15 | 鎖住配額列後檢查與插入串行 | `seller_coupon_quotas`；建立時 `InsertQuotaIfAbsent`＋`LockQuota`（FOR UPDATE）再 `CountActiveSellerCampaigns`；`SetQuota` 是 upsert；預設 `DefaultSellerQuota = 5` |
| 補償券：每次 grant 建一個新模板，`ref_case_no` 只進標題，沒有冪等鍵；賣家補償無到期日（2099）、admin 補償 +3 個月 | 售後與營運補償 | 成立；沒有冪等鍵不成立：BD9 | 以來源事件的身分當冪等鍵（D4） | `IssueCompensation`：一券一活動（總量 1）；活動與券都有 `UNIQUE (source_type, source_id)`，重送回原本的券；到期日由呼叫方給（B7-4 轉接層決定 2099 或 +3 個月）；沿用賣家補償規則（門檻 ≥ 面額 5 倍、與面額差 ≥ 100）；來源種類 `AFTERSALES_CASE`、`ADMIN_COMPENSATION` |
| 建立驗證 `validateTemplateParams`（CASH 要金額、PERCENT 1–99 與上限、start ≤ end）；沒有 CHECK | 規則自洽 | 成立；缺 CHECK 與折抵上限檢查不成立：BD31、BD32 | 輸入檢查加 schema CHECK 當後備 | `CampaignSpec.Validate` 與 CHECK 同一組條件，加上 CASH 折抵不得超過門檻（門檻 0 不套用，Will 2026-11-23）；FREE_SHIPPING 已於 Java V5 移除，不搬 |
| `issuer_type`＋`issuer_account_id`＋`issuer_store_id`（V6 expand，CHECK 留給下一次 contract） | 區分 admin 與賣家發行 | 成立；沒有 CHECK 不成立：BD31 | 以 CHECK 保證互斥 | `issuer_type`＋`issuer_admin_id`＋`issuer_store_id`，`campaigns_issuer_check` 互斥；賣家活動 `store_id = issuer_store_id`（`campaigns_seller_scope_check`）；admin 身分用單體的 `admin_id`（BIGINT） |
| 折抵計算 `calculateDiscount`（CASH：min；PERCENT：amount×percent/100 取上限） | 算折抵金額 | 成立 | 整數運算、金額為新台幣元 | `model.Discount`；`Fit` 限制訂單金額 ≤ `MaxOrderAmount` 避免溢位 |
| `coupon_redemption_logs`：寫入失敗吞掉（BD7）、`instance_id`／`template_id` 未知寫 0、`trace_id`／`request_id` 恆空、`detail VARCHAR(500)` | 稽核 | 稽核成立；寫法不成立：BD7、BD23、BD30 | 稽核與操作同一個 transaction | `coupon_redemption_logs`：同 transaction 寫入、失敗整筆失敗；未知的欄位用 NULL；沒有 trace 欄；理由 ≤ 255 字元；action 的值域有 CHECK |
| Redis 模板快取 `coupon:tpl:*`、`coupon:avail:*` | 降低讀取延遲 | 不成立：同進程直接查 DB；快取失效不完整且有死 key（BD29、BD30） | 沒有量測前不加快取 | 刪除 |
| `LocalDateTime` 讀寫 `TIMESTAMPTZ`、JVM 預設時區的 cron | 時間比較正確 | 不成立：BD26 | UTC、`timestamptz`、注入時鐘 | 全 `timestamptz`；模組不呼叫 `time.Now()`，用 `Deps.Now`；比較都在 Go 以同一個 `now` |
| `updated_at` 觸發器、MyBatis-Plus 整列 `updateById` | 維護更新時間；改狀態 | 不成立：整列覆寫會蓋掉並發遞增（BD5） | 條件式 UPDATE，只改需要的欄 | 沒有觸發器；每個 UPDATE 自己設 `updated_at`；狀態轉換都是條件式 UPDATE |
| 出站 gRPC：Order 的 MEMBER 集合與訂單狀態、Buyer 與 Seller 的顯示資料；失敗回 -1 或空 | 資格與顯示需要別的服務的資料 | 資料需求成立；降級不成立：BD28 | 同進程直接呼叫模組 API，失敗就失敗 | 資格需要的兩項定義成 port（`Stores` 停權狀態、`Members`）；Account 註冊時間不再需要並失敗即回錯誤；顯示資料在 B7-3a 由 `httpapi/coupon` 組裝：店家卡片 `seller.StoreCards`、買家暱稱 `buyer.BatchDisplay`，讀取失敗整個請求失敗（unavailable 或原錯誤）；訂單狀態不需要（D3 不做對帳）；Account 的電話不再顯示（舊明細經 `AccountService` 取電話；CRM 的規則是客戶資料永遠不含電話，賣家用買家 ID 指定名單）；`Stores` 的正式 adapter 在 `cmd/hawkr/coupon.go`（`seller.Store(ctx, id).Suspended()`，讀不到店家就整個失敗）；`Members` 的投影到 B7-4 才有，在那之前 `cmd/hawkr` 接的 port 回錯誤（MEMBER 活動的領取與列表失敗，不降級成「不是會員」） |
| 賣家 DESIGNATED 的自發自用防護（解析 principal 比對） | 賣家不能發券給自己 | 成立 | 身分比對在入口，核心不重複 | B7-3a 完成：在 `httpapi/coupon.CreateCampaign` 比對名單與呼叫者的買家身分（名單的 ID 是買家 ID，與 CRM 客戶的 `buyer_id` 同一種，不再是帳號 ID，不需要 principal 解析），相同回 invalid_argument 並指出 `campaign.designated_buyer_ids`（`TestCreateCampaignRejectsSelfDesignation`） |
| `CouponTargetTypeRules`（賣家只能 ALL、MEMBER、DESIGNATED） | 限制賣家可建立的資格 | 成立 | 權限在入口與核心都檢查 | `CreateCampaign` 只接受 ALL、DESIGNATED（MEMBER 的消費紀錄投影接上之前，入口回 invalid_argument 指向 `campaign.target_type`）；COMPENSATION 只經 `IssueCompensation`；賣家的 DESIGNATED 名單不得為空，而且只能是這家店的 CRM 客戶（Will 2026-11-24，入口以 `crm.GetCustomer` 檢查，不是客戶回 invalid_argument 指向 `campaign.designated_buyer_ids[n]`）；賣家的活動一律是自己的店；店家不能結束補償活動（NotFound） |
| 無 FK（所有關聯靠應用層） | 避免跨服務耦合 | 同 schema 內不成立：沒有理由不用 FK | 同 schema 用 FK，跨 schema 不建（單體規則 2） | 券與資格名單 FK 到活動（同 schema）；沒有跨 schema FK（`buyer_id`、`store_id` 是業務鍵）；稽核紀錄不設 FK（只追加，不擋刪除） |
| `CouponExpiringWarningJob`：每日 01:00（JVM 時區）、`days=1`、`instance_id > cursor` 游標、逐筆發 MQ、publish 失敗被吞（at-most-once） | 到期前提醒買家一次 | 成立；但時區混用（cron 用 JVM 預設、窗用 UTC）、吞錯、游標不成立（游標以後才符合的券永遠漏：進窗時被 LOCKED 後釋放、晚領的券） | 每張券一筆提醒紀錄 `voucher_expiry_reminders`（`voucher_id` 主鍵，Will 2026-11-24），與 `PublishRequired` 同 transaction；River periodic job 每小時以 `FOR UPDATE SKIP LOCKED`＋`LIMIT` 分批；提前量 24h 常數（同 Java）、UTC、注入時鐘；條件：AVAILABLE、未提醒、`end_at` 在 (now, now+24h]、`created_at <= end_at - 24h`（領到時已進窗的不補提醒）；整批失敗逐張重試、單張失敗記錄後跳過，下一輪重試；索引 `campaigns_by_end_at`：掃描先以 `end_at` 短窗找活動、再經 `vouchers_by_campaign` 找券，沒有它得掃全部活動；紀錄不含 `end_at`（`end_at` 沒有修改路徑）；不呼叫 Buyer.ResolvePrincipals（帳號解析歸 notification） |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 認證與身分：`GrpcAuthContext.requireAccountType`、`parseLong` 吞格式錯誤（BD14、BD24） | 只有對的角色與帳號能呼叫 | 成立；吞錯誤不成立 | Connect 入口依 session 取身分，非法值回 InvalidArgument | B7-3a 完成：Connect 入口依 session 取買家（`CurrentBuyerID`）或店家（`seller.ActingStore`，能力由 `httpapi/coupon` 的宣告表決定：建立 CAP_MARKETING，列表、明細、結束活動 CAP_CONTENT_CONTRACT：停權的店家要能找出活動 ID 並結束它，同 social 的讀取宣告）；ID 格式錯誤回 invalid_argument 並指出欄位（`rpc.ParseID`、`ParseUUID`），不吞成 0；核心的輸入都是已解析的型別，核心自己檢查正數與長度（`Invalid`） |
| 錯誤：envelope `code`＋`GrpcResponseCode` 7xxx，兩種契約並存（BD22） | client 區分失敗原因 | 不成立：兩種契約並存 | 分類錯誤加 `ErrorInfo.reason`（ADR api-error-model） | 核心回 `apperr` 分類加 `model.Reason`；B7-3a 完成：對外的 `ErrorReason` 與 CodeMeta 錯誤碼去向見下方「CodeMeta 值域與錯誤碼的去向」；錯誤句子依請求語系產生，`LocalizedMessage.locale` 取錯誤實際的語系（`apperr.Error.Locale()`，測試 `TestErrorLocaleFollowsTheRequestLocale`） |
| 分頁：SQL 取一頁後在記憶體過濾，`total` 是該頁筆數（BD12） | 列表分頁與總數正確 | 不成立 | keyset 或 SQL 內分頁 | B7-3a 完成：`internal/read` 的 keyset（`created_at`、`id` 由新到舊），分頁與狀態篩選都在 SQL 內，page token 綁住篩選；結帳與可領列表需要 `qualify` 判定的，批次取出後在 Go 以同一個 `Check`／`Fit` 過濾直到湊滿一頁；契約沒有總數（AIP-158，沒有畫面需要） |
| 下游呼叫的逾時與重試：10 秒 deadline、失敗回降級值 | 外部呼叫有界 | 不成立：BD28 | 同進程呼叫，失敗即失敗 | 沒有外部呼叫；port 由組裝點提供，錯誤原樣回傳 |

## CodeMeta 值域與錯誤碼的去向（B7-3a）

依 ADR `docs/adr/2026-09-25-codemeta-retirement.md`：client 會分支的封閉集合是 proto enum，client 需要在同一個 code 底下分支的錯誤原因是 `ErrorReason`，其餘是模組句子（`internal/text`，`ErrorInfo` 不帶）。CodeMeta 的生成檔不動，隨 B7-5 退場。

| CodeMeta 值域 | 去向 |
|---|---|
| `CouponType`（CASH、PERCENT） | proto enum `DiscountType`；Go `model.DiscountType`、DB CHECK 同值 |
| `CouponTargetType`（ALL、MEMBER、DESIGNATED、COMPENSATION） | proto enum `TargetType`；賣家目前只能建 ALL、DESIGNATED |
| `CouponTemplateStatus`（DRAFT、ACTIVE、INACTIVE、EXPIRED、REVOKED） | proto enum `CampaignStatus`（DRAFT、ACTIVE、ENDED）；REVOKED→ENDED，INACTIVE（停權停用）與 EXPIRED（排程）不存在（D8 修訂、BD25）；到期是 `SellerCampaign.expired`（推導） |
| `CouponInstanceStatus`（UNUSED、RESERVED、USED、REVOKED、EXPIRED…） | proto enum `VoucherStatus`（AVAILABLE、LOCKED、REDEEMED、VOIDED）；EXPIRED→`Voucher.expired`（推導）；來源另有 `VoucherOrigin`（CLAIM、GRANT、COMPENSATION） |
| `IssuerType` | 不對外：模組的 `Issuer`（`issuer_type` CHECK）；client 從方法知道身分，店家看到的都是自己的活動 |
| `CouponRedemptionAction` | 不對外：`coupon_redemption_logs.action` 的 CHECK（CLAIM、ISSUE、LOCK、REDEEM、RELEASE、VOID、END_CAMPAIGN） |
| `CouponEventType`（`coupon.issued`、`coupon.expiring`） | `coupon.issued` 不搬（D7、BD27）；`coupon.expiring` 是 in-process 事件 `VoucherExpiring`（B7-4c，`internal/expiry`） |

| 舊錯誤碼 | 去向 |
|---|---|
| 7001 `COUPON_NOT_FOUND` | not_found，句子（`ReasonCampaignNotFound`、`ReasonVoucherNotFound`）；別家的與不存在的相同（防枚舉） |
| 7002 `COUPON_EXPIRED`、7021 `COUPON_CLAIM_OUT_OF_WINDOW` | `ErrorReason.COUPON_OUTSIDE_WINDOW`（failed_precondition） |
| 7020 `COUPON_CLAIM_REQUIRES_ACTIVE`、7013 `COUPON_PUBLISH_REQUIRES_DRAFT` | `COUPON_CAMPAIGN_NOT_ACTIVE`（領券時活動不是進行中）；發布的狀態錯誤屬 B7-3b（需要時再加 ErrorReason） |
| 7005 `COUPON_QUOTA_EXCEEDED` | `ErrorReason.COUPON_QUOTA_EXCEEDED` |
| 7006 `COUPON_ISSUE_LIMIT_REACHED` | `ErrorReason.COUPON_ISSUE_LIMIT_REACHED` |
| 7009 `COUPON_INVALID_STATUS`、7014 `COUPON_DISABLE_REQUIRES_ACTIVE` | 賣家結束活動冪等，不再有這個錯誤（D5），賣家的活動也沒有 DRAFT，所以 B7-3a 的 RPC 都不回它，ErrorReason 不收；admin 的作廢與發放（B7-3b）需要時再加 |
| 7003、7004、7008、7023、4011、4013（鎖券與驗券） | 模組句子（`ReasonVoucherUnavailable`、`ReasonNotApplicableToStore`、`ReasonVoucherLockedByOtherOrder`…），不進 `ErrorReason`：呼叫者是結帳（B8）與 `legacy/couponserver`（B7-4），不是買家與店家的 Connect |
| 7007 `COUPON_ALREADY_RECEIVED` | 不再是錯誤：領券冪等，回既有的券 |
| 7010 `COUPON_SELF_ISSUE_NOT_ALLOWED` | invalid_argument 加 `BadRequest`（`campaign.designated_buyer_ids`） |
| 7011 `COUPON_NOT_CLAIMED` | 只在 `legacy/couponserver`（B7-4，D1 回「未領取」） |
| 7012 `COUPON_CLAIM_NOT_REQUIRED`、7022 `COUPON_DISTRIBUTE_REQUIRES_ACTIVE` | 刪除：沒有 claim_required（D2）、沒有代領（D6） |
| 7015、7016、7017、7019 | invalid_argument 加 `BadRequest`（欄位路徑），句子是模組的 `Invalid*` |
| 新增（舊契約沒有） | `COUPON_NOT_ELIGIBLE`（不符合資格，舊的是 403）、`COUPON_STORE_SUSPENDED`（D8 修訂） |

## B7-3a 對外契約的決定（計畫沒寫的欄位級取捨）

- 服務名稱 `CouponAppService`（資源被買家與店家共用，API 設計規範 §二），7 支 RPC：買家 `ListMyVouchers`、`ListClaimableCampaigns`、`ClaimCoupon`，賣家 `CreateCampaign`、`ListCampaigns`、`GetCampaign`、`EndCampaign`。賣家的店家一律由呼叫者此刻的店推導，請求不帶 `store_id`。
- 賣家第 5 支 `ListBuyerCouponsForSeller` 沒有做：計畫的 BD33 與 D6 判為刪除（沒有消費端、對 production 資料永遠不命中），派工的「8 支」與它牴觸，已回報。
- 「無發放上限」與「上限 0」：`optional int32 total_issue_limit`，不設是無上限、設了至少 1，0 回 invalid_argument（`TestCreateCampaignDistinguishesNoLimitFromZero`、`TestSellerCampaignWireKeepsNoLimitUnset`）。
- 列表的過濾在 SQL：可領列表排除時間窗外與別人的 DESIGNATED，停權狀態在一個請求內每家店只問 `Stores` 一次（`qualify.PerRequest`）；`SellerCampaign.expired` 由模組的時鐘算。port 讀取失敗仍讓整個請求失敗。
- 結帳可用的券是 `ListMyVouchers` 帶 `checkout`（店家、訂單金額）：只回現在鎖得住的券（與 `Lock` 同一組檢查），附 `estimated_discount`；沒有另開 RPC。

## 與 coupon-service 的差異

- 沒有資料搬移：schema 是新的 baseline；舊的 `coupon_templates`、`coupon_instances`、`coupon_claims`、`coupon_redemption_logs` 不搬。
- 狀態碼從數字字串改為 UPPER_SNAKE 文字加 CHECK；主鍵從 TSID 改為 UUIDv7。
- 領券、指定發放、補償都建立券；結帳只能鎖已存在的券（沒領過回「未領取」，由轉接層處理，D1）。
- 補償券多一個必填的來源識別（`source_type`、`source_id`）；賣家補償的到期日由呼叫方給。
- 活動提前結束後，已鎖定的券可繼續核銷或釋放；釋放時活動已結束則券作廢。
- `usage_limit_per_account`、FREE_SHIPPING、Redis 快取、`coupon.issued` 事件、對帳與自動過期 job 都不搬。
