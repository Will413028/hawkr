import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/features/social/data/social_content_repository.dart';
import 'package:hawkr_buyer/features/social/presentation/post_detail_screen.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_seller_repository.dart';
import 'social_fakes.dart';

Future<void> _pumpDetail(WidgetTester tester, FakeSocialPublic public) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final router = GoRouter(
    initialLocation: '/social/post/101',
    routes: [
      GoRoute(
        path: '/social/post/:postId',
        builder: (_, state) => PostDetailScreen(
          postId: int.parse(state.pathParameters['postId']!),
        ),
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        socialPublicClientProvider.overrideWithValue(public.client()),
        sellerRepositoryProvider.overrideWithValue(fakeSellerRepository()),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp.router(routerConfig: router),
      ),
    ),
  );
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 100));
  await tester.pump();
}

ConnectException _suspended() =>
    ConnectException(Code.permissionDenied, 'suspended');

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('NOT_FOUND 顯示找不到，不是網路錯誤，也不記瀏覽', (tester) async {
    final public = FakeSocialPublic(
      getPostError: ConnectException(Code.notFound, 'gone'),
    );
    await _pumpDetail(tester, public);

    expect(find.text('找不到這則貼文'), findsOneWidget);
    expect(find.text('重試'), findsNothing);
    expect(public.recordViewCalls, 0);
  });

  testWidgets('找不到只試一次，不自動重試', (tester) async {
    final public = FakeSocialPublic(
      getPostError: ConnectException(Code.notFound, 'gone'),
    );
    await _pumpDetail(tester, public);
    expect(public.getPostCalls, 1);

    await tester.pump(const Duration(seconds: 10));
    expect(public.getPostCalls, 1);
    expect(find.text('找不到這則貼文'), findsOneWidget);

    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('暫時性的失敗會自動重試（對照組）', (tester) async {
    final public = FakeSocialPublic(
      getPostError: ConnectException(Code.unavailable, 'down'),
    );
    await _pumpDetail(tester, public);
    final first = public.getPostCalls;

    await tester.pump(const Duration(seconds: 10));
    expect(public.getPostCalls, greaterThan(first));

    await tester.pumpWidget(const SizedBox.shrink());
  });

  testWidgets('打開記一次瀏覽；記瀏覽失敗（含停權）不影響畫面', (tester) async {
    final public = FakeSocialPublic(
      post: fakePost(),
      recordViewError: _suspended(),
    );
    await _pumpDetail(tester, public);

    expect(public.recordViewCalls, 1);
    expect(find.text('貼文內文'), findsWidgets);
    expect(tester.takeException(), isNull);
  });

  test('recordPostView 吞掉任何失敗，含 ACCOUNT_SUSPENDED', () async {
    for (final error in [
      _suspended(),
      ConnectException(Code.unavailable, 'down'),
    ]) {
      final public = FakeSocialPublic(recordViewError: error);
      final repo = SocialContentRepository(
        public.client(),
        FakeSocialApp().client(),
      );

      await repo.recordPostView(101);

      expect(public.recordViewCalls, 1);
    }
  });

  test('伺服器失敗不被當成找不到', () async {
    final public = FakeSocialPublic(
      getPostError: ConnectException(Code.unavailable, 'down'),
    );
    final repo = SocialContentRepository(
      public.client(),
      FakeSocialApp().client(),
    );

    await expectLater(repo.getPost(101), throwsA(isA<ConnectException>()));
  });

  test('NOT_FOUND 轉成 NotFoundException', () async {
    final repo = SocialContentRepository(
      FakeSocialPublic(
        getPostError: ConnectException(Code.notFound, 'gone'),
      ).client(),
      FakeSocialApp().client(),
    );
    await expectLater(repo.getPost(101), throwsA(isA<NotFoundException>()));
  });

  test('讚一則已刪除的貼文：NOT_FOUND 轉成 NotFoundException', () async {
    final repo = SocialContentRepository(
      FakeSocialPublic().client(),
      FakeSocialApp(
        likeError: ConnectException(Code.notFound, 'gone'),
      ).client(),
    );
    await expectLater(
      repo.setPostLiked(101, liked: true),
      throwsA(isA<NotFoundException>()),
    );
    await expectLater(
      repo.setPostLiked(101, liked: false),
      throwsA(isA<NotFoundException>()),
    );
  });

  Future<void> pullToRefresh(WidgetTester tester) async {
    await tester.fling(
      find.byType(CustomScrollView).first,
      const Offset(0, 400),
      1000,
    );
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    await tester.pump(const Duration(seconds: 1));
  }

  testWidgets('F1 已顯示的貼文下拉重讀得到找不到：改顯示找不到，不留舊貼文', (tester) async {
    final public = FakeSocialPublic(post: fakePost());
    await _pumpDetail(tester, public);
    expect(find.text('貼文內文'), findsWidgets);

    public.getPostError = ConnectException(Code.notFound, 'gone');
    await pullToRefresh(tester);

    expect(find.text('找不到這則貼文'), findsOneWidget);
    expect(find.text('貼文內文'), findsNothing);
    expect(find.text('載入失敗，請稍後再試'), findsNothing);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('F8 下拉重讀失敗（非找不到）：保留舊貼文並顯示失敗提示', (tester) async {
    final public = FakeSocialPublic(post: fakePost());
    await _pumpDetail(tester, public);

    public.getPostError = ConnectException(Code.unavailable, 'down');
    await pullToRefresh(tester);

    expect(find.text('載入失敗，請稍後再試'), findsOneWidget);
    expect(find.text('貼文內文'), findsWidgets);

    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pump(const Duration(seconds: 3));
  });
}
