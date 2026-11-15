# moderation 模組

## 職責

社群內容（貼文、留言、討論區留言）與買家的檢舉：送出、案件、內容快照與處置。管理員對案件下決定，處置經擁有處分的模組建立：買家寫的內容隱藏或警告在 social、買家帳號停權經 `accountsuspension`、店家寫的內容下架是 seller 的措施。行為規格的唯一來源是 `backend/docs/modules/moderation.md`；聊天與評價對象在 A6，店家與商品的通報與案件留在 seller。

## 擁有的資料

schema `moderation`（`db/schema.sql`）：`cases`（案件與決定）、`reports`（檢舉，不存結果）、`case_snapshots`（內容快照）、`reporter_daily_counts`（每日上限）。讀取模型：無。

## 發布的讀取介面

無。

## 模組內部結構

- 根套件只保留 `Module`、`New`、轉呼叫方法、對外型別別名與 River 登記。
- `internal/model` 型別與值域；`report` 送出；`handle` 處理與結案；`snapshot` 快照的正規化與雜湊；`retention` 快照保留 job；`view` 讀取與轉換；`store` 是 sqlc。
- 依賴圖的唯一來源是 `internal/arch/modules.go`；依賴 social、seller、buyer、`accountsuspension`，不依賴 identity（停權只經 `accountsuspension`），不依賴 notification。

## 不變量

- 一個對象至多一個進行中案件：partial UNIQUE `cases_one_open`；開案用 `INSERT … ON CONFLICT DO NOTHING` 再讀（`TestThirtyReportsShareOneOpenCaseAndOutcomesFollowTheDecision`）。
- 案件結案是終態，不重開、不改判；結案 UPDATE 以 `(id, version, closed_at IS NULL)` 轉移，0 列回 `VersionConflict` 並回滾處置（`TestDecideRollsBackTheActionWhenCloseLosesTheRace`）。決定只存在案件，檢舉的結果由案件推導，不存逐筆 outcome。
- 去重是 `UNIQUE (case_id, reporter_buyer_id)`：進行中案件重送回傳既有檢舉、不吃額度，結案後可再檢舉。
- 每日上限（`model.DailyReportLimit`）是單一 upsert（`BumpDailyCount`），重送已檢舉過的對象在扣額度之前回傳（`TestConcurrentSubmitsAreCappedAtTheDailyLimit`、`TestAtTheLimitAResendSucceedsButANewTargetIsRefused`）。
- 快照每案每份內容一列，`UNIQUE (case_id, content_sha256)`；雜湊取 `snapshot.Encode` 正規化後的內容；結案滿 `model.SnapshotRetention` 後由 `moderation.purge_snapshots` 刪除（`TestSnapshotsArePurgedAfterTheRetentionAndCasesStay`），案件與檢舉列留著。
- 處置與結案在同一個 transaction；案件列最後才寫：擁有處分的模組（social、identity、seller）的列都排在 moderation 之前，先呼叫它們、後寫自己的列（`internal/platform/lockorder`）；每次處理只走一條處置路徑。
- 店家寫的內容只能走內容下架，內容必須經 social 驗證屬於管理員指定的店家；買家不能隱藏或警告店家，店家不能被停權。
- 對象必須存在而且公開可讀，不能檢舉自己（含同一帳號的其他身分）；檢舉原因只能是 `policy.CommunityReporterSelectable()` 的八個值。

## 不負責

- 處分的紀錄與效果：social（隱藏、警告）、identity（停權紀錄）、seller（措施與申覆）。
- 店家與商品的通報、案件與分案：seller 的 governance。
- 後台的待辦與逾期（A7-1e）、對外 API（A7-2）、通知（A8）、聊天與評價對象（A6）。

## 決策紀錄

沿用的機制與差異在 `CARRYOVER.md`；案件粒度、決定與結果推導、快照與保留期在 ADR `docs/adr/2026-10-29-account-suspension-and-moderation.md`。
