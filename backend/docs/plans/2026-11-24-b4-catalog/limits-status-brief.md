# B4-2b 追加簡報：內容上限（L）與商品狀態切換（S）（v2，已依 critique 修訂）

證據標記：【量】本次在 repo 內查到（附位置）；【查】critique 對照外部頁面確認（附來源）；【二手】只有第三方來源；【推】推論；【未查證】沒有可靠來源，不拿來當任何數字的依據。

## 前提題：這段程式是否排定要重寫、搬走或刪除？
- `catalog/internal/model/product.go` 的常數與 `CanTransition`、`catalog/db/schema.sql` 是 B4-2 新寫的單體模組，B4 計畫沒有排定重寫【量：decision-brief D16–D19 以此為終態】。
- 會被丟棄的是舊 Java `product-service`（`SellerProductOffShelfServiceTest.java:34-35`【量】）與 client 手寫呼叫；那是我們自己的遷移排程 → SELF-IMPOSED。
- 唯一外部約束：系統未上線，沒有既有資料或舊版 App（根 `AGENTS.md`「上線前不受相容性約束」）。
- 重工量（修正）：L 動 `product.go`、`description/description.go`、`schema.sql` 四條 CHECK、`api/` catalog proto、`uitext-gen` 新增 extractor 與 Go/TS/Dart 三個 emitter、seller-app 編輯器改讀生成值；S 動 `CanTransition`、寫入驗證與測試。

---

## 決策 L：內容上限怎麼定、放在哪

### 建議
1. **定值原則**：每個上限一條來源理由：(a) 同類平台公開上限（只算【查】）、(b) 我們 client 版面／編輯器既有限制、(c) payload 或儲存預算、(d) 產品選擇（明說是我們定的）。【二手】【未查證】只能當旁證。
2. **描述改為「整份描述總字數 3000」**：seller-app 的 3000 比對的是 `_totalLength`＝所有區塊 `block.text.length` 加總（`rich_text_editor.dart:30, 337-342, 1007`【量】），是整份描述上限，不是每段。映射：`MaxDescriptionRunes = 3000`，計入段落、清單項、表格格的文字（不計 alt）；刪掉 `MaxParagraphRunes`（被總數涵蓋）。表格 50×10、格 200 與清單 50 項、項 200 保留為結構上限（一格／一項不得超過 200，總數另受 3000 限）。注意 Dart `String.length` 算 UTF-16 單位、Go 算 rune，emoji 會不一致 → 生成值同一個，client 計數器改用 rune 計數【推】。seller-app 表格／清單的文字是否計入 `block.text`【未查證】，實作時對齊。
3. **唯一來源是 `api/` 的自訂 field option**（`hawkr.common.v1alpha1.limits`：max_items、max_runes），由 `uitext-gen` 生成 Go/TS/Dart 常數。這延續 `step10f-7-design.md` D1 的既有決定「自訂 option，不引入 protovalidate」【量：step10f-7-design.md:9, 13】。但不是「同一條路直接重用」：`backend/cmd/uitext-gen/accepted.go` 寫死只收 `repeated PolicyCategory`【量，critique】，要新寫 extractor 與三個 emitter。
4. **DB CHECK 與唯一來源的同步**：建議**保留 CHECK 當最後防線，加 drift test**：測試解析 `schema.sql` 的 CHECK 字面值（標題 100、軸名 20、選項值 40、軸 position IN (1,2)），斷言等於生成的 Go 常數。repo 已有同型先例：`media/internal/model/lifecycle_test.go:13-19` 解析 `schema.sql` 的 CHECK 清單比對 Go 值【量】。不選「由 option 生成 CHECK」（atlas 的 schema.sql 是手寫 SSOT，生成 SQL 要動 migration 流程，成本高於一個測試）；不選「刪掉 DB 重複」（業界基線是 DB 保留不變式當防線，應用層驗證有繞過路徑時兜底）。描述的長度不進 CHECK（JSONB，已由寫入端驗證，`schema.sql:41-42`【量】）。
5. 建議值：

| 上限 | 現值 | 建議 | 來源理由 |
|---|---|---|---|
| 主圖 | 9 | 9 | (b) buyer-app `maxImages = 9`（`product_image_thumbnail_grid.dart:19`【量】）；蝦皮 9【二手】 |
| 標題 | 100 runes | 100 | (d) 產品選擇＋(c) 列表卡片兩行可容；蝦皮 80/120/255 互相矛盾、Shopify 255【未查證】→ 不以平台為據 |
| 選項軸 | 2 | 2 | D18 已拍【量】；蝦皮 2 層【二手】 |
| 軸名 | 20 runes | 20 | (b)(d)：軸名是「顏色」「尺寸」類短標籤，seller-app 選項 UI 以單行 chip 呈現【推】；無平台來源 |
| 選項值 | 40 runes | 40 | (d)：值如「XL」「深海藍／霧面」，40 容得下型號字串，且 SKU 名＝兩值拼接 ≤ 80+分隔，仍短於標題 100【推】 |
| 每軸值數 | 50 | 50 | (d)；實際被 SKU 100 先綁住（2 軸時 50×2 已達上限）；蝦皮 20 或 50【未查證，來源矛盾】 |
| SKU 數 | 100 | 100 | (d) 獨立上限，先於「軸×值」（50×50=2500）生效；Shopify 2048 是 2025-10-15 對全商家開放、此前 100【查：shopify.dev changelog】，台灣中小賣家用不到 2048 |
| 描述總字數 | 段落 5000 | 總 3000 | (b) seller-app 總數 3000【量】；蝦皮描述 3000【二手】 |
| 表格 50×10、格 200；清單 50 項、項 200 | 同 | 同 | (b) `pasted_table_parser.dart:44-45`【量】；清單類推表格格長 |
| alt | 200 runes | 200 | (b) 與表格格、清單項同一個「短文字」級距；alt 寫作指引一般建議一兩句話【推】 |
| 描述圖片 | 20 | 20 | (d)；蝦皮約 12【未查證，不採】 |
| 描述區塊 | 100 | 100 | (c) 單體 Connect 讀取上限是 `maxRequestBytes = 2 << 20`（2 MiB，`internal/httpapi/server.go:181, 284`【量】；connect-go 未設時無預設上限）。總字數 3000 後文字 ≤ 12 KB，最大的是表格結構：50×10×200 runes×4 B ≈ 400 KB/表，100 區塊全為滿表會超過 2 MiB → 結構上限仍要有；建議另限「每份描述表格 ≤ 5」【推】，或接受 3000 總字數已讓滿表不可能（格文字計入總數時，所有表格合計 ≤ 3000 runes）——若 2. 的計入規則成立，100 區塊安全 |

### 業界基線
- protobuf 生態：protovalidate（`buf.validate`）把 `max_len`、`max_items` 寫在 proto；hawkr 已決定不引入（step10f-7 D1【量】），`rg "buf.validate|protovalidate" api backend packages` 無程式命中【量】。
- Shopify：mutation 以 `userErrors` 回報驗證錯誤，client 以後端錯誤為準【推，未逐頁查】。
- 蝦皮 `v2.product.get_item_limit`（client 讀伺服器上限）：頁面擋抓取，存在但欄位【未查證】。
- 資料庫不變式：DB CHECK 作兜底、應用層給友善錯誤，是一般做法【推】。

### 現有機制為什麼存在
- `product.go:40-41` 註解：表格與清單照 seller-app，其餘「待確認」【量】。Java 時期上限分散四端。

### 選項
- **A 自訂 option＋uitext-gen 生成＋CHECK drift test（建議）**。失敗條件：新 extractor/三 emitter 在 B4-3 前做不完 → 先生 Go＋drift test，client 暫靠伺服器 `InvalidInputError` 顯示。
- **B protovalidate**：需新依賴（Will 核准）且推翻 step10f-7 D1；Dart runtime 有無【未查證】。失敗條件：client 端無法用，SSOT 只到伺服器。
- **C 伺服器 RPC 回傳上限**：失敗條件：多一支 RPC＋快取，未上線無「不發版調整」需求。
- **D 維持 Go 常數、client 各寫**：失敗條件：四端＋DB 五份漂移。

### 推翻建議的事實
- 營運要不發版調上限 → C。seller-app `block.text` 不含表格／清單文字且決定維持 → 總字數只計段落，表格須另設數量上限。

### 可逆性與代價
- 數值：上線前任意改；上線後放寬安全、收緊要處理既有資料。SSOT：A→C 低成本。drift test 可隨時刪。

### 限制帳本
| 限制 | 分類 | 來源 |
|---|---|---|
| 新增依賴需 Will 同意 | REAL（規則） | 根 `AGENTS.md`「依賴與供應鏈」 |
| step10f-7 D1「不引入 protovalidate」 | SELF-IMPOSED（我們的先例，可重開） | `step10f-7-design.md:9` |
| Dart 無官方 protovalidate runtime | 【未查證】，不計入 | — |
| seller-app 3000、50×10 | SELF-IMPOSED | `rich_text_editor.dart:30`、`pasted_table_parser.dart:44` |
| Connect 讀取 2 MiB | SELF-IMPOSED（可調，但現值合理） | `server.go:181` |
| atlas `schema.sql` 手寫 | SELF-IMPOSED | 決定 CHECK 不生成、改 drift test |

---

## 決策 S：DRAFT／ACTIVE／ARCHIVED 的切換

### 建議
- **任兩狀態可互切**（含 ACTIVE／ARCHIVED → DRAFT），`CanTransition` 只檢查 `to.Valid() && to != s`；回草稿不清 `first_published_at`。
- **ACTIVE 不變式在「狀態為 ACTIVE 的每一次寫入」都檢查**（切換進 ACTIVE、以及 ACTIVE 商品的全量編輯 `ProductInput`），不只在切換時；違反就拒絕該次寫入（不自動降回 DRAFT）。
- 不變式實際只需新增一條：**至少 1 張主圖**。其餘已由每次寫入保證：分類 `category_id UUID NOT NULL`（`schema.sql:46`【量】）且寫入必填（`validate.go:38`【量】）；SKU 至少 1 個（`normalizeSkus` 空清單回 `ReasonRequired`，`validate.go:170`【量】）；價格 `skus_price_check CHECK (price > 0)`（`schema.sql:114`【量】）→ 原「有價 SKU」檢查多餘，刪除。
- `first_published_at` 只在第一次進 ACTIVE 寫入（CHECK `products_first_published_check`【量：schema.sql:65】）。刪除（`deleted_at`）正交，刪後不可切換；治理下架不進 status（D7【量：decision-brief:87】）。

### 業界基線
- Shopify `ProductStatus`：ACTIVE／ARCHIVED／DRAFT（2025-10 起另有 UNLISTED）；**「Duplicated and unarchived products default to DRAFT」**——取消封存落在 DRAFT，支持允許 →DRAFT【查：shopify.dev ProductStatus】。ACTIVE ≠ 已發布到銷售管道；Shopify 不以內容檢查擋 ACTIVE【查：同頁只寫狀態與發布分離】。完整切換矩陣無官方頁【未查證】。
- WooCommerce draft／pending／private／publish 互切、trash 另一軸【未查證】。蝦皮賣家 NORMAL／UNLIST 與平台 BANNED 分離【未查證】。
- 我們比 Shopify 多一條主圖不變式：Shopify 的 ACTIVE 不等於買家看得到，我們的 ACTIVE 直接進公開列表（`products_public_new_idx WHERE status='ACTIVE'`【量】），所以要擋。

### 現有機制為什麼存在
- 現行 D→A、A→Ar、Ar→A 來自 D16 把 Java 4003 對應成 ARCHIVED【量：decision-brief:150】；「不能回草稿」無寫明理由，推測怕失去「曾發布」語意，但已由 `first_published_at` 記錄【推】。

### 選項
- **A 全互切＋每次寫入檢查 ACTIVE 不變式（建議）**。失敗條件：有副作用把 DRAFT 當「從未曝光」→ 改看 `first_published_at`。
- **B 現狀**。失敗條件：已上架想藏起來重編只能 ARCHIVED，語意混淆，且沒保護任何不變式。
- **C 只在切換時檢查**。失敗條件：ACTIVE 商品編輯時移除全部主圖仍公開（critique 指出的洞）。

### 推翻建議的事實
- 法規或營運要求上架審核 → 加 PENDING_REVIEW（D16 已列觸發）。
- 要讓賣家「先存半成品」不中斷 → 改為違反時自動降 DRAFT，而非拒絕。

### 可逆性與代價
- 低：規則在 `CanTransition` 與 `validate.go`；資料不受影響，上線後收緊不需遷移。

### 限制帳本
| 限制 | 分類 | 來源 |
|---|---|---|
| 訂單、評價引用商品列 → 只能軟刪 | REAL（資料完整性） | `schema.sql:37`、D16 |
| 治理措施由 seller 模組擁有 | REAL（模組邊界，D7） | decision-brief:87 |
| 「照 Java 4003」「與 D16 寫法一致」 | SELF-IMPOSED | D16、decision-brief:150 |

---

## 要 Will 拍板的題目
1. L-SSOT：上限放 `api/` 自訂 option＋`uitext-gen` 生成三端（建議），不用 protovalidate（沿用 step10f-7 D1）。
2. L-DB：DB CHECK 保留、加 drift test 比對生成常數（建議），而非生成 CHECK 或刪除 CHECK。
3. L-描述：改為整份描述總字數 3000、計入段落／清單／表格文字，刪每段上限（建議）。
4. L-數值：標題 100、軸名 20、選項值 40、alt 200、每軸 50、SKU 100、描述圖 20、區塊 100 都屬我們的產品選擇（理由見表），照現值（建議）。
5. S-切換：三狀態全互切，回草稿保留 `first_published_at`（建議）。
6. S-不變式：ACTIVE 時每次寫入要求 ≥1 主圖，違反就拒絕（建議），而非自動降 DRAFT。
