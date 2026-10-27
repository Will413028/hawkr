# 改成一般電商：刪除生鮮專屬功能

**Goal：** 依 ADR `docs/adr/2026-10-27-general-ecommerce-scope-cut.md`，repo 裡不再有 pricebook 行情、食譜、產地地圖、首頁季節裝飾、企業福利頁、食品業者登錄、FDA 轉報、Uber 配送的 client 與 gateway 程式；採購助理照常運作但不依賴 pricebook；client 不再顯示配送溫層、產銷履歷與商品認證。終態以步驟 8 的驗收判定（`inventory.sh` 每個單元只剩「延後」列出的命中）。

**上層：** `docs/plans/2026-09-24-go-modular-monolith.md`（全域約束、模組完成的定義）。主清單見本機個人脈絡檔。

**附件：** `2026-10-27-ecommerce-scope-cut/`：`inventory.sh`（每個單元的引用盤點；計數的唯一來源；排除文件、計畫目錄、migrate 測試的原樣樣本與 `*.tsv`）。快照 commit：`a6f9a76f7c`。

**「歸 0」的判定：** 每一步的驗收寫「`inventory.sh <單元>` 只剩預期命中」。預期命中是步驟內「預期剩下」列出的檔案，加上開工時在進度表逐檔登記、並寫明理由的命中（只能是與該功能無關的同字詞，例如註解裡的英文 recipe、CodeMeta 生成物裡仍被 Java 用到的值域）。沒登記的命中就是沒刪乾淨。

## 前提（開工時核對過，2026-10-27）

- 會改到或取代的既有部署是否在線：不在線。系統從未上線（根目錄 `AGENTS.md`），不受相容性窗口、expand-contract、App 舊版相容約束；每一步的回滾就是 revert。
- Connect 契約都是 `v1alpha1`，`buf breaking` 設 `ignore_unstable_packages: true`（`api/buf.yaml`），刪 RPC、欄位、enum 值不會被擋（已確認，讀檔）。
- 已套用的 migration 不得修改或刪除（`internal/platform/migrate`）；刪表一律新增一個 `DROP` 的 migration。壓成 baseline 是主計畫階段 C 的事（已確認，主計畫）。
- CodeMeta 凍結：不新增值域、不手改生成檔（根目錄 `AGENTS.md`）。只服務已刪功能的 CodeMeta 值域，改 `docs/代碼管理（CodeMeta）/` 的定義後跑 `generate.sh --all`，Java 生成物要手動 `git rm`（本機個人脈絡檔；預設（可推翻）：刪；理由是值域的唯一消費者已刪）。
- 作品集轉 public 計畫（`docs/plans/2026-10-24-portfolio-public.md`）同時在進行，兩邊都改 website 與 App。兩個計畫的步驟都在 main 上，開工前 `git log --oneline -5` 確認沒有另一個 session 正在改同一個目錄（Will 2026-10-25：其他工作照常進 main）。
- 本機驗證照本機個人脈絡檔：e2e、Playwright、storybook、`characterize.sh` 不在本機跑，標「待 CI」；Flutter 開發中只跑 analyze、custom_lint、改到的功能的測試與讀原始碼的測試，步驟結束前兩支 App 各跑一次完整測試與 `scripts/deadcode.sh`。
- 採購助理的品項解析在拿掉 pricebook 後只剩分類樹召回（`taxonomyRetriever`）與 LLM 召回（`llmRetriever`），兩者都已存在（`services/ai-service/internal/procurement/cropidentity/README.md`「解析路徑」；已確認，讀檔）。分類樹召回能不能離線（不打 LLM、不打真實服務）對 golden set 跑：未確認，步驟 5 開工時查。
- 解析品質的線上評測 `cmd/resolver-eval` 要打 gateway、要 `JWT_SECRET` 與 LLM（`cmd/resolver-eval/main.go:183,206`），碰憑證，本計畫不跑；品質判斷改用步驟 5 的離線量測（已確認，讀檔）。
- 賣家中心的作物品項搜尋（`CropCatalogSearch`，gateway `PricebookCropCatalogSearch` → `SearchCropCatalog`）只在 insights 用，不在賣家上架表單（`rg -l useCategoryResolution website` 只命中 insights 與 hooks；已確認）。所以刪 pricebook 不改上架流程。
- Java 的配送方式聚合會略過溫層空白的配送方式（`product-service/.../ProductAggregateRules.java:72`），所以 D5 的過渡期 client 要送固定的常溫值，不能不送（已確認，讀檔）。常溫的代碼值在步驟 7 開工時查 `ShippingTemperature`。

**驗收基線：** 用作品集轉 public 計畫進度表步驟 3 的結果：與基線相同的既有失敗是 `backend-monolith / lint`（22／35 項）、`pnpm audit`（16 個）、pricebook integration 兩項（日期寫死在 2026-08；隨步驟 6 刪除）。每一步開工時先在 base 跑該步的驗收清單一次，新的失敗記進進度表，不在本計畫修。

## 本階段依賴的不變量

| 不變量 | 依賴它的機制 |
|---|---|
| 模組的 import 必須在依賴表內 | `internal/arch/modules.go`（content、seller、agent 的列；刪功能後多出的依賴要從表移除） |
| `schema.sql` 與 migration 套用結果一致 | `TestSchemaFileMatchesMigrations`；每個刪表的步驟同時改 `db/schema.sql`、新增 migration、`atlas migrate hash` |
| 生成物與來源一致 | `make connect`、`pnpm --filter @hawkr/api gen:api`、`packages/hawkr_api` 的 `generate.sh`；`make uitext`；`make sqlc`；兩支 App 的 build_runner；CI 的 drift 檢查 |
| 目錄 enum 的使用者面文字只在共用文案目錄 | `backend/internal/platform/uitext/catalog/*.json`、`check-enum-text`、`check_enum_text`；刪 enum 成員時一起刪 key |
| 現有 API 清單只准縮小 | `backend/api/openapi/openapi.yaml`、`gateway/internal/router/openapi_coverage_test.go`；刪 gateway 路由時刪對應的 operation |
| 關店與個資清理的鎖順序 | `seller/internal/purge`、`seller/internal/governance/authority.go`（與 `ExpiredFoodRegistrations` 鎖同一組店家列）；步驟 4 刪食品業者登錄後，鎖順序的說明與證明要照剩下的參與者重寫 |
| ai-service 的 schema 與 migration 一致 | `services/ai-service/db/schema/procurement.sql`、`db/migrations`；步驟 5 改 `procurement_price_previews` 時兩邊同時改 |
| 每個模組的沿用盤點與 AGENTS.md 只寫現行 | `CARRYOVER.md`、`AGENTS.md`（seller、content、agent） |

## 規模

計數來源：`bash backend/docs/plans/2026-10-27-ecommerce-scope-cut/inventory.sh <單元>`（快照 `a6f9a76f7c`，引用檔數，不是行數）。

| 單元 | 主要位置（檔數） | 步驟 |
|---|---|---|
| welfare、seasonal、origin | website 共約 35 | 1 |
| recipe | backend 模組與 httpapi 33、website 約 75、buyer-app 15、admin 7、生成物 12 | 2 |
| uber（client 與 gateway） | seller-app 31、buyer-app 22、website 28、gateway 12；Java 122 延後 | 3 |
| foodreg、fda | backend 模組 93、httpapi 43、admin 30、website 26、seller-app 21、生成物與文案目錄約 30 | 4 |
| 採購的 pricebook 依賴 | ai-service `internal/procurement` 75 檔（`rg -l -i pricebook services/ai-service/internal/procurement`）、website procurement 約 20 | 5 |
| pricebook | pricebook-service 196、ai-service 其餘、website 84、gateway 30、pkg 13、CI 與 infra 約 15 | 6 |
| clientfields | website 約 130、seller-app 52、buyer-app 40、admin 24；Java 165 延後 | 7 |

## 範圍

- 做：ADR 的 D1–D5 中 client、gateway、Go 程式、契約、文案目錄、CI 與指令檔的部分。
- 不做：改公司時期的設計書（作品集轉 public 計畫決定文件的去留；只改 Will 在 2026-09 之後寫的 ADR、計畫、`AGENTS.md`、`CARRYOVER.md`）。不重新設計採購的流程，只拿掉價格依據。
- 延後：
  - Java shipping 的 Uber 程式、payuni-webhook-service 的 Uber webhook，連同只服務它們的 proto RPC 與生成物、CodeMeta 的 Uber 值域（`ShippingMethod` 3003、`UberDeliveryOutcome`、`ShipmentAnomalyType` 的 4 個 UBER 值、錯誤碼 9024）、`.env.template` 與 `docker-compose-local.yml` 的 Uber 設定、`docs/runbooks/uber-quote-retention/`，以及 4 份 Uber ADR 標 superseded（觸發：主計畫 B3 fulfillment 改寫）。
  - Java 的配送溫層、產銷履歷、商品認證欄位與 client 送的常溫值（觸發：B4 catalog、B3 fulfillment、B6 cart 改寫；B4 完成時 client 的常溫值一起刪）。
  - CodeMeta 裡同時被 Java 用到、無法在本計畫刪的值域（觸發：CodeMeta 隨最後一個 Java 消費者退場）。

## 步驟

- [x] **1. 純前端：企業福利頁、首頁季節與水果裝飾、產地地圖**（被擋於：無）
  - 範圍：刪 `website/features/corporate-welfare`、路由與導覽入口、SEO 與 sitemap 列、`website/scripts` 裡的登記；`HomeSeasonalBanner`、`HomeSeasonalAtmosphere`、`FruitBurst`、`seasonal-gradient-waves-renderer.ts` 與 `website/scripts` 裡的登記；`website/features/market/origins`、`OriginPicker`、`OriginSellerPreview`、`LocationPickerModal`、`TaiwanMap`、`lib/origin`、市集篩選裡的產地條件；buyer-app 分類頁的 `taiwan_origin_picker_sheet.dart` 與它在 `category_screen.dart` 的入口（同一個功能）。市集的商品列表保留。
  - 消費端：`bash …/inventory.sh welfare`、`… seasonal`、`… origin`。
  - 預期剩下：無。
  - 不能動：市集商品列表與篩選的其他條件；首頁其他區塊的版面。
  - 驗收：三個單元的 `inventory.sh` 只剩預期命中；buyer-app analyze、custom_lint、分類頁的測試、deadcode、build_runner 生成比對；website lint、`test:unit`、vitest `--changed`、build（帶 `API_BASE_URL=http://127.0.0.1:9/api/v1` 與同值的 `NEXT_PUBLIC_API_BASE_URL`）、`test:build-output`；`pnpm design:check --apps-root ..`；mutation：在首頁 import 回已刪的元件，tsc 失敗。e2e 與視覺回歸待 CI。
  - 停止條件：刪產地條件會改到市集 API 的請求參數，而後端仍要求它。

- [x] **2. 食譜**（被擋於：無）
  - 範圍：content 模組 `internal/recipe`、store 查詢、公開 API 與 Connect handler；`api/hawkr/content/v1alpha1/content.proto`、`api/hawkr/admin/content/v1alpha1/content.proto` 的食譜 RPC 與 message；三個生成端；新 migration 刪食譜的表，`schema.sql` 同步；media 的食譜引用類型（`media/ref_type.go`、`finalize_args.go`）與 `internal/legacy/mediaservice` 的對應；`platform/adminperm/adminperm.go` 的食譜權限；`platform/revalidate` 與 `pkg/revalidate` 的食譜快取標籤；admin `recipes`；website 的 `RecipeRail`、`lib/recipes` 與食譜頁；buyer-app 的食譜；gateway 若還有食譜 REST 就刪路由與 OpenAPI operation；content 的 `AGENTS.md`、`CARRYOVER.md`。
  - 消費端：`bash …/inventory.sh recipe`（`rg -n -i recipe backend/services/pricebook-service backend/services/ai-service backend/services/java` 的 3 個命中逐一判斷）。
  - 預期剩下：CodeMeta `AssetReferenceType` 的食譜值與它在 Java media-service 的使用（Java 仍在用，延後到 B1 media）；生成物中由它衍生的檔。
  - 不能動：content 的 banner、app version、圖片。
  - 驗收：`inventory.sh recipe` 只剩預期命中；backend `make connect`、`make sqlc`、gofmt、vet、`go test ./...`、content 的 integration、`atlas migrate validate`、arch 測試；`buf lint`；TS 與 Dart 生成物 drift；admin lint、test、build；website 同步驟 1；buyer-app analyze、custom_lint、改到的測試、deadcode、build_runner 生成比對；mutation：在 `schema.sql` 加回食譜表，`TestSchemaFileMatchesMigrations` 失敗。
  - 停止條件：食譜表被 content 以外的模組或 job 讀取。

- [x] **3. Uber 配送：client 與 gateway**（被擋於：無）
  - 範圍：seller-app `features/uber_delivery`；buyer-app、website、admin 的 Uber 配送顯示與操作；gateway 的 Uber 路由、handler、OpenAPI operation 與 coverage 登記；`pkg` 裡只給 gateway 用的 Uber 型別；`.env.template`、`docker-compose-local.yml` 裡只給 gateway 的 Uber 設定。Java shipping、payuni-webhook-service 的 Uber 不動（延後），所以只服務 Uber 的 ADR（`2026-09-11-uber-commerce-model-seller-direct`、`2026-09-11-uber-logistics-fee-charge-trigger`、`2026-09-14-uber-create-idempotency-precommit-key`、`2026-09-06-uber-webhook-slice-in-payuni-webhook-service`）在 B3 刪 Java 時才標 superseded（寫進延後）。
  - 預期剩下：`backend/services/java` 全部；`pkg` 與 `backend/api` 裡 Java 仍用到的 proto 與 CodeMeta。
  - 消費端：`bash …/inventory.sh uber`（Java 122 不在本步範圍）；`rg -n -i uber backend/gateway/internal/router`。
  - 不能動：其他配送方式的訂單顯示；Java 服務的介面（gateway 只是不再呼叫）。
  - 驗收：`inventory.sh uber` 只剩預期命中；gateway 的 gofmt、vet、test、`openapi_coverage_test`；兩支 App 與 website、admin 的驗收同步驟 1、2；mutation：gateway 加回一條 Uber 路由而不加 operation，coverage 測試失敗。
  - 停止條件：訂單頁的資料模型把 Uber 當成唯一的即時配送狀態來源，刪掉會讓其他配送方式顯示錯誤。

- [x] **4. 食品業者登錄與 FDA 轉報**（被擋於：無）
  - 範圍：seller `internal/foodreg`、`model` 裡的型別與代碼、`stint.AttachFood` 與 `Store.ApprovedFoodRegistration`、purge 的食品登錄清理與 2 年刪除、closing 的退場處理、admin 店家動作的食品登錄前置條件；governance 的 `case_referrals`、`model.FDAReferral`、`ReviewCaseFDAReferralByIDs`、`seller.review_case_fda_referral` 函式；新 migration 刪 `food_registrations`、`case_referrals` 與函式，`schema.sql`、`objects.sql` 同步；`api/hawkr/seller`、`api/hawkr/admin/seller`（`seller.proto`、`governance.proto`）與 `admin/reviewqueue` 的相關 RPC、欄位、enum 值；三個生成端；`uitext` 目錄的 key 與 `make uitext`；reviewqueue 的食品登錄來源；admin `food-registrations` 與治理頁的 FDA 轉報；website 賣家入駐與賣家中心的食品登錄、店面的公開字號；seller-app 的食品登錄；`internal/legacy` 與 `services/seller-service` 的殘留引用；seller 的 `AGENTS.md`（含鎖順序段）、`CARRYOVER.md`；三份 ADR（`2026-10-18-seller-governance-consolidated`、`2026-10-19-seller-kyc-payuni-consolidated`、`2026-10-20-admin-review-queue`）加修訂段指向本計畫的 ADR。
  - 消費端：`bash …/inventory.sh foodreg`、`… fda`；`rg -n "ExpiredFoodRegistrations|AttachFood|ApprovedFoodRegistration|FDAReferral" backend`。
  - 不能動：通報、案件、處分、申訴、主管機關來函保存（`authority.go`）、KYC、PayUni；關店與個資清理的其他參與者與它們的時限。
  - 預期剩下：`services/seller-service`、`services/java` 裡的命中（舊服務隨 A3 收尾與階段 B 刪除；開工時逐檔登記，有執行期呼叫就停）；CodeMeta 中仍被 Java 用到的值域。
  - 驗收：兩個單元的 `inventory.sh` 只剩預期命中；backend 同步驟 2，加 seller 的全部 integration（負載高時 `-p 2`）；`make uitext` 無 diff、`check-enum-text`、`check_enum_text`、`scripts/check-enum-text-cross.sh`；admin、website、seller-app 同步驟 1、2；mutation：在 `schema.sql` 加回 `food_registrations`，schema 測試失敗。鎖順序：開工時先找出守住「主管機關來函登記先鎖帳號店家列」的測試（`authority.go:331` 的登記；候選 `termination_lifecycle_integration_test.go`、`cases_concurrency_integration_test.go`），寫進進度表；沒有就先補一支，再刪食品登錄；mutation 是把登記的鎖店家列移到讀來函之後，該測試要失敗。
  - 停止條件：主管機關來函保存的保存條件依賴食品登錄的資料（不只是共用鎖）。

- [ ] **5. 採購助理拿掉 pricebook 依賴**（被擋於：無；必須在步驟 6 之前）
  - 開工先做（2026-10-28 已做，結果記進進度表）：分類樹召回的程式可離線（不打 LLM），但 repo 與本機沒有完整的分類樹（`internal/cache/testdata/category_tree_snapshot.json` 沒有父子關係；完整樹只在舊公司資料庫），pricebook 也沒有本機資料，所以量不到改動前的基線（決定 D6）。`rg -l -i pricebook services/ai-service` 的 89 檔去向由開工盤點列出，實作時照它。
  - 範圍（ai-service）：`cropidentity` 拿掉 `recall.EntityRetriever`（`SearchPriceableEntities`）與 `usableOf`（取不到價就丟掉），召回只剩分類樹與 LLM；`ListGroupCropOptions`、`ResolveCategoryMapping` 的使用改成分類樹；`pricing/`、`domain/quote_pricing.go`、`domain/pricing_compile.go` 的市場行情依據；`domain/deal_temperature.go` 與價格冷熱；`commerce/` 送出詢價時附的行情；`goldenset` 與 eval 的期望值；`testkit/pricebook.go`；`usecase/pricebook_category_mapping.go`；`grpc/procurement_handler.go` 與 `price_preview_proto_test.go` 的行情欄位；`agent/di.go`、`app/app.go` 裡只為採購接的 pricebook；`procurement_price_previews`：`pricing_mode` 只剩 `BUYER_ENTERED_NO_REFERENCE`，以新 migration 刪掉 `pricing_mode`（只剩一個值就不需要欄位）、`quality_knowledge_state`、`reference_availability`、`reference_facts_hash`、`reference_payload_json`、`unavailable_reason`、`deal_temperature_json`、`market_price_basis_json` 與對應 CHECK，`db/schema/procurement.sql` 同步（依 D2「報價只看供應商報的價」與根目錄 `AGENTS.md`「不留技術債」：上線前沒有資料，不留永遠是 NULL 的欄位；開工時逐欄確認它只承載行情，承載別的就停）。`internal/client/pricebook` 只剩助理工具在用（步驟 6 刪）。gateway 的 RFQ 回應裡的行情欄位與 OpenAPI。website `procurement-assistant` 的行情面板（`RfqDraftFlow` 的 market panel、`rfq-price-contract*`、`QuoteBatchSummary` 的行情比較）、`procurement-rfq-demo`、`procurement-rfq-prototype` 的行情資料；seller-app `features/procurement` 的行情顯示；CodeMeta 的 `ProcurementDealTemperatureReason`；ADR `2026-09-18-deal-temperature-outcome-definition` 標 superseded。另依 D7：`quality_knowledge_state` 保留（承載買家是否知道品質，不是行情）；`procurement_confirmed_rfq_snapshots` 與 `procurement_rfq_publication_approvals` 的 `pricing_mode`、`reference_availability`、`deal_temperature_status` 與 `publication_execute.go` 依它們擋發布的檢查一起刪；`domain/item_resolution.go` 的 `PricebookQueryCodes`、`PricebookCommodityCode` 與 `ReferencePriceRange` 帶到賣家內容的部分隨行情刪。新增離線評測：以 `backend/infra/seed-data/04-product.sql` 的分類樹（33 節點）做成測試替身，pricebook 用一呼叫就失敗的替身、模型設 nil，逐筆對 golden set 呼叫 `Resolve` 並輸出 `goldenset.Evaluate`；`golden_queries.yaml` 依新行為重寫成這棵樹涵蓋的品項與分類期望值。
  - 消費端：`rg -l -i 'pricebook|deal_?temperature|DealTemperature|market_?basis|observed_?price' backend/services/ai-service backend/gateway website/features/procurement* website/features/vendor-portal seller-app/lib/features/procurement website/tests`。
  - 不能動：採購的對話流程、詢價建立與送出、供應商 portal、報價收集與「接受報價後建單」；agent 模組。
  - 預期剩下：ai-service 的 `internal/client/pricebook`、`internal/agent/tools.go`、`internal/usecase/pricebook_client.go`、`internal/config`（含 `strict_env_test.go`）、`agent/di.go` 與 `app/app.go` 裡給助理工具的接線（步驟 6 刪）；已套用的 ai-service migration 檔。
  - 驗收：消費端指令只剩預期命中；離線評測逐筆結果與命中率貼進進度表（沒有改動前基線，D6）；ai-service gofmt、vet、`go test ./...`、integration；`cropidentity` 的 `resolver_v2_test.go`、`facet_test.go` 改成新行為後通過；website、seller-app 同步驟 1、2；mutation：把 `EntityRetriever` 接回召回，新寫的「解析不呼叫 pricebook」測試失敗（測試用一個呼叫就失敗的 pricebook 替身）。e2e（`procurement-*`、`external-vendor-portal`）待 CI，行情相關的 spec 刪除。
  - 停止條件：離線評測上，解析不出任何分類的查詢超過全部的三分之一，表示分類樹不夠當品項來源，回報 Will 再決定；開工盤點發現要刪的欄位承載行情以外的資料（`quality_knowledge_state` 已依 D7 保留）。
  - 需要人做的事：要跑線上 `resolver-eval` 時，Will 提供環境與憑證（預設不跑）。

- [ ] **6. 刪 pricebook**（被擋於：步驟 5）
  - 範圍：`services/pricebook-service` 整個目錄；`docs/api-fixtures/pricebook-*`；notification、invoice、crm 三個服務各自的 pricebook 命中（開工時逐檔判斷：測試替身或 DI 裡的殘留就刪，執行期呼叫就停）；`api/proto/pricebook`、`pkg/proto/pricebook`、Java `shared/java-proto` 的 `pricebook.proto` 與 `ai.proto` 裡引用它的部分；gateway 的 pricebook 路由、gRPC client、OpenAPI operation（含賣家上架用的作物品項 picker `SearchCropCatalog`）；ai-service 的 `internal/client/pricebook`、`usecase/pricebook_client.go`、助理工具 `get_seasonal_price_suggestion`、`recommend_seasonal_fruits`、設定；website `seller-center/insights` 的行情雷達、價格表、價格地圖、`lib/pricebook`、`useMarket*`、`useForecastCurve`、`useObservedPriceHistory`、`useCategoryPriceRollup`、`useSpatialSurface`、`useCategoryResolution`、`CropCatalogSearch`；seller-app、buyer-app、admin 的命中；`go.work`、`Makefile`、`dev.sh`、compose、`.env.template`、`infra`、`monitoring`、`.github/workflows`（`backend-integration-tests` 的 pricebook job）、`internal/legacy`、`internal/arch`、`cmd` 的引用；CodeMeta 的 `Pricebook*`、`FreshnessLabel`、`PredictionStatus` 值域；主計畫的 A4 改成「取消，見本計畫」；ADR `2026-09-14-pricebook-freshness-describes-age-not-usability`、`2026-09-13-procurement-category-fallback`（若只描述 pricebook 對應）標 superseded，其他提到 pricebook 的 ADR 加一行修訂。
  - 消費端：`bash …/inventory.sh pricebook`。
  - 不能動：賣家中心 insights 裡不靠 pricebook 的區塊（商品瀏覽數等）；分類樹。
  - 預期剩下：已套用的 migration 檔裡的字樣（不能改）；CodeMeta 中仍被 Java 用到的值域。
  - 驗收：`inventory.sh pricebook` 只剩預期命中；`go work` 的每個成員 gofmt、vet、test、`GOWORK=off go mod tidy` 無 diff；gateway 同步驟 3；`proto-generated-diff`；Java `mvn -pl <受影響模組> -am` 的編譯與測試（temurin-21）；CodeMeta `generate.sh --all` 無 drift；website、seller-app、buyer-app、admin 同步驟 1、2；`legacy-endpoints`、`repo-hygiene`、`migration-inventory` 的檢查；mutation：在 website 加回 `import … from "@/lib/pricebook/repo"`，tsc 失敗。
  - 停止條件：Java、notification、invoice、crm 任一服務在執行期呼叫 pricebook（不只是共用的 proto、CodeMeta 或測試替身）。

- [ ] **7. client 拿掉配送溫層、產銷履歷與商品認證**（被擋於：無；開工時先做本步的盤點）
  - 範圍：開工先跑 `inventory.sh clientfields`，把 client 的命中逐檔分成「顯示」「輸入」「送出」三類寫進進度表。顯示：刪（購物車與結帳的溫層分組、商品頁與首頁的「有機」「產銷履歷」標籤、認證徽章）；輸入：刪選擇器；送出：Java 仍要求的欄位改送固定常溫值，集中在每個 client 一個常數，註解寫明 B4 時刪；admin `product-certifications` 與賣家端的認證申請整個刪（Java 的認證流程延後）。
  - 消費端：`bash …/inventory.sh clientfields`（Java、ai-service、pricebook、invoice、notification、social、im 的命中不在本步範圍；ai-service 若在步驟 5 後仍有，記進進度表）。
  - 不能動：Java 的契約與資料；其他配送屬性（配送方式、運費）。
  - 預期剩下：每個 client 的常溫常數與使用處；CodeMeta 生成物（`admin/src/lib/codemeta/generated.ts`、`buyer-app/lib/core/codemeta/code_meta.dart` 等，`ShippingTemperature` 仍被 Java 用）；design token（`website/design-system/cross-platform-tokens.json`）裡溫層色的 token：client 沒有使用處時刪，三端一起改並跑 `design:check`。
  - 驗收：client 命中只剩預期命中；website、admin、兩支 App 同步驟 1、2；`pnpm design:check`；mutation：拿掉 seller-app 送出時的常溫常數，送出的測試失敗（測試斷言 request 帶常溫值）。e2e 與視覺回歸待 CI。
  - 停止條件：Java 對某個欄位的要求無法用固定值滿足（例如依溫層計算運費且常溫值會算錯）。

- [ ] **8. 收尾**：完成定義逐項核對（每一步的 `inventory.sh` 結果、改到的模組的 `AGENTS.md` 與 `CARRYOVER.md` 只寫現行）；獨立設計審查（design-review skill，看有沒有為了刪除而加的過渡寫法）；`inventory.sh` 留在附件不進 CI（一次性的盤點）；延後項附觸發條件寫進主計畫的 B1（AssetReferenceType 的食譜值）、B3（Uber 的 Java 程式與 4 份 Uber ADR 標 superseded）、B4、B6 與 CodeMeta 段，並搬回主清單；主計畫的 A9 寫明採購已不依賴 pricebook；ADR 的 superseded 標記雙向；需要人做的事：步驟 5 若 Will 決定跑線上評測，在那一步處理，收尾沒有。驗收：`bash …/inventory.sh` 全部單元的輸出貼進進度表，只剩各步「預期剩下」與進度表登記的命中；主計畫 `rg -n "ecommerce-scope-cut" docs/plans/2026-09-24-go-modular-monolith.md` 在 A4、A9、B1、B3、B4、B6 都有命中。

## 決定

- **D1–D5**：見 ADR `docs/adr/2026-10-27-general-ecommerce-scope-cut.md`（已決 2026-10-27，Will）。
- **D6**（Will 2026-10-28）：步驟 5 不量改動前的基線；離線評測用本機種子分類樹（33 節點），golden set 依新行為重寫。理由：完整分類樹只在舊公司資料庫，改成一般電商後生鮮大分類樹沒有保留價值。
- **D7**（Will 2026-10-28）：`procurement_price_previews.quality_knowledge_state` 保留；下游確認快照與發布核准表的行情欄位與發布阻擋一起刪（D2「報價只看供應商報的價」的直接結果）。
- 步驟 5 的品項解析改成只用分類樹與 LLM 召回、`procurement_price_previews` 刪行情欄位：都是 D2「品項只解析到平台分類樹、報價只看供應商報的價」的直接結果，不另列決定。解析品質掉太多時照步驟 5 的停止條件回報。
- CodeMeta 值域：只服務已刪功能的照前提刪（凍結規則只禁止新增與手改生成檔；刪是隨消費者退場，ADR `2026-09-25-codemeta-retirement`），仍被 Java 用到的延後。不另列決定。
- 審查（2026-10-27，獨立 pi 實例，9 項）：全部改進計畫；第 7 項「上架作物 picker 是 UI 流程改動」查證後不成立（picker 只在 insights，見前提），改成刪除 insights 的 `CropCatalogSearch`。

## 進度

狀態值：未開始／進行中／程式完成待驗收／完成。

| 步驟 | 狀態 | 已跑的驗收 | 未跑的驗收與原因 | commit |
|---|---|---|---|---|
| 1 website 純前端 | 完成 | 開工盤點（base `9651e29a35`）：welfare 20、seasonal 6、origin 16 檔；base 基線：website lint、test:unit 全過，buyer-app analyze、custom_lint 全過。完成後：三個單元 `inventory.sh` 0 命中；website tsc、lint、test:unit（2190 pass／0 fail／1 skip）、vitest `--changed main`、build（`API_BASE_URL` 指向 127.0.0.1:9）、test:build-output（9 pass）、`design:check --apps-root ..`；buyer-app analyze、custom_lint、`test/features/category`（44）、deadcode、build_runner 後無 drift；mutation：首頁 import 回 `HomeSeasonalBanner`，tsc TS2307 失敗 | e2e、視覺回歸待 CI；buyer-app 完整測試依前提延後 | `83733d7dc8` |
| 2 食譜 | 完成 | 開工盤點（base `7b69ca7351`）：recipe 約 150 檔；停止條件不成立（食譜表只有 content 讀，media 只有引用類型列舉）。完成後 `inventory.sh recipe` 只剩登記的命中：content 已套用的 migration 與 `atlas.sum`（不可改）、CodeMeta 生成的 `media/ref_type.go`、`pkg/media/ref_type.go` 與鏡像它的 `media/finalize_args.go`（Java 仍用 7119，延後 B1）、Java `AssetReferenceType`、pricebook 與 ai-service 已套用 migration 的同字詞、website `lib/seller/ai-image.ts` 的 AI 修圖提示文字（與 ai-service 種子同文，不是食譜功能）、`scripts/check-fullwidth-var-expansion.sh`（Makefile recipe）。backend：`make connect`、`make sqlc`（無 drift）、gofmt、`go vet ./...`（含 integration tag）、`go test ./...`、受影響套件的 integration（cmd/hawkr、content、media、httpapi、legacy/mediaservice、revalidate、arch）、content `atlas migrate validate`、`make uitext`（無 drift）；`buf lint`；TS 與 Dart 生成（buf 1.50.0）；`packages/hawkr_api` analyze 與 test（118）；`@hawkr/api` test（90）；admin lint、test（1165）、build；website tsc、lint、test:unit、vitest `--changed main`、build、test:build-output（9）、`design:check`；buyer-app analyze、custom_lint、搜尋與受影響測試（147）、deadcode、build_runner 無 drift、architecture_check；seller-app analyze 與 connect_auth 測試；mutation：`schema.sql` 加回 `content.recipes`，`TestSchemaFileMatchesMigrations/content` 失敗 | e2e、視覺回歸、完整 integration（`go test -tags integration ./...`）待 CI；兩支 App 完整測試依前提延後；未在 base 先跑本步驗收基線（漏做，失敗的項目都已確認是本步改動所致並修正） | `ece22fbce1` |
| 3 Uber client 與 gateway | 完成 | 開工：push 後 CI（`f7e19b2808` 與步驟 1、2 的 commit）每個 job 都沒開跑，annotation 是 GitHub「近期付款失敗或花費上限」，步驟 1、2 的 CI 驗收仍是待 CI；盤點（base `f7e19b2808`）：Java 122 之外 115 檔；停止條件不成立（Uber 在訂單頁只是附加分支，其他配送方式走 PayUni 託運單；Java `CheckoutValidator` 不把 3003 放進 `availableOptions`）。base 基線：gateway gofmt、vet、test 全過；website tsc、test:unit（2187）全過，lint 1 個既有格式錯誤（`lib/config/__tests__/media-env.unit.test.ts`，作品集計畫 6a 帶入，不在本計畫修）；admin lint、test（1165）全過；buyer-app analyze、custom_lint、cart／order／core 測試（247）全過；seller-app analyze 與受影響測試全過。完成後 `inventory.sh uber` 只剩登記的命中：Java 122、Java 仍實作的 proto（order、shipping、product_import）與其 Go 生成物（`pkg/proto`、`internal/legacy/gen/order`）、CodeMeta 生成物（Go、TS、Dart）與 `generate.sh`、`bizcode.go` 的 9024、`.env.template` 與 `docker-compose-local.yml` 的 shipping-service／payuni-webhook-service 設定、`docs/runbooks/uber-quote-retention/` 的 SQL（Java 表）、三支斷言「3003 一律 unsupported」的回歸測試（website `shipping-requirements`、`shipment-provider`，buyer `shipping_presentation_test`）。另以 `rg -i uber` 廣搜各 client，補刪 inventory 模式抓不到的命中（buyer 訂單物流說明與 933 文案、匯入範本的 `sku_shipping_uber` 欄、`backend/AGENTS.md` 的 provider 清單）。gateway gofmt、vet、test、`openapi_coverage_test`（上限 465→460）、`gateway-json-contract-check.py`；website tsc、lint（同基線）、test:unit（2181 pass／0 fail）、vitest `--changed main`（1333）、build（`API_BASE_URL` 指向 127.0.0.1:9）、test:build-output（9）、`design:check --apps-root ..`；admin lint、test（1165）、build；buyer-app format、analyze、custom_lint、deadcode、enum_text、build_runner（`cart_controller.g.dart` 隨來源重生）、architecture_check、cart／order／core／product 測試（413）；seller-app format、analyze、custom_lint、deadcode、enum_text、build_runner 無漂移、core／order／seller／product 測試（1068）；`check-legacy-endpoints.py`、`check-fullwidth-var-expansion.sh`、`check-enum-text-cross.sh`。mutation：gateway 加回 `GET /admin/uber_deliveries` 不加 operation，`TestEveryRouteIsInOpenAPISpec` 失敗；website `shippingDestinationKind` 把 3003 歸回 address，2 支測試失敗；buyer `shippingPresentationOf` 把 3003 歸回 homeDelivery，5 支測試失敗 | e2e、視覺回歸、actionlint、coverage ratchet 待 CI（CI 目前因 GitHub 帳務無法開跑）；兩支 App 完整測試依前提延後 | `035768cc3b` |
| 4 食品業者登錄與 FDA 轉報 | 完成 | 開工：停止條件不成立（主管機關來函保存只與食品登錄共用店家列鎖，不讀其資料）；守住「來函登記先鎖帳號店家列」的是既有的 `TestAuthorityOrderRegisteredAfterThePurgeShowsAlreadyPurged`（mutation：拿掉 `authority.go` 登記時的 `stint.LockAccounts`，該測試失敗，但是卡到 go test 10 分鐘逾時才失敗）。新 migration `20261027164917_seller_drop_food_registrations_and_referrals` 刪 `food_registrations`、`case_referrals`、`case_referral_replies` 與相關函式；proto 刪 RPC 與欄位（欄號與 enum 值 reserved）；uitext 刪目錄 key 9 個、模組句子 19 個。另刪公開店家 REST 的 `foodBusinessRegNo`、`isProcessedFood`（恆為空、無讀者；Will 2026-10-28 同意）與 `docs/runbooks/seller-food-registration-runbook.md`；補 `TestKYCReviewStartsFromTheReopeningWhenItIsTheLatest`、`TestAgreementAcceptancesBindExactlyOneSourceByKind`（原守護隨 foodreg 測試刪除；兩支都以測試資料庫內的 mutation 證明會失敗）。`inventory.sh foodreg`／`fda` 剩下登記的命中：已套用 migration 與 `atlas.sum`、proto reserved 名稱與生成描述檔、`services/seller-service` 與 Java（無執行期呼叫已刪功能）、`pkg/errors/bizcode`、通報／處分／來函的「食安」字詞（`FOOD_SAFETY` 類別、`SubmitFoodSafetyReport`、`FOOD_SAFETY_INCIDENT`、`RISK_GROUND_FOOD_SAFETY`）、CodeMeta 生成物、`docs/runbooks/food-safety/sql`、website 一般文案兩處。backend：`make connect`、`make sqlc`、`make uitext`、TS／Dart 生成重跑無 drift、gofmt、`go vet ./...`（含 integration tag）、`go test ./...`、seller 全部 integration（`-p 2`）、httpapi／cmd/hawkr／arch／reviewqueue／legacy integration、seller `atlas migrate validate`、`buf lint`、gateway test 與 `TestEveryRouteIsInOpenAPISpec`、`hawkr_api` test（118）、`@hawkr/api`（90）、`@hawkr/ui-text`（55）、`hawkr_ui_text`（46）；admin lint、test（1129 pass／0 fail）、build；website tsc、lint、test:unit（2179 pass／0 fail／1 skip）、build（`API_BASE_URL` 與 `NEXT_PUBLIC_API_BASE_URL` 指向 127.0.0.1:9/api/v1）、test:build-output、`gen:legal` 無 drift、`design:check`；seller-app analyze、custom_lint、format、build_runner、完整測試（2282）、deadcode、enum_text；buyer-app analyze；`check-enum-text-cross.sh`。mutation：`schema.sql` 加回 `food_registrations`，`TestSchemaFileMatchesMigrations/seller` 失敗。獨立審查兩輪（第一輪 3 項：e2e 斷言漏改已修、同意紀錄 CHECK 補測試、鎖順序守護駁回成立） | e2e、視覺回歸、coverage ratchet（seller-app baseline 只移除已刪檔、總行數未重算）待 CI；未在 base 先跑本步驗收基線 | 見 commit |
| 5 採購拿掉 pricebook 依賴 | 未開始 | | | |
| 6 刪 pricebook | 未開始 | | | |
| 7 client 拿掉溫層與認證 | 未開始 | | | |
| 8 收尾 | 未開始 | | | |
