import 'dart:async';

import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_auth/hawkr_auth.dart' show SignInFailed;
import 'package:hawkr_buyer/features/order/data/receipt_photo_local_service.dart';
import 'package:hawkr_buyer/features/notification/data/push/background_handler.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail, HeadersExtension;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/storage/auth_storage.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/auth_session_provider.dart';
import 'package:hawkr_buyer/features/auth/presentation/auth_controller.dart';
import 'package:hawkr_buyer/features/notification/data/push/manager.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/services.dart';
import 'package:hawkr_buyer/shared/utils/hk_log.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_auth_session.dart';
import '../../support/firebase_core_mock.dart';
import '../../support/secure_storage_mock.dart';

/// 登入：原生 SDK 登入換到 session token，再以 GetSession 取得帳號寫入 session。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  setUp(() {
    mockFirebaseCore();
    PushNotificationManager.e2eDisabled = true;
    addTearDown(() => PushNotificationManager.e2eDisabled = false);
    SharedPreferences.setMockInitialValues({});
    mockSecureStorage({});
  });

  ({ProviderContainer c, FakeAuthSession auth, List<String?> getSessions})
  harness({
    FakeAuthSession? authSession,
    _ProfileAdapter? adapter,
    FutureOr<GetSessionResponse> Function()? getSession,
  }) {
    final auth = authSession ?? FakeAuthSession();
    final getSessions = <String?>[];
    final container = ProviderContainer(
      overrides: [
        authSessionProvider.overrideWith((ref) async => auth),
        connectAuthTransportProvider.overrideWith(
          (ref) => FakeTransportBuilder()
              .unary(
                BuyerAppService.getProfile,
                (_, _) => buyer.GetProfileResponse(
                  profile: buyer.Profile(id: 'buyer-7002', displayName: 'Cora'),
                ),
              )
              .build(),
        ),
        // GetSession 經過真的身分 interceptor：驗證登入之後的請求帶 session token。
        identityClientProvider.overrideWith(
          (ref) => IdentityAppServiceClient(
            FakeTransportBuilder()
                .unary(IdentityAppService.getSession, (req, ctx) {
                  getSessions.add(ctx.requestHeaders['authorization']);
                  return getSession?.call() ??
                      GetSessionResponse(account: Account(id: 'acc_01J8Z'));
                })
                .build(interceptors: [sessionBootstrapInterceptor(ref)]),
          ),
        ),
        dioProvider.overrideWithValue(
          Dio()..httpClientAdapter = adapter ?? _ProfileAdapter(),
        ),
      ],
    );
    addTearDown(container.dispose);
    return (c: container, auth: auth, getSessions: getSessions);
  }

  Future<bool> signIn(ProviderContainer c, OAuthProvider provider) =>
      c.read(authControllerProvider.notifier).signInWith(provider);

  test('Google：原生登入換到 session token，GetSession 帶這個 token，寫入帳號與暱稱', () async {
    final h = harness();

    expect(await signIn(h.c, OAuthProvider.google), isTrue);

    expect(h.auth.calls, ['signIn google']);
    expect(h.getSessions, ['Bearer live']);
    final session = h.c.read(sessionProvider).value;
    expect(session?.userId, 'acc_01J8Z');
    expect(session?.nickname, 'Cora');
  });

  test('LINE：同樣經原生登入', () async {
    final h = harness();

    expect(await signIn(h.c, OAuthProvider.line), isTrue);

    expect(h.auth.calls, ['signIn line']);
    expect(h.c.read(sessionProvider).value?.userId, 'acc_01J8Z');
  });

  test('使用者取消：回 false，不呼叫 GetSession，沒有 session', () async {
    final h = harness(authSession: FakeAuthSession()..signInResult = false);

    expect(await signIn(h.c, OAuthProvider.google), isFalse);
    expect(await signIn(h.c, OAuthProvider.line), isFalse);

    expect(h.getSessions, isEmpty);
    expect(h.c.read(sessionProvider).value, isNull);
  });

  test('GetSession 停權錯誤：清除 session token、往外丟，不寫入帳號', () async {
    final h = harness(
      getSession: () => throw ConnectException(
        Code.permissionDenied,
        'account suspended',
        details: [
          ErrorDetail(
            'google.rpc.LocalizedMessage',
            (LocalizedMessage()
                  ..locale = 'zh-TW'
                  ..message = '帳號已停權')
                .writeToBuffer(),
          ),
        ],
      ),
    );

    await expectLater(
      signIn(h.c, OAuthProvider.google),
      throwsA(
        isA<SuspendedException>().having((e) => e.message, "message", "帳號已停權"),
      ),
    );

    expect(h.auth.calls, ['signIn google', 'clear']);
    expect(h.auth.isSignedIn, isFalse);
    expect(h.c.read(sessionProvider).value, isNull);
  });

  test('暫時失敗保留 session token，再次登入直接重試 GetSession', () async {
    var fail = true;
    final h = harness(
      getSession: () {
        if (fail) throw ConnectException(Code.internal, 'temporary');
        return GetSessionResponse(account: Account(id: 'recovered'));
      },
    );
    await expectLater(
      signIn(h.c, OAuthProvider.google),
      throwsA(isA<NetworkException>()),
    );
    expect(h.auth.isSignedIn, isTrue);
    fail = false;
    expect(await signIn(h.c, OAuthProvider.google), isTrue);
    expect(h.auth.calls, ['signIn google']);
    expect(h.c.read(sessionProvider).value?.userId, 'recovered');
  });

  test('主動登入暫時失敗後由背景完成，訪客購物車只合併一次', () async {
    var calls = 0;
    final adapter = _ProfileAdapter();
    final guestId = await AuthStorage.guestCartId();
    final h = harness(
      adapter: adapter,
      getSession: () {
        if (++calls == 1) {
          throw ConnectException(Code.unavailable, 'unavailable');
        }
        return GetSessionResponse(account: Account(id: 'recovered'));
      },
    );
    await expectLater(
      signIn(h.c, OAuthProvider.google),
      throwsA(isA<NetworkException>()),
    );
    expect(adapter.merges, isEmpty);
    await _until(
      () async =>
          h.c.read(sessionProvider).value?.nickname == 'Cora' &&
          adapter.merges.isNotEmpty,
    );
    expect(h.c.read(sessionProvider).value?.userId, 'recovered');
    expect(h.c.read(sessionProvider).value?.nickname, 'Cora');
    expect(adapter.merges, [guestId]);
    await Future<void>.delayed(const Duration(milliseconds: 1600));
    expect(adapter.merges, [guestId]);
    expect(calls, 2);
  });

  test('冷啟動 GetSession 回 unauthenticated：清除 token，沒有 session', () async {
    final auth = FakeAuthSession(signedIn: true);
    final h = harness(
      authSession: auth,
      getSession: () => throw ConnectException(Code.unauthenticated, 'revoked'),
    );
    expect(await h.c.read(sessionProvider.future), isNull);
    expect(auth.isSignedIn, isFalse);
    expect(auth.calls, ['clear']);
  });

  test('原生登入失敗：SignInFailed 往外丟（文字給使用者看），不呼叫 GetSession', () async {
    final auth = FakeAuthSession()
      ..signInError = const SignInFailed('登入失敗，請稍後再試');
    final h = harness(authSession: auth);
    await expectLater(
      signIn(h.c, OAuthProvider.google),
      throwsA(
        isA<SignInFailed>().having(
          (e) => e.toString(),
          'message',
          '登入失敗，請稍後再試',
        ),
      ),
    );
    expect(h.getSessions, isEmpty);
    expect(h.c.read(sessionProvider).value, isNull);
  });

  for (final method in ['read', 'write']) {
    for (final restore in [true, false]) {
      test(
        'cache owner $method failure preserves session and cache: restore=$restore',
        () async {
          final messages = <String?>[];
          final sink = HKLog.sentryBreadcrumbSink;
          HKLog.sentryBreadcrumbSink = (entry) => messages.add(entry.message);
          addTearDown(() => HKLog.sentryBreadcrumbSink = sink);
          TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
              .setMockMethodCallHandler(
                const MethodChannel(
                  'plugins.it_nomads.com/flutter_secure_storage',
                ),
                (call) async {
                  final args = call.arguments as Map;
                  if (args['key'] == 'cache_owner') {
                    if (call.method == method) {
                      throw PlatformException(code: 'keystore_reset');
                    }
                    if (call.method == 'read') return 'acc_01J8Z';
                  }
                  return null;
                },
              );
          final photos = SharedPreferencesReceiptPhotoLocalService();
          await photos.write('order-1', 'photo');
          final prefs = await SharedPreferences.getInstance();
          await prefs.setStringList(fcmProcessedIdsKey, ['push-1']);
          final h = harness(authSession: FakeAuthSession(signedIn: restore));
          if (restore) {
            expect(
              (await h.c.read(sessionProvider.future))?.userId,
              'acc_01J8Z',
            );
          } else {
            expect(await signIn(h.c, OAuthProvider.google), isTrue);
          }
          expect(h.c.read(sessionProvider).value?.userId, 'acc_01J8Z');
          expect(await photos.read('order-1'), 'photo');
          expect(prefs.getStringList(fcmProcessedIdsKey), ['push-1']);
          expect(messages, contains(contains('cache owner')));
          await Future<void>.delayed(Duration.zero);
        },
      );
    }
  }

  test('同帳號冷啟動暫時失敗後點登入，保留 user cache', () async {
    await AuthStorage.setCacheOwner('acc_01J8Z');
    final photos = SharedPreferencesReceiptPhotoLocalService();
    await photos.write('order-1', 'photo');
    final prefs = await SharedPreferences.getInstance();
    await prefs.setStringList(fcmProcessedIdsKey, ['push-1']);
    var calls = 0;
    final h = harness(
      authSession: FakeAuthSession(signedIn: true),
      getSession: () {
        if (++calls == 1) throw ConnectException(Code.unavailable, 'offline');
        return GetSessionResponse(account: Account(id: 'acc_01J8Z'));
      },
    );
    expect(await h.c.read(sessionProvider.future), isNull);
    expect(await signIn(h.c, OAuthProvider.google), isTrue);
    await Future<void>.delayed(Duration.zero);
    expect(await photos.read('order-1'), 'photo');
    expect(prefs.getStringList(fcmProcessedIdsKey), ['push-1']);
  });

  test('切換帳號才清 user cache', () async {
    await AuthStorage.setCacheOwner('previous');
    final photos = SharedPreferencesReceiptPhotoLocalService();
    await photos.write('order-1', 'photo');
    final h = harness();
    await signIn(h.c, OAuthProvider.google);
    await Future<void>.delayed(Duration.zero);
    expect(await photos.read('order-1'), isNull);
    expect(await AuthStorage.cacheOwner(), 'acc_01J8Z');
  });

  test('前景與三次背景都失敗後，冷啟動仍合併並清除 guest cart', () async {
    final guest = await AuthStorage.guestCartId();
    var calls = 0;
    final first = harness(
      getSession: () {
        calls++;
        throw ConnectException(Code.unavailable, 'offline');
      },
    );
    await expectLater(
      signIn(first.c, OAuthProvider.google),
      throwsA(isA<NetworkException>()),
    );
    await Future<void>.delayed(const Duration(milliseconds: 10900));
    expect(calls, 4);
    expect(await AuthStorage.existingGuestCartId(), guest);
    final adapter = _ProfileAdapter();
    final second = harness(authSession: first.auth, adapter: adapter);
    expect((await second.c.read(sessionProvider.future))?.userId, 'acc_01J8Z');
    await _until(() async => await AuthStorage.existingGuestCartId() == null);
    expect(adapter.merges, [guest]);
    expect(await AuthStorage.existingGuestCartId(), isNull);
    await signIn(second.c, OAuthProvider.google);
    expect(adapter.merges, [guest]);
  });

  test('冷啟動 guest 階段由背景恢復，也合併並清除 guest cart', () async {
    final guest = await AuthStorage.guestCartId();
    final adapter = _ProfileAdapter();
    var calls = 0;
    final h = harness(
      authSession: FakeAuthSession(signedIn: true),
      adapter: adapter,
      getSession: () {
        if (++calls == 1) throw ConnectException(Code.unavailable, 'offline');
        return GetSessionResponse(account: Account(id: 'recovered'));
      },
    );
    expect(await h.c.read(sessionProvider.future), isNull);
    await _until(
      () async =>
          h.c.read(sessionProvider).value?.userId == 'recovered' &&
          await AuthStorage.existingGuestCartId() == null,
    );
    expect(h.c.read(sessionProvider).value?.userId, 'recovered');
    expect(adapter.merges, [guest]);
    expect(await AuthStorage.existingGuestCartId(), isNull);
    await signIn(h.c, OAuthProvider.google);
    expect(adapter.merges, [guest]);
  });

  test('GetSession 缺少帳號時保留 session token', () async {
    final h = harness(getSession: () => GetSessionResponse());
    await expectLater(signIn(h.c, OAuthProvider.google), throwsException);
    expect(h.auth.isSignedIn, isTrue);
    expect(h.auth.calls, ['signIn google']);
  });

  test('GetSession 明確拒絕憑證時清除 session token', () async {
    final h = harness(
      getSession: () => throw ConnectException(Code.unauthenticated, 'invalid'),
    );

    await expectLater(
      signIn(h.c, OAuthProvider.google),
      throwsA(isA<UnauthorizedException>()),
    );

    expect(h.auth.calls, ['signIn google', 'clear']);
    expect(h.c.read(sessionProvider).value, isNull);
  });
}

/// 登入後的 REST（合併購物車、`GET /app/buyers/me`）：一律回同一個 body，不連網路。
class _ProfileAdapter implements HttpClientAdapter {
  final merges = <String>[];
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.path == '/app/carts/merge') {
      merges.add((options.data as Map)['guestId'] as String);
    }
    return ResponseBody.fromString(
      '{"code":0,"data":{"nickname":"Cora"}}',
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

Future<void> _until(Future<bool> Function() condition) async {
  final deadline = DateTime.now().add(const Duration(seconds: 5));
  while (!await condition()) {
    if (DateTime.now().isAfter(deadline)) fail('condition not met within 5s');
    await Future<void>.delayed(const Duration(milliseconds: 20));
  }
}
