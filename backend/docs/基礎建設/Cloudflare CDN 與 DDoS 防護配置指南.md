# Cloudflare Gateway 配置指南

> 使用 Cloudflare 為 API Gateway 提供 CDN、DDoS 防護和 SSL

> ⚠️ **現況校正（2026-06-28）**：本指南描述較早期架構，部分已與現況不符——
> ① origin 埠實為 **gateway `:9900`**（非 `:8080`，前方 Traefik 終結 TLS）；
> ② prod WebSocket 走 **`wss://api.example.com`**（與 REST 同網域；`ws.example.com` 已不存在，`test-ws` 僅為 dev 殘留 DNS）；
> ③ prod 官網 `/api/v1/*` 經 Cloudflare Worker 同源反代。
> 完整且當前的拓撲 / 子網域 / 資料流以 [`系統架構與資料流向`](系統架構與資料流向.md) 為準。

---

## 一、架構概覽

```
                                    Cloudflare Network
┌─────────────────────────────────────────────────────────────────────────────┐
│                                                                              │
│   ┌──────────────┐    ┌──────────────┐    ┌──────────────┐                 │
│   │   Anycast    │    │     WAF      │    │     SSL      │                 │
│   │   CDN Edge   │───▶│   Firewall   │───▶│  Termination │                 │
│   └──────────────┘    └──────────────┘    └──────────────┘                 │
│          │                                        │                         │
└──────────┼────────────────────────────────────────┼─────────────────────────┘
           │                                        │
           ▼                                        ▼
    ┌─────────────┐                         ┌─────────────────┐
    │   Flutter   │                         │   Origin Server │
    │     App     │                         │   (VM4:8080)    │
    │             │                         │   Gateway       │
    └─────────────┘                         └─────────────────┘

用戶請求流程:
測試環境: Flutter App ──▶ test-api.example.com ──▶ Cloudflare ──▶ 203.0.113.20:8080
生產環境: Flutter App ──▶ api.example.com ──▶ Cloudflare ──▶ [生產服務器IP]:8080
```

---

## 二、Cloudflare 設置步驟

### 2.1 添加網站到 Cloudflare

1. 登入 [Cloudflare Dashboard](https://dash.cloudflare.com)
2. 點擊 **Add a Site**
3. 輸入域名：`example.com`
4. 選擇方案（Free 方案即可開始）
5. Cloudflare 會掃描現有 DNS 記錄

### 2.2 配置 DNS 記錄

添加以下 DNS 記錄：

**測試環境：**

| 類型 | 名稱 | 內容 | Proxy | TTL |
|------|------|------|-------|-----|
| A | `test-api` | `203.0.113.20` | ✅ Proxied | Auto |
| A | `test-ws` | `203.0.113.20` | ✅ Proxied | Auto |

**生產環境：**

| 類型 | 名稱 | 內容 | Proxy | TTL |
|------|------|------|-------|-----|
| A | `api` | `[生產服務器IP]` | ✅ Proxied | Auto |
| A | `ws` | `[生產服務器IP]` | ✅ Proxied | Auto |

**說明：**
- `test-api.example.com` → 測試環境 API Gateway (HTTP/HTTPS)
- `test-ws.example.com` → 測試環境 WebSocket IM Service
- `api.example.com` → 生產環境 API Gateway
- `ws.example.com` → 生產環境 WebSocket IM Service

### 2.3 更新域名 NS 記錄

在域名註冊商處將 NS 記錄改為 Cloudflare 提供的：
```
ns1.cloudflare.com
ns2.cloudflare.com
```

---

## 三、SSL/TLS 配置

### 3.1 SSL 模式選擇

進入 **SSL/TLS** → **Overview**：

| 模式 | 說明 | 推薦 |
|------|------|------|
| Off | 無 SSL | ❌ |
| Flexible | Cloudflare ↔ 用戶加密，源站不加密 | ⚠️ 不推薦 |
| Full | 全程加密，但不驗證源站證書 | ✅ 可用 |
| Full (Strict) | 全程加密 + 驗證源站證書 | ✅✅ 推薦 |

**推薦使用 Full (Strict)**，需要在源站配置 SSL 證書。

### 3.2 源站 SSL 證書配置

#### 方案 A：使用 Cloudflare Origin Certificate（推薦）

1. 進入 **SSL/TLS** → **Origin Server**
2. 點擊 **Create Certificate**
3. 選擇：
   - Private key type: RSA (2048)
   - Hostnames: `*.example.com`, `example.com`
   - Certificate Validity: 15 years
4. 下載證書和私鑰

在 VM4 上配置：

```bash
# SSH 到 VM4
ssh root@203.0.113.20

# 創建證書目錄
mkdir -p /etc/ssl/cloudflare

# 將證書和私鑰保存
cat > /etc/ssl/cloudflare/origin.pem << 'EOF'
-----BEGIN CERTIFICATE-----
[你的證書內容]
-----END CERTIFICATE-----
EOF

cat > /etc/ssl/cloudflare/origin.key << 'EOF'
-----BEGIN PRIVATE KEY-----REDACTED-----END PRIVATE KEY-----
EOF

# 設置權限
chmod 600 /etc/ssl/cloudflare/origin.key
chmod 644 /etc/ssl/cloudflare/origin.pem
```

#### 方案 B：使用 Let's Encrypt

```bash
# 安裝 certbot
apt update && apt install -y certbot

# 獲取證書（需要先將 DNS 指向服務器）
certbot certonly --standalone -d test-api.example.com -d test-ws.example.com
```

### 3.3 配置 Gateway HTTPS

修改 Gateway 配置以支持 HTTPS：

```go
// gateway-service/cmd/main.go

func main() {
    // ... 現有代碼 ...

    // HTTPS 配置
    tlsConfig := &tls.Config{
        MinVersion: tls.VersionTLS12,
    }

    server := &http.Server{
        Addr:      ":443",
        Handler:   router,
        TLSConfig: tlsConfig,
    }

    // 同時監聽 HTTP（用於健康檢查）
    go func() {
        http.ListenAndServe(":8080", router)
    }()

    // 啟動 HTTPS
    log.Fatal(server.ListenAndServeTLS(
        "/etc/ssl/cloudflare/origin.pem",
        "/etc/ssl/cloudflare/origin.key",
    ))
}
```

或者使用 Nginx 作為反向代理：

```nginx
# /etc/nginx/sites-available/test-api.example.com

server {
    listen 443 ssl http2;
    server_name test-api.example.com;

    ssl_certificate /etc/ssl/cloudflare/origin.pem;
    ssl_certificate_key /etc/ssl/cloudflare/origin.key;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;

    # 只允許 Cloudflare IP
    # https://www.cloudflare.com/ips/
    allow 173.245.48.0/20;
    allow 103.21.244.0/22;
    allow 103.22.200.0/22;
    allow 103.31.4.0/22;
    allow 141.101.64.0/18;
    allow 108.162.192.0/18;
    allow 190.93.240.0/20;
    allow 188.114.96.0/20;
    allow 197.234.240.0/22;
    allow 198.41.128.0/17;
    allow 162.158.0.0/15;
    allow 104.16.0.0/13;
    allow 104.24.0.0/14;
    allow 172.64.0.0/13;
    allow 131.0.72.0/22;
    deny all;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $http_cf_connecting_ip;
        proxy_set_header X-Forwarded-For $http_cf_connecting_ip;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Cloudflare 請求頭
        proxy_set_header CF-Connecting-IP $http_cf_connecting_ip;
        proxy_set_header CF-IPCountry $http_cf_ipcountry;
        proxy_set_header CF-Ray $http_cf_ray;
    }
}

# HTTP 重定向到 HTTPS
server {
    listen 80;
    server_name test-api.example.com;
    return 301 https://$server_name$request_uri;
}
```

---

## 四、Cloudflare 安全配置

### 4.1 WAF 規則

進入 **Security** → **WAF**：

#### 推薦規則集
- **Cloudflare Managed Ruleset**: 開啟
- **OWASP Core Ruleset**: 開啟（可設為 Log 模式先觀察）

#### 自定義規則

```
# 規則 1: 限制非必要 HTTP 方法
(http.request.method ne "GET" and
 http.request.method ne "POST" and
 http.request.method ne "PUT" and
 http.request.method ne "DELETE" and
 http.request.method ne "OPTIONS")
→ Block

# 規則 2: 阻止常見攻擊路徑
(http.request.uri.path contains "wp-admin" or
 http.request.uri.path contains ".env" or
 http.request.uri.path contains "phpinfo" or
 http.request.uri.path contains ".git")
→ Block

# 規則 3: 保護管理端點（如果有）
(http.request.uri.path contains "/admin" and
 not ip.src in {你的辦公室IP})
→ Block
```

### 4.2 Rate Limiting

進入 **Security** → **WAF** → **Rate limiting rules**：

```
# 規則 1: API 通用限流
URI Path: /api/*
Requests: 100 requests per 1 minute
Action: Block for 1 minute

# 規則 2: 登入限流（更嚴格）
URI Path: /api/auth/login
Requests: 5 requests per 1 minute
Action: Block for 5 minutes

# 規則 3: 驗證碼限流
URI Path: /api/auth/sms-code
Requests: 3 requests per 1 minute
Action: Block for 10 minutes
```

### 4.3 Bot Management（付費功能）

如果有 Pro 方案，啟用 Bot Fight Mode：
- **Security** → **Bots** → **Bot Fight Mode**: On

### 4.4 Under Attack Mode

遭受攻擊時：
- **Overview** → **Under Attack Mode**: On
- 會顯示 5 秒 JavaScript 挑戰

---

## 五、WebSocket 配置

### 5.1 啟用 WebSocket

進入 **Network**：
- **WebSockets**: On

### 5.2 WebSocket 超時設置

Cloudflare 默認 WebSocket 超時為 100 秒，需要心跳保持連接：

```dart
// Flutter 端 - 30 秒心跳
Timer.periodic(Duration(seconds: 30), (_) {
  if (_isConnected) {
    _channel.sink.add(jsonEncode({'type': 'heartbeat'}));
  }
});
```

### 5.3 WebSocket DNS 記錄

添加專門的 WebSocket 子域名：

| 類型 | 名稱 | 內容 | Proxy |
|------|------|------|-------|
| A | `ws` | `203.0.113.20` | ✅ Proxied |

前端配置：
```dart
// lib/config/api_config.dart
static const String prodWsUrl = 'wss://ws.example.com/ws';
```

---

## 六、性能優化

### 6.1 緩存規則

> ⚠️ **本節是當初的建議配置，不是線上現況**（下面這兩條規則線上並不存在）。
> 「現在實際生效的 cache rule 是什麼」唯一真實來源＝`k8s/external-services/cloudflare/cache-rules.md`
> ＋機器可讀快照 `k8s/external-services/cloudflare/rulesets/http_request_cache_settings.json`。
> 要改快取行為請走《Cloudflare Worker 與 Serverless 部署治理規範》§5 的流程（改完必回寫快照）。

進入 **Caching** → **Cache Rules**：

```
# 規則 1: 不緩存 API 響應
URI Path: /api/*
Cache eligibility: Bypass cache

# 規則 2: 緩存靜態資源（如果 Gateway 提供）
URI Path: /static/*
Cache eligibility: Eligible for cache
Edge TTL: 1 month
Browser TTL: 1 week
```

### 6.2 Page Rules（如果需要更細緻控制）

```
# 規則 1: API 完全繞過緩存
URL: api.example.com/api/*
Settings:
  - Cache Level: Bypass
  - Disable Performance

# 規則 2: 健康檢查
URL: api.example.com/health
Settings:
  - Cache Level: Bypass
```

### 6.3 Argo Smart Routing（付費）

如果延遲很重要，考慮啟用 Argo：
- **Traffic** → **Argo** → Enable
- 可減少 30% 以上延遲

---

## 七、監控與分析

### 7.1 Analytics

進入 **Analytics & Logs**：
- 查看請求量、帶寬、緩存命中率
- 監控威脅和已阻止的請求

### 7.2 Logs（Enterprise）

如果是 Enterprise 方案，可以啟用 Logpush：
- 將日誌推送到 S3/GCS/Azure Blob

### 7.3 Health Checks

進入 **Traffic** → **Health Checks**：

```
Name: gateway-health
Address: 203.0.113.20
Port: 8080
Path: /health
Interval: 60 seconds
```

---

## 八、Flutter 前端配置更新

### 8.1 更新 API 配置

```dart
// lib/config/api_config.dart

class ApiConfig {
  // 生產環境 - 透過 Cloudflare
  static const String prodBaseUrl = 'https://api.example.com';
  static const String prodWsUrl = 'wss://ws.example.com/ws';

  // 測試環境 - 透過 Cloudflare
  static const String testBaseUrl = 'https://test-api.example.com';
  static const String testWsUrl = 'wss://test-ws.example.com/ws';

  // 本地開發 - 直連 VM
  static const String devBaseUrl = 'http://203.0.113.20:8080';
  static const String devWsUrl = 'ws://203.0.113.20:8302/ws';

  // OSS 圖片域名
  static const String ossEndpoint = 'https://media.example.com';

  static String get baseUrl {
    if (kDebugMode) return devBaseUrl;      // 本地開發
    if (kProfileMode) return testBaseUrl;   // 測試環境
    return prodBaseUrl;                      // 生產環境
  }

  static String get wsUrl {
    if (kDebugMode) return devWsUrl;
    if (kProfileMode) return testWsUrl;
    return prodWsUrl;
  }
}
```

### 8.2 處理 Cloudflare 錯誤頁面

```dart
// lib/services/interceptors/cloudflare_interceptor.dart

class CloudflareInterceptor extends Interceptor {
  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    final response = err.response;

    // Cloudflare 錯誤頁面
    if (response != null) {
      final body = response.data?.toString() ?? '';

      // 503 Service Unavailable (Under Attack Mode)
      if (response.statusCode == 503 && body.contains('Cloudflare')) {
        // 提示用戶稍後重試
        return handler.reject(DioException(
          requestOptions: err.requestOptions,
          error: 'Service temporarily unavailable, please retry',
          type: DioExceptionType.badResponse,
        ));
      }

      // 1020 Access Denied (WAF 阻止)
      if (body.contains('1020') || body.contains('Access denied')) {
        return handler.reject(DioException(
          requestOptions: err.requestOptions,
          error: 'Request blocked by security rules',
          type: DioExceptionType.badResponse,
        ));
      }

      // 429 Rate Limited
      if (response.statusCode == 429) {
        return handler.reject(DioException(
          requestOptions: err.requestOptions,
          error: 'Too many requests, please slow down',
          type: DioExceptionType.badResponse,
        ));
      }
    }

    handler.next(err);
  }
}
```

---

## 九、DNS 切換計劃

### 9.1 準備階段

1. ✅ 在 Cloudflare 添加域名
2. ✅ 配置所有 DNS 記錄
3. ✅ 配置 SSL 證書
4. ✅ 設置 WAF 規則
5. ✅ 配置 Rate Limiting

### 9.2 測試階段

1. 修改本地 hosts 文件測試：
   ```
   # /etc/hosts (Mac/Linux) 或 C:\Windows\System32\drivers\etc\hosts (Windows)
   104.21.x.x test-api.example.com
   104.21.x.x test-ws.example.com
   ```

2. 測試所有 API 端點
3. 測試 WebSocket 連接
4. 測試 SSL 證書

### 9.3 切換階段

1. 在域名註冊商更改 NS 記錄
2. 等待 DNS 傳播（24-48 小時）
3. 監控 Cloudflare Analytics
4. 確認無問題後刪除測試 hosts 記錄

---

## 十、成本估算

| 方案 | 價格 | 包含功能 |
|------|------|----------|
| Free | $0/月 | 基本 CDN、SSL、DDoS 防護、有限 WAF |
| Pro | $20/月 | 進階 WAF、Image Optimization、Argo |
| Business | $200/月 | 100% SLA、WAF 自定義規則、Railgun |
| Enterprise | 定價 | 專屬支持、進階安全、日誌 |

**建議：** 從 Free 方案開始，如果需要進階 WAF 或 Rate Limiting 再升級到 Pro。

---

**文檔版本**: 1.0
**最後更新**: 2025-12-24
**維護者**: Hawkr Backend Team
