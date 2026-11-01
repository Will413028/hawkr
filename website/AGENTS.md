# Hawkr Website — Next.js 買家與賣家網頁

## 開發指令
- **啟動開發伺服器**: `pnpm dev`
- **正式環境編譯**: `pnpm build`
- **Lint 與類型檢查**: `pnpm lint`（tsc、biome 與各檢查腳本）
- **測試**: `pnpm test`（Vitest）、`pnpm test:unit`（`tests/*.test.ts`，node:test）；E2E `pnpm test:e2e`。單元測試不得連真的主機（`tests/setup/hermetic.mjs` 擋非 loopback；沒有 handler 的 msw 請求會失敗）；子程序用 `hermeticEnv()`。`pnpm run test:build-output`（`tests/build-output/`：靜態外殼、404、預渲染模式，CI 會跑）檢查 `pnpm build` 的產物；改根 layout 在 `<Suspense>` 外的元件後必跑 `pnpm build`，本機兩個 API base 設成 loopback。
- **共用文案目錄**: 在共用套件 `@hawkr/ui-text`（`packages/hawkr-ui-text-ts`），import 自 `@hawkr/ui-text/keys` 與 `@hawkr/ui-text/format`。元件用 `useUiText()`、RSC 與測試用 `uiTextFor(locale)`（`lib/i18n/ui-text.ts`；`getTranslations()`／`getLocale()` 會讓 ISR 的頁變動態）；回傳字串的純函式第一個參數收 `ut: UiText`。名詞對 UNSPECIFIED 與不認得的值回 undefined：要文字用 `optionalText(ut, m)`，要「未知」用 `commonUnknown()`；錯誤文字用 `@hawkr/api/connect/errors` 的 `errorText`。enum 文字規則見根目錄 `AGENTS.md`，`pnpm lint` 的 `check-enum-text` 擋。
- **佔位圖**: `pnpm gen:placeholders` 依 `scripts/gen-placeholders.ts` 與 `design-system/cross-platform-tokens.json` 的 `brand.*` 產生 `public/placeholders/*.svg`；`tests/placeholders.test.ts` 比對
- **自動格式化**: `pnpm format`

## 目錄結構
```
app/                        # Next.js App Router；買家與賣家頁面在 app/[locale]/（語系是 URL 第一段，見「多語系路由」），
                            #   route groups：(public) 公開頁、(commerce) 購物、(account) 買家帳號、(auth) 認證、
                            #   (seller) 賣家中心、(content) 內容頁，另有 vendor、hawkr-agent-rfq-*；
                            #   不本地化的在 app/ 根：api/、(delegated)（代行工作區，只有 zh-TW，自己的根 layout）、
                            #   (canary)、(commerce)/store-map/return、robots/sitemap/icon 等 metadata 檔、global-error、global-not-found
components/                 # 共用 React 元件（shared/RootDocument.tsx 是整份 HTML 文件，根 layout 都用它）
features/                   # 頁面功能模組
lib/                        # 工具函式與資料層（SEO 結構化資料在 lib/seo/）
styles/                     # globals.css（import manifest）、foundation/、components/、features/、legacy/（待遷移）
assets/                     # TS/TSX import 的模組資產（只有字型）
public/                     # 靜態資源；placeholders/ 是產生物（見「佔位圖」），不得手改，照片與插畫的位置都用它
docs/                       # 技術文件
```

## CSS 現狀 & 遷移方向
- `styles/globals.css` 是 CSS import manifest + Tailwind `@theme`/token bridge，只放 `@import` 與 token，不寫頁面或元件樣式。
- **進行中**：CSS → Tailwind v4 遷移。樣式搬到 TSX `className`，整檔遷完刪除 `.css` 與 globals.css 的 import。

## 技術規範
- **核心架構**: Next.js 16 (App Router), TypeScript, pnpm。
- **樣式管理**: Tailwind CSS v4, Framer Motion (動畫)。
- **代碼規範**: Biome（設定在 `biome.json`，以 `pnpm lint`／`pnpm format` 為準）。
- **元件開發**:
  - 預設使用 **Server Components**，僅在有互動需求時使用 `"use client"`。
  - 元件與檔案名稱用 PascalCase。
- **圖片處理**: 務必使用 Next.js `<Image>` 元件進行優化。
- **UI 禁止 emoji**: 圖示一律用 `components/shared/AppIcon.tsx`（語意名稱對照 Material Symbols，圖形由 `@hawkr/icons` 繪製），沒有例外；`scripts/check-no-emoji.ts` 強制。
- **SEO**: 結構化資料用 `lib/seo/jsonld.ts`，metadata 在 page/layout 定義。URL 一律經 `lib/seo/urls.ts` 的 builder 並帶 locale（不手拼）；`generateMetadata` 用 `pageMetadata({ title, description, path })`（`lib/seo/metadata.ts`，讀 `params.locale`，頁面仍是靜態）。

## 多語系路由
細節見 `docs/i18n-routing.md`；決策在 ADR `backend/docs/adr/2026-10-17-multi-locale-architecture.md` D1；admin 與 staff 介面不在內。
- **網址**：zh-TW 在 `/…`、en 在 `/en/…`；URL 是唯一的語系來源，不看 `Accept-Language` 或 cookie。語系清單與各介面的已發佈清單在 `lib/intl/locales.ts`；把 `en` 加進已發佈清單前 `LAUNCH_BLOCKERS` 要清空（`tests/locale-launch-gate.test.ts`）。
- **寫新頁面**：放在 `app/[locale]/`；RSC 從 `params.locale`（`localeFromParam`）讀語系、文字用 `uiTextFor(locale)`，不用 `getLocale()`。
- **不本地化的路徑**：不在 `app/[locale]` 又沒副檔名的 route 要加進 `lib/intl/paths.ts` 的 `NON_LOCALIZED_PREFIXES`／`NON_LOCALIZED_EXACT`，否則被改寫成 `/zh-TW/…` 而 404；`public/` 新增頂層目錄要同步 middleware 的 matcher（`tests/middleware-matcher.test.ts`）；`next.config.ts` 路徑規則照 `withLocalizedRedirects`、`privateRouteSource` 寫。
- **API 語系**：server 端（SSR、ISR）呼叫要明確傳 `locale`，沒傳就丟錯；瀏覽器端取自 `lib/intl/active-locale.ts`，不讀 `<html lang>`（翻譯會改寫它）。
- **部署與訪客 IP**：部署在 Vercel（區域 `hnd1`，設定在 `vercel.json`）。訪客 IP 只經 `lib/http/visitor-ip.ts` 的 `getVisitorIp(request)` 讀（`x-real-ip`，Vercel 覆寫、client 偽造不了）；不讀其他 client 位址 header（Vercel 不管的 header 會原樣透傳，client 偽造得了）。
- **導覽層**：站內路徑不含語系，**不要在呼叫點手拼 `/en`**。出口：連結 `components/shared/Link`（要 `next/link` 行為用 `lib/intl/native-link`）；router `useRouter()`（`lib/intl/use-router.ts`）；server 導向 `redirect({ href, locale })`；整頁導覽 `commitDocumentNavigation()`；其他站內目標過 `useLocalizedHref()`／`localizeHref()`；讀路徑用 `lib/intl/navigation.ts` 的 `usePathname()`；依網址判斷頁面類型先 `stripLocale`。守門 `tests/navigation-layer.test.ts`，mock 見 `docs/i18n-routing.md`。

共通規則（UI 改動的授權、Git、不留技術債）見根目錄的 `AGENTS.md`。

## 登入狀態
伺服器決定；機制見 `README.md`「顧客登入」。E2E 等解析用 `data-auth-status`。
- **來源**：`lib/auth/dal.ts` 的 `verifySession()`（不在 `'use cache'` 內呼叫）。根 layout 不 await，promise 交給 `AuthProvider`（`lib/auth/context.tsx`，不 suspend）。
- **四態**：`unknown | guest | signedIn | unavailable`。`useAuth()` 沒有「是否登入」的布林（`unknown` 會被算成沒登入）：用 `status` 分支，`identity` 隨之收斂。只有確定的答案才是 `guest`；網路錯誤、逾時、5xx 是 `unavailable`，**不是登出**。
- **公開頁**：`unknown` 顯示中性等待（隱藏或骨架），不先當訪客；等答案的點擊用 `useAuthGate`。帳號查詢的 fetcher 經 `lib/auth/account-query.ts` 的 `accountQueryFn(status, fn)`（非 `signedIn` 是 `skipToken`），`status` 取自 `useAccountStatus()`，載入中用 `accountQueryPending`；不用 `enabled:`、自己的布林或 `placeholderData`（`tests/auth-state-source-guard.test.ts` 守）。
- **受保護區**：server layout 用 `components/shared/ZoneGate.tsx`（`lib/auth/zone.ts`）導向，`middleware.ts` 只看 cookie。`ZoneGate` 把本次請求的身分與工作區當 prop 給 `ZoneIdentityProvider`（`lib/auth/zone-context.tsx`）：`useZoneIdentity()` 讀身分（非 null，zone 外丟錯）；區內 query key 用它加工作區，`useAccountStatus()` 恆為 `signedIn`，不受根層舊答案影響；`unavailable` 顯示 `ZoneUnavailable`。
- **身分改變的事件**（`signOut(to, replace)`、401）由 `AuthProvider` 清私有資料後整頁導向，呼叫端不導航；`router.refresh()` 由最新的 promise 勝出。跨分頁：已登入分頁聽到帳號不同或登出就重載，訪客分頁聽到登入只 `router.refresh()`；bfcache 還原的已登入頁重載；非 signedIn 收 401 不導。

## UI 元件工作流程

- UI 工作前先讀 `DESIGN.md`、`components/README.md` 與相關元件，重用既有品牌與語意 token。
- 起 `pnpm storybook`，用 `hawkr-storybook` MCP（`http://127.0.0.1:6006/mcp`：`docs-list`、`docs-show`、`get-storybook-story-instructions`）；工具不能熱載入時用 `pnpm storybook:mcp <tool> '<json>'`。不自創元件 props，import 用實際的 `@/components/...`。
- 共用 UI 改動同一次變更內更新 stories 與 API 文件；用 MCP `test-run` 看渲染，跑 `pnpm storybook:typecheck`（`storybook:test` 交給 CI）。
- 跨平台改動看 `design-system/cross-platform.md`，跑 `pnpm design:check --apps-root ..`；App 的 API 文件頁在 build 時讀各 App 的 `docs/component-library.md`；Flutter iframe 的 props 不是 Dart widget API，以 App 的 API 文件與原始碼為準，widget 測試在各 App 目錄跑。
- 預覽要 deterministic，不連真實交易。

<!-- BEGIN:nextjs-agent-rules -->

## This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
