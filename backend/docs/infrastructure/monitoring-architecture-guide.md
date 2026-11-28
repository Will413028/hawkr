# 監控架構完整教學 (Monitoring Architecture Guide)

> 目標讀者：剛接觸此專案的開發者，不熟悉監控工具。  
> 讀完這份文件，你能理解「為什麼這樣設計」、「每個元件在做什麼」、「出問題時去哪裡看」。

---

## 一、為什麼要監控？

服務上線之後，你無法一直盯著 terminal 看 log。需要有一套系統幫你：

1. **知道現在出問題了** → 告警（Alertmanager）
2. **知道問題發生在哪** → 追蹤（Tempo distributed tracing）
3. **知道系統現在的狀態** → 指標（Prometheus metrics）
4. **找到當時發生了什麼** → 日誌（Loki log aggregation）

這四件事共同構成「可觀測性（Observability）」。

---

## 二、三大支柱

現代可觀測性圍繞三個維度，本專案三個都有：

### Metrics（指標）
- **是什麼**：數字，反映某時間點的系統狀態
- **例子**：HTTP 請求數、錯誤率、記憶體用量、Goroutine 數量
- **工具**：Prometheus
- **特點**：低成本、可長期保存（目前設定 15 天）、適合設定閾值告警

### Logs（日誌）
- **是什麼**：文字，紀錄發生了什麼事
- **例子**：`[ERROR] order not found: order_id=123`
- **工具**：Loki
- **特點**：資訊豐富、佔空間大（7 天保留）、適合 debug

### Traces（追蹤）
- **是什麼**：一筆請求從頭到尾經過哪些服務、每段花多少時間
- **例子**：`POST /checkout → order-service(50ms) → product-service(20ms) → payment-service(100ms)`
- **工具**：Tempo
- **特點**：能找到跨服務的效能瓶頸、占空間最大（7 天保留）

### 三者如何串聯？

這是最強大的地方。在 Grafana 裡：
- 看到某個 trace_id → 點一下 → 跳到 Loki 看當時那筆請求的所有 log
- 看到 metrics 某時間點飆高 → 查同時間的 trace → 找到是哪支 API 出問題

---

## 三、整體架構圖

```
使用者手機
    │
    ▼
App VM × N（K3s）
┌─────────────────────────────────────────────┐
│                                             │
│  Gateway → [21 個微服務]                     │
│                                             │
│  每個服務內嵌 OTel SDK（Go / Java）           │
│  自動產生：Traces + Metrics                  │
│       │                                     │
│  ┌────▼──────────────────┐                  │
│  │ K8s ExternalService   │                  │
│  │ otel-collector → Infra│─── OTLP push ──→ │
│  └───────────────────────┘                  │
│                                             │
│  ┌──────────────────────┐                   │
│  │ Prometheus Agent     │─── remote_write ──┼→ Prometheus（central）
│  │ 自動發現所有 pod      │                   │
│  │ scrape /metrics      │                   │
│  └──────────────────────┘                   │
│                                             │
│  ┌──────────────────────┐                   │
│  │ Promtail DaemonSet   │─── push logs ────→│ Loki
│  │ 收集 pod stdout log  │                   │
│  └──────────────────────┘                   │
└─────────────────────────────────────────────┘

Monitoring VM（Docker Compose，Infra VM 上）
┌────────────────────────────────────────────────┐
│                                                │
│  OTel Collector（接收端）                       │
│    ← OTLP（traces + metrics + logs）           │
│    → Tempo（traces）                           │
│    → Prometheus（metrics via remote write）    │
│    → Loki（logs via OTLP HTTP）                │
│                                                │
│  Prometheus（metrics 中心）                     │
│    ← remote_write from Prometheus Agent        │
│    ← scrape from OTel Collector                │
│    ← file_sd_configs → Infra VM exporters      │
│                                                │
│  Loki（log 中心）                              │
│    ← push from OTel Collector                  │
│    ← push from Promtail DaemonSet              │
│                                                │
│  Tempo（trace 中心）                           │
│    ← push from OTel Collector                  │
│                                                │
│  Grafana（統一視覺化 :3000）                    │
│    → 查詢 Prometheus / Loki / Tempo           │
│                                                │
│  Alertmanager（:9093）                         │
└────────────────────────────────────────────────┘

Infra VM（Docker Compose，基礎設施）
┌────────────────────────────────────────────────┐
│  PostgreSQL + postgres_exporter                │
│  Redis + redis_exporter                        │
│  RabbitMQ（內建 /metrics endpoint）            │
│  Elasticsearch                                 │
│    ↑ file_sd_configs scrape（30秒熱更新）      │
└────────────────────────────────────────────────┘
```

---

## 四、每個元件詳解

### 4.1 OTel Collector（最重要的中間件）

**比喻**：像一個「快遞分揀中心」。

微服務只需要把 telemetry 送到一個地方（OTel Collector），它負責：
- 接收 traces → 轉發給 Tempo
- 接收 metrics → 轉發給 Prometheus
- 接收 logs → 轉發給 Loki

好處：微服務不需要知道後端是什麼，只管送到 OTel Collector。  
未來想換 Tempo 為其他 tracing 後端，只改 Collector 設定，所有服務不用動。

設定檔：`monitoring/otel-collector/otel-config.yaml`

```yaml
receivers:
  otlp:                    # 接收 OTLP 協議（Go/Java SDK 預設格式）
    protocols:
      grpc: { endpoint: 0.0.0.0:4317 }
      http: { endpoint: 0.0.0.0:4318 }

exporters:
  otlp/tempo:              # 送 traces 到 Tempo
    endpoint: tempo:4317
  prometheusremotewrite:   # 送 metrics 到 Prometheus
    endpoint: http://prometheus:9091/api/v1/write
  otlphttp/loki:           # 送 logs 到 Loki
    endpoint: http://loki:3100/otlp

pipelines:
  traces:
    receivers: [otlp]
    exporters: [otlp/tempo]
  metrics:
    receivers: [otlp]
    exporters: [prometheusremotewrite]
  logs:
    receivers: [otlp]
    exporters: [otlphttp/loki]
```

### 4.2 Prometheus（Metrics 中心）

**比喻**：像一個「時序資料庫」，專門存「時間 + 數字」。

**兩種收集模式：**

**Pull（拉取）**：Prometheus 主動去服務的 `/metrics` endpoint 抓數據。
- 適合：Infra VM 上的 exporter（Prometheus 能直接連到）
- 設定：`monitoring/prometheus/prometheus.yml` 的 `file_sd_configs`

**Push（推送）**：服務主動送數據給 Prometheus。
- 適合：K3s pod（ClusterIP 無法從外部直接 scrape）
- 工具：Prometheus Agent（跑在每個 App VM 的 K3s 裡）
- Agent 用 `kubernetes_sd_configs` 自動發現 pod，scrape 後 `remote_write` 給中央 Prometheus

**為何 K3s pod 不能被外部 scrape？**

K3s 裡的 pod 用 ClusterIP，只在 Kubernetes 叢集內部可見。Monitoring VM 無法跨 VM 去連 ClusterIP。所以改為 push 模式：在 K3s 裡跑 Prometheus Agent，它在內部 scrape，再推到外部。

**file_sd_configs 是什麼？**

舊做法：直接在 `prometheus.yml` 裡寫死 IP。每次新增服務要改設定、重啟 Prometheus。

新做法：`prometheus.yml` 只指定「去哪個資料夾的 JSON 檔讀 target 清單」，30 秒自動熱更新。

```
monitoring/prometheus/targets/
  infra.json          ← Infra VM 上的 exporter IP
  monitoring.json     ← Monitoring VM 自身元件
```

### 4.3 Loki（Log 中心）

**比喻**：像 Prometheus，但存的是文字 log，而不是數字。

**核心概念：Labels（標籤）**

Loki 不像 Elasticsearch 那樣對 log 內容建全文索引（那樣很吃資源）。  
Loki 只對 labels 建索引，log 內容用 grep 方式搜尋。

```
Labels（有索引，查詢快）：  service_name=order-service, env=dev
Log 內容（無索引，grep）：  order_id=123 payment failed
```

**兩種 log 進入 Loki 的方式：**

1. **OTel SDK → OTel Collector → Loki**：Go/Java 服務透過 OTel SDK 送 log，帶有 `trace_id`，可做 Trace→Log 跳轉
2. **Promtail DaemonSet**：直接讀 K3s pod 的 stdout log 檔案，推送到 Loki

### 4.4 Tempo（Trace 中心）

**比喻**：像一條「請求的生命線」，記錄一筆請求從進入 Gateway 到所有微服務的完整旅程。

**關鍵概念：**

- **Trace**：一筆請求的完整旅程
- **Span**：旅程中每一段（每個函式、每個服務呼叫）
- **Trace ID**：一筆請求唯一的 ID，跨所有服務都相同

**為何能跨服務追蹤？**

OTel SDK 會把 `trace_id` 放在 HTTP header（`traceparent`）或 gRPC metadata 裡傳遞。每個服務都把這個 ID 帶到自己產生的 span，最後 Tempo 把相同 `trace_id` 的 spans 串在一起。

### 4.5 Grafana（統一視覺化）

**比喻**：像一個「統一的控制室」，把所有監控數據展示在同一個地方。

重要功能：
- **Dashboards**：預製的視覺化圖表（Go runtime、JVM、HTTP 請求等）
- **Explore**：手動查詢 Prometheus/Loki/Tempo
- **Trace to Logs**：在 Trace 詳情頁面直接跳到對應的 Loki log

Grafana 網址：`http://203.0.113.10:3000`（帳號見 `monitoring/.env`）

### 4.6 Alertmanager（告警路由）

接收來自 Prometheus 的告警，依照 label 決定發給誰、用什麼方式。

目前設定：
- `env=prod` → Alertmanager 立即通知
- `env=dev` → 只記錄，不通知（避免干擾）
- 預設 receiver：blackhole（不符合任何 route 的告警靜默丟棄）

設定檔：`monitoring/alertmanager/config.yml`

---

## 五、一筆請求的完整數據流

以「買家下單」為例：

```
1. 手機 POST /api/v1/app/orders
           │
2. Gateway 收到請求
   OTel SDK 產生 trace_id=abc123，建立 root span
   傳遞 traceparent header 給下游服務
           │
3. order-service 收到 gRPC 呼叫
   OTel SDK 從 metadata 取出 trace_id=abc123
   建立 child span（order-service）
   呼叫 product-service（傳遞 trace_id）
           │
4. product-service 呼叫
   建立 child span（product-service）
   檢查庫存
           │
5. 三個服務的 OTel SDK 同時在 background 推送到 OTel Collector
   
   OTel Collector 分發：
   - Traces → Tempo（完整的 abc123 旅程圖）
   - Metrics → Prometheus（HTTP 請求計數 +1）
   - Logs   → Loki（log 內容帶 trace_id=abc123）

6. 在 Grafana：
   - 看到 metrics 圖表（請求數飆升）
   - 查 Tempo：找到 trace_id=abc123，看到整條請求耗時分佈
   - 點「Logs for this trace」：跳到 Loki，看到這筆請求所有服務的 log
```

---

## 六、Go 服務 vs Java 服務的差異

兩種服務的 OTel SDK 設定方式不同：

| 設定 | Go 服務 | Java 服務 |
|------|---------|-----------|
| 環境識別（Metrics） | `ENV=dev` 環境變數 | `OTEL_RESOURCE_ATTRIBUTES=deployment.environment=dev` |
| OTel endpoint | 程式碼內指定 / 環境變數 | `OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317` |
| 自動 instrumentation | 手動加 middleware | Java Agent 自動注入（不需改程式碼） |

K8s Helm 設定位置：`k8s/values/overlays/{dev,prod}/common.yaml`

```yaml
# 每個環境都需要設定這兩個環境變數
config:
  ENV: "dev"                                   # Go 服務用
  OTEL_RESOURCE_ATTRIBUTES: "deployment.environment=dev"  # Java 服務用
```

---

## 七、環境隔離

兩個環境（dev / prod）共用同一套 Monitoring VM，透過 labels 區分：

**Metrics（Prometheus label）：**
```
env="dev"   → dev 環境的指標
env="prod"  → prod 環境的指標（告警閾值更嚴格）
```

**Traces（Tempo resource attribute）：**
```
deployment.environment="dev"
deployment.environment="prod"
```

在 Grafana 的 Dashboard 裡，可以用下拉選單切換 `env`，只看特定環境的數據。

---

## 八、Prometheus Agent 的必要性

**問題**：Monitoring VM 無法 scrape K3s 裡的 pod，因為 ClusterIP 只在 K3s 內部可見。

**解法**：在每個 App VM 的 K3s 裡部署 Prometheus Agent：

```yaml
# k8s/monitoring/prometheus-agent.yaml
# 功能：在 K3s 內部 scrape pod，再 remote_write 到外部 Prometheus
args:
  - --enable-feature=agent   # Agent 模式：只 scrape，不存儲
  - --config.file=/etc/prometheus/prometheus.yml

# 自動發現帶有 prometheus.io/scrape: "true" annotation 的 pod
kubernetes_sd_configs:
  - role: pod
    namespaces: {names: [hawkr]}
```

每個微服務的 K8s deployment 帶上 annotation：
```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9900"      # HTTP port
  prometheus.io/path: "/metrics"  # Go 服務；Java 服務是 /actuator/prometheus
```

---

## 九、重要設定檔位置

```
backend-payuni/monitoring/
├── docker-compose.yml              # 監控棧容器定義（版本已鎖定）
├── .env                            # 實際密碼設定（不進 git）
├── .env.example                    # 設定範本
├── Makefile                        # make up / down / restart / logs
├── scripts/
│   └── init-tempo-datasource.sh    # 啟動時自動設定 Tempo datasource
├── otel-collector/
│   └── otel-config.yaml            # OTel Collector pipeline 設定
├── prometheus/
│   ├── prometheus.yml              # Prometheus 主設定（file_sd_configs）
│   ├── targets/                    # ← 填 IP 的 JSON 檔（不進 git）
│   │   ├── infra.json              # Infra VM exporter IPs
│   │   └── monitoring.json        # Monitoring VM 自身元件
│   └── rules/
│       ├── service-alerts.yml     # HTTP / Go runtime 告警規則
│       └── business-alerts.yml    # 業務告警（訂單、支付）
├── loki/
│   └── local-config.yaml          # Loki 設定（含 retention）
├── tempo/
│   └── tempo-config.yaml          # Tempo 設定（含 retention）
├── alertmanager/
│   └── config.yml                 # 告警路由
└── grafana/
    └── provisioning/              # 自動載入的 datasource / dashboard

backend-payuni/k8s/
├── monitoring/
│   ├── prometheus-agent.yaml      # Prometheus Agent DaemonSet（K3s 部署）
│   └── promtail-daemonset.yaml    # Promtail DaemonSet（收集 pod log）
└── external-services/
    ├── otel-collector.yaml         # K8s ExternalService → Infra VM
    ├── prometheus.yaml             # K8s ExternalService → Infra VM
    └── loki.yaml                   # K8s ExternalService → Infra VM
```

---

## 十、日常維運 SOP

### 新增微服務時

1. K8s Deployment 加上 annotation（Helm chart 已自動處理）：
   ```yaml
   prometheus.io/scrape: "true"
   prometheus.io/port: "{{ HTTP port }}"
   ```
2. 確認服務有設定 `ENV` 和 `OTEL_RESOURCE_ATTRIBUTES`（`common.yaml` 已統一設定）
3. 不需要修改 Prometheus 設定（kubernetes_sd_configs 自動發現）

### 監控 VM 重啟

```bash
cd ~/monitoring
make up           # 啟動所有元件
```

`make up` 會在 docker compose up 後自動執行 `scripts/init-tempo-datasource.sh` 設定 Grafana datasource。

### 修改 Infra VM scrape 目標

直接修改 `monitoring/prometheus/targets/infra.json`，Prometheus 30 秒內自動載入，不需要重啟。

### 版本升級注意事項

所有 image 版本都鎖定在 `docker-compose.yml`：
```yaml
image: prom/prometheus:v3.9.1
image: grafana/loki:3.6.6
image: grafana/tempo:2.10.1
```

升版前先看 changelog，特別注意：
- OTel Collector：exporter 名稱可能改變（曾有 `loki` exporter 被移除，要改成 `otlphttp/loki`）
- Loki：新版可能要求新設定項（曾遇到 `delete_request_store` 必填）
- Tempo：資料格式有時不向下相容，volume 需要 chown

---

## 十一、常見問題排查

完整排查清單見：`monitoring/TROUBLESHOOTING.md`

### Q：Grafana 看不到任何 metrics

1. 確認服務有帶 `ENV` 環境變數（Go 服務）或 `OTEL_RESOURCE_ATTRIBUTES`（Java 服務）
2. 確認 OTel Collector 有收到資料：`docker logs otel-collector | grep -i error`
3. 確認 Prometheus 能 scrape：`http://203.0.113.10:9091/targets` 看每個 target 的 State

### Q：Grafana 看不到 traces

1. 確認服務 tracing sampling 設定（prod 預設 10%，dev 100%）
2. 確認 OTel Collector 連得到 Tempo：`docker logs otel-collector`
3. 在 Grafana Explore → Tempo 搜尋 `{service.name="your-service"}`

### Q：Grafana 看不到 logs

1. 確認 Promtail DaemonSet 在 K3s 上有跑：`kubectl get pod -n hawkr -l app=promtail`
2. 確認 Loki 能存入：`docker logs loki | grep -i error`
3. 在 Grafana Explore → Loki 搜尋 `{service_name="your-service"}`

---

## 十二、監控數據的保留期限

| 數據類型 | 保留期限 | 儲存位置 |
|---------|---------|---------|
| Metrics | 15 天 | `prometheus_data` Docker volume |
| Logs | 7 天 | `loki_data` Docker volume |
| Traces | 7 天 | `tempo_data` Docker volume |

調整保留期限：
- Metrics：`monitoring/prometheus/prometheus.yml` 的 `--storage.tsdb.retention.time`
- Logs：`monitoring/loki/local-config.yaml` 的 `retention_period`
- Traces：`monitoring/tempo/tempo-config.yaml` 的 `block_retention`
