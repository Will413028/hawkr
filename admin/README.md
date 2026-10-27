# Hawkr 後台管理系統

Hawkr 的後台管理平台，用於管理商戶、會員、商品、訂單、Banner、文案等業務資料。

## 設計與 AI 開發規範

- [`DESIGN.md`](./DESIGN.md)：通用 UI、品牌、元件、響應式與無障礙的唯一設計規範。
- [`AGENTS.md`](./AGENTS.md)：AI 實作優先級、工程契約與驗證要求。
- [`PRODUCT.md`](./PRODUCT.md)：產品脈絡與 ADM 代行營運的安全邊界。

新增或修改介面時先讀 `DESIGN.md`；涉及 ADM、權限或敏感操作時再讀 `PRODUCT.md` 與對應 feature spec。

## 技術棧

| 項目 | 技術 |
|------|------|
| Framework | Next.js 16 (App Router) |
| Language | TypeScript |
| UI | React 19 + Tailwind CSS v4 |
| Animation | Framer Motion |
| Charts | Chart.js |
| Canvas | Fabric.js、Konva（Banner 編輯器，dynamic import） |
| 套件管理 | pnpm |

## 功能模組

### 已上線
- **賣家管理**：列表、篩選、認證審核（身分證 / 存摺等證件）、PAYUNi 金流狀態同步、批准 / 停用 / 刪除
- **分類審核**：賣家提交的商品分類審核
- **Banner 管理**：列表 + Fabric.js/Konva 編輯器
- **通知模板**：模板管理
- **AI 修圖**：AI Style 管理
- **文案管理**：站內文案編輯

### 待 API
- Dashboard、會員、商品、交易、售後爭議、數據分析、SEO

## 架構重點

### 認證

- 後台網域由 **Cloudflare Access** 保護（Google 登入、email 白名單；ADR `backend/docs/adr/2026-09-27-identity-clerk.md` 文末修訂）。`/` 直接進入 `/admin`，layout 以 `IdentityAdminService.GetSession` 讀管理員資料；後端無法連線時顯示錯誤與重試。管理員身分只由單體驗證 Access JWT。
- 瀏覽器只呼叫同源 `/api/*`。BFF 與代行專用 route 讀取 Access 加上的 `Cf-Access-Jwt-Assertion`，作為 `Authorization: Bearer` 送到單體；不照抄瀏覽器的 `Authorization` 或 cookie，也不在 BFF 判斷身分。
- 登出先呼叫 `IdentityAdminService.SignOut` 撤銷進行中的代行；成功或回 `Unauthenticated`／`PermissionDenied` 時導向 `/api/admin/logout`，由 server 依 Access assertion 前往 Access logout，或在本機顯示重新產生 dev token 的指示。其他錯誤留在原頁並顯示「登出失敗」。
- 這項撤銷保證只涵蓋後台按鈕登出。離職處置須從 Access 白名單移除、在 Zero Trust 撤銷該使用者的 Access sessions，並從後台 UI 撤銷該管理員進行中的代行；外部 Access 登出不會自動呼叫 `SignOut`。若撤銷步驟未完成，代行受 30 分鐘 hard TTL 與 15 分鐘閒置逾時限制。
- 管理員的顯示資料（email）由 connect-query 讀 `IdentityAdminService.GetSession`，只在 query cache，不寫進 `localStorage`，不得用來判斷登入或授權。
- `middleware.ts` 只擋跨來源的 `/api/*` 寫入；整個網域的入站驗證由 Access 處理。

### 環境（固定 per deployment）
- dev / prod 各自 build+deploy，無 runtime 切換
- 後端（單體的入口）由 server-side `BACKEND_BASE_URL` env var 決定（`services/backend.ts`）；未設則是本機的單體 `http://localhost:8080`
- 部署時只在單體設定 `CF_ACCESS_TEAM_DOMAIN`（完整 `https://<team>.cloudflareaccess.com`）及 `CF_ACCESS_ADMIN_AUD`（application AUD tag）。後台 BFF 不讀這兩項；後台網域、workers.dev 與 preview 必須同受 Access 保護或關閉後兩者。
- 本機在 `admin/.env.local` 設 `ADMIN_DEV_TOKEN`，值由 `backend` 執行 `make dev-token args='-admin -user dev_ops'` 產生；單體以相同 `DEV_AUTH_KEY_FILE` 執行 `make serve`。部署時不設定 dev key 或 dev token；單體在 dev key 與 Access 設定並存時會拒絕啟動。
- 瀏覽器只打同源 `/api/bff`，無 CORS 問題

### API 路徑（經 Gateway proxy）
```
/api/v1/admin/*    # 需 ADMIN 角色
/api/v1/public/*   # 匿名可存取
/api/v1/app/*      # 需登入
```

## 專案結構

```
src/
├── app/                            # Next.js App Router
│   ├── layout.tsx                  # Root layout
│   ├── globals.css                 # 全站 base 與 legacy compatibility bridge
│   └── admin/                      # 後台頁面（共享 Sidebar + Topbar layout）
│       ├── merchants/              # 賣家管理 + [id] 認證詳情
│       ├── category-audit/
│       ├── banners/                # Banner 列表 + editor/
│       ├── notification-templates/
│       ├── ai-styles/
│       ├── dev-tools/
│       └── (disputes | transactions | members | products | analytics | seo)/
├── components/
│   ├── Layout/                     # Sidebar、Topbar
│   │   └── AdminBrand.tsx          # 官方 SVG Logo 的唯一 UI 入口
│   ├── UI/                         # Button、StatusBadge、StatCard…
│   ├── magicui/                    # magicui 元件
│   ├── merchant/
│   └── banner/                     # Banner 編輯器（Fabric.js / Konva）
├── services/
│   ├── api.ts                      # 同源 API wrapper（timeout、envelope）
│   ├── auth.ts                     # 登出（SignOut → Access）
│   ├── app-env.ts                  # build-time APP_ENV
│   └── backend.ts                  # server-only BACKEND_BASE_URL resolver
├── middleware.ts                    # 跨來源寫入防護
├── styles/
│   └── AdminVisualRefresh.css      # canonical Admin visual layer 與 semantic tokens
└── types/ · utils/ · lib/
```

## 本機開發

```bash
pnpm install          # 在 repo 根目錄（website 與 admin 共用一個 workspace 與 lockfile）
cd admin              # 以下指令在 admin 目錄執行
pnpm dev              # http://localhost:3000
pnpm build
pnpm start            # 跑 production build
pnpm lint
```

## 部署

目前沒有部署環境：公司時期的 dev / production Worker 環境（`env.*`）、custom domain 與部署 workflow 已刪除。
`wrangler.jsonc` 只留頂層設定（`pnpm preview`、`pnpm cf-typegen` 用）；部署設定等選定 demo 部署目標時再加。
`BACKEND_BASE_URL` 是 runtime 變數，沒設時打本機單體；`NEXT_PUBLIC_APP_ENV` 為 build-time。

## 慣例

- **Path alias**：`@/` → `src/`
- **後端 URL**：不要硬編碼；server-side 讀 `BACKEND_BASE_URL`，client-side 一律打同源 `/api/bff`
- **頁面元件**：一律 `'use client'`（後台不需 SSR）
- **Banner 編輯器**：Fabric.js/Konva 必須 `next/dynamic` + `{ ssr: false }` 載入
- **設計系統**：通用規則依 [`DESIGN.md`](./DESIGN.md)，semantic CSS 由 `src/styles/AdminVisualRefresh.css` 提供；不要從 legacy aliases 建立新樣式。
- **驗證**：介面變更至少通過 `pnpm lint`、`pnpm test`、`pnpm exec next build --webpack` 與 desktop/mobile/keyboard smoke。

## 授權

Copyright © 2025-2026 Hawkr. All rights reserved.
