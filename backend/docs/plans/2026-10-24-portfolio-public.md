# 作品集轉 public：改名為 Hawkr，以單一初始 commit 開新的 public repo

**Goal：** 新的 public repo `Will413028/hawkr` 以單一初始 commit 開始，樹裡沒有下列任何一項：

- 舊品牌名
- 舊公司的法人與合作方
- 同事資訊
- 真實個資
- 未授權素材
- 舊公司的端點與 secrets

四個 client 與單體都在新名字下 build、測試通過，CI 在 public repo 逐 job 綠燈。舊 repo 改名封存為 private。終態的驗收見步驟 9。

**上層：** 不屬於 `2026-09-24-go-modular-monolith.md` 的搬遷階段；主清單見本機個人脈絡檔。本檔暫代「全域約束與完成定義」。

**附件：** `2026-10-24-portfolio-public/`，由步驟 1 產出：

- `scan.py`
- `inventory.md`
- `ci-local-map.md`
- `rename-map.md`

快照 commit `e5d5a8d8e`。

## 用語

- **舊名**：改名前的品牌名，包含：
  - 英文的各種寫法，可重現的定義是 denylist 第一列的不分大小寫 pattern（允許中間有 `-`、`_` 或無分隔）；
  - 中文名；
  - 設計 token 等處用的兩字母縮寫（D6）。

  本計畫與附件一律寫成 `<舊名>`，不寫明文。
- **denylist**：
  - 包含舊名的中文、舊公司的法人與合作方、舊網域、同事的姓名與帳號。
  - 明文只放在不進版控的 `.local/publish/denylist.txt`；`.local/` 由 `.git/info/exclude` 排除。
  - 同目錄另有 issue 搬移腳本 `migrate.py` 與同事比對規則，也不進版控。

## 全域約束與完成定義（暫代）

- denylist 的任何一項都不寫進追蹤檔，本計畫與附件也一樣。盤點裡的作者欄只寫「Will」或「他人」。
- 不使用未授權的素材（Will 2026-10-18）。老闆同意的範圍只有「程式碼」，下列項目都不在範圍內：
  - 文案、圖片、品牌、法律頁。
  - 公司時期的設計書、規範與供應商文件例外：Will 決定公開（D1），清洗品牌、人名與供應商的特店資訊後保留。
- 不連到舊公司（根目錄 `AGENTS.md` 共通規則）。Firebase、Sentry、OAuth 的設定與 key 一律移除，不能只是改名。
- 本機不跑 e2e 與瀏覽器測試（`AGENTS.local.md`）。CI 目前也完全不跑 e2e（2026-10-24 量測：`grep -rlE 'playwright|test:e2e|storybook:test' .github/workflows` 是 0），處理方式見 D5。
- 改名分支不推到舊 repo，因為 private repo 的 Actions 額度有限。CI 等價檢查依 `ci-local-map.md` 在本機跑，CI 本身在步驟 8 的新 repo 跑。
- `rename.py` 排除本計畫與附件。
- 步驟 5 到 7 的本機驗收不跑完整測試（Will 2026-10-26）：兩支 App 的完整 `flutter test`、website 與 admin 的整套 unit 測試，以及 backend 全部 module 的 test，都延到步驟 8 由新 repo 的第一輪 CI 跑。本機只跑改到的部分的測試，加上 analyze、lint、build 與生成檢查。這台機器 load average 長期在 250 以上，一支 App 的完整測試要 30 分鐘以上。
- 完成定義：
  1. `scan.py --all` 的品牌、同事、個資、舊端點四類都是 0。
  2. gitleaks 掃追蹤中的檔是 0。
  3. `ci-local-map.md` 的每一列（e2e 與完整測試除外）在本機通過；完整測試由第 4 項的 CI 涵蓋。
  4. 新 repo 第一輪 CI 的每個 job 綠燈。

## 前提（開工時核對過，2026-10-24）

- 會改到或取代的既有部署是否在線：不在線。系統從未上線，舊公司的 infra 也不再連線（根目錄 `AGENTS.md`）。改名不受相容性約束：DB 名、queue 名、環境變數前綴直接改，不留別名。
- 被改的程式會不會被重寫、搬走或刪除：會。
  - `backend/services/`（含 10 個 Java 服務，1,842 個檔）與 `backend/gateway/` 預定在主計畫的階段 B、C 刪除。含舊名的 6,485 個檔，有一半以上在這兩個目錄。
  - 但單體還靠它們跑沒搬完的模組（主計畫 L175、L237），不能先拿掉。
  - 所以這部分只用腳本機械式改名，人工只做清洗，不做其他投入。
- 步驟 3 完成後改名就合併進本機 main，步驟 4 到 7 在 main 上做；其他工作（含 A7）可以照常進 main，一律用新名字寫，`rename.py --check` 守住舊名不再混入（Will 2026-10-25，取代原本「本階段 main 不進其他工作、A7 停在 A7-2v 之前」的預設）。必要條件：main 不推舊 repo（不耗 Actions 額度），新 repo 由最後的樹匯出單一初始 commit，main 上的中間狀態不會公開。
- 新名字是 Hawkr，中英文介面都用「Hawkr」。暫不申請商標與網域。logo 用 `brand/hawkr/`（Will 2026-10-23，已確認）。
- 開新 repo，只放單一初始 commit，不帶公司時期的歷史。舊 repo 封存為 private（Will 2026-10-23，已確認）。
- 本階段不換金流與物流：PayUni、AMEGO、7-ELEVEN 換成 Stripe Connect 與綠界是之後的階段（Will 2026-10-23，已確認）。它們的整合程式碼保留；供應商文件與測試帳號資訊依 D1 與步驟 6 處理。
- 領域通用化不在本階段，例如 seller 的食品登錄改成 `Permit`、示範品類改成一般電商。那是作品集內容的取捨，不是授權問題，照 Pending 原本的觸發點處理。
- 文件與預設設定裡的主機名改用保留網域：說明用 `example.com`（RFC 2606），本機用 `*.localhost`（RFC 6761）。反向 DNS 的前綴看 D4（預設，可推翻）。
- 舊 repo 的 main 領先 origin 的 commit（A7-1b、Hawkr 素材、本計畫與步驟 1 附件）只在本機。開工前先推「還沒改名」的 main，留下完整的私人封存，也避免之後推改名後的 main 觸發舊 repo 的 CI（Will 2026-10-24 授權）。這次之後，main 不再推到舊 repo。

**驗收基線：** 步驟 1 在 `e5d5a8d8e` 的 detached worktree 依 `ci-local-map.md` 建立（2026-10-24）。既有失敗 4 列，不在本階段修：

- `backend-monolith / lint`：CI 固定的 golangci-lint 報一般 22 項、integration tag 35 項。
- `backend-integration-tests` 的 pricebook-service：`internal/usecase` 兩項 rollup 測試把日期寫死在 2026-08，和現在的時鐘比而失敗。
- `buyer-ci / Generated code`：`session_provider.dart` 在 `83c98ca2a` 改過，`.g.dart` 沒有重新生成。
- `workspace-ci / deps`：`pnpm audit --prod` 報 16 個弱點，隨 registry 的公告變動。

NOT RUN 與負載造成的假失敗見 `ci-local-map.md` 最後一節。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 生成物由唯一來源生成，不手改。改名只改來源與生成器，改完重新生成 | <ul><li>`api/` 的四個生成端（backend `make connect`、TS 套件 `gen:api`、Dart 套件 `generate.sh`）</li><li>`make uitext`</li><li>twaddr CSV 生成的兩個套件</li><li>`website/docs/legal/*.md` 生成的 `features/legal/documents/*.gen.ts`</li><li>各模組 sqlc、go-jet 生成的 `internal/store`</li><li>CodeMeta 的 `generate.sh --all`</li></ul> |
| proto 的 package 名就是線路上的識別碼 | 四端生成物的 import、單體的 `connectServices`、`buf breaking` 的基準（`api-contract.yml`）、uitext catalog 的 descriptor key |
| migration 被 `atlas.sum` 鎖住 | 各模組的 `db/migrations` 與 `atlas.sum`；改了 DB 名或 schema 名要重算 sum。上線前可以改寫已進 main 的 migration |
| 舊公司的識別碼在改名過程中仍要偵測得到 | <ul><li>`scripts/check-legacy-endpoints.py` 有 10 類規則（主機名、IP、R2 account、OAuth client、registry 等），其中有些規則本身含 `<舊名>`</li><li>`scripts/legacy-endpoints-baseline.json` 放行了 259 個檔、996 處</li></ul>所以舊服務設定先在步驟 2 移除，`rename.py` 在步驟 3 才跑，並跳過這 10 類規則命中的位置 |
| 舊 repo 的 commit 與 issue 參照在新 repo 會失效 | 53 個檔有 issue 或 PR 連結，16 個檔有 `git show <sha>` 指引（審查者量測，步驟 1 重算）；計畫與 ADR 的 commit hash |

## 規模

| 類別 | 數量（`e5d5a8d8e`） | 產生計數的指令 |
|---|---|---|
| 追蹤中的檔 | 13,505 | `git ls-files \| wc -l` |
| 含 `<舊名>` 英文寫法的檔 | 6,485 | `scan.py --category brand --english \| wc -l` |
| 含 `<舊名>` 英文寫法的檔，依目錄 | `backend/services/java` 1,717、`backend/internal` 617、website 565、seller-app 535、buyer-app 451、gateway 392，其餘由 `scan.py` 輸出 | 同上，依第一、二層目錄聚合 |
| 含 `<舊名>` 中文的檔 | 301 | `scan.py --category brand` |
| 舊網域 | 264 個檔 | `scan.py --category endpoint` |
| 舊法人與合作方名稱 | 4＋16 個檔 | 同上 |
| 圖片與字型 | 1,562：website 506、seller-app 558、buyer-app 467、admin 21、其他 10 | `git ls-files \| grep -E -i '\.(png\|jpe?g\|webp\|svg\|ico\|gif\|ttf\|otf\|pdf)$'` |
| 同事比對 | 385 個檔、1,529 處；其中 1,193 處來自 email 與手機的通用規則，多半是測試假資料，在步驟 1 分類 | `.local/publish/colleague_scan.py`，不印名字 |
| 測試資料的 email | 至少 3 個疑似真實業者的網域共 160 處、gmail 34 處、舊網域 75 處 | `git grep -h -o -I -E '<email 規則>'`，依網域聚合 |
| 公司時期的設計文件 | `backend/docs` 下的 `詳細設計書` 71、`基本設計書` 10、`開發規範` 24、`基礎建設` 12、`第三方整合` 8、`runbooks` 19 等；`詳細設計書` 的 commit 有三分之二是他人 | `git ls-files backend/docs`；`git log --format=%an -- <目錄>` |
| 舊公司的外部服務設定 | 兩支 App 的 Firebase options（5 個檔）、admin 的 Sentry 設定、`notification-service` 的 Firebase | `git ls-files \| grep -E -i 'google-services\|GoogleService-Info\|firebase\|sentry'` |
| workflow | 26 支；用到的 secrets 0 個；沒有 `pull_request_target`；pre-commit 只裝了 `commit-msg` hook | `ls .github/workflows`、`git grep 'secrets\.' .github`、`.pre-commit-config.yaml` |
| gitleaks（追蹤中的檔） | 72 筆：步驟 1 逐筆分類為雜湊誤報 33、測試值 35、要人看或私鑰標頭字面 4；其中 S1、S2 見步驟 2、6 | `inventory.md` 第 5 節 |

## 範圍

- 做：
  - 移除舊公司的外部服務設定。
  - 識別碼改名。
  - 品牌、文案、logo、app icon、法律頁。
  - 素材替換。
  - 清洗文件、測試資料與 agent 指令檔。
  - 守門改造。
  - 依 D5 處理 e2e 的 CI。
  - 開新 repo 與切換。
  - 依 D3 處理 issue。
- 不做：
  - 換金流供應商：之後的階段，已決定。
  - 領域通用化：不是授權問題。
  - 刪除 legacy 服務：主計畫階段 B、C。
  - 把對外 API 從 `v1alpha1` 改名成 `v1`：主計畫排在上線前做。在這裡做的話，`buf breaking` 會提早開始擋 A7 之後的契約變更。
- 延後：
  - 申請商標與網域。觸發條件：要部署 demo 時；屆時把反向 DNS 前綴改成自己的網域。
  - 刪除舊 repo。觸發條件：新 repo 用滿 3 個月，而且沒有回頭查舊歷史的需要；刪除前問 Will。
  - 兩支 App 既有的圖片載入失敗佔位（各數十處以色塊加 `AppIcons.imageOutlined`、`brokenImage` 等手寫，`rg -n "AppIcons\.(imageOutlined|imageNotSupported|brokenImage)" lib`）改用 5e 的佔位元件。它們不是 G11 素材，與授權無關；5e 只統一自己碰到的位置與 `AppCachedImage`。觸發條件：改到那些元件時順手換，或新 repo 的 CI 視覺回歸建立基準之後一次換。
  - 品牌色的設計延伸（D11 收斂規則；觸發條件：新 repo 的 CI 視覺回歸建立基準之後）：
    - 暗色模式沒有覆寫的 25 條規則（`contrast.py` 的 `KNOWN_DARK_GAPS`，只擋增加），以及 seller-center 暗色 `.is-info` 圖示在卡片底上 2.72–2.98（未到 3:1；腳本以 page 底合成，判成 3.39）。
    - 星星評分圖示用輔色，對 cream 約 2.0（舊金色也差不多；`color_policy_test` 釘住 `rating`、`gold` 等於輔色）。
    - 危險色 `#B33A0B` 與品牌橘 `#B8441F` 同色相，seller dashboard 的正負成長色幾乎同色（數字有 +/- 號）；rich text 色盤有兩格幾乎同色。
    - `contrast.py` 的偵測缺口：CSS 註解裡的逗號會被當成選擇器（`KNOWN_DARK_GAPS` 的 seller-center 2 筆中有 1 筆是誤判）；暗色檢查用 page 底，不是元件所在的卡片底。
    - App 圖示的 dev／stage 版不再以底色區分買家與賣家（只剩 DEV／STAGE 斜帶與顯示名稱）；iOS 原生啟動畫面仍是透明圖；新的橫式 lockup 比舊的寬，在原本的 `BoxFit.contain` 框裡略小。

## 步驟

- [x] **1. 盤點與基線（spike）**（被擋於：無）
  - 範圍：
    - 寫附件 `scan.py`：
      - 讀 `.local/publish/denylist.txt`。
      - 依類別掃描追蹤中的檔：品牌、同事、個資（email、手機、身分證、統編、地址）、舊端點（沿用 `check-legacy-endpoints.py` 的 10 類規則，不套 baseline 與 `.md` 排除）、素材、文件作者。
      - 輸出只有路徑、類別與計數，不印命中的字串。
    - 寫附件 `ci-local-map.md`：26 支 workflow 各對到本機的等價指令。e2e 與 Playwright 標「不在本機」；`_flutter-quality` 標成由兩支 App 的 workflow 呼叫；`ios-lockfile-check` 照 `AGENTS.local.md` 只手動跑。
    - 寫附件 `rename-map.md`，逐條列出：
      - 舊字串對到新字串，`<舊名>` 只寫代號，明文對照放 `.local/publish/`；
      - Go module 路徑：`github.com/<舊帳號>/<舊名>/...` 改成 `github.com/Will413028/hawkr/...`，舊服務的 module 一併改成這個前綴；
      - 目錄名、`@hawkr/*` 套件、Dart 套件、DB 與 queue 名、環境變數前綴、容器名；
      - 縮寫依 D6；
      - 排除清單：本計畫、附件、`legacy-endpoints` 規則命中處、`atlas.sum`。
    - 產出附件 `inventory.md`，每一列都有「去向」欄：
      - 素材逐檔：自製、開源套件（附授權）、或來源不明；
      - 文件逐檔：要清洗的類別（品牌、人名、特店資訊）；
      - 個資逐處：假資料或疑似真實；
      - 供應商文件與測試帳號資訊；
      - gitleaks 的 17 筆；
      - issue 或 PR 連結、`git show <sha>` 指引；
      - `.claude/`、`.agents/` 的指令檔與 skill 裡的個人路徑與私人參照。
    - 在 `e5d5a8d8e` 上依 `ci-local-map.md` 全部跑一次，既有失敗記成驗收基線。
  - 消費端：無（只讀）。
  - 不能動：任何追蹤中的檔。
  - 驗收：
    - `scan.py --check-inventory` 的去向空格數是 0；`ci-local-map.md` 涵蓋 `ls .github/workflows` 的每一支。
    - mutation：在附件目錄的暫存複本刪掉一列去向，檢查要失敗；用完刪除複本。
    - mutation：在附件目錄的暫存檔放一個 denylist 詞，`scan.py` 要抓到。
  - 停止條件：發現 secrets 的明文（不是雜湊、也不是測試值）就停下，回報路徑，不印值。
- [x] **2. 移除舊公司的外部服務設定**（被擋於：舊 repo 的 main 先推）
  - 範圍：
    - 刪除兩支 App 的 Firebase options、admin 的 Sentry DSN、`notification-service` 的 Firebase 設定與 OAuth client。改成由環境變數注入，沒有值時停用。
    - gateway 文件檢視器寫死的帳密與簽章 key（`inventory.md` 的 S1）同樣改成環境變數注入，沒有值時停用（Will 2026-10-24）。
    - 舊網域改成保留網域。
    - 自家網域與媒體主機改由環境變數決定（website、admin、buyer 的分享連結），不留保留網域的程式常數；API 主機是必要設定，正式環境缺值時啟動或 build 失敗並指名變數，dev 預設本機單體（步驟 2 審查後決定，2026-10-25）。
    - 本機開發改回 `localhost`，移除自訂主機名與自簽 HTTPS：它的必要條件（與公司測試 API 同 site 共用 cookie）已不存在（Will 2026-10-25）。
    - 刪除公司時期各部署環境專屬的設定（wrangler 的部署 env、image-gateway 的自訂網域路由、Terraform 的 mq env、監控的正式環境部分），保留本機開發、build 與 CI 需要的部分；部署設定等決定 demo 的部署目標時重寫（Will 2026-10-25）。
    - `check-legacy-endpoints.py` 每一類規則在全樹命中 0。
    - `legacy-endpoints-baseline.json` 清空，`.md` 與 `/docs/` 的排除拿掉。
    - `check-legacy-endpoints.py` 新增「GCP 專案編號」規則：Firebase 的 app ID 與 iOS 專案設定以 OAuth client 規則抓不到的形式帶著專案編號（步驟 2 開工時量測：`seller-app/ios/Runner.xcodeproj/project.pbxproj` 12 行只有這條規則抓得到）。`company-infra-ip` 補上規則漏掉的 6 個 IP（dev VM、App VM、CI runner、已除役的 prod-infra VM、GCP 固定出口與文件範例裡的一個真實用戶端 IP；步驟 2 以全樹公網 IPv4 字面值清單逐一對照文件脈絡找出）。
    - 不在本步驟：`scan.py` endpoint 類別裡的法人與合作方名稱（測試資料歸步驟 6，法律頁與頁尾歸步驟 4）、標成 uncertain 的測試資料 email 網域（`<舊網域-9>` 到 `<舊網域-11>`，步驟 6 的 Q-P2）、`check-legacy-endpoints.py` 規則本身的字面值（步驟 7 依 D7 改造）。
  - 消費端：`git grep -l -I -E 'firebase_options_|sentry\.(client|server|edge)\.config|SENTRY_DSN'`；各 App 的 `main_*.dart`；admin 的 `next.config`。
  - 不能動：沒有設定時的行為是「功能停用」，不是當掉。
  - 驗收：
    - `scan.py --category endpoint` 的舊網域代號 `<舊網域-1>` 到 `<舊網域-8>` 與 `legacy:*` 規則是 0（排除 `check-legacy-endpoints.py` 自身）；`check-legacy-endpoints.py` 在空的 baseline 下通過。
    - 兩支 App 的完整 `flutter test`、admin 的 unit 與 build、notification-service 的測試通過。
    - mutation：把一個舊 Firebase project id 放回去，`scan.py` 要失敗。
- [x] **3. 識別碼改名（機械式）**（被擋於：D4、D6、D8、步驟 2；3a 到 3c 必須在同一個分支連著做完才能 build）
  - 範圍：
    - 附件 `rename.py` 依 `rename-map.md` 執行：
      - 可重跑，對已改名的樹再跑一次沒有 diff；
      - 用 `git mv` 改目錄；
      - 內容先換長字串、再換短字串。
    - 改完重新生成不變量表第一列的所有生成物。
    - 3a：Go、proto、TS、Dart 的套件與生成物。
    - 3b：Java 的 package、目錄與 Maven 座標。
    - 3c：App 的 bundle ID 與 applicationId；DB、queue、容器名、環境變數。
    - 執行與審查中補上的規則（明文在 `.local/publish/rename-map.plain.tsv`，代號與理由見 `rename-map.md`）：X3、X4 的專屬規則；A02 多保護不帶網域的舊 org/repo 寫法；A03S 改內容裡的 Java 套件目錄路徑；A18 到 A27 補縮寫在正則跳脫、alternation、小寫單字前綴與冒號寫法的漏網處，`scan.py` 的縮寫類用同一套邊界。
    - Java 套件從舊前綴搬到 `io.github.will413028.hawkr` 之後，Spring Boot 自動設定的隱含字母序改變：`ObsAutoConfiguration` 與 `MqErrorHandlerMetricsAutoConfiguration` 改成明寫 `before`／`after`，不靠套件名排序。後者改名前的條件永遠不成立、從未註冊，明寫之後照 Javadoc 的意圖在有 `MeterRegistry` 時註冊（審查後決定，Will 2026-10-25 確認保留）。
  - 消費端：`scan.py --category brand` 的英文部分要是 0；`.github`、`scripts`、所有 Makefile、`docker-compose*.yml`、`.pre-commit-config.yaml`。
  - 不能動：proto 的欄位編號與 JSON 名，只改 package；migration 只改名字，不改語意。
  - 驗收：
    - `ci-local-map.md` 的每一列（e2e 除外）跑完和基線比較，不得多出失敗。
    - `rename.py` 再跑一次後 `git status` 乾淨。
    - mutation：把一個 proto 檔的 package 改回舊名，四個生成端之一的 drift 檢查與 `scan.py` 都要失敗。
  - 停止條件：同一個字串在兩處代表不同東西，例如資料值和程式識別碼撞名。列出位置後停下。
- [x] **4. 品牌、文案與法律頁**（被擋於：步驟 3、D9、D10；4b 另被擋於 D11）
  - 開工量測（`main` `7e33467ec`，denylist 依 D9 加列之後）：`scan.py --category brand` 計入的 519 個檔、2,387 處；`brand-path` 3 個檔（設計書檔名含中文字首）；`<舊名-子品牌>` 英文 217 個檔（uncertain，D9 歸步驟 5）。
  - 範圍：
    - 術語表（`backend/docs/開發規範/使用者面用語術語表.md`）先改：品牌列的 zh-TW 改成 Hawkr，加上 D9 的衍生名對照；檔頭「賣家面的中文以賣家條款 §17.4、§18 為唯一來源」改成以本表為準（D10）。
    - `<舊名-中文>` 與中文字首的衍生名（D9 的 denylist 列 `<舊名-字首-中文>`）全樹清除：
      - 使用者面文字：uitext catalog、website、admin、兩支 App 的 l10n 與顯示名稱（iOS `BUNDLE_DISPLAY_NAME`、Android label）、email 範本、通知文字、SEO 標題與 JSON-LD。英文介面裡的 `<舊名-子品牌>` 也改成 D9 的新名。
      - 測試資料與 fixture：斷言品牌的改成 Hawkr；當成業者或公司名的換成虛構的中文名（「範例」開頭，例如「範例生鮮股份有限公司」「範例小農」），保持中文字形，測試的意圖不變（中文空白、全形字、公司後綴的正規化）。
      - 設計文件、註解與 README：品牌改成 Hawkr；檔名含中文字首的 3 份設計書以 `git mv` 改名，全樹引用跟著改。這是 D1 清洗的品牌部分，提前在本步驟做；人名與特店資訊仍歸步驟 6。
    - logo、favicon、app icon、啟動畫面、OG 圖改用 `brand/hawkr/`，App 的各尺寸由 SVG 產生，產生指令寫進 `brand/hawkr/README.md`。
    - 法律頁依 D10：隱私、服務條款、退貨、使用者政策四頁（`website/features/legal/pages/`）與 `website/docs/legal/` 的四份文件，內文一律換成簡短的作品集說明（示範專案、非實際營運、不收集真實資料、營運者是 Hawkr 示範專案）。版面元件、路由、錨點與 `seller-agreements.json` 的種類與網址不變；生效日改成本步驟的日期，`generate-legal.ts` 重新生成 website 與 backend `agreement_gen.go`。設計文件與註解裡引用舊條號（例如賣家條款 §18.4）的地方不改，條號指私人封存裡的舊版。
    - 根目錄與各子專案的 README、`AGENTS.md` 改成 Hawkr 的敘述。
    - 品牌色依 D11（子步驟 4b，在上面的文字、logo 與法律頁合併之後做）：
      - 三端的 `brand.*` 色票（舊 Brand Book 的 15 色與指向舊 Figma 的註解）換成 Hawkr 色票並改名：`#E4572E` 原色只給 logo、大標與裝飾；互動元件（按鈕、連結、選取態）用加深一階的 `#B8441F`（白字 5.41:1）；輔色 `#F2A541` 配深色字 `#1F2933`（7.19:1）。
      - 舊名 token 的使用處改指向新的語意 token；季節裝飾色收斂到 Hawkr 色的明暗階。中性色與狀態色（成功、警告、危險）是通用色，保留。
      - 契約 `website/design-system/cross-platform-tokens.json` 與三端來源（website `styles/foundation/*.css`、兩支 App 的 `app_colors.dart`、`hk_tokens.dart`）同一次改；`pnpm design:check --apps-root ..` 驗三端一致。
      - 每組前景色與背景色的 WCAG 對比，以腳本對新色票計算，一般文字不低於 4.5:1；不合格的組合列出並停下。
  - 允許殘留（本步驟結束時 `scan.py --category brand` 只能剩這些）：
    - `<舊名>` 英文：A02 保護的舊 GitHub org 連結（51 個檔，步驟 6 依 D3 改寫）、`scripts/check-legacy-endpoints.py` 規則本身的字面值（步驟 7 依 D7 改造）。兩者都在 `rename.py` 的保護區間內。
    - `<舊名-子品牌>` 英文的程式識別碼、路徑與素材檔名（uncertain，不計入 exit code；步驟 5 處理）。
  - 消費端：`legal-generated-diff.yml`、`ui-text-generated-diff.yml`；`pnpm design:check --apps-root ..`；3 份改名設計書的引用 `git grep -n -F '<改名前的檔名>'`。
  - 不能動：版面與流程，只換文字與圖（法律頁依 D10 只換內文）。用語依術語表。
  - 驗收：
    - `scan.py --category brand --report` 計入的列（類別欄是 `brand`）只有代號 `<舊名>`；`rename.py --check` exit 0（英文命中都在保護區間內）；`scan.py --category brand-path` exit 0。
    - 英文 l10n 不含 `<舊名-子品牌>`：`git grep -i -l -E '<子品牌 pattern>'` 對 `website/messages`、`packages/hawkr-ui-text-ts/src/messages`、`backend/internal/platform/uitext/catalog`、兩支 App 的 `lib/core/l10n` 是 0。
    - 生成檢查（legal、uitext）與 design token 檢查通過；website、admin 的 unit 測試與 build；兩支 App 的 analyze、custom_lint、完整測試、`scripts/deadcode.sh`；backend 改到的 Go module 的 test 與 Java 改到的服務的測試。
    - mutation：在一個 l10n 檔放回 `<舊名>` 中文、在另一個檔放一個中文字首的衍生名，`scan.py --category brand` 都要 exit 1。
- [x] **5. 素材替換**（被擋於：D2、D12、步驟 1）
  - 開工量測（`main` `8f00f5656`，2026-10-26）：素材 1,873 個檔，`scan.py --category assets` 全部命中（盤點的路徑停在 `e5d5a8d8e`，步驟 3、4 改名或新增的 89 個檔「不在盤點」，其餘「來源不明」）。依盤點分組的現存數：G02 41、G03 34、G04 49、G05 115、G06 351、G07 9（另有改名後的 80 個）、G08 33、G09 77、G10 4、G11 573、G13 12、G99 495；以檔名比對，幾乎每個都有程式引用（會高估，同名檔合併計算）。指令：`scan.py --category assets --summary`；分組與引用數的一次性腳本不進 repo。
  - 一個 session 做不完，拆成 5a 到 5g，依序做；每個子步驟各自 commit 與驗收。共同規則：
    - 刪除依 `inventory.md` 的去向與 D12；被刪的每個檔名以 `rg -n -F '<檔名>'` 確認引用是 0（同名檔要以路徑確認），結果記在 `inventory.md` 該列。
    - 不能動：畫面版面，圖的尺寸與比例照舊；替代品放進原本的框。
    - 種子資料與 fixture 的圖片路徑跟著改。
    - 本機不跑 e2e、Playwright、storybook（全域約束）；畫面沒有人眼看過的部分標成「待 CI 視覺回歸」。
  - **5a 盤點對齊與小組素材**：
    - `inventory.md` 的素材表更新到現在的路徑：步驟 3 改名的字型、步驟 4 由 `brand/hawkr/` 產生的 App 圖示與 favicon（G02）與新增的 Hawkr 檔記成「自製（`brand/hawkr/`）」；Q-A 已決的去向寫進各列，`gen_inventory.py` 的「待 Will 確認」表把 Q-A1 到 Q-A8 移到「已決」並指向 D12。`inventory.md` 由 `gen_inventory.py` 產生：改產生器再重新產生，先以 `git log -p` 確認步驟 1 之後沒有人手改過產出（有的話先併進產生器）。`scan.py --check-inventory` 的缺漏與空格是 0。
    - G03 剩下的 34 個舊 logo、G04 標章與商標（改文字標籤，文字依術語表）、G08 影片、G10 QR 碼（換合成的、編碼 `https://example.com` 的 QR）刪除或替換；G13 Storybook 的幾何圖形保留，記成無創作性。
    - 根目錄新增 `THIRD_PARTY_NOTICES.md`，列出 repo 內散布的第三方素材與授權（本子步驟先放 jf-openhuninn 的 OFL-1.1；5b、5d 補上）。
    - 驗收：`scan.py --check-inventory` exit 0；本子步驟處理的群組在 `scan.py --category assets` 為 0；改到的 client 的 unit 測試（App 是改到的功能的測試與 analyze）。
  - **5b 字型**（Q-A5）：jf-openhuninn 衍生字型保留，重新生成並把檔名與 name table 的家族名改成 Hawkr 的名字，manifest 重算；OFL-1.1 授權文件照原樣保留（OFL 禁止以保留字型名發布修改版，改名正是它要求的）。
    - 驗收：每個 woff2／ttf 的 name table 對 denylist 命中 0（以 fontTools 讀出後交給 `scan.py --paths`）；website、admin 的 build；字型 manifest 的檢查通過。
    - 停止條件：重新生成要連網或要原始字型而本機沒有時停下回報，不手改二進位檔。
  - **5c 採購助理的吉祥物與動畫**（Q-A1、Q-A7、D11 收斂規則）：G06 的角色素材（website 345、buyer-app 6）與 G09 的 Lottie 刪除，畫面上的位置改用 `brand/hawkr/` 的圖形或佔位 SVG；吉祥物的葉子色（`website/features/procurement-assistant/` 的 leaf palette、釘住它的測試、生成 Lottie 的腳本）與素材一起刪。`brand-abbr` 剩下的 8 個檔有 6 個隨素材消失；另 2 個是 seller-app 沒有引用的 wallet SVG（G11），5c 一併刪除。
    - 替代品（2026-10-26 開工時依 D12 與 `website/DESIGN.md`「product UI icon 只透過 `IconlyIcon`」定）：角色與吉祥物的位置放既有的 Hawkr 標誌元件（`BrandLogo`、處理中的狀態用 `BrandLogoLoader`）；G09 的 UI 動畫圖示與食材插圖改成 `IconlyIcon` 靜態圖示，5d 換 registry 的圖形時一起換成 Material，不另建第二套圖示入口。G05 的 7 個 Lottie（`feed/`、`seller-features/`）與 Lottie 播放元件留到 5d。
    - 驗收：G06、G09 在 `scan.py --category assets` 為 0；`scan.py --category brand-abbr` exit 0；website 的 lint、`test:unit`、vitest、build、`test:build-output`。
  - **5d UI 圖示換 Material**（Q-A2、Q-A3、D12）：G05（Iconly）與 G99（設計稿匯出的 UI 圖示）刪除。兩支 App 改用 Flutter 內建的 `Icons`（不放檔案）；website、admin 改用 Material Symbols 的 SVG（Apache-2.0），只放用到的圖示，記進 `THIRD_PARTY_NOTICES.md`。沒有對應 Material 圖示的品類插圖（例如 `fruits/`、`leaves/`）歸 5e 的佔位圖。另外兩項從 5a 轉來：
      - website 首頁的「有機」「產銷履歷」兩個 chip：5a 刪掉標章圖後只剩空的灰圓（`HomeCompactCategoryNav.tsx` 的 `CompactCategoryChip` 落到沒有 `badge` 的分支），補上 Material Symbols 圖示（例如 `eco`、`verified`）。
      - buyer-app 的 `retry_payment_sheet.dart` 在執行時從外部網址載入 Apple Pay 與 Mastercard 的 logo：不在 repo 裡，素材掃描抓不到，性質同 Q-A4 的第三方商標，比照改成文字或 Material 圖示，並拿掉外部網址。依 client 分成 5d1 buyer-app、5d2 seller-app、5d3 website 與 admin，各自 commit。
      - 5c 轉來（5d3）：`IconlyIcon` 裡暫時借用不貼切圖形的名稱換成對應的 Material 圖示：`refresh`（暫用 Swap，看起來像排序）與 `FoodCategoryIcon` 的九個品類（react-iconly 沒有食材圖形，暫用葉子、愛心等）；G05 的 7 個 Lottie 刪除後，Lottie 播放元件（`LottieIcon`、`useThemedLottie`、`AnimatedIcon`、`PostActionLottieIcon`、`lib/animation/lottie-csp.ts`）、`lottie-react` 依賴，以及 `website/AGENTS.md`、`DESIGN.md` 提到 Lottie 的規則一起刪。
    - 圖示入口與依賴（2026-10-26 開工時由主 session 依 D12「三端圖形一致、授權只有一份（Apache-2.0）」與 5c 的「5d 換 registry 的圖形」定）：除了 G05、G99 的檔案，App 的 `iconly` 字型套件與 website 的 `react-iconly` 也換成 Material，兩個依賴刪除。只換檔案的話，每端會變成 Iconly 字型與 Material 混用，與 D12 的終態不符；兩個套件本身是 MIT，但 Iconly 圖形的上游授權沒有確認（推測，未查證）。做法：
      - 每端維持一個圖示入口，名稱不再帶 Iconly：App 的 `IconlyTokens` 改名 `AppIcons`、值改成 `Icons.*`（統一 outlined 風格，導覽選取態用對應的 filled），`IconlyColorTokens` 改名 `AppIconColors`；Lottie 的 `IconlyNavIcon`／`IconlyNavAsset`、SVG 的 `IconlyAssetIcon` 與 seller 的 `IconlySemanticAssets` 刪除，語意名稱併入 `HkIconName`（`HkIcon` 一律畫 `Icon`），刪除後剩下的非 UI 圖示 SVG 直接用 `SvgPicture.asset`。App 的 Iconly Lottie 刪完後 `lottie` 依賴一起刪；`buyer-app/assets/Emptybox.json`（沒有引用的 Lottie，`scan.py` 的 Lottie 規則只看 `assets/lottie/` 路徑而沒掃到）一起刪。
      - website 的 `IconlyIcon` 改名 `AppIcon`，改由 Material Symbols SVG 繪製；admin 的 UI 圖示用同一套來源，原本以字串路徑引用 `/icons/*.svg` 的地方改成元件引用。SVG 放在哪裡（兩個 app 各放用到的，或共用的 TS 套件）在 5d3 開工時定。
      - 原本的 Lottie 動畫狀態（導覽選取、空狀態）改成靜態圖示。導覽的選取態用 filled、未選取用 outlined（Material 3 導覽列的做法），加上既有的顏色與文字標籤，不只靠顏色（WCAG 1.4.1）。
      - 兩支 App 各留一份 `AppIcons` 與 `HkIconName`，不抽共用 package：兩邊的語意名稱集合不同（buyer 與 seller 的功能不同），三端共用的是 Material 的圖示名稱；design token 也是各端各一份、由 `design:check` 比對。重新評估的條件：兩支 App 出現同一語意要用同一圖形、卻各自改出不同結果時。
      - 圖形一致的範圍：同一個 Material 家族、同一份授權（Apache-2.0）。Flutter 的 `Icons` 是 Material Icons 字型，網頁用的是 Material Symbols（較新、有 fill 與 weight 軸），同名圖示的細節可能不同；D12 指定 App 用內建 `Icons`，接受這個差異。
    - 收尾設計審查（2026-10-26）記下的取捨，都不改：
      - `@hawkr/icons` 的 `MaterialSymbol` 以名稱查一份共用的 path 表，import 它的 client 會帶整份表（5d 結束時 189 個名稱，原始碼 127 KB、gzip 約 32 KB；website 的 build 裡它在一個共用 chunk）。換來的是 `name` 可以是有型別的動態值（admin 的 `icon: MaterialSymbolName` 設定陣列）；`react-iconly` 原本也是以名稱取整套圖形。重新評估的條件：`test:build-output` 或之後的效能預算顯示這份表成為首屏 JS 的主要來源時，改成每個圖示一個 named export。
      - website 的 `AppIcon` 保留語意名稱層（與 App 的 `HkIconName` 同一種做法；`data-icon-name` 是測試與 e2e 的選擇器），不同語意可以對到同一個 Material 圖形（例如 `download` 與 `paper-download`）；admin 直接用 Material 名稱，因為它沒有既有的語意名稱、圖示用法也少。
      - 兩支 App 的 `HkIcon` widget 與 `AppIconColors` 的實作目前相同，各留一份的理由與重新評估條件同上一段，條件也涵蓋 widget 實作本身改出差異時。
      - 三端同一語意用同一圖形沒有機械守門（App 的 `Icons.*` 與網頁的 Material Symbols 名稱沒有對照表）：D12 已接受兩個字型的細節差異；出現同一語意在三端是不同圖形的實例時，再加對照表與檢查。
    - 驗收：G05、G99 在 `scan.py --category assets` 為 0，`fruits/`、`leaves/` 的 88 個品類插圖除外（歸 5e）；`iconly`、`lottie`（App）、`react-iconly`、`lottie-react` 不在任何 `pubspec.yaml`／`package.json`；改到的 client 的程式、測試與指令檔（`AGENTS.md`、`DESIGN.md`）對 `rg -i 'iconly|lottie'` 為 0（守住這兩個名稱的守門測試本身除外）；admin 引用的圖示路徑都指向存在的檔案（改成元件引用後由型別檢查守）；`THIRD_PARTY_NOTICES.md` 列出 Material Symbols（Apache-2.0）與 Flutter 隨附的 Material Icons；每支 App 的 analyze、custom_lint、改到的功能的測試、`scripts/deadcode.sh`（`pubspec.yaml` 的 assets 宣告跟著刪）；website、admin 的 lint、改到的部分的 unit、build。
  - **5d4 admin 的 Remix Icon 換 Material Symbols**（D12；5d3 審查發現，2026-10-26 由主 session 依 D12「website、admin 用 Material Symbols」排入）：admin 的 `layout.tsx` 從 jsDelivr CDN 載入 Remix Icon 4.1.0 的字型與 CSS，110 個檔、約 500 處以 `ri-*` class 使用（`rg -c '\bri-[a-z0-9-]+' admin/src`）。它不是 repo 裡的檔，素材掃描抓不到；留著的話 admin 有兩套圖示與兩份授權，還在執行時依賴外部 CDN。全部改成 `@hawkr/icons` 的 `MaterialSymbol`（`StatCard` 的 `icon` 字串 API 改成 `MaterialSymbolName`），刪掉 CDN 的 `<link>` 與 `next.config.ts` CSP 裡只為它放行的 `cdn.jsdelivr.net`，`admin/AGENTS.md` 的目錄註解跟著改。在 5d3 合併之後做。
    - 驗收：`rg '\bri-[a-z0-9-]+' admin/src` 為 0；admin 不含 `remixicon`、`cdn.jsdelivr.net`；admin 的 lint、test、build。
  - **5e 照片與插畫換佔位圖**（Q-A3、D2）：G11 刪除。依 client 分成 5e1（buyer-app、seller-app 各一支 branch）與 5e2（website），各自 commit。
    - 開工分類（2026-10-27，主 session）：結果寫在 `inventory.md` G11 各列去向的開頭，規則在 `gen_inventory.py` 的 `G11_CLASSES`。引用以路徑的最後兩段比對同一個 client 目錄的文字檔；有程式引用的 App 素材沒有分類、或分了類卻沒有引用，產生器都會停下。動態組出的路徑在開工時查過兩層，都沒有：一是 `gen_inventory.py` 註解裡的兩條 `rg`；二是把每個「無引用」檔所在的目錄路徑（例如 `assets/order_card`、`/images/seller-onboarding`）拿去搜同一個 client 的程式，命中的都是完整路徑引用的「有引用」檔、守門測試或文件，沒有以目錄組路徑的程式。G11 的 652 個檔：
      - 無引用 397（buyer 137、seller 242、website 17、backend 1）：直接刪除。只在 `pubspec.yaml` 宣告的，宣告一起刪；只有測試以字串提到的，測試一起改。
      - UI 圖示 16（buyer 1、seller 15）：比照 5d 換 Material（`AppIcons`／`HkIcon`）。
      - 介面元件圖 20（buyer 2、seller 18）：含文字的按鈕、分頁、色票與底色圖，改成程式繪製的元件；文字、顏色與圖示走 design token，不放圖。改成元件時用設計系統元件自己的尺寸：步驟 5 的「替代品放進原本的框」指版面上的位置（周圍排版不因替換而移動），不是照原圖的像素尺寸（2026-10-27 主 session 在 5e1 修正輪 2 裁決：照原圖尺寸得在共用元件加特例參數，標章文字也會小於最小字級）。
      - 照片與插畫 219（buyer 50、seller 55、website 114）：換佔位圖，形式見下一段。
      - 認證標章與第三方商標 9 個改歸 G04（D12 Q-A4）：`seller-app/assets/product_setup3/upload_tag1`–`6` 改文字標籤，另 3 個沒有引用，直接刪除。
    - 佔位圖的形式（2026-10-27 主 session 定，未經 Will 個別確認）：
      - App 用程式繪製，不放檔案。每支 App 新增一個佔位元件：單色底加 Material 圖示，底色與圖示色取 Hawkr 色票的 token。照片、封面、預設頭像與插畫的位置都用它；沒有框的裝飾圖（`fruits/`、`leaves/` 的掉落與下拉刷新動畫）直接畫 Material 圖示。理由：Flutter 的 `Image.asset`／`AssetImage` 不能畫 SVG，每個使用處本來就要改；畫在元件裡就不需要產生器與資源宣告，顏色跟著主題 token，也和 5d「App 用內建 `Icons`、不放檔案」一致。它是每支 App 唯一的佔位元件：預設的網路圖片載入失敗畫面（buyer 的 `AppCachedImage`、seller 的 `OptimizedNetworkImage`）也改用它。兩支 App 各留一份：顏色 token 各端一份（`AppColors`，由 `design:check` 比對），repo 也沒有共用的 Flutter UI package，為一個元件新建 package 還要接 CI；重新評估的條件同 5d 的 `HkIcon`，另加「出現第二個兩支 App 都要的共用元件」。實作後兩份的用途集合與配色依各 App 的位置而不同（buyer 分買家與店家頭像；seller 的店家封面疊在淺色頂欄下，用深底），介面與版面算法相同。
      - website 用產生的 SVG 檔：`website/scripts/gen-placeholders.ts`（`pnpm gen:placeholders`）輸出到 `website/public/placeholders/`，用法寫在腳本的 doc comment。圖示的 path 取自 `@hawkr/icons`（repo 內 Material Symbols 的唯一來源），顏色讀 `website/design-system/cross-platform-tokens.json` 的 `brand.*`；同用途同尺寸的位置共用一張；輸出只由腳本內的表決定（不含時間戳、順序固定）。`website/tests/` 加一支測試，在記憶體重新產生並與檔案逐字比對。CSS 背景、`next/image` 與資料檔（例如 `seller-guide-manifest.json` 各頁的 `asset`）都改指向新檔。網頁需要的是可以直接替換的檔案（CSS 背景與資料檔只認路徑），所以 website 不採 App 的元件做法。`next/image` 走自訂 loader（`ossImageLoader` → `buildImageUrl`），以 `/` 開頭的本機路徑原樣放行（`lib/media/cdn.ts` 的 `isLocalPublicAsset`），`/placeholders/*.svg` 不必改 loader；`lib/media/public-assets.ts` 的 `placeholders` 改指向新檔。
      - `seller-guide-manifest.json` 是舊設計稿（Figma）的匯出，repo 內沒有產生它的腳本；它的 `objectKey`、`sha256`、`bytes`、`source` 描述的是已刪除的手冊頁圖與舊公司 R2 上的物件。5e2 把它改成手寫的頁面資料（頁序、slug、標題、`asset`），移出 `generated/`；`resolveSellerGuideBookPageAsset` 走媒體閘道的分支一起刪，佔位圖是本機的小檔，不需要 CDN。測試改守「每頁的 `asset` 存在於 `public/`」。手冊與導覽換成佔位圖後沒有資訊，D13 決定改成文字版（Will 2026-10-27，見 5g）；5e2 的頁面資料與佔位圖在 5g 刪除。
    - 5d 的遺漏（2026-10-27 開工時發現，5e1 一起修）：seller-app 的 `test/shared/widgets/accessibility_control_inventory_test.dart` 失敗。這支測試以字串掃 `notifications_screen.dart` 與 `customer_detail_screen.dart`，5d2 刪掉了它要找的註解與 `icon_copy_20.svg`。5d 只跑了 import 到改動檔的測試，以 `File(...)` 讀原始碼的測試沒有跑到。所以 5e1 起，兩支 App 每輪都跑全部讀原始碼的測試（`rg -l "readAsStringSync|listSync" test`）。
    - 既有缺陷（引用了不存在的素材，畫面上是破圖；與 G11 同性質，在 5e 一起修）：
      - 5e1 seller：`product_setup_spec_screen.dart` 引用不存在的 `assets/product_setup1/card1_c*.png`、`card2_c*.png`（公司時期起就是壞的）。物流商 logo 改文字標籤（D12 Q-A4），規格卡改成程式繪製的文字元件。
      - 5e2：`website/lib/seller/ai-image.ts` 的 AI 風格範例縮圖指向 `public/images/ai-styles/*.webp` 的 11 個檔，這些檔從沒進過版控（`git log --diff-filter=D` 為空），改指向佔位圖。
    - 驗收：
      - G11、G04 在 `scan.py --category assets` 為 0；`scan.py --check-inventory` exit 0；`deleted-assets.tsv` 追加刪除的檔，刪除後的路徑引用為 0。
      - website 的 `pnpm gen:placeholders` 重跑後 `git status` 乾淨。
      - 每支 App：analyze、custom_lint、改到的功能的測試、全部讀原始碼的測試、`scripts/deadcode.sh`。
      - website：lint、`test:unit`、改到的部分的 vitest、build、`test:build-output`（build 的輸出裡引用到的本機素材都存在，涵蓋被判成無引用的 17 個檔）。
      - mutation：App 的佔位元件改回畫 `Image.asset`，測試要失敗；改一個產生出的 SVG 的一個字元，drift 測試要失敗。
    - 審查紀錄（2026-10-27）：
      - 開工設計審查 5 項：改 3（`seller-guide-manifest.json` 與閘道分支、佔位元件是唯一的並接預設的載入失敗畫面、驗收加 build 的素材檢查），記 1（兩支 App 各一份佔位元件），提 1（D13）。
      - 程式碼審查每支 branch 1 輪：buyer MED 1（佔位元件在無界父層撐成無限大）、LOW 1；seller MED 1（駁回，見下）、LOW 3；website 0。主 session 另抓 2 項：5d1 的 padding 回歸（`design_policy_test`）、測試裡的已刪清單讓刪除後的引用永遠不是 0。修正輪 1 全部修完。seller 的 MED 是「契約寫 `AppCachedImage`，改的卻是 `OptimizedNetworkImage`」：seller-app 的預設載入失敗畫面就是後者，駁回並改計畫文字。
      - seller 修正輪 2：修正輪 1 要求介面元件照原圖外框，得在共用元件加特例參數，標章也放不進最小字級，主 session 撤回這條（裁決見上方「介面元件圖」）。
      - 收尾設計審查 3 項：記 1（兩份佔位元件已分岔，理由與條件見「佔位圖的形式」）；駁回 2：手冊頁的 `id` 仍是 `seller-guide-index.ts` 章節與熱點的鍵，不能只留 slug；`lib/media/cdn.ts` 的媒體閘道服務的是使用者上傳圖片的 backend objectKey，不是被刪素材的機制，不需要退場條件。
  - **5f 子品牌識別碼改名與 gate**（D9）：`<舊名-子品牌>` 英文的程式識別碼、路由與檔名依 D9 改成新名（`rename.py` 加規則，可重跑），denylist 該列拿掉 `#uncertain`，`scan.py --category brand` 的允許殘留只剩步驟 4 寫的 `<舊名>` 兩項。
    - 驗收：
      - `scan.py --category assets` exit 0（每個素材都是自製或有授權來源）；`scan.py --category brand --include-uncertain` 計入的列只有 `<舊名>`；`rename.py --check` exit 0、再跑一次無 diff；`scan.py --category brand-abbr` exit 0。
      - 兩支 App 改到的功能的測試、website 與 admin 改到的部分的 unit 測試與 build 通過（完整測試依全域約束延到步驟 8）。
      - mutation：加一個不在盤點裡的 png，`scan.py --category assets` 要 exit 1；在一個檔放回 `<舊名-子品牌>` 英文，`scan.py --category brand` 要 exit 1。
  - **5g 賣家手冊改成文字版**（D13 選 C，Will 2026-10-27；在 5f 之後做）
    - 現況（2026-10-27 主 session 量測）：`/seller-guide`（`website/features/content/seller-guide/SellerGuidePage.tsx`）上方是互動翻頁書 `SellerGuideBook`（2,028 行，加上 `SellerGuideBookNavigation`、`SellerGuidePageCurl`、`lib/content/seller-guide-book.ts`、`seller-guide-index.ts` 的章節與熱點、`seller-guide-bookmarks.ts`、`seller-guide-pages.json`），55 頁都是同一張佔位圖；下方折疊的「文字操作版」由 `lib/content/seller-guide-data.ts` 渲染（8 章，每節有摘要、步驟、注意事項與一張 `guide-screen` 佔位圖）。`seller-guide-data.ts` 的檔頭說唯一來源是 `docs/seller-manual.md`，這份文件不存在，它自己就是唯一來源。文字由公司時期的他人與 Will 寫成，屬於老闆同意範圍外的文案（全域約束），所以要重寫，不是只換版面。
    - 做法（預設，可推翻）：
      - 文字版成為頁面本體，不再折疊；翻頁書、它的導覽、翻頁動畫、章節熱點、書籤、`seller-guide-pages.json`，以及只服務它們的測試與 e2e 刪除。
      - 每節的 `poster` 拿掉，不放佔位圖；`guide-page.svg`、`guide-screen.svg` 從產生器的表移除。
      - 內容依現在的賣家中心重寫：以 `website/features/seller-center/**` 的路由、畫面標題與按鈕文字為準，步驟只寫畫面上真的有的操作；用語依 `backend/docs/開發規範/使用者面用語術語表.md`；涉及金流、物流、電子發票供應商的段落寫成通用說法（本階段不換供應商，換供應商的階段再補）。寫作由實作 agent 起草，主 session 逐節對照路由與畫面核對。
      - `seller-guide-data.ts` 維持唯一來源，檔頭改成現況。
      - 一般電商範圍裁減（ADR `docs/adr/2026-10-27-general-ecommerce-scope-cut.md` D1–D5，計畫 `docs/plans/2026-10-27-ecommerce-scope-cut.md`）要刪的功能不寫進手冊，即使畫面現在還在：行情雷達、價格地圖與作物品項、食譜、食品業者登錄、FDA 轉報、Uber 配送、配送溫層、產銷履歷與商品認證、採購報價的行情比較。這是已決的刪除，不是停止條件；寫成手冊再刪會讓裁減計畫多一個消費端。
    - 停止條件：某節描述的功能（不屬於上面的裁減清單）在現在的賣家中心找不到對應畫面時，列出來交 Will 決定刪節或改寫，不憑舊文字推測。
    - 驗收：`/seller-guide` 不再引用翻頁書與佔位圖（`rg -n "SellerGuideBook|seller-guide-pages|guide-page|guide-screen" website` 只剩刪除的守門）；每節的 `path` 都對得到 `website/app` 下存在的路由（加一支測試）；`scan.py --category brand,colleague,merchant,pii` 對改到的檔與 base 相同或更少；website 的 lint、`test:unit`、改到的部分的 vitest、build、`test:build-output`，`gen:placeholders` 重跑無 diff。

- [x] **6. 文件、測試資料、個資與指令檔**（被擋於：D1、步驟 1）
  - 範圍：
    - 依 D1 保留所有文件，清洗其中的品牌、人名與供應商的特店資訊；不移除文件，引用不必改寫。
    - 疑似真實的業者、人名、電話、email 換成合成資料：`example.com`、虛構業者名、不會被分配的號碼。
    - 從註解、測試與文件中移除同事的姓名與帳號。
    - 移除供應商文件裡的特店代號與測試帳號資訊。
    - gitleaks 的 72 筆依 `inventory.md` 第 5 節逐筆處理。S2（PayUni 代理商 key 形狀的共用值，與 7 個檔共用的 hex 值）視為舊公司的真憑證，換成虛構值（Will 2026-10-24）。
    - issue 或 PR 連結改成新 repo 的 issue（D3）或拿掉。
    - `git show <sha>` 指引改成指向檔案。
    - 計畫與 ADR 的 commit hash 保留，視為歷史參照，在 `backend/docs/plans/README` 加一句「hash 指私人封存」。
    - `.claude/`、`.agents/` 的個人路徑與私人參照移除。
    - `backend/docs/詳細設計書/58A-電子發票前端實作規格.md`（第 144、428、1137、2512 行一帶）與 `58-電子發票服務（Invoice Service）詳細設計書.md`（第 12265 行一帶）仍寫著 5d 之前的圖示慣例（`IconlyIcon`、Lottie、admin 的 Remix Icon），改成現況（`AppIcon`、`@hawkr/icons` 的 `MaterialSymbol`）或標明是當時的實作紀錄（5d 收尾時發現）。
  - 消費端：`scan.py --category brand,colleague,pii` 涵蓋 `backend/docs/`、`website/docs/` 與各子專案的文件。
  - 不能動：測試的意圖。換了資料，測試仍斷言相同的行為。
  - 驗收：
    - `scan.py --category colleague,pii` 是 0；`scan.py --category endpoint --include-uncertain` 也是 0（法人、合作方與測試資料 email 網域，步驟 2 留下的部分；法律頁的法人名稱由步驟 4 清）。
    - gitleaks 掃追蹤中的檔是 0。雜湊造成的誤報逐筆列在 `.gitleaksignore`，附理由。
    - 改到的測試套件通過。
    - mutation：放回一個同事帳號，`scan.py` 要失敗。
  - 拆分（2026-10-27，規模：colleague 36 檔＋uncertain 43、pii 115 檔、endpoint uncertain 56 檔、merchant 35、private 114、refs 784 檔）：
    - [x] **6a 同事、個資與合作方**：colleague（含 uncertain）、pii、endpoint uncertain 換成合成資料或移除，含會被裁減計畫刪除的 12 個檔。驗收：上面第一條的兩個 scan 加 `--category colleague --include-uncertain` 為 0；改到的測試套件通過；mutation：放回一個同事帳號，scan 失敗。
    - [x] **6b 憑證**：gitleaks 72 筆與 S2。驗收：上面第二條；改到的測試套件通過。
    - [x] **6c 特店、私人參照與文件慣例**：merchant、private、refs 的 issue／PR 連結與 `git show` 指引、`plans/README` 的 hash 說明、電子發票設計書的圖示慣例。驗收：`scan.py --category merchant,private` 為 0；refs 只剩計畫與 ADR 的 commit hash。
- [x] **7. 守門與 e2e 的 CI**（被擋於：D5、D7、步驟 2 到 6）
  - 範圍：
    - 依 D7 改造舊端點檢查與 denylist 檢查的執行位置。步驟 2 清空 baseline 之後，「命中只能減少」的 baseline、`--update` 與只限單體路徑的 `ZERO_TOLERANCE` 已經沒有必要條件（既有命中是 0），改造時一併拿掉，改成全樹零容忍。舊公司 GitHub org 的引用（`github.com` 與 `org/repo` 兩種寫法）目前沒有規則：步驟 2 只清掉會在執行時連過去的一處（監控的 git-sync），其餘是 issue 連結，步驟 6 依 D3 改寫後再加這條規則。縮寫的偵測還有一個缺口：縮寫後接任意小寫單字、或出現在 camelCase 結尾時，`scan.py` 的縮寫類抓不到（步驟 3 第二輪審查；當時樹上這類殘留是 0），改造時補上並以 mutation 驗證。
    - `.pre-commit-config.yaml` 加上 `pre-commit` 與 `pre-push` 的 hook type。既有 clone 要重跑 `pre-commit install`，寫進根目錄 `AGENTS.md`。
    - 每支 workflow 加 `workflow_dispatch`，步驟 8 要能逐支手動觸發。
    - 依 D5 新增 e2e 的 workflow。
  - 消費端：`legacy-endpoints.yml`、`repo-hygiene.yml`、`.pre-commit-config.yaml`、根目錄 `AGENTS.md` 的 clone 指引。
  - 不能動：守門的語意是「擋新增的引用」。
  - 驗收：
    - 兩支檢查在步驟 2 到 6 完成後的樹上通過。
    - mutation：任一檔加入一個舊網域，再加入 `<舊名>` 中文，兩支檢查都要失敗。
    - 全樹的耗時記進進度表。
  - 拆分（2026-10-27，兩邊檔案不重疊，可平行）：
    - [x] **7a 守門**：`scripts/check-legacy-endpoints.py` 改成全樹零容忍（拿掉 baseline、`--update`、`ZERO_TOLERANCE`），加舊 GitHub org 規則；名字類掃描留在計畫附件（暫時的，步驟 9 移除），補縮寫缺口；`.pre-commit-config.yaml` 加 pre-commit／pre-push 的名字檢查（呼叫附件的腳本）；CI 不新增檢查（D7 修訂）；刪除 `legacy-endpoints.yml`（舊端點檢查改由本機 pre-push hook 執行），`repo-hygiene.yml` 加 `workflow_dispatch`；根目錄 `AGENTS.md` 的 clone 指引。驗收：上面的前兩條與耗時，加上縮寫缺口的 mutation。
    - [x] **7b workflow**：其餘每支 workflow 加 `workflow_dispatch`；依 D5 新增 website 與 admin 的 Playwright workflow（含視覺基準由 CI 產生的流程）。驗收：`actionlint` 通過；e2e 的實跑在步驟 8 新 repo 的第一輪 CI（本機不跑）。
- [ ] **8. 切換**（被擋於：D3、步驟 1 到 7；需要 Will 授權）
  - 範圍：
    1. 清空其他 worktree 與分支（改名分支已在步驟 3 完成後合併進本機 main）。
    2. 在最新的樹上重跑 `scan.py --all` 與 gitleaks，兩者都是 0。
    3. 用 `git archive` 匯出到新目錄 `.projects/personal/hawkr`，`git init`，建立單一初始 commit。
    4. 建 `Will413028/hawkr`，先設成 private、關閉 Actions，push。
    5. Will 在 GitHub 上檢視。
    6. 轉成 public（Will 授權）。
    7. 開啟 Actions，把 fork PR 設成「Require approval for all outside collaborators」。
    8. 開啟 secret scanning 與 push protection。
    9. 在 main 加 ruleset，只禁止 force push 與刪除，不要求走 PR。後續修正以一般 commit 直接推。
    10. 用 `gh workflow run` 逐支觸發，`_flutter-quality` 除外，它由兩支 App 的 workflow 呼叫。
    11. 舊 repo 改名成 `<舊名>-archive` 並封存（Will 授權）。
    12. 依 D3 處理 issue。
    13. 更新本機個人脈絡檔、second-brain 專案頁與 runbook 的現況檔；daily 是歷史紀錄，不改。
  - 消費端：second-brain 指向舊路徑的現況檔（`rg -l '.projects/personal/<舊名>' wiki .agents scripts`）；memory 目錄的對應。
  - 不能動：舊 repo 的歷史，只封存，不改寫。
  - 驗收：
    - 新 repo 的 `git rev-list --count HEAD` 在初始 push 時是 1。
    - 用 GitHub API 查到 repo 是 public、fork 核准與 push protection 已開啟、ruleset 存在。
    - 第一輪 CI 的每個 job 綠燈。修正以後續 commit 推上，不重寫初始 commit。
  - 停止條件：轉 public 前，scan 或 gitleaks 任何一類大於 0。
  - 需要人做的事：授權轉 public 與封存舊 repo；在 GitHub 上檢視一次新 repo。
- [ ] **9. 收尾**：
  - 完成定義逐項核對。
  - 做獨立的設計審查（design-review skill）。
  - 移除本機的名字檢查 hook 與計畫附件的掃描腳本（D7 修訂：名字檢查只在切換前後暫用），以及舊端點檢查 `scripts/check-legacy-endpoints.py` 與它的 hook、引用（根目錄 `AGENTS.md` 的地圖列與「不連到舊公司」的檢查說明、`backend/AGENTS.md` 規則 7、`ci-local-map.md`）（Will 2026-10-27）。
  - 延後項附上觸發條件，搬回主清單。
  - A7 的進度表改指向新 repo 的路徑，並依 D5 改寫 `AGENTS.local.md` 的 e2e 那一條。
  - 本計畫的 `<舊名>` 代號保留；計畫檔隨新 repo 公開。
  - 驗收：新 repo 的 CI 結論、`scan.py --all` 的輸出、design-review 的分流結果，都貼在進度表。

## 決定

- **D1 公司時期的文件要不要公開**（狀態：已決 2026-10-24，Will 選 A；擋住步驟：6）
  - A：公開，但清洗品牌、人名與供應商資訊。文件完整，但超出老闆同意的「程式碼」範圍，而且設計書的作者以同事為主。
  - B（建議）：只公開 Will 與 agent 在 2026-09 之後寫的文件，也就是 ADR、計畫、`AGENTS.md`、`CARRYOVER.md`。公司時期由他人撰寫的設計書、規範與供應商文件，只留在私人封存。
    - 這和「不使用未授權素材」一致。
    - 代價：之後 A7 到 B 的步驟要引用設計書時，得讀本機的封存 checkout。
  - C：連 ADR 與計畫也不公開。作品集會失去最有價值的設計紀錄，不建議。
  - 結論：A。設計書、規範與供應商文件都公開，清洗後保留（Will 2026-10-24）。
- **D2 來源不明的圖片換成什麼**（狀態：已決 2026-10-24，Will 照建議；擋住步驟：5）
  - A（建議）：品牌圖用 `brand/hawkr/`。商品與店家的示範圖改用程式產生的佔位 SVG，例如單色底加品類圖示，不放照片。授權零風險、檔案小，代價是畫面樸素。
  - B：用 Unsplash 的照片。畫面好看，但要逐張記來源。Unsplash 授權禁止把照片彙整成類似或競爭的服務，放進 repo 散布是否越界，要逐張判斷。
  - C：AI 生成圖。授權狀態依工具條款，看作品集的人可能有疑慮。
- **D3 搬進舊 repo 的 274 個 issue 怎麼處理**（狀態：已決 2026-10-24，Will 照建議；擋住步驟：6、8）
  - A（建議）：在新 repo 重建。用 `.local/publish/migrate.py` 加上 `<舊名>` 與舊路徑的替換，建好後再掃一次。issue 不會漏，作品集也看得到待辦的管理。GitHub 不能把 issue 從 private repo 移轉到 public repo，所以只能重建。
  - B：留在私人封存，新 repo 從空的開始，待辦靠計畫檔追蹤。
- **D4 沒有自己網域時，反向 DNS 用什麼前綴**（bundle ID、Android applicationId、Java package；狀態：已決 2026-10-24，Will 照建議；擋住步驟：3）
  - A（建議）：`io.github.will413028.hawkr`。這是 GitHub 帳號的命名空間，Maven Central 也承認；它屬於 Will，不會撞名。缺點是長。
  - B：`dev.hawkr`。比較短，但 Will 沒有這個網域，之後被別人註冊就等於用了別人網域的 ID。
- **D5 e2e 與瀏覽器測試在哪裡跑**（狀態：已決 2026-10-24，Will 照建議；擋住步驟：7、9）
  - 現況：
    - 本機不跑（Will 2026-10-24）。
    - CI 也沒有 workflow 在跑：website-ci 只跑 `test:unit` 與 `test:build-output`，截圖基準檔也不在版控。
    - 所以目前 e2e 在哪裡都沒有執行。
  - A（建議）：在本階段新增 website 與 admin 的 Playwright workflow。public repo 的 Actions 免費，額度不再是理由。
    - 視覺回歸的基準截圖第一次在 CI 產生，以 artifact 下載後 commit。
    - 之後比對，Linux runner 與 macOS 的字型差異就不會發生。
    - A7-2v（backend 驗收搬進 CI）照原計畫做。
  - B：e2e 列進延後，觸發條件寫「新 repo 上線後」。現況就繼續是「沒有 e2e」。
- **D6 設計 token 等處的兩字母縮寫要不要改**（狀態：已決 2026-10-24，Will 照建議；擋住步驟：3）
  - 範圍：數千處，例如 `<縮寫>-primary`、`<縮寫>_tokens`，三端的 design token 共用。
  - A（建議）：一起改成 `hk-`／`hk_`，由 `rename.py` 處理，`design:check` 驗三端一致。成本只有腳本的對照，留著會讓人看出舊名。
  - B：保留。兩個字母不構成品牌，少一類改動。
- **D7 denylist 與舊端點的檢查放在哪裡**（狀態：已決 2026-10-24，Will 照建議；擋住步驟：7）
  - 問題：短字串的雜湊沒有加鹽就公開，猜得到的人可以直接比對出來，同事姓名尤其不能這樣。
  - A（建議）：
    - 名字類（品牌、法人、同事）只在本機的 pre-commit 與 pre-push 檢查，讀不進版控的明文清單。
    - CI 只跑不含名字的類別（舊 IP、帳號 ID 格式）、gitleaks 與 GitHub 的 secret scanning。
    - 代價：外部 PR 不受名字檢查。目前只有 Will 一個開發者，可以接受。
  - 修訂（Will 2026-10-27）：CI 不新增檢查（gitleaks、secret scanning 不加進 CI）；本機的名字檢查也是暫時的，步驟 9 移除。既有的 `legacy-endpoints.yml` 刪除，`scripts/check-legacy-endpoints.py` 改由本機 pre-push hook 執行，步驟 9 與名字檢查一起移除。
  - B：CI 用 repository secret 當 key 做 HMAC。fork PR 拿不到 secret，這項檢查會跳過。
  - C：公開未加鹽的雜湊。不建議。

- **D8 縮寫的同族寫法與撞名的資料值**（狀態：已決 2026-10-24，Will 照建議選 A，見 ADR 的修訂段；擋住步驟：3；事實來自步驟 1 的 `rename-map.md` 第 6 到 8 節）
  - 現況：D6 只寫 `<縮寫>-`／`<縮寫>_`。步驟 1 找到同一個縮寫的其他寫法：UI 元件庫與頁面前綴（325 個檔、8,310 處）、Pascal 類別前綴（431 個檔、5,686 處）、日誌類別與 camelCase 識別碼；還有大寫的資料值：代行工作階段編號與 header 格式（X2，29 個檔）、訂單與出貨單編號前綴（X3，27 個檔）、測試編號（X4）。
  - A（建議）：全部改成 `hk` 系列，資料值的格式前綴連同產生碼、驗證正則、CHECK、種子與 fixture 一起改，migration 改寫後重算 `atlas.sum`。上線前不受相容性約束（根目錄 `AGENTS.md`），D6 的理由「留著會讓人看出舊名」對同族寫法同樣成立；`scan.py --category brand-abbr` 才能當成 0 的 gate。
  - B：只改 D6 列的兩種與同族的程式識別碼，資料值（X2 到 X4）保留。改動少，但 `brand-abbr` 要為資料值開例外清單。

- **D9 舊名中文字首的子品牌與功能名**（狀態：已決 2026-10-25，Will 照建議選 A；擋住步驟：4、5）
  - 現況：denylist 只有完整的 `<舊名-中文>`。同一個中文字首另有子品牌 `<舊名-子品牌>`（AI 採購助理，原本標 uncertain）、功能名（幫找、動態、市集、彈珠）、關係企業名與 42 個檔的測試店名。
  - A（建議）：算舊名。使用者面文字把字首換成 Hawkr（Hawkr Agent、Hawkr 幫找、Hawkr 動態、Hawkr 市集），版面不動；denylist 加 `<舊名-字首-中文>` 一列並計入 gate。程式識別碼與路徑在步驟 5 刪完素材後才改，避免改名後又刪，也不讓 `inventory.md` 的代號化素材路徑對不上。
  - B：改成不掛品牌的通用功能名（採購助理、幫我找）。讀起來自然，但每處都要改寫，長度改變可能動到版面。
  - C：不算舊名，保留。衍生名仍會讓人看出舊品牌。
- **D10 法律頁**（狀態：已決 2026-10-25，Will 選 B；擋住步驟：4）
  - 現況：隱私、服務條款、退貨、使用者政策四頁是公司時期主要由他人撰寫的 TSX；賣家條款、KYC 告知、食品登錄告知、自行開立聲明四份 Markdown 是 Will 在 2026-10 寫的，也是術語表裡賣家用語的來源。
  - A（建議）：分開處理。四頁換成簡短說明；Will 寫的四份保留全文，只換營運者並加示範聲明。
  - B：八份都換成簡短的作品集說明。最乾淨；代價是術語表要改以自己為準，賣家同意流程的文件內容與生效日跟著變，設計文件引用的舊條號只對得上私人封存。
  - C：全部保留全文，只換營運者。他人寫的四頁不在老闆同意的「程式碼」範圍內。
  - 結論：B（Will 2026-10-25）。
- **D11 品牌色**（狀態：已決 2026-10-25，Will 照建議；擋住步驟：4b）
  - 現況：三端共用 43 個顏色 token，其中 15 個是舊公司 Brand Book 的品牌色，註解指向舊公司的 Figma；主按鈕是深綠 `#334c0f`。這套色票本身是舊品牌素材，不在老闆同意的「程式碼」範圍內。Hawkr 只定了 4 色（`brand/hawkr/README.md`）；白字配 `#E4572E` 對比是 3.68，不到 WCAG AA 一般文字要求的 4.5。
  - 主色：A（建議）互動元件用加深一階的 `#B8441F`（5.41），原色只給 logo 與裝飾，也就是把品牌色和可及的互動色分開；B 直接用 `#E4572E`，只有大字合格。
  - 舊色票：A（建議）換成 Hawkr 色票並改名，使用處改指向語意 token，不留舊品牌色票與 Figma 註解；B 只改語意 token 的值，舊色票留給裝飾用。
  - 結論：兩題都選 A（Will 2026-10-25）。
  - 4b 實作後的補充決定（Will 2026-10-25，都照建議）：
    - `#B8441F` 當文字放在 panel 底、淺橘底或半透明選取底上時，對比 3.87–4.48 不到 4.5，這些組合的文字改用加深的 `primaryDeep`（`#8F3315`）；色票不變。
    - 暗色模式的墨綠 surface 與頁面裡手寫的橄欖綠色值，同屬舊品牌色，換成以 `#1F2933` 為基底的中性深色階並收進 token，在 4b 一起做。
    - admin 側欄用深色 `#1F2933`，橘色只用在選取項與主按鈕。
    - 舊色票的三個藍色當成資訊狀態色，保留原值、改名 `status-info`，與成功、警告、危險三色的處理一致。
  - 收斂規則（Will 2026-10-25）：4b 經 3 輪審查，每輪都找到換色的新後果。修正輪 3 修完第 3 輪的清單，以舊色命名的 token 機械改名；Agent 吉祥物的葉子色（`<舊名-子品牌>-leaf-palette.ts` 與生成 Lottie 的腳本）歸步驟 5，與 Lottie 素材一起處理。之後的審查只看修正輪 3 的 diff 是否正確（視覺壞掉、對比不合格、狀態語意錯）。再出現的設計延伸寫進「延後」，觸發條件是新 repo 的 CI 視覺回歸建立基準之後。

- **D12 素材的待確認題 Q-A1 到 Q-A8**（狀態：已決 2026-10-26，Will 全部照盤點的預設；擋住步驟：5）
  - 吉祥物與子品牌角色素材、影片、Lottie、QR 碼（Q-A1、Q-A6、Q-A7、Q-A8）：沒有一批確定有授權，全部刪除；QR 換成合成的。
  - Iconly 與設計稿匯出的 UI 圖示（Q-A2、Q-A3）：刪除，換 Material 系。兩支 App 用 Flutter 內建 `Icons`，website、admin 用 Material Symbols 的 SVG，三端圖形一致、授權只有一份（Apache-2.0）；捨棄網頁用 Lucide（三端不一致）。照片與插畫照 D2 換佔位圖。
  - 官方認證標章與第三方商標（Q-A4）：刪除，改文字標籤。
  - jf-openhuninn 衍生字型（Q-A5）：保留，重新生成並改家族名，授權文件與 notice 照 OFL 保留。
  - 5b 開工時補的兩題（Will 2026-10-26）：admin 用的 2,698 字舊子集（家族名仍是保留名 `jf-openhuninn-2.0`）從原始字型以同一組字元重新生成並改名，admin 的載入方式不變；website 那份沒有程式引用的同內容子集刪除。原始字型與授權由主 session 從上游鎖定的 commit 下載並核對 sha256（Will 同意解除 5b 的「要連網就停」）。
- **D13 賣家導覽與操作手冊換成佔位圖後沒有資訊**（狀態：已決 2026-10-27，Will 選 C；擋住步驟：5g）
  - 現況：website 的賣家操作手冊（55 頁，每頁是一張圖）與賣家導覽（22 張操作畫面）、賣家招募頁的畫面輪播，內容都是舊品牌介面的截圖。依 D2 換成佔位圖後，版面還在，但每頁都是同一張底色加圖示，看不到內容。
  - A（建議）：保留版面與佔位圖，之後用 Hawkr 自己的畫面截圖補回。觸發條件：D5 的 Playwright workflow 在新 repo 跑起來之後，由 e2e 截圖產生（自製、可重跑）。代價是補回之前這幾頁沒有內容。
  - B：下架手冊與導覽（刪路由、元件與資料檔）。最乾淨；代價是作品集少了賣家教學這一塊，之後要補回得重做。
  - C：改成文字版的手冊。手冊頁的內容畫在圖裡（舊設計稿匯出），文字要重寫，成本最高。
  - 結論：C（Will 2026-10-27）。做法見步驟 5g。

拍板：D1 選 A，D2 到 D8 照建議（Will 2026-10-24）。D9 照建議、D10 選 B、D11 照建議（Will 2026-10-25）。D12 照預設（Will 2026-10-26）。D13 選 C（Will 2026-10-27）。推送「還沒改名」的 main 已由 Will 授權（2026-10-24）。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 盤點與基線 | 完成 | `scan.py --check-inventory`：4,087 列、去向空格 0、缺漏 0；mutation：清空一格去向 exit 1、刪一列素材 exit 1、暫存檔放舊網域被抓到 exit 1；附件對 denylist 命中 0；`ci-local-map.md` 涵蓋 26 支 workflow（diff 為空）；基線 36 列：PASS 29、FAIL 4、NOT RUN 3 | iOS lockfile 檢查（只手動跑）、coverage ratchet（CI 專屬）、e2e（D5，交 CI）；耗時是高負載下量的，只當量級 | 見 git log |
| 2 移除舊公司的外部服務設定 | 完成 | 全樹：`check-legacy-endpoints.py` 空 baseline 通過；`scan.py --category endpoint` 的 `<舊網域-1>` 到 `<舊網域-8>` 與 `legacy:*` 為 0（只剩規則檔自身）；mutation：GA4 設計書放回舊 Firebase project id，`scan.py` 與 `check-legacy-endpoints.py` 都失敗、還原後 0。`ci-local-map.md` 對照（最終 commit 或其後只改文件的 commit 上重跑）：commit-messages、legacy-endpoints、repo-hygiene、migration-inventory、gateway-json-contract、atlas validate、backend-go-checks（gofmt、vet、test、tidy）、proto-generated-diff、backend-java-tests 兩個 job、backend-integration-tests 九個模組、admin-ci、website-ci、ui-text-guard、buyer-ci 與 seller-ci 的 static、tests、hermetic、generated 全部與基線相同或通過。與基線相同的失敗：pricebook integration 兩項、buyer generated-code（`session_provider.g.dart`）。負載假失敗（單獨重跑通過）：ai-service `TestRewindUseCase_NarrationRunsInParallel`、website vitest `useProcurementSession`。獨立審查：設計審查 1 輪（7 項：改 5、記 1、提 2 由 Will 決定）、程式碼審查 4 輪，累積清單的 FIX 列都已修並各有 mutation | e2e、Playwright、storybook（D5，待 CI）；iOS 建置與實機上的 Firebase 停用路徑（本機不能建 iOS）；coverage ratchet（CI 專屬，buyer `coverage_baseline.json` 的行數是手改檔名清單後的舊值）；`promtool`／`amtool`（本機沒有映像）；`workspace-ci / deps` 沒重跑（沒改 lockfile） | `49af4af07` 到 `6894baa5f` |
| 3 識別碼改名 | 完成（2026-10-25 fast-forward 合併進本機 main） | `rename.py --check` exit 0（再跑一次無 diff）；`scan.py`：`brand-path` 0，`brand --english` 只剩 A02 保護的舊 org 連結 51 個檔（步驟 6）與 `check-legacy-endpoints.py` 的規則字面值（步驟 7），`brand-abbr` 只剩 8 個 Lottie／svg 的 base64（步驟 5）；附件與計畫檔對 denylist 明文 0。mutation：`buyer.proto` 的 package 改回舊名，`make connect` drift 2 個檔、`buf lint` 失敗、`scan.py` 抓到。`ci-local-map.md` 全部 54 列重跑兩輪（修正輪 1 之後整套重跑，修正輪 2 之後重跑 Java 兩列）：與基線相同的失敗只有 `backend-monolith / lint`（22／35 項，名字正規化後位置逐項相同）、`pnpm audit`（16 個）、pricebook integration 兩項；buyer generated-code 的既有 drift 已由重新生成修好；`buf breaking` 對改名前 PASS（`ignore_unstable_packages`，已更正 `ci-local-map.md`）；Java integration 229 項。負載假失敗：website `useProcurementSession` 逾時，同時段對改名前的 main 跑同一檔也失敗（load average 約 170–220），第一輪整套時通過。獨立審查 2 輪（累積 F1–F7、R1–R3、G1–G4 共 14 項，全部已修並各有證明或 mutation） | e2e、Playwright、storybook（D5，待 CI）；iOS lockfile（只手動跑）；coverage ratchet（CI 專屬）；woff2 內部 name table 是否含舊名（離線無法重新生成，步驟 5）；字型 manifest 的生成器要連網，改以同一個 sha256 演算法重算 | `f20f74f25` 到本列所在 commit |
| 4 品牌、文案與法律頁 | 完成 | 4b（品牌色）：主 session 在合併後的 main 重跑 `contrast.py` exit 0（402 組、判定 361、不合格 0），`design:check` 通過（81 個 token），舊 `brand.*` 色票名 grep 0，`scan.py --category brand` 計入的列只剩 `<舊名>`，`rename.py --check` exit 0。implementer 的 mutation：`design:check` 改 seller 的 `brandPrimaryStrong` 失敗；`contrast.py` 把 action.primary 指向原色、淡底文字改回 primaryStrong、輔色當文字、半透明底、成對 badge、暗色圖形、淺色 token 當文字、Dart 別名、admin、無暗色覆寫各自 exit 1；fabric 色盤與 maplibre 色值放回 `var(` 測試失敗；只掃追蹤檔（build 輸出放同樣寫法仍 exit 0）。implementer 最後一輪跑過：website lint、`test:unit`、vitest、build、`test:build-output`；admin biome、tsc、test、build；兩支 App 的 analyze、custom_lint、完整測試（buyer 1,455、seller 2,305）、deadcode、`check_design_tokens.sh`。獨立審查 4 輪（累積 H1–H4、M1–M6、L1–L9、T1–T2 全部已修；第 4 輪依收斂規則只看正確性，剩 1 個 LOW 寫進延後）。4a：`scan.py --category brand` 計入的列只剩 `<舊名>`（52 個檔，A02 的 51 個加規則檔）、`brand-path` 0、`rename.py --check` exit 0；mutation：l10n 檔放回 `<舊名-中文>`、另一檔放中文字首衍生名，`scan.py --category brand` 都 exit 1，對照組 exit 0。主 session 在合併後的 main 重跑：CodeMeta `generate.sh --all` 無 drift、`design:check` 通過、seller-app 完整測試 2,305 項通過。implementer 各自跑過：website lint、`test:unit`、vitest、build、`test:build-output`；admin lint、test、build；buyer-app 與 seller-app 的 analyze、custom_lint、完整測試、deadcode；backend 單體與 9 個舊服務 module 的 gofmt、vet、test，改到的 Go integration、Java 三個服務；legal 生成無 drift。獨立審查 1 輪（R1–R8 與 F1 共 9 項，全部已修，R1 有 mutation） | e2e、Playwright、storybook 與視覺回歸（D5，待 CI；要看 `seller-pricing-page.spec`、initial loader 與全站換色）；沒有人眼看過換色後的畫面；iOS build（本機不能建）；OG 與 apple-icon 的實際渲染沒看過輸出圖；coverage ratchet（CI 專屬）；`*IT` 的 Java integration 沒確認 surefire 有跑 | `2606123866` 到 `cc6985fa9a` |
| 5 素材替換 | 完成：5a–5g（5d 含 5d1–5d4，5e 含 5e1、5e2） | 5g（`b14f8b19c2`、`97be32beed`、修正輪 1 `eafaba5361`，2026-10-27）：主 session 在 branch 上重跑：翻頁書、書籤、索引與 `guide-page`／`guide-screen` 在 website 的引用 0；手冊的裁減用語與供應商名 0；相關的 node 測試 175 項全過；biome、`tsc --noEmit` exit 0；build（帶 loopback 的 `API_BASE_URL`）exit 0、`test:build-output` 9 項過、`gen:placeholders` 重跑無 diff；新的路由測試 mutation：path 指向不存在的路由、只有 layout 的目錄都失敗。逐節對照畫面的獨立審查 8 項（皆 LOW）：修 7（手機驗證的說法改成開店引導、物流商取件按鈕不加引號、「配送異常」只在配送中分頁、#開店FAQ、附錄改用小節編號、路由測試只認 page 檔並逐段解析），駁回 1（登出不再清的書籤 key：從未上線，沒有瀏覽器存著它）。裁減：第 9 章市場行情整章刪除（依一般電商範圍裁減 ADR），黑貓與 7-11 兩節同一個畫面，合併成「物流與退貨」。手冊沒涵蓋的畫面（只列不加）：商店預覽、發票與稅籍、措施與申覆、開店方案、停權公告。未跑：e2e、視覺回歸（頁面未經人眼，待 CI）。 <br>5f（`3b4f094f01`、`4fd9fba4d9`，2026-10-27）：主 session 在 branch 上重跑：`rename.py --check` exit 0；`scan.py --category brand` 計入的列與 base 逐列相同（52 檔，全是 `<舊名>`）；`brand-abbr`、`assets` exit 0；`git grep`／`git ls-files` 的 `<舊名-子品牌>` 為 0；website 改到的 32 個 `tests/*.test.ts`（node test runner）290 項全過；`go test ./internal/usecase/ -run Certification`（seller-service）通過；shipping-service 改到的三個檔是手動執行的 `main()` 腳本（不是 JUnit），`mvn -pl shipping-service -am test` 編譯通過；獨立審查：無缺陷。實作 agent 回報（未重跑）：website lint、build、`test:build-output`，兩個 mutation（放回 `<舊名-子品牌>` 該列計入；不在盤點的 png 使 assets exit 1）。`--include-uncertain` 另剩 `<舊名-中文別名>` 1 處：buyer-app 字級設定的示範句，判定為一般用語而保留（主 session 2026-10-27，未經 Will 個別確認；改它是 UX 文案）；同列其他命中（Java 付款頁標題、物流標籤是品牌用法，換成 Hawkr；Go 測試的店名換成中性字串）在 `4fd9fba4d9` 清掉。5f 的 `rename-map.md` 是手補 A28、A29 兩列（檔頭寫明不得以 `gen_rename_map.py` 重新生成）。 <br>5a：主 session 在合併前的 branch 上重跑：`scan.py --check-inventory` exit 0（空格 0、缺漏 0）；`scan.py --category assets` 1,696 處，依盤點分組只剩 G05、G06、G07、G09、G11、G99（G01–G04、G08、G10、G13 為 0）；`scan.py --category brand` 52 個檔、159 處，與 base 相同；刪除的 91 個素材以完整路徑與檔名比對引用都是 0（檔名命中只有子字串誤判）；buyer-app `custom_lint`、website vitest 4 個檔 45 項。implementer 跑過：website lint、`test:unit`、vitest、build、`test:build-output`；admin lint、test、build；兩支 App 的 analyze、custom_lint、deadcode；buyer、seller 的完整測試在 Will 決定延後前已跑過並通過。mutation：盤點清空一格去向 exit 1；新增未追蹤的 png，`git add -N` 後才被抓到（不在盤點=1）；F3 的圖示尺寸改 24，測試失敗。獨立審查 1 輪（F1–F7）：F2–F7 已修（F5 第一次修過頭，修正輪 2 補回文字標籤），F1 首頁空圓轉入 5d。5b：主 session 在合併前的 branch 上重跑：88 個 tracked 字型檔的 name table（name ID 13 授權全文除外）以 fontTools 讀出交給 `scan.py --all --only-paths --include-uncertain`，各類別 0（mutation：一份改回舊家族名，brand 抓到 9 處）；`generate_openhuninn.py check` 80 個分片、11,345 字元 exit 0，`check-subset`（admin 子集）exit 0；`scan.py --check-inventory` exit 0；website `font-loading-contract.test.ts` 3 項、lint、build、`fonts:check-build`（80 個 face）；admin lint、build；Roboto、Zen Maru Gothic、Noto Sans TC 的 OFL 授權檔版權段與字型 name ID 0 以程式比對相符。implementer 的 mutation：分片換回舊內容 `check` exit 1；admin 子集家族名改回保留名 `check-subset` exit 1。獨立審查 1 輪（R1–R4）：R1（name ID 7 商標聲明、19 範例文字含上游名）駁回，保留名限制只及於字型名稱欄位，生成器已檢查；R2–R4 已修；修正輪後主 session 重跑盤點檢查發現新增的授權檔沒登記（R5，exit 1），修正輪 2 修好 5c：主 session 在合併後的 branch 上重跑：`scan.py --category assets` 1,614 → 1,182（G06 353、G09 77 都是 0，另 2 個是下面的 wallet SVG）；`scan.py --category brand-abbr` exit 0（另刪 seller-app 2 個無引用的 wallet SVG）；`scan.py --check-inventory` exit 0；`deleted-assets.tsv` 追加 432 列（子品牌路徑寫代號，`gen_inventory.py` 由 git 歷史還原；mutation：放一列還原不到的代號 exit 1），刪除後路徑引用全為 0（3 個同名命中人工確認無關）；website lint、`test:unit`（2,221 pass）、build、`test:build-output`（9 pass）；vitest 3,133 pass、1 項 `useProcurementSession` 冷啟動逾時，未改的 base 同一檔同樣逾時（負載），不算 5c 造成；buyer-app analyze（implementer）。implementer 的 mutation：接回已刪模組 tsc TS2307；拿掉 sprite 的 `aria-label`／`data-motion` 測試失敗；重新連線改回 loop 測試失敗。獨立審查 1 輪：0 HIGH、3 MED、5 LOW，修正輪 1 修 7 項；`refresh` 暫用 Swap 圖形轉入 5d3。 5d：主 session 在各 branch 修正輪之後重跑：seller-app format（0 changed）、analyze、custom_lint、deadcode（352 assets）皆通過，圖示相關 35 個測試檔 +312、import 改到的 lib 檔的另 87 個測試檔 +827 全過；buyer-app format、analyze、custom_lint、deadcode（204 assets）皆通過，改到與 import 改到的 lib 檔的 111 個測試檔 +898 全過；website lint（2,090 個檔）、`test:unit`（2,215 項，pass 2,214、skip 1）、vitest `--changed`（306 檔、3,102 項）、build、`test:build-output`（9）皆通過；`@hawkr/icons` lint、test（6，含 drift）、重跑 gen 無 diff；admin lint、test（1,165）、build；website tsc。合併後的 main：`scan.py --category assets` 1,182 → 661（刪 521 個；另刪不在掃描範圍的 `Emptybox.json`），G05 為 0，G99 只剩自製的 `website/app/icon.svg`，`fruits/`、`leaves/` 88 個改歸 G11（5e）；`deleted-assets.tsv` 追加 522 列，`--check-inventory` exit 0；`brand` 52 個檔、159 處與 base 相同，`brand-abbr` 0，改到的檔的 brand、colleague、pii 計數與 base 逐檔相同；`check-legacy-endpoints.py`、`rename.py --check`、兩支 App 的規則 E（`check_enum_text`）通過；`@hawkr/icons` 的 gitleaks 0；`rg -i 'iconly|lottie'` 只剩守門測試與兩份設計書（轉入步驟 6）。implementer 的 mutation：`HkIcon` 一個名稱改畫 `SvgPicture`、pubspec 加回 `lottie`、放回 `Icons.apple`、改 `.gen.ts` 的一個 path、import `react-iconly`、放回 `ri-search-line`、`MaterialSymbol` 預設尺寸改回 24，各自讓對應測試失敗。審查：開工設計審查 6 項（改 3、記 2、5d3 定 1）；程式碼審查每支 branch 1 輪（5d1 MED 1：OAuth 與付款列用 `Icons.apple`、`Icons.g_mobiledata` 商標圖形；5d3 MED 1：admin 的 Remix Icon，另排 5d4；LOW 5），主 session 另抓 1 項（admin 全域 CSS 蓋掉 `size`），修正輪 1 全部修完，5d4 審查 0；收尾設計審查 4 項全記（見 5d 段） 5e：主 session 在各 branch 最後一輪修正之後重跑：buyer-app format（0 changed）、analyze、custom_lint、deadcode（14 assets）、規則 E，改到的、import 改到的 lib 的與全部讀原始碼的 66 個測試檔 +561 全過；seller-app 同上（13 assets），62 個測試檔 +528 全過；兩支 App 的引用歸零（路徑最後兩段，含測試檔）0 列、清單裡的檔都不在；website `gen:placeholders` 重跑無 diff、lint（2,091 個檔）、`test:unit`（2,217 項，pass 2,216、skip 1）、vitest `--changed`（306 檔、3,102 項）、build、`test:build-output`（9），`@hawkr/icons` test 與重跑 gen 無 diff。主 session 的 mutation：兩支 App 的 `HkPlaceholder` 改畫 `Image.asset`，`hk_placeholder_test` 與 `asset_references_test` 失敗（buyer -11、seller -23）；改一個佔位 SVG 的一個字元，drift 測試失敗。合併後的 main：`scan.py --category assets` 661 → 0（G11、G04 為 0，新增 32 個佔位 SVG 記為 G14 自製）；`deleted-assets.tsv` 追加 661 列，刪除後的路徑引用全為 0（2 列被 website 的過時註解引用，改註解後歸 0），`--check-inventory` exit 0；brand（52 個檔、159 處）、brand-abbr、colleague、pii、endpoint 的計數與 base 相同；`check-legacy-endpoints.py`、`rename.py --check` 通過。implementer 的 mutation：佔位元件包回 `SizedBox(double.infinity)` 時無界父層的案例失敗；CSS 背景改回已刪的 `.webp`，引用測試失敗。審查見 5e 段「審查紀錄」 | e2e、Playwright、storybook 與視覺回歸（待 CI；`cart-image-fallback`、`cart-seller-selection` 只改了 fixture）；完整測試（延到步驟 8）。5b：invoice-service 只跑了讀 testdata 的 package（implementer） 5c：畫面沒有人眼看過（待 CI 視覺回歸）；e2e 兩支 spec 只改了原始碼。5d：畫面沒有人眼看過（待 CI 視覺回歸：全站圖示換圖形、App 導覽與空狀態改靜態、步驟條與通知改色塊加圖示、admin 圖示對齊）；website e2e 三支 spec 只改了原始碼；coverage ratchet（CI 專屬；兩支 App 的 baseline 只更新檔名清單，行數待 CI 重算）；兩支 App 與 website、admin 的完整測試（步驟 8） 5e：畫面沒有人眼看過（待 CI 視覺回歸：兩支 App 的頭像、封面、登入頁月球、下拉刷新與落葉、圖片編輯器的按鈕與貼紙、商品標籤頁的標章與按鈕、錢包分頁與「紀錄」標籤、物流規格卡；website 全站的佔位圖、賣家手冊與導覽、FruitBurst 圖集）；website e2e `seller-guide-touch.spec.ts` 只改了原始碼（刪掉守已刪閘道退回機制的兩支測試）；完整測試（步驟 8） | `92b9e6d524` 到 `91b34796dc`；5b `7dde8c36d0` 到 `aaa99de5a0`；5c `1cc1b604a7` 到 `e4349e5ddc`；5d `8eb8d87264` 到 `1b565b5579`；5e `d680f3d150` 到 `df5fd757e3` |
| 6 文件、測試資料、個資與指令檔 | 完成：6a–6c | 6c（`12aa9af7ae`–`86c69356a3`，2026-10-27）：主 session 在 rebase 後的 branch 與合併後的 main 重跑：`scan.py --category merchant,private,colleague,pii` exit 0（merchant、private 本步起改為守門類別）、`endpoint --include-uncertain` exit 0、gitleaks 追蹤中的檔 0、`check-legacy-endpoints.py` 通過、CodeMeta `generate.sh --diff` 206 相同；以舊版 scan.py 掃同一棵樹的 merchant、private 殘餘抽查：本機路徑是 `features/home/` 的子字串誤判，特店代號是 `HAWK` 加短號的合成值。implementer 跑過：`go vet`（含 integration tag）、ai-service goldenset、兩支 App 的 `coverage_ratchet_test`；mutation：暫存檔放特店代號、本機路徑、`AGENTS.local.md`，scan 各 exit 1。scan.py 校準：本機路徑加前置邊界、特店代號只認緊接賦值的值並略過 camelCase、佔位字樣加 `fake`／`testonly`、第三方 repo 的 issue 連結不算 refs、`HAWK` 短號與共用合成店號豁免。refs 只剩裸 `#編號`（盤點去向保留）。指令檔裡的 `AGENTS.local.md` 一度依 private 規則改寫成「本機個人設定」，Will 2026-10-27 決定改回檔名並從 private 規則移除（檔名是慣例，不洩漏內容；還原時帶回的個人路徑改成 `$HOME`）。PayUni 代理商前綴 `DALY` 全樹換成 `HAWK`（Will 2026-10-27；179 個檔，含 seller 的 CHECK、migration 與 `atlas migrate hash`、proto 註解與生成碼、CodeMeta 來源與生成檔、種子資料），重跑：`scan.py --category merchant,private,colleague,pii` 與 `endpoint --include-uncertain` exit 0、CodeMeta `--diff` 206 相同、改到的單體套件與 seller-service、gateway、pkg 的 Go unit、`cmd/hawkr` 的 `TestSchemaFileMatchesMigrations` 與 dev-seed、seller 的 PayUni integration、Java 8 個模組的 Maven test、admin `merchant-kyc-application`、seller-app `order_field_contract_test`、website `repo.unit.test.ts` 通過；mutation：暫存檔放本機路徑，private exit 1。刪 `website/debug-storybook.log`；新建 `backend/docs/plans/README.md`（hash 與 `#編號` 指私人封存）。<br>6b（`395e90f216`，2026-10-27，主 session 直接做）：gitleaks 對追蹤中的檔（工作區複本）0 筆；mutation：加一個 key 形狀的值 exit 1。S2 實測：共用 hex 值本來就是 `0123…` 的虛構值（誤報，行內標註），真正的舊憑證是 Java 那組 HashKey＋HashIV（11 個檔），換成 `TestOnlyFake…`；payuni-java-client、payment、shipping 的 Maven test 通過。排除方式：雜湊檔（`atlas.sum`、`Podfile.lock`、`buf*.yaml`）用 `.gitleaks.toml` 的路徑 allowlist，程式碼的測試值用行內 `gitleaks:allow` 附理由，文件的 7 筆公開範例列在 `.gitleaksignore`；PayUni 文件的 EncryptInfo 範例與 stagedToken 範例改成佔位文字。與盤點去向不同：光貿公開的測試 App Key 保留（換掉會失去唯一對照官方文件原值的簽章案例）。改到的 Go 套件、invoice-service、Java product／payment、website `delegated.unit.test.ts`、admin `delegated-operation-upstream.test.ts` 通過。<br>6a（`11f9ee57d5`–`589a56dfec`，2026-10-27）：主 session 在 rebase 後的 branch 重跑：`scan.py --category colleague,pii,endpoint --include-uncertain` exit 0；commit 訊息以 `--only-paths` 掃描，除暫存路徑本身的 brand-path 外為 0；改到的 proto 註解重跑 `make connect` 無 diff；invoice-service amego 簽章測試通過（golden 簽章因統編換成合成值而重算）。implementer 跑過（未重跑）：改到的 Go 套件與舊服務 unit、website vitest 216／node:test 88、admin node:test 95、兩支 App 改到的測試檔、Java 改到的服務 unit；mutation：暫存檔含同事帳號 scan exit 1。行為取捨：`market_registry.v1.csv` 門牌改虛構並重算 manifest sha256；seller-app 運送設定的客服信箱改 `service@example.com` | Go、Java integration（Docker 未啟動）；TS 與 Dart 生成碼的 diff 由 CI api-contract 檢查；e2e 與完整測試（步驟 8） | `11f9ee57d5`–`589a56dfec` |
| 6 文件、測試資料、個資與指令檔 | 未開始 | | | |
| 7 守門與 e2e 的 CI | 完成：7a、7b（7b 的 workflow 待步驟 8 實跑） | 7a（`ea9fc5f373`–`0dbf945608`，2026-10-27）：主 session 在合併前的 branch 重跑：`check-legacy-endpoints.py` 全樹 exit 0（29 秒）；`check-denylist.py` 全樹 exit 0（62 秒，高負載）；mutation（暫存檔 `git add -N`）：舊網域 → 舊端點檢查 exit 1、名字檢查 exit 1；中文舊名、縮寫後接小寫單字、camelCase 結尾的縮寫 → 名字檢查各 exit 1（中文舊名不在舊端點檢查的範圍，依 D7 名字只在名字檢查）。主 session 修正：舊端點檢查從每次 commit 改到 pre-push、名字檢查略過自身（縮寫補漏規則對自己的說明誤報）、步驟 6 漏掉的 13 個檔的舊 org 參照改成 `#編號` 或 `<org>/`（pricebook-service `go test ./...` 通過）。implementer：baseline 檔與 `--update`、`ZERO_TOLERANCE` 刪除，`legacy-endpoints.yml` 刪除，`.pre-commit-config.yaml` 加 pre-commit、pre-push hook type（兩個 hook 註明步驟 9 移除），根目錄與 backend `AGENTS.md` 改成本機 hook。7b（`74943d43dd`）：`actionlint` 全部 workflow exit 0；其餘 workflow 原本都有 `workflow_dispatch`；新增 `website-e2e.yml`（build 後跑 `test:e2e:hermetic` 兩個 project、`storybook:test`，缺視覺基準時以 `--update-snapshots` 產生並上傳 artifact）；admin 沒有 e2e 套件 | `website-e2e.yml` 實跑（步驟 8 的第一輪 CI）；目前沒有 spec 呼叫 `toHaveScreenshot`，第一輪不會產生基準；`website/.gitignore` 忽略 `tests/visual/**/*-snapshots/`，commit 基準前要改；`test:seo` 未納入（不同 build flavor） | `ea9fc5f373`–`74943d43dd` |
| 8 切換 | 未開始 | | | |
| 9 收尾 | 未開始 | | | |
