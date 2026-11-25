//
//  Generated code. Do not modify.
//  source: hawkr/catalog/v1alpha1/catalog.proto
//

import "package:connectrpc/connect.dart" as connect;
import "catalog.pb.dart" as hawkrcatalogv1alpha1catalog;
import "catalog.connect.spec.dart" as specs;

/// CatalogPublicService 是任何人都能讀的商品、搜尋與分類，不需要登入，回應不依觀看者（可共享快取）。
/// 公開可讀的商品：沒刪除、店家此刻可曝光（seller.pub_exposable_stores，時間用請求的 clock）、狀態是 ACTIVE 或 ARCHIVED；
/// 只有 ACTIVE 的出現在列表與搜尋，ARCHIVED 的商品頁讀得到但 availability 是不可購買。其他一律與不存在相同（not_found）。
/// 公開訊息不帶賣家的狀態 enum，只帶 availability。
extension type CatalogPublicServiceClient (connect.Transport _transport) {
  /// ListProducts 是買家瀏覽的商品列表（分類頁、市集、店家的商品頁）；不帶關鍵字，關鍵字搜尋用 SearchProducts。filter 的欄位全部選填，之間是 AND。
  /// 要看收藏的商品用 BuyerAppService.ListFavorites，追蹤的店家的商品用 BuyerAppService.ListFollowedStoreProducts。
  Future<hawkrcatalogv1alpha1catalog.ListProductsResponse> listProducts(
    hawkrcatalogv1alpha1catalog.ListProductsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CatalogPublicService.listProducts,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SearchProducts 依關鍵字找商品（標題、描述文字、標籤、分類名稱與選項的 searchable_text，pg_trgm），預設依相關度排序。
  Future<hawkrcatalogv1alpha1catalog.SearchProductsResponse> searchProducts(
    hawkrcatalogv1alpha1catalog.SearchProductsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CatalogPublicService.searchProducts,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SuggestProductNames 是搜尋框輸入時的建議：商品名稱符合的前幾筆，不分頁，最多 20 筆。
  Future<hawkrcatalogv1alpha1catalog.SuggestProductNamesResponse> suggestProductNames(
    hawkrcatalogv1alpha1catalog.SuggestProductNamesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CatalogPublicService.suggestProductNames,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetProduct 讀一個公開可見的商品的完整內容（含 SKU 與庫存）。ACTIVE 與 ARCHIVED 讀得到（ARCHIVED 的 availability 是不可購買）；
  /// 不存在、草稿、已刪除、店家不可曝光回 not_found。不帶觀看者狀態（是否已收藏、是否訂閱補貨）：那些由 BuyerAppService 提供。
  Future<hawkrcatalogv1alpha1catalog.GetProductResponse> getProduct(
    hawkrcatalogv1alpha1catalog.GetProductRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CatalogPublicService.getProduct,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListCategories 回整棵平台分類樹（兩層，頂層在前，同層依平台排序）。分類只在 migration 變動，不分頁。
  /// 名稱依 Accept-Language 協商；沒有該語言時是 zh-TW。
  Future<hawkrcatalogv1alpha1catalog.ListCategoriesResponse> listCategories(
    hawkrcatalogv1alpha1catalog.ListCategoriesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CatalogPublicService.listCategories,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SuggestHashtags 是商品標籤輸入與搜尋的建議：名稱符合的標籤與其商品數（只算可曝光店家的 ACTIVE 商品），商品數多的在前。沒有分頁，最多 20 筆。
  Future<hawkrcatalogv1alpha1catalog.SuggestHashtagsResponse> suggestHashtags(
    hawkrcatalogv1alpha1catalog.SuggestHashtagsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CatalogPublicService.suggestHashtags,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SuggestTrendingHashtags 是熱門商品標籤：各標籤的 ACTIVE 商品數（只算可曝光店家），多的在前。只取前 max_results 名，不分頁。
  Future<hawkrcatalogv1alpha1catalog.SuggestTrendingHashtagsResponse> suggestTrendingHashtags(
    hawkrcatalogv1alpha1catalog.SuggestTrendingHashtagsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CatalogPublicService.suggestTrendingHashtags,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
/// SellerCatalogAppService 是賣家管理自己店家的商品。店家由登入身分推導，請求不帶 store_id；
/// 商品不是這家店的、或已刪除，與不存在一樣回 not_found（規範 §二 防枚舉）。每個方法需要賣家能力 CAP_PRODUCT_MANAGE。
/// etag 只涵蓋賣家可編輯的內容與狀態：庫存、銷量、收藏數、評分、首次上架時間不改它，所以調整庫存或有人下單不會讓編輯中的表單衝突。
extension type SellerCatalogAppServiceClient (connect.Transport _transport) {
  /// ListMyProducts 列出自己店家沒刪除的商品，任何狀態，不看店家曝光；最新建立的在前。
  Future<hawkrcatalogv1alpha1catalog.ListMyProductsResponse> listMyProducts(
    hawkrcatalogv1alpha1catalog.ListMyProductsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.listMyProducts,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetMyProduct 讀自己店家的商品完整內容（任何狀態，含 etag 與每個 SKU 的庫存）。
  Future<hawkrcatalogv1alpha1catalog.GetMyProductResponse> getMyProduct(
    hawkrcatalogv1alpha1catalog.GetMyProductRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.getMyProduct,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CountMyProducts 回自己店家符合篩選的商品數（導覽列的低庫存徽章用 status = ACTIVE、stock_status = LOW）。
  Future<hawkrcatalogv1alpha1catalog.CountMyProductsResponse> countMyProducts(
    hawkrcatalogv1alpha1catalog.CountMyProductsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.countMyProducts,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CreateProduct 建立一個草稿商品（建立一律是草稿；要上架呼叫 SetProductStatus）。
  /// 同一家店以同一個 request_id 重送回第一次建立的商品目前的樣子，不重複建立；內容不同回 invalid_argument。
  Future<hawkrcatalogv1alpha1catalog.CreateProductResponse> createProduct(
    hawkrcatalogv1alpha1catalog.CreateProductRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.createProduct,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdateProduct 整體取代商品的內容（狀態不變；等同 AIP-134 的 update_mask = "*"，沒有 update_mask）：沒有帶的圖片、選項軸與 SKU 組合會被移除；既有 SKU 的庫存不在這裡改（用 AdjustStock、SetStock）。
  /// etag 必填（在請求上，因為寫入的 ProductInput 與讀取的 SellerProduct 是不同的型別），與目前版本不符回 aborted 與 VERSION_CONFLICT，不寫入。商品目前是 ACTIVE 時，輸入至少要有一張主圖，否則回 failed_precondition。
  Future<hawkrcatalogv1alpha1catalog.UpdateProductResponse> updateProduct(
    hawkrcatalogv1alpha1catalog.UpdateProductRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.updateProduct,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SetProductStatus 切換商品狀態：草稿、上架、下架任兩個都能互切，切成目前的狀態也回成功（不改 etag）。
  /// 切成 ACTIVE 時商品至少要有一張主圖，否則回 failed_precondition。首次上架寫入 first_publish_time，之後不清除。
  /// etag 必填（上架是把看到的內容公開，內容在狀態不變時仍可能被別人改動，規範 §七）。
  Future<hawkrcatalogv1alpha1catalog.SetProductStatusResponse> setProductStatus(
    hawkrcatalogv1alpha1catalog.SetProductStatusRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.setProductStatus,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// DeleteProduct 刪除商品：從賣家清單消失、不可還原，列保留（訂單與評價仍引用）。已經刪除的也回成功。etag 選填，帶了就檢查。
  Future<hawkrcatalogv1alpha1catalog.DeleteProductResponse> deleteProduct(
    hawkrcatalogv1alpha1catalog.DeleteProductRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.deleteProduct,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// AdjustStock 以有號變動量調整自己店家某個 SKU 的庫存（進貨加、出貨減）。
  /// 同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。調整後不得小於 0，否則回 failed_precondition。不改商品的 etag。
  Future<hawkrcatalogv1alpha1catalog.AdjustStockResponse> adjustStock(
    hawkrcatalogv1alpha1catalog.AdjustStockRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.adjustStock,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SetStock 把自己店家某個 SKU 的庫存設成絕對值（盤點、商品表單輸入庫存），以 expected_quantity 比對現值：
  /// 現值不等於預期（期間有訂單扣減或別處調整）回 failed_precondition，不寫入，client 重讀後再決定。
  /// 同一家店以同一個 request_id 重送且內容相同只生效一次；內容不同回 invalid_argument。不改商品的 etag。
  Future<hawkrcatalogv1alpha1catalog.SetStockResponse> setStock(
    hawkrcatalogv1alpha1catalog.SetStockRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.SellerCatalogAppService.setStock,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
