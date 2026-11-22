import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart' show ErrorInfo;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;

pb.Review reviewRecord(
  String id, {
  int rating = 5,
  String content = '',
  String authorName = '',
  int images = 0,
  String? orderItemId,
}) => pb.Review(
  id: id,
  rating: rating,
  content: content,
  author: pb.ReviewAuthor(name: authorName),
  orderItemId: orderItemId,
);

/// `ReviewPublicService` 的 fake。商品評價以頁給：第 n 頁的 token 是 `page-n`，最後一頁沒有 next token。
class FakeReviewPublic {
  FakeReviewPublic({
    this.pages = const [[]],
    this.error,
    this.stats,
    this.statsError,
  });

  /// `GetProductReviewStats` 的回應；沒給時依 [pages] 的評價算出，只供沒有關心統計的測試。
  pb.GetProductReviewStatsResponse? stats;

  /// 設了之後 `GetProductReviewStats` 以它失敗。
  ConnectException? statsError;

  final List<pb.GetProductReviewStatsRequest> statsRequests = [];

  List<List<pb.Review>> pages;

  /// 設了之後 `ListProductReviews` 一律以它失敗。
  ConnectException? error;

  final List<pb.ListProductReviewsRequest> requests = [];

  ReviewPublicServiceClient client() => ReviewPublicServiceClient(
    FakeTransportBuilder()
        .unary(ReviewPublicService.getProductReviewStats, (req, _) async {
          statsRequests.add(req);
          if (statsError != null) throw statsError!;
          if (stats != null) return stats!;
          final all = pages.expand((p) => p).toList();
          return pb.GetProductReviewStatsResponse(
            count: all.length,
            average: all.isEmpty
                ? 0
                : all.fold<int>(0, (a, r) => a + r.rating) / all.length,
            distribution: [
              for (var star = 5; star >= 1; star--)
                pb.RatingCount(
                  rating: star,
                  count: all.where((r) => r.rating == star).length,
                ),
            ],
          );
        })
        .unary(ReviewPublicService.listProductReviews, (req, _) async {
          requests.add(req);
          if (error != null) throw error!;
          final i = req.pageToken.isEmpty
              ? 0
              : int.parse(req.pageToken.substring('page-'.length));
          return pb.ListProductReviewsResponse(
            reviews: pages[i],
            nextPageToken: i + 1 < pages.length ? 'page-${i + 1}' : '',
          );
        })
        .build(),
  );
}

/// `ReviewAppService` 的 fake：我的評價依 `order_item_id` 篩選，寫評價記下請求。
class FakeReviewApp {
  FakeReviewApp({
    this.mine = const [],
    this.createError,
    this.failOrderItemIds = const {},
  });

  /// 這些訂單項目的 `ListMyReviews` 以 Unavailable 失敗。
  Set<String> failOrderItemIds;

  List<pb.Review> mine;
  ConnectException? createError;

  final List<pb.CreateReviewRequest> creates = [];
  final List<pb.ListMyReviewsRequest> myRequests = [];

  ReviewAppServiceClient client() => ReviewAppServiceClient(
    FakeTransportBuilder()
        .unary(ReviewAppService.createReview, (req, _) async {
          creates.add(req);
          if (createError != null) throw createError!;
          return pb.CreateReviewResponse(
            review: pb.Review(
              id: 'new-review',
              rating: req.rating,
              content: req.content,
              orderItemId: req.orderItemId,
            ),
          );
        })
        .unary(ReviewAppService.listMyReviews, (req, _) async {
          myRequests.add(req);
          if (failOrderItemIds.contains(req.orderItemId)) {
            throw ConnectException(Code.unavailable, 'down');
          }
          return pb.ListMyReviewsResponse(
            reviews: [
              for (final r in mine)
                if (!req.hasOrderItemId() || r.orderItemId == req.orderItemId)
                  r,
            ],
          );
        })
        .build(),
  );
}

/// 帶 hawkr `ErrorReason` 的 Connect 錯誤（同伺服器的 `ErrorInfo`）。
ConnectException connectErrorWithReason(Code code, ErrorReason reason) =>
    ConnectException(
      code,
      reason.name,
      details: [
        ErrorDetail(
          'google.rpc.ErrorInfo',
          (ErrorInfo()
                ..reason = reason.name
                ..domain = 'hawkr')
              .writeToBuffer(),
        ),
      ],
    );
