import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/product/presentation/search_screen.dart';
import 'package:hawkr_buyer/shared/widgets/category_top_bar.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  testWidgets(
    'search landing has distinct sections and compact honest actions',
    (tester) async {
      final semantics = tester.ensureSemantics();
      await _setViewport(tester, 320);
      final dio = _stubSearchDio();
      addTearDown(dio.close);

      await _pumpSearchScreen(tester, dio: dio, textScale: 1.4);

      final search = find.byType(CategorySearchField);
      var searchSemantics = tester.getSemantics(search).getSemanticsData();
      expect(tester.getSize(search).height, HkControl.minTap);
      expect(searchSemantics.label, '商品搜尋');
      expect(searchSemantics.hint, '輸入商品名稱或關鍵字');
      expect(searchSemantics.flagsCollection.isTextField, isTrue);

      expect(find.text('今日料理靈感'), findsNothing);
      expect(find.text('人氣商品'), findsOneWidget);
      expect(find.text('為你挑選的新鮮好物'), findsOneWidget);
      expect(find.text('推薦給您'), findsNothing);
      expect(find.text('加入購物車'), findsNothing);
      expect(find.text('查看'), findsNWidgets(2));
      expect(find.bySemanticsLabel('查看商品：高麗菜'), findsOneWidget);

      final firstCard = tester.getRect(
        find.byKey(const ValueKey('search-result-tile-product-1')),
      );
      final secondCard = tester.getRect(
        find.byKey(const ValueKey('search-result-tile-product-2')),
      );
      expect(secondCard.top, greaterThan(firstCard.bottom));
      expect(tester.takeException(), isNull);

      await tester.enterText(find.byType(TextField), '今晚煮高麗菜');
      await tester.pump();

      searchSemantics = tester.getSemantics(search).getSemanticsData();
      expect(searchSemantics.label, '商品搜尋');
      expect(searchSemantics.hint, '輸入商品名稱或關鍵字');
      expect(searchSemantics.value, '今晚煮高麗菜');
      expect(
        tester.getSize(find.bySemanticsLabel('清除搜尋')),
        const Size.square(HkControl.minTap),
      );
      expect(tester.takeException(), isNull);
      semantics.dispose();
    },
  );

  testWidgets('search recommendations switch to two columns on tablet', (
    tester,
  ) async {
    await _setViewport(tester, 768);
    final dio = _stubSearchDio();
    addTearDown(dio.close);

    await _pumpSearchScreen(tester, dio: dio);

    final firstCard = tester.getRect(
      find.byKey(const ValueKey('search-result-tile-product-1')),
    );
    final secondCard = tester.getRect(
      find.byKey(const ValueKey('search-result-tile-product-2')),
    );
    final content = tester.getRect(
      find.byKey(const ValueKey('search-home-content')),
    );

    expect(firstCard.top, closeTo(secondCard.top, 0.01));
    expect(secondCard.left, greaterThan(firstCard.right));
    expect(content.width, 720);
    expect(content.left, 24);
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pumpSearchScreen(
  WidgetTester tester, {
  required Dio dio,
  double textScale = 1,
}) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(textScale)),
            child: child!,
          ),
          home: const SearchScreen(),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
  await tester.pump(const Duration(milliseconds: 50));
}

Dio _stubSearchDio() {
  final dio = Dio(BaseOptions(baseUrl: 'https://search.test'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) {
        if (options.path == '/public/products') {
          handler.resolve(
            Response<Map<String, dynamic>>(
              requestOptions: options,
              statusCode: 200,
              data: {
                'data': {
                  'records': [
                    {
                      'id': 'product-1',
                      'name': '高麗菜',
                      'priceMin': 500,
                      'sellerName': '南畝王生技',
                      'imageUrl': '',
                    },
                    {
                      'id': 'product-2',
                      'name': '紅豆',
                      'priceMin': 195,
                      'sellerName': '南畝王生技中都',
                      'imageUrl': '',
                    },
                  ],
                },
              },
            ),
          );
          return;
        }
        handler.reject(
          DioException(
            requestOptions: options,
            type: DioExceptionType.badResponse,
            response: Response<void>(requestOptions: options, statusCode: 404),
          ),
        );
      },
    ),
  );
  return dio;
}

Future<void> _setViewport(WidgetTester tester, double width) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
}
