/// App 的登入：Clerk（ADR `2026-09-27-identity-clerk.md`）。[ClerkSession] 以 Clerk 的 Frontend API 登入、登出與
/// 取得 session token；畫面、系統瀏覽器與 Sign in with Apple 由 App 提供。對外 API 的請求以 [ClerkSession.token]
/// 帶 `Authorization: Bearer`（transport.dart 的 `authInterceptor`）。
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:clerk_auth/clerk_auth.dart' as clerk;
import 'package:http/http.dart' as http;

/// 以系統瀏覽器登入的 provider（OAuth）。Apple 用原生的 Sign in with Apple（[ClerkSession.signInWithApple]）；
/// Google 不能在 App 內嵌的 WebView 登入，所以不用 Clerk SDK 預設的 WebView。
enum BrowserSignIn {
  google(clerk.Strategy.oauthGoogle),
  line(clerk.Strategy(name: 'oauth', provider: 'line'));

  const BrowserSignIn(this.strategy);

  final clerk.Strategy strategy;
}

/// 在系統瀏覽器（iOS 的 ASWebAuthenticationSession）開啟 [url]，等它導向 [callbackScheme] 的網址後回傳那個網址；
/// 使用者取消時回 null。
typedef BrowserAuthenticator =
    Future<Uri?> Function(Uri url, String callbackScheme);

/// 登入沒有完成。[message] 是給使用者看的文字，`toString()` 就是它。
final class ClerkSignInException implements Exception {
  const ClerkSignInException(
    this.message, {
    this.noResponse = false,
    this.cause,
  });

  final String message;

  /// 沒有收到 Clerk 的回應（連線失敗、逾時）。
  final bool noResponse;

  final Object? cause;

  @override
  String toString() => message;
}

final class SessionTokenRefused implements Exception {
  const SessionTokenRefused(this.cause);
  final Object cause;
}

bool isSessionTokenRefused(Object error) => error is SessionTokenRefused;

final class ClerkServiceUnavailable implements Exception {
  const ClerkServiceUnavailable(this.statusCode);
  final int statusCode;
}

class ClerkHttpService extends clerk.DefaultHttpService {
  const ClerkHttpService();

  @override
  Future<http.Response> send(
    clerk.HttpMethod method,
    Uri uri, {
    Map<String, String>? headers,
    Map<String, dynamic>? params,
    String? body,
  }) async {
    final response = await super.send(
      method,
      uri,
      headers: headers,
      params: params,
      body: body,
    );
    return _checkResponse(response);
  }

  static http.Response _checkResponse(http.Response response) {
    if (response.statusCode >= 500) {
      throw ClerkServiceUnavailable(response.statusCode);
    }
    return response;
  }
}

/// 登入的狀態與 session token 的來源。
///
/// Clerk 的 session token 效期約 60 秒，SDK 在到期前換新；登入狀態（Clerk 的 client）以 [clerk.Persistor]
/// 保存，App 重開時還原。帳號由 identity 在第一個認證過的請求時建立，登入之後 App 呼叫 `GetSession`。
abstract interface class ClerkSession {
  /// 以 [publishableKey] 連上 Clerk，並還原 [persistor] 保存的狀態。連不上 Clerk 時以保存的狀態啟動：登入中的
  /// 使用者仍是登入中，token 在連上之後才換得到。
  ///
  /// SDK 在背景換新 token 失敗時（例如離線）交給 [onBackgroundError]；下次需要 token 時再換。
  static Future<ClerkSession> start({
    required String publishableKey,
    required clerk.Persistor persistor,
    void Function(Object error, StackTrace stack)? onBackgroundError,
  }) async {
    final auth = clerk.Auth(
      config: clerk.AuthConfig(
        publishableKey: publishableKey,
        persistor: persistor,
        httpService: const ClerkHttpService(),
        // 啟動時不先換 token：SDK 換 token 失敗時會拋錯，離線啟動就會失敗。第一次需要 token 時才換，之後 SDK
        // 在到期前自己換新。
        sessionTokenPolling: false,
        // 不每 10 秒重新讀取 client：session 失效由換 token 失敗與伺服器的 unauthenticated 發現。
        clientRefreshPeriod: Duration.zero,
        telemetryPeriod: Duration.zero,
        // 預設重試 8 次、最長退避 30 秒；請求在等 token，失敗要早點讓使用者知道。
        retryOptions: const clerk.RetryOptions(maxAttempts: 3),
      ),
    );
    await auth.initialize();
    return ClerkSession.withAuth(auth, onBackgroundError: onBackgroundError);
  }

  /// 以已經初始化的 [auth] 建立；測試以 fake 的 [clerk.Auth] 建立。
  factory ClerkSession.withAuth(
    clerk.Auth auth, {
    void Function(Object error, StackTrace stack)? onBackgroundError,
  }) = _AuthSession;

  /// 沒有設定 publishable key 時（開發與測試）：永遠是登出，登入時拋 [ClerkSignInException]。
  const factory ClerkSession.disabled() = _DisabledSession;

  /// Clerk 是否有登入中的 session。
  bool get isSignedIn;

  /// 登入的使用者在 Clerk 的主要 email；登出或沒有 email 時是 null。讀本機保存的使用者資料，不連線。
  String? get email;

  /// 沒有 session 時回 null；換發遭拒拋 [SessionTokenRefused]；連線、逾時與服務失敗保留原始例外。
  /// 同時的呼叫共用一次換發。
  Future<String?> token();

  /// 以系統瀏覽器登入 [provider]：Clerk 給出 provider 的網址，[authenticate] 開瀏覽器，provider 導回 [redirect]
  /// 之後完成登入（新使用者由 Clerk 建立）。使用者取消時回 false。失敗時拋 [ClerkSignInException]。
  Future<bool> signInWithBrowser(
    BrowserSignIn provider, {
    required Uri redirect,
    required BrowserAuthenticator authenticate,
  });

  /// 以 Sign in with Apple 給的 [identityToken] 登入（新使用者由 Clerk 建立）。失敗時拋 [ClerkSignInException]。
  Future<void> signInWithApple(String identityToken);

  /// 登出：先在 Clerk 撤銷這個 session，再清除本機保存的狀態。撤銷失敗時本機照樣清除。
  Future<void> signOut();

  void dispose();
}

/// Clerk 的 rotating token：OAuth 完成後 provider 導回 App 的網址帶著它，用來完成這次的登入。
const _kRotatingTokenNonce = 'rotating_token_nonce';

/// 換發 token 最多等多久：SDK 的 HTTP 請求沒有逾時，網路卡住時請求不會一直等下去。
const _kTokenTimeout = Duration(seconds: 10);

/// 登出時每個對 Clerk 的請求最多等多久：SDK 的 HTTP 請求沒有逾時，伺服器沒有回應時登出不卡住。
const _kSignOutTimeout = Duration(seconds: 5);

final class _AuthSession implements ClerkSession {
  _AuthSession(
    this._auth, {
    void Function(Object error, StackTrace stack)? onBackgroundError,
  }) : _onBackgroundError = onBackgroundError;

  final clerk.Auth _auth;
  final void Function(Object error, StackTrace stack)? _onBackgroundError;
  Future<String?>? _pendingToken;

  @override
  bool get isSignedIn => _auth.isSignedIn;

  @override
  String? get email => switch (_auth.user?.email) {
    final email? when email.isNotEmpty => email,
    _ => null,
  };

  @override
  Future<String?> token() {
    if (!_auth.isSignedIn) return Future.value();
    return _pendingToken ??= _fetchToken().whenComplete(
      () => _pendingToken = null,
    );
  }

  Future<String?> _fetchToken() async {
    try {
      final token = await _inBackgroundZone(
        _auth.sessionToken,
      ).timeout(_kTokenTimeout);
      return token.jwt;
    } on clerk.ClerkError catch (error) {
      if (error.code == clerk.ClerkErrorCode.serverErrorResponse ||
          error.code == clerk.ClerkErrorCode.noSessionTokenRetrieved ||
          error.code == clerk.ClerkErrorCode.noSessionFoundForUser) {
        throw SessionTokenRefused(error);
      }
      rethrow;
    }
  }

  /// SDK 換 token 之後會排一個計時器在到期前再換，那個計時器失敗時直接拋錯（沒有人接），離線時每分鐘一個沒有
  /// 處理的錯誤。在自己的 error zone 呼叫：計時器建在這個 zone，它的錯誤交給 [_onBackgroundError]；這次呼叫
  /// 的結果經 zone 外建立的 [Completer] 交回呼叫端，錯誤照常拋給呼叫端。
  Future<T> _inBackgroundZone<T>(Future<T> Function() call) {
    final result = Completer<T>();
    runZonedGuarded(
      () => Future.sync(
        call,
      ).then(result.complete, onError: result.completeError),
      (error, stack) => _onBackgroundError?.call(error, stack),
    );
    return result.future;
  }

  @override
  Future<bool> signInWithBrowser(
    BrowserSignIn provider, {
    required Uri redirect,
    required BrowserAuthenticator authenticate,
  }) {
    return _guard(() async {
      await _auth.oauthSignIn(strategy: provider.strategy, redirect: redirect);
      final url = _auth
          .signIn
          ?.firstFactorVerification
          ?.externalVerificationRedirectUrl;
      if (url == null) {
        throw const ClerkSignInException('登入失敗，請稍後再試');
      }
      final callback = await authenticate(Uri.parse(url), redirect.scheme);
      if (callback == null) return false;
      // 新使用者的 sign-in 會停在 transferable，導回的網址不一定帶 rotating token；沒有時重新讀取 client，
      // 由 [_finish] 轉成 sign-up。
      final nonce = callback.queryParameters[_kRotatingTokenNonce];
      if (nonce != null && nonce.isNotEmpty) {
        await _auth.completeOAuthSignIn(token: nonce);
      } else {
        await _auth.refreshClient();
      }
      await _finish();
      return true;
    });
  }

  @override
  Future<void> signInWithApple(String identityToken) {
    return _guard(() async {
      await _auth.idTokenSignIn(
        provider: clerk.IdTokenProvider.apple,
        token: identityToken,
      );
      await _finish();
    });
  }

  /// 第一次以這個 provider 登入的使用者，sign-in 停在 transferable：轉成 sign-up 建立使用者並登入。
  Future<void> _finish() async {
    if (!_auth.isSignedIn &&
        (_auth.signIn?.isTransferable == true ||
            _auth.signUp?.isTransferable == true)) {
      await _auth.transfer();
    }
    if (!_auth.isSignedIn) {
      throw const ClerkSignInException('登入沒有完成，請再試一次');
    }
  }

  Future<T> _guard<T>(Future<T> Function() body) async {
    try {
      return await body();
    } on ClerkSignInException {
      rethrow;
    } on clerk.ClerkError catch (e) {
      throw ClerkSignInException('登入失敗，請稍後再試', cause: e);
    } on SocketException catch (e) {
      throw ClerkSignInException(_kNetworkMessage, noResponse: true, cause: e);
    } on TimeoutException catch (e) {
      throw ClerkSignInException(_kNetworkMessage, noResponse: true, cause: e);
    }
  }

  @override
  Future<void> signOut() async {
    _pendingToken = null;
    // clerk_auth 0.0.18-beta 的 signOut 在送出 `DELETE /client` 之前就清掉 client token，Clerk 認不出是哪個
    // client，伺服器端的 session 不會結束；所以先以 session 的 remove 撤銷。
    if (_auth.session case final session?) {
      try {
        await _auth.signOutOf(session).timeout(_kSignOutTimeout);
      } catch (_) {
        // 沒撤銷到的 session 最晚在它的期限結束；本機照樣登出。
      }
    }
    // SDK 在送出之前就清掉保存的 client token；等不到回應時不再等，下次啟動 Clerk 會給一個新的 client。
    try {
      await _auth.signOut().timeout(_kSignOutTimeout);
    } on TimeoutException {
      // 見上。
    }
  }

  @override
  void dispose() => _auth.terminate();
}

const _kNetworkMessage = '網路連線失敗，請檢查網路設定';

final class _DisabledSession implements ClerkSession {
  const _DisabledSession();

  static const _notConfigured = ClerkSignInException('登入服務尚未設定');

  @override
  bool get isSignedIn => false;

  @override
  String? get email => null;

  @override
  Future<String?> token() => Future.value();

  @override
  Future<bool> signInWithBrowser(
    BrowserSignIn provider, {
    required Uri redirect,
    required BrowserAuthenticator authenticate,
  }) => Future.error(_notConfigured);

  @override
  Future<void> signInWithApple(String identityToken) =>
      Future.error(_notConfigured);

  @override
  Future<void> signOut() async {}

  @override
  void dispose() {}
}

/// 把 Clerk SDK 要保存的狀態（client token、session 與 client、environment 的快取）存進 App 提供的 key-value
/// 儲存（兩支 App 是 Keychain）。值存成 JSON，key 加上 [prefix]。
///
/// SDK 寫入時不等待結果：寫入與讀取的失敗交給 [onError]，不變成沒有處理的非同步錯誤；讀不出來的值當成沒有。
final class KeyValuePersistor implements clerk.Persistor {
  KeyValuePersistor({
    required Future<String?> Function(String key) read,
    required Future<void> Function(String key, String value) write,
    required Future<void> Function(String key) delete,
    void Function(Object error, StackTrace stack)? onError,
    this.prefix = 'clerk.',
  }) : _read = read,
       _write = write,
       _delete = delete,
       _onError = onError;

  final Future<String?> Function(String key) _read;
  final Future<void> Function(String key, String value) _write;
  final Future<void> Function(String key) _delete;
  final void Function(Object error, StackTrace stack)? _onError;
  final String prefix;

  @override
  Future<void> initialize() async {}

  @override
  void terminate() {}

  @override
  Future<T?> read<T>(String key) async {
    try {
      final raw = await _read('$prefix$key');
      return raw == null ? null : jsonDecode(raw) as T?;
    } catch (e, st) {
      _onError?.call(e, st);
      return null;
    }
  }

  @override
  Future<void> write<T>(String key, T value) async {
    try {
      await _write('$prefix$key', jsonEncode(value));
    } catch (e, st) {
      _onError?.call(e, st);
    }
  }

  @override
  Future<void> delete(String key) async {
    try {
      await _delete('$prefix$key');
    } catch (e, st) {
      _onError?.call(e, st);
    }
  }
}
