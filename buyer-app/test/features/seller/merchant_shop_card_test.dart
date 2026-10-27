import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:convert';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/product/presentation/merchant_item_detail_screen.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_shop_tab.dart';
import 'package:hawkr_buyer/shared/icons/market_favorite_icon.dart';
import 'package:hawkr_buyer/shared/widgets/hk_frosted_card_surface.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

class _RecordingAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];
  final List<String> restockProductIds = [];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return ResponseBody.fromString(
      jsonEncode(const <String, dynamic>{
        'code': 0,
        'message': 'success',
        'data': <String, dynamic>{},
      }),
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
  const cases = <({double viewport, double cardWidth})>[
    (viewport: 320, cardWidth: 296),
    (viewport: 390, cardWidth: 358),
    (viewport: 430, cardWidth: 398),
    // 768dp storefront uses two columns: (720 - 16 gap) / 2 = 352.
    (viewport: 768, cardWidth: 352),
  ];

  for (final config in cases) {
    for (final textScale in <double>[1, 1.15, 1.4]) {
      testWidgets(
        'shop card fits ${config.viewport.toInt()}dp at ${textScale}x text',
        (tester) async {
          final adapter = await _pumpCard(
            tester,
            viewportWidth: config.viewport,
            cardWidth: config.cardWidth,
            textScale: textScale,
          );

          expect(tester.takeException(), isNull);
          expect(adapter.requests, isEmpty);
          expect(find.text('4.8 (128則評價)'), findsOneWidget);
          expect(find.text(r'NT$ 299~350'), findsOneWidget);
          expect(find.text(r'NT$ 350~499'), findsOneWidget);
          final stackedActions = find.byKey(
            const ValueKey('merchant-shop-actions-stacked'),
          );
          final inlineActions = find.byKey(
            const ValueKey('merchant-shop-actions-inline'),
          );
          expect(
            stackedActions.evaluate().length + inlineActions.evaluate().length,
            1,
            reason: 'the responsive action group must choose one safe layout',
          );
          final isStacked = stackedActions.evaluate().isNotEmpty;

          final title = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-title-row')),
          );
          final rating = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-rating-row')),
          );
          final price = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-price-row')),
          );
          final actions = tester.getRect(
            isStacked ? stackedActions : inlineActions,
          );

          expect(rating.top - title.bottom, closeTo(4, 0.1));
          expect(price.top - rating.bottom, closeTo(4, 0.1));
          expect(
            actions.top - price.bottom,
            closeTo(4, 0.1),
            reason: 'price and CTA group should use the compact 4dp rhythm',
          );
          expect(title.overlaps(rating), isFalse);
          expect(rating.overlaps(price), isFalse);

          final saleParagraph = tester.renderObject<RenderParagraph>(
            find.byKey(const ValueKey('merchant-shop-sale-price')),
          );
          final listParagraph = tester.renderObject<RenderParagraph>(
            find.byKey(const ValueKey('merchant-shop-list-price')),
          );
          expect(saleParagraph.didExceedMaxLines, isFalse);
          expect(listParagraph.didExceedMaxLines, isFalse);

          final image = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-image')),
          );
          final content = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-content')),
          );
          final card = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-card-surface')),
          );
          final naturalBodyHeight = image.height > content.height
              ? image.height
              : content.height;
          expect(image.width / image.height, closeTo(4 / 3, 0.01));
          expect(
            card.height,
            closeTo(naturalBodyHeight + 14, 0.1),
            reason:
                'card height must follow its tallest content column plus 10/4 padding',
          );

          final favoriteSize = tester.getSize(
            find.byKey(const ValueKey('merchant-shop-favorite-action')),
          );
          expect(favoriteSize, const Size(48, 48));
          for (final key in [
            'merchant-shop-add-action',
            'merchant-shop-buy-action',
          ]) {
            final size = tester.getSize(find.byKey(ValueKey(key)));
            expect(
              size.height,
              HkControl.minTap,
              reason: '$key retains a standards-compliant tap target',
            );
            expect(size.width, greaterThanOrEqualTo(HkControl.minTap));
          }
          for (final label in ['加入購物車', '立即結帳']) {
            final paragraph = tester.renderObject<RenderParagraph>(
              find.text(label),
            );
            expect(
              paragraph.didExceedMaxLines,
              isFalse,
              reason: '$label must remain fully readable',
            );
          }

          final add = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-add-action')),
          );
          final buy = tester.getRect(
            find.byKey(const ValueKey('merchant-shop-buy-action')),
          );
          if (isStacked) {
            expect(buy.top - add.bottom, closeTo(4, 0.1));
          } else {
            expect(buy.left - add.right, closeTo(4, 0.1));
            expect(buy.top, closeTo(add.top, 0.1));
          }
        },
      );
    }
  }

  testWidgets('shop list keeps 16px between frosted product cards', (
    tester,
  ) async {
    await _pumpShopTab(tester);

    final cards = find.byKey(const ValueKey('merchant-shop-card-surface'));
    expect(cards, findsNWidgets(2));
    final first = tester.getRect(cards.at(0));
    final second = tester.getRect(cards.at(1));

    expect(second.top - first.bottom, HkSpace.lg);
    expect(
      find.byKey(const ValueKey('merchant-shop-view-action')),
      findsNWidgets(2),
      reason:
          'The current seller-list contract omits stock; cards must fail '
          'closed to product detail instead of promising direct purchase.',
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('card uses the shared frosted surface and content hierarchy', (
    tester,
  ) async {
    await _pumpCard(tester, viewportWidth: 390, cardWidth: 358);

    final card = tester.widget<HkFrostedCardSurface>(
      find.byKey(const ValueKey('merchant-shop-card-surface')),
    );
    expect(card.radius, HkRadius.xl);
    expect(card.blurSigma, HkGlass.navBarBlurSigma);
    expect(
      find.descendant(
        of: find.byKey(const ValueKey('merchant-shop-card-surface')),
        matching: find.byType(BackdropFilter),
      ),
      findsOneWidget,
    );
    expect(
      tester
          .getSize(find.byKey(const ValueKey('merchant-shop-card-surface')))
          .height,
      lessThan(172),
    );
    expect(
      tester
          .getSize(find.byKey(const ValueKey('merchant-shop-title-row')))
          .height,
      lessThan(48),
    );
    final ratingStar = tester.widget<Icon>(
      find.byKey(const ValueKey('merchant-shop-rating-star')),
    );
    final ratingRow = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-rating-row')),
    );
    final ratingLabel = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-rating-label')),
    );
    expect(ratingStar.size, 13);
    expect(ratingLabel.center.dy, closeTo(ratingRow.center.dy, 0.5));

    final titleRow = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-title-row')),
    );
    final content = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-content')),
    );
    final favorite = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-favorite-action')),
    );
    expect(favorite.top, closeTo(titleRow.top, 0.1));
    expect(favorite.right, closeTo(content.right, 0.1));
    final image = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-image')),
    );
    final hashtag = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-hashtag-chip')),
    );
    expect(hashtag.left - image.left, closeTo(2, 0.1));
    expect(hashtag.width, lessThan(image.width - 4));

    final addSurface = tester.widget<Container>(
      find.byKey(const ValueKey('merchant-shop-cart-surface')),
    );
    final buySurface = tester.widget<Container>(
      find.byKey(const ValueKey('merchant-shop-checkout-surface')),
    );
    expect(tester.getSize(find.byWidget(addSurface)).height, 40);
    expect(tester.getSize(find.byWidget(buySurface)).height, 40);
    final cardRect = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-card-surface')),
    );
    final addSurfaceRect = tester.getRect(
      find.byKey(const ValueKey('merchant-shop-cart-surface')),
    );
    expect(
      cardRect.bottom - addSurfaceRect.bottom,
      closeTo(8, 0.1),
      reason: 'CTA bottom spacing should stay compact',
    );
    expect(
      (addSurface.decoration! as BoxDecoration).color,
      AppColors.commerceCartAction,
    );
    expect(
      (buySurface.decoration! as BoxDecoration).color,
      AppColors.commerceCheckoutAction,
    );
    final actionIcons = tester.widgetList<Icon>(
      find.descendant(
        of: find.byKey(const ValueKey('merchant-shop-card-surface')),
        matching: find.byType(Icon),
      ),
    );
    expect(
      actionIcons.map((icon) => icon.icon),
      containsAll(<IconData>[
        AppIcons.shoppingCartOutlined,
        AppIcons.paymentsOutlined,
      ]),
    );

    final sale = tester.widget<Text>(
      find.byKey(const ValueKey('merchant-shop-sale-price')),
    );
    final list = tester.widget<Text>(
      find.byKey(const ValueKey('merchant-shop-list-price')),
    );
    expect(sale.style!.color, AppColors.actionPrimary);
    expect(sale.style!.fontWeight, FontWeight.w700);
    final listSpans = (list.textSpan! as TextSpan).children!;
    expect((listSpans.first as TextSpan).text, r'NT$ ');
    expect((listSpans.first as TextSpan).style!.decoration, isNull);
    expect((listSpans.last as TextSpan).text, '350~499');
    expect(
      (listSpans.last as TextSpan).style!.decoration,
      TextDecoration.lineThrough,
    );
  });

  testWidgets('zero rating is labeled as no rating', (tester) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 358,
      rating: 0,
      reviewCount: 0,
    );

    expect(find.text('尚無評分'), findsOneWidget);
    expect(find.textContaining('0.0'), findsNothing);
  });

  testWidgets('web market heart aligns with title and fills after selection', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 358,
      interactiveFavorite: true,
    );

    expect(
      find.byKey(const ValueKey('merchant-shop-favorite-outline')),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel('加入收藏'), findsOneWidget);

    await tester.tap(
      find.byKey(const ValueKey('merchant-shop-favorite-action')),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('merchant-shop-favorite-filled')),
      findsOneWidget,
    );
    final filledHeart = tester.widget<MarketFavoriteIcon>(
      find.byKey(const ValueKey('merchant-shop-favorite-filled')),
    );
    expect(filledHeart.isFavorite, isTrue);
    expect(filledHeart.iconData, AppIcons.favorite);
    expect(filledHeart.size, 24);
    expect(filledHeart.color, AppColors.favoriteActive);
    expect(find.bySemanticsLabel('取消收藏'), findsOneWidget);

    await tester.tap(
      find.byKey(const ValueKey('merchant-shop-favorite-action')),
    );
    await tester.pumpAndSettle();

    expect(
      find.byKey(const ValueKey('merchant-shop-favorite-filled')),
      findsNothing,
    );
    final outlineHeart = tester.widget<MarketFavoriteIcon>(
      find.byKey(const ValueKey('merchant-shop-favorite-outline')),
    );
    expect(outlineHeart.isFavorite, isFalse);
    expect(outlineHeart.iconData, AppIcons.favoriteBorder);
    expect(outlineHeart.size, 24);
    expect(outlineHeart.color, AppColors.neutralMutedText);
    expect(find.bySemanticsLabel('加入收藏'), findsOneWidget);
  });

  testWidgets('favorite press matches the web 150ms ease-out interaction', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 358,
      interactiveFavorite: true,
    );

    final action = find.byKey(const ValueKey('merchant-shop-favorite-action'));
    expect(
      find.descendant(of: action, matching: find.byType(AnimatedSwitcher)),
      findsNothing,
    );
    final gesture = await tester.startGesture(tester.getCenter(action));
    await tester.pump();

    var pressScale = tester.widget<AnimatedScale>(
      find.byKey(const ValueKey('merchant-shop-favorite-press-scale')),
    );
    expect(pressScale.scale, 0.9);
    expect(pressScale.duration, const Duration(milliseconds: 150));
    expect(pressScale.curve, Curves.easeOut);

    await gesture.cancel();
    await tester.pump();

    pressScale = tester.widget<AnimatedScale>(
      find.byKey(const ValueKey('merchant-shop-favorite-press-scale')),
    );
    expect(pressScale.scale, 1);
    expect(
      find.byKey(const ValueKey('merchant-shop-favorite-outline')),
      findsOneWidget,
    );

    final completedGesture = await tester.startGesture(
      tester.getCenter(action),
    );
    await tester.pump();
    await completedGesture.up();
    await tester.pump();

    expect(
      find.byKey(const ValueKey('merchant-shop-favorite-filled')),
      findsOneWidget,
    );
  });

  testWidgets('favorite press becomes immediate when motion is disabled', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 358,
      interactiveFavorite: true,
      disableAnimations: true,
    );

    final action = find.byKey(const ValueKey('merchant-shop-favorite-action'));
    final gesture = await tester.startGesture(tester.getCenter(action));
    await tester.pump();

    var pressScale = tester.widget<AnimatedScale>(
      find.byKey(const ValueKey('merchant-shop-favorite-press-scale')),
    );
    expect(pressScale.scale, 1);
    expect(pressScale.duration, Duration.zero);

    await gesture.up();
    await tester.pump();

    pressScale = tester.widget<AnimatedScale>(
      find.byKey(const ValueKey('merchant-shop-favorite-press-scale')),
    );
    expect(pressScale.scale, 1);
    expect(
      find.byKey(const ValueKey('merchant-shop-favorite-filled')),
      findsOneWidget,
    );
  });

  testWidgets('list payload card performs no eager detail or review requests', (
    tester,
  ) async {
    final adapter = await _pumpCard(tester, viewportWidth: 390, cardWidth: 358);
    await tester.pump(const Duration(milliseconds: 100));

    expect(adapter.requests, isEmpty);
    expect(find.byType(HkBrandLoader), findsNothing);
    expect(find.text('4.8 (128則評價)'), findsOneWidget);
    expect(find.text(r'NT$ 299~350'), findsOneWidget);
  });

  testWidgets('sold-out restock action calls the real subscription endpoint', (
    tester,
  ) async {
    final adapter = await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 358,
      inventory: 0,
      signedIn: true,
    );
    await tester.pumpAndSettle();

    await tester.tap(
      find.byKey(const ValueKey('merchant-shop-restock-action')),
    );
    await tester.pumpAndSettle();

    expect(adapter.restockProductIds, ['1001']);
    expect(find.text('已開啟補貨通知，補貨時會通知你'), findsOneWidget);
    expect(find.text('已開啟補貨通知'), findsOneWidget);

    // A parent rebuild can still carry the stale list value `false`; the
    // successful local state must stay disabled and avoid a duplicate POST.
    await tester.tap(
      find.byKey(const ValueKey('merchant-shop-favorite-action')),
    );
    await tester.pump();
    expect(find.text('已開啟補貨通知'), findsOneWidget);
    final restock = find.byKey(const ValueKey('merchant-shop-restock-action'));
    final detector = tester.widget<GestureDetector>(
      find.descendant(of: restock, matching: find.byType(GestureDetector)),
    );
    expect(detector.onTap, isNull);
    expect(adapter.restockProductIds, ['1001']);
    expect(tester.takeException(), isNull);
    await tester.pump(const Duration(milliseconds: 2100));
  });

  testWidgets('unknown inventory fails closed to one product-detail action', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 358,
      inventory: null,
    );

    final view = find.byKey(const ValueKey('merchant-shop-view-action'));
    expect(view, findsOneWidget);
    expect(tester.getSize(view).height, HkControl.minTap);
    expect(find.text('查看商品'), findsOneWidget);
    expect(find.text('加入購物車'), findsNothing);
    expect(find.text('立即結帳'), findsNothing);
    expect(find.text('補貨通知'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('an existing restock subscription cannot be submitted twice', (
    tester,
  ) async {
    final adapter = await _pumpCard(
      tester,
      viewportWidth: 390,
      cardWidth: 358,
      inventory: 0,
      isRestockSubscribed: true,
      signedIn: true,
    );

    final restock = find.byKey(const ValueKey('merchant-shop-restock-action'));
    expect(find.text('已開啟補貨通知'), findsOneWidget);
    final detector = tester.widget<GestureDetector>(
      find.descendant(of: restock, matching: find.byType(GestureDetector)),
    );
    expect(detector.onTap, isNull);
    expect(adapter.requests, isEmpty);
  });

  testWidgets('detail cancellation result re-enables the card restock action', (
    tester,
  ) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 900);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final router = GoRouter(
      initialLocation: '/',
      routes: [
        GoRoute(
          path: '/',
          builder: (context, state) => Scaffold(
            body: Center(
              child: SizedBox(
                width: 358,
                child: MerchantShopCard(
                  merchantId: '2683382637',
                  merchantName: '金湖水產',
                  sku: '1001',
                  title: '澎湖現流急凍小花枝切片家庭號',
                  imageUrls: const [],
                  fallbackPrice: 299,
                  rating: 4.8,
                  inventory: 0,
                  isFav: false,
                  onToggleFavorite: () {},
                  isRestockSubscribed: true,
                ),
              ),
            ),
          ),
        ),
        GoRoute(
          path: '/merchant-product',
          builder: (context, state) {
            final detail = state.extra! as MerchantItemDetailScreen;
            return Scaffold(
              body: Column(
                children: [
                  Text(
                    detail.initialRestockSubscribed ? '詳情已同步訂閱狀態' : '詳情狀態錯誤',
                  ),
                  ElevatedButton(
                    key: const ValueKey('cancel-restock-in-detail'),
                    onPressed: () => context.pop(<String, dynamic>{
                      'productId': '1001',
                      'isFavorite': false,
                      'isRestockSubscribed': false,
                    }),
                    child: const Text('取消補貨通知並返回'),
                  ),
                ],
              ),
            );
          },
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(child: MaterialApp.router(routerConfig: router)),
    );
    await tester.pumpAndSettle();
    expect(find.text('已開啟補貨通知'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('merchant-shop-card-surface')));
    await tester.pumpAndSettle();
    expect(find.text('詳情已同步訂閱狀態'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('cancel-restock-in-detail')));
    await tester.pumpAndSettle();

    expect(find.text('補貨通知'), findsOneWidget);
    final restock = find.byKey(const ValueKey('merchant-shop-restock-action'));
    final detector = tester.widget<GestureDetector>(
      find.descendant(of: restock, matching: find.byType(GestureDetector)),
    );
    expect(detector.onTap, isNotNull);
    expect(tester.takeException(), isNull);
  });

  testWidgets('card and actions expose labeled accessible tap targets', (
    tester,
  ) async {
    final handle = tester.ensureSemantics();
    try {
      await _pumpCard(tester, viewportWidth: 390, cardWidth: 358);

      expect(
        find.bySemanticsLabel(r'查看商品：澎湖現流急凍小花枝切片家庭號，價格：NT$ 299~350'),
        findsOneWidget,
      );
      expect(find.bySemanticsLabel('加入購物車'), findsOneWidget);
      expect(find.bySemanticsLabel('立即結帳'), findsOneWidget);
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    } finally {
      handle.dispose();
    }
  });
}

Future<_RecordingAdapter> _pumpCard(
  WidgetTester tester, {
  required double viewportWidth,
  required double cardWidth,
  double textScale = 1,
  double rating = 4.8,
  int? reviewCount = 128,
  int? inventory = 12,
  bool interactiveFavorite = false,
  bool signedIn = false,
  bool isRestockSubscribed = false,
  bool disableAnimations = false,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(viewportWidth, 900);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);

  final adapter = _RecordingAdapter();
  final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);
  var favorite = false;

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(dio),
        connectAuthTransportProvider.overrideWith(
          (_) => FakeTransportBuilder().unary(
            BuyerAppService.subscribeRestock,
            (request, _) {
              adapter.restockProductIds.add(request.productId);
              return buyer.SubscribeRestockResponse();
            },
          ).build(),
        ),
        if (signedIn) sessionProvider.overrideWith(_SignedInSession.new),
      ],
      child: Consumer(
        builder: (context, ref, _) {
          if (signedIn) ref.watch(sessionProvider);
          return ScreenUtilInit(
            designSize: const Size(414, 896),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => StatefulBuilder(
              builder: (context, setCardState) => MaterialApp(
                home: MediaQuery(
                  data: AppMediaQueryPolicy.apply(
                    MediaQueryData(
                      size: Size(viewportWidth, 900),
                      textScaler: TextScaler.linear(textScale),
                      disableAnimations: disableAnimations,
                    ),
                  ),
                  child: Scaffold(
                    body: Center(
                      child: SizedBox(
                        width: cardWidth,
                        child: MerchantShopCard(
                          widgetRef: interactiveFavorite ? null : ref,
                          merchantId: '2683382637',
                          merchantName: '金湖水產',
                          sku: '1001',
                          title: '澎湖現流急凍小花枝切片家庭號',
                          imageUrls: const [],
                          fallbackPrice: 299,
                          fallbackPriceMax: 350,
                          rating: rating,
                          reviewCount: reviewCount,
                          inventory: inventory,
                          isFav: favorite,
                          onToggleFavorite: () =>
                              setCardState(() => favorite = !favorite),
                          hashtags: const ['冷凍水產'],
                          listPriceMin: '350',
                          listPriceMax: '499',
                          salePriceMin: '299',
                          salePriceMax: '350',
                          isRestockSubscribed: isRestockSubscribed,
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          );
        },
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 10));
  return adapter;
}

class _SignedInSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'buyer-1', nickname: 'Test Buyer');
}

Future<void> _pumpShopTab(WidgetTester tester) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 900);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);

  final adapter = _RecordingAdapter();
  final dio = Dio(BaseOptions(baseUrl: 'https://test.local'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);

  const products = <Map<String, dynamic>>[
    {
      'id': '1001',
      'name': '高麗菜',
      'priceMin': '500',
      'priceMax': '500',
      'rating': '0',
      'stock': '',
      'isRestockSubscribed': false,
    },
    {
      'id': '1002',
      'name': '南畝王生技中都－台灣紅豆',
      'priceMin': '195',
      'priceMax': '195',
      'rating': '0',
      'stock': '',
      'isRestockSubscribed': false,
    },
  ];

  await tester.pumpWidget(
    ProviderScope(
      overrides: [dioProvider.overrideWithValue(dio)],
      child: Consumer(
        builder: (context, ref, _) => MaterialApp(
          home: Scaffold(
            body: MerchantShopTab(
              ref: ref,
              merchantId: '2683382637',
              merchantName: '南畝王生技',
              prefetchedProducts: products,
              prefetchedReady: true,
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}
