import 'package:hawkr_auth/hawkr_auth.dart';

/// 測試用的 [AuthSession]：不碰 SDK、Keychain 與伺服器。登入中時 [token] 回 [tokenValue]；[tokenError] 不是 null 時
/// 讀 token 失敗；[signIn] 依 [signInResult] 回傳（false 是使用者取消），[signInError] 不是 null 時拋出。
/// [calls] 依序記錄登入與清除。
class FakeAuthSession implements AuthSession {
  FakeAuthSession({
    this.signedIn = false,
    this.tokenValue = 'live',
    List<String>? calls,
  }) : calls = calls ?? <String>[];

  bool signedIn;
  String? tokenValue;
  Object? tokenError;
  Object? signInError;
  bool signInResult = true;
  final List<String> calls;

  @override
  AuthConfig get config => const AuthConfig();

  @override
  bool get isSignedIn => signedIn;

  @override
  Future<String?> token() async {
    if (tokenError case final error?) throw error;
    return signedIn ? tokenValue : null;
  }

  @override
  Future<bool> signIn(OAuthProvider provider) async {
    calls.add('signIn ${provider.name}');
    if (signInError case final error?) throw error;
    if (!signInResult) return false;
    signedIn = true;
    return true;
  }

  @override
  Future<void> clear() async {
    calls.add('clear');
    signedIn = false;
  }
}
