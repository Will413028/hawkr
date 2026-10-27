import 'dart:async';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'dart:io';

import 'package:clerk_auth/clerk_auth.dart' as clerk;
import 'package:hawkr_auth/clerk.dart';
import 'package:test/fake.dart';
import 'package:test/test.dart';

final _redirect = Uri.parse('hawkrbuyer://oauth-callback');

clerk.Session _session({List<String> providers = const [], String? email}) =>
    clerk.Session.fromJson({
      'id': 'sess_1',
      'status': 'active',
      'public_user_data': <String, dynamic>{},
      'user': {
        'id': 'user_1',
        'create_organization_enabled': false,
        if (email != null) ...{
          'primary_email_address_id': 'idn_1',
          'email_addresses': [
            {
              'id': 'idn_1',
              'email_address': email,
              'reserved': false,
              'updated_at': 0,
              'created_at': 0,
              'verification': {
                'status': 'verified',
                'strategy': 'from_oauth_google',
              },
            },
          ],
        },
        'external_accounts': [
          for (final (i, provider) in providers.indexed)
            {
              'id': 'eac_$i',
              'provider': provider,
              'provider_user_id': 'p$i',
              'approved_scopes': '',
              'email_address': 'u@example.com',
              'verification': {'status': 'verified', 'strategy': provider},
            },
        ],
      },
    });

clerk.SignIn _signIn({
  clerk.Status status = clerk.Status.unverified,
  String? providerUrl = 'https://accounts.google.com/o/oauth2/auth',
}) => clerk.SignIn(
  id: 'sia_1',
  status: clerk.Status.needsFirstFactor,
  supportedIdentifiers: const [],
  supportedFirstFactors: const [],
  supportedSecondFactors: const [],
  firstFactorVerification: clerk.Verification(
    status: status,
    strategy: clerk.Strategy.oauthGoogle,
    attempts: null,
    expireAt: DateTime.utc(2030),
    externalVerificationRedirectUrl: providerUrl,
  ),
);

/// Clerk SDK 的 [clerk.Auth]：記錄呼叫的順序，登入的結果由測試決定。
class _FakeAuth extends Fake implements clerk.Auth {
  final calls = <String>[];

  bool signedIn = false;

  @override
  clerk.Session? session;

  @override
  clerk.SignIn? signIn;

  @override
  clerk.SignUp? get signUp => null;

  Future<clerk.SessionToken> Function() onSessionToken = () async =>
      clerk.SessionToken(jwt: 'jwt-1');

  /// 完成 OAuth 或以 ID token 登入之後：true 是登入，false 停在 transferable（新使用者）。
  bool completesSignIn = true;

  /// transferable 轉成 sign-up 之後是否登入（false：Clerk 還要更多資料）。
  bool transferSignsIn = true;

  Object? oauthError;
  Object? signOutOfError;

  @override
  bool get isSignedIn => signedIn;

  @override
  clerk.User? get user => session?.user;

  @override
  Future<clerk.SessionToken> sessionToken({
    clerk.Organization? organization,
    String? templateName,
  }) {
    calls.add('sessionToken');
    return onSessionToken();
  }

  @override
  Future<void> oauthSignIn({
    required clerk.Strategy strategy,
    required Uri? redirect,
  }) async {
    calls.add('oauthSignIn $strategy $redirect');
    if (oauthError case final error?) throw error;
    signIn = _signIn();
  }

  @override
  Future<void> completeOAuthSignIn({required String token}) async {
    calls.add('completeOAuthSignIn $token');
    _complete();
  }

  @override
  Future<void> refreshClient() async {
    calls.add('refreshClient');
    _complete();
  }

  @override
  Future<void> idTokenSignIn({
    required clerk.IdTokenProvider provider,
    required String? token,
  }) async {
    calls.add('idTokenSignIn ${provider.name} $token');
    _complete();
  }

  @override
  Future<void> transfer() async {
    calls.add('transfer');
    signedIn = transferSignsIn;
  }

  void _complete() {
    if (completesSignIn) {
      signedIn = true;
    } else {
      signIn = _signIn(status: clerk.Status.transferable);
    }
  }

  @override
  Future<void> signOutOf(clerk.Session session) async {
    calls.add('signOutOf ${session.id}');
    if (signOutOfError case final error?) throw error;
  }

  @override
  Future<void> signOut() async {
    calls.add('signOut');
    signedIn = false;
    session = null;
  }
}

void main() {
  group('token', () {
    test('沒有登入時回 null，不向 Clerk 換發', () async {
      final auth = _FakeAuth();

      expect(await ClerkSession.withAuth(auth).token(), isNull);
      expect(auth.calls, isEmpty);
    });

    test('登入中回 SDK 的 token；同時的呼叫共用一次換發', () async {
      final auth = _FakeAuth()..signedIn = true;
      final pending = Completer<clerk.SessionToken>();
      auth.onSessionToken = () => pending.future;
      final session = ClerkSession.withAuth(auth);

      final first = session.token();
      final second = session.token();
      pending.complete(clerk.SessionToken(jwt: 'jwt-1'));

      expect(await first, 'jwt-1');
      expect(await second, 'jwt-1');
      expect(auth.calls, ['sessionToken']);
    });

    test('Clerk 拒絕換發（session 已結束）時拋 refused', () async {
      final auth = _FakeAuth()..signedIn = true;
      auth.onSessionToken = () async => throw const clerk.ClerkError(
        code: clerk.ClerkErrorCode.serverErrorResponse,
        message: 'session ended',
      );

      await expectLater(
        ClerkSession.withAuth(auth).token(),
        throwsA(isA<SessionTokenRefused>()),
      );
    });

    for (final status in [500, 502, 503]) {
      test('Clerk FAPI $status 保留 unreachable', () async {
        final auth = _FakeAuth()..signedIn = true;
        auth.onSessionToken = () async {
          final service = ClerkHttpService();
          try {
            await http.runWithClient(
              () => service.send(
                clerk.HttpMethod.post,
                Uri.parse('https://clerk.example/token'),
              ),
              () => MockClient(
                (_) async =>
                    http.Response('{"errors":[{"message":"down"}]}', status),
              ),
            );
          } finally {
            service.terminate();
          }
          return clerk.SessionToken(jwt: 'unused');
        };
        await expectLater(
          ClerkSession.withAuth(auth).token(),
          throwsA(isA<ClerkServiceUnavailable>()),
        );
      });
    }

    test('Clerk 429 重試耗盡與 timeout 不算 refused', () async {
      for (final error in [
        const clerk.ClerkError(
          code: clerk.ClerkErrorCode.tooManyRetries,
          message: 'rate limited',
        ),
        TimeoutException('timeout'),
      ]) {
        final auth = _FakeAuth()..signedIn = true;
        auth.onSessionToken = () async => throw error;
        await expectLater(
          ClerkSession.withAuth(auth).token(),
          throwsA(same(error)),
        );
      }
    });

    test('連不上 Clerk 時拋出原本的例外', () async {
      final auth = _FakeAuth()..signedIn = true;
      auth.onSessionToken = () async => throw const SocketException('offline');

      await expectLater(
        ClerkSession.withAuth(auth).token(),
        throwsA(isA<SocketException>()),
      );
    });

    test('SDK 在背景換 token 的計時器失敗時交給 onBackgroundError，不變成沒有處理的錯誤', () async {
      final auth = _FakeAuth()..signedIn = true;
      auth.onSessionToken = () async {
        Timer(Duration.zero, () => throw const SocketException('offline'));
        return clerk.SessionToken(jwt: 'jwt-1');
      };
      final background = <Object>[];
      final session = ClerkSession.withAuth(
        auth,
        onBackgroundError: (error, _) => background.add(error),
      );

      expect(await session.token(), 'jwt-1');
      await Future<void>.delayed(const Duration(milliseconds: 10));

      expect(background, [isA<SocketException>()]);
    });
  });

  group('signInWithBrowser', () {
    test('Clerk 給的網址交給瀏覽器，以導回網址的 rotating token 完成登入', () async {
      final auth = _FakeAuth();
      Uri? opened;
      String? scheme;

      final done = await ClerkSession.withAuth(auth).signInWithBrowser(
        BrowserSignIn.line,
        redirect: _redirect,
        authenticate: (url, callbackScheme) async {
          opened = url;
          scheme = callbackScheme;
          return _redirect.replace(
            queryParameters: {'rotating_token_nonce': 'nonce-1'},
          );
        },
      );

      expect(done, isTrue);
      expect(opened.toString(), 'https://accounts.google.com/o/oauth2/auth');
      expect(scheme, 'hawkrbuyer');
      expect(auth.calls, [
        'oauthSignIn oauth_line $_redirect',
        'completeOAuthSignIn nonce-1',
      ]);
    });

    test('使用者在瀏覽器取消：回 false，不完成登入', () async {
      final auth = _FakeAuth();

      final done = await ClerkSession.withAuth(auth).signInWithBrowser(
        BrowserSignIn.google,
        redirect: _redirect,
        authenticate: (_, _) async => null,
      );

      expect(done, isFalse);
      expect(auth.calls, ['oauthSignIn oauth_google $_redirect']);
    });

    test('新使用者：導回沒有 rotating token 時重新讀取 client，再轉成 sign-up', () async {
      final auth = _FakeAuth()..completesSignIn = false;

      final done = await ClerkSession.withAuth(auth).signInWithBrowser(
        BrowserSignIn.google,
        redirect: _redirect,
        authenticate: (_, _) async => _redirect,
      );

      expect(done, isTrue);
      expect(auth.calls.skip(1), ['refreshClient', 'transfer']);
    });

    test('流程結束仍沒有登入時拋 ClerkSignInException', () async {
      final auth = _FakeAuth()
        ..completesSignIn = false
        ..transferSignsIn = false;

      await expectLater(
        ClerkSession.withAuth(auth).signInWithBrowser(
          BrowserSignIn.google,
          redirect: _redirect,
          authenticate: (_, _) async => _redirect,
        ),
        throwsA(isA<ClerkSignInException>()),
      );
    });

    test('Clerk 的錯誤與網路錯誤轉成給使用者看的文字', () async {
      final auth = _FakeAuth()
        ..oauthError = const clerk.ClerkError(
          code: clerk.ClerkErrorCode.serverErrorResponse,
          message: 'strategy not enabled',
        );
      Future<bool> run() => ClerkSession.withAuth(auth).signInWithBrowser(
        BrowserSignIn.google,
        redirect: _redirect,
        authenticate: (_, _) async => _redirect,
      );

      await expectLater(
        run(),
        throwsA(
          isA<ClerkSignInException>()
              .having((e) => e.noResponse, 'noResponse', isFalse)
              .having((e) => e.toString(), 'message', '登入失敗，請稍後再試'),
        ),
      );

      auth.oauthError = const SocketException('offline');
      await expectLater(
        run(),
        throwsA(
          isA<ClerkSignInException>().having(
            (e) => e.noResponse,
            'noResponse',
            isTrue,
          ),
        ),
      );
    });
  });

  test('signInWithApple：以 identity token 登入，新使用者轉成 sign-up', () async {
    final auth = _FakeAuth()..completesSignIn = false;
    auth.signIn = _signIn(status: clerk.Status.transferable);

    await ClerkSession.withAuth(auth).signInWithApple('apple-id-token');

    expect(auth.calls, ['idTokenSignIn apple apple-id-token', 'transfer']);
    expect(auth.isSignedIn, isTrue);
  });

  group('signOut', () {
    test('先在 Clerk 撤銷 session，再清除本機的狀態', () async {
      final auth = _FakeAuth()
        ..signedIn = true
        ..session = _session();

      await ClerkSession.withAuth(auth).signOut();

      expect(auth.calls, ['signOutOf sess_1', 'signOut']);
    });

    test('撤銷失敗時本機照樣登出', () async {
      final auth = _FakeAuth()
        ..signedIn = true
        ..session = _session()
        ..signOutOfError = const SocketException('offline');

      await ClerkSession.withAuth(auth).signOut();

      expect(auth.calls, ['signOutOf sess_1', 'signOut']);
      expect(auth.isSignedIn, isFalse);
    });
  });

  test('email：登入的使用者在 Clerk 的主要 email，沒有時是 null', () {
    final auth = _FakeAuth()
      ..signedIn = true
      ..session = _session(email: 'buyer@example.com');
    final session = ClerkSession.withAuth(auth);

    expect(session.email, 'buyer@example.com');
    auth.session = _session();
    expect(session.email, isNull);
    auth.session = null;
    expect(session.email, isNull);
  });

  test('disabled：永遠是登出，登入時說明尚未設定', () async {
    const session = ClerkSession.disabled();

    expect(session.isSignedIn, isFalse);
    expect(session.email, isNull);
    expect(await session.token(), isNull);
    await expectLater(
      session.signInWithApple('token'),
      throwsA(isA<ClerkSignInException>()),
    );
  });

  group('KeyValuePersistor', () {
    test('值以 JSON 存在加了前綴的 key，讀回原本的型別', () async {
      final store = <String, String>{};
      final persistor = KeyValuePersistor(
        read: (key) async => store[key],
        write: (key, value) async => store[key] = value,
        delete: (key) async => store.remove(key),
      );

      await persistor.write(r'$client', {'id': 'client_1'});
      await persistor.write('_clerkClient_Token_1', 'token-1');

      expect(store.keys, [r'clerk.$client', 'clerk._clerkClient_Token_1']);
      expect(await persistor.read<Map<String, dynamic>>(r'$client'), {
        'id': 'client_1',
      });
      expect(await persistor.read<String>('_clerkClient_Token_1'), 'token-1');
      await persistor.delete('_clerkClient_Token_1');
      expect(await persistor.read<String>('_clerkClient_Token_1'), isNull);
    });

    test('讀寫失敗交給 onError，讀不出來的值當成沒有', () async {
      final errors = <Object>[];
      final persistor = KeyValuePersistor(
        read: (_) async => '{not json',
        write: (_, _) async => throw StateError('keychain'),
        delete: (_) async {},
        onError: (error, _) => errors.add(error),
      );

      expect(await persistor.read<String>('k'), isNull);
      await persistor.write('k', 'v');

      expect(errors, [isA<FormatException>(), isA<StateError>()]);
    });
  });
}
