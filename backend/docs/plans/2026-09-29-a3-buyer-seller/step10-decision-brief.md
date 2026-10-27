# A3b 步驟 10（client）決策簡報

HEAD `63060424e`。Will 2026-10-07 全照建議，B3、B4 採業界基準的版本（見兩份 ADR 的 2026-10-07 修訂）。起草者沒有參與盤點；四份唯讀盤點（website、admin、兩支 App、契約與 backend，未入庫）只當線索，結論先對 SSOT（計畫、ADR、seller `AGENTS.md`／`CARRYOVER.md`、API 規範）。

## 結論

- B3 改為 `StoreCard` 加 `town_code` 與評分摘要（legacy 已有批次讀取 `ListStoreRatingStats`）；B4 改為送件時查詢並存快照（下方 B3、B4 的建議段是決定前的版本）。
- 需要 Will 的有 **20 項**（B1–B20），其中 4 項是有取捨的設計決定（B1–B4，照完整格式），15 項是 UI 同意（B5–B19，精簡格式），1 項是切分（B20）。其餘約 **30 項 SSOT 已經決定**（附錄 A），照做、不問。
- 最重要的一項是 **B1**：計畫第 145 行要求在步驟 10 刪掉 REST 的 9240 處理，這和 ADR `2026-10-05-store-exposure-to-legacy.md:35`（Will 2026-10-05）衝突。建議保留到最後一條有賣家守門的 REST 路由退場，並改寫計畫第 145 行。
- 照 SSOT 必須改的契約（不需要選擇，知會）：`PublicStore` 補 `seo_title`／`seo_description`。後台 SEO 在 8e 有寫入者，公開店面卻讀不到（`api/hawkr/seller/v1alpha1/seller.proto` 的 `PublicStore` 沒有這兩欄，`internal/httpapi/seller/public.go` 也沒有對應）。這樣後台改的 SEO 沒有任何讀取者，website 的 `metadata.ts` 也會退化。
- 計畫要更正兩處，都不需要選擇：
  - buyer 的 `GetSubscriberDashboardStats` 移到步驟 11 刪除。seller-service 的 `GetSubscriberCount`（`services/seller-service/internal/client/buyer/client.go:30`）還在呼叫它，在步驟 10 刪會讓 go-checks 編不過。
  - 只為儀表板存在的 legacy 清單，補上 `Seller.GetProductViewStats`。它唯一的呼叫方是 `gateway/internal/handler/seller_dashboard.go`；gateway 的 client 方法在步驟 10 刪，proto 隨 seller-service 在步驟 11 刪。
- 盤點沒提到、但 SSOT 指派給步驟 10 的工作：
  - `X-Active-Role` 改名 `X-Perspective`：計畫第 67 行寫「最晚在步驟 10」，目前 20 個檔案仍在用。
  - 商家名單的「確認不是同一家」分支：prospect ADR `2026-10-04-prospect-contact-points-and-matching.md:48`。
  - 治理佐證改用機敏上傳：治理 ADR `:92`。

---

## 1. 前提衝突

### B1. REST 9240 處理的刪除時機

**衝突**
- 計畫第 145 行：「呼叫改成 seller 的 Connect API 時，website 與 seller-app 刪除 REST 的 9240 處理」。
- ADR `2026-10-05-store-exposure-to-legacy.md:35`（Will 2026-10-05）：9240／9241 維持，「client 依它們分支，依原因區分由 B4、B8 的 Connect `ErrorReason` 取代」。
- 現況：gateway 仍對所有留在 REST、有賣家守門的路由回 9240。範圍包括商品、匯入、優惠券、CRM、發票、物流、貼文、售後、訂單、報價、AI，約 170 處 `sellerHandle`／`requireSeller`／`sellerViewGuard`（`gateway/internal/router/seller_capability.go:47-53`；`CARRYOVER.md:206`：9d 之後 `sellerViewGuard` 仍回 9240）。Java product 上架同樣回 9240。
- 刪了會怎樣：website `lib/api/client.ts:607-611,635-637` 與 seller-app `core/networking/dio_provider.dart:196-214,262-285` 是這些 REST 呼叫唯一的「停權中」即時訊號。刪掉之後，只能等 `GetMyStore` 下一次讀取。website 有 30 秒 staleTime 的空窗，見 `SellerCenterShell.tsx:83-92`。

**業界基準**：帳號被鎖時，每個端點回同一個機器可讀的錯誤，由 client 在傳輸層集中處理。例如 Shopify Admin API 對 frozen／locked shop 一律回 402／423，Stripe 對受限帳號回同一組 `error.code`。換傳輸時，舊傳輸的處理跟著最後一個舊端點退場，不提前退場。

**既有機制為什麼存在**：舊 gateway 把停權塌縮成 HTTP 409 加 bizcode 9240（設計書 46 §7.2）。REST interceptor 是 REST 端的集中處理；Connect 端的 `sellerSuspendedInterceptor`／`onSellerSuspended` 在 8a 已經接好，兩條都導到同一個 `onSellerSuspendedSignal`／`hk-seller:suspended`。

**選項**
| 選項 | 取捨 |
|---|---|
| (a) 保留 REST 9240 interceptor，直到最後一條有賣家守門的 REST 路由退場；步驟 10 只刪 seller 模組專屬的舊碼（seller-app 的 9233 not-found、9204／9205 的刪店文案、website 的 `isSellerProfileMissing`） | 跟 ADR 一致，停權訊號不中斷；代價是兩條傳輸各有一份處理，兩者已經收斂到同一個事件 |
| (b) 照計畫第 145 行現在刪 | 少一段程式，但 B 階段之前，商品、發票等頁面被停權擋下時只會看到一般錯誤 |
| (c) 刪 REST 分支，改成每次 REST 失敗都重讀 `GetMyStore` | 每個錯誤多一次 RPC；用推斷取代明確訊號，是為了避開單一情境而加的特判 |

**約束帳**
| 約束 | 性質 | 來源 |
|---|---|---|
| 有賣家守門的 REST 路由要到 B 階段各模組搬遷時才退場 | REAL（排定在之後步驟、在邊界外的依賴） | 主計畫 B 階段；ADR 2026-10-05:35 |
| CodeMeta 凍結，9240 不能改成別的表達 | REAL（Will 的決定，ADR） | ADR `2026-09-25-codemeta-retirement.md` |
| 計畫第 145 行要求在步驟 10 刪除 | SELF-IMPOSED（寫在 ADR 2026-10-05 之前的計畫文字） | 計畫:145 |

**建議 (a)。** 把計畫第 145 行改成：「刪除 seller 模組專屬的 REST 錯誤碼處理；9240 的 REST interceptor 保留，跟著最後一條賣家守門的 REST 路由退場」。端到端驗證（試點通過條件 7）照樣在步驟 10 以 Connect 做。
**終點與觸發**：只剩 Connect 的 `SELLER_SUSPENDED` 分類器。觸發條件是 gateway 的 `sellerHandle`／`requireSeller`／`sellerViewGuard` 呼叫點歸零（最晚 B8）。在 seller `CARRYOVER.md:204` 那一列記下這個觸發條件。

---

## 2. 契約與架構

### B2. 賣家看自己店的評分與粉絲數（`GetMyStore` 沒有 stats）

**缺口**
- seller-app 的自己店面頁頁首有評分 chip（`page_header.dart:800`）與「N 位訂閱者」（`:117/540`），商品預覽有評分（`product_preview_screen.dart:398`），profile 有粉絲數（`profile_screen.dart:424`）。這些現在都打 `/public/sellers/{id}/rating|stats`。
- website 的賣家預覽頁 `seller-center/preview/[id]/layout.tsx:23` 讀公開店面。
- ADR `2026-10-03-public-store-identity-and-visibility.md:36` 規定賣家預覽自己還沒公開的店面要讀 `GetMyStore`，但 `GetMyStore` 沒有 stats。`GetStore` 只回可曝光的店，暫停的店會讀不到。

**業界基準**：Etsy Shop Manager 與 Amazon Seller Central 都在賣家後台顯示自己的評分與評論數，不論是否在架。API 形狀上，Stripe 用 `expand[]` 在資源上附加衍生資料，Google 用 AIP-157 的 read mask。

**既有機制為什麼存在**：舊的 `/public/sellers/{id}/stats|rating` 不分狀態、兩種鍵都收（`CARRYOVER.md:225`），自己與別人共用同一條讀取路徑。8c 改成只回公開的店，自己的店就沒有讀取來源了。

**選項**
| 選項 | 取捨 |
|---|---|
| (a) `GetMyStoreResponse` 加 optional `StoreStats stats`，與 `GetStore` 共用 `internal/httpapi/seller` 的組裝，平行讀、各 300ms，讀不到就不設 | 一次呼叫，預覽與公開頁同形狀；代價是每次 `GetMyStore` 多讀一次 legacy review（有上限、失敗不影響回應） |
| (b) 新增 `GetMyStoreStats` | `GetMyStore` 的熱路徑不受影響；多一支 RPC、多一次往返 |
| (c) 不改契約：可曝光時呼叫 `GetStore(store_code)`，否則隱藏 | 不改契約；暫停的店（以前有評價）看不到自己的評分，也違反 ADR:36 的預覽規則 |
| (d) 放進 `GetMyDashboard` | 停權時 `CAP_MARKETING` 會拒絕；評分不是期間指標，語意混在一起 |

**約束帳**
| 約束 | 性質 | 來源 |
|---|---|---|
| review 還在 Java，評價只能經 legacy 讀 | REAL（排定 B 階段） | `CARRYOVER.md:166` |
| 預覽自己的店讀 `GetMyStore` | REAL（Will 的 ADR 決定） | ADR 2026-10-03:36 |
| 避免增加 `GetMyStore` 的延遲 | SELF-IMPOSED（沒有量測） | — |

**建議 (a)。** 粉絲數一起放進來（buyer 同一個 DB，成本低）；website 的 `SellerSelf.rating` 沒有讀取者，刪除（盤點 U9 待實作時確認）。
**終點與觸發**：review 搬進單體時，評分改讀模組的公開 API。`GetMyStore` 的 p95 因為 review 變差時，改成 AIP-157 的 `read_mask` 按需讀取。

### B3. 店家卡片與推薦卡片缺的顯示欄位

**缺口**：`common.StoreCard` 只有 `id`、`store_code`、`name`、`logo`、`cover_image`、`status`。
- buyer-app 追蹤卡讀地址與評分（`my_subscription_screen.dart:474-587,654`）；收藏與瀏覽列的「距離」是縣市加鄉鎮（`:754,:959`）。
- website 推薦列 `MarketplaceSeller` 讀 `description`、`city`、`district`、`certified`、`prize`（`lib/api/index.ts` 約 95–120 行）；buyer-app 推薦列讀 `city`、`district`（`category_repository.dart:259-295`）。
- 改成逐卡呼叫 `GetStore` 會變成 N+1，ADR 2026-10-03:58 已經否決。

**業界基準**：Etsy 與蝦皮的店家卡片顯示店名、所在地、星等與評論數；Amazon 的賣家參照只有名稱。契約上被參照的實體只有一種形狀，衍生資料由伺服器組裝或用 expand（AIP-122 參照加 BatchGet；ADR 2026-10-03 修訂已採用）。

**既有機制為什麼存在**：舊 client 逐卡打 `/public/sellers/{id}` 拿整筆 profile，所以什麼欄位都有（A3a 第 4 輪 N8）。

**選項**
| 選項 | 取捨 |
|---|---|
| (a) `StoreCard` 加 `town_code`（seller 自己的資料，不多讀；個資清除後不設）。評分只留在已經有伺服器組裝的地方（`RecommendedSeller`、`GetStore`）；`description`、`prize`、`certified` 從卡片 UI 拿掉 | 一個欄位就補回「所在地」，這是最常讀的缺口；追蹤卡失去評分，是版面變更 |
| (b) 不改契約，UI 拿掉所在地與評分 | 改動最小；買家看不到店在哪裡，生鮮的產地與距離感是核心資訊 |
| (c) 卡片也帶評分：buyer 清單的組裝每頁多一次 legacy review 批次讀取 | 追蹤卡維持現狀；每個帶卡片的清單都多一個 Java 依賴與延遲 |

**約束帳**
| 約束 | 性質 | 來源 |
|---|---|---|
| review 在 Java，清單帶評分要多一次 legacy 讀取 | REAL（排定 B 階段） | `CARRYOVER.md:166` |
| 卡片只回「曾經公開過」的店（`store_id` 可逐號列舉） | REAL（外部約束，到 uuid 為止） | ADR 2026-10-03 修訂 |
| `certified`：只有 KYC 核准的店能公開（`model.Exposable`），所以 OPEN 卡片一律已認證 | REAL（事實，不是約束；這個欄位沒有資訊量） | seller `AGENTS.md:20` |
| 卡片欄位越少越好 | SELF-IMPOSED | — |

**建議 (a)。** website 商品頁的賣家區塊與店面頁本來就有 `store_code`，改用 `GetStore`，不受這項影響。
**終點與觸發**：review 搬進單體後，`StoreCard` 加評分，由 `internal/httpapi` 組裝；觸發條件是 review 模組完成搬遷。

### B4. 後台 KYC 審核並列 GCIS 登記資料（計畫第 109 行）

**現況**
- admin 沒有任何 GCIS 的呼叫。
- GCIS 查詢只有賣家端的 `SellerAppService.LookupBusinessRegistration`：要賣家身分、每帳號每分鐘 10 次。
- `CARRYOVER.md:264` 記著：要並列時，由 admin 的 Connect 方法呼叫同一個 `gcis` client；`:245` 寫「審核員以核准時的登記資料為準」。

**業界基準**：Stripe Connect 與 Middesk、Persona 這類 KYB 服務，在申請送出時自動比對公司登記，存下結果，審核員看到的是並列與不一致的標記，不是自己去查。

**既有機制為什麼存在**：GCIS 原本只用來帶入表單，審核員沒有來源（`CARRYOVER.md:264`），統編正確性只有格式與檢查碼把關。

**選項**
| 選項 | 取捨 |
|---|---|
| (a) admin RPC `GetKYCBusinessRegistration(application_id)`：伺服器從申請取統編、在 transaction 外查 GCIS、不快取；UI 在統編與公司地址旁並列，不一致的地方標出來 | 與 `:245`／`:264` 一致，查到的是審核當下的登記；每次開詳情就打一次外部服務 |
| (b) admin RPC 以任意統編查詢 | 變成不受限的查詢器；與申請脫鉤，沒有稽核意義 |
| (c) `SubmitKYC` 時由 job 查詢並存結果（業界基準） | 自動、可標記；要新表與 job，資料是送出時的、不是審核時的；需要改 ADR |
| (d) 不做 | 審核員繼續沒有對照來源 |

**約束帳**
| 約束 | 性質 | 來源 |
|---|---|---|
| GCIS 是外部服務、沒有 SLA，不能在 transaction 內呼叫 | REAL | backend 單體規則 5；`CARRYOVER.md:239-245` |
| 只有一位審核員、量很小 | REAL（現況） | ADR `2026-10-04-admin-roles.md` |
| 以 `CARRYOVER.md:264` 的作法為準 | SELF-IMPOSED（之前的記錄，不是決定） | — |

**建議 (a)。** UI 照既有詳情頁的版面加一欄「登記資料」，不建新元件。
**終點與觸發**：(c) 的自動比對與標記。觸發條件是第二位審核員加入，或 KYC 每日送件超過人工逐筆對照能處理的量。

---

## 3. UI 同意事項

每項列現在的文字或行為（file:line）、建議、理由。版面與流程都沿用既有元件與 token。

| # | 項目 | 現在 | 建議 | 理由 |
|---|---|---|---|---|
| B5 | 後台「刪賣家角色」入口（109） | 商家詳情只有「刪除帳號」按鈕（`admin/src/app/admin/merchants/[id]/page.tsx:1155-1166`），打已經刪掉的 `DELETE /accounts/{id}` | 加「關閉店家」按鈕與對話框（理由必填）呼叫 `CloseStore`；「刪除帳號」改呼叫 `IdentityAdminService.DeleteAccount`，成功文案分開「已刪除」與「等待未結訂單完成後刪除」（`PENDING_ORDERS`）；對話框拿掉「刪除是終局」 | Will 2026-09-30 定了後台只有這兩種刪除（計畫:118）；Amazon Seller Central 把關閉賣家帳號和顧客帳號分開 |
| B6 | 買家的檢舉與食安通報入口（109） | 兩端都沒有店家或商品層級的入口；buyer-app 只有內容檢舉 `showReportSheet` | 店面頁與商品頁的「更多」選單加「檢舉商家」（登入：`CreateReport`）；website 另提供匿名的食安通報表單（`SubmitFoodSafetyReport`）。沿用 buyer-app 的 report sheet 與 website 既有表單元件 | Etsy、Amazon、蝦皮都在店面與商品頁提供檢舉入口；8g 已開放 RPC（`CARRYOVER.md:282`） |
| B7 | 賣家能否自己改 SEO（109） | 沒有任何賣家端 UI；8a 定為只有 admin（`CARRYOVER.md:29`） | 維持只有 admin，步驟 10 不做；觸發條件是上線前的 SEO 規劃（ADR 2026-10-03:45） | 現在沒有 UI，不改就沒有遷移成本；Shopify 讓店家改，等產品定位再決定 |
| B8 | `GetMyStore` 的「還缺什麼才能公開」（109） | 兩端都沒有顯示 | website 賣家中心首頁與 seller-app profile 認證卡位置，放一張清單卡，項目照 `exposure.missing_requirements`（營運中、未停權、KYC、店名、logo、所在地），每項連到對應頁 | Shopify 的 setup guide、Stripe 的 `requirements.currently_due`；KYC 不再有內容前置條件，賣家需要知道為什麼店面沒有公開（7c 決定，計畫:97） |
| B9 | 商品建立前的 KYC 閘 | seller-app `seller_certification_gate.dart:10-20` 只在 `approved` 時開放商品建立與物流設定；website `cert-gate.ts:96` 同樣 | 商品建立的閘拿掉（與 B8 一起改成提示）；物流設定的閘照舊，資料改由 `GetMyKYC.flow.status` 推導 | ADR 2026-10-05 決策 3：「賣家可以在店家可曝光之前上架商品，買家看不到」；Shopify 允許收款設定完成前先建商品 |
| B10 | 通報狀態下拉改成指令按鈕（109） | `admin/.../violation-reports/[source]/[id]/page.tsx:347-376` 是 select 加「推進至 X」按鈕；只有食安有確認收件，嚴重度寫死 critical | 依狀態顯示按鈕：未確認→「確認收件」（選嚴重度）；已確認→「開始調查」「駁回」；調查中→「成立」「駁回」（附說明）；結案後不顯示。所有類別都要確認收件 | 契約只有三個指令，沒有通用的改狀態（8g，`CARRYOVER.md:276`） |
| B11 | 店家卡片在非營業中時的呈現（109） | website 收藏頁連到店面頁，把 not_found 當載入失敗；buyer-app 追蹤卡以數字 `store_id` 導頁（`my_subscription_screen.dart:1519-1524`） | OPEN：可點、以 `store_code` 導頁；PAUSED：顯示名稱加「暫停營業」標籤、不可點；CLOSED：「已關閉」標籤、不可點；名稱不設時：「已關閉的店家」（這句已由 ADR 2026-10-03:37 決定） | PAUSED、CLOSED 的店面頁一定 not_found，可點只會導到錯誤頁；Amazon 訂單上停業賣家只顯示名稱 |
| B12 | 儀表板文案 | website `DashboardPage.tsx:1385`「累計營收」、`:1407`「本週」、`:1438`「週環比 ±N%」；seller-app `dashboard_screen.dart:767`「累計營收」、`:1235` `_growthPrefix = '週環比'`、`:478`「關鍵數據與週環比變化」；landing `sellerFeatureOverviewData.ts:133` | 「近 7 日營收」；比較改成「較前 7 日 ±N%」（client 以 current／previous 計算，previous 為 0 或區塊不設時顯示「—」）；「本週」改「近 7 日」；卡片下方顯示期間日期（`current_period`）；`:478` 改「關鍵數據與前 7 日比較」；landing 改「核對近 7 日營收、待出貨訂單、商品瀏覽與新追蹤者」 | 8d 決定契約只回原始值、期間是最近 7 個完整日（計畫:116）；ShopifyQL 的 `COMPARE TO previous_period`、GA4 都寫「與前一期間比較」 |
| B13 | 錢包的成交餘額 | seller-app `wallet_screen.dart:1547`「可提領成交餘額」、website `WalletClient.tsx:190`「平台的成交餘額」，數字都是 dashboard 的歷來 `salesAmount` | 拿掉金額，保留「前往 PAYUNi 查看與提領」和既有說明；同步改 `seller-guide-data.ts:207` | 歷來營收本來就不是可提領餘額，契約也沒有累計值；Shopify Payments、Stripe 的餘額來自金流帳本，系統要到 payment 模組才有 |
| B14 | 自助刪帳號入口（計畫:70） | seller-app 一個「刪除帳號」按鈕打 `DELETE /app/sellers/me`（`profile_other_options_card.dart:272-319`）；buyer-app 與 website 沒有入口 | seller-app 拆成「關閉商店」（`CloseMyStore`，有未結訂單時提示）與「刪除帳號」（`DeleteAccount`；還有店時提示先關店，Apple 登入先 `RevokeAppleSignIn`）；buyer-app 與 website 在帳號設定加「刪除帳號」 | `DeleteAccount` 的契約是有店就 `failed_precondition`（identity.proto:29），client 不把兩步串成一個非原子的流程；App Store 5.1.1(v) 與 Google Play 要求 App 內能刪帳號 |
| B15 | 治理停權的提示 | `SellerSuspendedNotice.tsx:27-30,62` 與 seller-app `suspension_notice_screen.dart:144-147` 用 CodeMeta 的原因文字；治理停權沒有原因 | 管理員停權：依 `SuspensionReason` 在呈現層的對照表顯示；治理停權：「因違規處分停權，詳見違規紀錄」並連到違規頁 | 違規停權的理由在違規紀錄（seller `AGENTS.md:50`） |
| B16 | 商家名單「確認不是同一家」 | 建立撞到時沒有分支；疑似重複頁只能合併 | 撞到 `PROSPECT_MATCH_FOUND` 時，對話框列出相符的業者，讓使用者勾選「不是同一家」再送；疑似重複頁每對加「不是同一家」按鈕（`MarkProspectsDistinct`） | prospect ADR:48 指定在步驟 10 問 Will |
| B17 | 錢包的「上次同步」 | `WalletClient.tsx:280-289`「上次同步」與「資料可能已延遲，建議重新整理」 | 隨刷新按鈕一起拿掉，不顯示 `check_time` | 刷新按鈕已決定刪除（計畫:106）；沒有可以採取的動作，提示只會造成焦慮；Stripe、Shopify 都不顯示 |
| B18 | 後台 PayUni 重綁 | 今天對 `forward_failed` 再按一次「核准」 | 綁定失敗（`bind_status = FAILED`）時，在 PayUni 卡片顯示「重新綁定」按鈕，呼叫 `RebindPayUni`；`RefreshPayUniStatus`、觀測紀錄、稽核清單不做 UI，等有需求 | 核准只接受 `SUBMITTED`，不加這個按鈕就沒有路徑恢復失敗的綁定（8f） |
| B19 | buyer-app 店面頁的「已認證」徽章 | `merchant_screen.dart:265` 依 `overallCertificationStatus == 'approved'` 顯示 | 拿掉 | 能讀到店面的店一定已經 KYC 核准，徽章永遠成立、沒有資訊量（見 B3 約束帳） |

---

## 4. 切分

### B20. 步驟 10 的子步驟

**現況**
- 9d 之後，gateway 的賣家 REST 在本機回 Unimplemented：賣家中心、後台商家頁、店面頁全部不可用（計畫:143）。系統沒有部署，這只是本機開發的不便（A2 前提），不是 REAL 的約束。
- 規模約 102 個檔：website 41、admin 19、buyer-app 17、seller-app 25。
- gateway 的路由、openapi 與 `maxRESTOperations` 要在同一個 commit 跟著那條路由的最後一個消費者刪除（`openapi_coverage_test.go`）。

**業界基準**：大型遷移按「被取代的表面」切，每一刀都刪掉一組舊端點，不按 client 切（Strangler Fig；Stripe 的 API 遷移以端點群退場）。

**選項**：(a) 按 client 切成四塊；(b) 按表面切，同一組路由的所有消費者放在一起；(c) 一次做完。
(a) 讓同一組 gateway 路由跨兩個子步驟，舊路由只能等到最後才刪。(c) 超出一個 session，W7 的審查單位也太大。

**建議 (b)**，各自一個 session，每個都有 code-review 與 design-review（W7）：

| 子步驟 | 範圍 | 刪除的舊東西 | 待決項 |
|---|---|---|---|
| **10a 共用基礎** | TS 共用套件（pnpm workspace，Will 2026-09-26 已決定）；`X-Active-Role` → `X-Perspective`（計畫:67，四端加 backend 與規範）；契約補欄位（PublicStore SEO，以及 B2、B3 若採用）、`make connect`、四端重新生成 | website 與 admin 重複的 `errors.ts` 與重試 | B2、B3 |
| **10b 賣家自己（website 賣家中心）** | GetMyStore／UpdateMyStore／ApplyStore／Onboarding、KYC 精靈、PayUni、違規與申訴（機敏上傳）、儀表板、刪除 `bindSellerRole` 與刷新按鈕；9240 照 B1 | — | B1、B8、B9、B12、B13、B15、B17 |
| **10c 賣家自己（seller-app）** | 同上，另加 `hawkr_twaddr` 依賴與地址改代碼、KYC 精靈改成單例草稿（`seller_certification_upload_screen.dart` 4173 行，最大的一塊）、session bootstrap、刪店與刪帳號 | gateway 的 `/app/sellers/*`、`/app/certifications*`、`/app/businessRegistration`、儀表板路由，以及 order、crm 的儀表板 legacy RPC 與 CSV 去向 | B14 |
| **10d 公開讀取（website 加 buyer-app）** | GetStore、BatchGetStores、推薦（marketplace 改由 client 組合）、`RecordProductView`（只給訪客）、店家卡片狀態、`store_id` → `store_code` 轉址刪除、buyer-app 路由改用 `store_code` | `/public/sellers/{id}`、`/stats`、`/rating`、`/recommended`、`/marketplace`、`/products/:id/view`；seller-app 的 `page_echo_tab.dart` 也用 `/public/sellers/{id}`（還混了買家 ID），要在同一刀處理 | B6、B11、B19 |
| **10e admin** | 店家清單、統計、SEO、停權、KYC 審核（顯示全文）、治理、商家名單、會員方案頁與邀請面板移除 | gateway 的 admin sellers、certifications、governance、prospects 路由 | B4、B5、B10、B16、B18 |

順序：10a 先做；接著 10b、10c，因為賣家中心是本機壞得最多的地方；10d、10e 順序不限。buyer 的 `GetSubscriberDashboardStats` 與 `Seller.GetProductViewStats` 的 proto 在步驟 11 刪。

**約束帳**
| 約束 | 性質 | 來源 |
|---|---|---|
| 一個 gateway 路由要等最後一個消費者改完才能刪 | REAL（CI 雙向對照） | `openapi_coverage_test.go` |
| 本機的賣家 REST 不可用 | SELF-IMPOSED（沒有部署） | A2 前提、計畫:143 |
| 每個子步驟一個 session | SELF-IMPOSED（審查單位） | W7 |

---

## 附錄 A：SSOT 已經決定、不問

| 項目 | 決定 | 來源 |
|---|---|---|
| 匿名瀏覽 beacon | 只給未登入者送（website `ProductViewTracker` 以 `useRecordHistory().enabled` 分流；buyer-app 在 `!hasUsableSession` 分支呼叫 `RecordProductView`）。public transport 不帶憑證，伺服器分不出登入者 | seller `AGENTS.md:127`、`CARRYOVER.md:161` |
| `/public/marketplace`（盤點 Q10） | 聚合刪除，由 client 組合：推薦用 `SuggestRecommendedSellers`，貼文、商品走既有的 REST | 計畫:108 |
| vendor portal RFQ 的 3 條 | 留在 gateway 到 A9 | 計畫:129 |
| `payuniWebsiteUrl`（核准時） | 刪除；admin 的「PayUni 送審網址」欄位與 `platformStorefrontUrl` 拿掉 | 計畫:122、`CARRYOVER.md:271` |
| 頁碼改 keyset | 後台清單改成上一頁、下一頁的分頁 UI，不顯示總數（members 頁是範本） | 試點計畫的 client 沿用盤點（Will 2026-09-26「後台清單改分頁 UI」）；API 規範 §120-121 |
| 後台清單篩選 forwarded／forward_failed | 改成 `payuni_bind_status` 篩選；列表不帶 email、評分、城市（城市由 `town_code` 在呈現層對照） | `CARRYOVER.md:214` |
| KYC `currentStep`／`stepData`／申請 ID | 刪除；續填位置由 `missing_fields` 推導，暫存的非資料欄位由 client 保管；草稿是每帳號一筆加 etag | `CARRYOVER.md:96,249,255` |
| 賣家端的 KYC 審核歷史 | 只有 `last_rejection`；後台有 `GetKYCApplication.reviews`（舊到新，取最後一筆退回） | `CARRYOVER.md:270`；8f |
| KYC 顯示全文 | 先 `RevealKYCSensitive`（理由必填）才拿到全文與證件 `view_uri`，平常只有遮蔽值 | 計畫:123、`CARRYOVER.md:273` |
| 違規的處分種類 | 只有警告與停權；罰款、部分停權選項拿掉；`statement` 必填 | 計畫:88；governance.proto |
| FDA 轉報文件 | 伺服器回結構化資料，client 組出文件（維持現在的 `.md` 下載） | 計畫:126、`CARRYOVER.md:277` |
| PayUni 原碼（`merStatus` 等）與 `freshness` | 不對外；client 用 `outcome`、`problem.remedy`、`fields`；不認得的錯誤碼走 `remedy` | `CARRYOVER.md:250`；計畫:106 |
| 地址 city／district 改 `town_code` | website 重用 `lib/addresses/twaddr.ts` 與 `TownFields`；seller-app 加 `packages/hawkr_twaddr` 依賴，取代兩份手抄表 | 計畫:130（「seller 的兩份在步驟 10 改用」）；ADR `2026-10-04-address-town-code.md` |
| 公開店面只有所在地 | `PublicStore` 只有 `town_code`，店面頁不顯示地址明細 | 計畫:130 |
| 公開讀取的鍵 | URL 與導頁用 `store_code`；只有 `store_id` 的來源以 `BatchGetStores` 解析（回應帶 `store_code`）；`sellerPath` 的 id fallback 刪除 | ADR 2026-10-03:24,36,38 |
| 個資清除後的卡片名稱 | 「已關閉的店家」 | ADR 2026-10-03:37 |
| `overallCertificationStatus` 的資料來源 | 是否已認證：`exposure.missing_requirements` 不含 `KYC_APPROVED`；細分狀態：`GetMyKYC.flow.status` | 契約；seller `AGENTS.md:20` |
| `sellerName` | 是店名的別名（`share-card.ts:61-66` 的 fallback 鏈），對應 `store_name`，不是缺口 | 程式本身 |
| `totalProducts`（buyer-app 店面頁） | catalog 的資料，改用既有 `/public/sellers/{id}/products` 回應的總數；seller 契約不加 | 計畫:45（商品側留在 B） |
| 停權原因的顯示文字 | proto enum 加 client 呈現層的對照表；CodeMeta 凍結 | ADR `2026-09-25-codemeta-retirement.md` |
| 錢包 PayUni 刷新按鈕 | 拿掉 | 計畫:106 |
| `bindSellerRole` | 刪除 | 計畫:139,145 |
| `store_id` → `store_code` 轉址 | 刪除；真正的 404 判斷保留 | ADR 2026-10-03:38 |
| 賣家會員方案頁、商家名單邀請面板 | 移除 | 計畫:96、決定第 7 項 |
| 代行店面（website `/adm`） | 503 到 A10，不改 | 計畫:107 |
| TS 共用套件 | pnpm workspace 的 internal package 加 `transpilePackages`。觸發條件其實在 A3a 已經成立（admin 的 `BuyerAdminService` 與 website 的 buyer 都已改用 Connect），已經晚了一步，放在 10a 做 | 試點計畫:170（Will 2026-09-26）；主計畫:42 |
| `X-Active-Role` → `X-Perspective` | 最晚在步驟 10 | 計畫:67 |
| 治理佐證 | 申訴佐證改用機敏上傳 | 治理 ADR:92；計畫:126 |
| `SELLER_SUSPENDED` 端到端 | 在步驟 10 驗證 | 錯誤模型 ADR:97 |
| 後台 SEO | `UpdateStore` 必帶 etag，衝突時用 `submitError().kind === "conflict"` 顯示 | `CARRYOVER.md:29` |
| `PublicStore` SEO 欄位 | 補上 `seo_title`、`seo_description`（只有 admin 寫入、只有公開頁讀取，缺了是契約漏洞） | 契約先於實作；8e |

## 附錄 B：盤點裡錯誤或過時的說法

- website.md B8 與 apps.md 1A-B8：把 `/public/marketplace` 寫成待決的 Q10。計畫:108 已經決定由 client 組合。
- website.md 1.A.1：`sellerName` 標成要問 Will 的缺口。它只是店名的 fallback 別名。
- backend.md §4：「client need not branch on login」。public transport 是 `credentials: "omit"`，伺服器認不出登入者；SSOT 規定只有訪客送（seller `AGENTS.md:127`）。
- admin.md #14：`payuniWebsiteUrl` 寫成待決。計畫:122 已經刪除。
- apps.md §6：沒有列出 `X-Active-Role` 改名；四份盤點都沒有提到計畫:67 的期限。
- apps.md §4：只提到 seller-app 的「可提領成交餘額」，漏了 website `WalletClient.tsx:190` 的「平台的成交餘額」與 `seller-guide-data.ts:207`。
- apps.md §4 的 seller-app 行號有偏移（「累計營收」實際在 `dashboard_screen.dart:767`，`:478` 是週環比描述）。
- 計畫:145 把 buyer 的 `GetSubscriberDashboardStats` 列在步驟 10 刪除，與 seller-service 還在呼叫它衝突（backend.md 指出了，計畫沒有）。
