# AGENTS.md 分層與瘦身：指令檔只放改程式前必須知道的規則

**Goal：** repo 裡每個 `AGENTS.md` 只放「在這個目錄改程式前必須知道的現行規則與去哪裡查」；行為規格在設計書、理由在 ADR、不變量由測試守。每一層有大小上限，由 `scripts/check-agents-md.py` 在 CI 擋下超過的檔案，例外清單在收尾時是空的，或只剩 D4 指定擁有者的 seller 一項（步驟 7 的驗收）。

**上層：** 主清單見本機個人脈絡檔。本檔暫代「全域約束與完成定義」。

**附件：** 無（盤點的數字附在前提與步驟裡，產生指令寫在旁邊；快照 commit `9a5966d`）。

## 全域約束與完成定義（暫代主計畫）

- 搬走的內容一律先落到目的地（設計書、ADR、`CARRYOVER.md`）再從 `AGENTS.md` 刪除；不得只刪不搬。目的地已有同義內容時，刪除前在 commit 訊息寫明對應的段落。
- 引用 `AGENTS.md` 段落的地方（文件、程式註解、測試）在同一個 commit 改指向新位置；例外是已套用的 migration（`backend/internal/modules/*/db/migrations/*.sql`）：不得修改（`backend/AGENTS.md` 單體規則 6，改了 `atlas.sum` 也會失效），它們引用的規則在新 `AGENTS.md` 保留同名的錨點句，或在原節名處留一行指向新位置。
- 每個要改的目錄開工前跑 `git status --short <目錄>`；有其他 session 未提交的改動就跳過該單元並回報，不混進對方的改動。
- 不改任何程式行為；只動文件、模板、檢查腳本與它的測試。
- 完成定義（每個 `AGENTS.md`）：
  - 大小與單行長度在 D1 的上限內；
  - 只有模板的節（模組）或該層允許的內容（根目錄、子專案，見 D2）；
  - 每條不變量是一句規則，加上守門的測試名稱或 DB 約束，以及規格的位置；
  - 沒有日期、事件經過、「原本…改成…」這類歷史敘述。

## 前提（開工時核對過，2026-10-28）

- 會改到或取代的既有部署是否在線：不在線（系統從未上線，根目錄 `AGENTS.md`）。本計畫只改文件與 CI 檢查，回滾就是 revert。
- Claude Code 在 session 開始時載入根目錄的 `AGENTS.md`，讀到某個目錄的檔案時才載入路徑上各層的 `AGENTS.md`（來源：本 session 實測，Read `backend/internal/modules/media/media.go` 時注入了 `backend/AGENTS.md` 與 `media/AGENTS.md`，2026-10-28；已確認）。
- Codex 從 repo 根目錄串接到工作目錄的 `AGENTS.md`，合計預設上限 32KiB（`project_doc_max_bytes`），超過的部分截斷（來源：Codex 文件；未在本機實測，未確認）。
- 目前大小（bytes，`git ls-files '*AGENTS.md' | xargs wc -c`，commit `9a5966d`）：根目錄 20,779（去掉表格對齊空白後約 11,216）、backend 14,456、website 12,713、admin 10,117、buyer-app 4,585、seller-app 3,520、`packages/hawkr_auth` 457；模組：seller 168,637、buyer 16,100、social 15,461、identity 13,568、agent 10,550、accountdeletion 8,589、discovery 8,214、delegation 7,127、content 5,259、reviewqueue 4,360、media 3,346。
- 引用 `AGENTS.md` 的檔案（`rg -l "AGENTS\.md" --glob '!**/node_modules/**' --glob '!**/AGENTS.md' .`）：約 100 個，其中 `backend/docs/plans/` 的歷史計畫佔多數；程式與測試裡 25 處（`rg -c "AGENTS\.md" --glob '*.{go,ts,dart,sql}'`）。歷史計畫是紀錄，不改；現行文件、ADR、程式與測試的引用要跟著改。
- 模組 `AGENTS.md` 的角色由單體 ADR `docs/adr/2026-09-24-go-modular-monolith.md` 第 41 行定義（「職責、擁有的資料、不變量、讀取模型、不負責什麼」），由 `cmd/new-module/templates/module/AGENTS.md.tmpl` 產生；`cmd/hawkr/modules_test.go` 只禁止「沿用盤點」一節（已確認）。
- seller 模組正由範圍裁減計畫 `backend/docs/plans/2026-10-27-ecommerce-scope-cut.md` 修改中，`backend/internal/modules/seller/AGENTS.md` 有另一個 session 未提交的改動（`git status`，2026-10-28；已確認）。

**驗收基線：** 收尾前的 CI 是 commit `9a5966d` 上的結果；本計畫不新增會失敗的既有檢查。檢查腳本在步驟 1 建立，建立時的超限清單就是本計畫的工作清單。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 模組的 `pub_*` 讀取介面在擁有者的 `AGENTS.md`「發布的讀取介面」一節 | `backend/AGENTS.md` 單體規則 2 的最後一句；其他模組的 `AGENTS.md` 與 `CARRYOVER.md` 引用 |
| 模組 `AGENTS.md` 不得有「沿用盤點」一節 | `cmd/hawkr/modules_test.go`（約第 59–81 行） |
| `make new-module` 產生的 `AGENTS.md` 各節 | `cmd/new-module/templates/module/AGENTS.md.tmpl`、`cmd/new-module/main.go`（約第 66 行）、`cmd/new-module/main_test.go`（約第 242 行要求「## 模組內部結構」「依賴圖的唯一來源」「自己套件宣告最小的 `Deps`」「`CARRYOVER.md`」） |
| 已套用的 migration 不得修改 | `backend/AGENTS.md` 單體規則 6、各模組 `db/migrations/atlas.sum`；migration 註解裡對 `AGENTS.md` 的引用（例：`seller/db/migrations/20261021144547_seller_kyc_review_pause.sql:6`） |
| admin `AGENTS.md` 的部分內文由測試斷言 | `admin/tests/admin-design-documentation.test.ts`（約第 55–71 行） |
| `next dev` 會在 website、admin 的 `AGENTS.md` 補回 `nextjs-agent-rules` 區塊 | `next/dist/server/lib/app-info-log.js` 的 `ensureAgentRulesForDev`；大小上限要把這個區塊算進去 |

## 範圍

- 做：大小與格式的檢查腳本與 CI；單體 ADR 的修訂與模組模板；根目錄、子專案與 10 個非 seller 模組的瘦身；seller 的瘦身。
- 不做：改程式行為；改歷史計畫（`backend/docs/plans/` 已完成的計畫）裡對 `AGENTS.md` 的引用（它們是紀錄）；`AGENTS.local.md`（不進版控，已另外整理）。
- 延後：依 D4。

## 步驟

- [x] **1. 檢查腳本與 CI**（被擋於：D1、D2）
  - 範圍：新增 `scripts/check-agents-md.py`（只用標準函式庫）：對 `git ls-files '*AGENTS.md'` 逐檔檢查層級（根目錄、子專案、`backend/internal/modules/*`、`packages/*`）的大小上限（bytes）與單行長度上限（字元），數值依 D1；`nextjs-agent-rules` 區塊不計入（由 `next dev` 維護）。例外清單寫在腳本裡，每項附「由第幾步清掉」；建立時列入所有超限的檔。附 `scripts/test_check_agents_md.py`（`unittest`，repo 目前沒有 Python 測試，不引入 pytest）。在 `.github/workflows/repo-hygiene.yml` 加兩步：`python3 scripts/check-agents-md.py` 與 `python3 -m unittest scripts/test_check_agents_md.py`。
  - 消費端：`.github/workflows/repo-hygiene.yml`；`rg -n "check-agents-md" .`
  - 不能動：其他 hygiene 檢查。
  - 驗收：`python3 scripts/check-agents-md.py` 在建立時的 HEAD 通過；`python3 -m unittest scripts/test_check_agents_md.py` 通過；mutation：把一個不在例外清單的檔加長到超過上限，腳本要失敗；在例外清單放一個已經不超限的檔，腳本要失敗（例外只能縮小）；push 後 repo-hygiene 的這兩步在 CI 的 job 紀錄裡有執行。

- [x] **2. 修訂單體 ADR 與模組模板**（被擋於：D2）
  - 範圍：依 ADR 修訂流程修訂 `docs/adr/2026-09-24-go-modular-monolith.md` 第 41 行：模組 `AGENTS.md` 是「改這個模組前必須知道的規則」，固定節為 D2 的模組節；行為規格的唯一來源依 D3。D3 選 A 時，`backend/AGENTS.md`「契約規則」的 SSOT 優先序裡「設計書」一層加上 `docs/modules/`，並在步驟 4、6 碰到的每份舊設計書檔頭標明「單體模組的現行規格見 `docs/modules/<模組>.md`」。改 `cmd/new-module/templates/module/AGENTS.md.tmpl`、`main.go` 與 `main_test.go`。`docs/開發規範/設計文件治理規範.md` §1 的文件 taxonomy 加一列「模組規格」（`docs/modules/`，與詳細設計書同樣的 PURE SPEC 規則）。單體 ADR 的修訂已在 ADR 拍板時寫好（2026-10-28）。`backend/AGENTS.md` 單體規則 2 的最後一句改成新的節名（若 D2 改了節名）。
  - 消費端：`rg -n "AGENTS\.md" backend/cmd backend/internal/arch backend/docs/adr`；`main.go` 約第 66 行與模板第 3 行對 content 模組的引用。
  - 不能動：「沿用盤點」放 `CARRYOVER.md` 的規則與 `modules_test.go` 的檢查。
  - 驗收：`cd backend && GOWORK=off go test ./cmd/new-module/ ./cmd/hawkr/`；`main_test.go` 改成斷言 D2 的固定節；mutation：模板加回 D2 不允許的節（例如「## 已知限制」），`main_test.go` 要失敗；刪掉一個固定節，也要失敗。

- [x] **3. 根目錄與子專案**（被擋於：步驟 1）
  - 範圍：根目錄表格去掉對齊空白並把契約表的長格收到 D1 的單行上限內（細節移到對應 ADR）；backend、website、admin 依 D2 的子專案內容界線收斂到上限內；buyer-app、seller-app 的「元件庫維護」重複段合併到一處並互相指向。
  - 消費端：`rg -n "AGENTS\.md" --glob '!backend/docs/plans/**' --glob '!**/node_modules/**' .` 逐筆分類（指向本步驟改的檔、指向模組、與本計畫無關）。已知要跟著改的：`admin/tests/admin-design-documentation.test.ts`（斷言 admin `AGENTS.md` 內文）、`website/tests/visual/_helpers.ts:6`（引用節名「CSS 現狀 & 遷移方向」）、`admin/` 下 `ConvertPanel.tsx:31`（引用「UI 契約」）。
  - 不能動：根目錄的跨專案契約表的語意、`nextjs-agent-rules` 區塊。
  - 驗收：步驟 1 的腳本通過，例外清單少掉這幾檔；`cd admin && pnpm test tests/admin-design-documentation.test.ts` 通過；`website` 的 visual helper 只在 CI 跑（本機不跑瀏覽器測試），標「待 CI」。

- [ ] **4. 非 seller 的模組（buyer、social、identity、agent、accountdeletion、discovery、delegation、content、reviewqueue、media）**（被擋於：步驟 1、2，D3）
  - 範圍：每個模組一個子步驟（4a…4j），各自驗收、各自 commit。開工前照全域約束檢查未提交改動（2026-10-28 時 reviewqueue 有其他 session 的改動）。照新模板重寫：行為規格搬進 D3 決定的位置，理由搬進 ADR，已知限制搬進同一個規格位置；不變量改成一句規則加守門測試名稱與規格位置。同一個 commit 改掉現行文件、ADR、`CARRYOVER.md`、程式與測試對被移除段落的引用（migration 除外，見全域約束）。
  - 消費端：`rg -n "AGENTS\.md" --glob '!backend/docs/plans/**' --glob '!**/node_modules/**' .` 逐筆分類；特別注意沒寫模組名的「`AGENTS.md`「已知限制」」寫法（例：reviewqueue、agent 的 `CARRYOVER.md`）。
  - 不能動：`pub_*` 的形狀與說明（只能改寫法，不能刪掉）；任何程式。
  - 驗收：腳本通過、例外清單少掉該模組；`cd backend && GOWORK=off go test ./cmd/hawkr/ ./internal/arch/`；搬走的每條規格在 commit 訊息附「原段落 → 新位置」；抽查：從新 `AGENTS.md` 的每個「規格位置」連結都能打開到含那條規則的段落（`python3` 小腳本或人工逐條，結果貼進進度表）。

- [ ] **5. seller 開工前的確認**（被擋於：範圍裁減計畫的食品業者登錄步驟已 commit）
  - 範圍：確認 `backend/internal/modules/seller/AGENTS.md` 沒有其他 session 的未提交改動；量測當時的大小與節。
  - 驗收：`git status --short backend/internal/modules/seller/` 沒有 `AGENTS.md`；大小記進進度表。
  - 停止條件：仍有未提交改動就停下，不和對方的改動混在一起。

- [ ] **6. seller 模組**（被擋於：步驟 2、5，D3、D4）
  - 範圍：同步驟 4，依節拆成子步驟：6a 不變量、6b 已知限制、6c 其餘節。規格的位置依 D3。
  - 消費端：同步驟 4；另有 ADR `2026-10-19-seller-kyc-payuni-consolidated.md:44`、`backend/docs/runbooks/seller-food-registration-runbook.md:3`（範圍裁減若已刪這份 runbook 就略過）。
  - 不能動：同步驟 4。
  - 驗收：同步驟 4；依 D4，例外清單少掉 seller，或 seller 的例外項改成指向拆模組計畫。

- [ ] **7. 收尾**：完成定義逐項核對；獨立設計審查（design-review skill）；`scripts/check-agents-md.py` 的例外清單為空（D4 選 B 時只剩 seller 一項，且指向拆模組計畫）；延後項附觸發條件搬回主清單。驗收：`python3 scripts/check-agents-md.py` 通過；`rg -n "EXCEPTIONS" -A20 scripts/check-agents-md.py` 的清單符合上述；repo-hygiene CI 綠燈。

## 決定

- **D1 大小與單行上限**（狀態：已決 2026-10-28，Will；擋住步驟：1、3、4、6；需要的事實：無）
  - 大小一律以 bytes 計（中文一字 3 bytes），單行以字元計。
  - A：根目錄 12,000 bytes、子專案 10,000、模組與 `packages/*` 6,000；單行 500 字元。依據：去掉對齊空白的根目錄約 11,216 bytes；最大的子專案 backend 14,456 收斂後可到 10,000；模組固定節各一段約 4,000–6,000。在最深的路徑（根目錄＋backend＋模組）合計 28,000 bytes 以內：Claude Code 每次碰模組要載入的量有上限；若 Codex 文件的 32KiB 上限屬實（未實測），也落在其內。（建議）
  - B：統一 8KB、單行 400 字元——更嚴，根目錄的契約表要大幅外移到 ADR，backend 的單體規則要拆檔。
  - C：只限模組（6KB），根目錄與子專案不設限——工作量小，但根目錄與子專案會再長回去。
  - 結論：A。ADR：`docs/adr/2026-10-28-agents-md-layering.md`。
- **D2 各層的內容界線**（狀態：已決 2026-10-28，Will；擋住步驟：1、2、3、4、6）
  - A：根目錄＝專案地圖、跨專案契約、共通規則；子專案＝指令、技術慣例、該子專案特有的坑；模組＝固定七節：職責（一段）、擁有的資料、發布的讀取介面（`pub_*`）、模組內部結構（依賴圖的唯一來源與 `Deps`，`main_test.go` 現在就要求）、不變量（每條一句＋守門測試＋規格位置）、不負責、決策紀錄（只有一行指向 `CARRYOVER.md` 與相關 ADR）。讀取模型併入「擁有的資料」一行；已知限制與特性測試不在模組 `AGENTS.md`（前者進規格位置，後者進 `CARRYOVER.md`）。（建議）
  - B：模組保留現在的節，只限大小——不改 ADR，但規格仍會寫進來，靠上限硬擋。
  - 結論：A。ADR：`docs/adr/2026-10-28-agents-md-layering.md`。
- **D3 模組行為規格搬去哪裡**（狀態：已決 2026-10-28，Will；擋住步驟：4、6）
  - 現況對照（取自各模組 `AGENTS.md` 開頭的引用）：buyer → 39、59；content → 25、54；delegation → 56；identity → 44；media → 08；seller → 32、44（另有 18、46 相關）；accountdeletion、agent、discovery、reviewqueue、social 的開頭沒有引用設計書。這些設計書多是公司時期為微服務寫的（例：08 是多媒體微服務），內容和單體的現況有落差。
  - A：每個模組新增 `backend/docs/modules/<模組>.md`，作為單體模組行為規格的唯一來源，舊設計書只當歷史參考、在檔頭標明；SSOT 優先序裡「設計書」一層改指向它。規格只有一個家，而且是為單體寫的，不混進舊服務文件。（建議）
  - B：寫進既有的詳細設計書，加一節「單體的現行不變量」；沒有設計書的模組新增一份——不新增目錄，但同一份文件混著舊微服務的設計與單體的現況，讀的人分不清哪段有效。
  - C：Go 的 package doc（`doc.go`）——在程式旁邊，但不適合長篇規格，也不在設計書的引用體系裡。
  - 結論：A。ADR：`docs/adr/2026-10-28-agents-md-layering.md`。
- **D4 seller 瘦身後仍超過模組上限時怎麼辦**（狀態：已決 2026-10-28，Will；擋住步驟：6、7）
  - 事實：seller 的 `AGENTS.md` 168,637 bytes、`CARRYOVER.md` 約 283KB；職責涵蓋治理、潛在賣家、KYC、協議等多個子領域，搬走規格後是否能到 6,000 bytes 未知。
  - A：把「seller 拆成多個模組」排進本計畫成為步驟 6d，收尾時例外清單必須為空——治本，但拆模組是架構變更（依賴表、schema、事件），規模遠大於本計畫。
  - B：seller 拆模組另立一份階段計畫；本計畫收尾時 seller 是例外清單的唯一一項，該項寫明「由拆模組計畫清掉」，拆模組計畫的完成定義包含把它移出例外清單。例外有擁有者與關閉條件，不是無主的債。（建議）
  - C：放寬模組上限到 seller 塞得下——等於沒有上限。
  - 結論：B。ADR：`docs/adr/2026-10-28-agents-md-layering.md`。

拍板：2026-10-28 Will 四題照建議。

- **D5 兄弟子專案共用的規則**（狀態：已決 2026-10-28，Will；步驟 3 之後追加）：先讓規則不需要存在（CI 強制、刪掉描述不存在流程的、留一行指向各自文件），不寫兄弟互指；真正共用又無法強制的才以 `@路徑` 引用一份中立檔。ADR：`docs/adr/2026-10-28-agents-md-layering.md` 文末修訂。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 檢查腳本與 CI | 完成 | 本機：`python3 scripts/check-agents-md.py` exit 0；`python3 -m unittest scripts/test_check_agents_md.py` 8 項 OK；mutation：media 加長到 9,947 bytes 與 600 字元的一行 → exit 1；例外清單放不超限的 buyer-app → exit 1；`actionlint` 通過 ；CI repo-hygiene run 40552353091（commit 41ed095）兩步 success | 無 | e495fe3 |
| 2 ADR 與模組模板 | 完成 | 本機：`GOWORK=off go test ./cmd/new-module/ ./cmd/hawkr/` ok；`go vet ./cmd/new-module/`、`gofmt -l` 無輸出；新測試 `TestGeneratedAgentsHasOnlyTheFixedSections` 在舊模板上失敗（節是職責…已知限制、讀取模型…）；mutation：模板加回「## 已知限制」→ 失敗，刪掉「## 不負責」→ 失敗；`python3 scripts/check-agents-md.py` exit 0。`backend/AGENTS.md` 單體規則 2 的節名「發布的讀取介面」與 D2 相同，未改；產生器不建 `docs/modules/<模組>.md`，main.go 的下一步提示指向它 | backend-monolith CI（push 後看） | 見 git log |
| 3 根目錄與子專案 | 完成 | 本機：`python3 scripts/check-agents-md.py` exit 0（EXCEPTIONS 拿掉根目錄、backend、website）；`python3 -m unittest scripts/test_check_agents_md.py` OK；mutation：backend 加長到 10,067 bytes → exit 1。大小（website 不含 nextjs 區塊）：根目錄 10,215、backend 9,945、website 8,254；admin 本來就在上限內，未改。backend 的結構與規則全文搬到 `docs/開發規範/單體開發規範.md`（編號不變），website 的多語系路由全文搬到 `website/docs/i18n-routing.md`，根目錄契約表的細節由既有 ADR／規範／設計文件承接；App 的元件庫維護共用三條留在 buyer-app | 引用節名的 `website/tests/visual/_helpers.ts`（節名未改）只在 CI 跑；`admin-design-documentation.test.ts` 未跑（admin 未改） | 見 git log |
| 4 非 seller 的模組 | 未開始 | | | |
| 5 seller 開工前的確認 | 未開始 | | | |
| 6 seller 模組 | 未開始 | | | |
| 7 收尾 | 未開始 | | | |
