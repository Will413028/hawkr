# A3b 步驟 10d（公開讀取）開工的決定題

> Will 2026-10-15 同意 D1–D7 全照建議。簡報寫於 HEAD `af1b0dcc7`；之後 `local/10e` 已合進 main（`a3672bd26`），所以切分表的「時機」欄作廢：每一刀都從合併後的 main 開工，`maxRESTOperations` 由 492 起算（492 → 488 → 487 → 484 → 482）；10e 的措施檔案以 main 的現況為準。

HEAD `af1b0dcc7`；並行分支 `local/10e`（merge-base `d700a7caa`，`maxRESTOperations` 在 10e 上是 492，main 是 508）。v1 草稿與獨立審查的每一項 HIGH、MED 都回原始碼查過再處理，處理結果在文末附錄「審查意見的處理」。兩份盤點（`public-read-client-inventory.md`、`public-read-backend-inventory.md`，HEAD `af1b0dcc7`）只當線索。引用的 SSOT：計畫 `2026-09-29-a3-buyer-seller.md`、`step10-decision-brief.md`（B 項與附錄 A）、`step10b`、`step10c` brief、ADR `2026-10-03-public-store-identity-and-visibility`（含修訂）、`2026-10-05-store-exposure-to-legacy`、`local/10e` 上的 `2026-10-09-store-seo-platform-generated`、`seller.proto`、`report.proto`、`common/v1alpha1/store_card.proto`、legacy `backend/api/proto/seller/seller.proto`、seller `AGENTS.md`。

## 一頁摘要

| # | 題目 | 建議（一行） | 要同意 UX？ |
|---|---|---|---|
| D1 | 各處「進店」入口與店面 404 | 依來源分：商品、貼文的連結有伺服器端可見性保證（商品由 Java、貼文由 D2），直接用 REST 的 `storeCode`；聊天、訂單、購物車、優惠券、通知沒有狀態，每個畫面一次 `BatchGetStores`，依 B11 呈現；店面 not_found 用一頁不分原因的 404；店家還不能公開時賣家端的分享停用 | 是 |
| D2 | 不可曝光店家的公開內容（全站貼文 feed、貼文、店家身分的留言、討論區、評價）與聊天入口 | 規則與商品相同：店家此刻可曝光才出現在公開面。social 比照 ADR 2026-10-05 對 Java 的作法，消費 `seller.exposure.changed`，在讀取的 SQL 過濾；評價由 gateway 檢查父資源；買家開啟聊天只限「曾經公開過」的店（StoreCard 的規則）。要修訂 ADR 2026-10-05 | 否 |
| D3 | 食品業者登錄字號的公開揭露（**新功能，不是 10d 造成的退步**） | 建議在 10d 一起做：`PublicStore` 加 OUTPUT_ONLY 的 `food_business_registration_number`（不公開 `processed`），商品頁的店家區塊與店面頁的商家資訊各加一列；也可以另開一項 | 是 |
| D4 | B6 檢舉入口與食安通報 | 店面頁、商品頁各加「⋯」選單（website `Dropdown`＋`Dialog`，App 用 bottom sheet）；檢舉表單另做，**不沿用 `showReportSheet`（推翻 B6 的這半句）** | 是 |
| D5 | 店家 sitemap | `SellerPublicService.ListStores`（AIP-132），`PublicStore.update_time`；先輸出單一 sitemap，可曝光店家數超過 45,000 時改分片 | 否 |
| D6 | 賣家預覽與店主自己的讀取 | 公開路由對所有呼叫者回一樣的內容，不加店主例外；預覽改讀店主自己的來源（`GetMyStore`、`/app/sellers/products`、`/app/sellers/me/posts`），seller-app 的聊天商品選擇器與商品預覽同樣改讀店主端點 | 是 |
| D7 | buyer-app 店面路由參數（buyer-app `AGENTS.md` 要求確認路由改動） | `GoRoute('/merchant/:sellerId')` 改成 `/merchant/:storeCode`：路徑字面不變，參數名與 `MerchantScreen` 建構子一起改 | 是（路由） |

**要 Will 同意的 UX**（文案草稿都在各 D 內）：
- D1：店面 404 頁；賣家端分享停用的提示。
- D3：「食品業者登錄字號」一列的位置。
- D4：「⋯」選單的位置；檢舉表單；website 未登入時的分流；食安通報表單；成功與錯誤訊息。
- D6：預覽的橫幅；店家還沒公開時，評價與迴響分頁的空狀態（website 預覽與 seller-app 的迴響分頁）。
- D7：路由參數改名。
- K15：統計讀不到時顯示「—」，從儀表板延伸到店面頁頂部（延伸既有的核准，不同意請說）。

**推翻或修正既有的核准**：
1. **B6**「沿用 buyer-app 的 report sheet」：被 D4 推翻。B6 核准時還沒比對兩份契約（原因清單、說明必填、佐證、端點都不同）。
2. **計畫:164**「匿名瀏覽只給未登入者送 `RecordProductView`」：改成兩端都送、由伺服器判斷。契約已經寫明「不必讓 client 判斷登入狀態」，伺服器也這樣實作（K9）。這是照「契約先於實作」修正計畫的字句，不是推翻 Will 的決定。
3. **計畫:164**「`PublicStore` 補 `seo_*`」：10e 的 ADR 2026-10-09 已經推翻，10d 照 10e 做（K3）。
4. **ADR 2026-10-05** 的影響範圍擴大到 social（D2）：這是修訂，決策本身不變。
5. 不屬於推翻、但要知道的範圍：10e 刪掉 `violationDisplay.ts` 與 `violation_display.dart`，類別標籤搬到 `measureDisplay.ts` 與 `measure_display.dart`（D4 改引用新檔）；10e 也把 twaddr 搬進 `@hawkr/twaddr`。

**切分**：只有 seller-app 那一刀（第 1 刀）和 10e 完全沒有重疊，可以在 10e 合併前做。其他 6 刀都排在 10e 合併之後。`maxRESTOperations` 的變化：main 上 508 → 508；10e 合併後 492 → 492（契約）→ 488（gateway＋social）→ 487（buyer-app）→ 484（website 非店面）→ **482**（website 店面）。

---

## 照 SSOT 或已核准的決定直接做（知會）

不同意哪一項就說。v1 改過的列標 ★。

| # | 項目 | 做法 | 來源 |
|---|---|---|---|
| K1 | 零消費者的治理路由 4 條 | 刪 `GET /public/sellers/:id/violations/public`、`POST /public/food_safety_reports`、`POST /app/violation_reports`、`GET /app/violation_reports/me`，連同 handler、只給它們用的 DTO、4 個 governance gRPC wrapper、`routes_governance.go` 的路由註解。已驗證沒有消費者：`rg -n "violation_reports\|food_safety_reports\|violations/public" website buyer-app/lib seller-app/lib` 沒有命中。★ 排在 10e 之後，因為 10e 也改 `routes_governance.go` 與 `governance_client.go` | 計畫:202；`step10b-decision-brief.md:162` |
| K2 | 被刪的 6 條核心路由 | `/public/sellers/{id}`、`/stats`、`/rating`、`/recommended`、`/public/marketplace`、`POST /public/products/:id/view` | `step10-decision-brief.md:190` |
| K3 | 店面的 SEO | 照 10e ADR：title、description 由 `lib/seo/store-summary.ts` 從 `PublicStore`（`town_code` 經 `@hawkr/twaddr/lookup` 轉換）與 `StoreStats` 生成 | ADR 2026-10-09「後果」 |
| K4 | URL 與導頁的鍵 | 公開網址一律用 `store_code`；刪除 `sellerPath`、`sellerCanonicalUrl` 的 id fallback，以及沒有呼叫者的 `sellerCardPath`。`store_code` 在一個 stint 內不變（UNIQUE＋CHECK，`seller/db/schema.sql:82,106`，沒有任何 UPDATE 改它），所以訂單快照的 `storeCode` 不會過期 | 附錄 A:222；ADR 2026-10-03 |
| K5 | `store_id` → `store_code` 轉址 | 刪 `lib/seller/canonical-redirect.ts`、`isSellerKeyShape` 防線、e2e 的 308 斷言、`sellers/[id]/products/[pid]` 舊網址 | 附錄 A:230 |
| K6 | 真正的 404 | middleware 的 `/sellers/` 分支改查 `GetStore(store_code)`：`not_found`、`invalid_argument` 改寫成 404，其他錯誤放行。形狀檢查改成 `stores_store_code_check` 的範圍 | 附錄 A:230 |
| K7 | 店面各分頁的參數 | 從 `GetStore` 取 `store.id`，傳給留下的 REST（products、posts、reviews、forum）與追蹤 | ADR 2026-10-03（URL 用 code，API 參照用 id） |
| K8 ★ | 賣家預覽的網址 | `/seller-center/preview` 不帶 id，刪掉 `/seller-center/preview/[id]`。資料來源見 D6 | ADR 2026-10-03:36；B2 |
| K9 ★ | 匿名瀏覽 beacon | website `ProductViewTracker` 與 buyer-app 商品頁**無條件**呼叫 `SellerPublicService.RecordProductView`，fire-and-forget，吞掉 `resource_exhausted` 與 `invalid_argument`。登入狀態由伺服器判斷：`httpapi/seller/public.go:152` 對有 principal 的呼叫直接回成功；website 經同源反代會帶 Bearer 或 Clerk cookie。`useRecordHistory` 不改。刪掉 `middleware/ratelimit.go` 的豁免與測試 | `seller.proto` RecordProductView 註解（「也不必讓 client 判斷登入狀態」） |
| K10 | marketplace 改由 client 組合 | 推薦店家改用 `SuggestRecommendedSellers`；熱門商品改用 `GET /public/products?sort=SALES`；`latestPosts` 沒有消費者，不組。首頁 bootstrap 刪掉 `marketplace` domain，`schemaVersion` 直接改。順序會從「最新」變成真的依銷量：gateway 送的 `sales_desc` 被 Java 的白名單落到 `created_at DESC`（`marketplace.go:116`、`ProductRepository.java:176-181`），現在的行為是 bug | 附錄 A:209 |
| K11 | 推薦店家在 client 重排 | 刪除 `homeSnapshot.ts` 的 `sellerQualityScore`，照伺服器的順序 | ADR `2026-10-03-recommended-sellers-scoring`；B3 |
| K12 | 卡片的欄位與狀態 | 推薦 rail、追蹤、收藏、瀏覽紀錄、會員卡、聊天標頭改讀 `StoreCard`，依 B11 呈現；拿掉 certified、簡介、得獎；所在地由 `town_code` 經 twaddr 轉換（website 用 10e 的 `@hawkr/twaddr`）。刪除 `MarketSellerCard`、`latitude/longitude` 的讀取、buyer-app 追蹤頁的逐店補讀 | B3、B11 |
| K13 | 店面頁頂部的欄位 | 店面頁不是卡片，B3 不適用：簡介、得獎、官網照 `PublicStore` 顯示；地址只到縣市與鄉鎮市區 | `seller.proto` `PublicStore` 註解；附錄 A:221 |
| K14 | 「已認證」徽章 | buyer-app 刪除 | B19 |
| K15 ★ | 統計讀不到時 | `StoreStats` 欄位沒有設定時，店面頁頂部那一格顯示「—」；卡片上的評分沒有設定時整段不顯示。**這是把 B12 與 10c K19 對儀表板核准的「—」延伸到店面頁**，不同意請說 | B12；10c K19 |
| K16 | 商品頁的店家區塊 | 改成讀 `GetStore(商品 REST 的 seller.storeCode)`：商品看得到就代表店家可曝光，所以讀得到。`storeCode` 空著時不顯示進店連結 | ADR 2026-10-05 決策 2 |
| K17 | 商品清單逐店補店名 | 刪掉 buyer-app 的 `fetchSellerInfo`、`_hydrateSellers`：REST 本來就帶店名與 logo | ADR 2026-10-03 修訂 |
| K18 ★ | 討論區留言者的頭像（buyer-app `merchant_chat_tab.dart`、seller-app `page_echo_tab.dart`／`getUserProfileMap`） | 刪掉以 `userId` 打 `/public/sellers/{id}` 的退路。**理由（v1 寫錯了）**：`userId` 依留言者的角色是 `buyer_id` 或 `store_id`（`cms_seller_forum.role` 的欄位註解、`seller_forum.go:193-204`），但 `ForumRecord` 沒有角色欄位，client 分不出 id 屬於哪個命名空間。另一方面，social 寫入時已經依角色解析 `userName`、`avatarKey`，紀錄本身就是權威。否決的另一個做法是在 forum DTO 加作者類型：那要改 A7 的 social，而且沒有消費者需要它。seller-app 的 `AuthStorage.saveUserProfileFor/getUserProfileFor` 一起刪 | backend `AGENTS.md`「三種 id 不可互換」 |
| K19 | buyer-app 通知的寄件者 | 只取店家鍵，用 `BatchGetStores` 批次解析；帳號鍵刪除 | 同上 |
| K20 ★ | （移到 D7） | — | — |
| K21 | 分享連結 | gateway `getPostShareURL` 解析不到 `store_code` 時回錯；`type=seller` 分支沒有消費者，刪除；三處 client 的退路改用 `storeCode`；website `buildSellerShareUrl` 刪 id fallback | K4、K5 |
| K22 | App 下載橫幅的 deep link | `deepLink` prop 早就沒有作用，與兩個呼叫點一起刪 | 根 `AGENTS.md`「不留技術債」 |
| K23 | `/stats` 的 `totalProducts` | buyer-app 沒有畫面顯示它，刪除 | `merchant_screen.dart:201` |
| K24 ★ | website 快取 | `GetStore`、`BatchGetStores`、`SuggestRecommendedSellers` 的 server 讀取包成與 `getCachedSeller` 等價的 wrapper，tag 是 `seller:<store_code>`。review-service 與單體 seller 都同時送 `seller:<id>` 與 `seller:<code>`。審查 L3 問 id tag 是否還有讀者：有，商品與評價這些仍以 `store_id` 為鍵的 REST wrapper 掛 `sellerTag(id)`（`cached.ts:365,700` 等），所以保留 | 原始碼 |
| K25 ★ | gateway 的連帶清理 | 每刀刪掉沒有呼叫者的 gRPC wrapper 與 `rpc-calls.csv` 的 gateway 列；`enrichPostsWithSellerInfo` 搬到 `post.go`。**新增**：D2 要一個 `BatchGetSellerMeta` 的 gateway wrapper（目前沒有：`rg BatchGetSellerMeta backend/gateway/internal` 沒有命中），`rpc-calls.csv` 加對應的 gateway 列；legacy `SellerMeta` 加一個欄位（D2） | 10c K24；先例 `af1b0dcc7` |
| K26 | 設計書與契約註解 | 28、53、45、02 §5.13c、65 的對應段落；`discovery.proto:13-17` 的註解與 10a T2 對齊 | 計畫:299 |
| K27 ★ | 防回流 | buyer-app 的 `dead_gateway_endpoint_guardrail_test.dart` 加上被刪的路由；website e2e 改用 `sellerMethod()` 的 Connect mock；buyer-app 的 widget 與 hermetic 測試比照既有 Connect client 測試的 fake 方式（盤點 G13，實作時找出既有作法，不另建一套） | 10c 先例 |
| K28 ★新 | 產地地圖的商家預覽（`OriginSellerPreview.tsx`） | `previewStoreIds` 改用 `BatchGetStores` 解析名稱、logo、code，依 B11 呈現。現在是從商品視窗借 code（`aggregateOriginCounts.ts:130-136`），商品不在視窗裡時名稱變「查看商家」、連結退回 id，K4 刪掉 fallback 之後就沒有連結 | B11；K4 |
| K29 ★新 | 「附近商家」排序與產地地圖分組 | rail 的縣市名改由 `town_code` 經 twaddr 轉成縣市名，`FeedStories` 以 `normalizeCity` 對上 `TAIWAN_CITY_CENTERS`（加上臺／台的單元測試）。這裡不會有代碼空間混用：排序只用名稱。產地地圖的分組讀的是 legacy `/public/seller-cities/summary` 與商品 REST 的 `seller.city`，資料不來自 seller Connect，10d 不動；觸發條件是 B（catalog）搬遷 | 盤點 Q8 |
| K30 ★新 | 聊天的 `merchantId` | buyer-app `chat_thread_screen.dart:456,477` 的 `merchantId` 是 `store_id`：gateway 的 `resolveChatTargetID`（`chat.go:147`）把買家的對象解析成 `store_id`。對話列表已經帶 `storeName`、`logoKey`、`storeCode`（im 經 `GetSellerBatch` 補上），經 route extra 傳入；狀態由 D1 的 `BatchGetStores` 提供 | 審查 L5 |
| K31 ★新 | buyer-app 的死碼 | 刪除 `SellerRepository.getSellerProducts`（零消費者，盤點 G12） | 根 `AGENTS.md`「不留技術債」 |

---

## 要 Will 決定

### D1. 各處「進店」入口與店面 404

**現況**
- 舊的 `/public/sellers/{id}` 回所有沒關的店。`GetStore` 只回可曝光的店（ADR 2026-10-03:36），所以 10d 之後暫停、未通過 KYC 的店面一律 404。
- 已經有狀態的入口（追蹤、收藏、瀏覽紀錄、會員卡、推薦）由 K12 處理。
- 沒有狀態的入口依資料來源分成三類：
  - **伺服器端已經保證可見性**：商品卡與商品頁（Java 在讀取時隱藏不可曝光的店的商品，ADR 2026-10-05 決策 2），以及 D2 之後的貼文。
  - **伺服器端已經補資料、但沒有狀態**：聊天。im 經 legacy `GetSellerBatch` 補店名與 logo（`im-service/internal/client/seller/client.go:50`），這支不套用曝光規則。
  - **Java 或 legacy 的快照、沒有補資料**：訂單（`order.go:58,148`，`omitempty`）、購物車（`cart.go:60`）、優惠券、通知。
- 賣家自己的分享：seller-app `page_home_feed_screen.dart:258` 與 website 賣家中心的分享卡。店家還不能公開時，分享出去的連結會 404。

**業界基準**
- 探索與店面只放公開的店；訂單、聊天這類參照在對方停業後仍顯示名稱並標上狀態。Etsy、Amazon 的訂單在賣家停業後仍顯示賣家名；Amazon 對停業賣家只顯示名稱、不給連結。
- 參照卡片由伺服器內嵌在回應裡（GraphQL 欄位展開、Stripe `expand[]`、AIP-122 參照加 BatchGet；ADR 2026-10-03 第一則修訂已經採用）。
- 不存在與不公開的店面回同一個頁面，不透露原因（ADR 2026-10-03:36）。

**既有機制為什麼存在**：舊端點同時是店面、卡片的補資料與鍵的解析，不分狀態，所以 client 連到哪裡都打得開。「可曝光」是 7c 才有的概念，legacy REST 從沒帶過它。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 所有來源都直接用 REST 的 `storeCode`，打不開就落到 404 | 不多打任何呼叫；但暫停或關閉的店仍然可點、點了是錯誤頁，違反 B11「暫停、已關閉不可點」；`storeCode` 空著的訂單沒有連結 |
| (b) **依來源分**：有伺服器保證的（商品、貼文）直接用 `storeCode`；聊天、訂單、購物車、優惠券、通知每個畫面一次 `BatchGetStores`（最多 100 個 id），依 B11 呈現 | 商品與貼文的連結都有效，只在可曝光翻轉後最多一個對帳週期內（`seller.reconcile_exposure` 每 15 分鐘）可能落到 404；其他來源每個畫面多一次往返 |
| (c) 讓伺服器在所有來源的回應裡內嵌 `StoreCard` | 最接近基準。聊天這一處可以做到：把 legacy `GetSellerBatch` 換成卡片語意，再帶上狀態。但這要改 legacy proto、im-service 的對應與 gateway 的聊天 DTO 三層，換來的只是省掉一次批次讀。訂單、購物車、優惠券、通知是 Java 或 legacy 的快照，gateway 沒有補資料的機制，要新增 enrichment |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 訂單、購物車、優惠券、通知的 REST 沒有店家狀態，gateway 也沒有替它們補資料 | REAL：依賴在邊界外，直到各模組搬遷；在這些 handler 新增 enrichment，搬遷時整段丟掉 | backend `AGENTS.md`「遷移期間有兩套程式碼」；主計畫 A4–A7、B |
| 聊天的補資料在 im（legacy），改成卡片語意要跨三層 | REAL：im 在邊界外。legacy proto 本身可以改（ADR 2026-10-05 的前提），但 im 的對應與 gateway DTO 改了就是丟棄的工作 | 同上 |
| 商品看得到 ⇒ 店家可曝光；D2 之後貼文也一樣 | REAL（事實），落後最多一個對帳週期 | ADR 2026-10-05 決策 2；D2 |
| `store_code` 在一個 stint 內不變 | REAL（事實） | `schema.sql:82,106` |
| 不可曝光的店面不能讀 | REAL（已核准） | ADR 2026-10-03:36 |

（v1 列在這裡的「每畫面多一次 `BatchGetStores`」是代價，「404 文案要同意」是流程要求，兩者都不是把建議推離基準的約束，已經移出。）

**建議 (b)**。基準是伺服器內嵌卡片；商品與貼文已經由伺服器保證可見性；其他來源因為依賴在邊界外（REAL），改由 client 每個畫面批次讀一次。
- 訂單、購物車這些 `storeCode` 有時空著的來源，一律以 `BatchGetStores` 的結果為準。
- 賣家自己的店：以 `GetMyStore` 的 `PUBLIC_LISTING` 能力判斷（ADR 2026-10-05 的 2026-10-08 修訂；seller-app `myStore.can(...)`、website `lib/seller/store.ts`），不能公開時停用分享。
- **要同意的文案**：
  - 店面 404（website 新增店面專用的 not-found，middleware 改寫到它、回 404；buyer-app 取代現在的空狀態）：
    - 標題：「找不到這家店」
    - 說明：「這家店可能暫停營業、已經關閉，或網址有誤。」
    - 按鈕：「回首頁」（website）、「返回」（App）
  - 參照入口：沿用 B11 已核准的「暫停營業」「已關閉」「已關閉的店家」，沒有新文案。
  - 賣家的分享（seller-app 貼文與店面分享、website 賣家中心的分享卡）：按鈕停用，並顯示「店家公開後才能分享店面連結」。

**終點與觸發**：order、cart、coupon、notification、im 各自搬進單體時，Connect 回應內嵌 `StoreCard`，client 刪掉該畫面的 `BatchGetStores`。觸發條件是各模組的搬遷步驟。

### D2. 不可曝光店家的公開內容與聊天入口

**現況**（v1 只看到 `/:id/*` 子路由，漏了全站 feed，對洩漏面的描述也不準，這裡重寫）
- **全站貼文**：`GET /public/posts`（`post.go:29-55`）與 `/public/posts/:id` 回所有店的貼文。
  - gateway 的 `enrichPostsWithSellerInfo` 走 legacy `GetSellerBatch` → `seller.BatchStores`，後者註明「不套用曝光規則」（`profile/profile.go:596`）。
  - social 自己也以 legacy `GetSeller`／`BatchGetSellerMeta` 填店名（`social-service/internal/client/seller/client.go:35,56`），只濾掉已關閉的店。
- **誰能發貼文**：`CapMarketing` 只要求營運中、沒有停權、沒有等待刪除（`model/capability.go:80`），不要求可曝光。所以 KYC 還沒核准的店可以發貼文，貼文會出現在全站 feed，帶店名、logo 與 `storeCode`，連結指向 404 的店面。
- **停權的店**：停權當下 `CapMarketing` 失效，不能再發新貼文，但已經發出的貼文留在 feed。商品這邊，Java 已經把它們隱藏了（ADR 2026-10-05）。所以因食安違規停權的店，商品不見了，行銷貼文還在推。
- **以店家身分留言**：任何店都能在**別家**貼文底下留頂層留言（`comment.go:115-146` 只限制回覆），留言快照帶店名。
- **討論區**：`/:id/forum` 與 `/forum/:id/replies` 直接以 id 讀，不經任何店家檢查；`/forum/:id/replies` 的路徑上沒有店家 id。
- **評價**：`/:id/reviews`、`/:id/reviews/stats` 經 resolver，只濾掉已關閉的店。從沒公開過的店不可能有評價：結帳要 `can_accept_orders`（＝`Exposable`，ADR 2026-10-05 決策 3）。所以問題只在後來失去可曝光的店。
- **聊天**：買家的 `SendChatMessage` 經 `resolveChatTargetID`，只濾掉已關閉的店。對話列表由 im 經 `GetSellerBatch` 補店名。這是**唯一真正能讓第三方逐號讀出沒公開過的店名的路徑**：對連號的 `store_id` 送一則訊息，再讀對話列表。`StoreCard` 的註解把聊天列為卡片的使用處，規則是「沒公開過的店名不能被讀出」。
- **v1 說錯的地方**：v1 說「沒公開過的店的評價、貼文與店名可以逐號讀到」。實際上評價不存在；貼文與討論區只會帶出店家**自己發布**的內容。洩漏的對象不是第三方列舉，而是違反「公開面只放可曝光的店」這條規則。
- 商品不受影響：Java 已經在讀取時隱藏不可曝光的店的商品，包括 `/:id/products`。

**業界基準**
- 店家的公開內容跟著店家的可見性走，商品與行銷內容一起隱藏：Etsy 休假模式與停權的店，商品與店面一起隱藏；Facebook 粉專取消發布後，貼文從動態消息消失；Amazon 的 listing suppression 不改商品本身，只在讀取時隱藏。
- 可見性在讀取時，由內容擁有者以餵進來的店家狀態判斷（event-carried state transfer，Fowler〈What do you mean by "Event-Driven"?〉）。不在先分頁之後才過濾：先分頁再過濾會讓頁數與總數都不準。
- 子集合的讀取跟著父資源：看不到父資源就回 not found（Google AIP-211、AIP-193；GitHub 私有 repo 的子資源一律 404）。

**既有機制為什麼存在**：social 與 resolver 是在公司時期寫的，當時店家狀態只有「關閉」與「停權」兩種，停權靠 product 的快照下架；「可曝光」在 7c 才出現。ADR 2026-10-05 把它帶進了 Java，但沒有涵蓋 social、review 與 im。

**選項**

可見性的規則：
| 規則 | 取捨 |
|---|---|
| (R1) **店家此刻可曝光**，它的貼文、討論區、店家身分的互動與評價才出現在公開面 | 與商品的規則（ADR 2026-10-05）相同，也就是同一個 `Exposable`；停權的店的行銷內容會跟著消失 |
| (R2) 曾經公開過就可以出現（卡片的語意） | 只擋住從沒公開過的店；停權的店，商品不見了、貼文還在，兩條規則不一致 |

貼文這類跨店清單的作法：
| 作法 | 取捨 |
|---|---|
| (a) **social 消費 `seller.exposure.changed`**，記下每家店可不可曝光，在公開讀取的 SQL 上加條件（feed、依 hashtag、單店貼文、貼文詳情、討論區與回覆），熱門 hashtag 的計數也用同一個條件 | 與 ADR 2026-10-05 給 Java 的作法相同：分頁與計數都準。seller 端的事件、relay 與全量重新公告都已經存在（9c、9d）。social 目前只有 publisher、沒有 consumer，要加一個 consumer、一張表與一個 migration |
| (b) gateway 拿到 social 的結果之後再過濾 | 先分頁後過濾，頁數與總數都不準；ADR 2026-10-05 已經對 Java 否決過同一個作法 |
| (c) 只把 enrichment 改成卡片語意（審查的建議） | 擋住了 gateway 帶出的店名，但 social 自己也填店名，而且貼文內容本身照樣出現在 feed，R1、R2 都沒有達成 |
| (d) 發文改成要求 `PUBLIC_LISTING` | 擋住了沒公開過的店發文；但停權或失去可曝光之後，舊貼文還在。而且和商品「可以在公開前先準備」不一致 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| social、review、im 在邊界外，要到 A7、A6 與 im 的搬遷步驟 | REAL。但它推的方向是「seller 端完整做，legacy 只做最小的過濾」，不是「不做」。ADR 2026-10-05 對 Java 已經這樣判過：可見性的漏洞不能等 | 主計畫 A6、A7；ADR 2026-10-05「考慮過的方案」 |
| 先分頁後過濾會讓頁數不準 | REAL（技術事實），排除 (b) | ADR 2026-10-05 |
| 買家要能就訂單售後聯絡暫停中的店 | REAL（產品語意；卡片的規則涵蓋聊天） | `store_card.proto:20-24` |
| `store_id` 在過渡期連號、可列舉 | REAL（外部，到換成 uuid 為止） | ADR 2026-10-03 繼承約束 |
| 可曝光翻轉之後，最多落後一個對帳週期或一次事件傳遞 | REAL（事實），與商品相同 | ADR 2026-10-05「後果」 |
| social 現在沒有 consumer | SELF-IMPOSED（工作量） | — |
| 「等各模組搬遷時再處理」 | SELF-IMPOSED（計畫的分段） | — |

**建議 R1＋(a)**，依內容的擁有者分開做：
- **social**（貼文、討論區、店家身分的互動）：
  - 加 `seller.exposure.changed` 的 consumer，比照 Java 的 `SellerExposureChangedConsumer`：依版本寫入、丟棄舊版本、可以重送。上線時由 seller 全量重新公告一次（9d 的作法）。
  - 公開讀取的 SQL 加「店家可曝光」的條件：`ListPosts`／`CountPosts`、`ListPostsByHashtagName`／`CountPostsByHashtagName`、`GetPost` 的公開路徑、討論區的列表與回覆；熱門 hashtag 與 hashtag 的貼文數改用同一個條件計算，比照 ADR 2026-10-05 勘誤對 Java hashtag 計數的處理。
  - 店家本身仍然可以在公開前先寫貼文，公開後才出現，與商品一致。
  - 寫入：一家**不可曝光**的店，不能在別家的貼文留言，也不能在別家的討論區留言；別人也不能在不可曝光的店的討論區留言。店家在自己貼文底下的回覆照舊。這樣不用在讀取時過濾留言，也就不會讓 `comment_count` 跟顯示的不一致。
  - 貼文都來自可曝光的店之後，gateway 的 `enrichPostsWithSellerInfo` 不必改語意（K25 只是搬檔）。
- **review**（`/:id/reviews`、`/:id/reviews/stats`）：gateway 以新的 `BatchGetSellerMeta` wrapper 檢查父資源，`can_accept_orders = false` 時回 404，與 `GetStore` 的 not_found 同義。單一父資源，沒有分頁的問題。
- **聊天**：買家傳訊的對象必須「有卡片、而且沒有關閉」，也就是曾經公開過。legacy `SellerMeta` 加一個欄位（例如 `has_public_card`），由 `seller.StoreCards` 的同一條規則算出（`first_exposed_at` 有值，或此刻可曝光）。改 legacy proto 是 ADR 2026-10-05 的前提允許的；`resolveChatTargetID` 改讀它。暫停中的店照樣可以聯絡。
- resolver 的 store_code 分支、過期的註解、swag `@Param`、mock 測試，照現況改寫（只收 `store_id`）。
- **公開路由不依呼叫者變化**，沒有店主例外。店主要讀的東西走 D6 的店主端點，所以不會有快取把店主才看得到的內容發給別人。
- **ADR**：修訂 ADR 2026-10-05，影響範圍加上 social（consumer 與過濾）、gateway（評價、聊天）與 legacy `SellerMeta` 的新欄位；後果段寫明 social 端的對應物在 A7 刪除。
- **測試**：
  - 不可曝光的店的貼文不在 feed、依 hashtag 的清單、單店貼文裡，總數也不算它；貼文詳情回 not found。
  - 店家重新可曝光之後，貼文回來。
  - 舊版本的事件不會覆蓋新版本。
  - 不可曝光的店在別家貼文留言被拒。
  - 失去可曝光的店，讀它的評價回 404。
  - 買家對沒公開過的店傳訊回 404，對暫停中、曾經公開過的店傳訊成功。

**終點與觸發**：A7 social 搬進單體時，social 模組直接消費 `StoreExposureChanged`（或經 seller 的公開 API），legacy 的 consumer 與表刪除。A6 review 與 im 搬遷時，gateway 的檢查隨路由刪除，legacy `SellerMeta` 的新欄位在步驟 11 隨 legacy proto 刪除。`store_id` 換成 uuid 之後，列舉的風險消失，但 R1 的規則不變。

### D3. 食品業者登錄字號的公開揭露（新功能）

**範圍先說清楚**：10d 之前，前台**沒有任何畫面**顯示登錄字號（`rg -n -i "foodBusinessReg\|registrationNumber" website buyer-app/lib`，排除賣家中心與 KYC，沒有命中）。所以這不是 10d 造成的退步，而是一項新功能。Will 可以選在 10d 做，也可以另開一項。我建議在 10d 做，理由是邊際成本低：K16 本來就要重寫商品頁的店家區塊，契約那一刀本來就要改 `PublicStore`。

**現況**
- 舊 DTO 的註解寫「法定應公開揭露」（`handler/seller.go:67`），但沒有畫面讀它。
- `PublicStore` 刻意不帶。資料在 `Store.food_business`（`registration_number`、`processed`），個資清除時保留，因為它是公開的登記號（`CARRYOVER.md:132`）。
- 平台自己的〈使用者政策〉§十要求賣家「於商品頁面明確標示……食品業者登錄字號」（`UserPolicyPage.tsx:266-269`）。

**法規（已查證的部分）**
- 《食品安全衛生管理法》第 8 條第 3 項：公告類別與規模的食品業者要登錄才能營業，範圍包括有工廠、商業或公司登記的販售業者。
- 衛福部〈以通訊交易方式訂定之食品或餐飲服務定型化契約應記載及不得記載事項〉第三點（104-09-30 施行，法源是消保法第 17 條）：應登錄的企業經營者，要在**通訊交易契約**中載明登錄字號。地方衛生局會查核（臺北市 2020 年查到 4 家未揭露）。
- 義務的主體是賣家（企業經營者）。沒有查證到明文規定「平台」必須顯示。

**業界基準**：平台以結構化欄位收下法定的公開識別資訊，由平台統一顯示，不靠賣家寫進每一則商品說明。例如歐盟 DSA 第 30 條要求顯示交易者資訊、Amazon 的「Detailed Seller Information」、Etsy 在歐盟的賣家資訊區塊。

**既有機制為什麼存在**：8c 設計 `PublicStore` 時「只列前台需要的欄位」，當時前台沒有地方顯示登錄字號，也沒有人回頭查法規與自家的政策。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) **`PublicStore` 加 OUTPUT_ONLY 的 `string food_business_registration_number`**，空字串表示不顯示；商品頁的店家區塊與店面頁的商家資訊各顯示一列 | 商品頁那一處對上自家政策的「商品頁面」；店面頁對上 Amazon、DSA 的「賣家資訊」位置。只公開字號，不公開賣家的輸入旗標 `processed`，符合 `PublicStore`「逐欄列出前台需要的欄位」的原則 |
| (a′) 沿用 `FoodBusiness` 訊息（v1 的建議） | 會把 `processed` 一起公開。審查 M9 指出這一點，我同意，改成 (a) |
| (b) 維持現狀，靠賣家把字號寫進商品說明 | 不改契約；平台握有結構化資料卻不顯示，也無法稽核 |
| (c) 放在商品上（catalog） | 每件商品重複同一個字號；catalog 在 B 才搬 |
| (d) 不在 10d 做，另開一項 | 範圍最乾淨；K16 與契約那一刀之後要再改一次 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 應登錄的業者要在通訊交易契約中載明登錄字號 | REAL（法規；義務主體是賣家） | 應記載事項第三點；消保法第 17 條 |
| 平台本身有沒有顯示義務 | REAL，但**缺資料**。要由律師確認（Will 決定要不要問）。在確認之前，建議 (a) 的理由也成立：自家已經公布的政策要求商品頁標示、資料已經在平台手上、而且是公開登記資料，沒有個資成本 | — |
| 契約成立的地方是結帳流程 | REAL：結帳在 Java（B8）、在邊界外，所以 10d 不在結帳頁加。商品頁是下單前最後一個由我們控制的頁面 | 主計畫 B8 |
| `PublicStore` 註解寫「沒有食品業者登錄」 | SELF-IMPOSED（8c 的選擇，上線前可以改） | `seller.proto:974` |

**建議 (a)，在 10d 做**（Will 也可以選 (d)）。
- 契約：`PublicStore` 加 `string food_business_registration_number = 12 [(google.api.field_behavior) = OUTPUT_ONLY];`（10e 已經把 10、11 設成 reserved）。註解改成「沒有聯絡電話、詳細地址與任何版本欄位；食品業者登錄字號是公開登記資料，沒有登記時是空字串」。
- **要同意的文案**：商品頁的店家區塊與店面頁頂部的商家資訊，在所在地之後加一列「食品業者登錄字號　{字號}」；空字串時整列不顯示。
- 附帶發現，只提一次：`FoodBusiness` 只在 `processed = true` 時要求字號，但法定的登錄範圍也包括有商業、公司登記的販售業者。這要 KYC 的擁有者或律師確認，不在 10d 決定。

**終點與觸發**：B8 結帳搬進單體時，結帳頁的賣家資訊從同一個欄位讀，觸發條件是 B8；B 搬 catalog 時，商品層級的法定標示由商品欄位承載。

### D4. B6 檢舉入口與食安通報

**現況**
- B6 核准的內容是：「店面頁與商品頁的『更多』選單加『檢舉商家』……website 另提供匿名的食安通報表單……**沿用 buyer-app 的 report sheet 與 website 既有表單元件**」（`step10-decision-brief.md:152`）。
- 但兩端的店面頁與商品頁都沒有「更多」選單（`merchant_header.dart:191-285`；`:2562-2577` 的 `moreHoriz` 屬於評論列）。website 完全沒有檢舉的 UI。
- buyer-app 的 `showReportSheet` 是內容檢舉：CodeMeta 的 8 個原因、`POST /app/reports`、說明可以不填、不收佐證。`CreateReport` 要 `ViolationCategory`（7 類）、**必填**說明（≤2000 字）、可選的商品、訂單與最多 5 張佐證。
- `SubmitFoodSafetyReport` 是匿名的：電話與 email 至少一個、不收圖片，重複送出與新的通報回應相同。
- 兩支 RPC 都接受曾經公開過的店（包括暫停、已關閉的店），`CreateReport` 也接受 `order_no`。

**業界基準**
- 店面與商品各有一個次要的檢舉入口，放在溢出選單或頁尾：蝦皮在右上「⋯」提供「檢舉此商品」「檢舉賣場」；Etsy 有「Report this item」「Report this shop」；Amazon 有「Report an issue with this product」。
- 檢舉表單由原因、說明、選填附件組成；從商品頁進入時自動帶上商品。
- 匿名的食安通報屬於主管機關型的入口（例如食藥署的民眾通報）：要留聯絡方式、不收附件，與登入後的檢舉分開。

**既有機制為什麼存在**：公司時期只做了內容檢舉（貼文、評論、留言、聊天）。店家層級的檢舉有後端、沒有任何 client 入口（K1 就是要刪那些路由）。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) **兩頁各加一個「⋯」按鈕**，選單裡是「檢舉商家」；檢舉表單另做（website 以既有的 `components/ui/Dialog.tsx`、`Input.tsx`、10e 的 `features/seller-center/measures/appealEvidenceUpload.ts` 組成；App 用既有的 bottom sheet 與 `MediaRepository`） | 符合 B6 的位置與蝦皮的作法；website 端照 B6「沿用 website 既有表單元件」；buyer-app 不沿用 report sheet |
| (b) 不加按鈕，改成頁面底部的文字連結（Etsy 型） | 不動頂部版面；但和 B6 的「更多選單」不同 |
| (c) **照 B6**：重用 `showReportSheet`，加一個 variant | 少一個元件；但原因清單、必填規則、佐證、送出端點都不同，variant 等於把兩張表單塞進同一個元件 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 版面、流程、文案要同意 | REAL（repo 規則） | 根 `AGENTS.md`「UI 改動」 |
| B6 的位置是「更多」選單 | REAL（已核准） | `step10-decision-brief.md:152` |
| B6 的「沿用 buyer-app 的 report sheet」 | 核准過，但前提不成立：核准時沒有比對兩份契約。這裡提出來讓 Will 改判 | 同上；`report.proto`；`report_sheet.dart:271` |
| `CreateReport` 的欄位規則（說明必填、≤5 張、同店同類每日一次） | REAL（契約） | `report.proto` |
| 匿名通報只在 website | REAL（已核准） | B6 |

**建議 (a)**。**這推翻 B6 的「沿用 buyer-app 的 report sheet」**，理由是兩個表單的契約完全不同（原因的列舉、說明必填、佐證、端點）。B6 的其餘部分照做，包括 website 沿用既有表單元件。
- **位置**：website 店面頁頂部動作列的最後、商品頁店家區塊的右上角，各一個「⋯」（`components/ui/Dropdown.tsx`）；buyer-app 店面頁頂部按鈕列、商品頁 AppBar 的分享旁邊，各一個「⋯」。選單目前只有一項。
- **登入的買家**（兩端）：「檢舉商家」開啟表單，送 `CreateReport`；從商品頁進入時帶 `product_id`，並顯示商品名稱。
- **website 未登入**：開啟一個分流，食品安全問題可以直接通報（送 `SubmitFoodSafetyReport`），其他問題先登入，登入後回到原頁。
- **buyer-app 未登入**：沿用既有的登入提示。
- **類別標籤**：沿用 10b、10c 已核准的違規類別表。10e 合併後，它在 website 的 `features/seller-center/measures/measureDisplay.ts`（`CATEGORY_LABEL` 目前沒有 export）與 seller-app 的 `measure_display.dart`。website 把這張表移到 `lib/seller/` 底下一個共用的呈現模組，讓 measures 與檢舉表單共用；buyer-app 照抄一份。一份語意、三端同字。
- **要同意的文案**：
  - 選單項目：「檢舉商家」
  - 檢舉表單：
    - 標題「檢舉商家」
    - 「問題類型」（必選）
    - 「說明」（必填，提示文字「請描述發生的情況，例如哪件商品、什麼時候、發生了什麼」，最多 2000 字）
    - 「照片（選填，最多 5 張）」
    - 從商品頁進入時顯示「相關商品：{商品名稱}」
    - 按鈕「送出檢舉」
  - 成功：「已收到你的檢舉，我們會盡快處理。」
  - `already_exists`：「你今天已經就這個問題檢舉過這家店了。」
  - `resource_exhausted`：沿用既有的限流訊息。
  - website 未登入的分流：
    - 標題「檢舉商家」
    - 「食品安全問題（例如變質、異物、標示不實）可以不登入直接通報。」按鈕「食安通報」
    - 「其他問題請先登入再檢舉。」按鈕「登入後檢舉」
  - 食安通報表單：
    - 標題「食安通報」
    - 說明「主管機關可能需要聯絡你，請至少留下電話或 Email。聯絡方式會在 90 天後刪除。」
    - 「事件說明」（必填，最多 2000 字）、「電話」「Email」「稱呼（選填）」
    - 按鈕「送出通報」
    - 成功與重複送出都顯示「已收到你的通報，謝謝你。」（契約規定兩者回應相同）
- `ListMyReports` 沒有入口，B6 也沒有要求，10d 不做。

**終點與觸發**：
- 最常見的食安情境是收貨後才出問題，而那時店家可能已經暫停、店面 404，這種情況只能從訂單進入。`CreateReport` 已經接受 `order_no`。訂單詳情頁的「檢舉商家」入口在 B8 ordering 搬遷、訂單詳情改走 Connect 時補上。
- 之後要加「檢舉這件商品」或其他次要動作時，放進同一個「⋯」。

### D5. 店家 sitemap

**現況**
- 10e ADR 的「後果」寫：10d 店面頁改讀 `GetStore` 時「補店家的 sitemap」（Will 已核准，`local/10e:…/2026-10-09-store-seo-platform-generated.md:47`）。
- `website/app/sitemap.ts` 只有靜態頁；沒有任何 Connect 方法能列出所有可曝光的店；`PublicStore` 沒有時間欄位。

**業界基準**
- sitemap 由完整列舉加上真實的 `lastmod` 組成，大站才分片（sitemaps.org 每檔 50,000 筆；Next.js `generateSitemaps`）。Shopify 自動產生 `sitemap_products_*`；Etsy 的店面都在 sitemap 裡。
- 列舉用標準 List（Google AIP-132：`page_size`、`page_token`），回傳資源本身；`update_time` 是標準欄位（AIP-148）。

**既有機制為什麼存在**：公司時期只有推薦清單，沒有完整列舉。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) **`ListStores`**：只列可曝光的店，依 `id` keyset 分頁（`page_size` 預設 100、上限 1000）；`PublicStore` 加 OUTPUT_ONLY 的 `update_time`；website **先輸出單一 sitemap**，server 端逐頁讀完 | 標準 List，之後的店家目錄頁也能用；目前店家數遠小於單檔上限 |
| (a′) (a) 加上現在就用 `generateSitemaps` 分片（v1 的建議） | `generateSitemaps` 要先知道片數，keyset List 不回總數、也不能隨機跳到第 N 片，v1 沒有交代怎麼分。目前也不需要 |
| (b) sitemap 專用的 RPC（只回 code 與時間） | 多一種只給一個呼叫者的形狀 |
| (c) 延到 B 和商品 sitemap 一起做 | 違背已核准的 10e ADR |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 10d 要補店家 sitemap | REAL（已核准的 ADR） | ADR 2026-10-09「後果」 |
| 單檔 50,000 筆 | REAL（標準） | sitemaps.org |
| prod 還沒開索引 | 不是約束（sitemap 在開索引前就該正確） | 設計書 48 |
| 改 `seller.proto` 會和 10e 衝突 | SELF-IMPOSED（排程：放在 10e 合併後） | — |

**建議 (a)**。
- `update_time = GREATEST(stores.updated_at, stores.first_exposed_at)`。店家第一次變成可曝光時，`updated_at` 不會前進，取兩者較大的才不會早於上架日（審查 L4）。非公開欄位的變更也會推進 `updated_at`，`lastmod` 因此偶爾會偏早觸發重爬，可以接受；這個語意寫在欄位註解裡。
- `ListStores` 走 `tierPublic` 與 GET 快取；sitemap 以 ISR 重建。

**終點與觸發**：可曝光的店家數超過 45,000（留 10% 餘裕）時，改用 `generateSitemaps`，依 `id` 範圍分片（`ListStores` 加 `filter` 的 id 下界），sitemap index 指向各片。B 搬 catalog 時，商品 sitemap 用同樣的 List 加 `update_time` 模式。

### D6. 賣家預覽與店主自己的讀取

**現況**
- website `/seller-center/preview/[id]` 的 layout 用公開的 `getCachedSeller(id)`（共用的 server 快取），各分頁就是公開店面頁加上 `previewMode`。
- 9d 之後，Java 隱藏不可曝光的店的商品（`ProductPublicationPolicy.java:11-12`），`/:id/products` 不看呼叫者是誰。所以**還沒公開的店（預覽最主要的使用情境）在預覽裡看不到任何商品**。10d 改讀 `GetStore` 之後，整個預覽會變成 404。
- 同一個根因造成的其他地方：
  - seller-app 聊天的商品選擇器讀 `/public/sellers/$storeId/products`（`chat_picker_remote_service.dart:28`），還沒公開的店選不到自己的商品。
  - seller-app 的商品預覽讀公開的商品詳情（`product_preview_screen.dart`），店家還沒公開時讀不到。
  - seller-app 的迴響分頁讀 `/public/sellers/$id/forum`（`post_repository.dart:510`）；D2 之後，不可曝光的店讀不到。
- 店主自己的端點都已經存在：`GET /app/sellers/products`、`/products/:id`、`/skus`（`CapProductManage`）、`GET /app/sellers/me/posts`（`CapMarketing`）、`GET /app/sellers/me/reviews`（`CapMarketing`）、`GetMyStore`（含 stats，B2）。

**業界基準**
- 店主預覽讀的是店主有權限的資料，包括還沒公開的內容：Shopify 在開店前以密碼頁擋住店面，店主從後台預覽主題與草稿商品；Shopify 後台商品的「預覽」對草稿商品也能用。
- 公開端點對所有人回一樣的內容，才可以放心快取。如果同一個 URL 依身分回不同內容（例如 GitHub 私有 repo），回應要標 `Cache-Control: private`，每一層快取都要依身分分開。

**既有機制為什麼存在**：公司時期店面對所有沒關的店都打得開，預覽只要在公開頁加上 `previewMode` 就好。可曝光的規則出現之後，這個假設就不成立了。

**選項**

| 選項 | 取捨 |
|---|---|
| (a) 公開路由加店主例外（v1 的 D2） | 同一個 URL 依呼叫者回不同內容，website 的 `unstable_cache` 與 CDN 都要依身分分開，否則店主看到的內容會被快取、再發給匿名訪客（審查 M1）。而且商品由 Java 隱藏，gateway 的例外救不了（審查 M2） |
| (b) **公開路由不依呼叫者變化；店主的讀取改走店主端點**：預覽的頂部讀 `GetMyStore`，商品分頁讀 `/app/sellers/products` 並只列上架中的，貼文分頁讀 `/app/sellers/me/posts` 並只列已發布的；評價與迴響照讀公開路由，店家不可曝光時顯示空狀態；seller-app 的商品選擇器與商品預覽改讀 `/app/sellers/products*` | 不新增任何路由；公開快取保持單純；預覽呈現的是「公開後買家會看到的樣子」。店主端點的 DTO 和公開的不同，要在呈現層轉換 |
| (c) 店家還沒公開時，預覽只顯示頂部 | 最省事；但預覽最主要的用途就是公開前檢查商品 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| 商品、貼文、評價的店主端點是 legacy REST，要到 B、A7、A6 才搬 | REAL（邊界外）。但端點已經存在，不用新增 | 主計畫 B、A6、A7 |
| Java 對不可曝光的店的商品一律隱藏，不看呼叫者 | REAL（ADR 2026-10-05 決策 2） | ADR 2026-10-05 |
| 店主端點要 `CapProductManage`、`CapMarketing`，停權的店讀不到 | REAL（已核准的能力模型）。停權的店在 seller-app 本來就進不了貼文頁（`route_capabilities.dart:66`），website 也有同樣的頁面能力表 | ADR `2026-09-18-seller-capability-authorization` |
| 討論區沒有店主端點 | REAL（social 在 A7 才搬；新增一條 REST 是丟棄的工作，路由數也會增加） | 主計畫 A7 |
| 預覽的版面與文案要同意 | REAL（repo 規則） | 根 `AGENTS.md`「UI 改動」 |

**建議 (b)**。
- 預覽是 client 端讀取（或 server 端帶使用者 token），**不進共用的 `unstable_cache`**。
- 停權的店：預覽頁沿用既有的頁面能力（缺能力時顯示既有的 `SellerSuspendedNotice`），不另做。
- **要同意的文案**：
  - 預覽橫幅（`SellerStorefrontPreviewBanner`），店家還不能公開時：「你的店家還沒公開，買家目前看不到這個店面。以下是公開後買家會看到的樣子。」可以公開時維持現有文字。
  - 店家還不能公開時，預覽的評價分頁：「店家公開後，買家的評價會顯示在這裡。」
  - 店家還不能公開時，預覽的迴響分頁與 seller-app 的迴響分頁：「店家公開後，買家的迴響會顯示在這裡。」
  - 預覽的商品分頁只列上架中的商品，沒有新文案。

**終點與觸發**：B（catalog）、A7（social）、A6（review）搬進單體時，預覽改讀各模組的店主 Connect 方法；討論區的店主讀取在 A7 由 social 模組提供。

### D7. buyer-app 店面路由的參數（精簡格式）

- **現況**：`GoRoute('/merchant/:sellerId')`，`MerchantScreen` 收數字的 `sellerId`／`sellerIdStr`；buyer-app `AGENTS.md:51` 要求調整路由結構之前先取得確認。
- **建議**：路徑字面不變，參數改名為 `:storeCode`；`MerchantScreen` 的建構子改收 `storeCode`，`store.id` 由 `GetStore` 回填（沿用 `_resolvedMerchantId` 的回填機制）；所有 `context.push('/merchant/...')` 呼叫點改傳 `storeCode`。
- **為什麼不只換參數的語意**：v1 的作法會讓 `:sellerId` 實際上裝著 `store_code`，名稱和內容錯位，是技術債。避免它的唯一代價是這次確認，屬於 SELF-IMPOSED。
- **不改成 `/store/:code` 的原因**：路徑字面改變會牽動所有呼叫點與 deep link 的對應，卻沒有使用者看得到的好處。

---

## 切分

依 B20：每刀在同一個 commit 刪掉最後一個消費者已經改完的路由，連同 openapi 與 `maxRESTOperations`。

**和 10e 的重疊**（`git diff --name-status main...local/10e`）：
- 10e 改的手寫檔包括：`website/lib/api/index.ts`、`features/cart/CartClient.tsx`、`app/(public)/sellers/[id]/(storefront)/page.tsx`、`features/sellers/storefront/metadata.ts`、`lib/seo/*`；twaddr 從 `website/lib/addresses/` 搬進 `packages/hawkr-twaddr-ts`；`api/hawkr/seller/v1alpha1/seller.proto` 與四端的生成碼；gateway 的 `handler/seller.go`、`grpc/seller_client.go`、`grpc/governance_client.go`、`router/routes_governance.go`、`router/constants.go`、`openapi_coverage_test.go`（508 → 492）、`openapi.yaml`、`scripts/legacy-endpoints-baseline.json`；seller-app 的 `my_store.dart`、`app_router.dart`、`violation_display.dart`（已刪）等。
- 任何刪路由的刀都會改到 `openapi_coverage_test.go` 與 `openapi.yaml`。所以只有不刪路由、也不碰上述檔案的刀，才能在 10e 前做。

| 刀 | 內容 | 刪除的路由 | `maxRESTOperations` | 時機 |
|---|---|---|---|---|
| 1 | seller-app：K18（`page_echo_tab` 退路、`getUserProfileMap`、`AuthStorage`）、K21 的自組網址、D1 的分享停用（只讀 `myStore.can(PUBLIC_LISTING)`，不改 `my_store.dart`）、D6 的 seller-app 部分（商品選擇器、商品預覽改讀店主端點；迴響分頁的空狀態） | —（`/{id}` 少了 seller-app 這個消費者） | 508 | **10e 合併前**。碰的檔案（`page_echo_tab.dart`、`seller_repository.dart`、`auth_storage.dart`、`page_home_feed_screen.dart`、`chat_picker_remote_service.dart`、`product_preview_screen.dart`）都不在 10e 的 diff 裡 |
| — | **`local/10e` 合進 main** | 16 個 | 492 | |
| 2 | 契約與單體：D3 的欄位、D5 的 `ListStores` 與 `update_time`、legacy `SellerMeta` 的新欄位（D2）、K26 的 `discovery.proto` 註解；`make connect`，四端重新生成 | — | 492 | 建在 10e 的 `seller.proto` 上 |
| 3a | social：D2 的 consumer、表、migration、公開 SQL 的條件、hashtag 計數、寫入的限制；ADR 2026-10-05 修訂 | — | 492 | |
| 3b | gateway：K1、D2 的評價檢查與聊天條件、`BatchGetSellerMeta` wrapper、resolver 改寫、K21 的 `share.go`、K25 | 治理 4 條 | 492 → 488 | 改到 10e 也改過的 `governance_client.go`、`seller_client.go`、`routes_governance.go` |
| 4 | buyer-app 全部：K9、K10、K12、K14、K16–K19、K21、K23、K27、K30、K31、D1、D3（顯示）、D4、D7 | `/:id/rating` | 488 → 487 | 依賴第 2 刀（D3 欄位） |
| 5 | website 非店面：首頁、市集、動態的推薦與熱門商品（K10、K11）、K12 的卡片、K28 產地預覽、K29 附近排序、`ProductViewTracker`（K9）、D1 的參照入口（含 `CartClient.tsx`） | `/recommended`、`/public/marketplace`、`/products/:id/view` | 487 → 484 | 用 `@hawkr/twaddr` |
| 6 | website 店面：`GetStore`、K6 的 404、K5 刪轉址、K7、K8 與 D6 的預覽、K16、K3 的 SEO、D5 的 sitemap、D3、D1 的 404 頁與分享停用、D4（website 端，類別表移到共用模組）；K25、K26 的設計書 | `/public/sellers/:id`、`/:id/stats` | 484 → **482** | 建在 10e 的 `store-summary.ts`、`metadata.ts` 上 |

- 第 4 刀和 10e 沒有手寫檔的重疊，只有 `openapi.yaml`（生成檔）與 `maxRESTOperations` 那一行。如果 10e 遲遲沒有合併，可以移到 10e 之前，由後合併的一方重新生成、重算。
- 每一刀各自做 code-review 與 design-review。第 3a、3b 刀有安全面，code-review 要包含 D2 列出的負向測試。第 3a 刀改了 legacy 服務的讀取路徑，要用 backend-e2e-verify 在本機證明事件確實送到 social 並生效。

---

## 附錄 A：審查意見的處理

| 審查項 | 處理 | 說明 |
|---|---|---|
| H1 全站 feed 的洩漏 | **接受，作法不同** | 已查證：`routes_post.go:38-39`、`post.go:54`、`profile.go:596`「不套用曝光規則」、`capability.go:80`。另外查到 social 自己也以 legacy `GetSeller` 填店名，以及店家可以在別家貼文留言（`comment.go:115-146`）。審查建議把 enrichment 改成卡片語意，但那只擋住 gateway 帶出的店名，貼文內容本身照樣出現，所以不採用為根本修法。改成 D2 的 R1＋(a)：social 消費曝光事件，在讀取時過濾。同時修正 v1 對洩漏面的錯誤描述 |
| H2 D4 推翻 B6 | 接受 | 摘要與 D4 都明寫推翻「沿用 report sheet」並說明理由；選項 (c) 改寫成「照 B6」；website 端照 B6 用既有元件（`Dropdown`、`Dialog`、`Input`、佐證上傳） |
| H3 第 4 刀不能先做 | **接受，而且比審查更嚴** | 審查認為第 1 到 3 刀可以先做。但 gateway 那一刀改到 10e 也改過的 `governance_client.go`、`seller_client.go`、`routes_governance.go`；任何刪路由的刀都會改到 `openapi_coverage_test.go`。所以只留 seller-app 一刀在 10e 前。路由數重算：508 → 492 → 488 → 487 → 484 → 482 |
| M1 店主例外的快取洩漏 | 接受，問題不再存在 | 不加店主例外（D2），店主的讀取走 D6 的店主端點，公開路由不依呼叫者變化 |
| M2 預覽的商品分頁 | 接受 | 已查證 `ProductPublicationPolicy.java` 不看呼叫者。根本修法是 D6 改讀店主端點；同一個根因再補 seller-app 的商品選擇器與商品預覽 |
| M3 K18 的理由 | 接受 | 改正理由，加上被否決的另一個做法 |
| M4 D1 漏了既有的伺服器補資料、約束帳誤用 | 接受 | 來源分成三類；選項 (c) 改寫；約束帳移除代價與流程列，補上 legacy enrichment 的 REAL 列 |
| M5 漏掉的入口 | 接受 | K28（產地預覽）、K29（附近排序）。附近排序只用名稱，沒有代碼空間混用；產地地圖不在 10d |
| M6 K9 違反契約 | 接受 | 兩端都無條件送；計畫:164 的字句列為修正（摘要第 2 點） |
| M7 K20 不能放在知會 | 接受 | 移到 D7，參數改名 `:storeCode` |
| M8 sitemap 分片 | 接受 | 先輸出單一 sitemap，超過 45,000 時以 id 範圍分片 |
| M9 D3 的位置、範圍與形狀 | 接受 | 明寫是新功能、Will 可以另開；改成 string 欄位、不公開 `processed`；約束帳寫明缺的資料由誰補、補到之前的理由；商品頁對應自家政策 |
| L1 gateway wrapper | 接受 | K25 |
| L2 K15 的核准範圍 | 接受 | K15 標成延伸，摘要的 UX 清單也列出 |
| L3 id tag 的讀者 | 已回答 | 有讀者（`cached.ts:365,700` 等），保留（K24） |
| L4 `update_time` | 接受 | `GREATEST(updated_at, first_exposed_at)` |
| L5 聊天的 `merchantId` | 接受 | K30 |
| L6 死碼與 Connect fake | 接受 | K31、K27 |
| L7 訂單頁的檢舉入口 | 接受 | D4 的終點與觸發 |
| L8 forum 的處理 | 接受 | 討論區與回覆的過濾放在 social（它知道 `seller_id`），不在 gateway |

沒有完全駁回的項目；只有 H1 的具體修法沒有採用，理由如上。

## 附錄 B：盤點錯誤或過時的地方

- backend Q4（`getSellerShareURL`）：`type=seller` 沒有消費者，直接刪（K21）。
- client Q9（deep link）：prop 早就沒有作用（K22）。
- client Q11（review 只送 `seller:<id>`）：review-service 已經送兩個 tag（K24）。
- client Q6、G4：契約與附錄 A:221 已經回答（K13）。
- buyer G2（訂單快照的 `storeCode` 會過期）：不會；真正的缺口是沒有狀態（D1）。
- G11（`sales_desc` 與 `SALES`）：不同義，現在的熱門商品是 bug（K10）。
- backend Q3 漏了聊天也用同一個 resolver（D2）。
- 附錄 A:226 的 `totalProducts`：沒有畫面顯示它（K23）。
- seller-app `seller_repository.dart` 的 `getUserProfileMap` 註解已經寫明 id 是混合的命名空間，與 K18 的新理由一致；`product_preview_screen.dart:301` 的「多打 `/rating`」只是過期的註解，實際呼叫的是商品的評分（`/rating` 只有 buyer-app 一個消費者）。

## 沒有查證到的

- 平台本身是否有顯示登錄字號的法定義務；蝦皮、momo 的實際顯示方式（D3）。
- 法規的引用來自衛福部的公告摘要，沒有在全國法規資料庫核對現行條文全文。
- buyer-app 通知的 payload 實際帶哪個鍵（K19）。
- social 的公開 SQL 清單（D2）是依 `db/queries/posts.sql`、`forum.sql`、`hashtag.sql` 的 query 名稱列的，第 3a 刀開工時要逐支對照呼叫路徑，確認沒有遺漏（例如貼文留言列表是否要跟著貼文的可見性）。
- `/public/products/:id` 底下的評分與評價，在商品被隱藏時是否仍然讀得到。這屬於 catalog（B），不在 10d。
