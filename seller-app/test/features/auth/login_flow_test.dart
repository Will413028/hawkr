import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/core/session/my_store.dart';
import 'package:hawkr_seller/core/storage/auth_storage.dart';
import 'dart:async';

import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/layout/app_media_query_policy.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/router/redirect_logic.dart';
import 'package:hawkr_seller/core/session/session_provider.dart'
    show sessionRestoreErrorProvider;
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/auth/presentation/auth_controller.dart';
import 'package:hawkr_seller/features/auth/presentation/auth_screen.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_clerk_session.dart';
import 'support/auth_test_fakes.dart';
import 'support/login_harness.dart';
import '../../support/default_error_text.dart';

/// 以「登入後的 session」餵真實 resolveAuthRedirect，斷言 redirect 決策。
String? redirectFrom(ProviderContainer container, String location) {
  return resolveAuthRedirect(
    isLoggedIn: container.read(sessionProvider).isLoggedIn,
    needsOnboarding: container.read(needsOnboardingProvider),
    myStore: container.read(myStoreSnapshotProvider),
    location: location,
    storeUnavailable: container.read(storeUnavailableProvider),
  );
}

/// GetSession 回 `acc-1`。
FakeAuthRepository accountRepo() {
  final repo = FakeAuthRepository();
  when(() => repo.currentAccountId()).thenAnswer((_) async => 'acc-1');
  return repo;
}

/// 系統瀏覽器導回 App 的網址。
final oauthCallback = Uri.parse('hawkrseller://oauth-callback');

const appleCredentials = AppleCredentials(
  identityToken: 'a-tok',
  authorizationCode: 'a-code',
);

/// 伺服器的 LocalizedMessage：使用者看到的句子（Status.message 是開發者訊息，不顯示）。
ErrorDetail _localized(String message) => ErrorDetail(
  'google.rpc.LocalizedMessage',
  (LocalizedMessage()
        ..locale = 'zh-TW'
        ..message = message)
      .writeToBuffer(),
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    // PushNotificationManager is a static singleton whose field initializer calls
    // FirebaseMessaging.instance (→ Firebase.app()), which throws [core/no-app]
    // unless a Firebase app is registered first. setupFirebaseCoreMocks() installs
    // a pigeon mock that satisfies initializeCore/initializeApp, then
    // Firebase.initializeApp() registers the DEFAULT app so Firebase.app() works.
    // Initialize Flavor so config-dependent code can access F.appFlavor
    // without throwing. Guard avoids the assert-once constraint when re-running tests.
    if (!F.isInitialized) F.init(Flavor.dev);

    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });
  setUp(() {
    resetLoginStorage();
  });

  testWidgets('停權訊息顯示一次，重新掛載 AuthScreen 不重播', (tester) async {
    final c = ProviderContainer();
    addTearDown(c.dispose);
    c.read(sessionRestoreErrorProvider.notifier).setMessage('帳號已停權');
    Widget screen() => UncontrolledProviderScope(
      container: c,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => const MaterialApp(home: AuthScreen()),
      ),
    );
    await tester.pumpWidget(screen());
    await tester.pump();
    expect(find.text('帳號已停權'), findsOneWidget);
    expect(c.read(sessionRestoreErrorProvider), isNull);
    await tester.pump(const Duration(seconds: 5));
    await tester.pumpWidget(const SizedBox.shrink());
    await tester.pumpWidget(screen());
    await tester.pump();
    expect(find.text('帳號已停權'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('auth route 忽略過期的 message extra', (tester) async {
    final c = ProviderContainer(
      overrides: [appRouterObserversProvider.overrideWithValue([])],
    );
    addTearDown(c.dispose);
    final router = c.read(goRouterProvider);
    addTearDown(router.dispose);
    router.go(AuthRoutes.auth, extra: {'message': '過期停權訊息'});
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: c,
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, _) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pump();
    // 未登入時店家狀態讀完（null）會排一個 provider 刷新的 timer，推進到它結束。
    await tester.pump(const Duration(milliseconds: 1));
    expect(find.byType(AuthScreen), findsOneWidget);
    expect(find.text('過期停權訊息'), findsNothing);
    expect(c.read(sessionRestoreErrorProvider), isNull);
    expect(tester.takeException(), isNull);
  });

  testWidgets('auth surface caps 2.0x system text at 1.4x without overflow', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => ProviderScope(
          child: MaterialApp(
            builder: (context, child) => MediaQuery(
              data: AppMediaQueryPolicy.apply(
                MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(2)),
              ),
              child: child!,
            ),
            home: const AuthScreen(),
          ),
        ),
      ),
    );
    await tester.pump();

    final authContext = tester.element(find.byType(AuthScreen));
    expect(MediaQuery.textScalerOf(authContext).scale(10), 14);
    expect(find.text('歡迎來到 Hawkr'), findsOneWidget);
    expect(tester.takeException(), isNull);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });

  for (final textScale in <double>[1, 1.15, 1.4]) {
    testWidgets('320dp OAuth pill keeps its full label at ${textScale}x text', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(320, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ScreenUtilInit(
          designSize: const Size(414, 896),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => ProviderScope(
            child: MaterialApp(
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: TextScaler.linear(textScale)),
                child: child!,
              ),
              home: const AuthScreen(),
            ),
          ),
        ),
      );
      await tester.pump(const Duration(milliseconds: 840));

      const provider = 'google';
      const labelText = '透過Google繼續';
      final pill = find.byKey(const ValueKey('auth-oauth-$provider-button'));
      final icon = find.byKey(const ValueKey('auth-oauth-$provider-icon'));
      final label = find.byKey(const ValueKey('auth-oauth-$provider-label'));
      final labelTextFinder = find.descendant(
        of: label,
        matching: find.byType(Text),
      );

      expect(pill, findsOneWidget);
      expect(icon, findsOneWidget);
      expect(label, findsOneWidget);
      expect(find.bySemanticsLabel(labelText), findsOneWidget);
      expect(
        find.byKey(const ValueKey('auth-oauth-$provider-tap-hint')),
        findsNothing,
      );

      final pillRect = tester.getRect(pill);
      final iconRect = tester.getRect(icon);
      final labelRect = tester.getRect(label);
      expect(pillRect.height, greaterThanOrEqualTo(48));
      expect(pillRect.contains(iconRect.topLeft), isTrue);
      expect(pillRect.contains(iconRect.bottomRight), isTrue);
      expect(iconRect.overlaps(labelRect), isFalse);
      expect(
        iconRect.left - pillRect.left,
        greaterThanOrEqualTo(12),
        reason: '大字級仍須保留 OAuth 圖示與按鈕左緣的安全內距',
      );

      final labelWidget = tester.widget<Text>(labelTextFinder);
      expect(labelWidget.data, labelText);
      expect(labelWidget.maxLines, 1);
      expect(labelWidget.softWrap, isFalse);
      expect(labelWidget.overflow, TextOverflow.ellipsis);
      final paragraph = tester.renderObject<RenderParagraph>(
        find.descendant(of: labelTextFinder, matching: find.byType(RichText)),
      );
      expect(
        paragraph.didExceedMaxLines,
        isFalse,
        reason:
            'scale=$textScale pill=$pillRect icon=$iconRect '
            'label=$labelRect '
            'intrinsic=${paragraph.getMaxIntrinsicWidth(double.infinity)}',
      );
      expect(tester.takeException(), isNull);
      semantics.dispose();
    });
  }

  test(
    '#1 Google 成功（已 onboard）：系統瀏覽器登入 Clerk → GetSession → 寫 session + redirect dashboard',
    () async {
      final picker = FakeOAuthPicker(browserCallback: oauthCallback);
      final clerk = FakeClerkSession();
      final container = makeLoginContainer(
        picker: picker,
        repo: accountRepo(),
        clerk: clerk,
      );

      final signedIn = await container
          .read(authControllerProvider.notifier)
          .signInWith(OAuthProvider.google);

      expect(signedIn, isTrue);
      expect(clerk.calls, ['browser google']);
      expect(picker.browserSchemes, ['hawkrseller']);
      final session = container.read(sessionProvider);
      expect(session.isLoggedIn, isTrue);
      expect(session.accountId, 'acc-1');
      expect(
        container.read(sellerStoreIdProvider),
        kSellerId.toString(),
      ); // 登入時已同步 GetMyStore
      expect(container.read(needsOnboardingProvider), isFalse);
      expect(redirectFrom(container, AuthRoutes.auth), MainRoutes.dashboard);
    },
  );

  test('#1b LINE：同樣經系統瀏覽器', () async {
    final clerk = FakeClerkSession();
    final container = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: accountRepo(),
      clerk: clerk,
    );

    await container
        .read(authControllerProvider.notifier)
        .signInWith(OAuthProvider.line);

    expect(clerk.calls, ['browser line']);
    expect(container.read(sessionProvider).isLoggedIn, isTrue);
  });

  test(
    '#2 Google 新賣家：登入時 GetMyStore 回 not_found（沒有店家） → redirect onboard',
    () async {
      final container = makeLoginContainer(
        picker: FakeOAuthPicker(browserCallback: oauthCallback),
        repo: accountRepo(),
        needsOnboarding: true,
      );

      await container
          .read(authControllerProvider.notifier)
          .signInWith(OAuthProvider.google);

      final session = container.read(sessionProvider);
      expect(session.isLoggedIn, isTrue);
      expect(container.read(needsOnboardingProvider), isTrue);
      expect(redirectFrom(container, AuthRoutes.auth), AuthRoutes.onboard);
    },
  );

  test('#2b 新賣家：GetMyStore 回答前停在登入頁（不閃 dashboard），回答後才導 onboard', () async {
    final gate = Completer<void>();
    final container = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: accountRepo(),
      needsOnboarding: true,
      meGate: gate.future,
    );
    final locations = <String?>[];
    // 同 router：聽 session 與店家的衍生狀態，通知時 redirect 讀到的是重算後的值。
    void record(Object? previous, Object? next) =>
        locations.add(redirectFrom(container, AuthRoutes.auth));
    container
      ..listen(sessionProvider, record)
      ..listen<bool?>(needsOnboardingProvider, record)
      ..listen<MyStore?>(myStoreSnapshotProvider, record)
      ..listen<bool>(storeUnavailableProvider, record);

    final signIn = container
        .read(authControllerProvider.notifier)
        .signInWith(OAuthProvider.google);
    await pumpEventQueue();

    final pending = container.read(sessionProvider);
    expect(pending.isLoggedIn, isTrue);
    expect(container.read(needsOnboardingProvider), isNull);
    expect(redirectFrom(container, AuthRoutes.auth), isNull);

    gate.complete();
    await signIn;
    await pumpEventQueue();

    expect(container.read(needsOnboardingProvider), isTrue);
    expect(locations, isNot(contains(MainRoutes.dashboard)));
    expect(locations.last, AuthRoutes.onboard);
  });

  test('#2c GetMyStore 失敗且沒有快照：不當成已 onboard，導去重試頁（fail-closed）', () async {
    final container = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: accountRepo(),
      meFails: true,
    );

    await container
        .read(authControllerProvider.notifier)
        .signInWith(OAuthProvider.google);

    expect(container.read(needsOnboardingProvider), isNull);
    expect(container.read(storeUnavailableProvider), isTrue);
    expect(
      redirectFrom(container, AuthRoutes.auth),
      SellerRoutes.storeUnavailable,
    );
    expect(
      redirectFrom(container, MainRoutes.dashboard),
      SellerRoutes.storeUnavailable,
    );
  });

  test(
    '#3 Apple 成功：以 identity token 登入 Clerk → 寫 session + redirect dashboard',
    () async {
      final clerk = FakeClerkSession();
      final container = makeLoginContainer(
        picker: FakeOAuthPicker(apple: appleCredentials),
        repo: accountRepo(),
        clerk: clerk,
      );

      final signedIn = await container
          .read(authControllerProvider.notifier)
          .signInWith(OAuthProvider.apple);

      expect(signedIn, isTrue);
      expect(clerk.calls, ['apple a-tok']);
      final session = container.read(sessionProvider);
      expect(session.isLoggedIn, isTrue);
      expect(redirectFrom(container, AuthRoutes.auth), MainRoutes.dashboard);
    },
  );

  test('#6 使用者取消瀏覽器或 Apple 的畫面 → false、不呼叫 GetSession、不寫 session', () async {
    final repo = accountRepo();
    final container = makeLoginContainer(
      picker: FakeOAuthPicker(), // null = 取消
      repo: repo,
    );
    final controller = container.read(authControllerProvider.notifier);

    expect(await controller.signInWith(OAuthProvider.google), isFalse);
    expect(await controller.signInWith(OAuthProvider.apple), isFalse);

    expect(container.read(sessionProvider).isLoggedIn, isFalse);
    verifyNever(() => repo.currentAccountId());
  });

  test('#7 GetSession 暫時失敗 → 保留 Clerk、冒泡、不寫 session', () async {
    final repo = FakeAuthRepository();
    when(() => repo.currentAccountId()).thenThrow(
      AuthRequestException.from(
        ConnectException(Code.internal, 'backend 500'),
        defaults: zhDefaultErrorText,
      ),
    );
    final clerk = FakeClerkSession();
    final container = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: repo,
      clerk: clerk,
    );

    await expectLater(
      container
          .read(authControllerProvider.notifier)
          .signInWith(OAuthProvider.google),
      throwsException,
    );
    expect(clerk.calls, ['browser google']);
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
  });

  test('GetSession 回應缺少帳號時保留 Clerk', () async {
    final repo = FakeAuthRepository();
    when(
      () => repo.currentAccountId(),
    ).thenThrow(const MalformedAuthResponse('missing account'));
    final clerk = FakeClerkSession();
    final c = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: repo,
      clerk: clerk,
    );
    await expectLater(
      c.read(authControllerProvider.notifier).signInWith(OAuthProvider.google),
      throwsA(isA<MalformedAuthResponse>()),
    );
    expect(clerk.isSignedIn, isTrue);
  });

  test('暫時故障後再次登入重試 GetSession，不重新開啟 OAuth', () async {
    final repo = FakeAuthRepository();
    when(() => repo.currentAccountId()).thenThrow(
      AuthRequestException.from(
        ConnectException(Code.unavailable, 'offline'),
        defaults: zhDefaultErrorText,
      ),
    );
    final clerk = FakeClerkSession();
    final c = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: repo,
      clerk: clerk,
    );
    await expectLater(
      c.read(authControllerProvider.notifier).signInWith(OAuthProvider.google),
      throwsA(isA<AuthRequestException>()),
    );
    when(() => repo.currentAccountId()).thenAnswer((_) async => 'acc-1');
    expect(
      await c
          .read(authControllerProvider.notifier)
          .signInWith(OAuthProvider.google),
      isTrue,
    );
    expect(clerk.calls, ['browser google']);
  });

  for (final code in [Code.unauthenticated, Code.permissionDenied]) {
    test('GetSession 明確拒絕憑證時結束 Clerk session', () async {
      final repo = FakeAuthRepository();
      when(() => repo.currentAccountId()).thenThrow(
        AuthRequestException.from(
          ConnectException(code, 'invalid'),
          defaults: zhDefaultErrorText,
        ),
      );
      final clerk = FakeClerkSession();
      final container = makeLoginContainer(
        picker: FakeOAuthPicker(browserCallback: oauthCallback),
        repo: repo,
        clerk: clerk,
      );

      await expectLater(
        container
            .read(authControllerProvider.notifier)
            .signInWith(OAuthProvider.google),
        throwsA(isA<AuthRequestException>()),
      );
      expect(clerk.calls, ['browser google', 'signOut']);
      expect(container.read(sessionProvider).isLoggedIn, isFalse);
    });
  }

  for (final code in [
    Code.canceled,
    Code.unknown,
    Code.deadlineExceeded,
    Code.resourceExhausted,
    Code.unimplemented,
    Code.internal,
    Code.unavailable,
    Code.dataLoss,
    Code.failedPrecondition,
    Code.aborted,
    Code.invalidArgument,
  ]) {
    test('$code 是暫時失敗，不拒絕 Clerk', () {
      expect(
        AuthRequestException.from(
          ConnectException(code, 'retry'),
          defaults: zhDefaultErrorText,
        ).isTransient,
        isTrue,
      );
    });
  }

  for (final error in [
    const MalformedAuthResponse('missing account'),
    ...[Code.failedPrecondition, Code.aborted, Code.invalidArgument].map(
      (code) => AuthRequestException.from(
        ConnectException(code, 'unexpected'),
        defaults: zhDefaultErrorText,
      ),
    ),
  ]) {
    test('冷啟動非拒絕失敗保留 Clerk：$error', () async {
      final repo = FakeAuthRepository();
      when(() => repo.currentAccountId()).thenThrow(error);
      final clerk = FakeClerkSession(signedIn: true);
      final c = makeLoginContainer(
        picker: FakeOAuthPicker(),
        repo: repo,
        clerk: clerk,
      );
      await c.read(sessionProvider.notifier).initialize();
      expect(c.read(sessionProvider).isLoggedIn, isFalse);
      expect(clerk.isSignedIn, isTrue);
      expect(clerk.calls, isEmpty);
    });
  }

  test('cache owner 未知時保留 AI 修圖歷史', () async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('image_editor_ai_history', '[]');
    await AuthStorage.restoreAccount('A');
    expect(prefs.getString('image_editor_ai_history'), '[]');
    expect(prefs.getString('auth_cache_owner'), 'A');
  });

  test('換帳號清除前任 AI 修圖歷史', () async {
    await AuthStorage.restoreAccount('A');
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('image_editor_ai_history', '[]');
    await AuthStorage.restoreAccount('B');
    expect(prefs.containsKey('image_editor_ai_history'), isFalse);
    expect(prefs.getString('auth_cache_owner'), 'B');
  });

  test('背景重試停權會結束 Clerk 並發布伺服器文字', () async {
    final repo = FakeAuthRepository();
    var calls = 0;
    when(() => repo.currentAccountId()).thenAnswer((_) async {
      throw AuthRequestException.from(
        ConnectException(
          ++calls == 1 ? Code.unavailable : Code.permissionDenied,
          'account suspended',
          details: [_localized('帳號已停權')],
        ),
        defaults: zhDefaultErrorText,
      );
    });
    final clerk = FakeClerkSession(signedIn: true);
    final c = makeLoginContainer(
      picker: FakeOAuthPicker(),
      repo: repo,
      clerk: clerk,
    );
    await c.read(sessionProvider.notifier).initialize();
    expect(clerk.isSignedIn, isTrue);
    await Future<void>.delayed(const Duration(milliseconds: 1600));
    expect(clerk.isSignedIn, isFalse);
    expect(c.read(sessionRestoreErrorProvider), '帳號已停權');
    expect(calls, 2);
  });

  test('限流冷啟動自動重試，成功後保持同帳號快取', () async {
    await AuthStorage.restoreAccount('acc-1');
    await _saveCachedEditHistory();
    final repo = FakeAuthRepository();
    var calls = 0;
    when(() => repo.currentAccountId()).thenAnswer((_) async {
      if (++calls == 1) {
        throw AuthRequestException.from(
          ConnectException(Code.resourceExhausted, 'retry'),
          defaults: zhDefaultErrorText,
        );
      }
      return 'acc-1';
    });
    final clerk = FakeClerkSession(signedIn: true);
    final c = makeLoginContainer(
      picker: FakeOAuthPicker(),
      repo: repo,
      clerk: clerk,
    );
    await c.read(sessionProvider.notifier).initialize();
    expect(c.read(sessionProvider).isLoggedIn, isFalse);
    expect(calls, 1);
    await Future<void>.delayed(const Duration(milliseconds: 1600));
    expect(c.read(sessionProvider).isLoggedIn, isTrue);
    expect(calls, 2);
    expect(clerk.calls, isEmpty);
    expect(await _cachedEditHistory(), '[]');
  });

  testWidgets('冷啟動持續限流後登入同帳號不清除快取', (tester) async {
    await AuthStorage.restoreAccount('acc-1');
    await _saveCachedEditHistory();
    final repo = FakeAuthRepository();
    var calls = 0;
    when(() => repo.currentAccountId()).thenAnswer((_) async {
      calls++;
      throw AuthRequestException.from(
        ConnectException(Code.resourceExhausted, 'retry'),
        defaults: zhDefaultErrorText,
      );
    });
    final clerk = FakeClerkSession(signedIn: true);
    final c = makeLoginContainer(
      picker: FakeOAuthPicker(),
      repo: repo,
      clerk: clerk,
    );
    await c.read(sessionProvider.notifier).initialize();
    expect(c.read(sessionProvider).isLoggedIn, isFalse);
    expect(calls, 1);
    await tester.pump(const Duration(milliseconds: 1500));
    expect(calls, 2);
    await tester.pump(const Duration(milliseconds: 3000));
    expect(calls, 3);
    await tester.pump(const Duration(milliseconds: 6000));
    expect(calls, 4);
    await tester.pump(const Duration(minutes: 1));
    expect(calls, 4);
    expect(clerk.calls, isEmpty);
    await c.read(sessionProvider.notifier).login(accountId: 'acc-1');
    expect(await _cachedEditHistory(), '[]');
    c.dispose();
  });

  test(
    '#8 登入後模擬 restart → initialize 從 Clerk 與 GetSession 恢復 session',
    () async {
      final clerk = FakeClerkSession();
      final c1 = makeLoginContainer(
        picker: FakeOAuthPicker(browserCallback: oauthCallback),
        repo: accountRepo(),
        clerk: clerk,
      );
      await c1
          .read(authControllerProvider.notifier)
          .signInWith(OAuthProvider.google);
      expect(c1.read(sessionProvider).isLoggedIn, isTrue);
      c1.dispose();

      // 第二個 container（模擬 app 重啟，沿用同一份 mock 持久層與 Clerk 保存的登入）
      final c2 = makeLoginContainer(
        picker: FakeOAuthPicker(),
        repo: accountRepo(),
        clerk: clerk,
      );
      await c2.read(sessionProvider.notifier).initialize();

      final restored = c2.read(sessionProvider);
      expect(restored.isLoggedIn, isTrue);
      expect(restored.accountId, 'acc-1');
      // GetMyStore 還沒回答：不導頁，停在載入狀態。
      expect(c2.read(needsOnboardingProvider), isNull);
      expect(redirectFrom(c2, AuthRoutes.auth), isNull);

      // onSessionRestored 同步賣家之後才進主畫面。
      await c2.read(currentStoreProvider.future);
      expect(redirectFrom(c2, AuthRoutes.auth), MainRoutes.dashboard);
    },
  );

  test('#9 restart 時 Clerk 的 session 已結束 → 不恢復，清掉保存的帳號', () async {
    final clerk = FakeClerkSession();
    final c1 = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: accountRepo(),
      clerk: clerk,
    );
    await c1
        .read(authControllerProvider.notifier)
        .signInWith(OAuthProvider.google);
    c1.dispose();
    clerk.signedIn = false;

    final c2 = makeLoginContainer(
      picker: FakeOAuthPicker(),
      repo: FakeAuthRepository(),
      clerk: clerk,
    );
    await c2.read(sessionProvider.notifier).initialize();

    expect(c2.read(sessionProvider).isLoggedIn, isFalse);
  });

  test('#10 登出 → 登出 Clerk、清 session、redirect 回 /auth', () async {
    final clerk = FakeClerkSession();
    final container = makeLoginContainer(
      picker: FakeOAuthPicker(browserCallback: oauthCallback),
      repo: accountRepo(),
      clerk: clerk,
    );
    final controller = container.read(authControllerProvider.notifier);
    await controller.signInWith(OAuthProvider.google);
    expect(container.read(sessionProvider).isLoggedIn, isTrue);

    await controller.logout();

    expect(clerk.calls, ['browser google', 'signOut']);
    final session = container.read(sessionProvider);
    expect(session.isLoggedIn, isFalse);
    expect(redirectFrom(container, MainRoutes.dashboard), AuthRoutes.auth);
  });
}

Future<void> _saveCachedEditHistory() async {
  final prefs = await SharedPreferences.getInstance();
  await prefs.setString('image_editor_ai_history', '[]');
}

Future<String?> _cachedEditHistory() async {
  final prefs = await SharedPreferences.getInstance();
  return prefs.getString('image_editor_ai_history');
}
