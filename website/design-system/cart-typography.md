# 買家購物車字體層級

官網與買家 App 的購物車採用少量語意角色，讓買家先看見區塊與應付金額，再閱讀品名、數量、配送及輔助資訊。保留 Hawkr 字體與品牌色，不改用另一套字型，也不建立新的字級偏好。

依據 [Google Material 3 官方字體表](https://developer.android.com/develop/ui/compose/designsystems/material3#typography)，選用 titleLarge 22/28、titleMedium/bodyLarge 16/24、bodyMedium 14/20、headlineSmall 24/32。購物車頁面與區塊標題統一採 22/28，數量使用 16/24；依使用者指定，只保留22、16、14、24四種文字尺寸。字重依繁中購物車的閱讀密度調整為正文400、需要強調600；這是 Hawkr 的產品適配，不宣稱逐點等同所有 Material 元件。

[Apple Typography](https://developer.apple.com/design/human-interface-guidelines/typography) 強調依用途選字體角色、尊重 Dynamic Type 與較大文字下的閱讀和操作。iOS 預設 body 為17pt，與這裡Flutter/Web預設16邏輯像素不同；本次保留既有跨平台 AppTypography 與字級政策，不把CSS px、Flutter logical px和iOS pt當成同一單位。

| 角色 | 預設字級／行距 | 字重 | 使用位置 |
| --- | --- | --- | --- |
| 頁面標題 | 22／28 | 600 | Web購物車、空車、登入狀態與App購物車導覽列 |
| 區塊標題 | 22／28 | 600 | 訂單摘要、配送、地址／門市抽屜、優惠券 |
| 品名／小標題 | 16／24 | 600 | 商品名稱、運送方式名稱 |
| 正文 | 16／24 | 400 | 說明、地址、付款項目、輸入內容 |
| 輔助資訊 | 14／20 | 400 | 規格、收件資訊、到貨提示、張數 |
| 一般金額 | 16／24 | 600 | 單價、商品小計、運費 |
| 應付金額 | 24／32 | 600 | 最終總額 |
| 按鈕 | 16／24 | 600 | 主要與次要操作 |
| 數量 | 16／24 | 600 | 數量欄位；App以QuantityStepper.countTextStyle傳入購物車角色，其他頁預設不變 |

Web金額與數量使用等寬數字功能，減少更新時的寬度變化；App沿用原有平台字型與數量步進器，僅在購物車傳入16px數字樣式。Web 頁面及區塊標題保留品牌圓體；正文、資料、按鈕保留 Roboto Latin＋平台繁中文字型。App 從原 AppTypography 派生局部 CartTypography，保留平台字型。窄版保持原有換行佈局；中文不加寬字距。

## 字級設定與例外

- Web 沿用 `lib/theme/text-scale.ts` 的小／中／大（0.875／1／1.125），以既有 root rem 縮放；Storybook另提供1.4倍壓力預覽。這些是現有產品偏好，不是Apple Dynamic Type級距。
- App 沿用 `AppTextSize`、既有儲存值及 `AppMediaQueryPolicy`，不另加狀態或改變全域縮放上下限；原生系統縮放仍受現行App政策約束，本次不宣稱完整覆蓋所有iOS accessibility字級。
- 字級調整不縮小命中區：Web次要操作至少44px、主要48px；App至少48px。較大文字可讓控制項增高或換行。
- **三端溫層標籤共用 Web 外觀**：Web 與兩 App 統一灰／橘／藍文字、同色 1px 框線、透明圓角膠囊及 11px 預設字級；App 使用 `HkTemperatureTag(value:)`，保留文字縮放與選擇控制的 48px 命中區。Web 預設高度 18px，App 隨文字縮放增高；標籤不套用購物車正文／輔助資訊角色。依使用者要求沿用的低對比橘／藍色票，已列為 [跨平台契約](./cross-platform.md) 的 `requested-web-legacy-parity` 例外，需三端一起調整。Cart stories 僅對溫層標籤排除 color-contrast 規則，其他規則與文字仍檢查；這不代表 App 文字對比通過。

Web角色定義在 `styles/features/cart-typography.css`，由正式購物車及抽屜載入；22px是本表明確選用的局部角色，不改foundation的共同token契約。預覽入口為Storybook `Web/Cart/Cart`；App為真實Flutter `component=cart`。本地測試、CI、部署及真實交易證據各自記錄。

## 本地驗證備註

Web實際購物車使用隔離API資料，在1440／390／320px與原小／中／大偏好共9組檢查，商品列和推薦卡無水平溢出，按鈕至少44px；桌面1.4倍長品名與規格分行後不再互相擠壓。App驗證正式AppTextSize切換及還原、320／390px與系統1.0／1.4倍，並檢視真實Flutter Web畫面。

字型來源、載入及fallback設定沒有改動；本次只使用現有字型，未增加字型下載。最後type掃描保留兩個已說明提示：溫層標籤11px依Web對齊契約保留、優惠券勾選符號12px屬圖示。售完徽章目前在品名下方使用14px輔助角色。22px局部標題角色已在上表明列。
