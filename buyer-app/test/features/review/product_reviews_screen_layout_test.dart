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
