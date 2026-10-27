import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_web_auth_2/flutter_web_auth_2.dart';
import 'package:sign_in_with_apple/sign_in_with_apple.dart';

import 'logging.dart';

enum OAuthProvider {
  google,
  apple,
  line;

  String get label => switch (this) {
    google => 'Google',
    apple => 'Apple',
    line => 'LINE',
  };
}

class AppleCredentials {
  const AppleCredentials({
    required this.identityToken,
    required this.authorizationCode,
  });

  final String identityToken;
  final String authorizationCode;
}

abstract interface class OAuthPicker {
  Future<Uri?> authenticateInBrowser(Uri url, String callbackScheme);

  Future<AppleCredentials?> pickApple();
}

class DeviceOAuthPicker implements OAuthPicker {
  const DeviceOAuthPicker({this.log = discardAuthLog});

  final AuthLogger log;

  @override
  Future<Uri?> authenticateInBrowser(Uri url, String callbackScheme) async {
    try {
      final result = await FlutterWebAuth2.authenticate(
        url: url.toString(),
        callbackUrlScheme: callbackScheme,
      );
      return Uri.parse(result);
    } on PlatformException catch (error, stack) {
      if (error.code == 'CANCELED') return null;
      log(AuthLogLevel.error, '[OAuthPicker] 瀏覽器登入失敗', error, stack);
      throw Exception('登入失敗，請稍後再試');
    }
  }

  @override
  Future<AppleCredentials?> pickApple() =>
      AppleAuthService.signInWithApple(log: log);
}

final oauthPickerProvider = Provider<OAuthPicker>(
  (ref) => DeviceOAuthPicker(log: ref.watch(authLoggerProvider)),
);

class AppleAuthService {
  static Future<AppleCredentials?> signInWithApple({
    AuthLogger log = discardAuthLog,
  }) async {
    try {
      final credential = await SignInWithApple.getAppleIDCredential(
        scopes: [
          AppleIDAuthorizationScopes.email,
          AppleIDAuthorizationScopes.fullName,
        ],
      );
      final identityToken = credential.identityToken ?? '';
      if (identityToken.isEmpty) {
        throw Exception('Apple 授權失敗，請稍後再試');
      }
      return AppleCredentials(
        identityToken: identityToken,
        authorizationCode: credential.authorizationCode,
      );
    } on SignInWithAppleAuthorizationException catch (error, stack) {
      if (error.code == AuthorizationErrorCode.canceled ||
          error.code == AuthorizationErrorCode.unknown) {
        return null;
      }
      log(AuthLogLevel.error, '[AppleAuth] 授權失敗', error, stack);
      throw Exception('Apple 登入失敗，請稍後再試');
    } catch (error, stack) {
      log(AuthLogLevel.error, '[AppleAuth] 登入失敗', error, stack);
      final message = error.toString();
      if (message.contains('canceled') || message.contains('cancelled')) {
        return null;
      }
      if (message.contains('network') || message.contains('Network')) {
        throw Exception('網路連線失敗，請檢查網路設定');
      }
      throw Exception('Apple 登入失敗，請稍後再試');
    }
  }
}
