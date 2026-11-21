# moderation 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

來源：social-service 的檢舉部分（`backend/services/social-service` 的 `reports`、`moderation_actions`、`usecase/report.go`、`usecase/dispatcher.go`，簡稱 `SI`），盤點 `backend/docs/plans/2026-10-18-a7-social/domain-inventory.md`（M12、M17、M24），決定 A7-D7、A7-D13、A7-D14、A7-D20（計畫 `2026-10-18-a7-social.md`）。對外 API 與特性測試在 A7-2。

## 沿用盤點

social-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 主鍵 BIGINT tsid（`reports`、`moderation_actions`，M1） | Java 消費者要以數字 ID 讀 | 不成立：沒有別的模組以 bigint 指稱檢舉或案件；seller 的認定以不透明參照引用檢舉 | 應用層 UUIDv7（ADR 2026-09-25-row-identity-by-data-kind） | 改：案件、檢舉、快照都是 UUIDv7；檢舉人與買家對象用 buyer 的 BIGINT 業務鍵 |
| 一筆檢舉一列、各自帶 `status` 與 `resolved_action`，處置逐筆（`SI/schema.sql:85-97`） | 逐筆處置 | 不成立：同一對象的 30 筆檢舉要處置 30 次，結果不一致；seller 的通報可在分案前逐筆判掉，社群檢舉一律掛案 | 檢舉掛案，一案一個決定（A7-D14） | 改：案件的鍵是對象（內容 `(target_kind, content_id)`、買家 `buyer_id`，兩個 partial UNIQUE `WHERE closed_at IS NULL`，不用生成欄）；決定只存在案件（`decision`、`decision_category`、`decision_action`、`decided_by_admin_id`、`closed_at`，CHECK 同為 NULL 或同不為 NULL）；檢舉不存結果，由案件推導；結案是終態，不重開、不改判 |
| `moderation_actions` 審核動作 audit log，寫失敗被丟棄、沒人讀（M12、SD12，`SI/schema.sql:118-133`） | 稽核每次處置 | 不成立：處分的紀錄歸擁有效果的模組（10e R12，A7-D9） | 處置與紀錄同一個 transaction、可讀取 | 刪：不建處置紀錄表；案件的 `decision` 是決定紀錄，處分本身的紀錄在 social（隱藏、警告）、identity（停權）、seller（措施）；處置與結案同一個 transaction，寫失敗整個回滾（SD12） |
| `AUTO_REJECT` 與 `REOPEN` 動作（系統自動駁回、重開） | 系統建立者與重開 | 不成立：A7-D7 第 3 項不新增系統建立者；結案是終態 | 重開用新案件 | 刪：不沿用 `AUTO_REJECT`；沒有重開，結案後的新檢舉開新案件 |
| 處置 dispatcher 依 `target_type` 分流，作者解析沒有 kind，先當店家（M12、SD10，`SI/usecase/dispatcher.go:42-89`） | 對象種類決定處置 | 不成立（缺陷）：撞號時猜錯人 | 對象帶 (kind, id)，作者由擁有者回報 | 改：檢舉帶 `(target_kind, id)`；作者向 social 讀（`ContentOwner`），買家作者轉帳號只在 `BAN` 呼叫 `accountsuspension` 之前由 buyer 的 `AccountIDByBuyerID` 做一次；店家不能被停權，店家的內容走 seller 的內容下架措施，且內容必須經 social 驗證屬於指定的店家 |
| `WARN_USER` 只寫稽核，對使用者沒有任何效果（SD11，`SI/usecase/report.go:235-237`） | 警告要有效果 | 不成立（缺陷） | 警告是使用者讀得到的處分紀錄 | 改：`WARN` 呼叫 social `Warn`，作者在「我的處分」讀得到型別與理由 |
| 檢舉原因 8 種 `SPAM`、`SEXUAL`、`FRAUD`、`HARASSMENT`、`FOOD_SAFETY`、`POLITICAL`、`FAKE_NEWS`、`OTHER`（CodeMeta `ReportReasonCode`，`SI/schema.sql:95`） | CodeMeta 的舊契約 | 凍結，不新增值域；政策分類是平台共用的一份（Q5） | 平台政策分類加各入口的子集 | 改：用 `PolicyCategory` 的社群檢舉子集 `CommunityReporterSelectable()`（恰好八個）。對映：`SPAM`→`SPAM`、`HARASSMENT`→`HARASSMENT`、`FRAUD`→`FRAUD`、`OTHER`→`OTHER`、`SEXUAL`→`SEXUAL_CONTENT`（新增）、`POLITICAL`→`OTHER`、`FAKE_NEWS`→`MISLEADING_INFO`、`FOOD_SAFETY`→`FOOD_SAFETY`；`HATE` 新開放（業界檢舉選單都有仇恨言論）；`POLITICAL` 不是平台政策類別，歸 `OTHER` |
| 每人每天 20 筆、24 小時去重，先查再寫（M24、SD23，`SI/usecase/report.go:35,100-112`） | 防濫用 | 成立（濫用防護仍需要），但先 count 再比較有並行空窗，24 小時去重讓結案後與跨日行為不一致 | DB 唯一約束加原子計數；速率限制交給入口 | 改：每日上限 20 保留（`reporter_daily_counts` 單列 upsert，台北日換日重算，未達上限才加一，原子）；去重是 `UNIQUE (case_id, reporter_buyer_id)`，進行中案件重送回傳既有檢舉且不吃額度，結案後可再檢舉；速率限制在 `httpapi` 入口，不在本模組 |
| 檢舉人 `reporter_id` 依 active role 存 buyer_id 或 store_id，查不到現任時退回 account_id（`SI/schema.sql:106`） | 店家與買家都能檢舉 | 不成立：只有買家檢舉社群內容；退回 account_id 讓同一欄混了三種身分空間 | 單一鍵 | 刪：不沿用 reporter fallback；`reporter_buyer_id` 單一鍵，買家對象存 `buyer_id` |
| 提交不驗證對象存在、不禁止檢舉自己；`target_snapshot` 欄位從未寫入（SD21、SD23，`SI/schema.sql:86`） | 無（缺陷） | 不成立 | 提交前驗證，凍結當時內容 | 改：對象必須存在而且公開可讀（被隱藏的回 NotFound）；不能檢舉自己（含同一帳號的其他身分與店家所屬帳號）；送出時凍結快照（A7-D20） |
| 快照內容的保留（舊：無期限，欄位從未寫入） | 申訴與稽核要看當時的內容 | 申訴期至少六個月（DSA 第 20 條）；seller 措施申覆期限 184 天 | 保留期後刪除個資 | 新：結案後 184 天刪快照（與 seller `StoreRetention` 同值，各自一個常數），案件與檢舉列留著；進行中案件的快照不刪；`snapshot_id ON DELETE SET NULL` |
| 案件 `version` 樂觀鎖欄 | 管理員看到的檢舉集合要等於結案時的集合 | 成立：決定與（內容下架的）來源檢舉都基於管理員看過的集合；`closed_at IS NULL` 只保證一次轉移，不保證集合沒變 | 樂觀並行控制（讀到的版本在寫入時還是同一版） | 保留並補完：新檢舉掛進既有案件時 `version` 加一（`AttachToCase`，同 transaction，鎖案件列；開案的那筆不加，版本從 1 起），結案以版本比對，掛入與結案互相等待，所以來源檢舉 ID 不必另外鎖（`TestAReportAttachedAfterTheAdminReadMakesTheCloseConflict`） |
| 內容下架引用最早的 20 筆來源檢舉 | seller 認定的來源上限 `MaxSourceModerationReports` | 成立 | 引用有上限、備註寫總數 | 保留：引用最早 20 筆，備註寫真正的檢舉數，超過 20 筆時其餘檢舉只在 moderation 看得到（`TestTakedownNoteCountsEveryReportButCitesTheEarliestTwenty`） |
| `cases_open_by_time` 索引（`opened_at, id` WHERE 進行中） | A7-1e 後台待辦依開案時間 keyset 讀進行中的案件 | 消費者還不存在（預建，沒有量測） | 索引隨第一個讀它的查詢一起建 | 暫留，觸發：A7-1e 的待辦查詢落地時用 `EXPLAIN` 確認有用到；若 A7-1e 改用別的排序鍵就刪 |
| River 週期 job（`moderation.purge_snapshots`、`moderation.media_sweep`）與分批 500 | 快照與說明要在保留期後清掉、引用同步要有保底掃描 | 成立；seller 的保留清理與 social 的 media 同步是同型 job | 排程 job 分批、冪等 | 照 seller、social 的做法：每日清、啟動跑一次、`UniqueOpts ByPeriod 12h`；media 同步用 `media.RefSyncer` |
| 證據保全：快照引用的圖片只存 `asset_id`，社群在刪文與店家退場時釋放自己的引用（A7-D21） | 申訴期內要能重現被檢舉的那一版，含圖片 | 成立：不保全的話，作者刪貼文後只剩指向已回收資產的字串 | 保全證據（在引用端 hold，不複製檔案） | 新增：快照以 `(GOVERNANCE_EVIDENCE, 快照 ID)` 在 media 登記引用（借用既有的 ref type，CodeMeta 凍結不新增值），同步 job 寫入、快照刪除時才解除；沒有新的 media API。已知殘餘：新快照的同步 job 還沒跑完、而作者已刪貼文且 GC 已回收時，圖片已經不在（同 media 規格 §7 的方向：一般引用同樣有這個窗口）；買家頭像只存 object key，沒有 asset ID，不在保全範圍，觸發：要保全頭像時由 buyer 提供 asset ID |
| 檢舉人的說明文字與每日計數無保留期（A7-D21） | 個資最小化（GDPR 5(1)(e)） | 說明是自由文字，可能含個資 | 說明與快照同期清、刪帳時清 | 新增：說明在結案滿 184 天與快照一起清；檢舉人刪帳時（買家刪除事件，同 transaction 訂閱者；沿用 social 的訂閱模式而不是讓 accountdeletion 依賴 moderation）清他全部檢舉（含同帳號歷史身分）的說明、冪等紀錄（request_hash 是說明的雜湊）與每日計數列，檢舉列與 `reporter_buyer_id`（假名鍵）留著。每日計數列對未刪帳的檢舉人每人一列、有界，不清 |
| 髒字詞典（M17） | 預先過濾 | 屬於 social 的寫入路徑（A7-1a 已放在 `platform/textfilter`） | 詞庫有擁有者與後台維護 | 不在本模組：詞庫的擁有者與後台維護是另一個決定，觸發是營運要求新增詞 |
| 扁平的 `snapshot.Content` 加種類專屬欄位（A6-4 設計 §9 第 1 列） | 依種類的 tagged union（每種一個 struct） | 「既有雜湊不變」不是約束（未上線，根目錄 `AGENTS.md`） | 依種類的 struct | 改（A6-D21 W4 (a)）：`snapshot.Content` 是 tagged union（Social、Buyer、Review、Chat 恰好一個有值，Decode 驗種類與欄位一致），後台 `GetCase` 帶 `snapshot` oneof；後台畫面在 A6-6 |
| 隱藏表在內容模組、moderation 只存案件 | 同（處分紀錄與效果同模組，10e R12） | 成立 | 同 | 沿用：評價隱藏在 review、聊天訊息隱藏在 im |
| HIDE 遇到已清除內容視為成功 | 同（隱藏的目的已達成；不讓案件卡住） | 成立 | 同 | 沿用，擴到評價與聊天訊息（擁有者回 NotFound 視為成功、不寫隱藏紀錄）；`internal/handle/AGENTS.md` 與 `backend/docs/modules/moderation.md` 的過時句（FailedPrecondition）已改正，以程式與測試為準 |
| 證據保全以 media 引用（`GOVERNANCE_EVIDENCE`） | 同 | 成立 | 同 | 沿用，範圍擴到聊天前後文的圖片（`snapshot.Content.AssetIDs`：被檢舉訊息與前後文圖片的聯集，墓碑不算） |
| 處分終態、不重開不改判（`Decide`），更正在擁有處分的模組做 | 處分可撤銷，撤銷記在原決定旁並留稽核（DSA 第 20 條申訴後撤銷；各平台在原 enforcement 上記 reversal） | 不成立：後台沒有任何還原入口，計畫驗收要「檢舉→隱藏→還原」 | 撤銷紀錄與原決定同處 | 改（A6-D21 W1 (a)）：`ReverseHide`，案件維持結案、決定不改，`cases.reversed_*` 三欄與 `adminaudit`；同 transaction 呼叫 review／im／social 的 `Restore`，帶來源參照 `moderation:case:<id>` 只還原本案造成的隱藏（冪等的 `Hide` 讓後到的案件沿用先到那一筆，撤銷後到案件回 FailedPrecondition，不動別案的隱藏；沿用別案隱藏的案件因此不能撤銷，只有造成該隱藏的案件能撤銷它）；稽核 detail 記 `restored`；擁有者回 NotFound（內容已清除）視為成功並照常記下撤銷；社群留言、討論區留言一併受惠 |
| 評價與聊天訊息的隱藏與還原要不要取擁有者的守門 | 守門擋停用身分產生新資料；處分不產生新資料 | 不成立：取了會讓已停用作者的處分失敗 | 處分不被作者狀態擋 | 不取（`LockActive`、`LockNotClosed`、`LockNotRetired` 都不呼叫，`TestHideWorksWhileTheAuthorIsDeactivated`） |
| 檢舉人只有買家 | 平台雙邊都能檢舉（聊天是雙方私訊） | 買家單方檢舉下，聊天案件的作者全是店家，HIDE 回 `DecideHideStoreContent` | 雙邊 | 改（A6-4c，A6-D21 W2、W3 (a)，Will 2026-11-21）：`reports`、`report_requests`、`reporter_daily_counts` 加 `reporter_store_id` 與恰好一欄 CHECK，額度 20 依檢舉人種類各算；店家只收 CHAT_MESSAGE；店家寄的訊息與評價回覆走 seller 內容下架措施。沒有員工角色模型：能回聊天的店家帳號就能檢舉 |
| WARN 寫在 social 的買家警告 | 買家處分紀錄放 buyer 或獨立 enforcement 模組 | 沿用成立 | 同 | 沿用：觸發是第三種非社群對象需要警告時（評價、聊天訊息的買家作者的警告仍在 social） |
| 聊天快照凍結檢舉人自己的前後文訊息與圖片 | 證據要重現被檢舉訊息的對話情境（A6-D3） | 成立；但檢舉人刪帳的清理不清這些前後文 | 證據留存期內保留，個資面另做法務審查 | 記錄：聊天前後文含檢舉人自己的訊息與圖片，視為證據，隨 A7-D21 的 184 天保留期一併保留；觸發：上線前的法務與隱私審查時重評（是否遮蔽檢舉人自己的前後文）。不改程式（`internal/snapshot/snapshot.go` 的聊天快照） |
| 前後文窗格固定 5 則 | 依時間窗或依則數 | A6-D3 | 同 | 沿用 `model.ChatContextWindow = 5` |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。A7-1d 沒有對外入口（舊 social-service 照常服務），這一節只記已知的對應，Connect 的宣告表與 handler 在 A7-2。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 檢舉人身分由 gateway 解析 principal，失敗退回原值（M18） | 跨服務時 gateway 才有身分 | 不成立：同一個程序，入口解析 session 後直接帶 buyer_id | 入口認證、模組收已解析的業務鍵 | 改：`SubmitReport` 收 `ReporterBuyerID`，由 A7-2 的 handler 從 session 解析 |
| 錯誤用 CodeMeta 錯誤碼與字串訊息 | 舊契約 | 不成立：新 API 用 `apperr` 的 kind 與 `ErrorReason` | 結構化錯誤 | 改：`apperr`（InvalidInput、NotFound、FailedPrecondition、ResourceExhausted、VersionConflict）；伺服器句子在模組 `internal/text`（檢舉人受眾 zh-TW、en；管理員受眾只有 zh-TW），語系取自 context，`TestModerationApperrCallsCarryNoSentenceLiterals` 守門 |
| 列表分頁 page／pageSize 加 `COUNT(*)`（M20） | client 顯示總頁數 | 不成立 | AIP-158 keyset | 後台待辦在 A7-1e、案件列表（`Cases`）在 A7-2f-4 用 keyset |
| 每日 20 筆在入口限流（M24） | 防濫用 | 上限是業務規則，在模組內強制；入口另有一般限流 | 業務上限在資料層原子強制 | 改：上限在模組內單一 upsert 強制，入口限流不負責 |

## 與 social-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。

- 檢舉對象：貼文、留言、討論區留言、買家、評價與聊天訊息（舊：`REVIEW`、`POST`、`POST_COMMENT`、`SELLER_FORUM`、`CHAT_MESSAGE`、`USER`）。
- 檢舉不存 `status`、`resolved_action`、`resolved_by`、`resolved_at`：結果由案件推導。
- 沒有 `evidence` 欄：證據是送出時凍結的快照。
- 檢舉說明 `description` 選填，最多 500 字，頭尾沒有空白。
- 處置種類：`HIDE`、`WARN`、`BAN`、`TAKEDOWN`（舊：`HIDE_TARGET`、`WARN_USER`、`BAN_USER`、`DISMISS`）；`DISMISS` 對映案件決定 `NO_VIOLATION`；新增 `TAKEDOWN`（店家的內容走 seller 措施）。
- 每次處理只有一種處置；同一案件組合多種處置是終態（見規格 §6）。
- 內容下架的認定引用案件最早的 20 筆檢舉（seller 的上限 `MaxSourceModerationReports`）。

## 公司時期未完成項

legacy-backlog「A7 social」段裡屬於檢舉面的 3 項（A7-5 對照現行程式碼）；#2692（公開單篇不檢查發布狀態）屬 social，見 `social/CARRYOVER.md`。

| 項目 | 結論 | 依據 |
|---|---|---|
| #1632：`AdminResolve BAN_USER` 用 `report.TargetID` 而不是解析後的 `TargetOwnerID` | 不再成立（SD10）：檢舉帶 `(target_kind, id)`，`BAN` 只接受買家作者，作者由 social 的 `ContentOwner` 解析（內容已刪時用案件快照裡凍結的作者），再由 buyer 的 `AccountIDByBuyerID` 轉帳號後 `Suspend`；不使用檢舉的對象 ID 當帳號；店家作者回 FailedPrecondition | `internal/handle/handle.go:214-222`（BAN 分支）、`:232` `author`；`TestBuyerBranchActionsLeaveRecordsInTheOwningModules`（`decide_integration_test.go:49`）、`TestWarnAndBanUseTheFrozenAuthorWhenTheContentIsGone`（`lifecycle_integration_test.go:31`）、`TestBanRefusesAnAccountThatOwnsAStore`（`decide_integration_test.go:174`） |
| #2779：檢舉佐證在 DB commit 結果不明時盲目 Release | 不再成立（A7-D21）：沒有 commit 結果不明時 Release 的路徑。證據是 DB 裡的快照列；media 的保全引用只由同步 job 依快照目前的狀態寫入，`Release` 只被快照過期清除呼叫，且與 `DeleteSnapshots` 在同一個 transaction，commit 失敗就整個回滾、快照與引用都還在 | `internal/mediaref/mediaref.go:75` `Release`，唯一呼叫處 `internal/retention/retention.go:111`（`rg -n 'holds\.Release\|\.Release\(' internal/modules/moderation` 只有這一處）；`TestASnapshotHoldsItsImageUntilTheSnapshotIsPurged`（`lifecycle_integration_test.go:137`）、`TestSnapshotsArePurgedAfterTheRetentionAndCasesStay`（`submit_integration_test.go:277`） |
| #3513 稽核寫入沒有 logger | 修掉（SD12）：`moderation_actions` 表刪除；後台處置的紀錄由 platform 的 `adminaudit` 在結案的同一個 transaction 寫，寫失敗整個回滾，不再 best-effort 丟棄，有讀取端（營運子命令 `List`），不需要另外的 logger | `internal/handle/handle.go:134,144`（`audit`）；`TestDecideWritesTheAdminAuditRecordInTheSameTransaction`（`list_integration_test.go:163`）、`adminaudit` 的 `TestRecordRollsBackWithTheCallersTransaction` |
| #3513 整合測試 | 修掉：本模組有 integration 測試（submit、decide、lifecycle、list、schema 等），CI 的 backend-monolith integration job 執行（A7-1d 完成列的 run 43680980602） | `internal/modules/moderation/*_integration_test.go` |
| #3513 SLA 告警 | 延後（Will 2026-11-19），觸發：單體的告警管道建立時或上線前，屆時以「逾期待辦數」指標加告警規則。原狀：待辦列有期限（`pub_review_items`：開案起 72 小時，到期前 24 小時起算警示窗，`db/objects.sql:16-19`），但沒有主動告警的程式，計畫也沒有決定；A7-D14 只延後「依檢舉數或觸及量提高優先度」，不是告警 | `db/objects.sql:16-19`；計畫 A7-D14 |
| #3513 字典詞庫 | 延後，有觸發：詞庫不在本模組，擁有者與後台維護是另一個決定，觸發是營運要求新增詞（上方沿用盤點「髒字詞典（M17）」列）；social 側的詞表是過渡內容（`social/CARRYOVER.md` M17） | 本檔沿用盤點 M17 列；`social/CARRYOVER.md` M17 |
