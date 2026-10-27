import 'dart:async';

import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/router/redirect_logic.dart';
import 'package:hawkr_seller/core/router/session_listenable.dart';
import 'package:hawkr_seller/core/session/clerk_session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/presentation/welcome_screen.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:go_router/go_router.dart';

import '../../support/fake_clerk_session.dart';
import 'support/auth_test_fakes.dart';
import 'support/login_harness.dart';

GoRouter _router(Ref ref) {
  final sessionListenable = SessionChangeNotifier(ref);
  ref.onDispose(sessionListenable.dispose);

  Widget placeholder(String text) => Scaffold(body: Text(text));

  return GoRouter(
    initialLocation: '/welcome',
    refreshListenable: sessionListenable,
    redirect: (context, state) {
      final session = ref.read(sessionProvider);
      return resolveAuthRedirect(
        isLoggedIn: session.isLoggedIn,
        needsOnboarding: ref.read(needsOnboardingProvider),
        myStore: ref.read(myStoreSnapshotProvider),
        storeUnavailable: ref.read(storeUnavailableProvider),
        location: state.matchedLocation,
      );
    },
    routes: [
      GoRoute(path: '/welcome', builder: (_, _) => const WelcomeScreen()),
      GoRoute(path: '/auth', builder: (_, _) => placeholder('AUTH')),
      GoRoute(path: '/main/dashboard', builder: (_, _) => placeholder('HOME')),
      GoRoute(path: '/onboard', builder: (_, _) => placeholder('ONBOARD')),
    ],
  );
}

final _routerProvider = Provider<GoRouter>(_router);

/// 冷啟動停在 WelcomeScreen（品牌載入），自動登入的結果決定導去登入、onboarding 或主畫面。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });

  setUp(() {
    resetLoginStorage();
  });

  Future<void> coldStart(
    WidgetTester tester, {
    required bool clerkSignedIn,
    bool needsOnboarding = false,
  }) async {
    final clerk = FakeClerkSession(signedIn: clerkSignedIn);
    final account = FakeAuthRepository();
    when(() => account.currentAccountId()).thenAnswer((_) async => 'acc-1');
    final container = ProviderContainer(
      overrides: [
        clerkSessionProvider.overrideWith((ref) async => clerk),
        authRepositoryProvider.overrideWithValue(account),
        ...myStoreOverrides(needsOnboarding: needsOnboarding),
        postLoginSideEffectsProvider.overrideWith(SyncSellerOnRestore.new),
      ],
    );
    addTearDown(container.dispose);

    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) =>
              MaterialApp.router(routerConfig: container.read(_routerProvider)),
        ),
      ),
    );
    // 讀 storage 與 GetMyStore 的每一步 await 逐幀推進，直到導頁離開載入畫面。
    for (var i = 0; i < 10; i++) {
      await tester.pump(const Duration(milliseconds: 50));
    }
    await tester.pumpAndSettle();
  }

  testWidgets('Clerk 登入中、有帳號 → 開啟就是登入狀態，進主畫面', (tester) async {
    await coldStart(tester, clerkSignedIn: true);

    expect(find.text('HOME'), findsOneWidget);
    expect(find.byType(WelcomeScreen), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('Clerk 沒有登入 → 到登入頁', (tester) async {
    await coldStart(tester, clerkSignedIn: false);

    expect(find.text('AUTH'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('Clerk 登入中、有帳號、還沒建店 → 到 onboarding', (tester) async {
    await coldStart(tester, clerkSignedIn: true, needsOnboarding: true);

    expect(find.text('ONBOARD'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('恢復途中 WelcomeScreen 被 dispose → 不會對它已失效的 ref 呼叫 read', (
    tester,
  ) async {
    final initGate = Completer<void>();

    Widget shell(Widget child) => ProviderScope(
      overrides: [
        sessionProvider.overrideWith(() => _BlockingSession(initGate)),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(home: child),
      ),
    );

    await tester.pumpWidget(shell(const WelcomeScreen()));
    await tester.pump();

    // 自動登入卡在 initialize()；此時卸載 WelcomeScreen，模擬恢復完成前已導頁離開。
    await tester.pumpWidget(shell(const SizedBox.shrink()));

    initGate.complete();
    await tester.pump();

    expect(tester.takeException(), isNull);
  });
}

class _BlockingSession extends Session {
  _BlockingSession(this._gate);
  final Completer<void> _gate;

  @override
  Future<void> initialize() => _gate.future;
}
