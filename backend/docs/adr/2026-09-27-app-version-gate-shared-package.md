# App 版本門檻的檢查邏輯改成兩支 App 共用的套件

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-27 |
| 影響範圍 | `buyer-app`、`seller-app`、`packages/hawkr_app_update` |

取代 `2026-09-18-app-version-gate-decisions.md` 的 D10（兩 App 各自實作一份）。逐欄規格仍在設計書 54。

## 脈絡

- 繼承：D10 選各自實作，理由是兩個 repo 沒有共用套件與 workspace，跨 repo 的 CI 對拍擋不到。現在不成立：五個 repo 已合併成 monorepo，兩支 App 以 path 依賴共用的 `packages/hawkr_api`。
- 對齊只靠紀律（D10 自己登記的代價），實際已經分岔：buyer 的 repository 失敗時丟出錯誤、由 provider 接住，seller 的 repository 回 `null`；讀版本一邊是 async、一邊是同步的快取；失敗的 log 等級不同。兩邊的單元測試都沒有涵蓋這段規則。
- 設計審查（content 模組 A1 收尾）追出這條前提不成立；Will 2026-09-27 決定抽成共用套件。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 維持各自一份，把分岔對齊 | 規則（逾時、fail-open、節流的錨點）仍有兩份，改設計書 54 要改兩處，漂移會再發生 | 否決 |
| 放進 `packages/hawkr_api` | 那是對外 API 的生成物與 transport；版本門檻是設計書 54 的產品行為，改動的理由不同 | 否決 |
| **新增 `packages/hawkr_app_update`（純 Dart）** | 多一個套件與它的 CI | **採用** |

## 決策

- 共用的是檢查的規則：只在 iOS 檢查、讀不到自己的版本不檢查、整個呼叫最多 3 秒、失敗一律放行並保留上一次的結果、冷啟動不看節流、回到前景以上次**成功**的檢查為錨節流 10 分鐘（`VersionGate`）。
- 套件是純 Dart，不依賴 Flutter 與 riverpod：平台判斷、讀版本與 log 由 App 注入（buyer 要能編譯成 web，seller 經 `platform_io` 包裝），App 只留把結果放進 state 的 provider。
- 強制更新的畫面與路由的攔截留在各自的 App：兩邊的版面與 design token 不同，畫面的改動另外取得同意。
- 規則由套件以 fake transport 的單元測試守住，CI 是 `.github/workflows/dart-packages.yml`。

## 後果

- 設計書 54 §4.6 的對照表不再需要對照 repository 與 provider：規則只有一份。
- §4.6.1 另外兩個「必須逐位元組相同」的檔不在這次的範圍：`app_version_utils.dart` 依賴 `package_info_plus`（Flutter plugin），也被 `X-App-Version` header 使用；`brand_startup_splash.dart` 是畫面。要共用的話是另一個依賴 Flutter 的套件，另外決定。
