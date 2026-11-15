# moderation 模組行為規格

本文是單體 `moderation` 模組行為的唯一來源。社群內容與買家的檢舉：送出、案件、內容快照、處置。決策見計畫 `backend/docs/plans/2026-10-18-a7-social.md` 的 A7-D7、A7-D9、A7-D13、A7-D14、A7-D20 與 ADR `docs/adr/2026-10-29-account-suspension-and-moderation.md`。

## 1. 範圍與公開介面

- 根套件公開 `Module`：`SubmitReport`、`DecideCase`、`Case`、`PurgeSnapshots`、`AddWorkers`、`PeriodicJobs`，以及只靠 DB、buyer、media 與 job 佇列的 `Retention`（`NewRetention`：快照與說明的保留期、快照圖片的 media 引用、買家刪除事件的訂閱者；worker 與事件組裝只組它，不組完整模組）。對外 API（Connect handler）與後台待辦在 A7-2、A7-1e。
- 依賴 social、seller、buyer、`accountsuspension`、media（快照圖片的引用）；不依賴 identity、不依賴 notification（A7-1d 不發事件，A8 依案件決定通知時再加）。
- 擁有的資料：`moderation.cases`、`reports`、`case_snapshots`、`reporter_daily_counts`、`media_ref_sync`、`retired_assets`（快照圖片引用的同步，演算法在 media）；主鍵 UUIDv7，不建跨 schema FK。
- 對象：貼文、留言、討論區留言（`content_id`）與買家（`buyer_id`）。聊天與評價對象在 A6-4。
- 發布：`moderation.pub_review_items(at)`（後台待辦，見 §7）；`db/objects.sql` 放函式與複合型別，`db/pub.sql` 是消費端 sqlc 讀的形狀。

## 2. 送出檢舉（SubmitReport）

- 輸入：檢舉人 `ReporterBuyerID`、對象、原因類別、選填說明（去頭尾空白，最多 500 字）。原因只能是 `policy.CommunityReporterSelectable()` 的八個值（SPAM、HARASSMENT、HATE、SEXUAL_CONTENT、MISLEADING_INFO、FOOD_SAFETY、FRAUD、OTHER）；seller 通報用的 `ReporterSelectable()` 不變。
- 對象必須存在而且公開可讀（social `ContentSnapshot` 的 `Exists` 與 `Readable`，歸屬只查一次；買家對象是 buyer 的公開顯示資料存在且沒有刪除），否則 NotFound（被隱藏的內容也回 NotFound，不洩漏）。
- 不能檢舉自己：買家作者與買家對象比對帳號（同一帳號的歷史身分也算自己），店家作者比對店家所屬帳號；回 InvalidInput。
- 順序：先查這位檢舉人在這個對象的進行中案件裡有沒有檢舉，有就回傳既有的（`Created` 為假，不吃額度，不管內容現在還看不看得到）→ 沒有才做不鎖列的讀取（對象、作者、快照）→ 扣每日額度 → 開案或取進行中案件 → 掛進既有案件時 `version` 加一並鎖住案件列（已結案就整個重來；自己開的案件版本是 1）→ 快照 get-or-create（新快照在同 transaction 登記圖片引用）→ 寫檢舉。同一位檢舉人並行重送在最後一步撞唯一鍵：整個 transaction 回滾（額度與版本退回）重來，第二次回傳既有的。
- 每日上限：每位檢舉人每個台北日（`platform/twtime.Day`）最多新增 20 筆，單一 upsert 原子地換日重算、未達上限才加一；到上限且是新檢舉回 ResourceExhausted，重試時間是到下一個台北午夜。
- 案件：`INSERT … ON CONFLICT DO NOTHING` 之後讀進行中的案件；一個對象至多一個進行中案件（內容與買家兩個 partial UNIQUE），檢舉掛進它。結案後新檢舉開新案件。
- 快照：內容由擁有者模組提供（social 的本文、媒體參照與作者；buyer 的公開顯示名稱與頭像），`snapshot.Encode` 正規化（換行統一、去頭尾空白、媒體參照排序去重）後序列化成確定的 JSON，雜湊取 SHA-256；每案每份內容一列，內容改過才多一份；檢舉引用送出時的那份。快照引用的圖片資產（`asset_ids`）在 media 以 `(GOVERNANCE_EVIDENCE, 快照 ID)` 登記引用（A7-D21 證據保全）：引用只由同步 job 寫入，內容的作者換圖、刪貼文或店家退場釋放自己的引用時，圖片仍被快照引用，不會被 media 回收。買家頭像只存 object key，沒有 asset ID，不在保全範圍。

## 3. 處理案件（DecideCase）

- 輸入：案件、管理員看到的版本、決定。`NO_VIOLATION` 不帶其他欄位；`VIOLATION` 帶認定類別、處置種類與給作者的理由（必填、最多 500 字）。
- 處置種類與對象：`HIDE` 只對留言與討論區留言（作者必須是買家）；`WARN` 與 `BAN` 對留言、討論區留言與買家（作者必須是買家）；`TAKEDOWN` 對貼文、留言與討論區留言（作者必須是店家）。買家分支的類別限於 `CommunityAssessable()`，店家分支限於 `Assessable()`。
- 內容已被刪除時：`HIDE` 與 `TAKEDOWN` 沒有東西可以處置，回 FailedPrecondition、案件維持進行中（管理員可以改選不違規、警告或停權）；`WARN` 與 `BAN` 針對作者，作者取自案件快照裡凍結的作者，不因作者刪掉內容而躲過。
- 買家分支：`HIDE` 與 `WARN` 呼叫 social（來源參照 `moderation:case:<案件 ID>`）；`BAN` 先經 buyer 把買家轉成帳號再呼叫 `accountsuspension.Suspend`，有店的帳號回 `ACCOUNT_HAS_STORE`。
- 店家分支：管理員指定店家，必須與 social 回報的作者店家與所屬店家一致，才呼叫 seller `CreateMeasureInTx` 建立內容下架措施（認定引用案件最早的 20 筆檢舉，`finding_moderation_reports`；備註寫真正的檢舉數）。
- 全部在一個 transaction：處置之後最後才寫案件的決定；結案 UPDATE 以 `(id, version, closed_at IS NULL)` 轉移，0 列回 VersionConflict 並回滾處置；管理員讀到版本之後有新檢舉掛進來，版本已加一，結案就衝突，所以處置（含內容下架的來源檢舉）一定基於管理員看過的檢舉集合。案件已結案、不存在、內容已不存在各有明確的錯誤。
- 案件結案是終態：不重開、不改判；更正在擁有處分的模組做。處置頁在讀取時查措施現況（seller API），不訂閱 `MeasureRevoked`。

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
