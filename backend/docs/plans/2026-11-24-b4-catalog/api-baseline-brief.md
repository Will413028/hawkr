# B4-3 catalog Connect API：業界基準對照與設計決策簡報

快照：dev `cb273c7d9`。輸入：`b4-3-proto-draft.md`（下稱「草稿」）、計畫 `backend/docs/plans/2026-11-24-b4-catalog.md`、規範 `docs/共通規範/API 設計規範（Connect）.md`（下稱「規範」）、特性測試 `backend/internal/httpapi/catalog/characterization/scenarios_test.go`。

證據標記：【量】在 repo 讀檔或跑指令（附指令）；【文件】公開文件（附 URL；本 session 沒有重新開啟網頁，內容依既有知識，交付前建議抽查標★的幾條）；【推】推論。

---


## 拍板與審查修正（2026-11-26，Will：全部照建議）

本簡報經獨立審查，下列修正優先於正文：

- **Q13 改成現在就做**：website（`website/features/seller-center/products/hooks/useProductForm.ts:206-210`）與 seller-app（`seller-app/lib/features/product/presentation/product_setup_spec_screen.dart:306,737`）的商品表單直接輸入 SKU 庫存絕對值。B4-3 新增 `SetStock(sku_id, quantity, expected_quantity)`，現值不等於預期就回 `FAILED_PRECONDITION`；表單改呼叫它。D23 的 `AdjustStock`（有號增減）並存。業界：Shopify `inventorySetQuantities` 的比對值【文件，欄位名在新版可能改為 `changeFromQuantity`，實作前查】。
- **Q6 拆兩題**：庫存頁的 10 不是常數，是依銷售速度算的建議安全庫存（最低 5，無銷量時 10，`website/features/seller-center/inventory/InventoryPage.tsx:39-48,64-74,299-305`）。6a 賣家提醒門檻：每商品選填、預設 5，以 SKU 判斷，只算 ACTIVE，售完另計，伺服器計數。6b 依銷售速度的建議安全庫存保留，改由伺服器提供。兩者都是 UI 改動，Will 已同意。
- **Q10**：website 已有「商品已下架」頁（`ProductUnavailableView.tsx:68`）與不可購買分支（`ProductPurchaseBox.tsx:155-160`）；buyer-app 商品頁沒有，需新做（文案 Will 已同意走 UI 流程）。Shopify「archived 回 404」改為【推】；Storefront API 對未發布商品回 null【文件】。
- **Q14（新增）**：公開商品不暴露賣家狀態 enum，只回公開用的 `availability`（可購買／不可購買）。
- **Q4**：Stripe 是「至少 24 小時後可能清除」【文件】；同鍵併發用 `INSERT … ON CONFLICT DO NOTHING` 後讀回並比對請求雜湊，不靠唯一衝突等待；寫測試。`SetStock` 也必填 `request_id`。Shopify `inventoryAdjustQuantities` 2026-04 版起必填冪等鍵【文件】。
- **Q5**：「AIP-154 允許排除計算欄位」改為【推】，理由改為 Shopify 商品與庫存分屬兩個資源。
- **Q3**：etag 放請求是偏離 AIP-134（資源帶 etag）的 SELF-IMPOSED 選擇，理由是讀寫型別不同，寫進規範第七節。
- **Q1**：「收藏、追蹤屬 buyer」是 SELF-IMPOSED（自己的模組邊界），理由是依賴方向。
- **Q8**：`total_size` 選填且可為估計值【文件，AIP-158】；後台介面不顯示頁碼改為【推】。

## 0. 前提問題

### 0a. 這一步改的程式之後會不會被重寫、搬走或刪除？

- B4-3 產出：`api/hawkr/catalog/v1alpha1/catalog.proto`、新的 `api/hawkr/admin/catalog/v1alpha1/catalog.proto`、`backend/internal/httpapi/catalog` 的 handler、模組補的查詢與 schema（`version`、`request_id`、篩選、搜尋、低庫存）。
- 後續步驟對它們的處置【量：`sed -n 85,104p backend/docs/plans/2026-11-24-b4-catalog.md`】：
  - B4-4 新建 `legacy/catalogserver`（B6／B8 刪），**不碰** B4-3 的 proto 與 handler。
  - B4-5 在公開讀取加限制措施排除、B4-5d 加 `CatalogDelegatedService`：都是**附加**，不改 B4-3 的 RPC。
  - B4-6 刪 gateway 路由與四端手寫 client、改用生成 client：B4-3 的 proto 是 B4-6 的輸入，不被丟棄。
- 結論：B4-3 的程式是終局，沒有排定的重寫；丟棄量 0 檔【推，依上列步驟範圍文字】。所以沒有「反正之後會改」的理由讓任何決策偏離業界基準。唯一的時序約束是 B4-6 才改 client：B4-3 定的契約若需要 UI 改動，UI 在 B4-6 才落地，但**契約本身要在 B4-3 定成終局形狀**（SELF-IMPOSED 分期，不是 REAL）。

### 0b. 草稿本身的框架有沒有錯的前提？

有一個：**草稿以「舊路由逐條對照」當主要輸入**（第 3 節 59 條路由、46 支 gRPC）。這保證沒漏掉消費者（計畫 D5 的正確用途），但也把舊系統的形狀帶進新契約，至少四處被帶歪：

1. `followed_*_only` 留在公開列表（Q2）——舊 REST 的參數，不是「追蹤中」畫面該有的資源。
2. 賣家清單砍掉 `keyword`，理由是舊賣家 client 沒送；但 website 庫存頁**有搜尋框，是在 client 對已抓的一頁做過濾**【量：`rg -n "search" website/features/seller-center/inventory/InventoryPage.tsx` → 第 205、329–337、444–449 行】。改成伺服器分頁後 client 過濾只看得到一頁，等於壞掉（Q11）。
3. 下架商品的公開頁：舊行為「讀得到但不可購買」（特性測試 `scenarios_test.go:110-113`），草稿改成 not_found 卻沒列成決策（Q10）。
4. 低庫存門檻：舊表單有每商品的門檻欄位，新 schema 丟了它，草稿以「沒有新增資料」為由選常數（Q6）。

正確的輸入順序是：**畫面需求（client-inventory 的呼叫點，看畫面在做什麼而不是送了哪些參數）→ 業界資源模型 → 舊路由只用來確認沒漏**。本簡報照這個順序重審。特性測試是舊行為證據，不是規格：Q10、Q11 若照本建議，對應情境要改（見第 3 節）。

---

## 1. 逐項決策

每項格式：建議 → 業界基準 → 現有機制為何存在 → 選項與失敗條件 → 推翻條件 → 可逆性 → 限制帳。

### Q1　service 切分與命名（要不要買家 service）

**建議**：同草稿，三支：`CatalogPublicService`、`SellerCatalogAppService`、`CatalogAdminService`；**不開** catalog 的買家 service。觀看者相關的讀取（收藏、追蹤、補貨訂閱、追蹤店家的商品流）放在擁有那些關係的 buyer 模組的 `BuyerAppService`。

- 業界基準：
  - Shopify 依呼叫者分兩套 API：Storefront API（買家、公開、可快取）與 Admin API（商家）；買家的個人狀態（customer、cart）在 Storefront 以 buyer identity 帶入，不另開商品 API【文件】https://shopify.dev/docs/api/storefront 、https://shopify.dev/docs/api/admin-graphql
  - Google AIP 以資源為中心、授權交給 IAM，不依角色切 service【文件】https://google.aip.dev/121 。
  - Amazon SP-API（賣家）與商店前台完全分離【文件】https://developer-docs.amazon.com/sp-api/
- 現有機制：規範 §二要求 service 名表示呼叫身分（`<角色><模組>AppService`），因為 Connect handler 以 service 宣告授權與快取類別（§九的 `Cache-Control` 依 service 類別決定）【量：規範 §二、§九】。
- 選項：
  - (a) 三支（建議）。失敗條件：出現 catalog 自己擁有、只有登入買家能讀的資料（目前沒有：收藏、追蹤都屬 buyer）。
  - (b) 加 `BuyerCatalogAppService`。失敗條件：它的內容其實是 buyer 擁有的關係，catalog 變成要讀 buyer 的資料，依賴方向反了。
  - (c) 照 AIP 單一 service、靠授權區分。失敗條件：§九的快取類別無法以 service 判定，與全 repo 慣例衝突。
- 推翻條件：catalog 新增「買家對商品的私有狀態」且由 catalog 擁有（例如瀏覽紀錄歸 catalog）。
- 可逆性：加一支 service 是附加變更，成本低。
- 限制帳：

| 限制 | 類別 | 來源 |
|---|---|---|
| 規範 §二 依角色切 service、§九 依 service 定快取 | SELF-IMPOSED（自己的規範），但跨所有模組一致，改它不屬 B4-3 | 規範 |
| 收藏、追蹤屬 buyer 模組 | REAL（本步的模組邊界，`buyer.proto:53-137`） | 草稿 Q1【量】 |

### Q2　「只看收藏的商品／追蹤店家的商品」放哪

**建議（與草稿起草版不同）**：從 `ProductFilter` 刪掉 `followed_product_only`、`followed_seller_only`，`CatalogPublicService.ListProducts` 宣告**不依觀看者**（可共享快取）。
- 收藏商品：已有 `BuyerAppService.ListFavorites`，直接用。
- 追蹤店家的商品流：新增 `BuyerAppService.ListFollowedStoreProducts`（buyer 模組擁有追蹤關係，經 catalog 模組的 Go API 以 `store_ids` 取商品，keyset 分頁）。

- 業界基準：
  - AIP-132 的 `filter` 是對集合本身欄位的篩選；「與我的關係」在 Google 的設計是另一個以使用者為父資源的集合（`users/{u}/favorites`）【文件】https://google.aip.dev/132 、https://google.aip.dev/122
  - Etsy Open API v3 的收藏是使用者底下的資源，不是商品列表的參數【文件★】https://developers.etsy.com/documentation/reference
  - 社群／電商的「追蹤中」動態（Instagram、蝦皮「關注中」）是個人化 feed，與公開瀏覽分開，前者不進共享快取【推】。
- 現有機制：舊 `/public/products` 一支端點服務所有列表畫面，所以把個人化參數塞進去【量：草稿 Q2，`deriveListView.ts:203`】。
- 選項：
  - (a) 留在公開 `ListProducts`（草稿起草版）。失敗條件：最熱的公開列表 RPC 必須宣告 viewer-dependent，所有登入買家的分類頁、市集頁都變成 `private, no-store`（規範 §九），只為了一個分頁。
  - (b) client 先取追蹤店家 ID，再以 `filter.store_ids` 呼叫公開列表。失敗條件：追蹤數超過 `store_ids` 上限（草稿提 50）就失效，且 client 要自己處理合併分頁。
  - (c) buyer 的 RPC（建議）。失敗條件：追蹤店家多到 `store_id IN (...)` 的 keyset 查詢變慢——那時改成 fan-out-on-write 的 feed 表，仍在同一支 RPC 背後，契約不變。
- 推翻條件：量到 CDN 對公開列表的命中率不重要（例如流量全在 SSR、且 SSR 本身不帶憑證）。
- UI：不改畫面，只改 website「追蹤中」分頁呼叫的 RPC（B4-6 接線）。
- 可逆性：日後若要加回篩選是附加欄位；反方向（先放進去再拿掉）是破壞性變更。先拿掉較便宜。
- 限制帳：

| 限制 | 類別 | 來源 |
|---|---|---|
| 盤點寫「保留已收藏／已追蹤篩選」 | SELF-IMPOSED（盤點是舊行為紀錄，不是規格） | `interface-inventory.md` §1.1 |
| 依觀看者的回應不得進共享快取 | REAL（安全性；不能違反） | 規範 §九 |

### Q3　更新語意：整體取代 vs `update_mask`；etag 放哪

**建議**：保留整體取代，但名實相符：
1. 方法維持 `UpdateProduct`，說明寫明「整體取代，等同 AIP-134 的 `update_mask = "*"`」，不加 mask 欄位。
2. etag 放在**請求**（`UpdateProductRequest.etag`），不放進 `ProductInput`；同時修訂規範 §七：「請求本體不是資源（寫入型別與讀取型別不同）時，etag 在請求」。
3. 讀寫分型（`ProductInput` vs `Product`）維持：寫入需要 staged image token、讀取回 asset，型別本來就不同。

- 業界基準：
  - AIP-134：Update 帶資源與 `update_mask`，mask 省略時語意由 API 定義，`*` 是全量取代【文件】https://google.aip.dev/134
  - Shopify 同時有局部的 `productUpdate` 與宣告式整體取代的 `productSet`（給「同步整份商品」的呼叫者，缺的 variant 會被刪）【文件】https://shopify.dev/docs/api/admin-graphql/latest/mutations/productSet
  - Shopify、Stripe 的寫入都用與輸出不同的 input 型別（GraphQL `ProductInput`、Stripe 的 params）；AIP 讓請求帶資源本身，是因為它的資源沒有 write-only 的 token 欄位【文件】https://docs.stripe.com/api/products/update
  - AIP-154 允許 etag 出現在請求（Delete、自訂方法）【文件】https://google.aip.dev/154
- 現有機制：B4-2b 已定 `ProductInput` 整體取代；模組 `UpdateProduct` 也是【量：草稿 Q3】。表單每次送完整內容。
- 選項：
  - (a) 整體取代＋請求層 etag（建議）。失敗條件：出現只改子集合的第二個寫入者（批次改價、代行只改標題、匯入工具）——那時加 `update_mask` 是附加欄位，沒帶 mask 仍是全量。
  - (b) 現在就加 mask。失敗條件：沒有消費者，`title`、`category_id`、`skus` 又是必填，mask 的大部分組合是無效請求，多一層要測的語意。
- 推翻條件：B4-5d 代行的修改是 `PATCH`（局部）——要在 B4-5d 定形狀時再看；它是另一支 service，不影響本支。
- 可逆性：加 mask 是附加、低成本。
- 限制帳：etag 在資源內（規範 §七）——SELF-IMPOSED，規範本身沒有考慮讀寫分型，修規範一行即可。

### Q4　冪等（`request_id`）的範圍與必填

**建議**：同草稿：`CreateProduct`、`AdjustStock` 帶 `request_id`，**必填**；更新、上下架、刪除不帶（etag 或自然冪等已涵蓋）。補三點草稿沒寫的：
1. 鍵的範圍是 `(store_id, method)`，存放：Create 在 `products (store_id, request_id)` 唯一；AdjustStock 在 `stock_movements` 的既有 key。保存期限：永久（跟著資料列，不需要 Stripe 那種 24 小時清理）。
2. 同鍵不同內容回 `INVALID_ARGUMENT`（Stripe 是 400 `idempotency_error`，同義）；比對內容用請求的雜湊存在列上。
3. 同鍵併發（兩個請求同時在途）：唯一索引讓後到者等待後回第一筆的結果，不回錯（Stripe 回 409，我們有交易可以做得更好）。

- 業界基準：
  - Stripe：所有 POST 接受 `Idempotency-Key`（選填），保存 24 小時，參數不同回錯【文件】https://docs.stripe.com/api/idempotent_requests
  - AIP-155：`request_id` 欄位、UUID，建議用在非冪等的建立【文件】https://google.aip.dev/155
  - Shopify 庫存調整 mutation（`inventoryAdjustQuantities`）要求冪等鍵（`@idempotent` directive），理由正是「變動量重送會重複計算」【文件★】https://shopify.dev/docs/api/admin-graphql/latest/mutations/inventoryAdjustQuantities
- 為何必填而非 Stripe 的選填：Stripe 是公開 API、無法強迫所有整合方；我們的四個 client 都是自己的，必填沒有成本，選填則留下 AdjustStock 重送重複加減的洞【推】。
- 選項：(a) 兩者必填（建議），失敗條件：出現無法產生 UUID 的呼叫者（不存在）；(b) 選填（social 前例），失敗條件：client 漏帶時 AdjustStock 重複生效；(c) 所有寫入都帶，失敗條件：與 etag 重複保護，多一份存放。
- 推翻條件：無實際可能【推】。
- 可逆性：必填改選填是放寬，低成本；反之是破壞性。
- 限制帳：social 前例選填——SELF-IMPOSED（一致性）。

### Q5　狀態切換的樂觀鎖；庫存調整是否改 etag

**建議**：
1. `SetProductStatus` 的 etag **必填**（同草稿），`ProductSummary` 帶 etag 讓列表能直接上下架。
2. **etag 的範圍明訂為「賣家可編輯的內容」**：庫存（`stock_qty`）、銷量、收藏數、評分、`first_publish_time` 都不進 etag，`AdjustStock` 與計數更新不改 etag。在 `Product.etag` 的註解寫明這個範圍。
3. 治本做法是把庫存視為獨立資源（Shopify 的 `InventoryLevel` 模型）：契約上 `AdjustStock` 已經是以 `sku_id` 定址的獨立方法、`UpdateProduct` 不改既有 SKU 庫存，所以只需寫明，不需拆訊息。

- 業界基準：
  - Shopify 的商品與庫存是不同資源（`Product`／`ProductVariant` vs `InventoryItem`／`InventoryLevel`），改庫存不影響商品的編輯；庫存的「設成絕對值」用 `compareQuantity` 做 CAS【文件★】https://shopify.dev/docs/api/admin-graphql/latest/mutations/inventorySetQuantities
  - AIP-154：etag 代表資源目前的狀態；計算欄位可以排除在外由 API 自定【文件】https://google.aip.dev/154
  - Shopify、Etsy 的商品狀態切換都**不**要求版本【文件】https://shopify.dev/docs/api/admin-graphql/latest/mutations/productUpdate ——這點我們比業界嚴，理由見下。
- 為何比 Shopify 嚴仍建議：規範 §七「決定依據的內容在狀態不變時仍可能被改的，etag 必填」；上架是「把我看到的內容公開」，多人（店員、代行 B4-5d）同時編輯時，用過期畫面上架會公開別人剛改的內容。成本只是列表多一欄【推】。
- 選項與失敗條件：
  - etag 必填＋範圍排除庫存（建議）。失敗條件：client 把 etag 當成「資料是否最新」用來判斷庫存是否變了（不該，HTTP `ETag` 才是）。
  - etag 必填但庫存也改 etag。失敗條件：賣家編輯商品時每次有訂單扣庫存就 `VERSION_CONFLICT`，熱賣商品幾乎無法編輯。
  - 不要求 etag（Shopify 做法）。失敗條件：多人編輯時誤上架別人的未審內容。
- 推翻條件：確定只有單人管理店家且不做代行——但 B4-5d 已決做代行（D9），所以不成立。
- 可逆性：etag 必填改選填是放寬，低成本。
- 限制帳：規範 §七——SELF-IMPOSED 但與業界差異有理由，維持。

### Q6　低庫存：定義、門檻歸屬、單位、計數

**建議（與草稿不同）**：
1. **門檻歸屬：每商品選填 `low_stock_threshold`，沒設用平台預設 5**（伺服器常數，回應帶出）。放回舊表單已有的「低庫存提醒門檻」欄位【量：`ProductFormSections.tsx:214`】，所以**不是新增 UI**，是讓既有欄位有效。
2. **判斷單位是 SKU**：商品「偏低」＝至少一個沒刪除的 SKU 滿足 `0 < stock_qty ≤ 門檻`；**售完（`stock_qty = 0`）另算**。
3. **只算 ACTIVE**：下架、草稿不在賣，提醒是噪音。
4. **計數 RPC 改通用形狀**：`CountMyProducts(filter)`，`filter` 與 `ListMyProducts` 共用（`status`、`stock_status = LOW | OUT_OF_STOCK`、`query`）；導覽徽章送 `status=ACTIVE, stock_status=LOW`。比 `CountLowStockProducts` 通用、不需要為每種徽章開一支 RPC。

- 業界基準：
  - WooCommerce：商店層級的低庫存門檻（預設 2）＋每商品／每變體可覆寫，另有獨立的「缺貨門檻」，低庫存與缺貨分開通知【文件】https://woocommerce.com/document/configuring-woocommerce-settings/products/#inventory
  - BigCommerce：每商品、每變體的 `inventory_warning_level`【文件】https://developer.bigcommerce.com/docs/rest-catalog/products
  - Shopify 核心沒有每商品門檻，靠報表與 app；它的計數查詢是 `productsCount(query:)` 這種吃篩選的通用形狀【文件★】https://shopify.dev/docs/api/admin-graphql/latest/queries/productsCount
  - 共同點：門檻可由商家設定、以變體（SKU）判斷、缺貨與低庫存分開。
- 現有機制：website 寫死 5 與 10 兩個數字、client 自己數【量：草稿 Q6 所列 `SellerCenterShell.tsx:128-135`、`InventoryPage.tsx` `SAFE_STOCK_DEFAULT`】；新 schema 在 B4-2 沒帶門檻欄。
- 選項：
  - (a) 伺服器常數 5、任何狀態（草稿）。失敗條件：賣家的商品週轉差異大（一天賣 50 件 vs 一個月 1 件），單一常數對兩者都錯；既有表單欄位變成無作用的假欄位，要刪 UI。
  - (b) 請求帶 `stock_at_most`。失敗條件：門檻散落在 client，三端各寫各的（正是現況的問題）。
  - (c) 每商品選填門檻＋平台預設（建議）。失敗條件：賣家要的是整店預設而不是每商品——那時加店家層預設（seller 模組設定），判斷順序商品 > 店家 > 平台，契約不變。
  - (d) 每 SKU 門檻（BigCommerce）。失敗條件：表單現在是每商品一個欄位，多 SKU 的輸入 UI 要新做。
- 推翻條件：Will 決定刪掉表單上的門檻欄位（那 (a) 成立）。
- UI：(c) 讓既有欄位生效（需確認欄位語意＝每商品、套用到每個 SKU）；庫存頁的 10 改成讀伺服器的判斷（行為變更，**需 Will 同意**）；徽章只算 ACTIVE（語意變更，**需同意**）。
- 可逆性：加欄位是 migration＋附加 proto 欄位；之後刪是破壞性但系統未上線，成本低。
- 限制帳：

| 限制 | 類別 | 來源 |
|---|---|---|
| 新 schema 沒有門檻欄 | SELF-IMPOSED（B4-2 的決定，在我們邊界內，加一欄即可） | 草稿 Q6 |
| 「不新增資料」 | SELF-IMPOSED（工作量） | 草稿 Q6 |

### Q7　商品頁的組合：店家卡、分類路徑、觀看者狀態

**建議**：同草稿：`GetProduct` 不依觀看者（可共享快取）；收藏、補貨訂閱狀態由 client 另呼叫 `BuyerAppService.BatchCheckFavorites`／`ListRestockSubscriptions`；店家卡由 httpapi 批次組進回應；分類路徑由 `ListCategories`（62 節點、可快取）推得，不另開方法。

- 業界基準：
  - 大型電商的商品頁是「可快取的公開內容＋個人化片段另取」（edge 組合、client hydration）；Shopify Storefront 的 `Product` 沒有觀看者狀態，願望清單是 app 另取【文件】https://shopify.dev/docs/api/storefront/latest/objects/Product 、【推】。
  - Shopify 的分類（Standard Product Taxonomy）在商品上回分類物件，祖先另由分類查詢取得【文件★】https://shopify.dev/docs/api/admin-graphql/latest/objects/TaxonomyCategory
- 現有機制：舊商品詳情回 `isFavorited`、`isRestockSubscribed`，因為舊 gateway 沒有共享快取的概念【量：草稿 Q7 `product.go:786-788`】。
- 選項：(a) 分開（建議），失敗條件：商品頁的首屏一定要伺服器端渲染收藏狀態且多一次往返不可接受——量到再說；(b) 帶 `viewer_*`，失敗條件：流量最大的公開讀取失去共享快取；(c) 另開 `GetProductPath`，失敗條件：兩層、62 節點的樹，多一支 RPC 沒有省到任何東西。
- 推翻條件：分類樹成長到數千節點（client 抓整棵不划算）→ 在 `Product` 帶 `category_path`（附加欄位）。
- UI：無畫面改動；client 多一次 buyer 呼叫（B4-6）。
- 可逆性：之後加 `category_path` 是附加。

### Q8　後台列表分頁

**建議**：同草稿：keyset＋`total_size`，後台商品頁從頁碼改成「上一頁／下一頁＋總數」；上一頁由 client 保留 token 堆疊。後台首頁、分析頁的商品總數用 `ListProducts` 的 `total_size`（`page_size` 取小值）——但標註：分析類數字長期應屬報表 API，不在 catalog 列表。

- 業界基準：
  - AIP-158：游標分頁，`total_size` 選填【文件】https://google.aip.dev/158
  - Stripe API 與 Dashboard 都是游標（`starting_after`／`ending_before`）＋前後頁，不提供跳頁【文件】https://docs.stripe.com/api/pagination
  - Shopify Admin（API 與後台介面）是游標前後頁，不顯示頁碼【文件】https://shopify.dev/docs/api/usage/pagination-graphql
- 現有機制：後台頁碼來自舊 REST 的 offset 分頁【量：草稿 Q8 `products/page.tsx:72-104`】。
- 選項：(a) keyset＋total（建議），失敗條件：營運真的需要跳到第 N 頁（沒有證據；他們用關鍵字與店家篩選找商品）；(b) offset 頁碼，失敗條件：翻頁途中新增商品會重複或漏掉、深頁慢，且要修規範 §六開例外；(c) 只給總數，商品頁 keyset（等同 a）。
- 推翻條件：營運回報跳頁需求。
- UI：**頁碼改上一頁／下一頁，需 Will 同意**。
- 可逆性：加 offset 是附加；但規範例外一旦開了會擴散。
- 限制帳：規範 §六——SELF-IMPOSED，但與業界一致，維持。

### Q9　列表與搜尋：一支還是兩支

**建議**：同草稿，兩支：`ListProducts`（不帶關鍵字，單一排序鍵）與 `SearchProducts`（必填 `query`，預設相關度），共用 `ProductFilter`；`SuggestProductNames` 是預測搜尋。

- 業界基準：
  - Shopify Storefront 分開 `products(query:, sortKey:)`（篩選式列表）、`search(query:)`（相關度、有 `RELEVANCE` 排序）與 `predictiveSearch`【文件】https://shopify.dev/docs/api/storefront/latest/queries/search 、https://shopify.dev/docs/api/storefront/latest/queries/predictiveSearch
  - Google Retail API 的 `Search` 與 Product 的 `List` 是不同方法【文件】https://cloud.google.com/retail/docs/reference/rpc/google.cloud.retail.v2#searchservice
  - AIP-132 的 List 不排相關度；相關度搜尋是自訂方法【文件】https://google.aip.dev/132 、https://google.aip.dev/136
- 理由：相關度排序的游標組成（分數＋ID）與單一鍵不同；計畫 D15 留了「量測觸發換引擎」的門，獨立的 `SearchProducts` 讓換引擎時只換一支的背後。
- 選項：(a) 兩支（建議），失敗條件：client 必須依有沒有關鍵字切換 RPC（website 市集頁一個 if）；(b) 一支、`query` 選填，失敗條件：`sort` 合法值依 `query` 有無而變，換搜尋引擎時要拆。
- 推翻條件：無。可逆性：合併是破壞性，但未上線成本低。

### Q10　下架、草稿、不可曝光店家的公開商品頁（草稿沒列成決策）

**建議（與草稿不同）**：
- `DRAFT`、已刪除、店家不可曝光（停權、關店）、（B4-5 起）被限制措施下架：`NOT_FOUND`。
- `ARCHIVED`（賣家自己下架、可還原）：**讀得到**，回 `status = ARCHIVED`、`available_for_sale = false`；不進任何列表與搜尋。
- 新增 output 欄位：`Product.available_for_sale`、`Sku.available_for_sale`（Shopify Storefront 同名語意：上架中且有庫存），client 以它決定能否加入購物車，不自己由庫存與狀態推導。
- 訂單頁的商品資訊以**結帳快照**（計畫 D20）呈現，不依賴商品頁可讀；商品頁可讀只服務「收藏清單、外部連結、分享連結」。

- 業界基準：
  - Shopify 線上商店：商品不在銷售管道（draft／archived）時商品頁 404；Storefront API 對未發布商品回 null【文件★】https://shopify.dev/docs/api/storefront/latest/queries/product 。Shopify 是單一商家，沒有跨店收藏。
  - Amazon：下架或無貨的 ASIN 頁保留，顯示「Currently unavailable」【推，公開可觀察行為，沒有文件連結】。
  - Etsy：不再上架的 listing 頁顯示「此商品已無法購買」並推薦類似商品【推，同上】。eBay 已結束的 listing 保留一段時間可讀【推】。
  - 共同點：**市集（有買家收藏、分享連結）保留頁面、標示不可購買；單店系統 404**。hawkr 是市集。
  - 治理下架（違規）各家都是移除或改成通用的「已不存在」頁，不顯示原內容【推】。
- 現有機制：Java 對下架商品回內容加 `purchasable=false`【量：`scenarios_test.go:110-113`】；草稿依「公開只讀 ACTIVE」一律 not_found，沒說明理由。
- 選項：
  - (a) 全部非 ACTIVE 都 404（草稿）。失敗條件：買家收藏的商品一被賣家暫時下架，收藏清單點進去就是 404，重新上架後又出現；分享連結與搜尋引擎的頁面在暫時下架期間失效。
  - (b) ARCHIVED 可讀、其餘 404（建議）。失敗條件：賣家下架是為了隱藏內容（例如內容有誤）——那是「回草稿」（D22 允許 ACTIVE→DRAFT）的用途，兩個狀態語意因此分開：DRAFT＝藏起來、ARCHIVED＝停售但留著頁面。這個語意要寫進 `ProductStatus` 註解與賣家端文案。
  - (c) 全部可讀＋不可購買。失敗條件：草稿從未公開過，被枚舉讀到等於洩漏；停權店家的商品仍可見違反 D7 治理。
- 推翻條件：Will 認為賣家「下架」的直覺就是「別人看不到」——那選 (a)，並在賣家端把「下架」改叫「停售並隱藏」。
- UI：買家商品頁需要「已下架」狀態的呈現（舊 UI 有 `purchasable=false` 的分支，沿用即可【推，未逐畫面查】）；賣家端下架按鈕的說明文字要區分「下架（頁面保留）」與「改回草稿（隱藏）」——**文案改動需同意**。
- 快取：(b) 不依觀看者，仍可共享快取。
- 可逆性：高。只差 handler 的一個條件與一個 output 欄位。
- 限制帳：「公開只讀 ACTIVE」——SELF-IMPOSED（草稿自訂）。D7 治理下架必須不可見——REAL（法遵）。

### Q11　賣家清單的關鍵字篩選（草稿刪掉了）

**建議（與草稿不同）**：`ListMyProducts` 加 `query`（比對標題與 SKU 選項，用同一個 `searchable_text`），並用 `MyProductFilter{status, stock_status, query}` 與 `CountMyProducts` 共用。

- 業界基準：Shopify Admin `products(query: "title:…")`、Shopee／蝦皮賣家中心商品列表都有伺服器端關鍵字搜尋【文件】https://shopify.dev/docs/api/admin-graphql/latest/queries/products
- 現有機制與錯誤前提：舊賣家 client 沒送 `keyword`，因為它抓 100 筆在 client 過濾【量：`InventoryPage.tsx:329-337`】；新契約有分頁，client 過濾只看到第一頁。所以「沒有 client 送」不代表沒有需求——畫面有搜尋框。
- 選項：(a) 伺服器 `query`（建議），失敗條件：無；(b) 不加，client 抓全部再過濾，失敗條件：商品超過 `page_size` 上限 100 就漏。
- UI：不改畫面，搜尋改由伺服器做（B4-6）。可逆性：附加欄位。

### Q12　（草稿默默做的）公開與賣家共用同一個 `Product` 訊息

**建議（與草稿不同）**：拆成 `Product`／`ProductSummary`（公開）與 `SellerProduct`／`SellerProductSummary`（賣家）。草稿的 `Product` 有 7 個「只有賣家讀才設」或「只有公開讀才設」的欄位（`store`、`status`、`etag`、`asset_id`、`create_time`、`update_time`、`total_stock`）【量：草稿第 447–500 行的註解】。

- 業界基準：Shopify Storefront `Product` 與 Admin `Product` 是兩個型別；規範 §五已經為後台這樣做【文件】https://shopify.dev/docs/api/storefront/latest/objects/Product 、https://shopify.dev/docs/api/admin-graphql/latest/objects/Product ；【量：規範 §五】
- 選項：(a) 共用（草稿），失敗條件：client 從型別看不出哪些欄位一定有，公開回應若誤設 `asset_id`、`etag`（內部 ID）也不會被型別或測試擋；(b) 拆開（建議），失敗條件：兩邊共通欄位改動要改兩處——值型別（`Sku`、`ProductImage`、`OptionAxis`）照樣共用，重複的只有最上層約 12 個欄位。
- 可逆性：未上線，任何時候都可拆；越晚拆 client 改越多。UI：無。

### Q13　（草稿默默做的）庫存調整只有變動量

**建議**：維持 `AdjustStock`（delta，計畫 D23 已決）；記錄觸發：賣家端出現「盤點：輸入實際數量」的流程時，加 `SetStock(sku_id, quantity, expected_quantity)`（Shopify `inventorySetQuantities` 的 `compareQuantity` CAS 模式）。現在的庫存頁若是輸入絕對值，B4-6 接線時要由 client 換算 delta，可能在換算與送出之間被訂單扣減而算錯——**B4-6 前要查庫存頁的輸入是絕對值還是增減**【推，本 session 沒查】。

---

## 2. 對草稿 proto 的修改清單

1. `ProductFilter` 刪 `followed_product_only`、`followed_seller_only`；`ListProducts` 在 `publicViewerDependent` 宣告 `false`（7 個公開方法全部 `false`）。（Q2）
2. buyer 的 `BuyerAppService` 新增 `ListFollowedStoreProducts`（buyer proto；B4-3 範圍要加上，或明訂歸 buyer 模組的下一步）。（Q2）
3. `UpdateProduct` 說明改為「整體取代（等同 `update_mask = "*"`）」；etag 留在請求；規範 §七加一行「寫入型別與資源不同時 etag 在請求」。（Q3）
4. `CreateProduct`、`AdjustStock` 的 `request_id` 說明補：範圍 `(store_id, 方法)`、永久保存、併發同鍵回第一筆結果。（Q4）
5. `etag` 欄位註解寫明範圍：賣家可編輯的內容；庫存、計數、`first_publish_time` 不影響。（Q5）
6. 刪 `CountLowStockProducts`；新增 `CountMyProducts(MyProductFilter) → {count, low_stock_threshold_default}`；`ListMyProducts` 改收 `MyProductFilter{status, stock_status, query}`；新 enum `StockStatus{UNSPECIFIED, LOW, OUT_OF_STOCK}`；`ProductInput` 加選填 `low_stock_threshold`；`SellerProduct` 回 `low_stock_threshold`（選填）。（Q6、Q11）
7. `GetProduct` 說明改為：ARCHIVED 讀得到；`Product` 加 `ProductStatus status`（公開只會是 ACTIVE 或 ARCHIVED）與 `bool available_for_sale`，`Sku` 加 `bool available_for_sale`；`ProductStatus` 註解寫明 DRAFT＝隱藏、ARCHIVED＝停售但頁面保留。（Q10）
8. 拆訊息：公開 `Product`／`ProductSummary` 不含 `etag`、`asset_id`、`create_time`、`update_time`、`total_stock`；新增 `SellerProduct`／`SellerProductSummary`（含上述與 `status`、`etag`、`low_stock_threshold`、`total_stock`），賣家方法改回這兩個型別。（Q12）
9. 後台 `ListProducts` 維持 keyset＋`total_size`（不變）。

RPC 數：公開 7、賣家 8（`CountLowStockProducts` 換成 `CountMyProducts`）、後台 2，buyer 另加 1。

## 3. 對計畫 B4-3 的修改清單

1. 範圍：「低庫存數 API」改為「`CountMyProducts` 與每商品低庫存門檻（`catalog.products.low_stock_threshold` 選填欄位，migration）」。
2. 範圍：加「`BuyerAppService.ListFollowedStoreProducts`（buyer 模組，經 catalog Go API）」，或在計畫中明列歸屬步驟。
3. 範圍：`ListMyProducts` 的 `query` 與 `stock_status` 篩選。
4. 特性測試（舊行為證據，依本建議修訂）：
   - 「listed product is public and unlisting hides it from lists」：保留「下架後公開頁讀得到、不可購買」，斷言改成 `available_for_sale == false` 與 `status == ARCHIVED`（Q10 選 (b) 時）；選 (a) 則改成 `ErrNotFound` 並在情境加 `skipOnLegacy` 理由。
   - 新增：賣家以關鍵字找到自己第 101 筆以後的商品（Q11）；低庫存計數只算 ACTIVE、售完分開（Q6）；`AdjustStock` 後 `UpdateProduct` 用舊 etag 仍成功（Q5）。
5. mutation 清單（`mutation/b4-3.json`）加：`AdjustStock` 改成遞增 `version` → 「調整庫存後以舊 etag 修改成功」測試紅；公開 `GetProduct` 放行 DRAFT → 草稿不可見測試紅；公開 `ListProducts` 宣告改回 viewer-dependent 不影響正確性，不需 mutation。
6. 驗收加：規範 §七的 etag 位置修訂一併提交。
7. B4-6 前置查核：庫存頁輸入是絕對值還是增減（Q13）。

## 4. 摘要表

| 決策 | 建議 | 與草稿 | UI 改動（需同意） | 可逆 |
|---|---|---|---|---|
| Q1 service 切分 | 三支，不開 catalog 買家 service | 同 | 無 | 是，低成本 |
| Q2 追蹤／收藏篩選 | 移出公開列表；收藏用 `ListFavorites`，追蹤店家商品流放 `BuyerAppService` | **不同**（草稿起草為留在公開列表） | 無（只改呼叫） | 是 |
| Q3 更新語意 | 整體取代＝mask `*`；etag 在請求；修規範一行 | 同（補規範修訂） | 無 | 是，加 mask 為附加 |
| Q4 冪等 | Create、AdjustStock 必填 `request_id`；補範圍、保存、併發語意 | 同（補細節） | 無 | 是 |
| Q5 狀態樂觀鎖 | etag 必填；etag 範圍排除庫存與計數 | 同（明訂範圍） | 無 | 是 |
| Q6 低庫存 | 每商品選填門檻＋平台預設 5；以 SKU 判斷；售完分開；只算 ACTIVE；通用 `CountMyProducts` | **不同**（草稿：常數、任何狀態、專用 RPC） | 是：庫存頁 10→伺服器判斷、徽章只算 ACTIVE；表單門檻欄位恢復作用 | 是，migration 小 |
| Q7 商品頁組合 | 不帶觀看者狀態；分類路徑由樹推得 | 同 | 無 | 是 |
| Q8 後台分頁 | keyset＋`total_size`，上一頁／下一頁 | 同 | 是：頁碼→前後頁 | 是 |
| Q9 列表 vs 搜尋 | 兩支，共用篩選 | 同 | 無 | 是 |
| Q10 下架商品頁 | ARCHIVED 可讀＋`available_for_sale=false`；DRAFT／刪除／不可曝光 404 | **不同**（草稿全部 404，且未列為決策） | 是：賣家端「下架」與「回草稿」的說明文案 | 是，高 |
| Q11 賣家關鍵字 | `ListMyProducts` 加伺服器 `query` | **不同**（草稿刪除） | 無 | 是 |
| Q12 訊息分型 | 公開與賣家拆成兩組訊息 | **不同** | 無 | 是 |
| Q13 庫存設定 | 維持 delta；盤點流程出現時加 CAS 的 `SetStock` | 新增觸發 | 無 | 是 |

## 5. 不確定處

- 標★的 Shopify 條目（`inventoryAdjustQuantities` 冪等鍵要求、`productsCount`、`TaxonomyCategory`、Storefront 對未發布商品回 null）依既有知識，本 session 沒重新開網頁；Amazon、Etsy、eBay 的不可購買頁行為是公開可觀察行為，沒有文件連結（標【推】）。
- Q10 的買家端「已下架」呈現是否已存在：只看到舊 `purchasable` 欄位，沒逐畫面查。
- Q13 庫存頁輸入形式沒查。
- 本簡報依 methodology-brief 的規則應由新的 subagent 審一次前提問題；這份是起草者自評，尚未經獨立審查。
