import 'package:dio/dio.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../session/account_restriction.dart';
import '../session/auth_session_provider.dart';
import '../session/force_logout_provider.dart';
import '../session/session_provider.dart';
import 'suspension_signal.dart';

part 'auth_interceptor.g.dart';

@Riverpod(keepAlive: true)
AuthInterceptor authInterceptor(Ref ref) =>
    AuthInterceptor(() => currentSessionToken(ref), ref);

/// 登入後的身分：每個請求帶 identity 的 session token（`Authorization: Bearer`，訪客不帶）。token 是 Keychain 裡的
/// opaque token（ADR `2026-10-30-identity-self-hosted-federated-login.md`），不換新；讀不到 token 時不送出，以連線錯誤結束。
/// 收到 401 就強制登出（登出時清除 token），回到登入。403 的 envelope 帶 reason `ACCOUNT_SUSPENDED`
/// 時是停權（A7-D18），進受限狀態並提示，不登出。自己放行非 2xx 回應（`validateStatus`）的呼叫不經過 onError，
/// 使用者主動的寫入要自己用 [isRestSuspension] 判斷。背景寫入（[isBackgroundRestWrite]）收到停權時靜默。
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
      handler.reject(
        DioException(
          requestOptions: options,
          type: DioExceptionType.connectionError,
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
  Future<void> onError(
    DioException err,
    ErrorInterceptorHandler handler,
  ) async {
    try {
      final response = err.response;
      if (response?.statusCode == 401) {
        _triggerForceLogout();
      } else if (response != null &&
          !isBackgroundRestWrite(err.requestOptions) &&
          await isRestSuspension(response) &&
          _ref.mounted) {
        await _ref.read(accountRestrictionProvider.notifier).rejected();
      }
    } catch (_) {
      // 判斷與狀態更新的失敗不能吞掉原本的請求錯誤。
    } finally {
      handler.next(err);
    }
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
