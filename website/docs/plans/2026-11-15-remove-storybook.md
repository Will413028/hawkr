# 拆除 website Storybook

**Goal：** website 不再依賴 Storybook：原本 31 個 story 檔的 139 個 play 都改寫成 Vitest browser mode 的 `*.browser.test.tsx` 並在 CI 執行，a11y 由 Playwright e2e 的 axe 掃描守，Storybook 的依賴、patch、設定、scripts 與 CI job 全部刪除（驗收見步驟 7）。

**上層：** 主清單在 repo 外，見本機個人脈絡檔。

**附件：** `website/docs/plans/2026-11-15-remove-storybook/`（`inventory.md` 盤點與去向、`plays.txt` play 計數）；快照 commit `8494f0fa`。

## 全域約束與完成定義（暫代主計畫）

- 不在本機跑任何瀏覽器測試（Vitest browser mode、Playwright、`storybook:test`）：push 到分支開 PR，由 CI 跑；CI 通過前一律回報「待 CI」。
- agent 不自行新增或升級依賴：先列套件名稱與版本、核對發佈滿兩週，經 Will 同意才改。
- 改寫是等價搬移：每個 play 的斷言都要在新測試裡有對應（可以合併，不可以刪弱）；沒有斷言的展示不搬。
- 完成定義：盤點表每列的去向都已執行；`rg -i storybook` 只剩盤點表標「保留」的歷史紀錄與本計畫；CI 的 website 檢查（website-ci、website-e2e 每個 job）綠。

## 前提（開工時核對過，2026-11-15）

- 會改到或取代的既有部署是否在線：不在線。Storybook 只在本機與 CI 跑，沒有部署（`rg -n storybook .github/workflows` 只有 `website-e2e.yml` 的測試 job）；不受回滾與相容性窗口約束。
- 拆除 Storybook、a11y 改用 axe 進 Playwright e2e、`stories/apps` 與 `CrossPlatform` 先確認再刪（來源：Will，2026-11-15；已確認）。
- `@vitest/browser-playwright` 5.0.5、`@testing-library/react` ^16.3.3、`@testing-library/dom` ^10.4.1、`msw` ^2.14.6 已在 `website/package.json`，改寫不需新依賴（來源：`website/package.json`，2026-11-15；已確認）。被測元件在 Storybook 下靠 `@storybook/nextjs-vite` 模擬 Next.js 模組，browser mode 不需新依賴就能替代（未確認，步驟 1 的 spike 驗證）。
- `design:check` 不依賴 stories：它只讀 `website/design-system/*.json` 的 `sources`（`styles/foundation/*.css`、`styles/components/ui-kit.css`、兩支 App 的 `lib/core/theme/*.dart`）（來源：`website/scripts/check-cross-platform-tokens.mjs:153-180`，2026-11-15；已確認，步驟 5 再以執行結果證明）。
- `storybook:apps`（`website/scripts/build-app-catalogs.mjs`）只把 Flutter 元件目錄建成 web 版放進 `output/flutter` 給 Storybook 當靜態目錄，沒有 workflow 呼叫它（`rg -n 'storybook:apps\|build-app-catalogs' .github` 無結果）；Flutter 目錄本身（`buyer-app/lib/catalog/`、`seller-app/lib/main_catalog.dart`）可用 `flutter run` 獨立執行（來源：`website/.storybook/main.ts` 的 `staticDirs`，2026-11-15；已確認）。
- 改寫期間 story 保留到步驟 5 才刪：Storybook 的逐 story axe（`parameters.a11y.test: "error"`）在步驟 4 的 axe 掃描上線前繼續守 a11y，不留空窗（預設（可推翻））。
- 新測試放在被測元件旁（`<元件>.browser.test.tsx`），共用 render helper 與 setup 放 `website/tests/browser/`（預設（可推翻））。

**驗收基線：** 快照 commit 上 CI 的 website-e2e 最近一次 dev run（`gh run list -w website-e2e.yml -b dev -L 1`，run 43667284291）開工時仍在跑，步驟 1 開工前讀它每個 job 的結論並記到進度表；有失敗的 job 以那份結果為既有失敗，不在本計畫修。本機可跑的 `pnpm lint`、`pnpm test`、`pnpm knip`、`pnpm design:check --apps-root ..` 在步驟 1 開工時跑一次記錄。

步驟 1 開工時的結果（2026-11-15）：run 43667284291 每個 job 綠；`pnpm lint`、`pnpm test`、`pnpm design:check --apps-root ..` 通過；`pnpm knip` 在快照上就失敗（CI 不跑它），基線與比對方法見下一段；`pnpm architecture:check` 在快照上就失敗（`components/ui/Badge.tsx` 不存在）。

knip 基線（2026-11-15 重做）：附件 `knip-baseline.json` 是 d69bbbbc（併入 origin/dev 後）的 `pnpm knip --reporter json` 經 `python3 knip-diff.py --normalize` 正規化，每筆以（分類、檔案、名稱）為鍵、不含行列位置。各步驟的 knip 驗收是在 repo 根目錄跑 `pnpm -C website -s knip --reporter json | python3 website/docs/plans/2026-11-15-remove-storybook/knip-diff.py`，exit 0（沒有新增；消失的只計數）。換基線的理由：原基線只存名稱並去重，丟掉檔案，exports 475→434、types 447→427、unlisted 5→1，所以完整輸出和它比永遠多 65 筆；另 1 筆 types 與 15 筆 catalogReferences 是 d69bbbbc 併入的 dev 帶來的（922a4ccf→d69bbbbc 逐筆比對），不是本計畫的改動。8494f0fa、922a4ccf 的輸出逐筆相同，所以步驟 1 沒有新增；d69bbbbc 與 0e4b2b02 逐筆相同。腳本 mutation：在輸出加一筆假 export，exit 1。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| unit 測試程序維持 hermetic，不載入瀏覽器或 Next 設定 | `website/vitest.config.ts` 的 `unit` project（`environment: "node"`、`tests/setup/hermetic.mjs`）；新的 browser project 不得放進預設 `pnpm test` |
| Playwright 瀏覽器版本與 CI 映像一致 | `.github/workflows/website-e2e.yml` 的「確認 Playwright 版本與映像一致」步驟；新的 browser job 沿用同一映像與檢查 |
| e2e 的 server 回應全由 `page.route` 攔截 | `website/tests/e2e/**`；步驟 4 的 axe 掃描沿用既有 spec 的 mock，不打真的後端 |

## 規模

| | 步驟 1（試點） | 步驟 2（基礎元件） | 步驟 3（Cart、DescriptionEditor） | 產生計數的位置 |
|---|---|---|---|---|
| story 檔 | 3 | 21 | 7 | `inventory.md` §1（`rg -c '\bplay\s*:' website/stories`） |
| play | 8 | 74 | 57 | 同上 |

## 範圍

- 做：play 改寫、browser 測試的 runner 與 CI job、axe 進 e2e、刪除 Storybook 與連帶的 Flutter 目錄 web 宿主、文件更新。
- 不做：視覺回歸（Storybook 目前也沒有）；admin（沒有 Storybook）；Flutter 目錄的其他改動（D2 只刪 Storybook 宿主留下的嵌入模式與 base href）。
- 延後：`/about` 的 `.stage-cta` 樣式同時寫在 `HomeProcessCards.tsx` 的 Tailwind 與未分層的 `home.css`，後者永遠蓋過前者（code-review 第 2 輪，Will 2026-11-15 決定延後；觸發條件：下次修改 `/about` 首屏或把 `home.css` 收進 layer 時，收斂到單一來源）。
- 延後：公開頁 axe 進 CI（觸發條件：#369 的錄製 fixture 可用時，把 `expectNoAxeViolations` 接到不帶 `@live` 的公開頁測試）。
- 延後：Flutter 元件目錄的 web 版發佈（觸發條件：Will 需要在瀏覽器分享 App 元件目錄時）；屆時一併重新評估現在沒有消費端的 `?component=&textScale=` 深連結解析與 `component-catalog.sh build`（`CATALOG_BASE_HREF`），保留理由：本機手動開特定 section 仍可用。

## 步驟

**mutation 流程（各步共用）：** 在本步驟的 PR（`remove-storybook` 系列分支 → `dev`，`pull_request` 會觸發 `website-e2e.yml`）推一個只含破壞的暫時 commit，確認對應 job 紅燈後 `git revert` 它，紅燈與回綠的 run 連結記進進度表。

- [x] **1. Browser runner 與試點**（被擋於：無）
  - 範圍：spike 在不加依賴的前提下驗證三項，結論逐項寫進本步驟：JSX 轉譯與 React `act` 環境（tsconfig `jsx: react-jsx`，沒有 `@vitejs/plugin-react`）、Next.js 模組（`next/navigation`、`next/image`、`next/link`）用 `vi.mock` 或 Vite alias 替代、全域 CSS 匯入。`website/vitest.config.ts` 新增 `browser` project（include `**/*.browser.test.tsx`，`@vitest/browser-playwright` chromium headless），只由新 script `test:browser`（`vitest run --project browser`）跑，`pnpm test` 不變；`website/tests/browser/` 放 setup（全域 CSS、Intl provider 的 render helper）。改寫 `Button`、`Dialog`、`NavigationRecovery`（含 `sb.mock` spy 的替代）共 8 個 play。`.github/workflows/website-e2e.yml` 新增「元件 browser 測試」job（同 storybook job 的映像與版本檢查），與 storybook job 並存。
  - 消費端：`rg -n 'vitest.config|VITEST_STORYBOOK|--project' website .github`
  - 不能動：`unit` project 的設定與 hermetic setup；storybook job 與 story 檔（步驟 5 才刪）。
  - 驗收：本機 `pnpm lint`、`pnpm test`、`pnpm knip` 通過；PR 上新 job 綠且 log 顯示 8 個以上測試通過；mutation：把 `Button` 的 disabled 處理改壞（mutation 流程見下）。
  - spike 結論（CI 實跑證實）：(a) Vite 8 讀 tsconfig 的 `jsx: react-jsx` 轉 TSX，不需 `@vitejs/plugin-react`；沒開 vitest globals，`tests/browser/setup.ts` 自己註冊 `cleanup`。(b) 試點元件沒有用到 `next/*`；`vi.mock("next/navigation", importOriginal)` 對 Vite 預先打包的依賴會匯入失敗（CI 實證：does not provide an export named 't'）；步驟 2、3 需要 router 時改 mock 專案自己的 `@/lib/intl/use-router`（`importOriginal` 保留 `useLocalizedHref`，`useRouter` 換成 `tests/browser/static-router.ts` 的靜態 router），不 mock `next/*`，唯一例外是 `next/image`：它是 CJS，預先打包後 default 變成物件，渲染 `AppImage` 丟 "Element type is invalid"（CI 實證，ProductGallery 3 條）。Storybook 的替身來自 `@storybook/nextjs-vite` 內部套件，步驟 N 移除後不能依賴；`vi.mock("next/image")` 放在 setup 檔也不生效（CI 實證，修正後仍同樣 3 條失敗），所以改成解析時替換：browser project 的 `resolve.alias` 把正規表示式 `^next/image$` 對到 `tests/browser/next-image.tsx`（輸出 `<img>`：src 原樣、fill 絕對定位、丟掉 loader／quality／priority 等 next 專屬屬性），並把 `next/image` 加進該 project 的 `optimizeDeps.exclude`。已查：其餘被測元件沒有用到 `next/dynamic` 或 `next/link`（`next/link` 只經專案自己的 `@/lib/intl/*`，而且 89 條現有通過）。(c) 全域 CSS 在 setup 匯入與 `.storybook/preview.tsx` 相同的 4 檔。另外：瀏覽器沒有 `process`，browser project 以 `define` 替換 `process.env.NODE_ENV` 與 loopback 的 `NEXT_PUBLIC_API_BASE_URL`，setup 補空的 `process` 墊片（Next client 的行為）。改寫注意：`vitest/browser` 的 `toHaveTextContent` 傳字串是整段比對，不是 jest-dom 的子字串。
  - 延後：`.storybook/preview.css` 的字型與 body 顏色沒搬進 setup（觸發條件：改寫的斷言依賴字型、顏色或版面時，把需要的 token 搬進 `tests/browser/`）。
  - 停止條件：Next.js 模組的替代需要新依賴（例如 `@vitejs/plugin-react` 或 Storybook 以外的 Next 模擬套件）——列出套件與版本回報，等 Will 決定。
- [x] **2. 基礎元件改寫**（被擋於：步驟 1）
  - 範圍：`inventory.md` §1 標「步驟 2 改寫」的 21 檔、74 個 play。可拆成兩個子批（各自 PR 綠才算完成）。
  - 消費端：無（新增測試檔）。
  - 不能動：被測元件的程式（測試搬移暴露的缺陷另開 bug-fix，不在本步驟順手改）。
  - 驗收：每個 story 檔的 play 數與新測試的斷言對照表貼在 PR 描述（`plays.txt` 為來源）；CI 元件 browser job 綠；mutation：抽 3 個元件（含 `DatePicker`、`ProcurementClarifyQuestion`）各破壞一個被斷言的行為，job 要失敗。
  - 停止條件：某個 play 依賴 Storybook 專屬行為且無法等價改寫——回報，不刪弱斷言。
  - 延後：深色主題的測試只斷言 `data-theme=dark`（與原 play 相同，但該屬性由 render helper 設定，驗不到深色樣式；觸發條件：步驟 4 的 axe 或任何深色樣式回歸需要元件層證據時補樣式斷言）；只出現在 `*.browser.test.tsx` 的 Tailwind class 不會產生（`globals.css` 的 `@source not`；觸發條件：某個改寫需要 app 程式碼沒用過的外框 class，屆時讓 browser setup 掃描測試檔）。
  - 已決（Will 2026-11-15 選查清來源）：knip 基線。66 筆中 65 筆是原基線格式去重造成的假差異，1 筆 types 來自併入的 dev；基線與比對方法已改，見驗收基線段。
- [x] **3. Cart、DescriptionEditor 系列**（被擋於：步驟 1；與步驟 2 都會改 `website/vitest.config.ts` 的 browser project 與 `website/tests/browser/`，兩步依序合併，不並行）
  - 範圍：`inventory.md` §1 標「步驟 3 改寫」的 7 檔、57 個 play；msw service worker 與 `description-image-fixtures.ts` 搬到 `website/tests/browser/`。
  - 消費端：`rg -n 'description-image-fixtures|mockServiceWorker' website .github`
  - 不能動：`.github/workflows/website-e2e.yml` 第 86 行描述的圖片網域判定（fixture 攔的網域不變）。
  - 驗收：同步驟 2 的對照表與 CI；mutation：`DescriptionEditorDeletion` 與 `Cart` 各破壞一個被斷言的行為，job 要失敗。
  - 實作發現（CI 實證）：vitest browser 預設視窗寬 414px，Editor.js 在 650px 以下視為手機版、不依滑鼠移動區塊工具列（`editorjs.mjs` 的 `be()` 與 `enableModuleBindings`），依賴 hover 的測試要先 `page.viewport(1280, 800)` 再掛載；`*Sync` matcher 只檢查一次，掛載或操作後的 DOM 斷言要包在 `waitFor`。
  - 延後：`DescriptionEditorImage` 的 msw worker 啟動失敗時清理步驟丟 TypeError 蓋掉原錯誤、`Cart` 檔內手動 `cleanup()` 與 setup 重複並依賴 hook 順序（觸發條件：任一處造成難診斷的 CI 失敗，或下次修改 `tests/browser/setup.ts` 或這兩個測試檔的清理流程時（步驟 5 沒有整理到，2026-11-15 改寫））；Cart 的 `--hk-text-scale` 斷言只驗到測試 helper 自己設的值（觸發條件：需要驗外層字級設定時，改由 app 的 layout 設定）。
- [x] **4. a11y 改用 axe 進 Playwright e2e**（被擋於：步驟 1；D1 已決）
  - 範圍：依 D1 加入套件，在既有 e2e（`website/tests/e2e/`）的主要頁面加 axe 掃描，規則等級對齊 Storybook 的 `a11y.test: "error"`（違規即失敗）；涵蓋原 story 的元件出現的頁面清單寫在 spec 開頭註解之外的 PR 描述。
  - 消費端：`rg -ln 'test\(' website/tests/e2e`
  - 不能動：既有 e2e 的 mock 與斷言。
  - 驗收：CI website-e2e 每個 shard 綠；mutation：移除一個被掃描頁面上表單元素的 label，掃描要失敗。
  - 停止條件：既有頁面掃出違規——列清單回報，由 Will 決定修或以規則例外接受，不自行關規則。
- [x] **5. 確認並刪除 Storybook**（被擋於：步驟 2、3、4）
  - 範圍：先在刪除後的工作區跑 `pnpm design:check --apps-root ..` 與 `pnpm design:test` 證明不依賴 stories；再依 `inventory.md` §3 刪除全部 story、`.storybook/`、`vitest.storybook.config.ts`、`tsconfig.storybook.json`、7 個 scripts、依賴（6 個加 knip 判定的第 7 個）、patch 與 `pnpm-workspace.yaml` 條目、`UPSTREAM-WATCH.md` 條目、dependabot 規則、CI 的 storybook job、相關腳本與測試、`.mcp.json`，並更新盤點表列出的文件與 `.claude/rules/website.md`。
  - 消費端：`rg -il 'storybook|stories/' --hidden -g '!**/node_modules/**' -g '!.git/**'`
  - 不能動：盤點表標「保留」的歷史紀錄。
  - 驗收：`pnpm install` 後 lockfile 無 `@storybook/`（`rg -c '@storybook/' pnpm-lock.yaml` 為 0）；本機 `pnpm lint`、`pnpm test`、`pnpm knip`、`pnpm design:check --apps-root ..`、`pnpm design:test`、根目錄 repo-hygiene 對應檢查通過；CI website-ci、website-e2e、supply-chain、buyer-ci、seller-ci（App 檔案改動觸發）每個 job 自己的結論綠；上面的消費端搜尋只剩保留項與本計畫。
- [x] **6a. 公開頁 axe 進 CI**（已取代：Will 2026-11-15 決定公開頁 a11y 先不進 CI——另養一套公開頁 mock 對個人專案的維護成本過高；e5c2865e 由 6aac8968 revert；CI 首跑抓到的公開頁違規照修（5aefe6a7，未經 CI 驗證）；延後項見「範圍」。原文：來源：步驟 6 設計審查 blocking；Will 2026-11-15 選「本 PR 補 mock」）：`public-static-routes` 全標 `@live`、CI 不跑，公開頁沒有 axe。範圍：e2e 以 `page.route` mock 訪客外框的 4 個端點（`/public/categories/tree`、`/public/hashtags/popular`、`/public/tags/trending`、`/public/posts`），新增不帶 `@live` 的 spec 對 23 條公開靜態路由跑 `expectNoAxeViolations`；不改既有 `@live` 測試，日後 #369 的錄製 fixture 可替換這些 mock。被擋於：無。消費端：`rg -n 'expectNoAxeViolations|PUBLIC_STATIC_ROUTES' website/tests/e2e`。不能動：既有 `@live` spec、不變量「e2e 的 server 回應全由 `page.route` 攔截」。驗收：website-e2e 每個 shard 綠且新 spec 有執行（job log 看得到 23 條）；mutation：拿掉 `/faq`（或任一公開頁）某個表單或連結的可及名稱，新 spec 失敗，revert 後回綠。
- [x] **6b. 狀態層 axe 進 browser test**（來源：步驟 6 設計審查 should；Will 2026-11-15 選「補 axe」並同意 devDependencies 宣告 `axe-core` 4.14.0，與 lockfile 同版）：`tests/browser/` 加共用 helper 以 `axe-core` 對 `document` 跑並斷言零違規；在互動後才出現的狀態呼叫：Dialog、Dropdown、Sheet、DatePicker 展開，Input 錯誤狀態，DescriptionEditor 的錯誤與刪除狀態。被擋於：無。消費端：`rg -l 'browser.test.tsx' website`。不能動：既有斷言（只加不刪）。驗收：元件 browser job 綠；mutation：拿掉 Dialog 標題與 `aria-labelledby` 的關聯，對應 axe 斷言失敗，revert 後回綠；knip（`knip-diff.py`）new 0。
- [ ] **7. 收尾**：完成定義逐項核對；design-review skill 審整體 diff；延後項（Flutter 目錄 web 版）附觸發條件搬回主清單；合回 `dev`、推送前跑觸發到的檢查，移除 worktree 與分支。驗收：完成定義三項各附指令輸出；design-review 報告；`git worktree list` 不含 `remove-storybook`。

## 決定

- **D1 a11y 掃描用哪個套件**（狀態：已決 2026-11-15，Will；擋住步驟：4；需要的事實：無）
  - A：`@axe-core/playwright`——Deque 官方的 Playwright 整合，業界常見做法；新增一個 devDependency（axe-core 4.11.4 已是傳遞依賴）。（建議）
  - B：直接用 `axe-core` 並在 `page.evaluate` 注入——不加整合套件，但要自己寫注入與結果格式。
  - 結論：A，`@axe-core/playwright` 4.14.0（2026-10-15 發佈，peer `playwright-core >= 1.0.0`）；步驟 4 開工時以 `npm view @axe-core/playwright@4.14.0 dependencies` 確認帶入的 `axe-core` 版本；ADR：無（Will 已決定方向，這題只是套件選擇）。

- **D2 Flutter 元件目錄的嵌入模式**（狀態：已決 2026-11-15，Will；擋住步驟：5；需要的事實：無）
  - 背景：Storybook 是 `embedded=true` 與 `/flutter/buyer|seller/` base href 唯一的宿主，刪除後沒有消費端；原範圍「不改 Flutter 目錄」與共通規則「不留技術債」衝突。
  - 結論：兩支 App 刪除 `embedded` 模式，`component-catalog.sh` 預設 base href 改為 `/`；延後項的 web 版發佈屆時重新設計。

拍板順序：無未決題。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 Browser runner 與試點 | 完成 | lint、test、knip（對 8494f0fa 逐筆無新增，2026-11-15 重測）本機；PR #377 新 job 8 測試綠（run 43672612523，含 8 個 e2e shard）；mutation 忽略 `disabled` 時只有 `is disabled when disabled is set` 失敗（run 43672321091），revert 後回綠 | — | 36649e77..a74318de |
| 2 基礎元件改寫 | 完成 | lint、`test:unit` 本機；PR #377 在 dcd3f3bd 全部檢查綠（website-e2e run 43680495072：元件 browser job 25 檔 92 測試、8 個 e2e shard；website-ci run 43680495013）；mutation 各只有目標測試失敗後 revert 回綠：DatePicker 選日期不回報 onChange（run 43675932096）、ClarifyQuestionCard 描述不 trim（run 43679360536）、Tabs 非受控點擊不切換（run 43679556022）；play 對照表在 PR 描述；`/code-review` 2a 兩輪、2b 一輪，finding 全部修正或附理由駁回 | knip：0e4b2b02 對新基線（d69bbbbc）`knip-diff.py` new 0；原「多 66 筆」已查清，見驗收基線段 | e83cd330..dcd3f3bd |
| 3 Cart、DescriptionEditor | 完成 | lint、`test:unit`、knip（`knip-diff.py` new 0）本機；PR #377 在 44284b2e 全部檢查綠（website-e2e run 43688865121：元件 browser job 32 檔 151 測試、8 個 e2e shard）；mutation（run 43688094850）拿掉存檔失敗後的重試時只有「最後一次存檔失敗 · 補存後 onChange 跟上」失敗、不可購買商品數量不停用時只有「disables quantity for an unavailable item…」失敗，revert 後回綠；play 對照表在 PR 描述；`/code-review` 兩輪加 Will 同意的第三輪，契約對照審查 1 項 should 已修 | — | 831991d8..44284b2e |
| 4 axe 進 e2e | 完成 | 掃描接在 protected／public static routes 與 `/cart`；首跑 51 頁違規（run 43696298022），Will 決定全部修；修正後 website-e2e 全綠（run 43709161412，元件 browser job、8 個 e2e shard，含 `/cart`）；mutation 拿掉 `/seller-center/coupons/new` 優惠券名稱的 label 時掃描報 `label`／`label-title-only` 失敗（run 43706283438），revert 後回綠；`label-content-name-mismatch` 的成因以診斷 commit 加 axe 4.14.0 原始碼證實（axe 以 `screenReader=false` 計算可見文字，`aria-hidden` 的可見文字也算）；`/code-review` 兩輪：第一輪 9 項全修，第二輪 10 項：修 7 項（live region 字串、表格 region class 合併附 red-proof；手機寬度空 region×3 改為只在溢出時成為可聚焦 region；sr-only 名稱元素不得為 span 的測試；ImageUploader 文字單一來源；AI 助手按鈕以 aria-describedby 補用途），Will 2026-11-15 決定；本機 lint、test、test:unit、knip 綠 | 上傳鈕名稱含檔案提示：已接受（Will 2026-11-15）；早退狀態 h1 收斂到共用外框：延後，觸發條件為新增早退狀態時；`public-static-routes` 標 `@live`，CI 不跑 | 519fe31e..dfe9696a |
| 5 確認並刪除 Storybook | 完成 | 本機：lockfile `@storybook/` 0 筆、`pnpm lint`、`pnpm test`（2936）、`test:unit`、`design:check --apps-root ..`、`design:test`、knip（`knip-diff.py` new 0）、`check-doc-paths.py`、buyer `flutter test`（1499）、seller `flutter test`（2279）；PR #377 在 ad987139 全部 workflow 綠（website-e2e run 43716667299、website-ci 43716667534、supply-chain 43716667533、buyer 43716667377、seller 43716667411 等 12 個）；消費端搜尋只剩四個保留檔、本計畫與附件、測試圖檔 key `public/storybook/…svg`（測試資料，不依賴 Storybook）；`/code-review` 兩輪：第一輪 10 項（修 8、延後 1 後經 D2 改為修、駁回 1：元件層 a11y 由步驟 4 前提決定），第二輪 9 項依 Will 2026-11-15 指示一次修完，以 CI 為準不開第三輪 | — | 92b20f3c..ad987139 |
| 6a 公開頁 axe 進 CI | 已取代（Will 2026-11-15） | 首跑（run 43719354572）抓到公開頁違規 13 頁，已修（5aefe6a7） | 公開頁修正未經 CI 驗證：spec 已 revert | e5c2865e、6aac8968 |
| 6b 狀態層 axe | 完成 | 本機 lint、test（2936）、test:unit、design:check、knip new 0；首跑（run 43719354572）抓到 DatePicker、Dropdown、DescriptionEditor 違規，修正與 harness 調整（元件層關掉整頁規則、掃描前等有限動畫與剛觸發的 transition）在 5aefe6a7、7948cc1d；淺色 Dropdown active 列 1.88:1 依 Will 2026-11-15 改 ink-600／ink-700（3d90d697）；website-e2e 全綠（run 43724678097：元件 browser 151、8 個 e2e shard）；mutation：舊 token（改回前）只有 axe 斷言失敗（run 43723835555，Dropdown color-contrast 1.88:1），新 token 回綠；計畫原訂的 Dialog 名稱 mutation 被既有 `getByRole(name)` 先擋下（run 43725184240），證明不了 axe，改以上述 token mutation 為證，已 revert（fdd0db51） | — | acc2a6b2..fdd0db51 |
| 7 收尾 | 進行中 | 本機（a8bf4afe 併入 origin/dev 後）：lint、`pnpm test`（2936）、`design:check --apps-root ..`、knip new 0；`rg -il storybook` 只剩三份保留歷史檔與本計畫附件（測試 fixture 名稱改名 e838be8b）；design-review 第 1 輪：5 項（blocking 1、should 2、nit 2）→ 6a、6b、步驟 3 延後項改觸發條件、Flutter 深連結記延後項，pre-push 腳本一項駁回（`scripts/pre-push-commit-msg.sh` 在 merge-base 已存在，本分支只改範圍，c5c727d9） | CI on c5c727d9 待；6a、6b 完成後重跑完成定義 | e838be8b、c5c727d9 |
