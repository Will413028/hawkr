# Hawkr 跨平台視覺契約

這份契約把 **Web、買家 App、賣家 App 現有且相同的設計值**連到實際程式碼，讓日後改動能被檢查。品牌規則入口仍是 [`DESIGN.md`](../DESIGN.md)。[cross-platform-tokens.json](./cross-platform-tokens.json) 是供檢查器與元件庫閱讀的來源對照，**不是第四套 runtime token，也不會產生尚未被產品使用的 CSS／Dart 檔案**。

## 日常檢查

在 Web repository 執行：

```sh
# Web CI 可獨立執行；明確顯示 App 尚未檢查
node scripts/check-cross-platform-tokens.mjs

# 本機同時檢查兩個 App；缺任一個 repository 就失敗
node scripts/check-cross-platform-tokens.mjs --apps-root ..

# 驗證檢查器會擋住真實 drift、失效 alias、缺檔與透明度改動
node --test tests/check-cross-platform-tokens.test.mjs
```

`--apps-root` 指向含有 `hawkr_buyer/` 與 `hawkr_seller/` 的工作區。契約沒有綁定開發者的絕對路徑。預設 Web root 依腳本位置決定，因此可從其他 cwd 呼叫。

## 現在保護什麼

- 15 個品牌色、12 個中性色，以及已一致的淺色表面、文字、主要操作、邊框與狀態角色。
- 4px 間距節奏、foundation 圓角 6／8／12／16／24／32 與 pill。
- 已一致的 12／14／16／18／20／24 字級、正文與強調字重、正文行高。
- 標準／強調 easing，以及 Web UI kit 與 App 共用的 200ms 動效時間。

目前共 **72 個三端共同 token**。兩 App 的 3 個溫層標籤色票另列為沿用 Web 舊外觀的追蹤例外。各平台 binding 都有來源檔案、CSS 變數或 Dart `Class.member`；TextStyle 再指向 `fontSize`／`fontWeight`／`height`。checker 會解析 alias，避免只檢查色票卻漏掉已改色的 semantic role。

| Runtime 來源 | 責任 |
| --- | --- |
| `styles/foundation/00-color-config.css` | Web 色票與淺／深色語意 |
| `styles/foundation/01-tokens-theme.css` | Web foundation 字級、間距、圓角與動效 |
| `styles/components/ui-kit.css` | Web 共享元件的 UI token |
| 兩個 App 的 `lib/core/theme/app_colors.dart` | Flutter 色票與語意 alias |
| 兩個 App 的 `lib/core/theme/hk_tokens.dart` | Flutter 間距、圓角、控制項與動效 |
| 兩個 App 的 `lib/core/theme/app_typography.dart` | Flutter 文字角色 |

## 已確認、尚未統一的差異

以下是程式碼現況，不把相近值當成已一致。JSON 的 `exceptions` 會檢查已記錄的值；若值改了卻沒更新說明，檢查仍會失敗。

| 角色 | Web | 買家 App | 賣家 App | 收斂方式 |
| --- | --- | --- | --- | --- |
| 區塊標題 | 依 UI／foundation 角色 | `sectionTitle` 18 | `sectionTitle` 16 | 用窄版與較大文字比較；決定共同角色或明確的密度 variant |
| App 字體 | Roboto Latin＋系統繁中；品牌標題另用圓體 | `family = null`，承接平台字體 | `family = 'Roboto'` | 保留字形與平台可讀性；不能只改 family 就宣稱外觀一致 |
| `textSecondary` alias | 對應文字角色分開定義 | `#72796e` | `#42493f`，原碼註明 AA 需求 | 新元件使用明確 `textPrimary`／`textMuted`；先驗證對比再改相容 alias |
| 收藏啟用色 | `#ff0000` | `#ff3b4f` | `#ff3b4f` | 比較淺／深底的實心愛心，選定社群色；保持與 danger 分開 |
| 基本卡片圓角 | 共享 `Card` 預設 14 | Material `CardTheme` 16 | Material `CardTheme` 16 | 比較相同卡片責任；買家 `ProductCard` 另有 compact 12／wide 16 |
| 對話框圓角 | `Dialog` 28 | `HkRadius.dialog` 24 | `HkRadius.dialog` 24 | 分別確認 modal 與 sheet 幾何，直接收斂既有 primitive |

卡片與 Dialog 的 runtime 使用位置分別為 `components/ui/Card.tsx`、`components/ui/Dialog.tsx`，以及兩個 App 的 `lib/core/theme/app_theme.dart`。`radius.componentCard`／`radius.componentDialog` 檢查的是這些來源使用的 token；checker 不解析元件 props 是否仍選用該 token，這部分由元件 stories 與 UI 測試檢查。

一般輸入欄位以兩個 App 的 `AppTheme.inputDecorationTheme` 為外觀參照：Web `Input`、`Textarea`、`Select` 預設為 bordered，md 採 16px 文字、16px 水平內距、12px 圓角與奶油表面，單行最小高度 48px；正常邊框 1px、聚焦 1.5px、錯誤且聚焦 2px，停用時保留表面與可讀文字。`floatingLabel` 可在空白未聚焦時置於欄內，聚焦或填入後縮至 12px 並停留在上邊框。Web 明確指定的 flat／faded／underlined 保留原有外觀，sm 精簡欄位保留滑鼠 32px／觸控至少 44px。這是共用欄位的收斂，不代表全部輸入表面或全域 control token 已統一：買家填色為不透明 `#fffdf7`、hint 為 `#72796e`；賣家填色為同色 245/255 透明度、hint 為 `#42493f`。Web 錯誤文字維持較深的 `danger-text` 可讀性角色，且沿用自己的深色 theme 與原生操作語意；App 專用手機欄位、dropdown 及平台字體仍需分別比較。

另有三種不能做數值等號的差異：

1. **觸控尺寸**：App `HkControl.minTap = 48`；Web `Button` 目前 md 為 44、lg 為 48，但 UI kit 的 `--hkui-control-h-md` 仍是 40。新觸控主要操作先使用現有 48 尺寸。檢查 token 不等於量到實際 hit area。
2. **深色模式**：Web 有 theme overrides，App `AppColors` 是靜態常數，沒有對等的 mode source。共同淺色色票不代表深色模式相同。
3. **陰影與字體縮放**：Flutter `blurRadius` 與 CSS blur 的渲染 API 不同；Web foundation／UI kit 也仍有兩套陰影。字級以 16px Web root 與未縮放 Flutter logical px／sp 比對，並不關閉或驗證使用者字體縮放。

目前 `DESIGN.md` 部分元件現況段落落後於程式：例如 Button 文件寫 md 40，但 `Button.tsx` 已是 44；Card 文件記錄鍵盤啟動缺口，但 `Card.tsx` 已有 Enter／Space 處理。依品牌入口所訂的 source authority，元件庫應呈現實際 production 元件，不能複製過期描述當成驗收結果。

## 溫層標籤對齊 Web（2026-09-12）

買家與商家 App 的顯示型溫層標籤統一沿用 Web 正式 [`TemperatureTag`](../features/cart/components/TemperatureTag.tsx) 的樣式，透過各端 `HkTemperatureTag(value:)` 提供；不再保留各端原本的 12／14px、大框線或不同溫層色票。

| 顯示規則 | 三端基準 |
| --- | --- |
| 常溫／3201 | `#6b6b6b` |
| 冷藏／3202 | `#ee964b` |
| 冷凍／3203 | `#51a2ff` |
| 標籤 | 透明底、同色 1px 外框、完整膠囊、11px／700 文字、左右內距 8px |
| 排版 | 基準高度 18px，多標籤間距 4px，可換列，標籤內沒有圖示 |
| 未知資料 | 保留後端原文並採灰色，不推測成常溫；未提供的溫層不能補造 |

Web Storybook 的正式商品列已量測三種兩字標籤皆為 40×18 CSS px，文字行高 16.5px。Flutter 保留平台字型與使用者文字縮放，標籤高度可隨文字增長；不能以固定高度、FittedBox 或關閉縮放換取像素相同。顯示型標籤不新增按鈕語意；溫層選擇器維持原有選取回呼、狀態及至少 48px 點擊範圍。

Web 實際顏色位於 TSX 的 Tailwind class，與 `DESIGN.md` 的溫層 semantic roles 及 `--temp-*` foundation aliases 不同。本次依明確指定的 Web 外觀對齊，`cross-platform-tokens.json` 將三個 `temperatureTag.*` binding 列為 `requested-web-legacy-parity` 例外，僅檢查兩 App 的現況，不提升為 canonical 色票。Web 對照依正式元件與渲染量測，不能宣稱現行 CSS／Dart checker 已解析 TSX。

已知可讀性限制：在 App cream `#fffdf7` 上，冷藏／冷凍文字的對比約為 2.27:1／2.59:1；11px 文字未達 [WCAG 一般文字 4.5:1](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html) 的要求。這是本次沿用 Web 外觀的設計差異，不能以元件庫 a11y 測試通過宣稱已解決。未來調整須同時更新 Web 與兩 App 的溫層前景／底色並重新檢查對比；不可把這組舊色擴散到其他元件。

## 購物車操作與文字層級

購物車採用既有平台控制項層級：Web 次要操作至少 44px、主要操作 48px；買家 App 沿用 `HkControl.minTap = 48`。正文與輸入使用 16px、輔助資訊 14px；溫層標籤獨立於正文尺寸規則，本次樣式依上節的 Web 對齊與追蹤例外。Web 文字旁圖示 20px／獨立圖示 24px，App 使用 24px。窄版與大字模式將商品資訊、配送費與付款金額換行，保留完整文字與操作空間；App 的 `CartCheckoutAction` 可點擊或上滑到原結帳確認頁。這些是購物車使用端的尺寸收斂，沒有修改共享 Button 或全域 control token，驗證入口為 `Web/Cart/Cart` 與買家 `component=cart` 真實 widget 預覽。 字體層級參考 Material 3 與 Apple 的語意角色及縮放原則，使用局部22/28頁面與區塊標題、16/24正文與數量、14/20輔助、24/32總額；完整對應、字級設定與Web對齊溫層例外見 [購物車字體層級](./cart-typography.md)。

購物車選取範圍為一家店。Web頁頭只保留全車清空，保留店內全選，刪除操作只放在各商品旁，其他店可透過「改選本店商品」取代目前勾選；App原有單店controller同樣明示改選與本店可購買商品範圍。改選不移除購物車商品，取消勾選與刪除最後已選項後不自動改選其他店。訂單摘要／結帳入口標示店名，物流與商品更新中不使用前店驗證結果。溫層顯示沿用上節共用元件；Web不可購買項目不列入金額與結帳草稿，仍可從商品列逐項移除。

購物車店頭採「店名｜賣家商店」同列排列，地點置於次要資訊列。手機數量控制放在商品縮圖旁；縮圖依可用寬度調整，保留 Web 44px／App 48px 觸控尺寸，長品名、規格及原溫層完整換行。

## 新增或調整元件時

1. 從 Web `components/ui/*`／`components/shared/*` 或 App 既有 shared widgets 開始，替真實元件補上 default、loading、disabled、error、selected 等適用狀態。
2. 先找相同的 semantic role，再選 token。不要把 primary 與 primary-strong、danger 與 favorite、foundation 與 UI component radius 當成相同意思。
3. 改共同 token 時，同步修改實際 runtime 來源與 JSON；跑三端檢查、既有元件測試，再於元件庫確認窄寬版、文字縮放與互動狀態。只更新 JSON 不會改產品外觀。
4. 真正的平台例外需寫明原因、來源及收斂條件；先沿用已有例外，不另造相近色或圓角。收斂完成後把 binding 移回共同 token，移除已過時的 exception。

## 檢查範圍

checker 讀取上述明列檔案的頂層 `:root` 與 Dart 常數／TextStyle getter，忽略 dark、media 與 route selector。未知語法、失效 alias、缺檔和非預期透明度都會失敗，避免無法解析時靜默通過。它不執行 Dart、不計算完整 CSS cascade、不掃描全部畫面的 token 採用率，也不證明視覺回歸、對比、字形 fallback 或線上部署。這些仍需對真實元件與產品畫面驗證。
