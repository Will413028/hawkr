# Hawkr 賣家 App 產品與 UX 全面改造設計書

- 文件狀態：實作中基準（核心流程與全域 UI shell 已完成，持續做真機視覺驗收）
- 建立日期：2026-07-18
- 最近更新：2026-07-19
- 適用專案：`hawkr_seller`
- 主要目標：在不遺失既有功能的前提下，重新設計整個賣家 App，與買家 App 的品牌系統對齊，並採用 iOS 26 Liquid Glass 的操作層語言
- 盤點基準：`lib/core/router/app_router.dart` 的 61 個路由、`lib/features/**/presentation` 的 61 個 Screen／Page 類別，以及所有影響流程完成的 dialog、bottom sheet、embedded panel

> 本文件先定義「每頁為什麼存在、要幫賣家完成什麼、必須覆蓋哪些狀態、內容要如何重新排列」，並作為目前實作與驗收的共同基準。重新設計可推翻舊 layout，但不得遺失路由能力、業務狀態或既有 mutation。

## 0.1 實作快照（2026-07-19）

已完成：

- 保留並以測試鎖定全部 61 條賣家路由；路由常數、宣告與唯一性皆有自動化覆蓋。
- 建立暖奶油內容層、玻璃導覽層、Material 語意 icon、adaptive page、內容卡、狀態卡、玻璃 tab、確認 dialog 與浮動五分頁導覽。
- 重做五個主入口：總覽、商店、錢包、訂單、設定；啟動頁只顯示 Hawkr 品牌載入，不再顯示牛牛歡迎頁。
- 統一店舖 onboarding、商品五步 Wizard、商品預覽、訂單列表／詳情／物流／售後、顧客 CRM、聊天／AI、通知、優惠券、PAYUNi、物流設定與商家認證的頁面外殼與操作層。
- 舊 `/order/matched` 改由 canonical 訂單列表承接；舊 matched 畫面與無路由引用的配送詳情畫面已移除。
- 訂單文件與舊收據 deep link 不再渲染假發票、假 QR、假卡號或假日期；商品預覽不再注入假產地與假店舖地址。
- 全域設計規則測試涵蓋 Material 圖示、語意色、字級、字重、圓角、間距、WCAG 對比、320px 小螢幕與 Reduce Motion。

仍需在發版前完成：

- 以 dev 真實帳號逐頁做 loading／empty／error／長文字／鍵盤／大字級的視覺回歸截圖。
- 驗證 iPhone 小／大尺寸、橫向、iPad、深色模式、Reduce Transparency 與 VoiceOver 焦點順序。
- Refero 訂閱恢復後補做核心 flow 的外部案例對照；目前所有查詢仍為 `NO_SUBSCRIPTION`，不可宣稱已引用其畫面。

## 1. 結論摘要

賣家 App 不應只是買家 App 換成後台資料，而應是一個「每日營運工作台」：

1. 首頁回答「今天最需要處理什麼」。
2. 訂單回答「哪一筆、什麼期限、下一個動作是什麼」。
3. 商品與店舖回答「目前是否可賣、缺什麼資料、怎麼快速修正」。
4. 貼文與顧客回答「誰在互動、怎麼轉成回購」。
5. 錢包回答「能不能收款、款項在哪裡、接下來要完成什麼」。
6. 設定回答「店舖資料、物流、通知、認證與風險狀態是否完整」。

視覺上採「暖奶油內容層 + 深森林綠品牌層 + 克制的玻璃操作層」。Liquid Glass 只用在導覽、頁首、分頁、浮動控制、dialog／sheet 與少量高價值互動元件，不把長列表和所有卡片全部模糊化。這符合 Apple 將 Liquid Glass 定位為浮在內容之上的 controls／navigation functional layer，而不是內容背景的原則。

## 2. 研究基準與限制

### 2.1 已使用的產品參考

| 參考 | 採用重點 | 不採用內容 |
|---|---|---|
| Hawkr 買家 App | 奶油白／森林綠／金色品牌 token、固定 logical px、玻璃 top bar、玻璃 tab、浮動 bottom nav、Material 靜態 icon | 不照搬買家購物流程或資訊密度 |
| Apple Liquid Glass / HIG Materials | 操作與導覽形成獨立玻璃層；維持內容可讀性；克制使用 tint；尊重 Reduce Transparency、Increase Contrast、Reduce Motion | 不假裝 Flutter blur 等同 Apple 原生光學材質；不在所有內容卡套 blur |
| Stripe Dashboard mobile | 財務與營運指標優先、趨勢比較、餘額／撥款／提醒可快速掃讀 | 不複製 Stripe 品牌或金融術語 |
| Shopify mobile | 商品、訂單、履約、顧客與商店編輯是賣家核心 IA | 不複製 Shopify 導覽或元件外觀 |
| Square Dashboard mobile | 單一商務總覽、任務／警示、報表與顧客資訊整合 | 不把所有模組塞回同一首頁 |

公開參考：

- Apple HIG Materials: <https://developer.apple.com/design/human-interface-guidelines/materials>
- Apple WWDC25 Meet Liquid Glass: <https://developer.apple.com/videos/play/wwdc2025/219/>
- Shopify iOS App: <https://apps.apple.com/us/app/shopify-sell-online-in-person/id371294472>
- Stripe Dashboard iOS App: <https://apps.apple.com/us/app/stripe-dashboard/id978516833>
- Square Dashboard iOS App: <https://apps.apple.com/us/app/square-dashboard-for-pos/id992958748>

### 2.2 Refero 研究狀態

2026-07-18 已分別查詢：

- 視覺風格：warm trustworthy agricultural commerce SaaS
- iOS 商家儀表板：sales / orders / tasks / alerts / quick actions
- iOS 商品與庫存：product / inventory / filters / empty / loading / error
- iOS 賣家流程：onboarding / store setup / product publishing / first fulfillment

四類查詢均回傳 `NO_SUBSCRIPTION`，無法取得 Refero screen、flow 或 style 內容。因此本版設計書沒有引用或假裝分析任何 Refero 案例。待訂閱恢復後，須在不推翻本文件功能基準的前提下補做：

- 每個核心工作流至少 3 個 screen 參考。
- onboarding、商品上架、訂單履約、售後各至少 1 條完整 flow。
- 將參考拆成「採用／拒絕／只借用的單一細節」，不可整套複製其他品牌。

## 3. 現況盤點與主要問題

### 3.1 規模

- 61 個具名路由與 61 個 `GoRoute` 宣告。
- 61 個 Screen／Page 類別，另有大量 dialog、bottom sheet、嵌入式聊天與媒體工具。
- 五個主導航：總覽、商店、錢包、訂單、設定。
- 已有 `AppIcons` registry（Material Icons）、`HkIcon` 語意圖示、Hawkr SVG Logo。
- 稽核快照仍有 21 處直接 `Icons.*`，以及 88 處 `Image.asset`／`SvgPicture.asset`。後者包含真正內容／品牌／物流圖像，不能一律替換；但一般 UI action 必須統一走 `AppIcons`／`HkIcon`。

### 3.2 已確認的資訊架構與流程缺口

| 編號 | 現況 | 風險 | 設計決策 |
|---|---|---|---|
| IA-01 | `PageScreen` 的 `TabController` 長度是 4，畫面只顯示貼文／商城／留言 3 個 Tab；第 4 個聊天 panel 沒有可見入口 | 功能存在但使用者無法發現 | 商店頁只保留「店舖、商品、貼文」三區；留言與聊天整併到獨立「收件匣」，不藏第 4 Tab |
| IA-02 | 錢包頁同時承載認證、PAYUNi 開通、地址、優惠券與財務內容 | 主任務不清楚、頁面過長 | 錢包只保留財務／收款狀態；優惠券移到「行銷」，認證與物流 gate 顯示成任務卡並連到專頁 |
| IA-03 | 錢包流程仍顯示「上傳資料頁面（待實作）」 | 使用者會進入死路 | 上線前必須完成、接到認證上傳頁，或移除不可完成的 step |
| IA-04 | 新版 `SharedOrderDetailScreen` 與多個舊訂單詳情／收據／爭議頁並存 | 狀態、文案、操作規則漂移 | 建立單一訂單詳情殼層；付款、物流、收據、售後是區段或子頁，不再複製整頁 |
| IA-05 | 舊訂單畫面仍有 `User`、`Enter description`、`Refund`、`Report to platform`、`No matched orders` | 中英混用且不像正式產品 | 全面中文化；所有錯誤與空狀態走同一 copy contract |
| IA-06 | `OrderDetailsDeliveringScreen` 僅在自身檔案被引用 | 可能是孤兒或舊稿 | 實作前做 route reachability 測試；確認淘汰後刪除，不讓舊畫面回流 |
| IA-07 | 主商店路由先注入 placeholder 地址、圖片、促銷文案 | 真資料載入前可能閃出假資料 | 改成 branded skeleton；沒有資料就顯示明確未設定狀態，不顯示假店舖內容 |
| IA-08 | 商品有庫存欄位與 inventory 語意圖示，但沒有清楚獨立的庫存工作面 | 缺貨只能逐商品處理 | 商品中心新增庫存篩選與批次快速調整；是否獨立成頁由 API 能力確認 |
| IA-09 | 首頁只有本週數據與狀態數量，沒有集中任務優先序 | 有資料但賣家仍要自己找工作 | 首頁第一區改為「今日待辦」，營收指標退到第二層 |
| IA-10 | 直接 icon／PNG／SVG 來源混雜 | 同一語意多種畫風 | 建立 Material Icons + Brand Asset 單一合約，並以測試禁止新的任意 icon |
| IA-11 | 部分頁面只輸出 raw error 或英文 Retry | 無法知道怎麼復原 | 每頁必備 loading、empty、offline、retry、permission/gate、partial failure |
| IA-12 | 商品與貼文為多步流程，但「離開、背景終止、上傳失敗後復原」未形成一致 UX contract | 草稿遺失或重複送出 | 建立草稿 autosave、離開確認、背景恢復、upload retry、成功狀態 contract |

## 4. 新版資訊架構

### 4.1 五個主導航

| 主導航 | 目的 | 首屏內容 |
|---|---|---|
| 總覽 | 今日營運工作台 | 店舖健康、今日待辦、訂單風險、營收摘要、快捷動作 |
| 商店 | 管理消費者看到的店舖 | 店舖預覽、商品、貼文；庫存與上架狀態是商品內的首要 filter |
| 錢包 | 收款與資金狀態 | 可用／待入帳、撥款狀態、PAYUNi／認證 gate、交易／撥款紀錄 |
| 訂單 | 履約與售後工作佇列 | 待處理、配送中、售後、完成；依 deadline 與風險排序 |
| 設定 | 商家身分與營運設定 | 品牌資料、認證、物流、通知、幫助、帳號與風險 |

### 4.2 全域收件匣

聊天與通知不佔第五個主導航，但在總覽 top bar 提供兩個 Material icon action，並保留 unread badge。通知負責「系統事件」；聊天負責「人與人對話」。通知 deep link 必須能落到對應訂單、售後、商品、貼文、顧客或認證頁，不能只打開列表。

### 4.3 全域建立入口

商店頁提供一個 iOS 式玻璃 `Create` 浮動按鈕，按下後出現 action sheet：

- 新增商品
- 新增貼文
- 建立優惠券
- AI 圖片工具

每個 action 使用 Material icon；sheet 上緣放小型 `logo-mark.svg` 作品牌簽章，但不把 Logo 塞進每個 icon。

## 5. 視覺與互動系統

### 5.1 Liquid Glass 使用邊界

使用玻璃：

- 浮動 bottom navigation
- top bar / collapsed toolbar
- segmented tabs 與選取 pill
- modal、dialog、bottom sheet
- 浮動建立／篩選／搜尋控制
- 暫時性 active control

不使用玻璃：

- 長列表的每一列
- 商品與貼文圖片
- 大面積 App 背景
- 密集表單的每一個 section
- 低階資訊卡全部疊 blur

內容卡採奶油白實體 surface、1px ink-alpha border、低彩度 ink shadow。這可同時保留品牌溫度與閱讀效率。

### 5.2 色彩角色

| 角色 | 色彩 | 使用規則 |
|---|---|---|
| Brand / Primary | `brandPrimaryStrong` | 主要 CTA、active icon、重要數字；不可整頁大面積填滿 |
| Secondary | `brandPrimary` | 次要 accent、裝飾；不配一般尺寸的白字 |
| Premium / Highlight | `brandSecondary` | 驗證、重要成就、單一重點；不可當一般按鈕背景 |
| Content background | `surfacePage` | 所有頁面連續背景 |
| Card | `surfaceCream` | 內容分組與摘要 |
| Glass | warm translucent white/cream | chrome / overlay，需有 blur fallback |
| Danger | `danger` | 失敗、逾期、破壞性操作 |
| Info | `info` | 配送中、資訊提示 |

### 5.3 版面規則

- 使用 logical px，不依整台手機寬度縮放字級與 icon。
- 觸控區至少 48x48。
- 手機 gutter 16；tablet 24；wide 32。
- 主內容手機滿寬、tablet 最大 720、wide 最大 1120。
- 主頁 top bar 56（不含 status inset），bottom nav 約 68。
- 數字使用 tabular figures；金額和數量不可因位數跳動。
- 文字縮放至少驗證 1.0、1.3；不使用 `FittedBox` 壓縮重要資訊。

### 5.4 動效與觸覺

- Tab／nav 選取：200ms spring/ease，selection haptic。
- 成功：一次輕量 haptic + 非阻塞 toast；不可用長動畫阻擋下一步。
- 破壞性動作：warning haptic 只在確認提交時。
- Reduce Motion：停用 spring，保留即時狀態更新。
- Reduce Transparency／低階裝置：以 92–96% 不透明奶油 surface + border 取代 blur。

## 6. 圖示與 Hawkr Logo 規範

### 6.1 三層資產合約

1. **品牌層**：`assets/brand/logo-mark.svg`、`logo-horizontal.svg`、`wordmark.svg`、`logo-grass.svg`。只用於啟動、登入、top bar home、空狀態 halo、分享圖與極淡 watermark。
2. **導覽與模組層**：`HkIcon`（`HkIconName`）。用於主導航、主要賣家模組與空狀態；導覽選取態用 filled、未選取用 outlined，另有顏色與文字標籤，不只靠顏色區分。
3. **一般圖示層**：`HkIcon` / `AppIcons`。用於表單、狀態、工具列與一般 action。

例外保留原圖：

- Google、Apple、PAYUNi、Visa、7-11、黑貓等外部品牌 Logo。
- 商品、貼文、認證文件、店舖照片等內容媒體。
- 真正的插圖或教學圖，不把它誤當 UI icon。

### 6.2 Logo 融入方式

- 啟動頁：只顯示 Hawkr 品牌動態載入，不顯示牛牛歡迎頁。
- 主 top bar：無返回鍵的主頁使用 28–32px logo mark 作 home／brand anchor。
- 空狀態：靜態語意圖示放在帶有森林綠 halo 的品牌容器；下方可有 12–16px logo grass divider。
- 總覽 KPI：背景可使用 3–5% opacity logo-grass watermark，但不可干擾數字。
- 成功完成：可用 logo mark + Material check 的組合插圖；兩者是分層組合，不修改圖示本身。
- 不在每個按鈕、每個列表列、每個 icon 裡硬塞 Logo；那會降低語意辨識並讓品牌變成噪音。

### 6.3 頁面語意對照

| 功能 | 圖示 |
|---|---|
| 總覽 | `SellerCenterIcon.dashboard` |
| 店舖預覽 | `SellerCenterIcon.preview` |
| 商品 | `SellerCenterIcon.products` |
| 庫存 | `SellerCenterIcon.inventory` |
| 貼文 | `SellerCenterIcon.posts` |
| 優惠券 | `SellerCenterIcon.coupons` |
| 顧客 | `SellerCenterIcon.customers` |
| 錢包 | `SellerCenterIcon.wallet` |
| 訂單 | `SellerCenterIcon.orders` |
| 物流 | `SellerCenterIcon.shipping` |
| 訊息 | `SellerCenterIcon.messages` |
| 洞察 | `SellerCenterIcon.insights` |
| 違規 | `SellerCenterIcon.violations` |
| 設定 | `SellerCenterIcon.settings` |
| 登出 | `SellerCenterIcon.logout` |

### 6.4 程式規則

- Feature code 不新增 `Icons.*`；先補 `AppIcons` 或 `HkIconName`。
- UI action 不直接引用任意 PNG／SVG；先經 `HkIcon`／`AppIcons` 映射或明確列入 brand/external/content 例外。
- 所有 icon-only button 必須有 tooltip／semantic label。
- 一個語意只有一個 canonical icon；selected/unselected 只變 filled／outlined 與 tint。

## 7. 全域情境覆蓋矩陣

每個可讀取遠端資料的頁面至少覆蓋：

| 狀態 | 必須行為 |
|---|---|
| Initial loading | branded skeleton；不可閃 placeholder 假資料 |
| Pull refresh | 保留舊資料，顯示輕量 refresh，不整頁白屏 |
| Empty first-use | 靜態語意圖示 empty state + 解釋 + 單一主要 CTA |
| Empty filtered | 顯示目前 filter 條件與「清除篩選」 |
| Offline | 顯示最近同步時間與重試；可讀快取就不遮住內容 |
| Timeout / server error | 人類可理解訊息、重試、不輸出 raw exception |
| Partial failure | 成功區塊照常顯示，失敗區塊局部重試 |
| Unauthorized | 統一 session expiry 流程；不可每頁自行顯示不同錯誤 |
| Verification gate | 保留可閱讀內容，只鎖需要認證的 mutation，並說明如何解鎖 |
| Suspended / restricted | 顯示影響範圍、原因、期限、申訴入口；不可只跳 generic error |
| Pagination | 尾端 loading／retry，不重複列、不重置 scroll |
| Realtime update | 顯示狀態已更新，不讓卡片突然消失而無解釋 |
| Mutation sending | 按鈕局部 loading、避免重複提交 |
| Request sent | 明確區分「已送出」和「後端已接受／處理完成」 |
| Mutation success | 更新本地狀態 + 簡短回饋 + 下一步 |
| Mutation rejected | 保留輸入、顯示業務原因與可修復建議 |
| Destructive confirm | 說明影響；主要／取消位置一致；成功後不可殘留舊資料 |
| Unsaved draft | autosave 標記、離開確認、背景／重啟復原 |
| Upload | 壓縮、進度、失敗重試、取消、背景恢復、重複檔案 |
| Long content | 店名、地址、商品名、錯誤訊息、金額、99+ badge 不 overflow |
| Accessibility | VoiceOver 順序、48px target、1.3 text scale、Reduce Motion／Transparency |

## 8. 逐頁功能與設計規格

### 8.1 啟動、登入與 onboarding（4 路由）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/welcome` `WelcomeScreen` | 在 session bootstrap 完成前提供品牌載入，依登入／onboarding／停權狀態導向 | 全螢幕 warm cream；中央品牌 mark 動態；下方小型進度訊息；不顯示牛牛歡迎頁 | slow bootstrap、offline cached session、redirect loop guard；只用 Logo，不用功能 icon |
| `/auth` `AuthScreen` | Google／Apple／LINE 登入（Clerk）、隱藏 demo review login | 上半品牌月亮／農地 hero；下半單一玻璃登入 card；OAuth 按鈕保留外部品牌 Logo；條款與版本固定底部 | 取消、provider error、買家帳號誤入、demo entry；一般 action 用 Material icon |
| `/onboard` `SellerInformationScreen` | 建立／補齊店舖名稱、聯絡與地址，完成 seller 基本資料 | 三段式：品牌身份、聯絡方式、營運地址；頂部 progress；底部固定 CTA；「稍後設定」必須說明影響 | 新／既有 seller、資料預填、地址不完整、同步部分失敗、skip、重登續填；store/user/phone/location 語意圖示 |

### 8.2 總覽主頁（1 路由）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/main/dashboard` `DashboardScreen` | 每日營運工作台：顯示店舖健康、待辦、異常、營收與訂單摘要 | 1. 玻璃 top bar：Logo、店名、通知、聊天；2. 店舖健康 strip；3. 今日待辦卡（逾期／待出貨／售後／缺貨）；4. 2–3 個核心 KPI + 期間切換；5. 訂單狀態；6. 快捷動作；7. 最近活動 | 部分 provider 失敗、0 值、趨勢正負、99+、配送異常、未認證、停權、pull refresh；dashboard/orders/alert/insights 語意圖示 |

總覽不再放 9 張等權卡片。排序原則：

1. 有 deadline 或風險的任務。
2. 可直接採取行動的狀態。
3. 幫助判斷的指標。
4. 純資訊與最近活動。

### 8.3 商店、商品總覽、貼文與留言（主路由 1 + 內容區）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/main/category` `PageScreen` | 管理買家看到的店舖，而不是後台雜項集合 | Hero 顯示真實封面／Logo／店名／地區／評分／認證；下方玻璃 segmented tabs：「店舖」「商品」「貼文」；右下 glass Create；聊天不再藏在第 4 Tab | own vs preview、banner loading、未設定資料、認證 gate、seller refresh、無假 placeholder；preview/products/posts |
| 店舖內容 `PageHeader` / `PageHomeTab` | 預覽與編輯品牌呈現、店舖摘要、精選內容 | 「買家預覽」主 CTA；品牌完整度；店舖介紹、配送／免運、精選商品、最新貼文；edit action 直接連設定對應欄位 | 缺封面／地址／介紹、preview API 失敗、認證 badge、長店名；store/edit/location |
| 商品內容 `PageShopTab` | 管理上架、審核、下架、售罄與新增商品 | 頂部 search + filter chips（全部／上架／草稿／審核／下架／售罄）；庫存風險摘要；compact product rows；批次模式；主要 CTA 新增商品 | 未認證 onboarding、未設定物流、空商品、審核中不可編輯、上下架、刪除、缺庫存、partial load；products/inventory/search/filter |
| 貼文內容 `PageHomeTab` | 管理品牌內容與社群互動 | 頂部建立貼文、草稿／已發布 filter；post preview rows 顯示媒體、時間、互動與關聯商品；more menu 編輯／隱藏／刪除／分享 | 空貼文、上傳中、隱藏、媒體錯誤、刪除、互動同步；posts/image/video/more |
| 留言內容 `PageEchoTab` | 管理買家留言與公開回覆 | 移出商店主 tab，放到「互動」二級入口；未回覆優先；thread card + reply composer；可由通知 deep link | 空留言、未回覆、已回覆、刪除、更新回覆、被封鎖內容、分頁；chat/reply/alert |
| `EmbeddedChatListPanel` | 商店上下文中的即時聊天 | 不再做隱藏 tab；共用 `/chat` 收件匣，可從商品／店舖帶 context 開啟 | 連線、重連、未讀、無對話、封鎖／已失效商品；messages |

### 8.4 錢包與優惠券（主路由 1 + 3 優惠券路由）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/main/wallet` `WalletScreen` | 看收款能力、餘額、待入帳／撥款與金融設定狀態 | 1. balance hero；2. 待入帳／可撥款／最近撥款；3. 必要任務（認證、地址、PAYUNi）；4. transaction/payout tabs；5. 財務說明。優惠券移出 | loading、未開通、審核中、補件、失敗、地址缺失、餘額 0、敏感資訊遮罩；wallet/chart/verified |
| `/coupon/editor` `CouponEditorScreen` | 建立或編輯優惠券 template | step form：優惠類型→門檻／折扣→數量／期間→適用對象→預覽；底部固定儲存／發布 | draft、欄位衝突、日期錯誤、圖片失敗、API reject、離開草稿；coupon/calendar/users |
| `/coupon/add` `AddCouponScreen` | 對特定／分群顧客發券 | 對象選擇先行；搜尋 phone/account；已選 recipient chips；優惠券設定；發送摘要 | 查無顧客、重複顧客、無權限、部分發送失敗、重送；customers/search/coupon/send |
| `/coupon/detail` `CouponDetailScreen` | 查看優惠券狀態、領取／使用與條件 | status hero；發行／領取／使用 KPI；規則；適用對象；操作 menu（停用／刪除／複製） | active/expired/exhausted/deleted、0 領取、後端統計延遲；coupon/chart/more |

### 8.5 訂單、履約、付款、收據、售後（10 訂單路由 + 2 tool route）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/main/orders` `OrderListScreen` | 賣家的工作佇列 | 玻璃 header；search；segments「待處理／配送中／售後／完成」；chips 顯示數量；order card 第一行 deadline／risk，第二行買家與商品，底部只放下一個主要 action | realtime update、pagination、empty filter、after-sale lookup 部分失敗、黑貓已叫件但主狀態未同步、loading per order |
| `/orders/view` `OrderListScreen(showBack)` | 從外部入口打開完整訂單列表 | 與主訂單共用內容，只改 shell/back 行為，不另做 UI | deep link filter、返回來源保留 |
| `/order/details` `OrderDetailScreen` | 單一可信訂單詳情 | status/risk hero；primary next action；交易、配送、商品、付款、聯絡、歷程；sticky action bar | route extra 無效、資料更新、取消／退款、物流異常、聯絡買家、複製單號 |
| `/orders/matched/detail` `OrderDetailScreen` | 舊 matched deep link 相容 | 直接導向同一 `SharedOrderDetailScreen`；不可保留第二套設計 | legacy extra normalization、not found |
| `/order/matched` `OrderMatchedScreen` | 舊版 matched 訂單列表 | 規劃淘汰；能力併入 `/main/orders?segment=in_transit` | 若保留相容路由則 redirect，不渲染舊英文畫面 |
| `/order/payment-details` `OrderPaymentDetailsScreen` | 顯示付款狀態與歷程 | 成為訂單詳情的付款子頁／section；timeline + 金額拆分 + exception badge | 待付款、完成、失敗、逾期、重複付款、狀態衝突、退款 |
| `/order/receipts` `OrderReceiptsScreen` | 訂單收據／商品金額資料 | 成為「文件」sheet 或子頁；summary + 可下載文件 | 無文件、產生中、失敗、已過期 |
| `/order/receipts-detail` `OrderDetailsReceiptsScreen` | 舊收據 detail | 與上一頁整併；相容 route redirect | route 無資料、舊 deep link |
| `/order/dispute` `OrderDisputeScreen` | 舊退款／回報平台 UI | 規劃由售後 case flow 取代，不保留獨立英文表單 | 若案件不存在，引導建立／查看售後；不可直接假送出 |
| `/order/review` `OrderReviewScreen` | 查看買家對訂單／商品評價 | 評分摘要、商品別評價、文字／圖片、是否已回覆；不提供賣家冒充買家評分 | 無評價、部分商品、媒體失敗、hidden/moderated |
| `/order/logistics` `LogisticsTrackingScreen` | 查詢 PAYUNi／超商／宅配物流 | provider header；目前狀態；vertical timeline；單號 copy；refresh；異常 CTA | 無物流、查詢中、provider timeout、異常、已送達、退回 |
| `/after-sales` redirect | 售後入口 | 直接落訂單「售後」segment，保留 bottom nav | filter deep link |
| `/after-sales/detail` `AfterSalesDetailScreen` | 查看案件、證據、決策、退貨與退款 | case status hero；deadline；買賣雙方照片；申請資訊；resolution timeline；sticky context action | pending、平台接手、同意退貨、部分／全額退款、拒絕、退件、驗貨、退款失敗／人工處理 |
| `AfterSalesResolveSheet` | 賣家決策 | 大型玻璃 sheet；一次只選一種 resolution；逐步收集金額／原因／退貨物流；最後 review confirm | refundable max、必要證據、7-11／黑貓建立失敗、重複送出、request sent vs accepted |
| `AfterSalesConfirmReceiptDialog` | 確認收到退貨並啟動退款 | 顯示退貨單號、照片、驗貨結果與退款金額；高風險確認 | 已確認、案件狀態變更、退款建立失敗 |
| `PdfPreviewScreen` | 在 App 內預覽寄件單 PDF | document viewer + download/share/open external；過期資訊 | URL 無效、下載失敗、過期、離線 |
| `OrderDetailsDeliveringScreen` | 舊配送詳情 | 淘汰並併入 `SharedOrderDetailScreen` | 不新增入口 |

訂單卡 primary action 規則：

- 待付款：無 mutation，只顯示等待。
- 已付款／待出貨：確認訂單、建立物流或出貨。
- 黑貓：預約收件、上傳出貨照片、下載寄件單、取消出貨。
- 7-11：顯示交貨便代碼、下載／取得寄件單、取消出貨。
- 配送中：查看物流；有 anomaly 時 anomaly 優先。
- 已送達：等待買家確認；有售後時售後優先。
- 完成：查看詳情／評價，不顯示假操作。
- 售後：處理售後或確認退貨收件，取代一般履約 action。

### 8.6 商品建立、編輯、預覽與評價（9 路由）

新版 wizard 固定為：

1. 分類
2. 基本資訊
3. 媒體
4. 認證與標籤
5. 規格／價格／庫存／物流
6. 預覽與發布

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/product/picker` `ProductSetupPickerScreen` | 選 L1/L2/L3 分類或提議新分類 | search + recent + hierarchical drawer；底部顯示已選 path；候選分類有明確「待審核」說明 | 無分類、搜尋無結果、parent id 缺失、新分類重複、候選送出 |
| `/product/info` `ProductSetupInfoScreen` | 商品名稱、分類、產地、描述 | 上方 stepper；單欄表單；AI 協助作次要入口；描述可開 rich editor | 必填、舊資料回填、分類 mapping 失敗、長描述、autosave |
| `/product/images` `ProductSetupImageScreen` | 主圖、排序、裁切、描述媒體 | 主圖 slot + 可拖排序 gallery；上傳進度；AI 修圖；每張狀態清楚 | 權限、格式、壓縮、過大、上傳失敗、重試、刪除、排序、最少張數 |
| `/product/media-picker` `ProductSetupMediaPickerScreen` | 選擇／預覽商品媒體 | 系統 picker 後的 review screen；圖片／影片規格；確認選取 | 無媒體、不可讀、影片過長、取消 |
| `/product/tags` `ProductSetupTagScreen` | 食品／產銷認證、標籤、證號 | 可跳過的 tag grid；需要證號者 inline 展開；驗證結果 badge | 未登記、API unavailable、審核中、已驗證、過期 |
| `/product/specs` `ProductSetupSpecScreen` | SKU 規格、重量、價格、庫存、溫層、配送 | 先共用商品設定，再 SKU card list；批次套用價格／庫存／物流；底部錯誤摘要 | 價格／庫存非法、重複規格、0 庫存、無配送、quotes 失敗、認證不符 |
| `/product/preview` `ProductPreviewScreen` | 發布前以買家視角檢查；已發布商品也可預覽 | 完整買家商品頁；頂部「預覽模式」glass banner；底部編輯／發布，不顯示可購買假按鈕 | 缺資料、運費估算失敗、評價／圖片部分失敗、發布中／成功／拒絕 |
| `/product/reviews` `ProductReviewsScreen` | 查看商品評價與圖片 | rating distribution、filters、review list、seller reply status | 無評價、圖片失敗、分頁、moderated |
| `/product/edit` `ProductEditScreen` | 舊快速編輯價錢／庫存 | 規劃整併到 wizard 的 edit mode 或商品 quick edit sheet | legacy route 導到 canonical edit；不可維護第二套表單 |
| `ProductListingChatSheet` | AI 協助建立商品草稿 | 對話在 sheet，draft patch 可預覽並逐項採用；不得自動發布 | streaming、斷線、partial patch、拒絕套用、重試、敏感／不合法內容 |

### 8.7 貼文建立、編輯與全螢幕 feed（4 路由）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/post/media-picker` `PostMediaPickerScreen` | 選擇貼文圖片／影片並預覽 | media grid／preview；比例與時長提示；下一步固定底部 | 權限、空選取、影片不可讀、取消、格式不支援 |
| `/post/compose` `PostComposeScreen` | 撰寫內容、hashtag、關聯商品並發布 | 媒體 preview；文字 composer；hashtag autocomplete；linked product card；發布 CTA | autosave、upload、長度、敏感內容、商品失效、發布失敗／重試 |
| `/post/edit` `PostEditScreen` | 編輯已發布貼文 | 與 compose 共用 shell；顯示「已發布」與修改影響；可替換媒體規則要明確 | post deleted、權限、修改衝突、儲存失敗 |
| `/post/feed` `PageHomeFeedScreen` | 以全螢幕查看自身貼文、留言、按讚、分享 | 沉浸媒體；玻璃 top／side controls；留言 sheet；關聯商品 card | 無貼文、媒體錯、like optimistic rollback、留言分頁、hidden post |

### 8.8 顧客 CRM（5 路由）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/customers` `CustomerManagementScreen` | 搜尋、分群、查看顧客與 VIP／黑名單 | search；segments 全部/VIP/黑名單；customer rows 顯示最近購買、累計、tag；右上 filter | 空顧客、搜尋無結果、load error、type update rollback、分頁 |
| `/subscribers` `SubscriberListScreen` | 查看訂閱店舖的人 | subscriber rows + follow date／互動摘要；導顧客詳情或聊天 | 空、已取消訂閱、profile image error |
| `/customer/detail` `CustomerDetailScreen` | 單一顧客 360：消費、訂單、優惠券、標籤、聯絡 | identity header；KPI；quick actions；recent orders；tags；coupon；timeline | 顧客不存在、資料部分失敗、VIP／blocklist、電話／聊天不可用 |
| `/customer/spending-history` `CustomerSpendingHistoryScreen` | 交易、退款、訂單、優惠券紀錄 | summary hero；玻璃 segments；chronological ledger；filter period | 0 消費、退款大於消費、舊資料、長列表 |
| `/customer/order-detail` `CustomerOrderDetailScreen` | 從 CRM 看訂單 | 共用 `SharedOrderDetailScreen`，但保留 CRM 返回脈絡 | route extra 無效、訂單更新 |

CRM 必須明確區分：

- 訂閱者：關注店舖，不代表已購買。
- 顧客：至少有交易或 seller CRM 關係。
- VIP／黑名單：賣家管理標記，不改寫平台風險狀態。
- Tag：可多選、自訂、編輯與刪除；刪除前顯示影響人數。

### 8.9 認證、PAYUNi、停權與申訴（4 seller 路由 + 錢包狀態面板）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/seller/certification/upload` `SellerCertificationUploadScreen` | 個人／企業文件、OCR、銀行與營運資料上傳 | 分段 stepper；每步只收一類資料；document card 有 OCR state；最後 review | camera/photo permission、OCR unavailable／錯誤、文件重拍、敏感遮罩、個人／企業分支、部分上傳、續填 |
| 錢包狀態面板（`/main/wallet`，無獨立路由） | 查看 PayUni 審核狀態與補件 | 依 `GetMyKYC` 的 `flow.payuni` 顯示標題、說明、要修正的欄位；可修正時給「前往補件」，指向店家欄位時給「前往店家設定」 | 審核中、需補件、未通過、暫時無法處理、已終止、載入失敗 |
| `/seller/suspension-notice` `SuspensionNoticeScreen` | 明確告知停權範圍與可用能力 | danger hero；原因／證據／開始結束時間；受影響能力；可讀入口；申訴 CTA | 全停權／部分限制、期限、申訴中、已解除 |
| `/seller/measures` `SellerAppealScreen` | 查看處置並提出申訴 | measure list；選案件；證據／說明；送出 review；申訴 timeline | 無處置、過期不可申訴、草稿、附件失敗、已送出、平台決策 |
| `/seller/photo-tools` `SellerPhotoToolsScreen` | 舊 seller media AI 工具 | 與 `/photo-editor` 整併；保留 context adapter，不維護第二套 UI | 圖片／影片能力差異、prompt language、AI 失敗 |

### 8.10 設定、物流、通知與幫助（主路由 1 + 8 settings / anomaly route）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/main/settings` `ProfileScreen` | 店舖身份與營運設定首頁 | banner／avatar；店舖完整度；基本資料；認證；營運設定；帳號；危險區。編輯用專屬 sheet／page，不在長頁全部 inline | cache vs remote、upload、save partial failure、long address、未認證、通知 badge |
| `/profile` `ProfileScreen` | 從外部進入同一設定內容 | 與主設定共用；只改返回殼層，不複製頁面 | 返回來源 |
| `/settings/shipping` `ShippingSettingsScreen` | 管理 7-11／黑貓、寄退件地址與服務開關 | provider cards；啟用狀態；地址；溫層／服務；測試／驗證；危險停用 | 未認證、地址 invalid、TGOS timeout、開通失敗、部分 provider 可用 |
| `StoreMapWebViewScreen` | 7-11 門市選擇 | secure webview shell；進度；取消；回傳門市摘要確認 | URL 失效、WebView error、使用者取消、回傳資料缺失 |
| `/notifications` `NotificationsScreen` | 系統事件收件匣 | filter 全部／未讀／訂單／售後／互動／系統；group by date；mark all；multi-select delete | empty、partial deep link、read rollback、delete、load more、unknown type |
| `/notifications/settings` `NotificationSettingsScreen` | 控制各類通知 | 依「營運必要／交易／社群／行銷」分組；說明關閉影響；platform permission banner | OS permission denied、optimistic rollback、server mismatch |
| `/settings/help` `HelpCenterScreen` | FAQ、聯絡客服、版本資訊 | search FAQ；熱門問題；LINE／Email 外部品牌 action；診斷資訊 copy | 離線、外部 App 未安裝、版本讀取失敗 |
| `/shipment-anomalies` `ShipmentAnomalyDashboardScreen` | 集中處理配送異常 | severity KPI；filters；anomaly list；每列顯示訂單、事件、時間、建議；ack／ignore | high/medium/low、empty、realtime、ack/ignore rollback、對應訂單不存在 |

### 8.11 聊天、AI 與內容工具（4 route / tool）

| 路由／頁面 | 功能目的 | 新版內容 Layout | 必備情境與 Icon |
|---|---|---|---|
| `/chat` `ChatListScreen` | 買家／客服訊息收件匣 | segments 全部／未讀／客服；search；conversation row 帶訂單／商品 context；AI 助理固定入口 | connect/reconnect、offline cache、empty、unread、deleted participant、pagination |
| `/chat/thread` `ChatThreadScreen` | 即時對話、回覆、傳圖、分享訂單／商品 | glass header；message list；context banner；composer；`+` action sheet；reply preview | no messages 中文化、send pending/failed/retry、read receipt、upload、block、expired context、copy/reply/delete |
| `/chat/ai-assistant` `AiAssistantScreen` | 查詢未出貨、庫存、收款、商品分析 | 首屏 quick prompts；回答卡帶資料來源／時間；可跳對應頁；composer；session history | streaming、工具失敗、資料過期、無結果、人工確認；AI 不可直接執行高風險 mutation |
| `/photo-editor` `ImageEditorScreen` | 裁切、調整、文字、貼紙、AI 修圖與輸出 | canvas；底部 glass tool rail；undo/redo；export CTA；AI 是一個 tab | 無圖片、記憶體、AI unavailable、export/upload failure、permissions、Reduce Motion |
| `/rich-text-editor` `RichTextEditor` | 編輯商品／貼文長內容與表格 | top action bar；document canvas；keyboard toolbar；block insert；save | paste table、max length、keyboard、undo/redo、link invalid、draft |

## 9. Overlay 與非路由元件的設計規格

下列雖非 route，但會決定任務能否完成，必須列入 redesign 與測試：

| 類型 | 元件／流程 | 規格 |
|---|---|---|
| 出貨 | 黑貓預約、7-11 寄件、叫件、取消出貨、列印 PDF | 同一物流 action sheet 入口，依 provider 顯示必要欄位；送出後分 request sent／accepted |
| 售後 | resolution sheet、退貨物流、部分／全額退款、拒絕、確認收貨 | 每次只做一個決策；金額與不可逆影響在 confirm 前重述 |
| 帳號 | 登出、刪除帳號、停權 | 使用共用危險 dialog；刪除列出未完成訂單 gate 與不可逆後果 |
| Profile | 地址編輯、文字欄位編輯、頭像／封面選取 | glass sheet；保留未儲存輸入；地址分 city/district/detail |
| 商品 | 分類 drawer、新分類 dialog、離開 wizard、AI listing chat | 共用 step shell；error summary 能跳到欄位 |
| 內容 | 媒體 picker、分享、留言、hashtag autocomplete | 避免多層 sheet 疊 sheet；關閉後回原 scroll/context |
| 日期 | 日期 picker | iOS 玻璃 dialog；本地化；disabled range 清楚 |
| 通用 | confirm、toast、loading overlay、empty state | 全 App 單一元件與 copy contract |

## 10. 需要新增或補強的情境

### P0：沒有就不能安全重設

- [ ] 61 路由 reachability 測試與 legacy redirect 決策。
- [ ] 商店第 4 個隱藏 Chat Tab 的整併。
- [ ] 錢包「上傳資料頁面（待實作）」死路消除。
- [ ] 訂單 canonical detail 與舊詳情／收據／爭議路由整併。
- [ ] 全 App 中文化，移除使用者可見 `Retry`、`No messages`、`User` 等。
- [ ] 全域 loading / empty / error / offline / retry 元件。
- [ ] mutation「已送出」與「已受理／完成」狀態 contract。
- [ ] 商品／貼文草稿保存與離開復原 contract。
- [ ] 圖示 policy test，禁止繞過 `AppIcons`／`HkIcon` 直接寫 Material UI icon。

### P1：核心營運完善

- [ ] 首頁今日待辦與 deadline 排序。
- [ ] 商品庫存 risk filter、售罄與批次調整。
- [ ] 訂單 search、segment、risk、售後優先序。
- [ ] Wallet 真正的 balance／payout history；若 API 尚未提供，顯示能力範圍，不造假。
- [ ] 通知 deep link 完整 mapping。
- [ ] 配送異常直接連對應訂單與建議動作。
- [ ] CRM 分群、tag 影響人數與發券接續。
- [ ] 認證／PAYUNi 補件與進度 timeline。

### P2：成長與效率

- [ ] 商品／貼文／顧客跨模組 search。
- [ ] 獨立 Insights 二級頁：期間、銷售、流量、轉換、商品排行。
- [ ] 首頁快捷動作可依賣家常用行為排序。
- [ ] AI 助理回答帶資料時間、來源與一鍵跳頁。
- [ ] 批次商品上下架／庫存／優惠操作（需 API 能力確認）。

## 11. 逐頁 Definition of Done

每一頁完成重設，必須同時符合：

- [ ] 功能目的與主要 CTA 和本文件一致。
- [ ] loading、first-use empty、filtered empty、offline、error、retry 已覆蓋。
- [ ] 所有 mutation 有 sending、request sent、accepted/success、rejected。
- [ ] Icon 使用 `AppIcons` registry；外部品牌／內容圖像例外有註記。
- [ ] Logo 使用符合品牌層規則，沒有過度裝飾。
- [ ] 無新增 `Icons.*` 與未登錄 UI asset。
- [ ] VoiceOver label、focus order、48px target。
- [ ] 320／390／430 寬度無 overflow；tablet 有 max width。
- [ ] text scale 1.0／1.3；長店名／地址／金額／錯誤訊息。
- [ ] Reduce Motion／Transparency fallback。
- [ ] deep link、返回來源、bottom nav selected state 正確。
- [ ] widget tests 覆蓋主要狀態；iOS simulator 有基準 screenshot。
- [ ] `flutter analyze`、targeted tests、完整 test suite、iOS simulator build 通過。

## 12. 建議實作順序

### Wave 0：基準與防護

1. 保存目前 UI screenshots 與 route inventory。
2. 寫 route reachability、icon policy、state policy 測試。
3. 決定 legacy route redirect／淘汰表。

### Wave 1：設計系統與全域 shell

1. port buyer glass／breakpoint tokens。
2. common glass top bar、glass tab、floating bottom nav、dialog／sheet。
3. Material Icons + Logo asset contract。
4. loading／empty／error／offline／toast。

### Wave 2：五個主頁

1. 總覽。
2. 訂單。
3. 商店（店舖／商品／貼文）。
4. 錢包。
5. 設定。

### Wave 3：核心完整流程

1. 商品 wizard。
2. 訂單詳情／出貨／物流／售後。
3. 認證／PAYUNi。
4. 聊天／通知 deep link。

### Wave 4：CRM、優惠券、工具與清理

1. 顧客／訂閱／發券。
2. 貼文 feed／互動。
3. AI／photo／rich text。
4. 移除 legacy screens/assets，完成全路由 screenshot regression。

## 13. 本文件凍結前仍需確認的產品／後端能力

這些問題不阻擋視覺設計，但阻擋最終功能承諾：

- Wallet 是否已有可用餘額、待入帳、撥款紀錄 API；若沒有，新 UI 不可顯示假數字。
- 商品是否支援批次庫存／上下架；若只支援逐筆，先做多選 UI 但不可假裝批次成功。
- 商品與貼文草稿是否有後端 draft；若沒有，需定義 encrypted local draft 與跨裝置限制。
- Insights 是否有商品排行、流量、轉換 API。
- 客戶黑名單對訂單、聊天、發券的真實影響範圍。
- notification refType 的每一種 deep link 是否都有可取得 detail 的 API。
- PAYUNi request sent、審核 accepted、approved 的可觀測狀態與刷新方式。
- 停權是全能力鎖定或 mutation 級限制；UI 預設保留 read-only view。

## 14. 決策紀錄

| 日期 | 決策 |
|---|---|
| 2026-07-18 | 啟動頁只保留 Hawkr 品牌載入，不顯示牛牛歡迎頁 |
| 2026-07-18 | 允許重新設計每頁內容 layout，不受既有 Figma／畫面結構拘束 |
| 2026-07-18 | 先完成全頁功能目的、情境與設計書，再進入全面 UI 實作 |
| 2026-07-18 | 視覺方向為買家 App 品牌 + iOS 26 Liquid Glass 操作層 + 成熟商務 SaaS 資訊層級 |
| 2026-07-18 | UI icon 全面收斂至 `AppIcons`／`HkIcon`（Material Icons）；Logo 以品牌層方式融入，不破壞 icon 語意 |
| 2026-07-18 | Refero 因 `NO_SUBSCRIPTION` 暫時無法取得案例，後續恢復後補研究，不阻擋功能盤點 |
