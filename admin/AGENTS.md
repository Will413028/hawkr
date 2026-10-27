# Hawkr Admin — Next.js 後台管理系統

## 必讀文件與優先級

任何 AI 或自動化代理在修改本專案前，必須依任務讀取下列文件：

1. **所有 UI／UX 任務先讀 [`DESIGN.md`](./DESIGN.md)**：這是登入頁、Admin shell、管理模組與 ADM workspace 的通用視覺、元件、響應式及無障礙規範。
2. **涉及產品、安全、授權或 ADM 代行時再讀 [`PRODUCT.md`](./PRODUCT.md)**，以及對應的 `.impeccable/surfaces/` 與 server contract。
3. 優先級為：產品與安全契約 → `DESIGN.md` 通用規範 → feature surface/spec → 現行共用元件 → legacy page CSS。舊 Figma 註解、legacy variables 與歷史計畫不能覆蓋現行規範。

不要在本檔重抄色票；通用 tokens 只在 `DESIGN.md` 與 `src/styles/AdminVisualRefresh.css` 維護。若刻意改變設計系統，必須同步更新 `DESIGN.md` 和 `.impeccable/design.json`。

## 開發指令
- **啟動開發伺服器**: `pnpm dev`
- **正式環境編譯**: `pnpm build`
- **啟動正式環境**: `pnpm start`
- **Lint 檢查**: `pnpm lint`（`tsc --noEmit && biome check .` 加共用文案目錄的守門 `check-enum-text`，使用 Biome 取代 ESLint）
- **格式化**: `pnpm format`（`biome format --write .`）
- **測試**: `pnpm test`
- **共用文案目錄（proto enum 的使用者面文字）**: 在共用套件 `@hawkr/ui-text`（`packages/hawkr-ui-text-ts`），admin 不用 next-intl，直接用 `@hawkr/ui-text/keys` 的建構函式加 `@hawkr/ui-text/zh-TW` 的 `formatStaff`。enum 文字規則見根目錄 `AGENTS.md`，由 `pnpm lint` 的 `check-enum-text` 擋。畫面要顯示「未知」時用 `src/lib/ui-text.ts` 的 `staffTextOrUnknown`；錯誤顯示用 `src/lib/error-text.ts` 的 `errorMessage`，不寫 `userMessage(…) ?? "…"` 的後備句。
- **UI 圖示**: 共用套件 `@hawkr/icons`（`packages/hawkr-icons-ts`，Material Symbols），import 自 `@hawkr/icons/material-symbol` 的 `MaterialSymbol`，`name` 是 `MaterialSymbolName`，型別檢查守住圖示存在；新圖示在該套件的 `src/names.ts` 加名稱後跑 `pnpm --filter @hawkr/icons gen`，不得手改 `material-symbols.gen.ts`。不放 SVG 檔進 `public/`。

## 目錄結構
```
src/
├── app/                            # Next.js App Router
│   ├── admin/                      # 管理後台主體 (Sidebar + Topbar 佈局)
│   │   ├── merchants/              # 賣家管理與認證詳情
│   │   ├── banners/                # Banner 管理與 Fabric.js 編輯器
│   │   ├── category-audit/         # 分類審核
│   │   └── ...                     # 其他模組 (通知模板、AI 修圖、售後爭議等)
│   ├── api/
│   │   └── bff/[...path]/           # 同源 BFF，只轉送 allowlist headers，Access JWT 以 Bearer 帶給單體
│   └── layout.tsx                  # Root layout（self-hosted Roboto/openhuninn）
├── components/
│   ├── Layout/                     # 佈架元件 (Sidebar, Topbar)
│   ├── UI/                         # 通用元件 (Button, Badge, StatCard)
│   └── banner/, merchant/          # 功能專屬元件
├── services/                       # API client、固定環境設定與 server-only 後端 URL resolver
├── middleware.ts                   # 跨來源寫入防護
├── lib/, utils/                    # 工具函式 (cn helper)、Canvas 編輯邏輯
└── styles/                         # 全域樣式與頁面級 CSS
```

## 技術規範
- **核心架構**: Next.js 16 (App Router), TypeScript, pnpm。
- **樣式與動畫**: Tailwind CSS v4, Framer Motion。
- **編輯器與圖表**: Fabric.js (Canvas 編輯), Chart.js (數據分析)。
- **開發慣例**:
  - 後台頁面預設使用 `'use client'`（互動性強且無需 SEO）。
  - client 一律呼叫同源 `/api/bff` 或專用的 same-origin Admin route，見下方「環境」。
  - 跨來源的防護在後台自己這一層：`middleware.ts` 以 `src/lib/cross-origin.ts` 擋下 `/api/*` 的跨來源寫入（判準同後端的 `http.CrossOriginProtection`）。BFF 對後端是伺服器的請求，不轉送瀏覽器的 `Origin`，也不自己補一個。

## UI 實作契約

- 新介面先重用 `AdminBrand`、Layout、`components/UI`、`src/lib/a11y.ts` 與 `DESIGN.md` 的 semantic tokens；不得從 legacy `--color*`／`--brand-*` 建立新 palette。
- 網頁是 Logo 與品牌識別來源。只使用 `/public/brand/` 中的正式 SVG，並經 `AdminBrand` 選擇 light／dark／mark。
- 互動目標原則上至少 44px，focus 必須可見；dialog／mobile drawer 要 trap focus、背景 inert、支援 Escape 並 restore trigger focus。
- 對話框一律用共用的 `components/UI/Dialog`（`standard`、`operation`、`side` 三種外觀；focus trap、Escape、背景 inert、還焦、aria 與內容區捲動都在它裡面），不自己畫 overlay 或 `role="dialog"`（`tests/dialog-skeleton-guard.test.ts` 會擋）。短暫訊息一律用 `useToast()`（`components/UI/Toast`，`ToastProvider` 在 admin layout），色調 `success`／`warning`／`error` 走 token，頁面不自己放 toast 狀態、計時器或顏色；對話框開著時訊息會進最上層對話框內的 live region。
- Compound input 只有一層可見邊界。表格在窄螢幕於自身容器捲動，不能造成 document overflow。
- 所有新頁都要有真實 loading、empty、error、disabled／unavailable 狀態，不得用假資料或無功能控制填滿版面。
- 完成 UI 後至少執行 `pnpm lint`、`pnpm test`、`pnpm build`（與 CI 相同），並做 desktop、mobile 與 keyboard smoke。Local mock／截圖不是 DEV、CI、部署或 production proof。

## 關鍵機制
- **認證 (Auth)**: 後台網域由 Cloudflare Access 保護（Google 登入、email 白名單；ADR `backend/docs/adr/2026-09-27-identity-clerk.md` 文末修訂）。`CF_ACCESS_TEAM_DOMAIN` 與 `CF_ACCESS_ADMIN_AUD` 只在單體部署設定；workers.dev 與 preview 也須受保護或關閉。
  - 登入：Access 是登入畫面；`/` 進 `/admin`，layout 以 `IdentityAdminService.GetSession` 讀管理員資料，無法連線時顯示錯誤與重試。後端第一次看到 Access `sub` 建立管理員，email 僅供顯示；Access 白名單是唯一開通來源。
  - 呼叫後端：瀏覽器只打同源 `/api/*`。BFF 與代行專用 route 的 `adminSession.token(request)` 優先讀 Access 加上的 `Cf-Access-Jwt-Assertion`，沒有時才用 `ADMIN_DEV_TOKEN`，帶 `Authorization: Bearer` 給單體；不照抄瀏覽器的 header 或 cookie，BFF 不判斷身分。兩者皆無時 route 回 401。單體拒絕顧客 Clerk token 呼叫 `*AdminService`，也拒絕管理員 Access token 呼叫 `*AppService`。
  - 本機：以 `backend` 的 `make dev-token args='-admin -user dev_ops'` 產生 token，放進 `admin/.env.local` 的 `ADMIN_DEV_TOKEN`；後端的 `make serve` 使用同一把 dev key。部署時不設定 dev key 或 dev token；單體的 dev key 與 Access 設定互斥。
  - 登出（`AuthService.logout`）：先呼叫 `IdentityAdminService.SignOut` 撤銷進行中的代行；成功或回 `Unauthenticated`／`PermissionDenied` 時導向 `/api/admin/logout`，由 server 依當次 Access assertion 導往 Access logout，否則到本機 dev token 失效頁；其他錯誤留在原頁並顯示「登出失敗」。
  - 管理員的顯示資料由 connect-query 讀 `GetSession`，只在 query cache，不寫進 `localStorage`，也不作授權依據。middleware 只擋跨來源寫入；頁面入站由 Access 保護，管理員身分由單體判斷。
- **環境（固定 per deployment）**: dev/prod 各自 build+deploy，無 runtime 切換。後端由 server-side `BACKEND_BASE_URL` env var 決定（`services/backend.ts`，BFF 與代行 route 讀取，未設則是本機的單體 `http://localhost:8080`）。瀏覽器只打同源 `/api/bff`，免 CORS。
- **Canvas 編輯器**: Fabric.js 透過 `next/dynamic` 並設定 `{ ssr: false }` 載入，確保只在客戶端運行。

## ADM 代行營運不可跨越的邊界

- Admin 永遠是 actor，賣家／店家永遠是 target；UI 必須持續顯示 actor、target、reason、scope、expiry 與結束入口。
- 不取得、不交換、不儲存 seller token，也不呼叫 seller `/app` API。代行只走專用 same-origin handlers 與 server-issued operation session。
- 訂單與履約永久唯讀；出貨、取消、退款、收款、銀行、身分、密碼等能力不得出現在可寫路徑。
- Capability 必須在 UI、handler、adapter、route、bundle 與 gateway 一致 fail closed；hidden／disabled button 不是授權。
- Backend 未提供、工作階段失效、目標不符或結果不明時，顯示明確 unavailable／blocked 狀態，不能 fallback 成假成功。

<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->
