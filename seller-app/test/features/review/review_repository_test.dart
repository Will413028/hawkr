import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hawkr_seller/features/review/data/review_repository.dart';

import '../../support/connect_errors.dart';
import '../../support/default_error_text.dart';

ReviewRepository _repo(FakeTransportBuilder t) {
  final transport = t.build();
  return ReviewRepository(
    ReviewPublicServiceClient(transport),
    ReviewAppServiceClient(transport),
  );
}

pb.Review _review(String id, {String? orderItemId, int rating = 5}) =>
    pb.Review(
      id: id,
      productId: 'p1',
      rating: rating,
      content: 'c$id',
      orderItemId: orderItemId,
    );

void main() {
  test(
    'product stats come from GetProductReviewStats, not from the page',
    () async {
      final listRequests = <pb.ListProductReviewsRequest>[];
      final repo = _repo(
        FakeTransportBuilder()
            .unary(
              ReviewPublicService.getProductReviewStats,
              (r, _) => pb.GetProductReviewStatsResponse(
                count: 120,
                average: 4.25,
                distribution: [
                  pb.RatingCount(rating: 5, count: 80),
                  pb.RatingCount(rating: 1, count: 40),
                ],
              ),
            )
            .unary(ReviewPublicService.listProductReviews, (r, _) {
              listRequests.add(r);
              return pb.ListProductReviewsResponse(
                reviews: [_review('r1', rating: 1)],
                nextPageToken: 'n',
              );
            }),
      );
      final stats = await repo.productStats('p1');
      final page = await repo.listProductReviews(
        'p1',
        rating: 1,
        withImages: true,
      );
      expect(stats.count, 120);
      expect(stats.average, 4.25);
      expect(stats.distribution, {5: 80, 4: 0, 3: 0, 2: 0, 1: 40});
      expect(page.reviews, hasLength(1));
      expect(page.nextPageToken, 'n');
      expect(listRequests.single.rating, 1);
      expect(listRequests.single.withImages, isTrue);
    },
  );

  test('read failure throws instead of returning an empty page', () async {
    final repo = _repo(
      FakeTransportBuilder().unary(
        ReviewPublicService.listProductReviews,
        (r, _) => throw reasonError(Code.unavailable, 'X', message: '稍後再試'),
      ),
    );
    await expectLater(repo.listProductReviews('p1'), throwsA(anything));
  });

  test('empty page is a successful empty result', () async {
    final repo = _repo(
      FakeTransportBuilder().unary(
        ReviewPublicService.listProductReviews,
        (r, _) => pb.ListProductReviewsResponse(),
      ),
    );
    final page = await repo.listProductReviews('p1');
    expect(page.reviews, isEmpty);
    expect(page.nextPageToken, isEmpty);
  });

  test('own store review lookup pages until the order item matches', () async {
    final tokens = <String>[];
    final repo = _repo(
      FakeTransportBuilder().unary(ReviewAppService.listOwnStoreReviews, (
        r,
        _,
      ) {
        tokens.add(r.pageToken);
        return r.pageToken.isEmpty
            ? pb.ListOwnStoreReviewsResponse(
                reviews: [_review('a', orderItemId: 'x')],
                nextPageToken: 't2',
              )
            : pb.ListOwnStoreReviewsResponse(
                reviews: [_review('b', orderItemId: 'i2')],
              );
      }),
    );
    final found = await repo.findOwnStoreReviewByOrderItems({'i2'});
    expect(found?.id, 'b');
    expect(tokens, ['', 't2']);
    expect(await repo.findOwnStoreReviewByOrderItems({'none'}), isNull);
  });

  test(
    'ReplyReview sends review_id and content and returns the reply',
    () async {
      final requests = <pb.ReplyReviewRequest>[];
      final repo = _repo(
        FakeTransportBuilder().unary(ReviewAppService.replyReview, (r, _) {
          requests.add(r);
          return pb.ReplyReviewResponse(
            review: pb.Review(
              id: r.reviewId,
              reply: pb.ReviewReply(content: r.content),
            ),
          );
        }),
      );
      final out = await repo.replyReview('r9', '謝謝');
      expect(requests.single.reviewId, 'r9');
      expect(requests.single.content, '謝謝');
      expect(out.reply?.content, '謝謝');
    },
  );

  test('ReplyReview errors surface the server sentence', () async {
    final repo = _repo(
      FakeTransportBuilder().unary(
        ReviewAppService.replyReview,
        (r, _) => throw reasonError(
          Code.failedPrecondition,
          'REVIEW_NOT_REPLYABLE',
          message: '這則評價目前無法回覆',
        ),
      ),
    );
    try {
      await repo.replyReview('r', 'x');
      fail('should throw');
    } catch (e) {
      expect(errorText(e, defaults: zhDefaultErrorText), '這則評價目前無法回覆');
    }
  });
}
