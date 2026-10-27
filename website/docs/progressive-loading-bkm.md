# 漸進式載入 BKM

更新：2026-09-07。適用於 Hawkr 公開瀏覽頁與商品相簿。

目標是先讓使用者閱讀文字、操作導覽與搜尋，再逐步取得圖片；圖片抵達時保留原本的版面位置。這是漸進式載入（progressive loading），由資料、渲染與資源排程共同完成。

## 實作原則

| 層次 | 採用的做法 | 驗收重點 |
| --- | --- | --- |
| 換頁回饋 | 保留主分頁即時目的頁骨架；資料完成就呈現，沒有額外最短等待時間 | 載入中仍可切換其他分頁，最後一次操作生效 |
| 文字與操作 | 公開頁保留伺服器輸出的內容，不以圖片 `onLoad`、`decode()` 或 `window.load` 決定整頁可見性 | 圖片回應被延遲時，標題、商品文字與控制項仍可使用 |
| 首屏圖片 | 頁面 owner 指定少量首屏圖片 eager；第一張重要圖片可提高下載優先順序 | 不把 LCP 圖片設為 lazy，也不讓整個清單都競爭高優先順序 |
| 後續圖片 | 下方卡片、後續分頁與隱藏輪播使用瀏覽器原生 `loading="lazy"` | 遠處圖片可以延後，接近畫面時正常載入 |
| 圖片解碼 | 沿用 Next/Image 的非同步解碼，不額外等待所有圖片完成 | 圖片解碼不成為顯示卡片文字的前置條件 |
| 穩定版面 | 保留既有寬高、比例容器與圖片背景 | 圖片抵達前後，卡片文字與控制項位置一致 |
| 適量圖片 | 商品相簿縮圖使用既有媒體 gateway 的 4:3 card preset；GIF 經 `postCardPresetForMedia` 保留 motion | 縮圖不再主動要求整張原始主圖，商品主圖仍保留原圖 |
| 字型 | 保留現有 `font-display: optional`、字型分片與 fallback metrics | 不新增等待字型完成的顯示閘門，也不預載全部字型 |

`fetchPriority="low"` 只調整已排入佇列的請求優先順序，不等於延後請求。原生 lazy loading 的提前下載距離由瀏覽器及連線狀態決定；畫面外的圖片仍可能預先載入，因此不以「每一張畫面外圖片都沒有請求」作為通過條件。

## Hawkr 的責任邊界

- 首屏數量由首頁、動態、市集、搜尋等頁面 owner 決定，不在 `AppImage` 全域猜測。
- 公開動態由 `FeedClientShell` 明確啟用 `InfiniteFeed.progressiveImages`；商家貼文頁維持原本的 eager 排程。
- 搜尋與清單追加後的圖片沿用所在位置排程；排序期間保留原本可用的卡片與搜尋草稿。
- 商品相簿維持既有滑動軌道與 active index。切換到的主圖提高優先順序，其餘主圖延後；縮圖使用獨立衍生圖，避免縮圖提前下載同一份原圖。
- 相簿縮圖保留 4:3 構圖，採既有 card 衍生圖；不套用會將圖片裁成正方形的 thumb preset。
- `AppImage` 的 URL、簽名參數、錯誤替代內容、GIF 行為與業務資料來源維持既有契約。
- gateway 可能回退到原圖。URL 含有 preset 名稱或 HTTP 200 本身不能證明節省多少位元組；要以實際回應及傳輸量判定。

## 資料分區與串流的使用條件

獨立資料適合平行請求；有依賴的資料必須保留順序。慢速、非必要的區塊可以有自己的 Suspense 或查詢狀態，但失敗不能偽裝成有效的商品數量、價格或空清單。

目前公開頁已先取得 JSON 再輸出文字，沒有等待圖片下載。首頁的必要 bootstrap 失敗語意保護 ISR 的最後有效快取；市集共用商品請求已有去重。不要為了套用通用範例而移除這些契約。

本次沒有啟用新的 HTML 串流或 PPR。專案使用 Next.js 16.3.4 與 OpenNext 的既有 ISR 路徑；已檢查的 Next 靜態產生路徑會收集完整 HTML stream。`next dev` 中看到 Suspense 分段輸出，不能直接當作部署後會提早送出 HTML 的證據。未來拆分伺服器區塊時，須另驗證正式建置、快取命中及失效重建時的實際回應。

## 驗證方式

1. 使用手機 viewport，延遲真實 raster 圖片的回應，讓 HTML、JavaScript、CSS、字型與品牌 SVG 正常載入。
2. 確認文字與導覽先出現、載入遮罩釋放、搜尋可輸入，且沒有頁面錯誤。
3. 釋放圖片並記錄幾何位置與 layout shift；向下捲動確認延後的圖片可以載入。
4. 檢查實際 DOM 與請求：首屏優先、後續 lazy、相簿切換提升主圖優先順序，縮圖使用適合的 preset。
5. 保留搜尋排序、清單追加、圖片失敗、GIF 與相簿操作的相關測試。
6. DEV 以成功的 CI／Cloudflare 部署、HTTP 回應中的精確 `data-dpl-id` 和線上瀏覽器檢查共同確認。人工延遲測試驗證載入獨立性，不等於真實用戶速度、FPS 或 Core Web Vitals 的改善百分比。

本機瀏覽器檢查與發佈證據存放於 `output/playwright/progressive-content/`。這是執行產物，不是網站需要下載的資源。

## 官方參考

- [Next.js Image](https://nextjs.org/docs/app/api-reference/components/image)：圖片載入、解碼與尺寸 API。
- [Google：瀏覽器原生圖片延遲載入](https://web.dev/articles/browser-level-image-lazy-loading)：首屏例外、預先下載距離與尺寸保留。
- [Google：改善 LCP](https://web.dev/articles/optimize-lcp)：讓重要圖片可提早發現，減少其他資源競爭。
- [Google：Fetch Priority](https://web.dev/articles/fetch-priority)：下載優先順序與 lazy loading 的不同責任。
- [Next.js：Fetching Data](https://nextjs.org/docs/app/getting-started/fetching-data)：平行請求與分區 Suspense。
- [Vercel React Best Practices：Suspense boundaries](https://github.com/vercel-labs/agent-skills/blob/main/skills/react-best-practices/rules/async-suspense-boundaries.md)：依 UI 邊界安排慢速資料，套用前須確認部署環境的行為。
