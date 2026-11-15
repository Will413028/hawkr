# moderation 模組行為規格

本文是單體 `moderation` 模組行為的唯一來源。社群內容與買家的檢舉：送出、案件、內容快照、處置。決策見計畫 `backend/docs/plans/2026-10-18-a7-social.md` 的 A7-D7、A7-D9、A7-D13、A7-D14、A7-D20 與 ADR `docs/adr/2026-10-29-account-suspension-and-moderation.md`。

## 1. 範圍與公開介面

- 根套件公開 `Module`：`SubmitReport`、`DecideCase`、`Case`、`PurgeSnapshots`、`AddWorkers`、`PeriodicJobs`。對外 API（Connect handler）與後台待辦在 A7-2、A7-1e。
- 依賴 social、seller、buyer、`accountsuspension`；不依賴 identity、不依賴 notification（A7-1d 不發事件，A8 依案件決定通知時再加）。
- 擁有的資料：`moderation.cases`、`reports`、`case_snapshots`、`reporter_daily_counts`；主鍵 UUIDv7，不建跨 schema FK。
- 對象：貼文、留言、討論區留言（`content_id`）與買家（`buyer_id`）。聊天與評價對象在 A6-4。

## 2. 送出檢舉（SubmitReport）

- 輸入：檢舉人 `ReporterBuyerID`、對象、原因類別、選填說明（去頭尾空白，最多 500 字）。原因只能是 `policy.CommunityReporterSelectable()` 的八個值（SPAM、HARASSMENT、HATE、SEXUAL_CONTENT、MISLEADING_INFO、FOOD_SAFETY、FRAUD、OTHER）；seller 通報用的 `ReporterSelectable()` 不變。
- 對象必須存在而且公開可讀（social `ContentOwner` 的 `Exists` 與 `Readable`；買家對象是 buyer 的公開顯示資料存在且沒有刪除），否則 NotFound（被隱藏的內容也回 NotFound，不洩漏）。
- 不能檢舉自己：買家作者與買家對象比對帳號（同一帳號的歷史身分也算自己），店家作者比對店家所屬帳號；回 InvalidInput。
- 順序：不鎖列的讀取（對象、作者、快照）→ 查這位檢舉人在這個對象的進行中案件裡有沒有檢舉 → 有就回傳既有的（`Created` 為假，不吃額度）→ 沒有才扣每日額度 → 開案或取進行中案件 → 共享鎖案件（已結案就整個重來）→ 快照 get-or-create → 寫檢舉。同一位檢舉人並行重送在最後一步撞唯一鍵：整個 transaction 回滾（額度退回）重來，第二次回傳既有的。
- 每日上限：每位檢舉人每個台北日（`platform/twtime.Day`）最多新增 20 筆，單一 upsert 原子地換日重算、未達上限才加一；到上限且是新檢舉回 ResourceExhausted，重試時間是到下一個台北午夜。
- 案件：`INSERT … ON CONFLICT DO NOTHING` 之後讀進行中的案件；一個對象至多一個進行中案件，檢舉掛進它。結案後新檢舉開新案件。
- 快照：內容由擁有者模組提供（social 的本文、媒體參照與作者；buyer 的公開顯示名稱與頭像、是否刪除），`snapshot.Encode` 正規化（換行統一、去頭尾空白、媒體參照排序去重）後序列化成確定的 JSON，雜湊取 SHA-256；每案每份內容一列，內容改過才多一份；檢舉引用送出時的那份。

## 3. 處理案件（DecideCase）

- 輸入：案件、管理員看到的版本、決定。`NO_VIOLATION` 不帶其他欄位；`VIOLATION` 帶認定類別、處置種類與給作者的理由（必填、最多 500 字）。
- 處置種類與對象：`HIDE` 只對留言與討論區留言（作者必須是買家）；`WARN` 與 `BAN` 對留言、討論區留言與買家（作者必須是買家）；`TAKEDOWN` 對貼文、留言與討論區留言（作者必須是店家）。買家分支的類別限於 `CommunityAssessable()`，店家分支限於 `Assessable()`。
- 買家分支：`HIDE` 與 `WARN` 呼叫 social（來源參照 `moderation:case:<案件 ID>`）；`BAN` 先經 buyer 把買家轉成帳號再呼叫 `accountsuspension.Suspend`，有店的帳號回 `ACCOUNT_HAS_STORE`。
- 店家分支：管理員指定店家，必須與 social 回報的作者店家與所屬店家一致，才呼叫 seller `CreateMeasureInTx` 建立內容下架措施（認定引用案件最早的 20 筆檢舉，`finding_moderation_reports`）。
- 全部在一個 transaction：處置之後最後才寫案件的決定；結案 UPDATE 以 `(id, version, closed_at IS NULL)` 轉移，0 列回 VersionConflict 並回滾處置。案件已結案、不存在、內容已不存在各有明確的錯誤。
- 案件結案是終態：不重開、不改判；更正在擁有處分的模組做。處置頁在讀取時查措施現況（seller API），不訂閱 `MeasureRevoked`。

## 4. 檢舉結果的推導

檢舉不存結果：`Case` 回傳的每筆檢舉的結果由所屬案件的決定推導：案件進行中是 `PENDING`，`VIOLATION` 與 `NO_VIOLATION` 照案件。不沿用 seller 的逐筆 outcome。

## 5. 快照保留期

結案滿 184 天（與 seller 措施申覆期限同值，不短於它）由 `moderation.purge_snapshots`（每日，啟動時也跑一次）分批刪除快照；`reports.snapshot_id` 因 `ON DELETE SET NULL` 清空；案件與檢舉列留著。進行中的案件的快照不刪。

## 6. 終態與觸發

| 現在的做法 | 終態 | 觸發 |
|---|---|---|
| 一次處理只有一種處置 | 同一案件組合多種處置 | 產品要求同時隱藏與警告 |
| 濫用檢舉人不逐筆標記 | 依 `NO_VIOLATION` 比例或「明顯無根據」的決定值限制檢舉人 | 營運回報濫用檢舉 |
| 伺服器句子是 `apperr` 的字面字串 | 移進模組 `internal/text` 並由 `make uitext` 生成 | A7-2 開放對外 API |
