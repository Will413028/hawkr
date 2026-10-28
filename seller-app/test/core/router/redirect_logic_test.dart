import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/router/redirect_logic.dart';
import 'package:hawkr_seller/core/router/route_capabilities.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../support/my_store_fixtures.dart';

/// `resolveAuthRedirect` 是 GoRouter redirect 的純函式決策核心。
///
/// 它是 onboarding gating 的「單一真相來源」：登入後要不要進主畫面、
/// 要不要被擋去 onboarding，全由它決定（取代原本 redirect 只看 JWT、
/// 而 navigateAfterAuth 另看 needsOnboarding 的雙頭馬車）。
void main() {
  group('resolveAuthRedirect — 已登入、GetMyStore 還沒回答', () {
    for (final location in ['/auth', '/welcome', '/main/dashboard']) {
      test('$location → 不導頁，等 GetMyStore 的答案', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: null,
            myStore: null,
            location: location,
          ),
          isNull,
        );
      });
    }
  });

  group('resolveAuthRedirect — 未登入', () {
    test('受保護頁 → 導去 /auth', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: false,
          myStore: null,
          location: '/main/dashboard',
        ),
        '/auth',
      );
    });

    test('白名單 /auth → 不重導', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: false,
          myStore: null,
          location: '/auth',
        ),
        isNull,
      );
    });

    test('白名單 /welcome → 不重導', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: false,
          myStore: null,
          location: '/welcome',
        ),
        isNull,
      );
    });

    test('白名單 /onboard → 不重導', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: false,
          myStore: null,
          location: '/onboard',
        ),
        isNull,
      );
    });
  });

  group('resolveAuthRedirect — 已登入且 needsOnboarding=true（刪除後重登 / 新申請）', () {
    test('在 /auth（重登核心情境）→ 導去 /onboard，而非 dashboard', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: '/auth',
        ),
        '/onboard',
      );
    });

    test('已在主畫面 → 仍被擋回 /onboard（gating 防穿透）', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: '/main/dashboard',
        ),
        '/onboard',
      );
    });

    test('主畫面子頁 → 仍被擋回 /onboard', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: '/main/orders',
        ),
        '/onboard',
      );
    });

    test('已在 /onboard → 不重導（避免循環）', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: '/onboard',
        ),
        isNull,
      );
    });

    test('在 /welcome（splash 自行 bootstrap）→ 不重導', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: '/welcome',
        ),
        isNull,
      );
    });

    // 措施與申覆是帳號層級：沒有店家的賣家也要能查看（開店頁的「你有措施紀錄」入口）。
    // 路由樣式（含路徑參數）而不是實際網址，所以表單的 location 是 `/seller/measures/:measureId`。
    for (final location in [
      SellerRoutes.measures,
      SellerRoutes.measureRequest,
      '/seller/measures/:measureId',
    ]) {
      test('措施頁 $location → 放行', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: true,
            myStore: null,
            location: location,
          ),
          isNull,
        );
      });
    }

    test('只放行措施頁：其他賣家頁面仍導去 /onboard', () {
      for (final location in [
        '/seller/suspension-notice',
        '/seller/measure',
        '/seller/measures/submit', // 不是路由樣式：實際網址不放行，只有宣告過的樣式放行
        '/main/dashboard',
      ]) {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: true,
            myStore: null,
            location: location,
          ),
          '/onboard',
          reason: location,
        );
      }
    });

    test('沒有店家也能進的頁面全部在 routeCapabilities 宣告為不需要能力', () {
      for (final route in routesAllowedWithoutStore) {
        expect(routeCapabilities.containsKey(route), isTrue, reason: route);
        expect(routeCapabilities[route], isNull, reason: route);
      }
      expect(routesAllowedWithoutStore, {
        AuthRoutes.onboard,
        AuthRoutes.welcome,
        SellerRoutes.measures,
        SellerRoutes.measureRequest,
      });
    });

    test('未登入仍不能看措施頁', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: null,
          myStore: null,
          location: '/seller/measures',
        ),
        '/auth',
      );
    });
  });

  group('resolveAuthRedirect — 已登入且 needsOnboarding=false（正常賣家）', () {
    test('在 /auth → 導去 dashboard', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: false,
          myStore: null,
          location: '/auth',
        ),
        '/main/dashboard',
      );
    });

    test('已在主畫面 → 不重導', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: false,
          myStore: null,
          location: '/main/dashboard',
        ),
        isNull,
      );
    });

    test('在 /welcome（splash 自行 bootstrap）→ 不重導', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: false,
          myStore: null,
          location: '/welcome',
        ),
        isNull,
      );
    });
  });

  group(
    'resolveAuthRedirect — 已登入且停權（46 號設計書 §7.2 停權 gating，能力來自 GetMyStore）',
    () {
      test('非白名單頁（dashboard）→ 導去停權公告頁', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/main/dashboard',
          ),
          '/seller/suspension-notice',
        );
      });

      test('白名單：訂單子頁 /order/details → 不重導', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/order/details',
          ),
          isNull,
        );
      });

      test('白名單：訂單 tab /main/orders → 不重導', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/main/orders',
          ),
          isNull,
        );
      });

      test('白名單：售後 /after-sales → 不重導', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/after-sales',
          ),
          isNull,
        );
      });

      test('白名單：通知 /notifications → 不重導', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/notifications',
          ),
          isNull,
        );
      });

      test('白名單：申覆表單（路由樣式）→ 不重導', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: SellerRoutes.measureRequest,
          ),
          isNull,
        );
      });

      test('白名單：申覆頁本身 → 不重導', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/seller/measures',
          ),
          isNull,
        );
      });

      test('白名單：公告頁本身 → 不重導（避免循環）', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/seller/suspension-notice',
          ),
          isNull,
        );
      });

      test('非白名單：客戶管理 /customers → 導去停權公告頁（deny-by-default）', () {
        expect(
          resolveAuthRedirect(
            isLoggedIn: true,
            needsOnboarding: false,
            myStore: suspendedStore(),
            location: '/customers',
          ),
          '/seller/suspension-notice',
        );
      });
    },
  );

  group('resolveAuthRedirect — needsForceUpdate（置於所有判斷最前）', () {
    test('未登入 + 需強更 → 導強更頁，不是 /auth', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: false,
          myStore: null,
          location: '/main/dashboard',
          needsForceUpdate: true,
        ),
        '/force-update',
      );
    });

    test('未登入且停在白名單 /auth + 需強更 → 仍導強更頁', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: false,
          myStore: null,
          location: '/auth',
          needsForceUpdate: true,
        ),
        '/force-update',
      );
    });

    test('needsOnboarding + 需強更 → 強更優先於 onboarding', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: '/auth',
          needsForceUpdate: true,
        ),
        '/force-update',
      );
    });

    test('onboarding 頁本身 + 需強更 → 強更優先（onboarding 白名單不豁免）', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: '/onboard',
          needsForceUpdate: true,
        ),
        '/force-update',
      );
    });

    test('停權 + 需強更 → 強更優先於停權公告頁', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: false,
          myStore: suspendedStore(),
          location: '/main/dashboard',
          needsForceUpdate: true,
        ),
        '/force-update',
      );
    });

    test('停權白名單頁（訂單）+ 需強更 → 強更優先，停權白名單不豁免強更', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: false,
          myStore: suspendedStore(),
          location: '/main/orders',
          needsForceUpdate: true,
        ),
        '/force-update',
      );
    });

    test('強更頁本身 → 不重導（避免循環）', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: suspendedStore(),
          location: '/force-update',
          needsForceUpdate: true,
        ),
        isNull,
      );
    });

    test('needsForceUpdate=false → 既有四條分支行為不變（未登入仍導 /auth）', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: false,
          needsOnboarding: false,
          myStore: null,
          location: '/main/dashboard',
          needsForceUpdate: false,
        ),
        '/auth',
      );
    });

    test('省略 needsForceUpdate（預設 false）→ 既有呼叫端行為不變', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: false,
          myStore: null,
          location: '/auth',
        ),
        '/main/dashboard',
      );
    });
  });

  group('resolveAuthRedirect — 依能力表導向公告頁', () {
    String? redirect(MyStore? store, String location) => resolveAuthRedirect(
      isLoggedIn: true,
      needsOnboarding: false,
      myStore: store,
      location: location,
    );

    test('停權且沒有 PRODUCT_MANAGE → 商品頁導去公告頁', () {
      expect(
        redirect(suspendedStore(), '/product/info'),
        '/seller/suspension-notice',
      );
      expect(
        redirect(suspendedStore(), '/settings/shipping'),
        '/seller/suspension-notice',
      );
    });

    test('停權但保留 ORDER_FULFILL → 訂單與出貨相關頁放行', () {
      for (final location in [
        '/main/orders',
        '/orders/view',
        '/order/logistics',
        '/after-sales/detail',
        '/shipment-anomalies',
      ]) {
        expect(redirect(suspendedStore(), location), isNull, reason: location);
      }
    });

    test('能力表只看 GetMyStore 給的能力：移除 ORDER_FULFILL 後訂單頁被擋', () {
      final store = myStore(
        capabilities: [SellerCapability.SELLER_CAPABILITY_NOTIFICATION],
        suspended: true,
      );
      expect(redirect(store, '/main/orders'), '/seller/suspension-notice');
      expect(redirect(store, '/notifications'), isNull);
    });

    test('停權保留 ACCOUNT_CLOSE → 商店設定頁放行；停權期間保留聊天讀取 → 聊天放行', () {
      expect(redirect(suspendedStore(), '/main/settings'), isNull);
      expect(redirect(suspendedStore(), '/profile'), isNull);
      expect(redirect(suspendedStore(), '/chat'), isNull);
    });

    test('等待刪除：經營類與商店設定導去公告頁，訂單與售後放行', () {
      final store = pendingDeletionStore();
      expect(redirect(store, '/main/dashboard'), '/seller/suspension-notice');
      expect(redirect(store, '/main/settings'), '/seller/suspension-notice');
      expect(redirect(store, '/product/info'), '/seller/suspension-notice');
      expect(redirect(store, '/main/orders'), isNull);
      expect(redirect(store, '/after-sales'), isNull);
      expect(redirect(store, '/notifications'), isNull);
    });

    test('KYC 已核准但停權：只有主能力缺 NOT_SUSPENDED 的路由導去公告頁', () {
      final store = myStore(
        suspended: true,
        publicListing: publicListingMissing([
          SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
        ]),
        shippingSetup: shippingSetupMissing([
          SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
        ]),
      );
      expect(
        redirect(store, '/settings/shipping'),
        '/seller/suspension-notice',
      );
      expect(redirect(store, '/product/info'), '/seller/suspension-notice');
      expect(redirect(store, '/main/orders'), isNull);
      expect(redirect(store, '/main/settings'), isNull);
    });

    test('KYC 未核准：物流設定頁放行（主能力是 PRODUCT_MANAGE），不導去公告頁', () {
      const kyc = SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED;
      final store = myStore(
        publicListing: publicListingMissing([kyc]),
        shippingSetup: shippingSetupMissing([kyc]),
      );
      expect(redirect(store, '/settings/shipping'), isNull);
      expect(redirect(store, '/product/info'), isNull);
    });

    test('主能力沒有啟用、但缺的條件不是停權／等待刪除／營業中：不導頁，交給頁內閘門', () {
      final store = MyStore(
        GetMyStoreResponse(
          store: Store(id: 'store-1', onboardingCompleted: true),
          capabilities: [
            CapabilityStatus(
              capability: SellerCapability.SELLER_CAPABILITY_MARKETING,
              missing: [SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED],
            ),
          ],
        ),
      );
      expect(redirect(store, '/main/dashboard'), isNull);
    });

    test('主能力缺 STORE_ACTIVE：導去公告頁', () {
      final store = MyStore(
        GetMyStoreResponse(
          store: Store(id: 'store-1', onboardingCompleted: true),
          capabilities: [
            CapabilityStatus(
              capability: SellerCapability.SELLER_CAPABILITY_MARKETING,
              missing: [SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE],
            ),
          ],
        ),
      );
      expect(redirect(store, '/main/dashboard'), '/seller/suspension-notice');
    });

    test('沒有宣告的路由不擋，即使停權或等待刪除', () {
      expect(redirect(suspendedStore(), '/not/declared'), isNull);
      expect(redirect(pendingDeletionStore(), '/not/declared'), isNull);
      expect(redirect(myStore(), '/not/declared'), isNull);
    });

    test('營業中的店家（全部能力）→ 任何已宣告的頁面都放行', () {
      for (final location in [
        '/main/dashboard',
        '/product/info',
        '/customers',
        '/chat',
      ]) {
        expect(redirect(myStore(), location), isNull, reason: location);
      }
    });

    test('能力全部未啟用、缺項為空 → 不擋（缺項不含停權、刪除、營業中）', () {
      expect(
        redirect(myStore(capabilities: const []), '/main/dashboard'),
        isNull,
      );
    });

    test('伺服器沒有回報能力狀態（missingFor 是 null）→ 不擋，交給伺服器', () {
      final store = myStore(suspended: true, reportCapabilities: false);
      expect(
        store.missingFor(SellerCapability.SELLER_CAPABILITY_MARKETING),
        isNull,
      );
      expect(redirect(store, '/main/dashboard'), isNull);
      expect(redirect(store, '/product/info'), isNull);
    });

    test('沒有店家（not_found）→ 沿用 needsOnboarding 的判斷，不走能力表', () {
      expect(
        resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: const MyStore.absent(),
          location: '/main/dashboard',
        ),
        '/onboard',
      );
    });

    test('尚未讀到店家（null）→ 不擋', () {
      expect(redirect(null, '/main/dashboard'), isNull);
    });
  });

  group('redirectLocationOf：含路徑參數的網址以路由樣式判斷', () {
    Future<GoRouter> pumpRouter(
      WidgetTester tester, {
      required bool needsOnboarding,
    }) async {
      final router = GoRouter(
        initialLocation: SellerRoutes.measureRequestFor('v 1/二'),
        redirect: (context, state) => resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: needsOnboarding,
          myStore: null,
          location: redirectLocationOf(state),
        ),
        routes: [
          GoRoute(
            path: SellerRoutes.measureRequest,
            builder: (_, state) =>
                Text('form:${state.pathParameters['measureId']}'),
          ),
          GoRoute(
            path: AuthRoutes.onboard,
            builder: (_, _) => const Text('onboard'),
          ),
        ],
      );
      addTearDown(router.dispose);
      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.pumpAndSettle();
      return router;
    }

    testWidgets('沒有店家：申覆表單放行，id 完整帶進路徑參數（含需要編碼的字元）', (tester) async {
      await pumpRouter(tester, needsOnboarding: true);
      expect(find.text('form:v 1/二'), findsOneWidget);
    });

    testWidgets('請求解除用同一個路由樣式（mode 查詢參數）：沒有店家也放行，模式由網址決定，其他值無效', (
      tester,
    ) async {
      final router = GoRouter(
        initialLocation: SellerRoutes.measureRemediationFor('v 1/二'),
        redirect: (context, state) => resolveAuthRedirect(
          isLoggedIn: true,
          needsOnboarding: true,
          myStore: null,
          location: redirectLocationOf(state),
        ),
        routes: [
          GoRoute(
            path: SellerRoutes.measureRequest,
            builder: (_, state) => Text(
              '${measureFormKindOf(state.uri)!.name}:'
              '${state.pathParameters['measureId']}',
            ),
          ),
          GoRoute(
            path: AuthRoutes.onboard,
            builder: (_, _) => const Text('onboard'),
          ),
        ],
      );
      addTearDown(router.dispose);
      await tester.pumpWidget(MaterialApp.router(routerConfig: router));
      await tester.pumpAndSettle();
      expect(find.text('remediation:v 1/二'), findsOneWidget);
      expect(
        measureFormKindOf(Uri.parse(SellerRoutes.measureRequestFor('x')))?.name,
        'appeal',
      );
      expect(
        measureFormKindOf(Uri.parse('/seller/measures/x?mode=other')),
        isNull,
      );
      expect(measureFormKindOf(Uri.parse('/seller/measures/x?mode=')), isNull);
      expect(
        measureRequestRedirect(Uri.parse('/seller/measures/x?mode=other')),
        SellerRoutes.measures,
      );
      expect(
        measureRequestRedirect(Uri.parse(SellerRoutes.measureRequestFor('x'))),
        isNull,
      );
      expect(
        measureRequestRedirect(
          Uri.parse(SellerRoutes.measureRemediationFor('x')),
        ),
        isNull,
      );
    });

    testWidgets('已有店家：同一個網址照樣放行', (tester) async {
      await pumpRouter(tester, needsOnboarding: false);
      expect(find.text('form:v 1/二'), findsOneWidget);
    });
  });
}
