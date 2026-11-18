import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/seller/presentation/widgets/merchant_home_tab.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../social/social_fakes.dart';

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

class _ActiveSession extends Session {
  @override
  Future<SessionModel?> build() async =>
      const SessionModel(userId: 'account-3001', nickname: 'Iris');
}

Future<void> _pump(
  WidgetTester tester, {
  required FakeSocialApp app,
  bool guest = false,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final public = FakeSocialPublic(
    posts: [pb.Post(id: '101', body: '貼文')],
  );
  final container = ProviderContainer(
    overrides: [
      socialPublicClientProvider.overrideWithValue(public.client()),
      socialAppClientProvider.overrideWithValue(app.client()),
      sessionProvider.overrideWith(
        guest ? _GuestSession.new : _ActiveSession.new,
      ),
    ],
  );
  addTearDown(container.dispose);
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
                    builder: (context, ref, _) => MerchantHomeTab(
                      ref: ref,
                      merchantId: '7',
                      merchantName: '好店',
                    ),
                  ),
                ),
              ),
              GoRoute(
                path: '/login',
                builder: (_, _) => const Scaffold(body: Text('登入頁')),
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

  testWidgets('訪客在長壓預覽按讚：先跳登入提示，不送 LikePost、讚數不變', (tester) async {
    final app = FakeSocialApp();
    await _pump(tester, app: app, guest: true);
    await _openPreview(tester);
    expect(find.byIcon(AppIcons.favoriteBorder), findsOneWidget);

    await tester.tap(find.byIcon(AppIcons.favoriteBorder));
    await tester.pumpAndSettle();

    expect(find.text('前往登入'), findsOneWidget);
    expect(app.postLikes, isEmpty);
    expect(find.byIcon(AppIcons.favorite), findsNothing);
  });

  testWidgets('已登入按讚成功：預覽的心變實心並送出 LikePost', (tester) async {
    final app = FakeSocialApp();
    await _pump(tester, app: app);
    await _openPreview(tester);

    await tester.tap(find.byIcon(AppIcons.favoriteBorder));
    await tester.pumpAndSettle();

    expect(app.postLikes.single.postId, '101');
    expect(find.byIcon(AppIcons.favorite), findsOneWidget);
    expect(find.text('1'), findsOneWidget);
  });

  testWidgets('按讚失敗：預覽的讚狀態與數字還原', (tester) async {
    final app = FakeSocialApp(
      likeError: ConnectException(Code.unavailable, 'down'),
    );
    await _pump(tester, app: app);
    await _openPreview(tester);

    await tester.tap(find.byIcon(AppIcons.favoriteBorder));
    await tester.pumpAndSettle();

    expect(app.postLikes, hasLength(1));
    expect(find.byIcon(AppIcons.favoriteBorder), findsOneWidget);
    expect(find.byIcon(AppIcons.favorite), findsNothing);
    expect(find.text('0'), findsWidgets);
  });
}
