import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:fixnum/fixnum.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_home_tab.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../social/social_fakes.dart';

late ProviderContainer _container;

SwitchableSession get _session =>
    _container.read(sessionProvider.notifier) as SwitchableSession;

Future<void> _pump(
  WidgetTester tester, {
  required FakeSocialPublic public,
  required FakeSocialApp app,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(app.client()),
      sessionProvider.overrideWith(SwitchableSession.new),
    ],
  );
  addTearDown(container.dispose);
  _container = container;
  await container.read(sessionProvider.future);
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
                  body: Consumer(
                    builder: (context, ref, _) =>
                        MerchantHomeTab(merchantId: '7', merchantName: '好店'),
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

Future<void> _openPreview(WidgetTester tester) async {
  await tester.longPress(find.byType(InkWell).first);
  await tester.pumpAndSettle();
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('R2-2 換觀看者前發出的貼文讀取晚回來：丟棄，不蓋掉新的列表', (tester) async {
    final oldPosts = Completer<pb.ListPostsResponse>();
    var calls = 0;
    final public = FakeSocialPublic(
      postsResponder: (_) {
        if (++calls == 1) return oldPosts.future;
        return Future.value(
          pb.ListPostsResponse(
            posts: [pb.Post(id: '202', body: '新觀看者的貼文')],
          ),
        );
      },
    );
    await _pump(tester, public: public, app: FakeSocialApp());

    _session.signIn();
    await tester.pumpAndSettle();
    oldPosts.complete(
      pb.ListPostsResponse(
        posts: [pb.Post(id: '101', body: '舊觀看者的貼文')],
      ),
    );
    await tester.pumpAndSettle();
    await _openPreview(tester);

    expect(find.textContaining('新觀看者的貼文', findRichText: true), findsOneWidget);
    expect(find.textContaining('舊觀看者的貼文', findRichText: true), findsNothing);
  });

  testWidgets('R2-1 預覽開著時換觀看者：預覽關閉', (tester) async {
    final public = FakeSocialPublic(
      posts: [pb.Post(id: '101', body: '貼文')],
    );
    await _pump(tester, public: public, app: FakeSocialApp());
    await _openPreview(tester);
    expect(find.byIcon(AppIcons.favoriteBorder), findsOneWidget);

    _session.signIn();
    await tester.pumpAndSettle();

    expect(find.byIcon(AppIcons.favoriteBorder), findsNothing);
    expect(find.byIcon(AppIcons.favorite), findsNothing);
  });

  testWidgets('R2-1 按讚途中換觀看者後失敗：不把舊狀態還原到重讀後的貼文', (tester) async {
    final app = FakeSocialApp()..likeGate = Completer<void>();
    var calls = 0;
    final public = FakeSocialPublic(
      postsResponder: (_) async => ++calls <= 2
          ? pb.ListPostsResponse(
              posts: [pb.Post(id: '101', body: '貼文')],
            )
          : pb.ListPostsResponse(
              posts: [
                pb.Post(
                  id: '101',
                  body: '貼文',
                  viewerLiked: true,
                  likeCount: Int64(5),
                ),
              ],
            ),
    );
    await _pump(tester, public: public, app: app);
    _session.signIn();
    await tester.pumpAndSettle();
    await _openPreview(tester);

    await tester.tap(find.byIcon(AppIcons.favoriteBorder));
    await tester.pump();
    expect(app.postLikes, hasLength(1));

    _session.switchTo('account-3002');
    await tester.pumpAndSettle();
    app.likeGate!.completeError(ConnectException(Code.unavailable, 'down'));
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);

    await _openPreview(tester);
    expect(find.byIcon(AppIcons.favorite), findsOneWidget);
    expect(find.text('5'), findsOneWidget);
  });
}
