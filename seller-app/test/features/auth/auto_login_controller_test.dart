import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;
import 'package:hawkr_seller/core/storage/auth_storage.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/session/auth_session_provider.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/auth/presentation/auto_login_controller.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_auth_session.dart';
import 'support/auth_test_fakes.dart';
import 'support/login_harness.dart';
import '../../support/default_error_text.dart';

/// 冷啟動自動登入：session token 登入中時以 GetSession 恢復帳號。
/// 導頁見 welcome_screen_auto_login_test.dart。
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
    if (!F.isInitialized) F.init(Flavor.dev);
    // 恢復 session 時同步 chat / 頭像等 singleton，會碰到 Firebase。
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });

  setUp(resetLoginStorage);

  ProviderContainer makeContainer(
    FakeAuthSession auth, {
    bool needsOnboarding = false,
    Future<String> Function()? accountAnswer,
    bool authFails = false,
  }) {
    final account = FakeAuthRepository();
    when(() => account.currentAccountId()).thenAnswer(
      (_) async => accountAnswer == null ? 'acc-1' : accountAnswer(),
    );
    final container = ProviderContainer(
      overrides: [
        authSessionProvider.overrideWith((ref) async {
          if (authFails) throw StateError("keychain");
          return auth;
        }),
        authRepositoryProvider.overrideWithValue(account),
        ...myStoreOverrides(needsOnboarding: needsOnboarding),
        postLoginSideEffectsProvider.overrideWith(SyncSellerOnRestore.new),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  Future<AutoLoginResult> check(ProviderContainer container) =>
      container.read(autoLoginControllerProvider.notifier).check();

  test('session token 啟動失敗不重試 GetSession、不登出、不清快取', () async {
    await AuthStorage.restoreAccount('acc-1');
    await _saveCachedEditHistory();
    var calls = 0;
    final auth = FakeAuthSession(signedIn: true);
    final c = makeContainer(
      auth,
      authFails: true,
      accountAnswer: () async {
        calls++;
        throw AuthRequestException.from(
          ConnectException(Code.unauthenticated, 'unauthenticated'),
          defaults: zhDefaultErrorText,
        );
      },
    );
    expect(await check(c), isA<AutoLoginSignedOut>());
    await Future<void>.delayed(const Duration(milliseconds: 1600));
    expect(calls, 0);
    expect(auth.calls, isEmpty);
    expect(await _cachedEditHistory(), '[]');
  });

  test('GetSession 暫時失敗時保留 session token session', () async {
    final auth = FakeAuthSession(signedIn: true);
    final container = makeContainer(
      auth,
      accountAnswer: () async => throw StateError('offline'),
    );

    expect(await check(container), isA<AutoLoginSignedOut>());
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
    expect(auth.calls, isEmpty);
  });

  for (final code in [Code.unauthenticated, Code.permissionDenied]) {
    test('冷啟動 $code 結束 session token，停權保留訊息', () async {
      final auth = FakeAuthSession(signedIn: true);
      final container = makeContainer(
        auth,
        accountAnswer: () async => throw AuthRequestException.from(
          ConnectException(
            code,
            'account suspended',
            details: [_localized('帳號已停權')],
          ),
          defaults: zhDefaultErrorText,
        ),
      );
      expect(await check(container), isA<AutoLoginSignedOut>());
      expect(auth.isSignedIn, isFalse);
      if (code == Code.permissionDenied) {
        expect(container.read(sessionRestoreErrorProvider), '帳號已停權');
      }
    });
  }

  test('同帳號冷啟動保留快取，換帳號清除快取', () async {
    await AuthStorage.restoreAccount('acc-1');
    await _saveCachedEditHistory();
    final c = makeContainer(FakeAuthSession(signedIn: true));
    await check(c);
    expect(await _cachedEditHistory(), '[]');
    await AuthStorage.restoreAccount('other');
    expect(await _cachedEditHistory(), isNull);
  });

  test('session token 的 session 已經結束 → 不呼叫 GetSession', () async {
    final container = makeContainer(FakeAuthSession());

    expect(await check(container), isA<AutoLoginSignedOut>());
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
  });

  test('session token 登入中但沒有本機帳號 → GetSession 恢復帳號', () async {
    final auth = FakeAuthSession(signedIn: true);
    final container = makeContainer(auth);

    expect(await check(container), isA<AutoLoginSignedIn>());
    expect(container.read(sessionProvider).accountId, 'acc-1');
    expect(auth.calls, isEmpty);
  });

  test('session token 登入中、有帳號 → 恢復 session，已建店時不需要 onboarding', () async {
    final container = makeContainer(FakeAuthSession(signedIn: true));

    final result = await check(container);

    expect(
      result,
      isA<AutoLoginSignedIn>().having(
        (r) => r.needsOnboarding,
        'needsOnboarding',
        isFalse,
      ),
    );
    final session = container.read(sessionProvider);
    expect(session.isLoggedIn, isTrue);
    expect(session.accountId, 'acc-1');
    expect(container.read(sellerStoreIdProvider), kSellerId.toString());
  });

  test('session token 登入中、有帳號、還沒有 sellers row → 需要 onboarding', () async {
    final container = makeContainer(
      FakeAuthSession(signedIn: true),
      needsOnboarding: true,
    );

    expect(
      await check(container),
      isA<AutoLoginSignedIn>().having(
        (r) => r.needsOnboarding,
        'needsOnboarding',
        isTrue,
      ),
    );
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
