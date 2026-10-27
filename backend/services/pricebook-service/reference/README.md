# product_crop_seed（商品 ↔ 作物種子對照表）

P4 Task 13。給「賣家商品名 → 農業部（MOA）作物」的 v1 對應方式：**手工策劃
10–30 個高價值品項的種子對應表**（設計書 §7 commodity-mapping），不做自動
名稱比對。

## 範圍邊界

- 只有種子表本身的載入與名稱解析（`internal/domain/mapping`）與一支對 DB
  驗證解析率的診斷 CLI（`cmd/pricebook-seed-check`）。
- **不建 `product_crop_mapping` 表、不接 product-service**：商品實際綁定哪個
  作物（讀寫）歸 Plan C。P4 建一張沒有讀者的表是投機。
- 設計書 §7 原訂路徑是 `ml/pricebook/reference/product_crop_seed.csv`；該路徑
  因 Level 0 決策已不存在，且 `product_crop_mapping.product_id` 是 TSID、
  CSV 不可能把它當 key commit 下來。這裡改用**穩定名稱**（分類樹 L3 名／
  消費者俗名）當 key，放在服務自己的 `reference/` 目錄下。

## 資料來源與授權

CSV 每一列的 `note` 欄都標了來源，來源只有兩種：

1. `services/ai-service/internal/client/moa/cropname.go` 的
   `cropNameAliases`——俗名 → MOA 官方作物名，每一條都以 2026-07-24 對 MOA
   OpenData API 的實測結果驗證過（命中列數記在該檔案的行內註解）。
2. `services/ai-service/internal/cache/category_index.go` 的
   `synonyms`——消費者用詞 → 我方分類樹（`category` 服務）實際存在的
   L2/L3 名稱。

兩張表都是 ai-service 團隊已經逐列人工驗證過的資料，本表**不重新發明任何
對應關係**，只挑其中的高價值品項、標明來源，並對兩表間的已知分歧做裁決。
這兩個檔案本身不屬於 pricebook-service，本表只讀不改。

## match_kind 的兩種來源

| match_kind | 意思 | 可信度依據 |
|---|---|---|
| `consumer_alias` | 消費者可能打出來的俗名，需要正規化＋轉換才對得上 MOA 官方名 | `cropNameAliases` 的實測命中列數，或 `synonyms` 的分類樹確認 |
| `category_l3` | 我方分類樹的 L2/L3 名稱本身恰好等於 MOA 官方作物名，不需轉換 | `synonyms` 的 value 端證實該名稱確實存在於分類樹，`cropNameAliases` 的命中列數佐證它同時是有效 MOA 作物名 |

兩者都是「名稱 → 作物」，混在一起會讓日後的維護無法判斷該不該改某一列，所以
分開記。

## 已知分歧的裁決

`cropNameAliases` 與 `synonyms` 部分條目的目標名稱不同，或只存在於其中一張：

| alias_name | cropNameAliases | synonyms | 裁決 |
|---|---|---|---|
| 大白菜 | 包心白菜（16 列） | 包心白 | **以 MOA（包心白菜）為準** |
| 香菇 | （無此條目） | 濕香菇 | 只存在於 synonyms，以其為準 |
| 青花菜 | （無此條目） | 青花苔 | 只存在於 synonyms，以其為準 |
| 蔥 | （無此條目） | 青蔥 | 只存在於 synonyms，以其為準 |

**裁決原則**：本表的目的是對應到 `crop_group.name` / `crop.crop_code`——
我方 DB 是從 MOA 行情資料建的，`crop_group.name` 存的就是 MOA 官方大類名。
分類樹的名稱是給消費者導覽用的、不保證跟 MOA 命名一致；MOA 官方名不存在的
分歧就沒有取捨問題（後三條），存在分歧時（大白菜）以 MOA 為準，否則
`crop_group_name` 在 DB 裡查不到對應列。

## 每一列怎麼驗證的

CSV 本身的**結構**驗證由 `internal/domain/mapping.LoadSeed` 在載入時強制執行
（sha256、欄位恰填一個、alias_name 不重複、match_kind 合法——細節見該檔案
與 `seed_test.go`）。

CSV 的**內容對不對得上實際 DB 資料**（`crop_group_name` 是否真的存在於
`crop_group.name`、`crop_code` 是否存在於 `crop.crop_code`、`market_code`
若有填是否存在於 `market.market_code`）由 `cmd/pricebook-seed-check` 對指定
的資料庫跑一次驗證，逐列印出結果與整體解析率摘要。這是一支診斷工具，不是
部署閘門——`crop_group.name` 不是 unique（Task 2 之後 `group_code` 的 hash
含 category，同一個大類名在不同 category 下是不同的列），查到多筆時 CLI
會把全部列出來標記為需人工裁決，不會自動選一筆。

## manifest 的用途

`product_crop_seed.manifest.json` 記錄 CSV 內容的 sha256、列數與來源清單。
`LoadSeed` 載入時會重算 CSV 的 sha256 並與 manifest 比對，**不符時回
error、不是警告**——CSV 被改過但 manifest 沒同步更新，代表有人繞過了下面
這個改動流程的審查（例如直接改一格資料但沒人 review 到那個 diff）。

## 改動流程

1. 改 `product_crop_seed.csv`（新增/修改列時，`note` 欄務必標明來源：
   `cropNameAliases` 的哪一條、`synonyms` 的哪一條，或分歧裁決理由）。
2. 重算 manifest：

   ```bash
   cd services/pricebook-service/reference
   python3 -c "
   import hashlib, json, pathlib
   p = pathlib.Path('product_crop_seed.csv')
   digest = hashlib.sha256(p.read_bytes()).hexdigest()
   rows = sum(1 for _ in p.open()) - 1
   pathlib.Path('product_crop_seed.manifest.json').write_text(json.dumps({
       'schema': 'hawkr.product-crop-seed/v1',
       'file': p.name,
       'sha256': digest,
       'row_count': rows,
       'sources': [
           'services/ai-service/internal/client/moa/cropname.go (cropNameAliases)',
           'services/ai-service/internal/cache/category_index.go (synonyms)',
           'docs/superpowers/specs/2026-07-27-pricebook-service-design.md 13 (G2 量測)',
       ],
   }, indent=2, ensure_ascii=False) + '\n')
   print(digest, rows)
   "
   ```

3. 跑解析率檢查（需要一個可連到的 Postgres；預設走服務自己的
   `internal/config.Load()`，DB_* 環境變數比照其他 CLI）：

   ```bash
   cd services/pricebook-service
   DB_HOST=<host> DB_USER=... DB_PASSWORD=... DB_NAME=hawkr_pricebook \
     go run ./cmd/pricebook-seed-check
   ```

   看輸出確認新增/修改的列狀態是 `OK`；`NOT_FOUND` 代表 DB 裡還沒有這個
   `crop_group`/`crop`（可能要等下一次 MOA ingest 或分類同步）；`AMBIGUOUS`
   代表要在 CSV 裡把 `crop_group_name` 換成更精確的 `crop_code`，或等待
   mapping 的下一階段補上消歧義欄位。

4. `git add product_crop_seed.csv product_crop_seed.manifest.json` 兩個檔案
   一起 commit（缺一個會讓下次 `LoadSeed` 直接拒收）。

## 為什麼不做自動名稱比對

設計書 §7：現有 `get_seasonal_price_suggestion`
（`services/ai-service/internal/agent/tools.go:368-405`）就是自動比對的
失敗版本——它把賣家商品名（例「新鮮高麗菜 600g」）直接當 `CropName` 丟給
MOA API 做子字串查詢，而 MOA 要的是「甘藍-初秋」這種官方命名。這個做法有
兩種已實測的失敗模式（見 `services/ai-service/internal/client/moa/cropname.go`
頂端註解）：

- 長字串比對不到：「高麗菜」「新鮮高麗菜」「高麗菜 600g」全部回 0 列，因為
  官方名是「甘藍」。
- 短字串比對到錯的東西：「蔥」會同時命中「青蔥-北蔥」與「洋蔥」。

命中率沒有被上游量測過，但形狀上不可能高。兩種失敗模式都不是「差一點
對」，加模糊比對只會把「查無結果」換成「查到錯的」——而給錯作物的定價
建議比不給建議更糟（設計書 §7 原文：「不產生假建議」）。

因此 `internal/domain/mapping.Seed.Resolve` 只做**正規化**（去修飾前綴、去
括號內容、去帶單位的規格尾綴）後的**精確**查表，正規化與模糊比對的分界寫
在該函式的註解裡。查不到就回 `false`，呼叫端走「未對應」流程：UI 顯示
`unmapped` 狀態與「設定對應」入口，不產生假建議。未對應的商品（本表以外
的品項、或分辨不出來的別名）就是要人工介入，不是程式自動猜。

這條規則的範圍是**取價**：商品對應到的 crop 就是定價建議的最終答案，對錯沒有
後路。ai-service 採購助手的**分類檢索**另用離線 LLM 生成、不逐列覆核的 L2
別名表，是刻意的例外：那裡的候選只是讓買家點選，卡片永遠留著「自行填寫單價」，
錯別名的代價是多一次選擇，不是錯價；它也不改寫本表或 crop 映射。理由見 ADR
`docs/adr/2026-09-13-category-alias-offline-generation.md`。
