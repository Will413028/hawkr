# 社群互動者的身分：記成 (kind, id)，店家身分只用在回應自己的內容，由伺服器依資源推導

| 項目 | 值 |
|---|---|
| 狀態 | accepted（Will 2026-10-17，全照建議） |
| 日期 | 2026-10-17 |
| 影響範圍 | 舊 social-service（`cms_comment`、`cms_seller_forum`、`cms_post_like`、`cms_comment_like`、兩份 `social.proto`）、gateway 的 social handler 與留言 SSE、公開留言與討論區的輸出、website、buyer-app、seller-app；A7 social 模組照本 ADR 設計 |
| 取代 | 決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10d-3a-owner-surface-brief.md` 的 O2 (b)（貼文讚的 403）與 O2 文末「留言讚的撞號等 A7」；`step10d-decision-brief.md` D2 的寫入規則「不可曝光的店不能在別家貼文、別家討論區留言」與讀取端「店家身分的留言」過濾；`step10d-3a-decision-brief.md` 決定題的 `storeID != 0`（有店就以店家身分）與「留在別家的店名快照照舊顯示」（店家身分不再出現在別家內容）；遷移計畫 A7 的「讚的身分沒有角色欄，A7 重新設計」 |
| 計畫 | `docs/plans/2026-09-29-a3-buyer-seller.md` 步驟 10d 第 3a 刀；決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10d-3a-actor-brief.md` |

## 脈絡

`buyer_id`、`store_id` 是各自的 identity 序列，過渡期都是 bigint，會撞號（`backend/AGENTS.md`：業務表一律用業務鍵，兩者不可互換）。social 起初只有買家；公司時期把賣家改存 `store_id` 時沒加型別欄，留言與討論區後來各自補了 `role`，讚、檢舉、封鎖沒補。結果是：

- 留言與討論區的編輯、刪除只比 id，buyer 7 能改、刪 store 7 的留言，反之亦然。
- `buyer.deleted` 以 buyer_id 刪所有角色的留言與讚，同號店家的回覆一起消失；`account.deleted` 以 account_id 刪讚，值本來就不在同一個空間，而刪帳一定會發 `buyer.deleted`。
- 讚不記角色，store 7 讀得到 buyer 7 的讚；自我通知判斷跨命名空間。
- 身分來自 client 的 `X-Active-Role`（工作區是全域狀態），店主在賣家工作區對任何貼文留言都變成店家身分，包括別家的貼文；公開輸出帶內部 `userId`，client 用它判斷「是不是我」。

約束：

- 外部：舊 Java 消費者仍保存 BIGINT 的身分欄，兩份 `social.proto` 必須逐字相同；social 到 A7 才進單體，review、im 到 A6 才進單體。
- 繼承：website 仍送 `X-Active-Role`（chat、AI、通知還在讀，到 ADR `2026-10-07-connect-without-perspective` 的退場條件為止）；social 現在就可以不讀它。
- 沒有資料成本：系統從未上線，讚可以清空重算，違反規則的留言可以刪。

## 考慮過的方案

互動者怎麼記：

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：LinkedIn 每個讚與留言都有 `actor`（`urn:li:person` 或 `urn:li:organization`），讚的 id 是 `(actor, object)`，刪讚必須給建立它的 actor；Facebook 留言的 `from` 是 `User` 或 `Page`——互動記成「哪一種身分的哪一個 ID」 | 每張表加一欄 | 採用（M1） |
| M2 每種身分一欄加 `num_nonnulls = 1` | 兩種都合法時要兩個 partial unique、查詢分兩支 | 否決 |
| M3 全域 actor 登錄表 | 單體沒有擁有者，要新事件與回填，查名稱仍要知道型別 | 否決 |
| M4 等 uuid | 撞號消失，但期間照樣壞，仍不知道去哪個模組解析名稱與刪除 | 否決 |
| M5 存 `account_id` | 違反業務鍵規則；分不出店家回覆；換店時舊互動跟著人走 | 否決 |

店家身分用在哪裡：社群平台（Facebook 粉專、LinkedIn 公司頁）讓品牌在任何地方以品牌身分互動；平台上的商家（Etsy 評價回覆、Google 商家檔案、Amazon 商品問答）只在自己的頁面以商家身分回應，其他地方是個人。S1 社群型要保留 D2 的寫入規則並做身分選擇器；**S2 電商型**沒有同業互留，伺服器就能決定身分；S3 店家從不以店家身分，失去「店家回覆」。沒有 REAL 約束，選 S2 的理由是產品型態。

身分由誰決定：I1 維持 header 是上述現象的成因；I2 由 client 每次帶 actor，S2 下沒有可選的情況；**I3 伺服器依資源推導**，授權與身分同一個判斷。

## 決策

- **M1 互動者是 (kind, id)**：legacy 在讚表加 `actor_kind SMALLINT`（1=buyer、2=store，對齊既有的 `cms_comment.role`、`cms_seller_forum.role`，兩者沿用），讚的唯一鍵含 kind；`actor_kind` 與 `id` 同時決定「這是誰」，所有以身分比對的地方（編輯、刪除、自我通知、名稱與 principal 的查詢、刪除處理器）都比 (kind, id)。A7 終點：`actor_kind TEXT CHECK IN ('BUYER','STORE')`，讚表以 `(object_id, actor_kind, actor_id)` 為複合主鍵。事件與儲存帶 (kind, id)（proto 可用 `oneof` 或 kind＋id 表達）；請求不帶 actor，身分是 session principal 加資源歸屬推導（I3）。
- **S2 店家身分只用在回應自己內容的動作**：自己貼文的留言與回覆、對自己貼文底下的留言按讚、自己的討論區。其他地方一律本人。**貼文讚永遠是本人**（店主以本人身分讚自己的貼文允許，不推 `post.liked` 通知）。D2 的寫入規則與店家留言的讀取過濾因此不需要：店家身分只出現在自己的貼文與討論區，跟著它們一起被曝光過濾。「別人不能在不可曝光的店的討論區留言」與貼文、討論區的讀取過濾不變。
- **I3 伺服器依資源推導身分**：目標貼文、留言所在的貼文、討論區屬於呼叫者此刻的店 → 店家身分；否則 → 本人（沒有買家身分時拒絕，不退回 account_id）。gateway 的 social handler 不再讀 `X-Active-Role`、`loginRole`，只把單體帶來的兩個業務身分（`buyer_id`、`store_id`，沒有的是 0）原樣轉給 social，由 social 依擁有關係推導（資料在 social，gateway 要自己推導就得多一次 RPC，也有檢查與使用之間的時差）。停權的店在自己的內容上以店家身分動作照舊被擋，不退回本人。A7 終點：店主在自己內容上的動作在 `SellerAppService`，其餘在 `BuyerAppService`，伺服器仍以所有權驗證。範圍只含 social 的 handler；chat、通知收件匣、AI、product、檢舉與封鎖不動。
- **公開輸出不帶內部身分 ID**：公開留言與討論區列表、SSE 的 `comment.created`、`comment.replied`、`comment.like_changed` 拿掉 `userId`；列表與 SSE 帶伺服器算的 `isMine`（SSE 依每條連線的身分算）。名稱與頭像照舊。
- **刪除處理器依 kind**：`buyer.deleted` 只刪 kind=1，`seller.deleted` 只刪 kind=2 的讚；`account.deleted` 不再刪讚（值不在 account 空間，`buyer.deleted` 已涵蓋）。
- **資料處理**：兩張讚表清空後對帳計數；刪除違反 S2 的留言（店家身分出現在別家貼文、別家討論區）及其回覆並對帳留言數、回覆數。店主以本人身分在自己的貼文或討論區留言的舊資料保留不動：social 資料庫查不到貼文所屬店與留言者的對應（要跨服務查），且只有上線前的種子資料。

不加 `agent`（以組織身分動作者是誰）。觸發：店家有多名成員時比照 LinkedIn 加 `agent_account_id`。

## 後果

- 舊程式碼的改動在 A7 照本 ADR 重寫，不照搬；legacy 的 `actor_kind`、`isMine`、推導函式是 A7 模組的規格來源。
- `actor_id` 隨 buyer、store 換 uuid 的 migration 一起換型別；觸發：最後一個 Java 消費者退場。
- 若產品要讓店家在別處以店家身分互動（S1），加 I2（請求帶 actor）即可，不必遷移資料。
- 第 2 刀（`reports`、`user_blocks`、`moderation_actions` 帶 kind；Redis blocklist 的 key 帶 kind）排在 3a-② 之後，另出簡報決定「店家封鎖買家」的語意；在那之前封鎖過濾仍以呼叫者此刻推導出的身分 id 比對，不分 kind。
- 路由數不變；website 仍送 `X-Active-Role`，social 不讀。

## 修訂：A7 的 service 與停權的店（2026-10-17，Will）

原決策段保留歷史；決策簡報 `docs/plans/2026-09-29-a3-buyer-seller/step10d-a7-social-brief.md`（Will 全照建議）。

- **A7 終點改了 service 的歸屬**（取代決策段 I3 條目中「A7 終點：」那句：「店主在自己內容上的動作在 `SellerAppService`，其餘在 `BuyerAppService`，伺服器仍以所有權驗證」）：A7 的 API 是 `SocialPublicService`、`SocialAppService`、`SocialAdminService`，互動與店主對自己內容的管理（留言、回覆、刪除別人留在自己貼文與討論區的留言、貼文的發布與下架）都在 `SocialAppService`，公開讀取在 `SocialPublicService`。身分仍由伺服器依資源歸屬推導（I3 的推導規則不變）；以店家身分動作時，social 向 seller 檢查能力。理由：原寫法讓 client 先判斷「這是不是我的貼文」才知道打哪個 service，等於 client 決定身分；buyer-app 店主在自己貼文留言也要顯示店名。基準是 Google AIP、GitHub、YouTube 依資源分 service、能做什麼由伺服器對這個資源判斷。ADR `2026-10-07-connect-without-perspective` 與 API 設計規範同步修訂。
- **停權的店**：停權的店以店家身分在自己的內容上不能留言、回覆、按讚、編輯；可以下架、刪除，並讀取自己的貼文與留言。不退回本人身分。（取代決策段「停權的店在自己的內容上以店家身分動作照舊被擋」：舊寫法只涵蓋留言與回覆，按讚、編輯、刪除沒有分類，而且停權賣家進不了貼文列表，下架與刪除的按鈕按不到。）新增類（`CapMarketing`）擋下，收縮類（`CapContentContract`）放行，被拒回 `SELLER_SUSPENDED`。
- **封鎖與檢舉處分**：第 2 刀不在舊 social 做，併入 A7。個人封鎖存 buyer 模組、店家封鎖存 seller 模組（簡報 D2）；檢舉與處分由新的 `moderation` 模組負責，對象是店家的處分走 seller 治理的措施（簡報 D5′）。後果段「第 2 刀……排在 3a-② 之後」不再成立，封鎖的過渡行為（以呼叫者此刻推導出的身分 id 比對，不分 kind）維持到 A7 取代它。
- **脈絡更正**：脈絡段「舊 Java 消費者仍保存 BIGINT 的身分欄，兩份 `social.proto` 必須逐字相同」的後半不成立：Java 沒有呼叫 social（0 個 `.java` 檔，`events.csv` 也沒有 Java 消費 social 事件），所以「兩份 proto 逐字相同」不是外部約束。「ID 在過渡期是各自連號的 bigint」（ADR `2026-09-25` 的過渡規則）仍成立，決策與 (kind, id) 的理由不變。
