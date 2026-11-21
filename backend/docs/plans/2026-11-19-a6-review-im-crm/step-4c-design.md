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
| im／review 訂閱者 | 訂 `MeasureCreated`／`MeasureRevoked`：im 鎖訊息列、核對寄件方是店家且對話屬該店、`NOTIFY`；review 鎖評價列、核對 `store_id` | 未做 |
| moderation | `TargetReviewReply`（proto enum、Go、CHECK、`make uitext`／`make connect`、admin 選項）；`AllowedOn` TAKEDOWN 加 CHAT_MESSAGE、REVIEW_REPLY；`takedown` 改用依種類的快照；freeze REVIEW_REPLY | 未做 |
| W3 | `reports.reporter_store_id`＋恰好一欄 CHECK；`report_requests`、`reporter_daily_counts` 同樣分檢舉人種類；只有 CHAT_MESSAGE 接受店家 | 未做 |

## 3. 鎖順序
seller（建立措施）→ im(13)／review(12) 訂閱者鎖內容列 → moderation(24)；與 §5.2 一致【推】。併發 integration test（比照 `TestHideRacesWithEraseAndSendWithoutDeadlock`）：未做。

## 4. 讀取述詞【量】
`pub_content_takedowns()` 是 SQL 函式，社群用複本是因為量測；im／review 依 A6-D21 不建複本。效能未量測（改動僅 EXISTS 子查詢，列表頁每頁至多 max_rows 則）。

## 5. 預定 mutation（未執行）
im 拿掉 `pub_content_takedowns()` 分支；review 回覆遮蔽拿掉；social 過濾拿掉（chat 種類使 `splitContents` 報錯）；seller CHECK link 分支改錯；店家檢舉人套到 REVIEW（W3 後）；拿掉 exactly-one CHECK（W3 後）。

## 6. 未決
Q1 下架的 CHAT_MESSAGE 在對話預覽：目前同 HIDE（墓碑）。Q2 REVIEW_REPLY 下架只遮回覆，店家自己的視圖也遮（目前如此）。Q3 店家檢舉人由哪個店員角色／權限送出、店家每日額度：來源未定。
