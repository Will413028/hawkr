# A3b 步驟 10a 開工的決定題

HEAD `f60528fe5`。四份唯讀盤點（X-Perspective、TS 共用套件、契約欄位、B4，未入庫）加上兩輪獨立審查。

**結論（Will 2026-10-07）**：全照建議，以「第二版」為準（Will 要求以業界基準重評）：T1 改成整份生成碼加執行期的套件與版本統一；T4 同意以 GCIS 公開 API 取樣，快照存結構化的登記現況。第一版的 X-Perspective 改名與 website 工作區改名的知會、T4 的「存原樣文字」與切分已撤回，切分以計畫 10a 段為準。決定的落點：計畫 10a 段、ADR `2026-10-07-connect-without-perspective.md`、推薦排行 ADR 與 KYC ADR 的 2026-10-07 修訂。

## 照 SSOT 直接做、不問（知會）

- `PublicStore` 加 `seo_title = 10`、`seo_description = 11`，名稱與長度限制照 `admin.seller.v1alpha1.Store` 17、18。
- B2：`GetMyStoreResponse` 加 `StoreStats stats = 5`；`App` 注入粉絲數與 `Ratings` port，與 `GetStore` 共用 `Public.stats`。
- B3：`StoreCard` 加 `town_code`（plain string，空字串表示不設；API 規範:100，與 `PublicStore.town_code` 同形）、`optional int32 review_count`、`optional float rating_average`（「不設」＝讀不到）。批次評分組裝放在 `httpapi/seller`（與 `statsBudget` 同處，300ms），buyer 清單、`BatchGetStores`、discovery 共用；失敗只讓評分不設，不讓清單變 `Unavailable`。
- X-Perspective：header、讀取、單體 CORS allow-list（`internal/httpapi/origin.go:19`）、`internal/legacy/gatewayproxy/proxy.go` 轉發前刪除的清單（舊名直接換掉，不保留：gateway 與 Java 都不讀它）、website 代理 e2e 不快取的判斷（`app/api/v1/[...path]/route.ts:48`）、admin BFF 邊界測試、`identity.proto:10` 的註解、規範、設計書 17、44 與其他描述舊流程的文件（「gateway 依 header 選角色」的段落改寫）。dated ADR 與 reserved 的 `active_role` 不動；gateway 的「active role」（解析後的 `X-Login-Role`）是另一個概念。
- website 的「工作區」狀態一起改名成 perspective（識別字、檔名、localStorage key；約 158 個檔，其中約 71 個測試檔，獨立 commit）：計畫:67 的理由同樣適用，系統沒上線，key 不用搬移。不同意就說。
- B4 的細節：只有公司型 KYC 有快照（個人沒有統編）；job 照 `payuni/jobs.go` 的唯一性與三段式，以送件序號擋舊結果覆寫新送件；`splitAddress` 移到 `platform/twaddr` 共用；gcis 套件註解與 `CARRYOVER.md:264` 依 ADR 修訂改寫。

## 要 Will 決定

### T1. TS 共用套件：範圍與 workspace

**來源**：試點計畫:170（Will 2026-09-26）「pnpm workspace 的共用套件（Turborepo Internal Packages＋`transpilePackages`）」，範圍是錯誤分類器與重試；計畫:145（Will 2026-10-07）的 10a 同此範圍。
**現況**：website、admin 各是獨立的 pnpm workspace（各自的 lockfile、overrides、`patchedDependencies`；pnpm 11.2.2 對 10.16.0；TS 7.0.2 對 ^5）。`errors.ts` 兩份除一行註解外相同，import 各自生成的 `ErrorReason` 與 `google.rpc`；沒有其他 proto import `error_reason.proto`。重試不同：website 有伺服器端模式（Workers 不 sleep）。
**業界基準**：Turborepo／Next.js monorepo 範本是根目錄一個 workspace、一份 lockfile、`workspace:*`。

**T1-1 套件放什麼**
| 選項 | 取捨 |
|---|---|
| (a) 分類器、重試，加上它們用到的生成碼（`ErrorReason`、`google.rpc`）；兩端 `buf.gen.yaml` 排除這兩個路徑 | 符合 9/26 與 10/7 的範圍；`ErrorReason` 只生成一份 |
| (b) 整份 TS 生成碼也搬進來（TS 版 `hawkr_api`） | 一份生成、一份 drift；但 website 刻意不生成 admin 的 API，要嘛帶進 descriptor、要嘛拆兩個套件；兩端所有 `lib/gen` 的 import 改寫；擴大 10a 範圍 |
建議 **(a)**。(b) 的觸發：第三個 TS 消費端出現，或兩端生成設定開始分歧到需要對齊。

**T1-2 workspace 形狀**
| 選項 | 取捨 |
|---|---|
| (a) 根目錄一個 workspace（基準） | `workspace:*`、一份 lockfile；要做：pnpm 統一 11、合併 overrides（brace-expansion 寫法不同、tar／ws／form-data 只有 admin 有）、website 的 `patchedDependencies`（next、opennext）會作用到 admin 的 next、admin 改用 `allowBuilds` 並繼承 `minimumReleaseAge`、`check-dependency-overrides.ts` 改讀根目錄、CI 改在根目錄安裝並調整 cache 與 drift 路徑、website 的 Turbopack root 放寬並以 `opennextjs-cloudflare build` 驗 file tracing、根 `AGENTS.md` 地圖 |
| (b) 兩個 workspace 不合併，以 `link:` 引用 | 改動小；套件的依賴（`@bufbuild/protobuf`、`@connectrpc/connect`）由兩端各自解析，是 peer 依賴的折衷，不是 Internal Packages |

約束帳：版本不同、overrides 各自維護、CI 各自安裝、`packages/` 只有 Dart（`dart-packages.yml` 觸發 `packages/**`）都是 SELF-IMPOSED；套件同時要過 TS 7 與 TS 5 在兩端版本不同期間是 REAL（對齊 TS 是另一件事）。Workers 的 build 本機可驗，不是阻礙。
建議 **(a)**，並且 workspace 合併是 10a 的第一個 commit。

### T2. 推薦清單的評分

**衝突**：ADR 2026-10-03 的 10-07 修訂說每張 `StoreCard` 的評分由 `internal/httpapi` 批次組裝；推薦排行 ADR（2026-10-03:54）要 `RecommendedSeller` 帶「與排名一致」的評價數與平均（排行快照，每 15 分鐘）。並存時同一家店兩個數字。
**業界基準**：Etsy、Amazon 的卡片都顯示同一個即時評分，排名用的訊號不對外。

| 選項 | 取捨 |
|---|---|
| (a) 卡片即時讀（與其他清單相同），刪掉 `RecommendedSeller.review_count`、`rating_average`（`reserved`），修訂排行 ADR 的「輸出」段 | 一個數字、全站同一個來源；推薦多一次 legacy 批次讀取（與其他清單相同） |
| (b) discovery 用排行快照填卡片的評分，刪掉 `RecommendedSeller` 的兩欄 | 不多讀；同一欄位的新鮮度隨清單不同 |
| (c) 兩個都留 | client 要知道顯示哪一個 |

約束帳：「與排名一致」是我們先前的 ADR，SELF-IMPOSED；review 在 Java，REAL（B 階段）。消費端：website、buyer-app 的推薦列在 10d 才改用 Connect，現在沒有讀這兩欄的 client（10d 前驗證）。
建議 **(a)**。終點：review 搬進單體後，評分改讀它的公開 API，契約不變。

### T3. B4：審核依據的機制

ADR 修訂只寫「以當下的快照作為審核依據」。Approve／Reject 現在只比對 KYC 的 `version`（與賣家端的樂觀鎖共用）。
**業界基準**：AIP-154 的 etag 代表回傳的整個資源；Stripe、Middesk 的審核紀錄指向它依據的那次驗證。

| 選項 | 取捨 |
|---|---|
| (a) 後台檢視的 etag 由 KYC `version` 與快照序號組成；快照變了，舊 etag 核准回 `aborted`；`kyc_reviews` 記下依據的快照（查詢時間、結果，沒有快照記「無」） | 沿用既有的衝突處理；賣家端的 etag 不受影響 |
| (b) 快照寫入時遞增 KYC `version` | 一個版本；但賣家端在審核中不能改，實際無害，語意卻混在一起 |

另外四點一併確認：
- 沒有快照或結果是「不可用」時允許審核，畫面標示（ADR「不一致只標示、不自動退件」）。
- 只在「審核中」可重新查詢；決定之後才回來的首次結果照樣寫入，不影響已記下的依據。
- 比對規則：名稱與地址明細做 NFKC、台／臺、空白正規化後比較；地址先以 `splitAddress` 拆出 `town_code`，拆不開記「無法比對」，不算不一致；組織型態以 gcis 的結尾對照申請的 `company_type`，對不上的值記「無法比對」；設立日期零值是「未知」。
- 快照隨 KYC 個資一起清除（關店 30 天）：審核依據的價值在店家存續期間，關店後與申請本身同樣清除。

建議 **(a)** 加上以上四點。

### T4. 快照要不要存登記現況（核准設立、解散、廢止）

`CARRYOVER.md:245`（8b）：gcis 不回登記現況，理由是「現況欄位沒有可驗證的樣本，審核員以核准時的登記資料為準」——那時用途是填表帶入。KYB 的基準（Middesk、Stripe）把登記現況當成最主要的訊號：已解散的公司不該通過。

| 選項 | 取捨 |
|---|---|
| (a) 存原樣的現況文字，只顯示、不參與不一致判斷 | 審核員看得到「解散」；欄位名稱照公開文件、沒有真實樣本，讀不到時顯示「未提供」 |
| (b) 不存 | 維持 8b；審核員看不到解散 |

約束帳：沒有真實樣本，REAL（要連 GCIS 公開 API 取樣，是對外呼叫，需要你同意）。
建議 **(a)**。結構化（依現況自動標示）的觸發：取得一份真實的解散或廢止樣本。

## 10a 的切分（每刀一個 commit，依序）

1. workspace 合併（T1-2）。
2. 契約與後端：PublicStore SEO、B2、B3、T2，`make connect`、各端重新生成。
3. B4（`admin/seller/v1alpha1`）：migration 由主 session 產生、job、admin RPC 與權限。2 與 3 改不同的 proto，但都重新生成同一批目錄，所以依序。
4. TS 共用套件搬入（T1-1，改 `buf.gen.yaml` 與 `lib/connect`）。
5. X-Perspective（含 website 工作區改名）：改 `lib/connect/transport.ts`，排在 4 之後。

## 審查處理

改：GCIS 現況（T4）、T1 加 (a) 只搬生成的錯誤型別、T1 擴大範圍的說明、合併的代價、Workers 改成驗證步驟、T2 加業界基準選項與約束性質、T3 的 etag 機制與比對規則、刪除清單不保留舊名、T2→知會的工作區改名數字、切分順序與理由、評分組裝的位置、路徑與細節。駁回：無。

## 第二版（Will 要求：以業界基準重評，不受自己加的約束限縮）

### T1 改成基準
- T1-1 改建議 (b)：一個 TS 套件 `@hawkr/api`＝整份生成碼＋Connect 執行期（錯誤分類器、重試、共用的 interceptor 積木），與 Dart 的 `packages/hawkr_api`（生成碼＋`transport.dart`）對稱。website 不生成 admin API 的理由不成立：protoc-gen-es 每個 proto 一個模組，沒 import 的不進 bundle；admin API 的形狀不是秘密，保護在 Cloudflare Access。先前選 (a) 的理由是「10/7 的範圍」，屬 SELF-IMPOSED（計畫分段）。
- T1-2 維持根 workspace，並加 pnpm catalogs 統一共用依賴版本（typescript、next、react、@bufbuild/protobuf、@connectrpc/connect），admin 升 TS 7、Node 統一一個版本（根 `.nvmrc`；admin CI 與 api-contract 的 Node 22 改讀它）。「套件要同時過 TS 5 與 7」這條約束因此消失。Turborepo 工具本身不引入：兩個 app 一個套件，`pnpm -r`／`--filter` 足夠；觸發：CI 時間成為瓶頸。

### T5（新）X-Perspective：改名的前提不成立
- 事實：伺服器端的 Perspective 只有兩個用途：(1) `gatewayproxy` 轉成 `X-Login-Role` 給舊 gateway（gateway 仍以它區分買家／賣家的 REST 路由，`gateway/internal/middleware/auth.go`）；(2) `GetSession` 把它回傳成 `Account.perspective`，website 以它當工作區的預設（`website/lib/auth/active-role.ts:44`）——client 送、server 照規則回，是繞一圈。沒有任何 Connect 模組以視角做授權或改變行為；Connect 的服務名稱（`SellerAppService`／`BuyerAppService`）本身就表示視角。
- 業界基準：Etsy、Shopify、Amazon 的買家與賣家是不同的 API 面（不同 endpoint 或 app），「正在看哪個工作區」是 client 的 UI 狀態，不是每個請求的 header。
- 選項：(a) 照計畫:67 改名；(b) Connect 拿掉視角：刪 `rpc.Principal.Perspective`、`Account.perspective`（reserved）與 Connect transport 送的 header，website 的預設規則移到 client（有賣家角色且上次停在賣家工作區才是賣家）；header 只留在舊 REST 的轉發路徑，由 `gatewayproxy` 自己讀，名稱維持 `X-Active-Role`（舊 wire 的一部分），退場條件：gateway 最後一條區分角色的 REST 路由退場。
- 約束帳：舊 gateway 需要知道視角，REAL（排定的模組搬遷之前）；計畫:67 的改名，SELF-IMPOSED（前提是 header 是長期契約，已不成立）。
- 建議 (b)。website 的工作區狀態改名不做（它是 UI 的工作區，跟 header 名稱脫鉤後沒有不一致）。

### 第二版審查的處理（11 項全收）
- T5 事實更正：gateway 不是以 X-Login-Role 選路由，而是 handler 在同一條路由內選身分（post、review、notification_inbox、sse_stream、moderation、marketplace、chat、address_geocode、seller_forum、resolve_seller），chat 的 ws-ticket 把它寫進票，IM gRPC 也讀。退場條件：最後一個讀 loginRole 的 gateway／IM 消費端退場。
- T5 預設規則：`PerspectiveOf`（SELLER 提示＋有賣家角色，否則 BUYER）移進 `internal/legacy`，從 identity 公開 API 刪除；proxy 測試保留「沒有提示」「非賣家帶 SELLER」。
- T5 範圍：送 header 的舊 REST 端（website `lib/api/client.ts`、`ai-chat/stream-client.ts`、`sse/client.ts`、`notifications/repo.ts`；App 的 Dio 與 ws-ticket POST）保留；`ActiveRole`／`kActiveRoleHeader` 從 `packages/hawkr_api/lib/transport.dart` 移到 App 的 REST 層；文件：API 規範:32、`identity.proto` 註解、identity `CARRYOVER.md:75`、ADR 2026-09-30 加修訂、計畫 :67／:167／:169、website 的相關測試與 e2e mock。撤回改名的知會與切分第 5 刀。
- T1：套件以 subpath exports、不做 barrel；Dart 端同樣排除 admin，對稱只是形狀；catalog 不吞掉 website 的 `typescript-compiler-api` 別名（TS 6）；根 `.nvmrc` 用 LTS（Node 24，觸發：Node 26 進 LTS），取代 website 的 25；next 的 patch 套到 admin 是刻意的；admin 改 `allowBuilds`；Internal Packages 是不需要 turbo 的模式，不算推翻 9/26。
