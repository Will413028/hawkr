# 聊天即時通道：Connect server streaming 加 Postgres LISTEN/NOTIFY

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-11-19 至 2026-11-21（Will 決定） |
| 影響範圍 | 單體 `im` 模組、`ChatService`（Connect）、gateway 的 `/app/chat/*` 與 WebSocket、website、buyer-app、seller-app |
| 取代 | `2026-09-18-im-websocket-delivery-architecture.md` |
| 執行計畫 | `backend/docs/plans/2026-11-19-a6-review-im-crm.md`（決定 A6-D3、D5、D14 W6、D15、D16、D17、D23 W1、W3、W4） |

## 脈絡

舊 ADR 為「獨立 im-service 加 gateway 終止 WebSocket」設計：雙送、Redis ticket、單 replica 的記憶體 fan-out。im 搬進單體後，這個前提變了：進程由單體部署、身分是 session、不再有 gateway↔IM 的 gRPC 橋。

- 外部：Postgres 的 `NOTIFY` 在 commit 後才送達、rollback 不發、斷線期間的通知不補送；connect-web 在瀏覽器不支援雙向串流（`2026-09-26-external-contract-protobuf-connect.md` D7）。
- 繼承：系統從未上線，沒有相容性約束（根目錄 `AGENTS.md`）；舊事件 `chat.message.sent` 在 A8 之前形狀不變，因為 notification-service 還在舊系統，仍成立。
- 繼承：單體的 `go.mod` 沒有 redis；多一個元件要有必要條件，聊天寫入率沒有量測數字支持它。
- 繼承：WebSocket 從來不是送達保證，推播才是（舊碼註解 `message.go:620-622`）；串流只讀，送出是 unary，仍成立。

## 舊 ADR 各項的去向

| 舊項 | 去向 |
|---|---|
| ADR-01 雙送加 `(roomId, seqId)` 去重 | 取代：串流與斷線補洞走同一條讀取路徑（D2），以 seq 游標讀取，沒有「WS 一份、事件一份」的兩路內容；FCM 仍由 `chat.message.sent` 經 `imrelay` 給 notification |
| ADR-02 連線限聊天室、不維持全域連線 | 取代：串流以身分（買家或店家）為單位，一條串流收該身分所有對話的訊息、未讀與已讀（D1、D7）；讀取以對話授權，不靠過濾 |
| ADR-03 Gateway 終止 WebSocket、gateway↔IM gRPC stream | 不再適用：im 在單體內，Connect 由單體直接服務；gateway 的聊天路由與 WebSocket 轉發在 A6-6e 刪除 |
| ADR-04 in-memory fan-out，第 2 replica 前先補 backplane | 取代：D2 的 LISTEN/NOTIFY 讓多進程成立，不再有單進程不變量 |
| ADR-05 單次 Redis ticket | 不再適用：串流是一般 HTTP 請求，帶 session（web cookie、App Bearer，`2026-10-30-identity-self-hosted-federated-login.md`），沒有 URL 憑證，也就沒有 Redis SPOF |
| ADR-06 heartbeat 量測後設定 | 沿用精神：心跳、壽命、寫入期限歸 A6-5 的串流 helper；15 秒與 15 分鐘是估計值，實網量測在 A6-5／A6-6a 驗收 |
| ADR-07 自建而非 socket.io／Centrifugo | 沿用：模組在單體內自建；Centrifugo 升級路徑本次沒有重新評估 |
| ADR-08 reconnect Full Jitter | 本次未決定 client 退避；伺服器端壽命加 jitter（helper）。client 的退避由 A6-6 的 streaming client 實作時依此原則 |
| ADR-09 ClusterIP 加純 gRPC | 不再適用：im-service 退場，沒有獨立 service 暴露 |

## 決定

| # | 決定 | 否決的方案與理由 |
|---|---|---|
| D1 | server streaming 收、unary 送，不留 WebSocket；斷線以 `seq` 游標補洞（A6-D3） | 基準：ADR 09-26 D7 的預設即此。WebSocket 列為第二套契約：聊天沒有必須雙向的幀，送出本來就 unary；打字中在 D2 一併不做 |
| D2 | 即時推送走 Postgres `NOTIFY`，在送出的同一 transaction 發，commit 後才送達；串流收到後以 `MessagesAfter` 讀 DB；不做打字中；三端刪打字中（A6-D15）。基準：通知只是提示、內容在資料表，PG pub/sub 系統通常如此（`step-2b-design.md` W1，【推】憑記憶） | 單進程記憶體 hub：把進程數固定為 1（缺陷 ID4）。訂閱 `MessageSent` 或在 `platform/events` 加 commit 後掛點：註冊延到 A6-6e 會讓推送靜默失效，`Hide`、`Restore`、`MarkRead` 沒有事件可訂，`NOTIFY` 也不是業務事件。Redis／broker backplane：單體沒有 redis，沒有量測支持這個元件 |
| D3 | `NOTIFY` payload 加帶 `buyer_id`、`store_id` 路由鍵，內容不經通知（A6-D16 W1） | 只帶對話 ID 與 seq：每個進程對每則通知查一次 DB，成本隨進程數乘全站訊息量。每個身分一個頻道：動態 `LISTEN`／`UNLISTEN`、重連重建、頻道名上限 63 bytes |
| D4 | presence 只對有對話的對方顯示、不給最後上線時間（A6-D5）；以 unary 輪詢（W2）；任一方向有封鎖，對被封鎖方回離線（W3）。基準：WhatsApp、Messenger 對被封鎖方隱藏在線（`step-2b-design.md` W3） | 不做：三端聊天頁都用 presence。照舊加授權並給最後上線：洩漏行為時間。串流推送：要反向訂閱通道與掃描崩潰過期的離線，在線點容忍數十秒延遲，複雜度不值得 |
| D5 | 停權、封鎖不結束串流；只有關店由模組結束；session 失效與帳號刪除由 helper 定期重驗（預設 60 秒）結束（A6-D16 W4）。基準：短壽命憑證加定期重驗（`step-2b-design.md` W4） | 停權也結束：收回停權者本來就有的讀取（A7-D18）。撤銷時 `NOTIFY`：新增跨模組通道只為縮短只讀串流的延遲 |
| D6 | 保留已讀指示；`MarkAllRead` 只通知自己的其他裝置，不逐對話推給對方（A6-D16 W5） | 逐對話推：一個 transaction 內無上限個 `NOTIFY`。拿掉已讀：產品改動，三端 UI 要改 |
| D7 | 寫入 transaction 設時間上限，重連補洞起點往回放同一常數加時鐘誤差（A6-D17）。只因它決定投遞正確性：開始早於斷線、提交晚於斷線的訊息，`last_message_at` 可能小於存活時間，補洞會漏掉它 | 不設上限：補洞界線無法證明，漏訊息 |
| D8 | 卡片訊息延後：只出 `TEXT`、`IMAGE`，卡片之後只存伺服器擁有的參照，觸發條件是 ordering 或 catalog 進單體（A6-D14 W6、A6-D23 W3）。基準：商品或訂單卡片是參照，由平台依參照即時組出（Shopee，`step-2a-design.md` W6，【推】依公開產品行為） | 照舊存 client 帶的自由 JSON：預覽與內容繞過詞典。遞迴掃所有字串：伺服器替不認識的 JSON 背詞典責任。現在建 legacy 讀取 client：寫一份 B8 就刪的整合 |
| D9 | 聊天依集合分買家版與店家版方法，店家由呼叫者此刻的店推導，請求不帶 `store_id`（A6-D23 W1 (g)）；重開條件：一帳號可管多家店或店家有員工 | 選填 `store_id`（沒帶即買家）或必填 parent：與 `2026-10-07-connect-without-perspective.md`「client 不選身分」字面衝突，要修訂該 ADR。兩種身分合併列表：要改模組 `Subscribe`（一次一個身分） |
| D10 | `SendMessageRequest.request_id` 必填 UUIDv4，`UNIQUE (conversation_id, sender_kind, request_id)`，重送回傳既有訊息（A6-D23 W4）。基準：Slack `client_msg_id`、AIP-155 的 request ID | 不做、client 不自動重送：行動網路重送是常態，重複送出是使用者看得到的缺陷 |

## 後果

- 每個進程收到全部對話的通知，成本是一次 JSON 解析與兩次 map 查詢；授權在讀取，不在通知過濾。
- 接受：`NOTIFY` 的語意與高寫入率下的 commit 全域鎖是憑記憶，沒有量；LISTEN 連線必須直連或 session 模式，不在執行期偵測。重新評估：送出的 p95 延遲因 `NOTIFY` 升高時，改 outbox 表加輪詢（與 `imrelay` 同形）。
- 接受：已撤銷 session 最多多收約 60 秒自己的訊息；停權者仍可讀。
- 接受：A6-6 換新契約後，client 暫時不能從聊天送商品或訂單卡片，也沒有打字中。
- 重新評估：反代或 Cloudflare 的緩衝與閒置切斷與估計值不符；一帳號可管多家店（D9）。
