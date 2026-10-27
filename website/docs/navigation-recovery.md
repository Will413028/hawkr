# 導覽復原與登入離頁

本次修正補上已宣告導覽、但 router 尚未啟動時的復原機制，並讓登入完成後的整頁跳轉接管舊導覽。這是共用導覽的相容性修正，**不代表已證實 2026-09-12 Safari 事件的最初原因，也不代表正式站事故已修復**。

## 行為與期限

點擊共用 `Link` 或呼叫 `useObservedRouter().push/replace` 會宣告導覽意圖，保留目的地與操作來源。300ms 尚未收到 router start 時，只加入 `navigation.start_pending` breadcrumb；不在此時結束意圖，也不把它判為失敗。晚到的 router start 可延續同一筆導覽，再交給既有的進行中 watchdog。

| 目的地 | router 尚未啟動的復原期限 |
| --- | --- |
| 一般路由 | 10 秒 |
| `/`、`/feed`、`/market`、`/market/products`、`/search` | 20 秒 |
| 同 URL、只有 hash 改變、跨來源目的地 | 不啟用此自動復原 |

期限到時，只有仍由目前意圖持有、仍停在原 URL、頁面可見且未離頁的導覽，才可嘗試整頁開啟原目的地。自動復原限同來源 HTTP(S) URL，保留完整 query、hash 與 replace 語意；帶帳號密碼的 URL 不交給文件導覽。已改變 URL 的導覽視為到達或轉址，不強制拉回原目的地。

每一段使用者導覽最多自動復原一次。記憶體旗標與 `sessionStorage` 標記共同防止跨文件循環；新的可信使用者操作才重新開放下一段導覽的復原額度。儲存空間被封鎖、寫入失敗或無法讀回時，不自動重整；原本晚到的 router／資料完成流程仍可繼續，不把無法復原當成取消使用者原本的目的地。

以下事件會清除舊復原計時器：

- 新的可信指標、鍵盤、輸入或獨立輔助科技點擊，以及較新的導航意圖。同一手勢的放開點擊／鍵盤 activation 不會誤取消已啟動的導航。
- 頁面隱藏、`pagehide`、離線、Back／Forward、`popstate` 或 `hashchange`。
- 明確取消 `onNavigate`，或登入／角色切換接管為整頁導覽。

重新連線不自動重送舊導覽。BFCache 的 `pageshow` 不恢復前一次造訪留下的計時器。整頁離頁嘗試若同步拋錯會釋放離頁狀態；若使用者取消瀏覽器的離頁確認，留在可見頁面的下一個可信操作可開始新意圖。

## 共用 Link 與登入整合

`Link` 以實際 anchor 的 `href` 作為目的地，涵蓋 `as`、query、hash 與 `replace`。一般軟導覽、商品攔截路由和等待資料的路徑，統一讓 `onNavigate` 呼叫一次；呼叫 `preventDefault()` 會取消該意圖。下載、開新分頁及修飾鍵點擊保留瀏覽器行為。

等待目的地資料時，完成回呼必須核對當前意圖 ID 與原頁 URL。登入完成或新的程式導覽已接管時，晚到的舊資料不能再觸發 `router.push/replace`。等待中再次點擊同一連結，可建立新的意圖；QueryClient 共用仍在進行的請求，舊回呼不能清除新一次等待狀態。

`useCompleteLogin` 使用同步 ref 防止同一輪 provider 回呼重複完成。它先沿用 `redirectForRole` 套用角色和目的地政策，再以 `navigateDocument` 清除舊復原狀態並離頁。商品目的地與 query/hash 保留；不符合角色的受保護路徑仍使用既有角色預設頁。若文件導覽被拒絕或同步拋錯，登入的 leaving 狀態會解除，讓原有錯誤處理及重試繼續。

## 來源 API

這些是 repository 內部 API，從實際來源匯入；共用 `Link` 沒有新增必填 props。

```ts
import Link from "@/components/shared/Link";
import {
  beginNavigationIntent,
  cancelNavigationIntent,
  isNavigationIntentCurrent,
  navigateDocument,
} from "@/lib/navigation/observability";
```

| API | 契約 |
| --- | --- |
| `beginNavigationIntent({ target, trigger, mode?, componentId?, actionId?, replace?, reason?, recoveryOf? })` | 回傳意圖 ID 或 `null`；較新的意圖取代舊意圖。`trigger` 為 `link`／`router`，`mode` 為 `soft`／`hard`／`intercepted`。 |
| `isNavigationIntentCurrent(id)` | 只有仍屬目前 soft／intercepted 意圖時為 `true`；用於拒絕過期的非同步完成回呼。 |
| `cancelNavigationIntent(id?)` | 指定 ID 時只取消該筆；不帶參數時取消目前意圖。 |
| `navigateDocument(target, { componentId?, actionId?, replace?, reason?, recoveryOf? }?)` | 驗證同來源目的地、清除舊復原並記錄 hard handoff，再呼叫文件導覽。無效目的地回傳 `false`；已發起回傳 `true`，不等於目的地成功載入。同步文件導覽錯誤仍拋給呼叫端。 |

正式期限與取消條件由 [`recovery-state.ts`](../lib/navigation/recovery-state.ts) 管理；意圖、完成與跨文件關聯在 [`observability.ts`](../lib/navigation/observability.ts)，已開始的導覽由 [`navigation-watchdog.ts`](../lib/loading/navigation-watchdog.ts) 接手。登入使用 [`useCompleteLogin.ts`](../lib/auth/useCompleteLogin.ts)。來源仍是行為與 API 的最終依據。

## Stories 與驗證狀態

[`stories/web/NavigationRecovery.stories.tsx`](../stories/web/NavigationRecovery.stories.tsx) 使用正式導覽狀態機與共用 Button，替換文件導覽 I/O、縮短期限並驅動 router 開始／提交訊號。文件導覽只更新隔離預覽，沒有登入交換、真實 API 或交易。正式 Link 與登入 hook 的接線由 React 整合單元測試驗證；Storybook 內建 Link mock 會先取消點擊，不能拿它當正式 Next 路由的執行證據。

| Story export | 審查情境 |
| --- | --- |
| `UnstartedRecovery` | router 未啟動時只復原一次。 |
| `LateRouterStart` | 延遲啟動仍正常到達，不被提早判失敗。 |
| `LoginSupersedesPending` | 登入離頁接管舊等待，舊期限不能覆蓋目的地。 |

預覽將復原等待縮短至 800ms，方便互動審查；正式 10／20 秒期限由 Node 單元測試驗證，不能拿 story 的等待時間當成產品設定。

本次本機驗證：

| 證據層級 | 狀態 |
| --- | --- |
| 本機 React 整合單元測試 | `pnpm test lib/auth/useCompleteLogin.unit.test.ts components/shared/Link.navigation.unit.test.ts`：2 檔、15 項通過。涵蓋登入重入、角色／目的地保留、取消、replace、延遲回呼及重複點擊。 |
| 完整單元測試 | `pnpm test`：220 檔、2022 項全數通過；包含 storage key 分類、共用 Link 與手機底部導覽。 |
| core Node 測試 | observability、watchdog、route contract 三套最新合計 66 項通過。登入角色／redirect、樣式 ownership 與賣家通知 contract 另 17 項通過。 |
| MCP `test-run` | 最後完整一輪 146 項全數通過，包含 3 個新預覽。前一輪商品描述 story 的可見性等待失敗，限定重跑與完整重跑均通過，未修改商品描述程式。 |
| 型別與靜態檢查 | `pnpm storybook:typecheck`、本次 17 檔 Biome、`git diff --check` 通過。應用程式型別檢查排除既有 `output` 測試產物後通過；原始全庫 `tsc` 仍被 `output/playwright` 既有產物的型別錯誤阻擋。 |
| 瀏覽器預覽 | Chromium 桌面與 390px 手機尺寸完成實際預覽、截圖檢查。截圖位於 `output/playwright/navigation-recovery-*.png`；這是隔離狀態機驗證，不是正式登入或 Safari 裝置回歸。 |
| CI、push、部署、正式 Safari 回歸 | 本次尚無完成證據。 |

發布元件文件與部署產品是不同操作；此變更紀錄維持 draft。後續若部署，仍需核對部署版本與真實瀏覽器導覽，再觀察 Sentry 的取消、恢復及到達關聯；本機測試不能替代這些證據。
