import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_home_tab.dart';
import 'package:hawkr_buyer/features/social/presentation/post_entity_store.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../social/social_fakes.dart';

late ProviderContainer _container;
late ValueNotifier<String> _merchantId;

SwitchableSession get _session =>
    _container.read(sessionProvider.notifier) as SwitchableSession;

Future<void> _pump(
  WidgetTester tester, {
  required FakeSocialPublic public,
  required FakeSocialApp app,
  bool signedIn = false,
  Size size = const Size(390, 844),
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final container = ProviderContainer(
    retry: (_, _) => null,
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(app.client()),
      sessionProvider.overrideWith(SwitchableSession.new),
    ],
  );
  addTearDown(container.dispose);
  _container = container;
  _merchantId = ValueNotifier('7');
  addTearDown(_merchantId.dispose);
  await container.read(sessionProvider.future);
  if (signedIn) _session.signIn();
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp.router(
          theme: AppTheme.light,
          routerConfig: GoRouter(
            routes: [
              GoRoute(
                path: '/',
                builder: (_, _) => Scaffold(
                  body: ValueListenableBuilder<String>(
                    valueListenable: _merchantId,
                    builder: (_, id, _) =>
                        MerchantHomeTab(merchantId: id, merchantName: '好店'),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Future<void> _openPreview(WidgetTester tester, {int index = 0}) async {
  await tester.longPress(find.byType(InkWell).at(index));
  await tester.pumpAndSettle();
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('詳情（實體）按讚：開著的預覽卡跟著變，列表不重讀', (tester) async {
    final public = FakeSocialPublic(
      posts: [pb.Post(id: '101', body: '貼文')],
    );
    await _pump(tester, public: public, app: FakeSocialApp(), signedIn: true);
    await _openPreview(tester);
    expect(find.byIcon(AppIcons.favoriteBorder), findsOneWidget);

    await _container.read(postEntityStoreProvider.notifier).toggleLike(101);
    await tester.pumpAndSettle();

    expect(find.byIcon(AppIcons.favorite), findsOneWidget);
    expect(public.listPostsCalls, 1);
  });

  testWidgets('R3-1 按讚成功途中換觀看者：預覽關閉，重開後是新觀看者的資料，沒有被翻成取消讚', (tester) async {
    final app = FakeSocialApp()..likeGate = Completer<void>();
    var calls = 0;
    final public = FakeSocialPublic(
      postsResponder: (_) async => ++calls <= 1
          ? pb.ListPostsResponse(
              posts: [pb.Post(id: '101', body: '貼文')],
            )
          : pb.ListPostsResponse(
              posts: [pb.Post(id: '101', body: '貼文', viewerLiked: true)],
            ),
    );
    await _pump(tester, public: public, app: app, signedIn: true);
    await _openPreview(tester);
    await tester.tap(find.byIcon(AppIcons.favoriteBorder));
    await tester.pump();
    expect(app.postLikes, hasLength(1));

    _session.switchTo('account-3002');
    await tester.pumpAndSettle();
    app.likeGate!.complete();
    await tester.pumpAndSettle();
    await _openPreview(tester);

    expect(find.byIcon(AppIcons.favorite), findsOneWidget);
    expect(app.postUnlikes, isEmpty);
  });

  testWidgets('R3-6 切換店家：預覽卡關閉', (tester) async {
    final public = FakeSocialPublic(
      posts: [pb.Post(id: '101', body: '貼文')],
    );
    await _pump(tester, public: public, app: FakeSocialApp());
    await _openPreview(tester);
    expect(find.byIcon(AppIcons.favoriteBorder), findsOneWidget);

    _merchantId.value = '8';
    await tester.pumpAndSettle();

    expect(find.byIcon(AppIcons.favoriteBorder), findsNothing);
    expect(public.listPostsRequests.last.storeId, '8');
  });

  testWidgets('R3-7 預覽的貼文從列表消失：預覽關閉，沒有留下擋點擊的 barrier', (tester) async {
    final public = FakeSocialPublic(
      posts: [
        pb.Post(id: '101', body: '貼文'),
        pb.Post(id: '102', body: '另一則'),
      ],
    );
    await _pump(tester, public: public, app: FakeSocialApp());
    final barriersBefore = find.byType(ModalBarrier).evaluate().length;
    await _openPreview(tester);
    expect(
      find.byType(ModalBarrier).evaluate().length,
      greaterThan(barriersBefore),
    );

    _container.read(postEntityStoreProvider.notifier).remove(101);
    await tester.pumpAndSettle();

    expect(find.byIcon(AppIcons.favoriteBorder), findsNothing);
    expect(find.byType(ModalBarrier).evaluate().length, barriersBefore);
    expect(find.byType(InkWell), findsOneWidget);
  });

  testWidgets('F2 換觀看者後列表重讀失敗：顯示錯誤與重新載入，不是灰格；重試後回來', (tester) async {
    var fail = false;
    final public = FakeSocialPublic(
      postsResponder: (_) async {
        if (fail) throw ConnectException(Code.unavailable, 'down');
        return pb.ListPostsResponse(
          posts: [pb.Post(id: '101', body: '貼文')],
        );
      },
    );
    await _pump(tester, public: public, app: FakeSocialApp());
    expect(find.byType(InkWell), findsOneWidget);

    fail = true;
    _session.signIn();
    await tester.pump();
    await tester.pump(const Duration(seconds: 10));
    await tester.pumpAndSettle();

    expect(find.text('載入失敗'), findsOneWidget);
    expect(find.text('重新載入'), findsOneWidget);
    expect(find.byType(SliverGrid), findsNothing);

    fail = false;
    await tester.tap(find.text('重新載入'));
    await tester.pumpAndSettle();

    expect(find.text('載入失敗'), findsNothing);
    expect(find.byType(InkWell), findsOneWidget);
  });
}
