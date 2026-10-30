# Hawkr - 消費者端 App

台灣第一生鮮直銷平台的消費者端 Flutter 應用程式。

## 共用元件庫

[買家 App 元件庫文件](docs/component-library.md) 提供真實 widget 的 catalog、Dart API、啟動與測試方式；website 的 Storybook（`pnpm storybook`）可對照買家、賣家與 Web。

共用元件改動應在同一次 PR 更新正式 widget、catalog、API 文件與測試，Web 的 Storybook 在 build 時直接讀入 API 文件。文件與 catalog 的更新不代表已發布手機 App。

## 功能特色

### 核心功能
- **商品瀏覽** - 首頁商品展示、分類篩選、搜尋功能
- **商家系統** - 商家頁面、商家商品列表、商家貼文
- **購物車** - 加入購物車、數量調整、結帳流程
- **訂單管理** - 訂單列表、訂單詳情、訂單追蹤
- **即時聊天** - 與商家即時通訊、訊息通知
- **會員系統** - Google/Firebase 登入、會員資料管理
- **錢包系統** - 紅利點數、優惠券管理

### 進階功能
- **推播通知** - Firebase Cloud Messaging 推播
- **地址定位** - TGOS（內政部國土測繪中心）地址查詢與驗證、地理位置服務
- **社群功能** - 商家貼文、按讚、留言、回覆
- **收藏系統** - 商品收藏、商家關注、瀏覽紀錄

## 專案結構

```
lib/
├── main.dart                          # 應用程式入口
├── config/
│   └── environment.dart               # 環境設定 (production/stage)
├── core/
│   ├── exceptions/                    # 例外處理
│   ├── extensions/                    # 擴展方法
│   ├── networking/
│   │   ├── dio_provider.dart          # Dio HTTP 客戶端
│   │   └── oss_service.dart           # OSS 圖片服務
│   ├── notifications/
│   │   └── push_notification_manager.dart
│   ├── routing/
│   │   └── app_router.dart            # GoRouter 路由
│   ├── session/
│   │   └── session_provider.dart      # 登入狀態管理
│   └── storage/
│       └── auth_storage.dart          # Token 安全存儲
├── features/
│   ├── auth/                          # 認證 (Google/Firebase)
│   │   ├── data/auth_repository.dart
│   │   └── presentation/
│   ├── cart/                          # 購物車
│   │   ├── data/cart_repository.dart
│   │   └── presentation/
│   ├── order/                         # 訂單
│   │   ├── data/order_repository.dart
│   │   └── presentation/
│   ├── product/                       # 商品
│   │   ├── data/product_repository.dart
│   │   └── presentation/
│   ├── seller/                        # 商家
│   │   ├── data/seller_repository.dart
│   │   └── presentation/
│   ├── category/                      # 分類
│   │   ├── data/category_repository.dart
│   │   └── presentation/
│   ├── chat/                          # 聊天
│   │   ├── data/chat_repository.dart
│   │   └── presentation/
│   ├── follow/                        # 關注
│   │   └── data/follow_repository.dart
│   ├── visit/                         # 瀏覽紀錄
│   │   └── data/visit_repository.dart
│   ├── review/                        # 評論
│   │   └── data/review_repository.dart
│   ├── address/                       # 地址
│   │   └── data/address_repository.dart
│   ├── wallet/                        # 錢包 (紅利/優惠券)
│   │   └── data/wallet_repository.dart
│   ├── notification/                  # 通知
│   │   └── data/notification_repository.dart
│   ├── social/                        # 社群 (貼文)
│   │   └── data/social_repository.dart
│   ├── message/                       # 留言板
│   │   └── data/message_repository.dart
│   ├── home/                          # 首頁
│   │   └── data/banner_repository.dart
│   └── settings/                      # 設定
│       └── presentation/
└── shared/
    └── widgets/                       # 共用元件
```

## 技術架構

### 前端框架
- **Flutter** 3.47.7（由 `.fvmrc` 固定）
- **Dart** SDK ^3.8.0

### 架構模式
- **Feature-First** 架構 (按功能模組分目錄)
- **Riverpod** 狀態管理 (riverpod_annotation + code generation)
- **Freezed** 資料模型 (不可變 + JSON 序列化)
- **GoRouter** 宣告式路由
- **Repository Pattern** 資料存取層

### 主要依賴
| 套件 | 用途 |
|------|------|
| `hooks_riverpod` | 狀態管理 |
| `riverpod_annotation` | Riverpod 程式碼產生 |
| `freezed_annotation` | 不可變資料模型 |
| `dio` | HTTP 請求 |
| `go_router` | 路由管理 |
| `firebase_core` | Firebase 核心 |
| `firebase_messaging` | 推播通知 |
| `firebase_auth` | Firebase 認證 |
| `flutter_web_auth_2` | Clerk 的 OAuth（Google、LINE）在系統瀏覽器完成 |
| `sign_in_with_apple` | Sign in with Apple |
| `flutter_svg` | SVG 圖片 |
| `image_picker` | 圖片選擇 |

## API 整合

所有 API 端點已對齊 OpenAPI 規格。

### API 端點路徑規則
| 前綴 | 說明 |
|------|------|
| `/public/*` | 公開端點，不需認證 |
| `/app/*` | 需認證端點 (Bearer Token) |

### 主要 API 端點

#### 認證
- 登入與登出由 Clerk 處理（Google、LINE 經系統瀏覽器，Apple 用 Sign in with Apple），請求以 Clerk 的 session token 帶 `Authorization: Bearer`
- `IdentityAppService/GetSession`（Connect）- 登入後取得帳號（第一次登入時建立）

#### 會員
- `GET /app/buyers/me` - 取得個人資料
- `PUT /app/buyers/me` - 更新個人資料
- `GET /app/buyers/me/addresses` - 地址管理
- `GET /app/buyers/me/following` - 關注列表
- `GET /app/buyers/me/favorites` - 收藏列表
- `GET /app/buyers/me/history` - 瀏覽紀錄
- `GET /app/buyers/me/notifications` - 通知列表

#### 商品
- `GET /public/products` - 商品列表 (分頁)
- `GET /public/products/{id}` - 商品詳情
- `GET /public/products/{id}/reviews` - 商品評論
- `GET /public/products/{id}/rating` - 商品評分
- `GET /public/search/products` - 搜尋商品
- `GET /public/search/suggest` - 搜尋建議

#### 分類
- `GET /public/categories/tree` - 分類樹

#### 商家
- `GET /public/sellers/{id}` - 商家資訊
- `GET /public/sellers/{id}/products` - 商家商品列表
- `GET /public/sellers/{id}/posts` - 商家貼文

#### 購物車
- `GET /public/carts` - 取得購物車 (guest_id)
- `POST /public/carts/items` - 加入購物車
- `PUT /public/carts/items/{sku_id}/quantity` - 更新數量
- `DELETE /public/carts/items/{sku_id}` - 移除商品
- `POST /app/carts/merge` - 合併購物車

#### 訂單
- `GET /app/orders` - 訂單列表 (分頁)
- `GET /app/orders/{orderSn}` - 訂單詳情
- `POST /app/orders` - 建立訂單

#### 錢包
- `GET /app/buyers/me/points` - 紅利點數
- `GET /app/buyers/me/points/history` - 點數紀錄
- `GET /app/coupons` - 優惠券列表

#### 聊天
- `GET /app/chat/conversations` - 對話列表
- `GET /app/chat/history/{targetId}` - 聊天紀錄
- `POST /app/chat/messages` - 發送訊息
- WebSocket: `wss://{host}/app/chat/ws`

#### 裝置推播
- `POST /app/devices` - 註冊裝置
- `DELETE /app/devices/{deviceId}` - 取消註冊

## 開發環境設置

### 前置需求
- Flutter SDK 3.47.7（請使用 FVM）
- Dart SDK ^3.8.0
- VS Code
- Xcode (iOS 開發)

### 安裝步驟

```bash
# 克隆專案
git clone <repo-url>
cd <repo>/buyer-app

# 安裝依賴
flutter pub get

# 產生程式碼 (Riverpod / Freezed / JSON)
dart run build_runner build --delete-conflicting-outputs

# 運行 (dev 環境)；Clerk 顧客 application 的 publishable key 以 dart-define 帶入，不寫進程式碼，
# 沒有帶時只能以訪客使用
flutter run --flavor dev --dart-define=CLERK_PUBLISHABLE_KEY=pk_test_...

# 運行 (stage 環境)；API 主機沒有預設值，經 dart-define-from-file 提供
flutter run --flavor stage --dart-define-from-file=config/dart-defines.json

# 運行 (production 環境)
flutter run --flavor prod --dart-define-from-file=config/dart-defines.json
```

### 外部服務設定（Firebase、Sentry、API 主機）
這些值以編譯期 define 帶入，不寫進程式碼，也不進版控：複製 `config/dart-defines.example.json` 成
`config/dart-defines.json`（已被 `.gitignore` 排除）並填值，再以 `--dart-define-from-file=config/dart-defines.json` 帶入。

- `FIREBASE_*`：Firebase 專案的 iOS app 設定。必填欄位（API key、app id、sender id、project id）任一為空時，
  Firebase 整體停用：不初始化、不註冊推播、不送分析事件，App 仍可啟動與登入。iOS 另需把對應的
  `GoogleService-Info.plist` 放在 `ios/Runner/config/{dev,stage,prod}/`（不進版控）；沒有這個檔案時 build 不嵌入設定、
  原生端不呼叫 `FirebaseApp.configure()`。
- `SENTRY_DSN`：空值時 Sentry 不送任何事件。
- `GATEWAY_HOST`、`MEDIA_BASE_HOST`：API 與媒體主機；`dev` 沒有提供時連本機單體 `http://localhost:8080`，
  `stage` 與 `prod` 必須提供 `GATEWAY_HOST`，沒有預設值，缺少時 App 啟動前直接報錯。
- `PUBLIC_SITE_ORIGIN`：網頁公開網址，伺服器沒給分享連結時用來組分享連結；沒有提供時不開分享面板。
- `HELP_CENTER_LINE_ID`、`HELP_CENTER_LINE_URL`：幫助中心的客服 LINE 官方帳號；沒有提供時不顯示 LINE 入口。

## 建置發布

```bash
# iOS IPA（publishable key 用正式的 Clerk instance）
flutter build ios --release --flavor prod --dart-define=CLERK_PUBLISHABLE_KEY=pk_live_...
```

## 更新紀錄

### 2026-02-28
- 新增 Flavor 系統（dev/stage/prod）統一入口 `main.dart`，移除 `main_test.dart`、`main_stage.dart`、`main_production.dart`
- 升級 Google Sign-In 至 v7，共用 Firebase 專案配置
- 新增 HKLog 統一日誌工具
- 全面遷移所有 API 端點至 OpenAPI 規格
  - 認證: `/v1/auth/*` → `/public/auth/*`, `/app/auth/*`
  - 會員: `/member/*` → `/app/buyers/me`
  - 商品: `/v1/products`, `/product/skuinfo/*` → `/public/products`
  - 搜尋: `/v1/search/*` → `/public/search/*`
  - 商家: `/v1/sellers/*`, `/v1/merchant/*` → `/public/sellers/*`
  - 購物車: `/cart/cartItem/*` → `/public/carts/*`
  - 訂單: `/v1/orders/*` → `/app/orders`
  - 關注: `/fan/fans/follow` → `/app/buyers/me/following`
  - 收藏: `/fan/favorites` → `/app/buyers/me/favorites`
  - 瀏覽紀錄: `/visit/*` → `/app/buyers/me/history`
  - 通知: `/v1/notifications/*` → `/app/buyers/me/notifications/*`
  - 聊天: `/v1/app/chat/*` → `/app/chat/*`
  - 社群: `/blog/*` → `/public/posts/*`
  - 留言板: `/message/*` → `/app/sellers/forum`
  - 裝置: `/v1/devices/*` → `/app/devices`
  - Banner: `/v1/banners` → `/public/banners`
  - 分類: `/v1/public/categories/*` → `/public/categories/*`
  - 地址: `/v1/app/buyers/me/addresses` → `/app/buyers/me/addresses`
  - 評論: `/v1/reviews/*` → `/app/reviews`, `/public/products/*/reviews`
- UI 對齊設計稿 (個人資料設定、我的收藏頁)
- 加入延遲骨架屏載入 (300ms 後才顯示 skeleton)
- 修復缺失的 asset 圖片

### v1.0.0 (2025-12-22)
- 初始版本
- 整合阿里雲 OSS 圖片服務
- 支援正式分類與候選分類查詢

## 授權

本專案為私有專案，版權所有 © 2025-2026 Hawkr
