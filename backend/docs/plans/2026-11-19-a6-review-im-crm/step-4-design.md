# A6-4 moderation 加評價、聊天對象：實作設計 v1（A6-4 設計書，開工 design-review 第 1 輪已處理，§W 已決）

上層：計畫 `../2026-11-19-a6-review-im-crm.md` 的「A6-4」、A6-D3、A6-D19、A6-D20、不變量表與缺陷 RD3、ID1。路徑相對於 `backend/`；`M=internal/modules`、`P=internal/platform`。標記：【量】＝打開檔案或跑過指令；【推】＝讀程式推論；未查證＝沒有查到來源。核對時的 HEAD：dev `d9ac3908`。

## 0. 前提題：這一步的程式之後會不會被重寫、搬走或刪除？

| 程式 | 排程 | 來源 | 分類 |
|---|---|---|---|
| moderation 的對象種類、快照、處置分流 | 長期；moderation 是社群與 A6 對象的唯一檢舉入口 | `M/moderation/AGENTS.md`「職責」【量】 | — |
| review、im 的 `Hide`／`Restore` 與隱藏紀錄表 | 長期；A6-1、A6-2a 已建，本步只接上呼叫端 | `M/review/review.go:153-158`、`M/im/im.go:201-206`【量】 | — |
| Connect handler 與 admin／App 的檢舉入口 | A6-5／A6-6 才做；本步只改 proto enum 與 Go | 計畫 A6-5、A6-6 | 不在本步 |

結論：沒有過渡程式；照長期標準設計。

## 0.1 範圍

- `TargetKind` 加 `REVIEW`、`CHAT_MESSAGE`：proto `ReportTargetKind`、Go `model.TargetKind`、`cases_target_kind_check` 與 `cases_action_target_check` 的往前 migration、共用文案目錄的兩個 enum 條目（`make uitext`）。
- 檢舉送出：review、im 新增唯讀的快照 API；moderation 的 `freeze` 依種類分流；快照形狀擴充（§3）。
- 處置：HIDE／WARN／BAN 對兩種新對象的分流；HIDE 在 moderation 的 transaction 內呼叫 `review.Hide`／`im.Hide`。
- 還原：moderation 的還原入口（形狀見 §W1）在同一個 transaction 呼叫 `review.Restore`／`im.Restore`。
- 依賴表 `moderation += review, im`；lockorder 不用改（§5）。
- 測試、mutation、`CARRYOVER.md`／`AGENTS.md`／`docs/modules/moderation.md` 更新。

## 0.2 不在範圍

- Connect handler、admin 後台的對象選單（`admin/src/app/admin/moderation/moderation-text.ts` 的 `TARGET_KIND_OPTIONS`）、App 的檢舉入口：A6-5、A6-6。
- `reviewqueue`：`moderation.pub_review_items` 不分對象種類（`M/moderation/db/objects.sql:15-29`【量】），不改。
- review、im 的抹除 job、刪帳訂閱者：不動。
- 店家寫的內容（店家送的聊天訊息、評價的店家回覆）的處置效果：見 §W2，拍板前不實作。

## 0.3 決策總表（只寫落點）

| 來源 | 決定 | 本檔落點 |
|---|---|---|
| A6-D3 | 後台只看被檢舉訊息前後各 5 則，快照建立時固定 | §3.2 |
| A6-D19／A6-D20 | 停用身分的守門只在擁有者，寫入方用 `FOR KEY SHARE` | §5：moderation 的隱藏與還原**不取**守門 |
| 10e R12（不變量表） | 處分紀錄與效果同模組同 transaction，只有一個寫入者 | §2：只經 review、im 的模組 API |
| A7-D5′ | 店家的處分走 seller 措施 | §W2 |
| A7-D20／A7-D21 | 快照正規化、雜湊、證據保全 | §3 |
| 單體規則 4 | 由下往上鎖 | §5 |

## 0.4 查證過的現況【量】

| 事實 | 證據 |
|---|---|
| review 已有 `Hide`／`Restore`，transaction 內只鎖自己的評價列（`LockReview`＝`FOR NO KEY UPDATE`），新隱藏與還原都 bust 快取並發 `ReviewAggregateChanged`；不呼叫 buyer、seller | `M/review/internal/write/write.go:289-361`；`M/review/db/queries/own.sql:46-48` |
| im 已有 `Hide`／`Restore`，鎖訊息列（`LockMessage`，`FOR NO KEY UPDATE OF m`），同 tx 發 `NOTIFY`；不呼叫 buyer、seller；外層已有 transaction 時加入、不設自己的時間上限 | `M/im/internal/write/write.go:79-92,154-213`；`M/im/db/queries/hides.sql:3-6` |
| im 的對話預覽在讀取時推導，`LEFT JOIN im.hides … restored_at IS NULL`；沒有存預覽欄位，ID1 已在 A6-2a 從結構上消失 | `M/im/db/queries/conversations.sql:73,85`；`step-2a-design.md` §5.2、§9 第 1 項 |
| 兩張隱藏表：未還原的隱藏至多一筆（partial UNIQUE，`ON CONFLICT … WHERE restored_at IS NULL`），還原留列；`message_id` FK `ON DELETE CASCADE` | `M/review/db/queries/hides.sql:3-17`；`M/im/db/schema.sql:62-82` |
| review、im 的寫入守門：review `LockActive`、im `LockNotClosed`＋`LockNotRetired`（A6-3c1 已落地） | `M/review/internal/write/write.go:124`；`M/im/internal/write/send.go:69,73` |
| moderation 的 `TargetKind` 只有 POST、COMMENT、FORUM_ENTRY、BUYER；`Target` 的內容用 `ContentID uuid.UUID` | `M/moderation/internal/model/model.go:27-78` |
| `Action.AllowedOn` 與 DB 的 `cases_action_target_check` 一一對應 | `model.go:107-118`；`M/moderation/db/schema.sql:52-57` |
| `cases_one_open_content` 是 `(target_kind, content_id)`，UUID 的新對象直接適用 | `schema.sql:64` |
| 處置：`Decide` 一個 transaction；`GetCase` 不鎖；先呼叫擁有者模組，最後 `CloseCase`（moderation 自己的列）與 `adminaudit` | `M/moderation/internal/handle/handle.go:103-142,161-177` |
| HIDE 遇到內容已刪除：**不呼叫 social、回成功** | `handle.go:199-207`；`lifecycle_integration_test.go:289` `TestHideOfDeletedBuyerContentSucceedsWithoutCallingSocial` |
| 送出：`freeze` 是不鎖列的讀取（向 social 或 buyer），之後才依序鎖 moderation 自己的列 | `M/moderation/internal/report/report.go:135-200,290-316`；`internal/report/AGENTS.md` |
| 檢舉人只有買家（`reporter_buyer_id`） | `M/moderation/CARRYOVER.md`（reporter fallback 一列：「只有買家檢舉社群內容」）；`report.go:162` |
| 快照 `snapshot.Content` 是扁平 struct，`Encode` 正規化後取 SHA-256；`StoreOf` 只在作者是店家且 `AuthorID == StoreID` 時回店家 | `M/moderation/internal/snapshot/snapshot.go:17-77` |
| 全域鎖序：buyer(3) < seller(4) < review(12) < im(13) < social(16) < accountsuspension(23) < moderation(24) | `P/lockorder/lockorder.go:35-64` |
| 每支 integration test 的連線池都掛 lockorder 檢查器 | `P/testdb/testdb.go:274-296` |
| 依賴表 moderation 現為 social、seller、buyer、accountsuspension、media | `internal/arch/modules.go:37` |
| proto enum 由名稱對應（`rpc.EnumFromProto`），加值不用改 handler 的對應碼 | `internal/httpapi/moderation/app.go:62,80`；`admin.go:46` |
| `ReportTargetKind` 有共用文案目錄條目（四個值） | `P/uitext/catalog/zh-TW.json:179-192`、`en.json:48-59` |
| 後台沒有任何還原入口：`social.Restore` 沒有正式呼叫端；A7-D28 第 6 點修訂「不做 `RestorePost`」 | `rg -n "\.Restore\(ctx" --glob '!*_test.go' internal` 只命中三個模組自己的轉呼叫；`2026-10-18-a7-social.md:442` |
| 後台 `GetCase` 不帶快照內容；「GetCase 是否帶快照」在 A7 仍是待決 | `api/hawkr/admin/moderation/v1alpha1/moderation.proto:121-128`；`2026-10-18-a7-social.md:662`（2f-4 列末） |

### 0.4.1 與計畫或文件不符之處

1. **ID1 的 mutation**：計畫驗收寫「隱藏時不更新對話預覽要失敗」。im 沒有預覽欄位（上表），沒有「更新預覽」可拿掉。照 `step-2a-design.md` §9 第 1 項改成：「預覽推導拿掉 `hides` 述詞，A6-4 的整合測試要失敗」。
2. **聊天對象在買家單方檢舉下必然是店家寫的**：檢舉人只有買家；聊天的另一方只會是店家；檢舉自己的訊息被擋（`sameAccount`，`report.go:343-370`）。所以 `CHAT_MESSAGE` 案件的作者**全部是店家**，計畫寫的「隱藏經 im API」在現行規則下走不到（HIDE 對店家作者回 `DecideHideStoreContent`，`handle.go:199-201`）。這讓 §W2、§W3 成為本步的前提，不是枝節。
3. **文件和程式碼對不上**：`internal/handle/AGENTS.md:8` 與 `docs/modules/moderation.md:27` 寫「內容已刪除時隱藏回 FailedPrecondition」，程式與測試是「不呼叫、回成功」（上表）。本步順手把兩份文件改成跟程式一致（只改文件，以程式為準：A7 的測試名稱已經是現行規格）。
4. 計畫的鎖序寫「buyer → seller → review/im → moderation」。結論成立，但隱藏與還原根本不碰 buyer、seller（§5）；只有 §W2 選 (a) 時才會出現 seller。

## 1. 各模組新增與修改的 API（完整清單）

### 1.1 review（`M/review`）

| 方法 | 狀態 | 說明 |
|---|---|---|
| `Hide(ctx, HideInput) (Hide, error)` | 已有，不改 | 冪等，新隱藏時發 `ReviewAggregateChanged` |
| `Restore(ctx, RestoreInput) (Hide, error)` | 已有，不改 | 沒有未還原的隱藏回 FailedPrecondition |
| `ReviewForReport(ctx, id uuid.UUID, viewerBuyerID int64) (ReportedReview, error)` | **新增** | 唯讀不鎖。可讀規則同 `GetReview`（公開可讀述詞＋觀看者封鎖，`review.go:181`），不可讀與不存在都是 NotFound。回 `Review` 的作者、本文、評分、圖片 asset ID、`CreatedAt`、`StoreID`、`ProductID`、店家回覆（本文與時間） |

`ReportedReview` 放 `internal/model`、根套件別名。不另開 `GetReview` 給 moderation：它回的是顯示用投影（`ItemImageKey` 等），快照要的是證據欄位，兩者會各自演進。

### 1.2 im（`M/im`）

| 方法 | 狀態 | 說明 |
|---|---|---|
| `Hide`、`Restore` | 已有，不改 | 同 tx `NOTIFY` 讓串流重讀 |
| `MessageForReport(ctx, messageID uuid.UUID, reporter Party, window int) (ReportedMessage, error)` | **新增** | 唯讀不鎖，加入呼叫方的 transaction（`freeze` 在 `SubmitReport` 的 tx 內呼叫，`report.go:135`；被別的模組呼叫的方法不開 `InReadSnapshot`，規則 3，先例 `seller/governance.go:600`、`crm/internal/read/read.go:210`）；被檢舉訊息與前後文以一句 SQL 讀回，保證同一時點。授權：`reporter.Authorize()` 且是該對話的一方，否則 NotFound（不洩漏對話存在）。被檢舉的訊息已隱藏或已抹除回 NotFound。回被檢舉訊息與 `seq` 前後各 `window` 則（`window` 由 moderation 傳 `model.ChatContextWindow = 5`），每則帶 seq、寄件方種類與 ID（由對話的 `buyer_id`／`store_id` 推出）、型別、本文、圖片 asset ID、時間、`Hidden`／`Erased` |

前後文裡已隱藏或已抹除的訊息：照讀取路徑回墓碑（不帶內容，只帶旗標），**不把隱藏內容寫進證據快照**。理由：證據的範圍是「檢舉人當下看得到的對話」，A6-D3 的可見範圍也是以此為界；隱藏的那則有它自己的案件與快照。

### 1.3 moderation（`M/moderation`）

| 項目 | 改動 |
|---|---|
| `model.TargetKind` | 加 `TargetReview = "REVIEW"`、`TargetChatMessage = "CHAT_MESSAGE"`；`TargetKinds()` 同步 |
| `model.Action.AllowedOn` | HIDE：COMMENT、FORUM_ENTRY、REVIEW、CHAT_MESSAGE；WARN、BAN：再加 REVIEW、CHAT_MESSAGE；TAKEDOWN：§W2 決定（拍板前不加） |
| `model.ChatContextWindow = 5` | 新常數（A6-D3） |
| `Deps` | 加 `Review *review.Module`、`IM *im.Module`（`New` 的必填檢查一起加） |
| `report.Deps`、`handle.Deps` | 各加以介面表示的 review、im 依賴（只列用到的方法，測試可替身） |
| `report.freeze` | 依種類分流：`freezeReview`、`freezeChatMessage`；自己的內容檢查沿用 `sameAccount` |
| `handle.author` | 依種類取作者：REVIEW 永遠是買家，**一律用快照凍結的作者**（評價作者不會變，與聊天一致，不回頭問 review）；CHAT_MESSAGE 取快照凍結的寄件方（訊息內容會被抹除，作者不會變，**一律用快照**，不回頭問 im） |
| `handle.apply` 的 HIDE | REVIEW → `review.Hide`；CHAT_MESSAGE → `im.Hide`；回 NotFound（評價已被刪帳清除）時照 social 前例視為成功、不寫隱藏紀錄 |
| 還原入口 | 依 §W1 的拍板；預設形狀見 §W1 (a) |
| `snapshot.Content` | 擴充欄位（§3） |
| `cmd/hawkr/moderation.go` | `newModeration` 加 review、im 參數 |

### 1.4 seller

§W2 選 (b) 時不改。選 (a) 時的清單寫在 §W2。

### 1.5 不新增的東西

- 不新增 review、im 的守門呼叫（§5）。
- 不在 moderation 存隱藏理由：理由在 review、im 的 `hides`（`source_ref = moderation:case:<id>`，`handle.SourceRefPrefix`）。

## 2. 處置與還原的語意總表

| 對象 | 作者 | HIDE | WARN | BAN | 店家作者 |
|---|---|---|---|---|---|
| REVIEW | 買家 | `review.Hide`；評價已清除→成功、不寫 | `social.Warn`（買家處分的唯一擁有者，`handle.go:208-213`） | `accountsuspension.Suspend` | 不會發生（評價只有買家寫） |
| CHAT_MESSAGE | 買家或店家（§0.4.1 第 2 點：買家單方檢舉下只會是店家） | 買家作者：`im.Hide` | 買家作者：`social.Warn` | 買家作者：同上 | §W2 |

還原：只對「這個案件的處置是 HIDE」的案件；呼叫對象模組的 `Restore`，帶來源參照 `moderation:case:<案件 id>`，只還原來源相同的未還原隱藏（同一則內容的隱藏是冪等的，後到案件沿用先到案件那一筆，撤銷後到的案件不能還原別案的隱藏，回 FailedPrecondition）；內容已被清除或刪除（擁有者回 NotFound）時同 Hide 視為成功，撤銷仍記在案件上。還原和寫 moderation 的還原紀錄在同一個 transaction，鎖序 review／im → moderation。

## 3. 快照形狀

下列欄位以扁平形狀列出；最終是扁平還是依種類分開的 struct 隨 §W4 拍板（§9 第 1 列）：

```go
// 評價
Rating    int    `json:"rating,omitempty"`
ProductID int64  `json:"product_id,omitempty"`
SubjectStoreID int64 `json:"subject_store_id,omitempty"` // 評價所屬店家（上下文，不是作者）
ReplyBody string `json:"reply_body,omitempty"`   // 店家回覆，當作上下文；不是被檢舉的內容
// 聊天
ConversationID string           `json:"conversation_id,omitempty"`
Seq            int64            `json:"seq,omitempty"`
Context        []ContextMessage `json:"context,omitempty"`
```

```go
type ContextMessage struct {
    Seq        int64     `json:"seq"`
    SenderKind string    `json:"sender_kind"`
    SenderID   int64     `json:"sender_id"`
    Type       string    `json:"type"`
    Body       string    `json:"body,omitempty"`
    MediaRefs  []string  `json:"media_refs,omitempty"`
    SentAt     time.Time `json:"sent_at"`
    Hidden     bool      `json:"hidden,omitempty"`
    Erased     bool      `json:"erased,omitempty"`
}
```

### 3.1 REVIEW

`Kind=REVIEW`、`AuthorKind=BUYER`、`AuthorID=buyer_id`、`StoreID=0`（**刻意不放評價所屬的店家**：`StoreOf` 與 TAKEDOWN 的歸屬判斷用 `StoreID`，評價的店家不是作者，放進去會誤導；評價所屬店家另存上下文欄位 `SubjectStoreID`，不放 `StoreID`；評價被抹除後仍查得到）、`Body`、`MediaRefs`（評價圖片的 asset ID）、`AuthoredAt`、`Rating`、`ReplyBody`。

### 3.2 CHAT_MESSAGE（A6-D3）

被檢舉的那則放在既有欄位（`AuthorKind`、`AuthorID`、`Body`、`MediaRefs`、`AuthoredAt`），`ConversationID`、`Seq`；`Context` 是前後各 5 則（不足就少；不含被檢舉的那則），依 `seq` 由小到大。`StoreID`：寄件方是店家時等於 `AuthorID`（讓 `StoreOf` 與 §W2 (a) 的歸屬檢查直接適用），買家寄件時為 0。

- 正規化：`Normalize` 對 `Context` 每則套同一套本文與媒體規則；`Context` 依 `seq` 排序。
- 證據保全：`MediaRefs` 只放被檢舉那則的圖片；**前後文的圖片也一併 hold**（`AssetIds` 取兩者聯集）。理由：A7-D21 的目的是能重現檢舉當下的那一版，前後文是證據的一部分。保留期同 184 天。
- 雜湊：每案每份內容一列的規則不變。同一則訊息後來有新訊息不影響前文，但「後 5 則」會變，所以同一則訊息的第二位檢舉人可能產生第二份快照——這是正確的（兩人看到的上下文不同）。

## 4. migration

| 模組 | 檔 | 內容 |
|---|---|---|
| moderation | `db/migrations/<ts>_cases_review_chat_targets.sql` | `ALTER TABLE moderation.cases DROP CONSTRAINT cases_target_kind_check, ADD CONSTRAINT … IN ('POST','COMMENT','FORUM_ENTRY','BUYER','REVIEW','CHAT_MESSAGE')`；`cases_action_target_check` 同步 §1.3 的 `AllowedOn`；`schema.sql` 同步；`atlas.sum` |

`TestChecksMatchGo`（模組既有）守 Go 與 CHECK 一致。review、im 不需要 migration（隱藏表已在）。§W1 選 (a) 時 moderation 另加還原紀錄欄位（見 §W1）。

## 5. 鎖序分析

### 5.1 隱藏與還原要不要取擁有者守門：不要【推，依據下列程式】

- 守門的目的是擋「停用身分產生新資料」（A6-D19；`review/internal/purge/purge.go:71`、`im/internal/purge/purge.go:59` 的註解）。隱藏與還原不產生作者的新資料，是平台對既有內容的處分。
- 作者已關閉或已退場時：
  - review：申請刪除起公開讀取就看不到（`pub_inactive_buyers` 述詞），評價在抹除 job 裡刪除（`review/db/queries/purge.sql:14`）。刪除前隱藏 → 寫一筆隱藏，之後隨評價 CASCADE 消失【量：`review/db/schema.sql:67`、`im/db/schema.sql:65` 皆 `ON DELETE CASCADE`】；刪除後 → `LockReview` NotFound，moderation 視為成功（§1.3）。
  - im：抹除只標 `erased_at`，列還在（`messages.sql:61`），隱藏照寫、無害；讀取早已被 `pub_closed_buyers`／`pub_retired_stores` 擋住。
  - 如果取守門，`LockNotClosed`／`LockNotRetired` 對已停用的作者回 NotFound，反而讓處分失敗，管理員無法處理停用者的檢舉。
- 因此：**不加守門**。也沒有 buyer、seller 的列在這條路徑上被鎖。

### 5.2 實際的鎖序

| transaction | 依序鎖的模組 | 是否符合 `Modules` |
|---|---|---|
| `Decide`（HIDE REVIEW） | review（`LockReview`）→ moderation（`CloseCase`）→ adminaudit | 12 < 24 ✓ |
| `Decide`（HIDE CHAT_MESSAGE） | im（`LockMessage`）→ moderation | 13 < 24 ✓ |
| `Decide`（WARN／BAN） | 不變：social 或 identity → accountsuspension → moderation | 既有 |
| 還原 | review 或 im → moderation | ✓ |
| `SubmitReport` | `freeze` 不鎖；之後只鎖 moderation | 既有 |
| §W2 (a) 的店家下架 | seller（建立措施）→（im 訂閱者，如有）→ moderation | 4 < 13 < 24 ✓ |

### 5.3 與其他寫入路徑會不會死鎖【推】

- **im `Send`**：buyer `FOR KEY SHARE` → seller `FOR KEY SHARE` → im 新增訊息、更新對話列（`send.go:69-73`）。`Hide` 只鎖既有的訊息列（`FOR NO KEY UPDATE OF m`），新增訊息對 `conversations` 的 FK `KEY SHARE` 與它不衝突；兩者沒有共同的列可互等。
- **抹除 job**：review 刪評價（先 `FOR NO KEY UPDATE`，`purge.sql:10`）、im 標 `erased_at`，都只鎖自己模組的列，不碰 moderation；和 `Hide` 搶同一列只會排隊，不成環。
- **刪帳事件 transaction**（buyer → seller → 訂閱者）：review、im 的訂閱者只排 job（`im.go:255-266`；`review.go:214-223`），不鎖內容列；moderation 的 retention 訂閱者鎖 moderation 自己的列。這條 tx 從不等 review／im 的內容列，所以不和 `Decide` 成環。
- **`NOTIFY`**：Postgres 在 commit 時取全域佇列鎖，只序列化 commit，不參與列鎖等待。
- **不在 lockorder 檢查範圍**的：`im.hides` 對 `im.messages` 的 FK 隱含鎖（同模組，`lockorder.go` 套件註解已列）。

結論：計畫的鎖序成立；`Modules` 已把 review、im 排在 moderation 前，**不需要改登記**，只需要依賴表加 review、im（`TestLockOrderIsBottomUpOverTheDependencyTable` 會驗證）。證據等級【推＋量】：順序表與各查詢的鎖強度是【量】，「不成環」是讀程式的【推】，由 §7 的 integration test 在 lockorder 檢查器下跑過才算證明。

## 6. `internal/arch` 與 lockorder 登記

- `internal/arch/modules.go:37`：`"moderation": {"social", "seller", "buyer", "accountsuspension", "media", "review", "im"}`，註解補「評價、聊天訊息的隱藏與還原經 review、im」。review、im 本來就不依賴 moderation，不產生環。
- `InternalDependencies["moderation"]` 不變（新程式都在既有的 `report`、`handle`、`snapshot`、`model`）。
- `P/lockorder.Modules` 不變（§5）。

## 7. 檔數估計【推】

| 區 | 檔 |
|---|---|
| proto＋生成物 | `api/.../moderation.proto`、Go connect 生成、TS `@hawkr/api`、Dart `hawkr_api`（約 4–6） |
| 共用文案 | catalog 兩個 JSON＋生成的 TS／Dart／Go（約 6） |
| moderation | `model/model.go`、`snapshot/snapshot.go`、`report/report.go`、`handle/handle.go`、`moderation.go`、migration＋`schema.sql`＋`atlas.sum`、文件 3（`AGENTS.md`、`handle/AGENTS.md`、`CARRYOVER.md`）、`docs/modules/moderation.md`（約 12） |
| review | `review.go`、`model/model.go`、讀取實作 1、查詢 0–1（約 3–4） |
| im | `im.go`、`model/model.go`、`read/read.go`、`messages.sql`＋生成（約 5） |
| arch、cmd | `modules.go`、`cmd/hawkr/moderation.go`、`httpapi/moderation/admin_integration_test.go` 的組裝（3） |
| 測試 | moderation 新整合測試 1–2 檔、snapshot unit、review／im 的快照 API 測試（約 5） |

合計約 40 檔（生成物約一半），1 session 實作＋1 session 審查與修正；§W2 選 (a) 另加約 1 session。

## 8. 測試與 mutation

### 8.1 integration（CI 跑，本機不起容器）

1. **檢舉→隱藏→還原（評價）**：買家 A 檢舉買家 B 的評價 → 案件、快照（`Rating`、`MediaRefs` 對）→ HIDE → 公開列表、`GetReview`、`ProductStats` 看不到，`review.hides` 一筆、`source_ref` 是案件 → 還原 → 再看得到、`restored_at` 有值、統計回復、各發一次 `ReviewAggregateChanged`。
2. **檢舉→隱藏→還原（聊天）**：依 §W2／§W3 的結果造一則可隱藏的訊息 → 快照的 `Context` 恰好前 5、後 5（造 12 則以上），被隱藏的前文是墓碑 → HIDE → `ListMessages`、`MessagesAfter` 回墓碑；`ListConversations` 的預覽 `Hidden=true` 且不帶文字（ID1）→ 還原 → 預覽回到內容；`NOTIFY` 各一次。
3. 非對話方檢舉訊息 → NotFound；檢舉已隱藏的訊息 → NotFound；檢舉自己的評價或訊息 → InvalidInput。
4. 評價已被刪帳清除後 HIDE → 成功、沒有隱藏紀錄（對齊 `TestHideOfDeletedBuyerContentSucceedsWithoutCallingSocial`）。
5. 作者已關閉（買家 CLOSED、店家退場）時 HIDE 仍成功（證明不取守門）。
6. **鎖序**：以上每支都在 `testdb` 的 lockorder 檢查器下跑；另加並行測試：同一則訊息上 `Decide`（HIDE）與 im 抹除 job、與 `Send`（同對話）並行，各跑 N 次，無死鎖、無違規。
7. 快照 unit：`Context` 排序、正規化、雜湊穩定（同內容同雜湊）。
8. `TestChecksMatchGo` 涵蓋新的 CHECK。

### 8.2 mutation（在複本跑，second-brain `scripts/red-proof.py`，不得在工作目錄編輯後還原）

| # | mutant | 要失敗的測試 |
|---|---|---|
| 1 | review `RestoreHide` 不寫 `restored_at`（還原時不清隱藏紀錄） | 8.1-1 |
| 2 | im `RestoreHide` 同上 | 8.1-2 |
| 3 | im 預覽推導拿掉 `hides` 述詞（`conversations.sql:73,85`；ID1 的改寫，§0.4.1 第 1 點） | 8.1-2 的預覽斷言 |
| 4 | moderation HIDE 對 CHAT_MESSAGE 呼叫 `review.Hide`（分流錯） | 8.1-2 |
| 5 | `MessageForReport` 拿掉對話方授權 | 8.1-3 |
| 6 | `Context` 視窗改成 4 | 8.1-2 |
| 7 | HIDE 前加 `LockNotClosed`（加守門） | 8.1-5 |
| 8 | 把 `CloseCase` 移到 `im.Hide` 之前（moderation 先鎖） | lockorder 檢查器 |

## 9. 沿用盤點（寫進 `M/moderation/CARRYOVER.md`）

| 沿用的機制 | 從零設計會怎麼做 | 決定 |
|---|---|---|
| 扁平的 `snapshot.Content` 加種類專屬欄位 | 依種類的 tagged union（每種一個 struct） | 「既有雜湊不變」不是約束（未上線，根目錄 `AGENTS.md`）。從零設計是依種類的 struct；與 §W4 一併拍板：W4 選 (a) 時本步改成依種類分開的 struct，選延後時沿用扁平並以 W4 的落點為觸發 |
| 隱藏表在內容模組、moderation 只存案件 | 同（處分紀錄與效果同模組，10e R12） | 沿用 |
| HIDE 遇到已清除內容視為成功 | 同（隱藏的目的已達成；不讓案件卡住） | 沿用，並把兩份過時文件改正（§0.4.1 第 3 點） |
| 證據保全以 media 引用（`GOVERNANCE_EVIDENCE`） | 同 | 沿用，範圍擴到聊天前後文的圖片（§3.2） |
| 檢舉人只有買家 | 平台雙邊都能檢舉（聊天是雙方私訊） | 待決 §W3 |
| WARN 寫在 social 的買家警告 | 買家處分紀錄放 buyer 或獨立 enforcement 模組 | 沿用：A7 已把買家警告定在 social，換位置是另一個模組的重構；觸發：第三種非社群對象需要警告時 |
| 前後文窗格固定 5 則 | 依時間窗或依則數 | 沿用 A6-D3 |

review、im 的 `CARRYOVER.md`：各加一列「`ReviewForReport`／`MessageForReport` 是證據讀取，不給顯示用；可讀規則與顯示讀取共用同一個述詞」。

## §W 待拍板（已決：A6-D21，Will 2026-11-21，W1–W4 皆選 (a)；4b 依此改寫 §3 為依種類分開的 struct）

### W1 還原的入口與案件紀錄

- 事實：`Decide` 是終態、不重開、不改判（`handle/AGENTS.md:6`）；後台沒有任何還原入口，A7 刻意不做 `RestorePost`（`2026-10-18-a7-social.md:442`）；`social.Restore` 沒有正式呼叫端。計畫驗收卻要「檢舉→隱藏→還原」。
- 選項：
  - (a) moderation 新增 `ReverseHide(caseID, adminID, reason)`：案件維持 CLOSED、決定不改，案件加還原欄位（`reversed_at`、`reversed_by_admin_id`、`reverse_reason`，CHECK 只允許 HIDE 案件）＋`adminaudit`；同 tx 呼叫 review／im／social 的 `Restore`。社群的留言、討論區隱藏一併受惠。
  - (b) 只在 review、im 開後台還原 RPC（不經 moderation）；案件不知道自己的處置被撤。
  - (c) 本步不做還原入口，只以模組 API 測試還原；入口延到 A6-5 定 Connect 時。
- 業界：處分可撤銷且撤銷要留在原決定旁（DSA 第 20 條申訴後撤銷；YouTube、Meta 都在原 enforcement 上記 reversal）。
- 建議 (a)：決定紀錄與撤銷在同一處、可稽核；改動是 1 個 migration＋1 個方法。
- 擋住：§1.3 的還原入口、§8.1-1／-2 的還原步驟的寫法、proto 是否加 RPC。

### W2 店家寫的聊天訊息（與評價回覆）怎麼處分

- 事實：§0.4.1 第 2 點：買家單方檢舉下，聊天案件的作者全是店家；A7-D5′ 要求店家處分走 seller 措施；現行內容下架只支援 POST、COMMENT、FORUM_ENTRY（`P/contentkind`、seller 的 `measure_contents` CHECK、`seller.NewContentSnapshot` 的連結欄位只有貼文或討論串）。
- 選項：
  - (a) seller 內容下架加 `CHAT_MESSAGE`（評價回覆另加 `REVIEW_REPLY` 或不做）：`contentkind` 加值、`measure_contents` CHECK migration、`ValidateContentSnapshot` 的連結規則、social 訂閱者跳過非社群種類；im 的讀取述詞 join `seller.pub_content_takedowns()`（規則 2 允許，不建複本），並訂 `MeasureCreated`／`MeasureRevoked` 只發 `NOTIFY`。可申覆、可撤銷。約 +1 session。
  - (b) 本步不處分店家寫的訊息：店家作者只能 NO_VIOLATION（現行行為），另開 seller 的一般通報給營運處理。
  - (c) 對店家訊息直接 `im.Hide`（違反 A7-D5′：店家處分沒有申覆）。
- 業界：市集平台把賣家的違規訊息算進賣家帳號健康與申覆流程（Amazon Account Health、蝦皮罰分），不做沒有申覆的刪除。
- 建議 (a)，評價回覆同批加 `REVIEW_REPLY`（否則店家在回覆裡騷擾買家沒有入口）；若要壓 A6 範圍，退而選 (b) 並在 W3 開放店家檢舉，讓聊天對象至少能處分買家。
- 擋住：`AllowedOn` 的 TAKEDOWN 列、§8.1-2 的造資料方式、seller 是否在本步改動、檔數。

### W3 店家能不能檢舉聊天訊息

- 事實：檢舉人只有買家（`reporter_buyer_id`）；舊 seller-app 有檢舉入口（`client-inventory.md:45` 的 `report/data/{block,report}_repository.dart`）；不開放的話，買家寫給店家的騷擾訊息沒有任何檢舉路徑。
- 選項：(a) 檢舉人改為 (種類, ID)：`reports` 加 `reporter_store_id` 與「恰好一欄」CHECK，去重鍵與每日額度依種類各一份，只有 CHAT_MESSAGE 接受店家檢舉人；(b) 店家檢舉走 seller 既有的通報與分案（seller governance），moderation 不變；(c) 不開放。
- 業界：雙邊私訊平台雙方都能檢舉對方訊息（Airbnb、蝦皮聊天）。
- 建議 (a)：聊天案件與證據快照只有一套；(b) 會讓同一則訊息的兩方檢舉落在兩個系統。
- 擋住：`reports` 的資料模型與 migration、`SubmitReport` 的輸入形狀、App 的檢舉 proto（A6-5）。

### W4 後台看不看得到快照（A6-D3 的可見範圍怎麼落地）

- 事實：`GetCase` 不帶快照（`admin/moderation/v1alpha1/moderation.proto:125-128`）；A7 把「GetCase 是否帶快照」留成待決（`2026-10-18-a7-social.md:662`）。A6-D3 說「後台只看前後各 5 則的固定快照」，但今天後台什麼都看不到，管理員只能靠回頭讀對話——對私訊而言正是 A6-D3 要禁止的。
- 選項：(a) 本步在 `GetCase` 加 `snapshot`（依種類的 oneof，社群、買家、評價、聊天四種），admin 頁在 A6-6 顯示；(b) 延到 A6-5／A6-6。
- 業界：審核台一律在案件上顯示被檢舉內容的凍結版本。
- 建議 (a) 只做 proto 與 handler（後台 UI 在 A6-6）：本步已在定快照形狀，同時定對外形狀可避免第二次搬欄位；也讓 §9 第 1 列的觸發在本步就能重評。
- 擋住：proto 改動範圍、§9 快照形狀是否改成 union。

## 10. 未查證與限制

- 「不成環」是讀程式的推論，要 §8.1-6 在 CI 跑過。
- 本機不跑 integration 與 mutation 的容器部分，交 CI。
