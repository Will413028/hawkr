import 'package:flutter/services.dart';
import 'package:flutter_line_sdk/flutter_line_sdk.dart';
import 'package:google_sign_in/google_sign_in.dart';

/// Google 原生登入的縫：測試以 fake 取代，確認呼叫順序（見 `AuthSession.signIn`）。
abstract interface class GoogleSdk {
  /// 對應 `GoogleSignIn.instance.initialize`。[clientId] 是 iOS 的 OAuth client（ID token 的 `azp`），
  /// [serverClientId] 是 web client（ID token 的 `aud`），[nonce] 是這次登入要寫進 ID token 的單次值。
  Future<void> initialize({
    required String clientId,
    required String serverClientId,
    required String nonce,
  });

  /// 顯示 Google 的登入畫面，回傳 ID token；使用者取消時回 null。
  Future<String?> authenticate();

  Future<void> signOut();
}

/// LINE 原生登入的縫。
abstract interface class LineSdk {
  /// 以 [nonce]（`LoginOption.idTokenNonce`）登入，回傳 ID token；使用者取消時回 null。
  Future<String?> login({required String channelId, required String nonce});

  Future<void> logout();
}

final class DeviceGoogleSdk implements GoogleSdk {
  DeviceGoogleSdk();

  /// `signOut` 在 `initialize` 之前呼叫會拋錯；這次啟動還沒登入過就沒有 SDK 狀態可清。
  bool _initialized = false;

  @override
  Future<void> initialize({
    required String clientId,
    required String serverClientId,
    required String nonce,
  }) async {
    await GoogleSignIn.instance.initialize(
      clientId: clientId,
      serverClientId: serverClientId,
      nonce: nonce,
    );
    _initialized = true;
  }

  @override
  Future<String?> authenticate() async {
    try {
      final account = await GoogleSignIn.instance.authenticate();
      return account.authentication.idToken;
    } on GoogleSignInException catch (error) {
      if (error.code == GoogleSignInExceptionCode.canceled) return null;
      rethrow;
    }
  }

  @override
  Future<void> signOut() async {
    if (!_initialized) return;
    await GoogleSignIn.instance.signOut();
  }
}

final class DeviceLineSdk implements LineSdk {
  DeviceLineSdk();

  String? _setupChannelId;

  /// SDK 要求 `setup` 只呼叫一次，而且在其他方法之前。
  Future<void> _ensureSetup(String channelId) async {
    if (_setupChannelId == channelId) return;
    await LineSDK.instance.setup(channelId);
    _setupChannelId = channelId;
  }

  @override
  Future<String?> login({
    required String channelId,
    required String nonce,
  }) async {
    await _ensureSetup(channelId);
    try {
      final result = await LineSDK.instance.login(
        scopes: const ['profile', 'openid', 'email'],
        option: LoginOption(false, 'normal')..idTokenNonce = nonce,
      );
      return result.accessToken.idTokenRaw;
    } on PlatformException catch (error) {
      if (isLineLoginCanceled(error)) return null;
      rethrow;
    }
  }

  @override
  Future<void> logout() async {
    // 沒登入過就沒有 SDK 狀態可清；SDK 也還沒 setup。
    if (_setupChannelId == null) return;
    await LineSDK.instance.logout();
  }
}

/// iOS 的 LINE SDK 把使用者取消回成 `LineSDKError.authorizeFailed`（錯誤碼 3003），原因 `userCancelled`
/// 在 message 與 details 裡。
bool isLineLoginCanceled(PlatformException error) {
  if (error.code != '3003') return false;
  final text = '${error.message} ${error.details}'.toLowerCase();
  return text.contains('cancel');
}
