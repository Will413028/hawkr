# Hawkr Website — Next.js 買家與賣家網頁

## 開發指令
- **啟動開發伺服器**: `pnpm dev`
- **正式環境編譯**: `pnpm build`
- **Lint 與類型檢查**: `pnpm lint`（tsc、biome check 與架構／transport／dark mode 等檢查腳本）
- **測試**: `pnpm test`（Vitest）、`pnpm test:unit`（`tests/*.test.ts`，node:test）；E2E 用 `pnpm test:e2e`。單元測試不得連到真的主機（`tests/setup/hermetic.mjs` 擋非 loopback 連線；沒有 handler 的 msw 請求會失敗）；啟動子程序時用 `hermeticEnv()`（`tests/helpers/hermetic-env.ts`，`tests/hermetic-guard.test.ts` 檢查）。`pnpm run test:build-output` 檢查 `pnpm build` 預先渲染的 HTML。本機跑 `pnpm build` 要把兩個 API base 設成 loopback。
- **API 型別與 Connect 執行期**: 在共用套件 `@hawkr/api`（`packages/hawkr-api-ts`）：生成碼 import 自 `@hawkr/api/gen/…`，錯誤分類與重試是 `@hawkr/api/connect/errors`、`@hawkr/api/connect/retry`。改根目錄 `api/` 的 proto 後在根目錄跑 `pnpm --filter @hawkr/api gen:api`，不得手改生成檔
- **鄉鎮市區代碼表與查詢**: 在共用套件 `@hawkr/twaddr`（`packages/hawkr-twaddr-ts`），import 自 `@hawkr/twaddr/lookup`；資料檔由 `backend/internal/platform/twaddr` 生成，不得手改。
- **共用文案目錄**: 在共用套件 `@hawkr/ui-text`（`packages/hawkr-ui-text-ts`），import 自 `@hawkr/ui-text/keys` 與 `@hawkr/ui-text/format`。元件用 `useUiText()`、RSC 與測試用 `uiTextFor(locale)`（`lib/i18n/ui-text.ts`，不用 `getTranslations()`，否則 ISR 的頁會變動態）；回傳字串的純函式第一個參數收 `ut: UiText`。目錄的名詞對 UNSPECIFIED 與不認得的值回 undefined，要文字用 `optionalText(ut, m)`，要「未知」用 `commonUnknown()`；錯誤文字用 `@hawkr/api/connect/errors` 的 `errorText`。enum 文字規則見根目錄 `AGENTS.md`，由 `pnpm lint` 的 `check-enum-text` 擋。
- **佔位圖**: `pnpm gen:placeholders` 依 `scripts/gen-placeholders.ts` 的表與 `design-system/cross-platform-tokens.json` 的 `brand.*` 重新產生 `public/placeholders/*.svg`；`tests/placeholders.test.ts` 比對檔案、並檢查原始碼引用的本機素材路徑都存在
- **自動格式化**: `pnpm format`

## 目錄結構
```
app/                        # Next.js App Router；買家與賣家的頁面都在 app/[locale]/ 底下（語系是 URL 第一段，見「多語系路由」），
                            #   route groups：(public) 首頁與公開頁、(commerce) 購物、(account) 買家帳號、(auth) 認證、
                            #   (seller) 賣家中心、(content) 內容頁，另有 vendor、hawkr-agent-rfq-*；
                            #   不本地化的在 app/ 根：api/（route handlers）、(delegated)（代行工作區，只有 zh-TW，自己的根 layout）、
                            #   (canary)、(commerce)/store-map/return，以及 robots、sitemap、manifest、icon、apple-icon、
                            #   opengraph-image、global-error、global-not-found
components/                 # 共用 React 元件（shared/RootDocument.tsx 是整份 HTML 文件，根 layout 都用它）
features/                   # 頁面功能模組
lib/                        # 工具函式與資料層（SEO 結構化資料在 lib/seo/）
styles/                     # globals.css（import manifest）、foundation/、components/、features/、legacy/（待遷移）
assets/                     # TS/TSX import 的模組資產（目前只有字型）
public/                     # 靜態資源；placeholders/ 是 `pnpm gen:placeholders` 產生的佔位 SVG（表在 scripts/gen-placeholders.ts），不得手改，照片與插畫的位置都用它
docs/                       # 技術文件（directory-architecture.md、design-system-guidelines.md 等）
```

## CSS 現狀 & 遷移方向
- `styles/globals.css` 是 CSS import manifest + Tailwind `@theme`/token bridge，不要把頁面或元件樣式直接寫入（只放 `@import` 與 token 設定）。
- **進行中**：CSS → Tailwind CSS v4 utilities 遷移。把 CSS class 的樣式搬到 TSX 的 `className`，整檔遷移完成後刪除 `.css` 並移除 globals.css 的 import。

## 技術規範
- **核心架構**: Next.js 16 (App Router), TypeScript, pnpm。
- **樣式管理**: Tailwind CSS v4, Framer Motion (動畫)。
- **代碼規範**: Biome（設定在 `biome.json`，以 `pnpm lint`／`pnpm format` 為準）。
- **元件開發**:
  - 預設使用 **Server Components**，僅在有互動需求時使用 `"use client"`。
  - 元件命名與檔案名稱使用 PascalCase。
- **圖片處理**: 務必使用 Next.js `<Image>` 元件進行優化。
- **UI 禁止 emoji**: 圖示一律用 `components/shared/AppIcon.tsx`（語意名稱到 Material Symbols 的對照，圖形由共用套件 `@hawkr/icons` 繪製），沒有例外；`scripts/check-no-emoji.ts` 強制。
- **SEO**: 透過 `lib/seo/jsonld.ts` 提供結構化資料，並在 page/layout 中定義 metadata。URL 一律經 `lib/seo/urls.ts` 的 builder 並帶 locale（不手拼）；頁面的 `generateMetadata` 用 `pageMetadata({ title, description, path })`（`lib/seo/metadata.ts`，讀 `params.locale`，頁面仍是靜態）。

## 多語系路由
決策與理由在 ADR `backend/docs/adr/2026-10-17-multi-locale-architecture.md` D1；admin 與 staff 介面不在內。
- **網址**：zh-TW 在 `/…`、en 在 `/en/…`（next-intl `as-needed`，設定在 `i18n/routing.ts`）。URL 是唯一的語系來源：不依 `Accept-Language` 或 cookie 導向（`localeDetection:false`、`localeCookie:false`），不輸出 next-intl 的 `Link` header（`alternateLinks:false`，hreflang 由 metadata 產生）。明寫的 `/zh-TW/…` 導向 `/…`。
- **語系清單在 `lib/intl/locales.ts`**：`supportedLocales`；每個介面一份已發佈清單 `publishedLocales`。介面由 `surfaceOfPath`（`lib/intl/paths.ts`）依去掉前綴的路徑分：`seller` 是 `/seller`（含公開的招募頁 `/seller/pricing`，受眾是想開店的賣家）、`/seller-center`、`/seller-product-import-preview`；其餘都是 `buyer`（買家看的賣家資訊 `/sellers/*`、`/seller-guide`、`/seller-terms` 也是）。「已發佈」和「會路由」分開：prod flavor 只路由已發佈的（`/en` 回 404，連 `/en/…` 前綴都不認），dev flavor 路由全部支援的語系，未發佈的 `en` 才能在開發與 preview 測。hreflang、sitemap、`/en` 的索引規則只看已發佈清單。把 `en` 加進某個介面的清單，條件是該介面在英文下看不到任何未翻譯的文字（ADR 發佈門檻），而且 `LAUNCH_BLOCKERS`（同一個檔案）已經清空：`tests/locale-launch-gate.test.ts` 擋；發佈前要補的項目在計畫 `backend/docs/plans/2026-09-29-a3-buyer-seller.md` 的 I-web-0 條目。測試換清單用 `__setPublishedLocalesForTest`。
- **根 layout 有兩個**：`app/[locale]/layout.tsx`（`<html lang>` 跟語系，值是 `zh-TW`、`en`）與 `app/(delegated)/layout.tsx`（`/adm`，固定 zh-TW）；整份文件與 provider 樹在 `components/shared/RootDocument.tsx`。找不到任何路由的 404 是 `app/global-not-found.tsx`（Next 不傳 params，固定 zh-TW）。route 之內的 `notFound()` 由 `app/[locale]/not-found.tsx` 依語系渲染。
- **middleware**（`middleware.ts`）：Clerk 在最外層；站內的判斷（Agent gate、賣家 canonical 308、登入守衛）在去掉語系前綴的路徑上做，導向一律帶回前綴。**不本地化的路徑**在 `lib/intl/paths.ts`（`NON_LOCALIZED_PREFIXES`、`NON_LOCALIZED_EXACT`）：新增不屬於 `app/[locale]` 又沒有副檔名的 route 要加進去，否則會被改寫成 `/zh-TW/…` 而 404。**matcher 是封閉清單**：`public/` 新增頂層目錄要同步 matcher（`tests/middleware-matcher.test.ts`），`app/[locale]` 底下由程式產生圖片的 metadata route 要列在 `isMetadataImagePath`（`tests/metadata-image-routes.test.ts`）。
- **`next.config.ts` 的路徑規則**（導向、私有路由的 no-store）會為每個會路由的非預設語系多一份 `/<locale>/…` 的版本（prod 沒有，規則不變）；新增路徑規則時照既有的 helper（`withLocalizedRedirects`、`privateRouteSource`）寫，別只寫無前綴的。
- **寫新頁面**：放在 `app/[locale]/` 底下；RSC 讀語系從 `params.locale`（`localeFromParam`），不用 `getTranslations()`／`getLocale()`（會讀 header，把 ISR 的頁變動態）；文字用 `uiTextFor(locale)`。
- **API 語系**：Connect transport 送單一值的 `Accept-Language`（`lib/connect/transport.ts` 的 `acceptLanguageInterceptor`，所有 transport）。瀏覽器端取自 `AppProviders` 依路由設的目前語系（`lib/intl/active-locale.ts` 的 `setActiveLocale`／`activeLocale`）；**不讀 `<html lang>`**，瀏覽器翻譯會改寫它，API 收到的語系會更新帳號的 `preferred_locale`。server 端（SSR、ISR）沒有「目前頁面」（同一個程序同時服務很多請求，模組變數共用，所以不設不讀），呼叫的人要明確傳 `locale`（來源是它渲染的路由參數），沒傳就丟錯。
- **導覽層**（`lib/intl/`、`lib/navigation/`）：站內路徑在程式裡一律不含語系，出口加一次讀者的語系前綴；已帶前綴、外部網址、絕對網址（含同源）、`/api`、`/adm`、`public/` 與 Next 產生的檔案原樣通過。**不要在呼叫點手拼 `/en`**。出口：
  - 連結：`components/shared/Link`；要保留 `next/link` 自己行為的用 `lib/intl/native-link`。兩者以外不直接 import `next/link`；
  - router：`useRouter()`（`lib/intl/use-router.ts`）或 `useObservedRouter`（不直接用 `next/navigation` 的 `useRouter`）；HeroUI 元件的 `href` 經 `components/shared/HeroUIProvider.tsx` 的 `RouterProvider`；
  - server 導向：`redirect({ href, locale })`（`locale` 取自 `params.locale`）；
  - 整頁導覽：`commitDocumentNavigation()`（不直接寫 `window.location`）；刻意的整頁載入、經 prop 傳進來的站內路徑、`<form action>`、`window.open`、`history.pushState/replaceState` 的目標都過 `useLocalizedHref()`／`localizeHref()`；
  - 讀路徑：用 `lib/intl/navigation.ts` 的 `usePathname()`（next-intl 的），不用 `next/navigation` 的，否則 hydration 不一致（ADR 修訂）；只有 effect 裡要真實網址的 analytics 例外；
  - 依瀏覽器網址判斷頁面類型的純函式（`window.location.pathname` 進來的）先 `stripLocale`（`lib/intl/paths.ts`），規則只寫一份；內嵌 boot script 用 `STRIP_LOCALE_SCRIPT`。
  `tests/navigation-layer.test.ts` 擋直接繞過以上出口的寫法。元件測試的 `next/navigation` mock 用 `importOriginal` 保留其他匯出，`usePathname` 需要 `NextIntlClientProvider`（`tests/mocks/intl-test-wrapper.ts`；`withAppRouter` 已包含）。

共通規則（UI 改動的授權、Git、不留技術債）見根目錄的 `AGENTS.md`。

## UI 元件工作流程

- UI 工作前先讀 `DESIGN.md`、`components/README.md` 與相關的正式元件，重用既有的品牌與語意 token。
- 起 `pnpm storybook`，用 `hawkr-storybook` MCP（`http://127.0.0.1:6006/mcp`：`docs-list`、`docs-show`、`get-storybook-story-instructions`）；工具不能熱載入時用 `pnpm storybook:mcp <tool> '<json>'`。不自創元件 props，import 用實際的 `@/components/...`。
- 共用 UI 的改動在同一次變更內更新 stories 與 API 文件；用 MCP `test-run` 看實際渲染，並跑 `pnpm storybook:typecheck`（`storybook:test` 交給 CI）。
- 跨平台的改動看 `design-system/cross-platform.md`，跑 `pnpm design:check --apps-root ..`，App 文件用 `pnpm storybook:sync-app-docs` 同步；Flutter iframe 的 props 不是 Dart widget API，以 App 的 API 文件與原始碼為準，widget 測試在各 App 目錄跑。
- 預覽要 deterministic、不連真實交易。

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
