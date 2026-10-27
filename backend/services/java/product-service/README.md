# Product Service

商品管理服務，提供商品 CRUD、規格管理、圖片管理功能。

## 架構概覽

```
product-service/
├── src/main/java/io/github/will413028/hawkr/product/
│   ├── constant/          # 常數定義
│   │   ├── CacheKeyConstants.java      # Redis 快取鍵
│   │   └── RefTypeConstants.java       # Media 引用類型
│   ├── converter/         # 轉換器層
│   │   ├── ProductEntityConverter.java # Entity ↔ DTO
│   │   ├── ProductProtoConverter.java  # DTO ↔ Proto
│   │   └── ProductSkuConverter.java    # SKU Entity ↔ DTO
│   ├── dto/               # 資料傳輸物件
│   │   ├── request/       # 請求 DTO
│   │   ├── response/      # 回應 DTO
│   │   └── cache/         # 快取 DTO
│   ├── entity/            # 資料庫實體
│   │   ├── ProductEntity.java
│   │   ├── ProductSkuEntity.java
│   │   └── IdempotencyRequestEntity.java
│   ├── exception/         # 自訂例外
│   ├── grpc/              # gRPC 層
│   │   ├── ProductGrpcService.java     # gRPC Handler
│   │   └── client/
│   │       └── MediaGrpcClient.java    # Media Service 客戶端
│   ├── repository/        # 資料存取層
│   └── service/           # 業務邏輯層
│       ├── ProductService.java
│       ├── ProductCacheService.java
│       └── IdempotencyService.java
├── src/main/resources/
│   ├── application.yml
│   ├── logback-spring.xml
│   └── db/migration/      # Flyway 遷移腳本
│       └── V1__init_product_schema.sql
└── lombok.config
```

## 核心功能

### 1. gRPC API (4 個端點)

| RPC | 說明 |
|-----|------|
| `CreateProduct` | 建立商品（含冪等性） |
| `UpdateProduct` | 更新商品（含冪等性） |
| `GetProduct` | 取得商品詳情 |
| `GetProductList` | 取得商品列表（禁止 join product_sku） |

### 2. 冪等性設計

- 使用 `idempotency_request` 表追蹤請求狀態
- 狀態：`IN_PROGRESS` → `SUCCEEDED` / `FAILED`
- 重複請求返回先前結果，避免重複建立

### 3. Media Service 整合

- 透過 gRPC 呼叫 Media Service
- 支援 3 次重試，每次間隔 500ms
- 處理 `FinalizeImages`、`LinkImages`、`UnlinkImages`

### 4. 快取策略 (Cache-Aside)

| 快取類型 | Key 格式 | TTL |
|---------|---------|-----|
| 商品詳情 | `prod:detail:{id}` | 1 小時 |
| 商品列表 | `prod:list:{owner}:{hash}` | 5 分鐘 |

## 資料庫設計

### product 表
- `id` (UUID): 主鍵
- `name`: 商品名稱
- `rating`: 評分
- `status`: 狀態 (DRAFT/LISTED/UNLISTED)
- `category`: 分類 ID
- `gallery` (JSONB): 圖片列表
- `certifications` (JSONB): 認證列表
- `price_min`, `price_max`: 價格範圍

### product_sku 表
- `id` (UUID): 主鍵
- `product_id`: 所屬商品
- `sku_name`: SKU 名稱
- `weight`, `unit`: 重量與單位
- `price`: 價格
- `stock_qty`: 庫存
- `delivery_temp` (JSONB): 配送溫層
- `shipping_method` (JSONB): 運送方式

### idempotency_request 表
- `key` (VARCHAR): 主鍵
- `account_id`: 擁有者
- `endpoint`: 端點名稱
- `status`: 狀態
- `response_body` (JSONB): 回應內容

## 環境變數

| 變數 | 說明 | 預設值 |
|------|------|--------|
| `DB_HOST` | PostgreSQL 主機 | `127.0.0.1` |
| `DB_PORT` | PostgreSQL 埠 | `5432` |
| `DB_NAME` | 資料庫名稱 | `hawkr_product` |
| `REDIS_HOST` | Redis 主機 | `127.0.0.1` |
| `REDIS_PORT` | Redis 埠 | `6379` |
| `MEDIA_SERVICE_HOST` | Media Service 主機 | `127.0.0.1` |
| `MEDIA_SERVICE_GRPC_PORT` | Media Service gRPC 埠 | `9903` |
| `GRPC_PORT` | 本服務 gRPC 埠 | `9913` |

## 本地開發

```bash
# 啟動服務
cd services/java/product-service
mvn spring-boot:run

# 編譯
mvn clean package -DskipTests
```

## 層級分離規範

```
gRPC Handler → DTO → Service → Repository → Mapper/Converter → Entity
      ↓           ↓        ↓          ↓
    Proto      內部DTO   業務邏輯   資料存取
```

- Proto messages 僅用於傳輸層
- 內部使用 DTO 進行資料傳遞
- Entity 僅用於資料庫操作

## Observability

遵循 `observability-common` 規範：
- 使用 `ObsLogger` 進行結構化日誌
- 使用 `GrpcStubFactory` 建立 gRPC 客戶端（自動注入 trace context）
- 日誌格式：`<ClassName>.<action>.<result>`

## 禁止事項

- ❌ Variant / Spec 概念（統一使用 SKU）
- ❌ brand_id 欄位
- ❌ MQ/Outbox 機制
- ❌ GetProductList 中 join product_sku
- ❌ 直接使用 `new ManagedChannel` 或 gRPC Stub
