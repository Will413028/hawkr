import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/session/account_restriction.dart';
import 'package:hawkr_seller/core/session/auth_session_provider.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../features/auth/support/auth_test_fakes.dart';
import '../../support/fake_auth_session.dart';

/// 伺服器對停權帳號的 REST 寫入回 403（envelope 沒有 reason）：Dio 以 GetSession 核對，確認停權才進入受限狀態。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
          (call) async {
            switch (call.method) {
              case 'readAll':
                return <String, String>{};
              case 'containsKey':
                return false;
              default:
                return null;
            }
          },
        );
  });

  setUp(() => SharedPreferences.setMockInitialValues({}));

  (ProviderContainer, Dio, FakeAuthRepository) build({
    required bool suspended,
  }) {
    final repo = FakeAuthRepository();
    when(() => repo.currentSession()).thenAnswer(
      (_) async => AccountSession(
        accountId: 'acc-1',
        restriction: suspended
            ? const AccountRestriction(
                category: PolicyCategory.POLICY_CATEGORY_UNSPECIFIED,
                reason: '原文',
              )
            : null,
      ),
    );
    final container = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(_NoopChatRepository()),
        postLoginSideEffectsProvider.overrideWithValue(
          const NoopPostLoginSideEffects(),
        ),
        authSessionProvider.overrideWith(
          (ref) async => FakeAuthSession(signedIn: true),
        ),
        authRepositoryProvider.overrideWithValue(repo),
      ],
    );
    addTearDown(container.dispose);
    final dio = container.read(dioProvider)..httpClientAdapter = _Forbidden();
    container.read(sessionProvider.notifier).state = const SessionModel(
      accountId: 'acc-1',
      isLoggedIn: true,
    );
    return (container, dio, repo);
  }

  test('寫入的 403（onResponse）：GetSession 回報停權 → 進入受限狀態', () async {
    final (c, dio, _) = build(suspended: true);

    await dio.post<dynamic>(
      '/api/v1/x',
      options: Options(validateStatus: (_) => true),
    );
    await pumpEventQueue();

    expect(c.read(accountRestrictionStateProvider)?.reason, '原文');
    expect(c.read(sessionProvider).isLoggedIn, isTrue);
  });

  test('寫入的 403（onError）：GetSession 回報停權 → 進入受限狀態，錯誤照常丟出', () async {
    final (c, dio, _) = build(suspended: true);

    await expectLater(
      dio.delete<dynamic>('/api/v1/x'),
      throwsA(isA<DioException>()),
    );
    await pumpEventQueue();

    expect(c.read(accountRestrictionStateProvider)?.reason, '原文');
  });

  test('寫入的 403 但 GetSession 沒有停權 → 不進入受限狀態', () async {
    final (c, dio, repo) = build(suspended: false);

    await dio.post<dynamic>(
      '/api/v1/x',
      options: Options(validateStatus: (_) => true),
    );
    await pumpEventQueue();

    verify(() => repo.currentSession()).called(1);
    expect(c.read(accountRestrictionStateProvider), isNull);
  });

  test('GET 的 403 不核對', () async {
    final (c, dio, repo) = build(suspended: true);

    await dio.get<dynamic>(
      '/api/v1/x',
      options: Options(validateStatus: (_) => true),
    );
    await pumpEventQueue();

    verifyNever(() => repo.currentSession());
    expect(c.read(accountRestrictionStateProvider), isNull);
  });
}

class _Forbidden implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromString(
    jsonEncode({'code': 403, 'message': 'forbidden'}),
    403,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );

  @override
  void close({bool force = false}) {}
}

class _NoopChatRepository extends ChatRepository {
  _NoopChatRepository() : super(Dio());

  @override
  void stopPolling() {}

  @override
  void startPolling({Duration interval = const Duration(seconds: 1)}) {}

  @override
  void useHttpClient(Dio dio) {}
}
