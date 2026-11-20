# A6-2a im 模組（資料與送出）：實作設計 v1

- 對象：計畫 `../2026-11-19-a6-review-im-crm.md` 的步驟 A6-2a。計畫的決定（A6-D1、A6-D3、A6-D5、A6-D9、A6-D13）、預設與不變量是上位規則，本檔只把它們落到模組上；本檔與計畫衝突時，以計畫為準，並在 §9 列出本檔提議的計畫修改。來源沒有定案的取捨列在 §10，等 Will 決定。
- 快照：dev `418e7d77`（2026-11-20）。
- 標記：【量】＝打開檔案或跑過指令；【推】＝讀程式後推論。舊行為的出處都是本檔重新讀原始碼得到的 `file:line`；盤點裡的行號與這裡不同時，以這裡為準。
- 路徑縮寫：`M=backend/internal`、`IM=backend/services/im-service`、`IMi=IM/internal`、`J=backend/services/java`。

## 0. 前提題：這一步的程式之後會不會被重寫、搬走或刪除？

模組本體不會，它就是終態。過渡物有三個，各自有退場點：
- 訂單查詢的 `Orders` port，由 `M/legacy/orderservice` 實作（ordering 搬遷時改成 in-process）。
- `M/legacy/imrelay`（A8 notification 搬進單體時刪除）。
- 「進行中訂單」的判定依賴 Java 能回答什麼（§3.3、§10 W1）；ordering 搬遷後改成讀 ordering 的 `pub_*` 或 API。

即時通道（hub、streaming、presence）屬 A6-2b。本步的 `Send` 只負責發一個模組事件，2b 的 hub 訂閱它，所以 2b 不必改 2a 的寫入路徑。

## 0.1 範圍

- 新模組 `M/modules/im`（`make new-module name=im from=im-service`）：schema、送出（含守門）、對話與訊息讀取（2a 需要的部分，§5）、已讀、刪帳與退場的抹除、給 A6-4 的隱藏與還原 API 骨架（資料形狀，§1.3）。
- `M/legacy/orderservice` 加兩支方法：`OrderParties`（呼叫 `GetOrderForAfterSales`）、`HasOrderBetween`（§3.3）。
- `M/legacy/imrelay`（新）：`chat.message.sent` 照舊 wire 形狀送 `hawkr.im`（§6）。
- `CARRYOVER.md`、`AGENTS.md`、`cmd/hawkr` 註冊（`modules.go`、`trimmedColumns`、worker、刪帳與退場訂閱者）。
- `accountdeletion_lockorder_integration_test.go` 加 im 的情境（§8）。

## 0.2 不在範圍

- 即時通道、hub、presence、停權時強制斷線（A6-2b；ID2、ID4 在 2b）。
- Connect 與 handler（A6-5）；moderation 的聊天訊息對象（A6-4，本步只提供 `Hide`／`Restore` 根套件 API 與隱藏紀錄表）；client（A6-6）。
- 本步沒有對外入口，舊 im-service 照常服務 gateway。`imrelay` 只在測試中啟用（計畫不變量「並行期單一生效者」），A6-6e 才在 `cmd/hawkr` 接上。
- 特性測試：im 沒有 characterization 目錄（同 review，`step-1-design.md` §0.2）；骨架在 A6-5 隨 Connect driver 一起建。A6-2a 刻意改的行為（ID1、ID3、ID5–ID9、本檔 ★）在特性測試裡標成預期差異。

## 0.3 決策總表（都已由計畫、ADR 或單體規則決定，這裡只寫落點）

| # | 題 | 落點 | 出處 |
|---|---|---|---|
| E1 | ID | 對話、訊息、隱藏紀錄的 `id` 用 UUIDv7；`buyer_id`、`store_id` 用 bigint（外部 ID） | 計畫前提「ID 型別」；ADR `2026-09-25-row-identity-by-data-kind` |
| E2 | 對話鍵 | `UNIQUE(buyer_id, store_id)`，刪掉 `room_id` 字串（M2） | `domain-inventory.md` M2 |
| E3 | 訊息順序 | 每個對話一個 `seq`，在對話列上配發（§2） | 計畫 A6-2a；M3 |
| E4 | 未讀 | `last_seq - <自己這一方>_last_read_seq`，兩個游標都在對話列上，沒有 participants 表（§1.2） | 計畫「預設」；開工 design-review 第 1 項 |
| E5 | 分頁 | 訊息以 `seq` 做 keyset；對話列表以 `(last_message_at DESC, id DESC)` 做 keyset；都不回總數 | 計畫「預設」；ID9 |
| E6 | 送出守門 | 停權 → 停業 → 封鎖，封鎖有 A6-D1 例外，查詢失敗一律回錯誤（§3） | 計畫 A6-2a；A6-D1；ID5 |
| E7 | 事件 | 送出在同一個 transaction 發 `MessageSent`；relay 與 2b 的 hub 都訂閱它 | 單體規則 4；ID7 |
| E8 | 媒體 | 照 review 與 social 的 `mediaref`：transaction 外 `Finalize`，transaction 內寫列並排同步 job；抹除時記退場、排同步 | `M/modules/review/internal/purge/purge.go:161-173`；ID8 |
| E9 | 刪帳 | 訂閱 buyer、seller 的事件（A6-D9），在同一個 transaction 內只寫複本並排抹除 job，其他照 A6-1c（§4） | A6-D9、A6-D13 |
| E10 | 詞典 | `M/platform/textfilter` | 計畫範圍 |
| E11 | 長度 | 文字以 rune 計，上限 4096（舊碼 `IMi/usecase/message.go:40,397`、`IM/db/schema/schema.sql:33`，兩處一致，照舊） | 舊行為 |
| E12 | 讀取不改狀態 | 讀取方法不寫 `*_last_read_seq`，已讀只走 `MarkRead` | 計畫不變量（NO_SIDE_EFFECTS） |
| E13 | 已讀游標放哪 | 對話列上的 `buyer_last_read_seq`、`store_last_read_seq`，不建 participants 表。理由：聊天永遠只有買家與店家兩方（§3.2「聊天只有買家與店家兩方」），沒有多人對話的需求；參與者也沒有已讀游標以外的屬性。另建一張表會讓 `(kind, party_id)` 與 `conversations.buyer_id/store_id` 存兩份，又沒有約束保證兩者一致、保證剛好兩列，正是 §1.1 刪 `room_id` 的同一個理由。放在對話列上還讓「寄件人推進自己的已讀」併進配發 seq 的同一句 UPDATE（§2） | 開工 design-review 第 1 項 |

## 0.4 查證過的現況【量】

**舊 im-service**
- 房名是 `"b%d_s%d"`（`IMi/usecase/message.go:234-236`），不是盤點寫的 `<buyerID>_<sellerID>`；`ParseRoomID` 在 `:239-253`。
- `seq_id` 的配發方式：先 `pg_advisory_xact_lock(hashtext(room_id))`（`IM/db/queries/query.sql:64-67`，`IMi/repository/postgres/message.go:34-38`），再 `INSERT … seq_id = (SELECT COALESCE(MAX(seq_id),0)+1 …)`（`query.sql:69-84`）；另有 `UNIQUE(room_id, seq_id)`（`IM/db/schema/schema.sql:50`）。
- 送出的順序（`message.go:388-682`）：長度（:397）→ 解析房名（:400）→ 兩端 principal 嚴格解析，失敗回 Unavailable，解析不出來回 InvalidInput（:413-445）→ 同一人擋（:446-448）→ 停權賣家的訂單綁定（:465-472）→ 停業，**查詢失敗放行**（:477-493，ID5）→ 封鎖，讀 Redis 的 `pkg/blocklist`，雙向、沒有訂單例外（:497-499）→ 收件人已刪帳的 Redis 標記，fail-open（:511-515）→ 詞典，只查文字（:518-522）→ 圖片 `FinalizeAndLink`（:526-537）→ 插入訊息（:547-561）→ upsert 對話，**錯誤只記 log**（:563-565）→ 綁定訂單，best-effort（:569-574）→ 未讀 +1，**錯誤丟棄**（:577-581）→ hub 推送（:625-640）→ `go` 協程發 `chat.message.sent`（:642-679，ID7）。
- 訊息與對話的寫入不在同一個 transaction：`CreateWithLock` 只包 lock＋insert（`repository/postgres/message.go:34-`），對話 upsert、未讀、綁定各自獨立執行【推】。
- 停權時的訂單例外（`resolveOrderBinding`，`message.go:707-737`）：對話已綁定就放行；否則路徑①用 client 帶的 `order_no`，經 `GetOrderParties`（實際呼叫 `GetOrderForAfterSales`，`IMi/client/order/client.go:49-73`）三方比對，再加 `HasOrderBetween`；路徑②直接 `HasOrderBetween`。任何錯誤都視為「無法驗證」→ 擋（`:704-706`）。
- `HasOrderBetween` 在 Java 的判準是 `status NOT IN ('0','80')`，只回最近一筆的 order_no（`J/order-service/.../repository/OrderRepository.java:1220-1235`；`OrderGrpcService.java:2180-2195`）。所以它把已完成（50）、部分退款（60）、全額退款（70）也算成「有關聯訂單」（狀態表 `J/shared/common-core/.../code/OrderStatus.java:15-26`）。**它回答不了「進行中」**（§3.3）。
- `GetOrderForAfterSales` 查無訂單時回 body 內 `code=NOT_FOUND`，不是 gRPC 錯誤（`OrderGrpcService.java:2082-2089`）；回應的 `OrderAfterSalesView` 有 `OrderNo`、`BuyerId`、`StoreId`、`Status`（string）等，也有買家姓名、電話、地址（`M/legacy/gen/order/order.pb.go:8338`）。
- 售後的「活躍 case」只有 `aftersales.HasActiveCase(order_no)`（`backend/api/proto/aftersales/aftersales.proto:79,429-438`），單體的 `M/legacy/gen` 沒有 aftersales（`ls M/legacy/gen`：account、buyer、common、crm、media、order、product、review、seller）。
- 刪帳：`buyer.deleted`、`seller.deleted`、`account.deleted` 三個 consumer 都只拿 `AccountID` 呼叫 `RedactDeletedAccount`（`IMi/worker/buyer_deleted.go:27`、`seller_deleted.go:21`、`account_deleted.go:20`）；後者列舉該帳號**所有歷史** buyer_id 與 store_id，再加上 accountID 本身（`message.go:1060-1069,1112-1124`），清空它們寄出的訊息（`query.sql:148-159`：`content=''`、`deleted_at=NOW()`），把對話預覽改成墓碑（`query.sql:161-171`），釋放圖片引用（`message.go:1078-1092`），並寫 Redis 標記（`:1093-1106`）。
- 預覽：`computePreview`（`message.go:74-93`）把預覽算好存進 `messages.preview_text` 與 `conversations.last_message_preview`；卡片訊息的預覽是 client 帶的 `extra["preview"]`（`:346-354`）。讀取時被隱藏或已刪的內容由 `TombstoneContent`（`:55-64`）換掉，但對話列表直接回 `last_message_content`（ID1）。
- 歷史讀取：`GetChatHistory` 用請求帶的 `user_id`／`target_id` 組兩種房名，第一種讀不到就試第二種（`IMi/grpc/server.go:211-246`，ID6）；`GetHistory` 在 `beforeID=0` 時走 OFFSET 0，之後走 `id` 游標（`message.go:741-757`，ID9）。
- 事件形狀 `ChatMessageSentEvent` 定義在 `IMi/usecase/message.go:103-114`（**不在** `backend/pkg/mq/events.go`：`grep -n -i chat backend/pkg/mq/events.go` 沒有命中）；exchange `hawkr.im`、routing key `chat.message.sent`（`IMi/mq/chat_event_publisher.go:16,20`）；golden 在 `backend/tests/contract/golden/event_payload_keys.json:585-607`（10 個 key；`receiver_id` 的身分空間是買家用 buyer_id、賣家用 account_id）。賣家收件人填 account_id 的原因寫在 `message.go:650-661`。

**單體**
- 依賴表 `im → ordering, buyer, seller, media`（`M/arch/modules.go:30`），**沒有 identity**；`lockorder.Modules` 有 `im`，位置在 review 之後、crm 之前（`M/platform/lockorder/lockorder.go:47-48`）。
- buyer：`IsBlockingStore`（`M/modules/buyer/buyer.go:579`）、`IsBlockedBetween`（`:584`）、`BatchDisplay`（`:233`）、`ProfileByBuyerID`（`:215`）；事件 `BuyerDeletionRequested`、`BuyerDeleted`（`M/modules/buyer/events.go:25-28`；欄位 `BuyerID, AccountID`，`internal/model/model.go:306-318`），發布點在 `buyer/internal/deletion/deletion.go:65,135`。
- seller：`StoreBlocksBuyer`（`M/modules/seller/blocks.go:41`）、`Store`（`seller.go:469`）、`Authorize`（`:428`）、`CapChatSend`（`:441`）。`CapChatSend` 只需要 `RequirementActive`，停權時仍然可以用（`seller/internal/model/capability.go:73`；`RequirementNotSuspended` 只出現在別的能力，例如 `:78`）；`Authorize` 遇到缺 `RequirementActive` 時回 NotFound「店家已關閉」（`capability.go:180-181`）。
- 店家的生命週期事件：`StorePendingDeletion`（隨帳號刪除進入等待，`seller/internal/model/model.go:373-379`，發布點 `closing/closing.go:94`）；`StoreClosed`（可逆，「不是下游刪除資料的訊號」，`model.go:381-387`）；`StoreRetired`（不可逆，「店家的所有下游資料可以刪除」，`model.go:396-402`，在 seller purge job 的 transaction 內發布，`seller/internal/purge/purge.go:175-177`）。
- identity 沒有「解除連結某個登入方式」的事件或方法（`grep -n -i unlink M/modules/identity/identity.go` 沒有命中）；identity 只發 `AccountCreated`、`AccountDeleted`、`AdminRoleRevoked`（`M/modules/identity/events.go:6-12`）。
- accountdeletion 依序鎖 buyer、seller，再關閉 buyer 與 seller（`M/modules/accountdeletion/internal/request/request.go:368-392`）；有店家時在同一個流程裡 `Seller.BeginDeletion`（`:250-254`）。
- review 的 A6-1c 實作是這裡要照的範本：`review.LockAuthor` 用 `pg_advisory_xact_lock(hashtextextended('review.author:'||id,0))`（`M/modules/review/db/queries/inactive.sql`）；`Deactivate` 必須在 transaction 內，持鎖後寫 `inactive_authors` 再排 job（`review/internal/purge/purge.go:80-98`）；purge job 的唯一性排除 Running（`:71-78`）；保底掃描在 transaction 內補排（`:100-118`）；`cmd/hawkr/review.go:28-29` 現在就接刪帳訂閱者。social 另訂 `StoreRetired`（`cmd/hawkr/social.go:97`）。
- `M/legacy/orderservice` 的錯誤分流範本：`ErrOrderNotFound`（`orderservice.go:170-172`）、`OrderForReview`（`:202-`：傳輸錯誤、其他 code、不合法的欄位都回一般錯誤，`code==NOT_FOUND` 才回 `ErrOrderNotFound`）。
- `accountdeletion_lockorder_integration_test.go` 已有 social（`:60`）與 review（`:102`）兩個情境。

## 1. 資料模型（`im/db/schema.sql`，Atlas）

```
im.conversations
  id               UUID PK                         -- UUIDv7（應用層）
  buyer_id         BIGINT NOT NULL CHECK > 0
  store_id         BIGINT NOT NULL CHECK > 0
  last_seq         BIGINT NOT NULL DEFAULT 0 CHECK ≥ 0   -- 已配發的最後一個 seq
  buyer_last_read_seq  BIGINT NOT NULL DEFAULT 0 CHECK (≥ 0 AND ≤ last_seq)   -- 買家讀到的最後一個 seq
  store_last_read_seq  BIGINT NOT NULL DEFAULT 0 CHECK (≥ 0 AND ≤ last_seq)   -- 店家讀到的最後一個 seq
  last_message_at  TIMESTAMPTZ NOT NULL            -- 列表排序鍵；對話只由 AllocateSeq 建立，建立時就有首則訊息的時間
  created_at       TIMESTAMPTZ NOT NULL
  UNIQUE (buyer_id, store_id)
  INDEX (buyer_id, last_message_at DESC, id DESC)、(store_id, last_message_at DESC, id DESC)   -- 列表與總未讀共用

im.messages
  id               UUID PK                         -- UUIDv7
  conversation_id  UUID FK → conversations ON DELETE CASCADE
  seq              BIGINT NOT NULL CHECK > 0
  sender_kind      TEXT NOT NULL CHECK IN ('buyer','store')
  message_type     TEXT NOT NULL CHECK IN ('text','image','card')
  content          TEXT NULL CHECK char_length ≤ 4096   -- 文字訊息的內容；抹除後為 NULL
  image_asset_id   TEXT NULL
  image_object_key TEXT NULL
  -- 卡片訊息的欄位（以及 'card' 這個 message_type 值本身）等 §10 W6 決定；下面的 preview_hint 是 W6 選 (c) 時的形狀
  preview_hint     TEXT NULL CHECK char_length ≤ 100     -- 只有卡片訊息有（client 帶的一行摘要）
  created_at       TIMESTAMPTZ NOT NULL
  erased_at        TIMESTAMPTZ NULL                     -- 刪帳／退場抹除
  UNIQUE (conversation_id, seq)
  CHECK 形狀：erased_at IS NULL 時 text/card 要有 content、image 要有 asset 與 key；erased_at IS NOT NULL 時 content、image_*、preview_hint 全為 NULL
  INDEX (conversation_id, seq DESC)

im.hides                                            -- 照 social.hides、review.hides（A6-4 用）
  id, message_id FK, reason_category, reason_text, source_ref, admin_id,
  hidden_at, restored_at, restored_by, restore_reason；還原一致性 CHECK；
  partial UNIQUE (message_id) WHERE restored_at IS NULL

im.inactive_parties (kind TEXT CHECK, party_id BIGINT, since TIMESTAMPTZ, PK (kind, party_id))   -- 刪帳者、退場店家的複本
im.media_ref_sync / im.retired_assets               -- 照 review mediaref 的同步表
```

### 1.1 和舊 schema 的差異與理由

- 沒有 `room_id`（M2）、沒有 `receiver_id`：一對一對話裡，收件人就是對話的另一方，存兩份會讓兩份資料可能不一致（舊 schema 沒有任何約束保證 room_id 與 (buyer_id, store_id) 一致，`IM/db/schema/schema.sql:1-24`）。`sender_kind` 加上對話的兩方就足以還原寄件人的 ID，而且不會有舊 `sender_id` 一欄混兩種 ID 空間的問題（`schema.sql:29-30`）。
- `message_type` 從整數 1/2（沒有 CHECK，`schema.sql:31`）改成 TEXT CHECK，並把卡片訊息獨立成 `card`。舊碼的卡片是 `type=1` 加上看起來像 JSON 的 content，靠 `computePreview` 猜（`message.go:81-84`），這正是計畫缺陷表最後一列「三端手寫聊天預覽偵測」的根源。卡片要存什麼（伺服器擁有的參照、2a 不支援、或自由 JSON 逐欄過詞典）交 §10 W6，不「先存 opaque、之後再定」。
- 沒有 `is_read`、沒有兩欄未讀計數（舊 `schema.sql:12-13,39`，三套寫法見 `query.sql:40-62,99-135`）。
- 沒有 `last_message_content`、`last_message_preview`、`preview_text`：**預覽在讀取時由最後一則訊息推導**（§5.2），所以隱藏、抹除不需要另外更新對話列，ID1 從結構上消失。計畫的寫法是「同 tx 更新對話預覽」，本檔改掉它的理由在 §9 第 1 項。
- 沒有 `order_no`、`order_binding_verified_at`（舊 `schema.sql:16-18`）：停權的訂單例外改成每次送出時查（§3.2），理由在 §10 W2。
- 沒有 `hidden_at`／`hidden_reason`，改用隱藏紀錄表（10e R12、RD3 的同一套做法）；沒有 `deleted_at`（舊欄位只有抹除在寫，`query.sql:148-159`），改名 `erased_at`，並由 CHECK 保證抹除後內容真的不在。

### 1.2 未讀

- 「我在這個對話的未讀數」＝買家是 `last_seq - buyer_last_read_seq`，店家是 `last_seq - store_last_read_seq`。這個等式成立的前提是**送出時把寄件人自己那一方的游標推到新的 seq**（★，在 §2 配發 seq 的同一句 UPDATE 裡做）：這樣大於自己游標的訊息一定是對方寄的，不必掃訊息表。舊碼送出不會歸零寄件人自己的未讀（`message.go:577-581` 只加收件人的）；業界的聊天產品（LINE、WhatsApp、Messenger）在你回覆時都把對話視為已讀，見 §9 第 2 項。
- 總未讀＝買家 `SUM(last_seq - buyer_last_read_seq) WHERE buyer_id = ?`、店家 `SUM(last_seq - store_last_read_seq) WHERE store_id = ?`，走對話列上現成的 `(buyer_id …)`／`(store_id …)` 索引，不 join 別的表。被隱藏或抹除的訊息照樣算在未讀裡（顯示時是墓碑），和舊碼一致：舊碼隱藏時不調整計數（`query.sql:143-146`）。

### 1.3 隱藏（給 A6-4）

`Hide`／`Restore` 只在本步建好 API 與表，不接 moderation。隱藏的訊息在所有讀取路徑都回墓碑（`hidden=true`，不帶內容）；預覽是讀取時推導的，所以列表同時看不到被隱藏的內容（ID1）。A6-4 的 mutation「隱藏時不更新對話預覽」在這個設計下改成：「讓預覽推導略過隱藏述詞，測試要失敗」（§9 第 1 項）。

## 2. seq 的配發（不會有競態）

送出在同一個 transaction 內，用一句 SQL 同時建立對話（如果還不存在）、配發 seq，並把寄件人那一方的已讀游標推到新的 seq（★）：

```sql
-- name: AllocateSeq :one
INSERT INTO im.conversations (id, buyer_id, store_id, last_seq, buyer_last_read_seq, store_last_read_seq, last_message_at, created_at)
VALUES (@new_id, @buyer_id, @store_id, 1,
        CASE WHEN @sender_kind = 'buyer' THEN 1 ELSE 0 END,
        CASE WHEN @sender_kind = 'store' THEN 1 ELSE 0 END,
        @now, @now)
ON CONFLICT (buyer_id, store_id) DO UPDATE
  SET last_seq = im.conversations.last_seq + 1,
      buyer_last_read_seq = CASE WHEN @sender_kind = 'buyer' THEN im.conversations.last_seq + 1 ELSE im.conversations.buyer_last_read_seq END,
      store_last_read_seq = CASE WHEN @sender_kind = 'store' THEN im.conversations.last_seq + 1 ELSE im.conversations.store_last_read_seq END,
      last_message_at = EXCLUDED.last_message_at
RETURNING id, last_seq, (xmax = 0) AS created;
```

- 同一個對話的並行送出會在 `ON CONFLICT DO UPDATE` 的列鎖上排隊，所以拿到的 seq 一定不同而且連續；`UNIQUE(conversation_id, seq)` 是最終保證。舊碼的 `hashtext` 鎖是 32 位元雜湊，碰撞時會讓不相干的房間互相等（`domain-inventory.md` §1.2【推】），新寫法鎖的是對話列本身。
- 首則訊息時兩個並行請求會建立同一個對話：`ON CONFLICT` 讓後到的那一個變成 UPDATE，不會撞唯一鍵，也就不會碰到單體規則 3「撞唯一鍵後不在同一個 transaction 繼續下 SQL」的情況。首則訊息不必另外插入任何列：已讀游標就在這一列上。
- `SET` 裡的 `im.conversations.last_seq` 指更新前的值（PostgreSQL 的 UPDATE 右側一律讀舊列），所以寄件人的游標與新的 `last_seq` 相等；CHECK `≤ last_seq` 對新列求值，會通過。
- 這是送出 transaction 的第一個 im 列鎖（§3.4），在它之前只有 advisory lock（§4.3）。
- seq 不會回收：transaction 回滾時 `last_seq` 跟著回滾，所以 seq 沒有空洞【推：同一列的 UPDATE 與 INSERT 在同一個 transaction】。2b 的補洞游標依賴這一點。

## 3. 送出 `Send`

### 3.1 輸入與身分

```go
type Sender struct { Kind PartyKind; BuyerID int64; Store *seller.Store } // httpapi 從 session 解析，模組不接受 client 帶的寄件人
type SendInput struct {
    Sender         Sender
    ConversationID uuid.UUID // 既有對話；或
    CounterpartID  int64     // 首則訊息：買家寄時是 store_id，店家寄時是 buyer_id
    Type           MessageType
    Text           string    // text、card
    PreviewHint    string    // card 必填，≤100 rune
    StagedToken    string    // image
}
```

- 寄件人由 session 決定，對話以 ID 授權：給 `ConversationID` 時，寄件人必須是該對話的那一方（不是的話回 NotFound，不洩漏對話存在）。這取代舊碼「用 user_id、target_id 猜兩種房名」的做法（ID6；舊碼送出路徑 `message.go:358-385`、讀取路徑 `server.go:231-246`）。
- 同一人擋（舊碼 `message.go:446-448`）：買家的 account_id 等於店家的 `Store.AccountID` 時回 `SELF_MESSAGE_NOT_ALLOWED`。買家的 account_id 從 session 拿（買家寄）或 `buyer.AccountIDByBuyerID`（店家寄；`M/modules/buyer/buyer.go:228-231`，「讀取歷史或現任 stint 所屬的帳號鍵，不曝露個資」）【量】。用它而不是 `ProfileByBuyerID`：只要帳號鍵，不讀個資；買家是否存在仍由 G4 與對方不存在的檢查負責。舊碼的 principal 嚴格解析（`message.go:404-445`）不再需要：單體內 buyer_id、store_id 就是業務鍵（M14）。
- 對方不存在：店家 `seller.Store` 回 NotFound、買家 `ProfileByBuyerID` 回 NotFound 時，送出回 NotFound。

### 3.2 守門順序

舊碼的順序是停權 → 停業 → 封鎖 → 收件人已刪帳（`message.go:465-515`）。新順序保留前三項的先後（計畫指定），每一項都是 in-process 讀取，**任何查詢錯誤都回錯誤**（不放行，ID5）：

| # | 檢查 | 條件 | 例外 | 回應 |
|---|---|---|---|---|
| G1 | 停權 | 寄件人是店家且 `Store.Suspended()` | 雙方之間有「進行中」的訂單或售後（§3.3） | PermissionDenied `SELLER_SUSPENDED`（ADR `2026-09-25-api-error-model` D10；同 `capability.go:183` 的 reason） |
| G2 | 停業 | 對話店家的狀態不是營運中，雙向都擋（舊碼 `message.go:474-484`）；寄件人是店家時用 `seller.Authorize(store, CapChatSend)`，買家寄時看同一個條件 | 無 | FailedPrecondition `STORE_CLOSED`（舊 9207） |
| G3 | 封鎖 | `buyer.IsBlockingStore(buyer, store)` 或 `seller.StoreBlocksBuyer(store, buyer)`，任一方向都擋（雙向一致，A6-D1） | 有「進行中」的訂單或售後（§3.3，A6-D1） | FailedPrecondition `CHAT_BLOCKED`（舊 9202；照舊是明確的錯誤，不做 shadow ban，`message.go:495-496`） |
| G4 | 收件人已刪除 | 對方在 `im.inactive_parties`（§4） | 無 | FailedPrecondition `RECEIVER_DELETED`（舊 9206） |
| G5 | 詞典 | 文字訊息經 `textfilter` 命中；卡片訊息的檢查範圍依 §10 W6（選 (a) 時沒有 client 文字可查，選 (c) 時每個文字欄位都查） | 無 | `MODERATION_REJECTED` |

- 停業的定義：店家狀態是 `CLOSED`（`seller/internal/model/model.go:27`）。`PENDING_DELETION` 算不算停業，見 §10 W3。
- 例外的查詢只在需要時才做：G1 或 G3 命中時才呼叫 `Orders`（§3.3），這樣大多數送出不碰 Java。
- 舊碼的 `buyer↔buyer` 封鎖（`IsBlockedBetween`）不適用：聊天只有買家與店家兩方。
- 舊碼收件人已刪帳的檢查讀 Redis 而且 fail-open（`message.go:501-515`）；新碼以 DB 的複本為準，而且在 transaction 內持鎖後再查一次寄件人（§4.3）。

### 3.3 「進行中」的定義與 `Orders` port

A6-D1 的「進行中」是「訂單未完成或售後未結案」（`decision-brief.md:10`）。Java 現在能回答的：

| 資料 | 能回答 | 不能回答 |
|---|---|---|
| `HasOrderBetween(buyer, store)` | 雙方是否有狀態 `NOT IN (0,80)` 的訂單，以及最近一筆的單號（`OrderRepository.java:1220-1235`） | 「未完成」：50、60、70 也算有；只回最近一筆，比較舊的訂單看不到 |
| `GetOrderForAfterSales(order_no)` | 單一訂單的買家、店家、狀態（`order.pb.go:8338`） | 依雙方查訂單 |
| `aftersales.HasActiveCase(order_no)` | 單一訂單是否有活躍 case（`aftersales.proto:429-438`） | 依雙方查；單體也沒有 aftersales 的生成碼 |

所以，用現有的 RPC **沒辦法精確回答**「雙方之間是否有任何一筆未完成的訂單或未結案的售後」。但售後那一半不必另查：order 保證有活躍售後 case 的訂單不會離開 40（下段），所以「進行中」＝雙方有一筆狀態 ∈ {10,20,30,32,35,40} 的訂單【推】。做法列在 §10 W1，本檔先依建議 (c) 寫介面，換成 (a) 時只換 port 的實作：

- 自動完成在完成前反查 `hasActiveCase`，有就跳過（`J/order-service/.../scheduler/AutoCompleteScheduler.java:62-69`）；買家確認收貨前同步處置進行中的 case，退貨、退款、申訴中就擋下（`J/order-service/.../service/OrderService.java:943-954`）；狀態表寫 50「不可再發起售後」（`OrderStatus.java:22`）。【量：狀態表寫 60「唯一入邊 50→60」、54「唯一入邊 35→54，無出邊」，10–40 期間的部分退款不改狀態（`OrderStatus.java:15-26`）；開工 design-review 第 2 輪核對】


```go
type Orders interface {
    // OpenOrderBetween 回報 buyer 與 store 之間是否有進行中的訂單或未結案的售後。查不到時回錯誤，呼叫端一律擋（fail-closed）。
    OpenOrderBetween(ctx context.Context, buyerID, storeID int64) (bool, error)
}
```

`M/legacy/orderservice` 依計畫補兩支方法，不管 W1 選哪一個都需要它們：
- `HasOrderBetween(ctx, buyerID, storeID) (has bool, latestOrderNo string, err error)`：傳輸錯誤、非 0 的 code、`has=true` 但單號是空的，都回一般錯誤。
- `OrderParties(ctx, orderNo) (OrderParties{OrderNo, BuyerID, StoreID int64; Status string}, error)`：呼叫 `GetOrderForAfterSales`，照 `OrderForReview` 分流（`code==NOT_FOUND` → `ErrOrderNotFound`；其他 → 一般錯誤；ID 不是正整數、單號不一致、狀態空白 → 一般錯誤）。只取四個欄位，**不把回應裡的買家姓名、電話、地址帶出這個轉接層**（最小化）。crm 的 `RecordRefund` 取同樣的四個欄位（`domain-inventory.md` §1.3），A6-3 直接用這支。
- im 哪條路徑用哪一支：W1 選 (a) 時 im 只用新的 RPC，`OrderParties`、`HasOrderBetween` 給 A6-3 crm 用，或在 (c) 時由 im 組合；選 (c) 時 `OpenOrderBetween` = `HasOrderBetween` 取最近一筆 → `OrderParties` 看狀態 ∈ {10,20,30,32,35,40}；不查 aftersales（上段），不必新增 aftersales 的生成碼與 client。
- 這些都是 Java 的呼叫，在 transaction 外做（同 `OrderForReview` 的位置，`step-1-design.md` §3.1 第 3 步），只放在 G1／G3 命中時。

### 3.4 步驟

1. 輸入驗證：文字 1..4096 rune（`trimtext.Trim` 之後不能是空的）；卡片依 §10 W6；圖片要有 token。
2. 解析對話與對方（§3.1）；寄件人不是該對話的一方 → NotFound。
3. 守門 G1–G5（transaction 外的讀取；G1／G3 命中時才呼叫 `Orders`）。
4. 圖片：transaction 外 `media.Finalize`（同 review 的 E7）。
5. `InTx`：
   1. `LockParty(sender)`（advisory，§4.3）；查 `inactive_parties`：寄件人已在 → FailedPrecondition；收件人已在 → `RECEIVER_DELETED`。
   2. `AllocateSeq`（§2）：建立或鎖住對話列、配發 seq，同一句推進寄件人那一方的已讀游標（★，§1.2）。
   3. 插入訊息；圖片排媒體同步 job。
   4. `events.Publish(MessageSent{EventID, OccurredAt, ConversationID, MessageID, Seq, SenderKind, BuyerID, StoreID})`。relay（§6）與 2b 的 hub 都訂閱它；同一個 transaction 讓「訊息寫進去了」與「事件排進去了」一起成立或一起不成立（ID7）。
6. 回傳 `Message`（含 seq）。

- 舊碼對話 upsert 與未讀的錯誤只記 log 或直接丟棄（`message.go:563-565,577-581`）；新碼全部在同一個 transaction 內，任一步失敗整筆回滾。
- 守門在 transaction 外讀、寫在 transaction 內做：封鎖或停業在兩者之間改變時，最多放行一則在改變當下送出的訊息。這和 review 的 `Submit` 一樣（審查過的取捨）；刪帳的競態是唯一會留下不該存在的資料的情況，所以只有它在 transaction 內重查（§4.3）。

## 4. 刪帳與退場的抹除（ID3、A6-D9）

### 4.1 「只有一個身分」在 im 資料裡表達得出來嗎

能【量】。舊碼的問題是：消費者只拿 `AccountID`，再把該帳號的**所有**歷史 buyer_id 與 store_id 一起抹掉（`message.go:1060-1069`）。所以 `buyer.deleted` 會連店家身分的訊息也抹掉，反過來也一樣（ID3）。新 schema 以 `(sender_kind, 對話的 buyer_id 或 store_id)` 表示寄件人，可以只抹單一身分：

| 事件 | 抹除範圍 | 什麼時候發 |
|---|---|---|
| `buyer.BuyerDeleted`、`buyer.BuyerDeletionRequested` | 這個 buyer_id 寄出的訊息 | 帳號刪除（`buyer/internal/deletion/deletion.go:65,135`） |
| `seller.StoreRetired` | 這個 store_id 寄出的訊息 | 店家退場：可能是帳號刪除，也可能只是關店後過了申覆期（`seller/internal/model/model.go:396-402`），這時帳號與買家身分都還在 |

- 帳號刪除時，accountdeletion 關閉買家，並讓店家進入刪除流程（`request.go:250-254,368-392`），店家之後退場時發 `StoreRetired`。所以「帳號刪除才全抹」不需要另外的事件：買家與店家各自的事件涵蓋了兩個身分。
- **不訂閱 identity 的事件**：identity 沒有「解除連結」的概念（§0.4），`AccountDeleted` 只有 `AccountID`，im 要再列舉歷史身分才能用它，這就是舊碼 ID3 的根源。依賴表 `im` 也沒有 identity（`modules.go:30`）。計畫寫的「identity 事件改 in-process 訂閱」改成這段，見 §9 第 3 項。
- 每個失效的 buyer 身分都會發 `BuyerDeleted`【量】，沒有缺口：
  - 一個帳號一生只有一個 buyer 身分：`CREATE UNIQUE INDEX buyers_account ON buyer.buyers (account_id)`（`M/modules/buyer/db/schema.sql:37`）；ADR `docs/adr/2026-09-30-account-created-provisions-buyer.md:36`「刪除的帳號留墓碑……不會重新成為買家」。所以沒有「重建身分（re-key）」這條路徑；`BuyerIDsByAccount`（`buyer.go:248`）的回傳實際上最多一筆。
  - 買家變成 `CLOSED` 的 SQL 只有 `Close`（`buyer/db/queries/buyers.sql:110-117`），唯一的呼叫點是 `deletion.close`（`buyer/internal/deletion/deletion.go:97`），同一個函式最後發 `BuyerDeleted`（`:135`）；它只經 `CloseForDeletion`（`deletion.go:76-78` 的註解「關閉買家只有這一條路徑」，`buyer.go:456-459`），呼叫者只有 accountdeletion（`accountdeletion/internal/request/request.go:386`）。進入 `PENDING_DELETION` 的 `BeginDeletion` 發 `BuyerDeletionRequested`（`deletion.go:65`）。
  - `ClearPII`（`buyers.sql:128-142`）只清已關閉的身分，不改狀態，不是新的失效路徑。
  - 查法：`rg -n "SET status" backend/internal/modules/buyer/db/queries`、`rg -n "queries.Close(" backend/internal/modules/buyer`、`rg -n "CloseForDeletion" backend/internal backend/cmd`。

### 4.2 訂閱者（同一個 transaction，只做原子的狀態變更）

```go
func (m *Module) OnBuyerDeletionRequested(ctx, e buyer.BuyerDeletionRequested) error // → deactivate(buyer, e.BuyerID)；時機見 §10 W4
func (m *Module) OnBuyerDeleted(ctx, e buyer.BuyerDeleted) error                     // → deactivate(buyer, e.BuyerID)
func (m *Module) OnStoreRetired(ctx, e seller.StoreRetired) error                    // → deactivate(store, e.StoreID)
```

`deactivate`：要求在 transaction 內（同 `review/internal/purge/purge.go:83-85`）→ `LockParty(kind, id)` → `INSERT INTO im.inactive_parties … ON CONFLICT DO NOTHING` → `Enqueue(im.erase_party{kind, id})`。不在訂閱者裡改訊息列：一個身分寄出的訊息數沒有上限（單體規則 4；review 的設計審查第 2 項）。

### 4.3 與送出的競態

- 競態：買家正要送出一則訊息時進入刪帳。如果送出的 transaction 在訂閱者之後 commit，而抹除 job 在送出 commit 之前就跑完，這則訊息就會留下來。
- 解法照 A6-1c：`LockParty` = `pg_advisory_xact_lock(hashtextextended('im.party:' || kind || ':' || id, 0))`，訂閱者與送出（§3.4 第 5.1 步）都持這把鎖，送出持鎖後在 transaction 內重查 `inactive_parties`。兩者因此串行：送出先 commit，抹除 job 就看得到那則訊息；訂閱者先 commit，送出就會被拒。
- 這把鎖在 im 自己的命名空間，不碰 identity、buyer、seller 的列，不進 lockorder（同 `review/db/queries/inactive.sql` 的註解）。訂閱者的 transaction 已經持有 buyer（或 seller）的列鎖；送出在持 advisory lock 之前不鎖任何 buyer、seller 的列（守門只做普通的 SELECT），所以不會反向等待【推，§8 的 lockorder 測試會量】。
- 只鎖寄件人：收件人在送出時被刪，最多留下一則**寄給**已刪除者的訊息，它是對方寫的內容，不是刪帳者的個資（舊碼也接受這一點，`message.go:508-510`）。

### 4.4 抹除 job `im.erase_party`

- 唯一鍵 `(kind, party_id)`，狀態只算 Available、Pending、Retryable、Scheduled，不含 Running（同 `review/internal/purge/purge.go:71-78`：執行中又排進來的那一次排在它後面，不會被略過）。
- 依 `id` 升序分批（每批 200），每批一個 transaction：鎖這一批訊息 → `UPDATE … SET content=NULL, image_*=NULL, preview_hint=NULL, erased_at=now WHERE erased_at IS NULL` → 圖片記退場並排媒體同步（ID8，同 `review/internal/purge/purge.go:161-173`）。訊息列保留：seq 連續，2b 的補洞才不會出現空洞（舊碼同樣保留列，`message.go:52-53`）。
- 冪等：述詞是「這個身分還有沒抹除的訊息」，重跑只處理剩下的。
- 保底掃描（A6-D13 的第 2 項，照 `review/internal/purge/purge.go:100-118`）：每 15 分鐘，在一個 transaction 內找出「在 `inactive_parties`、但還有未抹除的訊息」的身分，最多 100 個，補排 job。River 的 `Enqueue` 一律在 transaction 內（`f1b4996e` 的修正）。
- 對話列本身不刪：另一方仍看得到歷史（墓碑）。顯示名稱讀取時由 `buyer.BatchDisplay` 的 `Deleted` 決定（`step-1-design.md` §0.4），模組不存快照。

## 5. 讀取與已讀（2a 需要的部分）

### 5.1 2a 做、2a 延後

| API | 2a | 理由 |
|---|---|---|
| `ListConversations(party, after, size)` | 做 | 資料形狀與 ID1 的預覽推導要在這裡證明 |
| `ListMessages(conversationID, party, beforeSeq, size)` | 做 | ID6（以對話 ID 授權）、ID9（seq keyset） |
| `MessagesAfter(conversationID, party, afterSeq, size)` | 做 | 2b 斷線補洞的資料面；串流本身在 2b |
| `UnreadTotal(party)` | 做 | 未讀模型（§1.2） |
| `MarkRead(conversationID, party, uptoSeq)` | 做 | 寫入方法；讀取方法不改狀態（E12）。一句 UPDATE 對話列上自己那一方的游標：`= GREATEST(現值, LEAST(uptoSeq, last_seq))`，只前進不後退，也不會超過 CHECK |
| `MarkAllRead(party)` | 做 | 舊 `MarkAllRead`（`message.go:960-979`）的對應：`UPDATE … SET <自己>_last_read_seq = last_seq WHERE buyer_id = ?`（或 `store_id`） |
| 舊 `MarkMessagesRead`（以訊息 ID 標已讀，`message.go:950-956`） | 不做 | 有 `<自己這一方>_last_read_seq` 之後「讀到第 N 則」就涵蓋了它；A6-5 對照時標成併入 `MarkRead` |
| `Hide`、`Restore` | API 與表 | A6-4 接 moderation |
| 串流、presence、強制斷線 | 2b | A6-D3、A6-D5 |

- 授權：每支讀取都帶 `party`（從 session 解析的 kind 與 ID），對話不屬於他時回 NotFound。店家讀取時要求 `seller.Authorize(store, CapChatRead)`（`capability.go:74`）。
- 封鎖不過濾列表（★）：舊碼在列表時過濾掉有封鎖的對話（`message.go:797-814`）。A6-D1 讓有進行中訂單時仍然可以聊，隱藏對話會讓「能聊卻看不到」。新碼列表全部回傳，並由 httpapi 標示「無法送出」的原因（A6-5）。這是行為改變，見 §9 第 4 項。

### 5.2 預覽在讀取時推導（ID1）

`ListConversations` 以 `(c.id, c.last_seq)` join `im.messages` 取最後一則，再 `LEFT JOIN` 未還原的 `im.hides`。預覽由模組回結構化的欄位，不回句子：

```go
type Preview struct { Type MessageType; Text *string; Hidden, Erased bool; SenderKind PartyKind }
```

- `Text` 只有文字訊息才給（rune 截斷 100，`message.go:88-93` 的長度照舊）；卡片給 `PreviewHint`；圖片沒有文字；隱藏或抹除的不給任何內容。顯示用的句子（「傳送了一張圖片」、「此訊息已被移除」）由 client 的呈現層依 enum 產生（共用文案目錄，A6-5、A6-6），模組不再寫死中文（舊碼 `message.go:45-46,76,83`）。
- 推播的預覽是 relay 用同一個函式從訊息算出來的（§6），所以推播與列表不會不一致。

## 6. `M/legacy/imrelay`

- 照 `M/legacy/sellerrelay` 與 `reviewrelay`：`OnMessageSent(q)` 在同一個 transaction 排一個 job（唯一鍵：事件 ID）；worker 在 transaction 外組 payload，以 `rabbitmq.PublishArgs{EventID: 模組事件的 UUIDv7}` 送 exchange `hawkr.im`、routing key `chat.message.sent`。
- 欄位對應（wire 型別全部是 string，同 `message.go:103-114`）：

| wire key | 舊值 | 新值 |
|---|---|---|
| `message_id` | TSID 十進位字串（`:664`） | 訊息 UUID 字串（型別不變） |
| `conversation_id` | 房名 `b<id>_s<id>`（`:665`） | 對話 UUID 字串（見 §9 第 5 項） |
| `sender_id` | buyer_id 或 store_id（`:666`） | 同 |
| `sender_name`、`sender_avatar` | 買家暱稱與頭像；店家是店名、logo，沒有 logo 時用頭像（`:594-618`） | 同，經 `buyer.BatchDisplay`／`seller.Store`；查不到時送空字串（舊碼也是降級成空字串，`:598-601`） |
| `receiver_id` | 收件人是買家時送 buyer_id；是店家時送 account_id（`:650-661`） | 同（店家用 `seller.Store.AccountID`） |
| `receiver_type` | `"buyer"`／`"seller"` | 同 |
| `store_id` | 對話的 store_id（`:671`） | 同 |
| `content_preview` | `computePreview` 的結果（`:543,672`） | 由 §5.2 的 Preview 轉成同樣的中文句子（只在 relay 裡轉；relay 隨 A8 刪除；句子的來源見下） |
| `sent_at` | RFC3339（`:673`） | 同 |

- `content_preview` 的句子（「傳送了一張圖片」這類）：共用文案目錄有對應條目時取自 `M/platform/uitext`；目前沒有【量：`rg -n "chat|Chat|訊息|圖片" backend/internal/platform/uitext/catalog/zh-TW.json` 只命中社群檢舉的「垃圾訊息」（`:80-81`）】。A6-5／A6-6 為 client 加聊天預覽的目錄條目時，relay 改取同一批條目；在那之前，句子是 relay 裡的過渡物，登記進 im 的 `CARRYOVER.md` 沿用盤點表，退場點 A8（relay 刪除）。不在模組本體寫死中文。
- 訊息在 job 執行前已經被抹除或隱藏時：不送（推播不能帶出已抹除的內容）。
- wire 測試：照 sellerrelay 的 `wire_test.go`，以 `go/ast` 解析 **`backend/services/im-service/internal/usecase/message.go` 的 `ChatMessageSentEvent`**（任務說明寫的 `backend/pkg/mq/events.go` 裡沒有這個 struct，§0.4），逐欄比對 tag 與型別；golden 對 `event_payload_keys.json:585-600` 的 10 個 key。
- 只在測試中啟用：`events.Subscribe(bus, imrelay.OnMessageSent(…))` 與 relay worker 在 A6-6e 才接 `cmd/hawkr`（計畫不變量「並行期單一生效者」）。

## 7. 根套件 API、錯誤、依賴與註冊

### 7.1 根套件（`im/im.go`）

```go
type Orders interface { OpenOrderBetween(ctx, buyerID, storeID int64) (bool, error) }
type Deps struct { DB; Events; Jobs; Media media.Media; Buyer *buyer.Module; Seller *seller.Module; Orders Orders; NewID; Now; Logger }
Send, MarkRead, MarkAllRead, Hide, Restore
ListConversations, ListMessages, MessagesAfter, UnreadTotal
MessageForRelay
OnBuyerDeletionRequested, OnBuyerDeleted, OnStoreRetired
ErasePartyArgs, SweepErase, AddWorkers, PeriodicJobs（媒體同步、抹除的保底掃描）
// 事件：MessageSent
```

### 7.2 錯誤原因（`apperr` 分類＋Go 端的 reason 常數；A6-5 加進 proto）

| 舊 | 新 |
|---|---|
| 9201 `SelfMessageNotAllowed`（`message.go:360`） | InvalidArgument `SELF_MESSAGE_NOT_ALLOWED` |
| 9240 `SellerSuspendedOperationBlocked`（`:470`） | PermissionDenied `SELLER_SUSPENDED` |
| 9207 `StoreClosedMessageNotAllowed`（`:484`） | FailedPrecondition `STORE_CLOSED` |
| 9202 `BlockedMessageNotAllowed`（`:498`） | FailedPrecondition `CHAT_BLOCKED` |
| 9206 `ReceiverAccountDeleted`（`:513`） | FailedPrecondition `RECEIVER_DELETED` |
| 9210 `ModerationRejected`（`:520`） | 與 social 共用的 `MODERATION_REJECTED` |
| InvalidInput「訊息內容過長」（`:398`） | InvalidArgument（欄位 `text`） |
| Unavailable「暫時無法驗證對話身分」（`:419,431`） | Unavailable：buyer、seller 或 `Orders` 查詢失敗（ID5） |
| 一般錯誤「sender is not a participant」（`:457`） | NotFound（不洩漏對話存在） |

### 7.3 依賴表與註冊

- `M/arch/modules.go` 的 im 列不改：`ordering` 以 port 間接使用，`buyer`、`seller`、`media` 已在表上；不加 identity（§4.1）。
- `cmd/hawkr/modules.go` 登記 im；`trimmedColumns` 登記 `messages.content`（文字訊息）、`messages.preview_hint`、`hides.reason_text`。`content` 現在只放文字訊息（卡片等 W6），trim CHECK 可以不帶條件；如果 W6 讓 `content` 也放卡片，改寫成條件式 `CHECK (message_type <> 'text' OR content !~ <trimtext.Pattern>)`，測試接受【量：讀 `cmd/hawkr/trimmed_text_integration_test.go`】：
  - 它用 regex 在整條 `pg_get_constraintdef` 裡找 `<欄> !~ '<Pattern>'::text`（`:96,115-117`），不要求 CHECK 只有這一段；`arch.CheckTrimmedText` 只擋 trim 函式與另列的空白碼位（`internal/arch/sqltext.go:114-`），不擋 `OR`。
  - 行為測試以 `extra` 補其他欄位讓條件成立（`trimmedColumn.extra`，`:25-29`；先例 `seller.stores.contact_name`、`seller.measure_requests.decision_reason`，`:47,51-52`），登記時帶 `extra: {"message_type": "text"}`。
  - 沒跑測試（要 Docker，本次不起容器）；實作時 `TestTrimmedTextChecksUseTheCanonicalPattern` 會量。
- 現在就接：`OnBuyerDeletionRequested`、`OnBuyerDeleted`、`OnStoreRetired` 的訂閱，以及抹除 job、保底掃描、媒體同步 worker（同 `cmd/hawkr/review.go:21-29` 的理由：A6-6e 前 im 的表沒有寫入者，訂閱接上沒有副作用）。
- A6-6e 才接：`imrelay` 的訂閱與 worker。

## 8. 測試

- **守門表格測試**（計畫的驗收）：寄件人 {買家, 店家} × 封鎖 {無, 買家封鎖店, 店封鎖買家} × 進行中 {無, 有, 查詢失敗}，期待值：無封鎖時放行；有封鎖且無進行中時 `CHAT_BLOCKED`；有封鎖且有進行中時放行；查詢失敗時 Unavailable。另外加停權 {否, 是} × 進行中，以及停業 × 兩個方向。
- **ID 回歸**（integration，`pg.NewDB`，CI）：
  - ID1：隱藏最後一則之後，列表的預覽 `Hidden=true` 而且不帶文字；抹除之後 `Erased=true`。
  - ID3：同一個帳號有買家與店家兩個身分；`BuyerDeleted` 之後只有買家身分寄的訊息被抹除，店家身分寄的還在；`StoreRetired` 反之。
  - ID5：`seller.Store` 或 `Orders` 回錯誤時，送出回錯誤，訊息表沒有新列。
  - ID6：A 的對話 ID 交給 B 讀或寫，回 NotFound；同一個數字同時是 buyer_id 與 store_id 時，不會讀到另一個身分空間的對話。
  - ID7：送出的 transaction 回滾時沒有 relay job；commit 時剛好一個。
  - ID8：抹除圖片訊息後，退場資產有紀錄、媒體同步 job 有排。
  - ID9：插入 25 則，以 `beforeSeq` 翻三頁，不重複、不漏；中間有新訊息插入時頁面穩定。
- **seq**：兩個 goroutine 對同一個（還不存在的）對話並行送 50 則，seq 是 1..100 且不重複，對話只有一列；結束時寄件人各自那一方的游標等於他最後一則的 seq。
- **已讀游標**：買家寄 3 則後店家的未讀是 3、買家是 0；店家 `MarkRead(uptoSeq=99)` 只推到 `last_seq`；`MarkRead` 給較小的 seq 不倒退；直接以 SQL 寫 `buyer_last_read_seq = last_seq + 1` 或 `-1` 被 CHECK 擋下。
- **刪帳競態**：送出持 `LockParty` 停住，訂閱者同時跑，後者等待；放行後抹除 job 看得到那則訊息。反過來，訂閱者先 commit，送出回錯誤。
- **抹除 job**：分批、重跑結果相同；唯一性不含 Running（執行中排入的那一次會再跑）；保底掃描補排被丟棄的 job。
- **鎖序**：`accountdeletion_lockorder_integration_test.go` 加 `TestDeletionLocksBuyerAndSellerBeforeImRows`（照 `:102` 的 review 版）：買家刪除時 im 的列在 buyer、seller 之後才鎖；另加 seller 退場的情境（`StoreRetired` 在 seller purge 的 transaction 內）。
- **relay**：wire（go/ast）、golden、每個事件剛好一個發送 job。
- **讀取不改狀態**：`ListMessages`、`MessagesAfter` 跑完後兩個已讀游標都不變。
- **mutation**（在複本跑，改回舊寫法時對應的測試要失敗）：
  1. 停業查詢失敗時放行（`message.go:485-492`）→ ID5 測試。
  2. 封鎖不看例外（`message.go:497-499`）→ 表格「有封鎖＋有進行中」那格。
  3. 例外查詢失敗時放行 → 表格「查詢失敗」那格。
  4. 抹除依帳號列舉所有身分（`message.go:1069`）→ ID3 測試。
  5. 送出拿掉 `LockParty` 或 transaction 內的重查 → 刪帳競態測試。
  6. 訂閱者拿掉 `Enqueue` → 抹除測試。
  7. 抹除 job 的唯一狀態加上 Running → 唯一性測試。
  8. 保底掃描不補排 → 保底測試。
  9. 預覽推導拿掉隱藏述詞 → ID1 測試。
  10. `MessageSent` 改成 commit 後發 → ID7 測試。
  11. `ListMessages` 改成 OFFSET → ID9 測試。
  12. 讀取方法裡更新已讀游標 → 讀取不改狀態的測試。
  13. `AllocateSeq` 不推進寄件人那一方的游標 → 未讀等式測試（寄件人的未讀應為 0）。
  14. 已讀游標的 CHECK 拿掉 `≤ last_seq`，或 `MarkRead` 拿掉 `LEAST(…, last_seq)` → 已讀游標測試。

## 9. 本檔提議的計畫修改（design-review 後交 Will 或直接改）

1. **ID1 的處置**：計畫寫「同 tx 更新對話預覽」。本檔不存預覽，改成讀取時從最後一則訊息推導（§5.2）：隱藏、還原、抹除三條路徑都不必記得去更新對話列，A6-4 也不必再呼叫一個「更新預覽」的 API。代價是列表多一次以主鍵 `(conversation_id, seq)` 的 join【推：每頁最多 100 列，走唯一索引】。A6-4 的 mutation 隨之改成「預覽推導拿掉隱藏述詞」。
2. **送出把寄件人自己的對話標成已讀**（§1.2）：這是行為改變，讓「未讀＝`last_seq - 自己那一方的游標`」成立，不必掃訊息。計畫的預設只寫了 `seq > last_read_seq`，沒寫寄件人自己寄的訊息怎麼排除，本檔補上。
3. **不訂閱 identity 的事件**：計畫 A6-2a 寫「buyer／seller／identity 事件改 in-process 訂閱」。依 §4.1，identity 沒有身分層級的事件，帳號刪除已經經由 buyer 與 seller 的事件涵蓋兩個身分；依賴表也不必加 identity。建議改成「buyer、seller 事件」。另外補上 `seller.StoreRetired`：只關店、不刪帳時，店家身分的訊息由它觸發抹除（social 的前例 `cmd/hawkr/social.go:97`）。`seller.StoreSuspended` 的強制斷線屬 2b。
4. **列表不再因封鎖隱藏對話**（§5.1）：A6-D1 讓有進行中訂單時仍可以聊，所以列表顯示全部對話，由 A6-5 標示無法送出的原因。
5. **`chat.message.sent` 的值**：`conversation_id` 從房名字串變成對話 UUID，`message_id` 從 TSID 變成 UUID，wire 型別（string）不變。notification 把兩者放進推播的 data（`services/notification-service/db/migrations/20260703160000_1176_flipped_events_template_snake.sql:120`）。buyer-app 收到推播後以 `senderId` 導向聊天室（`buyer-app/lib/main.dart:337`），seller-app 與 website 有沒有讀 `conversationId` 沒有逐一確認【推】；A6-6 改 client 時一起確認。
6. **計畫 A6-2a 的「`orderservice.GetOrderForAfterSales`、`HasOrderBetween`」**：建議把方法名寫成 `OrderParties`、`HasOrderBetween`，並註明 W1 的結論決定 im 用哪一支（§3.3）。
7. **任務說明裡的 wire 來源**：`chat.message.sent` 的 struct 在 `IMi/usecase/message.go:103-114`，不在 `backend/pkg/mq/events.go`；計畫 A6-2a 的驗收文字如果寫了後者，要改。

## 10. 需要 Will 決定

### W1 「進行中訂單或未結案售後」用什麼資料判定（擋 G1、G3 的例外）

- 前提題：A6-D1 要的是精確的「進行中」，但 Java 現有的 RPC 回答不了（§3.3）：`HasOrderBetween` 把已完成與已退款也算進去，而且只回最近一筆（`ORDER BY created_at DESC, id DESC LIMIT 1`，`OrderRepository.java:1224-1231`）。
- 約束：`backend/AGENTS.md`「舊服務……只做維持運作所需的修改」。為 im 新增一支 Java RPC 是新功能，不是維持運作；選 (a) 等於為這一步開這條規則的例外，要 Will 明說。
- 查證後縮小的範圍【量／推】：售後不必跨服務問。order 保證有活躍 case 的訂單停在 40 以下（自動完成反查 `hasActiveCase` 後跳過，`AutoCompleteScheduler.java:62-69`；確認收貨被進行中的 case 擋下，`OrderService.java:943-954`；50「不可再發起售後」，`OrderStatus.java:22`），所以「進行中」＝狀態 ∈ {10,20,30,32,35,40}【量：同上，`OrderStatus.java:15-26`】。原稿 (a) 寫的「50、60 且有活躍 case」與「可能也要問 aftersales」都不需要；order 自己已經有 `AfterSalesGrpcClient`（`J/order-service/.../grpc/client/AfterSalesGrpcClient.java:67`），但這裡用不到。
- 業界基準：交易平台以「訂單未結案（含售後）」判定，並且查的是**所有**訂單，不是最近一筆（Shopee、eBay 封鎖時仍保留進行中交易的溝通，`decision-brief.md:9`）。
- 選項：

| | 做法 | 改哪裡 | 精確度 | 每次例外查詢 | 退場點 |
|---|---|---|---|---|---|
| (a) | Java order 新增 `HasOpenOrderBetween(buyer, store)`：`status IN (10,20,30,32,35,40)`，只碰 order 自己的表 | Java order（mapper＋gRPC）、proto、`make` 重新生成 Go 碼、`legacy/orderservice` | 精確（所有訂單） | 1 個 RPC | B8 ordering 搬遷時刪掉這支 RPC，改讀 ordering 的 `pub_*` 或 API：新 RPC 的壽命只到 B8 |
| (b) | 照舊碼：`HasOrderBetween` 的 `NOT IN (0,80)` | 無 | 曾成交過一筆，封鎖就永遠不生效，A6-D1 的 (ii) 變成 (iii) | 1 個 RPC | 同 B8 |
| (c) | `HasOrderBetween` 取最近一筆 → `OrderParties`（`GetOrderForAfterSales`）看狀態 ∈ {10..40} | 只有單體的 `legacy/orderservice`（兩支方法計畫本來就要補，A6-3 crm 也用） | 看不到「最近一筆已結案、較舊的一筆還在配送」；錯的方向是**多擋**（封鎖照常生效），不是放行 | 2 個 RPC，只在 G1／G3 命中時 | B8：port 換成 in-process，im 不動 |

- 建議 (c)（改變原稿的 (a)）：(a) 的精確只多涵蓋一種情況（【推：頻率沒有量，上線前沒有資料可量】同一對買賣雙方有兩筆以上訂單，而且較新的那筆先結案），卻要違反舊服務的修改規則，做出一支 B8 就刪的 RPC；(c) 錯的方向是保守的（被封鎖的一方暫時不能聊，仍可透過訂單頁與客服處理），不改 Java，port 的形狀與 (a) 相同，B8 時直接換成精確的 in-process 查詢。CARRYOVER 寫下看不到的情況與退場點 B8。如果 Will 判斷那種情況不可接受，選 (a)，並在本檔記下這是 `backend/AGENTS.md` 規則的例外。

### W2 停權賣家的例外用同一個定義嗎

- 現況：停權的店家只能在「已綁定訂單」的對話送出，綁定用的是 `NOT IN (0,80)`，綁定之後永久有效（`message.go:707-737`、`IM/db/schema/schema.sql:15-18`）。
- 選項：(i) 與 W1 用同一個「進行中」判定，每次送出時查，不存綁定欄位；(ii) 照舊：曾經有關聯訂單就永久放行，保留 `order_no` 與綁定時間兩欄。
- 業界：停權通常只保留履約必要的溝通（未結案訂單），結案後不再有例外。
- 建議 (i)：一個定義、兩個例外共用，少兩個欄位與一條 client 帶 `order_no` 的「意圖參數」路徑（`message.go:330-336`）。

### W3 店家「等待刪除」（PENDING_DELETION）時可不可以聊

- 事實：店家隨帳號刪除進入 `PENDING_DELETION` 時，「不再做經營類的事」（`seller/internal/model/model.go:373-374`），但帳號會等開啟中的訂單結束才刪；`CapChatSend` 只要求 `RequirementActive`，與 `RequirementNotPendingDeletion` 分開列（`capability.go:73,77`），表示 seller 的設計允許等待刪除的店家聊天【推：`Conditions.Active` 的推導沒有逐行讀】。舊碼只擋 `closed`（`message.go:483`）。
- 選項：(i) 只有 `CLOSED` 算停業，等待刪除時雙向都可以聊；(ii) 等待刪除也算停業。
- 建議 (i)：等待刪除的店家正在把開啟中的訂單出貨完，買家需要能聯絡；與 seller 的能力表一致。

### W4 買家申請刪帳後，抹除從什麼時候開始

- 事實：`BuyerDeletionRequested` 在買家進入 PENDING_DELETION（還有開啟中的訂單）時發；`BuyerDeleted` 在關閉時發（`buyer/internal/model/model.go:312-318`）。review 與 social 都在申請時就讓內容不可見（`review/internal/purge/purge.go:1-3`、`social.go:625-630`）。
- 選項：(i) 照 review 與 social：申請時就寫複本、排抹除，之後這位買家不能再送訊息；(ii) 申請時不動，`BuyerDeleted` 才寫複本、排抹除，等待期間仍可以為開啟中的訂單聯絡店家。
- 業界：刪帳申請期間通常仍允許處理未完成的交易。
- 建議 (ii)：聊天是處理開啟中訂單的管道，和評價、貼文這類公開內容不同。選 (ii) 時只訂 `BuyerDeleted`（沒有開啟中的訂單時只發它，`social.go:632-633` 的註解），§4.2 的 `OnBuyerDeletionRequested` 拿掉。前提是等待刪除的買家還能登入，這一點沒有驗證【推】，實作開工時確認；如果不能登入，(i)、(ii) 沒有差別，選 (i) 與其他模組一致。

### W5 店家能不能主動開啟對話

- 事實：舊碼允許店家對任何買家送出首則訊息並建立對話（`message.go:373-382`）。
- 選項：(i) 照舊；(ii) 只有買家能開啟對話，店家只能回覆既有對話；(iii) 店家只能對有訂單關係的買家開啟。
- 業界：市集平台多半只讓買家開啟，或要求有交易關係，以避免店家騷擾或廣告。
- 建議 (i) 照舊，本步不改產品行為；如果要收緊，選 (ii)，它不需要查 Java。seller-app 有「選擇聊天對象」的畫面（`seller-app/lib/.../chat_picker_remote_service.dart`，`client-inventory.md:45`），收緊時 UI 要一起改，需要先取得同意。

### W6 卡片訊息存什麼（擋 §1 `im.messages` 的卡片欄位與 `message_type` 的 `card` 值、§3.2 G5、§3.4 第 1 步、§5.2 的卡片預覽、§7.3 `content` 的 trim CHECK 寫法）

- 事實：舊碼的卡片是 client 帶的任意 JSON，預覽取 client 帶的 `extra["preview"]`（`message.go:346-354`），詞典只查文字（`:518-522`），等於一條繞過詞典的路。原稿照搬成「先存 opaque JSON、只查 hint、A6-5 再定形狀」，開工 design-review 第 2 項指出這是沒有形狀的欄位。上線前沒有相容性約束，沒有理由先存。
- 選項：
  - (a) 2a 就定卡片形狀，只存伺服器擁有的參照：`card_kind`（`product`／`order`）與被引用的 ID，顯示資料（品名、圖片、價格、訂單狀態）由伺服器讀取時組出來；client 不帶任何文字，沒有可過詞典的內容。要在 2a 決定可引用哪些對象，並檢查寄件人看得到該對象（訂單要是雙方之間的；`OrderParties` 已有）。
  - (b) 2a 不支援卡片：`message_type` 只有 `text`、`image`；卡片連同形狀在 A6-5 定 proto 時加（加一個 enum 值與欄位是往前的 migration，上線前沒有相容性負擔）。
  - (c) 照舊存自由 JSON，但每個文字欄位都過 `textfilter`，hint 也過；伺服器不知道欄位語意，只能遞迴掃所有字串。
- 業界：交易平台聊天裡的商品或訂單卡片是「參照」，內容由平台依參照即時組出（價格、狀態會變，不能信 client 的快照），例如 Shopee 聊天的商品卡【推：依公開產品行為】。
- 建議 (b)：2a 的目標是資料與送出，卡片要引用哪些對象、client 怎麼顯示屬於 A6-5 的契約；先不支援就沒有繞過詞典的路，也不留沒有形狀的欄位。A6-5 定 proto 時依 (a) 的形狀加。若 Will 要 2a 就有卡片，選 (a)；不建議 (c)（伺服器替不認識的 JSON 背詞典責任，而且顯示內容仍是 client 的快照）。
- 選 (b) 時本檔要改：§1 拿掉 `card` 值與 `preview_hint`；G5 只剩文字；§5.2 的卡片預覽延到 A6-5；`content` 的 trim CHECK 不帶條件（§7.3）。

## 11. 開工 design-review 的處理

審查全文：session scratchpad 的 `a6-2a-design-review.md`（2026-11-20）。

| # | 發現 | 嚴重度 | 處置 |
|---|---|---|---|
| 1 | `im.participants` 與對話列的 `buyer_id/store_id` 存兩份、沒有一致性約束，一對一對話不需要第二張表 | should | 改：刪 participants，`buyer_last_read_seq`、`store_last_read_seq`（CHECK `0 ≤ … ≤ last_seq`）放在對話列；寄件人推進已讀併進 `AllocateSeq` 的同一句 UPDATE（E13、§1、§1.2、§2、§3.4、§5.1、§8） |
| 2 | 卡片先存 opaque JSON、詞典只查 hint，留下繞過詞典的路 | should | 交 Will：§10 W6（建議 2a 不支援卡片） |
| 3 | 舊 buyer 身分是否都發 `BuyerDeleted` 寫成「實作開工時確認」 | should | 查完：一個帳號只有一個 buyer 身分（`UNIQUE (account_id)`），關閉只有 `CloseForDeletion` 一條路徑且必發 `BuyerDeleted`，沒有缺口（§4.1）；不需要 §9 的提議 |
| 4 | W1 的簡報沒引用「舊服務只做維持運作的修改」，也沒交代 (a) 的售後判定由誰回答 | should | 改寫 W1：補上規則；查證售後不必跨服務（活躍 case 的訂單停在 40 以下），建議改成 (c)，退場點 B8 |
| 5 | relay 把 Preview 轉回中文句子，等於共用文案目錄之外又一份顯示文字 | nit | 改：§6 寫明有目錄條目就取自 `platform/uitext`，目前沒有，登記成 CARRYOVER 的過渡物，退場點 A8 |
| 6 | trim CHECK 能否條件式、同一人檢查用哪支 API，兩件寫成「實作時確認」 | nit | 查完：測試接受條件式 CHECK（以 `extra` 補條件，§7.3）；同一人檢查用 `buyer.AccountIDByBuyerID`（`buyer.go:228-231`，§3.1） |

第 2 輪（只審第 1 輪改過的段落）：第 1 輪 fixed 的 #1、#3、#4、#5、#6 核對成立，#2 仍交 Will。新發現 3 個 nit：

| # | 發現 | 處置 |
|---|---|---|
| 7 | §1 的卡片欄位畫的是 W6 (c) 的形狀，和建議的 (b) 相反 | 駁回（nit）：§1 已標「等 W6」，W6 拍板後在 2a1 開工時依 §10 W6 最後一點改 |
| 8 | W1「少見情況」沒標證據等級 | 改：標【推】，頻率上線前無資料可量 |
| 9 | §5.1 `MarkMessagesRead` 列仍寫泛稱 `last_read_seq` | 改：寫成自己這一方的欄位 |

另依第 2 輪核對，§3.3 與 W1 的「進行中＝狀態 10–40」由【推】改成【量】（`OrderStatus.java:15-26`）。

findings 9 (blocking 0)：fixed 7、recorded 0、raised 1（W6）、rejected 1。
