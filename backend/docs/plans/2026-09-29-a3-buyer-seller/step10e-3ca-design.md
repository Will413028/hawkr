# 10e 第 3c-a 刀：通報與案件，實作設計 v2

> **狀態**：Will 2026-10-18 對 §5 的頁面結構、§5.4 的文案、術語表的新列與 §10 的 Q1–Q6 回「照建議」。

讀取的是 `local/10e`：v1 讀 HEAD `06b158957`；v2 在 main 的 `097436097`（Will 決定 ADR D3／D4）合進來之後，重新對過單體規則（`backend/AGENTS.md` 規則 2、3、4、6）與 ADR `2026-09-24-go-modular-monolith` 文末的「修訂：跨模組讀取改用擁有者發布的 pub_*、D4 收窄」（下稱 D3／D4 修訂）。
SSOT（Will 2026-10-18）：`step10e-reports-brief.md`（v3，R1–R12，下稱 rb；3c-a 的範圍在 rb §4，被改寫的既有決定在 rb §3）；`step10e-3cd-brief.md`（下稱 3cd，§4 文案能沿用的照用）與 `step10e-3cd-inventory.md`；`step10e-3-brief.md`（v2，下稱 3b：K3′、K4′、D4′、D7–D10、K16，「實作時的修訂（3b）」第 5、6 點）；ADR `2026-10-18-seller-governance-consolidated`（下稱 gov ADR；本設計撰寫時依的是整合前的 `2026-10-02-seller-governance-in-monolith.md`，下文「gov ADR 文末的修訂段」指整合前那份檔的段落，內容已併入整合版）、`2026-10-17-multi-locale-architecture`、`2026-09-24-go-modular-monolith`；〈API 設計規範（Connect）〉；術語表。
v2：獨立審查（1 個 blocker、4 個 major、16 個 minor）與主 session 的裁示都已處理，對照見文末附錄 A。
路徑縮寫：`W` 是 worktree 根目錄；`S`＝`W/backend/internal/modules/seller`；`H`＝`W/backend/internal/httpapi`；`A`＝`W/admin/src/app/admin`；`G`＝`W/backend/gateway/internal`。
標記：**[量測]** 這次讀到原始碼或跑了指令；**[推論]** 由讀到的內容推得，沒有執行。
判定原則：上線前沒有相容性約束；每個取捨照業界從零的做法，只有基準定不了的產品或法務選擇列在 §10。

---

## 0. 範圍、決策總表與現況

### 0.1 範圍

| 部分 | 內容 |
|---|---|
| 政策分類（第一個 commit） | `PolicyCategory`（`api/hawkr/common/v1alpha1`）與 `internal/platform/policy`；各入口的子集；`reports`、`violations` 的值搬遷（`UNAUTHORIZED_SALE` → `UNLICENSED_SALE`）；目錄的 key 搬家；術語表先加；所有消費端 |
| 案件模型（R2、R3） | 通報與案件分開；每筆通報的平台判定類別與結果；不開案件直接結掉；案件的嚴重度與決定；移到其他案件；空案件自動關閉；3b 的來源規則改看結果；開發資料的轉換 |
| 措施與案件的連結 | `measures.source_case_id`（任何依據都可以連，Q4 (i)）；導出的措施＝明確連結 ∪ 認定推導；`ListMeasures` 的 `source_case_id` 與 `source_report_id` 兩個篩選 |
| 案件的附屬 | 註記（`governance_notes.case_id`，改嚴重度的改前改後也記在註記上）；轉報 v1（管道、選填案號，掛在案件）；轉報資料（不放聯絡方式） |
| R7 在 3c-a 的部分 | 重複送出補充到原通報（每筆上限 10 則）；不能檢舉自己的店；匿名聯絡方式的保留規則；`ReportReceived`（新通報與補充都發）、每筆一次的 `ReportResolved` |
| 可用性 | 通報、案件各一個推導函式；API 的 `can_*`、`available_outcomes`、`decision_options` 與指令的前置檢查共用 |
| 3d 的前置 | 待分案、待決定、待轉報的判定式、開始時間與索引（§6）；三支來源查詢、成員的資料集測試與佇列 RPC 在 3d（Q5） |
| admin | 「商家通報」頁（待分案、案件、所有通報）、通報詳情、案件詳情、6 個對話框、從案件開三種建立措施的對話框（預填與連結）、轉報資料、發票頁預填（3b 修訂 6） |
| 刪除 | 舊通報兩頁；gateway 的 7 條 admin 通報路由與它們的 handler、wrapper、DTO、openapi；`governance-wire-contract.test.ts` 與它在 baseline 的那一筆；`maxRESTOperations` 488 → 481 |
| 文件 | gov ADR 文末兩段修訂、設計書 32、兩份 runbook（K16、K19）、CARRYOVER、seller `AGENTS.md`、`backend/AGENTS.md` 的 platform 清單、術語表、API 設計規範的 common 清單 |

### 0.2 不在範圍

- 3c-b：詢問（R4，含配合要求的轉換）、轉報紀錄 v2（追蹤日期、主管機關回覆、「不轉報」）、主管機關來函、保存期與 `RetireDueStores` 的凍結。3c-a 到 3c-b 之間沒有地方記配合要求：系統外聯絡賣家的經過寫在案件的內部註記。
- 3d：`ReviewPolicy` 與所有類別的期限、三種待辦的來源查詢與成員的資料集測試（Q5）、`ListReviewQueue`、待辦頁。3c-a 的期限只有既有的食安 4 小時（§3.1）。
- 信任與安全上線前（10f）：賣家端的詢問、「我的檢舉」與補充說明、檢舉表單（同意勾選、聯絡方式快照、1919 說明）、權利人通報。
- A7-1c：社群的三個類別（`SPAM`、`HARASSMENT`、`HATE`）與 `Community()` 子集，和 moderation 的消費者同一刀（「契約與消費者放在同一段」）。
- A8：確認信與結果通知的送達（3c-a 只發事件，事件帶確認信要用的地址）。
- B4：商品層級的措施（R6）。
- 10e 收尾：曝光規則搬進 seller 擁有的 `seller.pub_exposable_stores(at)`（A7 計畫:178，Will 2026-10-18 A′）。3c-a 不新增任何 `pub_*`：讀取都在 seller 自己的 schema；檢舉的對象照舊經 `StoreCards` 判斷。

### 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| C1 | 政策分類 | 13 個值，各附條款依據；Go 在 `platform/policy`（`Values`、`ReporterSelectable`、`Assessable`）；seller 的 `model.Category` 刪除，全部改用 `policy.Category` | rb R10 |
| C2 | 舊值 | `UNAUTHORIZED_SALE` 改名；認定（`violations`）的 `OTHER` 沒有誠實的對應，migration 失敗並列出 ID，本機照 §2.3 的步驟重建 | rb R10「OTHER 只給通報人」；W2 先例（2c 設計 §9） |
| C3 | 子集給 client 的方式 | admin 一個常數模組（3c-a 只有 admin 用子集），伺服器照樣驗證。觸發：第二個 client 要用子集（10f 的檢舉表單）時，改成 proto 的 custom option，由 `uitext-gen` 生成三端常數 | 3cd §2 第 13 項「明確的子集常數」；先例 `VIOLATION_CATEGORY_OPTIONS` |
| C4 | 資料形狀 | `reports`（一位檢舉人的一次送出）＋`cases`（同一家店、同一個平台判定類別）；`reports.case_id`、`assessed_category` 以複合外鍵 `(case_id, store_id, assessed_category)` 指向 `cases (id, store_id, category)`；`cases_one_open` partial UNIQUE | rb R2；先例 `violations_store_account_fkey` |
| C5 | 結果與決定 | 結果五值，`ACTION_TAKEN` 一定在案件裡（CHECK）；案件存決定（`ACTION_TAKEN`／`NO_ACTION`）與 `close_reason`（`DECIDED`／`NO_REPORTS`）；結案時沒有結果的通報帶入預設結果與說明 | rb R2 |
| C6 | 移動 | 只限進行中案件裡、沒有結果、不是認定來源的通報；有導出措施或已記錄轉報的案件，最後一筆不能移走；移動留紀錄（`report_case_moves`，複合外鍵保證同一家店）；移走最後一筆時原案件以 `NO_REPORTS` 關閉，不算決定 | rb R2；變更留事實（Zendesk ticket audit） |
| C7 | 嚴重度 | 存在案件；`ChangeCaseSeverity` 必寫註記，改前改後記在那則註記的兩欄（CHECK 限案件的註記），不另開表 | rb R3「以明確的指令修改並寫註記」；v1 的獨立表要再加複合外鍵才保得住，同一個事實放一列比較簡單（§2.2） |
| C8 | 決定與措施（**Q1**） | 「採取措施」可選 ⇔ 有導出的措施（不論狀態），或已有成員的結果是「已採取措施」；「不採取措施」可選 ⇔ 沒有未撤銷的導出措施，也沒有「已採取措施」的成員。兩條合起來保證每個進行中的案件至少有一個可選的決定 | 審查 B1；主 session 裁示 |
| C9 | 可用性 | `ReportFacts.Availability()`、`CaseFacts.Availability()` 各一個函式，回每個動作不能做的原因；API 的布林值是「原因為空」，不輸出原因（K4′）；指令被擋時回第一個原因的句子 | 前提（Will 2026-10-15）；先例 `UpdateStatementUnavailableReasons` |
| C10 | 狀態 | `Report.state`（待分案／處理中／已有結果）、`Case.state`（進行中／已結案）由伺服器推導，不存 | rb R3「只存事實」；AIP-216 |
| C11 | etag | `reports.version` 隨補充、分案、設定結果、移動前進；`cases.version` 隨案件層級的處理前進，**補充不改案件的版本**（改由「有新的補充」標示，C14）；轉報與註記不帶 etag | etag ADR 修訂；審查 M3 |
| C12 | 鎖順序 | seller 內的全序：identity 帳號列（需要時；跨模組由下而上，放最前）→ 店家列（帳號全部的 stint，`store_id` 升序）→ 通報列（id 升序）→ **案件列（id 升序）** → 措施列。3c-a 的指令鎖店家→通報→案件；送出只鎖原通報 | 與 `create.go` 的「店家列先於通報列」一致；單體規則 4 與 D3／D4 修訂 (c) 第 3 層「seller 的列照內部既有順序……措施與通報在店家列之後」（F22） |
| C13 | 3b 來源規則 | `Report.Supporting()`＝結果不是未違規、內容不足或濫報；`can_cite_as_source`＝`Supporting()` 而且店家沒有關閉；轉報資料的通報、建立措施的預填共用 `Supporting()` | rb R2；審查 minor 11 |
| C14 | 重複送出（**Q3**） | 補充到原通報（`report_supplements`，佐證進 `report_evidence` 帶 `supplement_id`），每筆通報最多 10 則；去重只比對**還沒有結果**的通報；每次送出（新通報、補充、超過上限而丟掉）都發 `ReportReceived`，確認信寄到這次送出留的地址；補充不改案件的版本，案件顯示「有新的補充」；登入者的回應見 **Q2** | rb R7；審查 M3；Zendesk：已關閉的工單再回覆會開 follow-up |
| C15 | 自己的店 | 帳號是店家的擁有者時回 `invalid_argument`（`store_id`），兩個入口在呼叫者登入時都檢查 | rb R7 |
| C16 | 聯絡方式保留 | 有結果的清除時間＝`greatest(建立＋90 天, 結果＋30 天)`；不論有沒有結果，建立滿 365 天一律清（絕對上限，Will 2026-10-18 設計審查 #11），清理回報只靠上限清掉的筆數，見 §3.4 | rb R7；審查 minor 6 |
| C17 | 事件 | 照現行做法：指令的 `db.InTx` 內 `events.Publish`，型別在 seller `events.go`，沒有 outbox；A8 的訂閱者在同一個 transaction 內只做 `jobs.Enqueue`。新增 `ReportReceived`（帶 `supplement_id` 與這次送出的確認地址）；既有的 `ReportResolved` 改成每筆通報有了結果就發一次，不另加事件；刪 `ReportAcknowledged`；刪 `RequestCooperation` 與 `SellerCooperationRequested`（3c-b 以詢問取代）；案件不發事件 | rb R7、§3「事件」列；單體規則 4 與 D3／D4 修訂 (a)：同 transaction 的訂閱者只做讀取時推導不出來的原子變更，或只排入 job、寫 inbox（F22）；審查 M3 |
| C18 | 轉報 v1 | 掛在案件：`case_referrals`（每個案件一筆，複合外鍵＋CHECK 讓它只能掛在食安案件）；案號選填（3cd D′4）；轉報資料以案件組、不放聯絡方式（3cd D′8） | rb R2、R5 |
| C19 | 註記 | `governance_notes` 加 `case_id`（`num_nonnulls(case_id, measure_id) = 1`）；`MeasureNote` 改名 `GovernanceNote`；`CreateCaseNote`／`ListCaseNotes` | rb §3（v2 D4′ 改掛案件）；3cd §2 第 3 項 |
| C20 | 導出的措施（**Q4**） | `measures.source_case_id`：從案件建立的措施，不論依據（VIOLATION、RISK、CONTRACT）都記下來源案件，驗證同一家店、案件進行中；導出的措施＝`source_case_id` 是這個案件 ∪ 認定以案件的通報為來源。一個定義、四處 SQL（`CaseMeasureCounts` 與措施清單的三支查詢），由資料集測試逐一鎖住 | 審查 M1（RISK 停權與不帶認定的終止沒有來源通報）；主 session 裁示 |
| C21 | 3d 的前置（**Q5**） | 3c-a 交判定式、開始時間與索引；來源查詢與資料集測試在 3d | 審查 M4；主 session 裁示 |
| C22 | 配合要求 | 刪 RPC、權限與事件（沒有消費者：唯一的呼叫方是要刪的舊頁，而且它打的是 gateway）；`report_cooperation_requests` 留給 3c-b 轉成詢問，掛在不會進案件的通報上的列由 migration 擋下（R-d） | rb R4；審查 minor 4 |
| C23 | 3cd D′2 的 `repeated statuses` 慣例 | 不需要：案件狀態只有兩個，「等賣家」是 3c-b 的推導篩選，不寫進 API 規範 | rb §3（D′2 改成案件的篩選） |
| C24 | 伺服器句子 | 新檔的 `apperr` 沒有中文字面值；新增與改寫的句子進 seller `internal/text`（staff 或 default）；沒動的送出驗證句留到 I-server | 多語系 ADR D3；3cd §2 第 15 項 |
| C25 | gateway | 刪 7 條路由與只服務它們的程式；`mockSellerServiceClient` 的 7 個 stub 留著（舊 proto 仍宣告那些 RPC） | 3b K13；量測見 §0.4 |
| C26 | 讀取與跨模組入口 | admin 的讀取只由 `internal/httpapi/seller` 的 admin handler 呼叫，照 `AdminMeasures` 自己開 `db.InReadSnapshot`；讀取的本體寫成在 `db.Conn(ctx)` 上執行的函式，3d 的來源可以重用。其他模組會呼叫的方法不開 `InReadSnapshot`（現在只有 `create.go` 的 replay 一處）；3c-a 不新增這種方法。撞唯一鍵後不在同一個 transaction 繼續下 SQL：去重與轉報都用 `ON CONFLICT DO NOTHING`，`cases_one_open` 只是保底，撞到就整筆失敗 | 單體規則 3（`backend/AGENTS.md:29`）；D3／D4 修訂 (b)（F22） |
| C27 | 措施依通報篩選 | `ListMeasures.source_report_id` 與 `source_case_id` 並存；設定結果的對話框一律用前者判斷 D′9 的提示 | 審查 M2 |
| C28 | 請求的命名 | 作用在案件本身的方法用 `id`（`DecideCase`、`ChangeCaseSeverity`）；作用在案件的子項（轉報紀錄、轉報資料、註記）用 `case_id`，同 `DecideAppealRequest.measure_id`、`CreateMeasureNoteRequest.measure_id` | 審查 minor 16 |
| C29 | 用語（**Q6**） | 買家面「檢舉」、後台「通報」、併入原通報的內容叫「補充」；R7 的句子改用「檢舉」 | 審查 minor 12；主 session 裁示 |

### 0.4 查證過的現況（設計依賴的事實）

| # | 事實 | 證據 |
|---|---|---|
| F1 | 通報狀態機 `PENDING → INVESTIGATING → VERIFIED｜DISMISSED`；嚴重度、確認收件、FDA 欄位都在 `reports` | [量測] `S/db/schema.sql:211-277`；`S/internal/governance/report.go:164-300` |
| F2 | 去重是三個 partial UNIQUE，沒有結果的條件；撞到時模組只回 ID；transaction 外先查一次重複就提前返回 | [量測] `schema.sql:285-293`；`report.go:102-105`、`:118-125` |
| F3 | 登入的重複回 `already_exists`，句子指向不存在的「我的通報」；匿名回空的回應 | [量測] `H/seller/reports.go:144-146`、`:174-190` |
| F4 | 送出只檢查店家存在與曾經公開，沒有「自己的店」檢查 | [量測] `report.go:99-101`；`S/governance.go:387-398` |
| F5 | 匿名聯絡方式在建立後 90 天清，不看結果；清理依 `created_at` 排序 | [量測] `S/internal/model/governance.go:162-163`；`S/internal/governance/jobs.go:31-52`；`S/db/queries/reports.sql:146-157` |
| F6 | 3b 的來源規則看 `status = DISMISSED` | [量測] `S/internal/measure/create.go:565-582` |
| F7 | `governance_notes` 只有 `measure_id`；3cd 規劃的 `report_id` 沒有做 | [量測] `schema.sql:520-529` |
| F8 | 佐證的讀取紀錄以 `(report_id, position)` 指向佐證；佐證位置 `< 5` | [量測] `schema.sql:299-307`、`:584-597` |
| F9 | admin 契約有 8 支通報 RPC；`Report` 除了 `id` 都沒有 `field_behavior` | [量測] `W/api/hawkr/admin/seller/v1alpha1/governance.proto:22-54`、`:195-234` |
| F10 | 權限 9 個 `seller.reports.*`／`seller.fda_exports.get`／`seller.report_evidence.get` | [量測] `W/backend/internal/platform/adminperm/adminperm.go:64-72`；`H/adminauth/adminauth.go:87-94` |
| F11 | 用到 Connect 通報 API 的 client 只有 `SourceReportsField`（client 端以 `status !== DISMISSED` 過濾）；舊兩頁打 gateway 的 REST；沒有 client 呼叫 `SellerReportAppService`、`SellerReportPublicService`（seller-app 的 `report_repository.dart` 是舊的社群檢舉 REST） | [量測] `A/merchants/[id]/SourceReportsField.tsx:36-47`；`A/violation-reports/page.tsx:123-139`；`rg -ln 'report_pb\|SellerReportAppService\|createReport\|listMyReports' website admin/src buyer-app/lib seller-app/lib` |
| F12 | `initialSourceReportIds` 目前沒有任何呼叫方傳；對話框沒有預填種類、結束時間、理由、類別的 props | [量測] `A/merchants/[id]/CreateMeasureDialog.tsx:40-50`、`:86-88`；呼叫點 `merchants/[id]/page.tsx:1542`、`invoices/complaints/[complaintId]/page.tsx:575` |
| F13 | 鎖順序：帳號全部的 stint → 來源通報（id 升序）→ 措施 | [量測] `S/AGENTS.md:44`；`create.go:408-455`、`:563-582`；`S/internal/stint/stint.go:36-55`、`:136-145` |
| F14 | gateway 的 admin 通報路由 7 條、只服務它們的 helper 與型別、7 個 wrapper、7 個 openapi operation 與 3 個 schema；`maxRESTOperations = 488`，openapi 實際 488 個 operation | [量測] `G/router/routes_governance.go:59-75`；`G/handler/governance.go:354-622`；`G/grpc/governance_client.go:43-86`；`W/backend/api/openapi/openapi.yaml` 的 `:2502-2857`、`:9773-9848`、`:10542-10633`；`G/router/openapi_coverage_test.go:28`（另一個 agent 以 python yaml 數過） |
| F15 | 刪舊頁會打破兩個既有測試：`measure-pages-wiring.test.ts:82-88` 斷言舊頁存在、`:28` 斷言側欄文字 | [量測] |
| F16 | `ViolationCategory` 的非生成消費檔：backend 程式 12＋目錄 1、backend 測試 20、admin 5＋測試 3、website 1＋測試 4、seller-app 測試 3、proto 4；packages 只有生成物（seller-app 的程式經生成的 `.uiText(l)` 擴充，不直接引用型別） | [量測] `rg -l 'ViolationCategory\|VIOLATION_CATEGORY\|violationCategory\|UNAUTHORIZED_SALE\|CategoryValues\(\)\|CategoryFoodSafety'`，排除 `gen/`、`*.pb.*`、`*.sql.go`、`node_modules`、`backend/services`、docs、migrations；`seller-app/lib/features/seller/data/measure_display.dart:46` |
| F17 | 模組文字：`internal/text/zh-TW.json` 有 `default` 與 `staff` 兩段，生成 `text_gen.go`；錯誤以 `apperr.X("%s", msg)` 帶入 | [量測] `S/internal/text/`；`S/internal/measure/appeal.go:22-28` |
| F18 | migration 用過 `uuidv7()`；DB 是 Postgres 18 | [量測] `S/db/migrations/20261011174237_seller_violation_notes_revisions.sql:85`；`W/backend/internal/platform/testdb/testdb.go:31` |
| F19 | protobuf-es 對訊息欄位名 `case` 不跳脫（保留的屬性名只有 `constructor`、`toString`、`toJSON`、`valueOf`）；admin proto 不生成 Dart | [量測] `W/node_modules/.pnpm/@bufbuild+protobuf@2.15.0/.../reflect/names.js:95-108`；`W/packages/hawkr_api/buf.gen.yaml:15-17` |
| F20 | 隱私權政策沒有寫匿名檢舉人聯絡方式的保存天數，保留規則改了不必改法律文字 | [量測] `W/website/features/legal/pages/PrivacyPage.tsx:105` |
| F21 | 治理事件都沒有訂閱者 | 引用 `step10e-reports-facts.md` §2（沒有重新量測） |
| F22 | ADR D3／D4 已定（Will 2026-10-18，main `097436097`），單體規則改寫：<br>規則 2：跨模組讀取依序判斷；各來源都擁有全部篩選與排序鍵時，在 httpapi 做 keyset composition；要用下層的屬性篩選，就 join 擁有者發布的唯讀 `pub_*`；讀取模型只在量測觸發後才建，由 outbox 投影。<br>規則 3：會被別的模組呼叫的方法不得用 `InReadSnapshot`，另給加入呼叫方 transaction 的入口；撞唯一鍵後不得在同一個 transaction 繼續下 SQL。<br>規則 4：同 transaction 的訂閱者只做兩種事；跨模組的列由下往上鎖：identity → buyer → seller → 上層 → 編排者。<br>規則 6：表的 migration 由 Atlas 產生，`pub_*` 用 `atlas migrate new` 手寫。<br>D3／D4 修訂 (c) 第 3 層：「seller 的列，照 seller 內部既有順序：店家／stint → 申請 → 註冊 → 公告列；措施與通報在店家列之後」 | [量測] `backend/AGENTS.md:28-32`；`backend/docs/adr/2026-09-24-go-modular-monolith.md:304-352`（(a)–(c) 在 `:335-342`）；replay 的快照 `S/internal/measure/create.go:278`；`AdminMeasures` 的快照 `S/internal/measure/list.go:150` |
| F23 | RISK 與 CONTRACT 依據的措施沒有認定，也就沒有來源通報；食安常見的即時處置是 RISK 停權（`risk_ground = FOOD_SAFETY`）或不帶認定的終止（`FOOD_SAFETY_INCIDENT`） | [量測] `S/internal/model/measure.go:913`（`RiskBasis` 只有事由）、`:930-936`（終止的 `Violation` 選填）；`schema.sql:431-433` |
| F24 | 撤銷措施不碰通報與案件 | [量測] `rg -n 'case\|report' S/internal/measure/revoke.go` 沒有命中 |
| F25 | `admin/tests/measure-dialogs.test.ts` 用到 `MeasureNoteSchema` | [量測] `:8`、`:174`、`:182`、`:586` |
| F26 | `reports_fda_check` 讓 FDA 欄位只在食安類別 | [量測] `schema.sql:269-273` |
| F27 | 3b 的建立措施對話框提供 `OTHER` 類別，所以本機很可能有 `OTHER` 的認定 | [量測] `A/merchants/[id]/violation-measure.ts:33-41` |
| F28 | 建立措施在店家已關閉時回「店家已關閉」（終止合作例外） | [量測] `create.go:437-441` |
| F29 | 案件頁要開的另外兩個建立措施對話框：`CreateRiskMeasureDialog`（props `storeId`、`onConfirm`）、`CreateTerminationDialog`（`storeId`、`accountId`、`availableGrounds`、`effects`，事由與說明由 `GetStore` 給） | [量測] `A/merchants/[id]/CreateRiskMeasureDialog.tsx:28-47`、`CreateTerminationDialog.tsx:26-59`；`merchants/[id]/page.tsx:1015`、`:1026`、`:1037` 的入口文字 |

---

## 1. 契約

### 1.1 政策分類（第一個 commit）

新檔 `api/hawkr/common/v1alpha1/policy_category.proto`：

```proto
// PolicyCategory 是平台的政策分類（step10e-reports-brief R10）：檢舉人選的類別、平台判定類別與認定的類別共用一份，
// 各入口只接受自己的子集（見各值的「入口」）。每個值都對應賣家合作條款的依據，給賣家的理由要能指到條款。
enum PolicyCategory {
  POLICY_CATEGORY_UNSPECIFIED = 0;
  // 有健康風險或疑似食物中毒，或未揭露安全疑慮的商品（條款第十一條、第十六條第 1 款）。入口：檢舉人、平台判定與認定。
  POLICY_CATEGORY_FOOD_SAFETY = 1;
  // 違法、違禁或來源不明，而沒有健康風險的商品（第十六條第 1 款）。入口：檢舉人、平台判定與認定。
  POLICY_CATEGORY_PROHIBITED_GOODS = 2;
  // 依法應有的登錄、許可或資格沒有完成就販售（第十一條第 2 項）。入口：同上。
  POLICY_CATEGORY_UNLICENSED_SALE = 3;
  // 虛假資料、標示不實或誤導的商品或店家資訊（第十六條第 2 款）。入口：同上。
  POLICY_CATEGORY_MISLEADING_INFO = 4;
  // 不實的認證、檢驗或標章（第十六條第 2 款的一種）。入口：同上。
  POLICY_CATEGORY_FAKE_CERTIFICATION = 5;
  // 冒用他人商標或外觀的商品本身（第十六條第 1 款）。入口：同上。
  POLICY_CATEGORY_COUNTERFEIT = 6;
  // 侵害他人商標、著作權、專利、營業秘密、肖像或其他權利的使用（第十六條第 5 款），例如店名或商品標題用了別人的商標；仿冒商品本身是 COUNTERFEIT。入口：同上。
  POLICY_CATEGORY_IP_INFRINGEMENT = 7;
  // 偽造交易、洗錢、詐欺、濫用優惠、操縱評價或規避風險控管（第十六條第 2 款）。入口：同上。
  POLICY_CATEGORY_FRAUD = 8;
  // 引導買家繞過平台付款，或規避平台服務費（第十六條第 3 款）。入口：同上。
  POLICY_CATEGORY_OFF_PLATFORM_TRADE = 9;
  // 未經授權蒐集、利用、揭露或出售買家個人資料（第十六條第 4 款）。入口：同上。
  POLICY_CATEGORY_PRIVACY = 10;
  // 冒用平台名義、未經授權的保證，或讓買家誤認交易主體（第十六條第 8 款）。入口：同上。
  POLICY_CATEGORY_IMPERSONATION = 11;
  // 干擾、探測或破壞平台與資訊安全，或以爬蟲、自動化不當取得資料（第十六條第 6、7 款）。入口：只有平台判定與認定。
  POLICY_CATEGORY_PLATFORM_ABUSE = 12;
  // 檢舉人無法歸類時選。理由要指到條款，所以分案時要改判成有依據的類別，或直接設定結果。入口：只有檢舉人。
  POLICY_CATEGORY_OTHER = 13;
}
```

- 數字順序照 R10 的表；client 的顯示順序由子集常數決定，不看數字。A7-1c 從 14 起加社群的三個值。`PRIVACY` 的顯示名稱改成「不當使用個人資料」（§5.4 (i)，新提案）。
- 刪 `seller.proto` 的 `ViolationCategory`（`:1004-1014`）；`report.proto`（`MyReport.category`、`CreateReportRequest.category`）、admin `governance.proto`（`Report.category`、`ListReportsRequest.category`、`NewFinding.category`）、admin `measure.proto`（`Violation.category`）與 `seller.proto`（`MeasureFinding.category`）改成 `hawkr.common.v1alpha1.PolicyCategory`。
- `ViolationSeverity` 的註解（`seller.proto:1016`，「通報的嚴重度由後台確認收件時決定」）改成「認定套用的嚴重度，也是案件的嚴重度（開案時決定，之後只經 `ChangeCaseSeverity` 修改）」；它經 `make uitext` 進目錄的說明欄。
- 各入口接受的子集（伺服器驗證，proto 註解寫明）：

| 入口 | 子集 | 值數 |
|---|---|---|
| `CreateReport.category`、檢舉人選的類別 | `ReporterSelectable`：除了 `PLATFORM_ABUSE` | 12 |
| 匿名食安（`SubmitFoodSafetyReport`） | 固定 `FOOD_SAFETY` | 1 |
| 平台判定類別（分案、移動）、案件的類別、`NewFinding.category`（認定） | `Assessable`：除了 `OTHER` | 12 |
| 清單的類別篩選 | 依那一欄：檢舉人選的類別或案件的類別取上面的子集，生效的類別（§1.3）可以是全部 13 個 | — |

- Go：`internal/platform/policy`（platform 只依賴 platform，`internal/arch` 不必登記）：`type Category string`、13 個常數、`Values()`、`ReporterSelectable()`、`Assessable()` 與同名的 `(c Category) ReporterSelectable() bool`、`Assessable() bool`。`TestChecksMatchGo` 改成每個 CHECK 對自己的子集（`reports.category` ↔ `ReporterSelectable`；`violations.category`、`cases.category` ↔ `Assessable`）。
- API 規範第一節的 common 清單加上「政策分類 `PolicyCategory`（seller 與 A7 的 moderation 共用）」；`backend/AGENTS.md` 的 platform 清單加 `policy`。

### 1.2 檢舉人的契約（`hawkr/seller/v1alpha1/report.proto`）

```proto
// ReportOutcome 是一筆檢舉的結果（step10e-reports-brief R2）。還沒有結果時是 UNSPECIFIED（與 resolve_time 不設同時成立）。
enum ReportOutcome {
  REPORT_OUTCOME_UNSPECIFIED = 0;
  // 平台採取了措施（只會是案件裡的檢舉）。
  REPORT_OUTCOME_ACTION_TAKEN = 1;
  // 查過之後沒有違規。
  REPORT_OUTCOME_NO_VIOLATION = 2;
  // 內容不足，無法判斷。
  REPORT_OUTCOME_INSUFFICIENT = 3;
  // 與另一筆檢舉重複。
  REPORT_OUTCOME_DUPLICATE = 4;
  // 濫用檢舉。
  REPORT_OUTCOME_ABUSIVE = 5;
}
// 刪除 ReportStatus。

message MyReport {
  reserved 7; reserved "status";
  …（1–6、8–10 不變；category 改型別）
  ReportOutcome outcome = 11 [(google.api.field_behavior) = OUTPUT_ONLY];
}

message CreateReportResponse {
  // 新的檢舉；supplemented 時是原本那一筆（去重的鍵含買家，所以一定是呼叫者自己的）。
  MyReport report = 1;
  // Q2 的建議：這次的內容補充到同一個台北日、同一家店、同一個類別、還沒有結果的那一筆。
  bool supplemented = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  // supplemented 時給檢舉人看的那一句（伺服器的句子，#D′1）。
  google.rpc.LocalizedMessage notice = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
}
```

- `CreateReport` 的註解改成：
  - 去重期間內、還沒有結果的那一筆存在時，內容補充到它（`supplemented`）；原本那一筆已有結果時，這次是新的檢舉。
  - 那一筆已有 10 則補充時回 `failed_precondition`（#D′3）。
  - 店家屬於呼叫者的帳號時回 `invalid_argument`（`store_id`）。
- `SubmitFoodSafetyReport` 的註解改成：
  - 重複時補充到原本那一筆；超過 10 則時內容不存，回應照舊沒有內容。
  - 呼叫者登入而且店家屬於他時回 `invalid_argument`。
  - 聯絡方式在「結果設定後 30 天、且不早於送出後 90 天」清除；不論有沒有結果，送出滿 365 天一律清除。
- Q2 選 (b) 時 `CreateReportResponse` 不變，重複照舊回 `already_exists`，訊息換成 #D′1。

### 1.3 後台：通報與案件（`hawkr/admin/seller/v1alpha1/governance.proto`）

**Service 的變動**（AIP 方法；`NO_SIDE_EFFECTS` 的以 GET 呼叫）

| RPC | 動作 | AIP |
|---|---|---|
| `ListReports` | 改：篩選 `state`、`category`（生效的類別）、`store_id`、`case_id` | 132 |
| `GetReport` | 改：帶補充、etag、`can_*` | 131 |
| `MarkReportSupplementsSeen` | 新（fix3）：記下管理員看過這筆通報的全部補充；帶讀到的 etag、**不讓 etag 前進**、冪等 | 136 |
| `TriageReport` | 新：判定類別並掛到案件（沒有就開一個） | 136 |
| `ResolveReport` | 改：設定結果（取代 VERIFIED／DISMISSED） | 136 |
| `MoveReport` | 新：改判定類別，移到那個類別的案件 | 136 |
| `ListCases`、`GetCase` | 新 | 132、131 |
| `DecideCase` | 新：決定即結案 | 136 |
| `ChangeCaseSeverity` | 新：改嚴重度並寫註記 | 136 |
| `RecordFDAReferral`、`GetFDAExport` | 改：以案件為單位 | 136、131 |
| `CreateCaseNote`、`ListCaseNotes` | 新 | 133、132 |
| `CreateMeasure` | 改：加 `source_case_id`（§1.4） | 133 |
| `ListMeasures` | 改：加 `source_case_id`、`source_report_id`（§1.4） | 132 |
| `AcknowledgeReport`、`StartInvestigation`、`RequestCooperation` | 刪 | — |

**列舉**（`ReportState`、`CaseState` 是推導的；`CaseCloseReason`、`CaseDecision`、`SupplementMatch` 是儲存的事實；都只有 admin 用，文字是 admin 的單端表）

```proto
// ReportState 是通報此刻在後台的處理狀態，由伺服器推導（不儲存）。
enum ReportState { REPORT_STATE_UNSPECIFIED = 0; REPORT_STATE_AWAITING_TRIAGE = 1; REPORT_STATE_IN_CASE = 2; REPORT_STATE_RESOLVED = 3; }
// AWAITING_TRIAGE：沒有案件也沒有結果（3d 的「待分案」）。IN_CASE：在進行中的案件裡、還沒有結果。RESOLVED：有結果。
enum CaseState { CASE_STATE_UNSPECIFIED = 0; CASE_STATE_OPEN = 1; CASE_STATE_CLOSED = 2; }
enum CaseCloseReason { CASE_CLOSE_REASON_UNSPECIFIED = 0; CASE_CLOSE_REASON_DECIDED = 1; CASE_CLOSE_REASON_NO_REPORTS = 2; }
// NO_REPORTS：通報都移到其他案件，系統關閉，不是一次決定。
enum CaseDecision { CASE_DECISION_UNSPECIFIED = 0; CASE_DECISION_ACTION_TAKEN = 1; CASE_DECISION_NO_ACTION = 2; }
// 匿名的補充以哪一種聯絡方式比對到原通報；BOTH 是電話與聯絡信箱都相同。
enum SupplementMatch { SUPPLEMENT_MATCH_UNSPECIFIED = 0; SUPPLEMENT_MATCH_PHONE = 1; SUPPLEMENT_MATCH_EMAIL = 2; SUPPLEMENT_MATCH_BOTH = 3; }
// 刪除 CooperationTemplate 與 CooperationRequest；FDAChannel、FDAReferral 不變（reference_no 空字串＝沒有案號）。
```

**`Report`**（v1alpha1 可以直接改；刪掉的號碼 reserved）

```proto
message Report {
  string id = 1 [IDENTIFIER];
  string store_id = 2 [OUTPUT_ONLY];
  string store_name = 24 [OUTPUT_ONLY];              // K5′；清理後空字串，client 顯示店家 ID
  string product_id = 3 [OUTPUT_ONLY]; string order_no = 4 [OUTPUT_ONLY];
  PolicyCategory category = 5 [OUTPUT_ONLY];         // 檢舉人選的
  string description = 6 [OUTPUT_ONLY];
  string reporter_buyer_id = 7 [OUTPUT_ONLY];
  ReporterContact contact = 8 [OUTPUT_ONLY];         // 只有 GetReport 帶（清單與轉報資料都不帶）
  google.protobuf.Timestamp contact_erase_time = 9 [OUTPUT_ONLY];
  reserved 10 to 15, 19, 21;
  reserved "status", "severity", "acknowledge_due_time", "acknowledge_time", "acknowledged_by_admin_id",
           "investigate_time", "fda_referral", "cooperation_requests";
  ReportState state = 25 [OUTPUT_ONLY];
  string case_id = 26 [OUTPUT_ONLY];                 // 沒有案件時空字串
  PolicyCategory assessed_category = 27 [OUTPUT_ONLY];  // 平台判定類別＝案件的類別；沒有案件時 UNSPECIFIED
  google.protobuf.Timestamp triage_time = 28 [OUTPUT_ONLY];   // 第一次分案的時間，移動不改
  string triaged_by_admin_id = 29 [OUTPUT_ONLY];
  // 分案的期限，只在待分案時設。3c-a 只有檢舉人選了食安的有期限（建立後 4 小時，現行規則）；3d 起所有類別。
  google.protobuf.Timestamp triage_due_time = 30 [OUTPUT_ONLY];
  hawkr.seller.v1alpha1.ReportOutcome outcome = 31 [OUTPUT_ONLY];
  google.protobuf.Timestamp resolve_time = 16 [OUTPUT_ONLY];
  string resolved_by_admin_id = 17 [OUTPUT_ONLY];
  string resolution_note = 18 [OUTPUT_ONLY];         // 給檢舉人的結果說明
  repeated Evidence evidence = 20 [OUTPUT_ONLY];     // 只有 GetReport 帶；原通報與補充的佐證都在這裡
  repeated ReportSupplement supplements = 32 [OUTPUT_ONLY];  // 只有 GetReport 帶，舊到新
  int32 supplement_count = 33 [OUTPUT_ONLY];
  google.protobuf.Timestamp latest_supplement_time = 34 [OUTPUT_ONLY];  // 沒有補充時不設
  int32 discarded_submission_count = 35 [OUTPUT_ONLY];  // 補充已達上限、內容沒有存下的匿名送出次數
  string etag = 36;                                  // reports.version
  bool can_triage = 37 [OUTPUT_ONLY];
  bool can_resolve = 38 [OUTPUT_ONLY];               // 等於 available_outcomes 不為空
  repeated OutcomeOption available_outcomes = 39 [OUTPUT_ONLY];   // 每個結果帶 negates_source（設計審查 #4，附錄 D）
  PolicyCategory effective_category = 42 [OUTPUT_ONLY];     // 在案件裡是平台判定類別，其他是檢舉人選的類別（附錄 D #4）
  bool has_unseen_supplements = 43 [OUTPUT_ONLY];           // 附錄 D #8
  reserved 44; reserved "supplements_seen_time";            // 已看過的標記是補充編號，不輸出（附錄 D #8）
  repeated TriageTarget triage_targets = 45 [OUTPUT_ONLY];  // 只有 GetReport 帶；每個類別的去處（附錄 D #1）
  bool can_move = 40 [OUTPUT_ONLY];
  bool can_cite_as_source = 41 [OUTPUT_ONLY];        // 3b 修訂 6-2：SourceReportsField 改看它；店家已關閉時是 false
  google.protobuf.Timestamp create_time = 22 [OUTPUT_ONLY];
  google.protobuf.Timestamp update_time = 23 [OUTPUT_ONLY];
}

message ReportSupplement {
  string id = 1;
  string description = 2;
  google.protobuf.Timestamp create_time = 3;
  repeated int32 evidence_positions = 4;             // 這則補充帶來的佐證在 Report.evidence 的位置
  SupplementMatch matched_by = 5;                    // 匿名的補充才有（Q3）；登入的補充是 UNSPECIFIED
}
```

**`Case`**

```proto
message Case {
  string id = 1 [IDENTIFIER];
  string store_id = 2 [OUTPUT_ONLY]; string store_name = 3 [OUTPUT_ONLY];
  hawkr.common.v1alpha1.PolicyCategory category = 4 [OUTPUT_ONLY];
  hawkr.seller.v1alpha1.ViolationSeverity severity = 5 [OUTPUT_ONLY];
  CaseState state = 6 [OUTPUT_ONLY];
  CaseCloseReason close_reason = 7 [OUTPUT_ONLY];    // 進行中時 UNSPECIFIED
  CaseDecision decision = 8 [OUTPUT_ONLY];           // 只在 DECIDED
  string decision_note = 9 [OUTPUT_ONLY];            // 結案時給檢舉人的結果說明（帶入沒有結果的通報）
  string opened_by_admin_id = 10 [OUTPUT_ONLY]; google.protobuf.Timestamp open_time = 11 [OUTPUT_ONLY];
  string closed_by_admin_id = 12 [OUTPUT_ONLY]; google.protobuf.Timestamp close_time = 13 [OUTPUT_ONLY];
  int32 report_count = 14 [OUTPUT_ONLY];
  int32 unresolved_report_count = 15 [OUTPUT_ONLY];  // 結案時會帶入預設結果的筆數
  FDAReferral fda_referral = 16 [OUTPUT_ONLY];       // 沒有轉報時不設
  repeated ReportMove moves = 17 [OUTPUT_ONLY];      // 只有 GetCase 帶：移入與移出這個案件的移動，舊到新
  // 上一次處理的時間：成員進出、成員有了結果、改嚴重度、記錄轉報、結案。補充、註記與措施的建立或撤銷不改它。
  google.protobuf.Timestamp update_time = 18 [OUTPUT_ONLY];
  // 成員裡有補充還沒被看過的通報數（補充不改案件 etag，改由這個數字提醒；審查 M3，附錄 D #8、fix3）。
  int32 unseen_supplement_report_count = 28 [OUTPUT_ONLY];   // 取代已移除的案件旗標（19 reserved）
  string etag = 20;                                  // cases.version
  bool can_decide = 21 [OUTPUT_ONLY];                // 等於 decision_options 不為空；進行中的案件一定是 true（C8 的不變量）
  repeated DecisionOption decision_options = 22 [OUTPUT_ONLY];
  bool can_change_severity = 23 [OUTPUT_ONLY];
  bool can_link_measure = 24 [OUTPUT_ONLY];          // 案件進行中：新的措施可以記它為來源案件（終止合作的入口另看 GetStore.can_create_termination）
  bool can_create_measure = 25 [OUTPUT_ONLY];        // can_link_measure 而且店家沒有關閉（警告、停權與風險措施要開著的店）
  bool can_record_fda_referral = 26 [OUTPUT_ONLY];
  bool can_export_fda = 27 [OUTPUT_ONLY];
}
// 一個可以選的決定，帶結案時會帶入沒有結果的通報的結果（client 不自己對應）。
message DecisionOption { CaseDecision decision = 1; hawkr.seller.v1alpha1.ReportOutcome default_outcome = 2; }
message ReportMove { string report_id = 1; string from_case_id = 2; string to_case_id = 3;
  string moved_by_admin_id = 4; google.protobuf.Timestamp move_time = 5; }
```

**請求與回應**

```proto
message ListReportsRequest {
  int32 page_size = 1;  string page_token = 2;       // 預設 20，上限 100
  reserved 3; reserved "status";
  // 生效的類別（3cd D′10）：在案件裡的是平台判定類別，其他是檢舉人選的類別；UNSPECIFIED 不篩。
  PolicyCategory category = 4;
  string store_id = 5;
  // UNSPECIFIED 不篩。
  ReportState state = 6;
  // 只列這個案件的通報；空字串不篩。
  // 排序：state 是 AWAITING_TRIAGE 或有 case_id 時依建立時間舊到新（兩者都設時一樣，結果是交集），其他新到舊。
  string case_id = 7;
}
message TriageReportRequest {
  string id = 1 [REQUIRED];
  PolicyCategory assessed_category = 2 [REQUIRED];   // Assessable 子集
  // 去處（設計審查 #1，附錄 D）：oneof { join{case_id, etag}；open_new{severity} }，取自 Report.triage_targets；鎖內驗證意圖仍然成立。
  CaseTarget target = 5 [REQUIRED];                  // 取代條件式必填的 severity = 3（reserved）
  string etag = 4 [REQUIRED];
}
message TriageReportResponse { Report report = 1; }
message ResolveReportRequest {
  string id = 1 [REQUIRED];
  hawkr.seller.v1alpha1.ReportOutcome outcome = 2 [REQUIRED];  // 必須在 Report.available_outcomes 裡
  string note = 3;                                   // 給檢舉人的結果說明，最多 2000 字
  string etag = 4 [REQUIRED];
}
message ResolveReportResponse { Report report = 1; }
message MoveReportRequest {
  string id = 1 [REQUIRED];
  PolicyCategory assessed_category = 2 [REQUIRED];   // 新的平台判定類別，必須和目前的不同
  CaseTarget target = 5 [REQUIRED];                  // 規則同 TriageReportRequest.target（severity = 3 reserved）
  string etag = 4 [REQUIRED];
}
message MoveReportResponse {
  Report report = 1;
  bool source_case_closed = 2;                       // 原案件因為通報都移走而關閉（NO_REPORTS）
}
message ListCasesRequest {
  int32 page_size = 1; string page_token = 2;        // 依開案時間新到舊
  CaseState state = 3;                               // UNSPECIFIED 不篩；3c-b 另加「等賣家」的篩選
  PolicyCategory category = 4;                       // 案件的類別
  string store_id = 5;
}
message ListCasesResponse { repeated Case cases = 1; string next_page_token = 2; }
message GetCaseRequest { string id = 1 [REQUIRED]; }
message GetCaseResponse { Case case = 1; }
message DecideCaseRequest {
  string id = 1 [REQUIRED];
  CaseDecision decision = 2 [REQUIRED];              // 必須在 Case.decision_options 裡
  string note = 3;                                   // 給檢舉人的結果說明，最多 2000 字；帶入沒有結果的通報
  string etag = 4 [REQUIRED];
}
message DecideCaseResponse { Case case = 1; }
message ChangeCaseSeverityRequest {
  string id = 1 [REQUIRED];
  ViolationSeverity severity = 2 [REQUIRED];         // 必須和目前的不同
  string note = 3 [REQUIRED];                        // 原因，1–2000 字，寫成一則內部註記
  string etag = 4 [REQUIRED];
}
message ChangeCaseSeverityResponse { Case case = 1; }
// 以下作用在案件的子項，用 case_id（C28）。
message RecordFDAReferralRequest {
  string case_id = 1 [REQUIRED];
  FDAChannel channel = 2 [REQUIRED];
  string reference_no = 3;                           // 選填，最多 100 字（3cd D′4）
}
message RecordFDAReferralResponse { Case case = 1; bool already_recorded = 2; }
message GetFDAExportRequest { string case_id = 1 [REQUIRED]; }
message GetFDAExportResponse {
  Case case = 1;
  // 案件裡結果不是未違規、內容不足或濫報的通報（含補充與佐證的位置），舊到新；不帶檢舉人的聯絡方式（3cd D′8）。
  repeated Report reports = 2;
  Store store = 3;                                   // 被檢舉的店家（已關閉的也回）
}
message CreateCaseNoteRequest { string case_id = 1 [REQUIRED]; GovernanceNote note = 2 [REQUIRED]; }
message CreateCaseNoteResponse { GovernanceNote note = 1; }
message ListCaseNotesRequest { string case_id = 1 [REQUIRED]; int32 page_size = 2; string page_token = 3; }
message ListCaseNotesResponse { repeated GovernanceNote notes = 1; string next_page_token = 2; }  // 舊到新
```

- 檔頭註解改寫：通報與案件分開；指令只在前置條件成立時可以做，否則 `failed_precondition`；可用性看 `can_*`。
- `NewFinding.source_report_ids` 的註解改成「每一筆都必須是同一家店，而且結果不是未違規、內容不足或濫報」。
- `case` 是合法的欄位名（F19）；`GetCaseResponse.case` 照規範「Get 回 `{ <資源> <資源> = 1 }`」。

### 1.4 措施與案件、通報的連結

```proto
message CreateMeasureRequest {
  …（不變）
  // 選填：從案件建立時帶案件 id。任何種類與依據都可以帶；措施的店家必須是案件的店家（invalid_argument），
  // 案件必須進行中（failed_precondition）。之後這個措施算是案件導出的措施（ListMeasures.source_case_id）。
  string source_case_id = 14;
}
message ListMeasuresRequest {
  …（不變）
  // 只列這個案件導出的措施：source_case_id 是這個案件，或依據的認定（含引用的既有認定）以這個案件的通報為來源；空字串不篩。
  string source_case_id = 12;
  // 只列依據的認定（含引用的既有認定）以這筆通報為來源的措施；空字串不篩。設定結果的對話框用它（D′9 的提示）。
  string source_report_id = 13;
}
```

- 兩個篩選可以和其他篩選組合；三支清單查詢（`AdminMeasures`、`AdminMeasuresPendingRequests`、`AdminMeasuresReviewDue`）都加同一組條件（§2.4）。
- 案件詳情的「導出的措施」用 `source_case_id`，沿用措施清單的呈現與狀態（3cd D′3）。
- `requestHash`（`create.go:183`）把 `source_case_id` 編進去：同一個 `request_id` 帶不同的案件是不同的請求。
- 若 Q4 選 (ii)：刪 `CreateMeasureRequest.source_case_id` 與 `measures.source_case_id`；`ListMeasures.source_case_id` 只剩認定的推導；Q1 的兩條規則改成不阻擋的提示（§3.1 的「Q4 (ii) 的差異」）。

### 1.5 註記

`MeasureNote` 改名 `GovernanceNote`，欄位不變，加一個欄位：

```proto
message GovernanceNote {
  …（1–4 不變）
  // 這則註記是改嚴重度時寫的：改前與改後。只有案件的註記可能有，措施的註記不設。
  SeverityChange severity_change = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
}
message SeverityChange { hawkr.seller.v1alpha1.ViolationSeverity from_severity = 1; hawkr.seller.v1alpha1.ViolationSeverity to_severity = 2; }
```
`CreateMeasureNoteRequest.note`、`ListMeasureNotesResponse.notes` 改型別。`CreateCaseNote`／`ListCaseNotes` 的規則照措施註記：只新增、不帶 etag、不改案件的版本，案件不存在回 `not_found`；已結案的案件也可以寫。`CreateCaseNote` 不能自己帶 `severity_change`（只由 `ChangeCaseSeverity` 寫）。

### 1.6 刪除的契約

- RPC：`AcknowledgeReport`、`StartInvestigation`、`RequestCooperation`，連同它們的請求與回應。
- 訊息與列舉：`CooperationRequest`、`CooperationTemplate`；`seller.v1alpha1.ReportStatus`、`ViolationCategory`。
- 權限：`seller.reports.acknowledge`、`seller.reports.start_investigation`、`seller.reports.request_cooperation`、`seller.reports.record_fda_referral`（改名，§1.9）。

### 1.7 錯誤（不新增 `ErrorReason`）

| 情形 | code | details |
|---|---|---|
| etag 不符（含成功後帶舊 etag 重送） | aborted | `ErrorInfo{VERSION_CONFLICT}`，`metadata.conflictTarget` 是 `report`（通報的 etag 過時）或 `case`（案件的 etag 過時，含分案與移動要加入的案件）；不新增 reason，client 依它說是通報還是案件剛被更新（fix3） |
| 必填欄位沒填、列舉值不在那個入口的子集、字數超過 | invalid_argument | `BadRequest` 指向欄位 |
| 分案或移動：開新案件沒帶嚴重度、加入進行中的案件卻帶了嚴重度 | invalid_argument | `severity` |
| 移動到相同的類別；改嚴重度卻和目前相同 | invalid_argument | `assessed_category`；`severity` |
| 檢舉自己的店 | invalid_argument | `store_id` |
| 建立措施帶的案件不存在、不是這家店的 | invalid_argument | `source_case_id` |
| 可用性不成立（已分案、已有結果、不在進行中的案件、是認定來源、最後一筆而案件有導出措施、案件已結案、類別不是食安、沒有通報、C8 的措施條件、建立措施時案件已結案、登入者的補充已達上限） | failed_precondition | 只有 `LocalizedMessage`（第一個原因的句子；4A-3 先例：不帶欄位、不帶 reason） |
| 通報或案件不存在 | not_found | — |

`H/rpc/fields.go` 新增欄位名：`AssessedCategory`、`Outcome`、`Decision`、`CaseID`、`State`、`SourceCaseID`、`SourceReportID`。

### 1.8 冪等與 etag

| 方法 | etag | 理由 |
|---|---|---|
| `TriageReport`、`ResolveReport`、`MoveReport` | 必填（通報的版本） | 決定依據通報的內容；補充會在狀態不變時改變內容，讓過期的分案擋下來 |
| `DecideCase`、`ChangeCaseSeverity` | 必填（案件的版本） | 依據案件的嚴重度與成員；補充不改案件的版本，由 `unseen_supplement_report_count` 提醒（C11） |
| `RecordFDAReferral` | 不帶 | 只依據狀態，只記一次；記過時回原本的紀錄（`already_recorded`），沿用現行行為 |
| `CreateCaseNote` | 不帶 | 只新增，不改任何版本（2c G10、D4′） |
| `CreateReport` | 不帶，沒有 `request_id` | 去重由 partial UNIQUE；沒收到回應的重送會變成一則補充，受送出額度與補充上限限制 |
| `CreateMeasure` | 不變（選填 `request_id`） | `source_case_id` 編進 request hash |

版本前進的寫入（同一個 transaction）：
- `reports.version`：補充、分案、設定結果（個別或結案帶入）、移動。補充每筆最多 10 則，所以一筆通報的分案最多被補充擋 10 次。
- `cases.version` 與 `updated_at`：成員加入或移出、成員有了結果、改嚴重度、決定、自動關閉、記錄轉報。補充、寫註記、建立或撤銷措施都不改；措施的條件在指令裡鎖住後重算（§3.2）。

重送成功過的指令一律先撞版本（`aborted`），所以沒有「成功了卻回錯」的分支要另外處理。

### 1.9 權限（`adminperm` 與 `adminauth`）

| 方法 | 權限 | 變動 |
|---|---|---|
| `ListReports`、`GetReport` | `seller.reports.list`、`.get` | 不變 |
| `MarkReportSupplementsSeen` | `seller.reports.mark_supplements_seen` | 新（fix3）：adminauth 一支方法一個權限（`TestEachPermissionBelongsToOneMethod`），所以不共用 `.get`；目錄裡與 `.get` 一起授予 |
| `TriageReport` | `seller.reports.triage` | 新 |
| `ResolveReport` | `seller.reports.resolve` | 名稱不變，語意改成設定結果 |
| `MoveReport` | `seller.reports.move` | 新 |
| `ListCases`、`GetCase` | `seller.cases.list`、`.get` | 新 |
| `DecideCase` | `seller.cases.decide` | 新 |
| `ChangeCaseSeverity` | `seller.cases.change_severity` | 新 |
| `RecordFDAReferral` | `seller.cases.record_fda_referral` | 改名（原 `seller.reports.record_fda_referral`） |
| `GetFDAExport` | `seller.fda_exports.get` | 不變 |
| `CreateCaseNote`、`ListCaseNotes` | `seller.case_notes.create`、`.list` | 新 |
| 佐證端點 | `seller.report_evidence.get` | 不變 |

新增 9 個、刪除 4 個（含改名的舊名）。都進 `Catalog()`，OWNER 擁有全部。`CreateMeasure`、`ListMeasures` 的權限不變。

---

## 2. Schema 與 migration

模組慣例與單體規則 6：
- 只有 up。
- 3c-a 只動表，`atlas migrate diff` 產生後在 DDL 之間手寫轉換，重算 `atlas.sum`；沒有 `pub_*`，所以不用 `atlas migrate new`。
- `schema.sql` 與套用結果相同，由 `TestSchemaFileMatchesMigrations` 守。
- 時間戳晚於 main 上最新的一支（目前 `20261018000223`）。

seller 之後還會有手寫的 `pub_*` migration：10e 收尾的 `seller.pub_exposable_stores(at)`，以及 A7 計畫的 `seller.pub_blocks`（A7 計畫:177-178）。協調方式：各刀都取 rebase 當下 main 最新一支之後的時間戳；後 rebase 的一方重新產生時間戳並 `atlas migrate hash`，不改已落地的檔。

### 2.1 政策分類的 migration（第一個 commit，`<ts>_seller_policy_category.sql`）

```sql
-- 認定的類別不能是 OTHER（R10：OTHER 只給檢舉人選）；沒有誠實的對應時整個失敗並列出 ID（W2 先例），本機照 §2.3 重建後重跑。
DO $$ DECLARE ids text; BEGIN
  SELECT string_agg(id::text, ', ' ORDER BY id) INTO ids FROM seller.violations WHERE category = 'OTHER';
  IF ids IS NOT NULL THEN RAISE EXCEPTION '認定的類別 OTHER 沒有對應的政策類別：%', ids; END IF;
END $$;
ALTER TABLE seller.reports DROP CONSTRAINT reports_category_check;
ALTER TABLE seller.violations DROP CONSTRAINT violations_category_check;
UPDATE seller.reports SET category = 'UNLICENSED_SALE' WHERE category = 'UNAUTHORIZED_SALE';
UPDATE seller.violations SET category = 'UNLICENSED_SALE' WHERE category = 'UNAUTHORIZED_SALE';
ALTER TABLE seller.reports ADD CONSTRAINT reports_category_check CHECK (category IN (<ReporterSelectable 的 12 個>));
ALTER TABLE seller.violations ADD CONSTRAINT violations_category_check CHECK (category IN (<Assessable 的 12 個>));
```
- 改值不會撞 `reports_buyer_daily`：改名之前沒有 `UNLICENSED_SALE` 的列。
- `measures.request_hash` 不重算：只影響本機資料以舊類別的同一個 `request_id` 重送（會回「內容不同」）［推論］。

### 2.2 案件的 schema（第三個 commit，`<ts>_seller_report_cases.sql`）

```sql
-- 案件（rb R2）：同一家店、同一個平台判定類別，同時最多一個進行中的案件。嚴重度在開案時設定，之後只經 ChangeCaseSeverity 修改。
-- 決定就是結案（終態）；通報都移走時以 NO_REPORTS 關閉，不算決定。version 是後台的 etag，updated_at 是上一次處理。
CREATE TABLE seller.cases (
    id                 UUID PRIMARY KEY,
    store_id           BIGINT NOT NULL REFERENCES seller.stores (store_id),
    category           TEXT NOT NULL,
    severity           TEXT NOT NULL,
    version            BIGINT NOT NULL DEFAULT 1,
    opened_at          TIMESTAMPTZ NOT NULL,
    opened_by_admin_id BIGINT NOT NULL,
    closed_at          TIMESTAMPTZ,
    closed_by_admin_id BIGINT,
    close_reason       TEXT,
    decision           TEXT,
    decision_note      TEXT,
    updated_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT cases_report_key UNIQUE (id, store_id, category),   -- reports 的複合外鍵
    CONSTRAINT cases_store_key UNIQUE (id, store_id),              -- measures.source_case_id、report_case_moves 的複合外鍵
    CONSTRAINT cases_category_key UNIQUE (id, category),           -- case_referrals 的複合外鍵
    CONSTRAINT cases_category_check CHECK (category IN (<Assessable 的 12 個>)),
    CONSTRAINT cases_severity_check CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL')),
    CONSTRAINT cases_close_check CHECK (
        (closed_at IS NULL AND closed_by_admin_id IS NULL AND close_reason IS NULL AND decision IS NULL AND decision_note IS NULL) OR
        (closed_at >= opened_at AND closed_by_admin_id > 0 AND close_reason = 'DECIDED' AND decision IN ('ACTION_TAKEN', 'NO_ACTION')
         AND (decision_note IS NULL OR char_length(decision_note) BETWEEN 1 AND 2000)) OR
        (closed_at >= opened_at AND closed_by_admin_id > 0 AND close_reason = 'NO_REPORTS' AND decision IS NULL AND decision_note IS NULL)
    ),
    CONSTRAINT cases_admin_check CHECK (opened_by_admin_id > 0),
    CONSTRAINT cases_version_check CHECK (version >= 1),
    CONSTRAINT cases_updated_check CHECK (updated_at >= opened_at)
);
CREATE UNIQUE INDEX cases_one_open ON seller.cases (store_id, category) WHERE closed_at IS NULL;
CREATE INDEX cases_opened ON seller.cases (opened_at DESC, id DESC);
CREATE INDEX cases_open ON seller.cases (opened_at DESC, id DESC) WHERE closed_at IS NULL;
CREATE INDEX cases_closed ON seller.cases (opened_at DESC, id DESC) WHERE closed_at IS NOT NULL;
CREATE INDEX cases_store ON seller.cases (store_id, opened_at DESC, id DESC);
-- 3d「待轉報」的來源：決定採取措施的食安案件（是否已轉報由 NOT EXISTS case_referrals 判斷）。
CREATE INDEX cases_decided_food_safety ON seller.cases (closed_at, id) WHERE category = 'FOOD_SAFETY' AND decision = 'ACTION_TAKEN';

-- reports：刪 status、severity、acknowledged_*、investigating_at、fda_*；加下列欄位與約束。
ALTER TABLE seller.reports
    ADD COLUMN case_id UUID, ADD COLUMN assessed_category TEXT,
    ADD COLUMN triaged_at TIMESTAMPTZ, ADD COLUMN triaged_by_admin_id BIGINT,
    ADD COLUMN outcome TEXT,
    ADD COLUMN discarded_submissions INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN version BIGINT NOT NULL DEFAULT 1;
--   reports_store_key      UNIQUE (id, store_id)（report_case_moves 的複合外鍵）
--   reports_case_fkey      FOREIGN KEY (case_id, store_id, assessed_category) REFERENCES seller.cases (id, store_id, category)
--   reports_triage_check   (case_id IS NULL) = (assessed_category IS NULL) AND (case_id IS NULL) = (triaged_at IS NULL)
--                          AND (triaged_at IS NULL) = (triaged_by_admin_id IS NULL) AND (triaged_at IS NULL OR triaged_at >= created_at)
--   reports_outcome_check  outcome IN ('ACTION_TAKEN', 'NO_VIOLATION', 'INSUFFICIENT', 'DUPLICATE', 'ABUSIVE')
--   reports_resolved_check (outcome IS NULL) = (resolved_at IS NULL) AND (resolved_at IS NULL) = (resolved_by_admin_id IS NULL)
--                          AND (resolution_note IS NULL OR (resolved_at IS NOT NULL AND char_length(resolution_note) BETWEEN 1 AND 2000))
--                          AND (resolved_at IS NULL OR resolved_at >= created_at)
--   reports_action_taken_check  outcome IS DISTINCT FROM 'ACTION_TAKEN' OR case_id IS NOT NULL
--   reports_admin_check    triaged_by_admin_id > 0 AND resolved_by_admin_id > 0
--   reports_counts_check   discarded_submissions >= 0 AND version >= 1
--   刪 reports_status_check、reports_severity_check、reports_fda_channel_check、reports_acknowledged_check、
--      reports_status_times_check、reports_fda_check；檢舉人與內容的 CHECK 不變
-- 索引：刪 reports_status；加
CREATE INDEX reports_awaiting_triage ON seller.reports (created_at, id) WHERE case_id IS NULL AND outcome IS NULL;
CREATE INDEX reports_case ON seller.reports (case_id, created_at, id) WHERE case_id IS NOT NULL;
-- 去重只比對還沒有結果的通報（C14）：三個 partial UNIQUE 的 WHERE 加 AND outcome IS NULL。
-- 聯絡方式保留（C16）：reports_contact_retention 曾改成 (resolved_at) 的部分索引，後來清理查詢改成依 created_at 掃描（reports_contact_cap），
-- EXPLAIN 不選 resolved_at 的索引，fix3 的 migration 把它刪掉（附錄 D）。

-- 措施的來源案件（C20）：從案件建立的措施，任何依據都記下；複合外鍵保證是同一家店的案件。
ALTER TABLE seller.measures ADD COLUMN source_case_id UUID,
    ADD CONSTRAINT measures_source_case_fkey FOREIGN KEY (source_case_id, store_id) REFERENCES seller.cases (id, store_id);
CREATE INDEX measures_source_case ON seller.measures (source_case_id) WHERE source_case_id IS NOT NULL;

-- 補充（R7）：去重期間內同一位檢舉人的重複送出，內容補充到原通報；只新增，每筆通報最多 10 則（模組檢查，C14）。
-- matched_by 只在匿名的補充有值（Q3）。
CREATE TABLE seller.report_supplements (
    id          UUID PRIMARY KEY,
    report_id   UUID NOT NULL REFERENCES seller.reports (id),
    description TEXT NOT NULL,
    matched_by  TEXT,
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT report_supplements_report_key UNIQUE (id, report_id),
    CONSTRAINT report_supplements_description_check CHECK (char_length(description) BETWEEN 1 AND 2000),
    CONSTRAINT report_supplements_matched_by_check CHECK (matched_by IN ('PHONE', 'EMAIL', 'BOTH'))
);
CREATE INDEX report_supplements_report ON seller.report_supplements (report_id, created_at, id);
-- report_evidence：加 supplement_id（NULL 是原通報的佐證），FK (supplement_id, report_id) → report_supplements (id, report_id)；
--   （本節的 migration 先只放寬成 >= 0，沒有上界；`seller_report_evidence_position_bound` 補上 < 55；設計審查 #7 之後都被取代，見附錄 D：鍵改成
--   (report_id, submission_seq, position)，position 0..4、submission_seq 0..10，「每次送出最多 5 張」由 DB 保證；evidence_accesses 跟著換鍵。）

-- 移動紀錄（C6，append-only）：複合外鍵保證通報與兩個案件都是同一家店。
CREATE TABLE seller.report_case_moves (
    id           UUID PRIMARY KEY,
    report_id    UUID NOT NULL,
    store_id     BIGINT NOT NULL,
    from_case_id UUID NOT NULL,
    to_case_id   UUID NOT NULL,
    admin_id     BIGINT NOT NULL,
    moved_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT report_case_moves_report_fkey FOREIGN KEY (report_id, store_id) REFERENCES seller.reports (id, store_id),
    CONSTRAINT report_case_moves_from_fkey FOREIGN KEY (from_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT report_case_moves_to_fkey FOREIGN KEY (to_case_id, store_id) REFERENCES seller.cases (id, store_id),
    CONSTRAINT report_case_moves_distinct_check CHECK (from_case_id <> to_case_id),
    CONSTRAINT report_case_moves_admin_check CHECK (admin_id > 0)
);
CREATE INDEX report_case_moves_from ON seller.report_case_moves (from_case_id, moved_at, id);
CREATE INDEX report_case_moves_to ON seller.report_case_moves (to_case_id, moved_at, id);

-- 註記掛在案件或措施（剛好一個）；改嚴重度的改前改後記在那則註記上（C7），只有案件的註記可以有。
ALTER TABLE seller.governance_notes ALTER COLUMN measure_id DROP NOT NULL,
    ADD COLUMN case_id UUID REFERENCES seller.cases (id),
    ADD COLUMN severity_from TEXT, ADD COLUMN severity_to TEXT,
    ADD CONSTRAINT governance_notes_owner_check CHECK (num_nonnulls(case_id, measure_id) = 1),
    ADD CONSTRAINT governance_notes_severity_check CHECK (
        (severity_from IS NULL) = (severity_to IS NULL) AND
        (severity_from IS NULL OR (case_id IS NOT NULL AND severity_from <> severity_to
         AND severity_from IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL') AND severity_to IN ('LOW', 'MEDIUM', 'HIGH', 'CRITICAL'))));
CREATE INDEX governance_notes_case ON seller.governance_notes (case_id, created_at, note_id) WHERE case_id IS NOT NULL;

-- 轉報 v1（C18）：每個案件最多一筆；複合外鍵＋CHECK 讓它只能掛在食安案件。3c-b 加追蹤日期、主管機關的回覆與「不轉報」。
CREATE TABLE seller.case_referrals (
    case_id      UUID PRIMARY KEY,
    category     TEXT NOT NULL,
    channel      TEXT NOT NULL,
    reference_no TEXT,
    referred_at  TIMESTAMPTZ NOT NULL,
    admin_id     BIGINT NOT NULL,
    CONSTRAINT case_referrals_case_fkey FOREIGN KEY (case_id, category) REFERENCES seller.cases (id, category),
    CONSTRAINT case_referrals_category_check CHECK (category = 'FOOD_SAFETY'),
    CONSTRAINT case_referrals_channel_check CHECK (channel IN ('HOTLINE_1919', 'ONLINE_FORM', 'FAX_HEALTH_DEPT', 'IN_PERSON', 'OTHER')),
    CONSTRAINT case_referrals_reference_no_check CHECK (char_length(reference_no) BETWEEN 1 AND 100),
    CONSTRAINT case_referrals_admin_check CHECK (admin_id > 0)
);
-- report_cooperation_requests 不動（C22，3c-b 轉成詢問）。
```

- 嚴重度改用註記上的兩欄（C7）：v1 的獨立表 `case_severity_changes` 要再加 `(note_id, case_id)` 的 UNIQUE 與複合外鍵，才保證不會指到措施的註記；放在註記上，一個 CHECK 就保證，讀取也不必 join。
- `schema.sql` 裡 `seller.measures` 在 `seller.cases` 之前建立，所以 `measures_source_case_fkey` 寫在 `cases` 之後的 `ALTER TABLE`（同 `measures_current_revision_fkey` 的寫法）。

DB 保證不了、由模組保證並以資料集測試守的跨列不變量：
- I1 進行中的案件至少有一筆通報（開案與第一筆通報同一個 transaction；移走最後一筆時同一個 transaction 關閉）。
- I2 已結案的案件，每筆通報都有結果。
- I3 決定是「不採取措施」的案件，沒有通報的結果是「已採取措施」。
- I4 決定是「採取措施」的案件，決定當下有導出的措施（不論狀態）或「已採取措施」的成員。之後撤銷措施不改寫歷史。
- I5 每個進行中的案件，`decision_options` 至少有一個（C8 的兩條規則合起來推得）。
- I6 有導出措施或轉報紀錄的案件不會因移動而變空（C6）：空案件以 `NO_REPORTS` 關閉，轉報資料要有通報才能匯出。

### 2.3 開發資料的轉換（rb R2「開發資料」）

先檢查，任何一條成立就整個失敗並列出 ID（W2 先例：沒有誠實的轉換就不寫）：

| 檢查 | 原因 |
|---|---|
| R-a：`status = VERIFIED` 而 `category = OTHER` | 案件的類別必須是 Assessable，「已採取措施」又一定要在案件裡 |
| R-b：有 FDA 轉報，卻不會進任何案件的通報（`DISMISSED`，或 `PENDING` 而沒確認收件）。`reports_fda_check` 讓轉報只在食安類別（F26），所以 `OTHER` 不會有轉報 | 轉報掛在案件上 |
| R-c：會合併進同一個進行中案件的通報裡，有兩筆以上 FDA 轉報 | 一個案件一筆轉報 |
| R-d：`report_cooperation_requests` 掛在不會進任何案件的通報上（`DISMISSED`；`PENDING` 而沒確認收件；確認收件或調查中而 `category = OTHER`） | 3c-b 把配合要求轉成案件的詢問，這些列沒有去處 |

逐列轉換：

| 現在 | 轉換後 |
|---|---|
| `PENDING`，沒確認收件 | 待分案：`case_id`、結果都是 NULL |
| `PENDING` 已確認收件，或 `INVESTIGATING`，類別不是 `OTHER` | 進行中的案件，每個 `(store_id, category)` 一個：`severity` 取這幾筆中最嚴重的，`opened_at`／`opened_by` 取最早的確認收件（同時間取 id 小的），`updated_at` 取成員的 `updated_at` 最晚的；通報的 `assessed_category = category`、`triaged_at`／`triaged_by` 取自己的確認收件；`investigating_at` 不留（「調查中」拿掉） |
| `PENDING` 已確認收件，或 `INVESTIGATING`，類別是 `OTHER` | 待分案（確認收件與嚴重度不留：它們從沒判定過政策類別；這是丟掉值，不是寫錯的值，migration 測試斷言） |
| `VERIFIED` | 每筆一個已結案的案件：`DECIDED`、`ACTION_TAKEN`，`severity` 取自己的，開案取確認收件，`closed_at`／`closed_by` 取 `resolved_*`，`updated_at = resolved_at`，`decision_note = resolution_note`；通報的結果 `ACTION_TAKEN`，`resolved_*` 與說明保留 |
| `DISMISSED` | 不開案件；結果 `NO_VIOLATION`，`resolved_*` 與說明保留；確認收件與嚴重度不留 |
| 有 FDA 轉報（通過 R-b、R-c） | `case_referrals`：取它所在案件，`channel`、`reference_no`、`referred_at = fda_reported_at`、`admin_id = fda_reported_by_admin_id` |

- 案件的 ID 用 `uuidv7()`（F18）。所有通報 `version = 1`、`discarded_submissions = 0`；既有措施的 `source_case_id` 都是 NULL（以前沒有案件，導出關係由認定推導）。轉換不發事件。
- `finding_reports`、`evidence_accesses` 不動。
- 測試見 §7.4。

本機資料過不了檢查時（C2 的 `OTHER` 認定最可能：3b 的對話框提供 `OTHER`，F27；認定被措施與版本引用，不能單獨刪）：在本機 Postgres 重建單體的 `hawkr` database（`DROP DATABASE hawkr` 後 `CREATE DATABASE hawkr`；連線位址照 `AGENTS.local.md`：這台 Mac 要用非 loopback 的位址），再 `make migrate DATABASE_URL=…`、`make dev-seed`。這會清掉所有模組的本機資料；手動測試照 `backend-e2e-verify` skill 改用新的 `dev_<名稱>` 帳號。CI 的 DB 是新建的，不受影響。

### 2.4 查詢（sqlc）

`db/queries/reports.sql`（改）
- `InsertReport` 不變（`ON CONFLICT DO NOTHING`）；`DuplicateBuyerReport`、`DuplicateContactReport` 加 `outcome IS NULL`，後者回 `matched_by`（電話與 email 都相同是 `BOTH`，否則是相同的那一個）。
- `InsertReportSupplement`；`SupplementCount(report_id)`；`InsertReportEvidence` 帶 `submission_seq`、次內位置與 `supplement_id`（`NextEvidencePosition` 已刪，附錄 D #7）；`InsertReportSupplement` 帶 `seq`；`ReportSupplements(report_ids)`；`IncrementDiscardedSubmissions(report_id)`。
- `LockReport`、`LockReports(ids)`（`ORDER BY id FOR UPDATE`）、`ReportByID`、`ReportsByIDs`。
- `TriageReport`、`MoveReport`、`ResolveReport`（條件含 `version = @expected` 與狀態；0 列交給模組分辨是版本還是狀態）；`ResolveCaseReports(case_id)`（`outcome IS NULL` 的成員全部帶入，`RETURNING id, reporter_buyer_id`，發事件用）；`BumpReportVersion`。
- `AdminReports`（新到舊）與 `AdminReportsQueue`（舊到新：`state = AWAITING_TRIAGE` 或 `case_id`）：生效的類別是 `coalesce(assessed_category, category)`；店名與店家狀態 JOIN `seller.stores`；`supplement_count`、`latest_supplement_time` 用子查詢。
- `ReportFindingSources(report_ids)`：在 `finding_reports` 的 report id。
- `EraseExpiredContacts`：條件改成 `resolved_at IS NOT NULL AND created_at < @created_cutoff AND resolved_at < @resolved_cutoff`，`ORDER BY resolved_at`（原本 `:153` 是 `created_at`，改了才用得到新的部分索引）。
- `EvidenceAssets`、`BuyerReports`、`InsertEvidenceAccess` 不變。

`db/queries/cases.sql`（新）
- `InsertCase`、`CaseByID`、`CasesByIDs`、`LockCase`、`LockOpenCase(store_id, category)`、`CaseReportIDs(case_id)`、`BumpCase(id, now)`（`version + 1`、`updated_at = now`）。
- `DecideCase`、`CloseEmptyCase`（`WHERE closed_at IS NULL AND NOT EXISTS (成員)`）、`ChangeCaseSeverity`（都帶 `version = @expected`，`CloseEmptyCase` 例外：它在移動的 transaction 裡、案件列已鎖）。
- `AdminCases`（篩選 state、category、store；keyset `(opened_at, id)` 新到舊）；`CaseReportCounts(case_ids)`（成員數、沒有結果的、已採取措施的、最新補充時間）。
- **導出的措施**（C20 的定義）：措施 m 是案件 C 導出的 ⇔ `m.source_case_id = C`，或 `EXISTS (measure_findings mf JOIN finding_reports fr ON fr.violation_id = mf.violation_id JOIN reports r ON r.id = fr.report_id WHERE mf.measure_id = m.id AND r.case_id = C)`。`CaseMeasureCounts(case_ids)` 依它回每個案件的導出措施數與其中 `status = 'ENFORCED'` 的數目（`count(DISTINCT m.id)`）；§1.4 的 `ListMeasures` 篩選是同一個條件，在清單、待處理請求的收件匣、複審到期三支查詢各有一份，連同這一支共四處 SQL，由 §7.2 的資料集測試逐一鎖住（不拆成共用的 SQL 函式：見附錄 C）。
- `InsertReportCaseMove`、`CaseMoves(case_id)`（移入或移出）。
- `InsertCaseReferral`（`ON CONFLICT DO NOTHING`）、`CaseReferral`、`CaseReferrals(case_ids)`。

`db/queries/measures.sql`（改）
- `InsertMeasureNote`、`MeasureNotes` 改成 `InsertGovernanceNote(note_id, measure_id?, case_id?, admin_id, body, severity_from?, severity_to?, now)`，加 `CaseNotes(case_id, after_*)`（舊到新）。
- `InsertMeasure` 加 `source_case_id`。
- 三支措施清單加兩個條件：`source_case_id` 用上面的定義；`source_report_id` 是 `EXISTS (measure_findings mf JOIN finding_reports fr ON fr.violation_id = mf.violation_id WHERE mf.measure_id = m.id AND fr.report_id = @source_report_id)`。

`internal/devseed/devseed.go` 的 `clearChildrenSQL`：加 `report_case_moves`、`case_referrals`、`report_supplements`、`cases` 的刪除；`governance_notes` 的那一句加 `OR case_id IN (…)`；`measures` 的刪除放在 `cases` 之前（`source_case_id` 指向案件），`reports` 也在 `cases` 之前（`TestEverySellerTableHasASeedDecision` 會擋漏掉的表）。

---

## 3. 模組設計

### 3.1 model（`S/internal/model/governance.go` 改，`report_case.go` 新）

```go
// Outcome 是一筆通報的結果（rb R2）；值與 reports_outcome_check、proto 的 ReportOutcome 一致。
type Outcome string // ACTION_TAKEN、NO_VIOLATION、INSUFFICIENT、DUPLICATE、ABUSIVE
func OutcomeValues() []Outcome

// Decision 是案件的決定；DefaultOutcome 是結案時帶入沒有結果的通報的結果，API 的 DecisionOption.default_outcome 用它。
type Decision string // ACTION_TAKEN → ACTION_TAKEN；NO_ACTION → NO_VIOLATION
func (d Decision) DefaultOutcome() Outcome

type CaseCloseReason string // DECIDED、NO_REPORTS

// MaxSupplements 是一筆通報最多的補充則數。
const MaxSupplements = 10

type Report struct {
	… // 不變的欄位；Category 改成 policy.Category
	CaseID           *uuid.UUID
	AssessedCategory *policy.Category
	TriagedAt        *time.Time; TriagedByAdminID *int64
	Outcome          *Outcome
	ResolvedAt       *time.Time; ResolvedByAdminID *int64; ResolutionNote *string
	Version          int64
	StoreName        *string
	SupplementCount  int; LatestSupplementAt *time.Time; DiscardedSubmissions int
	Supplements      []Supplement // 單筆讀取
	Evidence         []Evidence   // 單筆讀取；Evidence 加 SupplementID *uuid.UUID
}

// State 是通報此刻的處理狀態（ReportState）；ListReports 的 state 篩選與它由資料集測試鎖住。
func (r Report) State() ReportState
// AwaitingTriage 是 3d 的「待分案」：沒有案件也沒有結果。開始時間是 CreatedAt。
func (r Report) AwaitingTriage() bool
// TriageDueAt 是分案的期限，只在待分案時有值；3c-a 只有檢舉人選了食安（建立後 FoodSafetyTriageSLA = 4 小時，原 FoodSafetyAckSLA），
// 3d 改由 ReviewPolicy 給所有類別。
func (r Report) TriageDueAt() *time.Time
// Supporting：結果沒有否定這筆通報（沒有結果、已採取措施或重複），可以當認定的來源、列進轉報資料、預填進建立措施（rb R2 的 3b 規則）。
func (r Report) Supporting() bool

type Case struct {
	ID uuid.UUID; StoreID int64; StoreName *string
	Category policy.Category; Severity Severity; Version int64
	OpenedAt time.Time; OpenedByAdminID int64
	ClosedAt *time.Time; ClosedByAdminID *int64; CloseReason *CaseCloseReason
	Decision *Decision; DecisionNote *string
	ReportCount, UnresolvedReportCount int
	LatestSupplementAt *time.Time
	Referral *FDAReferral
	Moves []ReportMove // 單筆讀取
	UpdatedAt time.Time
}
func (c Case) Open() bool
// Report.HasUnseenSupplements：還沒有結果，而且存在編號大於 SupplementsSeenSeq 的補充（案件的計數是這樣的成員數，附錄 D #8）。
// AwaitingDecision 是 3d 的「待決定」。3c-a：進行中；3c-b 加上「沒有還沒回覆的詢問」。
func (c Case) AwaitingDecision() bool
// DecisionClockStart 是「待決定」的開始時間。3c-a：OpenedAt；3c-b 改成 max(OpenedAt, 最後一筆詢問被回覆或撤回的時間)。
func (c Case) DecisionClockStart() time.Time

// CaseFacts 是判斷案件能做什麼需要的事實：同一個讀取快照，或指令裡鎖住之後讀。
type CaseFacts struct {
	Case                 Case
	StoreClosed          bool // 案件的店家（stint）已關閉
	ActionTakenReports   int  // 結果是已採取措施的成員數
	ResultingMeasures    int  // 導出的措施數，不論狀態（C20 的定義）
	UnrevokedMeasures    int  // 其中未撤銷的
}
// ActionTakenSupported：已經有作為可以說「已採取措施」——有導出的措施（已撤銷的也算），或已有成員的結果是已採取措施。
// 案件的「採取措施」與成員個別的「已採取措施」共用它（C8）。
func (f CaseFacts) ActionTakenSupported() bool
// AwaitingReferral 是 3d 的「待轉報」：食安、決定採取措施、還沒有轉報紀錄（3c-b 加上「沒有標記不轉報」）。開始時間是 ClosedAt。
func (f CaseFacts) AwaitingReferral() bool

// Availability 是案件每個動作不能做的原因，空的就是可以做。這是唯一的寫法：API 的 can_*、decision_options 與指令的前置檢查都只看它；
// 新增或改動條件時改這個函式，不另寫布林判斷（同 Measure.*UnavailableReasons 的規則）。
func (f CaseFacts) Availability() CaseAvailability
type CaseAvailability struct {
	Decide                                                               map[Decision][]CaseBlock
	ChangeSeverity, LinkMeasure, CreateMeasure, RecordReferral, Export []CaseBlock
}

// ReportFacts 是判斷通報能做什麼需要的事實；Case 是它所在的案件（沒有時 nil）。
type ReportFacts struct {
	Report        Report
	StoreClosed   bool
	Case          *CaseFacts
	FindingSource bool // 是某個認定的來源（finding_reports）
}
func (f ReportFacts) Availability() ReportAvailability
type ReportAvailability struct {
	Triage, Move, CiteAsSource []ReportBlock
	Resolve                    map[Outcome][]ReportBlock // 每個結果不能設的原因
}
```

規則表（`Availability` 的內容；原因依序列出，錯誤取第一個）：

| 動作 | 原因（依序） |
|---|---|
| 分案 | `RESOLVED`（已有結果）、`ALREADY_TRIAGED`（已在案件裡） |
| 設定結果（每個結果） | `RESOLVED`；`ACTION_TAKEN` 另有 `NEEDS_CASE`（還沒分案）、`NO_ACTION_BASIS`（案件的 `ActionTakenSupported()` 不成立） |
| 移動 | `RESOLVED`、`NOT_IN_OPEN_CASE`、`FINDING_SOURCE`、`LAST_REPORT_WITH_MEASURE`（它是案件唯一的成員，而案件有導出的措施）、`LAST_REPORT_WITH_REFERRAL`（它是案件唯一的成員，而案件已記錄轉報） |
| 當認定的來源 | `NEGATED`（結果是未違規、內容不足或濫報）、`STORE_CLOSED` |
| 結案：採取措施 | `CLOSED`、`NO_ACTION_BASIS`（沒有導出的措施，也沒有已採取措施的成員） |
| 結案：不採取措施 | `CLOSED`、`UNREVOKED_MEASURE_EXISTS`（有未撤銷的導出措施）、`REPORT_ACTION_TAKEN`（I3） |
| 改嚴重度、連結措施 | `CLOSED` |
| 建立措施的入口（警告、停權、風險） | `CLOSED`、`STORE_CLOSED` |
| 記錄轉報 | `NOT_FOOD_SAFETY`、`NO_REPORTS`（以 `NO_REPORTS` 關閉的案件）、`REFERRAL_RECORDED`（記過時指令回原紀錄，不是錯誤） |
| 匯出轉報資料 | `NOT_FOOD_SAFETY`、`NO_REPORTS` |

- **I5 成立的理由**：`ActionTakenSupported()` 成立時「採取措施」可選；不成立時沒有任何導出措施（所以也沒有未撤銷的），也沒有已採取措施的成員，「不採取措施」可選。審查 B1 的情境（措施 M → 成員設為已採取措施 → M 被撤銷）下，M 仍算導出的措施，所以「採取措施」可選，案件不會卡住。
- 「可以設定的結果」與「可以選的決定」的順序固定：已採取措施、未違規、內容不足、重複、濫報；採取措施、不採取措施。
- 「記錄轉報」不要求先決定：轉報可能先於決定發生（例如嚴重事件先通報主管機關），先記下的事實讓 3d 的「待轉報」不出現。
- **Q4 (ii) 的差異**：沒有 `source_case_id`，導出的措施只看認定推導；`NO_ACTION_BASIS`、`UNREVOKED_MEASURE_EXISTS` 從阻擋的原因改成提示（`Availability` 另回 `Warnings`，對話框顯示 #E3 的提示版），I4、I5 刪除，`LAST_REPORT_WITH_MEASURE` 保留。

### 3.2 指令：transaction 與鎖順序

全部在 `S/internal/governance`（`triage.go`、`resolve.go`、`move.go`、`case.go` 新檔；`report.go` 留送出與讀取）。鎖順序（C12）：seller 內的全序是 **identity 帳號列（需要時）→ 店家列（帳號全部的 stint，`stint.LockAccountOf`，`store_id` 升序）→ 通報列（id 升序）→ 案件列（id 升序）→ 措施列**。
- 這是 D3／D4 修訂 (c) 第 3 層「措施與通報在店家列之後」的細化：案件列排在通報列之後、措施列之前。現在沒有流程同時鎖案件與措施；之後若加案件改判（rb R12 的觸發條件），它會在案件之後連帶撤銷措施，這個順序已經對。
- 修訂說的「鎖」含會衝突的 `INSERT`：`InsertReport` 的去重衝突與 `InsertCase` 的 `cases_one_open` 都排在店家列之後。送出不鎖店家，但它在插入之前不持有任何鎖，只會和同一個去重鍵的送出衝突。
- 3c-a 的指令不鎖其他模組的列。之後要鎖 identity 帳號列時（例如 R11 的暫停受理），依規則 4 放在最前面：identity 帳號列是錨點（F22）。
- 和 `CreateMeasure`（帳號 stint → 來源通報 → 措施）同一個前綴，不會互鎖。
- 改變案件成員或通報結果的指令都先鎖店家，所以同一家店的分案、移動、結案彼此串行；`cases_one_open` 是保底。
- `RevokeMeasure` 與 `CreateMeasure` 也先鎖帳號的 stint，所以 C8 的措施條件在指令裡讀到的是確定的值，不必鎖措施列。
- 每個指令都在鎖住之後讀、再以 `Availability()` 判斷；先做無狀態的驗證（列舉的子集、字數、必填），結果不依賴時間。

| 指令 | transaction 內的步驟 | 寫入 | 事件 |
|---|---|---|---|
| `Triage` | 讀通報取 `store_id` → 鎖店家 → 鎖通報（比對版本）→ `LockOpenCase(store, 類別)`：有就驗「嚴重度不可設」並加入；沒有就驗「嚴重度必填」並 `InsertCase` → `Availability().Triage` | 通報 `case_id`、`assessed_category`、`triaged_*`、版本＋1；案件 `BumpCase`（新案件是 1） | — |
| `Resolve` | 讀通報 → 鎖店家 → 鎖通報（版本）→ 有案件就鎖案件並讀 `CaseMeasureCounts`、`CaseReportCounts`、店家狀態 → `Availability().Resolve[outcome]` | 通報結果、`resolved_*`、說明、版本＋1；有案件時 `BumpCase` | `ReportResolved` |
| `Move` | 讀通報 → 鎖店家 → 鎖通報（版本）→ 依 id 升序鎖原案件與目標的進行中案件（沒有就之後新建）→ `ReportFindingSources`、`CaseMeasureCounts`、`CaseReportCounts` → `Availability().Move`、類別不同 → 目標規則同分案 | 通報 `case_id`、`assessed_category`、版本＋1；`report_case_moves`；原案件 `BumpCase`，沒有成員時 `CloseEmptyCase`（`NO_REPORTS`，`closed_by` 是這位管理員）；目標案件 `BumpCase` | — |
| `DecideCase` | 讀案件 → 鎖店家 → `CaseReportIDs` → 鎖通報（升序）→ 鎖案件（版本）→ 讀 `CaseMeasureCounts`、成員結果 → `Availability().Decide[decision]` | 案件 `DECIDED`、決定、說明、`closed_*`、`BumpCase`；`ResolveCaseReports`（預設結果、說明、版本＋1） | 每筆帶入結果的通報各一個 `ReportResolved` |
| `ChangeCaseSeverity` | 讀案件 → 鎖店家 → 鎖案件（版本）→ `Availability().ChangeSeverity`、嚴重度不同 | 案件嚴重度、`BumpCase`；`governance_notes`（`case_id`、原因、`severity_from`、`severity_to`） | — |
| `RecordFDAReferral` | 讀案件 → 鎖店家 → 鎖案件 → `Availability().RecordReferral`；已有紀錄回原紀錄 | `case_referrals`；`BumpCase` | — |
| `CreateCaseNote` | 驗證 → 寫入（不鎖案件，案件不存在由外鍵擋下 → `not_found`） | `governance_notes` | — |
| `CreateMeasure`（改） | 照現行：先查 `request_id` 的重送，再 `lockTarget`（帳號的 stint）；帶 `source_case_id` 時接著讀案件：不存在或不是這家店 → `invalid_argument`（`SourceCaseID`）；案件已結案 → `failed_precondition`（#S31）。讀案件不必鎖：關閉案件的指令都先鎖同一組 stint | `measures.source_case_id` | 照現行（`MeasureCreated`） |

- `DecideCase` 先讀成員再鎖：讀之前已經鎖了店家，所有改成員的寫入者都先鎖店家，所以成員在讀與鎖之間不會變［推論，由 §7.5 的並行測試驗證］。補充不改成員，也不鎖案件。
- 版本與狀態都不符時先回版本衝突（3a 先例）。
- 建立措施的來源通報檢查（`create.go` 的 `lockSourceReports`）把 `status == DISMISSED` 換成 `!report.Supporting()`，句子改成模組文字（#S19）；同一家店的規則不變。

### 3.3 送出與補充（R7）

`Submit`（`report.go`）：
1. 無狀態驗證不變；類別改成 `ReporterSelectable`（匿名固定 `FOOD_SAFETY`）。
2. transaction 外：讀店家（`StoreByID`）；`in.AccountID > 0` 而 `store.AccountID == in.AccountID` → `invalid_argument`（`StoreID`，#D′2）。順序在「曾經公開過」（facade 的 `StoreCards`）之後，從沒公開過的店照舊是同一個 `not_found`。`SubmitReportInput.UploaderAccountID` 改名 `AccountID`（登入的呼叫者：上傳佐證與擋自己的店），匿名入口在呼叫者登入時也帶上。
3. **刪掉 transaction 外的重複檢查與提前返回**（`report.go:102-105`）：重複的送出現在要寫成補充，也要轉正佐證。
4. 佐證照舊在 transaction 外轉正。
5. transaction：`InsertReport`；
   - **寫入成功**：
     - 寫佐證、排入引用同步。
     - 發 `ReportReceived`；匿名時帶這次送出的 email（就是新通報存下的那一個）。
     - 回 `Report`。
   - **撞到去重**（只會是沒有結果的那一筆）：讀出並 `LockReport`。
     - 鎖住後它已經有結果（剛被結案）：重做一次 `InsertReport`（原通報已離開 partial UNIQUE），最多兩次。
     - 已有 `MaxSupplements`（10）則補充：
       - 登入：回 `failed_precondition`（#D′3）。已轉正的佐證沒有引用，由 media 的 GC 回收（沿用撞到重複時的先例）。
       - 匿名：內容不存，`discarded_submissions + 1`，發 `ReportReceived{Discarded: true}`（帶這次的 email），回應照舊。
     - 其餘：
       - 寫 `report_supplements`（匿名帶 `matched_by`）。
       - 佐證的 `submission_seq` 是這則補充的 `seq`（= 已有則數 + 1，通報已鎖住）、次內位置從 0 連號、帶 `supplement_id`，`mediaref.Changed`（ref 仍是通報 ID）。
       - 通報版本＋1。**不鎖、不改案件**（C11）。
       - 發 `ReportReceived{SupplementID}`（匿名帶這次的 email）。
       - 回 `SubmitReportResult{ReportID, Supplemented: true, Report}`：登入時 `Report` 是原本那一筆（去重的鍵含買家，一定是呼叫者自己的，回應用它，minor 2）；匿名時是 nil（CR-5 不變）。
6. 登入的 handler 依 Q2；匿名照舊回空的回應。

- **確認信不洩漏「這支電話用過」**（審查 M3）：新通報、補充、超過上限三種情形，都發一個 `ReportReceived`，帶這次送出的 email。A8 用同一個範本寄到這個地址，回應也相同，所以送出的人分辨不出是哪一種。
  - 這次送出的地址只在 in-process 的事件與 A8 在同一個 transaction 排入的 job 參數裡，seller 不落表。
  - A8 那一刀要設定：job 完成或丟棄後清掉參數裡的地址（已查證：River v0.47.0 預設保留完成的 job 24 小時、丟棄的 job 7 天，`rivershared/riversharedmaintenance/river_shared_maintenance.go:35-37`，專案沒有覆寫）；義務寫在計畫書 A8 與 `events.go` 的 `ReportReceived` 註解（附錄 D #10）。
  - 剩下的推論：結果信只寄給原通報存下的地址。以別人的電話送出的人，要等很久沒收到結果，才能很弱地推論，這一點接受；Q3 寫明。
- **上限 10 則**：
  - 補充只會在原通報的去重期間內出現（登入一個台北日、匿名一個台北 ISO 週）。登入者在那一天的額度最多 240 次，所以上限主要擋的是換 IP 的匿名送出。
  - 觸發：登入的檢舉人正當地碰到上限時，重新評估上限的值。
- 去重只比對沒有結果的通報：被判濫報的人，同一天可以再送一筆新的（送出額度照舊）。R11 的暫停受理上線時一起看。
- `SubmitReport` 是 A7-D5′ 之後 moderation 會呼叫的入口：
  - 依規則 3，它不開 `InReadSnapshot`（C26）。
  - 現在的版本要在 transaction 外呼叫（`report.go:94-96`，因為佐證在 transaction 外轉正）。
  - moderation 需要在自己的 transaction 內建立檢舉時，依規則 3 另給加入呼叫方 transaction 的入口（D3／D4 修訂 (b) 的形狀：例如 `SubmitReportInTx`，佐證由呼叫方先轉正或不帶，冪等由呼叫方的狀態轉移擁有）。它在 A7 的刀和它的消費者一起加，3c-a 不先做。

### 3.4 匿名聯絡方式的保留（C16）

- `model.ContactRetention`（90 天，從建立起算）保留，新增 `ContactRetentionAfterOutcome = 30 * 24 * time.Hour` 與 `ContactRetentionCap = 365 * 24 * time.Hour`。
- `EraseExpiredContacts` 兩條路（`cap_cutoff = now − 365 天`、`created_cutoff = now − 90 天`、`resolved_cutoff = now − 30 天`）：
  - 有結果的：`created_at < created_cutoff` 而且 `resolved_at < resolved_cutoff`。
  - 絕對上限（Will 2026-10-18，設計審查 #11，GDPR 5(1)(e) 儲存限制）：`created_at < cap_cutoff`，不論有沒有結果、案件有沒有決定；邊界是嚴格大於 365 天（剛好 365 天不清）。
  - 依 `created_at` 掃描（新增部分索引 `reports_contact_cap`）；其餘（分批、`SKIP LOCKED`、記 `contact_erased_at`）不變。
- 觀測：清理回報每筆清掉的通報是否**只靠上限**清的（`ContactErasure.Capped`；有結果而且結果規則也到期的不算）；大於 0 時記一筆 warn log（「匿名通報的聯絡方式滿 365 天，不論有沒有結果都強制清除」，帶 `capped`、`erased`）。沒有結果卻拖滿一年，代表處理流程卡住了，這是看得到的觀測點；3d 的待辦把逾期列出來是另一條路，兩者不互相依賴。
- 取代：原本的「沒有結果就不清、可能一直保留、出現滿 180 天仍沒有結果的匿名通報時再訂上限」整段（設計審查 #11：觸發條件沒有任何機制會讓人看到）。
- 文件：`SubmitFoodSafetyReport` 的註解、設計書 32 §3.1、§6.1 與 §9 的保留句、ADR `2026-10-18-seller-governance-consolidated` 的保留規則、seller `AGENTS.md` 的保留句、CARRYOVER 的清理列；隱私權政策沒有寫天數（F20），不用改。

### 3.5 事件（C17）

發法照現行：在指令的 `db.InTx` 內以 `events.Publish` 發出，型別定義在 `model`、由 seller `events.go` re-export，沒有 outbox。依單體規則 4 與 D3／D4 修訂 (a)，同 transaction 的訂閱者只做兩種事：
- 只寫自己的列、而且讀取時推導不出來的原子變更。
- 只排入 job、寫 inbox。

A8 的確認信與結果通知屬於後者：同一個 transaction 內 `jobs.Enqueue`，送達在 transaction 外（F22）。

```go
// ReportReceived（新增）在每次送出的 transaction 內發出：新的檢舉、補充（SupplementID）、補充已達上限而沒有存下的匿名送出（Discarded）。
// ConfirmationEmail 是這次送出留的 email（只有匿名，登入者是 nil，A8 用站內通知）：確認信寄到這裡。它只在事件與 A8 的 job 參數裡，
// 不帶原通報人的聯絡方式（設計書 32 §7「匿名通報人的聯絡方式不放進事件」改成這一句）。
type ReportReceived struct {
	OccurredAt time.Time; EventID uuid.UUID
	ReportID uuid.UUID; SupplementID *uuid.UUID; Discarded bool
	StoreID int64; ReporterBuyerID *int64; Category policy.Category
	ConfirmationEmail *string
}
// ReportResolved（沿用既有的型別，S/internal/model/governance.go:438-446）在通報有了結果的 transaction 內發出，每筆通報一次：
// 個別設定、直接結掉或案件結案帶入都算。Outcome 的型別從 ReportStatus 換成 Outcome，加 CaseID（沒有案件時 nil）。
type ReportResolved struct { OccurredAt time.Time; EventID, ReportID uuid.UUID; StoreID int64; ReporterBuyerID *int64; Outcome Outcome; CaseID *uuid.UUID }
```
- 刪 `ReportAcknowledged`、`SellerCooperationRequested`（`S/events.go:33-40` 的 re-export 一起改）。都沒有訂閱者（F21）。
- A8 寄結果時要讀匿名檢舉人存下的聯絡方式。那支讀取由 A8 的刀加，是跨模組入口，不開 `InReadSnapshot`（C26）。
- 案件不發事件。3d 若因 ADR D3 的比對改成事件投影，觸發點就是 §6 表裡的指令，由 3d 加。

### 3.6 讀取

- `Report(id)`（`GetReport`）、`Case(id)`（`GetCase`）、`AdminReports`、`AdminCases`、`FDAExport` 只由 `internal/httpapi/seller` 的 admin handler 呼叫，不是跨模組入口。
  - 它們照 `AdminMeasures`（`S/internal/measure/list.go:150`）自己開 `db.InReadSnapshot`：單體規則 3 要求多個語句組成一個結果時開快照，又規定會被別的模組呼叫的方法不得用它。
  - 讀取的本體是在 `db.Conn(ctx)` 上執行的函式，外層才開快照，所以 3d 的合併層可以在自己的快照裡重用本體（C26）。
- 讀取都在 seller 自己的 schema，不 join 任何 `pub_*`（規則 2）。
- 每頁固定的查詢數：
  - 清單本身。
  - `CasesByIDs`（成員的案件）。
  - `CaseMeasureCounts`。
  - `ReportFindingSources`。
  - `CaseReportCounts`。
  - 店家狀態跟著清單的 JOIN。
  
  `can_*` 與 `available_*` 在清單與單筆都給，由同一個 `Availability()`。
- `FDAExport(caseID)`：案件、`Supporting()` 的成員（含補充與佐證位置、不含聯絡方式）、店家（`stint.Load`，已關閉的也讀）；`Availability().Export` 不成立回 `failed_precondition`。
- 註記：`CaseNotes` 與 `MeasureNotes` 共用驗證 `model.NoteBody(body)`（從 `measure/notes.go:21` 抽出，句子進模組文字）。

### 3.7 伺服器句子（seller `internal/text`）

新檔的 `apperr` 不帶中文字面值（C24）；§5.4 (g) 的句子進 `zh-TW.json`，`make uitext` 生成 `text_gen.go`。`staff` 段只有 zh-TW；`default` 段是檢舉人看的（zh-TW，英文之後補）。admin 顯示伺服器的 `LocalizedMessage`。

---

## 4. httpapi（`H/seller`）

- `governance.go`：
  - `GovernanceModule` 介面：刪 `AcknowledgeReport`、`StartInvestigation`、`RequestCooperation`；加 `TriageReport`、`MoveReport`、`AdminCases`、`Case`、`DecideCase`、`ChangeCaseSeverity`、`CreateCaseNote`、`CaseNotes`；`RecordFDAReferral`、`FDAExport` 改收案件 ID。
  - `toReport`、`toCase` 依 §1.3；`decision_options` 的 `default_outcome` 取 `Decision.DefaultOutcome()`。
  - 清單與 `GetFDAExport` 的 `Report.contact` 一律不設（只有 `GetReport`）。
- 欄位對應（`rpc.FieldPaths`）：

| 方法 | 對應 |
|---|---|
| `ListReports` | `State`→`state`、`Category`→`category`、`StoreID`→`store_id`、`CaseID`→`case_id` |
| `TriageReport`、`MoveReport` | `AssessedCategory`→`assessed_category`、`Severity`→`severity`、`ExpectedVersion`→`etag` |
| `ResolveReport` | `Outcome`→`outcome`、`Note`→`note`、`ExpectedVersion`→`etag` |
| `ListCases` | `State`→`state`、`Category`→`category`、`StoreID`→`store_id` |
| `DecideCase` | `Decision`→`decision`、`Note`→`note`、`ExpectedVersion`→`etag` |
| `ChangeCaseSeverity` | `Severity`→`severity`、`Note`→`note`、`ExpectedVersion`→`etag` |
| `RecordFDAReferral` | `Channel`→`channel`、`ReferenceNo`→`reference_no` |
| `CreateCaseNote` | `Body`→`note.body` |
| `CreateMeasure`（`createMeasurePaths` 的共同部分） | 加 `SourceCaseID`→`source_case_id` |
| `ListMeasures` | 加 `SourceCaseID`→`source_case_id`、`SourceReportID`→`source_report_id` |

- page token 綁住篩選（`rpc.FilterScope`）：`ListReports` 綁 state、category、store、case；`ListCases` 綁 state、category、store；`ListMeasures` 加 source_case、source_report。
- 版本：`rpc.Etag(version)`、`rpc.RequiredVersionField`（沿用）。
- `governance_measures.go`：`CreateMeasure` 接 `source_case_id`（`rpc.ParseUUID`），`ListMeasures` 接兩個新篩選；`MeasureNote` 改名。
- `reports.go`：`CreateReport` 依 Q2；`SubmitFoodSafetyReport` 在呼叫者登入時把帳號帶進 `AccountID`；`toMyReport` 帶 `outcome`。
- `adminauth.go`：§1.9 的對應表（13 列）；啟動檢查會擋漏掉或多餘的宣告。
- 列舉一致性：`EnumMismatches` 加 `PolicyCategory`↔`policy.Values()`、`ReportOutcome`、`CaseDecision`、`CaseCloseReason`、`SupplementMatch`；推導的 `ReportState`、`CaseState` 以表格測試對 `Report.State()`、`Case.Open()`。

---

## 5. admin UI

版面、流程與文案都是新的，照根目錄規則要先取得同意：本節的頁面結構與 §5.4 的文案一次送審。重用 3a 措施頁的元件與 pattern：
- `measures/page.tsx` 的分頁籤與 keyset 分頁。
- `[id]/page.tsx` 的 `run()` 與 `DialogName`。
- `ScheduleReviewDialog.tsx` 的對話框骨架與 a11y hooks。
- `runMeasureAction` 的版本衝突重讀。
- `MeasureNotesSection.tsx`。

［量測：另一個 agent 的盤點］

### 5.1 資訊架構與檔案

| 路徑 | 內容 |
|---|---|
| `/admin/violation-reports` | 「商家通報」：分頁籤 待分案｜案件｜所有通報；每 30 秒自動更新（3cd D′5） |
| `/admin/violation-reports/[id]` | 通報詳情（3cd K1 的路由） |
| `/admin/violation-reports/cases/[id]` | 案件詳情（靜態段 `cases` 先於 `[id]`，通報 ID 是 UUID 不會撞） |

| 檔案 | 動作 |
|---|---|
| `A/violation-reports/page.tsx` | 整頁改寫（Connect `ListReports`、`ListCases`） |
| `A/violation-reports/[source]/[id]/page.tsx` | 刪除 |
| `A/violation-reports/report-list.ts` | 新：分頁籤 → 請求、狀態與欄位文字（純函式） |
| `A/violation-reports/report-text.ts` | 新：admin 單端表 `ReportOutcome`、`CaseDecision`、`ReportState`、`CaseState`＋`CaseCloseReason`、`SupplementMatch`、`FDAChannel`（`Record<Exclude<E, UNSPECIFIED>, string>`，不認得走 `staffTextOrUnknown`） |
| `A/violation-reports/report-actions.ts` | 新：驗證、請求組裝、失敗句（比照 `measure-actions.ts`；字數用 `@hawkr/api/text/server-text`） |
| `A/violation-reports/fda-export.ts` | 新：組轉報資料 `.md`（K2，client 組） |
| `A/violation-reports/[id]/page.tsx`、`TriageReportDialog.tsx`、`ResolveReportDialog.tsx`、`MoveReportDialog.tsx` | 新 |
| `A/violation-reports/cases/[id]/page.tsx`、`CaseSections.tsx`、`DecideCaseDialog.tsx`、`ChangeCaseSeverityDialog.tsx`、`RecordFDAReferralDialog.tsx` | 新 |
| `A/governance/NotesSection.tsx` | 新：參數化的註記區（list／create 由呼叫端給，標題 id 由呼叫端給，註記的標頭顯示 `severity_change`）；`MeasureNotesSection.tsx` 改成包它 |
| `W/admin/src/lib/policy-category.ts` | 新：`REPORTER_CATEGORIES`、`ASSESSABLE_CATEGORIES`（C3）與 `policyCategoryText`（`commonPolicyCategory`） |
| `W/admin/src/lib/deadline.ts` | 新：D′5 的期限文字（`剩 {時長}`／`已逾期 {時長}`、四分之一轉 warning、時長的捨去規則）；3d 沿用 |
| `A/merchants/[id]/CreateMeasureDialog.tsx` | `initialSourceReportIds` 換成 `initial?: { kind?, endTime?, statement?, category?, sourceReportIds? }`（`useState` 初始值；`request_id` 照舊在開啟時產生，改過就換新的）；加 `sourceCaseId?` |
| `A/merchants/[id]/CreateRiskMeasureDialog.tsx`、`CreateTerminationDialog.tsx` | 加 `sourceCaseId?`；`measure.ts` 的 `buildCreateRiskMeasure`、`buildCreateTermination` 與 `requestKey`、`terminationRequestKey` 帶上它；`violation-measure.ts` 的 `buildCreateViolationMeasure`、`violationMeasureRequestKey` 同 |
| `A/merchants/[id]/SourceReportsField.tsx` | 改看 `report.canCiteAsSource`（3b 修訂 6-2），刪 `ReportStatus` import；說明句改（#F3） |
| `A/merchants/[id]/violation-measure.ts` | 類別選項改用 `ASSESSABLE_CATEGORIES`；新增 `measureInitialFromSuggestion(rec, now)`（3b 修訂 6-1） |
| `A/invoices/complaints/[complaintId]/page.tsx` | 6-1：`:534` 欄標題、`:539-549` 開啟時帶初始值、`:147` 的 `measureStoreId` 改成 `{ storeId, initial? }`、`:575-579` 傳入、`:97-102` 註解 |
| `A/measures/[id]/MeasureSections.tsx`、`A/merchants/[id]/measure.ts` | 類別文字改用 `policyCategoryText` |
| `W/admin/src/components/Layout/Sidebar.tsx` | 「商家違規 / 調查」（`:38-40`，「內容治理」組）移到「賣家 / 商品」組、放在「賣家管理 / 審核」之後，改名「商家通報」 |
| `SlaCountdown.tsx:23`、`Invoice.css:178` | 註解裡的路徑 |

### 5.2 頁面與狀態

**商家通報（清單頁）**
- 分頁籤：
  - 待分案：`ListReports(state=AWAITING_TRIAGE)`，舊到新。欄位是店家、檢舉人選的類別、分案期限、建立時間；類別篩選用檢舉人可選的 12 類。
  - 案件：`ListCases(state)`，狀態篩選 全部｜進行中｜已結案。欄位是店家、類別、嚴重度、狀態（`unseen_supplement_report_count > 0` 時加標記 #A24）、通報數、開案時間；類別篩選用 Assessable 的 12 類。
  - 所有通報：`ListReports()`，新到舊。欄位是店家、類別（生效的類別）、狀態、案件、建立時間；類別篩選用全部 13 類。
- 換分頁籤或篩選回到第一頁（`changeTab` 同措施頁）。列連到詳情。
- 載入、讀取失敗加重試、兩種空狀態、分頁照措施頁。

**通報詳情**
- 載入、讀取失敗（#B24，加重試）。
- 區塊：通報內容、補充（有才顯示；匿名補充的標示 #B9；丟掉的次數 #B25）、檢舉人、佐證、處理（狀態、平台判定類別、案件連結、分案、結果、結案、給檢舉人的結果說明、分案期限）。
- 按鈕依 `can_*` 隱藏、不顯示原因（K4′）：分案（`can_triage`）、設定結果（`can_resolve`）、移到其他案件（`can_move`）、查看案件（有 `case_id`）。
- 分案與移動的對話框不再讀 `ListCases`：每個類別的去處（加入進行中的案件，或開新案件）來自 `Report.triage_targets`，送出時原樣放進請求的 `target`；案件頁的「移到其他案件」先 `GetReport` 取得去處再開對話框（附錄 D #1；#D22 的讀取中與失敗不再適用，案件頁讀不到這筆通報時顯示 A19 的「通報讀取失敗」）。

**案件詳情**
- 載入、讀取失敗（#C22，加重試）；`unseen_supplement_report_count > 0` 時顯示 #C23；案件裡的通報列有 `has_unseen_supplements` 的加 #C26。
- 區塊：
  - 案件：店家、類別、嚴重度、狀態、開案、結案、給檢舉人的結果說明。
  - 通報：`ListReports(case_id)` 舊到新，每列有設定結果、移動的入口與連結。
  - 移動紀錄。
  - 轉報主管機關（食安）。
  - 導出的措施：`ListMeasures(source_case_id)`，列「{種類}・{狀態}・措施 {id}」，上方是 #C24 的說明。
  - 內部註記：`NotesSection` 接 `CreateCaseNote`／`ListCaseNotes`；改嚴重度的註記標頭顯示 #C7。
- 按鈕：
  - 結案：`can_decide`。
  - 建立措施、建立風險措施：`can_create_measure`。
  - 終止合作：`can_link_measure`，而且 `GetStore.can_create_termination`。
  - 改嚴重度：`can_change_severity`。
  - 下載轉報資料：`can_export_fda`。
  - 記錄已轉報主管機關：`can_record_fda_referral`。
- 三個建立措施的對話框都帶 `sourceCaseId`，頂端顯示 #E22。
  - 終止合作的事由與說明來自 `SellerAdminService.GetStore`（同商家頁）：讀取中按鈕不出現，失敗時顯示 #C25 加重試。
  - 建立措施（違規依據）另帶 `initial.category`（案件的類別）與 `initial.sourceReportIds`。
  - `initial.sourceReportIds` 依序讀 `ListReports(case_id)`，取 `canCiteAsSource` 的通報，湊滿 20 筆或沒有下一頁為止，所以是最早的 20 筆（rb R2「選其中 20 筆」）。超過 20 筆時顯示 #E18。
- 結案對話框開啟時重讀 `GetCase`：讀取中與失敗照 #E20，讀完之前不能送出。
  - 選項只列 `decision_options`，帶入的結果名稱取它的 `default_outcome`（#E4）。
  - `unseen_supplement_report_count > 0` 時顯示 #C23（同一句，帶筆數），不擋送出。
- 設定結果對話框開啟時讀 `ListMeasures(source_report_id=這筆通報)`：讀取失敗顯示 #D23，仍可送出。
  - 選了未違規、內容不足或濫報，而且結果裡有狀態不是已撤銷的措施時，顯示 D′9 的提示（#D18）。
  - 這是並排呈現伺服器的關聯資料，不是推導規則。
- 成功時不另外提示（3cd #71）；只有「沒有寫入」（記錄轉報已記過）與「移動造成原案件關閉」顯示橫幅。版本衝突時重讀並顯示衝突句，對話框保留輸入（`runMeasureAction` 的做法）。

### 5.3 對話框

六個新的：分案、設定結果、移到其他案件、結案、改嚴重度、記錄已轉報主管機關。骨架照 `ScheduleReviewDialog.tsx`（`modal-overlay`、`useEscapeKey`／`useFocusTrap`／`useModalIsolation`、`aria-busy`）；全部加進 `admin-governance-modal-a11y.test.ts` 的 `GOVERNANCE_SOURCES`。
- 選項一律來自伺服器：設定結果只列 `available_outcomes`，結案只列 `decision_options`。
- 平台判定類別的選項是 `ASSESSABLE_CATEGORIES`，移動時排除目前的類別。
- 建立風險措施與終止合作沿用商家頁的兩個對話框，只加 `sourceCaseId`（它們已在 a11y 清單裡）。

### 5.4 文案表（一次核准）

來源欄的意思：
- **已核准**：3cd §4 的 #n（Will 2026-10-18 對 §1–§4 照建議）、v1 D6、v2 D6′、3b 修訂或 rb 的 R 題原文。
- **沿用**：3a、3b 已上線的原文。
- **目錄**：共用文案目錄。
- **新提案**：要你核准。

標「v2 改」「v2 新增」的列是這一版改的。依 rb §3，3cd §4 的通報頁約 60 句改成案件頁，能沿用的都標出來。

用語檢查：
- rb R2 的「已處置／處置／不處置」用了術語表的退場用語（「處置（指對賣家的平台措施）→ 措施」），所以結果與決定改成「已採取措施／採取措施／不採取措施」（#C1、#C2，新提案）。
- 給檢舉人看的伺服器句子用「檢舉」（#D′1–#D′3），後台用「通報」：見 Q6 與 (i)。

**(a) 側欄與清單頁**

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| A1 | 側欄項目（「賣家 / 商品」，「賣家管理 / 審核」之後） | 商家通報 | 已核准（3cd #1；v1 D6-1） |
| A2 | 頁面標題 | 商家通報 | 已核准（3cd #2） |
| A3 | 頁面說明 | 買家與公眾對店家的通報。先分案：同一家店、同一個類別的通報歸到同一個案件；再在案件裡決定是否採取措施。每 30 秒自動更新。 | 新提案（取代 3cd #3：沒有確認收件與調查了） |
| A4 | 分頁籤 | 待分案｜案件｜所有通報 | 新提案（「待分案」是 R3 的種類名） |
| A5 | 案件的狀態篩選 | 全部｜進行中｜已結案 | 已核准（rb §3 改寫的 3cd D′2；「等賣家」在 3c-b 加） |
| A6 | 類別篩選的第一個選項 | 所有類別 | 已核准（3cd #5） |
| A7 | 類別選項 | 政策分類的名稱（子集見 §5.2） | 目錄 |
| A8 | 待分案的欄位 | 店家｜檢舉人選的類別｜分案期限｜建立時間 | 新提案（3cd #26 已核准的是「通報人選的類別」，配合 Q6 改成「檢舉人選的類別」） |
| A9 | 分案期限：未逾期 | 剩 {時長}（剩下不到四分之一時 warning；絕對時間放在 title） | 已核准（3cd #9，D′5） |
| A10 | 分案期限：已逾期（danger） | 已逾期 {時長} | 已核准（3cd #10，D′5） |
| A11 | 沒有期限（3c-a 只有食安有期限） | - | 已核准（3cd #11） |
| A12 | 案件的欄位 | 店家｜類別｜嚴重度｜狀態｜通報數｜開案時間 | 新提案（前四個沿用 3cd #6 的欄名） |
| A13 | 案件的狀態 | 進行中｜已結案：採取措施｜已結案：不採取措施｜已結案：沒有通報 | 新提案（「沒有通報」是 R2 原文） |
| A14 | 所有通報的欄位 | 店家｜類別｜狀態｜案件｜建立時間 | 新提案（「類別」照 3cd D′10 是生效的類別） |
| A15 | 通報的狀態 | 待分案｜處理中｜{結果}（#C1） | 新提案（「處理中」同 R3 給檢舉人的用詞） |
| A16 | 案件欄 | 案件 {id}；沒有案件時 - | 新提案 |
| A17 | 店家欄沒有店名時 | 店家 ID | 沿用規則（3cd #113、§2 第 17 項） |
| A18 | 載入 | 載入中... | 沿用（`measures/page.tsx:64`） |
| A19 | 讀取失敗（網路層失敗時；Connect 錯誤照伺服器的句子）＋「重試」 | 通報讀取失敗｜案件讀取失敗 | 前者已核准（3cd #14），後者新提案 |
| A20 | 空狀態：待分案 | 沒有待分案的通報 | 新提案 |
| A21 | 空狀態：案件 | 尚無案件｜有篩選時：沒有符合條件的案件 | 新提案（句型同 3cd #15、#16） |
| A22 | 空狀態：所有通報 | 尚無通報｜有篩選時：沒有符合條件的通報 | 已核准（3cd #15、#16） |
| A23 | 分頁 | 上一頁｜第 {n} 頁｜下一頁 | 沿用 |
| A24 | 案件列的標記（`unseen_supplement_report_count > 0`） | 有新的補充 | 新提案（v2 新增，審查 M3） |

**(b) 通報詳情**

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| B1 | 返回鈕的 aria-label | 返回列表 | 沿用 |
| B2 | 載入 | 載入通報中... | 已核准（3cd #21） |
| B3 | 標題 | 通報詳情 | 已核准（3cd #23） |
| B4 | 副標 | 查看店家・通報 {id}・建立於 {時間} | 已核准（3cd #24） |
| B5 | 區塊 | 通報內容 | 已核准（3cd #25） |
| B6 | 欄位 | 店家｜檢舉人選的類別｜商品 ID｜訂單編號｜說明 | 已核准（3cd #26 的一部分；「通報人」改「檢舉人」配合 Q6） |
| B7 | 沒填的欄位 | - | 已核准（3cd #27） |
| B8 | 區塊與項目 | 補充｜補充 {n}・{時間} | 新提案 |
| B9 | 匿名補充的標示 | 以電話比對到時：以電話比對到這筆通報的匿名送出，送出者不一定是原檢舉人。以聯絡信箱比對到時：以聯絡信箱比對到這筆通報的匿名送出，送出者不一定是原檢舉人。兩者都相同時不顯示 | 新提案（v2 改：用 B12 的「聯絡信箱」；加 BOTH） |
| B10 | 區塊 | 檢舉人 | 已核准（3cd #28「通報人」，配合 Q6 改字） |
| B11 | 登入的檢舉人 | 買家 ID {id} | 已核准（3cd #29） |
| B12 | 匿名檢舉人 | 匿名｜電話｜聯絡信箱｜姓名 | 已核准（3cd #30） |
| B13 | 聯絡方式已清除 | 聯絡方式已於 {時間} 依保留期限清除 | 已核准（3cd #31，新規則下仍成立） |
| B14 | 區塊與項目 | 佐證｜佐證 {n}｜補充 {m} 的佐證 {n} | 前兩個已核准（3cd #32），第三個新提案 |
| B15 | 沒有佐證 | 沒有佐證 | 已核准（3cd #32a） |
| B16 | 區塊 | 處理 | 新提案（取代 3cd #33「處理紀錄」） |
| B17 | 欄位 | 狀態｜平台判定類別｜案件｜分案｜結果｜結案 | 「平台判定類別」已核准（D6′-3），其餘新提案 |
| B18 | 分案、結案的值 | {admin ID}・{時間} | 沿用（註記的格式；3cd #34） |
| B19 | 分案期限（只在待分案） | 分案期限：{日期時間}（剩 {時長}）；逾期時括號內是 danger「已逾期 {時長}」 | 格式已核准（D′5），「分案期限」新提案（取代 3cd #26a 的「確認期限」） |
| B20 | 欄位與空值 | 給檢舉人的結果說明｜沒有說明 | 已核准（3cd #35、#35a「給通報人的…」，配合 Q6 改字） |
| B21 | 按鈕 | 分案｜設定結果｜移到其他案件｜查看案件 | 新提案 |
| B22 | 不能操作時 | 依 `can_*` 隱藏按鈕，不顯示原因 | 已核准（3cd #46，K4′） |
| B23 | 版本衝突（重讀後） | 這筆通報剛被更新，頁面已重新載入；請確認最新狀態後再操作。 | 新提案（句型沿用 `merchants/[id]/measure.ts:601-602`） |
| B24 | 讀取失敗 | 網路層失敗時「通報讀取失敗」，Connect 錯誤照伺服器的句子；加「重試」 | 已核准（3cd #22）（v2 新增，審查 minor 10） |
| B25 | 補充已達上限而丟掉的匿名送出 | 另有 {n} 次重複送出因補充已達上限，內容沒有存下。 | 新提案（v2 新增，審查 M3） |

**(c) 案件詳情與名稱**

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| C1 | 結果的名稱（admin 單端表） | 已採取措施｜未違規｜內容不足｜重複｜濫報 | 新提案：「已處置」改成「已採取措施」（術語表），其餘四個是 R2 原文 |
| C2 | 決定的名稱 | 採取措施｜不採取措施 | 新提案（R2「處置／不處置」，同上理由） |
| C3 | 載入 | 載入案件中... | 新提案（句型同 B2） |
| C4 | 標題 | 案件詳情 | 新提案 |
| C5 | 副標 | 查看店家・案件 {id}・開案於 {時間} | 新提案（句型同 B4） |
| C6 | 區塊與欄位 | 案件｜店家｜類別｜嚴重度｜狀態｜開案｜結案｜給檢舉人的結果說明 | 新提案（欄名多數沿用 3cd #6、#35） |
| C7 | 改嚴重度的註記標頭 | 改嚴重度：{舊} → {新} | 新提案（v2 改：取代 v1 的「嚴重度紀錄」區塊，改記在註記上） |
| C8 | 區塊 | 通報（{n} 筆） | 新提案 |
| C9 | 通報的列 | 通報 {id}・{類別}・建立 {時間}｜{狀態}｜補充 {n} 則・最新 {時間} | 前段沿用（3b 修訂 2 的來源通報選項），其餘新提案（v2 改：加最新補充的時間） |
| C10 | 區塊與項目 | 移動紀錄｜通報 {id} 於 {時間} 從案件 {id} 移入（{admin ID}）｜通報 {id} 於 {時間} 移到案件 {id}（{admin ID}） | 新提案 |
| C11 | 以沒有通報關閉的案件 | 這個案件的通報都已移到其他案件，案件已自動關閉，不算一次決定。 | 新提案（R2） |
| C12 | 區塊（食安案件） | 轉報主管機關 | 新提案 |
| C13 | 還沒轉報 | 還沒有記錄轉報 | 新提案 |
| C14 | 已轉報 | 已於 {時間} 記錄轉報（{管道}，案號 {no}）；沒有案號時：已於 {時間} 記錄轉報（{管道}） | 已核准（3cd #39） |
| C15 | 區塊、列、空狀態 | 導出的措施｜{種類}・{狀態}・措施 {id}｜還沒有導出的措施 | 已核准（3cd #41–#43） |
| C16 | 註記區 | 照 `MeasureNotesSection.tsx:84-159` 原文 | 沿用（3cd #44） |
| C17 | 按鈕 | 結案｜建立措施｜建立風險措施｜終止合作｜改嚴重度 | 「結案」「建立措施」已核准（3cd #36）；「建立風險措施」「終止合作」沿用（`merchants/[id]/page.tsx:1026`、`:1037`）；「改嚴重度」新提案（v2 改：加兩個入口，C20） |
| C18 | 按鈕（食安） | 下載轉報資料（.md）｜記錄已轉報主管機關 | 已核准（3cd #37） |
| C19 | 下載失敗 | 轉報資料讀取失敗，請再試一次。 | 已核准（3cd #70） |
| C20 | 版本衝突 | 這個案件剛被更新，頁面已重新載入；請確認最新狀態後再操作。 | 新提案（同 B23） |
| C21 | 成功時 | 不另外提示；只有「沒有寫入」（#E15）與移動造成原案件關閉（#D13）顯示橫幅 | 已核准（3cd #71）＋新提案 |
| C22 | 讀取失敗 | 網路層失敗時「案件讀取失敗」，加「重試」 | 新提案（v2 新增，審查 minor 10；句型同 3cd #14） |
| C23 | 有還沒看過的補充（頁首提示與結案對話框） | 有 {n} 筆通報有還沒看過的補充。（n 取自 `unseen_supplement_report_count`） | 已核准（Will 2026-10-18，fix3；取代「這個案件在上一次處理之後有新的補充，結案前請先看過。」） |
| C26 | 案件裡的通報列：這筆通報有還沒看過的補充（`has_unseen_supplements`） | 新的補充 | 已核准（Will 2026-10-18，fix3） |
| C24 | 導出的措施的說明 | 包含從這個案件建立的措施，以及依據的認定引用了這個案件的通報的措施。 | 新提案（v2 新增，C20） |
| C25 | 店家資料讀取失敗（終止合作的入口） | 店家資料讀取失敗；加「重試」 | 新提案（v2 新增，C20） |

**(d) 分案、移動、設定結果**

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| D1 | 分案：標題 | 分案 | 新提案 |
| D2 | 分案：說明 | 平台判定類別決定通報歸到這家店的哪個案件（同一家店、同一個類別一個案件），也決定是否走食安的轉報流程；不會改掉檢舉人選的類別。 | 新提案（改寫 3cd #51） |
| D3 | 分案：欄位與提示 | 平台判定類別｜嚴重度；提示「檢舉人選的是 {類別}」 | 已核准（3cd #50「通報人選的是」，配合 Q6 改字） |
| D4 | 已有進行中的案件 | 這家店的「{類別}」已有進行中的案件（嚴重度：{嚴重度}，{n} 筆通報），分案後會加入那個案件。 | 新提案 |
| D5 | 沒有進行中的案件 | 這家店的「{類別}」沒有進行中的案件，分案後會開一個新案件，請選擇嚴重度。 | 新提案 |
| D6 | 驗證 | 請選擇平台判定類別。｜請選擇嚴重度。 | 已核准（3cd #52） |
| D7 | 分案：送出與失敗 | 確認分案｜分案失敗，通報沒有變動；請確認網路狀態後再試一次。 | 新提案（句型同 3cd #54） |
| D8 | 共用 | 送出中…｜取消｜請選擇 | 沿用 |
| D9 | 移動：標題 | 移到其他案件 | 新提案 |
| D10 | 移動：說明 | 改這筆通報的平台判定類別，通報會移到這家店那個類別的案件。原案件的通報都移走時，原案件會自動關閉，不算一次決定。 | 新提案 |
| D11 | 移動：欄位 | 新的平台判定類別（目前的類別不在選項裡）；有沒有進行中的案件照 D4、D5 | 新提案 |
| D12 | 移動：送出與失敗 | 確認移動｜移動失敗，通報沒有變動；請確認網路狀態後再試一次。 | 新提案 |
| D13 | 移動後原案件關閉（橫幅） | 原案件的通報都已移走，原案件已自動關閉。 | 新提案 |
| D14 | 設定結果：標題 | 設定結果 | 新提案 |
| D15 | 設定結果：說明 | 結果設定後不能修改。還沒分案的通報只能直接設為未違規、內容不足、重複或濫報。 | 新提案 |
| D16 | 在案件裡的通報 | 沒有個別設定結果的通報，會在案件結案時帶入案件的決定。 | 新提案 |
| D17 | 選了未違規、內容不足或濫報 | 結果是未違規、內容不足或濫報的通報，不能當新認定的來源。 | 新提案（改寫 3cd #57） |
| D18 | 同上，而且已是措施的來源 | 這筆通報已是措施 {id} 的依據來源。設為{結果}不會撤銷那個措施；如果它只依據這筆通報，請到措施詳情撤銷認定。 | 已核准（3cd #57a，D′9）；「判定不成立」換成「設為{結果}」（新提案）。v2：資料來源改成 `ListMeasures(source_report_id)`，文字不變 |
| D19 | 欄位 | 給檢舉人的結果說明（選填） | 已核准（3cd #56，配合 Q6 改字） |
| D20 | 驗證 | 請選擇結果。｜結果說明最多 2000 字。 | 前者新提案，後者已核准（3cd #58） |
| D21 | 設定結果：送出與失敗 | 確認設定｜設定結果失敗，通報沒有變動；請確認網路狀態後再試一次。 | 新提案 |
| D22 | 分案、移動讀這家店的案件 | 載入「載入中...」；失敗「案件讀取失敗」加「重試」；讀完之前不能送出 | 載入沿用，失敗同 A19（v2 新增，審查 minor 10） |
| D23 | 設定結果讀措施失敗 | 措施讀取失敗，無法確認這筆通報是不是措施的來源。加「重試」；仍可送出 | 新提案（v2 新增，審查 M2） |

**(e) 結案、改嚴重度、轉報、建立措施**

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| E1 | 結案：標題 | 結案 | 已核准（3cd #56） |
| E2 | 結案：選項 | 採取措施｜不採取措施（只列 `decision_options`） | 新提案（#C2） |
| E3 | 結案：規則說明 | 採取措施：要有從這個案件建立的措施（已撤銷的也算），或已有通報的結果是已採取措施。不採取措施：從這個案件建立的措施都已撤銷，而且沒有通報的結果是已採取措施。 | 新提案（v2 改：依 Q1 的新規則） |
| E4 | 結案：帶入說明 | 結案後不能重開。還沒有結果的 {n} 筆通報會帶入「{結果}」。（{結果} 取伺服器給的 `default_outcome`） | 新提案（v2 改：結果名稱由伺服器給，審查 minor 8） |
| E5 | 結案：欄位與說明 | 給檢舉人的結果說明（選填）；說明：會帶入還沒有結果的通報；已經有結果的通報不受影響。 | 欄名已核准（3cd #56），說明新提案 |
| E6 | 結案：驗證 | 請選擇結案結果。｜結果說明最多 2000 字。 | 已核准（3cd #58） |
| E7 | 結案：送出與失敗 | 確認結案｜結案失敗，案件沒有變動；請確認網路狀態後再試一次。 | 「確認結案」已核准（3cd #59）；失敗句的「通報」換成「案件」（新提案） |
| E8 | 改嚴重度：標題、欄位 | 改嚴重度｜嚴重度｜原因（內部註記） | 「嚴重度」已核准，其餘新提案 |
| E9 | 改嚴重度：placeholder 與說明 | 寫下為什麼改嚴重度（最多 2000 字）｜原因會寫成一則內部註記，只有後台看得到。 | 新提案（placeholder 句型同註記） |
| E10 | 改嚴重度：驗證 | 請選擇嚴重度。｜嚴重度和目前相同。｜請填寫原因，最多 2000 字。 | 第一句已核准（3cd #52），其餘新提案 |
| E11 | 改嚴重度：送出與失敗 | 確認修改｜改嚴重度失敗，案件沒有變動；請確認網路狀態後再試一次。 | 新提案 |
| E12 | 記錄轉報：標題、欄位、說明 | 記錄已轉報主管機關｜管道｜主管機關的案號（選填）｜轉報本身在系統外完成；這裡只記一次，記過之後不能修改。 | 已核准（3cd #60） |
| E13 | 記錄轉報：管道 | 1919 食安專線｜線上表單｜傳真衛生局｜親自送件｜其他 | 已核准（3cd #61） |
| E14 | 記錄轉報：驗證 | 請選擇管道。｜案號最多 100 字。 | 已核准（3cd #62） |
| E15 | 已經記錄過（橫幅） | 這個案件先前已記錄轉報，以下是原本的紀錄，這次輸入的內容沒有寫入。 | 已核准（3cd #63）；「這筆通報」換成「這個案件」（新提案） |
| E16 | 記錄轉報：失敗 | 記錄轉報失敗，案件沒有變動；請確認網路狀態後再試一次。 | 已核准（3cd #64）；「通報」換成「案件」（新提案） |
| E17 | 建立措施：預填 | 類別＝案件的類別；來源通報＝案件裡可以當來源的通報，最早的 20 筆 | 已核准（3cd §2 第 14 項；R2） |
| E18 | 建立措施：超過 20 筆 | 這個案件超過 20 筆可以當來源的通報，已預先勾選最早的 20 筆。 | 新提案 |
| E19 | 建立措施：其餘 | 照 `CreateMeasureDialog.tsx` 原文，成功「已建立措施。」 | 沿用（3b 修訂 2） |
| E20 | 結案對話框開啟時重讀案件 | 載入「載入中...」；失敗「案件讀取失敗」加「重試」；讀完之前不能送出 | 載入沿用，失敗同 A19（v2 新增） |
| E21 | 結案對話框裡的新補充提示 | 同 #C23，不擋送出 | 新提案（v2 新增，審查 M3） |
| E22 | 從案件開啟的三個建立措施對話框（頂端） | 這個措施會記為案件 {id} 導出的措施。 | 新提案（v2 新增，C20） |
| E23 | 建立風險措施、終止合作的其餘文字 | 照 `CreateRiskMeasureDialog.tsx`、`CreateTerminationDialog.tsx` 原文 | 沿用（v2 新增） |

**(f) 建立措施的來源通報、發票申訴頁（3b 修訂 6）**

| # | 位置 | zh-TW | 來源 |
|---|---|---|---|
| F1 | 來源通報的選項 | 通報 {id}・{類別}・建立 {時間} | 沿用（3b 修訂 2） |
| F2 | 來源通報的空狀態、載入 | 這家店沒有可以當來源的通報。｜載入通報中... | 沿用（3b 修訂 2） |
| F3 | 來源通報清單的說明 | 選填；最多 20 筆，只列這家店可以當來源的通報（結果不是未違規、內容不足或濫報）。 | 新提案（取代「只列這家店還沒判定不成立的通報」；規則改了） |
| F4 | 建議卡的欄標題 | 建議理由 | 已核准（3cd #72） |
| F5 | 預填的種類 | 停權 | 目錄（3cd #73） |
| F6 | 預填的給賣家的理由 | 建議的 `suggestedReason` 原文 | 伺服器（3cd #74） |
| F7 | 沒有建議天數時 | 不預填結束時間，送出時由「請填寫結束時間。」擋下 | 沿用（3cd #75） |

**(g) 模組文字（seller `internal/text`）**

`staff` 段（後台看的；admin 照原樣顯示）：

| # | key | zh-TW | 來源 |
|---|---|---|---|
| S1 | `report.not_found` | 找不到這筆通報。 | 沿用（`report.go:23` 的字面值搬家，加句號） |
| S2 | `case.not_found` | 找不到這個案件。（建立措施帶的案件不存在時也用它） | 新提案 |
| S3 | `triage.category_required` | 請選擇平台判定類別。 | 已核准（3cd #82） |
| S4 | `triage.category_not_assessable` | 「其他」不能當平台判定類別；請改判成有依據的類別，或直接設定結果。 | 新提案（R10 的分界） |
| S5 | `triage.severity_required` | 開新案件要選嚴重度。 | 新提案 |
| S6 | `triage.severity_with_open_case` | 這家店的這個類別已有進行中的案件，嚴重度以案件為準；要改請到案件改嚴重度。 | 新提案 |
| S7 | `report.unavailable.resolved` | 這筆通報已經有結果。 | 新提案 |
| S8 | `report.unavailable.already_triaged` | 這筆通報已經分案。 | 新提案 |
| S9 | `report.unavailable.needs_case` | 還沒分案的通報不能設為已採取措施；請先分案。 | 新提案 |
| S10 | `report.unavailable.no_action_basis` | 案件還沒有從它建立的措施，不能設為已採取措施。 | 新提案（v2 改：依 Q1 的新規則，已撤銷的也算） |
| S11 | `report.unavailable.not_in_open_case` | 只有在進行中的案件裡、還沒有結果的通報可以移動。 | 新提案 |
| S12 | `report.unavailable.finding_source` | 這筆通報已是認定的依據來源，不能移到其他案件。 | 新提案（R2） |
| S13 | `move.same_category` | 新的平台判定類別和目前相同。 | 新提案 |
| S14 | `resolve.outcome_required` | 請選擇結果。 | 新提案 |
| S15 | `resolve.note_length` | 結果說明最多 2000 字。 | 已核准（3cd #58、#84） |
| S16 | `case.unavailable.closed` | 案件已結案。 | 新提案 |
| S17 | `case.unavailable.no_action_basis` | 還沒有從這個案件建立的措施，也沒有通報的結果是已採取措施，不能決定採取措施。 | 新提案（v2 改：依 Q1 的新規則） |
| S18 | `case.unavailable.unrevoked_measure_exists` | 從這個案件建立的措施還沒有撤銷，不能決定不採取措施；請先撤銷措施。 | 新提案（Q1） |
| S19 | `source_report.negated` | 結果是未違規、內容不足或濫報的通報，不能當認定的來源。 | 新提案（取代 `create.go:578` 的「不成立的通報不能成立違規」） |
| S20 | `case.unavailable.report_action_taken` | 案件裡有通報的結果是已採取措施，不能決定不採取措施。 | 新提案（I3） |
| S21 | `decide.decision_required` | 請選擇結案結果。 | 已核准（3cd #58） |
| S22 | `severity.required`、`severity.unchanged` | 請選擇嚴重度。｜嚴重度和目前相同。 | 前者已核准（3cd #52），後者新提案 |
| S23 | `severity.note_length` | 請填寫改嚴重度的原因，最多 2000 字。 | 新提案 |
| S24 | `referral.not_food_safety`、`export.not_food_safety` | 平台判定類別是食安的案件才能轉報主管機關。｜平台判定類別是食安的案件才能匯出轉報資料。 | 已核准（3cd #80、#81）；「通報」換成「案件」（新提案） |
| S25 | `referral.no_reports`、`export.no_reports` | 這個案件沒有通報，不能轉報。｜這個案件沒有通報，不能匯出轉報資料。 | 新提案 |
| S26 | `referral.channel_required`、`referral.reference_no_length` | 請選擇管道。｜案號最多 100 字。 | 已核准（3cd #62、#85） |
| S27 | `note.body_length` | 內部註記必須是 1 到 2000 字。 | 沿用（現行 `model.Required` 組出的句子，加句號；措施與案件的註記共用） |
| S28 | `list.state_invalid`、`list.category_invalid` | 狀態不合法。｜類別不合法。 | 新提案（取代 `report.go:405`、`:411` 的字面值） |
| S29 | `report.unavailable.last_report_with_measure` | 這個案件已有導出的措施，不能把最後一筆通報移走。 | 新提案（v2 新增，C6） |
| S30 | `measure.source_case_other_store` | 案件不是這家店的。 | 新提案（v2 新增，C20） |
| S31 | `measure.source_case_closed` | 案件已結案，不能再從它建立措施。 | 新提案（v2 新增，C20） |
| S32 | `report.unavailable.store_closed` | 店家已關閉，不能再引用它的通報建立警告或停權。（只用在 API 的原因，`CreateMeasure` 照舊回「店家已關閉」） | 新提案（v2 新增，審查 minor 11） |

`default` 段（給檢舉人看的；用詞依 Q6）：

| # | key | zh-TW | 來源 |
|---|---|---|---|
| D′1 | `report.duplicate_supplemented` | 你今天已經檢舉過這家店的這個類別，這次的內容已經補充到原本的檢舉。 | rb R7 已核准的句子把「通報」改成「檢舉」（新提案，v2 改，Q6） |
| D′2 | `report.own_store` | 不能檢舉自己的店。 | 新提案（v2 改，Q6） |
| D′3 | `report.supplement_limit` | `{max, plural, other {這筆檢舉已經補充 {max} 次，不能再補充。}}`（`max` 是 `MaxSupplements`） | 新提案（v2 新增，審查 M3） |

送出的其他驗證句（`report.go:26-84`，例如「通報類別不合法」「匿名通報必須留下電話或 email」）沒有改，留到 I-server（以檔案為單位的 ratchet 只要求不增加）；I-server 改寫它們時照 Q6 的用詞。

**(h) 轉報資料（`.md`，client 組；rb §3 照 3cd D′8，改成以案件為單位）**

- `# 食安通報轉報資料（案件 {id}）`（3cd 已核准的標題，「通報」換成「案件」：新提案）
- `## 1. 時間`：開案時間｜結案時間（還沒結案時寫「還沒結案」）（台北時間）（新提案）
- `## 2. 被通報的店家`：店名｜店號｜聯絡人｜聯絡電話｜地址｜食品業者登錄字號（沒有時寫「無」）（已核准）
- `## 3. 通報（{n} 筆）`，每筆 `### 通報 {k}（{通報時間}）`：
  - 商品 ID｜訂單編號（沒有提供時寫「通報人沒有提供」，已核准）。
  - 內容。
  - 補充（`- {時間}：{內容}`，新提案）。
  - 佐證「{n} 份，另附」或「無」（已核准）。
- 結尾：`本檔由後台於 {時間} 產生，供轉報主管機關使用。`（已核准）
- 只列結果不是未違規、內容不足或濫報的通報；不放檢舉人的聯絡方式（3cd D′8 已核准；同意之後再放，跟著 10f 的表單）。

這份是給主管機關的後台文件，照 3cd 已核准的「通報」用詞，不跟 Q6 改。

**(i) 政策分類的名稱（目錄）與術語表**

目錄 `common.PolicyCategory.*` 的 zh-TW 用 rb R10 的名稱（已核准），只有一個改字：
- 食品安全｜違法或禁售商品｜未經許可販售｜不實或誤導資訊｜假認證｜仿冒商品｜侵害智慧財產權｜詐欺或操縱交易｜繞過平台交易｜**不當使用個人資料**｜冒用或誤導交易主體｜干擾或濫用平台｜其他。
- 「個人資料」改成「不當使用個人資料」：「個人資料」不是違規的名稱（新提案，v2 改，審查 minor 13）。
- 取代舊的 7 個名稱（食安、假認證、誤導資訊、仿冒、未授權販售、詐騙、其他），賣家端的措施依據跟著改字。說明欄由 proto 註解帶入。

術語表「治理」加政策分類的 13 列（先加術語表，再進目錄）。英文是新提案，`en` 發佈前由你審：

| zh-TW | en（新提案） | 定義 | 使用處 |
|---|---|---|---|
| 食品安全 | food safety | 有健康風險或疑似食物中毒，或未揭露安全疑慮的商品（條款第十一條、16.1） | 檢舉類別、平台判定類別、認定 |
| 違法或禁售商品 | illegal or prohibited goods | 違法、違禁或來源不明，而沒有健康風險的商品（16.1） | 同上 |
| 未經許可販售 | unlicensed sale | 依法應有的登錄、許可或資格沒有完成就販售（11.2） | 同上 |
| 不實或誤導資訊 | false or misleading information | 虛假資料、標示不實或誤導的商品或店家資訊（16.2） | 同上 |
| 假認證 | fake certification | 不實的認證、檢驗或標章（16.2） | 同上 |
| 仿冒商品 | counterfeit goods | 冒用他人商標或外觀的商品本身（16.1） | 同上 |
| 侵害智慧財產權 | intellectual property infringement | 侵害他人商標、著作權、專利或其他權利的使用；仿冒商品本身是「仿冒商品」（16.5） | 同上 |
| 詐欺或操縱交易 | fraud or transaction manipulation | 偽造交易、洗錢、詐欺、濫用優惠、操縱評價（16.2） | 同上 |
| 繞過平台交易 | off-platform transactions | 引導買家繞過平台付款或規避平台服務費（16.3） | 同上 |
| 不當使用個人資料 | misuse of personal data | 未經授權蒐集、利用、揭露或出售買家個人資料（16.4） | 同上（v2 改） |
| 冒用或誤導交易主體 | impersonation | 冒用平台名義、未經授權的保證，或讓買家誤認交易主體（16.8） | 同上 |
| 干擾或濫用平台 | platform abuse | 干擾或破壞平台與資訊安全、以爬蟲或自動化不當取得資料（16.6、16.7）；檢舉人不能選 | 平台判定類別、認定 |
| 其他 | other | 檢舉人無法歸類時選；分案時要改判成有依據的類別或直接設定結果 | 檢舉類別 |

依 Q6 的建議再加 3 列（v2 新增；術語表 :7 說 admin 用同一套中文詞，所以「通報」要寫明是後台的受眾）：

| zh-TW | en（新提案） | 定義 | 使用處 |
|---|---|---|---|
| 檢舉 | report | 買家或公眾向平台指出店家、商品或內容違反規範；給買家、賣家看的按鈕、表單、通知與伺服器句子用這個詞 | 10d D4 的「⋯ → 檢舉」、使用者政策 §十五、「我的檢舉」、通知 |
| 通報 | report（後台） | 後台對一筆檢舉的稱呼：一位檢舉人的一次送出；只用在 admin（`staff` 受眾） | admin「商家通報」、案件頁 |
| 補充 | additional information | 同一位檢舉人在去重期間內再次送出、併入原本那一筆的內容 | 伺服器句子、admin、「我的檢舉」 |

結果（#C1）與決定（#C2）的詞在 3c-a 只出現在 admin；10f 做「我的檢舉」、A8 寫通知時進術語表與目錄（3cd §2 第 4 項的先例）。

**(j) 會刪掉的字串**：舊通報兩頁約 60 處，整頁刪除（清單見 `step10e-3cd-inventory.md` §2.3），不另外改文。

**數字**（以上表格的列，人工數；一列有多句時算一列）：
- 只含已核准的：41 列（其中 6 列只是配合 Q6 把「通報人」改成「檢舉人」）。
- 只含沿用的：14 列。
- 目錄與伺服器：3 列，另有政策分類的名稱 1 組。
- **含新提案的：108 列**：清單 12、通報詳情 10、案件詳情 21、對話框 34、來源通報的說明 1、模組文字 27、轉報資料 3。其中 19 列只是把已核准的句子改幾個字。
- 術語表新增 16 列（英文 16 個詞都是新提案）。
- 這一版改或新增的列：29 列，表上標了「v2 改」「v2 新增」。

### 5.4a 文案核准紀錄（Will 2026-10-18，第二輪）

3c-a-4 與設計審查之後，Will 核准下列文案（上面表格裡標「新提案」的對應列即為已核准）：

| 項目 | 文案 | 對應 |
|---|---|---|
| 伺服器句子（員工受眾） | `report.version_conflict`「這筆通報剛被更新，請重新讀取後再送出。」｜`case.version_conflict`「這個案件剛被更新，請重新讀取後再送出。」｜`case.unavailable.store_closed`「店家已關閉，不能再從這個案件建立警告、停權或風險措施。」｜`case.unavailable.referral_recorded`「這個案件先前已記錄轉報。」｜`report.unavailable.last_report_with_referral`「這個案件已記錄轉報，不能把最後一筆通報移走。」 | S 系列 |
| 伺服器句子（檢舉人受眾） | `report.submit_conflict`「送出時發生衝突，請稍後再試一次。」；`httpapi/seller/reports.go` 的「檢舉太頻繁，請稍後再試」（取代「通報太頻繁」） | 附錄 C 的「留給文案審核」 |
| admin 欄位標籤 | 設定結果下拉的「結果」；結案下拉的「結案結果」 | D 系列、E 系列 |
| admin aria-label | 清單篩選的「類別」「狀態」 | A 系列 |
| `.md` 空值 | 店家欄位空值寫「無」 | rb 修訂 13（延伸登記號碼規則） |
| E12 送出鍵（改） | 記錄轉報對話框的送出鍵是「記錄轉報」（標題維持「記錄已轉報主管機關」） | E12 |
| `triage.target_required`（伺服器，員工受眾） | 「請選擇加入的案件，或開新案件。」（請求沒有指定去處時的欄位錯誤；client 依 `triage_targets` 組請求，正常不會出現） | 附錄 D #1（Will 2026-10-18 第三輪核准） |
| 案件頁「導出的措施」讀取失敗 | 「措施讀取失敗」（加重試） | C 系列（Will 2026-10-18 第三輪核准） |
| `.md` 轉報資料的結構與標籤 | 以 `fda-export.ts` 為準：標題與開頭句；`## 1. 時間`、`## 2. 被通報的店家`、`## 3. 通報（N 筆）`；每筆 `### 通報 i（時間）`；「商品 ID」「訂單編號」沒填寫「通報人沒有提供」；「內容：」「補充：」「佐證：」；空值「無」；案件沒有結案寫「還沒結案」 | rb 修訂 13（Will 2026-10-18 第三輪核准） |
| C23 頁首提示與結案對話框（改） | 「有 {n} 筆通報有還沒看過的補充。」 | C23（Will 2026-10-18 第三輪核准） |
| C26 案件裡通報列的標記 | 「新的補充」 | C26（Will 2026-10-18 第三輪核准） |
| C27 分案或移動要移入的案件不成立（`target_case`；同時是伺服器句子 `case.target_version_conflict`） | 「要移入的案件剛被更新，請重新選擇。」 | C27（Will 2026-10-18 核准最後一輪審查 L1 的提案文案；取代原本分案、移動的案件衝突共用 C20 的「這個案件剛被更新…」，它讀起來像畫面上的案件） |
| C28 分案或移動要開新案件，而這個類別剛有進行中的案件（`category_case_opened`；同時是伺服器句子 `case.category_case_opened`） | 「這個類別剛有進行中的案件，請重新選擇。」 | C28（Will 2026-10-18 核准最終審查 Part 2 的提案；取代這個情形原本共用 C27 的「要移入的案件剛被更新…」：管理員沒看過那個案件，動詞也不是「移入」） |

審查指出的字串，對照核准表的結論：
- 「管道還沒選」（`buildReferral` 的內部例外）：不是新文案——送出前 `validateReferral` 已用 E14／S26 核准的「請選擇管道。」擋下；萬一漏了，例外訊息也改成同一句。
- 「措施讀取失敗」、`.md` 轉報資料的標籤、`triage.target_required`：第三輪（fix3）Will 核准，列在上表。
- 沒有仍待核准的句子。

### 5.5 發票頁的預填（3b 修訂 6-1）

`measureInitialFromSuggestion(rec, now)`：
- 種類＝停權。
- 結束時間＝`now + suggestedSuspendDays × 24h`；天數是空字串、0 或不是正整數時不預填，由「請填寫結束時間。」擋下。
- 理由＝`suggestedReason`，空字串時不預填。
- 類別不預填（`UNSPECIFIED`，由管理員選）。

只有「依建議建立措施」帶預填，常駐的「建立措施」不帶。`invoice-adm-contract.test.ts:1141`（這頁的治理方法只有 `createMeasure`）不受影響［量測］。

---

## 6. 3d 會用到的

3c-a 交判定式、開始時間與索引；三支來源查詢與成員的資料集測試在 3d（Q5 的建議）。
- ADR D3 已定（F22）：各來源都擁有全部篩選與排序鍵時，在 httpapi 做 keyset composition（單體規則 2）。這就是 3cd D′1；A7-D8 也選了同一個做法（A7 計畫:71）。
- 來源查詢要和 3d 建立的這些東西一起寫：放在 platform 的全序比較函式、kind 的整數 rank、`FilterScope` 與 N＋1 的取法（3cd D′1 的不變式）。所以放在 3d，不必在 3c-a 先做一半。
- 3c-a 的清單篩選（`ListReports(state=AWAITING_TRIAGE)`、`ListCases(state=OPEN)`）已經用同一個 Go 判定式，並有自己的資料集測試（§7.2）。

| 種類（rb R3） | 對象 | 成員（Go 判定式；3d 寫同義的 SQL 並以資料集測試鎖住） | 開始時間 | 3c-a 提供的存取路徑 | key 在待辦期間是否不變 |
|---|---|---|---|---|---|
| 待分案 | 通報 | `Report.AwaitingTriage()` | `CreatedAt` | `reports_awaiting_triage` | 不變 |
| 待決定 | 案件 | `Case.AwaitingDecision()`（3c-b 加詢問的條件） | `Case.DecisionClockStart()` | `cases_open` | 開始時間只在 3c-b 的詢問回覆或撤回時重新起算；改嚴重度改變期限（rb R3 允許的例外） |
| 待轉報 | 案件 | `CaseFacts.AwaitingReferral()`（3c-b 加「不轉報」） | `ClosedAt` | `cases_decided_food_safety` 加 `NOT EXISTS case_referrals` | 不變 |

- 期限輸出：3c-a 只有 `Report.triage_due_time`（食安 4 小時）。3d 加 `ReviewPolicy`，都只在義務還在時輸出（3cd §2 第 12 項）：
  - `triage_due_time` 擴到所有類別（72 小時）。
  - 新增 `Case.decide_due_time`：依嚴重度，嚴重 24 小時、高 3 天、中 7 天、低 14 天（rb R3 已核准的起始值）。
  - 新增 `Case.fda_referral_due_time`：決定後 24 小時。
- `ReviewQueueItem.subject` 加 `report_id`、`case_id`（3c-b 再加 `inquiry_id`、`authority_order_id`）；連結到 §5.1 的兩個詳情頁。
- 期限的顯示用 `admin/src/lib/deadline.ts`（3c-a 建立）。
- **期限的開始時間必須由伺服器輸出**（設計審查 #2，Will 2026-10-18 記錄）：`deadline.ts` 的色階（剩下不到整段期限的四分之一轉 warning）需要開始時間。3c-a 的分案期限用通報的 `create_time`，client 拿得到；但 3d 的 `Case.decide_due_time` 開始時間是 `DecisionClockStart()`，3c-b 之後變成 `max(開案, 最後一筆詢問回覆或撤回)`，只有伺服器知道。所以 3d 輸出期限時一併輸出開始時間（例如期限訊息 `{due_time, start_time}`，或伺服器算好的 `warn_time`），client 只依時間點換顏色，不自己重建開始時間的規則。觸發：3d 加 `Case.decide_due_time` 的同一刀。
- 3d 的來源由 httpapi 的合併層在它自己的 `InReadSnapshot` 內呼叫，所以來源方法不開快照：直接用 §3.6 的讀取本體或新的查詢，跑在 `db.Conn(ctx)` 上（C26）。
- 讀取模型只在量測觸發後才建（規則 2：由 outbox 投影，D3／D4 修訂第 5 點），所以 3c-a 不為佇列發事件。若之後觸發，開啟與關閉的寫入點是：
  - 分案與直接設定結果：離開待分案。
  - 開案：進入待決定。
  - 結案與自動關閉：離開待決定；食安採取措施時進入待轉報。
  - 記錄轉報：離開待轉報。
  - 改嚴重度：期限改變。

---

## 7. 測試

### 7.1 model（unit）
- `ReportFacts.Availability()`、`CaseFacts.Availability()` 的原因表：逐格斷言原因與順序；`Decision.DefaultOutcome`；`Report.State`、`Supporting`、`TriageDueAt`（只在待分案、只有食安、邊界 ±1µs）；`Report.HasUnseenSupplements`（補充數等於、大於已看過的編號；有結果不提醒），整合測試 `TestUnseenSupplementsDoNotDependOnTheClock`（補充的時鐘倒退或相同仍是沒看過）。
- `policy`：值不重複、子集是值的子集、`OTHER` 只在檢舉人、`PLATFORM_ABUSE` 只在 Assessable。

### 7.2 資料集測試（integration）
- 成員 ⇔ 判定式：資料集含每種通報狀態（待分案、在進行中的案件、有結果而沒有案件、在已結案的案件、移動過）。
  - `ListReports(state=X)` 的集合等於 `Report.State() == X` 的集合；`ListCases(state)` 同理。
  - 翻頁不漏不重（keyset 測試照 `TestAdminInboxKeysetHasNoGapsOrDuplicates`）。
- 導出的措施的兩處定義：資料集含明確連結（RISK 停權、不帶認定的終止）、引用既有認定的措施、已撤銷的措施、兩個案件共用一個認定、來源通報在不同案件。
  - `ListMeasures(source_case_id)` 的集合等於 `CaseMeasureCounts` 的總數，未撤銷的子集等於 `UnrevokedMeasures`；三條清單路徑（清單、待處理請求的收件匣、複審到期）各自驗證，資料集要讓每條路徑都有成員、也有「導出但不屬於這條路徑」與「屬於這條路徑但沒有導出」的措施。
  - `ListMeasures(source_report_id)` 等於這筆通報在 `finding_reports` 裡的認定的措施。
  - **mutation**：EXISTS 少掉明確連結，或少掉 `measure_findings` 的引用（只看新認定），測試要失敗。
- **I5：每個進行中的案件都有可以選的決定**。資料集含審查 B1 的情境：導出措施 → 成員設為已採取措施 → 撤銷措施。每個指令之後，對全部進行中的案件斷言 `decision_options` 不為空。**mutation**：`ActionTakenSupported()` 改成只算未撤銷的措施，測試要失敗。
- 跨列不變量 I1–I4、I6（每個指令之後對全部案件檢查）。

### 7.3 `can_*` ⇔ 前置條件
- 表格驅動，`available == 指令成功`：
  - 通報：待分案／在進行中的案件／有結果／在已結案的案件 × 是不是認定來源 × 案件有沒有導出措施（無、未撤銷、只有已撤銷）× 案件有沒有轉報紀錄 × 是不是唯一的成員 × 店家開或關；動作是分案、每個結果、移動、當來源。
  - 案件：進行中（有無導出措施、措施是否撤銷、有無已採取措施的成員）／已決定／自動關閉 × 食安與否 × 有無轉報 × 店家開或關；動作是兩個決定、改嚴重度、連結措施、建立措施、轉報、匯出。
- **mutation**：反轉任一個原因，測試要失敗。
- `can_cite_as_source` ⇔ `CreateMeasure`（警告或停權）接受那筆來源。**mutation**：`lockSourceReports` 改回看 `DISMISSED`（新的值裡沒有），測試要失敗。
- `CreateMeasure.source_case_id`：
  - 別家店的案件回 `invalid_argument`，已結案的回 `failed_precondition`，任何依據（警告、停權、RISK、終止）都能連。
  - 同一個 `request_id` 換案件回「內容不同」（**mutation**：request hash 漏掉 `source_case_id`，測試要失敗）。

### 7.4 migration
- `TestPolicyCategoryMigrationRenamesTheValueAndRefusesOtherFindings`：`UNAUTHORIZED_SALE` 的通報與認定改名；有 `OTHER` 認定時失敗並列出 ID。
- `TestReportCasesMigrationMapsEveryStatus`：每種舊狀態逐欄斷言 §2.3 的表，含：
  - `OTHER` 已確認收件的通報。
  - 同店同類別多筆合併、嚴重度取最嚴重、`updated_at`。
  - 轉報搬到案件、既有措施的 `source_case_id` 是 NULL。
- `TestReportCasesMigrationRefusesRowsWithoutAnHonestCase`：R-a、R-b、R-c、R-d 各一個反例，斷言失敗訊息列出 ID。
- `TestSchemaFileMatchesMigrations`、`TestChecksMatchGo`（子集對 CHECK）。
- `TestReportSchemaConstraints` 改寫，DB 要擋下：
  - `ACTION_TAKEN` 而沒有案件。
  - 不同店家或類別（複合外鍵）；移動紀錄的通報與案件不同店；措施的來源案件不同店。
  - `num_nonnulls` 不符；措施的註記帶嚴重度；轉報掛在非食安案件。

### 7.5 指令與並行
- 分案：開新案件與加入既有案件；嚴重度的兩條規則；兩個 goroutine 對同店同類別分案 → 只有一個進行中的案件（**mutation**：拿掉店家鎖，測試要失敗或撞 23505）。
- 移動：
  - 規則（含 `LAST_REPORT_WITH_MEASURE`、`LAST_REPORT_WITH_REFERRAL`）；移動紀錄。
  - 移走最後一筆 → `NO_REPORTS`、`source_case_closed`、不發事件、不算決定（**mutation**：拿掉自動關閉，I1 的檢查要失敗）。
- 結案：
  - 預設結果與說明只帶入沒有結果的通報；每筆一個 `ReportResolved`；已有結果的不變。
  - C8 的兩條規則；B1 的情境能結案。
- 壓力測試（各 200 次，先例 `TestConcurrentSourceRevokeAndLinkedCreateNeverDeadlock`）：結案對補充、移動對補充、分案對 `CreateMeasure`、結案對帶 `source_case_id` 的 `CreateMeasure`，都不互鎖。
- etag：
  - 補充讓通報的版本前進、**案件的版本不變**（**mutation**：補充時 `BumpCase`，測試要失敗）。
  - 舊 etag 的分案回 `aborted`；寫註記不改版本。
  - 補充之後案件的 `unseen_supplement_report_count` > 0；分案、移動與 `MarkReportSupplementsSeen` 那筆通報之後該筆回 false，對別的成員設定結果、改嚴重度不影響（附錄 D #8）。
  - `MarkReportSupplementsSeen`（`TestMarkReportSupplementsSeenRecordsOnlyTheMarkerAndNeverAdvancesTheVersion`）：記下之後通報與案件的版本、`updated_at` 不變；重複記下不改任何東西；讀到之後又有新補充，舊 etag 回版本衝突（`conflictTarget = report`）、標記不動；有了結果的通報不寫入；案件指令的版本衝突 `conflictTarget = case`、分案與移動要加入的案件不成立 `conflictTarget = target_case`、要開新案件而類別剛有進行中的案件 `conflictTarget = category_case_opened`。另有 `TestUnseenSupplementsDoNotDependOnTheClock`（補充的時鐘倒退或與標記相同仍是沒看過；變異：改回比時間 → 失敗）。`TestUnseenSupplementsCompareTheLatestSeqNotTheCount`：Go 的 `HasUnseenSupplements` 比最新一則的編號（`LatestSupplementSeq`，`max(seq)`），與 SQL 的 `EXISTS seq > supplements_seen_seq` 同一個定義；直接寫入不連號的編號（2、5）鎖住兩者一致（變異：改回比則數 → 失敗）。變異：記下時讓版本前進、不比對 etag 都會讓它失敗。

### 7.6 送出、保留、事件
- 補充：
  - 登入與匿名各一；佐證位置接著編號、`supplement_id` 正確、引用同步登記在通報 ID。
  - 原通報有結果時是新的通報（**mutation**：partial UNIQUE 拿掉 `outcome IS NULL`，測試要失敗）。
  - 登入者拿到原本那一筆；`matched_by` 的三個值。
- 上限：第 11 次送出時，登入回 `failed_precondition`，匿名不存內容、`discarded_submissions` 加一、回應不變（**mutation**：拿掉上限，測試要失敗）。
- 不洩漏（審查 M3）：用同一支電話、不同 email 送出三次（第一次新通報、第二次補充、超過上限）。
  - 每次的回應都相同，每次都發一個 `ReportReceived`，`ConfirmationEmail` 都是那次的 email。
  - 不存補充者的聯絡方式（**mutation**：補充不發事件，測試要失敗）。
  - 沿用 `TestFoodSafetyReportResponsesDoNotRevealDuplicates`。
- 自己的店：登入的兩個入口都擋，從沒公開過的店仍是 `not_found`（**mutation**：拿掉檢查，測試要失敗）。
- 保留（**mutation**：條件只看建立時間，測試要失敗）：

  | 建立 | 結果 | 清不清 |
  |---|---|---|
  | 91 天前 | 29 天前 | 不清 |
  | 60 天前 | 31 天前 | 不清 |
  | 91 天前 | 31 天前 | 清 |
  | — | 沒有結果 | 不清 |

- 事件：事件失敗時整筆回滾。

### 7.7 httpapi、gateway、client
- httpapi：`CheckFieldPaths`；REQUIRED 欄位由 descriptor 逐一清掉後要指到路徑；page token 綁篩選；清單與 `GetFDAExport` 不帶 `contact`；`adminauth` 啟動檢查；`TestEveryServiceInTheContractIsMounted`；`EnumMismatches`；`decision_options` 的 `default_outcome`。
- gateway：`TestEveryRouteIsInOpenAPISpec`、`maxRESTOperations = 481`；`scripts/gateway-json-contract-check.py`；`governance_test.go` 刪 `:52-73`；`check-test-skips.py` 的預期 skip 不變。
- admin：
  - 路由與既有測試：`report-pages-wiring.test.ts`（新路由存在、`[source]/[id]` 已刪、側欄文字）；`measure-pages-wiring.test.ts:28`、`:82-88` 改寫；`measure-dialogs.test.ts` 的 `MeasureNoteSchema` 改 `GovernanceNoteSchema`；刪 `governance-wire-contract.test.ts`。
  - 新頁：各分頁籤的請求；`can_*` 對按鈕；三個建立措施的入口與 `sourceCaseId`；a11y 清單加 6 個對話框。
  - 純函式：`deadline.ts` 的邊界與捨去；`fda-export.ts`（沒有聯絡方式、只列 Supporting、佐證數）。
  - 建立措施：`merchant-create-measure-dialog.test.ts`。
    - fixture 改設 `canCiteAsSource`，要有一筆「還沒有結果」但 `canCiteAsSource=false` 的，斷言它不會列出，證明 client 不看狀態。
    - `initial` 預填之後可以修改，送出的是修改後的值，發票的類別仍要選。
    - `sourceCaseId` 進請求與 request key。
  - `invoice-adm-contract.test.ts`：預填只在「依建議建立措施」、欄標題。
- website、seller-app：類別的 import 與期望文字改成新名稱（含「不當使用個人資料」）。

---

## 8. Commit 計畫

每個 commit 都跑 §9 與自己範圍相符的檢查，綠燈才 commit；commit 後 `git log -1 --stat` 核對。審查：3c-a-1、3c-a-3、3c-a-4 各做 code-review；3c-a-3 開工前與 3c-a-4 完成後各跑一次 design-review。

| # | commit | 內容 | 為什麼可以獨立綠燈 |
|---|---|---|---|
| 3c-a-1 | `feat(api): 政策分類 PolicyCategory 取代 ViolationCategory（R10）` | 見下 ① | 只換分類，通報的狀態機不動 |
| 3c-a-2 | `feat(admin): 建立措施對話框可預填，發票停權建議帶入種類、結束時間與理由` | 3b 修訂 6-1：`CreateMeasureDialog.initial`、`measureInitialFromSuggestion`、發票頁、測試 | 只動 admin，與案件無關 |
| 3c-a-3 | `feat(seller): 通報與案件分開，結果、補充與保留規則（R2、R3、R7）` | 見下 ③ | 舊通報兩頁打的是 gateway 的 REST，不受 Connect 契約影響［量測 F11］；admin 用到改名與改欄位的地方都在同一個 commit 改：`SourceReportsField`、`MeasureNotesSection`、`measure-dialogs.test.ts`（F25） |
| 3c-a-4 | `feat(admin): 商家通報與案件頁，刪除舊通報頁與 gateway 的 admin 通報路由` | 見下 ④ | 舊頁、路由與它們的測試一起刪 |

① 3c-a-1 的內容：
- 術語表的 13 列；`policy_category.proto` 與 5 個 proto 的引用；`ViolationSeverity` 的註解。
- `platform/policy`；seller model 改用它；§2.1 migration；子集驗證（`CreateReport`、`NewFinding`）。
- 目錄搬家與 `make uitext`；三個生成端。
- admin（`policy-category.ts`、`CreateMeasureDialog` 的選項、類別文字、測試）、website（`measureDisplay.ts`、測試、e2e mock）、seller-app 測試。
- 文件：設計書 32 §1.2、§3.1、§3.2 的值；CARRYOVER 的類別列；gov ADR 文末「修訂：政策分類」；API 規範的 common 清單；`backend/AGENTS.md` 的 platform 清單。

③ 3c-a-3 的內容：
- §2.2–§2.4、§3、§4；admin 契約、檢舉人契約與 `CreateMeasure`／`ListMeasures` 的新欄位；權限；devseed。
- admin：`SourceReportsField` 改看 `canCiteAsSource`（3b 修訂 6-2）與它的測試；`GovernanceNote` 改名的消費者（`MeasureNotesSection.tsx`、`measure-actions.ts`、`measure-dialogs.test.ts`）。
- 文件：術語表 Q6 的 3 列；seller `AGENTS.md`、CARRYOVER（沿用盤點與 etag 表）、gov ADR 文末「修訂：通報與案件分開」、設計書 32 §3、§4、§6、§7（含 §7 的事件句）。

④ 3c-a-4 的內容：
- §5：頁面、對話框、側欄、共用註記、`deadline.ts`、`fda-export.ts`；三個建立措施對話框的 `sourceCaseId`。
- 刪舊兩頁。
- gateway：刪 7 條路由、handler、wrapper、DTO、openapi、`governance_test.go:52-73`；`maxRESTOperations` 改 481；`routes_governance.go` 去掉沒用的 `admin` 參數與 `routes.go:70` 的傳入。
- 刪 `governance-wire-contract.test.ts`；最後跑 `check-legacy-endpoints.py --update`。
- 兩份 runbook：K16 單人營運、K19 改成移到其他案件、Ch1–Ch3 改寫、K18 的期限數字留給 3d。
- 設計書 32 §5.3、§8。

最後在計畫書 10e 段記完成紀錄，在 rb 加「實作時的修訂（3c-a）」。

---

## 9. 驗證命令（與 CI 相同）

```bash
# backend 單體（backend-monolith.yml）
cd backend && gofmt -l cmd internal
GOWORK=off go vet ./... && GOWORK=off go vet -tags integration ./...
GOWORK=off go test -race ./...
DOCKER_HOST=$(docker context inspect desktop-linux -f '{{.Endpoints.docker.Host}}') GOWORK=off go test -race -tags integration ./...
PATH=$HOME/.local/bin:$PATH make connect && git status --porcelain -- internal/httpapi/gen internal/legacy/gen   # buf 版本對照 api/.buf-version
docker info >/dev/null && DOCKER_CONTEXT=desktop-linux make sqlc && git status --porcelain -- internal/modules
PATH=$HOME/.local/bin:$PATH make uitext && git status --porcelain
GOWORK=off go mod tidy && git status --porcelain -- go.mod go.sum
make lint-monolith                                          # 只看新增的問題
(cd internal/modules/seller/db && atlas migrate validate --dir file://migrations)
# gateway（backend-go-checks.yml、backend-gateway-json-contract.yml；在 backend/ 下跑）
(cd gateway && gofmt -l . && go vet ./... && go vet -tags integration ./...)
(cd gateway && go test -json ./...) | python3 scripts/check-test-skips.py --module gateway --reset
printf '' | python3 scripts/check-test-skips.py --final
python3 scripts/gateway-json-contract-check.py --stats
(cd gateway && go test ./internal/router -run TestEveryRouteIsInOpenAPISpec -count=1)
# 契約（api-contract.yml）
cd .. && buf lint api
pnpm --filter @hawkr/api gen:api && pnpm --filter @hawkr/api exec tsc --noEmit && pnpm --filter @hawkr/api exec biome check . && pnpm --filter @hawkr/api test
(cd packages/hawkr_api && ../../api/generate.sh buf.gen.yaml && dart analyze && dart test)   # 不跑 dart format lib
# 共用文案（ui-text-generated-diff.yml、ui-text-guard.yml、ts-packages.yml、dart-packages.yml）
bash scripts/check-enum-text-cross.sh
pnpm --filter @hawkr/ui-text lint && pnpm --filter @hawkr/ui-text test
(cd packages/hawkr_ui_text && dart analyze && dart test)
# admin（admin-ci.yml）
(cd admin && pnpm lint && pnpm test && pnpm build)
# website（website-ci.yml；只有 3c-a-1 動到）
(cd website && pnpm run lint && pnpm run design:check --apps-root .. && pnpm run test:unit \
  && API_BASE_URL=http://127.0.0.1:9/api/v1 pnpm test \
  && API_BASE_URL=http://127.0.0.1:9/api/v1 NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:9/api/v1 pnpm run build)
# 兩支 App（seller-ci.yml、buyer-ci.yml：hawkr_api 與 hawkr_ui_text 重新生成都會觸發）
(cd seller-app && fvm flutter analyze && fvm flutter test && bash scripts/deadcode.sh)
(cd buyer-app && fvm flutter analyze && fvm flutter test && bash scripts/deadcode.sh)
# repo（legacy-endpoints.yml、repo-hygiene.yml、commit-messages.yml）
python3 scripts/check-legacy-endpoints.py            # 3c-a-4 在所有刪改之後先跑 --update
bash scripts/check-fullwidth-var-expansion.sh
```

admin 的手動 smoke（`admin/AGENTS.md`：完成 UI 後要做 desktop、mobile 與 keyboard smoke；本機 `pnpm dev`，後台 token 照 `admin/AGENTS.md` 的本機段）：
- desktop：三個分頁籤、兩個詳情頁、六個對話框與三個建立措施的入口各走一次，含讀取失敗與空狀態。
- mobile：窄畫面下表格在自身容器捲動、頁面不橫向溢出。
- keyboard：每個對話框可用 Tab 走完、Escape 關閉、焦點回到開啟它的按鈕。

local mock 與截圖不是 DEV、CI 或部署的證據。

看 CI 時看每個 job 自己的結論。沒有跑過的部分在完成回報時列出。

---

## 10. 待決題

**Q1 結案的決定和導出的措施要怎麼一致**（v2 依審查 B1 改寫）
- 現況：rb R2 存「處置或不處置」，沒有寫和措施的關係。結案時沒有結果的通報會帶入結果，並發 `ReportResolved`；A8 之後會把結果告訴檢舉人。撤銷措施不碰案件（F24）。
- 基準：DSA 16(5) 要把決定告訴每位檢舉人；17 要求每個限制都附理由通知被限制的人，這裡的措施就是那個通知（VERIFIED，引用 rb 的研究）。
- 選項：
  - **(a) 建議：照 C8 的兩條規則。**
    - 「採取措施」要有導出的措施（已撤銷的也算），或已有成員是已採取措施。
    - 「不採取措施」要沒有未撤銷的導出措施，也沒有已採取措施的成員。
    - 每個進行中的案件至少有一個可選（I5，資料集測試與 mutation 守著），所以不會卡住。
    - 「已撤銷的也算」的意思是：措施做過就是採取過措施，撤銷是之後的更正（申覆或平台更正），不改寫當時的決定。
  - (b) 不檢查：決定與措施各自獨立。會出現「告訴檢舉人已採取措施，賣家卻沒有任何措施」，或「不採取措施，卻有生效中的停權」。
  - (c) 只提示、不阻擋：對話框顯示同樣的條件，管理員可以照樣送出。
- 依賴：(a) 要「導出的措施」涵蓋 RISK 與終止，也就是 Q4 (i)；Q4 選 (ii) 時，這題只能是 (c)。

**Q2 登入檢舉人重複送出時，回成功還是錯誤**
- 現況：rb R7 把句子稱為「錯誤句」，但內容已經寫進原本那一筆。
- 基準：錯誤回應表示操作沒有發生（AIP-193）；client 對錯誤會保留表單，使用者因此重送。Zendesk 合併重複的請求時，回報的是成功。
- 選項：
  - **(a) 建議：回成功。**
    - `CreateReportResponse.supplemented = true`；`report` 是原本那一筆（模組在去重的鍵含買家時回它）；`notice` 是 #D′1。
    - 句子留在伺服器，因為它說的是伺服器的去重規則（台北日、同店、同類別）。在回應裡放 `LocalizedMessage` 有先例（`MeasureActionUnavailability.message`）。
    - 補充已達 10 則時回 `failed_precondition`（#D′3）。
  - (b) 照字面：維持 `already_exists`，訊息換成 #D′1。
- 現在沒有 client 送出檢舉（F11），兩者的成本都只在契約。

**Q3 匿名的補充：標示、確認信與上限**（v2 依審查 M3 擴充）
- 現況：匿名的去重以電話或 email 其中一個比對。改成補充之後：
  - 知道某人電話的人送出的內容，會出現在那個人的通報上。
  - A8 上線後，若只有新通報寄確認信，「收不收得到確認信」就透露了這支電話這週檢舉過這家店（CARRYOVER:65 的保證會破）。
  - 換 IP 的匿名送出可以一直補充。
- 選項：
  - **(a) 建議：照 R7 補充，並且：**
    - 記下比對方式（`matched_by`：電話、聯絡信箱、兩者都相同）。只有一種相同時，後台標示 #B9。
    - 每次送出都發 `ReportReceived`：新通報、補充、超過上限而丟掉的都發，帶這次送出留的 email。A8 用同一個範本寄到那個地址，地址只在事件與 job 參數裡，不落 seller 的表。
    - 每筆通報最多 10 則補充。超過時，登入者回 #D′3；匿名者內容不存、計數加一、回應與確認信照舊。
    - 剩下的推論：結果信只寄給原通報存下的地址。以別人的電話送出的人，要等很久沒收到結果，才能很弱地推論，接受。
  - (b) 只在這次的電話與 email 都和原通報相同時補充，其他情形不存；事件與確認信照 (a)。代價是會丟掉合法的補充，例如原本只留了電話、這次多留了 email。
  - (c) 照 R7 字面：全部補充、不標示，只有新通報寄確認信。A8 上線後就是上面那個探測器。

**Q4 措施怎麼連到案件**（v2 新增，審查 M1）
- 現況：導出的措施只能從認定的來源通報推導。食安最常見的即時處置是 RISK 停權（`risk_ground = FOOD_SAFETY`），或不帶認定的終止（`FOOD_SAFETY_INCIDENT`），兩者都沒有來源通報（F23），所以推導不到。
- 選項：
  - **(i) 建議：`measures.source_case_id`（C20）。**
    - 從案件建立的措施，不論依據都記下來源案件，驗證同一家店、案件進行中。
    - 導出的措施＝明確連結 ∪ 認定推導。
    - 案件頁開放三種建立措施的入口（建立措施、建立風險措施、終止合作）。
    - 代價：一個欄位與複合外鍵、三個對話框加一個 prop、request hash 多一欄。
  - (ii) 不記連結，只看認定推導，Q1 改成只提示（§3.1 的「Q4 (ii) 的差異」）。比較小，但 RISK 停權中的店家仍可能被決定「不採取措施」，檢舉人收到「未違規」。

**Q5 rb §4 排在 3c-a 的三種待辦，做到哪裡**（v2 新增，審查 M4）
- 現況：
  - rb §4 把待分案、待決定、待轉報列在 3c-a；3cd §3 把 seller 的待辦來源、合併與待辦頁列在 3d。
  - ADR D3 已定為 httpapi 的 keyset composition（F22），所以來源查詢不會因為改投影而白做。
  - 但來源要對齊 3d 才建立的全序比較函式、kind 的 rank 與 `FilterScope`（3cd D′1 的不變式）。
- 選項：
  - **(a) 建議：3c-a 交判定式、開始時間與索引；三支來源查詢、成員的資料集測試、`decide_due_time`、`fda_referral_due_time` 都在 3d（§6）。** 3c-a 的清單篩選已經用同一個判定式，並有資料集測試。
  - (b) 照 rb §4 字面：3c-a 就寫三支來源查詢與資料集測試。這樣 platform 的比較函式與 kind 的 rank 也要提前到 3c-a。

**Q6 給檢舉人看的用語**（v2 新增，審查 minor 12）
- 現況：術語表沒有「通報」「檢舉」「補充」。買家面已核准的按鈕與法律文件用「檢舉」（10d D4 的「⋯ → 檢舉」、使用者政策 §十五）；後台已核准「商家通報」；術語表 :3 要求伺服器句子用表上的詞，:7 又說 admin 用同一套中文詞。
- 選項：
  - **(a) 建議：分受眾。**
    - 買家、賣家看的用「檢舉」，後台用「通報」，併入的內容叫「補充」；術語表加三列，寫明受眾。
    - R7 的句子改成「你今天已經檢舉過這家店的這個類別，這次的內容已經補充到原本的檢舉。」（#D′1）；自己的店是「不能檢舉自己的店。」（#D′2）。
    - admin 已核准的「通報人」改成「檢舉人」（6 列，見 §5.4）。
  - (b) 全部用「檢舉」：admin 的「商家通報」改成「商家檢舉」，契約與表名不改。
  - (c) 全部用「通報」：改 10d 已核准的按鈕與兩份法律文件的用詞。

---

## 附錄 A：相對 v1 的變更（審查與主 session 裁示的對照）

| 項目 | 處理 |
|---|---|
| B1 案件可能永遠不能結案 | C8 改成主 session 定的兩條規則；I5 不變量、資料集測試與 mutation（§7.2）；S10、S17、E3 改寫；Q1 改寫 |
| M1 RISK 與 CONTRACT 的措施推導不到 | C20：`measures.source_case_id`、`CreateMeasureRequest.source_case_id`、導出＝連結 ∪ 推導；案件頁三種入口；`LAST_REPORT_WITH_MEASURE`（I6）；(ii) 的差異寫在 §3.1；Q4 |
| M2 D′9 的提示沒有資料來源 | C27：`ListMeasures.source_report_id` 保留；設定結果對話框一律用它（#D18、#D23） |
| M3 匿名補充成為探測器 | C14、§3.3、§3.5：補充與丟掉的送出也發 `ReportReceived`（`supplement_id`、`Discarded`、這次的確認地址）；補充不改案件的版本，改由 `has_unseen_supplements` 與案件的 `unseen_supplement_report_count`（#A24、#C23、#E21）；上限 10 則與超過時的行為（#B25、#D′3）；併入 Q3 |
| M4 rb §4 的範圍 | §0.2、§6 寫明；Q5 |
| minor 1 3c-a-3 不能獨立綠燈 | §8：`measure-dialogs.test.ts`、`MeasureNotesSection.tsx`、`measure-actions.ts` 改在 3c-a-3 |
| minor 2 送出流程 | §3.3：刪 transaction 外的重複檢查；登入者的補充由模組回原本那一筆 |
| minor 3 約束 | 嚴重度改記在註記的兩欄（CHECK 限案件的註記，C7，我的取捨：不另開表）；`report_case_moves` 加 `store_id` 與三個複合外鍵 |
| minor 4 開發資料轉換 | R-b 改正（`OTHER` 不會有轉報）；`cases.updated_at` 的值；R-d（配合要求掛在不會進案件的通報上） |
| minor 5 本機 migration 失敗 | §2.3 寫明重建本機 DB 的步驟 |
| minor 6 保留的推論 | §3.4 照實寫，加觸發條件（建立滿 180 天仍沒有結果） |
| minor 7 清理的排序 | `EraseExpiredContacts` 改 `ORDER BY resolved_at` |
| minor 8 帶入的結果名稱 | `decision_options` 帶 `default_outcome`（#E4） |
| minor 9 B9 | 用「聯絡信箱」；`matched_by` 加 `BOTH`，兩者都相同時不顯示 |
| minor 10 畫面狀態 | #B24、#C22、#C25、#D22、#D23、#E20；§9 加 desktop、mobile、keyboard smoke |
| minor 11 已關閉的店 | `can_create_measure`、`can_cite_as_source` 看店家狀態；`can_link_measure` 給終止合作 |
| minor 12 術語 | 術語表 3 列、#D′1–#D′3 改用「檢舉」、6 列已核准文字配合改字；Q6 |
| minor 13 分類名稱 | 「不當使用個人資料」／misuse of personal data（新提案） |
| minor 14 文件 | `backend/AGENTS.md` 的 platform 清單、`ViolationSeverity` 的註解、`ListReportsRequest` 兩個篩選同時設的排序 |
| minor 15 驗證與協調 | §9 加 `check-test-skips.py`；§2 寫 A7 的 migration 時間戳協調 |
| minor 16 命名 | C28：作用在案件本身用 `id`，作用在子項用 `case_id`（同 `DecideAppealRequest.measure_id`） |
| 審查者對刪 `RequestCooperation` 的補充 | §0.2：3c-a 到 3c-b 之間用案件的內部註記記系統外的聯絡 |
| 審查者對去重的補充 | §3.3：被判濫報的人同一天可以再送，R11 上線時一起看 |

## 附錄 B：沒有驗證的部分

- 沒有跑任何 build、測試、sqlc 或 Atlas。§2 的 SQL 是草圖，Atlas 產生的 DDL 以實際輸出為準。
- `DecideCase`「先讀成員再鎖」的正確性，依賴「所有改成員的寫入者都先鎖店家」。這是推論，要由 §7.5 的並行測試證明。
- 生成的 Dart 擴充是否仍叫 `.uiText(l)`，讓 seller-app 的程式不必改（F16），是推論。3c-a-1 跑 seller-app 的 analyze 確認。
- River 完成的 job 保留多久、A8 怎麼清掉 job 參數裡的地址：一般知識，沒有查證，留給 A8。
- D3／D4 的決定是 v2 寫完之後才合進來的（`097436097`）。我已逐條對過規則 2、3、4、6 與修訂段，只改了引用與 §3.3、§6、Q5 的說法；設計本身是 seller 內部的，沒有因此改形狀。
- 舊通報頁目前讀 snake_case 的欄位，對今天的 gateway 可能已經顯示空白（另一個 agent 依 commit 日期推論，沒有跑）。這不影響本刀，因為舊頁會整頁刪除。
- §5.4 的數字是我人工數表格的列，不是程式算的。
- 新提案的文字只照術語表檢查過用詞，沒有和賣家條款逐字對照。

## 附錄 C：3c-a-3 審查之後的修正（2026-10-18）

3c-a-3（`beccafbb4`）經獨立審查（0 個 blocker、1 個 major、6 個 minor、2 個 nit），修正與本文的差異：

| 項目 | 處理 |
|---|---|
| major：`CreateMeasure` 的 `source_case_id` 錯誤沒有欄位 | `createMeasurePaths` 的共同部分加 `SourceCaseID`→`source_case_id`；httpapi 測試走 interceptor，斷言四種請求形狀（警告、依據認定的停權、風險停權、終止合作）的 BadRequest 欄位是 `source_case_id`、沒有「欄位沒有附給 client」的 log |
| minor：導出措施的定義有四份 SQL，資料集測試只涵蓋預設清單 | 測試改成三條清單路徑（清單、待處理請求的收件匣、複審到期）加 `CaseMeasureCounts` 逐一驗證（§7.2）。**沒有**改成單一的 SQL 定義：Atlas 社群版略過函式，要手寫 migration 並讓 sqlc 吃得下 `RETURNS TABLE`／`SETOF`；`CaseMeasureCounts` 要集合形式（`UNION`，吃兩個索引），純量的謂詞會把它變成 OR 的 join；也與 C20（Will 核准的「資料集測試鎖住」）不同。觸發：seller 第一個 SQL 函式（`pub_*`，A7-1a 與 10e 收尾）落地時，連同 arch 與契約測試一起重估 |
| minor：有轉報紀錄的食安案件可以被清空 | C6、I6、§3.1 的規則表加 `LAST_REPORT_WITH_REFERRAL`：案件唯一的成員、而案件已記錄轉報時不能移走（與 `LAST_REPORT_WITH_MEASURE` 同一個機制，進 `Availability().Move`，錯誤取第一個原因）；案件的事實一律載入轉報紀錄（`caseFacts` 不再有 `withReferral` 參數）；新增一句伺服器句子（`report.unavailable.last_report_with_referral`，待文案審核） |
| minor：保留的推論沒有寫進文件 | seller `AGENTS.md` 加沒有結果不清除（可能無限期保留）與 180 天的觸發（§3.4）；設計書 32 §6.1 由 3c-a-4 補 |
| minor：`report_evidence_position_check` 只剩下界 | 新 migration 補 `position < 55`（§2.2）；`TestChecksMatchGo` 綁 `model.MaxReportEvidence` |
| minor（測試）：`TestReportSchemaConstraints` 一個案例違反多條約束 | 每個案例只違反一條約束、指名是哪一條（`PgError.ConstraintName`），並驗證拿掉那一條約束之後語句成功（`wantViolates`）。審查沒有發現的兩件事：37 個案例中有 21 個沒有用到 `$1` 卻傳了參數，pgx 回「參數數量不符」，測試等於沒有跑到資料庫；跑到之後，`cases_close_check` 對 NULL 放行（「有結案時間卻沒有原因」「DECIDED 卻沒有決定」都被接受），以 `IS TRUE` 修正（新 migration `seller_cases_close_check_null_safe`） |
| nit：`checkSourceCase` 自己判斷案件是否結案 | 改用 `model.CaseFacts{…}.Availability().LinkMeasure`；`CaseBlock.Text(CaseActionLinkMeasure)` 在已結案時回 #S31 的句子（句子不變） |
| 留給文案審核 | 5 句不在 §5.4 (g) 核准表的新句子，與 `通報太頻繁`→`檢舉太頻繁` 的字面值，及上面新增的 1 句，都在 3c-a-4 的文案輪處理 |

---

## 附錄 D：設計審查之後的修正（2026-10-18，3c-a-4 之後）

3c-a-4 完成後的獨立設計審查（12 項，A 3、B 6、C 3）與 3c-a-4 的 code review，Will 決定（2026-10-18）修正的部分如下；契約在 v1alpha1、上線前，直接改、所有消費端同一次變更重新生成。審查的 #6（共用 `<Dialog>` 元件）另開一刀，這一輪只讓 C3 的 CSS 能被它吸收，不為單一對話框加例外。

| 項目 | 處理 |
|---|---|
| #1 分案與移動的嚴重度規則（client 另讀 `ListCases` 推導「開案或加入」） | 伺服器輸出去處：`Report.triage_targets`（`TriageTarget{category, case_id, severity, case_etag, case_report_count}`，只有 `GetReport` 帶、只在能分案或能移動時有值，順序同政策分類，移動時不含目前的類別）。`TriageReportRequest`／`MoveReportRequest` 的 `severity` 改成 `reserved`，換成 `CaseTarget target`：`oneof { JoinCase{case_id, etag}；OpenNewCase{severity} }`。`joinCase` 在鎖內驗證意圖：加入要求進行中的案件就是讀到的那個、版本沒前進；開新案件要求這個類別此刻沒有進行中的案件；不成立都回 `case.version_conflict`（沿用既有句子，不悄悄改成另一個去處）。沒有設去處回 `invalid_argument(target)`（新句子 `triage.target_required`，待核准）。`triage.severity_with_open_case` 刪除。admin 的 `use-open-cases.ts`、`openCaseOf` 刪除，對話框的類別選項與提示只看 `triage_targets`；案件頁的「移到其他案件」先 `GetReport` 再開對話框 |
| #4 client 重寫 `Report.Supporting()` | `available_outcomes` 改成 `repeated OutcomeOption{outcome, negates_source}`（與 `DecisionOption` 同形）、新增 `Report.effective_category`。admin 刪 `negatesAsSource` 的規則複本（改查伺服器給的表）、`isSupportingOutcome` 與 `buildFdaExport` 的再過濾（`GetFDAExport` 只回 Supporting 的通報）、`effectiveCategory`。code review MED1 同此 |
| #5 網路失敗的判斷寫了三份 | `@hawkr/api/connect/errors` export `isNoResponse(err)`（包過與沒包過的 fetch `TypeError` 都算）；`factsFromError` 的 `noResponse`、admin（刪 `lib/error-text.ts` 的 `isNetworkFailure`）、website `step-up-message.ts` 都用它；測試在套件。code review MED2 同此 |
| #7 佐證的位置在整筆通報連號（`position < 55`，「每次最多 5 張」退成模組檢查） | 鍵改成 `(report_id, submission_seq, position)`：`submission_seq` 0 是原通報、1 到 10 是補充（`report_supplements.seq`，`UNIQUE (report_id, seq)`、`CHECK 1..10`），`position` 0 到 4；`supplement_id` 與 `submission_seq` 一致由 `CHECK ((submission_seq = 0) = (supplement_id IS NULL))` 與複合外鍵 `(supplement_id, report_id, submission_seq)` 保證；`evidence_accesses` 加 `submission_seq` 並跟著用同一個鍵。API 的「第 n 張」與 `/evidence/{n}` 是依 `(submission_seq, position)` 排序的序號，讀取時算、不存。migration `20261018133847_seller_report_submission_seq.sql` 轉換既有資料（補充依建立時間編號、佐證在各自那次送出內重編、讀取紀錄跟著換鍵，`TestSubmissionSeqMigrationRenumbersEvidenceWithinEachSubmission`）。`NextEvidencePosition` 刪除，`model.MaxReportEvidence` 只剩序號的上界 |
| #8 「有新的補充」拿 `cases.updated_at` 當已看過的點 | 每筆通報自己的 `reports.supplements_seen_seq`（SMALLINT 0..10：管理員看過的最新補充的編號 `report_supplements.seq`，0 = 沒看過；**最後一輪審查**：原本記時間，拿應用程式時鐘比另一個實例寫的補充時間，時鐘不同步時標記成功卻仍顯示未看過，所以改比編號、不依賴時鐘；分案、移動記下當時最新一則的編號，補充只在鎖住通報列時新增，所以編號精確）。**哪些動作記下**：分案、移動、**明確的已讀 `MarkReportSupplementsSeen`**（fix3：業界作法是開啟時明確標已讀，GET 不帶副作用）——都帶讀到的 etag，etag 含當時全部補充，所以做了就等於看過；**不記**：讀取（`GetReport` 不改狀態）、設定結果（有了結果的通報不再提醒，記了也看不出來）、結案帶入的預設結果（沒有逐筆看過）、改案件嚴重度與記錄轉報（案件層級）。`MarkReportSupplementsSeen(id, etag)`：權限 `seller.reports.mark_supplements_seen`；只鎖通報列；etag 不符回 `VERSION_CONFLICT`（讀到之後又有新補充，那則還沒被看到，不能記）；已看過、沒有補充、已有結果時不寫入照樣回成功（冪等）；**不讓 `reports.version` 與 `updated_at` 前進**（已讀標記不是對通報的處理，若前進，通報頁一載入就會讓另一個分頁或對話框裡的 etag 過時、造成不必要的衝突），標記存「記下當時最新一則補充的編號」（etag 相符，所以就是當時全部補充）；回傳的通報 etag 與請求相同。通報頁顯示補充時呼叫它（etag 衝突時重讀再記）；案件頁的通報列用 `Report.has_unseen_supplements` 標「新的補充」，頁首與結案對話框用案件的 `unseen_supplement_report_count`（取代已移除的案件旗標，19 reserved）說「有 {n} 筆通報有還沒看過的補充。」。`Report.has_unseen_supplements` = 還沒有結果而且存在編號大於 `supplements_seen_seq` 的補充（Go 端 `SupplementCount > SupplementsSeenSeq`，編號從 1 連號、只新增不刪除）；案件的計數 = 這樣的成員數（`CaseReportCounts` 的 `unseen_supplement_reports`）。`Case.update_time` 不再用來判斷補充 |
| fix3：案件頁「移到其他案件」衝突 | 對話框拿的是開啟時讀的那一筆通報；衝突時連同它重讀（`GetReport`），對話框的去處與 etag 更新後同一個對話框重送成功。重讀失敗時不說「頁面已重新載入」，改顯示伺服器自己的衝突句（`runMeasureAction`）。測試 `案件頁移到其他案件遇到版本衝突…`（變異：衝突時只重讀清單 → 失敗） |
| fix3：版本衝突說是通報還是案件 | `ErrorInfo.metadata.conflictTarget`（`report`｜`case`｜`target_case`｜`category_case_opened`；值定義在 `ErrorReason.VERSION_CONFLICT`），由 `errReportVersion`、`errCaseVersion`（案件指令自己的 etag）、`errTargetCaseVersion`（分案與移動要加入的案件不成立，最後一輪審查新增）、`errCategoryCaseOpened`（要開新案件而類別剛有進行中的案件，最終審查新增）設；`submitError` 的 conflict 帶 metadata；admin 的 `reportConflictText` 依它顯示通報、案件、「要移入的案件」（C27）或「這個類別剛有進行中的案件」（C28）的句子（通報頁與案件頁的通報操作） |
| fix3：`reports_contact_retention` | 刪除（migration `20261018150400_seller_drop_unused_contact_retention_index.sql`）：本機 PostgreSQL 18、30 萬列、含與不含它的 `EXPLAIN` 都選 `reports_contact_cap`（依 `created_at`），它沒有使用者 |
| fix3：背景重讀不閃「載入中」 | `useAllMeasures` 的 `loading` 只在第一次讀（`isLoading`）或還有下一頁；`fresh`（每次掛載要最新的）才看 `isFetching` |
| fix3：網路失敗判斷 | `GovernanceNotice.tsx` 的 `governanceErrorText` 改用 `isNoResponse` |
| #9 預填來源通報時 client 逐頁讀完 | `ListReportsRequest.citable_as_source`（SQL 與 `Availability().CiteAsSource` 同一個定義，資料集測試鎖住；`TestReportAvailabilityIsExactlyWhatTheCommandsAccept`）。admin 只多讀一頁 `page_size = 21`，`earliestSourceReports` 刪除 |
| #10 A8 必須清掉 job 參數裡的確認信地址 | 寫進計畫書 A8 與 `events.go` 的 `ReportReceived` 註解；「未查證」換成已查證：River v0.47.0（`backend/go.mod`）`rivershared/riversharedmaintenance/river_shared_maintenance.go:35-37` 預設保留完成的 job 24 小時、丟棄的 job 7 天（專案沒有覆寫 `*JobRetentionPeriod`），所以寄送失敗的 job 會把地址留 7 天。義務：A8 的寄送 job 在完成或丟棄時清掉參數裡的地址，或參數只放 report／supplement 的參照 |
| #11 沒有結果的匿名通報聯絡方式無限期保留 | Will 決定（2026-10-18）：絕對上限——建立滿 365 天一律清，不論有沒有結果，另外保留原規則（`greatest(建立 + 90 天, 結果 + 30 天)`）；清理 job 回報並記 log 只靠上限清掉的筆數。見 §3.4 |
| #12 盤點表過時 | seller `CARRYOVER.md` 的 `CreateReport` 去重（改成成功回應、`supplemented = true`）與 `GetFDAExport`／`GetReport` 的聯絡方式兩列改寫成現行行為；新增 #1、#4、#7、#8、#9 的列 |
| #2 期限的開始時間（記錄） | §6 的 3d 清單加一條：期限的開始時間必須由伺服器輸出（3d 加 `Case.decide_due_time` 的同一刀）；觸發條件寫在那裡 |
| #3 檢舉人選的類別可以決定 4 小時分案期限、嚴重度卻被拒絕（記錄） | 理由寫進 `CARRYOVER.md` 食安 SLA 那一列：被操弄的代價不同、類別是通報的事實而嚴重度是平台判斷、防濫用靠結果與暫停受理、3d 之後整個被審閱政策取代 |
| #6 共用 Dialog、全域 `.modal-content` 規則 | 共用 `<Dialog>` 另開一刀。C3：把全域的 `.modal-content` 捲動規則換成「外層限高、直欄，只有 `.modal-body` 捲動，頁首與頁尾固定」（`AdminVisualRefresh.css`，`tests/modal-layout.test.ts` 鎖住結構），MergeDialog、CreateProspectDialog 不會有兩層捲軸；之後的共用元件吃同一組結構 |
| code review LOW | `report-list-page.test.ts` 的 `?.disabled ?? true` 改成會在按鈕不存在時失敗；`bringInText` 不把 UNSPECIFIED 顯示成「未知」；導出的措施與設定結果對話框讀措施改成讀完所有頁（`use-all-measures.ts`）；計畫書的完成紀錄補上 design-review；文案見 §5.4a |
| 沒有動 | 設計審查 #6 的共用 Dialog 元件本身（另一刀） |
