import 'package:hawkr_auth/oauth.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/auth_session_provider.dart';
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

  /// 以 [provider] 登入：Google、LINE 的原生 SDK 取得 ID token，換到 identity 的 session token；之後以 GetSession
  /// 取得帳號（第一次登入的使用者由伺服器在這時建立），寫入 session 與 onboarding 狀態。
  ///
  /// 使用者取消時回 false；其他錯誤一律 throw（`toString()` 是給使用者看的文字）。暫時錯誤保留已換到的
  /// session token，冷啟動可再呼叫 GetSession。
  Future<bool> signInWith(OAuthProvider provider) async {
    final authSession = await ref.read(authSessionProvider.future);
    final signedIn =
        authSession.isSignedIn || await authSession.signIn(provider);
    if (!signedIn) return false;

    ref.read(sessionProvider.notifier).cancelRestore();
    final String accountId;
    try {
      accountId = await ref.read(authRepositoryProvider).currentAccountId();
    } catch (error) {
      if (error is AuthRequestException && !error.isTransient) {
        try {
          await authSession.clear();
        } catch (clearError, stack) {
          HKLog.logE('[AuthController] 清除登入狀態失敗', clearError, stack);
        }
      } else {
        ref.read(sessionProvider.notifier).retryRestore();
      }
      rethrow;
    }
    await ref.read(sessionProvider.notifier).login(accountId: accountId);
    await ref.read(currentStoreProvider.future);
    return true;
  }

  /// 登出：先註銷裝置、再撤銷 session 並清除 token、最後清空本機（見 [Session.logout]）。
  Future<void> logout() => ref.read(sessionProvider.notifier).logout();

  /// 讀取目前 session
  SessionModel get session => ref.read(sessionProvider);
}
