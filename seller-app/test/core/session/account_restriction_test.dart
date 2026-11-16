import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/account_suspended_interceptor.dart';
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

  group('accountSuspendedInterceptor', () {
    Future<int> call({
      required bool background,
      required bool suspended,
    }) async {
      var signals = 0;
      final failure = suspended
          ? reasonError(Code.permissionDenied, 'ACCOUNT_SUSPENDED')
          : reasonError(Code.permissionDenied, 'SELLER_SUSPENDED');
      final transport = FakeTransportBuilder()
          .unary(
            ContentPublicService.checkAppVersion,
            (req, ctx) => throw failure,
          )
          .unary(
            SocialPublicService.recordPostView,
            (req, ctx) => throw failure,
          )
          .build(interceptors: [accountSuspendedInterceptor(() => signals++)]);
      try {
        if (background) {
          await SocialPublicServiceClient(
            transport,
          ).recordPostView(RecordPostViewRequest(postId: 'p1'));
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
      expect(await call(background: false, suspended: true), 1);
    });

    test('背景寫入收到 ACCOUNT_SUSPENDED 靜默略過', () async {
      expect(await call(background: true, suspended: true), 0);
    });

    test('其他原因不通知', () async {
      expect(await call(background: false, suspended: false), 0);
    });
  });
}
