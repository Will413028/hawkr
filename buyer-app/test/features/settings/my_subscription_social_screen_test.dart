import 'dart:async';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart'
    as common;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as api;
import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/config/media_image.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/follow/data/follow_repository.dart';
import 'package:hawkr_buyer/features/product/data/product_repository.dart';
import 'package:hawkr_buyer/features/review/data/review_repository.dart';
import 'package:hawkr_buyer/features/seller/data/seller_model.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/features/settings/presentation/my_subscription_screen.dart';
import 'package:hawkr_buyer/features/visit/data/visit_repository.dart';
import 'package:fixnum/fixnum.dart' show Int64;
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockSellerRepository extends Mock implements SellerRepository {}

class _MockProductRepository extends Mock implements ProductRepository {}

class _MockVisitRepository extends Mock implements VisitRepository {}

class _MockFollowRepository extends Mock implements FollowRepository {}

class _MockReviewRepository extends Mock implements ReviewRepository {}

void main() {
  late _MockSellerRepository sellerRepository;
  late _MockProductRepository productRepository;

  setUp(() {
    sellerRepository = _MockSellerRepository();
    productRepository = _MockProductRepository();
    _visitRepository = _MockVisitRepository();
    _followRepository = _MockFollowRepository();
    _reviewRepository = _MockReviewRepository();

    when(
      () => sellerRepository.getMerchantInfo(any()),
    ).thenAnswer((_) async => _merchantInfo);
    when(() => sellerRepository.getRatingStats(any())).thenAnswer(
      (_) async => RatingStats.fromJson(
        _merchantRating['data']! as Map<String, dynamic>,
      ),
    );
    when(
      () => _followRepository.listFollows(pageToken: any(named: 'pageToken')),
    ).thenAnswer(
      (_) async => buyer.ListFollowsResponse(
        follows: [buyer.Follow(storeId: _store.id, store: _store)],
        totalSize: 1,
      ),
    );
    _stubFavorites([_favorite()]);
    when(
      () => _visitRepository.listVisits(pageToken: any(named: 'pageToken')),
    ).thenAnswer(
      (_) async => buyer.ListVisitsResponse(
        visits: [
          buyer.Visit(product: _product())
            ..ensureVisitTime().seconds = Int64(
              DateTime.now()
                      .subtract(const Duration(hours: 2))
                      .millisecondsSinceEpoch ~/
                  1000,
            ),
        ],
      ),
    );
    when(
      () => _visitRepository.batchCheckFavorites(any()),
    ).thenAnswer((_) async => {88});
    when(
      () => _reviewRepository.getMerchantProductReviewSummary(any()),
    ).thenAnswer(
      (_) async => <String, dynamic>{
        'hasTotal': true,
        'total': 18,
        'rating': 4.8,
      },
    );
  });

  test('saved product cards request a 4:3 image from the CDN', () {
    final policy = ImageConfig.policyFor(ImageScene.productLandscapeCard);

    expect(policy.width, 384);
    expect(policy.height, 288);
    expect(policy.width! / policy.height!, 4 / 3);
    // 伺服端裁切語意由 Worker preset 決定：card = 720×540 cover（同 4:3）。
    expect(
      ImageConfig.presetFor(ImageScene.productLandscapeCard),
      ImagePreset.card,
    );
  });

  testWidgets('favorite entry opens directly with the social commerce card', (
    tester,
  ) async {
    await _pumpScreen(
      tester,
      sellerRepository: sellerRepository,
      productRepository: productRepository,
      initialTab: 1,
      width: 320,
      textScale: 1.4,
    );

    expect(find.text('我的收藏'), findsOneWidget);
    expect(find.text('你的收藏牆'), findsOneWidget);
    expect(find.text('海風水產'), findsOneWidget);
    expect(find.text('炙燒鯖魚一夜干'), findsOneWidget);
    expect(find.text('已收藏'), findsOneWidget);
    expect(find.text('加入購物車'), findsOneWidget);
    expect(find.text('立即結帳'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('subscription-glass-backdrop')),
      findsOneWidget,
    );
    final favoriteCard = find.byKey(const ValueKey('favorite_88'));
    final actionSurface = find.byKey(
      const ValueKey('subscription-product-actions-88'),
    );
    expect(favoriteCard, findsOneWidget);
    expect(actionSurface, findsOneWidget);
    expect(
      find.descendant(of: favoriteCard, matching: find.byType(BackdropFilter)),
      findsWidgets,
    );
    expect(
      find.descendant(of: actionSurface, matching: find.byType(BackdropFilter)),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('subscription-product-add-icon')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('subscription-product-buy-icon')),
      findsOneWidget,
    );
    expect(find.bySemanticsLabel('取消收藏'), findsOneWidget);
    final favoriteImageSize = tester.getSize(
      find.byKey(const ValueKey('subscription-product-image-88')),
    );
    expect(favoriteImageSize.width, 112);
    expect(
      favoriteImageSize.width / favoriteImageSize.height,
      closeTo(4 / 3, 0.001),
    );

    final addButton = tester.widget<FilledButton>(
      find.byKey(const ValueKey('subscription-product-add')),
    );
    final buyButton = tester.widget<FilledButton>(
      find.byKey(const ValueKey('subscription-product-buy')),
    );
    expect(
      addButton.style?.backgroundColor?.resolve(const <WidgetState>{}),
      AppColors.commerceCartAction,
    );
    expect(
      addButton.style?.foregroundColor?.resolve(const <WidgetState>{}),
      AppColors.commerceCartForeground,
    );
    expect(
      buyButton.style?.backgroundColor?.resolve(const <WidgetState>{}),
      AppColors.commerceCheckoutAction,
    );
    expect(
      buyButton.style?.foregroundColor?.resolve(const <WidgetState>{}),
      AppColors.commerceCheckoutForeground,
    );
    expect(tester.takeException(), isNull);
    verify(() => _visitRepository.listFavorites(pageToken: '')).called(1);
  });

  testWidgets('favorite glass cards keep the shared 16px vertical rhythm', (
    tester,
  ) async {
    _stubFavorites([_favorite(), _favorite(id: '89', name: '海鹽午仔魚一夜干')]);

    await _pumpScreen(
      tester,
      sellerRepository: sellerRepository,
      productRepository: productRepository,
      initialTab: 1,
    );

    final firstCard = find.byKey(const ValueKey('favorite_88'));
    final secondCard = find.byKey(const ValueKey('favorite_89'));
    expect(firstCard, findsOneWidget);
    expect(secondCard, findsOneWidget);

    final gap =
        tester.getTopLeft(secondCard).dy - tester.getBottomLeft(firstCard).dy;
    expect(gap, HkSpace.lg);
    expect(tester.takeException(), isNull);
  });

  testWidgets('tab switch keeps title and recent-view context in sync', (
    tester,
  ) async {
    await _pumpScreen(
      tester,
      sellerRepository: sellerRepository,
      productRepository: productRepository,
      initialTab: 1,
    );

    expect(
      find.byKey(const ValueKey('subscription-tab-content-1')),
      findsOneWidget,
    );
    await tester.tap(find.text('瀏覽'));
    await tester.pump();

    expect(find.text('我的收藏'), findsNothing);
    expect(find.text('瀏覽紀錄'), findsOneWidget);
    expect(
      find.byKey(const ValueKey('subscription-tab-content-1')),
      findsNothing,
    );
    expect(
      find.byKey(const ValueKey('subscription-tab-content-2')),
      findsOneWidget,
    );

    await tester.pumpAndSettle();

    expect(find.text('瀏覽紀錄'), findsOneWidget);
    expect(find.text('最近足跡'), findsOneWidget);
    expect(find.textContaining('小時前看過'), findsOneWidget);
    expect(find.text('炙燒鯖魚一夜干'), findsOneWidget);
    final historyImageSize = tester.getSize(
      find.byKey(const ValueKey('subscription-product-image-88')),
    );
    expect(
      historyImageSize.width / historyImageSize.height,
      closeTo(4 / 3, 0.001),
    );
    expect(tester.takeException(), isNull);
    verify(() => _visitRepository.listVisits(pageToken: '')).called(1);
    verify(() => _visitRepository.batchCheckFavorites([88])).called(1);
  });

  testWidgets('checkout does not require seller id and exposes progress', (
    tester,
  ) async {
    final pendingProduct = Completer<Map<String, dynamic>>();
    _stubFavorites([_favorite(withStore: false)]);
    when(
      () => productRepository.getProductFullInfo('88'),
    ).thenAnswer((_) => pendingProduct.future);

    await _pumpScreen(
      tester,
      sellerRepository: sellerRepository,
      productRepository: productRepository,
      initialTab: 1,
    );

    await tester.tap(find.text('立即結帳'));
    await tester.pump();

    verify(() => productRepository.getProductFullInfo('88')).called(1);
    expect(
      find.byKey(const ValueKey('subscription-product-checkoutCoins-loading')),
      findsOneWidget,
    );

    pendingProduct.complete(<String, dynamic>{'skus': const []});
    await tester.pumpAndSettle();
    expect(find.text('立即結帳'), findsOneWidget);
    expect(tester.takeException(), isNull);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('every subscription tab returns to settings instead of popping', (
    tester,
  ) async {
    final router = GoRouter(
      initialLocation: '/',
      routes: [
        GoRoute(
          path: '/',
          builder: (_, state) => Scaffold(
            body: Text('main-tab-${state.uri.queryParameters['tab']}'),
          ),
        ),
        GoRoute(
          path: '/settings/subscription',
          builder: (_, state) => MySubscriptionScreen(
            initialTab:
                int.tryParse(state.uri.queryParameters['initialTab'] ?? '') ??
                0,
          ),
        ),
      ],
    );
    addTearDown(router.dispose);

    await _pumpRouterScreen(
      tester,
      router: router,
      sellerRepository: sellerRepository,
      productRepository: productRepository,
    );

    for (final tab in [0, 1, 2]) {
      unawaited(router.push('/settings/subscription?initialTab=$tab'));
      await tester.pumpAndSettle();

      await tester.tap(find.bySemanticsLabel('返回'));
      await tester.pumpAndSettle();

      expect(find.text('main-tab-4'), findsOneWidget);
    }
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'favorites load the next cursor page only when scrolled to the end',
    (tester) async {
      final firstPage = [
        for (var i = 0; i < 6; i++)
          _favorite(id: '${100 + i}', name: '第一頁商品$i'),
      ];
      when(() => _visitRepository.listFavorites(pageToken: '')).thenAnswer(
        (_) async => buyer.ListFavoritesResponse(
          favorites: firstPage,
          nextPageToken: 'page-2',
          totalSize: 7,
        ),
      );
      when(
        () => _visitRepository.listFavorites(pageToken: 'page-2'),
      ).thenAnswer(
        (_) async => buyer.ListFavoritesResponse(
          favorites: [_favorite(id: '200', name: '第二頁商品')],
          totalSize: 7,
        ),
      );

      await _pumpScreen(
        tester,
        sellerRepository: sellerRepository,
        productRepository: productRepository,
        initialTab: 1,
      );

      // 第一頁載入後不會自己把後面的頁讀完；計數用 API 的 total_size。
      verify(() => _visitRepository.listFavorites(pageToken: '')).called(1);
      verifyNever(() => _visitRepository.listFavorites(pageToken: 'page-2'));
      expect(find.text('7'), findsOneWidget);

      await tester.dragUntilVisible(
        find.byKey(const ValueKey('favorite_200')),
        find.byType(Scrollable).first,
        const Offset(0, -300),
      );
      await tester.pumpAndSettle();

      verify(
        () => _visitRepository.listFavorites(pageToken: 'page-2'),
      ).called(1);
      expect(find.byKey(const ValueKey('favorite_200')), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('subscription presents a clear relationship state', (
    tester,
  ) async {
    await _pumpScreen(
      tester,
      sellerRepository: sellerRepository,
      productRepository: productRepository,
      width: 320,
      textScale: 1.4,
    );

    expect(find.text('我的訂閱'), findsOneWidget);
    expect(find.text('你關注的店家'), findsOneWidget);
    expect(find.text('海風水產'), findsOneWidget);
    expect(find.text('4.8 · 36 則評價'), findsOneWidget);
    expect(find.bySemanticsLabel('取消訂閱 海風水產'), findsOneWidget);
    final followingCard = find.byKey(const ValueKey('following_42'));
    expect(followingCard, findsOneWidget);
    expect(
      find.descendant(of: followingCard, matching: find.byType(BackdropFilter)),
      findsOneWidget,
    );
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pumpRouterScreen(
  WidgetTester tester, {
  required GoRouter router,
  required SellerRepository sellerRepository,
  required ProductRepository productRepository,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sellerRepositoryProvider.overrideWithValue(sellerRepository),
        productRepositoryProvider.overrideWithValue(productRepository),
        visitRepositoryProvider.overrideWithValue(_visitRepository),
        followRepositoryProvider.overrideWithValue(_followRepository),
        reviewRepositoryProvider.overrideWithValue(_reviewRepository),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp.router(
          theme: ThemeData(
            useMaterial3: true,
            colorScheme: ColorScheme.fromSeed(seedColor: AppColors.primary),
            textTheme: AppTypography.textTheme,
          ),
          routerConfig: router,
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Future<void> _pumpScreen(
  WidgetTester tester, {
  required SellerRepository sellerRepository,
  required ProductRepository productRepository,
  int initialTab = 0,
  double width = 390,
  double textScale = 1,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sellerRepositoryProvider.overrideWithValue(sellerRepository),
        productRepositoryProvider.overrideWithValue(productRepository),
        visitRepositoryProvider.overrideWithValue(_visitRepository),
        followRepositoryProvider.overrideWithValue(_followRepository),
        reviewRepositoryProvider.overrideWithValue(_reviewRepository),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          theme: ThemeData(
            useMaterial3: true,
            colorScheme: ColorScheme.fromSeed(seedColor: AppColors.primary),
            textTheme: AppTypography.textTheme,
          ),
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(textScale)),
            child: child!,
          ),
          home: MySubscriptionScreen(initialTab: initialTab),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

late _MockVisitRepository _visitRepository;
late _MockFollowRepository _followRepository;
late _MockReviewRepository _reviewRepository;

final _store = common.StoreCard(
  id: '42',
  name: '海風水產',
  storeCode: '42',
  logo: api.Image(uriTemplate: ''),
);

buyer.ProductCard _product({
  String id = '88',
  String name = '炙燒鯖魚一夜干',
  bool withStore = true,
}) => buyer.ProductCard(
  id: id,
  name: name,
  minPrice: Int64(260),
  maxPrice: Int64(320),
  rating: 4.8,
  available: true,
  storeId: withStore ? _store.id : null,
  store: withStore ? _store : null,
);

buyer.Favorite _favorite({
  String id = '88',
  String name = '炙燒鯖魚一夜干',
  bool withStore = true,
}) => buyer.Favorite(
  product: _product(id: id, name: name, withStore: withStore),
);

void _stubFavorites(List<buyer.Favorite> favorites) {
  when(
    () => _visitRepository.listFavorites(pageToken: any(named: 'pageToken')),
  ).thenAnswer(
    (_) async => buyer.ListFavoritesResponse(
      favorites: favorites,
      totalSize: favorites.length,
    ),
  );
}

const _merchantInfo = <String, dynamic>{
  'data': <String, dynamic>{
    'storeName': '海風水產',
    'city': '基隆市',
    'district': '中正區',
    'rating': 4.8,
  },
};

/// `GET /public/sellers/{id}/rating` — 評論數只有這支有，SellerInfo 不帶。
const _merchantRating = <String, dynamic>{
  'code': 0,
  'data': <String, dynamic>{'storeId': '42', 'rating': 4.8, 'reviewCount': 36},
};
