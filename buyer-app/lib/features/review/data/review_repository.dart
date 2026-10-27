import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:hawkr_buyer/shared/utils/hk_log.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import 'review_model.dart';

part 'review_repository.g.dart';

@riverpod
ReviewRepository reviewRepository(Ref ref) =>
    ReviewRepository(ref.watch(dioProvider));

class ReviewRepository {
  final Dio _dio;
  ReviewRepository(this._dio);

  int _parseIntValue(dynamic value) {
    if (value == null) return 0;
    if (value is int) return value;
    if (value is num) return value.toInt();
    return int.tryParse(value.toString()) ?? 0;
  }

  double _parseRatingValue(dynamic value) {
    if (value == null) return 0;
    final raw = value is num
        ? value.toDouble()
        : double.tryParse(value.toString());
    if (raw == null || raw <= 0) return 0;
    final rating = raw > 5 && raw <= 50 ? raw / 10 : raw;
    return rating.clamp(0, 5).toDouble();
  }

  List<Map<String, dynamic>> _extractReviewRecords(dynamic data) {
    if (data is List) {
      return data
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
    }
    if (data is Map) {
      for (final k in ['data', 'rows', 'list', 'items', 'reviews']) {
        final v = data[k];
        if (v is List) {
          return v
              .whereType<Map>()
              .map((e) => Map<String, dynamic>.from(e))
              .toList();
        }
        if (v is Map) {
          final records = v['records'];
          if (records is List) {
            return records
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList();
          }
        }
      }
    }
    return [];
  }

  Future<List<Review>> getProductReviews(int skuId) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/public/products/$skuId/reviews',
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'];
      final list = (data is Map ? data['records'] : data) as List? ?? [];
      return list
          .map((e) => Review.fromJson(e as Map<String, dynamic>))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// Public merchant product reviews (used by merchant_item_detail_screen and product_reviews_screen).
  Future<List<Map<String, dynamic>>> getMerchantProductReviews(
    String skuId,
  ) async {
    try {
      final resp = await _dio.get<dynamic>('/public/products/$skuId/reviews');
      return _extractReviewRecords(resp.data);
    } on DioException {
      // fall through
    }
    return [];
  }

  Future<Map<String, dynamic>> getMerchantProductReviewSummary(
    String skuId,
  ) async {
    try {
      final resp = await _dio.get<dynamic>(
        '/public/products/$skuId/reviews',
        queryParameters: const {'page': 1, 'size': 1},
      );
      final data = resp.data;
      if (data is! Map) {
        final records = _extractReviewRecords(data);
        return {'hasTotal': false, 'total': records.length, 'rating': 0.0};
      }

      final inner = data['data'];
      final stats = data['stats'] is Map
          ? data['stats'] as Map
          : (inner is Map && inner['stats'] is Map
                ? inner['stats'] as Map
                : null);
      // GET /public/products/{id}/reviews 的 data.stats 是 ReviewStatsBlock：
      // totalCount / averageRating / rating{1..5}Count / withImagesCount。
      // 它已無 omitempty，零評論商品也照樣吐 0（不再是 undefined），所以
      // `stats.totalCount` 存在與否就是「有沒有 stats 區塊」的唯一判準。
      final totalSource =
          data['total'] ??
          (inner is Map ? inner['total'] : null) ??
          stats?['totalCount'];
      final total = _parseIntValue(totalSource);
      final records = _extractReviewRecords(data);

      final rating = stats != null
          ? _parseRatingValue(stats['averageRating'])
          : 0.0;

      return {
        'hasTotal': totalSource != null,
        'total': totalSource != null ? total : records.length,
        'rating': rating,
      };
    } on DioException {
      return {'hasTotal': false, 'total': 0, 'rating': 0.0};
    }
  }

  /// Post a review for a merchant product.
  Future<bool> postMerchantProductReview({
    required String merchantId,
    required String skuId,
    required int rating,
    String? content,
    int? memberId,
  }) async {
    final safeRating = rating.clamp(1, 5);
    final body = <String, dynamic>{'rating': safeRating};
    if (content != null && content.trim().isNotEmpty) {
      body['content'] = content.trim();
    }
    if (memberId != null && memberId > 0) body['memberId'] = memberId;
    try {
      final resp = await _dio.post<dynamic>('/app/reviews', data: body);
      final status = resp.statusCode ?? 0;
      return status >= 200 && status < 300;
    } on DioException {
      return false;
    }
  }

  /// GET /app/me/reviews — 我的評論歷史
  Future<List<Map<String, dynamic>>> getMyReviews({
    int page = 1,
    int size = 20,
  }) async {
    try {
      final resp = await _dio.get<Map<String, dynamic>>(
        '/app/me/reviews',
        queryParameters: {'page': page, 'size': size},
      );
      final body = resp.data!;
      if (body['code'] != 0) {
        throw ServerException((body['message'] ?? '請求失敗').toString());
      }
      final data = body['data'];
      final list =
          (data is Map ? (data['records'] ?? data['list']) : data) as List? ??
          [];
      return list
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      throw const NetworkException();
    }
  }

  /// 反查訂單的買家評論（B-145 完成前的 fallback）。
  ///
  /// /app/me/reviews API 的 review entity 沒有 `orderNo` 欄位，只有
  /// `orderItemId`，因此 caller 必須傳入該訂單所有 OrderItem 的 id 集合，
  /// 此處比對 review.orderItemId 找出對應評論（同一訂單可能有多個 item，
  /// 各自有獨立 review，回傳第一筆命中的）。
  ///
  /// 內部分頁掃 [getMyReviews]（最多 [maxPages] 頁）。
  ///
  /// 後端在 GET /app/orders/{orderNo} 加 review 欄位後（B-145），
  /// 訂單詳情可直接從 Order.reviewRating / reviewContent 取，
  /// 不再需要這個反查 API call。
  Future<Map<String, dynamic>?> findReviewByOrderItemIds(
    Set<String> orderItemIds, {
    int maxPages = 5,
    int pageSize = 50,
  }) async {
    if (orderItemIds.isEmpty) return null;
    for (var page = 1; page <= maxPages; page++) {
      final reviews = await getMyReviews(page: page, size: pageSize);
      for (final review in reviews) {
        final id = (review['orderItemId'] ?? '').toString().trim();
        if (id.isNotEmpty && orderItemIds.contains(id)) {
          return review;
        }
      }
      // 沒滿一頁 = 沒有更多資料，提早停止
      if (reviews.length < pageSize) break;
    }
    return null;
  }

  Future<Review> addReview({
    required String orderNo,
    required int orderItemId,
    required int skuId,
    required int rating,
    required String content,
    List<String>? stagedTokens,
  }) async {
    try {
      // SubmitOrderItemReviewRequest：orderNo / orderItemId / productId /
      // skuName / rating / content / stagedTokens / productImageUrl。
      // 這裡曾多送一個 `sku_id` —— 後端從來沒有這個欄位（舊名是 product_id），
      // Go 的 json binding 直接忽略它，所以它從第一天起就是空轉。已移除。
      final params = {
        'orderNo': orderNo,
        'orderItemId': orderItemId.toString(),
        'rating': rating,
        'content': content,
        if (stagedTokens != null) 'stagedTokens': stagedTokens,
      };
      final resp = await _dio.post<Map<String, dynamic>>(
        '/app/reviews',
        data: params,
      );

      final body = resp.data;
      HKLog.logD(
        '[ReviewRepository.addReview] status=${resp.statusCode}',
        body,
      );

      final code = body?['code'];

      // HTTP 409 或 body code 409 → 已評論過
      if (resp.statusCode == 409 || code == 409) {
        throw const ServerException('此商品已評論過');
      }

      // body code 0 → 成功。
      // ⚠️ 這裡曾經是 `resp.statusCode == 201 || code == 0`：提交評論原本回 HTTP 201，
      // backend#2198 已改回 200（envelope 走 response.OK，業務結果一律看 code 不看
      // HTTP status）。201 那一項自此恆為 false，留著只會讓人以為 HTTP status 也是
      // 成功判準。真正接得住的一直是 `code == 0`。
      if (code == 0) {
        return Review(id: 0, skuId: skuId, rating: rating, content: content);
      }

      throw ServerException((body?['message'] ?? '評論失敗').toString());
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) throw const UnauthorizedException();
      if (e.response?.statusCode == 409) {
        throw const ServerException('此商品已評論過');
      }
      throw const NetworkException();
    }
  }
}
