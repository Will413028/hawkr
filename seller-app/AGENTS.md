# Hawkr Seller — Flutter Seller App

## 開發指令
- **執行開發環境 (Debug)**: `make dev-debug` (使用 `fvm flutter run --flavor dev`)
- **執行預發布環境 (Debug)**: `make stage-debug`
- **代碼生成 (Build Runner)**: `fvm dart run build_runner build --delete-conflicting-outputs`
- **代碼分析與格式化**: `make analyze` / `make format`
- **單元測試**: `make test`
- **enum 文字守門**: `make enum-text`（規則見根目錄 `AGENTS.md`，CI 的 `_flutter-quality.yml` 同一條）
- **環境清理與重裝**: `make clean-reinstall` (清理快取並重裝 Pods)

## 元件庫維護

- 共用 UI 工作先讀 [賣家 App 元件庫](docs/component-library.md)，在同一次 PR 更新正式 widget、catalog 案例、Dart API 文件與相關測試；不要複製另一套預覽專用元件或主題。
- 其餘規則（catalog 指令、審查入口與文件同步、版本與發布）兩支 App 共用一份，在 `../buyer-app/AGENTS.md`「元件庫維護」。

## 目錄結構
```
lib/
├── app.dart                       # App 入口 (MaterialApp + ProviderScope)
├── main.dart                      # 啟動點 (Flavor 初始化 + Firebase 設定)
├── core/                          # 基礎設施 (Networking, Router, Session, Theme)
├── features/                      # 功能模組 (Feature-First)
│   ├── auth/                      # 登入（Clerk：Google、Apple、LINE）
│   ├── product/                   # 商品 CRUD、規格標籤、圖片編輯
│   ├── order/                     # 訂單管理、各狀態頁面
│   ├── seller/                    # 賣家認證、PAYUNi、錢包、個人檔案
│   ├── chat/                      # 即時通訊、聊天室
│   └── ...                        # 其他 (Dashboard, Post, Notification, Wallet)
├── shared/                        # 跨模組共用 (Widgets, Utils)
└── config/                        # Flavor 與 Firebase Options 配置
```

## 技術規範
- **核心架構**: Flutter（版本以 `.fvmrc` 為準），Feature-First 分層。
- **狀態管理**: Riverpod 3（`flutter_riverpod`／`hooks_riverpod`），provider 用 `riverpod_generator` 產生。
- **UI 邏輯**: Flutter Hooks (處理動畫、Controller 生命週期)。
- **資料模型**: Freezed (Immutable) 與 json_serializable。
- **網路通訊**: Dio。
- **本地存儲**: Flutter Secure Storage (敏感資訊) / SharedPreferences。

## 開發慣例
- **Feature 內部結構**: `presentation/` 下扁平化放置，大型 Feature 用前綴命名 (例: `order_list_screen.dart`)。
- **代碼生成**: 修改 `Freezed` 模型或 `Riverpod` 提供者後，必須執行 `build_runner`。
- **Icons 規範**: feature code 透過 `AppIcons`（`lib/shared/icons/app_icons.dart`）或 `HkIcon`／`HkIconName` 取圖示，底層是 Flutter 內建的 Material Icons，不放圖示檔；品牌 Logo 才用圖片。
- **共通規範**: 跨專案規則見根目錄 `AGENTS.md`。
