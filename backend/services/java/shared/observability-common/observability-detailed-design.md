以下是 AI 實作版（Claude）《observability-common 內部 Library 詳細設計書 v1.0》：
特性：每個 public method 都有 pseudo-code、行為固定、幾乎不能偏離。
（工程師/AI 僅需依本文實作；除非本文明確允許，否則不得自行補邏輯或新增功能。）

⸻

observability-common

Implementation Hard Spec v1.0 (AI build / Claude)

0. 目的

建立 Java 內部共用 Library，統一處理：
	•	gRPC (server+client) 的 W3C traceparent 續接與傳遞
	•	x-correlation-id / x-request-id 續接與傳遞
	•	RabbitMQ publish/consume 的 headers 注入與還原
	•	統一 MDC 欄位（供 JSON logger 輸出）

v1.0 禁止事項：不得解析 payload、不得推導 event 名稱、不得提供 HTTP gateway/filter、不得做 metrics/exporter 部署。

⸻

1. 技術前提（固定）
	•	Java 21
	•	Spring Boot 3.x
	•	gRPC Java（io.grpc）
	•	gRPC Spring Boot Starter（net.devh:grpc-spring-boot-starter 3.x）
	•	RabbitMQ：Spring AMQP (org.springframework.amqp.*) + com.rabbitmq.client.Channel
	•	Logging：SLF4J + MDC（org.slf4j.MDC）
	•	OpenTelemetry：API + Context propagation（W3C Trace Context）

⸻

2. 統一協議（Hard Rules）

2.1 IDs 與格式
	•	correlation_id：UUIDv4 lowercase，必定存在
	•	request_id：UUIDv4（建議），允許缺失（v1.0 不自產）
	•	trace_id/span_id：由 OpenTelemetry span 提供

2.2 Key 命名（全小寫，固定）

gRPC metadata keys
	•	traceparent
	•	x-correlation-id
	•	x-request-id

RabbitMQ header keys
	•	traceparent
	•	x-correlation-id
	•	x-request-id
	•	event

2.3 缺失處理（固定）
	•	缺 traceparent：建立 root span（不丟錯）
	•	缺 x-correlation-id：立即產 UUIDv4
	•	缺 x-request-id：維持缺失（不產生）

⸻

3. 專案結構（固定）

Package root：io.github.will413028.hawkr.common.obs

io.github.will413028.hawkr.common.obs
├── config
│   ├── ObsProperties.java
│   ├── ObsAutoConfiguration.java
│   └── GrpcTargetsProperties.java
├── context
│   ├── ObsContext.java
│   ├── ObsScope.java
│   └── ObsKeys.java
├── grpc
│   ├── ObsGrpcServerInterceptor.java
│   ├── ObsGrpcClientInterceptor.java
│   ├── GrpcChannelRegistry.java
│   └── GrpcStubFactory.java
├── log
│   └── ObsLogger.java
├── mq
│   ├── ObsRabbitPublisher.java
│   └── ObsRabbitListenerAdvice.java
└── util
    ├── IdGenerator.java
    └── Preconditions.java

允許新增 internal private helper class，但不得新增 public API。

⸻

4. 配置（固定）

4.1 application.yml keys

hawkr:
  obs:
    service: <this-service-name>   # 必填
    env: <env-name>                # 已棄用，由 otelcol-agent 注入
    grpc:
      targets:                     # optional（若此服務不作為 gRPC client）
        <targetName>:
          host: <dns-or-service-name>
          port: <int>
          plaintext: <true|false>

4.1.1 application.yml keys（範例）
hawkr:
  obs:
    service: product-service
    env: prod
    grpc:
      targets:
        media:
          host: media-service
          port: 9937
          plaintext: true

4.2 ObsProperties（固定 fields）
	•	service: String（必填）
	•	env: String（已棄用，由 otelcol-agent 注入）
	•	grpc.targets: Map<String, GrpcTarget>（若此服務作為 gRPC client）

GrpcTarget fields：
	•	host: String（必填）
	•	port: int（必填）
	•	plaintext: boolean（必填）

⸻

5. 核心：ObsKeys（固定）

5.1 MDC keys（固定字串）
	•	trace_id
	•	span_id
	•	correlation_id
	•	request_id
	•	event
	•	service

注意：`env` 已移除，由 otelcol-agent 在 infrastructure 層注入。

5.2 transport keys（固定字串）
	•	gRPC: traceparent, x-correlation-id, x-request-id
	•	MQ : traceparent, x-correlation-id, x-request-id, event

⸻

6. 核心：ObsContext / Scope（必實作且行為固定）

6.1 ObsContext（對外 public API 固定）

public final class ObsContext {
  public ObsScope open(ObsCarrier carrier);
  public ObsScope withEvent(String event);

  public String getCorrelationIdOrNull();
  public String getRequestIdOrNull();
  public String getEventOrNull();
}

ObsCarrier（固定）

public record ObsCarrier(
  String correlationId,
  String requestId,
  String traceparent
) {}

traceparent 在 carrier 內僅作資料傳遞，不做解析；解析由 OTel propagator/Interceptor 負責。

6.2 實作要求（固定）
	•	使用 ThreadLocal<ContextState> 保存：
	•	correlationId
	•	requestId
	•	event
	•	open() 建立 scope，close() 必須恢復上一層狀態（支援巢狀 scope）
	•	每次 open()/withEvent() 都必須同步 MDC（寫入/更新）
	•	每次 close() 必須回復 MDC 到上一層狀態（或移除）

6.3 pseudo-code：ObsContext.open

method open(carrier):
  prevState = threadLocal.get()  // may be null
  nextCorrelation = carrier.correlationId
  if nextCorrelation is null or blank:
      nextCorrelation = IdGenerator.uuidV4Lower()

  nextRequest = carrier.requestId
  if nextRequest is blank:
      nextRequest = null  // v1.0 do not generate

  nextEvent = (prevState != null) ? prevState.event : null

  nextState = new State(
      correlationId = nextCorrelation,
      requestId = nextRequest,
      event = nextEvent
  )

  threadLocal.set(nextState)

  // MDC sync (service/env handled elsewhere but should exist)
  MDC.put("correlation_id", nextCorrelation)
  if nextRequest != null: MDC.put("request_id", nextRequest) else MDC.remove("request_id")
  if nextEvent != null: MDC.put("event", nextEvent) else MDC.remove("event")

  return new ObsScope(closeAction = restore prevState)

6.4 pseudo-code：ObsContext.withEvent

method withEvent(event):
  require event not blank
  prevState = threadLocal.get()  // may be null
  if prevState is null:
      // MUST NOT create correlation_id here; only open() creates it.
      nextState = new State(correlationId=null, requestId=null, event=event)
  else:
      nextState = prevState.copy(event=event)

  threadLocal.set(nextState)

  MDC.put("event", event)

  return new ObsScope(closeAction = restore prevState)

注意：withEvent() 不負責產生 correlation_id。
correlation_id 一律由 inbound open() 或 interceptors 建立。

6.5 ObsScope（固定）

public interface ObsScope extends AutoCloseable {
  @Override void close();
}

pseudo-code:

close():
  restore threadLocal to prevState (may be null)
  update MDC to match restored state


⸻

7. gRPC Server Interceptor（必實作、行為固定）

7.1 類別

ObsGrpcServerInterceptor implements io.grpc.ServerInterceptor

7.2 行為（固定）
	•	從 metadata 讀取：
	•	traceparent
	•	x-correlation-id
	•	x-request-id
	•	建立 span：
	•	使用 OpenTelemetry propagator extractor 從 traceparent 建 parent context
	•	在 server side 建 span name：grpc.server/<fullMethodName>
	•	開啟 ObsContext scope（open）
	•	同步 MDC：
	•	除 ObsContext keys 外，必須補：trace_id, span_id, service
	•	注意：env 由 otelcol-agent 注入，lib 不再設置
	•	在 call 結束（onClose）時：
	•	span end
	•	scope close
	•	MDC 清理回復

7.3 pseudo-code（完整）

interceptCall(call, headers, next):

  methodName = call.getMethodDescriptor().getFullMethodName()

  correlation = headers.get("x-correlation-id") // may be null
  request = headers.get("x-request-id")        // may be null
  traceparent = headers.get("traceparent")     // may be null

  // 1) Extract parent context from traceparent
  parentOtelContext = otelPropagator.extract(currentOtelContext, headersGetter)

  // 2) Start span
  span = tracer.spanBuilder("grpc.server/" + methodName)
              .setParent(parentOtelContext)
              .setSpanKind(SERVER)
              .startSpan()

  // 3) Put span into OTel context
  otelScope = span.makeCurrent()

  // 4) Open ObsContext scope (will generate correlation if missing)
  obsScope = obsContext.open(new ObsCarrier(correlation, request, traceparent))

  // 5) Sync MDC stable fields (only if SpanContext is valid)
  if span.getSpanContext().isValid():
      MDC.put("trace_id", span.getSpanContext().getTraceId())
      MDC.put("span_id", span.getSpanContext().getSpanId())
  MDC.put("service", obsProperties.service)
  // Note: env is injected by otelcol-agent, not by lib

  // 6) Continue call, wrap listener to close on end
  listener = next.startCall(call, headers)

  return new ForwardingServerCallListener(listener) {
      onComplete():
          try:
              span.setStatus(OK)
          finally:
              span.end()
              obsScope.close()
              otelScope.close()
              cleanup trace_id/span_id (remove)
      onCancel():
          try:
              span.setStatus(ERROR)
          finally:
              span.end()
              obsScope.close()
              otelScope.close()
              cleanup
  }


⸻

8. gRPC Client Interceptor（必實作、行為固定）

8.1 類別

ObsGrpcClientInterceptor implements io.grpc.ClientInterceptor

8.2 行為（固定）
	•	從 ObsContext 取得 correlation_id/request_id
	•	若 correlation_id 仍為 null（理論不應發生）：
	•	立即生成 UUIDv4 並 寫入 ObsContext（開新 scope 不允許）
	•	作法：直接 MDC.put + threadLocal 更新（在 ObsContext 提供 internal method）
	•	注入 metadata：
	•	traceparent：OTel inject
	•	x-correlation-id
	•	x-request-id（若存在）

8.3 pseudo-code

interceptCall(method, callOptions, next):

  return new ForwardingClientCall(next.newCall(method, callOptions)) {

      start(responseListener, headers):

          // 1) ensure correlation
          correlation = obsContext.getCorrelationIdOrNull()
          if correlation is null:
              correlation = IdGenerator.uuidV4Lower()
              obsContext.forceSetCorrelationId(correlation) // internal-only

          request = obsContext.getRequestIdOrNull()

          // 2) inject traceparent
          otelPropagator.inject(currentOtelContext, headersSetter)

          // 3) inject ids
          headers.put("x-correlation-id", correlation)
          if request != null:
              headers.put("x-request-id", request)

          delegate.start(responseListener, headers)
  }


⸻

9. gRPC Channel Registry / Stub Factory（集中模式，必實作）

9.1 GrpcChannelRegistry（public API 固定）

public final class GrpcChannelRegistry {
  public ManagedChannel getOrCreate(String targetName);
  public void shutdownAll();
}

行為固定
	•	以 ConcurrentHashMap<String, ManagedChannel> 快取
	•	若 targetName 不存在配置：IllegalArgumentException
	•	channel 建立：
	•	ManagedChannelBuilder.forAddress(host, port)
	•	plaintext=true → usePlaintext()
	•	必須在 Spring context close 呼叫 shutdownAll()

pseudo-code:

getOrCreate(name):
  cfg = props.grpc.targets.get(name)
  if cfg missing: throw IllegalArgumentException

  return map.computeIfAbsent(name, n -> buildChannel(cfg))

9.2 GrpcStubFactory（public API 固定）

public final class GrpcStubFactory {
  public <T> T blocking(String targetName, Function<Channel, T> stubFactory);
}

pseudo-code:

blocking(target, stubFactory):
  channel = registry.getOrCreate(target)
  stub = stubFactory.apply(channel)
  return stub

v1.0 不提供 future/async stub；如需擴充版本號升級。

⸻

10. RabbitMQ Publish Wrapper（必實作、行為固定）

10.1 類別與 API（固定）

public final class ObsRabbitPublisher {
  public void publish(String exchange, String routingKey, byte[] body, String event);
}

10.2 行為（固定）
	•	event 必填，空值直接 throw
	•	headers 注入：
	•	event
	•	x-correlation-id（必）
	•	x-request-id（若有）
	•	traceparent（OTel inject）
	•	publish 使用 Spring RabbitTemplate

pseudo-code:

publish(exchange, routingKey, body, event):
  require event not blank
  correlation = obsContext.getCorrelationIdOrNull()
  if correlation null:
      correlation = IdGenerator.uuidV4Lower()
      obsContext.forceSetCorrelationId(correlation)

  request = obsContext.getRequestIdOrNull()

  rabbitTemplate.convertAndSend(exchange, routingKey, body, message -> {
      headers = message.getMessageProperties().getHeaders()
      headers["event"] = event
      headers["x-correlation-id"] = correlation
      if request != null: headers["x-request-id"] = request

      // inject traceparent into headers
      otelPropagator.inject(currentOtelContext, setterForRabbitHeaders(headers))
      return message
  })


⸻

11. RabbitMQ Consume Advice（必實作、行為固定）

11.1 類別

ObsRabbitListenerAdvice implements org.aopalliance.intercept.MethodInterceptor

使用 RabbitListenerContainerFactory 的 advice chain 掛載。
v1.0 固定使用 Advice，不接受其他做法。

11.2 行為（固定）
	•	從 Message headers 讀取：
	•	traceparent
	•	x-correlation-id
	•	x-request-id
	•	event
	•	Extract parent context → start span（kind=CONSUMER）
	•	ObsContext.open()（缺 correlation → 生成）
	•	若有 event → withEvent(event)（覆蓋）
	•	finally：span end + scope close + MDC cleanup

pseudo-code:

invoke(invocation):
  message = find Message argument from invocation args
  headers = message.properties.headers

  correlation = headers["x-correlation-id"]
  request = headers["x-request-id"]
  event = headers["event"]
  traceparent = headers["traceparent"]

  parentCtx = otelPropagator.extract(currentOtelContext, headersGetter(headers))

  span = tracer.spanBuilder("mq.consume/" + (event ?? "unknown"))
              .setParent(parentCtx)
              .setSpanKind(CONSUMER)
              .startSpan()

  otelScope = span.makeCurrent()
  obsScope = obsContext.open(new ObsCarrier(correlation, request, traceparent))

  if event not blank:
      eventScope = obsContext.withEvent(event)
  else:
      eventScope = null

  // fill MDC trace/span/service (only if SpanContext valid)
  if span.getSpanContext().isValid():
      MDC.put("trace_id", span.traceId)
      MDC.put("span_id", span.spanId)
  MDC.put("service", obsProperties.service)
  // Note: env is injected by otelcol-agent, not by lib

  try:
      return invocation.proceed()
  catch ex:
      span.recordException(ex)
      span.setStatus(ERROR)
      throw ex
  finally:
      span.end()
      if eventScope != null: eventScope.close()
      obsScope.close()
      otelScope.close()
      MDC.remove("trace_id"); MDC.remove("span_id")


⸻

12. AutoConfiguration（必實作）

12.1 ObsAutoConfiguration（固定）
	•	@AutoConfiguration
	•	@EnableConfigurationProperties(ObsProperties.class)
	•	必須註冊 Beans：
	•	ObsContext
	•	ObsGrpcServerInterceptor（必須加上 @GrpcGlobalServerInterceptor 實現全域註冊）
	•	ObsGrpcClientInterceptor
	•	GrpcChannelRegistry
	•	GrpcStubFactory
	•	若 classpath 有 RabbitTemplate：
	•	註冊 ObsRabbitPublisher
	•	若 classpath 有 Rabbit listener container factory：
	•	自動把 ObsRabbitListenerAdvice 掛入 advice chain
	•	不得要求各服務手動掛載

12.2 gRPC Server Interceptor 全域註冊機制（固定）

使用 net.devh:grpc-spring-boot-starter 提供的 @GrpcGlobalServerInterceptor 註解，
確保 ObsGrpcServerInterceptor 自動掛載到所有 gRPC 服務端點。

服務工程師不需要任何額外配置，只要引入 observability-common 依賴即可。

pseudo-code:

@Bean ObsContext

@Bean
@GrpcGlobalServerInterceptor  // 全域註冊到所有 gRPC services
ObsGrpcServerInterceptor

@Bean ObsGrpcClientInterceptor
@Bean GrpcChannelRegistry
@Bean GrpcStubFactory

if (RabbitTemplate present):
  @Bean ObsRabbitPublisher

if (RabbitListenerContainerFactory present):
  customize containerFactory -> add advice ObsRabbitListenerAdvice


⸻

13. 使用方式（服務工程師唯一做法）

13.0 專案設定（必做）

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

# OpenTelemetry 設定（必要，啟用 trace_id/span_id）
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

Step 3: 設定 logback-spring.xml（src/main/resources/）

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <include resource="logback-obs-base.xml"/>
</configuration>
```

Step 4: 設定 lombok.config（專案根目錄，與 pom.xml 同層）

建立檔案 lombok.config，內容如下：

```
# lombok.config
lombok.log.custom.declaration = io.github.will413028.hawkr.common.obs.log.ObsLogger io.github.will413028.hawkr.common.obs.log.ObsLogger.forClass(TYPE)
```

設定後即可使用 @CustomLog 註解。

⸻

13.1 gRPC Server
	•	不需任何程式碼
	•	interceptor 由 AutoConfiguration 全域註冊
	•	所有 gRPC 請求自動建立 span、自動傳遞 correlation_id

⸻

13.2 gRPC Client

唯一允許方式（自動帶上 traceparent、x-correlation-id、x-request-id）：

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

呼叫時會自動注入：
	•	traceparent（W3C Trace Context）
	•	x-correlation-id
	•	x-request-id（若存在）

⸻

13.3 RabbitMQ

Producer（自動注入 headers）：

@Service
public class OrderService {

    @Autowired
    private ObsRabbitPublisher publisher;

    public void createOrder(byte[] orderData) {
        // event 必填，會自動帶上 traceparent、correlation_id
        publisher.publish("order.exchange", "order.created", orderData, "order.create.success");
    }
}

Consumer（不需改 method signature）：

@Component
public class OrderConsumer {

    @RabbitListener(queues = "order.queue")
    public void handleOrder(Message message) {
        // 直接寫業務邏輯
        // trace/correlation/span 已由 ObsRabbitListenerAdvice 自動處理
    }
}

禁止在 method 內讀 headers、建 span。

⸻

13.4 ObsLogger（v1.2 新增，v1.3 更新）

業務層結構化日誌工具，className 與 methodName 自動偵測加入 event。

Event 格式：`<ClassName>.<methodName>.<result>`
- ClassName：由 forClass() 自動取得（使用 SimpleClassName）
- methodName：由 stack trace 自動偵測（若不在 method 內則為空）
- result：使用者傳入（可為空字串）

API（固定）：

public final class ObsLogger {
    public static ObsLogger forClass(Class<?> clazz);  // className 自動 prepend
    public static ObsLogger forName(String name);       // 無 className prefix

    public void info(String result, String msg);
    public void info(String result, String msg, String bzCol, String bzValue);
    public void warn(String result, String msg);
    public void warn(String result, String msg, String bzCol, String bzValue);
    public void error(String result, String msg);
    public void error(String result, String msg, String bzCol, String bzValue);
    public void error(String result, String msg, Throwable ex);
    public void error(String result, String msg, String bzCol, String bzValue, Throwable ex);
}

使用範例（使用 @CustomLog）：

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
            // event = "UserGrpcService.register.success"
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

行為（固定）：
	•	result 必填（可為空字串，空字串時 event 不含 result 部分）
	•	msg 必填（可為空字串）
	•	methodName 自動從 stack trace 偵測（lambda/synthetic 方法會被過濾）
	•	event 格式：<ClassName>.<methodName>.<result>
	•	bzCol/bzValue 可選，用於注入業務上下文（如 order_id、account_id）
	•	bzCol/bzValue 在 log 輸出後自動清理

常用 bzCol 命名：
	•	order_id - 訂單 ID
	•	account_id - 帳號 ID
	•	product_id - 產品 ID
	•	seller_account_id - 賣家帳號 ID
	•	payment_id - 支付 ID

已移除：ObsEvent.withEvent() - 改用 ObsLogger

⸻

13.5 自動獲得的功能（零程式碼）

- ✅ gRPC Server 自動追蹤（interceptor 自動註冊）
- ✅ trace_id / span_id / correlation_id 跨服務自動傳遞
- ✅ Access Log（錯誤/慢請求自動記錄）
- ✅ MQ 消費自動建立 span 與 context

⸻

14. 禁止事項（Hard Stop）
	•	禁止業務端使用：
	•	ManagedChannelBuilder
	•	NettyChannelBuilder
	•	OkHttpChannelBuilder
	•	newBlockingStub/newFutureStub/newStub
	•	禁止業務端手寫：
	•	gRPC metadata
	•	RabbitMQ headers

⸻

15. 驗收條件（必通過）
	1.	任一 gRPC request：Tempo 可見 parent-child span chain
	2.	任一 MQ message：producer span 與 consumer span 同 trace（若上游帶 traceparent）
	3.	任一 log：必含 correlation_id；若有 span 則含 trace_id/span_id

⸻

16. gRPC Server Guard / Access Log 設計（v1.1 新增）

16.1 設計目標（不可偏離）
	1.	即使業務層不寫任何 log，也能留下「被呼叫過」的痕跡
	2.	任何 exception 都能被觀測到（log + trace），但不影響 gRPC 正常回應行為
	3.	不產生爆量 log，不吞錯誤，不改寫業務回應

16.2 Interceptor 角色定位（Hard Rule）
	•	gRPC Server Interceptor 只做觀測（observe）
	•	Interceptor = Guard + Recorder，不是 Controller、不是 ErrorHandler

絕對禁止在 interceptor 層：
	•	組 response
	•	呼叫 responseObserver.onNext / onCompleted / onError
	•	吞掉 exception

⸻

17. Access Log 觸發規則（固定）

17.1 觸發條件

狀態                      | 是否印 log | log level
--------------------------|------------|----------
gRPC Status != OK         | 必印       | ERROR
OK 且耗時 > slowThreshold | 必印       | WARN
OK 且正常                 | 抽樣       | INFO

17.2 Access Log 格式（v1.2 更新）

Event 格式：`<grpcService>.<grpcMethod>.<end|error>`

| 狀態 | event | msg | stack_trace |
|------|-------|-----|-------------|
| Status != OK | `service.method.error` | exception.message | exception.stackTrace |
| OK (slow/normal) | `service.method.end` | "" (empty) | - |

MDC 欄位：
	•	trace_id（若 SpanContext valid）
	•	span_id（若 SpanContext valid）
	•	correlation_id
	•	event
	•	service

已移除欄位：grpc.service, grpc.method, grpc.status, grpc.duration_ms, env

17.3 MDC 命名規範（Hard Rule）
	•	僅允許 snake_case（trace_id, span_id）
	•	禁止 camelCase（traceId, spanId）
	•	interceptor 必須移除其他框架設置的 camelCase 欄位

17.4 Invalid SpanContext 處理規範（Hard Rule）
	•	SpanContext.isValid() == false 時：
	•	不輸出 trace_id / span_id（欄位缺席）
	•	禁止輸出全 0 值（如 00000000000000000000000000000000）
	•	原因：全 0 會讓人誤以為 trace 有串，實際上 Tempo 查不到任何東西

⸻

18. Access Log 配置（固定）

18.1 application.yml keys

hawkr:
  obs:
    access-log:
      ok-sample-rate: 0.01      # OK 請求抽樣比例（0.0-1.0）
      slow-threshold-ms: 1000   # 慢請求閾值（毫秒）

18.2 配置說明
	•	ok-sample-rate：
	•	0.0 = 不印 OK access log（預設）
	•	0.01 = 1% 抽樣
	•	1.0 = 全量（不建議 production）
	•	slow-threshold-ms：
	•	超過此閾值視為 SLOW，固定印 WARN
	•	預設 1000ms

18.3 環境建議配置

環境       | ok-sample-rate | slow-threshold-ms
-----------|----------------|------------------
DEV        | 0.0            | 1000
PRODUCTION | 0.001          | 1000

⸻

19. gRPC Server Interceptor 行為規範（v1.1 完整流程）

19.1 Intercept 流程（不可更動順序）

1. onCallStart
   - 解析 inbound metadata (traceparent, x-correlation-id, x-request-id)
   - 建立 server span
   - 建立 ObsContext scope
   - 寫入 MDC (trace_id, span_id, service)（若 SpanContext valid）
   - 記錄 startTime

2. wrap ServerCall
   - 攔截 close(Status status, Metadata trailers)

3. wrap ServerCall.Listener
   - 不處理業務邏輯
   - 不 catch 改寫回應
   - 僅在發生 exception 時：
     - span.recordException(e)
     - span.setStatus(ERROR)
     - rethrow（絕不吞掉）

4. onCallClose（ServerCall.close）
   - 計算 duration
   - 設置 event MDC（格式：grpcService.grpcMethod.end|error）
   - 依 Status / duration 決定是否印 access log
   - 結束 span
   - 清理 MDC / ObsContext

19.2 Access Log 輸出邏輯（pseudo-code）- v1.2 更新

if status != OK:
    MDC.put("event", grpcService + "." + grpcMethod + ".error")
    if exception != null:
        log.error(exception.message, exception)  // with stack trace
    else:
        log.error(status.description ?? "")
else if duration > slowThresholdMs:
    MDC.put("event", grpcService + "." + grpcMethod + ".end")
    log.warn("")  // empty msg
else:
    if random() < okSampleRate:
        MDC.put("event", grpcService + "." + grpcMethod + ".end")
        log.info("")  // empty msg

⸻

20. Error Handling 規範（非常重要）

20.1 允許行為
	•	記錄 exception 到 span（span.recordException）
	•	印 error log
	•	讓 gRPC framework 正常處理錯誤回傳

20.2 禁止行為
	•	catch 後 onNext 偽裝成功
	•	catch 後吞掉錯誤
	•	在 interceptor 層改寫 Status

20.3 Exception 類型處理規則

類型                      | 行為
--------------------------|----------------------------------
RuntimeException          | recordException + ERROR + rethrow
Error（如 OutOfMemoryError）| 不攔、不吞，直接 rethrow

⸻

21. v1.1 新增驗收條件
	1.	不寫任何 log.info，gRPC error 仍能在 Loki 查到
	2.	Loki error log 必須包含 correlation_id
	3.	Tempo 可看到完整 server span（含 error）
	4.	OK 呼叫不會產生大量 access log
	5.	interceptor 不影響原本 gRPC 回應行為