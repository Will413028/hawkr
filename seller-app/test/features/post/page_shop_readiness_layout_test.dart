import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/post/data/shop_tab_repository.dart';
import 'package:hawkr_seller/features/post/presentation/widgets/page_shop_tab.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/onboarding_tasks.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_my_store.dart';
import '../../support/my_store_fixtures.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _NoProducts implements ShopTabRepository {
  const _NoProducts();

  @override
  Future<List<Map<String, dynamic>>> readCachedProducts(
    String sellerId,
  ) async => [];

  @override
  Future<List<Map<String, dynamic>>> refreshProducts(String sellerId) async =>
      [];
}

class _Case {
  const _Case({
    required this.missing,
    required this.status,
    required this.expectedText,
  });

  final List<SellerRequirement> missing;
  final KYCFlowStatus status;
  final String expectedText;
}

const _cases = <String, _Case>{
  '尚未認證': _Case(
    missing: [SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED],
    status: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
    expectedText: '待認證',
  ),
  '認證審核中': _Case(
    missing: [SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED],
    status: KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW,
    expectedText: 'PayUni 審核中',
  ),
  '缺商店資料與停權': _Case(
    missing: [
      SellerRequirement.SELLER_REQUIREMENT_STORE_NAME,
      SellerRequirement.SELLER_REQUIREMENT_LOGO,
      SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE,
      SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
      SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
    ],
    status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
    expectedText: 'PayUni 審核未通過',
  ),
};

Future<void> _pump(
  WidgetTester tester,
  _Case entry,
  double width, {
  GoRouter? router,
}) async {
  final server = FakeMyStoreServer(
    response: myStoreResponse(
      publicListing: publicListingMissing(entry.missing),
    ),
  );
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(_LoggedIn.new),
      myStoreRepositoryProvider.overrideWithValue(server.repository),
      shopTabRepositoryProvider.overrideWithValue(const _NoProducts()),
      onboardingInputsProvider.overrideWith(
        (ref) async => const OnboardingInputs(),
      ),
      sellerClientProvider.overrideWithValue(
        SellerAppServiceClient(
          FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) {
            return GetMyKYCResponse(
              application: KYCApplication(flow: KYCFlow(status: entry.status)),
            );
          }).build(),
        ),
      ),
    ],
  );
  addTearDown(container.dispose);
  final sub = container.listen(currentStoreProvider, (_, _) {});
  addTearDown(sub.close);
  await container.read(currentStoreProvider.future);

  tester.view.physicalSize = Size(width, 874);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => router == null
            ? MaterialApp(
                theme: AppTheme.light,
                builder: _scaled,
                home: const Scaffold(body: PageShopTab(sellerId: 'store-1')),
              )
            : MaterialApp.router(
                theme: AppTheme.light,
                builder: _scaled,
                routerConfig: router,
              ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Widget _scaled(BuildContext context, Widget? child) => MediaQuery(
  data: MediaQuery.of(
    context,
  ).copyWith(textScaler: const TextScaler.linear(1.4)),
  child: child!,
);

void main() {
  testWidgets('還沒通過認證也能從店面新增商品，直接進商品選擇頁', (tester) async {
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) =>
              const Scaffold(body: PageShopTab(sellerId: 'store-1')),
        ),
        GoRoute(
          path: '/product/picker',
          builder: (_, _) => const Scaffold(body: Text('商品選擇頁')),
        ),
        GoRoute(
          path: '/main/wallet',
          builder: (_, _) => const Scaffold(body: Text('認證頁')),
        ),
      ],
    );
    addTearDown(router.dispose);
    await _pump(tester, _cases['尚未認證']!, 402, router: router);

    await tester.tap(
      find.byKey(const ValueKey('seller-add-product-floating-action')),
    );
    await tester.pumpAndSettle();

    expect(find.text('商品選擇頁'), findsOneWidget);
    expect(find.text('認證頁'), findsNothing);
  });

  for (final entry in _cases.entries) {
    for (final width in [320.0, 402.0, 440.0]) {
      testWidgets('${entry.key} fits ${width}pt at 1.4x text', (tester) async {
        await _pump(tester, entry.value, width);

        expect(find.text('賣家開店任務'), findsOneWidget);
        expect(find.text('公開前必須'), findsOneWidget);
        expect(find.text('建議完成'), findsOneWidget);
        expect(find.text(entry.value.expectedText), findsOneWidget);
        expect(find.text('您尚未實名認證'), findsNothing);
        expect(find.text('開始販售前的準備'), findsNothing);

        final action = find.byKey(
          const ValueKey('seller-add-product-floating-action'),
        );
        expect(action, findsOneWidget);
        expect(tester.getSize(action).height, greaterThanOrEqualTo(48));
        expect(tester.takeException(), isNull);
      });
    }
  }
}
