# IM 即時通訊：投遞策略與連線架構

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-08-30 |
| 影響範圍 | `services/im-service`、`gateway/internal/handler/chat.go`、`api/proto/im`、buyer／seller／web 三端 client |
| 對應規格 | `docs/詳細設計書/14-即時通訊微服務（IM Service）詳細設計書.md` |

> 本檔保存 `WHY THIS DESIGN` / `WHY NOT THE ALTERNATIVES`。
> frame 白名單、房間命名、停權 gating 判準、失敗處置表一律在 14 號詳細設計書，本檔不複製。

---

## 脈絡

即時通訊要在三種 client（web 瀏覽器、buyer App、seller App）之間傳訊，
而三者的連線可靠性差異極大：瀏覽器分頁會被凍結、行動網路會經過 carrier NAT、
App 會進背景。任何「server 判斷對方在不在線」的設計都會在這三種情境下各錯一次。

---

## ADR-01 投遞策略：雙送 ＋ 前端去重

**不採用**：① 用本地 map 判在線 ② Redis presence ③ WS 送失敗才補 FCM。

**決策**：每則訊息**同時**走 WS（best-effort）與 MQ event（→ FCM），
前端以 `(roomId, seqId)` 去重。

server 無法 100% 確定訊息送達（WS write 成功不代表對方收到）。
放棄「判斷在不在線」這件事之後，投遞變成零遺漏、零判斷，而且天然多實例友善。

## ADR-02 WS 連線範圍：聊天室限定，不維持全域連線

**不採用**：登入即保持全域 WS 連線。

FCM 在 1–3 秒內就能觸發 in-app banner，不值得為了「新訊息提示」而讓每個登入使用者
各掛一條長連線。連線數因此大幅下降。

## ADR-03 連線終止層：Gateway 終止 ＋ Gateway↔IM gRPC stream

**不採用**：edge 直送 im-service（terminate-once）。

Gateway 保留 public WebSocket，往 IM 走 typed bidirectional gRPC。
這讓「client transport 變更」與「IM server cutover」不必綁在同一次上線。
terminate-once 的前置條件是 edge passthrough ＋ traefik→ClusterIP，規模到了再評估。

## ADR-04 跨 replica fan-out：目前 in-memory，不預先做 backplane

**不採用**：一開始就上 Redis pub/sub 或 MQ backplane。

規模未到。**觸發條件是明確的：要開第 2 個 replica 的那一刻**，
必須先有 Redis pub/sub backplane ＋ presence-in-Redis ＋ sticky／一致性 LB。
在那之前開第 2 replica 會靜默漏訊息。

## ADR-05 WS 連線票：單次 Redis ticket

**不採用**：長命 JWT 放在 URL query。

單次、30 秒、URL-only 的 ticket 安全性明顯較好：URL 外洩的價值窗口只有 30 秒。
接受的代價是 **Redis 成為 ws-ticket 的 SPOF**（fail-closed），
未來若要收斂，方向是無狀態 HMAC 短票降級。

## ADR-06 heartbeat 間隔：量測後設定，不 hardcode

**不採用**：直接寫死一個數字。

要扛 carrier NAT（約 30 秒）就得取其 1/3～1/2，而 Cloudflare／Zeabur edge 對 `/ws`
的 idle timeout 未量測。在量到之前，值由 WS-level ping/pong 承擔。

## ADR-07 build vs buy：自建 Go im-service

**不採用**：socket.io／managed 服務／一開始就用 Centrifugo。

買賣家雙身份的 domain 模型已經建好且 debug 過，自建的遷移風險最低。
Centrifugo 列為**文件化的升級路徑**——當自維 scaling／presence 開始吃掉工程時間時走它。

## ADR-08 reconnect 退避：Full Jitter

**不採用**：固定指數退避。

固定指數會讓單 replica 重啟時所有 client lockstep 同步重連，
連線風暴之外還有一次鑄票風暴。Full Jitter 把它去相關化。

## ADR-09 service 暴露：ClusterIP ＋ 純 gRPC

**不採用**：NodePort ＋ 獨立 management HTTP port。

IM 只暴露 9933 gRPC，health 與 business RPC 共用 listener，metrics 走 OTLP push。
不需要 NodePort 讓 Prometheus scrape，也就不需要一個可以從 query 帶 identity 的 HTTP 面。

---

## 後果

- 雙送策略讓前端**必須**實作 `seqId` 去重；去重寫錯的症狀是訊息重複而非遺漏。
- 單 replica 是現行不變式，不是暫時狀態：擴容前要先補 backplane（ADR-04）。
- Redis 抖動會讓 web 端全部連不上聊天（ADR-05 接受的代價）。
