import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/api_request_helper.dart';
import '../../../core/networking/dio_provider.dart';

part 'product_repository.g.dart';

class ProductRepository with ApiRequestHelper {
  ProductRepository(this._dio);
  final Dio _dio;

  @override
  Dio get dio => _dio;

  // ── 商品列表 ─────────────────────────────────────────

  Future<Map<String, dynamic>> getMyProducts({
    int page = 1,
    int size = 20,
    String? status,
  }) async {
    return apiGet(
      '/api/v1/app/products',
      queryParams: {
        'view': 'manage',
        'page': '$page',
        'page_size': '$size',
        if (status != null) 'status': status,
      },
    );
  }

  // ── 商品 CRUD ────────────────────────────────────────

  Future<Map<String, dynamic>> createProduct({
    required String idempotencyKey,
    required String name,
    required String categoryId,
    String? origin,
    String? description,
    List<Map<String, dynamic>>? descriptionBlocks,
    List<Map<String, dynamic>>? images,
    List<Map<String, dynamic>>? skus,
    List<Map<String, dynamic>>? certifications,
    String? status,
  }) async {
    Map<String, dynamic>? descriptionInput;
    if (descriptionBlocks != null && descriptionBlocks.isNotEmpty) {
      descriptionInput = {'version': '1.0', 'blocks': descriptionBlocks};
    } else if (description != null && description.isNotEmpty) {
      descriptionInput = {
        'version': '1.0',
        'blocks': [
          {'type': 'text', 'content': description},
        ],
      };
    }

    return apiPost(
      '/api/v1/app/products',
      body: {
        'idempotencyKey': idempotencyKey,
        'name': name,
        'categoryId': categoryId,
        if (origin != null && origin.isNotEmpty) 'origin': origin,
        if (descriptionInput != null) 'description': descriptionInput,
        if (images != null && images.isNotEmpty) 'images': images,
        if (skus != null && skus.isNotEmpty) 'skus': skus,
        if (certifications != null && certifications.isNotEmpty)
          'certifications': certifications,
        if (status != null) 'status': status,
      },
    );
  }

  Future<Map<String, dynamic>> updateProduct(
    String productId, {
    String? idempotencyKey,
    String? name,
    String? categoryId,
    String? origin,
    String? description,
    List<Map<String, dynamic>>? descriptionBlocks,
    List<Map<String, dynamic>>? images,
    List<Map<String, dynamic>>? skus,
    List<Map<String, dynamic>>? certifications,
    String? status,
  }) async {
    Map<String, dynamic>? descriptionInput;
    if (descriptionBlocks != null && descriptionBlocks.isNotEmpty) {
      descriptionInput = {'version': '1.0', 'blocks': descriptionBlocks};
    } else if (description != null && description.isNotEmpty) {
      descriptionInput = {
        'version': '1.0',
        'blocks': [
          {'type': 'text', 'content': description},
        ],
      };
    }

    return apiPut(
      '/api/v1/app/products/$productId',
      body: {
        if (idempotencyKey != null) 'idempotencyKey': idempotencyKey,
        if (name != null) 'name': name,
        if (categoryId != null) 'categoryId': categoryId,
        if (origin != null) 'origin': origin,
        if (descriptionInput != null) 'description': descriptionInput,
        if (images != null) 'images': images,
        if (skus != null) 'skus': skus,
        if (certifications != null) 'certifications': certifications,
        if (status != null) 'status': status,
      },
    );
  }

  Future<Map<String, dynamic>> deleteProduct(dynamic productId) async {
    return apiDelete('/api/v1/app/products/$productId');
  }

  Future<Map<String, dynamic>> setProductStatus(
    dynamic productId, {
    required String status,
  }) async {
    return apiPut(
      '/api/v1/app/products/$productId/status',
      body: {'status': status},
    );
  }

  // ── 商品詳情 ─────────────────────────────────────────

  Future<Map<String, dynamic>> getProductSkus(dynamic productId) async {
    return apiGet('/api/v1/app/products/$productId/skus');
  }

  Future<bool> verifyCertification({
    required String certNo,
    required String type,
  }) async {
    final result = await apiGet(
      '/api/v1/app/products/certifications/verify',
      queryParams: {'cert_no': certNo, 'type': type},
    );
    final data = result['data'];
    if (data is Map<String, dynamic>) {
      final exists = data['exists'];
      if (exists is bool) return exists;
      final normalized = exists?.toString().toLowerCase().trim() ?? '';
      return normalized == 'true' || normalized == '1';
    }
    return false;
  }

  // ── 評論 ─────────────────────────────────────────────

  /// `GET /public/products/{id}/reviews`
  ///
  /// Page-trap envelope（backend#2216 C8）：`records` / `stats` / `total` /
  /// `page` / `pageSize` 全部收在 `data` 之下，三個數值欄一併字串化。這裡只取
  /// `records`；日後若要讀數值欄，一律 `int.tryParse`，不可 `as int`。
  Future<List<Map<String, dynamic>>> getSellerProductReviewsPublic(
    String sellerId,
    dynamic skuId,
  ) async {
    final result = await apiGet('/api/v1/public/products/$skuId/reviews');
    final data = result['data'];
    if (data is Map) {
      final records = data['records'];
      if (records is List) {
        return records.whereType<Map<String, dynamic>>().toList();
      }
    }
    return [];
  }

  // ── 配送計算 ──────────────────────────────────────────

  /// 取得賣家可用的配送方式（含 onboarding 狀態）
  /// GET /app/shipping_methods
  /// 每筆含 shippingMethod / goodsType / available / enabled / onboardingStatus
  /// 上架/編輯商品畫面用來過濾「賣家尚未開通」的配送方式
  Future<List<Map<String, dynamic>>> getSellerShippingMethods() async {
    final resp = await apiGet('/api/v1/app/shipping_methods');
    if (resp['code'] == 0) {
      final data = resp['data'];
      if (data is List) {
        return data.whereType<Map<String, dynamic>>().toList();
      }
    }
    return const [];
  }

  /// 查詢配送報價
  /// POST /app/shipping_quotes
  /// 根據包裝尺寸與重量回傳可用配送方式、運費、溫層
  Future<Map<String, dynamic>> getShippingQuotes({
    required String length,
    required String width,
    required String height,
    required String weight,
    required String weightUnit,
  }) {
    return apiPost(
      '/api/v1/app/shipping_quotes',
      body: {
        'length': length,
        'width': width,
        'height': height,
        'weight': weight,
        'weightUnit': weightUnit,
      },
    );
  }

  // ── 商品詳情 ─────────────────────────────────────────

  /// 取得超商門市地圖參數（PayUni Form POST）
  /// POST /app/store_map_sessions
  Future<Map<String, dynamic>?> getStoreMapSession() async {
    final resp = await apiPost(
      '/api/v1/app/store_map_sessions',
      body: {'appType': 'seller'},
    );
    if (resp['code'] == 0 && resp['data'] is Map<String, dynamic>) {
      return resp['data'] as Map<String, dynamic>;
    }
    return null;
  }

  Future<Map<String, dynamic>> getProductDetail(dynamic productId) {
    return apiGet('/api/v1/app/products/$productId');
  }

  // ── 分類 ─────────────────────────────────────────────

  Future<List<Map<String, dynamic>>> fetchCategories({
    bool forceRefresh = false,
  }) async {
    final result = await apiGet('/api/v1/app/categories/tree');
    final data = result['data'];
    if (data is List) {
      return data.cast<Map<String, dynamic>>();
    }
    return [];
  }

  Future<List<Map<String, dynamic>>> fetchSellerCategoryTree() async {
    final result = await apiGet('/api/v1/app/categories/tree');
    final data = result['data'];
    if (data is List) {
      return data.cast<Map<String, dynamic>>();
    }
    return [];
  }

  Future<Map<String, dynamic>> getCategoryPath(int categoryId) {
    return apiGet('/api/v1/public/categories/$categoryId/path');
  }

  Future<Map<String, dynamic>> getCategoryPathByCatalogId(int catalogId) {
    return apiGet('/api/v1/public/categories/$catalogId/path');
  }

  Future<Map<String, dynamic>> getCategoryPathByCandidateId(int candidateId) {
    return apiGet('/api/v1/public/categories/$candidateId/path');
  }

  Future<Map<String, dynamic>> submitCategoryCandidate({
    required String name,
    required int parentId,
    String? icon,
  }) {
    return apiPost(
      '/api/v1/app/categories',
      body: {
        'name': name,
        'parentId': parentId.toString(),
        if (icon != null) 'icon_url': icon,
      },
    );
  }

  // ── 評分 & 評論 ──────────────────────────────────────

  Future<Map<String, dynamic>> getPublicProductRatingSummary(
    dynamic productId,
  ) {
    return apiGet('/api/v1/public/products/$productId/rating');
  }

  /// 與 [getSellerProductReviewsPublic] 同一支端點、同一份解析，
  /// 只是給商品預覽頁的另一個入口。**直接委派**，避免兩份 envelope 解析各自
  /// 漂移（C8 之前它們就是複製貼上的兩份，端點改形狀時得記得改兩處）。
  Future<List<Map<String, dynamic>>> listPublicProductReviews(
    String sellerId,
    dynamic skuId,
  ) {
    return getSellerProductReviewsPublic(sellerId, skuId);
  }

  Future<List<Map<String, dynamic>>> getSellerProductGrades(
    String sellerId,
    dynamic skuId,
  ) async {
    final result = await apiGet('/api/v1/app/products/$skuId/skus');
    final data = result['data'];
    if (data is List) {
      return data.cast<Map<String, dynamic>>();
    }
    if (data is Map<String, dynamic>) {
      final skus = data['skus'];
      if (skus is List) {
        return skus.cast<Map<String, dynamic>>();
      }
      final records = data['records'];
      if (records is List) {
        return records.cast<Map<String, dynamic>>();
      }
    }
    return [];
  }

  // ── 商家商品列表 ──────────────────────────────────────

  Future<List<Map<String, dynamic>>> getSellerProducts(String sellerId) async {
    final result = await getMyProducts();
    final data = result['data'];
    // data 是 {"records":[...],"total":N,...} 分頁結構
    if (data is Map<String, dynamic>) {
      final records = data['records'];
      if (records is List) {
        return records.cast<Map<String, dynamic>>();
      }
    }
    // 向後相容：如果 data 直接是 List
    if (data is List) {
      return data.cast<Map<String, dynamic>>();
    }
    return [];
  }
}

@riverpod
ProductRepository productRepository(Ref ref) {
  return ProductRepository(ref.watch(dioProvider));
}
