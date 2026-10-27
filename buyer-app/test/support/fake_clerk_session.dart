import 'package:hawkr_auth/clerk.dart';

/// 測試用的 [ClerkSession]：不連 Clerk。登入中時 [token] 回 [tokenValue]；[tokenError] 不是 null 時換 token 失敗
/// （連不上 Clerk）。[calls] 依序記錄登入與登出。
class FakeClerkSession implements ClerkSession {
  FakeClerkSession({
    this.signedIn = false,
    this.tokenValue = 'live',
    this.email,
  });

  bool signedIn;
  String? tokenValue;
  Object? tokenError;

  @override
  String? email;
  final calls = <String>[];

  @override
  bool get isSignedIn => signedIn;

  @override
  Future<String?> token() async {
    if (tokenError case final error?) throw error;
    return signedIn ? tokenValue : null;
  }

  /// 以 [authenticate] 模擬系統瀏覽器：回 null（使用者取消）時不登入。
  @override
  Future<bool> signInWithBrowser(
    BrowserSignIn provider, {
    required Uri redirect,
    required BrowserAuthenticator authenticate,
  }) async {
    calls.add('browser ${provider.name}');
    final callback = await authenticate(
      Uri.parse('https://clerk.example/oauth/${provider.name}'),
      redirect.scheme,
    );
    if (callback == null) return false;
    signedIn = true;
    return true;
  }

  @override
  Future<void> signInWithApple(String identityToken) async {
    calls.add('apple $identityToken');
    signedIn = true;
  }

  @override
  Future<void> signOut() async {
    calls.add('signOut');
    signedIn = false;
  }

  @override
  void dispose() {}
}
