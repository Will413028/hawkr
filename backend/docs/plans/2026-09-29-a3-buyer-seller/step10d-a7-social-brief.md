# 10d 身分第 1 刀之後：社群（social）怎麼治本（決策簡報 v4）

**Will 決定（2026-10-17）：全照建議**——P、D1、D2、D3、D4、D5′ 與文案、畫面改動 C1–C10 都核可（「都照建議」）。落地記錄見 A3 計畫步驟 10d 的「社群怎麼治本」一段與相關 ADR 文末修訂。

v4 由沒寫過 v1–v3 的 subagent 審查 v3 之後改寫。下面的 worktree 狀態是**寫簡報當時**的歷史：對照的是 worktree `hawkr-10d3id1`（HEAD `8f7f6ed11`，身分第 1 刀未提交）與 main `524ef1079`；身分第 1 刀之後已照原樣提交（`6b1cf36af`）。
路徑縮寫：`S=backend/services/social-service/internal`、`G=backend/gateway/internal`、`M=backend/internal`（單體）、`P=backend/docs/plans`、`W=website`、`SA=seller-app/lib`、`BA=buyer-app/lib`。
標記：**【量】**＝跑過指令或打開檔案看過；**【推】**＝讀程式推論，沒有實際操作；**【猜】**＝估計。

---

## 結論先講

**根因**：這幾天的問題都在舊 social 上補，而舊 social 在 A7 會整個重寫。「social 要等到 A7 才搬」只是計畫的排序（SELF-IMPOSED），不是外部限制。A7 需要的 identity、buyer、seller 都已經在單體裡，Java 也沒有呼叫 social【量】。

**v4 和 v3 不同的地方**
1. **D5 重切**。social 的檢舉處分現在可以直接停用店主的整個帳號，完全繞過 10e 的措施模型；店家層級的檢舉也和 seller 治理重複。v4 改成：處分店家一律走 seller 治理（措施），後台佇列依 10e 已核准的觸發條件，在 A7 提升到平台層。
2. **D2 效果表更正**。買家之間本來就不能傳訊息；買家封鎖店家時，店家仍要看得到自己貼文下的留言與自己商品的評價。另外補上 buyer-app 兩個既有的入口，以及 CRM「黑名單」這個重複的概念。
3. 補上 A7 漏掉的一個依賴環：social → delegation 的代行稽核，改成發事件解決。社群 REST 是 42 個 operation，不是 40 個。
4. 身分第 1 刀照原樣先提交，重新排程的文件另外提交。

| # | 題目 | 建議 | Will 要做的 |
|---|---|---|---|
| **P** | 繼續補舊 social，還是現在搬 | 先提交身分第 1 刀，**10d 這條線的下一步改做 A7**。3a-②、第 2 刀、D2–D5 都併進 A7，舊 social 不再加改動。10d 的 3b–5 可以在 A7 的空檔做，第 6 刀排在 A7-4 之後 | 核可新排序 |
| **D1** | A7 的社群 API 放哪個 service | `SocialPublicService`、`SocialAppService`、`SocialAdminService`。身分由伺服器依資源判斷；店主的管理也放在 `SocialAppService` | 核可（這會改掉 10-17 核定的 A3 最後一句） |
| **D2** | 封鎖 | 分成兩種。**個人封鎖**由買家發起，存在 buyer 模組；**店家封鎖**由店家發起，存在 seller 模組。從留言封鎖時，client 只送留言的參照。效果見 D2 的表。CRM「黑名單」到 A6 時併進店家封鎖 | 核可模型與效果；核可 C5–C7、C10 |
| **D3** | 停權的店能做什麼 | 新增類的動作擋下，收縮類的動作放行，收縮需要的讀取也放行（停權賣家進得了貼文列表） | 核可；核可 C1 與畫面改動 |
| **D4** | 留言框的身分、刪除、回覆按鈕 | 伺服器在每則留言帶「你能不能編輯、刪除、回覆」，在貼文帶「你以誰的身分、能不能留言」 | 核可；核可 C2–C4 |
| **D5′** | 檢舉與處分放哪裡 | 新的 `moderation` 模組負責收檢舉、隱藏內容、處分買家。**對象是店家的處分一律走 seller 治理**：店家內容被平台下架變成一種措施（「內容下架」），警告、停權沿用既有措施，帳號底下有店時拒絕 BAN。後台佇列在 A7 提升到平台層 | 核可；核可 C9 與「平台下架進入措施與申覆」的流程改動；知會 10e |

**這段期間的缺口（只說一次）**：A7 的 client 刀（A7-4）落地之前，本機環境會有這些缺口：已關閉或停權的店的貼文仍然公開；seller-app 留言選單沒有封鎖；停權賣家進不了貼文列表；店主在公開頁看不到刪別人留言的按鈕；檢舉 BAN 仍可停用店主的帳號。系統從未上線，沒有使用者受影響。

---

## 名詞

- **舊 social**：`backend/services/social-service`，前面接舊 gateway 的 REST，四端用手寫的呼叫。
- **單體**：`backend/internal/modules/*`，對外用 Connect，client 從 `api/` 的 proto 生成。
- **A7**：遷移計畫裡把 social 搬進單體的步驟（`P/2026-09-24-go-modular-monolith.md:207`）。A6 是 review、im、crm，A8 是 notification，A10 是代行編排。
- **legacy 轉接**：單體暫時替還沒搬的舊服務提供 gRPC 或 MQ 訊息，放在 `M/legacy/`，舊服務搬走就刪掉。
- **kind**：身分種類，買家或店家。買家 7 號和店家 7 號是不同的人。
- **措施**：平台對賣家帳號或店家做的決定，目前有警告、停權、終止合作三種，可以申覆（術語表）。由 10e 負責。
- **REAL／SELF-IMPOSED**：REAL＝這一步改不了的限制，例如別的系統、法律或契約、缺資料，或要到排定步驟才會動的外部依賴（現在改就是白做）。SELF-IMPOSED＝我們自己定的分段、改動量、和既有程式一致、工作量。
- **收縮類／新增類**：收縮類讓內容變少，例如下架、刪除；新增類讓內容變多或擴散，例如發布、留言、按讚、編輯。

---

## P. 前提題：繼續補舊 social，還是現在搬

### 白話

3a-①、3a-①b、身分第 1 刀這三刀都做在舊 social 上，後面還排了 3a-②、第 2 刀，以及 v2 的 D3、D4。這些在 A7 全部要重寫。ADR 自己也寫了「舊程式碼的改動在 A7 照本 ADR 重寫，不照搬」（`backend/docs/adr/2026-10-17-social-actor-identity.md`〈後果〉）【量】。遷移計畫也規定「gateway 不再加功能」（`P/2026-09-24-go-modular-monolith.md` A0）【量】。

### 量測【量】

| 項目 | 數字 | 來源 |
|---|---|---|
| 已花在舊 social 的改動（10-15 之後） | 3a-① 51 檔；3a-①b 62 檔；身分第 1 刀 117 個追蹤檔（+2036／−3965） | `git show --shortstat`；`git diff --stat HEAD` |
| social-service Go | 9,379 行，測試 15,313 行，生成碼 3,550 行 | `find … \| xargs wc -l` |
| RPC | 34 支；呼叫方是 gateway（34 支）與 seller-service（`GetPostList`，沒人用） | `rpc-servers.csv`、`rpc-calls.csv` |
| 表 | 10 張 | `schema.sql` 的 `CREATE TABLE` |
| 事件 | 14 列，沒有 Java 消費者 | `events.csv` |
| 舊 REST | **42 個** operation（Post 13、Comment 7、Forum 6、Moderation 5、AdminModeration 4、AdminPost 2、PostHashtag 2、Hashtag 2、CommentRealtime 1），另有代行貼文路由 6 條（`G/router/routes_delegated.go:107-113`） | 依 tag 數 `openapi.yaml` |
| Java 呼叫 social | 0 個 `.java` 檔；`java-proto` 裡有一份沒人用的 `social.proto` | `rg -l "SocialService\|SocialServiceGrpc\|import.*social" backend/services/java -g '*.java'` |
| 對照：A3a buyer | 48 RPC、12 表、40 個 REST，步驟 2–6 在 09-29 到 10-01 完成 | `P/2026-09-29-a3-buyer-seller.md:17-27` |

規模和 buyer 差不多。多出來的部分有：單體第一個 server streaming（即時留言）、封鎖與檢舉的重新設計、D5′ 的措施新種類與平台佇列。**我估 A7 是 buyer 的 2–3 倍工作量【猜】。**

### A7 依賴什麼

| A7 需要的 | 現況【量】 | 現在搬的做法 | 性質 |
|---|---|---|---|
| identity、buyer、seller | 已在單體。舊 social 現在就打單體的 legacy gRPC（`S/../config/config.go` 三個預設 `9941`；`backend/Makefile:254` `LEGACY_GRPC_ADDR = :9941`） | 改成同一個程序內呼叫 | 不是限制 |
| media、product（Java） | 單體已有 `media` 模組、`M/legacy/mediaservice`、`M/legacy/productservice` | 照 A3 的做法：port 加 legacy 實作 | 不是限制 |
| **代行稽核（delegation）** | 排程發布 worker 為代行排程的貼文寫稽核（`S/worker/post_schedule_worker.go:260`）；依賴表是 delegation → social（`M/arch/modules.go:36`） | social 不直接呼叫 delegation（會形成環），改成發事件，由 delegation 訂閱並記稽核 | 不是限制；A7-0 要定案 |
| im、review（A6） | social 只為處置呼叫它們（`S/usecase/dispatcher.go:51,53`）；im、review 讀 social 寫在 Redis 的封鎖集合（`im-service/internal/usecase/message.go:497,809`、`review-service/internal/grpc/handler.go:43`） | 處置由 `moderation` 經 legacy client 呼叫；「是否封鎖」由單體的 legacy gRPC 回答，帶 (kind, id)，im、review 只換掉讀 Redis 那一行 | **REAL**：im、review 要到 A6 才搬，對它們只能做最小改動；但這不推遲 A7 |
| notification-service（A8） | 經 RabbitMQ 消費社群事件 | 單體加 social 的 relay（前例 `M/legacy/sellerrelay`） | **REAL**：A8 之前事件形狀不變 |
| 代行編排（A10） | gateway 經 `G/grpc/delegation_client.go` 呼叫 5 支 social RPC | 單體提供這 5 支的 legacy server（前例 `M/legacy/sellerserver`） | **REAL**：只推向「多 5 支 legacy RPC」 |
| 聊天裡的封鎖（A6） | 兩支 App 的聊天、buyer-app 的評價以舊路由 `/app/blocks/{userId}` 封鎖 | legacy server 接住 3 支舊封鎖 RPC，寫進 D2 的新紀錄 | **REAL**：只推向「legacy server 多 3 支」 |
| 主計畫 A1→A11 的排序 | 單體 ADR D8 只對階段 B 規定順序；階段 A 的順序只是清單 | 改排序 | **SELF-IMPOSED** |
| A3 還沒結束（10d 3b–6、10e 第 3 刀以後、步驟 11） | `P/2026-09-29-a3-buyer-seller.md:297,332,363` | 10d 剩下的刀不依賴 social；10e 碰的是 seller 治理，見「和 10e 配合」 | **SELF-IMPOSED**（協調成本） |

### 業界基準

- **Strangler Fig**（Martin Fowler；Azure 架構中心同名模式）：新功能和模型修正都做在新系統，舊系統只做維持運作需要的改動；越早搬，重工越少。單體 ADR D8 寫的是同一件事（「系統從未上線，沒有資料搬移、沒有雙跑」）。
- **Shopify 的 componentization**：一次搬一個元件，在搬的時候把邊界畫對。
- 反過來的情況是：舊系統要長期上線服務、新系統還很遠，這時才值得先修舊系統。我們兩個條件都不成立。

### 既有機制為什麼存在

A7 排在 A6 之後，是因為依賴表寫 social → im、review，從下往上搬可以少寫過渡程式。但這兩個依賴只來自檢舉處置，D5′ 把處置移出 social 之後，social 就不再依賴 im、review。前幾份簡報把「排定的步驟」當成 REAL，但照定義那是計畫分段。

### 選項

| 選項 | 取捨 |
|---|---|
| (P1) 繼續在舊 social 做 3a-②、第 2 刀、D3、D4 | 每一刀都要做兩次，舊 gateway 繼續變大 |
| **(P2) 提交身分第 1 刀，接著做 A7，3a-②、第 2 刀、D2–D5′、O3 併進 A7** | 舊系統不再加改動，治本的設計直接寫進單體；代價是 A7 期間有頂部列的缺口 |
| (P3) 丟掉身分第 1 刀，直接做 A7 | 第 1 刀已做完、審過、檢查全過，提交的成本接近零。丟掉的話，A7 開發期間本機的舊 social 仍有越權（買家 7 號能改店家 7 號的留言） |
| (P4) 只把讚與留言搬進單體 | actor brief T5 已否決：計數、刪除、曝光過濾會跨兩個系統 |

### 約束帳

| 約束 | 性質 | 會讓建議偏離基準嗎 |
|---|---|---|
| social 排在 A6 之後 | SELF-IMPOSED（計畫分段） | 否，改排序 |
| A7 規模大 | SELF-IMPOSED（改動量） | 否 |
| 和 10e 的協調 | SELF-IMPOSED（工作量） | 否，見執行順序 |
| L′ 共用文案目錄還沒落地 | SELF-IMPOSED（另一個 session 的排程） | 否：基準本來就是共用目錄，A7-4 等它不算偏離 |
| im、review 要到 A6 才搬 | REAL | 只推向「對它們做最小改動」 |
| notification 要到 A8 才搬 | REAL | 只推向「事件經 relay 保持原形」 |
| 代行編排要到 A10 才搬 | REAL | 只推向「單體提供 5 支 legacy RPC」 |
| buyer、store 的 ID 仍是 bigint（Java 還存著） | REAL | 只推向「`actor_id` 先用 bigint」（ADR 已寫觸發條件） |

### 建議：P2

- **終點**：social 是單體的一個模組，對外只有 Connect。以下全部刪除：舊 social-service、gateway 的社群路由與 SSE、`openapi.yaml` 的 42 個 operation、四端手寫的社群呼叫、`pkg/blocklist`、兩份 `social.proto`。
- **觸發**：身分第 1 刀提交之後。
- **3a-②**：併進 A7 的模組本體，在模組裡讀 seller 的曝光狀態。已核准的規則（10d 的 D2、第 3a 刀的切分）照用，只換落地的位置。
- **10d 剩下的刀**：3b–5 不依賴 social，可以在 A7 的空檔做。第 6 刀（website 店面）和 A7-4 都會碰店面的貼文分頁，排在 A7-4 之後，避免同一頁做兩次【推】。

---

## D1. A7 的社群 API 放哪個 service

**問題**：已核准的 A3 最後一句寫「A7 時，店主在自己內容上的動作在 `SellerAppService`，其餘在 `BuyerAppService`」（`step10d-3a-actor-brief.md:96`；主計畫 A7 段）。它和兩個已核准的決定不能同時成立：buyer-app 裡店主在自己的貼文留言要顯示店名（UX 2），以及身分由伺服器推導。照那句話做，buyer-app 呼叫 `BuyerAppService` 在自己的貼文留言，結果只有兩種：被拒，或是 service 名稱說買家、實際卻寫成店家。

**業界基準**（一般知識，沒有逐項查官方文件）：Google AIP、GitHub、YouTube 都依資源分 service，「你能做什麼」由伺服器針對這個資源判斷。YouTube 的頻道主和觀眾用同一個 `comments.insert`。反例是 Shopify 的 Storefront API 對 Admin API（ADR 10-07 引用），但那是兩組不同的資源，不是同一個留言動作依資源決定身分。

**既有機制為什麼存在**：ADR 10-07 為了拿掉 `X-Active-Role`，寫成「服務名稱表示身分」；當時只有 buyer、seller 兩個模組碰到身分。專案的命名規則其實是「模組＋對象」：`<模組>AppService` 給登入的帳號用，買家賣家都算（`API 設計規範（Connect）.md:27`）【量】。

| 選項 | 取捨 |
|---|---|
| **(a) `Social{Public,App,Admin}Service`**，店家身分的動作由 social 推導，再向 seller 檢查能力 | 符合 UX 2、伺服器推導、命名規則；A6 的聊天也是同一個形狀（一段對話裡買家和店家各有身分） |
| (b) 照 A3 那句 | client 要自己判斷「這是我的貼文」才知道打哪個 service，等於 client 在決定身分 |
| (c) `BuyerAppService` 碰到自己的貼文就改成店家 | 名稱和行為不一致 |

| 約束 | 性質 | 偏離基準嗎 |
|---|---|---|
| ADR 10-07「服務名稱表示身分」 | SELF-IMPOSED（可以修訂） | 否 |
| 店家能力宣告只掛在 `SellerAppService` | SELF-IMPOSED（和既有做法一致） | 否：social 的方法照樣宣告「以店家身分時需要的能力」 |

**建議 (a)**，在 A7 的 API 刀實作。要修訂的文件：ADR 10-17 的 I3 段、主計畫 A7 段、ADR 10-07 文末、`API 設計規範（Connect）.md` 第 34 行與 ADR `2026-09-29-seller-suspension-and-business-identity-in-monolith`（能力宣告可以掛在其他模組的 `AppService`，由擁有資料的模組推導身分後向 seller 檢查）。**文案**：無。

---

## D2. 封鎖

**問題**
- 紀錄只有號碼，沒有 kind（`user_blocks(blocker_id, blocked_id)`）【量】，所以店家 7 號封鎖某人，可能連帶擋到買家 7 號【推】。
- 現在一律雙向隱藏（`backend/pkg/blocklist/blocklist.go:89-109`）【量】：店家封鎖買家，結果買家反而看不到這家店。seller-app 的說明文字卻寫成單向（`SA/features/report/presentation/widgets/block_actions.dart:19`）【量】。
- im、review、social 都讀 social 寫在 Redis 的集合。共用 Redis 集合當資料來源，正是 A3 在其他地方拿掉的做法（`P/2026-09-29-a3-buyer-seller.md:39-40`）。
- 現有的入口【量】：seller-app 的聊天與封鎖名單頁；seller-app 留言選單的入口已被身分第 1 刀拿掉；**buyer-app 從貼文封鎖店家**（`BA/features/social/presentation/post_detail_screen.dart:230`、`merchant_posts_feed_screen.dart:566`）、**從商品評價封鎖評價者**（`BA/features/product/presentation/merchant_item_detail_screen.dart:2571`）、聊天、封鎖名單頁。website 沒有封鎖使用者的入口。
- **重複的概念**：seller-app 的 CRM 有「黑名單」分頁（`SA/features/customer/presentation/customer_management_screen.dart:316`，crm 的 `customer_type = 'blocklist'`）。我沒有找到任何依它攔阻的程式【推：rg 只找到標籤】，所以店家以為封鎖了，實際上沒有效果。

**業界基準**（一般知識）
- 個人封鎖（Facebook、Instagram）：雙方都看不到彼此，也不能互傳訊息。
- 頁面或店家封鎖（Facebook 粉專的 Ban、Etsy 店家封鎖會員）：單向。對方不能再留言、按讚、傳訊息，但仍看得到公開內容。
- 封鎖不能拿來擋掉真實的評價（Amazon、Etsy 的評價都不受賣家封鎖影響）；店家一定看得到自己商品的評價，也一定管得到自己頁面上的留言。
- 從留言封鎖時，client 送的是「這則留言」，由伺服器找出作者。

**既有機制為什麼存在**：公司時期只考慮買家封鎖買家。賣家加入後改存店家號碼，卻沒有記 kind，只好用猜的（`S/usecase/block.go:94-115`）。

**放在哪個模組**：im、review、social 都已經依賴 buyer 與 seller（`M/arch/modules.go:30,31,34`）【量】。如果封鎖放在 social，im 要查封鎖就得依賴 social，而 social 現在依賴 im，會形成環。seller 存買家編號不需要依賴 buyer（`seller.reports.reporter_buyer_id` 是前例）。

| 選項 | 取捨 |
|---|---|
| **(a) 個人封鎖放 buyer 模組**（和追蹤 `buyer.seller_subscriptions` 放在一起），**店家封鎖放 seller 模組**（屬於店家設定） | 不用改依賴表；資料跟著發起者；A6 時 im 直接查這兩個模組 |
| (b) 一張表放在 social，帶 (發起者 kind, id, 對象 kind, id) | im、review 查它會形成環 |
| (c) 新增 `block` 模組 | 多一個模組只為兩張小表 |

**效果（v4 更正）**

| 誰封鎖誰 | 名稱 | 效果 |
|---|---|---|
| 買家 → 買家 | 個人封鎖 | 雙方看不到彼此的留言與評價（買家之間本來就沒有聊天） |
| 買家 → 店家 | 個人封鎖 | 買家看不到這家店的貼文與討論區，所以也無法在那裡留言；雙方不能互傳訊息。**店家仍看得到**這位買家留在自己貼文下的既有留言與評價，可以管理、回覆。商品與店面照常顯示【推：電商的交易資訊不因封鎖隱藏】 |
| 店家 → 買家 | 店家封鎖 | 對方不能在這家店的貼文與討論區留言、回覆、按讚，不能傳訊息給店，但仍看得到公開內容。既有留言保留，店家可以自己刪。**評價不受影響**。不含禁止下單（Etsy 有），要做的話放在 B 階段的 ordering |

| 約束 | 性質 | 偏離基準嗎 |
|---|---|---|
| im、review 要到 A6 才搬 | REAL | 只推向「單體以 legacy gRPC 回答是否封鎖」 |
| 聊天與評價的入口送的是對方的內部編號（舊契約） | REAL（A6 才重新設計） | 只推向「legacy server 接住 3 支舊 RPC」 |
| 賣家條款 §12.3：平台保留真實的負面評價 | REAL（契約） | 不偏離；它是店家封鎖不碰評價的依據 |
| Apple 審查指南 1.2：有 UGC 的 App 要能封鎖使用者 | REAL（第三方規範） | 不偏離；入口一定要回來 |
| CRM「黑名單」在 crm-service | REAL（crm 要到 A6 才搬，現在改就是白做） | 只推向「A6 時併入」 |

**建議 (a)＋上表的效果**，在 A7 實作。從留言、討論區封鎖時，client 送留言的參照；buyer-app 從貼文封鎖店家時送店家代碼。seller-app 的留言選單入口在 A7-4 回來；**website 賣家中心的留言選單也建議加同一個入口**。**終點**：A7 落地上述設計；A6 時聊天與評價入口改用對話或評價的參照，CRM「黑名單」併入店家封鎖（CRM 的分頁改成顯示封鎖名單，或拿掉）。**文案**：C5–C7、C10。

---

## D3. 停權的店能做什麼

**問題**
1. 店家身分在自己的內容上，留言與回覆會被擋（`S/usecase/comment.go:182-184`、`forum.go`），但按讚、編輯、刪除不擋【量】。
2. **死結**：停權賣家能下架、刪除（`G/router/routes_post.go:75,78` 掛 `CapContentContract`），卻進不了貼文列表與單篇（同檔 :73、:76 掛 `CapMarketing`；`SA/core/router/route_capabilities.dart:63-66`、`W/features/seller-center/components/pageCapabilities.ts:40` 都要求 `MARKETING`）【量】。按鈕在進不去的頁面裡，等於按不到。

**業界基準**（一般知識）：受限的商家仍能刪除、下架自己的內容，不能新增內容或互動；能做的動作需要的讀取一定要給。專案自己也有同樣的原則：「停權賣家必須留得住自清的手」（`routes_post.go:63-69`），稅務補件也開過「鎖住即死結」的例外（`G/authz/capability.go:55-57`）。

**既有機制為什麼存在**：按讚與編輯從來沒有被逐條分類；列表被鎖，是公司時期 46 號「純唯讀查看也鎖」的裁決。

| 選項 | 取捨 |
|---|---|
| **(a) 新增類擋下、收縮類放行、收縮需要的讀取放行** | 和單體的能力模型同一個原則；死結消失 |
| (b) 只擋建立與回覆 | 停權期間還能改舊留言、按讚 |
| (c) 全部擋，連刪除也擋 | 停權的店連不當內容都清不掉 |

| 約束 | 性質 | 偏離基準嗎 |
|---|---|---|
| 46 號「純唯讀也鎖」 | SELF-IMPOSED（公司時期規範，上線前不受約束） | 否 |

**建議 (a)**，在 A7 實作。social 以店家身分動作時，向 seller 查能力（單體 seller 已有這兩個能力，`M/modules/seller/internal/model/capability.go:26,28`）【量】：
- `CapMarketing`（新增類）：建立、發布、編輯貼文；店家身分的留言、回覆、留言讚、編輯留言、討論區發言與編輯。
- `CapContentContract`（收縮類）：下架、刪除貼文；刪除自己的留言；刪除別人留在自己貼文與討論區的留言；店主讀自己的貼文列表、單篇與留言。
- 停權時不退回本人身分；被拒時回 `SELLER_SUSPENDED`；兩端依能力直接不顯示入口。

ADR 10-17 改成：「停權的店以店家身分在自己的內容上不能留言、回覆、按讚、編輯；可以下架、刪除，並讀取自己的貼文與留言。不退回本人身分。」**文案**：C1、C1′。

---

## D4. 觀看者能做什麼，由伺服器告訴 client

**問題**：身分已經改由伺服器決定，但顯示層還在看「我在哪個畫面」【量，讀程式】：

| 地方 | 現在怎麼決定 | 結果【推】 |
|---|---|---|
| website 留言框頭像、送出瞬間的暫時作者 | 看是不是在賣家中心（`W/features/feed/components/PostComments.tsx`、`W/lib/hooks/useSocial.ts`） | 店主在公開頁留言，先顯示會員身分，伺服器回應後才變成店名 |
| website 刪除按鈕 | 「在賣家中心」或「是我的」（`W/features/feed/components/commentPermissions.ts:17-23`） | 店主在公開頁看不到刪別人留言的按鈕 |
| website、buyer-app 回覆按鈕 | 登入就顯示 | 回覆只限店家（`S/usecase/comment.go:178-181`），其他人按了一定失敗 |
| buyer-app 留言框 | 一律顯示會員頭像，沒有刪除 | 店主送出後才變店名；誰都刪不了自己的留言 |

**業界基準**：GitHub GraphQL 的 `viewerCanUpdate`、`viewerCanDelete`；Facebook Graph API 的 `can_remove`、`can_comment`；YouTube 的 `canReply`。能力放在每個物件上，client 不自己組規則。專案前例：`Violation.can_appeal`、`GetMyStore` 回傳的能力清單。

**既有機制為什麼存在**：以前身分來自「在哪個工作區」，所以 client 這樣判斷是對的。

| 選項 | 取捨 |
|---|---|
| (a) client 自己比對「貼文的店＝我的店」 | 擁有權規則在三端各寫一份 |
| (b) 只在貼文層級回「能不能管理」 | 每則留言各自不同的規則（被隱藏、被封鎖）到時還要再改 |
| **(c) 每則留言帶能力，貼文帶身分** | client 照著顯示，之後新增規則只改伺服器 |

**建議 (c)**，寫進 A7 的 proto（OUTPUT_ONLY）：每則留言、討論區發言帶 `viewer_can_edit`、`viewer_can_delete`、`viewer_can_reply`，以及 `is_mine`；貼文、討論區帶 `viewer_actor`（kind、名稱、頭像）與 `viewer_can_comment`。即時留言改成 server streaming 後，同樣每條連線各算一次。
- **直接做**（跟隨已核准的 UX 2、修 bug）：留言框顯示店名；非店主不再看到回覆按鈕。
- **要同意**：C2、C3、C4（新入口）。buyer-app 不加「編輯」。

---

## D5′. 檢舉與處分放哪裡（v4 重切）

**問題**
1. social 依賴 im、review 的唯一原因是檢舉處置（`S/usecase/dispatcher.go:51,53`）【量】。檢舉的對象涵蓋評價、貼文、留言、討論區、聊天訊息、使用者（`schema.sql:94`），範圍是全平台的內容，不只是社群。
2. **處分店家時繞過措施**：處置結果有 `WARN_USER`、`BAN_USER`（`schema.sql:100`）【量】。`BAN_USER` 會把被檢舉的店家換算成店主帳號，再呼叫 identity 的 `SuspendAccount`（`S/usecase/dispatcher.go:118-147`；單體 `M/legacy/accountservice/server.go:140`）【量】，整個登入就被停用，沒有措施、沒有給賣家的理由、也不能申覆。但 seller 模組的不變量是「措施是效果的唯一來源」（`M/modules/seller/AGENTS.md`），賣家條款 §18.4 也規定「下架、停權…得申覆」（`website/docs/legal/seller-terms.md:184`）【量】。
3. **店家層級的檢舉重複**：social 的 `USER` 檢舉加上 `FOOD_SAFETY`、`FRAUD` 原因，和 seller 的通報（`seller.reports`，類別同樣有 `FOOD_SAFETY`、`FRAUD`，還接 FDA 轉報；`M/modules/seller/db/schema.sql:211-238`）【量】是同一件事。10d 的 D4 已經決定店面頁的檢舉直接送到 seller 治理。
4. **已核准的觸發條件**：10e 第 3 刀 D7（Will 2026-10-16）寫明「social 搬進單體的那一步，就是後台待辦佇列實體化的觸發條件：佇列移到平台層，由各模組的事件投影」（`step10e-3-brief.md:178-179`）【量】。
5. **10e 的前例**：平台強制下架賣家的商品是一種措施，只是因為執行者在 Java 才排到 B4（`step10e-measure-brief.md` §7）【量】。

**業界基準**（一般知識，沒有逐項查官方文件）
- 收檢舉、排佇列是跨產品的一套系統（Meta、YouTube：同一個佇列處理貼文、留言、訊息、評價）。被處理的內容由各自的擁有者提供「隱藏」動作。DSA 第 16 條的通知與處理機制也是平台層級。
- **處分是記在被處分的對象上**。商家看到的是同一個執法頁面：Amazon Seller Central 的 Account Health 把商品下架、政策違規、停權放在一起，每一筆都可以申訴；Meta 的帳號狀態頁也把內容移除與帳號限制放在一起。DSA 第 17 條（處理理由，含下架與降低曝光）、第 20 條（內部申訴），以及 P2B 第 4 條（對商家的限制要附理由、可申訴）。
- 商家治理和一般使用者的內容審核是不同的 bounded context（專案 ADR 2026-09-18 D2 也這樣判），但兩者要銜接：內容審核發現商家違規時，交給商家的執法流程。

**既有機制為什麼存在**：ADR 2026-09-18 D1 把三套半的檢舉路徑收斂到 social 既有的表，當時是微服務，放在已經有表的服務最省事。D3 讓 BAN_USER 變成真正的帳號停權，那時還沒有措施模型。

| 選項 | 取捨 |
|---|---|
| (a) v3：獨立的 `moderation` 模組，可以對任何人警告或 BAN，有自己的後台佇列 | 照搬「BAN 繞過措施」；店家層級的檢舉兩套；後台出現第二個佇列，和 10e D7 的終點牴觸 |
| **(b) `moderation` 負責收件、隱藏內容、處分買家；對象是店家的處分一律交給 seller 治理；後台佇列提升到平台層** | 店家只有一個執法頁面（措施與申覆），每一筆都有理由、可以申覆；買家的處分仍有自己的地方；要改 10e 擁有的 schema 與術語表 |
| (c) 全部併進 seller 治理 | 買家不是商家；ADR 2026-09-18 D2 與 10e 都判成不同的 context |

**(b) 的具體規則**
- **收件**：`moderation` 收貼文、留言、討論區、評價、聊天訊息與買家的檢舉。檢舉對象是店家本身的，直接建立 seller 的通報，不在 `moderation` 另存一份。
- **內容的處理**：隱藏內容由 `moderation` 呼叫擁有者的 API（social 在單體內；im、review 在 A6 之前經 legacy client）。內容作者是**買家**時，處理紀錄留在 `moderation`。作者是**店家**時（例如店家的貼文、店家身分的留言），處理方式是建立措施，新種類叫「內容下架」：依據是認定，認定的來源是這筆檢舉。seller 發出措施事件，social 訂閱後隱藏對應的內容。賣家在「措施與申覆」看得到這一筆，也可以申覆。
- **警告、停權店家**：一律使用既有的措施，`moderation` 不提供 `WARN_USER`、`BAN_USER` 給店家。
- **停權買家**：沿用 identity 的 `SuspendAccount`。但**帳號底下有店（stint 未關閉）時拒絕**，理由是停用登入就等於停權這家店，必須改走措施（賣家條款 §18.4）。
- **後台佇列**：依 10e D7，在 A7 實體化成平台層的工作項目，由 seller 與 `moderation` 的事件投影；API 形狀照 10e 第 3d 刀的 `ListReviewQueue`。如果 10e 第 3d 刀還沒落地，就直接用平台層的形狀做，不先做 seller 內部的版本【推：避免做兩次】。
- **依賴表**：social 拿掉 im、review；新增 `"moderation": {"social", "im", "review", "identity", "buyer", "seller"}`；notification 加上 moderation。seller 不依賴 social（social 訂閱 seller 的事件，方向正確）。

| 約束 | 性質 | 偏離基準嗎 |
|---|---|---|
| ADR 2026-09-18 把檢舉放在 social | SELF-IMPOSED（微服務時期的放法） | 否，以新 ADR 取代該部分 |
| im、review 要到 A6 才搬 | REAL | 只推向「隱藏訊息、評價走 legacy client」 |
| 賣家條款 §18.4（下架、停權可以申覆） | REAL（契約） | 不偏離；它要求店家的處分走措施 |
| 10e 擁有措施 schema 與術語表，而且正在開發 | SELF-IMPOSED（協調） | 否：A7-1 動 `measures_kind_check` 與術語表之前先知會 10e |
| 沒有買家條款，買家對內容處理的申訴規則不存在 | REAL（缺資料） | 只影響「買家申訴」這一項，見文末「還不能定的」 |

**建議 (b)**。**終點**：檢舉只有一個入口與一個後台佇列；店家的所有處分都是措施；social 不再依賴 im、review。**觸發**：A7。要寫一份新 ADR 取代 2026-09-18 的 D1、D3（D2「商家治理不併入共用底層」不變，這裡做的是銜接，不是合併）。**文案**：C9；畫面改動是「店家貼文被平台下架後，出現在措施與申覆」。

---

## 執行順序

每一刀照 A3 的慣例：一個 session、code-review 加 design-review、跑和 CI 相同的檢查。

| 刀 | 內容 | 刪掉什麼 |
|---|---|---|
| **0a 提交身分第 1 刀**（照原樣） | worktree 合併 main：只落後 `524ef1079`（純文件），只和 A3 計畫一個檔重疊，不必重新生成或重跑契約檢查。照審查過的內容提交 | — |
| **0b 文件**（Will 核可本簡報之後，另一個 commit） | 主計畫：A7 提前、刪掉過期的「seller 認證申請的 port」；A3 計畫：3a-②、第 2 刀改指向 A7，第 6 刀排在 A7-4 之後；ADR 10-17 依 D1–D3、D5′ 修訂；ADR 10-07、API 規範第 34 行修訂；actor brief 約束帳的「Java 消費者仍在」、O3 的「REAL」更正 | 舊系統上的 3a-② 與第 2 刀 |
| **A7-0 盤點**（唯讀） | 34 RPC、42＋6 個 REST、1 條 SSE、14 列事件、10 張表、四端呼叫點；代行稽核改成事件；和 10e 對齊措施新種類與平台佇列；寫 `P/2026-10-xx-a7-social.md` | — |
| **A7-1 模組本體** | social（貼文、留言、讚帶 `actor_kind`、討論區、hashtag、計數、曝光、停權能力、etag、UUIDv7）；buyer 的個人封鎖、seller 的店家封鎖；`moderation`；seller 的「內容下架」措施；平台佇列 | — |
| **A7-2 對外 API** | social、moderation 的 Public／App／Admin；buyer、seller 的封鎖方法；D4 的欄位；即時留言 server streaming。**動 `buyer.proto`、`seller.proto` 前先知會 10e** | — |
| **A7-3 legacy 轉接** | social 事件的 relay；legacy server（代行 5 支、封鎖 3 支、「是否封鎖」查詢）；im、review 換掉讀 Redis 那一行；處置的 legacy client | social-service 的 MQ 消費者 |
| **A7-4 client** | 四端改用生成的 client；D2–D5′ 的畫面與文案；O3 的店主留言讀取；收件匣先出 UI 稿 | 四端的社群手寫呼叫；gateway 的社群路由、SSE、`grpc/social_client.go`；`openapi.yaml` 的 42 個 operation；3a-① 的例外路由 |
| **A7-5 收尾** | CSV 去向、模組完成的定義、獨立設計審查 | `social-service`、兩份 `social.proto`、`pkg/blocklist` |

**和 10e 配合**
- 10e 在改 seller 治理、admin、L′ 共用文案目錄。A7-1 要動 `seller.measures`（新種類）與術語表，A7-2 要動 `seller.proto`、admin 的治理介面，動之前都要先知會。誰後落地，誰就負責重新生成並重算 `maxRESTOperations`。
- A7 的新文案寫進 L′ 的共用文案目錄。L′-1 還沒落地的話，A7-4 等它；A7-0 到 A7-3 不受影響。
- 平台佇列和 10e 第 3d 刀是同一件事，A7-0 時一起定先後。

---

## 要核可的文案與畫面改動

用語依 `使用者面用語術語表.md`。各端維持自己的「貼文／動態」用字。英文之後再補。

| # | 位置 | 文字 | 性質 |
|---|---|---|---|
| C1 | 停權時，seller-app 貼文列表上方 | 「店家目前受平台措施限制，貼文只能下架或刪除。」 | 新文案＋**畫面改動**（停權時進得了列表；發布、編輯、新增停用） |
| C1′ | 停權時，website 賣家中心動態列表上方 | 「店家目前受平台措施限制，動態只能下架或刪除。」 | 同上。不寫「停權」：client 只拿到 `suspended`，終止合作在申覆期間也是這個狀態 |
| C2 | 以店家身分留言時，website 與 buyer-app 的留言框提示 | 「以「{店名}」身分留言…」 | 新文案 |
| C3 | 以店家身分回覆時的提示 | 「以「{店名}」身分回覆…」 | 新文案 |
| C4 | 店主在 website 公開頁與 buyer-app 刪除別人的留言；buyer-app 刪除自己的留言 | website 沿用「刪除這則留言？」；buyer-app 沿用「刪除留言」「確定要刪除這則留言嗎？」 | **新入口**，沒有新字串 |
| C5 | seller-app 留言、討論區選單恢復封鎖；website 賣家中心留言選單新增 | 「封鎖此人」、「封鎖「{名稱}」？」（既有字串） | seller-app 是恢復；**website 是新入口** |
| C6 | 店家封鎖的確認說明 | 「封鎖後，對方將無法在你的貼文與討論區留言、按讚，也無法傳送訊息給你；對方仍看得到你公開的內容，評價不受影響。你可以隨時在「封鎖名單」解除封鎖。」 | 改既有文案（現行：「封鎖後，對方將無法傳送訊息或留言給你；你可以隨時在「封鎖名單」解除封鎖。」） |
| C7a | buyer-app 封鎖**店家**的確認說明 | 「封鎖後，您將看不到這家店的貼文與討論區，雙方也無法互傳訊息。可隨時在「封鎖名單」解除。」 | 改既有文案（現行：「封鎖後對方將無法傳送訊息給您，您也無法傳送訊息給對方。可隨時在「封鎖名單」解除。」） |
| C7b | buyer-app 封鎖**買家**的確認說明 | 「封鎖後，您與對方將看不到彼此的留言與評價。可隨時在「封鎖名單」解除。」 | 新文案（買家之間沒有聊天，現行的訊息說明不適用） |
| C8 | 店主看自己不公開、但有留言的貼文（seller-app） | 店家不能公開：「店家目前不公開，買家看不到這些留言。」；自己下架：「這則貼文已下架，買家看不到這些留言。」；平台下架沿用「這則貼文已被平台下架，買家看不到留言。」 | 新文案＋**畫面改動**（不公開時也讀得到、刪得了留言） |
| C8′ | 同上（website） | 「店家目前不公開，買家看不到這些留言。」；「這則動態已下架，買家看不到這些留言。」；平台下架沿用「這則動態已被平台下架，買家看不到留言。」 | 同上 |
| C9 | 措施的新種類名稱（賣家端措施頁、通知、admin） | 「內容下架」（en content removal） | 新名詞，先加進術語表（10e 擁有），再進文案目錄。**畫面改動**：店家貼文被平台下架後，出現在「措施與申覆」，可以申覆 |
| C10 | seller-app CRM 的「黑名單」分頁（A6 才做） | 改用「封鎖名單」，內容就是店家封鎖 | 先決定方向，A6 時實作 |
| — | 停權、封鎖被拒 | 沿用 `SELLER_SUSPENDED` 的既有處理 | 入口依能力隱藏 |

收件匣（跨貼文的留言列表）在 A7-4 前另外出 UI 稿，不在本輪核可。

---

## 還不能定的（需要 Will 另外決定，或缺資料）

1. **買家對內容處理的申訴**：業界基準是 DSA 第 20 條，被處理的人可以申訴一次。專案沒有買家條款（`website/docs/legal/` 只有賣家文件）【量】，申訴規則沒有依據。建議 A7 先做「通知作者並附理由」（DSA 第 17 條，伺服器產生），申訴等買家條款寫好之後再加。這是缺資料造成的延後，不是工作量。
2. **聊天與訂單**：店家封鎖買家後，訂單相關的聯絡是否例外，屬於 A6 im 的設計，這輪不定。
3. **措施新種類的細節**（內容下架的期間、能不能「補正後請求解除」）：要和 10e 一起定，A7-0 時提出。

---

## 量了什麼、推了什麼

- **量過**：P 的所有數字與 A7 依賴表的現況欄；Java 沒有呼叫 social；代行稽核的依賴環；BAN_USER 的路徑；seller 通報的類別；10e D7 的觸發條件；D2 的 schema、入口與 CRM 標籤；D3 的路由與能力；C4–C7 的現行字串；worktree 只落後一個文件 commit。
- **推論**：D4 表的「結果」欄；D2 的撞號後果；CRM「黑名單」沒有效果（只用 rg 找過，沒有實際操作）；第 6 刀與 A7-4 重疊；平台佇列直接做成平台層可以避免做兩次。都沒有實際操作三端或打 API。
- **估計**：A7 是 buyer 的 2–3 倍工作量。
- **一般知識，沒有查官方文件**：Fowler／Azure、Shopify、Google AIP、GitHub、YouTube、Facebook、Instagram、Etsy、Amazon Account Health、Meta 帳號狀態、DSA 第 16、17、20 條、P2B 第 4 條。

## 附錄：查證指令（worktree，除非另外註明）

```
python3 讀 backend/api/openapi/openapi.yaml，依 tags（含 Hashtag）與路徑比對社群 operation   # 42
awk -F, '$2=="social-service"{print $1}' …/rpc-calls.csv | sort | uniq -c                       # gateway 34、seller-service 1
grep -i social …/rpc-calls.csv | grep '^social-service'                                         # 含 DelegationService、SuspendAccount
rg -n 'RecordAuditIntent\(' backend/services/social-service/internal                             # post_schedule_worker.go:260
sed -n 85,150p backend/services/social-service/internal/usecase/dispatcher.go                   # BanUser → SuspendAccount
rg -n 'SuspendAccount' backend/internal/legacy/accountservice/server.go                          # :140
sed -n 205,240p <main>/backend/internal/modules/seller/db/schema.sql                             # seller.reports 類別
sed -n 175,180p <main>/backend/docs/plans/2026-09-29-a3-buyer-seller/step10e-3-brief.md         # 佇列的觸發條件
rg -n "'blocklist'" seller-app/lib                                                               # CRM 黑名單
rg -n 'blockUserId' buyer-app/lib                                                                # buyer-app 的兩個入口
rg -n 'CREATE TABLE|buyer_id|store_id' backend/services/im-service/db/schema/schema.sql          # 聊天只有買家↔店家
git -C <main> merge-base --is-ancestor 44e94522b 8f7f6ed11; git -C <main> diff --stat 8f7f6ed11 main
```
