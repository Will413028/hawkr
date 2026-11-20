# A6-2b im 模組（即時通道）：實作設計 v1

- 對象：計畫 `../2026-11-19-a6-review-im-crm.md` 的步驟 A6-2b。計畫的決定（A6-D3、A6-D5、A6-D15）、不變量與 A6-2a 設計書 `step-2a-design.md` 是上位規則，本檔只把它們落到模組上；本檔與計畫衝突時以計畫為準，並在 §12 列出本檔提議的計畫修改。來源沒有定案的取捨列在 §13（W1–W5），等 Will 決定。
- 快照：dev `4670f1ea`（2026-11-20）。v2：已套用開工 design-review 的 7 項處理（§15）。
- 標記：【量】＝打開檔案或跑過指令；【推】＝讀程式後推論，或憑記憶、沒有在本次查證的外部事實。舊行為的出處都是本檔重新讀原始碼得到的 `file:line`。
- 路徑縮寫：`M=backend/internal`、`IM=backend/services/im-service/internal`、`GW=backend/gateway/internal`、`W=website`。
- 本次沒有跑任何測試、容器或網路呼叫（integration 只在 CI 跑）。

## 0. 前提題：這一步的程式之後會不會被重寫、搬走或刪除？

模組本體不會，它就是終態。過渡物與它們的退場點：

- 舊 im-service 的 hub、WebSocket、presence（`IM/usecase/hub.go`、`IM/grpc/chat_stream.go`）與 gateway 的 WebSocket 路由：A6-6e 刪（計畫 A6-6）。2b 不改它們，舊服務照常服務 gateway。
- 2b 沒有對外入口：Connect 串流 handler、proto 在 A6-5；`cmd/hawkr` 把 `Module.Run` 接上也在 A6-5（有 handler 才有訂閱者；§0.1）。2b 的程式在 A6-5 之前只被測試呼叫，這和 2a 的 `imrelay`（A6-6e 才接線）同一個處境。
- 即時通道的**發送端**（`NOTIFY`）現在就加進 `Send`、`Hide`、`Restore`、`MarkRead`、`MarkAllRead`：im 在 A6-6e 之前沒有任何正式寫入者（舊服務寫的是舊表），所以沒有副作用，也不需要開關。

一個要攤開的前提：計畫、A6-2a 設計書 §0、E7 寫的是「2b 的 hub 訂閱 `MessageSent`」。A6-D15 之後這句不成立，hub 不訂事件匯流排（§2.2）。

## 0.1 範圍：2b 與 A6-5 的分界

原則：2b 是「模組側的機制」，不需要 proto 與 HTTP 就能用 integration 測試證明；A6-5 只做「把機制翻成 Connect」。

| 項 | 2b（本步，模組） | A6-5（Connect） |
|---|---|---|
| 送出端 `NOTIFY` | `write` 套件在同一個 transaction 發（§2） | — |
| LISTEN 連線、訂閱表、扇出 | `im/internal/live`（§3、§4） | `cmd/hawkr` 呼叫 `im.Module.Run(ctx)`（與 handler 一起註冊） |
| 訂閱 API | `Module.Subscribe(ctx, SubscribeInput) (*Subscription, error)`、`Subscription.Next(ctx)`、`Subscription.Revalidate(ctx, Party)`（拉取式，§1）；只交出業務事件與業務結束原因 | `ChatService.Subscribe` 串流 handler：把 `Event` 轉成 proto；呼叫下列「串流傳輸 helper」，並在它的重驗 tick 內重新驗證 session、重讀店家後呼叫 `Revalidate` |
| 串流傳輸 helper（heartbeat、MaxLifetime＋jitter、逐次寫入 deadline、重驗節奏、`GuardUnavailable`） | 不在模組（ADR 第 64 行的義務屬傳輸層，與 im 無關） | `httpapi` 層一個可重用的 helper（暫名 `httpapi/rpc` 的 stream helper），A6-5 建，第一個串流 RPC 就是它的使用者；模組只提供 `Next`、`Revalidate`、`Close` |
| 授權 | 以 `Party`＋對話 ID（§5.1） | 從 session 解析 `Party`（ID6）；`SessionFrom`、`SuspensionFrom` |
| 結束串流的原因 | 業務原因 `Ended{Reason: Shutdown｜LostEligibility}`（§5.2） | 業務原因與 helper 自己的傳輸原因（壽命到期、session 失效、重驗暫時失敗）一起對到 Connect code 與 `ErrorReason` |
| 未讀與事件種類 | `Event` 的種類（§6） | proto `oneof`；訊息種類 enum 與文案目錄 |
| presence | 表、`Module.Presence` 讀取（§7） | `ChatService.GetPresence` unary（W2 選輪詢時） |
| 心跳 | 無（模組不產生 `Heartbeat`） | helper 在 `Next` 閒置時寫 heartbeat frame；代理緩衝的驗證（§5.3） |

好處：2b 的完成定義（§11）不依賴 `buf generate`、`openapi_coverage_test` 或 handler 測試；A6-5 的 handler 薄到只做轉譯，轉譯的測試不需要資料庫的併發情境。

## 0.2 不在範圍

- proto、Connect handler、gateway 與 `gatewayproxy` 的任何改動、三端 client（A6-5、A6-6）。
- 打字中：A6-D15 不做，三端的打字中在 A6-6 刪。
- 管理後台的聊天快照（A6-4，A6-D3）。
- 訊息抹除（刪帳）的即時推送：見 §6.3，不發。
- 水平擴展以外的規模量測：`NOTIFY` 的吞吐上限沒有量（§14）。

## 0.3 決策總表

| # | 題 | 落點 | 出處 |
|---|---|---|---|
| E1 | 傳輸 | server streaming 收、unary 送，沒有 WebSocket | A6-D3 |
| E2 | 即時通知 | Postgres `NOTIFY`，在送出同 transaction 發；PG 在 commit 後才送達；沒有 `platform/events` 的 commit 後掛點 | A6-D15 |
| E3 | 串流讀資料 | 一律經既有的 `MessagesAfter`；推送與斷線補洞同一條讀取路徑 | A6-D15 |
| E4 | 多進程 | 每個進程一條專用 LISTEN 連線；不再有單進程不變量 | A6-D15 |
| E5 | presence | 只對有對話的對方、不給最後上線時間 | A6-D5 (ii) |
| E6 | 打字中 | 不做 | A6-D15 |
| E7 | 身分 | 從 session 取，以對話 ID 授權，不收 client 帶的身分 | ID6；計畫 A6-2 |
| E8 | 中斷後復原 | 以游標補洞：server 不保存「誰收到什麼」，保存的只有訂閱者自己的游標 | 本檔 §4 |
| E9 | 慢消費者 | 合併（coalesce）待處理訊號，不丟訊息；超過上限回 `Resync` | 本檔 §4.3 |

## 0.4 查證過的現況【量】

**舊 im-service 的即時通道**
- hub 只在本進程：`MessageHub` 以三張記憶體 map（`clients`、`presenceSubs`、`lastSeen`）與 `register`／`unregister`／`broadcast` 三個 channel 運作（`IM/usecase/hub.go:14-34`），`Run` 是單一 goroutine 的 select 迴圈（`:37-125`）。這就是 ID4。
- **presence 不是 Redis**：它是 hub 的記憶體狀態：上線＝同一個 (userID, loginRole) 的第一條連線註冊（`:47-62`），離線＝最後一條斷開並記 `lastSeen`（`:81-104`，寫入 `:92`）。計畫與任務描述裡的「舊 Redis presence」不正確；Redis 在 im-service 只用於封鎖名單、收件人已刪帳標記與 sync 的速率限制（`IM/infrastructure/redis.go:11-13`；`IM/usecase/message.go:227,298-302`；刪帳標記見 A6-2a 設計書 §0.4）。單體的 `go.mod` 沒有 redis（`rg -n redis backend/go.mod` 無命中）。
- presence 以 (userID, loginRole) 為鍵，買家只看到「店家 App 在線」而不是「這個人的買家 App 在線」（`IM/domain/hub.go:3-7`）。
- presence 的訂閱沒有授權：`presence_sub` 只要 `to` 與 `peerRole` 合法就訂（`IM/usecase/client.go:54-58`，`parsePresenceKey` `:118-126`），任何人可以訂任何人（ID2）；每筆 presence 事件帶 `lastSeenAt`（`hub.go:316-329`）。
- 推送在 commit 之後、transaction 之外：`SendToUserRole` 在訊息寫入之後呼叫（`IM/usecase/message.go:625-638`），失敗只記 log；註解寫明「always attempt both WS and FCM … zero message loss」（`:620-622`）：WebSocket 從來不是送達保證，推播才是。
- 發送緩衝與丟棄：每條連線 256 格的 channel（`IM/grpc/chat_stream.go:20,43`），滿了就丟（`hub.go:176-181`）；丟掉的訊息靠 client 的 sync 補。
- 未讀推送：每次送出、已讀都算一次總未讀，推 `unread_changed`（`message.go:1131-1155`，呼叫點 `:640,967,974,1022`）。
- client 幀：`sync`（最多 20 個房間，`client.go:128-130`；每個 actor 速率限制 5 次，`message.go:285-288,298-302`；沒有權限的房間回與「沒有新訊息」相同的 `sync_done`，`client.go:143-153`）、`presence_sub`／`presence_unsub`、`typing`（每對象每秒 1 次，`client.go:13,107-114`）、`read_receipt`（只轉送給對方，不寫資料庫，`:74-81`）、`ping`→`pong`（`:82-`）。沒有 inbound 的 `chat`：送出是 unary（`client.go:36-39` 註解）。
- 強制斷線：`seller.suspended` 的 consumer 呼叫 `DisconnectUser`（`IM/worker/seller_suspended.go:14-21` 的註解；`hub.go:331-348`）。理由寫在註解裡：「不容既有長連線在停權後續傳非訂單關聯訊息」。但這條連線**只收**（上列沒有 inbound chat），這個理由在舊碼自己的設計下也不成立；現在送出是 unary，由 G1 逐則擋住（A6-2a §3.2）。
- 開啟連線的身分是 gateway 在第一幀帶的 `user_id`、`login_role`，im-service 不驗證（`IM/grpc/chat_stream.go:34,124-137`）；gateway 以一次性 ticket（TTL 30 秒，`GW/handler/chat_ws_ticket.go:18`）與每帳號 10 條連線上限（`GW/handler/chat_ws_connlimit.go:9`）把關，寫入逾時 10 秒（`GW/handler/constants.go:25`）。
- 三端 client 的狀態機常數：心跳 15 秒、30 秒沒有任何 frame 判定半開（`W/lib/chat/ws.ts:70,73`）、最多重連 5 次（`:79`）；三端各有一份（`client-inventory.md` §2.4）。

**單體**
- 沒有任何 `LISTEN`／`NOTIFY`／`pg_notify` 的使用【量：`rg -n "NOTIFY|LISTEN|pg_notify|WaitForNotification" backend/internal backend/cmd --type go -l` 無命中，只有 River 自己的依賴內部會用，與本模組無關【推】】。
- `platform/db` 只暴露連線池（`DB.Pool()`，`M/platform/db/db.go:93-94`），設定可由 `Pool().Config().ConnConfig` 取得；註解已預告若前面放 transaction pooling 的 PgBouncer 要改設定（`db.go:44-46`）。`LISTEN` 要 session 層級的連線，與 transaction pooling 不相容【推：PgBouncer 的已知限制，憑記憶】。
- 測試端有 `testdb.Server.NewURL(t, steps...)`（`M/platform/testdb/testdb.go:225`），可以拿到連線字串開專用連線；`pg.NewDB` 給的 `*db.DB` 可以 `Pool().Config()`。
- events：`Publish` 必須在 transaction 內、handler 同步執行、任一失敗整個 rollback（`M/platform/events/bus.go:1-8,15-21`）。這是「原子性通知」機制，不是「commit 後」機制。
- `cmd/hawkr` 的 `serveAll` 在 ctx 結束後呼叫 `http.Server.Shutdown`，等進行中的請求最多 30 秒（`cmd/hawkr/main.go:65-66,537-541`）。`Shutdown` 不會取消進行中的 handler【推：標準函式庫行為】，所以長壽命的串流要自己聽 ctx，否則每次部署都等滿 30 秒（§5.2）。
- HTTP 入口收 HTTP/1.1 與 h2c（`main.go:508-514`）。
- session：`Authenticate` 解析 token，閒置 7 天（web）／14 天（App）、絕對 90 天（`M/modules/identity/internal/session/session.go:26-30`），解析時一天最多續期一次（`:120-141`）；停權帳號照樣登入（`:121-122` 註解），能做什麼由 `httpapi.withSession` 依動作性質決定（`M/httpapi/session.go:20-31`）：停權時讀取放行、寫入回 `ACCOUNT_SUSPENDED`。
- seller 的能力表：`CapChatRead`、`CapChatSend` 都只需要 `RequirementActive`（`M/modules/seller/internal/model/capability.go:62-65`）；停權時仍授予，關店（CLOSED）時沒有（`:55-60` 註解「關閉的店家沒有任何能力」）。`Party.Authorize` 對店家檢查 `CapChatRead`（`M/modules/im/internal/model/model.go:64-72`）。
- 2a 已有的讀取：`MessagesAfter` 以 `(conversation_id, seq > after)` 讀、舊的在前，每次先 `authorized`（`M/modules/im/internal/read/read.go:122-134`）；隱藏與抹除的訊息回墓碑；`UnreadTotal` 不 join 訊息（`:146-155`）。
- `AllocateSeq` 的 `ON CONFLICT DO UPDATE` 取得對話列鎖，直到 transaction 結束才放（A6-2a §2）：同一個對話的兩則訊息，seq 較大者必定在 seq 較小者 commit 之後才 commit【推：列鎖語意】。這是 §4.2「補洞不會漏」的前提。
- 2a 的 `MarkRead` 的 SQL 回傳 `read_seq` 與 `last_seq`，不回「游標是否真的前進」（`im/db/queries/conversations.sql:23-28`）；`Hide` 取得 `LockMessage` 但沒有回傳 conversation、seq（`write.go:109-122`）。§8 列要改的查詢。
- 沒有串流 RPC：`rg -n "stream " api -g '*.proto'` 無命中【量】；ADR D3、D7 與 `API 設計規範（Connect）` §十一只規定「server streaming、續傳用請求游標」，並寫下第一個串流要處理的三件事：逐次寫入 deadline、續傳游標、經 gateway 與單體的不緩衝測試（ADR `2026-09-26-external-contract-protobuf-connect.md:64`）。
- website 的本機代理 `passThroughToUpstream` 以 `fetch` 轉回串流、帶 `req.signal`（`W/lib/api/local-proxy.ts:144-181`）；`app/api/connect/[...path]/route.ts` 只在本機啟用，部署環境由 web host 的第一方反向代理處理 `/api/connect` 前綴（該檔頭註解）。

## 1. 模組 API（`im` 根套件新增）

```go
// 訂閱：一個 Party 一條拉取式的事件流。
type SubscribeInput struct {
    Party   Party
    // Cursors 是 client 已經有的每個對話的最後 seq（≤ MaxCursors）；對話不屬於 Party 的項目被忽略，結果與「沒有新訊息」不可區分。
    Cursors map[uuid.UUID]int64
}

func (m *Module) Subscribe(ctx context.Context, in SubscribeInput) (*Subscription, error)
func (s *Subscription) Next(ctx context.Context) (Event, error) // 阻塞到下一個業務事件（沒有 Heartbeat）；串流結束回 *Ended
// Revalidate 由串流傳輸 helper 在重新驗證 session、重讀店家之後呼叫：以新的 Party 取代舊快照並重跑 Party.Authorize；
// 失去讀取資格（關店）→ 訂閱以 Ended{LostEligibility} 結束。session 失效、帳號刪除屬 identity，由 helper 自己結束串流，不經模組。
func (s *Subscription) Revalidate(ctx context.Context, p Party) error
func (s *Subscription) Close()

// Run 持有 LISTEN 連線與 presence 的延長批次，直到 ctx 結束；ctx 結束時所有訂閱以 Shutdown 結束。LISTEN 連線來自 Deps.ListenConnConfig（§3.1）。
func (m *Module) Run(ctx context.Context) error

// Presence 回報呼叫者的對話的對方是否在線（§7）。
func (m *Module) Presence(ctx context.Context, p Party, conversationIDs []uuid.UUID) ([]PresenceState, error)
```

- 拉取式（`Next`）而不是 channel：呼叫端（handler）還在寫上一個 frame 時，模組不會往它的緩衝裡堆東西，慢消費者的壓力自然落在合併的待處理集合（§4.3），不需要另外設計「緩衝滿了怎麼辦」。
- `Event` 的種類、`Ended` 的原因、`PresenceState` 的欄位見 §5.2、§6、§7；`PresenceState` **沒有最後上線時間欄位**（A6-D5），欄位不存在比欄位留空更不會被誤用。
- `Subscribe` 的錯誤：身分不完整或店家沒有 `CapChatRead`（關店）→ 沿用 `Party.Authorize` 的錯誤；超過每 Party 的全域並行上限（§4.3，以 `presence_streams` 計數）→ `ResourceExhausted`；`Cursors` 超過上限 → `InvalidInput`。
- 2a 的 `im.Module` 沒有 `Run`，新增它不改變既有方法的簽名。`write.Deps`、`read.Deps` 的變更見 §8。

## 2. 發送端：`NOTIFY` 與 payload

### 2.1 頻道與 payload

- 單一頻道 `im_events`，不分 Party 頻道。理由：Party 頻道（如 `im_b_<id>`）要求 LISTEN 連線隨訂閱者進出動態 `LISTEN`／`UNLISTEN`，連線在斷線重連時還要重建整張清單；單一頻道讓連線狀態恆定（一條 `LISTEN im_events`），過濾在進程內做。代價是每個進程收到全部對話的通知（§2.3 的成本）。
- payload 是緊湊 JSON（PG 的 payload 預設上限 8000 bytes【推：PG 文件，憑記憶】）：

```json
{"v":1,"k":"m","c":"<conversation uuid>","b":123,"s":456,"q":17}
```

| key | 意義 |
|---|---|
| `v` | payload 版本，監聽端遇到不認得的版本忽略並計數 |
| `k` | 種類：`m` 新訊息、`v` 可見度改變（隱藏／還原）、`r` 某一方的已讀游標前進、`u` 某一方的批次已讀 |
| `c` | 對話 ID（`u` 沒有） |
| `b`、`s` | 對話的 buyer_id、store_id（`u` 只有被標已讀的那一方） |
| `q` | 相關 seq：`m` 是新 seq、`v` 是被改可見度的那則的 seq、`r` 是新游標值（`u` 沒有） |
| `d` | 只有 `r`：`"B"` 或 `"S"`，哪一方的游標前進 |

- payload 最大長度：UUID 36 字元＋兩個 int64 十進位 19 字元＋其餘欄位約 100 bytes，遠小於 8000【量：以上述欄位長度相加；測試 T14 以最大值斷言】。
- **這個 payload 比 A6-D15 字面的「只帶對話 ID 與 seq」多了 `b`、`s` 與種類**：理由與替代見 W1。內容不含任何訊息文字、名稱或個資，只有業務鍵（外部 ID）。

### 2.2 發出的位置：直接在 `write` 的 transaction 內，不經 `events.Publish`

- `write.Send`（`write/send.go:107` 的 `events.Publish(MessageSent)` 之後）、`Hide`、`Restore`、`MarkRead`、`MarkAllRead` 在各自的 `InTx` 內以 `SELECT pg_notify('im_events', $1)`（經 `db.Conn(ctx)`，即同一個 transaction）發出。PG 保證 `NOTIFY` 在 transaction commit 時才排入、rollback 就不發【推：PG 語意，憑記憶；T1 測試證明】。
- 為什麼不是「訂閱 `MessageSent` 的同 tx handler 去 `NOTIFY`」：(1) 訂閱的註冊在 `cmd/hawkr`，而 relay 與 `imrelay` 的註冊刻意延到 A6-6e；那條路要嘛讓即時推送依賴一個沒接線就靜默失效的註冊，要嘛把 NOTIFY 的註冊提早到 A6-2b，模組單獨用 `im.New` 就不會有推送。(2) `Hide`、`Restore`、`MarkRead` 沒有事件可訂，要為即時通道新增三個「只給自己用」的事件型別。直接在 `write` 發，模組自己的 integration 測試就能完整證明「commit 後才推」。(3) `NOTIFY` 不是業務事件，不該與跨模組的事件契約混在一起。這與 A6-D15「不在 `platform/events` 加 commit 後掛點」一致。
- `MarkAllRead`：一次標很多對話。逐對話的 `r` 會在一個 transaction 內發出無上限個 `NOTIFY`，所以只發**一個** `u`（只帶被標已讀的那一方的 buyer_id 或 store_id），對方不收到 `PeerRead`（§6.2）。
- `MarkRead` 只在游標真的前進時發 `r`（§8）；沒有前進（冪等重送）不發。
- 同一個 transaction 內相同 channel＋payload 的 `NOTIFY` 會被 PG 合併成一個【推：PG 文件，憑記憶】；`m` 與 `v` 帶 seq，所以不同訊息不會被誤合併。

### 2.3 成本與已知限制

- 每個進程收到全部對話的通知，進程內只對有訂閱者的 Party 做事（§3.3）：沒有本機訂閱者的通知，成本是一次 JSON 解析與兩次 map 查詢，不碰資料庫。所以 payload 帶 `b`、`s`：沒有它們，每個進程都得為每則通知查一次資料庫才知道要不要處理（W1）。
- `NOTIFY` 的 commit 路徑在 PG 內有全域鎖，高寫入率下會讓 commit 串行化【推：憑記憶，沒有量】。聊天的寫入率遠低於會發生這件事的量級，但沒有數字支持這句；觸發條件與替代（outbox 輪詢）寫在 §14，不在 2b 做。
- 沒有任何進程在 LISTEN 時，`NOTIFY` 不留下任何東西，不累積佇列【推：PG 語意】。

## 3. 監聽端：LISTEN 連線與扇出（`im/internal/live`）

### 3.1 專用連線

- 每個進程一條專用連線，**不從連線池借**：借走一條長期不歸還的連線會永遠減少池容量，也會被池的 `MaxConnLifetime`、健康檢查回收；專用連線不受影響。代價是多佔資料庫一條連線（每進程一條，要算進 `max_connections`）。T16 斷言 listener 運行時 `Pool().Stat().AcquiredConns()` 沒有增加（`db_integration_test.go:184` 的同一個斷言方式）。
- **不變量（寫進 `im/AGENTS.md`）：LISTEN 連線必須是直連或 session 模式的 PG，不能經 transaction pooling（§0.4）。** 做法是明確設定而不是偵測：`Deps.ListenConnConfig *pgx.ConnConfig` 指向專用的直連連線字串（A6-5 的 `cmd/hawkr` 從獨立的環境變數讀入）；為 nil 時由 `pgx.ConnectConfig` 以主 DSN（`pool.Config().ConnConfig.Copy()`）建立，此時主 DSN 同樣受上述不變量約束，部署文件記載。不在執行期以 `SHOW` 或 LISTEN 往返推測連線型態：那種偵測無法在 CI 證明（CI 的資料庫沒有 pooler），寫了也只是未被測試的程式。違反不變量的症狀是收不到通知，由 T2 在部署前的 smoke 與 §14 的部署確認項目承擔。
- `Run` 的迴圈：連線 → `LISTEN im_events` → `WaitForNotification(ctx)` 迴圈把 payload 交給扇出（§3.3）；連線出錯（`WaitForNotification` 回錯、`LISTEN` 失敗）→ 關連線，以指數退避加 jitter（0.5 秒起、上限 30 秒）重連。每 10 秒在同一條連線上以 `SELECT now()` 記下「最後確認存活的資料庫時間」`lastOK`（同時讓死連線在 10 秒內被偵測）。**每次連線成功後**（含第一次）對所有訂閱發一個內部的 `epoch` 訊號，並帶 `lastOK`（§4.4）；第一次連線的 `lastOK` 是 `Run` 啟動時、第一次撥號之前經連線池讀到的資料庫時間（讀不到就退避重試，成功前不撥號），所以在第一次 LISTEN 成功之前就開的串流也會補洞。重連與退避是手寫的，沒有用 `jackc/pgxlisten`：見 H26。連線期間沒有人會收到的通知（斷線區間內的 `NOTIFY`）永遠不會補送，PG 不保留【推】。

### 3.2 為什麼斷線不是問題

訂閱者不依賴「每一則通知都收到」：通知只是「對話 C 在 seq≥q 有東西」的提示，內容從資料庫經 `MessagesAfter` 以自己的游標讀（§4.2）。所以：漏掉通知＝晚一點、最差到下一次通知、`epoch` 訊號或 client 的下一次補洞才讀到，不會漏資料、不會重複。這與 A6-D15「推送與斷線補洞同一條讀取路徑」是同一件事。

### 3.3 扇出

- 進程內一個 `Hub`：`subs map[partyKey]map[*sub]struct{}`，`partyKey = (kind, id)`，以 `sync.RWMutex` 保護。**鍵含 kind**：buyer_id 5 與 store_id 5 是不同的 Party（ID6 的同一個數字撞兩種身分空間，T6）。
- 收到通知：解析 payload（解析失敗或版本不認得 → 丟棄並計數，不影響連線）→ 依 `k` 找 `(BUYER, b)`、`(STORE, s)` 的訂閱者（`u`、`r` 依 `d` 只找該方或對方，§6）→ 對每個訂閱者呼叫 `sub.mark(signal)`。`mark` **永不阻塞**（§4.3）。
- 訂閱的登記與移除在 `Subscribe`／`Close`：`Close` 要冪等，ctx 取消與 handler 返回都會呼叫。

## 4. 訂閱者：游標、補洞與慢消費者

### 4.1 狀態

每個 `sub` 有：`party`、`delivered map[uuid.UUID]int64`（每個對話已送出的最後 seq，上限 MaxDelivered）、`pending`（合併後的待處理訊號：對話 ID → `minQ`、`maxQ` 與種類旗標）、一個容量 1 的喚醒 channel、`Revalidate` 換入的 `Party` 快照。心跳、重驗與壽命計時器不在 `sub`，在 helper（§5.3）。

### 4.2 讀取路徑：一條

`sub` 的 `Next` 在有待處理訊號時，**對每個有訊號的對話**呼叫模組已有的 `MessagesAfter(conversation, party, base, size)`：

- `base`：`c` 在 `delivered` 裡 → `delivered[c]`；否則 → `pending[c].minQ - 1`，**不是**合併後的最大 `q - 1`。合併把同一對話的多個訊號收成一格，若取最大 `q` 當讀取起點，兩則在第一次 `Next` 之前到達的訊息會只交出較新的那一則，較舊的被丟給 client 的缺口補洞（§4.5），等於讓合併悄悄損失訊息。從 `minQ - 1` 讀會一併讀到後面所有 seq，之後 `delivered[c]` 前進、更大的 `q` 因 `q ≤ delivered` 略過。`mark` 對同一對話維護 `minQ = min(舊, q)`、`maxQ = max(舊, q)`。
- 讀到的訊息依序交出（`Event.Message`），`delivered[c]` 前進到最後一則；`HasMore` 時繼續讀，每個對話一次最多 MaxBacklog 則，超過就把它從 `delivered` 移除並交出 `Resync{conversation}`（client 用 unary 的 `ListMessages` 補）。
- 可見度改變（`v`）：同一對話的多個 `v` 以 `pending` 的旗標集合合併成「要重讀的 seq 集合」（每則 `v` 的 seq 各自保留，不取最大），逐個以 `MessagesAfter(c, party, seq-1, 1)` 重讀該 seq 那則，交出 `Message{Updated: true}`（墓碑或還原後的內容，同一個 seq，client 以 seq 取代）；不改 `delivered`。
- 每次讀取都走 `authorized`（對話屬於這個 Party），所以一個 Party 永遠讀不到別人的對話，即使通知的 `b`、`s` 被偽造或過濾有誤：授權在讀取，不在過濾（T6）。
- 不存在「推送用的另一條資料路徑」：沒有任何訊息內容經 `NOTIFY` 或在進程間以別的形式傳遞。

**為什麼不會漏（兩個命題）**
1. 同一對話的 seq 較大者必在較小者 commit 之後才 commit（§0.4，列鎖）。所以任何時刻，已 commit 的 seq 是連續前綴；`MessagesAfter(after = delivered)` 不會跳過一則「還沒 commit 但 seq 較小」的訊息。【推：列鎖語意；T3、T4 以併發測試佐證】
2. 通知先於讀取：`Subscribe` **先**登記進 `Hub`，**再**讀 `Cursors` 的補洞，最後才開始處理即時訊號。登記與補洞讀取之間 commit 的訊息，要嘛被補洞讀到（`delivered` 已前進，之後的通知因 `q ≤ delivered` 略過），要嘛在補洞讀完之後以通知處理；兩者都不漏，也不重複（T4）。

### 4.3 慢消費者與背壓

- 扇出端（監聽 goroutine）只做 `sub.mark`：在 `sub` 自己的互斥鎖下更新 `pending[c]`（`minQ = min(舊, q)`、`maxQ = max(舊, q)`，§4.2），再非阻塞地喚醒；**不寫任何 channel 緩衝、不等待消費者**。一個停住的 handler 不會拖慢監聽連線、其他訂閱者或 `Send`。
- `pending` 以對話合併（只保留 `minQ`、`maxQ` 與 `v` 的 seq 集合），所以記憶體上限是這個 Party 的對話數，再加硬上限 MaxPending：超過時清空 `pending` 並設 `overflow`，下一次 `Next` 交出 `Resync{all}`，client 依 `ListConversations` 重建。
- 為什麼是「合併」而不是「丟棄後讓 client 補」或「阻塞」：舊碼是丟棄（`hub.go:176-181`），丟了就靠 client 的 sync；合併不會丟東西，因為訊息內容本來就不在緩衝裡（§4.2）；阻塞會讓一個停住的連線扣住整個監聽迴圈。
- 真正的慢（handler 的 `stream.Send` 卡住）由串流傳輸 helper 的逐次寫入 deadline 處理（ADR 第 64 行列為第一個串流 RPC 的義務；helper 在 A6-5 建，§0.1）；deadline 到期 handler 返回，`Close` 釋放訂閱。2b 提供 `Close`，A6-5 提供 deadline。
- 每個 Party 的並行訂閱上限 MaxStreamsPerParty＝10，**全域**：多進程下每進程各算 10 條沒有意義。`Subscribe` 在一個 transaction 內以 `pg_advisory_xact_lock(hashtextextended(kind || ':' || party_id, 0))` 序列化同一 Party 的訂閱，數 `im.presence_streams` 中該 `(kind, party_id)` 且 `expires_at > now()` 的列，已達上限回 `ResourceExhausted`，否則寫入本串流的列（§7.2）。這個列同時是 presence 的來源，不另建計數表。advisory lock 是 session 外的交易鎖、不鎖業務列，不進 `lockorder` 的順序（T17 驗證）。崩潰的串流在 PresenceTTL（60 秒）內仍佔名額，這個暫時性偏緊可接受（上限的目的是擋單一帳號耗盡資源）。**寫入失敗時 `Subscribe` 失敗**（上限執行是 fail-closed）；延長批次連續失敗到列可能過期（2 次）時，`Run` 以 `Shutdown` 結束本進程所有訂閱，讓上限在延長失敗時也不放寬（§7.2）。舊 gateway 每帳號 10 條（`chat_ws_connlimit.go:9`）是進程內 map（`:21-28`），所以全域化是行為上的加強，不是退步。

### 4.4 監聽連線中斷後的復原

`Run` 每次連線成功後對每個 `sub` 呼叫 `sub.epoch(lastOK)`：(1) 對 `delivered` 裡的每個對話視為有訊號（`pending[c] = {minQ: delivered[c]+1, maxQ: ∞}`，`Next` 時以 `base = delivered[c]` 讀到最新），**並各交一個 `Resync{conversation}`**：中斷期間的隱藏、還原與對方已讀的 NOTIFY 漏掉了，那些變化不經 seq 游標補得回來，client 依 §4.5 重取該對話（訊息補讀與 Resync 同時存在，client 以 seq 去重）；(2) 對**不在** `delivered` 的對話，只對中斷期間有訊息的那些交出 `Resync{conversation}`，而不是 `Resync{all}`：`im.conversations` 有 `last_message_at TIMESTAMPTZ NOT NULL`（`im/db/schema.sql:22`），並有 `(buyer_id, last_message_at DESC, id DESC)`、`(store_id, …)` 兩個索引（`schema.sql:31-32`），每次送出由 `AllocateSeq` 的 upsert 寫入（`queries/conversations.sql:6,17`）。`epoch` 以 Party 的那一方查 `WHERE <buyer_id|store_id> = $id AND last_message_at > $lastOK`，結果不在 `delivered` 的對話各交一個 `Resync{conversation}`（一次一個索引範圍掃描，筆數有 MaxPending 上限，超過就 `Resync{all}`）。`lastOK` 是監聽連線最後一次確認存活的**資料庫**時間（§3.1），兩邊都是資料庫時間：2b 把 `AllocateSeq` upsert 寫 `last_message_at` 的值從注入的 `Now` 改成資料庫 `now()`（§8），與 H25 同一原則，不需要 skew，也不假設主機跑 NTP。`last_message_at` 只在新訊息時前進；中斷期間只有可見度或已讀游標改變、而且不在 `delivered` 的對話，client 沒有該對話的游標、本來就會在打開時重新取，不需要 Resync；在 `delivered` 裡的對話一律 Resync（上述 (1)）。這解掉了「每次監聽重連，所有訂閱者同時收到 `Resync{all}` 並同時打 `ListConversations`」的重連風暴。重連期間 `Subscribe` 仍可呼叫：它先登記、再讀補洞，與 §4.2 命題 2 相同。epoch 的讀取量 ≤ 訂閱數 ×（`delivered` 對話數次 `MessagesAfter` ＋ 一次索引掃描），每次走 `(conversation_id, seq)` 唯一索引【推】；沒有量。

**時間比較的已知界線（2b1 修正輪）**：`last_message_at` 由 `GREATEST(舊值, clock_timestamp())` 寫入（不倒退，且就是這則訊息的 `created_at` 與 `MessageSent.OccurredAt`，`AllocateSeq` 回傳同一個值）。寫入 transaction 的開始早於 `lastOK`、提交晚於斷線的送出，其 `last_message_at`（語句時間）仍可能小於 `lastOK`，補洞以 `last_message_at > lastOK` 會漏掉它；要補這個縫需要「im 寫入 transaction 最長開多久」的界線（`lastOK - margin`），而 repo 沒有任何這個界線（連線池、`InTx`、`statement_timeout`、`idle_in_transaction_session_timeout`、`transaction_timeout` 都沒有設定，httpapi 也沒有請求 deadline），所以本步不補、不發明數字；待決與觸發條件記在 `CARRYOVER.md`。

### 4.5 client 契約（給 A6-5、A6-6）

- 串流開啟時帶 `cursors`（最近活動的對話 ≤ MaxCursors＝50；舊 sync 一幀 20 個，`client.go:128-130`，那是「draft value」，這裡也是估計值，上限可調）；收到的訊息帶對話 ID 與 seq；**client 以 seq 是否連續判斷缺口**，缺口時用 unary 的 `MessagesAfter`／`ListMessages` 補。這個契約讓伺服器不需要知道 client 有沒有收到。
- `Ready` 事件標出補洞結束（取代舊的每房間 `sync_done`，`client.go:170-185`）。
- `Resync` 事件＝「請重新取列表與缺口」，client 不需要區分原因；`Resync{conversation}` 要重取該對話的訊息（含隱藏狀態）與對方已讀游標，`Resync{all}` 重取列表與所有開著的對話。同一個對話的 Resync 可能與補洞的訊息事件一起到，client 以 seq 去重，不假設事件順序。
- sender 的其他裝置也會收到自己寄出的訊息（兩個 Party 都收到 `m`，§6）；送出的那一台以 `Send` 回傳的訊息 ID 去重。這是行為改變：舊碼只推收件人（`message.go:625-638`），寄件人的其他裝置要等下次列表。

## 5. 授權與結束

### 5.1 訂閱時

- 身分：`Party` 由 A6-5 的 handler 從 session 解析（`rpc.SessionFrom`、principal），模組不收 client 帶的 `user_id`／`login_role`（舊碼 `chat_stream.go:124-137`；ID6）。一個擁有買家與店家兩種身分的帳號，要開兩條串流（各一個 Party）；兩條不共用狀態，這對應舊碼的「per-role」presence 語意。
- 授權：`Party.Authorize`（店家要 `CapChatRead`，關店後沒有）；`Cursors` 的對話以 `authorized` 檢查，不屬於者靜默忽略（與舊碼 `client.go:143-153`〔#2499〕同一個目的：不讓 client 用游標探測對話是否存在）。

### 5.2 結束串流：業務原因與傳輸原因的分界

模組只有一個業務條件和一個機制：`Next` 回 `*Ended{Reason}` 之後訂閱已關閉。

| 原因 | 誰產生 | 觸發 |
|---|---|---|
| `Shutdown` | 模組 | `Run` 的 ctx 結束（部署、停機），即時 |
| `LostEligibility` | 模組（`Revalidate`） | 以新 `Party` 重跑 `Party.Authorize` 失敗：店家已關閉（`CapChatRead` 消失，`capability.go:55-60`） |
| client 取消 | 模組（ctx） | ctx 取消，即時 |

下列是**傳輸層**的結束條件，歸串流傳輸 helper（A6-5，§0.1），不在模組：MaxLifetime（預設 15 分鐘，±10% jitter）、逐次寫入 deadline、重驗節奏 RecheckEvery（60 秒）與 `Guard` 的暫時性錯誤計數（`GuardUnavailable`，連續 3 次）、session 失效（`Authenticate` 回 Unauthenticated）與帳號刪除。helper 每個 RecheckEvery 呼叫 handler 提供的 `Guard`（重新驗證 session、重讀店家）；`Guard` 成功就把新的 `Party` 交給 `Subscription.Revalidate`，失敗就由 helper 自己結束串流並選擇 Connect code。模組對 `Guard`、壽命、心跳、deadline 一無所知，這讓模組的測試不需要時鐘或傳輸，也讓第二個串流 RPC 直接重用 helper。

- **停權不結束串流**（W4）：串流只讀。店家停權時 `CapChatRead` 仍授予（`capability.go:62-65`），帳號停權時讀取放行（`httpapi/session.go:20-31`，A7-D18），送出由 G1 逐則擋（A6-2a §3.2）。舊碼的強制斷線（`seller_suspended.go`）理由是「不容既有長連線續傳」，而連線只收，這個理由在舊碼自己的設計下已經不成立（§0.4）。計畫 A6-2「停權、停業、session 失效時結束串流」改成「關店時由模組結束；session 失效、帳號刪除由串流傳輸 helper 結束」（§12 第 2 項）。
- **封鎖不結束串流**：A6-D1 與 A6-2a §5.1：封鎖只影響送出，列表與讀取不過濾；封鎖發生時串流上不需要任何動作。要結束串流的業務條件只有失去讀取資格，封鎖不是。
- 重驗用 `Guard`，不在 im 內自己查 identity：依賴表的 im 列沒有 identity（`M/arch/modules.go:30`，A6-2a §4.1），由 handler（屬 `httpapi`，它本來就依賴 identity）提供閉包：`Authenticate(token)`、`seller.Store` 重讀。**stale `Party`**：`Party.Store` 是請求當下的快照（`model.go:47-51`），店家狀態在串流期間會變，所以 helper 把 `Guard` 回傳的**新** `Party` 交給 `Revalidate`；T8 驗證關店時 `Revalidate` 以 `LostEligibility` 結束。
- `Guard` 的成本（helper 的成本，屬 A6-5）：每 60 秒一次 `Authenticate`（一次查詢，一天最多一次寫入續期，`session.go:136-141`）加店家的一次讀取；10 個並行串流每分鐘約 10 次【推：未量】。串流存在本身會讓 session 的閒置期持續被續期，直到絕對逾時 90 天；壽命結束後 client 重連走一般的 `withSession`，沒有新增的繞過路徑。
- 為什麼用輪詢而不是事件推送撤銷：session 撤銷在 identity，帳號停權在 `accountsuspension`（im 不依賴它們），要事件就得讓這兩個模組為 im 發 NOTIFY 或新增依賴，只為縮短 60 秒的延遲；而串流只讀，延遲期間的風險是「已撤銷的 session 多收到最多 60 秒自己的訊息」。

### 5.3 心跳與長連線（歸串流傳輸 helper，A6-5）

本節是給 A6-5 的 helper 的規格，不是 2b 的程式。ADR 第 64 行把逐次寫入 deadline 與不緩衝測試列為第一個串流 RPC 的義務，heartbeat 與壽命是同一類傳輸問題，所以放同一個可重用的 helper，不放進 im。

- heartbeat：`Next` 閒置 HeartbeatEvery（預設 15 秒）時 helper 寫一個 heartbeat frame；與三端 client 現有的 15 秒（`ws.ts:70`）同值，client 的「30 秒沒有任何 frame 判定半開」（`ws.ts:73`）原樣保留，等於容忍錯過一個心跳。`Ready` 事件由 helper 補上心跳間隔與最長壽命（模組的 `Ready` 只帶總未讀，§6.1）。
- 為什麼需要：website 在部署環境是第一方反向代理（`apiProxyRewrites` 的 rewrites 與 web host 的反代，`local-proxy.ts:118-142`；`route.ts` 檔頭），中間任何一層對「閒置連線」或「總時長」都可能有上限；沒有資料的串流會被當成閒置切掉。**這些上限的實際值沒有查證**【推，憑對一般反向代理與託管平台的了解，沒有量；Cloudflare 與 web host 的設定都不在這個 repo】：因此 MaxLifetime 取 15 分鐘（遠低於常見的小時級上限，高於重連成本），並且在 A6-5／A6-6a 的驗收裡實測「經 web host 反代、經 Cloudflare 的串流不被緩衝、閒置 60 秒以上不被切」（ADR 第 64 行的義務）。
- 本機：`passThroughToUpstream` 逐筆轉回、帶 `req.signal`（`local-proxy.ts:144-181`），串流在本機可用；它沒有逾時，本機驗證不涵蓋中間層的行為（人工項目）。
- App 的半開連線由 connectrpc 的 `pingInterval` 偵測（ADR 第 64 行指向試點 6.6）；heartbeat frame 是應用層的第二道。
- 壽命結束不是錯誤：client 以最後的 seq 游標重連（§4.5）；jitter 避免一次部署後所有 client 同時到期。

### 5.4 關閉時的清理

`Subscription.Close` 與 `Run` 結束：移出 `Hub`、刪 presence 列（§7，同時釋出全域名額）。`Run` 結束時所有 `Next` 以 `Shutdown` 返回，handler 隨之返回，`http.Server.Shutdown` 才不會等滿 30 秒（§0.4）。A6-5 把 `Run` 的 ctx 與 `serveAll` 的 ctx 接成同一個。

## 6. 事件種類（含未讀）

### 6.1 `Event`

| 種類 | 內容 | 何時 |
|---|---|---|
| `Ready` | 目前總未讀 | 補洞完成後一次（helper 轉成 frame 時另加心跳間隔與最長壽命，§5.3） |
| `Message` | `Message`、`Updated`（`true`＝同 seq 的可見度改變） | 新訊息、隱藏、還原 |
| `Unread` | 總未讀（`UnreadTotal`，一次索引上的 SUM） | 見下 |
| `PeerRead` | 對話 ID、對方游標 seq | 對方 `MarkRead` 前進（W5） |
| `Resync` | 對話 ID 或全部 | §4.2、§4.3、§4.4 |

### 6.2 未讀

- 未讀的真相仍是 2a 的 `last_seq - 自己的游標`（A6-2a §1.2）；串流不維護計數，只在狀態改變後讀一次總數交給 client（取代舊的 `unread_changed`，`message.go:1131-1155`；website 的 `useChatUnreadStream` 只在 WS 重連時打一次 REST 基準，其餘靠這個事件，`W/lib/chat/useChatUnreadStream.ts:6-12`）。
- 發出時機：處理完一輪 `m`（收件人那一方）、收到 `r`（自己這一方，例如另一台裝置已讀）、收到 `u`（自己這一方）之後，**每輪最多算一次** `UnreadTotal`（合併，避免一批訊息算 N 次）。
- `PeerRead`：`r` 帶 `d`＝讀的那一方；另一方的串流交出 `PeerRead{conversation, seq}`。要有 `PeerRead` 就要有「對方游標」的讀取：2a 的 `Conversation`／`Message` 讀取沒有對方游標，2b 加一個 `PeerReadSeq`（§8）。`MarkAllRead` 的 `u` 不產生 `PeerRead`（W5）。
- 舊 `read_receipt` 是 client 發的幀、伺服器只轉送、不落庫（`client.go:74-81`）；新做法是游標進資料庫、由資料庫的游標推導，client 不需要「發」已讀回執，這也是 unary-only 送出下唯一合理的做法。

### 6.3 不發的事件

- 抹除（刪帳）：`erase_party` 一批最多 200 則（A6-2a §4.4），逐則 `v` 會在一個 transaction 內發 200 個 NOTIFY；已開著的畫面在重新整理或下一次補洞時才看到墓碑。理由：抹除的對象是離開的人的舊訊息，不是即時互動；client 端已經持有的副本本來就收不回。觸發條件：產品要求「刪帳後已開的對話立刻變墓碑」。
- 對話建立：首則訊息就是 `m`，client 收到不認得的對話 ID 時自己 `ListConversations`。
- 打字中、presence 變化：不在串流上（打字中依 A6-D15；presence 見 §7 與 W2）。

## 7. Presence（A6-D5 (ii)）

### 7.1 前提題與選項

A6-D5 已決定要顯示在線、只對有對話的對方、不給最後上線時間；本節只決定「怎麼做」。舊的做法是進程記憶體（§0.4），多進程時每個進程只看到連在自己身上的人。業界基準：Slack、Discord 把 presence 放在共享狀態或專用服務、以心跳＋過期判定在線；單機以下的系統用資料庫列＋過期。

| | 做法 | 多進程 | 狀態 | 成本 |
|---|---|---|---|---|
| (a) 資料庫列＋心跳過期，**輪詢讀取** | 開串流時寫一列，每 20 秒由 `Run` 批次延長，關閉時刪（同一列兼作全域並行上限的計數，§4.3）；`Presence` 查「有未過期的列」 | 成立（共享真相是資料庫） | `im.presence_streams`（UNLOGGED，§7.2） | 開／關各一次寫入，每進程每 20 秒一個批次 UPDATE；讀取是索引查詢 |
| (b) 同上，**加上在串流上推送變化** | (a)＋上下線時 `NOTIFY kind=p`，對方的串流交出 `Presence` 事件 | 成立 | 同 (a)＋離線（崩潰過期）的事件要另一個掃描者 | 多一種通知、一個掃描器、訂閱端要追蹤「我要看誰」；而且串流是單向的，client 無法告訴伺服器「我現在在看哪個對話」，只能對所有對話推 |
| (c) 進程記憶體＋NOTIFY 互相廣播 | 舊做法加上跨進程同步 | 要快照協議，新進程啟動時不知道別人在線 | 無表 | 最複雜，沒有共享真相 |

建議 (a)：presence 是顯示用的輔助資訊，最多晚 20～60 秒是可接受的；輪詢（unary）沒有訂閱狀態、授權逐次判定、可限流，而 (b) 要為單向串流補一個「我在看誰」的反向通道（server streaming 沒有 inbound），或對全部對話推送。這是真的取捨（推送的即時性 vs 狀態與複雜度），列為 W2。

### 7.2 資料（選 (a)）

```
im.presence_streams       -- UNLOGGED：presence 是可重建的短暫狀態，不需要 WAL
  stream_id   UUID PK                 -- UUIDv7，每條串流一列
  kind        TEXT NOT NULL CHECK IN ('BUYER','STORE')
  party_id    BIGINT NOT NULL CHECK > 0
  expires_at  TIMESTAMPTZ NOT NULL    -- 一律由資料庫 now() 計算與比較
  INDEX (kind, party_id, expires_at)
```

- 寫入：`Subscribe` 在 §4.3 的 advisory lock transaction 內先數、後寫一列（`expires_at = now() + PresenceTTL 60 秒`），**然後**才登記進 `Hub`、讀補洞（登記先於補洞的命題 2 不變）；`Run` 每 PresenceBeat（20 秒）對本進程所有串流做一個 `UPDATE … SET expires_at = now() + ttl WHERE stream_id = ANY(@ids)`；`Close` 刪該列。崩潰沒有 `Close`，由過期處理：讀取只認 `expires_at > now()`；週期 job（沿用 `PeriodicJobs`）清掉過期超過 10 分鐘的列。TTL＝3×延長間隔，容忍錯過兩次。
- 時鐘：以資料庫的 `now()` 為唯一的跨進程時間來源。presence 列由多個進程寫、多個進程讀，各進程時鐘的偏差會直接變成「在線」判定的誤差；單一來源讓這個誤差為零，不必假設 NTP。這與專案「測試注入 clock」的規則不衝突：該規則針對進程內邏輯，這裡的時間不經進程。測試不 sleep：要讓列過期，直接在測試中以 SQL 把該列的 `expires_at` 改到過去（`UPDATE … SET expires_at = now() - interval '1 second'`），檢查讀取與清理 job；TTL 與延長間隔是 `Deps` 的設定，延長批次由注入的 ticker 驅動。沿用盤點見 H25。
- 失敗語意分兩段：**延長批次**單次失敗只記 log；連續 2 次失敗（下一次就可能過期）時結束本進程所有訂閱（§4.3），client 依游標重連，重連時重新計數，上限不因延長失敗而放寬；聊天內容的送達優先（與舊碼「presence 與訊息送達分開」一致，`message.go:620-622`）。**開串流時**的寫入失敗讓 `Subscribe` 失敗，因為那一列同時是全域並行上限的計數（§4.3，fail-closed）。
- 為什麼 UNLOGGED：不需要在崩潰後存活，省 WAL；崩潰後整表清空＝所有人離線，等心跳重建，正是想要的語意。【推：PG UNLOGGED 的語意；Atlas 對 UNLOGGED 的支援沒有查，2b1 開工時確認，不行就退回一般表】
- 沒有最後上線時間：表裡只有「現在是否有未過期的列」，離線後列刪除或過期，沒有任何欄位保存最後一次的時間（A6-D5）。

### 7.3 讀取與授權

`Presence(ctx, p, conversationIDs)`（一次 ≤ 50 個）：

1. 每個對話以 `authorized` 檢查屬於呼叫者，否則整個呼叫回 NotFound（不洩漏；ID2 的根因是無授權，這裡的授權單位是「對話」，有對話才有資格）。
2. 對方的 (kind, id) 由對話列取得；對方在 `im.inactive_parties`（已刪帳或退場）→ 離線。
3. 任一方向有封鎖（`buyer.IsBlockingStore`、`seller.StoreBlocksBuyer`）→ 離線，與「對方真的離線」不可區分（W3）。查詢失敗回錯誤，不放行（ID5 的同一原則）。
4. 其餘：`EXISTS (SELECT 1 FROM im.presence_streams WHERE kind = $1 AND party_id = $2 AND expires_at > $now)`。
5. 回傳 `[]PresenceState{ConversationID, Online bool}`，**沒有時間欄位**。

- 授權單位是「共有對話」而不是「對方的 ID」：呼叫者無法用任意 ID 探測任意人的在線狀態。
- 需要的依賴：`read.Deps` 目前只有 `DB`（`read.go:25`），`Presence` 要 `Buyers`、`Stores`（與 `write.Deps` 同一組 port），§8。
- 單一對話的 presence 來自該對方 Party（kind＋id）：店家在線＝有一條以該 store_id 開的串流；同一個人買家那一條不影響（對應舊碼 per-role，`hub.go` 的 `PresenceKey`）。

## 8. 對 2a 程式的改動清單（2b1 的檔案表）

| 檔 | 改動 |
|---|---|
| `im/internal/live/`（新） | `Hub`、`Listener`（連線、`SELECT now()` 存活標記與重連）、`sub`、payload 編解碼、presence 寫入與延長批次；**不含** heartbeat、壽命、deadline（A6-5 的 helper） |
| `im/internal/write/send.go` | `Send` 的 transaction 內加 `pg_notify`（`m`） |
| `im/internal/write/write.go` | `Hide`、`Restore`（`v`，需要 conversation、buyer、store、seq：`LockMessage` 改成回傳這些）、`MarkRead`（`r`，只在游標前進時）、`MarkAllRead`（`u`） |
| `im/db/queries/conversations.sql` | `MarkReadBuyer`／`MarkReadStore` 回傳「是否前進」（以舊值 CTE 比對）、`MarkAllRead*` 回 `RETURNING` 以判斷是否有列變動；讀取加 `PeerReadSeq` |
| `im/db/queries/conversations.sql`（`AllocateSeq`） | upsert 寫 `last_message_at` 改用資料庫 `now()`，刪掉時間參數（§4.4、M27） |
| `im/db/queries/messages.sql` | `LockMessage` 回 `conversation_id, buyer_id, store_id, seq` |
| `im/db/queries/presence.sql`（新）、`schema.sql`、migration | `presence_streams`；`pg_notify` 呼叫 |
| `im/internal/read/read.go` | `Deps` 加 `Buyers`、`Stores`；`Presence`；`Conversation`／`Message` 帶 `PeerReadSeq`（W5） |
| `im/im.go` | `Subscribe`、`Run`、`Presence`、`Subscription`（含 `Revalidate`）、`Event`、`Ended` 等型別別名；`Deps` 新增 `ListenConnConfig` 與常數設定（`Buyers`、`Stores` 已在） |
| `im/internal/purge`（或 live） | `PeriodicJobs` 加 presence 過期清理 |
| `im/AGENTS.md`、`im/CARRYOVER.md` | 不變量（§9 的列）、「不負責」刪掉即時通道、LISTEN 連線的前提 |
| `internal/arch/modules.go` | 不改：im 依賴 `buyer`、`seller`、`ordering`、`media`，沒有 identity、`accountsuspension`（`Guard` 由 handler 提供） |
| `cmd/hawkr` | **2b 不接**；A6-5 與 handler 一起呼叫 `Run` |
| `step-2a-design.md`、計畫 | §12 的文字修正 |

## 9. 沿用盤點（從零設計會怎麼做）

| # | 舊機制（出處） | 從零設計會怎麼做 | 去向 |
|---|---|---|---|
| H1 | `MessageHub` 三張 map＋三個 channel＋單 goroutine `Run`（`hub.go:14-125`） | 進程內只需要「Party → 訂閱者」的表；跨進程由 PG 提供 | 重寫：`live.Hub`（只有 map＋RWMutex，沒有 register channel） |
| H2 | 推送在 commit 後、transaction 外，直接寫連線 channel（`message.go:625-638`） | 同 transaction 發通知，由資料庫保證 commit 後才送達；內容從資料庫讀 | 取代：§2、§4.2（ID4） |
| H3 | 每連線 256 格緩衝，滿了丟（`chat_stream.go:20`、`hub.go:176-181`） | 不緩衝內容；合併待處理訊號，超過上限要求 client 重取 | 取代：§4.3 |
| H4 | `Broadcast`、`BroadcastExcept`、`ConnectionCount`（`hub.go:137-140,205-234,194-203`） | 沒有呼叫端就不存在 | 刪除（計畫「不做」） |
| H5 | `SendToUser`、`SendToUserRole` 按 userID＋loginRole 找連線（`hub.go:143-184`） | 按 (kind, id) 找；kind 就是 role | 取代：`partyKey`（ID6：鍵含 kind） |
| H6 | 雙軌送達：WS 與推播同時送，client 靠 `seq_id` 去重（`message.go:620-622`） | 串流不是送達保證，推播另走；去重靠 seq 與訊息 ID | 保留原則：推播仍由 `imrelay`（A6-2a §6）；串流事件與推播互不依賴 |
| H7 | presence 在 hub 記憶體、`lastSeen`（`hub.go:47-104,236-329`） | 共享狀態＋過期；不保存最後上線時間 | 取代：§7（ID2、A6-D5）；W2 決定輪詢或推送 |
| H8 | `presence_sub`／`presence_unsub` 幀、任何人可訂任何人（`client.go:54-63,118-126`） | 以共有對話授權的 unary 讀取 | 取代：§7.3（ID2） |
| H9 | presence 的 `lastSeenAt`（`hub.go:316-329`） | 不給 | 刪除（A6-D5） |
| H10 | `typing`／`stop_typing` 與每秒 1 次的限流（`client.go:64-73,107-114`） | 不做（A6-D15） | 刪除；三端在 A6-6 刪 |
| H11 | `read_receipt` 轉送幀、不落庫（`client.go:74-81`） | 已讀游標落庫，對方的游標推導出「已讀」 | 取代：`PeerRead`（W5）；`MarkRead` 的 `r` |
| H12 | `ping`→`pong` 應用層（`client.go:82-`）；client 15 秒送、30 秒無 frame 判定半開（`ws.ts:70,73`） | 伺服器端在閒置時送 heartbeat，client 只判活；屬傳輸層 | 取代：A6-5 的串流傳輸 helper 的 heartbeat frame（§5.3）；常數保留 |
| H13 | `sync` 幀：最多 20 房間、每 actor 5 次速率限制（Redis）、無權限回空（`client.go:51,128-185`；`message.go:285-302`） | 開串流時帶游標；沒有「幀」，也就沒有幀的速率限制；速率限制放在入口（`rpc.SubjectFrom` 的限流身分） | 取代：`SubscribeInput.Cursors`（§4.5）；無權限靜默忽略保留 |
| H14 | `sync_done`（每房間，`client.go:170-185`） | 一個「補洞結束」標記 | 取代：`Ready`（§6.1） |
| H15 | `unread_changed` 推送（`message.go:1131-1155`） | 狀態改變後讀一次總數 | 保留語意：`Unread` 事件（§6.2） |
| H16 | `seller.suspended` 強制斷線（`seller_suspended.go`；`hub.go:331-348`） | 串流只讀，停權不需要斷線 | 刪除（W4）；結束原因改為 §5.2 |
| H17 | 連線身分來自 gateway 第一幀的 `user_id`、`login_role`，im-service 不驗（`chat_stream.go:34,124-137`） | 身分只從 session 來 | 取代：`Party`（ID6） |
| H18 | gateway 一次性 ticket、TTL 30 秒（`chat_ws_ticket.go:18`）與 `ws-ticket` 端點 | 串流請求自己帶 session 憑證（cookie 或 Bearer） | 刪除：A6-6e（端點）；A6-5 不新增 |
| H19 | 每帳號 10 條並行連線，進程內 map（`chat_ws_connlimit.go:9,21-28`） | 多進程下的上限要全域：以共享的資料庫列計數 | 取代：MaxStreamsPerParty＝10，數 `presence_streams`（§4.3） |
| H20 | gateway 寫入逾時 10 秒（`GW/handler/constants.go:25`） | 逐次寫入 deadline | 取代：A6-5 的 handler（ADR 第 64 行義務） |
| H21 | Redis：封鎖名單、收件人已刪標記、sync 限流（`redis.go:11-13`） | 單體沒有 Redis，封鎖與刪帳都是 in-process | 刪除：A6-2a 已處理；2b 沒有 Redis 依賴 |
| H22 | `profile_cache` 為推送幀帶發送者名稱與頭像（`IM/usecase/profile_cache.go`；`message.go:594-618`） | 事件只帶業務鍵，顯示資料由 httpapi 以 `BatchDisplay` 補 | 取代：`Event.Message` 不帶名稱頭像；A6-5 補 |
| H23 | 舊 WebSocket 的外層協議（Gorilla、JSON 幀、`chat_grpc.go` 的 gateway 橋接） | server streaming 的 proto | 刪除：A6-6e |
| H24 | 三端各一份 WS 狀態機（`client-inventory.md` §2.4） | 一個共用的串流 client 行為契約，常數寫在文件 | 取代：A6-6（本檔 §4.5、§5.3 是契約的伺服器端） |
| H25 | （新機制）presence 的過期時間用哪個時鐘 | 多進程共享的狀態用單一時間來源：資料庫 `now()` | 新設計：§7.2；測試以 SQL 改 `expires_at`，不注入時鐘 |
| H26 | （新機制）LISTEN 連線的重連與退避，手寫 | 業界有現成套件 `jackc/pgxlisten`（pgx 作者的 LISTEN 迴圈，含重連與處理器登記）；新依賴要 Will 同意（根 `AGENTS.md`「依賴與供應鏈」：列出套件與版本、核對發佈滿兩週）。本檔**不新增依賴**，手寫約 60 行：要的只是「一個頻道、固定處理器、重連後發 `epoch`」，而 `epoch` 需要 `lastOK` 與重連成功的掛點（§4.4），套件的處理器模型要再包一層才拿得到；重連的退避由 T15 單元測試證明 | 取代自寫以外的選項已記錄；若 Will 傾向採用套件，換掉 `Listener` 一個檔，不影響其餘（見 §15 #7） |

## 10. 消費端（2b 之後仍不動，直到 A6-6e）

只列查詢，數字是 2026-11-20 的 `rg` 命中行數【量】。2b 不改任何一個；它們都在 A6-6 或 A6-6e 才改或刪。

| 消費端 | 指令 | 命中 | 何時 |
|---|---|---|---|
| gateway 的 WebSocket 與 ticket | `rg -n "ChatWebSocket\|acquireWSSlot\|serveChatWebSocketGRPC\|ChatWSAuth\|ChatWSTicket" backend/gateway --type go \| rg -v _test`（行數） | 28 | A6-6e 刪路由與 `openapi.yaml` 的 `/app/chat/ws`、`/app/chat/ws-ticket` |
| 單體的 WebSocket 轉發 | `rg -n -i "websocket\|upgrade" backend/internal/legacy/gatewayproxy backend/internal/httpapi --type go` | 26 | `gatewayproxy` 的 WebSocket 轉發與 `httpapi/session.go` 的 `asksForUpgrade`（停權時升級視為寫入，A7-D18）：A6-6e 後若沒有別的 WebSocket 路徑，`asksForUpgrade` 與其測試一併檢視 |
| website | `rg -n "getChatSocket\|ws-ticket\|presence_sub\|unread_changed\|read_receipt\|typing\|chat_ack" website/lib website/components website/features --glob '!**/__tests__/**'` | 85 | A6-6a：`lib/chat/ws.ts` 換成串流 client；CSP 的 `withWebSocketSiblings`（`next.config.ts`、`lib/security/csp.ts:54`）在沒有任何 WS 後移除 |
| buyer-app | `rg -n "WebSocket.connect\|presence_sub\|unread_changed\|read_receipt\|typing" buyer-app/lib` | 41 | A6-6c |
| seller-app | `rg -n "WebSocket.connect\|presence_sub\|unread_changed\|read_receipt\|typing" seller-app/lib` | 31 | A6-6d |
| notification-service | `chat.message.sent`（`imrelay`，A6-2a §6） | — | A8；2b 不影響（推播與串流互不依賴，H6） |
| 舊 im-service | `backend/services/im-service/internal/{usecase/hub.go,grpc/chat_stream.go,worker/seller_suspended.go}` | — | A6-7 刪 |

留到 A6-6e 的：舊 hub／WebSocket、gateway 路由、`gatewayproxy` 的 WebSocket 轉發；`imrelay` 與 `Run` 的 `cmd/hawkr` 接線（`Run` 在 A6-5 就要接，因為 handler 在 A6-5 註冊；但 handler 在 A6-6e 之前不會有流量，因為 gateway 還在把 `/app/chat/*` 轉給舊服務）。

## 11. 測試

integration 測試用 `pg.NewDB`／`testdb.Server.NewURL`（CI 跑，本機不起容器；本檔沒有跑任何一個）。能脫離資料庫的邏輯（payload 編解碼、`sub` 的合併與游標、計時）用單元測試，時鐘與 ticker 注入。

| # | 測試 | 層 |
|---|---|---|
| T1 | `NOTIFY` 只在 commit 後送達：`Send` 的 transaction 回滾（以外層 `InTx` 回傳錯誤）→ 監聽端沒有收到；commit → 恰好一個，payload 欄位正確 | integration |
| T2 | 多進程：兩個 `im.Module`（各自 `Run`、各自專用連線）共用一個資料庫；A 送出、B 的訂閱者收到訊息 | integration |
| T3 | 監聽連線中斷：`pg_terminate_backend` 殺掉 listener 的 backend，期間在 `delivered` 內的對話送出 3 則；重連後訂閱者以 `epoch` 補齊，順序正確、不重複 | integration |
| T3b | 重連的 Resync 範圍：中斷期間，一個不在 `delivered` 的對話有新訊息、另一個沒有；重連後只收到前者的 `Resync{conversation}`，沒有 `Resync{all}`；十個訂閱者同時重連時總 `Resync{all}` 數為 0 | integration |
| T4 | 登記先於補洞：在補洞讀取前後各插入一則（測試縫：`live` 套件的 `afterRegister` 掛點），兩則都恰好收到一次 | integration |
| T5 | `Hide`／`Restore` 在外層 transaction 內：回滾不發；commit 後雙方的串流收到 `Message{Updated}`（墓碑→還原），seq 相同 | integration |
| T6 | 身分隔離：buyer_id＝store_id＝5，兩個 Party 的串流互相收不到對方的對話；`Cursors` 帶別人的對話 ID 被忽略且與無新訊息不可區分；偽造 `b`、`s` 的通知（直接 `pg_notify`）不會讓別人讀到內容（授權在讀取） | integration |
| T7 | 慢消費者：訂閱者停止 `Next`，同時送出 1000 則橫跨 600 個對話；`Send` 不因此變慢（逾時斷言）、`pending` 有界、恢復後收到全部或 `Resync{all}` | integration＋單元 |
| T8 | `Revalidate`：以關店的店家 `Party` 呼叫 → `Ended{LostEligibility}`；店家被停權 → **不結束**且仍收訊息；傳入新 `Party` 取代舊快照（之後以新 `Party` 授權） | integration（沒有時鐘、沒有 Guard；Guard 節奏、`GuardUnavailable`、session 失效是 A6-5 helper 的測試，見下表） |
| T9 | `Run` 的 ctx 結束 → 全部 `Shutdown`、presence 列已刪 | integration |
| T10 | 未讀：買家寄、店家串流收到 `Unread` 且等於 `UnreadTotal`；同一 Party 的第二個串流在 `MarkRead` 後收到 `Unread`；`MarkAllRead` 發一個 `u`；一輪多則只算一次總數 | integration |
| T11 | `PeerRead`：`MarkRead` 前進 → 對方串流收到；游標沒前進（重送）→ 沒有 `NOTIFY`；`MarkAllRead` 不產生 `PeerRead`（W5 選 (i) 時） | integration |
| T12 | presence：開串流→對方 `Presence` 在線；關→離線；崩潰（不呼叫 `Close`）→ TTL 後離線；心跳延長；非共有對話 → NotFound（ID2）；任一方向封鎖 → 離線（W3）；對方在 `inactive_parties` → 離線；`PresenceState` 型別沒有時間欄位（編譯期）；`Buyers` 或 `Stores` 查詢失敗 → 回錯誤 | integration |
| T13 | `Subscribe` 的授權：關店的店家 → 錯誤；超過 MaxStreamsPerParty → `ResourceExhausted`；兩個 `im.Module`（兩個進程）各開 6 條同一 Party 的串流，合計第 11 條被拒（全域）；十個並行 `Subscribe` 同一 Party 恰好成功 10 個（advisory lock）；`Close` 後名額釋出；`presence_streams` 寫入失敗 → `Subscribe` 失敗；`Cursors` 超過上限 → `InvalidInput` | integration |
| T14 | payload：最大欄位長度 < 8000；未知版本、非 JSON、缺欄位 → 丟棄並計數、連線不斷 | 單元 |
| T15 | 重連退避（0.5 秒起、上限 30 秒、含 jitter，注入時鐘與隨機源） | 單元 |
| T16 | listener 不佔連線池：`Run` 運行時 `Pool().Stat().AcquiredConns()` 不變 | integration |
| T17 | 鎖序：`accountdeletion_lockorder_integration_test.go` 既有的 im 情境加上 `NOTIFY` 之後仍通過（`pg_notify` 不鎖列）；`Hide` 在 moderation 的 transaction 內的鎖序照 A6-4 | integration |
| T18 | 連貫性：同一對話 50 則並行送出，串流收到的 seq 嚴格遞增且連續（命題 1） | integration |
| T19 | 合併不丟訊息：訂閱者對某對話沒有游標（不在 `delivered`），在第一次 `Next` 之前對該對話連送 2 則（`mark` 兩次、合併成一格），之後的 `Next` 依序交出兩則（seq 連續、各一次）；`v` 的兩個不同 seq 也各重讀一次 | 單元（`sub` 的 `mark`／`Next` 搭假 reader）＋integration |
| T20 | 時間只來自資料庫：sqlc 生成的 presence 查詢與 `AllocateSeq` 的參數型別沒有時間型別（反射或讀生成檔），`expires_at`、`last_message_at` 只由 SQL 的 `now()` 產生 | 單元 |

### A6-5 的串流傳輸 helper 承接的測試（不在 2b，列在這裡避免漏掉）

- 閒置 15 秒寫 heartbeat（注入 ticker）；超過 MaxLifetime 結束且有 jitter；逐次寫入 deadline 到期結束；每 RecheckEvery 呼叫 `Guard`、成功則 `Revalidate`；`Guard` 回 Unauthenticated 結束；連續 3 次暫時性錯誤 `GuardUnavailable`；ADR 第 64 行的不緩衝測試。對應的 mutation：不重驗（`Guard` 只在開頭呼叫）、壽命不 jitter、不寫 deadline。

### mutation（在複本跑，改回舊寫法或拿掉機制時對應的測試要失敗）

| # | 改壞 | 必須失敗的測試 |
|---|---|---|
| M1 | `Send` 不發 `NOTIFY` | T1、T2 |
| M2 | `NOTIFY` 改成 transaction 外、commit 後以連線池發（等於 hook 做法的對照） | T1（回滾案例會收到） |
| M3 | `Send` 在 `AllocateSeq` 之前就發 `NOTIFY`（payload 的 seq 還沒配發，用 `last_seq` 舊值）而後續步驟失敗回滾 | T1（回滾案例）、T18（seq 不符） |
| M4 | 補洞讀取改成先於登記 | T4 |
| M5 | `partyKey` 不含 kind | T6 |
| M6 | 訂閱讀取略過 `authorized`（以通知的 `b`、`s` 為授權） | T6（偽造通知） |
| M7 | `mark` 改成阻塞式寫 channel | T7 |
| M8 | `pending` 不設上限／超過不交 `Resync` | T7 |
| M9 | `epoch` 不對 `delivered` 補讀 | T3 |
| M10 | 重連成功後不發 `epoch` | T3 |
| M11 | `Revalidate` 不取代 `Party`（用舊快照） | T8 |
| M12 | 停權也結束串流 | T8（停權不結束） |
| M14 | `MarkRead` 不論是否前進都發 `r` | T11 |
| M15 | `MarkAllRead` 逐對話發 `r` | T10（一個 `u`） |
| M16 | `Presence` 略過共有對話檢查 | T12 |
| M17 | `Presence` 略過封鎖檢查／封鎖查詢失敗時放行 | T12 |
| M18 | presence 讀取不看 `expires_at` | T12（崩潰案例） |
| M19 | `Subscribe` 拿掉每 Party 上限；或改回進程內計數（不數 `presence_streams`）；或不取 advisory lock | T13（全域案例、並行案例） |
| M20 | listener 改從連線池借連線 | T16 |
| M21 | `Run` 結束不結束訂閱 | T9 |
| M22 | `Unread` 每則都算一次總數（不合併） | T10 |
| M23 | `v` 通知時不重讀（把舊內容再送一次） | T5 |
| M24 | `Hide`／`Restore` 不發 `v` | T5 |
| M25 | `mark` 把同一對話的 `q` 以 max 合併、`Next` 以 `maxQ - 1` 為 `base`（合併丟訊息） | T19 |
| M26 | `epoch` 對不在 `delivered` 的對話一律交 `Resync{all}`（不查 `last_message_at`） | T3b |
| M28 | `epoch` 不對 `delivered` 的對話交 `Resync{conversation}`（只補讀訊息） | 中斷期間隱藏與已讀的整合測試（`TestListenerReconnectResyncsDeliveredConversationsWithHiddenAndReadChanges`）、單元 `TestEpochReadsFromCursorsAndResyncsDeliveredAndActiveUnknownConversations` |
| M29 | `listenLoop` 不讀啟動時的資料庫時間（初始 `lastOK` 為零值、第一次連線不發 `epoch`） | `TestStreamOpenedBeforeTheFirstListenReceivesLaterCommits`、單元 `TestFirstConnectionEpochCarriesTheStartupDatabaseTime` |
| M30 | `ExtendPresenceStreams` 改回只 UPDATE（不補回被清掉的列） | `TestPresenceExtendReinsertsSweptRows` |
| M31 | `MarkRead` 的 `advanced` 改回與語句快照比較（拿掉 `FOR UPDATE` 的 CTE） | `TestConcurrentMarkReadNotifiesOnce` |
| M32 | `Subscribe` 把 `stopCtx` 的指派移到登記進 `h.subs` 之後、不持 `s.mu` | 單元 `TestSubscribeWithConcurrentlyCancelledContextHasNoRace`（`go test -race`） |
| M27 | presence 的 `expires_at` 或 `last_message_at` 以進程時鐘寫入／比較 | T20（靜態守門：sqlc 生成的 presence 與 `AllocateSeq` 查詢參數裡沒有時間型別參數，`expires_at`、`last_message_at` 只由 SQL 的 `now()` 產生） |

## 12. 本檔提議的計畫修改

1. **「單進程」殘留**：A6-D15 已改不變量表（計畫第 51 行），但「範圍」仍寫「不做跨進程扇出（單一進程，見不變量）」（計畫第 76 行）、「延後」仍列「im 跨進程扇出（`LISTEN/NOTIFY` 或 broker）」（第 81 行）。兩處與 D15 矛盾，建議刪除。
2. **A6-2 步驟文字（第 154 行）**：「停權、停業、session 失效時結束串流」改為「關店時由模組結束串流；session 失效、帳號刪除由串流傳輸 helper 結束；停權與封鎖不結束」（W4，Will 決定後才改）。
3. **A6-2a 設計書**：E7 與 §0「2b 的 hub 訂閱它」改為「2b 的即時通道在 `write` 內以 `NOTIFY` 發送，不訂 `MessageSent`」；`MessageSent` 的訂閱者只剩 relay。
4. **A6-5 的清單**：加上 `ChatService.Subscribe`、`GetPresence`（W2 選 (a) 時）、`MarkRead`／`MarkAllRead`；訊息種類 enum 外加事件種類（§6.1）；**可重用的串流傳輸 helper（heartbeat、MaxLifetime＋jitter、逐次寫入 deadline、Guard 重驗節奏與 `GuardUnavailable`，§5.2、§5.3；承接 ADR 第 64 行的義務與上節的 helper 測試）**；`cmd/hawkr` 讀專用的 LISTEN 連線設定（§3.1）；串流的驗收含「經 web host 反代與 Cloudflare 不被緩衝、閒置不被切」。
5. **A6-6**：三端刪除打字中、`ws-ticket`、`presence_sub`；三端的狀態機換成共同契約（§4.5、§5.3）。
6. **任務與盤點裡的「舊 Redis presence」**：改成「舊 hub 的記憶體 presence」（§0.4）。

## 13. 要 Will 決定的題目（已決：全照建議，A6-D16，Will 2026-11-20）

### W1 NOTIFY payload 帶什麼（A6-D15 的字面是「對話 ID 與 seq」）

- 前提題：D15 的目的是「通知只是提示，內容走資料庫」，這一點本檔完全遵守；只是 payload 是否還能多帶**路由用的業務鍵**。
- 約束（真的）：每個進程收到全部對話的通知（單一頻道，§2.1）。
- 選項：
  - (i) 只帶對話 ID＋seq＋種類：每個進程對每則通知查一次資料庫取 buyer_id、store_id，才知道本機有沒有訂閱者；成本隨 進程數 × 全站訊息量 增長。
  - (ii) 加帶 `b`、`s`：進程內以 map 過濾，沒有本機訂閱者的通知不碰資料庫；payload 約 100 bytes；內容仍沒有訊息文字或個資（外部業務鍵本來就在每個 URL 與列表裡）。
  - (iii) 每個 Party 一個頻道（`im_b_123`、`im_s_456`）：payload 只帶 seq；代價是連線要動態 `LISTEN`／`UNLISTEN` 並在重連時重建，頻道名稱上限 63 bytes。
- 業界：以 PG 做 pub/sub 的系統通常把「路由鍵」放在 payload、內容放在資料表；每用戶一頻道的做法在連線數大時被視為不佳【推：憑記憶，沒有查證】。
- 建議 (ii)：目的（內容不經通知）沒有被放寬，只多帶兩個路由鍵；(i) 的成本是真的，且隨規模上升。

### W2 presence 用輪詢（unary）還是在串流上推送

- 前提題：A6-D5 已決定顯示在線；沒決定怎麼送。串流是單向的（server streaming），client 沒有辦法告訴伺服器「我正在看哪個對話」；舊碼靠 client 發 `presence_sub` 幀（`client.go:54-58`）。
- 選項：(a) `GetPresence` unary，client 在對話畫面開著時每 30～60 秒對目前對話輪詢；(b) 伺服器對該 Party 所有對話的對方，在上下線時於串流上推 `Presence` 事件，另需要掃描者處理崩潰過期的離線（§7.1）；(c) 保留進程記憶體（單進程）：與 D15 的多進程矛盾，不列為可行。
- 業界：多數聊天產品的在線點最多容忍數十秒延遲；推送的即時性是錦上添花，其複雜度（反向訂閱通道、掃描者）不是。
- 建議 (a)：沒有訂閱狀態、授權逐次判定、可限流；之後需要時可以在同一個 `NOTIFY` 頻道加一個種類，不需要重做表。
- 本檔的 §7、T12、M16–M18 以 (a) 寫成；選 (b) 時 §7.1 的 (b) 欄是增量。

### W3 封鎖時，被封鎖的一方還看得到對方在線嗎

- 事實：A6-D5 只說「只對有對話的對方顯示」，沒說封鎖；A6-D1 讓有進行中訂單時封鎖不阻擋聊天，所以「有對話」不等於「沒有封鎖」。
- 選項：(i) 任一方向有封鎖就回離線，與真的離線不可區分；(ii) 不看封鎖，有對話就給。
- 業界：WhatsApp、Messenger 等把封鎖者對被封鎖者隱藏在線狀態。
- 建議 (i)：封鎖的意圖是讓對方探知不到自己；在線不是履約必要資訊（與送出的例外不同）；成本是 `Presence` 多兩次 in-process 讀取（`IsBlockingStore`、`StoreBlocksBuyer`）。

### W4 串流結束的條件：停權不結束；撤銷的偵測延遲最多 60 秒

- 前提題：計畫 A6-2 寫「停權、停業、session 失效時結束串流」。舊碼的強制斷線（`seller_suspended.go`）是為了不讓長連線續傳訊息，但連線只收（§0.4）；現在送出是 unary，由 G1 每則擋。
- 事實：停權的店家與帳號仍可讀取（`CapChatRead` 不要求未停權；A7-D18），所以對停權者結束串流，等於收回他們本來就有的讀取。關店才失去 im 的讀取資格。session 撤銷、帳號刪除在 identity，im 不依賴；偵測由 helper 以 `Guard` 輪詢（預設 60 秒，由 helper 決定），不新增事件。
- 本題只問**業務條件**：哪些 im 看得到的狀態要讓串流結束。心跳、壽命、重驗節奏是傳輸層，歸 A6-5 的 helper（§5.2），不在這裡決定。
- 選項：(i) 本檔 §5.2：停權、封鎖不結束；只有關店（失去 `CapChatRead`）讓模組結束串流（session 失效、帳號刪除另由 helper 的重驗結束）；(ii) 照計畫字面：停權也結束；(iii) (i) 加讓 identity、accountsuspension 在撤銷時 `NOTIFY`（跨模組改動）以縮短偵測延遲。
- 業界：連線型服務的授權撤銷通常是「短壽命憑證＋定期重驗」，立即撤銷要付額外的撤銷通道；只讀串流容忍秒到分鐘級的延遲。
- 建議 (i)：與 A7-D18 一致，不新增跨模組通道；已撤銷 session 的最壞後果是多收 ≤60 秒自己的訊息（串流只讀，不能做任何事）。這會改計畫第 154 行的文字。

### W5 已讀指示（對方看到我「已讀」）要不要保留，以及批次已讀要不要推給對方

- 事實：舊 client 會顯示已讀（buyer-app `chat_thread_screen.dart` 等；website `lib/chat/ws.ts` 處理 `read_receipt`／`chat_ack`）。舊碼的 `read_receipt` 只是 client 發的轉送幀，不落庫（`client.go:74-81`）；2a 的資料模型有對方的游標，但 2a 的讀取沒有給對方游標。
- 選項：(i) 保留：2b 加 `PeerReadSeq` 到讀取，`MarkRead` 前進時發 `r`、對方串流交出 `PeerRead`；`MarkAllRead` 只發 `u`（更新自己的其他裝置），**不**推給每個對方；(ii) 保留且 `MarkAllRead` 也逐對話推（一個 transaction 內無上限個 `NOTIFY`，不建議）；(iii) 不保留已讀指示（產品改動，三端 UI 要拿掉，需要先取得同意）。
- 建議 (i)：產品行為不變；批次已讀的對方在下次補洞或打開對話時才看到，因為對這種動作要求即時是不必要的、而 (ii) 的成本沒有上限。

## 14. 沒有驗證的項目（誠實列出）

- `NOTIFY`／`LISTEN` 的語意（commit 後才送達、rollback 不發、同 tx 相同 payload 合併、payload 8000 bytes、斷線期間的通知不補送、PgBouncer 的 transaction pooling 不相容）：憑對 PostgreSQL 的了解寫成，本次沒有查文件、沒有跑；T1、T3、T14 在 CI 會量到前幾項。LISTEN 連線必須直連或 session 模式是明確設定的不變量（§3.1），不在執行期偵測，CI 沒有 pooler 所以測不到違反情形；部署時以 T2 同形的 smoke 確認（我不知道這個專案的正式環境用什麼連線方式）。
- `NOTIFY` 在高寫入率下 commit 全域鎖的吞吐上限：憑記憶，沒有量，也沒有聊天寫入率的數字。觸發條件：送出的 p95 延遲或 commit 等待時間在量測中因 `NOTIFY` 升高；替代：outbox 表＋輪詢（與 `imrelay` 同形）。
- 經 web host 反向代理與 Cloudflare 的串流行為（緩衝、閒置切斷、總時長上限）：這個 repo 沒有這些設定，本檔的 15 秒心跳與 15 分鐘壽命是估計值，驗收放在 A6-5／A6-6a。
- Atlas 對 `UNLOGGED` 表的支援：沒查，2b1 開工時確認，不行就用一般表（對正確性沒有影響）。
- 同一對話的 seq 與 commit 次序一致（§4.2 命題 1）：由列鎖語意推出，T18 以併發測試佐證，不是證明。
- 各常數（PresenceTTL 60 秒、PresenceBeat 20 秒、MaxStreamsPerParty 10、MaxCursors 50、MaxBacklog 200、MaxPending 500、存活標記 10 秒）：估計值，不是量測值，放成 `Deps` 可調的設定；HeartbeatEvery 15 秒、RecheckEvery 60 秒、MaxLifetime 15 分鐘同為估計值，歸 A6-5 的 helper 設定。
- 重連風暴的規模：`epoch` 已只 Resync 有變動的對話（§4.4），但每個訂閱者重連時仍各做 `delivered` 對話數次 `MessagesAfter` 與一次索引掃描，沒有量；觸發條件：每進程並行串流數量測超過數千、或重連後資料庫讀取尖峰在量測中可見時，改為對 `epoch` 讀取分批與 jitter。
- `Guard` 每 60 秒的成本（每串流一次 `Authenticate`＋店家讀取）：估計。
- website 其他 client 的 `unread_changed` 之外還有哪些依賴 WebSocket 事件（盤點的 85／41／31 行命中是 `rg` 行數，不是呼叫點數）：A6-6 逐端確認。

## 15. 審查處理對照

開工 design-review（2026-11-20）。審查全文：session scratchpad 的 `a6-2b-design-review.md`。

| # | 發現 | 嚴重度 | 處置 |
|---|---|---|---|
| 1 | 對話不在 `delivered` 時以合併後最大 `q-1` 為讀取起點，同一對話在第一次 `Next` 前的多則訊息只交出最新一則，較舊的被丟給 client 補洞 | should | 改：`pending` 保留 `minQ`／`maxQ`，不在 `delivered` 時 `base = minQ-1`；`v` 保留各自 seq（§4.1、§4.2、§4.3）；加 T19 與 M25（max 合併 → T19 失敗） |
| 2 | heartbeat、MaxLifetime＋jitter、逐次寫入 deadline、Guard 重驗節奏屬傳輸層（ADR 第 64 行），卻放進模組 | should | 改：移到 A6-5 的可重用串流傳輸 helper；模組只交出業務事件與 `Shutdown`／`LostEligibility`，新增 `Subscription.Revalidate`、刪 `SubscribeInput.Guard` 與 `Heartbeat` 事件（§0.1、§1、§5.2、§5.3、§6.1、§8、H12、§11 T8／T9／helper 測試表、§12 第 2、4 項）；W4 改成只問業務條件（§13） |
| 3 | 以 `SHOW`／LISTEN 往返偵測 PgBouncer，無法在 CI 證明 | should | 改：刪偵測與 T16 的第二半；LISTEN 用專用 `Deps.ListenConnConfig`，不變量寫在 `im/AGENTS.md`（§3.1、§11 T16、§14） |
| 4 | MaxStreamsPerParty＝10 是每進程，多進程下失去意義 | should | 改：`Subscribe` 在 advisory lock 內數 `presence_streams`，全域執行，fail-closed（§4.3、§7.2、H19）；T13 加全域與並行案例，M19 擴充 |
| 5 | 監聽重連對每個訂閱者發 `Resync{all}`，造成 client 重連風暴 | should | 改：`im.conversations` 有 `last_message_at`（`schema.sql:22`）與索引（`schema.sql:31-32`），`epoch` 只對中斷期間有訊息、且不在 `delivered` 的對話發 `Resync{conversation}`，`lastOK` 取自資料庫（§3.1、§4.4）；T3b、M26；剩餘的讀取量記入 §14 並寫觸發條件 |
| 6 | presence 過期用進程注入的 `Now`，多進程時鐘偏差進入在線判定 | nit | 改：一律資料庫 `now()`，測試以 SQL 改 `expires_at`（§7.2、H25）；M27 改為 T20 的靜態守門（第 2 輪） |
| 7 | 手寫 LISTEN 重連與退避，沒有說明為何不用 `jackc/pgxlisten` | nit | 記：H26 寫明考慮過、新依賴需 Will 同意（根 `AGENTS.md`）、本檔不新增依賴及理由；Will 若要採用只換 `Listener` 一個檔 |

第 2 輪（只審修正處）：

| # | 發現 | 嚴重度 | 處理 |
|---|---|---|---|
| 8 | §4.3 扇出端仍寫 `pending[c] = max(…)`，與 §4.2 的 minQ／maxQ 矛盾 | should | 改：改成 minQ／maxQ（§4.3） |
| 9 | `epoch` 以進程時鐘寫的 `last_message_at` 比資料庫時間，靠未量的 5 秒 skew | nit | 改：`AllocateSeq` 改寫資料庫 `now()`，刪 skew（§4.4、§8、M27） |
| 10 | 延長批次失敗時全域上限實際 fail-open | nit | 改：連續 2 次失敗結束本進程所有訂閱（§4.3、§7.2） |
| 11 | M27 可能沒有測試會失敗 | nit | 改：T20 靜態守門 |

findings 11 (blocking 0)：fixed 10、recorded 1、raised 0、rejected 0。

## 16. 實作與本設計的差異（8526c9b8 與第一次修正輪）

讓設計書與程式一致；每一項都是已經在程式裡的決定。

1. `internal/arch/modules.go` 的 `InternalDependencies` 登記 `live`（依賴 `model`、`store`、`text`）與 `write` 對 `live` 的依賴（§8 沒有列）。
2. `MarkAllRead` 的 SQL 是 `:execrows`，回傳被改動的對話數，`n == 0` 不發 `u`（§6.2 的「有對話被改動才發」）。
3. 新增 uitext 句子：`ImTooManyCursors`、`ImStreamLimit`、`ImPresenceTooMany`（模組的 `internal/text`，zh-TW 與 en）。
4. `Subscription.Revalidate` 回傳 `error`：失去資格時回 `*Ended{LostEligibility}`，Party 不是這條訂閱的身分回一般錯誤（§5.2 只寫「結束」）。
5. `AllocateSeq` 回傳它寫的 `last_message_at`（`clock_timestamp()`，與舊值取大者），`Send` 用同一個值當 `messages.created_at` 與 `MessageSent.OccurredAt`；`Deps.Now` 不再用於送出的訊息時間（Hide、Restore 與媒體同步仍用）。
6. `MarkRead` 的 `advanced` 以 `FOR UPDATE` 的 CTE 鎖住並讀出更新前的值再比較，不再讀語句快照。
7. `Streams` 介面：`Extend` 收 `[]Stream`（id、kind、party）並以 upsert 補回被清掉的列；新增 `DatabaseNow`（`Run` 啟動時讀一次，§3.1）與 `DBStreams.Sweep`。存活標記清理（`im.presence_sweep`）從 `purge` 搬到 `live`，有自己的 `PresenceSweepInterval`。upsert 的代價：訂閱剛被 `Close` 刪列、同時延長批次正在寫入時，列可能被補回並存活到過期（最長 `PresenceTTL`）。
8. 串流並行上限的 advisory lock 鍵是 `im.streams:<kind>:<id>`，與送出的 `im.party:` 鎖不同命名空間。
9. `epoch` 對 `delivered` 的對話交 `Resync{conversation}`（§4.4）；第一次連線也發 `epoch`（§3.1）。
10. 授權在訂閱讀取路徑的單一呼叫點是 `internal/read/read.go` 的 `MessagesAfter` 裡的 `s.authorized(...)`（M6 的改壞點）。
