# moderation 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

來源：social-service 的檢舉部分（`backend/services/social-service` 的 `reports`、`moderation_actions`、`usecase/report.go`、`usecase/dispatcher.go`，簡稱 `SI`），盤點 `backend/docs/plans/2026-10-18-a7-social/domain-inventory.md`（M12、M17、M24），決定 A7-D7、A7-D13、A7-D14、A7-D20（計畫 `2026-10-18-a7-social.md`）。對外 API 與特性測試在 A7-2。

## 沿用盤點

social-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 主鍵 BIGINT tsid（`reports`、`moderation_actions`，M1） | Java 消費者要以數字 ID 讀 | 不成立：沒有別的模組以 bigint 指稱檢舉或案件；seller 的認定以不透明參照引用檢舉 | 應用層 UUIDv7（ADR 2026-09-25-row-identity-by-data-kind） | 改：案件、檢舉、快照都是 UUIDv7；檢舉人與買家對象用 buyer 的 BIGINT 業務鍵 |
| 一筆檢舉一列、各自帶 `status` 與 `resolved_action`，處置逐筆（`SI/schema.sql:85-97`） | 逐筆處置 | 不成立：同一對象的 30 筆檢舉要處置 30 次，結果不一致；seller 的通報可在分案前逐筆判掉，社群檢舉一律掛案 | 檢舉掛案，一案一個決定（A7-D14） | 改：案件鍵 `(target_kind, target_key)`，partial UNIQUE `WHERE closed_at IS NULL`；決定只存在案件（`decision`、`decision_category`、`decision_action`、`decided_by_admin_id`、`closed_at`，CHECK 同為 NULL 或同不為 NULL）；檢舉不存結果，由案件推導；結案是終態，不重開、不改判 |
| `moderation_actions` 審核動作 audit log，寫失敗被丟棄、沒人讀（M12、SD12，`SI/schema.sql:118-133`） | 稽核每次處置 | 不成立：處分的紀錄歸擁有效果的模組（10e R12，A7-D9） | 處置與紀錄同一個 transaction、可讀取 | 刪：不建處置紀錄表；案件的 `decision` 是決定紀錄，處分本身的紀錄在 social（隱藏、警告）、identity（停權）、seller（措施）；處置與結案同一個 transaction，寫失敗整個回滾（SD12） |
| `AUTO_REJECT` 與 `REOPEN` 動作（系統自動駁回、重開） | 系統建立者與重開 | 不成立：A7-D7 第 3 項不新增系統建立者；結案是終態 | 重開用新案件 | 刪：不沿用 `AUTO_REJECT`；沒有重開，結案後的新檢舉開新案件 |
| 處置 dispatcher 依 `target_type` 分流，作者解析沒有 kind，先當店家（M12、SD10，`SI/usecase/dispatcher.go:42-89`） | 對象種類決定處置 | 不成立（缺陷）：撞號時猜錯人 | 對象帶 (kind, id)，作者由擁有者回報 | 改：檢舉帶 `(target_kind, id)`；作者向 social 讀（`ContentOwner`），買家作者轉帳號只在 `BAN` 呼叫 `accountsuspension` 之前由 buyer 的 `AccountIDByBuyerID` 做一次；店家不能被停權，店家的內容走 seller 的內容下架措施，且內容必須經 social 驗證屬於指定的店家 |
| `WARN_USER` 只寫稽核，對使用者沒有任何效果（SD11，`SI/usecase/report.go:235-237`） | 警告要有效果 | 不成立（缺陷） | 警告是使用者讀得到的處分紀錄 | 改：`WARN` 呼叫 social `Warn`，作者在「我的處分」讀得到型別與理由 |
| 檢舉原因 8 種 `SPAM`、`SEXUAL`、`FRAUD`、`HARASSMENT`、`FOOD_SAFETY`、`POLITICAL`、`FAKE_NEWS`、`OTHER`（CodeMeta `ReportReasonCode`，`SI/schema.sql:95`） | CodeMeta 的舊契約 | 凍結，不新增值域；政策分類是平台共用的一份（Q5） | 平台政策分類加各入口的子集 | 改：用 `PolicyCategory` 的社群檢舉子集 `CommunityReporterSelectable()`（恰好八個）。對映：`SPAM`→`SPAM`、`HARASSMENT`→`HARASSMENT`、`FRAUD`→`FRAUD`、`OTHER`→`OTHER`、`SEXUAL`→`SEXUAL_CONTENT`（新增）、`POLITICAL`→`OTHER`、`FAKE_NEWS`→`MISLEADING_INFO`、`FOOD_SAFETY`→`FOOD_SAFETY`；`HATE` 新開放（業界檢舉選單都有仇恨言論）；`POLITICAL` 不是平台政策類別，歸 `OTHER` |
| 每人每天 20 筆、24 小時去重，先查再寫（M24、SD23，`SI/usecase/report.go:35,100-112`） | 防濫用 | 成立（濫用防護仍需要），但先 count 再比較有並行空窗，24 小時去重讓結案後與跨日行為不一致 | DB 唯一約束加原子計數；速率限制交給入口 | 改：每日上限 20 保留（`reporter_daily_counts` 單列 upsert，台北日換日重算，未達上限才加一，原子）；去重是 `UNIQUE (case_id, reporter_buyer_id)`，進行中案件重送回傳既有檢舉且不吃額度，結案後可再檢舉；速率限制在 `httpapi` 入口，不在本模組 |
| 檢舉人 `reporter_id` 依 active role 存 buyer_id 或 store_id，查不到現任時退回 account_id（`SI/schema.sql:106`） | 店家與買家都能檢舉 | 不成立：只有買家檢舉社群內容；退回 account_id 讓同一欄混了三種身分空間 | 單一鍵 | 刪：不沿用 reporter fallback；`reporter_buyer_id` 單一鍵，買家對象存 `buyer_id` |
| 提交不驗證對象存在、不禁止檢舉自己；`target_snapshot` 欄位從未寫入（SD21、SD23，`SI/schema.sql:86`） | 無（缺陷） | 不成立 | 提交前驗證，凍結當時內容 | 改：對象必須存在而且公開可讀（被隱藏的回 NotFound）；不能檢舉自己（含同一帳號的其他身分與店家所屬帳號）；送出時凍結快照（A7-D20） |
| 快照內容的保留（舊：無期限，欄位從未寫入） | 申訴與稽核要看當時的內容 | 申訴期至少六個月（DSA 第 20 條）；seller 措施申覆期限 184 天 | 保留期後刪除個資 | 新：結案後 184 天刪快照（與 seller `StoreRetention` 同值，各自一個常數），案件與檢舉列留著；進行中案件的快照不刪；`snapshot_id ON DELETE SET NULL` |
| 髒字詞典（M17） | 預先過濾 | 屬於 social 的寫入路徑（A7-1a 已放在 `platform/textfilter`） | 詞庫有擁有者與後台維護 | 不在本模組：詞庫的擁有者與後台維護是另一個決定，觸發是營運要求新增詞 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。A7-1d 沒有對外入口（舊 social-service 照常服務），這一節只記已知的對應，Connect 的宣告表與 handler 在 A7-2。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 檢舉人身分由 gateway 解析 principal，失敗退回原值（M18） | 跨服務時 gateway 才有身分 | 不成立：同一個程序，入口解析 session 後直接帶 buyer_id | 入口認證、模組收已解析的業務鍵 | 改：`SubmitReport` 收 `ReporterBuyerID`，由 A7-2 的 handler 從 session 解析 |
| 錯誤用 CodeMeta 錯誤碼與字串訊息 | 舊契約 | 不成立：新 API 用 `apperr` 的 kind 與 `ErrorReason` | 結構化錯誤 | 改：`apperr`（InvalidInput、NotFound、FailedPrecondition、ResourceExhausted、VersionConflict）；伺服器句子目前是字面字串，A7-2 開放對外 API 時移進模組 `internal/text` |
| 列表分頁 page／pageSize 加 `COUNT(*)`（M20） | client 顯示總頁數 | 不成立 | AIP-158 keyset | 後台待辦與案件列表在 A7-1e、A7-2 用 keyset；本步只有單一案件的讀取 |
| 每日 20 筆在入口限流（M24） | 防濫用 | 上限是業務規則，在模組內強制；入口另有一般限流 | 業務上限在資料層原子強制 | 改：上限在模組內單一 upsert 強制，入口限流不負責 |

## 與 social-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。

- 檢舉對象：貼文、留言、討論區留言與買家（舊：`REVIEW`、`POST`、`POST_COMMENT`、`SELLER_FORUM`、`CHAT_MESSAGE`、`USER`）；評價與聊天對象在 A6-4。
- 檢舉不存 `status`、`resolved_action`、`resolved_by`、`resolved_at`：結果由案件推導。
- 沒有 `evidence` 欄：證據是送出時凍結的快照。
- 檢舉說明 `description` 選填，最多 500 字，頭尾沒有空白。
- 處置種類：`HIDE`、`WARN`、`BAN`、`TAKEDOWN`（舊：`HIDE_TARGET`、`WARN_USER`、`BAN_USER`、`DISMISS`）；`DISMISS` 對映案件決定 `NO_VIOLATION`；新增 `TAKEDOWN`（店家的內容走 seller 措施）。
- 每次處理只有一種處置；同一案件組合多種處置是終態（見規格 §6）。
- 內容下架的認定引用案件最早的 20 筆檢舉（seller 的上限 `MaxSourceModerationReports`）。
