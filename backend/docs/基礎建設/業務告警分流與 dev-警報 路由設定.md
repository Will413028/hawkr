# 業務告警分流與 dev-警報 路由設定

> 對應 issue #898（建立路由通道）+ #1353（第二個使用者）。
> 業務正確性告警與 infra 系統告警分流到不同 Discord 頻道，降低告警疲勞、受眾分離。

## 頻道劃分

| 頻道 | 用途 | Alertmanager 判定 |
|---|---|---|
| `monitor-alert`（既有） | infra 系統告警（ServiceDown / 磁碟 / CPU / RabbitMQ 無 consumer 等） | 預設 receiver `discord`（無 `channel` label） |
| `dev-警報` | dev 環境**業務正確性**告警 | `channel: business` + `env: dev` |
| `prod-警報` | prod 環境**業務正確性**告警 | `channel: business` + `env: prod` |

目前業務告警使用者：ATM 退款卡單（#1353）、假日資料覆蓋（#898）。

## 資料流

```
metric (obs 層 sql_exporter 等，帶 env 標籤)
  └─ Prometheus rule（business-alerts.yml，labels: channel=business）
       └─ Alertmanager route（match channel=business + env）
            └─ discord-webhook-gateway（依路徑選 webhook）
                 └─ Discord 頻道（dev-警報 / prod-警報）
```

**關鍵原則**：告警只宣告 `channel: business`；**env 由 Prometheus scrape target 的 label 提供**（見 `prometheus/targets/*.json`），rule 不硬編 env。dev/prod 由同一條 rule 依 metric 的 env 自動分流。

## 組成元件（皆在 `monitoring/`）

### 1. discord-webhook-gateway（多頻道）
`monitoring/discord-webhook-gateway/main.go`：依 HTTP 路徑對應不同 webhook（env 注入，密鑰不入版控）：

| 路徑 | 頻道 | env 變數 |
|---|---|---|
| `/alert` | monitor-alert（infra，必填） | `DISCORD_WEBHOOK_URL` |
| `/alert/dev-alert` | dev-警報 | `DISCORD_WEBHOOK_URL_DEV_ALERT` |
| `/alert/prod-alert` | prod-警報 | `DISCORD_WEBHOOK_URL_PROD_ALERT` |

未設某 env 變數 → 該路徑不註冊（Alertmanager 指向時回 404，部署時可察覺）。

### 2. Alertmanager 路由
`monitoring/alertmanager/config.yml`：`channel: business` + `env` 的 route 須排在 env-only route **之前**（first-match wins）→ 對應 `discord-dev-alert` / `discord-prod-alert` receiver（指向上表路徑）。

### 3. 告警規則
`monitoring/prometheus/rules/business-alerts.yml`：業務 rule 一律加 `labels: { channel: business }`。

## 環境變數同步（依[開發工作流程 §3](../開發規範/開發工作流程.md)）

business 頻道 webhook 為**密鑰**，只放 gitignored `monitoring/.env`（範本見 `monitoring/.env.example`）：
```
DISCORD_WEBHOOK_URL_DEV_ALERT=<dev-警報 webhook>
DISCORD_WEBHOOK_URL_PROD_ALERT=<prod-警報 webhook>
```
部署：於 monitoring 主機（203.0.113.10）`monitoring/.env` 設定後 `docker compose up -d discord-webhook-gateway alertmanager`。**絕不寫進任何進版控檔**。

## 新增一個業務告警（SOP）

1. **產出 metric**（obs 層，[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)：偵測不寫 application code）：如 co-located `sql_exporter` 加 collector，或既有服務已有的業務 metric。
2. **加 rule** 到 `business-alerts.yml`，`labels: { channel: business, severity: ... }`。
3. **完成**——路由基建已就緒，dev/prod 自動分流（前提：該 metric 的 scrape target 帶正確 env label）。

> 責任邊界：偵測歸 observability（本路由 + metric producer），資料/狀態的修復歸維運或既有冪等寫入路徑，不引入 application reconciler。
