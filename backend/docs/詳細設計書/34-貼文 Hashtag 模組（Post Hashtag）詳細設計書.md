# 貼文 Hashtag 模組（Post Hashtag）詳細設計書

---

> 對應 issue：[B-153 / #706] 買家&賣家 App｜貼文 — 新增 hashtag 支援（schema 欄位 + 搜尋 / 建議 / 熱門 API）
> 設計藍本：本模組鏡像 [`33-商品 Hashtag 模組（Product Hashtag）詳細設計書.md`](33-商品%20Hashtag%20模組（Product%20Hashtag）詳細設計書.md)，僅在 schema 命名、語言（Go vs Java）、可見性定義與 RPC 名稱上差異化。共用業務規則（normalize / 字元 / 長度 / 數量）與錯誤碼。

---

## 1. 模組概述

### 1.1 模組定位

| 項目 | 說明 |
|------|------|
| 模組性質 | `social-service` **內部子模組**，無獨立部署 |
| 所屬服務 | `services/social-service/`（Go） |
| 所屬 DB | `hawkr_social`（PostgreSQL） |
| Node ID Base | 250（social-service 命名空間，避開 product 的 200） |
| 設計藍本 | [33-商品 Hashtag 模組詳細設計書](33-商品%20Hashtag%20模組（Product%20Hashtag）詳細設計書.md) |
| 共用元件 | 業務規則（normalize / validator）、錯誤碼 3015-3018、Trending 區間規則 |

### 1.2 職責範圍

- Hashtag 主表（`cms_hashtag`）與貼文關聯表（`cms_post_hashtag`）的 schema 設計與寫入規則
- Canonical / Display 雙 normalize pipeline（Go 實作，邏輯等同 product）
- 字元規則、長度上限、每貼文 hashtag 上限驗證（沿用 product 3015-3018）
- `cms_hashtag.post_count` 全歷史 counter 同事務維護（單一寫入路徑）
- Hashtag 相關 RPC：自動補全（auto-complete）、熱門查詢（trending）
- 貼文 RPC（CreatePost / UpdatePost / GetPostList / GetPostDetail / Hide / Restore / SoftDelete）對 hashtag 欄位的整合

### 1.3 非職責範圍

| 不負責 | 歸屬 |
|--------|------|
| 貼文本體 CRUD | social-service post.proto 既有 RPC |
| 貼文檢舉 / 封鎖 | [31-內容檢舉與封鎖](31-內容檢舉與封鎖（Content%20Reporting%20%26%20Block）詳細設計書.md) |
| 與商品 hashtag 跨服務聚合查詢 | 不在本模組範圍（保持兩邊分開，見 §1.5） |
| Hashtag follow / 訂閱 | 不在本模組範圍 |
| 中文時間衰減 / 個人化 trending | 不在本模組範圍 |

### 1.4 名詞定義

| 名詞 | 說明 |
|------|------|
| Canonical name | `cms_hashtag.name`，去除 `#` / NFC / lower 後的唯一識別字串 |
| Display name | `cms_post_hashtag.display_name`，per-binding 保留原大小寫的版本（NFC，**不** lower） |
| Per-binding display | 同一 canonical 在不同貼文可有不同 display（IG 風格） |
| Counter（`cms_hashtag.post_count`） | 該 hashtag 被多少**公開可見**貼文引用的即時計數（單一寫入路徑同事務維護） |
| 公開可見 | `cms_post.deleted_at IS NULL AND cms_post.hidden_at IS NULL` |
| Trending | 依使用者指定**日期區間**內 `cms_post_hashtag.create_time` 落入區間的 distinct 公開貼文數降冪排行（**不**等同於 `cms_hashtag.post_count`） |

### 1.5 與商品 Hashtag 模組的關係

- **完全分離**：兩個服務各自 owner 一份 hashtag 表，counter 獨立。
- **規則共用**：normalize / validator 邏輯一致（同一份規格、相同錯誤碼），同一 raw 輸入在兩處產生同一 canonical（例：`#美食` → canonical `美食`）。
- **不做跨服務查詢 / 不合併端點**：賣家 compose 貼文時的 autocomplete 只看 cms_hashtag.post_count，與商品 hashtag 視覺與統計分離。
- **路徑刻意分流**：商品 `/public/hashtags/*`、貼文 `/public/tags/*`。

---

## 2. 資料庫設計

**Database**：`hawkr_social`
**Migration tool**：Atlas declarative（social-service 既有）

### 2.1 cms_hashtag 主表

| 欄位名 | 邏輯名 | 型態 | Null | PK | Default | 備註 |
|---|---|---|---|---|---|---|
| id | hashtag ID | BIGINT | N | Y | | TSID（Node ID Base: 250） |
| name | canonical 名稱 | VARCHAR(40) | N | | | 唯一識別；normalize 規則見 §3.1 |
| post_count | 引用計數 | INT | N | | 0 | denormalized；僅算公開可見貼文；同事務 row-lock 維護（單一寫入路徑為唯一真實來源；§4） |
| create_time | 建立時間 | TIMESTAMPTZ | N | | NOW() | |
| update_time | 更新時間 | TIMESTAMPTZ | N | | NOW() | 由 application 層 UPDATE 同步維護（同 cms_post pattern — social-service **無 trigger**）；所有 `UPDATE cms_hashtag` 必須含 `, update_time = NOW()` |

**索引**：

| 索引名 | 定義 | 用途 |
|---|---|---|
| `uk_cms_hashtag_name` | UNIQUE (name) | canonical 唯一性 |
| `idx_cms_hashtag_name_pattern` | (name varchar_pattern_ops) | `LIKE 'prefix%'`（PREFIX 模式 suggest） |
| `idx_cms_hashtag_name_trgm` | GIN (name gin_trgm_ops) | CONTAIN 模式 suggest（pg_trgm 中間匹配） |

> 不建 `(post_count DESC, name)` 索引：suggest 流程都帶 `q` 前綴篩 + LIMIT 小（≤ 20），sort 走記憶體即可；無 admin 「全歷史熱門列表」需求。

> **Extension 需求**：本 migration 需同時 `CREATE EXTENSION IF NOT EXISTS pg_trgm;`。Dev / stage / prod 各環境需先 query 確認是否已啟用（見 §10）。

### 2.2 cms_post_hashtag 關聯表

| 欄位名 | 邏輯名 | 型態 | Null | PK | Default | 備註 |
|---|---|---|---|---|---|---|
| post_id | 貼文 ID | BIGINT | N | Y（複合） | | **不加 FK**（與 cms_post 既有風格一致） |
| hashtag_id | hashtag ID | BIGINT | N | Y（複合） | | **不加 FK**（同上） |
| display_name | 顯示名稱 | VARCHAR(40) | N | | | 賣家輸入時保留原大小寫的版本（trim + strip `#` + NFC，**不** lower） |
| position | 排序位置 | SMALLINT | N | | 0 | 賣家編輯時順序（0 起算）；詳情 hashtag list 依此升序 |
| create_time | 建立時間 | TIMESTAMPTZ | N | | NOW() | **貼文掛上此 hashtag 的時間**（§5.2 GetTrendingHashtags 日期區間欄位） |

**主鍵**：`PRIMARY KEY (post_id, hashtag_id)` — 同 hashtag 不可重複綁定同一貼文（去重由 §3.1 normalize 完成，DB 主鍵作為終局保險）。

**索引**：

| 索引名 | 定義 | 用途 |
|---|---|---|
| `idx_cms_post_hashtag_lookup` | (hashtag_id, post_id) | 「點 tag → 貼文列表」反查（§5.3） |
| `idx_cms_post_hashtag_created` | (create_time, hashtag_id) | 區間範圍掃描 + group by（§5.2 用） |
| 隱含索引 | 複合主鍵 (post_id, hashtag_id) | post → hashtag 列表查詢 |

### 2.3 寫入規則

- **新增綁定**：`INSERT INTO cms_post_hashtag (post_id, hashtag_id, display_name, position) VALUES (...)`
- **移除綁定**：`DELETE FROM cms_post_hashtag WHERE post_id = ? AND hashtag_id = ANY(?::bigint[])`
- **更新既有綁定（kept 分支，display / position 變動）**：自訂 SQL `UPDATE cms_post_hashtag SET display_name = ?, position = ? WHERE post_id = ? AND hashtag_id = ?`

---

## 3. 業務規則

> §3.1 ~ §3.3 與 product hashtag（33 §3）邏輯完全一致；Go 實作放 `internal/domain/hashtag/`。

### 3.1 normalize 規則

對單一 raw 輸入產生 (canonical, display) 雙輸出：

1. `strings.TrimSpace`（含全形 NBSP ` ` / `　`）
2. Strip 開頭連續 `#`
3. Unicode NFC（`golang.org/x/text/unicode/norm`）
4. **canonical 路徑**：ASCII letter 轉 lower（中文 / 全形不變，rune 級判斷，不可用 `strings.ToLower` 全做）
5. **display 路徑**：上述 NFC 結果，**不**做 lower

回傳 `ok=false` 條件：raw 為空 / strip 後為空。

### 3.2 字元規則

允許：ASCII letter（a-z）、ASCII digit（0-9）、CJK Unified Ideographs（Basic + Extension A-H + Compatibility）。
拒絕：空白、`#`、ASCII 標點 / 符號、emoji、控制字元 → `HASHTAG_INVALID_CHAR (3015)`。

### 3.3 display 與 canonical 對應

- canonical 是 dedup / 主鍵基準；display 是賣家輸入時的原大小寫版本
- 同 canonical 多輸入 → 取**輸入序第一個** display（與 product 一致）
- per-binding：同 canonical 在不同貼文可有不同 display

### 3.4 數量與長度

- 單個 hashtag code-point 數需 ∈ [1, 40] → 超出 `HASHTAG_LENGTH_EXCEEDED (3016)`
- 同貼文 canonical 去重後 size ≤ 10 → 超出 `HASHTAG_COUNT_EXCEEDED (3017)`
- normalize 後為空 → `HASHTAG_EMPTY_AFTER_NORMALIZE (3018)`

### 3.5 錯誤碼引用

| Code | Constant | 說明 |
|---|---|---|
| 3015 | `HASHTAG_INVALID_CHAR` | 含禁用字元 |
| 3016 | `HASHTAG_LENGTH_EXCEEDED` | 長度 1-40 外 |
| 3017 | `HASHTAG_COUNT_EXCEEDED` | 數量 > 10 |
| 3018 | `HASHTAG_EMPTY_AFTER_NORMALIZE` | normalize 後空 |

**Code 共用、message 可情境化**：3015-3018 numeric code 與 sentinel name 一律與 product 共用，**不**新增 social-namespace 別名（例如 `HASHTAG_INVALID_CHAR_POST`）。但 message 文案允許依使用情境調整 — 例如 3017 在 product 場景說「每個商品最多 10 個」，在 post 場景說「每篇貼文最多 10 個」。Gateway envelope 的 `messageZh` 直接透傳服務送出的訊息，所以情境化 message 對 UX 較友善。

---

## 4. Counter 維護（`cms_hashtag.post_count`）

### 4.1 觸發場景

| 場景 | 對應 RPC / 流程 | counter 變動 |
|---|---|---|
| Create 公開貼文 with tags | CreatePost（cms_post 寫入 + cms_post_hashtag insert） | 每個綁定 hashtag +1 |
| Update 公開貼文的 tags 集合 | UpdatePost diff（added / removed） | added +1, removed -1 |
| Hide 公開貼文 | HidePost（hidden_at = NOW()） | 該 post 所有 binding 全 -1 |
| Restore 已 hide 貼文 | RestorePost（hidden_at = NULL） | 該 post 所有 binding 全 +1 |
| Soft-delete 貼文 | SoftDeletePost / SoftDeletePostsBySellerID | 該 post 所有 binding 全 -1 |

> Update 路徑若僅改 title / content，不動 tags → counter 不變（PATCH 區分「沒傳 vs 傳空」見 §5.4）。

### 4.2 並發保護

- 任何多個 hashtag 的 counter delta，**強制按 `hashtag.id ASC` 排序**逐筆 UPDATE，避免不同 transaction 取 lock 順序不同造成死鎖（鏡像 product 33 §4.2）
- 同事務內 row-lock；無需 advisory lock

### 4.3 與 Trending 查詢的差異

| 概念 | 來源欄位 | 語意 | 排序基礎 |
|---|---|---|---|
| `cms_hashtag.post_count` | denormalized counter | **全歷史**公開貼文引用次數 | suggest API |
| Trending | `cms_post_hashtag.create_time` 區間 distinct 公開 post 數 | **使用者指定區間**內被掛上的貼文數 | trending API |

兩者**不等價**：post_count 是全歷史定點數，trending 是區間 distinct 累積數。

### 4.4 一致性與邊界

- **單一寫入路徑為唯一真實來源**（[後端服務責任邊界規範](../開發規範/後端服務責任邊界規範.md)）：不做 reconciliation job，避免 race condition 與資料漂移
- 任何路徑修改 tags / 可見性，**counter delta 必須在同一 transaction 內完成**
- 部分失敗 → 整個 transaction rollback，counter 不會半套

---

## 5. RPC 規格

> 所有新 RPC 加在 `api/proto/post/post.proto` 的 `PostService` 內。
> 對外數字一律 string（[Java 微服務開發規範](../開發規範/Java%20微服務開發規範.md) §2.1）。

### 5.1 SearchHashtags（auto-complete）

```protobuf
rpc SearchHashtags(SearchPostHashtagsRequest) returns (SearchPostHashtagsResponse);

enum PostHashtagSearchMode {
    POST_HASHTAG_SEARCH_PREFIX = 0;
    POST_HASHTAG_SEARCH_CONTAIN = 1;
}

message SearchPostHashtagsRequest {
    string q = 1;
    PostHashtagSearchMode mode = 2;
    int32 limit = 3;
}
message PostHashtagItem {
    string name = 1;
    string post_count = 2;
}
message SearchPostHashtagsResponse {
    int32 code = 1;
    string message = 2;
    repeated PostHashtagItem data = 3;
}
```

#### 規則

- `q` 輸入任意大小寫，server normalize 後查
- `limit` < 1 → default 10；> 20 → 20（與 issue 一致）
- 預設 mode = PREFIX（`LIKE 'canonical%'`，走 `idx_cms_hashtag_name_pattern`）
- CONTAIN mode：`name % canonical`（pg_trgm similarity operator），按 similarity DESC, post_count DESC, name ASC 排序
- 排序：post_count DESC, name ASC（PREFIX 模式），CONTAIN 模式見上

### 5.2 GetTrendingHashtags

```protobuf
rpc GetTrendingHashtags(GetTrendingPostHashtagsRequest) returns (GetTrendingPostHashtagsResponse);

message GetTrendingPostHashtagsRequest {
    string start_from = 1;
    string end_to = 2;
    int32 limit = 3;
}
message PopularPostHashtagItem {
    string hashtag = 1;
    string count = 2;
}
message GetTrendingPostHashtagsResponse {
    int32 code = 1;
    string message = 2;
    repeated PopularPostHashtagItem data = 3;
}
```

#### 日期區間規則（鏡像 product 33 §5.2.2）

| start_from | end_to | 解算後區間 |
|---|---|---|
| 空 | 空 | `[today - 1 month, today]`（預設近 1 個月） |
| 給 | 空 | `[start_from, min(start_from + 1 year, today)]` |
| 空 | 給 | `[end_to - 1 year, end_to]` |
| 給 | 給 | `[start_from, end_to]` |

驗證：
- 日期格式 `YYYY-MM-DD`，非則 INVALID_ARGUMENT
- start ≤ end
- end - start ≤ 1 year

SQL 邊界（左閉右開）：`create_time >= startDate.atStartOfDay() AND create_time < endDate.plusDays(1).atStartOfDay()`

#### 排序與 limit

- 排序：`count DESC, hashtag ASC`
- `limit` < 1 → default 20；> 50 → 50

#### Query SQL（核心）

```sql
SELECT h.name AS hashtag, COUNT(DISTINCT ph.post_id)::int AS count
FROM cms_post_hashtag ph
JOIN cms_post p ON p.id = ph.post_id
JOIN cms_hashtag h ON h.id = ph.hashtag_id
WHERE ph.create_time >= $1 AND ph.create_time < $2
  AND p.deleted_at IS NULL AND p.hidden_at IS NULL
GROUP BY h.name
ORDER BY count DESC, h.name ASC
LIMIT $3;
```

走 `idx_cms_post_hashtag_created` + `idx_cms_post_pk`。

### 5.3 GetPostList — `tag` query 支援

既有 `GetPostListRequest` 加 `string tag = 5;`：

- 空字串 → 不篩
- 非空 → server normalize 後查；JOIN 對應 SQL：

```sql
-- name: ListPostsByHashtagName :many
SELECT p.* FROM cms_post p
JOIN cms_post_hashtag ph ON ph.post_id = p.id
JOIN cms_hashtag h ON h.id = ph.hashtag_id
WHERE h.name = $1 AND p.deleted_at IS NULL AND p.hidden_at IS NULL
ORDER BY p.create_time DESC LIMIT $2 OFFSET $3;
```

`PostListItem` 加 `repeated string tags = 12;`，列表填充採批次 `ListPostHashtagsByPostIDs(post_ids)` 後在 service 層 group by post_id。

### 5.4 CreatePost / UpdatePost — `tags` 整合

```protobuf
message CreatePostRequest {
    // 既有 1-7（reserved 6）；下一個可用 = 8
    repeated string tags = 8;
}

message UpdatePostRequest {
    // 既有 1-9（reserved 7）；下一個可用 = 10
    repeated string tags = 10;
    bool tags_present = 11;  // true → tags field 被明確設定（含空陣列）；false → 不動
}
```

**CreatePost 流程**：
1. validate + normalize tags（§3）
2. 開 transaction：
   1. INSERT cms_post
   2. UPSERT cms_hashtag（依輸入序）
   3. INSERT cms_post_hashtag（position = 輸入序 0-based）
   4. counter delta：addedHashtagIds（按 id ASC）逐筆 +1
3. commit

**UpdatePost 流程**（**PATCH 語意：區分「沒傳 vs 傳空」**）：

| 請求形式 | 行為 |
|---|---|
| body 無 `tags` key → `tags_present=false` | 不動 hashtag binding 與 counter |
| body `"tags": []` → `tags_present=true, tags=[]` | 視為全清空（removedSet = 現有所有，addedSet 空） |
| body `"tags": ["美食"]` → `tags_present=true, tags=[...]` | 全集替換（diff added / removed / kept） |

- Gateway 解析 body 時偵測 JSON 是否含 `tags` key，據此設定 `tags_present`
- `tags_present=true` 時走 normalize + diff + apply 流程，同事務 counter delta；公開可見才實際 delta（已 hide 的 post 修改 tags 不動 counter）
- `tags_present=false` 完全跳過 hashtag 流程

> 為何選 `bool tags_present` 而非 wrapper message / FieldMask：與 social-service 既有 `UpdatePostRequest` 風格一致（其他欄位也都是普通 field，handler 用 `len() > 0` 判斷是否更新），加一個 explicit flag 是最小侵入解。

### 5.5 Hide / Restore / SoftDelete — counter 整批調整

| RPC / 內部呼叫 | 觸發 | counter delta |
|---|---|---|
| HidePost | hidden_at NULL → NOW() | 該 post 所有 binding 全 -1 |
| RestorePost | hidden_at non-null → NULL | 該 post 所有 binding 全 +1 |
| SoftDeletePost / SoftDeletePostsBySellerID | deleted_at NULL → NOW() | 該 post 所有 binding 全 -1 |

實作：`HashtagUsecase.AdjustCountersForVisibilityChange(postID, delta)` 先 `ListHashtagIDsByPostID`（已 ORDER BY id ASC），然後逐筆 UPDATE。

---

## 6. Gateway API 規格

> Base path：`/api/v1`。錯誤回應走 gateway envelope（[API 設計規範](../開發規範/API%20設計規範.md)「十二、統一回應格式（Envelope）」）。

### 6.1 新端點

| Method | Path | gRPC | Auth | limit / default |
|---|---|---|---|---|
| GET | `/public/tags/suggest?q=&mode=&limit=` | `PostService.SearchHashtags` | public | limit default 10 / max 20 |
| GET | `/public/tags/trending?start_from=&end_to=&limit=` | `PostService.GetTrendingHashtags` | public | limit default 20 / max 50 |

**SearchHashtags query**：
- `q`：必填，賣家輸入字串
- `mode`：可選 `prefix` (default) / `contain`
- `limit`：可選

**GetTrendingHashtags query**：
- `start_from`：可選，`YYYY-MM-DD`
- `end_to`：可選，`YYYY-MM-DD`
- `limit`：可選

### 6.2 既有端點變動（向後相容）

| 端點 | 變動 | client 影響 |
|---|---|---|
| `POST /sellers/posts` | request body 多吃可選 `tags []string` | 不傳 = 空，照舊 |
| `PATCH /sellers/posts/{id}` | request body 多吃可選 `tags []string`；區分「沒傳 vs 傳空」 | 不傳 = 不動；傳 `[]` = 清空；傳 `[...]` = 替換 |
| `GET /public/posts/{id}` | response.data 多回 `tags []string` | 忽略則無感 |
| `GET /public/posts` | response 每筆多回 `tags`；query 多吃可選 `tag=` filter | 忽略 / 不傳 = 照舊 |
| `GET /sellers/posts` | response 每筆多回 `tags` | 忽略則無感 |

### 6.3 JSON Response 範例

```jsonc
// GET /public/posts/{id}
{
  "code": 0, "message": "success",
  "data": {
    "id": "12345", "title": "...", "content": "...",
    "tags": ["美食", "生鮮", "宅配"],
    // ... other fields ...
  }
}

// GET /public/tags/suggest?q=美
{
  "code": 0, "message": "success",
  "data": [
    { "name": "美食", "post_count": "1284" },
    { "name": "美味", "post_count": "432" }
  ]
}

// GET /public/tags/trending
{
  "code": 0, "message": "success",
  "data": [
    { "hashtag": "美食", "count": "57" },
    { "hashtag": "宅配", "count": "31" }
  ]
}
```

### 6.4 錯誤回應範例

```jsonc
// POST /sellers/posts with tags=["美食", "🍔"]
HTTP 400
{
  "code": 3015,
  "message": "hashtag 含不允許字元（僅接受中文/英文/數字，不可含空白、#、emoji 或標點）",
  "messageZh": "hashtag 含不允許字元（僅接受中文/英文/數字，不可含空白、#、emoji 或標點）",
  "traceId": "..."
}
```

---

## 7. 模組目錄結構

```
services/social-service/
├── db/
│   ├── schema/schema.sql                    # 加 cms_hashtag / cms_post_hashtag + pg_trgm
│   ├── migrations/YYYYMMDDHHMMSS_*.sql      # atlas migrate diff 產生
│   └── queries/hashtag.sql                  # sqlc queries（§3 列出）
├── internal/
│   ├── domain/hashtag/
│   │   ├── normalizer.go
│   │   ├── normalizer_test.go
│   │   ├── validator.go
│   │   └── validator_test.go
│   ├── repository/postgres/
│   │   ├── hashtag_repository.go            # wrap sqlc gen
│   │   └── gen/hashtag.sql.go               # sqlc generated
│   ├── usecase/
│   │   ├── hashtag.go                       # HashtagUsecase
│   │   ├── hashtag_test.go
│   │   └── post.go                          # 既有，整合 hashtag 流程
│   └── grpc/handler.go                      # 加 SearchHashtags / GetTrendingHashtags

api/proto/post/post.proto                    # 加欄位 + 2 個 RPC
gateway/internal/handler/
├── post.go                                  # 加 tags body / query 解析
└── post_hashtag.go                          # 新增，SearchHashtags / GetTrendingHashtags handler
gateway/internal/router/routes_post.go       # 註冊新路由
```

---

## 8. Testing 計畫

### 8.1 Unit tests（domain layer，table-driven）

- `normalizer_test.go`：trim / strip# / NFC / lower 各情境、全形空白、emoji、半 / 全形 `#` 開頭、NFC 不同點碼合併
- `validator_test.go`：4 個錯誤碼各覆蓋、同 canonical 多輸入取首 display、exactly 10 / 11 boundary、CJK Extension B 邊界

### 8.2 Usecase tests（sqlmock 或 testcontainer）

- upsert 對既有 / 不存在 hashtag 行為
- apply diff：added / removed / kept counter delta 正確
- apply 順序：mock 確認 hashtag.id ASC 排序
- visibility 改變（hide / restore / soft-delete）→ counter 整批 delta
- search PREFIX vs CONTAIN 走不同 SQL
- popular 區間：兩端空 / 單空 / 全給 / 越界

### 8.3 Integration tests

- CreatePost with tags → DB 確認 cms_post_hashtag rows + cms_hashtag.post_count+1
- HidePost / RestorePost → counter -1 / +1
- SoftDeletePost / SoftDeletePostsBySellerID → counter -1
- PATCH 區分「沒傳 vs 傳空」：未傳 → counter 不變；傳 `[]` → 全清；傳 `["新"]` → diff

### 8.4 Gateway tests

- `/public/tags/suggest?q=美&mode=prefix` → 200 + 排序正確
- `/public/tags/trending?start_from=2026-01-01&end_to=2026-02-01` → 200
- `/public/posts?tag=美食` → 200 + 只回該 tag post

### 8.5 Chaos / Consistency

- 同事務內 upsert + insert + counter delta 任一失敗 → 全 rollback，counter 不會半套
- 不寫 reconciliation test（不做 reconciliation job）

---

## 9. Migration 與 Rollout

### 9.1 Dev / Prod 環境檢查

部署前需 query 兩邊環境確認：

```sql
-- 確認 pg_trgm 已啟用
SELECT extname FROM pg_extension WHERE extname = 'pg_trgm';

-- 確認 social-service 用的 PG 版本 >= 9.1（pg_trgm 最低需求）
SELECT version();
```

若 prod 未啟用 pg_trgm，migration 內 `CREATE EXTENSION IF NOT EXISTS pg_trgm;` 會自動建立（需 superuser 權限，依 Atlas 部署流程確認）。

### 9.2 部署順序

變更依下列順序落地，前一階段未上線時後一階段會缺依賴：

1. social-service DB migration apply（`cms_hashtag` / `cms_post_hashtag` / `pg_trgm`）
2. social-service 部署：domain / repository / usecase（無對外行為變動）
3. social-service 部署：`post.proto` 欄位與 2 支 RPC 上線
4. gateway 部署：HTTP 端點上線，前端可開始接

無 feature flag 需要（純加欄位 / 新端點，向後相容）。

### 9.3 Backfill

- 既有 `cms_post` 預設無 tags（cms_post_hashtag 空），前端視為空陣列
- 不寫 backfill；賣家編輯既有貼文時自然填入 tags

### 9.4 監控

| 指標 | 異常條件 | 處理 |
|---|---|---|
| `cms_hashtag.post_count` vs 實際 distinct count（每週抽樣 top 100 hashtag） | 任何差異 | 視為單一寫入路徑 bug；查最近相關 trace（Create / Update / Hide / Restore / SoftDelete）並修補 |
| trending query p99 latency | > 500ms | 檢查 `idx_cms_post_hashtag_created` 是否被優化器選用（EXPLAIN ANALYZE） |
| suggest query p99 latency | > 200ms | 檢查 trgm / pattern 索引是否走 |

---

## 10. 安全與權限

- 所有新端點為 public 讀取（與 `/public/posts/*` 一致），無 auth requirement
- 寫入路徑（CreatePost / UpdatePost）走 seller-only auth，沿用既有 middleware
- 無新增 admin 端點
