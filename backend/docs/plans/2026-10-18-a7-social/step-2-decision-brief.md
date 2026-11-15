# A7-2 對外 API 設計簡報（social＋moderation 的 Connect）

證據標記：【量】＝讀檔或跑指令看到；【推】＝由讀到的內容推論；【猜】＝憑業界常識、未查證本 repo 外的來源。
路徑相對 worktree `.worktrees/a7-2-external-api`；計畫檔簡稱「計畫」＝`backend/docs/plans/2026-10-18-a7-social.md`，API 規範簡稱「規範」＝`docs/共通規範/API 設計規範（Connect）.md`。

## 結論先講

1. A7-2 的大方向（依使用者類別分 service、讚做成冪等的自訂方法、瀏覽另開 POST、moderation 自成 package、keyset 分頁）都在業界基準內，不需要重構。【推】
2. **有一個真缺陷要在 A7-2 修**：`PublicService` 的 GET 一律回 `Cache-Control: public, no-cache`，不看呼叫者有沒有登入（`backend/internal/httpapi/httpcache.go:20-35`【量】），也沒有 `Vary: Authorization, Cookie`（同檔 :37 只加 `Accept-Encoding`【量】）。A7-2 要在公開讀取裡放依觀看者而定的內容（`viewer_can_*`、`is_mine`，計畫:418；買家封鎖的店的貼文不出現，計畫 A7-4a 段），CDN 或任何共享快取就可能把 A 的個人化回應給 B。【推；目前所有 Public 回應都不依觀看者，所以還沒出事：`api/hawkr/*` 唯一的觀看者狀態 `followed_store_ids` 在 `BuyerAppService`，`api/hawkr/buyer/v1alpha1/buyer.proto:395`【量】】
3. 把 A7-2 依「資源的垂直切片」拆成 5 個 session，不依層（proto→handler→測試）拆。

## 0. 前提題：A7-2 碰到的程式有沒有排定之後要改寫或刪？

| 會被改或刪的東西 | 排程 | 何時 | A7-2 在它上面花的功夫 | 分類 |
|---|---|---|---|---|
| gateway 的 40 條 social REST＋6 條代行路由 | A7-5 刪，A7-2d 先換代行 | A7-5 | 0：A7-2 不改它們，只新建 Connect（計畫:410-433）【量】 | SELF-IMPOSED（我們的分段） |
| `services/social-service`、`api/proto/social` | A7-5 刪（S1，計畫 decisions S1 列） | A7-5 | 0 | SELF-IMPOSED |
| 四端手寫 client | A7-4 換成生成 client | A7-4 | 0：A7-2 不碰 client | SELF-IMPOSED |
| 特性測試的 legacy driver（`internal/httpapi/social/characterization`） | 最後一個 `legacy.sh` 刪時一起刪（A7-2v） | A7-5 | 中：A7-2 要補封鎖的情境與 Connect driver（計畫:414、:428） | SELF-IMPOSED，但它是遷移的驗收工具，丟掉是設計好的終點，不算返工 |
| 舊 im／review 的封鎖與檢舉 | A6（S3） | A6 | 0 | SELF-IMPOSED |

結論：A7-2 本身是「終態程式」，沒有寫了又要丟的部分；唯一的拋棄物是特性測試 driver，那是驗收工具。沒有 REAL 的排程限制（沒有外部方、法規、缺資料或不歸我們的依賴）。【推】
需要留意的是反方向：A7-2 的 proto 一旦被 A7-4 四端消費，改形狀要四端一起改；但 `v1alpha1` 加上「上線前不受相容性約束」（根 `AGENTS.md` 共通規則），改的成本只是重生成＋client 跟改，可逆。【推】

## 1. 各設計點對照業界

每點格式：基準｜現有設計與它為什麼存在｜選項｜失敗條件｜推翻建議的事實｜可逆性｜約束帳。

### 1.1 service 依使用者類別（Public／App／Admin）切，而不是依資源

- 基準：Google AIP 是一個 service 管一組資源，授權逐方法判斷【猜】；但「依讀者切 API」也是主流：Shopify 的 Storefront API（公開、可快取）對 Admin API（員工）、Stripe 的 publishable／secret key 面、Reddit 的 public JSON 對 OAuth API【猜】。
- 現有：規範 §二（:22-30【量】）定「一個使用者類別一個 service，授權跟著 service 走」，有 interceptor、宣告表與快取規則（§九）綁在 audience 上（`httpcache.go:20` 用 `rpc.Audience`【量】）。A7-2 照它做 `SocialPublicService`／`SocialAppService`／`SocialAdminService`（計畫:412-415）。
- 判斷：符合 Shopify 型的基準，且讓「單一執行點」的授權成立（A7-D18 引的 OWASP）。**照做**。
- 唯一要補的是 1.2 的快取。
- 約束帳：規範 §二＝SELF-IMPOSED（我們的規範），但它本身就是業界做法，不是遷就舊碼。
- 推翻條件：出現一個方法同時要給訪客與管理員、且回應形狀相同（目前沒有）。可逆性：高（改 service 名＝重生成）。

### 1.2 依觀看者而定的欄位與 Public 快取（**本步驟的主要缺陷**）

- 基準：Reddit、X、Instagram 的讀取 API 在登入時把 `likes`／`viewer_has_liked` 之類的欄位內嵌回去，回應標成私有；匿名時回可共享快取的版本【猜】。另一派（Twitter 早期、Mastodon 的 `/relationships`）把觀看者狀態放另一個批次端點，主體保持可快取【猜】。HTTP 規範：依憑證而定的回應必須 `private` 或 `Vary` 憑證 header（RFC 9111 §3.5：帶 `Authorization` 的請求，共享快取預設不可存，除非回應明寫 `public`——而我們正好寫了 `public`）【推】。
- 現有：A7-D4（brief `step10d-a7-social-brief.md:241`【量】）定每則留言帶 `viewer_can_edit/delete/reply`、`is_mine`，貼文帶 `viewer_actor`、`viewer_can_comment`；封鎖語意（Will 2026-10-23）讓列表依觀看者過濾。website 那側已計畫只讓匿名讀進 `unstable_cache`（計畫 A7-4a 段），但伺服器端的 `Cache-Control` 沒跟上。
- 選項：
  - A（建議）：`cachePolicy` 在 ctx 有呼叫者時改回 `private, no-store`，匿名照舊 `public, no-cache`＋ETag；並一律加 `Vary: Authorization, Cookie`（防代理把匿名版本給登入者）。改一處，所有 Public service 受益。失敗條件：某 CDN 對 `Vary: Cookie` 直接不快取（Cloudflare 對帶 cookie 的請求本來就預設不快取 HTML 以外的內容【猜】）→ 只影響登入者的命中率，正確性不變。
  - B：觀看者狀態全部搬到 `SocialAppService` 的批次方法（例如 `BatchGetPostViewerStates`），公開讀取永遠不依觀看者。仿 `followed_store_ids`（`buyer.proto:395`）。失敗條件：封鎖過濾是「列表少幾列」，不是附加欄位，B 做不到——買家封鎖的店的貼文仍會出現在公開列表，違反已決的封鎖語意。所以 B 只能處理欄位，不能處理過濾，仍需要 A。
  - C：依觀看者的讀取全搬進 AppService（`ListFeed` 在 App、`ListPosts` 在 Public）。方法數翻倍、client 依登入狀態切兩支，業界少見【推】。
- 建議：A。B 不採（解不了過濾）。
- 推翻條件：量到登入者的公開讀取占比高且 CDN 命中是成本瓶頸（尚未上線，無此數據）。
- 可逆性：高。約束帳：無 REAL；`httpcache.go` 是我們的碼。
- 這一點改的是共用的 httpapi，規範 §九（:183）要同時加一列；屬於「修共用規範」，**要 Will 點頭**（見 §2 開放項 O1）。

### 1.3 封鎖的組裝放在 handler

- 現有：buyer 與 seller 互不依賴，「買家封鎖自己的店」「店家封鎖店主本人的買家身分」、名單顯示（已關閉買家、已退場店）由 httpapi handler 經 identity 判斷（計畫:414【量】）。
- 基準：業界的封鎖通常歸一個「關係／safety」服務（Mastodon 的 `blocks`/`mutes` 在帳號關係模組、Instagram 的 restrict/block 在 relationship 服務）【猜】。本 repo 把兩種封鎖分給 buyer、seller 兩模組（A7-1b 已完成，計畫進度表【量】）。
- 判斷：在既有模組邊界下，跨模組的「同一人」規則只能放在依賴兩邊的那一層；本 repo 的前例是編排模組（`accountsuspension`、`accountdeletion`，A7-D15）而不是 handler。
- 選項：
  - A（現計畫）：handler 組裝。成本最低；失敗條件：同一規則要被第二個入口（例如 A7-2d 代行、或 A6 的聊天封鎖入口）用到時就會重寫一份。
  - B：新編排模組 `blocking`（或把規則放進 `accountsuspension` 那一層的同類模組），handler 只轉呼叫。失敗條件：只有一個入口時是多一層。
- 建議：A6 會讓聊天、評價也用封鎖（S3、Q1），屆時第二個入口必然出現【推】。**現在先做 A，但把規則寫成 httpapi 內一個單獨的函式＋測試**，A6 開工時若第二入口成立就升成編排模組。這是可逆的小決定，不需 Will，除非他偏好現在就做 B（O4）。
- 約束帳：「buyer、seller 互不依賴」＝SELF-IMPOSED（我們的模組規則），但是正確的規則，不推翻。

### 1.4 `viewer_can_*` 欄位

- 基準：GitHub GraphQL 的 `viewerCanUpdate`／`viewerCanReact`、Reddit 的 `can_mod_post` 都是伺服器算的權限旗標，client 只照顯示【猜】。這是業界標準做法，A7-D4 照做。
- 唯一風險是 1.2 的快取，已處理。另提醒：列表每列的店家封鎖要批次查（計畫:414 已寫 `seller.StoresBlockingBuyer`），否則 N+1。【量：計畫文字】
- 已決（A7-D4），不挑戰。

### 1.5 `RecordPostView` 獨立 POST＋匿名每日 salt 雜湊去重

- 基準：YouTube、Medium、Reddit 的瀏覽數都由獨立的 beacon／事件端點記，不在讀取裡寫【猜】；規範 §四（:92【量】）也明文「讀取不得順便寫入」。匿名去重用每日輪替 salt 的雜湊，是 Plausible、Fathom 這類隱私導向分析工具的做法【猜】。
- 已決（A7-D11、SD18）。不挑戰。
- 補一點設計：`RecordPostView` 放 `SocialPublicService`（匿名也要能叫），屬 Public 的 POST → `private, no-store`（`httpcache.go:22`【量】），A7-D18 的宣告表要標成「自助」或「寫入但停權者靜默略過」——計畫 A7-4 已寫背景寫入收到 `ACCOUNT_SUSPENDED` 靜默略過；但停權者瀏覽不計數是否是想要的語意，計畫沒寫【推】→ 開放項 O3。
- 防灌水：每日一次去重擋住重整，擋不住腳本換 IP／UA 刷匿名瀏覽。業界做法是限流＋異常偵測【猜】；httpapi 已有 `ratelimit.go`【量】，A7-2 只要把這支方法掛上限流即可。

### 1.6 熱度排序放在 `ListPosts`

- 基準：AIP-132 以 `order_by` 欄位支援多種排序；Reddit 的 `/hot`、`/top?t=week` 是同一資源的不同排序【猜】。放在 `ListPosts` 是基準。
- 建議形狀：`order_by` 接受有限的字串（`"publish_time desc"`、`"trending_week"`）或用 enum；keyset 分頁在熱度排序下要以（分數、id）做游標，分數在時間窗內會變，翻頁可能重複或漏【推】。業界的處理是快照分數或接受漂移（Reddit 接受）【猜】。「本週主題」只取前幾筆時，用 AIP 的 `Suggest`／`max_results` 不分頁（規範 :88）更簡單 → 開放項 O2。

### 1.7 賣家中心的動態數

- 現有：計畫只寫「賣家中心的動態數」（:420），計數由 stats 表維護（A7-D11）。
- 基準：Shopify 的店家後台、Instagram Insights 都是作者專屬的讀取方法，回作者看得到的計數（含草稿、被隱藏的）【猜】。
- 建議：`SocialAppService` 一支 `GetStoreSocialSummary`（或掛在 `ListPosts` 的回應），以店家推導身分＋能力宣告。和公開計數「所有人看到同一個數字」不衝突：公開數是貼文上的數字，作者摘要是另一個資源。形狀由 A7-4a 的 `useSellerPosts` 需要的欄位決定（計畫 A7-4a 段）【推】。

### 1.8 `LikeComment`／`UnlikeComment` 對 reaction 資源

- 基準：只有一種反應時，冪等的 like／unlike（或 PUT／DELETE）是主流：Mastodon `favourite`／`unfavourite`、GitHub 的 star PUT／DELETE、Instagram 的 like【猜】。多種表情的 reaction 資源（Slack、GitHub reactions、Discourse reactions 外掛）是產品有多種反應時才需要【猜】。
- 已決（CD1；貼文讚也已是 `LikePost`／`UnlikePost`，`interface-inventory.md` §1 #7【量】）。符合基準，不挑戰。
- 推翻條件：產品要多種反應 → 那時換成 `posts/*/reactions` 子資源，`v1alpha1` 期間可直接改。

### 1.9 OUTPUT_ONLY

- 規範 §五（:108-109【量】）與 `rpc.ApplyUpdateMask` 已自動忽略 OUTPUT_ONLY 欄位；A7-D4 的欄位標 OUTPUT_ONLY 是 AIP-203 基準。不挑戰。

### 1.10 content banner 點擊時經 social 確認貼文存在

- 現有：計畫:424 寫「點擊時經 social 公開 API 確認貼文存在」，驗收是「不存在的貼文 ID 被拒」（:430）。
- 基準：CMS 的推薦位（banner）通常在**寫入時**驗證參照存在（外鍵或跨服務檢查），**讀取時**容忍參照失效（目標被刪就不顯示或導到 404）【猜】。點擊時才查等於每次點擊多一次呼叫，且目標在建立後被刪仍會發生。
- 「點擊時」的字面可能是筆誤或指 admin 建立 banner 時的「點選目標」。驗收「不存在的 ID 被拒」對應的是寫入時檢查【推】。
- 建議：建立／更新 banner 時呼叫 social 模組 API（不是公開 Connect）驗證貼文存在且可公開；讀取 banner 時目標已不可見就不輸出該 banner（或 client 顯示 404）。依賴方向 content → social（計畫:424 已寫加進依賴表）。→ 開放項 O5（請 Will 確認字義）。

### 1.11 錯誤原因

- 規範 §八（:150【量】）：只有 client 要在同一個 code 下分支時才加 `ErrorReason`。計畫只列 `ACCOUNT_SUSPENDED`、`ACCOUNT_HAS_STORE`（:423）。
- 建議 A7-2 開工時逐方法列「client 要分支的失敗」，預期會需要的【推】：封鎖導致不能留言（是否要讓 client 顯示「你被這家店封鎖」？業界多數**不揭露**被封鎖，Instagram、X 對被封鎖者顯示通用錯誤或直接隱藏入口【猜】）、檢舉每日上限（`resource_exhausted`＋`RetryInfo` 已夠，不需新 reason）、檢舉自己（`invalid_argument`）。封鎖不揭露與 `viewer_can_comment=false` 一致：client 依欄位隱藏入口，伺服器拒絕時回通用的 `permission_denied`，不另加 reason。

### 1.12 moderation 獨立 package

- `api/hawkr/moderation/v1alpha1`＋`api/hawkr/admin/moderation/v1alpha1`（計畫:415）。目前 `api/hawkr` 還沒有 moderation 或 social 的 proto（`find api/hawkr -name '*.proto'` 只列到 seller、identity、…、admin/reviewqueue【量】）。
- 基準：Reddit 的 report、Discourse 的 flags 都是獨立於貼文的資源與 API【猜】；本 repo 的模組＝package 慣例（規範 :15）也要求這樣。照做。
- 申覆（appeal）：A7-D9 說案件結案是終態、更正在擁有處分的模組做；DSA 第 20 條要求平台提供內部申訴管道【猜：法條內容憑記憶】。計畫目前沒有 `AppealCase` 類方法【推：未在 A7-2 範圍看到】。A7-D20 已把濫用檢舉人依 DSA 延後，申覆是否同樣延後到法遵檢查，計畫沒寫 → 開放項 O6。若 DSA 適用（平台在歐盟營運才適用，目前是台灣市場【推】），那是 REAL；否則 SELF-IMPOSED。

## 2. 已決 vs 開放

已決（不重議；上面已核對都符合基準）：
- A7-D4 `viewer_*` 欄位（brief:241）——符合基準；但它觸發 1.2 的快取缺陷，缺陷要修，決定本身不改。
- A7-D10 不做留言 streaming。
- A7-D11 計數與 `RecordPostView`、匿名雜湊去重、伺服器熱度排序。
- CD1 `LikeComment`／`UnlikeComment`。
- 封鎖語意（Will 2026-10-23，`block-brief.md`）。
- A7-D18 宣告表、A7-D9 作者讀自己的處分。
- 規範 §二 依使用者類別切 service。

沒有發現與業界基準衝突的已決項。【推】

開放（需要 Will）：
- **O1**：Public GET 有呼叫者時改 `private, no-store`＋`Vary: Authorization, Cookie`（改 `httpcache.go` 與規範 §九）。建議：做，放在 A7-2 第一個子步驟。
- **O2**：「本週主題」用 `ListPosts` 的 `order_by`（分頁、熱度變動時翻頁可能重複），或用不分頁的 `SuggestTrendingPosts`（`max_results`）。建議：後者，若 website 只顯示前幾筆（`lib/api/posts.ts:104-121`，開工時核對）。
- **O3**：停權帳號瀏覽貼文時，`RecordPostView` 要計數（宣告為自助）或靜默不計（宣告為寫入）。建議：計數——瀏覽是事實紀錄不是內容寫入，X、Reddit 停權者可瀏覽【猜】。
- **O4**：封鎖的跨身分規則現在留 handler，或直接做成編排模組。建議：留 handler 的單一函式，A6 第二入口出現時升級。
- **O5**：content banner 的檢查是建立時（建議）還是點擊時（計畫字面）。
- **O6**：申覆（appeal）在 A7 做、延後到法遵檢查，或確認 A7-D9「更正在擁有處分的模組做」即為申覆路徑。建議：延後並寫觸發條件（同 A7-D20 的格式）。

可以不問直接定（實作細節）：方法名以 `interface-inventory.md` §1 擬名為起點並照 AIP 改；`ErrorReason` 逐方法盤點；`RecordPostView` 掛限流；作者摘要方法的形狀。

## 3. 建議拆分（每個一個 session，各自驗收）

依資源的垂直切片：每個子步驟都包含 proto、handler、宣告表、adminperm、`connectServices` 註冊、三端生成、handler 測試與該資源的特性測試。好處是每個 session 結束都是可合併、可驗收的完整功能，審查時看得到「契約＋執行點＋測試」一起；依層拆會讓 proto 先合併卻沒有執行點，測試要到最後才寫，違反「契約先於實作」以外的每條驗證規則。【推】

| 子步驟 | 範圍 | 驗收（每項附 mutation） |
|---|---|---|
| A7-2a 骨架與公開讀取 | O1 的 `cachePolicy` 修正＋規範 §九；`SocialPublicService` 的 `GetPost`、`ListPosts`（含熱度或 O2 的方法）、`ListComments`、hashtag 讀取；`viewer_*` 欄位；`Post` 單篇網址 | 登入者的 Public GET 回 `private`、匿名回 `public`＋ETag（mutation：拿掉呼叫者判斷要失敗）；封鎖店家的登入者列表不含該店貼文；讀取全是 `NO_SIDE_EFFECTS`（descriptor 測試）；`buf lint`、三端生成無 drift |
| A7-2b 互動寫入 | `RecordPostView`（含匿名、限流）、`SharePost`、`LikePost`／`UnlikePost`、`LikeComment`／`UnlikeComment`、留言與討論區的建立／編輯／刪除；A7-D18 宣告 | `RecordPostView` 是 POST（mutation：搬進 `GetPost` 要失敗，計畫:431）；停權者寫入回 `ACCOUNT_SUSPENDED`；被店家封鎖者留言被拒 |
| A7-2c 店主側 | `SocialAppService` 的貼文 CRUD、發布／下架、排程讀取與取消（若不併入 A7-2d）、店家範圍 `ListComments`＋`unreplied`、作者摘要（動態數）；能力宣告與 seller ADR 修訂段 | 能力宣告雙向對照；缺 `CapMarketing` 被拒；`unreplied` 篩選正確 |
| A7-2e 封鎖與作者處分 | buyer、seller 的封鎖方法與名單組裝（1.3）；讀自己的處分、隱藏理由、停權理由（A7-D9，宣告為讀取＋「停權帳號可呼叫」測試）；封鎖的特性測試情境 | 同一人跨身分封鎖被拒；已關閉買家顯示成已刪除；特性測試 Connect driver 通過、刻意差異標 SD |
| A7-2f moderation 與 admin | `moderation/v1alpha1`（送出檢舉回 ID 與收件確認）、`admin/moderation`、`SocialAdminService`（還原隱藏、後台貼文列表的檢舉數批次組裝）、adminperm 權限、content banner（O5） | 越權回 `not_found`；管理員角色缺權限回 `PERMISSION_DENIED`；不存在的貼文 ID 建 banner 被拒 |

順序：2a 先（快取修正是其他公開讀取的前提）；2b、2c、2e 只依賴 2a，可平行；2f 最後（後台貼文列表要用到 2c 的貼文形狀）。A7-2d（代行）依賴 2c 的貼文寫入形狀。

每個子步驟的「完成」以 CI 的 backend monolith 與 api-contract job 結論為證據（本機不起容器的規則，見全域指示）。

## 4. 審查修正與拍板（Will 2026-11-15，O1–O7 全部照建議）

獨立審查 9 項（blocking 0），以下修正取代上文對應段落：

- **O1 判斷依據**：§1.2 寫的「ctx 有沒有呼叫者」不對。停權帳號與無效 token 呼叫 Public 時以訪客執行（`backend/internal/httpapi/session.go:28-29,45-46,65-68`），會被誤判成匿名。改依「請求是否帶憑證」（`rpc.Credential(r)`）：帶憑證回 `private, no-store`；測試涵蓋停權、無效 token、正常登入三種。`Vary: Authorization, Cookie` 能否讓 CDN 分開快取未查證；部署前提：Cloudflare 對帶憑證的請求設 bypass cache 規則，部署時驗證。RFC 9111 §3.5 管的是請求帶 `Authorization` 的回應能否由共用快取儲存。`rpc.go:20` 的註解「Public 不讀身分」與規範矛盾，2a 一併修。
- **O3**：前提改正。停權者呼叫 Public 本來就以訪客執行，`RecordPostView` 計為匿名瀏覽；接受現況，不另寫。
- **O7（新增）**：建立型 POST（留言、檢舉、發文）帶 `request_id`（AIP-155），重送回第一次的結果；同一檢舉人重複檢舉同一目標回原檢舉 ID（與 A7-D20 的去重一致）。
- **拆分**：2f 依賴的是 2a 定的貼文形狀，不是 2c；2a 之後 2b、2c、2e、2f 可平行。代號跳過 2d，因為 A7-2d 已是代行貼文那一步。
- O2 另開不分頁的熱門方法；O4 跨身分封鎖規則放 handler 的單一函式；O5 banner 在建立時驗證、讀取時目標失效不顯示；O6 申覆延後（觸發：平台在 EU 營運，或使用者回報需要申覆管道）。
