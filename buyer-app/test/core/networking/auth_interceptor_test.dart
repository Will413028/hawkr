import 'dart:io';

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/app_version_headers.dart';
import 'package:hawkr_buyer/core/networking/auth_interceptor.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/auth_session_provider.dart';
import 'package:hawkr_buyer/core/session/force_logout_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_auth_session.dart';

const _session = SessionModel(userId: 'acc_01J', nickname: 'n');

class _FakeSession extends Session {
  _FakeSession(this.initial);

  final SessionModel? initial;

  @override
  Future<SessionModel?> build() async => initial;
}

/// Dio 的身分：帶 session token；取不到 token 時不送出；401 就強制登出，不重送。
void main() {
  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    // 初始化 header（版本與 App 類型），才分得出「不送 X-Active-Role」與「根本沒初始化」。
    PackageInfo.setMockInitialValues(
      appName: 'hawkr',
      packageName: 'io.github.will413028.hawkr.buyer',
      version: '1.2.3',
      buildNumber: '7',
      buildSignature: '',
    );
    await AppVersionHeaders.initialize();
  });

  Future<({Dio dio, ProviderContainer c, List<RequestOptions> sent})> harness(
    SessionModel? session, {
    required int status,
    FakeAuthSession? auth,
    Object? authStartError,
  }) async {
    final fake = auth ?? FakeAuthSession(signedIn: session != null);
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(() => _FakeSession(session)),
        authSessionProvider.overrideWith((ref) async {
          if (authStartError != null) throw authStartError;
          return fake;
        }),
      ],
    );
    addTearDown(container.dispose);
    await container.read(sessionProvider.future);
    final sent = <RequestOptions>[];
    final dio = container.read(dioProvider)
      ..httpClientAdapter = _RecordingAdapter(status, sent);
    return (dio: dio, c: container, sent: sent);
  }

  test('帶 session token 與版本 header，不送 X-Active-Role', () async {
    final h = await harness(_session, status: HttpStatus.ok);

    await h.dio.get<dynamic>('/app/buyers/me');

    expect(h.sent.single.headers['Authorization'], 'Bearer live');
    expect(h.sent.single.headers['X-App-Version'], '1.2.3');
    expect(h.sent.single.headers['X-App-Type'], isNotEmpty);
    expect(h.sent.single.headers.containsKey('X-Active-Role'), isFalse);
    expect(h.c.read(forceLogoutProvider), isFalse);
  });

  test('401：強制登出，不重送', () async {
    final h = await harness(_session, status: HttpStatus.unauthorized);

    await expectLater(
      h.dio.get<dynamic>('/app/buyers/me'),
      throwsA(isA<DioException>()),
    );

    expect(h.sent, hasLength(1));
    expect(h.c.read(forceLogoutProvider), isTrue);
  });

  test('訪客的 401：不帶 token，也不彈「登入已過期」', () async {
    final h = await harness(null, status: HttpStatus.unauthorized);

    await expectLater(
      h.dio.get<dynamic>('/app/orders'),
      throwsA(isA<DioException>()),
    );

    expect(h.sent.single.headers.containsKey('Authorization'), isFalse);
    expect(h.c.read(forceLogoutProvider), isFalse);
  });

  test('取不到 token（連不上 session token）：不送出，以連線錯誤結束，不登出', () async {
    final h = await harness(
      _session,
      status: HttpStatus.ok,
      auth: FakeAuthSession(signedIn: true)
        ..tokenError = const SocketException('offline'),
    );

    await expectLater(
      h.dio.get<dynamic>('/app/buyers/me'),
      throwsA(
        isA<DioException>().having(
          (e) => e.type,
          'type',
          DioExceptionType.connectionError,
        ),
      ),
    );

    expect(h.sent, isEmpty);
    expect(h.c.read(forceLogoutProvider), isFalse);
  });

  test('session token 無法啟動：當成訪客，照常送出、不帶 token', () async {
    final h = await harness(
      null,
      status: HttpStatus.ok,
      authStartError: StateError('keychain'),
    );

    await h.dio.get<dynamic>('/public/products');

    expect(h.sent.single.headers.containsKey('Authorization'), isFalse);
  });

  test('其他錯誤與 session 無關，不登出', () async {
    final h = await harness(_session, status: HttpStatus.forbidden);

    await expectLater(
      h.dio.get<dynamic>('/app/buyers/me'),
      throwsA(isA<DioException>()),
    );

    expect(h.c.read(forceLogoutProvider), isFalse);
  });

  test('authInterceptorProvider 是 dioProvider 掛的同一個實例', () async {
    final h = await harness(_session, status: HttpStatus.ok);

    expect(h.dio.interceptors, contains(h.c.read(authInterceptorProvider)));
  });
}

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter(this.status, this.sent);

  final int status;
  final List<RequestOptions> sent;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    sent.add(options);
    return ResponseBody.fromString(
      '{"code":0,"data":{}}',
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
