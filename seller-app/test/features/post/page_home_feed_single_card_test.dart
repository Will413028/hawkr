import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/config/image_config.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/page_home_feed_screen.dart';
import 'package:hawkr_seller/shared/widgets/hk_liquid_glass.dart';
import 'package:hawkr_seller/shared/widgets/optimized_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockPostRepository extends Mock implements PostRepository {}

class _PopObserver extends NavigatorObserver {
  int pops = 0;

  @override
  void didPop(Route<dynamic> route, Route<dynamic>? previousRoute) {
    pops++;
    super.didPop(route, previousRoute);
  }
}

const _postId = 42;
const _sellerId = 7;
const _longProductName = '南歐王生技中都－台灣紅豆（500G／包）產地限定超長商品名稱';
const _feed = PageHomeFeedScreen(initialIndex: 0, sellerName: '測試店家');

Post _postFixture() => Post(
  id: _postId,
  sellerId: _sellerId,
  contentType: 'IMAGE',
  imageKeys: const ['public/post/20260911/harvest.GIF?rev=2'],
  content: List.filled(
    9,
    '這是一段用來驗證窄螢幕與放大字級的產地長文，展開說明後可閱讀完整內文，互動按鈕仍然保持可用。#產地直送',
  ).join('\n'),
  likeCount: 1234567,
  commentCount: 654321,
  linkedProduct: const <String, dynamic>{
    'id': '866542675787809994',
    'name': _longProductName,
    'price': '123456789',
    'imageKey': '',
  },
);

EdgeInsets _safeInsets(Size size) => size.width > size.height
    ? const EdgeInsets.fromLTRB(59, 0, 59, 21)
    : const EdgeInsets.only(top: 62, bottom: 34);

Future<void> _pumpFeed(
  WidgetTester tester, {
  required Size size,
  required double scale,
  GoRouter? router,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);

  final repository = _MockPostRepository();
  when(
    () => repository.listMyPosts(),
  ).thenAnswer((_) async => [_postFixture()]);
  when(() => repository.listCommentsRaw(_postId)).thenAnswer((_) async => []);

  Widget withMediaQuery(BuildContext context, Widget? child) => MediaQuery(
    data: MediaQuery.of(context).copyWith(
      textScaler: TextScaler.linear(scale),
      padding: _safeInsets(size),
    ),
    child: child!,
  );
  await tester.pumpWidget(
    ProviderScope(
      overrides: [postRepositoryProvider.overrideWithValue(repository)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        minTextAdapt: false,
        splitScreenMode: true,
        builder: (_, _) => router == null
            ? MaterialApp(
                theme: AppTheme.light,
                builder: withMediaQuery,
                home: _feed,
              )
            : MaterialApp.router(
                theme: AppTheme.light,
                builder: withMediaQuery,
                routerConfig: router,
              ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  for (final size in const [
    Size(320, 720),
    Size(402, 874),
    Size(440, 956),
    Size(874, 402),
  ]) {
    for (final scale in [1.0, 1.4]) {
      testWidgets(
        '$size / $scale: buyer-style preview keeps media and actions fixed while caption expands',
        (tester) async {
          final semantics = tester.ensureSemantics();
          try {
            await _pumpFeed(tester, size: size, scale: scale);
            final pageView = find.byKey(
              const ValueKey('seller-post-card-page-view'),
            );
            expect(
              tester.widget<PageView>(pageView).scrollDirection,
              Axis.horizontal,
            );

            final window = find.byKey(
              const ValueKey('seller-post-card-$_postId'),
            );
            final media = find.byKey(
              const ValueKey('seller-post-media-$_postId'),
            );
            final actions = find.byKey(
              const ValueKey('seller-post-action-bar-$_postId'),
            );
            final product = find.byKey(
              const ValueKey('seller-post-linked-product-$_postId'),
            );
            final caption = find.byKey(
              const ValueKey('seller-post-caption-text-$_postId'),
            );
            final toggle = find.byKey(
              const ValueKey('seller-post-caption-toggle-$_postId'),
            );
            final scroll = find.byKey(
              const ValueKey('seller-post-content-scroll-$_postId'),
            );
            final windowRect = tester.getRect(window);
            final safe = _safeInsets(size);
            expect(windowRect.left, closeTo(safe.left + 8, 0.01));
            expect(
              windowRect.right,
              closeTo(size.width - safe.right - 8, 0.01),
            );
            expect(windowRect.top, closeTo(safe.top + 8, 0.01));
            expect(
              windowRect.bottom,
              closeTo(size.height - safe.bottom - 12, 0.01),
            );
            final decoration =
                tester.widget<DecoratedBox>(window).decoration as BoxDecoration;
            expect(
              decoration.borderRadius,
              BorderRadius.circular(HkRadius.xxl),
            );
            final mediaBefore = tester.getRect(media);
            final actionBefore = tester.getRect(actions);
            expect(actionBefore.height, 64);
            expect(actionBefore.bottom, closeTo(windowRect.bottom, 0.01));
            expect(actions.hitTestable(), findsOneWidget);
            expect(
              find.byType(TextField),
              findsNothing,
              reason: 'The comment composer opens in its own sheet.',
            );
            expect(find.text(_longProductName), findsOneWidget);
            expect(
              tester.getRect(product).left,
              greaterThanOrEqualTo(windowRect.left),
            );
            expect(
              tester.getRect(product).right,
              lessThanOrEqualTo(windowRect.right),
            );
            final amount = find.textContaining('123456789');
            expect(amount, findsOneWidget);
            expect(
              tester.renderObject<RenderParagraph>(amount).didExceedMaxLines,
              isFalse,
            );

            final postImage = tester.widget<OptimizedNetworkImage>(
              find.byKey(const ValueKey('seller-post-image-$_postId')),
            );
            expect(postImage.scene, ImageScene.postImage);
            expect(postImage.fit, BoxFit.contain);
            expect(postImage.imageUrl, endsWith('harvest.GIF?rev=2'));
            expect(
              tester
                  .getSemantics(find.bySemanticsLabel('GIF 動圖'))
                  .getSemanticsData()
                  .label,
              'GIF 動圖',
            );
            for (final label in ['返回店舖', '編輯貼文', '刪除貼文']) {
              final control = find.bySemanticsLabel(label);
              expect(control, findsOneWidget);
              expect(tester.getSize(control), const Size.square(48));
              expect(control.hitTestable(), findsOneWidget);
            }
            for (final control in tester.widgetList<HkLiquidGlassIconButton>(
              find.byType(HkLiquidGlassIconButton),
            )) {
              expect(control.backgroundColor, AppColors.neutralWhite);
              expect(control.surfaceOpacity, 0.92);
            }
            expect(find.text('99+'), findsNWidgets(2));
            expect(find.bySemanticsLabel('共 1234567 個讚'), findsOneWidget);
            expect(find.bySemanticsLabel('留言，共 654321 則'), findsOneWidget);
            expect(tester.widget<Text>(caption).maxLines, 1);
            expect(tester.getSize(toggle).height, greaterThanOrEqualTo(48));

            // Only the caption panel grows; the photo window and action bar stay put.
            await tester.ensureVisible(toggle);
            final toggleLabelRight = tester.getTopRight(find.text('查看說明')).dx;
            await tester.tap(toggle);
            await tester.pumpAndSettle();
            expect(tester.widget<Text>(caption).maxLines, isNull);
            expect(tester.getRect(media), mediaBefore);
            expect(tester.getRect(actions), actionBefore);
            expect(actions.hitTestable(), findsOneWidget);
            expect(tester.takeException(), isNull);
            final vertical = find.descendant(
              of: scroll,
              matching: find.byType(Scrollable),
            );
            await tester.scrollUntilVisible(
              toggle,
              300,
              scrollable: vertical,
              maxScrolls: 30,
            );
            await tester.pumpAndSettle();
            expect(find.text('收起說明').hitTestable(), findsOneWidget);
            expect(
              tester.getTopRight(find.text('收起說明')).dx,
              closeTo(toggleLabelRight, 0.01),
              reason:
                  'Expanded and collapsed labels stay on the same right edge.',
            );
            await tester.tap(toggle);
            await tester.pumpAndSettle();
            expect(tester.widget<Text>(caption).maxLines, 1);
            expect(tester.getRect(media), mediaBefore);
            expect(tester.getRect(actions), actionBefore);
            expect(tester.takeException(), isNull);
          } finally {
            semantics.dispose();
          }
        },
      );
    }
  }

  testWidgets('feed back button pops a real route exactly once', (
    tester,
  ) async {
    final observer = _PopObserver();
    final router = GoRouter(
      initialLocation: '/store/feed',
      observers: [observer],
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => const Scaffold(body: Text('root route')),
          routes: [
            GoRoute(
              path: 'store',
              builder: (_, _) => const Scaffold(body: Text('store route')),
              routes: [GoRoute(path: 'feed', builder: (_, _) => _feed)],
            ),
          ],
        ),
      ],
    );
    addTearDown(router.dispose);
    await _pumpFeed(
      tester,
      size: const Size(402, 874),
      scale: 1.4,
      router: router,
    );
    await tester.tap(find.byKey(const ValueKey('seller-post-back-button')));
    await tester.pumpAndSettle();
    expect(router.routeInformationProvider.value.uri.path, '/store');
    expect(find.text('store route'), findsOneWidget);
    expect(observer.pops, 1);
    expect(
      router.canPop(),
      isTrue,
      reason: 'The root route must remain below the store route.',
    );
    expect(tester.takeException(), isNull);
  });
}
