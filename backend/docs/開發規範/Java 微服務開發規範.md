# Java 微服務開發規範

> **適用範圍（2026-09-24 起）**：只適用遷移期間仍在運作的 Java 服務，Java 服務全部改寫成 Go 後（遷移計畫階段 B）退場（`docs/adr/2026-09-24-go-modular-monolith.md` D1）。其中與語言無關的契約規則（型別、TSID、身分與授權、CodeMeta）已併入 `backend/AGENTS.md`。


本文件彙整 Java 微服務開發的強制規範，供開發者日常參照。

---

## 1. 命名規範

| 類別 | 風格 | 範例 |
|------|------|------|
| 變數 / 方法 | `camelCase` | `accountId`、`getOrderDetail()` |
| 類別 | `PascalCase` | `OrderService`、`ShippingFeeService` |
| Enum 值 | `UPPER_SNAKE_CASE` | `MAIN_ISLAND`、`SEVEN_ELEVEN_STORE_PICKUP` |
| JSON Body 欄位 | `camelCase` | `{ "tradeNo": "", "accountId": "" }` |
| JSONB 內容 key | `camelCase`（與 Java 欄位名一致） | — |

---

## 2. 型別對應（Proto / Java / DB）

### 2.1 數字欄位

Proto 的 `string` 要求僅適用於 **gRPC 介面與對外 API**。後端內部使用原生數字型別，在 gRPC 邊界轉換。

| 層級 | 型別 | 說明 |
|------|------|------|
| Proto 定義 | `string` | 所有數字欄位（ID/金額/數量/計數）一律 string |
| Gateway HTTP JSON | string | 對外回應以字串傳遞 |
| Java gRPC Service | `parseLong()` / `formatLong()` | gRPC 邊界轉換 |
| Java Service / Entity | `Long` / `Integer` | 原生數字型別 |
| DB Schema | `BIGINT` / `INTEGER` | 原生數字型別 |

**禁止**：
- Proto 使用 `int32` / `int64` / `double`（見下方例外）
- Service 層直接用 Proto string 做數值計算
- Entity 用 `String` 存儲數字欄位

**例外規則**：

| # | 項目 | 條件 |
|---|------|------|
| 1 | Gateway JWT 注入的身分欄位（`account_id` 等） | 僅限 Request message、Gateway 注入、前端不可控。Response 中同欄位仍必須 `string` |
| 2 | Response `int32 code` | 全專案慣例，暫免 |
| 3 | 純微服務間 gRPC 欄位 | 不經 Gateway 暴露至前端的內部通訊允許原生數值型別 |

### 2.2 金額欄位

| 規則 | 值 |
|------|------|
| 貨幣單位 | 新台幣**元**（非「分」） |
| DB | `BIGINT` |
| Java | `Long`（禁止 `BigDecimal` / `Double` / `Float`） |
| Proto | `string` |

### 2.3 時間欄位（全系統統一 UTC）

| 層級 | 型別 | 說明 |
|------|------|------|
| DB | `TIMESTAMPTZ` | 禁止 `TIMESTAMP`，詳見 `PostgreSQL 設計規範與最佳實踐.md §三` |
| Java | `LocalDateTime`（值語意 UTC） | 禁止 `OffsetDateTime` / `Instant` / `ZonedDateTime` |
| 取得時間 | `DateTimeUtils.nowUtc()` | **禁止**裸用 `LocalDateTime.now()` |
| Proto / API | ISO 8601 帶 Z | 格式：`2026-03-31T08:30:00Z`，前端自行轉換時區 |
| MetaObjectHandler | `LocalDateTime.class` + `DateTimeUtils.nowUtc()` | 由 `MyBatisPlusAutoConfiguration` 統一管理 |

時區策略：DB 存 UTC、Java 值語意 UTC、API 輸出帶 Z。

---

## 3. 主鍵與 TSID

### 3.1 主鍵選擇

| 類型 | 用途 | Entity 配置 |
|------|------|-------------|
| TSID | 對外暴露的業務表 | `@TableId(type = IdType.INPUT)` + `TsidUtil.generate()` |
| BIGSERIAL | 內部表（日誌/關聯/事件/Webhook） | `@TableId(type = IdType.AUTO)` |
| UUID | 臨時資源（upload_session） | `@TableId(type = IdType.INPUT)` + `UUID.randomUUID()` |

### 3.2 Node ID 分配表

| 微服務 | HTTP Port | gRPC Port | Node ID Base |
|--------|-----------|-----------|-------------|
| payment | 9924 | 9925 | 0 |
| media | 9936 | 9937 | 100 |
| product | 9912 | 9913 | 200 |
| order | 9914 | 9915 | 300 |
| cart | 9926 | 9927 | 400 |
| coupon | 9950 | 9951 | 500 |
| shipping | 9930 | 9931 | 600 |
| aftersales | 9922 | 9923 | 700 |
| product-import | 9960 | 9961 | 800 |
| payuni-webhook | 9946 | 9947 | 500（與 coupon 共用，見下） |

本表涵蓋全部 10 個 Java 微服務。**Node ID Base 的真實來源是各服務 `src/main/resources/application.yml` 的 `hawkr.tsid.node-id-base`**，本表是它的彙整；新增服務時挑下一個未使用的百位段並同步本表。每個服務實際佔用 `base` ~ `base+99`，區間內由 `pod-index` 取值。

Port 欄位以 `k8s/values/{service}-service.yaml` 的 `SERVER_PORT` / `GRPC_PORT`（部署值）為準。少數服務 `application.yml` 的本機預設值與部署值不同（例：shipping 本機預設 9928 / 9929，部署為 9930 / 9931），這不影響 Node ID 分配。

**payuni-webhook 與 coupon 共用 500 是刻意的**：payuni-webhook 全庫只有 `webhook_inbox` 與 `outbox_events` 兩張表，主鍵皆為 `BIGSERIAL`（`V1__init_webhook_tables.sql`），該服務**不產生任何 TSID**，因此不會與 coupon 的 500 區段碰撞。日後若該服務新增 TSID 主鍵的表，必須先改配一個未使用的區段。

TSID 初始化由 `TsidConfiguration`（common-core）自動完成，**禁止**微服務手動呼叫 `TsidUtil.init()`。

---

## 4. gRPC RPC 設計規則

### 4.1 角色分離（強制）

不同角色**必須定義獨立 RPC**，禁止共用 RPC + 條件分流。

| 角色 | 命名模式 | 範例 |
|------|---------|------|
| 公開（匿名） | `GetPublic{Resource}` | `GetPublicProduct` |
| App（BUYER/SELLER） | `Get{Resource}` / `List{Resource}` | `GetProduct`、`ListSellerProducts` |
| Admin | `{Action}{Resource}ForAdmin` | `GetProductForAdmin` |
| 內部服務間 | `Get{Resource}Simple` / `Get{Resource}Internal` | `GetProductSimple` |

**禁止**：以特殊值（`accountId=0`、`null`）或 boolean flag（`isAdmin`）區分角色。

### 4.2 內部微服務 gRPC 授權（HMAC 簽章，SSOT = 詳細設計書 52 號）

微服務間 gRPC「內部身分」由 **HMAC-SHA256 簽章驗證**（issue #1366）——`x-internal-service` 字面值**不再是信任依據**（原「非空即信任」曾使任意同 ns pod 帶該 header 即取得 internal bypass、拉取全站訂單 PII）。完整協議見 **詳細設計書 52 號《微服務間內部授權》**。

- 出站帶四 header：`x-internal-service`（caller 名，資訊用）+ `x-internal-ts` + `x-internal-nonce` + `x-internal-sig`（`base64(HMAC-SHA256(secret, serviceName\nts\nnonce\naccountIdOrEmpty))`）；由 `GrpcAuthClientInterceptor`（common-core）於 `GrpcStubFactory`/`GrpcChannelRegistry` 鏈上**自動加簽**，Client 透過 `GrpcStubFactory` 建立即可。
- server 端 `GrpcAuthServerInterceptor` **只對帶 `x-internal-service` 的呼叫驗簽**；驗簽通過才 `GrpcAuthContext.isInternalCall()=true` → `requireAccountType()` 跳過。不帶 internal header 的呼叫（Gateway 使用者呼叫）維持 `requireAccountType` 使用者授權路徑、不被 HMAC 擋。
- **分階段 rollout**：`INTERNAL_AUTH_MODE` = `off | permissive | enforcing`；permissive（無/無效 sig 放行 + WARN）觀察 `internal.auth.unsigned` 歸零後才切 enforcing（無有效 sig → `UNAUTHENTICATED`）。密鑰 `hawkr-internal-auth`（+ `_PREV` 雙密鑰輪替），enforcing + 缺 secret → fail-closed。

**禁止**：
- Gateway 帶 `x-internal-service` / 簽內部授權（Gateway→service 是使用者面，走 `requireAccountType`）
- 手動傳遞 auth headers（interceptor 已自動處理）
- 直接改生成/簽章相關檔案繞過驗簽；以 `x-internal-service` 字面值作授權依據

---

## 5. 身分與授權

### 5.1 Header 規範

| Header | 來源 | 說明 |
|--------|------|------|
| `x-account-id` | Gateway JWT 注入 | 已驗證身分識別，禁止客戶端覆寫 |
| `x-account-type` | Gateway JWT 注入 | BUYER/SELLER/ADMIN，Phase 1 唯一授權依據 |
| `x-role` | 保留未來 | 現階段不單獨作為授權依據 |

- 身分驗證僅在 Gateway 進行，下游微服務不得自行驗證外部使用者
- Gateway 必須清除外部請求中同名 header 後重新注入

### 5.2 merId vs accountId

| 欄位 | 用途 | 使用範圍 |
|------|------|---------|
| `accountId` | 系統內部使用者識別 | **所有**業務 CRUD（商品/訂單/分類/購物車） |
| `merId` | PayUni 金流平台商家編號 | **僅限** PayUni 付款/退款/Webhook |

- Gateway 僅注入 `x-account-id`、`x-account-type`，**不注入** `merId`
- `merId` 由前端 body 傳入或後端從訂單資料查詢
- **禁止**將 `merId` 用於商品/訂單/分類/購物車等業務場景

---

## 6. Enum 與禁止硬編碼

### 6.1 基本規則

所有業務語意值（字串/數值/狀態碼/Event 名稱/MQ key/Header key）必須定義為 Enum 或 Constants，**禁止硬編碼**。

### 6.2 跨服務 Enum 位置

唯一合法位置：`services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/`

| 使用範圍 | 定義位置 |
|---------|---------|
| 單一微服務 | 該服務內部 |
| 兩個以上微服務 | `common-core/utils/code/` |

### 6.3 CodeMeta 介面

跨服務 Enum 必須實作 `CodeMeta` 介面，提供 `getCode()` 和 `getLabel()` 方法。

補充邊界：

- 只要該值會透過 HTTP API、gRPC、MQ payload、Gateway DTO、跨服務欄位或共享 SDK 暴露給其他團隊/語言做分支判斷，就視為共享契約，必須先納入 CodeMeta
- 只有單一微服務內部使用、且不會外露給第三方判斷的 local enum / constant，才可以留在服務內部
- gRPC 契約若需調整共享值，必須先改 `api/proto`，再同步到 `services/java/shared/java-proto/src/main/proto`；禁止直接修改 Java proto 副本

### 6.4 CodeMeta Enum 合法值驗證

工具類：`CodeMetaValidator`（common-core `utils/code/`），支援所有 `CodeMeta` Enum 的通用驗證。

**核心規則**：所有對應 CodeMeta Enum 的輸入欄位，寫入 DB 前**必須驗證合法性**。

```java
// 使用方式
CodeMetaValidator.validate(WeightUnit.class, unitCode, "unit");
// 不合法時拋出 INVALID_ARGUMENT
```

涉及 Enum 欄位的新增或修改，都要檢查是否有驗證，缺少則補上。

---

## 7. 共用工具

### 7.1 運費計算 — 一律呼叫 shipping-service gRPC

**運費金額的唯一真實來源是 shipping-service**：RPC `CalculateShippingOptions`（定義於 `api/proto/shipping/shipping.proto`）。common-core **不提供**運費金額計算，只提供區域判定、單位換算等純函式，供其他微服務在本地做前置判斷與過濾。

common-core 運費輔助工具（唯一合法位置：`services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/shipping/`）：

| 類別 | 說明 |
|------|------|
| `ShippingAreaResolver` | 三碼郵遞區號 → `ShippingArea`（本島 / 離島）；`resolveRoute()` 由寄件 + 收件郵遞區號判定 `ShippingRoute` |
| `WeightConverter` | `convertToGrams(weight, WeightUnit)`，各重量單位統一換算為公克 |
| `DeliveryZone` | 內部計算用配送區域（由寄件 / 收件區域組合判定，供費率查表） |
| `SellerShippingFilter` | 依賣家當下已開通的配送方式過濾 shipping options，並計算 onboarding 指紋（`toMethodKey()` / `isAvailable()` / `onboardingSig()`） |

相關 Enum（`common-core/utils/code/`）：`ShippingMethod`、`ShippingTemperature`、`ShippingArea`、`ShippingRoute`、`WeightUnit` — 合法值一律以 [業務類型碼](../代碼管理（CodeMeta）/業務類型碼.md) 為準。

**強制規則**：
1. 需要「運費金額」→ **一律呼叫 shipping-service gRPC `CalculateShippingOptions`**，**禁止**在各微服務內自行實作運費計算邏輯（含「簡化版」）
2. 只需判斷配送區域 / 換算重量單位 → 使用上表 common-core 工具，禁止各自實作
3. 費率級距是 shipping-service 的 `shipping_fee_tiers` 表（由 `ShippingFeeService` 讀取），費率變更只動這裡，禁止在別處放死常數
4. 上表工具是給**其他服務本地使用**的（現行呼叫端：product-service、order-service），shipping-service 自身不依賴它們
5. 第 3 個服務需要某項 shipping 計算 → 先查 common-core，沒有則抽上去；common-core 不支援目前需求（新配送方式、新計價邏輯等）**必須先提出討論**，不得自行繞過

完整設計見 [21-運費計算模組（Shipping Fee Module）詳細設計書](../詳細設計書/21-運費計算模組（Shipping%20Fee%20Module）詳細設計書.md) 與 [06-物流微服務（Shipping Service）詳細設計書](../詳細設計書/06-物流微服務（Shipping%20Service）詳細設計書.md)。

### 7.2 其他共用靜態工具

| 工具類 | 說明 |
|--------|------|
| `ProtoConversionUtils` | Proto 與 Java 物件的轉換 |
| `CodeMetaValidator` | Enum 合法值驗證（詳見 §6.4） |
| `InputValidator` | 通用輸入驗證（尺寸/整數/金額/手機/地址），規範見 [26-輸入驗證模組（Java InputValidator）詳細設計書](../詳細設計書/26-輸入驗證模組（Java%20InputValidator）詳細設計書.md) |
| `DateTimeUtils` | UTC 時間工具 — `nowUtc()` / `formatIso8601()` / `toEpochMillis()` |

---

## 8. Observability 規範

### 8.1 統一依賴

微服務僅需依賴 `observability-common`，禁止自行宣告 OTel / Micrometer Tracing / logstash-logback-encoder 依賴或版本。

### 8.2 強制使用方式

```java
// Logger — 統一用 ObsLogger
private static final ObsLogger log = ObsLogger.forClass(XxxService.class);
// Event 命名：ClassName.action.result

// gRPC Client — 透過 GrpcStubFactory 建立
grpcStubFactory.blocking("media", MediaGrpc::newBlockingStub);

// RabbitMQ — 透過 ObsRabbitPublisher 發送
obsRabbitPublisher.publish(exchange, routingKey, payloadBytes, event);
```

### 8.3 嚴格禁止

- 建立 `ManagedChannel` / `NettyChannelBuilder`
- 手動 `new` gRPC Stub
- 手寫 gRPC metadata（trace / correlation）
- 手寫 RabbitMQ headers
- 使用 `System.out.println`

### 8.4 日誌配置

每個微服務 `src/main/resources/logback-spring.xml` 必須引用共用 base：

```xml
<configuration>
    <include resource="logback-obs-base.xml"/>
</configuration>
```

### 8.5 OTel / Actuator 配置

統一由 `observability-common` 的 `observability-defaults.properties` 提供。
微服務 `application.yml` **禁止**配置 `otel.*`、`management.endpoints.*`、`management.tracing.*`、`management.otlp.*`。

### 8.6 RabbitMQ 注意事項

微服務 RabbitMQ 配置**禁止**將 `Jackson2JsonMessageConverter` 宣告為全域 Bean（會導致 `byte[]` 雙重序列化）。

### 8.7 ComponentReadiness 契約（強制）

> ⚠️ 任何「啟動時載入 / 啟動後快取就緒才能正確提供服務」的元件，**必須**實作 `ComponentReadiness` 介面，否則 K8s 會在 cache 尚未 warm 時就把流量導入，產生誤導性錯誤（例如冷啟動視窗的結帳 `ALL_SHIPPING_EXCEEDED`）。

**介面位置**：`services/java/shared/observability-common/src/main/java/io/github/will413028/hawkr/common/obs/health/ComponentReadiness.java`

```java
public interface ComponentReadiness {
    /** 元件是否已就緒接受流量 */
    boolean isReady();
    /** 元件名稱，用於 health endpoint 顯示 */
    String name();
}
```

**自動掃描機制**：observability-common 提供 `ComponentReadinessHealthIndicator`，會：
- 自動收集所有實作 `ComponentReadiness` 的 Spring bean
- 任一元件 `isReady() == false` → 整體 health 報 `DOWN`
- 全部 `isReady() == true` → 報 `UP`

**Health endpoint 配置**（observability-defaults.properties 集中管理，微服務不需重複設定）：

```properties
management.endpoint.health.probes.enabled=true
management.endpoint.health.group.readiness.include=*
```

`/actuator/health/readiness` 群組包含全部 HealthIndicator（含 db / redis / rabbit / componentReadiness 等），K8s readinessProbe 必須指向此路徑。

**K8s probe 路徑契約**（透過 helm chart 自動套用）：

| Probe | 路徑 | 含義 |
|---|---|---|
| readinessProbe | `/actuator/health/readiness` | 全部就緒指標（db、redis、rabbit、componentReadiness）皆 UP 才可接流量 |
| livenessProbe | `/actuator/health/liveness` | 僅 Spring `livenessState`；**不**檢查 cache 與外部依賴，避免冷啟動觸發 restart |
| startupProbe | `/actuator/health/readiness` | 與 readiness 相同，啟動完成的判定基準 |

**強制規範**：

1. 任何 `@PostConstruct` 載入快取的元件 **必須**實作 `ComponentReadiness`，flag 在載入完成後翻 true
2. **禁止**讓 livenessProbe 指向 readiness 群組（會造成冷啟動 restart loop）
3. **禁止**在微服務 `application.yml` 中覆寫 `management.endpoint.health.*` 配置（屬 §8.5 統一管理範圍）
4. 若元件以「lazy + circuit breaker」消除冷啟動視窗（如 `ShippingFeeCacheClient`，詳見 20 §4.9），則不需實作 `ComponentReadiness`

**參考實作**：
- `shipping-service`：`ShippingFeeService`、`HolidayService`（同進程 DB cache）
- `order-service`：`ShippingFeeCacheClient` 走 lazy + CB 路線，不實作此介面（見 20 §4.9）

---

## 9. MyBatis-Plus + PostgreSQL

### 9.1 JSONB 規範

- 每種結構一個 TypeHandler
- 禁止 `@MappedTypes(List.class)` 和 `@JsonProperty`
- JSONB 內容 key 一律 **camelCase**（與 Java 欄位名一致）
- TypeHandler 使用裸 `new ObjectMapper()`，**禁止**設定 `PropertyNamingStrategies.SNAKE_CASE`
  - 原因：裸 ObjectMapper 預設即 camelCase，無需額外設定，減少人為遺漏風險

### 9.2 自訂 SQL

有 TypeHandler 必須加 `@ResultMap`。

**Partial Projection 反模式禁令**（強制）：

`@Select` / `@SelectProvider` / XML mapper 的回傳型別若為 `*Entity` / `List<*Entity>` / `Optional<*Entity>`，SQL 子句**必須**涵蓋該 Entity 的所有非 transient 欄位（`SELECT *` / `SELECT t.*` 或明列全欄位皆可）。

若只需部分欄位 → **必須**改回傳 Java record 或 DTO（欄位名與 SELECT 對齊），存放於 `entity/projection/{Name}Projection.java`。

```java
// ✅ 部分欄位回 record
public record SkuProductRef(Long skuId, Long productId, Integer stockQty) {}
@Select("SELECT id AS sku_id, product_id, stock_qty FROM product_sku WHERE id IN ...")
List<SkuProductRef> findSkuProductRefs(@Param("ids") List<Long> ids);

// ❌ 部分欄位卻宣告 Entity（caller 取到未 SELECT 欄位即 NPE）
@Select("SELECT id, stock_qty FROM product_sku WHERE id IN ...")
List<ProductSkuEntity> findByIdIn(@Param("ids") List<Long> ids);  // 假 Entity，會炸
```

Code Review 必查：新增 / 修改任何 `@Select` 方法時，對照 SQL 與回傳型別欄位完整性。

### 9.3 Flyway（V1 凍結 + append-only）

- **V1 凍結**：每服務的首份 `V1__init_*.sql` 是唯讀基線，**禁止任何修改**（含格式、空白、註解 — 改了就是 checksum mismatch，遠端服務 crash loop）
- **已 commit 的 V2、V3… 與 V1 同等視為唯讀**：改錯了補下一號修正，不回頭改既有檔案
- **所有 schema 演進走 `V{N+1}__{verb}_{target}.sql` append-only 新增**，禁止跳號（`out-of-order=false`）。`verb` 用 `add` / `create` / `drop` / `alter` / `rename` / `data`，命名細則見設計書 38 §3.2
- **Flyway 預設開啟**（各服務 `application.yml` 皆為 `${FLYWAY_ENABLED:true}`）：部署即自動同步，是 always-on 機制，禁止靠人工對遠端 DB 手跑 DDL 取代 migration
- **破壞性 DDL**（`DROP TABLE` / `DROP COLUMN` / `ALTER COLUMN TYPE` 縮小型別）必須在 PR 描述標紅 + 預先 `pg_dump` 備份
- **seed 資料禁止寫進 migration**，走 `infra/seed-data/`

流程：本地 `./dev.sh restart {service}` 看 Flyway log 確認 migrate 成功 → 測試 → push → CI/CD 自動對遠端 migrate。

完整機制見 [38-Flyway 自動同步與 DB 自動初始化詳細設計書](../詳細設計書/38-Flyway%20自動同步與%20DB%20自動初始化（Flyway%20Auto%20Sync%20&%20DB%20Auto%20Init）詳細設計書.md)。

### 9.4 相容性：新增衍生欄位必須回填、危險變更必須拆兩段

`ADD COLUMN ... DEFAULT 'x'` 會把**既有每一列**都填成 `x`；若該欄位其實可由既有欄位推導，不變式從第一天就是破的 → 同一支 migration 內緊接回填 `UPDATE`，恆真的不變式再加 `CHECK` 交給 DB 守。

`DROP COLUMN` / 改名 / 縮小型別這類危險變更，必須拆成 expand → contract 兩個 release，不得單次切換。

判準、動作與 PR review checklist 見 [DB Migration 相容性規範（expand-contract）](./DB%20Migration%20相容性規範（expand-contract）.md)。

---

## 10. 共用配置（AutoConfiguration）

### 10.1 基本規則

兩個以上微服務共用的配置**必須定義在 `common-core`**，禁止各自維護。

命名規範：統一前綴 `hawkr.*`，**禁止**自創前綴（如 `media.*`、`oss.*`）。

### 10.2 共用配置表

| 配置 | Property Key | 環境變數 | AutoConfiguration |
|------|-------------|---------|-------------------|
| 媒體 base URL | `hawkr.media.base-url` | `MEDIA_BASE_URL` | `MediaUrlAutoConfiguration` |
| OTel/Actuator | `otel.*` / `management.*` | - | `ObsAutoConfiguration` |
| TSID | `hawkr.tsid.*` | - | `TsidConfiguration` |
| MyBatis-Plus | — | - | `MyBatisPlusAutoConfiguration`（分頁 + MetaObjectHandler + 樂觀鎖 `OptimisticLockerInnerInterceptor`，規則見 §10.5） |
| RabbitMQ Manual ACK | — | - | `RabbitMqAutoConfiguration`（manualAckContainerFactory） |
| Jackson 時區 | `spring.jackson.time-zone` | - | `JacksonAutoConfiguration`（ObjectMapper + UTC 時區） |

**優先級**：env var > 服務 application.yml > `common-core-defaults.properties`

### 10.3 媒體 URL 組裝

統一用 `MediaUrlProperties.buildUrl(objectKey)`（域名唯一來源 `MEDIA_BASE_URL`，49 號設計書 §8.5），由微服務組裝，Gateway 僅透傳。

### 10.4 新增共用配置流程

ConfigurationProperties → AutoConfiguration → defaults.properties → imports 註冊 → 更新配置表

### 10.5 樂觀鎖（`@Version`）三件套

樂觀鎖在本專案是 **註解 + interceptor + DB schema 三件套**，缺一即潛在故障（MyBatis-Plus `updateById` 撞 `Parameter 'MP_OPTLOCK_VERSION_ORIGINAL' not found`）。三件套由 `common-core` 統一保證，**禁止**各服務自行 override `MybatisPlusInterceptor` Bean。

| 件 | 位置 | 規則 |
|---|---|---|
| `OptimisticLockerInnerInterceptor` | `common-core/MyBatisPlusAutoConfiguration.mybatisPlusInterceptor()` | **必須**註冊（位於 `PaginationInnerInterceptor` 之前，符合官方建議順序） |
| `@Version private Integer version` | Entity | 業務需要樂觀鎖的表 entity 加 `@Version` 即可，**不**需要 `@TableField(fill=INSERT)` — DB DEFAULT 0 已保證初值 |
| DB `version` 欄位 | Flyway migration | **必須** `INTEGER NOT NULL DEFAULT 0`，禁止 nullable / 無 default |

**為何缺一即炸**：

`@Version` 註解觸發 MyBatis-Plus SQL 生成端把 `WHERE version = #{MP_OPTLOCK_VERSION_ORIGINAL}` 寫進 BoundSql。parameterize 階段需要 `OptimisticLockerInnerInterceptor` 把 `entity.getVersion()` 注入到 BoundSql 的 additionalParameter；缺 interceptor → 找不到該 binding 即拋 `BindingException`。

**禁止事項**：

- ❌ 各服務自行定義 `@Bean MybatisPlusInterceptor` override common-core 的版本（即使刻意加上 OptimisticLocker 也不行 — 應該推回 common-core 統一）
- ❌ 用 `LambdaUpdateWrapper.set(...).eq("id", X)` 繞過樂觀鎖（補丁）
- ❌ 業務層 `if (entity.getVersion() == null) entity.setVersion(0)` 補預設值（DB DEFAULT 0 已負責，不需要 Java 端補）
- ❌ 拿掉 `@Version` 或改 interceptor 跳 null（破壞設計）

**自我驗證**：

新增 `@Version` entity 後，本地跑兩條 case：

1. **Happy path**：INSERT → SELECT → `updateById` → 再 SELECT，確認 `version` 由 0 自動 +1 → 1，且生成 SQL 含 `WHERE version=?`
2. **反向（樂觀鎖本體生效）**：手動 `UPDATE table SET version=99 WHERE id=X`（模擬被搶先），再對舊 entity 跑 `updateById`，斷言回傳 `0 rowsAffected`（樂觀鎖正確拒絕陳舊更新）。若回 1，代表 interceptor 沒生效，回頭檢查 `MybatisPlusInterceptor` Bean 是否被服務自行 override 蓋掉了 common-core 的版本

---

## 11. 時間戳管理

### 11.1 三層機制，缺一不可

| 層級 | 機制 | 說明 |
|------|------|------|
| 主要 | `MetaObjectHandler`（MyBatisPlusAutoConfiguration） | 自動填充 `createdAt` / `updatedAt` |
| 安全網 | DB `DEFAULT NOW()` + trigger | 防止 handler 遺漏 |
| 禁止 | 業務程式碼手動 set | 禁止 `entity.setCreatedAt(...)` |

### 11.2 DB DEFAULT 不萬能

`@TableField(fill = FieldFill.INSERT)` 存在但 handler 沒填值時，MyBatis-Plus 會送 `NULL` 覆蓋 DB DEFAULT。因此 **handler 正確實作是第一優先**。

### 11.3 審計規則

審計時**必須驗內容**而非存在性：
- **錯**：「有 MetaObjectHandler → 合規」
- **對**：確認 `insertFill` 有填 `createdAt` + `updatedAt`、`updateFill` 有填 `updatedAt`

---

## 12. 微服務設定範本

每個微服務的 `application.yml` 必須包含以下配置：

```yaml
hawkr:
  obs:
    service: <service-name>   # 例如：product、order、payment
  tsid:
    enabled: true
    node-id-base: <見 §3.2 表>  # 例如：product = 200
    pod-index: ${POD_INDEX:0}
```

---

## 13. 測試規範

### 13.1 服務實例唯一性（強制）

執行 E2E 測試或整合測試時，**必須確保每個微服務只有一個實例在運行**：

| 檢查項目 | 說明 |
|---------|------|
| Docker 容器 | 同一服務只能有一個容器（禁止多個 Docker Compose 專案啟動相同服務） |
| 本地 Java 程序 | IDE 或 `mvn spring-boot:run` 啟動的服務，不得與 Docker 容器同時運行同一服務 |
| MQ Consumer 唯一 | 每個 RabbitMQ Queue 在測試期間只能有一個 Consumer，避免消費競爭導致訊息分流 |

**原因**：RabbitMQ 預設 Round-Robin 分配。若同一 Queue 有多個 Consumer，訊息會被隨機分配到不同實例，導致部分訊息被舊版本程式碼處理、測試結果不可預測（間歇性失敗）、資料不一致。

### 13.2 測試前檢查

```bash
# 確認無重複 Java 程序
ps aux | grep 'java.*hawkr' | grep -v grep

# 確認每個 Queue 只有一個 Consumer
docker exec hawkr-rabbitmq rabbitmqctl list_queues name consumers
```

### 13.3 RabbitMQ Port 限制

本地開發環境的 RabbitMQ **必須綁定 `127.0.0.1`**，禁止暴露到 `0.0.0.0`：

```yaml
ports:
  - "127.0.0.1:5672:5672"
  - "127.0.0.1:15672:15672"
```

### 13.4 測試資料規範

DB Enum 欄位**必須使用程式碼定義的合法值**。

**禁止**：
- 中文值（如「袋」、「包」、「盒」）
- 自創值（如小寫 `kg`，正確為 `KG`）
- 大小寫錯誤
- 空字串

**踩坑紀錄**：
- `product_sku.unit` 曾有中文值和小寫 `kg`，導致 `WeightUnit.fromCode()` crash，運費計算+結帳全面失敗
- `product.status` 曾有空字串，導致 `ProductStatus.fromValue()` crash

---

## 14. Java 檔名大小寫注意事項

新增或重新命名 Java 檔案後，**必須**確認 Git 追蹤的檔名與 class 名稱大小寫一致。

**原因**：macOS 檔案系統不區分大小寫，本地編譯正常但 CI/CD（Linux）區分大小寫，會導致大量編譯錯誤。

```bash
# 確認檔名正確
git ls-files | grep -i "YourNewFileName"
```

**踩坑紀錄**：`AftersalesGrpcService.java` vs `AfterSalesGrpcService` class 名稱不一致，macOS 正常但 CI/CD 產生 100+ 編譯錯誤。

---

## 15. 重複實作防護

### 15.1 新增前必須檢查

新增 Config / Bean / 工具類 / 共用方法前，**必須先搜尋 `common-core` 確認無現有實作**。

### 15.2 跨服務重複必須抽取

同一實作存在於 2 個以上微服務時，**必須先抽取到 `common-core`**，禁止各自維護。

### 15.3 踩坑紀錄

MetaObjectHandler 曾在 8 個微服務各有一份幾乎相同的程式碼。修 bug 時只修單一檔案而未質疑「為何有 8 份」。已完成抽取到 `MyBatisPlusAutoConfiguration`。
