# A7-0 盤點（領域、資料、機制切片）：`backend/services/social-service` → social／moderation 模組

唯讀盤點，快照 `172260d31`，2026-10-18。路徑相對於 `backend/`，`S=services/social-service`、`SI=services/social-service/internal`、`G=gateway/internal`。
決策來源：`docs/plans/2026-09-29-a3-buyer-seller/step10d-a7-social-brief.md`（P、D1、D2、D3、D4、D5′，Will 2026-10-17 全照建議）、ADR `2026-10-17-social-actor-identity`。
標記：【量】＝打開檔案或跑過指令；【推】＝讀程式推論；【猜】＝估計。本檔只盤「領域、資料、機制」；RPC／REST／client 呼叫點是別的切片。

---

## 0. 摘要

- 10 張表（`rg -c 'CREATE TABLE' S/db/schema/schema.sql` = 10【量】），18 支 migration（`ls S/db/migrations/*.sql | wc -l` = 18【量】），1 個 extension（`pg_trgm`，`schema.sql:224`），107 個 sqlc 查詢（`rg -c '^-- name:' S/db/queries/*.sql` 各檔 11／3／13／10／23／31／12／4 加總 = 107【量】）。
- 去向：`social` 7 張（`cms_post`、`cms_comment`、`cms_comment_like`、`cms_post_like`、`cms_seller_forum`、`cms_hashtag`、`cms_post_hashtag`）；`moderation` 2 張（`reports`、`moderation_actions`）；封鎖 1 張（`user_blocks`）**拆成 buyer 的個人封鎖與 seller 的店家封鎖，舊表刪除**（D2）。
- 事件：`events.csv` 中 social 當生產者 12 列、當消費者 3 列，`counter.delta` 兩邊都是 social，所以 14 列不重複（`awk -F, '$3=="social-service"' events.csv | wc -l` = 12；`$4==` = 3；`rg -c social-service` = 14【量】）。與 brief 的「14 列」一致。
- **最重要的發現**（細節見 §6）：
  1. 公開的 `GetPostDetail` 不過濾草稿、賣家下架、平台下架的貼文，且照樣累加 `view_count`（SD1）。
  2. 平台強制隱藏的留言與討論區回覆，**讀取端沒有任何地方過濾 `hidden_at`**，檢舉處置的「隱藏」對留言與討論區等於沒做；而且沒有任何還原路徑（SD2、SD13）。
  3. 封鎖在 social 只影響「列表過濾」，寫入端完全不擋；唯一的資料來源是 Redis，沒有重建、沒有 TTL，Redis 清空等於所有封鎖失效（SD3、SD14）。
  4. `post.created` 只在「建立貼文」時發，**草稿也發**；發布（互動式與排程）從來不發（SD8）。
  5. 代行稽核的 `RecordAuditIntent` 是**要回應的授權閘門**（回 9271 就取消排程），不是單向通知；brief 的「改成發事件」不能原樣成立（Q1）。
- SSE 的伺服器端**不在 social**，在 gateway（`G/sse/`、`G/handler/sse_stream.go`）；social 只發 fat event。`Last-Event-ID` 的重複投遞風險在 gateway（SD20，【推】）。
- 沒有 etag：social 沒有任何 ETag 機制（`rg -i etag S --glob '!*_test.go'` 無結果【量】）；最接近的是貼文的 `version` 樂觀鎖。
- 沒有曝光（exposure）與店家狀態過濾：`rg -i 'exposure|exposed|closed|suspend' S/internal`（非測試、非生成）只命中停權旗標與 BAN 呼叫【量】；公開列表只問 `deleted_at/hidden_at/publish_status`（`S/db/queries/posts.sql:8-14`）。

---

## 1. 領域與資料

### 1.1 表

來源：`S/db/schema/schema.sql`（246 行）。sqlc 查詢在 `S/db/queries/`，生成碼在 `SI/repository/postgres/gen/`。DB 無 FK（全檔 `rg -c 'REFERENCES|FOREIGN KEY'` = 0【量】）。

| # | 表（schema 行） | 主鍵／ID | 值得注意的欄位 | 約束與索引 | 寫入（file:line） | 讀取（file:line） | 去向 |
|---|---|---|---|---|---|---|---|
| 1 | `cms_post`（2-63） | `id BIGINT`，TSID（`SI/usecase/di.go:29`） | `seller_id`＝**store_id**（欄名沒改）；`like/comment/view/share_count`；`hidden_at/hidden_reason`（平台隱藏）；`deleted_at`（軟刪）；`publish_status SMALLINT` 預設 4402（草稿 4401／已發布 4402／已下架 4403，CodeMeta）；`version`；排程欄 `scheduled_publish_at`、`schedule_state`、`scheduled_by_session_id`、`schedule_lease_*`、`schedule_next_attempt_at`、`schedule_retry_count`；`image`（object key）、`image_asset_id`、`product_id`（單一） | **`publish_status` 沒有 CHECK**；`version >= 0`；`schedule_state` 六值 CHECK（`pending/done/expired/cancelled_by_revocation/cancelled_by_deletion/cancelled_by_governance`）；`pending ⇒ scheduled_publish_at NOT NULL`；索引 `(seller_id, deleted_at, create_time DESC)`、partial `(scheduled_publish_at) WHERE schedule_state='pending'`；表註解禁硬刪（71） | 建立 `post_repository.go:83`；改 `:132`（CAS 強制，`posts.sql:151-155`）；發布狀態 `:185`（CAS 可停用，`posts.sql:178-190`）；排程 `:224`；軟刪 `:249`；整店軟刪 `:304`；隱藏／還原 `:366/:373`；view/share `:253/:257`；計數 `:261-275`（由 `counter_delta.go:48-55` 呼叫）；排程 worker `post_schedule_repository.go:24-140`；對帳 `:315/:319` | 公開列表 `:43`（`posts.sql:8-14` 過濾 deleted/hidden/published）；單篇 `:23`（**不過濾**，`posts.sql:1-2`）；我的貼文 `:323`；後台 `:350`（join `reports`，`posts.sql:60-90`）；標籤列表 `:377` | **social**。`seller_id` 改 `store_id`（brief D5′ 的 seller 契約）；`publish_status` 改 TEXT enum＋CHECK（可選 `draft/published/unpublished`）；排程欄見 Q3；`view/share_count` 見 Q4；`hidden_*` 改為隱藏紀錄（見 Q6） |
| 2 | `reports`（78-101） | `id BIGINT` TSID | `reporter_id`（依「當下 active role」存 buyer_id 或 store_id，**無 kind**，`:106` 註解）；`target_type`（REVIEW/POST/POST_COMMENT/SELLER_FORUM/CHAT_MESSAGE/USER）、`target_id`（內容 id，跨服務的 bigint）、`target_owner_id`（提交當下解析的內容作者，**無 kind**）；`reason_code`（8 值）；`evidence JSONB`；`target_snapshot JSONB`（**從未寫入**，SD21）；`status`（PENDING/RESOLVED_HIDDEN/RESOLVED_WARNED/RESOLVED_BANNED/DISMISSED/APPEALED）；`resolved_action`（HIDE_TARGET/WARN_USER/BAN_USER/DISMISS）、`resolved_by` | 四個 CHECK（`:94-100`）；索引 `(status, create_time DESC)`、`(reporter_id, target_type, target_id, create_time DESC)`、`(target_type, target_id, create_time DESC)`；**無唯一約束**（24 小時去重只靠查詢，`reports.sql:1-9`） | 提交 `report_repository.go:99`（經 `report.go:93-120`）；後台直接下架合成報告 `report.go:132-151`（`reporter_id=0`）；裁決 `:164`；重開 `:195` | 我的檢舉 `:117`；後台列表／單筆 `:134/:153`；累犯數 `:213`（`reports.sql:83-91`，90 天、排除 DISMISS）；`cms_post` 後台 join（`posts.sql:60-90`） | **moderation**（D5′）。`reporter`、`target_owner` 帶 (kind, id)；`target_id` 的型別見 Q2；**對象是店家的檢舉不另存**，直接進 seller 通報；`target_snapshot` 改為提交時必寫（Q8） |
| 3 | `user_blocks`（109-117） | `id BIGINT` TSID | `blocker_id`、`blocked_id`（**無 kind**，`:119` 註解：現任 buyer_id 或 store_id，查無時退回 account_id） | `UNIQUE(blocker_id, blocked_id)`、`blocker_id <> blocked_id`、索引 `(blocked_id)` | `block_repository.go:18`（`blocks.sql:1-6` 冪等 upsert）、刪 `:30`；**同時**寫 Redis `block:out:/block:in:`（`SI/usecase/block.go:69,122`） | 列表 `:51`；`:37` Get（無呼叫者，【量】`rg 'blockRepo.Get\(' SI --glob '!*_test.go'` 無結果）；列表過濾**不讀這張表**，讀 Redis（`blocklist_filter.go:19,34,49`） | **刪除，拆成兩張**：buyer 的個人封鎖（買家→買家、買家→店家）、seller 的店家封鎖（店家→買家）（D2）。**不搬資料**（系統從未上線，且舊紀錄無 kind 無從歸屬），刪表時一併刪 Redis 兩組 key 的寫入端 |
| 4 | `moderation_actions`（123-137） | `id BIGINT` TSID | `report_id`、`admin_id`、`affected_user_id`（無 kind）、`action`（6 值：含 AUTO_REJECT、REOPEN，**無 CHECK**）、`note`、`attempted_content`（**從未寫入**）、`previous_state/new_state JSONB` | 三個索引；無 CHECK | `moderation_action_repository.go:18`（呼叫端 `report.go:262-271,317-328`，錯誤被丟棄） | **無讀取者**：`ListModerationActionsByReport/ByUser`（`moderation.sql:6-15`）在非生成、非測試碼無呼叫（【量】`rg 'ListModerationActionsBy' S --glob '!*_test.go' --glob '!**/gen/**'` 只有 .sql 本身） | **moderation**，但重新設計成「處置紀錄」：寫入與狀態轉換同一個 tx、`action` 加 CHECK、補讀取（Q8）。AUTO_REJECT、`attempted_content` 沒有寫入者，刪 |
| 5 | `cms_comment`（140-161） | `id BIGINT` TSID | `post_id`；`user_id`＋`role`（1 買家／2 店家，CHECK `(1,2)`）＝(kind, id)；`user_name/user_avatar/reply_to_name`（建立時的反正規化快照）；`parent_id`／`reply_to_id` 用 **0 當哨兵**（非 NULL）；`like_count`、`reply_count`；`deleted_at`；`hidden_at/hidden_reason` | CHECK role；索引 `(post_id, parent_id, deleted_at, create_time)`、`(parent_id, deleted_at, create_time DESC)`；**無 FK、無深度約束** | 建立 `comment_repository.go:100`；改 `:119`；軟刪 `:130`（`comments.sql:53-54` **無 `deleted_at IS NULL` 述詞**）；隱藏 `:134`；計數 `:141-155`；整人軟刪 `:184`（`buyer_deleted.go:22`） | 頂層／回覆列表 `:26/:47`（`comments.sql:4-28` **不過濾 `hidden_at`**）；單筆 `:18` | **social**。`actor_kind TEXT CHECK` ＋ actor_id（ADR 10-17 M1 終點）；`parent_id` 改 NULL＋模組內 FK；反正規化名稱欄見 Q9 |
| 6 | `cms_comment_like`（167-175） | `id BIGSERIAL` | `comment_id`、`user_id`＋`actor_kind`（CHECK `(1,2)`） | `UNIQUE(comment_id, actor_kind, user_id)` | `comment_repository.go:168`（`ON CONFLICT DO NOTHING`，`comments.sql:71-73`）；刪 `:172`；整人刪 `:180`（`buyer_deleted.go:37`、`seller_deleted.go:64`） | `:157`、批次 `:176` | **social**，主鍵改複合自然鍵（`comment_id, actor_kind, actor_id`），去掉 `BIGSERIAL` |
| 7 | `cms_post_like`（181-189） | `id BIGSERIAL` | `post_id`、`user_id`＋`actor_kind`，**CHECK `actor_kind = 1`**（貼文讚永遠是買家，`:188`） | `UNIQUE(post_id, actor_kind, user_id)` | `post_repository.go:288`、刪 `:292`、整人刪 `:300` | `:277`、批次 `:296` | **social**；`actor_kind` 恆為 1，從零設計不存這欄（主鍵 `(post_id, buyer_id)`）。ADR 10-17 為了「兩張讚表同一種讀寫」才保留它（SELF-IMPOSED），見 §2 M2 |
| 8 | `cms_seller_forum`（195-215） | `id BIGINT` TSID | `seller_id`＝store_id；`user_id`＋`role`＝(kind, id)；`user_name/user_avatar/reply_to_name`；`parent_id/reply_to_id` 哨兵 0；`reply_count`（**同步維護**，非 counter.delta）；`deleted_at`；`hidden_at/hidden_reason` | CHECK role；索引 `(seller_id, parent_id, deleted_at, create_time DESC)`、`(parent_id, deleted_at, create_time DESC)` | 建立 `forum_repository.go:71`；改 `:90`（`forum.sql:36-38` **不過濾 deleted/hidden**）；軟刪 `:101`（無 `deleted_at` 述詞）；隱藏 `:105`；計數 `:112/:116`；整人刪 `:120` | 列表 `:29/:50`（`forum.sql:1-19` **不過濾 `hidden_at`**）；單筆 `:18` | **social**；表名改 `forum_entries`，`seller_id`→`store_id` |
| 9 | `cms_hashtag`（226-235） | `id BIGINT` TSID | `name VARCHAR(40)`（canonical：NFC＋ASCII 小寫）、`post_count` | `UNIQUE(name)`、`varchar_pattern_ops`（前綴 LIKE）、GIN `gin_trgm_ops`（包含搜尋）；**`post_count` 無下限保護**（`hashtag.sql:7-10` 沒有 GREATEST） | upsert `hashtag_repository.go:31`；計數 `:35`（`HashtagUseCase` `hashtag.go:140-143, 215-233`） | 前綴／包含搜尋 `:106/:110`；趨勢 `:114`；批次取名 `:91` | **social**。`post_count` 是否維護見 Q5 |
| 10 | `cms_post_hashtag`（237-246） | 複合 PK `(post_id, hashtag_id)` | `display_name VARCHAR(40)`（保留原大小寫）、`position SMALLINT` | 索引 `(hashtag_id, post_id)`、`(create_time, hashtag_id)` | `hashtag_repository.go:39/:48/:57/:64` | `:68/:72/:76`；趨勢查詢用 `ph.create_time`（`hashtag.sql:67-79`） | **social** |

附註：
- `cms_post.image` 與 `image_asset_id` 並存（`schema.sql:50`，註解 `:45-49`：`image_asset_id` NULL 而 `image` 非 NULL＝回填前的 legacy 列）。系統從未上線、沒有 legacy 列，A7 只需一欄資產參照（`media` 模組）。
- `schema.sql:38-49`、`posts.sql:123-125,147-150` 仍寫「media reconciler 每輪重放」，但該機制已由 migration `20260906114425_retire_media_reconciler_table_comment.sql` 退場（SD22，文件漂移）。
- 沒有「save／收藏」表（`20260713131104_remove_post_save.sql` 已移除）。

### 1.2 social-service 擁有的資料與不變量（程式強制處）

| 不變量 | 強制處 | 備註 |
|---|---|---|
| 公開中的貼文恰好 1 張圖、內文必填（B1） | `SI/usecase/post.go:111-125, 133-137, 299-304, 420-445`；多圖在 gRPC 邊界擋 `SI/grpc/handler.go:294,354` | 草稿可不完整。排程中的貼文視同公開要件（`post.go:299-304`）【量】 |
| 貼文更新一律 CAS（`version`）；發布／下架 CAS 可停用（`expected_version < 0`） | `posts.sql:151-155, 178-190`；`post.go:234-239` | 賣家自助路徑也走 CAS（#2772）【量】 |
| 貼文歸屬：改、刪、發布前比對 `post.SellerID == sellerID` | `post.go:230, 411-416, 496-498`；`posts.sql:223-224`（述詞帶 seller_id，rows=0 就不做副作用） | `GetPost` 刻意無 seller 述詞（`handler.go:211-217`），讀取歸屬靠 handler 的三條分支【量】 |
| 排程中的貼文恆為草稿（pending ⇒ publish_status=4401） | `posts.sql:196-216` 的 `publish_status <> 已發布` 述詞；`post.go:420-429` | hashtag 計數論證依賴它【量】 |
| hashtag 計數只算「公開可見」（已發布、未刪、未隱藏）的貼文 | `post.go:177-178, 336-347, 452-463, 499`；`post_hider.go:60`；`post.go:698-712`；worker `post_schedule_worker.go:187` | 草稿不計數；`AdjustHashtagPostCount` 沒有下限，靠呼叫端的 wasVisible 判斷【量】 |
| 標籤規則：最多 10 個、每個 ≤40 字元、僅 `a-z0-9` 與漢字 | `SI/domain/hashtag/validator.go:6-9, 73-81` | 不含假名、韓文、底線、拉丁變音字母（Q5）【量】 |
| 留言兩層：回覆的 parent 永遠壓成頂層 | `comment.go:178-186`；`forum.go:108-116` | 沒有資料庫約束【量】 |
| 回覆只限貼文所屬的店；店家身分只用在自己貼文／討論區 | `SI/usecase/actor.go:39-57`；`comment.go:188-195`；`forum.go:118-124`（ADR 10-17 S2） | 身分第 1 刀剛落地（`6b1cf36af`）【量】 |
| 作者才能改留言；作者或貼文所屬店才能刪 | `comment.go:294-296, 321-329`；`forum.go:148-150, 165-167` | 比對 (kind, id)【量】 |
| 停權的店不能以店家身分留言／回覆（討論區同） | `comment.go:196-198`；`forum.go:88-90, 122-124`；旗標來自 gRPC metadata（`handler.go:25-33`） | 讚、編輯、刪除不擋（brief D3 問題 1）【量】 |
| 自己不能封鎖自己（比對 login principal） | `block.go:42-64` | fail-closed：兩邊都要解析得到（#2789）【量】 |
| 檢舉：每人每天最多 20 筆、同對象 24 小時內回傳既有紀錄 | `report.go:35, 100-112`；`reports.sql:1-9` | 沒有唯一約束，check-then-insert 有競態（SD23）【推】 |
| 裁決：副作用先做、再轉狀態；狀態更新帶 `status IN (PENDING, APPEALED)` | `report.go:182-244`；`reports.sql:45` | 並發裁決時副作用可能做兩次（HIDE/BAN 本身冪等）【推】 |
| BAN 的對象是內容作者（提交當下快照），不是內容 id | `report.go:290-298` | 作者無 kind（SD10）【量】 |
| 軟刪貼文不釋放媒體引用；整店退場批次釋放 | `post.go:522-535`；`seller_deleted.go:49-104` | 批次 200 筆（5 秒 deadline）【量】 |
| 貼文不得硬刪 | `schema.sql:71` 表註解 | 與軟刪列永不回收圖片並存【量】 |

### 1.3 碰到但不擁有的資料

| 資料 | 擁有者 | 社交怎麼碰 | A7 後 |
|---|---|---|---|
| 買家名稱／頭像、粉絲清單 | buyer | 讀：`client/buyer/client.go:32,49,86`；`GetSubscribersList` 全頁讀入記憶體（`:49-84`） | `buyer` 公開 API（social→buyer 已在依賴表，`internal/arch/modules.go` social 列）；粉絲 fan-out 見 Q3 |
| 店家名稱／logo、principal 解析 | seller | 讀：`client/seller/client.go:31,49,70,124` | `seller` 公開 API；principal 解析整個消失（單體事件直接帶 account_id 或由 notification 解析，見 M18） |
| 登入 principal、停權 | identity | 寫：`client/account/client.go:84`（`SuspendAccount`）；讀：`GetAccountInfo*` | BAN 買家走 identity（D5′）；`GetAccountInfo*` 在 social 內有定義但本切片未找到呼叫者（【推】`rg 'GetAccountInfo' SI --glob '!*_test.go'` 只在 client 與介面） |
| 商品基本資料 | catalog（Java product-service，過渡經 `M/legacy/productservice`） | 讀：`client/product/client.go:29`；貼文只存 `product_id` | `catalog` 公開 API；`product_id` 歸屬驗證缺口見 SD24 |
| 媒體資產與引用 | media | 寫：`FinalizeAndLink`／`ReleaseReferences`（`client/media/client.go:33,40`） | `media` 模組已在單體 |
| 評價、聊天訊息的隱藏 | review、im | 寫：`client/review/client.go:28`、`client/im/client.go:28` | D5′：由 `moderation` 經 legacy client 呼叫 |
| 代行 session 與稽核 | account-service → 單體 delegation | 寫：`client/account/delegation.go:35,84` | Q1 |
| 網站快取標籤 | website | 寫：`pkg/revalidate`（`post.go:78-84`） | `platform/revalidate`（單體已有） |

---

## 2. 沿用的機制（沿用盤點的原料）

「從零設計」一欄是一行答案；「去向」是 A7 的建議落點。

| # | 機制 | 位置與做法 | 從零設計會怎麼做 | 去向 |
|---|---|---|---|---|
| M1 | **ID 方案**：貼文、留言、討論區、hashtag、report、block、audit 用 TSID bigint；兩張讚表用 `BIGSERIAL` | `SI/usecase/di.go:29`（`tsid.NewGenerator`）；`schema.sql:168,182` | Java 沒有呼叫 social（brief P【量】），social 擁有的實體可以直接用 UUIDv7（row-identity ADR）；`actor_id` 因 buyer／store 仍是 bigint 而維持（ADR 10-17「觸發：最後一個 Java 消費者退場」）。`reports.target_id` 要指向 review／im 的 bigint，見 Q2 | social 實體 UUIDv7；讚表複合自然鍵；`actor_id bigint` 沿用 |
| M2 | **互動者 (kind, id)** 與讚表 `actor_kind` | `SI/usecase/actor.go:5-57`；`schema.sql:171,185,150,205` | 留言／討論區：`actor_kind TEXT CHECK`＋`actor_id`。**貼文讚恆為買家**，不需要 kind 欄（`schema.sql:188` CHECK 把它鎖死為 1） | social。post_like 去 kind（SELF-IMPOSED 的對稱性不保留） |
| M3 | **計數器**：`like_count`／`comment_count`／`reply_count` 先發 `counter.delta`（RabbitMQ）、自己消費並 `±1`；10 分鐘一次全表對帳；討論區 `reply_count` 同步 `±1` 且無對帳；`view/share_count` 直接 `+1` | 發：`post.go:573,596`、`comment.go:218-221,337-340,396,415`；收：`worker/counter_delta.go:31-78`；對帳：`counter_reconcile.go:24,52-66`、`posts.sql:261-289`、`comments.sql:78-104`；討論區：`forum.go:132,173` | 單一 DB 內不需要 MQ：同 tx 更新（熱點貼文的 row lock 疑慮要量測，原因是 2026-07-31 的「避開 row lock 競爭」，ADR `2026-09-18-mq-social-counter-async-delta`）；或讀時 `COUNT` 加索引；對帳 job 若保留則用 River。事件非冪等，是舊架構自己承認的缺口（`counter_reconcile.go:20-23`） | Q4。**不搬 `counter.delta` 事件與消費者**（自己發自己收，沒有外部消費者，`events.csv`）；`counter_reconcile` 隨之退場 |
| M4 | **Redis key**（social 讀寫的全部） | `block:out:<id>`、`block:in:<id>`（Redis SET，無 TTL）：`pkg/blocklist/blocklist.go:20-21`；寫 `usecase/block.go:69,122`；讀（social）`grpc/blocklist_filter.go:19,34,49`；讀（別服務）`im-service/internal/usecase/message.go:497,809`、`review-service/internal/grpc/handler.go:43`。**沒有其他 key**：`rg 'rdb\.|redis\.' SI --glob '!*_test.go'` 只有這條路徑【量】。啟動連不上 Redis 只警告（`infrastructure/redis.go:21-26`），讀取 fail-open（`blocklist.go:89-114`） | 不用 Redis 當資料來源（A3 已在別處拿掉同樣做法，brief D2）；封鎖是 DB 的資料，過濾用 DB 讀取（Q7） | 刪除。A6 之前由單體的 legacy gRPC 回答「是否封鎖」（brief 依賴表），im／review 換掉讀 Redis 那一行 |
| M5 | **etag** | social **沒有**。最接近：`cms_post.version` CAS（`posts.sql:151-155`）；委派寫入用 `expectedVersion`（`handler.go:349,410`，`delegation.ParseExpectedVersion`） | AIP-154 的 `etag`＝由 `version` 推導；寫入帶 `etag` 或不帶。留言、討論區目前**無版本**，編輯為 last-write-wins（`comments.sql:48-51`） | social：貼文 `version`→`etag`（brief A7-1 列了 etag）；留言／討論區是否加見 Q10 |
| M6 | **Hashtag 抽取／搜尋** | **不從內文抽取**：client 送 `tags` 陣列（`handler.go:307, 371`）；正規化 `domain/hashtag/normalizer.go:20-32`（trim→去前導`#`→NFC→只小寫 ASCII），驗證 `validator.go:20-81`；`Apply` 以 id 升冪做 diff＋計數（`hashtag.go:43-152`，避免死結）；前綴 `LIKE name%`＋`post_count>0`（`hashtag.sql:53-58`）；包含 `name % q`（pg_trgm 相似度，`:60-65`）；趨勢 `COUNT(*)` 以**綁定時間**分組（`:67-79`），窗口預設 1 個月、最長 1 年（`hashtag.go:317-402`） | 保留顯式標籤（沒有解析內文的歧義）；字元集要重新決定（Q5）；趨勢用發布時間而非綁定時間；`post_count` 若可由查詢導出就不維護 | social（Q5） |
| M7 | **排程發布 worker** | 輪詢（預設 30 秒，批 50）＋租約（120 秒）＋CAS 終寫＋重試（5 次，退避 60→1800 秒）；三條收斂（刪除／平台下架／逾 24 小時窗口）每個 tick 先跑；`outbox.Scheduler` 驅動並吞 panic：`worker/post_schedule_worker.go:77-242`、`di.go:174-214`、`config/config.go:170-183`、`post_schedule.sql` | 單體用 River 的 scheduled job（`run_at`＝排程時間，內建重試、唯一鍵、取消），租約／輪詢／六態 `schedule_state` 欄位都由 job 表承擔（`platform/jobs`）。取消條件（刪除、平台下架、session 撤銷）在 job 執行時再檢查一次 | Q3。state machine 不搬；`scheduled_publish_at`＋`scheduled_by_session_id` 留在貼文上，執行交給 job |
| M8 | **代行稽核（delegated audit）** | worker 在發布前 `RecordAuditIntent`（`post_schedule_worker.go:248-283`，回 `AuditLogID`；9271＝session 已撤銷→`cancelByRevocation` `:164-169,422-435`），發布後 `ConfirmAuditResult`（`:289-329`，重試到總預算 2 秒，失敗留 PENDING）。`origin=scheduled` 是提權欄位，由 account-service 驗證 service 名稱（`:244-247`）。其餘代行讀寫在 handler 呼叫 `delegation.Validate`（`handler.go:194,288,345,406,573`）。冪等鍵 `schedule:<postID>:<scheduledAt.Unix()>`（`:159`） | **請求／回應式授權閘門**，不是通知。單向事件取代不了「intent 失敗＝不要發布」 | Q1（brief A7-0 要定案的項目；依賴方向 `delegation→social`，`internal/arch/modules.go` delegation 列【量】） |
| M9 | **Outbox／MQ 發布** | **沒有 outbox**：直接同步 publish，`_ =` 丟棄錯誤（如 `comment.go:216-223,248,277`、`post.go:213,573,584`、`post_hider.go:73`）；發布者 `mq/publisher.go:72-182`；9 種 routing key：`post.created`、`post.liked`、`post.hidden_by_platform`、`comment.created`、`comment.replied`、`comment.updated`、`comment.deleted`、`comment.like_changed`、`counter.delta`（`events.csv` 另列 gateway SSE 為消費者）；消費 `buyer.deleted`、`seller.deleted`、`counter.delta`（`worker/di.go:245-269`，MaxRetries 3） | 單體規則 4：下層通知上層且需原子性用 `events.Publish`（tx 內）；副作用用 `jobs.Enqueue`。fat event（含 `CommentSnapshot`）A8 之前要維持原形（brief 依賴表） | A7-3 relay（前例 `M/legacy/sellerrelay`）；`counter.delta` 退場；`buyer.deleted`／`seller.deleted` 改為單體事件訂閱 |
| M10 | **SSE** | **不在 social**。social 只發 fat event；`comment_snapshot.go:22-48` 建 `CommentSnapshot`，`resolveCommentIdentity`（`:70-98`）解析名稱頭像（失敗重試一次）。伺服器端在 gateway：`G/sse/hub.go`（事件 id＝`<epochMillis>-<seq>`，pod 內計數，`:379-383`）、`ringbuffer.go`（每 topic 環形緩衝，`Since(lastEventID)` 找不到就回 `bufferMiss`，`:66-85`）、`G/handler/sse_stream.go` | Connect server streaming；重播語意與 fan-out 來源見 Q11 | 單體 A7-2（brief：即時留言改 server streaming）；gateway 的 `sse/` 隨之刪除 |
| M11 | **封鎖**（資料＋過濾） | 見 M4；`block.go:42-71` 寫 DB 再寫 Redis（失敗丟棄）；`resolvePrincipal` 先問 seller 再把結果餵 buyer（`:94-115`，無 kind，所以撞號時猜）；過濾在 handler 層、分頁之後（`handler.go:114,780,810,972`）；**寫入端完全不擋**（`rg 'IsBlocked' SI/usecase` 無結果【量】） | brief D2 已定模型與效果；實作機制（讀取模型）是 Q7 | buyer／seller 模組；social 只做讀取過濾與寫入檢查 |
| M12 | **檢舉處置 dispatcher** | `FanoutHideDispatcher`（`usecase/dispatcher.go:42-59`）依 `target_type` 分流：POST→`PostHider`、POST_COMMENT／SELLER_FORUM→直接 `hidden_at`、REVIEW／CHAT_MESSAGE→gRPC；`AccountBanDispatcher.BanUser`（`:118-148`）解析 principal 後 `SuspendAccount`；`WARN_USER` 只寫稽核（`report.go:235-237`）。`ResolveOwner`（`dispatcher.go:64-89`）：POST→`seller_id`、留言／討論區→`user_id`（**無 kind**）、REVIEW／CHAT→0 | brief D5′ 已定：店家處分走 seller 治理、買家處分留 moderation。**內容作者是店家時，處理＝建立措施「內容下架」**，由 seller 事件讓 social 隱藏 | moderation（D5′）；`PostHider` 的副作用（計數、快取、事件）留在 social，由 moderation 呼叫 social API 或訂閱事件觸發 |
| M13 | **曝光／可見性過濾** | 公開列表：`deleted_at IS NULL AND hidden_at IS NULL AND publish_status=已發布`（`posts.sql:8-22`、`:315-332`、`hashtag.sql:67-79`）；單篇不過濾（`posts.sql:1-2`，SD1）；留言／討論區列表不看貼文狀態也不看 `hidden_at`（SD2）；**沒有任何店家狀態（曝光、關閉、停權）的過濾**（§0 的 rg）。停權只靠 gRPC metadata 旗標擋店家身分的留言／回覆（`handler.go:25-33`） | 「公開可讀」是一個述詞（店家可曝光∧貼文已發布∧未隱藏∧未刪），所有讀取（貼文、留言、討論區、標籤、趨勢、搜尋）共用 | social：Q5 的一部分；brief 3a-② 已核准併入 A7 |
| M14 | **能力／停權檢查** | 無。靠 gateway 掛能力（`G/router/routes_post.go:63-78`，brief D3）；social 只收旗標 | 向 seller 查能力（`CapMarketing`／`CapContentContract`，brief D3），在 social 內判斷 | social＋seller（brief D3 已定） |
| M15 | **媒體參照** | 建立／換圖：`FinalizeAndLink`（commit 前、致命）→寫列→`ReleaseReferences`（commit 後、best-effort）：`post.go:148, 263, 363-369, 743-756`；軟刪**刻意不釋放**（`post.go:522-535`）；整店退場批次釋放 200 筆（`seller_deleted.go:27, 88-103`） | 單體內 `media` 模組同 tx 或 job；軟刪是否永遠留引用：從零設計是「保留期後釋放」 | social＋media。軟刪保留引用的理由（media GC 會刪實體）在單體內可用同 tx 解決，決定見 Q9 |
| M16 | **商品參照** | `product_id`（單一）只存值；列表／詳情時現查（`handler.go:116, 262-267`、`post.go:625-637`）；失敗回 nil 不報錯 | 寫入時驗證存在且屬於該店；讀時批次 | social＋catalog（SD24） |
| M17 | **髒字詞典** | 硬編碼 14 個詞、子字串比對（`pkg/moderation/dictionary.go:12-24`，Phase 2「DB 驅動」未做）；呼叫處只有建立貼文、留言、討論區發言與回覆（`post.go:139`、`comment.go:169`、`forum.go:81,100`），**不含任何編輯路徑**（SD6） | 詞庫要有擁有者與後台維護；是否保留預先過濾見 Q12 | moderation（Q12） |
| M18 | **principal 解析** | 事件要投遞到「收件匣身分」：店家→account_id、買家→buyer_id（`comment.go:140-161`）；解析失敗**靜默退回原值**（`client/seller/client.go:70-122`，`unresolved_ids` 只記 warn）；一次留言最多 3–4 次同步 RPC（`comment.go:200, 236, 246, 259`） | 事件帶業務鍵（store_id／buyer_id）與 kind，由 notification（A8）自己解析身分空間；social 不認識 account_id | A7-3 relay 維持原事件形狀；原始碼不搬 |
| M19 | **讀時補名稱頭像＋反正規化快照** | 列表時 `lookupActors` 批次覆蓋 `user_name/user_avatar`（`handler.go:858-913, 1015-1035`），DB 欄位是失敗時的後備；快照寫在列上（`comment.go:200-210`） | 只讀時補，不存快照；刪除買家後 PII 不留在列上 | social（Q9）。兩支 `cmd/backfill-*` 隨快照欄位一起刪 |
| M20 | **分頁** | `page/pageSize`＋`COUNT(*)`，上限 50（後台 100）：`domain/constants.go:6-14`、`post.go:97-109`；list 與 count 兩條獨立語句（無快照） | AIP-158 keyset（`platform/pagination`）；是否保留總數見 Q10 | social；`filter-after-page` 問題隨之消失（SD15） |
| M21 | **兩層留言、哨兵 0** | `comment.go:178-186`；`schema.sql:147-148` | `parent_id` NULL＋同表 FK＋CHECK 深度 | social |
| M22 | **追蹤者 fan-out**（`post.created`） | `CreatePost` 取全部粉絲 id（逐頁 100 筆，`client/buyer/client.go:49-84`）→逐一同步 publish（`post.go:196-216`），一個粉絲一則事件 | 事件發一次（`StorePostPublished`），由 notification 展開粉絲；且觸發時機應是「公開」，不是「建立」（SD8） | A7-3 relay 維持舊事件形狀（brief：A8 前不變）；觸發點與展開見 Q3 |
| M23 | **整人／整店刪除處理器** | `buyer.deleted`：軟刪該買家（kind=1）的留言與討論區、刪兩種讚（`buyer_deleted.go:22-40`）；`seller.deleted`：整店貼文軟刪＋釋放媒體＋刪店家身分的留言讚（`seller_deleted.go:53-103`） | 同 tx 的事件訂閱；範圍要完整（SD16） | social（訂閱 buyer／seller 單體事件） |
| M24 | **檢舉防濫用** | 每人每天 20 筆、24 小時去重（`report.go:35, 100-112`） | DB 唯一約束＋時間桶或冪等鍵；速率限制交給入口（`httpapi` 已有限流） | moderation |
| M25 | **一次性回填指令** | `cmd/backfill-comment-seller-snapshot`、`cmd/backfill-seller-content-identity`（`SI/repository/postgres/comment_snapshot_backfill.go`、`seller_identity_backfill.go`） | 從未上線，沒有要回填的資料 | **刪除** |
| M26 | **網站快取失效** | 貼文增刪改、隱藏還原、排程發布後 `BustAsync`（`post.go:78-84`；`post_hider.go:67-70`；`post_schedule_worker.go:439-445`）。標籤只送 entity tag（`post:<id>`、`hashtag:<id>`） | 同樣的需求，用 `platform/revalidate`，在 tx 後 | social |
| M27 | **optimistic 讚的 SSE 計數** | `comment.go:395-428`：因為計數非同步，事件裡的 `likeCount` 用 `existing.LikeCount±1`（樂觀值，下限 0） | 計數同 tx 後直接讀真值 | 隨 M3 一起消失 |

---

## 3. CodeMeta 值域與錯誤碼

social 用到的 CodeMeta（`rg -o --no-filename 'codemeta\.[A-Za-z]+' S/internal S/db --glob '!*_test.go' --glob '!**/gen/**' | sort | uniq -c`，【量】）：

| 值域 | 用到的值 | 儲存處 | 單體對應（根目錄 AGENTS.md「共享代碼值」） |
|---|---|---|---|
| `PostPublishStatus` | `Draft` 4401／`Published` 4402／`Unpublished` 4403（`domain/constants.go:29-31`） | `cms_post.publish_status SMALLINT`，**無 CHECK**（`schema.sql:23`）；對外：舊 REST 的 `publishStatus` 碼值字串（`G/handler/post.go:956`） | 儲存的代碼：擁有資料的模組（Go 型別＋CHECK）；對外：proto enum。**不搬數值碼** |
| `ReportTargetType` | `Post`、`PostComment`、`SellerForum`、`Review`、`ChatMessage`、`User` | `reports.target_type`，CHECK（`schema.sql:94`） | proto enum（moderation）；`CHAT_MESSAGE`、`REVIEW` 在 A6 前走 legacy client |
| `ReportReasonCode` | 8 值（SPAM…OTHER；`Other` 為後台直接下架的預設，`report.go:134`） | `reports.reason_code`，CHECK（`:95`） | proto enum；與 seller 通報的類別對齊（brief D5′ 事實 3） |
| `ReportStatus` | `Pending`、`Appealed`、`ResolvedHidden`、`ResolvedWarned`、`ResolvedBanned`、`Dismissed` | `reports.status`，CHECK（`:96`） | moderation 狀態機（見 Q8：`APPEALED` 現在只是「重開」） |
| `ReportResolveAction` | `HideTarget`、`WarnUser`、`BanUser`、`Dismiss` | `reports.resolved_action`，CHECK（`:100`） | D5′：店家不再有 Warn／Ban；買家 Warn 的實際效果見 SD11 |
| `ModerationActionType` | 只用到 `Reopen`（`report.go:326`）；`AutoReject` 從無寫入者 | `moderation_actions.action`，**無 CHECK**（`:128`） | 縮成實際用到的值並加 CHECK |

非 CodeMeta 的 social 自有值：`actor_kind`／`role` 1 買家、2 店家（`domain/constants.go:19-22`，CHECK）；`schedule_state` 六值（`domain/post_schedule.go`，CHECK `schema.sql:53-59`）；`target_status`／`status` 篩選字串 `published/hidden/deleted`（`posts.sql:35-37,84-87`，無 enum）；留言列表的 `Action` 字串 `HIDE/RESTORE`（`handler.go:681-705`）。

**錯誤碼**（`SI/usecase/errors.go`、`SI/domain/errors.go`、`pkg/errors/bizcode/bizcode.go`）：

| 碼 | 名稱 | 出處 | 用途 |
|---|---|---|---|
| 9210 | `ModerationRejected` | `bizcode.go:186`；`errors.go:12` | 髒字詞典命中 |
| 9211 | `ReportRateLimited` | `bizcode.go:187`；`errors.go:13` | 檢舉每日上限（gRPC 層改回 `ResourceExhausted`，`handler_moderation.go:54-56`） |
| 9240 | `SellerSuspendedOperationBlocked` | `bizcode.go:202`；`errors.go:32` | 停權賣家的留言／回覆 |
| 9266 | `DelegationResourceVersionConflict` | `domain/errors.go:15` | CAS 失敗 |
| 9271 | `DelegationSessionRevoked` | `post_schedule_worker.go:167` | 排程時 session 已撤銷 |
| 9275 | `DelegationResourceNotOwned` | `domain/errors.go:26` | 代行讀寫別家貼文 |
| 9283 | `DelegationScheduledPostNotPublishable` | `post_schedule_worker.go:228` | 終寫 CAS 輸了 |
| 9284 | `DelegationScheduledPostRetryExhausted` | `post_schedule_worker.go:391` | 排程重試用盡 |
| 9290 | `DelegationResourceAlreadyInState` | `errors.go:22` | 已發布貼文再排程 |
| 3015–3018 | hashtag：`InvalidChar`／`LengthExceeded`／`CountExceeded`／`EmptyAfterNormalize` | `domain/hashtag/errors.go:24-27` | 標籤驗證 |
| （無碼） | `NotFound`／`Forbidden`／`InvalidInput`、`ErrCannotBlockSelf`、`ErrUnresolvedBlockPrincipal`、`ErrAlreadyResolved`、`ErrUnknownTargetOwner`、`ErrPostImageRequired`／`ErrPostContentBlank`／`ErrPostTooManyImages` | `errors.go`、`block.go:17-28`、`report.go:18,33` | 舊契約把這些映成 `InvalidInput`（「legacy 沿用 InvalidInput」，`errors.go:24-25`）；新契約用 `api/` 的 `ErrorReason` |

單體對應：停權用既有 `SELLER_SUSPENDED`（brief 文案表最後一列）；其餘 `ErrorReason` 要在 A7-2 新增（本切片不列）。

---

## 4. 背景作業、排程與外部 provider

| 項目 | 位置 | 節奏／觸發 | 副作用 | 去向 |
|---|---|---|---|---|
| 排程貼文發布 | `worker/post_schedule_worker.go:77`；`worker/di.go:174-214` | 30 秒輪詢（`config/config.go:171`） | 寫 `cms_post`、呼叫 account-service 稽核、website revalidate | Q3（River job） |
| 計數對帳 | `worker/counter_reconcile.go:24` | 10 分鐘，全表掃描（`posts.sql:265` 的 `ponytail` 註解自承） | 改 4 種計數 | 退場（M3、Q4） |
| `counter.delta` 消費者 | `worker/di.go:261-267` | 事件 | 計數 `±1` | 退場 |
| `buyer.deleted` 消費者 | `worker/di.go:247-253` | 事件，MaxRetries 3 | 軟刪、刪讚 | 單體訂閱（M23） |
| `seller.deleted` 消費者 | `worker/di.go:255-260` | 事件，MaxRetries 3 | 軟刪、釋放媒體、刪讚 | 單體訂閱；seller 現在只在 `StoreRetired`（保留期後）才發 `seller.deleted`（`internal/modules/seller/CARRYOVER.md:186`，【推】來自文件，非本切片原始碼）→ 關店後貼文在保留期內仍公開（brief 缺口之一） |
| 一次性回填 | `S/cmd/backfill-*` | 手動 | — | 刪除（M25） |
| 外部 provider | **無直接外部 provider**。經內部服務：account-service、buyer-service、seller-service、media、product（Java）、review、im（`client/*`，gRPC，`pkg/serviceauth` HMAC，【推】來自 `client/media/client.go` 與 `internal_auth_env_test.go` 的存在）；外部 HTTP：website revalidate（`pkg/revalidate`） | — | — | 全部變成同程序呼叫；revalidate 沿用 |

---

## 5. 與單體的耦合（A7 依賴表的原料）

- 現行 arch 表：`"social": {im, review, catalog, buyer, seller, identity, media}`；`"delegation"` 依賴 `social`；`"notification"` 依賴 `social`（`internal/arch/modules.go`【量】）。
- 依 brief D5′：social 拿掉 `im`、`review`；新增 `moderation`（依賴 social、im、review、identity、buyer、seller）；notification 加 moderation。social **不依賴 delegation**（環）。
- social 對 seller 的需求：店家名稱／logo、principal（消失）、能力（D3）、曝光（3a-②）、店家封鎖（D2）。對 buyer：名稱／頭像、粉絲、個人封鎖。
- 事件（`events.csv` 的 social 列，disposition 全空）：

| routing key | 方向 | 消費者 | 建議 disposition |
|---|---|---|---|
| `post.created`／`post.liked`／`post.hidden_by_platform` | 發 | notification-service | A7-3 relay，原形保留到 A8 |
| `comment.created`／`comment.replied`／`comment.like_changed` | 發 | notification-service＋gateway SSE | relay 給 notification；SSE 由 server streaming 取代，不再需要事件給 gateway |
| `comment.updated`／`comment.deleted` | 發 | 只有 gateway SSE | 隨 SSE 一起刪（沒有其他消費者，`events.csv`） |
| `counter.delta` | 發＋收（自己） | social 自己 | 刪除（M3） |
| `buyer.deleted`／`seller.deleted` | 收 | social | 單體事件訂閱（M23） |

---

## 6. 可疑的缺陷與出人意料的行為

處分欄：**修**＝在 A7 的新碼修，並寫一個**舊行為會失敗**的回歸測試；**接受**＝舊行為隨舊碼退場，新碼不會有，附理由；**待決**＝依賴某個 Q。本切片沒有修舊 social（brief P：舊 social 不再加改動）。

| # | 缺陷／行為 | 證據 | 為什麼是問題 | 處置 |
|---|---|---|---|---|
| SD1 | 公開 `GetPostDetail` 回傳草稿、賣家已下架、平台隱藏的貼文，且累加 `view_count` | `posts.sql:1-2`（只過濾 `deleted_at`）；`handler.go:251-253`（預設分支 `IncrViewCount`）；gateway 直接轉發 `G/handler/post.go:104-124`【量】 | 知道 id（分享連結、舊通知）的人能讀到被平台隱藏的內容；草稿可在發布前被讀；`view_count` 被灌 | **修**：公開讀取共用「公開可讀」述詞（M13、Q5）；回歸測試＝以匿名身分讀平台隱藏貼文應 NotFound |
| SD2 | 平台隱藏的**留言與討論區回覆**沒有任何讀取端過濾；留言列表也不看貼文本身的狀態 | `comments.sql:4-28`、`forum.sql:1-19`（只過濾 `deleted_at`）；`handler.go:800-801`（`GetCommentList` 沒有取貼文）；`rg 'hidden' SI/grpc/mapper.go` 只命中商品註解【量】 | `HideComment`／`HideForumEntry`（`comments.sql:117-120`、`forum.sql:47-50`）設了 `hidden_at` 卻沒人讀它：檢舉裁決「隱藏」對留言與討論區無效；已隱藏／已下架／草稿貼文的留言照樣公開列出 | **修**（A7 讀取述詞）；回歸測試＝隱藏後列表不得含該留言 |
| SD3 | **封鎖不擋寫入**：被封鎖者仍能在封鎖者（店家）的貼文留言、回覆、按讚；封鎖只在列表層過濾 | `rg 'IsBlocked\|blocklist\.' SI/usecase` 只有 `block.go:69,122`（Add/Remove）【量】；過濾只在 `grpc/blocklist_filter.go` | brief D2 的效果表要求「店家封鎖買家：對方不能留言、回覆、按讚」；舊 social 完全沒做 | **修**（D2 的新設計，寫入端檢查）；測試＝店家封鎖買家後該買家留言被拒 |
| SD4 | `replyToID` 沒有驗證：可指向別篇貼文、已刪除或已隱藏的留言；`reply_to_name` 與通知照常產生 | `comment.go:178-186` 只驗 `parentID`；`:204-208` 取 `replyToName`、`:253-278` 對 `replyTo.UserID` 發 `comment.replied`，都不比對 `replyTo.PostID == postID`【量】 | 可對任意留言作者發出通知（內文由發送者決定），並洩漏別篇貼文的留言者名稱 | **修**：新碼只接受同貼文、未刪、可見的回覆目標 |
| SD5 | 重複刪除不冪等：`SoftDeleteComment`／`SoftDeleteForumEntry` 沒有 `deleted_at IS NULL` 述詞，`DeleteComment`／`DeleteComment`（forum）不檢查已刪除 | `comments.sql:53-54`；`forum.sql:30-31`；`comment.go:315-342`（取回後不看 `DeletedAt`）；`forum.go:159-175`【量】 | 每次重送 DELETE 都再發一次 `counter.delta -1` 與 `comment.deleted`（SSE）；討論區 `reply_count` 重複扣（有 `GREATEST(0)` 下限，但數字仍錯），而討論區**沒有對帳**（`counter_reconcile.go:52-66` 不含 forum）。留言的錯誤最多 10 分鐘後被對帳修正 | **修**（新碼 `WHERE deleted_at IS NULL` ＋ rows=0 不做副作用，同 `posts.sql:219-224` 的作法）；測試＝連刪兩次只扣一次 |
| SD6 | 髒字詞典**只擋建立**，編輯完全繞過 | `moderation.Check` 只有 4 個呼叫處：`post.go:139`、`comment.go:169`、`forum.go:81,100`；`UpdatePost`（`post.go:225`）、`UpdateComment`（`comment.go:284`）、`UpdateForumComment`（`forum.go:136`）都沒有【量】 | 先發乾淨內容再改成違規內容 | **修**（若 Q12 決定保留預先過濾，則所有寫入路徑共用同一個入口）；測試＝以違規詞編輯被拒 |
| SD7 | 平台隱藏後內容仍可被作者編輯；已刪除的討論區回覆也可編輯 | `comments.sql:48-51`（只 `deleted_at IS NULL`，無 `hidden_at`）；`forum.sql:36-38`（**兩者都不查**；usecase 只在 `forum.go:145` 先查 `DeletedAt`）【量】 | 被處置的內容可被改寫再申覆；與 SD6 疊加 | **修**：隱藏中的內容不可編輯（待 Q6 決定作者能否看到自己被隱藏的內容） |
| SD8 | `post.created`（通知追蹤者）在**建立草稿時**發，**發布時不發** | 唯一呼叫處 `post.go:213`，前面沒有 `publishStatus` 判斷（`:196-216`）；`UpdatePublishStatus`（`post.go:406-479`）與 worker `publishOne`（`post_schedule_worker.go:127-242`）都不發【量，`rg 'PublishPostCreated\(' SI` 只此一處呼叫】 | 代行建立的草稿會推播給所有追蹤者（內容尚未公開）；互動式發布與排程發布永遠沒人收到通知 | **修**：觸發點改為「轉為公開」（Q3）；測試＝建立草稿不發、發布才發 |
| SD9 | 推播預覽以**位元組**截斷（`preview[:100]`），CJK 內容常切在字元中間 | `post.go:198-200`；`comment.go:229-232, 263-266`；`domain/constants.go:14`（`PreviewMaxLength = 100`）【量】 | 中文每字 3 位元組，100 不是 3 的倍數，切點約 2/3 機率落在字中間 ⇒ 推播與 inbox 出現亂碼 `�`（【推】未實測） | **修**（新碼以 rune 截斷；`platform/text` 或 `ui-text` 若已有共用規則則沿用，Go `TrimSpace`／rune 計長的先例見 `@hawkr/api` 的 `text/server-text`）；測試＝100 個漢字的預覽是合法 UTF-8 |
| SD10 | `BAN_USER` 與 `ResolveOwner` 的身分**無 kind**，且先當 store 解析再當 buyer 解析 | `dispatcher.go:72-83`（留言／討論區回 `UserID` 不帶 `Role`）；`:125-143`（先 `sellerClient.ResolvePrincipalStrict` 再把結果餵 `buyerClient`）；`report.go:290-298`【量】 | buyer_id 與 store_id 同號時（撞號是 ADR 10-17 的前提），舉報買家 7 的留言可能停用店家 7 的店主帳號；且把 account_id 餵進 buyer 軸可能被誤認為 buyer_id（【推】TSID 機率低） | **修**（reports/audit 帶 (kind, id)，D5′ 的新模型）；BAN 對象是店家時不可到達（改走措施）。測試＝同號 buyer／store 的 BAN 解析到正確帳號 |
| SD11 | `WARN_USER` 只寫稽核，**使用者不會被通知、也沒有任何效果**；累犯計數卻算它 | `report.go:235-237`；`reports.sql:83-91`（`prior_offence_actions` 含 WARN）【量】 | 管理員按「警告」以為對方被警告 | **待決**：D5′ 已移除店家的 Warn（走措施）；買家 Warn 要不要發通知與留紀錄（DSA 17 的理由通知，brief 文末「買家對內容處理」未解的第 1 項） |
| SD12 | 稽核寫入是 best-effort 且被丟棄，與 schema 註解「append-only，每次下架／處置都必須有一筆」矛盾；表**沒有讀取者**；`attempted_content`、`AUTO_REJECT` 從無寫入者 | `report.go:247-272, 309-329`（`_ =`）；`schema.sql:121`；`moderation.sql:6-15` 無呼叫；§1.1 表 4【量】 | 處置與稽核不在同一個 tx：狀態已轉換、稽核可能缺 | **修**：新 moderation 的處置與稽核同 tx（Q8） |
| SD13 | 駁回／重開**不還原**被隱藏的內容；留言與討論區**沒有任何還原路徑** | `report.go:235-237`（DISMISS 無副作用）、`:304-331`（Reopen 只改狀態）；`handler.go:696-702` 只有貼文的 RESTORE；`rg -i 'restore' db/queries/comments.sql db/queries/forum.sql` 無結果【量】 | 申覆成功或誤判後，留言與討論區內容永遠隱藏 | **待決** Q6（隱藏／還原的語意與 DSA 申訴） |
| SD14 | 封鎖的**唯一讀取來源是 Redis**，沒有重建、沒有 TTL；寫 Redis 失敗被丟棄；讀失敗 fail-open | `block.go:65-70,122`；`pkg/blocklist/blocklist.go:51-61,89-97`；`rg -i 'warm\|rebuild\|SMembers' pkg/blocklist S/internal` 無重建程式【量】 | Redis 重啟／清空／寫入失敗 ⇒ DB 說封鎖了、列表照樣顯示；im／review 同樣讀這份 | **接受**（舊碼隨 D2 退場）；新碼以 DB 為唯一來源 |
| SD15 | 封鎖過濾在**分頁之後**，且以不帶 kind 的原始數字比對 | `handler.go:114, 780, 810, 972`（過濾後才組結果，`total` 仍是未過濾的）；`blocklist_filter.go:19,34,49`（`viewerID` 是 `viewer.ID`，貼文比對 `p.SellerID`＝store_id，留言比對 `c.UserID` 不看 `Role`）【量】 | 一頁可能 0 筆但還有下一頁；`total` 與內容對不上；買家 7 封鎖了「某 id 7」會同時隱藏店家 7 | **接受**（舊碼退場）；新碼的過濾在查詢內（Q7、Q10） |
| SD16 | `seller.deleted` 處理不完整：貼文軟刪後**不調整 hashtag `post_count`、不 bust 快取**，也不處理該店的討論區與店家身分的留言；`DeletePost` 的計數失敗只記 warn；hashtag 計數**沒有對帳** | `seller_deleted.go:53-103`（無 hashtag 呼叫）；`posts.sql:291-293`；`post.go:511-518`；`post_schedule_worker.go:184-186`（「本專案沒有、也不允許有對帳程序」）【量】 | 被退場店家的標籤永遠留在自動完成與趨勢排序；`AdjustHashtagPostCount` 無下限（`hashtag.sql:7-10`）。店家留言不刪屬於 ADR 10-17 的取捨（留言因貼文軟刪而不可見）：【推】但 `GetCommentList` 不查貼文狀態（SD2），所以仍可列出 | **修**：退場以同 tx 調整計數（或 Q5 決定計數改為讀時導出） |
| SD17 | 讚與留言的計數：`counter.delta` **非冪等**（重送重複計數）；`ToggleLike` 為 get→create 兩步，雙擊會發兩次 `+1`（`ON CONFLICT DO NOTHING` 讓列只有一筆） | `counter_reconcile.go:20-23`（自承）；`post.go:565-574`、`comment.go:387-396`；`posts.sql:247-249`、`comments.sql:71-73`【量，競態為【推】】 | 計數與通知（`post.liked`）可能重複，由 10 分鐘對帳修正 | **接受**：M3 退場；新碼以 `INSERT … ON CONFLICT DO NOTHING RETURNING` 判斷是否真的新增（測試＝並發雙擊只加一） |
| SD18 | `view_count`／`share_count` 無驗證、無去重、無頻率限制；委派者讀取也算一次瀏覽 | `post.go:538-551`；`handler.go:241`（委派分支 `IncrViewCount`）、`posts.sql:226-230` | 數字可被刷；草稿與隱藏貼文也被計 | **待決** Q4 |
| SD19 | 標籤搜尋不轉義 `LIKE` 萬用字元；搜尋輸入只做 normalize 不驗字元集 | `hashtag.go:267`（只 `Normalize`）；`normalizer.go:20-32`；`hashtag.sql:53-58`（`name LIKE $1 || '%'`）【量】 | `_`、`%` 成為萬用字元（影響限於自動完成，上限 20 筆且 `post_count>0`）；pg_trgm 對 CJK 的行為取決於資料庫 locale【猜，未實測】 | **修**（新碼先驗證字元集或轉義）；CJK trigram 見 Q5 |
| SD20 | **SSE（gateway）事件 id 由 gateway 在收到 MQ 訊息時產生**，MQ 重送會得到新 id ⇒ `Last-Event-ID` 無法去重；id 是 pod 內序號，換 pod 重連一律 `bufferMiss` | `G/sse/hub.go:379-383`（`<epochMillis>-<seq>`，`seqCounter` 為 Hub 欄位）；`ringbuffer.go:66-85`（找不到就回 miss）；`hub.go:190-191`；social 發布時帶 `clientRequestId`（`comment.go:248,277`）供 client 去重【推】，社群資料本身沒有序號 | 非 social 原始碼的缺陷；A7 改 server streaming 後要重新定義重播語意（Q11）。**我沒有實測重複投遞**，僅憑讀 `hub.go`／`ringbuffer.go` 推論 | **接受**（gateway 的 SSE 隨 A7-4 刪除）；新設計見 Q11 |
| SD21 | `reports.target_snapshot` 從未寫入；舉報之後內容被編輯或刪除，管理員看到的是修改後的內容 | `rg 'TargetSnapshot' S` 只有 `domain/repository.go:222`、`report_repository.go:109`（轉傳）、sqlc 設定；`report.go` 全檔無賦值【量】 | 沒有「被舉報當下的內容」證據；DSA 申訴需要 | **修**（Q8：提交時凍結內容快照） |
| SD22 | 文件漂移：註解仍描述已退場的 media reconciler | `schema.sql:38-49`、`posts.sql:123-125,147-150` vs migration `20260906114425_retire_media_reconciler_table_comment.sql`【量，檔名】 | 誤導 A7 的實作者以為還有「每輪重放」收斂 | **接受**（舊碼不搬）；新模組不寫這類註解 |
| SD23 | 檢舉去重與限流是 check-then-insert，無唯一約束，並發時可重複建立；`SubmitReport` 不驗證目標存在也不禁止舉報自己 | `report.go:100-119`；`reports` 表只有索引（`schema.sql:102-104`）；`ResolveOwner` 的錯誤被忽略（`report.go:113-117`）【量/推】 | 同一對象可得到多筆檢舉；不存在的目標也可建單；目標作者為 0 時 BAN 不可用（`ErrUnknownTargetOwner`） | **修**（moderation 的提交先驗證目標存在且可見；去重用唯一鍵） |
| SD24 | 貼文的 `product_id` 不驗證存在，也不驗證屬於該店 | `post.go:131-219` 沒有呼叫 `productClient`；gateway `G/handler/post.go:147-160` 只轉傳【推：沒有讀 gateway 其他驗證層】 | 可連結到別家的商品或不存在的商品；讀時 `postProductFromPointer` 靜默回 nil（`handler.go:116,262-267`） | **修**（寫入時經 catalog 驗證歸屬；A7 內可在 tx 前查） |
| SD25 | 停權旗標來自 gRPC metadata，讀取 fail-open | `handler.go:25-33` | 旗標漏傳就不擋；單體內改為直接向 seller 查能力即可 | **接受**（D3 取代） |

---

## 7. 本切片浮現、brief 沒有回答的設計題

不重問 A7-D1..D5′ 已決定的項目（service 分法、封鎖歸屬與效果、停權能力分類、可見性欄位 `viewer_can_*`、moderation 的範圍與店家處分走措施）。

**Q1　排程發布的代行稽核（brief 說「改成發事件」）怎麼落地？**
`RecordAuditIntent` 是請求／回應：回傳 `AuditLogID`、冪等鍵去重，並在 session 已撤銷時回 9271 讓 worker 取消排程（`post_schedule_worker.go:160-174, 248-283`）；`ConfirmAuditResult` 是第二階段（`:289-329`）。單向事件取代不了「intent 失敗就不發布」。單體的 `delegation` 依賴 `social`（`internal/arch/modules.go`），所以 social 不能 import delegation。選項：(a) social 定義一個 `ScheduledPublishGate` port，由 `cmd/hawkr` 接線注入 delegation 的實作（社群模組不 import delegation，但仍同步呼叫）；(b) 排程的執行編排整個移給 delegation（A10），social 只提供「發布」API，delegation 的 River job 到點時先稽核再呼叫；(c) 事件加「session 撤銷時 delegation 主動取消排程」，到點不再詢問。建議待 A7-0 與 delegation 擁有者定；(b) 最符合依賴方向，但代價是排程功能要等 A10 或在 A7 暫時關閉。

**Q2　`reports.target_id` 與處置的目標型別。**
目標涵蓋 social（UUIDv7 若採 M1）、review、im（bigint，A6 前在 legacy）。要用 `text`＋`target_type`，還是 (type, bigint/uuid) 兩欄？並且 A6 搬完後是否換成各模組的 UUID。

**Q3　貼文公開的觸發點與 fan-out，以及排程的實作。**
(a) `post.created` 改在「轉為公開」時發（SD8），含互動式發布與排程發布；(b) 事件發一次、由 notification（A8）展開粉絲，還是 A7 relay 繼續展開成每粉絲一則（舊形狀，brief 要求 A8 前不變）；(c) 排程用 River scheduled job 取代輪詢＋租約＋六態欄位（M7），並且「到點時再檢查」刪除、平台下架、曝光、session 狀態。

**Q4　計數器策略與 `view/share_count` 的去留。**
單體同一個 DB：同 tx 更新、讀時 `COUNT`、還是保留非同步？熱點貼文的 row lock 疑慮（ADR `2026-09-18-mq-social-counter-async-delta`）要不要量測。`view_count`／`share_count` 沒有去重也沒有消費端驗證（SD18）：保留、改成不精確的取樣、還是移除（需要產品確認）。

**Q5　hashtag：計數、字元集、搜尋。**
(a) `post_count` 是否仍維護（維護＝SD16 那類漂移，沒有對帳；導出＝每次自動完成要 join 貼文與店家曝光）；(b) 字元集：現在只允許 `a-z0-9` 與漢字（`validator.go:73-81`），假名、韓文、底線、全形英數都被拒，zh-TW／en 兩語系是否足夠；(c) 「包含」搜尋用 pg_trgm（`hashtag.sql:60-65`），CJK 的 trigram 行為取決於資料庫 locale，單體的 Postgres 18 locale 是否支援要實測；(d) 趨勢以綁定時間（`create_time`）還是發布時間分窗（現在排程貼文以草稿時間計，`hashtag.sql:67-79`）。

**Q6　內容被處理（隱藏）之後的語意。**
目前被隱藏的留言／討論區回覆沒人過濾（SD2）、也不能還原（SD13）。需要決定：作者自己能不能看見被隱藏的內容與理由（DSA 17）；駁回／重開時是否自動還原；被隱藏的內容能否編輯；買家內容的處理紀錄放 moderation 時，還原由誰觸發。brief 文末「買家對內容處理的申訴」只決定了「先通知作者並附理由」。

**Q7　封鎖的讀取模型。**
單體規則 2 禁止跨 schema join，而貼文、留言、討論區的列表必須在查詢內排除封鎖關係（SD15：分頁之後過濾不可接受）。選項：(a) social 維護封鎖讀取模型，由 buyer／seller 的封鎖事件在同 tx 更新（規則 2 的標準做法）；(b) 每頁取回後呼叫 buyer／seller 批次判斷（等於分頁後過濾）；(c) 兩個封鎖表就近放在 social（與 D2「就近放在發起者」矛盾）。需與 buyer、seller 兩模組的事件形狀一起定。

**Q8　moderation 的處置紀錄與證據。**
`moderation_actions` 目前是寫不讀的表（SD12）；`reports.target_snapshot` 從未寫（SD21）。新模組要：處置與稽核同 tx、狀態與動作的 CHECK、提交時凍結內容快照、`APPEALED` 是「重開」還是真正的申訴狀態（買家申訴規則未定，brief 文末第 1 項）。另外要確認 `AdminListPosts` 的檢舉數 join（`posts.sql:60-90`）在 reports 搬到 moderation 後怎麼做（讀取模型，或後台直接問 moderation，brief 的平台佇列是否涵蓋）。

**Q9　反正規化與刪除保留。**
(a) `cms_comment`／`cms_seller_forum` 的 `user_name`、`user_avatar`、`reply_to_name` 快照欄（M19）是否完全拿掉，只在讀時補；買家刪除後 PII 不留在列上（`buyer_deleted.go:22` 只軟刪，名稱欄仍在）；(b) 軟刪貼文「永不釋放媒體引用」（M15）在單體是否改成保留期後釋放；(c) 軟刪列的保留期與清理 job。

**Q10　分頁、總數與留言版本。**
Connect 用 AIP-158 keyset：留言按時間倒序（頂層與回覆）、討論區回覆正序（`comments.sql:7,17`；`forum.sql:14`），要不要在 API 提供總數（舊 REST 有 `total`）。留言與討論區沒有版本，是否加 `etag`（M5）以避免編輯覆蓋。

**Q11　即時留言 server streaming 的來源與重播。**
舊 SSE：gateway 每 pod 一個 queue、每 topic 一個 5 分鐘環形緩衝、id 是 pod 內序號（M10、SD20）。單體若多 replica，fan-out 來源是什麼（程序內 channel 只適用單 replica；多 replica 需 Postgres LISTEN/NOTIFY 或 RabbitMQ）；Connect streaming 的續傳是否需要（還是斷線後重拉列表）；去重鍵是否沿用 `clientRequestId`。

**Q12　髒字詞典的歸屬與範圍。**
硬編碼 14 個子字串（`pkg/moderation/dictionary.go:12-24`），Phase 2「DB 驅動」從未做；與 im、review 共用同一份。要不要在 A7 保留這個預先過濾、放哪個模組（`moderation`？`platform`？）、是否所有寫入路徑（含編輯，SD6）都經過、命中時是否寫處置紀錄（`AUTO_REJECT` 的原意，從未實作）。

---

## 8. 量了什麼、推了什麼、沒涵蓋什麼

**量（打開檔案或跑過指令）**：10 張表與所有欄位／約束（`schema.sql` 全檔）；107 個 sqlc 查詢的 `posts/comments/forum/hashtag/reports/moderation/blocks/post_schedule` 檔；`SI/usecase` 的 `post.go`、`comment.go`、`forum.go`、`block.go`、`dispatcher.go`、`report.go`、`hashtag.go`、`post_hider.go`、`actor.go`、`errors.go`、`comment_snapshot.go`；`SI/grpc/handler.go`（全檔）、`handler_moderation.go`、`blocklist_filter.go`；`SI/worker/*.go`（含 `post_schedule_worker.go` 全檔）；`SI/mq/publisher.go`；`pkg/blocklist/blocklist.go`；`pkg/moderation/dictionary.go`；`SI/domain/hashtag/*`；`G/sse/ringbuffer.go`、`hub.go` 的 Subscribe／Publish／flushBatch／nextEventID；`events.csv` 的 social 列；brief、ADR 10-17、A3 buyer 盤點格式。

**推（讀程式推論，沒有實際操作）**：SD4、SD9（亂碼機率）、SD10 的 account_id 誤配、SD17 的雙擊競態、SD20、SD23 的並發、SD24（沒有讀 gateway 其他驗證層）；§4 外部 provider 的內部授權描述；`seller.deleted` 只在 `StoreRetired` 才發（來自 seller CARRYOVER 文件，不是我讀 seller 原始碼）。

**沒涵蓋**：`SI/usecase/delegation_audit_client.go` 與 `SI/client/account/delegation.go` 的線上協定細節（只讀了 worker 側）；`SI/repository/postgres/post_schedule_repository.go` 的 Claim／Publish SQL 全文（只讀了 `post_schedule.sql` 前 60 行）；`SI/domain/post_schedule.go`、`SI/config/config.go` 的 Validate 不等式；gateway 的其他社群 handler（`G/handler/post.go` 其餘、`share.go`、`sse_stream.go`、`admin_delegated_post.go`）與 `G/grpc/social_client.go`；RPC／REST／四端 client 呼叫點（別的切片）；`pkg/mq` 的 publisher 內部（重試、confirm、持久化）；實際執行任何測試或服務（唯讀）；pg_trgm 在 CJK 的實際行為（Q5(c)）；單體 Postgres 的 locale。
