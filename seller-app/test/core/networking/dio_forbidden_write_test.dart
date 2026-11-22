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
import 'package:hawkr_seller/features/chat/data/chat_session.dart';
import 'package:hawkr_seller/features/chat/data/chat_stream_client.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import '../../features/chat/support/chat_fakes.dart';
import 'package:mocktail/mocktail.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../features/auth/support/auth_test_fakes.dart';
import '../../support/fake_auth_session.dart';

/// 伺服器對停權帳號的 REST 寫入回 403：envelope 的 reason 是 ACCOUNT_SUSPENDED 才進入受限狀態，背景寫入靜默。
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
    required Map<String, Object?> body,
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
        chatSessionProvider.overrideWithValue(_NoopChatSession()),
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
    final dio = container.read(dioProvider)
      ..httpClientAdapter = _Forbidden(body);
    container.read(sessionProvider.notifier).state = const SessionModel(
      accountId: 'acc-1',
      isLoggedIn: true,
    );
    return (container, dio, repo);
  }

  const suspendedBody = {
    'code': 403,
    'message': 'forbidden',
    'reason': 'ACCOUNT_SUSPENDED',
  };

  test(
    'REST 403 帶 ACCOUNT_SUSPENDED（onResponse）：立即進入受限，GetSession 補上原因',
    () async {
      final (c, dio, _) = build(suspended: true, body: suspendedBody);

      await dio.post<dynamic>(
        '/api/v1/x',
        options: Options(validateStatus: (_) => true),
      );
      await pumpEventQueue();

      expect(c.read(accountRestrictionStateProvider)?.reason, '原文');
      expect(c.read(sessionProvider).isLoggedIn, isTrue);
    },
  );

  test('REST 403 帶 ACCOUNT_SUSPENDED（onError）：進入受限狀態，錯誤照常丟出', () async {
    final (c, dio, _) = build(suspended: true, body: suspendedBody);

    await expectLater(
      dio.delete<dynamic>('/api/v1/x'),
      throwsA(isA<DioException>()),
    );
    await pumpEventQueue();

    expect(c.read(accountRestrictionStateProvider)?.reason, '原文');
  });

  test('REST 403 帶 ACCOUNT_SUSPENDED 但 GetSession 沒有停權：仍然受限', () async {
    final (c, dio, _) = build(suspended: false, body: suspendedBody);

    await dio.post<dynamic>(
      '/api/v1/x',
      options: Options(validateStatus: (_) => true),
    );
    await pumpEventQueue();

    expect(c.read(accountRestrictionStateProvider), isNotNull);
  });

  test('沒有 reason 的一般 403：不進入受限，也不打 GetSession', () async {
    final (c, dio, repo) = build(
      suspended: true,
      body: const {'code': 403, 'message': 'forbidden'},
    );

    await dio.post<dynamic>(
      '/api/v1/x',
      options: Options(validateStatus: (_) => true),
    );
    await pumpEventQueue();

    verifyNever(() => repo.currentSession());
    expect(c.read(accountRestrictionStateProvider), isNull);
  });

  test('其他 reason 的 403：不進入受限', () async {
    final (c, dio, repo) = build(
      suspended: true,
      body: const {'code': 403, 'message': 'x', 'reason': 'SELLER_SUSPENDED'},
    );

    await dio.post<dynamic>(
      '/api/v1/x',
      options: Options(validateStatus: (_) => true),
    );
    await pumpEventQueue();

    verifyNever(() => repo.currentSession());
    expect(c.read(accountRestrictionStateProvider), isNull);
  });

  test('背景寫入（kBackgroundWriteExtra）收到 ACCOUNT_SUSPENDED：靜默略過', () async {
    final (c, dio, repo) = build(suspended: true, body: suspendedBody);

    await dio.post<dynamic>(
      '/api/v1/x',
      options: Options(
        extra: const {kBackgroundWriteExtra: true},
        validateStatus: (_) => true,
      ),
    );
    await pumpEventQueue();

    verifyNever(() => repo.currentSession());
    expect(c.read(accountRestrictionStateProvider), isNull);
  });

  test('登出時的註銷（kSkipAuthInterceptorExtra）收到 ACCOUNT_SUSPENDED：靜默略過', () async {
    final (c, dio, repo) = build(suspended: true, body: suspendedBody);

    await dio.delete<dynamic>(
      '/api/v1/x',
      options: Options(
        extra: const {kSkipAuthInterceptorExtra: true},
        validateStatus: (_) => true,
      ),
    );
    await pumpEventQueue();

    verifyNever(() => repo.currentSession());
    expect(c.read(accountRestrictionStateProvider), isNull);
  });
}

class _Forbidden implements HttpClientAdapter {
  _Forbidden(this.body);

  final Map<String, Object?> body;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async => ResponseBody.fromString(
    jsonEncode(body),
    403,
    headers: {
      Headers.contentTypeHeader: [Headers.jsonContentType],
    },
  );

  @override
  void close({bool force = false}) {}
}

class _NoopChatSession extends ChatSession {
  _NoopChatSession()
    : super(
        FakeChatRepository(),
        () => UiLocale.zhTW,
        client: ChatStreamClient(open: (_) => const Stream.empty()),
      );

  @override
  void stop() {}

  @override
  void start() {}
}
