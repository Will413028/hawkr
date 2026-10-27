# 三端元件庫交付與驗證

本機入口：<http://127.0.0.1:6006/?path=/story/cross-platform-comparison--overview>

範圍為買家 App、賣家 App、主網站含賣家中心。Storybook 10.6.0 直接渲染正式 Web components；兩 App 以各自 `lib/main_catalog.dart` 編譯真實 Flutter widget，再嵌入同一目錄。MCP `docs-list`／`docs-show` 同時提供 Web props 和同步的 Flutter API 文件。

## 本次 UI 更新

- 採購分類重查：`crop_disambiguation` 在後端 `allowFreeText=true` 時，同時顯示分類選項與重新描述欄位。`onSubmitFreeText(text)` 只送文字重查；`onSubmitCropOther({ description, priceAmount })` 保留自填價語意。只有 `crop-other` 且 `allowFreeText=false` 才直接顯示自填價表單。大類、小類與自填價即使共用 `slotKey`，題目內容改變也會清除上一題草稿；單純 pending 更新保留草稿。新增相容行為，SemVer 記為 minor 意圖；分類內容、查找順序與行情可用性由後端決定。
- 採購自訂品項：`ClarifyQuestionCard` 使用正式 `Input`、`Textarea` 與 `Button`，整理描述、字數與單價單位。`crop_disambiguation` 僅含 `crop-other` 時直接顯示表單；有其他候選時保留原選項與展開操作。公開 props 不變，`onSubmitCropOther({ description, priceAmount })` 仍提交 trim 後字串，兩欄非空、非 pending 且有 callback 才能送出；不新增價格格式契約。`ProcurementClarifyQuestion.stories.tsx` 以固定資料展示，沒有真實交易或 API 請求。
- 共用欄位：`Input`、`Textarea`、`Select` 預設改為 App 對齊的描邊外觀，包含尺寸、浮動標籤、焦點、錯誤與停用狀態；`Input.stories.tsx` 提供 App 外觀、大字級及保留變體。完整差異見 [跨平台契約](../design-system/cross-platform.md)。預設外觀與尺寸改變依維護規範記為 major 意圖；需維持舊畫面的使用端應明確指定變體，並檢查固定高度與密集布局。
- 貼文輸入：高亮文字與 textarea 使用相同排版和捲動範圍，hashtag 不再因字重不同而偏離游標；`PostContent.stories.tsx` 覆蓋原畫面、浮動標籤、輸入選取、長文捲動、深色及窄版大字級，記為相容修正。
- 商品描述：分組工具列、格式整理、閱讀預覽、放大編輯、可靠的刪除與復原、文字色盤，以及沒有說明欄的圖片上傳、重試與替換；詳細操作及本機驗證見 [商品描述編輯器驗證](./description-editor-validation.md)。

## 2026-09-12 採購自訂品項卡本機驗證

本次以既有問答卡、`DESIGN.md` 的 Operate 欄位規範和使用者截圖為基準，保留奶油卡面、品牌識別與提交資料語意。桌面單價與確認按鈕底部對齊，窄版改為上下排列；320px／1.4 倍字級改為身份標記與卡片上下排列，未見橫向溢出。畫面紀錄為 `output/playwright/custom-item-{desktop,mobile,dark,narrow}-final.png`。

MCP `test-run` 的 7 個新增案例（含 a11y）、既有問答卡 34 項 Node 檢查、interrupt 17 項單元測試與此次 TSX／JSON 的 Biome 檢查通過。CSS 無錯誤，仍有 31 項既存 lint 警告。最後一次 `pnpm storybook:typecheck` 受工作區其他修改 `lib/navigation/recovery-state.ts:128` 的 TS2774 阻擋；該檔不屬於本次 UI 範圍。以上是本機元件驗證，未執行 CI 或部署。

## 2026-09-13 採購分類逐層確認本機驗證

`openConfirmation.question` 是重新整理與 snapshot 校正時的權威問句；前端保留後端的 opaque optionId、allowFreeText 與選項，不從分類階段或 categoryId 自行推導。有效問句存在時抑制 legacy 分類卡。相容舊版 snapshot：同一個仍開啟的中斷沒有 question 欄位時，校正保留現有題目；不同中斷不沿用上一題。

本機 MCP `test-run` 的 12 個採購問答 stories（含互動與 a11y）、4 個 session hook 測試檔共 41 項、問答與 session wiring 等 70 項 Node 檢查，以及 `pnpm storybook:typecheck` 通過。完整 MCP 測試首次遇到開發伺服器中斷，重啟後透過 `pnpm storybook:mcp test-run '{}'` 完整執行，151 個 stories 全部通過。CSS 檢查無錯誤，仍有 31 項既有警告。實際檢視桌面與 320px／深色／1.4 倍字級渲染；窄卡片的分類選項改為單欄，避免長選項逐字直排。這些都是固定 fixture 與本機驗證，沒有連線至真實交易或部署。

## 初次建立元件庫的交付範圍

- 85 個預覽案例：17 個正式 Web 元件、品牌基礎、兩 App 各七類元件與小螢幕案例，以及四類三端對照。
- `hawkr-storybook` MCP 已登錄到本機 Codex，endpoint `http://127.0.0.1:6006/mcp`。其他 MCP client 可參照 repository `.mcp.json`。已開啟的 agent session 若未重新載入工具，使用 `pnpm storybook:mcp` 呼叫同一端點。
- 72 個共同 token、248 項來源檢查；[跨平台契約](../design-system/cross-platform.md) 明列六項既有差異。這是目前共用基礎元件與代表性交易元件的目錄，完整 feature 畫面尚未全部收錄。
- 兩個 App 的原 ThemeData 原樣抽出為 `AppTheme.light`，正式 App 和 catalog 共用。
- 實際修正：Web 表單提示、錯誤、未選取 tabs、危險按鈕、警告 chips 的文字對比；日期輸入的 ARIA；賣家 App 加減 SVG 顯示。原始品牌色票保持一致。
- 新增 Web component-library CI workflow；App catalog tests 置於各自既有 `test/`，可由完整 Flutter test suite 發現。

## 2026-09-12 初次建立元件庫的本機驗證

| 檢查 | 結果 |
| --- | --- |
| 官方 MCP `test-run` | 65 個 Web／基礎案例通過，包含 a11y；5 個明確深色案例斷言實際 dark theme |
| Flutter catalog tests | Buyer 48、Seller 27 通過；含 320／390／768px 和 1.4 倍字級 |
| Flutter 回歸 | Buyer 共87個相關 tests；Seller 14個既有設計政策與2個round-action tests 通過 |
| 真實瀏覽器 | 兩 App 按鈕、必填表單恢復、數量／溫層／物流互動通過；小螢幕與大字級未見 overflow |
| Web 既有 unit project | 206 files、1,926 tests 通過 |
| Button/Input/Dialog 等 Node 回歸 | 40 tests 通過 |
| Token checker 自測 | 8 tests 通過；三端 gate 248 項來源檢查通過 |
| Scoped TypeScript、Biome、theme-token gate | 通過 |
| `pnpm storybook:build:all` | 成功，兩 App 的 JS Web build 加上 Storybook static output |

App／三端 iframe stories 標記 `!test`，共20個，不計入65個 React story tests。它們以 Flutter tests 與實際瀏覽器驗證，不把 iframe 容器測試當成原生 widget 驗證。

上表是首次本機驗證紀錄；當時尚未執行遠端 CI 或部署。完整網站 `tsc --noEmit` 原設定會掃到本機 `output/playwright` 歷史診斷檔，含已不存在的臨時路徑；本次使用獨立 `tsconfig.storybook.json` 檢查元件庫及其 imports，未改寫或刪除歷史檔案。

## 重現與維護

商品描述編輯器後續修正、色盤操作與最新測試範圍，見 [商品描述編輯器驗證](./description-editor-validation.md)。

```sh
pnpm install --frozen-lockfile
pnpm exec playwright install chromium
# 預設讀 monorepo 的 buyer-app、seller-app；其他位置設定 HAWKR_APPS_ROOT
pnpm storybook:apps
pnpm storybook
```

另開終端執行：

```sh
pnpm storybook:mcp docs-list
pnpm storybook:mcp get-storybook-story-instructions
pnpm storybook:mcp test-run
pnpm storybook:typecheck
pnpm design:check --apps-root ..
```

靜態輸出在 `output/storybook`，含已建置的 App。MCP 由 Storybook 開發伺服器提供；靜態網站本身沒有 MCP endpoint。`pnpm storybook:test` 是 CI 的同組瀏覽器測試入口。原有 `pnpm test` 維持單元測試，不載入 Storybook 的 Next 環境設定。

畫面紀錄在 `output/playwright/component-library-comparison-final.png` 和 `output/playwright/catalog-*.png`。App API 文件就是兩支 App 的 `docs/component-library.md`，`stories/apps/*Api.mdx` 在 build 時直接讀入，不需同步。

## 2026-09-13 骨架對齊

各頁響應式骨架、API、量測與已知限制見 [骨架對齊驗證](./skeleton-alignment.md)。本次是 Web 來源與本機 Storybook 修正，不代表元件庫或產品已發布。
