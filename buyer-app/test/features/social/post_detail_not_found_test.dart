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
}
