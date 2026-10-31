/// App 的登入：Google、LINE 的原生 SDK 取得 ID token，換成 identity 的 opaque session token
/// （ADR `2026-10-30-identity-self-hosted-federated-login.md` D2），token 存在 [SessionTokenStore]。對外 API 的請求以
/// [AuthSession.token] 帶 `Authorization: Bearer`。換發與登出的 RPC 由 App 經 [IdentityGateway] 提供（此套件不依賴
/// `hawkr_api`）。
library;

import 'logging.dart';
import 'oauth.dart';
import 'sdk.dart';
import 'session_token_store.dart';

/// 編譯期設定（`--dart-define-from-file=config/dart-defines.json`）。
final class AuthConfig {
  const AuthConfig({
    this.googleIosClientId = '',
    this.googleWebClientId = '',
    this.lineChannelId = '',
  });

  const AuthConfig.fromEnvironment()
    : googleIosClientId = const String.fromEnvironment('GOOGLE_IOS_CLIENT_ID'),
      googleWebClientId = const String.fromEnvironment('GOOGLE_WEB_CLIENT_ID'),
      lineChannelId = const String.fromEnvironment('LINE_CHANNEL_ID');

  /// iOS 的 OAuth client ID：ID token 的 `azp`（伺服器的 `GOOGLE_APP_CLIENT_IDS`）。
  final String googleIosClientId;

  /// web 的 OAuth client ID：ID token 的 `aud`（伺服器的 `GOOGLE_WEB_CLIENT_ID`），傳給 SDK 的 `serverClientId`。
  final String googleWebClientId;

  /// LINE Login channel ID：ID token 的 `aud`（與 web 同一個 channel）。
  final String lineChannelId;

  bool isConfigured(OAuthProvider provider) => switch (provider) {
    OAuthProvider.google =>
      googleIosClientId.isNotEmpty && googleWebClientId.isNotEmpty,
    OAuthProvider.line => lineChannelId.isNotEmpty,
  };
}

/// App 對 identity 的 `IdentityPublicService` 的呼叫。錯誤原樣拋出，由 App 轉成自己的例外。
abstract interface class IdentityGateway {
  /// `IssueAppLoginNonce`：單次使用的 nonce。
  Future<String> issueNonce();

  /// `ExchangeIdToken`：回新 session 的 token。
  Future<String> exchange({
    required OAuthProvider provider,
    required String idToken,
    required String nonce,
  });
}

/// 登入沒有完成。[message] 是給使用者看的文字，`toString()` 就是它。
final class SignInFailed implements Exception {
  const SignInFailed(this.message, {this.cause});

  static const notConfigured = SignInFailed('登入服務尚未設定');

  final String message;
  final Object? cause;

  @override
  String toString() => message;
}

const _kSignInFailedMessage = '登入失敗，請稍後再試';

class AuthSession {
  AuthSession({
    required SessionTokenStore store,
    required IdentityGateway gateway,
    required this.config,
    String? restoredToken,
    GoogleSdk? google,
    LineSdk? line,
    AuthLogger log = discardAuthLog,
  }) : _store = store,
       _gateway = gateway,
       _google = google ?? DeviceGoogleSdk(),
       _line = line ?? DeviceLineSdk(),
       _log = log,
       _token = restoredToken;

  /// 讀出保存的 token 再建立。保存處讀不到（Keychain 失敗）時以登出狀態開始。
  static Future<AuthSession> restore({
    required SessionTokenStore store,
    required IdentityGateway gateway,
    required AuthConfig config,
    GoogleSdk? google,
    LineSdk? line,
    AuthLogger log = discardAuthLog,
  }) async {
    String? token;
    try {
      token = await store.read();
    } catch (error, stack) {
      log(AuthLogLevel.error, '[Auth] 讀取 session token 失敗', error, stack);
    }
    return AuthSession(
      store: store,
      gateway: gateway,
      config: config,
      restoredToken: token,
      google: google,
      line: line,
      log: log,
    );
  }

  final AuthConfig config;
  final SessionTokenStore _store;
  final IdentityGateway _gateway;
  final GoogleSdk _google;
  final LineSdk _line;
  final AuthLogger _log;
  String? _token;

  bool get isSignedIn => _token != null && _token!.isNotEmpty;

  /// 沒有登入時回 null。
  Future<String?> token() async => _token;

  /// 以 [provider] 登入。使用者取消時回 false；失敗時拋 [SignInFailed] 或 [IdentityGateway] 的例外。
  ///
  /// 順序固定：先 `IssueAppLoginNonce`，再把 nonce 交給 SDK（Google 是重新 `initialize(nonce:)` 之後才
  /// `authenticate`，LINE 是 `idTokenNonce`），最後 `ExchangeIdToken`。Google 的 nonce 只能經 `initialize` 帶，
  /// 而每次登入的 nonce 都是新的，所以每次登入都重新 initialize（google_sign_in 文件寫只呼叫一次，iOS 實作只覆寫
  /// 設定；升級 SDK 時要重驗，`auth_session_test.dart` 守住順序）。
  Future<bool> signIn(OAuthProvider provider) async {
    if (!config.isConfigured(provider)) throw SignInFailed.notConfigured;
    final nonce = await _gateway.issueNonce();
    final String? idToken;
    try {
      idToken = await switch (provider) {
        OAuthProvider.google => _googleIdToken(nonce),
        OAuthProvider.line => _line.login(
          channelId: config.lineChannelId,
          nonce: nonce,
        ),
      };
    } catch (error, stack) {
      _log(AuthLogLevel.error, '[Auth] ${provider.label} 原生登入失敗', error, stack);
      throw SignInFailed(_kSignInFailedMessage, cause: error);
    }
    if (idToken == null) return false;
    if (idToken.isEmpty) {
      throw const SignInFailed(_kSignInFailedMessage);
    }
    final sessionToken = await _gateway.exchange(
      provider: provider,
      idToken: idToken,
      nonce: nonce,
    );
    try {
      await _store.write(sessionToken);
    } catch (error, stack) {
      _log(AuthLogLevel.error, '[Auth] 保存 session token 失敗', error, stack);
      throw SignInFailed(_kSignInFailedMessage, cause: error);
    }
    _token = sessionToken;
    return true;
  }

  Future<String?> _googleIdToken(String nonce) async {
    await _google.initialize(
      clientId: config.googleIosClientId,
      serverClientId: config.googleWebClientId,
      nonce: nonce,
    );
    return _google.authenticate();
  }

  /// 清除本機的登入狀態：先清記憶體中的 token（之後的請求不再帶），再刪保存的 token 並登出兩家 SDK。保存處或 SDK
  /// 失敗只記 log，本機照樣是登出。伺服器端的撤銷（`SignOut`）由 App 在呼叫這個之前送出，要帶這個 token。
  Future<void> clear() async {
    _token = null;
    Future<void> attempt(String what, Future<void> Function() action) async {
      try {
        await action();
      } catch (error, stack) {
        _log(AuthLogLevel.warning, '[Auth] $what失敗', error, stack);
      }
    }

    await attempt('刪除 session token ', _store.delete);
    await attempt('Google 登出', _google.signOut);
    await attempt('LINE 登出', _line.logout);
  }
}
