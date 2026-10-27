# **DB Migration 相容性規範（expand–contract）**

本文件規範 schema 變更如何與「不斷線部署（zero-downtime deploy）」共存。
所有含 DB migration 的 PR 必須遵守，偏離需事前提出並經團隊確認。

---

## 一、為什麼需要這條規範

部署採 **surge 滾動更新**（`maxUnavailable: 0 / maxSurge: 1`）：新 pod 通過 readiness
後，舊 pod 才會下線。因此每次部署都有一段「**新舊 code 同時在線**」的視窗。

而 migration 在新 code 生效前（或同時）就套用：

| 服務 | Migration 工具 | 套用時機 |
|---|---|---|
| Go | Atlas declarative（`db/schema/schema.sql` 為 source of truth，`migrate diff` 生成 SQL） | deploy 前的 CI `migrate` job（`atlas migrate apply`） |
| Java | Flyway（`src/main/resources/db/migration/V*.sql`） | **Spring Boot 應用啟動時**自動執行 |

兩者皆導致：**新 schema 先到，舊 code 仍在跑**。

> 結論：每個 migration 都必須讓「**舊版 code 在新 schema 上仍能正常運作**」。
> 否則 surge 視窗內舊 pod 會 500，違反不斷線目標。

---

## 二、安全變更（單一 release 可直接做）

向後相容、舊 code 不受影響：

- **加表**。
- **加欄位**：必須 `NULL` 或帶 `DEFAULT`（舊 code 的 INSERT 不含此欄也不會失敗）。
- **加 index**：PostgreSQL 用 `CREATE INDEX CONCURRENTLY`（避免鎖表阻塞線上查詢）。
- **加 / 放寬 constraint**：僅當既有資料全數滿足，且舊 code 寫入不會違反。
- **加碼值**：本專案不使用 PostgreSQL native enum，狀態／類型欄位一律 `VARCHAR` 存 CodeMeta 碼值（見 [PostgreSQL 設計規範與最佳實踐](PostgreSQL%20設計規範與最佳實踐.md) 第五章），新增碼值只改 CodeMeta Markdown 並重新生成，不動 DB。

---

## 三、新增衍生欄位必須回填既有列（backfill）

「加欄位帶 `DEFAULT`」只保證**部署安全**（舊 code 的 INSERT 不會失敗），
**不保證語意正確**：`ADD COLUMN ... DEFAULT 'x'` 會把**每一列**都填成同一個 `x`，
包含那些「照既有資料判斷根本不該是 `x`」的列。DEFAULT 沒有分流能力，回填要自己寫。

### 判準（符合任一 → 該 migration 必須含回填語句）

新欄位的值**能由同表既有欄位推導**時：

- 生命週期／狀態欄位（`status`、`state`、`phase`），而表上已有 `deleted_at`、
  `cancelled_at`、`finished_at` 這類「事件已發生」的時間戳
- 布林旗標（`is_*`、`has_*`）可由既有欄位判定
- 由既有欄位正規化／反正規化而來的欄位

### 動作（三步，缺一不可）

1. `ADD COLUMN ... DEFAULT <新列的預設值>`
2. **同一個 migration 檔內**緊接回填：`UPDATE <table> SET <新欄位> = ... WHERE <既有欄位條件>;`
   - Go／Atlas：`schema.sql` 表達不了資料回填 —— `atlas migrate diff` 生完 DDL 後，
     手動把 `UPDATE` 追加到**該次新生成的檔案**，再跑 `atlas migrate hash` 重算 checksum
     （這是唯一允許手改 migration 的情形，且只能改剛生成、尚未套用的那一檔）
   - Java／Flyway：回填 `UPDATE` 寫在同一支 `V{N+1}` 檔內
3. 若該不變式**恆真**（不只是這次要修的存量），一併加 `CHECK` 約束 ——
   讓 DB 來守，而不是靠「下一個人記不記得」

> 回填不可拆到下一個 release：拆開＝中間有一段視窗資料是錯的，而讀新欄位的 code 已經上線。

### 反例（本專案真實事故）

`services/buyer-service/db/migrations/20260427044537_add_buyer_deletion_status.sql` 與
`services/seller-service/db/migrations/20260427052936_add_seller_deletion_status.sql`
為 `buyers` / `sellers` 加上 `status ... NOT NULL DEFAULT 'active'`，
卻沒有回填 `deleted_at IS NOT NULL` 的既有列 —— 少的就是這兩句：

```sql
UPDATE buyers  SET status = 'closed' WHERE deleted_at IS NOT NULL;
UPDATE sellers SET status = 'closed' WHERE deleted_at IS NOT NULL;
```

於是「已軟刪除的帳號 `status` 仍是 `active`」：不變式
`deleted_at IS NOT NULL ⇒ status = 'closed'` 對 migration 之前的列全部不成立，
直到 2026-08-01 才被發現並人工更正（dev buyers 2 列、sellers 2 列）。
現已補上 `chk_buyers_deleted_requires_closed` / `chk_sellers_deleted_requires_closed`
兩條 CHECK 約束：同類漏回填會在 migration 套用當下直接失敗，不再靜默帶過。

---

## 四、危險變更（必須拆成三個 release：expand → 停用 → contract）

以下變更會讓舊 code 立即壞掉，**禁止在單一 release 內完成**：

- 改欄位名 / 刪欄位
- 改型別、改 nullability 為 `NOT NULL`、收緊 constraint
- 刪表、下架既有碼值（歷史資料列仍持有該值，且舊 code 仍可能寫入）

### 為什麼是三段而不是兩段

本 repo 的部署管線裡 **migration 結構性地先於 code**：`deploy-k8s.yml` 的
`deploy: needs: [..., migrate]`，且 chart 是 `maxUnavailable:0 / maxSurge:1`
（新 pod 通過 readiness 前舊 pod 不下線）。

所以每一個 release 的 migrate 生效時，**服務中的必然是「上一個 release」的 code**。
把「移除舊欄位」與「只用新欄位」放進同一個 release，等於讓 DROP 打在
「仍在雙寫舊欄位」的那一版 pod 上 —— 整段 surge 視窗每一次讀寫都是
`42703 undefined_column`。兩段式流程在這條管線下**照做也會炸**，中間必須多一段
「code 先停用、DB 不動」的 release。

### expand–contract 三階段流程

以「欄位改名 `phone` → `mobile`」為例：

| 階段 | Release | DB 變更 | Code 變更 |
|---|---|---|---|
| **Expand** | R1 | 加 `mobile`（nullable） | 同時寫 `phone` + `mobile`；讀優先 `mobile`、fallback `phone`。回填既有資料。 |
| —（過渡）| — | 確認 R1 全節點上線、舊 pod 全下線 | — |
| **停用（stop-using）** | R2 | **無** | **完全停止讀寫 `phone`**；只用 `mobile`。舊欄位仍留在 schema 裡。 |
| —（過渡）| — | 確認 R2 全節點上線、舊 pod 全下線 | — |
| **Contract** | R3 | 移除 `phone` | **無**（R2 已經不碰它了） |

要點：
- **R1／R2／R3 必須是三次獨立部署**，每次都確認前一版 code 已完全退場才進下一段。
- 加欄位（expand）與刪欄位（contract）**絕不可同一個 migration**，
  且刪欄位**絕不可與「停止使用該欄位」同一個 release**。
- 型別變更同理：先加新欄位雙寫 → 回填 → 切讀 → 停用舊欄位 → 再下一版刪。

### ⚠️ 「停用」要用生成後的 SQL 判定，不是讀原始碼

R2 的驗收條件是「**送到 Postgres 的 SQL 不再提到該欄位**」，而不是「Go 原始碼裡看不到欄位名」。
兩者可以不一致：

- **sqlc 在生成當下就把 `SELECT *` / `RETURNING *` 展開成具名欄位清單**。
  只要欄位還在 declarative schema 裡，用 `SELECT *` 寫的查詢，生成出來的 SQL 就會含它 ——
  即使 Go 那一側早就沒有任何地方讀那個欄位。這種耦合在 `.sql` 的 diff 裡**完全看不見**。
- 對策：R2 把整列查詢改成**顯式欄位清單並略過該欄位**，並加一支機械守衛測試去 grep
  生成檔的 SQL 本體；R3 DROP 之後再改回 `SELECT *`，守衛隨相容層一起移除。

判定方式（不要靠肉眼 review diff）：把**上一版**生成檔裡的每一支 SQL 以 `PREPARE`
打到「已套用本版 migration」的隔離容器，全部通過才算相容。`PREPARE` 在 parse／plan 期
就會抓到 `42703`，不需要真實資料。

### ⚠️ 「沒有資料 / 沒有人在用」不是免疫證明

`42703 undefined_column` 與 `42P01 undefined_table` **在 parse／plan 期就報錯**，
與匹配到幾列毫無關係 —— `UPDATE t SET gone_col='x' WHERE id=<不存在的 id>` 照樣失敗。
所以「這張表在該環境是 0 列」不能當作跳過三段式的理由。

真正決定風險的是**端點可不可達**（路由有沒有註冊、有沒有 feature flag 擋著），
不是前端有沒有在用這個功能。相對地 `23514 check_violation` 才真的需要有寫入才會踩 ——
評估時要把這兩類錯誤分開看。

### ⚠️ `--atomic` 只回滾一半

deploy 用 `helm upgrade --wait --atomic`，失敗時自動回滾 **image**；
但 Atlas／Flyway 都沒有 down migration，**schema 不會回滾**。

在 migrate-先於-deploy 的管線裡，一次失敗的部署留下的是「**舊 code ＋ 新 schema**」
這個兩邊都沒驗證過的組合，而不是回到部署前的狀態。所以含 schema 變更的 release，
rollback 計畫不能只寫「`--atomic` 會自動回滾」，必須明寫 roll-forward 步驟。

---

## 五、PR Review Checklist（含 migration 的 PR 必檢）

- [ ] 此 migration 是否**只含安全變更**（第二節）？若含危險變更，是否已拆成 expand / 停用 / contract **三個** release？
- [ ] 加欄位是否為 `NULL` 或帶 `DEFAULT`？
- [ ] **新欄位的值能否由既有欄位推導（第三節判準）？若能，同一檔內是否有回填 `UPDATE`？恆真的不變式是否加了 `CHECK`？**
- [ ] 加 index 是否用 `CONCURRENTLY`？
- [ ] **舊版 code 在新 schema 上能否正常運作**（surge 視窗安全）？判定依據是「把上一版**生成後**的 SQL 以 `PREPARE` 打到已套用本版 migration 的隔離容器，全部通過」，不是讀原始碼推論。
- [ ] 含 schema 變更的 release，rollback 計畫是否寫了 **roll-forward 步驟**（`--atomic` 只回滾 image，不回滾 schema）？
- [ ] Go：`atlas migrate diff` 生成的 SQL 是否含非預期的破壞性 op（DROP / ALTER TYPE）？
- [ ] Java：Flyway 在啟動時套用，新 migration 是否對「尚在線的舊 pod」無害？
- [ ] contract（刪除）階段是否確認前一版 code 已完全下線？

---

## 六、相關文件

- [PostgreSQL 設計規範與最佳實踐](PostgreSQL%20設計規範與最佳實踐.md)
- 不斷線部署整體設計：issue #939
