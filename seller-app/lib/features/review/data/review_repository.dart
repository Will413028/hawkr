import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hawkr_api/images.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'review_repository.g.dart';

@riverpod
ReviewRepository reviewRepository(Ref ref) => ReviewRepository(
  ref.watch(reviewPublicClientProvider),
  ref.watch(reviewAppClientProvider),
);

/// 店家對評價的回覆。
class ReviewReplyItem {
  const ReviewReplyItem({required this.content, this.replyTime});

  final String content;
  final DateTime? replyTime;
}

/// 一則評價（`Review`）。
class ReviewItem {
  const ReviewItem({
    required this.id,
    required this.productId,
    required this.rating,
    required this.content,
    required this.imageUrls,
    required this.authorName,
    required this.authorAvatarUrl,
    this.createTime,
    this.orderItemId,
    this.reply,
  });

  factory ReviewItem.fromProto(pb.Review r) => ReviewItem(
    id: r.id,
    productId: r.productId,
    rating: r.rating,
    content: r.content,
    imageUrls: [for (final i in r.images) i.uri(ImagePreset.detail)],
    authorName: r.author.hasName() ? r.author.name : '',
    authorAvatarUrl:
        r.hasAuthor() &&
            r.author.hasAvatar() &&
            r.author.avatar.uriTemplate.isNotEmpty
        ? r.author.avatar.uri(ImagePreset.avatar)
        : '',
    createTime: r.hasCreateTime() ? r.createTime.toDateTime() : null,
    orderItemId: r.hasOrderItemId() ? r.orderItemId : null,
    reply: r.hasReply()
        ? ReviewReplyItem(
            content: r.reply.content,
            replyTime: r.reply.hasReplyTime()
                ? r.reply.replyTime.toDateTime()
                : null,
          )
        : null,
  );

  final String id;
  final String productId;
  final int rating;
  final String content;
  final List<String> imageUrls;
  final String authorName;
  final String authorAvatarUrl;
  final DateTime? createTime;
  final String? orderItemId;
  final ReviewReplyItem? reply;
}

/// 評價統計（`GetProductReviewStats`）：數字只來自伺服器，client 不加總評價列。
class ReviewStats {
  const ReviewStats({
    required this.count,
    required this.average,
    required this.distribution,
  });

  factory ReviewStats.fromProto(
    int count,
    double average,
    Iterable<pb.RatingCount> distribution,
  ) => ReviewStats(
    count: count,
    average: average,
    distribution: {
      5: 0,
      4: 0,
      3: 0,
      2: 0,
      1: 0,
      for (final d in distribution) d.rating: d.count,
    },
  );

  final int count;
  final double average;

  /// 星等（1 到 5）到評價數。
  final Map<int, int> distribution;
}

class ReviewPage {
  const ReviewPage({required this.reviews, required this.nextPageToken});

  final List<ReviewItem> reviews;
  final String nextPageToken;
}

/// 評價（`ReviewPublicService`、`ReviewAppService`）。讀取失敗一律丟出例外，呼叫端分開處理「讀取失敗」與「沒有評價」。
class ReviewRepository {
  ReviewRepository(this._public, this._app);

  static const pageSize = 50;

  final ReviewPublicServiceClient _public;
  final ReviewAppServiceClient _app;

  Future<ReviewPage> listProductReviews(
    String productId, {
    String pageToken = '',
    int? rating,
    bool withImages = false,
  }) async {
    final res = await _public.listProductReviews(
      pb.ListProductReviewsRequest(
        productId: productId,
        pageSize: pageSize,
        pageToken: pageToken,
        rating: rating,
        withImages: withImages,
      ),
    );
    return ReviewPage(
      reviews: res.reviews.map(ReviewItem.fromProto).toList(),
      nextPageToken: res.nextPageToken,
    );
  }

  Future<ReviewStats> productStats(String productId) async {
    final res = await _public.getProductReviewStats(
      pb.GetProductReviewStatsRequest(productId: productId),
    );
    return ReviewStats.fromProto(res.count, res.average, res.distribution);
  }

  Future<ReviewPage> listOwnStoreReviews({String pageToken = ''}) async {
    final res = await _app.listOwnStoreReviews(
      pb.ListOwnStoreReviewsRequest(pageSize: pageSize, pageToken: pageToken),
    );
    return ReviewPage(
      reviews: res.reviews.map(ReviewItem.fromProto).toList(),
      nextPageToken: res.nextPageToken,
    );
  }

  /// 在自己店的評價裡找屬於 [orderItemIds] 的第一則（最多掃 [maxPages] 頁）；沒有找到回 null。
  /// 失敗丟出例外，與「買家沒有評價」分開。
  Future<ReviewItem?> findOwnStoreReviewByOrderItems(
    Set<String> orderItemIds, {
    int maxPages = 5,
  }) async {
    if (orderItemIds.isEmpty) return null;
    var token = '';
    for (var i = 0; i < maxPages; i++) {
      final page = await listOwnStoreReviews(pageToken: token);
      for (final r in page.reviews) {
        final id = r.orderItemId;
        if (id != null && orderItemIds.contains(id)) return r;
      }
      if (page.nextPageToken.isEmpty) return null;
      token = page.nextPageToken;
    }
    return null;
  }

  /// 回覆評價（已有回覆時覆蓋）。失敗的文字由呼叫端用 `errorText` 顯示伺服器的句子。
  Future<ReviewItem> replyReview(String reviewId, String content) async {
    final res = await _app.replyReview(
      pb.ReplyReviewRequest(reviewId: reviewId, content: content),
    );
    return ReviewItem.fromProto(res.review);
  }
}
