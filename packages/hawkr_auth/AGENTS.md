# hawkr_auth

兩支 Flutter App 共用的 Google／LINE 原生登入（沒有 Apple 登入）與 identity session token 的 Keychain 保存。流程：`IssueAppLoginNonce` → 原生 SDK 取得帶 nonce 的 ID token → `ExchangeIdToken` → token 存 `flutter_secure_storage`，請求以 `Authorization: Bearer` 帶。`hawkr_api` 只提供 API 契約與 transport；此套件不依賴它，換發的 RPC 由 App 實作 `IdentityGateway`。

- Google 的 nonce 只能經 `GoogleSignIn.instance.initialize(nonce:)` 帶，所以每次登入先取新 nonce 再重新 initialize，之後才 `authenticate`；google_sign_in 文件寫 initialize 只呼叫一次，iOS 實作只覆寫設定，`test/auth_session_test.dart` 守住順序，升級 SDK 時重驗。
- 伺服器驗證的值：Google `aud` = `GOOGLE_WEB_CLIENT_ID`（App 傳 `serverClientId`）、`azp` 在 `GOOGLE_APP_CLIENT_IDS`（App 傳 iOS `clientId`）；LINE `aud` = channel ID，nonce 用 `LoginOption.idTokenNonce`。
- session token 只經 `SessionTokenStore`（預設 Keychain），`test/token_storage_guard_test.dart` 守住。

- Flutter 版本見 `.fvmrc`。
- 驗證：`fvm flutter pub get`、`fvm dart analyze`、`fvm dart format --output=none --set-exit-if-changed lib test`、`fvm flutter test`。
- URL scheme、Info.plist 與 client ID 設定由各 App 提供（見 App 的 README「登入設定」）；畫面與路由留在 App。
