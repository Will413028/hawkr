# 識別碼改名對照（步驟 1 附件）

快照 commit：e5d5a8d8e（程式碼與 332da5e51 相同）。由 `gen_rename_map.py` 依 denylist 展開規則並在追蹤中的檔上計數；本檔只有代號，明文對照在不進版控的 `.local/publish/rename-map.plain.tsv`，`rename.py`（步驟 3）讀後者。

本檔不能再用 `gen_rename_map.py` 重新生成：生成器在改名前的樹上計數，而改名已套用；A18 到 A29、第 7 節的補充與撞名清單的修正輪結論是手工維護的，生成器沒有這些規則，環境變數表也已依改名後的樹刪除 X6 刪掉的列。

讀法：`<舊名>` 是 denylist 第一列定義的舊名英文各種寫法；`<縮寫>` 是 D6 的兩字母縮寫；`<舊名-中文>` 是中文名。新字串 `hawkr` 的大小寫變體依命中形狀決定（見第 1 節）。
每條規則的計數指令：`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count <規則 id>`；命中路徑：`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --files <規則 id>`（路徑已代號化，`scan.py --resolve` 可還原）。計數不含計畫、附件與 `.local/`。

D8=A（2026-10-24，Will 已決）：縮寫的所有同族寫法（A10 到 A12 與第 6、7 節的其他家族）與第 8 節的資料值（X1 到 X4）一律改成 `hk`／`HK`／`Hk` 系列，連同產生碼、驗證正則、CHECK 約束、種子資料與 fixture；migration 改寫並在步驟 3 重算 `atlas.sum`。原 R-1 因此結案。

## 待 Will 確認

| 代號 | 問題 | 預設 |
|---|---|---|
| R-2 | 舊服務 Go module 只換前綴（`github.com/<舊名>/gateway` → `github.com/Will413028/hawkr/gateway`），不併進 `github.com/Will413028/hawkr/backend/…` 的目錄結構 | 只換前綴，後綴不變（最小改動；`backend/gateway` 預定在階段 B、C 刪除） |
| R-3 | 中文名 `<舊名-中文>`（含 App 顯示名稱、法律頁、測試資料裡的商號）不在 rename.py：中文在步驟 4（使用者面文字）與步驟 6（測試資料）逐項處理 | 同左 |

## 1. 舊字串對新字串

### 1.1 英文各種寫法（`<舊名>`）

- 指令：`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A07`；總檔數另見 `scan.py --category brand --english --report | wc -l`（6,485 個檔）。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。

| 命中形狀（代號化） | 形狀 | 檔數 | 處數 | 新字串 |
|---|---|---|---|---|
| `<舊名>` | 全小寫、無分隔 | 6346 | 54936 | `hawkr` |
| `<舊名>` | 每個字首大寫（駝峰或 Title）、無分隔 | 275 | 689 | `Hawkr` |
| `<舊名>` | 首字大寫、無分隔 | 59 | 175 | `Hawkr` |
| `<舊名>` | 全小寫、連字號 | 57 | 155 | `hawkr` |
| `<舊名-空格>` | 每個字首大寫（駝峰或 Title）、空格 | 32 | 43 | `Hawkr` |
| `<舊名>` | 全大寫、無分隔 | 16 | 20 | `HAWKR` |
| `<舊名-空格>` | 全大寫、空格 | 5 | 7 | `HAWKR` |

說明：代號化後各形狀都顯示成 `<舊名>`，所以另列「形狀」欄；`rename.py` 的明文表（TSV 的 FORM 列）逐形狀列出。沒有出現的寫法（例如底線分隔）不列。

### 1.2 規則總表（依序套用）

先套 skip，再套特例（A03 到 A06，比通用規則先），最後套通用規則與縮寫。同一處只被第一個命中的規則改寫。

| id | 規則 | 對象 | 新字串 | 範圍 | 計數 | 搜尋指令 | 備註 |
|---|---|---|---|---|---|---|---|
| A01 | 舊公司端點規則命中處（10 類） | 保護 | `（不改）` | 全樹 | 308 個檔、1269 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A01` | `check-legacy-endpoints.py` 的 10 類規則；步驟 2 先移除，rename.py 在步驟 3 才跑並跳過殘餘命中。 |
| A02 | 舊 GitHub 組織與舊 registry 名稱 | 保護 | `（不改）` | 全樹 | 51 個檔、152 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A02` | 外部擁有的名字，改名只會產生不存在的連結；issue 連結由步驟 6 處理。 |
| A03 | Java 套件與反向網域：com.<舊名> | 內容 | `io.github.will413028.hawkr` | Java、proto 的 java_package、Spring 設定、logging 與 mapper 設定 | 1768 個檔、10098 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A03` | D4；比通用規則先套，否則會變成 `com.hawkr`。 |
| A04 | iOS bundle ID 與 Apple 商家 ID 的反向網域：food.<舊名> | 內容 | `io.github.will413028.hawkr` | pbxproj、xcconfig、plist、entitlements、文件 | 29 個檔、85 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A04` | D4；`merchant.food.<舊名>` 同樣改成 `merchant.` 加 D4 前綴，但 Apple 開發者後台要另外重建。 |
| A05 | 舊服務的 Go module 前綴：github.com/<舊名>/ | 內容 | `github.com/Will413028/hawkr/` | go.mod、go.work 的 replace、import、go_package、buf 設定 | 1825 個檔、5006 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A05` | 計畫步驟 1：舊服務的 module 一併改成這個前綴；後綴（gateway、go-pkg、*-service）不變。 |
| A06 | 單體的 Go module：github.com/Will413028/<舊名> | 內容 | `github.com/Will413028/hawkr` | go.mod、import、buf.gen、Makefile、workflow | 617 個檔、2463 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A06` | 只換名字，帳號不變。 |
| A07 | 通用規則：英文各種寫法 | 內容 | `@CASE` | 全樹（除 skip 與排除清單） | 6486 個檔、56025 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A07` | 依命中字串的大小寫形狀取代：全小寫（含連字號）→ hawkr；首字大寫 → Hawkr；全大寫 → HAWKR。 |
| A08 | 縮寫（D6）：小寫連字號 | 內容 | `hk-` | CSS 自訂屬性、Tailwind class、DOM id、檔名 | 305 個檔、2462 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A08` | D6；左邊界是非英數字，所以 `pdf-` 不會被改。 |
| A09 | 縮寫（D6）：小寫底線 | 內容 | `hk_` | Dart 檔名與 import、識別碼、Go 與 TS 的常數 | 547 個檔、1199 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A09` | D6；同上。 |
| A10 | 縮寫：大寫連字號與底線（資料值撞名，見第 8 節） | 內容 | `HK-／HK_` | HTTP header、ID 格式、測試常數 | 62 個檔、176 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A10` | D6 沒明講大寫；D8=A（2026-10-24）已決：同族寫法一併改；出現在資料值的部分見第 8 節。 |
| A11 | 縮寫：UI 元件庫前綴 `<縮寫>ui-`、頁面前綴 `<縮寫>h-` | 內容 | `hkui-／hkh-` | website、admin 的 CSS 與 TSX | 325 個檔、8310 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A11` | D6 沒列出的同族前綴；D8=A 已決：一併改。 |
| A12 | 縮寫：Pascal 類別前綴 `<縮寫Pascal>`＋大寫字母 | 內容 | `Hk` | Dart、TS 的類別與型別 | 431 個檔、5686 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A12` | D6 沒列出；D8=A 已決：一併改；鎖定檔與 JSON 內的亂數 id 要排除（見第 7 節）。 |
| A13 | 縮寫：日誌類別 `<縮寫>Log` | 內容 | `HKLog` | Dart 的日誌工具與文件 | 113 個檔、700 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A13` | 同上。 |
| A14 | 縮寫：camelCase `<縮寫>`＋大寫字母 | 內容 | `hk` | TS 識別碼 | 12 個檔、79 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A14` | denylist 標為 uncertain；go.sum 與 lottie 的亂數字串會誤中，要排除。 |
| A15 | 路徑：Java 套件目錄 com/<舊名>/ | 路徑 | `io/github/will413028/hawkr/` | `git mv`，目錄深度增加 | 1633 個檔 | `scan.py --category brand-path --report`（路徑）；明細見第 3 節 | 依 A03；先於 A16。 |
| A16 | 路徑：目錄與檔名裡的舊名 | 路徑 | `@CASE` | `git mv` | 2028 個檔 | `scan.py --category brand-path --report`（路徑）；明細見第 3 節 | 同 A07，依命中形狀。 |
| A17 | 路徑：縮寫檔名 <縮寫>_*.dart、<縮寫>-* | 路徑 | `hk_／hk-` | `git mv` | 66 個檔 | `scan.py --category brand-path --report`（路徑）；明細見第 3 節 | 同 A08、A09；內容裡的 import 同步改。 |
| A18 | 縮寫補漏：`<縮寫>ui` 單獨出現（右邊不是連字號） | 內容 | `hkui` | localStorage／sessionStorage 鍵、`[data-<縮寫>ui]`、註解與測試名 | 31 個檔、37 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A18` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A19 | 縮寫補漏：`<縮寫>:` 冒號命名空間 | 內容 | `hk` | 瀏覽器事件名與 storage 鍵（`<縮寫>:theme` 等） | 25 個檔、37 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A19` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A20 | 縮寫補漏：簽章網域前綴 `<縮寫>origin1:`、`<縮寫>wst1:` | 內容 | `hk` | 閘道、單體、website 的 HMAC 網域前綴；簽與驗兩端同一次改 | 7 個檔、9 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A20` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A21 | 縮寫補漏：`<縮寫>/<縮寫>ui` 對照寫法與 User-Agent 產品名 `<縮寫>/1.0` | 內容 | `hk` | 註解、seller 協議測試 | 3 個檔、5 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A21` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A22 | 縮寫補漏：正則 alternation `(?:<縮寫>\|…)` | 內容 | `hk` | website/scripts 的檢查腳本 | 1 個檔、1 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A22` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A23 | 縮寫補漏：正則跳脫 `\b` 之後的 Pascal 前綴 | 內容 | `Hk` | Dart 測試的 RegExp | 1 個檔、1 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A23` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A24 | 縮寫補漏：測試單號前綴 `<縮寫大寫>TEST-` | 內容 | `HKTEST` | 第三方 API 範例的 order_id | 1 個檔、1 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A24` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A25 | 縮寫補漏：header 前綴 `X-<縮寫Pascal>-` | 內容 | `Hk` | 設計書的禁止轉發清單 | 1 個檔、1 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A25` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A26 | 縮寫補漏：`<數字>_<縮寫>` 活動代碼 | 內容 | `hk` | analytics 的 utm_campaign 白名單 | 1 個檔、1 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A26` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A27 | 縮寫補漏：`<縮寫>x-relay` 路由 | 內容 | `hkx-relay` | admin 的 Sentry tunnel 路由 | 1 個檔、1 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A27` | 步驟 3 修正輪 1：A08 到 A14 的邊界規則（左邊是非英數字、右邊是連字號或底線或大寫字母）漏掉的同族寫法；排在 A14 之後。 |
| A28 | 子品牌（D9）：`<舊名-子品牌>` 只改舊名那一段 | 內容 | `@CASE`（Hawkr Agent） | website 的元件、路由、測試、env 變數名、`.agents` 技能 | 121 個檔、1076 處 | `git grep -i -c -E '<舊名-子品牌>'`（規則樣板是 denylist 該列的舊名段加上向前斷言，生成器沒有這條規則） | 步驟 5f：分隔與大小寫形狀照命中，只換舊名那一段；排在 A27 之後。 |
| A29 | 路徑：目錄與檔名裡的子品牌 | 路徑 | `@CASE` | `git mv` | 28 個檔 | `git ls-files \| rg -i '<舊名-子品牌>'` | 步驟 5f：同 A28。 |

### 1.3 通用規則涵蓋的類別（明細）

類別的搜尋樣板是依該類別的語意寫的；這些處數有重疊（同一個字串可能同時屬於兩類）。

| 類別 | 說明 | 新樣貌 | 套用規則 | 計數 | 搜尋指令 |
|---|---|---|---|---|---|
| K01 | Go import 路徑（單體） | `github.com/Will413028/hawkr/backend/…` | A06 | 617 個檔、2463 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K01` |
| K02 | Go import 路徑（舊服務與 pkg） | `github.com/Will413028/hawkr/<後綴>` | A05 | 1825 個檔、5006 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K02` |
| K03 | Java 套件與 import | `io.github.will413028.hawkr.…` | A03 | 1768 個檔、10098 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K03` |
| K04 | npm 套件範圍與 workspace filter | `@hawkr/api、@hawkr/ui-text、@hawkr/twaddr` | A07 | 377 個檔、891 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K04` |
| K05 | Dart 套件 import | `package:hawkr_<後綴>` | A07 | 893 個檔、3444 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K05` |
| K06 | proto package 宣告 | `hawkr.<領域>.v1alpha1（v1alpha1 不改）` | A07 | 80 個檔、81 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K06` |
| K07 | proto import 路徑 | `import "hawkr/…"` | A07 | 15 個檔、36 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K07` |
| K08 | 容器名與 compose 網路 | `hawkr-<名稱>` | A07 | 7 個檔、57 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K08` |
| K09 | 資料庫名稱（連線字串與環境變數） | `hawkr、hawkr_<服務>` | A07 | 27 個檔、57 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K09` |
| K10 | 資料庫名稱（URL 路徑） | `/hawkr、/hawkr_<服務>` | A07 | 62 個檔、112 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K10` |
| K11 | RabbitMQ exchange、queue 與 routing key | `hawkr.<領域>、hawkr.<領域>.dlx` | A07 | 73 個檔、351 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K11` |
| K12 | 環境變數前綴（大寫） | `HAWKR_…` | A07 | 16 個檔、20 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K12` |
| K13 | URL scheme（深層連結） | `hawkr://、hawkrbuyer://、hawkrseller://` | A07 | 49 個檔、195 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K13` |
| K14 | 儲存鍵與 JSON 字串鍵（sessionStorage、版本字串） | `"hawkr:…"` | A07 | 280 個檔、1396 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K14` |
| K15 | HTTP header 名稱 | `X-Hawkr-…` | A07 | 1 個檔、5 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K15` |
| K16 | 指標名稱（Prometheus） | `hawkr_<…>_total` | A07 | 16 個檔、82 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K16` |
| K17 | Spring 設定前綴與 Java 屬性鍵 | `hawkr.<服務>.*` | A07 | 115 個檔、3793 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K17` |
| K18 | buf 模組名稱 | `buf.build/hawkr/api` | A07 | 2 個檔、2 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K18` |
| K19 | 開發用 token 的 issuer 與其他字串常數 | `"hawkr-dev"` | A07 | 53 個檔、93 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K19` |
| K20 | 網站與其他套件名稱（package.json name、pubspec name） | `hawkr-web、@hawkr/api、hawkr_buyer` | A07 | 25 個檔、141 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K20` |
| K21 | CSS 自訂屬性與 Tailwind class（縮寫） | `--hk-primary、text-hk-muted` | A08 | 170 個檔、1619 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K21` |
| K22 | Dart 縮寫檔名與 import | `hk_tokens.dart` | A09 | 475 個檔、1054 處 | `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K22` |

## 2. Go module 路徑

- 指令：`git ls-files | grep -E '(^|/)go\.(mod|work)$'` 共 14 個檔（13 個 go.mod 加 `backend/go.work`）；`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K01`、`--count K02`。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。改名後每個 module 要 `GOWORK=off go mod tidy` 確認沒有 diff（`AGENTS.local.md`），`go.work` 的 `use` 路徑是目錄、不改。

| go.mod | 目前的 module | 新 module | replace | 備註 |
|---|---|---|---|---|
| `backend/gateway/go.mod` | `github.com/<舊名>/gateway` | `github.com/Will413028/hawkr/gateway` | replace `github.com/<舊名>/go-pkg` → `../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/go.mod` | `github.com/Will413028/<舊名>/backend` | `github.com/Will413028/hawkr/backend` | - | 改前綴；module 路徑與 replace 左側同步 |
| `backend/monitoring/discord-webhook-gateway/go.mod` | `discord-webhook-gateway` | `discord-webhook-gateway` | - | 沒有舊名，不改 |
| `backend/pkg/go.mod` | `github.com/<舊名>/go-pkg` | `github.com/Will413028/hawkr/go-pkg` | - | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/ai-service/go.mod` | `github.com/<舊名>/ai-service` | `github.com/Will413028/hawkr/ai-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/crm-service/go.mod` | `github.com/<舊名>/crm-service` | `github.com/Will413028/hawkr/crm-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/im-service/go.mod` | `github.com/<舊名>/im-service` | `github.com/Will413028/hawkr/im-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/invoice-service/go.mod` | `github.com/<舊名>/invoice-service` | `github.com/Will413028/hawkr/invoice-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/notification-service/go.mod` | `github.com/<舊名>/notification-service` | `github.com/Will413028/hawkr/notification-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/pricebook-service/go.mod` | `github.com/<舊名>/pricebook-service` | `github.com/Will413028/hawkr/pricebook-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/review-service/go.mod` | `github.com/<舊名>/review-service` | `github.com/Will413028/hawkr/review-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/seller-service/go.mod` | `github.com/<舊名>/seller-service` | `github.com/Will413028/hawkr/seller-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |
| `backend/services/social-service/go.mod` | `github.com/<舊名>/social-service` | `github.com/Will413028/hawkr/social-service` | replace `github.com/<舊名>/go-pkg` → `../../pkg`（目錄，不改） | 改前綴；module 路徑與 replace 左側同步 |

- `backend/go.work` 的 `use` 清單 12 個目錄（`.`、`./gateway`、`./monitoring/discord-webhook-gateway`、`./pkg`、`./services/{ai,crm,im,invoice,notification,pricebook,review,seller,social}-service`）都是相對目錄，不含舊名，不改。
- proto 的 `option go_package`：`backend/api/proto/**` 的 `github.com/<舊名>/go-pkg/proto/…` 依 A05 改；`backend/buf.gen.connect.yaml` 的 `go_package_prefix` 依 A06 改。這些改完要重新生成 `backend/pkg/proto` 與 `backend/internal/legacy/gen`，不能只改生成物的 import。

## 3. 目錄名與檔名

- 路徑含舊名的追蹤檔 2028 個；路徑含縮寫的追蹤檔 66 個（`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --category brand-path` 的同義指令：`scan.py --category brand-path --report | wc -l` 另含 uncertain 與 colleague 列）。
- 指令：`git ls-files | …` 依 denylist 展開後的規則；也可用 `scan.py --category brand-path --report`。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。順序：先 `git mv` 所有目錄與檔名（由深到淺），再改內容；`rename.py` 再跑一次不得有 diff。

頂層需要 `git mv` 的目錄（巢狀的跟著走）：

| 目錄（代號化） | 新路徑 | 底下的追蹤檔數 | 規則 |
|---|---|---|---|
| `api/<舊名>` | `api/hawkr` | 23 | A16／A17 |
| `backend/cmd/<舊名>` | `backend/cmd/hawkr` | 36 | A16／A17 |
| `backend/internal/httpapi/gen/<舊名>` | `backend/internal/httpapi/gen/hawkr` | 37 | A16／A17 |
| `backend/services/java/aftersales-service/src/main/java/com/<舊名>` | `backend/services/java/aftersales-service/src/main/java/com/hawkr` | 79 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/aftersales-service/src/test/java/com/<舊名>` | `backend/services/java/aftersales-service/src/test/java/com/hawkr` | 41 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/cart-service/src/main/java/com/<舊名>` | `backend/services/java/cart-service/src/main/java/com/hawkr` | 15 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/cart-service/src/test/java/com/<舊名>` | `backend/services/java/cart-service/src/test/java/com/hawkr` | 5 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/coupon-service/src/main/java/com/<舊名>` | `backend/services/java/coupon-service/src/main/java/com/hawkr` | 38 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/coupon-service/src/test/java/com/<舊名>` | `backend/services/java/coupon-service/src/test/java/com/hawkr` | 18 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/media-service/src/main/java/com/<舊名>` | `backend/services/java/media-service/src/main/java/com/hawkr` | 44 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/media-service/src/test/java/com/<舊名>` | `backend/services/java/media-service/src/test/java/com/hawkr` | 18 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/order-service/src/main/java/com/<舊名>` | `backend/services/java/order-service/src/main/java/com/hawkr` | 118 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/order-service/src/test/java/com/<舊名>` | `backend/services/java/order-service/src/test/java/com/hawkr` | 81 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/payment-service/src/main/java/com/<舊名>` | `backend/services/java/payment-service/src/main/java/com/hawkr` | 192 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/payment-service/src/test/java/com/<舊名>` | `backend/services/java/payment-service/src/test/java/com/hawkr` | 77 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/payuni-webhook-service/src/main/java/com/<舊名>` | `backend/services/java/payuni-webhook-service/src/main/java/com/hawkr` | 27 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/payuni-webhook-service/src/test/java/com/<舊名>` | `backend/services/java/payuni-webhook-service/src/test/java/com/hawkr` | 10 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/product-import-service/src/main/java/com/<舊名>` | `backend/services/java/product-import-service/src/main/java/com/hawkr` | 59 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/product-import-service/src/test/java/com/<舊名>` | `backend/services/java/product-import-service/src/test/java/com/hawkr` | 16 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/product-service/src/main/java/com/<舊名>` | `backend/services/java/product-service/src/main/java/com/hawkr` | 128 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/product-service/src/test/java/com/<舊名>` | `backend/services/java/product-service/src/test/java/com/hawkr` | 56 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/shared/common-core/src/main/java/com/<舊名>` | `backend/services/java/shared/common-core/src/main/java/com/hawkr` | 308 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/shared/common-core/src/test/java/com/<舊名>` | `backend/services/java/shared/common-core/src/test/java/com/hawkr` | 19 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/shared/observability-common/src/main/java/com/<舊名>` | `backend/services/java/shared/observability-common/src/main/java/com/hawkr` | 22 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/shared/payuni-java-client/src/main/java/com/<舊名>` | `backend/services/java/shared/payuni-java-client/src/main/java/com/hawkr` | 54 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/shared/payuni-java-client/src/test/java/com/<舊名>` | `backend/services/java/shared/payuni-java-client/src/test/java/com/hawkr` | 4 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/shipping-service/src/main/java/com/<舊名>` | `backend/services/java/shipping-service/src/main/java/com/hawkr` | 147 | Java 套件目錄，深度增加（A15） |
| `backend/services/java/shipping-service/src/test/java/com/<舊名>` | `backend/services/java/shipping-service/src/test/java/com/hawkr` | 57 | Java 套件目錄，深度增加（A15） |
| `packages/<舊名>-api-ts` | `packages/hawkr-api-ts` | 45 | A16／A17 |
| `packages/<舊名>-api-ts/src/gen/<舊名>` | `packages/hawkr-api-ts/src/gen/hawkr` | 23 | A16／A17 |
| `packages/<舊名>-twaddr-ts` | `packages/hawkr-twaddr-ts` | 8 | A16／A17 |
| `packages/<舊名>-ui-text-ts` | `packages/hawkr-ui-text-ts` | 17 | A16／A17 |
| `packages/<舊名>_api` | `packages/hawkr_api` | 92 | A16／A17 |
| `packages/<舊名>_api/lib/gen/<舊名>` | `packages/hawkr_api/lib/gen/hawkr` | 60 | A16／A17 |
| `packages/<舊名>_app_update` | `packages/hawkr_app_update` | 6 | A16／A17 |
| `packages/<舊名>_auth` | `packages/hawkr_auth` | 15 | A16／A17 |
| `packages/<舊名>_twaddr` | `packages/hawkr_twaddr` | 8 | A16／A17 |
| `packages/<舊名>_ui_text` | `packages/hawkr_ui_text` | 14 | A16／A17 |
| `seller-app/design-system/<舊名>-seller` | `seller-app/design-system/hawkr-seller` | 1 | A16／A17 |
| `website/assets/fonts/<舊名>-rounded-v2` | `website/assets/fonts/hawkr-rounded-v2` | 82 | A16／A17 |

不在上述目錄底下、本身檔名含舊名或縮寫的檔：舊名 11 個、縮寫 66 個。依副檔名：

| 類型 | 副檔名 | 檔數 |
|---|---|---|
| 縮寫 | dart | 66 |
| 舊名 | png | 8 |
| 舊名 | svg | 2 |
| 舊名 | csv | 1 |

字型檔（`.woff2`）的檔名與檔內的家族名都含舊名：檔名由 `git mv` 改，檔內 name table 與 manifest 的雜湊要重新生成（第 8 節）。

## 4. npm 套件

- 指令：`git ls-files | grep 'package.json$'`；`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K04`、`--count K20`。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。改完以 `pnpm install --lockfile-only` 重新產生 `pnpm-lock.yaml`（鎖定檔不由 rename.py 改）。

| package.json | 現名 | 新名 | 備註 |
|---|---|---|---|
| `admin/package.json` | `<舊名>-admin` | `hawkr-admin` | 套件名；消費端的 import、`transpilePackages`、workflow 的 `pnpm --filter` 同步 |
| `backend/workers/image-gateway/package.json` | `image-gateway` | `image-gateway` | 不變 |
| `package.json` | `<舊名>` | `hawkr` | 套件名；消費端的 import、`transpilePackages`、workflow 的 `pnpm --filter` 同步 |
| `packages/<舊名>-api-ts/package.json` | `@<舊名>/api` | `@hawkr/api` | 套件名；消費端的 import、`transpilePackages`、workflow 的 `pnpm --filter` 同步 |
| `packages/<舊名>-twaddr-ts/package.json` | `@<舊名>/twaddr` | `@hawkr/twaddr` | 套件名；消費端的 import、`transpilePackages`、workflow 的 `pnpm --filter` 同步 |
| `packages/<舊名>-ui-text-ts/package.json` | `@<舊名>/ui-text` | `@hawkr/ui-text` | 套件名；消費端的 import、`transpilePackages`、workflow 的 `pnpm --filter` 同步 |
| `website/package.json` | `<舊名>-web` | `hawkr-web` | 套件名；消費端的 import、`transpilePackages`、workflow 的 `pnpm --filter` 同步 |

- 連字號寫法 `<舊名>`（admin 的 `<舊名>-admin`）也由 A07 處理，新名 `hawkr-admin`。
- `pnpm-workspace.yaml` 的 catalog 與 workflow 裡的 `--filter @<舊名>/…` 由 A07 改。

## 5. Dart 套件

- 指令：`git ls-files | grep 'pubspec.yaml$'`；`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count K05`、`--count K22`。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。目錄由 `packages/<舊名>_*` 改成 `packages/hawkr_*`（第 3 節）；`pubspec.lock` 由 `fvm flutter pub get` 重新產生；`dart run build_runner build --delete-conflicting-outputs` 重新生成 `*.g.dart`／`*.freezed.dart`（CI 的生成檢查）。

| pubspec.yaml | 現名 | 新名 | 備註 |
|---|---|---|---|
| `buyer-app/pubspec.yaml` | `<舊名>_buyer` | `hawkr_buyer` | 套件名與 `package:` import、path 依賴同步 |
| `packages/<舊名>_api/pubspec.yaml` | `<舊名>_api` | `hawkr_api` | 套件名與 `package:` import、path 依賴同步 |
| `packages/<舊名>_app_update/pubspec.yaml` | `<舊名>_app_update` | `hawkr_app_update` | 套件名與 `package:` import、path 依賴同步 |
| `packages/<舊名>_auth/pubspec.yaml` | `<舊名>_auth` | `hawkr_auth` | 套件名與 `package:` import、path 依賴同步 |
| `packages/<舊名>_twaddr/pubspec.yaml` | `<舊名>_twaddr` | `hawkr_twaddr` | 套件名與 `package:` import、path 依賴同步 |
| `packages/<舊名>_ui_text/pubspec.yaml` | `<舊名>_ui_text` | `hawkr_ui_text` | 套件名與 `package:` import、path 依賴同步 |
| `seller-app/pubspec.yaml` | `<舊名>_seller` | `hawkr_seller` | 套件名與 `package:` import、path 依賴同步 |

## 6. 資料庫、訊息佇列、環境變數、容器、bundle ID、proto、Java

- 計數指令：K09 到 K19 見第 1.3 節；本節的清單由 `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py` 在追蹤中的檔上展開，不含值（只有名稱的形狀）。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。

### 6.1 資料庫名稱

| 名稱（代號化） | 新名 | 出現的檔數 |
|---|---|---|
| `<舊名>` | `Hawkr` | 246 |
| `<舊名>` | `hawkr` | 172 |
| `<舊名>_seller` | `hawkr_seller` | 54 |
| `<舊名>_buyer` | `hawkr_buyer` | 40 |
| `<舊名>` | `Hawkr` | 31 |
| `<舊名>_ai` | `hawkr_ai` | 25 |
| `<舊名>_pricebook` | `hawkr_pricebook` | 16 |
| `<舊名>_payment` | `hawkr_payment` | 16 |
| `<舊名>_shipping` | `hawkr_shipping` | 15 |
| `<舊名>_api` | `hawkr_api` | 13 |
| `<舊名>_social` | `hawkr_social` | 12 |
| `<舊名>_invoice` | `hawkr_invoice` | 12 |
| `<舊名>_product` | `hawkr_product` | 12 |
| `<舊名>_twaddr` | `hawkr_twaddr` | 11 |
| `<舊名>_crm` | `hawkr_crm` | 10 |
| `<舊名>_notification` | `hawkr_notification` | 9 |
| `<舊名>_im` | `hawkr_im` | 8 |
| `<舊名>_review` | `hawkr_review` | 8 |
| `<舊名>_order` | `hawkr_order` | 8 |
| `<舊名>_webhook` | `hawkr_webhook` | 7 |
| `<舊名>_auth` | `hawkr_auth` | 6 |
| `<舊名>_aftersales` | `hawkr_aftersales` | 6 |
| `<舊名>_coupon` | `hawkr_coupon` | 6 |
| `<舊名>` | `HAWKR` | 5 |
| `<舊名>_media` | `hawkr_media` | 4 |
| `<舊名>_cart` | `hawkr_cart` | 3 |
| `<舊名>_content` | `hawkr_content` | 2 |
| `<舊名>_procurement` | `hawkr_procurement` | 2 |
| `<舊名>_marketing` | `hawkr_marketing` | 1 |
| `<舊名>_account` | `hawkr_account` | 1 |
| `<舊名>_xxx` | `hawkr_xxx` | 1 |
| `<舊名>_bootstrap` | `hawkr_bootstrap` | 1 |
| `<舊名>_ReadAll` | `Hawkr_ReadAll` | 1 |
| `<舊名>_newservice` | `hawkr_newservice` | 1 |
| `<舊名>_settlement` | `hawkr_settlement` | 1 |

單體沒有自己的服務 DB 名；舊服務的 `<舊名>_<服務>` 與單體的 `<舊名>` 都是本機 docker 的資料庫（`backend/docker-compose-*.yml`、`Makefile`、`.env.template`），前提表已確認不在線，直接改名、不留別名。Atlas 的 `atlas.hcl`、`atlas.sum` 要重算（`atlas migrate hash`）。

### 6.2 點號名稱（proto package、MQ exchange 與 routing key、Spring 屬性前綴）

| 名稱（代號化，前 60 個） | 新名 | 出現的檔數 |
|---|---|---|
| `<舊名>.shipping` | `hawkr.shipping` | 52 |
| `<舊名>.order` | `hawkr.order` | 50 |
| `<舊名>.admin.seller.v1alpha1` | `hawkr.admin.seller.v1alpha1` | 33 |
| `<舊名>.seller` | `hawkr.seller` | 31 |
| `<舊名>.common.v1alpha1.Image` | `hawkr.common.v1alpha1.Image` | 29 |
| `<舊名>.food` | `hawkr.food` | 28 |
| `<舊名>.buyer` | `hawkr.buyer` | 25 |
| `<舊名>.content` | `hawkr.content` | 25 |
| `<舊名>.invoice` | `hawkr.invoice` | 24 |
| `<舊名>.media` | `hawkr.media` | 24 |
| `<舊名>.product` | `hawkr.product` | 23 |
| `<舊名>.common.v1alpha1.Deadline` | `hawkr.common.v1alpha1.Deadline` | 21 |
| `<舊名>.account` | `hawkr.account` | 19 |
| `<舊名>.common.v1alpha1.PolicyCategory` | `hawkr.common.v1alpha1.PolicyCategory` | 19 |
| `<舊名>.common.v1alpha1` | `hawkr.common.v1alpha1` | 19 |
| `<舊名>.common.DelegationContext` | `hawkr.common.DelegationContext` | 18 |
| `<舊名>.seller.v1alpha1` | `hawkr.seller.v1alpha1` | 17 |
| `<舊名>.notification` | `hawkr.notification` | 17 |
| `<舊名>.admin` | `hawkr.admin` | 17 |
| `<舊名>.im` | `hawkr.im` | 15 |
| `<舊名>.review` | `hawkr.review` | 15 |
| `<舊名>.product.import` | `hawkr.product.import` | 15 |
| `<舊名>.merchant` | `hawkr.merchant` | 15 |
| `<舊名>.admin.identity.v1alpha1` | `hawkr.admin.identity.v1alpha1` | 14 |
| `<舊名>.common.v1alpha1.StoreCard` | `hawkr.common.v1alpha1.StoreCard` | 14 |
| `<舊名>.payment` | `hawkr.payment` | 14 |
| `<舊名>.delegation` | `hawkr.delegation` | 14 |
| `<舊名>.marketing` | `hawkr.marketing` | 14 |
| `<舊名>.pricebook` | `hawkr.pricebook` | 13 |
| `<舊名>.seller_workspace` | `hawkr.seller_workspace` | 13 |
| `<舊名>.common.v1alpha1.PostalAddress` | `hawkr.common.v1alpha1.PostalAddress` | 12 |
| `<舊名>.common.v1alpha1.ErrorReason` | `hawkr.common.v1alpha1.ErrorReason` | 12 |
| `<舊名>.identity.v1alpha1` | `hawkr.identity.v1alpha1` | 11 |
| `<舊名>.seller.v1alpha1.ViolationSeverity` | `hawkr.seller.v1alpha1.ViolationSeverity` | 11 |
| `<舊名>.seller.v1alpha1.MeasureKind` | `hawkr.seller.v1alpha1.MeasureKind` | 11 |
| `<舊名>.seller.v1alpha1.KYCStatus` | `hawkr.seller.v1alpha1.KYCStatus` | 11 |
| `<舊名>.order.dlx` | `hawkr.order.dlx` | 11 |
| `<舊名>.seller.v1alpha1.RevokeReason` | `hawkr.seller.v1alpha1.RevokeReason` | 10 |
| `<舊名>.seller.v1alpha1.TerminationGround` | `hawkr.seller.v1alpha1.TerminationGround` | 10 |
| `<舊名>.seller.v1alpha1.RiskGround` | `hawkr.seller.v1alpha1.RiskGround` | 10 |
| `<舊名>.seller.v1alpha1.MeasureState` | `hawkr.seller.v1alpha1.MeasureState` | 10 |
| `<舊名>.seller.v1alpha1.MeasureBasis` | `hawkr.seller.v1alpha1.MeasureBasis` | 10 |
| `<舊名>.seller.v1alpha1.MeasureRequestState` | `hawkr.seller.v1alpha1.MeasureRequestState` | 10 |
| `<舊名>.seller.v1alpha1.MeasureRequestKind` | `hawkr.seller.v1alpha1.MeasureRequestKind` | 10 |
| `<舊名>.seller.v1alpha1.ViolationStatus` | `hawkr.seller.v1alpha1.ViolationStatus` | 10 |
| `<舊名>.account.AdminAccountStats` | `hawkr.account.AdminAccountStats` | 10 |
| `<舊名>.governance` | `hawkr.governance` | 10 |
| `<舊名>.admin.buyer.v1alpha1` | `hawkr.admin.buyer.v1alpha1` | 9 |
| `<舊名>.admin.content.v1alpha1` | `hawkr.admin.content.v1alpha1` | 9 |
| `<舊名>.buyer.v1alpha1` | `hawkr.buyer.v1alpha1` | 8 |
| `<舊名>.content.v1alpha1` | `hawkr.content.v1alpha1` | 8 |
| `<舊名>.content.v1alpha1.AppType` | `hawkr.content.v1alpha1.AppType` | 8 |
| `<舊名>.seller.v1alpha1.ReportOutcome` | `hawkr.seller.v1alpha1.ReportOutcome` | 8 |
| `<舊名>.seller.v1alpha1.KYCFlowStatus` | `hawkr.seller.v1alpha1.KYCFlowStatus` | 8 |
| `<舊名>.aftersales` | `hawkr.aftersales` | 8 |
| `<舊名>.payment.dlx` | `hawkr.payment.dlx` | 8 |
| `<舊名>.identity.v1alpha1.Role` | `hawkr.identity.v1alpha1.Role` | 7 |
| `<舊名>.identity.v1alpha1.DeleteAccountResponse` | `hawkr.identity.v1alpha1.DeleteAccountResponse` | 7 |
| `<舊名>.seller.v1alpha1.MeasureRequestCloseCause` | `hawkr.seller.v1alpha1.MeasureRequestCloseCause` | 7 |
| `<舊名>.seller.v1alpha1.StoreStatus` | `hawkr.seller.v1alpha1.StoreStatus` | 7 |

共 3727 個不同的點號名稱。其中以 `.v1alpha1` 結尾的是對外 API 的 proto package（`v1alpha1` 不改，計畫「不做」）；沒有版本後綴的（`<舊名>.order`、`<舊名>.shipping` 等）是舊 gRPC 的 package 與 RabbitMQ exchange 名，兩者字串相同、語意不同但規則相同（都改前綴）。

### 6.3 容器、網路、環境變數、指標、URL scheme、儲存鍵

| 種類 | 名稱（代號化） | 新名 | 出現的檔數 |
|---|---|---|---|
| 容器名（compose `container_name`） | `<舊名>-otel-collector` | `hawkr-otel-collector` | 2 |
| 容器名（compose `container_name`） | `<舊名>-postgres` | `hawkr-postgres` | 1 |
| 容器名（compose `container_name`） | `<舊名>-redis` | `hawkr-redis` | 1 |
| 容器名（compose `container_name`） | `<舊名>-rabbitmq` | `hawkr-rabbitmq` | 1 |
| 容器名（compose `container_name`） | `<舊名>-aftersales` | `hawkr-aftersales` | 1 |
| 容器名（compose `container_name`） | `<舊名>-prometheus` | `hawkr-prometheus` | 1 |
| 容器名（compose `container_name`） | `<舊名>-loki` | `hawkr-loki` | 1 |
| 容器名（compose `container_name`） | `<舊名>-tempo` | `hawkr-tempo` | 1 |
| 容器名（compose `container_name`） | `<舊名>-alertmanager` | `hawkr-alertmanager` | 1 |
| 容器名（compose `container_name`） | `<舊名>-discord-webhook-gateway` | `hawkr-discord-webhook-gateway` | 1 |
| 容器名（compose `container_name`） | `<舊名>-promtail` | `hawkr-promtail` | 1 |
| 容器名（compose `container_name`） | `<舊名>-grafana` | `hawkr-grafana` | 1 |
| 容器名（compose `container_name`） | `<舊名>-config-sync` | `hawkr-config-sync` | 1 |
| 容器名（compose `container_name`） | `<舊名>-config-reload-watcher` | `hawkr-config-reload-watcher` | 1 |
| 容器名（compose `container_name`） | `<舊名>-node-exporter` | `hawkr-node-exporter` | 1 |
| 容器名（compose `container_name`） | `<舊名>-prod-postgres-exporter` | `hawkr-prod-postgres-exporter` | 1 |
| 容器名（compose `container_name`） | `<舊名>-prod-sql-exporter` | `hawkr-prod-sql-exporter` | 1 |
| 環境變數（大寫前綴） | `<舊名>` | `HAWKR` | 7 |
| 環境變數（大寫前綴） | `<舊名>_APPS_ROOT` | `HAWKR_APPS_ROOT` | 4 |
| 環境變數（大寫前綴） | `<舊名>_MEDIA_HOSTS` | `HAWKR_MEDIA_HOSTS` | 2 |
| 環境變數（大寫前綴） | `<舊名>_SYNTHETIC_SMOKE__` | `HAWKR_SYNTHETIC_SMOKE__` | 1 |
| URL scheme | `<舊名>` | `hawkr` | 26 |
| URL scheme | `<舊名>buyer` | `hawkrbuyer` | 20 |
| URL scheme | `<舊名>seller` | `hawkrseller` | 10 |
| 指標名稱 | `<舊名>_payment_failed_total` | `hawkr_payment_failed_total` | 6 |
| 指標名稱 | `<舊名>_payment_total` | `hawkr_payment_total` | 6 |
| 指標名稱 | `<舊名>_holiday_coverage_days` | `hawkr_holiday_coverage_days` | 5 |
| 指標名稱 | `<舊名>_orders_created_total` | `hawkr_orders_created_total` | 4 |
| 指標名稱 | `<舊名>_atm_refund_stuck_total` | `hawkr_atm_refund_stuck_total` | 4 |
| 指標名稱 | `<舊名>_pricebook_market_unrecoverable_gap_days` | `hawkr_pricebook_market_unrecoverable_gap_days` | 4 |
| 指標名稱 | `<舊名>_cart_created_total` | `hawkr_cart_created_total` | 3 |
| 指標名稱 | `<舊名>_pricebook_ingest_stuck_running_total` | `hawkr_pricebook_ingest_stuck_running_total` | 3 |
| 指標名稱 | `<舊名>_grant_payout_held_total` | `hawkr_grant_payout_held_total` | 2 |
| 指標名稱 | `<舊名>_checkout_completed_total` | `hawkr_checkout_completed_total` | 2 |
| 指標名稱 | `<舊名>_inventory_lock_total` | `hawkr_inventory_lock_total` | 2 |
| 指標名稱 | `<舊名>_inventory_lock_failed_total` | `hawkr_inventory_lock_failed_total` | 2 |
| 指標名稱 | `<舊名>_payment_reconcile_outcome_total` | `hawkr_payment_reconcile_outcome_total` | 2 |
| 指標名稱 | `<舊名>_pricebook_market_accepted_gap_days` | `hawkr_pricebook_market_accepted_gap_days` | 2 |
| 指標名稱 | `<舊名>_pricebook_market_stale_open_days` | `hawkr_pricebook_market_stale_open_days` | 1 |
| 指標名稱 | `<舊名>_order_amount_total` | `hawkr_order_amount_total` | 1 |
| 指標名稱 | `<舊名>_payment_success_total` | `hawkr_payment_success_total` | 1 |
| 指標名稱 | `<舊名>_shipping_provider_status_unknown_total` | `hawkr_shipping_provider_status_unknown_total` | 1 |
| 指標名稱 | `<舊名>_shipping_provider_not_status_event_total` | `hawkr_shipping_provider_not_status_event_total` | 1 |
| 指標名稱 | `<舊名>_payuni_errors_total` | `hawkr_payuni_errors_total` | 1 |
| HTTP header | `X-<舊名>-Key-Id` | `X-Hawkr-Key-Id` | 1 |
| HTTP header | `X-<舊名>-Timestamp` | `X-Hawkr-Timestamp` | 1 |
| HTTP header | `X-<舊名>-Nonce` | `X-Hawkr-Nonce` | 1 |
| HTTP header | `X-<舊名>-Signature` | `X-Hawkr-Signature` | 1 |
| HTTP header | `X-<舊名>-Event-Id` | `X-Hawkr-Event-Id` | 1 |
| compose 網路 | `<舊名>-network` | `hawkr-network` | 1 |

其他未列出的 `<舊名>-<後綴>` 形式（`<舊名>-website`、`<舊名>-logo`、`<舊名>-postgres` 等）一律走 A07；`<舊名>-food`、`<舊名>-seller-prod` 等外部名稱走 A02 或步驟 2。

### 6.4 Java 套件與 Maven 座標

15 個 pom.xml。groupId 與 aggregator 的 artifactId 的對照如下（`<name>` 欄位是顯示名稱，依 A07 的 Title 形狀改）：

| pom.xml | 座標與新值 |
|---|---|
| `backend/services/java/aftersales-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/cart-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/coupon-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/media-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/order-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/payment-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/payuni-webhook-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/pom.xml` | `<舊名>-backend-aggregator` → `hawkr-backend-aggregator`；`com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/product-import-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/product-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/shared/common-core/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/shared/java-proto/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/shared/observability-common/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/shared/payuni-java-client/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |
| `backend/services/java/shipping-service/pom.xml` | `com.<舊名>` → `io.github.will413028.hawkr` |

Java 原始碼與測試的 `package`／`import`、`src/main/java/com/<舊名>/…` 目錄一併依 A03、A15 改；`application.yml` 的屬性前綴（K17）、`@ConfigurationProperties(prefix=…)`、`logging.level.com.<舊名>`、MyBatis 的 `mapper-locations` 與 `type-aliases-package` 同步（以測試啟動 Spring context 驗證）。Maven Central 承認 `io.github.<帳號>` 命名空間（D4），所以座標不需要自己的網域。

### 6.5 App 的 bundle ID 與 applicationId（D4）

| 現值（代號化） | 新值 | 出現的檔數 |
|---|---|---|
| `merchant.food.<舊名>.sandbox` | `merchant.io.github.will413028.hawkr.sandbox` | 7 |
| `food.<舊名>.seller` | `io.github.will413028.hawkr.seller` | 7 |
| `merchant.food.<舊名>` | `merchant.io.github.will413028.hawkr` | 6 |
| `food.<舊名>.buyer` | `io.github.will413028.hawkr.buyer` | 6 |
| `food.<舊名>.seller.dev` | `io.github.will413028.hawkr.seller.dev` | 5 |
| `food.<舊名>.seller.stage` | `io.github.will413028.hawkr.seller.stage` | 5 |
| `food.<舊名>.buyer.dev` | `io.github.will413028.hawkr.buyer.dev` | 4 |
| `food.<舊名>.buyer.stage` | `io.github.will413028.hawkr.buyer.stage` | 3 |
| `food.<舊名>.user.RunnerTests` | `io.github.will413028.hawkr.user.RunnerTests` | 1 |
| `food.<舊名>.web` | `io.github.will413028.hawkr.web` | 1 |

- Android 的 `applicationId` 不在追蹤中的檔裡（`git ls-files | grep -i '(^|/)android/'` 是 0）；步驟 3c 建立 Android 設定時直接用 `io.github.will413028.hawkr.buyer`、`.seller`。
- App 的 URL scheme（K13）：`<舊名>buyer://`、`<舊名>seller://`、`<舊名>://`，與 iOS `Info.plist` 的 `CFBundleURLSchemes`、Dart 的路由與後端回跳網址同步改。
- 需要在外部服務重建、不是改檔就能完成的識別碼：Apple Developer 的 App ID 與 Merchant ID、Clerk 的 redirect URL 與 OAuth 應用、Firebase 專案（步驟 2 已移除）。這些不屬於 rename.py。

### 6.6 proto package

75 個 `.proto` 檔、37 個不同的 package。`api/<舊名>/` 目錄改名（第 3 節），`import` 路徑、`buf.yaml` 的 `lint.ignore`／`breaking` 路徑與模組名（K18）同步。

| package（代號化） | 新 package | 檔數 | 類別 |
|---|---|---|---|
| `com.<舊名>.payment.grpc` | `com.hawkr.payment.grpc` | 2 | 舊 gRPC |
| `<舊名>.account` | `hawkr.account` | 4 | 舊 gRPC |
| `<舊名>.admin.buyer.v1alpha1` | `hawkr.admin.buyer.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.admin.content.v1alpha1` | `hawkr.admin.content.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.admin.identity.v1alpha1` | `hawkr.admin.identity.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.admin.reviewqueue.v1alpha1` | `hawkr.admin.reviewqueue.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.admin.seller.v1alpha1` | `hawkr.admin.seller.v1alpha1` | 6 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.aftersales` | `hawkr.aftersales` | 2 | 舊 gRPC |
| `<舊名>.agent.v1alpha1` | `hawkr.agent.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.ai` | `hawkr.ai` | 2 | 舊 gRPC |
| `<舊名>.buyer` | `hawkr.buyer` | 2 | 舊 gRPC |
| `<舊名>.buyer.v1alpha1` | `hawkr.buyer.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.cart` | `hawkr.cart` | 2 | 舊 gRPC |
| `<舊名>.category` | `hawkr.category` | 2 | 舊 gRPC |
| `<舊名>.common` | `hawkr.common` | 2 | 舊 gRPC |
| `<舊名>.common.v1alpha1` | `hawkr.common.v1alpha1` | 6 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.content.v1alpha1` | `hawkr.content.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.coupon` | `hawkr.coupon` | 2 | 舊 gRPC |
| `<舊名>.crm` | `hawkr.crm` | 2 | 舊 gRPC |
| `<舊名>.discovery.v1alpha1` | `hawkr.discovery.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.gateway` | `hawkr.gateway` | 2 | 舊 gRPC |
| `<舊名>.identity.v1alpha1` | `hawkr.identity.v1alpha1` | 1 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.im` | `hawkr.im` | 2 | 舊 gRPC |
| `<舊名>.invoice` | `hawkr.invoice` | 2 | 舊 gRPC |
| `<舊名>.media` | `hawkr.media` | 2 | 舊 gRPC |
| `<舊名>.membership.billing.v1` | `hawkr.membership.billing.v1` | 2 | 舊 gRPC |
| `<舊名>.membership.v1` | `hawkr.membership.v1` | 2 | 舊 gRPC |
| `<舊名>.notification` | `hawkr.notification` | 2 | 舊 gRPC |
| `<舊名>.order` | `hawkr.order` | 2 | 舊 gRPC |
| `<舊名>.pricebook` | `hawkr.pricebook` | 2 | 舊 gRPC |
| `<舊名>.product` | `hawkr.product` | 2 | 舊 gRPC |
| `<舊名>.product_import` | `hawkr.product_import` | 2 | 舊 gRPC |
| `<舊名>.review` | `hawkr.review` | 2 | 舊 gRPC |
| `<舊名>.seller` | `hawkr.seller` | 2 | 舊 gRPC |
| `<舊名>.seller.v1alpha1` | `hawkr.seller.v1alpha1` | 2 | 對外 API（`v1alpha1` 不改） |
| `<舊名>.shipping` | `hawkr.shipping` | 2 | 舊 gRPC |
| `<舊名>.social` | `hawkr.social` | 2 | 舊 gRPC |

- 線路上的識別碼（`/<package>.<Service>/<Method>`、JSON 的 `@type`）跟著 package 變；`v1alpha1` 的 `buf breaking` 還沒生效（上線前），所以不需要相容。四個生成端與 `api-contract.yml` 的 drift 檢查是驗證手段（步驟 3 驗收）。
- uitext catalog 的 key 是不帶 package 前綴的 descriptor 路徑（`buyer.Gender.…`），不含舊名；`make uitext` 重新生成即可。

## 7. 縮寫（D6）的邊界規則與撞名風險

- 指令：各家族的計數見第 1.2 節 A08 到 A14；檔案分佈與撞名搜尋可用 `python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A08` 等。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。

### 7.1 邊界規則

- **左邊界**：縮寫前面一個字元不是英數字（開頭、空白、引號、`-`、`_`、`.`、`/`、`(`、`:`、`=` 等都算邊界）。這樣 `--<縮寫>-primary`、`text-<縮寫>-muted`、`lib/…/<縮寫>_button.dart`、`.<縮寫>-card` 都會被改，`pdf_`、`pdf-` 不會（前面是字母 `p`）。
- **補充（A18 到 A27）**：左邊界另外接受正則跳脫（`\b`）與 `(?:`；右邊界另外接受冒號、`ui`＋非英數字、小寫單字簽章前綴（`<縮寫>origin1:`）。`scan.py` 的 brand-abbr 用同樣邏輯（`abbr_extra`），並多一列寬鬆偵測（uncertain）供人工確認剩下的都是不相關類別。
- **右邊界**：連字號或底線本身就是右邊界（A08、A09）；Pascal 與 camelCase 家族要求縮寫後面是大寫字母加小寫字母（`<縮寫Pascal>` 後接 `[A-Z][a-z]`）。
- **大小寫**：A08、A09 只改小寫；`<縮寫大寫>-` 與 `<縮寫大寫>_`（A10）另外處理，因為出現在資料值。
- **不改的路徑**：`pnpm-lock.yaml`、`pubspec.lock`、`go.sum`、`atlas.sum`（雜湊與 base64 字串會誤中 A12、A14，而且由工具重算）；Lottie JSON 內的亂數 id（整批在 Q-A7 預設刪除）。

### 7.2 家族與數量

| 規則 | 家族 | 檔數 | 處數 | 檔案類型（前 6） |
|---|---|---|---|---|
| A08 | `<縮寫>-` | 305 | 2462 | tsx 162、ts 82、css 31、md 15、dart 8、go 3 |
| A09 | `<縮寫>_` | 547 | 1199 | dart 471、ts 40、md 18、go 10、json 3、mdx 2 |
| A10 | `<縮寫大寫>-／_` | 62 | 176 | ts 18、go 18、md 13、dart 8、proto 2、template 1 |
| A11 | `<縮寫>ui-、<縮寫>h-` | 325 | 8310 | tsx 229、css 45、ts 39、md 7、dart 3、json 2 |
| A12 | `<縮寫Pascal>` | 431 | 5686 | dart 411、md 8、json 7、svg 2、mdx 2、mjs 1 |
| A13 | `<縮寫>Log` | 113 | 700 | dart 108、md 3、swift 2 |
| A14 | `<縮寫camel>` | 12 | 79 | ts 7、json 4、tsx 1 |

### 7.3 撞名與誤中的風險（已找到的）

- 目標字串本身在樹裡已存在：`hk-`、`hk_` 只出現在計畫與 ADR（排除範圍）；`Hk`＋大寫小寫的現有出現都在 Lottie JSON 與 `pnpm-lock.yaml` 的亂數字串裡，不是識別碼，沒有語意撞名。
- `pdf_`、`pdf-` 一類以縮寫字母結尾的單字：左邊界擋掉，已用全樹搜尋確認 `<縮寫>-`／`<縮寫>_` 的命中前一個字元沒有英數字。
- 十六進位與 base64：`dfe9d9` 之類的色碼不含連字號或底線，不會被 A08、A09 命中；A12 與 A14 會誤中亂數 id，所以要排除鎖定檔與 Lottie JSON（第 7.1 節）。
- 單獨出現、後面不接連字號或底線的縮寫字母：步驟 3 修正輪 1 逐項盤點後，品牌寫法（`<縮寫>ui`、`<縮寫>:`、簽章前綴、正則 alternation 與跳脫、header 與活動代碼）由 A18 到 A27 改；其餘（Unix `<縮寫>` 指令、十六進位色碼、雜湊與 base64、`SIG_DFL` 一類信號常數）不是品牌，不改。
- 大寫的 `<縮寫大寫>-`、`<縮寫大寫>_`：是 HTTP header、ID 格式與測試常數，見第 8 節。

## 8. 撞名清單（步驟 3 一次處理的清單）

同一個字串在兩處代表不同東西，不能只靠通用規則。D8=A 已決：X1 到 X4 與 X7 全部改成 `HK` 系列，列於下表的連動清單，步驟 3 一次處理，不再是停止條件；步驟 3 的停止條件只適用於改名過程中新發現、不在此表的撞名。

- 指令：`python3 backend/docs/plans/2026-10-24-portfolio-public/gen_rename_map.py --count A10`（大寫縮寫）；其餘類型見各列。
- 快照：e5d5a8d8e（程式碼與 332da5e51 相同）。

| 代號 | 字串 | 兩個意思 | 數量 | 處理 | 去向 |
|---|---|---|---|---|---|
| X1 | `<縮寫大寫>-Operation-Session`、`<縮寫大寫>-Origin-Auth`、`<縮寫大寫>-Operation-Credential` | HTTP header 名稱（admin BFF、閘道、單體、測試與設計書） | 34 個檔、94 處 | 改成 `HK-…`，所有端同步（線路契約）；兩端任一漏改會讓代行授權失敗 | 改（A10） |
| X2 | `<縮寫大寫>-YYYY-NNNN` 格式的代行工作階段編號與 `<縮寫大寫>-<數字>` 的測試值 | 資料值的格式（後端產生、CHECK 或正則驗證、測試 fixture）與程式識別碼同一個字串 | 29 個檔、78 處 | 格式前綴與驗證正則、CHECK 約束、種子資料同步改；migration 內容會變，要重算 `atlas.sum` | 改（D8=A 已決）；連動：產生碼的格式前綴、驗證正則、CHECK 約束、種子資料與 fixture |
| X3 | `<縮寫大寫><日期><流水號>`（6 位數以上） | 訂單與出貨單編號的前綴：產生碼與大量測試 fixture 都用同一個字串，且不帶連字號，A08 到 A10 不會命中 | 27 個檔、82 處 | 前綴換成 `HK`；連動：訂單與出貨單編號的產生碼、驗證正則、CHECK 約束、種子資料與所有 fixture 同步改（本列不帶連字號，A08 到 A10 命中不到，rename.py 要有專屬規則） | 改（D8=A 已決） |
| X4 | `<縮寫大寫>` 加 3 到 5 位數 | 測試用編號 | 1 個檔、7 處 | 同 X3 的連動清單（測試編號的 fixture 與斷言） | 改（D8=A 已決） |
| X5 | 舊 GitHub 組織名與舊 registry 名稱 | 外部擁有的名字（issue 連結、`ghcr.io/…`、`<舊名>-food/…`） | 65 個檔、169 處 | A02 保護；改成通用規則會產生不存在的連結；連結由步驟 6 改成新 repo 的 issue 或拿掉 | 不改（A02） |
| X6 | `<舊名>_…-<編號>` 形狀的 Sentry 問題編號 | 舊公司 Sentry 專案的外部識別碼（出現在註解與文件） | 2 個檔、2 處 | 不是我們的名字；改名會變成不存在的識別碼，應整段註解改寫或刪除（步驟 6） | 刪除註解，不改名 |
| X7 | `<縮寫大寫>_ECHO_*` 等測試常數 | 測試環境的常數名 | 1 個檔、2 處 | A10 一併改 | 改（A10） |
| X8 | 字型、Lottie 與其他有內容雜湊的檔 | manifest 內記錄了檔案的 sha256；改名或重新生成檔案後雜湊不同（詳見下方雜湊綁定） | 見下表 | 重新生成並更新 manifest，或整批刪除（Q-A5、Q-A7） | 重新生成或刪除 |
| X9 | 舊 Apple Merchant ID 與 Clerk、Firebase、OAuth 的外部設定 | 外部服務裡以舊名建立的資源（不是檔案） | - | 改名只改檔案；外部資源要在新帳號重建（第 6.5 節） | 不屬於 rename.py |
| X10 | 英文 10 個字元變成 5 個字元 | 測試裡以字串長度、欄寬、截斷或排序為前提的斷言（用舊名組出邊界長度） | 無法機械搜尋 | 全測試套件跑一次，失敗的逐項看；不是風險「已排除」 | 改後跑測試 |

雜湊綁定（檔案內容的 sha256 或 sha1 被別的追蹤檔記錄，內容或檔名變動後會對不上）：

| 記錄雜湊的檔（代號化） | 被記錄的檔數 | 性質 | 處理 |
|---|---|---|---|
| `website/assets/fonts/<舊名>-rounded-v2/manifest.json` | 83 | 字型（Q-A5） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/capabilities/manifest.json` | 3 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/entry-thinking-cover.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/01-vegetables-water-bamboo.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/02-fruits-pomelo.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/03-seafood-pacific-saury.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/04-meat-duck-breast.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/05-egg-dairy-cheese-wheel.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/06-processed-dried-fruit.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/07-spices-red-chili.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/08-grains-rice-ear.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/autumn/09-flowers-chrysanthemum.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/01-vegetables-spring-bamboo-shoot.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/02-fruits-spring-green-plum.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/03-seafood-spring-flying-fish.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/04-meat-spring-herb-chicken-leg.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/05-egg-dairy-spring-soft-boiled-egg.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/06-processed-spring-tofu-cube.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/07-spices-spring-garlic.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/08-grains-spring-wheat-ear.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/spring/09-flowers-spring-white-calla-lily.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/01-vegetables-summer-okra.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/02-fruits-summer-irwin-mango.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/03-seafood-summer-spear-squid.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/04-meat-summer-grilled-pork-belly.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/05-egg-dairy-summer-yogurt-cup.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/06-processed-summer-popsicle.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/07-spices-summer-thai-basil.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/08-grains-summer-corn.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/summer/09-flowers-summer-hydrangea.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/01-vegetables-cauliflower.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/02-fruits-jujube.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/03-seafood-mullet.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/04-meat-beef-hotpot-roll.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/05-egg-dairy-butter-block.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/06-processed-dumpling.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/07-spices-ginger.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/08-grains-red-bean-sack.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/assets/lottie/shared/<舊名-子品牌>/seasonal-contained-v3/winter/09-flowers-poinsettia.lottie.json` | 1 | Lottie／角色素材（Q-A1、Q-A7） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/lib/animation/<舊名-子品牌>-season.ts` | 35 | 其他 | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/lib/content/generated/seller-guide-manifest.json` | 15 | 其他 | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/lib/fonts/README.md` | 1 | 字型（Q-A5） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/public/images/<舊名-子品牌>/seasonal-food-only-v3/manifest.json` | 1 | 其他 | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/public/images/lossless-icons.manifest.json` | 6 | 其他 | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/public/images/responsive-seasonal-icons.manifest.json` | 7 | 其他 | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/scripts/fonts/check-build.mjs` | 1 | 字型（Q-A5） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/scripts/fonts/generate_openhuninn.py` | 1 | 字型（Q-A5） | 改檔名或內容後重新生成 manifest；或隨素材刪除 |
| `website/tests/font-loading-contract.test.ts` | 1 | 其他 | 改檔名或內容後重新生成 manifest；或隨素材刪除 |

## 9. 排除清單與生成物

### 9.1 rename.py 不改內容的檔

| 類別 | 範圍或數量 | - | 處理 |
|---|---|---|---|
| 計畫與附件 | `backend/docs/plans/2026-10-24-portfolio-public.md`、`backend/docs/plans/2026-10-24-portfolio-public/` | - | 計畫檔與附件目錄（`scan.py` 同樣排除；計畫的 `<舊名>` 代號保留，隨新 repo 公開） |
| `.local/` | 不進版控 | - | 明文詞表與搬 issue 的腳本 |
| `pnpm-lock.yaml` | 1 個檔 | - | `pnpm install --lockfile-only`（改完 package.json 與 pnpm-workspace.yaml 後） |
| `pubspec.lock` | 7 個檔 | - | 各套件目錄 `fvm flutter pub get` |
| `go.sum` | 12 個檔 | - | 各 module `GOWORK=off go mod tidy`（local module 不進 go.sum；有舊名的字串只可能出現在雜湊誤中） |
| `package-lock.json` | 1 個檔 | - | `npm install --package-lock-only`（`backend/workers/image-gateway`） |
| `atlas.sum` | 19 個檔 | - | 各模組 db 目錄 `atlas migrate hash --env dev`（遷移檔內容變了，雜湊要重算；已進 main 的 migration 可改寫，上線前沒有相容性約束） |
| `legacy-endpoints` 規則命中處（A01） | 308 個檔、1269 處 | - | 步驟 2 先清到 0；此前 rename.py 的 skip 規則保護 |

### 9.2 生成物與生成器（改來源、重新生成，不用文字取代）

| 生成物 | 輸出位置 | 生成指令與注意 |
|---|---|---|
| `api/` 的對外 API | `backend/internal/httpapi/gen/`、`backend/internal/legacy/gen/`（`make connect`）；`packages/<舊名>-api-ts/src/gen/`（`pnpm --filter @hawkr/api gen:api`）；`packages/<舊名>_api/lib/gen/`（`../../api/generate.sh buf.gen.yaml`，先暖機兩個 dart 外掛，見 `AGENTS.local.md`） | buf 版本看 `api/.buf-version`，PATH 先放 `$HOME/.local/bin`；離線時 BSR 檢查跑不了 |
| 舊 gRPC 的 proto | `backend/pkg/proto/**`（Go）與 `backend/services/java/shared/java-proto`（Java）；來源 `backend/api/proto/**` | `backend/buf.gen.legacy.yaml` 與 `buf.gen.yaml` |
| 共用文案目錄 | `backend/internal/platform/uitext/catalog_gen.go`、`packages/<舊名>-ui-text-ts` 的 `*.gen.*`、`packages/<舊名>_ui_text` 的 `*.g.*`、golden 檔 | `make uitext`（`backend/cmd/uitext-gen`） |
| twaddr | `packages/<舊名>_twaddr/lib/src/twaddr_data.g.dart`、`packages/<舊名>-twaddr-ts/src/twaddr.gen.ts` | 由 `backend/internal/platform/twaddr` 的 CSV 生成 |
| 法律頁 | `website/features/legal/documents/*.gen.ts` | 由 `website/docs/legal/*.md` 生成（步驟 4 先改寫文件） |
| sqlc 與 go-jet | 各模組 `internal/store`、`internal/jet`、`internal/pubsql` | `make sqlc`、`make jet`（需要 Docker；`make sqlc` 會先 `rm -rf internal/store`，先 `docker info`） |
| CodeMeta | Go、go-media、web、admin、flutter、Java 六種輸出 | `generate.sh --all`（Java 產生器不刪舊檔） |
| App 的 build_runner | `*.g.dart`、`*.freezed.dart`（兩支 App） | `fvm dart run build_runner build --delete-conflicting-outputs`；CI 的生成檢查 |
| website 內容清單 | `website/lib/content/generated/seller-guide-manifest.json` | 改來源後重新生成（內含舊設計工具的檔案識別碼，步驟 6 一併移除） |

## 10. 重跑與驗證

- `rename.py`（步驟 3）讀 `.local/publish/rename-map.plain.tsv`；先 skip，再依本檔規則表的順序改寫；可重跑，第二次執行 `git status` 必須乾淨。
- 改完以 `python3 backend/docs/plans/2026-10-24-portfolio-public/scan.py --category brand --english` 驗證英文部分是 0，`--category brand-abbr`、`--category brand-path` 也要是 0（`brand-abbr` 含 D6 沒列出的家族，見 D8=A）。
- mutation：把一個 proto 檔的 package 改回舊名，四個生成端之一的 drift 檢查與 `scan.py` 都要失敗（計畫步驟 3 驗收）。

