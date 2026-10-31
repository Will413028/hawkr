import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_auth/hawkr_auth.dart';

const _config = AuthConfig(
  googleIosClientId: 'ios-client',
  googleWebClientId: 'web-client',
  lineChannelId: 'line-channel',
);

final class _Calls {
  final events = <String>[];
}

final class _FakeGateway implements IdentityGateway {
  _FakeGateway(this.calls);

  final _Calls calls;
  int issued = 0;
  Object? exchangeError;

  @override
  Future<String> issueNonce() async {
    calls.events.add('issueNonce');
    return 'nonce-${++issued}';
  }

  @override
  Future<String> exchange({
    required OAuthProvider provider,
    required String idToken,
    required String nonce,
  }) async {
    calls.events.add('exchange:${provider.name}:$idToken:$nonce');
    final error = exchangeError;
    if (error != null) throw error;
    return 'session-token';
  }
}

final class _FakeGoogle implements GoogleSdk {
  _FakeGoogle(this.calls);

  final _Calls calls;
  String? idToken = 'google-id-token';
  Object? authenticateError;

  @override
  Future<void> initialize({
    required String clientId,
    required String serverClientId,
    required String nonce,
  }) async {
    calls.events.add('google.initialize:$clientId:$serverClientId:$nonce');
  }

  @override
  Future<String?> authenticate() async {
    calls.events.add('google.authenticate');
    final error = authenticateError;
    if (error != null) throw error;
    return idToken;
  }

  @override
  Future<void> signOut() async => calls.events.add('google.signOut');
}

final class _FakeLine implements LineSdk {
  _FakeLine(this.calls);

  final _Calls calls;
  String? idToken = 'line-id-token';

  @override
  Future<String?> login({
    required String channelId,
    required String nonce,
  }) async {
    calls.events.add('line.login:$channelId:$nonce');
    return idToken;
  }

  @override
  Future<void> logout() async => calls.events.add('line.logout');
}

final class _MemoryStore implements SessionTokenStore {
  String? value;
  Object? writeError;

  @override
  Future<String?> read() async => value;

  @override
  Future<void> write(String token) async {
    final error = writeError;
    if (error != null) throw error;
    value = token;
  }

  @override
  Future<void> delete() async => value = null;
}

void main() {
  late _Calls calls;
  late _FakeGateway gateway;
  late _FakeGoogle google;
  late _FakeLine line;
  late _MemoryStore store;
  late AuthSession session;

  setUp(() {
    calls = _Calls();
    gateway = _FakeGateway(calls);
    google = _FakeGoogle(calls);
    line = _FakeLine(calls);
    store = _MemoryStore();
    session = AuthSession(
      store: store,
      gateway: gateway,
      config: _config,
      google: google,
      line: line,
    );
  });

  group('Google', () {
    test(
      '先取 nonce，再用這個 nonce 重新 initialize，之後才 authenticate，最後換 session',
      () async {
        expect(await session.signIn(OAuthProvider.google), isTrue);
        expect(calls.events, [
          'issueNonce',
          'google.initialize:ios-client:web-client:nonce-1',
          'google.authenticate',
          'exchange:google:google-id-token:nonce-1',
        ]);
      },
    );

    test('每次登入都取新 nonce 並重新 initialize', () async {
      await session.signIn(OAuthProvider.google);
      calls.events.clear();
      await session.signIn(OAuthProvider.google);
      expect(calls.events, [
        'issueNonce',
        'google.initialize:ios-client:web-client:nonce-2',
        'google.authenticate',
        'exchange:google:google-id-token:nonce-2',
      ]);
    });

    test('使用者取消：回 false，不換 session，不保存 token', () async {
      google.idToken = null;
      expect(await session.signIn(OAuthProvider.google), isFalse);
      expect(calls.events.any((e) => e.startsWith('exchange')), isFalse);
      expect(session.isSignedIn, isFalse);
      expect(store.value, isNull);
    });

    test('SDK 失敗：拋 SignInFailed（使用者看的文字），記 log，不換 session', () async {
      final logs = <AuthLogLevel>[];
      google.authenticateError = StateError('boom');
      session = AuthSession(
        store: store,
        gateway: gateway,
        config: _config,
        google: google,
        line: line,
        log: (level, message, error, stack) => logs.add(level),
      );
      await expectLater(
        session.signIn(OAuthProvider.google),
        throwsA(
          isA<SignInFailed>().having(
            (e) => e.toString(),
            'message',
            '登入失敗，請稍後再試',
          ),
        ),
      );
      expect(logs, [AuthLogLevel.error]);
      expect(calls.events.any((e) => e.startsWith('exchange')), isFalse);
    });
  });

  group('LINE', () {
    test('nonce 交給 LINE SDK，ID token 連同同一個 nonce 換 session', () async {
      expect(await session.signIn(OAuthProvider.line), isTrue);
      expect(calls.events, [
        'issueNonce',
        'line.login:line-channel:nonce-1',
        'exchange:line:line-id-token:nonce-1',
      ]);
    });

    test('使用者取消：回 false', () async {
      line.idToken = null;
      expect(await session.signIn(OAuthProvider.line), isFalse);
      expect(session.isSignedIn, isFalse);
    });
  });

  test('設定沒填：不取 nonce，拋「登入服務尚未設定」', () async {
    session = AuthSession(
      store: store,
      gateway: gateway,
      config: const AuthConfig(),
      google: google,
      line: line,
    );
    for (final provider in OAuthProvider.values) {
      await expectLater(
        session.signIn(provider),
        throwsA(
          isA<SignInFailed>().having(
            (e) => e.toString(),
            'message',
            '登入服務尚未設定',
          ),
        ),
      );
    }
    expect(calls.events, isEmpty);
  });

  test('換 session 成功：token 存進保存處並供請求使用', () async {
    await session.signIn(OAuthProvider.google);
    expect(session.isSignedIn, isTrue);
    expect(await session.token(), 'session-token');
    expect(store.value, 'session-token');
  });

  test('換 session 失敗：例外原樣拋出，沒有登入', () async {
    final failure = StateError('exchange failed');
    gateway.exchangeError = failure;
    await expectLater(
      session.signIn(OAuthProvider.line),
      throwsA(same(failure)),
    );
    expect(session.isSignedIn, isFalse);
    expect(store.value, isNull);
  });

  test('保存 token 失敗：拋 SignInFailed，沒有登入', () async {
    store.writeError = StateError('keychain');
    await expectLater(
      session.signIn(OAuthProvider.google),
      throwsA(isA<SignInFailed>()),
    );
    expect(session.isSignedIn, isFalse);
  });

  test('restore：還原保存的 token；保存處讀不到時以登出開始', () async {
    store.value = 'saved';
    final restored = await AuthSession.restore(
      store: store,
      gateway: gateway,
      config: _config,
      google: google,
      line: line,
    );
    expect(restored.isSignedIn, isTrue);
    expect(await restored.token(), 'saved');

    final failing = await AuthSession.restore(
      store: _ThrowingStore(),
      gateway: gateway,
      config: _config,
      google: google,
      line: line,
    );
    expect(failing.isSignedIn, isFalse);
  });

  test('clear：token 立刻清掉，保存的 token 刪除，兩家 SDK 登出；其中一步失敗不影響其餘', () async {
    await session.signIn(OAuthProvider.google);
    calls.events.clear();
    await session.clear();
    expect(session.isSignedIn, isFalse);
    expect(await session.token(), isNull);
    expect(store.value, isNull);
    expect(calls.events, ['google.signOut', 'line.logout']);

    final failing = AuthSession(
      store: _ThrowingStore(),
      gateway: gateway,
      config: _config,
      restoredToken: 'x',
      google: google,
      line: line,
    );
    calls.events.clear();
    await failing.clear();
    expect(failing.isSignedIn, isFalse);
    expect(calls.events, ['google.signOut', 'line.logout']);
  });

  group('session token 只存安全儲存', () {
    setUp(() => FlutterSecureStorage.setMockInitialValues({}));

    test('預設的保存處是 Keychain（flutter_secure_storage）', () async {
      final container = ProviderContainer(
        overrides: [authConfigProvider.overrideWithValue(_config)],
      );
      addTearDown(container.dispose);
      expect(
        container.read(sessionTokenStoreProvider),
        isA<SecureSessionTokenStore>(),
      );
      final restored = await AuthSession.restore(
        store: container.read(sessionTokenStoreProvider),
        gateway: gateway,
        config: _config,
        google: google,
        line: line,
      );
      await restored.signIn(OAuthProvider.google);
      expect(
        await const FlutterSecureStorage().read(
          key: SecureSessionTokenStore.key,
        ),
        'session-token',
      );
      await restored.clear();
      expect(
        await const FlutterSecureStorage().read(
          key: SecureSessionTokenStore.key,
        ),
        isNull,
      );
    });
  });
}

final class _ThrowingStore implements SessionTokenStore {
  @override
  Future<String?> read() async => throw StateError('keychain');

  @override
  Future<void> write(String token) async => throw StateError('keychain');

  @override
  Future<void> delete() async => throw StateError('keychain');
}
