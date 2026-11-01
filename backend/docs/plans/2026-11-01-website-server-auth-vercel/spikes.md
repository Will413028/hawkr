# 步驟 1 spike 紀錄（2026-11-01）

Vercel 專案 `hawkr`（team `will413028s-projects`，Hobby）：以 API 建立，`rootDirectory: website`、`framework: nextjs`、function 區域 `hnd1`；**沒有連 GitHub**（連上即 push main 自動部署 production）。部署用 CLI 送預覽：

```
vercel deploy --yes --archive=tgz -b NEXT_PUBLIC_FLAVOR=prod -b NEXT_PUBLIC_WEB_ORIGIN=… -b NEXT_PUBLIC_ADM_ORIGIN=… \
  -b API_BASE_URL=http://127.0.0.1:9/api/v1 -b NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:9/api/v1 -e API_BASE_URL=http://127.0.0.1:9/api/v1
```

## a. 上傳方式（量測）

- 不帶 `--archive=tgz` 逐檔上傳，第一次就撞 Hobby 上限：`Too many requests - try again in 1 day (more than 5000, code: "api-upload-free")`。之後一律 `--archive=tgz`。
- 根目錄要有 `.vercelignore`，而且規則要以 `/` 錨定（`/backend`、`/api`、`/brand`…）：未錨定的 `api`、`brand` 會連 `website/lib/api`、`website/lib/brand` 一起排除，build 出現 150 個 `Module not found`。步驟 2 把這份檔納入版控。
- build log 有 `Error while parsing config file: …/pnpm-lock.yaml`（CLI 讀 lockfile 的警告），build 本身照常用 pnpm 安裝成功。

## b. 現況（純 ISR）在 Vercel 能否建置與服務（量測）

- `main`（`29d1bb3` 之後）原樣部署成功，未改程式。預覽有 Vercel Authentication 保護，直接 `curl` 回 302 到 `vercel.com/sso-api`；用 `vercel curl <path> --deployment <url>` 取得 bypass token：`/` 回 `200`，0.78s。

## c. 開 `cacheComponents` 的改寫量（量測，本機 `next build`）

逐層剝開的結果（spike 分支 `spike/vercel-cc`，未 merge）：

1. 42 個 route segment config 不相容：`dynamic` 30、`revalidate` 12（共 41 檔）。機械式刪除即可，語意改由 `'use cache'`／`cacheLife` 表達。
2. `generateStaticParams` 必須回至少一筆：3 處回 `[]`（`lib/api/static-params.ts` 的 `sellerStaticParams`、`products/[id]`、`sellers/[id]/products/[pid]`）。既有程式已有 sentinel 寫法（`orders/[orderNo]`、`measures/[measureId]` 回 `{…: "0"}`）。
3. `next build --debug-prerender`：405 頁中 34 頁 prerender 失敗，涵蓋 23 個路由：
   - 12 個「URL data（`usePathname()` 等）在 Client Component 但不在 `<Suspense>` 內」：`/adm/[storeId]/*` 11 頁與 `/adm/unavailable`。
   - 11 個「uncached or runtime data」：`/[locale]`（首頁）、`cart`、`feed`、`login`、`market/products`、`search`、`products/[id]`（含攔截路由 `(.)products/[id]`）、`orders/[orderNo]/refund`、`sellers/[id]/card`、`hawkr-agent-rfq-demo`。
   - 原始 log：spike 時的 scratchpad `build-cc4.log`（未保存）；重現：在 spike 分支跑 `next build --debug-prerender`。
- 結論：約 8% 的頁需要加 Suspense 或 `'use cache'`，未達停止條件（約一半）。

## d. 訪客 IP header（量測）

在預覽部署加一個回顯 route（`spike/vercel-ip`），帶偽造 header 請求：

```
無偽造：{"xff":"211.12.255.194","xri":"211.12.255.194","vff":"211.12.255.194","cf":null,"region":"hnd1"}
帶 X-Forwarded-For: 6.6.6.6、X-Real-IP: 7.7.7.7、CF-Connecting-IP: 8.8.8.8：
       {"xff":"211.12.255.194","xri":"211.12.255.194","vff":"211.12.255.194","cf":"8.8.8.8","region":"hnd1"}
本機出口 IP（api.ipify.org）：211.12.255.194
```

- Vercel 覆寫 `x-forwarded-for`、`x-real-ip`，client 偽造不進來：前提成立，步驟 2 解除阻擋。讀取點用 `x-real-ip`。
- **`cf-connecting-ip` 在 Vercel 上原樣透傳 client 送的值**：留著任何讀它的程式就是可偽造的 IP（限流可被繞過）。步驟 2 必須清掉全部讀取點。
- function 確實跑在 `hnd1`（`VERCEL_REGION`）。

## e. function 記憶體上限（量測＋推論）

- 專案設定（API 讀回）：`fluid: true`、`functionDefaultMemoryType: standard`、`functionDefaultTimeout: 300`。
- `standard` 對應 2 GB／1 vCPU 是依 Vercel 文件（Fluid compute 的記憶體類型），推論，未在 function 內量測。步驟 3 的回歸門檻（50%）即 1 GB。

## 未做（擋於單體部署到 `oci-a1`）

- 反代 `/api/auth/*` 的 3xx 與多個 `Set-Cookie`、Connect 串流、`hnd1`→`oci-a1` 的 RTT、`revalidateTag` 的 tag 失效：都需要公開可達的單體。步驟 3 開工前補做；步驟 2（行為不變的平台遷移）不依賴它們。

## f. 單體公開後的反代與登入（2026-11-01，量測）

部署：`oci-a1` 上 `~/hawkr-spike`（單體 `serve`＋Postgres 18 容器，1 CPU／1.5GiB 與 0.5 CPU／512MiB；舊服務位址填佔位值 `127.0.0.1:9`），經 Tailscale Funnel `https://oci-a1.tail357fef.ts.net:10000` 對外；website 預覽固定別名 `https://hawkr-spike.vercel.app`（OAuth dev client 已登記該 callback）。

- **計畫前提錯誤**：`app/api/{connect,auth,v1}/[...path]` 的 route handler 只在本機開發啟用（`isLocalApiProxyEnabled`），部署環境回 404；原本靠 Cloudflare 的路由把 `/api/*` 轉給單體。Vercel 上要用 `next.config.ts` 的 `rewrites().beforeFiles` 把 `/api/connect/:path*`（到單體根目錄）、`/api/auth/:path*`、`/api/v1/:path*` 轉到單體。spike 分支 `spike/vercel-proxy` 以 `SPIKE_API_ORIGIN` 驗證。
- 經 Vercel rewrite：`GetSession` 無 cookie 回單體的 `401 unauthenticated`；`/api/auth/google/start` 回 `302` 到 Google 並帶 `Set-Cookie: __Host-hk_login=…; HttpOnly; Secure; SameSite=Lax`。真人 Google 登入：單體記 `identity：登入建立帳號 account_id=1`，瀏覽器帶 cookie 呼叫 GetSession 回 `200 {"account":{"id":"1"}}`。重導向、多個 Set-Cookie 經 rewrite 原樣傳遞，成立。
- **第二個設定錯誤**：`NEXT_PUBLIC_API_BASE_URL` 在 build 時寫死；設成跨來源的值（spike 第一次是 `http://127.0.0.1:9/api/v1`）時，瀏覽器的 Connect 直接打那個 host（`lib/connect/transport.ts` `connectBaseUrl`），登入後頁面仍顯示未登入。改成同源 `https://hawkr-spike.vercel.app/api/v1` 後瀏覽器打 `/api/connect/…`，頁首正常顯示登入狀態。部署環境的值必須同源，prod 設定應拒絕跨來源值。
- RTT（Vercel `hnd1` function → Funnel → `oci-a1`，10 次 `/healthz`）：`[1540,347,235,253,194,185,174,187,158,136]` ms。Funnel 經 Tailscale 中繼，只能當上限；正式連線方式定案後重量。
- LINE 真人登入、`revalidateTag` 失效：未測。
