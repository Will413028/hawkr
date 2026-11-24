# B4 追加簡報：分類清單（D8）、hashtag（D14）、描述格式（D17 重開）

快照 dev（2026-11-24）。標記同 `decision-brief.md`。

## 前提題

- 被改的程式：Java product-service 的分類種子、hashtag 表、描述 JSON；全部是 B4 重寫對象，不是限制（SELF-IMPOSED）。
- seller-app 已有區塊式描述編輯器（`seller-app/lib/shared/widgets/rich_text_editor.dart`：text、image、table、list 區塊，`product_setup_info_screen.dart` 使用【量】），buyer 端有從描述抽 hashtag 的 fallback（`buyer-app/lib/features/product/data/hashtag_extractor.dart`【量】）。我們擁有這些程式，「已經寫好」不是限制，但會改變各選項的成本。
- 分類樹的消費者：catalog（商品掛分類）、procurement（RFQ 品項解析到平台分類樹，存 `item_resolution_category_id`，resolver 自建索引，`synonyms.json` 目前空表，`backend/internal/modules/procurement/CARRYOVER.md` M14【量】）、buyer-app／website 的分類頁【量】。
- 重做量：procurement 經 `legacy/categoryservice` port 讀分類樹，B4-4 改接 catalog 時只換 adapter；`synonyms.json` 是空表，分類 ID 換掉不丟資料【量】。描述區塊的顯示已在 buyer-app（`merchant_item_detail_screen.dart:1648-1907`）與 website（`website/features/product/ProductDetailLayout.tsx:63`、`:152` table）【量】，seller-app 有編輯器；admin 未查。

## 限制帳

| 限制 | 分類 | 推離基線？ |
|---|---|---|
| 多語：平台內容存 `LocalizedText`（JSONB，必有 zh-TW），搜尋併入英文名（ADR `2026-10-17-multi-locale-architecture.md:118-119`） | REAL（已接受 ADR） | 否 |
| 讀取模型只在量測觸發後才建（`backend/AGENTS.md` 單體規則 2） | REAL（已接受規則） | 否（D14 A 照它） |
| 既有的 seller 編輯器、client 抽取 hashtag、Java 種子 | SELF-IMPOSED | 不作為理由，只影響成本 |
| procurement 已依分類樹做 resolver | SELF-IMPOSED（我們擁有） | 否：只換 adapter |

## D8 初版分類法從哪來

- 業界基線：
  - 不自創，採用公開、版本化的標準分類法再取子集：Shopify Standard Product Taxonomy（開源、MIT、每季版本、約 26 個頂層、含 Google 對照，GitHub `Shopify/product-taxonomy`【文件】）、Google product taxonomy（Merchant Center 用【文件】）。
  - 平台內只開放子集；每個分類保留標準 ID，日後匯出到 Google Merchant、比價網、廣告可直接對照。
  - 深度 2–3 層；商品只掛葉節點。
- 現有機制：Java migration 種子 3 筆（「蔬菜」「水果」等，`V1:271`【量】）；dev seed 另有 id 4–33 的小樹（procurement `CARRYOVER.md` M14【量】）；都沒有標準 ID。
- 選項：
  - A：Shopify 分類法的子集（頂層 8–12 個一般綜合電商常見類：服飾配件、美妝個護、3C 電子、居家生活、食品飲料、母嬰、運動戶外、寵物、文具辦公、玩具；各 2 層），內部主鍵 UUIDv7（D6），另存 `shopify_taxonomy_id text`（Shopify ID 是字串，例如 `aa-1`）當對外對照，升版時以 Shopify 的 mapping 檔更新對照欄、不動內部 ID；名稱存 `LocalizedText`（zh-TW 必填、en），搜尋併入英文名（多語 ADR）；父節點；版本化 migration。失敗條件：Shopify 的切法不符臺灣慣用（例如「美妝」在其下為 Health & Beauty）→ 名稱可在地化，結構照標準。（建議）
  - B：參考蝦皮臺灣類目自己編。失敗條件：沒有公開穩定 ID、無對外對照、日後要自己維護版本。
  - D：內部分類樹自己定，Shopify ID 只當對照欄。與 A 的差別只在結構是否照 Shopify；A 已是「內部 ID＋對照欄」，結構照標準可省掉自己維護的切法，故選 A。
  - C：Google product taxonomy 子集。與 A 接近；Shopify 版本有維護節奏與屬性定義（日後可加商品屬性），故選 A。
- 推翻的事實：要上架受管制品類（食品、保健、化妝品）→ 那幾個葉節點需加資格審核（scope-cut ADR 的重評條件）。
- 反悔代價：上線前低；上線後改結構要搬商品。
- 待交付：B4-2 開工前我起草葉節點清單（約 40–60 個）交 Will 看。

## D14 商品與貼文的 hashtag 要不要合併

- 業界基線：
  - 商品 tag 與內容 tag 是兩個用途：Shopify product tags 是賣家內部分類與篩選，不對買家做熱門排行【文件】；蝦皮商品描述可帶 hashtag（上限 18），點擊後是商品搜尋結果【文件】；Instagram／小紅書的 hashtag 頁聚合的是貼文，商品另以購物標籤掛在貼文上【文件】。
  - 跨內容與商品的統一標籤頁，需要一個擁有聚合結果的搜尋或讀取模型（小紅書、淘寶由搜尋系統負責）。
- 現有機制：
  - social 用明確欄位 `hashtag_inputs`，有 `SuggestTags`、`SuggestTrendingTags`、依標籤列貼文（`api/hawkr/social/v1alpha1/social.proto:49`、`:55`、`:389`【量】）。
  - 商品的 hashtag 寫在描述文字裡由 client 抽（上限 10），另有 Java 的 `SearchHashtags`、`GetPopularHashtags`（website `website/lib/search/repo.ts:234-235` 打 `/public/hashtags/popular`，`client-inventory.md` §6.1【量，盤點】）。
- 選項：
  - A：不合併資料與排行；統一「定義」：正規化規則一份（抽到 `platform`）、商品也改用明確欄位 `hashtag_inputs`（不從描述抽，刪 client 的抽取 fallback），上限照 social。商品的建議與熱門由 catalog 自己聚合；client 的標籤頁需要同時顯示貼文與商品時，在 client 或 httpapi 組兩個 API 的結果。失敗條件：產品要「跨貼文與商品的單一熱門排行」→ 需要讀取模型（觸發才建）。（建議）
  - B：合併成平台 hashtag：一個模組（discovery）擁有跨 social 與 catalog 的聚合與排行。失敗條件：跨 schema 聚合要讀取模型或 `pub_*`，現在沒有量測或產品需求支撐（違反「讀取模型只在量測觸發後才建」，`backend/AGENTS.md` 單體規則 2）。
  - C：照舊從描述抽 hashtag。失敗條件：兩套輸入模型、描述編輯會改 tag、伺服器與 client 各一份抽取規則。
- 推翻的事實：產品要求統一熱門排行。反悔代價：低（定義已統一，之後加讀取模型即可）。

## D17 描述格式（重開：v2 的成本前提不成立）

- v2 選「純文字＋描述圖片」的理由是「四端不需要 renderer 與編輯器」；但 seller-app 已有區塊編輯器（text、image、table、list），buyer 端已有對應的顯示【量，前提題】。理由消失，重新比較。
- 業界基線：多 client（Web＋原生 App）的 rich text 用**結構化 JSON 區塊**，不用 HTML：Shopify metafield `rich_text_field` 是 JSON AST、Sanity Portable Text、Contentful Rich Text、Notion blocks【文件】；HTML 要 sanitize 且原生端渲染不一致。純文字＋圖片是蝦皮做法，最簡但無清單、表格。
- 選項：
  - A：proto `oneof` 區塊：段落（純文字、無行內樣式）、圖片（media key＋alt）、項目清單、表格（限欄列上限）；伺服器驗證區塊數、文字長度、圖片引用（圖片引用與 `product_images` 同樣 `Confirm`／`MarkDeleted`）；四端依生成型別渲染。失敗條件：要行內粗體、連結→段落改成帶 marks 的 span（Portable Text 形狀），向後相容可加。（建議）
  - B：維持 v2 的純文字＋描述圖片。失敗條件：刪掉已有的清單、表格能力；賣家端編輯器要改寫成較少功能。
  - C：HTML。失敗條件：sanitize、XSS、App 端渲染。
- 推翻的事實：表格與清單沒有任何賣家或買家畫面在用（B4-1 確認）→ 去掉那兩種區塊。
- A 的成本：三端已有區塊顯示與編輯，改成讀生成型別；admin 若要顯示描述需新建（B4-3 查）。
- 反悔代價：低（區塊可加可減）。

## 終態與觸發

- D8：內部分類樹＋Shopify 對照；觸發受管制品類→加資格審核；觸發營運改分類→管理畫面。
- D14：各自聚合；觸發「跨貼文與商品的熱門排行」→ discovery 擁有讀取模型。
- D17：四種區塊；觸發行內樣式→段落改 span＋marks。
