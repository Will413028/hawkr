# 10e 第 4a 刀：admin 商家名單的清單頁與詳情頁改走 Connect，實作設計 v2.1

> **狀態**：v2。Will 2026-10-20 對 §5 的 N 類文案與 E10 的狀態篩選回「照提案」（文末決定紀錄）。v2.1 處理開工前的 design-review（9 項，附錄 B）：5 項改（含 Preview 改 POST 的契約註記）、2 項記、2 項駁回。v1 經一輪獨立審查（1 HIGH、9 MED、11 LOW，全部處理，對照見附錄 A；審查全文在 session scratchpad `4a-design-review.md`，不入庫）。

讀取的是 `local/10e` 的 `c6af4ed88`（與 main 相同），worktree 沒有未提交的改動［量測：`git status --short`］。
盤點：2026-10-18 的唯讀盤點（快照 `06b158957`，下稱「盤點」）與 2026-10-20 對 `c6af4ed88` 的複核（下稱「複核」）；兩份都是 session scratchpad，結論收進本文 §0.4，不入庫。
SSOT：計畫 10e 段（`2026-09-29-a3-buyer-seller.md:346-386`）；`step10e-decision-brief.md`（下稱 db：D1、D4 第 8–10 點、K2、K17、K18、K19；Will 2026-10-09 照建議）；`step10-decision-brief.md` B16；ADR `2026-10-04-prospect-contact-points-and-matching.md`（下稱 prospect ADR：D2、D3、D4、文末修訂）；`api/hawkr/admin/seller/v1alpha1/prospect.proto`；admin `AGENTS.md`；術語表。
路徑縮寫：`W` 是 worktree 根目錄；`A`＝`W/admin/src`；`SP`＝`A/app/admin/seller-prospects`；`G`＝`W/backend/gateway/internal`；`M`＝`W/backend/internal/modules/seller`。
標記：**[量測]** 這次讀到原始碼或跑了指令；**[推論]** 由讀到的內容推得；**[外部]** 外部來源。
判定原則同 3d：上線前沒有相容性約束；每個取捨照業界從零的做法；只有基準定不了的產品選擇、UI 改動（根 `AGENTS.md`「UI 改動」）或要改寫已核准決定的，列給 Will。

---

## 0. 前提題：這刀要改的程式之後會不會被重寫、搬走或刪除？

| 程式 | 之後的排程 | 判定 |
|---|---|---|
| admin `SP/page.tsx`、`SP/[id]/**`、建立對話框 | 沒有。這就是 Connect 版的終點 | 會留下，照從零的做法做 |
| gateway 的 prospect 路由、handler、wrapper、openapi | 本刀刪 12 條，4b 刪剩下 4 條與整檔 | 刪除，不在裡面最佳化 |
| `SP/duplicates/**`、`SP/reconciliation/**`、`MergeDialog` | 4b 改走 Connect | 本刀只做讓它們繼續編譯的最小改動（E1） |
| admin `services/media-upload.ts` 的 `uploadImageToMedia` | B1（media 搬進單體）會換掉整條上傳路；沒有排定 | 會留下到 B1，本刀只加三個 fetch 選項（E6） |
| 「品牌圖必須是公開資產」由誰保證 | B1：由 media 依 ref type 決定（db D1 的終點） | 本刀不碰；db D1 已接受到 B1 之前只靠 admin 的預設值（附錄 A #7） |
| seller-service 的 prospect proto 與 mock stub、`rpc-servers.csv` | 步驟 11 刪（db K24） | 範圍外 |

結論：本刀改的 admin 程式都會留下；gateway 這一側是刪除。

---

## 0.1 範圍

| 部分 | 內容 |
|---|---|
| 清單頁 `SP/page.tsx`、`ProspectFilters.tsx`、`ProspectBrandImage.tsx` | `ListProspects` keyset 分頁（`A/lib/keyset-pages.ts`）；篩選對到契約欄位；兩張統計卡改讀 `ListProspectDuplicates`、`ListProspectTaxIDMatches` 的第一頁（E10） |
| 建立 `CreateProspectDialog.tsx`、`ProspectFormFields.tsx`、`MatchCard.tsx` | `CreateProspect`、`PreviewProspect`；聯絡方式編輯器（db D4 第 8 點）；撞到相符業者時開 `ProspectMatchDialog`（E8） |
| 詳情 `SP/[id]/page.tsx`、`ProspectEditForm.tsx`、`ArchiveDialog.tsx` | `GetProspect`、`UpdateProspect`（依 mask）、`ArchiveProspect`、`RestoreProspect`；修改與還原撞到相符業者時開同一個 `ProspectMatchDialog`（db D4 第 9 點） |
| 圖片 `SP/[id]/ProspectMediaPanel.tsx`、`prepareProspectImage.ts`、`A/lib/seller-prospect-media.ts`、`A/services/seller-prospect-media.ts` | 上傳改走 `uploadImageToMedia`（db D1）；換圖以 `UpdateProspect` 的 `logo_upload`／`cover_upload` |
| 轉成店家 `SP/[id]/ConvertPanel.tsx`、`ConvertConfirmDialog.tsx` | `ConvertProspect` 帶 etag；確認框的業者摘要改成主要聯絡方式 |
| 新元件 `SP/ProspectMatchDialog.tsx` | B16：建立、修改、還原共用 |
| 刪除 | `InvitePanel.tsx`、`InviteTokenReveal.tsx`（db K18） |
| `A/lib/seller-prospects.ts` | enum 標籤改成以生成的 enum 為鍵；聯絡方式與 update mask 的純函式；REST 型別只留 4b 頁面還在用的部分 |
| `A/lib/` 共用 | 把 `runMeasureAction` 一族的通用部分搬到 `A/lib/connect-action.ts`，失敗結果帶出 reason 與 metadata（E3） |
| gateway | 刪 12 條路由與對應的 handler、wrapper、DTO、openapi、測試；`maxRESTOperations` 481 → 469 |

## 0.2 不在範圍（4b）

- 疑似重複頁、合併、統編對帳頁（含 reconciliation 的轉成店家）、`MarkProspectsDistinct`；修改模式的「與這家合併」出口（E9）。
- gateway 剩下 4 條路由、`admin_seller_prospect.go` 整檔、`SellerProspectsPath`、`routes_admin_seller_prospect.go`。
- seller `CARRYOVER.md:181,306` 與計畫 10e 的完成紀錄（db「第 4b 刀之後更新」）。
- 修改撞到統編（`failed_precondition`，prospect ADR D1「該做的是合併」）的合併出口：與 E9 一起在 4b 處理；4a 只顯示伺服器句子。
- 單體不動（E7 撤回，附錄 A #7）。

---

## 0.3 決策總表

| # | 題目 | 決定 | 依據 |
|---|---|---|---|
| E1 | 切法（盤點 Q1） | **依頁面**：4a 把清單頁與詳情頁整頁換成 Connect，包含詳情頁的轉成店家與清單頁的兩張統計卡；刪的路由仍是 db 表的 12 條（list、create、preview、get、put、delete、restore、PUT media、invites×2、upload_sessions×2），`convert`、`duplicates`、`reconciliation`、`merge` 留給 4b 的頁面。`ConvertConfirmDialog` 的 props 本來就是窄的摘要（`ConvertProspectSummary`，檔頭註解寫明為了兩個呼叫端共用），4a 把 `publicEmail`／`publicPhone` 改名成 `primaryEmail`／`primaryPhone`，4b 的 REST 呼叫端（只有 `SP/reconciliation/page.tsx`）只改對應的兩個欄位名。4b 頁面從 4a 改到的檔案 import 的符號（`DuplicatePair`、`SellerProspect`、`TaxIDMatch`、字串版 `matchedOnLabel`、`PROSPECT_STATUSES`、`OUTREACH_STATUSES`）原樣保留到 4b，新的 enum 表用新名字（E11） | 業界 strangler fig 以頁面為單位替換［推論］；db 的切分依路由群組，屬於我們自己的計畫分段（SELF-IMPOSED）。依路由切時，詳情頁的 `ConvertPanel` 要在 Connect 的 `Prospect` 上繼續打 REST，比依頁面多一個混合狀態［量測：`SP/[id]/ConvertPanel.tsx:72`］。舊路由從 9d 起已不通（盤點 §結論，[推論]，§9），所以「中間狀態要能用」不是約束 |
| E2 | 錯誤文字 | `governanceErrorText`（只認 REST 的 `ApiClientError`，`A/app/admin/merchants/[id]/GovernanceNotice.tsx:29`）在 prospect 頁全部換掉：送出走 E3 的 `runAction`，讀取失敗走 `connectErrorText(err, fallback)`（`A/lib/error-text.ts:18`，admin 52 處呼叫）。fallback 只在沒收到回應時出現，保留現有句子 | 盤點文案 13 說「後備句全刪（R3）」：R3 只擋 `userMessage(…) ?? "…"`，不擋 `connectErrorText` 的 fallback［量測，複核 D4］，所以照現行慣例 |
| E3 | 送出失敗的處理（**新的共用件**） | 通用部分從 `A/app/admin/merchants/[id]/measure.ts:652-720` 搬到 `A/lib/connect-action.ts`：`runAction(call, reload, fallback, conflictText)`、`actionErrorText(caught, fallback, conflictText)`、`ConflictText`、`ActionResult`；`conflictText` 必填（lib 不依賴 app）。`measure.ts` 只留 `MEASURE_CONFLICT_TEXT`，不留包裝：所有呼叫端改呼叫 `runAction`／`actionErrorText`，措施頁明確傳 `MEASURE_CONFLICT_TEXT`，`measure-pages-wiring.test.ts:66` 的比對式跟著改（v2.1，附錄 B #6）。`A/app/admin/merchants/[id]/kyc-application.ts:493-525` 的 `runReviewDecision` 是同一個流程的第二份，一併改用 `runAction`；重讀失敗時的語意統一成 `runMeasureAction` 的版本（回伺服器句子，不說「已重新讀取」），KYC 這一點是行為修正（附錄 B #1）。`ActionResult` 的 `message` 結果加選填的 `reason` 與 `metadata`（以 `classifyError(factsFromError(caught))` 取得），prospect 以 `reason === ErrorReason.PROSPECT_MATCH_FOUND` 與 `metadataList(metadata.prospectIds)` 開 E8 的對話框。共用套件的 `submitError` 不改（Dart 的 `submit.dart` 是對應實作，只改 TS 會不對稱）。prospect 的建立、修改、還原、封存、轉成店家都走 `runAction`：衝突時先重讀再回 conflict | 從零設計：一支共用 helper。現況已有 8 個檔、5 種資源在用一支叫 measure 的函式［量測：`rg -l "runMeasureAction\|measureErrorText" A`］，第 6 種資源再借用或另寫一份都是錯的方向。必要條件：兩個以上資源共用同一個「衝突先重讀」流程，成立。reason 與 metadata 的必要條件：第一個要依錯誤原因分支的送出（`PROSPECT_MATCH_FOUND`），成立；搬進共用套件 `submitError`（TS 與 Dart 同步）的觸發：第二個 client 需要依 reason 分支送出結果（附錄 B #9）。要改的測試：`error-text.test.ts`、`merchant-measure.test.ts`、`measure-dialogs.test.ts` 的 import（附錄 A #2） |
| E4 | 成功提示 | 儲存、封存、還原、轉成店家、換圖的成功訊息改用 `useToast()`；`GovernanceNoticeBanner` 只留給需要停在頁面上的錯誤 | admin `AGENTS.md`「短暫訊息一律用 `useToast()`」（現行規則） |
| E5 | 同一頁兩個寫入共用 etag | 保留兩個區塊（文字表單、圖片），頁面持有唯一的 `prospect`（含 etag），兩個區塊都以它的 etag 呼叫 `UpdateProspect`，成功後以回應取代它。文字表單的草稿與它開啟時的基準分開存；基準換成新版本時，**只有回應中 `logo_upload`、`cover_upload` 以外的全部可寫欄位（文字、`vendor_type`、`outreach_status`、`priority_stars`、`contacts`、`category_ids`）都等於表單的基準**才沿用新 etag（換圖不改這些），否則當作衝突：保留草稿、顯示 E3 的衝突句。判定寫成純函式並測試 | 契約允許文字與圖片在同一個寫入（`prospect.proto` UpdateProspect 註解），合成一個表單是另一個做法，但那是版面改動，業界兩種都有（Shopify 的商品媒體即時儲存、文字走儲存列）［推論］，不構成改版面的理由。盤點風險 4 |
| E6 | 圖片上傳（db D1 已核准） | `stageProspectImage` 改成先 `validateProspectImage` 再 `uploadImageToMedia(file)`（`isSecure` 預設 false）。舊專用路徑的三個 fetch 選項 `credentials: "omit"`、`referrerPolicy: "no-referrer"`、`signal: AbortSignal.timeout(120_000)` 搬進共用 helper 的 PUT（Banner、食譜、prospect 都受益）；共用 helper 的 http→https 改寫不動；舊的「網址必須 https、不含帳密」檢查不搬（presigned URL 由我們的 media-service 給；共用 helper 的 http→https 改寫是 OSS 時期留下的，整條上傳路徑 B1 會換掉，不在這裡重做，附錄 B #5）。`ai-styles/page.tsx:1090` 的同名副本刪掉、改用共用 helper（先比對兩份的差異，有行為差異就停下回報；附錄 B #7） | db D1；盤點風險 3。presigned URL 不需要 cookie，`omit` 在跨來源時與預設的 `same-origin` 等效、同來源時少送 cookie［推論］ |
| E7 | （v1 的「單體拒絕機敏的品牌圖」） | **撤回**。db D1 已接受到 B1 之前只靠 admin 的預設值；Banner、食譜同樣沒有伺服器檢查，只補 prospect 是局部修補；終點是 B1 由 media 依 ref type 決定 | 附錄 A #7 |
| E8 | B16 對話框 `ProspectMatchDialog` | 建立、修改、還原收到 `PROSPECT_MATCH_FOUND` 時（E3 的 `reason`），取 `prospectIds`；再以送出的值（建立與修改是表單的值，還原是該業者目前的名稱、統編與聯絡方式）呼叫一次 `PreviewProspect`，與 `prospectIds` 取交集，列出名稱、型態、主要聯絡方式與**命中原因**（`matched_on`）；`prospectIds` 中 Preview 沒回的（並行寫入），逐筆 `GetProspect` 補上、不顯示原因。每列勾選「不是同一家」，全部勾選才能送出，送出時 `confirmed_distinct_prospect_ids` 帶**全部** ID（契約：涵蓋全部相符的業者才寫入）。重送又撞到時以新的 ID 集合重開，已勾過且仍在的保留勾選。**對話框不用 `Dialog` 的 `form`**：按鈕是 `type="button"`，因為 React 的 submit 會穿過 portal 冒泡到外層建立對話框的 `<form>`（`Dialog.tsx:225,295` 沒擋），行為測試斷言外層 onSubmit 不被觸發 | 錯誤裡直接附比對結果（AIP-193 的自訂 detail）不採用：錯誤模型 ADR D1「details 只用 `google/rpc/error_details.proto` 的標準型別，不自訂 detail 型別」（附錄 B #2）；prospect ADR D3；db D3（補 `matched_on` 的理由是讓管理員判斷）；db D4 第 9 點；契約 `CreateProspect`、`RestoreProspect`、`PreviewProspect` 註解；「不是同一家」沒有清除的方法（`prospect.proto:91`） |
| E9 | 修改撞到相符業者時的合併出口（盤點 Q2） | 照 prospect ADR D3「讓管理員選擇合併」：修改模式每列加「與這家合併」，開預填這一對的 `MergeDialog`；還原模式不加（封存的業者不能合併，`prospect.proto` MergeProspects 註解）。**放在 4b**（`MergeDialog` 在 4b 才改走 Connect） | prospect ADR D3 已核准；db D4 第 9 點只寫「同一個對話框」。4a 與 4b 之間修改模式只有「確認不是同一家」與「取消」，沒有上線，不是約束 |
| E10 | 清單 | keyset，「上一頁／第 N 頁／下一頁」，不顯示總數（db K2）。狀態篩選拿掉「全部」，預設「進行中」（契約 `ListProspectsRequest.status` 的 UNSPECIFIED 只列進行中，UI「全部」從 REST 時期就標錯，盤點落差 1）。統計卡顯示第一頁筆數，有下一頁時加「+」（db D4 第 10 點；統編對帳卡沿用同規則） | 業界 CRM 清單預設只列進行中、封存以篩選切換（HubSpot、Salesforce list view）［外部，盤點 Q3，本次未重新查證］。契約加 `repeated statuses` 的觸發：員工需要跨狀態搜尋 |
| E11 | enum 文字 | `ProspectStatus`、`VendorType`、`OutreachStatus`、`ContactKind`、`ProspectMatchTier`、`ProspectMatchReason` 的表改成 `Record<Exclude<E, E.UNSPECIFIED>, string>`（新名字，例如 `prospectStatusText`；字串版的舊表留給 4b，E1），不認得的值用 `unknownText()`（目錄 `common.unknown`）。可選的招商進度另列 `EDITABLE_OUTREACH_STATUSES`，不含 `CONVERTED`（契約：不能設成 CONVERTED，`prospect.proto:30,48`）。6 個 enum 都不在共用目錄，admin 單端可以自己寫表 | 複核 A（manifest `inCatalog=false`、`serverOwned=false`）；`scripts/check-enum-text-cross.sh` |
| E12 | 聯絡方式與 update mask | 純函式：每個類型恰一個主要（只有一個時它就是主要）；刪掉主要的那筆、或把它改成別的類型時，原類型的第一筆遞補，新類型沒有主要時它成為主要；新增的列預設類型是電子郵件；送出前以 `@hawkr/api/text/server-text` 去空白並依類型檢查長度（EMAIL 255、PHONE 50、WEBSITE／SOCIAL 500，最多 50 筆）。mask 只列改過的欄位，用 `@hawkr/api/proto/field-mask`；不送 `"*"`（會要求兩張圖）；`category_ids` 沒有 UI，永不列進 mask；沒有任何改動時不送出、儲存鍵停用（契約：沒帶 mask 視為列出所有非零值的欄位） | 契約 `ProspectContact`、`UpdateProspect` 註解；db K17 |
| E13 | 詳情頁的 404 特判 | 刪掉「404 代表狀態不允許編輯」的特判（`SP/[id]/page.tsx`、`ProspectEditForm.tsx`），`GetProspect` 回任何狀態，不能編輯的狀態由頁面依 `status` 隱藏表單 | 契約 `GetProspect`；盤點落差 6 |
| E14 | 測試的形狀 | 行為測試用 `admin/tests/fixtures/dom-harness.ts`（JSDOM＋Connect 假 transport，已有 10 支在用）；純函式用 unit。`seller-prospects-contract.test.ts`：刪 `:17`、`:30`、`:64`（`parseProspectPaged` 隨 list 一起刪）與 `:216` 的 token 外流守門（它只為邀請面板存在，`:194-196`；留著會在新程式的 `const token = await uploadImageToMedia(…)` 加 `router.push(` 上誤報）；`:42`「不含 CONVERTED」改測 `EDITABLE_OUTREACH_STATUSES`。`seller-prospect-media.test.ts`：`:104` 改斷言走 `uploadImageToMedia`，`:22`、`:75` 隨被測函式刪；它在 `scripts/legacy-endpoints-baseline.json:44` 的那一筆隨 `--update` 更新 | 複核 D5；盤點 §2 |
| E15 | `PreviewProspect` 走 POST（**改契約的註記，v2.1**） | `prospect.proto` 的 `PreviewProspect` 從 `NO_SIDE_EFFECTS` 改成 `IDEMPOTENT`：client 以 POST 呼叫，仍可自動重試（〈API 設計規範（Connect）〉:173）。規範 :91 加第二個例外：「請求可能超過網址長度上限的讀取（例如帶整張表單內容的比對預覽）不標，改標 `IDEMPOTENT`，以 POST 呼叫」。三個生成端重新生成；`H/seller/prospect_integration_test.go:72` 一帶以 GET 呼叫 Preview 的測試改 POST | connect-es 只要方法是 `NO_SIDE_EFFECTS` 且 `useHttpGet` 就送 GET，沒有長度退路［量測：`@connectrpc/connect-web@2.2.0` `connect-transport.js:70-76`］；契約允許 50 筆、每筆最多 500 字的聯絡方式，編進 query 會超過 Cloudflare 的 16 KB 網址上限［外部，推論］；表單內容也不該進 URL。業界：GET 只適合小的讀取請求（Connect 協定文件的 GET 說明）［外部，推論］。先例：vendor portal 的 Public 方法不標、走 POST（`H/vendor_portal_test.go:163-164`） |
| E16 | 圖片網址與來源網址的檢查（v2.1） | 刪 `A/lib/seller-prospect-media.ts` 的 `prospectMediaUrl` 網域白名單，圖片一律以 `Image.uri_template` 顯示（ADR `2026-09-25-api-image-uri-template.md`：client 不做網域白名單）；`publicHttpsUrl` 擋 IP 與 `.local` 是伺服器會去取網址時的防護，來源網址只作稽核、伺服器不取，改成與伺服器相同的規則（https、有主機、最長 2048）（附錄 B #3） | 圖片 ADR；`prospect.proto:187-188` |
| E17 | 上傳前在瀏覽器重新點陣化、縮圖、裁切（`prepareProspectImage.ts`） | **沿用**：剝除照片的 EXIF（含位置）並讓操作者預覽裁切；縮圖本身可由 image gateway 的 preset 處理。重新評估的觸發：B1 由伺服器剝除 EXIF 時，只留裁切預覽（附錄 B #8） | 媒體 ADR `2026-09-18-media-storage-r2-image-gateway.md` |

---

## 0.4 查證過的現況

| # | 事實 | 證據 |
|---|---|---|
| G1 | `prospect.proto` 與 gateway 的 prospect 程式（路由、handler、wrapper、testutil）從 `06b158957` 到 `c6af4ed88` 沒有改動；16 條路由、16 個 operation、16 個 schema、12 支 wrapper | [量測] 複核 A，`git diff 06b158957 c6af4ed88 -- <路徑>` 空 |
| G2 | admin 以外沒有這 16 條路由的消費者 | [量測] `rg -il "seller_prospects\|sellerProspects\|seller-prospects\|SellerProspect" website buyer-app seller-app scripts packages` 0 筆（複核 C） |
| G3 | `maxRESTOperations` 481 ＝ openapi 實數 481；測試同時擋超過與低於（`G/router/openapi_coverage_test.go:28`、`:171-174`），4a 設成 469 | [量測] 複核 D7 |
| G4 | prospect 的四個對話框已改用共用 `Dialog`（`A/components/UI/Dialog.tsx:61-114`，支援巢狀：只有最上層處理 Escape 與 focus）；`GOVERNANCE_SOURCES` 已不存在，`admin-governance-modal-a11y.test.ts` 自動掃所有 `<Dialog`；`dialog-skeleton-guard.test.ts` 擋自己畫 overlay | [量測] 複核 D2、D3 |
| G5 | `admin/tests/modal-layout.test.ts:53-67` 寫死 `SP/CreateProspectDialog.tsx` 與 `SP/duplicates/MergeDialog.tsx` 的路徑；本刀不改這兩個檔名 | [量測] 複核 D3 |
| G6 | `governanceErrorText` 不處理 `ConnectError`（走 `caught.message`，會帶 `[code]` 前綴）；`connectErrorText` 是 Connect 頁面的慣例 | [量測] 複核 D4 |
| G7 | `runMeasureAction` 等在 `A/app/admin/merchants/[id]/measure.ts:652-720`，被 8 個檔（措施、通報、案件、來函、商家詳情）使用 | [量測] `rg -l "runMeasureAction\|measureErrorText" A` |
| G8 | `uploadImageToMedia`（`A/services/media-upload.ts`）的 PUT 沒有 `credentials`、`referrerPolicy`、timeout；`stageProspectImage`（`A/services/seller-prospect-media.ts`）有；3c-b 的 `RecordInquiryReplyDialog.tsx:26` 已用通用路由上傳（`isSecure: true`） | [量測] |
| G9 | 單體轉正 prospect 圖片（`M/internal/prospect/images.go:108`，`Uploader{ID: adminID, Admin: true}`）不檢查 `Secure` | [量測] 複核 E1 |
| G10 | `ConvertConfirmDialog` 被 `SP/[id]/ConvertPanel.tsx:20` 與 `SP/reconciliation/page.tsx:12-15` import（`MergeDialog.tsx` 只在註解提到）；props 是窄摘要 `ConvertProspectSummary{displayName, taxId, publicEmail, publicPhone}` | [量測] `rg -n ConvertConfirmDialog SP` |
| G11 | admin transport 對 `NO_SIDE_EFFECTS` 用 GET（`A/lib/connect/transport.ts:30`），`PreviewProspect` 是 `NO_SIDE_EFFECTS`，所以名稱、統編、聯絡方式會在 query string | [量測] `prospect.proto:38-40` |
| G12 | 伺服器句子 `model/prospect.go:157`「%s 有多個聯絡方式時必須恰好一個是主要的」會帶出 `EMAIL` 這類內部代碼；E12 的編輯器以單選避免觸發 | [量測] 複核 D6 |
| G13 | 「16 條路由從 9d 起已不通」：gateway wrapper 打到單體的 `internal/legacy/sellerserver`，它實作 15 支方法、沒有 `Admin*Prospect` 與 `CreateSellerProspect` | [量測方法清單；實際會回錯是推論] 複核 B（盤點寫 17 支是誤記） |

## 0.5 約束帳（本刀新出現的）

| 約束 | 判定 | 來源 |
|---|---|---|
| 每刀刪掉最後一個消費者已改完的路由 | SELF-IMPOSED（我們的計畫分段），保留：審查大小 | 計畫 10e:386 |
| 4a／4b 之間詳情頁與 4b 頁面要能同時運作 | 不成立：舊路由已不通（G13），沒有上線 | 根 `AGENTS.md`「上線前不受相容性約束」 |
| `ConvertConfirmDialog` 要同時服務 Connect 與 REST 呼叫端到 4b | SELF-IMPOSED（切刀造成），代價只是兩個欄位改名（E1） | G10 |
| 合併要兩邊都是進行中 | **REAL**（契約與模組） | `prospect.proto` MergeProspects；`lifecycle.go:184-187`（盤點 Q2） |
| 上傳 session 在 B1 之前留在 gateway | **REAL**（單體沒有上傳 API） | 計畫 8h-2；媒體 ADR |
| 「先重用既有元件與 pattern」 | SELF-IMPOSED，符合基準，保留：`Dialog`、`keyset-pages`、`runAction`、`useToast`、`DeadlineText` 不涉及 | 根 `AGENTS.md`「UI 改動」 |
| 不能送 `"*"` mask、不能送空 mask | **REAL**（契約） | `prospect.proto` UpdateProspect 註解 |

---

## 1. 契約

不改。12 支 RPC 中本刀用 `ListProspects`、`CreateProspect`、`PreviewProspect`、`GetProspect`、`UpdateProspect`、`ArchiveProspect`、`RestoreProspect`、`ConvertProspect`、`ListProspectDuplicates`（統計卡）、`ListProspectTaxIDMatches`（統計卡）。生成碼已在 `@hawkr/api`（第 0 刀 `ad7d55822`）。

## 2. 單體

不改（E7 撤回）。

## 3. admin

### 3.1 清單頁 `SP/page.tsx`

- `useQuery(listProspects, {pageSize: 20, pageToken, status, outreachStatus, vendorType, category, region, keyword})`；篩選值仍放 URL query（現行慣例，`ProspectFilters.tsx` 檔頭）；enum 篩選在 URL 存可讀的小寫字串（`active`、`archived`、`converted` 等），讀回時以白名單轉成 enum，不認得的回預設（同 `A/lib/store-search.ts:37-41` 的 `parseFilterTab`）。
- 分頁照 `A/lib/keyset-pages.ts`；換篩選回到第一頁。
- 兩張統計卡：`listProspectDuplicates({pageSize: 50})`、`listProspectTaxIDMatches({pageSize: 50})` 的第一頁筆數，`next_page_token` 不空時加「+」。
- 清單列的圖片用 `logo_image.uri_template`（`ProspectBrandImage.tsx` 改讀 `Image`）。

### 3.2 建立 `CreateProspectDialog.tsx`

- 即時查重：`PreviewProspect`（名稱、統編、contacts），停打 300 ms 後送；結果列在 `MatchCard`（`tier` 與 `matched_on` 多值以「、」連接）。查重失敗不擋建立（§5 #12）。
- 送出 `CreateProspect`；`already_exists`（統編）顯示伺服器句子；`PROSPECT_MATCH_FOUND` 開 `ProspectMatchDialog`（E8），確認後帶 ID 重送。
- 成功：toast，導到詳情頁。

### 3.3 詳情頁 `SP/[id]/page.tsx`

- `GetProspect`；頁面持有 `prospect`（E5）。非進行中時隱藏編輯表單與圖片區塊，顯示狀態說明（§5 #19）。
- `ProspectEditForm`：草稿與基準分開；招商進度的下拉用 `EDITABLE_OUTREACH_STATUSES`（目前是 CONVERTED 的業者不會進到表單，非進行中隱藏）；送出以 E12 算 mask；改到聯絡方式撞到相符業者開 `ProspectMatchDialog`；衝突走 E3 並保留草稿。
- `ArchiveDialog`：封存帶 etag；還原不帶 etag，撞到相符業者開 `ProspectMatchDialog`（還原模式）。
- `ProspectMediaPanel`：上傳（E6）後以 `UpdateProspect`，mask 只列 `logo_upload` 或 `cover_upload`，`source_url` 選填、有填才檢查 https。
- `ConvertPanel`：`ConvertProspect{id, store_id, etag}`；`ConvertConfirmDialog` 的業者摘要取主要的電子郵件與電話（E1）。
- 刪 `InvitePanel.tsx`、`InviteTokenReveal.tsx` 與頁面上的引用。

### 3.4 `ProspectMatchDialog.tsx`（新）

- props：`mode: "create" | "update" | "restore"`、`prospectIds: string[]`、`candidate`（送出的名稱、統編、聯絡方式，給 Preview）、`initiallyConfirmed: string[]`、`onConfirm(ids)`、`onCancel`。內部一次 `PreviewProspect`，補讀缺的 `GetProspect`（E8）；讀取中、讀取失敗（可重試）兩種狀態。
- 用共用 `Dialog`（巢狀在建立對話框之上，G4），不用 `form` prop（E8）。

### 3.5 `A/lib/`

- `connect-action.ts`（E3，新）：通用部分搬移、`conflictText` 必填、結果帶 `reason` 與 `metadata`；`measure.ts` 留措施預設句的一行包裝；非措施的呼叫端改呼叫 `runAction`。
- `seller-prospects.ts`：E11 的新表、E12 的純函式；4b 頁面還在用的 REST 型別與字串版 `matchedOnLabel`、`PROSPECT_STATUSES`、`OUTREACH_STATUSES` 原樣保留（4b 刪）。
- `seller-prospect-media.ts`：刪掉「此環境尚未提供品牌圖片編輯功能…」（§5 #26）與 REST payload 型別。
- `services/seller-prospects.ts`：只留 `duplicates`、`reconciliation`、`merge`、`convert`（4b 刪）。
- `services/media-upload.ts`：E6 的三個選項，只加在公開上傳的 PUT（presigned URL）；`isSecure: true` 走 `secure_upload` 的分支（`invoices/jobs/[jobId]/page.tsx:1062`、`RecordInquiryReplyDialog.tsx:26`）是同來源的 API，不動。呼叫端：Banner、食譜、prospect；`ai-styles/page.tsx:1090` 有自己的同名副本，不受影響。

## 4. gateway（刪除）

依盤點 §2（複核 G1 確認未變）：
- 路由 12 條（`G/router/routes_admin_seller_prospect.go`）：list、create、preview、get、put、delete、restore、PUT media、POST invites、DELETE invites/:invite_id、POST media/upload_sessions、POST media/upload_sessions/:session_id/complete。
- `G/handler/admin_seller_prospect.go`：對應的 12 個 handler、2 個 helper、`adminOperatorAccountID`、4 個 mapper、10 個 DTO；wrapper 8 支（`G/grpc/seller_client.go` 的 Create、List、Get、Update、Archive、UpdateMedia、CreateInvite、RevokeInvite）。
- openapi 12 個 operation、10 個 schema。
- 測試：`G/router/admin_seller_prospect_binding_test.go` 整檔刪；`admin_seller_prospect_routes_test.go` 16 → 4；`G/handler/admin_seller_prospect_media_test.go` 刪 3 個 UpdateMedia 測試；`G/handler/testutil_test.go` 8 個 `*Fn` 改 stub（2b 的先例）。
- `maxRESTOperations` 481 → 469；`scripts/check-legacy-endpoints.py --update`；`gateway-json-contract-check.py` 跑一次，check_a 的 4 筆掛在 `AdminSellerProspectItem`（4b 才刪），預期不用 `--update`：check_a 依檔案、鍵與方向比對，`AdminSellerProspectItem` 留到 4b（openapi `createdByAccountId` 仍是 2 筆）［推論，實作時以輸出為準］。**與 db 不同**：db 寫「4a 要 `--update`」。
- 實作者先逐一讀刪除清單上的每個符號、確認沒有 4b 路由還在用，再刪（delegation「刪除既有符號」）。

---

## 5. 文案表（4a 的部分；A＝已核准、C＝跟隨契約或 SSOT、E＝沿用、**N＝新寫，要 Will 核可**）

行號是 `c6af4ed88`（複核 D6）。用語：頁面標題、側欄與「查無此站外業者」用「站外業者」（現行）；頁內句子用「業者」（伺服器句子與契約註解的說法）。術語表沒有這組詞，4a-2 補上「站外業者／業者、進行中、已封存、已入駐、聯絡方式、主要」。

| # | 位置 | 改前 → 改後 | 類 |
|---|---|---|---|
| 1 | 新表（E11）、`ArchiveDialog.tsx:123` | 狀態 ACTIVE「營運中」→「進行中」（proto 註解與伺服器句子都是「進行中」，`prospect.proto:106`、`prospect.go:348`）；ARCHIVED「已封存」、CONVERTED「已入駐」照舊 | C |
| 2 | `SP/page.tsx:140` 副標 | →「尚未加入平台的目標賣家名單，供業務團隊追蹤聯絡與入駐進度。預設只列進行中的業者，已封存或已入駐的請切換狀態篩選。」 | N |
| 3 | `ProspectFilters.tsx:126` | 狀態篩選拿掉「全部」，預設「進行中」；選項：進行中／已封存／已入駐 | N（E10） |
| 4 | `SP/page.tsx:292` | 「第 {page} / {totalPages} 頁，共 {total} 筆」→「第 {page} 頁」 | A（db K2） |
| 5 | `SP/page.tsx:120-130,159` | 兩張統計卡：第一頁筆數，有下一頁加「+」；aria「疑似重複：{n} 組候選，點選查看清單」 | A（db D4 第 10 點）＋N（統編對帳卡沿用、aria） |
| 6 | `SP/page.tsx:42`、`MatchCard.tsx:10`、`A/lib/seller-prospects.ts:219` | 不認得的 enum →「未知」 | C（`common.unknown`） |
| 7 | `CreateProspectDialog.tsx:259-266`、`ProspectEditForm.tsx` 4 個聯絡欄位 | → 區塊「聯絡方式」、類型「電子郵件／電話／網站／社群帳號」、單選「主要」、按鈕「新增聯絡方式」 | A（db D4 第 8 點） |
| 8 | 同上 | 每列「移除」；空狀態「尚未新增聯絡方式」；說明「同一種類型有多個時選一個主要的：主要的電子郵件用來寄詢價邀請，主要的網站給採購搜尋。」 | N |
| 9 | `CreateProspectDialog.tsx:255` 統編提示 | →「選填；業者名稱、統編與聯絡方式都會用於下方的即時查重」 | N |
| 10 | `MatchCard.tsx:8-9` | 「T1・統編唯一鍵」「T2・聯絡方式相符」照舊；新增「T3・名稱相同」 | E＋N |
| 11 | 新表（E11；舊的字串版 `A/lib/seller-prospects.ts:208-217` 留給 4b） | 「統一編號相同」「社群帳號相同」照舊；「公開信箱相同」→「電子郵件相同」、「公開電話相同」→「電話相同」、「官網網域相同」→「網站網域相同」；新增「名稱相同」；多個原因以「、」連接 | C（照 `prospect.proto:158-169` 的 enum 註解）＋N（連接方式） |
| 12 | `CreateProspectDialog.tsx:229` | →「即時查重暫時失敗，不影響建立；送出時仍會再比對一次。」 | N |
| 13 | 4a 檔案的 6 處後備句（另 3 處在 4b 的 `MergeDialog.tsx:122`、`reconciliation/page.tsx:138,162`；`CreateProspectDialog.tsx:227` 由 #12 取代） | 保留原句，改由 `connectErrorText` 或 `runAction` 帶入（E2） | E |
| 14 | `ProspectMatchDialog` 標題 | 「可能是同一家業者」 | N |
| 15 | 建立模式說明 | 「下列業者的聯絡方式與這次填寫的相同。是同一家的話請取消，改去編輯既有的業者；確認都不是同一家，逐一勾選後再送出。」 | N |
| 16 | 修改、還原模式說明 | 修改：「下列業者的聯絡方式與這次修改後的相同。確認都不是同一家，逐一勾選後再儲存。」（4b 加合併出口時補「是同一家的話可以合併」）；還原：「下列進行中的業者聯絡方式與這筆相同。是同一家的話不需要還原；確認都不是同一家，逐一勾選後再還原。」 | N |
| 17 | 對話框操作 | 每列勾選「不是同一家」；按鈕「確認不是同一家並建立／儲存／還原」、「取消」；讀取中「正在讀取相符的業者…」；讀取失敗「無法讀取相符的業者。」＋「重試」 | A（B16 流程）＋N |
| 18 | `SP/[id]/page.tsx:155`、`ProspectEditForm.tsx:165` | 「查無此站外業者，或其狀態不允許編輯」→「查無此站外業者」 | C（E13） |
| 19 | `SP/[id]/page.tsx:119` 結尾 | →「…須先還原為進行中才能編輯；封存時刪除的 logo 與封面不會回來。」 | N |
| 20 | 編輯的版本衝突 | 「資料已被其他操作更新。畫面已重新讀取，你輸入的內容還在，請確認後再送出。」 | N（句型同 `kyc-application.ts:62-64`） |
| 20b | 封存、轉成店家的版本衝突 | 「資料已被其他操作更新。畫面已重新讀取，請確認最新狀態後再操作。」 | N（句型同 `MEASURE_CONFLICT_TEXT`） |
| 21 | `ArchiveDialog.tsx:130` 封存說明 | →「封存後這筆業者會立即從採購候選搜尋、統編對帳與疑似重複清單消失，寄給它的詢價連結會失效，logo 與封面會刪除；欄位僅供檢視，之後可以還原為進行中（圖片與詢價連結不會回來）。」 | N（後果量測自 `lifecycle.go:42-52`、`backend/internal/modules/agent/agent.go:149-151`，盤點文案 21） |
| 22 | `ArchiveDialog.tsx:131` 還原說明 | →「還原後這筆業者會重新出現在採購候選搜尋、統編對帳與疑似重複清單，可以繼續編輯與聯絡追蹤；封存時刪除的圖片不會回來。聯絡方式與其他業者相同時，會先請你確認是不是同一家。」 | N |
| 23 | `ProspectMediaPanel.tsx:286` | 來源網址「（必填）」→「（選填）」 | C（契約） |
| 24 | `ProspectMediaPanel.tsx:308` | →「請上傳品牌提供或允許使用的素材；有來源頁面就一併記錄。網址只作來源紀錄，不會自動擷取圖片。」 | N |
| 25 | `ProspectMediaPanel.tsx:124` | →「來源頁面網址必須是 https 開頭的公開網址。」（有填才檢查） | N |
| 26 | `A/lib/seller-prospect-media.ts:17` | 「此環境尚未提供品牌圖片編輯功能…」→ 刪 | C |
| 27 | `ProspectMediaPanel.tsx:161` | →「資料已由其他操作更新（文字或圖片）。畫面已重新讀取，請確認最新資料後再重新選擇圖片。」（圖片區塊走 `runAction` 自動重讀；Will 2026-10-21 改核） | N |
| 28 | `A/services/seller-prospect-media.ts:28,32,42,48` 的 4 句上傳錯誤 | 共用 helper 丟的是技術訊息（`media-upload.ts:33,70,83,92`）；圖片區塊捕捉後一律顯示既有的使用者句子「圖片上傳失敗，請重新選擇圖片後再試。」，不把技術訊息給操作者 | E |
| 29 | `InvitePanel`、`InviteTokenReveal` | 隨檔刪 | A（db K18） |
| 30 | `ConvertConfirmDialog.tsx:110-111`「信箱 ·」「電話 ·」 | 取主要的電子郵件、電話；沒有時照現行的「未填寫」 | E |
| 31 | 成功 toast | 「已建立業者」「已儲存」「已封存」「已還原」「已更新 logo」「已更新封面」「已轉成店家」 | N（現行橫幅句子改短，E4） |

---

## 6. 測試

### 6.1 unit（`admin/tests`）

- 聯絡方式：每類型恰一個主要；只有一個時自動主要；刪掉主要時同類型第一筆遞補；超過長度與筆數的錯誤。
- update mask：只列改過的欄位；不含 `"*"`；不含 `category_ids`；聯絡方式整批列 `contacts`；清空文字欄位時列出該欄位。
- E5 的 etag 沿用判定：回應文字等於基準 → 沿用；不等 → 衝突。
- enum 表：每個非 UNSPECIFIED 值都有文字（以生成的 enum 窮舉），不認得的值 →「未知」。
- `runAction`：原測試改 import 後照跑；新增：`failed_precondition`＋`PROSPECT_MATCH_FOUND` 時結果帶 `reason` 與 `metadata.prospectIds`。
- 共用上傳 helper：公開上傳的 PUT 帶三個選項，`isSecure` 分支不帶。
- update mask：沒有改動時回空、呼叫端不送出。
- 聯絡方式：改類型時兩個類型的主要都重算。

### 6.2 行為（`dom-harness`）

- 建立撞到 `PROSPECT_MATCH_FOUND`（metadata `prospectIds` 兩筆）：開對話框、Preview 一次、列出命中原因、兩筆都勾才能送、重送的 `confirmed_distinct_prospect_ids` 等於兩筆。**突變**：只帶一筆時測試要失敗。
- 巢狀送出：在對話框按確認，外層建立表單的 onSubmit 只因重送被呼叫一次，不因冒泡多一次。
- Preview 沒回的 ID 以 `GetProspect` 補上、不顯示原因。
- 重送又撞到第三筆：對話框以三筆重開，前兩筆保持勾選。
- 修改衝突：`aborted` → 重讀 → 草稿還在、顯示 #20。
- 還原撞到相符業者：還原模式說明、不帶 etag。
- 清單：翻頁帶 `page_token`、換篩選回第一頁；統計卡有下一頁時顯示「+」。

### 6.3 gateway

- `TestEveryRouteIsInOpenAPISpec`、`openapi_coverage_test`（469）、刪後的 routes test 只剩 4 條。

## 7. Commit 計畫

| # | 內容 | 審查 |
|---|---|---|
| 4a-0 | `A/lib/connect-action.ts`（E3，含 KYC 的 `runReviewDecision`）；共用上傳 helper 的三個選項與刪 `ai-styles` 的副本（E6）；`PreviewProspect` 改 `IDEMPOTENT`、規範 :91、三個生成端與單體測試（E15） | code-review |
| 4a-1 | 清單頁、建立、`ProspectMatchDialog`、詳情頁、圖片、轉成店家；刪邀請面板；同一個 commit 刪 gateway 12 條路由與 `maxRESTOperations` 469 | code-review |
| 4a-2 | 文件：計畫 10e 的 4a 完成紀錄；8h-2 那句更正（db D1「上傳走 gateway 通用的 `/app/media` 路由」；**與 db 不同**：db 排在 4b 之後，D1 在 4a 落地，所以提前）；術語表補 prospect 的詞（§5） | 併入完成前的 design-review |

design-review 兩次：Will 決定後、派 4a-0 之前一次，只看本設計；4a-2 之後看整刀。

實作分派：主 session 寫派工、驗證與審查；實作派 Sonnet subagent。

## 8. 驗證命令（與 CI 相同；派工前照當時的 `.github/workflows/` 再核對一次）

- admin（`admin-ci.yml`）：`pnpm --filter admin lint`（含 `tsc` 與 `check-enum-text`）、`test`、`build`。
- gateway（`backend-gateway*.yml`）：`gofmt`、`go vet ./...`（含 `-tags integration`）、`go test ./...`；`python3 scripts/gateway-json-contract-check.py`。
- `scripts/check-enum-text-cross.sh`（新的 enum 表）。
- api（`api-contract.yml`，E15）：`buf lint`、三個生成端的 drift（backend `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `../../api/generate.sh buf.gen.yaml`；PATH 先放 `$HOME/.local/bin`）；單體 `go test ./internal/httpapi/...`（integration 帶 `DOCKER_HOST`）。
- 跨專案：`scripts/check-migration-inventory.py`；最後一步 `scripts/check-legacy-endpoints.py --update` 再跑一次不帶 `--update`。

## 9. 沒有驗證的部分

- 「16 條路由從 9d 起已不通」是讀方法清單的推論，沒有實際打（G13）。結論只影響「中間狀態要能用」不成立的理由；即使能通，E1 仍成立（沒有上線）。
- 管理員 token 走通用上傳路徑、單體以 `Uploader{Admin: true}` 轉正：讀程式碼成立，3c-b 有同形狀的呼叫端，沒有 live 跑過；實作後以 `backend-e2e-verify` 或 dom-harness 以外的本機堆疊跑一次換 logo。
- `PreviewProspect` 的 GET 問題由 E15 改成 POST，不再是未驗證項。
- json-contract 的 check_a 是否需要 `--update`（§4）：實作時以輸出為準。
- E10 的業界基準沿用盤點的查證，本次沒有重新查。

---

## 決定紀錄（Will 2026-10-20「照提案」）

- 2026-10-21：#27 改成「…畫面已重新讀取，請確認最新資料後再重新選擇圖片。」（圖片區塊衝突時自動重讀，舊的「重新讀取」按鈕拿掉）。
- §5 的 N 類文案全部照提案，含 #3 狀態篩選拿掉「全部」、預設「進行中」（E10）與用語「站外業者」（標題、側欄）／「業者」（頁內句子）。

## 實作時的修訂（4a-1）

- E8：`PreviewProspectResponse` 的 matches 不帶 contacts（`prospect.proto:283`），v2.1 的「Preview 回的列顯示主要聯絡方式」做不到。改成：`prospectIds` 每一筆都 `GetProspect`（名稱、型態、狀態、主要聯絡方式），`PreviewProspect` 一次只用來取命中原因（`matched_on`，與 `prospectIds` 取交集；沒回的那幾列不顯示原因）。審查 4a-1 MED。
- §3.3：非進行中的業者不隱藏欄位，改成唯讀顯示（§5 #19、#21 的「僅供對照／僅供檢視」就是這個意思），只隱藏編輯、圖片與轉成店家的操作。審查 4a-1 LOW。
- `ArchiveDialog` 的模式（封存或還原）在開啟時固定；衝突重讀後狀態已不符合這個模式時，只留「取消」並顯示 §5 #20b。審查 4a-1 LOW。
- 來源網址輸入框上限 2048（E16），不是沿用的 2000。審查 4a-1 LOW。
- 業務優先星等：契約是 int32，非整數送不出去；輸入框只接受數字（不新增錯誤句）。實作者回報。

---

## 附錄 A：審查意見的處理（v1 → v2）

| # | 意見（嚴重度） | 處理 |
|---|---|---|
| 1 | `runAction` 把 `PROSPECT_MATCH_FOUND` 變成一句文字，拿不到 ID（HIGH） | E3：`message` 結果帶 `reason`、`metadata`；只改 admin 的 lib，不改共用套件（Dart 對應實作） |
| 2 | 搬移後的預設句與測試斷言（MED） | E3：`conflictText` 必填、`measure.ts` 留一行包裝；列出要改 import 的測試 |
| 3 | 4b 頁面 import 的字串版表（MED） | E1、E11：舊表原名保留到 4b，新表用新名字 |
| 4 | G10 呼叫端數（LOW） | 改正：只有 reconciliation |
| 5 | `seller-prospect-media.test.ts` 漏列（MED） | E14 |
| 6 | 邀請面板的 token 守門測試（MED） | E14：必要條件隨邀請面板消失，刪 |
| 7 | E7 重開已核准的 D1，且只補 prospect（MED） | 撤回 E7 |
| 8 | E7 的做法與斷言（LOW） | 隨 #7 撤回 |
| 9 | 巢狀對話框的 submit 冒泡（MED） | E8：不用 `form` prop，加行為測試 |
| 10 | 命中原因拿得到（MED） | E8：Preview 一次取 `matched_on` |
| 11 | 招商進度排除 CONVERTED、改類型的主要（LOW） | E11、E12 |
| 12 | 空 mask（LOW） | E12 |
| 13 | E5 的比對範圍（LOW） | E5：圖片以外的全部可寫欄位 |
| 14 | URL 存數字（LOW） | §3.1：存可讀字串、白名單 |
| 15 | 「已入駐」與「已轉成店家」不一致（MED） | §5 #1、#3 用「已入駐」（現行） |
| 16 | 「業者」與「站外業者」混用（LOW） | §5 開頭定案，4a-2 補術語表 |
| 17 | 文案分類（LOW） | #1、#11 改 C；#28 保留使用者句子；#13 改 6 處；#30「未填寫」 |
| 18 | 修改撞到統編沒有出口（LOW） | §0.2：與 E9 一起在 4b |
| 19 | E6 的呼叫端（LOW） | §3.5 列全 |
| 20 | 路徑不完整（LOW） | §5 #21 |
| 21 | 與 db 的兩處落差沒標（LOW） | §4、§7 標「與 db 不同」 |

## 附錄 B：開工前的 design-review（v2 → v2.1，9 項；審查全文在 session scratchpad `4a-design-review-start.md`）

| # | 發現 | 處理 |
|---|---|---|
| 1 | `kyc-application.ts:493-525` 的 `runReviewDecision` 是第二份同流程，重讀失敗的語意不同（A，高） | **改**：E3 一併收，語意統一 |
| 2 | `PROSPECT_MATCH_FOUND` 只帶 ID，應在錯誤裡附 `ProspectMatch`（B，中；建議交 Will） | **駁回**：錯誤模型 ADR D1 明定不自訂 detail 型別，ID 放 `ErrorInfo.metadata` 正是那個形狀；client 多一次 Preview 是已接受的代價 |
| 3 | `prospectMediaUrl` 白名單與 `publicHttpsUrl` 比伺服器嚴（A，中高） | **改**：E16 |
| 4 | Preview 走 GET（C，中；建議交 Will） | **改**：E15。照規範的既有先例加一條例外，可逆、沒有上線，不需要另外決定 |
| 5 | 共用 helper 的 http→https 改寫（A，中） | **駁回**：整條上傳路徑在 B1 換掉（前提題），不在將被取代的程式裡最佳化；設計書不再拿它當依據 |
| 6 | `measure.ts` 的一行包裝只為了不改原始碼比對測試（B，中低） | **改**：E3 不留包裝 |
| 7 | `ai-styles/page.tsx:1090` 的上傳副本沒有結論（B/C，中） | **改**：E6 刪副本 |
| 8 | `prepareProspectImage` 沒評估（A，低） | **記**：E17 |
| 9 | admin 在 `submitError` 外再分類一次（B，低） | **記**：E3 寫下搬進共用套件的觸發 |

## 附錄 C：完成前的 design-review（8 項；審查全文在 session scratchpad `4a-design-review-done.md`）

| # | 發現 | 處理 |
|---|---|---|
| 1 | 撞到相符業者 → 對話框 → 帶 ID 重送的流程在建立、修改、還原各寫一份（B，高） | **改**：收成一個 hook，三處共用；4b 的合併出口加在這一份 |
| 2 | `runAction` 照搬 `runMeasureAction` 的簽名、丟掉寫入的回應，prospect 用閉包帶值，建立傳空的 reload（B，高） | **改**：`runAction` 回傳寫入的回應（`{ok: true, value}`），reload 選填；4a-0 的呼叫端一起改 |
| 3 | 相符業者對話框 1 次 Preview 加 N 次 GetProspect（B，中） | **記**：相符筆數通常是個位數、只有一位營運人員，N 次讀取的代價可忽略；Preview 回應帶 contacts 要改契約與模組。觸發：相符筆數常超過 5，或對話框開啟的等待被抱怨 → Preview 的 matches 帶 contacts（或 `BatchGetProspects`，AIP-231） |
| 4 | client 重寫伺服器驗證，與規範 :109「驗證只在模組一處」衝突（A，中） | **駁回**：:109 說的是伺服器端不把規則分成 proto 與模組兩處（不用 protovalidate），不是禁止 client 預檢；repo 為 client 鏡像伺服器文字規則提供了 `@hawkr/api/text/server-text`（根 `AGENTS.md` 地圖），預檢讓操作者在送出前看到錯，伺服器仍是最後一道 |
| 5 | 寫入後 `setQueryData` 又整組 `invalidate`，詳情被重讀；4b 用的 REST 快取 key 一起失效（B，中） | **改**：寫入後只更新詳情、只讓清單類查詢失效；REST 快取 key 隨 4b 刪 |
| 6 | 統計卡的 5 分鐘快取照搬（A，低） | **記**：舊理由「與清單頁共用快取」不成立，「查詢成本高」部分成立（整表自連接）；保留 5 分鐘，觸發：疑似重複的處理改成即時需要（例如 4b 的頁面處理後回到清單頁），改成寫入後失效 |
| 7 | 文字表單與圖片區塊共用 etag、client 比對欄位（B，低） | **記**：E5 的取捨；觸發：第二位營運人員同時編輯同一筆業者成為常態，或圖片區塊加入文字欄位 → 合成一個表單、一次送出 |
| 8 | 從 REST 版照搬的過時註解（A，中） | **改**：刪改 |

另外在完成前一輪一併處理：§5 #27 改核的句子（Will 2026-10-21）；非進行中的業者仍顯示圖片區塊、只停用操作（舊版行為，修正輪把它整塊隱藏是退步）。
