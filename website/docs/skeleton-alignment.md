# 骨架載入的響應式對齊

2026-09-13，本機 Web 修正；不代表 CI、DEV／PROD 部署或兩 App 驗證。

骨架以正式元件的容器、斷點、圖片比例、文字行盒及控制項位置為準。固定資料的 Storybook 預覽不連真實交易。

## 範圍

| 畫面 | 修正 |
| --- | --- |
| 市集、搜尋商品結果 | 保留商品卡 1px 邊界空間，圖片內距 12／6px、正文內距 16／8px；跟隨實際欄數與 4:3 圖片比例 |
| 市集整頁 | 960px 切換側欄；小於 768px 沿用 680px 容器上限與 14px 邊距；工具列和故事列與正式來源對照 |
| 動態 | 整頁沿用正式 StoryRail 及其收合偏好；局部 recovery 僅替換資料網格，不重複故事列及頁面邊距 |
| 商品詳情與彈窗 | 單一 1440px 容器；1024px 切為雙欄；24／16px 外側內距、40／28／20px 卡面內距；依實際 glass-card 的 1px 邊框，主圖與縮圖均 4:3 |
| 首頁、搜尋入口、商店、結帳、登入 | 各自對照正式版型；搜尋入口顯示推薦食譜貨架；商品查詢結果使用局部 ProductGridSkeleton |
| 賣家中心 | 表格資料欄／手機 data-label／勾選欄／代行權限；132×99 貼文封面；58px 手機折疊列；發票狀態、表單、260px 分析圖表及行情雷達列 |
| 局部載入 | 商品評價及商家統計、首頁搜尋與 Agent 入口、Banner、優惠券、買家發票與申訴、聊天抽屜、MegaMenu、通用 Drawer |

共用 skeleton 動畫仍使用原品牌色與 reduced-motion 設定。所有 shimmer 元素限制在實際容器內；正式 Banner 保留原圖自然比例。

## API 與使用方式

```tsx
import RouteLoadingSkeleton, {
  FeedGridSkeleton,
  ProductGridSkeleton,
} from '@/components/shared/RouteLoadingSkeleton';

// 已有標頭、故事列及容器的動態資料槽
<FeedGridSkeleton />
// 已有篩選及排序的商品資料槽
<ProductGridSkeleton variant="market" />
// 搜尋頁已有待載入播報時避免重複 live region
<ProductGridSkeleton variant="search" announce={false} />
// 整頁 fallback
<RouteLoadingSkeleton page variant="product-detail" />
```

`RouteLoadingSkeleton` 的 `page`、`scope`、`variant`、`columns` API 保留。`columns` 只控制預留卡片數，不指定 CSS 欄數。`FeedGridSkeleton` 是新增的資料區元件；`RouteLoadingVariants` 與 `RouteStorySkeleton` 為內部版型，使用端仍由公開入口選擇。

賣家元件由 `@/features/seller-center/components/SellerSkeletons`／`SellerPageSkeletons` 匯入。新增的 `labels`、`delegated`、`canPublish`、`chartKind`、`tableKind` 與 `variant="panel"` 需與既有表格、權限及最終內容一致；不可把通用卡片當作發票表單。`SellerMobileDisclosureSkeleton`、`SellerInvoiceOverviewSkeleton` 對應正式手機折疊列及發票總覽。

新增 API 屬向後相容 minor 意圖；幾何修正屬 patch。版本記錄保留 draft，不發布元件庫或產品。

## 本機驗證

- Storybook MCP 全量執行：192 則 story assertions 通過。七則 story 的既有對比不足仍被 a11y 偵測：賣家發票表頭 4.22:1、分析圖表文字 4.15／4.49:1、AssistantEntryLink 副文案 3.74:1；不能宣稱全量 a11y 通過。
- `pnpm storybook:typecheck` 通過；變更的 TS／TSX／JSON 通過 Biome。
- 額外 architecture 檢查被既有缺少 `components/ui/Badge.tsx` 中止（非本次刪除）；未把這項檢查記為通過。
- 路由骨架：13 種預覽 × 21 個寬度（320–2560px，共 273 案），檢查水平溢出、文字截出容器、4:3 圖片／縮圖比例。另測深色、1.4 倍字級、DPR 2 與 reduced motion 共 50 案，未見溢出或殘留動畫。
- 正式 Next 頁面對照：市集／商品詳情／動態，在 19 個寬度共 57 案比較第一列圖片相對頁面起點的 x／y／width／height，全部誤差不超過 0.1 CSS px；包含 768px／840px 的 strict/inclusive 邊界。
- 賣家：112 個寬度／主題／字級案，手機折疊列與正式 fixture 均為 58px；9 則 seller、3 則 Drawer assertions 通過。
- 賣家分析追加：45 個版型案及 9 個最終表格案通過，歷史／預測圖表與正式260px圖面匹配；雷達列按斷點使用76／104／132px。5 則功能斷言通過，保留既有文字對比警告。
- 首頁／搜尋／商店／結帳／登入／通用：66 個寬度案與 6 則 MCP assertions 通過。
- 局部載入：5 個寬度共 55 案無溢出，包含已知查詢文字的 Agent 入口；14 則相關 stories 功能斷言與 12 個 unit tests 通過，Agent 副文案仍有既有對比警告。
- 斷點精度追加：18 個邊界斷言（640／720／768／840／1024／1080／1280px）通過。
- 受影響導覽／商品元件 unit tests：39 項通過；既有載入及樣式歸屬 Node contract tests：22 項通過。

可重現的瀏覽器腳本、座標 JSON、截圖與各範圍報告保留於 `output/playwright/skeleton-alignment/`。本機 Storybook 使用 6006，正式 Next 開發頁使用 3100；正式頁對照為唯讀瀏覽，不下單、不變更後端資料。

## 精度界線

已知的容器位置、第一列圖片、斷點、文字起點與固定操作區可以精確量測。尚未取得的文字長度、商品及圖片數量、Banner 自然比例、選填標籤／商店描述、登入會員才有的首頁搜尋，會影響內容到齊後的高度；通用 Drawer 也依最終內容而異。沒有資料時不以猜測的固定高度聲稱整頁逐像素一致。

Storybook 與本機瀏覽器證據不是已登入賣家真實資料、實機 Safari／Android、遠端 CI 或已部署版本的證據。
