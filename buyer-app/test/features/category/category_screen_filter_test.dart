import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/core/analytics/analytics_events.dart';
import 'package:hawkr_buyer/core/analytics/analytics_providers.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/category/presentation/category_screen.dart';
import 'package:hawkr_buyer/features/visit/data/visit_repository.dart';
import 'package:hawkr_buyer/shared/widgets/buyer_floating_navigation_layout.dart';
import 'package:hawkr_buyer/shared/widgets/category_top_bar.dart';
import 'package:hawkr_buyer/shared/widgets/hk_choice_chip.dart';
import 'package:hawkr_buyer/shared/widgets/hk_liquid_glass.dart';
import 'package:hawkr_buyer/shared/widgets/product_card.dart';
import 'package:hawkr_buyer/shared/widgets/product_grid_spec.dart';
import 'package:hawkr_buyer/shared/widgets/shared_bottom_nav.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockAnalyticsEvents extends Mock implements AnalyticsEvents {}

class _SignedInSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Test Buyer');
}

/// 批次檢查停在 [pendingCheck]，讓測試在請求進行中操作收藏。
class _PendingFavoritesRepository extends Fake implements VisitRepository {
  final pendingCheck = Completer<Set<int>>();
  final writes = <String>[];
  var checks = 0;

  @override
  Future<Set<int>> batchCheckFavorites(Iterable<int> productIds) {
    checks++;
    return pendingCheck.future;
  }

  @override
  Future<void> addFavorite(int productId) async => writes.add('+$productId');

  @override
  Future<void> removeFavorite(int productId) async => writes.add('-$productId');
}

typedef _RecordedRequest = ({String path, Map<String, dynamic> query});

class _MarketAdapter implements HttpClientAdapter {
  _MarketAdapter({this.withProducts = false});

  final bool withProducts;
  final List<_RecordedRequest> requests = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add((path: options.path, query: Map.of(options.queryParameters)));
    final body = switch (options.path) {
      '/public/categories/tree' => {
        'data': [
          {
            'id': '100',
            'name': '海鮮',
            'children': [
              {
                'id': '101',
                'name': '魚類',
                'children': [
                  {'id': '103', 'name': '虱目魚'},
                ],
              },
              {'id': '102', 'name': '甲殼類'},
            ],
          },
          {'id': '200', 'name': '蔬菜', 'children': <Object>[]},
        ],
      },
      '/public/hashtags/popular' => {
        'data': {
          'records': [
            {'hashtag': '冷凍水產', 'count': '8'},
            {'hashtag': '#當季', 'count': '5'},
          ],
        },
      },
      '/public/marketplace' => {
        'data': {
          'recommendedSellers': [
            {
              'storeId': '9001',
              'storeName': '金湖水產',
              'logoKey': '',
              'logoUrl': '',
              'city': '高雄市',
              'district': '前鎮區',
            },
          ],
        },
      },
      _ => {
        'data': {
          'records': withProducts
              ? List.generate(
                  6,
                  (index) => {
                    'id': '${index + 1}',
                    'name': '測試商品 ${index + 1}',
                    'priceMin': '${100 + index}',
                    'priceMax': '${120 + index}',
                    'listPriceMin': '${150 + index}',
                    'listPriceMax': '${170 + index}',
                    'imageUrl': '',
                    'storeId': '9001',
                    'sellerName': '測試商家',
                    'seller': {
                      'storeId': '9001',
                      'name': '高雄小農',
                      'city': '高雄市',
                      'district': '美濃區',
                    },
                    'hashtags': [
                      '冷凍',
                      {'name': '海鮮'},
                    ],
                  },
                )
              : <Object>[],
          'total': withProducts ? 6 : 7,
          'page': 1,
          'pageSize': 20,
        },
      },
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

  _RecordedRequest get lastProductsRequest =>
      requests.lastWhere((request) => request.path == '/public/products');
}

void main() {
  testWidgets(
    'a favorite check still in flight does not undo the buyer\'s local change',
    (tester) async {
      final favorites = _PendingFavoritesRepository();
      await _pumpMarket(
        tester,
        width: 390,
        withProducts: true,
        signedInFavorites: favorites,
      );
      ProductCard card(String id) => tester.widget<ProductCard>(
        find.byKey(ValueKey('market-product-$id')),
      );

      // 檢查還沒回來：使用者先收藏再取消收藏商品 1。
      expect(favorites.checks, 1);
      card('1').onFavoriteTap!();
      await tester.pump();
      await tester.pump();
      expect(card('1').isFavorited, isTrue);
      card('1').onFavoriteTap!();
      await tester.pump();
      await tester.pump();
      expect(card('1').isFavorited, isFalse);
      expect(favorites.writes, ['+1', '-1']);

      // 回應是在使用者取消之前讀到的伺服器狀態：1、2 都有收藏。
      favorites.pendingCheck.complete({1, 2});
      await tester.pump();
      await tester.pump();

      expect(card('1').isFavorited, isFalse);
      expect(card('2').isFavorited, isTrue);
      expect(tester.takeException(), isNull);
    },
  );

  for (final width in <double>[320, 390, 768]) {
    testWidgets('market filters have no overflow at ${width.toInt()}px', (
      tester,
    ) async {
      final adapter = await _pumpMarket(tester, width: width);

      expect(tester.takeException(), isNull);
      expect(find.byType(CategorySearchField), findsOneWidget);
      expect(
        find.byKey(const ValueKey('market-seller-stories')),
        findsOneWidget,
      );
      expect(find.text('推薦店家'), findsNothing);
      final sellerRing = tester.widget<Container>(
        find.byKey(const ValueKey('market-seller-ring-9001')),
      );
      expect(
        tester.getSize(find.byKey(const ValueKey('market-seller-ring-9001'))),
        const Size.square(68),
      );
      final sellerRingDecoration = sellerRing.decoration! as BoxDecoration;
      expect(
        sellerRingDecoration.color,
        AppColors.neutralWhite.withValues(alpha: 0.72),
      );
      expect(
        sellerRingDecoration.border!.top.color,
        AppColors.neutralWhite.withValues(alpha: 0.88),
      );
      for (final label in ['推薦', '追蹤中', '附近']) {
        expect(find.text(label), findsNothing);
      }
      expect(find.byKey(const ValueKey('market-story-tabs')), findsNothing);
      expect(
        find.byKey(const ValueKey('market-results-toolbar')),
        findsOneWidget,
      );
      expect(find.byKey(const ValueKey('market-filter-panel')), findsNothing);
      expect(find.byKey(const ValueKey('market-sort-options')), findsNothing);
      expect(find.text('7 件商品'), findsOneWidget);
      final searchTop = tester.getTopLeft(find.byType(CategorySearchField)).dy;
      final toolbarTop = tester
          .getTopLeft(find.byKey(const ValueKey('market-results-toolbar')))
          .dy;
      final sellersTop = tester
          .getTopLeft(find.byKey(const ValueKey('market-seller-stories')))
          .dy;
      expect(searchTop, lessThan(toolbarTop));
      expect(toolbarTop, lessThan(sellersTop));
      expect(adapter.lastProductsRequest.query, {'page': 1, 'page_size': 20});

      await tester.tap(find.byKey(const ValueKey('market-filter-button')));
      await tester.pumpAndSettle();
      expect(find.byKey(const ValueKey('market-filter-panel')), findsOneWidget);
      expect(
        tester.widget<BottomSheet>(find.byType(BottomSheet)).showDragHandle,
        isFalse,
      );
      _expectRectInside(
        tester.getRect(
          find.byKey(const ValueKey('market-filter-sheet-surface')),
        ),
        tester.getRect(_dragBarFinder('market-filter-drag-handle')),
      );
      expect(
        find.byKey(const ValueKey('market-category-picker')),
        findsOneWidget,
      );
      await tester.ensureVisible(
        find.byKey(const ValueKey('market-apply-price')),
      );
      await tester.pump();
      expect(tester.takeException(), isNull);
      expect(find.byKey(const ValueKey('market-temp-3201')), findsNothing);
      expect(find.byKey(const ValueKey('market-cert-3702')), findsNothing);

      await _tapVisible(
        tester,
        find.byKey(const ValueKey('market-category-picker')),
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('market-category-sheet')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('market-category-drag-handle')),
        findsOneWidget,
      );
      expect(tester.takeException(), isNull);
      await tester.tap(find.byKey(const ValueKey('market-category-done')));
      await tester.pumpAndSettle();
    });
  }

  testWidgets('market search stays accessible at 320px and 1.4x text', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _pumpMarket(tester, width: 320, textScale: 1.4);

    final search = find.byType(CategorySearchField);
    var searchSemantics = tester.getSemantics(search).getSemanticsData();
    expect(tester.getSize(search).height, HkControl.minTap);
    expect(searchSemantics.label, '市集商品搜尋');
    expect(searchSemantics.hint, '輸入商品名稱或關鍵字');
    expect(searchSemantics.flagsCollection.isTextField, isTrue);

    final field = find.descendant(of: search, matching: find.byType(TextField));
    expect(tester.widget<TextField>(field).decoration?.hintMaxLines, 1);
    await tester.enterText(field, '虱目魚');
    await tester.pump(const Duration(milliseconds: 400));

    searchSemantics = tester.getSemantics(search).getSemanticsData();
    expect(searchSemantics.label, '市集商品搜尋');
    expect(searchSemantics.hint, '輸入商品名稱或關鍵字');
    expect(searchSemantics.value, '虱目魚');
    expect(
      tester.getSize(find.bySemanticsLabel('清除搜尋')),
      const Size.square(HkControl.minTap),
    );
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('route intent is applied to the first product-list request', (
    tester,
  ) async {
    final adapter = await _pumpMarket(
      tester,
      width: 390,
      initialKeyword: ' 虱目魚 ',
      initialSort: '最新上架',
    );

    final searchField = tester.widget<TextField>(
      find.descendant(
        of: find.byType(CategorySearchField),
        matching: find.byType(TextField),
      ),
    );
    expect(searchField.controller!.text, '虱目魚');

    final productListRequests = adapter.requests
        .where(
          (request) =>
              request.path == '/public/products' &&
              request.query['page_size'] == 20,
        )
        .toList(growable: false);
    expect(productListRequests, hasLength(1));
    expect(productListRequests.single.query['keyword'], '虱目魚');
    expect(productListRequests.single.query['sort'], 'LATEST');
  });

  testWidgets('browse scope lives inside the design-system sort sheet', (
    tester,
  ) async {
    await _pumpMarket(tester, width: 390);

    expect(find.byKey(const ValueKey('market-story-tabs')), findsNothing);
    await tester.tap(find.byKey(const ValueKey('market-sort-button')));
    await tester.pumpAndSettle();

    expect(find.text('商品瀏覽與排序'), findsOneWidget);
    expect(find.text('瀏覽範圍'), findsOneWidget);
    expect(find.text('排序方式'), findsOneWidget);
    expect(find.text('綜合推薦'), findsOneWidget);
    for (final tab in ['recommended', 'following', 'nearby']) {
      final option = find.byKey(ValueKey('market-browse-$tab'));
      expect(option, findsOneWidget);
      expect(tester.getSize(option).height, greaterThanOrEqualTo(48));
    }
    expect(
      tester
          .widget<HkChoiceChip>(
            find.byKey(const ValueKey('market-browse-recommended')),
          )
          .selected,
      isTrue,
    );
    expect(
      tester.getSize(find.byKey(const ValueKey('market-sort-sheet'))).height,
      lessThanOrEqualTo(600),
    );

    await tester.tap(find.byKey(const ValueKey('market-browse-nearby')));
    await tester.pumpAndSettle();
    expect(find.byKey(const ValueKey('market-sort-sheet')), findsNothing);
    expect(find.text('排序 1'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('market-sort-button')));
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<HkChoiceChip>(
            find.byKey(const ValueKey('market-browse-nearby')),
          )
          .selected,
      isTrue,
    );
  });

  for (final config in <({double width, int columns, double textScale})>[
    (width: 320, columns: 1, textScale: 1),
    (width: 390, columns: 2, textScale: 1),
    (width: 390, columns: 2, textScale: 1.4),
    (width: 768, columns: 3, textScale: 1),
  ]) {
    testWidgets(
      'product results use the shared ${config.columns}-column Web card grid '
      'at ${config.width.toInt()}px / ${config.textScale}x',
      (tester) async {
        await _pumpMarket(
          tester,
          width: config.width,
          withProducts: true,
          textScale: config.textScale,
        );

        expect(tester.takeException(), isNull);
        final grid = tester.widget<SliverGrid>(
          find.byKey(const ValueKey('market-product-grid')),
        );
        final delegate =
            grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount;
        final spec = ProductGridSpec.forWidth(
          config.width,
          textScale: config.textScale,
        );
        expect(delegate.crossAxisCount, config.columns);
        expect(delegate.mainAxisExtent, closeTo(spec.mainAxisExtent, 0.001));
        expect(find.byType(ProductCard), findsWidgets);
        final visibleCards = tester
            .widgetList<ProductCard>(find.byType(ProductCard))
            .toList(growable: false);
        for (final card in visibleCards) {
          expect(
            card.favoriteSurfaceStyle,
            ProductFavoriteSurfaceStyle.hashtag,
          );
        }
        final favoriteSurfaces = find.byKey(
          const ValueKey('product-card-favorite-surface'),
        );
        expect(favoriteSurfaces, findsNWidgets(visibleCards.length));
        for (final surface in favoriteSurfaces.evaluate()) {
          final surfaceFinder = find.byWidget(surface.widget);
          expect(tester.getSize(surfaceFinder), const Size.square(33));
          final glass = surface.widget as HkLiquidGlassSurface;
          expect(glass.variant, HkLiquidGlassVariant.clear);
          expect(glass.surfaceOpacity, HkGlass.regularSurfaceOpacity);
          expect(glass.paintSurfaceColorBeneathGradient, isTrue);
          expect(glass.shadows, isNotEmpty);
          final blur = tester.widget<BackdropFilter>(
            find.descendant(
              of: surfaceFinder,
              matching: find.byType(BackdropFilter),
            ),
          );
          expect(blur.enabled, isTrue);
          expect(blur.filterConfig, isNotNull);
        }
        expect(
          tester
              .getTopLeft(find.byKey(const ValueKey('market-seller-stories')))
              .dy,
          lessThan(tester.getTopLeft(find.byType(ProductCard).first).dy),
        );
        expect(find.text('#冷凍'), findsWidgets);
        expect(find.text('#海鮮'), findsWidgets);
      },
    );
  }

  for (final config in <({double width, int columns})>[
    (width: 320, columns: 1),
    (width: 390, columns: 2),
    (width: 768, columns: 3),
  ]) {
    testWidgets(
      'category grid uses the 16px product role under nonlinear scaling at '
      '${config.width.toInt()}px',
      (tester) async {
        await _pumpMarket(
          tester,
          width: config.width,
          withProducts: true,
          textScaler: const _ProductRoleNonlinearScaler(),
        );

        expect(tester.takeException(), isNull);
        final grid = tester.widget<SliverGrid>(
          find.byKey(const ValueKey('market-product-grid')),
        );
        final delegate =
            grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount;
        final spec = ProductGridSpec.forWidth(config.width, textScale: 1.3);
        expect(delegate.crossAxisCount, config.columns);
        expect(delegate.mainAxisExtent, closeTo(spec.mainAxisExtent, 0.001));
      },
    );
  }

  testWidgets('search/category/price/sort use the backend contract', (
    tester,
  ) async {
    final adapter = await _pumpMarket(tester, width: 390);

    final searchField = find.descendant(
      of: find.byType(CategorySearchField),
      matching: find.byType(TextField),
    );
    await tester.enterText(searchField, '虱目魚');
    await tester.pump(const Duration(milliseconds: 400));
    expect(adapter.lastProductsRequest.query['keyword'], '虱目魚');

    await tester.tap(find.byKey(const ValueKey('market-sort-button')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('market-sort-price_asc')));
    await tester.pumpAndSettle();
    expect(adapter.lastProductsRequest.query['sort'], 'PRICE_ASC');

    await tester.tap(find.byKey(const ValueKey('market-filter-button')));
    await tester.pumpAndSettle();
    await _tapVisible(
      tester,
      find.byKey(const ValueKey('market-category-picker')),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('market-category-l2-101')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('market-category-l2-102')));
    await tester.pump();
    await tester.tap(find.byKey(const ValueKey('market-category-done')));
    await tester.pumpAndSettle();
    expect(adapter.lastProductsRequest.query['category_id'], '101,102');

    final minField = find.descendant(
      of: find.byKey(const ValueKey('market-price-min')),
      matching: find.byType(TextField),
    );
    final maxField = find.descendant(
      of: find.byKey(const ValueKey('market-price-max')),
      matching: find.byType(TextField),
    );
    await tester.ensureVisible(maxField);
    await tester.pump();
    await tester.enterText(minField, '100');
    await tester.enterText(maxField, '500');
    await _tapVisible(tester, find.byKey(const ValueKey('market-apply-price')));
    await tester.pump(const Duration(milliseconds: 50));
    expect(adapter.lastProductsRequest.query['price_min'], '100');
    expect(adapter.lastProductsRequest.query['price_max'], '500');

    await _tapVisible(
      tester,
      find.byKey(const ValueKey('market-hashtag-冷凍水產')),
    );
    await tester.pump(const Duration(milliseconds: 50));
    expect(adapter.lastProductsRequest.query['hashtags'], '冷凍水產');

    final clearAll = find.byKey(const ValueKey('market-clear-all-filters'));
    await _tapVisible(tester, clearAll);
    await tester.pump(const Duration(milliseconds: 50));
    final cleared = adapter.lastProductsRequest.query;
    expect(
      cleared['sort'],
      'PRICE_ASC',
      reason: 'Web clear-all preserves sort',
    );
    for (final key in [
      'keyword',
      'category_id',
      'delivery_temp',
      'price_min',
      'price_max',
      'hashtags',
      'certification_types',
    ]) {
      expect(cleared, isNot(contains(key)));
    }
  });

  testWidgets('invalid price range is blocked before the API call', (
    tester,
  ) async {
    final adapter = await _pumpMarket(tester, width: 320);
    await tester.tap(find.byKey(const ValueKey('market-filter-button')));
    await tester.pumpAndSettle();

    final minField = find.descendant(
      of: find.byKey(const ValueKey('market-price-min')),
      matching: find.byType(TextField),
    );
    final maxField = find.descendant(
      of: find.byKey(const ValueKey('market-price-max')),
      matching: find.byType(TextField),
    );
    await tester.ensureVisible(maxField);
    await tester.pump();
    await tester.enterText(minField, '500');
    await tester.enterText(maxField, '100');
    final before = adapter.requests.length;
    await _tapVisible(tester, find.byKey(const ValueKey('market-apply-price')));
    await tester.pump();

    expect(find.text('最高價不可低於最低價'), findsOneWidget);
    expect(adapter.requests.length, before);
  });

  testWidgets('filter sheet does not offer an origin picker', (tester) async {
    await _pumpMarket(tester, width: 390, withProducts: true);
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const ValueKey('market-filter-button')));
    await tester.pumpAndSettle();

    expect(find.byKey(const ValueKey('market-origin-picker')), findsNothing);
    expect(find.text('產地'), findsNothing);
  });

  testWidgets(
    'market uses a full-height scroller with in-content Navbar clearance',
    (tester) async {
      await _pumpMarket(
        tester,
        width: 390,
        withProducts: true,
        withFloatingNavigation: true,
        viewPaddingBottom: 34,
      );

      final scrollable = find.byKey(const ValueKey('market-scroll-view'));
      expect(tester.getSize(scrollable), const Size(390, 844));

      await tester.pump(const Duration(milliseconds: 901));
      await tester.timedDrag(
        scrollable,
        const Offset(0, -220),
        const Duration(milliseconds: 320),
      );
      await tester.pump();

      var slide = tester.widget<AnimatedSlide>(
        find.byKey(const ValueKey('buyer-bottom-nav-auto-hide-slide')),
      );
      expect(slide.offset.dy, greaterThan(1));

      await tester.drag(scrollable, const Offset(0, 100));
      await tester.pump();
      slide = tester.widget<AnimatedSlide>(
        find.byKey(const ValueKey('buyer-bottom-nav-auto-hide-slide')),
      );
      expect(slide.offset, Offset.zero);

      final marketView = tester.widget<CustomScrollView>(scrollable);
      marketView.controller!.jumpTo(
        marketView.controller!.position.maxScrollExtent,
      );
      await tester.pumpAndSettle();
      expect(
        find.byKey(const ValueKey('market-navigation-clearance')),
        findsOneWidget,
      );
      final footer = tester.getRect(find.text('已顯示全部商品'));
      final navigation = tester.getRect(
        find.byKey(const ValueKey('buyer-bottom-nav-surface')),
      );
      expect(
        footer.bottom,
        lessThanOrEqualTo(navigation.top - HkSpace.md),
        reason: '市集最後一筆內容必須完整停在懸浮 Navbar 上方',
      );
      expect(tester.takeException(), isNull);
    },
  );
}

Future<void> _tapVisible(WidgetTester tester, Finder finder) async {
  await tester.ensureVisible(finder);
  await tester.pump();
  await tester.tap(finder);
  await tester.pump();
}

void _expectRectInside(Rect outer, Rect inner) {
  expect(inner.left, greaterThanOrEqualTo(outer.left));
  expect(inner.top, greaterThanOrEqualTo(outer.top));
  expect(inner.right, lessThanOrEqualTo(outer.right));
  expect(inner.bottom, lessThanOrEqualTo(outer.bottom));
}

Finder _dragBarFinder(String key) => find.descendant(
  of: find.byKey(ValueKey(key)),
  matching: find.byType(DecoratedBox),
);

Future<_MarketAdapter> _pumpMarket(
  WidgetTester tester, {
  required double width,
  String initialKeyword = '',
  String initialSort = '',
  bool withProducts = false,
  bool withFloatingNavigation = false,
  double viewPaddingBottom = 0,
  double textScale = 1,
  TextScaler? textScaler,
  VisitRepository? signedInFavorites,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, width >= 600 ? 1024 : 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final adapter = _MarketAdapter(withProducts: withProducts);
  final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
    ..httpClientAdapter = adapter;
  addTearDown(() => dio.close());
  final analytics = _MockAnalyticsEvents();
  when(
    () => analytics.trackViewItemList(
      any(),
      listId: any(named: 'listId'),
      listName: any(named: 'listName'),
    ),
  ).thenAnswer((_) async {});
  final container = ProviderContainer(
    overrides: [
      dioProvider.overrideWithValue(dio),
      analyticsEventsProvider.overrideWithValue(analytics),
      if (signedInFavorites != null) ...[
        sessionProvider.overrideWith(_SignedInSession.new),
        visitRepositoryProvider.overrideWithValue(signedInFavorites),
      ],
    ],
  );
  addTearDown(container.dispose);
  // 已登入的 App 在進市集前 session 早已解析完成。
  if (signedInFavorites != null) await container.read(sessionProvider.future);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(context).copyWith(
              padding: EdgeInsets.only(bottom: viewPaddingBottom),
              viewPadding: EdgeInsets.only(bottom: viewPaddingBottom),
              textScaler: textScaler ?? TextScaler.linear(textScale),
            ),
            child: child!,
          ),
          home: withFloatingNavigation
              ? BuyerFloatingNavigationLayout(
                  navigationVisible: true,
                  visibilityResetKey: 'market',
                  page: CategoryScreen(
                    initialKeyword: initialKeyword,
                    initialSort: initialSort,
                  ),
                  navigation: SharedBottomNav(
                    currentIndex: 1,
                    onDestinationRequested: (_) async => true,
                  ),
                )
              : CategoryScreen(
                  initialKeyword: initialKeyword,
                  initialSort: initialSort,
                ),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 50));
  await tester.pump(const Duration(milliseconds: 50));
  return adapter;
}

final class _ProductRoleNonlinearScaler extends TextScaler {
  const _ProductRoleNonlinearScaler();

  @override
  double scale(double fontSize) {
    if (fontSize < 12) return fontSize;
    if (fontSize < 14) return fontSize * 1.4;
    if (fontSize < 16) return fontSize * 1.35;
    return fontSize * 1.3;
  }

  @override
  double get textScaleFactor => 1;
}
