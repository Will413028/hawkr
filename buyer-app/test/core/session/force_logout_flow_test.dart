import 'package:hawkr_buyer/core/session/force_logout_listener.dart';
import 'package:hawkr_buyer/core/session/force_logout_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/shared/utils/require_login.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test('a session is usable only with an account id', () {
    expect(
      hasUsableSession(const SessionModel(userId: 'acc_1', nickname: '')),
      isTrue,
    );
    expect(
      hasUsableSession(const SessionModel(userId: '', nickname: '')),
      isFalse,
    );
    expect(hasUsableSession(null), isFalse);
  });

  testWidgets('guest is asked to log in and keeps the continue path', (
    tester,
  ) async {
    final session = _FakeSession(null);
    final container = ProviderContainer(
      overrides: [sessionProvider.overrideWith(() => session)],
    );
    addTearDown(container.dispose);
    bool? allowed;
    await container.read(sessionProvider.future);
    final router = GoRouter(
      initialLocation: '/market',
      routes: [
        GoRoute(
          path: '/market',
          builder: (context, _) => Scaffold(
            body: Consumer(
              builder: (context, ref, _) => TextButton(
                key: const ValueKey('protected-action'),
                onPressed: () async {
                  allowed = await requireLogin(context, ref);
                },
                child: const Text('收藏'),
              ),
            ),
          ),
        ),
        GoRoute(
          path: '/login',
          builder: (_, state) => Scaffold(
            body: Text('登入頁:${state.uri.queryParameters['continue'] ?? ''}'),
          ),
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byKey(const ValueKey('protected-action')));
    await tester.pumpAndSettle();

    expect(find.text('需要登入'), findsOneWidget);
    expect(find.text('此操作需要登入後才能繼續，是否前往登入？'), findsOneWidget);

    await tester.tap(find.text('取消'));
    await tester.pumpAndSettle();
    expect(allowed, isFalse);
    expect(session.logoutCalls, 0);
    expect(router.routerDelegate.currentConfiguration.uri.path, '/market');

    await tester.tap(find.byKey(const ValueKey('protected-action')));
    await tester.pumpAndSettle();
    await tester.tap(find.text('前往登入'));
    await tester.pumpAndSettle();

    expect(allowed, isFalse);
    expect(find.text('登入頁:/market'), findsOneWidget);
  });

  testWidgets('force logout dialog uses root navigator and requests login', (
    tester,
  ) async {
    final session = _FakeSession(
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer'),
    );
    final container = ProviderContainer(
      overrides: [sessionProvider.overrideWith(() => session)],
    );
    addTearDown(container.dispose);
    await container.read(sessionProvider.future);
    final navigatorKey = GlobalKey<NavigatorState>();
    String? requestedContinuePath;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp(
            navigatorKey: navigatorKey,
            builder: (_, child) => ForceLogoutListener(
              navigatorKey: navigatorKey,
              currentPath: () => '/market?sort=latest',
              onLoginRequired: (path) => requestedContinuePath = path,
              child: child!,
            ),
            home: const Scaffold(body: Text('市集')),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    container.read(forceLogoutProvider.notifier).trigger();
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.text('需要登入'), findsOneWidget);
    expect(find.text('此操作需要登入後才能繼續，是否前往登入？'), findsOneWidget);
    expect(find.text('取消'), findsOneWidget);
    expect(find.text('前往登入'), findsOneWidget);
    expect(session.logoutCalls, 0);

    await tester.tap(find.text('前往登入'));
    await tester.pumpAndSettle();

    expect(session.logoutCalls, 1);
    expect(requestedContinuePath, '/market?sort=latest');
    expect(container.read(forceLogoutProvider), isFalse);
  });
  testWidgets('停權提醒顯示伺服器訊息', (tester) async {
    final session = _FakeSession(
      const SessionModel(userId: 'buyer-1', nickname: 'Buyer'),
    );
    final container = ProviderContainer(
      overrides: [sessionProvider.overrideWith(() => session)],
    );
    addTearDown(container.dispose);
    await container.read(sessionProvider.future);
    final navigatorKey = GlobalKey<NavigatorState>();
    String? requestedContinuePath;

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp(
            navigatorKey: navigatorKey,
            builder: (_, child) => ForceLogoutListener(
              navigatorKey: navigatorKey,
              currentPath: () => '/market?sort=latest',
              onLoginRequired: (path) => requestedContinuePath = path,
              child: child!,
            ),
            home: const Scaffold(body: Text('市集')),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();

    container
        .read(forceLogoutProvider.notifier)
        .trigger(message: '帳號已停權，請聯絡客服');
    await tester.pump();
    await tester.pumpAndSettle();

    expect(find.text('需要登入'), findsOneWidget);
    expect(find.text('帳號已停權，請聯絡客服'), findsOneWidget);
    expect(find.text('取消'), findsOneWidget);
    expect(find.text('前往登入'), findsOneWidget);
    expect(session.logoutCalls, 0);

    await tester.tap(find.text('前往登入'));
    await tester.pumpAndSettle();

    expect(session.logoutCalls, 1);
    expect(requestedContinuePath, '/market?sort=latest');
    expect(container.read(forceLogoutProvider), isFalse);
  });
}

class _FakeSession extends Session {
  _FakeSession(this.initialSession);

  final SessionModel? initialSession;
  int logoutCalls = 0;

  @override
  Future<SessionModel?> build() async => initialSession;

  @override
  Future<void> logout() async {
    logoutCalls++;
    state = const AsyncData(null);
  }
}
