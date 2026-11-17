import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/account_restriction.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../features/auth/support/auth_test_fakes.dart';
import '../../features/auth/support/login_harness.dart';
import '../../support/connect_errors.dart';
import '../../support/fake_auth_session.dart';

const _restriction = AccountRestriction(
  category: PolicyCategory.POLICY_CATEGORY_UNSPECIFIED,
  reason: '原文',
);

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
    TestWidgetsFlutterBinding.ensureInitialized();
  });
  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  group('GetSession 回報停權', () {
    test('冷啟動：進入受限狀態，保持登入，不結束 session token', () async {
      final repo = FakeAuthRepository();
      when(() => repo.currentSession()).thenAnswer(
        (_) async =>
            const AccountSession(accountId: 'acc-1', restriction: _restriction),
      );
      final auth = FakeAuthSession(signedIn: true);
      final c = makeLoginContainer(repo: repo, auth: auth);

      await c.read(sessionProvider.notifier).initialize();

      expect(c.read(sessionProvider).isLoggedIn, isTrue);
      expect(c.read(accountRestrictionStateProvider)?.reason, '原文');
      expect(auth.calls, isEmpty);
      expect(auth.isSignedIn, isTrue);
    });

    test('沒有停權：沒有受限狀態', () async {
      final repo = FakeAuthRepository();
      when(
        () => repo.currentSession(),
      ).thenAnswer((_) async => const AccountSession(accountId: 'acc-1'));
      final c = makeLoginContainer(
        repo: repo,
        auth: FakeAuthSession(signedIn: true),
      );

      await c.read(sessionProvider.notifier).initialize();

      expect(c.read(accountRestrictionStateProvider), isNull);
    });

    test('登出清除受限狀態', () async {
      final repo = FakeAuthRepository();
      when(() => repo.currentSession()).thenAnswer(
        (_) async =>
            const AccountSession(accountId: 'acc-1', restriction: _restriction),
      );
      when(() => repo.signOut()).thenAnswer((_) async {});
      final c = makeLoginContainer(
        repo: repo,
        auth: FakeAuthSession(signedIn: true),
      );
      await c.read(sessionProvider.notifier).initialize();

      await c.read(sessionProvider.notifier).logout();

      expect(c.read(accountRestrictionStateProvider), isNull);
    });
  });

  group('前景請求的 ACCOUNT_SUSPENDED', () {
    test('進入受限狀態並以 GetSession 補上資料，不登出', () async {
      final repo = FakeAuthRepository();
      var fetched = false;
      when(() => repo.currentSession()).thenAnswer((_) async {
        final restricted = fetched;
        fetched = true;
        return AccountSession(
          accountId: 'acc-1',
          restriction: restricted
              ? const AccountRestriction(
                  category: PolicyCategory.POLICY_CATEGORY_UNSPECIFIED,
                  reason: '補上的理由',
                )
              : null,
        );
      });
      final auth = FakeAuthSession(signedIn: true);
      final c = makeLoginContainer(repo: repo, auth: auth);
      await c.read(sessionProvider.notifier).initialize();
      expect(c.read(accountRestrictionStateProvider), isNull);

      await c.read(sessionProvider.notifier).onAccountSuspendedSignal();

      expect(c.read(sessionProvider).isLoggedIn, isTrue);
      expect(c.read(accountRestrictionStateProvider)?.reason, '補上的理由');
      expect(auth.calls, isEmpty);
    });

    test('GetSession 讀不到時仍進入受限狀態', () async {
      final repo = FakeAuthRepository();
      var calls = 0;
      when(() => repo.currentSession()).thenAnswer((_) async {
        if (++calls == 1) return const AccountSession(accountId: 'acc-1');
        throw StateError('offline');
      });
      final c = makeLoginContainer(
        repo: repo,
        auth: FakeAuthSession(signedIn: true),
      );
      await c.read(sessionProvider.notifier).initialize();

      await c.read(sessionProvider.notifier).onAccountSuspendedSignal();

      expect(c.read(accountRestrictionStateProvider), isNotNull);
      expect(c.read(sessionProvider).isLoggedIn, isTrue);
    });
  });

  group('onAccountSuspendedSignal 核對', () {
    Future<(ProviderContainer, FakeAuthRepository)> loggedIn(
      Future<AccountSession> Function() next, {
      AccountRestriction? initial,
    }) async {
      final repo = FakeAuthRepository();
      var first = true;
      when(() => repo.currentSession()).thenAnswer((_) {
        if (first) {
          first = false;
          return Future.value(
            AccountSession(accountId: 'acc-1', restriction: initial),
          );
        }
        return next();
      });
      when(() => repo.signOut()).thenAnswer((_) async {});
      final c = makeLoginContainer(
        repo: repo,
        auth: FakeAuthSession(signedIn: true),
      );
      await c.read(sessionProvider.notifier).initialize();
      return (c, repo);
    }

    test('訊號後 GetSession 回報沒有停權：仍然受限（伺服器剛明確拒絕）', () async {
      final (c, _) = await loggedIn(
        () async => const AccountSession(accountId: 'acc-1'),
      );
      expect(c.read(accountRestrictionStateProvider), isNull);

      await c.read(sessionProvider.notifier).onAccountSuspendedSignal();

      expect(c.read(accountRestrictionStateProvider), isNotNull);
    });

    test('訊號立即進入受限，不等 GetSession', () async {
      final gate = Completer<AccountSession>();
      final (c, _) = await loggedIn(() => gate.future);

      final pending = c
          .read(sessionProvider.notifier)
          .onAccountSuspendedSignal();

      expect(c.read(accountRestrictionStateProvider), isNotNull);
      gate.complete(
        const AccountSession(accountId: 'acc-1', restriction: _restriction),
      );
      await pending;
      expect(
        c.read(accountRestrictionStateProvider)?.reason,
        _restriction.reason,
      );
    });

    test('冷啟動 GetSession 沒有停權：清除受限狀態', () async {
      final (c, _) = await loggedIn(
        () async => const AccountSession(accountId: 'acc-1'),
        initial: _restriction,
      );
      expect(c.read(accountRestrictionStateProvider), isNotNull);

      await c.read(sessionProvider.notifier).initialize();

      expect(c.read(accountRestrictionStateProvider), isNull);
    });

    test('進行中的補細節查詢不吃掉後來的訊號：換帳號後的訊號照樣進入受限', () async {
      final gate = Completer<AccountSession>();
      final (c, _) = await loggedIn(() => gate.future);
      final notifier = c.read(sessionProvider.notifier);
      final first = notifier.onAccountSuspendedSignal();

      await notifier.login(accountId: 'acc-2');
      expect(c.read(accountRestrictionStateProvider), isNull);
      final second = notifier.onAccountSuspendedSignal();

      expect(c.read(accountRestrictionStateProvider), isNotNull);
      gate.complete(const AccountSession(accountId: 'acc-1'));
      await Future.wait([first, second]);
      expect(c.read(accountRestrictionStateProvider), isNotNull);
    });

    test('登出後才回來的 GetSession 丟棄，不留下受限狀態', () async {
      final gate = Completer<AccountSession>();
      final (c, _) = await loggedIn(() => gate.future);
      final pending = c
          .read(sessionProvider.notifier)
          .onAccountSuspendedSignal();

      await c.read(sessionProvider.notifier).logout();
      gate.complete(
        const AccountSession(accountId: 'acc-1', restriction: _restriction),
      );
      await pending;

      expect(c.read(accountRestrictionStateProvider), isNull);
      expect(c.read(sessionProvider).isLoggedIn, isFalse);
    });

    test('換帳號登入後才回來的 GetSession 丟棄', () async {
      final gate = Completer<AccountSession>();
      final (c, _) = await loggedIn(() => gate.future);
      final pending = c
          .read(sessionProvider.notifier)
          .onAccountSuspendedSignal();

      await c.read(sessionProvider.notifier).login(accountId: 'acc-2');
      gate.complete(
        const AccountSession(accountId: 'acc-1', restriction: _restriction),
      );
      await pending;

      expect(c.read(accountRestrictionStateProvider), isNull);
    });

    test('同時多個訊號只打一次 GetSession，結束後可再核對', () async {
      final gate = Completer<AccountSession>();
      final (c, repo) = await loggedIn(() => gate.future);
      final notifier = c.read(sessionProvider.notifier);
      clearInteractions(repo);

      final all = <Future<void>>[
        notifier.onAccountSuspendedSignal(),
        notifier.onAccountSuspendedSignal(),
        notifier.onAccountSuspendedSignal(),
      ];
      gate.complete(
        const AccountSession(accountId: 'acc-1', restriction: _restriction),
      );
      await Future.wait(all);
      verify(() => repo.currentSession()).called(1);
      clearInteractions(repo);

      await notifier.onAccountSuspendedSignal();
      verify(() => repo.currentSession()).called(1);
    });
  });

  group('connectInterceptors 的 ACCOUNT_SUSPENDED', () {
    Future<int> call({required bool background, required String reason}) async {
      var signals = 0;
      final failure = reasonError(Code.permissionDenied, reason);
      final transport = FakeTransportBuilder()
          .unary(
            ContentPublicService.checkAppVersion,
            (req, ctx) => throw failure,
          )
          .unary(
            SellerAppService.refreshMyPaymentAccount,
            (req, ctx) => throw failure,
          )
          .build(
            interceptors: connectInterceptors(
              acceptLanguage: () => 'zh-TW',
              onAccountSuspended: () => signals++,
            ),
          );
      try {
        if (background) {
          await SellerAppServiceClient(
            transport,
          ).refreshMyPaymentAccount(RefreshMyPaymentAccountRequest());
        } else {
          await ContentPublicServiceClient(
            transport,
          ).checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));
        }
      } on ConnectException {
        // 錯誤原樣丟出。
      }
      return signals;
    }

    test('前景請求收到 ACCOUNT_SUSPENDED 通知一次', () async {
      expect(await call(background: false, reason: 'ACCOUNT_SUSPENDED'), 1);
    });

    test('背景寫入（RefreshMyPaymentAccount）收到 ACCOUNT_SUSPENDED 靜默略過', () async {
      expect(await call(background: true, reason: 'ACCOUNT_SUSPENDED'), 0);
    });

    test('其他原因不通知', () async {
      expect(await call(background: false, reason: 'SELLER_SUSPENDED'), 0);
    });
  });
}
