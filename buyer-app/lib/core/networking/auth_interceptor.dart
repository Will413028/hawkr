import 'package:hawkr_auth/clerk.dart' show isSessionTokenRefused;
import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../session/clerk_session_provider.dart';
import '../session/force_logout_provider.dart';
import '../session/session_provider.dart';

part 'auth_interceptor.g.dart';

@Riverpod(keepAlive: true)
AuthInterceptor authInterceptor(Ref ref) =>
    AuthInterceptor(() => currentSessionToken(ref), ref);

/// 登入後的身分：每個請求帶 Clerk 的 session token（`Authorization: Bearer`，訪客不帶）。token 由 Clerk 的 SDK
/// 換新（ADR `2026-09-27-identity-clerk.md`）；取不到 token（連不上 Clerk）時不送出，以連線錯誤結束。收到 401
/// 就強制登出，回到登入。
class AuthInterceptor extends Interceptor {
  AuthInterceptor(this._sessionToken, this._ref);

  final Future<String?> Function() _sessionToken;
  final Ref _ref;

  @override
  Future<void> onRequest(
    RequestOptions options,
    RequestInterceptorHandler handler,
  ) async {
    final String? token;
    try {
      token = await _sessionToken();
    } catch (e) {
      final refused = isSessionTokenRefused(e);
      if (refused) _triggerForceLogout();
      handler.reject(
        DioException(
          requestOptions: options,
          type: refused
              ? DioExceptionType.cancel
              : DioExceptionType.connectionError,
          error: e,
          message: 'session token unavailable',
        ),
      );
      return;
    }
    if (token != null && token.isNotEmpty) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) {
    if (err.response?.statusCode == 401) {
      _triggerForceLogout();
    }
    handler.next(err);
  }

  void _triggerForceLogout() {
    // Apple App Review 合規：純訪客（從未登入、無 session）不該被「登入已過期」
    // 強制登出攔截。force-logout 只對「原本有 session、session 已失效」有意義；
    // 訪客頁面背景請求（orders / after_sales / member-cards 等）打到 authed
    // endpoint 回 401 時，若仍 trigger 會彈「登入已過期」並讓 ForceLogoutListener
    // 用無 Navigator 的 context 導頁而拋例外。
    final session = _ref.read(sessionProvider).value;
    if (session == null) return;
    _ref.read(forceLogoutProvider.notifier).trigger();
  }
}
