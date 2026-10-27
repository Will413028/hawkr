# 賣家 App 元件庫

`lib/main_catalog.dart` 是獨立的 Flutter 入口，直接使用正式元件與
`AppTheme.light`，不初始化 Firebase、登入、推播、API 或分析服務。
App 與目錄共用 `lib/core/theme/app_theme.dart`，原本 `app.dart` 的
ThemeData 僅抽出為共用來源，沒有改變品牌色或元件預設值。

## 啟動與建置

在 `hawkr_seller` 執行，使用 `.fvmrc` 鎖定的 Flutter 3.41.2：

```sh
fvm flutter pub get
fvm flutter run -d chrome --target lib/main_catalog.dart --web-port 7352
fvm flutter test --no-pub test/catalog/component_catalog_test.dart
fvm flutter analyze --no-pub lib/main_catalog.dart lib/core/theme/app_theme.dart lib/app.dart test/catalog
fvm flutter build web --no-pub --target lib/main_catalog.dart --output build/catalog --base-href /flutter/seller/ --no-web-resources-cdn
```

也可使用 `bash scripts/component-catalog.sh run|build|test`。腳本優先使用
專案 FVM SDK 或 `fvm flutter`；環境沒有 FVM 時才使用 PATH 的 Flutter。

最後一個命令提供 Web Storybook iframe 使用的靜態檔；請將整個
`build/catalog/` 放在 host 的 `/flutter/seller/`，保留 assets 與 canvaskit。
單獨提供根目錄服務時，將 `--base-href` 改成 `/`。
Flutter Web host 只啟動元件庫；正式 seller App 的 Web 支援不在此範圍。

## Storybook 深連結

例：`/flutter/seller/index.html?component=buttons&textScale=1.4&embedded=true`。
頁首回歸：`/flutter/seller/index.html?component=chrome&textScale=1.4&embedded=true`。

| ID | 正式來源 | 覆蓋內容 |
| --- | --- | --- |
| `foundations` | `AppColors`, `AppTypography`, `HkSpace`, `HkRadius`, `HkControl` | 品牌色、文字階層、尺寸規範 |
| `chrome` | `GreenHeaderBar`, `HkGlassAppBar`, `HeaderTextButton`, `HkMediaPickerLayout`, `PageHeader` | 含頂部 62px 安全區的連續玻璃表面；長標題、副標題、返回與儲存；正式 Scaffold 頁首；相片預覽與圖庫；長地點／獎項的商店頁首與圓形操作 |
| `buttons` | Material button themes, `HkButtonLabel`, `HkRoundActionButton`, `HkFrostedIconButton` | 主要、次要、文字、停用、長文、可操作加減；圓形霧面送出／儲存／更多與處理中 |
| `inputs` | Material input theme, `AppDropdown` | 輸入、驗證錯誤、停用、分類選單、勾選 |
| `chips` | `HkChoiceChip`, `HkStatusBadge` | 篩選切換與 5 種狀態語意 |
| `cards` | `HkContentCard`, `HkFrostedCardSurface`, `HkPageSection` | 一般、可點擊、提升與霧面表面 |
| `states` | `FruitSpinner`, `HkAsyncStatePanel` | 載入、空狀態、錯誤與重試、離線 |
| `commerce` | `HkTemperatureTag`, `TemperatureTag`, `ProductTemperatureTags`, `LinkedProductCard`, `PillButton`, `StepIndicator` | 溫層選擇、缺圖商品、按內容寬度並排的訂單細節／出貨按鈕、物流步驟 |

`component` 預設 `foundations`；`textScale` 預設 1 並限制在正式政策的
0.8–1.4；`embedded=true` 隱藏搜尋與分類導覽，保留元件標題和實際範例。
所有資料都是本機範例，按鈕只改變元件庫狀態。Storybook 靜態 host 請使用明確的 `index.html` 路徑。

## 正式 Flutter API

賣家目前使用 Material `FilledButton` / `OutlinedButton` / `TextButton`
與 `TextFormField`，樣式來自 `AppTheme.light`；沒有名為 `AppButton`
或 `HkTextField` 的共用包裝。以下範例放在已套用正式 Theme 的 widget
`build` 方法中，`onSave` / `onRetry` 等為呼叫端提供的 callback。

### 頁首與安全區域

```dart
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:hawkr_seller/shared/widgets/green_header_bar.dart';

Scaffold(
  appBar: HkGlassAppBar(
    title: const Text('訂單詳細資訊與出貨進度追蹤'),
    subtitle: '配送地址與收件人聯絡資訊',
    actions: [HeaderTextButton(text: '儲存', onPressed: onSave)],
  ),
  body: const SafeArea(top: false, child: Text('訂單內容')),
);
```

功能頁使用 `HkGlassAppBar`；既有自訂頁首使用 `GreenHeaderBar`。
`GreenHeaderBar` 名稱為相容舊呼叫保留，正式外觀使用奶油色玻璃表面與深色圖示。
共用頁首要為左右操作保留空間，長文字在可用寬度內省略；副標題與放大文字需要足夠行高。
不要以固定螢幕比例、負位移或縮小整列文字來避開按鈕。
狀態列圖示亮度和頂部安全區域底色必須與所在頁首一致，從共用 chrome 設定維護。
`Scaffold.appBar` 已處理上方安全區域時，body 不重複加上頂部 inset；
自訂 body 頁首可使用 `GreenHeaderBar(includeTopSafeArea: true)`，由頁首內層避開頂部 inset，
外層玻璃表面延伸至狀態列；頁面的外層 `SafeArea` 必須設 `top: false`，橫向仍保留左右 inset。
預設 `includeTopSafeArea: false` 保留既有已處理安全區域的呼叫方式。
不要在玻璃頁首上方再加不同底色的狀態列色塊，否則安全區與工具列仍會形成分色。

`chrome` 預覽引用正式 widget，按返回或儲存只累加本機操作次數。

商店 `PageHeaderLayoutSpec` 依實際字級、可用寬度及地點／訂閱者／獎項文字量測身份區，
外層展開高度與內層資訊區共用同一算法。可選 `awardText`／`fansCount` 讓外層提供同一份顯示資料；
地點縮寫規則維持一致。`chrome` 的商店範例使用 sellerId 0 與本機資料，不啟用正式資料請求。
內嵌 `Scaffold` 使用明確高度，避免在可捲動元件庫中收到無界高度。
元件庫會檢查安全區域內的版面，實際狀態列底色仍須由原生裝置驗證。
頂部安全區範例刻意注入 62 logical px inset；同一正式表面包住 62px 安全區與 56px 工具列，
總高 118px，背景使用訂單／錢包相同的 `AppColors.bgPrimary`。
測試除了檢查高度和內容起點，也比較 inset 邊界兩側的實際渲染像素，避免接縫出現突兀色差。

### 相片預覽與圖庫

```dart
import 'package:hawkr_seller/shared/widgets/hk_media_picker_layout.dart';

Scaffold(
  appBar: const HkGlassAppBar(title: Text('選擇照片')),
  body: HkMediaPickerLayout(
    preview: photoPreview,
    grid: photoGrid,
  ),
);
```

`HkMediaPickerLayout({required Widget preview, required Widget grid})` 供商品與貼文選圖頁共用，
由父層提供有限寬高；不要直接放入沒有高度限制的 `SingleChildScrollView`。
直向預覽最多佔可用高度六成，保留圖庫操作空間；可用寬度至少 480px 且大於高度時並排，
方形預覽受高度與寬度限制。元件內含 `SafeArea(top: false)`，適合接在已處理頂部安全區域的 AppBar 下方。
相片載入、選取狀態與送出仍由呼叫端管理。

`chrome` 的色塊照片為固定本機資料，使用正式版面並可點選切換預覽，
不請求相簿權限、不載入手機相片，也不執行圖片上傳。
Catalog 測試檢查直向和橫向的預覽／圖庫同時可見、縮圖選取會更新預覽；
正式元件另有可用高度、圖庫捲動與旋轉回歸測試。

### 按鈕、輸入與數量

```dart
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/widgets/hk_round_action_button.dart';

FilledButton(
  onPressed: onSave, // 傳 null 為停用；callback 由功能頁提供。
  child: const HkButtonLabel('儲存商品'),
);

TextFormField(
  decoration: const InputDecoration(labelText: '商品名稱'),
  validator: (value) => value == null || value.trim().isEmpty
      ? '請填寫商品名稱' : null,
);

HkRoundActionButton(
  semanticLabel: '增加數量',
  symbol: HkRoundActionSymbol.plus,
  onPressed: onIncrease,
);
```

`HkButtonLabel(String label, {TextStyle? style, TextAlign textAlign,
String? semanticsLabel})` 保持一行且在空間不足時省略，輔助技術仍讀取完整文字。
`HkRoundActionButton` 需要 `semanticLabel`、`symbol` 和可為 null 的
`onPressed`；32px 圓形外保留 48px 操作範圍。

### 留言、聊天與更多操作

店舖留言板、聊天輸入列、貼文留言的送出／儲存，以及 `...` 更多操作，
統一使用 `HkFrostedIconButton`。輸入列外層不再包覆整片有底色或框線的方形底板；
輸入欄位保留自己的圓角表面，圖示使用獨立圓形霧面玻璃。
更多操作使用 `AppIcons.moreHoriz` 的無方框三點圖示。

```dart
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_icon_button.dart';

HkFrostedIconButton(
  label: isEditing ? '儲存編輯' : '送出留言',
  icon: isEditing ? AppIcons.check : AppIcons.send,
  onPressed: canSubmit ? onSubmit : null,
  loading: submitting,
  loadingLabel: isEditing ? '正在儲存編輯' : '正在送出留言',
  semanticHint: canSubmit ? null : '輸入留言後即可送出',
);

HkFrostedIconButton(
  label: '更多留言操作',
  icon: AppIcons.moreHoriz,
  onPressed: onOpenMenu,
);
```

必填 `label`、`icon` 與可為 null 的 `onPressed`；可選 `loading`（預設 false）、
`loadingLabel`（預設「處理中」）及 `semanticHint`。
`label` 提供 tooltip 與操作語意；表面與觸控範圍固定為 48 logical px，圖示為 24px。
停用與處理中仍保留同一個圓形霧面表面，處理中顯示進度且不能再次點按。
驗證、送出、錯誤保留草稿與焦點仍由功能畫面處理；共用按鈕不持有交易狀態。
元件庫 `buttons` 分類可同時檢查一般、停用、處理中、更多及儲存五種狀態。

非同步操作須在點擊時固定本次內容與回覆／編輯目標，處理中阻止重送及切換目標。
失敗時保留草稿與操作模式並提示錯誤；離開畫面後，延遲回應不得更新已釋放的狀態。
更多選單的取消不執行操作，編輯與回覆模式必須互斥。
操作回歸包含圓形邊緣點擊、取消按壓、鍵盤與讀屏啟動、延遲／失敗回應，
以及 `integration_test/frosted_actions_smoke_test.dart` 的隔離原生鍵盤與點擊測試。
原生測試使用正式 widget 與本機 callback，不能代替已登入帳號的後端操作驗收。

### 篩選、狀態與卡片

```dart
import 'package:hawkr_seller/shared/widgets/hk_choice_chip.dart';
import 'package:hawkr_seller/shared/widgets/hk_content_card.dart';
import 'package:hawkr_seller/shared/widgets/hk_status_badge.dart';

HkChoiceChip(
  label: '上架中',
  selected: selectedFilter == '上架中',
  onTap: () => setState(() => selectedFilter = '上架中'),
);

const HkStatusBadge(label: '待出貨', tone: HkStatusTone.warning);

HkContentCard(
  elevated: true,
  semanticLabel: '查看範例商品',
  onTap: onOpenProduct,
  child: const Text('季節鮮採蔬菜箱'),
);
```

`HkChoiceChip` 的 `label`、`selected`、`onTap` 都必填；`icon` 與 `selectedIcon` 選填，
選取時換成 `selectedIcon`，讓選取狀態不只靠顏色。
`HkStatusTone` 是 `neutral / success / warning / danger / info`。
`HkContentCard` 必填 `child`；預設 padding16、radius16、非 elevated；
只有提供 `onTap` 才變成可操作卡片，並應同時提供 `semanticLabel`。

### 載入、回饋與配送

```dart
import 'package:hawkr_seller/shared/widgets/hk_async_state_panel.dart';
import 'package:hawkr_seller/shared/widgets/fruit_spinner.dart';
import 'package:hawkr_seller/shared/widgets/temperature_tag.dart';
import 'package:hawkr_seller/shared/widgets/hk_temperature_tag.dart';
import 'package:hawkr_seller/shared/widgets/linked_product_card.dart';
import 'package:hawkr_seller/shared/widgets/step_indicator.dart';

const FruitSpinner(size: 40);

HkAsyncStatePanel(
  kind: HkAsyncStateKind.error,
  title: '暫時無法載入',
  description: '請稍後再試。',
  actionLabel: '重新載入',
  onAction: onRetry,
);

const HkTemperatureTag(value: '3202');

TemperatureTag.selectable(
  code: '3202', // 3201 常溫、3202 冷藏、3203 冷凍。
  isSelected: chilledSelected,
  onTap: onSelectChilled,
);

LinkedProductCard(
  linkedProduct: const {'name': '季節蔬菜箱', 'price': 680},
  inset: false,
  onTap: onOpenProduct,
);

const StepIndicator(
  steps: StepIndicator.logisticsSteps,
  currentStep: 1, // 零起算，賣家物流：已下單 / 待出貨 / 配送中 / 已送達。
);
```

`HkAsyncStateKind` 支援 `empty / filteredEmpty / offline / error / restricted`；
必填 `kind`、`title`、`description`，有 `actionLabel` 才顯示操作。
`LinkedProductCard` 接受實際 `linkedProduct` Map，`name` 為空不顯示；
圖片欄位未提供時顯示正式缺圖狀態，`inset: false` 用於父層連續內容表面。

完整建構子以 `lib/shared/widgets/` 對應檔案為準。`PillButton` 位於
`lib/features/order/presentation/order_card_widgets/pill_button.dart`，只用於
訂單卡片操作，必填 `label` 與 `color`，可選 `onTap` 與 `outlined`。
按鈕寬度由文字與內距決定，外層保留至少 48 × 48 logical px 的操作範圍。
一組訂單操作使用 `Wrap`，不替每個按鈕指定整列寬度；空間充足時並排，放大文字或操作較多時才換行。
`commerce` 直接使用正式「細節」與「預約出貨」按鈕，沒有用固定寬度包裝掩蓋尺寸問題；
測試在 320／402px 與 1／1.4 倍文字檢查內容寬度、同行排列和 48dp 操作範圍，
並從可視膠囊上方的觸控留白點擊，確認留白也能觸發操作。

## 一致性與維護

- 基礎分類沿用 buyer 目錄的分類 ID、查詢參數與目標尺寸，方便在 Storybook 並排比較；`chrome` 額外集中賣家頁首與選圖版面。
- ScreenUtil 停用寬高及文字縮放；只由實際 constraints 改變版面，文字遵循
  `AppMediaQueryPolicy`。測試直接渲染目錄入口，在 320、390、402、440、768 logical px
  與 1.4 倍文字檢查所有分類，並驗證篩選、表單、數量及營運操作。
- 新元件請加入正式 `shared/widgets` 或 feature widget，再在此入口引用；
  不要為了預覽另做一份相似樣式。
- 目前涵蓋共用基礎與無需後端的營運元件。依賴登入、API、支付、圖片上傳、
  完整訂單資料的畫面仍應使用自己的整合測試，不能由此目錄視為已驗證。
- Flutter 與 Web 的字型、文字語意 token 等既有差異在跨平台 token 文件追蹤；
  此入口呈現正式實作，不用目錄專用覆寫遮蔽差異。

### 不同裝置的版面回歸

Flutter 版面使用 logical pixels，不能直接拿手機面板的實體解析度當元件寬度。
例如測試中的 402 × 874 logical px 配合 3 倍 device pixel ratio，
會產生 1206 × 2622 physical px；元件仍依可用的 402px 寬度與安全區域排版。
Display Zoom、文字大小與橫向旋轉可能改變可用空間，應讀取當前 constraints 和 MediaQuery，
不要依手機型號選擇硬編碼尺寸。

內容列使用 `Expanded` / `Flexible` 分配剩餘寬度，多個操作或標籤使用 `Wrap`；
只有完整內容可捲動時才限制可視高度，避免固定卡片高度裁切大字內容。
頁首、圖示與觸控範圍保留 logical px 尺寸，寬螢幕可限制內容最大寬度，
鍵盤出現時仍應能捲動到輸入框和送出操作。

`component_catalog_test.dart` 額外以 320 × 568、402 × 874、440 × 956、874 × 402
檢查 `chrome` 分類，包含 1 / 1.4 倍文字、3 倍像素密度、直向頂部／底部 inset
與橫向左右 inset，並驗證頁首儲存按鈕可操作。
這些測試的安全區域為明確設定的模擬值，不宣稱所有 iOS 版本或顯示設定的 inset 相同。

原生驗收還需在 iPhone 16 Pro 或同尺寸模擬器打開正式賣家頁面，
檢查首次開啟、頁面切換、捲動、鍵盤、直橫向及系統大字設定，
確認狀態列與頁首沒有異色區塊、瀏海遮住操作或文字越界。
Widget／catalog 測試不等於已安裝到手機，也不等於已登入的完整功能流程驗收。

## 版本與文件維護

共用 UI 修改需一起更新 catalog、Dart API 文件及測試；由 Web 的 `pnpm storybook:sync-app-docs` 同步。

受保護的 Web catalog 使用自訂 `web/flutter_bootstrap.js`，不安裝離線 service worker。舊 UI 版本以獨立 bundle 保留；回復文件庫版本不等於回復已發布的 App。

### 配送溫層與 Web 對齊

- `HkTemperatureTag(value:)` 是買家／商家共用 API：3201／AMBIENT／常溫為灰 `#6B6B6B`，3202／CHILLED／冷藏為橘 `#EE964B`，3203／FROZEN／冷凍為藍 `#51A2FF`。未知值保留原文並用灰色；空值不產生標籤。
- 顯示標籤使用透明底、全圓角、1dp 同色邊框、左右 8dp、11dp 粗體、零字距；1 倍字級時最小高 18dp。Flutter 文字行盒使用 16dp 加上下邊框，保留系統字級放大與增高，沒有固定高度裁字。
- `TemperatureTag` 相容原有 code／label 呼叫；`.selectable` 外加 48dp 操作範圍、selected 語意與獨立勾選符號，標籤本身仍為同一外觀。
- 商品上架、物流設定、訂單卡片、訂單細節與物流追蹤均使用共用標籤。商品預覽 `ProductTemperatureTags` 以真正的 `deliveryTemp` 字串／清單顯示，可換行、去除重複標籤；缺值顯示「尚未指定」，不再固定顯示常溫／冷藏。詳情 API 失敗或欄位無效時保留已知溫層，成功回傳空清單時清除。
- `commerce` catalog 包含三種溫層、選取、未知值與缺值範例，使用正式元件與正式 AppTheme。


### 2026-09-21 裝置版面修正（patch，尚未發行）

- 首頁玻璃頁首背景延伸至頂部安全區，內容只避開一次動態島；營收卡依金額與字體實際寬度改為上下排列。
- 設定頁載入與淺色內容使用深色狀態列文字，只有封面區域使用白字；捲離封面自動恢復深色，商店固定封面收合時仍維持白字。
- 共用頁首替返回／儲存操作保留寬度，長標題以省略號呈現，副標題保留兩行文字所需高度。
- 物流姓名／地址輸入框保留 48dp 最小觸控高度，並隨放大文字增高。
- 商品與貼文選圖頁共用 `HkMediaPickerLayout`，短視窗保留圖庫高度，橫向將預覽與圖庫並排，避免正方形預覽把圖庫擠出畫面。
- 無公開 API 移除；需以新版 App 安裝包發行才會影響手機。這是未發行 patch 紀錄，不是 App Store、TestFlight 或文件庫發布證據。

### 2026-09-22 訂單與錢包頂部色差修正（patch，尚未發行）

- `GreenHeaderBar` 新增可選 `includeTopSafeArea`，讓狀態列安全區與工具列共用一片玻璃表面，而非只調整狀態列圖示亮度。
- 訂單、營運錢包及錢包認證選擇頁採用此模式，外層僅保留左右／底部安全區，內容不重複位移。
- `chrome` catalog 新增 62px＋56px 的實際共用元件預覽與邊界像素回歸；固定 inset 為模擬資料，不代表所有裝置或 iOS 設定相同。
- 保留既有 2026-09-21 版面修正。這筆紀錄不表示已更新 TestFlight、App Store 或使用者手機上的安裝版本。

### 2026-09-22 訂單卡片自適應排版（patch，尚未發行）

- 保留既有圖示、色彩、膠囊圓角及資訊層級，調整資訊與操作的空間分配。
- 訂單操作按內容決定寬度，保留 48dp 觸控範圍；卡片底列在可用空間足夠時緊湊排列，空間不足才換行。
- 列表左右留白調為 16dp、卡片間距 12dp；寬視窗將內容置中並限制為 720dp。篩選按鈕保留 48dp 最小觸控高度。
- 買家與時間、地址與物流資訊依內容與可用寬度調整，長資訊不與相鄰控制項重疊；總金額需要更多空間時保留完整內容。
- 版面依 logical pixels、實際文字寬度及系統字級處理，不按 iPhone 型號套固定比例。元件庫覆蓋 320／402px 與 1／1.4 倍文字的按鈕案例；完整訂單卡片另由功能測試驗證。
- 此紀錄為未發行的程式修正與本機回歸範圍，不代表已更新使用者手機或完成登入後的真實出貨流程。

### 2026-09-22 商店三分頁與詳細流程版面檢查（patch，尚未發行）

- 檢查貼文、商城、留言，以及貼文詳細／新增／編輯、商品詳細／評價／上架步驟的排版，沿用既有圖示、資產、色彩和圓角。
- 修正商店頁首長地點／獎項在大字體超出固定高度；貼文長互動數字、回覆資訊、商品選擇卡和價格改為受限寬度內換行。
- 留言保留單一輸入框、回覆／編輯控制器、防重送與失敗草稿；姓名／日期可換行、回覆操作保留 48dp，短視窗加鍵盤時依剩餘高度調整可見輸入行數。
- 商城卡片保留商品原圖比例，價格不省略；商品詳情的價格、庫存、評價資訊可換行。上架步驟頁首隨字體增高；認證選項使用自然高度，對話框可在鍵盤上方捲動；操作與必填標示避免擠壓。
- 分類搜尋與新增分類操作依可用寬度排列；短橫向加鍵盤時，上架表單與步驟內容維持可捲動，底部操作可到達。未認證、審核中、缺物流設定及商城空狀態另用封閉資料驗證。
- 共用上一步／完成且預覽底欄以正式 AppTheme、320／402／440px、1／1.4 倍文字驗證完整標籤、48dp 操作範圍及回呼，未做不必要的外觀修改。
- `chrome` 新增正式 PageHeader 的長資料預覽與圓形控制尺寸檢查；功能測試另覆蓋短橫向和鍵盤。未改變商品上架、認證、售後或貼文提交的業務規則。
- 此為本機未發行 UI 修正；封閉模擬資料與模擬器畫面不代表真實登入操作或手機安裝包已更新。

### 2026-09-22 商店貼文詳情對齊買家版（patch，尚未發行）

- 以買家商店進入的 `MerchantPostsFeedScreen` 為視覺參考，賣家保留自己的貼文資料、左右切篇及管理路由。
- 貼文採安全區內的大圖視窗、圖片置中與模糊背景；底部依序呈現店家／日期、緊湊關聯商品、可展開說明，以及固定的按讚／留言／分享列。
- 關聯商品縮圖採單軸解碼，保留原圖比例後裁切至 44dp 方形；避免長方形圖片先被拉伸。
- 左上關閉返回與右上管理操作保留 48dp 觸控範圍；賣家仍有編輯、刪除，並保留確認及 API 錯誤回饋。
- 留言由底部面板開啟，沿用賣家身分、回覆與刪除規則；單一輸入控制器、草稿、巢狀回覆對象與防重複送出均保留。
- 小螢幕、大字體與短橫向時依可用高度限制說明面板；鍵盤開啟時留言內容可捲動，輸入與送出操作保持可達。
- 原生比較使用兩版正式 widget、相同合成貼文及本機範例圖片；未修改買家程式，也不代表已發行或完成真實業務交易。
