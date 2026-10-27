# Realtime 推播平台：SSE、內嵌 gateway、不引入 Redis

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-18 |
| 影響範圍 | gateway（`internal/sse`／`internal/realtime`／`internal/mq`）、hawkr-website、buyer／seller App |
| 規格 | [41 號 Realtime SSE 推播平台詳細設計書](../詳細設計書/41-Realtime%20SSE%20推播平台（Realtime%20SSE%20Push%20Platform）詳細設計書.md) |

## 脈絡

平台需要「server → client 單向即時推送」：留言、通知、賣家工作區訂單狀態。
當時既有的三種機制各有既定用途——Chat 已用 WebSocket、行動端已接 FCM、web 通知走 30 秒 polling。
問題是要新增第四種、還是沿用其中一種；以及若新增，要放在哪裡執行。

## 決策一：用 SSE，不共用 WebSocket、不留 polling

| 機制 | 適用場景 | 為何不用於本平台 |
|---|---|---|
| **SSE**（採用） | 留言、通知、低庫存提醒、訂單狀態跳動 — 凡 server → client 單向推送 | — |
| WebSocket（Chat 已用） | 雙向訊息（聊天、typing、presence） | per-user 訂閱模型、ticket auth 模型，對「per-post 匿名訂閱」場景過重；流量耦合風險 |
| Polling | 低頻 / 容忍延遲 | 延遲高、伺服器負載大、UX 差 |
| FCM | App 背景 / 殺死狀態原生推播 | 不適用 web；不適用「App 前景看貼文」即時場景 |

**SSE 與 WebSocket / FCM 互補非取代**：Chat 維持 WebSocket、行動端推播維持 FCM；
SSE 補上「web 與 mobile 前景的 server→client 單向推送」，且所有此類需求集中走同一個平台，
避免基礎設施膨脹。

### 為何不保留 polling 作為兜底

| 「保留 polling」常見理由 | 認真檢視 |
|---|---|
| SSE 斷線時 polling 兜底 | `EventSource` 內建自動重連 ＋ `Last-Event-ID` replay 已 cover 網路抖動；真的整段網路斷，polling 也救不了 |
| Ingress / proxy buffer SSE 造成延遲 | 那是 server 配置問題，應修 `X-Accel-Buffering: no` ＋ ingress `proxy_buffering off`，不是用 polling 繞 |
| Pod 重啟 buffer 失效 | 已設計 `bufferMiss=true` 通知 client 做**一次性** fetch refresh（不是持續 polling） |
| 完全離線 | 離線時 polling 也拿不到資料；連回網路 SSE 自動恢復 |

## 決策二：行動端 notification 不開 SSE

| 維度 | Web | Mobile（buyer / seller） |
|---|---|---|
| 既有推播能力 | 無 FCM 訂閱、只有 30s polling | 已接 FCM；visible 顯示 banner／系統通知中心、silent 觸發前景 refresh |
| 背景送達 | tab 在背景使用者看不到 | OS 通知中心保證可見 |
| 前景送達 | polling 30s 延遲 | FCM data-only message，App 主動 refresh |
| 引入 SSE 的邊際效益 | 高（從無到有） | 低（與 FCM 重複；額外消耗連線與電量） |
| 跨裝置 read 同步 | 需要（與 mobile 同步） | 既有 FCM data refresh 已涵蓋 |

行動端已透過 FCM 雙通道達成「背景 banner ＋ 前景 refresh」，重複加 SSE 屬於過度設計。

**重新評估的條件**：FCM 出現可靠性問題，或出現「跨裝置 mark-read 秒級同步行動端」的需求。

## 決策三：內嵌 gateway，不開獨立 sse-service

| 評估面 | gateway 內嵌（採用） | 獨立 sse-service |
|---|---|---|
| 架構複雜度 | 低（只多 3 個 package） | +1 個 pod、+1 個 k8s values、+1 個 deployment、ingress 規則新增 |
| Auth 模型 | 沿用 gateway middleware（cookie／JWT 已就緒） | sse-service 端再實作一次，或走 gateway proxy（多一跳） |
| Scale 模型 | gateway 本就 horizontal scale，SSE 長連線跟著 pod 數線性擴展 | 同樣 horizontal scale，沒有額外好處 |
| Scale 上限 | 單 Go pod 保守估 5000 conn，10 pod = 50k conn | 同 |
| Pod 升級影響 | gateway 升級 → SSE 斷線（EventSource 自動重連，client 透明） | 同 |
| 觀測 | 既有 gateway 監控加 SSE 指標即可 | 新監控板 |

獨立 sse-service 是 premature scaling。Channel Registry／SSE Hub／MQ Consumer 三個 package
的抽象已留好出路，未來真要獨立，整包搬到新服務 ＋ ingress 加路由即可。

### 為何不需要 Redis

| Redis 用途 | 是否需要 |
|---|---|
| 跨 pod presence | ❌ 每條連線綁定一個 pod 直到斷線，不需知道它在哪個 pod |
| Connection roaming（連線中途換 pod） | ❌ SSE 斷了重連到新 pod，client 透明 |
| 跨 pod replay buffer | ❌ 已由「per-pod ring buffer ＋ bufferMiss client refresh」涵蓋 |
| Pub/sub broadcast 多 pod | ❌ RabbitMQ broadcast queue 已是此功能 |

唯一可能引入 Redis 的時機是「全平台同時 N 人在看某貼文」這類跨 pod presence 顯示。

## 後果

接受的代價：gateway 升級會中斷所有 SSE 連線（靠 client 自動重連吸收）；
ring buffer 只在單一 pod 內，pod 重啟後由 `bufferMiss` 觸發一次性全刷。

**重新評估獨立 sse-service 的觸發條件**：

| 指標 | 閾值 | 動作 |
|---|---|---|
| gateway pod 平均 active SSE conn | > 3000 持續一週 | 評估獨立 sse-service |
| gateway HTTP req/s 與 SSE conn 數的 scale 比 | 明顯不一致（HTTP 平穩但 SSE 暴漲） | 評估獨立（scale 維度不同） |
| gateway 升級造成 SSE 斷線的使用者反饋 | 反覆出現 | 評估獨立 |
| 單 pod active conn 接近 5000 cap 的頻率 | 每週 > 1 次告警 | 先提高 cap 或 scale pod，再評估獨立 |
