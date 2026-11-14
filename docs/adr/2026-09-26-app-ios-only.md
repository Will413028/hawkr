# App 只發 iOS：拿掉平台維度與 UI 的平台分支，Google 登入統一以 web client id 為 audience

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-26 |
| 影響範圍 | content 模組（版本門檻）／`api/` content v1alpha1／CodeMeta／舊 gateway 的 access log／account-service 的 Google audience／buyer-app、seller-app（含 UI 的平台分支與測試）／admin 版本管理頁 |

取代 `2026-09-18-app-version-gate-decisions.md` 的 D8（新增 `AppPlatform`）與 D11 裡的 `X-App-Platform` header。

## 脈絡

App 不再開發 Android（Will 決定，2026-09-26）；Android 的程式、`APP_PLATFORM_ANDROID` 與 Android 的 Google client 已在 `030bc512b` 刪除。

約束逐條標來源：
- 外部：系統還沒上線，沒有線上使用者、資料或已發布的 App 版本需要相容。
- 繼承：D8 新增 `AppPlatform` 的理由是「`ClientType` 分不出 iOS／Android」；只剩 iOS 後不再成立，拿掉。
- 繼承：seller 傳 `serverClientId`、buyer 不傳，原因是 Android 的 google_sign_in 需要 `serverClientId` 才回 idToken；Android 不在了，這個差異只剩兩套做法，拿掉。
- 繼承：App 的轉場、登入頁的 Apple 按鈕、auth 路由、重新付款依 `TargetPlatform` 分支，非 Apple 平台走網站風格的轉場、隱藏 Apple 按鈕；那是為 Android 準備的，現在只剩 flutter_test 的預設平台（android）會走到，測試因此大多驗不出貨的分支。
- 外部：flutter_test 在測試環境把 `defaultTargetPlatform` 固定成 android，`debugDefaultTargetPlatformOverride` 在每個 testWidgets 本體結束、tearDown 之前就被 invariant 檢查，無法用 `flutter_test_config.dart` 全域改成 iOS（Flutter 3.41 的 `foundation/_platform_io.dart`、`flutter_test/lib/src/binding.dart`）。
- 繼承：`api/` 是 v1alpha1（`api/buf.yaml` 的 `ignore_unstable_packages`），上線前可以直接做 breaking 修改。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：單一平台時，版本門檻以 App 類型為 key，不設平台欄位（YAGNI） | 之後若要支援第二個平台，要把維度加回來（proto 欄位、主鍵、admin） | ✓ |
| 保留 `AppPlatform` 當擴充點，只剩 `IOS` 一個值 | 必填欄位、主鍵的一半、每個請求的 header 都只為一個值存在；client 送寫死的常數 | ❌ |
| 基準：Google idToken 的 audience 固定為後端自己的 client（web client id，Google「Authenticate with a backend server」） | buyer 多傳一個 `serverClientId` | ✓ |
| 維持 buyer 的 aud 是 iOS client id、seller 是 web client id | allowlist 要同時收 iOS 與 web 兩種 id，兩支 App 做法不同 | ❌ |
| 基準：只出貨一個平台時，UI 不依平台分支，測試在任何主機都驗到出貨的行為 | seller 的 web build 也會用 Cupertino 轉場、顯示 Apple 登入 | ✓ |
| 逐支測試加 `TargetPlatformVariant.only(iOS)`，保留平台分支 | 程式留著只為非出貨平台存在的分支；新測試忘了加 variant 就又驗錯分支 | ❌ |

## 決策

- **D1** 版本門檻以 `app_type` 為 key：proto 拿掉 `AppPlatform` 與兩個 `platform` 欄位；`content.app_version_gates` 拿掉 `platform` 欄位，主鍵改成 `app_type`；CodeMeta 刪除 `AppPlatform`；App 不再送 `X-App-Platform`，舊 gateway 的 access log 不再記 `app_platform`。
- **D2** 兩支 App 的 Google 登入都傳 `serverClientId`，idToken 的 aud 是 web client id；account-service 的 allowlist 只收 `GOOGLE_OAUTH_{SELLER,BUYER}_WEB_CLIENT_ID`，auth contract 只剩 web 的 surface。
- **D3** App 的 UI 不再依 `TargetPlatform` 分支：轉場一律 Cupertino（網站風格的 `HkPageTransitionsBuilder`、`HkRouteMotionPolicy` 刪除）、登入頁一律顯示 Apple 登入、auth 路由一律 `CupertinoPageRoute`、重新付款交給 `PayTokenService` 判斷可用性。`kIsWeb`（web 用 web client id、不取 APNs token）與 `dart:io` 的 `Platform.isIOS`（原生外掛是否存在：Apple Pay、Vision OCR、版本檢查）保留，它們判斷的是執行環境能不能用，不是介面長相。

## 後果

- App 只在 iOS 上做版本檢查；web 與測試環境（非 iOS）一律放行，與原本相同。
- 舊 gateway 的 app-version 端點與 content-service 仍以 `(app_type, platform)` 識別，characterization 測試的 REST driver 固定送 `IOS`；它們隨 content-service 一起刪除。
- 推播裝置 token 與登入 API 的 `platform` 欄位仍在（只收 `ios`），搬 notification 進單體時拿掉。
- seller 的 web build 也用 Cupertino 轉場並顯示 Apple 登入（D3 接受的代價）。新的 UI 不要再依 `TargetPlatform` 分支；需要依執行環境判斷時用 `kIsWeb` 或 `dart:io` 的 `Platform`。
- 重新評估的條件：決定再支援第二個原生平台時，把平台維度加回來（新開 ADR）。
