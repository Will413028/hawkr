import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/app_version_headers.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/clerk_session_provider.dart';
import 'package:hawkr_buyer/core/session/force_logout_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:package_info_plus/package_info_plus.dart';

import '../../support/fake_clerk_session.dart';

const _session = SessionModel(userId: 'acc_01J', nickname: 'Cora');

class _FakeSession extends Session {
  _FakeSession(this.initial);

  final SessionModel? initial;

  void clear() => state = const AsyncData(null);

  @override
  Future<SessionModel?> build() async => initial;
}

class _PendingTokenClerk extends FakeClerkSession {
  _PendingTokenClerk() : super(signedIn: true);
  final pending = Completer<String?>();
  final requested = Completer<void>();

  @override
  Future<String?> token() {
    requested.complete();
    return pending.future;
  }
}

final _authInterceptor = Provider<Interceptor>(sessionAuthInterceptor);
final _bootstrapInterceptor = Provider<Interceptor>(
  sessionBootstrapInterceptor,
);

/// Clerk 的 session token 接上 Connect 的身分：帶 Bearer，unauthenticated 時強制登出。假的單體只接受
/// `Bearer live`。
Future<({List<Headers> seen, ProviderContainer c})> _call(
  SessionModel? initial, {
  String token = 'live',
  List<Interceptor> Function(Interceptor auth)? interceptors,
}) async {
  final clerk = FakeClerkSession(signedIn: initial != null, tokenValue: token);
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(() => _FakeSession(initial)),
      clerkSessionProvider.overrideWith((ref) async => clerk),
    ],
  );
  addTearDown(container.dispose);
  await container.read(sessionProvider.future);

  final seen = <Headers>[];
  final auth = container.read(_authInterceptor);
  final transport = FakeTransportBuilder()
      .unary(ContentPublicService.checkAppVersion, (req, ctx) {
        seen.add(ctx.requestHeaders);
        if (ctx.requestHeaders['authorization'] != 'Bearer live') {
          throw ConnectException(Code.unauthenticated, 'session revoked');
        }
        return CheckAppVersionResponse();
      })
      .build(interceptors: interceptors?.call(auth) ?? [auth]);
  try {
    await ContentPublicServiceClient(
      transport,
    ).checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));
  } on ConnectException {
    // 失敗的情境由各測試檢查結果。
  }
  return (seen: seen, c: container);
}

void main() {
  test('實際 provider 接線：bootstrap 可取帳號，其餘未恢復請求不帶 Bearer', () async {
    if (!F.isInitialized) F.init(Flavor.dev);
    final seen = <String?>[];
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => _FakeSession(null)),
        clerkSessionProvider.overrideWith(
          (ref) async => FakeClerkSession(signedIn: true),
        ),
        connectHttpClientProvider.overrideWithValue((request) async {
          seen.add(request.header['authorization']);
          return HttpResponse(
            200,
            Headers()..['content-type'] = 'application/json',
            Stream.value(
              Uint8List.fromList(utf8.encode('{"account":{"id":"acc-1"}}')),
            ),
            Headers(),
          );
        }),
      ],
    );
    addTearDown(c.dispose);
    await c.read(sessionProvider.future);
    final response = await c
        .read(identityClientProvider)
        .getSession(GetSessionRequest());
    expect(response.account.id, 'acc-1');
    await expectLater(
      ContentPublicServiceClient(
        c.read(connectAuthTransportProvider),
      ).checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
      throwsA(
        isA<ConnectException>().having(
          (e) => e.code,
          'code',
          Code.unauthenticated,
        ),
      ),
    );
    expect(seen, ['Bearer live']);
  });

  test('GetSession 沒有 Bearer 時不送出、不把 401 當成帳號拒絕', () async {
    final session = _FakeSession(_session);
    final clerk = FakeClerkSession();
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => session),
        clerkSessionProvider.overrideWith((ref) async => clerk),
      ],
    );
    addTearDown(c.dispose);
    await c.read(sessionProvider.future);
    var calls = 0;
    final transport = FakeTransportBuilder()
        .unary(IdentityAppService.getSession, (req, ctx) {
          calls++;
          throw ConnectException(Code.unauthenticated, 'unauthenticated');
        })
        .build(interceptors: [c.read(_authInterceptor)]);
    await expectLater(
      IdentityAppServiceClient(transport).getSession(GetSessionRequest()),
      throwsA(
        isA<ConnectException>().having(
          (e) => e.code,
          'code',
          Code.unauthenticated,
        ),
      ),
    );
    expect(calls, 0);
    expect(clerk.calls, isEmpty);
    expect(c.read(forceLogoutProvider), isFalse);
  });

  test('取 token 期間登出，不帶先前帳號的 Bearer', () async {
    final session = _FakeSession(_session);
    final clerk = _PendingTokenClerk();
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => session),
        clerkSessionProvider.overrideWith((ref) async => clerk),
      ],
    );
    addTearDown(c.dispose);
    await c.read(sessionProvider.future);
    final tokenProvider = FutureProvider<String?>(
      (ref) => currentSessionToken(ref),
    );
    final token = c.read(tokenProvider.future);
    await clerk.requested.future;
    session.clear();
    clerk.pending.complete('old-account');
    expect(await token, isNull);
  });

  test('Clerk 存在但尚未恢復：只有 GetSession 帶 token', () async {
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => _FakeSession(null)),
        clerkSessionProvider.overrideWith(
          (ref) async => FakeClerkSession(signedIn: true),
        ),
      ],
    );
    addTearDown(c.dispose);
    await c.read(sessionProvider.future);
    final seen = <String?>[];
    final transport = FakeTransportBuilder()
        .unary(IdentityAppService.getSession, (req, ctx) {
          seen.add(ctx.requestHeaders['authorization']);
          return GetSessionResponse(account: Account(id: 'acc-1'));
        })
        .unary(ContentPublicService.checkAppVersion, (req, ctx) {
          seen.add(ctx.requestHeaders['authorization']);
          return CheckAppVersionResponse();
        })
        .build(interceptors: [c.read(_bootstrapInterceptor)]);
    await IdentityAppServiceClient(transport).getSession(GetSessionRequest());
    final gated = FakeTransportBuilder()
        .unary(ContentPublicService.checkAppVersion, (req, ctx) {
          seen.add(ctx.requestHeaders['authorization']);
          return CheckAppVersionResponse();
        })
        .build(interceptors: [c.read(_authInterceptor)]);
    await expectLater(
      ContentPublicServiceClient(
        gated,
      ).checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0')),
      throwsA(isA<ConnectException>()),
    );
    expect(seen, ['Bearer live']);
    final tokenProvider = FutureProvider<String?>(
      (ref) => currentSessionToken(ref),
    );
    expect(await c.read(tokenProvider.future), isNull);
  });

  test('帶 Clerk 的 session token，不登出', () async {
    final r = await _call(_session);

    expect(r.seen.map((h) => h['authorization']), ['Bearer live']);
    expect(r.c.read(forceLogoutProvider), isFalse);
  });

  test('session 失效：不重送，直接強制登出', () async {
    final r = await _call(_session, token: 'revoked');

    expect(r.seen.map((h) => h['authorization']), ['Bearer revoked']);
    expect(r.c.read(forceLogoutProvider), isTrue);
  });

  test('訪客：不帶 token，也不彈「登入已過期」', () async {
    final r = await _call(null);

    expect(r.seen, isEmpty);
    expect(r.c.read(forceLogoutProvider), isFalse);
  });

  test(
    'Connect 與其他 client 共用同一份 header：只有版本與 App 類型，不帶 X-Active-Role',
    () async {
      if (!F.isInitialized) F.init(Flavor.dev);
      PackageInfo.setMockInitialValues(
        appName: 'hawkr',
        packageName: 'io.github.will413028.hawkr.buyer',
        version: '1.2.3',
        buildNumber: '7',
        buildSignature: '',
      );
      await AppVersionHeaders.initialize();

      final r = await _call(
        _session,
        interceptors: (auth) => connectInterceptors(auth: auth),
      );

      expect(r.seen.single['x-active-role'], isNull);
      expect(r.seen.single['authorization'], 'Bearer live');
      expect(AppVersionHeaders.headers.keys.toSet(), {
        AppVersionHeaders.versionHeader,
        AppVersionHeaders.typeHeader,
      });
    },
  );
}
