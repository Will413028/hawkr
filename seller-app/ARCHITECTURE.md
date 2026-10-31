# Architecture

## 1. 概觀 (Overview)

- **專案類型**: Flutter E-commerce Seller App (Thin Client)
- **後端依賴**: Go Microservices (REST API)
- **核心目標**: 快速迭代、減少樣板代碼 (Boilerplate)、高維護性
- **主要職責**: 專注於 UI 展示、表單輸入與狀態管理。複雜業務邏輯 (Business Logic) 一律下放至後端

## 2. 核心原則 (Core Principles)

1. **Feature-First Structure**: 專案結構以「功能 (Feature)」為單位切分，而非以技術層 (Layer) 切分
2. **No Interfaces**: Repository 層不定義抽象介面，直接實作具體類別。利用 Riverpod 的 override 機制進行 Mocking
3. **Single Model Strategy**: 使用單一 Freezed 類別同時作為 Entity 與 DTO。除非後端 API 結構極度巢狀或不合理，否則不進行 Model Mapping
4. **Thin Client**: 只有 UI 邏輯 (Validation, Animation, Navigation) 留在 App。價格計算、庫存檢查、訂單狀態流轉等邏輯必須由 API 處理
5. **Declarative State**: 優先使用 Riverpod Generator (@riverpod) 與 AsyncValue 處理非同步狀態
6. **Caching**: 所有列表型資料必須使用 `ref.keepAlive()` 避免切換頁面時重複請求

## 3. 架構分層 (Layers)

### A. Presentation Layer (UI & Logic)

- **Widgets**: 使用 `HookConsumerWidget`
- **Controllers**: 使用 Riverpod (`AsyncNotifier` / `FutureProvider`) 管理狀態。負責呼叫 Repository 並處理 Loading/Error 狀態
- **UI Logic**: 使用 Flutter Hooks (`useTextEditingController`, `useAnimation`) 處理 View 內部的短暫狀態

### B. Data Layer (Data Access)

- **Repository**: 直接實作 API 呼叫邏輯（無 Interface）
- **Model**: 使用 Freezed + JsonSerializable 定義資料結構
- **Data Source**: Dio 直接呼叫

### C. Core Layer (Infrastructure)

全域共用的設定與工具：Dio 實例、GoRouter 設定、Theme、Extensions、Storage 封裝

## 4. 目錄結構 (Directory Structure)

```
lib/
├── app.dart                       # App Entry Point (MaterialApp + ProviderScope)
├── main.dart                      # Entry point (flavor init + runApp)
├── config/                        # Flavor & Firebase config
│
├── core/                          # 共用基礎設施
│   ├── auth/                      # Firebase Phone 驗證工具
│   ├── exceptions/                # AppException 定義
│   ├── extensions/                # AsyncValue UI helpers
│   ├── networking/                # Dio instance, interceptors, API response
│   ├── router/                    # GoRouter configuration
│   ├── session/                   # Session state (Freezed model + Riverpod Provider)
│   ├── storage/                   # Flutter Secure Storage wrapper
│   └── theme/                     # AppColors
│
├── features/                      # 功能模組 (Feature-First)
│   ├── auth/                      # 登入（原生 SDK：Google、LINE；session token 在 Keychain）
│   │   ├── data/                  # auth_repository, auth_model, user_model
│   │   └── presentation/         # auth_controller, auth_screen, ...
│   ├── dashboard/                 # 首頁 (presentation only)
│   ├── merchant/                  # 商家資訊、認證、PAYUNi、個人檔案
│   │   ├── data/                  # merchant_repository, merchant_model
│   │   └── presentation/         # certification_controller, profile_controller, ...
│   ├── product/                   # 商品 CRUD、圖片、規格、標籤
│   │   ├── data/                  # product_repository, product_model
│   │   └── presentation/         # product_images_controller, ...
│   ├── order/                     # 訂單列表、詳情、各狀態頁面
│   │   ├── data/                  # order_repository, order_model
│   │   └── presentation/         # order_list_controller, order_detail_controller
│   ├── chat/                      # 聊天列表、聊天室、嵌入式面板
│   │   ├── data/                  # chat_repository, chat_model
│   │   └── presentation/
│   ├── post/                      # 社群貼文、動態牆
│   │   ├── data/                  # post_repository, post_model
│   │   └── presentation/
│   ├── coupon/                    # 優惠券
│   │   ├── data/                  # coupon_repository, coupon_model
│   │   └── presentation/
│   ├── customer/                  # 顧客管理
│   │   ├── data/                  # customer_repository, customer_model
│   │   └── presentation/
│   ├── notification/              # 通知
│   │   ├── data/                  # notification_repository, notification_model
│   │   └── presentation/
│   ├── media/                     # 媒體上傳 (data only)
│   │   └── data/                  # media_repository
│   ├── wallet/                    # 錢包 (presentation only)
│   ├── settings/                  # 設定 (presentation only)
│   └── photo_editor/              # 圖片編輯器 (presentation only)
│
└── shared/                        # 跨 Feature 共用
    ├── widgets/                   # FloatingToast, GreenHeaderBar, RichTextEditor, ...
    └── utils/                     # HKLog, ImageFormatDetector, PlatformIO
```

> **Feature 內部結構**: `presentation/` 下扁平放置 controllers、screens、widgets，不再細分子目錄。只有 data/ 和 presentation/ 兩個子層級。

## 5. 技術選型 (Tech Stack)

| Category | Technology |
|----------|-----------|
| Framework | Flutter (Latest Stable) |
| State Management | Riverpod 2.0+ (with riverpod_generator) |
| UI Logic | Flutter Hooks |
| Immutability/Serialization | Freezed, json_serializable |
| Networking | Dio |
| Navigation | GoRouter |
| Local Storage | Flutter Secure Storage (Token/Session), SharedPreferences (簡單設定) |
| Database | None (無本地資料庫) |

## 6. 數據流向 (Data Flow)

```
User Action → Controller (Riverpod Provider) → Repository → Remote API (Go Microservices)
     ↑                                                              │
     └──── UI Rebuild (ref.watch) ← State Update (AsyncValue.data) ←┘
```

## 7. 錯誤處理 (Error Handling)

| Layer | Strategy |
|-------|----------|
| Repository | 捕捉 DioException → 轉換為自定義 AppException (NetworkError, UnauthorizedError) |
| Controller | 使用 `AsyncValue.guard(() => ...)` 自動處理 try-catch → AsyncError |
| UI (Global) | Dio Interceptor 處理 401 (Token 過期) 自動登出 |
| UI (Local) | 使用 `.when()` 或擴充方法顯示錯誤訊息 (SnackBar/Dialog) |

## 8. 測試策略 (Testing Strategy)

| Type | Description |
|------|-------------|
| Unit Tests | 針對 Utility functions 或 StateNotifier 邏輯 |
| Repository Tests | 使用 http_mock_adapter Mock Dio，測試 JSON 解析與錯誤處理 |
| Widget/Integration Tests (重點) | 使用 `ProviderScope(overrides: [...])` Mock Repository Provider，測試 UI 與 Controller 互動 |

## 9. 命名慣例 (Naming Conventions)

| Type | Convention | Example |
|------|-----------|---------|
| Files | snake_case.dart | `product_repository.dart` |
| Classes | PascalCase | `ProductRepository` |
| Variables | camelCase | `orderList` |
| Providers | Generator 自動產生 | Class `ProductController` → `productControllerProvider` |
| Methods | 動詞開頭 | `fetchProducts`, `updateStatus` |
| Enums | PascalCase | `OrderStatus.pending` |

> **Note to AI**: Always verify if a specialized "DTO" is truly needed before creating one. Default to using the single Freezed model unless the API response is drastically different from the UI requirements.
