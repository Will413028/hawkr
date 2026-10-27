import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../features/chat/data/chat_repository.dart';
import '../../features/notification/data/push/manager.dart';
import '../../features/seller/data/current_store.dart';

/// 登入 / session 恢復後的副作用（chat polling、FCM 上報、seller 同步）。
///
/// 這些都是硬 singleton / 網路呼叫，在 `flutter test` 會丟
/// MissingPluginException 或留 pending timer。抽成可注入 seam：
/// 生產走 [RealPostLoginSideEffects]，測試 override 成 [NoopPostLoginSideEffects]。
abstract class PostLoginSideEffects {
  /// 主動登入成功後（fire-and-forget）。
  void onLogin();

  /// App 啟動以 GetSession 恢復 session 後（含 seller 同步，需 await）。
  Future<void> onSessionRestored();
}

class RealPostLoginSideEffects implements PostLoginSideEffects {
  RealPostLoginSideEffects(this._ref);

  final Ref _ref;

  @override
  void onLogin() {
    _ref.read(chatRepositoryProvider).startPolling();
    PushNotificationManager.instance.onUserLoggedIn();
  }

  @override
  Future<void> onSessionRestored() async {
    _ref.read(chatRepositoryProvider).startPolling();
    PushNotificationManager.instance.onUserLoggedIn();
    // 由 Session.initialize() 呼叫：等 currentStoreProvider 讀完 GetMyStore。它只依賴 session 的帳號，
    // 不會重新進入 initialize()。
    await _ref.read(currentStoreProvider.future);
  }
}

class NoopPostLoginSideEffects implements PostLoginSideEffects {
  const NoopPostLoginSideEffects();

  @override
  void onLogin() {}

  @override
  Future<void> onSessionRestored() async {}
}

final postLoginSideEffectsProvider = Provider<PostLoginSideEffects>(
  RealPostLoginSideEffects.new,
);
