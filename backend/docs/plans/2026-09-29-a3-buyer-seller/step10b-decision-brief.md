# A3b 步驟 10b 開工的決定題

HEAD `3dc022e86`。兩份唯讀盤點（website 賣家中心、gateway 賣家路由，未入庫）只當線索。起草者沒有參與盤點，結論先對 SSOT：計畫 `2026-09-29-a3-buyer-seller.md:145-199`、`step10-decision-brief.md`（10b 列與附錄 A）、seller `AGENTS.md`／`CARRYOVER.md`、ADR、`api/hawkr/seller/v1alpha1/seller.proto`、API 規範與 backend 的實際檢查。

## 結論

**Will 2026-10-07：D1–D5 全照建議**，知會項 K1–K14 照做。決定的落點：計畫 10b 段、KYC ADR 文末 10b 補充。

- 盤點列了 8 個待決問題（Q1–Q8）。其中 Q3、Q4、Q5、Q6 由 SSOT 或實作層級就能決定，列在「知會」。
- 要 Will 決定的有 **5 項**：
  - D1：組織型態（Q1）
  - D2：B8 清單與既有開店進度卡（Q2）
  - D3：優惠券與發票的 KYC 閘（Q7）
  - D4：等待刪除的提示，以及 B14 的 website 半邊（Q8）
  - D5：零消費者路由的刪除時機
- backend 查證的結果：shipping、coupon、invoice 都沒有檢查 KYC。website 這三個 KYC 閘全部只存在於 client（見 D3）。

---

## 照 SSOT 直接做、不問（知會）

不同意哪一項就說。

| # | 項目 | 做法 | 來源 |
|---|---|---|---|
| K1 | 物流設定閘的門檻（Q3） | 只在 `KYC_FLOW_STATUS_APPROVED` 時開放，與「已認證」同一個定義：`exposure.missing_requirements` 不含 `KYC_APPROVED`。舊的 `overallCertificationStatus === "approved"`（`ShippingClient.tsx:110`）在 PayUni 核准後才成立，`DeriveKYCFlow` 是它的取代 | 計畫:156（B9）；`step10-decision-brief.md:224`；seller `AGENTS.md:100`；`seller.proto:211`「含 PayUni 審核」 |
| K2 | KYC 表單的「營運說明（選填）」（Q4） | 刪掉這個欄位。它只存在 `stepData` 草稿 blob（`CertificationsClient.tsx:136,194,1860-1871`），送件 body 沒有帶（`:834-863`），admin、seller-app、backend 也都沒有讀取者（`rg -i "operation_?notes\|operationNote"` 只命中 website 這個檔），所以刪掉不會丟失任何資料 | 草稿是具型別的欄位、沒有自由格式 JSON（seller `AGENTS.md:79`）；`step10-decision-brief.md:214` |
| K3 | `SellerSelf` 與代行 repo 共用介面（Q5，實作層級） | 賣家路徑直接用 proto 的 `Store`／`GetMyStoreResponse`，先例是 buyer 的 `lib/follow/repo.ts`。代行 repo（`lib/seller-data/delegated-repos.ts:203-255`，BFF 回 503）在自己的邊界轉成同一個形狀，行為不變 | 代行到 A10 不動（計畫:107）是 REAL；「保留一個 view model」是 SELF-IMPOSED，而且會變成永久多一層 |
| K4 | PayUni「核准後被拒」的文案（Q6） | `needs_reonboarding` **不是**它的替代：它代表 MerID 在 PayUni 端已經是終止狀態，只能關店重新入駐（`model/payuni.go:343`）。舊的 `approvedAt` 分支（`DashboardPage.tsx:1130-1175`、`WalletClient.tsx:203-206`）真正要回答的是「能不能自己重送」，改由 `flow.can_resubmit`、`problem.remedy`（`CONTACT_SUPPORT`／`SELLER_FIX`／`RETRY_LATER`）與 `needs_reonboarding` 決定標題和動作；不再追「曾經核准」的歷史。pending／review 的提示「可隨時前往同步狀態」（`:1162`）也要改，因為刷新按鈕已經刪除 | `CARRYOVER.md:250`；seller `AGENTS.md:100`；計畫:164（B17） |
| K5 | 「發布動態」前置條件全部移除 | 涵蓋：儀表板任務 `post`、cert 任務的 `locked: !postDone`（`DashboardPage.tsx:747-775`）、認證頁的阻擋畫面（`CertificationsClient.tsx:596,989-1011`）、送件 409 的文案（`:872`）、`cert-gate.ts:102` 的 `prerequisiteMet`、`certification-prerequisite.ts`、賣家指南的「七項任務」與「② 發布動態」（`lib/content/seller-guide-data.ts:111,129`，盤點漏了） | `CARRYOVER.md:131`（Will 2026-10-03 刪除前置條件） |
| K6 | 版本衝突的 UX | 所有帶 etag 的寫入都要處理（`UpdateMyStore`、`CompleteMyStoreOnboarding`、`SaveKYCDraft`、`SubmitKYC`、`CancelKYC`、`UpdateMyPayUniProfile`、`SubmitAppeal`）：依 `VERSION_CONFLICT` 保留使用者的輸入並顯示最新版本，不重新載入；沿用 admin 的 `submitError().kind === "conflict"` 模式。`payuni-merchant-profile.ts` 的「請重新整理後再試」與 `submitWithRetry` 刪除 | API 規範（Connect）:131、:173；`step10-decision-brief.md:237` |
| K7 | KYC 的聯絡 email 與手機 | 個人與企業都必填 `contact_email`、`contact_phone`（09 開頭）。`companyEmail` 刪除。表單照契約加欄位 | `CARRYOVER.md:108-109` |
| K8 | 商家資料（PayUni）7 欄改 5 欄 | `merAddr`／`merAddrPost` 拿掉，地址一律取店家地址 | `CARRYOVER.md:117`；`seller.proto` `PayUniRegistration` 9-13 |
| K9 | KYC 審核歷史 | 只顯示 `last_rejection`；退回要補的欄位改用 `KYCField` 的呈現層對照表 | `step10-decision-brief.md:215` |
| K10 | 申訴佐證 | 改用機敏上傳（`uploadSecureDocument`）。`SellerAppealDialog` 原本的公開上傳元件換掉 | `step10-decision-brief.md:235` |
| K11 | 違規的類別、嚴重度、處分 | 文字放在 client 呈現層的對照表；`violationDisplay.ts:14-21` 的「沒有對照表」規則作廢 | ADR `2026-09-25-codemeta-retirement.md` |
| K12 | B15 停權提示 | 在 10b 範圍內，已經同意。盤點的派工說明漏了它 | `step10-decision-brief.md:188` |
| K13 | 畫面可用的操作 | 依 `capabilities` 決定，伺服器仍逐方法檢查；`suspensionWhitelist.ts` 改成由能力推導 | `seller.proto:169-170` |
| K14 | 儀表板區塊沒有值 | 區塊未設定（含等待刪除時 `GetMyDashboard` 回 `failed_precondition`）顯示「—」，不用 `?? 0` | 計畫:159（B12） |

---

## 要 Will 決定

### D1. KYC 的組織型態 `business.company_type`（Q1）

**現況**
- 契約是自由字串（`seller.proto:602`），DB 只限制 1–50 字（`db/schema.sql:504`）。
- 送件時不是必填，`kyc_applications_complete_check` 不要求它（`schema.sql:542`）。
- **沒有送給 PayUni**：bind 請求只帶公司名稱與統編（`payuni/sync.go:249-263`）。
- 唯一的用途是 B4 與 GCIS 比對：正規化後比較文字（`model/kyc_registration.go:146,166`；ADR `2026-10-02`:133）。
- GCIS 的公司型態取自名稱結尾，有五種（`gcis.go:274`）；商業登記的型態是原文（`gcis.go:193`），值域沒有取樣（T4 只取到現況 01）。
- website 的下拉（`CertificationsClient.tsx:1468-1486`）有「獨資商號」「行號」「合作社」「其他」，前兩者跟 GCIS 的值必然比成不一致，後兩者永遠比不到。10a 留給 10b 的提醒是「選項要涵蓋 GCIS 的型態」（計畫:195）。

**業界基準**
- Stripe 的 `company.structure`、Adyen 的 `organization.type` 是封閉列舉，因為型態決定要收哪些文件。
- Middesk 這類 KYB 服務以統編向登記機關取型態，不讓申請人自填。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 維持字串，下拉換成 GCIS 的詞彙：公司五種加商業登記的獨資、合夥；拿掉其他選項；統編帶入時直接選中 | 改動最小；商業登記的值域沒有取樣，可能漏掉；仍然收一份登記機關已經有的資料 |
| (b) 改成 proto enum：伺服器把 GCIS 文字對到 enum，對不到的記「無法比對」；DB 加 CHECK | 符合「封閉集合用列舉」（`CARRYOVER.md:250`）；契約、migration、admin proto 與兩端下拉都要改 |
| (c) 刪除 `company_type`（契約加 reserved，同時刪 DB 欄位與比對欄位）；審核畫面顯示快照裡的 `organization_type` | 不收沒有用途的資料，比對也只剩有意義的欄位；GCIS 查不到時審核員看不到型態，但可以看公司登記文件（`company_documents`）或重新查詢（T3） |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| PayUni 不需要組織型態 | 事實（這條約束不存在） | `sync.go:249-263` |
| 商業登記型態的值域未知 | REAL（外部資料，沒有樣本） | 計畫:194（T4 取樣） |
| B4 比對包含組織型態 | SELF-IMPOSED（10a 自己訂的規則，系統沒上線） | ADR `2026-10-02`:133 |
| seller-app、admin 也有這個欄位 | SELF-IMPOSED（在 10c、10e 改，都在邊界內） | 計畫:197、199 |

**建議 (c)**。型態不影響任何要求或下游，業界只在型態決定要收哪些文件時才讓申請人填。做法：放在 10b 的第一個 commit（契約、migration、比對、admin proto），seller-app 與 admin 的下拉在 10c、10e 跟著刪。
**終點與觸發**：哪天型態開始影響要收的文件或 PayUni 的參數，再加回成 enum（即選項 (b)）。

### D2. B8「還缺什麼才能公開」與既有的開店進度卡（Q2）

**現況**
- 儀表板的 `OnboardingProgressCard`（`DashboardPage.tsx:418-460`，任務由 `:685-862` 組出）有七項：商店資料、發布動態、實名認證、PayUni、物流、第一個商品、食品登錄。
- 曝光條件和非曝光的任務混在一起，「商店資料」要求的也比曝光多（介紹、電話、地址明細）。
- B8（計畫:155）只定了「首頁放清單、每項連到對應頁」，沒說要不要動既有的卡。

**業界基準**：Shopify 首頁的 Setup guide 是一張清單卡，把「上線必須」和「建議」分組；Stripe 用 `requirements.currently_due` 列出待辦。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 用 `missing_requirements` 清單取代這張卡；物流、商品、食品另外做提示 | 只有一個來源；物流、商品的引導要另外找位置 |
| (b) 保留這張卡，分成兩組：「公開前必須」由 `missing_requirements` 驅動（店名、logo、所在地連 `/seller-center/me`，KYC 連認證頁並以 `flow.status` 當狀態文字，停權連違規頁）；「建議完成」放物流、第一個商品、食品登錄、商店介紹 | 一張卡，沿用既有元件；PayUni 併進 KYC 那一列（`KYC_APPROVED` 已經包含 PayUni）；版面要改 |
| (c) 另外加一張卡 | 首頁同時出現兩張進度卡，資訊重複 |

**約束帳**：「沿用既有卡的七項結構」是 SELF-IMPOSED（一致性）；UI 版面改動需要 Will 同意是 REAL（根 `AGENTS.md` UI 規則）。

**建議 (b)**。「公開前必須」一律照伺服器的推導，client 不自己判斷。
**終點**：伺服器加新的曝光條件時，清單自動出現新項目，client 只要補呈現層的文字。

### D3. 優惠券與發票的 KYC 閘（Q7）

**現況**
- website 的 KYC 閘：
  - 優惠券：`SellerCouponsClient.tsx:67,83`、`CouponFormClient.tsx:126`
  - 發票：整頁換成閘門，`SellerInvoiceOverviewPage.tsx:107,128-133`
  - 物流：`ShippingClient.tsx:110`
- backend 都沒有檢查 KYC：
  - coupon-service、shipping-service 只用 `BatchGetSellerMeta` 取店名（`coupon/.../SellerGrpcClient.java:23`、`shipping/.../SellerGrpcClient.java:35`）。
  - invoice-service 也沒有讀 KYC：`TAX_ID_MISMATCH_KYC` 沒有資料來源（`invoice-service/internal/grpc/query_read.go:826-828`）。
- 契約的能力把優惠券歸在 `MARKETING`、發票歸在 `INVOICE_MANAGE`，都和 KYC 無關（`seller.proto:183-199`）。
- 店家沒有曝光時，買家看不到它的商品，也不能結帳（ADR `2026-10-05`:34-35）。
- B9 已經拿掉商品的 KYC 閘、保留物流的閘（計畫:156）。

**業界基準**：Shopify 在收款啟用之前就能設定折扣與稅務；Stripe Connect 的 capability 只擋收款與撥款，不擋設定。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 兩個閘都拿掉，只看 `capabilities` | 與 B9 的商品同理，也跟契約的能力定義一致；優惠券在曝光之前買家看不到，不會被用到 |
| (b) 兩個閘都保留，改由 `flow.status == APPROVED` 推導 | 只是 client 的規則，伺服器不擋 |
| (c) 優惠券拿掉，發票保留 | 發票的稅務審核有自己的狀態（`taxReviewStatus`），KYC 閘和它重疊 |

**約束帳**：伺服器沒有這兩個閘，所以「舊 UI 有閘」是 SELF-IMPOSED（client 自己的規則，與現有行為一致）。沒有 REAL 的約束。

**建議 (a)**。
**終點與觸發**：B7 coupon 與發票改寫進單體時，前置條件由各自的模組回 `failed_precondition`，client 依錯誤處理。

### D4. 等待刪除的提示，以及 B14 的 website 半邊（Q8）

**現況**
- `STORE_STATUS_PENDING_DELETION`（`seller.proto:146-147`）只有一個來源：管理員 `DeleteAccount` 遇到未結訂單（admin `identity.proto:26-27`）。賣家自己刪帳號時，只要還有店就回 `failed_precondition`（`identity.proto:30`）。
- website 沒有等待刪除的畫面，Shell 只認得停權和未完成開店。
- B14（計畫:161）要 website 的帳號設定加「刪除帳號」，但 B20 的切分表只把 B14 放在 10c（`step10-decision-brief.md:189`）。website 與 buyer-app 的半邊沒有分到任何子步驟。
- website 也沒有「關閉商店」入口（`rg "關閉商店\|CloseMyStore" website` 沒有程式命中）。只有 website 的賣家照 B14 會被要求先關店，卻沒有地方能關。
- 隱私權頁寫的是「於 App 設定刪除帳號」（`PrivacyPage.tsx:177-178`）。

**業界基準**：Shopify admin 能在網頁上停用商店，Amazon Seller Central 能在網頁上關閉賣家帳號，兩者都把關店和刪除帳號分開。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 10b 一起做：加等待刪除的提示（沿用 `SellerSuspendedNotice` 的版型，文案「帳號刪除處理中，等待未結訂單完成；期間只能處理訂單」）；賣家中心設定加「關閉商店」（`CloseMyStore`，沿用確認對話框）；帳號設定加「刪除帳號」（`DeleteAccount`，有店時導向關店） | website 的流程完整；10b 多一些 UI，文案要 Will 同意 |
| (b) 10b 只做提示，B14 的 website 與 buyer-app 半邊另開一個小步驟 | 10b 範圍小；在那之前 website 沒有關店入口 |
| (c) 10b 都不做 | 管理員刪帳號後，賣家中心只會出現零散的 `failed_precondition` |

**約束帳**：「B14 只在 10c」是 SELF-IMPOSED（切分表漏列）；`DeleteAccount` 有店就拒絕是 REAL（契約，identity.proto:30）。

**建議 (a)**，並同意上面的提示文案。buyer-app 的刪除帳號放在 10c（同一套文案，seller-app 也在那一刀）。

### D5. 刪除範圍：沒有消費者的路由（B20）

B20 定的是：每一刀在同一個 commit 刪除最後一個消費者已經改完的路由（計畫:168）。目前 openapi 有 531 個操作，等於 `maxRESTOperations`（`openapi_coverage_test.go:27`，以 yaml 計數驗證）。

| 類別 | 路由 | 消費者（已驗證） | 計數 |
|---|---|---|---|
| A：只有 website 使用 | `PUT /app/sellers/me/food-business-registration`（`routes_seller.go:50`）、`PATCH /app/sellers/me/payuni/merchant_profile`（`:76`） | 只有 website `lib/seller/repo.ts` | 531 → **529** |
| A'：website 是最後一個有在用的消費者 | `POST /app/sellers/me/payuni/status_syncs`（`:74`）、`DELETE /app/certifications/:id`（`routes_admin.go:58`） | seller-app 的 `refreshPayuniStatus`（`seller_repository.dart:442`）與 `cancelCertification`（`:358`）沒有呼叫者（`rg` 只命中定義） | → **527** |
| 零消費者，屬於賣家中心這一塊 | `GET /app/sellers/me/store_visitors`（`routes_seller.go:90`）、`GET /app/sellers/me/violations/:id/appeal`（`routes_governance.go:78`） | 四個 client 都沒有（website `violations.ts:7` 只出現在註解） | → **525** |
| 零消費者，屬於 10d 那一塊 | app 的 `violation_reports` 兩條、`POST /public/food_safety_reports`、`GET /public/sellers/:id/violations/public` | 沒有 | 留到 10d（B6 的 Connect 入口在那一刀） |

**建議**：10b 刪到 **525**。
- 理由：零消費者的路由，「最後一個消費者已改完」本來就成立，而且這兩條和 10b 同屬賣家中心這一塊。
- A' 要連同 seller-app 的兩個死方法一起刪；`getCertificationApplication`（`:352`）也沒有呼叫者，順便刪掉，它打的路由留到 10c。
- 同一個 commit 要改的地方：
  - `seller_capability_test.go:42,44,119`
  - `sellerKYCAndPayuniNoStoreRouteFloor` 從 9 改成 6（`routes_kyc_payuni_cache_control_test.go:24`）
  - handler、client 方法、它們的測試
  - `legacy-endpoints-baseline.json`（數量下降也會失敗）
- legacy proto 的 RPC 留到步驟 11。
- 儀表板路由還有 seller-app 在用，照計畫留到 10c。

---

## 附錄：盤點錯誤或過時的地方

- 附錄 A 的行號引用有偏移：`overallCertificationStatus` 在 `step10-decision-brief.md:224`（盤點寫 227）、`last_rejection` 在 :215（寫 220）、`stepData` 在 :214（寫 219）、PayUni 原碼在 :219（寫 223）、治理佐證在 :235（寫 234）。
- Q6 假設 `needs_reonboarding` 可能就是「核准後被拒」的替代。實際上它代表 MerID 終止狀態（`model/payuni.go:343`），見 K4。
- Q3、Q7 列為「未查」。已經查證：shipping、coupon、invoice 都沒有 KYC 檢查；Q3 的門檻 SSOT 已經定了（K1）。
- 前置條件的清單漏了認證頁的阻擋畫面（`CertificationsClient.tsx:596,989-1011`）與賣家指南的文字（`seller-guide-data.ts:111,129`）。
- Q4 沒有指出 `operationNotes` 從來沒有隨送件送出，也沒有讀取者，所以刪掉不會丟失資料。
- Q8 寫「B14 不在 10b」是對的，但漏了 B14 的 website 與 buyer-app 半邊沒有分到任何子步驟，以及 website 沒有關店入口。
- gateway 盤點的 `rg -l merchant_profile` 也命中 buyer-app `merchant_header.dart`，那是 widget 的 Key，不是路由（盤點的分類沒有受影響）。
