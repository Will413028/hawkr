# 資料列的識別依資料性質決定：實體用 UUIDv7，值的集合與關聯表用複合鍵

| 項目 | 值 |
|---|---|
| 狀態 | accepted |
| 日期 | 2026-09-25 |
| 影響範圍 | 單體所有模組的主鍵與對外 ID；PostgreSQL 設計規範 §一、§二 |
| 上層決策 | `2026-09-24-go-modular-monolith.md` D1、D3、D8 |

## 脈絡

單體起初沿用公司時期的主鍵規範（PostgreSQL 設計規範 §二：對外的業務表用應用層產生的 TSID，內部表用 `BIGSERIAL`，業務表禁用 uuid），`pkg/tsid` 也照搬成 `platform/tsid`。TSID 是 42 bit 毫秒加 22 bit 序號，沒有 node 位元：serve 的多個 replica 加上 worker 同時寫入時，同一毫秒會產生相同的 ID；而且以 1970 為 epoch 左移 22 位，2039-09-07 之後會變成負數。

約束：

- 外部：單一 PostgreSQL 18、每個模組一個 schema、不建跨 schema FK（D3）；多個 process 同時寫入（D1）。
- 外部：對外 JSON 的 ID 是字串（JavaScript 的 Number 只精確到 2^53）。
- 繼承：B1 之前 media 仍是 Java 服務，業務列要先用自己的 ID 登記圖片引用，再寫入業務列，所以寫入前就要有 ID。來自 media 的順序契約（`internal/modules/media`）；B1 前仍成立，之後可以在同一個 transaction 完成，所以不是選型的主因。
- 繼承：還在運作的 Java 服務把帳號等 ID 存成 `BIGINT`（例如 media、product-import、coupon 的 V1 migration）。來自 D8 的並存；到階段 B 改寫完為止仍成立。
- 拿掉：規範 §二的三分法與「業務表禁用 uuid」。它的前提是每個 Java 服務有自己的資料庫、再加上依部署分配的 node 編號，單體兩者都沒有；系統也還沒上線，沒有相容的包袱。

## 考慮過的方案

| 方案 | 代價 | 判定 |
|---|---|---|
| 基準：UUIDv7。免協調、時間有序（B-tree 的 locality 接近連號）、至少 62 bit 隨機不可猜（[RFC 9562](https://www.rfc-editor.org/rfc/rfc9562) §2.1、§5.7、§6.2）。Postgres 18 內建 [`uuidv7()`](https://www.postgresql.org/docs/18/functions-uuid.html)；Buildkite 由整數改成 v7（[文章](https://buildkite.com/blog/goodbye-integers-hello-uuids)） | 每個鍵多 8 bytes，主鍵、外鍵與索引都要付 | 實體採用（見 E） |
| A. TSID 補上 node 位元（Snowflake 型，[Instagram](https://instagram-engineering.com/sharding-ids-at-instagram-1cf5a71e5a5c)） | 單一資料庫不需要自己做分散式發號。node 編號變成永久的部署狀態：k8s Deployment 沒有穩定序號，serve、worker、migrate、測試都要分到不重複的號，分錯就靜默產生重複主鍵。公司時期的分配表就有兩個服務共用同一段的例子 | 否決 |
| B. 全部 `bigint GENERATED ALWAYS AS IDENTITY`（單一 PG 的預設做法，[PG wiki: Don't use serial](https://wiki.postgresql.org/wiki/Don%27t_Do_This#Don.27t_use_serial)） | 連號對外可列舉，洩漏營運量，也讓越權存取更容易試出來。資料跨 shard 搬移時 ID 與所有引用都要改寫 | 實體否決；只當有量測理由的例外 |
| C. 內部 bigint 加對外 UUID（dual key） | 每個邊界都要換算兩把鍵，跨模組時還要決定存哪一把 | 否決 |
| D. 所有表一律單欄 UUIDv7，包括子列、關聯列與紀錄表 | 值的集合沒有身分：整批替換的寫法下，每一列的 UUID 每次存檔都換一個，是假的身分；關聯列本來就由兩端的鍵識別 | 否決 |
| E. 依資料性質決定：身分、排序、歸屬、分區或分片鍵分開考慮，再決定主鍵由哪些欄組成 | 要逐表判斷；規則寫在下方的表與設計規範 §二 | 採用 |

UUIDv4 不列入：隨機插入破壞索引的 locality。

## 決策

- **D1**：依資料性質決定識別。

  | 資料的性質 | 識別 |
  |---|---|
  | 實體：會被 API、其他模組、media、事件或 job 以 ID 指稱（包括有身分的子資料，例如要單筆退款的訂單明細） | `id uuid PRIMARY KEY`，不設 DEFAULT，應用層用 Go 標準庫的 `uuid.NewV7()` 產生；seed 與 backfill 在 INSERT 裡明寫 `uuidv7()` |
  | 值的集合：只隨上層整批讀寫、不被引用（例如食譜的食材、步驟） | `(parent_id, position)`；需要身分時，連同寫入方式一起升級成實體 |
  | 只在上層底下識別、但身分要穩定 | `(parent_id, item_no)`：item_no 建立後不變，排序另存 position，配號要處理併發 |
  | 純關聯表 | `(a_id, b_id)`，需要反查時加反向索引 |
  | 讀取模型、設定、1:1 延伸表 | 來源實體的 ID 或自然鍵 |
  | 大量、只追加的紀錄 | `uuid DEFAULT uuidv7()`；只有量測證明鍵寬是瓶頸時才改 identity，另立 ADR |
  | 給人看的編號（訂單號、售後案號） | 另開 `UNIQUE` 欄，由擁有的模組設計格式，不得由 ID 截斷或取模 |
  | 持有即授權的 token | 不是 ID：至少 128 bit 的 CSPRNG，存雜湊（RFC 9562 §8：UUID 不能當權限憑證） |
  | 過渡：ID 仍被 Java 存成 `BIGINT` 的模組（identity、buyer、seller 等） | 暫用 `bigint GENERATED ALWAYS AS IDENTITY`；最後一個 Java 消費者改寫後，以一個 migration 換成 uuid |

- **D2**：ID 的順序只代表大約的建立時間；業務上的排序用明確的欄位（例如 `created_at`），ID 只當排序的最後一個 tiebreaker。
- **D3**：分區、分片等到真的需要時才逐表設計，現在不預先加 tenant 欄。
- **D4**：刪除 `platform/tsid`，改用 Go 1.27 標準庫的 `uuid`（`NewV7` 不回傳錯誤、同一個 process 內遞增），不另包一層、也不用第三方套件；PostgreSQL 設計規範 §二對單體改寫成本 ADR 的規則，舊服務在改寫前照舊。
- **D5**：對外契約把 ID 標成 uuid 格式的字串；前端一律當成不透明字串，不解析成數字。

## 理由

- **實體用 UUIDv7、不用 identity**：在單一資料庫內，sequence 被多個模組引用並不會撞號；否決 identity 的理由是可列舉與資料跨 shard 搬移時要改寫 ID 與所有引用。UUID 的代價實測：20 萬筆的主鍵索引 6.2 MB，bigint 是 4.4 MB。
- **實體不設 DEFAULT**：寫入前就要拿 ID 去登記引用或事件。設了 DEFAULT，漏給 ID 時 DB 會靜默產生另一個 ID，登記的引用就指向不存在的列；不設的話，漏給 ID 是 NOT NULL 違反，當下就失敗。
- **值的集合用複合鍵**：子資料要不要有 ID 由寫入方式決定，不是由它是不是子表決定。整批替換的集合沒有身分；有身分的子資料照樣用 UUID。
- **紀錄表也用 UUIDv7、不預設 identity**：只有一種 surrogate 型別，Go 型別與 sqlc override 只要一套，也能用 `RANGE (id)` 分區。現在沒有量測數字證明省下的 8 bytes 值得多一套規則。
- **過渡期保留 bigint**：當下就換成 UUID 要同時改 10 個 Java 服務的欄位型別，違反 D8 的「舊服務只做維持運作所需的修改」。代價是這些模組之後要多做一次 migration；上線前這個 migration 沒有資料成本。

## 後果

**接受的代價**

- UUID 每個鍵多 8 bytes，主鍵、外鍵與每個含 ID 的索引都要付。
- ID 仍是 `BIGINT` 的模組（A2、A3 起）在最後一個 Java 消費者退場後要多做一次換型別的 migration。
- 前端現有把 ID 解析成數字的程式要改成字串（例如 buyer-app 的 `recipe_model.dart`，隨 Connect 試點步驟 6）。
- ordering（B8）的訂單號要重新設計，不照搬 `OrderNoGenerator`：`"O" + TSID % 10^9` 加上 `UNIQUE(order_no)`，約 3.7 萬筆訂單就有一半機率撞號（生日界）。

**應該重新評估的條件**

- 量測顯示鍵寬是熱門路徑的瓶頸：該表改 identity，另立 ADR。
- 需要只憑 ID 就路由到分片，或改用依 key range 分散的資料庫（Spanner、CockroachDB 的時間有序主鍵會集中寫入同一段）：重評 D1。
