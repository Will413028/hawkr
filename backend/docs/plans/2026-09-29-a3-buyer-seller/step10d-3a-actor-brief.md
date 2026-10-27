# 10d 第 3a 刀追加：社群互動者的身分模型、貼文的照片規則（決定題 v2，已經 fresh subagent 審查）

> Will 2026-10-17 全照建議核定（A1–A4、B1）；UX 第 5、6 項文案照用，用字依 seller-app 改為「貼文」（見下方要同意的 UX）。結論記在計畫 `2026-09-29-a3-buyer-seller.md` 的「第 3a 刀開工的決定」。

HEAD `36fb5d894`（main）；3a-①b 進行中的改動在 `hawkr-10d3a1b` worktree（只讀，未提交）。
路徑縮寫：`S=backend/services/social-service`、`G=backend/gateway/internal`、`W=website`、`SA=seller-app/lib`、`BA=buyer-app/lib`。行號是 main。
SSOT 與前例：根 `AGENTS.md`「上線前不受相容性約束」「沿用也是決策」「不留技術債」；`backend/AGENTS.md`「`account_id` 是登入 principal；`buyer_id`、`store_id` 是業務鍵」；ADR `2026-09-25-row-identity-by-data-kind`、`2026-10-03-public-store-identity-and-visibility`、`2026-10-07-connect-without-perspective`、`2026-09-30-account-created-provisions-buyer`；`step10d-decision-brief.md` D2；`step10d-3a-owner-surface-brief.md` O1、O2；遷移清冊 `idrefs.csv:4`。審查紀錄沒有保存。

## 要 Will 決定的事

| # | 題目 | 建議 |
|---|---|---|
| **A1** | 互動者在資料裡怎麼記 | **actor = (kind, id)**，是終點不是過渡。legacy 在 9 欄旁加 `actor_kind SMALLINT`（1/2，對齊既有 `role`）；A7 終點是 `actor_kind TEXT CHECK IN ('BUYER','STORE')`、讚表以 `(object_id, actor_kind, actor_id)` 為複合主鍵 |
| **A2** | 店家身分用在哪裡 | **電商型（S2）**：店家身分只用在**回應自己內容**的動作——自己貼文的留言與回覆、對自己貼文底下的留言按讚、自己的討論區。其他地方一律本人。**貼文讚永遠是本人**，店主以本人身分讚自己的貼文允許（同 O2） |
| **A3** | 身分由誰決定 | **伺服器依資源推導（I3）**，social 不再讀 `X-Active-Role`／`loginRole`。只改 social 的 gateway handler，其他讀 loginRole 的路由不動 |
| **A4** | 什麼時候做 | **現在寫 ADR，接著做第 1 刀（互動），在 3a-② 之前**；取代 3a-①b 的後端那一半。第 2 刀（檢舉、封鎖、處置紀錄）緊接在 3a-② 之後，另出簡報定「店家封鎖買家」的語意 |
| **B1** | 貼文的照片規則 | **公開中的貼文恰好 1 張照片、內文必填**，由 social 在每一條進入公開的路徑檢查；草稿可以沒有照片。代行的「最多 6 張」改成 1 張（現在第 2–6 張被靜默丟掉） |

- `maxRESTOperations`：0（維持 493）。REST 只動回應欄位（加 `isMine`、拿掉公開的 `userId`），由 Check B 與 openapi 一起改。
- 推翻或修改的已核准決定：O2 (b)、O2 文末「留言讚等 A7」、D2 的寫入規則「不可曝光的店不能在別家貼文、別家討論區留言」與讀取端「店家身分的留言」過濾、3a 決定題的 `storeID != 0` 與「留在別家的店名快照照舊顯示」。細節在文末。
- 要同意的 UX：6 項，見文末。

---

## 現況（main，全部已查證）

**資料**：9 欄都是 `BIGINT`、沒有 FK、只有 `cms_comment.role`、`cms_seller_forum.role` 記角色（`S/db/schema/schema.sql:78-209`）。追蹤是 `buyer.seller_subscriptions(buyer_id, store_id)`，型別正確；分享不記身分。`buyer_id`、`store_id` 是各自的 identity 序列（`buyer/db/schema.sql:10`、`seller/db/schema.sql:32`），會撞號。

**身分怎麼來**：client 的 `X-Active-Role` → 單體 `OfLoginRole`（帳號 principal 一定得到 1 或 2，`internal/legacy/accounttype/accounttype.go:34-42`）→ gateway `loginRole` → 解析成 buyer_id 或 store_id。授權卻看帳號類型：留言的 `storeID` 來自 `isSellerRole`（`G/handler/resolve_seller.go:66-70`），social 以它判斷能不能回覆（`S/internal/usecase/comment.go:138-140`，回覆只限店主）。gateway 其實已從單體 header 拿到呼叫者的 `buyerId` 與 `storeId`（`G/middleware/auth.go:236-240`）。

**現在就壞掉的行為**

1. **越權**：留言與討論區的編輯、刪除只比 id（`comment.go:265,292`；`forum.go:132,149`），buyer 7 可以改、刪 store 7 的留言，反之亦然。
2. **刪錯資料**：`buyer.deleted` 以 buyer_id 軟刪所有角色的留言、討論區並刪讚（`S/internal/worker/buyer_deleted.go:22-40`；查詢只比 `user_id`，`comments.sql:110-115`、`forum.sql:43-45`、`posts.sql:258-259`）→ 同號店家的回覆一起消失。`seller.deleted`（保留期清理才發）以 store_id 刪讚（`seller_deleted.go:64-72`）。`account.deleted` 以 account_id 刪讚（`account_deleted.go:22-30`）；刪帳本來就會發 `buyer.deleted`（`cmd/hawkr/buyer_events.go:10-23`），這段純屬多餘且刪錯。
3. **讀寫撞號**：讚不記角色；賣家工作區讀 `isLiked` 用 store_id（`resolve_seller.go:293-320`），store 7 看到 buyer 7 的讚。
4. **通知判斷跨命名空間**：留言的自我判斷 `userID == post.SellerID`（`comment.go:197`）、留言讚的 `userID == existing.UserID`（`comment.go:366`）；按讚者名稱一律以買家查（`comment.go:399`）；`likerPrincipalID` 對不認識的 id 原樣回傳（`comment.go:380`）。
5. **封鎖靠猜**：seller 軸解析出的 account_id 再餵給 buyer 軸（`block.go:94-116`）；檢舉、封鎖的身分跨軸退路（`G/handler/moderation.go:47-75`）。
6. **公開輸出內部 ID**：SSE 的 `comment.like_changed`、`comment.created` 與公開留言列表都帶 `userId`（`G/realtime/comment_channel.go:133-146`；`pkg/mq/events.go:192-206`）；buyer-app 以它判斷 `fromMe` 與「自己的留言」（`BA/shared/widgets/comments_bottom_sheet.dart:574-581`；`BA/features/seller/presentation/widgets/merchant_chat_tab.dart:966-967`）。
7. **website 賣家工作區按讚 403**（worktree 的 3a-①b）：公開頁照樣帶 SELLER，錯誤被吞（`W/lib/hooks/useSocial.ts:611-628`）。UI 路徑沒有實際操作過。
8. **3a-② 的規則擋錯人**：「不可曝光的店不能在別家貼文留言」以 `storeID != 0`（帳號有店）判斷（`step10d-3a-decision-brief.md:148`）。

---

## A1. 互動者在資料裡怎麼記

**業界基準**：LinkedIn 每個讚與留言都有 `actor`（`urn:li:person` 或 `urn:li:organization`），以組織身分動作另記 `agent`；讚的 id 是 `(actor, object)`；刪讚必須給建立它的 actor（[Network Update Social Actions](https://learn.microsoft.com/en-us/linkedin/marketing/community-management/shares/network-update-social-actions)，已查證）。Facebook 留言的 `from` 是 `User|Page`（部分查證）。共通點：互動記成「哪一種身分的哪一個 ID」。

**既有機制為什麼存在**：social 起初只有買家；公司時期把賣家改存 store_id 時沒加型別欄，留言與討論區後來各自補了 `role`，讚、檢舉、封鎖沒補。是局部修補，不是設計。

| 選項 | 取捨 |
|---|---|
| (M1) **`actor_kind` + `actor_id`** | 一種寫法套到 9 欄；`role` 已是這個形狀；換 uuid 只改 `actor_id` 型別 |
| (M2) 每種身分一欄＋`num_nonnulls = 1` | 單體在「只會有一種身分」時用（`seller.reports.reporter_buyer_id`）；兩種都合法時要兩個 partial unique、查詢分兩支 |
| (M3) 全域 actor 登錄表 | 單體沒有擁有者（identity 只管 account principal），要新事件與回填，查名稱仍要知道型別 |
| (M4) 等 uuid | 撞號消失，但仍不知道去哪個模組解析名稱、刪除、通知；要等最後一個 Java 消費者，期間照樣壞 |
| (M5) 存 `account_id` | 違反業務鍵規則；分不出店家回覆；換店（新 stint）時舊互動跟著人走 |

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| ID 在過渡期是各自連號的 bigint | REAL（Java 以 BIGINT 保存）；推向「現在就要 kind」，終點換 uuid 後 kind 仍需要 | ADR 2026-09-25 過渡規則 |
| 不建跨 schema FK | REAL（單體架構前提）；讓多型關聯不能下 FK 的缺點不存在 | 單體 ADR D3 |
| legacy 用 SMALLINT 對齊 `role` | SELF-IMPOSED（一致性，省 Java proto 同步）；終點改 TEXT 照單體慣例 | `seller.measures.kind` |

**建議 M1，作為終點**。對外：proto 以 `oneof actor { buyer_id; store_id }` 表達寫入方；公開讀取不輸出買家 ID（給名稱、頭像，店家給 `store_code`），「是不是我」由伺服器算成 `isMine`。不加 `agent`，觸發：店家有多名成員時比照 LinkedIn 加 `agent_account_id`。

## A2. 店家身分用在哪裡

**業界基準分兩派**：社群平台（Facebook 粉專、LinkedIn 公司頁、Instagram 專業帳號、YouTube 頻道）讓品牌在任何地方以品牌身分互動；平台上的商家（Etsy 評價回覆，已附來源；Google 商家檔案、Amazon 商品問答，一般知識）只在自己的頁面以商家身分回應，其他地方是個人。

**既有機制為什麼存在**：website 的工作區是全域狀態，店主在賣家工作區的任何互動都變成店家身分，包括在別家貼文留頂層留言（`comment.go:115-146` 只限制回覆）。這是副作用，不是產品決定；D2 因此多了一條寫入規則。

| 選項 | 取捨 |
|---|---|
| (S1) 社群型 | 品牌曝光多；同業互留、灌數、引流；要保留 D2 那條規則；client 要做身分選擇器（I2） |
| (S2) **電商型** | 沒有同業互動；D2 的寫入規則與店家留言的讀取過濾都不需要；伺服器就能決定身分 |
| (S3) 店家從不以店家身分 | 失去「店家回覆」 |

**約束帳**：本題沒有 REAL 約束；選 S2 的理由是產品型態（電商、店家與買家是兩種身分）。A1 的資料模型兩派都支援：**終點觸發**——若產品要讓店家在別處以店家身分互動，加 I2（請求帶 actor）即可，不必遷移資料。

**建議 S2**，規則寫死成一句：**店家身分只用在回應自己內容的動作；貼文讚永遠是本人**。

## A3. 身分由誰決定

**業界基準**：身分是每個動作明確的、由伺服器依角色授權（Facebook 的 Page token、LinkedIn 請求內的 `actor`），不是全域環境狀態。本專案終點也是這樣：Connect 不讀工作區（ADR 2026-10-07）。

| 選項 | 取捨 |
|---|---|
| (I1) 維持 header | 現況 3、7、8 的成因 |
| (I2) client 每次帶 actor | S1 才需要；S2 下沒有可選的情況 |
| (I3) **伺服器依資源推導**：目標貼文、留言所在的貼文、討論區屬於呼叫者此刻的店 → 店家；否則本人 | 不需要 client 輸入；授權與身分同一個判斷；gateway 已有 `buyerId`、`storeId`，不加 RPC |

**範圍（避免擴散）**：只改 social 的 gateway handler——`post.go` 的留言、回覆、改、刪、留言讚、貼文讚、讀取個人化，`seller_forum.go` 全部，`sse_stream.go` 的個人化（每人連線上限改以 account_id 計）。`resolveOptionalBuyerID` 保留給 review（`review.go:224`）與 marketplace（第 5 刀刪）；chat、通知收件匣、AI、`product.go`、檢舉與封鎖（第 2 刀）不動。website 照送 `X-Active-Role`，social 不讀。停權的店在自己的內容上以店家身分動作照舊被擋，不退回本人。

**建議 I3**。A7 終點：店主在自己內容上的動作在 `SellerAppService`，其餘在 `BuyerAppService`，伺服器仍以所有權驗證。

## A4. 什麼時候做

**業界基準**：資料模型缺陷發現時修根本；搬遷中的舊系統只做讓它正確所需的最小修改，新系統照終點（strangler fig）。D2 對同樣情況已判過：「social 在 A7 才進單體」是 REAL，但只推向「legacy 修改要小」，不支持延後。

**對 O2 那條 REAL 的重新檢查**：「丟棄的工作」只對舊程式碼成立；模型的決定寫進 ADR，A7 照抄。缺陷現在就造成越權（現況 1）與刪錯資料（現況 2），且讓 3a-② 的規則擋錯人。上線前沒有資料遷移成本。

| 選項 | 取捨 |
|---|---|
| (T1) 維持 3a-①b 的 403，其餘等 A7 | 越權、刪錯資料留到 A7；3a-② 擋錯人 |
| (T2) 貼文讚一律走買家軸 | 只修現況 7，是繞過 |
| (T3) **ADR＋舊 social 最小正確修正** | 修掉全部現象；0 條路由；舊程式碼的改動 A7 刪掉 |
| (T4) A7 整個提前 | 約 33–36 個 REST operation 改 Connect，加 A7 其他待辦；10d、10e 後續都要等 |
| (T5) 只把讚與留言搬進單體 | 計數、刪除、曝光過濾跨兩系統一致性；不建議 |

**T3 範圍**
- **ADR**（新增，例如 `2026-10-17-social-actor-identity.md`）：M1、S2、I3；遷移計畫 A7 那條改成「照 ADR」。
- **第 1 刀（互動，3a-② 之前）**
  - schema：`cms_post_like`、`cms_comment_like` 加 `actor_kind`，唯一鍵含 kind；`cms_comment.role`、`cms_seller_forum.role` 沿用。migration：清空兩張讚表後由 `ReconcilePostLikeCounts` 對帳；刪除違反 S2 的列（店家身分在別家貼文、別家討論區的留言）並對帳留言數。
  - social：讚的讀寫、刪除帶 kind；**編輯、刪除的擁有權比 (kind, id)**；兩個自我通知判斷改用 kind（店家作者 ↔ 呼叫者此刻的店）；按讚者名稱、principal 依 kind 查；刪除處理器 buyer → kind=1、store → kind=2，刪掉 `account.deleted` 的刪讚。
  - gateway：I3；刪掉 social 路徑上的 account_id 退路；撤掉 3a-①b 的 403 與 `TogglePostLikeRequest.role`，改成 `actor_kind`。
  - 對外欄位：公開留言列表、SSE 的 `comment.created`、`comment.like_changed` 拿掉 `userId`；留言列表與 SSE 給伺服器算的 `isMine`（SSE 依連線者算）；website、buyer-app、seller-app 的 `fromMe` 與「自己的留言」改讀它。
- **第 2 刀（3a-② 之後，另出簡報）**：`reports`、`user_blocks`、`moderation_actions` 帶 kind；`block.go` 的猜測改依 kind；Redis blocklist 的 key 帶 kind，review、im 只改讀 key。**「店家封鎖買家」的語意**（封鎖後能不能聊天、評價、在討論區留言）跨 review、im，在那份簡報決定。

**約束帳**

| 約束 | 性質 | 來源 |
|---|---|---|
| social 在 A7 才進單體 | REAL（排定的步驟）；只推向「修改要小」 | 主計畫 A7；D2 同一判法 |
| review、im 在 A6 才進單體 | REAL；第 2 刀只動它們讀 key 的地方 | 主計畫 A6 |
| 兩份 `social.proto` 逐字相同 | REAL（Java 消費者仍在） | 3a-①b code review |
| website 仍送 `X-Active-Role` | REAL（chat、AI、通知還在讀，到 ADR 2026-10-07 退場條件為止）；social 現在就可以不讀 | ADR 2026-10-07 |
| 「舊服務只做維持運作所需的修改」 | SELF-IMPOSED（單體 ADR D8）；修越權與刪錯資料本來就符合 | 單體 ADR D8 |
| 拿掉 `userId` 要四端同刀 | SELF-IMPOSED（改動量），不能拿來延後 | — |
| 第 2 刀排在 3a-② 之後 | SELF-IMPOSED（計畫分段）；理由是語意要另外決定、3a-② 急迫（已關閉店家的貼文還在公開面） | `step10d-3a-decision-brief.md` §10 |

**建議 T3**。**終點與觸發**：A7 social 模組照 ADR；`actor_id` 隨 buyer、store 換 uuid 的 migration 一起換型別（觸發：最後一個 Java 消費者退場）；店家多成員時加 `agent`。

---

## B1. 貼文的照片規則

**現況**：內容模型是一張圖＋文字的全螢幕卡片（`cms_post.image`、`image_asset_id` 單欄；proto 陣列「取第一個」，`S/internal/grpc/handler.go:294-295,355-356`）。website 店主建立與發布要照片、代行建草稿可以沒有（`W/features/seller-center/posts/PostComposeClient.tsx:523-529`）；seller-app 建立與發布都不檢查；gateway、social 都不檢查（`G/handler/post.go:155-175`；`S/internal/usecase/post.go:114,374`）。店主自建預設已發布，建立等於發布。**代行允許 6 張**（`G/handler/admin_delegated_post.go:598-602`、設計書 56 第 3246、3939 行），social 只存第一張，其餘靜默丟掉。

**業界基準**：Instagram 動態發布一定要有媒體（[Content Publishing](https://developers.facebook.com/docs/instagram-platform/content-publishing)）；Threads、LINE VOOM 以明確的貼文類型支援純文字。照片要不要必填由內容模型決定；不變量由伺服器在狀態轉移時檢查，草稿可以不完整。

| 選項 | 取捨 |
|---|---|
| (P-a) **公開中的貼文恰好 1 張照片、內文必填**，social 檢查 | 符合現在的內容模型；一個規則來源；0 條路由 |
| (P-b) 文字貼文成為正式類型 | 新功能，要版面同意 |
| (P-c) 只統一前端 | 直接打 API 照樣繞過 |
| 多張照片（照設計書 56） | 全螢幕單圖卡片要改成輪播，三端版面與資料模型都要改；是產品新功能 |

**約束帳**：social 在 A7 才進單體（REAL，只決定程式碼放哪）；CodeMeta 凍結（SELF-IMPOSED，legacy 用既有 `InvalidInput`＋中文訊息，A7 用 `ErrorReason`）；「和 website 一致」（SELF-IMPOSED，不是理由）。

**建議 P-a**，在 social 檢查（gateway 只把代行上限 6 改成 1）：
- 檢查點：`CreatePost` 以已發布建立；`UpdatePublishStatus` 發布（含 `ScheduledAt` 排程）；`UpdatePost` 對已發布或排程中的貼文清圖。`RestorePost` 不改發布狀態，由前三條保證，不另檢。
- 照片數量：多於 1 張一律拒絕（不再靜默截斷）；設計書 56 改成 1 張。
- seller-app 建立與發布加同一條檢查（3a-①b 的 F1 併入）。
- **終點與觸發**：A7 `Post.Publish` 由聚合檢查；要文字貼文或多張照片時走 P-b 或輪播，觸發是產品決定。

---

## 推翻或修改的已核准決定

| 決定 | 怎麼變 |
|---|---|
| O2 (b)「gateway 403、social 拒絕 store 身分的貼文讚」 | 貼文讚一律本人，帶 kind；403 與 `role` 撤掉 |
| O2 文末「留言讚的撞號等 A7（REAL）」 | 重新分類：模型現在定（ADR），實作在第 1 刀 |
| D2 寫入規則「不可曝光的店不能在別家貼文、別家討論區留言」 | S2 之後不需要；「別人不能在不可曝光的店的討論區留言」與貼文、討論區的讀取過濾不變 |
| D2 讀取端「店家身分的留言」過濾、3a 決定題「留在別家的店名快照照舊顯示」 | 不需要：店家身分只在自己的內容上，跟著貼文、討論區一起被過濾 |
| 3a 決定題「店家身分的留言以 `storeID != 0` 判斷」 | 改成 `actor_kind = store` |
| 遷移計畫 A7「讚的身分沒有角色欄…A7 重新設計」（worktree 新增） | 改成「照 ADR，legacy 已在 10d 修正」 |
| 3a-①b design review W1 | 不再適用 |

不變：O1、O3、O4、O5、D6、ADR 2026-10-07。

## 3a-①b worktree 的處置

- **撤掉**：兩份 `social.proto` 的 `TogglePostLikeRequest.role` 與生成碼、social `ToggleLike` 的 `role` 拒絕、gateway 的 403 與 `@Failure 403`、對應測試、遷移計畫 A7 新增的「讚的身分」那條。
- **保留**：seller-app 管理面（拿掉自己按讚、刪 `double_tap_like_media.dart`、徽章、發布與下架、分享）、`scheduled_publish_at`、website 發布徽章與 `postPublishActions`、A7「店主的留言讀取」那條。
- 3a-①b 可以先以「保留」的部分提交，撤掉的部分由第 1 刀取代；或整包等第 1 刀一起提交。建議前者，讓 3a-①b 與身分修正各自可審。

## 要同意的 UX

1. 店主在 website 賣家工作區對**別家**貼文、別家討論區留言時，顯示自己的買家暱稱，不再是店名。
2. 店主在**自己店**的貼文留言、回覆、自己討論區留言時，不論從 website 哪個工作區或 buyer-app，都顯示店名（現在 buyer-app 與買家工作區顯示暱稱）。
3. 店主對自己貼文底下的留言按讚，不論從哪個 App，都算店家的讚，各 App 的愛心狀態一致。
4. 店主以本人身分讚自己的貼文：website 與 buyer-app 允許（seller-app 照 O1 沒有按讚）。website 賣家工作區在公開頁按讚恢復正常。
5. seller-app 建立貼文沒有照片時：按鈕可以按、按了提示、不送出。文案（核定）：「請先補上照片」／「每則貼文至少需要 1 張照片才能發布。」；發布按鈕沿用已核准的「請先編輯貼文並加入至少 1 張照片，再回來發布。」
6. 伺服器的後備訊息（只有繞過 client 時看得到）：「每則貼文需要 1 張照片才能發布」「貼文內容不能空白」「每則貼文只能有 1 張照片」（核定）。

## 沒有查證的

- website 賣家工作區實際走到公開頁並按讚（從程式碼推論）。
- Facebook `from` 的官方欄位表（只看到搜尋摘要）；Google 商家檔案、Amazon 問答、Meta 全域 ID 是一般知識。
- 沒有跑任何測試。
