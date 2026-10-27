# App 版本檢查與強制更新（App Version Gate）的架構決策

| 項目 | 值 |
|---|---|
| 狀態 | accepted（D8 與 D11 的 `X-App-Platform` 已 superseded by `2026-09-26-app-ios-only.md`；D10 superseded by `2026-09-27-app-version-gate-shared-package.md`） |
| 日期 | 2026-09-18 |
| 影響範圍 | `content-service`（2026-09-26 起是單體的 content 模組）／Gateway／`hawkr_buyer`／`hawkr_seller`／`hawkr-admin` |

逐欄規格見 `docs/詳細設計書/54-App 版本檢查與強制更新（App Version Gate）.md`。
本檔只保存「為什麼選這個、否決了什麼」。

## D1 設定存哪：新建專用表 `app_version_gates`

| 方案 | 代價 | 判定 |
|---|---|---|
| 複用 content-service `site_contents` key-value | `site_contents` 是文案 key-value，把結構化 JSON 塞進 `content_text` ＝ 欄位不誠實、無法用 DB 約束驗證 | ❌ |
| **新建專用表** | 多一張表 | **✓** 可加 `CHECK` 與複合主鍵，把不變式交給 DB 守 |

## D2 承載服務：content-service

> 2026-09-26：content-service 搬進單體後由 content 模組承載（遷移計畫 A1），理由不變。

content-service 是既有「站台營運可維護資料」的承載者（`site_contents`／banner／recipe
皆有 admin CRUD，三層落點可直接照抄）。Gateway 無 DB；account／notification 域不符。

**已登記的代價**：它因此成為每次 App 冷啟動的依賴（詳細設計書 §9.2）。

## D3 版本比較：semver 三段、後端獨佔

| 方案 | 代價 | 判定 |
|---|---|---|
| build number 參與比較 | build number 跨平台不一致（Android 走 `git rev-list --count`、iOS Xcode Cloud 走 run 序號），拿它比較必然出錯 | ❌ |
| App 端比較 | 比較邏輯兩端各一份 ＝ 兩個真實來源；兩 App 又各自複製 ＝ 三份，必然漂移 | ❌ |
| **只比 semver ＋ 後端獨佔** | App 端拿不到「差幾版」這類資訊 | **✓** 單一 SSOT |

附帶好處：不需要 `pub_semver` 依賴——它在兩 repo `pubspec.lock` 均為 transitive 依賴，
直接 import 會踩 `depend_on_referenced_packages` lint。

## D4 只做強制更新，不預留 `latestVersion`

需求只要求強制；預留無消費者的欄位 ＝ 零消費抽象。
本 repo 已有 `can_accept_orders` 抽象做了卻全 repo 零消費的教訓
——設計意圖從未接線，比沒做更糟。

## D5 不做 Gateway 逐請求版本攔截

**理由不是成本**——Gateway 已 wire go-redis 且有逐請求 Redis 檢查的現成先例
（`pkg/blocklist`、`pkg/ratelimit`），要做並不貴。真正理由有三：

1. App 非敵對環境，使用者不是攻擊者，只是還沒更新
2. App 端「冷啟動 ＋ 前景恢復」兩檢查點已覆蓋含「放量當下正在使用」的場景
3. 伺服端硬擋卡在**判別不出舊 App**（詳細設計書 §3.1 第 2 點），先做也擋不準

🔴 **重新評估的前置條件**：組合判準（Origin allowlist ＋ path namespace ＋ header 缺席）
的可行性必須先實測。沒有這一步不要重開 D5。

## D5b gate 置於 `_bootstrapApplication` 開頭，不另建推播抑制機制

`_initializeFirebaseMessaging()` 的呼叫點在 `_bootstrapApplication` 內，由
`BrandStartupGate` 在 widget `initState` 之後才 await ⇒ gate 放在 bootstrap 最前面時，
推播導航根本還沒 arm，不需要抑制機制。

## D6 API 失敗一律 fail-open，且用獨立短 timeout 3 秒

後端故障不得鎖死所有使用者；且後端不可用時 App 本來就無法運作，fail-open 不擴大實際可用面。

**為什麼不沿用共用 Dio 的 timeout**：那是 30 秒，弱網下啟動畫面會卡 30 秒才 fail-open。

## D7 商店連結存 DB 且 **nullable**

> 2026-09-26：單體以空字串表示沒有設定（`NOT NULL DEFAULT ''`，「沒有設定」只有一種表示法，見 content 模組
> `CARRYOVER.md` 的沿用盤點）。本條的重點不變：不逼出佔位網址，App 拿到空值時隱藏按鈕。

| 方案 | 代價 | 判定 |
|---|---|---|
| 硬編碼在 App | repo 內查無 App Store 數字 App ID ⇒ 會逼出「找不到就寫死錯的」 | ❌ |
| 存 DB 且 `NOT NULL` | 會逼 seed 填佔位字串，App 端照導 ⇒ 使用者撞 404，**比沒有按鈕更糟** | ❌ |
| **存 DB 且 nullable** | admin 要記得填 | **✓** App 端拿到空值時隱藏按鈕並改顯示文字指引，不導向任何猜測 URL |

## D8 沿用既有 `AppType`，只新增 `AppPlatform`

> superseded by `2026-09-26-app-ios-only.md`：App 只發 iOS，平台維度拿掉。

新建一個與 `AppType` 同語意的 `AppClient` ＝ **同義 enum 分叉**，違反 CodeMeta SSOT。
`ClientType`(MOBILE_APP/WEB_BROWSER) 粒度只到「App vs 瀏覽器」、分不出 iOS/Android，
故 `AppPlatform` 確為新增而非重複——三者是**正交維度，不該合併**。

## D9 不新增 gate 專用業務錯誤碼

檢查結果以 `data.updateRequired` 表達，是正常回應而非錯誤路徑。
走錯誤碼會讓 Gateway 業務碼塌縮邏輯與前端解析都變複雜，且語意不對。

## D10 兩 App 各自實作一份，不抽 shared package、不做 CI checksum 對拍

> superseded by `2026-09-27-app-version-gate-shared-package.md`：monorepo 之後前提不成立，檢查邏輯改成共用的 `packages/hawkr_app_update`。

| 方案 | 代價 | 判定 |
|---|---|---|
| 抽 shared package／monorepo 化 | 兩 repo 無 shared package、無 workspace，超出範圍 | ❌ |
| **CI checksum 對拍** | 跨 repo CI 需要另一 repo 的 checkout 憑證與同步時序；且 Flutter 側 CI gate 只掛 `pull_request`、兩 App 走直接 push dev ⇒ **擋不到** | ❌ 已評估並否決 |
| **各自實作一份** | 靠紀律對齊 | **✓** 以詳細設計書為 SSOT ＋ §4.6 兩 repo 實作對照表 |

⚠️ 這個決策把對齊責任交給了紀律。緩解手段是 §4.6 的對照表與 §4.6.2 的必測不變式，
**不是**「CI 會抓到」。

## D11 App 對**所有**後端請求帶版本 header

> `X-App-Platform` 部分 superseded by `2026-09-26-app-ios-only.md`：只帶 `X-App-Version` 與 `X-App-Type`。

消費者是**可觀測性**：Gateway access log 記錄版本後，才答得出「線上各版本 App 的請求
佔比」「新版放量後舊版是否歸零」。

🔴 **價值不包含「單憑 header 缺席就能硬擋舊 App」**——見 D5 與詳細設計書 §3.1 第 2 點。
