# Hawkr Website — Next.js 買家與賣家網頁

使用 Next.js 16（App Router）、TypeScript、Tailwind CSS v4 構建的 Hawkr 買家與賣家網頁。

## 快速開始

### 前置需求
- Node.js 26（版本看 repo 根目錄 `.nvmrc`）
- pnpm 12+（repo 根目錄 `package.json` 以 `packageManager: pnpm@12.15.0` 鎖定）

### 安裝依賴
website 與 admin 是同一個 pnpm workspace，在 repo 根目錄安裝（lockfile 是根目錄的 `pnpm-lock.yaml`）：
```bash
pnpm install   # 在 repo 根目錄
cd website
```

### 啟動開發伺服器
```bash
pnpm dev
```
應用將在 [http://localhost:3000](http://localhost:3000) 啟動。Chrome、Firefox 在 `localhost` 接受 Secure cookie，直接用 http 即可；Safari 或其他需要 HTTPS 的情況改跑 `pnpm dev --experimental-https`（https://localhost:3000）。

> 第一次跑前請先建立 `.env.local`（見下方「配置教學」），否則 flavor / API base 會走預設值。

## 開發指令

| 指令 | 說明 |
|------|------|
| `pnpm dev` | 啟動開發伺服器（Turbopack，含自動重載） |
| `pnpm build` | 編譯正式環境 standalone build |
| `pnpm start` | 啟動正式環境伺服器 |
| `pnpm lint` | `tsc --noEmit` 型別檢查 + Biome lint（針對 app/components/features/lib/scripts/tests/...） |
| `pnpm format` | Biome 自動格式化整個專案 |
| `pnpm architecture:check` | 跑 `scripts/check-architecture.ts` 驗證目錄邊界規則 |
| `pnpm dark-mode:check` | 跑 `scripts/check-dark-mode.ts` 檢查 dark mode 樣式 |
| `pnpm knip` | 偵測未使用的檔案 / export / 依賴 |
| `pnpm test:unit` | Node test runner 跑 `tests/*.test.ts` 單元測試 |
| `pnpm test:e2e` | 執行 Playwright E2E 測試 |
| `pnpm test:e2e:ui` | 以 UI 模式執行 E2E 測試 |
| `pnpm build:seo` | 以 prod flavor + indexable 旗標 build（SEO 驗證用） |
| `pnpm test:seo` | 跑 SEO 專用 Playwright suite（`playwright.seo.config.ts`） |
| `pnpm lhci:desktop` / `pnpm lhci:mobile` | Lighthouse CI（desktop / mobile 設定） |
| `pnpm smoke:google` | 線上驗證 Google Client ID 是否為正確 prod 值 |

## 目錄結構

```
app/                        # Next.js App Router boundary（只放路由與 metadata）
├── [locale]/               # 買家與賣家頁面；語系是 URL 第一段（zh-TW 無前綴、en 在 /en），根 layout
│   ├── (public)/           # 首頁、market、feed、search、seller storefront
│   ├── (commerce)/         # cart、checkout、orders、payment
│   ├── (account)/          # buyer account workspace
│   ├── (seller)/           # seller public entry + seller-center
│   ├── (auth)/             # login、register、auth callback
│   └── (content)/          # FAQ、legal、content pages
├── (delegated)/            # 代行工作區 /adm（只有 zh-TW、不本地化，自己的根 layout）
├── api/                    # App Router route handlers
├── sitemap.ts              # 動態 sitemap
├── robots.ts               # robots.txt
├── manifest.ts             # PWA manifest
├── global-not-found.tsx    # 找不到任何路由的 404
└── opengraph-image.tsx     # 預設 OG image（不本地化）

assets/                     # TS/TSX import 的模組資產
└── fonts/                  # 字型

components/                 # 跨 app 共用 React UI
├── ui/                     # 無業務語意 design-system primitives
├── shared/                 # Hawkr 跨 feature 共用元件
└── seo/                    # JSON-LD / structured-data 元件

features/                   # 產品功能模組與 route-facing screens
├── account/  cart/  checkout/  feed/  market/  seller-center/  ...

lib/                        # infrastructure 層
├── config/                 # env.ts / flavor.ts — flavor 驅動的設定來源
├── api/                    # API client
├── auth/                   # GIS 登入、active-role、phone OTP、redirect
├── seo/                    # jsonld / metadata / urls / indexability / tags
├── media/                  # OSS image loader 等
├── content/  services/  theme/  utils/  ...

public/                     # 瀏覽器 URL 直接讀取的靜態檔（brand/placeholders/...）

styles/                     # Tailwind entry、foundation、legacy/global CSS
├── globals.css             # import manifest + Tailwind @theme/token bridge（不寫頁面樣式）
├── foundation/  components/  features/  legacy/
```

更完整的目錄邊界規則請看 `docs/directory-architecture.md`（並由 `pnpm architecture:check` 強制）。

## 配置教學

### 1. 建立本機環境變數

設定範本在 `.env.example`，複製一份到 `.env.local` 再依環境補值：

```bash
cp .env.example .env.local
```

`.env.local` 已被 `.gitignore` 排除，請勿把 secret commit 進 repo。`NEXT_PUBLIC_*` 會被寫入瀏覽器 bundle，只能放可公開的值；沒有 `NEXT_PUBLIC_` 前綴的變數只留在 server / runtime。

### 2. Flavor 驅動的設定

本專案的環境設定**以 `NEXT_PUBLIC_FLAVOR` 為核心**：`lib/config/env.ts` 依 flavor 決定 timeout、logging 與缺值時的預設。網域與主機一律來自環境變數，程式裡沒有任何站台或媒體主機的常數：

| Flavor | 站台 origin（`NEXT_PUBLIC_WEB_ORIGIN`） | 後端 API |
|--------|----------|----------|
| `dev`（預設） | 未設就用 `http://localhost:3000`（`pnpm dev`） | http://localhost:8080（本機單體） |
| `prod` | 必填，缺值時啟動／build 失敗並指名變數 | `API_BASE_URL`（絕對網址，含 `/api/v1`）與 `NEXT_PUBLIC_API_BASE_URL`，無預設，缺值同樣失敗 |

預設 `dev`，避免缺設定時誤選 production。

### 3. 環境變數說明

| 變數 | 用途 | 本機是否必填 |
|------|------|--------------|
| `NEXT_PUBLIC_FLAVOR` | 環境選擇器 `dev` / `prod`；推導其餘設定 | 否（預設 `dev`） |
| `API_BASE_URL` | Server 端呼叫後端的 API base URL（絕對網址，含 `/api/v1`）。dev 留空則用本機單體；**prod 必填** | 否 |
| `NEXT_PUBLIC_API_BASE_URL` | Browser 端 API base URL；`next build` 時寫入 bundle。dev 留空則走同源 proxy；**prod 必填** | 否 |
| `NEXT_PUBLIC_WEB_ORIGIN` | 這份部署的站台 origin（canonical、sitemap、CSRF 同源檢查與「這是不是正式網域」的判斷都取自它）。只能是 origin（無路徑、query、帳密）。dev 留空用 `http://localhost:3000`；**prod 必填** | 否 |
| `NEXT_PUBLIC_ADM_ORIGIN` | ADM 的 origin（代行工作區「返回 ADM」）。dev 留空用 `http://localhost:3001`；**prod 必填** | 否 |
| `MEDIA_UPLOAD_ORIGINS` | 預簽名上傳（`PUT`）的目的地 origin，逗號分隔的 `https://` origin，host 可帶一個 `*.` 開頭（虛擬主機式 bucket）。只用於 build 時產生 CSP `connect-src`；沒設就不在 CSP 內（上傳會被瀏覽器擋下） | 否 |
| `NEXT_PUBLIC_GOOGLE_CLIENT_ID` | Google Identity Services 前端 client id（廠商入口的二次驗證 GIS 按鈕） | 廠商入口需要 |
| `NEXT_PUBLIC_FIREBASE_*` | Firebase Web SDK 設定，**僅供 phone OTP 註冊綁手機，不參與 OAuth** | 手機註冊需要 |
| `NEXT_PUBLIC_MEDIA_BASE_URL` | 圖片／媒體 gateway 來源（R2 + Cloudflare Worker），也是 app 唯一認得的自家媒體主機。**留空 = 沒有媒體 origin**，object key 不會組出任何網址，圖片就是不顯示（不會退到別的主機） | **要在本機跑 E2E 或 `pnpm build` 就必填**（dev 值 `https://test-media.example.com`；見下方第 8 節） |
| `NEXT_PUBLIC_LINE_URL` | 客服 LINE 連結（聯絡頁的 LINE 項目）；須為 `https://` 網址。**留空 = 不顯示任何 LINE 入口**（不會退到預設帳號） | 否 |
| `APPLE_PAY_DOMAIN_ASSOCIATION` | Apple Pay 網域驗證檔內容（單行）；由 `/.well-known/apple-developer-merchantid-domain-association(.txt)` 提供，未設回 404 | 否 |
| `ORIGIN_AUTH_SECRET` | backend#2264 限流身分簽章金鑰。server 端每次打 gateway 時附上 `X-HK-Origin-Auth`，讓 gateway 知道這次呼叫該算在哪個訪客（或算成 SSR）頭上，而不是把全站流量算在 Worker 的單一出口位址上。Worker binding **確實不存在時**才是不簽章；candidate upload 的 secrets file 是 additive，僅省略 GitHub secret 會繼承既有 binding，不會自動切成 unsigned。目前沒有部署環境，此項沒有現行操作。CI 用的是另一把 `ORIGIN_AUTH_CI_SECRET`（只在 GitHub Actions step 上，不進 Worker） | 否（binding 不存在時只是粒度退化） |
| `REVALIDATE_SECRET` | 後端服務 POST `/api/revalidate` 時帶的 `x-revalidate-secret`；producer 只送 `product:<id>` / `seller:<id>` 等 entity tag，web 邊界自動展開為對應的 aggregate tags。失效強度由 tag 形狀決定：entity tag 立即過期（寫入者下一次讀取即為新內容），aggregate 列表維持 stale-while-revalidate 以保留重生去重與後端故障時的 stale 兜底 | server 端 ISR busting 需要 |

> 認證已**不再使用 Better Auth / 獨立 DB**。舊版的 `BETTER_AUTH_*`、`BFF_SESSION_*`、`DB_*`、`NEXT_PUBLIC_AUTH_DRIVER` 已移除，無需設定。顧客登入走單體的 Google / LINE OAuth 與 HttpOnly 的 session cookie（見下方「顧客登入」）。

最小本機設定可從這份開始：

```dotenv
NEXT_PUBLIC_FLAVOR=dev

# 通常不需覆寫（flavor 會推導），需指向自架後端時才設
API_BASE_URL=
NEXT_PUBLIC_API_BASE_URL=

# 廠商入口的 Google 二次驗證
NEXT_PUBLIC_GOOGLE_CLIENT_ID=

# 僅手機 OTP 註冊需要
NEXT_PUBLIC_FIREBASE_API_KEY=
NEXT_PUBLIC_FIREBASE_AUTH_DOMAIN=
NEXT_PUBLIC_FIREBASE_PROJECT_ID=
NEXT_PUBLIC_FIREBASE_STORAGE_BUCKET=
NEXT_PUBLIC_FIREBASE_MESSAGING_SENDER_ID=
NEXT_PUBLIC_FIREBASE_APP_ID=

# server runtime（ISR cache busting）
REVALIDATE_SECRET=
```

### 4. Firebase 與 OAuth 設定

- **Firebase**：僅用於 phone OTP（手機註冊綁定）。Firebase Console → Project Settings → Web app config 取值；Authentication → Sign-in method 啟用 Phone，Authorized domains 加入 `localhost` 與正式網域；開發時可設 test phone numbers 節省簡訊額度。
- **顧客登入（Google、LINE）**：由單體處理（ADR `backend/docs/adr/2026-10-30-identity-self-hosted-federated-login.md`），website 沒有 secret 也沒有登入用的環境變數。登入頁（`features/auth`）把瀏覽器整頁導向 `/api/auth/{provider}/start?return_to=…`（經同源反代到單體），單體完成 OAuth 後設 HttpOnly 的 `__Host-hk_session` cookie 並 302 回登入頁；失敗時 302 回 `/login?error=<代碼>`。OAuth client 與 redirect URI 在單體的設定與各 provider 的 console，不在這裡。登入與否由 `GetSession` 判斷（`lib/auth/context.tsx`），登出是 `SignOut`；middleware 只看 cookie 在不在（樂觀導向，不打 API）。伺服器端（RSC、server action）只讀取：把訪客的 cookie 轉送給單體的 GET，寫入一律由瀏覽器送出（`lib/auth/session-cookie.server.ts`）。
- **廠商入口的 Google 二次驗證**：GCP Console → APIs & Services → Credentials → OAuth 2.0 Client ID（Web application），把本機與正式網域加入 Authorized JavaScript origins。

### 5. Next.js 配置 (`next.config.ts`)

重點：

- ~~`output: "standalone"`~~：已隨遷移 Cloudflare Workers 移除（commit `45084b4a`）；產物由 OpenNext 產出，不再是 Node standalone。
- ~~`cacheComponents: true`~~ / ~~`cacheLife`~~：**已於 #338 全數移除**（commit `6015754f`「關閉 cacheComponents 全站回傳統純 ISR」）。根因是 Cache Components 的 static-shell resume 在 workerd 會爆 128MB → prod 白畫面 + OOM。
  - **現行快取模型＝傳統 ISR**：各路由用 `export const revalidate = <秒>`（例：`app/[locale]/(public)/page.tsx` 首頁 60 秒）；資料層新鮮度在 `lib/api/cached.ts`（`unstable_cache` per fetcher，帶 tag ＋ revalidate TTL 安全網）。
  - `POST /api/revalidate`（帶 `REVALIDATE_SECRET`）以 tag 提前 bust，tag 名見 `app/api/revalidate/route.ts`。
  - ⚠️ **不要重新加回 `cacheComponents` / `experimental.dynamicIO` / `ppr`**；`'use cache'` 那套 cacheLife profile 已不適用。邊緣/持久層設定改在 `open-next.config.ts`。
- `images`：custom loader → Alibaba OSS image service（`lib/media/ossImageLoader.ts`），用 OSS 的 resize/quality/WebP pipeline 取代 Next 內建 optimizer（已移除舊的 `unoptimized: true`）。`qualities` 需先列出才能當 prop 用。
- `turbopack.root`：明確指定專案根目錄。
- `devIndicators: false`：隱藏左下角 dev-tools 指示器（dev-only）。
- `rewrites()`：Apple Pay domain-association 檔轉到 `/api/apple-pay/...`。
- `redirects()`：舊入口 301 導到 `/market` / `/feed` namespace。

新增 redirect 直接擴充 `redirects()`：

```typescript
async redirects() {
	return [
		{ source: "/old-page", destination: "/new-page", permanent: true },
	];
}
```

### 6. 部署設定（Cloudflare Workers / OpenNext）

目前沒有部署環境：公司時期的 dev / production Worker 環境（`env.*`）、secrets 清單、observability 目的地、
附屬 Worker 與部署 workflow 已刪除。部署設定等選定 demo 部署目標時重寫。

- **執行環境**：Cloudflare Workers（`@opennextjs/cloudflare` 轉譯 Next 產物）。
- **配置檔**：`wrangler.jsonc` 只留頂層設定（bindings / R2 / Durable Objects，本機 preview 與 `pnpm cf-typegen` 用）＋ `open-next.config.ts`（incrementalCache / tagCache / queue / cachePurge）。
- **環境變數**：`NEXT_PUBLIC_*`（含 `NEXT_PUBLIC_FLAVOR`、API base、Google/Apple/Firebase）在 **build 階段**寫入 browser bundle；server-only 值（`REVALIDATE_SECRET` 等）本機放 `.dev.vars`（見 `.dev.vars.example`），不落 repo 明文。
- **本機開發**：`pnpm dev` 即可；要在本機驗 Workers runtime 行為才需 OpenNext 本地模擬，一般開發不需要容器。

### 7. 工具配置

- `biome.json`：統一格式化與 lint，Tab 縮排、雙引號，啟用 React Hooks 檢查。
- `tsconfig.json`：開啟 `strict`、`noEmit`，`@/*` 路徑別名。
- `playwright.config.ts` / `playwright.seo.config.ts`：E2E 與 SEO 專用測試設定。
- `lighthouserc.desktop.js` / `lighthouserc.mobile.js`：Lighthouse CI 設定。
- `knip.json`：未使用程式碼 / 依賴偵測。
- `scripts/check-architecture.ts`：目錄邊界守門（`pnpm architecture:check`）。

### 8. 本機跑 E2E（Playwright）

CI 在跑 E2E 之前，會由 workflow **自己覆寫一份 `.env.local`**，把幾個變數補齊。
本機不會發生這件事，所以「照 `.env.example` 複製 → `pnpm test:e2e`」會紅在**環境**而不是程式碼上——
而且症狀看起來全都像程式壞了。完整前置如下：

```bash
# 1) .env.local 必須有 media gateway 來源（否則不會組出任何圖片網址）
echo 'NEXT_PUBLIC_MEDIA_BASE_URL=https://test-media.example.com' >> .env.local

# 2) 登入不需要任何 key：e2e 的已登入 spec 設 `__Host-hk_session` cookie 並 mock GetSession（tests/e2e/helpers/session-cookie.ts）。
#    Google client id 由 playwright.config.ts 以假值帶入（廠商入口的 GIS 由 E2E mock 掉）。

# 3) 用系統 Chrome 跑（本機沒有 CI 那顆 chrome-headless-shell）
PW_CHANNEL=chrome pnpm test:e2e

# 只跑單一檔案 / 單一 project：
PW_CHANNEL=chrome pnpm exec playwright test tests/e2e/site-stability.spec.ts --project=mobile
```

| 前置 | 沒設會怎樣（症狀） |
|------|--------------------|
| `NEXT_PUBLIC_MEDIA_BASE_URL=https://test-media.example.com` | 沒有媒體 origin → `buildImageUrl` 不組網址 → 圖片全部缺席。`site-stability.spec.ts` 斷言頁面的圖片都載得到，於是**乾淨的樹也必紅** |
| `PW_CHANNEL=chrome` | 找不到瀏覽器。CI 裝的是 `--only-shell`（chrome-headless-shell），本機通常沒有；本機到 Playwright CDN 實質不通，硬抓會卡很久 |
| port 3000 淨空 | `playwright.config.ts` 本機是 `reuseExistingServer: true`——3000 上若是別的東西，Playwright 會直接拿它當受測站，測到的不是這份 code |
| 連得到本機單體（`localhost:8080`） | `@live` 標籤那批打真後端。要跳過改用 `pnpm test:e2e:hermetic` |

**不必自己設的（列出來是為了讓人不要照抄 CI 的設定）**：

| CI 有寫、本機不用寫 | 為什麼 |
|---|---|
| `NEXT_PUBLIC_API_BASE_URL`、`API_BASE_URL`、`PLAYWRIGHT_E2E_API_PROXY` | `playwright.config.ts` 的 `webServer.env` 已經在本機注入這三個（同源 base + `test-api` 後端 + proxy 開關）。在 `.env.local` 再寫一份只會製造兩個真實來源 |
| `NEXT_PUBLIC_FIREBASE_*`（6 個） | **沒有任何應用程式碼讀它們**，`firebase` 也不在 `package.json` 依賴裡；CI 設定帶著它們是既有慣性。本機 E2E 不受影響 |

> 這張表本身有過一次教訓（2026-08-03）：它列了 media/瀏覽器/port/後端四項，卻漏了
> `NEXT_PUBLIC_GOOGLE_CLIENT_ID`，於是一輪全量本機 E2E 出現 26 支登入測試 timeout。
> 那個失敗樣態看起來完全像「登入功能壞了」——**而這一節存在的唯一目的，就是讓人不要
> 把環境問題誤判成程式壞掉**。漏一項，它就製造出自己要防的那個誤判。新增 CI 專屬環境
> 變數時，請一併回來補這裡。

`pnpm dev` 服務的是 `http://localhost:3000`；Playwright 本機另外自己起一支
`next dev`（見 `playwright.config.ts` 的 `webServer`，port 由 `PORT` 決定），兩者不是同一個 server，port 相同時會被 `reuseExistingServer` 重用。

> ⚠️ **本機跑的是 `next dev`，CI 跑的是 production build（`pnpm build` + `pnpm start`）。**
> 這個不對稱會製造「本機紅、CI 綠」而且看起來像程式壞掉的失敗。實例（2026-08-03）：
> 註解裡一個 Tailwind class 佔位符讓 `globals.css` 在 Turbopack 下 parse 失敗，
> 本機全量 E2E 98 紅、CI 全綠，因為 production build 容得下同一份產物。
> 本機看到大量非預期失敗時，**先看 `[WebServer]` 那幾行有沒有編譯錯誤**再懷疑測試本身；
> `tests/tailwind-comment-candidates.test.ts` 就是為了讓這一類問題在 `pnpm test:unit` 就先紅。

## 技術規範

### 核心架構
- **框架**：Next.js 16（App Router、Cache Components）
- **語言**：TypeScript（strict）
- **包管理**：pnpm 12
- **樣式**：Tailwind CSS v4
- **動畫**：Framer Motion / GSAP
- **UI**：HeroUI + 自家 `components/ui` primitives

### 代碼規範

- **縮排**：Tab（Biome）
- **引號**：JS/TS 用雙引號（`"`）
- **元件**：預設 Server Component，僅互動時加 `"use client"`；檔名與元件用 PascalCase
- **圖片**：一律用 `next/image`（走 OSS custom loader 自動取得 WebP / srcset）

```typescript
import Image from "next/image";

<Image src="/placeholders/post.svg" alt="Product" width={400} height={300} priority />;
```

### SEO & 結構化資料

SEO 邏輯收斂在 `lib/seo/`：

- `metadata.ts` — 集中產出 page/layout 的 `metadata`
- `jsonld.ts` — JSON-LD 結構化資料（搭配 `components/seo/`）
- `urls.ts` — canonical / absolute URL 組裝
- `indexability.ts` — 控制各環境是否 indexable
- `tags.ts` — faceted / tag 相關

路由層的 `app/sitemap.ts`、`app/robots.ts`、`app/manifest.ts`、`app/opengraph-image.tsx` 提供 sitemap / robots / manifest / 預設 OG。SEO 驗證跑 `pnpm build:seo` + `pnpm test:seo` + `pnpm lhci:*`。

```typescript
import type { Metadata } from "next";
// app/[locale]/(public)/products/[id]/page.tsx
export async function generateMetadata({ params }): Promise<Metadata> {
	// 用 lib/seo/metadata.ts 的 helper 組裝
}
```

## 常見開發場景

### 新增頁面
在對應的 route group 下建目錄與 `page.tsx`（注意目錄邊界規則，跑 `pnpm architecture:check` 確認）。

### 新增 API 路由
```typescript
// app/api/products/route.ts
import { type NextRequest, NextResponse } from "next/server";

export async function GET(request: NextRequest) {
	return NextResponse.json({ message: "Hello" });
}
```

### 新增環境變數
1. 先在 `.env.example` 補上變數名與註解。
2. 把實際值放進 `.env.local` 或部署 secret / variable。
3. 要進瀏覽器才用 `NEXT_PUBLIC_` 前綴；server-only secret 不要加。
4. 多數設定應由 `NEXT_PUBLIC_FLAVOR` 在 `lib/config/env.ts` 推導，新增前先確認是否該走 flavor。

### 修改 Linting 規則
編輯 `biome.json` 後執行 `pnpm lint`（會跑 tsc + biome），或 `biome check . --fix` 自動修。

### 更新依賴
```bash
pnpm outdated
pnpm update
```

## 故障排除

### 開發伺服器啟動失敗
```bash
rm -rf .next
pnpm dev
```

### 型別 / lint 錯誤
```bash
pnpm lint            # tsc --noEmit + biome check
biome check . --fix  # 自動修可修的部分
```

### 格式化不一致
```bash
pnpm format
```

## 資源

- [Next.js 文檔](https://nextjs.org/docs)
- [TypeScript 文檔](https://www.typescriptlang.org/docs/)
- [Biome 文檔](https://biomejs.dev/)
- [Tailwind CSS 文檔](https://tailwindcss.com/docs)

## 貢獻指南

1. 從 `dev` branch 開功能分支（`git checkout -b feature/AmazingFeature`）
2. 提交變更（`git commit -m 'feat: add AmazingFeature'`）
3. Push 分支（`git push origin feature/AmazingFeature`）
4. 對 `dev` 開 Pull Request

## 授權

ISC
