import 'dart:async';

import 'package:hawkr_api/account.dart' show AppleHandOver;
import 'package:hawkr_auth/clerk.dart';
import 'package:hawkr_auth/oauth.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/clerk_session_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../seller/data/current_store.dart';
import '../data/auth_repository.dart';
import '../../../shared/utils/hk_log.dart';

export '../../../core/session/session_provider.dart' show sessionProvider;
export 'package:hawkr_auth/oauth.dart';

part 'auth_controller.g.dart';

/// 登入。
///
/// keepAlive：登入流程含原生 picker / 畫面切換的多個 await，期間 read 此
/// controller 的 widget（AuthScreen 只 read 不 watch）期間 provider 無 listener。
/// autoDispose 下 picker 回來後再用 ref 會拋「Cannot use the Ref ... after it
/// has been disposed」，導致流程直接中斷。keepAlive 讓 controller 在整段非同步流程存活（與 buyer app
/// 對齊）。
@Riverpod(keepAlive: true)
class AuthController extends _$AuthController {
  @override
  Future<void> build() async {}

  /// 以 [provider] 登入：Google、LINE 開系統瀏覽器，Apple 用 Sign in with Apple；Clerk 登入之後以 GetSession
  /// 取得帳號（第一次登入的使用者由伺服器在這時建立），寫入 session 與 onboarding 狀態。
  ///
  /// 使用者取消時回 false；其他錯誤一律 throw（`toString()` 是給使用者看的文字）。暫時錯誤保留 Clerk
  /// session，冷啟動可再呼叫 GetSession。
  Future<bool> signInWith(OAuthProvider provider) async {
    final clerk = await ref.read(clerkSessionProvider.future);
    final picker = ref.read(oauthPickerProvider);
    AppleCredentials? appleCredentials;
    final signedIn =
        clerk.isSignedIn ||
        switch (provider) {
          OAuthProvider.google => await _inBrowser(
            clerk,
            picker,
            BrowserSignIn.google,
          ),
          OAuthProvider.line => await _inBrowser(
            clerk,
            picker,
            BrowserSignIn.line,
          ),
          OAuthProvider.apple => await () async {
            appleCredentials = await _withApple(clerk, picker);
            return appleCredentials != null;
          }(),
        };
    if (!signedIn) return false;

    ref.read(sessionProvider.notifier).cancelRestore();
    final String accountId;
    try {
      accountId = await ref.read(authRepositoryProvider).currentAccountId();
    } catch (error) {
      if (error is AuthRequestException && !error.isTransient) {
        try {
          await clerk.signOut();
        } catch (signOutError, stack) {
          HKLog.logE('[AuthController] Clerk 登出失敗', signOutError, stack);
        }
      } else {
        ref.read(sessionProvider.notifier).retryRestore();
      }
      rethrow;
    }
    final credentials = appleCredentials;
    if (credentials != null) unawaited(_linkApple(credentials));
    await ref.read(sessionProvider.notifier).login(accountId: accountId);
    await ref.read(currentStoreProvider.future);
    return true;
  }

  Future<bool> _inBrowser(
    ClerkSession clerk,
    OAuthPicker picker,
    BrowserSignIn browser,
  ) => clerk.signInWithBrowser(
    browser,
    redirect: kOAuthRedirect,
    authenticate: picker.authenticateInBrowser,
  );

  Future<AppleCredentials?> _withApple(
    ClerkSession clerk,
    OAuthPicker picker,
  ) async {
    final credentials = await picker.pickApple();
    if (credentials == null) return null;
    await clerk.signInWithApple(credentials.identityToken);
    return credentials;
  }

  /// 不阻擋登入：失敗不影響登入，下一次原生 Apple 登入會再送一次。
  Future<void> _linkApple(AppleCredentials credentials) async {
    try {
      final handOver = await ref
          .read(authRepositoryProvider)
          .linkAppleCredential(
            identityToken: credentials.identityToken,
            authorizationCode: credentials.authorizationCode,
          );
      switch (handOver.result) {
        case AppleHandOver.linked:
          break;
        case AppleHandOver.unavailable:
          HKLog.logD('[AuthController] 伺服器暫時無法收下 Apple 憑證: ${handOver.error}');
        case AppleHandOver.failed:
          HKLog.logE('[AuthController] 交付 Apple 憑證失敗', handOver.error);
      }
    } catch (error, stack) {
      HKLog.logE('[AuthController] 交付 Apple 憑證失敗', error, stack);
    }
  }

  /// 登出：先註銷裝置、再登出 Clerk、最後清空本機（見 [Session.logout]）。
  Future<void> logout() => ref.read(sessionProvider.notifier).logout();

  /// 讀取目前 session
  SessionModel get session => ref.read(sessionProvider);
}
