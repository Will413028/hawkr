import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/features/category/data/category_repository.dart';
import 'package:hawkr_buyer/features/category/presentation/category_landing_screen.dart';
import 'package:dio/dio.dart';
import '../../support/fake_discovery_client.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _SequenceAdapter implements HttpClientAdapter {
  _SequenceAdapter(this.responses, {this.gate});

  final List<Object> responses;
  final Completer<void>? gate;
  final List<RequestOptions> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    await gate?.future;
    final index = requests.length <= responses.length
        ? requests.length - 1
        : responses.length - 1;
    final response = responses[index];
    final failed = response is Map && response['failStatus'] != null;
    return ResponseBody.fromString(
      jsonEncode(response),
      failed ? response['failStatus'] as int : 200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  testWidgets('shows loading before the single request completes', (
    tester,
  ) async {
    final gate = Completer<void>();
    final adapter = _SequenceAdapter([
      {
        'data': {'records': <Object>[]},
      },
    ], gate: gate);
    final dio = _dioWith(adapter);
    addTearDown(dio.close);

    await _pumpSection(tester, dio);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 1));

    expect(
      find.byKey(const ValueKey('category-marketplace-loading')),
      findsOneWidget,
    );
    expect(adapter.requests, hasLength(1));
    expect(adapter.requests.single.path, '/public/products');
    expect(adapter.requests.single.queryParameters['sort'], 'SALES');

    gate.complete();
    await tester.pumpAndSettle();
    expect(
      find.byKey(const ValueKey('category-marketplace-empty')),
      findsOneWidget,
    );
  });

  testWidgets('shows an honest empty state for an empty product list', (
    tester,
  ) async {
    final adapter = _SequenceAdapter([
      {
        'data': {'records': <Object>[]},
      },
    ]);
    final dio = _dioWith(adapter);
    addTearDown(dio.close);

    await _pumpSection(tester, dio);
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('category-marketplace-empty')),
      findsOneWidget,
    );
    expect(find.text('暫無推薦'), findsOneWidget);
    expect(adapter.requests, hasLength(1));
  });

  testWidgets(
    'renders valid product cards and ignores records without a name',
    (tester) async {
      final adapter = _SequenceAdapter([
        {
          'data': {
            'records': [
              {'id': '', 'name': '無效商品'},
              {
                'id': 'product-1',
                'name': '高麗菜',
                'imageUrl': '',
                'accountId': 'seller-1',
                'sellerName': '南畝王生技',
                'priceMin': '500',
              },
            ],
          },
        },
      ]);
      final dio = _dioWith(adapter);
      addTearDown(dio.close);

      await _pumpSection(tester, dio);
      await tester.pumpAndSettle();

      expect(
        find.byKey(const ValueKey('category-marketplace-products')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('category-marketplace-product-product-1')),
        findsOneWidget,
      );
      expect(find.text('高麗菜'), findsOneWidget);
      expect(find.text('NT\$ 500 / 斤'), findsOneWidget);
      expect(find.text('無效商品'), findsNothing);
      expect(adapter.requests, hasLength(1));
    },
  );

  testWidgets('a failed request shows retry and refreshes from network', (
    tester,
  ) async {
    final adapter = _SequenceAdapter([
      {'failStatus': 500},
      {
        'data': {
          'records': [
            {'id': 'recovered', 'name': '重試成功', 'priceMin': '120'},
          ],
        },
      },
    ]);
    final dio = _dioWith(adapter);
    addTearDown(dio.close);

    await _pumpSection(tester, dio);
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('category-marketplace-error')),
      findsOneWidget,
    );
    expect(find.text('推薦商品載入失敗'), findsOneWidget);
    expect(adapter.requests, hasLength(1));

    await tester.tap(find.byKey(const ValueKey('category-marketplace-retry')));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.text('重試成功'), findsOneWidget);
    expect(adapter.requests, hasLength(2));
    expect(adapter.requests.map((request) => request.path).toSet(), {
      '/public/products',
    });
  });
}

Dio _dioWith(HttpClientAdapter adapter) =>
    Dio(BaseOptions(baseUrl: 'https://test.local'))
      ..httpClientAdapter = adapter;

Future<void> _pumpSection(WidgetTester tester, Dio dio) {
  return tester.pumpWidget(
    ProviderScope(
      overrides: [
        categoryRepositoryProvider.overrideWithValue(
          CategoryRepository(dio, fakeDiscoveryClient()),
        ),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => const MaterialApp(
          home: Scaffold(body: CategoryMarketplaceProductSection()),
        ),
      ),
    ),
  );
}
