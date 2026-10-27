# Seller App 測試策略

這份策略規定每個 PR 都能重現的 gate。PR 測試不連 Hawkr API，不建立或修改
商品、訂單、物流、貼文、訊息與帳號。

## 執行層級

| 層級 | 觸發 | 命令／workflow | 外部依賴 | 失敗處理 |
| --- | --- | --- | --- | --- |
| Unit + widget + coverage | 每個 `dev` PR | `flutter test --no-pub --exclude-tags hermetic-integration --coverage` | 無 | 阻擋 PR |
| Hermetic app integration | 每個 `dev` PR | `flutter test --no-pub --tags hermetic-integration test/integration` | 僅封閉 fake | 阻擋 PR |

`test/integration/hermetic_app_router_smoke_test.dart` 直接渲染 production
`MyApp` 與 production `GoRouter`，覆寫 Firebase observer、登入後副作用和 Dio
transport。Fake adapter 只接受具名 GET fixture；任何 POST、PUT、PATCH、DELETE
或未知 endpoint 都立即失敗。因此可在 Linux `flutter-tester` 離線驗證未登入
cold start、加密 session 還原、redirect 與 dashboard render。

## Coverage ratchet

基準位於 `tool/coverage_baseline.json`，必須由完整 unit/widget suite 的 LCOV
實測產生，不使用任意目標百分比。`tool/coverage_ratchet.dart` 排除 `.g.dart`、
`.freezed.dart` 與 `.mocks.dart`，以未四捨五入的行數比例比較並阻擋：

目前基準為 8,865 / 46,757 行（18.96%），涵蓋 371 個 production files；另有
18 個 legacy-uninstrumented sources 明列於 baseline，不能在 PR 中任意增加。

- 當前 coverage 低於 checked-in baseline。
- PR 內 baseline 低於 base branch 的 immutable floor。
- production source 增加，卻沒有相應增加被測試載入的 source。
- 原本會被載入的 production source 從 LCOV 消失。

CI 會保存 LCOV 與 JSON summary 14 天。只有在測試覆蓋確實增加或 production
source 合理移除時才更新 baseline；同一個 PR 不能靠調低 baseline 放行。

## 關鍵路徑矩陣

| 風險 | Repository／contract | Controller／state | UI／route |
| --- | --- | --- | --- |
| Auth/session restore | `auth_repository_test.dart` | `login_flow_test.dart` | hermetic production-router smoke |
| 出貨失敗與 rollback | `cvs_shipping_mutation_test.dart`、`payuni_logistics_service_test.dart` | shipping mutation tests | `shipping_settings_responsive_test.dart` |
| FCM register/retry | `device_registration_test.dart` | typed registrar state tests | — |
| Route restore | `route_payload_test.dart` | revision/race tests | production router inventory + smoke |
| Orders/notifications | backend contract tests | provider/controller tests | order/notification widget tests |

`test/config/testing_strategy_policy_test.dart` 會鎖住這些代表性測試與 skip 的
quarantine 規則。

## Flaky quarantine

目前 `tool/test_quarantine.json` 為空，沒有 blanket skip。新增 `skip` 前必須填：
檔案、測試名稱、owner、tracking issue、可重現環境、移除條件與最晚移除日期。
Quarantine 仍需在獨立 job 定期執行，不能取代同一風險的 hermetic PR gate。
