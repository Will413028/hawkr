# B4 決策簡報：catalog 從零設計（業界基線 vs 現況）

快照 dev `496b048e2`。證據標記：【量】＝讀過 source 或跑過指令；【推】＝推論；【文件】＝公開文件的已知做法，沒在本 repo 實測。盤點見同目錄 `interface-inventory.md`、`client-inventory.md`、`defects.md`。

## 前提題：被改的程式是否排定要重寫、搬走或刪除？

- Java product-service 是重寫對象：資料模型、RPC 形狀、Redis 快取、MQ 拓撲都是 SELF-IMPOSED（自己加的限制），不沿用。
- Java order-service 與 cart-service 排在 B8、B6 改寫，B4 期間仍經 gRPC 呼叫 product（cart 2 支、order 3 支，`client-inventory.md` §5【量】）。這是 REAL（真正的限制）：B4 需要一層暫時的轉接，B6／B8 刪除。
- gateway 的代行商品路由（7 條，`client-inventory.md` §2【量】）直接打 product-service gRPC，A10 才改寫。刪 Java product 時它們會斷，見 D9。
- 依賴表已定 `catalog → {seller, buyer, media, fulfillment}`（`internal/arch/modules.go:22`【量】）；fulfillment 還不在單體（B3 並行中），見 D10。

## 限制帳

| 限制 | 分類 | 來源 |
|---|---|---|
| B6／B8 之前 cart、order 在 Java，跨進程扣庫存、讀商品 | REAL | 主計畫 B6、B8 未完成【量】 |
| 代行商品路由在 gateway，A10 才改寫 | REAL（順序可調） | 主計畫 A10【量】 |
| 系統沒上線，不搬資料、不受相容性約束 | 有利條件 | 根 `AGENTS.md` |
| 依賴方向：buyer、seller 在 catalog 下層；review、cart、ordering 在上層 | 與基線一致 | `modules.go` |
| 溫層、產銷履歷、有機標章、商品認證不帶 | 已決（範圍裁切 ADR） | 主計畫 B4 段【量】 |
| Java 的 TSID＋BIGSERIAL 混用、±1 計數器、Redis 快取 | SELF-IMPOSED，不沿用 | `defects.md` CD3、CD10 |

## 業界基線

- **商品＝SPU＋SKU**，價格與庫存在 SKU；上架狀態在商品，SKU 可個別停售（Shopify product／variant、蝦皮 item／model【文件】）。
- **庫存扣減以帳本為準**：`stock_movements` 以呼叫方的冪等鍵 UNIQUE，先插帳本列（撞鍵即回上次結果）、再 `UPDATE … SET qty = qty - n WHERE qty >= n`，同一 transaction；歸還以原扣減列為依據，不信任呼叫方帶的數量（Shopify inventory adjustments、Stripe idempotency key【文件】）。單體內結帳與扣庫存同 transaction（B8）。
- **衍生計數**（銷量、收藏數、評分）由事實擁有者在同一 transaction 更新，或以集合重算；不用無序的 ±1 事件。
- **搜尋**：資料量小時用 Postgres（`tsvector`＋`pg_trgm`）；量測觸發才上搜尋引擎。
- **ID**：同一聚合用同一方案；對外不透明字串。

## 決定題

每題：選項、取捨、建議、擋住哪些步驟。

### D1 Java cart／order 的過渡呼叫怎麼接（擋 B4-4）
- A：`internal/legacy/catalogserver` 實作現有 product.proto 的 5 支內部 RPC（扣、還庫存、結帳批次讀、加購物車查詢），加 `legacy/catalogconsumer` 收 order 的 `order.cancelled`、`order.sales.accumulated`，都轉呼叫 catalog 模組的公開 API；B6／B8 刪除。同 `legacy/couponserver`、`couponconsumer` 前例。（建議）
- B：B4 等 B6／B8 一起做——把 B4 拖到 ordering 後面，且 B8 前置本來就含 B4，形成循環。
- 結論：未決。

### D2 庫存模型（擋 B4-2；處理 CD1、CD2）
- A：帳本 `catalog.stock_movements`（冪等鍵 UNIQUE、記品項與數量），先插帳本再條件扣減，同 transaction；歸還只依原扣減列的 payload，呼叫方只帶冪等鍵（部分歸還帶品項，但不得超過原扣減）。（建議，業界基線）
- B：照 Java「先查再扣、最後記」——CD1 雙扣仍在。
- 結論：未決。

### D3 銷量、收藏數、評分怎麼更新（擋 B4-3；處理 CD3、CD4、CD5）
- A：buyer 在下層，catalog 以 in-process 事件在 buyer 的收藏 transaction 內同步更新收藏數（同 transaction，不會亂序）；review 在上層，呼叫 catalog 的 `SetRating(product, avg, count)`，傳 review 端重算的絕對值；銷量與取消歸還在 B8 前由 `legacy/catalogconsumer` 收 order 的 MQ 事件（銷量以 `order_no`、歸還以 `stock_tx_id` 冪等），B8 改成結帳／完成的同 transaction 呼叫。去重表（`sales_count_event_dedup`）不再需要。（建議）
- B：照 Java 的 MQ ±1 與去重表——CD3、CD4、CD5 都留著。
- 結論：未決。

### D4 對外 API 形狀（擋 B4-3）
- A：Connect `CatalogAppService`（公開瀏覽＋買家）、`SellerCatalogAppService`（賣家商品、庫存、分類選擇）、`CatalogAdminService`（分類管理、強制下架→D7 的措施）；店家由呼叫者推導，不收 `store_id`（同時消除 CD9）。依〈API 設計規範（Connect）〉。（建議，與 B7 一致）
- B：一支 service 涵蓋三種角色——權限宣告混在一起。
- 結論：未決；proto 草稿在 B4-3 開工前交審。

### D5 沒有 client 呼叫的路由與 RPC（擋 B4-1 的去向欄）
- 清單見 `client-inventory.md` §6.5【量】：認證 11 條（本來就裁切）、related、rating、`/public/products/{id}/skus`、產地與城市聚合、`DELETE /admin/products/cache`、`batch-change-category`、admin 分類的 create／delete／restore／`{id}/products`、`/app/hashtags`；Java 端 `GetStock` 與 order 的 `getStock`／`getPublicProduct`（CD8）。
- A：不改寫，隨 Java 與 gateway 一起刪；之後有畫面需要再依新契約加。（建議）
- B：照搬成 Connect——為沒有消費者的端點寫契約與測試。
- 例外待你確認：admin 分類的建立／刪除／還原沒有畫面，但分類樹沒有其他寫入入口。建議 B4 只帶種子資料與 admin 的審核狀態變更（見 D8），新增分類延後到有畫面時。
- 結論：未決。

### D6 ID 方案（擋 B4-2；處理 CD10）
- A：商品、SKU、分類、hashtag 一律 UUIDv7（單體其他模組現行做法），對外不透明字串；content banner 的商品動作同時換成 UUID（主計畫 B4 段）。（建議）
- B：沿用 TSID＋BIGSERIAL。
- 結論：未決。

### D7 商品限制措施（擋 B4-5；主計畫 B4 段已決方向，Will 2026-10-18）
- 內容已決：seller 的 `measure_contents.content_kind` 加 PRODUCT，有理由、可申覆、擋重新上架；admin 強制下架對話框改建措施；主管機關來函指名的商品頁資料依保存期限保存。
- 要拍的只是放法：A：B4 的獨立子步驟（B4-5），與商品寫入同一波上線，「第一次部署前一定要有」照主計畫。（建議）B：另開計畫。
- 結論：未決。

### D8 分類（擋 B4-2）
- 現況：分類樹在 product-service，admin 有審核頁；`admin/src/app/admin/category-audit/page.tsx:70` 送 `PATCH /admin/categories`（不帶 id），gateway 只有 `PATCH /categories/:id`，推定 404（`client-inventory.md` 意外 1【推】，列 CD11）。
- A：分類歸 catalog，B4 帶分類樹、賣家選分類、admin 審核狀態；審核頁改用新 Connect（批次狀態變更），CD11 隨之消失。（建議）
- B：分類另立模組——依賴表沒有這個模組，要改架構。
- 結論：未決。

### D9 代行商品路由怎麼辦（擋 B4-6 刪除 Java）
- A：B4 只改 gateway 的代行商品 handler 轉呼叫單體（gateway→單體 Connect 或 legacy 轉接），A10 再改寫成 delegation 模組——多寫一次轉接，A10 丟掉。
- B：B4 的刪除步驟（B4-6）被 A10 的商品部分擋住；B4-1 到 B4-5 照做，Java product 留到 A10 商品代行完成——Java 多活一段，但不寫丟棄的轉接。
- C：代行商品路由併進 B4，直接在 delegation 模組實作（依賴表已允許 delegation→catalog）。範圍加 7 條路由與代行稽核，A10 只剩訂單。（建議：不寫丟棄程式，且 A7-2d 已有代行 Connect 第一版形狀可沿用）
- 結論：未決。

### D10 catalog→fulfillment 的依賴（擋 B4-2 的配送方式欄位）
- 依賴表裡 catalog 依賴 fulfillment，推測是 SKU 的可用配送方式與運費試算【推】。fulfillment 由 B3 並行改寫，本 session 不碰。
- A：B4 的商品只存「賣家啟用的配送方式代碼」，驗證與運費試算留在 fulfillment（B3）；`CalculateShippingOptions` 不搬進 catalog，B3 擁有。catalog 暫時不 import fulfillment。（建議）
- B：B4 等 B3 完成再做配送欄位。
- 結論：未決；要和 B3 session 對齊 `CalculateShippingOptions` 的歸屬。

### D11 shipping_ambient.go 的刪除範圍（擋 B4-6）
- 呼叫點 7 個：product 4 個（`seller_product.go:485`、`:613`、`admin_delegated_product.go:1100`、`shipping_options.go:101`），shipping-service 3 個（`shipping.go:501`、`:598`、`:750`）（`client-inventory.md` §1b【量】）。你指定整檔歸 B4。
- 問題：刪掉 shipping handler 的常溫過濾後，Java shipping-service 若仍回冷藏／冷凍列，client 會看到。
- A：B4 刪整檔；同一變更讓 Java shipping-service 不再產生非常溫列（刪種子／過濾在來源），以特性測試證明配送方式清單只剩常溫。（建議）
- B：B4 只刪 product 的 4 個呼叫點，shipping 的 3 個留給 B3——違反你給的歸屬。
- 協調：shipping-service 與 gateway `shipping.go` 是 B3 正在改寫的範圍。拍板後由本 session 把 D11 的結論寫給 B3 session（或寫進 B3 計畫的交界段），B4-6 動 shipping 部分前確認 B3 沒有同一檔的未合併變更；若 B3 已先刪 Java shipping，shipping 的 3 個呼叫點隨 B3 消失，B4 只刪剩下的。
- 結論：未決。

### D12 ai-service 的 product 呼叫（擋 B4-1、B4-4、B4-6）
- ai-service 呼叫 ProductService 的 `GetProduct`、`GetProductList`、`GetStock`（`backend/services/ai-service/internal/client/product/adapter.go`）與 CategoryService 的 `GetCategoryTree`（`client/category/adapter.go`）【量】；A9 進行中，完成時刪 ai-service。
- A：B4-6 被 A9 的「刪 ai-service」擋住；A9 搬進單體的模組若先完成，經 `legacy/productservice` port 用 product，B4-4 一起改接 catalog。不為 ai-service 寫轉接。（建議）
- B：`catalogserver` 加這 4 支（含 `GetStock`）——寫一份 A9 完成就丟的轉接。
- 結論：未決。

### D13 分類資料來源（擋 B4-2）
- admin 分類的建立／刪除／還原沒有畫面（D5），分類樹沒有其他寫入入口。
- A：分類樹以 migration 種子資料建立；admin 只有審核狀態變更；新增分類延後到有分類管理畫面。（建議）
- B：照搬 admin 分類 CRUD 成 Connect，沒有畫面。
- 結論：未決。

## 既有缺陷的建議處置

| 缺陷 | 建議處置 |
|---|---|
| CD1 扣庫存雙扣 | 修（D2），回歸測試：同一冪等鍵併發兩次只扣一次 |
| CD2 歸還量信任呼叫方 | 修（D2），回歸測試：帶超過原扣減的數量被拒 |
| CD3 收藏數漂移 | 修（D3） |
| CD4 評分亂序 | 修（D3），傳絕對值 |
| CD5 去重與帳本無清理 | 去重表隨 D3 消失；帳本保留（金流稽核等級，量小），接受 |
| CD6 運費試算郵遞區號 | 歸 B3（D10），本階段不搬 |
| CD7 認證事件無消費者 | 隨裁切刪除 |
| CD8 order 死方法 | 隨 B8 刪；`catalogserver` 不實作 |
| CD9 `GetProduct` 擁有者判斷 | 修（D4：店家由呼叫者推導） |
| CD10 ID 混用 | 修（D6） |
| CD11 admin 分類審核 404 | 修（D8） |

## 拍板順序

D1、D2、D5、D6、D9、D10、D11、D12、D13 互不相依，可一次拍；D3 依 D1（銷量過渡）；D4、D8 依 D5（哪些端點進契約）；D7 只是放法。D10 需要和 B3 session 對齊後再定案。
