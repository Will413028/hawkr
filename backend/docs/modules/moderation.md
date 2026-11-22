# moderation 模組行為規格

本文是單體 `moderation` 模組行為的唯一來源。社群內容與買家的檢舉：送出、案件、內容快照、處置。決策見計畫 `backend/docs/plans/2026-10-18-a7-social.md` 的 A7-D7、A7-D9、A7-D13、A7-D14、A7-D20 與 ADR `docs/adr/2026-10-29-account-suspension-and-moderation.md`。

## 1. 範圍與公開介面

- 根套件公開 `Module`：`SubmitReport`、`MyReports`、`DecideCase`、`ReverseHide`（撤銷已結案的 HIDE 決定，§3.1）、`Case`、`CaseWithSnapshot`（同一個唯讀快照讀案件與最近一份凍結的內容快照，後台 `GetCase` 用）、`OpenReportCounts`（貼文在進行中案件裡的檢舉數，只算進行中案件，後台貼文列表 A7-2f-5 用）、`Cases`（後台列案件：狀態與對象種類篩選、`(opened_at, id)` 由大到小的 keyset，預設 20、上限 50）、`PurgeSnapshots`、`AddWorkers`、`PeriodicJobs`，以及只靠 DB、buyer、media 與 job 佇列的 `Retention`（`NewRetention`：快照與說明的保留期、快照圖片的 media 引用、買家刪除事件的訂閱者；worker 與事件組裝只組它，不組完整模組）。對外 API 是 `httpapi/moderation`：`ModerationAppService`（買家）與 `ModerationAdminService`（`ListCases`、`GetCase`、`DecideCase`、`ReverseHide`，權限 `moderation.cases.list`、`get`、`decide`、`reverse`）；`DecideCase` 在結案的同一個 transaction 寫後台動作紀錄（`platform/adminaudit`，對象 `moderation_case`，detail 只放決定、處置、類別、對象種類與版本，不放理由文字）。後台待辦在 A7-1e。
- 依賴 social、review、im、seller、buyer、`accountsuspension`、media（快照圖片的引用）；不依賴 identity、不依賴 notification（A7-1d 不發事件，A8 依案件決定通知時再加）。
- 擁有的資料：`moderation.cases`、`reports`、`case_snapshots`、`reporter_daily_counts`、`media_ref_sync`、`retired_assets`（快照圖片引用的同步，演算法在 media）；主鍵 UUIDv7，不建跨 schema FK。
- 對象：貼文、留言、討論區留言、評價、聊天訊息（`content_id`）與買家（`buyer_id`）。另有店家對評價的回覆（`REVIEW_REPLY`，`content_id` 是評價 ID）。檢舉人是買家或店家（`reporter_buyer_id`、`reporter_store_id` 恰好一欄）：店家只能檢舉聊天訊息；評價、評價回覆與其他對象只有買家檢舉。
- 發布：`moderation.pub_review_items(at)`（後台待辦，見 §7）；`db/objects.sql` 放函式與複合型別，`db/pub.sql` 是消費端 sqlc 讀的形狀。

## 2. 送出檢舉（SubmitReport）

- 輸入：檢舉人 `ReporterBuyerID`、對象、原因類別、選填說明（去頭尾空白，最多 500 字）。原因只能是 `policy.CommunityReporterSelectable()` 的八個值（SPAM、HARASSMENT、HATE、SEXUAL_CONTENT、MISLEADING_INFO、FOOD_SAFETY、FRAUD、OTHER）；seller 通報用的 `ReporterSelectable()` 不變。
- 對象必須存在而且公開可讀（social `ContentSnapshot` 的 `Exists` 與 `Readable`，歸屬只查一次；買家對象是 buyer 的公開顯示資料存在且沒有刪除），否則 NotFound（被隱藏的內容也回 NotFound，不洩漏）。
- 不能檢舉自己：買家作者與買家對象比對帳號（同一帳號的歷史身分也算自己），店家作者比對店家所屬帳號；回 InvalidInput。
- 順序：帶 `request_id` 時最先以 (檢舉人, `request_id`) 查（`reports_request_key`），查到就回第一次的檢舉，不看案件現在的狀態、不吃額度，內容（對象、類別、說明的雜湊）不同回 InvalidInput（A7-D28 第 3 點）→ 先查這位檢舉人在這個對象的進行中案件裡有沒有檢舉，有就回傳既有的（`Created` 為假，不吃額度，不管內容現在還看不看得到）→ 沒有才做不鎖列的讀取（對象、作者、快照）→ 扣每日額度 → 開案或取進行中案件 → 掛進既有案件時 `version` 加一並鎖住案件列（已結案就整個重來；自己開的案件版本是 1）→ 快照 get-or-create（新快照在同 transaction 登記圖片引用）→ 寫檢舉。同一位檢舉人並行重送在最後一步撞唯一鍵：整個 transaction 回滾（額度與版本退回）重來，第二次回傳既有的。
- 每日上限：每位檢舉人（買家與店家各自計數，上限相同）每個台北日（`platform/twtime.Day`）最多新增 20 筆，單一 upsert 原子地換日重算、未達上限才加一；到上限且是新檢舉回 ResourceExhausted，重試時間是到下一個台北午夜。
- 案件：`INSERT … ON CONFLICT DO NOTHING` 之後讀進行中的案件；一個對象至多一個進行中案件（內容與買家兩個 partial UNIQUE），檢舉掛進它。結案後新檢舉開新案件。
- 快照：內容由擁有者模組提供（social 的本文、媒體參照與作者；buyer 的公開顯示名稱與頭像；review 經 `ReviewForReport` 的評價、評分與店家回覆；im 經 `MessageForReport` 的被檢舉訊息與 seq 前後各 `ChatContextWindow`（5）則，前後文已隱藏或已抹除的是墓碑、不帶內容；兩者可讀規則同顯示路徑、不存在與不可讀都是 NotFound，檢舉人要是對話的一方）。快照是依對象種類的 tagged union（`snapshot.Content`：Social、Buyer、Review、Chat 恰好一個有值），聊天快照的圖片資產是被檢舉訊息與前後文圖片的聯集；快照由 `snapshot.Encode` 正規化（換行統一、去頭尾空白、媒體參照排序去重）後序列化成確定的 JSON，雜湊取 SHA-256；每案每份內容一列，內容改過才多一份；檢舉引用送出時的那份。快照引用的圖片資產（`asset_ids`）在 media 以 `(GOVERNANCE_EVIDENCE, 快照 ID)` 登記引用（A7-D21 證據保全）：引用只由同步 job 寫入，內容的作者換圖、刪貼文或店家退場釋放自己的引用時，圖片仍被快照引用，不會被 media 回收。買家頭像只存 object key，沒有 asset ID，不在保全範圍。

## 3. 處理案件（DecideCase）

- 輸入：案件、管理員看到的版本、決定。`NO_VIOLATION` 不帶其他欄位；`VIOLATION` 帶認定類別、處置種類與給作者的理由（必填、最多 500 字）。
- 處置種類與對象：`HIDE` 對留言、討論區留言、評價與聊天訊息（作者必須是買家：評價作者永遠是買家，聊天訊息的寄件方取自快照，店家寄的回 FailedPrecondition）；`WARN` 與 `BAN` 對留言、討論區留言、評價、聊天訊息與買家（作者必須是買家）；`TAKEDOWN` 對貼文、留言、討論區留言、聊天訊息與評價回覆（作者必須是店家；聊天訊息的寄件方取自快照，評價回覆的店家是評價所屬店家）。聊天訊息與評價回覆的下架措施建立時，im、review 的同 transaction 訂閱者鎖內容列並核對歸屬（不符回 `TakedownContentError`、措施回滾），可見性由讀取述詞 join `seller.pub_content_takedowns()`，不建複本；下架的訊息與 HIDE 一樣是墓碑，回覆下架只遮回覆（店家自己的視圖也遮）、評價照常。買家分支的類別限於 `CommunityAssessable()`，店家分支限於 `Assessable()`。
- 內容已被刪除時：`HIDE` 視為成功（隱藏的目的已達成）、不呼叫擁有者、不寫隱藏紀錄，案件照常結案（評價被刪帳清除、擁有者回 NotFound 也一樣）；`TAKEDOWN` 用快照凍結的店家驗證歸屬並建立措施；`WARN` 與 `BAN` 針對作者，作者取自案件快照裡凍結的作者，不因作者刪掉內容而躲過。評價與聊天訊息的作者一律取快照凍結的作者，不回頭問擁有者。
- 評價與聊天訊息的隱藏與還原不取擁有者的守門（`LockActive`、`LockNotClosed`、`LockNotRetired`）：守門擋的是停用身分產生新資料，處分不產生；取了反而讓已停用作者的處分失敗。
- 買家分支：留言與討論區留言的 `HIDE`、全部對象的 `WARN` 呼叫 social（來源參照 `moderation:case:<案件 ID>`）；評價的 `HIDE` 呼叫 `review.Hide`、聊天訊息的 `HIDE` 呼叫 `im.Hide`（同一個來源參照）；`BAN` 先經 buyer 把買家轉成帳號再呼叫 `accountsuspension.Suspend`，有店的帳號回 `ACCOUNT_HAS_STORE`。
- 店家分支：管理員指定店家，必須與 social 回報的作者店家與所屬店家一致，才呼叫 seller `CreateMeasureInTx` 建立內容下架措施（認定引用案件最早的 20 筆檢舉，`finding_moderation_reports`；備註寫真正的檢舉數）。
- 全部在一個 transaction：處置之後最後才寫案件的決定；結案 UPDATE 以 `(id, version, closed_at IS NULL)` 轉移，0 列回 VersionConflict 並回滾處置；管理員讀到版本之後有新檢舉掛進來，版本已加一，結案就衝突，所以處置（含內容下架的來源檢舉）一定基於管理員看過的檢舉集合。案件已結案、不存在、內容已不存在各有明確的錯誤。
- 案件結案是終態：不重開、不改判；HIDE 的更正是 §3.1 的撤銷，其他處置的更正在擁有處分的模組做。處置頁在讀取時查措施現況（seller API），不訂閱 `MeasureRevoked`。

### 3.1 撤銷隱藏（ReverseHide）

- 輸入：案件、管理員（`adminaudit.Actor`）、原因（1 到 500 字）。只對已結案、處置是 `HIDE`、還沒撤銷的案件：進行中、`WARN`／`BAN`／`TAKEDOWN`、不違規、已撤銷過都是 FailedPrecondition；不存在是 NotFound。
- 案件維持結案、決定不改；`cases.reversed_at`、`reversed_by_admin_id`、`reverse_reason` 三欄同有同無（CHECK 只允許 HIDE 案件、撤銷不早於結案）。
- 同一個 transaction：先讀案件（不鎖）、呼叫擁有內容的模組 `Restore`（review、im、social；帶來源參照 `moderation:case:<案件 id>`，只還原來源相同的未還原隱藏：沒有未還原的隱藏、或未還原的隱藏是別的案件造成的（`Hide` 冪等，後到的案件沿用先到那一筆）都回該模組的 FailedPrecondition、什麼都不寫；內容已被清除或刪除（擁有者回 NotFound）時視為成功，同 `Hide`，撤銷仍照常記下）、最後寫案件的撤銷欄位與 `adminaudit.Record`。鎖序 review／im／social → moderation，同 `Decide`。
- 沿用別案隱藏的案件不能撤銷：HIDE 若沿用了別的案件已造成的隱藏（`Hide` 冪等），該案件的 ReverseHide 回 FailedPrecondition；只有造成這筆隱藏的案件能撤銷它，要還原需撤銷那個案件。稽核 detail 的 `restored` 記擁有者是否真的還原（內容已清除為 false）。
- 對外：`ModerationAdminService.ReverseHide`，權限 `ModerationCasesReverse`（`moderation.cases.reverse`；一支方法一個權限，不與 `DecideCase` 共用），宣告 `audited`。

## 4. 檢舉結果的推導

檢舉不存結果：`Case` 回傳的每筆檢舉的結果由所屬案件的決定推導：案件進行中是 `PENDING`，`VIOLATION` 與 `NO_VIOLATION` 照案件。不沿用 seller 的逐筆 outcome。

## 5. 快照保留期

結案滿 184 天（與 seller 措施申覆期限同值，不短於它）由 `moderation.purge_snapshots`（每日，啟動時也跑一次）分批刪除快照（同 transaction 把快照引用的資產記成退場並排入同步 job，由 job 解除 media 的引用）並清掉該案檢舉的說明文字；`reports.snapshot_id` 因 `ON DELETE SET NULL` 清空；案件與檢舉列留著。進行中的案件的快照與說明不刪。

檢舉人刪帳（買家的 `BuyerDeletionRequested`、`BuyerDeleted`，同 transaction 的訂閱者，含同一帳號的歷史身分）：清掉他全部檢舉的說明與 `reporter_daily_counts` 的列；檢舉列與 `reporter_buyer_id`（假名鍵）留著（A7-D21）。

鎖順序：moderation 的列排在 social、seller、buyer、`accountsuspension` 之後（`platform/lockorder.Modules`）；送出時依序是 `reporter_daily_counts`、`cases`、`case_snapshots`、`media_ref_sync`；處理時先呼叫擁有處分的模組、最後才寫 `cases`；刪帳訂閱者在 accountdeletion 鎖完買家與店家之後才鎖 `reports`、`reporter_daily_counts`。

## 6. 終態與觸發

| 現在的做法 | 終態 | 觸發 |
|---|---|---|
| 一次處理只有一種處置 | 同一案件組合多種處置 | 產品要求同時隱藏與警告 |
| 濫用檢舉人不逐筆標記 | 依 `NO_VIOLATION` 比例或「明顯無根據」的決定值限制檢舉人 | 營運回報濫用檢舉 |

## 7. 後台待辦（pub_review_items）

- `moderation.pub_review_items(at)` 一列是一個進行中案件（`cases.closed_at IS NULL`，與部分索引 `cases_open_by_time` 同一個判定式，和 `decision IS NULL` 等價，由 `cases_closed_together_check` 保證；A7-D14 一案一列）。沒有排序，reviewqueue 排序與分頁。
- 欄位：`kind` 固定 `MODERATION_CASE`；`item_id` 與 `case_id` 是案件 ID（`item_id` 是它的文字）；`start_at`、`due_at`。
- 期限（上線前的起始值）：從開案時間 `opened_at`（首筆檢舉的提交時間，`SubmitReport` 開案時寫入）起 72 小時，開始（轉黃）是期限前 24 小時，起始值取自 seller 非食安通報的分案期限，是 moderation 自己的值，沒有即時連動；社群類別沒有食安，不用 4 小時那一檔。時長只寫在 `db/objects.sql` 的 `review_moderation_case(at)`；`at` 由呼叫方傳入，不用 `now()`，期限與 `at` 無關。
- 守門：`reviewqueue` 的 `TestModerationCasesAreInTheQueueExactlyWhenOpen`（每種領域狀態進不進待辦）、`TestCrossSourceKeysetPagingWithModeration`；`cmd/hawkr` 的 `TestPubContractsMatchMigrations` 比對 `pub.sql`。

## 8. 我的檢舉的合併清單（`ListMyReports`，組裝在 `internal/httpapi/moderation`）

- 呼叫者的全部檢舉（社群與店家）是一支清單，模組之間不互相依賴，所以合併不在任何模組：`httpapi/moderation` 的 `App.ListMyReports` 呼叫 moderation 的 `MyReports` 與 seller 的 `BuyerReports`（買家一律取自 session 的買家身分，只讀自己的），在 handler 內合併。店家檢舉的單筆讀取與補充在 `hawkr.seller.v1alpha1.SellerReportAppService`（`GetMyReport`、`AddReportSupplement`），不經這裡。
- 排序：`(create_time 新到舊、社群在前、id 新到舊)`，三個鍵都相同的兩筆順序固定（`mergedReport.before`）。
- 分頁：每頁兩邊各取 `size` 筆（模組以多取一筆判斷還有沒有下一頁，`Next` 非 nil 就是還有），合併後取前 `size` 筆。`page_size` 預設 20、上限取兩個來源上限的較小者。
- 游標（`myReportsCursor`）是不透明 token，記兩個來源各自的下一頁位置與各自是否已取完（`CommunityDone`、`StoreDone`）：每個來源的游標只推進到「已輸出的最後一筆」，本頁取到但沒輸出的留給下一頁；來源只有在 `Next == nil` 而且取到的每一筆都已輸出時才算取完，取完的來源之後不再讀。token 綁 procedure 名稱與買家（`rpc.PageScope`），偽造的 token 回 invalid_argument。
- 項目形狀：`MyReportItem` 是共用欄位（`id`、`category`、`description`、`create_time`、`resolve_time`）加 `oneof subject { community, store }`。社群的結果由案件的決定推導（§4），`resolve_time` 是案件結案時間；店家的 `store` 是完整的 `StoreReport`（結果與 `resolution_note` 是 seller 的），共用欄位由它複製而來，所以對店家檢舉在線上出現兩次（`step10f-3-design.md` H1b 接受的代價，盤點見 seller `CARRYOVER.md` 的 10f-1 段）。
- 守門：`merge_test.go` 的 `TestListMyReportsInterleavesBothSourcesByTimeAcrossPages`、`TestListMyReportsOrdersTiesCommunityFirstThenIDDescending`、`TestListMyReportsOneSourceExhaustedStopsBeingRead`、`TestListMyReportsTailCrossingThePageBoundaryKeepsTheUnemittedSideCursor`、`TestListMyReportsRejectsForgedCursors`；`my_reports_integration_test.go` 的 `TestListMyReportsMergesTheRealModulesInterleavedOwnerOnly`。
