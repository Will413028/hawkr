# Discord 告警通知設定指南

## 概覽

監控系統透過以下管道將告警發送到 Discord：

```
Prometheus（觸發告警規則）
    → Alertmanager（路由分組）
        → discord-webhook-gateway（格式轉換）
            → Discord Webhook（發送到頻道）
```

告警觸發時，Discord 頻道會收到帶有顏色標記的嵌入式訊息：
- 🔥 **紅色** — 告警觸發（FIRING）
- ✅ **綠色** — 告警恢復（RESOLVED）

---

## 第一部分：建立 Discord Server

> 如果已有 Server，跳到[第二部分](#第二部分建立告警專用頻道)。

### 1. 建立帳號

1. 前往 https://discord.com/register 註冊帳號（或下載 Discord 桌面/手機 App）
2. 完成 Email 驗證

### 2. 建立 Server

1. 打開 Discord，點擊左側最下方的 **「+」** 圖示
2. 選擇 **「自行建立」**（Create My Own）
3. 選擇 **「僅供我和我的朋友使用」**（For me and my friends）
4. 輸入 Server 名稱，例如：`Hawkr 監控`
5. 點擊 **「建立」**

### 3. 邀請團隊成員

1. 右鍵點擊 Server 名稱 → **「邀請其他人」**
2. 複製邀請連結，傳給需要收到告警的團隊成員
3. 可設定連結有效期限（建議 7 天或永不過期）

---

## 第二部分：建立告警專用頻道

### 1. 建立頻道

1. 在 Server 左側頻道列表，點擊類別旁的 **「+」**
2. 選擇 **「文字」**（Text Channel）
3. 頻道名稱輸入：`monitoring-alerts`（建議用英文，好辨識）
4. 點擊 **「建立頻道」**

### 2. （選用）建立頻道分類

建議將監控相關頻道整理到一個分類下：

1. 右鍵點擊 Server 名稱 → **「建立類別」**
2. 類別名稱：`🔔 監控`
3. 將 `monitoring-alerts` 頻道拖曳到此類別下

### 3. （選用）設定頻道權限

如果不希望所有人都能在告警頻道發言：

1. 右鍵點擊 `monitoring-alerts` → **「編輯頻道」**
2. 點擊左側 **「權限」**
3. 將 `@everyone` 的 **「傳送訊息」** 設為 ❌
4. Webhook Bot 的訊息不受此限制，告警仍會正常發送

---

## 第三部分：建立 Discord Webhook

### 1. 進入頻道設定

1. 右鍵點擊 `monitoring-alerts` 頻道 → **「編輯頻道」**
2. 左側選單點擊 **「整合」**（Integrations）

### 2. 建立 Webhook

1. 點擊 **「建立 Webhook」**（Create Webhook）
2. 修改 Webhook 名稱為：`Hawkr Alertmanager`
3. 確認頻道為 `monitoring-alerts`
4. （選用）上傳 Bot 頭像
5. 點擊 **「複製 Webhook URL」**

Webhook URL 格式如下：

```
https://discord.com/api/webhooks/1234567890123456789/abcdefghijklmnopqrstuvwxyz...
```

> **重要：** Webhook URL 等同於發送權限，不要洩漏或提交到版控。

### 3. 儲存 Webhook URL

記下這個 URL，下一步要填入 `.env`。

---

## 第四部分：設定監控系統

### 1. 設定環境變數

```bash
cd backend-payuni/monitoring
```

編輯 `.env`，填入剛才複製的 Webhook URL：

```env
DISCORD_WEBHOOK_URL=https://discord.com/api/webhooks/1234567890/your_token_here
```

> 如果還沒有 `.env`，先從範本建立：`cp .env.example .env`

### 2. 啟動（或重啟）監控服務

```bash
# 首次啟動
make up

# 已在運行，僅重啟 alertmanager + discord-webhook-gateway
docker compose up -d --build discord-webhook-gateway
docker compose restart alertmanager
```

`--build` 會重新建置 `discord-webhook-gateway` 的 Docker image。

### 3. 確認服務狀態

```bash
docker compose ps
```

確認 `hawkr-alertmanager` 和 `hawkr-discord-webhook-gateway` 都是 `Up` 狀態。

查看 gateway 日誌確認啟動正常：

```bash
docker compose logs discord-webhook-gateway
```

應看到：

```
Discord webhook gateway listening on :8080
```

---

## 第五部分：測試告警

### 方法一：透過 Alertmanager API 手動觸發

```bash
curl -X POST http://localhost:9093/api/v2/alerts \
  -H 'Content-Type: application/json' \
  -d '[
    {
      "labels": {
        "alertname": "TestAlert",
        "severity": "warning",
        "env": "dev"
      },
      "annotations": {
        "summary": "這是一則測試告警，確認 Discord 通知正常運作"
      }
    }
  ]'
```

幾秒後 Discord `monitoring-alerts` 頻道應收到紅色嵌入式訊息。

### 方法二：直接測試 discord-webhook-gateway

```bash
curl -X POST http://localhost:8080/alert \
  -H 'Content-Type: application/json' \
  -d '{
    "status": "firing",
    "alerts": [
      {
        "status": "firing",
        "labels": {
          "alertname": "TestAlert",
          "severity": "critical",
          "env": "dev"
        },
        "annotations": {
          "summary": "直接測試 Discord Webhook Gateway"
        }
      }
    ]
  }'
```

> 注意：`discord-webhook-gateway` 預設不對外暴露 port，如果從 VM 外測試需走方法一。

### 方法三：等待真實告警

確認至少有一個 Prometheus target 為 `DOWN`（Prometheus UI → Status → Targets），`ServiceDown` 規則會在 1 分鐘後觸發告警。

---

## 告警路由規則說明

目前 Alertmanager 的路由設定（`alertmanager/config.yml`）：

| 環境 | 行為 | 重複間隔 |
|------|------|---------|
| `env: dev` | 發送到 Discord | 2 小時 |
| `env: prod` | 發送到 Discord | 30 分鐘 |
| 其他（無 `env` 或未匹配） | 發送到 Discord | 4 小時（預設） |

**預設 receiver 為 Discord**，所有告警都會發送通知。

### 告警分組

- `group_by: ["alertname", "env"]` — 同名稱 + 同環境的告警合併為一則通知
- `group_wait: 30s` — 收到第一則告警後等 30 秒，收集同組告警一次發送
- `group_interval: 5m` — 同一組有新告警時，至少間隔 5 分鐘再發一次

---

## Discord 訊息格式

告警訊息以 Discord Embed 呈現：

**觸發（FIRING）：**
```
🔥 FIRING — 2 alert(s)
├── [DEV] ServiceDown
│   Severity: critical
│   gateway 服務無法連線
└── [DEV] HighHttpErrorRate
    Severity: warning
    gateway HTTP 5xx 錯誤率超過 5%
```

**恢復（RESOLVED）：**
```
✅ RESOLVED — 1 alert(s)
└── [DEV] ServiceDown
    Severity: critical
    gateway 服務無法連線
```

---

## 疑難排解

### Discord 沒收到訊息

1. **確認 Webhook URL 正確**
   ```bash
   # 直接用 curl 測試 Webhook 是否有效
   curl -X POST "你的WEBHOOK_URL" \
     -H 'Content-Type: application/json' \
     -d '{"content": "Hello from Hawkr!"}'
   ```
   如果 Discord 收到 `Hello from Hawkr!`，表示 URL 正確。

2. **確認 gateway 有收到請求**
   ```bash
   docker compose logs -f discord-webhook-gateway
   ```
   觸發測試告警後，應該看到日誌輸出。如果沒有，問題在 Alertmanager → gateway 這段。

3. **確認 Alertmanager 路由匹配**
   ```bash
   # 檢查目前活躍的告警
   curl http://localhost:9093/api/v2/alerts | python3 -m json.tool
   ```
   確認告警有 `env` label（`dev` 或 `prod`），否則會被 blackhole 吃掉。

4. **確認 Prometheus 有送告警到 Alertmanager**
   - 打開 Prometheus UI：http://localhost:9091/alerts
   - 確認有告警規則處於 `FIRING` 狀態

### discord-webhook-gateway 啟動失敗

```
請在 .env 設定 DISCORD_WEBHOOK_URL
```

`.env` 缺少 `DISCORD_WEBHOOK_URL` 或值為空。填入後重啟：

```bash
docker compose up -d discord-webhook-gateway
```

### Discord 回傳 429 Too Many Requests

Discord Webhook 有 rate limit（約 30 次/分鐘）。如果告警過多會被限流。解法：

- 調大 Alertmanager 的 `group_interval` 和 `repeat_interval`
- 確認沒有 flapping（告警反覆觸發/恢復）

---

## 附錄：新增其他 Discord 頻道（按環境分流）

如果日後想把 dev 和 prod 告警分到不同頻道：

### 1. 在 Discord 建立兩個頻道 + Webhook

- `#alerts-dev` → Webhook URL A
- `#alerts-prod` → Webhook URL B

### 2. 修改 docker-compose.yml

```yaml
discord-webhook-gateway:
  environment:
    - DISCORD_WEBHOOK_URL=${DISCORD_WEBHOOK_URL_DEV}  # dev
    
discord-webhook-gateway-prod:
  build: ./discord-webhook-gateway
  environment:
    - DISCORD_WEBHOOK_URL=${DISCORD_WEBHOOK_URL_PROD}  # prod
```

### 3. 修改 alertmanager/config.yml

```yaml
receivers:
  - name: "discord-dev"
    webhook_configs:
      - url: "http://discord-webhook-gateway:8080/alert"
        send_resolved: true
  - name: "discord-prod"
    webhook_configs:
      - url: "http://discord-webhook-gateway-prod:8080/alert"
        send_resolved: true

route:
  routes:
    - match:
        env: dev
      receiver: "discord-dev"
    - match:
        env: prod
      receiver: "discord-prod"
```

目前還不需要這樣做，等告警量增加再考慮。
