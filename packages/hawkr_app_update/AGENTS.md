# hawkr_app_update

兩支 Flutter App 共用的 App 版本門檻（強制更新）檢查邏輯。畫面、路由攔截與 provider 留在各自的 App。

- 行為規格：`docs/spec.md`（檢查規則、兩支 App 的接線、更新畫面、版本 header）。後端的規格在 `backend/docs/modules/content.md`。
- Flutter 版本見 `.fvmrc`。
- 驗證：`fvm flutter pub get`、`fvm dart analyze`、`fvm dart format --output=none --set-exit-if-changed lib test`、`fvm dart test`（套件只依賴 `test`）。
