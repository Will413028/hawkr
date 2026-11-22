import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/features/review/data/review_repository.dart';
import 'package:hawkr_seller/features/product/presentation/product_reviews_screen.dart';
import 'package:hawkr_seller/shared/widgets/hk_choice_chip.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _FakeReviewRepository implements ReviewRepository {
  @override
  Future<ReviewPage> listProductReviews(
    String productId, {
    String pageToken = '',
    int? rating,
    bool withImages = false,
  }) async => ReviewPage(
    reviews: [
      ReviewItem(
        id: 'review-1',
        productId: productId,
        rating: 5,
        content: '水果新鮮，包裝也很完整。',
        imageUrls: const [],
        authorName: '陳**',
        authorAvatarUrl: '',
        createTime: DateTime.utc(2026, 7, 19, 8),
      ),
    ],
    nextPageToken: '',
  );

  @override
  Future<ReviewStats> productStats(String productId) async =>
      ReviewStats.fromProto(1, 5, const []);

  @override
  dynamic noSuchMethod(Invocation invocation) => throw UnimplementedError();
}

void main() {
  testWidgets('seller review page uses adaptive slivers and all filters', (
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
          builder: (_, _) => const MaterialApp(
            home: ProductReviewsScreen(
              sellerId: '123',
              skuId: 'sku-1',
              productName: '範例安心蔬果箱',
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('seller-product-reviews-scroll-view')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('seller-product-reviews-summary-card')),
      findsOneWidget,
    );
    final background = tester.widget<ColoredBox>(
      find.byKey(const ValueKey('seller-product-reviews-background')),
    );
    expect(background.color, AppColors.bgPrimary);
    expect(find.byType(HkChoiceChip), findsNWidgets(7));
    expect(find.text('顧客回饋'), findsOneWidget);
    expect(find.text('水果新鮮，包裝也很完整。'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}
