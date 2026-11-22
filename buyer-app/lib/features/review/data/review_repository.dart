import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'review_repository.g.dart';

@riverpod
ReviewRepository reviewRepository(Ref ref) => ReviewRepository(
  ref.watch(reviewPublicClientProvider),
  ref.watch(reviewAppClientProvider),
);

/// 商品評價的統計，由 `GetProductReviewStats` 回傳，client 不分頁加總。[distribution] 是 5 到 1 星各自的筆數。
typedef ProductReviewStats = ({
  int count,
  double average,
  Map<int, int> distribution,
});

/// 評價（`ReviewPublicService`、`ReviewAppService`）。讀取失敗一律丟出，不回空清單：呼叫端要把
/// 「讀取失敗」和「沒有評價」分開呈現。
class ReviewRepository {
  ReviewRepository(this._public, this._app);

  /// 每頁筆數（API 預設 20，上限 100）。
  static const pageSize = 100;

  /// 讀一個商品的評價最多讀幾頁；超過時只回前面的，避免單一商品拖住畫面。
  static const maxPages = 10;

  final ReviewPublicServiceClient _public;
  final ReviewAppServiceClient _app;

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

  /// 商品的公開評價，只供清單顯示與篩選，數量與平均分不由它算。依序讀到沒有下一頁或 [maxPages] 頁。有觀看者封鎖過濾時一頁可以少於 [pageSize]，
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

  /// 商品評價的數量、平均分與各星等筆數，來自伺服器的 `GetProductReviewStats`。失敗丟出，
  /// 呼叫端不可把失敗當成 0 則評價。
  Future<ProductReviewStats> getProductReviewStats(String productId) async {
    final response = await _public.getProductReviewStats(
      pb.GetProductReviewStatsRequest(productId: productId),
    );
    final distribution = {5: 0, 4: 0, 3: 0, 2: 0, 1: 0};
    for (final bucket in response.distribution) {
      if (distribution.containsKey(bucket.rating)) {
        distribution[bucket.rating] = bucket.count;
      }
    }
    return (
      count: response.count,
      average: response.average,
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

  /// 這些訂單項目裡依傳入順序第一個有我可見評價的那則；都沒有回 null。各項目並行查詢，
  /// 有項目失敗但其他項目查到評價時回查到的那則，沒查到任何評價且有失敗時丟出第一個失敗。
  Future<pb.Review?> findMyReviewForOrderItems(
    Iterable<String> orderItemIds,
  ) async {
    final results = await Future.wait([
      for (final id in orderItemIds) _findOrError(id),
    ]);
    Object? firstError;
    StackTrace? firstStack;
    for (final result in results) {
      if (result.review != null) return result.review;
      if (result.error != null && firstError == null) {
        firstError = result.error;
        firstStack = result.stack;
      }
    }
    if (firstError != null) Error.throwWithStackTrace(firstError, firstStack!);
    return null;
  }

  Future<({pb.Review? review, Object? error, StackTrace? stack})> _findOrError(
    String orderItemId,
  ) async {
    try {
      return (
        review: await findMyReviewForOrderItem(orderItemId),
        error: null,
        stack: null,
      );
    } catch (e, s) {
      return (review: null, error: e, stack: s);
    }
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
