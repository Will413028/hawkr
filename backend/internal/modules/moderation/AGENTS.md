# moderation 模組

## 職責

社群內容（貼文、留言、討論區留言）與買家的檢舉：送出、案件、內容快照與處置。管理員對案件下決定，處置經擁有處分的模組建立：買家寫的內容隱藏或警告在 social、買家帳號停權經 `accountsuspension`、店家寫的內容下架是 seller 的措施。行為規格的唯一來源是 `backend/docs/modules/moderation.md`；聊天與評價對象在 A6，店家與商品的通報與案件留在 seller。

## 擁有的資料

schema `moderation`（表在 `db/schema.sql`，發布的函式與複合型別在 `db/objects.sql`）：`cases`（案件與決定）、`reports`（檢舉，不存結果）、`case_snapshots`（內容快照與它引用的圖片資產）、`reporter_daily_counts`（每日上限）、`media_ref_sync` 與 `retired_assets`（快照圖片在 media 的引用同步）。讀取模型：無。

## 發布的讀取介面

`moderation.pub_review_items(at)`：後台待辦的 moderation 部分（reviewqueue 讀，契約在 `db/pub.sql`），一列一個進行中案件（`closed_at IS NULL`），種類 `MODERATION_CASE`。期限的時長只寫在 `objects.sql` 的 `review_moderation_case(at)`，規格見 `backend/docs/modules/moderation.md` §7。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與 River 登記。
- `internal/model` 型別與值域；`report` 送出；`handle` 處理與結案；`snapshot` 快照的正規化、雜湊與讀回；`retention` 快照與說明的保留期、買家刪除事件的訂閱者；`mediaref` 快照圖片的 media 引用（演算法在 media）；`text` 伺服器句子（`make uitext`）；`view` 讀取與轉換；`store` 是 sqlc。`Retention`（`NewRetention`）是 worker 與事件組裝用的子集，不需要 social、seller、accountsuspension。
- 依賴圖的唯一來源是 `internal/arch/modules.go`；依賴 social、seller、buyer、`accountsuspension`、media，不依賴 identity（停權只經 `accountsuspension`），不依賴 notification。

## 不變量

- 一個對象至多一個進行中案件：partial UNIQUE `cases_one_open_content`、`cases_one_open_buyer`；開案用 `INSERT … ON CONFLICT DO NOTHING` 再讀（`TestThirtyReportsShareOneOpenCaseAndOutcomesFollowTheDecision`）。
- 案件結案是終態，不重開、不改判；結案 UPDATE 以 `(id, version, closed_at IS NULL)` 轉移，0 列回 `VersionConflict` 並回滾處置（`TestDecideRollsBackTheActionWhenCloseLosesTheRace`）；新檢舉掛進既有案件讓 `version` 加一，管理員讀到之後才掛進來的檢舉讓結案衝突（`TestAReportAttachedAfterTheAdminReadMakesTheCloseConflict`）。決定只存在案件，檢舉的結果由案件推導，不存逐筆 outcome。
- 去重是 `UNIQUE (case_id, reporter_buyer_id)`：進行中案件重送回傳既有檢舉、不吃額度、不管內容現在看不看得到，結案後可再檢舉（`TestResendAfterTheContentIsHiddenReturnsTheExistingReport`）。
- 冪等：檢舉帶 `request_id` 時，`UNIQUE (reporter_buyer_id, request_id)` 加 `request_hash` 比對內容；同一位檢舉人的同一個 `request_id` 一律回第一次的檢舉，先於「進行中案件去重」與每日額度，不看案件現在的狀態（A7-D28 第 3 點，`TestRequestIDResendReturnsTheFirstResultEvenAfterTheCaseIsClosed`）；內容不同是 InvalidInput。對外只回檢舉 ID 與收件時間，不露案件 ID。
- 每日上限（`model.DailyReportLimit`）是單一 upsert（`BumpDailyCount`），重送已檢舉過的對象在扣額度之前回傳（`TestConcurrentSubmitsAreCappedAtTheDailyLimit`、`TestAtTheLimitAResendSucceedsButANewTargetIsRefused`）。
- 快照每案每份內容一列，`UNIQUE (case_id, content_sha256)`；雜湊取 `snapshot.Encode` 正規化後的內容；結案滿 `model.SnapshotRetention` 後由 `moderation.purge_snapshots` 刪除（`TestSnapshotsArePurgedAfterTheRetentionAndCasesStay`）並清掉檢舉說明（`TestDescriptionsAreClearedWithTheSnapshots`），案件與檢舉列留著。
- 快照引用的圖片在 media 有一筆以快照為 ref 的引用，引用只由同步 job 寫入，快照刪除才解除（`TestASnapshotHoldsItsImageUntilTheSnapshotIsPurged`）；檢舉人刪帳時同 transaction 清掉他的說明與每日計數列，檢舉列與 `reporter_buyer_id` 留著（`TestReporterDeletionClearsDescriptionsAndDailyCountButKeepsTheReports`）。
- 內容已被刪除時，警告與停權用快照裡凍結的作者，隱藏與內容下架回 FailedPrecondition（`TestWarnAndBanUseTheFrozenAuthorWhenTheContentIsGone`）。
- 處置與結案在同一個 transaction；案件列最後才寫：擁有處分的模組（social、identity、seller）的列都排在 moderation 之前，先呼叫它們、後寫自己的列（`internal/platform/lockorder`）；每次處理只走一條處置路徑。送出時依序鎖 `reporter_daily_counts`、`cases`、`case_snapshots`、`media_ref_sync`；刪帳訂閱者在 accountdeletion 鎖完買家與店家之後才鎖 `reports` 與 `reporter_daily_counts`。
- 店家寫的內容只能走內容下架，內容必須經 social 驗證屬於管理員指定的店家；買家不能隱藏或警告店家，店家不能被停權。
- 對象必須存在而且公開可讀，不能檢舉自己（含同一帳號的其他身分）；檢舉原因只能是 `policy.CommunityReporterSelectable()` 的八個值。

## 不負責

- 處分的紀錄與效果：social（隱藏、警告）、identity（停權紀錄）、seller（措施與申覆）。
- 店家與商品的通報、案件與分案：seller 的 governance。
- 後台的待辦與逾期（A7-1e）、對外 API（A7-2）、通知（A8）、聊天與評價對象（A6）。

## 決策紀錄

沿用的機制與差異在 `CARRYOVER.md`；案件粒度、決定與結果推導、快照與保留期在 ADR `docs/adr/2026-10-29-account-suspension-and-moderation.md`。
