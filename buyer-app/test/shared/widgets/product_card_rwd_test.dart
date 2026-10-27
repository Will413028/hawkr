import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:ui' show Tristate;

import 'package:hawkr_buyer/shared/widgets/product_card.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_liquid_glass.dart';
import 'package:hawkr_buyer/shared/widgets/product_grid_spec.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final config in <({double viewport, double width})>[
    (viewport: 320, width: 140),
    (viewport: 390, width: 165),
    (viewport: 768, width: 240),
    (viewport: 1024, width: 320),
  ]) {
    for (final textScale in <double>[1, 1.15, 1.4]) {
      testWidgets('card has no overflow at ${config.viewport.toInt()}px and '
          '${textScale}x text', (tester) async {
        await _pumpCard(
          tester,
          viewportWidth: config.viewport,
          cardWidth: config.width,
          cardHeight: config.width * 3 / 4 + 72 + ((textScale - 1) * 20),
          textScale: textScale,
          name: '澎湖現流急凍小花枝切片家庭號',
          price: '299',
          priceMax: '350',
          listPriceMin: '350',
          listPriceMax: '499',
          hashtags: const ['冷凍水產', '花枝'],
        );

        expect(tester.takeException(), isNull);
        expect(find.text('#冷凍水產'), findsOneWidget);
        expect(find.text('#花枝'), findsOneWidget);

        final imageRect = tester.getRect(
          find.byKey(const ValueKey('product-card-image')),
        );
        final layerRect = tester.getRect(
          find.byKey(const ValueKey('product-card-hashtag-layer')),
        );
        final actionRect = tester.getRect(
          find.byKey(const ValueKey('product-card-add-action')),
        );
        final addSurfaceRect = tester.getRect(
          find.byKey(const ValueKey('product-card-add-surface')),
        );
        final surfaceRect = tester.getRect(
          find.byKey(const ValueKey('product-card-surface')),
        );
        final sellerRect = tester.getRect(
          find.byKey(const ValueKey('product-card-seller-row')),
        );

        // The add action remains part of the single card surface and is
        // centered precisely on the media/body seam.
        expect(actionRect.center.dy, closeTo(imageRect.bottom, 0.1));
        _expectRectInside(surfaceRect, addSurfaceRect);
        expect(sellerRect.right, lessThanOrEqualTo(actionRect.left));
        _expectRectInside(imageRect, layerRect);
        for (var index = 0; index < 2; index++) {
          final chipRect = tester.getRect(
            find.byKey(ValueKey('product-card-hashtag-$index')),
          );
          _expectRectInside(layerRect, chipRect);
          expect(chipRect.overlaps(actionRect), isFalse);
        }

        if (textScale == 1) {
          _expectTextFitsChip(tester, 0, '#冷凍水產');
          _expectTextFitsChip(tester, 1, '#花枝');
        }
      });
    }
  }

  for (final viewport in <double>[320, 390, 768]) {
    testWidgets('card uses its 16px typography role for nonlinear scaling at '
        '${viewport.toInt()}px', (tester) async {
      const expectedScale = 1.3;
      final spec = ProductGridSpec.forWidth(viewport, textScale: expectedScale);
      await _pumpCard(
        tester,
        viewportWidth: viewport,
        cardWidth: spec.itemWidth,
        cardHeight: spec.mainAxisExtent,
        textScaler: const _ProductRoleNonlinearScaler(),
        name: '澎湖現流急凍小花枝切片家庭號',
        price: '299',
        priceMax: '350',
        listPriceMin: '350',
        listPriceMax: '499',
        hashtags: const ['冷凍水產', '花枝'],
      );

      expect(tester.takeException(), isNull);
      final cardContext = tester.element(find.byType(ProductCard));
      expect(ProductCard.effectiveTextScale(cardContext), expectedScale);
      final surfaceRect = tester.getRect(
        find.byKey(const ValueKey('product-card-surface')),
      );
      final imageRect = tester.getRect(
        find.byKey(const ValueKey('product-card-image')),
      );
      expect(
        surfaceRect.height - imageRect.height,
        closeTo(
          ProductGridSpec.bodyExtent +
              (expectedScale - 1) * ProductGridSpec.bodyTextScaleGrowth,
          0.001,
        ),
      );
    });
  }

  testWidgets('one physical tap invokes exactly one action', (tester) async {
    var cardTaps = 0;
    var addTaps = 0;
    await _pumpCard(
      tester,
      hashtags: const ['冷凍水產'],
      onTap: () => cardTaps++,
      onAddToCartTap: () => addTaps++,
    );

    await tester.tap(find.byKey(const ValueKey('product-card-title')));
    await tester.pump(const Duration(milliseconds: 200));
    expect(cardTaps, 1);
    expect(addTaps, 0);

    await tester.tap(find.byKey(const ValueKey('product-card-add-action')));
    await tester.pump();
    expect(cardTaps, 1, reason: 'the nested add action must not bubble');
    expect(addTaps, 1);
  });

  testWidgets('favorite action is independent from card and cart taps', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    try {
      var cardTaps = 0;
      var favoriteTaps = 0;
      await _pumpCard(
        tester,
        hashtags: const ['冷凍水產'],
        onTap: () => cardTaps++,
        isFavorited: true,
        onFavoriteTap: () => favoriteTaps++,
      );

      expect(
        find.byKey(const ValueKey('product-card-favorite')),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel('取消收藏 小花枝'), findsOneWidget);
      final semanticsData = tester
          .getSemantics(find.bySemanticsLabel('取消收藏 小花枝'))
          .getSemanticsData();
      expect(semanticsData.flagsCollection.isButton, isTrue);
      expect(semanticsData.flagsCollection.isEnabled, Tristate.isTrue);
      expect(semanticsData.flagsCollection.isToggled, Tristate.isTrue);
      expect(
        tester.getSize(find.byKey(const ValueKey('product-card-favorite'))),
        const Size.square(HkControl.minTap),
      );
      expect(
        tester.getSize(
          find.byKey(const ValueKey('product-card-favorite-surface')),
        ),
        const Size.square(36),
      );
      final favoriteSurface = tester.widget<HkLiquidGlassSurface>(
        find.byKey(const ValueKey('product-card-favorite-surface')),
      );
      expect(favoriteSurface.variant, HkLiquidGlassVariant.clear);
      expect(favoriteSurface.surfaceOpacity, HkGlass.regularSurfaceOpacity);
      expect(favoriteSurface.paintSurfaceColorBeneathGradient, isTrue);
      final favoriteBlur = tester.widget<BackdropFilter>(
        find.descendant(
          of: find.byKey(const ValueKey('product-card-favorite-surface')),
          matching: find.byType(BackdropFilter),
        ),
      );
      expect(favoriteBlur.enabled, isTrue);
      expect(favoriteBlur.filterConfig, isNotNull);
      final favoriteIcon = tester.widget<Icon>(
        find.descendant(
          of: find.byKey(const ValueKey('product-card-favorite-surface')),
          matching: find.byType(Icon),
        ),
      );
      expect(favoriteIcon.color, AppColors.favoriteActive);
      await tester.tap(find.byKey(const ValueKey('product-card-favorite')));
      await tester.pump();
      expect(favoriteTaps, 1);
      expect(cardTaps, 0);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('favorite uses stronger clear glass while chips stay opaque', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      hashtags: const ['冷凍水產'],
      onFavoriteTap: () {},
      favoriteSurfaceStyle: ProductFavoriteSurfaceStyle.hashtag,
    );

    final favoriteSurface = tester.widget<HkLiquidGlassSurface>(
      find.byKey(const ValueKey('product-card-favorite-surface')),
    );
    expect(favoriteSurface.variant, HkLiquidGlassVariant.clear);
    expect(favoriteSurface.surfaceOpacity, HkGlass.regularSurfaceOpacity);
    expect(favoriteSurface.paintSurfaceColorBeneathGradient, isTrue);
    expect(
      tester.getSize(find.byKey(const ValueKey('product-card-favorite'))),
      const Size.square(HkControl.minTap),
    );
    expect(
      tester.getSize(
        find.byKey(const ValueKey('product-card-favorite-surface')),
      ),
      const Size.square(33),
    );
    final glassDecoration =
        tester
                .widget<DecoratedBox>(
                  find.byKey(
                    const ValueKey('product-card-favorite-glass-fill'),
                  ),
                )
                .decoration
            as BoxDecoration;
    expect(glassDecoration.color, isNull);
    final specularGradient = glassDecoration.gradient! as LinearGradient;
    final baseFill = AppColors.neutralWhite.withValues(
      alpha: HkGlass.regularSurfaceOpacity,
    );
    expect(
      specularGradient.colors.first.a,
      closeTo(
        Color.alphaBlend(
          AppColors.neutralWhite.withValues(
            alpha: HkGlass.clearSpecularOpacity,
          ),
          baseFill,
        ).a,
        0.001,
      ),
    );

    final hashtagContainer = tester.widget<Container>(
      find.descendant(
        of: find.byKey(const ValueKey('product-card-hashtag-0')),
        matching: find.byType(Container),
      ),
    );
    final hashtagDecoration = hashtagContainer.decoration! as BoxDecoration;
    expect(hashtagDecoration.color, AppColors.bgCard);
  });

  testWidgets('bare plus fallback invokes card action once', (tester) async {
    var cardTaps = 0;
    await _pumpCard(tester, hashtags: const ['冷凍水產'], onTap: () => cardTaps++);

    await tester.tap(find.byKey(const ValueKey('product-card-add-action')));
    await tester.pump();
    expect(cardTaps, 1);
  });

  testWidgets('sold-out card stays readable and disables add action', (
    tester,
  ) async {
    var cardTaps = 0;
    var addTaps = 0;
    await _pumpCard(
      tester,
      hashtags: const ['冷凍水產'],
      isSoldOut: true,
      onTap: () => cardTaps++,
      onAddToCartTap: () => addTaps++,
    );

    expect(find.byKey(const ValueKey('product-card-sold-out')), findsOneWidget);
    expect(find.text('已售完'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('product-card-add-action')));
    await tester.pump();
    expect(addTaps, 0);
    expect(cardTaps, 0);

    await tester.tap(find.byKey(const ValueKey('product-card-title')));
    await tester.pump();
    expect(cardTaps, 1);
  });

  testWidgets('card and add action expose separate labeled semantics', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    try {
      await _pumpCard(
        tester,
        hashtags: const ['冷凍水產'],
        onTap: () {},
        onAddToCartTap: () {},
      );

      expect(
        find.bySemanticsLabel('查看商品：小花枝，商家：金湖水產，價格：新台幣 200 元'),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel('加入購物車：小花枝'), findsOneWidget);
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    } finally {
      handle.dispose();
    }
  });

  testWidgets('Web surface tokens and 4:3 media parent contract match', (
    tester,
  ) async {
    const cardWidth = 240.0;
    const textPanelHeight = 72.0;
    await _pumpCard(
      tester,
      viewportWidth: 768,
      cardWidth: cardWidth,
      cardHeight: cardWidth * 3 / 4 + textPanelHeight,
      hashtags: const ['冷凍水產'],
    );

    final surface = tester.widget<Container>(
      find.byKey(const ValueKey('product-card-surface')),
    );
    final decoration = surface.decoration! as BoxDecoration;
    expect(decoration.color, const Color(0xFFFFFDF7));
    expect(decoration.color, AppColors.bgCard);
    expect(decoration.borderRadius, BorderRadius.circular(HkRadius.md));
    final foreground = surface.foregroundDecoration! as BoxDecoration;
    final border = foreground.border! as Border;
    expect(border.top.color, AppColors.borderSubtle);
    expect(border.top.width, 1);
    expect(decoration.boxShadow, hasLength(1));
    for (var index = 0; index < HkShadows.sm.length; index++) {
      final actual = decoration.boxShadow![index];
      final expected = HkShadows.sm[index];
      expect(actual.color, expected.color);
      expect(actual.offset, expected.offset);
      expect(actual.blurRadius, expected.blurRadius);
    }

    final imageRect = tester.getRect(
      find.byKey(const ValueKey('product-card-image')),
    );
    // The foreground border preserves the exact Web 4:3 parent geometry.
    expect(imageRect.width / imageRect.height, closeTo(4 / 3, 0.001));
  });

  for (final config in <({double viewport, double width, double cardRadius})>[
    (viewport: 390, width: 165, cardRadius: HkRadius.md),
    (viewport: 1024, width: 320, cardRadius: HkRadius.lg),
  ]) {
    testWidgets('media and body share one flush surface at '
        '${config.viewport.toInt()}px', (tester) async {
      await _pumpCard(
        tester,
        viewportWidth: config.viewport,
        cardWidth: config.width,
        cardHeight: config.width * 3 / 4 + 72,
        hashtags: const ['冷凍水產'],
      );

      final mediaRect = tester.getRect(
        find.byKey(const ValueKey('product-card-media')),
      );
      final frameFinder = find.byKey(
        const ValueKey('product-card-media-frame'),
      );
      final frameRect = tester.getRect(frameFinder);
      final bodyRect = tester.getRect(
        find.byKey(const ValueKey('product-card-body')),
      );
      expect(mediaRect.width / mediaRect.height, closeTo(4 / 3, 0.001));
      expect(frameRect.left, closeTo(mediaRect.left, 0.1));
      expect(frameRect.top, closeTo(mediaRect.top, 0.1));
      expect(frameRect.right, closeTo(mediaRect.right, 0.1));
      expect(frameRect.bottom, closeTo(mediaRect.bottom, 0.1));
      expect(bodyRect.left, closeTo(mediaRect.left, 0.1));
      expect(bodyRect.right, closeTo(mediaRect.right, 0.1));
      expect(bodyRect.top, closeTo(mediaRect.bottom, 0.1));

      final frame = tester.widget<ClipRSuperellipse>(frameFinder);
      expect(
        frame.borderRadius,
        BorderRadius.only(
          topLeft: Radius.circular(config.cardRadius),
          topRight: Radius.circular(config.cardRadius),
        ),
      );
      final surface = tester.widget<Container>(
        find.byKey(const ValueKey('product-card-surface')),
      );
      final decoration = surface.decoration! as BoxDecoration;
      expect(decoration.borderRadius, BorderRadius.circular(config.cardRadius));
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('body rows reserve the seam action and keep even vertical gaps', (
    tester,
  ) async {
    await _pumpCard(tester, hashtags: const ['冷凍水產']);

    final titleRect = tester.getRect(
      find.byKey(const ValueKey('product-card-title')),
    );
    final priceRect = tester.getRect(
      find.byKey(const ValueKey('product-card-price-row')),
    );
    final sellerRect = tester.getRect(
      find.byKey(const ValueKey('product-card-seller-row')),
    );
    final mediaRect = tester.getRect(
      find.byKey(const ValueKey('product-card-media')),
    );
    final surfaceRect = tester.getRect(
      find.byKey(const ValueKey('product-card-surface')),
    );
    expect(priceRect.width, closeTo(titleRect.width, 0.1));
    expect(titleRect.width - sellerRect.width, closeTo(44, 0.1));

    final actionRect = tester.getRect(
      find.byKey(const ValueKey('product-card-add-action')),
    );
    expect(sellerRect.right, lessThanOrEqualTo(actionRect.left));

    final topGap = sellerRect.top - mediaRect.bottom;
    final sellerTitleGap = titleRect.top - sellerRect.bottom;
    final titlePriceGap = priceRect.top - titleRect.bottom;
    final bottomGap = surfaceRect.bottom - priceRect.bottom;
    expect(topGap, closeTo(bottomGap, 0.1));
    expect(sellerTitleGap, closeTo(titlePriceGap, 0.1));
    expect(topGap, closeTo(sellerTitleGap, 0.1));
  });

  testWidgets('seller metadata sits between clean media and compact title', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 165,
      cardHeight: 165 * 3 / 4 + 72,
      hashtags: const ['冷凍水產'],
    );

    final mediaFinder = find.byKey(const ValueKey('product-card-media'));
    final mediaRect = tester.getRect(mediaFinder);
    final sellerRect = tester.getRect(
      find.byKey(const ValueKey('product-card-seller-row')),
    );
    final titleRect = tester.getRect(
      find.byKey(const ValueKey('product-card-title')),
    );
    expect(sellerRect.top, greaterThanOrEqualTo(mediaRect.bottom));
    expect(sellerRect.bottom, lessThanOrEqualTo(titleRect.top));

    final sellerText = tester.widget<Text>(find.text('金湖水產'));
    final titleText = tester.widget<Text>(find.text('小花枝'));
    expect(sellerText.style?.fontSize, 14);
    expect(titleText.style?.fontSize, 14);
    expect(
      tester.getSize(find.byKey(const ValueKey('product-card-seller-avatar'))),
      const Size.square(14),
    );

    final gradientDecorations = tester
        .widgetList<DecoratedBox>(
          find.descendant(of: mediaFinder, matching: find.byType(DecoratedBox)),
        )
        .map((widget) => widget.decoration)
        .whereType<BoxDecoration>()
        .where((decoration) => decoration.gradient != null);
    expect(
      gradientDecorations,
      isEmpty,
      reason: 'normal product media must not be darkened by an overlay',
    );
  });

  testWidgets('hashtag chips hug content and align to the left', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 165,
      cardHeight: 165 * 3 / 4 + 72,
      hashtags: const ['花枝'],
    );

    final layerRect = tester.getRect(
      find.byKey(const ValueKey('product-card-hashtag-layer')),
    );
    final chipRect = tester.getRect(
      find.byKey(const ValueKey('product-card-hashtag-0')),
    );
    final textRect = tester.getRect(find.text('#花枝'));

    expect(chipRect.left, closeTo(layerRect.left, 0.1));
    expect(chipRect.width, closeTo(textRect.width + 16, 1.1));
    expect(chipRect.width, lessThan(layerRect.width));
  });

  testWidgets('mobile keeps the complete active price and defers list price', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 165,
      cardHeight: 165 * 3 / 4 + 72,
      price: '299',
      priceMax: '350',
      listPriceMin: '350',
      listPriceMax: '499',
      hashtags: const [],
    );

    expect(find.text('350~499'), findsNothing);
    final activePrice = find.textContaining('299~350');
    expect(activePrice, findsOneWidget);
    expect(tester.getSize(activePrice).width, greaterThan(0));

    await _pumpCard(
      tester,
      viewportWidth: 768,
      cardWidth: 240,
      cardHeight: 240 * 3 / 4 + 72,
      price: '299',
      priceMax: '350',
      listPriceMin: '350',
      listPriceMax: '499',
      hashtags: const [],
    );

    expect(find.text('350~499'), findsOneWidget);
  });

  testWidgets('card follows Web limit and renders only two hashtags', (
    tester,
  ) async {
    await _pumpCard(tester, hashtags: const ['#冷凍水產', '花枝', '家庭號', '限時優惠']);

    expect(find.text('#冷凍水產'), findsOneWidget);
    expect(find.text('#花枝'), findsOneWidget);
    expect(find.text('#家庭號'), findsNothing);
    expect(find.text('#限時優惠'), findsNothing);
    expect(find.byKey(const ValueKey('product-card-hashtag-2')), findsNothing);
  });

  testWidgets('a single unusually long hashtag stays inside the card', (
    tester,
  ) async {
    await _pumpCard(tester, hashtags: const ['這是一個非常非常長的冷凍水產標籤']);

    expect(tester.takeException(), isNull);
    final layerRect = tester.getRect(
      find.byKey(const ValueKey('product-card-hashtag-layer')),
    );
    final chipRect = tester.getRect(
      find.byKey(const ValueKey('product-card-hashtag-0')),
    );
    _expectRectInside(layerRect, chipRect);
    final text = tester.widget<Text>(find.text('#這是一個非常非常長的冷凍水產標籤'));
    expect(text.overflow, TextOverflow.ellipsis);
  });

  testWidgets('add action keeps a bare plus and a 2dp card-colour ring', (
    tester,
  ) async {
    await _pumpCard(tester, hashtags: const ['冷凍水產']);

    final surface = tester.widget<Container>(
      find.byKey(const ValueKey('product-card-add-surface')),
    );
    final decoration = surface.decoration! as BoxDecoration;
    expect(decoration.color, AppColors.primary);
    expect(decoration.shape, BoxShape.circle);
    expect(tester.getSize(find.byWidget(surface)), const Size.square(40));
    final border = decoration.border! as Border;
    expect(border.top.color, AppColors.bgCard);
    expect(border.top.width, 2);
    expect(
      tester.getSize(find.byKey(const ValueKey('product-card-add-action'))),
      const Size.square(HkControl.minTap),
    );
    final mediaRect = tester.getRect(
      find.byKey(const ValueKey('product-card-media')),
    );
    final actionRect = tester.getRect(
      find.byKey(const ValueKey('product-card-add-action')),
    );
    expect(actionRect.center.dy, closeTo(mediaRect.bottom, 0.1));
    final cardRect = tester.getRect(
      find.byKey(const ValueKey('product-card-surface')),
    );
    final addSurfaceRect = tester.getRect(
      find.byKey(const ValueKey('product-card-add-surface')),
    );
    _expectRectInside(cardRect, addSurfaceRect);

    final plus = tester.widget<Icon>(
      find.descendant(
        of: find.byKey(const ValueKey('product-card-add-surface')),
        matching: find.byType(Icon),
      ),
    );
    expect(plus.icon, AppIcons.add);
    expect(plus.size, 18);
  });

  testWidgets('wraps only product media in a Hero when a tag is supplied', (
    tester,
  ) async {
    const heroTag = 'product-card:test:42';
    await _pumpCard(tester, hashtags: const [], heroTag: heroTag);

    final hero = tester.widget<Hero>(find.byType(Hero));
    expect(hero.tag, heroTag);
    expect(hero.transitionOnUserGestures, isTrue);
    expect(hero.flightShuttleBuilder, isNotNull);
    expect(
      find.descendant(
        of: find.byType(Hero),
        matching: find.byKey(const ValueKey('product-card-media-frame')),
      ),
      findsOneWidget,
    );
    expect(
      find.descendant(
        of: find.byType(Hero),
        matching: find.byKey(const ValueKey('product-card-title')),
      ),
      findsNothing,
    );
    expect(
      find.descendant(
        of: find.byType(Hero),
        matching: find.byKey(const ValueKey('product-card-add-action')),
      ),
      findsNothing,
    );
  });

  testWidgets('press feedback snaps when Reduce Motion is enabled', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      hashtags: const [],
      disableAnimations: true,
      onTap: () {},
    );

    final gesture = await tester.startGesture(
      tester.getCenter(find.byKey(const ValueKey('product-card-surface'))),
    );
    await tester.pump();
    final scale = tester.widget<AnimatedScale>(find.byType(AnimatedScale));
    expect(scale.scale, 1);
    expect(scale.duration, Duration.zero);
    await gesture.up();
  });
}

Future<void> _pumpCard(
  WidgetTester tester, {
  double viewportWidth = 320,
  double cardWidth = 140,
  double cardHeight = 177,
  double textScale = 1,
  TextScaler? textScaler,
  String name = '小花枝',
  String price = '200',
  String priceMax = '',
  String listPriceMin = '',
  String listPriceMax = '',
  VoidCallback? onTap,
  VoidCallback? onAddToCartTap,
  bool isSoldOut = false,
  bool isFavorited = false,
  VoidCallback? onFavoriteTap,
  ProductFavoriteSurfaceStyle favoriteSurfaceStyle =
      ProductFavoriteSurfaceStyle.elevated,
  Object? heroTag,
  bool disableAnimations = false,
  required List<String> hashtags,
}) async {
  final viewportHeight = viewportWidth >= 600 ? 1024.0 : 844.0;
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(viewportWidth, viewportHeight);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      child: MaterialApp(
        home: MediaQuery(
          data: MediaQueryData(
            size: Size(viewportWidth, viewportHeight),
            textScaler: textScaler ?? TextScaler.linear(textScale),
            disableAnimations: disableAnimations,
          ),
          child: Scaffold(
            body: Center(
              child: SizedBox(
                width: cardWidth,
                height: cardHeight,
                child: ProductCard(
                  name: name,
                  price: price,
                  priceMax: priceMax,
                  listPriceMin: listPriceMin,
                  listPriceMax: listPriceMax,
                  sellerName: '金湖水產',
                  hashtags: hashtags,
                  isSoldOut: isSoldOut,
                  isFavorited: isFavorited,
                  onFavoriteTap: onFavoriteTap,
                  favoriteSurfaceStyle: favoriteSurfaceStyle,
                  heroTag: heroTag,
                  onTap: onTap,
                  onAddToCartTap: onAddToCartTap,
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
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

void _expectTextFitsChip(WidgetTester tester, int index, String label) {
  final chipRect = tester.getRect(
    find.byKey(ValueKey('product-card-hashtag-$index')),
  );
  final text = tester.widget<Text>(find.text(label));
  final painter = TextPainter(
    text: TextSpan(text: label, style: text.style),
    textDirection: TextDirection.ltr,
  )..layout();
  expect(painter.width + 16, lessThanOrEqualTo(chipRect.width + 0.1));
}

void _expectRectInside(Rect outer, Rect inner) {
  const tolerance = 0.1;
  expect(inner.left, greaterThanOrEqualTo(outer.left - tolerance));
  expect(inner.top, greaterThanOrEqualTo(outer.top - tolerance));
  expect(inner.right, lessThanOrEqualTo(outer.right + tolerance));
  expect(inner.bottom, lessThanOrEqualTo(outer.bottom + tolerance));
}
