---
description: Test runner, build environment and deletion rules for the website
paths:
  - "website/**"
---

# Website

- 改到的 `tests/*.test.ts` 用 `test:unit` 的 node:test 指令跑（`node --experimental-test-module-mocks --import tsx --import ./tests/setup/hermetic.mjs --test <files>`）；vitest `--project unit` 只收 `*.unit.test.ts` 與 `__tests__`。派 agent 驗收時照這條寫。
- `pnpm run build` 一律帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值的 `NEXT_PUBLIC_API_BASE_URL`：dev 預設 `http://localhost:8080`，prerender 會打到本機正在跑的單體；prod flavor 缺值就失敗。派 agent 跑 build 時把這行寫進 prompt。
- 刪 CSS、頁面或路由時，同一次變更 `rg` 各元件旁 `*.browser.test.tsx` 的 import 與 `tests/e2e/*-routes.spec.ts` 的手寫計數（`toHaveLength`），兩者只在 CI 的 `website e2e` 跑。
- e2e 對 Suspense 串流的路由（`page.goto` 用 `domcontentloaded`）找元素時，locator 限定在 `main` 底下：React 揭示串流內容前，`body > div#S:n` 的隱藏副本與已 hydrate 的頁面並存，未限定的 locator 會偶發 strict mode 兩個元素。
