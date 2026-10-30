import 'package:hawkr_auth/clerk.dart';
import 'package:hawkr_auth/oauth.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/clerk_session_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../shared/utils/hk_log.dart';
import '../data/auth_repository.dart';

export '../../../core/session/session_provider.dart' show sessionProvider;
export 'package:hawkr_auth/oauth.dart';

part 'auth_controller.g.dart';

/// 唯一的登入入口：以 Clerk 登入（Google、LINE 經系統瀏覽器，Apple 用 Sign in with Apple），再以 GetSession
/// 取得帳號（第一次登入的使用者由伺服器在這時建立），寫入 session。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 watch 此
/// controller 的 widget 可能釋放。Riverpod 3 下 autoDispose 後再用 ref 會拋
/// 「Cannot use the Ref ... after it has been disposed」，導致流程在
/// ref.read 直接中斷。keepAlive 讓 controller 在整段非同步流程存活。
@Riverpod(keepAlive: true)
class AuthController extends _$AuthController {
  @override
  AsyncValue<void> build() => const AsyncData(null);

  /// 以 [provider] 登入。使用者在瀏覽器或 Apple 的畫面取消時回 false；其他錯誤一律 throw（`toString()` 是給
  /// 使用者看的文字）；暫時錯誤保留 Clerk session，冷啟動可再呼叫 GetSession。
  Future<bool> signInWith(OAuthProvider provider) async {
    state = const AsyncLoading();
    try {
      await ref.read(sessionProvider.future);
      final clerk = await ref.read(clerkSessionProvider.future);
      final signedIn =
          clerk.isSignedIn || await _signInToClerk(clerk, provider);
      if (signedIn) {
        await _completeSignIn(clerk);
      }
      state = const AsyncData(null);
      return signedIn;
    } catch (e, st) {
      state = AsyncError(e, st);
      rethrow;
    }
  }

  // ─────────────────────── internals ───────────────────────

  Future<bool> _signInToClerk(
    ClerkSession clerk,
    OAuthProvider provider,
  ) async {
    final picker = ref.read(oauthPickerProvider);
    Future<bool> inBrowser(BrowserSignIn browser) => clerk.signInWithBrowser(
      browser,
      redirect: kOAuthRedirect,
      authenticate: picker.authenticateInBrowser,
    );
    switch (provider) {
      case OAuthProvider.google:
        return inBrowser(BrowserSignIn.google);
      case OAuthProvider.line:
        return inBrowser(BrowserSignIn.line);
      case OAuthProvider.apple:
        final credentials = await picker.pickApple();
        if (credentials == null) return false;
        await clerk.signInWithApple(credentials.identityToken);
        return true;
    }
  }

  /// Clerk 登入之後：GetSession 取得帳號、寫入 session。
  Future<void> _completeSignIn(ClerkSession clerk) async {
    ref.read(sessionProvider.notifier).cancelRestore();
    final String accountId;
    try {
      accountId = await ref.read(authRepositoryProvider).currentAccountId();
    } catch (error) {
      if (error is UnauthorizedException || error is SuspendedException) {
        try {
          await clerk.signOut();
        } catch (signOutError, stack) {
          HKLog.logE('[AuthController] Clerk 登出失敗', signOutError, stack);
        }
      } else {
        ref
            .read(sessionProvider.notifier)
            .retryRestore(onSuccess: _finishSignIn);
      }
      rethrow;
    }
    await _finishSignIn(accountId);
  }

  Future<void> _finishSignIn(String accountId) async {
    await ref.read(sessionProvider.notifier).login(userId: accountId);
    // GetSession 沒有個人資料，暱稱從 GET /app/buyers/me 讀 — 失敗不影響登入。
    try {
      final nickname = await ref.read(sessionProvider.notifier).fetchNickname();
      if (nickname != null) {
        await ref.read(sessionProvider.notifier).updateNickname(nickname);
      }
    } catch (e) {
      HKLog.logD('[AuthController] profile sync skipped: $e');
    }
  }
}
