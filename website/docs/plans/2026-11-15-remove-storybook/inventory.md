# 盤點：website Storybook 拆除

快照 commit：`8494f0fa`（dev HEAD，含 `5beba7cf`）。路徑從 repo 根目錄起算。

## 1. Story 檔與 play（去向）

產生：`rg -c '\bplay\s*:' website/stories | sort`（\b 排除 CSS `display:` 的誤配；結果存在 `plays.txt`，31 檔、合計 139）；全部 story 檔：`find website/stories -name '*.stories.*' | sort`（38 檔）。

| Story 檔 | play 數 | 去向 |
|---|---|---|
| `website/stories/Foundations.stories.tsx` | 0 | 刪除：沒有 play、沒有斷言 |
| `website/stories/ProductGallery.stories.tsx` | 3 | 步驟 2 改寫 |
| `website/stories/web/AfterSalePhotoUploader.stories.tsx` | 2 | 步驟 2 改寫 |
| `website/stories/web/BrandLogoLoader.stories.tsx` | 0 | 刪除：沒有 play、沒有斷言 |
| `website/stories/web/Button.stories.tsx` | 4 | 步驟 1 試點改寫 |
| `website/stories/web/BuyerQuoteComparison.stories.tsx` | 6 | 步驟 2 改寫 |
| `website/stories/web/CalendarMonth.stories.tsx` | 2 | 步驟 2 改寫 |
| `website/stories/web/Card.stories.tsx` | 1 | 步驟 2 改寫 |
| `website/stories/web/DatePicker.stories.tsx` | 4 | 步驟 2 改寫 |
| `website/stories/web/Dialog.stories.tsx` | 1 | 步驟 1 試點改寫 |
| `website/stories/web/DrawerSkeleton.stories.tsx` | 1 | 步驟 2 改寫 |
| `website/stories/web/Dropdown.stories.tsx` | 5 | 步驟 2 改寫 |
| `website/stories/web/ImportItemStatusBadge.stories.tsx` | 1 | 步驟 2 改寫 |
| `website/stories/web/InlineLoading.stories.tsx` | 2 | 步驟 2 改寫 |
| `website/stories/web/Input.stories.tsx` | 5 | 步驟 2 改寫 |
| `website/stories/web/NavigationRecovery.stories.tsx` | 3 | 步驟 1 試點改寫（用到 `sb.mock` 的 spy） |
| `website/stories/web/PostContent.stories.tsx` | 6 | 步驟 2 改寫 |
| `website/stories/web/ProcurementClarifyQuestion.stories.tsx` | 9 | 步驟 2 改寫 |
| `website/stories/web/QuoteBatchSummary.stories.tsx` | 8 | 步驟 2 改寫 |
| `website/stories/web/RouteLoadingSkeleton.stories.tsx` | 4 | 步驟 2 改寫 |
| `website/stories/web/Select.stories.tsx` | 3 | 步驟 2 改寫 |
| `website/stories/web/SellerSkeletons.stories.tsx` | 5 | 步驟 2 改寫 |
| `website/stories/web/Sheet.stories.tsx` | 2 | 步驟 2 改寫 |
| `website/stories/web/Switch.stories.tsx` | 1 | 步驟 2 改寫 |
| `website/stories/web/Tabs.stories.tsx` | 3 | 步驟 2 改寫 |
| `website/stories/web/Textarea.stories.tsx` | 1 | 步驟 2 改寫 |
| `website/stories/web/Cart.stories.tsx` | 14 | 步驟 3 改寫 |
| `website/stories/web/DescriptionEditor.stories.tsx` | 10 | 步驟 3 改寫 |
| `website/stories/web/DescriptionEditorColor.stories.tsx` | 5 | 步驟 3 改寫 |
| `website/stories/web/DescriptionEditorDeletion.stories.tsx` | 13 | 步驟 3 改寫 |
| `website/stories/web/DescriptionEditorImage.stories.tsx` | 9 | 步驟 3 改寫（用 msw 圖片 fixture） |
| `website/stories/web/DescriptionEditorTableDeletion.stories.tsx` | 2 | 步驟 3 改寫 |
| `website/stories/web/DescriptionColorPicker.stories.tsx` | 4 | 步驟 3 改寫 |
| `website/stories/web/Chip.stories.tsx` | 0 | 刪除：只有展示，沒有斷言；a11y 改由步驟 4 的頁面掃描覆蓋 |
| `website/stories/web/Stars.stories.tsx` | 0 | 刪除：同上 |
| `website/stories/apps/Buyer.stories.tsx` | 0 | 刪除：只嵌入 `storybook:apps` 產出的 Flutter web 目錄，Storybook 是唯一宿主 |
| `website/stories/apps/Seller.stories.tsx` | 0 | 刪除：同上 |
| `website/stories/CrossPlatform.stories.tsx` | 0 | 刪除：token 對照的展示；token 一致性由 `design:check` 讀 `website/design-system/*.json` 的 sources 守，不讀 stories |

| `website/stories/web/Cart.stories.tsx` 的 `BuyerApp`、`BuyerAppLargeText`（1077–1114 行，無 play） | 0 | 刪除：iframe 嵌入 `storybook:apps` 的 Flutter 目錄，理由同 `stories/apps`；Cart 其餘 story 照步驟 3 |
| `website/stories/apps/FlutterCatalog.tsx`、`apps/BuyerApi.mdx`、`apps/SellerApi.mdx` | — | 刪除：Flutter 目錄 web 宿主的元件與說明 |
| `website/stories/Introduction.mdx` | — | 刪除：Storybook 首頁導覽 |
| `website/stories/documentation/Maintenance.mdx`（68 行） | — | 步驟 5 逐段判斷：仍適用的維護規則搬到 `website/docs/component-library.md`，只關於 Storybook 操作的刪除；搬移對照列在 PR 描述 |

play 合計：步驟 1 = 8、步驟 2 = 74、步驟 3 = 57，共 139。非 story 檔來源：`find website/stories -type f ! -name '*.stories.*'`（5 檔）。

## 2. Storybook 執行期提供、改寫時要另外補的東西

來源：`website/.storybook/preview.tsx`、`website/.storybook/main.ts`。

| 機制 | 去向 |
|---|---|
| `ThemeDecorator`（light／dark、`textScale`） | 改寫成 browser 測試的共用 render helper 參數；只有原 play 有斷言的主題或字級才保留 |
| `IntlDecorator`（`NextIntlClientProvider` + `messagesFor(defaultLocale)`） | 共用 render helper |
| 全域 CSS（`styles/globals.css` 等 4 檔與 `.storybook/preview.css`） | browser project 的 setup 檔匯入；`preview.css` 只留測試需要的規則 |
| `sb.mock` spy：`lib/hooks/useQueryIdentity.ts`、`lib/navigation/document-location.ts`、`lib/navigation/recovery-state.ts`、`lib/analytics/gtag.ts` | 用到的測試檔以 `vi.mock`／`vi.spyOn` 自己宣告 |
| `@storybook/nextjs-vite` 的 Next.js 模擬（`next/navigation`、`next/image`、`next/link`；`nextjs.appDirectory: true`） | 步驟 1 的 spike 決定（`rg -l 'next/navigation\|next/image\|next/link' website/stories` 為 0，但被測元件內部會用） |
| msw service worker（`.storybook/public/mockServiceWorker.js`）與 `.storybook/description-image-fixtures.ts` | 搬到 browser 測試目錄（步驟 3） |
| `parameters.a11y.test: "error"`（每個 story 跑 axe） | 步驟 4 的 Playwright axe 掃描取代 |
| `.storybook/catalog-assets.ts`（Flutter 目錄資產的 Vite plugin） | 刪除（隨 `storybook:apps`） |

## 3. 其他引用（去向）

產生：`rg -il 'storybook|stories/' --hidden -g '!**/node_modules/**' -g '!pnpm-lock.yaml' -g '!website/stories/**' -g '!.git/**'`

| 位置 | 去向 |
|---|---|
| `website/package.json` scripts：`storybook`、`storybook:build`、`storybook:test`、`storybook:typecheck`、`storybook:apps`、`storybook:build:all`、`storybook:mcp` | 刪除（步驟 5）；新增 `test:browser`（步驟 1） |
| `website/package.json` 依賴：`storybook`、`@storybook/addon-a11y`、`@storybook/addon-docs`、`@storybook/addon-mcp`、`@storybook/addon-vitest`、`@storybook/nextjs-vite`（6 個），第 7 個以 knip 判定（候選 `vite` 8.0.3，若只被 Storybook 用） | 刪除（步驟 5） |
| `website/vitest.config.ts` 的 `VITEST_STORYBOOK` 分支 | 換成 browser project（步驟 1），步驟 5 刪 storybook 分支 |
| `website/vitest.storybook.config.ts`、`website/tsconfig.storybook.json`、`website/.storybook/` | 刪除（步驟 5） |
| `website/scripts/build-app-catalogs.mjs`、`website/scripts/storybook-paths.mjs`、`website/scripts/storybook-mcp.mjs`、`website/tests/app-catalog-build.test.mjs`、`website/tests/storybook-paths.test.ts`、`website/.mcp.json` | 刪除（步驟 5） |
| `.github/workflows/website-e2e.yml`：`storybook` job、檔頭第 3 行與第 86 行註解 | 新增 browser job（步驟 1），步驟 5 刪 storybook job 並改註解 |
| `.github/dependabot.yml:33`（`@storybook/nextjs-vite` 規則） | 刪除（步驟 5） |
| `pnpm-workspace.yaml:187` `patchedDependencies` 與 `patches/@storybook__nextjs-vite@10.6.2.patch` | 刪除（步驟 5） |
| `patches/UPSTREAM-WATCH.md:161-173` | 刪除該條目（步驟 5） |
| 文件：`website/AGENTS.md`、`.claude/rules/website.md`、`.claude/rules/flutter-apps.md`、`website/docs/component-library.md`、`website/docs/description-editor-validation.md`、`website/docs/navigation-recovery.md`、`website/docs/skeleton-alignment.md`、`website/design-system/cart-typography.md`、`buyer-app/README.md`、`seller-app/README.md`、`buyer-app/docs/component-library.md`、`seller-app/docs/component-library.md`、`buyer-app/lib/catalog/component_catalog.dart`、`seller-app/lib/main_catalog.dart` | 改寫成指向 browser 測試或 Flutter 目錄的本機執行方式（步驟 5） |
| 歷史紀錄：`backend/docs/plans/2026-09-29-a3-buyer-seller/public-read-client-inventory.md`、`backend/docs/plans/2026-10-18-a7-social/client-inventory.md`、`docs/adr/2026-09-13-procurement-category-fallback.md`、`docs/adr/2026-10-28-agents-md-layering.md` | 保留：已完成計畫與 ADR 是當時的紀錄，不改寫 |
