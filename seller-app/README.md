# Hawkr Seller (商家端)

Hawkr Seller 是 Hawkr 平台的商家端 Flutter App，提供商品管理、訂單處理、客戶經營、社群貼文、客服聊天與金流認證等功能。

## 共用元件庫

[賣家 App 元件庫文件](docs/component-library.md) 提供真實 widget 的 catalog、Dart API、啟動與測試方式；[管理後台 UI 元件庫](https://admin.example.com/admin/documentation) 可對照買家、賣家與 Web。

共用元件改動應在同一次 PR 更新正式 widget、catalog、API 文件與測試，再由 Web 的文件同步及版本發布流程整合。UI 元件庫版本與手機 App 發行版本獨立；既有 artifact 不覆寫，文件更新不會自動發布手機 App。

**版本**: 1.0.1 (build_number 於打包時透過 git commit 數動態推導)
**Flutter SDK**: 3.41.2（使用 FVM 管理；Dart 設定區間為 >=3.8.0 <4.0.0）
**支援平台**: iOS / Web

## 快速開始

### 環境準備

1. 本專案使用 FVM 進行 SDK 版本管理，請確保已安裝 `fvm`。
2. 執行 `fvm install` 安裝 `.fvmrc` 指定的 Flutter 版本（目前 3.41.2），再 `fvm flutter pub get` 載入依賴套件。

> [!IMPORTANT]
> **一律透過 `fvm flutter` 執行**，或設 `fvm global 3.41.2` 讓 `flutter` 指向 pin 的版本。
> 若系統另裝了全域 Flutter（例如 Homebrew cask），它會在 `PATH` 中蓋過 FVM，導致用「錯版本的 engine 去編譯本專案的 framework」。iOS build 會炸出一堆 **Flutter SDK 內部**錯誤（如 `Type 'ui.SemanticsHitTestBehavior' not found`、`PlatformDispatcher.lineHeightScaleFactorOverride` 不存在、`ImageFilter.bounds` not found）——這是**版本錯配**，不是你的 code 有問題。
> 用 `which flutter` + `flutter --version` 確認解析到的是 FVM 版本（路徑含 `fvm/`、engine 與 framework 同版）。被全域 flutter 污染後，`fvm flutter clean` 可把 `ios/Flutter/Generated.xcconfig` 的 `FLUTTER_ROOT` 重生回 FVM。

### 多環境 (Flavor) 建構

本專案使用 4 個環境，透過 flavor 切換：

| Flavor | Gateway Host | 用途 |
| --- | --- | --- |
| `dev` | 預設 `http://localhost:8080`（本機單體），可用 `GATEWAY_HOST` 覆蓋 | 開發環境 |
| `feDev` | 同 `dev`，用 `GATEWAY_HOST` 指向前端開發環境 | 前端開發環境 |
| `stage` | 必須以 `GATEWAY_HOST` 提供，沒有預設值；缺少時 `make` 拒絕建置、App 啟動時直接報錯 | 預發布環境 |
| `prod` | 必須以 `GATEWAY_HOST` 提供，沒有預設值；缺少時 `make` 拒絕建置、App 啟動時直接報錯 | 生產環境 |

### 外部服務設定（Firebase、Sentry、API 主機）

這些值都以編譯期 define 帶入，不寫進程式碼，也不進版控。複製 `config/dart-defines.example.json` 成
`config/dart-defines.json`（已被 `.gitignore` 排除）並填值，`make` 的目標偵測到這個檔案就以
`--dart-define-from-file` 帶入；直接用 `fvm flutter run` 時自行加上
`--dart-define-from-file=config/dart-defines.json`。

- `FIREBASE_*`：Firebase 專案的 iOS app 設定。必填欄位（API key、app id、sender id、project id）任一為空時，
  Firebase 整體停用：不初始化、不註冊推播、不送分析事件，App 仍可啟動與登入。iOS 另需把對應的
  `GoogleService-Info.plist` 放在 `ios/Runner/config/<flavor>/`（不進版控）；沒有這個檔案時 build 會略過嵌入、
  App 不呼叫原生 `FirebaseApp.configure()`。
- `SENTRY_DSN`：空值時 Sentry 不送任何事件。
- `GATEWAY_HOST`、`MEDIA_BASE_HOST`：API 與媒體主機；`dev` 沒有提供時連本機單體。
- `HELP_CENTER_LINE_ID`、`HELP_CENTER_LINE_URL`：幫助中心的客服 LINE 官方帳號；沒有提供時不顯示 LINE 入口。

**Native 建構** (iOS):
> [!NOTE]
> 原生層僅有 `dev`、`stage`、`prod` 三種 flavor。`feDev` 環境需要藉由 `dev` 原生外殼並在 Dart 層注入參數來啟動。

Clerk 顧客 application 的 publishable key 以 `--dart-define=CLERK_PUBLISHABLE_KEY=pk_...` 帶入（`make` 的目標用
`CLERK_PUBLISHABLE_KEY=...` 變數），不寫進程式碼；沒有帶時登入顯示「登入服務尚未設定」。

```bash
# 測試環境 (dev)
fvm flutter run --flavor dev --dart-define=FLUTTER_FLAVOR=dev --dart-define=CLERK_PUBLISHABLE_KEY=pk_test_...

# 前端開發環境 (feDev)
fvm flutter run --flavor dev --dart-define=FLUTTER_FLAVOR=feDev

# 預發布環境 (stage)
fvm flutter run --flavor stage --dart-define=FLUTTER_FLAVOR=stage
```

**Web 建構** (flavor 透過 `--dart-define` 傳入):
```bash
fvm flutter build web --dart-define=APP_FLAVOR=feDev
```

### Firebase 配置

每個 flavor 有獨立的 Firebase 設定：
- iOS: `ios/Runner/config/{dev,stage,prod}/GoogleService-Info.plist`（不進版控）
- Dart: 編譯期 define `FIREBASE_*`，範本 `config/dart-defines.example.json`，讀取在 `lib/config/firebase_config.dart`

### 品質閘門

```bash
fvm dart format --output=none --set-exit-if-changed lib test integration_test tool
fvm flutter analyze --no-pub
fvm dart run custom_lint
fvm flutter test --no-pub
bash scripts/deadcode.sh
```

PR 會另外執行 coverage ratchet 與離線 production-router smoke；quarantine 規則見
[`docs/testing_strategy.md`](docs/testing_strategy.md)。

## 專案結構

```
lib/
├── main.dart                  # 入口：讀取 flavor → 初始化 Firebase → 啟動 App
├── app.dart                   # MyApp：Material3 主題、zh_TW 本地化、新訊息 toast
├── config/
│   ├── flavor.dart            # Flavor enum (dev/feDev/stage/prod) + 全域存取器
│   ├── flavor_extension.dart  # 各環境 URL 設定 (gateway, API namespaces)
│   └── firebase_config.dart   # Firebase 編譯期 define（缺值時停用）
├── models/                    # 資料模型 (API 回應、訂單、商品、貼文、使用者等)
├── screens/                   # 頁面 (76 個畫面)
├── services/                  # 後端溝通 & 業務邏輯 (23 個服務)
├── widgets/                   # 共用元件
│   └── rich_text_editor/      # 圖文編輯器 (支援文字+圖片混排)
└── utils/                     # 工具類 (platform_io, hk_log, error_handler 等)
```

## API 架構

所有 API 請求透過 **Gateway** 統一路由，採用三層命名空間：

| 命名空間 | 前綴 | 認證 | 說明 |
| --- | --- | --- | --- |
| Public | `/api/v1/public/` | 匿名 | 公開資料 (商品瀏覽、分類查詢) |
| App | `/api/v1/app/` | JWT 必要 | 一般功能 (商品管理、訂單、聊天) |
| Admin | `/api/v1/admin/` | 管理員 | 管理後台 |

### 認證流程

1. 以 Clerk 登入（顧客 application）：Google、LINE 經系統瀏覽器，Apple 用 Sign in with Apple；Clerk 的登入狀態存在 Keychain
2. 登入後呼叫 `IdentityAppService/GetSession`（Connect）取得帳號，第一次登入時伺服器建立帳號
3. 每次請求以 Clerk 的 session token 帶 `Authorization: Bearer <token>`（SDK 自動換新），另帶 `X-Active-Role: SELLER`
4. Apple 原生登入且 GetSession 成功之後，背景呼叫 `LinkAppleCredential` 把憑證交給伺服器（失敗不影響登入）；刪除帳號時由伺服器撤銷 App 的授權，App 端不需要額外步驟

### 圖片與媒體

圖片一律經 Cloudflare R2 + image-gateway Worker（後端 49 號設計書）：

1. **上傳**: `MediaRepository.uploadWithSession` (`lib/features/media/data/media_repository.dart`) → CreateSession → PUT 到後端回傳的 presigned URL → CompleteSession → 取得 stagedToken
2. **公開讀取**: `buildImageUrl(objectKey, preset)` (`lib/config/media_image.dart`) 是唯一組裝入口，產出 `<MEDIA_BASE_URL>/i/<preset>/<object_key>`；域名跟隨 flavor 自動切換 (`lib/config/flavor.dart`)，**禁止硬編網域**
3. **私有檔**: `tmp/` / `secure/` 前綴的物件 Worker 直接回 404，需由後端發預簽名 URL，再以 `ProtectedMediaRemoteService.fetchBytes` 取回 bytes

## 主要畫面

### 登入與驗證

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 歡迎頁 | `welcome_screen.dart` | App 入口，自動登入檢查 |
| 登入/註冊 | `auth_screen.dart` | Google、Apple、LINE 登入按鈕，背景農場插畫 |

### 主導航

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 主導航 | `main_navigation_screen.dart` | 底部導航列：首頁/訂單/商店/個人 |
| 儀表板 | `dashboard_screen.dart` | 商家首頁：銷售統計、待處理訂單 |
| 個人資料 | `profile_screen.dart` | 商家資訊、設定入口 |
| 設定 | `settings_screen.dart` | App 設定、帳號管理 |

### 商品管理

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 商品選擇器 | `product_picker_screen.dart` | 從目錄選取商品加入店面 |
| 商品編輯 | `product_edit_screen.dart` | 編輯已有商品資訊 |
| 商品資訊設定 | `product_info_setup_screen.dart` | 新增商品：名稱、分類、產地、描述 |
| 商品圖片設定 | `product_image_setup_screen.dart` | 商品照片上傳 (最多 10 張) |
| 商品規格設定 | `product_spec_setup_screen.dart` | 規格等級、價格、庫存設定 |
| 商品標籤設定 | `product_tag_setup_screen.dart` | 商品標籤管理 |
| 商品預覽 | `merchant_product_preview_screen.dart` | 買家視角預覽，含圖片輪播、規格選擇、評價 |
| 商品評價 | `product_reviews_screen.dart` | 查看顧客評價與評分 |

### 訂單處理

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 搶單頁 | `map_offer_screen.dart` | 待搶單/待出貨/已完成三分頁 |
| 訂單列表 | `order_list_screen.dart` | 全部訂單瀏覽 |
| 訂單媒合 | `order_matched_screen.dart` | 已媒合訂單管理 |
| 訂單詳情 (配送) | `order_details_delivering_screen.dart` | 配送中訂單追蹤 |
| 訂單詳情 (付款) | `order_details_payment_screen.dart` | 付款資訊 |
| 訂單詳情 (收據) | `order_details_receipts_screen.dart` | 收據/發票 |
| 訂單詳情 (評價) | `order_details_rate_screen.dart` | 訂單評價 |
| 售後服務 | `after_sales_screen.dart` | 退換貨處理 |
| 物流追蹤 | `logistics_tracking_screen.dart` | 配送進度追蹤 |

### 商家專頁 & 社群

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 商家專頁 | `page_screen.dart` | Home/Shop/Echo/Chat 四分頁 |
| 首頁動態牆 | `page_home_feed_screen.dart` | 貼文九宮格 → 全螢幕瀏覽 |
| 全螢幕動態 | `page_full_screen_feed_screen.dart` | 貼文全螢幕滑動瀏覽 |
| 發文 | `post_compose_screen.dart` | 圖文/影片貼文撰寫 |
| 貼文詳情 | `post_detail_screen.dart` | 留言、按讚、分享 |
| 媒體選擇 (貼文) | `post_media_picker_screen.dart` | 選擇貼文圖片/影片 |
| 媒體選擇 (商品) | `merchant_media_picker_screen.dart` | 選擇商品圖片 |

### 客戶經營

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 客戶管理 | `customer_management_screen.dart` | 客戶列表、標籤篩選 |
| 客戶詳情 | `customer_detail_screen.dart` | 個別客戶資訊、訂單歷史 |
| 消費紀錄 | `customer_spending_history_screen.dart` | 客戶消費統計 |

### 聊天客服

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 聊天列表 | `chat_list_screen.dart` | 客服對話列表 |
| 聊天對話 | `chat_thread_screen.dart` | 即時訊息對話 |
| 嵌入式聊天列表 | `embedded_chat_list_panel.dart` | 嵌入商家專頁的聊天面板 |
| 嵌入式聊天對話 | `embedded_chat_thread_panel.dart` | 嵌入式對話視窗 |

### 金流與認證

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 錢包 | `wallet_screen.dart` | 認證閘門、PayUNi 審核狀態面板、帳戶與認證（食品業者登錄補件）、前往提領（系統內建瀏覽器開 PayUNi 登入頁）、優惠券 |
| 商家認證 | `merchant_certification_screen.dart` | 認證申請列表 |
| 認證文件上傳 | `merchant_certification_upload_screen.dart` | 上傳認證文件 (身分證、存摺等) |
| 商家資訊 | `merchant_information_screen.dart` | 商家基本資料 |

### 圖片編輯工具

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 圖片編輯器 | `image_editor_screen.dart` | 裁切、濾鏡、文字、貼圖 |
| 商品照片工具 | `merchant_photo_tools_screen.dart` | 商品拍照輔助工具 |
| 照片編輯器 | `photo_editor_screen.dart` | 進階照片編輯 (標章、單價、免運標籤) |

### 通知

| 畫面 | 檔案 | 說明 |
| --- | --- | --- |
| 通知列表 | `notifications_screen.dart` | 系統通知、訂單更新 |
| 通知設定 | `notification_settings_screen.dart` | 推播開關設定 |

## 核心服務

| 服務 | 檔案 | 說明 |
| --- | --- | --- |
| API 呼叫 | `api_service.dart` | 統一 HTTP 呼叫入口 (使用 http 套件) |
| HTTP Client | `dio_manager.dart` | Dio 實例管理 (攔截器、token 注入) |
| 認證儲存 | `auth_storage.dart` | JWT / Session 持久化 |
| Session | `session_store.dart` | 當前登入狀態 (userId, merchantId, token) |
| 登入 | `clerk_session_provider.dart`、`oauth_picker.dart` | Clerk 的登入狀態與 session token、系統瀏覽器與 Sign in with Apple |
| 聊天 | `chat_service.dart` | WebSocket 即時聊天 |
| 訂單 | `order_service.dart` | 訂單 CRUD、搶單、狀態更新 |
| 商品上傳 | `product_upload_service.dart` | 商品建立/更新流程封裝 |
| 媒體上傳 | `media_upload_service.dart` | 新版上傳 (gRPC 上傳會話 → OSS presigned PUT) |
| OSS 服務 | `oss_service.dart` | 圖片讀取 URL 產生 (presigned URL) |
| 圖片處理 | `image_processing_service.dart` | 壓縮、格式轉換 |
| AI 圖片 | `ai_image_service.dart` | Gemini AI 圖片處理 |
| OCR | `apple_vision_ocr_service.dart`, `id_card_ocr_service.dart`, `bank_book_ocr_service.dart` | 證件辨識 |
| 推播 | `push_notification_manager.dart`, `push_notifications.dart` | FCM 推播通知 |
| 商家頭像 | `merchant_avatar_helper.dart` | 頭像載入 (含 presigned URL) |
| 訂單狀態 | `order_status_notifier.dart` | 訂單即時狀態更新 |
| PayUNi 狀態 | `payuni.dart`、`seller_status_refresh.dart` | `GetMyKYC` 的狀態（含 `flow.payuni`）；推播只觸發重讀 |

## 共用元件

| 元件 | 檔案 | 說明 |
| --- | --- | --- |
| 圖文編輯器 | `rich_text_editor/` | 商品描述圖文混排編輯 (文字 + 圖片 blocks) |
| 底部導航列 | `app_bottom_navigation_bar.dart` | 全域底部導航 |
| 商家頭像 | `merchant_avatar_widget.dart` | 跨頁面共用的商家頭像元件 |
| 留言面板 | `comments_bottom_sheet.dart` | 貼文留言底部彈出面板 |
| 環境標籤 | `environment_banner.dart` | 非 production 環境顯示提示 |
| 載入動畫 | `truck_loading_overlay.dart` | 卡車載入動畫全螢幕遮罩 |
| 商品設定進度 | `product_setup_progress_bar.dart` | 商品新增步驟進度條 |

## 主要依賴

| 類別 | 套件 |
| --- | --- |
| HTTP | `http`, `dio` |
| Firebase | `firebase_core`, `firebase_auth`, `firebase_messaging` |
| 登入 | `hawkr_api`（`clerk_auth`）, `flutter_web_auth_2`, `sign_in_with_apple` |
| Google | `google_generative_ai` |
| 媒體 | `image_picker`, `flutter_image_compress`, `photo_manager`, `video_player`, `cached_network_image` |
| UI | `flutter_svg`, `rive`, `liquid_glass_easy`, `font_awesome_flutter` |
| 工具 | `shared_preferences`, `uuid`, `geolocator`, `url_launcher`, `crypto` |
