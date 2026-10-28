# SEO 技術契約詳細設計書

## 0. 文件元資訊 (Metadata)

| 項目 | 值 |
|---|---|
| 維護者 | Will |
| 對應程式碼 | `website/lib/seo/`、`website/app/robots.ts`、`website/app/sitemap.ts`、`backend/internal/platform/revalidate`、`backend/services/java/product-service/.../domain/text/DescriptionTextExtractor.java` |
| 相關 doc | §02 產品微服務（`pure_description`/`searchable_text` 欄位定義）、§47 GA4 分析追蹤（姊妹主題：本文管「被搜尋引擎發現」，47 管「被追蹤成效」） |
| 相關 ADR | [structured data 缺資料省略欄位、canonical 由 route param 重建](../adr/2026-09-18-seo-structured-data-and-canonical-source.md)、[店家頁的 SEO 由平台從可見資料生成](../adr/2026-10-09-store-seo-platform-generated.md) |

---

## 1. 概述與目標 (Overview & Goals)

### 1.1 文件定位

本文件描述 website 與後端之間的 SEO **技術契約**：website 端的 metadata／JSON-LD／robots／sitemap 模組責任與邊界、商品頁與店家頁 title／description 的生成規則、店家資料變動到網站快取失效的鏈路、以及控制全站是否可被索引的開關。

### 1.2 服務一句話定位

本文件不對應單一可部署服務，而是跨 repo 的 SEO 主題文件（比照 §47 GA4 的編寫方式）。

### 1.3 設計目標 (Goals) 與 非目標 (Non-Goals)

| 類型 | 項目 |
|---|---|
| **Goal** | 定義 website 技術 SEO 的模組責任與契約：metadata／JSON-LD／canonical／robots／sitemap 各由誰產生、輸入從哪來 |
| **Goal** | 定義商品頁與店家頁 `<title>`／`<meta description>`／og 的生成規則：由 website 純函式從頁面看得到的資料生成，沒有手寫欄位，缺的欄位省略，不補空字串或樣板 |
| **Goal** | 定義索引開關（`env.seoIndexable`）的判定條件與失效行為 |
| **Non-Goal** | **不**涵蓋 `hawkr_buyer`/`hawkr_seller` Flutter app 的 ASO（App Store Optimization）或 deep link 索引——App 不被搜尋引擎爬取，屬不同領域 |
| **Non-Goal** | **不**做 SEO 成效分析（流量、排名、轉換率）——那是 §47 GA4 的範疇，本文件只管「技術上能不能被發現、能不能被正確索引」 |

### 1.4 與其他設計書／服務的關係

- §02 產品微服務：`pure_description`/`searchable_text` 欄位的完整定義與萃取規則見該文件 §3.6；本文件只定義它們在 SEO 鏈路中的角色（§5.1）。
- §47 GA4 分析追蹤：與本文件是跨 repo 觀測/成長類主題的姊妹文件——47 管「使用者行為被追蹤」，48（本文件）管「頁面被搜尋引擎發現與索引」。

---

## 2. 範圍與情境 (Context & Scope)

### 2.1 系統情境圖 (C4 Context)

```mermaid
flowchart LR
  Crawler["搜尋引擎 Crawler\n（Googlebot 等）"] -->|HTTP GET| Site[website]
  Site -->|REST / Connect| BE["後端（單體入口、gateway）"]
  BE --> SELLER["seller 模組（stores）"]
  BE --> ProductSvc[product-service]
  SELLER -.->|POST /api/revalidate\n（River job，bust cache tag）| Site
```

### 2.2 邊界與責任

| | 涵蓋 (In-scope) | 不涵蓋 (Out-of-scope，由誰負責) |
|---|---|---|
| 責任 | website 技術 SEO（metadata/JSON-LD/robots/sitemap、title／description 生成）；店家資料變動到網站快取失效的鏈路；product-service 的 SEO 相關欄位現況 | App ASO/deep link（未來另立文件）；內容行銷與外部連結建設（行銷團隊）；SEO 成效分析（§47 GA4） |

### 2.3 角色與身份模型

- **搜尋引擎 crawler**：匿名，透過 `robots.txt`/`sitemap.xml` 探索與索引公開頁面。
- **一般訪客**：無 SEO 相關操作權限，僅為 SEO 優化的最終受益對象（透過搜尋結果找到頁面）。

---

## 3. 約束與前提 (Constraints & Assumptions)

| 類型 | 項目 |
|---|---|
| 技術約束 | Launch gate：`env.seoIndexable`（`lib/config/env.ts`）僅在 `flavor === "prod"` 且 `NEXT_PUBLIC_SEO_INDEXABLE === "true"` 時為真；未達成時 `app/robots.ts` 回傳 `disallow: /`，全站不可索引 |
| 既有系統約束 | `hawkr-website` 商品/賣家資料走 Next.js `'use cache'`（`cacheLife("seller")`/`cacheTag`），非每次請求即時打後端 |
| **假設 (Assumptions)** | 僅服務台灣市場、單一語言 zh-TW（`messages/` 只有一份語系檔）。本文件不含 i18n／hreflang 契約；拓展海外市場時本假設失效 |

---

## 4. 架構 (Architecture)

### 4.1 解法總綱 (Solution Strategy)

SEO 分兩條線：

1. **website 技術 SEO**（root 靜態設定 + per-page 動態 metadata）：`lib/seo/*` 模組化——`metadata.ts` 產生 root/per-page metadata、`jsonld.ts` 產生各類 structured data、`urls.ts` 是 canonical/OG url 的唯一來源（不從 request URL 取，避免追蹤參數混入）、`indexability.ts` 定義 noindex 路由清單，統一供 `robots.ts`/`sitemap.ts` 與各頁 `generateMetadata` 使用。
2. **title／description 由平台生成**：商品頁與店家頁的 `<title>`、`<meta description>`、og:title、og:description 由 `lib/seo/product-summary.ts`、`lib/seo/store-summary.ts` 從頁面看得到的資料生成，兩者共用 `lib/seo/summary-rules.ts`（門檻、長度上限、截斷、title 片段去重各一份）；沒有手寫欄位。店家頁 title 是 `{店名} | {縣市}{鄉鎮市區} | Hawkr`，空片段略去、相鄰相同的片段只出現一次；description 在店家介紹夠長（10 字，以 code point 計）時取介紹的摘要，否則由所在地與評分組句，缺的欄位省略。店家資料變動時，後端以 on-demand revalidate webhook bust 網站快取（§6.4）。

### 4.2 元件視圖 (C4 Component)

```mermaid
flowchart TB
  subgraph web["website"]
    URLS["lib/seo/urls.ts\n(canonical 唯一來源)"] --> MD["lib/seo/metadata.ts\n(buildRootMetadata/buildPageMetadata)"]
    URLS --> JSONLD["lib/seo/jsonld.ts\n(Organization/WebSite/Product/\nBreadcrumb/Seller/FAQ schema)"]
    RULES["lib/seo/summary-rules.ts"] --> PS["product-summary.ts"]
    RULES --> SS["store-summary.ts"]
    PS --> MD
    SS --> MD
    IDX["lib/seo/indexability.ts\n(noindex 路由清單)"] --> ROBOTS[app/robots.ts]
    IDX --> SITEMAP[app/sitemap.ts]
    MD --> PAGE["各頁 generateMetadata\n(getCachedSeller/getCachedProduct)"]
    JSONLD --> PAGE
  end
  subgraph backend["後端"]
    SELLERMOD["seller 模組\n(店家資料寫入)"] -->|revalidate job| REVALIDATE["/api/revalidate\n(bust seller tag)"]
  end
  REVALIDATE -.-> PAGE
```

| 模組/package | 責任 | 依賴 |
|---|---|---|
| `lib/seo/metadata.ts` | 產生 root/per-page `<title>`/`<meta description>`/OG/Twitter card | `urls.ts` |
| `lib/seo/jsonld.ts` | 產生 Organization/WebSite/Product/Breadcrumb/Seller/FAQ structured data（`schema-dts` 型別檢查，缺資料時省略欄位不捏造） | `urls.ts` |
| `lib/seo/urls.ts` | canonical/OG url 建構。**一律由 route param 重建，不得從 request URL 取**（追蹤參數會污染 canonical，理由見 [ADR](../adr/2026-09-18-seo-structured-data-and-canonical-source.md)） | — |
| `lib/seo/indexability.ts` | `INDEXABLE_STATIC_ROUTES`（sitemap 用）、`PRIVATE_PREFIXES`（noindex 但可爬，讓連結權重流動） | — |
| `lib/seo/summary-rules.ts` | 商品頁與店家頁共用的 description 規則：門檻、長度上限、截斷、title 片段去重 | — |
| `lib/seo/product-summary.ts`、`lib/seo/store-summary.ts` | 從商品或店家頁看得到的資料生成 title／description（meta 與 og 是同一個值） | `summary-rules.ts` |
| seller 模組 + `internal/platform/revalidate` | 店家資料寫入的 transaction 內排入 revalidate job，commit 後呼叫 `POST /api/revalidate` | website revalidate route |

### 4.3 部署拓撲與 Port

N/A——本文件涉及的皆為既有服務既有 port，無新增部署單元。

### 4.4 命名／契約規約

- Cache tag：`seller:{id}`，id 是網址上的 `store_id` 或 `store_code`，兩個都發（`lib/seo/tags.ts` `sellerTag()` 與單體 `internal/platform/revalidate.StoreTags` 需保持一致，為前後端共用的 webhook 契約）。

---

## 5. 資料設計 (Data Design)

### 5.1 資料模型

| 表 | 欄位 | 型別 | 說明 |
|---|---|---|---|
| `product`（product-service） | `pure_description` | `text` | 從商品 description JSONB 萃取的純文字（見 §02 §3.6），作為 `searchable_text` 的組成之一，間接供 `pg_trgm` 檢索（非直接建索引）。**product-service 內部欄位，不經 gRPC／Gateway 對外**——網站端商品 meta description 由 website 自行以 `productSeoDescription()` 從 `product.description` 萃取 |
| `product`（product-service） | `searchable_text` | `text` | `name` + `pure_description` + 分類名 + SKU 名 + hashtag 彙整，供站內搜尋 GIN index 用，非傳統搜尋引擎 SEO |

### 5.2 索引與查詢模式

N/A——本文件涉及欄位皆沿用既有索引設計（`pg_trgm` GIN index 見 §02），無新增查詢路徑。

### 5.3 序號／排序／一致性

N/A。

### 5.4 Schema 演進與遷移

N/A——店家沒有 SEO 專屬欄位，title／description 由 website 生成。

---

## 6. API 與介面契約 (API & Interface Contracts)

### 6.1 對外 REST（經 Gateway）

| 路由 | 用途 | 備註 |
|---|---|---|
| 公開賣家 API（既有） | 網站 `generateMetadata` 讀取店家資料 | 取店名、介紹、所在地（`town_code` 經 `@hawkr/twaddr/lookup` 轉縣市與鄉鎮市區名稱）與評價統計，輸入 `store-summary.ts` 生成 title／description |

### 6.2 gRPC（服務間）

N/A——沒有 SEO 專屬的服務間呼叫。

### 6.3 非同步事件 (MQ)

N/A——店家資料變動的快取失效走 §6.4 的 revalidate webhook，非 MQ 事件。

### 6.4 快取失效 Webhook

| 項目 | 值 |
|---|---|
| 呼叫方 | 單體的 seller 模組（`internal/platform/revalidate`，River job），與 `review-service`／`product-service` 共用同一套 webhook 契約（見「後端服務責任邊界規範」§9.7） |
| 端點 | `hawkr-website` `POST /api/revalidate` |
| Payload | cache tag 清單（本情境為 `seller:{store_id}` 與 `seller:{store_code}`） |
| 觸發時機 | 由 seller 模組決定，見 `docs/modules/seller.md` §12.1 |
| 失敗行為 | 重試與停用條件見 `docs/modules/seller.md` §12.1；停用或重試用盡時前端快取不失效，需等 `cacheLife("seller")` TTL 到期 |

### 6.5 Next.js Route Handler（非後端 API）

`app/robots.ts`、`app/sitemap.ts`——依 `env.seoIndexable`／`INDEXABLE_STATIC_ROUTES` 產生，純前端 repo 內部路由，不經 Gateway。

`sitemap.ts` 目前只收錄 `INDEXABLE_STATIC_ROUTES`；動態實體（商品／賣家／食譜）URL 的收錄追蹤於 #3493。

---

## 7. 執行時視圖 (Runtime View)

### 7.1 主要場景（happy path）

**場景 1：搜尋引擎爬取**——crawler 先讀 `robots.txt` 確認 `seoIndexable` 為真，再讀 `sitemap.xml` 取得可索引 URL 清單，逐一 GET 頁面取得 HTML（含 `<title>`/`<meta>`/JSON-LD）。

**場景 2：店家資料更新**——店家資料寫入後透過 webhook bust 對應店家的網站快取，下次請求即以新資料重新生成 title／description（非等待 TTL）。

### 7.2 時序圖

```mermaid
sequenceDiagram
  participant S as 賣家
  participant BE as 單體（seller 模組）
  participant J as River job
  participant WEB as website
  S->>BE: 更新店名／介紹等
  BE->>BE: 寫入 + 排入 revalidate job（同一個 transaction）
  BE-->>S: 回應
  J->>WEB: POST /api/revalidate（seller:{store_id}、seller:{store_code}）
  Note over WEB: 下次請求 getCachedSeller 讀到新值，store-summary 重新生成
```

### 7.3 失效與重試

| 外部呼叫 | 逾時/重試 | 冪等 | 降級行為 |
|---|---|---|---|
| revalidate job → `/api/revalidate` | 逾時與重試次數見 `docs/modules/seller.md` §12.1 | 冪等（bust 同一 tag 重複執行無副作用） | Webhook 未設定或重試用盡 → 前端維持舊快取直到 `cacheLife("seller")` TTL 到期，非阻斷性降級 |
| JSON-LD 缺資料欄位 | N/A | N/A | `productSchema()`/`sellerSchema()` 缺 price/rating 等資料時直接省略該欄位，不捏造假值（理由見 [ADR](../adr/2026-09-18-seo-structured-data-and-canonical-source.md)） |

### 7.4 並發與順序

N/A——皆為請求時渲染（RSC `generateMetadata`），無長連線或並發競態疑慮。

### 7.5 連線／狀態生命週期

N/A（無狀態服務）。

---

## 8. 跨切面關注點 (Crosscutting Concerns)

### 8.1 安全與授權

- **XSS/meta injection 風險評估**：店名與店家介紹是使用者可控輸入，生成的 title／description 最終寫入 `<title>`/`<meta>`。網站端透過 Next.js `Metadata` API 回傳純字串物件（`generateMetadata`），由框架序列化輸出，非 `dangerouslySetInnerHTML` 或字串拼接注入 HTML，框架層自動 escape → **風險已由既有實作路徑規避，非待辦項**。

### 8.2 可觀測性 (Observability)

SEO 的把關點在 **CI**，不在 runtime：

- Lighthouse SEO score gate：`lighthouserc.mobile.cjs` / `lighthouserc.desktop.cjs`，`categories:seo` 門檻 `minScore: 0.9`。
- Playwright SEO 契約測試套件：`tests/seo/`。

**沒有 runtime 告警**：索引覆蓋率下滑、爬取錯誤率上升不會觸發任何通知。SEO 退化的偵測依賴人工查看搜尋引擎主控台。

### 8.3 錯誤處理與降級 (Failure Modes & Degradation)

見 §7.3。

### 8.4 一致性與冪等

店家資料更新的快取失效為 at-least-once bust（job 在 transaction 內排入，失敗由 River 重試，不阻斷主流程），透過 tag 冪等特性確保重複 bust 無副作用。

### 8.5 設定與機密

| Env Var | 用途 | Per-env 差異 |
|---|---|---|
| `NEXT_PUBLIC_SEO_INDEXABLE` | 是否允許搜尋引擎索引（配合 `flavor === "prod"` 共同決定 `env.seoIndexable`） | 非 prod flavor 恆為 false；prod 需額外設此變數為 `"true"` 才開啟索引，即使已上 prod 也預設關閉直到手動開關 |
| `WEB_REVALIDATE_BASE_URL` / `WEB_REVALIDATE_SECRET`（單體） | revalidate webhook 目標與驗證密鑰 | 未設定時 revalidate 停用（見 §6.4） |

---

## 9. 品質需求與 NFR (Quality Requirements / SLO)

### 9.1 SLO / SLI

| 指標 (SLI) | 目標 (SLO) | 量測點 |
|---|---|---|
| Lighthouse SEO score | ≥ 0.9（低於即 CI 失敗） | CI `lighthouserc.*.cjs` |

### 9.2 容量與規模 (Capacity & Scaling)

N/A——SEO 相關運算皆為輕量字串/schema 產生，無容量瓶頸疑慮。

### 9.3 效能預算

N/A。

---

## 10. 部署與維運 (Deployment & Operations)

### 10.1 環境與設定

見 §8.5 環境變數表。`NEXT_PUBLIC_SEO_INDEXABLE` 需在 prod 環境手動設為 `"true"` 才會真正開放索引，是正式上線 SEO 前必須確認的開關，非部署自動生效。

### 10.2 遷移步驟

N/A——本文件涉及欄位皆已上線穩定，無待遷移項目。

### 10.3 健康檢查與優雅關閉

N/A。

### 10.4 Runbook

N/A。

---

## 11. 測試策略 (Test Strategy)

| 層級 | 覆蓋 | 關鍵場景 |
|---|---|---|
| CI gate | Lighthouse SEO score | `lighthouserc.mobile.cjs`/`lighthouserc.desktop.cjs`，門檻 `minScore: 0.9`，抽測首頁/市集/FAQ/商品頁/賣家頁 |
| e2e (Playwright) | canonical、JSON-LD、img-alt、h1 唯一性、social title | `tests/seo/entity-templates.seo.spec.ts`、`tests/seo/h1-uniqueness.seo.spec.ts`、`tests/seo/social-title.seo.spec.ts`，獨立跑於 `playwright.seo.config.ts` |

---

## 12. 術語表 (Glossary)

| 術語 | 定義 |
|---|---|
| SEO | Search Engine Optimization，搜尋引擎優化 |
| ASO | App Store Optimization，App 商店搜尋優化（本文件 Non-Goal，見 §1.3） |
| JSON-LD | 一種 structured data 格式，供搜尋引擎理解頁面實體語意（本專案用 `schema-dts` 型別檢查產生） |
| Canonical | `<link rel="canonical">`，告知搜尋引擎該頁面的權威 URL，避免重複內容判定 |
| OG (OpenGraph) | 社群平台（Facebook 等）分享預覽用的 meta 標籤協定 |
| hreflang | 告知搜尋引擎同一內容的不同語言版本對應關係的 meta 標籤 |
| Slug | URL 中語意化、可讀的識別字串（如 `/products/fresh-apple` 而非純數字 ID） |
| ISR / `'use cache'` | Next.js 的快取重新驗證機制，本文件情境為 on-demand tag-based revalidate |
| Lighthouse | Google 提供的網頁品質稽核工具，含 SEO 分類評分 |
| Core Web Vitals | Google 定義的網頁使用者體驗核心指標（LCP/INP/CLS） |
