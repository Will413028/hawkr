---
description: Test runner, build environment and deletion rules for the website
paths:
  - "website/**"
---

# Website

- 改到的 `tests/*.test.ts` 用 `test:unit` 的 node:test 指令跑（`node --experimental-test-module-mocks --import tsx --import ./tests/setup/hermetic.mjs --test <files>`）；vitest `--project unit` 只收 `*.unit.test.ts` 與 `__tests__`。派 agent 驗收時照這條寫。
- `pnpm run build` 一律帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值的 `NEXT_PUBLIC_API_BASE_URL`：dev 預設 `http://localhost:8080`，prerender 會打到本機正在跑的單體；prod flavor 缺值就失敗。派 agent 跑 build 時把這行寫進 prompt。
- 刪 CSS、頁面或路由時，同一次變更 `rg` 各元件旁 `*.browser.test.tsx` 的 import 與 `tests/e2e/*-routes.spec.ts` 的手寫計數（`toHaveLength`），兩者只在 CI 的 `website e2e` 跑。
- e2e 中用 `domcontentloaded`（含 reload）的導航一律走 `gotoStreamed`／`reloadStreamed`（`tests/e2e/helpers/streamed-navigation.ts`），由 `check-e2e-streamed-goto` 守門；預設 `load` 的導航不在守門範圍。原因：頁面含串流的 Suspense 邊界，`domcontentloaded` 時 `body > div[hidden][id^="S:"]` 的隱藏副本仍與已 hydrate 的頁面並存，locator 會偶發 strict mode 兩個元素；必須在串流中斷言的 spec 用 `// streamed-goto-allow: <原因>` 註明。
- e2e 裡 spec 自己的 Connect mock 一律在 `setupGatewayMocks` 之後註冊：它的 `mockSocialDefaults` 等預設回應是後註冊的話，Playwright 讓後註冊的 route 優先，會蓋掉 spec 的 mock（頁面變空、斷言找不到元素）。
- 伺服器端（server component、`"use cache"` 包裝、route handler）建立 Connect transport 一律明確傳路由的 locale（沒有路由語系就用 `defaultLocale`）：`activeLocale()` 只在瀏覽器可用，server 端缺 locale 時讀取在送出前就丟錯，被 `prerenderSafe` 吞成空資料，本機 build 與單元測試都是綠的，只有 build log 的 `[cache-fill] … failed` 看得出來。
