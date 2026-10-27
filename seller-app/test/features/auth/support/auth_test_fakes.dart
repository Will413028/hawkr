import 'package:hawkr_auth/oauth.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:mocktail/mocktail.dart';

/// 可設定回傳 / 取消 / 拋例外的假 picker：[browserCallback] 是系統瀏覽器導回的網址（null = 取消），[apple] 是
/// Sign in with Apple 的憑證（null = 取消）。[browserSchemes] 記錄瀏覽器等待的 callback scheme。
class FakeOAuthPicker implements OAuthPicker {
  FakeOAuthPicker({this.browserCallback, this.apple, this.throwError});

  Uri? browserCallback;
  AppleCredentials? apple;
  Object? throwError;
  final browserSchemes = <String>[];
  var applePrompts = 0;

  @override
  Future<Uri?> authenticateInBrowser(Uri url, String callbackScheme) async {
    if (throwError != null) throw throwError!;
    browserSchemes.add(callbackScheme);
    return browserCallback;
  }

  @override
  Future<AppleCredentials?> pickApple() async {
    if (throwError != null) throw throwError!;
    applePrompts++;
    return apple;
  }
}

class FakeAuthRepository extends Mock implements AuthRepository {}
