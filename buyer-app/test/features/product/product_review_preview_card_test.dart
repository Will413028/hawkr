import 'package:hawkr_buyer/l10n/l10n.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/features/product/presentation/merchant_item_detail_screen.dart';

import '../review/review_fakes.dart';

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  Future<void> pump(
    WidgetTester tester,
    ReviewPublicServiceClient client,
    ValueNotifier<String> sku,
  ) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [reviewPublicClientProvider.overrideWithValue(client)],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, __) => MaterialApp(
            locale: const Locale('zh'),
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: Scaffold(
              body: ValueListenableBuilder<String>(
                valueListenable: sku,
                builder: (_, id, __) => ProductReviewPreviewCard(
                  merchantId: '7',
                  skuId: id,
                  productName: '測試商品',
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  testWidgets('讀取失敗時標題不顯示 (0)', (tester) async {
    final client = FakeReviewPublic(
      error: ConnectException(Code.unavailable, 'down'),
    ).client();
    await pump(tester, client, ValueNotifier('55'));
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('商品評價'), findsOneWidget);
    expect(find.textContaining('商品評價 ('), findsNothing);
  });

  testWidgets('標題的評價數取自統計，不是第一頁的筆數', (tester) async {
    final client = FakeReviewPublic(
      pages: [
        [for (var i = 0; i < 100; i++) reviewRecord('r$i')],
      ],
      stats: pb.GetProductReviewStatsResponse(count: 230, average: 4.5),
    ).client();
    await pump(tester, client, ValueNotifier('55'));
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('商品評價 (230)'), findsOneWidget);
    expect(find.text('商品評價 (100)'), findsNothing);
  });

  testWidgets('統計讀取失敗時標題不顯示數字，評價預覽仍在', (tester) async {
    final client = FakeReviewPublic(
      pages: [
        [reviewRecord('r1', content: '好吃')],
      ],
      statsError: ConnectException(Code.unavailable, 'down'),
    ).client();
    await pump(tester, client, ValueNotifier('55'));
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('商品評價'), findsOneWidget);
    expect(find.textContaining('商品評價 ('), findsNothing);
    expect(find.text('好吃'), findsOneWidget);
  });

  testWidgets('自己的評價不顯示檢舉入口，別人的顯示；沒有暱稱顯示匿名名稱', (tester) async {
    final client = FakeReviewPublic(
      pages: [
        [
          reviewRecord('mine', content: '我的', isMine: true),
          reviewRecord('other', content: '別人的', authorName: '小明'),
        ],
      ],
    ).client();
    await pump(tester, client, ValueNotifier('55'));
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('我的'), findsOneWidget);
    expect(find.text('別人的'), findsOneWidget);
    expect(find.text('匿名會員'), findsOneWidget);
    expect(find.bySemanticsLabel('更多評價操作'), findsOneWidget);
  });

  testWidgets('skuId 換掉後重新載入，舊 sku 較晚回來的回應不覆蓋', (tester) async {
    final slow = <String, Completer>{};
    final client = ReviewPublicServiceClient(
      FakeTransportBuilder().unary(ReviewPublicService.listProductReviews, (
        req,
        _,
      ) async {
        if (req.productId == '55') await slow['55']!.future;
        return pb.ListProductReviewsResponse(
          reviews: [
            reviewRecord('r-${req.productId}', content: 'c-${req.productId}'),
          ],
        );
      }).build(),
    );
    slow['55'] = Completer<void>();
    final sku = ValueNotifier('55');
    await pump(tester, client, sku);
    await tester.pump();

    sku.value = '66';
    await tester.pump(const Duration(milliseconds: 100));
    slow['55']!.complete();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('c-66'), findsOneWidget);
    expect(find.text('c-55'), findsNothing);
  });
}
