# 拆除 website Storybook

**Goal：** website 不再依賴 Storybook：原本 33 個 story 檔的 144 個 play 都改寫成 Vitest browser mode 的 `*.browser.test.tsx` 並在 CI 執行，a11y 由 Playwright e2e 的 axe 掃描守，Storybook 的依賴、patch、設定、scripts 與 CI job 全部刪除（驗收見步驟 6）。

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

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| unit 測試程序維持 hermetic，不載入瀏覽器或 Next 設定 | `website/vitest.config.ts` 的 `unit` project（`environment: "node"`、`tests/setup/hermetic.mjs`）；新的 browser project 不得放進預設 `pnpm test` |
| Playwright 瀏覽器版本與 CI 映像一致 | `.github/workflows/website-e2e.yml` 的「確認 Playwright 版本與映像一致」步驟；新的 browser job 沿用同一映像與檢查 |
| e2e 的 server 回應全由 `page.route` 攔截 | `website/tests/e2e/**`；步驟 4 的 axe 掃描沿用既有 spec 的 mock，不打真的後端 |

## 規模

| | 步驟 1（試點） | 步驟 2（基礎元件） | 步驟 3（Cart、DescriptionEditor） | 產生計數的位置 |
|---|---|---|---|---|
| story 檔 | 3 | 23 | 7 | `inventory.md` §1（`rg -c 'play\s*:' website/stories`） |
| play | 8 | 78 | 58 | 同上 |

## 範圍

- 做：play 改寫、browser 測試的 runner 與 CI job、axe 進 e2e、刪除 Storybook 與連帶的 Flutter 目錄 web 宿主、文件更新。
- 不做：視覺回歸（Storybook 目前也沒有）；admin（沒有 Storybook）；Flutter 目錄本身的改動（只改指向 Storybook 的說明）。
- 延後：Flutter 元件目錄的 web 版發佈（觸發條件：Will 需要在瀏覽器分享 App 元件目錄時）。

## 步驟

**mutation 流程（各步共用）：** 在本步驟的 PR（`remove-storybook` 系列分支 → `dev`，`pull_request` 會觸發 `website-e2e.yml`）推一個只含破壞的暫時 commit，確認對應 job 紅燈後 `git revert` 它，紅燈與回綠的 run 連結記進進度表。

- [ ] **1. Browser runner 與試點**（被擋於：無）
  - 範圍：spike 在不加依賴的前提下驗證三項，結論逐項寫進本步驟：JSX 轉譯與 React `act` 環境（tsconfig `jsx: react-jsx`，沒有 `@vitejs/plugin-react`）、Next.js 模組（`next/navigation`、`next/image`、`next/link`）用 `vi.mock` 或 Vite alias 替代、全域 CSS 匯入。`website/vitest.config.ts` 新增 `browser` project（include `**/*.browser.test.tsx`，`@vitest/browser-playwright` chromium headless），只由新 script `test:browser`（`vitest run --project browser`）跑，`pnpm test` 不變；`website/tests/browser/` 放 setup（全域 CSS、Intl provider 的 render helper）。改寫 `Button`、`Dialog`、`NavigationRecovery`（含 `sb.mock` spy 的替代）共 8 個 play。`.github/workflows/website-e2e.yml` 新增「元件 browser 測試」job（同 storybook job 的映像與版本檢查），與 storybook job 並存。
  - 消費端：`rg -n 'vitest.config|VITEST_STORYBOOK|--project' website .github`
  - 不能動：`unit` project 的設定與 hermetic setup；storybook job 與 story 檔（步驟 5 才刪）。
  - 驗收：本機 `pnpm lint`、`pnpm test`、`pnpm knip` 通過；PR 上新 job 綠且 log 顯示 8 個以上測試通過；mutation：把 `Button` 的 disabled 處理改壞（mutation 流程見下）。
  - 停止條件：Next.js 模組的替代需要新依賴（例如 `@vitejs/plugin-react` 或 Storybook 以外的 Next 模擬套件）——列出套件與版本回報，等 Will 決定。
- [ ] **2. 基礎元件改寫**（被擋於：步驟 1）
  - 範圍：`inventory.md` §1 標「步驟 2 改寫」的 23 檔、78 個 play。可拆成兩個子批（各自 PR 綠才算完成）。
  - 消費端：無（新增測試檔）。
  - 不能動：被測元件的程式（測試搬移暴露的缺陷另開 bug-fix，不在本步驟順手改）。
  - 驗收：每個 story 檔的 play 數與新測試的斷言對照表貼在 PR 描述（`plays.txt` 為來源）；CI 元件 browser job 綠；mutation：抽 3 個元件（含 `DatePicker`、`ProcurementClarifyQuestion`）各破壞一個被斷言的行為，job 要失敗。
  - 停止條件：某個 play 依賴 Storybook 專屬行為且無法等價改寫——回報，不刪弱斷言。
- [ ] **3. Cart、DescriptionEditor 系列**（被擋於：步驟 1；與步驟 2 都會改 `website/vitest.config.ts` 的 browser project 與 `website/tests/browser/`，兩步依序合併，不並行）
  - 範圍：`inventory.md` §1 標「步驟 3 改寫」的 7 檔、58 個 play；msw service worker 與 `description-image-fixtures.ts` 搬到 `website/tests/browser/`。
  - 消費端：`rg -n 'description-image-fixtures|mockServiceWorker' website .github`
  - 不能動：`.github/workflows/website-e2e.yml` 第 86 行描述的圖片網域判定（fixture 攔的網域不變）。
  - 驗收：同步驟 2 的對照表與 CI；mutation：`DescriptionEditorDeletion` 與 `Cart` 各破壞一個被斷言的行為，job 要失敗。
- [ ] **4. a11y 改用 axe 進 Playwright e2e**（被擋於：D1）
  - 範圍：依 D1 加入套件，在既有 e2e（`website/tests/e2e/`）的主要頁面加 axe 掃描，規則等級對齊 Storybook 的 `a11y.test: "error"`（違規即失敗）；涵蓋原 story 的元件出現的頁面清單寫在 spec 開頭註解之外的 PR 描述。
  - 消費端：`rg -ln 'test\(' website/tests/e2e`
  - 不能動：既有 e2e 的 mock 與斷言。
  - 驗收：CI website-e2e 每個 shard 綠；mutation：移除一個被掃描頁面上表單元素的 label，掃描要失敗。
  - 停止條件：既有頁面掃出違規——列清單回報，由 Will 決定修或以規則例外接受，不自行關規則。
- [ ] **5. 確認並刪除 Storybook**（被擋於：步驟 2、3、4）
  - 範圍：先在刪除後的工作區跑 `pnpm design:check --apps-root ..` 與 `pnpm design:test` 證明不依賴 stories；再依 `inventory.md` §3 刪除全部 story、`.storybook/`、`vitest.storybook.config.ts`、`tsconfig.storybook.json`、7 個 scripts、依賴（6 個加 knip 判定的第 7 個）、patch 與 `pnpm-workspace.yaml` 條目、`UPSTREAM-WATCH.md` 條目、dependabot 規則、CI 的 storybook job、相關腳本與測試、`.mcp.json`，並更新盤點表列出的文件與 `.claude/rules/website.md`。
  - 消費端：`rg -il 'storybook|stories/' --hidden -g '!**/node_modules/**' -g '!.git/**'`
  - 不能動：盤點表標「保留」的歷史紀錄。
  - 驗收：`pnpm install` 後 lockfile 無 `@storybook/`（`rg -c '@storybook/' pnpm-lock.yaml` 為 0）；本機 `pnpm lint`、`pnpm test`、`pnpm knip`、`pnpm design:check --apps-root ..`、`pnpm design:test`、根目錄 repo-hygiene 對應檢查通過；CI website-ci、website-e2e、supply-chain、buyer-ci、seller-ci（App 檔案改動觸發）每個 job 自己的結論綠；上面的消費端搜尋只剩保留項與本計畫。
- [ ] **6. 收尾**：完成定義逐項核對；design-review skill 審整體 diff；延後項（Flutter 目錄 web 版）附觸發條件搬回主清單；合回 `dev`、推送前跑觸發到的檢查，移除 worktree 與分支。驗收：完成定義三項各附指令輸出；design-review 報告；`git worktree list` 不含 `remove-storybook`。

## 決定

- **D1 a11y 掃描用哪個套件**（狀態：未決；擋住步驟：4；需要的事實：版本與發佈日期，步驟 4 開工時查）
  - A：`@axe-core/playwright`——Deque 官方的 Playwright 整合，業界常見做法；新增一個 devDependency（axe-core 4.11.4 已是傳遞依賴）。（建議）
  - B：直接用 `axe-core` 並在 `page.evaluate` 注入——不加整合套件，但要自己寫注入與結果格式。
  - 結論：待 Will 核准套件與版本；ADR：無（Will 已決定方向，這題只是套件選擇）。

拍板順序：D1 可以現在拍，步驟 1–3 不受它影響。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 Browser runner 與試點 | 未開始 | | | |
| 2 基礎元件改寫 | 未開始 | | | |
| 3 Cart、DescriptionEditor | 未開始 | | | |
| 4 axe 進 e2e | 未開始 | | | |
| 5 確認並刪除 Storybook | 未開始 | | | |
| 6 收尾 | 未開始 | | | |
