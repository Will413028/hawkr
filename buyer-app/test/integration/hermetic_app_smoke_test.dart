@Tags(['hermetic-integration'])
library;

import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/routing/app_router.dart';
import 'package:hawkr_buyer/core/routing/buyer_navigation_observer.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/auth/presentation/auth_screen.dart';
import 'package:hawkr_buyer/features/category/presentation/category_screen.dart';
import 'package:hawkr_buyer/features/home/presentation/home_screen.dart';
import 'package:hawkr_buyer/features/product/presentation/search_screen.dart';
import 'package:hawkr_buyer/shared/widgets/buyer_navigation_frame.dart';
import 'package:hawkr_buyer/shared/widgets/shared_bottom_nav.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../support/fake_content_transport.dart';

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

/// A closed, deterministic transport for the routes covered by this smoke.
///
/// Every request is recorded. Unknown paths fail immediately and non-GET
/// methods are rejected, making accidental live mutation impossible.
class _HermeticAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];

  static const allowedPaths = {
    '/public/categories/tree',
    '/public/hashtags/popular',
    '/public/posts',
    '/public/products',
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
        'Hermetic smoke rejected ${options.method} ${options.path}',
      );
    }
    if (!allowedPaths.contains(options.path)) {
      throw StateError('Hermetic smoke has no fixture for ${options.path}');
    }

    final data = switch (options.path) {
      '/public/categories/tree' => {'data': <Object>[]},
      '/public/hashtags/popular' => {
        'data': {'records': <Object>[]},
      },
      '/public/posts' => {
        'data': {'records': <Object>[]},
      },
      '/public/products' => {
        'data': {'records': <Object>[], 'total': 0},
      },
      _ => throw StateError('unreachable fixture'),
    };
    return ResponseBody.fromString(
      jsonEncode(data),
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
  testWidgets(
    'production router renders against controlled fakes with no live writes',
    (tester) async {
      await HttpOverrides.runZoned(
        () async {
          tester.view.devicePixelRatio = 1;
          tester.view.physicalSize = const Size(390, 844);
          addTearDown(tester.view.resetDevicePixelRatio);
          addTearDown(tester.view.resetPhysicalSize);

          final adapter = _HermeticAdapter();
          final dio = Dio(BaseOptions(baseUrl: 'https://hermetic.invalid'))
            ..httpClientAdapter = adapter;
          addTearDown(dio.close);

          final container = ProviderContainer(
            overrides: [
              dioProvider.overrideWithValue(dio),
              connectHttpClientProvider.overrideWithValue(
                (_) => throw StateError('Hermetic smoke has no live Connect'),
              ),
              connectTransportProvider.overrideWithValue(
                fakeContentTransport(),
              ),
              sessionProvider.overrideWith(_GuestSession.new),
              appRouterObserversProvider.overrideWithValue([
                buyerNavigationObserver,
              ]),
            ],
          );
          addTearDown(container.dispose);

          final router = container.read(appRouterProvider);
          addTearDown(router.dispose);
          router.go('/search');

          await tester.pumpWidget(
            UncontrolledProviderScope(
              container: container,
              child: ScreenUtilInit(
                designSize: const Size(414, 896),
                enableScaleWH: () => false,
                enableScaleText: () => false,
                builder: (_, __) => MaterialApp.router(
                  routerConfig: router,
                  builder: (context, child) => BuyerNavigationFrame(
                    router: router,
                    navigatorKey: rootNavigatorKey,
                    navigationObserver: buyerNavigationObserver,
                    child: child!,
                  ),
                ),
              ),
            ),
          );
          await _pumpFrames(tester);
          expect(find.byType(SearchScreen), findsOneWidget);
          _expectNoFrameworkException(tester, 'search');

          router.go('/merchant-product');
          await _pumpFrames(tester);
          expect(find.byType(CategoryScreen), findsOneWidget);
          expect(router.routerDelegate.state.uri.path, '/');
          expect(router.routerDelegate.state.uri.queryParameters['tab'], '1');
          _expectNoFrameworkException(tester, 'marketplace redirect');

          router.go(
            '/marketplace?sort=${Uri.encodeQueryComponent('最新上架')}'
            '&q=${Uri.encodeQueryComponent('虱目魚')}',
          );
          await _pumpFrames(tester);
          expect(find.byType(CategoryScreen), findsOneWidget);
          expect(router.routerDelegate.state.uri.path, '/');
          expect(router.routerDelegate.state.uri.queryParameters, {
            'sort': 'LATEST',
            'q': '虱目魚',
            'tab': '1',
          });
          expect(
            adapter.requests.where(
              (request) =>
                  request.path == '/public/products' &&
                  request.queryParameters['page_size'] == 20 &&
                  request.queryParameters['keyword'] == '虱目魚' &&
                  request.queryParameters['sort'] == 'LATEST',
            ),
            hasLength(1),
          );
          _expectNoFrameworkException(tester, 'canonical marketplace intent');

          await tester.tap(
            find.byKey(const ValueKey('buyer-bottom-nav-item-0')),
          );
          await _pumpFrames(tester);
          expect(find.byType(HomeScreen), findsOneWidget);
          expect(router.routerDelegate.state.uri.queryParameters['tab'], '0');
          expect(
            tester
                .widget<SharedBottomNav>(find.byType(SharedBottomNav))
                .currentIndex,
            0,
          );

          await tester.ensureVisible(
            find.byKey(const ValueKey('home-products-see-more')),
          );
          await tester.tap(
            find.byKey(const ValueKey('home-products-see-more')),
          );
          await _pumpFrames(tester);
          expect(find.byType(CategoryScreen), findsOneWidget);
          expect(router.routerDelegate.state.uri.queryParameters['tab'], '1');
          expect(
            router.routerDelegate.state.uri.queryParameters['sort'],
            'LATEST',
          );

          // Repeating the same destination catches stale root URIs: if the
          // PageView switched without updating `?tab=`, this second CTA would
          // target the location GoRouter already believes is active.
          await tester.tap(
            find.byKey(const ValueKey('buyer-bottom-nav-item-0')),
          );
          await _pumpFrames(tester);
          expect(find.byType(HomeScreen), findsOneWidget);
          expect(router.routerDelegate.state.uri.queryParameters['tab'], '0');

          await tester.ensureVisible(
            find.byKey(const ValueKey('home-products-see-more')),
          );
          await tester.tap(
            find.byKey(const ValueKey('home-products-see-more')),
          );
          await _pumpFrames(tester);
          expect(find.byType(CategoryScreen), findsOneWidget);
          expect(router.routerDelegate.state.uri.queryParameters, {
            'tab': '1',
            'sort': 'LATEST',
          });
          _expectNoFrameworkException(
            tester,
            'market to home and repeatedly back to the canonical market URI',
          );

          router.go('/cart');
          await _pumpFrames(tester);
          expect(find.byType(AuthScreen), findsOneWidget);
          expect(
            router.routeInformationProvider.value.uri.path,
            '/login',
            reason:
                'guest account routes must fail closed at the production guard',
          );
          _expectNoFrameworkException(tester, 'guest guard');

          expect(adapter.requests, isNotEmpty);
          expect(adapter.requests.map((request) => request.method).toSet(), {
            'GET',
          });
          expect(
            adapter.requests.map((request) => request.path).toSet(),
            everyElement(isIn(_HermeticAdapter.allowedPaths)),
          );
        },
        createHttpClient: (_) {
          throw StateError('External network is disabled in hermetic tests');
        },
      );
    },
  );
}

Future<void> _pumpFrames(WidgetTester tester) async {
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

void _expectNoFrameworkException(WidgetTester tester, String route) {
  final exceptions = <Object>[];
  Object? exception;
  while ((exception = tester.takeException()) != null) {
    exceptions.add(exception!);
  }
  expect(
    exceptions,
    isEmpty,
    reason: '$route should render without exceptions',
  );
}
