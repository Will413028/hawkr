# 系統監控架構 (Monitoring Architecture) 詳細設計書

## 1. 架構總覽

### 1.1 VM 拓樸

所有環境（Prod / Dev）統一使用 k3s 部署微服務，透過 Kubernetes ExternalService 連線到 Infra VM 上的基礎設施（含監控棧）。

```
App VM × N（K3s）                          Monitoring VM（Docker Compose）
┌──────────────────────────────┐           ┌────────────────────────────────┐
│  gateway / order-svc / ...   │           │                                │
│  （21 microservices）         │           │  Prometheus（central）          │
│         │                    │           │    ← remote_write (app metrics)│
│    OTLP push (4317/4318)     │           │    ← file_sd_configs (infra)   │
│         │                    │           │                                │
│  ┌──────▼──────────────────┐ │           │  OTel Collector                │
│  │ K8s ExternalService     │ │           │    ← OTLP (traces/logs)       │
│  │ otel-collector → Infra  │ │           │    → Tempo (traces)           │
│  │ prometheus    → Infra   │ │           │    → Prometheus (metrics)     │
│  │ postgresql    → Infra   │ │           │    → Loki (logs)             │
│  │ redis / rabbitmq / loki │ │           │                                │
│  └─────────────────────────┘ │           │  Grafana (3000)               │
│                              │           │  Alertmanager (9093)          │
│  ┌──────────────────────┐    │           │  Loki (3100) · Tempo (3200)   │
│  │ Prometheus Agent     │────┼─ remote ─→│                                │
│  │ kubernetes_sd_configs│    │   write   └───────────────┬────────────────┘
│  │ env: "dev"           │    │                           │ file_sd scrape
│  └──────────────────────┘    │                           │
│                              │           ┌───────────────▼────────────────┐
│  ┌──────────────────────┐    │           │  Infra VM（Docker）             │
│  │ Promtail DaemonSet   │────┼─ push ──→ │  PostgreSQL + exporter         │
│  │ → pod logs → Loki    │    │   logs    │  Redis + exporter              │
│  └──────────────────────┘    │           │  RabbitMQ (built-in metrics)   │
└──────────────────────────────┘           └────────────────────────────────┘
```

**Metrics 收集雙軌：**
- **App VM → Prometheus Agent**：kubernetes_sd_configs 自動發現 pod，scrape metrics，remote_write 到 central Prometheus（帶 `env` label）
- **Infra VM → Central Prometheus**：file_sd_configs 直接 scrape exporter（target JSON 檔案，修改後 30 秒自動生效）

**K3s ExternalService 定義**（`k8s/external-services/`）：

| Service Name | 目標 IP | Ports | 說明 |
|---|---|---|---|
| `postgresql` | `<INFRA_VM_IP>` | 5432 | PostgreSQL |
| `redis` | `<INFRA_VM_IP>` | 6379 | Redis |
| `rabbitmq` | `<INFRA_VM_IP>` | 5672 | RabbitMQ |
| `otel-collector` | `<INFRA_VM_IP>` | 4317 (gRPC), 4318 (HTTP) | OpenTelemetry Collector |
| `loki` | `<INFRA_VM_IP>` | 3100 | Loki（Promtail 推送 log） |
| `prometheus` | `<INFRA_VM_IP>` | 9091 | Prometheus（Prometheus Agent remote_write 目標） |

k3s 內的 service 用 `otel-collector:4317` 等 ExternalName 作為 endpoint，所有環境配置一致。Monitoring VM IP 變更時，只需更新 `k8s/external-services/` 裡的 Endpoints IP 即可。

### 1.2 技術棧（LGTM + OTel）

| 元件 | 用途 | Port |
|------|------|------|
| **OTel Collector** | 接收 OTLP，分發至 Tempo / Prometheus / Loki | 4317 (gRPC), 4318 (HTTP) |
| **Prometheus** | Metrics 儲存，scrape app VMs / infra VMs | 9091 (外部) |
| **Loki** | Log 聚合儲存 | 3100 |
| **Tempo** | Distributed tracing 儲存 | 3200 |
| **Alertmanager** | 告警路由，依環境分級通知 | 9093 |
| **Grafana** | 統一視覺化儀表板 | 3000 |

---

## 2. 設計決策

### 2.1 為何選擇 LGTM + OTel 而非其他方案？

| 方案 | 優點 | 缺點 | 決策 |
|------|------|------|------|
| **LGTM + OTel**（本專案） | 全開源、免費、Grafana 原生整合、Trace-Log-Metric 互相跳轉 | 自行維運 | ✅ 採用 |
| Datadog / New Relic | SaaS 免維運、功能完整 | 費用昂貴（按量計費，微服務數量多時成本高） | ❌ 不採用 |
| ELK Stack | Log 搜尋能力強 | 不含 Tracing、資源消耗高、設定複雜 | ❌ 不採用 |

### 2.2 為何用雙軌收集（OTel Push + Promtail）？

**微服務 → OTel Collector（Push）**：
- 應用程式 SDK 產生的 traces / metrics 帶有 `trace_id`，能做 Trace → Log 跳轉
- Traces、Metrics 用同一條 pipeline，架構統一

**微服務 Log → Promtail DaemonSet（Push to Loki）**：
- 每台 App VM (k3s) 部署 Promtail DaemonSet，收集 `/var/log/pods/hawkr_*` 下的 pod log
- 透過 `pipeline_stages` 從檔案路徑提取 `service_name`、`container` label
- 透過 regex 從 log 內容提取 `trace_id` 作為 structured metadata
- 推送至 Infra VM 的 Loki（透過 K8s ExternalService `loki:3100`）

**基礎設施 Container → Promtail（Docker socket）**：
- PostgreSQL / Redis / RabbitMQ 等基礎設施跑在 Infra VM 的 Docker Compose 上
- Promtail 透過 Docker socket 無侵入式收集 stdout/stderr
- Docker 服務發現自動提取 container_name、compose_service 等 label

### 2.3 為何 Prometheus 用 Remote Write 而非 Scrape OTel Collector？

舊做法：OTel Collector 開啟 `prometheus` exporter 在 `:8889`，讓 Prometheus 來 scrape（Pull）。

新做法：OTel Collector 用 `prometheusremotewrite` 直接推送（Push）給 Prometheus。

**選 Remote Write 的原因：**
- Prometheus 啟動時加了 `--web.enable-remote-write-receiver`，已原生支援
- Push 模式延遲更低（不用等下一個 scrape interval）
- 減少 OTel Collector 需要對外暴露的端口數量

### 2.4 為何監控設定的 IP 不入 Git？

App VM / Infra VM 的 IP 透過以下機制管理，均不進 git：

| 設定 | 存放位置 | 使用時機 |
|------|---------|---------|
| Infra scrape 目標 IP | `monitoring/prometheus/targets/*.json`（gitignored） | Prometheus file_sd_configs 每 30 秒熱載入 |
| App metrics 收集 | K3s Prometheus Agent（kubernetes_sd_configs 自動發現 pod） | 自動，無需設定 IP |
| Promtail 推送目標 IP（Loki） | K8s ExternalService `loki` endpoint | Promtail DaemonSet 設定 |
| 服務的 INFRA VM IP（DB / Redis 等） | K8s ExternalService endpoint / GitHub Secret | K8s manifest 或 CI/CD 注入 |

> `prometheus.yml` 本身是靜態檔案（可進 git），IP 只存在於 `targets/*.json`。新增 Infra 目標只改 JSON 檔案，不需重啟 Prometheus。

### 2.5 為何不在每個環境各自部署一套監控？

三個環境共用一套監控 VM 的理由：
- 小團隊維護成本低（一個 Grafana 看所有環境）
- 告警集中管理，prod 和 non-prod 用 `env` label 路由分級通知
- 監控本身不需要高可用（prod 服務掛掉不影響監控，反而要靠監控發現問題）

---

## 3. 資料收集策略

### 2.1 應用遙測：OTel SDK → OTel Collector（Push）

各微服務整合 OTel SDK，透過 OTLP 協定主動推送至監控 VM 的 OTel Collector。

**資料流：**
```
微服務 (OTel SDK)
  │  OTLP gRPC/HTTP
  ▼
OTel Collector (監控 VM)
  ├── Traces  → Tempo
  ├── Metrics → Prometheus (remote write)
  └── Logs    → Loki
```

**服務需設定的環境變數：**
```
OTEL_EXPORTER_OTLP_ENDPOINT=otel-collector:4317       # Go 服務 gRPC（K8s ExternalService）
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4318 # Java 服務 HTTP
ENV=prod                                               # Go 服務 pkg/otel 的 deployment.environment
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=prod   # Java 服務由 K8s Deployment env 或 CI/CD 注入
```

### 3.2 Pod Log：Promtail DaemonSet（Push to Loki）

每台 App VM (k3s) 部署 Promtail DaemonSet，收集 `hawkr` namespace 下所有 pod log，推送至 Infra VM 的 Loki。

**設定檔：** `k8s/monitoring/promtail-daemonset.yaml`（ConfigMap 與 DaemonSet 定義於同一份 manifest）

**提取的 label 與 metadata：**

| 來源 | 提取方式 | 用途 |
|------|---------|------|
| `service_name` | `pipeline_stages` regex 從 filename 提取 | Loki label（低基數） |
| `container` | `pipeline_stages` regex 從 filename 提取 | Loki label |
| `trace_id` | `pipeline_stages` regex 從 log 內容提取 | Loki structured metadata（高基數） |
| `environment` | `static_configs` 固定值 | Loki label |

### 2.3 基礎設施 Metrics：Prometheus Scrape（Pull）

Prometheus 從監控 VM 以 file_sd（`targets/*.json`）scrape **Infra VM 上的 exporter 與 co-located sql-exporter**。**App 服務 metrics 不走 file_sd**——一律由 App VM 的 Prometheus Agent（kubernetes_sd）remote_write 進來（見 §1.1 / §2.4），避免同一服務被 agent + file_sd 雙重收集（會使 `sum`/`rate` 灌水、TSDB series 翻倍）。

Infra VM 需預先部署：
- `postgres_exporter`（port 9187）
- `redis_exporter`（port 9121）
- RabbitMQ 本身有內建 Prometheus 端點（port 15692）

### 3.3 Trace correlation 雙通道與 baggage 政策（Gateway → 內部 gRPC）

Gateway 到內部服務同時存在**兩條** correlation 通道，各有用途，**禁止再加第三套**：

| 通道 | 載體 | 傳遞機制 | 用途 |
|------|------|---------|------|
| W3C `traceparent`/`tracestate` | gRPC metadata | `otelgrpc` stats handler 自動 Inject/Extract（`pkg/middleware/grpc/interceptor/otel_interceptor.go`；gateway client 端掛在 `gateway/internal/grpc/client.go` DefaultDialOpts） | Tempo span 父子鏈的唯一來源 |
| 自訂 `x-trace-id`/`x-request-id` | gRPC metadata | `gateway/internal/middleware/tracing.go` → `WithTraceContext` → `AuthMetadataInterceptor`（`gateway/internal/grpc/metadata_interceptor.go`；API 設計規範 §6.2） | Response envelope 的 `traceId`、log correlation；不建立 span 關係 |

**Baggage 政策：**

- Gateway 在 trust boundary 丟棄 browser 自帶的 W3C `baggage` header（`tracing.go` Extract 後 `ContextWithoutBaggage`）——否則 outbound `otelgrpc` 會把外部可控值原樣注入所有內部 gRPC metadata。
- 內部服務間目前不使用 baggage。未來要用時由 gateway 在丟棄點之後自行寫入，永不透傳外部值；如需轉送外部 baggage，必須先建立 allowlist 與 size limit 的獨立 review。

**已完成收斂（#21）：** gateway 與其他 Go 服務均透過 `pkg/otel` 使用 OTLP gRPC（4317）與同一套 resource／sampler 設定；Java 服務仍使用 OTLP HTTP（4318）。

---

## 3. 環境隔離（env label）

所有 Metrics、Traces、Logs 都會帶上 `env` 標籤，在 Grafana 中可用 `env` 變數過濾。

| 標籤值 | 來源 | 對應 VM |
|--------|------|---------|
| `prod` | Go `ENV`／Java `OTEL_RESOURCE_ATTRIBUTES` + Promtail `static_configs` `environment` label | Prod App VM |
| `dev` | 同上 | Dev App VM |
| `non-prod` | Prometheus scrape label | Infra VM |
| `monitoring` | Promtail `environment` label | 監控 VM 自身 containers |

---

## 4. 監控 VM 部署（首次設定）

### 4.1 前置條件

- 監控 VM 已安裝 Docker + Docker Compose
- 監控 VM 已安裝 `envsubst`（`apt install gettext-base`）
- 所有 App VM / Infra VM 對監控 VM 的防火牆已開放（見第 7 節）

### 4.2 Clone 並設定

```bash
git clone <repo>
cd backend-payuni/monitoring

# 複製並填入環境變數
cp .env.example .env
vim .env
```

`.env` 需填入的值（對照 `.env.example`）：

```bash
PROD_APP_IP=
STAGE_APP_IP=
DEV_APP_IP=
PROD_INFRA_IP=
STAGE_DEV_INFRA_IP=

GF_SECURITY_ADMIN_PASSWORD=<設定一個強密碼>
```

### 4.3 啟動

```bash
# 必須用 make up，它會先 envsubst 生成 prometheus.yml 再啟動
make up
```

> ⚠️ **不要直接 `docker compose up -d`**，因為 `prometheus.yml` 是從 template 生成的，直接啟動會找不到檔案。

### 4.4 驗證

```bash
docker compose ps             # 確認所有服務 running
docker compose logs -f prometheus  # 確認 prometheus 載入成功
```

存取 Grafana：`http://[IP_ADDRESS]`（帳號 admin，密碼為 .env 中設定的值）

---

## 5. CI/CD 整合（App VM 自動部署）

所有 App VM 使用 k3s 部署微服務，CI/CD 透過 `kubectl apply` 或 image tag 更新部署。

### 5.1 Branch → 環境對應

| Git Branch | 環境 | App VM |
|------------|------|--------|
| `main` | prod | [IP_ADDRESS] |
| 其他 | dev | [IP_ADDRESS] |

### 5.2 監控相關的 K8s 資源

每個環境的 k3s cluster 需部署以下監控相關資源：

**ExternalService（連線 Infra VM）：**
- `otel-collector`、`loki`、`postgresql`、`redis`、`rabbitmq`

**Promtail DaemonSet：**
- 使用 `k8s/monitoring/promtail-daemonset.yaml` 定義的 ConfigMap（`k8s/Makefile`／`k8s/scripts/init-environment.sh` 用 `sed` 替換 `PLACEHOLDER_ENV` 後 `kubectl apply`）
- 掛載 `/var/log/pods` 收集 pod log

**各微服務 Deployment env：**
```yaml
- name: OTEL_EXPORTER_OTLP_ENDPOINT
  value: "otel-collector:4317"        # Go 服務 gRPC
- name: ENV
  value: "prod"                       # Go pkg/otel → deployment.environment
- name: OTEL_RESOURCE_ATTRIBUTES
  value: "deployment.environment=prod" # Java 服務依環境設定
```

### 5.3 GitHub Secrets 清單

在 GitHub Repo Settings → Secrets and variables → Actions 設定：

| Secret 名稱 | 說明 | 範例值 |
|------------|------|--------|
| `MONITORING_VM_IP` | Infra VM IP（監控棧所在） | `[IP_ADDRESS]` |
| `FIREBASE_CREDENTIALS_JSON_PROD` | Firebase 憑證 | JSON 字串 |
| `FIREBASE_CREDENTIALS_JSON_STAGE` | 同上 | 同上 |
| `FIREBASE_CREDENTIALS_JSON_DEV` | 同上 | 同上 |
| `PROD_HOST` | Prod App VM IP | `[IP_ADDRESS]` |
| `DEV_HOST` | Dev App VM IP | `[IP_ADDRESS]` |
| `SSH_PRIVATE_KEY` | SSH 私鑰 | PEM 格式 |
| `GHCR_PAT` | GitHub Container Registry Token | ghp_xxx |

---

## 6. 告警規則與通知

### 6.1 規則分類

| 檔案 | 覆蓋範圍 |
|------|---------|
| `prometheus/rules/service-alerts.yml` | 服務 down、HTTP 5xx、P99 延遲、gRPC 錯誤率 |
| `prometheus/rules/infra-alerts.yml` | PostgreSQL 連線池、Redis 記憶體、RabbitMQ queue、CPU/記憶體/磁碟 |
| `prometheus/rules/business-alerts.yml` | 訂單量驟降、支付失敗率、庫存鎖定失敗、低庫存 |

所有規則都掛載至 `/etc/prometheus/rules/*.yml` 並在 `prometheus.yml` 透過 `rule_files` 載入。

### 6.2 告警嚴重度路由

```
Alertmanager 路由邏輯：
  channel=business, env=dev   → Discord dev-警報（repeat 2h）
  channel=business, env=prod  → Discord prod-警報（repeat 30m）
  其他（infra）,     env=dev   → Discord monitor-alert（repeat 2h）
  其他（infra）,     env=prod  → Discord monitor-alert（repeat 30m）
```

**架構：** Alertmanager → webhook → `discord-webhook-gateway` container → Discord Webhook API

**前提：** Infra VM `.env` 需設定 `DISCORD_WEBHOOK_URL`（必填，monitor-alert 頻道）；`DISCORD_WEBHOOK_URL_DEV_ALERT` / `DISCORD_WEBHOOK_URL_PROD_ALERT`（選填，business 頻道，未設定時該路徑不註冊，Alertmanager 指向時回 404）。見 §17.1。

### 6.3 Business Metrics 前提條件

`business-alerts.yml` 裡的告警依賴各微服務主動 export 以下 Prometheus metrics：

| Metric 名稱 | 應由哪個服務 export |
|------------|-------------------|
| `hawkr_orders_created_total` | order-service |
| `hawkr_payment_failed_total` | payment-service |
| `hawkr_cart_created_total` | cart-service |

> ⚠️ 若服務尚未 export 上述 metrics，對應告警會持續顯示 `no data`，不會誤報但也不會正確觸發。

---

## 7. 防火牆規則

### Infra VM 需開放 Inbound

所有 App VM (k3s) 透過 ExternalService 連線到 Infra VM，需開放以下 port：

| Port | 來源 | 用途 |
|------|------|------|
| 4317 | 所有 App VM (k3s) | OTel OTLP gRPC（Go 服務推送 traces/metrics） |
| 4318 | 所有 App VM (k3s) | OTel OTLP HTTP（Java 服務推送 traces/metrics） |
| 3100 | 所有 App VM (k3s) | Loki（Promtail DaemonSet 推送 logs） |
| 3000 | 開發者 IP | Grafana |
| 9093 | 開發者 IP | Alertmanager（選用） |

### App VM / Infra VM 需開放 Inbound（來源：監控 VM）

| Port | 用途 |
|------|------|
| 9900 以上各服務 port | Prometheus scrape metrics |
| 9187 | postgres_exporter |
| 9121 | redis_exporter |
| 15692 | RabbitMQ Prometheus 端點 |

---

## 8. Grafana 儀表板

### 8.1 存取

URL：`http://<INFRA_VM_IP>:3000`
帳號：`admin`
密碼：`.env` 中的 `GF_SECURITY_ADMIN_PASSWORD`

> 匿名存取已關閉，必須登入才能使用。

### 8.2 預設儀表板（自動 Provisioning）

| 儀表板 | 用途 |
|--------|------|
| **Overview** | 全環境服務健康狀態、流量、錯誤率 |
| **Services** | 各微服務 HTTP/gRPC 詳細指標、延遲、記憶體 |
| **Business** | 訂單量、支付成功率、庫存鎖定、購物車 |
| **Infrastructure** | PostgreSQL、Redis、RabbitMQ 詳細指標 |
| **PostgreSQL** | DB 連線、查詢效能、資料表狀態 |

所有儀表板支援 `env` 變數過濾（右上角下拉選 prod / dev）。

### 8.3 Trace → Log 跳轉

Tempo 追蹤頁面中，點擊任一 Span 的 `trace_id`，可直接跳轉至 Loki 中對應的 log 行（前提：應用程式在 log 中輸出 `trace_id=xxx` 格式）。

---

## 9. 常用維運指令

```bash
# 監控 VM
cd backend-payuni/monitoring

make up          # 啟動（自動 envsubst 生成 prometheus.yml）
make down        # 停止
make restart     # 重啟
make logs        # 追蹤所有服務 log

# 查看特定服務
docker compose logs -f prometheus
docker compose logs -f alertmanager
docker compose logs -f otel-collector

# 重新載入 Prometheus 設定（不重啟）
curl -X POST http://localhost:9091/-/reload

# 查看目前 active alerts
curl http://localhost:9093/api/v2/alerts
```

---

## 10. 目錄結構說明

```
monitoring/
├── .env                        # 實際 IP（gitignored，從 .env.example 複製）
├── .env.example                # IP 範本（含各 VM IP 說明）
├── .gitignore                  # 排除 .env、smtp_password、prometheus.yml
├── Makefile                    # make up / down / restart / logs
├── docker-compose.yml          # 監控 VM 上的所有服務
│
├── alertmanager/
│   ├── config.yml              # 路由規則（prod 分開通知）
│   └── smtp_password           # Gmail App Password（gitignored）
│
├── grafana/provisioning/
│   ├── dashboards/             # 儀表板 JSON 自動載入設定
│   └── datasources/            # Prometheus / Loki / Tempo 資料源設定
│
├── loki/
│   └── local-config.yaml       # Loki 設定（7 天 log 保留）
│
├── otel-collector/
│   └── otel-config.yaml        # OTel Collector 設定（含 memory limiter）
│
├── prometheus/
│   ├── prometheus.yml          # 實際設定（gitignored，由 make up 生成）
│   ├── prometheus.yml.template # 含 ${VAR} 佔位符的設定範本（在 git）
│   └── rules/
│       ├── service-alerts.yml
│       ├── infra-alerts.yml
│       └── business-alerts.yml
│
├── promtail/
│   ├── promtail-config.yaml        # Infra VM 自身 Docker container log 收集
│   └── promtail-agent-config.yaml  # （保留）Docker Compose 環境用的 agent 設定
│
└── tempo/
    └── tempo-config.yaml       # Tempo 追蹤儲存設定
```

---

## 11. Trace 查詢操作指南（Tempo）

### 11.1 透過 Grafana UI

1. 進入 Grafana → **Explore**
2. 選擇 Datasource: **Tempo**
3. 查詢模式：

**By Trace ID（最精準）**
- 選擇 "TraceQL" 查詢類型
- 直接貼上 traceId，例如：`9fa6ad79b4246b64770c86c924df2098`

**By Service（搜尋某服務的 trace）**
```
{ resource.service.name = "cart-service" }
```

**By gRPC Method（搜尋某 API 的 trace）**
```
{ name = "hawkr.product.ProductService/GetProductList" }
```

**By Duration（搜尋慢請求）**
```
{ resource.service.name = "product-service" && duration > 500ms }
```

**By Error（搜尋錯誤 trace）**
```
{ status = error }
```

### 11.2 透過 HTTP API

```bash
# 用 traceId 直接查詢
curl -s "http://localhost:3200/api/traces/{traceId}" | python3 -m json.tool

# 搜尋最近的 trace
curl -s "http://localhost:3200/api/search?q={resource.service.name=\"product-service\"}&limit=5"
```

### 11.3 Trace 結構說明

一個完整的跨服務 trace 包含：

```
Gateway (root span)
  └── cart-service/AddItem (gRPC SERVER span)          ← Spring Boot auto-instrumentation
      ├── grpc.server/CartService/AddItem              ← ObsGrpcServerInterceptor 自訂 span
      └── product-service/GetProductSimple (CLIENT)
          ├── grpc.server/ProductService/GetProductSimple  ← ObsGrpcServerInterceptor
          └── ProductService/GetProductSimple (SERVER)     ← Spring Boot auto-instrumentation
```

每個 gRPC 呼叫會產生兩種 span：
1. **Spring Boot auto-instrumentation span**: 由 `grpc-spring-boot-starter` 的 `OTelClientStatsHandler` 產生
2. **ObsGrpcServerInterceptor span**: 由自訂攔截器產生，包含更多業務資訊（correlation_id、access log）

### 11.4 從 X-Trace-ID 追蹤

Gateway 每個 HTTP 回應都帶 `X-Trace-ID` header：

```bash
curl -s -D - "http://localhost:9900/api/v1/products?page=1&size=3" 2>&1 | grep -i "x-trace-id"
# X-Trace-ID: b6c83aa439d91c52b935b0fce4a0a153
```

將此 ID 貼入 Grafana Tempo 即可查看完整 trace 樹。

---

## 12. Log 查詢操作指南（Loki）

### 12.1 JSON 日誌格式

所有 Java 微服務使用 `logstash-logback-encoder` 輸出 JSON 格式：

```json
{
  "ts": "2026-02-12T16:20:25.814+09:00",
  "msg": "",
  "logger": "io.github.will413028.hawkr.common.obs.grpc.ObsGrpcServerInterceptor",
  "level": "INFO",
  "trace_id": "9fa6ad79b4246b64770c86c924df2098",
  "span_id": "7860842ae1fa2058",
  "correlation_id": "bb80bec0-6cb5-4ff2-9701-15b2c9fdcd4c",
  "service": "cart-service",
  "event": "CartService.AddItem.start"
}
```

### 12.2 日誌欄位說明

| 欄位 | 說明 | 格式 |
|------|------|------|
| `trace_id` | OTel Trace ID | 32 hex chars，與 Gateway X-Trace-ID 一致 |
| `span_id` | OTel Span ID | 16 hex chars |
| `correlation_id` | 業務關聯 ID | UUID v4，在整個請求鏈路中一致 |
| `service` | 微服務名稱 | 如 `cart-service`、`product-service` |
| `event` | 事件名稱 | 格式: `ClassName.method.result` |

### 12.3 Event 命名規範

```
{GrpcService}.{Method}.start    ← 請求開始
{GrpcService}.{Method}.end      ← 請求正常結束
{GrpcService}.{Method}.error    ← 請求異常
```

### 12.4 Loki 查詢語法（LogQL）

所有環境統一使用 k3s Promtail DaemonSet，label 命名一致：

| Label | 說明 | 範例 |
|-------|------|------|
| `service_name` | 微服務名稱 | `cart-service`、`gateway` |
| `container` | 容器名稱 | `cart-service`、`gateway` |
| `environment` | 環境 | `dev`、`prod` |
| `job` | 固定值 | `kubernetes-pods` |
| `trace_id` | Structured metadata | `9fa6ad79b4246b64770c86c924df2098` |

```logql
# 搜尋某服務的所有日誌
{service_name="cart-service"}

# 指定環境
{service_name="cart-service", environment="prod"}

# 用 trace_id 跨服務追蹤（structured metadata 過濾，效能最佳）
{job="kubernetes-pods"} | trace_id = "9fa6ad79b4246b64770c86c924df2098"

# 搜尋錯誤日誌（Go + Java 皆為 JSON 格式）
{service_name="payment-service"} | json | level = "error"

# 搜尋特定事件
{service_name="order-service"} |= "OrderService.CreateOrder.error"

# 搜尋慢請求
{service_name="product-service"} | json | level = "warn"

# 只看有 trace_id 的 log
{job="kubernetes-pods"} | trace_id != ""

# 搜尋特定 correlation_id
{job="kubernetes-pods"} |= "correlation_id" |= "bb80bec0-6cb5-4ff2-9701-15b2c9fdcd4c"
```

### 12.5 trace_id Structured Metadata

Promtail DaemonSet 會從 log 內容中提取 `trace_id` 並存為 Loki structured metadata（非 label，避免高基數問題）。所有環境皆已啟用。

**Go 與 Java 服務統一輸出 JSON 格式：**

Go 服務：
```json
{"level":"info","ts":"2026-03-27T08:46:28.344+0800","caller":"interceptor/logging.go:81","msg":"gRPC request completed","trace_id":"abc123...","span_id":"def456..."}
```

Java 服務：
```json
{"ts":"2026-03-27T08:46:28Z","level":"INFO","msg":"...","trace_id":"abc123...","span_id":"def456..."}
```

**提取規則：** regex 匹配 `"trace_id": "..."` 或 `"trace_id":"..."`（含/不含空格皆可）。

因為 Go 和 Java 服務都輸出 JSON，Loki 的 `| json` parser 可統一解析所有服務的 log 欄位。

### 12.6 用 kubectl 直接搜尋日誌

```bash
# 搜尋特定 traceId 的所有日誌（跨服務）
kubectl logs -l app.kubernetes.io/part-of=hawkr -n hawkr --all-containers | grep "9fa6ad79b4246b64770c86c924df2098"

# 搜尋特定服務的錯誤
kubectl logs deploy/payment-service -n hawkr | grep '"level":"ERROR"' | tail -10

# 即時追蹤某服務 log
kubectl logs -f deploy/gateway -n hawkr
```

---

## 13. 效能監控操作指南（Prometheus）

### 13.1 常用 PromQL 查詢

**JVM 記憶體使用**
```promql
jvm_memory_used_bytes{area="heap"}
```

**gRPC 請求速率**
```promql
rate(grpc_server_calls_total[5m])
```

**gRPC 請求延遲 P99**
```promql
histogram_quantile(0.99, rate(grpc_server_call_duration_seconds_bucket[5m]))
```

**HTTP 請求速率（Gateway）**
```promql
rate(http_server_requests_seconds_count[5m])
```

### 13.2 已暴露的 Actuator Endpoints

| Endpoint | URL | 說明 |
|----------|-----|------|
| Health | `/actuator/health` | 健康檢查（含詳細資訊） |
| Info | `/actuator/info` | 應用資訊 |
| Metrics | `/actuator/metrics` | 全部指標列表 |
| Prometheus | `/actuator/prometheus` | Prometheus 格式指標 |

---

## 14. 常用除錯流程

### 14.1 微服務 Log Debug 完整指南

以下是在 Grafana 中對微服務進行 log debug 的完整步驟。

#### Step 1: 進入 Grafana Explore

1. 打開 `http://<INFRA_VM_IP>:3000`，登入
2. 左側選 **Explore**（指南針圖示）
3. 上方 Datasource 選 **Loki**

#### Step 2: 找到問題 log

**情境 A — 知道哪個服務出問題：**
```logql
{service_name="order-service"} |= "ERROR"
```

**情境 B — 不確定哪個服務，用關鍵字搜：**
```logql
{job="kubernetes-pods"} |= "connection refused"
```

**情境 C — 看某段時間的所有 error：**
```logql
{job="kubernetes-pods"} |~ "ERROR|FATAL|panic"
```

> 右上角時間範圍選擇器可調整查詢時段（如 Last 15 minutes、Last 1 hour）。

#### Step 3: 用 trace_id 追蹤跨服務鏈路

從 error log 中找到 `trace_id`，然後查詢同一個 request 在所有服務的 log：

```logql
{job="kubernetes-pods"} | trace_id = "4408d9f7f14b5275d12ca52751375282"
```

這會列出該 request 經過的所有微服務（如 gateway → order-service → product-service → 單體）的完整 log。

#### Step 4: 跳轉 Tempo 看 Trace 樹

在 Loki log 結果中，如果 log 行包含 `trace_id`，Grafana 會自動顯示 **View Trace** 連結。點擊後跳轉到 Tempo，以視覺化方式查看：
- 每個服務的處理時間
- 哪個 span 最慢
- 哪個 span 出錯

#### Step 5: 結合 Metrics 確認系統狀態

在 Grafana 切換到 **Prometheus** datasource，查看出問題時段的 metrics：

```promql
# 該服務的 gRPC 錯誤率
rate(grpc_server_calls_total{service_name="order-service", grpc_code!="OK"}[5m])

# JVM 記憶體（Java 服務）
jvm_memory_used_bytes{service_name="order-service", area="heap"}
```

### 14.2 常見問題排查速查表

| 症狀 | 第一步 | LogQL 查詢 |
|------|--------|-----------|
| API 回 500 | 從 gateway log 找 trace_id | `{service_name="gateway"} \|= "status\": 500"` |
| gRPC UNAVAILABLE | 查目標服務是否活著 | `{service_name="目標服務"} \|= "started"` |
| 回應很慢 | Tempo 找慢 trace | Tempo: `{ duration > 2s }` |
| MQ 消費失敗 | 查 consumer 的 ERROR | `{service_name="order-service"} \|= "ERROR" \|= "consumer"` |
| DB 連線失敗 | 查服務啟動 log | `{service_name="xxx"} \|= "connection" \|= "refused"` |

### 14.3 使用者回報問題 → 追蹤完整鏈路

```
1. 取得 traceId：
   - 方法 A: 從 API response header `X-Trace-ID`
   - 方法 B: 從 App 端錯誤日誌
   - 方法 C: 在 Loki 搜尋關鍵字找到對應 log，取出 trace_id
2. 在 Grafana Loki 用 trace_id 查詢跨服務 log：
   {job="kubernetes-pods"} | trace_id = "<traceId>"
3. 或在 Grafana Tempo 貼入 traceId，查看 trace 樹
4. 點擊 "Logs for this span" 跳轉到 Loki 查看該 span 的詳細日誌
```

### 14.4 發現慢請求

```
1. Grafana → Explore → Tempo
2. 查詢: { resource.service.name = "product-service" && duration > 1s }
3. 查看 span 樹，找到耗時最長的子 span
4. 結合 Prometheus 查看當時的 JVM 指標
```

### 14.5 MQ 消費失敗

```
1. 搜尋 Order Service 的 ERROR 日誌
   {service_name="order-service"} |= "ERROR"
2. 取得日誌中的 trace_id 或 correlation_id
3. 用 trace_id 查詢完整鏈路：
   {job="kubernetes-pods"} | trace_id = "<trace_id>"
4. 在 Tempo 中查看 Payment → MQ → Order 的完整 trace
```

### 14.6 確認服務是否正常匯出 Trace

```bash
# 1. 發送測試請求
curl -s -D - "http://localhost:9900/api/v1/products?page=1&size=1" 2>&1 | grep "X-Trace-ID"

# 2. 等待 10 秒（OTLP batch exporter 預設間隔）

# 3. 查詢 Tempo
curl -s "http://localhost:3200/api/traces/{traceId}" | python3 -m json.tool
```

---

## 15. 跨服務追蹤原理

### 15.1 Trace Context 傳播鏈

```
Client Request
    │
    ▼
Gateway (TracingMiddleware)
    │  產生 root span + traceId
    │  設定 X-Trace-ID response header
    │
    │  gRPC call（OTelClientStatsHandler 注入 traceparent metadata）
    ▼
Java 微服務 A (gRPC Server)
    │  Spring Boot auto-instrumentation 建立 server span
    │  ObsGrpcServerInterceptor:
    │    1. 從 metadata 萃取 traceparent (TextMapPropagator.extract)
    │    2. 建立 server span (Tracer.spanBuilder)
    │    3. 寫入 MDC: trace_id, span_id, correlation_id, service
    │
    │  gRPC call to Service B（ObsGrpcClientInterceptor 注入 traceparent + correlation_id）
    ▼
Java 微服務 B (gRPC Server)
    │  同上流程（trace_id 一致，correlation_id 由上游傳入）
    │
    │  MQ publish（ObsRabbitPublisher 注入 traceparent + correlation_id 到 headers）
    ▼
Java 微服務 C (MQ Consumer)
    │  ObsRabbitListenerAdvice:
    │    1. 從 message headers 萃取 traceparent
    │    2. 建立 consumer span
    │    3. correlation_id 延續
```

### 15.2 雙軌追蹤機制

| 機制 | ID | 傳播方式 | 用途 |
|------|-----|---------|------|
| **OTel Trace** | `trace_id` (32 hex) | W3C traceparent header | 鏈路追蹤、Tempo 查詢 |
| **Correlation ID** | UUID v4 | 自訂 gRPC metadata / MQ header | 業務關聯、日誌搜尋 |

### 15.3 關鍵元件

| 元件 | 位置 | 職責 |
|------|------|------|
| `OTelServerStatsHandler` | 所有 Go 服務 gRPC server（11/11） | 接收 traceparent，建立 server span |
| `OTelClientStatsHandler` | Gateway + 所有 Go 服務 gRPC client（全覆蓋） | 注入 `traceparent` 到 gRPC metadata |
| `ObsGrpcServerInterceptor` | observability-common（Java） | 萃取 traceparent、建立 span、管理 MDC |
| `ObsGrpcClientInterceptor` | observability-common（Java） | 注入 traceparent + correlation_id 到下游 gRPC |
| `ObsRabbitPublisher` | observability-common（Java） | 注入 traceparent + correlation_id 到 MQ headers |
| `ObsRabbitListenerAdvice` | observability-common（Java） | 萃取 MQ headers、建立 consumer span |

---

## 16. OTel 配置參考

### 16.1 統一預設配置

所有配置集中在 `observability-common` 的 `observability-defaults.properties`：

```properties
# OTel SDK
otel.sdk.disabled=false
otel.service.name=${hawkr.obs.service}
otel.traces.exporter=${OTEL_TRACES_EXPORTER:otlp}
otel.exporter.otlp.endpoint=${OTEL_EXPORTER_OTLP_ENDPOINT:http://127.0.0.1:4318}

# Actuator
management.endpoints.web.exposure.include=health,info,metrics,prometheus
management.endpoint.health.show-details=always

# Distributed Tracing
management.tracing.enabled=${TRACING_ENABLED:true}
management.tracing.sampling.probability=${TRACING_SAMPLING:1.0}
management.otlp.tracing.endpoint=${OTEL_EXPORTER_OTLP_ENDPOINT:http://127.0.0.1:4318}/v1/traces
```

### 16.2 各微服務配置

每個微服務的 `application.yml` 只需設定服務名稱：

```yaml
hawkr:
  obs:
    service: <service-name>     # 必填：服務名稱
    access-log:
      ok-sample-rate: 1.0       # OK 請求取樣率（1.0 = 100%，正式環境建議 0.1）
      slow-threshold-ms: 1000   # 慢請求閾值（超過此值 WARN 級別記錄）
```

### 16.3 環境變數覆蓋

| 環境變數 | 預設值 | 說明 |
|---------|--------|------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://127.0.0.1:4318` | OTLP endpoint |
| `TRACING_ENABLED` | `true` | 啟用/停用 tracing |
| `TRACING_SAMPLING` | `1.0` | 取樣率（正式環境建議 0.1~0.5） |

---

## 17. 待改善項目

### 17.1 告警通知（已完成，經 Discord；原 LINE Notify 計畫已棄用未實作）

Alertmanager 目前經 `discord-webhook-gateway` 送往 Discord（見 §6.2），最初規劃的 LINE Notify 從未實作、已棄用。

**設定步驟：**
1. Discord 頻道 → 整合 → Webhook → 建立/複製 Webhook URL
2. SSH 進 Infra VM，在 `monitoring/.env` 設定 `DISCORD_WEBHOOK_URL`（必填，monitor-alert 頻道）；`DISCORD_WEBHOOK_URL_DEV_ALERT` / `DISCORD_WEBHOOK_URL_PROD_ALERT`（選填，business 頻道）
3. `make up`（重建 discord-webhook-gateway container）

### 17.2 監控自身的監控（Dead-Man's-Switch）

監控 VM / Prometheus / Alertmanager 自己掛掉時，所有告警會靜默且無人通知。**這個盲點目前沒有補上**：Watchdog 心跳（永遠 firing 的 `Watchdog` rule ＋ Alertmanager `deadman` route ＋ stack 外的 healthchecks.io 端點）曾實作過，但因為沒有外部 heartbeat 端點可打而整批撤回（`monitoring/prometheus/rules/watchdog.yml` 與 alertmanager 的 `deadman` route 現已不存在，`monitoring/` 內 grep 不到任何 `Watchdog` / `deadman`）。**留著不 arm 反而危險**——永遠 firing 的 Watchdog 少了 `deadman` route 會落到預設 discord receiver，把告警頻道灌爆。要恢復必須同時補齊三樣：rule、route、外部 heartbeat 端點。

**外部黑箱 uptime（2026-08-04 更新，backend#2192 實查）**：[UptimeRobot](https://uptimerobot.com) **已不是「可選補強」，已在用** —— 現有 4 支 monitor 覆蓋 prod / dev 的 web 與 admin 前台。原文寫的「另註冊 UptimeRobot 監控 `http://<INFRA_VM_IP>:3000`（Grafana）」是尚未落實的建議，且 Grafana 那顆至今沒建；保留在此是為了說明**外部黑箱這層的原始動機**（監控自身掛掉的盲點），不代表現況。

選型注意（2026-08-04 實測，避免下一輪重踩）：

- **UptimeRobot 免費層有 Keyword monitor**，能對 raw HTML 做內容斷言（白屏／殼頁這類「HTTP 200 但內容是空的」只有內容斷言抓得到）。但 **monitor type 建立後不可編輯**，要從 HTTP(s) 換成 Keyword 只能刪掉重建；且 **HEAD 沒有 body**，用 HEAD 就做不了任何內容斷言。
- **Sentry Uptime Monitor 做不到內容斷言**：可用的 assertion op 只有 `status_code_check` / `json_path` / `header_check`（實測送其他 op 回 400 並列出合法 variant 清單）。要內容斷言就不能靠它。
- **我方 canary 全部跑在 Cloudflare 上**（探針、rollup、告警發送皆是），`cloudflare/canary/probe.mjs` 那條 CF 外部 vantage 目前只掛 `workflow_dispatch`、不是常態路徑 → **CF 層本身故障時，我方自建告警發不出來**。外部黑箱這層正是為此存在，別再把它當「可選」。

### 17.3 Business Metrics

`business-alerts.yml` 已有 ATM 退款卡單（#1353）與假日資料覆蓋（#898）告警，metric 由 co-located sql-exporter（`targets/*infra*.json` 的 :9399，`env` 由 scrape target 提供）產出。新增業務告警見 [業務告警分流與 dev-警報 路由設定](業務告警分流與%20dev-警報%20路由設定.md)。

### 17.4 Grafana 存取限制

Grafana port 3000 對外開放，僅靠帳密保護。建議用阿里雲安全組限制來源 IP。
