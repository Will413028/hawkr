# B4-5 商品限制措施：決策簡報 v2（定稿，Will 2026-11-28 全部拍板）

證據標記：【量】= 本次讀檔、跑 rg 或讀官方文件確認（附指令或 URL）；【文件】= 計畫、ADR、簡報的文字；【文件，未於本次查證】= 只引用法規或文件名稱，沒有讀原文；【推】= 由程式或文件推論，沒有實際跑過。
路徑相對 repo 根目錄。v1 已查證、本版沿用的事實不再重查：`CONTENT_TAKEDOWN`＋`content_kind PRODUCT`、只收申覆、公開頁 NotFound、食安法草案 43-1、`product_ids TEXT[]→UUID[]`。

## 結論先講

1. **可見性只有一個定義**：catalog 在 `db/objects.sql` 新增 SQL 函式 `catalog.buyer_visible_products(at)`（「買家看得到的商品」＝賣家設 `ACTIVE` ∧ 店家可曝光 ∧ 沒有生效中的商品下架措施），所有買家面讀取改走它；`internal/arch` 加一條檢查，catalog 的 query 直接讀 `catalog.products` 卻不在白名單（賣家、後台、內部用）就讓 CI 失敗。盤點時找到兩個現有漏洞：**購物車／結帳快照與扣庫存都不看店家曝光**【量】，停權店家的商品今天仍可結帳。
2. **法律保全（legal hold，「為了證據暫停刪除」）放在 media，只作用在 SECURE（加密、不公開）證據副本上**（Will 2026-11-28 選 (b)）：措施建立或來函凍結時，擁有者 catalog 用新的 media `CopyToSecure` 把商品圖複製成 SECURE 副本，保全只登記在副本上；**公開原圖照一般刪除流程處理**（公開網址失效＝停止公開存取）。新表 `media.asset_holds(asset_id, source, until)`；**每一條會把資產或暫存上傳推進 `DELETING`、或實際刪物件的路徑都要擋住被保全者**：SQL 認領處跳過，物件刪除前再檢查（§2 逐路徑盤點）。不用 S3 Object Lock 當唯一來源（要 bucket 開版本控制、只認單一版本、沒有到期日【量】）。
3. **證據快照由擁有內容的模組寫**：catalog 收到 `MeasureCreated`（內容種類 PRODUCT）或新事件 `AuthorityOrderTargetsRecorded` 時，在同一個 transaction 寫一份不可改的快照（JSON 內容＋圖片的 SECURE 副本 asset 參照），並對副本登記保全。措施列不存快照 ID，以 `(來源種類, 來源 ID, 商品 ID)` 當鍵：由依賴規則決定（`internal/arch/modules.go:19,22`，catalog→seller【量】，seller 寫不到 catalog 回傳的 ID），不列為問題；這偏離 Will 原話「措施存快照 ID」，見 §3。
4. **admin**：`admin/products` 拿掉直接切狀態（`handleToggleStatus`，打舊 REST `PUT /api/v1/admin/products/{id}/status`【量】）；強制下架改成重用 `ContentTakedownDialog` 的措施建立對話框，走 Connect `CreateMeasure`。欄位與文案見 §4（Will 2026-11-28 已同意；商品專屬違規類別清單在 B4-5e 開工前再與 Will 確認）。
5. 步驟拆成 B4-5a（可見性＋PRODUCT 種類與 `OWNER_HELD`）→ B4-5b（media 保全與 `CopyToSecure`）→ B4-5c（快照＋上架檢查）→ B4-5e（來函凍結＋admin UI），見 §5。

---

## 0. 前提問題：要改的程式是否已排定重寫、搬移或刪除？

| 要改的地方 | 是否排定 | 重做量 | 分類 |
|---|---|---|---|
| catalog queries、`product`、`stock`、`restock` | 否（B4-1～4 剛寫好） | 0 | — |
| media `reclaim` | 否（B1 完成的模組） | 0 | — |
| seller 措施與來函（`schema.sql`、`authority_orders.go`、`events.go`） | 否（A9 完成） | 0 | — |
| `internal/legacy/catalogserver/server.go`（`SkuCartInfo`、`CheckoutSnapshot` 的呼叫端，legacy 訂單路徑） | 是，B4-6 刪 legacy | B4-5 **不碰**它：可見性改在 catalog 的 `SkuSnapshots`／`Deduct` 內部，呼叫端簽章不變，B4-6 丟掉的是 0 行本步程式【推】 | SELF-IMPOSED（自己的計畫） |
| `admin/src/app/admin/products/page.tsx`（340 行）、`merchants/[id]/ProductForceUnpublishDialog.tsx` | 是，B4-6 換生成 client | 若在舊 REST 上改 UI，B4-6 會整段換掉 API 呼叫【推】；本版直接用 Connect 寫新對話框、刪掉舊切換，**沒有拋棄式工作** | SELF-IMPOSED |

結論：沒有任何 REAL 的排程推離基準；全部直接做終局形狀。

---

## 1. 可見性的單一定義（B4-5a）

### 現況盤點【量】

指令：
```
rg -n "pub_exposable_stores" backend/internal/modules/catalog/db/queries      # 8 處（products 5、exports 2、restock 1）
rg -c "pub_content_takedowns" backend/internal/modules/catalog                # 0：catalog 目前完全不看措施
rg -n "catalog\.(products|product_)" backend/internal --glob '*.sql' --glob '!internal/modules/catalog/**'   # 0：別的模組不直接讀 catalog 的表
```

買家面 query（各自重複寫一次謂詞）：

| query（檔） | 現在的謂詞 | Go 入口 → 呼叫端 |
|---|---|---|
| `GetPublicProduct`（products.sql:103） | `status IN (ACTIVE, ARCHIVED)` ∧ 曝光 | `PublicProduct` → `httpapi/catalog/public.go`、`content/internal/banner/banner.go` |
| `ListPublicProducts`（:196） | ACTIVE ∧ 曝光 | `ListPublicProducts`、`ListPublicProductsOfStores` → `httpapi/catalog/public.go`、`cmd/hawkr/procurement.go`、`httpapi/buyer/followed_products.go` |
| `SuggestProductNames`（:227）、`SuggestHashtags`（:240）、`TrendingHashtags`（:251） | ACTIVE ∧ 曝光 | `httpapi/catalog/public.go`、`httpapi/social/public.go`、`social/internal/post/read.go` |
| `GetProductCards`（exports.sql:2） | `include_inactive` 或（ACTIVE ∧ 曝光） | `ProductCards` → `cmd/hawkr/buyer_ports.go`（收藏、瀏覽紀錄）、`cmd/hawkr/social.go`（貼文商品卡） |
| `ListStoresWithPublicProducts`（:18） | ACTIVE ∧ 曝光 | `discovery/internal/ranking/recompute.go` |
| `GetSkuSnapshots`（:24） | **只看 status 與刪除，不看曝光** | `CheckoutSnapshot`、`SkuCartInfo` → `cmd/hawkr/buyer_ports.go`、`legacy/catalogserver` |
| `GetSkuRestockState`（restock.sql:2） | ACTIVE ∧ 曝光 ∧ 有庫存 | `SkuPurchasable`、補貨 worker |
| `DeductSkuStock`（stock.sql:30） | **只看 SKU 未刪與庫存，不看商品狀態與曝光** | `DeductStock`（訂單扣庫存） |

非買家面（賣家自己的、後台、內部計數，不走可見性函式，列進白名單）：`InsertProduct`、`GetProductByRequest`、`LockOwnProduct`、`OwnProductDeleted`、`UpdateProductContent`、`SetProductStatus`、`SoftDeleteProduct`、`RecomputePriceRange`、`GetOwnProduct`、`GetAdminProduct`、`ListSellerProducts`、`CountSellerProducts`、`ListAdminProducts`、`CountAdminProducts`、`LockProduct`、`SetProductRating`、`AddProductSales`、`GetProductStoreIDs`、`IncrementProductFavorites`、`DecrementProductFavorites`、`AdjustOwnSkuStock`、`SetOwnSkuStock`、`GetOwnLiveSku`、`SkuInStore`、`GetOwnSkuState`，以及 restock 佇列的 claim／finish 類。

**新發現的漏洞**【量】：`SkuSnapshots` 的 `Active` 只判 `ACTIVE` 與未刪（`internal/product/export.go:72`），`DeductSkuStock` 不看商品；所以店家被停權或未完成開店後，購物車裡的商品仍可結帳扣庫存。這正是「每個 query 各寫一次」的失效樣子，單一定義會一起修掉。

### 業界基準
- 資料庫層：把「誰看得到」寫成一個 view 或函式（Postgres row-level security 的「policy」是同一個想法），所有讀取經過它；Shopify、Amazon 的商品「可售性（buyability／availability）」都是單一服務算出的一個布林，前台、購物車、結帳共用，結帳在提交點再算一次【推，憑業界常識，未上網查】。
- 「提交點再算一次」：結帳最後一步在扣庫存的 transaction 裡重新判斷，不信任購物車讀到的舊值。

### 既有機制為什麼存在
- 各 query 自己 join `seller.pub_exposable_stores(at)`：B4-1～3 時只有「狀態＋曝光」兩個條件，重複寫還看得住【推】。
- `GetPublicProduct` 收 `ARCHIVED`：讓賣家自己下架的商品，分享出去的連結仍能打開並顯示不可購買（`TestArchivedProductPageIsPublicButDraftIsNot`，v1 已查）。
- 模組間讀別人的資料只經 `pub_*` 函式，arch 測試 `TestSQLCReadsOnlyItsOwnSchemaAndAllowedPubContracts` 機械檢查（`internal/arch/sqlc_test.go`）【量】。模組自己的函式放 `db/objects.sql`，`CheckDesiredStateSplit` 檢查放對檔（`internal/arch/sqlc.go:189`）【量】；catalog 目前沒有 `objects.sql`【量】。

### 選項：形狀

| 選項 | 內容 | 失效條件 |
|---|---|---|
| **A（建議）SQL 函式** `catalog.buyer_visible_products(at timestamptz) RETURNS SETOF catalog.products`，`LANGUAGE sql STABLE`，放 `objects.sql`、手寫 migration | 能帶 `at`（`pub_exposable_stores(at)` 需要時間參數，測試注入時鐘）；sqlc 把 SETOF 本模組的表型別化 | Postgres 不把它內聯（inline）進呼叫端的查詢，列表變成先算全部再過濾。單層的 `pub_*` 函式內聯已有前例：im、review 以逐列 `EXISTS` 呼叫 `seller.pub_content_takedowns()`（`im/db/queries/conversations.sql:66`、`review/db/queries/own.sql:33`），migration `20261121120000_seller_pub_content_takedowns_enforced_filter.sql` 的說明寫明這是為了走部分唯一索引 `measure_contents_one_enforced`【文件】。注意：`seller/db/pub.sql:40-42` 是給 sqlc 型別檢查用的 `$$` 存根，資料庫裡的實際定義（`seller/db/objects.sql:294`、上述 migration）是 `BEGIN ATOMIC`【量】，所以 body 寫法不是主要風險。**真正未驗證的風險是外層函式再包住巢狀的 `pub_*` 函式**（`buyer_visible_products` 內呼叫 `pub_exposable_stores(at)` 與 `pub_content_takedowns()`），兩層都要被內聯才不會退化【推】→ 驗收以 `EXPLAIN` 判定（見 §5 B4-5a） |
| B view `catalog.buyer_visible_products` | 最直觀 | view 不能帶參數，只能用 `now()`：測試無法注入時間，且與 `pub_exposable_stores(at)` 的介面不合 |
| C 只靠 sqlc 慣例（每個 query 照抄一段謂詞＋lint 比對字串） | 改動最小 | 就是現況；漏洞已經證明它會漏（上面兩處） |

### 選項：定義的細節

- 條件：`p.status = 'ACTIVE' AND p.deleted_at IS NULL AND p.store_id IN (SELECT store_id FROM seller.pub_exposable_stores(at)) AND NOT EXISTS (SELECT 1 FROM seller.pub_content_takedowns() t WHERE t.content_kind = 'PRODUCT' AND t.content_id = p.id)`。措施存在性用既有的 partial unique index `measure_contents_one_enforced (content_kind, content_id)`【量】，不需新索引。
- 措施的部分在 B4-5a 就寫進函式，**且 B4-5a 一併完成 seller 端 `PRODUCT` 種類的全部資料模型**：`content_kind` 加 `PRODUCT`（CHECK 與 `platform/contentkind` 的 `contentkind.Product`）、`snapshot_state` 加 `OWNER_HELD`（連動 `measure_contents_snapshot_state_check`、`measure_contents_snapshot_fields_check`：`OWNER_HELD` 的 `snapshot_*` 與 `link_*` 欄全 NULL）、`measure_contents_link_check`（`seller/db/schema.sql:575-591`）加 `PRODUCT` 分支（`link_*` 全 NULL）。理由：B4-5a 的措施分支要經真正的 `CreateMeasure` 測（建立 `PRODUCT` 措施列），不手寫 INSERT 造列；半套種類（只有列舉值、沒有快照狀態與 link 分支）會讓 `CreateMeasure` 無法建出合法的 `PRODUCT` 列。選這個而不是把措施條件移到 B4-5c：後者讓 B4-5c 回頭再改一次可見性函式與它的測試。（2026-11-28 Will 定案；審查第 2 輪 F7。）
- `ARCHIVED` 單筆頁（已決：D24 Q10，Will 2026-11-26，`backend/docs/plans/2026-11-24-b4-catalog.md` D24 列）：**第二個函式** `catalog.buyer_viewable_products(at)` = 同條件但狀態 `IN ('ACTIVE','ARCHIVED')`，只給 `GetPublicProduct` 用；可見性仍只在一個檔、兩個函式共用同一段曝光與措施條件（後者呼叫前者的條件，或兩者都呼叫一個內部 `catalog.product_buyer_blocked(p, at)`）。
- 結帳：`GetSkuSnapshots` 的 `active` 改為「商品在 `buyer_visible_products(at)` 裡」；`DeductSkuStock` 改成 `UPDATE … FROM catalog.buyer_visible_products(now)`，不可見就扣 0 列，`whyNotDeducted` 回新的原因 `PRODUCT_NOT_PURCHASABLE`（提交點再判一次）。
- `GetProductCards(include_inactive=true)`（收藏、瀏覽紀錄要顯示「已下架」卡）：保持可讀到，但卡片多回一個 `visible` 布林由函式算出，client 依它顯示不可點；不在卡片上露出被措施下架的標題與圖？——建議不露，`visible=false` 的卡只回 ID 與「已無法瀏覽」狀態。這是跟隨契約的顯示變更（proto 加欄位），屬 UI 規則的「跟隨 API 契約」，不另問。

### lint（CI 失敗條件）

放 `internal/arch`（與 `CheckPubShapeUse` 同一層，既有 Go 測試即 CI 閘門），不另寫 scripts/：
- 規則：catalog `db/queries/*.sql` 的每個 `-- name:` 區塊，若文字參照 `catalog.products`（含 `JOIN`），且 query 名不在 `arch` 內的白名單 map（上表非買家面清單，每筆附一句用途），就失敗；參照 `catalog.buyer_visible_products(` 或 `catalog.buyer_viewable_products(` 的不受限。
- 也擋 catalog query 直接 join `seller.pub_exposable_stores` 或 `seller.pub_content_takedowns`（只有 `objects.sql` 的函式可以），否則有人會繞過函式自己拼謂詞。
- 新 query 預設不在白名單 → 失敗，迫使作者選邊。

**什麼事實會推翻建議**：`EXPLAIN` 顯示兩種 body 寫法都無法內聯且列表 p95 明顯退化 → 改為在 query 內以 sqlc 的 `sqlc.embed` 或 CTE 共用片段，lint 改比對片段名。
**可逆性**：低成本；函式與 lint 都在本模組內。
**限制帳本**

| 限制 | 分類 | 來源 |
|---|---|---|
| 模組只能經 `pub_*` 讀 seller | REAL（本計畫內不改的架構規則，ADR `2026-09-24-go-modular-monolith` D3） | `catalog/db/sqlc.yaml` 註解 |
| D7：措施不寫進商品狀態欄 | SELF-IMPOSED（已決，與基準一致） | decision-brief D7 |
| catalog 沒有 `objects.sql` | SELF-IMPOSED（新增即可，需改 `sqlc.yaml` 讀它） | 【量】 |

---

## 2. media 層的法律保全（B4-5b）

### 決定（Will 2026-11-28，選項 (b)）
措施建立或來函凍結時，擁有者（catalog）用新的 media `CopyToSecure` 把商品圖複製成 SECURE（加密、不公開）證據副本，**保全只登記在 SECURE 副本上**；公開原圖照一般刪除流程處理（編輯換圖、商品刪除、退場清理都照舊 `MarkDeleted`），公開網址失效就是「停止公開存取」。
依據：
- 歐盟 TERREG（2021/784）第 6 條：保存被移除內容及相關資料，目的是供行政、司法救濟與調查用。【文件，未於本次查證】
- 美國 18 U.S.C. §2258A：通報後保存內容的義務。【文件，未於本次查證】
- 雲端物件保全（S3 Object Lock、GCS hold，見下「業界基準」，這兩項本簡報已讀原文）只鎖住物件不被刪或覆寫，**不管公開存取**：被保全的公開物件仍然是公開的。所以「保存證據」與「停止公開」是兩件事：保全放在不公開的副本，公開面照刪。

被放棄的選項 (a)（保全原公開圖、URL 仍可存取）：下架後原圖仍可公開取得，處置只做一半。

### 現況【量】
- `MarkDeleted` 只在擁有者的 transaction 內寫 `media.asset_deletion_requests`（`internal/reclaim/reclaim.go:87`，SQL 是 `RequestDeletions`，`db/queries/assets.sql:23-29`）；`ApplyDeletions` 由排程呼叫 `ApplyDeletionRequests`（`assets.sql:31-44`）把未套用的請求標成 `DELETING`；`DeleteAssets` 在 transaction 外刪物件（`reclaim.go:228`）。
- 呼叫 `MarkDeleted` 的地方（不含 media 自己與測試）共 19 行、16 個檔（本輪重跑【量】；審查者得 19 行、15 檔，檔數差一，以本次輸出為準）：`rg -n "\.MarkDeleted\(" backend/internal backend/cmd --glob '*.go' --glob '!*_test.go' --glob '!internal/modules/media/**'`。與本題相關的：catalog 編輯換圖（`catalog/internal/product/product.go:631`）、社群貼文編輯（`social/internal/post/post.go:443`）、各模組 purge（social `purge.go:371`、review `purge.go:163`、im `purge.go:171`、seller `purge.go:160,250`）、moderation 保存期清理（`retention.go:120`）。
- **catalog 沒有訂閱 `StoreRetired`**（`rg -n StoreRetired backend/internal/modules/catalog backend/cmd/hawkr` 只有 crm、moderation、imageai、sellerrelay、im、social）：退場店家的商品圖今天**永遠不會刪**。這是另一個缺口（保存期過後的個資清理），不屬 B4-5，排成明確項目：B4-6 加一步「catalog 訂閱 `seller.StoreRetired`（同 transaction，照 moderation、im 的寫法），軟刪該店商品並對圖片 `MarkDeleted`」，觸發條件：B4-5b 完成。已寫進計畫的 B4-6 與 catalog `CARRYOVER.md`。
- 公開圖片的 `Cache-Control` 是 `public, max-age=3600`，沒有 CDN purge（`internal/storage/keys.go:23`）：原圖刪除後最長 1 小時內快取仍可能送出，這是「停止公開」的已知延遲（SELF-IMPOSED，沒有 CDN purge 能力前不處理）。

### 進入 `DELETING` 與實際刪物件的路徑盤點（F2 重做，保全只在 SECURE 副本，但副本也會走這些路徑）【量】

指令：`rg -n "'DELETING'" backend/internal/modules/media --glob '*.sql' --glob '!migrations/**'`、`rg -n "Store\.Delete\(|deleteSecureObjects" backend/internal/modules/media --glob '*.go' --glob '!*_test.go'`。

| # | 路徑 | 位置 | 做什麼 | 保全檢查放哪 |
|---|---|---|---|---|
| 1 | `RequestDeletions` | `assets.sql:23-29`、`reclaim.go:87` | 只寫請求列，不改資產狀態 | 不改（擁有者照常表達刪除意圖，19 個呼叫點不動） |
| 2 | `ApplyDeletionRequests` | `assets.sql:31-44` | 請求列 `picked` → 資產 `STAGED→DELETING`，並把請求標 `applied_at` | `picked` 的 WHERE 加 `NOT EXISTS (SELECT 1 FROM media.asset_holds h WHERE h.asset_id = r.asset_id AND h.until > sqlc.arg(now))`（放在 `LIMIT` 之前，被保全的請求不佔批次）；被保全的請求維持 `applied_at IS NULL`，保全到期或解除後下一輪自然套用 |
| 3 | `ClaimAssetsOfUploads` | `assets.sql:46-48`，唯一呼叫端 `reclaim.go:208` | 暫存上傳逾期回收時把它的資產 `STAGED→DELETING`（不經刪除請求） | WHERE 加同一條 `NOT EXISTS` |
| 4 | `MarkReclaimedUploadDeleting` | `uploads.sql:114-118`、`reclaim.go:203` | 暫存上傳列 → `DELETING`，回傳 `asset_id` | WHERE 加：該列的 `asset_id` 與 `planned_asset_id` 都沒有生效保全。無列時 `reclaimUpload` 要讓整個 transaction 回滾（含前一步 `ReclaimUpload` 插入的 `RECLAIMED` 認領列），回傳 `won=false`，**暫存列維持原狀態、不留在 `DELETING`** |
| 5 | `deleteUpload` | `reclaim.go:296-326` | 刪 `planned_asset_id` 的機敏物件、`tmp_key`、`final_key`（公開資產沒有 planned id 時 `final_key` 就是資產物件） | 刪任何物件之前，先在 transaction 內鎖暫存列與其資產列並再查一次保全；有保全就不刪、同一個 transaction 內把暫存列從 `DELETING` 退回原狀態並刪掉 `RECLAIMED` 認領列 |
| 6 | `deleteAsset` | `reclaim.go:330-364`（物件刪除在 `:331-337`） | 刪物件（公開 `Store.Delete`、SECURE `deleteSecureObjects`），再另開 transaction 標 `DELETED` | 刪物件之前，transaction 內 `SELECT … FOR UPDATE` 鎖資產列並再查一次保全；有保全就不刪物件、資產 `DELETING→STAGED`、請求列 `applied_at` 清回 NULL |
| 7 | `upload` 套件的失敗清理 | `upload/direct.go:58`、`upload/process.go:143,170`、`upload/secure_upload.go:323` | 刪自己上傳失敗留下的 key | 本輪**未讀**這四處；B4-5b 開工第一步讀它們，確認只刪尚未成為資產的 key（不會碰有保全的資產物件）；若會碰到則同樣加再檢查（停止條件） |

不變量（B4-5b 寫進 `media/AGENTS.md`）：**被保全的資產永遠不離開原本的狀態**（`STAGED` 的不會變 `DELETING`），**被保全資產所屬的暫存上傳列不會留在 `DELETING`**；SQL 的認領處（2、3、4）是第一道，物件刪除前的再檢查（5、6）是最後一道，因為「SQL 認領之後、物件刪除之前才登記保全」的競態只有最後一道擋得住。`Hold` 以 `FOR SHARE` 鎖資產列並在資產不是 `STAGED` 時回 `AssetGone`，與上面的 `FOR UPDATE` 互斥。呼叫方（快照）收到 `AssetGone` 時在快照內記「登記時圖片已刪除」，不失敗。

### 業界基準【量】
- AWS S3 Object Lock 的 legal hold：「prevents an object version from being overwritten or deleted… doesn't have an associated fixed amount of time and remains in effect until removed」；「Object Lock works only in buckets that have S3 Versioning enabled」；只保護指定的版本，簡單 DELETE 仍會加 delete marker。<https://docs.aws.amazon.com/AmazonS3/latest/userguide/object-lock.html>
- GCS temporary hold／event-based hold：持有期間「the object cannot be deleted or replaced」，沒有到期，要手動解除。<https://docs.cloud.google.com/storage/docs/object-holds>
- 共通點：保全是「附在物件上、多來源可疊加、明確解除」的旗標，與刪除意圖分開；它不管物件是否公開。雲端版沒有到期日，到期由應用程式解除。

### 既有機制為什麼存在
刪除請求與套用分兩段（B1-D25）：擁有者只寫意圖，media 自己的 transaction 決定何時真正刪，避免擁有者 transaction 回滾後物件已刪。這個分段正好是插入保全的位置。

### 設計
```
media.asset_holds (
  asset_id   UUID NOT NULL REFERENCES media.assets(id),
  source     TEXT NOT NULL,      -- 例如 'catalog.evidence_snapshot'
  source_id  UUID NOT NULL,
  until      TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (asset_id, source, source_id)
)
```
- API：`Hold(ctx, assetIDs, source, sourceID, until)`、`ExtendHold(source, sourceID, until)`（只能延長）、`ReleaseHold(source, sourceID)`；都要求在呼叫方 transaction 內（同 `MarkDeleted`）。
- `CopyToSecure(ctx, assetID, …) (newAssetID, error)`：把公開資產複製成 SECURE 資產（走 media 既有的機敏加密）。規則 5 禁止在 DB transaction 內做外部 I/O，所以實際的物件複製不能在擁有者的 transaction 內做：形狀是兩段式（transaction 內只寫「複製請求」並回傳預先配好的新資產 ID，擁有者拿它登記保全；worker 在 transaction 外複製、完成後才把副本資產標為可讀），複製完成前原圖不得被刪（原圖上有一筆短暫的「待複製」旁路，不是法律保全，複製完成即釋放）【推：由規則 5 推出，未實作驗證】。若實作發現這個形狀需要在原公開圖上長期登記法律保全才成立，**停下來問 Will**（會偏離「保全只在副本」）。
- 副本的生命週期：快照過期時 catalog 先 `ReleaseHold`、再對副本 `MarkDeleted`（副本走一般刪除流程，不另開路徑）。
- 到期清理：media 的定期 job 刪 `until < now` 的保全列（純整理，正確性不靠它，因為判斷本來就看 `until`）。
- 不做 Object Lock：本機與目前的儲存是 S3 相容的 Garage（`backend/scripts/garage-init.sh`）；Object Lock 需要版本控制 bucket，且 hold 是逐版本的，與我們「刪除＝刪 key」的模型不合；資料庫保全已足以擋住我們自己的刪除路徑。終局與觸發：正式環境選定的儲存支援 Object Lock **且**出現「證據須防竄改（WORM，寫一次不能改）」的要求時，在 `Hold` 同時對副本物件下 legal hold。

| 選項 | 失效條件 |
|---|---|
| **A（採用）DB 保全表＋SQL 認領處跳過＋物件刪除前再檢查** | 有人繞過 media 直接刪儲存物件（今天沒有這種路徑【推】） |
| B 在 `MarkDeleted` 拒絕被保全的資產 | 19 個呼叫點都要處理新錯誤，且擁有者的 transaction（例如商品編輯）會因證據保全而失敗——賣家改不了自己的商品 |
| C 儲存層 Object Lock 為唯一來源 | Garage 支援度未知【推】；需版本控制；沒有到期，仍要應用程式解除 |

**可逆性**：中等；新表、一個複製 API 與幾個 WHERE 條件，沒有上線資料。

**限制帳本**

| 限制 | 分類 | 來源 |
|---|---|---|
| 物件刪除在 transaction 外（B1-D25） | SELF-IMPOSED（自己的設計，理由仍成立） | `reclaim.go:84` |
| 外部 I/O 不在 DB transaction 內（`CopyToSecure` 因此兩段式） | SELF-IMPOSED（單體規則 5，理由仍成立：交易內外部呼叫會拖住鎖） | `backend/AGENTS.md` 規則 5 |
| 儲存是 Garage／S3 相容，未選定正式供應商 | REAL（還沒有可用的正式儲存） | `garage-init.sh` |
| 食安法草案 43-1：通知時持有的資料保存 180 日 | REAL（法規，草案未施行） | v1／reports-brief R5 |

---

## 3. 由擁有模組產生的證據快照（B4-5c、B4-5e；圖片保全依 §2 的決定）

### 現況【量】
- 今天的快照由**呼叫方**組好交給 seller：admin 或 moderation 呼叫 `seller.NewContentSnapshot(kind, body, authoredAt, mediaCount, …)`（`seller/governance.go:740`、`moderation/internal/handle/handle.go:396`），seller 存進 `measure_contents.snapshot_*`；只存本文與**圖片張數**，不存圖片本身。
- 撤銷滿 184 天清空：`ClearExpiredContentSnapshots`（`seller/db/queries/measures.sql:530`）；店家退場時 `ClearContentSnapshotsOfStores` 立即清空。保存常數 `ContentSnapshotRetention`（`seller.go:696`）。
- 來函：`authority_orders.hold_until`，可延長（`ExtendAuthorityOrderHold`，`authority_orders.sql:84-88`），**完成也可能延長**：`CompleteAuthorityOrder`（`authority_orders.sql:73-77`）寫入新的 `hold_until`，條件是 `hold_until <= 新值`；資料提供類來函完成時延到「提供時間＋AuthorityHold」（`seller/internal/governance/authority.go:469-490`）【量】；`authority_order_targets.product_ids TEXT[] ≤ 50`；登記時不發任何事件（v1 已查）。
- 依賴方向：`catalog: {seller, buyer, media}`、`seller: {identity, media, payment}`（`internal/arch/modules.go:19,22`）。seller 不能呼叫 catalog。

### 業界基準【推，與 v1 一致】
證據保全的通例是「收到要求的那一刻，由持有資料的系統凍結一份當時的副本，暫停相關刪除」；內容的擁有者最清楚內容的完整形狀（商品有描述區塊、SKU、價格、多張圖），交給呼叫方拼字串會漏。

### 設計（建議）
```
catalog.evidence_snapshots (
  id          UUID PRIMARY KEY,
  source      TEXT NOT NULL CHECK (source IN ('MEASURE','AUTHORITY_ORDER')),
  source_id   UUID NOT NULL,
  product_id  UUID NOT NULL,
  store_id    BIGINT NOT NULL,
  captured_at TIMESTAMPTZ NOT NULL,
  content     JSONB NOT NULL,          -- 標題、描述區塊、選項軸、SKU 與價格、圖片的 SECURE 副本 asset_id（與原公開圖 asset_id 並列供對照）與 alt、content_version
  retain_until TIMESTAMPTZ,            -- NULL = 來源仍生效
  UNIQUE (source, source_id, product_id)
)
```
- 不可改：沒有 UPDATE 內容的 query；只有 `retain_until` 會被設定或延長（lint：`evidence_snapshots` 的 UPDATE 只准動 `retain_until`）。
- 建立：catalog 訂閱 `MeasureCreated`（同 transaction，照 review `TakedownCreated` 的寫法），對 `content_kind = PRODUCT` 的每件：鎖商品列、核對屬於措施的店家（不是就回錯誤讓措施回滾）、對每張圖 `media.CopyToSecure` 取得 SECURE 副本、寫快照（內容指向副本）、`media.Hold(副本, 'catalog.evidence_snapshot', 快照 id, until=+∞ 以遠的哨兵值)`。**只對副本登記保全**；公開原圖不保全，之後的編輯換圖或商品刪除照舊 `MarkDeleted`。
- 撤銷：訂閱 `MeasureRevoked`，`retain_until = revoked_at + 184 天`（與 seller 的 `ContentSnapshotRetention` 同值，引用同一常數），同步以 `retain_until` 重設副本的保全（保全只因這份快照存在，可以重設）。
- 來函：seller 在登記來函與追加對象的 transaction 發 `AuthorityOrderTargetsRecorded{order_id, store_id, product_ids, hold_until}`；延長 `hold_until` 發 `AuthorityOrderHoldExtended{order_id, hold_until}`，**完成來函時若 `hold_until` 變晚也發同一事件**（審查意見原說完成不需事件，查證不成立：完成只要求新值不早於舊值，會延長，見上方現況）；撤銷來函發 `AuthorityOrderRevoked`（retain 改為撤銷時間；撤銷後是否立即釋放，照 seller 店家層級保存 `purge.sql:37` 的做法：`revoked_at IS NULL` 才保全，即立即釋放）。catalog 對每件商品寫 `source='AUTHORITY_ORDER'` 的快照，`retain_until = hold_until`。
- 多來源：同一商品被措施與來函各自凍結時是兩份快照、兩組保全，各自到期，不需要「取最晚」的合併邏輯（保全表天然疊加）。
- 清理：catalog 定期 job 刪 `retain_until < now` 的快照、`ReleaseHold`，並對副本 `MarkDeleted`。
- 店家退場（已決，不列問題）：**措施來源**的快照跟既有 SSOT `ClearContentSnapshotsOfStores`（seller 退場時立即清空措施內容快照）一致，退場時清空、`ReleaseHold` 並對副本 `MarkDeleted`；**來函來源**保留到 `hold_until`（法定保存，優先於退場清理）。
- seller 端：`measure_contents` 對 `PRODUCT` 的列，`snapshot_state` 為 `OWNER_HELD`（快照在擁有模組），`snapshot_*` 欄位全 NULL；`OWNER_HELD` 與 `measure_contents_link_check` 的 `PRODUCT` 分支已在 B4-5a 隨 `PRODUCT` 種類一起加入（F7）。admin 看措施詳情時經 catalog 的 Connect RPC `GetEvidenceSnapshot(source, source_id, product_id)` 讀。

### 措施不存快照 ID（由依賴規則決定）
依賴方向 catalog→seller（`internal/arch/modules.go:19,22`，REAL，本計畫不改的架構規則），seller 寫措施時拿不到 catalog 產生的 ID；catalog 的訂閱者在同一 transaction 也寫不到 seller 的表。因此採 A，不交 Will 決定。註：這偏離 Will 原話「措施存快照 ID」。

| 選項 | 失效條件 |
|---|---|
| **A（建議）複合鍵引用**：快照以 `(source, source_id, product_id)` 唯一，措施列不存 ID | 同一措施需要對同一商品存兩份快照（目前沒有這需求） |
| B 快照 ID 由 seller 先產生（UUIDv7）放進事件，catalog 用它當主鍵，seller 存同一個值 | 等於 A 換個名字，但 seller 存了一個它無法驗證存在的外部 ID |
| C seller 宣告 port（`ContentEvidence`），`cmd/hawkr` 注入 catalog 實作，seller 建措施前呼叫取 ID | 反轉依賴（seller 執行期依賴 catalog），arch 依賴表要加例外 |

### social 快照的改造：延後
觸發條件：下一個新的內容種類加入措施時，把 social、im、review 的快照也改成擁有模組寫、存圖片保全。代價（命名）：延後期間有兩種快照形狀並存（`CAPTURED` 存在 seller，`OWNER_HELD` 存在擁有模組），admin 措施詳情要分兩種讀法。

**什麼事實會推翻**：法規定稿要求保存「指名之前」的版本 → 需要商品版本歷史，快照不夠。
**可逆性**：中等；新表與事件，沒有上線資料。

**限制帳本**

| 限制 | 分類 | 來源 |
|---|---|---|
| 依賴方向 catalog→seller | REAL（模組架構 ADR，本計畫內不改） | `internal/arch/modules.go:19,22` |
| 撤銷後 184 天清除 | SELF-IMPOSED（自己的保存政策，取自詐欺條例六個月） | `seller.go:696` |
| 來函保存期 `hold_until` | REAL（法規） | `authority_order.go:49` |
| social 改造延後 | SELF-IMPOSED（分段；觸發條件如上） | Will 2026-11-28 |
| 保全只在 SECURE 副本、公開原圖照常刪除 | 已決（Will 2026-11-28 選 (b)，法規依據見 §2） | §2 |

---

## 4. admin 介面（B4-5e；Will 2026-11-28 同意下表欄位與文案）

**刪**
- `admin/src/app/admin/products/page.tsx` 的 `handleToggleStatus` 與其按鈕（:87 起；打舊 REST，管理員直接把賣家商品切 `ACTIVE`／`INACTIVE`，無理由、不可申覆）。
- `merchants/[id]/ProductForceUnpublishDialog.tsx`（它自己的註解寫明「沒有理由欄位…是刻意的」，因為舊端點收不了理由【量】）。

**新**：`ProductTakedownDialog`，重用 `posts/ContentTakedownDialog.tsx` 的欄位元件與 `buildTakedown`／`validateTakedown`／`takedownRequestKey`（抽成共用，不複製），送 Connect `CreateMeasure`（`kind CONTENT_TAKEDOWN`、`contents[].kind PRODUCT`）。

| 項目 | 文案（建議） |
|---|---|
| 標題 | 下架商品 |
| 說明 | 商品 {標題}（店家 {店家名}）會以內容下架措施下架，買家看不到也不能購買；賣家可以申覆。 |
| 已選商品 | 清單 1～20 件（`MaxTakedownContents` 沿用），商家頁可多選，products 頁單件 |
| 類別 * | 下拉，`CONTENT_TAKEDOWN_CATEGORIES`（`admin/src/lib/policy-category.ts`）加上商品專屬的違規類別，例如食品標示不實、食安疑慮、禁售品、仿冒；**確切清單在 B4-5e 開工前與 Will 確認**（同時決定後端 policy category 值域與 `ErrorReason` 是否要加） |
| 嚴重度 * | 同貼文 |
| 給賣家的理由 * | 同貼文 |
| 內部註記 * | 同貼文 |
| 按鈕 | 確認下架／下架中…／關閉 |
| 失敗 | 下架失敗，商品狀態沒有變動；請確認網路狀態後再試一次。 |

- products 列表：被措施限制的商品顯示「平台下架中」標記，連到既有措施詳情頁；撤銷、裁定申覆沿用措施詳情頁，不新做。
- 措施詳情頁對 `PRODUCT` 內容顯示 catalog 快照（標題、描述、SKU 價格、圖片）。

失效條件：admin 需要「不算違規」的下架（分類放錯）。那應是通知賣家修改，不是處置。可逆性：低成本（前端）。

---

## 5. 步驟拆分

共通：實作派 Sonnet；commit 前 `gofmt -l`、`make lint-monolith`、`GOWORK=off go vet -tags integration ./...`、`python3 scripts/mutation/run.py --validate`；integration 與 mutation 交 CI。下列測試名是計畫名稱，實作時照此命名；mutation 以「檔、舊字串→新字串、預期變紅的測試」登記進 `mutation/b4-5.json`。

### B4-5a 可見性單一定義＋PRODUCT 種類與 OWNER_HELD
- 範圍：〔新〕`catalog/db/objects.sql`（`buyer_visible_products`、`buyer_viewable_products`）、手寫 migration、`sqlc.yaml` 讀 objects.sql；`internal/arch` 的 catalog 可見性 lint 與白名單。〔改〕seller 的 `PRODUCT` 種類資料模型一次做完（F7）：`measure_contents_content_kind_check` 加 `PRODUCT`、`snapshot_state` 加 `OWNER_HELD`（`snapshot_state_check`、`snapshot_fields_check` 同步）、`measure_contents_link_check`（`seller/db/schema.sql:575-591`）加 `PRODUCT` 分支（`link_*` 全 NULL）、`platform/contentkind` 加 `Product`（`Values()`、`Valid`）、`CreateMeasure` 接受 `PRODUCT` 內容並以 `OWNER_HELD` 存（`snapshot_*` 全 NULL）；共用文案目錄（`make uitext`，三端生成物）；§1 表內 10 支 query 改走函式；`DeductSkuStock` 與 `whyNotDeducted` 新原因；`GetProductCards` 加 `visible`；proto 對應欄位與 `ErrorReason`，TS／Dart 生成。
- 被擋於：無。
- 既有缺陷修正：`GetSkuSnapshots`／`DeductSkuStock` 不看店家曝光（§1「新發現的漏洞」）留在本步範圍，PR 標題／內文與計畫都明寫為**既有缺陷修正**，並附修正前會失敗的回歸測試（停權店家的 SKU：`CheckoutSnapshot` 回 `active=true`、`DeductStock` 成功扣庫存——修正前綠即代表重現成功，修正後改為斷言不可購買）。
- 消費端：`rg -n "pub_exposable_stores|pub_content_takedowns" backend/internal/modules/catalog/db/queries`（完成後為 0）；`rg -n "\.(PublicProduct|ListPublicProducts|ListPublicProductsOfStores|ProductCards|CheckoutSnapshot|SkuCartInfo|SkuPurchasable|StoresWithPublicProducts|Suggest\w+|TrendingHashtags|DeductStock)\(" backend/internal backend/cmd --glob '!internal/modules/catalog/**'`；`rg -n "contentkind\.|ContentKind|REVIEW_REPLY" backend/internal backend/cmd api packages admin/src website buyer-app/lib seller-app/lib`（種類列舉的擴充點）。
- 不能動的部分：`legacy/catalogserver`（B4-6 刪）、seller schema 中 `PRODUCT` 種類以外的部分、admin、catalog 快照與事件訂閱（B4-5c）。
- 驗收：
  - integration（措施一律經真的 `CreateMeasure` 建立，不手寫 INSERT）：`TestSuspendedStoreProductIsNotVisible`（停權店家商品：公開頁 NotFound、列表無、購物車快照 `active=false`、`DeductStock` 回 `PRODUCT_NOT_PURCHASABLE`）；`TestProductUnderMeasureIsNotVisible`（建立 `PRODUCT` 措施後同上；撤銷後恢復）；`TestCreateMeasureStoresProductContentAsOwnerHeld`（`snapshot_state = 'OWNER_HELD'`、`snapshot_*` 與 `link_*` 全 NULL）；`TestArchivedProductPageStaysViewableUnderViewableFunction`（D24）。
  - EXPLAIN（F9 與新增的扣庫存項）：seed 10,000 件商品、200 家店（其中 20 家停權）、50 筆 `PRODUCT` 措施，`ANALYZE` 後對 `ListPublicProducts` 與 `DeductSkuStock` 各跑 `EXPLAIN`。通過條件：計畫中**沒有 `Function Scan` 節點**（代表 `buyer_visible_products` 被內聯，不是整個函式先算完再過濾），**且**對 `catalog.products` 沒有 Seq Scan，且走具名索引（索引名寫進 PR；列表走 status／store 相關索引）。`DeductSkuStock` 必須維持「以 SKU 主鍵更新」的路徑（更新 `catalog.product_skus` 時以主鍵定位，可見性只是同一個 `UPDATE … FROM` 的過濾，不得變成掃商品表再回頭配 SKU）。任一條不過 = 停止條件。
  - lint 單元測試：`TestCatalogVisibilityLintRejectsUnlistedProductQuery`（新增一支讀 `catalog.products` 且不在白名單的 query → 失敗）。
  - mutation：①`objects.sql` 拿掉 `store_id IN (SELECT store_id FROM seller.pub_exposable_stores(at))` → `TestSuspendedStoreProductIsNotVisible` 紅；②拿掉 `NOT EXISTS (… pub_content_takedowns() …)` → `TestProductUnderMeasureIsNotVisible` 紅；③`DeductSkuStock` 拿掉 `FROM catalog.buyer_visible_products(` 的過濾 → `TestSuspendedStoreProductIsNotVisible` 的扣庫存斷言紅；④`GetSkuSnapshots` 的 `active` 改回只看 status → 同一測試的購物車斷言紅；⑤arch 白名單多放 `GetPublicProduct` 以外的買家面 query（例如 `ListPublicProducts`）→ `TestCatalogVisibilityLintRejectsUnlistedProductQuery` 或白名單比對測試紅；⑥`contentkind.Values()` 拿掉 `Product` → `TestCreateMeasureStoresProductContentAsOwnerHeld` 紅。
- 停止條件：巢狀 `pub_*` 函式無法內聯、上述 EXPLAIN 不通過；或發現 catalog 以外有 SQL 直接讀 `catalog.products`（目前 0）。

### B4-5b media 保全與 CopyToSecure
- 範圍：〔新〕`media.asset_holds`、`Hold`／`ExtendHold`／`ReleaseHold`、到期整理 job、`CopyToSecure`（兩段式，見 §2）。〔改〕§2 路徑盤點表的 2–6：`ApplyDeletionRequests`、`ClaimAssetsOfUploads`、`MarkReclaimedUploadDeleting` 的 SQL 加保全條件；`reclaimUpload` 在無列時回滾；`deleteUpload`、`deleteAsset` 刪物件前鎖列再檢查；第 7 項先讀、必要時同樣處理。`media/AGENTS.md` 加不變量。
- 被擋於：無（可與 B4-5a 平行）。
- 消費端：`rg -n "\.MarkDeleted\(|ApplyDeletions|DeleteAssets" backend/internal backend/cmd`（呼叫端不需改，只確認）；`rg -n "'DELETING'" backend/internal/modules/media --glob '*.sql' --glob '!migrations/**'`（完成後每一處進入 `DELETING` 的 SQL 都有保全條件，除了 `ListDeleting*`／`Mark*Deleted` 這類只讀或收尾的）。
- 不能動的部分：19 個 `MarkDeleted` 呼叫點；儲存層 Object Lock。
- 驗收（每條路徑一支 integration，下列對應 §2 表的編號）：
  - 路徑 2：`TestApplyDeletionsSkipsHeldAsset`（保全中的資產 `MarkDeleted` 後跑 Apply＋Delete，資產仍 `STAGED`、物件仍在、請求 `applied_at` 仍為 NULL）；`TestExpiredHoldAllowsDeletion`（保全到期後同一請求被套用並刪除）。
  - 路徑 3：`TestClaimAssetsOfUploadsSkipsHeldAsset`（直接造一筆保全，逾期上傳回收後資產仍 `STAGED`）。
  - 路徑 4：`TestReclaimUploadWithHeldAssetKeepsUploadStateAndRollsBackClaim`（暫存列維持原狀態、沒有 `RECLAIMED` 認領列、不在 `DELETING`）。
  - 路徑 5：`TestDeleteUploadRechecksHoldBeforeDeletingObjects`（測試直接把一筆暫存列設成 `DELETING` 並造保全 → 不刪任何物件、暫存列退回原狀態、認領列被刪；公開 `final_key` 與 SECURE `planned_asset_id` 各一個案例）。
  - 路徑 6：`TestDeleteAssetRechecksHoldBeforeDeletingObject`（Apply 之後、Delete 之前登記保全 → 物件保留、資產退回 `STAGED`、請求 `applied_at` 清回 NULL；公開與 SECURE 各一個案例）。
  - 其他：`TestHoldOnDeletingAssetReturnsAssetGone`；`TestTwoHoldSourcesStackAndReleasingOneKeepsAsset`；`TestCopyToSecureProducesIndependentSecureCopy`（原圖之後被刪，副本仍可讀、內容相同）；`TestCopyToSecureKeepsOriginalUntilCopyCompletes`。
  - mutation：①`ApplyDeletionRequests` 的 `picked` 拿掉 `NOT EXISTS` → `TestApplyDeletionsSkipsHeldAsset` 紅；②同處 `h.until > sqlc.arg(now)` 改 `h.until < sqlc.arg(now)` → `TestApplyDeletionsSkipsHeldAsset`（保全中被套用）與 `TestExpiredHoldAllowsDeletion`（到期仍不套用）紅；③`ClaimAssetsOfUploads` 拿掉 `NOT EXISTS` → `TestClaimAssetsOfUploadsSkipsHeldAsset` 紅；④`MarkReclaimedUploadDeleting` 拿掉保全條件 → `TestReclaimUploadWithHeldAssetKeepsUploadStateAndRollsBackClaim` 紅；⑤`deleteUpload` 拿掉刪前再檢查 → `TestDeleteUploadRechecksHoldBeforeDeletingObjects` 紅；⑥`deleteAsset` 拿掉刪前再檢查 → `TestDeleteAssetRechecksHoldBeforeDeletingObject` 紅；⑦`Hold` 拿掉「資產不是 `STAGED` 回 `AssetGone`」→ `TestHoldOnDeletingAssetReturnsAssetGone` 紅。
- 停止條件：`STAGED` 以外還有表示「存在中」的狀態而語意不清；§2 第 7 項的清理會碰到已成為資產的物件而再檢查放不進去；`deleteUpload` 無法判定退回哪個原狀態（需要新欄位記錄前一狀態時停下回報）；`CopyToSecure` 兩段式不成立、需要在公開原圖上長期登記法律保全（偏離「保全只在副本」）；Garage 刪除行為與預期不同。

### B4-5c 證據快照與上架檢查
- 範圍：〔新〕catalog 訂閱 `MeasureCreated`／`MeasureRevoked`、`catalog.evidence_snapshots`、清理 job（建立時對圖片 `CopyToSecure` 並只對副本 `Hold`）；`GetEvidenceSnapshot` RPC；`SetProductStatus` 在措施期間拒絕切 `ACTIVE`（新 `ErrorReason`；必要條件：主計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md:392`（Will 2026-10-18）要求措施「擋住重新上架」，少了它賣家看到自己商品是 `ACTIVE`、買家卻看不到，狀態誤導。它只拒絕這個狀態轉換，措施從不寫 `status` 欄，D7 不受影響；審查意見建議移除，不採納）；`cmd/hawkr` 註冊訂閱。（`PRODUCT` 種類、`OWNER_HELD` 與 link 分支已在 B4-5a。）
- 被擋於：B4-5a、B4-5b。
- 消費端：`rg -n "MeasureCreated|MeasureRevoked" backend/internal backend/cmd`；`rg -n "SetProductStatus" backend/internal backend/cmd`。
- 不能動的部分：social／im／review 的快照路徑（延後）；admin UI；seller 的 `PRODUCT` 資料模型（B4-5a 已定）。
- 驗收：
  - integration：`TestMeasureCreatedHidesProductAndSellerCannotReactivate`（建立措施後公開頁 NotFound、賣家切 `ACTIVE` 被拒、編輯仍可）；`TestRevokeMakesActiveProductVisibleAgain`（撤銷後原 `ACTIVE` 商品立即恢復可見，B4 計畫驗收句改寫為此）；`TestMeasureForProductOfOtherStoreRollsBackWhole`；`TestSnapshotEqualsContentAtCreationAndSurvivesEdits`（快照內容等於建立當下）；`TestOriginalImageDeletedAfterEditButSecureCopyKept`（編輯換圖後公開舊圖被刪、SECURE 副本仍在且可讀）；`TestSnapshotCleanupReleasesHoldAndDeletesCopiesAfterRetention`（撤銷 184 天後清理 job 刪快照、釋放保全、副本 `MarkDeleted`）。
  - mutation：①`SetProductStatus` 拿掉措施檢查 → `TestMeasureCreatedHidesProductAndSellerCannotReactivate` 紅；②訂閱改非同步（不在同 transaction）→ `TestMeasureForProductOfOtherStoreRollsBackWhole` 紅；③`Hold` 登記在原公開圖而不是副本 → `TestOriginalImageDeletedAfterEditButSecureCopyKept` 紅；④快照不呼叫 `CopyToSecure`（內容指向原圖）→ 同一測試紅；⑤`retain_until` 改用 `created_at` 起算 → `TestSnapshotCleanupReleasesHoldAndDeletesCopiesAfterRetention` 紅。
- 停止條件：`MeasureCreated` 的內容種類資訊不足以在 catalog 判斷 PRODUCT；只收申覆的流程（無 `REMEDIATION`）因新 kind 需改 seller 狀態機。

### B4-5e 來函凍結事件、admin 措施對話框、商品政策類別
- 範圍：〔新〕seller 事件 `AuthorityOrderTargetsRecorded`、`AuthorityOrderHoldExtended`（延長與「完成時延長」都發）、`AuthorityOrderRevoked`；catalog 訂閱寫 `AUTHORITY_ORDER` 快照（同樣只保全 SECURE 副本）。〔改〕`product_ids TEXT[]→UUID[]`（驗證商品存在：seller 不能讀 catalog，改由 catalog 訂閱者驗證，不存在就回錯讓登記回滾）。admin：§4 全部——新 `ProductTakedownDialog` 走 Connect `CreateMeasure`；刪 `admin/products` 的直接切狀態（`handleToggleStatus`）與 `ProductForceUnpublishDialog`；在 `admin/src/lib/policy-category.ts` 加商品專屬政策類別（例如食品標示不實、食安疑慮、禁售品、仿冒；確切清單開工前與 Will 確認）並同步後端值域。
- 被擋於：B4-5c；商品政策類別的確切清單由 Will 確認。
- 消費端：`rg -n "product_ids|ProductIDs" backend/internal/modules/seller api/hawkr admin/src`；`rg -n "ProductForceUnpublishDialog|handleToggleStatus|admin/products/.*status" admin/src`（完成後為 0）；`rg -n "CONTENT_TAKEDOWN_CATEGORIES" admin/src`。
- 不能動的部分：gateway 舊 REST 路由本身（B4-6 刪）。
- 驗收：
  - integration：`TestAuthorityOrderTargetsFreezeSnapshotAndCopies`（登記指名商品後賣家改圖，SECURE 副本為登記當下）；`TestHoldExtendsWithAuthorityHoldUntil`（延長 `hold_until` 後保全跟著延長）；`TestCompletingProviderOrderExtendsHold`（資料提供類來函完成並延長 `hold_until` 後保全跟著延長）；`TestRevokedAuthorityOrderReleasesHold`；`TestUnknownProductInTargetsFailsRegistration`。
  - admin：lint／typecheck／unit；`ProductTakedownDialog` 送出的 request 形狀測試；類別清單的單元測試。
  - mutation：①事件不在登記 transaction 發 → `TestUnknownProductInTargetsFailsRegistration`（回滾）紅；②延長不發 `AuthorityOrderHoldExtended` → `TestHoldExtendsWithAuthorityHoldUntil` 紅；③完成時不發延長事件 → `TestCompletingProviderOrderExtendsHold` 紅；④對話框把 `PRODUCT` 換成 `POST` → request 形狀測試紅。
- 停止條件：來函指名商品的驗證需要 seller 同步讀 catalog（違反依賴方向）而訂閱者回錯方案不成立。

---

## 已決事項（Will 2026-11-28，無待決問題）

1. 賣家自己下架（`ARCHIVED`）的商品頁是否仍對買家開放：**已由 D24 決定**（Will 2026-11-26，`backend/docs/plans/2026-11-24-b4-catalog.md` D24 列：ARCHIVED 公開可讀、不可購買），不再是問題。
2. 被下架商品的公開圖片：**選項 (b)**。措施建立或來函凍結時，擁有者經新的 media `CopyToSecure` 複製成 SECURE 證據副本並只保全副本；公開原圖照一般刪除流程處理。
3. admin 對話框欄位與文案（§4）：**同意**。商品專屬政策類別加在 `admin/src/lib/policy-category.ts`（例如食品標示不實、食安疑慮、禁售品、仿冒）；**確切清單在 B4-5e 開工前與 Will 確認**。
4. 治本方向：Will 2026-11-28「照治本的做法」；審查不再開第 3 輪，直接定稿。

---

## 審查紀錄

計數：第 1 輪 12 項（F1–F12）：修 9（F1、F2、F6、F7、F8、F9、F10、F11、F12）、記錄 2（F4、F5：移出問題，已有 SSOT）、拒絕 1（F3，證據見計畫 `backend/docs/plans/2026-09-24-go-modular-monolith.md:392`）。第 2 輪 4 項（F2、F7、F9 的未盡之處，加扣庫存路徑 S1）：修 4（本次定稿），其中 F2、F7、F9 是第 1 輪「已修」之處被第 2 輪再次指出不足，故於下表第 2 輪列重新處置。第 2 輪的嚴重度（blocking）本次輸入沒有審查者的標記，不填。Will 2026-11-28 決定定稿、不開第 3 輪。

### 第 2 輪（本次定稿修正）

| # | finding | 處置 | 證據 |
|---|---|---|---|
| F2（再） | 路徑盤點仍不完整：保全改成只放 SECURE 副本後，所有把資產或暫存上傳推進 `DELETING`、或實際刪物件的路徑都要重新盤點；`deleteUpload` 與 `MarkReclaimedUploadDeleting` 沒被涵蓋，暫存列可能留在 `DELETING` | 已修：§2 逐路徑表（7 條），保全條件放進認領 SQL（`ApplyDeletionRequests`、`ClaimAssetsOfUploads`、`MarkReclaimedUploadDeleting`）並在 `deleteUpload`、`deleteAsset` 刪物件前再檢查；不變量「被保全資產不離開原狀態、暫存列不留在 `DELETING`」；B4-5b 每條路徑一支 integration 與具體 mutation | `media/internal/reclaim/reclaim.go:190-226,296-364`、`media/db/queries/assets.sql:31-48`、`media/db/queries/uploads.sql:114-118`；第 7 項（`upload` 套件四處 `Store.Delete`）本輪未讀，列為 B4-5b 開工第一步 |
| F7（再） | B4-5a 只加 `PRODUCT` 列舉值，仍無法經真的 `CreateMeasure` 建出合法 `PRODUCT` 列 | 已修：`OWNER_HELD` 與 link 分支、`contentkind.Product` 全部移到 B4-5a；B4-5c 不再碰 seller 資料模型 | `seller/db/schema.sql:575-591`、`platform/contentkind/contentkind.go:8,18,23` |
| F9（再） | EXPLAIN 驗收只看 Seq Scan，沒擋住函式沒內聯 | 已修：通過條件加「沒有 `Function Scan` 節點」「走具名索引」 | §5 B4-5a |
| S1（新） | `DeductSkuStock` 的 EXPLAIN 驗收缺項，且可能被可見性函式拖成掃商品表 | 已修：`ListPublicProducts` 與 `DeductSkuStock` 各自 EXPLAIN（seed 10,000 件商品）；`DeductSkuStock` 必須維持主鍵更新路徑 | §5 B4-5a |

### 第 1 輪

| # | finding | 處置 | 證據 |
|---|---|---|---|
| F1 | §2／§5 `MarkDeleted` 呼叫端數字錯（寫 17 處 14 檔） | 修正為 19 行、16 檔；審查者的 15 檔差一，以重跑輸出為準 | `rg -n "\.MarkDeleted\(" internal cmd --glob '*.go' --glob '!*_test.go' --glob '!internal/modules/media/**'` → 19 行；`rg -l` 同條件 → 16 檔 |
| F2 | 漏了第二條進入 `DELETING` 的路 | §2 補 `reclaimUpload`→`ClaimAssetsOfUploads`；第 2 輪再擴充為完整路徑表 | `media/internal/reclaim/reclaim.go:208`、`media/db/queries/assets.sql:46-48`、`media/db/queries/uploads.sql:104-118` |
| F3 | 建議移除 B4-5c `SetProductStatus` 拒絕切 `ACTIVE` | 不採納，保留；寫明必要條件（擋住重新上架，避免賣家見 ACTIVE、買家看不到的誤導狀態），只拒轉換、不寫 `status`，D7 不變 | `backend/docs/plans/2026-09-24-go-modular-monolith.md:392` |
| F4 | 問題 1（快照引用方式）不該交 Will | 移出問題，改寫為由依賴規則決定，註明偏離 Will 原話「措施存快照 ID」 | `internal/arch/modules.go:19,22` |
| F5 | 問題 4（退場清空）已有 SSOT | 移出問題：措施來源跟 `ClearContentSnapshotsOfStores`，來函來源保留到 `hold_until` | `seller/db/queries/measures.sql`（`ClearContentSnapshotsOfStores`）、§3 |
| F6 | §1 內聯的推理錯 | 部分採納：改以 im／review 逐列 EXISTS 的前例與 enforced_filter migration 為據，真正風險改為外層函式包巢狀 `pub_*`。另查得「`pub_content_takedowns` 是 `$$` body」只對 sqlc 存根成立，資料庫實際定義是 `BEGIN ATOMIC` | `seller/db/pub.sql:40-42`（`$$` 存根）、`seller/db/objects.sql:294-300` 與 `migrations/20261121120000_seller_pub_content_takedowns_enforced_filter.sql`（`BEGIN ATOMIC`）、`im/db/queries/conversations.sql:66`、`review/db/queries/own.sql:33` |
| F7 | B4-5a 驗收靠暫時 fixture 或等 B4-5c | B4-5a 一併加 `PRODUCT`，用真措施列測；第 2 輪再擴充為完整資料模型 | §1、§5 B4-5a |
| F8 | B4-5b mutation 不具體 | 改為 `h.until > sqlc.arg(now)` → `h.until < sqlc.arg(now)`，列出預期變紅的測試名 | §2、§5 B4-5b |
| F9 | B4-5a `EXPLAIN` 驗收不具體 | 指名 `ListPublicProducts`、`DeductSkuStock`，seed 量，無 Seq Scan 並記索引名；第 2 輪再加 `Function Scan` 條件 | §5 B4-5a |
| F10 | §3 補「`CompleteAuthorityOrder` 不需事件」 | 查證不成立，改寫相反結論：完成會寫入新 `hold_until`，資料提供類完成時延長，因此延長時也發 `AuthorityOrderHoldExtended` | `seller/db/queries/authority_orders.sql:73-77`、`seller/internal/governance/authority.go:469-490` |
| F11 | 結帳缺口的歸屬 | 留在 B4-5a，標為既有缺陷修正，附修正前會失敗的回歸測試，PR 與計畫明寫 | §5 B4-5a、`catalog/internal/product/export.go:72` |
| F12 | `StoreRetired` 缺口只在簡報延後 | 排成明確項目：B4-6 一步，觸發條件為 B4-5b 完成；寫進 catalog `CARRYOVER.md` | §2、`backend/internal/modules/catalog/CARRYOVER.md` |
