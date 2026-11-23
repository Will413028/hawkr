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
| 模板狀態 `INACTIVE`＋`disabled_by_suspension`（停權停用、解除時精準還原） | 賣家停權時可逆地停用其活動 | 成立，但屬 D8 的事件訂閱（B7-4） | 訂閱停權事件，同 transaction 處理 | B7-2 的 schema 只有 DRAFT/ACTIVE/ENDED；停權的可逆停用需要的狀態或欄位在 B7-4 以 migration 加入（本步驟分支內，尚未併入 dev，可壓平）。**待 Will 決定的取捨：停權是否要可逆停用，或改成「停權期間 Lock 時檢查店家狀態」而不改活動** |
| `usage_limit_per_account`（存了沒有執行） | 每人領取上限 | 不成立：沒有執行邏輯、沒有 client 設定（BD16）；實際限制是 `(template, owner)` 唯一索引＝每人 1 張 | 每人上限用 UNIQUE 或條件式檢查 | 刪除欄位；每位買家每個活動至多一張，`vouchers_campaign_buyer_key`（已核銷與已作廢的也占名額） |
| `total_issue_limit = -1` 表示無限，`issued_count` 在領取後遞增、回傳值被忽略 | 總量上限 | 成立；寫法不成立：BD4、BD5、BD14（`-1` 魔術值） | `UPDATE … SET issued = issued + 1 WHERE issued < limit`；無上限用 NULL | `total_limit` NULL＝無上限（上限至少 1，與「上限 0」分開）；`IncrementIssued` 條件式 UPDATE，回 0 列＝`ReasonIssueLimitReached`；`campaigns_issued_count_check` 當後備 |
| 資格判定 `isEligible`（只在領券呼叫）與 `selectAvailableForBuyer`（列表的另一份等價 SQL） | 領取與使用時判斷買家是否符合 | 判斷成立；兩份實作不成立：BD1、BD2、BD11 | 資格只有一份判定，領取與套用都呼叫 | `qualify.Check`（狀態、時間窗、資格）＋`qualify.Fit`（店家範圍、門檻、折抵）；`Claim`、`Lock`、`Quote` 共用；列表（B7-3）不另寫第二份規則 |
| ALL 資格：買家註冊時間 ≤ 模板建立時間 | 防止新註冊帳號領取既有的全站券（推測的原意，Java 沒有文件） | 不確定：沒有文件說明原意；Java 在查不到註冊時間時兩條路徑行為相反（BD11） | 業界的全站券通常不看註冊時間，新客券另做一種 | **沿用，待 Will 確認是否保留**：`TargetAll` 判定 `BuyerCreatedAt ≤ campaign.created_at`，由 `Buyers` port 提供註冊時間，讀取失敗整個請求失敗（BD28）。不保留則刪 `Buyers` port 與這條判定 |
| MEMBER 資格：查 Java order 的 `GetBuyerMerchantSummary`，失敗回空集合 | 在該店有消費紀錄才符合 | 資格成立；降級不成立：BD28（order 掛掉時 MEMBER 券整批消失） | 讀取方持有投影，由訂單完成事件餵入（D3） | `Members` port（`ports.go`）；B7-2 只定義 port、測試用假的；投影與事件消費在 B7-4；port 失敗整個請求失敗 |
| DESIGNATED、COMPENSATION：`coupon_target_accounts`；`insertIgnore` 逐筆寫 | 資格名單 | 成立 | 名單表，批次寫入、冪等 | 保留表 `coupon_target_accounts`，`InsertTargetAccounts` 以 `unnest` 一次寫入；刪帳時清名單（BD19）在 B7-4 的訂閱者 |
| `validateCoupon`（不鎖、不建 instance；只在驗券檢查狀態、時間窗、scope、門檻） | 結帳前預覽折抵 | 成立；與 reserve 檢查項目不同不成立：BD1、BD2 | 預覽與套用用同一組檢查 | `Quote` 與 `Lock` 同一組檢查（資格、時間窗、店家範圍、門檻），回傳折抵金額；已由同一筆訂單持有的重送略過資格（D5） |
| `reserveCoupon`：先查再插入、UNIQUE 撞了回系統錯誤、不重驗、`writeLog` 吞例外、`applicableStoreId` 恆 NULL | 一張券同時只被一筆訂單鎖；重送冪等 | 鎖的語意成立；寫法不成立：BD2、BD3、BD6、BD7 | 在呼叫方 transaction 內以條件式 UPDATE 鎖定，冪等 | `Lock`：條件式 `UPDATE … WHERE status = 'AVAILABLE'`；同一筆訂單重送冪等成功，別單 `ReasonVoucherLockedByOtherOrder`；一筆訂單只持有一張券（`vouchers_order_no_key`）；鎖沒有到期時間（D3）；稽核同 transaction |
| `consumeCoupon(instanceId, orderNo)`、`releaseCoupon(instanceId, orderNo)`（Release 也處理 USED→UNUSED，賣家取消已付款訂單） | 付款後核銷；取消、逾時、賣家取消後還券 | 成立；以 instance id 為鍵不成立：D3 A 決定以 `order_no` 為鍵（回應遺失時 Java 不知道 instance id） | 狀態轉換跟著訂單，冪等 | `Redeem(orderNo)`、`Release(orderNo)`：條件式 UPDATE、重送冪等；Release 處理 LOCKED 與 REDEEMED（保留賣家取消已付款訂單還券）；活動已結束時 Release 把券作廢而不是回到可用（D5 的 AVAILABLE 都作廢；共享鎖活動列與結束排隊）；Redeem 不檢查活動狀態與時間（BD8、BD10 到 B8） |
| `AutoReleaseJob` 對帳（每分鐘查 order 狀態補 consume 或 release） | Release 漏送時補救 | 不成立：D3 A 決定不做反查 order 的對帳與推導式釋放；BD25（批次被 orphan 卡住、fail-open） | 釋放跟著訂單狀態轉換在同一個 transaction | 刪除；Java 取消路徑補 Release 與重試在 B7-4；漏送由 admin `Void` 人工處理（可作廢 LOCKED） |
| `revokeByTemplate`（賣家收回：作廢 UNUSED 與 RESERVED） | 賣家停止活動 | 語意不成立：BD8（進行中訂單核銷失敗且無補償） | 只擋新的領取與套用；已鎖定的照常完成（D5） | `EndCampaign`：ACTIVE→ENDED，作廢 AVAILABLE，LOCKED 保留；先鎖活動列，與領券、發券排隊；已結束重送成功；賣家路徑以 `StoreID` 限定自己的活動（別人的回 NotFound） |
| `voidInstance`（admin，只作廢 UNUSED） | 營運作廢單張券 | 不完全成立：BD17、D3（卡在 RESERVED 的券要能人工解除） | 作廢單張券，稽核 | `Void`：AVAILABLE 或 LOCKED 可作廢，已核銷的不行（`ReasonInvalidStatus`）；稽核記 admin 與理由 |
| `BuyerDeletedConsumer`、`SellerSuspended/UnsuspendedConsumer`（MQ） | 刪帳、停權時處理券與活動 | 成立；MQ 不成立：buyer、seller 已在單體（D8） | 訂閱 in-process 事件，同 transaction | 不在 B7-2：B7-4 訂閱 `BuyerDeleted`、`StoreSuspended`、`StoreUnsuspended` |
| 賣家配額：`checkQuota`（count 後比較）與 `updateQuota`（先查後寫） | 限制每家店同時進行的活動數（預設 5） | 成立；寫法不成立：BD15 | 鎖住配額列後檢查與插入串行 | `seller_coupon_quotas`；建立時 `InsertQuotaIfAbsent`＋`LockQuota`（FOR UPDATE）再 `CountActiveSellerCampaigns`；`SetQuota` 是 upsert；預設 `DefaultSellerQuota = 5` |
| 補償券：每次 grant 建一個新模板，`ref_case_no` 只進標題，沒有冪等鍵；賣家補償無到期日（2099）、admin 補償 +3 個月 | 售後與營運補償 | 成立；沒有冪等鍵不成立：BD9 | 以來源事件的身分當冪等鍵（D4） | `IssueCompensation`：一券一活動（總量 1）；活動與券都有 `UNIQUE (source_type, source_id)`，重送回原本的券；到期日由呼叫方給（B7-4 轉接層決定 2099 或 +3 個月）；沿用賣家補償規則（門檻 ≥ 面額 5 倍、與面額差 ≥ 100）；來源種類 `AFTERSALES_CASE`、`ADMIN_COMPENSATION` |
| 建立驗證 `validateTemplateParams`（CASH 要金額、PERCENT 1–99 與上限、start ≤ end）；沒有 CHECK | 規則自洽 | 成立；缺 CHECK 與折抵上限檢查不成立：BD31、BD32 | 輸入檢查加 schema CHECK 當後備 | `CampaignSpec.Validate` 與 CHECK 同一組條件，加上 CASH 折抵不得超過門檻（門檻 0 不套用，Will 2026-11-23）；FREE_SHIPPING 已於 Java V5 移除，不搬 |
| `issuer_type`＋`issuer_account_id`＋`issuer_store_id`（V6 expand，CHECK 留給下一次 contract） | 區分 admin 與賣家發行 | 成立；沒有 CHECK 不成立：BD31 | 以 CHECK 保證互斥 | `issuer_type`＋`issuer_admin_id`＋`issuer_store_id`，`campaigns_issuer_check` 互斥；賣家活動 `store_id = issuer_store_id`（`campaigns_seller_scope_check`）；admin 身分用單體的 `admin_id`（BIGINT） |
| 折抵計算 `calculateDiscount`（CASH：min；PERCENT：amount×percent/100 取上限） | 算折抵金額 | 成立 | 整數運算、金額為新台幣元 | `model.Discount`；`Fit` 限制訂單金額 ≤ `MaxOrderAmount` 避免溢位 |
| `coupon_redemption_logs`：寫入失敗吞掉（BD7）、`instance_id`／`template_id` 未知寫 0、`trace_id`／`request_id` 恆空、`detail VARCHAR(500)` | 稽核 | 稽核成立；寫法不成立：BD7、BD23、BD30 | 稽核與操作同一個 transaction | `coupon_redemption_logs`：同 transaction 寫入、失敗整筆失敗；未知的欄位用 NULL；沒有 trace 欄；理由 ≤ 255 字元；action 的值域有 CHECK |
| Redis 模板快取 `coupon:tpl:*`、`coupon:avail:*` | 降低讀取延遲 | 不成立：同進程直接查 DB；快取失效不完整且有死 key（BD29、BD30） | 沒有量測前不加快取 | 刪除 |
| `LocalDateTime` 讀寫 `TIMESTAMPTZ`、JVM 預設時區的 cron | 時間比較正確 | 不成立：BD26 | UTC、`timestamptz`、注入時鐘 | 全 `timestamptz`；模組不呼叫 `time.Now()`，用 `Deps.Now`；比較都在 Go 以同一個 `now` |
| `updated_at` 觸發器、MyBatis-Plus 整列 `updateById` | 維護更新時間；改狀態 | 不成立：整列覆寫會蓋掉並發遞增（BD5） | 條件式 UPDATE，只改需要的欄 | 沒有觸發器；每個 UPDATE 自己設 `updated_at`；狀態轉換都是條件式 UPDATE |
| 出站 gRPC：Account 註冊時間、Order 的 MEMBER 集合與訂單狀態、Buyer 與 Seller 的顯示資料；失敗回 -1 或空 | 資格與顯示需要別的服務的資料 | 資料需求成立；降級不成立：BD28 | 同進程直接呼叫模組 API，失敗就失敗 | 資格需要的兩項定義成 port（`Buyers`、`Members`）並失敗即回錯誤；顯示資料在 B7-3 由 httpapi 組裝；訂單狀態不需要（D3 不做對帳） |
| 賣家 DESIGNATED 的自發自用防護（解析 principal 比對） | 賣家不能發券給自己 | 成立 | 身分比對在入口，核心不重複 | 不在核心：需要 identity 解析，B7-3 在 Connect 的賣家建立活動入口做（`CreateCampaign` 只收已解析的 buyer_id） |
| `CouponTargetTypeRules`（賣家只能 ALL、MEMBER、DESIGNATED） | 限制賣家可建立的資格 | 成立 | 權限在入口與核心都檢查 | `CreateCampaign` 只接受 ALL、MEMBER、DESIGNATED；COMPENSATION 只經 `IssueCompensation`；賣家的 DESIGNATED 名單不得為空，賣家的活動一律是自己的店 |
| 無 FK（所有關聯靠應用層） | 避免跨服務耦合 | 同 schema 內不成立：沒有理由不用 FK | 同 schema 用 FK，跨 schema 不建（單體規則 2） | 券與資格名單 FK 到活動（同 schema）；沒有跨 schema FK（`buyer_id`、`store_id` 是業務鍵）；稽核紀錄不設 FK（只追加，不擋刪除） |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 認證與身分：`GrpcAuthContext.requireAccountType`、`parseLong` 吞格式錯誤（BD14、BD24） | 只有對的角色與帳號能呼叫 | 成立；吞錯誤不成立 | Connect 入口依 session 取身分，非法值回 InvalidArgument | B7-3 決定；核心的輸入都是已解析的型別，核心自己檢查正數與長度（`Invalid`） |
| 錯誤：envelope `code`＋`GrpcResponseCode` 7xxx，兩種契約並存（BD22） | client 區分失敗原因 | 不成立：兩種契約並存 | 分類錯誤加 `ErrorInfo.reason`（ADR api-error-model） | 核心回 `apperr` 分類加 `model.Reason`；對外的 `ErrorReason` 與 CodeMeta 錯誤碼去向在 B7-3 逐一決定 |
| 分頁：SQL 取一頁後在記憶體過濾，`total` 是該頁筆數（BD12） | 列表分頁與總數正確 | 不成立 | keyset 或 SQL 內分頁 | B7-3；`coupon_vouchers` 已有 `vouchers_by_buyer` 索引支援 |
| 下游呼叫的逾時與重試：10 秒 deadline、失敗回降級值 | 外部呼叫有界 | 不成立：BD28 | 同進程呼叫，失敗即失敗 | 沒有外部呼叫；port 由組裝點提供，錯誤原樣回傳 |

## 與 coupon-service 的差異

- 沒有資料搬移：schema 是新的 baseline；舊的 `coupon_templates`、`coupon_instances`、`coupon_claims`、`coupon_redemption_logs` 不搬。
- 狀態碼從數字字串改為 UPPER_SNAKE 文字加 CHECK；主鍵從 TSID 改為 UUIDv7。
- 領券、指定發放、補償都建立券；結帳只能鎖已存在的券（沒領過回「未領取」，由轉接層處理，D1）。
- 補償券多一個必填的來源識別（`source_type`、`source_id`）；賣家補償的到期日由呼叫方給。
- 活動提前結束後，已鎖定的券可繼續核銷或釋放；釋放時活動已結束則券作廢。
- `usage_limit_per_account`、FREE_SHIPPING、Redis 快取、`coupon.issued` 事件、對帳與自動過期 job 都不搬。
