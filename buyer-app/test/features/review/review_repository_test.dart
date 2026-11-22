import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/networking/error_reason.dart';
import 'package:hawkr_buyer/features/review/data/review_repository.dart';

import 'review_fakes.dart';

void main() {
  group('listAllProductReviews', () {
    test('讀到沒有下一頁為止，依序合併', () async {
      final fake = FakeReviewPublic(
        pages: [
          [reviewRecord('a'), reviewRecord('b')],
          [reviewRecord('c')],
        ],
      );
      final repo = ReviewRepository(fake.client(), FakeReviewApp().client());

      final reviews = await repo.listAllProductReviews('55');

      expect([for (final r in reviews) r.id], ['a', 'b', 'c']);
      expect(fake.requests.map((r) => r.productId), everyElement('55'));
      expect(fake.requests.map((r) => r.pageToken), ['', 'page-1']);
    });

    test('一頁可以是空的但還有下一頁，結束只看 next_page_token', () async {
      final fake = FakeReviewPublic(
        pages: [
          [],
          [reviewRecord('a')],
        ],
      );
      final repo = ReviewRepository(fake.client(), FakeReviewApp().client());

      final reviews = await repo.listAllProductReviews('55');

      expect(reviews.single.id, 'a');
    });

    test('讀取失敗丟出錯誤，不回空清單', () async {
      final fake = FakeReviewPublic(
        error: ConnectException(Code.unavailable, 'down'),
      );
      final repo = ReviewRepository(fake.client(), FakeReviewApp().client());

      await expectLater(
        repo.listAllProductReviews('55'),
        throwsA(isA<ConnectException>()),
      );
    });

    test('沒有評價是空清單，不是錯誤', () async {
      final repo = ReviewRepository(
        FakeReviewPublic().client(),
        FakeReviewApp().client(),
      );

      expect(await repo.listAllProductReviews('55'), isEmpty);
    });
  });

  group('getProductReviewStats', () {
    test('取伺服器的數量、平均與各星等，缺的星等補 0', () async {
      final fake = FakeReviewPublic(
        stats: pb.GetProductReviewStatsResponse(
          count: 230,
          average: 4.5,
          distribution: [
            pb.RatingCount(rating: 5, count: 200),
            pb.RatingCount(rating: 1, count: 30),
          ],
        ),
      );
      final repo = ReviewRepository(fake.client(), FakeReviewApp().client());

      final stats = await repo.getProductReviewStats('55');

      expect(stats.count, 230);
      expect(stats.average, 4.5);
      expect(stats.distribution, {5: 200, 4: 0, 3: 0, 2: 0, 1: 30});
      expect(fake.statsRequests.single.productId, '55');
    });

    test('讀取失敗丟出錯誤，不回 0', () async {
      final fake = FakeReviewPublic(
        statsError: ConnectException(Code.unavailable, 'down'),
      );
      final repo = ReviewRepository(fake.client(), FakeReviewApp().client());

      await expectLater(
        repo.getProductReviewStats('55'),
        throwsA(isA<ConnectException>()),
      );
    });
  });

  group('我的評價', () {
    test('依訂單項目查詢，送 order_item_id', () async {
      final app = FakeReviewApp(
        mine: [
          reviewRecord('r1', orderItemId: '10'),
          reviewRecord('r2', orderItemId: '11'),
        ],
      );
      final repo = ReviewRepository(FakeReviewPublic().client(), app.client());

      final found = await repo.findMyReviewForOrderItems(['9', '11']);

      expect(found?.id, 'r2');
      expect(app.myRequests.map((r) => r.orderItemId), ['9', '11']);
    });

    test('有項目失敗但其他項目查到評價時回查到的那則', () async {
      final app = FakeReviewApp(
        mine: [reviewRecord('r2', orderItemId: '11')],
        failOrderItemIds: {'9'},
      );
      final repo = ReviewRepository(FakeReviewPublic().client(), app.client());

      final found = await repo.findMyReviewForOrderItems(['9', '11']);

      expect(found?.id, 'r2');
    });

    test('沒查到任何評價且有項目失敗時丟出失敗', () async {
      final app = FakeReviewApp(failOrderItemIds: {'9'});
      final repo = ReviewRepository(FakeReviewPublic().client(), app.client());

      await expectLater(
        repo.findMyReviewForOrderItems(['9', '11']),
        throwsA(isA<ConnectException>()),
      );
    });

    test('都沒有回 null', () async {
      final repo = ReviewRepository(
        FakeReviewPublic().client(),
        FakeReviewApp().client(),
      );

      expect(await repo.findMyReviewForOrderItems(['9']), isNull);
    });
  });

  group('createReview', () {
    test('送訂單編號、訂單項目 ID（字串）、星等、內文與圖片 token', () async {
      final app = FakeReviewApp();
      final repo = ReviewRepository(FakeReviewPublic().client(), app.client());

      final review = await repo.createReview(
        orderNo: 'SO-1',
        orderItemId: '77',
        rating: 4,
        content: '好吃',
        stagedTokens: ['t1', 't2'],
      );

      final sent = app.creates.single;
      expect(sent.orderNo, 'SO-1');
      expect(sent.orderItemId, '77');
      expect(sent.rating, 4);
      expect(sent.content, '好吃');
      expect(sent.imageStagedTokens, ['t1', 't2']);
      expect(review.id, 'new-review');
    });

    test('失敗原樣丟出，REVIEW_ALREADY_EXISTS 可由 errorReasonOf 辨認', () async {
      final app = FakeReviewApp(
        createError: connectErrorWithReason(
          Code.alreadyExists,
          ErrorReason.REVIEW_ALREADY_EXISTS,
        ),
      );
      final repo = ReviewRepository(FakeReviewPublic().client(), app.client());

      Object? caught;
      try {
        await repo.createReview(
          orderNo: 'SO-1',
          orderItemId: '77',
          rating: 5,
          content: 'x',
        );
      } catch (e) {
        caught = e;
      }

      expect(caught, isNotNull);
      expect(errorReasonOf(caught!), ErrorReason.REVIEW_ALREADY_EXISTS);
    });
  });
}
