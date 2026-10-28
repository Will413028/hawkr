# Redis 快取管理

## 1. 概述

本文件記錄系統中所有 Redis 使用情況（含非快取類的 session / 限流 / 序號）、開發階段清除快取的 API、除錯指引，以及 **§5 容量與失效模型（512MB 容量契約）**。

> 本檔的 key 清冊同時是容量契約的輸入：**新增任何 key 前綴必須登記在 §2**，否則 §5 的預算算不準（2026-07-30 覆核時發現 8 類線上 key 未登記，見 §2.6/§2.7）。

---

## 2. 快取清單

### 2.1 Product Service

| Key Pattern | 範例 | TTL | 用途 | 寫入時機 | 清除時機 |
|-------------|------|-----|------|---------|---------|
| `prod:detail:{productId}` | `prod:detail:816291586719387592` | 1 小時 | 商品詳情（Cache-Aside） | 首次查詢 cache miss 時 | 商品更新/刪除時 + **庫存變更（扣減/歸還）時** |
| `prod:list:{ownerId}:{queryHash}` | `prod:list:2628080754:a3b1c9d2` | 5 分鐘 | 商品列表查詢結果 | 列表查詢 cache miss 時 | 商品更新/新增/刪除時 |

**注意**：`queryHash` 為查詢條件的 MD5 前 8 碼。

### 2.2 Cart Service

| Key Pattern | 範例 | TTL | 用途 | 寫入時機 | 清除時機 |
|-------------|------|-----|------|---------|---------|
| `cart:a:{accountId}` | `cart:a:12345` | 7 天 | 登入用戶購物車（Hash 結構，field=skuId） | 加入/修改/查詢購物車時 | 合併購物車、清空購物車時 |
| `cart:g:{guestId}` | `cart:g:uuid-xxx` | 7 天 | 訪客購物車（Hash 結構，field=skuId） | 訪客加入/修改購物車時 | 合併購物車後刪除 |
| `seller:meta:{sellerAccountId}` | `seller:meta:2628080754` | 1 天 | 賣家名稱 + logoUrl JSON | 查詢購物車列表時 | 賣家資訊更新時（目前無自動清除） |
| `merge_lock:{guestId}:{accountId}` | `merge_lock:uuid:12345` | 10 秒 | MergeCart 操作互斥鎖 | mergeCart 開始時 | 自動過期 |

### 2.3 Coupon Service

| Key Pattern | 範例 | TTL | 用途 | 寫入時機 | 清除時機 |
|-------------|------|-----|------|---------|---------|
| `coupon:tpl:{templateId}` | `coupon:tpl:99001` | 30 分鐘 | 優惠券模板詳情 | 查詢 cache miss 時 | 模板更新時 |
| `coupon:avail:*` | `coupon:avail:abc123` | 5 分鐘 | 可領取優惠券列表 | 列表查詢 cache miss 時 | 模板狀態變更時（批次清除） |

### 2.4 Buyer Service

| Key Pattern | 範例 | TTL | 用途 | 寫入時機 | 清除時機 |
|-------------|------|-----|------|---------|---------|
| `fav:set:{buyerId}` | `fav:set:2001` | 無（持久） | 用戶收藏商品 ID 集合（SET 結構） | 冷啟動 cache miss 時從 DB 載入全量 | 按讚/取消時即時 SADD/SREM |

**注意**：
- 此 SET 為持久快取（無 TTL），Redis 是讀取的主要來源，DB 是持久化備份
- 冷啟動：首次查詢時若 key 不存在，會從 `favorites` 表載入所有 `product_id` 並 `SADD` 到 Redis
- 用戶收藏數量通常在數百筆以內，SET 佔用記憶體極小
- Redis 不可用時自動降級到 DB 直接查詢

### 2.5 Order Service

| Key Pattern | 範例 | TTL | 用途 | 寫入時機 | 清除時機 |
|-------------|------|-----|------|---------|---------|
| `order:seq:{yyyyMMdd}` | `order:seq:20260322` | **無（實測 TTL=-1）** | 訂單流水號自增序列 | 建立訂單時 INCR | 不會自動過期 |
| `checkout:{checkoutId}` | `checkout:550e8400-e29b-41d4-a716-446655440000` | 30 分鐘 | 結帳批次有效性驗證（一次性使用） | Preview 產生 checkoutId 時 | Confirm 驗證後刪除 / 自動過期 |

**注意**：`order:seq` 為 counter（INCR），清除後當日序號會重置，通常**不需要清除**。
**注意（2026-07-30 實測修正）**：前綴是 `order:seq:`（`OrderConstants.REDIS_KEY_ORDER_SEQ`），非舊記載的 `order_seq:`；且**沒有 TTL**（dev 8 把 key 全數 `TTL=-1`，最舊 `order:seq:20260310`）。每天一把、每把 37 bytes 永久累積，屬 §5 的無界成長項（量級可忽略但仍無界）。
**注意**：`checkout:*` 為一次性 token，Confirm 成功後自動刪除。若測試需要重複 Confirm，須重新呼叫 Preview 取得新的 checkoutId。

### 2.6 Gateway（**非快取**：session 與限流）

| Key Pattern | 範例 | TTL | 用途 | 實作位置 |
|-------------|------|-----|------|---------|
| `session:{sha256(token)}` | `session:9f86d081884c7d65…`（64 字小寫 hex） | 7 天（滑動窗，剩餘低於門檻才續期） | **Web 登入 session（不能失敗的狀態）** | `pkg/redis/session.go`（`SessionKey` / `PutSession` / `GetSessionWithTTL`） |
| `ratelimit:{tier}:{ip}` | `ratelimit:moderate:2a06:…` | 秒級（實測剩餘 50s） | 一般限流計數 | `gateway/internal/middleware/ratelimit.go:250` |
| `ratelimit:authfail:{ip}` | `ratelimit:authfail:1.2.3.4` | 秒級 | 認證失敗獨立限流 | `gateway/internal/middleware/ratelimit.go:133` |

**注意**：`session:*` **不在** §3 清除 API 的任何 group 內（`gateway/internal/cache/redis.go:18-26`），flush 不會登出使用者——這是刻意的。session 是本檔唯一「丟了就是線上故障」的資料，容量與逐出風險見 §5。

**注意（#2049）**：鍵是 token 的 **sha256**，不是 token 本身。`SCAN session:*` / `KEYS session:*` 因此**不再洩漏可冒用的憑證**（舊格式直接把 bearer token 印在 keyspace 上，讀得到 keyspace 即等於帳號接管）。反過來說，要對照某把 key 屬於誰，只能拿 `web_sessions.token_hash` 去比對（帳號級撤銷也是靠這點才逐得掉快取，見 17 號設計書 §3）；持有原始 cookie 值時則自行 `sha256` 後查。

### 2.7 其他服務

| Key Pattern | TTL（2026-07-30 dev 實測） | 用途 | 實作位置 |
|-------------|------|------|---------|
| `im:account:deleted:{accountId}` | **無（TTL=-1）** | 帳號已刪除標記 | `services/im-service/internal/usecase/message.go:906` |
| `cat:all` | ≈10 分鐘 | 整張商品分類表（**單把 512KB，見 §5**） | `services/java/product-service/.../CacheKeyConstants.java:26` |
| `seller:violations:public:*` | — | 賣家違規公開資訊 | 清除 API group `seller` |
| `search:hot_keywords` | **無（TTL=-1）** | **孤兒 key**：repo 內零引用，search-service 已退役；1,177 bytes 永久殘留 | 無（可安全刪除，屬維運動作） |

---

## 3. 快取清除 API（開發用）

### 3.1 清除所有快取

```
GET /api/v1/public/caches/flush
```

清除所有服務的 Redis 快取（不含 order_seq 流水號）。

**回應**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "product": 3,
    "cart": 5,
    "coupon": 2,
    "total": 10
  }
}
```

`data` 中各欄位為該服務清除的 key 數量。

### 3.2 清除指定服務快取

```
GET /api/v1/public/caches/flush/:service
```

**service 參數**：

| 值 | 清除的 Key Pattern |
|----|-------------------|
| `product` | `prod:detail:*`、`prod:list:*` |
| `cart` | `cart:a:*`、`cart:g:*`、`seller:meta:*` |
| `coupon` | `coupon:tpl:*`、`coupon:avail:*` |

**回應**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "product": 3,
    "total": 3
  }
}
```

**錯誤回應**（無效 service）：
```json
{
  "code": 400,
  "message": "無效的服務名稱，可用值：product, cart, coupon"
}
```

---

## 4. 除錯指引

### 4.1 何時懷疑快取問題

遇到以下現象時，應優先考慮 Redis 快取：

1. **DB 已更新但 API 回傳舊資料** — 最常見的快取問題
2. **直接改 DB 後行為不符預期** — DB 修改不會觸發快取清除
3. **間歇性資料不一致** — 部分請求 cache hit（舊資料）、部分 cache miss（新資料）
4. **重啟微服務後問題消失** — 可能是快取 TTL 還沒過期

### 4.2 除錯步驟

```bash
# 1. 查看特定商品的快取（本地）
docker exec hawkr-redis redis-cli -a $REDIS_PASSWORD GET "prod:detail:{productId}"

# 2. 查看特定商品的快取（遠端）
docker exec hawkr-redis redis-cli -h $REDIS_HOST -a $REDIS_PASSWORD GET "prod:detail:{productId}"

# 3. 清除特定 key
docker exec hawkr-redis redis-cli -a $REDIS_PASSWORD DEL "prod:detail:{productId}"

# 4. 查看所有商品快取 key
docker exec hawkr-redis redis-cli -a $REDIS_PASSWORD KEYS "prod:*"

# 5. 一鍵清除（透過 API）
curl http://localhost:9900/api/v1/public/caches/flush
curl http://localhost:9900/api/v1/public/caches/flush/product
```

### 4.3 注意事項

- 清除快取是**安全操作**，最壞情況只是下次查詢多一次 DB 讀取
- `order:seq` 流水號不會被清除，避免訂單編號重複
- `merge_lock` 互斥鎖 TTL 只有 10 秒，通常不需清除
- 修改遠端 DB 後，**遠端 Redis 也需要清除**（本地和遠端是獨立的 Redis 實例）
- `session:*` 不在任何 flush group 內，清快取不會登出使用者

---

## 5. 容量與失效模型（容量契約）

> 本節是 `docs/開發規範/硬上限依賴接入治理規範（配額與容量）.md` §5 登記表 R1 / R2 所指的「預算數字」。
> 所有數字為 **2026-07-30 實測**：dev 走 Infra VM `docker exec hawkr-redis redis-cli INFO / MEMORY USAGE / TTL`；prod 走 dev-infra Prometheus API（`redis_*` 指標，env label 區分）。**修改本節數字前必須重測，不得沿用。**

### 5.1 供給（硬上限）

| 項目 | 值 | 來源 |
|---|---|---|
| `maxmemory` | 512 MB（536,870,912 bytes） | `infra/docker-compose.yml:70`；dev/prod 兩實例實測皆為此值 |
| `maxmemory-policy` | `allkeys-lru` | 同上 |
| 持久化 | `--appendonly yes`（AOF），`aof_last_bgrewrite_status:ok` | 同上 + `INFO persistence` |
| `maxclients` | 10,000（預設） | `INFO clients` |

**`allkeys-lru` 的語意**：記憶體達 `maxmemory` 時，Redis 從**全部 key**（不只有 TTL 的）挑近似 LRU 逐出。它**不區分 session 與快取**——快取長大就會逐出 session。

### 5.2 需求（現況用量）

| 指標 | dev | prod |
|---|---|---|
| `used_memory` | 2,693,080 B（2.57 MB）＝ **0.50%** | 2,935,928 B（2.80 MB）＝ **0.55%** |
| `used_memory_peak` | 7.36 MB（1.44%） | — |
| db0 key 數 | 76 | 24 |
| `evicted_keys` | **0**（uptime 34.6 天） | **0**（uptime 87.3 天） |
| `expired_keys` | 20,127 | 407,671 |
| `connected_clients` | 26 | 15 |
| 指令速率 | 1.32 cmd/s 平均（`sum(redis_commands_total)` 3,961,513 ÷ 2,992,033 s） | — |

**逐出從未發生過**（兩環境 `redis_evicted_keys_total` 皆為 0）——目前 §5.4 的風險是**理論風險，尚未實現**。

### 5.3 單位成本（每類 key 佔多少）

`MEMORY USAGE` 實測（dev 全 76 把 key 逐一量測）：

| 類別 | 每把 bytes | 現有把數 | 小計 |
|---|---|---|---|
| `cat:all` | **524,322** | 1 | 524,322（**佔全部資料的 92.3%**） |
| `prod:list:*` | 118 ~ 12,339 | 5 | 35,205 |
| `cart:a:*`（hash） | 1,828 | 1 | 1,828 |
| `im:account:deleted:*` | 48 | 33 | 1,584 |
| `search:hot_keywords`（zset） | 1,177 | 1 | 1,177 |
| `ban:lookup:*` | 270 ~ 302 | 3 | 842 |
| `addr:val` / `addr:sug` | 287 ~ 359 | 2 | 646 |
| `seller:meta:*` | 161 ~ 209 | 3 | 579 |
| `fav:set:*`（set） | 53 ~ 269 | 14 | ≈1,200 |
| **`session:*`** | **128 ~ 144** | 4 | **560（佔 0.1%）** |
| `order:seq:*` | 37 | 8 | 296 |
| `ratelimit:*` | 67 | 1 | 67 |
| **資料合計** | | 76 | **≈ 568 KB** |

`used_memory` 2.57 MB − 資料 0.57 MB ≈ **2.0 MB 為 Redis 自身固定開銷**（AOF buffer、client buffer、字典等）。

### 5.4 預算推導：512MB 能撐多少？離上限多遠？

以 `used_memory` 2.57 MB 為固定佔用，剩餘可用 ≈ 534,177,832 bytes：

| 問題 | 答案 | 算法 |
|---|---|---|
| 512MB 能撐多少併發 session？ | **≈ 371 萬**（3,709,568） | 534,177,832 ÷ 144 B（實測 session 上界） |
| 現況離上限多遠？ | **199 倍頭寸**（用了 0.50%） | 2.57 MB / 512 MB |
| 什麼會先吃掉 512MB？ | **不是 session** | session 4 把＝560 B＝全部資料的 0.1% |
| `prod:list` 要多少把才逼近上限？ | **≈ 43,300 把同時存活** | 534,177,832 ÷ 12,339 B（實測最大值）；key 基數 ＝ ownerId × queryHash 組合、TTL 5 分鐘，**基數本身無上限** |
| 無 TTL 的 key 呢？ | 隨帳號數線性成長，**無界** | `fav:set:*`（每買家 1 把，53~269 B）＋ `im:account:deleted:*`（每刪除帳號 1 把，48 B）＋ `order:seq:*`（每天 1 把）皆 `TTL=-1`。以 100 萬買家、平均 100 B 估：`fav:set` 單類即約 100 MB ＝ 上限的 20% |

**結論**：session 不是容量壓力來源，**快取才是**；而快取一旦把記憶體推到上限，`allkeys-lru` 逐出的可能正是 session。

### 5.5 失效模型（上限到達時系統怎麼表現）

| 階段 | 現象 | 可觀測性 |
|---|---|---|
| < 85% | 正常 | `RedisMemoryHigh` 未觸發 |
| 趨勢逼近 | 6 小時斜率外推 24 小時後 > 85% | **`RedisMemoryExhaustionPredicted`（warning）** |
| ≥ 85% | 預警 | `RedisMemoryHigh`（warning） |
| = 100% | **開始逐出**：快取與 session 一視同仁被丟 | **`RedisKeysEvicted`（critical）** |
| session 被逐出後 | 使用者需重新登入 | gateway 側「`redis.Nil` 當成確實登出」已由 #2014 治本 A 消除；但 **session 本身確實消失，重登不可避免** |
| Redis 整體連不上 | 寫 session 失敗 → 新登入全滅 | **`RedisDown`（critical）** |

告警定義位置：`monitoring/prometheus/rules/infra-alerts.yml` 的 `redis-alerts` group。

### 5.6 維運待辦（**不進 codebase**，由後端工程師戴維運帽子執行）

按 `docs/開發規範/後端服務責任邊界規範.md`（§22），以下屬一次性維運動作，禁止寫成 application code（backfill / 巡檢 job 一律拒絕）：

| # | 動作 | 誰做 | 何時 |
|---|---|---|---|
| 1 | 讓本次新增的 3 條 Redis 告警生效：Infra VM（203.0.113.10）`cd /root/backend-payuni && git pull` → `docker restart hawkr-prometheus`。**規則檔不在 CI 部署路徑上**——Prometheus 直接 bind-mount `/root/backend-payuni/monitoring/prometheus/rules`（`docker inspect` 實測），且未帶 `--web.enable-lifecycle`（`/-/reload` 不可用，只能重啟）。監控 stack 只有 dev-infra 這一套，同時涵蓋 dev＋prod 兩顆 Redis，做一次即可 | 後端工程師 | 本次變更 merge 後 |
| 2 | 刪除孤兒 key `search:hot_keywords`（search-service 已退役，1,177 B） | 後端工程師 | 隨時，低優先 |
| 3 | 評估 `cat:all`（單把 512KB）改為分片或縮小 payload——它單獨佔資料量 92.3%，是逐出風險的最大單一貢獻者 | 需先開票走設計 | 記憶體佔比 > 20% 時 |
| 4 | 若未來要把 session 與快取的逐出預算真正隔離：需**獨立 Redis 實例**（`maxmemory` 是實例級設定，切 db index 無法隔離）。現階段以 §5.4 的 199 倍頭寸與 `RedisKeysEvicted` 告警承擔風險，不預先拆分 | 需業務/架構裁決 | `RedisKeysEvicted` 首次觸發時立即重評 |
