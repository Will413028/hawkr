import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' show SemanticsAction, Tristate;

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/social/presentation/merchant_posts_feed_screen.dart';
import 'package:hawkr_buyer/features/social/presentation/post_preview_layout.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/app_cached_image.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

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

class _SocialFeedAdapter implements HttpClientAdapter {
  _SocialFeedAdapter({this.withStoreCode = true});

  final bool withStoreCode;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    final body = switch (options.path) {
      '/public/sellers/7/posts' => [
        {
          'id': 101,
          'sellerId': 7,
          if (withStoreCode) 'storeCode': 'nanmu',
          'sellerName': '南崁王生技',
          'logoUrl': '',
          'imageKeys': ['post/101.jpg'],
          'imageUrls': ['https://example.test/post.jpg'],
          'commentCount': 3,
          'createdAt': '2026-07-05T01:22:00+08:00',
          'content':
              '中都農合 x #迷客夏 共同合作小迷豆漿，想跟博恩賀瓏喝一樣的豆漿與豆奶飲品系列嗎？'
              '快衝去迷客夏購買！全台迷客夏熱銷上市中，這是第三行以後才會看到的完整內文。',
          'product': {
            'id': 55,
            'name': '南崁王生技中都－台灣紅豆',
            'price': '195',
            'image': '',
          },
        },
        {
          'id': 102,
          'sellerId': 7,
          'sellerName': '第二篇商家',
          'logoUrl': '',
          'imageKeys': ['post/102.jpg'],
          'imageUrls': ['https://example.test/post-2.jpg'],
          'commentCount': 0,
          'createdAt': '2026-07-06T01:22:00+08:00',
          'content': '第二篇貼文',
        },
      ],
      '/public/posts/101/comments' => {
        'data': {
          'records': [
            {
              'id': 201,
              'userId': 'member-1',
              'userName': 'Cora Chou',
              'content': '第一層留言',
              'replyCount': 4,
              'likeCount': 2,
              'isLiked': true,
            },
            {
              'id': 202,
              'parentCommentId': 201,
              'userId': 'member-2',
              'userName': '範例生鮮',
              'content': '這是下方回覆內容',
              'likeCount': 1,
            },
            {
              'id': 203,
              'parentId': 202,
              'userId': 'member-3',
              'userName': '農友',
              'content': '回覆回覆仍留在同一討論串',
            },
          ],
        },
      },
      _ => {'data': <Object>[]},
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
}

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('商家連結以貼文的 storeCode 導頁，沒有 storeCode 的貼文連結停用', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = _SocialFeedAdapter();
    addTearDown(dio.close);
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, __) => const MerchantPostsFeedScreen(merchantId: '7'),
        ),
        GoRoute(
          path: '/merchant/:storeCode',
          builder: (_, state) =>
              Scaffold(body: Text('店舖 ${state.pathParameters['storeCode']}')),
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          dioProvider.overrideWithValue(dio),
          sessionProvider.overrideWith(_GuestSession.new),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pump();

    await tester.tap(find.byKey(const ValueKey('post-merchant-link-101')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(find.text('店舖 nanmu'), findsOneWidget);
    expect(find.text('店舖 7'), findsNothing);
  });

  testWidgets('貼文沒有 storeCode 時商家連結不能點', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = _SocialFeedAdapter(withStoreCode: false);
    addTearDown(dio.close);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          dioProvider.overrideWithValue(dio),
          sessionProvider.overrideWith(_GuestSession.new),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) =>
              const MaterialApp(home: MerchantPostsFeedScreen(merchantId: '7')),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    await tester.pump();

    final link = tester.widget<InkWell>(
      find.byKey(const ValueKey('post-merchant-link-101')),
    );
    expect(link.onTap, isNull);
  });

  for (final scenario in <({double width, double textScale})>[
    (width: 320, textScale: 1.4),
    (width: 390, textScale: 1),
  ]) {
    testWidgets('單卡貼文預覽在 ${scenario.width.toInt()}px 保持照片置中、'
        '商品列與互動列整體滑動', (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = Size(scenario.width, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      final semantics = tester.ensureSemantics();

      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _SocialFeedAdapter();
      addTearDown(dio.close);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            dioProvider.overrideWithValue(dio),
            sessionProvider.overrideWith(_GuestSession.new),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (context, child) => MaterialApp(
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: TextScaler.linear(scenario.textScale)),
                child: child!,
              ),
              home: const MerchantPostsFeedScreen(merchantId: '7'),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 200));
      await tester.pump();

      final media = find.byKey(const ValueKey('post-media-101'));
      expect(media, findsOneWidget);
      expect(
        tester
            .widget<ColoredBox>(
              find.byKey(const ValueKey('post-preview-surface')),
            )
            .color,
        AppColors.bgPrimary,
      );
      final preview = find.byKey(const ValueKey('post-preview-window-101'));
      expect(preview, findsOneWidget);
      final previewRect = tester.getRect(preview);
      expect(previewRect.left, closeTo(8, 0.01));
      expect(previewRect.right, closeTo(scenario.width - 8, 0.01));
      expect(previewRect.top, closeTo(8, 0.01));
      expect(
        find.byKey(const ValueKey('related-product-chip-101')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('post-merchant-link-101')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('post-static-backdrop')),
        findsOneWidget,
      );
      expect(
        find.byKey(const ValueKey('post-card-motion-101')),
        findsOneWidget,
      );
      final postImage = tester.widget<AppCachedImage>(
        find.byKey(const ValueKey('post-image-content-101')),
      );
      expect(postImage.scene, ImageScene.fullscreen);
      expect(postImage.fit, BoxFit.contain);
      expect(
        tester.getCenter(find.byKey(const ValueKey('post-centered-media-101'))),
        tester.getCenter(media),
      );
      expect(
        tester.getSize(find.byKey(const ValueKey('post-back-button'))),
        const Size.square(48),
      );
      expect(
        find.descendant(
          of: find.byKey(const ValueKey('post-back-button')),
          matching: find.byIcon(AppIcons.close),
        ),
        findsOneWidget,
      );
      final cardBody = find.byKey(const ValueKey('post-card-body-surface-101'));
      expect(cardBody, findsOneWidget);
      expect(tester.widget<ColoredBox>(cardBody).color, AppColors.bgElevated);
      expect(
        find.descendant(of: cardBody, matching: find.byType(BackdropFilter)),
        findsNothing,
        reason: '內文、商品與操作列應共用不透明卡面',
      );
      expect(
        find.descendant(
          of: cardBody,
          matching: find.byWidgetPredicate(_hasSoftTopDivider),
        ),
        findsNothing,
        reason: '導購與互動列不應被切成獨立 footer',
      );

      final actionCenters = [
        const ValueKey('post-like-action-101'),
        const ValueKey('post-comment-action-101'),
        const ValueKey('post-share-action-101'),
      ].map((key) => tester.getCenter(find.byKey(key))).toList();
      expect(actionCenters[0].dy, closeTo(actionCenters[1].dy, 0.01));
      expect(actionCenters[1].dy, closeTo(actionCenters[2].dy, 0.01));
      expect(find.text('第一層留言'), findsNothing);

      final caption = find.byKey(const ValueKey('post-caption-text'));
      expect(tester.widget<Text>(caption).maxLines, 1);
      expect(find.text('查看說明'), findsOneWidget);
      final panel = find.byKey(const ValueKey('post-glass-panel-101'));
      final collapsedPanelRect = tester.getRect(panel);
      final actionBar = find.byKey(const ValueKey('post-action-bar-101'));
      final actionBarRect = tester.getRect(actionBar);
      expect(
        actionBarRect.left,
        closeTo(PostPreviewLayout.actionBarHorizontalInset, 0.01),
      );
      expect(
        actionBarRect.right,
        closeTo(
          scenario.width - PostPreviewLayout.actionBarHorizontalInset,
          0.01,
        ),
      );
      expect(
        actionBarRect.bottom,
        closeTo(844 - PostPreviewLayout.actionBarBottomInset, 0.01),
      );
      expect(actionBarRect.height, PostPreviewLayout.actionBarHeight);
      final productChip = find.byKey(
        const ValueKey('related-product-chip-101'),
      );
      expect(
        previewRect.bottom,
        closeTo(actionBarRect.bottom, 0.01),
        reason: '互動列收進預覽卡底緣，不再是卡外懸浮 footer',
      );
      expect(
        previewRect.bottom - collapsedPanelRect.bottom,
        closeTo(
          PostPreviewLayout.actionBarHeight +
              PostPreviewLayout.panelBottomInset,
          0.01,
        ),
      );
      expect(collapsedPanelRect.left, closeTo(previewRect.left, 0.01));
      expect(collapsedPanelRect.right, closeTo(previewRect.right, 0.01));
      expect(collapsedPanelRect.bottom, closeTo(actionBarRect.top, 0.01));
      expect(find.ancestor(of: cardBody, matching: preview), findsOneWidget);
      expect(find.ancestor(of: panel, matching: cardBody), findsOneWidget);
      expect(find.ancestor(of: actionBar, matching: cardBody), findsOneWidget);
      expect(
        tester.getTopLeft(productChip).dy,
        lessThan(tester.getTopLeft(caption).dy),
      );
      expect(
        tester
            .widget<AnimatedOpacity>(
              find.byKey(const ValueKey('post-photo-scrim-101')),
            )
            .opacity,
        0,
      );

      await tester.tap(find.byKey(const ValueKey('post-caption-toggle')));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(tester.widget<Text>(caption).maxLines, isNull);
      expect(find.text('收起說明'), findsOneWidget);
      expect(tester.getRect(panel).top, lessThan(collapsedPanelRect.top));
      expect(tester.getRect(actionBar), actionBarRect);
      expect(
        tester
            .widget<AnimatedOpacity>(
              find.byKey(const ValueKey('post-photo-scrim-101')),
            )
            .opacity,
        1,
      );

      await tester.tapAt(Offset(previewRect.center.dx, previewRect.top + 140));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));
      expect(tester.widget<Text>(caption).maxLines, 1);
      expect(tester.getRect(actionBar), actionBarRect);
      expect(
        tester
            .widget<AnimatedOpacity>(
              find.byKey(const ValueKey('post-photo-scrim-101')),
            )
            .opacity,
        0,
      );

      if (scenario.width == 390) {
        await tester.tap(
          find.byKey(const ValueKey('post-comment-action-101')).hitTestable(),
        );
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 500));
        await tester.pump();

        expect(find.byKey(const ValueKey('comments-panel')), findsOneWidget);
        expect(find.text('第一層留言'), findsOneWidget);
        expect(find.text('這是下方回覆內容'), findsOneWidget);
        expect(find.text('回覆回覆仍留在同一討論串'), findsOneWidget);

        final close = find.byKey(const ValueKey('comments-panel-close'));
        final rootReply = find.byKey(const ValueKey('comment-reply-201'));
        final replyReply = find.byKey(const ValueKey('comment-reply-202'));
        final rootLike = find.byKey(const ValueKey('comment-like-201'));
        final replyLike = find.byKey(const ValueKey('comment-like-202'));
        final moreReplies = find.byKey(
          const ValueKey('comment-more-replies-201'),
        );
        final send = find.byKey(const ValueKey('send-comment-action'));
        for (final target in <Finder>[
          close,
          rootReply,
          replyReply,
          rootLike,
          replyLike,
          moreReplies,
          send,
        ]) {
          expect(target, findsOneWidget);
          expect(tester.getSize(target).width, greaterThanOrEqualTo(48));
          expect(tester.getSize(target).height, greaterThanOrEqualTo(48));
        }

        final rootLikeSemantics = tester
            .getSemantics(rootLike)
            .getSemanticsData();
        expect(rootLikeSemantics.flagsCollection.isToggled, Tristate.isTrue);
        final replyLikeSemantics = tester
            .getSemantics(replyLike)
            .getSemanticsData();
        expect(replyLikeSemantics.flagsCollection.isToggled, Tristate.isFalse);

        var sendSemantics = tester.getSemantics(send).getSemanticsData();
        expect(sendSemantics.flagsCollection.isEnabled, Tristate.isFalse);
        expect(sendSemantics.hasAction(SemanticsAction.tap), isFalse);

        await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
        await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
        await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

        await tester.tap(rootReply);
        await tester.pump();
        expect(find.text('回覆 @Cora Chou'), findsOneWidget);
        final cancelReply = find.byKey(const ValueKey('cancel-comment-reply'));
        expect(cancelReply, findsOneWidget);
        expect(tester.getSize(cancelReply), const Size.square(48));
        expect(find.bySemanticsLabel('送出回覆'), findsOneWidget);

        await tester.tap(cancelReply);
        await tester.pump();
        expect(find.text('回覆 @Cora Chou'), findsNothing);
        expect(find.bySemanticsLabel('送出留言'), findsOneWidget);

        await tester.enterText(
          find.byKey(const ValueKey('comments-panel-input')),
          '測試留言',
        );
        await tester.pump();
        sendSemantics = tester.getSemantics(send).getSemanticsData();
        expect(sendSemantics.flagsCollection.isEnabled, Tristate.isTrue);
        expect(sendSemantics.hasAction(SemanticsAction.tap), isTrue);

        Navigator.of(
          tester.element(find.byKey(const ValueKey('comments-panel'))),
        ).pop();
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));

        final pageView = find.byKey(const ValueKey('post-card-page-view'));
        final backdrop = find.byKey(const ValueKey('post-static-backdrop'));
        final backdropRect = tester.getRect(backdrop);
        final backdropWidget = tester.widget(backdrop);
        final firstCardLeft = tester.getTopLeft(preview).dx;
        final actionLeftBeforeDrag = tester.getTopLeft(actionBar).dx;
        final gesture = await tester.startGesture(tester.getCenter(pageView));
        await gesture.moveBy(const Offset(-140, 0));
        await tester.pump();

        expect(tester.getRect(backdrop), backdropRect);
        expect(identical(tester.widget(backdrop), backdropWidget), isTrue);
        expect(tester.getTopLeft(preview).dx, lessThan(firstCardLeft));
        expect(
          tester.getTopLeft(actionBar).dx,
          lessThan(actionLeftBeforeDrag),
          reason: '互動列必須跟著所屬貼文卡滑動，不能停在螢幕上',
        );
        expect(
          tester
              .widget<Opacity>(
                find.byKey(const ValueKey('post-card-depth-101')),
              )
              .opacity,
          lessThan(1),
        );

        await gesture.moveBy(const Offset(-180, 0));
        await gesture.up();
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));
        expect(
          find.byKey(const ValueKey('post-preview-window-102')),
          findsOneWidget,
        );
        expect(
          tester.getRect(find.byKey(const ValueKey('post-static-backdrop'))),
          backdropRect,
        );
        final secondPreview = find.byKey(
          const ValueKey('post-preview-window-102'),
        );
        final secondActionBar = find.byKey(
          const ValueKey('post-action-bar-102'),
        );
        expect(secondActionBar, findsOneWidget);
        expect(
          find.ancestor(of: secondActionBar, matching: secondPreview),
          findsOneWidget,
        );
        expect(find.text('第二篇商家'), findsOneWidget);
      }
      expect(tester.takeException(), isNull);
      semantics.dispose();
    });
  }
}
