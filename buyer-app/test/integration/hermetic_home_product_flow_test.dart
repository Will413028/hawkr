@Tags(['hermetic-integration'])
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/home/presentation/home_product_controller.dart';
import 'package:hawkr_buyer/features/home/presentation/home_screen.dart';
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

/// A deterministic transport for the production home product data flow.
///
/// Product responses are deliberately controlled by the test so assertions
/// can observe the in-flight skeleton and every terminal state. Any unrelated
/// request fails immediately instead of leaking to a live service.
class _ControlledHomeAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];
  final List<Completer<ResponseBody>> _productResponses = [];

  Completer<ResponseBody> queueProductResponse() {
    final response = Completer<ResponseBody>();
    _productResponses.add(response);
    return response;
  }

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    if (options.method != 'GET') {
      throw StateError(
        'Hermetic home test rejected ${options.method} ${options.path}',
      );
    }
    if (options.path == '/public/posts') {
      return _jsonResponse({
        'code': 0,
        'data': {'records': <Object>[]},
        'message': 'success',
      });
    }
    if (options.path == '/public/products') {
      if (_productResponses.isEmpty) {
        throw StateError('No scripted product response is available');
      }
      return _productResponses.removeAt(0).future;
    }
    throw StateError('Hermetic home test has no fixture for ${options.path}');
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets(
    'home skeleton becomes a product grid after the real repository succeeds',
    (tester) async {
      final harness = await _pumpHome(tester);
      final response = harness.adapter.queueProductResponse();
      await harness.remount();

      expect(
        find.byKey(const ValueKey('home-product-grid-loading')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('home-product-grid')), findsNothing);

      response.complete(
        _jsonResponse({
          'code': 0,
          'data': {
            'records': [
              {
                'id': 'product-1',
                'name': '測試高麗菜',
                'imageKey': '',
                'priceMin': '120',
                'priceMax': '150',
                'seller': {'storeId': 'seller-1', 'name': '測試小農'},
              },
            ],
            'page': '1',
            'pageSize': '6',
            'total': '1',
          },
          'message': 'success',
        }),
      );
      await _pumpUntil(
        tester,
        find.byKey(const ValueKey('home-product-product-1')),
      );

      expect(
        find.byKey(const ValueKey('home-product-grid-loading')),
        findsNothing,
      );
      expect(find.byKey(const ValueKey('home-product-grid')), findsOneWidget);
      expect(find.text('測試高麗菜'), findsOneWidget);
      expect(
        harness.container.read(homeProductControllerProvider).isInitialLoading,
        isFalse,
      );
      _expectFirstPageRequest(harness.adapter, expectedCount: 1);
    },
  );

  testWidgets('empty response replaces the home skeleton with an empty state', (
    tester,
  ) async {
    final harness = await _pumpHome(tester);
    final response = harness.adapter.queueProductResponse();
    await harness.remount();

    expect(
      find.byKey(const ValueKey('home-product-grid-loading')),
      findsOneWidget,
    );
    response.complete(
      _jsonResponse({
        'code': 0,
        'data': {
          'records': <Object>[],
          'page': '1',
          'pageSize': '6',
          'total': '0',
        },
        'message': 'success',
      }),
    );
    await _pumpUntil(
      tester,
      find.byKey(const ValueKey('home-products-empty-state')),
    );

    expect(
      find.byKey(const ValueKey('home-product-grid-loading')),
      findsNothing,
    );
    expect(find.text('暫無推薦'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('home-products-initial-error')),
      findsNothing,
    );
    final state = harness.container.read(homeProductControllerProvider);
    expect(state.isInitialLoading, isFalse);
    expect(state.initialError, isNull);
    _expectFirstPageRequest(harness.adapter, expectedCount: 1);
  });

  testWidgets('home error leaves the skeleton and retry can recover', (
    tester,
  ) async {
    final harness = await _pumpHome(tester);
    final firstResponse = harness.adapter.queueProductResponse();
    await harness.remount();

    expect(
      find.byKey(const ValueKey('home-product-grid-loading')),
      findsOneWidget,
    );
    firstResponse.complete(
      _jsonResponse({
        'code': 5030,
        'data': {'records': <Object>[]},
        'message': '推薦服務暫時無法使用',
      }),
    );
    await _pumpUntil(
      tester,
      find.byKey(const ValueKey('home-products-initial-error')),
    );

    expect(
      find.byKey(const ValueKey('home-product-grid-loading')),
      findsNothing,
    );
    expect(find.text('推薦服務暫時無法使用'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('home-products-empty-state')),
      findsNothing,
    );

    final retryResponse = harness.adapter.queueProductResponse();
    await tester.tap(find.byKey(const ValueKey('home-products-retry-initial')));
    await tester.pump();
    expect(
      find.byKey(const ValueKey('home-product-grid-loading')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('home-products-initial-error')),
      findsNothing,
    );

    retryResponse.complete(
      _jsonResponse({
        'code': 0,
        'data': {
          'records': [
            {
              'id': 'product-retry',
              'name': '重試成功商品',
              'imageUrl': '',
              'priceMin': '88',
              'sellerName': '測試商家',
            },
          ],
          'page': '1',
          'pageSize': '6',
          'total': '1',
        },
        'message': 'success',
      }),
    );
    await _pumpUntil(
      tester,
      find.byKey(const ValueKey('home-product-product-retry')),
    );

    expect(
      find.byKey(const ValueKey('home-product-grid-loading')),
      findsNothing,
    );
    expect(find.text('重試成功商品'), findsOneWidget);
    _expectFirstPageRequest(harness.adapter, expectedCount: 2);
  });
}

Future<_HomeHarness> _pumpHome(WidgetTester tester) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);

  final adapter = _ControlledHomeAdapter();
  final dio = Dio(BaseOptions(baseUrl: 'https://hermetic.invalid'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);
  final container = ProviderContainer(
    overrides: [
      dioProvider.overrideWithValue(dio),
      connectHttpClientProvider.overrideWithValue(
        (_) => throw StateError('Hermetic home test has no live Connect'),
      ),
      connectTransportProvider.overrideWithValue(fakeContentTransport()),
      sessionProvider.overrideWith(_GuestSession.new),
    ],
  );
  addTearDown(container.dispose);
  final harness = _HomeHarness(
    tester: tester,
    adapter: adapter,
    container: container,
  );
  return harness;
}

class _HomeHarness {
  const _HomeHarness({
    required this.tester,
    required this.adapter,
    required this.container,
  });

  final WidgetTester tester;
  final _ControlledHomeAdapter adapter;
  final ProviderContainer container;

  Future<void> remount() {
    return HttpOverrides.runZoned(
      () async {
        await tester.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: ScreenUtilInit(
              designSize: const Size(390, 844),
              enableScaleWH: () => false,
              enableScaleText: () => false,
              builder: (_, __) => const MaterialApp(home: HomeScreen()),
            ),
          ),
        );
        await tester.pump();
      },
      createHttpClient: (_) {
        throw StateError('External network is disabled in hermetic home tests');
      },
    );
  }
}

Future<void> _pumpUntil(
  WidgetTester tester,
  Finder finder, {
  int maxFrames = 20,
}) async {
  for (var frame = 0; frame < maxFrames && finder.evaluate().isEmpty; frame++) {
    await tester.pump(const Duration(milliseconds: 25));
  }
  expect(finder, findsOneWidget);
  expect(tester.takeException(), isNull);
}

void _expectFirstPageRequest(
  _ControlledHomeAdapter adapter, {
  required int expectedCount,
}) {
  final productRequests = adapter.requests
      .where((request) => request.path == '/public/products')
      .toList(growable: false);
  expect(productRequests, hasLength(expectedCount));
  for (final request in productRequests) {
    expect(request.queryParameters, {'page': 1, 'page_size': 6});
  }
}

ResponseBody _jsonResponse(Object body) {
  return ResponseBody.fromString(
    jsonEncode(body),
    200,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );
}
