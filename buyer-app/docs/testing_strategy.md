# Buyer App 測試策略

這份策略規定每個 PR 都能重現的 gate。PR 測試不連 Hawkr API，也不能改動
購物車、訂單、追蹤、訊息或帳號資料。

## 執行層級

| 層級 | 觸發 | 命令／workflow | 外部依賴 | 失敗處理 |
| --- | --- | --- | --- | --- |
| Unit + widget + coverage | 每個 `dev` PR | `flutter test --no-pub --exclude-tags hermetic-integration --coverage --coverage-path coverage/lcov.info` | 無 | 阻擋 PR |
| Hermetic app integration | 每個 `dev` PR | `flutter test --no-pub --tags hermetic-integration test/integration` | 僅受控 fake | 阻擋 PR |

`test/integration/hermetic_app_smoke_test.dart` 使用 production `GoRouter`、
實際頁面與 Riverpod provider，但覆寫 session、Dio transport 與 Firebase
observer。Fake adapter 只接受白名單 GET；任何 POST、PUT、PATCH、DELETE 或未建
fixture 的 endpoint 都會直接失敗。這層可在 Linux `flutter-tester` 離線執行。

## Coverage ratchet

基準位於 `tool/coverage_baseline.json`，由當下完整 unit/widget suite 實測產生，
不是任意目標百分比。`tool/coverage_ratchet.dart` 會：

- 排除 `.g.dart`、`.freezed.dart`、`.mocks.dart`。
- 以整數分數比較行覆蓋率，禁止低於 `dev` base branch。
- 阻擋仍存在、但從 LCOV 消失的既有 instrumented source。
- 阻擋新增後完全沒有被任何測試載入的 production source。
- 輸出 `coverage/coverage-summary.json`，CI 同時保存 LCOV 14 天。

本機重現：

```sh
flutter test --no-pub \
  --exclude-tags hermetic-integration \
  --coverage \
  --coverage-path coverage/lcov.info
dart run tool/coverage_ratchet.dart \
  --lcov coverage/lcov.info \
  --baseline tool/coverage_baseline.json
```

只有在測試範圍確實增加或 production source 合理移除時才更新 baseline。先以
`--print-baseline` 產生候選內容，review instrumented/source 清單及 line counts
後再取代檔案。PR workflow 仍會讀 base branch baseline 作 immutable floor，所以
同一個 PR 不能藉由調低 checked-in baseline 繞過 gate。

## 關鍵路徑矩陣

| Domain | Repository／contract | Controller／state | Widget／route |
| --- | --- | --- | --- |
| Auth | `auth_repository_identity_test.dart` | `auth_controller_test.dart`, `session_provider_test.dart` | `auth_oauth_pill_layout_test.dart`, hermetic guest guard |
| Cart | `cart_add_error_test.dart` | `cart_controller_state_test.dart` | `cart_surface_rwd_test.dart` |
| Order | `order_repository_controller_test.dart`, `order_backend_contract_test.dart` | `order_repository_controller_test.dart` | `order_card_redesign_test.dart`, `order_detail_redesign_test.dart` |
| Notification | `notification_repository_error_test.dart` | `notification_controller_error_test.dart` | `notification_pagination_error_widget_test.dart`, `notification_router_cold_start_test.dart` |
| Pagination | `home_product_repository_test.dart`, notification repository tests | `home_product_controller_test.dart`, notification/order controller tests | `home_product_section_rwd_test.dart`, notification pagination widget |

每個新 critical flow 至少要在 repository/contract、controller/state、
widget/route 三欄中各有一個可辨識的測試；若該層不存在，PR 說明要明確寫出原因。

## Flaky quarantine

目前沒有 quarantine 或 blanket skip。測試不可只寫 `skip: true` 後合併。若真有
環境型 flaky，必須放入獨立 quarantine job，並在測試名稱旁記錄：

- owner
- tracking issue
- 可重現環境
- 移除條件
- 最晚移除日期

Quarantine 仍需定期執行並回報結果，且不能取代相同風險的 hermetic PR gate。
