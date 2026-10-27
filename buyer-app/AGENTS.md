# Hawkr Buyer — Flutter Buyer App

## 開發指令
- **執行開發環境 (Debug)**: `fvm flutter run --flavor dev -t lib/main.dart`
- **執行預發布環境 (Debug)**: `fvm flutter run --flavor stage -t lib/main.dart`
- **代碼生成 (Build Runner)**: `fvm dart run build_runner build --delete-conflicting-outputs`
- **代碼分析與格式化**: `fvm flutter analyze`、`fvm dart run custom_lint` / `fvm dart format lib test integration_test tool`（CI 以 `--output=none --set-exit-if-changed` 檢查）
- **單元測試**: `fvm flutter test`
- **獲取依賴**: `fvm flutter pub get`

## 元件庫維護

下面四條中，後三條也適用 seller-app（`../seller-app/AGENTS.md` 指向這裡）。

- 共用 UI 工作先讀 [買家 App 元件庫](docs/component-library.md)，在同一次 PR 更新正式 widget、catalog 案例、Dart API 文件與相關測試；不要複製另一套預覽專用元件或主題。
- 使用 `.fvmrc` 固定的 Flutter SDK；`bash scripts/component-catalog.sh run` 開啟目錄，`bash scripts/component-catalog.sh test` 檢查 catalog，另跑受影響的既有 widget tests 與 analyze。
- 共用審查入口是管理後台的 `/admin/documentation`（部署環境尚未重建）。App 文件改動後，在 `website/` 執行 `pnpm storybook:sync-app-docs`；需要發布元件庫時先和使用者確認做法。
- UI 元件庫版本與手機 App 版本分開管理。已發布的來源 commit 與 artifact 保持不可變，更新以新 UI 版本發布；修改文件、建立 PR 或發布 catalog 都不代表已部署手機 App。

## 目錄結構
```
lib/
├── main.dart                      # 啟動點 (Flavor 初始化 + Firebase 設定)
├── core/                          # 基礎設施 (Networking, Router, Notifications, Theme)
├── features/                      # 功能模組 (Feature-First)
│   ├── home/                      # 首頁、Banner、探索
│   ├── category/                  # 分類瀏覽、篩選、搜尋
│   ├── product/                   # 商品詳情、評論 (Review)
│   ├── cart/                      # 購物車、結帳
│   ├── order/                     # 訂單列表、詳情、追蹤
│   ├── chat/                      # 即時通訊 (WebSocket)
│   ├── social/                    # 商家貼文、關注、造訪記錄
│   └── ...                        # 其他 (Auth, Wallet, Notification, Address)
├── shared/                        # 跨模組共用 (Widgets, Utils)
└── config/                        # Firebase 與 Flavor 配置
```

## 技術規範
- **核心架構**: Flutter（版本以 `.fvmrc` 為準），Feature-First 分層。
- **狀態管理**: Riverpod 3（`hooks_riverpod`），provider 用 `riverpod_generator` 產生。
- **UI 邏輯**: Flutter Hooks (動畫與生命週期管理)。
- **網路與存儲**: Dio (API 通訊), Flutter Secure Storage (加密存儲)。
- **效能優化**: 
  - `cached_network_image`: 強制設定 `memCacheWidth` (建議 400px) 並搭配骨架屏。
  - 模型分離: `ProductSummaryModel` (列表用) 與 `ProductDetailModel` (詳情用)。

## 開發慣例
- **下拉選單**: `AppDropdown<T>`／`ProfileDropdownField<T>` 以領域值為單位（`items: List<T>`、`itemLabel` 必填、`onSelected` 收到值），不以顯示文字當鍵再反查；沒有值是 `null`，「沒有填」那一項是 `items` 裡值為 null 的一項。契約值的文字取自 `hawkr_ui_text`。
- **Icons 規範**: feature code 透過 `AppIcons`（`lib/shared/icons/app_icons.dart`）或 `HkIcon`／`HkIconName` 取圖示，底層是 Flutter 內建的 Material Icons，不放圖示檔；品牌 Logo 才用圖片。
- **代碼生成**: 修改 `Freezed` 模型或 `Riverpod` 提供者後，必須執行 `fvm dart run build_runner build --delete-conflicting-outputs`（API client 手寫於 repository）。

## 驗證與範圍

- 完成前跑 `fvm flutter analyze`、`fvm flutter test`、`(cd ../packages/hawkr_ui_text && dart pub get && dart run tool/check_enum_text.dart ../../buyer-app/lib)`（enum 文字守門，規則見根目錄 `AGENTS.md`），動到 Freezed／Riverpod 時先跑 build_runner。UI 改動在模擬器實際走過涉及的頁面、表單、API 串接與錯誤顯示，並確認 request／response 符合後端契約。
- 調整共用元件、共用 Layout、路由結構、全域狀態或主要頁面結構前，先說明並取得使用者確認。
- 不留暫存的計畫或分析文件；需要新增正式文件時，先說明位置與用途。

共通規則（UI 改動的授權、Git、不留技術債）見根目錄的 `AGENTS.md`。
