import 'package:hawkr_buyer/l10n/l10n.dart';
import 'package:dio/dio.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/l10n/ui_locale.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/features/review/presentation/product_reviews_screen.dart';
import 'package:hawkr_buyer/shared/widgets/hk_choice_chip.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'review_fakes.dart';

void main() {
  testWidgets('review page uses adaptive slivers and complete rating filters', (
    tester,
  ) async {
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          dioProvider.overrideWithValue(Dio()),
          reviewAppClientProvider.overrideWithValue(FakeReviewApp().client()),
          reviewPublicClientProvider.overrideWithValue(
            FakeReviewPublic(
              pages: [
                [
                  reviewRecord(
                    'review-1',
                    content: '水果新鮮，包裝也很完整。',
                    authorName: '陳**',
                  ),
                ],
              ],
            ).client(),
          ),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: ProductReviewsScreen(
              merchantId: 'merchant-1',
              skuId: 'sku-1',
              productName: '範例安心蔬果箱',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('product-reviews-scroll-view')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('product-reviews-summary-card')),
      findsOneWidget,
    );
    final background = tester.widget<ColoredBox>(
      find.byKey(const ValueKey('product-reviews-background')),
    );
    expect(background.color, AppColors.bgPrimary);
    expect(find.byType(HkChoiceChip), findsNWidgets(7));
    expect(find.text('顧客回饋'), findsOneWidget);
    expect(find.text('水果新鮮，包裝也很完整。'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('讀取失敗顯示失敗與重試，不顯示成沒有評價；重試成功後顯示評價', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        [reviewRecord('review-1', content: '好吃', authorName: '陳**')],
      ],
      error: ConnectException(Code.internal, 'boom'),
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          dioProvider.overrideWithValue(Dio()),
          reviewAppClientProvider.overrideWithValue(FakeReviewApp().client()),
          reviewPublicClientProvider.overrideWithValue(fake.client()),
          defaultErrorTextProvider.overrideWithValue((_) => '服務暫時不可用'),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: ProductReviewsScreen(
              merchantId: 'merchant-1',
              skuId: 'sku-1',
              productName: '範例',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('product-reviews-load-failure')),
      findsOneWidget,
    );
    expect(find.text('服務暫時不可用'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('product-reviews-empty-state')),
      findsNothing,
    );

    fake.error = null;
    await tester.tap(find.byKey(const ValueKey('product-reviews-retry')));
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('product-reviews-load-failure')),
      findsNothing,
    );
    expect(find.text('好吃'), findsOneWidget);
  });

  testWidgets('總數與星等分布取自統計，超過 100 則時不是已載入的筆數', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        [for (var i = 0; i < 3; i++) reviewRecord('r$i')],
      ],
      stats: pb.GetProductReviewStatsResponse(
        count: 230,
        average: 4.5,
        distribution: [
          pb.RatingCount(rating: 5, count: 200),
          pb.RatingCount(rating: 1, count: 30),
        ],
      ),
    );
    await _pumpScreen(tester, fake);

    expect(find.textContaining('230'), findsWidgets);
    expect(find.textContaining('200'), findsWidgets);
    expect(find.text('4.5'), findsWidgets);
  });

  testWidgets('統計讀取失敗顯示失敗與重試，不顯示成 0 則評價', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        [reviewRecord('r1', content: '好吃')],
      ],
      statsError: ConnectException(Code.internal, 'boom'),
    );
    await _pumpScreen(tester, fake);

    expect(
      find.byKey(const ValueKey('product-reviews-load-failure')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('product-reviews-summary-card')),
      findsNothing,
    );

    fake.statsError = null;
    await tester.tap(find.byKey(const ValueKey('product-reviews-retry')));
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('product-reviews-summary-card')),
      findsOneWidget,
    );
  });

  testWidgets('選星等與有圖片篩選時，由伺服器篩選並從第一頁重讀', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        [reviewRecord('r1', rating: 5)],
      ],
    );
    await _pumpScreen(tester, fake);
    expect(fake.requests.single.hasRating(), isFalse);
    expect(fake.requests.single.withImages, isFalse);

    await tester.tap(find.byKey(const ValueKey('product-review-filter-4星')));
    await tester.pumpAndSettle();
    expect(fake.requests.last.rating, 4);
    expect(fake.requests.last.withImages, isFalse);
    expect(fake.requests.last.pageToken, '');

    final imageChip = find.byKey(const ValueKey('product-review-filter-有圖片'));
    await tester.ensureVisible(imageChip);
    await tester.pumpAndSettle();
    await tester.tap(imageChip);
    await tester.pumpAndSettle();
    expect(fake.requests.last.hasRating(), isFalse);
    expect(fake.requests.last.withImages, isTrue);
    expect(fake.requests.length, 3);
    expect(fake.statsRequests.length, 1);
  });

  testWidgets('有圖片篩選顯示統計的附圖筆數，標題筆數也隨之顯示', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        [reviewRecord('r1', rating: 5)],
      ],
      stats: pb.GetProductReviewStatsResponse(
        count: 230,
        average: 4.5,
        withImagesCount: 37,
      ),
    );
    await _pumpScreen(tester, fake);

    final imageChip = find.byKey(const ValueKey('product-review-filter-有圖片'));
    await tester.ensureVisible(imageChip);
    await tester.pumpAndSettle();
    expect(
      find.descendant(of: imageChip, matching: find.text('有圖片 (37)')),
      findsOneWidget,
    );
    await tester.tap(imageChip);
    await tester.pumpAndSettle();
    expect(find.text('37 則'), findsOneWidget);
  });

  testWidgets('沒有暱稱的作者顯示共用文案的匿名名稱', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        [reviewRecord('r1', content: '好吃')],
      ],
    );
    await _pumpScreen(tester, fake);

    expect(find.text('匿名會員'), findsOneWidget);
  });

  testWidgets('依 next_page_token 載入更多，超過 10 頁也不截斷', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        for (var i = 0; i < 12; i++) [reviewRecord('r$i', content: '內容$i')],
      ],
    );
    await _pumpScreen(tester, fake);

    final more = find.byKey(const ValueKey('product-reviews-more'));
    var taps = 0;
    while (true) {
      await tester.scrollUntilVisible(
        more,
        300,
        scrollable: find.byType(Scrollable).first,
        continuous: true,
      );
      await tester.ensureVisible(more);
      await tester.pumpAndSettle();
      await tester.tap(more);
      await tester.pumpAndSettle();
      taps++;
      if (fake.requests.length == 12 || taps > 20) break;
    }

    expect(taps, 11);
    expect(fake.requests.map((r) => r.pageToken), [
      '',
      for (var i = 1; i < 12; i++) 'page-$i',
    ]);
    await tester.scrollUntilVisible(
      find.text('內容11'),
      300,
      scrollable: find.byType(Scrollable).first,
    );
    expect(find.text('內容11'), findsOneWidget);
  });

  testWidgets('下一頁失敗保留已載入的評價並可再試', (tester) async {
    final fake = FakeReviewPublic(
      pages: [
        [reviewRecord('r0', content: '第一頁')],
        [reviewRecord('r1', content: '第二頁')],
      ],
      failTokens: {'page-1'},
    );
    await _pumpScreen(tester, fake);

    final more = find.byKey(const ValueKey('product-reviews-more'));
    await tester.scrollUntilVisible(
      more,
      300,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.ensureVisible(more);
    await tester.pumpAndSettle();
    await tester.tap(more);
    await tester.pumpAndSettle();

    expect(find.text('第一頁'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('product-reviews-load-failure')),
      findsNothing,
    );
    expect(more, findsOneWidget);

    fake.failTokens = {};
    await tester.pump(const Duration(seconds: 10));
    await tester.pumpAndSettle();
    await tester.scrollUntilVisible(
      more,
      300,
      scrollable: find.byType(Scrollable).first,
    );
    await tester.ensureVisible(more);
    await tester.pumpAndSettle();
    await tester.tap(more);
    await tester.pumpAndSettle();
    expect(find.text('第二頁', skipOffstage: false), findsOneWidget);
    expect(fake.requests.map((r) => r.pageToken), ['', 'page-1', 'page-1']);
  });

  testWidgets('沒有評價是空狀態，不是失敗', (tester) async {
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          dioProvider.overrideWithValue(Dio()),
          reviewAppClientProvider.overrideWithValue(FakeReviewApp().client()),
          reviewPublicClientProvider.overrideWithValue(
            FakeReviewPublic().client(),
          ),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => const MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            home: ProductReviewsScreen(
              merchantId: 'merchant-1',
              skuId: 'sku-1',
              productName: '範例',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('product-reviews-load-failure')),
      findsNothing,
    );
    expect(
      find.byKey(const ValueKey('product-reviews-empty-state')),
      findsOneWidget,
    );
  });
}

Future<void> _pumpScreen(WidgetTester tester, FakeReviewPublic fake) async {
  tester.view.physicalSize = const Size(390, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(Dio()),
        reviewAppClientProvider.overrideWithValue(FakeReviewApp().client()),
        reviewPublicClientProvider.overrideWithValue(fake.client()),
        defaultErrorTextProvider.overrideWithValue((_) => '服務暫時不可用'),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => const MaterialApp(
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          home: ProductReviewsScreen(
            merchantId: 'merchant-1',
            skuId: 'sku-1',
            productName: '範例',
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}
