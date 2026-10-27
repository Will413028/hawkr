# Java Internal Observability Library

Java 微服務共用的 Observability / 分散式追蹤 / 關聯式 Logging 內部 Library。

本 Library 用於統一處理：

- gRPC 呼叫的 trace / correlation 傳遞  
- RabbitMQ message 的 trace / correlation 傳遞  
- JSON log 所需的 context（trace_id / correlation_id / event 等）

本 Library 為 infra 層元件，不包含任何業務邏輯。

---

## 1. 解決什麼問題？

未使用本 Library 時，常見問題如下：

- gRPC / MQ 呼叫斷 trace  
- log 無法查出上下游關係  
- correlation_id 各服務各自產生，無法串接  
- 工程師手寫 header，規則不一致  

使用本 Library 後，保證：

- 每一筆請求都有 correlation_id  
- gRPC / MQ trace 不會中斷  
- log 可透過 correlation_id 查出完整鏈路  
- 工程師不需要、也不能自行處理 header / metadata  

---

## 2. 本 Library 的職責範圍（Scope）

### 2.1 會做的事

- gRPC Server Interceptor  
- gRPC Client Interceptor  
- gRPC Channel / Stub 集中管理（禁止各自 new）  
- RabbitMQ publish / consume header 注入與還原  
- ObsContext + MDC 管理（trace / correlation / event）  
- Spring Boot Auto Configuration（零侵入）

### 2.2 不會做的事

- 不解析 payload  
- 不推導 event 名稱  
- 不處理 HTTP Gateway  
- 不負責 metrics / exporter / collector 部署  
- 不做任何業務判斷  

---

## 3. 核心識別欄位（Hard Rules）

### 3.1 correlation_id（必定存在）

- UUID v4（lowercase）
- inbound 有 → 使用
- inbound 無 → 自動產生

### 3.2 request_id（可能存在）

- inbound 有 → 使用
- inbound 無 → v1.0 不自動產生

### 3.3 trace_id / span_id

- 使用 W3C traceparent
- 有則續接，無則建立 root span

---

## 4. Header / Metadata 規範（禁止自行處理）

### 4.1 gRPC Metadata（全小寫）

- traceparent  
- x-correlation-id  
- x-request-id  

### 4.2 RabbitMQ Headers（全小寫）

- traceparent  
- x-correlation-id  
- x-request-id  
- event  

---

## 5. 如何在服務中使用

### 5.1 加入依賴

```xml
<!-- Observability Common -->
<dependency>
  <groupId>io.github.will413028.hawkr</groupId>
  <artifactId>observability-common</artifactId>
  <version>1.0.0</version>
</dependency>

<!-- OpenTelemetry Spring Boot Starter（必要，啟用 trace_id/span_id）-->
<dependency>
  <groupId>io.opentelemetry.instrumentation</groupId>
  <artifactId>opentelemetry-spring-boot-starter</artifactId>
</dependency>
```

**在 `<dependencyManagement>` 中加入 BOM：**

```xml
<dependencyManagement>
  <dependencies>
    <dependency>
      <groupId>io.opentelemetry.instrumentation</groupId>
      <artifactId>opentelemetry-instrumentation-bom</artifactId>
      <version>2.10.0</version>
      <type>pom</type>
      <scope>import</scope>
    </dependency>
  </dependencies>
</dependencyManagement>
```

**前置依賴（微服務必須已引入）：**

```xml
<!-- gRPC Spring Boot Starter - 用於全域 interceptor 註冊 -->
<dependency>
  <groupId>net.devh</groupId>
  <artifactId>grpc-spring-boot-starter</artifactId>
  <version>3.0.0.RELEASE</version>
</dependency>
```

---

### 5.2 application.yml（必填，放在各微服務）

> **注意**：此配置需放在各微服務的 `src/main/resources/application.yml`，不是內部 lib。

```yaml
hawkr:
  obs:
    service: product-service
    grpc:
      targets:
        media:
          host: media-service.internal
          port: 9937
          plaintext: true
    access-log:
      ok-sample-rate: 1.0      # 開發環境設 1.0，生產環境設 0.0~0.1
      slow-threshold-ms: 1000  # 慢請求閾值（毫秒）

# OpenTelemetry 設定（必要，啟用 trace_id/span_id）
otel:
  sdk:
    disabled: false
  service:
    name: ${hawkr.obs.service}
  traces:
    exporter: none             # 本地開發用 none，生產用 otlp
  # 生產環境配置：
  # exporter:
  #   otlp:
  #     endpoint: http://otel-collector:4317
  #     protocol: grpc
```

---

### 5.2.1 Access Log 配置說明（v1.1 新增）

| 參數 | 預設值 | 說明 |
|------|--------|------|
| ok-sample-rate | 0.0 | OK 請求抽樣比例（0.0-1.0），0.01=1% |
| slow-threshold-ms | 1000 | 超過此閾值視為 SLOW，固定印 WARN |

**環境建議配置：**

| 環境 | ok-sample-rate | slow-threshold-ms |
|------|----------------|-------------------|
| DEV | 0.0 | 1000 |
| PRODUCTION | 0.001 | 1000 |

**Access Log 觸發規則（Hard Rule）：**
- ERROR（Status != OK）：100% 印 ERROR log
- SLOW（OK 但超過閾值）：100% 印 WARN log
- OK（正常）：依 ok-sample-rate 抽樣印 INFO log

**Access Log 格式（v1.2 更新）：**
- Event 格式：`<grpcService>.<grpcMethod>.<end|error>`
- ERROR：msg = exception.message, stack_trace = exception.stackTrace
- SLOW/OK：msg = ""（空字串）

⸻

5.2.2 Configuration Examples

核心原則（Hard Rule）：

grpc.targets 只存在於「主動呼叫他人」的服務
	•	會呼叫別人 → 必須設定
	•	只被呼叫 → 不可設定

⸻

Scenario 1：Product ⇄ Media（雙向 gRPC）

# product-service
hawkr:
  obs:
    service: product-service
    grpc:
      targets:
        media:
          host: media-service.internal
          port: 9937
          plaintext: true

# media-service
hawkr:
  obs:
    service: media-service
    grpc:
      targets:
        product:
          host: product-service.internal
          port: 9921
          plaintext: true


⸻

Scenario 2：Product → Media（單向呼叫）

# product-service
hawkr:
  obs:
    service: product-service
    grpc:
      targets:
        media:
          host: media-service.internal
          port: 9937
          plaintext: true

# media-service
hawkr:
  obs:
    service: media-service


⸻

Scenario 3：Media Only Provider（純被呼叫）

hawkr:
  obs:
    service: media-service


⸻

Misconfiguration Handling

若服務未設定 grpc.targets 卻嘗試呼叫他人：

GrpcStubFactory.blocking("product", ...)

必定行為：
	•	直接丟出 IllegalArgumentException
	•	錯誤訊息必須包含：
	•	service name
	•	targetName
	•	明確指出 grpc.targets 未設定

⸻

Naming Convention（強制）
	•	service：canonical service name
	•	product-service, media-service
	•	targetName：被呼叫服務簡稱
	•	product, media, payment

禁止：
	•	mediaService
	•	media_svc
	•	ms

⸻

6. gRPC Server

**全域註冊機制：**
- 使用 `@GrpcGlobalServerInterceptor` 註解（來自 net.devh:grpc-spring-boot-starter）
- Library 自動將 ObsGrpcServerInterceptor 掛載到所有 gRPC 服務端點
- 服務工程師不需要撰寫任何程式碼

**每一個 gRPC request 會自動：**
- 解析 `traceparent` → 建立 server span
- 繼承 `x-correlation-id`（若無則自動產生 UUID v4）
- 繼承 `x-request-id`（若有）
- 同步 MDC（trace_id, span_id, correlation_id, request_id, service, env）

**MDC 命名規範（Hard Rule）：**
- 僅允許 snake_case：`trace_id`, `span_id`
- 禁止 camelCase：`traceId`, `spanId`
- interceptor 會自動移除其他框架設置的 camelCase 欄位

**Invalid SpanContext 處理（Hard Rule）：**
- `SpanContext.isValid() == false` 時，不輸出 `trace_id` / `span_id`
- 禁止輸出全 0 值（會造成 Tempo 無法查詢）

⸻

7. gRPC Client（Product → Media）

禁止寫法：

ManagedChannel channel = ManagedChannelBuilder
    .forAddress("media-service.internal", 9937)
    .usePlaintext()
    .build();

MediaGrpc.MediaBlockingStub stub =
    MediaGrpc.newBlockingStub(channel);

唯一允許寫法：

MediaGrpc.MediaBlockingStub stub =
    grpcStubFactory.blocking("media", MediaGrpc::newBlockingStub);

stub.finalizeImages(request);


⸻

8. RabbitMQ Producer

禁止：
	•	手寫 headers
	•	直接使用 RabbitTemplate

正確方式：

obsRabbitPublisher.publish(
  "order.exchange",
  "order.created",
  payloadBytes,
  "order.create.success"
);


⸻

9. RabbitMQ Consumer

@RabbitListener(queues = "order.created.queue")
public void handle(String body, Channel channel, long tag) {
  log.info("processing order created");
}


⸻

10. ObsLogger 業務日誌工具

業務層結構化日誌工具，className 自動加入 event。

```java
// 方式 1：手動宣告
private static final ObsLogger log = ObsLogger.forClass(UserGrpcService.class);

// 方式 2：使用 Lombok @CustomLog（需設定 lombok.config）
// lombok.log.custom.declaration = io.github.will413028.hawkr.common.obs.log.ObsLogger io.github.will413028.hawkr.common.obs.log.ObsLogger.forClass(TYPE)
```

使用範例：

```java
// event = "UserGrpcService.login.success"
log.info("login.success", "Login completed");

// event = "UserGrpcService.login"（只有 action，沒有 result）
log.info("login", "Processing login");

// 帶業務上下文
log.info("login.success", "Login completed", "account_id", "USR-12345");

// 錯誤日誌
log.error("login.failed", "Login failed", exception);
```

API：
	•	`info(actionResult, msg)` / `info(actionResult, msg, bzCol, bzValue)`
	•	`warn(actionResult, msg)` / `warn(actionResult, msg, bzCol, bzValue)`
	•	`error(actionResult, msg)` / `error(actionResult, msg, Throwable)`
	•	`error(actionResult, msg, bzCol, bzValue)` / `error(actionResult, msg, bzCol, bzValue, Throwable)`

Event 格式：`<ClassName>.<action>.<result>` 或 `<ClassName>.<action>`

⸻

11. 禁止事項（Hard Stop）
	•	禁止手寫 gRPC metadata
	•	禁止手寫 RabbitMQ headers
	•	禁止 new ManagedChannel / NettyChannelBuilder
	•	禁止 new gRPC Stub
	•	禁止在業務層操作 trace / correlation

⸻

12. 驗收標準
	•	任一請求可用 correlation_id 串起完整 log
	•	Tempo 可看到完整 span chain
	•	MQ producer / consumer 位於同一 trace
	•	gRPC client → server trace 不斷

⸻

13. VSCode 檢查用正則（必掃）

\b(ManagedChannelBuilder|NettyChannelBuilder|OkHttpChannelBuilder)\b
\bnewBlockingStub\b|\bnewFutureStub\b|\bnewStub\b


⸻

14. Logging 與 Loki 整合規範

14.1 責任切分（Hard Rule）
	•	internal observability lib：
	•	僅負責將 trace / correlation / event 注入 MDC
	•	不負責 log 輸出位置
	•	不設定 Loki endpoint
	•	各微服務：
	•	統一輸出 log 至 stdout
	•	由 Loki 基礎設施收集

⸻

14.2 logback 設定放置位置（固定）

每一個使用本 Library 的微服務必須自行建立：

src/main/resources/logback-spring.xml


⸻

14.3 最小標準 logback 範例（JSON + MDC）- v1.3 更新

**推薦方式：使用共用 logback 配置**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <include resource="logback-obs-base.xml"/>
</configuration>
```

`logback-obs-base.xml` 由 observability-common 提供，已包含：
- 統一欄位命名：`ts`, `level`, `msg`, `logger`, `stack_trace`
- MDC 欄位輸出：`service`, `trace_id`, `span_id`, `correlation_id`, `event`
- 已移除：`thread`（不在統一 schema 內）
- 已抑制 noisy logger：`io.grpc.internal`, `io.netty`, `io.opentelemetry`

**自訂 logback 配置（如需覆蓋預設值）：**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <include resource="logback-obs-base.xml"/>
    <!-- 自訂設定 -->
    <logger name="io.github.will413028.hawkr" level="DEBUG"/>
</configuration>
```


⸻

14.4 Loki 收集說明
	•	log 一律輸出至 stdout
	•	Loki / Promtail / Agent 負責收集
	•	微服務無需設定 Loki endpoint

⸻

14.5 核心結論

observability internal lib 只保證 MDC 正確
logging pipeline 與 Loki 收集完全由 infra 處理

