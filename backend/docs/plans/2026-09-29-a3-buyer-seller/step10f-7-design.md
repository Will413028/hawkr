# 10f-7 設計：設計審查的修正

盤點快照 dev `603cac14`（2026-11-22）。累積清單在計畫 `../2026-09-29-a3-buyer-seller.md` 的 10f-7 段；本檔寫每一項的做法與決定。簡報經一位獨立審查者檢查：findings 10（blocking 0）：fixed 9、raised 1（D3）、rejected 0。

## 決定（Will 2026-11-22）

| 編號 | 題目 | 決定 | 捨棄的選項 |
|---|---|---|---|
| D1 | 檢舉可選類別的 proto custom option 掛在哪 | 掛在收值的請求欄位：店家檢舉 `CreateReportRequest.category` 與社群檢舉的請求欄位各列出可收的值，`uitext-gen` 各生成一份三端常數。自訂 option，不引入 protovalidate。認定用的子集只有 admin 一個 client，留在 admin 常數（C3 的觸發不成立） | 掛在 enum 值（共用的 enum 要認識各入口）；只做店家一組（buyer-app 手寫的社群清單留著） |
| D2 | admin 的 CSV 預覽 | 另開 `Preview` RPC，與匯入共用同一個解析函式，回應專為預覽設計；admin 刪掉自己的解析。版面與欄位不動，預覽階段顯示伺服器的錯誤訊息 | 匯入 RPC 加 `validate_only`（預覽資料要塞進匯入的回應，且該 RPC 宣告必寫稽核）；保留 client 解析 |
| D3 | 匯入後怎麼在同一個 transaction 排入推送 | platform 的匯入在 transaction 內發布「行事曆已匯入」事件，`internal/legacy` 的 relay 訂閱後排入 job。A5 刪除 relay 時一併刪 platform 的事件型別與 `worker` 的 invoice client 接線 | transaction 改由 handler 開（改掉匯入與稽核紀錄的 transaction 邊界） |

D1 的依據：ADR `docs/adr/2026-10-18-seller-governance-consolidated.md` 第 56 行與 `step10e-3ca-design.md` C3 已決定用 proto custom option 並由 `uitext-gen` 生成；形狀是這次定的。`docs/adr/2026-09-25-codemeta-retirement.md` 否決的是把顯示文字放進 option，與值的子集無關。

## 第 1 刀：檢舉（A#1、A#2、A#3、A#8）

### A#1 可選類別

現況：店家檢舉可選 12 個（`policy.ReporterSelectable()`、seller `reports_category_check`、admin `REPORTER_CATEGORIES`），website `NOT_SELECTABLE` 的補集與 buyer-app `storeReportCategories` 是 16 個（多了 SPAM、HARASSMENT、HATE、SEXUAL_CONTENT）；社群檢舉可選 8 個（`policy.CommunityReporterSelectable()`、moderation `reports_category_check`、buyer-app `communityReportCategories`）。

做法：

- `api/` 新增一個欄位層級的 custom option（repo 的第一個 extension），值是 `PolicyCategory` 的清單；掛在 seller `CreateReportRequest.category` 與 moderation 的檢舉請求欄位。兩個欄位的註解改成指向 option，不再用文字列舉。
- `uitext-gen` 讀 buf image 裡這個 option，生成 Go、TS（`@hawkr/ui-text`）、Dart（`hawkr_ui_text`）各兩份常數；生成物納入 `ui-text-generated-diff` 的比對路徑。**開工第一步先驗證 `uitext-gen` 讀得到 extension，讀不到就停下回報。**
- Go 的 `ReporterSelectable()`、`CommunityReporterSelectable()` 改回傳生成的清單；兩個 CHECK 仍由 `TestChecksMatchGo` 對齊。
- 刪除手寫清單：admin `REPORTER_CATEGORIES`、website `NOT_SELECTABLE`、buyer-app `storeReportCategories` 與 `communityReportCategories`，改用生成的常數。`UNSPECIFIED` 不在清單裡。
- 測試改成鎖「社群類別不在店家檢舉的清單」。
- 一併更正過期的註解：seller `db/schema.sql` 的 `reports_category_check` 上方、`policy_category.proto` 各值的「入口：檢舉人」。

### A#2 補充的可用性

- `GetMyReportResponse`、`AddReportSupplementResponse` 加 `can_supplement` 與 `supplement_max`（先例：`can_reply` 與 `reply_attachment_max`）。
- seller 模組加一處來源 `CanSupplement`，`AddSupplement` 的兩個擋下條件（已有結果、已達上限）改用它，並納入 `TestCanFlagsMatchWhetherTheActionSucceeds`（seller `AGENTS.md`「動作可用性各只有一處來源」）。
- website、buyer-app 刪掉自己的推導與上限常數；「補充說明（n/10）」的上限取自 `supplement_max`。

### A#3 mutation 清單

- `mutation/a3-10f-1.json`：拿掉詢問的擁有者檢查（`ownsInquiry`）；拿掉「已回覆不可再回覆」。
- `mutation/a3-10f-2a.json`：拿掉 `reporter_buyer_id` 的 NOT NULL。integration 測試由 migration 建庫，mutant 要改 migration；先確認不會被 checksum 檢查先擋下（那樣變紅的原因不對），會的話停下回報。
- A#1、A#2 各附 mutation，寫進 `mutation/a3-10f-7a.json`。

### A#8 盤點

- seller `CARRYOVER.md` 新增 10f-1 段：附件轉正流程、以帳號為範圍並豁免賣家能力、互斥兩欄的回覆者、`MyReportItem` 的共用欄位對店家檢舉出現兩次（A#5）、待回覆數掛在清單回應（A#6）。
- 合併清單（組裝在 `httpapi/moderation`）寫進 `backend/docs/modules/moderation.md` 與 moderation `CARRYOVER.md` 的傳輸表。

## 第 2 刀：工作日行事曆（B#1 到 B#4、B#9、B#10）

### B#1、B#2 推給 invoice-service 的鏡像

現況：`internal/platform/bizcalendar/mirror.go` 用手寫 goroutine 與 ticker；匯入後的觸發只在記憶體，行程結束就丟。`httpapi/bizcalendar/admin.go` 持有匯入後觸發推送的 hook（`Pusher`、`WithMirror`），沒有引用窗口常數。

做法（D3）：

- platform 的 `Store.Import` 在 transaction 內發布「行事曆已匯入」事件（platform 發布的先例：`delegatedactor`）。
- 新套件 `internal/legacy/bizcalendarrelay`：訂閱事件並在同一個 transaction `Enqueue` 推送 job；另有 24 小時的 periodic job 對帳（先例：`moderation/internal/retention`）。推送窗口常數搬進來。
- `worker` 行程接上 invoice client。
- 刪除 `mirror.go`、`admin.go` 的 hook、`cmd/hawkr/main.go` 的 goroutine。
- 「從今天起連續涵蓋到哪一天」的計算留在 platform 的 calendar（B#3 長期要用）。
- 主計畫 A5 記一行：刪 relay 時一併刪 platform 的事件型別與 `worker` 的 invoice client 接線。

### B#3 涵蓋徽章

回應的 `last_covered_date` 換成 `covered_through`（從今天起連續涵蓋的最後一天；今天所在年份沒匯入就不設）。handler 注入 clock。門檻與徽章文字留在 admin（只有 zh-TW、單一 client）。

### B#4 沒有讀者的 schema

新 migration 刪 `year` 欄、`days_year_check`、`days_year_day_check` 與 `days_by_year` 索引：沒有非測試的讀者，年份範圍由解析檢查。

### B#9 預覽（D2）

新增 `Preview` RPC（唯讀，不寫稽核），呼叫與匯入相同的解析函式，回應帶預覽面板現有欄位需要的資料；admin 刪掉 `holiday-calendar.ts` 的解析。

### B#10 CSV 解析

改用 `encoding/csv`（`FieldsPerRecord = 4`），保留標頭、整年、重複日與年份的檢查。這推翻 `parse.go` 說明裡的三條行為：加引號的欄位與檔案中間的空行變成可接受（政府資料的備註欄含逗號時不再整份被拒）。錯誤訊息仍是 zh-TW 並帶行號，BOM 仍自己去掉；對應測試改寫。

## 第 3 刀：mutation runner（B#8）

`admin/package.json` 拆出不含 glob 的單檔測試 script，`test` 改呼叫它；runner 的 admin 與 website 指令改由 `package.json` 取得，守門測試改讀 `package.json`。website 的部分超出累積清單，是同一種重複。

## 驗收

各 stack 與 CI 相同的檢查；`buf lint`、生成物無 drift；A#1、A#2、B#1 到 B#4 各附 mutation 清單由 CI 跑；三刀各自 code-review，全部完成後只對 10f-7 的修改跑第 2 輪設計審查。

## 順帶發現（不在 10f-7）

ADR `docs/adr/2026-10-18-seller-governance-consolidated.md` 寫認定可用類別 12 個，程式的 `Assessable()` 是 16 個（含社群類別）；admin 的 12 個是店家認定下拉的刻意子集。
