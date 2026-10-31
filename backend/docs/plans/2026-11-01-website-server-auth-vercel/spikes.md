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
