# Media Service（多媒體微服務）

管理靜態資源（Images/Videos）的生命週期與存取權限。

## 服務職責

- 與雲端物件儲存（Aliyun OSS）交互：簽發上傳憑證
- 機敏檔案流式處理：AES 邊收邊加密邊傳至 OSS
- Staging 機制：管理「暫存」到「正式」的狀態轉換
- 引用計數（Reference Counting）：記錄哪些業務實體正在使用該圖片
- 垃圾回收（Garbage Collection）：定期清除過期或無人引用的檔案

## 技術棧

- Java 21
- Spring Boot 3.2.1
- gRPC（grpc-spring-boot-starter 3.1.0）
- PostgreSQL（MyBatis-Plus）
- Aliyun OSS SDK
- JWT（jjwt 0.12.5）

## 快速開始

### 前置條件

1. JDK 21
2. PostgreSQL 14+
3. Maven 3.8+
4. 阿里雲 OSS（可選，開發模式可不配置）

### 環境變數

| 變數名 | 說明 | 預設值 |
|--------|------|--------|
| `DB_HOST` | 資料庫主機 | `127.0.0.1` |
| `DB_PORT` | 資料庫端口 | `5432` |
| `DB_NAME` | 資料庫名稱 | `media` |
| `DB_USER` | 資料庫使用者 | `dbadmin` |
| `DB_PASSWORD` | 資料庫密碼 | `Hawkr2024Admin` |
| `GRPC_PORT` | gRPC 服務端口 | `9903` |
| `OSS_ENDPOINT` | OSS Endpoint | - |
| `OSS_ACCESS_KEY_ID` | OSS Access Key ID | - |
| `OSS_ACCESS_KEY_SECRET` | OSS Access Key Secret | - |
| `OSS_BUCKET_NAME` | OSS Bucket 名稱 | `hawkr-media` |
| `MEDIA_BASE_URL` | 媒體基礎 URL | - |
| `TOKEN_SECRET` | JWT 簽名密鑰（至少 32 字元） | 內建預設值 |
| `GC_ENABLED` | 是否啟用 GC | `true` |

### 資料庫初始化

服務使用 Flyway 自動執行 Migration：

```bash
# 建立資料庫
createdb -U dbadmin media

# 啟動服務後自動執行 Migration
```

### 啟動服務

```bash
cd services/java/media-service

# 編譯
mvn clean package -DskipTests

# 執行
java -jar target/media-service-1.0.0.jar
```

### 端口說明

| 端口 | 用途 |
|------|------|
| `9936` | HTTP REST API |
| `9934` | gRPC 服務 |

## API 介面

### HTTP API（供前端呼叫，透過 Gateway）

前端統一透過 Gateway 呼叫，Gateway 會去除 `/api/v1` 前綴後轉發至本服務。

| 前端呼叫路徑 | 本服務路徑 | 說明 |
|-------------|-----------|------|
| `POST /api/v1/media/upload-sessions` | `/media/upload-sessions` | 申請上傳會話 |
| `POST /api/v1/media/upload-sessions/{id}/complete` | `/media/upload-sessions/{id}/complete` | 完成上傳（公開上傳） |
| `POST /api/v1/media/secure_upload/{id}` | `/media/secure_upload/{id}` | 機敏檔案流式上傳 |
| `GET /api/v1/media/secure-download/{assetId}` | `/media/secure-download/{assetId}` | 機敏檔案流式下載 |

> **注意**：所有 HTTP API 需攜帶 JWT Token（`Authorization: Bearer <token>`），Gateway 會驗證後注入 `X-Account-Id` Header。

### gRPC API（供內部服務呼叫）

```protobuf
service MediaService {
    rpc FinalizeImages(FinalizeImagesRequest) returns (FinalizeImagesResponse);
    rpc LinkImages(LinkImagesRequest) returns (LinkImagesResponse);
    rpc UnlinkImages(UnlinkImagesRequest) returns (UnlinkImagesResponse);
}
```

## 業務流程

### 公開資源上傳流程

```
1. 前端 → POST /api/v1/media/upload-sessions → 取得 OSS 簽名 URL
2. 前端 → 直傳 OSS
3. 前端 → POST /api/v1/media/upload-sessions/{id}/complete → 取得 staged_token
4. 業務服務 → gRPC FinalizeImages → 轉正為正式資產
5. 業務服務 → gRPC LinkImages → 建立引用關係
```

> `ref_type` 對外契約使用 `AssetReferenceType.value()`；但 `object_key` 路徑中的分類目錄仍使用對應 `code().toLowerCase()`，例如輸入 `7104` 仍會落到 `shipping/...`。

### 機敏資源上傳流程

```
1. 前端 → POST /api/v1/media/upload-sessions (isSecure=true)
2. 前端 → POST /api/v1/media/secure_upload/{id} → 流式加密上傳 → 取得 staged_token
3. 業務服務 → gRPC FinalizeImages → 轉正為正式資產
4. 業務服務 → gRPC LinkImages → 建立引用關係
```

## GC（垃圾回收）

預設配置：
- **執行時間**：每日凌晨 3 點
- **過期 Session 清理**：狀態為 INIT/COMPLETED 且已過期
- **孤兒 Asset 清理**：無引用且創建超過 7 天

可透過環境變數調整：

```yaml
media:
  gc:
    enabled: true           # 啟用/停用 GC
    cron: "0 0 3 * * ?"     # Cron 表達式
    orphan-asset-days: 7    # 孤兒保留天數
    batch-size: 100         # 批次處理大小
```

## 資料庫 Schema

### upload_session（上傳會話表）

| 欄位 | 類型 | 說明 |
|------|------|------|
| `id` | UUID | 會話 ID |
| `account_id` | BIGINT | 上傳者 ID |
| `object_key` | VARCHAR(512) | OSS 路徑 |
| `is_secure` | BOOLEAN | 是否機敏檔案 |
| `status` | VARCHAR(4) | 狀態碼（7001-7004） |
| `expires_at` | TIMESTAMPTZ | 有效期 |

### media_asset（資源資產表）

| 欄位 | 類型 | 說明 |
|------|------|------|
| `id` | BIGINT | 資產 ID（TSID） |
| `uploader_account_id` | BIGINT | 上傳者真實登入 account_id（一般買/賣家/後台上傳，`createMediaAsset` 寫入）；系統直傳與匯入轉正列為 NULL |
| `import_store_id` | BIGINT | 賣家 store_id，僅 CSV/URL 商品匯入管線轉正（`ImportAssetService.commitFinalize`）寫入；其餘為 NULL |
| `object_key` | VARCHAR(512) | OSS 路徑 |
| `ref_type` | VARCHAR(64) | 資源分類（Finalize 時設定，對應 AssetReferenceType） |
| `is_secure` | BOOLEAN | 是否機敏檔案 |
| `encryption_key` | VARCHAR(256) | 加密金鑰（機敏專用） |

### asset_reference（引用關聯表）

| 欄位 | 類型 | 說明 |
|------|------|------|
| `id` | BIGSERIAL | 主鍵 |
| `asset_id` | UUID | 資產 ID（FK） |
| `ref_type` | VARCHAR(64) | 業務類型 |
| `ref_id` | VARCHAR(64) | 業務 ID |

## 狀態碼（CodeMeta）

### MediaUploadSessionStatus

| 代碼 | 值 | 說明 |
|------|-----|------|
| INIT | 7001 | 已建立上傳會話 |
| COMPLETED | 7002 | 已完成上傳等待轉正 |
| FINALIZED | 7003 | 已轉正為正式資產 |
| EXPIRED | 7004 | 會話過期可由 GC 清理 |

### AssetReferenceType

| 代碼 | 值 | 說明 |
|------|-----|------|
| PRODUCT | 7101 | 商品服務引用圖片 |
| PRODUCT_DESC | 7102 | 商品描述圖片引用 |
| SELLER_LOGO | 7103 | 賣家 logo 引用 |
| SHIPPING | 7104 | 物流相關圖片引用 |
| SELLER_COVER | 7105 | 賣家封面圖片引用 |
| IDENTITY_VERIFICATION | 7106 | 使用者實名驗證圖片引用 |
| CHAT_MESSAGE | 7107 | 聊天訊息圖片引用 |
| POST | 7108 | 貼文圖片引用 |
| BANNER | 7109 | Banner 圖片引用 |
| AFTERSALE | 7110 | 售後證據照片引用 |
| CERTIFICATION | 7111 | 商家認證文件引用 |
| BUYER_AVATAR | 7112 | 買家頭像引用 |
| OTHER | 7199 | 未分類圖片引用 |

## 重構摘要

本次重構完全依據設計書重新實作，移除所有舊有遺留：

### 已移除的舊 API

- `GetPresignedUrl`
- `UploadFile`
- `DeleteFile`
- `GetFileInfo`

### 新增/修正的功能

1. **FinalizeImages 重入語意**：已 FINALIZED 的 token 不報錯，回傳既有資產
2. **LinkImages/UnlinkImages 冪等**：使用 `ON CONFLICT DO NOTHING`
3. **GC 7 天緩衝期**：防止誤刪剛建立但尚未建立引用的資產
4. **機敏檔案流式處理**：secure_upload/secure-download 接口
5. **跨服務契約對齊**：FinalizedAsset 包含 `is_secure` 欄位

## 測試

### 單元測試

```bash
# 執行單元測試
mvn test

# 測試覆蓋項目
- FinalizeImages 重入/冪等行為
- Link/Unlink 冪等/重試安全
- GC 條件測試（過期 Session、孤兒 Asset、7 天緩衝）
```

### 整合測試（Gateway + Media Service）

#### 前置條件

1. 啟動 Gateway（預設 port 9900）
2. 啟動 Media Service（預設 HTTP port 9936）
3. 取得有效的 JWT Token

```bash
# 1. 取得 JWT Token（透過登入 API）
TOKEN=$(curl -s -X POST http://localhost:9900/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"phone":"0912345678","password":"test123"}' | jq -r '.data.accessToken')

echo "Token: $TOKEN"
```

#### 測試上傳會話

```bash
# 2. 建立上傳會話（公開檔案）
curl -X POST http://localhost:9900/api/v1/media/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "fileName": "test-image.jpg",
    "contentType": "image/jpeg",
    "isSecure": false
  }'

# 回應範例：
# {
#   "code": 200,
#   "data": {
#     "sessionId": "550e8400-e29b-41d4-a716-446655440000",
#     "uploadUrl": "https://oss.example.com/...",
#     "objectKey": "public/2026/02/02/xxx.jpg",
#     "expiresAt": "2026-02-02T12:00:00Z"
#   }
# }
```

#### 測試機敏檔案上傳

```bash
# 3. 建立機敏上傳會話
SESSION_RESPONSE=$(curl -s -X POST http://localhost:9900/api/v1/media/upload-sessions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "fileName": "id-card.jpg",
    "contentType": "image/jpeg",
    "isSecure": true
  }')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.data.sessionId')
echo "Session ID: $SESSION_ID"

# 4. 流式上傳機敏檔案
curl -X POST "http://localhost:9900/api/v1/media/secure_upload/$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/path/to/test-file.jpg"

# 回應範例：
# {
#   "code": 200,
#   "data": {
#     "stagedToken": "<stagedToken>"
#   }
# }
```

#### 驗證路由正確性

```bash
# 直接呼叫 Media Service（繞過 Gateway，用於除錯）
curl -X POST http://localhost:9936/media/upload-sessions \
  -H "X-Account-Id: 1" \
  -H "Content-Type: application/json" \
  -d '{
    "fileName": "direct-test.jpg",
    "contentType": "image/jpeg",
    "isSecure": false
  }'

# 如果回應正常，表示 Media Service 路徑配置正確
```

#### 健康檢查

```bash
# Gateway 健康檢查
curl http://localhost:9900/health

# Media Service 健康檢查
curl http://localhost:9936/health
```

## 相關文件

- [多媒體微服務設計書](../../docs/詳細設計書/08-多媒體微服務（Media%20Service）詳細設計書.md)
- [Observability 規範](../../services/java/shared/observability-common/README.md)
- [CodeMeta 定義](../../services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/)
