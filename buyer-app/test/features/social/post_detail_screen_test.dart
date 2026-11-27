import 'dart:io';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/social/presentation/post_detail_screen.dart';
import 'package:hawkr_buyer/features/social/presentation/post_preview_layout.dart';
import 'package:hawkr_buyer/shared/widgets/app_cached_image.dart';
import 'package:fixnum/fixnum.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/image.pb.dart' as common;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_seller_repository.dart';
import 'social_fakes.dart';

bool _hasSoftTopDivider(Widget widget) {
  if (widget is! DecoratedBox || widget.decoration is! BoxDecoration) {
    return false;
  }
  final border = (widget.decoration as BoxDecoration).border;
  return border is Border &&
      border.top.style != BorderStyle.none &&
      border.top.color == AppColors.borderSoft &&
      border.left.style == BorderStyle.none &&
      border.right.style == BorderStyle.none &&
      border.bottom.style == BorderStyle.none;
}

FakeSocialPublic _detail() => FakeSocialPublic(
  post: pb.Post(
    id: '101',
    store: StoreCard(id: '7', name: '南畝王生技'),
    likeCount: Int64(868),
    commentCount: Int64(74),
    shareCount: Int64(31),
    body:
        '中都農合 x #迷客夏 共同合作小迷豆漿，想跟博恩賀瓏喝一樣的豆漿與豆奶飲品系列嗎？'
        '快衝去迷客夏購買！全台迷客夏熱銷上市中，這段文字需要超過兩行才會完整顯示。',
    product: pb.PostProduct(
      id: '55',
      name: '南畝王生技中都－台灣紅豆',
      minPrice: Int64(195),
      image: common.Image(
        uriTemplate:
            'https://test-media.example.com/i/{preset}/public/product/red-bean.webp',
      ),
    ),
  ),
);

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('貼文詳細頁在 320px 與 1.4x 字級下維持單一不透明卡面', (tester) async {
    const screenWidth = 320.0;
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(screenWidth, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final public = _detail();

    final router = GoRouter(
      initialLocation: '/social/post/101',
      routes: [
        GoRoute(
          path: '/social/post/:postId',
          builder: (_, state) => PostDetailScreen(
            postId: int.parse(state.pathParameters['postId']!),
          ),
        ),
        GoRoute(
          path: '/merchant/:storeCode',
          builder: (_, state) =>
              Scaffold(body: Text('店舖 ${state.pathParameters['storeCode']}')),
        ),
        GoRoute(
          path: '/marketplace',
          builder: (_, __) => const Scaffold(body: Text('市集')),
        ),
      ],
    );
    addTearDown(router.dispose);

    await HttpOverrides.runZoned(
      () => tester.pumpWidget(
        ProviderScope(
          overrides: [
            socialPublicClientProvider.overrideWithValue(public.client()),
            sellerRepositoryProvider.overrideWithValue(
              fakeSellerRepository(
                cards: [
                  StoreCard(
                    id: '7',
                    storeCode: 'nanmu',
                    name: '南畝王生技',
                    status: StoreCardStatus.STORE_CARD_STATUS_OPEN,
                  ),
                ],
              ),
            ),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => MaterialApp.router(
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(1.4)),
                child: child!,
              ),
              routerConfig: router,
            ),
          ),
        ),
      ),
      createHttpClient: (_) {
        throw StateError('External network is disabled in widget tests');
      },
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    await tester.pump();

    expect(find.byKey(const ValueKey('post-detail-surface')), findsOneWidget);
    expect(
      find.byKey(const ValueKey('post-merchant-avatar-101')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('post-merchant-link-101')),
      findsOneWidget,
    );
    expect(
      find.byKey(const ValueKey('related-product-chip-101')),
      findsOneWidget,
    );
    final relatedProductImage = tester.widget<AppCachedImage>(
      find.descendant(
        of: find.byKey(const ValueKey('related-product-chip-101')),
        matching: find.byType(AppCachedImage),
      ),
    );
    expect(
      relatedProductImage.imageUrl,
      'https://test-media.example.com/i/card/public/product/red-bean.webp',
    );
    expect(find.text('進入店舖'), findsOneWidget);
    expect(public.recordViewCalls, 1);
    final surface = tester.widget<Scaffold>(
      find.byKey(const ValueKey('post-detail-surface')),
    );
    expect(surface.backgroundColor, Colors.transparent);
    final cardBody = find.byKey(
      const ValueKey('post-detail-card-body-surface'),
    );
    expect(cardBody, findsOneWidget);
    expect(tester.widget<ColoredBox>(cardBody).color, AppColors.bgElevated);
    expect(
      find.descendant(of: cardBody, matching: find.byType(BackdropFilter)),
      findsNothing,
      reason: '內文、導購列與操作列不再是多層毛玻璃 footer',
    );
    expect(
      find.descendant(
        of: cardBody,
        matching: find.byWidgetPredicate(_hasSoftTopDivider),
      ),
      findsNothing,
      reason: '導購與互動列不應被切成獨立 footer',
    );

    final caption = find.byKey(const ValueKey('post-detail-caption-text'));
    final merchantLink = find.byKey(const ValueKey('post-merchant-link-101'));
    final productChip = find.byKey(const ValueKey('related-product-chip-101'));
    final glassPanel = find.byKey(const ValueKey('post-detail-glass-panel'));
    final actionBar = find.byKey(const ValueKey('post-detail-action-bar'));
    final photoScrim = find.byKey(const ValueKey('post-detail-photo-scrim'));
    final previewWindow = find.byKey(
      const ValueKey('post-detail-preview-window'),
    );
    final postScroll = find.byKey(const ValueKey('post-detail-scroll'));
    final mediaSurface = find.byKey(const ValueKey('post-detail-media-101'));
    final centeredMedia = find.byKey(
      const ValueKey('post-detail-centered-media-101'),
    );
    expect(tester.widget<Text>(caption).maxLines, 1);
    expect(
      tester.getTopLeft(merchantLink).dy,
      lessThan(tester.getTopLeft(caption).dy),
    );
    expect(
      tester.getTopLeft(productChip).dy,
      greaterThan(tester.getTopLeft(merchantLink).dy),
    );
    expect(
      tester.getTopLeft(productChip).dy,
      lessThan(tester.getTopLeft(caption).dy),
    );
    expect(find.text('查看說明'), findsOneWidget);
    expect(
      tester
          .getSize(find.byKey(const ValueKey('post-detail-caption-toggle')))
          .height,
      greaterThanOrEqualTo(48),
    );
    expect(tester.widget<AnimatedOpacity>(photoScrim).opacity, 0);

    final collapsedPanelTop = tester.getTopLeft(glassPanel).dy;
    final actionBarRect = tester.getRect(actionBar);
    expect(
      actionBarRect.left,
      closeTo(PostPreviewLayout.actionBarHorizontalInset, 0.01),
    );
    expect(
      actionBarRect.right,
      closeTo(screenWidth - PostPreviewLayout.actionBarHorizontalInset, 0.01),
    );
    expect(
      actionBarRect.bottom,
      closeTo(844 - PostPreviewLayout.actionBarBottomInset, 0.01),
    );
    expect(actionBarRect.height, PostPreviewLayout.actionBarHeight);
    final previewRect = tester.getRect(previewWindow);
    expect(previewRect.left, greaterThan(0));
    expect(previewRect.top, greaterThan(0));
    expect(previewRect.right, lessThan(screenWidth));
    expect(
      previewRect.bottom,
      closeTo(actionBarRect.bottom, 0.01),
      reason: '詳情互動列應在同一張預覽卡的底緣',
    );
    expect(
      previewRect.bottom - tester.getRect(glassPanel).bottom,
      closeTo(
        PostPreviewLayout.actionBarHeight + PostPreviewLayout.panelBottomInset,
        0.01,
      ),
    );
    final glassPanelRect = tester.getRect(glassPanel);
    expect(glassPanelRect.left, closeTo(previewRect.left, 0.01));
    expect(glassPanelRect.right, closeTo(previewRect.right, 0.01));
    expect(glassPanelRect.bottom, closeTo(actionBarRect.top, 0.01));
    expect(
      find.ancestor(of: cardBody, matching: previewWindow),
      findsOneWidget,
    );
    expect(find.ancestor(of: glassPanel, matching: cardBody), findsOneWidget);
    expect(find.ancestor(of: actionBar, matching: cardBody), findsOneWidget);
    expect(find.ancestor(of: cardBody, matching: postScroll), findsOneWidget);
    expect(
      find.ancestor(of: mediaSurface, matching: postScroll),
      findsOneWidget,
    );
    expect(
      (tester.getCenter(centeredMedia) - tester.getCenter(mediaSurface))
          .distance,
      lessThan(0.1),
    );

    await tester.tap(find.byKey(const ValueKey('post-detail-caption-toggle')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(tester.widget<Text>(caption).maxLines, isNull);
    expect(find.text('收起說明'), findsOneWidget);
    expect(tester.widget<AnimatedOpacity>(photoScrim).opacity, 1);
    expect(tester.getTopLeft(glassPanel).dy, lessThan(collapsedPanelTop));
    expect(tester.getRect(actionBar), actionBarRect);
    expect(
      tester.getSize(find.byKey(const ValueKey('post-detail-like'))).height,
      greaterThanOrEqualTo(48),
    );
    expect(tester.takeException(), isNull);

    await tester.tapAt(Offset(previewRect.center.dx, previewRect.top + 140));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));
    expect(tester.widget<Text>(caption).maxLines, 1);
    expect(tester.widget<AnimatedOpacity>(photoScrim).opacity, 0);
    expect(tester.getRect(actionBar), actionBarRect);

    await tester.tap(merchantLink);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(find.text('店舖 nanmu'), findsOneWidget);
    expect(find.text('店舖 7'), findsNothing);
  });
}
