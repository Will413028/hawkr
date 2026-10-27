import 'package:dio/dio.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:hawkr_buyer/shared/utils/hk_log.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import 'product_model.dart';

part 'product_repository.g.dart';

@riverpod
ProductRepository productRepository(Ref ref) => ProductRepository(
  ref.watch(dioProvider),
  BuyerAppServiceClient(ref.watch(connectAuthTransportProvider)),
);

class ProductRepository {
  final Dio _dio;
  final BuyerAppServiceClient _buyer;
  ProductRepository(this._dio, this._buyer);

  Future<List<Product>> fetchProducts({
    int page = 1,
    int size = 50,
    int? categoryId,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/products',
        queryParameters: {
          'page': page,
          'page_size': size,
          if (categoryId != null) 'category_id': categoryId,
        },
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'];
      final list = (data is Map ? data['records'] : data) as List? ?? [];
      return list
          .map((e) => Product.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  Future<Product> getSkuDetail(int skuId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/products/$skuId',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      return Product.fromJson(body['data'] as Map<String, dynamic>);
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// Get all products for a merchant.
  Future<List<dynamic>> getMerchantProducts(String merchantId) async {
    try {
      final resp = await _dio.get<dynamic>(
        '/public/sellers/$merchantId/products',
      );
      final data = resp.data;
      if (data is List) return data;
      if (data is Map) {
        // 支援兩種格式：
        // 1. {data: [...]}
        // 2. {data: {records: [...], total: N}}
        final inner = data['data'];
        if (inner is List) return inner;
        if (inner is Map) {
          final records = inner['records'] ?? inner['items'] ?? inner['list'];
          if (records is List) return records;
        }
        for (final k in ['rows', 'products']) {
          if (data[k] is List) return data[k] as List;
        }
      }
    } on DioException {
      // fall through
    }
    return [];
  }

  /// 取單一商品的圖片清單。
  ///
  /// 資料來源＝商品詳情 `GET /public/products/{id}`（49 號成對契約：優先 objectKey
  /// 陣列 `imageKeys`，其次 `imageUrls`，最後退回封面 `imageKey`/`imageUrl`）。
  /// Gateway 沒有 `/public/sellers/{id}/products/{sku}/images` 這條路由。
  Future<List<String>> getProductImages(String productId) async {
    if (productId.isEmpty) return [];
    final info = await getProductFullInfo(productId);
    if (info.isEmpty) return [];
    return _extractImageList(info);
  }

  /// 從商品 payload 萃取圖片清單（49 號成對契約，key 優先於 url）。
  static List<String> _extractImageList(Map<String, dynamic> info) {
    final urls = <String>[];
    final list = info['imageKeys'] ?? info['imageUrls'];
    if (list is List) {
      for (final item in list) {
        final v = item.toString().trim();
        if (v.isNotEmpty && v != 'null') urls.add(v);
      }
    }
    if (urls.isEmpty) {
      final cover = (info['imageKey'] ?? info['imageUrl'] ?? '')
          .toString()
          .trim();
      if (cover.isNotEmpty && cover != 'null') urls.add(cover);
    }
    return urls;
  }

  /// 取某賣家第一項商品的封面圖（聊天室「推薦商品」用）。
  /// 資料來源＝ `GET /public/sellers/{id}/products` 列表既有的圖片欄位。
  Future<String> getMerchantFirstProductImage(String merchantId) async {
    if (merchantId.isEmpty) return '';
    final products = await getMerchantProducts(merchantId);
    for (final p in products) {
      if (p is! Map) continue;
      final imgs = _extractImageList(Map<String, dynamic>.from(p));
      if (imgs.isNotEmpty) return imgs.first;
    }
    return '';
  }

  /// Get SKU image URL.
  Future<String?> getSkuImageUrl(String skuId) async {
    try {
      final resp = await _dio.get<dynamic>('/public/products/$skuId/brief');
      final data = resp.data;
      if (data is String) return data;
      if (data is Map) {
        return (data['url'] ?? data['imageUrl'] ?? data['data'] ?? '')
            .toString();
      }
    } on DioException {
      // fall through
    }
    return null;
  }

  /// Get full product info.
  Future<Map<String, dynamic>> getProductFullInfo(String skuId) async {
    try {
      final resp = await _dio.get<dynamic>('/public/products/$skuId');
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        return data['data'] as Map<String, dynamic>? ?? data;
      }
    } on DioException {
      // fall through
    }
    return {};
  }

  Future<Map<String, dynamic>> getCategoryPath(int categoryId) async {
    try {
      final resp = await _dio.get<dynamic>(
        '/public/categories/$categoryId/path',
      );
      final data = resp.data;
      if (data is Map<String, dynamic>) {
        return data['data'] as Map<String, dynamic>? ?? data;
      }
    } on DioException {
      // fall through
    }
    return {};
  }

  /// Get merchant product grades.
  Future<List<Map<String, dynamic>>> getMerchantProductGrades(
    String merchantId,
    String skuId,
  ) async {
    if (skuId.isEmpty) return [];
    try {
      final resp = await _dio.get<dynamic>('/public/products/$skuId/rating');
      final data = resp.data;
      Iterable raw = [];
      if (data is List) raw = data;
      if (data is Map) {
        for (final k in ['data', 'rows', 'list', 'grades']) {
          if (data[k] is List) {
            raw = data[k];
            break;
          }
        }
      }
      return raw
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
    } on DioException {
      return [];
    }
  }

  Future<List<String>> fetchSuggestions(String q) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/search/suggest',
        queryParameters: {'q': q},
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'] as Map<String, dynamic>? ?? {};
      final list = data['suggestions'] as List? ?? [];
      return list.map((e) => e.toString()).toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  // ---------------------------------------------------------------------------
  // Visit & Favorites (used by my_subscription_screen, merchant_item_detail)
  // ---------------------------------------------------------------------------

  Future<bool> addFavoriteProduct({required String productId}) async {
    try {
      final pid = int.tryParse(productId);
      if (pid == null || pid <= 0) return false;
      await _buyer.addFavorite(buyer.AddFavoriteRequest(productId: productId));
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<bool> removeFavoriteProduct(String productId) async {
    try {
      await _buyer.removeFavorite(
        buyer.RemoveFavoriteRequest(productId: productId),
      );
      return true;
    } catch (_) {
      return false;
    }
  }

  Future<void> visitProduct({
    required String merchantId,
    required String merchantName,
    required String productId,
    required String productName,
  }) async {
    if (merchantId.isEmpty || productId.isEmpty) return;
    // §4：RecordVisitReq.productId 為 TSID 數字字串，送 number 會 400
    // "cannot unmarshal number into ... of type string"（2026-07-03 實測）
    if (int.tryParse(productId) == null) return;
    try {
      await _buyer.recordVisit(buyer.RecordVisitRequest(productId: productId));
    } catch (_) {
      // best-effort, ignore errors
    }
  }

  // ---------------------------------------------------------------------------
  // Restock Subscriptions (#942)
  // ---------------------------------------------------------------------------

  /// 訂閱補貨通知 — POST /app/products/{productId}/restock_subscriptions
  /// 後端冪等：已訂閱再呼叫直接回 200，不重複建立。
  Future<bool> subscribeRestock(String productId) async {
    final pid = productId.trim();
    if (pid.isEmpty || int.tryParse(pid) == null) return false;
    try {
      await _buyer.subscribeRestock(
        buyer.SubscribeRestockRequest(productId: pid),
      );
      return true;
    } catch (_) {
      return false;
    }
  }

  /// 取消補貨通知訂閱 — DELETE /app/products/{productId}/restock_subscriptions
  /// 後端冪等：未訂閱也回 200。
  Future<bool> unsubscribeRestock(String productId) async {
    final pid = productId.trim();
    if (pid.isEmpty || int.tryParse(pid) == null) return false;
    try {
      await _buyer.unsubscribeRestock(
        buyer.UnsubscribeRestockRequest(productId: pid),
      );
      return true;
    } catch (_) {
      return false;
    }
  }

  /// 取得分享連結 — GET /app/share_url?type={type}&id={id}
  Future<Map<String, dynamic>?> getShareUrl({
    required String type,
    required String id,
  }) async {
    if (id.isEmpty) return null;
    try {
      final params = {'type': type, 'id': id};
      HKLog.logD('[ProductRepository] getShareUrl, params: $params');
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/share_url',
        queryParameters: params,
      );
      final body = resp.data!;
      HKLog.logD('[ProductRepository] getShareUrl, body: $body');
      if (body['code'] == 0 && body['data'] is Map<String, dynamic>) {
        return body['data'] as Map<String, dynamic>;
      }
      return null;
    } on DioException {
      return null;
    }
  }
}
