import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/features/post/data/shop_tab_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_shop_tab.dart';
import 'package:hawkr_seller/features/seller/data/onboarding_tasks.dart';
import 'package:hawkr_seller/shared/widgets/app_bottom_navigation_bar.dart';
import 'package:hawkr_seller/shared/widgets/seller_floating_navigation_layout.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets('add-product action stays above the visible seller Navbar', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          shopTabRepositoryProvider.overrideWithValue(
            const _ReadyShopRepository(),
          ),
          onboardingInputsProvider.overrideWith(
            (ref) async => const OnboardingInputs(),
          ),
        ],
        child: ScreenUtilInit(
          designSize: const Size(414, 896),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          minTextAdapt: false,
          splitScreenMode: true,
          builder: (_, _) => MaterialApp(
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(context).copyWith(
                padding: const EdgeInsets.only(bottom: 34),
                viewPadding: const EdgeInsets.only(bottom: 34),
              ),
              child: child!,
            ),
            home: SellerFloatingNavigationLayout(
              page: const PageShopTab(sellerId: '7'),
              navigation: AppBottomNavigationBar(
                currentIndex: NavIndex.category,
                onTap: (_) {},
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();

    final action = tester.getRect(
      find.byKey(const ValueKey('seller-add-product-floating-action')),
    );
    final navbar = tester.getRect(
      find.byKey(const ValueKey('seller-bottom-nav-surface')),
    );

    expect(action.height, greaterThanOrEqualTo(HkControl.minTap));
    expect(action.bottom, lessThanOrEqualTo(navbar.top - HkSpace.md));
    expect(tester.takeException(), isNull);

    await tester.pumpWidget(const SizedBox.shrink());
  });
}

class _ReadyShopRepository implements ShopTabRepository {
  const _ReadyShopRepository();

  @override
  Future<List<Map<String, dynamic>>> readCachedProducts(String sellerId) async {
    return const [];
  }

  @override
  Future<List<Map<String, dynamic>>> refreshProducts(String sellerId) async {
    return const [];
  }
}
