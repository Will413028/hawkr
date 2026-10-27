import 'dart:async';

import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_auth/clerk.dart';
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
import 'package:hawkr_buyer/core/session/clerk_session_provider.dart';
import 'package:hawkr_buyer/features/auth/presentation/auth_controller.dart';
import 'package:hawkr_buyer/features/notification/data/push/manager.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter/services.dart';
import 'package:hawkr_buyer/shared/utils/hk_log.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_clerk_session.dart';
import '../../support/firebase_core_mock.dart';
import '../../support/secure_storage_mock.dart';

/// 登入：以 Clerk 登入，再以 GetSession 取得帳號寫入 session。
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

  ({
    ProviderContainer c,
    FakeClerkSession clerk,
    List<String?> getSessions,
    List<String> events,
    List<LinkAppleCredentialRequest> links,
  })
  harness({
    FakeClerkSession? clerkSession,
    _FakePicker? picker,
    _ProfileAdapter? adapter,
    FutureOr<GetSessionResponse> Function()? getSession,
    FutureOr<LinkAppleCredentialResponse> Function()? link,
  }) {
    final clerk = clerkSession ?? FakeClerkSession();
    final getSessions = <String?>[];
    final events = <String>[];
    final links = <LinkAppleCredentialRequest>[];
    final container = ProviderContainer(
      overrides: [
        clerkSessionProvider.overrideWith((ref) async => clerk),
        oauthPickerProvider.overrideWithValue(picker ?? _FakePicker()),
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
        // GetSession 經過真的身分 interceptor：驗證登入之後的請求帶 Clerk 的 token。
        identityClientProvider.overrideWith(
          (ref) => IdentityAppServiceClient(
            FakeTransportBuilder()
                .unary(IdentityAppService.getSession, (req, ctx) {
                  getSessions.add(ctx.requestHeaders['authorization']);
                  events.add('getSession');
                  return getSession?.call() ??
                      GetSessionResponse(account: Account(id: 'acc_01J8Z'));
                })
                .unary(IdentityAppService.linkAppleCredential, (req, _) {
                  events.add('link');
                  links.add(req);
                  return link?.call() ?? LinkAppleCredentialResponse();
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
    return (
      c: container,
      clerk: clerk,
      getSessions: getSessions,
      events: events,
      links: links,
    );
  }

  Future<bool> signIn(ProviderContainer c, OAuthProvider provider) =>
      c.read(authControllerProvider.notifier).signInWith(provider);

  test('Google：在系統瀏覽器登入 Clerk，GetSession 帶 Clerk 的 token，寫入帳號與暱稱', () async {
    final picker = _FakePicker();
    final h = harness(picker: picker);

    expect(await signIn(h.c, OAuthProvider.google), isTrue);

    expect(h.clerk.calls, ['browser google']);
    expect(picker.browserSchemes, ['hawkrbuyer']);
    expect(h.getSessions, ['Bearer live']);
    final session = h.c.read(sessionProvider).value;
    expect(session?.userId, 'acc_01J8Z');
    expect(session?.nickname, 'Cora');
  });

  test('LINE：同樣經系統瀏覽器', () async {
    final h = harness();

    expect(await signIn(h.c, OAuthProvider.line), isTrue);

    expect(h.clerk.calls, ['browser line']);
    expect(h.c.read(sessionProvider).value?.userId, 'acc_01J8Z');
  });

  test('Apple：以 Sign in with Apple 的 identity token 登入 Clerk', () async {
    final h = harness();

    expect(await signIn(h.c, OAuthProvider.apple), isTrue);

    expect(h.clerk.calls, ['apple apple-id-token']);
    expect(h.getSessions, ['Bearer live']);
  });

  test(
    'Apple：GetSession 之後把 identity token 與 authorization code 交給伺服器，各一次',
    () async {
      final h = harness();

      expect(await signIn(h.c, OAuthProvider.apple), isTrue);
      await pumpEventQueue();

      expect(h.events, ['getSession', 'link']);
      expect(h.links.single.idToken, 'apple-id-token');
      expect(h.links.single.authorizationCode, 'apple-code');
    },
  );

  test('Apple：交付失敗只記錄，登入照常完成', () async {
    final h = harness(
      link: () => throw ConnectException(Code.unavailable, 'down'),
    );

    expect(await signIn(h.c, OAuthProvider.apple), isTrue);
    await pumpEventQueue();

    expect(h.links, hasLength(1));
    expect(h.c.read(sessionProvider).value?.userId, 'acc_01J8Z');
  });

  test('Apple：伺服器未設定 Apple（unavailable）：不當成錯誤，登入照常完成', () async {
    final h = harness(
      link: () =>
          throw ConnectException(Code.unavailable, 'apple not configured'),
    );

    expect(await signIn(h.c, OAuthProvider.apple), isTrue);
    await pumpEventQueue();

    expect(h.links, hasLength(1));
    expect(h.c.read(sessionProvider).value?.userId, 'acc_01J8Z');
  });

  test('Apple：交付一直沒有回應也不擋登入', () async {
    final never = Completer<LinkAppleCredentialResponse>();
    final h = harness(link: () => never.future);

    expect(await signIn(h.c, OAuthProvider.apple), isTrue);

    expect(h.c.read(sessionProvider).value?.userId, 'acc_01J8Z');
  });

  test('Google 與 LINE 不交付 Apple 憑證', () async {
    final google = harness();
    await signIn(google.c, OAuthProvider.google);
    final line = harness();
    await signIn(line.c, OAuthProvider.line);
    await pumpEventQueue();

    expect(google.links, isEmpty);
    expect(line.links, isEmpty);
  });

  test('還原既有的 Clerk session 不交付 Apple 憑證', () async {
    final h = harness(clerkSession: FakeClerkSession(signedIn: true));

    expect(await signIn(h.c, OAuthProvider.apple), isTrue);
    await pumpEventQueue();

    expect(h.links, isEmpty);
  });

  test('使用者取消：回 false，不呼叫 GetSession，沒有 session', () async {
    final h = harness(picker: _FakePicker(cancel: true));

    expect(await signIn(h.c, OAuthProvider.google), isFalse);
    expect(await signIn(h.c, OAuthProvider.apple), isFalse);

    expect(h.getSessions, isEmpty);
    expect(h.c.read(sessionProvider).value, isNull);
  });

  test('GetSession 停權錯誤：結束 Clerk session、往外丟，不寫入帳號', () async {
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

    expect(h.clerk.calls, ['browser google', 'signOut']);
    expect(h.clerk.isSignedIn, isFalse);
    expect(h.c.read(sessionProvider).value, isNull);
  });

  test('暫時失敗保留 Clerk，再次登入直接重試 GetSession', () async {
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
    expect(h.clerk.isSignedIn, isTrue);
    fail = false;
    expect(await signIn(h.c, OAuthProvider.google), isTrue);
    expect(h.clerk.calls, ['browser google']);
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

  test('冷啟動 token 換發遭拒時登出 Clerk，不送 GetSession', () async {
    final clerk = FakeClerkSession(signedIn: true)
      ..tokenError = const SessionTokenRefused('revoked');
    final h = harness(clerkSession: clerk);
    expect(await h.c.read(sessionProvider.future), isNull);
    expect(clerk.isSignedIn, isFalse);
    expect(clerk.calls, ['signOut']);
    expect(h.getSessions, isEmpty);
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
          final h = harness(clerkSession: FakeClerkSession(signedIn: restore));
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
      clerkSession: FakeClerkSession(signedIn: true),
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
    final second = harness(clerkSession: first.clerk, adapter: adapter);
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
      clerkSession: FakeClerkSession(signedIn: true),
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

  test('GetSession 缺少帳號時保留 Clerk', () async {
    final h = harness(getSession: () => GetSessionResponse());
    await expectLater(signIn(h.c, OAuthProvider.google), throwsException);
    expect(h.clerk.isSignedIn, isTrue);
    expect(h.clerk.calls, ['browser google']);
  });

  test('GetSession 明確拒絕憑證時結束 Clerk session', () async {
    final h = harness(
      getSession: () => throw ConnectException(Code.unauthenticated, 'invalid'),
    );

    await expectLater(
      signIn(h.c, OAuthProvider.google),
      throwsA(isA<UnauthorizedException>()),
    );

    expect(h.clerk.calls, ['browser google', 'signOut']);
    expect(h.c.read(sessionProvider).value, isNull);
  });
}

class _FakePicker implements OAuthPicker {
  _FakePicker({this.cancel = false});

  final bool cancel;
  final browserSchemes = <String>[];

  @override
  Future<Uri?> authenticateInBrowser(Uri url, String callbackScheme) async {
    browserSchemes.add(callbackScheme);
    return cancel ? null : Uri.parse('$callbackScheme://oauth-callback');
  }

  @override
  Future<AppleCredentials?> pickApple() async => cancel
      ? null
      : const AppleCredentials(
          identityToken: 'apple-id-token',
          authorizationCode: 'apple-code',
        );
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
