# **PostgreSQL Database 設計規範**

本文件為專案統一之資料庫設計與實作規範，所有後端工程師必須遵守。

任何偏離本規範的設計，需事前提出並經團隊確認。

---

## **一、基本型別使用規範**

| **類型** | **規範** |
| --- | --- |
| 主鍵 | 依資料的性質決定：實體是 `uuid`（UUIDv7），值的集合與關聯表是複合鍵，見第二章 |
| 時間 | 一律使用 timestamptz（含時區） |
| 金額 | 一律使用 bigint，單位為新台幣「元」；禁止 numeric / float / double |
| JSON | 使用 jsonb，禁止 json |
| 布林 | 使用 boolean |
| 陣列 | 使用 text[]、uuid[] 等原生 array |
| 文字 | 預設用 text，非必要不用 varchar(n) |

---

## **二、主鍵規範**

單體（`internal/modules/`）的主鍵依資料的性質決定。理由與被否決的方案見 `docs/adr/2026-09-25-row-identity-by-data-kind.md`，改這張表等於改那份 ADR。

| **資料的性質** | **識別** |
| --- | --- |
| 實體：會被 API、其他模組、media、事件或 job 以 ID 指稱（包括有身分的子資料，例如要單筆退款的訂單明細） | `id uuid PRIMARY KEY`，**不設 DEFAULT**，應用層用 Go 標準庫的 `uuid.NewV7()` 產生；seed 與 backfill 在 INSERT 裡明寫 `uuidv7()` |
| 值的集合：只隨上層整批讀寫、不被引用（例如食譜的食材、步驟） | `PRIMARY KEY (parent_id, position)`，`CHECK (position >= 0)`；需要身分時，連同寫入方式一起升級成實體 |
| 只在上層底下識別、但身分要穩定 | `PRIMARY KEY (parent_id, item_no)`：item_no 建立後不變，排序另存 position，配號要處理併發 |
| 純關聯表 | `PRIMARY KEY (a_id, b_id)`，需要反查時加 `(b_id, a_id)` 索引 |
| 讀取模型、設定、1:1 延伸表 | 來源實體的 ID 或自然鍵 |
| 大量、只追加的紀錄 | `id uuid PRIMARY KEY DEFAULT uuidv7()`；只有量測證明鍵寬是瓶頸時才改 identity，另立 ADR |
| 給人看的編號（訂單號、售後案號） | 不是主鍵：另開 `UNIQUE` 欄，由擁有的模組設計格式，不得由 ID 截斷或取模 |
| 持有即授權的 token | 不是 ID：至少 128 bit 的 CSPRNG，只存雜湊（UUID 不能當權限憑證） |
| 過渡：ID 仍被 Java 服務存成 `BIGINT` 的模組（identity、buyer、seller 等） | `id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY`；最後一個 Java 消費者改寫後，以一個 migration 換成 uuid |

注意事項：

- 實體的 id 不設 DEFAULT：寫入前就要拿 ID 登記引用或事件，漏給 ID 必須當下失敗，不能讓 DB 靜默產生另一個。
- ID 的順序只代表大約的建立時間。業務上的排序用明確的欄位（例如 `created_at`），ID 只當最後一個 tiebreaker。
- 禁用 `bigserial`／`serial`：需要整數主鍵時用 `GENERATED ALWAYS AS IDENTITY`。禁用 UUIDv4（`gen_random_uuid()`）當主鍵：隨機插入破壞索引的 locality。
- Go 的型別是標準庫的 `uuid.UUID`（Go 1.27），sqlc 以 `db_type: "uuid"` 的 override（`import: "uuid"`、`type: "UUID"`）對應，不用 `pgtype.UUID` 或第三方套件。
- 對外 JSON 的 ID 是 uuid 格式的字串，前端當成不透明字串，不解析。

舊服務在改寫前照舊，只做維持運作所需的修改：Java 用 TSID（`Java 微服務開發規範.md` §3），Go 舊服務用 `pkg/tsid`，內部表用 `bigserial`。

金額與其他數字欄位一律 `bigint` / `integer`，對外 API 以 string 傳遞、在邊界轉換（見 `backend/AGENTS.md`「型別、金額與時間」）。

---

## **三、時間欄位標準**

### 時區規範（強制）

- **儲存與傳輸一律 UTC**，禁止儲存本地時間（如 UTC+8）。
- **前端依使用者時區自行轉換**，後端不做時區轉換。
- 格式遵循 ISO 8601：`2026-03-31T08:30:00Z`

| 層級 | 規範 |
| --- | --- |
| DB 欄位型別 | `TIMESTAMPTZ`（一律含時區，禁止 `TIMESTAMP`） |
| DB DEFAULT / Trigger | `NOW()`（TIMESTAMPTZ 下自動含時區） |
| Java 型別 | `LocalDateTime`（值語意為 UTC） |
| Java 取得當前時間 | `DateTimeUtils.nowUtc()`（禁止裸用 `LocalDateTime.now()`） |
| Proto / API 傳輸 | ISO 8601 字串（`YYYY-MM-DDTHH:mm:ssZ`） |
| JDBC Session | `SET timezone = 'UTC'`（由 common-core 統一設定） |

每一張業務表必須包含：

| **欄位** | **型別** | **預設** |
| --- | --- | --- |
| created_at | timestamptz | now() |
| updated_at | timestamptz | now() |
| deleted_at | timestamptz | NULL（選用，軟刪除） |

---

## **四、updated_at 的更新方式**

**單體（`internal/modules/`）**：在每個修改內容的 UPDATE 語句明寫 `updated_at = NOW()`，不用 trigger。`updated_at` 的意思是「內容最後被修改的時間」；計數、狀態旗標這類與內容無關的更新（例如食譜的 `view_count`）不刷新它，而 trigger 分不出這兩種更新。sqlc 的每個 UPDATE 都是明寫的 query，漏寫會在 review 與測試裡看到。

**舊服務（改寫前）**：照下面的 trigger 規範。

### （以下用 product 表的 updated_at 為例）

舊服務的所有表統一使用以下 trigger 函數（每個服務的 V1 migration 各自建立一次，同服務後續 migration 直接重用）：

```sql
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

套用到資料表（trigger 命名一律 `update_<table>_updated_at`）：

```sql
DROP TRIGGER IF EXISTS update_product_updated_at ON product;
CREATE TRIGGER update_product_updated_at
    BEFORE UPDATE ON product
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

---

## **五、狀態 / 類型欄位規範（禁用 PostgreSQL native enum）**

> **單體的規則**（ADR `docs/adr/2026-09-25-codemeta-retirement.md` D3）：碼值由擁有資料的模組以 Go 具名字串型別加常數定義，欄位是 `text` 加 `CHECK`，值＝Go 常數＝對外 proto enum 的值名去掉型別前綴（UPPER_SNAKE），不分配數字號段；測試比對 CHECK 的值集合與 Go 的 `Values()`。仍然不用 native enum。本章以下以 CodeMeta 為唯一來源的規則，只適用還沒搬進單體的舊服務，隨各模組搬遷退場。

### **1. 原則**

以下條件的欄位必須是登記過的碼值，不得存自由文字：

- 狀態數量有限
- 業務語意明確
- 會影響流程判斷
- 不允許自由輸入

**但一律不使用 PostgreSQL native enum（`CREATE TYPE ... AS ENUM`）**。全 repo 零個 `CREATE TYPE`，欄位一律宣告為 `VARCHAR(n)`，存 CodeMeta 定義的碼值（Value）。

理由：

- 碼值要跨 Java / Go / Flutter / Web / proto / MQ 共享，**SSOT 在 `docs/代碼管理（CodeMeta）/`**，由 `docs/代碼管理（CodeMeta）/scripts/generate.sh` 生成各語言常數；DB type 若也持有一份定義就變成第二份真實來源，必然漂移。
- PG native enum 新增值要跑 DDL（`ALTER TYPE ... ADD VALUE`），且無法在同一交易內立即使用；碼值增修應該只改 Markdown + 重新生成，不該綁 migration。
- 合法值在**寫入前**由應用層守：Java `CodeMetaValidator.requireValidValue(XxxEnum.class, value, "field")`（選填欄位用 `validateValueIfPresent`；`services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code/CodeMetaValidator.java`），詳見 [Java 微服務開發規範](Java%20微服務開發規範.md) §6。

---

### **2. 欄位定義範例**

碼值型狀態欄（跨服務／對外可見，走 CodeMeta）：

```sql
-- orders.status，對應 OrderStatus（0=待付款, 10=已付款, …）
status VARCHAR(4) NOT NULL DEFAULT '0'

-- product.status，對應 ProductStatus（4001=草稿, 4004=已上架）
status VARCHAR(4) NOT NULL DEFAULT '4002'

-- upload_session.status，對應 MediaUploadSessionStatus
status VARCHAR(4) NOT NULL
```

欄位語意必須以 `COMMENT ON COLUMN` 標明對應的 CodeMeta Enum 名，例如：

```sql
COMMENT ON COLUMN category.status IS '分類狀態 - CommonStatus（9003=軟刪除, 9004=啟用）';
```

**碼表本身不在本文件複製**——Enum 名與完整碼值一律查 `docs/代碼管理（CodeMeta）/`：[業務狀態碼.md](../代碼管理（CodeMeta）/業務狀態碼.md)、[業務類型碼.md](../代碼管理（CodeMeta）/業務類型碼.md)、[內部技術碼.md](../代碼管理（CodeMeta）/內部技術碼.md)，維護流程見 [README.md](../代碼管理（CodeMeta）/README.md)。

**純服務內部、不跨服務也不對前端暴露的狀態**（例：`buyers.status`、outbox 的 `pending`/`done`）可不進 CodeMeta，但仍為 `VARCHAR`，並以 `CHECK` 約束把合法值釘在 DB：

```sql
"status" varchar(20) NOT NULL DEFAULT 'active',
CONSTRAINT "chk_buyers_status" CHECK (status IN ('active', 'pending_deletion', 'closed'))
```

---

## **六、JSONB 使用規範**

- 適用於「彈性結構資料」
- 不可取代正規化資料表
- 必須能說明為何不用 table + FK

欄位定義：

```sql
attributes jsonb NOT NULL DEFAULT '{}'::jsonb
```

索引（必要時）：

```sql
CREATE INDEX idx_products_attributes
ON products USING GIN (attributes);
```

---

## **七、Array 使用規範**

適用於：

- 標籤
- 多值簡單屬性
- 非關聯資料

欄位範例：

```sql
tags text[] NOT NULL DEFAULT '{}'::text[]
```

索引：

```sql
CREATE INDEX idx_products_tags

ON products USING GIN (tags);
```

---

## **八、索引命名規範**

格式統一：

```sql
idx_<table>_<column>
```

範例：

```sql
idx_products_status

idx_products_created_at

idx_products_category_id
```

Unique Index：

```sql
uidx_<table>_<column>
```

---

## **九、外鍵（FK）設計規範**

- FK 欄位名稱必須為 <ref_table>_id
- 必須明確是否允許 NULL
- 預設 ON DELETE 不使用 CASCADE，除非是純附屬資料

範例：

```sql
category_id uuid REFERENCES categories(id)
```

---

## **十、Soft Delete 規範**

若使用軟刪除：

```sql
deleted_at timestamptz NULL
```

查詢必須過濾：

```sql
WHERE deleted_at IS NULL
```

---

## **十一、Table Spec 文件強制欄位**

每一張表必須提供以下文件欄位：

| **欄位** | **說明** |
| --- | --- |
| Column | 欄位名稱 |
| Data Type | PostgreSQL 型別 |
| Nullable | 是否可為 NULL |
| PK | 是否為主鍵 |
| Unique | 是否唯一 |
| Default | 預設值 |
| Example | 範例資料 |
| Description | 業務說明 |
| Index | 索引類型 |
| FK | 參考表 |

---

## **十二、冪等寫入與唯一鍵衝突**

冪等寫入一律用 `INSERT ... ON CONFLICT`。**禁止以「讓 DB 拋唯一鍵違反再 catch」實作冪等分流**——交易內任一句 SQL 出錯即進入 aborted（`25P02`），冪等賴以復原的回查會被自己觸發的錯誤打掉，且只在真的撞鍵時發作。

### **1. 選哪個變體**

| 需求 | 寫法 |
| --- | --- |
| 要拿回既有那一列 | `ON CONFLICT <目標> DO UPDATE SET <冪等鍵> = EXCLUDED.<冪等鍵> RETURNING *` |
| 只需知道有沒有插入 | `ON CONFLICT <目標> DO NOTHING`，判 affected rows |
| 需要 CAS 述詞 | `ON CONFLICT … DO UPDATE … WHERE <述詞>`，0 rows 即 CAS 不成立 |

- 預設指名衝突目標。`CREATE UNIQUE INDEX` 建的用欄位推論 `ON CONFLICT (col_a, col_b)`；`ON CONFLICT ON CONSTRAINT` 只認表定義裡的 `UNIQUE(...)`，對 index 會報 constraint does not exist。partial index 的推論要帶上相同述詞。
- 不指名的 `DO NOTHING` 只用於 expand-contract 窗口內新舊約束並存時，須註明退場條件，窗口結束收回成指名。
- `UPDATE` 沒有 `ON CONFLICT`，撞唯一鍵只能拋錯，處置見下節。
- `DO NOTHING` 遇到並發未提交的衝突列不等待、直接跳過，後續回查也看不到；需要回傳既有列時用 `DO UPDATE`。

### **2. 已經拋錯之後的合法處置**

catch `23505` 本身不違規，**在同一個交易裡繼續下 SQL 才違規**。

| 形狀 | 做法 |
| --- | --- |
| 分類後回傳 | 包成具名 sentinel／業務錯誤直接 `return`，不再下任何 SQL |
| 交易外回查 | 讓交易 rollback，回查跑在 `WithTx` 之外、用外層 `ctx` |
| 立刻中止 | 交易內偵測到衝突即回業務錯誤，放棄該筆 |

### **3. 守門**

`scripts/check-idempotent-write.py` 偵測 `23505` 判定的 if-block 內是否還有帶 `ctx` 的呼叫。靜態檢查看不出 `ctx` 上有沒有交易，合法例外登記在 `scripts/idempotent-write-allowlist.txt`。目前手動執行，尚未掛進 CI。

---

## **十三、禁止事項**

- 禁止 status 存自由文字：單體是模組定義的碼值＋`CHECK`；舊服務是 CodeMeta 登記的碼值（或服務內部狀態＋`CHECK`），見第五章
- 禁止使用 PostgreSQL native enum（`CREATE TYPE ... AS ENUM`）
- 禁止金額用 float
- 禁止時間用 timestamp（無時區）
- 禁止在 JSON 中塞可正規化的資料
- 舊服務：禁止在 API / 程式碼自行定義碼值而未登記到 `docs/代碼管理（CodeMeta）/`（單體的碼值由擁有的模組定義，見第五章）
- 禁止以「讓 DB 拋唯一鍵違反再 catch」實作冪等分流，禁止在 catch 到 `23505` 之後於同一交易內繼續下 SQL，見第十二章