# A6-4c 店家寫的內容的下架與店家檢舉人：實作設計（W2 (a)、W3 (a)，A6-D21）

上層：`step-4-design.md` §1、§2、§5.2、§W2、§W3。標記：【量】＝打開檔案或跑過指令；【推】＝讀程式推論。本檔隨實作分批更新；「狀態」欄標明已做與未做。

## 1. 既有社群下架路徑的檢查清單【量】

moderation `Decide` → `apply`：作者取自案件快照或 social `ContentOwner` → `takedown`：快照必須有內容、`CheckTakedownStore`（作者、歸屬都是快照的店家）→ `ActiveTakedownOfContent` 已有生效措施就綁定既有措施 → 讀來源檢舉 → `measureSnapshot`（`seller.NewContentSnapshot`＋`ValidateContentSnapshot`）→ `CreateMeasureInTx` → seller 發 `MeasureCreated` → social 訂閱者 `ApplyTakedowns`（鎖內容列、核對歸屬、寫複本）→ 讀取述詞看複本。撤銷與申覆：seller 發 `MeasureRevoked` → `ReleaseTakedowns`。每日對帳讀 `pub_content_takedowns()`（只認三種社群種類）。

## 2. 各模組

| 模組 | 改動 | 狀態 |
|---|---|---|
| `platform/contentkind` | 加 `CHAT_MESSAGE`、`REVIEW_REPLY`；`Social()` 區分社群種類 | 已做 |
| seller | 模型常數；`NewContentSnapshot` 與 `validateSnapshot`：非社群種類連結必須全空；`measure_contents_content_kind_check` 與 `measure_contents_link_check` 的 forward migration `20261121101416` | 已做 |
| social | `OnMeasureCreated`／`OnMeasureRevoked` 只處理社群種類（`takedownContents` 過濾）；對帳查詢原本以 kind 分支，不受影響【量】 | 已做 |
| im | `messages.sql`、`conversations.sql` 的 `hidden` 改為「未還原的隱藏 或 `pub_content_takedowns()` 有該訊息」，涵蓋列表、補洞、預覽、relay、`MessagesForReport`；下架與 HIDE 在預覽上同一種墓碑（待確認，見 §6 Q1） | 已做 |
| review | 回覆下架只遮回覆、評價照常：`TakenDownReplyIDs` 查詢（join `pub_content_takedowns()`），`read.go` 的列表與單筆把 `Reply` 設為 nil；`REVIEW_REPLY` 的 `content_id` 是評價 ID（回覆沒有自己的 ID）【量：`reviews.reply_content`】 | 已做 |
| im／review 訂閱者 | 訂 `MeasureCreated`／`MeasureRevoked`：im（`write/takedown.go`）鎖訊息列（依 id 升序）、核對寄件方是店家且對話屬該店、對每則發 `NOTIFY`；review 鎖評價列、核對 `store_id`，不發通知（回覆不進評分彙總，review 沒有即時通道）。對象不存在成立（同 social：已清除內容的歸屬由 moderation 以快照核對）；歸屬不符回 `TakedownContentError` 讓措施回滾。`cmd/hawkr` 的 `newIM`、`newReview` 接線 | 已做；integration 未跑 |
| moderation | `TargetReviewReply`（proto enum 7、Go、`cases_target_kind_check`、`cases_action_target_check`、`make uitext`／`make connect`、admin `TARGET_KIND_OPTIONS`）；`AllowedOn` TAKEDOWN 加 CHAT_MESSAGE、REVIEW_REPLY；`takedown` 依快照種類組措施快照（`measureSnapshot`，聊天與回覆連結全空）；`snapshot.ReviewReply`（Content 的第五個種類，作者與歸屬是評價所屬店家）；freeze REVIEW_REPLY（`ReviewForReport`，回覆不存在或已被下架是 NotFound）；admin `CaseSnapshot` 加 `review_reply = 5` | 已做；integration 未跑 |
| W3 | `reports`、`report_requests`、`reporter_daily_counts` 加 `reporter_store_id`＋恰好一欄 CHECK，去重鍵與冪等鍵與每日計數依檢舉人種類各一份（`UNIQUE (case_id, reporter_store_id)`、`report_requests_store_key`、`reporter_daily_counts_store_key`，額度 20、買家與店家各自計數）；`SubmitInput.ReporterStoreID`（與 `ReporterBuyerID` 恰好一個為正）；店家只能檢舉 CHAT_MESSAGE，而且要通過 `im.Party.Authorize`（`CapChatRead`）與 `MessageForReport` 的對話方核對；本人核對改以帳號比（`reporterAccount`） | 已做；Connect handler 接線（App 的檢舉 proto 與 session 解出店家）是 A6-5 |

## 3. 鎖順序
seller（建立措施）→ im(13)／review(12) 訂閱者鎖內容列 → moderation(24)；與 §5.2 一致【推】。併發 integration test `TestTakedownRacesWithEraseAndSendWithoutDeadlock`、`TestReplyTakedownRacesWithReviewHide`（`takedown_chat_review_integration_test.go`）：已寫、本機不跑，交 CI。

## 4. 讀取述詞【量】
`pub_content_takedowns()` 是 SQL 函式，社群用複本是因為量測；im／review 依 A6-D21 不建複本。效能未量測（改動僅 EXISTS 子查詢，列表頁每頁至多 max_rows 則）。

## 5. 預定 mutation（未執行；本機不跑容器，交 CI 與 `scripts/red-proof.py`，不得在工作目錄編輯後還原）

| # | mutant | 要失敗的測試 |
|---|---|---|
| 1 | im 預覽與列表的 `pub_content_takedowns()` 分支拿掉 | `TestStoreChatTakedownTombstonesAndRevokeRestores` |
| 2 | review 回覆遮蔽（`takenDownReplies`）拿掉 | `TestReviewReplyTakedownHidesOnlyTheReply` |
| 3 | social `takedownContents` 的 `Social()` 過濾拿掉（聊天種類使 `splitContents` 報錯） | `TestStoreChatTakedownTombstonesAndRevokeRestores` |
| 4 | seller `measure_contents_link_check` 的非社群分支改成允許連結 | seller `TestChecksMatchGo` 與 `TestNonSocialContentKindsRequireEmptyLinks` |
| 5 | 店家檢舉人套到 REVIEW（`Validate` 的種類限制拿掉） | `TestStoreReporterIsAcceptedOnlyForChatMessages`、report 單元測試 |
| 6 | 拿掉 `reports_reporter_kind_check`（恰好一欄） | `TestChecksMatchGo`、`TestReporterKindCheckRejectsNoneAndBoth` |
| 7 | im 訂閱者拿掉歸屬核對 | `TestTakedownRejectsAMessageThatIsNotTheMeasureStores` |
| 8 | `BumpStoreDailyCount` 改成讀買家的計數列 | `TestStoreDailyQuotaIsCountedSeparatelyFromTheBuyer` |
| 9 | 把 `CloseCase` 移到 `Seller.CreateMeasureInTx` 之前 | lockorder 檢查器（併發測試） |

## 6. 已決（Will 2026-11-21）
- 下架的店家聊天訊息與 HIDE 一樣在列表與預覽顯示墓碑；REVIEW_REPLY 下架只遮回覆，店家自己的視圖也遮（照現行實作）。
- seller 沒有員工角色模型：能在聊天回覆的店家帳號（`CapChatRead`）就能檢舉，不另設權限。
- 店家每日檢舉額度 20，同買家，依檢舉人種類各自計數。

## 7. 未做與限制
- admin 建立內容下架措施的 handler（`httpapi/seller/governance_measures.go`）維持只收社群種類：那條路徑的內容快照與存在、歸屬核對靠 social 的 `ContentOwner`；im、review 的訂閱者雖然核對歸屬，handler 沒有讀聊天訊息與評價回覆快照的來源，放寬等於讓管理員手填快照。CHAT_MESSAGE 與 REVIEW_REPLY 的下架只經 moderation 案件。
- 店家檢舉人的 Connect 入口、買家與店家 App 的檢舉 proto：A6-5／A6-6。店家退場時店家檢舉人的說明與每日計數清除：`OnBuyerDeletionRequested` 只處理買家；店家檢舉的說明由保留期 job 清（結案 184 天），店家刪除事件訂閱者未做，列入 A6-5 前待決。
- integration 與 mutation 本機沒有跑，CI 結果出來前這些測試的斷言屬【推】。
