# im 模組的沿用盤點

<!-- 決策紀錄：不放進 AGENTS.md，因為 AGENTS.md 每次在這個目錄工作都會載入。模組現行的規則寫在 AGENTS.md。 -->

來源：`backend/docs/plans/2026-11-19-a6-review-im-crm/domain-inventory.md` §2 的 M 列（只列與 im 有關的）與 §6.2 的 ID 列；實作設計 `step-2a-design.md` v1，Will 的決定 A6-D14（W1–W6）。本檔對應 A6-2a1（資料與送出、已讀、刪帳與退場的抹除、`legacy/imrelay`）；即時通道與 presence 在 A6-2b（見〈即時通道的沿用盤點〉），Connect 與 handler 在 A6-5，隱藏的決定在 A6-4。

## 沿用盤點

im-service 的每個機制，搬進來時都重新判斷過（遷移計畫「模組完成的定義」）：ID 方案、extension、套件、共用 helper、舊的檢查邏輯、schema 慣例、CodeMeta 的值域與錯誤碼。從零設計會不同的，完成前提出來決定。對外 API（舊 gateway 的 handler）的機制在下一節。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| M1 對話、訊息 ID 是 TSID bigint，`messages.id` 同時是歷史游標 | Java 或別的服務拿對話或訊息 ID 當鍵 | 不成立：只有 `chat.message.sent` 的 `message_id`、`conversation_id` 進推播 data，沒有別的服務查實體 | 實體主鍵用應用層產生的 UUIDv7（row-identity ADR）；游標用有序的序號 | 改：`conversations.id`、`messages.id`、`hides.id` 是 UUIDv7；`buyer_id`、`store_id` 維持 bigint（外部業務鍵）；訊息游標是對話內的 `seq` |
| M2 房名 `b<buyerID>_s<storeID>` 字串，同時是路由、授權、鎖鍵與對外 ID | 一對買賣雙方要有唯一的對話 | 成立，但不需要字串 | 對話是 `(buyer_id, store_id)` 的唯一列，授權以對話 ID＋session 的一方判斷 | 改：刪 `room_id`，`UNIQUE (buyer_id, store_id)`；送出與讀取以對話 ID 授權，不是該對話的一方回 NotFound（ID6）；同一個數字同時是 buyer_id 與 store_id 時不會串到另一個身分空間 |
| M3 訊息順序：`pg_advisory_xact_lock(hashtext(room_id))`＋`MAX(seq_id)+1`、`UNIQUE(room_id, seq_id)` | 每個對話的訊息要有連續、唯一的序號，斷線補洞靠它 | 成立；但 32 位元雜湊鎖碰撞時會讓不相干的房間互等 | 單一 DB 在對話列上遞增計數取序 | 改：`AllocateSeq` 一句 `INSERT … ON CONFLICT DO UPDATE` 建立對話、配發 `last_seq`、同時推進寄件人的已讀游標；鎖的是對話列本身；rollback 時 `last_seq` 跟著回滾，seq 沒有空洞（2b 的補洞依賴它） |
| M4 Redis：封鎖讀取（`block:out:`／`block:in:`）、`im:account:deleted:<id>` 標記（fail-open、無 TTL）、同步速率限制、WS ticket | 送出要即時的封鎖與刪帳資料 | 不成立：封鎖在 buyer、seller 的 DB，刪帳事實由事件複製進 im 自己的表，單體同一個資料庫 | 同庫用擁有者的 API；查詢失敗就整個操作失敗（fail-closed） | 改：封鎖用 `buyer.IsBlockingStore`、`seller.StoreBlocksBuyer`；刪帳用 `im.inactive_parties`（G4）；刪除 Redis 依賴；速率限制與 WS ticket 屬 A6-2b、A6-5 |
| M5 `buyer_id`、`store_id` 的 `DEFAULT 0` 哨兵 | 舊資料沒有身分的訊息 | 不成立：系統從未上線 | NOT NULL ＋ CHECK > 0 | 改：`conversations.buyer_id`、`store_id` 是 `NOT NULL CHECK > 0` |
| M6 媒體：送出時 `FinalizeAndLink`、脫敏時 `ReleaseChatReferences` | media 呼叫與業務列寫入要一致，媒體呼叫不得在 DB transaction 內 | 不成立：失敗時留下孤兒 link | 與 review、social 同：transaction 外 `Finalize`，transaction 內寫列＋同步版本＋排同步 job，同步 job 依業務列現況 Link／Unlink | 改：`mediaref` 與 `media_ref_sync`、`retired_assets`；抹除時每批記退場資產並排同步（ID8）；同步 worker 與保底掃描已接進 `cmd/hawkr` 的 worker（`newIM`）。`internal/mediaref` 是各擁有媒體的模組的第 7 份同形外殼（演算法已共用在 `media.RefSyncer`），不抽成共用：整套同步（外殼、`media_ref_sync`、`retired_assets`）在 B1 換成 media 不認識業務的終點時一起刪除（ADR `2026-10-03-media-references-owner-side`；Will 2026-11-20）。重新評估：B1 之前又有模組要加一份時，改評估提前 B1 |
| M8 MQ 消費者：`buyer.deleted`、`seller.deleted`、`account.deleted`、`seller.suspended` | 刪帳與退場要驅動 im 抹除 | 成立 | 同庫內下層通知上層用同 transaction 的模組事件（單體規則 4） | 改：只訂 `buyer.BuyerDeleted` 與 `seller.StoreRetired`（只寫 `inactive_parties`、排 `im.erase_party` job）；不訂 identity 的 `AccountDeleted`（只有帳號鍵，要列舉歷史身分才能用，那是 ID3 的根源），不訂 `BuyerDeletionRequested`（等待刪除的買家仍能登入，為開啟中的訂單聯絡店家，A6-D14 W4；前提 2a1 開工時以 `identity/internal/model/model.go:123` 與 `sessions.sql` 確認）；`seller.StoreSuspended` 的強制斷線屬 2b |
| M9 對話列表讀時補名稱頭像，不存個資 | 列表要顯示對方 | 成立（顯示需要），但不能存快照 | 讀時批次補，已刪帳回旗標不回名稱 | 沿用：模組不存快照；`buyer.BatchDisplay` 由 httpapi 在 A6-5 補 |
| M11 事件：fat event（`chat.message.sent`）、`go` 協程發送、無 outbox | Java notification 依賴舊 wire | 成立（A6-6e 前仍由舊 im-service 發） | 模組事件只帶領域鍵，同 transaction 發布；relay 在 transaction 外組 wire payload | 改：`MessageSent` 只帶領域鍵、同 transaction 發布（ID7）；wire 形狀由 `legacy/imrelay` 維持，`conversation_id`、`message_id` 的值從房名、TSID 變成 UUID 字串（型別不變，設計書 §9 第 5 項），A6-6e 才接進 `cmd/hawkr` |
| M12 封鎖：`IsBlocked(sender, receiver)` 在送出與列表時，沒有訂單例外；列表過濾有封鎖的對話 | 封鎖要在送出時生效 | 成立，但 A6-D1 要求有進行中的訂單時仍可以聊 | 封鎖時保留進行中交易的溝通 | 改：G3 雙向擋，有進行中的訂單或售後時放行（`Orders` port）；列表不再因封鎖隱藏對話（設計書 §9 第 4 項），由 A6-5 標示無法送出的原因 |
| M13 詞典過濾 `pkg/moderation.Check`，只查文字 | 髒字詞典只有一份 | 不成立：單體有 `platform/textfilter` | 唯一詞典 | 改：G5 跑 `platform/textfilter`；卡片訊息 2a 不支援（W6），所以沒有繞過詞典的路 |
| M14 principal 解析：每次送出兩次 RPC 嚴格解析雙方，查不到 Unavailable | 事件收件人與授權要知道身分 | 不成立：單體內 `buyer_id`、`store_id` 就是業務鍵 | 身分在入口解析，模組只收已解析的身分 | 改：`Party` 由 httpapi 從 session 解析；對方存在性在首則訊息查 `buyer.ProfileByBuyerID`／`seller.Store`；買家帳號用 `buyer.AccountIDByBuyerID`（只要帳號鍵，不讀個資）；`chat.message.sent` 的收件人身分空間（買家 buyer_id、賣家 account_id）由 relay 照舊組 |
| M15、M16 in-memory hub、presence、ChatStream | 即時推送 | 成立，但屬 A6-2b | 見 Q4 | 不在 2a：`Send` 只發 `MessageSent`，2b 的 hub 訂閱它 |
| M18 分頁：對話列表 20/100，歷史用 `before_id` 游標加 OFFSET 0 | 無限捲動 | 成立（keyset） | AIP-158 keyset，不回總數 | 改：對話 `(last_message_at DESC, id DESC)` keyset，訊息以 `seq` keyset；頁大小預設 20、上限 100，多取一筆判斷下一頁（ID9） |
| ID1 對話預覽存成 `last_message_content`、`messages.preview_text`，隱藏後列表仍回內容 | 列表要顯示最後一則 | 成立，但存兩份會與隱藏、抹除不一致 | 讀時由最後一則推導 | 改：不存預覽；`ListConversations` 以 `(id, last_seq)` join 最後一則並排除隱藏、抹除（ID1 從結構上消失）；回結構化的 `Preview`，句子由 client 呈現層依 enum 產生 |
| ID3 刪帳抹除依 `AccountID` 列舉所有歷史身分 | 刪帳要抹掉該身分寄出的訊息 | 成立，但不該連另一個身分一起抹 | 事件帶身分鍵，只處理該身分 | 改：`BuyerDeleted` 只抹該 buyer_id 寄出的、`StoreRetired` 只抹該 store_id 寄出的（以 `(sender_kind, 對話的 buyer_id 或 store_id)` 表示寄件人，`erase_party` job 依 id 升序分批 200、可重跑；`im.erase_sweep` 保底掃描補排被丟棄的 job） |
| ID5 停業查詢失敗放行（fail-open）、對話 upsert 與未讀的錯誤只記 log | 守門要有明確結果 | 不成立 | fail-closed；寫入同一個 transaction | 改：G1–G4 任何查詢錯誤回 Unavailable；seq、訊息、事件在同一個 transaction，任一步失敗整筆回滾 |
| ID7 `chat.message.sent` 在 commit 後以 `go` 協程發送 | 事件與資料一起成立 | 不成立 | 同 transaction 發布，relay 在 transaction 外送 | 改：見 M11 |
| 未讀：兩欄計數與 `is_read` 三套寫法，送出不歸零寄件人自己的未讀 | 要有總未讀與每個對話的未讀 | 成立 | `last_seq - 自己的已讀游標`；回覆時視為已讀 | 改：兩個游標在對話列上（聊天只有兩方，不建 participants 表）；`AllocateSeq` 同句推進寄件人的游標（設計書 §9 第 2 項）；讀取方法不寫游標，已讀只走 `MarkRead`、`MarkAllRead` |
| 停權賣家的訂單綁定（`order_no`、`order_binding_verified_at`，client 帶 `order_no`、`NOT IN (0,80)`、綁定永久有效） | 停權時仍可為進行中的訂單聯絡 | 成立，但判準太寬（已完成與已退款也算） | 停權只保留履約必要的溝通 | 改（W1 (c)、W2 (i)）：G1 與 G3 共用一個「進行中」判定，每次送出時查，不存綁定欄位；以 `legacy/orderservice` 的 `HasOrderBetween` 取最近一筆再以 `OrderParties` 看狀態 ∈ {10,20,30,32,35,40}。已知看不到「最近一筆已結案、較舊的一筆還在配送」，錯的方向是多擋；退場點 B8（ordering 搬遷時 port 換成精確的 in-process 查詢） |
| 卡片訊息：client 帶任意 JSON，預覽取 `extra["preview"]`，詞典只查文字 | 聊天裡引用商品或訂單 | 成立，但沒有形狀的欄位繞過詞典 | 卡片是伺服器擁有的參照，顯示內容由平台依參照組出 | 不搬（W6 (b)）：`message_type` 只有 `TEXT`、`IMAGE`；卡片連同形狀在 A6-5 定 proto 時以往前的 migration 加 |
| `chat.message.sent` 的 `content_preview` 中文句子（「傳送了一張圖片」等） | 推播要有預覽文字 | 成立 | 共用文案目錄 | 過渡物：共用文案目錄目前沒有聊天條目（量：`rg -n "chat\|Chat\|訊息\|圖片" backend/internal/platform/uitext/catalog/zh-TW.json` 只命中社群檢舉的「垃圾訊息」），句子暫放 `legacy/imrelay`；A6-5、A6-6 為 client 加聊天預覽的目錄條目時改取同一批；退場點 A8（relay 刪除） |
| `LockParty`：送出與 `Deactivate` 之間的互斥（舊碼沒有；舊 `account:deleted` 標記無鎖） | 送出的寫入與「身分進入刪除」要有先後，否則抹除 job 看不到之後才 commit 的訊息 | 成立 | 同一個身分的狀態變更與依賴它的寫入以鎖串行 | 新增：`pg_advisory_xact_lock`（`im.party:<kind>:<id>`），不用列鎖，因為 `Send` 在送出前沒有鎖買家或店家的列（鎖列要多鎖別的模組的表，也會與 accountdeletion 的 identity → buyer → seller 鎖序互相牽動）；不進 `lockorder`，因為它只在 im 自己的 transaction 內取、沒有跨模組的持鎖次序。`Send` 依固定順序（買家先於店家）鎖雙方再重查 `inactive_parties`，`Deactivate` 一次只鎖一個，所以不會死結；前提：`Send` 不先持買家或店家的列鎖，改了要重看 |
| `im.erase_sweep` 每 15 分鐘的保底掃描 | job 用完 `MaxAttempts` 被丟棄後，身分仍在 `inactive_parties` 又還有未抹除的訊息 | 成立 | 週期性對帳補排 | 新增：唯一性以 `SweepInterval` 為時間窗（預設的 ByState 含 completed，會讓掃描被擋約一天）；抹除本身冪等可重跑 |
| `im.hides` 訊息隱藏紀錄 | 隱藏要可多次、可還原，並且能追溯 | 成立 | 形狀同 `review.hides`、`social.hides`：還原留列、同一則訊息未還原的隱藏至多一筆 | 新增：本模組只有資料與讀取（`Hidden`），隱藏與還原的決定由 moderation 呼叫 `Hide`／`Restore` |

## 即時通道的沿用盤點

來源：`backend/docs/plans/2026-11-19-a6-review-im-crm/step-2b-design.md` §9（H1–H26）、決定 A6-D15、A6-D16。舊機制的出處是該設計書重新讀 im-service 原始碼的行號。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| H1 `MessageHub` 三張 map、三個 channel、單 goroutine `Run` | 進程內要找到某個身分的連線 | 成立，但 register channel 與單一事件迴圈不需要 | 進程內「Party → 訂閱者」的表，跨進程交給資料庫 | 重寫：`live.Hub`（map＋RWMutex，鍵含 kind） |
| H2 推送在 commit 後、transaction 外，直接寫連線 channel | 新訊息要即時到 | 成立；但單進程記憶體推送在多進程下收不到 | 同 transaction 發通知，資料庫保證 commit 後才送達，內容從資料庫讀 | 取代：`write` 內 `pg_notify`＋`MessagesAfter` |
| H3 每連線 256 格緩衝，滿了丟 | 慢消費者不能拖住別人 | 成立 | 不緩衝內容；合併待處理訊號 | 取代：`pending` 以對話合併，超過上限 `Resync{all}` |
| H4 `Broadcast`、`BroadcastExcept`、`ConnectionCount` | 有呼叫端 | 不成立 | 沒有呼叫端就不存在 | 刪除 |
| H5 `SendToUser(Role)` 按 userID＋loginRole 找連線 | 買家與店家身分要分開 | 成立 | 按 (kind, id) 找 | 取代：`partyKey`（ID6） |
| H6 WS 與推播雙軌送達，靠 `seq_id` 去重 | 串流不是送達保證 | 成立 | 推播另走，去重靠 seq 與訊息 ID | 保留原則：推播仍由 `legacy/imrelay` |
| H7 presence 在 hub 記憶體、`lastSeen` | 多進程要看到全體 | 不成立：多進程只看到連在自己身上的人 | 共享狀態＋過期 | 取代：`im.presence_streams`，不保存最後上線時間（A6-D5） |
| H8 `presence_sub` 幀，任何人可訂任何人 | 在線狀態只給有對話的對方 | 成立（舊碼無授權＝ID2） | 以共有對話授權的讀取 | 取代：`Presence`，不是共有對話整個 NotFound |
| H9 presence 帶 `lastSeenAt` | A6-D5 不給最後上線時間 | 不成立 | 不給 | 刪除（型別沒有時間欄位） |
| H10 `typing` 與限流 | A6-D15 不做打字中 | 不成立 | — | 刪除 |
| H11 `read_receipt` 轉送幀、不落庫 | 對方要看到已讀 | 成立 | 已讀游標落庫、由游標推導 | 取代：`PeerRead`（`r` 通知）；批次已讀不推給對方 |
| H12 `ping`／`pong`、client 30 秒無 frame 判半開 | 長連線要偵測半開 | 成立 | 伺服器閒置時送 heartbeat | 不在模組：A6-5 的串流傳輸 helper |
| H13 `sync` 幀：20 房間、每 actor 5 次限流、無權限回空 | 斷線後補洞 | 成立 | 開串流時帶游標 | 取代：`Subscribe` 的 `Cursors`（≤50）；無權限靜默忽略保留 |
| H14 每房間 `sync_done` | 知道補洞結束 | 成立 | 一個結束標記 | 取代：`Ready` |
| H15 `unread_changed` 推送 | client 要即時總未讀 | 成立 | 狀態改變後讀一次總數 | 保留語意：`Unread` 事件，每輪最多算一次 |
| H16 `seller.suspended` 強制斷線 | 停權者不能續傳 | 不成立：串流只讀，送出是 unary 由 G1 逐則擋 | 只讀串流不需要斷線 | 刪除；只有關店由 `Revalidate` 結束 |
| H17 連線身分來自第一幀、im-service 不驗 | 身分要可信 | 成立（舊碼不驗） | 身分只從 session 來 | 取代：`Party`（ID6） |
| H19 每帳號 10 條並行連線，進程內 map | 擋單一帳號耗盡資源 | 成立；多進程下進程內計數失效 | 以共享狀態計數 | 取代：數 `presence_streams`，advisory lock 序列化，fail-closed |
| H21 Redis（封鎖、刪帳標記、限流） | — | 不成立 | — | 刪除：2b 沒有 Redis 依賴 |
| H22 推送幀帶發送者名稱與頭像 | 畫面要顯示 | 成立 | 事件只帶業務鍵，顯示資料由 httpapi 補 | 取代：`Event` 不帶名稱頭像 |
| H25（新）presence 過期用哪個時鐘 | 多進程共享的過期判定 | 成立 | 單一時間來源 | 資料庫 `now()`；測試以 SQL 改 `expires_at` |
| H26（新）LISTEN 連線手寫重連與退避 | 斷線自動恢復，並在重連成功時發 epoch | 成立 | `jackc/pgxlisten` | 手寫（約 60 行）：epoch 需要 `lastOK` 與重連掛點；新依賴要 Will 同意，要採用只換 `live/listener.go` |
| H27（新）presence 延長補回被清掉的列 | 資料庫重啟後 UNLOGGED 表是空的、被清理掉的串流仍在跑 | 成立 | 只 UPDATE 的延長會讓這些串流不被計數、不顯示在線 | 取代：延長以 upsert 補回；`Close` 與延長的競爭最多讓列多活一個 `PresenceTTL` |

## 即時通道的待決與重新評估觸發

常數（`internal/live` 的 `Config` 預設與 `PresenceSweepInterval`）都是估計值，不是量測值；下表是各自重新評估的觸發條件。門檻是起始估計，量到資料後以量測值取代。

| 項目 | 現值 | 重新評估的觸發 |
|---|---|---|
| `MaxStreamsPerParty` | 10 | 一週內 `Subscribe` 回 ResourceExhausted 的次數超過 `Subscribe` 總數的 1%，或單一帳號同時開啟的裝置數的 p95 超過 10 |
| `MaxCursors` | 50 | client 回報有超過 5% 的串流開啟時有超過 50 個最近活動的對話 |
| `MaxBacklog` | 200 | 因補洞超過 `MaxBacklog` 而交出的 `Resync{conversation}` 超過重連次數的 1% |
| `MaxPending` | 500 | 因 `pending` 溢出而交出的 `Resync{all}` 每天超過訂閱數的 0.1% |
| `MaxDelivered` | 1000 | 每條訂閱的 `delivered` 表在量測下占進程記憶體的比例超過 5%，或被修剪造成的重讀占補洞讀取的 10% 以上 |
| `PresenceTTL`、`PresenceBeat` | 60 秒、20 秒 | 日誌「延長串流存活標記失敗」每進程每小時超過一次，或 presence 讀到離線但串流仍在的回報 |
| `AliveEvery` | 10 秒 | 量到死連線偵測時間超過 10 秒的案例，或監聽迴圈的存活確認占資料庫查詢量的 1% 以上 |
| `BackoffBase`、`BackoffMax` | 0.5 秒、30 秒 | 一小時內單一進程重連超過 20 次，或資料庫維護中重連請求造成連線數告警 |
| `PresenceSweepInterval` | 15 分鐘 | `im.presence_streams` 的列數超過同時在線串流數的 10 倍 |

| 待決 | 為什麼不在本步做 | 觸發 |
|---|---|---|
| 監聽重連補洞的時間界線：`ActiveSince` 以 `lastOK - margin` 比較，margin 是「im 寫入 transaction 最長開多久」 | repo 沒有任何 transaction 時間界線（連線池、`InTx`、`statement_timeout`、`idle_in_transaction_session_timeout`、`transaction_timeout` 都沒設定，httpapi 沒有請求 deadline），不發明數字；沒有界線時，開始早於 `lastOK`、提交晚於斷線的送出，其 `last_message_at` 可能小於 `lastOK` 而漏掉（不在 `delivered` 的對話少一個 `Resync`） | 平台層設定 transaction 或 statement 的時間上限時，以那個上限當 margin；或 A6-5 為寫入路徑加請求 deadline 時 |
| `Presence` 一個對話約 5 次循序的資料庫往返（授權、封鎖、對方身分、在線） | 目前沒有 handler 呼叫，批次化要看實際的呼叫形狀 | A6-5 開始：接 `Presence` handler 時改成一次查詢完成授權與在線 |

## 傳輸層的沿用盤點

舊 gateway 的 handler 與 gRPC 的機制，以機制為單位（認證與身分、錯誤、分頁、快取、請求大小、限流、trace、下游呼叫的逾時與重試）。新的對外 API 照〈API 設計規範（Connect）〉。本步（A6-2a1）沒有對外入口，下列各項的落地在 A6-5。

| 沿用的機制 | 必要條件 | 今天是否成立 | 業界在同樣條件下的做法 | 決定 |
|---|---|---|---|---|
| 認證與身分（gateway 驗 JWT、傳 user_id、target_id，模組猜兩種房名） | 送出與讀取要知道呼叫者是買家或店家 | 成立 | session 在入口解析，模組只收已解析的身分，資源以 ID 授權 | 沿用語意、改機制：`Party` 由 httpapi 解析（店家帶 `seller.Store`，讀寫都以 `seller.Authorize` 檢查）；入口在 A6-5 |
| 錯誤碼（9201、9202、9206、9207、9210、9240） | client 要能分辨可處理的情境 | 成立 | 錯誤分類＋ reason | 改：`apperr` 分類加模組端的 `Reason` 常數（`SELF_MESSAGE_NOT_ALLOWED`、`SELLER_SUSPENDED`、`STORE_CLOSED`、`CHAT_BLOCKED`、`RECEIVER_DELETED`、`MODERATION_REJECTED`）；proto 的 `ErrorReason` 在 A6-5 定案；句子在 `internal/text` |
| 分頁 page／size＋總數、OFFSET | 見 M18 | 不成立 | keyset | 改：見 M18 |
| 下游呼叫的逾時與重試 | 訂單查詢不能無限等 | 成立 | 明確 deadline、fail-closed | 改：`orderservice` 的 5 秒逾時；任何失敗回 Unavailable，不當成「沒有進行中的訂單」 |
| 請求大小與限流 | 防濫用 | 成立 | 入口統一 | 不在模組：A6-5；文字上限 4096 個字元在模組與 DB CHECK |

## 與 im-service 的差異

資料沒有搬移（系統從未上線），schema 是新的 baseline。

- 表：`conversations`（UUIDv7、`UNIQUE (buyer_id, store_id)`、`last_seq` 與兩個已讀游標、無 `room_id`、無預覽欄位、無未讀計數欄、無 `order_no` 綁定）、`messages`（`seq`、`sender_kind`、`message_type`、`erased_at`；無 `receiver_id`、`is_read`、`deleted_at`、`preview_text`、`hidden_at`）、`hides`、`inactive_parties`、`media_ref_sync`、`retired_assets`。
- 儲存的代碼（`sender_kind`、`message_type`、`inactive_parties.kind`）是 UPPER_SNAKE（`BUYER`、`STORE`、`TEXT`、`IMAGE`），依 ADR 2026-09-25-codemeta-retirement D3 與 `TestStoredCodesInChecksAreUpperSnake`。
- 送出把寄件人自己那一方標成已讀；讀取方法不改任何狀態。
- 停業只有 `CLOSED` 算；停權的訂單例外與封鎖的例外是同一個「進行中」判定；首則訊息買家與店家都能開啟（W5）。
- 刪帳與退場只抹該身分寄出的訊息；等待刪除的買家仍能送出。
- 列表不因封鎖隱藏對話。
- `chat.message.sent` 的 `conversation_id`、`message_id` 是 UUID 字串。

`legacy/imrelay` 的 `typedWorker`、`baseEvent` 與其他舊 relay 的同名機制刻意重複（relay 是暫時的，notification 搬進單體後刪除），不抽共用。
