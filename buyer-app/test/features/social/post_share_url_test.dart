import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/features/social/presentation/merchant_posts_feed_screen.dart';
import 'package:hawkr_buyer/features/social/presentation/post_detail_screen.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_seller_repository.dart';
import 'social_fakes.dart';

const _shareUrl = 'https://hawkr.test/sellers/shop/posts/101';

/// 任何舊 REST 請求都記下來；分享不該再打 `getShareUrl`。
class _RecordingAdapter implements HttpClientAdapter {
  final List<RequestOptions> requests = [];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    requests.add(options);
    return ResponseBody.fromString(
      '{"code":0,"data":{}}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

pb.Post _post({String url = _shareUrl}) => fakePost()..url = url;

Future<_RecordingAdapter> _pump(
  WidgetTester tester,
  Widget home,
  pb.Post post,
) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final adapter = _RecordingAdapter();
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..httpClientAdapter = adapter;
  addTearDown(dio.close);
  final public = FakeSocialPublic(post: post, posts: [post]);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(dio),
        socialPublicClientProvider.overrideWithValue(public.client()),
        connectAuthTransportProvider.overrideWith(
          (_) => FakeTransportBuilder().build(),
        ),
        sellerRepositoryProvider.overrideWithValue(fakeSellerRepository()),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp.router(
          theme: AppTheme.light,
          routerConfig: GoRouter(
            routes: [GoRoute(path: '/', builder: (_, _) => home)],
          ),
        ),
      ),
    ),
  );
  for (var i = 0; i < 10; i++) {
    await tester.pump(const Duration(milliseconds: 100));
  }
  return adapter;
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('貼文詳情的分享用 Post.url，不打 getShareUrl', (tester) async {
    final adapter = await _pump(
      tester,
      const PostDetailScreen(postId: 101),
      _post(),
    );

    await tester.tap(find.byKey(const ValueKey('post-detail-share')));
    await tester.pumpAndSettle();

    expect(find.text(_shareUrl), findsOneWidget);
    expect(
      adapter.requests.where((r) => r.path.contains('share')),
      isEmpty,
      reason: '分享網址由伺服器放在 Post.url，client 不再另外查',
    );
    expect(adapter.requests, isEmpty);
  });

  testWidgets('店家貼文列表的分享用 Post.url，不打 getShareUrl', (tester) async {
    final adapter = await _pump(
      tester,
      const MerchantPostsFeedScreen(merchantId: '7'),
      _post(),
    );

    final share = find.byKey(const ValueKey('post-share-action-101'));
    await tester.ensureVisible(share);
    await tester.pumpAndSettle();
    await tester.tap(share);
    await tester.pumpAndSettle();

    expect(find.text(_shareUrl), findsOneWidget);
    expect(adapter.requests, isEmpty);
  });

  testWidgets('Post.url 為空時不開分享面板，也不自己拼網址', (tester) async {
    await _pump(tester, const PostDetailScreen(postId: 101), _post(url: ''));

    await tester.tap(find.byKey(const ValueKey('post-detail-share')));
    await tester.pumpAndSettle();

    expect(find.text('分享'), findsNothing);
  });
}
