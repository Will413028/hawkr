@Tags(['hermetic-integration'])
library;

import 'dart:async';
import 'dart:convert';

import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart'
    show Account, GetSessionResponse;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart'
    show Dashboard, GetMyDashboardResponse;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/app.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/session/auth_session_provider.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/presentation/auth_screen.dart';
import 'package:hawkr_seller/features/dashboard/presentation/dashboard_screen.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../support/fake_auth_session.dart';
import '../support/my_store_fixtures.dart';
import '../support/default_error_text.dart';

/// Closed transport used by the production-router smoke.
///
/// Unknown endpoints and every mutating method fail immediately, so this test
/// cannot silently escape to Hawkr dev/prod or alter seller data.
class _HermeticSellerAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];

  static const allowedGetPaths = {
    '/api/v1/app/chat/unread',
    '/api/v1/app/notifications/unread-count',
    '/api/v1/app/shipments/anomalies/stats',
  };

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (options.method != 'GET') {
      throw StateError(
        'Hermetic seller smoke rejected '
        '${options.method} ${options.path}',
      );
    }
    if (!allowedGetPaths.contains(options.path)) {
      throw StateError(
        'Hermetic seller smoke has no fixture for ${options.path}',
      );
    }

    final Object body = switch (options.path) {
      '/api/v1/app/chat/unread' => {'totalUnreadCount': 0},
      '/api/v1/app/notifications/unread-count' => {
        'code': 0,
        'data': {'count': 0},
      },
      '/api/v1/app/shipments/anomalies/stats' => {
        'code': 0,
        'data': <String, Object>{},
      },
      _ => throw StateError('unreachable fixture'),
    };

    return ResponseBody.fromString(
      jsonEncode(body),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  final secureStore = <String, String>{};

  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
          (call) async {
            final arguments =
                (call.arguments as Map?)?.cast<String, dynamic>() ??
                const <String, dynamic>{};
            final key = arguments['key'] as String?;
            switch (call.method) {
              case 'read':
                return key == null ? null : secureStore[key];
              case 'write':
                if (key != null) {
                  secureStore[key] = arguments['value'] as String;
                }
                return null;
              case 'delete':
                if (key != null) secureStore.remove(key);
                return null;
              case 'deleteAll':
                secureStore.clear();
                return null;
              case 'readAll':
                return Map<String, String>.from(secureStore);
              case 'containsKey':
                return key != null && secureStore.containsKey(key);
            }
            return null;
          },
        );
  });

  setUp(() {
    secureStore.clear();
    SharedPreferences.setMockInitialValues({});
  });

  testWidgets(
    'logged-out cold start and login session transition use production router',
    (tester) async {
      final harness = _SellerAppHarness();
      await harness.pump(tester);
      await _pumpFrames(tester);

      expect(find.byType(AuthScreen), findsOneWidget);
      expect(harness.router.routeInformationProvider.value.uri.path, '/auth');
      expect(harness.adapter.requests, isEmpty);
      _expectNoFrameworkException(tester, 'logged-out cold start');

      // login 會等 GetMyStore 回來；在 fake async 裡不直接 await，交給下面的 pump 推進。
      unawaited(
        harness.container
            .read(sessionProvider.notifier)
            .login(accountId: 'acc-42'),
      );
      await _pumpFrames(tester, count: 20);

      expect(find.byType(DashboardScreen), findsOneWidget);
      expect(
        harness.router.routeInformationProvider.value.uri.path,
        MainRoutes.dashboard,
      );
      expect(
        harness.adapter.requests.map((request) => request.method).toSet(),
        {'GET'},
      );
      expect(harness.container.read(sellerStoreIdProvider), '42');
      expect(
        harness.container.read(myStoreSnapshotProvider)?.storeCode,
        'HERMETIC01',
      );
      _expectNoFrameworkException(tester, 'login session transition');

      await harness.dispose(tester);
    },
  );

  testWidgets(
    'encrypted session restore reaches dashboard using controlled GET fixtures',
    (tester) async {
      SharedPreferences.setMockInitialValues({
        // 這些是 AuthStorage 的本地 SharedPreferences key（`auth_storage.dart`），
        // 不是 wire 欄位——不隨 Gateway 契約 camelCase 收斂。
        'account_id': 'acc-42',
        'seller_id': 42,
        'seller_name': 'Hermetic Seller',
      });
      final harness = _SellerAppHarness(authSignedIn: true);
      tester.view.padding = const FakeViewPadding(top: 62, bottom: 34);
      tester.view.viewPadding = const FakeViewPadding(top: 62, bottom: 34);
      addTearDown(tester.view.resetPadding);
      addTearDown(tester.view.resetViewPadding);
      await harness.pump(tester, size: const Size(402, 874));
      await _pumpFrames(tester, count: 24);

      final glass = find.byKey(const ValueKey('seller-dashboard-glass-header'));
      expect(tester.getTopLeft(glass).dy, 0);
      expect(tester.getTopLeft(find.byType(DashboardHeader)).dy, 62);
      expect(
        tester.getSize(glass).height,
        tester.getSize(find.byType(DashboardHeader)).height + 62,
      );

      expect(find.byType(DashboardScreen), findsOneWidget);
      expect(
        harness.router.routeInformationProvider.value.uri.path,
        MainRoutes.dashboard,
      );
      // 冷啟動恢復：session token 登入中 → GetSession（Connect，獨立的 session client）取回帳號，再讀 GetMyStore。
      expect(harness.connectCalls, [
        IdentityAppService.getSession.procedure,
        SellerAppService.getMyStore.procedure,
      ]);
      // 儀表板第一次開啟只讀一次（第一次讀取與「成為可見」的重讀共用一輪）。
      expect(harness.dashboardCalls, 1);
      expect(harness.adapter.requests, isNotEmpty);
      expect(
        harness.adapter.requests.map((request) => request.method).toSet(),
        {'GET'},
      );
      expect(
        harness.adapter.requests.map((request) => request.path),
        everyElement(isIn(_HermeticSellerAdapter.allowedGetPaths)),
      );
      expect(
        harness.container.read(myStoreSnapshotProvider)?.storeCode,
        'HERMETIC01',
      );
      _expectNoFrameworkException(tester, 'encrypted session restore');

      await harness.dispose(tester);
    },
  );
}

class _SellerAppHarness {
  _SellerAppHarness({bool authSignedIn = false})
    : connectCalls = [],
      adapter = _HermeticSellerAdapter(),
      dio = Dio(BaseOptions(baseUrl: 'https://hermetic.invalid')) {
    dio.httpClientAdapter = adapter;
    final auth = FakeAuthSession(signedIn: authSignedIn);
    container = ProviderContainer(
      overrides: [
        authSessionProvider.overrideWith((ref) async => auth),
        dioProvider.overrideWithValue(dio),
        // Connect 只開放 GetSession 與 GetMyStore；其他 procedure 沒註冊就直接失敗。
        myStoreRepositoryProvider.overrideWithValue(
          MyStoreRepository(
            SellerAppServiceClient(
              FakeTransportBuilder().unary(SellerAppService.getMyStore, (
                _,
                __,
              ) {
                connectCalls.add(SellerAppService.getMyStore.procedure);
                return myStoreResponse(
                  id: '42',
                  name: 'Hermetic Farm',
                  storeCode: 'HERMETIC01',
                );
              }).build(),
            ),
            defaults: zhDefaultErrorText,
          ),
        ),
        sessionIdentityClientProvider.overrideWithValue(
          IdentityAppServiceClient(
            FakeTransportBuilder().unary(IdentityAppService.getSession, (
              _,
              __,
            ) {
              connectCalls.add(IdentityAppService.getSession.procedure);
              return GetSessionResponse(account: Account(id: 'acc-42'));
            }).build(),
          ),
        ),
        // 儀表板走 Connect（GetMyDashboard）；一輪只讀一次，由 [dashboardCalls] 計次。
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyDashboard, (
              _,
              __,
            ) {
              dashboardCalls++;
              return GetMyDashboardResponse(dashboard: Dashboard());
            }).build(),
          ),
        ),
        postLoginSideEffectsProvider.overrideWithValue(
          const NoopPostLoginSideEffects(),
        ),
        appRouterObserversProvider.overrideWithValue(<NavigatorObserver>[]),
      ],
    );
    router = container.read(goRouterProvider);
  }

  final List<String> connectCalls;
  int dashboardCalls = 0;
  final _HermeticSellerAdapter adapter;
  final Dio dio;
  late final ProviderContainer container;
  late final GoRouter router;

  Future<void> pump(WidgetTester tester, {Size size = const Size(390, 844)}) {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = size;
    return tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MyApp(runtimeSideEffectsEnabled: false),
      ),
    );
  }

  Future<void> dispose(WidgetTester tester) async {
    await tester.pumpWidget(const SizedBox.shrink());
    tester.view.resetDevicePixelRatio();
    tester.view.resetPhysicalSize();
    router.dispose();
    container.dispose();
    dio.close(force: true);
  }
}

Future<void> _pumpFrames(WidgetTester tester, {int count = 12}) async {
  for (var index = 0; index < count; index++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

void _expectNoFrameworkException(WidgetTester tester, String phase) {
  final exceptions = <Object>[];
  Object? exception;
  while ((exception = tester.takeException()) != null) {
    exceptions.add(exception!);
  }
  expect(
    exceptions,
    isEmpty,
    reason: '$phase should render without framework/layout exceptions',
  );
}
