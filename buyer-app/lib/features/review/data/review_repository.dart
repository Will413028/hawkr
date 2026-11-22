import 'package:dio/dio.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../core/networking/dio_provider.dart';

part 'review_repository.g.dart';

@riverpod
ReviewRepository reviewRepository(Ref ref) => ReviewRepository(
  ref.watch(reviewPublicClientProvider),
  ref.watch(reviewAppClientProvider),
  ref.watch(dioProvider),
);

/// 商品評價的數量與平均分（商品卡片用）。
typedef ProductRatingStats = ({int count, double average});

/// 商品評價的統計：由畫面讀到的評價算出。
typedef ProductReviewSummary = ({
  int count,
  double average,
  Map<int, int> distribution,
});

/// 評價（`ReviewPublicService`、`ReviewAppService`）。讀取失敗一律丟出，不回空清單：呼叫端要把
/// 「讀取失敗」和「沒有評價」分開呈現。
class ReviewRepository {
  ReviewRepository(this._public, this._app, this._dio);

  /// 每頁筆數（API 預設 20，上限 100）。
  static const pageSize = 100;

  /// 讀一個商品的評價最多讀幾頁；超過時只回前面的，避免單一商品拖住畫面。
  static const maxPages = 10;

  final ReviewPublicServiceClient _public;
  final ReviewAppServiceClient _app;
  final Dio _dio;

  /// 商品的公開評價一頁；[pageToken] 空字串是第一頁，回應的 `nextPageToken` 空字串表示沒有下一頁。
  Future<pb.ListProductReviewsResponse> listProductReviews(
    String productId, {
    String pageToken = '',
    int? pageSize,
  }) => _public.listProductReviews(
    pb.ListProductReviewsRequest(
      productId: productId,
      pageSize: pageSize ?? ReviewRepository.pageSize,
      pageToken: pageToken,
    ),
  );

  /// 商品的公開評價，依序讀到沒有下一頁或 [maxPages] 頁。有觀看者封鎖過濾時一頁可以少於 [pageSize]，
  /// 所以結束只看 `nextPageToken`。
  Future<List<pb.Review>> listAllProductReviews(String productId) async {
    final reviews = <pb.Review>[];
    var token = '';
    for (var page = 0; page < maxPages; page++) {
      final response = await listProductReviews(productId, pageToken: token);
      reviews.addAll(response.reviews);
      token = response.nextPageToken;
      if (token.isEmpty) break;
    }
    return reviews;
  }

  /// 商品評價的數量與平均分，商品卡片缺這兩個值時補用。這是唯一的接縫：契約還沒有商品層級的統計，
  /// 目前沿用舊 gateway 的 `GET /public/products/{id}/reviews` 的 `stats` 區塊（A6-5e 的伺服器方法完成後，
  /// A6-6c2 改讀新方法並刪除這裡的舊呼叫；client 不分頁加總）。失敗丟出。
  Future<ProductRatingStats> productRatingStats(String productId) async {
    final resp = await _dio.get<Map<String, dynamic>>(
      '/public/products/$productId/reviews',
      queryParameters: const {'page': 1, 'size': 1},
    );
    final data = resp.data?['data'];
    final stats = data is Map ? data['stats'] : resp.data?['stats'];
    if (stats is! Map) return (count: 0, average: 0.0);
    final count = (stats['totalCount'] as num?)?.toInt() ?? 0;
    final raw = (stats['averageRating'] as num?)?.toDouble() ?? 0.0;
    final average = raw > 5 && raw <= 50 ? raw / 10 : raw;
    return (count: count, average: average.clamp(0.0, 5.0));
  }

  static ProductReviewSummary summarize(List<pb.Review> reviews) {
    final distribution = {5: 0, 4: 0, 3: 0, 2: 0, 1: 0};
    var sum = 0;
    var count = 0;
    for (final review in reviews) {
      final rating = review.rating;
      if (rating < 1 || rating > 5) continue;
      sum += rating;
      count++;
      distribution[rating] = distribution[rating]! + 1;
    }
    return (
      count: count,
      average: count == 0 ? 0.0 : sum / count,
      distribution: distribution,
    );
  }

  /// 這個訂單項目我可見的評價；沒有（沒寫過，或被平台隱藏）回 null。
  Future<pb.Review?> findMyReviewForOrderItem(String orderItemId) async {
    final response = await _app.listMyReviews(
      pb.ListMyReviewsRequest(pageSize: 1, orderItemId: orderItemId),
    );
    return response.reviews.isEmpty ? null : response.reviews.first;
  }

  /// 這些訂單項目裡第一個有我可見評價的那則；都沒有回 null。
  Future<pb.Review?> findMyReviewForOrderItems(
    Iterable<String> orderItemIds,
  ) async {
    for (final id in orderItemIds) {
      final review = await findMyReviewForOrderItem(id);
      if (review != null) return review;
    }
    return null;
  }

  /// 對一個已完成的訂單項目寫評價。[stagedTokens] 是圖片上傳完成後拿到的 token，最多 4 個。
  /// 已評價回 `REVIEW_ALREADY_EXISTS`、超過期限回 `REVIEW_WINDOW_EXPIRED`，由呼叫端依 `errorReasonOf` 處理。
  Future<pb.Review> createReview({
    required String orderNo,
    required String orderItemId,
    required int rating,
    required String content,
    List<String> stagedTokens = const [],
  }) async {
    final response = await _app.createReview(
      pb.CreateReviewRequest(
        orderNo: orderNo,
        orderItemId: orderItemId,
        rating: rating,
        content: content,
        imageStagedTokens: stagedTokens,
      ),
    );
    return response.review;
  }
}
