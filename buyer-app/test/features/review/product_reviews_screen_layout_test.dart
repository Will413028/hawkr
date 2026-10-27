import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/review/data/review_repository.dart';
import 'package:hawkr_buyer/features/review/presentation/product_reviews_screen.dart';
import 'package:hawkr_buyer/shared/widgets/hk_choice_chip.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _FakeReviewRepository extends ReviewRepository {
  _FakeReviewRepository() : super(Dio());

  @override
  Future<List<Map<String, dynamic>>> getMerchantProductReviews(
    String skuId,
  ) async {
    return [
      {
        'reviewId': 'review-1',
        'buyerId': 'buyer-1234',
        'username': '陳**',
        'rating': 5,
        'content': '水果新鮮，包裝也很完整。',
        'createdAt': '2026-07-19T08:00:00Z',
        'images': const [],
      },
    ];
  }
}

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
          reviewRepositoryProvider.overrideWithValue(_FakeReviewRepository()),
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
}
