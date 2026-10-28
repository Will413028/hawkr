# App 版本門檻 Runbook

行為規格：後端 `docs/modules/content.md` §5，App 端 `packages/hawkr_app_update/docs/spec.md`。本文只放出事時的排查，以及版本分佈的讀法與舊版 App 的放行判準。

## 1. 排查

| 症狀 | 排查順序 |
|---|---|
| 強更沒生效（改了設定，App 照樣進得去） | ① 直接呼叫 `CheckAppVersion` 確認 `update_required` ② 確認 App 送的 `client_version` 是 bare semver（`1.0.3`），不是顯示字串 `1.0.3(7)`：格式不對時後端放行，強更靜默不觸發 ③ 確認該 `app_type` 真的有一組設定：沒有設定時放行 ④ 確認 App 端的節流：回到前景距上次成功的檢查不滿 10 分鐘不會重檢，切到背景 10 分鐘以上再回來，或冷啟動 ⑤ 確認該來源 IP 沒有被限流（429）：被限流屬於檢查失敗，App 放行，兩端都看不出差別；同一個出口 IP 下的其他使用者也會用掉同一個配額，不限於自家流量 |
| 所有人被鎖在更新畫面外 | ① 後台立即把 `minimum_version` 改回目前已發布的版本（設定即時生效，不需部署）② 確認商店上真的有該版本 |
| App 卡在啟動畫面不動 | 檢查 bootstrap 內版本檢查外面的 try/catch 是否被移除：例外冒到 `BrandStartupGate` 不會解除啟動畫面 |
| 查不到版本欄 | ① 確認該請求類型是不是不帶版本 header 的來源（見 §2）② 確認該 client 的 header 注入點有沒有漏：長連線的握手也要帶 |

## 2. 版本分佈怎麼讀

兩支 App 對後端請求帶 `X-App-Version` 與 `X-App-Type`；Gateway 的 access log 記成 `app_version`、`app_type` 兩欄（各截斷 32 字元，進 Loki），Grafana dashboard `hawkr-app-versions`（`backend/monitoring/grafana/provisioning/dashboards/json/app-versions.json`）讀這兩欄。

- **分母只取 `app_version` 不為空字串的請求**，也就是本來就會帶版本 header 的那個母體。不要用「全部請求扣掉某幾類」的方式湊分母：網頁伺服器端取數、服務間呼叫與 health probe 都沒有這個 header，放進分母只會把每個版本的佔比一起稀釋。
- 以此為分母，下列三類自動既不進分子也不進分母：長連線（SSE、WebSocket，只在握手記一筆）、WebView 導覽（`loadRequest` 帶不了自訂 header）、圖片與下載網址直取。代價是重度使用聊天或即時通知的使用者，其活躍度不反映在請求數上，會被低估。讀數是請求佔比，不是使用者佔比。
- 所有統計查詢一律加 semver 白名單 `` app_version =~ `^[0-9]+\.[0-9]+\.[0-9]+$` ``：`app_version` 是匿名可控輸入，Gateway 只截斷不驗證。被白名單擋掉的量由 `backend/monitoring/loki/rules/fake/app-version.yml` 告警；它非 0 也可能代表 App 送錯格式（把顯示字串當版本送出），那會讓一整個版本從佔比中消失。
- 查詢窗口上限 7 天：Loki 實際保留期是 168 小時（`monitoring/loki/local-config.yaml`）。拉更長的範圍不會報錯，只會得到被截斷的假象。
- 與其他版本來源對帳前先對齊格式：Sentry 的 `release` 是 `<packageName>@<version>+<build>`，要切在第一個 `@` 之後、`+` 之前才是 bare semver，不要寫死 package name；Firebase Analytics 的 `app_version` 格式相同，但母體是 App 啟動與事件，不是 HTTP 請求，兩邊的百分比不可直接相減，只看各自的趨勢。

## 3. 舊版 App 的放行判準

破壞性的契約變更（例如移除舊欄位）前，用滾動 7 天、環境 prod、每個 `app_type` 各算一次；三條同時成立才放行：

| # | 條件 | 用意 |
|---|---|---|
| 1 | 舊版請求佔比 < 1% | 主判準：舊版流量已成尾巴 |
| 2 | 舊版請求絕對數 < 100 | 總量暴增會讓百分比自然變小，佔比達標但絕對數仍高代表真的還有人在用 |
| 3 | 分母（該 `app_type` 帶合法 semver 的請求總數）>= 10,000 | 分母太小時第 1 條假性達標 |

第 3 條不成立時，本判準不適用，改用 App Store Connect 的版本分佈（母體是安裝數，補上長連線使用者被低估的缺口）。三個門檻是推算值，不是實測校準值；App 放量後要回頭看實際的日請求量級再調整。

放行前手動跑一次（貼進 Grafana Explore；`$app_type` 換成 `BUYER` 或 `SELLER`，`$old` 換成要淘汰的版本號）：

```logql
# 1. 舊版佔比（0 到 1 的小數，門檻 < 0.01）
sum(count_over_time({container="gateway", environment="prod"} |= `gateway request` | json | app_type=`$app_type` | app_version=`$old` [7d]))
/
sum(count_over_time({container="gateway", environment="prod"} |= `gateway request` | json | app_type=`$app_type` | app_version =~ `^[0-9]+\.[0-9]+\.[0-9]+$` [7d]))

# 2. 舊版絕對數（門檻 < 100）
sum(count_over_time({container="gateway", environment="prod"} |= `gateway request` | json | app_type=`$app_type` | app_version=`$old` [7d]))

# 3. 分母樣本數（門檻 >= 10,000；dashboard 的「分母樣本數」panel 就是這條）
sum(count_over_time({container="gateway", environment="prod"} |= `gateway request` | json | app_type=`$app_type` | app_version =~ `^[0-9]+\.[0-9]+\.[0-9]+$` [7d]))
```

「舊版佔比 > 1%」不做成告警：規則裡要寫死現行版本號，每次發版都要有人改，最後會變成一條對著舊版號評估的死規則。放行是一次性的人為決策，用手動查詢；告警只留與版本號無關、恆真的不變式（非法版本值為 0）。

## 4. minimum_version 的維護

`minimum_version` 必須是已發布的版本，每次 App 送審通過或放量後都回後台更新；流程見 `docs/開發規範/Git 分支與交付路徑規範.md` §2.1。漏做不會報錯，只會讓 gate 停在舊版本而形同不存在。
