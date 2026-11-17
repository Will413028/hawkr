import 'dart:async';
import 'dart:io';

import 'package:dio/dio.dart' hide Headers, Interceptor;
import 'package:dio/dio.dart' as dio_lib show Headers;
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/account_restriction.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_buyer/features/chat/data/chat_repository.dart';
import 'package:hawkr_buyer/core/session/auth_session_provider.dart';
import 'package:hawkr_buyer/core/session/force_logout_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_auth_session.dart';

const _session = SessionModel(userId: 'acc_01J', nickname: 'n');

class _FakeSession extends Session {
  @override
  Future<SessionModel?> build() async => _session;
}

class _Auth extends Fake implements AuthRepository {
  int fetches = 0;
  Future<AccountSnapshot> Function()? answer;

  @override
  Future<AccountSnapshot> currentAccount() async {
    fetches++;
    if (answer != null) return answer!();
    return const AccountSnapshot(
      accountId: 'acc_01J',
      suspension: AccountSuspension(reason: '多次張貼廣告'),
    );
  }
}

ConnectException _denied({required bool suspended}) => ConnectException(
  Code.permissionDenied,
  'denied',
  details: [
    if (suspended)
      ErrorDetail(
        'google.rpc.ErrorInfo',
        (ErrorInfo()
              ..reason = 'ACCOUNT_SUSPENDED'
              ..domain = 'hawkr')
            .writeToBuffer(),
      ),
  ],
);

class _Status implements HttpClientAdapter {
  _Status(this.status, this.sent, [this.body]);

  final int status;
  final String? body;
  final List<RequestOptions> sent;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    sent.add(options);
    return ResponseBody.fromString(
      body ?? '{"code":$status,"message":"x"}',
      status,
      headers: {
        dio_lib.Headers.contentTypeHeader: [dio_lib.Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  ({ProviderContainer c, _Auth auth}) harness() {
    final auth = _Auth();
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_FakeSession.new),
        authSessionProvider.overrideWith(
          (ref) async => FakeAuthSession(signedIn: true),
        ),
        authRepositoryProvider.overrideWithValue(auth),
      ],
    );
    addTearDown(c.dispose);
    return (c: c, auth: auth);
  }

  BuyerAppServiceClient client(ProviderContainer c, ConnectException error) {
    final suspension = c.read(_suspension);
    final transport = FakeTransportBuilder()
        .unary(BuyerAppService.addFavorite, (_, _) => throw error)
        .unary(BuyerAppService.recordVisit, (_, _) => throw error)
        .build(interceptors: [suspension]);
    return BuyerAppServiceClient(transport);
  }

  group('restriction state', () {
    test('GetSession 成功且沒有停權：解除受限狀態', () async {
      final h = harness();
      h.auth.answer = () async => const AccountSnapshot(accountId: 'acc_01J');
      h.c.read(accountRestrictionProvider.notifier).rejected();

      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNull);
    });

    test('GetSession 失敗：保留原狀態', () async {
      final h = harness();
      h.auth.answer = () async => throw StateError('down');
      h.c.read(accountRestrictionProvider.notifier).rejected();

      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNotNull);
    });

    test('補讀細節期間登出：回來的結果不寫回', () async {
      final h = harness();
      final pending = Completer<AccountSnapshot>();
      h.auth.answer = () => pending.future;
      final n = h.c.read(accountRestrictionProvider.notifier)..rejected();
      await Future<void>.delayed(Duration.zero);
      expect(h.auth.fetches, 1);

      n.clear();
      pending.complete(
        const AccountSnapshot(
          accountId: 'acc_01J',
          suspension: AccountSuspension(reason: '舊帳號'),
        ),
      );
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNull);
    });

    test('補讀細節期間換帳號：不套用前一個帳號的結果', () async {
      final h = harness();
      final pending = Completer<AccountSnapshot>();
      h.auth.answer = () => pending.future;
      final n = h.c.read(accountRestrictionProvider.notifier)..rejected();
      await Future<void>.delayed(Duration.zero);

      n.apply(null);
      pending.complete(
        const AccountSnapshot(
          accountId: 'a',
          suspension: AccountSuspension(reason: 'A'),
        ),
      );
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNull);
    });
  });

  group('Connect', () {
    test('使用者主動的寫入收到 ACCOUNT_SUSPENDED：進受限狀態、請求提示、不登出', () async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      await expectLater(
        client(
          h.c,
          _denied(suspended: true),
        ).addFavorite(buyer.AddFavoriteRequest(productId: '1')),
        throwsA(isA<ConnectException>()),
      );
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNotNull);
      expect(h.c.read(accountRestrictionProvider)?.reason, '多次張貼廣告');
      expect(h.c.read(suspendedActionNoticeProvider), 1);
      expect(h.c.read(forceLogoutProvider), isFalse);
      expect(h.c.read(sessionProvider).value, isNotNull);
    });

    test('背景寫入（RecordVisit）收到 ACCOUNT_SUSPENDED：靜默，不進受限狀態、不提示', () async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      await expectLater(
        client(
          h.c,
          _denied(suspended: true),
        ).recordVisit(buyer.RecordVisitRequest(productId: '1')),
        throwsA(isA<ConnectException>()),
      );
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNull);
      expect(h.c.read(suspendedActionNoticeProvider), 0);
      expect(h.auth.fetches, 0);
    });

    test('沒有 ACCOUNT_SUSPENDED 的 permission_denied 不是停權', () async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      await expectLater(
        client(
          h.c,
          _denied(suspended: false),
        ).addFavorite(buyer.AddFavoriteRequest(productId: '1')),
        throwsA(isA<ConnectException>()),
      );
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNull);
      expect(h.c.read(suspendedActionNoticeProvider), 0);
    });
  });

  group('Dio（舊 REST）', () {
    Future<({Dio dio, ProviderContainer c, _Auth auth})> dioHarness(
      int status, {
      String body = '{"code":403,"message":"x","reason":"ACCOUNT_SUSPENDED"}',
    }) async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      final dio = h.c.read(dioProvider)
        ..httpClientAdapter = _Status(status, [], body);
      return (dio: dio, c: h.c, auth: h.auth);
    }

    Future<void> send(Dio dio, String method, String path) async {
      try {
        await dio.request<dynamic>(path, options: Options(method: method));
      } on DioException {
        // 結果由各測試檢查。
      }
      await Future<void>.delayed(Duration.zero);
    }

    test('403 帶 ACCOUNT_SUSPENDED：直接進受限狀態並提示，不登出', () async {
      final h = await dioHarness(HttpStatus.forbidden);
      await send(h.dio, 'POST', '/app/favorites');

      expect(h.c.read(accountRestrictionProvider), isNotNull);
      expect(h.c.read(suspendedActionNoticeProvider), 1);
      expect(h.c.read(forceLogoutProvider), isFalse);
      // 只有 rejected() 補讀細節一次，不再為確認 403 多打一次 GetSession。
      expect(h.auth.fetches, 1);
    });

    test('403 沒有 reason：不是停權', () async {
      final h = await dioHarness(
        HttpStatus.forbidden,
        body: '{"code":403,"message":"x"}',
      );
      await send(h.dio, 'POST', '/app/favorites');

      expect(h.c.read(accountRestrictionProvider), isNull);
      expect(h.auth.fetches, 0);
    });

    test('GET 的 403 沒有 reason：不是停權', () async {
      final h = await dioHarness(
        HttpStatus.forbidden,
        body: '{"code":403,"message":"x"}',
      );
      await send(h.dio, 'GET', '/app/buyers/me');

      expect(h.c.read(accountRestrictionProvider), isNull);
      expect(h.auth.fetches, 0);
    });
  });

  group('自己放行非 2xx 的呼叫', () {
    test('AI 小幫手送出訊息收到 403 ACCOUNT_SUSPENDED：進受限狀態並提示', () async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      final dio = Dio(BaseOptions(baseUrl: 'https://x.test'))
        ..httpClientAdapter = _Status(
          403,
          [],
          '{"code":403,"message":"x","reason":"ACCOUNT_SUSPENDED"}',
        );
      final repo = AiAssistantRepository(
        dio,
        onSuspended: () =>
            h.c.read(accountRestrictionProvider.notifier).rejected(),
      );

      await expectLater(
        repo.streamChat(message: 'hi').drain<void>(),
        throwsA(anything),
      );
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNotNull);
      expect(h.c.read(suspendedActionNoticeProvider), 1);
    });

    test('AI 小幫手的 403 沒有 reason：不是停權', () async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      final dio = Dio(BaseOptions(baseUrl: 'https://x.test'))
        ..httpClientAdapter = _Status(403, [], '{"code":403,"message":"x"}');
      final repo = AiAssistantRepository(
        dio,
        onSuspended: () =>
            h.c.read(accountRestrictionProvider.notifier).rejected(),
      );

      await expectLater(
        repo.streamChat(message: 'hi').drain<void>(),
        throwsA(anything),
      );

      expect(h.c.read(accountRestrictionProvider), isNull);
    });

    test('聊天連線（ws-ticket）收到 403 ACCOUNT_SUSPENDED：進受限狀態並提示', () async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      final repo = ChatRepository.withDio(
        sessionToken: () async => 'tok',
        accountId: () => 'acc_01J',
        dio: Dio(BaseOptions(validateStatus: (_) => true))
          ..httpClientAdapter = _Status(
            403,
            [],
            '{"code":403,"message":"x","reason":"ACCOUNT_SUSPENDED"}',
          ),
        onSuspended: () =>
            h.c.read(accountRestrictionProvider.notifier).rejected(),
      );
      addTearDown(repo.dispose);

      await repo.connect();
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNotNull);
      expect(h.c.read(suspendedActionNoticeProvider), 1);
    });

    test('聊天連線（ws-ticket）收到 403 沒有 reason：不是停權', () async {
      final h = harness();
      await h.c.read(sessionProvider.future);
      final repo = ChatRepository.withDio(
        sessionToken: () async => 'tok',
        accountId: () => 'acc_01J',
        dio: Dio(BaseOptions(validateStatus: (_) => true))
          ..httpClientAdapter = _Status(403, [], '{"code":403,"message":"x"}'),
        onSuspended: () =>
            h.c.read(accountRestrictionProvider.notifier).rejected(),
      );
      addTearDown(repo.dispose);

      await repo.connect();
      await Future<void>.delayed(Duration.zero);

      expect(h.c.read(accountRestrictionProvider), isNull);
    });
  });
}

final _suspension = Provider<Interceptor>(suspensionInterceptor);
