import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/home/data/home_product_model.dart';
import 'package:hawkr_buyer/features/home/presentation/home_widget/home_product_section.dart';
import 'package:hawkr_buyer/shared/widgets/product_card.dart';
import 'package:hawkr_buyer/shared/widgets/product_grid_spec.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  const cases = <({double width, int columns})>[
    (width: 320, columns: 1),
    (width: 360, columns: 1),
    (width: 390, columns: 2),
    (width: 680, columns: 2),
    (width: 768, columns: 3),
    (width: 1024, columns: 4),
  ];

  for (final entry in cases) {
    testWidgets('home grid uses ${entry.columns} columns at ${entry.width}px', (
      tester,
    ) async {
      await _setViewport(tester, entry.width);
      await tester.pumpWidget(
        ProviderScope(
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => const MaterialApp(
              home: Scaffold(
                body: CustomScrollView(
                  slivers: [HomeProductSection(isLoading: true, products: [])],
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(tester.takeException(), isNull);
      final grid = tester.widget<SliverGrid>(
        find.byKey(const ValueKey('home-product-grid-loading')),
      );
      final delegate =
          grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount;
      final spec = ProductGridSpec.forWidth(entry.width);
      expect(delegate.crossAxisCount, entry.columns);
      expect(delegate.mainAxisExtent, closeTo(spec.mainAxisExtent, 0.001));
    });
  }

  testWidgets('home grid reserves card height at 1.4x text scale', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: const TextScaler.linear(1.4)),
            child: child!,
          ),
          home: const Scaffold(
            body: CustomScrollView(
              slivers: [HomeProductSection(isLoading: true, products: [])],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(tester.takeException(), isNull);
    final grid = tester.widget<SliverGrid>(
      find.byKey(const ValueKey('home-product-grid-loading')),
    );
    final delegate =
        grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount;
    expect(
      delegate.mainAxisExtent,
      closeTo(
        ProductGridSpec.forWidth(390, textScale: 1.4).mainAxisExtent,
        0.001,
      ),
    );
  });

  for (final config in <({double width, int columns})>[
    (width: 320, columns: 1),
    (width: 390, columns: 2),
    (width: 768, columns: 3),
  ]) {
    testWidgets(
      'home grid uses the 16px product role under nonlinear scaling at '
      '${config.width.toInt()}px',
      (tester) async {
        await _setViewport(tester, config.width);
        await tester.pumpWidget(
          ProviderScope(
            child: MaterialApp(
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: const _ProductRoleNonlinearScaler()),
                child: child!,
              ),
              home: const Scaffold(
                body: CustomScrollView(
                  slivers: [HomeProductSection(isLoading: true, products: [])],
                ),
              ),
            ),
          ),
        );
        await tester.pump();

        expect(tester.takeException(), isNull);
        final grid = tester.widget<SliverGrid>(
          find.byKey(const ValueKey('home-product-grid-loading')),
        );
        final delegate =
            grid.gridDelegate as SliverGridDelegateWithFixedCrossAxisCount;
        final spec = ProductGridSpec.forWidth(config.width, textScale: 1.3);
        expect(delegate.crossAxisCount, config.columns);
        expect(delegate.mainAxisExtent, closeTo(spec.mainAxisExtent, 0.001));
      },
    );
  }

  testWidgets('section header uses compact typography and text-only action', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: CustomScrollView(
              slivers: [HomeProductSection(isLoading: true, products: [])],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    final title = tester.widget<Text>(find.text('商品推薦'));
    final seeMore = tester.widget<Text>(find.text('查看更多'));
    expect(title.style?.fontSize, 16);
    expect(seeMore.style?.fontSize, 14);
    expect(seeMore.style?.color, AppColors.brandPrimaryStrong);
    final seeMoreButton = tester.widget<TextButton>(
      find.byKey(const ValueKey('home-products-see-more')),
    );
    expect(seeMoreButton.style?.backgroundColor, isNull);
    expect(
      tester
          .getRect(find.byKey(const ValueKey('home-products-see-more')))
          .height,
      greaterThanOrEqualTo(HkControl.minTap),
    );
  });

  testWidgets('loading card keeps the same inset 4:3 media geometry', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    await tester.pumpWidget(
      const ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: CustomScrollView(
              slivers: [HomeProductSection(isLoading: true, products: [])],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    final mediaRect = tester.getRect(
      find.byKey(const ValueKey('home-product-skeleton-media')).first,
    );
    expect(mediaRect.width / mediaRect.height, closeTo(4 / 3, 0.001));
    expect(tester.takeException(), isNull);
  });

  testWidgets('see-more switches to the latest-sorted canonical market tab', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    final router = GoRouter(
      initialLocation: '/',
      routes: [
        GoRoute(
          path: '/',
          builder: (_, state) {
            final tab = state.uri.queryParameters['tab'];
            final sort = state.uri.queryParameters['sort'];
            if (tab != null || sort != null) {
              return Scaffold(body: Text('$tab|$sort'));
            }
            return const Scaffold(
              body: CustomScrollView(
                slivers: [HomeProductSection(isLoading: false, products: [])],
              ),
            );
          },
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pump();

    await tester.tap(find.byKey(const ValueKey('home-products-see-more')));
    await tester.pumpAndSettle();

    expect(find.text('1|LATEST'), findsOneWidget);
  });

  testWidgets('the product preview never renders more than six cards', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    final products = List.generate(500, _product);

    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: CustomScrollView(
              slivers: [
                HomeProductSection(isLoading: false, products: products),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    final grid = tester.widget<SliverGrid>(
      find.byKey(const ValueKey('home-product-grid')),
    );
    expect(grid.delegate.estimatedChildCount, HomeProductSection.previewLimit);
    expect(
      find.byType(ProductCard).evaluate().length,
      lessThanOrEqualTo(HomeProductSection.previewLimit),
    );
    expect(find.byKey(const ValueKey('home-product-499')), findsNothing);
    expect(find.text('已經到底了'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('initial error replaces the empty grid and retries explicitly', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    var retries = 0;

    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: CustomScrollView(
              slivers: [
                HomeProductSection(
                  isLoading: false,
                  products: const [],
                  initialErrorMessage: '商品載入失敗',
                  onRetryInitial: () => retries++,
                ),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(
      find.byKey(const ValueKey('home-products-initial-error')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('home-products-empty-state')),
      findsNothing,
    );
    await tester.tap(find.byKey(const ValueKey('home-products-retry-initial')));
    expect(retries, 1);
  });

  testWidgets('home product cards do not use a closing Hero flight', (
    tester,
  ) async {
    await _setViewport(tester, 390);
    await tester.pumpWidget(
      ProviderScope(
        child: MaterialApp(
          home: Scaffold(
            body: CustomScrollView(
              slivers: [
                HomeProductSection(isLoading: false, products: [_product(0)]),
              ],
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    final card = tester.widget<ProductCard>(
      find.byKey(const ValueKey('home-product-0')),
    );
    expect(card.heroTag, isNull);
  });
}

Future<void> _setViewport(WidgetTester tester, double width) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, width >= 600 ? 1024 : 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

HomeProduct _product(int index) => HomeProduct(
  id: '$index',
  name: '商品 $index',
  imageUrl: '',
  priceMin: '${100 + index}',
  priceMax: '',
  listPriceMin: '',
  listPriceMax: '',
  salePriceMin: '',
  salePriceMax: '',
  sellerName: '測試商家',
  sellerAvatarUrl: '',
  sellerAccountId: 'seller-1',
  hashtags: const [],
);

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
