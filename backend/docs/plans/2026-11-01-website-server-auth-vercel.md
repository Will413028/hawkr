# website 搬到 Vercel，登入狀態改由伺服器端決定

**Goal：** website 不含 Cloudflare Workers 專屬程式，可在 Vercel 建置與執行；`cacheComponents`（PPR）開啟；登入狀態由伺服器端 DAL `verifySession()` 決定並以 Suspense 動態洞串流，client 不再探測（無 `HYDRATION_GATE_MS`、無 `hydrated` 類旗標）。驗收見步驟 5。

**上層：** ADR `backend/docs/adr/2026-11-01-website-hosting-vercel.md`；主清單見本機個人脈絡檔。

**附件：** 無（盤點以下列指令重跑，快照 commit `29d1bb3`）。

## 全域約束與完成定義（暫代主計畫）

- 不部署正式環境；Vercel 預覽部署只在 spike 與收尾，由 Will 建立並連結 Vercel 專案（含登入與授權）。
- 不留雙平台相容層、不留 `if (cloudflare)` 分支；被取代的程式同一步刪除。
- e2e、`test:reliability`、`test:seo`、Storybook 測試不在本機跑，推上去由 CI `website-e2e.yml` 跑。
- build 一律帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值 `NEXT_PUBLIC_API_BASE_URL`。
- 版面、文案、流程不改（四態的骨架是 UI 改動，見 D3）。
- 完成定義：每步 `pnpm lint`、`pnpm typecheck`、`test`（vitest unit）、`test:unit`（node:test）、`pnpm build` 後的 `pnpm test:build-output` 綠；CI website-ci 與 website-e2e 每個 job 綠。

## 前提（開工時核對過，2026-11-01）

- 會改到的既有部署是否在線：不在線。website 從未部署，系統從未上線；不受回滾、相容性窗口約束，步驟回滾就是 revert。
- 平台是 Vercel、function 區域 `hnd1`，單體在 OCI `oci-a1`（ADR 2026-11-01，已確認）。
- 同源反代是 Next route handler（`app/api/connect/[...path]`、`app/api/auth/[...path]`），與平台無關（rg，已確認）。
- Next 16.4.1 的 `cacheComponents` 在 Vercel 上可用，現有頁面改寫量可控（未確認 → spike 步驟 1）。
- Vercel 的 function 經 route handler 反代時，多個 `Set-Cookie` 與 3xx 原樣傳回（未確認 → spike 步驟 1）。
- 訪客 IP 在 Vercel 由 `x-forwarded-for`／`x-real-ip` 提供，由平台改寫、client 無法偽造（Vercel docs: Request headers；未確認 → spike 步驟 1，擋住步驟 2）。`seller-leads` 用它當限流 key，該檔註解（`route.ts:151`）明說不信任 `x-forwarded-for`，所以這條必須實測。
- #338 是公司時期 website 的編號（原 repo 未併入，`website/README.md` 第 171 行有紀錄），對應 commit `6015754f`（「關閉 cacheComponents 全站回傳統純 ISR」）：Cache Components 的 static-shell resume 在 workerd 爆 128MB 記憶體上限，正式環境白畫面。

**驗收基線：** `29d1bb3` 上 website-ci 與 website-e2e 綠（步驟 1 開工時在 CI 重確認，失敗逐項列出、不在本階段修）。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 憑證是 httpOnly `__Host-hk_session`，JS 讀不到 | `lib/auth/session-cookie.server.ts`、`app/api/auth/[...path]`、DAL |
| 單體 `withSession` 是唯一授權執行點；website 的判斷只決定顯示與導向 | `backend/internal/httpapi/rpc/session.go`、`middleware.ts`、`RoleZoneGuard` |
| 伺服器端只讀（D6）：DAL 只呼叫 GetSession（GET），不改 session | DAL、ADR identity D6 |
| 暫時不可用不等於登出：只有確定的 401 轉成訪客 | `lib/auth/session-resolver.ts`（B-199） |
| `'use cache'` 內不讀 cookie | 所有標 `'use cache'` 的元件 |
| 訪客 IP 只有一個讀取點，且讀平台寫入、client 無法偽造的 header | 步驟 2 新增的 `getVisitorIp(request)`；`middleware.ts`、`app/api/public/{seller-leads,tab-bootstrap/v1/head,search/products,search/suggest}`、`app/(delegated)/adm/{enter,api/session/end,api/_forward.ts,api/_upstream.ts}` |
| 登入狀態只有一個來源：伺服器初值，事件後以 `router.refresh()` 重取 | `lib/auth/context.tsx`、`cross-tab-sync.ts`、`teardown.ts`、`useAuthGate.tsx` |

## 範圍

- 做：刪 OpenNext／wrangler／R2／DO 與 Workers 專屬程式；Vercel 設定（`vercel.json` 區域）；CSP、canonical host、訪客 IP、Sentry、快取失效改平台中立或 Vercel；開 `cacheComponents`；DAL；身分改伺服器初值；client 四態。
- 不做：正式部署與網域（第一次部署另行決定）；單體在 `oci-a1` 的部署（ADR 後續）；admin（仍在 Cloudflare Access 後面，不在本計畫）。
- 延後：Apple Pay 網域關聯（觸發：開真實付款或接 Apple Pay）；Vercel Pro（觸發：開真實付款，ADR 撤銷條件）。

## 步驟

- [ ] **1. spike：Vercel 上的 cacheComponents 與反代**（被擋於：Will 建立 Vercel 專案）
  - 範圍：在 scratch 分支開 `cacheComponents`，記錄 build 錯誤種類與數量（需要 `'use cache'` 或 Suspense 的頁數）；預覽部署後確認 `/api/auth/*` 的 3xx 與多個 `Set-Cookie`、Connect 串流、訪客 IP header；量 `hnd1`→`oci-a1` 的 RTT（單體尚未部署時以 `oci-a1` 上的 echo 端點代替）；確認 `revalidateTag` 在 Vercel 上讓 tag 失效；確認 function 的記憶體上限數字與讀數位置（Vercel Observability）。產出寫進 `backend/docs/plans/2026-11-01-website-server-auth-vercel/spikes.md` 並提交到 main；spike 程式在 scratch 分支，不 merge。
  - 消費端：無（spike）。
  - 驗收：spikes.md 每項附指令與原始輸出；IP 項：請求自帶偽造的 `x-forwarded-for`，記錄伺服器端讀到的值與真實 IP 是否一致。
  - 停止條件（加）：偽造的 IP 能被伺服器讀到 → 回報，步驟 2 的 IP 讀取點重新設計。
  - 停止條件：`cacheComponents` 需要改寫超過約一半頁面，或反代丟失 `Set-Cookie` → 回報，重拍 D1／D2。
  - 需要人做的事：建立 Vercel 專案、授權 GitHub 連結、觸發預覽部署（部署需授權）。
- [ ] **2. 平台遷移，行為不變**（被擋於：步驟 1，且其中「訪客 IP 無法偽造」已實測成立）
  - 範圍：刪 `open-next.config.ts`、`wrangler.jsonc`、`public/_headers`（併入 `next.config.ts` headers）、`@opennextjs/cloudflare` 依賴與 `preview`、`cf-typegen` script、`lint` script 裡寫死的 `open-next.config.ts` 路徑、`tests/open-next-*`；根目錄 `pnpm-workspace.yaml` 的 patchedDependencies 條目與 `patches/@opennextjs__cloudflare.patch`、`osv-scanner.toml` 對應的 IgnoredVulns 條目（供應鏈基準，刪除寫進 commit 訊息）。訪客 IP：新增唯一讀取點 `getVisitorIp(request)`，不變量表列的 9 個呼叫點與直接塞 `cf-connecting-ip` 的 seller-leads、search 測試全部改用它。`lib/seo/canonical-host.ts` 與 middleware 的 workers.dev 規則改成 Vercel 預覽網域（`*.vercel.app` 一律 noindex）。快取：刪 OpenNext 的 R2 incremental cache、DO queue、tag cache 綁定；`app/api/revalidate` 已用 `next/cache` 的 `revalidateTag`，不動。CSP、Sentry trace tag 去掉 Cloudflare 專屬項；新增 `vercel.json`（`regions: ["hnd1"]`）。
  - 消費端：`rg -l -i "opennext|open-next|wrangler|cloudflare|workers\.dev|cf-connecting|getCloudflareContext|DurableObject" --glob '!*.md' --glob '!node_modules' .`（整個 repo），每檔標去向；已知無關：`packages/hawkr-ui-text-ts/tools/check-enum-text.ts` 的 `.wrangler` 排除項、`backend/workers/image-gateway`（另一支 Worker，不在範圍）、`lib/media/cdn.ts` 的 OSS loader。另：`.github/workflows/website-*.yml`；根 `AGENTS.md` 地圖、`website/AGENTS.md`、`website/README.md`。
  - 不能動：渲染模型（仍純 ISR，`cacheComponents` 關）；路由與回應內容。
  - 驗收：完成定義的檢查；`pnpm install --frozen-lockfile` 成功；rg 清單只剩上面註明的無關命中；`rg -n "cf-connecting-ip" website` 無命中；mutation：讓 `getVisitorIp` 改讀 `cf-connecting-ip`，seller-leads 限流測試要失敗；預覽網域 noindex 測試在移除規則時失敗。
  - 停止條件：某個 Cloudflare 機制在 Vercel 沒有對應而且影響行為 → 回報。
  - 完成後的狀態：Apple Pay 關聯檔路由保留（與平台無關）；本機 `pnpm preview` 消失，改 `pnpm build && pnpm start`。
- [ ] **3. 開 cacheComponents ＋ DAL，身分由伺服器初值決定**（被擋於：步驟 2；單體公開可達後補完步驟 1 的反代、RTT、tag 失效三項）
  - 範圍：`cacheComponents: true`；`lib/api/cached.ts` 的 `unstable_cache` 遷到 `'use cache'`＋`cacheTag`＋`cacheLife`（Next 16 的基準；`unstable_cache` 在 Cache Components 下是舊 API）。它的 `singleFlight`（同 key 併發只打一次後端）先回答「從零設計會不會需要」：`'use cache'` 本身會併發去重時刪除，否則保留並在 commit 寫理由；新增 `lib/auth/dal.ts` 的 `verifySession()`（`react` `cache()` 包，讀 cookie → GetSession，回 `signedIn | guest | unavailable`）；根 layout 把 session promise 包在 Suspense 交給 `AuthProvider`，client 以 `use()` 讀；刪除 client 的初次探測與 `HYDRATION_GATE_MS`；受保護區 layout 以 DAL 導向，`RoleZoneGuard` 改伺服器端；刪 `website/README.md` 的「不要加回 cacheComponents」與零 PARTIALLY_STATIC gate（`rg -ln "PARTIALLY_STATIC" website/tests website/open-next.config.ts website/next.config.ts` 列出的檔，例如 `tests/seller-preview-ssr-production.test.ts`），改成 `tests/build-output/` 下的新測試：讀 build 產出的 prerender manifest，斷言受保護頁與含頁首的頁是 PARTIALLY_STATIC。
  - 消費端：`rg -ln "useAuth\(|hydrated|sessionSettled|sessionUnavailable|unstable_cache|cachedFetch" website --glob '*.ts*'`。
  - 不能動：不變量表全部；登入與登出流程的 URL 與 cookie。
  - 驗收：完成定義；mutation：DAL 對 401 回 `unavailable` 時，「401 轉訪客」測試要失敗；把 GetSession 移進 `'use cache'` 時 build 要失敗（Next 會擋讀 cookie）；把 gate 加回時「首屏無檢查中」測試要失敗。#338 回歸（預覽部署，Will 觸發）：首頁、PDP、`/account` 各以 `curl` 請求 20 次，全部 200 且 HTML 含頁首的固定標記（`data-testid` 由步驟 3 指定）；Vercel Observability 中這些 function 的最大記憶體低於步驟 1 記下的上限的 50%。
  - 停止條件：某頁讀 cookie 又必須整頁快取，無法拆成動態洞 → 回報。
  - 需要人做的事：觸發預覽部署。
- [ ] **4. client 四態**（被擋於：步驟 3、D3）
  - 範圍：`AuthProvider` 的狀態改 `unknown | guest | signedIn | unavailable`（`unknown` 只存在於 Suspense fallback）；刪 `hydrated`／`sessionSettled`／`sessionUnavailable` 旗標，消費端改讀狀態。狀態轉移（D4）：

    | 事件 | 轉到 | 機制 |
    |---|---|---|
    | 登入完成（OAuth callback 回站） | `signedIn` | 整頁導向，伺服器重算初值 |
    | 本分頁登出 | `guest` | SignOut 後 `teardown` 清 client 快取，`router.refresh()` |
    | 其他分頁登出或登入 | 依伺服器 | `cross-tab-sync` 收到訊號後 `router.refresh()` |
    | 任一呼叫回 401 | `guest` | 同上，`router.refresh()` |
    | 網路錯誤或 5xx | `unavailable` | 不登出，保留身分，可重試 |
  - 消費端：同步驟 3 的 rg，加 `lib/auth/{cross-tab-sync,teardown,useAuthGate}.ts*`。
  - 不能動：B-199 語意。
  - 驗收：完成定義；`rg -n "hydrated|sessionSettled|sessionUnavailable|HYDRATION_GATE_MS" website` 無命中；mutation：把網路錯誤映射成 `guest`，「暫時不可用不登出」測試要失敗；拿掉登出後的 `router.refresh()`，「登出後頁首不顯示頭像」測試要失敗；e2e 待 CI。
- [ ] **5. 收尾**：完成定義逐項核對；design-review；步驟 3 的 build-output 測試已接 CI；延後項附觸發條件搬回主清單；更新根 `AGENTS.md` 地圖（website 改 Vercel）、`website/AGENTS.md`、`website/README.md`、identity ADR 的 web 段落。需要人做的事：預覽部署驗證、第一次正式部署的決定。驗收：上述 rg 與 CI 連結。

## 決定

- **D1 頁首與帳號選單的身分怎麼到 client**（狀態：已決 2026-11-01，Will，選 A；擋住步驟 3；現在可拍，步驟 1 的事實若推翻則重交）
  - A：根 layout 不 await，把 `verifySession()` 的 promise 傳給 client Provider，Provider 用 `use()` 包在 Suspense 裡——shell 仍可靜態，身分在同一個 response 串流（Next.js docs: Authentication、`use`）。（建議，採用）
  - B：每個需要身分的 Server Component 各自呼叫 DAL、以 props 傳——client 元件很多時改動大，且仍要 client 狀態供互動元件。
- **D2 受保護區（`/account`、`/seller-center` 等）的角色檢查**（狀態：已決 2026-11-01，Will，選 A；擋住步驟 3）
  - A：layout 呼叫 DAL，不符 `redirect()`；middleware 只保留「有沒有 cookie」的樂觀導向；刪 `RoleZoneGuard`（Next 官方 DAL 模式）。（建議，採用）
  - B：保留 client `RoleZoneGuard`——角色在 client 判斷，與本計畫目標相反。
- **D3 `unknown`（Suspense fallback）時頁首顯示什麼**（狀態：已決 2026-11-01，Will，選 A；擋住步驟 4；UI 改動需同意）
  - A：固定尺寸的中性骨架（不顯示登入鈕、也不顯示頭像），避免位移與閃爍。（建議，採用）
  - B：先顯示訪客樣式——登入者會看到「登入」閃一下再變頭像。

- **D4 事件之後 client 的登入狀態怎麼更新**（狀態：已決 2026-11-01，Will，選 A；擋住步驟 4）
  - A：伺服器是唯一來源，登出、跨分頁訊號、401 之後 `router.refresh()` 重取初值；client 只在 refresh 完成前暫時顯示目標態（步驟 4 的轉移表）。（建議，採用）
  - B：client 自己維護狀態並直接改——快，但伺服器初值與 client 狀態成為兩個來源，會再分歧。

拍板順序：D1–D4 現在可一起拍；若步驟 1 的事實推翻前提，重交 D1／D2。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 spike | 進行中 | 上傳方式、純 ISR 建置、cacheComponents 改寫量（34／405 頁）、IP header 防偽、區域與記憶體設定（spikes.md） | 反代 3xx／Set-Cookie、RTT、tag 失效：單體尚未公開部署 | |
| 2 平台遷移 | 未開始 | | | |
| 3 cacheComponents ＋ DAL | 未開始 | | | |
| 4 client 四態 | 未開始 | | | |
| 5 收尾 | 未開始 | | | |
