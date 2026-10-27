# A3b 步驟 10e（admin）開工的決定題

HEAD `422349b06`。兩份唯讀盤點（admin 呼叫點、gateway 路由）只當線索，未入庫。起草者沒有參與盤點；結論都先對過 SSOT：計畫 `2026-09-29-a3-buyer-seller.md`（8e–8h、10b D1:198、10e:256）、`step10-decision-brief.md`（B4、B5、B10、B16、B18、附錄 A）、seller `CARRYOVER.md`、ADR（KYC `2026-10-02`、治理 `2026-10-02`、prospect `2026-10-04`、地址 `2026-10-04`、曝光 `2026-10-05`）、`api/hawkr/admin/{seller,identity}/v1alpha1`，以及原始碼。

## 結論

**Will 2026-10-09：D1–D5 全照建議**（D4 含實作者補的其他文案），知會項 K1–K22 照做。決定的落點：計畫 10e 段。

- 要 Will 決定的有 **5 項**：
  - **D1** 業者圖片的兩條上傳 session 路由：建議刪除，admin 改用 Banner、食譜已經在用的通用上傳。計畫 8h-2 與 10e 的字面在這裡互相牴觸。
  - **D2** SEO 長度：設計書 48 寫 60／160，契約與 DB 是 70／180，兩邊都沒有決策紀錄。建議以契約為準，同時更正設計書。
  - **D3** `ProspectDuplicate` 缺 `matched_on`：建議契約補上，疑似重複頁照舊顯示「為什麼疑似重複」。
  - **D4** UI 同意：把 13 點版面與文案改動收成一題，每點附改前改後。B5、B10、B16、B18 已核准的部分不重問，只列實際文案。
  - **D5** 違規清單頁（申訴佇列）：賣家送出的申訴，後台目前**沒有入口**可以找到並裁定。建議新增一頁。
- 知會 **K1–K22**，照 SSOT 直接做。
- 範圍：一共刪 **44 個 operation**（openapi 實數：12＋16＋16）。照 D1 建議 `maxRESTOperations` 513 → 469；3-3 先落地時是 511 → 467。分 6 刀（文末）。
- 盤點有 5 處錯誤或過時（附錄）。最重要的兩處：B10 的「所有類別都要確認收件」已經寫在計畫:157；`[id]/page.tsx` 的停權文案與現況不符。

---

## 照 SSOT 直接做、不問（知會）

不同意哪一項就說。

| # | 項目 | 做法 | 來源 |
|---|---|---|---|
| K1 | 傳輸 | 全部改走 connect-query（`adminTransport`），範本是 `membership-plans/page.tsx`、`BannerEditorClient`（`submitError`）、`members/account-list.ts`（keyset） | admin `AGENTS.md`；附錄 A |
| K2 | 分頁 | 商家、業者、通報、停權紀錄、疑似重複、統編對照都改成上一頁／下一頁，不顯示總數。「第 x / y 頁，共 n 筆」拿掉 | 附錄 A「頁碼改 keyset」 |
| K3 | etag | UpdateStore、Approve／RejectKYC、Update／Archive／Merge／Convert prospect、Revoke／DecideAppeal 都帶 etag。`VERSION_CONFLICT` 時重讀並保留輸入。SEO 存檔失敗今天只寫 `console.warn`（`seo/page.tsx:66`），改成顯示錯誤 | `CARRYOVER.md:29`；附錄 A |
| K4 | 商家清單的欄位與篩選 | 拿掉評分與登入信箱兩欄。城市改由 `town_code` 對照。「待審核」不再含草稿，`forward_failed` 改用 `payuni_bind_status=FAILED`，統計卡改成 int32 的新口徑。`status=CLOSED` 可以列出已關閉的店 | `CARRYOVER.md:214-215`；附錄 A |
| K5 | 以 email 搜尋店家 | 用兩段查詢：`IdentityAdminService.ListAccounts(query)` 找到帳號，再 `ListStores(account_id)`。搜尋框的「登入信箱」因此仍然成立 | `CARRYOVER.md:214`；`seller.proto` `ListStoresRequest.account_id` |
| K6 | 刪除帳號 | 改呼叫 `IdentityAdminService.DeleteAccount{account_id, reason}`，帳號用 `Store.account_id`，確認框的信箱用 `ListAccounts(query=account_id)`。理由上限 200 改 1000（`auditreason`）。gateway 的 `DELETE /admin/accounts` 已在 8e 刪除，現在這顆按鈕打的是死路由 | `CARRYOVER.md:213`；計畫:120；B5（2a 修訂，見計畫 10e） |
| K7 | 核准與退回 | 只有 `SUBMITTED` 可以核准，`forward_failed` 改走 B18 的重新綁定。「PayUni 送審網址」與 `platformStorefrontUrl` 拿掉。退回勾選的欄位對應 `KYCField`，`doc_${idx}` 合併成 `BUSINESS_COMPANY_DOCUMENTS`（契約沒有逐張的欄位）。上一次的退回理由改讀 `GetKYCApplication.reviews`（2b-0 修訂，見計畫 10e） | `CARRYOVER.md:270-271`；附錄 A |
| K8 | 組織型態 | 申請的 `companyType` 列刪除，改顯示 GCIS 快照的 `organization_type` | 計畫:198（10b D1） |
| K9 | PayUni 卡 | 改讀 `GetPayUniRegistration`（第一次核准前是 `not_found`）。狀態改看 `kyc_flow_status` 與 `outcome`。原碼徽章 `PayuniStatusBadge`、`lib/payuni-status.ts`、`FRESHNESS_LABEL` 刪除。聯絡欄位改讀 Store 的 `contact_name`、`contact_mobile`、`support_email`、`contact_phone`、`address` | `CARRYOVER.md:250,277`；10c D3 |
| K10 | 停權 | `SuspendStore` 的 reason 用 enum，標籤用呈現層的對照表（取代 CodeMeta `SELLER_SUSPENSION_REASON_LABEL`）。目前的停權改讀 `GetStore.admin_suspension`，紀錄改讀 `ListSuspensions`（含違規停權，有 `violation_id` 的連到違規）。違規停權的解除回 `failed_precondition`，畫面說明要從違規撤銷 | 計畫:118；B15；ADR codemeta（2a 修訂，見計畫 10e） |
| K11 | 通報收件匣 | 「來源」篩選改成「類別」，嚴重度篩選拿掉（嚴重度欄保留）：伺服器只篩狀態、類別、店家，待確認的通報還沒有嚴重度。詳情改用 `GetReport`，路由從 `[source]/[id]` 改成 `[id]`（順帶修掉超過 200 筆就找不到的 bug）。佐證圖改用 `view_uri` 經 `/api/bff` 讀取 | `CARRYOVER.md:80`；`governance.proto:243-253` |
| K12 | 違規建立 | 只有警告與停權，罰款、部分停權的選項刪除。「公開摘要」改成必填的「給賣家的處分理由」（`statement`），「同意公開」勾選與「high（公開於賣家頁）」的說明刪除。升級提示改讀 `escalated_from` | `CARRYOVER.md:72`；治理 ADR:68；附錄 A |
| K13 | FDA 轉報 | client 讀 `GetFDAExport` 組出 `.md`；記錄用 `FDAChannel` | 附錄 A |
| K14 | 撤銷與申訴裁定 | `RevokeReason` 與現在的下拉一對一（`APPEAL_ACCEPTED` 不能選）。裁定用 `ACCEPTED`／`REJECTED`，不再用 bool | `governance.proto` |
| K15 | KYC 顯示全文的機制 | 平常只顯示遮蔽值。按 `RevealKYCSensitive`（理由必填）才拿到全文與證件的 `view_uri`，15 分鐘後失效。對話框的文字在 D4 第 5 點 | 計畫:123；`CARRYOVER.md:273` |
| K16 | 審核者 | 只顯示 admin ID，不顯示姓名；identity 沒有查管理員的方法，今天也是顯示 ID，沒有退步 | `CARRYOVER.md:271`「姓名快照不成立」 |
| K17 | 業者表單 | 4 個平面的聯絡欄位改成 `contacts[]`（編輯器在 D4 第 8 點）。`priorityStars` 改成 int32。`category_ids` 不新增 UI | prospect ADR D2 |
| K18 | 會員方案與邀請 | 刪除 `seller-membership-plans` 頁與側欄入口（`Sidebar.tsx:175`），以及 `InvitePanel`、`InviteTokenReveal` | 計畫:256；附錄 A |
| K19 | `PROSPECT_MATCH_FOUND` 的 metadata | `@hawkr/api/connect/errors` 的 `ErrorFacts` 沒有 metadata（`errors.ts:34-45`）。共用套件加一個讀 `ErrorInfo.metadata` 的 helper，解析 `prospectIds`（逗號連接、由小到大） | 計畫:128；API 規範 §八 |
| K20 | admin 的鄉鎮對照 | 地址 ADR 的基準是「單一來源生成到各平台」，TS 目前只生成到 website（`twaddr/generate.go:143`）。比照 Dart 的 `packages/hawkr_twaddr`，TS 也做成共用套件：`twaddr.gen.ts` 與手寫的 `twaddr.ts` 移進 `packages/`，生成器、CI 的 `twaddr-generated-diff.yml` 路徑、根 `AGENTS.md` 的地圖、website 的 9 個 import 一起改，admin 從套件 import。不採用的做法：admin 複製一份，或直接顯示代碼 | 地址 ADR:43；根 `AGENTS.md` 地圖 |
| K21 | 代行營運 | 不動：`routes_delegated.go:34` 的 preflight、`/admin/sellers/:store_id/*` 的代行群組、`features/delegated-operations/**`。`delegated-operations-ui-contract.test.ts:105,1311-1401` 斷言商家清單有 `.merchant-email` 與 `isActive && !isSuspended`，要在同一個 commit 改成新欄位（只改測試，不改代行程式） | admin `AGENTS.md`「ADM 代行營運」 |
| K22 | 零消費者的路由 | `GET /admin/certifications`（`CARRYOVER.md:269`：admin 從沒呼叫）與 `PATCH /admin/violations/:id` 跟著所在的刀刪除。不做 `UpdateViolation` 的 UI | — |

---

## 要 Will 決定

### D1. 業者品牌圖的兩條上傳 session 路由

**現況**
- 計畫 8h-2（`:128`）：「上傳 session 仍在 gateway 的 media 路由（保留到 B1）」。舊盤點 `seller-api-gateway-inventory.md:131` 把這句解讀成保留 `POST /admin/seller_prospects/media/upload_sessions` 與 `…/:session_id/complete` 兩條。
- 計畫 10e（`:256`）：「刪除 gateway 的 … prospects 路由」，沒有例外。兩處字面衝突。
- 這兩條是專用路由。它們存在的唯一理由寫在 `routes_admin_seller_prospect.go:26-29`：通用的 `/app/media/upload_sessions` 讓呼叫端自己指定 `isSecure`，專用路由把它寫死成 `false`。
- admin 的 Banner 與食譜（已經是 Connect 的 content 模組）用的就是通用路由（`services/media-upload.ts` 的 `uploadImageToMedia`，預設 `isSecure:false`）。轉正時兩者完全相同：`Uploader{ID: adminID, Admin: true}`（content `images.go:26`、prospect `images.go:108`）。

**業界基準**：Shopify 的 `stagedUploadsCreate`、Stripe 的 File Upload 都是一個通用的上傳端點，用途（purpose）由伺服器依資源判斷，不會每種資源各開一條上傳路由。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 保留兩條專用路由到 B1 | gateway 多留 2 條 operation 與一份 handler、測試、openapi、json-contract 的 baseline；業者成了 admin 唯一一個上傳路徑不同的圖片擁有者 |
| (b) **刪除**，`stageProspectImage` 改呼叫 `uploadImageToMedia` | 與 Banner、食譜同一條路。「不讓呼叫端翻 isSecure」的保證變弱：要靠 admin helper 寫死，但 admin 本來就是受信任的呼叫端，翻錯的後果也只是 logo 顯示不出來，不會外洩 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 上傳 session 在 B1 之前留在 gateway | REAL（media 搬遷前沒有單體的上傳 API） | 計畫:128；媒體 ADR |
| 必須用業者專屬的路由 | SELF-IMPOSED（公司時期的防呆；admin 已經有通用 helper） | `routes_admin_seller_prospect.go:26-29` |

**建議 (b)**：在第 4a 刀刪除兩條路由，連同 handler、`admin_seller_prospect_media_test.go`、legacy-endpoints 與 json-contract 的 baseline。把 8h-2 那句更正成「上傳走 gateway 通用的 `/app/media` 路由」。
**終點與觸發**：B1 media 搬進單體時，由伺服器依 ref type 決定是不是機敏資產，`isSecure` 不再由呼叫端決定。

### D2. SEO 標題與描述的長度上限

**現況**
- 設計書 48 §4（`:132`、`:167`）寫的是：「`seo_title` 以 gateway 的 `max=60` 為準，`seo_description` 以 `max=160` 為準；DB 刻意較寬（70／180）作為安全餘裕；驗證權威在 gateway」。gateway 的實作也是這兩個值（`admin_seller.go:399-400`），admin 的計數器也是（`seo/page.tsx:274,311` 的 `/60`、`/160`）。
- 7a 搬進單體時把 DB 的寬度當成了驗證上限：`model/validation.go:87-90`、CHECK（`seller_core.sql:40-41`）、proto 註解（`seller.proto:169-172`）都是 70／180（commit `06c53c442`）。計畫、CARRYOVER、ADR 都沒有記錄這個改變（`git log -S'SEO 標題不可超過 70'` 只找到 7a 那一筆；`rg -n -i seo CARRYOVER.md` 沒有長度相關的列）。屬於「沿用也是決策」漏掉的一項。

**業界基準**：Google 依像素寬度截斷（標題約 600px），不是依字數。Shopify 的上限是標題 70、描述 320，編輯畫面只顯示計數，不另外警告。中文字比較寬，60 字的英文經驗值對中文本來就不準。硬上限防的是儲存與濫用，截斷的提醒是另一回事。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) **以契約為準 70／180**：admin 計數器改成 `/70`、`/180`；設計書 48 標註已被取代；CARRYOVER 補一列沿用盤點 | 不需要改後端；上限比公司時期寬 10／20 字 |
| (b) 回到 60／160：改 validation、CHECK、proto 註解 | 恢復公司時期的值，但它是英文的經驗值；要寫 migration |

**約束帳**：沒有 REAL 約束。上線前不受相容性約束，兩邊都可以改。

**建議 (a)**。
**終點與觸發**：上線前做 SEO 規劃時（ADR 2026-10-03:45）若決定加「可能被截斷」的提示，再依像素寬度做。

### D3. 疑似重複配對缺「命中原因」

**現況**：admin 的疑似重複表格與合併對話框都顯示 `matchedOn`（`duplicates/page.tsx:126`、`MergeDialog.tsx:212`）。`ProspectDuplicate` 只有 `left` 與 `right`（`prospect.proto:234-237`）；`ProspectMatch` 則有 `matched_on`（`:230`）。清單的來源有兩種：名稱相同，或比對鍵相同（ADR D4）。管理員要靠這個判斷該合併還是標記「不是同一家」，而合併不可逆。

**業界基準**：HubSpot 的「Manage duplicates」與 Salesforce 的 Duplicate Rules，每一對都會列出命中的屬性或規則。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) **契約補 `repeated string matched_on = 3`**，值域同 `ProspectMatch`（`NAME` 或比對鍵的種類），模組在產生配對時一併帶出 | 一個欄位、一處查詢；畫面不變 |
| (b) 拿掉這一欄 | 管理員要自己比對兩邊的資料，才能做不可逆的決定 |

**約束帳**：沒有 REAL 約束（契約與模組都在我們的範圍內）。

**建議 (a)**，放在第 0 刀。

### D4. UI 同意（版面、文案、流程）

已核准的 B5、B10、B16、B18 不重問，只列要用的文案。每一點都沿用既有元件與 design token。

1. **商家清單**（`merchants/page.tsx`）
   - 篩選分頁，改前：全部｜營業中｜待審核｜PayUni 審核中｜待重試｜停權中，加上可點的「本月增加賣家」統計卡。改後：全部｜已核准（`kyc_status=APPROVED`；原本的「營業中」其實就是 KYC 核准）｜待審核｜已退回｜待重新綁定（`payuni_bind_status=FAILED`）｜停權中｜已關閉。
   - 「PayUni 審核中」這一頁刪除：契約只能篩儲存的 `bind_status`，而 `BOUND` 也包含 PayUni 已核准的店家（`CARRYOVER.md:214`「推導值只顯示」）。審核中的狀態仍會顯示在每一列的徽章上。
   - 「本月增加」改成不能點的數字：沒有建立時間的篩選，這個分頁今天本來就沒有送任何參數。
   - 統計卡「已核准（含 PayUni 處理中）」改成「已核准」。
   - 「類型」徽章（個人／企業）拿掉：`StoreListItem` 沒有這個欄位，店家詳情裡有。
   - 「匯出」按鈕拿掉：它只會跳出「匯出功能開發中...」（`:182-184`），違反 admin `AGENTS.md`「不得用無功能控制」。
2. **停權與解除的文案**（已查證，見附錄第 4 點）
   - 確認框（`:2273`），改前「停權後該賣家商品將全部下架、未付款訂單將被取消」。改後「停權後買家看不到這家店與它的商品，未付款訂單會被取消，優惠券會停用。」
   - 成功訊息（`:825`），改前「已停權該賣家：商品已全部下架，未付款訂單已取消。」改後「已停權。買家已看不到這家店；未付款訂單與優惠券會陸續取消、停用。」
   - 解除停權（`:845`），改前「已解除停權：該賣家的商品與優惠券將自動還原。」改後「已解除停權。店家符合公開條件時商品會恢復顯示，優惠券會自動恢復；已取消的訂單不會恢復。」
3. **B5 關閉店家與刪除帳號**
   - 新增「關閉店家」按鈕，放在「刪除帳號」旁邊。對話框的說明照 website 的 `CloseStoreSection`：「關閉後商店與商品不再公開，logo 與封面會清除，賣家身分會移除；帳號與買家身分會保留。這個動作無法復原。有未結訂單時無法關閉。」理由必填。有未結訂單時顯示「這家店還有未完成的訂單，無法關閉」。
   - 「刪除帳號」的說明，改前「刪除即時生效且無法復原：…商品與優惠券隨之下架」。改後「刪除整個帳號（買家與店家）。沒有未完成的訂單時立即刪除，否則等最後一筆訂單完成後自動刪除。無法復原。」
   - 結果分兩種：`DELETED` 顯示「已刪除這個帳號」；`PENDING_ORDERS` 顯示「已受理。帳號有未完成的訂單，完成後會自動刪除；期間賣家只能處理訂單與售後。」
4. **B4 GCIS 並列**（公司型的申請）
   - 申請內容右邊加一欄「商業登記（GCIS）」，逐列並列名稱、統編、地址、設立日期，再加組織型態與登記現況兩列。
   - 不一致的欄位標紅色「不一致」，無法比對的標灰色「無法比對」。
   - 欄頭顯示「查詢時間 {時間}・查到」，或「查無此統編」、「查詢失敗」。查詢中顯示「登記資料查詢中…」，沒有快照時顯示「沒有登記資料」。
   - 登記現況不是核准設立時，顯示警示徽章「{status_text}（停業期間 …）」。
   - 「重新查詢」按鈕只在審核中的公司型申請出現。
5. **KYC 顯示全文**
   - 證號與帳號平常顯示遮蔽值，證件影像的位置顯示「需先顯示全文」。
   - 按「顯示全文」開對話框：「顯示全文會記錄你的帳號、時間與理由；15 分鐘內可以檢視完整號碼與證件影像。」理由必填，最多 1000 字。
   - 顯示後標示「全文顯示到 {HH:mm}」，到期後收回成遮蔽值，要再看就重新申請。
   - 改前：影像自動載入，號碼是明文（而且今天的影像路由已經刪除，本來就壞了）。
6. **B18 PayUni 卡**：`bind_status=FAILED` 時，在卡片上顯示「重新綁定」，取代再按一次「核准」。卡片的「最後同步」改成顯示 `status_time`。
7. **轉換業者時找店家**（`ConvertPanel.tsx:161,170,177`、`ConvertConfirmDialog.tsx:140`）
   - 標籤，改前「搜尋賣家（店名或聯絡信箱）」，改後「搜尋賣家（店名、電話、商店代號或登入信箱）」（信箱照 K5 用兩段查詢）。
   - 提示，改前「店名可能重複，請以聯絡信箱核對身分。」，改後「店名可能重複，請以商店代號與電話核對。」每一列顯示 `store_code` 與 `contact_phone`。
8. **業者的聯絡方式編輯器**：4 個固定欄位改成清單，每列是類型下拉、值、「主要」單選（每種類型恰好一個），加「新增聯絡方式」。版型沿用表單的既有輸入列。
9. **B16 延伸到修改與還原**：B16 核准的是建立與疑似重複頁；ADR 的 D3 與後果段（prospect ADR:37,48）規定修改與還原也要有同一個分支。用建立時的同一個對話框，不另做新的。
10. **疑似重複頁**：「結果已截斷」的提示改成分頁。列表頁的徽章顯示第一頁的筆數，有下一頁時加「+」（例如「20+」）。
11. **通報指令（B10）**：「確認收件」對所有類別都顯示，嚴重度在確認時選（計畫:157 已寫）。新增的部分是「要求賣家配合」：舊 UI 只在食安通報出現，契約對所有還沒駁回的通報都允許（`governance.proto:49-50`）。建議照契約，對所有類別顯示。
12. **SEO 計數器**：依 D2 顯示 `/70`、`/180`。
13. **商家詳情的「已關閉」狀態**：`GetStore` 也會回已關閉的店。頁首顯示「已關閉」徽章與 `closure`（誰關的、理由、時間），停權、關店、核准等操作全部隱藏。（**修訂 2026-10-21，第 5b 刀，Will 核可 F4、F6**：能不能做一律照伺服器的旗標與原因，不再由 client 從店家狀態推導。結束措施只看措施自己的 `can_end`（措施以帳號為範圍，關店不解除停權，所以已關閉的店家詳情頁會出現「結束」）；刪除帳號是帳號層級的動作，帳號讀得到就顯示（已關閉的店家頁會出現「刪除帳號」）；建立措施、核准、退回、重新查詢、重新綁定在已關閉時由伺服器說不能並給句子。）

**業界基準**：Stripe Dashboard 與 Shopify admin 的通則是：清單篩選只用儲存的狀態；不可逆的操作要說清楚後果並記理由；個資預設遮蔽、限時顯示（JIT access）。上面 13 點都照這些做。

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 清單只能篩儲存的事實，不能篩推導的流程狀態 | REAL（8e 的決定） | `CARRYOVER.md:214` |
| 停權不再下架商品，而是讓買家看不到 | REAL（已上線的機制） | ADR 2026-10-05 決策 2 |
| 保留舊的分頁、欄位與文案 | SELF-IMPOSED（上線前不受相容性約束） | 根 `AGENTS.md` |

**建議**：13 點全部照做，實作者寫的其他文案一併同意（10b E 項的先例）。

### D5. 違規清單頁（申訴佇列）

**現況**：`/admin/violations/[id]` 只能從「建立違規」成功後跳過去（`violations/new/page.tsx:111`），側欄與其他頁都沒有連結。賣家送出的申訴，管理員找不到，`DecideAppeal` 實際上沒有入口。後端在 8g 已經做了 `ListViolations`，可以篩店家、狀態、申訴狀態（`appeal_status=PENDING` 就是待裁定，`governance.proto:359-367`）。`CARRYOVER.md:80` 把「沒有 admin 的違規列表」判定為不成立。

**業界基準**：DSA 第 20 條要求平台有內部申訴機制並及時處理。Amazon Account Health 的申訴、Shopify 的 Trust & Safety 都有待處理佇列，依送出時間處理。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) **新增「違規處分」頁**：側欄放在「違規通報」下面；預設篩選「申訴待裁定」，另有全部、生效中、已撤銷；欄位是店家、類別、嚴重度、處分、狀態、申訴狀態、建立時間；點進去是既有的詳情頁。商家詳情的停權紀錄裡，違規停權連到違規詳情 | 一頁新的清單，版型照通報收件匣 |
| (b) 只在商家詳情加這家店的違規 | 管理員還是看不到全平台待裁定的申訴 |

**約束帳**：沒有 REAL 約束，RPC 已經存在。

**建議 (a)**，放在第 3 刀。

---

## 切分（每刀在同一個 commit 刪掉最後一個消費者已經改完的路由，連同 handler、wrapper、測試、openapi 與 `maxRESTOperations`）

| 刀 | 內容 | 刪除的 gateway 路由 | `maxRESTOperations`（3-3 先落地時減 2） |
|---|---|---|---|
| 0 | 前置：D3 契約與模組（`matched_on`）、`make connect` 與 TS 重新生成；K19 的 metadata helper；K20 的 TS twaddr 共用套件 | — | 513 |
| 1 | 商家清單、統計卡（含首頁 `admin/page.tsx:41`、分析頁 `analytics/page.tsx:66`）、SEO 頁、發票開立時查店家（`invoices/issuances/new/page.tsx:336`，改用 `GetStore`）、業者轉換面板的店家搜尋（`ConvertPanel.tsx:88` 也打 `GET /admin/sellers`，必須同刀改）；刪除會員方案頁；D4 第 1、7、12 點 | `GET /admin/sellers`、`/sellers/stats`、`PATCH /sellers/:store_id/seo`、membership 2 條、`seller_membership_plans` 2 條（共 7） | 506 |
| 2a | 商家詳情的店家資料、停權（K10）、B5 與刪除帳號（K6）、已關閉狀態；D4 第 2、3、13 點 | suspensions 3 條加 `GET …/suspension`（共 4）；`ResolveSellerAccountID` wrapper 在此歸零 | 502 |
| 2b | KYC 審核：`GetKYCApplication`、顯示全文、B4、核准與退回、B18、PayUni 卡；D4 第 4–6 點 | `GET /sellers/:store_id`、certifications 4 條（共 5）；`GetPayuniStatus` wrapper 歸零 | 497 |
| 3 | 治理：收件匣、通報詳情與指令、FDA、配合要求、違規建立、詳情、撤銷、裁定、D5 違規清單；D4 第 11 點。發票申訴頁連到 `/violations/new?seller_id=` 的參數一起改（`invoices/complaints/[complaintId]/page.tsx:527,554`） | 通報 7 條、違規 5 條（共 12）。非 admin 的 6 條留著（`routes_governance.go` 的前半） | 485 |
| 4a | 業者：清單、建立與預覽（B16）、詳情與編輯（聯絡方式、B16 延伸）、封存與還原、圖片（D1）；刪除邀請面板；D4 第 8、9 點 | list、create、preview、get、put、delete、restore、media、invites 2 條，加上 D1 的 upload_sessions 2 條（共 12） | 473 |
| 4b | 業者：疑似重複（D3、MarkProspectsDistinct）、統編對照、合併、轉換；D4 第 10 點 | duplicates、reconciliation、merge、convert（共 4）；`registerAdminSellerProspectRoutes`、`SellerProspectsPath` 刪除 | 465 |

- 每一刀各自做 code-review 與 design-review。
- 第 4b 刀之後更新 seller `CARRYOVER.md`（8e–8h 那幾列的「步驟 10」）、計畫 10e 的完成記錄，以及 8h-2 的那句更正。
- 驗收照 CI：admin 跑 `pnpm lint`、`pnpm test`、`pnpm build`；gateway 跑 `gofmt`、`go vet -tags integration`、`go test`、`TestEveryRouteIsInOpenAPISpec`、`gateway-json-contract-check.py`（4a 要 `--update`）；再跑 `check-legacy-endpoints.py --update`、`check-migration-inventory.py`。
- **和平行的 10c 3-3 共用的檔案**：`backend/api/openapi/openapi.yaml`、`router/openapi_coverage_test.go`（`maxRESTOperations`：3-3 是 513 → 511）、`router/constants.go`、`grpc/seller_client.go`、`handler/testutil_test.go`、計畫文件，可能還有 `cmd/doc.go`、兩份 baseline JSON。**後 commit 的一方 rebase 並重算**：`maxRESTOperations` 等於當時 openapi 的實際 operation 數。
- 3-3 與第 2b 刀都落地後，`GetSellerByAccountID`、`GetCertificationDraft`、`UpdateSeller` 這幾個 wrapper 就沒有呼叫者了。由後落地的那一方刪除。

---

## 附錄：盤點錯誤或過時的地方，以及沒有驗證的部分

1. admin 盤點 §3(b)2 說「所有類別都要確認收件」不在 B10 的文字裡。實際上計畫:157 已經寫了。只有「要求配合擴及所有類別」是新的（D4 第 11 點）。
2. admin 盤點把 D1 寫成「KEEP REST（plan 8h-2）」，沒有指出 10e:256 的字面與它衝突，也漏了 admin 已經有通用上傳 helper 這件事。
3. admin 盤點 GAP 1 說清單的「seller type」被 SSOT 刪除，但 `CARRYOVER.md:214` 只列了 email、評分、城市；類型是 D4 第 1 點。GAP 5「嚴重度篩選」其實已由 `CARRYOVER.md:80` 決定（K11），不是缺口。
4. 停權是否下架商品（admin 盤點 §8 未驗證）：已查證。停權**不再下架**。product 的快照下架 consumer 已刪除，改成依 `seller.exposure.changed` 在讀取時隱藏（ADR 2026-10-05 決策 2）；order 仍會取消未付款單（`order/mq/SellerSuspendedConsumer.java`，解除時不還原）；coupon 會停用、解除時還原（`SellerUnsuspendedConsumer.java`）；relay 仍發 `seller.suspended`、`seller.unsuspended`（`sellerrelay/relay.go:334,341`）。舊文案在 `[id]/page.tsx:825,845,2273`。
5. 兩份盤點都漏了：`/admin/violations/[id]` 沒有入口（D5），以及「匯出」是假按鈕（D4 第 1 點）。
- 已確認正確：44 個 operation（openapi 實數 46，扣掉代行的 preflight 與 `/sellers/{store_id}/orders`）；`maxRESTOperations` 513（`openapi_coverage_test.go:27`）；`ErrorFacts` 沒有 metadata；TS twaddr 只生成到 website；`DELETE /admin/accounts` 已刪除；`delegated-operations-ui-contract` 的斷言；兩份 baseline 的條目。
- 沒有驗證：管理員的 Access token 在本機 stack 能不能用通用的 `/app/media/upload_sessions` 上傳（Banner 與食譜今天就走這條，但沒有實際跑）；`testutil_test.go` 的 mock 是否內嵌 interface；沒有跑任何 build 或 test（唯讀）。
