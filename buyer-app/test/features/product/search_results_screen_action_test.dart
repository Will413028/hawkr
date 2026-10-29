import 'package:hawkr_buyer/core/analytics/analytics_events.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/analytics/analytics_providers.dart';
import 'package:hawkr_buyer/core/analytics/ga4_item.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/category/data/category_repository.dart';
import 'package:hawkr_buyer/features/category/data/product_item_model.dart';
import 'package:hawkr_buyer/features/product/presentation/search_results_screen.dart';
import 'package:hawkr_buyer/shared/widgets/category_top_bar.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockAnalyticsEvents extends Mock implements AnalyticsEvents {}

class _MockCategoryRepository extends Mock implements CategoryRepository {}

void main() {
  setUpAll(() {
    registerFallbackValue(const GA4Item(itemId: '0', itemName: 'test'));
  });

  testWidgets('search result exposes a working, stable add-to-cart action', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _setViewport(tester, const Size(320, 568));
    final requests = <RequestOptions>[];
    final dio = _searchDio(requests: requests);
    final router = _router();
    addTearDown(dio.close);
    addTearDown(router.dispose);

    await _pumpSearchResults(tester, dio: dio, router: router);

    final search = find.byType(CategorySearchField);
    final searchSemantics = tester.getSemantics(search).getSemanticsData();
    expect(tester.getSize(search).height, HkControl.minTap);
    expect(searchSemantics.label, '商品搜尋');
    expect(searchSemantics.hint, '輸入商品名稱或關鍵字');
    expect(searchSemantics.value, '高麗菜');
    expect(
      tester.getSize(find.bySemanticsLabel('清除搜尋')),
      const Size.square(HkControl.minTap),
    );

    final button = find.widgetWithText(ElevatedButton, '加入購物車');
    expect(button, findsOneWidget);
    expect(tester.getSize(button).height, 48);
    final label = tester.widget<Text>(
      find.descendant(of: button, matching: find.byType(Text)),
    );
    expect(label.maxLines, 1);
    expect(label.softWrap, isFalse);
    expect(label.overflow, TextOverflow.ellipsis);
    expect(find.byType(HkButtonLabel), findsOneWidget);
    expect(tester.takeException(), isNull);
    final searchRequest = requests.singleWhere(
      (request) => request.path == '/public/search/products',
    );
    expect(searchRequest.queryParameters['q'], '高麗菜');
    expect(searchRequest.queryParameters['page'], 1);
    expect(searchRequest.queryParameters['page_size'], 20);

    await tester.tap(button);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 350));

    expect(find.byType(BottomSheet), findsOneWidget);
    expect(find.text('台灣有機高山高麗菜'), findsWidgets);
    expect(
      requests.any(
        (request) => request.path == '/public/products/871936508549696815',
      ),
      isTrue,
    );
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('tapping search product opens its detail route', (tester) async {
    await _setViewport(tester, const Size(390, 844));
    final dio = _searchDio();
    final router = _router();
    addTearDown(dio.close);
    addTearDown(router.dispose);

    await _pumpSearchResults(tester, dio: dio, router: router);

    await tester.tap(
      find.bySemanticsLabel(r'查看商品：台灣有機高山高麗菜，價格：NT$ 168，商家：範例友善農場，庫存：12 件'),
    );
    await tester.pumpAndSettle();

    expect(find.text('商品詳情已開啟'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('duplicate empty product ids do not create duplicate Hero tags', (
    tester,
  ) async {
    await _setViewport(tester, const Size(390, 844));
    final dio = _searchDio();
    final categoryRepository = _MockCategoryRepository();
    when(
      () => categoryRepository.searchProducts(
        keyword: any(named: 'keyword'),
        page: any(named: 'page'),
        pageSize: any(named: 'pageSize'),
      ),
    ).thenAnswer(
      (_) async => const ProductListResult(
        records: [
          ProductItem(
            id: '',
            name: '無編號商品甲',
            priceMin: '100',
            priceMax: '100',
            sellerName: '測試商家',
            stockQty: 1,
          ),
          ProductItem(
            id: '',
            name: '無編號商品乙',
            priceMin: '200',
            priceMax: '200',
            sellerName: '測試商家',
            stockQty: 2,
          ),
        ],
        total: 2,
        page: 1,
        pageSize: 20,
      ),
    );
    final router = _router();
    addTearDown(dio.close);
    addTearDown(router.dispose);

    await _pumpSearchResults(
      tester,
      dio: dio,
      router: router,
      categoryRepository: categoryRepository,
    );

    expect(find.text('無編號商品甲'), findsOneWidget);
    expect(find.text('無編號商品乙'), findsOneWidget);
    expect(find.byType(Hero), findsNothing);

    await tester.tap(find.text('無編號商品甲'));
    await tester.pumpAndSettle();

    expect(find.text('商品詳情已開啟'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pumpSearchResults(
  WidgetTester tester, {
  required Dio dio,
  required GoRouter router,
  CategoryRepository? categoryRepository,
}) async {
  final analytics = _MockAnalyticsEvents();
  when(
    () => analytics.trackViewItemList(
      any(),
      listId: any(named: 'listId'),
      listName: any(named: 'listName'),
    ),
  ).thenAnswer((_) async {});
  when(
    () => analytics.trackSelectItem(
      any(),
      listId: any(named: 'listId'),
      listName: any(named: 'listName'),
    ),
  ).thenAnswer((_) async {});
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(dio),
        connectAuthTransportProvider.overrideWith(
          (_) => FakeTransportBuilder().build(),
        ),
        connectTransportProvider.overrideWith(
          (_) => FakeTransportBuilder().build(),
        ),
        if (categoryRepository != null)
          categoryRepositoryProvider.overrideWithValue(categoryRepository),
        analyticsEventsProvider.overrideWithValue(analytics),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp.router(
          routerConfig: router,
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: const TextScaler.linear(1.4)),
            child: child!,
          ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump(const Duration(milliseconds: 100));
}

GoRouter _router() => GoRouter(
  routes: [
    GoRoute(
      path: '/',
      builder: (_, __) => const SearchResultsScreen(query: '高麗菜'),
    ),
    GoRoute(
      path: '/merchant-product',
      builder: (_, __) => const Scaffold(body: Text('商品詳情已開啟')),
    ),
  ],
);

Dio _searchDio({List<RequestOptions>? requests}) {
  final dio = Dio(BaseOptions(baseUrl: 'https://search-results.test'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        requests?.add(options);
        if (options.path == '/public/search/products') {
          handler.resolve(
            Response<Map<String, dynamic>>(
              requestOptions: options,
              statusCode: 200,
              data: {
                'code': 0,
                'data': {
                  'records': [
                    {
                      // DEV /public/search/products ProductHitData contract:
                      // int64 identifiers and numeric values cross as strings.
                      'id': '871936508549696815',
                      'name': '台灣有機高山高麗菜',
                      'categoryId': '7',
                      'categoryName': '蔬菜',
                      'priceMin': '168',
                      'priceMax': '168',
                      'imageKey': '',
                      'imageUrl': '',
                      'storeId': '7489859548420286732',
                      'sellerName': '範例友善農場',
                      'stock': '12',
                      'isRestockSubscribed': false,
                    },
                  ],
                  'total': '1',
                  'page': '1',
                  'pageSize': '20',
                  'totalPages': '1',
                },
              },
            ),
          );
          return;
        }
        if (options.path == '/public/products/871936508549696815') {
          handler.resolve(
            Response<Map<String, dynamic>>(
              requestOptions: options,
              statusCode: 200,
              data: {
                'code': 0,
                'data': {
                  'skus': [
                    {'id': 4201, 'name': '一顆', 'price': 168, 'stock': 12},
                  ],
                  'shippingMethods': [
                    {'type': 'HOME_DELIVERY'},
                  ],
                },
              },
            ),
          );
          return;
        }
        handler.resolve(
          Response<Map<String, dynamic>>(
            requestOptions: options,
            statusCode: 200,
            data: {'code': 0, 'data': 0},
          ),
        );
      },
    ),
  );
  return dio;
}

Future<void> _setViewport(WidgetTester tester, Size size) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
}
