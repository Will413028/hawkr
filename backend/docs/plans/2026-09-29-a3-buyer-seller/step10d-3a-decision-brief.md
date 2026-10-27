# 10d 第 3a 刀（social 曝光過濾）決定題 v2

> Will 2026-10-16 全照建議核定（1a、1b 含文案、7b）；結論記在計畫 `2026-09-29-a3-buyer-seller.md` 的「第 3a 刀開工的決定」。

HEAD `ca2949edf`（main，`local/10e` 第 2c-3 刀已合入；v1 寫於 `17f0c3c0b`，本版引用的行號都在新 HEAD 重查過）。路徑縮寫：`S=backend/services/social-service`、`G=backend/gateway/internal`、`W=website`、`SA=seller-app/lib`、`BA=buyer-app/lib`。
SSOT：`step10d-decision-brief.md` D2、D6（Will 2026-10-15 核准）；ADR `2026-10-05-store-exposure-to-legacy.md`（含勘誤）；ADR `2026-09-26-external-contract-protobuf-connect.md` D5、D6；計畫 `2026-09-29-a3-buyer-seller.md`:307；根 `AGENTS.md`。v1 的審查紀錄沒有保存。

## 要 Will 決定的（3 題）

| # | 題目 | 建議 |
|---|---|---|
| **1a** | 店主讀單篇貼文：新增 REST `GET /app/sellers/posts/:id`，是 ADR 2026-09-26 D5「端點數只准變少」的例外 | **(A) 核准例外**，寫進 ADR 2026-09-26 文末修訂，A7 隨 social 的 REST 一起刪。`maxRESTOperations` 492 → 493 |
| **1b** | 店主畫面（seller-app 與 website 賣家中心）在貼文此刻不公開時的留言區 | **(B) 不呼叫公開路由，顯示空狀態**；文案見 1b |
| **7b** | 追蹤者推播改成「第一次發布時推」（建立即發布、手動發布、排程發布） | **核准，在 3a-② 直接做（取代 7a）**；不需要新欄位 |

其餘照 SSOT 直接做（知會）：2、3、4、5、6、8、9、10，以及「明確不過濾的讀取」。

**急迫性**：10e 第 2c-3 刀已經在 main（`ca2949edf`）。關店不再發 `seller.deleted`（改由保留期清理的 `StoreRetired` 發，`backend/internal/legacy/sellerrelay/relay.go:51`），所以**現在 main 上已關閉店家的貼文會一直留在公開 feed，直到 3a-② 上線**（或保留期滿被清理）。3a-② 應該排在 10d 其餘各刀之前。

**v1 的更正**（詳見審查紀錄）
- 1a 的成本依據改成 ADR 2026-09-26 D5，不是「棘輪指標」；(D) Connect 有單體呼叫 legacy review 的先例，改列為真選項。
- 1a 的店主分支回 NotFound，代行分支維持 `ResourceNotOwned`（gateway 依它寫稽核，`G/handler/admin_delegated_post.go:197-199`）。
- 1b 漏了 website 賣家中心的留言 modal 與 SSE，也漏了「店家可公開、但這篇被下架」的情況。
- 第 4 題漏了統一檢舉 `SubmitReport`。
- 第 6 題改成每次翻轉送一個 `revalidate.SellerTag`；website 沒有逐篇貼文或貼文 hashtag 的快取（v1 引用的 `cached.ts:602` 是商品的 hashtag）。
- 7b 不需要「已推過」欄位：`published_at` 下架不清，轉換前 `published_at IS NULL` 就是第一次發布。
- 第 9 題要連同 `comment_count`、`reply_count` 與對帳 SQL 一起改。

---

## 1a. 店主讀單篇貼文（要 Will）

**現況**
- gateway 的 `/app/sellers` 群組只有 POST、PUT、DELETE `/posts/:id`、GET `/me/posts`、publish、unpublish（`G/router/routes_post.go:70-77`）。唯一的單篇非公開讀取是代行的 `GET …/sellers/:store_id/posts/:post_id`（`G/router/routes_delegated.go:110`），店主不能用。`ListMyPosts` 沒有 id 篩選（`S/db/queries/posts.sql:24-50`）。
- website 編輯頁與賣家中心的深連結退路讀公開詳情（`W/features/seller-center/posts/PostEditClient.tsx:11-15,27`、`SellerPostsClient.tsx:240`）。能用，是因為公開詳情不檢查發布狀態與 `hidden_at`（`S/internal/grpc/handler.go:189-252`，`GetPost` 只看 `deleted_at`）。
- 第 3 題補上這個缺口之後，店主就不能編輯：代行留下的草稿、自己下架的、被平台下架的，以及店家不能公開時的**全部**貼文。賣家自己建立的貼文一律是已發布（`S/internal/usecase/post.go:110-113`），所以草稿只來自代行。D2 承諾「店家可以在公開前先寫貼文」，這會直接違反。
- 店主語意已經存在：`GetPostDetailRequest.owner_store_id`（`backend/api/proto/social/social.proto:195-211`），代行路徑對歸屬的貼文回所有發布狀態（`handler.go:209-226`）。`delegation.Validate` 在沒有 delegation 時直接放行（`backend/pkg/delegation/validate.go:176-179`）。

**業界基準**：管理面與公開面分開，管理面以 Get 讀單一資源。Shopify Admin API 的 `product(id)` 回草稿，Storefront API 只回已發布；WordPress REST 讀草稿要 `context=edit` 與 `edit_posts`；Google AIP-131 用 Get，不用「以 id 篩選的 List」代替。這和傳輸方式無關。

**既有機制為什麼存在**：公司時期公開詳情不檢查發布狀態，賣家中心就借用它；gateway 的店主路由只做了寫入與列表。

**選項**

| 選項 | 取捨 |
|---|---|
| (A) **REST `GET /app/sellers/posts/:id`**（`CapMarketing`，同 `ListMyPosts`）：gateway 帶 `owner_store_id = CurrentStoreID` 呼叫 `GetPostDetail`；social 在沒有 delegation 且 `owner_store_id != 0` 時走店主分支：非歸屬回 NotFound（不洩漏存在），不加 view count；代行分支照舊回 `ResourceNotOwned` | 與同一族的 CRUD 一起在 A7 刪；改動小。違反 ADR D5，要核准例外 |
| (D) **Connect 店主方法**：`api/` 新增 social 的店主 service 與 Post 訊息，單體以新的 legacy social client 呼叫 `GetPostDetail`（先例：`backend/internal/legacy/reviewservice`） | 符合 ADR D5／D6，契約在 A7 留下。但要在 A7 之前先設計 social 的 Post 資源；單體要新增 social 的 legacy 生成、連線設定與部署設定；`make connect` 四端；同一個資源讀在 Connect、寫在 REST |
| (B) `GET /app/sellers/me/posts?post_id=` | 不加路由，但以 List 冒充 Get（AIP-131 的反模式），回列表形狀，SQL 與 openapi 一樣要改，A7 一樣刪 |
| (C) 編輯頁從列表快取取資料 | 重新整理與深連結失效（`PostEditClient.tsx:15` 寫明 refresh-safe 是需求） |
| (E) 不補第 3 題的缺口 | 違反 D2／D6，草稿以 id 對外可讀 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| social 到 A7 才進單體；現在設計它的 Post 契約，A7 會連同整個資源（發布狀態、排程、下架原因、媒體）重新設計 | REAL（邊界外的依賴，排定在 A7；現在做是丟棄的工作）。排除 (D) | 主計畫 A7；D6 約束帳第 4 列 |
| 端點數只准變少；新的對外 API 用 Connect | SELF-IMPOSED（自己的 ADR，可以修訂）。(A) 要寫明例外 | ADR 2026-09-26 D5、D6；`openapi_coverage_test.go:171` |
| D6 選項 (b)「不新增任何路由」 | SELF-IMPOSED：D6 的盤點沒看到編輯頁依賴公開詳情 | D6 選項表 |
| 編輯頁要 refresh-safe | REAL（既有需求）。排除 (C) | `PostEditClient.tsx:15` |

**建議 (A)，並核准 ADR 2026-09-26 D5 的例外**：例外的條件寫成「legacy 服務在搬遷步驟之前，因修正可見性缺口而必須補的店主讀取，與同一族的路由一起刪除」。和 D6「討論區沒有店主端點」的差別：迴響、留言在店家不能公開時可以顯示空狀態，沒有功能損失；編輯沒有可接受的退化狀態。
**終點與觸發**：A7 social 模組提供店主的 Connect Get（或 AIP-157 的 view），這條 REST 與 ADR 例外一起刪。
**`maxRESTOperations`**：+1。

## 1b. 店主畫面的留言區（要 Will：範圍與文案）

**現況**
- seller-app：`listCommentsRaw`（`SA/features/post/data/post_repository.dart:353`）、回覆（`:439`），呼叫點 `SA/shared/widgets/comments_bottom_sheet.dart:133`、`SA/features/post/presentation/page_home_feed_screen.dart:104`。店主列表不篩狀態，包含草稿、已下架、平台下架（`post_repository.dart:206-223`）。
- website：賣家中心的貼文 modal 以 `<PostComments postId canModerate />` 讀公開留言、回覆與 SSE（`W/features/seller-center/posts/SellerPostDetailModal.tsx:178` → `W/lib/social/repo.ts:141,156`、`W/lib/sse/useCommentStream.ts:222`）。列表同樣含平台下架的貼文。
- 第 2 題之後，只要**這篇貼文此刻不公開**（店家不能公開，或貼文是草稿、已下架、平台下架），這些呼叫都會回 NOT_FOUND。受影響的是不公開之前已經有的留言。

**業界基準**：店主從管理面讀自己內容的全部留言（YouTube Studio 對私人影片也能管理留言）。也就是店主留言端點。

**既有機制為什麼存在**：公開留言列表不看父貼文，店主就借用它。

**選項**

| 選項 | 取捨 |
|---|---|
| (A) 店主留言與回覆列表（2 條 REST） | 業界基準；+2 路由，A7 刪；D6 對迴響以同一個 REAL 約束否決過 |
| (B) **兩端以同一條規則判斷「這篇此刻公開」**（店家 `PUBLIC_LISTING` ∧ 已發布 ∧ 未被平台下架），不公開時留言區顯示說明、不呼叫公開留言與 SSE | 與 D6 迴響同一個規則；店主暫時看不到舊留言（買家也看不到）。可公開但事件還沒到 social 的那一段，會落到既有的錯誤畫面 |
| (C) 公開留言路由加店主例外 | 違反 D2／D6（公開路由不依呼叫者變化） |

**約束帳**：「social 沒有店主讀取端點，新增是 A7 前丟棄的工作」——REAL，Will 已在 D6 對迴響判定。UI 文案要同意——REAL（根 `AGENTS.md`「UI 改動」）。
**建議 (B)**，文案（比照 D6 已核准的「店家公開後，買家的迴響會顯示在這裡。」，seller-app 用「貼文」、website 用「動態」）：
- 店家不能公開：「店家公開後，買家的留言會顯示在這裡。」（兩端相同）
- 草稿或自己下架：seller-app「貼文發布後，買家的留言會顯示在這裡。」；website「動態發布後，買家的留言會顯示在這裡。」
- 平台下架：seller-app「這則貼文已被平台下架，買家看不到留言。」；website 把「貼文」換成「動態」。

D2 的「店家在自己貼文底下的回覆照舊」不動：這些情況下沒有新的買家留言可回。
**終點與觸發**：A7 social 模組的店主留言方法。**`maxRESTOperations`**：0。

## 2. 留言列表與回覆跟著父貼文（照 D2）

`GetCommentList`（`S/internal/grpc/handler.go:723`）以 `post_id`／`parent_id` 直接查（`S/db/queries/comments.sql:4-40`），不看父貼文。基準：父資源看不到，子集合回 not found（AIP-211、AIP-193；D2 業界基準第 3 點）。做法：列表前以公開條件確認父貼文（回覆經 `GetComment(parent).post_id`）。這是檢查一個父資源，不是逐列過濾，不影響 `comment_count`。SSE 經 `GetPostDetail` 預檢（`G/handler/sse_stream.go:52-60`），第 3 題之後自動跟上；**已知限制**：已經開著的連線不會因翻轉而關閉（第 4 題之後只剩店主的回覆會送出），不另做。

## 3. 公開詳情（照 D2／D6）

新增 `GetPublicPost`（`deleted_at IS NULL AND hidden_at IS NULL AND publish_status = published AND 店家可曝光`），公開詳情、分享網址（`G/handler/share.go:71`）、SSE 預檢、第 2 題、第 4 題都用它。`GetPost` 不改（留言、檢舉、隱藏、代行都拿它當存在檢查，`handler.go:209-215`）。條件放在一個 SQL view，以整合測試列出所有公開 query（實作細節）。**必須在 1a 之後上**。

## 4. 寫入：不公開的貼文上的互動（照 D2 R1）

一律以公開條件檢查，不公開回 `ErrNotFound`：留言（`S/internal/usecase/comment.go:125-128`）、按讚（`post.go:511-515`，也不推 `post.liked`）、分享（`post.go:499-508`，現在連存在都不查）、`ReportPost`（`post.go:675-682`）、留言按讚與編輯（經留言 id 找到貼文）、**統一檢舉 `SubmitReport`**（`S/internal/usecase/report.go:93-119`，現在不檢查目標；POST 看貼文、POST_COMMENT 看所屬貼文、SELLER_FORUM 看店家可曝光）。
例外（收縮型，46 §3.3）：作者刪除自己的留言；店主對自己貼文的回覆、刪留言、編輯、下架。錯誤碼沿用 `ErrNotFound`。

## 5. hashtag 計數（D2 已決定）

刪 `cms_hashtag.post_count` 與維護程式（`AdjustHashtagPostCount`、`HashtagUseCase.Apply` 的計數部分、`AdjustCountersForVisibilityChange`、`S/internal/worker/post_schedule_worker.go:184` 一帶）；`SearchHashtagsByPrefix`／`ByContain`（`hashtag.sql:53-65`）改成「名稱候選 × 公開貼文計數」，熱門（`:67-79`）加曝光條件。先例：ADR 2026-10-05 勘誤（product-service V21）。v1 的「`bustPost` 改成 bust 全部 hashtag」刪掉：website 沒有貼文 hashtag 的快取。**終點與觸發**：A7，或 hashtag 查詢 p95 > 200 ms 時物化。

## 6. 快取失效（照 Java 先例）

seller 在 transaction 內排入的 website 失效（`backend/internal/modules/seller/internal/stint/stint.go:300-304`）與 social 收到事件沒有先後保證，對帳補發的翻轉（`exposure.go:167`）也沒有人 bust。做法：consumer 寫入（含同版本重送）之後，送**一個** `revalidate.SellerTag(storeID)`（website 展開成 sellers、products、posts，`W/lib/api/revalidate-request.ts:79-83,133`）；舊版本丟棄時不送。website 沒有逐篇貼文的快取（詳情 `no-store`，列表只掛 `posts`），所以不必列舉該店的貼文。先例：Java `StoreExposureService.apply`。

## 7. 追蹤者推播（7b 要 Will）

**現況**：`CreatePost` 不看發布狀態與曝光，對每個追蹤者發 `post.created`（`S/internal/usecase/post.go:172-193`），所以代行建立的草稿也推；之後的發布與排程發布都不推（`PublishPostCreated(` 只有 `post.go:190`）。結果是：代行建立、之後才發布的貼文從來不通知追蹤者；不能公開的店仍可發文並推播，推播連到 404。
**業界基準**：內容第一次公開時通知。YouTube 在影片轉為公開（含排程）時通知訂閱者；Facebook 粉專未發布時不通知追蹤者。

| 選項 | 取捨 |
|---|---|
| (7a) 在 `CreatePost` 的推播前加「已發布 ∧ 店家此刻可曝光」 | 擋住草稿推播與 R1 的漏洞；代行的貼文發布後仍然不推 |
| (7b) **推播移到「第一次轉為已發布」**：建立即發布、`SetPublishStatus`、排程 worker 三處，條件是轉換前 `published_at IS NULL` ∧ 店家此刻可曝光 | 業界基準；`published_at` 下架不清（`posts.sql:166-181`、`post_schedule.sql:106`），不需要新欄位。下架再發布、平台恢復、店家恢復曝光都不補推 |
| (7c) 不改 | 違反 D2 R1 |

**約束帳**：通知範圍是產品行為，要 Will 同意（程序，不是推離基準的約束）。「另開一項」是計畫分段（SELF-IMPOSED），代價是 7a 的程式會被 7b 整段改寫。
**建議 7b，在 3a-② 直接做**。不核准的話做 7a。

## 8. 部署順序（runbook）

表沒有列就是不可曝光（fail-closed，與 Java V20 相同）；topic exchange 沒有綁定的佇列時，訊息會被丟掉。順序：① social 帶新 consumer 起來（或先套 `backend/infra/mq/topology/topology.yaml`，比照 `:754` 的 `product.seller.exposure.changed`）→ ② 單體 `make worker` 在跑 → ③ `make reannounce-exposures`（`backend/Makefile:332`）。本機 `./dev.sh seed` 最後一步就是重新公告（`backend/infra/seed-test-data.sh:71`）。寫進 runbook 與 ADR 2026-10-05 修訂。

## 9. 被治理隱藏的留言（既有規則的 bug，併入 3a-②、獨立 commit）

`HideComment`、`HideForumEntry` 只寫 `hidden_at`，列表只看 `deleted_at`（`comments.sql:4-40`、`forum.sql:1-19`）。做法：列表加 `hidden_at IS NULL`；隱藏時同一個 transaction 調整 `cms_post.comment_count`、父留言 `reply_count` 與討論區的 `reply_count`；對帳 SQL（`comments.sql:92-104`、`posts.sql:277-289`）一起排除隱藏的。目前沒有留言的恢復路徑，所以不用處理加回。

## 10. 10e 第 2c-3 刀已合入（現況，知會）

`83edc27b1` 已經經 `ca2949edf` 合進 main。關店可逆：`seller.deleted` 只在保留期清理（`purge.Run` → `store.RetireDueStores` → `StoreRetired`，`backend/internal/modules/seller/internal/purge/purge.go:58`）才發，關店本身不再讓 social 軟刪貼文。關店會重新公告可曝光（`backend/internal/modules/seller/internal/closing/closing.go:96,251,372`），所以 3a-② 的曝光過濾一上線就會把已關閉店家的貼文拿掉；在那之前，main 上這些貼文仍在公開面。這是 3a-② 的急迫性來源，不是先後限制。3a 不改 `sellerrelay/relay.go`。

## 明確不過濾的讀取（新增）

以下維持現狀，3a-② 各加一條回歸測試：`AdminListPosts`、`AdminModeratePost`、治理的 `ResolveOwner`／`HideTarget`（`S/internal/usecase/dispatcher.go`）、代行 `GetPostDetail`（全部發布狀態）、`ListMyPosts`、1a 的店主讀取、排程 worker、`AdminGetReport`。

## 已由 D2／D6 決定、不再列題（查證過）

- 討論區的過濾與寫入限制（D2）；順帶修掉「討論區留言不確認目標店家存在」（`S/internal/usecase/forum.go:62-78`）。
- 「店家身分的留言」以伺服器推導的 `storeID != 0` 判斷（`G/handler/post.go:735-752`）。
- 失去曝光前留在別家的店名快照照舊顯示。
- marketplace（第 5 刀刪）經 `GetPostList`，被 `ListPosts` 的條件涵蓋；分享卡片被第 3 題涵蓋；D1 已經讓不能公開的店停用分享。
- 遷移清冊 `events.csv`、`replicas.csv` 不加列；記錄落在 ADR 2026-10-05 修訂的「A7 刪除」。

---

## 切分：3a 拆成兩刀

| 刀 | 內容 | `maxRESTOperations` |
|---|---|---|
| **3a-①** 店主讀取與消費者先到位 | social `GetPostDetail` 的店主分支（NotFound、不加 view count；代行分支不變）、proto 註解；gateway `GET /app/sellers/posts/:id`、openapi、swag；ADR 2026-09-26 例外；website `PostEditClient` 與深連結改讀它；1b 的兩端規則與文案（含不訂閱 SSE）；buyer-app 貼文詳情把 404 呈現成「找不到」而不是網路錯誤（`BA/features/social/data/social_repository.dart:196-197`，純跟隨契約的 bug 修正）。公開行為還沒變，單獨上不壞任何東西 | 492 → **493** |
| **3a-②** social 曝光過濾 | `pkg/mq` 常數與事件型別、topology、consumer（依版本 upsert、舊版本 ACK 不改、缺欄位 `mq.Permanent`）、`store_exposure` 表與 migration；公開條件（view）套到 feed、hashtag、單店貼文、詳情、分享、SSE 預檢、留言父資源、討論區；hashtag 計數改計算；寫入檢查（第 4 題，含 `SubmitReport`）；推播 7b（或 7a）；套用後送 `SellerTag`；第 9 題（獨立 commit）；ADR 2026-10-05 修訂；runbook；backend-e2e-verify | 493 |

之後：3b 493 → 489，第 4 刀 488，第 5 刀 485，第 6 刀 **483**（原本 482，差 1 是 1a；`ca2949edf` 的 `maxRESTOperations` 仍是 492）。2c-3 已在 main，②越早上越好。

## 測試清單

**3a-①**
- 店主讀自己的草稿、已下架、平台下架、不可曝光店家的貼文 → 200，內容完整。
- 負向：店主讀別家的貼文 → NotFound（不是 `ResourceNotOwned`）；代行讀別家 → 仍是 `ResourceNotOwned` 且寫稽核；缺 `CapMarketing` → 既有的能力拒絕；未登入 → 401。
- 店主讀取不加 `view_count`。
- website：編輯頁重新整理後仍載入下架的貼文；深連結到不在第一頁的貼文能顯示；`rg fetchPostDetail W/features/seller-center` 為空；modal 在貼文不公開時顯示說明、不打留言與 SSE。
- seller-app：三種文案各一條 widget 測試，且不打 `/public/posts/{id}/comments`；公開時照舊。
- buyer-app：詳情 404 顯示「找不到」。

**3a-② 讀取（testcontainers）**
- 不可曝光、沒有列、可曝光三種店，各有已發布、草稿、已下架、平台下架的貼文：feed、`?tag=`、`/public/sellers/:id/posts` 只回可曝光店家的已發布貼文，total 相符。
- 詳情、分享網址、SSE 預檢：不公開 → not found。留言與回覆列表：父貼文不公開 → not found。
- 討論區：店家不可曝光 → 依 D2。
- hashtag：自動完成與熱門不計不公開的貼文；只被不公開貼文用過的標籤不出現。
- 隱藏的留言不在列表，`comment_count`、`reply_count` 與列表一致，對帳後仍一致。
- 每支公開 query 都經過同一個公開條件（測試列出 query 名稱）。
- 「明確不過濾的讀取」各一條。

**3a-② consumer**
- 新版本寫入；舊版本 ACK 不改資料、不送失效；同版本重送 ACK 並再送一次；`event_type` 錯、缺欄位、版本 ≤ 0 → `mq.Permanent`；store_id 與版本接受 number 與 string（與 Java 相同）。
- 翻轉後恰好送一個 `seller:<id>`。店家恢復可曝光後貼文回到 feed。

**3a-② 寫入（負向為主）**
- 對不可曝光店家的貼文、草稿：留言、按讚、分享、`ReportPost`、`SubmitReport`（三種目標）→ not found；計數與 `post.liked` 不變。
- 經留言 id 的按讚、編輯：父貼文不公開 → not found。
- D2 的討論區與店家身分留言規則。
- 例外：作者刪除自己的留言、店主回覆自己貼文的留言成功。
- 推播（7b）：建立即發布且可曝光 → 推給每位追蹤者；代行草稿建立不推、之後發布才推；排程發布推；下架再發布不推；不可曝光時發布不推、恢復曝光不補推。

**活體（backend-e2e-verify）**：擷取 `seller.exposure.changed` wire、查 `store_exposure`、翻轉前後 API 對照、`make reannounce-exposures` 版本加一、手動發舊版本確認表不變；結束後刪 `cmd/mq-capture-*`。

## 沒有查證到的

- hashtag 改成讀取時計算後的實際延遲；資料量小，只能以 `EXPLAIN` 確認走 `idx_cms_post_hashtag_lookup`。
- `post.created` 的下游（notification）是否另有過濾；7b 在發送端擋，不依賴下游。

## 旁支發現（不在 3a 範圍，記下）

- `GetPostDetail` 每次都 `IncrViewCount`，SSE 預檢與分享網址查詢也各算一次瀏覽。3a-② 讓這兩處改用 `GetPublicPost`，可以順帶不再計數，但這屬於另一條規則，要不要併入另議。
- social 的 `bustPost` 送 `hashtag:<貼文 hashtag id>`，website 把它展開成**商品**熱門 hashtag 的快取 tag，兩邊的 id 不是同一個空間。
