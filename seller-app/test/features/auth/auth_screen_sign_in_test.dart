import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/session/auth_session_provider.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/auth/presentation/auth_controller.dart';
import 'package:hawkr_seller/features/auth/presentation/auth_screen.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';

import '../../support/fake_auth_session.dart';
import 'login_flow_test.dart' show accountRepo;
import 'support/login_harness.dart';

class _RecordingSideEffects implements PostLoginSideEffects {
  int logins = 0;

  @override
  void onLogin() => logins += 1;

  @override
  Future<void> onSessionRestored() async {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });
  setUp(resetLoginStorage);

  late FakeAuthSession auth;
  late _RecordingSideEffects effects;
  late ProviderContainer container;

  Future<void> pumpScreen(WidgetTester tester) async {
    auth = FakeAuthSession();
    effects = _RecordingSideEffects();
    container = ProviderContainer(
      overrides: [
        authSessionProvider.overrideWith((ref) async => auth),
        authRepositoryProvider.overrideWithValue(accountRepo()),
        ...myStoreOverrides(),
        postLoginSideEffectsProvider.overrideWithValue(effects),
      ],
    );
    addTearDown(container.dispose);
    final store = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(store.close);
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, _) => const AuthScreen()),
        GoRoute(path: '/main/dashboard', builder: (_, _) => const SizedBox()),
        GoRoute(path: '/onboard', builder: (_, _) => const SizedBox()),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pump(const Duration(seconds: 3));
  }

  Finder pill(String name) => find.byKey(ValueKey('auth-oauth-$name-button'));

  for (final provider in OAuthProvider.values) {
    testWidgets('點 ${provider.name} 按鈕以該 provider 登入並顯示成功', (tester) async {
      await pumpScreen(tester);

      await tester.tap(pill(provider.name));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      expect(auth.calls, ['signIn ${provider.name}']);
      expect(find.text('${provider.label} 登入成功'), findsOneWidget);
      expect(container.read(sessionProvider).isLoggedIn, isTrue);
      await tester.pump(const Duration(seconds: 5));
    });
  }

  testWidgets('使用者取消：不顯示錯誤也不顯示成功，維持在登入頁', (tester) async {
    await pumpScreen(tester);
    auth.signInResult = false;

    await tester.tap(pill('google'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(auth.calls, ['signIn google']);
    expect(find.textContaining('登入成功'), findsNothing);
    expect(find.text('取消了'), findsNothing);
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
    expect(effects.logins, 0);
    expect(find.byType(AuthScreen), findsOneWidget);
    // 取消後按鈕恢復可再點
    await tester.tap(pill('line'));
    await tester.pump();
    expect(auth.calls, ['signIn google', 'signIn line']);
    await tester.pump(const Duration(seconds: 5));
  });

  testWidgets('登入拋錯：顯示去掉 Exception 前綴的錯誤文字，不登入', (tester) async {
    await pumpScreen(tester);
    auth.signInError = Exception('網路不穩，請稍後再試');

    await tester.tap(pill('line'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('網路不穩，請稍後再試'), findsOneWidget);
    expect(find.textContaining('Exception'), findsNothing);
    expect(find.textContaining('登入成功'), findsNothing);
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
    expect(effects.logins, 0);
    await tester.pump(const Duration(seconds: 5));
  });

  testWidgets('登入成功觸發登入後副作用一次', (tester) async {
    await pumpScreen(tester);

    await tester.tap(pill('google'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(effects.logins, 1);
    await tester.pump(const Duration(seconds: 5));
  });
}
