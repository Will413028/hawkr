# A6-5a 對外契約設計：review、chat、crm 的 Connect 方法

快照：分支 `a6-5`（origin/dev `dca4be6f` 加計畫兩個 commit）。標記：【量】開檔或執行核對過，【推】推論。模組路徑 `M/` ＝ `backend/internal/modules/`。

## 0. 範圍與既定

- 本設計只定契約（proto 形狀、身分推導、錯誤、串流 helper 的責任）與 5b–5d 的分工；程式在 5b–5d。
- 既定不重開：A6-D3（server streaming＋unary 送出）、A6-D4（封鎖過濾，統計不濾）、A6-D8／CD8（CRM 不回聯絡資料）、A6-D14 W6（卡片是伺服器參照，client 不送文字）、A6-D15（不做打字中）、A6-D16 W1–W5、A6-D18 W3 (a)（CRM 不回優惠券）、A6-D18 W5 (a)（客戶列表拿掉關鍵字搜尋）、A6-D21 W3 (a)（店家只檢舉 `CHAT_MESSAGE`）、A6-D22。
- 命名照 `docs/conventions/api-design-guidelines-connect.md` 與 ADR `2026-10-07-connect-without-perspective` 的 10-17 修訂：計畫寫的 `ReviewService`、`ChatService`、`SellerCrmService` 是暫名，正式名見 §2。
- 並行期：舊服務在 gateway 後面處理 client 流量到 A6-6e。5b–5d 在單體註冊 Connect handler 不會讓 client 改走新路徑（client 在 A6-6 才改），系統從未上線、沒有舊資料要搬，所以單體 handler 與舊服務同時存在不違反「單一生效者」不變量；relay 與 consumer 仍到 6e 才接（計畫不變量）【推，依計畫不變量表第 9 列】。

## 1. 34 支逐支去向（取代計畫「34 扣 8」的口徑）

計畫的「8 支刪除」沿用 36 支口徑（6 支無呼叫端＋crm 已刪 2 支）【量，`interface-inventory.md:194`】。34 支內的實際去向：

| # | 舊 RPC | 去向 | 新方法 | 子步驟 |
|---|---|---|---|---|
| 1 | GetProductReviews | 對外 | `ReviewPublicService.ListProductReviews` | 5b |
| 2 | GetReview | 對外 | `ReviewAppService.GetReview` | 5b |
| 3 | ReplyReview | 對外 | `ReviewAppService.ReplyReview`（店家身分由評價所屬店推導，`CapMarketing`） | 5b |
| 4 | SubmitOrderReview | 對外 | `ReviewAppService.CreateReview` | 5b |
| 5 | ListMyOrderReviews | 對外 | `ReviewAppService.ListMyReviews`（篩選 `order_item_id`，取代 SA 翻頁找單一明細） | 5b |
| 6 | GetSellerOrderReviews | 對外，拆二 | `ReviewPublicService.ListStoreReviews`（觀看者封鎖過濾）＋`ReviewAppService.ListOwnStoreReviews`（呼叫者此刻的店，不帶 `store_id`，不濾；W1 (g)） | 5b |
| 7 | GetSellerOrderReviewStats | 對外 | `ReviewPublicService.GetStoreReviewStats`（公開與自己的店數字相同，A6-D4 統計不濾，一支即可） | 5b |
| 8 | ListStoreRatingStats | 單體內 | `review.StoreRatingStats`，6e 換接線 | — |
| 9 | HideReview | 單體內 | `review.Hide/Restore`（A6-4b） | — |
| 10 | SendMessage | 對外 | `ChatAppService.SendMessage` | 5c |
| 11 | GetChatHistory | 對外 | `ChatAppService.ListMessages` | 5c |
| 12 | GetConversations | 對外 | `ChatAppService.ListConversations` | 5c |
| 13 | GetUnreadCount | 對外 | `ChatAppService.GetUnreadCount` | 5c |
| 14 | MarkMessagesRead | 併入 #15 | `ChatAppService.MarkConversationRead(conversation_id, upto_seq)`（模組只有 `im.MarkRead` 一支） | 5c |
| 15 | MarkConversationRead | 對外 | 同上 | 5c |
| 16 | MarkAllRead | 對外 | `ChatAppService.MarkAllRead` | 5c |
| 17 | ChatStream | 對外，改 server streaming | `ChatAppService.StreamEvents` | 5c |
| 18 | HideMessage | 單體內 | `im.Hide/Restore`（A6-4b） | — |
| 19 | BroadcastMessage | 刪除 | 無呼叫端 | — |
| 20 | IsUserOnline | 刪除，由新方法接手 | `ChatAppService.GetPresence`（A6-D16 W2 輪詢） | 5c |
| 21 | GetConnectionStats | 刪除 | 無呼叫端；連線數走 metrics | — |
| 22 | DisconnectUser | 刪除 | 無呼叫端；只有關店由模組結束串流（A6-D16 W4） | — |
| 23 | GetCustomerList | 對外 | `CrmAppService.ListCustomers` | 5d |
| 24 | GetCustomerStats | 對外 | `CrmAppService.GetCustomer`（含標籤，取代 #32；暱稱頭像由 `buyer.BatchDisplay` 組） | 5d |
| 25 | UpdateCustomerStatus | 對外，縮成 VIP | `CrmAppService.UpdateCustomer`（`update_mask` 只允許 `type`，值 NORMAL／VIP；黑名單值 A6-D2 已刪） | 5d |
| 26 | CreateTag | 對外 | `CrmAppService.CreateTag` | 5d |
| 27 | ListTags | 對外 | `CrmAppService.ListTags` | 5d |
| 28 | UpdateTag | 對外 | `CrmAppService.UpdateTag`（只改名） | 5d |
| 29 | DeleteTag | 對外 | `CrmAppService.DeleteTag` | 5d |
| 30 | AddCustomerTags | 對外 | `CrmAppService.AddCustomerTags` | 5d |
| 31 | RemoveCustomerTag | 對外 | `CrmAppService.RemoveCustomerTag` | 5d |
| 32 | GetCustomerTags | 併入 #24 | `GetCustomer.customer.tags` | 5d |
| 33 | SearchCustomersByTags | 不搬 | 無 client 呼叫端（`step-3-design.md:247`）；`ListCustomers` 有單一標籤篩選 | — |
| 34 | GetBuyerVipStatuses | 單體內 | `crm.VIPStatuses`，6e 換接線（A6-D18 W1） | — |

合計：對外 25 支舊方法 → 24 個 Connect 方法（#14 併 #15、#32 併 #24、#6 拆二）；被取代 1（#20 → `GetPresence`）；單體內 4（#8、#9、#18、#34）；刪除 3（#19、#21、#22）；不搬 1（#33）。25＋1＋4＋3＋1＝34。新增、非舊 RPC 對應的方法：`ReviewAppService.ListOwnStoreReviews` 已列；檢舉與封鎖見 §3、§4。計畫的規模表與 A6-5 必做清單改指這張表。

檢查指令：本表 34 列＝`rg -c '^\| [0-9]+ \|' step-5-design.md` 在 §1 範圍內；舊方法名逐一在 `interface-inventory.md` §1 有對應【量，人工對照 2026-11-21】。

## 2. 服務與身分

- `ReviewPublicService`（未登入可讀；觀看者可選，宣告 `publicViewerDependent`）、`ReviewAppService`（登入）。評價是買家與店家共用的資源（買家寫、店家回），照 10-17 修訂用 `<模組>AppService`，身分由資源推導：`CreateReview`、`ListMyReviews` 是買家；`ReplyReview` 由評價所屬店推導店家身分並向 seller 檢查 `CapMarketing`；`ListOwnStoreReviews` 不帶 `store_id`，店家由 `seller.CurrentStore` 推導（W1 (g)，與 social 的 `ListMyPosts` 同形）。
- `ChatAppService`：聊天也是共用資源。單一對話或訊息上的方法（`ListMessages`、`SendMessage` 對既有對話、`MarkConversationRead`）由對話歸屬推導 Party：呼叫者是對話的買家方，或擁有對話的店；自己跟自己的店不能對話（`SELF_MESSAGE_NOT_ALLOWED`），所以同一帳號在同一對話只會有一種身分【推，依 im `Send` 守門】。沒有單一資源可推導的方法（列表、未讀、全部已讀、串流、presence、開新對話）見 W1。
- `CrmAppService`：資源只屬於店家，方法都帶 `store_id`（資源的上層），向 seller 宣告 `CapMarketing`。不叫 `SellerAppService` 的方法：CRM 是 crm 模組擁有的資源，服務屬於擁有資源的模組（規範 L23–31）。
- 所有新方法進 `httpapi/effects.go` 的 customerEffects（讀＝Read、寫＝Write）、店家身分方法進 seller 能力宣告表；漏列啟動失敗【量，facts §2】。讀取標 `NO_SIDE_EFFECTS`；`MarkConversationRead`、`MarkAllRead` 是寫。

## 3. 檢舉（`ReportTarget`）

- `ReportTarget{kind, id}` 與 `REVIEW`、`CHAT_MESSAGE`、`REVIEW_REPLY` 已在 `api/hawkr/moderation/v1alpha1/moderation.proto:32-62`【量】；買家路徑已可送這三種（A6-4b/4c）。5a 不改形狀。
- 擁有者檢查：參照只帶 ID，存在、可見與「檢舉人是對話一方」由擁有模組核對（`review.ReviewForReport`、`im.MessageForReport`），handler 不信任 client 給的其他欄位；不是自己看得到的對象回 `not_found`（規範 L37）。這是計畫 A6-5 驗收的 mutation 目標。
- 店家檢舉人入口：W2。
- 文件註解「logged-in BUYER」（moderation.proto:3、16）依 W2 結論改寫。

## 4. 封鎖入口

`BuyerAppService.BlockAuthor`、`SellerAppService.BlockBuyer` 的 `oneof source` 只有 `comment_id`、`forum_entry_id`，解析在 `httpapi/blocks/blocks.go`，只認 social【量】。從評價與聊天封鎖要加參照成員：

- 買家：`review_id`（封鎖評價的回覆店家 → `BlockStore` 語意由參照推導）與 `message_id`（封鎖聊天對方店家）。
- 店家：`review_id`（封鎖寫評價的買家）、`message_id`（封鎖聊天對方買家）。
- `blocks.go` 檔頭已寫「第二個封鎖入口出現時成為編排模組」；5b／5c 照做：解析器改為依參照種類問擁有模組（social、review、im）取得作者身分，封鎖本身仍寫在發起者模組（不變量表第 1 列）。對話參照用 `message_id` 而不是 `conversation_id`：與檢舉同一種參照，作者明確【推】。
- 這是 bug 級缺口還是設計選擇：計畫寫「封鎖入口沿用既有方法」，沿用方法、擴參照不改語意，不列 W 題。

## 5. 聊天契約

### 5.1 訊息種類與卡片

- `MessageKind` enum：`TEXT`、`IMAGE`（模組只存這兩種，`M/im/internal/model/model.go:47-52`【量】）；`_UNSPECIFIED=0`，client 把未知值當「不支援的訊息」顯示（規範 L104）。卡片見 W3。
- 預覽：伺服器回結構化 `Preview{kind, text?, hidden, erased, sender}`，顯示句子（「傳送了一張圖片」「此訊息已被移除」）進共用文案目錄 `uitext/catalog`，key 依 `MessageKind` 與 hidden／erased 狀態；三端手寫預覽偵測在 A6-6 刪除（計畫缺陷表最後一列）。
- 圖片：回 `Image{uri_template}`（規範 L107），不回 `ObjectKey`。

### 5.2 送出冪等

`SendInput` 沒有 `request_id`【量，`M/im/internal/model/model.go:196`】；Connect client 在 unavailable 時不會自動重試沒有冪等保證的寫入（規範 L175），App 手動重送會重複。見 W4。

### 5.3 `StreamEvents` 與 helper

- 請求：`store_id`（依 W1）、`cursors`（`map<string conversation_id, int64 seq>`，上限 50，模組 `MaxCursors`）。
- 回應 frame：`oneof event { Ready ready; MessageEvent message; UnreadChanged unread; PeerRead peer_read; Resync resync; Heartbeat heartbeat; }`；`Ready` 帶未讀總數，helper 補 `heartbeat_interval` 與 `max_lifetime`（`step-2b-design.md` §5.2）。
- helper：放在聊天 handler 的 package 內，不先抽到 `httpapi/rpc`；目前只有這一個串流方法【量，`rg -n 'returns \(stream' api` 無結果】。出現第二個 server streaming 方法時再抽到 `httpapi/rpc`。內容：
  - 心跳：`Next` 閒置 15 秒送 `Heartbeat`；client 30 秒沒收到 frame 當半開重連。
  - 壽命：15 分鐘 ±10% 抖動後正常結束（end-of-stream 無錯誤），client 用最後的 seq 重連。
  - 寫入期限：每次寫 frame 前以 `http.ResponseController.SetWriteDeadline` 設 10 秒，逾時結束；server 不設全域 `WriteTimeout`（會切斷所有串流）。
  - 重驗：每 60 秒重新 `Authenticate` session、重讀店家狀態，成功把新 Party 交給 `Subscription.Revalidate`，失敗結束。
  - 結束原因 → Connect code：壽命到＝正常結束；session 失效或帳號刪除＝`unauthenticated`；模組 `EndLostEligibility`（關店）＝`failed_precondition` + `STORE_CLOSED`；`EndShutdown` 與連續 3 次重驗暫時失敗＝`unavailable`（client 依 backoff 重連）；超過 `MaxStreamsPerParty`／`MaxCursors`＝`resource_exhausted`／`invalid_argument`。
  - 常數都是估計值（`step-2b-design.md:517`）；5c 驗收含經 web-host 反代與 Cloudflare 的實測：不緩衝、閒置超過 60 秒不被切【推，未量】。
- `im.Module.Run` 接 `cmd/hawkr` 的 serveAll ctx（5c）。
- 串流在 customerEffects 記為 Read；停權帳號仍可讀，所以停權不結束串流（A6-D16 W4）【量，`httpapi/session.go` 停權只擋 Write】。

### 5.4 Presence

`GetPresence(store_id?, conversation_ids ≤ 50)` → 每個對話 `online`；`NO_SIDE_EFFECTS`；不是自己的對話回 `not_found`。im CARRYOVER 記的每次約 5 次 DB 往返在 5c 合成一次查詢（`im/CARRYOVER.md:88`）。

## 6. CRM

- `ListCustomers(store_id, type?, tag_id?, sort, page)`：單一標籤篩選、排序 total_spent／order_count／first_order_at；不做關鍵字（A6-D18 W5）與多標籤搜尋（#33 不搬）。
- `Customer` 回應欄位白名單：`buyer_id`、`display{nickname, avatar, deleted}`、`type`、`total_spent`、`order_count`、`first_order_time`、`tags`。CD8 驗收：5d 的測試走 descriptor 斷言 `Customer` 與所有 CRM 回應 message 不含 phone、email、address 類欄位名；mutation 把 `phone` 加回 proto，測試要失敗。
- 舊 gateway 的四條非 RPC REST：
  - `meta`：由 `GetCustomer` 取代（客戶列存在＝有交易關係，暱稱頭像來自 `buyer.BatchDisplay`）【推】；只有 SA 呼叫，A6-6d 改。
  - `coupons`：刪（A6-D18 W3 (a)）；計畫 A6-5 段的 `coupons` 字樣是舊文字，改掉。
  - `lookup`（以電話找買家）、`orders`（店家看某買家的訂單）：W5、W6。

## 7. ErrorReason

模組等 A6-5 定案的原因【量，facts §4】：review `REVIEW_ALREADY_EXISTS`、`ORDER_NOT_REVIEWABLE`、`REVIEW_WINDOW_EXPIRED`、`REVIEW_NOT_REPLYABLE`；im `SELF_MESSAGE_NOT_ALLOWED`、`STORE_CLOSED`、`CHAT_BLOCKED`、`RECEIVER_DELETED`；crm `CRM_TAG_NAME_TAKEN`、`CRM_TAG_LIMIT`；共用 `MODERATION_REJECTED`。原則：只有 client 會依它分支的才進 enum；「找不到」類（`ORDER_NOT_FOUND`、`CRM_CUSTOMER_NOT_FOUND`、`CRM_TAG_NOT_FOUND`）一律 `not_found` 不帶 reason（規範 L37 反列舉）；`SELLER_SUSPENDED` 沿用既有值 2。`MODERATION_REJECTED`：review 送出的審核拒絕是欄位錯誤（invalid_argument 帶欄位違規），client 不依 reason 分支，5b 不加，只在 client 需要分支時再加。5b–5d 各自加自己的值，同一個 PR 更新 `uitext` 目錄的錯誤句。

## 8. 子步驟分工與驗收

| 子步驟 | 內容 | 驗收（每步） |
|---|---|---|
| 5b | review proto 兩個 service、handler、封鎖參照 `review_id`、blocks 編排化（第一個非 social 參照）、ErrorReason | `buf lint`／breaking（v1alpha1 允許）、生成 drift、handler 測試涵蓋 §1 的 5b 列、mutation：拿掉 `ReviewForReport` 擁有者檢查與封鎖參照的作者解析，測試要失敗（CI） |
| 5c | chat proto、helper、`Run` 接線、presence 合併查詢、封鎖與檢舉的 `message_id`、店家檢舉入口（W2）、MessageKind 與文案、送出冪等（W4） | 同上；helper 單元測試（心跳、壽命、寫入期限、重驗失敗各結束原因）；不緩衝測試經單體；實網反代測試列「需要人做的事」 |
| 5d | crm proto、handler、`cmd/hawkr` 建構 crm（不接 consumer）、CD8 descriptor 測試、W5／W6 結論 | 同上；CD8 mutation |

5c 依賴 5b 的 blocks 編排化（同一個解析器），5b 先做；5d 與 5c 無依賴。

## W. 待 Will 決定

**W1 聊天的店家身分怎麼指定（擋 5c）**
問題：列表、未讀、全部已讀、串流、presence、開新對話沒有單一資源可推導身分；同時是買家又有店的帳號要指出「看哪個收件匣」。
- (a) 請求帶選填 `store_id`：沒帶＝買家收件匣，帶了＝該店收件匣（向 seller 驗擁有與 `CapChatRead`）。`store_id` 是資源的上層（AIP parent），不是視角 header；CRM、店家評價列表用同一個形狀。業界：Slack／Stripe 以資源 ID 指定工作區或帳戶。
- (b) 拆兩組方法（`ListBuyerConversations`／`ListStoreConversations`…），方法數加倍。
- (c) 一律回兩種身分合併的列表，client 自己分：串流與未讀要合兩個 Party，模組 `Subscribe` 一次一個 Party，要改模組。
- (d) parent 一律必填：買家方用明確的 `buyer: me`（或 `buyers/me`），店家方用 `store_id`；沒有「沒帶就是買家」的隱含預設（AIP-122 的資源名稱）。
建議 (a) 或 (d)，偏 (d)：沒有隱含預設，最貼近 ADR「client 不選身分」的字面。無論選哪個，都要在 ADR `2026-10-07-connect-without-perspective` 加修訂，寫明「列表類方法指定資源上層不算視角」，否則和 10-17 修訂的「身分由伺服器推導，client 不選」字面衝突。

**W2 店家檢舉入口放哪（擋 5c）**
- (a) 沿用 `ModerationAppService.SubmitReport`，加選填 `store_id`：帶了＝以店家身分檢舉（只接受 `CHAT_MESSAGE`，其他種類 `invalid_argument`），`ListMyReports` 同樣加 `store_id` 列店家送出的檢舉。與 W1 同一個形狀。
- (b) `SellerAppService` 另開 `ReportChatMessage`（已有 `SellerReportAppService` 買家檢舉店家的先例，檢舉本來就不只在 moderation 一處）。
- (c) `SubmitReport` 不加欄位，由 `im.MessageForReport` 推導檢舉人是對話的買家方還是店家方：同一帳號不能和自己的店對話，所以一則訊息的檢舉人只會是一種身分【推，依 `SELF_MESSAGE_NOT_ALLOWED`】；店家方只能檢舉 `CHAT_MESSAGE` 本來就由擁有模組守。`ListMyReports` 沒有單一資源，照 W1 的結論帶上層。
建議 (c)：與 §2「單一資源方法由歸屬推導」一致，不需要「帶了 store_id 只收某種類」的特例。

**W3 卡片訊息（擋 5c）**
A6-D14 W6 (b) 定了「伺服器參照」的方向，但商品與訂單的擁有者還在 Java（B8），單體沒有讀商品卡或訂單摘要的方法【量，facts §4】。
- (a) 這次只出 `TEXT`、`IMAGE`，卡片延到 B8（商品、訂單搬進單體）後加 `MessageKind` 值與 `oneof card`；v1alpha1 加值不破壞。代價：A6-6 換到新契約後，client 暫時不能從聊天送商品或訂單卡片（舊程式有此功能），算使用者看得到的功能縮減。
- (b) 現在做：補 legacy orderservice／catalog 的讀取 client 組卡片資料，B8 時再換掉；等於先寫一份之後要刪的整合。
建議 (a)，延後的觸發條件寫「ordering 或 catalog 進單體」。

**W4 送出冪等（擋 5c，改資料模型）**
- (a) `SendMessageRequest.request_id`（UUIDv4，必填），im 加 `UNIQUE (conversation_id, sender_kind, request_id)`，重送回傳既有訊息；與 `SubmitReport` 同做法。
- (b) 不做，client 不自動重送。
建議 (a)：行動網路重送是常態，業界聊天（Slack `client_msg_id`、WhatsApp）都有 client 端訊息 ID。

**W5 CRM `lookup`（以電話找買家，擋 5d；影響 UI）**
今天單體一律回找不到（帳號沒有電話），兩支 client 的優惠券選買家元件還在呼叫【量】；CD8 禁止回電話。
- (a) 刪 `lookup`，優惠券選買家改從 `ListCustomers` 選（有過交易的客戶）。是 UI 流程改動，A6-6 前要你同意畫面。
- (b) 改成 buyer 擁有的「以電話精確比對、只回 buyer_id 與暱稱」查詢：電話仍是查詢鍵，等於讓店家確認某電話是不是平台買家，屬可列舉的個資探測。
建議 (a)。

**W6 CRM `orders`（店家看某買家的訂單，擋 5d）**
擁有者 ordering 還在 Java；單體沒有對應方法，只有 SA 客戶詳情分頁在用，website 用自己的訂單列表【量】。
- (a) 不放進 CRM；SA 客戶詳情的訂單分頁改用既有的店家訂單列表加 `buyer_id` 篩選（若 Java 店家訂單列表已支援；5d 先查，不支援就回報）。
- (b) 補 legacy orderservice 呼叫 Java `GetBuyerOrdersForSeller`，以 `CrmAppService.ListCustomerOrders` 暴露，B8 換成 ordering 模組。
建議 (a)：訂單屬 ordering，不該由 crm 服務代為暴露；(a) 若 Java 列表不支援篩選，退回 (b)。

## 已決（Will 2026-11-21）

- W1 (g)：沒有單一資源可推導身分的聊天方法依集合分方法，買家版與店家版各一支（例如 `ListConversations`／`ListStoreConversations`；未讀、全部已讀、串流、presence、開新對話同理），店家由「呼叫者此刻的店」推導（帳號至多一家現行店，`seller/db/schema.sql:102`），請求不帶 `store_id`；與 social 的 `ListMyPosts`、`ListStoreComments` 同形，ADR 不修訂。§5.3、§5.4 的 `store_id` 依此改為分方法。觸發重開：一個帳號可管多家店或店家有員工。
- W2 (c)：`SubmitReport` 不加欄位，由 `im.MessageForReport` 推導檢舉人是買家方或店家方；店家送出的檢舉列表照 W1 另開 `ListMyStoreReports`。
- W3 (a)：這次只出 `TEXT`、`IMAGE`；卡片延到 ordering 或 catalog 進單體（B8）後加。
- W4 (a)：`SendMessageRequest.request_id` 必填 UUIDv4，im 加 `UNIQUE (conversation_id, sender_kind, request_id)`，重送回傳既有訊息。
- W5 (a)：刪 `lookup`，優惠券選買家改從 `ListCustomers` 挑（UI 方向已同意，畫面細節在 A6-6 給 Will 看）。
- W6 (a)：CRM 不暴露訂單；seller-app 客戶詳情改用店家訂單列表加 `buyer_id` 篩選，5d 先查 Java 列表是否支援，不支援停下回報。
