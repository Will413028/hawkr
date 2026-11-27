//
//  Generated code. Do not modify.
//  source: hawkr/catalog/v1alpha1/catalog.proto
//

import "package:connectrpc/connect.dart" as connect;
import "catalog.pb.dart" as hawkrcatalogv1alpha1catalog;

/// CatalogPublicService 是任何人都能讀的商品、搜尋與分類，不需要登入，回應不依觀看者（可共享快取）。
/// 公開可讀的商品：沒刪除、店家此刻可曝光（seller.pub_exposable_stores，時間用請求的 clock）、狀態是 ACTIVE 或 ARCHIVED；
/// 只有 ACTIVE 的出現在列表與搜尋，ARCHIVED 的商品頁讀得到但 availability 是不可購買。其他一律與不存在相同（not_found）。
/// 公開訊息不帶賣家的狀態 enum，只帶 availability。
abstract final class CatalogPublicService {
  /// Fully-qualified name of the CatalogPublicService service.
  static const name = 'hawkr.catalog.v1alpha1.CatalogPublicService';

  /// ListProducts 是買家瀏覽的商品列表（分類頁、市集、店家的商品頁）；不帶關鍵字，關鍵字搜尋用 SearchProducts。filter 的欄位全部選填，之間是 AND。
  /// 要看收藏的商品用 BuyerAppService.ListFavorites，追蹤的店家的商品用 BuyerAppService.ListFollowedStoreProducts。
  static const listProducts = connect.Spec(
    '/$name/ListProducts',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.ListProductsRequest.new,
    hawkrcatalogv1alpha1catalog.ListProductsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SearchProducts 依關鍵字找商品（標題、描述文字、標籤、分類名稱與選項的 searchable_text，pg_trgm），預設依相關度排序。
  static const searchProducts = connect.Spec(
    '/$name/SearchProducts',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.SearchProductsRequest.new,
    hawkrcatalogv1alpha1catalog.SearchProductsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SuggestProductNames 是搜尋框輸入時的建議：商品名稱符合的前幾筆，不分頁，最多 20 筆。
  static const suggestProductNames = connect.Spec(
    '/$name/SuggestProductNames',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.SuggestProductNamesRequest.new,
    hawkrcatalogv1alpha1catalog.SuggestProductNamesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetProduct 讀一個公開可見的商品的完整內容（含 SKU 與庫存）。ACTIVE 與 ARCHIVED 讀得到（ARCHIVED 的 availability 是不可購買）；
  /// 不存在、草稿、已刪除、店家不可曝光回 not_found。不帶觀看者狀態（是否已收藏、是否訂閱補貨）：那些由 BuyerAppService 提供。
  static const getProduct = connect.Spec(
    '/$name/GetProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.GetProductRequest.new,
    hawkrcatalogv1alpha1catalog.GetProductResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListCategories 回整棵平台分類樹（兩層，頂層在前，同層依平台排序）。分類只在 migration 變動，不分頁。
  /// 名稱依 Accept-Language 協商；沒有該語言時是 zh-TW。
  static const listCategories = connect.Spec(
    '/$name/ListCategories',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.ListCategoriesRequest.new,
    hawkrcatalogv1alpha1catalog.ListCategoriesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SuggestHashtags 是商品標籤輸入與搜尋的建議：名稱符合的標籤與其商品數（只算可曝光店家的 ACTIVE 商品），商品數多的在前。沒有分頁，最多 20 筆。
  static const suggestHashtags = connect.Spec(
    '/$name/SuggestHashtags',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.SuggestHashtagsRequest.new,
    hawkrcatalogv1alpha1catalog.SuggestHashtagsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SuggestTrendingHashtags 是熱門商品標籤：各標籤的 ACTIVE 商品數（只算可曝光店家），多的在前。只取前 max_results 名，不分頁。
  static const suggestTrendingHashtags = connect.Spec(
    '/$name/SuggestTrendingHashtags',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.SuggestTrendingHashtagsRequest.new,
    hawkrcatalogv1alpha1catalog.SuggestTrendingHashtagsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
/// SellerCatalogAppService 是賣家管理自己店家的商品。店家由登入身分推導，請求不帶 store_id；
/// 商品不是這家店的、或已刪除，與不存在一樣回 not_found（規範 §二 防枚舉）。每個方法需要賣家能力 CAP_PRODUCT_MANAGE。
/// etag 只涵蓋賣家可編輯的內容與狀態：庫存、銷量、收藏數、評分、首次上架時間不改它，所以調整庫存或有人下單不會讓編輯中的表單衝突。
abstract final class SellerCatalogAppService {
  /// Fully-qualified name of the SellerCatalogAppService service.
  static const name = 'hawkr.catalog.v1alpha1.SellerCatalogAppService';

  /// ListMyProducts 列出自己店家沒刪除的商品，任何狀態，不看店家曝光；最新建立的在前。
  static const listMyProducts = connect.Spec(
    '/$name/ListMyProducts',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.ListMyProductsRequest.new,
    hawkrcatalogv1alpha1catalog.ListMyProductsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetMyProduct 讀自己店家的商品完整內容（任何狀態，含 etag 與每個 SKU 的庫存）。
  static const getMyProduct = connect.Spec(
    '/$name/GetMyProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.GetMyProductRequest.new,
    hawkrcatalogv1alpha1catalog.GetMyProductResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CountMyProducts 回自己店家符合篩選的商品數（導覽列的低庫存徽章用 status = ACTIVE、stock_status = LOW）。
  static const countMyProducts = connect.Spec(
    '/$name/CountMyProducts',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.CountMyProductsRequest.new,
    hawkrcatalogv1alpha1catalog.CountMyProductsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CreateProduct 建立一個草稿商品（建立一律是草稿；要上架呼叫 SetProductStatus）。
  /// 同一家店以同一個 request_id 重送回第一次建立的商品目前的樣子，不重複建立；內容不同回 invalid_argument。
  static const createProduct = connect.Spec(
    '/$name/CreateProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.CreateProductRequest.new,
    hawkrcatalogv1alpha1catalog.CreateProductResponse.new,
  );

  /// UpdateProduct 整體取代商品的內容（狀態不變；等同 AIP-134 的 update_mask = "*"，沒有 update_mask）：沒有帶的圖片、選項軸與 SKU 組合會被移除；既有 SKU 的庫存不在這裡改（用 AdjustStock、SetStock）。
  /// etag 必填（在請求上，因為寫入的 ProductInput 與讀取的 SellerProduct 是不同的型別），與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。商品目前是 ACTIVE 時，輸入至少要有一張主圖，否則回 failed_precondition。
  static const updateProduct = connect.Spec(
    '/$name/UpdateProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.UpdateProductRequest.new,
    hawkrcatalogv1alpha1catalog.UpdateProductResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// SetProductStatus 切換商品狀態：草稿、上架、下架任兩個都能互切，切成目前的狀態也回成功（不改 etag）。
  /// 切成 ACTIVE 時商品至少要有一張主圖，否則回 failed_precondition。首次上架寫入 first_publish_time，之後不清除。
  /// etag 必填（上架是把看到的內容公開，內容在狀態不變時仍可能被別人改動，規範 §七）。
  static const setProductStatus = connect.Spec(
    '/$name/SetProductStatus',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.SetProductStatusRequest.new,
    hawkrcatalogv1alpha1catalog.SetProductStatusResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeleteProduct 刪除商品：從賣家清單消失、不可還原，列保留（訂單與評價仍引用）。已經刪除的也回成功。etag 選填，帶了就檢查。
  static const deleteProduct = connect.Spec(
    '/$name/DeleteProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.DeleteProductRequest.new,
    hawkrcatalogv1alpha1catalog.DeleteProductResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// AdjustStock 以有號變動量調整自己店家某個 SKU 的庫存（進貨加、出貨減）。
  /// 同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。調整後不得小於 0，否則回 failed_precondition。不改商品的 etag。
  static const adjustStock = connect.Spec(
    '/$name/AdjustStock',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.AdjustStockRequest.new,
    hawkrcatalogv1alpha1catalog.AdjustStockResponse.new,
  );

  /// SetStock 把自己店家某個 SKU 的庫存設成絕對值（盤點、商品表單輸入庫存），以 expected_quantity 比對現值：
  /// 現值不等於預期（期間有訂單扣減或別處調整）回 failed_precondition，不寫入，client 重讀後再決定。
  /// 同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。不改商品的 etag。
  static const setStock = connect.Spec(
    '/$name/SetStock',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.SetStockRequest.new,
    hawkrcatalogv1alpha1catalog.SetStockResponse.new,
  );
}
/// CatalogDelegatedService 是代行營運中的平台管理員讀寫代行店家的商品：憑證是 X-HK-Operation-Session 標頭裡的代行 session（由 website 的 BFF 轉送，不放在請求訊息），
/// 不讀顧客或管理員的身分；目標店家是 session 建立當下綁定的那一家，請求不帶 store_id。回應一律不可快取（private, no-store）。
/// 商品、狀態與內容的規則與 SellerCatalogAppService 相同（同一組模組方法），回應也用同一組賣家訊息（SellerProduct、SellerProductSummary）。
/// 能力（products.read、products.write、products.publish）依方法宣告：憑證缺少或無效回 unauthenticated；session 已結束、到期或撤銷回 failed_precondition；
/// session 沒有授予對應的能力回 permission_denied；店家此刻不可代行（不存在、已關閉、等待刪除、停權）回 failed_precondition，只拒絕、不撤銷 session。
/// 稽核：成功的寫入在同一個 transaction 內留下一列 SUCCEEDED（寫入失敗時一併 rollback）；被拒絕的請求（含讀取）與寫入的業務失敗另開 transaction 留下一列 FAILED；
/// 成功的讀取不稽核。
abstract final class CatalogDelegatedService {
  /// Fully-qualified name of the CatalogDelegatedService service.
  static const name = 'hawkr.catalog.v1alpha1.CatalogDelegatedService';

  /// ListStoreProducts 列出代行店家沒刪除的商品，任何狀態，不看店家曝光；最新建立的在前（keyset 翻頁）。需要能力 products.read。
  static const listStoreProducts = connect.Spec(
    '/$name/ListStoreProducts',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.ListStoreProductsRequest.new,
    hawkrcatalogv1alpha1catalog.ListStoreProductsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetStoreProduct 讀代行店家的商品完整內容（任何狀態，含 etag）；別家的商品與不存在一樣回 not_found。需要能力 products.read。
  static const getStoreProduct = connect.Spec(
    '/$name/GetStoreProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.GetStoreProductRequest.new,
    hawkrcatalogv1alpha1catalog.GetStoreProductResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CreateStoreProduct 以代行店家建立草稿商品（建立一律是草稿；要上架再呼叫 PublishStoreProduct）。同一個 session 以同一個 request_id 重送回第一次建立的商品目前的樣子，
  /// 不重複建立、不重複寫稽核；內容不同回 failed_precondition。需要能力 products.write。
  static const createStoreProduct = connect.Spec(
    '/$name/CreateStoreProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.CreateStoreProductRequest.new,
    hawkrcatalogv1alpha1catalog.CreateStoreProductResponse.new,
  );

  /// UpdateStoreProduct 修改代行店家的商品：update_mask 列 ProductInput 最上層欄位的子集，沒列的欄位保留目前的值；etag 必填，與目前版本不符回 aborted 與
  /// VERSION_CONFLICT，不寫入也不留成功的稽核。重送規則同 CreateStoreProduct。需要能力 products.write。
  static const updateStoreProduct = connect.Spec(
    '/$name/UpdateStoreProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.UpdateStoreProductRequest.new,
    hawkrcatalogv1alpha1catalog.UpdateStoreProductResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// PublishStoreProduct 把商品上架（切成 ACTIVE；商品至少要有一張主圖，否則回 failed_precondition）；etag 必填。商品已經是上架時回成功、不改 etag。重送規則同 CreateStoreProduct。需要能力 products.publish。
  static const publishStoreProduct = connect.Spec(
    '/$name/PublishStoreProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.PublishStoreProductRequest.new,
    hawkrcatalogv1alpha1catalog.PublishStoreProductResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnpublishStoreProduct 把商品下架（切成 ARCHIVED）；etag 必填。商品已經是下架時回成功、不改 etag。重送規則同 CreateStoreProduct。需要能力 products.publish。
  static const unpublishStoreProduct = connect.Spec(
    '/$name/UnpublishStoreProduct',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.UnpublishStoreProductRequest.new,
    hawkrcatalogv1alpha1catalog.UnpublishStoreProductResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// BatchUnpublishStoreProducts 原子地下架多個商品（AIP-233）：一個 request_id、一個 transaction，全部下架或全部不動；一筆稽核列（PRODUCT_BATCH_UNPUBLISH）記錄本批商品，
  /// 一則賣家通知。任一項的 etag 與目前版本不符，整批拒絕，回 aborted 與 VERSION_CONFLICT，ErrorInfo.metadata 的 conflictingProductIds 列出衝突的商品 ID
  /// （逗號連接、由小到大排列）；已經下架的項目視為成功（不改它的 etag）；任一項不是代行店家的商品回 not_found，整批不動。同一個 request_id 重送回第一次的結果。需要能力 products.publish。
  static const batchUnpublishStoreProducts = connect.Spec(
    '/$name/BatchUnpublishStoreProducts',
    connect.StreamType.unary,
    hawkrcatalogv1alpha1catalog.BatchUnpublishStoreProductsRequest.new,
    hawkrcatalogv1alpha1catalog.BatchUnpublishStoreProductsResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );
}
