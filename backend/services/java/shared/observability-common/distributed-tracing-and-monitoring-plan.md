# 分散式追蹤與監控方案

作者: Wu Will
狀態: 草稿
類別: 使用手冊, 技術規格
最後編輯時間: 2026年2月1日 下午7:17

# 概述

本文件定義跨 Java/Go 服務的統一追蹤、日誌、指標收集方案。

---

## 技術選型：LGTM Stack

| 組件 | 用途 | 說明 |
| --- | --- | --- |
| **Grafana** | 統一查詢入口 | Dashboard、Logs、Traces 都在這裡看 |
| **Loki** | 日誌聚合 | 集中 log 池 |
| **Tempo** | 分散式追蹤 | 原生支援 OTLP |
| **Prometheus** | 指標收集 | 服務指標、基礎設施指標 |
| **Alertmanager** | 告警管理 | 告警規則與通知 |

---

## 統一協議 v1（硬規則）

> ⚠️ 以下規則請寫進團隊規範，所有服務必須遵守
> 

### 1. 三個核心 ID

| ID | 說明 | 產生者 |
| --- | --- | --- |
| `trace_id` | 分散式追蹤 ID | OTEL SDK 自動生成/傳遞 |
| `correlation_id` | 業務串接 ID | Gateway 產生，UUIDv4 |
| `request_id` | 入口請求 ID | Gateway 產生，UUIDv4 |

### 2. 入口治理：Go API Gateway

Gateway 負責：

- 若上游沒有 `x-correlation-id` → 產 UUIDv4
- 若上游沒有 `x-request-id` → 產 UUIDv4
- 往下游 gRPC 一律注入這些 header

### 3. gRPC Metadata 協議

所有 gRPC 呼叫 metadata key **全小寫**：

| Key | 必填 | 說明 |
| --- | --- | --- |
| `traceparent` | ✅ | W3C Trace Context |
| `tracestate` | ❌ | 全用 OTEL + Tempo，可不用 |
| `x-correlation-id` | ✅ | 業務串接 |
| `x-request-id` | ✅ | 入口請求 |

**缺失處理（硬規則）：**

| 缺什麼 | 處理方式 |
| --- | --- |
| 缺 `traceparent` | 自己起新 trace（當作 root span） |
| 缺 `x-correlation-id` | 自己產一個 UUIDv4，往下傳 |
| 缺 `x-request-id` | 自己產一個 UUIDv4 |

### 4. RabbitMQ Header 協議

**必填：**

- `traceparent`
- `x-correlation-id`
- `x-event-id`（UUIDv4，事件唯一 ID）

**強烈建議：**

- `x-idempotency-key`（會改狀態的 consumer 必填）
- `x-producer-service`
- `x-schema-version`

**Producer（硬規則）：**

1. 從當前 context 取 `traceparent`，注入 header
2. 從當前 context 取 `correlation_id`，注入 header
3. 產生新的 `event_id` (UUIDv4)，注入 header
4. 記 log（帶上 `mq.exchange` / `mq.routing_key` / `mq.event_id`）

**Consumer（硬規則）：**

1. 從 header 取 `traceparent` → 建立 child span
2. 從 header 取 `correlation_id` → 放進 context
3. 從 header 取 `event_id` → 用於 log 追蹤

**缺 Header 處理：**

| 缺什麼 | 處理方式 |
| --- | --- |
| 缺 `traceparent` | 起新 trace |
| 缺 `x-correlation-id` | 產新的 UUIDv4 |
| 缺 `x-event-id` | 產新的 UUIDv4，但要記 **warning log** |

---

## 統一 Log Schema（必須 JSON）

所有服務 log 只能輸出 JSON（stdout），欄位使用 `snake_case`。

### 應用程序必填欄位

這些欄位由各服務本身輸出，每筆 log 都必須包含：

| 欄位 | 說明 | 範例 |
| --- | --- | --- |
| `ts` | 時間戳 ISO8601+TZ | `2025-01-15T10:30:00+08:00` |
| `level` | 日誌等級 | `DEBUG` / `INFO` / `WARN` / `ERROR` |
| `service` | 服務名稱 | `order-service` |
| `trace_id` | 追蹤 ID | OTEL 產生 |
| `correlation_id` | 業務串接 ID | 從 context 拿 |
| `event` | 事件名稱 | 見下方命名規則 |
| `msg` | 訊息內容 |  |
| `logger` | Logger 名稱 | **Java 必填**，Go 可省略 |
| `span_id` | Span ID | 至少 ERROR 必填，建議全量 |

### Agent 自動附加欄位

這兩個欄位是靜態環境資訊，由 otelcol-agent 透過 `resource` 自動附加，**服務本身不需要寫**：

| 欄位 | 說明 | 設定方式 |
| --- | --- | --- |
| `env` | 環境 (`dev` / `stg` / `prod`) | agent 的 resource attribute |
| `version` | 版本（git sha 或 semver） | CI/CD 寫入環境變數 → agent 讀取附加 |

> 💡 這兩個欄位放在 agent 層處理的好處是：應用程序不需要自己維護環境配置，同一張 VM 上的所有服務也能保證值一致。
> 

### 事件命名規則

格式：`<domain>.<action>.<result>`

範例：

- `order.create.success`
- `payment.authorize.failed`
- `mq.consume.retry`

### 業務上下文欄位（視情況帶入）

當前請求涉及哪個業務資源，就帶進來對應的 ID。這些欄位是 Runbook Case A 能用業務 ID 直接反查全鏈路的基礎。

| 欄位 | 帶入時機 | 範例 |
| --- | --- | --- |
| `account_id` | 涉及帳號操作時 | `USR-67890` |
| `order_id` | 涉及訂單流程時 | `ORD-12345` |
| `payment_id` | 涉及支付流程時 | `PAY-67890` |

命名規則統一用 `snake_case`，格式跟對應的業務 ID 一致。如果之後新增其他核心實體，也按同樣規則補充進來。

> 💡 不需要一筆 log 帶所有業務 ID，只帶當前操作直接相關的就行。比如 order.create.success 帶 order_id，不需要同時帶 account_id（除非這個事件本身確實跟兩個實體都有關）。
> 

### 錯誤欄位（WARN/ERROR 必須）

| 欄位 | 說明 |
| --- | --- |
| `error.type` | 錯誤類型 |
| `error.message` | 錯誤訊息 |
| `error.stack` | Stack trace（prod 可視風險裁剪） |
| `error.code` | 錯誤代碼（建議：可治理、可告警） |

### RabbitMQ log 必帶欄位

| 欄位 | 說明 |
| --- | --- |
| `mq.exchange` | Exchange 名稱 |
| `mq.routing_key` | Routing Key |
| `mq.queue` | Queue 名稱 |
| `mq.event_id` | 事件 ID |
| `mq.retry_count` | 重試次數 |

### gRPC log 必帶欄位

| 欄位 | 說明 |
| --- | --- |
| `grpc.service` | gRPC 服務名稱 |
| `grpc.method` | 方法名稱 |
| `grpc.code` | 狀態碼 |
| `grpc.latency_ms` | 延遲（毫秒） |

---

## 部署架構

### Observability VM（集中平台）

使用 Docker Compose 部署：

- Grafana
- Loki
- Tempo
- Prometheus
- Alertmanager
- Postgres Exporter
- Redis Exporter
- RabbitMQ Exporter

### 服務 VM（Gateway / 各微服務）

每台服務 VM 部署：

| 組件 | 說明 |
| --- | --- |
| **otelcol-agent** | 統一收 traces + logs + metrics |
| **服務本體** | Java / Go |

---

## 設定檔範例

### 服務 VM：OTEL Collector Agent

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

  filelog:
    include:
      - /var/log/myservice/*.log
    operators:
      - type: json_parser

processors:
  batch:
    timeout: 5s
    send_batch_size: 1000

  memory_limiter:
    check_interval: 1s
    limit_mib: 1000
    spike_limit_mib: 200

exporters:
  otlp/tempo:
    endpoint: observability-vm:4317
    tls:
      insecure: true

  loki:
    endpoint: http://observability-vm:3100/loki/api/v1/push

  prometheusremotewrite:
    endpoint: http://observability-vm:9090/api/v1/write

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [otlp/tempo]

    logs:
      receivers: [otlp, filelog]
      processors: [memory_limiter, batch]
      exporters: [loki]

    metrics:
      receivers: [otlp]
      processors: [memory_limiter, batch]
      exporters: [prometheusremotewrite]

```

### Grafana Datasource 設定

```yaml
apiVersion: 1
datasources:
  - name: Loki
    type: loki
    url: http://loki:3100
    jsonData:
      derivedFields:
        - name: trace_id
          matcherRegex: '"trace_id":"([^"]+)"'
          url: '$${__value.raw}'
          datasourceUid: tempo

  - name: Tempo
    type: tempo
    url: http://tempo:3200
    uid: tempo

  - name: Prometheus
    type: prometheus
    url: http://prometheus:9090

```

> 💡 關鍵是 derivedFields 設定，讓 Loki log 可以直接連到 Tempo trace。
> 

---

## Runbook：如何查問題

### 統一入口：Grafana

所有查詢都在 Grafana 完成：

- **Explore → Loki**：查 log
- **Explore → Tempo**：查 trace
- **Explore → Prometheus**：查 metrics

### Case A：用業務 ID 查（最常用）

1. 你有 `order_id` 或 `payment_id`
2. 在 Grafana Explore 選 Loki，查詢：

```
{service="order-service"} | json | order_id="ORD-12345"

```

1. 從命中的 log 拿到 `correlation_id`
2. 用 `correlation_id` 再查一次（看到全鏈路 log）：

```
{service=~".+"} | json | correlation_id="abc-123-def"

```

1. 在 log 裡看到 `trace_id` → 點擊跳到 Tempo 看 span 細節

### Case B：MQ 卡住 / 重試爆炸

1. 查重試事件：

```
{service=~".+"} | json | event="mq.consume.retry"

```

1. 或查死信：

```
{service=~".+"} | json | event="mq.consume.deadletter"

```

1. 看 `mq.event_id`、`mq.retry_count`、`mq.queue`
2. 用 `trace_id` 去 Tempo 看是 producer 端慢，還是 consumer 端卡住

### Case C：gRPC 呼叫失敗

1. 查 gRPC 錯誤：

```
{service="api-gateway"} | json | grpc.code!="OK"

```

1. 看 `grpc.service`、`grpc.method`、`grpc.latency_ms`
2. 用 `trace_id` 去 Tempo 看完整呼叫鏈

---

## 各服務必做事項

### Go API Gateway

- [ ]  全域 middleware：產生/傳遞 `correlation_id`、`request_id`
- [ ]  gRPC client interceptor：注入 metadata
- [ ]  Log 輸出 JSON，符合 schema

### Java / Go 微服務

- [ ]  gRPC server interceptor：解析 metadata → 建 span → 把 `correlation_id` 放進 context
- [ ]  gRPC client interceptor：從 context 注入 metadata
- [ ]  RabbitMQ publish wrapper：注入 `traceparent` + `correlation_id` + `event_id`
- [ ]  RabbitMQ consume wrapper：從 header 還原 parent → consumer span
- [ ]  Logger：輸出 JSON，自動帶 `trace_id` / `span_id` / `correlation_id`

> ⚠️ 關鍵：不要讓工程師各自手寫 header/metadata，一定要封裝成共用套件（每語言一個 internal lib）。
> 

---

## Port 對照表

| 服務 | Port | 用途 |
| --- | --- | --- |
| Grafana | 3000 | Dashboard UI |
| Loki | 3100 | Log 收集 |
| Tempo | 3200 | Trace 查詢 (HTTP) |
| Tempo | 4317 | OTLP gRPC 接收 |
| Prometheus | 9090 | Metrics |
| Alertmanager | 9093 | 告警管理 |
| OTEL Collector | 4317 | OTLP gRPC |
| OTEL Collector | 4318 | OTLP HTTP |