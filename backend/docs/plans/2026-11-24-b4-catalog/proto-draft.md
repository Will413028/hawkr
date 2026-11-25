# B4-3 Connect proto 草稿（交 Will 審，尚未實作）

> **已被 D24 部分取代（2026-11-26）**：本稿是修訂前的底稿；與 `api-baseline-brief.md` 開頭的拍板段不一致處（例如 `ListProducts` 的 `followed_*` 篩選、公開與賣家共用 `Product`、低庫存定義、沒有 `SetStock`）以 D24 為準。

快照：dev `cb273c7d9`。本檔只是草稿：沒有改 `api/`、沒有跑 `buf`，proto 文字是否能通過 `buf lint` 尚未驗證（見第 6 節）。

證據標記：【量】＝讀過該檔案或跑過指令；【推】＝推論；【未查證】＝沒有可靠來源。

## 0. 依據與設計前提

每條規則都標出處，縮寫：`計畫`＝`backend/docs/plans/2026-11-24-b4-catalog.md`；`簡報`＝`…/2026-11-24-b4-catalog/decision-brief.md`；`IF`＝`interface-inventory.md`；`CI`＝`client-inventory.md`；`規範`＝`docs/共通規範/API 設計規範（Connect）.md`。

- **三支 service**（計畫 B4-3 範圍「D4 的三支 service」、D4 已決）：`CatalogPublicService`（任何人）、`SellerCatalogAppService`（賣家）、`CatalogAdminService`（管理員）。代行的 `CatalogDelegatedService` 是 B4-5d（計畫 D9、B4-5d），本稿排除。
- **檔案與 package**（規範 §一）：前台一個檔案 `api/hawkr/catalog/v1alpha1/catalog.proto`，Public 與賣家 App 兩個 service 共用此檔（既有檔頭寫明「服務與 RPC 隨後的步驟加入」【量】）；後台獨立 `api/hawkr/admin/catalog/v1alpha1/catalog.proto`，package `hawkr.admin.catalog.v1alpha1`。
- **店家身分由呼叫者推導，請求不帶 `store_id`**（簡報 D4、計畫不變量表第 45 列、CD9）：賣家 service 每個方法宣告賣家能力 `CAP_PRODUCT_MANAGE`，interceptor 以 `seller.ActingStore` 放進 ctx（規範 §二 第 36 點）。能力選擇的依據：舊 gateway 的 `/app/products*` 路由全部掛 `sellerHandle(CapProductManage)`（CI §1a 第 49 列註記【量】）。代價：停權的店家連讀自己的商品也被拒（`CapProductManage` 需要 `NOT_SUSPENDED`，`capability.go:79`【量】），與現況相同。
- **冪等與並發**（簡報 D4 選項 A、規範 §七）：寫入用 `request_id`（取代 `idempotency_request` 表），並發用 etag（取代 `version`／`expectedVersion`）。
- **不為無 client 的端點寫契約**（計畫 D5、簡報 D5）：下方每個 RPC 都有 CI 盤點的 client 呼叫點，或是計畫明列的新需求（D23 的 `AdjustStock`、「低庫存數 API」、「搜尋」）。
- **既有訊息不重新定義**：`ProductInput`、`ImageInput`、`DescriptionBlock`、`BulletList`、`Table`、`OptionAxisInput`、`SkuInput` 與 D21 的 `limits` option 沿用 `catalog.proto` 現況【量】；本稿新增的輸出訊息只重用其中的值型別（`BulletList`、`Table`）。
- **前台與後台訊息分開，值型別後台 import 前台**（規範 §五）：後台另有自己的 `Product`、`ProductSummary`。
- **分頁一律 keyset**（規範 §六）；`page_size` 預設 20、上限 100（上限依模組現有 `MaxListPageSize = 100`【量】，預設值沿用舊 gateway 的 `defaultPageSize` 20【推】）。
- **語系**（ADR `2026-10-17-multi-locale-architecture` D1，第 45 行【量】）：分類名稱由伺服器依 `Accept-Language` 回單一字串；回應 `Vary: Accept-Language`。

## 1. Service 清單

| Service | 檔案／package | 誰能呼叫 | 一句話用途 | RPC 數 |
|---|---|---|---|---|
| `CatalogPublicService` | `hawkr.catalog.v1alpha1` | 任何人（有帶 token 時 ctx 有呼叫者）；規範 §二 | 買家瀏覽：商品列表、搜尋、建議、商品頁、分類樹、hashtag 建議與熱門 | 7 |
| `SellerCatalogAppService` | `hawkr.catalog.v1alpha1` | 登入的賣家；每個方法宣告 `CAP_PRODUCT_MANAGE`，店家來自 `ActingStore`（宣告表放 `backend/internal/httpapi/catalog`，規範 §二 第 36 點） | 賣家管理自己店的商品：清單、讀取、建立、修改、上下架、刪除、調整庫存、低庫存數 | 8 |
| `CatalogAdminService` | `hawkr.admin.catalog.v1alpha1` | 管理員；`adminauth` 宣告權限（規範 §二 `AdminService`、`backend/AGENTS.md`） | 營運後台查商品：列表與詳情（唯讀） | 2 |

合計 3 支 service、17 個 RPC。

授權宣告：

- 賣家：8 個方法全部 `requires(seller.CapProductManage)`；沒有豁免。
- 後台：新權限 `catalog.products.list`、`catalog.products.get`（命名規則 `<模組>.<資源>.<動詞>`，`adminperm.go` 檔頭【量】），兩個方法都宣告 `auditReadOnly`（`adminauth.go:36-44` 的定義：讀取不留稽核【量】）。**B4-3 沒有後台寫入 RPC，所以沒有 `audited` 方法、沒有 `adminaudit.Record` 呼叫**：admin 改狀態是 B4-5 的商品限制措施（CI §1a 第 34 列、IF §1.1 `UpdateProductStatus`），措施由 seller 模組擁有並寫稽核。計畫 B4-3 範圍寫的「`adminauth` 權限宣告與 `adminaudit`」在本步只落實前半。
- Public：`publicViewerDependent` 表（`backend/internal/httpapi/viewer.go`，缺宣告不能啟動，規範 §九）：`ListProducts` 宣告 `true`（有 `followed_*_only` 篩選，見 Q2），其餘 6 個宣告 `false`。

## 2. Proto 草稿

### 2.1 `api/hawkr/catalog/v1alpha1/catalog.proto`（附加在既有訊息之後；既有的 `ProductInput` 等不動）

檔頭需新增的 import（既有的兩個保留）：

```proto
import "google/api/field_info.proto";
import "google/protobuf/timestamp.proto";
import "hawkr/common/v1alpha1/image.proto";
import "hawkr/common/v1alpha1/store_card.proto";
```

```proto
// CatalogPublicService 是任何人都能讀的商品、搜尋與分類，不需要登入。
// 只有 ACTIVE、沒刪除、店家此刻可曝光（seller.pub_exposable_stores，時間用請求的 clock）的商品讀得到；其他一律與不存在相同（not_found）。
service CatalogPublicService {
  // ListProducts 是買家瀏覽的商品列表（分類頁、市集、店家的商品頁）。filter 的欄位全部選填，之間是 AND。
  // 帶 followed_product_only 或 followed_seller_only 需要登入，沒登入回 unauthenticated；這兩個篩選讓回應依觀看者而定。
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // SearchProducts 依關鍵字找商品（標題、描述文字、hashtag、分類名稱的 searchable_text，pg_trgm），預設依相關度排序。
  rpc SearchProducts(SearchProductsRequest) returns (SearchProductsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // SuggestProductNames 是搜尋框輸入時的建議：商品名稱符合的前幾筆，不分頁，最多 20 筆。
  rpc SuggestProductNames(SuggestProductNamesRequest) returns (SuggestProductNamesResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // GetProduct 讀一個公開可見的商品的完整內容（含 SKU 與庫存）。讀不到（不存在、不是 ACTIVE、已刪除、店家不可曝光）回 not_found。
  // 不帶觀看者狀態（是否已收藏、是否訂閱補貨）：那些由買家 service 的 CheckFavorite、ListRestockSubscriptions 提供。
  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // ListCategories 回整棵平台分類樹（兩層，頂層在前，同層依平台排序）。分類只在 migration 變動，不分頁：節點數固定為 62（10 個頂層、52 個葉節點，計畫 B4-2a）。
  // 名稱依 Accept-Language 協商；沒有該語言時是 zh-TW。
  rpc ListCategories(ListCategoriesRequest) returns (ListCategoriesResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // SuggestHashtags 是商品標籤輸入與搜尋的建議：名稱符合的標籤與其商品數（只算可曝光店家的 ACTIVE 商品），商品數多的在前。沒有分頁，最多 20 筆。
  rpc SuggestHashtags(SuggestHashtagsRequest) returns (SuggestHashtagsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // SuggestTrendingHashtags 是熱門商品標籤：各標籤的 ACTIVE 商品數（只算可曝光店家），多的在前。只取前 max_results 名，不分頁。
  rpc SuggestTrendingHashtags(SuggestTrendingHashtagsRequest) returns (SuggestTrendingHashtagsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

// SellerCatalogAppService 是賣家管理自己店家的商品。店家由登入身分推導，請求不帶 store_id；
// 商品不是這家店的、或已刪除，與不存在一樣回 not_found（規範 §二 防枚舉）。每個方法需要賣家能力 CAP_PRODUCT_MANAGE。
service SellerCatalogAppService {
  // ListMyProducts 列出自己店家沒刪除的商品，任何狀態，不看店家曝光；最新建立的在前。
  rpc ListMyProducts(ListMyProductsRequest) returns (ListMyProductsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // GetMyProduct 讀自己店家的商品完整內容（任何狀態，含 etag 與每個 SKU 的庫存）。
  rpc GetMyProduct(GetMyProductRequest) returns (GetMyProductResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // CountLowStockProducts 回自己店家庫存偏低的商品數（賣家中心導覽列的徽章與儀表板用）。
  // 偏低的定義見 CountLowStockProductsResponse。
  rpc CountLowStockProducts(CountLowStockProductsRequest) returns (CountLowStockProductsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // CreateProduct 建立一個草稿商品（建立一律是草稿；要上架呼叫 SetProductStatus）。
  // 同一家店以同一個 request_id 重送回第一次建立的商品目前的樣子，不重複建立；內容不同回 invalid_argument。
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse);

  // UpdateProduct 整體取代商品的內容（狀態不變）：沒有帶的圖片、選項軸與 SKU 組合會被移除；既有 SKU 的庫存不在這裡改（用 AdjustStock）。
  // etag 必填，與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。商品目前是 ACTIVE 時，輸入至少要有一張主圖，否則回 failed_precondition。
  rpc UpdateProduct(UpdateProductRequest) returns (UpdateProductResponse) {
    option idempotency_level = IDEMPOTENT;
  }

  // SetProductStatus 切換商品狀態：草稿、上架、下架任兩個都能互切，切成目前的狀態也回成功。
  // 切成 ACTIVE 時商品至少要有一張主圖，否則回 failed_precondition。首次上架寫入 first_publish_time，之後不清除。
  // etag 必填（上架的判斷依據包含圖片，仍可能被別處改動，規範 §七）。
  rpc SetProductStatus(SetProductStatusRequest) returns (SetProductStatusResponse) {
    option idempotency_level = IDEMPOTENT;
  }

  // DeleteProduct 刪除商品：從賣家清單消失、不可還原，列保留（訂單與評價仍引用）。已經刪除的也回成功。etag 選填，帶了就檢查。
  rpc DeleteProduct(DeleteProductRequest) returns (DeleteProductResponse) {
    option idempotency_level = IDEMPOTENT;
  }

  // AdjustStock 以有號變動量調整自己店家某個 SKU 的庫存（進貨加、盤點減）。
  // 同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。調整後不得小於 0，否則回 failed_precondition。
  rpc AdjustStock(AdjustStockRequest) returns (AdjustStockResponse);
}

// ProductStatus 是商品的狀態（ADR 簡報 D16、D22）。
enum ProductStatus {
  PRODUCT_STATUS_UNSPECIFIED = 0;
  // 草稿：只有賣家看得到。
  PRODUCT_STATUS_DRAFT = 1;
  // 上架：公開可見（店家可曝光時）。
  PRODUCT_STATUS_ACTIVE = 2;
  // 下架：賣家自己下架，可還原。
  PRODUCT_STATUS_ARCHIVED = 3;
}

// ProductSort 是商品列表與搜尋的排序。
enum ProductSort {
  // 不指定：ListProducts 與 ListMyProducts 是 LATEST，SearchProducts 是 RELEVANCE。
  PRODUCT_SORT_UNSPECIFIED = 0;
  // 最新上架（first_publish_time 遞減）。
  PRODUCT_SORT_LATEST = 1;
  // 價格低到高（依 min_price）。
  PRODUCT_SORT_PRICE_ASC = 2;
  // 價格高到低（依 max_price）。
  PRODUCT_SORT_PRICE_DESC = 3;
  // 銷量多到少。
  PRODUCT_SORT_SALES = 4;
  // 評分高到低。
  PRODUCT_SORT_RATING = 5;
  // 相關度（只有 SearchProducts 接受；ListProducts 收到回 invalid_argument）。
  PRODUCT_SORT_RELEVANCE = 6;
}

// HashtagMatch 是多個 hashtag 的比對方式。
enum HashtagMatch {
  // 不指定：ANY。
  HASHTAG_MATCH_UNSPECIFIED = 0;
  // 商品帶有其中任何一個。
  HASHTAG_MATCH_ANY = 1;
  // 商品帶有全部。
  HASHTAG_MATCH_ALL = 2;
}

// ProductFilter 是列表與搜尋共用的篩選，全部選填，之間是 AND。清單欄位的上限是提案值（見草稿第 6 節）。
message ProductFilter {
  // 分類 ID，可以給頂層分類（包含它底下所有葉節點）或葉節點；多個之間是 OR。
  repeated string category_ids = 1 [(hawkr.common.v1alpha1.limits).max_items = 20];
  // 價格區間，新台幣元的整數，含邊界；比對商品的 [min_price, max_price] 與區間有交集。
  optional int64 price_min = 2;
  optional int64 price_max = 3;
  // 標籤（正規化後的名稱，或使用者輸入的樣子，規則同 ProductInput.hashtag_inputs）。
  repeated string hashtags = 4 [(hawkr.common.v1alpha1.limits).max_items = 10];
  HashtagMatch hashtag_match = 5;
  // 只看這些店家的商品；店家不可曝光的照樣排除。
  repeated string store_ids = 6 [(hawkr.common.v1alpha1.limits).max_items = 50];
  // 只列登入者收藏的商品。需要登入。
  bool followed_product_only = 7;
  // 只列登入者追蹤的店家的商品。需要登入。
  bool followed_seller_only = 8;
}

message ListProductsRequest {
  ProductFilter filter = 1;
  // 不指定是 LATEST；不接受 RELEVANCE。
  ProductSort sort = 2;
  // 預設 20，上限 100；負數回 invalid_argument。
  int32 page_size = 3;
  // page_token 與 filter、sort 綁定，其中任何一個與取得 token 時不同回 invalid_argument。
  string page_token = 4;
}

message ListProductsResponse {
  repeated ProductSummary products = 1;
  string next_page_token = 2;
}

message SearchProductsRequest {
  // 關鍵字；去頭尾空白後不得為空。
  string query = 1 [
    (google.api.field_behavior) = REQUIRED,
    (hawkr.common.v1alpha1.limits).max_runes = 100
  ];
  // filter 只用分類、價格、hashtag、店家；followed_* 不接受（設了回 invalid_argument）。
  ProductFilter filter = 2;
  // 不指定是 RELEVANCE。
  ProductSort sort = 3;
  // 預設 20，上限 100。
  int32 page_size = 4;
  string page_token = 5;
}

message SearchProductsResponse {
  repeated ProductSummary products = 1;
  string next_page_token = 2;
}

message SuggestProductNamesRequest {
  // 輸入的前綴或片段；正規化後是空的回空清單。
  string query = 1 [
    (google.api.field_behavior) = REQUIRED,
    (hawkr.common.v1alpha1.limits).max_runes = 100
  ];
  // 最多回傳幾筆，1 到 20；0 表示預設值 10，超過 20 以 20 計，負數回 invalid_argument。
  int32 max_results = 2;
}

message SuggestProductNamesResponse {
  // 商品名稱，相關度高的在前，已去重。
  repeated string names = 1;
}

message GetProductRequest {
  string product_id = 1 [(google.api.field_behavior) = REQUIRED];
}

message GetProductResponse {
  Product product = 1;
}

message ListCategoriesRequest {}

message ListCategoriesResponse {
  // 頂層分類，每個帶底下的葉節點。
  repeated Category categories = 1;
}

// Category 是平台分類樹的一個節點。商品只能掛在葉節點（沒有 children 的節點）。
// 路徑（頂層 → 葉節點）由這棵樹推得，沒有另外的方法。Shopify 對照 ID 是內部資料，不對外。
message Category {
  string id = 1 [(google.api.field_behavior) = IDENTIFIER];
  // 依請求語系的名稱。
  string name = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 底下的葉節點；葉節點本身沒有。
  repeated Category children = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
}

message SuggestHashtagsRequest {
  // 搜尋字串，可以帶開頭的 #；正規化後是空的回空清單。
  string query = 1 [(google.api.field_behavior) = REQUIRED];
  // true 是名稱包含 query，false 是以 query 開頭。
  bool contains = 2;
  // 最多回傳幾筆，1 到 20；0 表示預設值 10，超過 20 以 20 計，負數回 invalid_argument。
  int32 max_results = 3;
}

message SuggestHashtagsResponse {
  repeated HashtagCount hashtags = 1;
}

message SuggestTrendingHashtagsRequest {
  // 最多回傳幾名，1 到 50；0 表示預設值 20，超過 50 以 50 計，負數回 invalid_argument。
  int32 max_results = 1;
}

message SuggestTrendingHashtagsResponse {
  repeated HashtagCount hashtags = 1;
}

// HashtagCount 是一個商品標籤與它在範圍內的商品數。
message HashtagCount {
  // 正規化後的名稱，用來比對與連結。
  string name = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 最多商品使用的輸入樣子，用來顯示。
  string display = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 product_count = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// ===== 賣家 =====

message ListMyProductsRequest {
  // 只列這個狀態的商品；UNSPECIFIED 是全部。page_token 只能用在同一個 status 與 low_stock_only。
  ProductStatus status = 1;
  // 只列庫存偏低的商品（定義同 CountLowStockProductsResponse）。
  bool low_stock_only = 2;
  // 預設 20，上限 100。
  int32 page_size = 3;
  string page_token = 4;
}

message ListMyProductsResponse {
  repeated ProductSummary products = 1;
  string next_page_token = 2;
}

message GetMyProductRequest {
  string product_id = 1 [(google.api.field_behavior) = REQUIRED];
}

message GetMyProductResponse {
  Product product = 1;
}

message CountLowStockProductsRequest {}

message CountLowStockProductsResponse {
  // 庫存偏低的商品數：自己店家沒刪除的商品裡，至少有一個沒刪除的 SKU 庫存小於或等於 threshold（任何狀態）。
  int32 count = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 伺服器定義的門檻，client 顯示用，不自己寫死數字。
  int32 threshold = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
}

message CreateProductRequest {
  ProductInput product = 1 [(google.api.field_behavior) = REQUIRED];
  // UUIDv4（AIP-155）：同一家店以同一個 request_id 重送回第一次建立的商品；內容不同回 invalid_argument。必填。
  string request_id = 2 [
    (google.api.field_behavior) = REQUIRED,
    (google.api.field_info).format = UUID4
  ];
}

message CreateProductResponse {
  Product product = 1;
}

message UpdateProductRequest {
  string product_id = 1 [(google.api.field_behavior) = REQUIRED];
  // 讀取時拿到的 etag，必填。
  string etag = 2 [(google.api.field_behavior) = REQUIRED];
  // 完整內容，整體取代；沒有 update_mask（見草稿 Q3）。
  ProductInput product = 3 [(google.api.field_behavior) = REQUIRED];
}

// UpdateProductResponse 的商品是修改後的完整資源（含新的 etag）。
message UpdateProductResponse {
  Product product = 1;
}

message SetProductStatusRequest {
  string product_id = 1 [(google.api.field_behavior) = REQUIRED];
  // 目標狀態，不得是 UNSPECIFIED。
  ProductStatus status = 2 [(google.api.field_behavior) = REQUIRED];
  // 讀取時拿到的 etag，必填。
  string etag = 3 [(google.api.field_behavior) = REQUIRED];
}

message SetProductStatusResponse {
  Product product = 1;
}

message DeleteProductRequest {
  string product_id = 1 [(google.api.field_behavior) = REQUIRED];
  // 選填；帶了就與目前版本比對。
  string etag = 2;
}

message DeleteProductResponse {}

message AdjustStockRequest {
  // 要調整的 SKU；不屬於自己店家或已刪除回 not_found。
  string sku_id = 1 [(google.api.field_behavior) = REQUIRED];
  // 有號變動量，不得為 0；調整後的庫存必須在 0 到 2147483647 之間。
  int32 delta = 2 [(google.api.field_behavior) = REQUIRED];
  // UUIDv4（AIP-155）：同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。必填：變動量重送會重複計算。
  string request_id = 3 [
    (google.api.field_behavior) = REQUIRED,
    (google.api.field_info).format = UUID4
  ];
}

message AdjustStockResponse {
  // 調整後的 SKU（含新的庫存）。
  Sku sku = 1;
}

// ===== 輸出資源 =====

// ProductImage 是商品的一張圖（圖庫或描述圖片）。
message ProductImage {
  // 圖片資產 ID；只有賣家讀自己的商品才設，寫入沿用這張圖時放進 ImageInput.asset_id。
  string asset_id = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  hawkr.common.v1alpha1.Image image = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 替代文字，可空。
  string alt = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// ProductDescriptionBlock 是描述的一個區塊（輸出），剛好設一種；形狀對應寫入的 DescriptionBlock。
message ProductDescriptionBlock {
  oneof block {
    string paragraph = 1;
    ProductImage image = 2;
    BulletList bullet_list = 3;
    Table table = 4;
  }
}

// OptionAxis 是一個選項軸（輸出）。
message OptionAxis {
  string name = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated string values = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// Sku 是商品的一個 SKU。沒有該軸的位置，option 是空字串。
message Sku {
  string id = 1 [(google.api.field_behavior) = IDENTIFIER];
  string option1 = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  string option2 = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 售價，新台幣元的整數。
  int64 price = 4 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 原價（劃線價）；沒有設不設。
  optional int64 compare_at_price = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 現在的庫存，不為負。
  int32 stock_qty = 6 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// ProductHashtag 是商品的一個標籤。
message ProductHashtag {
  // 正規化後的名稱，用來比對與連結。
  string name = 1 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 使用者輸入的樣子，用來顯示。
  string display = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// Product 是商品的完整內容。公開讀取與賣家讀自己的商品共用這個訊息；只有賣家讀自己的商品才設的欄位在註解標明。
message Product {
  string id = 1 [(google.api.field_behavior) = IDENTIFIER];
  // 賣這個商品的店家；公開讀取才設（店家卡片讀不到時不設），賣家讀自己的商品不設。
  hawkr.common.v1alpha1.StoreCard store = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  string category_id = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
  string title = 4 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 只有賣家讀自己的商品才設；公開讀取的商品一定是 ACTIVE。
  ProductStatus status = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 第一次上架的時間；從沒上架過不設。
  google.protobuf.Timestamp first_publish_time = 6 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 沒刪除 SKU 的最低與最高售價，新台幣元的整數。
  int64 min_price = 7 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 max_price = 8 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 sales_count = 9 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 favorite_count = 10 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 平均評分，1 到 5；沒有評價不設。
  optional float rating_average = 11 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 review_count = 12 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated ProductImage gallery = 13 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated ProductDescriptionBlock description = 14 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated OptionAxis axes = 15 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated Sku skus = 16 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated ProductHashtag hashtags = 17 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 修改、上下架與刪除時帶回，client 視為不透明字串；只有賣家讀自己的商品才設。
  string etag = 18;
  // 只有賣家讀自己的商品才設。
  google.protobuf.Timestamp create_time = 19 [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp update_time = 20 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// ProductSummary 是列表的一列（商品卡片）。
message ProductSummary {
  string id = 1 [(google.api.field_behavior) = IDENTIFIER];
  // 公開列表才設（店家卡片讀不到時不設）。
  hawkr.common.v1alpha1.StoreCard store = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  string category_id = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
  string title = 4 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 主圖庫的第一張；沒有圖不設。
  hawkr.common.v1alpha1.Image cover_image = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 只有賣家清單才設。
  ProductStatus status = 6 [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp first_publish_time = 7 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 min_price = 8 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 max_price = 9 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 sales_count = 10 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 favorite_count = 11 [(google.api.field_behavior) = OUTPUT_ONLY];
  optional float rating_average = 12 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 review_count = 13 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 沒刪除 SKU 的庫存總和；只有賣家清單才設（庫存頁用，免得逐商品再讀 SKU）。
  optional int32 total_stock = 14 [(google.api.field_behavior) = OUTPUT_ONLY];
  // 只有賣家清單才設；讀取列表不給 etag 的話，列表上的上下架動作拿不到版本（見 Q5）。
  string etag = 15;
}
```

### 2.2 `api/hawkr/admin/catalog/v1alpha1/catalog.proto`（新檔）

```proto
syntax = "proto3";

// catalog 模組給後台的 API：商品的唯讀查詢。前台的 API 在 hawkr.catalog.v1alpha1。
//
// 後台沒有商品的寫入：管理員強制下架是商品限制措施（B4-5，由 seller 模組擁有），不直接改商品狀態；分類只在 migration 變動，沒有分類管理。
// 圖片、描述區塊、SKU 等值型別沿用前台的定義（API 設計規範（Connect）第五節）。
package hawkr.admin.catalog.v1alpha1;

import "google/api/field_behavior.proto";
import "google/protobuf/timestamp.proto";
import "hawkr/catalog/v1alpha1/catalog.proto";
import "hawkr/common/v1alpha1/image.proto";

// CatalogAdminService 只有管理員能呼叫；權限 catalog.products.list、catalog.products.get（adminauth，auditReadOnly）。
service CatalogAdminService {
  // ListProducts 列出全站沒刪除的商品（任何狀態），最新建立的在前。
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }

  // GetProduct 讀一個商品的完整內容（任何狀態，沒刪除）。不存在回 not_found。
  rpc GetProduct(GetProductRequest) returns (GetProductResponse) {
    option idempotency_level = NO_SIDE_EFFECTS;
  }
}

message ListProductsRequest {
  // 關鍵字，比對標題與標籤；空是不篩選。
  string query = 1;
  // 只列這家店的商品；空是不篩選。
  string store_id = 2;
  // 預設 20，上限 100。
  int32 page_size = 3;
  // page_token 與 query、store_id 綁定。
  string page_token = 4;
}

message ListProductsResponse {
  repeated ProductSummary products = 1;
  string next_page_token = 2;
  // 符合條件的商品總數（後台首頁與商品頁顯示）。
  int32 total_size = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
}

message GetProductRequest {
  string product_id = 1 [(google.api.field_behavior) = REQUIRED];
}

message GetProductResponse {
  Product product = 1;
}

// ProductSummary 是後台列表的一列。
message ProductSummary {
  string id = 1 [(google.api.field_behavior) = IDENTIFIER];
  string store_id = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  string title = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
  hawkr.catalog.v1alpha1.ProductStatus status = 4 [(google.api.field_behavior) = OUTPUT_ONLY];
  hawkr.common.v1alpha1.Image cover_image = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 min_price = 6 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 max_price = 7 [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp first_publish_time = 8 [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp create_time = 9 [(google.api.field_behavior) = OUTPUT_ONLY];
}

// Product 是後台看到的商品完整內容。沒有 etag：後台不寫入。
message Product {
  string id = 1 [(google.api.field_behavior) = IDENTIFIER];
  string store_id = 2 [(google.api.field_behavior) = OUTPUT_ONLY];
  string category_id = 3 [(google.api.field_behavior) = OUTPUT_ONLY];
  string title = 4 [(google.api.field_behavior) = OUTPUT_ONLY];
  hawkr.catalog.v1alpha1.ProductStatus status = 5 [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp first_publish_time = 6 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 min_price = 7 [(google.api.field_behavior) = OUTPUT_ONLY];
  int64 max_price = 8 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 sales_count = 9 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 favorite_count = 10 [(google.api.field_behavior) = OUTPUT_ONLY];
  optional float rating_average = 11 [(google.api.field_behavior) = OUTPUT_ONLY];
  int32 review_count = 12 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated hawkr.catalog.v1alpha1.ProductImage gallery = 13 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated hawkr.catalog.v1alpha1.ProductDescriptionBlock description = 14 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated hawkr.catalog.v1alpha1.OptionAxis axes = 15 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated hawkr.catalog.v1alpha1.Sku skus = 16 [(google.api.field_behavior) = OUTPUT_ONLY];
  repeated hawkr.catalog.v1alpha1.ProductHashtag hashtags = 17 [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp create_time = 18 [(google.api.field_behavior) = OUTPUT_ONLY];
  google.protobuf.Timestamp update_time = 19 [(google.api.field_behavior) = OUTPUT_ONLY];
}
```

### 2.3 設計說明（非 proto 本身，但影響實作）

1. **沒有新增 `ErrorReason`**：寫入失敗的 `ErrActiveRequiresGalleryImage`、`InsufficientStockError` 對應 `failed_precondition` 加 `LocalizedMessage`；輸入錯誤對應 `invalid_argument` 加 `BadRequest`（`InvalidInputError{Field, Reason}`）。規範 §八說 `ErrorInfo` 只在 client 需要在同一個 code 下分支時才帶；盤點沒有看到 client 依這兩種原因分支【推，沒有逐畫面查】。要分支再加（`error_reason.proto` 目前沒有任何 catalog 的值【量】）。
2. **熱門 hashtag 沒有時間窗**：Java 的 `GetPopularHashtags` 有 `startFrom`／`endTo`（依 `product_hashtag.created_at`，`ProductHashtagRepository.java:100`【量】），但四個 client 沒有任何呼叫傳這兩個參數（`rg -n "startFrom|endTo" website/...` 只命中無關的 `appendToLastPage`，buyer-app 只傳 `limit`【量】），且新 schema 的 `product_hashtags` 沒有 `created_at`【量】。依計畫 D5「契約只為有消費者的用途存在」不帶；觸發：有畫面要看某期間的熱門。
3. **分類篩選語意**：`category_ids` 可以給頂層，伺服器展開成它的葉節點。buyer-app 分類頁是多選分類（`category_screen.dart:324`【量】），舊 handler 也收多個 `category_id`（`product.go:93-116`【量】）；模組的 `ListPublicProducts` 目前只收單一 ID、精確比對【量】，這是模組要補的（第 5 節）。
4. **`store` 卡片**由 httpapi 以 `BatchGetStores` 批次組裝（跨模組讀取先用對方 API，`backend/AGENTS.md` 規則 2），不在 catalog 模組內 join。
5. **金額、計數型別**：金額 `int64`（規範 §四），計數 `int32`（與模組 `int32` 一致，有界）。`rating_average` 用 `float`（`StoreCard.rating_average` 先例【量】）。

## 3. 對照表

### 3.1 舊 gateway REST 路由（`client-inventory.md` §1a 52 條＋§2 代行 7 條，逐列）

「新 RPC」欄：S＝`SellerCatalogAppService`、P＝`CatalogPublicService`、A＝`CatalogAdminService`。

| # | 舊路由 | 盤點去向 | 新 RPC 或不對應的理由 |
|---|---|---|---|
| 1 | `GET /public/products` | 改寫（CI §1a） | P `ListProducts`（有關鍵字時 P `SearchProducts`） |
| 2 | `GET /public/products/:id` | 改寫 | P `GetProduct`（登入賣家看自己未上架的商品改走 S `GetMyProduct`） |
| 3 | `GET /public/products/:id/brief` | 改寫（「B4-3 定形狀」） | P `GetProduct`（不另開 brief；唯一呼叫是 `getSkuImageUrl`，gateway 註解稱該方法本身無人呼叫【量，`product.go:829-836`】；Q7） |
| 4 | `GET /public/products/:id/skus` | 刪（D5） | 不對應：無 client。SKU 已在 `Product.skus` |
| 5 | `GET /public/products/:id/rating` | 刪（D5） | 不對應：無 client。評分在 `Product.rating_average`／`review_count`；評論列表在 review 模組 |
| 6 | `GET /public/products/:id/related` | 刪（D5） | 不對應：無 client |
| 7 | `GET /public/hashtags` | 改寫 | P `SuggestHashtags` |
| 8 | `GET /public/hashtags/popular` | 改寫 | P `SuggestTrendingHashtags`（不帶時間窗，見 2.3 第 2 點） |
| 9 | `GET /public/origins/summary` | 刪（D5） | 不對應：無 client |
| 10 | `GET /public/seller-cities/summary` | 刪（D5） | 不對應：無 client |
| 11 | `GET /public/categories` | 刪（D5） | 不對應：無 client（平面清單）；樹用 P `ListCategories` |
| 12 | `GET /public/categories/tree` | 改寫（D8） | P `ListCategories` |
| 13 | `GET /public/categories/:id` | 刪（D5） | 不對應：無 client |
| 14 | `GET /public/categories/:id/path` | 改寫（「兩層，B4-3 定是否另開方法」） | P `ListCategories`（路徑由樹推得，不另開方法；Q7）。buyer-app 一處、seller-app 三處呼叫都改成讀樹 |
| 15 | `GET /public/categories/:id/children` | 刪（D5） | 不對應：無 client |
| 16 | `GET /app/products` | 改寫 | S `ListMyProducts`（只保留 status；`keyword`、`category_id`、`hashtags`、`match` 無賣家端呼叫，website `seller-repo.ts:352-357` 與 seller-app `product_repository.dart:24-35` 只送 status【量】） |
| 17 | `POST /app/products` | 改寫 | S `CreateProduct` |
| 18 | `GET /app/products/:id` | 改寫 | S `GetMyProduct` |
| 19 | `PUT /app/products/:id` | 改寫 | S `UpdateProduct` |
| 20 | `PUT /app/products/:id/status` | 改寫（D16） | S `SetProductStatus` |
| 21 | `DELETE /app/products/:id` | 改寫（D16） | S `DeleteProduct` |
| 22 | `GET /app/products/:id/skus` | 改寫（「併入商品讀取」） | S `GetMyProduct`（`Product.skus`，含庫存） |
| 23 | `GET /app/products/certifications/verify` | 刪（範圍裁切） | 不對應：範圍裁切 ADR `2026-10-27-general-ecommerce-scope-cut` |
| 24 | `GET /app/products/:id/certification_applications` | 刪（範圍裁切） | 不對應：同上 |
| 25 | `POST /app/products/:id/certification_applications` | 刪（範圍裁切） | 不對應：同上 |
| 26 | `POST …/certification_applications/:application_id/withdraw` | 刪（範圍裁切） | 不對應：同上 |
| 27 | `POST /app/categories` | 刪（D8） | 不對應：賣家不建分類 |
| 28 | `GET /app/categories/tree` | 改寫（D8，「賣家端改用 Public」） | P `ListCategories` |
| 29 | `POST /app/shipping_quotes` | 刪（D10） | 不對應：運費試算不屬 catalog（B3 歸 seller） |
| 30 | `GET /app/hashtags` | 刪（D5） | 不對應：無 client（seller-app 打的是 `/public/hashtags`，即第 7 列） |
| 31 | `GET /admin/products` | 改寫 | A `ListProducts`（只保留 `keyword`→`query`、`owner_id`→`store_id`；`status`、`category_id`、`hashtags`、`match` 無 admin 呼叫端【量，`admin/src` 的呼叫只送 keyword、owner_id、page】） |
| 32 | `DELETE /admin/products/cache` | 刪（D5） | 不對應：無 client，單體沒有 Redis 快取 |
| 33 | `GET /admin/products/:id` | 改寫 | A `GetProduct` |
| 34 | `PUT /admin/products/:id/status` | 改寫（B4-5，D7） | 不對應（本步）：改成商品限制措施，B4-5 做 |
| 35 | `GET /admin/product_certification_applications` | 刪（範圍裁切） | 不對應：範圍裁切 |
| 36 | `GET …/product_certification_applications/:id` | 刪（範圍裁切） | 不對應：範圍裁切 |
| 37 | `GET …/:id/audit_logs` | 刪（範圍裁切） | 不對應：範圍裁切 |
| 38 | `POST …/:id/approve` | 刪（範圍裁切） | 不對應：範圍裁切 |
| 39 | `POST …/:id/reject` | 刪（範圍裁切） | 不對應：範圍裁切 |
| 40 | `POST …/:id/revoke` | 刪（範圍裁切） | 不對應：範圍裁切 |
| 41 | `POST …/:id/reverify` | 刪（範圍裁切） | 不對應：範圍裁切 |
| 42 | `GET /admin/categories/tree` | 刪（D8，CD11） | 不對應：唯一呼叫端 category-audit 頁隨 D8 刪 |
| 43 | `POST /admin/categories` | 刪（D8） | 不對應：分類不做即時 CRUD |
| 44 | `PATCH /admin/categories/:id` | 刪（D8） | 不對應：同上 |
| 45 | `DELETE /admin/categories` | 刪（D8） | 不對應：同上 |
| 46 | `POST /admin/categories/:id/restore` | 刪（D8） | 不對應：同上 |
| 47 | `GET /admin/categories/:id/products` | 刪（D5） | 不對應：無 client |
| 48 | `POST /admin/products/batch-change-category` | 刪（D5） | 不對應：無 client |
| 49 | `GET /public/search/products` | 改寫（D15） | P `SearchProducts`（`sort_by`／`sort_order` 的 price、created_at 對應 `ProductSort`） |
| 50 | `GET /public/search/suggest` | 改寫（D15） | P `SuggestProductNames` |
| 51 | `GET /public/sellers/:id/products` | 改寫 | P `ListProducts`（`filter.store_ids` 一家） |
| 52 | `GET /app/share_url`（type=product） | 保留（不在 catalog） | 不對應：share 路由不在 catalog，product 分支改讀 catalog 模組 API（Go 呼叫，不是 Connect RPC） |
| 53 | `GET /admin/sellers/:store_id/products` | 改寫（B4-5d） | 不對應（本步）：`CatalogDelegatedService`，B4-5d |
| 54 | `POST …/products` | 改寫（B4-5d） | 不對應（本步）：同上 |
| 55 | `POST …/products/batch-unpublish` | 改寫（B4-5d） | 不對應（本步）：同上 |
| 56 | `GET …/products/:product_id` | 改寫（B4-5d） | 不對應（本步）：同上 |
| 57 | `PATCH …/products/:product_id` | 改寫（B4-5d） | 不對應（本步）：同上 |
| 58 | `POST …/products/:product_id/publish` | 改寫（B4-5d） | 不對應（本步）：同上 |
| 59 | `POST …/products/:product_id/unpublish` | 改寫（B4-5d） | 不對應（本步）：同上 |

統計：對應到 B4-3 RPC 的 20 條（1、2、3、7、8、12、14、16、17、18、19、20、21、22、28、31、33、49、50、51）；不對應 39 條，按原因：

| 原因 | 列號 | 條數 |
|---|---|---|
| D5 無 client | 4、5、6、9、10、11、13、15、30、32、47、48 | 12 |
| 範圍裁切（認證） | 23、24、25、26、35、36、37、38、39、40、41 | 11 |
| D8 分類（賣家不建、不做即時 CRUD、審核頁刪） | 27、42、43、44、45、46 | 6 |
| D10 運費試算 | 29 | 1 |
| B4-5 限制措施 | 34 | 1 |
| 不在 catalog（share） | 52 | 1 |
| B4-5d 代行 | 53–59 | 7 |
| 合計 | | 39 |

12＋11＋6＋1＋1＋1＋7＝39；對應 20；20＋39＝59＝52＋7。

### 3.2 舊 gRPC（`interface-inventory.md` §1，46 支，逐支）

| 舊 RPC | 盤點去向 | 新 RPC 或不在 B4-3 Connect 的理由 |
|---|---|---|
| `CreateProduct` | 改寫 | S `CreateProduct` |
| `UpdateProduct` | 改寫 | S `UpdateProduct` |
| `GetProduct` | 改寫 | S `GetMyProduct` |
| `GetPublicProduct` | 改寫 | P `GetProduct` |
| `GetProductList` | 改寫並拆開 | P `ListProducts`、P `SearchProducts`、S `ListMyProducts`、A `ListProducts`；procurement 的搜尋改接模組 Go API（B4-4，不是 Connect） |
| `GetProductForAdmin` | 改寫 | A `GetProduct` |
| `UpdateProductStatus` | 改寫（B4-5） | 不在本步：商品限制措施 |
| `UpdateSellerProductStatus` | 改寫 | S `SetProductStatus` |
| `BatchGetSellerProductsForDelegation` | 改寫（B4-5d） | 不在本步：`CatalogDelegatedService` |
| `BatchUpdateSellerProductStatus` | 改寫（B4-5d；賣家自己的批次路徑無 client 不做） | 不在本步：`CatalogDelegatedService`；賣家端不開批次 RPC（D5） |
| `DeleteProduct` | 改寫 | S `DeleteProduct` |
| `CalculateShippingOptions` | 刪（D10、D11） | 不對應 |
| `PurgeProductCache` | 刪（D5） | 不對應 |
| `GetRelatedProducts` | 刪（D5） | 不對應 |
| `ListProductsByCategoryForAdmin` | 刪（D5） | 不對應 |
| `BatchChangeProductCategoryForAdmin` | 刪（D5） | 不對應 |
| `SearchHashtags` | 改寫 | P `SuggestHashtags` |
| `GetPopularHashtags` | 改寫 | P `SuggestTrendingHashtags` |
| `GetPublicOriginsSummary` | 刪（D5） | 不對應 |
| `GetPublicSellerCitiesSummary` | 刪（D5） | 不對應 |
| `DeductStock` | 改寫 | 不在 Connect：`legacy/catalogserver`（B4-4，D1）；模組 API 已存在 |
| `RestoreStock` | 改寫 | 不在 Connect：同上 |
| `GetStock` | 刪（D5，CD8） | 不對應 |
| `GetProductSimple` | 改寫 | 不在 Connect：`legacy/catalogserver`（B4-4），B6 刪 |
| `BatchGetSkuCartInfo` | 改寫 | 不在 Connect：同上 |
| `BatchGetProductForCheckout` | 改寫 | 不在 Connect：結帳快照 API（D20）經 `legacy/catalogserver`（B4-4） |
| `GetProductsByIDs` | 改寫 | 不在 Connect：模組 Go API 批次卡片（B4-4，seller、social、buyer 的 port） |
| `ListStoreIDsWithPublicProductsInternal` | 改寫 | 不在 Connect：模組 Go API（B4-4，discovery 排名） |
| 認證 11 支：`VerifyCertification`、`SubmitProductCertification`、`ListProductCertificationApplications`、`WithdrawProductCertification`、`ListProductCertificationApplicationsForAdmin`、`GetProductCertificationApplicationForAdmin`、`ListProductCertificationAuditLogsForAdmin`、`ApproveProductCertificationForAdmin`、`RejectProductCertificationForAdmin`、`RevokeProductCertificationForAdmin`、`ReverifyProductCertificationForAdmin` | 刪（範圍裁切） | 不對應 |
| `CategoryService.GetCategoryTree` | 改寫 | P `ListCategories`；procurement 與 ai-service 見 B4-4、D12 |
| `CategoryService.GetChildCategories` | 刪（D5） | 不對應 |
| `CategoryService.CreateCategory` | 刪（D8） | 不對應 |
| `CategoryService.CreateCategoryForAdmin` | 刪（D8） | 不對應 |
| `CategoryService.UpdateCategoryForAdmin` | 刪（D8） | 不對應 |
| `CategoryService.BatchSoftDeleteCategoryForAdmin` | 刪（D8） | 不對應 |
| `CategoryService.RestoreCategoryForAdmin` | 刪（D8） | 不對應 |

列數核對：39 支 ProductService（20＋8＋認證 11）加 CategoryService 7 支＝46；上表把認證 11 支併成一列，其餘每支一列。

### 3.3 沒有舊 RPC 對應的新 RPC

| 新 RPC | 來源 |
|---|---|
| S `AdjustStock` | 計畫 D23（賣家調整庫存，模組已實作 `AdjustStock`） |
| S `CountLowStockProducts` | 計畫 B4-3 範圍「低庫存數 API」與範圍段「賣家中心低庫存數由伺服器提供」；現況是 website 抓 100 筆商品在 client 數（`SellerCenterShell.tsx:128-135`【量】） |
| P `SearchProducts`、P `SuggestProductNames` | 舊路由 49、50（舊 handler 都打 `GetProductList`，這裡依 D15 拆成搜尋與建議） |

## 4. 路由以外的 B4-3 工作（非 proto，列出來避免漏掉）

計畫 B4-3 範圍還有：收藏數訂閱（buyer in-process 事件，同 transaction）、review 呼叫 `SetRating`（傳絕對值）、銷量入口（`order_no` 冪等）、`product.restock` 改 in-process 事件給 buyer、`adminauth` 宣告。這些不產生 proto，不在本稿範圍。

## 5. 模組側目前缺的（proto 依賴它們才能實作）

對照 `catalog.go` 現況【量】，下列 proto 欄位與方法目前模組沒有，B4-3 實作要補（各是實作細節，不是決策，但影響工作量）：

1. **etag**：`Product` 沒有 `version` 欄，`UpdateProduct`／`SetProductStatus`／`DeleteProduct` 沒有 etag 參數（`grep -n "version\|etag" db/schema.sql internal/model/*.go` 無命中【量】）。需要 schema migration 加 `products.version`、寫入用條件式 `UPDATE … WHERE version = $n`，`AdjustStock` 要不要讓 etag 變動見 Q5 附註。
2. **`request_id` 冪等**：`CreateProduct` 沒有冪等鍵；`idempotency_request` 表不帶（D4），要有存放處（例如 `products` 加 `(store_id, request_id)` 唯一）。`AdjustStock` 的 `key` 參數已存在（賣家調整依 `store_id` 在店家內唯一，`catalog.md` §3【量】）。
3. **列表篩選與排序**：`ListPublicProducts` 目前只有 `categoryID`、依首次上架排序；缺 `category_ids`（含頂層展開）、價格、hashtag、`store_ids`、收藏／追蹤 ID 集合（經 buyer API）、五種排序與 keyset 游標的對應。`ListSellerProducts` 缺 `status`、`low_stock_only`、`total_stock`。後台列表與 `total_size` 沒有。
4. **搜尋**：`searchable_text` 欄位已有，沒有查詢（`catalog.md` §6「還沒有：搜尋與 hashtag 彙總」【量】）；`SuggestProductNames`、`SuggestHashtags`、`SuggestTrendingHashtags` 同。
5. **低庫存數**、**`AdjustStock` 回傳調整後的 SKU**（目前只回 `error`）。
6. **限制措施排除**（B4-5）：本步公開讀取不含，與 `catalog.md` §6 一致。

## 6. 未確認的細節（提案值或沒驗證）

- 清單欄位上限（`category_ids` 20、`hashtags` 10、`store_ids` 50、`query` 100 字）是我提的，來源只有：`hashtags` 10 對齊商品最多 10 個（`catalog.md` §5【量】）；其餘【未查證】沒有 client 或 SSOT 依據。`store_ids` 舊 handler 沒有上限。
- `limits` 自訂 option 是否能掛在這些請求欄位、`uitext-gen` 會不會因此生成不需要的常數：沒跑 `make uitext`。
- 本稿的 proto 沒有經過 `buf lint`：`HashtagMatch` 與 `ProductSort` 的值前綴、`ProductSummary` 在前台與後台兩個 package 同名（規範 §一允許同名）、`optional float` 與 `field_behavior` 並用，都應該合法但沒跑。
- 相關度排序的 keyset 分頁（`SearchProducts` 的 `RELEVANCE`）：游標要含相似度分數與 ID；`pg_trgm` 的 `similarity` 對同一查詢是確定的，但分頁途中新增商品會改變排名邊界。規範 §六只要求「翻頁途中有新增或刪除也不會重複或漏掉」，這點對相關度排序是否成立沒驗證【未查證】，實作時要有測試。
- 「熱門 hashtag」的商品數定義（distinct ACTIVE 商品、只算可曝光店家）依 IF §1.1 與舊 SQL 註解【量】，Java 的 `PopularHashtagService` 其他細節（區間端點、同分排序）沒讀。

## 7. 開放問題（來源沒有決定、有真實取捨）

**Q1　service 命名與數量**

- 事實：計畫 B4-3 寫「D4 的三支 service」；簡報 D4 選項 A 的文字列了「公開、買家、賣家、後台、代行各一支」。買家的收藏、追蹤、補貨訂閱已在 `BuyerAppService`（`buyer.proto:53-137`【量】），catalog 沒有只給買家的寫入。賣家 service 的名字：規範 §二表格寫 `<模組>AppService`，但前例有 `SellerProcurementAppService`、`SellerReportAppService`。
- 選項：(a) 三支，賣家那支叫 `SellerCatalogAppService`；(b) 三支，叫 `CatalogAppService`；(c) 另加 `BuyerCatalogAppService`（放「追蹤店家的商品」之類登入才有的讀取，見 Q2）。
- 建議 (a)：名字直接表示身分（規範 §二第 34 點「服務名稱本身就表示以哪個身分呼叫」），與 procurement、report 前例一致；本稿以 (a) 起草。

**Q2　`followed_product_only`、`followed_seller_only` 放在哪**

- 事實：IF §1.1 寫「保留 client 送的…已收藏／已追蹤篩選，經 buyer 模組 API」；website 的「追蹤中」分頁送 `followed_seller_only`（`deriveListView.ts:203`【量】），`followed_product_only` 在 website 型別裡有定義但我只找到測試與 `index.ts` 的參數【量】，buyer-app 沒有用。`BuyerAppService.ListFavorites` 已能列收藏商品卡片。規範 §九：依觀看者的 Public 讀取，帶憑證時 `private, no-store`，不進共享快取。
- 選項：(a) 兩個布林留在 `ListProducts`，該 RPC 宣告 viewer-dependent：登入買家的所有商品列表都不能被 CDN 快取；(b) 只保留 `followed_seller_only`，`followed_product_only` 刪掉（`ListFavorites` 已涵蓋）；(c) 把「追蹤中的店家的商品」拆成另一個 RPC（Q1 的 (c)），`ListProducts` 維持不依觀看者。
- 建議 (c) 或至少 (b)：(a) 讓最熱的列表 RPC 失去共享快取，而這兩個篩選只有一個畫面在用。但 (c) 偏離盤點寫的「保留」，要你決定；本稿草稿寫成 (a)（照盤點文字）。

**Q3　`UpdateProduct` 用整體取代還是 `update_mask`**

- 事實：規範 §七 的 `Update` 用 `update_mask`，沒帶時「視為列出所有非零值的欄位」，所以清空要列出來。既有 `ProductInput` 註解寫「修改是整體取代」（B4-2b 已定，模組 `UpdateProduct` 也是整體取代【量】）；`axes`、`skus`、`gallery` 本來就是整批取代語意（規範 §七「repeated 欄位整批替換」）。
- 選項：(a) 沒有 mask，請求就是 `ProductInput` 全量加 etag（本稿）；(b) 加 `update_mask`，transport 先讀目前資源、套 mask、再整筆交給模組（規範 §七 第 141 點的做法）。
- 建議 (a)：seller-app 與 website 的商品表單每次送完整表單；mask 只在 client 想送局部欄位時有用，而 `ProductInput` 的 `title`、`category_id`、`skus` 是必填，局部更新沒有消費者。要寫進規範 §七 作為「整體取代的資源不帶 mask」的註記。

**Q4　`request_id` 的範圍與必填**

- 事實：簡報 D4 寫「寫入用 `request_id` 冪等，取代 `idempotency_request` 表」；舊系統只有 `CreateProduct` 必填 idempotency key（IF §1.1 第 30 列），`UpdateProduct` 用 `version`。repo 前例：social `CreatePost` 的 `request_id` 選填（`social.proto:654`），代行與 admin 的建立大多選填、代行 session 內必填（`social.proto:860`）。
- 選項：(a) 只有 `CreateProduct` 與 `AdjustStock` 帶，兩者必填；更新、上下架、刪除靠 etag（本稿）；(b) 兩者選填（照 social 前例）；(c) 每個寫入都帶。
- 建議 (a)：這兩個是重送會重複效果的動作（重複建商品、重複加減庫存），舊系統的建立也是必填；更新與上下架重送時 etag 已經擋（回 `VERSION_CONFLICT`）。選填會讓 `AdjustStock` 的重送在沒帶 id 時重複扣加庫存。

**Q5　`SetProductStatus` 的 etag 必填與否（以及要不要讓列表帶 etag）**

- 事實：規範 §七：改變狀態的自訂方法，若決定依據的內容在狀態不變時仍可能被改，etag 必填；若只依據狀態機已經檢查的狀態，不帶。D22 的狀態機任兩狀態互切（沒有轉移檢查），但上架的前置條件是「至少一張主圖」，圖片是可變內容。模組在鎖列後的同一個 transaction 內檢查這個條件【量，`catalog.md` §5】，所以沒有「用舊資料判斷」的風險。
- 選項：(a) etag 必填（本稿），列表的 `ProductSummary` 因此要帶 etag 才能在列表上直接上下架；(b) etag 選填或不帶。
- 建議 (a)：照規範字面；成本只是列表多一個欄位。附註：`AdjustStock` 不改商品內容，建議不讓它更動 etag（庫存是帳本，與內容正交），否則賣家編輯商品時每次進貨都造成 `VERSION_CONFLICT`。

**Q6　低庫存的定義**

- 事實：現況兩處寫死不同的數字：website 導覽徽章與商品頁用 5（`SellerCenterShell.tsx:128-135`、`ProductsPage.tsx:603`），庫存頁預設 10（`InventoryPage.tsx` `SAFE_STOCK_DEFAULT`），買家卡片也用 10（`MarketProductCard.tsx:47`）；商品表單有「低庫存提醒門檻」欄位（`ProductFormSections.tsx:214`），但新 schema 沒有門檻欄位，舊列表也不回它（`InventoryPage.tsx:43-46` 註解）。現況是用商品層的 `stock` 數，新模型庫存在 SKU。
- 選項：(a) 伺服器常數門檻 5，回應帶 `threshold`；「偏低」＝至少一個沒刪除 SKU 庫存 ≤ 門檻，任何狀態（本稿）；(b) 請求帶 `stock_at_most`，伺服器無狀態，client 自己決定 5 或 10；(c) 每商品（或每 SKU）存門檻（新欄位、表單接上）。
- 建議 (a)：與「低庫存數由伺服器提供」的意圖一致，不新增資料；(c) 要新增欄位與 UI，沒有決策支持。代價：庫存頁的 10 與表單的門檻欄位要改（UI 改動，需你同意）。另一個子問題：計數單位用「商品數」還是「SKU 數」、要不要只算 ACTIVE：沿用現況（商品數、任何狀態）。

**Q7　把舊端點併進別的 RPC**

- 事實：`/public/products/:id/brief` 的唯一呼叫（`getSkuImageUrl`）依 gateway 註解是無人呼叫的方法；`/public/categories/:id/path` 盤點寫「兩層，B4-3 定是否另開方法」；舊商品詳情回 `isFavorited`、`isRestockSubscribed`（`product.go:786-788`【量】），買家 service 已有 `CheckFavorite`、`BatchCheckFavorites`、`ListRestockSubscriptions`。
- 選項：(a) 不另開 brief 與 path，`GetProduct` 不帶觀看者狀態，client 另呼叫買家 service（本稿）；(b) `GetProduct` 帶 `viewer_favorited`、`viewer_restock_subscribed`，該 RPC 宣告 viewer-dependent；(c) 另開 `GetProductPath`。
- 建議 (a)：商品頁是流量最大的公開讀取，維持不依觀看者才能走共享快取（規範 §九）；路徑由 62 個節點的樹推得。代價：website `ProductPurchaseBox`、`ProductDetailLayout` 與 buyer-app 的商品頁要多一次買家 RPC（B4-6 改 client 時處理）。

**Q8　後台列表的頁碼與總數**

- 事實：後台商品頁用頁碼分頁（`currentPage`）並顯示總數（`products/page.tsx:72-104`【量】），後台首頁與分析頁只取 `total`（`admin/page.tsx:45`、`analytics/page.tsx:66`）。規範 §六：一律 keyset，「需要總數時才加 `total_size`」。
- 選項：(a) keyset 加 `total_size`，後台商品頁改上一頁／下一頁（本稿，UI 改動）；(b) 為後台列表例外使用 `page`／offset（違反規範 §六，要先修訂規範）；(c) 只提供 `total_size`，不分頁的首頁用 `page_size=1` 取總數，商品頁維持 keyset。
- 建議 (a)：規範沒有為後台開例外；後台商品頁的使用者是營運，上一頁／下一頁足夠。UI 流程改動依根 `AGENTS.md`「UI 改動」要先取得同意。

**Q9　`ListProducts` 與 `SearchProducts` 要不要合併**

- 事實：舊 `/public/products` 同時收 `keyword` 與全部篩選（website 市集頁帶關鍵字，`website/lib/api/index.ts:140-175`【量】），`/public/search/products` 只收 keyword、category、sort。規範 §三：`List` 列舉集合、`Search` 是符合條件的全部結果，兩者都分頁。
- 選項：(a) 兩個 RPC 共用 `ProductFilter`；有關鍵字走 `SearchProducts`、排序多一個 `RELEVANCE`（本稿）；(b) 一個 `ListProducts`，`query` 選填，有 `query` 時預設相關度排序。
- 建議 (a)：D15 把搜尋與列表的排序語意分開（相似度加銷量 vs 單一鍵），分頁游標的組成不同；合併會讓 `sort` 的合法值依 `query` 有沒有而變。代價是 website 市集頁在有關鍵字時改呼叫另一支。

## 8. 我跑過的指令（推導對照用）

所有指令從 repo 根目錄執行，唯讀。

- `git rev-parse --show-toplevel`；`wc -l` 於兩份 `AGENTS.md`、計畫、附件各檔、規範、`catalog.proto`；`ls api/hawkr api/hawkr/*/v1alpha1`（找前例 service 與 package 佈局）。
- 讀：`AGENTS.md`、`backend/AGENTS.md`、`backend/docs/plans/2026-11-24-b4-catalog.md` 全文；`decision-brief.md` 的 D3、D4、D5、D14–D20（`sed -n 55,78p`、`128,181p`）；`followup-brief.md`；`limits-status-brief.md` 的 D21、D22；`interface-inventory.md`、`client-inventory.md` 全文；規範全文；`api/hawkr/catalog/v1alpha1/catalog.proto`、`common/v1alpha1/{limits,image,store_card,error_reason}.proto`；`backend/docs/modules/catalog.md`；`backend/internal/modules/catalog/{catalog.go,AGENTS.md,internal/model/*.go,db/schema.sql}`。
- 前例：`api/hawkr/social/v1alpha1/social.proto`（App 的賣家能力、`request_id`、`etag`、`SuggestTags`）、`api/hawkr/admin/content/v1alpha1/content.proto`（後台 etag、檔頭寫法）、`rg -n "request_id" api/hawkr --glob '*.proto'`、`rg -n "service \w+Service" api/hawkr --glob '*.proto'`。
- 授權：`rg -n "adminauth|adminperm|audited" backend/internal --glob '*.go' -l`、`sed -n 25,66p backend/internal/httpapi/adminauth/adminauth.go`、`sed -n 1,60p backend/internal/platform/adminperm/adminperm.go`、`backend/internal/httpapi/seller/declaration.go`、`backend/internal/modules/seller/internal/model/capability.go`、`backend/internal/httpapi/viewer.go`。
- 路由盤點：用一段 `python3`（stdin，不落檔）解析 `client-inventory.md` §1a 與 §2 的表列，得 52＋7＝59 列並逐列填對照；gRPC 46 支用 `interface-inventory.md` §1.1–§1.4 逐列核對（20＋8＋11＋7）。
- client 實際用到的參數：`rg -o 'Query\("…"\)|DefaultQuery\("…"' backend/gateway/internal/handler/{product,search,seller_product,admin_product,hashtag,seller}.go`；`sed -n` 於 `website/lib/api/index.ts`、`website/lib/search/repo.ts`、`website/lib/products/seller-repo.ts`、`seller-app/.../product_repository.dart`、`buyer-app/.../{product,category}_repository.dart`、`admin/src/app/admin/{page,products/page,merchants/[id]/page}.tsx`、`website/features/seller-center/{components/SellerCenterShell,inventory/InventoryPage,products/ProductsPage}.tsx`；`rg -n "followed_product_only|followed_seller_only|startFrom|endTo|low.?stock" website buyer-app seller-app admin`。
- Java 對照：`rg -n "startFrom|endTo|created_at" …/PopularHashtagService.java …/ProductHashtagRepository.java`。

## 9. 摘要

- 3 支 service、17 個 RPC（Public 7、賣家 8、後台 2）。
- 59 條舊路由：對應 20 條；不對應 39 條（D5 無 client 12、裁切認證 11、D8 分類 6、D10 運費 1、B4-5 措施 1、不在 catalog 1、B4-5d 代行 7）。
- 46 支舊 gRPC：對應 B4-3 Connect RPC 11 支；進 Go 模組 API 或 `legacy/catalogserver`（B4-4）7 支；B4-5、B4-5d 3 支；刪除 25 支（11＋7＋3＋25＝46）。
- 需要你決定的：Q1–Q9（第 7 節）；其中 Q2、Q6、Q8 會連動 UI 改動。
