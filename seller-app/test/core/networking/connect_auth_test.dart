import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/app_version_headers.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/auth_session_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/fake_auth_session.dart';

class _FakeSession extends Session {
  _FakeSession({this.loggedIn = true});
  final bool loggedIn;
  int logouts = 0;
  int suspendedSignals = 0;

  void clear() => state = const SessionModel();

  @override
  SessionModel build() =>
      SessionModel(accountId: loggedIn ? 'acc-1' : null, isLoggedIn: loggedIn);

  @override
  Future<void> logout() async => logouts++;

  @override
  void onSellerSuspendedSignal() => suspendedSignals++;
}

class _PendingTokenAuth extends FakeAuthSession {
  _PendingTokenAuth() : super(signedIn: true);
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

/// session token（`live`）接上 Connect 的身分。假的伺服器只接受 [valid]；每次送出記下
/// authorization 與 X-Active-Role。
Future<({List<String?> auth, List<String?> roles, _FakeSession session})>
_call({required String valid}) async {
  final session = _FakeSession();
  final authSession = FakeAuthSession(signedIn: true);
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(() => session),
      authSessionProvider.overrideWith((ref) async => authSession),
    ],
  );
  addTearDown(container.dispose);

  final auth = <String?>[];
  final roles = <String?>[];
  final transport = FakeTransportBuilder()
      .unary(ContentPublicService.checkAppVersion, (req, ctx) {
        auth.add(ctx.requestHeaders['authorization']);
        roles.add(ctx.requestHeaders['x-active-role']);
        if (ctx.requestHeaders['authorization'] != 'Bearer $valid') {
          throw ConnectException(Code.unauthenticated, 'expired');
        }
        return CheckAppVersionResponse();
      })
      .build(
        interceptors: connectInterceptors(
          auth: container.read(_authInterceptor),
        ),
      );
  try {
    await ContentPublicServiceClient(
      transport,
    ).checkAppVersion(CheckAppVersionRequest(clientVersion: '1.0.0'));
  } on ConnectException {
    // 失敗的情境由各測試檢查結果。
  }
  return (auth: auth, roles: roles, session: session);
}

void main() {
  test('實際 provider 接線：SELLER_SUSPENDED 轉成停權訊號，其他 reason 不轉', () async {
    if (!F.isInitialized) F.init(Flavor.dev);
    Future<(int, Object?)> call(String reason) async {
      final session = _FakeSession();
      final c = ProviderContainer(
        overrides: [
          sessionProvider.overrideWith(() => session),
          authSessionProvider.overrideWith(
            (ref) async => FakeAuthSession(signedIn: true),
          ),
          connectHttpClientProvider.overrideWithValue((request) async {
            final info =
                (ErrorInfo()
                      ..reason = reason
                      ..domain = 'hawkr')
                    .writeToBuffer();
            final body = jsonEncode({
              'code': 'permission_denied',
              'message': 'suspended',
              'details': [
                {'type': 'google.rpc.ErrorInfo', 'value': base64.encode(info)},
              ],
            });
            return HttpResponse(
              403,
              Headers()..['content-type'] = 'application/json',
              Stream.value(Uint8List.fromList(utf8.encode(body))),
              Headers(),
            );
          }),
        ],
      );
      addTearDown(c.dispose);
      c.read(sessionProvider);
      Object? error;
      try {
        await c
            .read(identityClientProvider)
            .listSessions(ListSessionsRequest());
      } catch (e) {
        error = e;
      }
      return (session.suspendedSignals, error);
    }

    final (signals, error) = await call('SELLER_SUSPENDED');
    expect(signals, 1);
    expect(error, isA<ConnectException>());
    expect((error! as ConnectException).code, Code.permissionDenied);
    expect((await call('VERSION_CONFLICT')).$1, 0);
  });

  test('實際 provider 接線：bootstrap 可取帳號，其餘未恢復請求不帶 Bearer', () async {
    if (!F.isInitialized) F.init(Flavor.dev);
    final seen = <String?>[];
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => _FakeSession(loggedIn: false)),
        authSessionProvider.overrideWith(
          (ref) async => FakeAuthSession(signedIn: true),
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
    c.read(sessionProvider);
    final response = await c
        .read(sessionIdentityClientProvider)
        .getSession(GetSessionRequest());
    expect(response.account.id, 'acc-1');
    await expectLater(
      c.read(identityClientProvider).listSessions(ListSessionsRequest()),
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
    final session = _FakeSession();
    final auth = FakeAuthSession();
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => session),
        authSessionProvider.overrideWith((ref) async => auth),
      ],
    );
    addTearDown(c.dispose);
    c.read(sessionProvider);
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
    expect(auth.calls, isEmpty);
    expect(session.logouts, 0);
  });

  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  test('取 token 期間登出，不帶先前帳號的 Bearer', () async {
    final session = _FakeSession();
    final auth = _PendingTokenAuth();
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => session),
        authSessionProvider.overrideWith((ref) async => auth),
      ],
    );
    addTearDown(c.dispose);
    c.read(sessionProvider);
    final tokenProvider = FutureProvider<String?>(
      (ref) => currentSessionToken(ref),
    );
    final token = c.read(tokenProvider.future);
    await auth.requested.future;
    session.clear();
    auth.pending.complete('old-account');
    expect(await token, isNull);
  });

  test('session token 存在但尚未恢復：只有 GetSession 帶 token', () async {
    final c = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => _FakeSession(loggedIn: false)),
        authSessionProvider.overrideWith(
          (ref) async => FakeAuthSession(signedIn: true),
        ),
      ],
    );
    addTearDown(c.dispose);
    c.read(sessionProvider);
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

  test('帶 session token、不帶 X-Active-Role（Connect 不讀視角），成功時不登出', () async {
    final r = await _call(valid: 'live');

    expect(r.auth, ['Bearer live']);
    expect(r.roles, [null]);
    expect(appConnectHeaders().containsKey(kActiveRoleHeader), isFalse);
    expect(appVersionHeaders()[kActiveRoleHeader], 'SELLER');
    expect(r.session.logouts, 0);
  });

  test('unauthenticated：直接登出，不重送', () async {
    final r = await _call(valid: 'other');

    expect(r.auth, ['Bearer live']);
    expect(r.session.logouts, 1);
  });
}
