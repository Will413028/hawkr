# Upstream patch watch

Tracks the vendored patches under `patches/` (registered in the root `pnpm-workspace.yaml`) whose only reason to exist is an
unresolved upstream defect. Each entry lists the redemption condition (when the
patch can be dropped) and how to check it. These patches are correct workarounds,
not tech debt — the debt would be forgetting to remove them once upstream ships.

There is no renovate/dependabot config in this repo, so this file is reviewed
manually. Re-check the relevant entry whenever a patched dependency is bumped
(and at least once per upstream release cycle).

**所有退場檢查都必須能在本機重現**：公司時期的 dev／prod 部署與 Sentry 專案已不在
範圍，不作為檢查依據。共用步驟（每條「Redemption action」都從這裡開始）：

1. 改 `pnpm-workspace.yaml` 的 `patchedDependencies` 拿掉該項、刪 patch 檔，
   在根目錄 `pnpm install`（lockfile 的 `patch_hash` 會隨之更新）。
2. 跑該條的本機指令。
3. `pnpm run check:deps`（根目錄）。

本機的 workerd 環境：`pnpm --filter website preview`、`pnpm --filter admin preview`
（`opennextjs-cloudflare build && opennextjs-cloudflare preview`，由 wrangler 在本機起 workerd）。
「未確認」表示無法從 patch 內容或上游資料推得，條目寫明缺什麼。

---

## `@vis.gl__react-maplibre@8.1.2.patch` — async container lifecycle guard

- **Files patched**: `src/components/map.tsx`, `dist/components/map.js`, and
  `dist/index.cjs`.
- **What it does**: snapshots and validates the map container after the
  asynchronous `mapLib` resolution, before calling `Maplibre.reuse` or the
  constructor. During a rapid React 19 unmount, the DOM ref can already be
  cleared while the passive-effect cleanup has not yet flipped `isMounted`;
  passing that null ref to MapLibre otherwise logs `Invalid type: 'container'
  must be a String or HTMLElement.`
- **Upstream**: `visgl/react-map-gl` issue **#1960** tracks this exact
  `Invalid type: 'container'` async ref-cleanup race and remains open. Issue
  **#2584** is a related React 19 / Next.js 16 navigation lifecycle race.
  Version `8.1.2` does not guard the container after its async boundary.
- **Installed version**: `@vis.gl/react-maplibre 8.1.2`, pulled transitively by
  `react-map-gl 8.1.2`.
- **Redemption condition**: a released `react-map-gl` version snapshots or
  validates `containerRef.current` after async module resolution, before both
  reuse and construction.
- **How to check**: 讀目標版本的 `src/components/map.tsx`（`rg -n 'containerRef.current' node_modules/@vis.gl/react-maplibre/src/components/map.tsx`），
  確認 `Promise.resolve(mapLib ...).then(...)` 內，在 `Maplibre.reuse` 與 `new Maplibre` 之前已檢查
  `containerRef.current`。`website/tests/price-topography-contract.test.ts` 會斷言 patch 檔與
  `patchedDependencies` 的登記，拿掉 patch 時要同步改它。
  行為面：未確認。`tests/e2e/insights-optional-market.spec.ts` 沒有收集 `console.error`／`pageerror`，
  也沒有快速切換 map 與 overview 的情境；缺的是這個 E2E。退場前先補一個
  （切換後斷言沒有 `Invalid type: 'container'` 的 console error），並在有 patch 與無 patch
  兩種狀態下各跑一次，確認它能抓到問題。
- **Redemption action**: upgrade `react-map-gl` → remove this patch and its
  `patchedDependencies` entry → install with a frozen lockfile (workspace root) → run the
  structural, browser, typecheck, and production-build gates.

---

## `@opennextjs__cloudflare.patch` — hunk A: queue.js treat-as-success

- **File patched**: `dist/api/durable-objects/queue.js`
- **What it does**: a revalidation response of `HTTP 200` with
  `x-nextjs-cache !== "REVALIDATED"` was hard-`throw new FatalError`. The patch
  logs via `debug()` and treats it as success, so normal stale-while-revalidate
  races no longer crash the durable-object queue (which would break dedup
  bookkeeping and re-hammer the backend).
- **Upstream**: opennextjs/cloudflare PR/issue **#1272**
  ("fix(queue): treat 200 + non-REVALIDATED response as success") — **OPEN**,
  stalled on a design discussion (collaborator questions whether STALE/HIT
  should count as success). No ETA, not in any release.
- **Installed version**: `@opennextjs/cloudflare 1.20.6` (bumped from 1.20.2 on
  2026-09-04 together with next 16.3.4). Neither 1.20.3 (2026-08-26, proxy.ts +
  Turbopack wasm patching for 16.3) nor 1.20.6 (2026-09-02, next 16.3.4 bump)
  contains this fix; #1272 was still OPEN at bump time — bumping does not
  remove the patch. Re-verified after the bump: the hunk applied cleanly and a
  local preview showed STALE → HIT within 2s with no FatalError in the log.
- **Redemption condition**: #1272 (or an equivalent fix) merged **and** shipped
  in a release.
- **How to check**: 讀 PR #1272 狀態與目標版本的 changelog；再讀安裝後的
  `node_modules/@opennextjs/cloudflare/dist/api/durable-objects/queue.js`，
  `rg -n 'REVALIDATED|FatalError' <該檔>` 確認 status 200 且 `x-nextjs-cache` 非 `REVALIDATED`
  不再 `throw new FatalError`。
  本機行為驗證（2026-09-04 已做過一次）：`pnpm --filter website preview`，對一個 ISR 路由在
  過期後連續請求兩次，確認 STALE → HIT 且 wrangler 的輸出沒有 `FatalError`。
- **Redemption action**: `pnpm up @opennextjs/cloudflare@<version with fix>` →
  remove this hunk from the patch → `pnpm --filter website preview` 並重做上面的
  STALE → HIT 檢查，確認沒有 `FatalError`、頁面不白屏。
- **Do not fork** OpenNext for this; maintaining a fork costs far more than a
  small vendored hunk.

## `@opennextjs__cloudflare.patch` — hunk B: flight-readable-stream-type

- **Files patched**: `dist/cli/build/open-next/createServerBundle.js` (wires the
  plugin in) + `dist/cli/build/patches/plugins/flight-readable-stream-type.js`
  (new build-time plugin).
- **What it does**: strips `type: "bytes"` from the `ReadableStream` constructor
  in `createInlinedDataReadableStream`. workerd lacks sufficient support for the
  `bytes` stream type, which otherwise breaks RSC Flight (hydration) streams.
- **Upstream**: no tracked PR yet; blocked on either workerd supporting
  `ReadableStream({ type: "bytes" })` or OpenNext shipping an equivalent
  built-in patch.
- **Redemption condition**: workerd supports the `bytes` stream type, or a
  released `@opennextjs/cloudflare` already contains this transform.
- **How to check**: bump 之後在 `website/` 跑 `pnpm exec opennextjs-cloudflare build`，
  `rg -c 'new ReadableStream\(\{type:"bytes",' .open-next/server-functions/default/handler.mjs`
  在 patch 存在時應為 0（被 plugin 改掉）；拿掉 patch 後若為 0，代表上游或 workerd 已處理，
  hunk 可退場。plugin 的 content-filter 不再比對到時會靜默不作用，所以要以這個輸出判斷，
  不要假設。再 `pnpm --filter website preview`，用 `curl -s localhost:8787/` 確認頁面
  的 inline Flight script 正常、瀏覽器載入後沒有 hydration 錯誤。
  未確認：不套這個 hunk 時 workerd 在本機的具體錯誤訊息（patch 與上游都只寫「RSC Flight
  stream 相關錯誤」），缺的是一次拿掉 hunk 的本機重現。
- **Redemption action**: 同 hunk A 的 bump 流程；上面兩個本機檢查都過才拿掉這個 hunk。

## `@opennextjs__cloudflare.patch` — hunk C: r2-incremental-cache write retry and single-flight

- **File patched**: `dist/api/overrides/incremental-cache/r2-incremental-cache.js`
- **What it does**（由 patch 內容推得）: R2 寫入遇到錯誤碼 10043（服務暫時不可用）或 10058
  （同一個 key 的寫入頻率限制）時，最多重試三次、指數退避加 jitter；同一個 key 的併發寫入
  合併成一個（single-flight），內容不同時等前一個結束再寫。
- **Upstream**: 未確認。patch 沒有附上游 issue／PR，缺的是搜尋 opennextjs/cloudflare 對
  `r2-incremental-cache` 寫入重試的 issue；找到後補在這裡。
- **Redemption condition**: 目標版本的 `r2-incremental-cache.js` 對 10043／10058 重試，
  且對同一 key 的併發寫入去重。
- **How to check**: `rg -n '10043|10058|singleFlight|inFlight' node_modules/@opennextjs/cloudflare/dist/api/overrides/incremental-cache/r2-incremental-cache.js`
  （拿掉 patch 後的安裝內容）。本機沒有可以觸發 R2 速率限制的環境，
  所以行為面的確認：未確認，缺的是不連真實 R2 時產生 10043／10058 的替身；以讀程式碼為準。
- **Redemption action**: bump 後確認上面的 `rg` 有輸出，拿掉這個 hunk，`pnpm --filter website preview`
  確認 ISR 寫入快取仍成功。

---

## `@editorjs__editorjs.patch` — caret position when the selection lands on an element

- **Files patched**: `dist/editorjs.mjs`、`dist/editorjs.umd.js`（兩者是同一個函式的不同打包；
  umd 的 hunk 是同一個修正）。
- **What it does**（由 patch 內容推得）: 取目前選取的 focus 節點與 offset 的函式，原本只往下走一層
  子節點；若 `childNodes[offset]` 不存在，取最後一個子節點並把 offset 當成它的文字長度，
  但那個子節點是 element 時 `textContent.length` 不是子節點數。patch 改成逐層往下走，
  遇到 element 就用 `childNodes.length` 當 offset 繼續。
- **Upstream**: 未確認。patch 沒附 issue；缺的是 codex-team/editor.js 對這個函式的 issue／PR
  與對應版本。
- **Installed version**: `@editorjs/editorjs 2.31.6`（website）。
- **Redemption condition**: 目標版本的同一個函式（`rg -n 'focusNode' node_modules/@editorjs/editorjs/dist/editorjs.mjs`
  找 `getSelection()` 附近，回傳 `[node, offset]` 的那個）逐層下探 element 子節點。
- **How to check**: 讀上述函式；`pnpm --filter website test`（vitest）與
  `pnpm --filter website test:unit` 的 `description-editorjs-*` 測試只涵蓋編輯器的資料與樣式，
  不涵蓋游標，所以行為面：未確認，缺的是一個重現游標落在 element 末端
  （例如行內元素之後）的 E2E；退場前先補，或在 `pnpm --filter website dev` 的賣家商品描述編輯器
  手動重現。
- **Redemption action**: `pnpm up @editorjs/editorjs@<version>` → 拿掉 patch 與
  `patchedDependencies` 項 → 共用步驟 → 上面的檢查。

---

---

## `next.patch` — CacheSignal cross-request timer cleanup on workerd

- **Files patched**: `dist(+esm)/server/app-render/cache-signal.js` and
  `dist(+esm)/server/node-environment-extensions/fast-set-immediate.external.js`.
- **What it does**: wraps the CacheSignal timer cleanup (`clearImmediate` /
  `clearTimeout`) in try/catch. The module-loading CacheSignal is an
  isolate-wide singleton; on workerd a timer scheduled by one request cannot be
  cleared from another (`Cannot perform I/O on behalf of a different request`,
  IoContext), which surfaced as Cloudflare 1101 on random SSR routes and as
  "點了連結沒反應" (Sentry DEV-S). The fast-set-immediate hunk is the one that
  matters in production: the pre-bundled `next-server/*.runtime.prod.js` copies
  carry their own CacheSignal, and only the global timer shim reaches them.
- **Upstream**: not filed against vercel/next.js as of 2026-09-04; no known fix.
- **Installed version**: `next 16.3.6`（根 lockfile；website 與 admin 同一個 next，patch 作用到兩者）。
  patch 原本對 16.2.10 切出，之後每次 bump 都能乾淨套用。
- **Redemption condition**: 一個 Next 版本的 `scheduleImmediateAndTimeoutWithCleanup`
  （或 OpenNext 的 timer shim）容許跨 request 的清理，且依本機檢查在 workerd 不再出現
  `Cannot perform I/O on behalf of a different request`。
- **How to check**（本機，website 與 admin 各做一次）:
  1. 靜態：升級後拿掉 patch，讀 `node_modules/next/dist/server/app-render/cache-signal.js`
     的 `scheduleImmediateAndTimeoutWithCleanup`，確認回傳的清理函式已包 try/catch 或不會跨 request 丟錯。
  2. 套用 patch 時：`pnpm --filter <app> preview` 先 build，
     `grep -c patchedClearTimeout .open-next/server-functions/default/handler.mjs` 必須 ≥ 1。
  3. 退場時：拿掉 patch，`pnpm --filter <app> preview`，對數個 SSR 路由同時發送請求
     （例如 `seq 60 | xargs -P 20 -I{} curl -s -o /dev/null localhost:8787/<SSR 路由>`），
     wrangler 輸出不得出現 `Cannot perform I/O on behalf of a different request`，回應不得有 1101。
  admin 的確認方式相同，把 `<app>` 換成 `admin`、路由換成 admin 的 SSR 頁面；admin 是否曾經出現過
  這個問題：未確認（現有紀錄只有 website 的 Sentry DEV-S，已不可查），所以 admin 的退場以步驟 1 與 3 為準。
  未確認：步驟 3 在拿掉 patch 時能不能穩定重現（原始證據是線上 Sentry 事件，沒有本機重現紀錄）；
  缺的是一次拿掉 patch 的本機負載測試。若不能重現，退場依據只剩步驟 1 的原始碼檢查。
- **Redemption action**: 升級 next → 共用步驟（`patchedDependencies` 的 `next` 與檔案）→ 上面的 1、3。
