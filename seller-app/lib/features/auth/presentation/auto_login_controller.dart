import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/session_provider.dart';
import '../../seller/data/current_store.dart';
import '../../../shared/utils/hk_log.dart';

part 'auto_login_controller.g.dart';

/// 冷啟動自動登入的結果，WelcomeScreen 依它決定導去哪裡。
sealed class AutoLoginResult {
  const AutoLoginResult();
}

/// 沒有可用的帳號：到登入頁。
class AutoLoginSignedOut extends AutoLoginResult {
  const AutoLoginSignedOut();
}

/// 從 GetSession 恢復了 session。[needsOnboarding] 是 `GetMyStore` 的答案。
class AutoLoginSignedIn extends AutoLoginResult {
  const AutoLoginSignedIn({required this.needsOnboarding});

  final bool needsOnboarding;
}

/// 冷啟動自動登入：有保存的 session token 時以 GetSession 恢復帳號。
///
/// 登入是否仍然有效由伺服器判斷：失效時請求收到 401，由中央 Dio 登出（見 `dio_provider.dart`）。
///
/// keepAlive：整段流程只碰自己的 `ref`，與呼叫它的 widget 的生命週期脫鉤。widget 若在流程中途
/// dispose（導頁離開），續行程式碼不會對已失效的 `WidgetRef` 呼叫 `read`；session 狀態一變化，
/// GoRouter 的 redirect 照樣會導頁。
@Riverpod(keepAlive: true)
class AutoLoginController extends _$AutoLoginController {
  @override
  void build() {}

  /// 由 WelcomeScreen mount 時呼叫一次。
  Future<AutoLoginResult> check() async {
    await ref.read(sessionProvider.notifier).initialize();
    final session = ref.read(sessionProvider);
    if (!session.isLoggedIn) {
      HKLog.logD('【自動登入】沒有登入');
      return const AutoLoginSignedOut();
    }
    HKLog.logD('【自動登入】已從 GetSession 恢復 session');
    return AutoLoginSignedIn(
      needsOnboarding: ref.read(needsOnboardingProvider) == true,
    );
  }
}
