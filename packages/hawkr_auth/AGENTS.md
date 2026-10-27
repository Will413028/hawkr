# hawkr_auth

兩支 Flutter App 共用的 Clerk session、Keychain 持久化與 Google／LINE／Apple 原生登入接線。`hawkr_api` 只提供 API 契約與 transport；此套件不依賴它。

- Flutter 版本見 `.fvmrc`。
- 驗證：`fvm flutter pub get`、`fvm dart analyze`、`fvm dart format --output=none --set-exit-if-changed lib test`、`fvm flutter test`。
- App 專屬的 OAuth redirect URL 由各 App 提供；畫面與路由留在 App。
