Java Internal Observability Library

設計書 v1.0（Hard Spec）

⸻

1. 目的與範圍

本文件定義 Java 微服務共用的 Internal Library，用於實作：
	•	分散式追蹤（OpenTelemetry / W3C Trace Context）
	•	業務關聯（correlation_id）
	•	RabbitMQ / gRPC 上下游追蹤串接
	•	統一 JSON Log Schema

本 Library 不包含任何業務語意，不解析 payload，不推導事件名。

⸻

2. 核心識別規則（Hard Rules）

2.1 correlation_id
	•	用途：跨服務、跨 transport 的業務關聯鍵
	•	格式：UUID v4（lowercase）
	•	規則：
	•	inbound 有 x-correlation-id → 使用
	•	inbound 無 → 立即產生 UUIDv4

2.2 request_id
	•	用途：單一入口請求追蹤
	•	格式：UUID v4（建議）
	•	規則：
	•	inbound 有 x-request-id → 使用
	•	inbound 無 → v1.0 不自動生成

2.3 trace_id / span_id
	•	使用 W3C traceparent
	•	有則續接，無則建立 root span

⸻

3. Header / Metadata 規範（全小寫）

3.1 gRPC Metadata
	•	traceparent
	•	x-correlation-id
	•	x-request-id

3.2 RabbitMQ Headers
	•	traceparent
	•	x-correlation-id
	•	x-request-id（optional）
	•	event（必填）

⸻

4. 模組設計

4.1 ObsContext

負責管理：
	•	correlation_id
	•	request_id
	•	trace_id / span_id
	•	event
	•	service

所有值需同步寫入 MDC：
	•	trace_id（若 SpanContext valid）
	•	span_id（若 SpanContext valid）
	•	correlation_id
	•	request_id
	•	event
	•	service

注意：`env` 已移至 otelcol-agent 層注入，lib 不再設置。

⸻

4.2 gRPC Server Interceptor

功能：
	1.	解析 inbound metadata
	2.	建立 server span（或 root span）
	3.	建立 ObsContext scope
	4.	同步 MDC
	5.	請求結束後清理

⸻

4.3 gRPC Client Interceptor

功能：
	1.	從 ObsContext 取得資料
	2.	注入 outbound metadata：
	•	traceparent
	•	x-correlation-id
	•	x-request-id（若存在）

⸻

4.4 gRPC Channel / Stub Factory（集中模式）

硬性規則
	•	禁止業務端 new channel
	•	禁止業務端 new stub
	•	所有 gRPC 呼叫 只能透過 internal lib

Target 定義（v1.0）
	•	target：media
	•	host：media-service.internal
	•	port：9937
	•	plaintext：true

⸻

4.5 RabbitMQ Publish Wrapper

功能：
	1.	強制要求 event
	2.	從 ObsContext 取：
	•	correlation_id
	•	request_id
	3.	注入 headers：
	•	traceparent
	•	x-correlation-id
	•	x-request-id（optional）
	•	event

⸻

4.6 RabbitMQ Consume Wrapper

功能：
	1.	從 headers 還原 traceparent → child span
	2.	還原 correlation_id / request_id
	3.	建立 ObsContext + MDC
	4.	執行 listener
	5.	清理 context

⸻

5. Spring Boot AutoConfiguration

Library 必須自動註冊：
	•	ObsContext
	•	gRPC Server Interceptor（使用 @GrpcGlobalServerInterceptor 全域註冊）
	•	gRPC Client Interceptor
	•	GrpcChannelRegistry
	•	GrpcStubFactory
	•	RabbitMQ Publisher / Consumer Advice（若 classpath 存在）

技術依賴：
	•	net.devh:grpc-spring-boot-starter 3.x（提供 @GrpcGlobalServerInterceptor 註解）

⸻

6. 設定檔（application.yml）

hawkr:
  obs:
    service: product-service
    # env: prod  # 已棄用，由 otelcol-agent 注入
    grpc:
      targets:
        media:
          host: media-service.internal
          port: 9937
          plaintext: true


⸻

7. 使用方式（給其他服務工程師）

7.1 專案設定

Step 1: 加入依賴（pom.xml）

```xml
<!-- Observability Common -->
<dependency>
    <groupId>io.github.will413028.hawkr.common</groupId>
    <artifactId>observability-common</artifactId>
    <version>1.0.0</version>
</dependency>

<!-- OpenTelemetry Spring Boot Starter（必要，啟用 trace_id/span_id）-->
<dependency>
    <groupId>io.opentelemetry.instrumentation</groupId>
    <artifactId>opentelemetry-spring-boot-starter</artifactId>
</dependency>
```

在 `<dependencyManagement>` 中加入 BOM：

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

Step 2: 設定 application.yml

```yaml
hawkr:
  obs:
    service: user-service          # 你的服務名稱（必填）
    access-log:
      ok-sample-rate: 1.0          # 開發環境設 1.0，生產環境設 0.0~0.1
      slow-threshold-ms: 500
    grpc:
      targets:                     # 若需呼叫其他 gRPC 服務
        media:
          host: media-service
          port: 9937
          plaintext: true

# OpenTelemetry 設定
otel:
  sdk:
    disabled: false
  service:
    name: ${hawkr.obs.service}
  traces:
    exporter: none                 # 本地開發用 none，生產用 otlp
  # 生產環境配置：
  # exporter:
  #   otlp:
  #     endpoint: http://otel-collector:4317
  #     protocol: grpc
```

Step 3: 設定 logback-spring.xml

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <include resource="logback-obs-base.xml"/>
</configuration>
```

Step 4: 設定 lombok.config（專案根目錄）

```
# lombok.config
lombok.log.custom.declaration = io.github.will413028.hawkr.common.obs.log.ObsLogger io.github.will413028.hawkr.common.obs.log.ObsLogger.forClass(TYPE)
```

⸻

7.2 gRPC Server

不需任何程式碼，interceptor 由 lib 全域註冊。
所有 gRPC 請求自動建立 span、自動傳遞 correlation_id。

⸻

7.3 gRPC Client

禁止 new channel / new stub，只能使用 GrpcStubFactory。
呼叫時會自動帶上 traceparent、x-correlation-id、x-request-id。

@Service
public class ProductService {

    @Autowired
    private GrpcStubFactory grpcStubFactory;

    public void uploadImage() {
        // 自動帶上所有追蹤資訊給 media-service
        MediaGrpc.MediaBlockingStub stub =
            grpcStubFactory.blocking("media", MediaGrpc::newBlockingStub);
        stub.upload(request);
    }
}

⸻

7.4 RabbitMQ

發送：使用 ObsRabbitPublisher（自動注入 headers）

@Service
public class OrderService {

    @Autowired
    private ObsRabbitPublisher publisher;

    public void createOrder(byte[] orderData) {
        // event 必填，會自動帶上 traceparent、correlation_id
        publisher.publish("order.exchange", "order.created", orderData, "order.create.success");
    }
}

消費：@RabbitListener 不需改 method signature

@Component
public class OrderConsumer {

    @RabbitListener(queues = "order.queue")
    public void handleOrder(Message message) {
        // 直接寫業務邏輯
        // trace/correlation/span 已由 ObsRabbitListenerAdvice 自動處理
    }
}

⸻

7.5 業務日誌（ObsLogger）

使用 @CustomLog（需先設定 lombok.config）：

@CustomLog
public class UserGrpcService {

    public void login(String accountId) {
        // event = "UserGrpcService.login"
        log.info("", "Start login");

        // event = "UserGrpcService.login.success"，帶 account_id
        log.info("success", "Login completed", "account_id", accountId);
    }

    public void register(String email) {
        try {
            // 業務邏輯...
            log.info("success", "User registered", "email", email);
        } catch (Exception e) {
            // event = "UserGrpcService.register.failed"，帶 exception
            log.error("failed", "Registration failed", "email", email, e);
        }
    }
}

不使用 Lombok 時，手動宣告：

public class UserGrpcService {
    private static final ObsLogger log = ObsLogger.forClass(UserGrpcService.class);
    // ...
}

⸻

7.6 自動獲得的功能（零程式碼）

- ✅ gRPC Server 自動追蹤（interceptor 自動註冊）
- ✅ trace_id / span_id / correlation_id 自動傳遞
- ✅ Access Log（錯誤/慢請求自動記錄）
- ✅ MQ 消費自動建立 span 與 context

⸻

8. 禁止事項（必檢查）
	•	❌ 手寫 gRPC metadata
	•	❌ 手寫 RabbitMQ headers
	•	❌ new ManagedChannel / NettyChannelBuilder
	•	❌ new gRPC stub

⸻

9. 驗收標準
	•	任一 log 可用 correlation_id 查出完整鏈路
	•	任一 MQ 消息可回溯 producer / consumer span
	•	任一 gRPC 呼叫可在 Tempo 中看到完整 parent-child 關係

⸻

10. gRPC Server Guard / Access Log（v1.1 新增，v1.2 更新）

設計目標：
	•	即使業務層不寫任何 log，也能留下「被呼叫過」的痕跡
	•	任何 exception 都能被觀測到，但不影響 gRPC 正常回應行為
	•	不產生爆量 log

Access Log 觸發規則：
	•	ERROR（Status != OK）：100% 印
	•	SLOW（OK 但超過閾值）：100% 印
	•	OK（正常）：依設定抽樣

Access Log 格式（v1.2 更新）：
	•	Event 格式：`<grpcService>.<grpcMethod>.<end|error>`
	•	ERROR：msg = exception.message, stack_trace = exception.stackTrace
	•	SLOW/OK：msg = ""（空字串）

配置：

hawkr:
  obs:
    access-log:
      ok-sample-rate: 0.0      # 預設不印 OK
      slow-threshold-ms: 1000  # 慢請求閾值

Interceptor 核心原則（Hard Rule）：
	•	只觀測，不改寫回應
	•	絕不吞掉 exception
	•	Guard + Recorder，不是 ErrorHandler

MDC 命名規範（Hard Rule）：
	•	僅允許 snake_case：trace_id, span_id
	•	禁止 camelCase：traceId, spanId（interceptor 會自動移除）
	•	SpanContext 無效時不輸出 trace_id/span_id（禁止全 0）

v1.1 驗收標準：
	•	不寫任何 log.info，gRPC error 仍能在 Loki 查到
	•	OK 呼叫不會產生大量 access log
	•	interceptor 不影響原本 gRPC 回應行為
	•	log 輸出不得有 camelCase 欄位（traceId, spanId）
	•	log 輸出不得有全 0 的 trace_id/span_id

⸻

11. ObsLogger 業務日誌工具（v1.2 新增，v1.3 更新）

業務層結構化日誌工具，className 與 methodName 自動偵測加入 event。

Event 格式：`<ClassName>.<methodName>.<result>`
- ClassName：由 forClass() 自動取得（使用 SimpleClassName）
- methodName：由 stack trace 自動偵測（若不在 method 內則為空）
- result：使用者傳入（可為空字串）

使用範例：

private static final ObsLogger log = ObsLogger.forClass(UserGrpcService.class);

public void login() {
    // event = "UserGrpcService.login.success"
    log.info("success", "Login completed");

    // event = "UserGrpcService.login"（result 為空）
    log.info("", "Processing");

    // 帶業務上下文
    // event = "UserGrpcService.login.success"
    log.info("success", "Login completed", "account_id", "USR-12345");

    // 錯誤日誌
    // event = "UserGrpcService.login.failed"
    log.error("failed", "Login failed", exception);
}

API 說明：
- result：可為空字串（event 不含 result 部分）
- msg：必填（可為空字串）
- bzCol/bzValue：選填業務上下文欄位

Lombok 整合：
lombok.log.custom.declaration = io.github.will413028.hawkr.common.obs.log.ObsLogger io.github.will413028.hawkr.common.obs.log.ObsLogger.forClass(TYPE)
使用 @CustomLog 即可

已移除：ObsEvent.withEvent()

⸻