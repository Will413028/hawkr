# Discord 監控 Bot 設計書

> 本文件為可落地實作的規格。後續實作請嚴格依本書執行，任何偏離須先回頭更新本書。

---

## 1. 目的與範圍

提供開發者（可以指定用戶）透過 Discord Slash Command 快速查詢 Hawkr 後端系統狀態與執行**低風險維運操作**的工具。

### 1.1 In Scope

- 服務健康狀態總覽與單服務查詢
- 服務 log / 錯誤摘要查詢
- RabbitMQ queue 列表、狀態、單一 queue 詳情、consumer 狀況
- Redis 快取清除（呼叫既有 API）
- 單一 RabbitMQ queue 訊息清空（purge）

### 1.2 Out of Scope（明確不做）

| 不做項目 | 原因 |
|---|---|
| 服務重啟 / redeploy | 風險高，去 k3s/CI 操作更可控 |
| 任意 SQL 查詢 | 攻擊面過大 |
| 自由文字自然語言問答 | 解析失敗率高，slash command 已足夠 |
| Metrics 圖表 | 已有 Grafana，避免重複造輪子 |
| Wildcard / all 的 MQ purge | 手滑風險高 |
| 跨環境操作（一隻 bot 同時管 dev 與 prod） | 必須一個環境一隻 bot |

---

## 2. 架構

```
┌─────────────┐    Slash Command    ┌──────────────────┐
│  Discord    │ ──────────────────▶ │  Discord Bot     │
│  (你的私人  │ ◀────────────────── │  (Node.js or Go) │
│   server)   │       Embed         │                  │
└─────────────┘                     └────────┬─────────┘
                                             │
                ┌────────────────────────────┼─────────────────────────┐
                │                            │                         │
                ▼                            ▼                         ▼
        ┌──────────────┐           ┌──────────────────┐       ┌────────────────┐
        │  k3s API     │           │ RabbitMQ         │       │  Gateway       │
        │  (kubectl)   │           │ Management API   │       │  (cache flush) │
        │  pods/logs   │           │ /api/queues      │       │                │
        └──────────────┘           └──────────────────┘       └────────────────┘
```

### 2.1 部署形態

- **執行位置**：Dev VM（203.0.113.20）以 systemd service 或 docker container 常駐
- **一個環境一隻 bot**：dev / prod 各自獨立 bot token、獨立 channel；嚴禁共用
- **重啟策略**：systemd `Restart=always` 或 docker `restart: unless-stopped`

### 2.2 技術選型建議

| 項目 | 選擇 | 理由 |
|---|---|---|
| 語言 | **Node.js + discord.js v14** | Slash command + autocomplete + interaction button 支援最完整、文件最齊 |
| RabbitMQ 查詢 | HTTP Management API（`http://rabbitmq:15672/api/*`） | 不必引 amqp client |
| k3s 查詢 | `kubectl` CLI subprocess 或 k8s client library | CLI 最簡單；若要長期維護建議用 client library |
| 設定檔 | `.env` | 與專案其他服務一致 |

---

## 3. 安全模型

| 規則 | 實作 |
|---|---|
| 只回應允許清單內的使用者 | 檢查 `ALLOWED_USER_IDS`（逗號分隔的 Discord user ID 清單）是否包含 `interaction.user.id`，不在清單者直接回 `❌ 無權限`。可進一步以 `WRITE_USER_IDS` 細分「能執行寫入指令」的子集，未設定時等同 `ALLOWED_USER_IDS` |
| 只在指定 channel 運作 | 檢查 `interaction.channelId` 是否在 `ALLOWED_CHANNEL_IDS`（逗號分隔）內 |
| 寫入指令需 confirm | 用 Discord Button interaction（30 秒 timeout） |
| `/mq-purge` 必填 queue 名 | 不接受 wildcard、空字串、`all`、`*` |
| Audit log | 所有寫入指令寫入本地 `audit.log`：`時間 / user / 指令 / 參數 / 結果` |
| Token 管理 | Discord token、k3s kubeconfig、RabbitMQ 帳密一律走環境變數，不寫死 |
| 環境標示 | bot 啟動時在 channel 發 `🟢 Bot online (env=dev)`，每則回應 footer 帶 `env=dev` |
| 敏感資訊遮罩 | log / error 回傳前以 regex 遮罩 `Bearer xxx`、email、手機、信用卡號 |

---

## 4. 指令完整規格

### 4.1 `/help`

- **參數**：無
- **行為**：回傳所有指令清單與範例（從程式內常數產生，與實際註冊指令保持同步）
- **回應**：Embed，依「查詢類 / 寫入類」分組

---

### 4.2 `/status`

- **參數**：無
- **行為**：列出所有受監控的服務 pod 健康狀態
- **資料來源**：`kubectl get pods -n hawkr -o json`
- **回應格式**：

  ```
  ✅ gateway              v1.2.3   Running   2d
  ✅ product-service      v0.8.1   Running   2d
  ❌ order-service        v0.8.1   CrashLoopBackOff   3m
  ✅ payment-service      v0.5.0   Running   6h
  ...
  Footer: env=dev | 11 services | 1 unhealthy
  ```

- **異常條件**：phase ≠ Running 或 restart count 5 分鐘內 >0

---

### 4.3 `/service`

- **參數**：
  - `name` (string, required, autocomplete) — 服務名稱
- **autocomplete 來源**：硬編碼服務清單（共 9 個：gateway, product, order, cart, payment, shipping, coupon, media, aftersales, payuni-webhook）
- **行為**：顯示單一服務詳情
- **回應內容**：
  - Pod 名稱與狀態
  - Image tag
  - Restart count
  - CPU / Memory usage（`kubectl top pod`）
  - 最近一次重啟時間
  - Readiness / Liveness probe 結果

---

### 4.4 `/logs`

- **參數**：
  - `name` (string, required, autocomplete) — 服務名
  - `lines` (integer, optional, default=50, max=500) — 行數
  - `level` (choice, optional) — `ERROR` / `WARN` / `INFO`，預設不過濾
- **行為**：`kubectl logs --tail={lines} -n hawkr deploy/{name}`，必要時 grep level
- **回應**：
  - ≤ 1900 字：code block 直接貼
  - \> 1900 字：以 `.txt` attachment 上傳
- **遮罩**：套用 §3 敏感資訊遮罩

---

### 4.5 `/errors`

- **參數**：
  - `name` (string, optional, autocomplete) — 限定服務，未填則全部
  - `minutes` (integer, optional, default=15, max=120) — 時間窗
- **行為**：對指定服務的 log 撈取 `ERROR` 級別，做 stack trace 第一行的 group by 計數
- **回應**：

  ```
  過去 15 分鐘錯誤摘要（env=dev）
  order-service    NullPointerException at OrderService.java:142    × 12   最後 2min
  payment-service  PayUniTimeoutException                            × 3    最後 8min
  (其他服務無錯誤)
  ```

- **無錯誤**：回 `✅ 過去 15 分鐘無 ERROR`

---

### 4.6 `/mq-list`

- **參數**：
  - `filter` (string, optional) — queue 名稱模糊比對（substring，case-insensitive）
- **行為**：呼叫 RabbitMQ Management API `GET /api/queues`，依 filter 過濾
- **回應**：

  ```
  order.shipping.label    msgs:1240   consumers:0  ⚠️
  order.payment.notify    msgs:0      consumers:2
  order.cancel            msgs:0      consumers:2
  order.aftersales        msgs:3      consumers:1
  ...
  Total: 12 queues
  ```

- **排序**：訊息數由多到少
- **超過 30 條**：截斷並提示「使用 filter 縮小範圍」

---

### 4.7 `/mq-status`

- **參數**：無
- **行為**：列出**異常**的 queue（messages > 100 或 consumers = 0 但有 publisher）
- **回應**：只列異常項；全部正常時回 `✅ 全部 queue 正常`

---

### 4.8 `/mq-queue`

- **參數**：
  - `queue` (string, required, **autocomplete from RabbitMQ API**) — queue 名
- **autocomplete 實作**：每次 autocomplete 請求即時呼叫 `/api/queues`，依使用者輸入做 substring filter，回最多 25 筆（Discord 限制）
- **行為**：呼叫 `GET /api/queues/{vhost}/{name}`
- **回應**：

  ```
  queue: order.shipping.label
  state: running
  messages: 1240 (ready: 1240, unacked: 0)
  consumers: 0  ⚠️
  publish rate: 5.2 msg/s
  deliver rate: 0 msg/s
  memory: 24 MB
  policy: ha-all
  ```

---

### 4.9 `/mq-consumers`

- **參數**：
  - `queue` (string, optional, autocomplete) — 限定 queue
- **行為**：呼叫 `GET /api/consumers`
- **回應**：每行一個 consumer：`queue / consumer_tag / channel host (pod IP) / prefetch / ack_required`

---

### 4.10 `/cache-flush`（寫入）

- **參數**：
  - `scope` (string, optional, default=`all`) — 預留欄位，目前實作只接受 `all`
- **行為**：
  1. Bot 回覆 confirm Embed + `[確定] [取消]` button，30 秒 timeout
  2. 確定後呼叫 `GET {GATEWAY_URL}/api/v1/public/caches/flush`
  3. 回傳 API response
- **Audit log**：必寫
- **Gateway URL**：取自 env `GATEWAY_URL`，dev 為 `https://test-api.example.com`

---

### 4.11 `/mq-purge`（寫入）

- **參數**：
  - `queue` (string, required, autocomplete) — 必填，禁止空字串、`*`、`all`
- **行為**：
  1. 先 `GET /api/queues/{vhost}/{name}` 取當前訊息數
  2. 回 confirm Embed：`⚠️ 即將清除 queue {name} 的 {N} 筆訊息，無法復原`
  3. 確定後呼叫 `DELETE /api/queues/{vhost}/{name}/contents`
  4. 回傳結果
- **Audit log**：必寫，包含 purge 前訊息數

---

## 5. 資料來源細節

### 5.1 k3s

- **連線方式**：bot 容器掛載 `~/.kube/config` 或使用 in-cluster ServiceAccount
- **必要權限**：`pods: get/list`、`pods/log: get`、`pods/exec: 不需要`、`metrics.k8s.io: get`（給 `/service` 的 CPU/Mem）
- **建議**：建立專用 ServiceAccount `discord-bot-readonly`，僅授予以上權限

### 5.2 RabbitMQ Management API

- **Base URL**：`http://rabbitmq:15672/api`（bot 與 rabbitmq 同網段）
- **vhost**：預設 `/`，URL encode 為 `%2F`
- **帳號**：建立專用 user `discord-bot`，權限：`monitoring` tag + 對應 vhost 的 configure/write/read（write 僅給 purge 用）

### 5.3 Gateway

- 僅用於 `/cache-flush`，呼叫公開 API，無需驗證

---

## 6. 環境變數

依[開發工作流程 §3 環境變數配置流程](../開發規範/開發工作流程.md)，**必須**同步以下位置：

| Key | 範例 | 說明 |
|---|---|---|
| `DISCORD_BOT_TOKEN` | `MTIzNDU2...` | Discord bot token（secret） |
| `DISCORD_CLIENT_ID` | `1234567890` | Application ID |
| `DISCORD_GUILD_ID` | `1234567890` | 你的 server ID（用於註冊 guild command，立即生效） |
| `ALLOWED_USER_IDS` | `1234567890,2345678901` | 允許執行**任何**指令的 Discord user ID 清單（逗號分隔） |
| `WRITE_USER_IDS` | `1234567890` | 允許執行**寫入類**指令（`/cache-flush`、`/mq-purge`）的子集；未設定時等同 `ALLOWED_USER_IDS` |
| `ALLOWED_CHANNEL_IDS` | `1234567890,2345678901` | 允許運作的 channel ID 清單（逗號分隔） |
| `BOT_ENV` | `dev` | 環境標示，顯示於每則回應 footer |
| `K8S_NAMESPACE` | `hawkr` | k3s namespace |
| `KUBECONFIG` | `/root/.kube/config` | kubeconfig 路徑 |
| `RABBITMQ_API_URL` | `http://rabbitmq:15672` | RabbitMQ Management API base |
| `RABBITMQ_VHOST` | `/` | vhost |
| `RABBITMQ_USER` | `discord-bot` | 專用唯讀（+ purge）帳號 |
| `RABBITMQ_PASSWORD` | `***` | secret |
| `GATEWAY_URL` | `https://test-api.example.com` | for `/cache-flush` |
| `AUDIT_LOG_PATH` | `/var/log/discord-bot/audit.log` | 寫入指令稽核 |

**部署檢查清單**：

- [ ] `.env`、`.env.dev`、`.env.template` 加入以上 key（無敏感值）
- [ ] `docker-compose-local.yml` 注入到 `discord-bot` 服務 environment
- [ ] `k8s/values/discord-bot.yaml` 建立，敏感值走 `secretRef`
- [ ] SSH 到 Dev VM 部署並驗證
- [ ] 驗證 bot 上線並回應 `/help`

---

## 7. 專案結構

新增獨立 repo 或放在 `tools/discord-bot/`（建議後者，與主專案同 mono repo）：

```
tools/discord-bot/
├── package.json
├── .env.template
├── Dockerfile
├── README.md
├── src/
│   ├── index.js              # 啟動點，註冊指令
│   ├── config.js             # env 載入
│   ├── audit.js              # audit log
│   ├── mask.js               # 敏感資訊遮罩
│   ├── commands/
│   │   ├── help.js
│   │   ├── status.js
│   │   ├── service.js
│   │   ├── logs.js
│   │   ├── errors.js
│   │   ├── mq-list.js
│   │   ├── mq-status.js
│   │   ├── mq-queue.js
│   │   ├── mq-consumers.js
│   │   ├── cache-flush.js
│   │   └── mq-purge.js
│   ├── clients/
│   │   ├── k8s.js            # kubectl wrapper
│   │   ├── rabbitmq.js       # Management API client
│   │   └── gateway.js        # cache flush
│   └── utils/
│       ├── confirm.js        # confirm button helper
│       └── format.js         # embed formatter
└── k8s/
    └── deployment.yaml
```

---

## 8. 實作里程碑

| 階段 | 內容 | 驗收 |
|---|---|---|
| M1 | 專案骨架、env 載入、bot 上線、`/help` | bot 在 channel 顯示 online，輸入 `/help` 有回應 |
| M2 | 唯讀指令：`/status`、`/service`、`/logs`、`/errors` | 每個指令對 dev 環境真實呼叫成功 |
| M3 | MQ 唯讀指令：`/mq-list`、`/mq-status`、`/mq-queue`、`/mq-consumers`，含 autocomplete | autocomplete 能即時拉到 queue 名 |
| M4 | 寫入指令：`/cache-flush`、`/mq-purge`，含 confirm button + audit log | 清快取/清 queue 成功，audit.log 有紀錄 |
| M5 | 容器化、k3s 部署、systemd 或 deployment yaml | bot 在 dev VM 常駐，重啟自動恢復 |

---

## 9. 風險與限制

| 風險 | 緩解 |
|---|---|
| Discord token 外洩 | 走 secretRef，不進 git；外洩立即 regenerate |
| `/mq-purge` 誤觸 | 必填 queue + confirm button + audit log 三層 |
| Bot 自己掛掉沒人通知 | 加入 `/healthz` HTTP endpoint，讓既有 monitoring 監看 |
| autocomplete 對 RabbitMQ API 壓力 | autocomplete 結果快取 5 秒 |
| log 含敏感資訊 | §3 遮罩規則必須在 unit test 覆蓋 |

---

## 10. 後續擴充想法（**不在本期實作**）

- `/grafana {dashboard}` 回 Grafana snapshot 連結
- `/deploy-status` 顯示最近一次 CI/CD 部署結果
- 主動告警（既有 `Discord 告警通知設定指南.md` 已負責，bot 不重複）
